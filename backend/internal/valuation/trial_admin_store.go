package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TrialAdminStore 是试算开关、策略与指标的管理存储 seam。
type TrialAdminStore interface {
	GetTrialControl(context.Context) (TrialControl, error)
	SetTrialControl(context.Context, bool) (TrialControl, error)
	ListTrialPolicies(context.Context, TrialPolicyFilter) ([]TrialPolicy, error)
	ReplaceTrialPolicies(context.Context, []TrialPolicyInput) error
	ListTrialMetrics(context.Context, TrialMetricFilter) ([]TrialMetricSummary, error)
	RecordTrialMetric(context.Context, TrialMetricEvent) error
}

// TrialMetricRecorder 只接收已投影指标，避免 HTTP 出口接触存储实现。
type TrialMetricRecorder interface {
	RecordTrialMetric(context.Context, TrialMetricEvent) error
}

// ErrTrialControlMissing 表示全局开关单行缺失。调用方按「关闭」处理（试算回 403 global_disabled），
// 不得升级成 500。
var ErrTrialControlMissing = errors.New("试算全局开关记录缺失，按关闭处理")

// TrialPolicyFilter 管理查询过滤项；空字段表示不过滤。
type TrialPolicyFilter struct {
	PolicyName  string
	Brand       string
	VehicleType string
	Tonnage     *float64
}

// TrialMetricFilter 管理查询过滤项；时间为 RFC3339，Limit 默认 200。
type TrialMetricFilter struct {
	From          string
	To            string
	PolicyName    string
	Brand         string
	VehicleType   string
	Tonnage       *float64
	Scenario      string
	FormulaHash   string
	PolicyVersion string
	Limit         int
}

// PostgresTrialAdminStore 使用 valuation 共享 pgx pool。
type PostgresTrialAdminStore struct {
	pool *pgxpool.Pool
}

func NewPostgresTrialAdminStore(pool *pgxpool.Pool) *PostgresTrialAdminStore {
	return &PostgresTrialAdminStore{pool: pool}
}

func (s *PostgresTrialAdminStore) GetTrialControl(ctx context.Context) (TrialControl, error) {
	if s == nil || s.pool == nil {
		return TrialControl{}, fmt.Errorf("试算控制 PostgreSQL 连接池未装配")
	}
	return loadTrialControl(ctx, s.pool)
}

type trialControlQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadTrialControl(ctx context.Context, q trialControlQuerier) (TrialControl, error) {
	var control TrialControl
	var updatedAt time.Time
	if err := q.QueryRow(ctx, `SELECT enabled, version, updated_at FROM valuation_trial_control WHERE id = 1`).
		Scan(&control.Enabled, &control.Version, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TrialControl{}, ErrTrialControlMissing
		}
		return TrialControl{}, fmt.Errorf("读取试算全局开关失败: %w", err)
	}
	control.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return control, nil
}

func (s *PostgresTrialAdminStore) SetTrialControl(ctx context.Context, enabled bool) (TrialControl, error) {
	if s == nil || s.pool == nil {
		return TrialControl{}, fmt.Errorf("试算控制 PostgreSQL 连接池未装配")
	}
	var control TrialControl
	var updatedAt time.Time
	// upsert 而非纯 UPDATE：单行缺失时关闭动作也必须能落地（缺失按关闭处理，不允许把恢复路径也堵死）。
	if err := s.pool.QueryRow(ctx, `INSERT INTO valuation_trial_control (id, enabled, version, updated_at)
		VALUES (1, $1, 1, NOW())
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled,
			version = valuation_trial_control.version + 1, updated_at = NOW()
		RETURNING enabled, version, updated_at`, enabled).Scan(&control.Enabled, &control.Version, &updatedAt); err != nil {
		return TrialControl{}, fmt.Errorf("更新试算全局开关失败: %w", err)
	}
	control.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return control, nil
}

const trialPolicyColumns = `id, policy_name, version, brand, vehicle_type, tonnage, max_width_ratio,
	usage_hour_ratios, condition_ratings, original_paint_values, has_license_plate_values,
	has_registration_certificate_values, has_maintenance_records_values, enabled`

func scanTrialPolicy(row pgx.Row) (TrialPolicy, error) {
	var policy TrialPolicy
	err := row.Scan(&policy.ID, &policy.Name, &policy.Version, &policy.Brand, &policy.VehicleType,
		&policy.Tonnage, &policy.MaxWidthRatio, &policy.UsageHourRatios, &policy.ConditionRatings,
		&policy.OriginalPaintValues, &policy.HasLicensePlateValues, &policy.HasRegistrationCertificateValues,
		&policy.HasMaintenanceRecordsValues, &policy.Enabled)
	return policy, err
}

func (s *PostgresTrialAdminStore) ListTrialPolicies(ctx context.Context, filter TrialPolicyFilter) ([]TrialPolicy, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("试算策略 PostgreSQL 连接池未装配")
	}
	tonnage := 0.0
	if filter.Tonnage != nil {
		tonnage = *filter.Tonnage
	}
	rows, err := s.pool.Query(ctx, `SELECT `+trialPolicyColumns+` FROM valuation_trial_policy
		WHERE ($1 = '' OR policy_name = $1) AND ($2 = '' OR brand = $2)
		AND ($3 = '' OR vehicle_type = $3) AND ($4 = 0 OR tonnage = $4::numeric)
		ORDER BY policy_name, brand, vehicle_type, tonnage`, filter.PolicyName, filter.Brand, filter.VehicleType, tonnage)
	if err != nil {
		return nil, fmt.Errorf("查询试算策略失败: %w", err)
	}
	defer rows.Close()
	policies := make([]TrialPolicy, 0)
	for rows.Next() {
		policy, err := scanTrialPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

// trialPolicyPayload 是策略情景列在写库前的 JSONB 投影；编码失败必须返回错误而不是 panic。
type trialPolicyPayload struct {
	usageHourRatios, conditionRatings, originalPaint, licensePlate, registration, maintenance string
}

func encodeTrialPolicyPayload(policy TrialPolicyInput) (trialPolicyPayload, error) {
	var payload trialPolicyPayload
	for _, field := range []struct {
		name   string
		value  any
		target *string
	}{
		{"usage_hour_ratios", policy.UsageHourRatios, &payload.usageHourRatios},
		{"condition_ratings", policy.ConditionRatings, &payload.conditionRatings},
		{"original_paint_values", policy.OriginalPaintValues, &payload.originalPaint},
		{"has_license_plate_values", policy.HasLicensePlateValues, &payload.licensePlate},
		{"has_registration_certificate_values", policy.HasRegistrationCertificateValues, &payload.registration},
		{"has_maintenance_records_values", policy.HasMaintenanceRecordsValues, &payload.maintenance},
	} {
		encoded, err := json.Marshal(field.value)
		if err != nil {
			return trialPolicyPayload{}, fmt.Errorf("序列化试算策略 %s 失败: %w", field.name, err)
		}
		*field.target = string(encoded)
	}
	return payload, nil
}

func (s *PostgresTrialAdminStore) ReplaceTrialPolicies(ctx context.Context, policies []TrialPolicyInput) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("试算策略 PostgreSQL 连接池未装配")
	}
	if err := validateTrialPolicySet(policies); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始试算策略更新事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM valuation_trial_policy`); err != nil {
		return fmt.Errorf("清空试算策略失败: %w", err)
	}
	for _, policy := range policies {
		payload, err := encodeTrialPolicyPayload(policy)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO valuation_trial_policy
			(policy_name, version, brand, vehicle_type, tonnage, max_width_ratio, usage_hour_ratios,
			condition_ratings, original_paint_values, has_license_plate_values,
			has_registration_certificate_values, has_maintenance_records_values, enabled)
			VALUES ($1, 1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb, $12)`,
			strings.TrimSpace(policy.Name), strings.TrimSpace(policy.Brand), strings.TrimSpace(policy.VehicleType), policy.Tonnage,
			policy.MaxWidthRatio, payload.usageHourRatios, payload.conditionRatings,
			payload.originalPaint, payload.licensePlate, payload.registration, payload.maintenance, policy.Enabled); err != nil {
			return fmt.Errorf("保存试算策略失败: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交试算策略更新事务失败: %w", err)
	}
	return nil
}

// 指标维度列是 Event 写入与 Summary 聚合读面的**单一投影**：INSERT 列、GROUP BY、ORDER BY、
// unpriced_reasons 的关联键都从这里来，改一处即改全链。
var trialMetricDimensionColumns = []string{
	"brand", "vehicle_type", "tonnage", "policy_name", "scenario", "formula_version", "formula_hash", "policy_version",
}

// 指标度量列与 trialMetricEventValues 的取值顺序一一对应；aggregate 是聚合读面同一列的表达式
// （空表示该列只写不聚合），这样 Event 写入与 Summary 读面共用一张字段表（单一投影）。
var trialMetricMeasures = []struct {
	column    string
	aggregate string
}{
	{"denominator", "SUM(denominator)"},
	{"priced", "SUM(priced)"},
	{"covered", "SUM(covered)"},
	{"too_wide", "SUM(too_wide)"},
	{"unpriced_reason", ""},
	{"follow_up_count", "SUM(follow_up_count)"},
	{"duration_ms", "SUM(duration_ms)"},
	{"model_name", ""},
	{"prompt_tokens", "SUM(prompt_tokens)"},
	{"completion_tokens", "SUM(completion_tokens)"},
	{"cost_micros", "SUM(cost_micros)"},
}

func trialMetricMeasureColumns() []string {
	columns := make([]string, len(trialMetricMeasures))
	for i, measure := range trialMetricMeasures {
		columns[i] = measure.column
	}
	return columns
}

// trialMetricSummaryAggregates 是 Summary 的 SUM 表达式清单（保持度量列顺序，与 Scan 顺序一致）。
func trialMetricSummaryAggregates() string {
	parts := make([]string, 0, len(trialMetricMeasures))
	for _, measure := range trialMetricMeasures {
		if measure.aggregate != "" {
			parts = append(parts, measure.aggregate)
		}
	}
	return strings.Join(parts, ", ")
}

// trialMetricInsertSQL 由列清单生成 INSERT（含 occurred_at 的时间类型转换）。
func trialMetricInsertSQL() string {
	columns := append([]string{"occurred_at"}, trialMetricDimensionColumns...)
	columns = append(columns, trialMetricMeasureColumns()...)
	placeholders := make([]string, len(columns))
	for i, column := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		if column == "occurred_at" {
			placeholders[i] += "::timestamptz"
		}
	}
	return `INSERT INTO valuation_trial_metrics (` + strings.Join(columns, ", ") + `) VALUES (` +
		strings.Join(placeholders, ", ") + `)`
}

// trialMetricEventValues 按列清单同一顺序取值，杜绝 INSERT 列表与实参错位。
func trialMetricEventValues(event TrialMetricEvent) []any {
	return []any{
		event.OccurredAt,
		event.Brand, event.VehicleType, event.Tonnage, event.PolicyName, event.Scenario,
		event.FormulaVersion, event.FormulaHash, event.PolicyVersion,
		event.Denominator, event.Priced, event.Covered, event.TooWide, event.UnpricedReason,
		event.FollowUpCount, event.DurationMS, event.ModelName, event.PromptTokens,
		event.CompletionTokens, event.CostMicros,
	}
}

func (s *PostgresTrialAdminStore) RecordTrialMetric(ctx context.Context, event TrialMetricEvent) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("试算指标 PostgreSQL 连接池未装配")
	}
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := s.pool.Exec(ctx, trialMetricInsertSQL(), trialMetricEventValues(event)...); err != nil {
		return fmt.Errorf("记录试算指标失败: %w", err)
	}
	return nil
}

func (s *PostgresTrialAdminStore) ListTrialMetrics(ctx context.Context, filter TrialMetricFilter) ([]TrialMetricSummary, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("试算指标 PostgreSQL 连接池未装配")
	}
	where, args := trialMetricWhere(filter)
	dimensions := strings.Join(trialMetricDimensionColumns, ", ")
	rows, err := s.pool.Query(ctx, `SELECT `+dimensions+`, `+trialMetricSummaryAggregates()+`
		FROM valuation_trial_metrics `+where+`
		GROUP BY `+dimensions+`
		ORDER BY `+dimensions+`
		LIMIT $`+fmt.Sprint(len(args)+1), append(args, filter.Limit)...)
	if err != nil {
		return nil, fmt.Errorf("查询试算指标失败: %w", err)
	}
	defer rows.Close()
	items := make([]TrialMetricSummary, 0)
	for rows.Next() {
		var item TrialMetricSummary
		if err := rows.Scan(&item.Brand, &item.VehicleType, &item.Tonnage, &item.PolicyName, &item.Scenario,
			&item.FormulaVersion, &item.FormulaHash, &item.PolicyVersion, &item.Denominator, &item.Priced,
			&item.Covered, &item.TooWide, &item.FollowUpCount, &item.DurationSumMS, &item.PromptTokens,
			&item.CompletionTokens, &item.CostMicros); err != nil {
			return nil, err
		}
		item.UnpricedReasons = make(map[string]int64)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reasonWhere := where + " AND unpriced_reason <> ''"
	if where == "" {
		reasonWhere = "WHERE unpriced_reason <> ''"
	}
	reasonRows, err := s.pool.Query(ctx, `SELECT `+dimensions+`, unpriced_reason, COUNT(*)
		FROM valuation_trial_metrics `+reasonWhere+`
		GROUP BY `+dimensions+`, unpriced_reason`, args...)
	if err != nil {
		return nil, fmt.Errorf("查询试算无价原因失败: %w", err)
	}
	defer reasonRows.Close()
	indices := make(map[string]int, len(items))
	for i, item := range items {
		indices[trialMetricKey(item.Brand, item.VehicleType, fmt.Sprintf("%.2f", item.Tonnage), item.PolicyName, item.Scenario, item.FormulaVersion, item.FormulaHash, item.PolicyVersion)] = i
	}
	for reasonRows.Next() {
		var brand, vehicleType, policyName, scenario, formulaVersion, formulaHash, policyVersion, reason string
		var tonnage float64
		var count int64
		if err := reasonRows.Scan(&brand, &vehicleType, &tonnage, &policyName, &scenario, &formulaVersion,
			&formulaHash, &policyVersion, &reason, &count); err != nil {
			return nil, err
		}
		if index, ok := indices[trialMetricKey(brand, vehicleType, fmt.Sprintf("%.2f", tonnage), policyName, scenario, formulaVersion, formulaHash, policyVersion)]; ok {
			items[index].UnpricedReasons[reason] = count
		}
	}
	return items, reasonRows.Err()
}

func trialMetricWhere(filter TrialMetricFilter) (string, []any) {
	conditions := make([]string, 0, 9)
	args := make([]any, 0, 9)
	add := func(column, operator string, value any) {
		args = append(args, value)
		conditions = append(conditions, column+" "+operator+fmt.Sprintf("$%d", len(args)))
	}
	if filter.From != "" {
		add("occurred_at", ">=", filter.From+"::timestamptz")
	}
	if filter.To != "" {
		add("occurred_at", "<=", filter.To+"::timestamptz")
	}
	if filter.PolicyName != "" {
		add("policy_name", "=", filter.PolicyName)
	}
	if filter.Brand != "" {
		add("brand", "=", filter.Brand)
	}
	if filter.VehicleType != "" {
		add("vehicle_type", "=", filter.VehicleType)
	}
	if filter.Tonnage != nil {
		add("tonnage", "=", fmt.Sprintf("%.2f", *filter.Tonnage)+"::numeric")
	}
	if filter.Scenario != "" {
		add("scenario", "=", filter.Scenario)
	}
	if filter.FormulaHash != "" {
		add("formula_hash", "=", filter.FormulaHash)
	}
	if filter.PolicyVersion != "" {
		add("policy_version", "=", filter.PolicyVersion)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func trialMetricKey(parts ...string) string {
	return strings.Join(parts, "\x00")
}
