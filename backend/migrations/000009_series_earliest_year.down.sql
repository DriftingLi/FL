-- 000009_series_earliest_year.down.sql
-- 回滚：删除 series 表的 earliest_factory_year 列
ALTER TABLE series DROP COLUMN IF EXISTS earliest_factory_year;
