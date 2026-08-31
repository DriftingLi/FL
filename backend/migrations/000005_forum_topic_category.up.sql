-- #364 论坛类别分流：主题获得「讨论 / 问答」两个类别。
--
-- 单表 + 判别列：回复 / 点赞 / 举报 / 收藏 / 图片上传 / 浏览历史 / 全局搜索 / 既有积分任务
-- 一律不动，继续按 topic_id 工作，天然覆盖新类别。
--
-- 存量帖子由 DEFAULT 'discussion' 直接回填（ADD COLUMN ... NOT NULL DEFAULT 在 Postgres
-- 中会把已有行填成默认值），因此不需要额外的 UPDATE 回填语句。

ALTER TABLE forum_topics
    ADD COLUMN category VARCHAR(16) NOT NULL DEFAULT 'discussion';

COMMENT ON COLUMN forum_topics.category IS '帖子类别：discussion=讨论 / question=问答；判别帖子意图的唯一依据（判区域仍看 chapter_id）';

-- 类别值域。不预留第三个值：求职信息是常驻实体，不在论坛内。
ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_category CHECK (category IN ('discussion', 'question'));

-- 非法组合在库层无法表示：问答帖一律不属于任何章节（问答帖 chapter_id 恒为 NULL）。
-- 行为层（service 返回 400）由 forum_category_contract_test.go 守住；
-- 本约束只在生产生效 —— 测试库由 AutoMigrate 建表、不执行本文件，故契约测试覆盖不到它。
ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_question_no_chapter CHECK (category <> 'question' OR chapter_id IS NULL);

-- 索引：与既有两条排序索引同构，只是前面多一列 category。
-- 学员端两个 Tab 的查询都带 category，且必须与 scope（chapter_id IS NULL）落在同一条 WHERE 里：
--   讨论 Tab = category='discussion' AND chapter_id IS NULL
--   问答 Tab = category='question'（chapter_id 恒 NULL，无需再筛区域）
CREATE INDEX idx_forum_topics_category_created
    ON forum_topics (category, created_at DESC);
CREATE INDEX idx_forum_topics_category_hot
    ON forum_topics (category, likes_count DESC, reply_count DESC, view_count DESC, id DESC);
