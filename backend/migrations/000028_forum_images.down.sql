-- 000028_forum_images.down.sql
-- 回滚：移除论坛主题/回复的 images 列。

BEGIN;

ALTER TABLE forum_replies
    DROP COLUMN IF EXISTS images;

ALTER TABLE forum_topics
    DROP COLUMN IF EXISTS images;

COMMIT;
