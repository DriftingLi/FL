-- 回滚 000043：删掉 scheduled_at 及其索引/约束（逆序）。
--
-- 该列是本票新增、无历史数据 ⇒ 回滚只丢失「已排期但未到点」的排期；那些行退回普通草稿
-- （status 本来就是 0，published_at 本来就是空），不会误发布任何内容。

ALTER TABLE featured_content
    DROP CONSTRAINT IF EXISTS chk_featured_content_scheduled_not_published;

DROP INDEX IF EXISTS idx_featured_content_scheduled_at;

ALTER TABLE featured_content
    DROP COLUMN IF EXISTS scheduled_at;
