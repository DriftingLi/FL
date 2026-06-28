-- 000007_vehicle_type_earliest_year.up.sql
-- 车型最早出厂年份：不同车型的最早出厂时间不同，用于前端级联限制出厂年份选择
-- 调整内容：
--   1. vehicle_types 表新增 earliest_factory_year 字段（默认 1980）
--   2. 为现有车型设置合理的最早出厂年份

-- =====================================================
-- 1. 新增字段
-- =====================================================
ALTER TABLE vehicle_types
    ADD COLUMN IF NOT EXISTS earliest_factory_year INTEGER NOT NULL DEFAULT 1980;

-- =====================================================
-- 2. 为现有车型设置最早出厂年份
--    依据：内燃叉车技术成熟较早，电动叉车（尤其锂电）发展较晚
-- =====================================================
UPDATE vehicle_types SET earliest_factory_year = 1985 WHERE name = '内燃平衡重式';
UPDATE vehicle_types SET earliest_factory_year = 1990 WHERE name = '内燃重型叉车';
UPDATE vehicle_types SET earliest_factory_year = 1995 WHERE name = '电动平衡重式';
UPDATE vehicle_types SET earliest_factory_year = 2000 WHERE name = '电动前移式';
UPDATE vehicle_types SET earliest_factory_year = 1998 WHERE name = '电动托盘搬运车';
UPDATE vehicle_types SET earliest_factory_year = 2002 WHERE name = '电动堆高车';
