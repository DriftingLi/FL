-- 非 partial 的（user_id, task_code）索引：终身计数取数形态支撑（spec #408 / #410）
-- 现有两个唯一索引（uq_points_task_claim_daily / uq_points_task_claim_ref）都是 partial，
-- 各只覆盖一臂；按 task_code 分组的终身计数走本条普通索引。
-- IF NOT EXISTS 幂等：既有部署若已存在同名索引则跳过（000002 曾建 idx_points_task_claim_user）。
CREATE INDEX IF NOT EXISTS idx_points_task_claim_user_code
    ON points_task_claim (user_id, task_code);
COMMENT ON INDEX idx_points_task_claim_user_code IS '领取占坑按（用户、任务码）分组的取数索引（#410 终身计数）';
