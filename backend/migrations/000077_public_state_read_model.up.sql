CREATE VIEW course_public_state AS
SELECT eligibility.*,
       NOT course.public_hidden AS public_visibility
FROM course_public_eligibility AS eligibility
JOIN course ON course.course_id = eligibility.course_id;

CREATE VIEW question_public_state AS
SELECT eligibility.*,
       NOT question.public_hidden AS public_visibility
FROM question_public_eligibility AS eligibility
JOIN question ON question.id = eligibility.id;
