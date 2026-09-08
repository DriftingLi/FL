-- 回滚 #706/#722：收紧类别值域至 discussion/question。
--
-- 必须先把 experience 帖归为 discussion 再收紧 CHECK，保证 up/down 可重放
-- （收紧后 experience 行会违反约束）。⚠️ 回滚会把备考经验帖改回讨论帖，
-- 类别信息不保留 —— 这是可重放的既定代价（#722 裁定）。

UPDATE forum_topics SET category = 'discussion' WHERE category = 'experience';

ALTER TABLE forum_topics
    DROP CONSTRAINT chk_forum_topics_category;

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_category
    CHECK (category IN ('discussion', 'question'));

COMMENT ON COLUMN forum_topics.category IS '帖子类别：discussion=讨论 / question=问答；判别帖子意图的唯一依据（判区域仍看 chapter_id）';
