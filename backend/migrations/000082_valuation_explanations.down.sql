DROP TABLE valuation_explanation_calls;
DELETE FROM valuation_operations WHERE operation_kind = 'explanation';
ALTER TABLE valuation_operations DROP COLUMN card_kind;
ALTER TABLE valuation_operations DROP COLUMN card_key;
ALTER TABLE valuation_operations DROP CONSTRAINT valuation_operations_status_check;
ALTER TABLE valuation_operations ADD CONSTRAINT valuation_operations_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'stale'));
