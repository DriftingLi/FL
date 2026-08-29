-- 微信支付订单表（Native / JSAPI 共用）。
-- 金额单位：分；out_trade_no 唯一，作为下单与回调对账键，回调幂等。
CREATE TABLE IF NOT EXISTS pay_orders (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT       NOT NULL,
    out_trade_no   VARCHAR(64)  NOT NULL,
    transaction_id VARCHAR(64)  DEFAULT '',
    pay_type       VARCHAR(16)  NOT NULL,           -- native / jsapi
    subject        VARCHAR(256) NOT NULL,
    amount         BIGINT       NOT NULL,           -- 分
    status         VARCHAR(16)  NOT NULL DEFAULT 'created',
    notify_raw     JSONB,
    paid_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pay_orders_out_trade_no ON pay_orders (out_trade_no);
CREATE INDEX IF NOT EXISTS idx_pay_orders_user_id ON pay_orders (user_id);
CREATE INDEX IF NOT EXISTS idx_pay_orders_status ON pay_orders (status);