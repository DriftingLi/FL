DROP TABLE IF EXISTS real_exam_paper_question;
DROP TABLE IF EXISTS real_exam_paper;
ALTER TABLE mock_exam DROP COLUMN IF EXISTS paper_id;
ALTER TABLE question_tag DROP COLUMN IF EXISTS is_source_tag;
