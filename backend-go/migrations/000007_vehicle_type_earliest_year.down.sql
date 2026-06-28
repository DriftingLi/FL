-- 000007_vehicle_type_earliest_year.down.sql
-- 回滚：删除 vehicle_types.earliest_factory_year 字段

ALTER TABLE vehicle_types DROP COLUMN IF EXISTS earliest_factory_year;
