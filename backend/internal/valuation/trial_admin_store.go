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
			return TrialControl{}, fmt.Errorf("试算全局开关缺失，按关闭处理")
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
	if err := s.pool.QueryRow(ctx, `UPDATE valuation_trial_control
		SET enabled = $1, version = version + 1, updated_at = NOW() WHERE id = 1
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
		if _, err := tx.Exec(ctx, `INSERT INTO valuation_trial_policy
			(policy_name, version, brand, vehicle_type, tonnage, max_width_ratio, usage_hour_ratios,
			condition_ratings, original_paint_values, has_license_plate_values,
			has_registration_certificate_values, has_maintenance_records_values, enabled)
			VALUES ($1, 1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb, $12)`,
			strings.TrimSpace(policy.Name), strings.TrimSpace(policy.Brand), strings.TrimSpace(policy.VehicleType), policy.Tonnage,
			policy.MaxWidthRatio, mustJSON(policy.UsageHourRatios), mustJSON(policy.ConditionRatings),
			mustJSON(policy.OriginalPaintValues), mustJSON(policy.HasLicensePlateValues),
			mustJSON(policy.HasRegistrationCertificateValues), mustJSON(policy.HasMaintenanceRecordsValues), policy.Enabled); err != nil {
			return fmt.Errorf("保存试算策略失败: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交试算策略更新事务失败: %w", err)
	}
	return nil
}

func (s *PostgresTrialAdminStore) RecordTrialMetric(ctx context.Context, event TrialMetricEvent) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("试算指标 PostgreSQL 连接池未装配")
	}
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO valuation_trial_metrics
		(occurred_at, brand, vehicle_type, tonnage, policy_name, scenario, formula_version, formula_hash,
		policy_version, denominator, priced, covered, too_wide, unpriced_reason, follow_up_count,
		duration_ms, model_name, prompt_tokens, completion_tokens, cost_micros)
		VALUES ($1::timestamptz,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		event.OccurredAt, event.Brand, event.VehicleType, event.Tonnage, event.PolicyName, event.Scenario,
		event.FormulaVersion, event.FormulaHash, event.PolicyVersion, event.Denominator, event.Priced,
		event.Covered, event.TooWide, event.UnpricedReason, event.FollowUpCount, event.DurationMS,
		event.ModelName, event.PromptTokens, event.CompletionTokens, event.CostMicros); err != nil {
		return fmt.Errorf("记录试算指标失败: %w", err)
	}
	return nil
}

func (s *PostgresTrialAdminStore) ListTrialMetrics(ctx context.Context, filter TrialMetricFilter) ([]TrialMetricSummary, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("试算指标 PostgreSQL 连接池未装配")
	}
	where, args := trialMetricWhere(filter)
	rows, err := s.pool.Query(ctx, `SELECT brand, vehicle_type, tonnage, policy_name, scenario,
		formula_version, formula_hash, policy_version, SUM(denominator), SUM(priced), SUM(covered),
		SUM(too_wide), SUM(follow_up_count), SUM(duration_ms), SUM(prompt_tokens), SUM(completion_tokens), SUM(cost_micros)
		FROM valuation_trial_metrics `+where+`
		GROUP BY brand, vehicle_type, tonnage, policy_name, scenario, formula_version, formula_hash, policy_version
		ORDER BY brand, vehicle_type, tonnage, policy_name, scenario, formula_hash, policy_version
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
	reasonRows, err := s.pool.Query(ctx, `SELECT brand, vehicle_type, tonnage, policy_name, scenario,
		formula_version, formula_hash, policy_version, unpriced_reason, COUNT(*)
		FROM valuation_trial_metrics `+reasonWhere+`
		GROUP BY brand, vehicle_type, tonnage, policy_name, scenario, formula_version, formula_hash, policy_version, unpriced_reason`,
		args...)
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

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
