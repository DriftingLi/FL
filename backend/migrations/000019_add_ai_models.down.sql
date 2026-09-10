-- 回滚：删除添加的 AI 模型配置

DELETE FROM ai_configs WHERE name IN (
    'Claude',
    'Qwen3.8 Flash',
    'GLM5.3 Flash',
    'MiMo2.5',
    'DeepSeek V4 Flash'
);
