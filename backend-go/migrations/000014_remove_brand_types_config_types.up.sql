-- 000014_remove_brand_types_config_types.up.sql
-- 重构残值评估配置：
--   1. 删除 brand_types 表（Kb 公式简化为 Kb = k_brand，不再需要 k_type）
--   2. 删除 config_types 表（000010 重构后已废弃，仅保留表结构）
--   3. 从 brands / original_prices / evaluations 表移除 brand_type 列
--   4. 更新 coefficient_configs.description 为中文精确描述

-- =====================================================
-- 1. 删除冗余字典表
-- =====================================================
DROP TABLE IF EXISTS config_types;
DROP TABLE IF EXISTS brand_types;

-- =====================================================
-- 2. brands 表移除 brand_type 列与索引
-- =====================================================
DROP INDEX IF EXISTS idx_brands_type;
ALTER TABLE brands DROP COLUMN IF EXISTS brand_type;

-- =====================================================
-- 3. original_prices 表移除 brand_type 列
--    重建唯一约束（8 字段 → 7 字段）与查询索引
-- =====================================================
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint
              WHERE conrelid = 'original_prices'::regclass AND contype = 'u') LOOP
        EXECUTE 'ALTER TABLE original_prices DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
    END LOOP;
END $$;

ALTER TABLE original_prices DROP COLUMN IF EXISTS brand_type;

ALTER TABLE original_prices
    ADD CONSTRAINT original_prices_7field_unique
    UNIQUE (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm);

DROP INDEX IF EXISTS idx_original_prices_lookup;
CREATE INDEX idx_original_prices_lookup ON original_prices(
    brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm
);

-- =====================================================
-- 4. evaluations 表移除 brand_type 列
-- =====================================================
ALTER TABLE evaluations DROP COLUMN IF EXISTS brand_type;

-- =====================================================
-- 5. 更新 coefficient_configs.description 为中文精确描述
--    说明每个参数的含义、取值范围与对残值的影响方向
-- =====================================================
UPDATE coefficient_configs SET description = CASE key
    WHEN 'lambda_electric'    THEN '电动叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.10~0.15）'
    WHEN 'lambda_combustion' THEN '内燃叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.08~0.12）'
    WHEN 'annual_usage_hours' THEN '年度标准使用小时数（行业平均年工时，用于计算使用强度比值，电动一般 1500~2000，内燃一般 1200~1800）'
    WHEN 'confidence_range'   THEN '残值置信区间幅度 ±（如 0.10 表示残值上下浮动 10%，值越大区间越宽）'
    WHEN 'k_hours_ratio_low'  THEN '使用强度比值下限阈值（实际工时/标准工时 < 此值时 Kh=1.10，使用强度低，保值加成）'
    WHEN 'k_hours_ratio_mid'  THEN '使用强度比值中段阈值（比值在 low 与此值之间时 Kh=1.00，正常使用）'
    WHEN 'k_hours_ratio_high' THEN '使用强度比值上段阈值（比值在 mid 与此值之间时 Kh=0.95，使用强度偏高）'
    WHEN 'k_hours_ratio_max'  THEN '使用强度比值上限阈值（比值在 high 与此值之间时 Kh=0.90，重型使用；超过此值 Kh=0.85）'
    ELSE description
END WHERE key IN (
    'lambda_electric', 'lambda_combustion', 'annual_usage_hours',
    'confidence_range', 'k_hours_ratio_low', 'k_hours_ratio_mid',
    'k_hours_ratio_high', 'k_hours_ratio_max'
);
