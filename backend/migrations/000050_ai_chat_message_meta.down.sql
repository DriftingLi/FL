-- 回滚 000050：删掉 ai_chat_messages.meta（消息正文、图片与来源列一行不动）。
--
-- 已落库的徽章与耗时随列删除而丢失：它是回答的**附注**（本次用了哪些能力、花了多久），
-- 不是回答本身，回滚后历史回放退化成「只有正文」，与 2242 之前的读面相一致。
ALTER TABLE ai_chat_messages DROP COLUMN IF EXISTS meta;
