#!/bin/sh
# ===== Nginx 启动脚本 =====
# 启动前用 envsubst 将 ${DOMAIN} 替换为实际域名
# 仅替换 DOMAIN 变量，避免影响 nginx 原生变量（如 $host、$remote_addr）

set -e

# 检查 DOMAIN 环境变量
if [ -z "${DOMAIN:-}" ]; then
    echo "[nginx-entrypoint] 警告: DOMAIN 环境变量未设置，server_name 将保留占位符"
    echo "[nginx-entrypoint] 请在 docker-compose 中配置 DOMAIN 环境变量"
else
    echo "[nginx-entrypoint] 替换 server_name 占位符为: ${DOMAIN}"
fi

# 仅替换 DOMAIN 变量（用 envsubst 的 envsubst_defined_vars 模式）
# 这样 $host、$remote_addr 等 nginx 变量不会被替换
envsubst '${DOMAIN}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

# 验证 nginx 配置语法
nginx -t

# 启动 nginx
exec nginx -g 'daemon off;'
