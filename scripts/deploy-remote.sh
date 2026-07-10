#!/usr/bin/env bash
# ======================================================================
# deploy-remote.sh — 远程服务器端后端部署脚本
# ======================================================================
# 仅部署后端服务（PostgreSQL + Go API）。
# 前端由 Cloudflare Pages 单独托管，不在此脚本管理范围内。
# 也可以手动执行：
#   bash deploy-remote.sh            # 正常部署
#   bash deploy-remote.sh --rollback # 回滚到上一个版本
# ======================================================================
set -euo pipefail

# ---- 颜色输出 ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[DEPLOY]${NC} $(date '+%H:%M:%S') $1"; }
log_ok()    { echo -e "${GREEN}[DEPLOY]${NC} $(date '+%H:%M:%S') ✅ $1"; }
log_warn()  { echo -e "${YELLOW}[DEPLOY]${NC} $(date '+%H:%M:%S') ⚠️  $1"; }
log_error() { echo -e "${RED}[DEPLOY]${NC} $(date '+%H:%M:%S') ❌ $1"; }

# ======================================================================
# 配置（可通过环境变量覆盖，或在此修改默认值）
# ======================================================================
MODE="${1:-deploy}"  # deploy | rollback

# 部署路径
DEPLOY_PATH="${DEPLOY_PATH:-/opt/forklift-training}"
BACKUP_DIR="${DEPLOY_PATH}/backups"

# Docker
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
BACKEND_SERVICE="${BACKEND_SERVICE:-backend}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"

# 注册表认证
REGISTRY="${REGISTRY:-ghcr.io}"
IMAGE_BACKEND="${IMAGE_BACKEND:-}"
IMAGE_FRONTEND="${IMAGE_FRONTEND:-}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# 健康检查
BACKEND_PORT="${BACKEND_PORT:-8080}"
HEALTH_CHECK_RETRIES="${HEALTH_CHECK_RETRIES:-20}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-6}"

# 迁移
SKIP_MIGRATION="${SKIP_MIGRATION:-false}"

# 清理策略：保留最近 N 个镜像
KEEP_IMAGES="${KEEP_IMAGES:-3}"

# ======================================================================
# 工具函数
# ======================================================================
check_dependency() {
    if ! command -v "$1" &> /dev/null; then
        log_error "未找到命令: $1，请先安装"
        exit 1
    fi
}

write_env_file() {
    # 生成 .env 文件（供 docker compose 使用）
    cat > "${DEPLOY_PATH}/.env" << ENVEOF
# 由 deploy-remote.sh 自动生成 — $(date '+%Y-%m-%d %H:%M:%S')
# 请勿手动编辑此文件

APP_ENV=production
PORT=${BACKEND_PORT}

# 数据库
DATABASE_URL=${DATABASE_URL:-}
DB_USER=${DB_USER:-forklift}
DB_PASSWORD=${DB_PASSWORD:-}

# 密钥
SECRET_KEY=${SECRET_KEY:-}
JWT_SECRET_KEY=${JWT_SECRET_KEY:-}
JWT_EXPIRES_HOURS=${JWT_EXPIRES_HOURS:-24}

# 默认账号
ADMIN_DEFAULT_PASSWORD=${ADMIN_DEFAULT_PASSWORD:-}
TUTOR_DEFAULT_PASSWORD=${TUTOR_DEFAULT_PASSWORD:-}
STUDENT_DEFAULT_PASSWORD=${STUDENT_DEFAULT_PASSWORD:-}

# CORS
CORS_ORIGINS=${CORS_ORIGINS:-}

# AI
ZHIPU_API_KEY=${ZHIPU_API_KEY:-}
ZHIPU_BASE_URL=${ZHIPU_BASE_URL:-https://open.bigmodel.cn/api/paas/v4}
ZHIPU_MODEL=${ZHIPU_MODEL:-glm-4.7-flash}
OPENAI_API_KEY=${OPENAI_API_KEY:-}

# Coze
COZE_PROJECT_ID=${COZE_PROJECT_ID:-}
COZE_OAUTH_APP_ID=${COZE_OAUTH_APP_ID:-}
COZE_OAUTH_KID=${COZE_OAUTH_KID:-}
COZE_OAUTH_PRIVATE_KEY=${COZE_OAUTH_PRIVATE_KEY:-}
COZE_OAUTH_PRIVATE_KEY_PATH=${COZE_OAUTH_PRIVATE_KEY_PATH:-}

# 后端镜像
BACKEND_IMAGE=${IMAGE_BACKEND}:${IMAGE_TAG}

# 上传
UPLOAD_FOLDER=/data/uploads
VOLUME_MOUNT_PATH=/data
MAX_CONTENT_LENGTH_MB=250

# 评估报告
VALUATION_PDF_OUTPUT_DIR=/data/reports
ENVEOF
    chmod 600 "${DEPLOY_PATH}/.env"
    log_ok ".env 文件已生成"
}

# ======================================================================
# 预部署检查
# ======================================================================
pre_deploy_check() {
    log_info ">>> 预部署检查..."

    check_dependency docker
    check_dependency curl

    # 检查 docker compose 可用性
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose V2 不可用"
        exit 1
    fi

    # 检查部署目录
    if [ ! -d "$DEPLOY_PATH" ]; then
        log_error "部署目录不存在: $DEPLOY_PATH"
        log_info "请先运行 setup-server.sh 初始化服务器"
        exit 1
    fi

    # 检查 compose 文件
    if [ ! -f "$DEPLOY_PATH/$COMPOSE_FILE" ]; then
        log_error "未找到 $COMPOSE_FILE"
        exit 1
    fi

    # 检查磁盘空间（至少 2GB）
    AVAIL_GB=$(df -BG "$DEPLOY_PATH" | awk 'NR==2 {print $4}' | sed 's/G//')
    if [ "${AVAIL_GB:-0}" -lt 2 ]; then
        log_error "磁盘空间不足 (${AVAIL_GB}GB < 2GB)"
        exit 1
    fi

    log_ok "预检查通过 (磁盘: ${AVAIL_GB}GB)"
}

# ======================================================================
# 创建备份
# ======================================================================
create_backup() {
    log_info ">>> 创建部署前备份..."

    TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
    BACKUP_FILE="${BACKUP_DIR}/backup_${TIMESTAMP}.txt"
    DB_BACKUP_FILE="${BACKUP_DIR}/db_backup_${TIMESTAMP}.sql.gz"

    mkdir -p "$BACKUP_DIR"

    # ---- 数据库备份（关键！迁移失败时可恢复） ----
    if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE" &>/dev/null; then
        log_info "备份数据库 (pg_dump + gzip)..."
        if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
            pg_dump -U "${DB_USER:-forklift}" -d forklift_training 2>/dev/null | gzip > "$DB_BACKUP_FILE"; then
            DB_SIZE=$(du -h "$DB_BACKUP_FILE" | cut -f1)
            log_ok "数据库备份完成: $DB_BACKUP_FILE ($DB_SIZE)"
        else
            log_warn "数据库备份失败，继续部署（迁移失败将无法回滚数据）"
            rm -f "$DB_BACKUP_FILE"
        fi
    else
        log_warn "数据库未运行，跳过数据库备份"
    fi

    # ---- 记录当前运行的容器和镜像 ----
    {
        echo "=== 备份时间: $(date) ==="
        echo "=== Git 提交: ${IMAGE_TAG:-unknown} ==="
        echo ""
        echo "--- 运行中的容器 ---"
        docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps 2>/dev/null || echo "无法获取容器状态"
        echo ""
        echo "--- 当前镜像 ---"
        docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" images 2>/dev/null || echo "无法获取镜像信息"
        echo ""
        echo "--- 备份 .env ---"
        if [ -f "$DEPLOY_PATH/.env" ]; then
            # 仅保存非敏感信息
            grep -v -E '(SECRET_KEY|JWT_SECRET_KEY|PASSWORD|API_KEY)' "$DEPLOY_PATH/.env" 2>/dev/null || true
        fi
        echo ""
        if [ -f "$DB_BACKUP_FILE" ]; then
            echo "--- 数据库备份 ---"
            echo "文件: $DB_BACKUP_FILE ($(du -h "$DB_BACKUP_FILE" | cut -f1))"
            echo "恢复命令: gunzip -c $DB_BACKUP_FILE | docker compose -f $DEPLOY_PATH/$COMPOSE_FILE exec -T $POSTGRES_SERVICE psql -U ${DB_USER:-forklift} -d forklift_training"
        fi
    } > "$BACKUP_FILE"

    # 标记当前版本（用于回滚）
    if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" &>/dev/null; then
        CURRENT_IMAGE=$(docker inspect \
            --format='{{.Config.Image}}' \
            "$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE")" 2>/dev/null || echo "unknown")
        echo "PREVIOUS_IMAGE=${CURRENT_IMAGE}" > "${BACKUP_DIR}/last-version.txt"
    fi

    # 清理旧数据库备份（保留最近 10 份，每份约几 MB）
    ls -t "$BACKUP_DIR"/db_backup_*.sql.gz 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null || true

    log_ok "备份完成: $BACKUP_FILE"
}

# ======================================================================
# 登录容器注册表
# ======================================================================
login_registry() {
    log_info ">>> 登录镜像注册表..."

    if [ -n "$GITHUB_TOKEN" ] && [ "$REGISTRY" = "ghcr.io" ]; then
        echo "$GITHUB_TOKEN" | docker login "$REGISTRY" -u "deploy" --password-stdin 2>/dev/null
        log_ok "已登录 $REGISTRY"
    elif [ -n "${REGISTRY_USERNAME:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
        echo "${REGISTRY_PASSWORD}" | docker login "$REGISTRY" -u "${REGISTRY_USERNAME}" --password-stdin 2>/dev/null
        log_ok "已登录 $REGISTRY"
    else
        log_warn "未配置注册表凭据，依赖本地缓存镜像"
    fi
}

# ======================================================================
# 拉取 Docker 镜像
# ======================================================================
pull_images() {
    log_info ">>> 拉取最新镜像..."

    if [ -n "$IMAGE_BACKEND" ]; then
        docker pull "${IMAGE_BACKEND}:${IMAGE_TAG}" || {
            log_error "后端镜像拉取失败: ${IMAGE_BACKEND}:${IMAGE_TAG}"
            exit 1
        }
        log_ok "后端镜像: ${IMAGE_BACKEND}:${IMAGE_TAG}"
    fi

    if [ -n "$IMAGE_FRONTEND" ]; then
        docker pull "${IMAGE_FRONTEND}:${IMAGE_TAG}" || {
            log_warn "前端镜像拉取失败，将跳过: ${IMAGE_FRONTEND}:${IMAGE_TAG}"
        }
        log_ok "前端镜像: ${IMAGE_FRONTEND}:${IMAGE_TAG}"
    fi
}

# ======================================================================
# 数据库迁移
# ======================================================================
run_migration() {
    if [ "$SKIP_MIGRATION" = "true" ]; then
        log_info ">>> 跳过数据库迁移（SKIP_MIGRATION=true）"
        return
    fi

    log_info ">>> 执行数据库迁移..."

    # 确保数据库运行
    if ! docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE" &>/dev/null; then
        log_info "启动数据库服务..."
        docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" up -d "$POSTGRES_SERVICE"
        sleep 5
    fi

    # 等待数据库就绪
    for i in $(seq 1 15); do
        if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
            pg_isready -U "${DB_USER:-forklift}" -d forklift_training &>/dev/null; then
            log_ok "数据库就绪"
            break
        fi
        sleep 2
    done

    # 通过临时容器运行迁移（使用镜像中独立的 migrate 二进制）
    if docker run --rm --network container:"$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE")" \
        -e "DATABASE_URL=${DATABASE_URL}" \
        "${IMAGE_BACKEND}:${IMAGE_TAG}" \
        /app/bin/migrate up 2>/dev/null; then
        log_ok "数据库迁移完成"
    else
        log_warn "自动迁移失败，请手动执行: cd backend-go && go run ./cmd/migrate up"
    fi
}

# ======================================================================
# 重启服务
# ======================================================================
restart_services() {
    log_info ">>> 重启服务..."

    cd "$DEPLOY_PATH"

    # 写入 .env 文件
    write_env_file

    # 重启后端服务（不重启数据库）
    docker compose -f "$COMPOSE_FILE" up -d --remove-orphans "$BACKEND_SERVICE" 2>&1 | tail -5
    log_ok "后端服务已重启"
}

# ======================================================================
# 健康检查
# ======================================================================
health_check() {
    log_info ">>> 健康检查..."

    RETRY=0
    while [ $RETRY -lt $HEALTH_CHECK_RETRIES ]; do
        # 检查容器状态
        BACKEND_STATUS=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null)
        if [ -z "$BACKEND_STATUS" ]; then
            log_error "后端容器未运行!"
            return 1
        fi

        # HTTP 健康检查
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
            --connect-timeout 3 --max-time 5 \
            "http://localhost:${BACKEND_PORT}/api/health" 2>/dev/null || echo "000")

        if [ "$HTTP_CODE" = "200" ]; then
            log_ok "后端健康检查通过 ($HTTP_CODE)"
            return 0
        fi

        RETRY=$((RETRY + 1))
        if [ $((RETRY % 5)) -eq 0 ]; then
            log_warn "等待服务就绪... ($HTTP_CODE) [$RETRY/$HEALTH_CHECK_RETRIES]"
        fi
        sleep "$HEALTH_CHECK_INTERVAL"
    done

    log_error "健康检查超时!"
    return 1
}

# ======================================================================
# 清理旧资源
# ======================================================================
cleanup() {
    log_info ">>> 清理旧资源..."

    # 清理悬空镜像
    DANGLING=$(docker images -f "dangling=true" -q 2>/dev/null | wc -l)
    if [ "$DANGLING" -gt 0 ]; then
        docker image prune -f 2>/dev/null
        log_ok "已清理 $DANGLING 个悬空镜像"
    fi

    # 清理旧备份（保留最近 10 个）
    if [ -d "$BACKUP_DIR" ]; then
        ls -t "$BACKUP_DIR"/backup_*.txt 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null || true
    fi

    # 清理 72 小时前的构建缓存
    docker builder prune -f --filter "until=72h" 2>/dev/null || true

    log_ok "清理完成"
}

# ======================================================================
# 回滚操作
# ======================================================================
do_rollback() {
    log_warn ">>> 执行回滚操作..."

    cd "$DEPLOY_PATH"

    # 读取上一个版本
    if [ -f "${BACKUP_DIR}/last-version.txt" ]; then
        # shellcheck disable=SC1090
        source "${BACKUP_DIR}/last-version.txt"
        if [ -n "${PREVIOUS_IMAGE:-}" ] && [ "$PREVIOUS_IMAGE" != "unknown" ]; then
            log_info "回滚到: $PREVIOUS_IMAGE"

            # 设置环境变量使 compose 使用旧镜像
            export BACKEND_IMAGE="$PREVIOUS_IMAGE"
            write_env_file

            docker compose -f "$COMPOSE_FILE" up -d --remove-orphans "$BACKEND_SERVICE"
            sleep 10

            if health_check; then
                log_ok "回滚成功"
                log_info "如需恢复数据库，可执行："
                log_info "  gunzip -c ${BACKUP_DIR}/db_backup_*.sql.gz | docker compose -f \$DEPLOY_PATH/\$COMPOSE_FILE exec -T \$POSTGRES_SERVICE psql -U ${DB_USER:-forklift} -d forklift_training"
            else
                log_error "回滚后健康检查也失败了! 需要人工介入!"
                log_info "可尝试恢复最近的数据库备份："
                log_info "  ls -t ${BACKUP_DIR}/db_backup_*.sql.gz | head -1"
                exit 1
            fi
        else
            log_error "无法获取上一个版本信息，回滚失败"
            exit 1
        fi
    else
        log_error "未找到备份文件，无法回滚"
        exit 1
    fi
}

# ======================================================================
# 主流程
# ======================================================================
main() {
    echo ""
    echo "=================================================="
    echo "  叉车维修培训系统 - 远程部署"
    echo "  模式: ${MODE}"
    echo "  镜像标签: ${IMAGE_TAG:-latest}"
    echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "=================================================="
    echo ""

    cd "$DEPLOY_PATH"

    case "$MODE" in
        --rollback)
            do_rollback
            ;;
        deploy|*)
            pre_deploy_check
            create_backup
            login_registry
            pull_images
            run_migration
            restart_services

            if ! health_check; then
                log_error "健康检查失败!"
                log_info "尝试回滚..."

                if [ -f "${BACKUP_DIR}/last-version.txt" ]; then
                    do_rollback
                fi
                exit 1
            fi

            cleanup
            log_ok "部署完成!"
            ;;
    esac

    echo ""
}

main
