-- #2673：LLM 抽取接线 —— 标签资格标记 + 抽检队列。
--
-- 第一性原理：抽取是弱标签链路的第一道质量闸门。模型输出不是事实，
-- 低置信字段与价格抽取失败都必须留下「可查对象」，否则下游校准无从知道
-- 哪些样本不该进标签集。本迁移只加两样东西：
--   1) market_transactions.label_eligible —— 抽取价格成功才为真；
--   2) market_sample_review_queue —— 一等可查的抽检队列行（含原因与明细 jsonb 文本）。
-- 评估记录表与评估路径零改动（spec #2671 硬约束）。

-- 标签资格：旧 raw 行未做过抽取，默认 false，不会把 raw 行误当有效标签。
ALTER TABLE market_transactions
    ADD COLUMN IF NOT EXISTS label_eligible BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN market_transactions.label_eligible IS
    '#2673：价格字段抽取成功并过判定才有标签资格；false 的样本不得进入残值率标签集';

-- 抽检队列：低置信字段（< 0.75）/ 价格抽取失败 各入一行。
-- 用独立表而非 jsonb 标记位：可索引、可按 reason 聚合、队列消费方有一等对象可查。
CREATE TABLE IF NOT EXISTS market_sample_review_queue (
    id          BIGSERIAL    PRIMARY KEY,
    sample_id   BIGINT       NOT NULL REFERENCES market_transactions(id) ON DELETE CASCADE,
    reason      VARCHAR(32)  NOT NULL,
    detail      TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE market_sample_review_queue IS
    '#2673：市场样本抽检队列——低置信字段 / 价格抽取失败等需人工复核的样本，按 reason 分类';
COMMENT ON COLUMN market_sample_review_queue.sample_id IS '指向 market_transactions.id；样本删除时级联清理队列行';
COMMENT ON COLUMN market_sample_review_queue.reason IS '入队原因：low_confidence / price_extraction_failed';

CREATE INDEX IF NOT EXISTS idx_market_sample_review_queue_reason
    ON market_sample_review_queue (reason);
