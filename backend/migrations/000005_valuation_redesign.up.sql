-- 000005_valuation_redesign.up.sql
-- 残值评估模块重构：按新填报逻辑重建表结构与种子数据
-- 清理旧的 evaluation_items / part_configs / historical_sales / evaluations
-- 新增字典表 + original_prices + 全新 evaluations

-- =====================================================
-- 1. 清理旧表
-- =====================================================
DROP TABLE IF EXISTS evaluation_items;
DROP TABLE IF EXISTS part_configs;
DROP TABLE IF EXISTS historical_sales;
DROP TABLE IF EXISTS evaluations;

-- =====================================================
-- 2. 改造 brands 表
-- =====================================================
ALTER TABLE brands DROP COLUMN IF EXISTS tier;
ALTER TABLE brands DROP COLUMN IF EXISTS models;
ALTER TABLE brands ADD COLUMN IF NOT EXISTS brand_type VARCHAR(50) NOT NULL DEFAULT '未分类';
ALTER TABLE brands ADD COLUMN IF NOT EXISTS k_brand DECIMAL(5,4) NOT NULL DEFAULT 1.0;
CREATE INDEX IF NOT EXISTS idx_brands_type ON brands(brand_type);

-- =====================================================
-- 3. 新增字典表
-- =====================================================
CREATE TABLE IF NOT EXISTS brand_types (
    name    VARCHAR(50) PRIMARY KEY,
    k_type  DECIMAL(5,4) NOT NULL DEFAULT 1.0
);

CREATE TABLE IF NOT EXISTS vehicle_types (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) UNIQUE NOT NULL,
    power_type  VARCHAR(20) NOT NULL
);

CREATE TABLE IF NOT EXISTS series (
    id      SERIAL PRIMARY KEY,
    brand   VARCHAR(50) NOT NULL,
    name    VARCHAR(50) NOT NULL,
    UNIQUE(brand, name)
);

CREATE TABLE IF NOT EXISTS tonnages (
    id      SERIAL PRIMARY KEY,
    value   DECIMAL(5,2) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS config_types (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS mast_types (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS mast_heights (
    id          SERIAL PRIMARY KEY,
    value_mm    INTEGER UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS battery_types (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS condition_ratings (
    id                  SERIAL PRIMARY KEY,
    rating              VARCHAR(10) UNIQUE NOT NULL,
    label               VARCHAR(20) NOT NULL,
    base_coefficient    DECIMAL(5,4) NOT NULL
);

CREATE TABLE IF NOT EXISTS region_coefficients (
    id              SERIAL PRIMARY KEY,
    province        VARCHAR(50) NOT NULL,
    city            VARCHAR(50) NOT NULL,
    coefficient     DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    UNIQUE(province, city)
);

-- =====================================================
-- 4. 车辆原价表
-- =====================================================
CREATE TABLE IF NOT EXISTS original_prices (
    id              BIGSERIAL PRIMARY KEY,
    brand_type      VARCHAR(50) NOT NULL,
    brand           VARCHAR(50) NOT NULL,
    vehicle_type    VARCHAR(50) NOT NULL,
    series          VARCHAR(50) NOT NULL,
    tonnage         DECIMAL(5,2) NOT NULL,
    config_type     VARCHAR(50) NOT NULL,
    mast_type       VARCHAR(50) NOT NULL,
    mast_height_mm  INTEGER NOT NULL,
    battery_type    VARCHAR(50),
    original_price  DECIMAL(12,2) NOT NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    updated_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE (brand_type, brand, vehicle_type, series, tonnage,
            config_type, mast_type, mast_height_mm, battery_type)
);
CREATE INDEX IF NOT EXISTS idx_original_prices_lookup ON original_prices(
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm
);

-- =====================================================
-- 5. 重建 evaluations 表
-- =====================================================
CREATE TABLE evaluations (
    id                          BIGSERIAL PRIMARY KEY,
    brand_type                  VARCHAR(50) NOT NULL,
    brand                       VARCHAR(50) NOT NULL,
    vehicle_type                VARCHAR(50) NOT NULL,
    series                      VARCHAR(50) NOT NULL,
    tonnage                     DECIMAL(5,2) NOT NULL,
    config_type                 VARCHAR(50) NOT NULL,
    mast_type                   VARCHAR(50) NOT NULL,
    mast_height_mm              INTEGER NOT NULL,
    factory_year                INTEGER NOT NULL,
    sale_year                   INTEGER NOT NULL,
    usage_hours                 INTEGER NOT NULL,
    original_paint              BOOLEAN NOT NULL,
    battery_type                VARCHAR(50),
    province                    VARCHAR(50) NOT NULL,
    city                        VARCHAR(50) NOT NULL,
    has_license_plate           BOOLEAN NOT NULL,
    has_registration_certificate BOOLEAN NOT NULL,
    has_maintenance_records     BOOLEAN NOT NULL,
    condition_rating            VARCHAR(10) NOT NULL,
    original_price              DECIMAL(12,2) NOT NULL,
    k_time                      DECIMAL(6,4),
    k_hours                     DECIMAL(6,4),
    k_brand                     DECIMAL(6,4),
    k_condition                 DECIMAL(6,4),
    k_market                    DECIMAL(6,4),
    estimated_value             DECIMAL(12,2) NOT NULL,
    confidence_low              DECIMAL(12,2),
    confidence_high             DECIMAL(12,2),
    report_pdf_path             VARCHAR(255),
    created_at                  TIMESTAMP DEFAULT NOW(),
    updated_at                  TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_evaluations_created ON evaluations(created_at DESC);
CREATE INDEX idx_evaluations_brand ON evaluations(brand);

-- =====================================================
-- 6. 清理旧 coefficient_configs 并写入新参数
-- =====================================================
DELETE FROM coefficient_configs WHERE key IN
  ('w_work_condition','w_brand','w_condition','w_market','k_market',
   'lambda_electric','lambda_combustion','confidence_range');

INSERT INTO coefficient_configs (key, value, description) VALUES
  ('lambda_electric',     0.120000, '电动叉车时间衰减系数 λ'),
  ('lambda_combustion',   0.100000, '内燃叉车时间衰减系数 λ'),
  ('annual_usage_hours',  1750.000000, '年度标准使用小时'),
  ('confidence_range',    0.100000, '残值置信区间幅度 ±'),
  ('k_hours_ratio_low',   0.700000, '工时区间阈值：低'),
  ('k_hours_ratio_mid',   1.000000, '工时区间阈值：中'),
  ('k_hours_ratio_high',  1.300000, '工时区间阈值：高'),
  ('k_hours_ratio_max',   1.600000, '工时区间阈值：最大')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, description = EXCLUDED.description;

-- =====================================================
-- 7. 种子数据：brand_types
-- =====================================================
INSERT INTO brand_types (name, k_type) VALUES
  ('进口一线', 1.15),
  ('进口二线', 1.05),
  ('国产一线', 1.00),
  ('国产二线', 0.92),
  ('国产其他', 0.85)
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 8. 种子数据：brands（重建品牌数据）
-- =====================================================
DELETE FROM brands;
INSERT INTO brands (name, brand_type, k_brand, is_active) VALUES
  ('林德',     '进口一线', 1.10, TRUE),
  ('丰田',     '进口一线', 1.08, TRUE),
  ('永恒力',   '进口一线', 1.06, TRUE),
  ('斗山',     '进口二线', 0.98, TRUE),
  ('海斯特',   '进口二线', 0.96, TRUE),
  ('凯斯',     '进口二线', 0.95, TRUE),
  ('合力',     '国产一线', 1.00, TRUE),
  ('杭叉',     '国产一线', 1.00, TRUE),
  ('比亚迪',   '国产一线', 1.02, TRUE),
  ('龙工',     '国产二线', 0.94, TRUE),
  ('柳工',     '国产二线', 0.93, TRUE),
  ('中联重科', '国产二线', 0.92, TRUE),
  ('宝骊',     '国产其他', 0.88, TRUE)
ON CONFLICT DO NOTHING;

-- =====================================================
-- 9. 种子数据：vehicle_types
-- =====================================================
INSERT INTO vehicle_types (name, power_type) VALUES
  ('电动平衡重式',   'electric'),
  ('电动前移式',     'electric'),
  ('电动托盘搬运车', 'electric'),
  ('电动堆高车',     'electric'),
  ('内燃平衡重式',   'combustion'),
  ('内燃重型叉车',   'combustion')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 10. 种子数据：series
-- =====================================================
INSERT INTO series (brand, name) VALUES
  ('林德', 'E系列'), ('林德', 'Xi系列'), ('林德', 'H系列'), ('林德', 'T-MATIC'),
  ('丰田', '8FBE系列'), ('丰田', '8FD系列'), ('丰田', 'Traigo系列'),
  ('永恒力', 'EFG系列'), ('永恒力', 'ETV系列'), ('永恒力', 'ERIC系列'),
  ('合力', 'A系列'), ('合力', 'X系列'), ('合力', 'CPCD系列'), ('合力', 'CPD系列'),
  ('杭叉', 'A系列'), ('杭叉', 'XH系列'), ('杭叉', 'XC系列'), ('杭叉', 'XE系列'), ('杭叉', 'XF系列'),
  ('比亚迪', 'CPD系列'),
  ('斗山', 'B30S系列'), ('斗山', 'BR系列'),
  ('海斯特', 'H系列'), ('海斯特', 'J系列'),
  ('龙工', 'A系列'),
  ('柳工', 'CLG系列'),
  ('中联重科', 'FD系列'),
  ('宝骊', 'KBE系列')
ON CONFLICT (brand, name) DO NOTHING;

-- =====================================================
-- 11. 种子数据：tonnages
-- =====================================================
INSERT INTO tonnages (value) VALUES
  (1.0),(1.5),(1.8),(2.0),(2.5),(3.0),(3.5),
  (4.0),(4.5),(5.0),(6.0),(7.0),(8.0),
  (10.0),(14.0),(16.0),(20.0),(25.0)
ON CONFLICT (value) DO NOTHING;

-- =====================================================
-- 12. 种子数据：config_types
-- =====================================================
INSERT INTO config_types (name) VALUES
  ('标准配置'), ('高配置')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 13. 种子数据：mast_types
-- =====================================================
INSERT INTO mast_types (name) VALUES
  ('两级门架'), ('三级门架'), ('四级门架')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 14. 种子数据：mast_heights
-- =====================================================
INSERT INTO mast_heights (value_mm) VALUES
  (3000),(4000),(4500),(5000),(6000),(7000)
ON CONFLICT (value_mm) DO NOTHING;

-- =====================================================
-- 15. 种子数据：battery_types
-- =====================================================
INSERT INTO battery_types (name) VALUES
  ('磷酸铁锂(LFP)'), ('三元锂(NCM)'), ('铅酸')
ON CONFLICT (name) DO NOTHING;

-- =====================================================
-- 16. 种子数据：condition_ratings
-- =====================================================
INSERT INTO condition_ratings (rating, label, base_coefficient) VALUES
  ('A', '优秀', 1.00),
  ('B', '良好', 0.90),
  ('C', '一般', 0.78),
  ('D', '较差', 0.65),
  ('E', '差',   0.50)
ON CONFLICT (rating) DO NOTHING;

-- =====================================================
-- 17. 种子数据：region_coefficients
-- =====================================================
INSERT INTO region_coefficients (province, city, coefficient) VALUES
  ('上海', '上海', 0.98),
  ('江苏', '苏州', 0.98),
  ('江苏', '南京', 0.99),
  ('浙江', '杭州', 0.98),
  ('浙江', '宁波', 0.99),
  ('广东', '广州', 0.99),
  ('广东', '深圳', 0.97),
  ('北京', '北京', 1.02),
  ('四川', '成都', 1.05),
  ('湖北', '武汉', 1.04),
  ('山东', '青岛', 1.00),
  ('河南', '郑州', 1.05)
ON CONFLICT (province, city) DO NOTHING;

-- =====================================================
-- 18. 种子数据：original_prices（基于互联网真实调研）
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price) VALUES
  ('进口一线','林德','电动平衡重式','E系列',1.5,'标准配置','两级门架',3000,'磷酸铁锂(LFP)',188000),
  ('进口一线','林德','电动平衡重式','Xi系列',2.0,'高配置','三级门架',4500,'磷酸铁锂(LFP)',235000),
  ('进口一线','林德','内燃平衡重式','H系列',3.0,'标准配置','两级门架',3000,NULL,165000),
  ('进口一线','林德','内燃平衡重式','H系列',5.0,'标准配置','三级门架',4000,NULL,280000),
  ('进口一线','丰田','电动平衡重式','8FBE系列',1.5,'标准配置','两级门架',3000,'磷酸铁锂(LFP)',175000),
  ('进口一线','丰田','内燃平衡重式','8FD系列',3.0,'标准配置','两级门架',3000,NULL,155000),
  ('进口一线','丰田','内燃平衡重式','8FD系列',5.0,'标准配置','三级门架',4500,NULL,265000),
  ('进口一线','永恒力','电动前移式','ETV系列',1.4,'标准配置','三级门架',5000,'磷酸铁锂(LFP)',220000),
  ('进口一线','永恒力','电动平衡重式','EFG系列',2.0,'标准配置','两级门架',3000,'铅酸',168000),
  ('进口二线','斗山','内燃平衡重式','B30S系列',3.0,'标准配置','两级门架',3000,NULL,118000),
  ('进口二线','海斯特','内燃平衡重式','H系列',3.0,'标准配置','两级门架',3000,NULL,122000),
  ('国产一线','合力','电动平衡重式','CPD系列',3.0,'标准配置','两级门架',3000,'磷酸铁锂(LFP)',98000),
  ('国产一线','合力','内燃平衡重式','CPCD系列',3.0,'标准配置','两级门架',3000,NULL,85000),
  ('国产一线','合力','内燃平衡重式','CPCD系列',5.0,'标准配置','三级门架',4000,NULL,145000),
  ('国产一线','杭叉','电动平衡重式','XE系列',2.0,'标准配置','两级门架',3000,'磷酸铁锂(LFP)',88000),
  ('国产一线','杭叉','电动平衡重式','XH系列',3.0,'高配置','三级门架',4500,'磷酸铁锂(LFP)',132000),
  ('国产一线','杭叉','内燃平衡重式','A系列',3.0,'标准配置','两级门架',3000,NULL,78000),
  ('国产一线','杭叉','内燃平衡重式','XF系列',5.0,'标准配置','三级门架',4500,NULL,138000),
  ('国产一线','比亚迪','电动平衡重式','CPD系列',4.0,'标准配置','三级门架',3000,'磷酸铁锂(LFP)',158000),
  ('国产一线','比亚迪','电动平衡重式','CPD系列',5.0,'高配置','三级门架',3000,'磷酸铁锂(LFP)',188000),
  ('国产二线','龙工','内燃平衡重式','A系列',3.0,'标准配置','两级门架',3000,NULL,72000),
  ('国产二线','柳工','内燃平衡重式','CLG系列',3.0,'标准配置','两级门架',3000,NULL,70000)
ON CONFLICT DO NOTHING;
