-- 000022_email_auth.up.sql
-- 邮箱注册/登录：hrwai_users.email 唯一（非空邮箱不可重复注册）

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS idx_hrwai_users_email_unique
    ON hrwai_users (email)
    WHERE email <> '';

COMMIT;
