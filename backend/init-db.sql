-- ===== 初始数据库扩展（生产环境 docker-compose 自动执行） =====
-- 此文件仅在 PostgreSQL 数据目录为空时执行一次

-- 启用 UUID 扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 设置时区
SET timezone = 'Asia/Shanghai';
