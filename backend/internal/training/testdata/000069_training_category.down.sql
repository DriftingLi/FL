BEGIN;

DROP INDEX IF EXISTS uq_practice_progress_nocred;
ALTER TABLE practice_progress DROP COLUMN IF EXISTS attempt_key;

ALTER TABLE question_practice_record DROP COLUMN IF EXISTS category_id;
ALTER TABLE mock_exam DROP COLUMN IF EXISTS category_id;
ALTER TABLE practice_progress DROP COLUMN IF EXISTS category_id;
ALTER TABLE user_contribution DROP COLUMN IF EXISTS category_id;
ALTER TABLE real_exam_paper DROP COLUMN IF EXISTS category_id;
ALTER TABLE question DROP COLUMN IF EXISTS category_id;
ALTER TABLE course DROP COLUMN IF EXISTS category_id;

DROP TABLE IF EXISTS training_category;

CREATE UNIQUE INDEX uq_practice_progress_nocred ON practice_progress (student_id, practice_mode) WHERE credential_id IS NULL;

COMMIT;
