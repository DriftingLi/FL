-- 回滚：政策法规（policy）恢复为资讯（news）
UPDATE featured_content SET category = 'news' WHERE category = 'policy';
COMMENT ON COLUMN featured_content.category IS '分类：company-公司动态, industry-行业新闻, product-产品资讯, news-资讯';
DROP INDEX IF EXISTS idx_featured_content_view_count;
