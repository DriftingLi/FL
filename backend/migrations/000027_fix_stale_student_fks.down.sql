-- 000027_fix_stale_student_fks.down.sql
-- 回滚：恢复指向 _deprecated_student 的外键（与 000014 改名后的状态一致）。
-- 注意：回滚后新注册学员再次写入将恢复外键冲突，仅用于应急还原。

BEGIN;

ALTER TABLE study_record
    DROP CONSTRAINT IF EXISTS study_record_student_id_fkey,
    ADD CONSTRAINT study_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE exam_record
    DROP CONSTRAINT IF EXISTS exam_record_student_id_fkey,
    ADD CONSTRAINT exam_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE exam_participant
    DROP CONSTRAINT IF EXISTS exam_participant_student_id_fkey,
    ADD CONSTRAINT exam_participant_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE question_practice_record
    DROP CONSTRAINT IF EXISTS question_practice_record_student_id_fkey,
    ADD CONSTRAINT question_practice_record_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE wrong_question
    DROP CONSTRAINT IF EXISTS wrong_question_student_id_fkey,
    ADD CONSTRAINT wrong_question_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE mock_exam
    DROP CONSTRAINT IF EXISTS mock_exam_student_id_fkey,
    ADD CONSTRAINT mock_exam_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

ALTER TABLE practice_progress
    DROP CONSTRAINT IF EXISTS practice_progress_student_id_fkey,
    ADD CONSTRAINT practice_progress_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES _deprecated_student(student_id) ON DELETE CASCADE;

COMMIT;
