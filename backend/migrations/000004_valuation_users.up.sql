-- 000004_valuation_users.up.sql
-- 残值评估模块独立用户表（与培训 Student 表完全独立，账号互不复用）
CREATE TABLE valuation_users (
    id            SERIAL         PRIMARY KEY,
    username      VARCHAR(50)    NOT NULL,
    password      VARCHAR(255)   NOT NULL,
    name          VARCHAR(100)   NOT NULL,
    phone         VARCHAR(20)    NOT NULL,
    email         VARCHAR(100)   DEFAULT '',
    company       VARCHAR(200)   DEFAULT '',
    status        SMALLINT       NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_valuation_users_username ON valuation_users (username);
CREATE UNIQUE INDEX idx_valuation_users_phone    ON valuation_users (phone);
COMMENT ON TABLE  valuation_users           IS '残值评估模块独立用户表（与培训 Student 表独立）';
COMMENT ON COLUMN valuation_users.id        IS '主键';
COMMENT ON COLUMN valuation_users.username  IS '用户名（注册时由手机号自动生成）';
COMMENT ON COLUMN valuation_users.password  IS '密码（bcrypt hash）';
COMMENT ON COLUMN valuation_users.name      IS '姓名';
COMMENT ON COLUMN valuation_users.phone     IS '手机号（唯一）';
COMMENT ON COLUMN valuation_users.email     IS '邮箱（可选）';
COMMENT ON COLUMN valuation_users.company   IS '公司（可选）';
COMMENT ON COLUMN valuation_users.status    IS '状态：1-启用，0-禁用';
COMMENT ON COLUMN valuation_users.created_at IS '创建时间';
