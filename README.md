# 叉车维修培训与残值评估系统

一套面向叉车维修培训与叉车残值评估的全栈系统，包含在线培训、考试练习、AI 助手，以及叉车残值评估与电池剩余寿命（RUL）评估等业务模块。系统按角色（学员 / 讲师 / 管理员）划分权限，并提供独立的残值评估工作区与 AI 助手工作区。

> 接口文档：完整的 HTTP 接口清单见 [API.md](API.md)（路径 / 方法 / 鉴权 / 说明）。

## 功能特性

### 培训学习模块

- **课程管理**：课程目录（专业方向 × 课程等级）、课程 CRUD、章节内容编排、PPT / 视频 / 图文混排
- **PPT 自动转图**：PPT 上传后由 LibreOffice sidecar 转换为 WebP 幻灯片并持久化到章节
- **考试系统**：课程考试、模拟考试、定级考试，自动判分、AI 评分与成绩统计
- **练习中心**：顺序练习（断点续练）、标签练习（断点续练）、自由/专项练习、错题本、练习统计
- **AI 助手**：基于大模型的流式对话（默认 DeepSeek，可切换管理员配置或用户自定义 OpenAI 兼容模型）
- **学员论坛**：独立的综合讨论区 + 每个章节内容下方的章节讨论区，发帖 / 回复（可回复别人的回复）/ 删除自己的内容，展示昵称与头像；**图文分离发图**——发帖/回复可附带图片（主题最多 9 张、回复最多 3 张，先传图后提交，支持粘贴），删除时图片一并清理，未发帖的悬空图片由定时任务回收

### 门户与内容

- **官网门户**：首页、内容详情页
- **精选内容**：官网精选内容板块（后台可配置、按类型展示）

### 残值评估模块

- **叉车残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域等参数，输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **电池 RUL 评估**：基于特征提取与混合神经网络的锂电池剩余寿命预测（含 SOH、置信区间与建议）
- **PDF 报告**：评估结果一键生成可下载的中文 PDF 报告
- **管理后台**：原价表配置、算法参数调整、系数表管理

### 账号体系

- **统一账号（hrwai_users）**：学员端 / 残值评估 / AI 助手共用一张用户表、一套 JWT（角色 `hrwai_user`），支持用户名或手机号登录
- **邮箱注册 / 登录**：支持邮箱注册与验证码登录，校验邮箱格式与唯一性（同一邮箱只能注册一个账户），验证码通过 SMTP 发送（开发环境未配置 SMTP 时降级为日志打印）
- **自定义资料（审核制）**：用户可提交昵称与头像修改（图片自动转 WebP，存储走 local/R2 统一抽象），由管理员在后台「资料审核」通过后生效；驳回可填写原因
- **讲师 / 管理员**：独立账号表（`tutor` / `admin`），管理员后台可统一管理 hrwai 用户

### 移动端（学员 App）

- **uni-app x** 跨端应用（Android / iOS / H5 / 微信小程序）
- 当前落地功能：学员登录 / 注册、首页、学习仪表盘；其余模块为占位

## 系统架构

单仓库包含四个可独立部署的部分：**后端（Go）**、**前端（Vue3）**、**移动端（uni-app x）**、**LibreOffice sidecar（PPT/图片转换）**。前后端通过 HTTP API 通信。

### 子域名多工作区（前端）

前端为单一应用，按访问子域名切换工作区，路由守卫在不同子域名间整页跳转：

| 子域名 | 工作区 | 主要角色 | 功能 |
| --- | --- | --- | --- |
| `www`（根域名） | 官网门户（**独立 Nuxt 仓库 hrwai-portal 接管**，不在本仓库 SPA 内） | 访客 | 首页、内容详情 |
| `training.` | 培训学习 | 学员（hrwai_user） | 课程、考试、练习（含断点续练）、错题本、AI 助手 |
| `mentor.` | 导师工作区 | 讲师 | 课程管理、题库、阅卷 |
| `valuation.` | 残值评估 | 访客 / hrwai_user | 整机残值、电池 RUL、报告、估值登录注册 |
| `manage.` | 管理后台 | 管理员 | 学员 / 讲师 / 课程 / 题库 / 残值配置 / AI 配置 |

登录态通过**父域名 httpOnly Cookie**（`hrwai_token`）共享，任意子域名登录一次后，其他子域名自动保持登录；`localStorage` 中的 token 仅作兼容保留。

### 统一鉴权体系（后端）

后端采用**单一 JWT 体系**：

- 统一用户表 `hrwai_users`（由原 `student` 与 `valuation_users` 合并而来，见 baseline 迁移 `000001`），角色为 `hrwai_user`，覆盖培训学员端、残值评估与 AI 助手三个前端
- 讲师（`tutor`）与管理员（`admin`）保持独立表，登录接口分别为 `/api/auth/tutor-login`、`/api/auth/admin-login`
- 统一签发密钥 `JWT_SECRET_KEY`，登出令牌写入 Redis 黑名单
- `VALUATION_JWT_SECRET_KEY` **已移除**：不再参与鉴权，统一使用 `JWT_SECRET_KEY`，代码与部署配置均已清理

统一响应结构：`{ "code": 0, "message": "...", "data": ... }`（code=0 表示成功）。健康检查 `/api/health` 除外（返回 `{"status":"ok",...}`，Redis 不可达时返回 503）。

健康检查分两个端点：`/api/health/live`（存活探针，仅校验进程存活，供容器编排使用）与 `/api/health`（就绪探针，额外探测 Redis 连通性）。

### 文件与转换链路

- **存储抽象**：`storage.Storage` 接口，`STORAGE_DRIVER=local` 写本地磁盘，`r2` 写 Cloudflare R2（S3 兼容），上传、PDF 报告、PPT 转图统一走该接口
- **LibreOffice sidecar**：独立 Flask 容器，通过 HTTP multipart 提供 `PPT → PDF → PNG → WebP` 与 `图片 → WebP` 转换，不再与 backend 共享 volume
- **图片压缩**：jpg/png/bmp/webp/tiff 上传自动转 WebP（质量 85），svg/gif 跳过；转换失败回退原格式

## 技术栈

### 后端（backend）

- 语言：Go 1.26
- Web 框架：Gin v1.10 + gin-contrib/cors
- 数据库：PostgreSQL 15 + GORM（主业务）+ pgx/v5（残值子模块手写 SQL）。注：残值评估子模块独立使用 pgx 直连池（`internal/valuation`），与主业务 GORM 双栈并存，互不依赖，保持现状不合并
- 缓存：Redis 7（go-redis/v9）
- 数据库迁移：golang-migrate/v4（up / down / force / status）
- 认证：golang-jwt/v5 + bcrypt（统一 JWT + Redis 黑名单）
- 限流：golang.org/x/time/rate（按客户端 IP 的 token bucket）
- 日志：zap（全栈统一，2026-08 #71 起 slog 退役）
- PDF 生成：gofpdf（中文 SimHei 字体）
- AI 集成：cloudwego/eino（流式对话）+ OpenAI 兼容客户端（sashabaranov/go-openai，评分/内容生成），默认 DeepSeek
- 对象存储：aws-sdk-go-v2（Cloudflare R2，S3 兼容 API）
- 测试库：glebarez/sqlite（单元测试用 SQLite）

### 前端（frontend）

- 框架：Vue 3.4 + TypeScript 5.7
- 构建：Vite 6
- UI：Element Plus 2.5 + @element-plus/icons-vue
- 状态管理：Pinia 2.1（auth / course / user / aiAssistant / valuationBattery / valuationEvaluation）
- 路由：vue-router 4（子域名多工作区 + 统一鉴权守卫）
- HTTP：axios（统一注入 Bearer Token、解包 `{code,message,data}`、`X-Silent` 静默模式）
- 图表：ECharts 6（DimensionRadar / FutureValueChart / BatteryRadar）
- 其他：dayjs、marked + highlight.js、pdfjs-dist、vditor（Markdown 编辑器，本地 CDN）、vuedraggable

### 移动端（training-app）

- 框架：uni-app x（Vue 3 + UTS / uvue）
- 工具：HBuilderX（运行 / 发行）
- 跨端目标：Android、iOS、H5、微信小程序
- 网络：uni.request 封装，Bearer Token 认证，依赖独立后端 `:8080`

### LibreOffice sidecar（libreoffice-sidecar）

- Python 3.12 + Flask + Pillow，LibreOffice + poppler-utils + 中文字体
- 接口：`POST /convert`（PPT→WebP 列表）、`POST /convert-image`（图片→WebP）、`GET /health`
- 单 worker + 进程内锁（LibreOffice 不支持并发），默认监听容器内 8000

### 基础设施

- 数据库：PostgreSQL 15
- 缓存：Redis 7（生产必需，用于 JWT 黑名单与缓存）
- 编排：Docker Compose（本地与生产）
- 反向代理 / 静态托管：Nginx（前端容器兼任 SSL 终止 + API 反代，PVE 环境使用 host 网络模式监听 51820）；`www` 主站请求由官网门户（独立 Nuxt 仓库）部署时注入的 `~^www\..+$` 分流块转发到门户容器（:3000），本仓库 SPA 只服务业务子域名
- CI/CD：GitHub Actions（ci.yml / cd.yml），镜像推送 ghcr.io，SSH（公网跳板 + ProxyJump）部署到自托管服务器
- 备选：Cloudflare Pages（frontend/wrangler.jsonc）

## 项目结构

```
叉车维修项目/
├── backend/                      # Go 后端（module: forklift-training）
│   ├── cmd/
│   │   ├── server/               # 服务入口（默认 :8080，启动时自动迁移 + 建默认账号）
│   │   └── migrate/              # 数据库迁移 CLI（up | down | force | status）
│   ├── internal/
│   │   ├── api/                  # 培训业务 Gin 路由与 handler
│   │   ├── service/              # 培训业务服务层（含 AI 助手 / AI 配置 / 文件服务）
│   │   ├── storage/              # 文件存储抽象（local / Cloudflare R2）
│   │   ├── model/ db/ config/ middleware/ cache/ migrate/ testutil/
│   │   └── valuation/            # 残值评估 + 电池 RUL 子模块（独立 handler/repository/service/config）
│   ├── pkg/
│   │   ├── response/             # 统一响应结构
│   │   └── pdf/                  # 中文 PDF 报告（gofpdf + SimHei）
│   ├── migrations/               # 迁移脚本（squash baseline 000001 + 增量 000002+）
│   ├── Dockerfile / entrypoint.sh / Makefile
│   ├── docker-compose.yml        # 本地 postgres + redis + libreoffice
│   ├── .env                      # 本地开发环境变量（gitignore，不入库）
│   └── init-db.sql
├── frontend/                     # Vue3 前端（forklift-training-frontend）
│   ├── src/                      # 源码（api/pages/components/stores/router/utils）
│   ├── Dockerfile / docker-entrypoint.sh
│   ├── nginx.default.conf / nginx-host.conf / nginx.local.conf 等
│   ├── wrangler.jsonc            # Cloudflare Pages 备选部署
│   └── .env.example
├── training-app/
│   └── 叉车维修培训学员端跨端应用/  # uni-app x 学员端移动 App
├── libreoffice-sidecar/          # PPT/图片转换服务（Flask + LibreOffice）
│   ├── app.py / Dockerfile / requirements.txt
├── scripts/
│   ├── deploy-remote.sh          # 服务器远程部署（支持 --rollback）
│   ├── setup-server.sh           # 服务器初始化
│   ├── lxc-install-docker.sh     # LXC 容器 Docker 安装（PVE 环境）
│   └── lxc-setup-ssh.sh          # LXC SSH 配置（PVE 环境）
├── nginx/                        # 宿主机 Nginx 参考配置（www 分流到门户、业务子域名反代）
├── .github/workflows/            # CI/CD（ci.yml / cd.yml）
├── docs/                         # 本地协作文档（不入库；ADR-0001~0008 例外，已版本化）
├── API.md                        # HTTP 接口清单（路径 / 方法 / 鉴权 / 说明）
├── deploy.sh                     # 本地 / 手动一键部署
└── docker-compose.prod.yml       # 生产编排（PostgreSQL + Redis + LibreOffice + 后端 + Nginx 前端）
```

## 环境要求

- Go ≥ 1.26
- Node.js ≥ 18（推荐 20+）
- Docker + Docker Compose（本地 PostgreSQL / Redis / LibreOffice，以及生产编排）
- PostgreSQL 15（由 docker-compose 提供或已有实例）
- Redis 7（生产必需；本地开发用于 JWT 黑名单与缓存）
- HBuilderX（仅移动端开发需要）

## 快速开始

### 1. 启动后端依赖（PostgreSQL + Redis + LibreOffice）

```bash
cd backend
docker compose up -d          # 启动 postgres + redis + libreoffice 容器
```

### 2. 配置环境变量

本地开发在 `backend/` 下创建 `.env`（当前文件不入库，可参考 `backend/docker-compose.yml` 与 `docker-compose.prod.yml` 中的变量名填写，至少配置 `DATABASE_URL` 与 `JWT_SECRET_KEY`）。

### 3. 执行迁移并启动后端

```bash
cd backend
make migrate-up               # 执行数据库迁移（亦可 make dev 一步到位）
make run                      # 启动服务，默认 :8080
```

启动后会自动加载 `.env`、连接数据库并执行迁移、创建默认账号。健康检查：

```bash
curl http://localhost:8080/api/health
# {"status":"ok","message":"backend is running"}
# Redis 不可达时返回 503：{"status":"degraded","redis":"unreachable",...}
```

### 4. 启动前端

```bash
cd frontend
cp .env.example .env.local    # 按需修改子域名 / API 地址
npm install
npm run dev                   # 默认 :5173
```

开发服务器已配置代理：`/api` 与 `/static` 请求转发至 `http://127.0.0.1:8080`。本地多子域名开发需在 `hosts` 添加：

```
127.0.0.1 training.localhost valuation.localhost mentor.localhost manage.localhost
```

### 5. 移动端（可选）

用 **HBuilderX** 打开 `training-app/叉车维修培训学员端跨端应用`，通过「运行」选择 Android / iOS / H5 / 微信小程序。运行前需先把 `config/env.uts` 中的后端地址改为可达的 `:8080` 地址。

### 6. 访问系统

- 前端（开发）：<http://localhost:5173>（或各子域名）
- 后端 API：<http://localhost:8080/api>

## 默认账号

后端启动时会自动创建默认账号（管理员 / 讲师 / 学员），密码由环境变量配置：

| 角色 | 用户名 | 环境变量 |
| --- | --- | --- |
| 管理员 | admin | `ADMIN_DEFAULT_PASSWORD` |
| 讲师 | tutor | `TUTOR_DEFAULT_PASSWORD` |
| 学员 | student | `STUDENT_DEFAULT_PASSWORD` |

残值评估与 AI 助手使用同一 `hrwai_users` 体系，可通过 `/api/auth/register`（或 `/api/valuation/auth/register`）自行注册。

## 配置说明

后端配置通过环境变量注入（非 production 环境自动读取 `backend/.env`）。生产环境（`APP_ENV=production`）会校验必填项（`SECRET_KEY`、`JWT_SECRET_KEY`、`DATABASE_URL`、`REDIS_ADDR`、`CORS_ORIGINS`、默认密码），缺失或仍为开发默认值时启动失败。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ENV` | 运行环境 | development |
| `PORT` | 服务端口 | 8080 |
| `DATABASE_URL` | PostgreSQL 连接串 | 由 docker-compose 提供 |
| `SECRET_KEY` | 应用密钥（生产必改） | dev-secret-key |
| `JWT_SECRET_KEY` | JWT 签名密钥，全系统统一（生产必改） | jwt-secret-key |
| `JWT_EXPIRES_HOURS` | JWT 有效期（小时） | 24 |
| `AUTH_COOKIE_NAME` / `AUTH_COOKIE_DOMAIN` / `AUTH_COOKIE_SECURE` | 登录态 Cookie 名称 / 父域名（子域名共享必需，生产如 example.com）/ 仅 HTTPS 发送（生产默认 true） | hrwai_token / localhost / false |
| `REDIS_ADDR` / `REDIS_PASSWORD` / `REDIS_DB` / `REDIS_POOL_SIZE` / `REDIS_KEY_PREFIX` | Redis 地址 / 密码 / 库 / 连接池 / 键前缀 | localhost:6379 / 空 / 0 / 10 / fl: |
| `CORS_ORIGINS` | 允许的前端来源（含五个子域名，逗号分隔） | 本地 localhost 系列 |
| `UPLOAD_FOLDER` / `VOLUME_MOUNT_PATH` | 上传目录 / 数据卷挂载路径 | static/uploads / 空 |
| `MAX_CONTENT_LENGTH_MB` | 上传大小上限 | 250 |
| `ADMIN_DEFAULT_PASSWORD` / `TUTOR_DEFAULT_PASSWORD` / `STUDENT_DEFAULT_PASSWORD` | 默认账号密码（生产必改） | admin123 / tutor123 / student123 |
| `RATE_LIMIT_ENABLED` / `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | 按 IP 限流开关 / 每秒请求 / 突发上限（生产默认开启） | true(prod) / 20 / 40 |
| `AI_API_KEY` / `AI_BASE_URL` / `AI_MODEL` | AI 供应商配置（OpenAI 兼容格式） | deepseek-v4-flash / https://api.deepseek.com |
| `LIBREOFFICE_SIDECAR_URL` | sidecar HTTP 地址；为空时降级为本地 exec 调用 | 空 |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` / `SMTP_FROM_NAME` | 邮箱验证码邮件 SMTP 配置（主机/端口/账号/密码/发件人/发件人名称） | 空 / 587 / 空 / 空 / 空 / 和润天下 |
| `EMAIL_CODE_TTL_MINUTES` | 邮箱验证码有效期（分钟） | 5 |
| `STORAGE_DRIVER` | 文件存储：`local` 本地磁盘 / `r2` Cloudflare R2 | local |
| `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` / `R2_PUBLIC_DOMAIN` | R2 凭证与公开域名（driver=r2 时必填） | 空 |
| `VALUATION_PDF_OUTPUT_DIR` | 评估报告 PDF 输出目录 | storage/reports |

AI 配置支持链式回退：`AI_API_KEY` > `DEEPSEEK_API_KEY` > `ZHIPU_API_KEY` > `OPENAI_API_KEY`；`AI_BASE_URL` 可回退 `DEEPSEEK_API_URL` / `ZHIPU_BASE_URL`；`AI_MODEL` 可回退 `MODEL` / `ZHIPU_MODEL`。此外管理员可在后台维护多套模型配置（`ai_configs` 表），按功能（AI 评分 / 内容生成 / AI 助手）绑定；用户可在 AI 助手内添加自己的 OpenAI 兼容模型（`ai_user_models` 表）。**存储安全**：所有 API Key 均使用 `SECRET_KEY` 派生的 AES-256-GCM 密钥加密后入库（`enc:v1:` 前缀），读取时自动解密，历史明文数据无缝兼容。

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
make dev-up          # 启动 postgres + redis + libreoffice 容器
make dev-down        # 停止容器
make dev-reset       # 清除数据卷并重建（数据丢失）
```

也可直接用 Go 运行迁移 CLI：

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down
go run ./cmd/migrate force <version>   # 修复 dirty 迁移状态
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

迁移脚本位于 `backend/migrations/`，采用 `序号_名称.up.sql` / `.down.sql` 成对组织。历史迁移（000001 ~ 000028）已 **squash 合并**为单一 baseline，新增迁移从 **000002** 开始编号：

| 迁移 | 说明 |
| --- | --- |
| `000001_init_baseline` | 全量 schema baseline（squash 000001~000029）：培训/题库/考试/练习/论坛/通知/审计/认证/精选内容/残值评估全部表 + 遗留 level 列删除 + FK 修复 |
| `000002_catalog_retirement` | 课程分类体系收敛：删除 `course.category` 列与 `knowledge_point` 表、`question.knowledge_point_id` 列，存量题目按显式映射回填题库标签（结构/液压/法规/故障诊断） |
| `000003_drop_question_tag_category` | 题库标签退役死字段 `category`（考点模块，全库无消费方） |

> 说明：squash 合并等价于原 29 个迁移按序执行后的最终态；`question_practice_record` 遗留 `level` 列已随合并删除（修复刷题提交 400）；各业务表 `student_id` 外键已指向 `hrwai_users`。

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

`docker-compose.prod.yml` 编排五个服务：

- `postgres`（PostgreSQL 15，持久化卷 `pgdata-prod`，默认不对外暴露端口；Docker 19.03 环境需保留 `seccomp:unconfined`）
- `redis`（Redis 7，仅内网，LRU 淘汰、禁用持久化）
- `libreoffice`（PPT/图片转换 sidecar，仅内网 8000）
- `backend`（Go 后端，仅内网，`127.0.0.1:${BACKEND_HOST_PORT:-8080}:8080` 映射到宿主机回环，供 host 网络模式的 Nginx 反代）
- `frontend`（Nginx，SSL 终止 + `/api` 反代 + 静态托管；PVE 环境使用 `network_mode: host` 监听 51820）

```bash
docker compose -f docker-compose.prod.yml up -d
```

### 远程部署（自托管服务器）

`scripts/deploy-remote.sh` 通过 SSH（公网跳板 + 可选 ProxyJump 进入 LXC）将 ghcr.io 镜像部署到服务器 `/opt/forklift-training`，支持 `--rollback` 回滚与异地备份同步。`scripts/setup-server.sh` 负责服务器初始化；`lxc-install-docker.sh` / `lxc-setup-ssh.sh` 用于 PVE LXC 测试环境。

### CI/CD（GitHub Actions）

- `ci.yml`：路径过滤 → gofmt / go vet / golangci-lint → 测试（race + cover，真实 PG + Redis）→ 前端 type-check + build → 安全扫描
- `cd.yml`：由 CI 主动 `workflow_dispatch` 触发（携带已验证 commit SHA），可选 `production` / `testing` 环境与跳过开关；构建并推送 backend / frontend / libreoffice 三个镜像到 ghcr.io → SSH 部署 → 健康检查 → 失败自动回滚；支持 Tailscale/ProxyJump 与异地备份

## 许可证

本项目为**和润天下人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
