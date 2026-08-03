-- 000021_audit_logs.up.sql
-- 管理员/讲师关键操作审计日志（P0 合规能力）

BEGIN;

CREATE TABLE IF NOT EXISTS audit_logs (
    id         BIGSERIAL   PRIMARY KEY,
    actor_id   INTEGER     NOT NULL,
    actor_role VARCHAR(50) NOT NULL,
    actor_name VARCHAR(100) NOT NULL DEFAULT '',
    action     VARCHAR(200) NOT NULL,
    path       VARCHAR(500) NOT NULL,
    method     VARCHAR(10)  NOT NULL,
    request_id VARCHAR(64)  NOT NULL DEFAULT '',
    ip         VARCHAR(64)  NOT NULL DEFAULT '',
    status     INTEGER     NOT NULL DEFAULT 0,
    detail     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created
    ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor
    ON audit_logs (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_role
    ON audit_logs (actor_role, created_at DESC);

COMMIT;
