-- ============================================================
-- 000012 DOWN: 回滚系列去重 + "其它"→"无" 恢复
-- 注意：合并后删除的重复 original_prices 记录无法恢复
-- ============================================================

-- ============================================================
-- Part 1: 恢复被合并的子系列行（series 表）
-- ============================================================

-- 1.1 林德 电动平衡重式 子系列
INSERT INTO series (brand, name, earliest_factory_year) VALUES
('林德', 'E系列（E16-E20）', 2010),
('林德', 'E系列（E25-E30）', 2010),
('林德', 'E系列（E35-E50）', 2010)
ON CONFLICT (brand, name) DO NOTHING;

-- 1.2 丰田 电动平衡重式 子系列
INSERT INTO series (brand, name, earliest_factory_year) VALUES
('丰田', '8系列（8FBE）', 2010)
ON CONFLICT (brand, name) DO NOTHING;

-- 1.3 丰田 内燃平衡重式 子系列
INSERT INTO series (brand, name, earliest_factory_year) VALUES
('丰田', '8系列（8FD）', 2010)
ON CONFLICT (brand, name) DO NOTHING;

-- 1.4 杭叉 电动平衡重式 子系列
INSERT INTO series (brand, name, earliest_factory_year) VALUES
('杭叉', 'XC系列（锂电专用）', 2015),
('杭叉', 'XH系列（高压锂电）', 2015)
ON CONFLICT (brand, name) DO NOTHING;

-- 1.5 林德 电动前移式 子系列（删除合并后的 R系列，恢复原子系列）
-- 注意：R系列下的 original_prices 无法拆分回子系列，仅恢复 series 表行
DELETE FROM series_config_options WHERE brand = '林德' AND series = 'R系列';
DELETE FROM series WHERE brand = '林德' AND name = 'R系列';

INSERT INTO series (brand, name, earliest_factory_year) VALUES
('林德', 'R系列（R14-R20）', 2010),
('林德', 'R系列（R14-R25）', 2010)
ON CONFLICT (brand, name) DO NOTHING;

-- 恢复 R系列子系列的 series_config_options
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
('林德', 'R系列（R14-R20）', 'battery', '无'),
('林德', 'R系列（R14-R20）', 'battery', '铅酸'),
('林德', 'R系列（R14-R25）', 'battery', '无'),
('林德', 'R系列（R14-R25）', 'battery', '铅酸')
ON CONFLICT (brand, series, dimension, option_name) DO NOTHING;

-- ============================================================
-- Part 2: 恢复其他子系列的 series_config_options
-- ============================================================
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
-- 林德 E系列子系列
('林德', 'E系列（E16-E20）', 'battery', '无'),
('林德', 'E系列（E16-E20）', 'battery', '磷酸铁锂(LFP)'),
('林德', 'E系列（E25-E30）', 'battery', '无'),
('林德', 'E系列（E25-E30）', 'battery', '磷酸铁锂(LFP)'),
('林德', 'E系列（E35-E50）', 'battery', '无'),
('林德', 'E系列（E35-E50）', 'battery', '铅酸'),
-- 丰田 8系列子系列
('丰田', '8系列（8FBE）', 'battery', '无'),
('丰田', '8系列（8FBE）', 'battery', '铅酸'),
('丰田', '8系列（8FD）', 'engine', '无'),
('丰田', '8系列（8FD）', 'engine', '进口发动机'),
('丰田', '8系列（8FD）', 'transmission', '无'),
('丰田', '8系列（8FD）', 'transmission', '自波'),
-- 杭叉 子系列
('杭叉', 'XC系列（锂电专用）', 'battery', '无'),
('杭叉', 'XC系列（锂电专用）', 'battery', '磷酸铁锂(LFP)'),
('杭叉', 'XH系列（高压锂电）', 'battery', '无'),
('杭叉', 'XH系列（高压锂电）', 'battery', '磷酸铁锂(LFP)')
ON CONFLICT (brand, series, dimension, option_name) DO NOTHING;

-- ============================================================
-- Part 3: 系列 "其它" → "无" 恢复
-- ============================================================
UPDATE series SET name = '无' WHERE name = '其它';
UPDATE original_prices SET series = '无' WHERE series = '其它';
UPDATE series_config_options SET series = '无' WHERE series = '其它';
