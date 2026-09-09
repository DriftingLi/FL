-- #742 批次一：全类别精选位 + 首篇经验专项分任务。

-- 精选位：帖子级质量信号（通用列，全类别可用）。管理端可精/可撤；
-- 三 Tab 精选筛选与标识展示；加精奖励 featured_bonus 见积分流水（featured_bonus:{topicID} 幂等）。
ALTER TABLE forum_topics
    ADD COLUMN is_featured BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN forum_topics.is_featured IS '精选位（#742）：管理端全类别可精/可撤，三 Tab 筛选与标识展示';

-- 任务上线时间：growth_first_experience 的存量口径 cutoff（「按任务上线后首次发布计」，
-- 上线前发布的存量经验帖不补发）。既有行回填为迁移应用时间，无其他消费方，无副作用。
ALTER TABLE points_task_config
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

COMMENT ON COLUMN points_task_config.created_at IS '任务上线时间（#742）：growth_first_experience 达成判定以此为截止（仅发布晚于该时间的经验帖计达成）';

-- 首篇经验专项分：growth 组，+20 终身一次（total_limit=1）。
-- 达成判定：存在发布时间晚于 created_at（任务上线时间）的 experience 帖；
-- 发帖通用口径 growth_post +10/日 不受影响。
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES ('growth_first_experience', '发布首篇备考经验', 'growth', 20, 1, 1, 'topic_create', '发布首篇备考经验帖（终身一次；任务上线后发布的经验帖计达成）')
ON CONFLICT (code) DO NOTHING;
