#!/usr/bin/env bash
# ===== 叉车维修培训系统 - 一键部署脚本 =====
# 用法：
#   ./deploy.sh              # 交互式部署
#   ./deploy.sh staging      # 部署到 staging
#   ./deploy.sh production   # 部署到 production
#   ./deploy.sh --help       # 帮助

set -euo pipefail

# ---- 颜色输出 ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# ---- 配置 ----
ENV="${1:-}"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE="backend-go/.env"
BACKEND_DIR="$PROJECT_DIR/backend-go"
FRONTEND_DIR="$PROJECT_DIR/frontend"

# ---- 帮助 ----
show_help() {
    cat << EOF
叉车维修培训系统 - 部署脚本

用法:
  ./deploy.sh [环境]

环境:
  staging      部署到预发布环境
  production   部署到生产环境
  (留空)       交互式选择

选项:
  --help       显示帮助信息
  --skip-build 跳过构建步骤（仅重启）
  --skip-migrate 跳过数据库迁移

部署流程:
  1. 环境检查（Docker、Go、Node.js）
  2. 加载环境变量
  3. 构建前端
  4. 数据库迁移
  5. Docker Compose 部署
  6. 健康检查

EOF
    exit 0
}

# ---- 参数解析 ----
SKIP_BUILD=false
SKIP_MIGRATE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --help)
            show_help
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-migrate)
            SKIP_MIGRATE=true
            shift
            ;;
        staging|production)
            ENV="$1"
            shift
            ;;
        *)
            error "未知参数: $1"
            show_help
            ;;
    esac
done

# ---- 交互式选择环境 ----
if [ -z "$ENV" ]; then
    echo "请选择部署环境:"
    echo "  1) staging (预发布)"
    echo "  2) production (生产)"
    read -rp "输入选项 [1-2]: " choice
    case "$choice" in
        1) ENV="staging" ;;
        2) ENV="production" ;;
        *) error "无效选项"; exit 1 ;;
    esac
fi

info "部署环境: ${ENV}"

# ---- 环境检查 ----
check_prerequisites() {
    info "检查运行环境..."

    if ! command -v docker &> /dev/null; then
        error "未安装 Docker，请先安装: https://docs.docker.com/get-docker/"
        exit 1
    fi
    ok "Docker: $(docker --version)"

    if ! docker compose version &> /dev/null; then
        error "Docker Compose V2 未安装"
        exit 1
    fi
    ok "Docker Compose: $(docker compose version --short)"

    if ! command -v go &> /dev/null; then
        warn "未安装 Go，将跳过数据库迁移"
        SKIP_MIGRATE=true
    else
        ok "Go: $(go version)"
    fi

    if ! command -v node &> /dev/null; then
        warn "未安装 Node.js，将跳过前端构建"
        SKIP_BUILD=true
    else
        ok "Node.js: $(node --version)"
    fi
}

# ---- 加载环境变量 ----
load_env() {
    if [ ! -f "$PROJECT_DIR/$ENV_FILE" ]; then
        error "环境变量文件不存在: $ENV_FILE"
        info "请复制 .env.production 为 .env 并填写配置"
        exit 1
    fi

    # 加载 .env 文件到当前 shell（兼容简单格式）
    set -a
    # shellcheck disable=SC1090
    source "$PROJECT_DIR/$ENV_FILE"
    set +a
    ok "环境变量已加载"
}

# ---- 前端构建 ----
build_frontend() {
    if [ "$SKIP_BUILD" = true ]; then
        info "跳过前端构建"
        return
    fi

    info "构建前端..."
    cd "$FRONTEND_DIR"

    if [ ! -d "node_modules" ]; then
        info "安装前端依赖..."
        npm ci
    fi

    npm run build

    if [ $? -ne 0 ]; then
        error "前端构建失败!"
        exit 1
    fi
    ok "前端构建完成: dist/"
    cd "$PROJECT_DIR"
}

# ---- 数据库迁移 ----
run_migration() {
    if [ "$SKIP_MIGRATE" = true ]; then
        info "跳过数据库迁移"
        return
    fi

    info "执行数据库迁移..."
    cd "$BACKEND_DIR"

    # 确保 PostgreSQL 可用
    if ! pg_isready -h localhost -p 5432 -U "${DB_USER:-forklift}" &> /dev/null; then
        warn "PostgreSQL 未运行，尝试启动..."
        docker compose -f "$PROJECT_DIR/$COMPOSE_FILE" up -d postgres
        sleep 5
    fi

    go run ./cmd/migrate version || true
    go run ./cmd/migrate up

    if [ $? -ne 0 ]; then
        error "数据库迁移失败!"
        exit 1
    fi
    ok "数据库迁移完成"
    cd "$PROJECT_DIR"
}

# ---- Docker 部署 ----
docker_deploy() {
    info "Docker Compose 部署中..."

    cd "$PROJECT_DIR"

    # 拉取最新镜像（如果使用远程镜像）
    if [ -n "${BACKEND_IMAGE:-}" ]; then
        docker compose -f "$COMPOSE_FILE" pull backend
    fi

    # 启动/重启服务
    docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

    if [ $? -ne 0 ]; then
        error "Docker 部署失败!"
        exit 1
    fi
    ok "容器已启动"
}

# ---- 健康检查 ----
health_check() {
    info "健康检查..."

    BACKEND_PORT="${BACKEND_PORT:-8080}"
    MAX_RETRIES=30
    RETRY=0

    while [ $RETRY -lt $MAX_RETRIES ]; do
        STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$BACKEND_PORT/api/health" 2>/dev/null || echo "000")

        if [ "$STATUS" = "200" ]; then
            ok "后端健康检查通过"
            break
        fi

        RETRY=$((RETRY + 1))
        echo -ne "\r⏳ 等待后端就绪... ($STATUS) [$RETRY/$MAX_RETRIES]"
        sleep 2
    done

    if [ "$RETRY" -ge "$MAX_RETRIES" ]; then
        error "后端健康检查超时!"
        warn "请检查日志: docker compose -f $COMPOSE_FILE logs backend"
        exit 1
    fi

    # 前端检查
    NGINX_PORT="${NGINX_PORT:-80}"
    FRONTEND_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$NGINX_PORT/health" 2>/dev/null || echo "000")
    if [ "$FRONTEND_STATUS" = "200" ]; then
        ok "前端健康检查通过"
    else
        warn "前端健康检查返回 $FRONTEND_STATUS"
    fi
}

# ---- 部署汇总 ----
deploy_summary() {
    echo ""
    echo "========================================"
    echo "  部署完成!"
    echo "========================================"
    echo ""
    echo "  环境:     ${ENV}"
    echo "  后端 API: http://localhost:${BACKEND_PORT:-8080}/api"
    echo "  前端:     http://localhost:${NGINX_PORT:-80}"
    echo ""
    echo "  管理命令:"
    echo "    查看日志: docker compose -f $COMPOSE_FILE logs -f"
    echo "    查看状态: docker compose -f $COMPOSE_FILE ps"
    echo "    停止服务: docker compose -f $COMPOSE_FILE down"
    echo "    重启服务: docker compose -f $COMPOSE_FILE restart"
    echo ""
    echo "========================================"
}

# ---- 主流程 ----
main() {
    echo ""
    echo "  🚀 叉车维修培训系统 - 一键部署"
    echo "=========================================="
    echo ""

    check_prerequisites
    load_env
    build_frontend
    run_migration
    docker_deploy
    health_check
    deploy_summary
}

main
