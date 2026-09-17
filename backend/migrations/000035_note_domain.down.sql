-- 回滚 #1078：note 退回 question_note。
--
-- ⚠️ 独立笔记（question_id IS NULL）在旧形状下无处安放：回滚**必须丢弃**它们，
-- 否则 ADD NOT NULL 会直接失败。被删的是「与题目无关的笔记」这一类数据，不可恢复。

DELETE FROM note WHERE question_id IS NULL;

DROP INDEX IF EXISTS idx_note_user_updated;

ALTER TABLE note ALTER COLUMN question_id SET NOT NULL;

ALTER INDEX idx_note_user RENAME TO idx_question_note_user;

ALTER TABLE note RENAME CONSTRAINT note_user_id_fkey TO question_note_user_id_fkey;
ALTER TABLE note RENAME CONSTRAINT note_question_id_fkey TO question_note_question_id_fkey;
ALTER TABLE note RENAME CONSTRAINT note_question_id_user_id_key TO question_note_question_id_user_id_key;
ALTER TABLE note RENAME CONSTRAINT note_pkey TO question_note_pkey;

ALTER TABLE note RENAME TO question_note;

COMMENT ON TABLE question_note IS NULL;
COMMENT ON COLUMN question_note.question_id IS NULL;
