package valuation

import (
	"context"
	"fmt"
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

func (r *PostgresTrialSnapshotReader) LoadTrialSnapshot(ctx context.Context) (*TrialSnapshot, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("试算快照 PostgreSQL 连接池未装配")
	}
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
		usage_hour_ratios, condition_ratings, enabled FROM valuation_trial_policy ORDER BY id`)
	if err != nil {
		return in, fmt.Errorf("读取试算策略快照失败: %w", err)
	}
	for rows.Next() {
		var policy TrialPolicy
		if err := rows.Scan(&policy.ID, &policy.Name, &policy.Version, &policy.Brand, &policy.VehicleType,
			&policy.Tonnage, &policy.MaxWidthRatio, &policy.UsageHourRatios, &policy.ConditionRatings, &policy.Enabled); err != nil {
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
