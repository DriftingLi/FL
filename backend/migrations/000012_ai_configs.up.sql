-- 多 AI 配置表：支持管理员保存多套命名配置，可绑定到不同 AI 功能
CREATE TABLE ai_configs (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    api_key     TEXT NOT NULL,
    base_url    VARCHAR(255) NOT NULL,
    model       VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_configs_active ON ai_configs (is_active);

-- AI 功能-配置绑定表：feature_key 唯一，每功能只能绑定一个配置
CREATE TABLE ai_feature_bindings (
    feature_key VARCHAR(50) PRIMARY KEY,
    config_id   INTEGER NOT NULL REFERENCES ai_configs(id) ON DELETE CASCADE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 初始化两个功能绑定（NULL config_id 占位通过允许 config_id NULL 实现，这里改为：未绑定时无记录）
COMMENT ON TABLE ai_configs IS 'AI 服务配置表（多套命名配置）';
COMMENT ON TABLE ai_feature_bindings IS 'AI 功能-配置绑定表（feature_key → config_id）';
