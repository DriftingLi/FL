-- 000008_knowledge_point_category_backfill.up.sql
-- 修复：部分数据库 knowledge_point 表 category 字段为空，导致按分类练习 404
-- 根因：000001 迁移文件被标记 applied 后才补充 seed，golang-migrate 不会重跑已应用迁移，
--       旧库中 knowledge_point 的 category 列保持为 NULL，使 GetCategoryQuestions 返回 404。
-- 本迁移幂等回填 6 条内置知识点的 category（与 000001 seed 保持一致）。
UPDATE knowledge_point SET category = 'CATEGORY_01' WHERE id = 1 AND (category IS NULL OR category = '');
UPDATE knowledge_point SET category = 'CATEGORY_04' WHERE id = 2 AND (category IS NULL OR category = '');
UPDATE knowledge_point SET category = 'CATEGORY_02' WHERE id = 3 AND (category IS NULL OR category = '');
UPDATE knowledge_point SET category = 'CATEGORY_03' WHERE id = 4 AND (category IS NULL OR category = '');
UPDATE knowledge_point SET category = 'CATEGORY_03' WHERE id = 5 AND (category IS NULL OR category = '');
UPDATE knowledge_point SET category = 'CATEGORY_04' WHERE id = 6 AND (category IS NULL OR category = '');
