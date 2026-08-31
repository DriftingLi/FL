-- #414 回滚：恢复 (student_id, practice_mode) 唯一约束并删除证件列
DROP INDEX IF EXISTS uq_practice_progress_cred;
DROP INDEX IF EXISTS uq_practice_progress_nocred;

ALTER TABLE practice_progress ADD CONSTRAINT practice_progress_student_id_practice_mode_key UNIQUE (student_id, practice_mode);

ALTER TABLE practice_progress DROP COLUMN IF EXISTS credential_id;
