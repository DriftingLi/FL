# 叉车维修培训与残值评估系统

一套面向叉车维修培训与叉车残值评估的全栈系统，包含在线培训、考试练习、AI 助手，以及叉车残值评估与电池剩余寿命（RUL）评估等业务模块。系统按角色（学生 / 讲师 / 管理员）划分权限，并提供独立的残值评估工作区。

## 功能特性

### 培训学习模块

- **课程管理**：课程 CRUD、章节内容编排、PPT / 视频 / 图文混排
- **考试系统**：课程考试、模拟考试、等级考试，自动判分与成绩统计
- **练习中心**：自由练习、知识点练习、错题本、练习统计
- **AI 助手**：基于大模型的智能问答与内容生成（Coze Web SDK / 智谱 GLM）

### 残值评估模块

- **叉车残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域等参数，输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **电池 RUL 评估**：基于特征提取与混合神经网络的锂电池剩余寿命预测（含 SOH、置信区间与建议）
- **PDF 报告**：评估结果一键生成可下载的中文 PDF 报告
- **管理后台**：原价表配置、算法参数调整、系数表管理

### 移动端（学员 App）

- **uni-app x** 跨端应用（Android / iOS / H5 / 微信小程序）
- 当前落地功能：学员登录、学习仪表盘（学习统计与九宫格入口）；其余模块为占位

## 系统架构

单仓库包含三个可独立部署的部分：**后端（Go）**、**前端（Vue3）**、**移动端（uni-app x）**。前后端通过 HTTP API 通信。

### 子域名多工作区（前端）

前端为单一应用，按访问子域名切换工作区，路由守卫在不同子域名间整页跳转：

| 子域名 | 工作区 | 主要角色 | 功能 |
| --- | --- | --- | --- |
| `www`（根域名 / localhost） | 官网门户 | 访客 | 首页、派单（`/dispatch` 占位） |
| `training.` | 培训学习 | 学员 | 课程、考试、练习、AI 助手 |
| `mentor.` | 导师工作区 | 讲师 | 课程管理、题库、阅卷 |
| `valuation.` | 残值评估 | 访客 / 估值用户 | 整机残值、电池 RUL、报告 |
| `manage.` | 管理后台 | 管理员 | 学员 / 讲师 / 课程 / 题库 / 残值配置 |

不同子域名的 `localStorage` token **不共享**，切换需重新登录。

### 双鉴权体系（后端）

后端存在两套相互独立的 JWT 体系：

- **培训体系**：`JWT_SECRET_KEY`，用户为 `student` / `tutor` / `admin`，路径前缀 `/api/*`（除 `/api/valuation/*`）。
- **残值体系**：`VALUATION_JWT_SECRET_KEY`，用户为 `valuation_user`，独立用户表与登录接口 `/api/valuation/auth/*`。

统一响应结构：`{ "code": 0, "message": "...", "data": ... }`（code=0 表示成功）。

## 技术栈

### 后端（backend）

- 语言：Go 1.26
- Web 框架：Gin v1.10 + gin-contrib/cors
- 数据库：PostgreSQL 15 + pgx/v5 + GORM
- 缓存：Redis 7（go-redis/v9）
- 数据库迁移：golang-migrate/v4
- 认证：golang-jwt/v5（双体系）
- 日志：zap
- PDF 生成：gofpdf（中文 SimHei 字体）
- AI 集成：智谱 GLM（OpenAI 兼容接口）+ go-openai（备用）+ Coze Web SDK
- 测试库：glebarez/sqlite（单元测试用 SQLite）

### 前端（frontend）

- 框架：Vue 3.4 + TypeScript 5.7
- 构建：Vite 6
- UI：Element Plus 2.5 + @element-plus/icons-vue
- 状态管理：Pinia 2.1（含独立 `valuationAuth` store）
- 路由：vue-router 4（子域名多工作区 + 双鉴权守卫）
- HTTP：axios
- 图表：ECharts 6（DimensionRadar / FutureValueChart / BatteryRadar）
- 其他：dayjs、marked + highlight.js、pdfjs-dist、vditor、vuedraggable、@coze/web-sdk

### 移动端（training-app）

- 框架：uni-app x（Vue 3 + UTS / uvue）
- 工具：HBuilderX（运行 / 发行）
- 跨端目标：Android、iOS、H5、微信小程序
- 网络：uni.request 封装，Bearer Token 认证，依赖独立后端 `:8080`

### 基础设施

- 数据库：PostgreSQL 15
- 缓存：Redis 7（生产环境必需，用于 JWT 黑名单与缓存）
- 编排：Docker Compose
- 反向代理 / 静态托管：Nginx（前端容器兼任 SSL 终止 + API 反代）
- CI/CD：GitHub Actions（ci.yml / cd.yml），通过公网 SSH（端口 2222）部署到自托管服务器
- 备选：Cloudflare Pages（wrangler.jsonc）

## 项目结构

```
叉车维修项目/
├── backend/                      # Go 后端（module: forklift-training）
│   ├── cmd/
│   │   ├── server/               # 服务入口（默认 :8080，启动时自动迁移 + 建默认账号）
│   │   └── migrate/              # 数据库迁移 CLI（up | down | version）
│   ├── internal/
│   │   ├── api/                  # 培训业务 Gin 路由与 handler
│   │   ├── service/              # 培训业务服务层
│   │   ├── model/ repository/ db/ config/ middleware/ migrate/ testutil/
│   │   └── valuation/            # 残值评估 + 电池 RUL 子模块（独立 handler/repository/service/config）
│   ├── pkg/
│   │   ├── response/             # 统一响应结构
│   │   └── pdf/                  # 中文 PDF 报告（gofpdf + SimHei）
│   ├── migrations/               # 迁移脚本（000001 ~ 000004）
│   ├── queries/ sqlc.yaml         # 残值子模块 SQL 与代码生成配置
│   ├── Dockerfile
│   ├── Makefile
│   ├── docker-compose.yml        # 本地 postgres + redis
│   ├── .env                      # 本地开发环境变量（已随仓库提供）
│   └── init-db.sql
├── frontend/                     # Vue3 前端（forklift-training-frontend）
│   ├── src/                      # 源码（api/pages/components/stores/router/utils）
│   ├── Dockerfile
│   ├── nginx.default.conf        # 多子域名 Nginx 站点配置
│   ├── docker-entrypoint.sh
│   ├── wrangler.jsonc            # Cloudflare Pages 备选部署
│   ├── .env.example
│   └── docs/
├── training-app/
│   └── 叉车维修培训学员端跨端应用/  # uni-app x 学员端移动 App
├── scripts/
│   ├── deploy-remote.sh          # 服务器远程部署（支持 --rollback）
│   └── setup-server.sh           # 服务器初始化
├── .github/workflows/            # CI/CD（ci.yml / cd.yml）
├── deploy.sh                     # 本地 / 手动一键部署
└── docker-compose.prod.yml       # 生产编排（PostgreSQL + Redis + 后端 + Nginx 前端）
```

## 环境要求

- Go ≥ 1.26
- Node.js ≥ 18（推荐 20+）
- Docker + Docker Compose（本地 PostgreSQL / Redis，以及生产编排）
- PostgreSQL 15（由 docker-compose 提供或已有实例）
- Redis 7（生产必需；本地开发用于 JWT 黑名单）
- HBuilderX（仅移动端开发需要）

## 快速开始

### 1. 启动后端依赖（PostgreSQL + Redis）

```bash
cd backend
docker compose up -d          # 启动 postgres + redis 容器
```

### 2. 执行迁移并启动后端

```bash
cd backend
make migrate-up               # 执行数据库迁移（亦可 make dev 一步到位）
make run                      # 启动服务，默认 :8080
```

启动后会自动加载 `.env`、连接数据库并执行迁移、创建默认账号。健康检查：

```bash
curl http://localhost:8080/api/health
# {"code":0,"message":"...","data":{...}}
```

### 3. 启动前端

```bash
cd frontend
cp .env.example .env.local    # 按需修改子域名 / API 地址
npm install
npm run dev                   # 默认 :5173
```

开发服务器已配置代理：`/api` 与 `/static` 请求转发至 `http://localhost:8080`。本地多子域名开发需在 `hosts` 添加：

```
127.0.0.1 training.localhost valuation.localhost mentor.localhost manage.localhost
```

### 4. 移动端（可选）

用 **HBuilderX** 打开 `training-app/叉车维修培训学员端跨端应用`，通过「运行」选择 Android / iOS / H5 / 微信小程序。运行前需先把 `config/env.uts` 中的后端地址改为可达的 `:8080` 地址。

### 5. 访问系统

- 前端（开发）：<http://localhost:5173>（或各子域名）
- 后端 API：<http://localhost:8080/api>

## 默认账号

后端启动时会自动创建默认账号（管理员 / 讲师 / 学员），密码由环境变量配置：

| 角色  | 用户名     | 环境变量                       |
| --- | ------- | -------------------------- |
| 管理员 | admin   | `ADMIN_DEFAULT_PASSWORD`   |
| 讲师  | tutor   | `TUTOR_DEFAULT_PASSWORD`   |
| 学员  | student | `STUDENT_DEFAULT_PASSWORD` |

残值评估工作区另有独立的 `valuation_user` 用户体系，通过 `/api/valuation/auth/register` 自行注册。

## 配置说明

后端配置通过环境变量注入（非 production 环境自动读取 `backend/.env`）。生产环境（`APP_ENV=production`）会校验必填项（`SECRET_KEY`、`JWT_SECRET_KEY`、`VALUATION_JWT_SECRET_KEY`、`DATABASE_URL`、`REDIS_ADDR`），缺失则启动失败。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ENV` | 运行环境 | development |
| `PORT` | 服务端口 | 8080 |
| `DATABASE_URL` | PostgreSQL 连接串 | 由 docker-compose 提供 |
| `SECRET_KEY` | 应用密钥（生产必改） | 空 |
| `JWT_SECRET_KEY` | 培训体系 JWT 签名密钥（生产必改） | 空 |
| `VALUATION_JWT_SECRET_KEY` | 残值体系 JWT 签名密钥（生产必改） | 空 |
| `JWT_EXPIRES_HOURS` | JWT 有效期（小时） | 24 |
| `REDIS_ADDR` | Redis 地址 | localhost:6379 |
| `REDIS_PASSWORD` | Redis 密码 | 空 |
| `REDIS_DB` / `REDIS_POOL_SIZE` / `REDIS_KEY_PREFIX` | Redis 库 / 连接池 / 键前缀 | 0 / 10 / fl: |
| `CORS_ORIGINS` | 允许的前端来源（含五个子域名，逗号分隔） | <http://localhost:5173> |
| `UPLOAD_FOLDER` | 上传目录 | static/uploads |
| `MAX_CONTENT_LENGTH_MB` | 上传大小上限 | 250 |
| `ADMIN_DEFAULT_PASSWORD` / `TUTOR_DEFAULT_PASSWORD` / `STUDENT_DEFAULT_PASSWORD` | 默认账号密码（生产必改） | 空 |
| `ZHIPU_API_KEY` / `ZHIPU_BASE_URL` / `ZHIPU_MODEL` | 智谱 GLM 配置 | glm-4.7-flash |
| `OPENAI_API_KEY` | OpenAI 密钥（ZHIPU 为空时备用） | 空 |
| `VALUATION_PDF_OUTPUT_DIR` | 评估报告 PDF 输出目录 | storage/reports |

前端环境变量（`.env.example`）：

- `VITE_API_BASE_URL`：API 地址，默认 `/api`（开发经 Vite 代理转发至 `localhost:8080`）
- `VITE_MAIN_DOMAIN` / `VITE_TRAINING_SUBDOMAIN` / `VITE_VALUATION_SUBDOMAIN` / `VITE_MENTOR_SUBDOMAIN` / `VITE_ADMIN_SUBDOMAIN`：五个工作区域名（生产通过 DNS A 记录指向同一 IP）

## 常用命令

### 后端（在 `backend/` 下）

```bash
make build           # 编译二进制到 bin/server
make run             # go run ./cmd/server
make test            # 运行测试（-race -cover）
make lint            # 代码检查（golangci-lint）
make fmt / tidy      # 格式化 / 整理依赖
make migrate-up      # 执行数据库迁移
make migrate-down    # 回滚最近一次迁移
make dev-up          # 启动 postgres + redis 容器
make dev-down        # 停止容器
make dev-reset       # 清除数据卷并重建（数据丢失）
```

也可直接用 Go 运行迁移 CLI：

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down
```

### 前端（在 `frontend/` 下）

```bash
npm install          # 安装依赖
npm run dev          # 启动开发服务器（:5173）
npm run build        # 生产构建（dist/）
npm run preview      # 预览构建产物
npm run type-check   # vue-tsc 类型检查
```

> 注：项目含 eslint / prettier 配置，但 `scripts` 中未定义 `lint` 命令，可手动 `npx eslint src`。

### 移动端（在 `training-app/叉车维修培训学员端跨端应用/` 下）

通过 **HBuilderX** 打开目录，使用菜单「运行 / 发行」选择目标平台。无 npm 脚本。

## 数据库迁移

迁移脚本位于 `backend/migrations/`，采用 `序号_名称.up.sql` / `.down.sql` 成对组织，当前共 **4 组**（000001 ~ 000004）：

- `000001_init_baseline` — 培训库基线
- `000002_question_reject_reason` — 题库驳回原因
- `000003_featured_content` — 精选内容
- `000004_valuation_users` — 残值评估用户表

执行 / 回滚：`make migrate-up` / `make migrate-down`。

## 部署

### 本地 / 手动一键部署

```bash
./deploy.sh                 # 交互式选择环境
./deploy.sh production      # 部署到生产
./deploy.sh --skip-build    # 跳过前端构建
./deploy.sh --skip-migrate  # 跳过数据库迁移
```

脚本流程：环境检查 → 加载 `.env` → 构建前端 → 数据库迁移 → `docker compose` 部署 → 健康检查。

### 生产编排

`docker-compose.prod.yml` 编排四个服务：

- `postgres`（PostgreSQL 15，持久化卷 `pgdata-prod`）
- `redis`（Redis 7，仅内网，LRU 淘汰、禁用持久化）
- `backend`（Go 后端，仅内网，经前端反代访问）
- `frontend`（Nginx，SSL 终止 + `/api` 反代 + 静态托管，暴露 80/443）

```bash
docker compose -f docker-compose.prod.yml up -d
```

### 远程部署（自托管服务器）

`scripts/deploy-remote.sh` 通过公网 SSH（端口 2222）将构建产物部署到服务器 `/opt/forklift-training`，支持 `--rollback`。`scripts/setup-server.sh` 负责服务器初始化。

### CI/CD（GitHub Actions）

- `ci.yml`：gofmt / go vet / golangci-lint → 测试（race + cover）→ 前端 type-check + build → 安全扫描 → 迁移校验
- `cd.yml`：构建并推送镜像（ghcr.io）→ 公网 SSH 部署 → 健康检查 → 失败自动回滚

## 许可证

本项目为**和润天下人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
