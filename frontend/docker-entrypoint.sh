#!/bin/sh
# ===== Nginx 启动脚本 =====
# 启动前用 envsubst 替换占位符：
#   - ${DOMAIN}            server_name 域名
#   - ${BACKEND_HOST_PORT}  backend 宿主机映射端口（默认 8080，测试环境用 18080 避冲突）
#   - ${PORTAL_HOST_PORT}   官网门户（hrwai-portal）宿主机监听端口（默认 3000）
# 仅替换以上变量，避免影响 nginx 原生变量（如 $host、$remote_addr）

set -e

# 检查 DOMAIN 环境变量
if [ -z "${DOMAIN:-}" ]; then
    echo "[nginx-entrypoint] 警告: DOMAIN 环境变量未设置，server_name 将保留占位符"
    echo "[nginx-entrypoint] 请在 docker-compose 中配置 DOMAIN 环境变量"
else
    echo "[nginx-entrypoint] 替换 server_name 占位符为: ${DOMAIN}"
fi

# backend 端口默认 8080（未设置 BACKEND_HOST_PORT 时）
# 注意：必须 export——envsubst 是子进程，只读 shell 局部变量会渲染为空端口
export BACKEND_HOST_PORT="${BACKEND_HOST_PORT:=8080}"
echo "[nginx-entrypoint] backend 代理端口: ${BACKEND_HOST_PORT}"

# portal 端口默认 3000（未设置 PORTAL_HOST_PORT 时）
export PORTAL_HOST_PORT="${PORTAL_HOST_PORT:=3000}"
echo "[nginx-entrypoint] portal 代理端口: ${PORTAL_HOST_PORT}"

# 仅替换 DOMAIN / BACKEND_HOST_PORT / PORTAL_HOST_PORT 变量（envsubst_defined_vars 模式）
# 这样 $host、$remote_addr 等 nginx 变量不会被替换
envsubst '${DOMAIN} ${BACKEND_HOST_PORT} ${PORTAL_HOST_PORT}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

# 验证 nginx 配置语法
nginx -t

# 启动 nginx
exec nginx -g 'daemon off;'
