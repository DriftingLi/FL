-- 000023_auth_framework.up.sql
-- 多登录方式框架：微信身份字段（账号/手机号/邮箱各自独立，账号注册时随机生成）

BEGIN;

ALTER TABLE hrwai_users
    ADD COLUMN IF NOT EXISTS wechat_openid  VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS wechat_unionid VARCHAR(128) NOT NULL DEFAULT '';

-- 同一微信 openid 只能绑定一个账号（非空时唯一）
CREATE UNIQUE INDEX IF NOT EXISTS idx_hrwai_users_wechat_openid_unique
    ON hrwai_users (wechat_openid)
    WHERE wechat_openid <> '';

COMMIT;
