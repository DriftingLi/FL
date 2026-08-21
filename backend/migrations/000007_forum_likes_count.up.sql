-- 论坛点赞反范式化：帖子/回复表增加 likes_count 计数列并回填存量（spec #279）
ALTER TABLE forum_topics ADD COLUMN IF NOT EXISTS likes_count INT NOT NULL DEFAULT 0;
ALTER TABLE forum_replies ADD COLUMN IF NOT EXISTS likes_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_forum_topics_likes_count ON forum_topics (likes_count);
CREATE INDEX IF NOT EXISTS idx_forum_replies_likes_count ON forum_replies (likes_count);
CREATE INDEX IF NOT EXISTS idx_forum_topics_hot_sort ON forum_topics (likes_count DESC, reply_count DESC, view_count DESC, id DESC);

-- 存量回填（幂等：重复执行以最新 COUNT 覆盖）
UPDATE forum_topics SET likes_count = sub.cnt
FROM (SELECT topic_id, COUNT(*)::int AS cnt FROM forum_topic_like GROUP BY topic_id) AS sub
WHERE sub.topic_id = forum_topics.id;

UPDATE forum_replies SET likes_count = sub.cnt
FROM (SELECT reply_id, COUNT(*)::int AS cnt FROM forum_reply_like GROUP BY reply_id) AS sub
WHERE sub.reply_id = forum_replies.id;
