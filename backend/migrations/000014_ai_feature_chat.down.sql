ALTER TABLE ai_chat_messages DROP COLUMN images;

DROP INDEX idx_ai_chat_sessions_user_feature;

ALTER TABLE ai_chat_sessions DROP COLUMN feature_key;
