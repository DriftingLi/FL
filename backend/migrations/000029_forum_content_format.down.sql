-- 回滚 ADR-0044 的正文格式声明位。
--
-- **有损性如实声明**：回滚会丢弃「哪些正文是 markdown」这一事实。
-- 但内容本身（content 列）一字不动——格式声明是纯附加信息，
-- 回滚后所有正文按纯文本渲染，与本次改造之前的行为一致。

ALTER TABLE forum_replies DROP CONSTRAINT IF EXISTS chk_forum_replies_content_format;
ALTER TABLE forum_replies DROP COLUMN IF EXISTS content_format;

ALTER TABLE forum_topics DROP CONSTRAINT IF EXISTS chk_forum_topics_content_format;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS content_format;
