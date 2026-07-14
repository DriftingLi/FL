-- 000008_seed_more_original_prices.down.sql
-- 回滚 000008：删除本迁移新增的 original_prices / series / brands 记录
-- 注意：为安全起见，仅删除本迁移显式新增的"非无"配置记录，
--       "无"配置记录由 000006 的 down 迁移统一负责清理

-- =====================================================
-- 1. 删除本迁移新增的 original_prices 记录
--    策略：删除所有 config_type != '无' 且匹配本迁移新增的
--          (brand, vehicle_type, series) 组合的记录
-- =====================================================
DELETE FROM original_prices
WHERE config_type != '无'
  AND (
      -- 林德 E系列 电动平衡重式 新增吨位
      (brand = '林德' AND vehicle_type = '电动平衡重式' AND series = 'E系列' AND tonnage IN (1.6, 2.0, 2.5, 3.0, 4.0, 5.0))
      -- 林德 H系列 内燃平衡重式 新增吨位
   OR (brand = '林德' AND vehicle_type = '内燃平衡重式' AND series = 'H系列' AND tonnage IN (2.0, 2.5, 3.5))
      -- 丰田 8FBE 电动平衡重式 新增吨位
   OR (brand = '丰田' AND vehicle_type = '电动平衡重式' AND series = '8FBE系列' AND tonnage IN (2.0, 3.0))
      -- 丰田 8FD 内燃平衡重式 新增吨位
   OR (brand = '丰田' AND vehicle_type = '内燃平衡重式' AND series = '8FD系列' AND tonnage IN (2.5, 3.5))
      -- 永恒力 EFG 电动平衡重式 新增吨位
   OR (brand = '永恒力' AND vehicle_type = '电动平衡重式' AND series = 'EFG系列' AND tonnage IN (1.5, 2.5, 3.0))
      -- 永恒力 ETV 电动前移式 新增吨位
   OR (brand = '永恒力' AND vehicle_type = '电动前移式' AND series = 'ETV系列' AND tonnage IN (2.0, 2.5))
      -- 合力 CPD 电动平衡重式 新增吨位
   OR (brand = '合力' AND vehicle_type = '电动平衡重式' AND series = 'CPD系列' AND tonnage IN (1.5, 2.0, 2.5, 3.5))
      -- 合力 CPCD 内燃平衡重式 新增吨位
   OR (brand = '合力' AND vehicle_type = '内燃平衡重式' AND series = 'CPCD系列' AND tonnage IN (2.0, 2.5, 3.5))
      -- 合力 内燃重型叉车 新增
   OR (brand = '合力' AND vehicle_type = '内燃重型叉车' AND series = 'CPCD系列' AND tonnage IN (10.0, 15.0))
      -- 杭叉 XE 电动平衡重式 新增吨位
   OR (brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XE系列' AND tonnage IN (1.5, 2.5))
      -- 杭叉 XH 电动平衡重式 新增吨位
   OR (brand = '杭叉' AND vehicle_type = '电动平衡重式' AND series = 'XH系列' AND tonnage IN (2.0, 2.5))
      -- 杭叉 A系列 内燃平衡重式 新增吨位
   OR (brand = '杭叉' AND vehicle_type = '内燃平衡重式' AND series = 'A系列' AND tonnage IN (2.0, 2.5, 3.5))
      -- 比亚迪 CPD 电动平衡重式 新增吨位
   OR (brand = '比亚迪' AND vehicle_type = '电动平衡重式' AND series = 'CPD系列' AND tonnage IN (1.6, 2.0, 2.5, 3.0))
      -- 龙工 LG 内燃平衡重式 新增吨位
   OR (brand = '龙工' AND vehicle_type = '内燃平衡重式' AND series = 'LG系列' AND tonnage IN (2.0, 2.5, 5.0))
      -- 龙工 LG 内燃重型叉车 新增
   OR (brand = '龙工' AND vehicle_type = '内燃重型叉车' AND series = 'LG系列' AND tonnage = 12.0)
      -- 电动托盘搬运车 新增（全品牌）
   OR (vehicle_type = '电动托盘搬运车' AND series IN ('EPT系列', 'A系列', 'CPD系列', 'EFG系列'))
      -- 电动堆高车 新增（全品牌）
   OR (vehicle_type = '电动堆高车' AND series IN ('ES系列', 'A系列', 'CPD系列'))
      -- 电动前移式 国产品牌新增
   OR (vehicle_type = '电动前移式' AND brand IN ('杭叉', '比亚迪', '合力') AND series IN ('A系列', 'CPD系列'))
  );

-- =====================================================
-- 2. 删除本迁移新增的 "无" 配置记录（仅针对新组合）
-- =====================================================
DELETE FROM original_prices
WHERE config_type = '无'
  AND (
      (brand = '中力' AND vehicle_type IN ('电动托盘搬运车', '电动堆高车'))
   OR (vehicle_type IN ('电动托盘搬运车', '电动堆高车') AND brand IN ('杭叉', '合力', '永恒力'))
   OR (vehicle_type = '电动前移式' AND brand IN ('杭叉', '比亚迪', '合力'))
   OR (brand = '中力')
  );

-- =====================================================
-- 3. 删除本迁移新增的 series（仅当无 original_prices 引用时）
-- =====================================================
DELETE FROM series
WHERE name IN ('EPT系列', 'ES系列', 'LG系列')
   OR (brand = '中力' AND name = '无');

-- =====================================================
-- 4. 删除本迁移新增的 brands（中力）
-- =====================================================
DELETE FROM brands WHERE name = '中力';
