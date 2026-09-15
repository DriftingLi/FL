-- #1007 回滚：回填数据不可逆（已挂分区的历史行不还原为 NULL —— 还原会让读面口径与实际作答上下文脱钩）。
DROP FUNCTION IF EXISTS backfill_qpr_credential();
DROP INDEX IF EXISTS idx_qpr_student_credential;
ALTER TABLE question_practice_record DROP COLUMN IF EXISTS credential_id;
