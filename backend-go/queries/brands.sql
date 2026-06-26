-- queries/brands.sql
-- 品牌表数据访问查询

-- name: ListBrands :many
-- 查询所有激活的品牌，按档次与名称排序
SELECT id, name, tier, k_brand, is_active, models, created_at
FROM brands
WHERE is_active = TRUE
ORDER BY k_brand DESC, name ASC;

-- name: GetBrandByName :one
-- 根据品牌名查询品牌
SELECT id, name, tier, k_brand, is_active, models, created_at
FROM brands
WHERE name = $1;

-- name: GetBrandByID :one
-- 根据 ID 查询品牌
SELECT id, name, tier, k_brand, is_active, models, created_at
FROM brands
WHERE id = $1;
