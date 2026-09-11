-- #811 收口：已采纳的帖子只能是问答帖（采纳状态只在问答帖有意义）。
--
-- 行为层由 service 守住（UpdateTopic 拒绝「已采纳帖改类别」，文案「已采纳的问答帖不能改类别，请先取消采纳」）；
-- 本迁移是生产兜底 CHECK——与 000005 的类别值域/问答帖不挂章节两条 CHECK 同精神：
-- 「非问答帖带 accepted_reply_id」是悬挂态，容忍它会让 solved 筛选与采纳链路判定读到脏数据。
--
-- 先清零历史悬挂行（生产 2026-09-11 核过：orphan_accepted=0，此步为 no-op；
-- 测试/临时库若留过实验数据也不致迁移失败）。只清指针不清分：
-- 答主已发的 accepted_bonus 按既有政策「取消采纳不回滚」保留（ADR/积分流水不可变）。
UPDATE forum_topics
   SET accepted_reply_id = NULL,
       solved_at         = NULL
 WHERE accepted_reply_id IS NOT NULL
   AND category <> 'question';

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_accepted_requires_question
    CHECK (accepted_reply_id IS NULL OR category = 'question');
