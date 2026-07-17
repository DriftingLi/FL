-- 000001_init_baseline.down.sql
-- 回滚 baseline：按依赖反向顺序 DROP 所有 35 张表
-- 注意：只 DROP 最终态存在的表（已被废弃的 brand_types / config_types 表不在最终态，无需 DROP）

-- =====================================================
-- Part 2 反向：残值评估相关表
-- =====================================================
DROP TABLE IF EXISTS battery_cycle_features;
DROP TABLE IF EXISTS battery_evaluations;
DROP TABLE IF EXISTS evaluations;
DROP TABLE IF EXISTS original_prices;
DROP TABLE IF EXISTS series_config_options;
DROP TABLE IF EXISTS engine_types;
DROP TABLE IF EXISTS transmission_types;
DROP TABLE IF EXISTS region_coefficients;
DROP TABLE IF EXISTS condition_ratings;
DROP TABLE IF EXISTS battery_types;
DROP TABLE IF EXISTS mast_heights;
DROP TABLE IF EXISTS mast_types;
DROP TABLE IF EXISTS tonnages;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS vehicle_types;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS coefficient_configs;

-- =====================================================
-- Part 1 反向：学员 / 题库 / 考试相关表
-- =====================================================
DROP TABLE IF EXISTS practice_progress;
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
