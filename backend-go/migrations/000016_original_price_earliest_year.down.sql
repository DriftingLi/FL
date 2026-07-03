-- 000016_original_price_earliest_year.down.sql
-- 回滚：删除 earliest_factory_year 字段
-- 注意：config_type/mast_type 的 '无' 值不回滚（无法还原原始空值）

ALTER TABLE original_prices DROP COLUMN IF EXISTS earliest_factory_year;
