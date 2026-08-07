-- ============================================================
-- 000002 down：恢复 course.category、knowledge_point 表与
-- question.knowledge_point_id，并清理迁移按显式映射回填的标签关联。
-- 注意：不恢复种子数据（回滚只保证结构可逆，数据由备份/重新部署恢复）。
-- ============================================================

-- 1. 恢复 course.category（旧数据回填 CATEGORY_01 占位）
ALTER TABLE course ADD COLUMN IF NOT EXISTS category VARCHAR(20);
UPDATE course SET category = 'CATEGORY_01' WHERE category IS NULL;
ALTER TABLE course ALTER COLUMN category SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_course_category ON course (category);
COMMENT ON COLUMN course.category IS '分类：CATEGORY_01-基础理论, CATEGORY_02-安全规范, CATEGORY_03-实操技能, CATEGORY_04-进阶提升';

-- 2. 恢复 knowledge_point 表
CREATE TABLE knowledge_point (
    id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    category    VARCHAR(32),
    parent_id   INT          REFERENCES knowledge_point(id) ON DELETE SET NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_kp_category ON knowledge_point (category);
CREATE INDEX idx_kp_parent   ON knowledge_point (parent_id);
COMMENT ON TABLE  knowledge_point IS '知识点表';
COMMENT ON COLUMN knowledge_point.category IS '课程分类：CATEGORY_01-基础理论, CATEGORY_02-安全规范, CATEGORY_03-实操技能, CATEGORY_04-进阶提升';

-- 3. 恢复 question.knowledge_point_id（连同外键与索引）
ALTER TABLE question ADD COLUMN IF NOT EXISTS knowledge_point_id INT REFERENCES knowledge_point(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_question_knowledge_point ON question (knowledge_point_id);

-- 4. 清理迁移按显式映射回填的标签关联。
--    回填发生在 knowledge_point_id 列删除之前，down 时该列已不存在，
--    无法再按知识点识别回填行，故按映射标签范围（structure/hydraulic/
--    regulation/fault_diagnosis）清理。
--    注意：若回滚前管理员已对这些标签人工打标，回滚后需人工核对恢复。
DELETE FROM question_tag_relation qtr
USING question_tag t
WHERE qtr.tag_id = t.id
  AND t.code IN ('structure', 'hydraulic', 'regulation', 'fault_diagnosis');
