-- #821/#823（ADR-0040）：备考经验从「学员自述的类别」变为「管理端认定的归类」。
--
-- 四件事：
--  1. 新增 is_experience（管理端认定），CHECK 保证 is_experience ⇒ is_featured
--     （经验区是精选的子集，不存在「经验但非精选」）。
--  2. category 值域收窄回 discussion|question（学员自述的意图）；存量 category='experience'
--     行一律降级为 'discussion'，**不发分、不追回**（ADR-0040 存量口径：自称不构成认定）。
--  3. 降级前把存量行 id 留成策展线索写入 system_settings ——降级后无法再从 category 区分
--     「原本自称经验」与「本来就是闲聊」，管理员重新策展需要这份待办清单。
--     线索**不含认定语义**：不置 is_experience、不发分、不进经验 Tab。
--  4. 删除 growth_first_experience 任务配置行（认定奖励已与加精合并为同一笔 featured_bonus）。
--
-- 顺序不可颠倒：先 UPDATE 再收窄 CHECK——Postgres 加 CHECK 时校验存量行，
-- 若先收窄则存量 experience 行会让迁移直接失败。

ALTER TABLE forum_topics
    ADD COLUMN is_experience BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN forum_topics.is_experience IS '备考经验认定（ADR-0040）：管理端授予的内容归类，非学员自述；蕴含 is_featured';

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_experience_requires_featured
    CHECK (NOT (is_experience AND NOT is_featured));

-- 降级前留痕（见上文第 3 点）。无存量行时写入空串，语义「无待策展」。
INSERT INTO system_settings (key, value, description, updated_at)
SELECT 'forum_legacy_experience_ids',
       COALESCE(string_agg(id::text, ',' ORDER BY id), ''),
       'ADR-0040 降级的存量经验帖 id 列表（管理端重新策展参考；不含认定语义）',
       NOW()
  FROM forum_topics
 WHERE category = 'experience'
ON CONFLICT (key) DO UPDATE
   SET value = EXCLUDED.value,
       description = EXCLUDED.description,
       updated_at = EXCLUDED.updated_at;

UPDATE forum_topics SET category = 'discussion' WHERE category = 'experience';

ALTER TABLE forum_topics DROP CONSTRAINT chk_forum_topics_category;

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_category
    CHECK (category IN ('discussion', 'question'));

COMMENT ON COLUMN forum_topics.category IS '帖子意图（ADR-0040）：discussion=讨论 / question=问答。学员自述，不含认定；判「是不是经验帖」看 is_experience';

-- 经验 Tab 的部分索引：只覆盖认定行，体积远小于全表，与既有两条 category 索引同构。
CREATE INDEX idx_forum_topics_experience_created
    ON forum_topics (created_at DESC) WHERE is_experience;
CREATE INDEX idx_forum_topics_experience_hot
    ON forum_topics (likes_count DESC, reply_count DESC, view_count DESC, id DESC) WHERE is_experience;

DELETE FROM points_task_config WHERE code = 'growth_first_experience';
