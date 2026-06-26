-- queries/evaluations.sql
-- 评估记录与部件状态表数据访问查询

-- name: CreateEvaluation :one
-- 创建评估记录主表，返回新生成的 ID
INSERT INTO evaluations (
    forklift_type, brand, model, original_price, purchase_year, sale_year,
    usage_hours, work_condition, fuel_type, can_drive, hydraulic_ok,
    k_time, k_hours, k_work, k_brand, k_condition, k_market,
    estimated_value, confidence_low, confidence_high, report_pdf_path
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21
)
RETURNING id, created_at, updated_at;

-- name: CreateEvaluationItem :one
-- 创建单条部件状态记录
INSERT INTO evaluation_items (
    evaluation_id, category_code, category_name, item_code, item_name,
    status, category_weight, item_weight, score
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9
)
RETURNING id;

-- name: GetEvaluation :one
-- 根据 ID 查询评估记录主表
SELECT id, forklift_type, brand, model, original_price, purchase_year, sale_year,
       usage_hours, work_condition, fuel_type, can_drive, hydraulic_ok,
       k_time, k_hours, k_work, k_brand, k_condition, k_market,
       estimated_value, confidence_low, confidence_high, report_pdf_path,
       created_at, updated_at
FROM evaluations
WHERE id = $1;

-- name: ListEvaluationItems :many
-- 根据评估 ID 查询所有部件状态
SELECT id, evaluation_id, category_code, category_name, item_code, item_name,
       status, category_weight, item_weight, score
FROM evaluation_items
WHERE evaluation_id = $1
ORDER BY id ASC;

-- name: ListEvaluations :many
-- 分页查询评估历史列表（可选按类型筛选）
SELECT id, forklift_type, brand, model, original_price, purchase_year, sale_year,
       usage_hours, work_condition, fuel_type, can_drive, hydraulic_ok,
       k_time, k_hours, k_work, k_brand, k_condition, k_market,
       estimated_value, confidence_low, confidence_high, report_pdf_path,
       created_at, updated_at
FROM evaluations
WHERE (sqlc.narg('forklift_type')::TEXT IS NULL OR forklift_type = sqlc.narg('forklift_type'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountEvaluations :one
-- 统计评估记录总数（用于分页）
SELECT COUNT(*) AS total
FROM evaluations
WHERE (sqlc.narg('forklift_type')::TEXT IS NULL OR forklift_type = sqlc.narg('forklift_type'));

-- name: UpdateEvaluationReportPath :one
-- 更新评估记录的 PDF 报告路径
UPDATE evaluations
SET report_pdf_path = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, report_pdf_path, updated_at;
