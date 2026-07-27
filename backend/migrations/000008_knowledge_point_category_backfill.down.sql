-- 000008_knowledge_point_category_backfill.down.sql
-- 回滚：将 6 条内置知识点的 category 置空（恢复到 000008 应用前的状态）
UPDATE knowledge_point SET category = NULL WHERE id IN (1, 2, 3, 4, 5, 6);
