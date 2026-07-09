#!/usr/bin/env bash
# ======================================================================
# setup-tailscale.sh — 服务器端 Tailscale 安装与配置脚本
# ======================================================================
# 在自托管服务器上执行，把服务器加入 Tailscale 网络，
# 让 GitHub Actions Runner 能通过 Tailscale 内网 IP SSH 进来。
#
# 用法（以 root 执行）：
#   bash setup-tailscale.sh
#
# 前置：需要一个 Tailscale 账号（免费版即可，最多 100 台设备）
# ======================================================================
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[TAILSCALE]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[TAILSCALE]${NC} ✅ $1"; }
log_warn()  { echo -e "${YELLOW}[TAILSCALE]${NC} ⚠️  $1"; }
log_error() { echo -e "${RED}[TAILSCALE]${NC} ❌ $1"; }

# 检查 root 权限
if [ "$(id -u)" -ne 0 ]; then
    log_error "请以 root 用户运行此脚本"
    exit 1
fi

# 检测 OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS="$ID"
    OS_VERSION="$VERSION_ID"
else
    OS="unknown"
    OS_VERSION=""
fi

log_info "检测到系统: $OS $OS_VERSION"

# ======================================================================
# 1. 安装 Tailscale
# ======================================================================
install_tailscale() {
    log_info ">>> 安装 Tailscale..."

    if command -v tailscale &> /dev/null; then
        log_ok "Tailscale 已安装: $(tailscale version)"
        return
    fi

    # Tailscale 官方一键安装脚本（兼容所有 Linux 发行版，包括 Debian Stretch）
    log_info "使用官方安装脚本..."
    curl -fsSL https://pkgs.tailscale.com/stable/install.sh | sh

    if ! command -v tailscale &> /dev/null; then
        log_error "Tailscale 安装失败"
        log_info "请手动安装: https://tailscale.com/download/linux"
        exit 1
    fi

    log_ok "Tailscale 安装完成: $(tailscale version)"
}

# ======================================================================
# 2. 启动 Tailscale 守护进程
# ======================================================================
start_daemon() {
    log_info ">>> 启动 Tailscale 守护进程..."

    # 确保服务启用
    systemctl enable tailscaled 2>/dev/null || true
    systemctl start tailscaled 2>/dev/null || {
        log_warn "systemd 服务启动失败，尝试手动启动..."
        tailscaled --tun=userspace-networking --socks5-server=localhost:1055 &
        sleep 2
    }

    # 验证守护进程运行
    if systemctl is-active --quiet tailscaled 2>/dev/null; then
        log_ok "tailscaled 已运行"
    elif pgrep -x tailscaled > /dev/null; then
        log_ok "tailscaled 已运行（手动模式）"
    else
        log_error "tailscaled 未运行"
        exit 1
    fi
}

# ======================================================================
# 3. 登录并加入 tailnet
# ======================================================================
join_tailnet() {
    log_info ">>> 加入 Tailscale 网络..."

    if tailscale status 2>/dev/null | grep -q "Logged out"; then
        log_info "未登录，开始登录..."
        log_info "即将打开浏览器认证，请复制 URL 到本地浏览器打开"
        echo ""
        tailscale up --ssh --hostname=forklift-server --advertise-tags=tag:server
    else
        log_ok "已加入 tailnet"
    fi

    # 显示当前 Tailscale IP
    TS_IP=$(tailscale ip -4 2>/dev/null | head -1)
    if [ -n "$TS_IP" ]; then
        echo ""
        echo "=================================================="
        echo "  Tailscale 已就绪"
        echo ""
        echo "  服务器 Tailscale IP:  $TS_IP"
        echo "  主机名:               forklift-server"
        echo ""
        echo "  请把上面的 IP 配置到 GitHub Secrets:"
        echo "    TAILSCALE_HOST = $TS_IP"
        echo "=================================================="
        echo ""
    else
        log_warn "未能获取 Tailscale IP，请检查登录状态"
        log_info "手动执行: tailscale status"
    fi
}

# ======================================================================
# 4. 配置 ACL 标签（提示用户在 Tailscale 后台配置）
# ======================================================================
check_acl() {
    log_info ">>> 检查 ACL 配置..."

    echo ""
    echo "  请在 Tailscale Admin Console 配置 ACL 规则:"
    echo "  https://login.tailscale.com/admin/acls"
    echo ""
    echo "  在 ACL 中添加以下 tag 定义和规则:"
    echo ""
    echo '  "tagOwners": {'
    echo '    "tag:server": ["autogroup:members"],'
    echo '    "tag:ci":     ["autogroup:members"]'
    echo '  },'
    echo ""
    echo '  "acls": ['
    echo '    // 允许 CI 标签的设备 SSH 到 server 标签的设备'
    echo '    {"action": "accept", "src": ["tag:ci"], "dst": ["tag:server:*"]},'
    echo '    // 允许你自己的设备访问服务器'
    echo '    {"action": "accept", "src": ["autogroup:members"], "dst": ["tag:server:*"]}'
    echo '  ],'
    echo ""
    echo "  并创建 OAuth Client（用于 GitHub Actions）:"
    echo "  https://login.tailscale.com/admin/settings/oauth"
    echo "  - Tags: tag:ci"
    echo "  - 把生成的 Client ID 和 Client Secret 配置到 GitHub Secrets"
    echo ""
}

# ======================================================================
# 5. 验证安装
# ======================================================================
verify() {
    log_info ">>> 验证安装..."

    ERRORS=0

    check_item() {
        if eval "$2" &>/dev/null; then
            echo "  $1  ✅"
        else
            echo "  $1  ❌"
            ERRORS=$((ERRORS + 1))
        fi
    }

    check_item "tailscale 命令"   "command -v tailscale"
    check_item "tailscaled 运行"  "systemctl is-active --quiet tailscaled || pgrep -x tailscaled"
    check_item "已登录"           "tailscale status 2>/dev/null | grep -v 'Logged out'"

    echo ""

    if [ "$ERRORS" -eq 0 ]; then
        log_ok "Tailscale 配置完成!"
    else
        log_error "发现 $ERRORS 个问题，请排查"
        exit 1
    fi
}

# ======================================================================
# 主流程
# ======================================================================
main() {
    echo ""
    echo "=================================================="
    echo "  Tailscale 安装与配置"
    echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "=================================================="
    echo ""

    install_tailscale
    start_daemon
    join_tailnet
    check_acl
    verify

    echo ""
    echo "=================================================="
    echo "  完成! 后续步骤:"
    echo ""
    echo "  1. 在 Tailscale Admin Console 配置 ACL:"
    echo "     https://login.tailscale.com/admin/acls"
    echo ""
    echo "  2. 创建 OAuth Client（给 GitHub Actions 用）:"
    echo "     https://login.tailscale.com/admin/settings/oauth"
    echo "     Tags: tag:ci"
    echo ""
    echo "  3. 在 GitHub 仓库添加 Secrets:"
    echo "     TS_OAUTH_CLIENT_ID  ← OAuth Client ID"
    echo "     TS_OAUTH_SECRET      ← OAuth Client Secret"
    echo "     TAILSCALE_HOST       ← $(tailscale ip -4 2>/dev/null | head -1 || echo '100.x.x.x')"
    echo "     SSH_USER             ← deploy"
    echo "     SSH_PRIVATE_KEY      ← SSH 私钥（完整内容）"
    echo "=================================================="
}

main
