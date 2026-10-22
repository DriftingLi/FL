#!/bin/sh
# ===== Caddy 启动脚本 =====
# 职责只有三件：校验必需的环境变量、按环境决定证书策略、验配置后启动。
# **不做 envsubst**：Caddyfile 用原生的 {$VAR} 占位符，由 Caddy 自己展开。
# 少一层文本替换，就少一条「模板占位符与替换清单不一致」的部署面（旧 nginx 链的血账）。

set -e

# ---- 1. 必需变量 ----
if [ -z "${DOMAIN:-}" ]; then
    echo "[caddy-entrypoint] 致命: DOMAIN 未设置，站群地址无法展开" >&2
    exit 1
fi
echo "[caddy-entrypoint] DOMAIN=${DOMAIN}"

export BACKEND_HOST_PORT="${BACKEND_HOST_PORT:=8080}"
echo "[caddy-entrypoint] backend 代理端口: ${BACKEND_HOST_PORT}"

# ---- 2. 回源鉴权 fail closed（票 #1746）----
# Caddyfile 的 (backend_api) 片段用 X-Origin-Token 命中才采信 EO-Connecting-IP。
# token 为空时那条 header 匹配会退化成「空键匹配空头」⇒ 任何直连者伪造 EO-Connecting-IP
# 都会被采信。与其静默降级，不如拒绝启动。
if grep -q 'X-Origin-Token' /etc/caddy/Caddyfile 2>/dev/null; then
    if [ -z "${EDGE_ORIGIN_TOKEN:-}" ]; then
        echo "[caddy-entrypoint] 致命: Caddyfile 启用 EdgeOne 回源鉴权，但 EDGE_ORIGIN_TOKEN 为空" >&2
        echo "[caddy-entrypoint] 请在部署环境提供 EDGE_ORIGIN_TOKEN（与门户 NUXT_API_ORIGIN_TOKEN 同值）" >&2
        exit 1
    fi
    echo "[caddy-entrypoint] EdgeOne 回源鉴权: 已启用（token 长度 ${#EDGE_ORIGIN_TOKEN}）"
else
    echo "[caddy-entrypoint] EdgeOne 回源鉴权: 模板未启用，跳过"
fi

# ---- 3. 证书策略按环境分档 ----
# 生产：CADDY_TLS 为空 ⇒ 走默认的自动 HTTPS（ACME，443 的 TLS-ALPN 为主、80 的 HTTP-01 兜底）。
# 非生产：关掉自动 HTTPS 并改用内部自签，冒烟不占公共证书配额、也不要求公网解析指向测试机。
# 判据是 TLS_MODE 而不是 APP_ENV：两套环境都声明自己 APP_ENV=production（后端行为一致），
# 证书策略却是部署面的事实，必须单独一个开关，否则测试冒烟会去给生产域名申请证书。
# ACME 账户邮箱也是「可选行」：写成 `email {$VAR}` 时，VAR 为空会让这行退化成光秃秃的
# `email`，Caddy 直接拒解析（2026-10-22 testing 冒烟实测）。所以整行由这里拼。
if [ -n "${CADDY_ACME_EMAIL:-}" ]; then
    CADDY_ACME_LINE="email ${CADDY_ACME_EMAIL}"
else
    CADDY_ACME_LINE=""
fi
export CADDY_ACME_LINE
echo "[caddy-entrypoint] ACME 邮箱: ${CADDY_ACME_EMAIL:-（未配置，用 Caddy 默认匿名账户）}"

if [ "${TLS_MODE:-auto}" = "auto" ]; then
    CADDY_TLS=""
    CADDY_GLOBAL=""
    echo "[caddy-entrypoint] 证书: 自动 HTTPS（ACME）"
else
    CADDY_TLS="tls internal"
    CADDY_GLOBAL="auto_https off"
    echo "[caddy-entrypoint] 证书: 内部自签（TLS_MODE=${TLS_MODE}，不申请公共证书）"
fi
export CADDY_TLS CADDY_GLOBAL

# ---- 4. 验配置再启动 ----
# 注意：validate 会展开 {$VAR}，所以必须在上面几个变量都 export 之后执行。
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
