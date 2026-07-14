-- 000011_seed_original_prices.down.sql
-- 回滚 000011 迁移：清空 original_prices 数据，删除新增的 series_config_options、series、字典项
-- 注意：000010 已清空 original_prices，本 DOWN 恢复到 000010 结束时的状态

-- 1. 清空 original_prices（000010 已清空，此处恢复为空）
DELETE FROM original_prices;

-- 2. 删除本迁移新增的 series_config_options
DELETE FROM series_config_options WHERE series IN (
  'K系列（K1）', 'K2系列（大力士）', 'G2系列', 'G3系列', 'H3系列', 'H4系列',
  'K3系列（锂电专用）', '前移式',
  'XA系列', 'XC系列（锂电专用）', 'XH系列（高压锂电）', 'J系列（大吨位）',
  'X系列', 'A系列（经济型）',
  '7系列（7FD）', '8系列（8FD）', 'Z系列（本土化）', '8系列（8FBN）',
  '8系列（8FBE）', '8系列（8FBR）',
  'E系列（E16-E20）', 'E系列（E25-E30）', 'E系列（E35-E50）',
  'R系列（R14-R20）', 'R系列（R14-R25）', 'L系列',
  'CPCD系列', 'N系列', 'CDD系列',
  'E系列',
  'CPD系列（大吨位）', 'S系列'
);

-- 删除杭叉 A系列、柳工 CLG系列 本迁移新增的 battery 维度（保留 000010 的 transmission+engine）
DELETE FROM series_config_options
WHERE (brand = '杭叉' AND series = 'A系列' AND dimension = 'battery')
   OR (brand = '柳工' AND series = 'CLG系列' AND dimension = 'battery');

-- 3. 删除本迁移新增的 series
DELETE FROM series WHERE (brand, name) IN (
  ('合力', 'K系列（K1）'), ('合力', 'K2系列（大力士）'), ('合力', 'G2系列'),
  ('合力', 'G3系列'), ('合力', 'H3系列'), ('合力', 'H4系列'),
  ('合力', 'K3系列（锂电专用）'), ('合力', '前移式'),
  ('杭叉', 'XA系列'), ('杭叉', 'XC系列（锂电专用）'), ('杭叉', 'XH系列（高压锂电）'),
  ('杭叉', 'J系列（大吨位）'), ('杭叉', 'X系列'), ('杭叉', 'A系列（经济型）'),
  ('丰田', '7系列（7FD）'), ('丰田', '8系列（8FD）'), ('丰田', 'Z系列（本土化）'),
  ('丰田', '8系列（8FBN）'), ('丰田', '8系列（8FBE）'), ('丰田', '8系列（8FBR）'),
  ('林德', 'E系列（E16-E20）'), ('林德', 'E系列（E25-E30）'), ('林德', 'E系列（E35-E50）'),
  ('林德', 'R系列（R14-R20）'), ('林德', 'R系列（R14-R25）'), ('林德', 'L系列'),
  ('龙工', 'CPCD系列'), ('龙工', 'N系列'), ('龙工', 'CDD系列'),
  ('柳工', 'E系列'),
  ('比亚迪', 'CPD系列（大吨位）'), ('比亚迪', 'S系列')
);

-- 4. 删除本迁移新增的字典项
DELETE FROM mast_types WHERE name IN (
  '两级标准门架', '两级宽视野门架', '三级全自由门架', '四级HD门架', '四级门架'
);

DELETE FROM mast_heights WHERE value_mm IN (
  2500, 2900, 3250, 3500, 8000, 9500, 10000, 11300, 12000
);

DELETE FROM tonnages WHERE value IN (
  1.4, 1.6, 2.8, 3.2, 3.8, 5.5, 6.5, 7.5, 8.5, 12.0, 32.0
);

DELETE FROM transmission_types WHERE name = '静压传动';
