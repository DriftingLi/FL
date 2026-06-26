# 叉车维修培训与残值评估系统

一个集成叉车维修培训、考试、考核与残值评估的综合平台。本仓库由原「维修培训」与「残值评估」两个独立项目合并而成，统一为一个 monorepo，单一后端进程，单一前端工程。

## 项目结构

```
叉车维修项目/
├── backend-go/              # Go 后端（维修培训 + 残值评估子模块）
│   ├── cmd/
│   │   ├── server/          # 服务入口（单一进程，:8080）
│   │   ├── migrate/         # 数据库迁移运行器
│   │   └── migrate-data/    # PG→PG 数据迁移工具
│   ├── internal/
│   │   ├── api/             # 维修培训 HTTP handlers
│   │   ├── config/          # 配置加载（.env + godotenv）
│   │   ├── db/              # GORM 数据库连接
│   │   ├── middleware/      # JWT/CORS/Recovery 中间件
│   │   ├── model/           # GORM 模型
│   │   ├── repository/      # 仓储层
│   │   ├── service/         # 业务服务层
│   │   └── valuation/       # 残值评估子模块（handler/model/repository/service/config）
│   ├── pkg/
│   │   ├── pdf/             # gofpdf PDF 生成（残值评估报告）
│   │   └── response/        # 统一响应封装
│   ├── migrations/          # golang-migrate SQL（000001~000004）
│   ├── queries/             # sqlc SQL 查询定义
│   ├── static/              # 静态资源（图片/视频/上传文件）
│   ├── assets/fonts/        # PDF 中文字体（simhei.ttf）
│   ├── docker-compose.yml   # 本地 PostgreSQL
│   ├── Makefile             # 开发命令
│   └── .env.example         # 环境变量模板
├── frontend/                # Vue 3 + TypeScript 前端
│   ├── src/
│   │   ├── api/             # API 客户端（含 valuation/ 子目录）
│   │   ├── components/      # 组件（practice/student/tutor/valuation/）
│   │   ├── composables/     # 组合式函数
│   │   ├── layouts/         # 布局组件
│   │   ├── pages/
│   │   │   ├── admin/       # 管理员页面
│   │   │   ├── auth/        # 登录/注册
│   │   │   ├── student/     # 学员页面
│   │   │   │   └── valuation/  # 残值评估页面
│   │   │   └── tutor/       # 导师页面
│   │   ├── router/          # 路由
│   │   ├── stores/          # Pinia 状态管理
│   │   ├── types/           # TypeScript 类型（含 valuation/）
│   │   └── utils/           # 工具函数（含 valuation 相关）
│   ├── vite.config.ts       # Vite 配置（含 /api 代理到 :8080）
│   └── package.json
├── docs/                    # 项目文档
│   ├── GLM-4.7-Flash.md
│   ├── OAuth JWT.md
│   ├── Web SDK.md
│   └── migration/risks.md
└── .gitignore
```

## 技术栈

### 后端（backend-go）

| 层 | 技术 | 说明 |
|---|---|---|
| 语言 | Go 1.26+ | 单一 module: `forklift-training` |
| Web 框架 | Gin v1.10 | 路由分组：`/api/*`（维修培训）+ `/api/valuation/*`（残值评估）|
| ORM | GORM v2 | 维修培训模块使用 |
| SQL 生成 | sqlc + pgx v5 | 残值评估模块使用 |
| 数据库 | PostgreSQL 15+ | 单一数据库，多 schema 共存 |
| 迁移 | golang-migrate v4 | 文件编号 000001~000004 |
| 认证 | golang-jwt v5 | 共享 JWT 中间件 |
| 配置 | godotenv + viper(indirect) | 统一 `.env` 配置 |
| 日志 | log/slog + go.uber.org/zap | 维修培训用 slog，残值评估用 zap |
| PDF | gofpdf | 残值评估报告生成 |
| AI | sashabaranov/go-openai | 智能助手 |

### 前端（frontend）

| 层 | 技术 | 版本 |
|---|---|---|
| 框架 | Vue 3 | 3.4+ |
| 语言 | TypeScript | 5.7+（全部 .ts，无 .js）|
| 构建 | Vite | 6.x |
| UI 库 | Element Plus | 2.5+ |
| 路由 | Vue Router | 4.x |
| 状态 | Pinia | 2.x |
| 3D | Three.js | 0.184 |
| 图表 | ECharts | 6.x |
| PDF 预览 | pdfjs-dist | 4.x |
| Markdown | marked + highlight.js | - |
| 拖拽 | vuedraggable | 4.x |
| AI SDK | @coze/web-sdk | 0.0.5 |

## 快速开始

### 前置要求

- Go 1.26+
- Node.js 18+ / npm
- Docker（用于本地 PostgreSQL）
- （可选）`golang-migrate` CLI、`sqlc` 用于代码生成

### 1. 启动后端

```bash
cd backend-go

# 复制环境变量模板
cp .env.example .env
# 按需修改 .env 中的 SECRET_KEY、JWT_SECRET_KEY、OPENAI_API_KEY 等

# 一键启动：PG 容器 + 迁移 + 运行服务
make dev

# 或分步执行
make dev-up         # 启动 PostgreSQL 容器
make migrate-up     # 执行数据库迁移
make run            # 运行服务（默认 :8080）

# 验证
curl http://localhost:8080/api/health
# {"status":"ok","message":"backend is running"}
```

### 2. 启动前端

```bash
cd frontend
npm install         # 安装依赖
npm run dev         # 启动开发服务器（:5173，自动代理 /api 到 :8080）
```

浏览器访问 http://localhost:5173

### 3. 构建生产版本

```bash
# 前端
cd frontend
npm run build       # 产物输出到 frontend/dist/

# 后端
cd backend-go
make build          # 产物输出到 backend-go/bin/server
```

## 后端开发

### 常用命令

| 命令 | 说明 |
|---|---|
| `make dev` | 启动容器 + 迁移 + 运行 |
| `make dev-up` / `make dev-down` | 启动/停止 PG 容器 |
| `make dev-reset` | 清除卷并重建（数据丢失）|
| `make migrate-up` / `make migrate-down` | 执行/回滚迁移 |
| `make migrate-data SOURCE="postgres://..."` | PG→PG 数据迁移 |
| `make build` | 编译二进制到 `bin/server` |
| `make test` | 运行测试（`-race -cover`）|
| `make lint` | golangci-lint 代码检查 |
| `make tidy` | 整理依赖 |
| `make fmt` | 格式化代码 |

### 路由结构

```
/api/health                    # 健康检查（公开）
/api/auth/*                    # 认证（登录/注册/管理员/导师）
/api/courses                   # 课程
/api/question-bank             # 题库
/api/practice/*                # 练习
/api/exam/*                    # 考试
/api/level-exam/*              # 等级考试
/api/mock-exam/*               # 模拟考试
/api/student/*                 # 学员
/api/tutor/*                   # 导师
/api/admin/*                   # 管理员
/api/grading/*                 # 评分
/api/wrong-questions           # 错题
/api/ai/*                      # AI 助手
/api/valuation/*               # 残值评估（JWTAuth 保护）
  ├── /brands                  # 品牌型号
  ├── /evaluations             # 评估
  ├── /historical              # 历史销售
  ├── /battery                 # 电池 RUL 预测
  ├── /part-configs            # 配件配置
  ├── /coefficient-configs     # 系数配置
  └── /reports                 # PDF 报告
```

### 默认账号

- 管理员：`admin` / `admin123`
- 导师：`tutor` / `tutor123`（启动时自动创建）
- 学员：`student` / `student123`（需通过 init 或注册创建）

## 前端开发

### 路由与页面

前端使用 Vue Router，主要路由模块：

- `/login` / `/register` — 认证
- `/` — 学员首页
- `/courses` — 课程列表/详情
- `/practice/*` — 知识练习/实操练习/3D 装配
- `/exam` / `/level-exam` / `/mock-exam` — 考试
- `/question-bank` / `/wrong-questions` — 题库/错题
- `/valuation/*` — 残值评估（首页/输入/结果/报告/电池）
- `/admin/*` — 管理员后台
- `/tutor/*` — 导师工作台

### Vite 代理

开发模式下，Vite 自动将以下路径代理到后端 `http://localhost:8080`：

- `/api/*` — API 请求
- `/static/*` — 静态资源

### 类型检查

```bash
cd frontend
npm run type-check   # vue-tsc --noEmit
```

## 残值评估模块

残值评估作为维修培训平台的内置模块，提供叉车二手残值评估能力。

### 评估维度

残值评估基于多维系数计算：

- **品牌系数**（KBrand）：按品牌型号查表
- **车况系数**（KCondition）：外观/机械/液压/电气等评分
- **工时系数**（KHours）：已使用工时衰减
- **年限系数**（KTime）：使用年限衰减
- **工况系数**（KWork）：作业环境强度
- **市场系数**（KMarket）：区域市场供需

### 电池 RUL 预测

独立的电池剩余使用寿命（RUL）预测功能：

- 输入电池型号/容量/循环次数/放电深度等参数
- 输出健康度评分、雷达图、RUL 预测
- 生成 PDF 报告（含中文字体 simhei.ttf）

### PDF 报告

残值评估报告通过 `pkg/pdf/` 生成：

- 模板：`pkg/pdf/template.go` / `battery_template.go`
- 字体：`assets/fonts/simhei.ttf`
- 雷达图：`pkg/pdf/radar.go`
- 输出目录：由 `VALUATION_PDF_OUTPUT_DIR` 配置（默认 `storage/reports`）

## 数据库迁移

迁移文件位于 `backend-go/migrations/`，统一编号：

| 编号 | 文件 | 说明 |
|---|---|---|
| 000001 | init.up/down.sql | 维修培训基础表 |
| 000002 | valuation.up/down.sql | 残值评估表 |
| 000003 | valuation_seed.up/down.sql | 残值评估种子数据 |
| 000004 | brand_models_repair.up/down.sql | 品牌型号修正 |

```bash
cd backend-go
make migrate-up      # 升级
make migrate-down    # 回滚
```

## 环境变量

完整模板见 `backend-go/.env.example`。关键变量：

### 应用与认证

| 变量 | 默认值 | 说明 |
|---|---|---|
| `APP_ENV` | development | 环境 |
| `PORT` | 8080 | 后端端口 |
| `SECRET_KEY` | dev-secret-key | 应用密钥（生产必填，不能为默认值）|
| `JWT_SECRET_KEY` | jwt-secret-key | JWT 密钥 |
| `JWT_EXPIRES_HOURS` | 1 | JWT 过期小时数 |

### 数据库

| 变量 | 说明 |
|---|---|
| `DATABASE_URL` | PostgreSQL 连接串 |

### 残值评估

| 变量 | 默认值 | 说明 |
|---|---|---|
| `VALUATION_PDF_OUTPUT_DIR` | storage/reports | PDF 报告输出目录 |
| `VALUATION_LOG_LEVEL` | info | 日志级别 |
| `VALUATION_LOG_FORMAT` | console | 日志格式（console/json）|
| `VALUATION_LOG_OUTPUT` | stdout | 日志输出 |
| `VALUATION_DB_MAX_OPEN_CONNS` | 20 | 数据库最大连接数 |
| `VALUATION_DB_MAX_IDLE_CONNS` | 5 | 数据库空闲连接数 |
| `VALUATION_DB_CONN_MAX_LIFETIME` | 3600 | 连接最大生命周期（秒）|

### CORS 与上传

| 变量 | 说明 |
|---|---|
| `CORS_ORIGINS` | 允许的跨域来源（逗号分隔）|
| `MAX_CONTENT_LENGTH_MB` | 上传文件大小限制（MB）|

### AI 与 OAuth

| 变量 | 说明 |
|---|---|
| `OPENAI_API_KEY` | OpenAI API Key |
| `COZE_PROJECT_ID` | Coze 项目 ID |
| `COZE_OAUTH_APP_ID` | Coze OAuth App ID |
| `COZE_OAUTH_KID` | Coze OAuth Key ID |
| `COZE_OAUTH_PRIVATE_KEY` | Coze OAuth 私钥（PEM）|
| `COZE_OAUTH_PRIVATE_KEY_PATH` | Coze OAuth 私钥文件路径 |

## 部署

### Docker

后端 Dockerfile 位于 `backend-go/Dockerfile`，前端可直接 `npm run build` 后将 `dist/` 部署到任意静态服务器或 CDN。

### Railway

后端 Railway 配置见 `backend-go/railway.toml` 与 `backend-go/nixpacks.toml`。

### Cloudflare Pages

前端可部署到 Cloudflare Pages，配置见 `frontend/wrangler.jsonc`。

## 项目合并说明

本仓库由以下两个独立项目合并而成：

1. **维修培训**（原 `维修培训/`）：叉车维修培训、考试、考核平台
2. **残值评估**（原 `残值评估/`）：叉车二手残值评估与电池 RUL 预测

### 合并要点

- **单一 Go module**：`forklift-training`，残值评估作为 `internal/valuation/` 子包
- **单一进程/端口**：所有路由由 `cmd/server/main.go` 装配，统一监听 `:8080`
- **路由隔离**：维修培训 `/api/*`，残值评估 `/api/valuation/*`
- **中间件共享**：JWT/CORS/Recovery 由 `internal/middleware/` 统一提供
- **配置统一**：`.env` + godotenv；残值评估子模块移除 viper 依赖
- **前端统一**：Vue 3 + TypeScript + Element Plus，残值评估页面整合到 `pages/student/valuation/`
- **数据库迁移合并**：000001~000004 顺序编号，避免冲突

## 文档

更多文档见 `docs/`：

- `GLM-4.7-Flash.md` — GLM-4.7-Flash 模型接入说明
- `OAuth JWT.md` — Coze OAuth JWT 认证
- `Web SDK.md` — Coze Web SDK
- `migration/risks.md` — 迁移风险评估

`backend-go/README.md` 包含后端更详细的开发说明。
