-- 000007_practice_progress.up.sql
-- 修复：部分数据库缺少 practice_progress 表（SQLSTATE 42P01）
-- 根因：旧版本数据库 schema 与 000001 迁移脚本不一致，practice_progress 表未创建
-- 本迁移幂等创建 practice_progress 表、索引与注释，保证顺序练习断点续练功能正常
CREATE TABLE IF NOT EXISTS practice_progress (
    id            INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id    INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    practice_mode VARCHAR(32)  NOT NULL,
    question_ids  JSONB        NOT NULL,
    current_index INT          NOT NULL DEFAULT 0,
    total         INT          NOT NULL DEFAULT 0,
    answers_state JSONB        NOT NULL DEFAULT '{}'::jsonb,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (student_id, practice_mode)
);

CREATE INDEX IF NOT EXISTS idx_practice_progress_student ON practice_progress (student_id);

COMMENT ON TABLE practice_progress IS '练习进度表（断点续练游标 + 答题状态持久化）';
COMMENT ON COLUMN practice_progress.answers_state IS '每题作答状态 JSONB，key 为 question_id，value 含 user_answer/is_correct/correct_answer/explanation 等';
