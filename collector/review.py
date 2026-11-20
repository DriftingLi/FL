"""未匹配待审队列的列名契约（#2676）。

对齐 #2671「原价反哺」决策：**不自动写回原价表**；未匹配样本按
品牌×型号×吨位聚合成待审队列（一期为聚合视图，无管理端 UI），
人工定期审核补原价条目，补入后 `collector rematch` 重跑匹配回收标签。

口径的**唯一真相在迁移 000067 的 `market_unmatched_queue` 视图**：视图的
WHERE / GROUP BY / ORDER BY 就是判定本身。本模块只声明聚合列名，供
`store.load_unmatched_queue` 的 SELECT 与 `cli` 的表头共用，避免列名在两处
手抄漂移；不在 Python 里另写一份聚合实现——同一判定两份实现必然漂移
（历史上排序就曾不一致），而口径已由真 PG 契约测试逐条钉住。
"""

from __future__ import annotations

# 聚合列：品牌 / 型号 / 吨位 + 样本数 + 成交价区间。必须与迁移 000067 的视图列一一对应。
UNMATCHED_GROUP_COLUMNS = (
    "brand", "model", "tonnage", "sample_count", "price_min", "price_max",
)
