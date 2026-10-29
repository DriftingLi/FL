# ADR-0090：Redis 退场 —— 临时 KV 收敛到 PG 单表、积分锁改 advisory lock、缓存层整体退役

- 状态：Accepted（2026-10-29）
- 日期：2026-10-29
- 领域：后端基础设施 / 部署（承接 ADR-0002 会话单一接口、ADR-0023 幂等簿记与锁、ADR-0060 配置默认值单点、ADR-0061 守护登记、ADR-0088 部署面；阶段 A 的 PG 18 升级见 #2109 与 `deploy/README.md` 的升级 runbook）
- 判据物：`backend/internal/kv/`（KV 语义与 TTL）、`internal/points`（advisory lock 与唯一索引分工）、`deploy/tests/`（`FORCE_DATA_SERVICES` 真值面）、`internal/deploy/topology_test.go`（漂移锁）

## 背景

Redis 在本仓承担六类用途，全部在 `backend/internal/cache` 这一个包后面：

1. 字典 / 级联 / 系数的读缓存（`dict:*`，60 分钟）；
2. 评估记录详情 / 列表 / 统计缓存（`eval:*`，10 / 5 分钟）；
3. 估价结果缓存（按请求 JSON 的 SHA256，3 分钟）；
4. AI 功能绑定缓存（`ai:bindings`，5 分钟）；
5. 短命状态：验证码（`email_code:*` / `phone_code:*` + 1 分钟节流）、图形码（`captcha:*`，校验即删）、JWT refresh 黑名单与身份吊销标记；
6. 积分入口的瞬态 `SETNX` 锁（`points:grant:*` / `shop:*` / `ai:tokens:*` / `points:penalty:*`）。

生产 Redis 从第一天起就是**纯缓存定位**（`--save "" --appendonly no`、`allkeys-lru`、200MB 上限），
所以它重启即丢黑名单与验证码；而它同时把「缓存可用性」与「数据可用性」拆成两层依赖。
访客量长期≈0，缓存层几乎不产生收益，维护成本（一个容器 + 一套配置 + 一套降级/fail-open 语义）却是实打实的。

## 决策

1. **临时 KV 收敛到 PG 单表 `ephemeral_kv`**（验证码 / 图形码 / 黑名单共用）。
   过期时间由**应用时钟**写入与判定（`expires_at > ?`），不用库内 `now()`：只有两侧同一时钟源，
   测试里注入时钟才能复现过期。过期行不靠即时 DELETE 回收，清扫是 `internal/daemon` 的 5 分钟守护
   （ADR-0061 §1 的登记表），滞后也读不到过期值。
   抢占语义用 `INSERT ... ON CONFLICT DO NOTHING` 的 `RowsAffected`（SETNX 等价），
   并在插入前先删同 key 的过期行 —— 否则「过期残留」会被当成「已被占用」。
2. **积分瞬态锁改 PG 会话级 advisory lock**，绑定固定连接（`sql.Conn`）：连接断开即释放，
   比 TTL 更不容易留粘锁；非 PG 方言（SQLite 契约测试库）自动退化为不加锁。
   锁的自述口径不变：**它只是进程级双写护栏，最终裁决仍由唯一索引 / 占坑表承担**（ADR-0023）。
3. **缓存层整体退役**：字典 / 评估 / 估价 / AI 绑定的缓存连同 key 契约、失效 pattern、
   静态契约测试一起删除，读路径直查 DB。
4. **不引入任何 PG 扩展**。唯一需要的「定时清理」由既有 daemon 承担，不需要 pg_cron；
   表规模不需要 pg_partman；缓存删掉后没有物化视图需要 pg_ivm。
5. **健康检查从探 Redis 改探数据库**：`/api/health` 由路由级中间件注入的 `*gorm.DB` 做
   `PingContext`；`/api/health/live` 仍不依赖任何外部组件。
6. **testing 冒烟放行数据服务重建**：冒烟栈每次重建、数据可抛弃，而数据服务闸门保护的是生产数据。
   CD 只给 `environment == testing` 注入 `FORCE_DATA_SERVICES=true`（真值面 `1/true/yes`），
   判定仍走 `decide_service`，只在确有配置漂移时 recreate。

## 被否选项

| 选项 | 否因 |
|---|---|
| 缓存迁成 PG 表 | 读还是读 PG，等于「取消缓存但保留复杂度」；访客≈0 时收益为零 |
| 换进程内缓存 | 需要 LRU 容量控制与跨实例失效；单实例下能跑，但把「将来扩容」变成隐形地雷 |
| 装 pg_cron / pg_partman / pg_ivm | 分别与既有 daemon、表规模、已删缓存三者都不匹配；还要自建镜像 + preload |
| 保留 Redis 作为「可选降级面」 | 那正是本次要消掉的双层依赖；降级语义（fail-open / fail-closed 分级）也是维护面 |
| 把 advisory lock 改成事务级 `xact` 锁 | 各入口的临界区跨越多个事务（先查后写），事务级锁套不进去 |
| 直接删掉积分锁 | 会改变并发错误码（占锁失败 → 唯一冲突），超出「去 Redis」的范围 |

## 后果与取舍

- **少一个容器、一套配置**：`REDIS_*` 变量、compose 的 redis 服务、CD/CI 的 redis 侧注入、
  部署脚本的 `wait_redis`、巡检脚本的容器清单全部消失。
- **PG 成为唯一依赖**：原来「缓存挂了业务还能跑」的那层隔离没有了。零流量下可接受；
  真到有量时再按 ADR-0013 的读面形状加回缓存（那时该重新评进程内 vs 外部）。
- **advisory lock 占连接**：临界区期间持有一条池连接。当前各临界区都很短，池 20 条无压力；
  若将来 AI 扣费路径变慢，需要重新评估连接预算。
- **测试库语义**：SQLite 契约测试库没有 `pg_try_advisory_lock`，锁在那里恒为空实现 ——
  这正是「锁只是护栏」的自证：契约测试靠唯一索引仍然全绿。

## 迁移顺序（已执行）

1. 阶段 A：PG 15 → 18、调参、启用 `pg_stat_statements`、换 RBD 镜像（`deploy/README.md` 的 runbook）。
2. 阶段 B：`000048_ephemeral_kv` 迁移 → `internal/kv` → 三类消费方换存储 → 积分锁改 advisory →
   删缓存层 → 配置/部署面/CI/文档收敛 → 本 ADR。

回滚：临时 KV 的数据是短命状态，回滚只需 `000048` 的 down（整表删除），代价是用户重发验证码 / 重新登录。
