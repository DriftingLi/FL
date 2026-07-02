-- queries/evaluations.sql
-- 残值评估记录表数据访问查询（重构后字段）
-- 注意：sqlc 未启用，本文件仅作为 SQL 参考文档保留；仓储代码以手写 pgx 方式实现
-- 重构说明：brand_type 列已移除（brand_types 表已删除，Kb = k_brand）

-- name: CreateEvaluation :one
-- 创建评估记录主表，返回新生成的 ID 与时间戳
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
RETURNING id, created_at, updated_at;

-- name: GetEvaluation :one
-- 根据 ID 查询评估记录主表
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
WHERE id = $1;

-- name: ListEvaluations :many
-- 分页查询评估历史列表（可选按品牌筛选）
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
WHERE (sqlc.narg('brand')::TEXT IS NULL OR brand = sqlc.narg('brand'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountEvaluations :one
-- 统计评估记录总数（用于分页）
SELECT COUNT(*) AS total
FROM evaluations
WHERE (sqlc.narg('brand')::TEXT IS NULL OR brand = sqlc.narg('brand'));

-- name: UpdateEvaluationReportPath :one
-- 更新评估记录的 PDF 报告路径
UPDATE evaluations
SET report_pdf_path = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, report_pdf_path, updated_at;
