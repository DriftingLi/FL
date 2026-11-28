ALTER TABLE valuation_operations ALTER COLUMN operation_key TYPE varchar(320);
ALTER TABLE valuation_operations ADD COLUMN operation_kind varchar(16) NOT NULL DEFAULT 'evaluation';
ALTER TABLE valuation_operations ADD COLUMN result_json jsonb;

ALTER TABLE valuation_operations DROP CONSTRAINT valuation_operations_status_check;
ALTER TABLE valuation_operations ADD CONSTRAINT valuation_operations_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'stale'));

CREATE TABLE valuation_suggestion_calls (
    call_key varchar(320) PRIMARY KEY,
    operation_key varchar(320) NOT NULL REFERENCES valuation_operations(operation_key) ON DELETE CASCADE,
    draft_id varchar(128) NOT NULL,
    ip_hash varchar(64),
    reserved_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE TABLE valuation_suggestion_budgets (
    scope varchar(16) NOT NULL CHECK (scope IN ('draft', 'ip', 'global')),
    budget_key varchar(128) NOT NULL,
    used integer NOT NULL CHECK (used >= 0),
    window_started_at timestamp with time zone NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, budget_key)
);

CREATE UNIQUE INDEX uq_valuation_suggestion_calls_operation_key
    ON valuation_suggestion_calls (operation_key);

COMMENT ON TABLE valuation_operations IS
    'Durable idempotency receipts for formal evaluations and draft suggestions. Metadata and structured suggestion results only; no prompt or request body retention.';
COMMENT ON TABLE valuation_suggestion_calls IS
    'One row per reserved actual model invocation. operation_key is unique so retries cannot consume another call. No prompt or response body.';
COMMENT ON TABLE valuation_suggestion_budgets IS
    'Atomic draft, HMAC client-IP and global call counters. Draft counters do not reset on login; IP/global counters use explicit time windows.';
