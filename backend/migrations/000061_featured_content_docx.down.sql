-- 回滚 000061：删掉 content_type / docx_url 及其 CHECK 约束（逆序）。
--
-- 该两列是本票新增 ⇒ 回滚只丢失「docx 文章的类型标记与文件指向」：那些行的 content 本就是空串，
-- 回滚后退化成一篇正文为空的 Markdown 文章（管理端看得见、可重新编辑），
-- 已上传的 .docx 文件仍留在存储 featured/docx/ 下，由悬空回收或人工清理。

ALTER TABLE featured_content
    DROP CONSTRAINT IF EXISTS chk_featured_content_content_type;

ALTER TABLE featured_content
    DROP COLUMN IF EXISTS docx_url;

ALTER TABLE featured_content
    DROP COLUMN IF EXISTS content_type;
