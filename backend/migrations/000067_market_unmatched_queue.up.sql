-- #2676：未匹配待审队列聚合视图。
--
-- 对齐 spec #2671「原价反哺」决策：不自动写回原价表；未匹配样本按
-- 品牌×型号×吨位聚合成待审队列（一期为聚合视图，无管理端 UI），人工定期
-- 审核补原价条目，补入后 collector rematch 重跑匹配回收标签。
--
-- 待审口径与 collector/review.py 的 group_unmatched 一致：
--   - 未拿到残值率标签（residual_rate_label IS NULL）；
--   - 未被清洗剔除（clean_status <> 'rejected'）。
-- 价格区间取组内有价样本的 min/max（价格缺失的样本计入样本数、不参与区间）。
CREATE OR REPLACE VIEW market_unmatched_queue AS
SELECT
    structured_specs ->> 'brand'                       AS brand,
    structured_specs ->> 'model'                       AS model,
    NULLIF(structured_specs ->> 'tonnage', '')::numeric AS tonnage,
    count(*)                                            AS sample_count,
    min(raw_price)                                      AS price_min,
    max(raw_price)                                      AS price_max
FROM market_transactions
WHERE residual_rate_label IS NULL
  AND clean_status <> 'rejected'
GROUP BY 1, 2, 3;

COMMENT ON VIEW market_unmatched_queue IS
    '未匹配待审队列（#2676）：按品牌×型号×吨位聚合的未匹配样本，含样本数与成交价区间；人工补原价后 rematch 回收标签';
