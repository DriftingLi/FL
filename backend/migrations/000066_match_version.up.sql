-- #2675：记录产出匹配/标签所用的匹配器版本，便于按版本分离重跑。
ALTER TABLE market_transactions
    ADD COLUMN IF NOT EXISTS match_version VARCHAR(32);

COMMENT ON COLUMN market_transactions.match_version IS '产出本行 match_level/matched_original_price/residual_rate_label 的匹配器版本';
