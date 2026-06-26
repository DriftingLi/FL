-- queries/historical_sales.sql
-- 历史成交数据表访问查询（用于算法离线校准）

-- name: CreateHistoricalSale :one
-- 插入一条历史成交记录
INSERT INTO historical_sales (
    forklift_type, brand, model, original_price, purchase_year, sale_year,
    usage_hours, work_condition, fuel_type, sale_price
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10
)
RETURNING id, imported_at;

-- name: ListHistoricalSales :many
-- 按叉车类型与品牌查询历史成交数据
SELECT id, forklift_type, brand, model, original_price, purchase_year, sale_year,
       usage_hours, work_condition, fuel_type, sale_price, imported_at
FROM historical_sales
WHERE (sqlc.narg('forklift_type')::TEXT IS NULL OR forklift_type = sqlc.narg('forklift_type'))
  AND (sqlc.narg('brand')::TEXT IS NULL OR brand = sqlc.narg('brand'))
ORDER BY imported_at DESC
LIMIT $1;
