-- 000024_training_catalog.down.sql
-- 回滚：删除培训目录扩展相关表、course 新列与索引。
-- 只删除本迁移新增的对象；既有数据不受影响（course 原列保留）。

BEGIN;

-- course 表新列与索引
DROP INDEX IF EXISTS idx_course_cert_template;
DROP INDEX IF EXISTS idx_course_level;
DROP INDEX IF EXISTS idx_course_specialty;
DROP INDEX IF EXISTS idx_course_status_specialty_level;

ALTER TABLE course
    DROP COLUMN IF EXISTS certificate_template_id,
    DROP COLUMN IF EXISTS practice_hours,
    DROP COLUMN IF EXISTS theory_hours,
    DROP COLUMN IF EXISTS level_id,
    DROP COLUMN IF EXISTS specialty_id;

-- 题目-标签关联
DROP TABLE IF EXISTS question_tag_relation;

-- 题库标签
DROP TABLE IF EXISTS question_tag;

-- 前置课程
DROP TABLE IF EXISTS course_prerequisite;

-- 证书模板
DROP TABLE IF EXISTS certificate_template;

-- 课程等级
DROP TABLE IF EXISTS course_level;

-- 专业方向
DROP TABLE IF EXISTS specialty;

COMMIT;
