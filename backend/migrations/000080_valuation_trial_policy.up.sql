-- 参考试算显式策略。默认不插入任何行：生产 allowlist 与宽度配置均为空。
-- demo 只能由非生产环境显式请求；pilot 也只对 enabled 的精确组合开放。
CREATE TABLE valuation_trial_policy (
    id BIGSERIAL PRIMARY KEY,
    policy_name VARCHAR(16) NOT NULL CHECK (policy_name IN ('demo', 'pilot')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    brand VARCHAR(100) NOT NULL,
    vehicle_type VARCHAR(100) NOT NULL,
    tonnage NUMERIC(10,2) NOT NULL CHECK (tonnage > 0),
    max_width_ratio NUMERIC(7,6) NOT NULL CHECK (max_width_ratio > 0 AND max_width_ratio <= 1),
    usage_hour_ratios JSONB NOT NULL CHECK (jsonb_typeof(usage_hour_ratios) = 'array' AND jsonb_array_length(usage_hour_ratios) > 0),
    condition_ratings JSONB NOT NULL CHECK (jsonb_typeof(condition_ratings) = 'array' AND jsonb_array_length(condition_ratings) > 0),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE valuation_trial_policy
    ADD COLUMN original_paint_values JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN has_license_plate_values JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN has_registration_certificate_values JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN has_maintenance_records_values JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE valuation_trial_policy
    ADD CONSTRAINT ck_valuation_trial_original_paint_values CHECK (jsonb_typeof(original_paint_values) = 'array'),
    ADD CONSTRAINT ck_valuation_trial_license_plate_values CHECK (jsonb_typeof(has_license_plate_values) = 'array'),
    ADD CONSTRAINT ck_valuation_trial_registration_certificate_values CHECK (jsonb_typeof(has_registration_certificate_values) = 'array'),
    ADD CONSTRAINT ck_valuation_trial_maintenance_records_values CHECK (jsonb_typeof(has_maintenance_records_values) = 'array');

CREATE UNIQUE INDEX uq_valuation_trial_policy_combo ON valuation_trial_policy (policy_name, brand, vehicle_type, tonnage);

CREATE TABLE valuation_trial_snapshots (
    id BIGSERIAL PRIMARY KEY,
    snapshot_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL UNIQUE,
    request_json JSONB NOT NULL,
    snapshot_json JSONB NOT NULL,
    formula_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_valuation_trial_snapshots_snapshot_id ON valuation_trial_snapshots (snapshot_id);
