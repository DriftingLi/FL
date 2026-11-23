BEGIN;

ALTER TABLE user_contribution ALTER COLUMN credential_id SET NOT NULL;
ALTER TABLE real_exam_paper ALTER COLUMN credential_id SET NOT NULL;

COMMIT;
