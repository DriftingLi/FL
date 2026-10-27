# deploy/ — 部署面

> ADR-0088：部署相关文件统一收在这里。部署 CLI 是 **Python 3.9+ 标准库单文件**（`deploy.py`），
> 目标服务器（Debian / PVE）自带 `python3`，不要求 Node。

## 目录

| 文件 | 作用 |
| --- | --- |
| `deploy.py` | 部署 CLI：按变更重启、迁移、回滚、健康检查、测试环境 down |
| `docker-compose.prod.yml` | 生产编排（postgres / redis / libreoffice / backend / frontend） |
| `env.defaults` | 部署默认值生成物（唯一事实源 `backend/internal/deploy/topology.go`；`cd backend && go run ./cmd/gen-deploy`） |
| `setup-server.sh` | 服务器首次初始化（Docker / 部署用户 / 目录 / 防火墙 / cron） |
| `lxc-install-docker.sh` / `lxc-setup-ssh.sh` | LXC 容器内 Docker / SSH 初始化 |
| `backup-daily.sh` / `rbd-snap-hourly.sh` | 每日逻辑备份 / 每小时 RBD 快照 |
| `forklift-watchdog.sh` + `systemd/` | 生产链路存活巡检与告警投递 |
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
- `--force-data-services`：允许重建 postgres / redis（危险，仅在确认卷/配置变更时有意义）。
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

`postgres` / `redis` 永远不因应用镜像变更进入计划；只有容器不存在/停止时才启动。

## 部署顺序

1. 预检查（docker / compose / curl / 磁盘 ≥ 2GB）；
2. 解析镜像加速代理 → 写 `.env`（内容未变则跳过写入）；
3. 记录当前运行镜像到 `backups/last-version.txt`；生产环境并行做 `pg_dump` 备份；
4. 只拉取本地缺失的镜像（内容哈希标签命中即跳过）；
5. 确保 postgres / redis 在运行，等待数据库就绪；
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

命名卷仍是 `forklift-training_pgdata-prod` / `forklift-training_redisdata-prod` 等。
换宿主机想换项目名时，显式设置 `COMPOSE_PROJECT_NAME`（并在迁移数据卷前确认）。

## 迁移失败门（ADR-0056 §6 / #1099）

- 默认：迁移失败 → `DeployError` → 进程退出 1，不重启应用。
- `ALLOW_MIGRATION_FAILURE=1`：显式带「新代码跑在旧 schema」的已知风险继续。
- `SKIP_MIGRATION=true`：整段迁移不跑（不存在失败与否）。
- dry-run 入口：`python3 deploy/deploy.py --migration-gate`（唯一判据点 `migration_failure_action`）。

## 判据物与本地校验

```bash
python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v
node --test deploy/tests/deploy-migration-gate.test.mjs
node --test deploy/tests/deploy-rollback-plan.test.mjs
docker compose -f deploy/docker-compose.prod.yml config -q
```

CI 在 `.github/workflows/ci.yml` 的 `el-controls-selftest` job 里跑这些判据（Node 用例需要 `python3`），
并额外跑一次 `docker compose -f deploy/docker-compose.prod.yml config -q` 的语法校验。
