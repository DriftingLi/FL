-- ============================================================
-- 000002 课程分类体系收敛：删除旧分类列与知识点树，存量题目按显式映射打标签
-- 决策：课程分类体系收敛（category/知识点树退役 → 方向×等级 + 题库标签）。
-- 代码层退役已完成（#51 课程、#52 题库/练习），本迁移只做 contract：
-- 数据回填 + 物理删除列/表。
-- ============================================================

-- 1. 存量题目按显式映射回填标签（知识点 → 标签）
--    叉车结构与基础(1) → 结构；液压与动力系统(2) → 液压；
--    安全操作规程(3) → 法规；故障诊断与排除(6) → 故障诊断。
--    其余知识点（日常检查与保养(4)、货叉作业技能(5) 等）保持未打标，
--    由管理员在题库标签页批量补。
INSERT INTO question_tag_relation (question_id, tag_id, created_at)
SELECT q.id, t.id, now()
FROM question q
JOIN knowledge_point kp ON kp.id = q.knowledge_point_id
JOIN question_tag t ON t.code IN ('structure', 'hydraulic', 'regulation', 'fault_diagnosis')
WHERE (kp.id = 1 AND t.code = 'structure')
   OR (kp.id = 2 AND t.code = 'hydraulic')
   OR (kp.id = 3 AND t.code = 'regulation')
   OR (kp.id = 6 AND t.code = 'fault_diagnosis')
ON CONFLICT (question_id, tag_id) DO NOTHING;

-- 2. 删除 question.knowledge_point_id（连同外键与索引）
DROP INDEX IF EXISTS idx_question_knowledge_point;
ALTER TABLE question DROP COLUMN IF EXISTS knowledge_point_id;

-- 3. 删除 knowledge_point 表（自引用外键与索引随表删除）
DROP TABLE IF EXISTS knowledge_point;

-- 4. 删除 course.category（连同索引与注释）
DROP INDEX IF EXISTS idx_course_category;
ALTER TABLE course DROP COLUMN IF EXISTS category;
