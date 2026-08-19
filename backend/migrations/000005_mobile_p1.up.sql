-- ADR-0018：移动端 P1 通用能力——论坛互动（点赞/举报）与通用收藏。
-- 全局搜索与学习资料聚合为纯查询（无新表）。
CREATE TABLE IF NOT EXISTS forum_topic_like (
    id BIGSERIAL PRIMARY KEY,
    topic_id BIGINT NOT NULL,
    user_id INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_forum_topic_like UNIQUE (topic_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_forum_topic_like_user ON forum_topic_like (user_id);

CREATE TABLE IF NOT EXISTS favorite (
    favorite_id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    target_type VARCHAR(20) NOT NULL,
    target_id INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_favorite_target UNIQUE (user_id, target_type, target_id)
);
CREATE INDEX IF NOT EXISTS idx_favorite_user ON favorite (user_id);

CREATE TABLE IF NOT EXISTS forum_report (
    id BIGSERIAL PRIMARY KEY,
    reporter_id INT NOT NULL,
    topic_id BIGINT,
    reply_id BIGINT,
    reason VARCHAR(500) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forum_report_status ON forum_report (status);
