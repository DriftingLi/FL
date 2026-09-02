-- 回滚：移除微信号列。
ALTER TABLE recruiter_users DROP COLUMN IF EXISTS wechat;
