-- 回滚目标证件分区
DROP INDEX IF EXISTS idx_question_credential;
DROP INDEX IF EXISTS idx_course_credential;
DROP INDEX IF EXISTS idx_hrwai_users_current_credential;
ALTER TABLE question DROP COLUMN IF EXISTS credential_id;
ALTER TABLE course DROP COLUMN IF EXISTS credential_id;
ALTER TABLE hrwai_users DROP COLUMN IF EXISTS current_credential_id;
DROP INDEX IF EXISTS idx_credential_sort;
DROP INDEX IF EXISTS idx_credential_category;
DROP INDEX IF EXISTS idx_credential_status;
DROP TABLE IF EXISTS credential;
