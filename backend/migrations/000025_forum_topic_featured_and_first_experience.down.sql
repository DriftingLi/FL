-- 回滚 #742 批次一：撤首篇经验专项分任务与精选位。
--
-- 删任务配置行会经 points_task_claim 的 ON DELETE CASCADE 级联清除该任务的领取记录，
-- 保证 down/up 可重放（重放后用户可再次领取——回滚即放弃已发放状态，属既定代价）。

DELETE FROM points_task_config WHERE code = 'growth_first_experience';

ALTER TABLE points_task_config DROP COLUMN IF EXISTS created_at;

ALTER TABLE forum_topics DROP COLUMN IF EXISTS is_featured;
