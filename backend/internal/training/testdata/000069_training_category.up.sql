BEGIN;

CREATE TABLE IF NOT EXISTS training_category (
    id                   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    legacy_credential_id INT UNIQUE,
    name                 VARCHAR(100) NOT NULL,
    sort_order           INT NOT NULL DEFAULT 0,
    status               SMALLINT NOT NULL DEFAULT 1,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO training_category (legacy_credential_id, name, sort_order, status)
SELECT id, name, sort_order, status
FROM credential
ORDER BY id
ON CONFLICT (legacy_credential_id) DO NOTHING;

ALTER TABLE course ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE question ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE real_exam_paper ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE user_contribution ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE practice_progress ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE practice_progress ADD COLUMN IF NOT EXISTS attempt_key VARCHAR(36);
ALTER TABLE mock_exam ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);
ALTER TABLE question_practice_record ADD COLUMN IF NOT EXISTS category_id INT REFERENCES training_category(id);

UPDATE course AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE question AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE real_exam_paper AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE user_contribution AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE practice_progress AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE mock_exam AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

UPDATE question_practice_record AS item SET category_id = category.id
FROM training_category AS category
WHERE item.category_id IS NULL AND item.credential_id = category.legacy_credential_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_practice_progress_category ON practice_progress (student_id, practice_mode, category_id) WHERE category_id IS NOT NULL;
DROP INDEX IF EXISTS uq_practice_progress_nocred;
CREATE UNIQUE INDEX uq_practice_progress_nocred ON practice_progress (student_id, practice_mode) WHERE credential_id IS NULL AND category_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_real_exam_paper_category_source ON real_exam_paper (category_id, source_ref) WHERE category_id IS NOT NULL;

COMMIT;
