DROP INDEX IF EXISTS idx_forum_topics_hot_sort;
DROP INDEX IF EXISTS idx_forum_replies_likes_count;
DROP INDEX IF EXISTS idx_forum_topics_likes_count;
ALTER TABLE forum_replies DROP COLUMN IF EXISTS likes_count;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS likes_count;
