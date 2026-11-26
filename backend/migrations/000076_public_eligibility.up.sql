CREATE VIEW training_category_public_eligibility AS
SELECT category.id,
       category.name,
       category.status,
       category.sort_order,
       CASE WHEN category.status <> 1 THEN 'category_disabled,' ELSE '' END ||
       CASE
           WHEN category.legacy_credential_id IS NULL THEN 'category_unmapped,'
           WHEN credential.id IS NULL THEN 'credential_missing,'
           WHEN credential.status <> 1 THEN 'credential_disabled,'
           ELSE ''
       END AS blocker_codes
FROM training_category AS category
LEFT JOIN credential ON credential.id = category.legacy_credential_id;

CREATE VIEW course_public_eligibility AS
WITH blockers AS (
    SELECT course.course_id, course.name, course.course_type, course.description,
           course.cover_image, course.points_price, course.sort_order, course.created_at,
           category.id AS category_id, category.name AS category_name, category.status AS category_status,
           course.status <> 1 AS not_published,
           category.blocker_codes AS category_blockers,
           course.public_hidden AS manually_hidden
    FROM course
    LEFT JOIN training_category_public_eligibility AS category ON category.id = course.category_id
)
SELECT course_id, name, course_type, description, cover_image, points_price, sort_order, created_at,
       category_id, category_name, category_status,
       CASE WHEN not_published THEN 'not_published,' ELSE '' END ||
       COALESCE(category_blockers, 'category_missing,') ||
       CASE WHEN manually_hidden THEN 'manually_hidden,' ELSE '' END AS blocker_codes,
       NOT not_published AND category_id IS NOT NULL AND category_blockers = '' AND NOT manually_hidden AS eligible
FROM blockers;

CREATE VIEW question_public_eligibility AS
WITH blockers AS (
    SELECT question.id, question.type,
           category.id AS category_id, category.name AS category_name,
           category.status AS category_status, category.sort_order AS category_sort_order,
           question.status <> 'published' AS not_published,
           source_question.question_id IS NOT NULL AS source_tag_excluded,
           category.blocker_codes AS category_blockers,
           question.public_hidden AS manually_hidden
    FROM question
    LEFT JOIN training_category_public_eligibility AS category ON category.id = question.category_id
    LEFT JOIN (
        SELECT DISTINCT relation.question_id
        FROM question_tag_relation AS relation
        JOIN question_tag AS tag ON tag.id = relation.tag_id
        WHERE tag.is_source_tag
    ) AS source_question ON source_question.question_id = question.id
)
SELECT id, type, category_id, category_name, category_status, category_sort_order,
       CASE WHEN not_published THEN 'not_published,' ELSE '' END ||
       CASE WHEN source_tag_excluded THEN 'source_tag_excluded,' ELSE '' END ||
       COALESCE(category_blockers, 'category_missing,') ||
       CASE WHEN manually_hidden THEN 'manually_hidden,' ELSE '' END AS blocker_codes,
       NOT not_published AND NOT source_tag_excluded AND category_id IS NOT NULL
           AND category_blockers = '' AND NOT manually_hidden AS eligible
FROM blockers;
