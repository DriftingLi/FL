-- #366 采纳状态机与积分直记：主题获得采纳落点（每帖只发一次分）。
--
-- 主题表新增 accepted_reply_id（可空，外键至 forum_replies.id，SET NULL）与 solved_at（可空）。
-- 回复表不加任何列——某条回答是否被采纳由主题上的 accepted_reply_id 推导。
-- 状态迁移与积分直记的幂等由 CAS 守住：UPDATE ... WHERE accepted_reply_id IS NULL。

ALTER TABLE forum_topics
    ADD COLUMN accepted_reply_id BIGINT REFERENCES forum_replies(id) ON DELETE SET NULL;

ALTER TABLE forum_topics
    ADD COLUMN solved_at TIMESTAMPTZ;

COMMENT ON COLUMN forum_topics.accepted_reply_id IS '被采纳的回复 ID（可空，仅问答帖有效，NULL 表示未解决）';
COMMENT ON COLUMN forum_topics.solved_at IS '问答帖解决时间（采纳时写入，取消时置空）';

CREATE INDEX IF NOT EXISTS idx_forum_topics_accepted_reply
    ON forum_topics (accepted_reply_id) WHERE accepted_reply_id IS NOT NULL;
