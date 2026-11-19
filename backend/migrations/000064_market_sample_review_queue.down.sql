-- 回滚 000064：先删抽检队列表，再摘掉标签资格列（逆序）。
-- 仅影响 #2673 新增的两处标记，market_transactions 本体与评估路径不受影响。

DROP TABLE IF EXISTS market_sample_review_queue;
ALTER TABLE market_transactions DROP COLUMN IF EXISTS label_eligible;
