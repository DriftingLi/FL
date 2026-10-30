# deploy/ — 部署面

> ADR-0088：部署相关文件统一收在这里。部署 CLI 是 **Python 3.9+ 标准库单文件**（`deploy.py`），
> 目标服务器（Debian / PVE）自带 `python3`，不要求 Node。

## 目录

| 文件 | 作用 |
| --- | --- |
| `deploy.py` | 部署 CLI：按变更重启、迁移、回滚、健康检查、测试环境 down |
| `docker-compose.prod.yml` | 生产编排（postgres / libreoffice / backend / frontend） |
| `env.defaults` | 部署默认值生成物（唯一事实源 `backend/internal/deploy/topology.go`；`cd backend && go run ./cmd/gen-deploy`） |
| `setup-server.sh` | 服务器首次初始化（Docker / 部署用户 / 目录 / 防火墙 / cron） |
| `lxc-install-docker.sh` / `lxc-setup-ssh.sh` | LXC 容器内 Docker / SSH 初始化 |
| `backup-daily.sh` / `rbd-snap-hourly.sh` | 每日逻辑备份 / 每小时 RBD 快照 |
| `forklift-watchdog.sh` + `systemd/` | 生产链路存活巡检与告警投递（CD 随包上传，**安装仍要人工**，见下文「巡检脚本与 systemd 单元」） |
| `tests/` | 增量计划 / 回滚 / 迁移门的常驻判据物（Python `unittest` + Node `--test`） |

## 常用命令

在部署根目录（CD 是 `/opt/forklift-training`）执行：

```bash
python3 deploy/deploy.py                 # 部署（CD 默认调用）
python3 deploy/deploy.py --plan          # 只读计划：会动哪些容器、是否要迁移
python3 deploy/deploy.py --plan --json   # 机器可读计划（stdout 只有 JSON）
python3 deploy/deploy.py rollback        # 回滚到 backups/last-version.txt
python3 deploy/deploy.py down            # 停栈，命名卷保留（testing 冒烟收尾）
python3 deploy/deploy.py local           # 本地 / 手动一键：构建前端 → 迁移 → 部署
```

常用开关：

- `--force`：重建全部**应用**服务（backend / frontend / libreoffice），不碰数据服务。
- `--force-data-services`：允许重建 postgres（危险，仅在确认卷/配置变更时有意义）。
- `--skip-migrate`：跳过迁移（本地模式直接用；远程仍可用 `SKIP_MIGRATION=true`）。
- `--remove-orphans`：`up` 时移除孤儿容器（默认不做，避免误删 profile 外服务）。

## 按变更重启怎么判定

每个服务先取三样事实：

1. 容器状态（不存在 / 停止 / 运行中）；
2. 运行容器的镜像（`Config.Image` + image ID）；
3. 运行容器的 `com.docker.compose.config-hash`，与 `docker compose config --hash=*` 现算值比较。

判定顺序（纯函数 `decide_service`，测试在 `tests/test_deploy.py`）：

| 情况 | 动作 |
| --- | --- |
| 容器不存在 | `create` |
| 容器停止 | `start` |
| 镜像 ID 不同（含同 tag 新镜像） | `recreate` |
| tag 不同但 image ID 相同（镜像源/代理前缀差异） | `skip` |
| compose config-hash 不同 | `recreate` |
| 以上都不是 | `skip` |
| **数据服务**配置漂移 | `blocked`（默认中止，需 `--force-data-services`） |

`postgres` 永远不因应用镜像变更进入计划；只有容器不存在/停止时才启动。

## 部署顺序

1. 预检查（docker / compose / curl / 磁盘 ≥ 2GB）；
2. 解析镜像加速代理 → 写 `.env`（内容未变则跳过写入）；
3. 记录当前运行镜像到 `backups/last-version.txt`；生产环境并行做 `pg_dump` 备份；
4. 只拉取本地缺失的镜像（内容哈希标签命中即跳过）；
5. 确保 postgres 在运行，等待数据库就绪；
6. 迁移兼容性预检（镜像内迁移版本不得落后于数据库）；
7. 等待备份完成 → 清 dirty → **先迁移**；
8. 算应用计划，只对变化的服务 `up -d`（backend 先于 frontend，libreoffice 最先）。
   **不带 `--no-deps`**：compose 的 config-hash 包含 `depends_on`，带它创建出的容器标签哈希
   与 `compose config --hash` 永远不同，backend/frontend 会被判「配置变更」每次重建。
   数据服务仍由 `plan_data` 的 blocked 保护，`up <app>` 不会重建未变化的依赖；
9. 后端 / 前端真实 HTTP 健康检查；
10. 清理悬空镜像、旧镜像、旧备份。

失败时：只有**已经动过应用容器**才自动回滚（回滚同样只重建 tag 变化的服务，并做运行镜像自证）；
镜像拉取 / 迁移预检阶段失败时线上版本没变，不回滚。

## .env 与默认值（2026-09-13 事故的反面教材）

- `.env` 由 `deploy.py` 生成在 **`$DEPLOY_PATH/.env`**，通过 `--env-file` 显式传给 compose。
- `deploy/env.defaults` 只被读进**内存默认值**（`getenv` 的最后一层兜底），**不会**写进 `os.environ`，
  更不会 `source`。CD 传入的值（例如 `PG_VOLUME=/srv/ceph/pgdata`）永远优先。
- 镜像引用（`BACKEND_IMAGE` / `FRONTEND_IMAGE` / `LIBREOFFICE_IMAGE`）只由 `.env` 控制，
  避免 `env.defaults` 的本地镜像名盖掉部署脚本算好的 registry + tag。

## compose 项目名与数据卷

compose 文件从仓库根迁到 `deploy/` 后，默认项目名会从目录名变化，命名卷会跟着改名（＝空库）。
因此 `deploy/docker-compose.prod.yml` 顶部钉了：

```yaml
name: ${COMPOSE_PROJECT_NAME:-forklift-training}
```

命名卷仍是 `forklift-training_pgdata-prod` 等（`redisdata-prod` 随 #2109 退役）。生产实际用的是 environment secret `PG_VOLUME=/srv/ceph/pgdata18`（RBD 挂载），见下一节升级 runbook。
换宿主机想换项目名时，显式设置 `COMPOSE_PROJECT_NAME`（并在迁移数据卷前确认）。

## 大版本升级：PostgreSQL 15 → 18（一次性 runbook，#2109）

镜像版本不能靠一次 `docker compose up` 换过去：PG 18 的二进制打不开 PG 15 的数据目录。
全仓零 `CREATE EXTENSION`（没有扩展要在新集群里重装），所以走**逻辑备份搬运**最省事。
本流程必须人工按序执行，不要塞进 `deploy.py` —— 它一次性、不可重复。

**两条 2026-10-29 生产实测出来的事实，动手前先读：**

1. **生产的 `PG_VOLUME`（`/srv/ceph/pgdata`）不是普通目录，是 Ceph RBD 镜像的挂载点**
   （`/dev/rbd0` → xfs；`/etc/fstab` 按 UUID 挂载、`/etc/ceph/rbdmap` 负责 map，镜像上还有每小时快照）。
   挂载点**不能 `mv`**（实测 `Device or resource busy`）—— 升级必须**另起一个镜像**，
   旧镜像原样留着就是回滚件。
2. **18+ 官方镜像的挂载点是 `/var/lib/postgresql`，不是 `/var/lib/postgresql/data`。**
   18 起 PGDATA 变成 `/var/lib/postgresql/<主版本>/docker`；卷挂在旧路径上会被入口脚本判成
   「升级残留数据」并拒绝启动（实测 `there appears to be PostgreSQL data in: /var/lib/postgresql/data`）。

前置：`docker compose -f deploy/docker-compose.prod.yml config` 已是 `postgres:18-alpine`；宿主机能给
postgres 容器约 2G（不够时把 `shared_buffers` 降到 384MB、`effective_cache_size` 降到 1GB，改在 compose 的
`command:` 里）。

下面统一用这个变量。**必须带 `--env-file`**：`COMPOSE_PROJECT_NAME` 在 `.env` 里，漏了它会算出
另一个项目名，容器名与卷名全部对不上。

```bash
cd "$DEPLOY_PATH"
COMPOSE="docker compose --env-file .env -f deploy/docker-compose.prod.yml"
```

0. 记下当前迁移版本（后面要逐字对上）：

   ```bash
   $COMPOSE run --rm backend /app/bin/migrate status
   ```

1. 逻辑备份（用**旧**容器里的 15 版 `pg_dump`；落到 root 盘，别落进马上要被挪走的数据目录）：

   ```bash
   docker exec forklift-pg-prod pg_dump -U forklift -Fc -d forklift_training -f /tmp/fl15.dump
   docker cp forklift-pg-prod:/tmp/fl15.dump "$DEPLOY_PATH/backups/forklift15-$(date +%Y%m%d).dump"
   ```

2. 停应用（搬迁期间不接受写入）：

   ```bash
   $COMPOSE stop backend frontend
   ```

3. 准备新数据目录并切 `PG_VOLUME`（**纯增量，不碰旧镜像**）：

   ```bash
   rbd create forklift-data/pgdata18 --size 20G
   echo "forklift-data/pgdata18 id=admin,keyring=/etc/ceph/ceph.client.admin.keyring" >> /etc/ceph/rbdmap
   DEV=$(rbd device map forklift-data/pgdata18)
   mkfs.xfs -L pgdata18 "$DEV"
   mkdir -p /srv/ceph/pgdata18 && mount "$DEV" /srv/ceph/pgdata18
   chown 70:70 /srv/ceph/pgdata18 && chmod 700 /srv/ceph/pgdata18   # postgres 镜像内 uid=70
   UUID=$(blkid -s UUID -o value "$DEV")
   echo "UUID=$UUID /srv/ceph/pgdata18 xfs rw,defaults,noatime,x-systemd.requires=rbdmap.service 0 0" >> /etc/fstab

   gh secret set PG_VOLUME --env production --body "/srv/ceph/pgdata18"
   sed -i 's|^PG_VOLUME=.*|PG_VOLUME=/srv/ceph/pgdata18|' .env   # CD 下次会写出同一个值
   ```

   命名卷的部署等价做法：换一个新卷名（旧卷保留）。判据相同 —— 18 必须在**空目录**上 initdb。

4. 停应用与旧库，再起新库（空目录由 18 initdb）：

   ```bash
   $COMPOSE stop backend frontend
   $COMPOSE stop postgres
   $COMPOSE up -d postgres
   $COMPOSE ps postgres      # 等 healthy
   ```

   ⚠️ 起来后的头几十秒，`pg_isready` 可能已经返回，但 `forklift_training` 还建在 initdb 的
   临时实例里 —— 这时连库会报 `database "forklift_training" does not exist`，**等 30 秒再判**。

5. 还原：

   ```bash
   docker cp "$DEPLOY_PATH/backups/forklift15-<日期>.dump" forklift-pg-prod:/tmp/fl15.dump
   docker exec forklift-pg-prod pg_restore -U forklift -d forklift_training \
     --no-owner --no-privileges /tmp/fl15.dump
   ```

6. 还原后自证：`migrate status` 的版本号必须与第 0 步**逐字相同**且 `CLEAN`；再抽查
   `hrwai_users` / `evaluations` / `points_ledger` 的行数与旧库一致。

7. 起服务：

   ```bash
   $COMPOSE up -d backend frontend
   ```

   **不要在这里手跑 `deploy.py`**：它会按当前 shell 的环境重写 `.env`，而 `SECRET_KEY` /
   `JWT_SECRET_KEY` / `DB_PASSWORD` 的权威来源是 CD —— 缺这些环境变量时会被写成空值，等于把生产锁死。
   `.env` 由 CD 生成（合并后的那次部署已经重写过一遍），本次不动它。

8. 冒烟：`curl -fsS https://<DOMAIN>/api/health` 返回 200；再走一遍登录与估值页。

**回滚**：旧镜像与旧挂载点一直没动过，切回去只要把 `PG_VOLUME` 指回去：

```bash
$COMPOSE stop postgres backend frontend
gh secret set PG_VOLUME --env production --body "/srv/ceph/pgdata"
sed -i 's|^PG_VOLUME=.*|PG_VOLUME=/srv/ceph/pgdata|' .env
# 镜像与挂载目标改回 15 那一版（git revert，或临时改 compose 的 image 与 volumes）之后：
$COMPOSE up -d postgres
```

回滚**会丢掉**切到 18 之后写入的数据，因此只在零流量窗口执行。

**升级后必做**：`deploy/rbd-snap-hourly.sh` 的快照目标是 `forklift-data/pgdata`，升级后在线数据
在 `pgdata18` 上 —— 不同步改掉，每小时快照就还在拍旧镜像，在线数据失去小时级覆盖。

**观测**：升级后 `SELECT calls, mean_exec_time, query FROM pg_stat_statements ORDER BY calls DESC LIMIT 20;`
就是「哪些查询最重」的答案 —— 没有它，后续任何「该不该加回缓存」的判断都只能靠猜。

## 迁移失败门（ADR-0056 §6 / #1099）

- 默认：迁移失败 → `DeployError` → 进程退出 1，不重启应用。
- `ALLOW_MIGRATION_FAILURE=1`：显式带「新代码跑在旧 schema」的已知风险继续。
- `SKIP_MIGRATION=true`：整段迁移不跑（不存在失败与否）。
- dry-run 入口：`python3 deploy/deploy.py --migration-gate`（唯一判据点 `migration_failure_action`）。

## 巡检脚本与 systemd 单元（forklift-watchdog）

`forklift-watchdog.sh` 与 `systemd/` 下的三个单元随 CD **上传**到 `$DEPLOY_PATH/deploy/`，
但**不会**自动装进系统 —— 写入 `/usr/local/bin` 与 `/etc/systemd/system/` 是系统级动作，
仍由人工执行（脚本头注释里有完整命令）。

改了脚本或单元之后**必须重装一次**，否则机器上继续跑旧判据。2026-10-29 的实例：
`forklift-redis-prod` 已从 compose 删除，而 `/usr/local/bin` 里的旧副本仍在盯它，
连续 3 次判红并真的发出了告警邮件。

```bash
cd "$DEPLOY_PATH"
install -m 755 deploy/forklift-watchdog.sh /usr/local/bin/forklift-watchdog.sh
install -m 644 deploy/systemd/forklift-watchdog.service \
               deploy/systemd/forklift-watchdog.timer \
               deploy/systemd/forklift-alert@.service /etc/systemd/system/
systemctl daemon-reload && systemctl restart forklift-watchdog.timer
/usr/local/bin/forklift-watchdog.sh --dry-run   # 自检：应打「达告警级别 0/6」
# 注意用**安装后的路径**自检：仓库里的脚本是 100644（靠 install -m 755 赋执行位），
# 直接跑 deploy/forklift-watchdog.sh 会 Permission denied。
```

脚本里的 `EXPECT_CONTAINERS` 是容器判据：**部署面增减容器时必须同步它**（否则误报）。
删掉某个服务（例如 #2109 的 redis）时，连它一起从这条清单里删。

## 判据物与本地校验

```bash
python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v
node --test deploy/tests/deploy-migration-gate.test.mjs
node --test deploy/tests/deploy-rollback-plan.test.mjs
docker compose -f deploy/docker-compose.prod.yml config -q
```

CI 在 `.github/workflows/ci.yml` 的 `el-controls-selftest` job 里跑这些判据（Node 用例需要 `python3`），
并额外跑一次 `docker compose -f deploy/docker-compose.prod.yml config -q` 的语法校验。

## 域名、资源域与回源（ADR-0077）

**双域族**：和润（门户 + 残值评估）在 `hrwai.cn`，易道（培训 / 讲师 / 管理 / 招聘）在 `gdyidao.cn`。本仓反代只服务后者；`hrwai.cn` 与本机没有直接关系（除 EdgeOne 回源到 `api.<domain>`）。旧域 `gccsmile.com` 按硬切口径整体退役，不留 301。

| host | 职责 |
| --- | --- |
| `api.<domain>` | **唯一 API 入口**：`/api`、`/swagger`、`/health`。移动端、门户回源、残值 BFF 都打这里 —— 页面域名与它解耦，下次换页面域名这三条链路不用动 |
| `training` / `mentor` / `manage` / `recruit`.`<domain>` | 四个工作区 SPA（同源 `/api` 反代） |
| `cdn.<domain>` | 对象存储直出（公开 URL 基址 `R2_PUBLIC_DOMAIN`）+ `/reports/`；仅 GET/HEAD，未知前缀 404（不吐 SPA） |
| `dify.<domain>` | 内网 Dify 与知识库图片反代（SSE 不缓冲） |
| `valuation.<domain>` | 一跳 301 到 `https://valuation.hrwai.cn/valuation`（北迁已完成，本仓不再有该工作区的 SPA） |
| 裸域 / `www.<domain>` | 一跳 301 到培训入口 |

**EdgeOne 回源口径**：新站的 `https://www.hrwai.cn/api/**` 由 EdgeOne 回源到 `https://api.<domain>`，回源鉴权见下节。

**回填旧 URL**（一次性，票 #1742；换域时按同一口径用 `-from` / `-to` 指新前缀再跑一次）：命令随后端镜像构建为 `/app/bin/backfill-asset-domain`。**切换日必须先 dry-run 记账、执行后第二次命中 0 行**，否则切完历史图片全碎。

```bash
docker compose --env-file .env -f deploy/docker-compose.prod.yml run --rm backend /app/bin/backfill-asset-domain            # dry-run：只报每表命中行数
docker compose --env-file .env -f deploy/docker-compose.prod.yml run --rm backend /app/bin/backfill-asset-domain -write     # 落库（整轮一个事务）
```

## 客户端 IP：EdgeOne 回源鉴权（票 #1746）

经 EdgeOne 回源时，反代看到的对端是边缘节点，但 EdgeOne 会带 `EO-Connecting-IP`（实测为真实客户端；同一次请求里的 `X-Forwarded-For` / `X-Real-IP` 都只是边缘链路）。门户（EdgeOne Pages）的 `/api` 代理回源时带 `X-Origin-Token`，**只有 token 命中才采信** `EO-Connecting-IP`（`frontend/Caddyfile` 的 `(backend_api)` 片段：命中走单独的 `handle`，`header_up` 只给这一个值、不追加对端），否则沿用「追加对端」的语义 —— 直连源站伪造该头无效。

- **两处同值**：本仓部署 secret `EDGE_ORIGIN_TOKEN`（production / testing 两个 environment 都要）+ 门户 Pages 环境变量 `NUXT_API_ORIGIN_TOKEN`。
- **fail closed**：模板启用鉴权而 token 为空时，前端容器入口直接拒绝启动 —— 空键会匹配空头，等于信任所有人的 `EO-Connecting-IP`。
- **轮换**：两处同时改，重新部署前后端即可，不需要改代码。
- **不采用**「把 EdgeOne 公共段塞进 `TRUSTED_PROXIES`」：公共段进白名单等于让任何直连者伪造来源。

## 生产诊断：某个端点到底有没有真实流量

回答「这接口有人调吗 / 多少次 / 谁在调」时，**只有一个面可用**：后端容器 named volume 里的 `app.log`。

```bash
# 在 pve-02（生产应用宿主）上，按路径计数
F=/var/lib/docker/volumes/forklift-training_logs-data/_data/app.log
grep -a "\"path\": \"/api/valuation/auth/logout\"" "$F" | wc -l
```

三条口径，少一条就会得出错结论：

1. **必须带 `-a`**。该文件含二进制字节，不加 `-a` 时 `grep` 只回一句 `binary file matches` 而**不给计数** —— 看着像 0，其实什么都没数。
2. **别用反代容器的访问日志**。`forklift-frontend-prod` 是 `network_mode: host`，反代的访问行直接进 docker `json-file` 驱动（不出落在文件里的 access.log）；而 **CD 每次部署都重建容器 ⇒ 历史清零**。实测（2026-09-30）该窗口只剩 10.2 小时 / 8875 行，且窗口内 `/api/**` 的 POST 一条都没有（只有扫描器探针）——在那里面「命中 0」与「没有流量」**不可区分**。
3. **计数不带方向，要回到行里看**。日志行由 `logger/access.go` 逐请求写一条 JSON：`{method, path, status, duration_ms, ip, user_id, user_role, request_id}`。判「是不是集成客户端在调」看 `user_id` / `user_role` 与同一 `ip` 前后的请求时序，而不是只看总数（2026-09-30 那次就是靠这个把「33 天 1 次」定性成人为试探而非使用）。

留存边界：`app.log` 由 lumberjack 按 `LOG_DIR=/data/logs`、`LOG_MAX_SIZE_MB`（生产 100）、`LOG_MAX_BACKUPS`（7）、`LOG_MAX_AGE_DAYS`（30）、`LOG_COMPRESS` 轮转（实现 `backend/internal/logger/logger.go` 的 `newRotatingFile`），跨容器重建留存；实测窗口 33 天 / 13.5 万行。⚠️ 归档只在**越过大小阈值时**才产生，所以「卷里只有一个 app.log」不等于「没配轮转」。
