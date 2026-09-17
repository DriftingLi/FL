-- 顺序即正确性（#1099 的 down 回滚验证首次真跑到这里才发现）：
-- mock_exam.paper_id 的外键指向 real_exam_paper，必须**先删这个列**（连带删掉约束），
-- 再删表；否则 DROP TABLE real_exam_paper 会被 mock_exam_paper_id_fkey 挡住：
--   cannot drop table real_exam_paper because other objects depend on it
ALTER TABLE mock_exam DROP COLUMN IF EXISTS paper_id;

DROP TABLE IF EXISTS real_exam_paper_question;
DROP TABLE IF EXISTS real_exam_paper;

ALTER TABLE question_tag DROP COLUMN IF EXISTS is_source_tag;
