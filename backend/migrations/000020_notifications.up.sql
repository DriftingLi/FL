-- 000020_notifications.up.sql
-- 站内信通知表（P0 通知基础设施，先仅站内信，后续可扩展邮件/企业微信等渠道）

BEGIN;

CREATE TABLE IF NOT EXISTS notifications (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    INTEGER     NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    type       VARCHAR(50) NOT NULL DEFAULT 'system',
    title      VARCHAR(255) NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    link       VARCHAR(500) NOT NULL DEFAULT '',
    is_read    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id) WHERE is_read = FALSE;

COMMIT;
