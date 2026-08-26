-- 000014: AI 助手功能化扩展
-- ai_chat_sessions 加 feature_key 区分各专项功能的会话（旧数据默认 ai_assistant）
-- ai_chat_messages 加 images 存用户消息附带图片 URL（JSON 数组字符串）

ALTER TABLE ai_chat_sessions
  ADD COLUMN feature_key VARCHAR(50) NOT NULL DEFAULT 'ai_assistant';

CREATE INDEX idx_ai_chat_sessions_user_feature ON ai_chat_sessions(user_id, feature_key);

ALTER TABLE ai_chat_messages
  ADD COLUMN images TEXT NULL;
