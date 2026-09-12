-- #706/#722 备考经验进场：帖子类别扩枚举 experience。
--
-- 只放宽值域 CHECK，chk_forum_topics_question_no_chapter 保持原样：
-- 该约束只禁 question+章节，experience 可挂章节（服务层同口径，勿收紧）。
-- 测试库走 AutoMigrate 不执行本文件，CHECK 由迁移脚本自身守住（migration-check 验证）。

ALTER TABLE forum_topics
    DROP CONSTRAINT chk_forum_topics_category;

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_category
    CHECK (category IN ('discussion', 'question', 'experience'));

COMMENT ON COLUMN forum_topics.category IS '帖子类别：discussion=讨论 / question=问答 / experience=备考经验；判别帖子意图的唯一依据（判区域仍看 chapter_id）';
