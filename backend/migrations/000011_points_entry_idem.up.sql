-- 通用积分簿记幂等占坑（ADR-0023）
-- 把 points_task_claim 的占坑先例推广到全部「流水直记」路径：
-- 「一事件一分」与回收 settle 传确定性键（accepted_bonus:{topicID} / rollback:{topicID} / redeem:{ref} 等），
-- 占坑行即「已处理」标记；占坑冲突 = 事件已处理，整笔事务回滚。
-- 依赖：000002_points_system（hrwai_users 需先存在语义校验方）；本表零外键（键为业务语义字符串）。

CREATE TABLE IF NOT EXISTS points_entry_idem (
  idem_key   VARCHAR(128) PRIMARY KEY,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE points_entry_idem IS '通用积分簿记幂等占坑（占坑行即「已处理」标记，ADR-0023）';
