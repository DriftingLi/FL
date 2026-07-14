-- 回滚初始化迁移：按依赖逆序删除所有表

DROP TABLE IF EXISTS async_task;
DROP TABLE IF EXISTS mock_exam;
DROP TABLE IF EXISTS wrong_question;
DROP TABLE IF EXISTS question_practice_record;
DROP TABLE IF EXISTS exam_answer;
DROP TABLE IF EXISTS exam_participant;
DROP TABLE IF EXISTS exam_session;
DROP TABLE IF EXISTS question;
DROP TABLE IF EXISTS knowledge_point;
DROP TABLE IF EXISTS ai_generation_log;
DROP TABLE IF EXISTS exam_record;
DROP TABLE IF EXISTS study_record;
DROP TABLE IF EXISTS chapter_file;
DROP TABLE IF EXISTS chapter;
DROP TABLE IF EXISTS course;
DROP TABLE IF EXISTS tutor;
DROP TABLE IF EXISTS admin;
DROP TABLE IF EXISTS student;
