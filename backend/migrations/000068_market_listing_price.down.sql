-- #2680 down：移除挂牌价列与配对索引。
DROP INDEX IF EXISTS idx_market_transactions_listing_pair;

ALTER TABLE market_transactions
    DROP COLUMN IF EXISTS listing_price;
