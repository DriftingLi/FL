-- 000028_forum_images.up.sql
-- 论坛发图（图文分离）：主题与回复各增加 images JSONB 列（默认 '[]'），
-- 存本站上传图片的 URL 数组，正文保持纯文本，不做 markdown 渲染。

BEGIN;

ALTER TABLE forum_topics
    ADD COLUMN IF NOT EXISTS images JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE forum_replies
    ADD COLUMN IF NOT EXISTS images JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN forum_topics.images IS '主题图片 URL 数组（images/forum/ 子目录，最多 9 张）';
COMMENT ON COLUMN forum_replies.images IS '回复图片 URL 数组（images/forum/ 子目录，最多 3 张）';

COMMIT;
