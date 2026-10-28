-- 回滚 000044：整表删除（索引随表走）。
--
-- 该表是本票新增、无历史数据 ⇒ 回滚只丢失「职位上声明的所需证件」，
-- 职位本身与其它列（含 status/forced_offline）一行不动。

DROP TABLE IF EXISTS job_credential_requirements;
