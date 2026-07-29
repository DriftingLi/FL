-- 000014_unified_hrwai_users.down.sql
-- 回滚:恢复旧表名,删除 hrwai_users 表
-- 注意:数据无法完整回滚(user_id 已经被改写,无法还原回 student.student_id / valuation_users.id)
-- 仅在紧急情况下使用,建议从备份恢复数据库

BEGIN;

-- 1. 恢复旧表名
ALTER TABLE _deprecated_student RENAME TO student;
ALTER TABLE _deprecated_valuation_users RENAME TO valuation_users;

-- 2. 更新 ai_generation_log.user_type:'hrwai_user' → 'student'
UPDATE ai_generation_log SET user_type = 'student' WHERE user_type = 'hrwai_user';

-- 3. 删除 hrwai_users 表(注意:user_id 已无法还原,引用表数据可能错乱)
DROP TABLE IF EXISTS hrwai_users;

-- 4. 恢复列注释
COMMENT ON COLUMN evaluations.user_id IS '残值评估提交者(valuation_users.id),NULL 表示匿名提交';
COMMENT ON COLUMN battery_evaluations.user_id IS '电池评估提交者(valuation_users.id),NULL 表示历史匿名数据';
COMMENT ON COLUMN ai_chat_sessions.user_id IS 'AI 助手会话归属 valuation_users.id';
COMMENT ON COLUMN ai_user_models.user_id IS '用户自定义模型归属 valuation_users.id';
COMMENT ON COLUMN study_record.student_id IS '学习记录归属 student.student_id';
COMMENT ON COLUMN exam_record.student_id IS '考试记录归属 student.student_id';
COMMENT ON COLUMN exam_participant.student_id IS '考试参与记录归属 student.student_id';
COMMENT ON COLUMN question_practice_record.student_id IS '题库练习归属 student.student_id';
COMMENT ON COLUMN wrong_question.student_id IS '错题记录归属 student.student_id';
COMMENT ON COLUMN mock_exam.student_id IS '模拟考试归属 student.student_id';
COMMENT ON COLUMN practice_progress.student_id IS '练习进度归属 student.student_id';

COMMIT;
