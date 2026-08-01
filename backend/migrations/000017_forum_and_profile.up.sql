-- 000017_forum_and_profile.up.sql
-- 学员端论坛（大论坛 + 课程讨论区）与用户资料扩展（昵称 / 头像）

BEGIN;

-- 1. hrwai_users 增加昵称与头像字段
ALTER TABLE hrwai_users
    ADD COLUMN IF NOT EXISTS nickname VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';

-- 2. 论坛主题表
-- course_id 为 NULL 表示"大论坛"（综合讨论）；非 NULL 表示该课程下的讨论区
CREATE TABLE IF NOT EXISTS forum_topics (
    id            BIGSERIAL     PRIMARY KEY,
    course_id     INTEGER       REFERENCES course(course_id) ON DELETE CASCADE,
    user_id       INTEGER       NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    title         VARCHAR(200)  NOT NULL,
    content       TEXT          NOT NULL,
    view_count    INTEGER       NOT NULL DEFAULT 0,
    reply_count   INTEGER       NOT NULL DEFAULT 0,
    last_reply_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_forum_topics_course
    ON forum_topics (course_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_topics_created
    ON forum_topics (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_topics_user
    ON forum_topics (user_id);

-- 3. 论坛回复表
CREATE TABLE IF NOT EXISTS forum_replies (
    id         BIGSERIAL   PRIMARY KEY,
    topic_id   BIGINT      NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    user_id    INTEGER     NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_forum_replies_topic
    ON forum_replies (topic_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_forum_replies_user
    ON forum_replies (user_id);

COMMIT;
