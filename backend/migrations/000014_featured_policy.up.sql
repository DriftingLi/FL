-- 内容精选分类：资讯（news）更名为政策法规（policy）
UPDATE featured_content SET category = 'policy' WHERE category = 'news';
COMMENT ON COLUMN featured_content.category IS '分类：company-公司动态, industry-行业新闻, product-产品资讯, policy-政策法规';
-- 为热点排序（按浏览量）添加索引
CREATE INDEX IF NOT EXISTS idx_featured_content_view_count ON featured_content (view_count DESC);
