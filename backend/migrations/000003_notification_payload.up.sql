-- ADR-0009 / Ticket #228：通知结构化 payload（JSONB 标记，加性契约）。
-- 资料审核等业务事件的通知携带结构化标记（如 {"review_status":"approved"|"rejected"}），
-- 前端据此做确定性判定，不再依赖标题文案。
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS payload JSONB;
