-- 消息元数据持久化（票 2245 决策、票 2242 落地）：
-- 助手消息行新增 meta JSON 列，存 v2 `done` 事件里的能力徽章与端到端耗时，
-- 供历史回看时把「这次回答用了哪些能力、花了多久」原样重放。
--
-- 为什么是 TEXT 而不是 JSONB：与同级 sources 列同形（000023 也是 TEXT），
-- 读侧一律 `json.Unmarshal` 成 DTO 的 *AIChatMessageMeta，写失败按「无元数据」处理，
-- 不做任何 SQL 侧查询 —— 既然列从不参与过滤/聚合，JSONB 的类型校验换不来判据。
--
-- 为什么不存 trace：契约自己写着「排查用，可不展示给用户」，对方存了却从不回放
-- （app.js:505 传空数组），学它只会多一份没人读的数据（票 2245 决策）。
--
-- 存量行为不变：NULL 让所有历史消息读出来是 nil 指针，DTO 序列化成字面 `null`，
-- 与 images / sources 两格的缺席形状一致。
ALTER TABLE ai_chat_messages
    ADD COLUMN IF NOT EXISTS meta TEXT;

COMMENT ON COLUMN ai_chat_messages.meta IS
    '助手消息元数据 JSON（{"badges":[{"name","emoji","cls"}],"elapsed":12.3}；票 2245/2242，空串=NOW 之前的历史行）';
