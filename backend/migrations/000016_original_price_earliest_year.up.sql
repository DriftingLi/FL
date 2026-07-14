-- 000016_original_price_earliest_year.up.sql
-- 1. original_prices 新增 earliest_factory_year 字段（每条记录独立配置最早出厂年份）
--    学生端选完吨位后，按品牌+车型+系列+吨位取这些原价记录中的最早年份作为出厂年份下限
-- 2. 用 series 表回填现有数据的 earliest_factory_year（series 不存在时保留默认 2000）
-- 3. 将 config_type / mast_type 的空字符串统一为 '无'（与前端 NONE_VALUE 常量一致）

-- =====================================================
-- 1. 新增 earliest_factory_year 字段
-- =====================================================
ALTER TABLE original_prices
    ADD COLUMN IF NOT EXISTS earliest_factory_year INTEGER NOT NULL DEFAULT 2000;

-- =====================================================
-- 2. 用 series 表回填现有数据
-- =====================================================
UPDATE original_prices op
SET earliest_factory_year = s.earliest_factory_year
FROM series s
WHERE s.brand = op.brand AND s.name = op.series;

-- =====================================================
-- 3. config_type / mast_type 空值统一为 '无'
--    original_prices 表中 config_type/mast_type 为 NOT NULL，但历史数据可能存空字符串
-- =====================================================
UPDATE original_prices SET config_type = '无' WHERE config_type = '' OR config_type IS NULL;
UPDATE original_prices SET mast_type = '无' WHERE mast_type = '' OR mast_type IS NULL;
