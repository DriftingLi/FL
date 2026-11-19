-- #2674 收口：清洗规则版本落库，让「规则版本化」可审计。
--
-- 第一性原理：清洗决定一个样本进不进弱标签集。若行上不记录是哪版规则判的，
-- 就无法回答「这条剔除是 v1 还是 v2 的规则产出」，也无法在规则升级后
-- 只重算旧版本行。与抽取侧 extraction_version 对称，清洗侧补一列 clean_version。
-- 这是清洗列（reclean 可写），不碰抽取 / 匹配 / 标签列，评估路径零改动。

ALTER TABLE market_transactions
    ADD COLUMN IF NOT EXISTS clean_version VARCHAR(32);

COMMENT ON COLUMN market_transactions.clean_version IS
    '#2674：产出本行清洗结果所用的 collector/clean/rules.py 版本（CLEAN_VERSION）；重跑清洗时更新为当前版本';
