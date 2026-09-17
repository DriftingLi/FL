-- #1076（ADR-0054）：任务中心口径收敛 —— 每日登录无条件化 + 成长分组退役。
--
-- 两件事：
--  1. 「每日登录」（daily_login）的行为前置退役：达成判定不再依赖「当日 user_daily_login
--     行存在」。该判定原本挂在**登录成功**与 **/auth/refresh 续期**两个落表入口上，而客户端
--     是否续期取决于 access token 是否过期（Web 端「本地未过期就直接用」）—— 于是 23:50 刷过
--     token 的学员次日 00:30 回来，token 仍在 2h 有效期内，当天不落表、任务领不了：
--     **客户端实现细节在决定一项每日任务的可用性**。判定改为「无行为前置」（service 侧
--     taskProgressFor 返回 nil，走既有 default 分支），事实源表随之删除。
--  2. 「成长任务」分组退役：growth_post / growth_reply / growth_mock 三条的
--     (daily_limit, total_limit) 与三条 daily_* 任务**逐字一致**（daily_limit=1,
--     total_limit=NULL）—— group 本就是 total_limit 的冗余投影（非空=终身一次 / 空=每日可再生），
--     'growth' 是贴错的标签。存量行改判为 'daily' 后再收窄 CHECK。
--
-- 顺序不可颠倒：先 UPDATE 再收窄 CHECK —— Postgres 加 CHECK 时校验存量行，
-- 若先收窄则存量 'growth' 行会让迁移直接失败。
--
-- 删表前确认无消费方：user_daily_login 的读路径只有 PointsService.hasDailyLogin，
-- 写路径为登录签发与 /auth/refresh 轮换，另有注销时的显式清理；四条路径随本次改动
-- 在同一提交内一并删除（含模型与 AutoMigrate 注册）。

UPDATE points_task_config SET "group" = 'daily' WHERE "group" = 'growth';

ALTER TABLE points_task_config DROP CONSTRAINT IF EXISTS points_task_config_group_check;

ALTER TABLE points_task_config
    ADD CONSTRAINT points_task_config_group_check CHECK ("group" IN ('daily','newbie'));

COMMENT ON COLUMN points_task_config."group" IS '任务分组（ADR-0054）：daily=每日可再生（total_limit 为空）/ newbie=终身一次（total_limit 非空）。group 是 total_limit 的投影，不是独立口径';

-- 每日登录去掉行为前置后，旧描述「每日登录或续期会话即达成」不再成立。
UPDATE points_task_config SET description = '每日进入任务中心即可领取'
WHERE code = 'daily_login';

DROP TABLE IF EXISTS user_daily_login;
