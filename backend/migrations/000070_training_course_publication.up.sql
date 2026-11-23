DO $$
DECLARE
    missing_ids INT[];
BEGIN
    SELECT array_agg(course_id ORDER BY course_id) INTO missing_ids
    FROM course
    WHERE status = 1
      AND specialty_id IS NOT NULL
      AND level_id IS NOT NULL
      AND category_id IS NULL;

    IF missing_ids IS NOT NULL THEN
        RAISE EXCEPTION 'Previously visible courses require a classification before cutover: %', missing_ids;
    END IF;

    IF EXISTS (
        SELECT 1 FROM credential c
        LEFT JOIN training_category t ON t.legacy_credential_id = c.id
        WHERE t.id IS NULL
    ) THEN
        RAISE EXCEPTION 'Legacy credentials without a unique training category mapping';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM (
            SELECT credential_id, category_id FROM course
            UNION ALL SELECT credential_id, category_id FROM question
            UNION ALL SELECT credential_id, category_id FROM real_exam_paper
            UNION ALL SELECT credential_id, category_id FROM user_contribution
            UNION ALL SELECT credential_id, category_id FROM practice_progress
            UNION ALL SELECT credential_id, category_id FROM mock_exam
            UNION ALL SELECT credential_id, category_id FROM question_practice_record
        ) AS item
        LEFT JOIN training_category t ON t.id = item.category_id
        WHERE item.credential_id IS NOT NULL AND t.legacy_credential_id IS DISTINCT FROM item.credential_id
    ) THEN
        RAISE EXCEPTION 'Legacy training references disagree with the category mapping';
    END IF;

    UPDATE course
    SET status = 0
    WHERE status = 1
      AND category_id IS NOT NULL
      AND (specialty_id IS NULL OR level_id IS NULL);
END $$;
