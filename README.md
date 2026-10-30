# 叉车维修培训与残值评估系统

面向叉车维修培训、学员就业对接与叉车残值评估的全栈系统：在线培训与考试练习、论坛问答、资料投稿、积分激励、AI 助手、企业招聘对接，以及整机残值与电池剩余寿命（RUL）评估。系统按角色划分工作区（学员 / 讲师 / 管理员 / 企业招聘者），前端以子域名承载。

领域术语以 [`CONTEXT.md`](./CONTEXT.md) 为准，架构决策记录在 [`docs/adr/`](./docs/adr/)（当前 91 篇）。

## 功能特性

### 培训学习（学员 · `training.`）

- **目标证件**：学员报考的外部持证目标，作为全局过滤器贯穿课程、题库、练习、模考、错题、搜索、收藏与投稿
- **课程目录**：`目标证件 → 专业方向 → 课程等级 → 课程` 三层虚拟树；章节支持 PPT / 视频 / 图文混排（PPT 经 LibreOffice sidecar 转 WebP）
- **练习与考试**：顺序 / 随机 / 专项 / 标签四种练习模式；模拟考试自动判分 + AI 评分（简答）；按套解锁的真题卷
- **学习与错题**：断点续学（章节 + 视频位置）；错题本按题收录、单题即时重做，与练习共用提交管线
- **论坛**：讨论 / 问答 / 备考经验三种模式，支持采纳、点赞、举报、楼中楼
- **资料投稿**：学员上传资料换取积分，先审后发，违规下架走积分对冲追回（ADR-0026）
- **积分域**：统一账务（余额 + 流水 + 幂等占坑），任务中心、问答采纳、AI 计费、兑换与扣罚共用一套簿记（ADR-0023）
- **在线简历**：维护简历、导出 PDF、向企业投递并查看进度
- **AI 助手**：大模型流式对话（DeepSeek 默认，OpenAI 兼容可换），按 token 计量扣积分
- **搜索与收藏**：course / question / content / topic 四类聚合搜索；course / chapter / question / featured / topic 五类对象可收藏

### 招聘对接（企业招聘者 · `recruit.` / 学员端职位广场）

- 企业发布职位（挂专业方向），学员按地区 / 方向筛选浏览
- 投递即授权简历可见（ADR-0025）；企业检索简历、发起联系请求，双向交换联系方式
- 招聘者管理：授信码开通、资料编辑、微信绑定、统计

### 残值评估（学员 `hrwai_user` · 公开界面在门户 `valuation.hrwai.cn`）

> 公开界面（填报 / 历史 / 报告）已北迁到门户仓的 Nuxt 应用（`valuation.hrwai.cn/valuation`）；本仓保留后端与数据、管理端的「残值配置」。旧子域现在只是一条 301。

- **整机残值**：按品牌、车型、吨位、出厂年份、工时、车况、区域等参数输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`；五维系数为出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **电池 RUL**：基于特征提取与混合神经网络的锂电池剩余寿命预测（含 SOH、置信区间与建议）
- **PDF 报告**：评估结果一键生成可下载的中文 PDF

### 管理后台（`manage.`）与讲师工作区（`mentor.`）

- 管理后台：账号与基础数据维护、课程 / 题库 / 标签、投稿与论坛审核、精选内容、职位管理、统计看板、AI 设置、审计日志、系统巡检、残值算法配置
- 讲师工作区：章节编排、题库创建 / 维护 / 驳回、题库标签、投稿审核

### 移动端（学员 App + 招聘者端）

- **uni-app x** 跨端应用（Android / iOS / H5 / 微信小程序），页面清单以 `pages.json` 为准
- 由 @zhengcookie 负责，功能状态与验收门见该目录的 [`AGENTS.md`](training-app/叉车维修培训学员端跨端应用/AGENTS.md) 与移动端自己的 ADR 体系（编号与根仓库无关）

## 系统架构

单仓库包含三个可独立部署的部分：**后端（Go）**、**前端（Vue 3）**、**移动端（uni-app x）**。前后端通过 HTTP API 通信，接口清单见 [`API.md`](./API.md)，Swagger 由后端 `/swagger` 提供。

### 子域名工作区（前端）

前端为单一应用，按访问子域名切换工作区；API 统一走 `api.<domain>`，页面域名与 API 解耦（ADR-0077）。

| 子域名 | 工作区 | 角色 | 主要内容 |
| --- | --- | --- | --- |
| `www` | 官网门户 | 访客 | **独立 Nuxt 仓库**，部署在腾讯云 EdgeOne；旧域 `gccsmile.com` 已硬切退役，不留 301（ADR-0077） |
| `training.` | 培训学习 | 学员 `hrwai_user` | 课程、练习、模考、真题、论坛、投稿、积分、AI 助手、简历、职位广场 |
| `mentor.` | 讲师工作区 | 讲师 `tutor` | 章节、题库、标签、投稿审核 |
| `manage.` | 管理后台 | 管理员 `admin` | 全站管理与配置 |
| `valuation.` | 残值评估 | 学员 `hrwai_user` | 已北迁，仅剩一跳 301 到门户 |
| `recruit.` | 招聘工作区 | 企业招聘者 `recruiter` | 职位发布、简历库、投递管理 |

生产登录态用 httpOnly Cookie，但两族域作用域不同（ADR-0022「招聘角色隔离」；两族并存时的族判定见 ADR-0067）：学员侧 `hrwai_token` 挂父域名以便各子域名共享，招聘者侧 `recruiter_token` 刻意收紧为 host-only，否则已登录学员的静默恢复路径会拿到 recruiter token、串角色。Cookie 不可用时降级为带一次性 `auth_token` 参数的跨子域名跳转。

### 统一账号与双令牌（后端）

- **统一账号**：`hrwai_users` + 一套 JWT，学员端 / 残值评估 / AI 助手共用；角色 `hrwai_user`、`tutor`、`admin`、`recruiter`（后三者为独立账号表）
- **双令牌（ADR-0016）**：access 2 小时（中间件仅收 access）+ refresh 7 天轮换；refresh 黑名单落在 PostgreSQL `ephemeral_kv` 表（#2109），access 自然过期
- **登录方式**：密码、邮箱验证码、手机验证码、微信扫码、微信小程序 `wx-login`
- **验证码（ADR-0001）**：邮箱（SMTP）与短信（腾讯云 SMS）是同一状态机两侧的 adapter，默认 5 分钟内有效、60 秒节流、错误上限 5 次
- 统一响应结构 `{ "code": <HTTP状态码>, "message": "...", "data": ... }`（ADR-0005）

### 后端按功能域分包（ADR-0070）

- **一个功能域 = 一个包**：`backend/internal/<域>/`；域清单的唯一真源是 [`backend/internal/apitypes/domains.go`](./backend/internal/apitypes/domains.go)（同时是 swagger definitions 前缀与前端契约的域边界）
- 包内按**文件名**分层：`handler*.go`（HTTP 出口与路由）/ `service.go` / `dto.go` / `errors.go`，不建子目录（子目录 = 多包 = 导出面被迫扩大，且让「目录即射程」的守卫失明）
- 跨域共享只有三个出口：注入参数 → [`backend/pkg/httpx`](./backend/pkg/httpx) → [`backend/internal/core`](./backend/internal/core)；**没有 repository 层**，读写都在各域里
- 装配根 [`backend/internal/api`](./backend/internal/api) 只做 `NewDeps` + 路由登记，域包不得反向依赖它；三条分层规矩由 [`backend/internal/layers`](./backend/internal/layers/layer_guard.go) 可执行地守着
- 迁移账本与踩坑血账见 [`docs/agents/domain-package-migration.md`](./docs/agents/domain-package-migration.md)，决策理由见 [`ADR-0070`](./docs/adr/ADR-0070-域包形态与目录即射程的收口.md)

### 文件与对象存储

`storage_driver` 可切换 `local` / `r2`（S3 兼容）。生产在资源域 `cdn.<domain>`（公开 URL 基址 `R2_PUBLIC_DOMAIN`，只允许 GET/HEAD）把 `featured/ avatars/ contributions/ resumes/ images/ chapters/ slides/ uploads/ reports/` 等前缀反代到对象存储网关。

## 技术栈

### 后端

- Go 1.26（CI 钉 1.26.10）、Gin v1.12、GORM + pgx/v5、PostgreSQL 18
- **PostgreSQL 为唯一存储面**：验证码、图形码、refresh 黑名单等临时 KV 落在 `ephemeral_kv` 单表（#2109 起 Redis 退役），过期清扫是 daemon 里的一条 5 分钟守护
- golang-migrate、golang-jwt/v5、viper、zap + lumberjack、aws-sdk-go-v2（S3）、gofpdf
- AI：CloudWeGo Eino + OpenAI 兼容接口（DeepSeek 默认）；PPT / 文档转换走 LibreOffice sidecar
- 测试：glebarez/sqlite（多数单测）+ `testutil.NewPostgresDB`（跑真实 `migrations/` 的库层契约测试，无 `DATABASE_URL` 时干净 skip，首跑在 CI）

### 前端

- Vue 3.5 + TypeScript 5.7 + Vite 8（rolldown 内核；分块判据在 `frontend/vite.config.ts` 的 `CHUNK_GROUPS`）
- Element Plus 2.14 + Tailwind CSS v4（与既有 scoped 样式增量共存）；Pinia 4；vue-router 5
- axios（access 过期自动刷新重试）、ECharts 6、dayjs、marked / markstream-vue / vditor、KaTeX、Mermaid
- 测试：Vitest 5 + @vue/test-utils + happy-dom

### 移动端

- uni-app x（Vue 3 + UTS / uvue）；HBuilderX 运行 / 发行；Jest 单测

### 基础设施

- PostgreSQL 18、S3 兼容网关（Ceph RGW）、LibreOffice sidecar
- Docker Compose 编排；Caddy 反向代理（host 网络，自动证书 + `/api` 反代 + 静态托管 + 对象存储反代）
- GitHub Actions CI/CD，公网 SSH 部署到自托管服务器；备选 Cloudflare Pages（`frontend/wrangler.jsonc`）

## 项目结构

```text
.
├── backend/                 # Go 后端（module: forklift-training）
│   ├── cmd/                 # server / migrate / 代码生成（gen-*）/ 一次性回填工具
│   ├── internal/            # api（装配根）/ <功能域>/ / core / model / kv / layers / testutil 等
│   ├── pkg/                 # httpx / paging / response
│   ├── migrations/          # 迁移脚本（up/down 成对）
│   ├── docs/                # Swagger 生成产物
│   └── Dockerfile / docker-compose.yml（本地 postgres + libreoffice + backend）
├── frontend/                # Vue 3 前端（api / pages / components / stores / router / composables）
│   ├── Caddyfile            # 生产反向代理（自动证书；api / cdn / 各工作区子域）
│   ├── wrangler.jsonc       # Cloudflare Pages 备选
│   └── .env.example
├── training-app/            # uni-app x 移动端（学员端 + 招聘者端）
├── deploy/                  # 部署面（ADR-0088）：deploy.py / 生产 compose / 服务器脚本 / systemd / 判据测试
├── libreoffice-sidecar/     # 文档转换服务镜像（PPT → WebP、简历 PDF）
├── scripts/                 # CI 守卫与预检（check-*.mjs、preflight-pr.ps1、submit-pr.ps1）
├── docs/                    # adr（入库）/ agents（入库）/ design（入库）/ verification（入库）；
│                            # 其余目录默认不入库（见 .gitignore）
├── viz/                     # 设计原型（本地生成，不入库）
├── Makefile                 # 仓库根命令入口
└── .github/workflows/       # ci / cd / testing-smoke / pr-evidence / branch-residue
```

## 环境要求

- Go ≥ 1.26（`go.mod` 与 CI 都钉 1.26；本机可更新）
- Node 20.19+ 或 22.12+（Vite 8 要求；CI 用 24）
- Python 3.9+（仅部署 CLI `deploy/deploy.py`；目标服务器 Debian / PVE 自带 `python3`）
- GNU Make（Windows 用 Git Bash / WSL）
- Docker + Docker Compose（本地依赖与生产编排）
- PostgreSQL 18（由 docker-compose 提供或已有实例；**不再需要 Redis**）
- HBuilderX（仅移动端开发）

## 快速开始

### 1. 启动后端依赖（PostgreSQL + LibreOffice）

```bash
cd backend
docker compose up -d          # postgres :5432 / libreoffice :8000 / backend :18080
```

后端容器把 8080 映射到宿主机 **18080**（避开 Windows Hyper-V 预留端口段），前端 dev 代理默认指向 `127.0.0.1:18080`。

### 2. 执行迁移并启动后端（仓库根目录）

```bash
make migrate-up               # 执行数据库迁移（亦可 make dev 一步到位）
make run                      # 启动服务，默认 :8080
```

启动时自动加载配置、连接数据库并执行迁移、创建默认账号。健康检查：

```bash
curl http://localhost:8080/api/health
# {"code":200,"message":"success","data":{...}}
```

Swagger（非生产默认开启）：<http://localhost:8080/swagger/index.html>

### 3. 启动前端（仓库根目录）

```bash
cp frontend/.env.example frontend/.env.local   # 按需修改子域名
make frontend-install
make frontend-dev                              # 默认 :5173
```

开发服务器把 `/api` 与 `/static` 代理到 `http://127.0.0.1:18080`。本地多子域名开发需在 `hosts` 添加：

```text
127.0.0.1 training.localhost valuation.localhost mentor.localhost manage.localhost recruit.localhost
```

### 4. 移动端（可选）

用 **HBuilderX** 打开 `training-app/叉车维修培训学员端跨端应用`，通过「运行」选择 Android / iOS / H5 / 微信小程序。运行前把 `config/env.uts` 中的后端地址改为可达地址。

### 5. 访问系统

- 前端（开发）：<http://localhost:5173>（或各子域名）
- 后端 API：<http://localhost:8080/api>

## 默认账号

后端启动时自动创建默认账号，密码由环境变量配置（生产环境强制覆盖开发默认值，否则启动失败）：

| 角色 | 用户名 | 环境变量 |
| --- | --- | --- |
| 管理员 | admin | `ADMIN_DEFAULT_PASSWORD` |
| 讲师 | tutor | `TUTOR_DEFAULT_PASSWORD` |
| 学员 | student | `STUDENT_DEFAULT_PASSWORD` |

企业招聘者不预置账号，由管理员通过授信码开通。

## 配置

后端配置全部通过环境变量注入（viper AutomaticEnv + 集中默认值），非 production 自动读取 `backend/.env`；完整模板见 [`backend/.env.example`](./backend/.env.example)。生产环境（`APP_ENV=production`）会校验必填项与弱口令，缺失或仍为开发默认值则启动失败。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ENV` / `PORT` | 运行环境 / 端口 | development / 8080 |
| `DATABASE_URL` | PostgreSQL 连接串 | 空（生产必填） |
| `SECRET_KEY` / `JWT_SECRET_KEY` | 应用密钥 / JWT 签名密钥（生产必改） | 开发占位值 |
| `CORS_ORIGINS` | 允许的前端来源（含各子域名，逗号分隔） | 开发默认 localhost 各端口 / 子域 |
| `AUTH_COOKIE_DOMAIN` / `RECRUITER_COOKIE_DOMAIN` | 学员 / 招聘者登录态 Cookie 域 | localhost / 空 |
| `STORAGE_DRIVER` + `R2_*` | `local` 或 `r2` 对象存储 | local / 空 |
| `AI_BASE_URL` / `AI_MODEL` | OpenAI 兼容模型接入 | <https://api.deepseek.com> / deepseek-v4-flash |
| `RATE_LIMIT_*` / `LOG_*` / `SWAGGER_*` | 限流、日志轮转、Swagger | 见 `.env.example` |

> 配置默认值的**唯一宿主是 [`backend/internal/config/config.go`](./backend/internal/config/config.go)**（ADR-0060），代码内部不再各写一份 fallback；本表与代码不一致时，以代码为准并来改正这里。
> 部署侧默认值是生成物 `deploy/env.defaults`（`make gen-deploy`），唯一事实源在 `backend/internal/deploy`。
> 前端变量见 [`frontend/.env.example`](./frontend/.env.example)（主域与各工作区子域名）；`VITE_API_BASE_URL` 可覆盖 API 基址（缺省 `/api`，开发经 Vite 代理）。生产用 DNS 把各子域名指向同一服务器 IP。

## 常用命令

### 后端（在仓库根目录）

```bash
make build           # 编译二进制到 backend/bin/server
make run             # go run ./cmd/server
make test            # go test ./... -race -cover
make vet             # go vet ./...
make lint            # golangci-lint（本机 Windows 暂不可用，由 CI 兜底）
make check           # vet + test（提交前轻量检查）
make fmt / tidy      # 格式化 / 整理依赖
make swagger         # 重新生成 Swagger 文档
make gen             # 生成链：swagger -> apitypes -> aifeatures（顺序固定）
make gen-deploy      # 重新生成 deploy/env.defaults 与 Caddyfile 生成区
make migrate-up      # 执行数据库迁移
make migrate-down    # 回滚最近一次迁移
make dev             # dev-up + migrate-up + run（一步到位）
make dev-up / dev-down / dev-reset   # 起停依赖容器 / 清卷重建（数据丢失）
```

也可直接用 Go 运行迁移 CLI：`cd backend && go run ./cmd/migrate up|down|version|force|check-columns`。

### 前端（在仓库根目录）

```bash
make frontend-install    # npm ci（按 package-lock）
make frontend-dev        # Vite 开发服务器 :5173
make frontend-build      # 生产构建（含 precompress）
make frontend-typecheck  # vue-tsc --noEmit
make frontend-test       # vitest run
make frontend-check      # typecheck + test
make check-all           # 后端 check + 前端 check
```

等价的原生命令是 `cd frontend && npm ci / npm run dev / ...`；`make` 只是从仓库根统一入口。

### 移动端（在 `training-app/叉车维修培训学员端跨端应用/` 下）

用 **HBuilderX** 打开目录，通过「运行 / 发行」选择目标平台；命令行测试与检查见移动端 `AGENTS.md`。

### 只跑一个测试

- 后端：`go test ./internal/core/ -race`；单用例 `-run TestXxx`
- 前端：`npx vitest run src/api/__tests__/page.spec.ts`；按名 `-t "用例名片段"`
- 移动端：按套件所在腿选 `jest --config`（选错腿会让 jest 当套件不存在）

## 数据库迁移

迁移脚本在 `backend/migrations/`，`序号_名称.up.sql` / `.down.sql` 成对组织。**这里不列清单**——清单看目录本身，意图看每条迁移的头部注释。

配对性与真实可执行由 CI 的 `migration-check` 保证：空库 `migrate up` → 列 / 索引对账（按 `current_schema()` 收窄）→ `migrate down` 回空库并反向断言。**部署侧迁移失败默认中止**（`ALLOW_MIGRATION_FAILURE=1` 才带已知风险继续）。

## 测试与检查

> 权威版本在 [`docs/agents/checks.md`](./docs/agents/checks.md)（生成链顺序、PG 契约测试纪律、守卫清单与 CI 触发模型）。本节只留入口。

- **后端**（`backend/`）：`gofmt -l .` → `go vet ./...` → `golangci-lint run ./...` → `go test ./...`。本机 Windows 暂时跑不了 `golangci-lint`，由 CI 的 `backend-lint` 兜底；WSL 下 Go 工具链在 `~/go/bin`
- **守卫**：改前端 UI / api 消费面 / 端点错误面 / 目录排序 / 图标字号后，跑对应的 `node scripts/check-*.mjs --all`；自检 `node --test scripts/*.test.mjs`
- **前端**（`frontend/`）：`make frontend-typecheck`、`make frontend-test`
- **部署配置**：`docker compose -f deploy/docker-compose.prod.yml config -q`；变更计划 `python3 deploy/deploy.py --plan`；判据测试见 `deploy/README.md`「判据物与本地校验」
- **安全检测**：触及认证 / 授权 / 密钥 / DB 连接 / AI 生成代码时跑 DeepSec（Shield），确认无新增 critical / high

## 部署

部署面自 ADR-0088 起聚合在 [`deploy/`](deploy/)，完整说明（按变更重启、回滚、PostgreSQL 大版本升级 runbook、域名 / 回源 / 生产诊断）见 [`deploy/README.md`](deploy/README.md)。

```bash
python3 deploy/deploy.py local                 # 构建前端 → 迁移 → 按变更部署 → 健康检查
python3 deploy/deploy.py local --skip-build    # 跳过前端构建
python3 deploy/deploy.py --plan                # 只打印计划（--json 机器可读）
python3 deploy/deploy.py rollback              # 回滚到 backups/last-version.txt
python3 deploy/deploy.py down                  # 停栈（testing 冒烟收尾，命名卷保留）
```

- `deploy/deploy.py` 按「容器状态 + 镜像 ID + compose config-hash」算计划，只重建真正变化的服务；`postgres` 默认永不因应用变更而重建（配置漂移会中止，需显式 `--force-data-services`）
- `deploy/docker-compose.prod.yml` 编排 `postgres` / `libreoffice` / `backend`（仅内网）/ `frontend`（Caddy，host 网络监听 80/443）
- CD 通过公网 SSH 把部署文件上传到服务器 `/opt/forklift-training` 后执行 `deploy.py`；失败自动回滚，备份默认落在 `$DEPLOY_PATH/backups`
- 服务器初始化、备份 / 快照、巡检脚本与 systemd 单元都在 `deploy/` 下

### CI/CD（GitHub Actions）

- **触发模型**：非 master 分支 push 跑全量 CI；**master 合并直发 production**（`gate` 回查来源 PR 的 `ci-summary` 与 testing 冒烟结论）；PR 事件不直接触发 ci / cd 主流水（PR 页显示的是同 commit 的分支检查），PR 侧另有 `testing-smoke` / `pr-evidence`
- `ci.yml`：`changes` → `backend-lint` / `backend-test`（按包分两片）/ `frontend-check` / `security-scan` / `migration-check` / `mobile-test` → `ci-summary`；含守卫与门自身的自检 job
- `cd.yml`：构建并推送镜像（内容未变则跳过）→ SSH 部署 → 健康检查 → 失败回滚；testing 探活通过后停容器
- `testing-smoke.yml`：PR 打开 / 同步时补齐 testing 冒烟，并等到部署真的成功才放行检查
- `pr-evidence.yml`：验收证据段校验，**只对命中运行时面的 PR 判红**；证据 = PR 正文一行 `跑于 <sha7>` + 门读数行原样（A1 形态，#2100）
- `branch-residue.yml`：每日扫描合并后残留分支（只报警、不删）
- **合并门禁**：ruleset 要求必检 `ci-summary` 通过且分支与 master 同步（PR 侧 `testing-smoke` 同属必过检查，须等到冒烟真的成功）；squash 生成的新 SHA 由生产 `gate` 回查来源 PR 对齐

## 相关文档

| 文档 | 内容 |
| --- | --- |
| [`CONTEXT.md`](./CONTEXT.md) | 领域词汇表（canonical 术语与 Avoid 清单） |
| [`API.md`](./API.md) | API 清单 |
| [`AGENTS.md`](./AGENTS.md) | AI / agent 工作约定、验收门与发布流程 |
| [`docs/adr/`](./docs/adr/) | 架构决策记录 ADR-0001 ~ ADR-0090（91 篇） |
| [`docs/agents/`](./docs/agents/) | agent 工作约定（**checks.md** 为测试与检查权威版本，**release.md** 为发布流程） |
| [`docs/README.md`](./docs/README.md) | `docs/` 索引与入库 / 本地分层规则 |
| [`deploy/README.md`](deploy/README.md) | 部署面：CLI、编排、数据库升级 runbook、域名 / 回源 / 诊断 |
| [`training-app/叉车维修培训学员端跨端应用/AGENTS.md`](training-app/叉车维修培训学员端跨端应用/AGENTS.md) | 移动端工作约定与验收门 |
| [`THIRD_PARTY.md`](./THIRD_PARTY.md) | 第三方组件与许可 |
| `docs/reference/CODE_WIKI.md` | 代码百科（本地不入库，故不设链接） |

## 许可证

本项目为**和润天下（广州）人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
