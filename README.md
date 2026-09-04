# 叉车维修培训与残值评估系统

面向叉车维修培训、学员就业对接与叉车残值评估的全栈系统。包含在线培训与考试练习、论坛问答、学员资料投稿、积分激励、AI 助手、企业招聘对接，以及叉车残值评估与电池剩余寿命（RUL）评估等模块。系统按角色划分工作区（学员 / 讲师 / 管理员 / 企业招聘者），前端以子域名承载独立工作区。

领域术语以 [`CONTEXT.md`](./CONTEXT.md) 为准，架构决策记录在 [`docs/adr/`](./docs/adr/)（ADR-0001 ~ ADR-0026）。

## 功能特性

### 培训学习（学员 · `training.`）

- **目标证件**：学员报考的外部持证目标（特种作业上岗证 / 职业技能等级），作为**全局过滤器**贯穿课程、题库、练习、模考、错题、搜索、收藏与投稿；首次登录强制预筛选
- **课程目录**：`目标证件 → 专业方向 → 课程等级 → 课程` 三层虚拟树；章节支持 PPT / 视频 / 图文混排（PPT 经 LibreOffice sidecar 转 WebP）
- **学习位置**：记录最后章节与视频播放位置，断点续学；章节完成以 progress ≥ 100 为单一事实源
- **练习**：顺序 / 随机 / 专项 / 标签四种模式，含结果卡、AI 解析、评论、考点与私有笔记五模块
- **模拟考试**：自动判分 + AI 评分（简答），按当前证件题库抽题
- **真题卷**：按套解锁的真题练习，真题题不进公共练习池
- **错题本**：按题收录，单题即时重做，与练习共用提交管线
- **论坛**：讨论与问答双模式，采纳奖励、点赞、举报、楼中楼回复
- **资料投稿**：学员自主上传（1–5 个文件，先审后发）换取积分；过审 +50、下载达阶追加，违规下架走积分对冲追回（ADR-0026）
- **积分域**：统一账务（余额 + 流水 + 幂等占坑）。任务中心、问答采纳、AI 按 token 计费、兑换与管理员扣罚共用一套簿记（ADR-0023）
- **在线简历**：学员维护简历，可导出 PDF，向企业投递并查看进度
- **AI 助手**：大模型流式对话（DeepSeek 默认，OpenAI 兼容可换），按 tokens 计量扣积分
- **全局搜索 / 收藏**：course / question / content / topic 四类聚合；多态收藏

### 招聘对接（企业招聘者 · `recruit.` / 学员端职位广场）

- **职位发布与广场**：企业发布职位并挂专业方向，学员端按地区 / 方向筛选浏览
- **投递与授权**：投递即授权简历可见（ADR-0025），企业侧查看投递列表与简历详情
- **简历库与交换**：企业检索简历、发起联系请求，双向交换联系方式
- **招聘者管理**：授信码开通、资料编辑、微信绑定、查看统计

### 残值评估（学员 `hrwai_user` · `valuation.`）

- **整机残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域等参数，输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **电池 RUL 评估**：基于特征提取与混合神经网络的锂电池剩余寿命预测（含 SOH、置信区间与建议）
- **PDF 报告**：评估结果一键生成可下载的中文 PDF 报告

### 管理后台（管理员 · `manage.`）

- 学员 / 讲师 / 招聘者账号管理、目标证件与专业方向维护
- 课程目录与章节、题库审核、标签管理
- 投稿审核、论坛管理、资料审核、精选内容、职位管理、统计看板
- AI 设置、内容生成、审计日志、系统巡检、残值算法配置

### 讲师工作区（`mentor.`）

- 章节内容编排与编辑、题库创建 / 维护 / 驳回、题库标签、投稿审核

### 移动端（学员 App）

- **uni-app x** 跨端应用（Android / iOS / H5 / 微信小程序）
- 已落地：登录（含微信小程序 `wx-login`）、首页、课程、考试、我的；其余模块为占位

## 系统架构

单仓库包含三个可独立部署的部分：**后端（Go）**、**前端（Vue3）**、**移动端（uni-app x）**。前后端通过 HTTP API 通信，接口清单见 [`API.md`](./API.md)，Swagger 文档由后端 `/swagger` 提供。

### 子域名多工作区（前端）

前端为单一应用，按访问子域名切换工作区，路由守卫在不同子域名间整页跳转：

| 子域名 | 工作区 | 角色 | 主要内容 |
| --- | --- | --- | --- |
| `www` | 官网门户 | 访客 | 由**独立 Nuxt 仓库**承载（本仓库不再含门户页面） |
| `training.` | 培训学习 | 学员 `hrwai_user` | 课程、练习、模考、真题、论坛、投稿、积分、AI 助手、简历、职位广场 |
| `mentor.` | 讲师工作区 | 讲师 `tutor` | 章节、题库、标签、投稿审核 |
| `manage.` | 管理后台 | 管理员 `admin` | 全站管理与配置 |
| `valuation.` | 残值评估 | 学员 `hrwai_user` | 整机残值、电池 RUL、评估报告 |
| `recruit.` | 招聘工作区 | 企业招聘者 `recruiter` | 职位发布、简历库、投递管理 |

生产环境登录态通过**父域名 httpOnly Cookie**（`hrwai_token` / `recruiter_token`）在子域名间共享；Cookie 不可用时降级为带一次性 `auth_token` 参数的跨子域名跳转交接。IP 直连模式（无 DNS 子域名）下不做跨域跳转，各工作区按路径在同一 origin 内访问。

### 统一账号与双令牌（后端）

- **统一账号**：`hrwai_users` 表 + 一套 JWT，学员端 / 残值评估 / AI 助手共用（历史上学员与估值用户的双 JWT 体系已收敛）
- **角色**：`hrwai_user`、`tutor`、`admin`、`recruiter`，后三者为独立账号表
- **双令牌（ADR-0016）**：access 2 小时（中间件仅收 access）+ refresh 7 天轮换；Redis 黑名单只管 refresh，access 自然过期
- **登录方式**：密码、邮箱验证码、手机验证码、微信扫码（开放平台）、微信小程序 `wx-login`
- **验证码（ADR-0001）**：邮箱（SMTP）与短信（腾讯云 SMS）是同一状态机两侧的 adapter，六态用途，默认 5 分钟内有效、60 秒节流、错误上限 5 次

统一响应结构：`{ "code": <HTTP状态码>, "message": "...", "data": ... }`（`code` 即 HTTP 状态码，`200` 表示成功，见 ADR-0005）。

### 文件与对象存储

`storage_driver` 可切换 `local` / `r2`（S3 兼容）。生产环境经 Nginx 把 `featured/ avatars/ contributions/ resumes/ images/ chapters/ slides/ uploads/ reports/` 等前缀反代到 S3 兼容对象存储网关，避免直链落到门户兜底路由导致 404。

## 技术栈

### 后端（backend）

- 语言：Go 1.26
- Web 框架：Gin v1.10 + gin-contrib/cors
- 数据库：PostgreSQL 15 + pgx/v5 + GORM
- 缓存 / 黑名单：Redis 7（go-redis/v9）
- 数据库迁移：golang-migrate/v4
- 认证：golang-jwt/v5（双令牌）
- 配置：viper（环境变量 + 默认值）
- 日志：zap + lumberjack（轮转）
- 对象存储：aws-sdk-go-v2 S3
- PDF 生成：gofpdf（中文 SimHei 字体）；PPT/文档转换走 LibreOffice sidecar
- AI：CloudWeGo Eino + OpenAI 兼容接口（DeepSeek 默认）+ go-openai
- 限流：golang.org/x/time
- 文档：swaggo/gin-swagger
- 测试：glebarez/sqlite（单元测试用 SQLite）

### 前端（frontend）

- 框架：Vue 3.4 + TypeScript 5.7
- 构建：Vite 6
- UI：Element Plus 2.5 + @element-plus/icons-vue + Tailwind CSS v4（与既有 scoped 样式增量共存）
- 状态管理：Pinia 2.1
- 路由：vue-router 4（子域名多工作区 + 角色守卫）
- HTTP：axios（access 过期自动刷新重试）
- 图表：ECharts 6（DimensionRadar / FutureValueChart / BatteryRadar）
- 其他：dayjs、marked + marked-highlight + highlight.js、markstream-vue、pdfjs-dist、vditor、vuedraggable、element-china-area-data
- 测试：vitest 4 + @vue/test-utils + happy-dom

### 移动端（training-app）

- 框架：uni-app x（Vue 3 + UTS / uvue）
- 工具：HBuilderX（运行 / 发行）；已配 jest 单测
- 跨端目标：Android、iOS、H5、微信小程序

### 基础设施

- 数据库：PostgreSQL 15；缓存：Redis 7（生产必需）
- 对象存储：S3 兼容网关（Ceph RGW）
- 文档转换：LibreOffice sidecar（`libreoffice-sidecar/`）
- 编排：Docker Compose
- 反向代理 / 静态托管：Nginx（host-network 监听 80/443，SSL 终止 + `/api` 反代 + 静态托管 + 对象存储反代）
- CI/CD：GitHub Actions（`ci.yml` / `cd.yml` / `testing-smoke.yml`），公网 SSH 部署到自托管服务器
- 备选：Cloudflare Pages（`frontend/wrangler.jsonc`）

## 项目结构

```
叉车维修项目/
├── backend/                      # Go 后端（module: forklift-training）
│   ├── cmd/
│   │   ├── server/               # 服务入口（默认 :8080，启动自动迁移 + 建默认账号）
│   │   ├── migrate/              # 数据库迁移 CLI（up | down | version）
│   │   ├── import-reference-content/  # 参考资料导入工具
│   │   └── backfill-evaluation-suggestions/  # 评估建议回填工具
│   ├── internal/
│   │   ├── api/                  # 培训 / 招聘 / 投稿 / 积分等 Gin 路由与 handler
│   │   ├── service/ model/ repository/ db/
│   │   ├── config/ logger/ middleware/ cache/ captcha/ clock/
│   │   ├── storage/              # 存储驱动（local / S3 兼容）
│   │   ├── security/ daemon/ pdfutil/ migrate/ testutil/
│   │   └── valuation/            # 残值评估 + 电池 RUL 子模块（独立 handler/repository/service/config）
│   ├── pkg/{paging,response}/    # 分页与统一响应
│   ├── migrations/               # 迁移脚本（000001 ~ 000020，共 20 组）
│   ├── docs/                     # Swagger 生成产物（docs.go / swagger.json|yaml）
│   ├── Dockerfile / Makefile
│   ├── docker-compose.yml        # 本地 postgres + redis + libreoffice + backend
│   ├── docker-compose.local.yml / docker-compose.override.yml.example
│   └── init-db.sql
├── frontend/                     # Vue3 前端（forklift-training-frontend）
│   ├── src/                      # 源码（api / pages / components / layouts / stores / router /
│   │                             #       composables / utils / config / types / constants / assets / icons）
│   ├── Dockerfile
│   ├── nginx-host.conf           # 生产 host 网络模式 Nginx（SSL + 反代 + 对象存储分流）
│   ├── nginx.default.conf        # 镜像内置 Nginx 配置（template，envsubst 渲染）
│   ├── docker-entrypoint.sh
│   ├── wrangler.jsonc            # Cloudflare Pages 备选部署
│   └── .env.example
├── training-app/
│   └── 叉车维修培训学员端跨端应用/  # uni-app x 学员端移动 App
├── libreoffice-sidecar/          # 文档转换服务镜像（PPT → WebP、简历 PDF）
├── viz/                          # 设计提案 / 设计系统静态原型（HTML，仅参考）
├── scripts/
│   ├── deploy-remote.sh          # 服务器远程部署（支持 --rollback）
│   ├── setup-server.sh           # 服务器初始化
│   ├── lxc-install-docker.sh / lxc-setup-ssh.sh   # LXC 容器初始化
│   └── backup-daily.sh / rbd-snap-hourly.sh       # 定时备份与快照
├── docs/                         # 协作文档（.gitignore 忽略 docs/*，仅 adr/ 入库）
│   ├── adr/                      # 架构决策记录（ADR-0001 ~ ADR-0026，入库）
│   └── plans/ reference/ archive/ agents/   # 方案、参考资料、归档、AI 约定（本地）
├── .github/workflows/            # CI/CD（ci.yml / cd.yml / testing-smoke.yml）
├── deploy.sh                     # 本地 / 手动一键部署
└── docker-compose.prod.yml       # 生产编排（PostgreSQL + Redis + LibreOffice + 后端 + Nginx 前端）
```

## 环境要求

- Go ≥ 1.26
- Node.js ≥ 18（推荐 20+）
- Docker + Docker Compose（本地依赖与生产编排）
- PostgreSQL 15、Redis 7（由 docker-compose 提供或已有实例）
- HBuilderX（仅移动端开发需要）

## 快速开始

### 1. 启动后端依赖（PostgreSQL + Redis + LibreOffice）

```bash
cd backend
docker compose up -d          # postgres :5432 / redis :6379 / libreoffice :8000 / backend :18080
```

`backend/docker-compose.yml` 把后端容器的 8080 映射到宿主机 **18080**，前端 dev 代理默认指向 `127.0.0.1:18080`。

### 2. 执行迁移并启动后端

```bash
cd backend
make migrate-up               # 执行数据库迁移（亦可 make dev 一步到位）
make run                      # 启动服务，默认 :8080
```

启动后会自动加载配置、连接数据库并执行迁移、创建默认账号。健康检查：

```bash
curl http://localhost:8080/api/health
# {"code":200,"message":"success","data":{...}}
```

Swagger（非生产环境默认开启）：<http://localhost:8080/swagger/index.html>

### 3. 启动前端

```bash
cd frontend
cp .env.example .env.local    # 按需修改子域名
npm install
npm run dev                   # 默认 :5173
```

开发服务器已配置代理：`/api` 与 `/static` 转发至 `http://127.0.0.1:18080`。本地多子域名开发需在 `hosts` 添加：

```
127.0.0.1 training.localhost valuation.localhost mentor.localhost manage.localhost recruit.localhost
```

### 4. 移动端（可选）

用 **HBuilderX** 打开 `training-app/叉车维修培训学员端跨端应用`，通过「运行」选择 Android / iOS / H5 / 微信小程序。运行前需把 `config/env.uts` 中的后端地址改为可达地址。

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

## 配置说明

后端配置全部通过环境变量注入（viper AutomaticEnv + 集中默认值），非 production 环境自动读取 `backend/.env`。生产环境（`APP_ENV=production`）会校验必填项与弱口令，缺失或仍为默认值则启动失败。

### 核心

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ENV` | 运行环境 | development |
| `PORT` | 服务端口 | 8080 |
| `SECRET_KEY` | 应用密钥（生产必改） | dev-secret-key |
| `DATABASE_URL` | PostgreSQL 连接串 | 空（生产必填） |
| `JWT_SECRET_KEY` | JWT 签名密钥（生产必改） | jwt-secret-key |
| `JWT_EXPIRES_HOURS` | access 令牌有效期（小时） | 2 |
| `JWT_REFRESH_EXPIRES_DAYS` | refresh 令牌有效期（天） | 7 |
| `CORS_ORIGINS` | 允许的前端来源（含各子域名，逗号分隔） | <http://localhost:5173> |
| `MAX_CONTENT_LENGTH_MB` | 上传大小上限 | 250 |

### Redis

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `REDIS_ADDR` | Redis 地址 | localhost:6379 |
| `REDIS_PASSWORD` | Redis 密码 | 空 |
| `REDIS_DB` / `REDIS_POOL_SIZE` / `REDIS_MIN_IDLE_CONNS` | 库 / 连接池 / 最小空闲 | 0 / 20 / 5 |
| `REDIS_KEY_PREFIX` | 键前缀 | fl: |
| `REDIS_MAX_RETRIES` / `REDIS_POOL_TIMEOUT` / `REDIS_IDLE_TIMEOUT` | 重试 / 池超时 / 空闲超时 | 3 / 3s / 5m |

### 存储与文档转换

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `STORAGE_DRIVER` | `local` 或 `r2`（S3 兼容） | local |
| `UPLOAD_FOLDER` | 本地上传目录 | 空 |
| `VOLUME_MOUNT_PATH` | 容器卷挂载根路径 | 空 |
| `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` / `R2_PUBLIC_DOMAIN` | S3 兼容对象存储凭据 | 空 |
| `LIBREOFFICE_SIDECAR_URL` | LibreOffice 转换服务地址 | 空 |

### 认证与通知通道

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `AUTH_COOKIE_NAME` / `AUTH_COOKIE_DOMAIN` | 学员登录态 Cookie 名与父域名（生产必填父域名） | hrwai_token / localhost |
| `RECRUITER_COOKIE_NAME` / `RECRUITER_COOKIE_DOMAIN` | 招聘者登录态 Cookie | recruiter_token / 空 |
| `CAPTCHA_ENABLED` | 图形验证码开关 | 生产默认开 |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` / `SMTP_FROM_NAME` | 邮件验证码通道 | 587 / 和润天下 |
| `EMAIL_CODE_TTL_MINUTES` | 验证码有效期（分钟） | 5 |
| `TENCENT_SMS_SECRET_ID` / `TENCENT_SMS_SECRET_KEY` / `TENCENT_SMS_SDK_APP_ID` / `TENCENT_SMS_SIGN_NAME` / `TENCENT_SMS_REGION` | 短信验证码通道 | ap-guangzhou |
| `TENCENT_SMS_TEMPLATE_REGISTER` / `_LOGIN` / `_PASSWORD` / `_BIND_PHONE` | 短信模板 ID | 空 |
| `WECHAT_MINI_PROGRAM_APP_ID` / `_SECRET` | 微信小程序登录 | 空 |
| `WECHAT_OPEN_PLATFORM_APP_ID` / `_SECRET` | 网页端微信扫码登录（与小程序凭证不可混用） | 空 |

### AI、限流、日志与文档

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `AI_BASE_URL` / `AI_MODEL` | OpenAI 兼容模型接入 | <https://api.deepseek.com> / deepseek-v4-flash |
| `RATE_LIMIT_ENABLED` / `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | 限流开关与速率 | 生产开 / 20 / 40 |
| `LOG_LEVEL` / `LOG_FORMAT` / `LOG_DIR` | 日志级别 / 格式 / 目录 | info / console / 空 |
| `LOG_MAX_SIZE_MB` / `LOG_MAX_BACKUPS` / `LOG_MAX_AGE_DAYS` / `LOG_COMPRESS` | 日志轮转 | 100 / 7 / 30 / true |
| `SWAGGER_ENABLED` / `SWAGGER_USER` / `SWAGGER_PASS` | Swagger 开关与 Basic 认证 | 非生产开 / 空 |
| `VALUATION_PDF_OUTPUT_DIR` | 评估报告 PDF 输出目录 | storage/reports |
| `VALUATION_DB_MAX_OPEN_CONNS` / `_IDLE_CONNS` / `_CONN_MAX_LIFETIME` | 残值库连接池 | 20 / 5 / 3600 |

### 前端环境变量（`.env.example`）

- `VITE_MAIN_DOMAIN`
- `VITE_TRAINING_SUBDOMAIN` / `VITE_VALUATION_SUBDOMAIN` / `VITE_MENTOR_SUBDOMAIN` / `VITE_ADMIN_SUBDOMAIN`

生产环境通过 DNS A 记录把各子域名指向同一服务器 IP。API 地址用 `VITE_API_BASE_URL`（缺省 `/api`，开发经 Vite 代理转发）。

## 常用命令

### 后端（在 `backend/` 下）

```bash
make build           # 编译二进制到 bin/server
make run             # go run ./cmd/server
make test            # 运行测试（-race -cover）
make lint            # 代码检查（golangci-lint）
make fmt / tidy      # 格式化 / 整理依赖
make swagger         # 重新生成 Swagger 文档
make migrate-up      # 执行数据库迁移
make migrate-down    # 回滚最近一次迁移
make dev             # dev-up + migrate-up + run（一步到位）
make dev-up / dev-down / dev-reset   # 起停依赖容器 / 清卷重建（数据丢失）
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
npm test             # vitest 单元测试
```

### 移动端（在 `training-app/叉车维修培训学员端跨端应用/` 下）

通过 **HBuilderX** 打开目录，使用菜单「运行 / 发行」选择目标平台。

## 数据库迁移

迁移脚本位于 `backend/migrations/`，采用 `序号_名称.up.sql` / `.down.sql` 成对组织，当前共 **20 组**：

| 序号 | 名称 | 说明 |
| --- | --- | --- |
| 000001 | baseline | 培训库基线 |
| 000002 | points_system | 积分域 |
| 000003 | forum_browse_dedup | 论坛浏览去重 |
| 000004 | real_exam_paper | 真题套卷 |
| 000005 | forum_topic_category | 论坛分类 |
| 000006 | forum_accept | 问答采纳 |
| 000007 | recruiter_users | 招聘者账号 |
| 000008 | job_cards | 职位卡 |
| 000009 | recruit_resume_views | 简历浏览 |
| 000010 | contact_requests | 联系请求 |
| 000011 | points_entry_idem | 积分幂等占坑 |
| 000012 | points_claim_lifetime_index | 任务领取索引 |
| 000013 | practice_progress_credential | 练习进度挂证件 |
| 000014 | recruiter_credit_code_unique | 授信码唯一 |
| 000015 | job_postings | 职位发布 |
| 000016 | positions | 岗位字典 |
| 000017 | region_city_level | 地区城市层级 |
| 000018 | recruiter_wechat | 招聘者微信绑定 |
| 000019 | practice_progress_orphan_merge | 孤立进度合并 |
| 000020 | contribution | 学员资料投稿 |

执行 / 回滚：`make migrate-up` / `make migrate-down`。

## 测试与检查

改动后**必须**跑完对应栈的检查，全绿才能提交：

- **后端（`backend/`）**：Go 工具链在 `~/go/bin`
  - `gofmt -l .`（应无输出）
  - `go vet ./...`
  - `golangci-lint run ./...`
  - `go test ./...`
- **前端（`frontend/`）**
  - `npm run type-check`（vue-tsc）
  - `npm test`（vitest）
- **部署配置**：改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验
- **安全检测**：改动触及认证 / 授权 / 密钥 / DB 连接 / AI 生成代码时，跑 DeepSec（Shield）扫描，确认无新增 critical/high

## 部署

### 本地 / 手动一键部署

```bash
./deploy.sh                      # 交互式确认后部署到生产
./deploy.sh production           # 直接部署到生产
./deploy.sh production --skip-build     # 跳过构建步骤（仅重启）
./deploy.sh production --skip-migrate   # 跳过数据库迁移
```

脚本流程：环境检查 → 加载 `.env` → 构建前端 → 数据库迁移 → `docker compose` 部署 → 健康检查。当前脚本仅支持 `production` 一个环境（testing 由 CD 流水线自动部署并探活后停容器）。

### 生产编排

`docker-compose.prod.yml` 编排五个服务：

- `postgres`（PostgreSQL 15，持久化卷 `pgdata-prod`）
- `redis`（Redis 7，仅内网，LRU 淘汰、禁用持久化）
- `libreoffice`（文档转换 sidecar）
- `backend`（Go 后端，仅内网，经前端反代访问）
- `frontend`（Nginx，host 网络监听 80/443：SSL 终止 + `/api` 反代 + 静态托管 + 对象存储前缀反代）

```bash
docker compose -f docker-compose.prod.yml up -d
```

### 远程部署（自托管服务器）

`scripts/deploy-remote.sh` 通过公网 SSH 将构建产物部署到服务器 `/opt/forklift-training`，支持 `--rollback` 回滚到上一个版本；备份默认落在 `/opt/forklift-backups`。配套脚本：`setup-server.sh`（初始化）、`lxc-install-docker.sh` / `lxc-setup-ssh.sh`（LXC 容器初始化）、`backup-daily.sh` / `rbd-snap-hourly.sh`（定时备份与快照）。

### CI/CD（GitHub Actions）

触发模型：**非 master 分支 push → 全量 CI →（CI 绿且分支有开启的 PR）testing 冒烟部署**；**PR 事件不触发流水线**（PR 页显示的是分支 push 的同 commit 检查，`ci-summary` 为合并必检）；**master 合并（push）不跑 CI，直接 CD 到 production**，前置由 `gate` job 回查来源 PR 的 `ci-summary` 结论与该 commit 的 testing 冒烟结论。

- `ci.yml`：`changes`（变更检测）→ `backend-lint`（gofmt / go vet / golangci-lint）→ `backend-test`（race + cover）→ `frontend-check`（type-check + build + 单测）→ `security-scan` → `migration-check` → `ci-summary`（汇总并派发 testing 部署）
- `cd.yml`：`gate`（解析目标环境 + 生产门禁）→ 构建并推送镜像（ghcr.io：backend / frontend / libreoffice，内容未变则跳过）→ 公网 SSH 部署 → 健康检查 → 失败自动回滚；testing 环境探活通过后停容器 → `notify`
- `testing-smoke.yml`：PR 开启时，若该 commit 尚无冒烟记录则补发一次 testing 部署

合并门禁：ruleset「protect master」要求必检 `ci-summary` 通过且分支与 master 同步 —— squash 会在 master 生成从未跑过 CI 的新 SHA，生产 `gate` 靠回查来源 PR 的结论对齐。

## 相关文档

| 文档 | 内容 |
| --- | --- |
| [`CONTEXT.md`](./CONTEXT.md) | 领域词汇表（canonical 术语与 Avoid 清单） |
| [`API.md`](./API.md) | API 清单 |
| [`AGENTS.md`](./AGENTS.md) | AI / agent 工作约定与发布流程 |
| [`docs/adr/`](./docs/adr/) | 架构决策记录（ADR-0001 ~ ADR-0026） |
| [`docs/agents/`](./docs/agents/) | issue tracker、triage labels、domain docs、security scan（本地） |
| [`student-api-docs.md`](./student-api-docs.md) | 学员端接口说明 |
| [`THIRD_PARTY.md`](./THIRD_PARTY.md) | 第三方组件与许可 |

## 许可证

本项目为**和润天下人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
