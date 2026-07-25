-- 000005_valuation_evaluations_user_id.down.sql
-- 回滚：移除 user_id 字段及其索引
DROP INDEX IF EXISTS idx_battery_evaluations_user;
DROP INDEX IF EXISTS idx_evaluations_user;

ALTER TABLE battery_evaluations DROP COLUMN IF EXISTS user_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS user_id;
