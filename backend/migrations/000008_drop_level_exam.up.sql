-- 下线定级考试：移除考试场次、参与记录与答题记录三张表（spec #284，ticket #285）
DROP TABLE IF EXISTS exam_answer CASCADE;
DROP TABLE IF EXISTS exam_participant CASCADE;
DROP TABLE IF EXISTS exam_session CASCADE;
