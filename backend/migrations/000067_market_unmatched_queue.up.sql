-- #2676：未匹配待审队列聚合视图。
--
-- 对齐 spec #2671「原价反哺」决策：不自动写回原价表；未匹配样本按
-- 品牌×型号×吨位聚合成待审队列（一期为聚合视图，无管理端 UI），人工定期
-- 审核补原价条目，补入后 collector rematch 重跑匹配回收标签。
--
-- 待审口径（唯一实现就在本视图，Python 侧只声明列名）：
--   - 有价格资格（label_eligible）：价格抽取失败的样本补原价也回收不了标签，
--     它们已在抽检队列（ADR-0097）待复核，不该在待审队列里永久占位；
--   - 未拿到残值率标签（residual_rate_label IS NULL）；
--   - 未被清洗剔除（clean_status <> 'rejected'）。
-- 价格区间取组内有价样本的 min/max（价格缺失的样本计入样本数、不参与区间）。
--
-- 吨位来自 LLM 抽取的 jsonb 文本，可能是 "3.0" 也可能是 "约3吨"：直接
-- ::numeric 会让整个视图查询报 InvalidTextRepresentation。这里只转换纯数值
-- 文本，其余落 NULL 组（不丢样本，分组变粗），保证视图任何数据下都可查。
--
-- 排序：样本多的组优先处理（人工补原价按此优先级），键做次级排序保证确定性。
CREATE OR REPLACE VIEW market_unmatched_queue AS
SELECT
    structured_specs ->> 'brand'                       AS brand,
    structured_specs ->> 'model'                       AS model,
    CASE
        WHEN structured_specs ->> 'tonnage' ~ '^[0-9]+(\.[0-9]+)?$'
        THEN (structured_specs ->> 'tonnage')::numeric
    END                                                 AS tonnage,
    count(*)                                            AS sample_count,
    min(raw_price)                                      AS price_min,
    max(raw_price)                                      AS price_max
FROM market_transactions
WHERE residual_rate_label IS NULL
  AND clean_status <> 'rejected'
  AND label_eligible
GROUP BY 1, 2, 3
ORDER BY 4 DESC, 1 ASC, 2 ASC, 3 ASC NULLS LAST;

COMMENT ON VIEW market_unmatched_queue IS
    '未匹配待审队列（#2676）：按品牌×型号×吨位聚合的未匹配样本，含样本数与成交价区间；人工补原价后 rematch 回收标签';
