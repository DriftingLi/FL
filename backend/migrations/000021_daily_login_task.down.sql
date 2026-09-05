-- #587 每日登录事实源回滚：删表、还原任务配置（daily_login → daily_checkin 与旧描述）。
DELETE FROM points_task_config WHERE code = 'daily_login';
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES ('daily_checkin', '每日打卡', 'daily', 5, 1, NULL, 'check_in', '每日打卡成功')
ON CONFLICT (code) DO NOTHING;
UPDATE points_task_config SET description = '上传头像且设置昵称（2/2）'
WHERE code = 'newbie_profile_basic' AND description = '上传头像且设置昵称';
UPDATE points_task_config SET description = '填写单位且绑定手机/邮箱（2/2）'
WHERE code = 'newbie_profile_contact' AND description = '填写单位且绑定手机/邮箱';
DROP TABLE IF EXISTS user_daily_login;
