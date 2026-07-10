#!/usr/bin/env bash
# ======================================================================
# swap-cf-tunnel.sh — 替换 Cloudflare Tunnel（保留同一域名）
# ======================================================================
# 适用场景：旧隧道改名 / 轮换泄漏的 token / 彻底重建。
# 前提：新隧道已在 Cloudflare Zero Trust 控制台创建，且已配置好
#       Public Hostname（api.forklifttraining.asia -> http://localhost:8080）。
# 注意：使用 --token 模式时，路由配置存在 Cloudflare 侧，
#       服务器本地无需任何配置文件，只需换 token 即可。
#
# 用法：
#   NEW_TOKEN=<新token> bash scripts/swap-cf-tunnel.sh
#   或
#   bash scripts/swap-cf-tunnel.sh <新token>
# ======================================================================
set -euo pipefail

TOKEN="${1:-${NEW_TOKEN:-}}"
CONTAINER_NAME="cloudflared"
IMAGE="cloudflare/cloudflared:latest"
DOMAIN="${TUNNEL_DOMAIN:-api.forklifttraining.asia}"

# ---- 检查 token ----
if [ -z "$TOKEN" ]; then
  echo "❌ 未提供 Tunnel Token"
  echo "用法: NEW_TOKEN=<token> bash scripts/swap-cf-tunnel.sh"
  exit 1
fi

# ---- 检查 docker ----
if ! command -v docker &> /dev/null; then
  echo "❌ 未找到 docker 命令"
  exit 1
fi

echo "=================================================="
echo "  替换 Cloudflare Tunnel"
echo "  域名: $DOMAIN"
echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "=================================================="

# ---- [1/3] 停止并删除旧容器 ----
echo "[1/3] 停止并删除旧容器 $CONTAINER_NAME ..."
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
  docker rm   "$CONTAINER_NAME" >/dev/null 2>&1 || true
  echo "  ✅ 已停止并删除旧容器"
else
  echo "  ⚠️  未找到名为 $CONTAINER_NAME 的旧容器，直接创建新的"
fi

# ---- [2/3] 用新 token 启动容器 ----
# 复用本地已存在的镜像，避免 Docker Hub 拉取超时（国内网络常见）
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "  ⚠️  本地无 $IMAGE，尝试拉取（国内可能超时）..."
  docker pull "$IMAGE"
fi

echo "[2/3] 用新 token 启动新容器 (--network host) ..."
docker run -d \
  --name "$CONTAINER_NAME" \
  --network host \
  --restart unless-stopped \
  "$IMAGE" \
  tunnel --no-autoupdate run --token "$TOKEN"

# ---- [3/3] 状态与验证 ----
echo "[3/3] 等待启动并查看状态 ..."
sleep 5
docker ps --filter "name=$CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"
echo ""
echo "📋 后续操作："
echo "  查看日志: docker logs -f $CONTAINER_NAME"
echo "  验证通道: curl -s https://$DOMAIN/api/health"
echo "  确认隧道在 Cloudflare 控制台显示 Healthy 后再删除旧隧道"
echo "=================================================="
