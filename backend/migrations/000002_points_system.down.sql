-- 回滚积分基座（逆序删表/列）
DROP TABLE IF EXISTS user_entitlement;
DROP TABLE IF EXISTS points_shop_item;
ALTER TABLE course DROP COLUMN IF EXISTS points_price;
ALTER TABLE hrwai_users DROP COLUMN IF EXISTS points_balance;
DROP TABLE IF EXISTS points_user_progress;
DROP TABLE IF EXISTS points_task_claim;
DROP TABLE IF EXISTS points_ledger;
DROP TABLE IF EXISTS points_task_config;
