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

# 设置时区
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

# 复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

# 复制 Nginx 配置
COPY nginx.default.conf /etc/nginx/conf.d/default.conf

# 健康检查端点
RUN echo "ok" > /usr/share/nginx/html/health

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost/health || exit 1

CMD ["nginx", "-g", "daemon off;"]
