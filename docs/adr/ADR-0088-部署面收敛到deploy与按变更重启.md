# ADR-0088：部署面收敛到 deploy/ 与按变更重启

- 状态：Accepted（2026-10-27）
- 日期：2026-10-27
- 领域：部署 / 交付链路（承接 ADR-0047 §5 部署事实源、ADR-0056 §6 迁移失败门）
- 判据物：`deploy/tests/`（增量计划 / 回滚 / 迁移门），`deploy/deploy.py --plan`

## 背景

部署脚本此前分在三处：`scripts/deploy-remote.sh`（1,400+ 行 bash，CD 在服务器上执行）、
根 `deploy.sh`（本地/手动一键）、根 `docker-compose.prod.yml`；初始化、备份、巡检脚本散在
`scripts/`，部署判据测试散在 `scripts/*.test.mjs`。两个直接后果：

1. **每次部署都对整栈执行 `docker compose up`**。compose 本身会跳过未变化容器，但脚本没有
   「变更计划」这个显式事实：运维看不到会动哪些服务；数据服务（postgres / redis）与业务服务
   走同一条 `up` 路径；迁移还排在应用重启之后 —— 新后端先起、旧 schema 先撞。
2. **部署面的阅读与修改要跨三个目录**，而 compose 里到处是相对路径（`./frontend/Caddyfile`、
   `./backend`），一移动就会踩「项目名变 → 命名卷变 → 空库启动」这类数据事故。

## 决策

1. **部署 CLI 换成 Python 3（仅标准库）**：`deploy/deploy.py`。目标服务器是 Debian / PVE，
   自带 `python3`；不再要求服务器装 Node，也不把 1,400 行 bash 继续加长。
2. **部署相关文件全部聚合到 `deploy/`**：compose、CLI、默认值生成物、初始化 / 备份 / 巡检脚本、
   systemd 单元、部署判据测试。仓库根的 `deploy.sh` 与 `scripts/deploy-remote.sh` 删除，
   路径由 README / checks.md / CD 统一改为 `deploy/...`。
3. **显式变更计划**：按「容器状态 + 镜像内容（image ID）+ compose config-hash」逐服务判定
   `skip / start / create / recreate / blocked`，只对非 `skip` 的服务执行
   `up -d --no-deps`。`--plan`（`--json`）只读输出计划。
4. **数据服务与业务服务分治**：`postgres` / `redis` 默认永不因应用镜像变更而重建；检测到配置
   漂移（例如 `PG_VOLUME` 被改）时判 `blocked` 并中止，必须显式 `--force-data-services`。
   `--force` 只重建应用服务。
5. **迁移先于应用重启**：先跑迁移兼容性预检 → 清 dirty → 用新后端镜像跑 `migrate up`；
   成功后才重建 backend / frontend。迁移失败仍按 ADR-0056 §6 默认中止。
6. **回滚只动变化的服务**：回滚以 `backups/last-version.txt` 里的历史镜像为准，只对 tag 变化的
   backend / frontend 执行重建，并做运行镜像自证；不再无差别 `down+up` 两个服务。
7. **compose 项目名钉死**：`name: ${COMPOSE_PROJECT_NAME:-forklift-training}`，保住
   `forklift-training_*` 命名卷；`.env` 仍生成在 `$DEPLOY_PATH/.env`，由 `--env-file` 显式传入。
8. **默认值只进内存**：`deploy/env.defaults` 由 `deploy.py` 读成内存字典作为最后一层兜底，
   绝不 `source`、绝不写回 `os.environ` —— 2026-09-13 的 `PG_VOLUME` 事故锁继续成立，
   且现在比文本锁更强：CD 提供的值在进程环境里原样保留。

## 备选

- **保留 bash，只加 `--changed-only`** —— 拒绝。变更计划的判据（image ID / config-hash）在 bash 里
  只能靠字符串拼装，测试面与错误面都差；而且「部署面聚合」本身就要动路径，一次做完比两次迁移便宜。
- **改用 Node.js** —— 拒绝。服务器端没有 Node 的保证（当前脚本只把 Node 用在前端构建），
  而 `python3` 是 Debian/PVE 的基础组件；用 Python 不会给运行时增加新依赖。
- **继续依赖 compose 的隐式跳过** —— 拒绝。它无法表达「数据服务不许动」「迁移必须先于重启」
  这两个安全约束，也无法在部署前给人看计划；隐式行为在事故复盘里不可见。
- **把 compose 留在仓库根，只移动脚本** —— 拒绝。compose 是部署面的核心文件，留在根上等于
  「聚合」只做了一半；相对路径与项目名的坑已经在决策 7 里被一次性收掉。

## 后果

- CD 的打包清单从 `scripts/deploy-remote.sh` 改为 `deploy/deploy.py` + `deploy/docker-compose.prod.yml`；
  testing 收尾改为 `python3 deploy/deploy.py down`。
- 服务器上的部署命令统一为 `python3 deploy/deploy.py`；旧的 `bash scripts/deploy-remote.sh` 不再维护。
- `deploy/tests/` 成为部署面判据物的固定位置；CI 增加 `python3 -m unittest discover -s deploy/tests`。
- 命名卷保持不变（项目名钉死）；第一次升级到本 ADR 的部署只会重建真正变化的容器。
- 迁移与回滚的「最小动作」语义写进了脚本与测试；未来若要支持「只迁移不重启」，在 `--plan`
  的 decision 层扩展，而不是在 compose 层加开关。
