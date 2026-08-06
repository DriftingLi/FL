-- 000025_course_sort_order.down.sql
-- 回滚：移除 course.sort_order 列与索引。

BEGIN;

DROP INDEX IF EXISTS idx_course_specialty_level_sort;

ALTER TABLE course
    DROP COLUMN IF EXISTS sort_order;

COMMIT;
