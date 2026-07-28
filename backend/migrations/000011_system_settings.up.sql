-- 21. 系统设置表（key-value 结构，承载 AI 等模块的动态配置）
CREATE TABLE system_settings (
    key         VARCHAR(50) PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    description VARCHAR(255),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 初始化 AI 服务配置占位（生产环境由管理员后台填写，本地开发从环境变量读取）
INSERT INTO system_settings (key, value, description) VALUES
    ('ai_api_key',  '', 'AI 服务 API Key（OpenAI 兼容格式，生产环境动态配置）'),
    ('ai_base_url', '', 'AI 服务 Base URL（如 https://api.deepseek.com）'),
    ('ai_model',    '', 'AI 模型名（如 deepseek-v4-flash）')
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE system_settings IS '系统设置表（key-value 结构）';
