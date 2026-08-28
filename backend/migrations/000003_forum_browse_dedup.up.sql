-- 论坛浏览去重：记录用户每日浏览的去重帖子，用于 daily_browse 任务（去重 + 排除自帖）
CREATE TABLE IF NOT EXISTS forum_topic_views (
  user_id    BIGINT      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
  topic_id   BIGINT      NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
  viewed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  view_date  DATE        NOT NULL DEFAULT CURRENT_DATE, -- Asia/Shanghai 业务日由应用层写入
  PRIMARY KEY (user_id, topic_id, view_date)
);
CREATE INDEX IF NOT EXISTS idx_forum_topic_views_user_date ON forum_topic_views(user_id, view_date);
COMMENT ON TABLE forum_topic_views IS '用户帖子浏览去重（每日每帖一次，排除自帖，用于 daily_browse 积分）';
