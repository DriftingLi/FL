-- #587 每日登录事实源 + 打卡任务替换（ADR-0028）。
--
-- user_daily_login：每日登录任务的事实源。
--   - 登录成功与 refresh 轮换（会话续期）均写入一行；(user_id, login_date) 复合主键幂等，
--     同一自然日多次登录/续期不重复计数。
--   - login_date 为 Asia/Shanghai 自然日日期，类型标注照 forum_checkin.check_date 先例
--     （DATE 类型；读写均以业务时区 00:00 为口径）。
--   - 挂 hrwai_users ON DELETE CASCADE（注销即清，与 forum_checkin 同构）。
CREATE TABLE IF NOT EXISTS user_daily_login (
    user_id    INTEGER NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    login_date DATE    NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, login_date)
);
COMMENT ON TABLE user_daily_login IS '每日登录事实源：登录成功或 refresh 续期各落一行（Asia/Shanghai 自然日，主键幂等）';

-- 任务中心：每日打卡（daily_checkin，领取制）退役 → 每日登录（daily_login）。
-- 打卡收益改由打卡模块直记（ADR-0028），任务中心不再承载「打卡领取」。
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES ('daily_login', '每日登录', 'daily', 5, 1, NULL, 'login', '每日登录或续期会话即达成')
ON CONFLICT (code) DO NOTHING;
DELETE FROM points_task_config WHERE code = 'daily_checkin';

-- 完善资料任务描述去掉「（2/2）」（该计数随 total=2 + progress 真实化由前端进度条呈现，
-- 描述内嵌「2/2」会与新进度读数并存造成误导，spec「完善资料进度异常」）。
UPDATE points_task_config SET description = '上传头像且设置昵称'
WHERE code = 'newbie_profile_basic' AND description LIKE '%（2/2）%';
UPDATE points_task_config SET description = '填写单位且绑定手机/邮箱'
WHERE code = 'newbie_profile_contact' AND description LIKE '%（2/2）%';
