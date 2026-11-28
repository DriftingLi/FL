package valuation

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTrialSnapshotReader 在单个只读可重复读事务中冻结全部试算事实。
type PostgresTrialSnapshotReader struct {
	pool *pgxpool.Pool
}

func NewPostgresTrialSnapshotReader(pool *pgxpool.Pool) *PostgresTrialSnapshotReader {
	return &PostgresTrialSnapshotReader{pool: pool}
}

func (r *PostgresTrialSnapshotReader) LoadTrialSnapshot(ctx context.Context, req TrialRequest) (*TrialSnapshot, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("试算快照 PostgreSQL 连接池未装配")
	}
	if err := validateTrialSnapshotReference(req); err != nil {
		return nil, err
	}
	var snapshot *TrialSnapshot
	if req.SnapshotID != "" {
		stored, err := r.loadStoredTrialSnapshot(ctx, req.SnapshotID)
		if err != nil {
			return nil, err
		}
		snapshot = stored
	} else {
		stored, err := r.loadTrialSnapshotByRequestID(ctx, req)
		if err == nil {
			snapshot = stored
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		} else {
			current, err := r.loadCurrentTrialSnapshot(ctx)
			if err != nil {
				return nil, err
			}
			requestJSON, err := json.Marshal(req)
			if err != nil {
				return nil, fmt.Errorf("序列化试算请求失败: %w", err)
			}
			snapshotJSON, err := json.Marshal(current.ReplayInput)
			if err != nil {
				return nil, fmt.Errorf("序列化试算快照失败: %w", err)
			}
			if _, err := r.pool.Exec(ctx, `INSERT INTO valuation_trial_snapshots
				(snapshot_id, request_id, request_json, snapshot_json, formula_hash)
				VALUES ($1, $2, $3::jsonb, $4::jsonb, $5)
				ON CONFLICT (request_id) DO NOTHING`, current.Hash, req.RequestID, requestJSON, snapshotJSON, TrialFormulaHash); err != nil {
				return nil, fmt.Errorf("保存试算快照失败: %w", err)
			}
			snapshot, err = r.loadTrialSnapshotByRequestID(ctx, req)
			if err != nil {
				return nil, err
			}
		}
	}
	control, err := loadTrialControl(ctx, r.pool)
	if errors.Is(err, ErrTrialControlMissing) {
		// 单行缺失按「关闭」处理：后面的 preflight 会给出 403 global_disabled，
		// 不能把配置缺失变成 500（#2958 审查项 5）。
		control = TrialControl{}
	} else if err != nil {
		return nil, err
	}
	snapshot.Control = control
	return snapshot, nil
}

func validateTrialSnapshotReference(req TrialRequest) error {
	if strings.TrimSpace(req.RequestID) == "" || len(req.RequestID) > 128 {
		return fmt.Errorf("试算 request_id 缺失或过长")
	}
	if req.SnapshotID == "" {
		return nil
	}
	decoded, err := hex.DecodeString(req.SnapshotID)
	if err != nil || len(decoded) != 32 || req.SnapshotID != strings.ToLower(req.SnapshotID) {
		return fmt.Errorf("试算 snapshot_id 必须是快照 SHA-256")
	}
	return nil
}

func (r *PostgresTrialSnapshotReader) loadStoredTrialSnapshot(ctx context.Context, snapshotID string) (*TrialSnapshot, error) {
	var requestJSON, snapshotJSON, formulaHash string
	err := r.pool.QueryRow(ctx, `SELECT request_json, snapshot_json, formula_hash
		FROM valuation_trial_snapshots WHERE snapshot_id = $1 ORDER BY id LIMIT 1`, snapshotID).
		Scan(&requestJSON, &snapshotJSON, &formulaHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("试算快照不存在: %s", snapshotID)
	}
	if err != nil {
		return nil, fmt.Errorf("读取试算快照失败: %w", err)
	}
	return decodeStoredTrialSnapshot(snapshotID, requestJSON, snapshotJSON, formulaHash)
}

func (r *PostgresTrialSnapshotReader) loadTrialSnapshotByRequestID(ctx context.Context, req TrialRequest) (*TrialSnapshot, error) {
	var snapshotID, storedRequestJSON, snapshotJSON, formulaHash string
	err := r.pool.QueryRow(ctx, `SELECT snapshot_id, request_json, snapshot_json, formula_hash
		FROM valuation_trial_snapshots WHERE request_id = $1`, req.RequestID).
		Scan(&snapshotID, &storedRequestJSON, &snapshotJSON, &formulaHash)
	if err != nil {
		return nil, err
	}
	currentJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化试算幂等请求失败: %w", err)
	}
	var storedReq TrialRequest
	if err := json.Unmarshal([]byte(storedRequestJSON), &storedReq); err != nil {
		return nil, fmt.Errorf("解析试算幂等请求失败: %w", err)
	}
	storedJSON, err := json.Marshal(storedReq)
	if err != nil {
		return nil, fmt.Errorf("规范化试算幂等请求失败: %w", err)
	}
	if !bytes.Equal(currentJSON, storedJSON) {
		return nil, fmt.Errorf("request_id 已用于不同试算请求")
	}
	return decodeStoredTrialSnapshot(snapshotID, storedRequestJSON, snapshotJSON, formulaHash)
}

func decodeStoredTrialSnapshot(snapshotID, requestJSON, snapshotJSON, formulaHash string) (*TrialSnapshot, error) {
	var req TrialRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return nil, fmt.Errorf("解析试算快照请求失败: %w", err)
	}
	var input ReplayInput
	if err := json.Unmarshal([]byte(snapshotJSON), &input); err != nil {
		return nil, fmt.Errorf("解析试算快照载荷失败: %w", err)
	}
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		return nil, err
	}
	if formulaHash != TrialFormulaHash || snapshot.Hash != snapshotID {
		return nil, fmt.Errorf("试算快照哈希或公式版本校验失败")
	}
	return snapshot, nil
}

func (r *PostgresTrialSnapshotReader) loadCurrentTrialSnapshot(ctx context.Context) (*TrialSnapshot, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("开始试算快照事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	input, err := loadTrialSnapshotTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("提交试算快照读取事务失败: %w", err)
	}
	return NewTrialSnapshot(input)
}

type trialSnapshotQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadTrialSnapshotTx(ctx context.Context, tx trialSnapshotQuerier) (ReplayInput, error) {
	var in ReplayInput
	rows, err := tx.Query(ctx, `SELECT id, name, k_brand, is_active FROM brands ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算品牌快照失败: %w", err)
	}
	for rows.Next() {
		var brand Brand
		if err := rows.Scan(&brand.ID, &brand.Name, &brand.KBrand, &brand.IsActive); err != nil {
			rows.Close()
			return in, err
		}
		in.Brands = append(in.Brands, brand)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, name, power_type, earliest_factory_year FROM vehicle_types ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算车型快照失败: %w", err)
	}
	for rows.Next() {
		var vehicleType VehicleType
		if err := rows.Scan(&vehicleType.ID, &vehicleType.Name, &vehicleType.PowerType, &vehicleType.EarliestFactoryYear); err != nil {
			rows.Close()
			return in, err
		}
		in.VehicleTypes = append(in.VehicleTypes, vehicleType)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, rating, label, base_coefficient FROM condition_ratings ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算车况快照失败: %w", err)
	}
	for rows.Next() {
		var condition ConditionRating
		if err := rows.Scan(&condition.ID, &condition.Rating, &condition.Label, &condition.BaseCoefficient); err != nil {
			rows.Close()
			return in, err
		}
		in.ConditionRatings = append(in.ConditionRatings, condition)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, province, city, coefficient FROM region_coefficients ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算区域快照失败: %w", err)
	}
	for rows.Next() {
		var region RegionCoefficient
		if err := rows.Scan(&region.ID, &region.Province, &region.City, &region.Coefficient); err != nil {
			rows.Close()
			return in, err
		}
		in.RegionCoefficients = append(in.RegionCoefficients, region)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT `+originalPriceCols+` FROM original_prices ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算原价快照失败: %w", err)
	}
	for rows.Next() {
		price, err := scanOriginalPrice(rows)
		if err != nil {
			rows.Close()
			return in, err
		}
		in.OriginalPrices = append(in.OriginalPrices, price)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, key, value, description, updated_at FROM coefficient_configs ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算系数快照失败: %w", err)
	}
	for rows.Next() {
		var coefficient CoefficientConfig
		var updatedAt time.Time
		if err := rows.Scan(&coefficient.ID, &coefficient.Key, &coefficient.Value, &coefficient.Description, &updatedAt); err != nil {
			rows.Close()
			return in, err
		}
		coefficient.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		in.Coefficients = append(in.Coefficients, coefficient)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, policy_name, version, brand, vehicle_type, tonnage, max_width_ratio,
		usage_hour_ratios, condition_ratings, original_paint_values, has_license_plate_values,
		has_registration_certificate_values, has_maintenance_records_values, enabled FROM valuation_trial_policy ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算策略快照失败: %w", err)
	}
	for rows.Next() {
		var policy TrialPolicy
		if err := rows.Scan(&policy.ID, &policy.Name, &policy.Version, &policy.Brand, &policy.VehicleType,
			&policy.Tonnage, &policy.MaxWidthRatio, &policy.UsageHourRatios, &policy.ConditionRatings,
			&policy.OriginalPaintValues, &policy.HasLicensePlateValues, &policy.HasRegistrationCertificateValues,
			&policy.HasMaintenanceRecordsValues, &policy.Enabled); err != nil {
			rows.Close()
			return in, err
		}
		in.TrialPolicies = append(in.TrialPolicies, policy)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return in, err
	}
	rows.Close()
	return in, nil
}
