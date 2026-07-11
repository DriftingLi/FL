# 叉车维修培训与残值评估系统

一套面向叉车维修培训与叉车残值评估的全栈系统，包含在线培训、考试练习、AI 助手，以及叉车残值评估与电池剩余寿命（RUL）评估等业务模块。系统按角色（学生 / 讲师 / 管理员）划分权限，提供完整的后台管理与学生学习闭环。

> 前端官网设计风格规范见 [Design.md](./Design.md)（和润天下 HRWAI 品牌规范）。

## 功能特性

### 培训学习模块
- **课程管理**：课程 CRUD、章节内容编排、PPT / 视频 / 图文混排
- **考试系统**：课程考试、模拟考试、等级考试，自动判分与成绩统计
- **练习中心**：自由练习、知识点练习、错题本、练习统计
- **AI 助手**：基于大模型的智能问答与内容生成（智谱 GLM / OpenAI）
- **3D 交互**：基于 Three.js 的叉车拆装演示与操作回放

### 残值评估模块
- **叉车残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域等参数，输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **电池 RUL 评估**：电池剩余使用寿命评估（对应前端 `valuationBattery` store）
- **PDF 报告**：评估结果一键生成可下载的 PDF 报告
- **管理后台**：原价表配置、算法参数调整、系数表管理

## 技术栈

### 后端（backend-go）
- 语言：Go 1.26
- Web 框架：Gin v1.10 + gin-contrib/cors
- 数据库：PostgreSQL 15 + pgx/v5 + GORM
- 数据库迁移：golang-migrate/v4
- 认证：golang-jwt/v5
- 日志：zap
- PDF 生成：gofpdf
- AI 集成：智谱 GLM（OpenAI 兼容接口）+ go-openai（备用）
- 测试库：glebarez/sqlite（SQLite，用于单元测试）

### 前端（frontend）
- 框架：Vue 3.4 + TypeScript 5.7
- 构建：Vite 6
- UI：Element Plus 2.5 + @element-plus/icons-vue
- 状态管理：Pinia
- 路由：vue-router 4
- HTTP：axios
- 图表：ECharts 6
- 3D：Three.js
- 其他：dayjs、marked + highlight.js、pdfjs-dist、vuedraggable、@coze/web-sdk

## 项目结构

```
叉车维修项目/
├── backend-go/                  # Go 后端
│   ├── cmd/                     # 入口：server（服务）、migrate（迁移）、migrate-data（数据搬迁）、visual_check（辅助检查）
│   ├── internal/                # 业务分层
│   │   ├── api/ config/ db/ middleware/ model/ repository/ service/ testutil/
│   │   └── valuation/           # 残值评估子模块（独立 handler/repository/service）
│   ├── migrations/              # PostgreSQL 迁移脚本（000001 ~ 000017）
│   ├── pkg/                     # 通用包（response、pdf 生成器等）
│   ├── storage/                 # 运行期存储（报告、登录态等）
│   ├── static/                  # 静态资源（上传文件、视频）
│   ├── .env                     # 本地开发环境变量（已随仓库提供）
│   ├── .env.production          # 生产环境变量模板
│   ├── docker-compose.yml       # 本地 PostgreSQL 容器
│   ├── Dockerfile               # 后端镜像构建
│   ├── railway.toml / nixpacks.toml  # 备选 PaaS 部署配置（Railway）
│   └── Makefile                 # 构建 / 迁移 / 开发命令
├── frontend/                    # Vue 前端
│   ├── src/                     # 源码（api/pages/components/... 见上文）
│   ├── wrangler.jsonc           # Cloudflare Pages 部署配置
│   └── package.json
├── nginx/                       # 可选 Nginx 反向代理配置
│   ├── nginx.conf               # 主配置
│   └── default.conf             # 站点配置（/api、/static 反代 + SPA 回退）
├── scripts/                     # 服务器部署与初始化脚本
│   ├── deploy-remote.sh         # 服务器远程部署（支持 --rollback）
│   ├── setup-server.sh          # 服务器初始化
│   ├── setup-tailscale.sh       # Tailscale 组网
│   └── swap-cf-tunnel.sh        # Cloudflare Tunnel 切换
├── .github/workflows/           # CI/CD（ci.yml 持续集成 / cd.yml 持续部署）
├── deploy.sh                    # 一键部署脚本（本地 / 手动）
├── docker-compose.prod.yml      # 生产编排（PostgreSQL + 后端镜像）
├── Design.md                    # 官网设计风格规范（和润天下 HRWAI）
└── README.md
```

> 仓库根目录另含 `forklift-backend.tar` 与 `postgres.tar`——预构建镜像导出，离线部署时可用 `docker load -i <file>` 直接载入，免去重新构建。

## 环境要求

- Go ≥ 1.26
- Node.js ≥ 18（推荐 20+）
- Docker + Docker Compose（用于本地 PostgreSQL）
- PostgreSQL 15（由 docker-compose 提供，或使用已有实例）

## 快速开始

### 1. 启动数据库

```bash
cd backend-go
docker compose up -d postgres
```

### 2. 配置后端环境变量

后端已提供两份环境变量文件：
- `.env` —— 本地开发配置（已随仓库提供，可直接使用）
- `.env.production` —— 生产环境模板，部署时复制为 `.env` 并填入实际值

```bash
# 本地开发：直接使用现有的 .env 即可
# 生产部署：基于模板创建 .env
cp .env.production .env
# 按需修改 SECRET_KEY / JWT_SECRET_KEY / ZHIPU_API_KEY 等
```

### 3. 执行数据库迁移并启动后端

```bash
make dev          # 等价于：dev-up + migrate-up + run
# 或分步执行
make migrate-up   # 执行迁移
make run          # 启动服务（默认 :8080）
```

后端启动时会自动：
- 加载 `.env`
- 连接 PostgreSQL 并执行迁移
- 初始化默认账号

启动后可通过健康检查接口验证：

```bash
curl http://localhost:8080/api/health
# {"status":"ok","message":"backend is running"}
```

### 4. 启动前端

```bash
cd frontend
npm install
npm run dev       # 默认 :5173
```

前端开发服务器已配置代理：`/api` 与 `/static` 请求转发至 `http://localhost:8080`。

### 5. 访问系统

- 前端：http://localhost:5173
- 后端 API：http://localhost:8080/api

## 默认账号

系统启动时会自动创建默认账号（管理员 / 讲师 / 学员），默认密码通过以下环境变量配置：

| 角色 | 用户名 | 环境变量 |
|------|--------|----------|
| 管理员 | admin | `ADMIN_DEFAULT_PASSWORD` |
| 讲师 | tutor | `TUTOR_DEFAULT_PASSWORD` |
| 学员 | student | `STUDENT_DEFAULT_PASSWORD` |

> 生产环境必须通过环境变量设置强密码，不可使用开发默认值。

## 配置说明

后端配置通过 `backend-go/.env` 注入，关键项：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `APP_ENV` | 运行环境 | development |
| `PORT` | 服务端口 | 8080 |
| `DATABASE_URL` | PostgreSQL 连接串 | 由 docker-compose 提供，本地开发参考 `.env` |
| `SECRET_KEY` | 应用密钥（生产必改） | 开发默认值见 `.env` |
| `JWT_SECRET_KEY` | JWT 签名密钥（生产必改） | 开发默认值见 `.env` |
| `JWT_EXPIRES_HOURS` | JWT 有效期（小时） | 1（`.env.production` 为 24） |
| `CORS_ORIGINS` | 允许的前端来源 | http://localhost:5173 |
| `UPLOAD_FOLDER` | 上传目录 | static/uploads |
| `MAX_CONTENT_LENGTH_MB` | 上传大小上限 | 250 |
| `ADMIN_DEFAULT_PASSWORD` | 管理员默认密码（生产必改） | 开发默认值见 `.env` |
| `TUTOR_DEFAULT_PASSWORD` | 讲师默认密码（生产必改） | 开发默认值见 `.env` |
| `STUDENT_DEFAULT_PASSWORD` | 学员默认密码（生产必改） | 开发默认值见 `.env` |
| `ZHIPU_API_KEY` | 智谱 GLM API Key（主用 AI） | 空 |
| `ZHIPU_BASE_URL` | 智谱 GLM 接口地址 | https://open.bigmodel.cn/api/paas/v4 |
| `ZHIPU_MODEL` | 智谱 GLM 模型名 | glm-4.7-flash |
| `OPENAI_API_KEY` | OpenAI 密钥（ZHIPU 为空时备用） | 空 |
| `COZE_*` | Coze OAuth 配置 | 空 |
| `VALUATION_PDF_OUTPUT_DIR` | 评估报告 PDF 输出目录 | storage/reports |

## 常用命令

### 后端（在 `backend-go/` 下）

```bash
make build           # 编译二进制到 bin/server
make run             # go run ./cmd/server
make test            # 运行测试（-race -cover）
make lint            # 代码检查（golangci-lint）
make fmt             # 格式化代码
make tidy            # 整理依赖
make migrate-up      # 执行数据库迁移
make migrate-down    # 回滚最近一次迁移
make dev-up          # 启动 PostgreSQL 容器
make dev-down        # 停止容器
make dev-reset       # 清除数据卷并重建（数据丢失）
make migrate-data SOURCE="postgres://..."  # 从旧库搬迁数据
```

### 前端（在 `frontend/` 下）

```bash
npm run dev          # 启动开发服务器
npm run build        # 生产构建
npm run preview      # 预览构建产物
npm run type-check   # TypeScript 类型检查
```

## 数据库迁移

迁移脚本位于 `backend-go/migrations/`，采用 `序号_名称.up.sql` / `.down.sql` 成对组织，共 17 组（000001 ~ 000017），覆盖初始化、残值评估表结构、级联过滤、种子数据、系数调整等。

```bash
make migrate-up      # 升级到最新
make migrate-down    # 回滚一步
```

## 部署

生产环境为**自托管服务器**（Proxmox PVE + Tailscale 组网），整体架构：

- **后端**：`docker-compose.prod.yml` 在服务器上以 Docker 运行 PostgreSQL 15 与 Go 后端镜像；后端通过 **Cloudflare Tunnel** 对外暴露。
- **前端**：由 **Cloudflare Pages** 托管，监听仓库 `master` 分支自动构建部署（配置见 `frontend/wrangler.jsonc`）；前端经 `CORS_ORIGINS` 配置直接调用后端 API。
- **可选反向代理**：`nginx/` 提供宿主 Nginx 配置，可在单台服务器上统一托管前端静态资源并反代 `/api`、`/static`（含 SPA 回退、gzip、安全头）。

### 本地 / 手动一键部署

根目录 `deploy.sh` 封装完整流程（环境检查 → 加载 `.env` → 构建前端 → 迁移数据库 → `docker compose` 启动 → 健康检查）：

```bash
./deploy.sh                 # 交互式选择 staging / production
./deploy.sh production      # 直接部署到生产
./deploy.sh --skip-build    # 跳过前端构建（仅重启）
./deploy.sh --skip-migrate  # 跳过数据库迁移
```

### 生产编排（docker-compose.prod.yml）

```bash
# 前置：准备 .env（可基于 .env.production 复制），并放置 coze_private_key.pem（如使用 Coze）
docker compose -f docker-compose.prod.yml up -d
```

该编排包含：
- `postgres`：PostgreSQL 15，数据卷 `pgdata-prod` 持久化，带健康检查
- `backend`：后端镜像（默认 `forklift-backend:latest`，或 `BACKEND_IMAGE` 指定 ghcr.io 镜像），依赖数据库健康后启动

### 服务器初始化与远程部署脚本

`scripts/` 目录提供部署配套脚本：

| 脚本 | 用途 |
|------|------|
| `deploy-remote.sh` | 服务器侧部署执行器，拉取镜像、运行编排与迁移；支持 `bash deploy-remote.sh --rollback` 回滚 |
| `setup-server.sh` | 新服务器初始化（安装 Docker、创建部署目录等） |
| `setup-tailscale.sh` | 配置 Tailscale 组网，使服务器加入私有 tailnet |
| `swap-cf-tunnel.sh` | 切换 / 重建 Cloudflare Tunnel 入口 |

### 持续集成与部署（CI/CD）

GitHub Actions 自动驱动质量门禁与发布（详见 `.github/workflows/`）：

- **`ci.yml`（持续集成）**：在 `master` / `develop` 的 Push 与 PR 触发，仅运行变更模块。阶段包括：
  - 后端：`gofmt` 格式检查、`go vet` 静态分析、`golangci-lint` 代码检查、单元测试（`-race -cover`，PostgreSQL 服务）、迁移脚本 up/down 完整性校验
  - 前端：`npm install`、TypeScript 类型检查、ESLint、`vite build`、依赖审计
  - 安全扫描：`govulncheck`、Trivy（Dockerfile，仅报告不阻断）
- **`cd.yml`（持续部署）**：CI 通过后自动触发（或手动 `workflow_dispatch`）：
  - 构建后端镜像并推送至 `ghcr.io`
  - 通过 **Tailscale SSH** 登录自托管服务器，上传 `deploy-remote.sh` 与 `docker-compose.prod.yml`，注入环境变量并执行部署
  - 部署后执行 API 健康检查，**失败时自动回滚**至上一镜像
  - 前端由 Cloudflare Pages 自动部署，无需本流水线处理

> 备选 PaaS 方案：`backend-go/railway.toml` 与 `backend-go/nixpacks.toml` 仍保留，可作为 Railway 等平台的部署配置。

## 项目约定

- **残值配置模块**：管理员后台仅保留两个配置表——叉车原价表（`original_prices`）与参数调整表（`coefficient_configs`）。
- **字典取值**：车辆系列选项不允许使用"无"，统一用"其它"；原价表的配置类型 / 门架类型不允许空字符串，统一用"无"。
- **最早出厂年份**：`original_prices` 每条记录独立配置 `earliest_factory_year`，学生端出厂年份下限按品牌+车型+系列+吨位级联查询其最小值。
- **品牌系数**：使用 `k_brand` 字段替代旧的 `k_type`；`config_types` 与 `brand_types` 表已废弃删除。
- **表格展示**：管理后台所有配置表格均不展示 ID 列。
- **提交信息**：使用中文编写 Git 提交信息。

## 版本更新历史

> 项目使用 Git 管理源码（GitHub 私有仓库 `DriftingLi/FL`），目前尚未打版本标签。以下按里程碑（提交时间线）记录，后续发布建议以 `vX.Y.Z` 打标签并同步本表。

### v1.0.0（2026-06-26，基线发布）
- 合并「维修培训」与「残值评估」为单一 monorepo（`backend-go` + `frontend`）
- Go 后端（Gin + GORM）单进程、单端口；维修培训路由 `/api/*` 与残值评估路由 `/api/valuation/*` 共享 JWT / CORS 中间件
- 前端统一 Vue 3 + TypeScript + Element Plus + 设计 token 体系，残值评估页面整合进 `student/valuation/`
- 数据库迁移统一编号 000001 ~ 000004
- 培训学习模块（课程 / 考试 / 练习 / AI 助手 / 3D 拆装）与残值评估模块（五维系数雷达图、置信区间、PDF 报告）全量上线
- 后端由 Python Flask 重构为 Go，与原 API 契约兼容
- 接入智谱 GLM（ZHIPU）为主、OpenAI 为备的 AI 能力

### 残值评估重构（2026-06-27 ~ 07-03）
- 级联筛选与"无"选项适配，支持无配置场景
- 车型 / 系列最早出厂年份字段及出厂年份级联限制
- 车况 Kc 修正项迁移至数据库；品牌系数由 `k_type` 重构为 `k_brand`（废弃 `config_types` / `brand_types` 表）
- 区域系数、参数调整表、原价表筛选与重置
- 估值报告新增「未来价值趋势图」
- 默认账号密码改为环境变量配置；移除仓库内 `.env.example`（改用 `.env` / `.env.production`）
- 升级 Go 至 1.26，迁移脚本累计至 000017

### 生产部署体系（2026-07-08 ~ 07-11）
- 新增 `docker-compose.prod.yml`（PostgreSQL + 后端镜像）与 `deploy.sh` 一键部署脚本
- 服务器初始化脚本：`scripts/setup-server.sh`、`setup-tailscale.sh`、`swap-cf-tunnel.sh`
- 远程部署执行器 `scripts/deploy-remote.sh`（支持 `--rollback` 回滚）
- 可选 `nginx/` 反向代理配置（/api、/static 反代 + SPA 回退）
- GitHub Actions：`ci.yml`（gofmt / vet / golangci-lint / 单测 / 安全扫描 / 迁移校验）+ `cd.yml`（构建推送 `ghcr.io` → Tailscale SSH 部署 → 健康检查 → 失败自动回滚）
- 前端接入 Cloudflare Pages 自动部署（配置见 `frontend/wrangler.jsonc`）
- 优化 CORS 配置与后端自检逻辑

## 许可证

本项目为**和润天下人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
