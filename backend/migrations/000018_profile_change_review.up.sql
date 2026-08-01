-- 000018_profile_change_review.up.sql
-- 用户资料（昵称/头像）修改审核：修改先进入待审队列，管理员通过后生效

BEGIN;

CREATE TABLE IF NOT EXISTS profile_change_requests (
    id            BIGSERIAL     PRIMARY KEY,
    user_id       INTEGER       NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    field_type    VARCHAR(20)   NOT NULL, -- 'nickname' | 'avatar'
    old_value     TEXT          NOT NULL DEFAULT '',
    new_value     TEXT          NOT NULL,
    status        VARCHAR(20)   NOT NULL DEFAULT 'pending', -- pending / approved / rejected
    reject_reason TEXT          NOT NULL DEFAULT '',
    reviewed_by   INTEGER,                                    -- 审核管理员（admin.id），未审核为 NULL
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profile_change_user
    ON profile_change_requests (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_profile_change_status
    ON profile_change_requests (status, created_at DESC);

COMMIT;
