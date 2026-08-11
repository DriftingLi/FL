// Package repository - 主 app 导出模块的估值数据 adapter
// 实现 service.ExportStore（seam 定义在消费方，见 spec #75 D4）。
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	vmain "forklift-training/internal/service"
)

// ExportStore 主 app 导出模块消费的估值数据访问 adapter（pgx 实现）。
type ExportStore struct {
	pool *pgxpool.Pool
}

// NewExportStore 构造导出 adapter。
func NewExportStore(pool *pgxpool.Pool) *ExportStore {
	return &ExportStore{pool: pool}
}

// ListEvaluationExports 评估记录导出行（与导出契约列一一对应，含主表用户 join）。
func (s *ExportStore) ListEvaluationExports(ctx context.Context) ([]vmain.EvaluationExportRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, COALESCE(u.account, '') AS account, COALESCE(u.username, '') AS username,
		       e.brand, e.vehicle_type, e.series, e.tonnage, e.config_type, e.mast_type,
		       e.mast_height_mm, e.factory_year, e.sale_year, e.usage_hours, e.original_paint,
		       e.province, e.city, e.has_license_plate, e.has_registration_certificate AS has_registration_cert,
		       e.has_maintenance_records, e.condition_rating, e.original_price, e.k_time, e.k_hours,
		       e.k_brand, e.k_condition, e.k_market, e.estimated_value, e.confidence_low, e.confidence_high,
		       e.report_pdf_path, e.created_at
		FROM evaluations AS e
		LEFT JOIN hrwai_users AS u ON u.id = e.user_id
		ORDER BY e.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]vmain.EvaluationExportRow, 0, 16)
	for rows.Next() {
		var r vmain.EvaluationExportRow
		var kTime, kHours, kBrand, kCondition, kMarket *float64
		var confLow, confHigh *float64
		var reportPath *string
		var createdAt time.Time
		if err := rows.Scan(
			&r.ID, &r.Account, &r.Username, &r.Brand, &r.VehicleType, &r.Series, &r.Tonnage,
			&r.ConfigType, &r.MastType, &r.MastHeightMM, &r.FactoryYear, &r.SaleYear,
			&r.UsageHours, &r.OriginalPaint, &r.Province, &r.City,
			&r.HasLicensePlate, &r.HasRegistrationCert, &r.HasMaintenanceRecords,
			&r.ConditionRating, &r.OriginalPrice,
			&kTime, &kHours, &kBrand, &kCondition, &kMarket,
			&r.EstimatedValue, &confLow, &confHigh, &reportPath, &createdAt,
		); err != nil {
			return nil, err
		}
		r.KTime, r.KHours, r.KBrand, r.KCondition, r.KMarket = kTime, kHours, kBrand, kCondition, kMarket
		r.ConfidenceLow, r.ConfidenceHigh = confLow, confHigh
		if reportPath != nil {
			r.ReportPDFPath = *reportPath
		}
		r.CreatedAt = createdAt
		out = append(out, r)
	}
	return out, rows.Err()
}
