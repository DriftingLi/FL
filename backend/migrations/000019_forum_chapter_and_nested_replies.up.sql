-- 000019_forum_chapter_and_nested_replies.up.sql
-- 论坛改为章节维度（course_id → chapter_id），并支持回复别人的回复（parent_id）

BEGIN;

-- 1. 主题表：course_id → chapter_id（NULL 仍表示综合讨论区）
ALTER TABLE forum_topics RENAME COLUMN course_id TO chapter_id;

ALTER TABLE forum_topics DROP CONSTRAINT IF EXISTS forum_topics_course_id_fkey;
ALTER TABLE forum_topics ADD CONSTRAINT forum_topics_chapter_id_fkey
    FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_forum_topics_course;
CREATE INDEX IF NOT EXISTS idx_forum_topics_chapter
    ON forum_topics (chapter_id, created_at DESC);

-- 2. 回复表：支持回复某条回复（parent_id NULL = 直接回复主题）
ALTER TABLE forum_replies ADD COLUMN IF NOT EXISTS parent_id BIGINT;
ALTER TABLE forum_replies ADD CONSTRAINT forum_replies_parent_id_fkey
    FOREIGN KEY (parent_id) REFERENCES forum_replies(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_forum_replies_parent ON forum_replies (parent_id);

COMMIT;
