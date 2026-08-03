-- 000023_auth_framework.down.sql
BEGIN;

DROP INDEX IF EXISTS idx_hrwai_users_wechat_openid_unique;
ALTER TABLE hrwai_users
    DROP COLUMN IF EXISTS wechat_openid,
    DROP COLUMN IF EXISTS wechat_unionid;

COMMIT;
