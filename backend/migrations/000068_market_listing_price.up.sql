-- #2680：挂牌价独立成列，让「挂牌 → 成交」的配对样本能落进一行。
--
-- 背景：`raw_price` 只有一个价位的语义（按 `raw_price_kind` 区分 listing / deal）。
-- 产权交易所省镜像的成交公告页**同时**给出「挂牌价格」与「成交金额」——这正是
-- 下游校准检验「清算折扣 δ 基线 0.85」需要的真配对样本。若只有一个价位列，
-- 配对关系在落库时就被丢掉了，下游再也算不出「成交/挂牌」折扣比。
--
-- 语义约定（列恒定，不随 kind 变）：
--   - `listing_price` 就是「挂牌价格」，永远是挂牌价；
--   - 成交样本：`raw_price` = 成交金额、`listing_price` = 挂牌价 ⇒ 两价齐备；
--   - 挂牌样本：两者相等（挂牌阶段的成交价尚未产生）；
--   - 其它采集面（rmfysszc / 京东 / 公拍网）：留 NULL——它们没有配对语义，
--     不该被下游误读成「挂牌价缺失」。
ALTER TABLE market_transactions
    ADD COLUMN IF NOT EXISTS listing_price NUMERIC(14,2);

COMMENT ON COLUMN market_transactions.listing_price IS
    '挂牌价格（元）。成交样本上与 raw_price（成交金额）配对，用于「挂牌→成交」折扣估计；非产权交易所采集面为 NULL（无配对语义）。';

-- 配对样本的检索索引：下游按「两价齐备」筛样本时走这个。
CREATE INDEX IF NOT EXISTS idx_market_transactions_listing_pair
    ON market_transactions (channel, auction_date)
    WHERE listing_price IS NOT NULL AND raw_price_kind = 'deal';
