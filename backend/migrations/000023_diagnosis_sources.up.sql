-- 智能维修诊断来源持久化（T5，见 #672；决策见 ADR-0033，取代 ADR-0032「历史不持久化」取舍）：
-- 助手消息行新增 sources JSON 列，随助手消息存 answer_sources，供历史回看展开。
-- 存量行为 NULL（回放时按无来源处理）；与 ADR-0032「历史不持久化 sources」的取舍变更同步。
ALTER TABLE ai_chat_messages
    ADD COLUMN IF NOT EXISTS sources TEXT;
