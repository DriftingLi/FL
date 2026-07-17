-- 000002_question_reject_reason.down.sql
ALTER TABLE question DROP COLUMN IF EXISTS reject_reason;
