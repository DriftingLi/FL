BEGIN;

CREATE TABLE wrong_question_category (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wrong_question_id INT NOT NULL REFERENCES wrong_question(id) ON DELETE CASCADE,
    category_id INT REFERENCES training_category(id),
    last_wrong_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_wrong_question_category
    ON wrong_question_category (wrong_question_id, category_id) WHERE category_id IS NOT NULL;
CREATE UNIQUE INDEX uq_wrong_question_unclassified
    ON wrong_question_category (wrong_question_id) WHERE category_id IS NULL;

WITH evidence AS (
    SELECT w.id AS wrong_question_id, r.category_id, r.created_at AS wrong_at
    FROM wrong_question w
    JOIN question_practice_record r ON r.student_id = w.student_id AND r.question_id = w.question_id
    JOIN question q ON q.id = r.question_id
    WHERE r.is_correct = false AND q.type <> 'short_answer' AND btrim(r.user_answer) <> ''

    UNION ALL

    SELECT w.id, e.category_id, COALESCE(e.submit_time, e.created_at)
    FROM mock_exam e
    CROSS JOIN LATERAL jsonb_array_elements(
        CASE WHEN jsonb_typeof(e.result -> 'details') = 'array' THEN e.result -> 'details' ELSE '[]'::jsonb END
    ) AS detail(item)
    JOIN wrong_question w ON w.student_id = e.student_id AND detail.item ->> 'question_id' = w.question_id::text
    WHERE e.status = 'submitted' AND detail.item -> 'is_correct' = 'false'::jsonb
)
INSERT INTO wrong_question_category (wrong_question_id, category_id, last_wrong_at)
SELECT wrong_question_id, category_id, max(wrong_at)
FROM evidence
GROUP BY wrong_question_id, category_id
ON CONFLICT DO NOTHING;

COMMIT;
