-- 添加 AI 模型配置
-- 包含：Claude、Qwen3.8 Flash、GLM5.3 Flash、MiMo2.5、DeepSeek V4 Flash

-- 注意：api_key 字段在生产环境会被加密存储，这里使用占位符
-- 实际部署时需要通过管理后台或 API 更新真实的 API Key

-- 1. Claude (Anthropic)
INSERT INTO ai_configs (name, api_key, base_url, model, description, is_active) VALUES
    ('Claude', 'sk-d9e292b59e2c55c695824e84f93b479ea75f7d38090edb202dd60aad7c95e365', 'https://api.anthropic.com', 'claude-opus-5', 'Anthropic Claude 模型', true)
ON CONFLICT (name) DO UPDATE SET
    api_key = EXCLUDED.api_key,
    base_url = EXCLUDED.base_url,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    updated_at = now();

-- 2. Qwen3.8 Flash (阿里云百炼)
INSERT INTO ai_configs (name, api_key, base_url, model, description, is_active) VALUES
    ('Qwen3.8 Flash', 'sk-d9e292b59e2c55c695824e84f93b479ea75f7d38090edb202dd60aad7c95e365', 'https://dashscope.aliyuncs.com/compatible-mode/v1', 'qwen3.8-flash', '阿里云通义千问 Qwen3.8 Flash 模型', true)
ON CONFLICT (name) DO UPDATE SET
    api_key = EXCLUDED.api_key,
    base_url = EXCLUDED.base_url,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    updated_at = now();

-- 3. GLM5.3 Flash (智谱)
INSERT INTO ai_configs (name, api_key, base_url, model, description, is_active) VALUES
    ('GLM5.3 Flash', 'sk-d9e292b59e2c55c695824e84f93b479ea75f7d38090edb202dd60aad7c95e365', 'https://open.bigmodel.cn/api/paas/v4/', 'glm-5.3-flash', '智谱 GLM5.3 Flash 模型', true)
ON CONFLICT (name) DO UPDATE SET
    api_key = EXCLUDED.api_key,
    base_url = EXCLUDED.base_url,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    updated_at = now();

-- 4. MiMo2.5 (小米)
INSERT INTO ai_configs (name, api_key, base_url, model, description, is_active) VALUES
    ('MiMo2.5', 'sk-d9e292b59e2c55c695824e84f93b479ea75f7d38090edb202dd60aad7c95e365', 'https://api.xiaomimimo.com/v1', 'mimo-v2.5', '小米 MiMo2.5 模型', true)
ON CONFLICT (name) DO UPDATE SET
    api_key = EXCLUDED.api_key,
    base_url = EXCLUDED.base_url,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    updated_at = now();

-- 5. DeepSeek V4 Flash
INSERT INTO ai_configs (name, api_key, base_url, model, description, is_active) VALUES
    ('DeepSeek V4 Flash', 'sk-d9e292b59e2c55c695824e84f93b479ea75f7d38090edb202dd60aad7c95e365', 'https://api.deepseek.com', 'deepseek-v4-flash', 'DeepSeek V4 Flash 模型', true)
ON CONFLICT (name) DO UPDATE SET
    api_key = EXCLUDED.api_key,
    base_url = EXCLUDED.base_url,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    updated_at = now();
