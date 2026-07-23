#!/usr/bin/env bash
# ======================================================================
# deploy-remote.sh v3 — 远程服务器端全栈部署脚本
# [env_val 版本] — 使用 env_val() 安全转义 .env 值
# ======================================================================
# 部署前端（Nginx）+ 后端（Go API）。
echo "[deploy-remote.sh] 版本: env_val v3 (全栈)"
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
FRONTEND_SERVICE="${FRONTEND_SERVICE:-frontend}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"

# 注册表认证
REGISTRY="${REGISTRY:-ghcr.io}"
IMAGE_BACKEND="${IMAGE_BACKEND:-}"
IMAGE_BACKEND="${IMAGE_BACKEND,,}"  # Docker 镜像名必须全小写
IMAGE_FRONTEND="${IMAGE_FRONTEND:-}"
IMAGE_FRONTEND="${IMAGE_FRONTEND,,}"  # Docker 镜像名必须全小写
IMAGE_TAG="${IMAGE_TAG:-latest}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# 健康检查
HEALTH_CHECK_RETRIES="${HEALTH_CHECK_RETRIES:-20}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-6}"

# SSL 证书目录（固定路径，由 write_ssl_certs() 写入，frontend 容器挂载）
SSL_CERT_DIR="${DEPLOY_PATH}/nginx/ssl"

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
    # Docker Compose .env 语法：简单值直接写，含特殊字符的值用双引号包裹
    # 多行值（如 PEM 密钥）将换行替换为 \n
    env_val() {
        local val="$1"
        if [ -z "$val" ]; then
            printf '""'
        elif printf '%s' "$val" | grep -q "[[:space:]\$+#{}()&|!<>'\";=]" 2>/dev/null; then
            # 含特殊字符：双引号包裹，换行转 \n（Docker Compose .env 不支持裸换行）
            printf '"'
            if command -v python3 >/dev/null 2>&1; then
                printf '%s' "$val" | python3 -c "import sys; sys.stdout.write(sys.stdin.read().rstrip('\n').replace('\n', r'\n'))"
            else
                # fallback: 用 perl（更常见于最小化安装）
                printf '%s' "$val" | perl -pe 's/\n/\\n/g' | head -c -2
            fi
            printf '"'
        else
            printf '%s' "$val"
        fi
    }

    # PEM 私钥写入独立文件（必须在 {} > .env 块之外，防止 log_ok ANSI 码污染 .env）
    if [ -n "${COZE_OAUTH_PRIVATE_KEY:-}" ]; then
        printf '%s' "${COZE_OAUTH_PRIVATE_KEY}" > "${DEPLOY_PATH}/coze_private_key.pem"
        chmod 600 "${DEPLOY_PATH}/coze_private_key.pem"
        log_ok "Coze 私钥已写入文件"
    fi

    {
        echo "# 由 deploy-remote.sh 自动生成 — $(date '+%Y-%m-%d %H:%M:%S')"
        echo "APP_ENV=production"
        echo "PORT=8080"

        printf 'DATABASE_URL='
        env_val "${DATABASE_URL:-}"; echo
        printf 'DB_USER='
        env_val "${DB_USER:-forklift}"; echo
        printf 'DB_PASSWORD='
        env_val "${DB_PASSWORD:-}"; echo

        printf 'SECRET_KEY='
        env_val "${SECRET_KEY:-}"; echo
        printf 'JWT_SECRET_KEY='
        env_val "${JWT_SECRET_KEY:-}"; echo
        echo "JWT_EXPIRES_HOURS=${JWT_EXPIRES_HOURS:-24}"

        printf 'ADMIN_DEFAULT_PASSWORD='
        env_val "${ADMIN_DEFAULT_PASSWORD:-}"; echo
        printf 'TUTOR_DEFAULT_PASSWORD='
        env_val "${TUTOR_DEFAULT_PASSWORD:-}"; echo
        printf 'STUDENT_DEFAULT_PASSWORD='
        env_val "${STUDENT_DEFAULT_PASSWORD:-}"; echo

        printf 'CORS_ORIGINS='
        env_val "${CORS_ORIGINS:-}"; echo

        printf 'ZHIPU_API_KEY='
        env_val "${ZHIPU_API_KEY:-}"; echo
        printf 'ZHIPU_BASE_URL='
        env_val "${ZHIPU_BASE_URL:-https://open.bigmodel.cn/api/paas/v4}"; echo
        printf 'ZHIPU_MODEL='
        env_val "${ZHIPU_MODEL:-glm-4.7-flash}"; echo
        printf 'OPENAI_API_KEY='
        env_val "${OPENAI_API_KEY:-}"; echo

        printf 'COZE_PROJECT_ID='
        env_val "${COZE_PROJECT_ID:-}"; echo
        printf 'COZE_OAUTH_APP_ID='
        env_val "${COZE_OAUTH_APP_ID:-}"; echo
        printf 'COZE_OAUTH_KID='
        env_val "${COZE_OAUTH_KID:-}"; echo
        echo "COZE_OAUTH_PRIVATE_KEY_PATH=/etc/secrets/coze_private_key.pem"
        # COZE_OAUTH_PRIVATE_KEY 不写入 .env（已写入独立文件，见上方）

        echo "# 残值评估 JWT 密钥（生产环境必需）"
        printf 'VALUATION_JWT_SECRET_KEY='
        env_val "${VALUATION_JWT_SECRET_KEY:-}"; echo

        echo "BACKEND_IMAGE=${IMAGE_BACKEND}:${IMAGE_TAG}"
        echo "FRONTEND_IMAGE=${IMAGE_FRONTEND}:${IMAGE_TAG}"
        echo "DOMAIN=${DOMAIN:-localhost}"

        echo "UPLOAD_FOLDER=/data/uploads"
        echo "VOLUME_MOUNT_PATH=/data"
        echo "MAX_CONTENT_LENGTH_MB=250"
        echo "VALUATION_PDF_OUTPUT_DIR=/data/reports"
        echo "REDIS_PASSWORD=${REDIS_PASSWORD:-}"
        echo "REDIS_DB=${REDIS_DB:-0}"
        echo "REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-10}"
        echo "REDIS_KEY_PREFIX=${REDIS_KEY_PREFIX:-fl:}"
    } > "${DEPLOY_PATH}/.env.tmp"
    rm -f "${DEPLOY_PATH}/.env"
    mv "${DEPLOY_PATH}/.env.tmp" "${DEPLOY_PATH}/.env"
    chmod 600 "${DEPLOY_PATH}/.env"
    log_ok ".env 文件已生成（$(wc -l < "${DEPLOY_PATH}/.env") 行）"
}

# ======================================================================
# 写入 SSL 证书文件（从 GitHub Secrets 注入的内容）
# ======================================================================
write_ssl_certs() {
    log_info ">>> 检查 SSL 证书..."

    mkdir -p "$SSL_CERT_DIR"

    # 检查证书内容是否已通过环境变量注入
    if [ -z "${SSL_FULLCHAIN:-}" ] || [ -z "${SSL_PRIVKEY:-}" ]; then
        log_warn "未通过环境变量提供 SSL 证书内容（SSL_FULLCHAIN/SSL_PRIVKEY）"
        # 检查证书文件是否已存在（可能是手动上传的）
        if [ -f "${SSL_CERT_DIR}/fullchain.pem" ] && [ -f "${SSL_CERT_DIR}/privkey.pem" ]; then
            log_ok "检测到已有证书文件，将复用: $SSL_CERT_DIR"
        else
            log_error "SSL 证书文件不存在且未通过 Secrets 注入: $SSL_CERT_DIR"
            log_info "请在 GitHub Secrets 中配置 SSL_FULLCHAIN 和 SSL_PRIVKEY，或手动上传证书文件"
            exit 1
        fi
        return
    fi

    # 写入证书文件（环境变量中的换行符已通过 printf %q 还原）
    printf '%s\n' "${SSL_FULLCHAIN}" > "${SSL_CERT_DIR}/fullchain.pem"
    printf '%s\n' "${SSL_PRIVKEY}" > "${SSL_CERT_DIR}/privkey.pem"

    # 设置权限（证书文件可读，私钥仅 owner 可读）
    chmod 644 "${SSL_CERT_DIR}/fullchain.pem"
    chmod 600 "${SSL_CERT_DIR}/privkey.pem"

    # 验证证书内容有效
    if ! openssl x509 -in "${SSL_CERT_DIR}/fullchain.pem" -noout 2>/dev/null; then
        log_error "证书文件无效（非合法的 X.509 格式）: ${SSL_CERT_DIR}/fullchain.pem"
        log_info "请检查 GitHub Secret SSL_FULLCHAIN 的内容是否完整（含 BEGIN/END CERTIFICATE 行）"
        exit 1
    fi

    log_ok "SSL 证书已从 GitHub Secrets 写入: $SSL_CERT_DIR"
    log_info "证书有效期至: $(openssl x509 -in "${SSL_CERT_DIR}/fullchain.pem" -noout -enddate 2>/dev/null | cut -d= -f2)"
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
    BACKEND_RUNNING=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null)
    FRONTEND_RUNNING=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$FRONTEND_SERVICE" 2>/dev/null)

    PREVIOUS_BACKEND="unknown"
    PREVIOUS_FRONTEND="unknown"
    if [ -n "$BACKEND_RUNNING" ]; then
        PREVIOUS_BACKEND=$(docker inspect \
            --format='{{.Config.Image}}' \
            "$BACKEND_RUNNING" 2>/dev/null || echo "unknown")
    fi
    if [ -n "$FRONTEND_RUNNING" ]; then
        PREVIOUS_FRONTEND=$(docker inspect \
            --format='{{.Config.Image}}' \
            "$FRONTEND_RUNNING" 2>/dev/null || echo "unknown")
    fi

    {
        echo "PREVIOUS_BACKEND_IMAGE=${PREVIOUS_BACKEND}"
        echo "PREVIOUS_FRONTEND_IMAGE=${PREVIOUS_FRONTEND}"
    } > "${BACKUP_DIR}/last-version.txt"

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
            log_error "前端镜像拉取失败: ${IMAGE_FRONTEND}:${IMAGE_TAG}"
            exit 1
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
    # 用 DB_USER/DB_PASSWORD 拼接连接串（和 docker-compose 一致），不依赖 GitHub Secret 的 DATABASE_URL
    MIGRATE_DB_URL="postgres://${DB_USER:-forklift}:${DB_PASSWORD}@localhost:5432/forklift_training?sslmode=disable"
    log_info "运行迁移: ${IMAGE_BACKEND}:${IMAGE_TAG} /app/bin/migrate up"
    if docker run --rm --network container:"$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE")" \
        -e "DATABASE_URL=${MIGRATE_DB_URL}" \
        "${IMAGE_BACKEND}:${IMAGE_TAG}" \
        /app/bin/migrate up 2>&1; then
        log_ok "数据库迁移完成"
    else
        log_warn "自动迁移失败，请手动执行: cd backend && go run ./cmd/migrate up"
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

    # 启动服务（不做健康等待，由后续 health_check() 统一处理）
    docker compose -f "$COMPOSE_FILE" up -d --wait-timeout 1 --remove-orphans 2>&1 | tail -10 || true
    log_info "等待容器稳定 (10s)..."
    sleep 10

    # 快速诊断：如果 backend 不在 running 状态，打日志
    local be_stat
    be_stat=$(docker compose -f "$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null)
    if [ -z "$be_stat" ]; then
        log_warn "后端容器未创建，最后 30 行日志："
        docker compose -f "$COMPOSE_FILE" logs --tail 30 "$BACKEND_SERVICE" 2>&1 || echo "  无法��取日志"
        echo ""
    fi
        echo ""
    fi

    log_ok "全栈服务已重启"
}

# ======================================================================
# 健康检查
# ======================================================================
health_check() {
    log_info ">>> 后端健康检查（通过前端容器反代 localhost/api/health）..."

    RETRY=0
    while [ $RETRY -lt $HEALTH_CHECK_RETRIES ]; do
        # 检查容器状态
        BACKEND_STATUS=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null)
        if [ -z "$BACKEND_STATUS" ]; then
            log_error "后端容器未运行!"
            return 1
        fi

        # HTTP 健康检查（backend 不再对外暴露端口，通过前端 80 端口反代检查）
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
            --connect-timeout 3 --max-time 5 \
            "http://localhost/api/health" 2>/dev/null || echo "000")

        if [ "$HTTP_CODE" = "200" ]; then
            log_ok "后端健康检查通过 ($HTTP_CODE)"
            break
        fi

        RETRY=$((RETRY + 1))
        if [ $((RETRY % 5)) -eq 0 ]; then
            log_warn "等待服务就绪... ($HTTP_CODE) [$RETRY/$HEALTH_CHECK_RETRIES]"
        fi
        sleep "$HEALTH_CHECK_INTERVAL"
    done

    # 后端检查未通过
    if [ "$HTTP_CODE" != "200" ]; then
        log_error "后端健康检查超时!"
        echo ""
        echo "=== 后端容器日志（最后 30 行）==="
        docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" logs --tail 30 "$BACKEND_SERVICE" 2>&1 || echo "无法获取日志"
        echo ""
        echo "=== 容器状态 ==="
        docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps 2>&1
        return 1
    fi

    # ===== 前端健康检查 =====
    log_info ">>> 前端健康检查 (localhost:80/health)..."

    FRONTEND_RETRY=0
    FRONTEND_MAX_RETRIES=10
    while [ $FRONTEND_RETRY -lt $FRONTEND_MAX_RETRIES ]; do
        FRONTEND_STATUS=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$FRONTEND_SERVICE" 2>/dev/null)
        if [ -z "$FRONTEND_STATUS" ]; then
            log_error "前端容器未运行!"
            echo ""
            echo "=== 前端容器日志（最后 30 行）==="
            docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" logs --tail 30 "$FRONTEND_SERVICE" 2>&1 || echo "无法获取日志"
            return 1
        fi

        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
            --connect-timeout 3 --max-time 5 \
            "http://localhost:80/health" 2>/dev/null || echo "000")

        if [ "$HTTP_CODE" = "200" ]; then
            log_ok "前端健康检查通过 ($HTTP_CODE)"
            return 0
        fi

        FRONTEND_RETRY=$((FRONTEND_RETRY + 1))
        sleep 3
    done

    log_error "前端健康检查超时!"
    echo ""
    echo "=== 前端容器日志（最后 30 行）==="
    docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" logs --tail 30 "$FRONTEND_SERVICE" 2>&1 || echo "无法获取日志"
    echo ""
    echo "=== 容器状态 ==="
    docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps 2>&1
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

        # 设置环境变量使 compose 使用旧镜像
        export BACKEND_IMAGE="${PREVIOUS_BACKEND_IMAGE:-unknown}"
        export FRONTEND_IMAGE="${PREVIOUS_FRONTEND_IMAGE:-unknown}"
        write_env_file

        # 回滚 backend
        if [ "${PREVIOUS_BACKEND_IMAGE:-unknown}" != "unknown" ]; then
            log_info "回滚后端到: $PREVIOUS_BACKEND_IMAGE"
            docker compose -f "$COMPOSE_FILE" down "$BACKEND_SERVICE" 2>&1 || true
            sleep 3
            docker compose -f "$COMPOSE_FILE" up -d "$BACKEND_SERVICE" 2>&1
        else
            log_warn "无后端历史版本，跳过后端回滚"
        fi

        # 回滚 frontend
        if [ "${PREVIOUS_FRONTEND_IMAGE:-unknown}" != "unknown" ]; then
            log_info "回滚前端到: $PREVIOUS_FRONTEND_IMAGE"
            docker compose -f "$COMPOSE_FILE" down "$FRONTEND_SERVICE" 2>&1 || true
            sleep 2
            docker compose -f "$COMPOSE_FILE" up -d "$FRONTEND_SERVICE" 2>&1
        else
            log_warn "无前端历史版本，跳过前端回滚"
        fi

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
            write_env_file
            write_ssl_certs
            create_backup
            login_registry
            pull_images
            # 先重启（启动 postgres + redis + backend，创建网络），再迁移
            restart_services
            run_migration

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
