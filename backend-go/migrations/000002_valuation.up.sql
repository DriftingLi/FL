-- 000002_valuation.up.sql
-- 残值评估模块建表（合并源 000001_init + 000002_brand_models + 000003_battery_rul）
-- brands 表直接包含 models JSONB 列，避免后续 ALTER

-- =====================================================
-- 1. 评估记录表 evaluations
-- =====================================================
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
CREATE INDEX IF NOT EXISTS idx_evaluations_type   ON evaluations(forklift_type);
CREATE INDEX IF NOT EXISTS idx_evaluations_brand  ON evaluations(brand);
CREATE INDEX IF NOT EXISTS idx_evaluations_created ON evaluations(created_at DESC);

-- =====================================================
-- 2. 部件状态表 evaluation_items
-- =====================================================
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
CREATE INDEX IF NOT EXISTS idx_eval_items_eval ON evaluation_items(evaluation_id);

-- =====================================================
-- 3. 品牌表 brands（直接包含 models JSONB 列）
-- =====================================================
CREATE TABLE IF NOT EXISTS brands (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) UNIQUE NOT NULL,
    tier        VARCHAR(20) NOT NULL,
    k_brand     DECIMAL(4,2) NOT NULL,
    models      JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMP DEFAULT NOW()
);

-- =====================================================
-- 4. 部件配置表 part_configs
-- =====================================================
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

-- =====================================================
-- 5. 系数配置表 coefficient_configs
-- =====================================================
CREATE TABLE IF NOT EXISTS coefficient_configs (
    id          SERIAL PRIMARY KEY,
    key         VARCHAR(50) UNIQUE NOT NULL,
    value       DECIMAL(10,6) NOT NULL,
    description VARCHAR(255),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- =====================================================
-- 6. 历史成交数据表 historical_sales
-- =====================================================
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
CREATE INDEX IF NOT EXISTS idx_historical_type_brand ON historical_sales(forklift_type, brand);

-- =====================================================
-- 7. 电池评估主表 battery_evaluations
-- =====================================================
CREATE TABLE IF NOT EXISTS battery_evaluations (
    id                BIGSERIAL PRIMARY KEY,
    battery_type      VARCHAR(20) NOT NULL,
    battery_model     VARCHAR(100),
    cycle_count       INTEGER NOT NULL,
    rul_cycles        INTEGER NOT NULL,
    soh_percent       DECIMAL(5,2) NOT NULL,
    confidence        DECIMAL(4,3) NOT NULL,
    confidence_low    INTEGER,
    confidence_high   INTEGER,
    feature_importance JSONB,
    report_pdf_path   VARCHAR(255),
    created_at        TIMESTAMP DEFAULT NOW(),
    updated_at        TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_battery_evals_type    ON battery_evaluations(battery_type);
CREATE INDEX IF NOT EXISTS idx_battery_evals_created ON battery_evaluations(created_at DESC);

-- =====================================================
-- 8. 周期特征表 battery_cycle_features
-- =====================================================
CREATE TABLE IF NOT EXISTS battery_cycle_features (
    id              BIGSERIAL PRIMARY KEY,
    evaluation_id   BIGINT NOT NULL REFERENCES battery_evaluations(id) ON DELETE CASCADE,
    cycle_index     INTEGER NOT NULL,
    feature_vector  JSONB NOT NULL,
    raw_stats       JSONB NOT NULL,
    soh_at_cycle    DECIMAL(5,2) NOT NULL,
    UNIQUE(evaluation_id, cycle_index)
);
CREATE INDEX IF NOT EXISTS idx_battery_features_eval ON battery_cycle_features(evaluation_id);
