-- 000009_drop_legacy_level_columns.up.sql
-- 修复：exam_session/knowledge_point/mock_exam/question/student 表存在遗留 level 列（NOT NULL 无默认），
--       但 Go 模型和 000001 迁移文件均未定义该字段，导致插入时报
--       "null value in column \"level\" violates not-null constraint" (SQLSTATE 23502)
-- 根因：旧版 schema 中曾使用 level 字段表示难度（beginner/intermediate/advanced），
--       后续代码重构移除了该字段的使用，但数据库列未同步清理。
--       所有 Go 代码均不读写 level 列（已通过 grep 确认），保留只会引发约束错误。
-- 本迁移幂等删除 5 张表的 level 列及相关索引，使 schema 与模型/迁移文件对齐。
-- 历史数据丢失但无影响：代码未使用 level 字段。

-- 删除 mock_exam 上的 level 索引（如存在）再删列
DROP INDEX IF EXISTS idx_mock_exam_level;
ALTER TABLE mock_exam        DROP COLUMN IF EXISTS level;

-- exam_session
ALTER TABLE exam_session     DROP COLUMN IF EXISTS level;

-- knowledge_point
ALTER TABLE knowledge_point  DROP COLUMN IF EXISTS level;

-- question
ALTER TABLE question         DROP COLUMN IF EXISTS level;

-- student（level 列有默认 'beginner'，但同样未被代码使用）
ALTER TABLE student          DROP COLUMN IF EXISTS level;
