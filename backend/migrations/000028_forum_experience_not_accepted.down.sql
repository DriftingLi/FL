-- 回滚 000028。
--
-- 先删 CHECK 再恢复列注释：迁移 000025 的原始注释文本（历史值），
-- 使 down 后与本迁移之前的状态一致。

ALTER TABLE forum_topics DROP CONSTRAINT IF EXISTS chk_forum_topics_experience_not_accepted;

COMMENT ON COLUMN points_task_config.created_at IS '任务上线时间（#742）：growth_first_experience 达成判定以此为截止（仅发布晚于该时间的经验帖计达成）';
