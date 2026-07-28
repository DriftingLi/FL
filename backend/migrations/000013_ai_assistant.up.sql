-- AI 助手模块：会话、消息、用户自定义模型配置
-- 依赖 valuation_users 表（残值评估独立用户）

-- AI 助手会话表
CREATE TABLE ai_chat_sessions (
    id          SERIAL         PRIMARY KEY,
    user_id     INTEGER        NOT NULL,  -- 关联 valuation_users.id
    title       VARCHAR(200)   NOT NULL DEFAULT '新会话',
    model_name  VARCHAR(100),             -- 本次会话使用的模型名
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ai_chat_sessions_user ON ai_chat_sessions (user_id, created_at DESC);

-- AI 助手消息表
CREATE TABLE ai_chat_messages (
    id          SERIAL         PRIMARY KEY,
    session_id  INTEGER        NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    role        VARCHAR(20)    NOT NULL,  -- 'user' | 'assistant' | 'system'
    content     TEXT           NOT NULL,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ai_chat_messages_session ON ai_chat_messages (session_id, created_at);

-- 用户自定义模型配置表（用户自行配置的 openai 格式 apikey）
CREATE TABLE ai_user_models (
    id          SERIAL         PRIMARY KEY,
    user_id     INTEGER        NOT NULL,  -- 关联 valuation_users.id
    name        VARCHAR(50)    NOT NULL,  -- 用户自定义名称
    api_key     TEXT           NOT NULL,
    base_url    VARCHAR(255)   NOT NULL,
    model       VARCHAR(100)   NOT NULL,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, model)  -- 同用户同模型名只能有一个
);
CREATE INDEX idx_ai_user_models_user ON ai_user_models (user_id);
