-- 000006_knowledge_point_category.down.sql
-- 回滚：移除 category 列及其索引
DROP INDEX IF EXISTS idx_kp_category;

ALTER TABLE knowledge_point DROP COLUMN IF EXISTS category;
