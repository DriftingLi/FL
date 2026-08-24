-- 回滚：恢复全局唯一约束（注意：若已存在多空串会回滚失败，需先清理）
DROP INDEX IF EXISTS idx_hrwai_users_phone_unique;
-- 尝试恢复全局唯一（若存在重复空串会报错，属预期）
CREATE UNIQUE INDEX IF NOT EXISTS hrwai_users_phone_key ON hrwai_users (phone);
