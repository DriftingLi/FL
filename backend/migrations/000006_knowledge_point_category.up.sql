-- 000006_knowledge_point_category.up.sql
-- 修复：部分数据库的 knowledge_point 表缺少 category 列（SQLSTATE 42703）
-- 根因：旧版本数据库 schema 与 000001 迁移脚本不一致，knowledge_point.category 未创建
-- 本迁移幂等补齐 category 列、索引与注释，保证 GetStats/GetCategories 等统计查询正常
ALTER TABLE knowledge_point
    ADD COLUMN IF NOT EXISTS category VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_kp_category ON knowledge_point (category);

COMMENT ON COLUMN knowledge_point.category IS '课程分类：CATEGORY_01-基础理论, CATEGORY_02-安全规范, CATEGORY_03-实操技能, CATEGORY_04-进阶提升';
