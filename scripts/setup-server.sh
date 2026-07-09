#!/usr/bin/env bash
# ======================================================================
# setup-server.sh — 自托管服务器首次初始化脚本
# ======================================================================
# 在目标服务器上执行一次，用于初始化部署环境。
# 用法（在服务器上以 root 或 sudo 用户执行）：
#   curl -sSL https://raw.githubusercontent.com/.../scripts/setup-server.sh | bash
# 或手动执行：
#   bash setup-server.sh
# ======================================================================
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[SETUP]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[SETUP]${NC} ✅ $1"; }
log_warn()  { echo -e "${YELLOW}[SETUP]${NC} ⚠️  $1"; }
log_error() { echo -e "${RED}[SETUP]${NC} ❌ $1"; }

# ---- 配置（按需修改）----
DEPLOY_USER="${DEPLOY_USER:-deploy}"
DEPLOY_PATH="${DEPLOY_PATH:-/opt/forklift-training}"
SSH_PORT="${SSH_PORT:-22}"

# ---- 检查是否以 root 运行 ----
if [ "$(id -u)" -ne 0 ]; then
    log_error "请以 root 用户运行此脚本"
    exit 1
fi

# ---- OS 检测 ----
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS="$ID"
else
    OS="unknown"
fi

log_info "检测到系统: $OS"

# ======================================================================
# 1. 安装基础依赖
# ======================================================================
install_docker() {
    log_info ">>> 安装 Docker..."

    if command -v docker &> /dev/null; then
        log_ok "Docker 已安装: $(docker --version)"
        return
    fi

    case "$OS" in
        ubuntu|debian)
            apt-get update -qq
            apt-get install -y -qq ca-certificates curl gnupg lsb-release
            mkdir -p /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/$OS/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
                https://download.docker.com/linux/$OS $(lsb_release -cs) stable" | \
                tee /etc/apt/sources.list.d/docker.list > /dev/null
            apt-get update -qq
            apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        centos|rhel|fedora)
            yum install -y yum-utils
            yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            systemctl start docker
            systemctl enable docker
            ;;
        *)
            log_error "不支持的操作系统: $OS"
            log_info "请手动安装 Docker: https://docs.docker.com/engine/install/"
            exit 1
            ;;
    esac

    # 非 root 也需要 docker compose 可用
    systemctl start docker 2>/dev/null || true
    systemctl enable docker 2>/dev/null || true

    log_ok "Docker 安装完成: $(docker --version)"
}

# ======================================================================
# 2. 创建部署用户
# ======================================================================
create_deploy_user() {
    log_info ">>> 创建部署用户: $DEPLOY_USER"

    if id "$DEPLOY_USER" &>/dev/null; then
        log_ok "用户 $DEPLOY_USER 已存在"
    else
        useradd -m -s /bin/bash "$DEPLOY_USER"
        log_ok "用户 $DEPLOY_USER 已创建"
    fi

    # 添加到 docker 组
    usermod -aG docker "$DEPLOY_USER"

    # 设置免密码 sudo（仅 docker 相关命令）
    echo "$DEPLOY_USER ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart docker, \
          /usr/bin/systemctl status docker, /usr/bin/docker, /usr/bin/docker compose" \
          > /etc/sudoers.d/deploy
    chmod 440 /etc/sudoers.d/deploy

    log_ok "sudo 权限已配置"
}

# ======================================================================
# 3. 创建部署目录结构
# ======================================================================
create_deploy_dirs() {
    log_info ">>> 创建部署目录..."

    mkdir -p "$DEPLOY_PATH"/{backups,data/{uploads/{chapters,slides},reports},nginx/ssl}
    chown -R "$DEPLOY_USER":"$DEPLOY_USER" "$DEPLOY_PATH"
    chmod 755 "$DEPLOY_PATH"

    log_ok "部署目录已创建: $DEPLOY_PATH"
}

# ======================================================================
# 4. 配置防火墙
# ======================================================================
configure_firewall() {
    log_info ">>> 配置防火墙..."

    if command -v ufw &> /dev/null; then
        # Ubuntu/Debian UFW
        ufw allow 80/tcp comment "HTTP"
        ufw allow 443/tcp comment "HTTPS"
        ufw allow "$SSH_PORT"/tcp comment "SSH"
        ufw --force enable 2>/dev/null || true
        log_ok "UFW 防火墙已配置"
    elif command -v firewall-cmd &> /dev/null; then
        # CentOS/RHEL firewalld
        firewall-cmd --permanent --add-service=http
        firewall-cmd --permanent --add-service=https
        firewall-cmd --permanent --add-port="$SSH_PORT"/tcp
        firewall-cmd --reload
        log_ok "firewalld 已配置"
    else
        log_warn "未检测到防火墙，请手动配置"
    fi
}

# ======================================================================
# 5. 配置系统参数
# ======================================================================
configure_system() {
    log_info ">>> 优化系统参数..."

    # vm.max_map_count（Elasticsearch 等需要，PostgreSQL 也可能用到）
    sysctl -w vm.max_map_count=262144 > /dev/null
    echo "vm.max_map_count=262144" > /etc/sysctl.d/99-docker.conf

    # 文件描述符限制
    if ! grep -q "nofile" /etc/security/limits.d/99-docker.conf 2>/dev/null; then
        cat > /etc/security/limits.d/99-docker.conf << EOF
* soft nofile 65536
* hard nofile 65536
EOF
    fi

    # 时区
    timedatectl set-timezone Asia/Shanghai 2>/dev/null || true

    log_ok "系统参数已优化"
}

# ======================================================================
# 6. 设置日志轮转
# ======================================================================
configure_logrotate() {
    log_info ">>> 配置 Docker 日志轮转..."

    if [ ! -f /etc/docker/daemon.json ]; then
        echo '{}' > /etc/docker/daemon.json
    fi

    # 使用 Python 安全地修改 JSON（如果可用），否则手动合并
    TMP=$(mktemp)
    if command -v python3 &> /dev/null; then
        python3 -c "
import json
with open('/etc/docker/daemon.json') as f:
    config = json.load(f)
config['log-driver'] = 'json-file'
config['log-opts'] = {
    'max-size': '10m',
    'max-file': '3'
}
with open('$TMP', 'w') as f:
    json.dump(config, f, indent=2)
"
        mv "$TMP" /etc/docker/daemon.json
    else
        cat > /etc/docker/daemon.json << 'DAEMON'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
DAEMON
    fi

    systemctl restart docker 2>/dev/null || true
    log_ok "Docker 日志轮转已配置 (max 10MB × 3)"
}

# ======================================================================
# 7. 创建初始 .env 模板
# ======================================================================
create_env_template() {
    log_info ">>> 创建环境变量模板..."

    if [ ! -f "$DEPLOY_PATH/.env" ]; then
        cat > "$DEPLOY_PATH/.env.template" << 'ENVEOF'
# ===== 叉车维修培训系统 - 环境变量模板 =====
# 复制为 .env 并填入实际值。
# 生产部署时由 GitHub Actions CD 流水线自动注入，无需手动编辑。

APP_ENV=production
PORT=8080

# 数据库（必需）
DATABASE_URL=postgres://forklift:请替换密码@postgres:5432/forklift_training?sslmode=disable
DB_USER=forklift
DB_PASSWORD=请替换密码

# 密钥（必需，至少 32 字符随机字符串）
SECRET_KEY=请替换为强随机字符串
JWT_SECRET_KEY=请替换为另一个强随机字符串
JWT_EXPIRES_HOURS=24

# 默认账号密码（生产必须修改）
ADMIN_DEFAULT_PASSWORD=请替换为强密码
TUTOR_DEFAULT_PASSWORD=请替换为强密码
STUDENT_DEFAULT_PASSWORD=请替换为强密码

# CORS（多个域名用逗号分隔）
CORS_ORIGINS=https://your-domain.com

# AI 配置
ZHIPU_API_KEY=your-zhipu-api-key
ZHIPU_BASE_URL=https://open.bigmodel.cn/api/paas/v4
ZHIPU_MODEL=glm-4.7-flash
OPENAI_API_KEY=

# Coze（可选）
COZE_PROJECT_ID=
COZE_OAUTH_APP_ID=
COZE_OAUTH_KID=

# 后端镜像（CD 流水线自动填充）
BACKEND_IMAGE=ghcr.io/YOUR_ORG/forklift-training-backend
ENVEOF
        chown "$DEPLOY_USER":"$DEPLOY_USER" "$DEPLOY_PATH/.env.template"
        log_ok ".env.template 已创建"
    else
        log_ok ".env 已存在，跳过模板创建"
    fi
}

# ======================================================================
# 8. 设置 cron 定时任务（每日备份 + 证书续期）
# ======================================================================
setup_cron() {
    log_info ">>> 配置定时任务..."

    CRON_FILE="/etc/cron.d/forklift-maintenance"

    cat > "$CRON_FILE" << 'CRONEOF'
# 叉车维修系统 - 定时维护任务
# 每日凌晨 2:00 清理 Docker 构建缓存
0 2 * * * root docker builder prune -f --filter "until=72h" > /dev/null 2>&1
# 每周日凌晨 3:00 清理旧日志
0 3 * * 0 root find /var/lib/docker/containers -name "*.log" -size +100M -exec truncate -s 0 {} \; 2>/dev/null
CRONEOF

    chmod 644 "$CRON_FILE"
    log_ok "定时任务已配置"
}

# ======================================================================
# 验证安装
# ======================================================================
verify() {
    log_info ">>> 验证安装..."

    ERRORS=0

    echo ""
    echo "  项目               状态"
    echo "  -----------------  --------"

    check_item() {
        if eval "$2" &>/dev/null; then
            echo "  $1  ✅"
        else
            echo "  $1  ❌"
            ERRORS=$((ERRORS + 1))
        fi
    }

    check_item "Docker"            "docker --version"
    check_item "Docker Compose"    "docker compose version"
    check_item "部署用户"          "id $DEPLOY_USER"
    check_item "部署目录"          "test -d $DEPLOY_PATH"
    check_item "Docker 运行中"     "docker info"

    echo ""

    if [ "$ERRORS" -eq 0 ]; then
        log_ok "全部检查通过!"
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
    echo "  叉车维修培训系统 - 服务器初始化"
    echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "=================================================="
    echo ""
    echo "  部署用户:  $DEPLOY_USER"
    echo "  部署路径:  $DEPLOY_PATH"
    echo "  SSH 端口:  $SSH_PORT"
    echo ""

    install_docker
    create_deploy_user
    create_deploy_dirs
    configure_firewall
    configure_system
    configure_logrotate
    create_env_template
    setup_cron
    verify

    echo ""
    echo "=================================================="
    echo "  服务器初始化完成!"
    echo ""
    echo "  后续步骤:"
    echo "  1. 将 GitHub Actions SSH 公钥添加到服务器:"
    echo "     ssh-copy-id -i <公钥路径> $DEPLOY_USER@<服务器IP>"
    echo ""
    echo "  2. 在 GitHub 仓库 Settings → Secrets and variables →"
    echo "     Actions 中添加以下 Secrets:"
    echo "       SSH_HOST       服务器 IP/域名"
    echo "       SSH_USER       $DEPLOY_USER"
    echo "       SSH_PORT       22"
    echo "       SSH_PRIVATE_KEY  SSH 私钥（完整内容，含 BEGIN/END）"
    echo "       DATABASE_URL   PostgreSQL 连接串"
    echo "       SECRET_KEY     应用密钥"
    echo "       JWT_SECRET_KEY JWT 密钥"
    echo "       ... 及其他环境变量"
    echo ""
    echo "  3. 复制配置文件到服务器:"
    echo "     scp docker-compose.prod.yml nginx/nginx.conf \\"
    echo "       nginx/default.conf $DEPLOY_USER@<服务器>:$DEPLOY_PATH/"
    echo ""
    echo "  4. 推送代码到 main 分支自动触发部署!"
    echo "=================================================="
}

main
