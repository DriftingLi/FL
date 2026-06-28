-- 000006_cascade_filtering.down.sql
-- 回滚：移除 "无" 条目和补充的 original_prices 记录

-- 删除补充的 original_prices（config_type = '无'）
DELETE FROM original_prices WHERE config_type = '无' AND mast_type = '无' AND mast_height_mm = 0;

-- 删除 "无" 系列条目
DELETE FROM series WHERE name = '无';

-- 删除字典表中的 "无" 条目
DELETE FROM config_types WHERE name = '无';
DELETE FROM mast_types WHERE name = '无';
DELETE FROM mast_heights WHERE value_mm = 0;
