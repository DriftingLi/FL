-- ADR-0040 补丁：把「备考经验帖不可被采纳」从**前提**变成**被守卫的不变式**。
--
-- 背景：ADR-0040 把经验定为管理端认定，并声明经验帖「可加精、不可被采纳」。但认定
-- 不限制意图，于是有两条路径能造出「经验 + 已采纳」的组合：
--   ① 管理员直接认定一篇 question 帖（该帖仍可被采纳）；
--   ② 作者把已认定的 discussion 帖编辑成 question（UpdateTopic 只在「已采纳」时拦类别迁移）。
-- 该组合与领域边界冲突（一次性提问归问答、可复用经验输出归经验），且会让「经验区」里
-- 出现带采纳状态的帖子。本迁移按既有先例（000026 的 accepted_requires_question）补 CHECK，
-- 行为层由 service 双向守卫。
--
-- 先清洗历史悬挂行（预期为 0 行：本迁移之前不存在认定功能之外的组合，但生产若有人工
-- 直改库则可能留下）。只清认定标记，不动采纳状态与已发分——采纳链路的分按既有政策保留。
UPDATE forum_topics
   SET is_experience = false
 WHERE is_experience
   AND accepted_reply_id IS NOT NULL;

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_experience_not_accepted
    CHECK (NOT (is_experience AND accepted_reply_id IS NOT NULL));

-- 顺带更正 000025 留下的列注释：growth_first_experience 已随 ADR-0040 退役
-- （配置行由 000027 删除），该列现为无消费方的通用「任务上线时间」。
-- 不改 000025 本身——已应用的迁移是历史事实，不应回溯修改。
COMMENT ON COLUMN points_task_config.created_at IS '任务上线时间（#742 引入）：通用语义，供需要存量口径的任务使用；原消费方 growth_first_experience 已随 ADR-0040 退役';
