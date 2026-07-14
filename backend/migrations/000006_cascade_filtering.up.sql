-- 000006_cascade_filtering.up.sql
-- 级联选择支持：以 original_prices 为真实数据源，添加 "无" 选项
-- 调整内容：
--   1. 字典表 config_types / mast_types / mast_heights 添加 "无" 条目
--   2. 为每个品牌的每个车型补齐 "无" 系列，确保级联至少返回一个选项
--   3. 补充 original_prices 中带 "无" 字段的组合，保证模糊匹配可命中
--   4. evaluations 表放宽 mast_height_mm 约束（允许 0 表示 "无"）

-- =====================================================
-- 1. 字典表添加 "无" 条目
-- =====================================================
INSERT INTO config_types (name) VALUES ('无') ON CONFLICT (name) DO NOTHING;
INSERT INTO mast_types (name) VALUES ('无') ON CONFLICT (name) DO NOTHING;
INSERT INTO mast_heights (value_mm) VALUES (0) ON CONFLICT (value_mm) DO NOTHING;

-- =====================================================
-- 2. 为每个品牌补齐 "无" 系列
--    series 表约束：UNIQUE(brand, name)
--    对 brands 表中每个品牌插入一条 (brand, '无') 记录
-- =====================================================
INSERT INTO series (brand, name)
SELECT b.name, '无'
FROM brands b
WHERE NOT EXISTS (
    SELECT 1 FROM series s WHERE s.brand = b.name AND s.name = '无'
);

-- =====================================================
-- 3. 补充 original_prices 中带 "无" 字段的组合
--    用于：当某品牌+车型没有具体配置/门架/门架高度时，仍可命中模糊匹配
--    策略：对每个 (brand_type, brand, vehicle_type, series) 组合，
--          若不存在带 "无" 的记录，则用该组合的最低原价补一条 "无" 记录
-- =====================================================
INSERT INTO original_prices (
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, battery_type, original_price, is_active
)
SELECT DISTINCT
    op.brand_type, op.brand, op.vehicle_type, op.series, MIN(op.tonnage),
    '无', '无', 0, NULL, MIN(op.original_price), TRUE
FROM original_prices op
WHERE NOT EXISTS (
    SELECT 1 FROM original_prices p2
    WHERE p2.brand_type = op.brand_type
      AND p2.brand = op.brand
      AND p2.vehicle_type = op.vehicle_type
      AND p2.series = op.series
      AND p2.config_type = '无'
)
GROUP BY op.brand_type, op.brand, op.vehicle_type, op.series
ON CONFLICT DO NOTHING;

-- =====================================================
-- 4. evaluations 表放宽约束
--    mast_height_mm 允许 0（表示 "无"）
--    原 CHECK 约束（若有）不需要修改，因为表定义中没有显式 CHECK
-- =====================================================
-- 无需 DDL：mast_height_mm 已是 INTEGER NOT NULL，0 是合法值
