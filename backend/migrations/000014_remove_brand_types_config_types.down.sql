-- 000014_remove_brand_types_config_types.down.sql
-- 回滚 000014：恢复 brand_type 列与 brand_types / config_types 表
-- 注意：回滚后 brands.brand_type 与 original_prices.brand_type 仅恢复列结构，
--       原始分类数据（如"进口一线"）无法恢复，需重新维护

-- =====================================================
-- 1. 恢复 evaluations.brand_type 列
-- =====================================================
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS brand_type VARCHAR(50);
UPDATE evaluations SET brand_type = '未分类' WHERE brand_type IS NULL;
ALTER TABLE evaluations ALTER COLUMN brand_type SET NOT NULL;

-- =====================================================
-- 2. 恢复 original_prices.brand_type 列与约束
-- =====================================================
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint
              WHERE conrelid = 'original_prices'::regclass AND contype = 'u') LOOP
        EXECUTE 'ALTER TABLE original_prices DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
    END LOOP;
END $$;

ALTER TABLE original_prices ADD COLUMN IF NOT EXISTS brand_type VARCHAR(50);
UPDATE original_prices SET brand_type = '未分类' WHERE brand_type IS NULL;
ALTER TABLE original_prices ALTER COLUMN brand_type SET NOT NULL;

ALTER TABLE original_prices
    ADD CONSTRAINT original_prices_8field_unique
    UNIQUE (brand_type, brand, vehicle_type, series, tonnage,
            config_type, mast_type, mast_height_mm);

DROP INDEX IF EXISTS idx_original_prices_lookup;
CREATE INDEX idx_original_prices_lookup ON original_prices(
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm
);

-- =====================================================
-- 3. 恢复 brands.brand_type 列与索引
-- =====================================================
ALTER TABLE brands ADD COLUMN IF NOT EXISTS brand_type VARCHAR(50) NOT NULL DEFAULT '未分类';
CREATE INDEX IF NOT EXISTS idx_brands_type ON brands(brand_type);

-- =====================================================
-- 4. 重建 brand_types 表与种子数据
-- =====================================================
CREATE TABLE IF NOT EXISTS brand_types (
    name    VARCHAR(50) PRIMARY KEY,
    k_type  DECIMAL(5,4) NOT NULL DEFAULT 1.0
);

INSERT INTO brand_types (name, k_type) VALUES
  ('进口一线', 1.15),
  ('进口二线', 1.05),
  ('国产一线', 1.00),
  ('国产二线', 0.92),
  ('国产其他', 0.85)
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 5. 重建 config_types 表与种子数据
-- =====================================================
CREATE TABLE IF NOT EXISTS config_types (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) UNIQUE NOT NULL
);

INSERT INTO config_types (name) VALUES
  ('标准配置'), ('高配置'), ('无')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 6. 还原 coefficient_configs.description 为简略中文（与 000005 一致）
-- =====================================================
UPDATE coefficient_configs SET description = CASE key
    WHEN 'lambda_electric'    THEN '电动叉车时间衰减系数 λ'
    WHEN 'lambda_combustion'  THEN '内燃叉车时间衰减系数 λ'
    WHEN 'annual_usage_hours' THEN '年度标准使用小时'
    WHEN 'confidence_range'   THEN '残值置信区间幅度 ±'
    WHEN 'k_hours_ratio_low'  THEN '工时区间阈值：低'
    WHEN 'k_hours_ratio_mid'  THEN '工时区间阈值：中'
    WHEN 'k_hours_ratio_high' THEN '工时区间阈值：高'
    WHEN 'k_hours_ratio_max'  THEN '工时区间阈值：最大'
    ELSE description
END WHERE key IN (
    'lambda_electric', 'lambda_combustion', 'annual_usage_hours',
    'confidence_range', 'k_hours_ratio_low', 'k_hours_ratio_mid',
    'k_hours_ratio_high', 'k_hours_ratio_max'
);
