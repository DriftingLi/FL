# ===== 前端多阶段构建 Dockerfile =====
# Stage 1: 构建 Vue 应用
# Stage 2: Caddy 托管静态资源 + 反代（证书自动申请与续期）

# ----- Builder 阶段 -----
FROM node:24-alpine AS builder

WORKDIR /app

# 安装依赖（利用层缓存）
COPY package.json package-lock.json ./
RUN npm ci --production=false

# 复制源码
COPY . .

# 生产构建（VITE_API_BASE_URL 通过 build-arg 注入）
ARG VITE_API_BASE_URL=/api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

RUN npm run build

# ----- Runtime 阶段 -----
FROM caddy:2-alpine

# 时区（日志与证书续期记录按本地时间读）
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

# 复制构建产物（Caddyfile 的 file_server 根目录是 /srv）
COPY --from=builder /app/dist /srv

# 反代配置：用 Caddy 原生 {$VAR} 占位符，启动时不做 envsubst
COPY Caddyfile /etc/caddy/Caddyfile

# 启动脚本（校验环境变量 → 定证书策略 → validate → run）
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Caddy 的数据目录（证书、OCSP 缓存）与配置目录
VOLUME /data

EXPOSE 80 443

# 健康检查：relay 到本机的 /health（Caddy 的 respond 直接返回 200）
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost/health || exit 1

# 默认 DOMAIN（可被 docker-compose environment 覆盖）
ENV DOMAIN=localhost

ENTRYPOINT ["/docker-entrypoint.sh"]
