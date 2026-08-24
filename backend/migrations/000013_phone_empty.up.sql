-- 手机号为空时不再生成占位值，允许空字符串多行并存
-- 原 phone 为全局 UNIQUE，空串多用户会冲突；改为部分唯一索引（仅非空手机号唯一）
ALTER TABLE hrwai_users DROP CONSTRAINT IF EXISTS hrwai_users_phone_key;
DROP INDEX IF EXISTS hrwai_users_phone_key;
-- 兼容旧库：若存在名为 hrwai_users_phone_idx 的索引也一并清理
DROP INDEX IF EXISTS idx_hrwai_users_phone_unique;

-- 创建部分唯一索引：仅对非空手机号做唯一约束，空串可重复
CREATE UNIQUE INDEX IF NOT EXISTS idx_hrwai_users_phone_unique
    ON hrwai_users (phone)
    WHERE phone <> '';

-- 将存量占位手机号（email_/wxp_/deleted__sentinel）置空，满足“非手机号注册时为空”
UPDATE hrwai_users SET phone = '' WHERE phone LIKE 'email\_%' ESCAPE '\' OR phone LIKE 'wxp\_%' ESCAPE '\' OR phone = 'deleted__sentinel' OR phone LIKE 'test\_%' ESCAPE '\';
