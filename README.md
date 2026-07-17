# 叉车维修培训与残值评估系统

一套面向叉车维修培训与叉车残值评估的全栈系统，包含在线培训、考试练习、AI 助手，以及叉车残值评估与电池剩余寿命（RUL）评估等业务模块。系统按角色（学生 / 讲师 / 管理员）划分权限，提供完整的后台管理与学生学习闭环。

## 功能特性

### 培训学习模块

- **课程管理**：课程 CRUD、章节内容编排、PPT / 视频 / 图文混排
- **考试系统**：课程考试、模拟考试、等级考试，自动判分与成绩统计
- **练习中心**：自由练习、知识点练习、错题本、练习统计
- **AI 助手**：基于大模型的智能问答与内容生成

### 残值评估模块

- **叉车残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域等参数，输出残值估算、置信区间与多维系数雷达图
- **评估公式**：`残值 = 原价 × Kt_adj × Kc × Km`，其中 `Kt_adj = Kt^(Kh/Kb)`
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km
- **PDF 报告**：评估结果一键生成可下载的 PDF 报告
- **管理后台**：原价表配置、算法参数调整、系数表管理

## 技术栈

### 后端（backend）

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
├── backend/                  # Go 后端
│   ├── cmd/                     # 入口：server（服务）、migrate（迁移）、visual_check（辅助检查）
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

## 环境要求

- Go ≥ 1.26
- Node.js ≥ 18（推荐 20+）
- Docker + Docker Compose（用于本地 PostgreSQL）
- PostgreSQL 15（由 docker-compose 提供，或使用已有实例）

## 快速开始

### 1. 启动数据库

```bash
cd backend
docker compose up -d postgres
```

### 2. 配置后端环境变量

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

- 前端：<http://localhost:5173>
- 后端 API：<http://localhost:8080/api>

## 默认账号

系统启动时会自动创建默认账号（管理员 / 讲师 / 学员），默认密码通过以下环境变量配置：

| 角色  | 用户名     | 环境变量                       |
| --- | ------- | -------------------------- |
| 管理员 | admin   | `ADMIN_DEFAULT_PASSWORD`   |
| 讲师  | tutor   | `TUTOR_DEFAULT_PASSWORD`   |
| 学员  | student | `STUDENT_DEFAULT_PASSWORD` |

## 配置说明

后端配置通过 `backend/.env` 注入，关键项：

| 变量                         | 说明                     | 默认值                                    |
| -------------------------- | ---------------------- | -------------------------------------- |
| `APP_ENV`                  | 运行环境                   | development                            |
| `PORT`                     | 服务端口                   | 8080                                   |
| `DATABASE_URL`             | PostgreSQL 连接串         | 由 docker-compose 提供，本地开发参考 `.env`      |
| `SECRET_KEY`               | 应用密钥（生产必改）             | 空                                      |
| `JWT_SECRET_KEY`           | JWT 签名密钥（生产必改）         | 空                                      |
| `JWT_EXPIRES_HOURS`        | JWT 有效期（小时）            | 1（`.env.production` 为 24）              |
| `CORS_ORIGINS`             | 允许的前端来源                | <http://localhost:5173>                |
| `UPLOAD_FOLDER`            | 上传目录                   | static/uploads                         |
| `MAX_CONTENT_LENGTH_MB`    | 上传大小上限                 | 250                                    |
| `ADMIN_DEFAULT_PASSWORD`   | 管理员默认密码（生产必改）          | 空                                      |
| `TUTOR_DEFAULT_PASSWORD`   | 讲师默认密码（生产必改）           | 空                                      |
| `STUDENT_DEFAULT_PASSWORD` | 学员默认密码（生产必改）           | 空                                      |
| `ZHIPU_API_KEY`            | 智谱 GLM API Key（主用 AI）  | 空                                      |
| `ZHIPU_BASE_URL`           | 智谱 GLM 接口地址            | <https://open.bigmodel.cn/api/paas/v4> |
| `ZHIPU_MODEL`              | 智谱 GLM 模型名             | glm-4.7-flash                          |
| `OPENAI_API_KEY`           | OpenAI 密钥（ZHIPU 为空时备用） | 空                                      |
| `COZE_*`                   | Coze OAuth 配置          | 空                                      |
| `VALUATION_PDF_OUTPUT_DIR` | 评估报告 PDF 输出目录          | storage/reports                        |

## 常用命令

### 后端（在 `backend/` 下）

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
```

### 前端（在 `frontend/` 下）

```bash
npm run dev          # 启动开发服务器
npm run build        # 生产构建
npm run preview      # 预览构建产物
npm run type-check   # TypeScript 类型检查
```

## 数据库迁移

迁移脚本位于 `backend/migrations/`，采用 `序号_名称.up.sql` / `.down.sql` 成对组织，共 17 组（000001 \~ 000017），覆盖初始化、残值评估表结构、级联过滤、种子数据、系数调整等。

## 许可证

本项目为**和润天下人工智能科技有限公司**内部系统，**未声明开源许可证**，仅供公司内部使用与授权合作方访问。未经授权，不得复制、分发、部署或修改本项目的任何部分。
