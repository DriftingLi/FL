-- #370 企业招聘者账号体系（第四角色 + cookie 隔离）
-- 独立表 recruiter_users，不进 hrwai_users；邀约制，管理员创建，企业信息必填.

CREATE TABLE recruiter_users (
    id              SERIAL         PRIMARY KEY,
    username        VARCHAR(50)    NOT NULL UNIQUE,
    password        VARCHAR(255)   NOT NULL,
    company_name    VARCHAR(200)   NOT NULL,
    credit_code     VARCHAR(50)    NOT NULL,
    business_scope  VARCHAR(255)   NOT NULL,
    contact_name    VARCHAR(50)    NOT NULL,
    contact_phone   VARCHAR(50)    NOT NULL,
    contact_email   VARCHAR(255)   NOT NULL,
    status          SMALLINT       NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE recruiter_users IS '企业招聘者账号（第四角色，独立表，邀约制）';
COMMENT ON COLUMN recruiter_users.username IS '登录账号（唯一）';
COMMENT ON COLUMN recruiter_users.company_name IS '企业名称';
COMMENT ON COLUMN recruiter_users.credit_code IS '统一社会信用代码';
COMMENT ON COLUMN recruiter_users.business_scope IS '主营';
COMMENT ON COLUMN recruiter_users.contact_name IS '对外联系人姓名';
COMMENT ON COLUMN recruiter_users.contact_phone IS '联系电话';
COMMENT ON COLUMN recruiter_users.contact_email IS '联系邮箱';
COMMENT ON COLUMN recruiter_users.status IS '状态：1-正常，0-禁用';
