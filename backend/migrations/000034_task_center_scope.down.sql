-- 回滚 #1076：重建每日登录事实源表、恢复成长分组与旧描述。
--
-- ⚠️ **已删的登录事实数据不可恢复**：up 里 DROP TABLE 掉的是「谁在哪天到访过」的流水，
-- 回滚只能把表结构建回来（空表）。需要这份历史的场合不要依赖回滚。

CREATE TABLE IF NOT EXISTS user_daily_login (
    user_id    INTEGER NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    login_date DATE    NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, login_date)
);
COMMENT ON TABLE user_daily_login IS '每日登录事实源：登录成功或 refresh 续期各落一行（Asia/Shanghai 自然日，主键幂等）';

UPDATE points_task_config SET description = '每日登录或续期会话即达成'
WHERE code = 'daily_login';

ALTER TABLE points_task_config DROP CONSTRAINT IF EXISTS points_task_config_group_check;

ALTER TABLE points_task_config
    ADD CONSTRAINT points_task_config_group_check CHECK ("group" IN ('daily','newbie','growth'));

COMMENT ON COLUMN points_task_config."group" IS NULL;

UPDATE points_task_config SET "group" = 'growth'
WHERE code IN ('growth_post','growth_reply','growth_mock');
