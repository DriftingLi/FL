-- #1482：为 hrwai_users.wechat_unionid 建一条查询索引，供 App 端微信登录「先按 unionid 认人」。
--
-- 为什么需要它：App 端换取到的 openid 与小程序的不是同一个字符串，能跨端认出同一个自然人的
-- 只有 unionid（口径与一手依据见 docs/design/1482-app-wechat-login.md 第 3.2 节）。
-- WechatAppService.resolveAccount 认人的第一步就是 `WHERE wechat_unionid = ? ORDER BY id ASC`；
-- 而这一列在 000001 建出来之后只有写路径（微信登录落库）、从没有读路径，所以历来没有索引。
--
-- 为什么**不是**唯一索引：这条迁移要在生产存量库上跑，而「同一个 unionid 已经有两行」在仓内
-- 不可复算（历史上完全可能有人先以手机号建号、之后又绑过微信身份）。给它加唯一约束，代价是
-- 一条登录链路的增强随时可能变成一次发布阻塞——迁移失败比慢查询贵得多。
-- 重复行下的行为已由 resolveAccount 的 `ORDER BY id ASC` 定序（认最早那个账号），不靠约束兜。
-- 将来真要上唯一约束，先在生产量一次存量：
--     SELECT wechat_unionid, count(*) FROM hrwai_users
--      WHERE wechat_unionid <> '' GROUP BY wechat_unionid HAVING count(*) > 1;
--   返回空集是加约束的前置条件，不是「加了再说」。
--
-- 为什么是偏索引（WHERE <> ''）：该列默认空串，且绝大多数账号根本没有微信身份；
-- 全表索引等于把一堆空串条目塞进 B-tree，与本仓其余「空值不占唯一位」的偏索引同一口径。

CREATE INDEX idx_hrwai_users_wechat_unionid
    ON hrwai_users (wechat_unionid)
    WHERE wechat_unionid <> '';
