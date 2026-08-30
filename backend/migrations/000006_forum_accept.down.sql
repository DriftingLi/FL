-- 回滚 #366
DROP INDEX IF EXISTS idx_forum_topics_accepted_reply;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS solved_at;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS accepted_reply_id;
