-- 000011_seed_original_prices.up.sql
-- 重新播种 original_prices 数据（适配配置类型重构后的新 config_type 格式）
-- 并按 Excel 数据（叉车配置大全.xlsx + 叉车价格对照表.xlsx）补充 series、字典表、series_config_options
--
-- config_type 由维度拼接：
--   电动 series（仅 battery 维度）：config_type = 电池值（如 '磷酸铁锂(LFP)'、'铅酸'）
--   内燃 series（transmission + engine 维度）：config_type = '传动/发动机'（如 '手波/国产发动机'）
--
-- 数据来源：
--   1. 迁移 000008 已有价格基准 + 林德官方/1688/阿里巴巴/招投标公告等网络公开数据
--   2. 叉车配置大全.xlsx + 叉车价格对照表.xlsx（用户提供的参考数据）
-- 价格单位：人民币元（含税参考价），均为新车指导价/市场价

-- =====================================================
-- 1. 补充字典表缺失项
-- =====================================================

-- 1.1 mast_types 补充（Excel 中出现但数据库尚无的门架类型）
INSERT INTO mast_types (name) VALUES
  ('两级标准门架'),
  ('两级宽视野门架'),
  ('三级全自由门架'),
  ('四级HD门架'),
  ('四级门架')
ON CONFLICT (name) DO NOTHING;

-- 1.2 mast_heights 补充
INSERT INTO mast_heights (value_mm) VALUES
  (2500), (2900), (3250), (3500),
  (8000), (9500), (10000), (11300), (12000)
ON CONFLICT (value_mm) DO NOTHING;

-- 1.3 tonnages 补充
INSERT INTO tonnages (value) VALUES
  (1.4), (1.6), (2.8), (3.2), (3.8),
  (5.5), (6.5), (7.5), (8.5),
  (12.0), (32.0)
ON CONFLICT (value) DO NOTHING;

-- 1.4 transmission_types 补充
INSERT INTO transmission_types (name) VALUES ('静压传动') ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 2. 补充 series 数据（含 earliest_factory_year）
--    数据来源：叉车配置大全.xlsx 的"最早出厂年份"列
--    使用 ON CONFLICT DO UPDATE 以便已存在的 series 也能更新 earliest_factory_year
-- =====================================================

INSERT INTO series (brand, name, earliest_factory_year) VALUES
  -- 合力
  ('合力', 'K系列（K1）', 2011),
  ('合力', 'K2系列（大力士）', 2022),
  ('合力', 'G2系列', 2020),
  ('合力', 'G3系列', 2023),
  ('合力', 'H3系列', 2020),
  ('合力', 'H4系列', 2023),
  ('合力', 'K3系列（锂电专用）', 2025),
  ('合力', '前移式', 2022),
  -- 杭叉
  ('杭叉', 'XA系列', 2020),
  ('杭叉', 'XC系列（锂电专用）', 2018),
  ('杭叉', 'XH系列（高压锂电）', 2022),
  ('杭叉', 'J系列（大吨位）', 2020),
  ('杭叉', 'X系列', 2019),
  ('杭叉', 'A系列（经济型）', 2018),
  -- 丰田
  ('丰田', '7系列（7FD）', 1999),
  ('丰田', '8系列（8FD）', 2007),
  ('丰田', 'Z系列（本土化）', 2010),
  ('丰田', '8系列（8FBN）', 2011),
  ('丰田', '8系列（8FBE）', 2015),
  ('丰田', '8系列（8FBR）', 2015),
  -- 林德
  ('林德', 'E系列（E16-E20）', 2010),
  ('林德', 'E系列（E25-E30）', 2012),
  ('林德', 'E系列（E35-E50）', 2012),
  ('林德', 'R系列（R14-R20）', 2010),
  ('林德', 'R系列（R14-R25）', 2010),
  ('林德', 'L系列', 2015),
  -- 龙工
  ('龙工', 'CPCD系列', 2012),
  ('龙工', 'N系列', 2018),
  ('龙工', 'CDD系列', 2019),
  -- 柳工
  ('柳工', 'E系列', 2020),
  -- 比亚迪
  ('比亚迪', 'CPD系列（大吨位）', 2018),
  ('比亚迪', 'S系列', 2018)
ON CONFLICT (brand, name) DO UPDATE
SET earliest_factory_year = EXCLUDED.earliest_factory_year;

-- =====================================================
-- 3. 补充 series_config_options 数据
--    规则：
--    - 内燃 series：transmission + engine 维度
--    - 电动 series：battery 维度
--    - 同时存在内燃与电动变体的 series：三维度全加
-- =====================================================

-- ----- 合力 内燃 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','K系列（K1）','transmission','手波'),
  ('合力','K系列（K1）','transmission','自波'),
  ('合力','K系列（K1）','transmission','无级变速'),
  ('合力','K系列（K1）','transmission','无'),
  ('合力','K系列（K1）','engine','国产发动机'),
  ('合力','K系列（K1）','engine','进口发动机'),
  ('合力','K系列（K1）','engine','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','K2系列（大力士）','transmission','自波'),
  ('合力','K2系列（大力士）','transmission','无'),
  ('合力','K2系列（大力士）','engine','国产发动机'),
  ('合力','K2系列（大力士）','engine','进口发动机'),
  ('合力','K2系列（大力士）','engine','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','G2系列','transmission','自波'),
  ('合力','G2系列','transmission','无'),
  ('合力','G2系列','engine','国产发动机'),
  ('合力','G2系列','engine','进口发动机'),
  ('合力','G2系列','engine','无')
ON CONFLICT DO NOTHING;

-- ----- 合力 电动 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','H3系列','battery','铅酸'),
  ('合力','H3系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','H4系列','battery','磷酸铁锂(LFP)'),
  ('合力','H4系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','K3系列（锂电专用）','battery','磷酸铁锂(LFP)'),
  ('合力','K3系列（锂电专用）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','G3系列','battery','磷酸铁锂(LFP)'),
  ('合力','G3系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','前移式','battery','铅酸'),
  ('合力','前移式','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 杭叉 内燃 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XA系列','transmission','自波'),
  ('杭叉','XA系列','transmission','无'),
  ('杭叉','XA系列','engine','国产发动机'),
  ('杭叉','XA系列','engine','进口发动机'),
  ('杭叉','XA系列','engine','无')
ON CONFLICT DO NOTHING;

-- ----- 杭叉 电动 series -----
-- 杭叉 A系列（电动平衡重式变体）：补 battery 维度（000010 已有 transmission+engine）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','A系列','battery','铅酸'),
  ('杭叉','A系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XC系列（锂电专用）','battery','磷酸铁锂(LFP)'),
  ('杭叉','XC系列（锂电专用）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XH系列（高压锂电）','battery','磷酸铁锂(LFP)'),
  ('杭叉','XH系列（高压锂电）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','J系列（大吨位）','battery','铅酸'),
  ('杭叉','J系列（大吨位）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','X系列','battery','铅酸'),
  ('杭叉','X系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','A系列（经济型）','battery','铅酸'),
  ('杭叉','A系列（经济型）','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 丰田 内燃 series（进口发动机）-----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','7系列（7FD）','transmission','自波'),
  ('丰田','7系列（7FD）','transmission','无'),
  ('丰田','7系列（7FD）','engine','进口发动机'),
  ('丰田','7系列（7FD）','engine','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8系列（8FD）','transmission','自波'),
  ('丰田','8系列（8FD）','transmission','无'),
  ('丰田','8系列（8FD）','engine','进口发动机'),
  ('丰田','8系列（8FD）','engine','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','Z系列（本土化）','transmission','手波'),
  ('丰田','Z系列（本土化）','transmission','自波'),
  ('丰田','Z系列（本土化）','transmission','无'),
  ('丰田','Z系列（本土化）','engine','进口发动机'),
  ('丰田','Z系列（本土化）','engine','无')
ON CONFLICT DO NOTHING;

-- ----- 丰田 电动 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8系列（8FBN）','battery','铅酸'),
  ('丰田','8系列（8FBN）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8系列（8FBE）','battery','铅酸'),
  ('丰田','8系列（8FBE）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8系列（8FBR）','battery','铅酸'),
  ('丰田','8系列（8FBR）','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 林德 电动 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','E系列（E16-E20）','battery','磷酸铁锂(LFP)'),
  ('林德','E系列（E16-E20）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','E系列（E25-E30）','battery','磷酸铁锂(LFP)'),
  ('林德','E系列（E25-E30）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','E系列（E35-E50）','battery','铅酸'),
  ('林德','E系列（E35-E50）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','R系列（R14-R20）','battery','铅酸'),
  ('林德','R系列（R14-R20）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','R系列（R14-R25）','battery','铅酸'),
  ('林德','R系列（R14-R25）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','L系列','battery','铅酸'),
  ('林德','L系列','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 龙工 内燃 series（国产发动机）-----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','CPCD系列','transmission','手波'),
  ('龙工','CPCD系列','transmission','自波'),
  ('龙工','CPCD系列','transmission','无'),
  ('龙工','CPCD系列','engine','国产发动机'),
  ('龙工','CPCD系列','engine','无')
ON CONFLICT DO NOTHING;

-- ----- 龙工 电动 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','N系列','battery','铅酸'),
  ('龙工','N系列','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','CDD系列','battery','铅酸'),
  ('龙工','CDD系列','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 柳工 内燃 series（国产发动机）-----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('柳工','E系列','transmission','自波'),
  ('柳工','E系列','transmission','无'),
  ('柳工','E系列','engine','国产发动机'),
  ('柳工','E系列','engine','无')
ON CONFLICT DO NOTHING;

-- 柳工 CLG系列（电动变体）：补 battery 维度（000010 已有 transmission+engine）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('柳工','CLG系列','battery','铅酸'),
  ('柳工','CLG系列','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 比亚迪 电动 series -----
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('比亚迪','CPD系列（大吨位）','battery','磷酸铁锂(LFP)'),
  ('比亚迪','CPD系列（大吨位）','battery','无')
ON CONFLICT DO NOTHING;

INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('比亚迪','S系列','battery','铅酸'),
  ('比亚迪','S系列','battery','无')
ON CONFLICT DO NOTHING;

-- =====================================================
-- 4. original_prices 数据（旧 series，来源：网络公开数据）
--    覆盖 Excel 未涉及的 series，如永恒力、宝骊、中力、斗山、海斯特、中联重科等
-- =====================================================

-- ----- 4.1 电动平衡重式 -----

-- 林德 E系列（进口一线，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '林德', '电动平衡重式', 'E系列', 1.6, '磷酸铁锂(LFP)', '两级门架', 3000, 266500, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 280000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列', 2.0, '铅酸', '两级门架', 3000, 240000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 295000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 302200, TRUE)
ON CONFLICT DO NOTHING;

-- 林德 Xi系列（进口一线，锂电专用）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '林德', '电动平衡重式', 'Xi系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 255000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'Xi系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 275000, TRUE)
ON CONFLICT DO NOTHING;

-- 丰田 8FBE系列（进口一线，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 2.0, '铅酸', '两级门架', 3000, 210000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 245000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 3.0, '铅酸', '三级门架', 4500, 265000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 300000, TRUE)
ON CONFLICT DO NOTHING;

-- 永恒力 EFG系列（进口一线，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 1.5, '铅酸', '两级门架', 3000, 158000, TRUE),
  ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 2.5, '铅酸', '两级门架', 3000, 185000, TRUE),
  ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 215000, TRUE),
  ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 245000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 X系列（国产一线，锂电专用）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '电动平衡重式', 'X系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 88000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'X系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 105000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（国产一线，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '电动平衡重式', 'CPD系列', 1.5, '铅酸', '两级门架', 3000, 98000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'CPD系列', 2.0, '铅酸', '两级门架', 3000, 115000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 132000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'CPD系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 128000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'CPD系列', 3.5, '磷酸铁锂(LFP)', '三级门架', 4500, 165000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 XH系列（国产一线，锂电高配）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动平衡重式', 'XH系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 92000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XH系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 4500, 125000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 XC系列（国产一线，锂电）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动平衡重式', 'XC系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 75000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XC系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 88000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 XE系列（国产一线，锂电）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动平衡重式', 'XE系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 78000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XE系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 95000, TRUE)
ON CONFLICT DO NOTHING;

-- 比亚迪 CPD系列（国产一线，LFP 专用）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 1.6, '磷酸铁锂(LFP)', '两级门架', 3000, 92000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 105000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 118000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 138000, TRUE)
ON CONFLICT DO NOTHING;

-- 宝骊 KBE系列（国产其他，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产其他', '宝骊', '电动平衡重式', 'KBE系列', 2.0, '铅酸', '两级门架', 3000, 62000, TRUE),
  ('国产其他', '宝骊', '电动平衡重式', 'KBE系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 78000, TRUE),
  ('国产其他', '宝骊', '电动平衡重式', 'KBE系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 88000, TRUE)
ON CONFLICT DO NOTHING;

-- ----- 4.2 电动前移式 -----

-- 丰田 Traigo系列（进口一线，锂电专用）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '丰田', '电动前移式', 'Traigo系列', 2.0, '磷酸铁锂(LFP)', '三级门架', 6000, 265000, TRUE),
  ('进口一线', '丰田', '电动前移式', 'Traigo系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 6000, 295000, TRUE)
ON CONFLICT DO NOTHING;

-- 永恒力 ETV系列（进口一线，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '永恒力', '电动前移式', 'ETV系列', 2.0, '铅酸', '三级门架', 5000, 229600, TRUE),
  ('进口一线', '永恒力', '电动前移式', 'ETV系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 6000, 280000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（国产一线，电动前移式）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '电动前移式', 'CPD系列', 2.0, '铅酸', '三级门架', 6000, 135000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（国产一线，电动前移式）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动前移式', 'A系列', 2.0, '铅酸', '三级门架', 6000, 150000, TRUE)
ON CONFLICT DO NOTHING;

-- 比亚迪 CPD系列（国产一线，电动前移式，LFP）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '比亚迪', '电动前移式', 'CPD系列', 1.5, '磷酸铁锂(LFP)', '三级门架', 5500, 120000, TRUE)
ON CONFLICT DO NOTHING;

-- ----- 4.3 电动托盘搬运车 -----

-- 中力 EPT系列（国产其他，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 1.5, '铅酸', '无', 0, 9900, TRUE),
  ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 2.0, '铅酸', '无', 0, 15000, TRUE),
  ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 2.0, '磷酸铁锂(LFP)', '无', 0, 17000, TRUE),
  ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 3.0, '磷酸铁锂(LFP)', '无', 0, 22000, TRUE)
ON CONFLICT DO NOTHING;

-- 中力 无（兜底系列，铅酸为主）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产其他', '中力', '电动托盘搬运车', '无', 1.5, '铅酸', '无', 0, 9500, TRUE),
  ('国产其他', '中力', '电动托盘搬运车', '无', 2.0, '铅酸', '无', 0, 14000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（国产一线，电动托盘搬运车，LFP）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动托盘搬运车', 'A系列', 2.0, '磷酸铁锂(LFP)', '无', 0, 28000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（国产一线，电动托盘搬运车，铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '电动托盘搬运车', 'CPD系列', 2.0, '铅酸', '无', 0, 25000, TRUE)
ON CONFLICT DO NOTHING;

-- 永恒力 EFG系列（进口一线，电动托盘搬运车，铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '永恒力', '电动托盘搬运车', 'EFG系列', 1.5, '铅酸', '无', 0, 35000, TRUE)
ON CONFLICT DO NOTHING;

-- ----- 4.4 电动堆高车 -----

-- 中力 ES系列（国产其他，LFP + 铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产其他', '中力', '电动堆高车', 'ES系列', 1.5, '铅酸', '两级门架', 3000, 21800, TRUE),
  ('国产其他', '中力', '电动堆高车', 'ES系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 25000, TRUE),
  ('国产其他', '中力', '电动堆高车', 'ES系列', 2.0, '铅酸', '两级门架', 3500, 52000, TRUE)
ON CONFLICT DO NOTHING;

-- 永恒力 ERIC系列（进口一线，铅酸 + LFP）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '永恒力', '电动堆高车', 'ERIC系列', 1.5, '铅酸', '两级门架', 3000, 45000, TRUE),
  ('进口一线', '永恒力', '电动堆高车', 'ERIC系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 55000, TRUE),
  ('进口一线', '永恒力', '电动堆高车', 'ERIC系列', 2.0, '铅酸', '两级门架', 3500, 62000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（国产一线，电动堆高车，LFP）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '电动堆高车', 'A系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3300, 55000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（国产一线，电动堆高车，铅酸）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '电动堆高车', 'CPD系列', 1.5, '铅酸', '两级门架', 3000, 32000, TRUE)
ON CONFLICT DO NOTHING;

-- ----- 4.5 内燃平衡重式 -----

-- 林德 H系列（进口一线，手波/自波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 2.0, '手波/进口发动机', '两级门架', 3000, 155000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 2.5, '手波/进口发动机', '两级门架', 3000, 168000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 2.5, '自波/进口发动机', '两级门架', 3000, 192000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 3.5, '手波/进口发动机', '三级门架', 4000, 195000, TRUE)
ON CONFLICT DO NOTHING;

-- 林德 T-MATIC（进口一线，自波专用 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '林德', '内燃平衡重式', 'T-MATIC', 2.0, '自波/进口发动机', '两级门架', 3000, 178000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'T-MATIC', 3.0, '自波/进口发动机', '三级门架', 4500, 215000, TRUE)
ON CONFLICT DO NOTHING;

-- 丰田 8FD系列（进口一线，手波/自波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口一线', '丰田', '内燃平衡重式', '8FD系列', 2.5, '手波/进口发动机', '两级门架', 3000, 168000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', '8FD系列', 2.5, '自波/进口发动机', '两级门架', 3000, 192000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', '8FD系列', 3.5, '手波/进口发动机', '三级门架', 4000, 195000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 A系列（国产一线，手波/自波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 72000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'A系列', 2.5, '手波/国产发动机', '两级门架', 3000, 80000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'A系列', 2.5, '自波/国产发动机', '两级门架', 3000, 92000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'A系列', 3.5, '手波/国产发动机', '三级门架', 4000, 102000, TRUE)
ON CONFLICT DO NOTHING;

-- 合力 CPCD系列（国产一线，手波 + 国产/进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 2.0, '手波/国产发动机', '两级门架', 3000, 75000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 2.5, '手波/国产发动机', '两级门架', 3000, 82000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 2.5, '手波/进口发动机', '两级门架', 3000, 105000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 3.5, '手波/国产发动机', '三级门架', 4000, 105000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（国产一线，手波/自波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 68000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.5, '手波/国产发动机', '两级门架', 3000, 75000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.5, '自波/国产发动机', '两级门架', 3000, 86000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.5, '手波/国产发动机', '三级门架', 4000, 95000, TRUE)
ON CONFLICT DO NOTHING;

-- 杭叉 XF系列（国产一线，手波 + 国产/进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '杭叉', '内燃平衡重式', 'XF系列', 2.5, '手波/国产发动机', '两级门架', 3000, 82000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'XF系列', 2.5, '手波/进口发动机', '两级门架', 3000, 105000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'XF系列', 3.5, '手波/国产发动机', '三级门架', 4000, 102000, TRUE)
ON CONFLICT DO NOTHING;

-- 斗山 B30S系列（进口二线，手波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口二线', '斗山', '内燃平衡重式', 'B30S系列', 2.5, '手波/进口发动机', '两级门架', 3000, 145000, TRUE),
  ('进口二线', '斗山', '内燃平衡重式', 'B30S系列', 3.0, '手波/进口发动机', '三级门架', 4500, 165000, TRUE)
ON CONFLICT DO NOTHING;

-- 斗山 BR系列（进口二线，手波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口二线', '斗山', '内燃平衡重式', 'BR系列', 2.5, '手波/进口发动机', '两级门架', 3000, 148000, TRUE),
  ('进口二线', '斗山', '内燃平衡重式', 'BR系列', 3.0, '手波/进口发动机', '三级门架', 4500, 168000, TRUE)
ON CONFLICT DO NOTHING;

-- 海斯特 H系列（进口二线，手波/自波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口二线', '海斯特', '内燃平衡重式', 'H系列', 2.5, '手波/进口发动机', '两级门架', 3000, 142000, TRUE),
  ('进口二线', '海斯特', '内燃平衡重式', 'H系列', 3.0, '手波/进口发动机', '三级门架', 4500, 162000, TRUE),
  ('进口二线', '海斯特', '内燃平衡重式', 'H系列', 3.0, '自波/进口发动机', '三级门架', 4500, 185000, TRUE)
ON CONFLICT DO NOTHING;

-- 海斯特 J系列（进口二线，手波 + 进口发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('进口二线', '海斯特', '内燃平衡重式', 'J系列', 3.0, '手波/进口发动机', '三级门架', 4500, 158000, TRUE),
  ('进口二线', '海斯特', '内燃平衡重式', 'J系列', 5.0, '手波/进口发动机', '三级门架', 4500, 220000, TRUE)
ON CONFLICT DO NOTHING;

-- 龙工 A系列（国产二线，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产二线', '龙工', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 58000, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'A系列', 3.0, '手波/国产发动机', '两级门架', 3000, 72000, TRUE)
ON CONFLICT DO NOTHING;

-- 龙工 LG系列（国产二线，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 2.0, '手波/国产发动机', '两级门架', 3000, 62000, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 2.5, '手波/国产发动机', '两级门架', 3000, 68000, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 5.0, '手波/国产发动机', '三级门架', 4500, 128000, TRUE)
ON CONFLICT DO NOTHING;

-- 柳工 CLG系列（国产二线，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产二线', '柳工', '内燃平衡重式', 'CLG系列', 2.5, '手波/国产发动机', '两级门架', 3000, 65000, TRUE),
  ('国产二线', '柳工', '内燃平衡重式', 'CLG系列', 3.0, '手波/国产发动机', '两级门架', 3000, 75000, TRUE)
ON CONFLICT DO NOTHING;

-- ----- 4.6 内燃重型叉车 -----

-- 合力 CPCD系列（国产一线，大吨位，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产一线', '合力', '内燃重型叉车', 'CPCD系列', 10.0, '手波/国产发动机', '三级门架', 4500, 380000, TRUE),
  ('国产一线', '合力', '内燃重型叉车', 'CPCD系列', 15.0, '手波/国产发动机', '三级门架', 4500, 450000, TRUE)
ON CONFLICT DO NOTHING;

-- 龙工 LG系列（国产二线，大吨位，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产二线', '龙工', '内燃重型叉车', 'LG系列', 12.0, '手波/国产发动机', '三级门架', 4500, 350000, TRUE)
ON CONFLICT DO NOTHING;

-- 中联重科 FD系列（国产二线，大吨位，手波 + 国产发动机）
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price, is_active) VALUES
  ('国产二线', '中联重科', '内燃重型叉车', 'FD系列', 10.0, '手波/国产发动机', '三级门架', 4500, 365000, TRUE),
  ('国产二线', '中联重科', '内燃重型叉车', 'FD系列', 16.0, '手波/国产发动机', '三级门架', 4500, 520000, TRUE)
ON CONFLICT DO NOTHING;

-- =====================================================
-- 5. original_prices 数据（Excel 价格对照表，92行）
--    数据来源：叉车价格对照表.xlsx
--    价格：取范围中位数 ×10000 转元
-- =====================================================

INSERT INTO original_prices (
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, original_price, is_active
) VALUES
  -- ========== 合力 ==========
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 2.0, '手波/国产发动机', '两级标准门架', 3000, 50000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 3.0, '手波/国产发动机', '两级标准门架', 3000, 57000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 3.0, '自波/国产发动机', '两级标准门架', 3000, 63000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 3.0, '手波/国产发动机', '三级全自由门架', 4500, 65000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 3.5, '自波/国产发动机', '三级全自由门架', 4500, 70000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K系列（K1）', 3.5, '无级变速/国产发动机', '两级标准门架', 3000, 100000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K2系列（大力士）', 5.0, '自波/国产发动机', '两级标准门架', 3000, 122500, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K2系列（大力士）', 5.0, '自波/国产发动机', '三级全自由门架', 4500, 132500, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K2系列（大力士）', 7.0, '自波/国产发动机', '两级标准门架', 3000, 170000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'K2系列（大力士）', 10.0, '自波/国产发动机', '两级标准门架', 3000, 235000, TRUE),
  ('国产一线', '合力', '内燃平衡重式', 'G2系列', 20.0, '自波/国产发动机', '两级标准门架', 3000, 500000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H3系列', 1.5, '铅酸', '两级标准门架', 3000, 82500, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H3系列', 2.0, '铅酸', '两级标准门架', 3000, 92500, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H3系列', 3.0, '铅酸', '两级标准门架', 3000, 115000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H3系列', 3.0, '铅酸', '三级全自由门架', 4500, 125000, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H4系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 132500, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'H4系列', 3.5, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 152500, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'K3系列（锂电专用）', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500, TRUE),
  ('国产一线', '合力', '电动平衡重式', 'G3系列', 6.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000, TRUE),
  ('国产一线', '合力', '电动前移式', '前移式', 1.5, '铅酸', '三级门架', 6000, 165000, TRUE),
  ('国产一线', '合力', '电动前移式', '前移式', 2.0, '铅酸', '四级门架', 9500, 240000, TRUE),

  -- ========== 杭叉 ==========
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级标准门架', 3000, 46000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 54000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.0, '自波/国产发动机', '两级标准门架', 3000, 60500, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.0, '自波/国产发动机', '三级全自由门架', 4500, 63000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.5, '自波/国产发动机', '三级全自由门架', 4500, 70000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 112500, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 7.0, '自波/国产发动机', '两级标准门架', 3000, 160000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'XA系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 120000, TRUE),
  ('国产一线', '杭叉', '内燃平衡重式', 'XA系列', 10.0, '自波/国产发动机', '两级标准门架', 3000, 215000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'A系列', 1.5, '铅酸', '两级标准门架', 3000, 77500, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'A系列', 2.0, '铅酸', '两级标准门架', 3000, 87500, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'A系列', 3.0, '铅酸', '两级标准门架', 3000, 105000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'A系列', 3.0, '铅酸', '三级全自由门架', 4500, 115000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XC系列（锂电专用）', 2.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XC系列（锂电专用）', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 122500, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XC系列（锂电专用）', 3.5, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 142500, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'XH系列（高压锂电）', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500, TRUE),
  ('国产一线', '杭叉', '电动平衡重式', 'J系列（大吨位）', 5.0, '铅酸', '两级标准门架', 3000, 200000, TRUE),
  ('国产一线', '杭叉', '电动前移式', 'X系列', 1.5, '铅酸', '三级门架', 6000, 155000, TRUE),
  ('国产一线', '杭叉', '电动前移式', 'X系列', 2.0, '铅酸', '四级门架', 10000, 225000, TRUE),
  ('国产一线', '杭叉', '电动堆高车', 'A系列（经济型）', 1.5, '铅酸', '两级门架', 3000, 30000, TRUE),
  ('国产一线', '杭叉', '电动堆高车', 'A系列', 2.0, '铅酸', '三级门架', 4500, 65000, TRUE),

  -- ========== 丰田 ==========
  ('进口一线', '丰田', '内燃平衡重式', '7系列（7FD）', 3.0, '自波/进口发动机', '两级标准门架', 3000, 200000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', '8系列（8FD）', 3.0, '自波/进口发动机', '两级标准门架', 3000, 225000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', '8系列（8FD）', 3.0, '自波/进口发动机', '三级全自由门架', 4500, 245000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', '8系列（8FD）', 5.0, '自波/进口发动机', '两级标准门架', 3000, 315000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', 'Z系列（本土化）', 3.0, '手波/进口发动机', '两级标准门架', 3000, 135000, TRUE),
  ('进口一线', '丰田', '内燃平衡重式', 'Z系列（本土化）', 3.0, '自波/进口发动机', '三级全自由门架', 4500, 155000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8系列（8FBN）', 1.5, '铅酸', '两级标准门架', 3000, 220000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8系列（8FBN）', 2.0, '铅酸', '两级标准门架', 3000, 255000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8系列（8FBN）', 3.0, '铅酸', '两级标准门架', 3000, 290000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8系列（8FBN）', 3.0, '铅酸', '三级全自由门架', 4500, 310000, TRUE),
  ('进口一线', '丰田', '电动平衡重式', '8系列（8FBE）', 1.5, '铅酸', '两级标准门架', 3000, 200000, TRUE),
  ('进口一线', '丰田', '电动前移式', '8系列（8FBR）', 1.5, '铅酸', '三级门架', 6000, 305000, TRUE),
  ('进口一线', '丰田', '电动前移式', '8系列（8FBR）', 2.0, '铅酸', '四级门架', 11300, 385000, TRUE),

  -- ========== 林德 ==========
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 3.0, '无级变速/进口发动机', '两级宽视野门架', 3000, 275000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 3.0, '无级变速/进口发动机', '三级全自由门架', 4500, 305000, TRUE),
  ('进口一线', '林德', '内燃平衡重式', 'H系列', 5.0, '无级变速/进口发动机', '两级宽视野门架', 3000, 385000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列（E16-E20）', 1.6, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 240000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列（E25-E30）', 3.0, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 310000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列（E25-E30）', 3.0, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 340000, TRUE),
  ('进口一线', '林德', '电动平衡重式', 'E系列（E35-E50）', 5.0, '铅酸', '两级宽视野门架', 3250, 440000, TRUE),
  ('进口一线', '林德', '电动前移式', 'R系列（R14-R20）', 1.6, '铅酸', '三级门架', 6000, 330000, TRUE),
  ('进口一线', '林德', '电动前移式', 'R系列（R14-R25）', 2.0, '铅酸', '四级HD门架', 12000, 450000, TRUE),
  ('进口一线', '林德', '电动堆高车', 'L系列', 1.6, '铅酸', '三级门架', 4500, 140000, TRUE),

  -- ========== 龙工 ==========
  ('国产二线', '龙工', '内燃平衡重式', 'CPCD系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 48500, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'CPCD系列', 3.0, '自波/国产发动机', '两级标准门架', 3000, 54000, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'CPCD系列', 3.5, '自波/国产发动机', '三级门架', 4500, 63000, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'CPCD系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 97500, TRUE),
  ('国产二线', '龙工', '内燃平衡重式', 'CPCD系列', 10.0, '自波/国产发动机', '三级门架', 4500, 195000, TRUE),
  ('国产二线', '龙工', '电动平衡重式', 'N系列', 1.5, '铅酸', '两级标准门架', 3000, 67500, TRUE),
  ('国产二线', '龙工', '电动平衡重式', 'N系列', 2.0, '铅酸', '两级标准门架', 3000, 77500, TRUE),
  ('国产二线', '龙工', '电动平衡重式', 'N系列', 3.0, '铅酸', '三级全自由门架', 4500, 100000, TRUE),
  ('国产二线', '龙工', '电动堆高车', 'CDD系列', 1.5, '铅酸', '两级门架', 3000, 21500, TRUE),
  ('国产二线', '龙工', '电动堆高车', 'CDD系列', 2.0, '铅酸', '三级门架', 4500, 47500, TRUE),

  -- ========== 柳工 ==========
  ('国产二线', '柳工', '内燃平衡重式', 'CLG系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 51500, TRUE),
  ('国产二线', '柳工', '内燃平衡重式', 'CLG系列', 3.0, '自波/国产发动机', '三级全自由门架', 4500, 60000, TRUE),
  ('国产二线', '柳工', '内燃平衡重式', 'CLG系列', 3.5, '自波/国产发动机', '三级门架', 4500, 52500, TRUE),
  ('国产二线', '柳工', '内燃平衡重式', 'E系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 110000, TRUE),
  ('国产二线', '柳工', '内燃平衡重式', 'E系列', 7.0, '自波/国产发动机', '两级标准门架', 3000, 152500, TRUE),
  ('国产二线', '柳工', '电动平衡重式', 'CLG系列', 2.0, '铅酸', '两级标准门架', 3000, 77500, TRUE),
  ('国产二线', '柳工', '电动平衡重式', 'CLG系列', 3.0, '铅酸', '三级门架', 4500, 100000, TRUE),

  -- ========== 比亚迪 ==========
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 1.5, '磷酸铁锂(LFP)', '两级标准门架', 3000, 62500, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 72500, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 90000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 100000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 3.5, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 5.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 165000, TRUE),
  ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 7.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000, TRUE),
  ('国产一线', '比亚迪', '电动堆高车', 'S系列', 1.5, '铅酸', '两级门架', 3000, 24000, TRUE),
  ('国产一线', '比亚迪', '电动堆高车', 'S系列', 2.0, '铅酸', '三级门架', 4500, 52500, TRUE)
ON CONFLICT DO NOTHING;

-- =====================================================
-- 6. "无" 配置兜底记录
--    对每个 (brand, vehicle_type, series) 组合，若尚无含 "无" 的 config_type 记录，
--    则补一条最低原价的兜底记录，保证级联过滤在 config_type/mast_type/mast_height 步骤不断链。
--    电动 series：config_type = '无'
--    内燃 series：config_type = '无/无'
-- =====================================================
INSERT INTO original_prices (
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, original_price, is_active
)
SELECT
    op.brand_type, op.brand, op.vehicle_type, op.series, MIN(op.tonnage),
    CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END,
    '无', 0, MIN(op.original_price), TRUE
FROM original_prices op
WHERE NOT EXISTS (
    SELECT 1 FROM original_prices p2
    WHERE p2.brand_type = op.brand_type
      AND p2.brand = op.brand
      AND p2.vehicle_type = op.vehicle_type
      AND p2.series = op.series
      AND p2.config_type = (CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END)
)
GROUP BY op.brand_type, op.brand, op.vehicle_type, op.series, (CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END)
ON CONFLICT DO NOTHING;
