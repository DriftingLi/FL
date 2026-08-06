-- 000026_evaluation_factuality.down.sql

BEGIN;

ALTER TABLE evaluations
    DROP COLUMN IF EXISTS suggestions,
    DROP COLUMN IF EXISTS lambda_electric,
    DROP COLUMN IF EXISTS lambda_combustion;

COMMIT;
