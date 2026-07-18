// Package repository - 残值评估主表 evaluations 数据访问
// 手写 pgx 仓储，参考 battery.go 风格
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/cache"
	"forklift-training/internal/valuation/model"
)

// EvaluationRepository 评估记录仓储
type EvaluationRepository struct {
	pool *pgxpool.Pool
}

// NewEvaluationRepository 构造评估记录仓储
func NewEvaluationRepository(pool *pgxpool.Pool) *EvaluationRepository {
	return &EvaluationRepository{pool: pool}
}

// CreateEvaluationParams 创建评估记录参数（与 evaluations 表字段对齐）
// 输入字段从 EvaluationRequest 派生，结果字段由 service 计算填入
type CreateEvaluationParams struct {
	// 输入字段
	Brand                      string
	VehicleType                string
	Series                     string
	Tonnage                    float64
	ConfigType                 string
	MastType                   string
	MastHeightMM               int
	FactoryYear                int
	SaleYear                   int
	UsageHours                 int
	OriginalPaint              bool
	Province                   string
	City                       string
	HasLicensePlate            bool
	HasRegistrationCertificate bool
	HasMaintenanceRecords      bool
	ConditionRating            string
	// 结果字段
	OriginalPrice  float64
	KTime          float64
	KHours         float64
	KBrand         float64
	KCondition     float64
	KMarket        float64
	EstimatedValue float64
	ConfidenceLow  float64
	ConfidenceHigh float64
	ReportPdfPath  string
}

// CreateEvaluation 插入评估主记录，返回新 ID
func (r *EvaluationRepository) CreateEvaluation(ctx context.Context, p *CreateEvaluationParams) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO evaluations (
			brand, vehicle_type, series, tonnage,
			config_type, mast_type, mast_height_mm,
			factory_year, sale_year, usage_hours, original_paint,
			province, city,
			has_license_plate, has_registration_certificate, has_maintenance_records,
			condition_rating,
			original_price, k_time, k_hours, k_brand, k_condition, k_market,
			estimated_value, confidence_low, confidence_high, report_pdf_path
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11,
			$12, $13,
			$14, $15, $16,
			$17,
			$18, $19, $20, $21, $22, $23,
			$24, $25, $26, $27
		)
		RETURNING id, created_at, updated_at`,
		p.Brand, p.VehicleType, p.Series, p.Tonnage,
		p.ConfigType, p.MastType, p.MastHeightMM,
		p.FactoryYear, p.SaleYear, p.UsageHours, p.OriginalPaint,
		p.Province, p.City,
		p.HasLicensePlate, p.HasRegistrationCertificate, p.HasMaintenanceRecords,
		p.ConditionRating,
		p.OriginalPrice, p.KTime, p.KHours, p.KBrand, p.KCondition, p.KMarket,
		p.EstimatedValue, p.ConfidenceLow, p.ConfidenceHigh, nullableString(p.ReportPdfPath),
	).Scan(&id, new(time.Time), new(time.Time))
	if err != nil {
		return 0, fmt.Errorf("插入评估记录失败: %w", err)
	}
	// 失效列表与统计缓存（新建记录会改变 list 与 count 结果）
	_ = cache.InvalidatePattern(ctx, "eval:count:*")
	_ = cache.InvalidatePattern(ctx, "eval:list:*")
	return id, nil
}

// GetEvaluation 按 ID 查询评估详情
func (r *EvaluationRepository) GetEvaluation(ctx context.Context, id int64) (*model.EvaluationDetail, error) {
		cacheKey := cache.SafeKey("eval", "get", fmt.Sprintf("%d", id))
		var result model.EvaluationDetail
		err := cache.GetOrSetJSON(ctx, cacheKey, 10*time.Minute, &result, func() (any, error) {
			row := r.pool.QueryRow(ctx, `
			SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm,
			       factory_year, sale_year, usage_hours, original_paint,
			       province, city,
			       has_license_plate, has_registration_certificate, has_maintenance_records,
			       condition_rating,
			       original_price, k_time, k_hours, k_brand, k_condition, k_market,
			       estimated_value, confidence_low, confidence_high, report_pdf_path,
			       created_at, updated_at
			FROM evaluations WHERE id = $1`, id)
		var (
			d          model.EvaluationDetail
			reportPath *string
			createdAt  time.Time
			updatedAt  time.Time
		)
		if err := row.Scan(
			&d.ID, &d.Brand, &d.VehicleType, &d.Series, &d.Tonnage,
			&d.ConfigType, &d.MastType, &d.MastHeightMM,
			&d.FactoryYear, &d.SaleYear, &d.UsageHours, &d.OriginalPaint,
			&d.Province, &d.City,
			&d.HasLicensePlate, &d.HasRegistrationCertificate, &d.HasMaintenanceRecords,
			&d.ConditionRating,
			&d.OriginalPrice, &d.KTime, &d.KHours, &d.KBrand, &d.KCondition, &d.KMarket,
			&d.EstimatedValue, &d.ConfidenceLow, &d.ConfidenceHigh, &reportPath,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if reportPath != nil {
			d.ReportPdfPath = *reportPath
		}
		d.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
		d.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		return d, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListEvaluations 分页查询评估列表
// brand 为空时不过滤
func (r *EvaluationRepository) ListEvaluations(ctx context.Context, brand string, limit, offset int) ([]model.EvaluationDetail, error) {
	cacheKey := cache.SafeKey("eval", "list", brand, fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset))
		var result []model.EvaluationDetail
		err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLStats, &result, func() (any, error) {
			var rows pgx.Rows
			var err error
			if brand != "" {
			rows, err = r.pool.Query(ctx, `
				SELECT id, brand, vehicle_type, series, tonnage,
				       config_type, mast_type, mast_height_mm,
				       factory_year, sale_year, usage_hours, original_paint,
				       province, city,
				       has_license_plate, has_registration_certificate, has_maintenance_records,
				       condition_rating,
				       original_price, k_time, k_hours, k_brand, k_condition, k_market,
				       estimated_value, confidence_low, confidence_high, report_pdf_path,
				       created_at, updated_at
				FROM evaluations WHERE brand = $1
				ORDER BY created_at DESC LIMIT $2 OFFSET $3`, brand, limit, offset)
		} else {
			rows, err = r.pool.Query(ctx, `
				SELECT id, brand, vehicle_type, series, tonnage,
				       config_type, mast_type, mast_height_mm,
				       factory_year, sale_year, usage_hours, original_paint,
				       province, city,
				       has_license_plate, has_registration_certificate, has_maintenance_records,
				       condition_rating,
				       original_price, k_time, k_hours, k_brand, k_condition, k_market,
				       estimated_value, confidence_low, confidence_high, report_pdf_path,
				       created_at, updated_at
				FROM evaluations
				ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
		}
		if err != nil {
			return nil, fmt.Errorf("查询评估列表失败: %w", err)
		}
		defer rows.Close()
		out := make([]model.EvaluationDetail, 0, limit)
		for rows.Next() {
			var d model.EvaluationDetail
			var reportPath *string
			var createdAt time.Time
			var updatedAt time.Time
			if err := rows.Scan(
				&d.ID, &d.Brand, &d.VehicleType, &d.Series, &d.Tonnage,
				&d.ConfigType, &d.MastType, &d.MastHeightMM,
				&d.FactoryYear, &d.SaleYear, &d.UsageHours, &d.OriginalPaint,
				&d.Province, &d.City,
				&d.HasLicensePlate, &d.HasRegistrationCertificate, &d.HasMaintenanceRecords,
				&d.ConditionRating,
				&d.OriginalPrice, &d.KTime, &d.KHours, &d.KBrand, &d.KCondition, &d.KMarket,
				&d.EstimatedValue, &d.ConfidenceLow, &d.ConfidenceHigh, &reportPath,
				&createdAt, &updatedAt,
			); err != nil {
				return nil, err
			}
			if reportPath != nil {
				d.ReportPdfPath = *reportPath
			}
			d.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
			d.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
			out = append(out, d)
		}
		return out, rows.Err()
	})
	return result, err
}

// CountEvaluations 统计评估记录总数
// brand 为空时统计全部
func (r *EvaluationRepository) CountEvaluations(ctx context.Context, brand string) (int, error) {
	cacheKey := cache.SafeKey("eval", "count", brand)
		var result int
		err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLStats, &result, func() (any, error) {
			var total int
			if brand != "" {
				if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM evaluations WHERE brand = $1`, brand).Scan(&total); err != nil {
				return nil, err
			}
		} else {
			if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM evaluations`).Scan(&total); err != nil {
				return nil, err
			}
		}
		return total, nil
	})
	return result, err
}

// UpdateEvaluationReportPath 更新 PDF 报告路径
func (r *EvaluationRepository) UpdateEvaluationReportPath(ctx context.Context, id int64, path string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evaluations SET report_pdf_path = $2, updated_at = NOW() WHERE id = $1`,
		id, nullableString(path))
	if err != nil {
		return err
	}
	// 失效该条记录的详情缓存
	_ = cache.Del(ctx, cache.SafeKey("eval", "get", fmt.Sprintf("%d", id)))
	return nil
}

// EvaluationExists 轻量级存在性判断
func (r *EvaluationRepository) EvaluationExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM evaluations WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}
