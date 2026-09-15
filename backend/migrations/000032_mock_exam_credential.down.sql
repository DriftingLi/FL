-- #1003 回滚：回填数据不可逆（已挂分区的历史行不还原为 NULL，还原无意义且会让历史读面丢记录）。
-- 仅删函数、索引与列。
DROP FUNCTION IF EXISTS backfill_mock_exam_credential();
DROP INDEX IF EXISTS idx_mock_exam_student_credential;
ALTER TABLE mock_exam DROP COLUMN IF EXISTS credential_id;
