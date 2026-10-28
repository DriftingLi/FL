-- 回滚 000045：删任务配置行 + 删浏览事实源表。
--
-- 先删配置行再删表：points_task_claim.task_code 对配置行是 ON DELETE CASCADE，
-- 回滚会连带清掉该任务的领取占坑行——这是预期行为（任务本身没了，占坑没有意义），
-- 但它是**不可恢复**的：回滚后重新 up，此前领过的分不会重发（流水与余额不动，见 ADR-0028/0023）。

DELETE FROM points_task_config WHERE code = 'browse_course';
DROP TABLE IF EXISTS course_views;
