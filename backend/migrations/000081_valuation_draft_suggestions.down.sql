DELETE FROM valuation_operations WHERE operation_kind = 'suggestion';
DROP TABLE valuation_suggestion_budgets;
DROP TABLE valuation_suggestion_calls;

ALTER TABLE valuation_operations DROP COLUMN result_json;
ALTER TABLE valuation_operations DROP COLUMN operation_kind;
ALTER TABLE valuation_operations DROP CONSTRAINT valuation_operations_status_check;
ALTER TABLE valuation_operations ADD CONSTRAINT valuation_operations_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed'));
ALTER TABLE valuation_operations ALTER COLUMN operation_key TYPE varchar(128);
