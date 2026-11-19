-- #2672：一期拍卖数据管线 tracer bullet —— market_transactions 新表。
--
-- 第一性原理：市场成交样本是「弱标签」——来源公告的原文与挂牌/成交价是事实，
-- 经抽取/清洗/匹配后才成为可用于校准的残值率标签。本迁移只立存储形状：
-- 原始留底与来源必填，其余阶段字段全部可空 / 带 raw 默认值，后续票逐阶段填入。
-- 评估记录表与评估路径零改动（spec #2671 硬约束）：这是独立新表，不碰 evaluations。
--
-- 字段口径（与 spec #2671 / CONTEXT.md「市场真值」词条一致）：
--   - 渠道 / 来源 URL / 原始描述 jsonb：事实来源，NOT NULL；
--   - raw_price + raw_price_kind：原始价格（挂牌 / 成交），未经清算折扣 δ 调整；
--   - appraisal_price：评估价或起拍价；
--   - structured_specs：结构化规格（品牌/车型/吨位/年份/工时...）；
--   - auction_date：成交/拍卖日期；
--   - clean_status + rejection_reason：清洗状态与剔除原因；
--   - field_confidence + extraction_version：逐字段置信度与抽取版本；
--   - match_level + matched_original_range：匹配层级与原价区间；
--   - premium_anomaly：溢价异常标记（成交价 > 评估价 × 1.5 的候选标记，具体规则由清洗票落）；
--   - residual_rate_label + matched_original_price：残值率标签（成交价 ÷ 匹配原价）与引用原价。
--
-- raw 阶段：collector 只写前三列 + 其余默认值；真正的抽取/清洗/匹配留给后续票。

CREATE TABLE IF NOT EXISTS market_transactions (
    id                      BIGSERIAL PRIMARY KEY,
    channel                 VARCHAR(64)  NOT NULL,
    source_url              TEXT         NOT NULL,
    raw_description         JSONB        NOT NULL,
    structured_specs        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    raw_price               NUMERIC(14, 2),
    raw_price_kind          VARCHAR(16),
    appraisal_price         NUMERIC(14, 2),
    auction_date            DATE,
    clean_status            VARCHAR(32)  NOT NULL DEFAULT 'raw',
    rejection_reason        TEXT,
    field_confidence        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    extraction_version      VARCHAR(64),
    match_level             VARCHAR(32),
    matched_original_range  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    premium_anomaly         BOOLEAN      NOT NULL DEFAULT false,
    residual_rate_label     NUMERIC(8, 6),
    matched_original_price  NUMERIC(14, 2),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE market_transactions IS
    '一期拍卖数据管线（#2672）：市场成交样本，raw 留底 → 清洗 → 匹配 → 残值率标签；与评估记录表完全独立';
COMMENT ON COLUMN market_transactions.channel IS '采集渠道稳定标识（如 rmfysszc）；与来源 URL 一起构成原始事实来源';
COMMENT ON COLUMN market_transactions.source_url IS '公告原文 URL（可断言的来源）。本票 fixture 的 canonical 链接即落此列';
COMMENT ON COLUMN market_transactions.raw_description IS '原始描述 jsonb（标题/发布时间/正文等）：原始留底，不随抽取/清洗改动';
COMMENT ON COLUMN market_transactions.raw_price IS '原始价格（挂牌或成交），未经清算折扣 δ 调整；δ 只在下游校准时施加（spec #2671）';
COMMENT ON COLUMN market_transactions.raw_price_kind IS 'raw_price 类型：listing（挂牌）/ deal（成交）等';
COMMENT ON COLUMN market_transactions.clean_status IS '清洗状态：raw / cleaned / rejected...；raw 阶段默认 raw';
COMMENT ON COLUMN market_transactions.premium_anomaly IS '溢价异常标记：成交价显著高于评估价时置位，具体阈值由清洗票版本化模块定义';
COMMENT ON COLUMN market_transactions.residual_rate_label IS '残值率标签 = 成交价 ÷ 匹配原价；仅在匹配成功后可算，raw 阶段为空';
COMMENT ON COLUMN market_transactions.matched_original_price IS '残值率标签引用的匹配原价（评估记录里的原价事实），与标签一并落库以便复核';

-- 采集去重与来源追踪：同一公告 URL 不重复入库（后续增量模式的幂等基础）。
CREATE INDEX IF NOT EXISTS idx_market_transactions_source
    ON market_transactions (channel, source_url);

-- 校准/统计按清洗状态与标签过滤；raw 行量大，标签稀疏，局部索引避免无谓体积。
CREATE INDEX IF NOT EXISTS idx_market_transactions_label
    ON market_transactions (clean_status, residual_rate_label)
    WHERE residual_rate_label IS NOT NULL;

-- 清洗阶段与溢价异常的取值域用 CHECK 兜住（值域这种永不误伤历史数据的约束才进库层，
-- 与 000062 的取舍同源）；存量无行，无回填风险。
ALTER TABLE market_transactions
    ADD CONSTRAINT chk_market_transactions_price_kind
    CHECK (raw_price_kind IS NULL OR raw_price_kind IN ('listing', 'deal'));

ALTER TABLE market_transactions
    ADD CONSTRAINT chk_market_transactions_clean_status
    CHECK (clean_status IN ('raw', 'cleaned', 'rejected'));
