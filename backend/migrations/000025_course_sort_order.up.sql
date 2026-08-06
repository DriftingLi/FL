-- 000025_course_sort_order.up.sql
-- 培训目录扩展（LH-11 复测修复）：
--  course 表新增 sort_order，支撑管理端目录树课程拖拽排序；
--  排序在所属（specialty_id, level_id）层级内生效，课程列表/目录树按 sort_order 升序。
-- 幂等写法，可重复执行、可回滚，不破坏既有数据。

BEGIN;

ALTER TABLE course
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

-- 目录树高频路径：同方向同等级下按 sort_order 排序
CREATE INDEX IF NOT EXISTS idx_course_specialty_level_sort
    ON course (specialty_id, level_id, sort_order);

COMMENT ON COLUMN course.sort_order IS '课程排序值（所属专业方向+课程等级层级内生效，越小越靠前）';

COMMIT;
