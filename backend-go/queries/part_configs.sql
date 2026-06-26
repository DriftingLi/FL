-- queries/part_configs.sql
-- 部件配置表数据访问查询

-- name: ListPartConfigs :many
-- 根据叉车类型查询所有部件配置，按类别与条目顺序排序
SELECT id, forklift_type, category_code, category_name, category_weight,
       item_code, item_name, item_weight
FROM part_configs
WHERE forklift_type = $1
ORDER BY id ASC;

-- name: GetPartConfigByItemCode :one
-- 根据叉车类型和条目编码查询部件配置
SELECT id, forklift_type, category_code, category_name, category_weight,
       item_code, item_name, item_weight
FROM part_configs
WHERE forklift_type = $1 AND item_code = $2;
