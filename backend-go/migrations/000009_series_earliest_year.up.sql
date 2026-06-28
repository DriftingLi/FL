-- 000009_series_earliest_year.up.sql
-- series 表添加 earliest_factory_year 字段
-- 每个系列的最早发布年份不同，用于级联限制出厂年份选择
-- 级联顺序调整：品牌 → 车辆类型 → 系列 → 吨位 → 出厂年份

-- =====================================================
-- 1. 添加列（默认 2000，后续按系列实际发布年份更新）
-- =====================================================
ALTER TABLE series
    ADD COLUMN IF NOT EXISTS earliest_factory_year INTEGER NOT NULL DEFAULT 2000;

-- =====================================================
-- 2. 按品牌+系列名设置最早出厂年份
--    数据来源：各品牌官方资料/产品发布历史
-- =====================================================

-- 林德（进口一线）
UPDATE series SET earliest_factory_year = 2015 WHERE brand = '林德' AND name = 'E系列';
UPDATE series SET earliest_factory_year = 2018 WHERE brand = '林德' AND name = 'Xi系列';
UPDATE series SET earliest_factory_year = 2006 WHERE brand = '林德' AND name = 'H系列';
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '林德' AND name = 'T-MATIC';

-- 丰田（进口一线）
UPDATE series SET earliest_factory_year = 2013 WHERE brand = '丰田' AND name = '8FBE系列';
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '丰田' AND name = '8FD系列';
UPDATE series SET earliest_factory_year = 2016 WHERE brand = '丰田' AND name = 'Traigo系列';

-- 永恒力（进口一线）
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '永恒力' AND name = 'EFG系列';
UPDATE series SET earliest_factory_year = 2012 WHERE brand = '永恒力' AND name = 'ETV系列';
UPDATE series SET earliest_factory_year = 2014 WHERE brand = '永恒力' AND name = 'ERIC系列';

-- 合力（国产一线）
UPDATE series SET earliest_factory_year = 2005 WHERE brand = '合力' AND name = 'A系列';
UPDATE series SET earliest_factory_year = 2015 WHERE brand = '合力' AND name = 'X系列';
UPDATE series SET earliest_factory_year = 1998 WHERE brand = '合力' AND name = 'CPCD系列';
UPDATE series SET earliest_factory_year = 2005 WHERE brand = '合力' AND name = 'CPD系列';

-- 杭叉（国产一线）
UPDATE series SET earliest_factory_year = 2008 WHERE brand = '杭叉' AND name = 'A系列';
UPDATE series SET earliest_factory_year = 2018 WHERE brand = '杭叉' AND name = 'XH系列';
UPDATE series SET earliest_factory_year = 2016 WHERE brand = '杭叉' AND name = 'XC系列';
UPDATE series SET earliest_factory_year = 2017 WHERE brand = '杭叉' AND name = 'XE系列';
UPDATE series SET earliest_factory_year = 2019 WHERE brand = '杭叉' AND name = 'XF系列';

-- 比亚迪（国产一线）
UPDATE series SET earliest_factory_year = 2013 WHERE brand = '比亚迪' AND name = 'CPD系列';

-- 斗山（进口二线）
UPDATE series SET earliest_factory_year = 2008 WHERE brand = '斗山' AND name = 'B30S系列';
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '斗山' AND name = 'BR系列';

-- 海斯特（进口二线）
UPDATE series SET earliest_factory_year = 2006 WHERE brand = '海斯特' AND name = 'H系列';
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '海斯特' AND name = 'J系列';

-- 龙工（国产二线）
UPDATE series SET earliest_factory_year = 2008 WHERE brand = '龙工' AND name = 'A系列';
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '龙工' AND name = 'LG系列';

-- 柳工（国产二线）
UPDATE series SET earliest_factory_year = 2012 WHERE brand = '柳工' AND name = 'CLG系列';

-- 中联重科（国产二线）
UPDATE series SET earliest_factory_year = 2010 WHERE brand = '中联重科' AND name = 'FD系列';

-- 宝骊（国产其他）
UPDATE series SET earliest_factory_year = 2008 WHERE brand = '宝骊' AND name = 'KBE系列';

-- 中力（国产其他）
UPDATE series SET earliest_factory_year = 2013 WHERE brand = '中力' AND name = 'EPT系列';
UPDATE series SET earliest_factory_year = 2015 WHERE brand = '中力' AND name = 'ES系列';

-- =====================================================
-- 3. "无" 系列设为全局最低年份（1980），表示无具体系列限制
-- =====================================================
UPDATE series SET earliest_factory_year = 1980 WHERE name = '无';
