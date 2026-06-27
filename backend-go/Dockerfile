# ===== Dockerfile：多阶段构建 Go 后端 =====
# 目标：产出精简镜像 (< 100MB)，不含 LibreOffice（转换服务通过 sidecar 提供）

# ----- Builder 阶段：编译静态二进制 -----
FROM golang:1.26-alpine AS builder

# 安装构建依赖（git 用于 go module 下载）
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 先复制依赖文件，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译静态二进制（CGO 禁用，适合 scratch/alpine）
# -ldflags="-s -w" 去除调试信息，减小体积
# -trimpath 移除本地路径信息
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

# ----- Runtime 阶段：最小化运行时镜像 -----
FROM alpine:3.20

# 安装最小运行时依赖
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

# 创建非 root 用户
RUN addgroup -S app && adduser -S app -G app

# 创建上传目录
RUN mkdir -p /app/bin /app/static/uploads/chapters /app/static/uploads/slides && \
    chown -R app:app /app

WORKDIR /app

# 从 builder 复制编译产物
COPY --from=builder /bin/server /app/bin/server

# 复制迁移文件（启动时自动迁移需要）
COPY --from=builder /build/migrations /app/migrations

# 切换非 root 用户
USER app

EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

# 启动命令
ENTRYPOINT ["/app/bin/server"]
