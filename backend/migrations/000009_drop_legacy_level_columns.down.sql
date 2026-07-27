-- 000009_drop_legacy_level_columns.down.sql
-- 回滚：恢复被删除的 level 列（数据不可恢复，仅恢复 schema）
-- 注意：恢复后所有记录的 level 字段将为 NULL，需手动重新填充。

ALTER TABLE mock_exam        ADD COLUMN IF NOT EXISTS level VARCHAR(20);
CREATE INDEX IF NOT EXISTS idx_mock_exam_level ON mock_exam (level);

ALTER TABLE exam_session     ADD COLUMN IF NOT EXISTS level VARCHAR(20);
ALTER TABLE knowledge_point  ADD COLUMN IF NOT EXISTS level VARCHAR(20);
ALTER TABLE question         ADD COLUMN IF NOT EXISTS level VARCHAR(20);
ALTER TABLE student          ADD COLUMN IF NOT EXISTS level VARCHAR(20) DEFAULT 'beginner';
