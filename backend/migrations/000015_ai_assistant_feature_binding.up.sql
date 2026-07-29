-- 扩展 ai_feature_bindings 表支持多对多绑定
-- 背景：AI 助手需要绑定多个配置（供用户前端选择），同 model 唯一
-- 其它功能（简答题评分、课程内容生成）保持单绑定机制（应用层限制）

-- 1. 添加自增 id 列作为新主键
ALTER TABLE ai_feature_bindings ADD COLUMN IF NOT EXISTS id SERIAL;

-- 2. 删除原 feature_key 主键约束
ALTER TABLE ai_feature_bindings DROP CONSTRAINT IF EXISTS ai_feature_bindings_pkey;

-- 3. 设置新主键为 id
ALTER TABLE ai_feature_bindings ADD PRIMARY KEY (id);

-- 4. 添加复合唯一约束 (feature_key, config_id)
-- 同一功能不能重复绑定同一配置
ALTER TABLE ai_feature_bindings ADD CONSTRAINT uq_ai_feature_config UNIQUE (feature_key, config_id);

-- 5. 为 feature_key 创建索引（保留单绑定功能的快速查询）
CREATE INDEX IF NOT EXISTS idx_ai_feature_bindings_feature_key ON ai_feature_bindings (feature_key);

-- 6. 更新表注释
COMMENT ON TABLE ai_feature_bindings IS 'AI 功能-配置绑定表（支持单绑定和多绑定功能）';
