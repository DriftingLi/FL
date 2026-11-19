-- 回滚 000063：删除 market_transactions 新表（逆序）。
--
-- 该表是本票新建的独立表，不承载任何存量业务数据；回滚只丢弃本管线采集的样本，
-- 对评估记录表 / 评估路径零影响（spec #2671 硬约束）。索引与约束随表一并消失。

DROP TABLE IF EXISTS market_transactions;
