-- 回滚非 partial（user_id, task_code）索引（spec #408 / #410）
DROP INDEX IF EXISTS idx_points_task_claim_user_code;
