-- 回滚 #811 收口：撤掉「已采纳只能是问答帖」的 CHECK。
-- 注意：down 不回填已清空的悬挂采纳指针（信息已丢失，且悬挂态本就无意义）——
-- down → up 重放后约束恢复，数据回到「无悬挂」状态。
ALTER TABLE forum_topics
    DROP CONSTRAINT IF EXISTS chk_forum_topics_accepted_requires_question;
