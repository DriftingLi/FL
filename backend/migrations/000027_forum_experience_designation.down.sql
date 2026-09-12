-- 回滚 ADR-0040 的认定改造。
--
-- **有损性如实声明**：降级后若存量行被编辑为 question 并采纳、或被重新认定，无法逐一还原。
-- 本 down 只还原「留痕列表内 + 仍是 discussion + 无采纳指针」的行，其余保持不动——
-- 宁可少还原，也不让 down 因 CHECK（chk_forum_topics_accepted_requires_question）中断。
--
-- 任务配置行会重新插入：已领取记录不回滚，故回滚后可能再次可领一次。
-- 这与 up 的取舍对称（up 删行时同样不追回已发分），属已知代价。

ALTER TABLE forum_topics DROP CONSTRAINT IF EXISTS chk_forum_topics_experience_requires_featured;

DROP INDEX IF EXISTS idx_forum_topics_experience_created;
DROP INDEX IF EXISTS idx_forum_topics_experience_hot;

-- 先放宽值域，再还原行（顺序与 up 相反、理由相同）。
ALTER TABLE forum_topics DROP CONSTRAINT chk_forum_topics_category;

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_category
    CHECK (category IN ('discussion', 'question', 'experience'));

UPDATE forum_topics
   SET category = 'experience'
 WHERE category = 'discussion'
   AND accepted_reply_id IS NULL
   AND id::text = ANY (
       string_to_array(
           COALESCE((SELECT value FROM system_settings WHERE key = 'forum_legacy_experience_ids'), ''),
           ','
       )
   );

DELETE FROM system_settings WHERE key = 'forum_legacy_experience_ids';

ALTER TABLE forum_topics DROP COLUMN IF EXISTS is_experience;

INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES ('growth_first_experience', '发布首篇备考经验', 'growth', 20, 1, 1, 'topic_create',
        '发布首篇备考经验帖（终身一次；任务上线后发布的经验帖计达成）')
ON CONFLICT (code) DO NOTHING;
