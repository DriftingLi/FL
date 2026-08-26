DROP INDEX IF EXISTS idx_course_is_featured;
DROP INDEX IF EXISTS idx_course_is_hot;
ALTER TABLE course DROP COLUMN IF EXISTS is_featured;
ALTER TABLE course DROP COLUMN IF EXISTS is_hot;
