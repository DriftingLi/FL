-- 000010_config_type_restructure.up.sql
-- 配置类型重构：电池类型并入配置类型
-- config_type 由三维度拼接：传动系统/发动机类型/电池配置（用 / 分隔，不支持的维度省略）
-- 新增 series_config_options 表定义每个 series 支持的维度及可选项
-- 移除 original_prices 和 evaluations 表的 battery_type 字段

-- =====================================================
-- 1. 新增维度字典表：transmission_types
-- =====================================================
CREATE TABLE IF NOT EXISTS transmission_types (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) UNIQUE NOT NULL
);

INSERT INTO transmission_types (name) VALUES
  ('手波'), ('自波'), ('无级变速'), ('无')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 2. 新增维度字典表：engine_types
-- =====================================================
CREATE TABLE IF NOT EXISTS engine_types (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) UNIQUE NOT NULL
);

INSERT INTO engine_types (name) VALUES
  ('国产发动机'), ('进口发动机'), ('混合动力'), ('无')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 3. battery_types 追加 "无" 选项（作为电池维度字典）
-- =====================================================
INSERT INTO battery_types (name) VALUES ('无') ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 4. 新增 series_config_options 表（规范化：series + 维度 + 选项）
-- =====================================================
CREATE TABLE IF NOT EXISTS series_config_options (
    id           SERIAL PRIMARY KEY,
    brand        VARCHAR(50) NOT NULL,
    series       VARCHAR(50) NOT NULL,
    dimension    VARCHAR(20) NOT NULL,   -- 'transmission' / 'engine' / 'battery'
    option_name  VARCHAR(50) NOT NULL,
    UNIQUE (brand, series, dimension, option_name)
);
CREATE INDEX IF NOT EXISTS idx_sco_lookup ON series_config_options(brand, series);

-- =====================================================
-- 5. 初始化 series_config_options 数据
--    规则：电动 series 仅 battery 维度；内燃 series 仅 transmission + engine 维度
--    进口品牌内燃 engine 含"进口发动机"+"无"；国产品牌内燃 engine 含"国产发动机"+"进口发动机"+"无"
-- =====================================================

-- ----- 电动 series（仅 battery 维度）-----

-- 林德 E系列（电动平衡重式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','E系列','battery','磷酸铁锂(LFP)'),
  ('林德','E系列','battery','铅酸'),
  ('林德','E系列','battery','无')
ON CONFLICT DO NOTHING;

-- 林德 Xi系列（电动，锂电专用）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','Xi系列','battery','磷酸铁锂(LFP)'),
  ('林德','Xi系列','battery','无')
ON CONFLICT DO NOTHING;

-- 丰田 8FBE系列（电动平衡重式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8FBE系列','battery','磷酸铁锂(LFP)'),
  ('丰田','8FBE系列','battery','铅酸'),
  ('丰田','8FBE系列','battery','无')
ON CONFLICT DO NOTHING;

-- 丰田 Traigo系列（电动前移式，锂电专用）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','Traigo系列','battery','磷酸铁锂(LFP)'),
  ('丰田','Traigo系列','battery','无')
ON CONFLICT DO NOTHING;

-- 永恒力 EFG系列（电动平衡重式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('永恒力','EFG系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','EFG系列','battery','铅酸'),
  ('永恒力','EFG系列','battery','无')
ON CONFLICT DO NOTHING;

-- 永恒力 ETV系列（电动前移式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('永恒力','ETV系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','ETV系列','battery','铅酸'),
  ('永恒力','ETV系列','battery','无')
ON CONFLICT DO NOTHING;

-- 永恒力 ERIC系列（电动堆高车，经济型铅酸为主）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('永恒力','ERIC系列','battery','铅酸'),
  ('永恒力','ERIC系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','ERIC系列','battery','无')
ON CONFLICT DO NOTHING;

-- 合力 X系列（电动平台，锂电）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','X系列','battery','磷酸铁锂(LFP)'),
  ('合力','X系列','battery','无')
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（电动平衡重式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','CPD系列','battery','磷酸铁锂(LFP)'),
  ('合力','CPD系列','battery','铅酸'),
  ('合力','CPD系列','battery','无')
ON CONFLICT DO NOTHING;

-- 杭叉 XH系列（电动平衡重式，锂电高配）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XH系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XH系列','battery','无')
ON CONFLICT DO NOTHING;

-- 杭叉 XC系列（电动，锂电）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XC系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XC系列','battery','无')
ON CONFLICT DO NOTHING;

-- 杭叉 XE系列（电动平衡重式，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XE系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XE系列','battery','铅酸'),
  ('杭叉','XE系列','battery','无')
ON CONFLICT DO NOTHING;

-- 比亚迪 CPD系列（电动平衡重式，比亚迪自产 LFP）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('比亚迪','CPD系列','battery','磷酸铁锂(LFP)'),
  ('比亚迪','CPD系列','battery','无')
ON CONFLICT DO NOTHING;

-- 宝骊 KBE系列（电动，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('宝骊','KBE系列','battery','磷酸铁锂(LFP)'),
  ('宝骊','KBE系列','battery','铅酸'),
  ('宝骊','KBE系列','battery','无')
ON CONFLICT DO NOTHING;

-- 中力 EPT系列（电动托盘搬运车，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中力','EPT系列','battery','磷酸铁锂(LFP)'),
  ('中力','EPT系列','battery','铅酸'),
  ('中力','EPT系列','battery','无')
ON CONFLICT DO NOTHING;

-- 中力 ES系列（电动堆高车，支持锂电+铅酸）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中力','ES系列','battery','磷酸铁锂(LFP)'),
  ('中力','ES系列','battery','铅酸'),
  ('中力','ES系列','battery','无')
ON CONFLICT DO NOTHING;

-- 中力 无（兜底系列，铅酸为主）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中力','无','battery','铅酸'),
  ('中力','无','battery','磷酸铁锂(LFP)'),
  ('中力','无','battery','无')
ON CONFLICT DO NOTHING;

-- ----- 内燃 series（transmission + engine 维度）-----

-- 林德 H系列（内燃平衡重式，进口动力，手动+自动）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','H系列','transmission','手波'),
  ('林德','H系列','transmission','自波'),
  ('林德','H系列','transmission','无'),
  ('林德','H系列','engine','进口发动机'),
  ('林德','H系列','engine','无')
ON CONFLICT DO NOTHING;

-- 林德 T-MATIC（内燃自动挡专用，进口动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','T-MATIC','transmission','自波'),
  ('林德','T-MATIC','transmission','无'),
  ('林德','T-MATIC','engine','进口发动机'),
  ('林德','T-MATIC','engine','无')
ON CONFLICT DO NOTHING;

-- 丰田 8FD系列（内燃平衡重式，丰田进口发动机，手动+自动）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8FD系列','transmission','手波'),
  ('丰田','8FD系列','transmission','自波'),
  ('丰田','8FD系列','transmission','无'),
  ('丰田','8FD系列','engine','进口发动机'),
  ('丰田','8FD系列','engine','无')
ON CONFLICT DO NOTHING;

-- 合力 A系列（内燃平衡重式，国产/进口发动机可选）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','A系列','transmission','手波'),
  ('合力','A系列','transmission','自波'),
  ('合力','A系列','transmission','无'),
  ('合力','A系列','engine','国产发动机'),
  ('合力','A系列','engine','进口发动机'),
  ('合力','A系列','engine','无')
ON CONFLICT DO NOTHING;

-- 合力 CPCD系列（内燃平衡重式，国产/进口发动机可选）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','CPCD系列','transmission','手波'),
  ('合力','CPCD系列','transmission','自波'),
  ('合力','CPCD系列','transmission','无'),
  ('合力','CPCD系列','engine','国产发动机'),
  ('合力','CPCD系列','engine','进口发动机'),
  ('合力','CPCD系列','engine','无')
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（内燃平衡重式，国产/进口发动机可选）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','A系列','transmission','手波'),
  ('杭叉','A系列','transmission','自波'),
  ('杭叉','A系列','transmission','无'),
  ('杭叉','A系列','engine','国产发动机'),
  ('杭叉','A系列','engine','进口发动机'),
  ('杭叉','A系列','engine','无')
ON CONFLICT DO NOTHING;

-- 杭叉 XF系列（内燃平衡重式，国产/进口发动机可选）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','XF系列','transmission','手波'),
  ('杭叉','XF系列','transmission','自波'),
  ('杭叉','XF系列','transmission','无'),
  ('杭叉','XF系列','engine','国产发动机'),
  ('杭叉','XF系列','engine','进口发动机'),
  ('杭叉','XF系列','engine','无')
ON CONFLICT DO NOTHING;

-- 斗山 B30S系列（内燃平衡重式，进口动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('斗山','B30S系列','transmission','手波'),
  ('斗山','B30S系列','transmission','自波'),
  ('斗山','B30S系列','transmission','无'),
  ('斗山','B30S系列','engine','进口发动机'),
  ('斗山','B30S系列','engine','无')
ON CONFLICT DO NOTHING;

-- 斗山 BR系列（内燃前移式，进口动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('斗山','BR系列','transmission','手波'),
  ('斗山','BR系列','transmission','自波'),
  ('斗山','BR系列','transmission','无'),
  ('斗山','BR系列','engine','进口发动机'),
  ('斗山','BR系列','engine','无')
ON CONFLICT DO NOTHING;

-- 海斯特 H系列（内燃平衡重式，进口动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('海斯特','H系列','transmission','手波'),
  ('海斯特','H系列','transmission','自波'),
  ('海斯特','H系列','transmission','无'),
  ('海斯特','H系列','engine','进口发动机'),
  ('海斯特','H系列','engine','无')
ON CONFLICT DO NOTHING;

-- 海斯特 J系列（内燃平衡重式，进口动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('海斯特','J系列','transmission','手波'),
  ('海斯特','J系列','transmission','自波'),
  ('海斯特','J系列','transmission','无'),
  ('海斯特','J系列','engine','进口发动机'),
  ('海斯特','J系列','engine','无')
ON CONFLICT DO NOTHING;

-- 龙工 A系列（内燃平衡重式，国产动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','A系列','transmission','手波'),
  ('龙工','A系列','transmission','自波'),
  ('龙工','A系列','transmission','无'),
  ('龙工','A系列','engine','国产发动机'),
  ('龙工','A系列','engine','无')
ON CONFLICT DO NOTHING;

-- 龙工 LG系列（内燃平衡重式，国产动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','LG系列','transmission','手波'),
  ('龙工','LG系列','transmission','自波'),
  ('龙工','LG系列','transmission','无'),
  ('龙工','LG系列','engine','国产发动机'),
  ('龙工','LG系列','engine','无')
ON CONFLICT DO NOTHING;

-- 柳工 CLG系列（内燃平衡重式，国产动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('柳工','CLG系列','transmission','手波'),
  ('柳工','CLG系列','transmission','自波'),
  ('柳工','CLG系列','transmission','无'),
  ('柳工','CLG系列','engine','国产发动机'),
  ('柳工','CLG系列','engine','无')
ON CONFLICT DO NOTHING;

-- 中联重科 FD系列（内燃重型叉车，国产动力）
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中联重科','FD系列','transmission','手波'),
  ('中联重科','FD系列','transmission','自波'),
  ('中联重科','FD系列','transmission','无'),
  ('中联重科','FD系列','engine','国产发动机'),
  ('中联重科','FD系列','engine','无')
ON CONFLICT DO NOTHING;

-- =====================================================
-- 6. 修改 original_prices 表：清空旧数据，移除 battery_type 字段
-- =====================================================
DELETE FROM original_prices;

ALTER TABLE original_prices DROP COLUMN IF EXISTS battery_type;

-- 重建唯一约束（8字段，移除 battery_type）
-- 用 DO 块动态删除 original_prices 上所有 UNIQUE 约束，避免依赖自动生成的约束名
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint
              WHERE conrelid = 'original_prices'::regclass AND contype = 'u') LOOP
        EXECUTE 'ALTER TABLE original_prices DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
    END LOOP;
END $$;
ALTER TABLE original_prices
    ADD CONSTRAINT original_prices_8field_unique
    UNIQUE (brand_type, brand, vehicle_type, series, tonnage,
            config_type, mast_type, mast_height_mm);

-- 重建查询索引（移除 battery_type，原索引本就不含 battery_type，重建以保持名称一致）
DROP INDEX IF EXISTS idx_original_prices_lookup;
CREATE INDEX IF NOT EXISTS idx_original_prices_lookup ON original_prices(
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm
);

-- =====================================================
-- 7. 修改 evaluations 表：移除 battery_type 字段
-- =====================================================
ALTER TABLE evaluations DROP COLUMN IF EXISTS battery_type;

-- =====================================================
-- 8. config_types 表保留（向后兼容管理员接口，不再用于级联查询）
-- =====================================================
