-- 章节幻灯片 URL 列表（PPT 转图后持久化，避免每次请求重新转图或扫描存储）
-- 存储 JSON 数组字符串，如 '["https://cdn.example.com/slides/12/slide_001.png", ...]'
-- 为 NULL 表示尚未生成；为 '[]' 表示已生成但为空。
ALTER TABLE chapter ADD COLUMN IF NOT EXISTS slide_urls TEXT;
