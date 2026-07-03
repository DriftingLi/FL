# 叉车维修培训系统后端（Go + Gin + PostgreSQL）

基于 Go + Gin + GORM + PostgreSQL 重构的后端服务，与原 Python Flask 版 API 契约完全兼容。

## 技术栈

| 层 | 技术 | 版本 |
|---|---|---|
| 语言 | Go | 1.22+ |
| Web 框架 | Gin | v1.10 |
| ORM | GORM | v2 |
| 数据库 | PostgreSQL | 15+ |
| 迁移 | golang-migrate | v4 |
| 认证 | golang-jwt | v5 |
| 密码 | bcrypt | golang.org/x/crypto |
| AI | sashabaranov/go-openai | - |
| 日志 | log/slog | 标准库 |

## 目录结构

```
backend-go/
├── cmd/
│   ├── server/          # 服务入口
│   ├── migrate/         # 迁移运行器
│   └── migrate-data/    # PG→PG 数据迁移工具（Python版→Go版 schema）
├── internal/
│   ├── api/             # HTTP handlers（按蓝图分文件）
│   ├── config/          # 配置加载
│   ├── db/              # 数据库连接
│   ├── middleware/      # 中间件（JWT/CORS/日志/Recovery）
│   ├── migrate/         # 迁移封装
│   ├── model/           # GORM 模型
│   ├── repository/      # 仓储层（待实现）
│   └── service/         # 业务服务层
├── migrations/          # golang-migrate SQL 文件
├── pkg/response/        # 统一响应
├── static/uploads/      # 上传目录
├── docker-compose.yml   # 本地 Docker PostgreSQL
├── Makefile             # 开发命令
└── .env.example         # 环境变量模板
```

## 本地开发快速开始

本地 PostgreSQL 一律运行在 Docker 容器中（不在宿主机直接安装）。

```bash
# 1. 复制环境变量模板
cp .env.example .env

# 2. 一键启动：PG 容器 + 迁移 + 运行服务
make dev

# 或分步执行
make dev-up         # 启动 PostgreSQL 容器
make migrate-up     # 执行数据库迁移
make run            # 运行服务（默认 :8080）

# 3. 验证
curl http://localhost:8080/api/health
# {"status":"ok","message":"backend is running"}
```

## 常用命令

| 命令 | 说明 |
|---|---|
| `make dev` | 启动容器 + 迁移 + 运行 |
| `make dev-up` | 启动 PG 容器 |
| `make dev-down` | 停止 PG 容器 |
| `make dev-reset` | 清除卷并重建（数据丢失） |
| `make migrate-up` | 执行迁移 |
| `make migrate-down` | 回滚迁移 |
| `make migrate-data SOURCE="postgres://..."` | PG→PG 数据迁移（Python版→Go版） |
| `make build` | 编译二进制 |
| `make test` | 运行测试 |
| `make lint` | 代码检查 |
| `make tidy` | 整理依赖 |

## 数据库迁移

```bash
# 升级
make migrate-up

# 回滚
make migrate-down

# PostgreSQL → PostgreSQL 数据迁移（Python 版生产库 → Go 版 schema）
make migrate-data SOURCE="postgres://user:pass@host:5432/forklift_training?sslmode=disable"
```

## API

所有 API 路径与原 Python 版完全一致，前端无需改动。

- `GET  /api/health` — 健康检查
- `POST /api/auth/login` — 学员登录
- `POST /api/auth/register` — 学员注册
- `POST /api/auth/admin-login` — 管理员登录
- `POST /api/auth/tutor-login` — 导师登录
- `GET  /api/auth/me` — 当前用户信息
- `GET  /api/courses` — 课程列表
- `GET  /api/question-bank` — 题库
- `GET  /api/practice/random` — 随机抽题
- ...（完整列表见 `docs/migration/api-contract.md`）

## 默认账号

系统启动时自动创建默认账号，密码通过环境变量配置：

- 管理员：`admin`，密码由 `ADMIN_DEFAULT_PASSWORD` 配置
- 导师：`tutor`，密码由 `TUTOR_DEFAULT_PASSWORD` 配置
- 学员：`student`，密码由 `STUDENT_DEFAULT_PASSWORD` 配置

> 开发环境默认值见 `.env.example`；生产环境必须设置强密码。

## 部署

### Docker

```dockerfile
# 多阶段构建（待 Task 39 完整实现）
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM alpine:latest
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
```

### Railway

生产环境配置见 `railway.toml` 与 `nixpacks.toml`（待 Task 40 实现）。
