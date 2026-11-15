-- #2550（#2512 G-03 / N3）：任务中心补「游览商城」「完成积分兑礼」两项。
--
-- 三件套必须齐（缺任一件就是一个空口任务，先例见 000045）：
--   1. 任务配置行        ← 本迁移
--   2. 行为事实源        ← 本迁移（mall_views）+ 既有 points_ledger（redeem_*）
--   3. 行为判定 case     ← internal/points/service.go 的 taskProgressFor
--
-- code 与移动端 #2523 的任务骨架约定一致（browse_mall / exchange_gift），不另起名。
--
-- browse_mall（游览商城，5 分，3 个商品/日）：原型 569:4532 的「游览商城(1/3)」写明
-- 「去商城游览 3 个商品」⇒ 与 browse_course 同形：一天一件商品只记一笔，按日去重后
-- COUNT 不同 SKU 数。事实源另立 mall_views，不复用 points_shop_item（那是商品目录，
-- 不记录「谁看过」）；也不复用 points_ledger（浏览不产生积分流水）。
--
-- exchange_gift（完成积分兑礼，10 分，1 次/日）：RedeemShop / RedeemCourse /
-- RedeemRealPaper 三条路径都收敛到 redeem 单点并写一条 reason=redeem_* 的
-- points_ledger 流水（ADR-0023 / ADR-0041：不可回退的流水就是事件）⇒ 不另立第三张
-- 事实表，判据直接读当日 redeem_* 流水，避免同一件事两处记账。
--
-- 分值：原型「今日可得 +80」= 既有 daily 合计 65 + 游览商城 5 + 完成积分兑礼 10；
-- 与「浏览课程」同档（5）的是轻浏览行为，兑礼涉及真实消耗（10）。

CREATE TABLE mall_views (
    user_id    INTEGER     NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    view_date  DATE        NOT NULL,
    sku        VARCHAR(64) NOT NULL REFERENCES points_shop_item(sku) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, view_date, sku)
);
COMMENT ON TABLE mall_views IS '「游览商城」按日去重的商品浏览事实（#2550）：打开商品详情即记一笔，一天一件商品一笔';
COMMENT ON COLUMN mall_views.view_date IS '浏览发生的 Asia/Shanghai 自然日：任务判据按它取当日去重商品数';

INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES
  ('browse_mall',   '游览商城',     'daily',  5, 1, NULL, 'mall_browse', '去商城游览 3 个商品'),
  ('exchange_gift', '完成积分兑礼', 'daily', 10, 1, NULL, 'gift_redeem', '完成 1 次积分兑礼')
ON CONFLICT (code) DO NOTHING;
