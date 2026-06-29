-- ============================================================
-- 000012: 系列去重 + "无"→"其它" 重命名
-- 问题一：合并重复系列（林德E系列、丰田8FBE/8FD、杭叉XC/XH、林德R系列）
-- 问题三：系列名 "无" → "其它"（三表同步：series / original_prices / series_config_options）
-- ============================================================

-- ============================================================
-- Part 1: 系列 "无" → "其它"
-- ============================================================
UPDATE series SET name = '其它' WHERE name = '无';
UPDATE original_prices SET series = '其它' WHERE series = '无';
UPDATE series_config_options SET series = '其它' WHERE series = '无';

-- ============================================================
-- Part 2: 合并重复系列
-- 策略：先删除目标系列的冲突行（与源系列 8 字段完全相同者），再 UPDATE 源系列
-- ============================================================

-- 2.1 林德 电动平衡重式：E系列（E16-E20）/ E系列（E25-E30）/ E系列（E35-E50）→ E系列
-- 先删除目标 E 系列中会被源覆盖的冲突行（保留 8 字段组合的唯一性）
DELETE FROM original_prices
WHERE brand = '林德' AND vehicle_type = '电动平衡重式' AND series = 'E系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '林德' AND vehicle_type = '电动平衡重式'
      AND series IN ('E系列（E16-E20）', 'E系列（E25-E30）', 'E系列（E35-E50）')
  );

UPDATE original_prices SET series = 'E系列'
WHERE brand = '林德' AND vehicle_type = '电动平衡重式'
  AND series IN ('E系列（E16-E20）', 'E系列（E25-E30）', 'E系列（E35-E50）');

DELETE FROM series_config_options
WHERE brand = '林德'
  AND series IN ('E系列（E16-E20）', 'E系列（E25-E30）', 'E系列（E35-E50）');

DELETE FROM series
WHERE brand = '林德'
  AND name IN ('E系列（E16-E20）', 'E系列（E25-E30）', 'E系列（E35-E50）');

-- 2.2 丰田 电动平衡重式：8系列（8FBE）→ 8FBE系列
DELETE FROM original_prices
WHERE brand = '丰田' AND vehicle_type = '电动平衡重式' AND series = '8FBE系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '丰田' AND vehicle_type = '电动平衡重式' AND series = '8系列（8FBE）'
  );

UPDATE original_prices SET series = '8FBE系列'
WHERE brand = '丰田' AND vehicle_type = '电动平衡重式' AND series = '8系列（8FBE）';

DELETE FROM series_config_options WHERE brand = '丰田' AND series = '8系列（8FBE）';
DELETE FROM series WHERE brand = '丰田' AND name = '8系列（8FBE）';

-- 2.3 丰田 内燃平衡重式：8系列（8FD）→ 8FD系列
DELETE FROM original_prices
WHERE brand = '丰田' AND vehicle_type = '内燃平衡重式' AND series = '8FD系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '丰田' AND vehicle_type = '内燃平衡重式' AND series = '8系列（8FD）'
  );

UPDATE original_prices SET series = '8FD系列'
WHERE brand = '丰田' AND vehicle_type = '内燃平衡重式' AND series = '8系列（8FD）';

DELETE FROM series_config_options WHERE brand = '丰田' AND series = '8系列（8FD）';
DELETE FROM series WHERE brand = '丰田' AND name = '8系列（8FD）';

-- 2.4 杭叉 电动平衡重式：XC系列（锂电专用）→ XC系列
DELETE FROM original_prices
WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XC系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XC系列（锂电专用）'
  );

UPDATE original_prices SET series = 'XC系列'
WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XC系列（锂电专用）';

DELETE FROM series_config_options WHERE brand = '杭叉' AND series = 'XC系列（锂电专用）';
DELETE FROM series WHERE brand = '杭叉' AND name = 'XC系列（锂电专用）';

-- 2.5 杭叉 电动平衡重式：XH系列（高压锂电）→ XH系列
DELETE FROM original_prices
WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XH系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XH系列（高压锂电）'
  );

UPDATE original_prices SET series = 'XH系列'
WHERE brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XH系列（高压锂电）';

DELETE FROM series_config_options WHERE brand = '杭叉' AND series = 'XH系列（高压锂电）';
DELETE FROM series WHERE brand = '杭叉' AND name = 'XH系列（高压锂电）';

-- 2.6 林德 电动前移式：R系列（R14-R20）/ R系列（R14-R25）→ R系列
-- 先创建 R系列（目标行）
INSERT INTO series (brand, name, earliest_factory_year)
VALUES ('林德', 'R系列', 2010)
ON CONFLICT (brand, name) DO NOTHING;

-- 删除目标 R系列中与源冲突的行
DELETE FROM original_prices
WHERE brand = '林德' AND vehicle_type = '电动前移式' AND series = 'R系列'
  AND (tonnage, config_type, mast_type, mast_height_mm) IN (
    SELECT tonnage, config_type, mast_type, mast_height_mm
    FROM original_prices
    WHERE brand = '林德' AND vehicle_type = '电动前移式'
      AND series IN ('R系列（R14-R20）', 'R系列（R14-R25）')
  );

UPDATE original_prices SET series = 'R系列'
WHERE brand = '林德' AND vehicle_type = '电动前移式'
  AND series IN ('R系列（R14-R20）', 'R系列（R14-R25）');

-- 合并 series_config_options
INSERT INTO series_config_options (brand, series, dimension, option_name)
SELECT DISTINCT '林德', 'R系列', dimension, option_name
FROM series_config_options
WHERE brand = '林德'
  AND series IN ('R系列（R14-R20）', 'R系列（R14-R25）')
ON CONFLICT (brand, series, dimension, option_name) DO NOTHING;

DELETE FROM series_config_options
WHERE brand = '林德' AND series IN ('R系列（R14-R20）', 'R系列（R14-R25）');

DELETE FROM series
WHERE brand = '林德' AND name IN ('R系列（R14-R20）', 'R系列（R14-R25）');
