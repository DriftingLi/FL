ALTER TABLE valuation_operations ADD COLUMN card_kind varchar(16);
ALTER TABLE valuation_operations ADD COLUMN card_key varchar(320);
COMMENT ON TABLE valuation_operations IS
    'Durable idempotency receipts retained with formal evaluations and structured conversation results; no TTL. Metadata only, no prompt or request body retention.';
ALTER TABLE valuation_operations DROP CONSTRAINT valuation_operations_status_check;
ALTER TABLE valuation_operations ADD CONSTRAINT valuation_operations_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'stale'));

CREATE TABLE valuation_explanation_calls (
    call_key varchar(320) PRIMARY KEY,
    operation_key varchar(320) NOT NULL REFERENCES valuation_operations(operation_key) ON DELETE CASCADE,
    card_kind varchar(16) NOT NULL CHECK (card_kind IN ('draft', 'formal', 'trial')),
    card_key varchar(320) NOT NULL,
    ip_hash varchar(64),
    reserved_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_valuation_explanation_calls_operation_key
    ON valuation_explanation_calls (operation_key);

COMMENT ON TABLE valuation_explanation_calls IS
    'One row per reserved explanation call. Shares valuation_suggestion_budgets; metadata only, no prompt or model response body.';
