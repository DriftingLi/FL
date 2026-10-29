-- #2109 阶段 B：Redis 退场后，带 TTL 的临时 KV 收敛到单表。
--
-- 为什么是一张表而不是三张（验证码 / 图形码 / JWT 黑名单各一张）：三者在 Redis 时代共用同一个
-- keyspace、同一条「带 TTL 的 get / set / setnx」语义；拆表只会得到三份几乎相同的实现与测试。
--
-- 为什么过期时间由**应用写入**而不是 DEFAULT now()：读路径要拿「应用当前时间」判过期
-- （expires_at > ?），只有两侧同一时钟源，测试里才可用注入时钟复现过期；用库内 now() 会变成两套时间。
--
-- 过期行不靠 DELETE 即时回收：读路径自带 expires_at > ?，清扫滞后也不会读到过期值；
-- 清扫是 internal/daemon 里的一条 5 分钟守护（ADR-0061 §1 的登记表）。

CREATE TABLE IF NOT EXISTS ephemeral_kv (
    cache_key   varchar(255) PRIMARY KEY,
    cache_value text        NOT NULL,
    expires_at  timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ephemeral_kv_expires_at ON ephemeral_kv (expires_at);

COMMENT ON TABLE ephemeral_kv IS '带 TTL 的临时 KV（验证码 / 图形码 / JWT 黑名单）；读路径按应用时钟判过期';
