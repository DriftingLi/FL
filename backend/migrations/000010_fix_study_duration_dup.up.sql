-- 000010_fix_study_duration_dup.up.sql
-- 修复：UpdateStudyProgress 历史版本在学生学习新章节时重复累加 duration：
--   1) duration 加到 First() 取出的"主记录"（按 record_id 升序的第一条）上
--   2) 新创建的章节记录又带 StudyDuration=duration（同一份 duration 被存两次）
--   导致 SUM(study_duration) 偏大，管理员端"总学习时长"虚高。
-- 代码层修复（course_service.go）已将新章节记录的 study_duration 改为 0；
-- 本迁移清理历史数据：每个 (student_id, course_id) 分组下仅保留主记录的 study_duration，
-- 其余章节占位记录的 study_duration 清零（这些记录仅用于统计 completedChapters）。
-- 注意：down 迁移无法恢复原始重复数据（已无意义），仅做占位。

-- 将每个 (student_id, course_id) 分组下非主记录的 study_duration 清零
UPDATE study_record
SET study_duration = 0
WHERE record_id NOT IN (
    SELECT min_id FROM (
        SELECT MIN(record_id) AS min_id
        FROM study_record
        GROUP BY student_id, course_id
    ) AS main_records
);
