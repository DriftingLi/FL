-- 000019_forum_chapter_and_nested_replies.down.sql

BEGIN;

ALTER TABLE forum_replies DROP CONSTRAINT IF EXISTS forum_replies_parent_id_fkey;
DROP INDEX IF EXISTS idx_forum_replies_parent;
ALTER TABLE forum_replies DROP COLUMN IF EXISTS parent_id;

ALTER TABLE forum_topics RENAME COLUMN chapter_id TO course_id;
ALTER TABLE forum_topics DROP CONSTRAINT IF EXISTS forum_topics_chapter_id_fkey;
ALTER TABLE forum_topics ADD CONSTRAINT forum_topics_course_id_fkey
    FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_forum_topics_chapter;
CREATE INDEX IF NOT EXISTS idx_forum_topics_course
    ON forum_topics (course_id, created_at DESC);

COMMIT;
