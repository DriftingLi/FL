-- 论坛增强：每日打卡（日历+排行榜）+ 评论点赞（spec #268 / tickets #269）
-- forum_checkin：每日打卡记录，PK(user_id, check_date) 保证自然日幂等
CREATE TABLE IF NOT EXISTS forum_checkin (
    user_id INT NOT NULL,
    check_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pk_forum_checkin PRIMARY KEY (user_id, check_date)
);
CREATE INDEX IF NOT EXISTS idx_forum_checkin_date ON forum_checkin (check_date);
CREATE INDEX IF NOT EXISTS idx_forum_checkin_user_date ON forum_checkin (user_id, check_date);

-- forum_reply_like：评论点赞，与 forum_topic_like 同构
CREATE TABLE IF NOT EXISTS forum_reply_like (
    id BIGSERIAL PRIMARY KEY,
    reply_id BIGINT NOT NULL REFERENCES forum_replies(id) ON DELETE CASCADE,
    user_id INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_forum_reply_like UNIQUE (reply_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_forum_reply_like_user ON forum_reply_like (user_id);
CREATE INDEX IF NOT EXISTS idx_forum_reply_like_reply ON forum_reply_like (reply_id);
