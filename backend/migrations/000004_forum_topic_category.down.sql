-- 回滚 #364：逆序删索引与列（约束随列一起消失）
DROP INDEX IF EXISTS idx_forum_topics_category_hot;
DROP INDEX IF EXISTS idx_forum_topics_category_created;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS category;
