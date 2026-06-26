-- queries/coefficient_configs.sql
-- 系数配置表数据访问查询

-- name: ListCoefficientConfigs :many
-- 查询所有系数配置
SELECT id, key, value, description, updated_at
FROM coefficient_configs
ORDER BY key ASC;

-- name: GetCoefficientByKey :one
-- 根据配置键查询系数
SELECT id, key, value, description, updated_at
FROM coefficient_configs
WHERE key = $1;

-- name: UpdateCoefficientValue :one
-- 更新某个系数的值，返回更新后的记录
UPDATE coefficient_configs
SET value = $2, updated_at = NOW()
WHERE key = $1
RETURNING id, key, value, description, updated_at;
