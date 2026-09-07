-- 下线被 fault_diagnosis 取代的纯 prompt 故障功能（见 ADR-0032）：
-- 清理存量 ai_feature_bindings 绑定行（fault_consult / fault_code_query 已从注册表移除，
-- 残留绑定行不再被任何解析面读取，仅留脏数据）。历史 ai_chat_sessions 行保留
-- （业务上旧会话不再展示，删除绑定不破坏外键——feature_key 为文本列，无引用约束）。
DELETE FROM ai_feature_bindings
WHERE feature_key IN ('fault_consult', 'fault_code_query');