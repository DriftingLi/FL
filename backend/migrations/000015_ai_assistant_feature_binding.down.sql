-- 回滚：恢复 ai_feature_bindings 表为单绑定结构

-- 1. 删除复合唯一约束
ALTER TABLE ai_feature_bindings DROP CONSTRAINT IF EXISTS uq_ai_feature_config;

-- 2. 删除 feature_key 索引
DROP INDEX IF EXISTS idx_ai_feature_bindings_feature_key;

-- 3. 删除 id 主键约束
ALTER TABLE ai_feature_bindings DROP CONSTRAINT IF EXISTS ai_feature_bindings_pkey;

-- 4. 删除 id 列
ALTER TABLE ai_feature_bindings DROP COLUMN IF EXISTS id;

-- 5. 恢复 feature_key 为主键
ALTER TABLE ai_feature_bindings ADD PRIMARY KEY (feature_key);

COMMENT ON TABLE ai_feature_bindings IS 'AI 功能-配置绑定表（feature_key → config_id）';
