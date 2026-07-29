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
	// 归属字段
	// UserID 为 0 表示匿名提交（user_id 落 NULL）；>0 表示登录用户提交
	UserID int
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
			estimated_value, confidence_low, confidence_high, report_pdf_path,
			user_id
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11,
			$12, $13,
			$14, $15, $16,
			$17,
			$18, $19, $20, $21, $22, $23,
			$24, $25, $26, $27,
			$28
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
		nullableUserID(p.UserID),
	).Scan(&id, new(time.Time), new(time.Time))
	if err != nil {
		return 0, fmt.Errorf("插入评估记录失败: %w", err)
	}
	// 失效列表与统计缓存（新建记录会改变 list 与 count 结果）
	_ = cache.InvalidatePattern(ctx, "eval:count:*")
	_ = cache.InvalidatePattern(ctx, "eval:list:*")
	return id, nil
}

// GetEvaluation 按 ID 查询评估详情（不按用户过滤）
// 用于公开的报告生成/下载场景（report.go），鉴权详情请用 GetEvaluationByUser
func (r *EvaluationRepository) GetEvaluation(ctx context.Context, id int64) (*model.EvaluationDetail, error) {
	cacheKey := cache.SafeKey("eval", "get", fmt.Sprintf("%d", id))
	var result model.EvaluationDetail
	err := cache.GetOrSetJSON(ctx, cacheKey, 10*time.Minute, &result, func() (any, error) {
		return r.scanEvaluationByID(ctx, id, 0, false)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEvaluationByUser 按 ID 查询评估详情，并校验归属（user_id 必须等于 userID）
// 用于登录用户查看自己的历史详情；不属于该用户的记录返回 pgx.ErrNoRows
func (r *EvaluationRepository) GetEvaluationByUser(ctx context.Context, id int64, userID int) (*model.EvaluationDetail, error) {
	// 详情缓存 key 带上 userID，避免跨用户串缓存
	cacheKey := cache.SafeKey("eval", "get", "user", fmt.Sprintf("%d", userID), fmt.Sprintf("%d", id))
	var result model.EvaluationDetail
	err := cache.GetOrSetJSON(ctx, cacheKey, 10*time.Minute, &result, func() (any, error) {
		return r.scanEvaluationByID(ctx, id, userID, true)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// scanEvaluationByID 执行单行查询并扫描为 EvaluationDetail。
// enforceOwner=true 时追加 user_id = $userID 过滤；=false 时仅按 id 查询（公开场景）
func (r *EvaluationRepository) scanEvaluationByID(ctx context.Context, id int64, userID int, enforceOwner bool) (model.EvaluationDetail, error) {
	var (
		d          model.EvaluationDetail
		reportPath *string
		createdAt  time.Time
		updatedAt  time.Time
	)
	var row pgx.Row
	if enforceOwner {
		row = r.pool.QueryRow(ctx, `
			SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm,
			       factory_year, sale_year, usage_hours, original_paint,
			       province, city,
			       has_license_plate, has_registration_certificate, has_maintenance_records,
			       condition_rating,
			       original_price, k_time, k_hours, k_brand, k_condition, k_market,
			       estimated_value, confidence_low, confidence_high, report_pdf_path,
			       created_at, updated_at
			FROM evaluations WHERE id = $1 AND user_id = $2`, id, userID)
	} else {
		row = r.pool.QueryRow(ctx, `
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
	}
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
		return d, err
	}
	if reportPath != nil {
		d.ReportPdfPath = *reportPath
	}
	d.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
	d.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return d, nil
}

// ListEvaluations 分页查询评估列表
// brand 为空时不过滤；userID>0 时仅返回该用户的记录，userID=0 时返回全部（公开统计场景）
func (r *EvaluationRepository) ListEvaluations(ctx context.Context, brand string, userID int, limit, offset int) ([]model.EvaluationDetail, error) {
	cacheKey := cache.SafeKey("eval", "list", brand, fmt.Sprintf("u%d", userID), fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset))
	var result []model.EvaluationDetail
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLStats, &result, func() (any, error) {
		// 动态拼装 WHERE：brand / user_id 均为可选过滤
		where := make([]string, 0, 2)
		args := make([]any, 0, 3)
		argIdx := 1
		if brand != "" {
			where = append(where, fmt.Sprintf("brand = $%d", argIdx))
			args = append(args, brand)
			argIdx++
		}
		if userID > 0 {
			where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
			args = append(args, userID)
			argIdx++
		}
		whereClause := ""
		if len(where) > 0 {
			whereClause = "WHERE " + joinStrings(where, " AND ")
		}
		args = append(args, limit, offset)
		query := fmt.Sprintf(`
			SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm,
			       factory_year, sale_year, usage_hours, original_paint,
			       province, city,
			       has_license_plate, has_registration_certificate, has_maintenance_records,
			       condition_rating,
			       original_price, k_time, k_hours, k_brand, k_condition, k_market,
			       estimated_value, confidence_low, confidence_high, report_pdf_path,
			       created_at, updated_at
			FROM evaluations %s
			ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)
		rows, err := r.pool.Query(ctx, query, args...)
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
// brand 为空时不过滤；userID>0 时仅统计该用户的记录，userID=0 时统计全部（公开统计场景）
func (r *EvaluationRepository) CountEvaluations(ctx context.Context, brand string, userID int) (int, error) {
	cacheKey := cache.SafeKey("eval", "count", brand, fmt.Sprintf("u%d", userID))
	var result int
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLStats, &result, func() (any, error) {
		where := make([]string, 0, 2)
		args := make([]any, 0, 2)
		argIdx := 1
		if brand != "" {
			where = append(where, fmt.Sprintf("brand = $%d", argIdx))
			args = append(args, brand)
			argIdx++
		}
		if userID > 0 {
			where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
			args = append(args, userID)
			argIdx++
		}
		whereClause := ""
		if len(where) > 0 {
			whereClause = "WHERE " + joinStrings(where, " AND ")
		}
		var total int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM evaluations %s`, whereClause)
		if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
			return nil, err
		}
		return total, nil
	})
	return result, err
}

// nullableUserID 把 0 转为 nil 便于 user_id 落 NULL（匿名提交）
func nullableUserID(uid int) any {
	if uid <= 0 {
		return nil
	}
	return uid
}

// joinStrings 用 sep 连接字符串切片（避免引入 strings 包的小工具）
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
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
