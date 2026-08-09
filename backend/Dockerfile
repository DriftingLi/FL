# ===== Dockerfile：多阶段构建 Go 后端 =====
# 目标：产出轻量 Go 后端镜像（LibreOffice 已拆分到 sidecar 容器）

# ----- Builder 阶段：编译静态二进制 -----
FROM golang:1.26-alpine AS builder

# 安装构建依赖（git 用于 go module 下载）
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 先复制依赖文件，利用 Docker 层缓存
COPY go.mod go.sum ./
# 国内网络环境设置 GOPROXY 加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

# 复制源码
COPY . .

# 编译静态二进制（CGO 禁用，适合 scratch/alpine）
# -ldflags="-s -w" 去除调试信息，减小体积
# -trimpath 移除本地路径信息
# -buildvcs=false 不嵌入 commit SHA：纯文档/配置提交时二进制与上次一致，
#   镜像层 digest 复用，部署侧跳过拉取（配合 CD 内容哈希标签）
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -buildvcs=false -ldflags="-s -w" -o /bin/server ./cmd/server
RUN go build -trimpath -ldflags="-s -w" -o /bin/migrate ./cmd/migrate

# ----- Runtime 阶段：轻量运行时镜像 -----
FROM alpine:3.20

# 安装运行时依赖（已移除 LibreOffice + poppler-utils + font-noto-cjk,改由 sidecar 容器提供）：
# - ca-certificates tzdata su-exec：基础依赖
# - wget：健康检查
# 注:tzdata 必须保留,cp /usr/share/zoneinfo/Asia/Shanghai 依赖此包
RUN apk add --no-cache \
        ca-certificates \
        tzdata \
        su-exec \
        wget && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建非 root 用户
RUN addgroup -S app && adduser -S app -G app

# 创建应用目录 + 运行时数据目录
# /data 在镜像层预设属主，named volume 首次挂载会继承此权限（app 可写）
# bind mount 场景需在宿主机设置目录权限，或通过 compose user: root override 由 entrypoint 修正
RUN mkdir -p /app/bin /app/static/uploads/chapters /app/static/uploads/slides \
        /data/uploads /data/reports /data/backups && \
    chown -R app:app /app /data

WORKDIR /app

# 从 builder 复制编译产物
COPY --from=builder /bin/server /app/bin/server
COPY --from=builder /bin/migrate /app/bin/migrate

# 复制迁移文件（启动时自动迁移需要）
COPY --from=builder /build/migrations /app/migrations

# 字体文件已通过 //go:embed 编译进二进制，无需单独复制

# 入口脚本：以 root 创建 /data 子目录并修正属主，再切换到 app 用户
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

# 以非 root 用户运行（entrypoint.sh 兼容 root override 场景）
USER app

# 入口脚本接收 CMD 作为参数，切换到 app 用户后执行
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/bin/server"]
