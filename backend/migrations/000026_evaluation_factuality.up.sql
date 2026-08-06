-- 000026_evaluation_factuality.up.sql
-- 评估事实性（ADR-0004）：建议与 λ 值在评估时点持久化锁定。
-- 此后管理员修改 coefficient_configs 不再改变历史评估记录的建议文字。

BEGIN;

ALTER TABLE evaluations
    ADD COLUMN IF NOT EXISTS suggestions      JSONB,
    ADD COLUMN IF NOT EXISTS lambda_electric  DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS lambda_combustion DOUBLE PRECISION;

COMMIT;
