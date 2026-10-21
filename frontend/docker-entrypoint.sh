#!/bin/sh
# ===== Nginx 启动脚本 =====
# 启动前用 envsubst 替换占位符：
#   - ${DOMAIN}             server_name 域名
#   - ${BACKEND_HOST_PORT}  backend 宿主机映射端口（默认 8080，测试环境用 18080 避冲突）
#   - ${EDGE_ORIGIN_TOKEN}  EdgeOne 回源鉴权 token（门户 /api 代理回源时带 X-Origin-Token）
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

# 回源鉴权 fail closed（票 #1746）：模板启用 X-Origin-Token 时必须有 token。
# 没有 token 时，nginx 的 map 会退化成「空 token 匹配空头」⇒ 任何直连者伪造
# EO-Connecting-IP 都会被采信。与其静默降级，不如拒绝启动（部署侧也会先校验）。
if grep -q 'x-origin-token' /etc/nginx/conf.d/default.conf.template 2>/dev/null; then
    if [ -z "${EDGE_ORIGIN_TOKEN:-}" ]; then
        echo "[nginx-entrypoint] 致命: 模板启用 EdgeOne 回源鉴权，但 EDGE_ORIGIN_TOKEN 为空" >&2
        echo "[nginx-entrypoint] 请在部署环境提供 EDGE_ORIGIN_TOKEN（与门户 NUXT_API_ORIGIN_TOKEN 同值）" >&2
        exit 1
    fi
    echo "[nginx-entrypoint] EdgeOne 回源鉴权: 已启用（token 长度 ${#EDGE_ORIGIN_TOKEN}）"
else
    echo "[nginx-entrypoint] EdgeOne 回源鉴权: 模板未启用，跳过"
fi

# 仅替换 DOMAIN / BACKEND_HOST_PORT / EDGE_ORIGIN_TOKEN 变量（envsubst_defined_vars 模式）
# 这样 $host、$remote_addr 等 nginx 变量不会被替换。
# 模板里出现的 ${…} 集合必须与本行清单严格相等（票 #1742 的结构锁）：
# 少列一个 = 模板里留下字面量、nginx -t 直接失败；多列只是噪音，同样判红。
envsubst '${DOMAIN} ${BACKEND_HOST_PORT} ${EDGE_ORIGIN_TOKEN}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

# 验证 nginx 配置语法
nginx -t

# 启动 nginx
exec nginx -g 'daemon off;'
