-- 000017_drop_original_prices_is_active.down.sql
-- 回滚：恢复 original_prices.is_active 字段
-- 注意：回滚后所有记录的 is_active 默认为 TRUE（无法恢复原 is_active=false 的状态）
-- 如需禁用某条记录，请直接删除该记录
ALTER TABLE original_prices
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
