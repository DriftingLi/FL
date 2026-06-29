-- 000010_config_type_restructure.down.sql
-- 回滚配置类型重构：恢复 battery_type 字段，删除维度字典表和 series_config_options 表
-- 注意：original_prices 数据已被 up 迁移清空，无法恢复，需重新录入

-- =====================================================
-- 1. 恢复 evaluations.battery_type 字段
-- =====================================================
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS battery_type VARCHAR(50);

-- =====================================================
-- 2. 恢复 original_prices.battery_type 字段和旧约束
-- =====================================================
ALTER TABLE original_prices ADD COLUMN IF NOT EXISTS battery_type VARCHAR(50);

-- 删除 8 字段唯一约束，恢复 9 字段唯一约束
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint
              WHERE conrelid = 'original_prices'::regclass AND contype = 'u') LOOP
        EXECUTE 'ALTER TABLE original_prices DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
    END LOOP;
END $$;
ALTER TABLE original_prices
    ADD CONSTRAINT original_prices_9field_unique
    UNIQUE (brand_type, brand, vehicle_type, series, tonnage,
            config_type, mast_type, mast_height_mm, battery_type);

-- 重建查询索引（本就不含 battery_type，重建以保持一致）
DROP INDEX IF EXISTS idx_original_prices_lookup;
CREATE INDEX IF NOT EXISTS idx_original_prices_lookup ON original_prices(
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm
);

-- =====================================================
-- 3. 删除 series_config_options 表
-- =====================================================
DROP TABLE IF EXISTS series_config_options;

-- =====================================================
-- 4. 删除维度字典表
-- =====================================================
DROP TABLE IF EXISTS engine_types;
DROP TABLE IF EXISTS transmission_types;

-- =====================================================
-- 5. 移除 battery_types 中的 "无" 选项
-- =====================================================
DELETE FROM battery_types WHERE name = '无';
