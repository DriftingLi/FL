-- 000005_valuation_evaluations_user_id.up.sql
-- 为 evaluations 与 battery_evaluations 增加 user_id 字段，实现历史记录按用户隔离
-- user_id 可空：兼容历史匿名数据；登录用户提交时写入其 valuation_users.id
ALTER TABLE evaluations
    ADD COLUMN IF NOT EXISTS user_id INTEGER;

ALTER TABLE battery_evaluations
    ADD COLUMN IF NOT EXISTS user_id INTEGER;

-- 按用户查询历史记录的高频索引
CREATE INDEX IF NOT EXISTS idx_evaluations_user ON evaluations(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_battery_evaluations_user ON battery_evaluations(user_id, created_at DESC);

COMMENT ON COLUMN evaluations.user_id IS '残值评估提交者（valuation_users.id），NULL 表示匿名提交';
COMMENT ON COLUMN battery_evaluations.user_id IS '电池评估提交者（valuation_users.id），NULL 表示历史匿名数据';
