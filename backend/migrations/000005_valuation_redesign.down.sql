-- 000005_valuation_redesign.down.sql
-- 回滚：删除新表，恢复旧 evaluations 结构

-- 删除新表
DROP TABLE IF EXISTS original_prices;
DROP TABLE IF EXISTS evaluations;
DROP TABLE IF EXISTS region_coefficients;
DROP TABLE IF EXISTS condition_ratings;
DROP TABLE IF EXISTS battery_types;
DROP TABLE IF EXISTS mast_heights;
DROP TABLE IF EXISTS mast_types;
DROP TABLE IF EXISTS config_types;
DROP TABLE IF EXISTS tonnages;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS vehicle_types;
DROP TABLE IF EXISTS brand_types;

-- 恢复 brands 旧结构
ALTER TABLE brands DROP COLUMN IF EXISTS brand_type;
ALTER TABLE brands DROP COLUMN IF EXISTS k_brand;
ALTER TABLE brands ADD COLUMN IF NOT EXISTS tier VARCHAR(20) NOT NULL DEFAULT 'tier1_domestic';
ALTER TABLE brands ADD COLUMN IF NOT EXISTS models JSONB NOT NULL DEFAULT '[]'::jsonb;

-- 恢复旧 evaluations 表
CREATE TABLE IF NOT EXISTS evaluations (
    id              BIGSERIAL PRIMARY KEY,
    forklift_type   VARCHAR(20) NOT NULL,
    brand           VARCHAR(50) NOT NULL,
    model           VARCHAR(100),
    original_price  DECIMAL(10,2) NOT NULL,
    purchase_year   INTEGER NOT NULL,
    sale_year       INTEGER NOT NULL,
    usage_hours     INTEGER NOT NULL,
    work_condition  VARCHAR(20) NOT NULL,
    fuel_type       VARCHAR(20),
    can_drive       BOOLEAN NOT NULL,
    hydraulic_ok    BOOLEAN NOT NULL,
    k_time          DECIMAL(6,4),
    k_hours         DECIMAL(6,4),
    k_work          DECIMAL(6,4),
    k_brand         DECIMAL(6,4),
    k_condition     DECIMAL(6,4),
    k_market        DECIMAL(6,4),
    estimated_value DECIMAL(10,2) NOT NULL,
    confidence_low  DECIMAL(10,2),
    confidence_high DECIMAL(10,2),
    report_pdf_path VARCHAR(255),
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- 恢复旧 evaluation_items 表
CREATE TABLE IF NOT EXISTS evaluation_items (
    id              BIGSERIAL PRIMARY KEY,
    evaluation_id   BIGINT NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    category_code   VARCHAR(50) NOT NULL,
    category_name   VARCHAR(50) NOT NULL,
    item_code       VARCHAR(100) NOT NULL,
    item_name       VARCHAR(100) NOT NULL,
    status          VARCHAR(20) NOT NULL,
    category_weight DECIMAL(5,4) NOT NULL,
    item_weight     DECIMAL(5,4) NOT NULL,
    score           DECIMAL(4,2) NOT NULL
);

-- 恢复旧 part_configs 表
CREATE TABLE IF NOT EXISTS part_configs (
    id              SERIAL PRIMARY KEY,
    forklift_type   VARCHAR(20) NOT NULL,
    category_code   VARCHAR(50) NOT NULL,
    category_name   VARCHAR(50) NOT NULL,
    category_weight DECIMAL(5,4) NOT NULL,
    item_code       VARCHAR(100) NOT NULL,
    item_name       VARCHAR(100) NOT NULL,
    item_weight     DECIMAL(5,4) NOT NULL,
    UNIQUE(forklift_type, item_code)
);

-- 恢复旧 historical_sales 表
CREATE TABLE IF NOT EXISTS historical_sales (
    id              BIGSERIAL PRIMARY KEY,
    forklift_type   VARCHAR(20) NOT NULL,
    brand           VARCHAR(50) NOT NULL,
    model           VARCHAR(100),
    original_price  DECIMAL(10,2) NOT NULL,
    purchase_year   INTEGER NOT NULL,
    sale_year       INTEGER NOT NULL,
    usage_hours     INTEGER NOT NULL,
    work_condition  VARCHAR(20) NOT NULL,
    fuel_type       VARCHAR(20),
    sale_price      DECIMAL(10,2) NOT NULL,
    imported_at     TIMESTAMP DEFAULT NOW()
);
