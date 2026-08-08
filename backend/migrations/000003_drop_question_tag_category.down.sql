-- ============================================================
-- 000003 down：恢复 question_tag.category 列。
-- 注意：不恢复旧值（列删除后数据已不可恢复），回滚只保证结构可逆，
-- 与 000002 down 的回滚约定一致（数据由备份/重新部署恢复）。
-- ============================================================

ALTER TABLE question_tag ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT '';
COMMENT ON COLUMN question_tag.category IS '考点模块分类';
