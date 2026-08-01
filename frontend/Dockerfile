# ===== 前端多阶段构建 Dockerfile =====
# Stage 1: 构建 Vue 应用
# Stage 2: Nginx 托管静态资源

# ----- Builder 阶段 -----
FROM node:22-alpine AS builder

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
FROM nginx:1.27-alpine

# 设置时区 + 安装 gettext（提供 envsubst）
RUN apk add --no-cache tzdata gettext && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

# 复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

# 复制 Nginx 配置模板（启动时由 envsubst 替换 ${DOMAIN} 占位符）
COPY nginx.default.conf /etc/nginx/conf.d/default.conf.template

# 复制启动脚本
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# 健康检查端点
RUN echo "ok" > /usr/share/nginx/html/health

# 设置非 root 运行所需的目录权限
# nginx master 进程需要写 pid/cache/log 目录；监听 51820(>1024) 无需 root
# 注意：镜像内 /var/run 是指向 /run 的符号链接，chown -R 只改链接本身，
# 必须直接 chown /run，否则 nginx 用户无法创建 /run/nginx.pid
RUN mkdir -p /var/cache/nginx/client_temp /var/cache/nginx/proxy_temp \
        /var/cache/nginx/fastcgi_temp /var/cache/nginx/uwsgi_temp \
        /var/cache/nginx/scgi_temp && \
    chown -R nginx:nginx /var/cache/nginx /run /var/log/nginx \
        /usr/share/nginx/html /etc/nginx/conf.d

EXPOSE 80 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost/health || exit 1

# 默认 DOMAIN（可被 docker-compose environment 覆盖）
ENV DOMAIN=localhost

# 以非 root 用户运行（nginx:alpine 内置 UID 101）
USER nginx

ENTRYPOINT ["/docker-entrypoint.sh"]
