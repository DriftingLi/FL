-- 000017_drop_original_prices_is_active.up.sql
-- 删除 original_prices 表的 is_active 字段
-- 原价表是纯数据表，记录要么有用要么删除，不需要软禁用功能
-- 该字段反而引入了 bug：前端旧表单未传 is_active 时 Go bool 默认 false，导致新增记录被级联查询过滤
-- 删除字段后，原 is_active=false 的记录会自动在学生端级联查询中可见
ALTER TABLE original_prices
    DROP COLUMN IF EXISTS is_active;
