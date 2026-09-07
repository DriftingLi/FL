-- 回滚 T5 来源持久化列（数据列删除，存量来源丢失；消息正文不受影响）。
ALTER TABLE ai_chat_messages
    DROP COLUMN IF EXISTS sources;
