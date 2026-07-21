-- 000003_featured_content.up.sql
-- 内容精选表（首页"内容精选"板块：公司动态 / 行业新闻 / 产品资讯 / 资讯）
CREATE TABLE featured_content (
    content_id   INT            GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title        VARCHAR(200)   NOT NULL,
    summary      VARCHAR(500),
    cover_image  VARCHAR(500),
    content      TEXT,
    category     VARCHAR(20)    NOT NULL DEFAULT 'industry',
    source       VARCHAR(100),
    status       SMALLINT       NOT NULL DEFAULT 0,
    view_count   INT            NOT NULL DEFAULT 0,
    sort_order   INT            NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_featured_content_status    ON featured_content (status);
CREATE INDEX idx_featured_content_category  ON featured_content (category);
CREATE INDEX idx_featured_content_published ON featured_content (published_at DESC);
COMMENT ON TABLE  featured_content IS '内容精选表（公司动态/行业新闻等）';
COMMENT ON COLUMN featured_content.content_id   IS '内容ID';
COMMENT ON COLUMN featured_content.title        IS '标题';
COMMENT ON COLUMN featured_content.summary      IS '摘要（首页卡片展示）';
COMMENT ON COLUMN featured_content.cover_image  IS '封面图URL（/static/uploads/featured/xxx.jpg）';
COMMENT ON COLUMN featured_content.content      IS '正文 Markdown 源码';
COMMENT ON COLUMN featured_content.category     IS '分类：company-公司动态, industry-行业新闻, product-产品资讯, news-资讯';
COMMENT ON COLUMN featured_content.source       IS '来源（如：公司官网、新华网）';
COMMENT ON COLUMN featured_content.status       IS '状态：0-草稿，1-已发布';
COMMENT ON COLUMN featured_content.view_count   IS '阅读量';
COMMENT ON COLUMN featured_content.sort_order   IS '排序值（小→大）';
COMMENT ON COLUMN featured_content.published_at IS '发布时间（发布时写入）';
COMMENT ON COLUMN featured_content.created_at  IS '创建时间';
COMMENT ON COLUMN featured_content.updated_at   IS '更新时间';
