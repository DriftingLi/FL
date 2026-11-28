-- #2958 试算全局开关与观测指标。默认关闭且不预置任何组合策略。
CREATE TABLE valuation_trial_control (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO valuation_trial_control (id, enabled, version) VALUES (1, FALSE, 1);

CREATE UNIQUE INDEX uq_valuation_trial_control_singleton ON valuation_trial_control (id);

CREATE TABLE valuation_trial_metrics (
    id BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    brand VARCHAR(100) NOT NULL,
    vehicle_type VARCHAR(100) NOT NULL,
    tonnage NUMERIC(10,2) NOT NULL,
    policy_name VARCHAR(16) NOT NULL,
    scenario VARCHAR(32) NOT NULL CHECK (scenario IN ('complete', 'masked', 'year_range')),
    formula_version VARCHAR(32) NOT NULL,
    formula_hash VARCHAR(64) NOT NULL,
    policy_version VARCHAR(128) NOT NULL,
    denominator INTEGER NOT NULL DEFAULT 1 CHECK (denominator = 1),
    priced INTEGER NOT NULL DEFAULT 0 CHECK (priced IN (0, 1)),
    covered INTEGER NOT NULL DEFAULT 0 CHECK (covered IN (0, 1)),
    too_wide INTEGER NOT NULL DEFAULT 0 CHECK (too_wide IN (0, 1)),
    unpriced_reason VARCHAR(64) NOT NULL DEFAULT '',
    follow_up_count INTEGER NOT NULL DEFAULT 0 CHECK (follow_up_count >= 0),
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    model_name VARCHAR(100) NOT NULL DEFAULT '',
    prompt_tokens BIGINT NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens BIGINT NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
    cost_micros BIGINT NOT NULL DEFAULT 0 CHECK (cost_micros >= 0),
    CHECK (covered <= priced),
    CHECK (priced + too_wide <= 1)
);

CREATE INDEX idx_valuation_trial_metrics_time ON valuation_trial_metrics (occurred_at DESC);
CREATE INDEX idx_valuation_trial_metrics_dimensions ON valuation_trial_metrics
    (policy_name, brand, vehicle_type, tonnage, scenario, formula_hash, policy_version, occurred_at DESC);
