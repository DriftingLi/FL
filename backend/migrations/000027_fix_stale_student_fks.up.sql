-- 000027_fix_stale_student_fks.up.sql
-- 修复：统一账号迁移（000014）后，7 张业务表的 student_id 外键仍指向被改名
-- 的旧表 _deprecated_student，导致新注册学员（仅存在于 hrwai_users）刷题提交、
-- 保存练习进度、错题入库、学习记录上报等写入全部命中外键冲突（SQLSTATE 23503）。
-- 本迁移将这 7 个外键改为指向统一账号表 hrwai_users(id)，与论坛等新表一致。

BEGIN;

ALTER TABLE study_record
    DROP CONSTRAINT IF EXISTS study_record_student_id_fkey,
    ADD CONSTRAINT study_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE exam_record
    DROP CONSTRAINT IF EXISTS exam_record_student_id_fkey,
    ADD CONSTRAINT exam_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE exam_participant
    DROP CONSTRAINT IF EXISTS exam_participant_student_id_fkey,
    ADD CONSTRAINT exam_participant_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE question_practice_record
    DROP CONSTRAINT IF EXISTS question_practice_record_student_id_fkey,
    ADD CONSTRAINT question_practice_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE wrong_question
    DROP CONSTRAINT IF EXISTS wrong_question_student_id_fkey,
    ADD CONSTRAINT wrong_question_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE mock_exam
    DROP CONSTRAINT IF EXISTS mock_exam_student_id_fkey,
    ADD CONSTRAINT mock_exam_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

ALTER TABLE practice_progress
    DROP CONSTRAINT IF EXISTS practice_progress_student_id_fkey,
    ADD CONSTRAINT practice_progress_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;

COMMIT;
