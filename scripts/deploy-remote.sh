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
REDIS_SERVICE="${REDIS_SERVICE:-redis}"

# 已知容器名（compose 中显式指定），用于部署前清理可能残留的冲突容器
KNOWN_CONTAINERS="forklift-backend-prod forklift-frontend-prod forklift-pg-prod forklift-redis-prod forklift-libreoffice-prod"

# 注册表认证
REGISTRY="${REGISTRY:-ghcr.io}"
IMAGE_BACKEND="${IMAGE_BACKEND:-}"
IMAGE_BACKEND="${IMAGE_BACKEND,,}"  # Docker 镜像名必须全小写
IMAGE_FRONTEND="${IMAGE_FRONTEND:-}"
IMAGE_FRONTEND="${IMAGE_FRONTEND,,}"  # Docker 镜像名必须全小写
IMAGE_LIBREOFFICE="${IMAGE_LIBREOFFICE:-}"
IMAGE_LIBREOFFICE="${IMAGE_LIBREOFFICE,,}"  # Docker 镜像名必须全小写
IMAGE_TAG="${IMAGE_TAG:-latest}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# 镜像加速代理（ghcr.io pull-through 缓存，如 127.0.0.1:5000）
# 设置后镜像地址自动改写为 ${REGISTRY_PROXY}/<org>/<image>，由本地代理缓存加速拉取
REGISTRY_PROXY="${REGISTRY_PROXY:-}"
if [ -n "$REGISTRY_PROXY" ]; then
    REGISTRY_PROXY="${REGISTRY_PROXY%/}"
    # 保存原始镜像名（代理不可用时回退直连 / 清理镜像时覆盖新旧两种路径）
    IMAGE_BACKEND_ORIG="${IMAGE_BACKEND}"
    IMAGE_FRONTEND_ORIG="${IMAGE_FRONTEND}"
    IMAGE_LIBREOFFICE_ORIG="${IMAGE_LIBREOFFICE}"
    case "$IMAGE_BACKEND" in
        ghcr.io/*) IMAGE_BACKEND="${REGISTRY_PROXY}/${IMAGE_BACKEND#ghcr.io/}" ;;
    esac
    case "$IMAGE_FRONTEND" in
        ghcr.io/*) IMAGE_FRONTEND="${REGISTRY_PROXY}/${IMAGE_FRONTEND#ghcr.io/}" ;;
    esac
    case "$IMAGE_LIBREOFFICE" in
        ghcr.io/*) IMAGE_LIBREOFFICE="${REGISTRY_PROXY}/${IMAGE_LIBREOFFICE#ghcr.io/}" ;;
    esac
    log_info "已启用镜像加速代理: ${REGISTRY_PROXY}"
fi

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

        echo "BACKEND_IMAGE=${IMAGE_BACKEND}:${IMAGE_TAG}"
        echo "FRONTEND_IMAGE=${IMAGE_FRONTEND}:${IMAGE_TAG}"
        echo "LIBREOFFICE_IMAGE=${IMAGE_LIBREOFFICE:-forklift-libreoffice}:${IMAGE_TAG}"
        echo "DOMAIN=${DOMAIN:-localhost}"

        echo "UPLOAD_FOLDER=/data/uploads"
        echo "VOLUME_MOUNT_PATH=/data"
        echo "MAX_CONTENT_LENGTH_MB=250"
        echo "VALUATION_PDF_OUTPUT_DIR=/data/reports"
        echo "REDIS_PASSWORD=${REDIS_PASSWORD:-}"
        echo "REDIS_DB=${REDIS_DB:-0}"
        echo "REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-10}"
        echo "REDIS_KEY_PREFIX=${REDIS_KEY_PREFIX:-fl:}"

        echo "# Cloudflare R2 对象存储（留空 STORAGE_DRIVER 或设为 local 则回退到本地磁盘）"
        echo "STORAGE_DRIVER=${STORAGE_DRIVER:-local}"
        printf 'R2_ACCOUNT_ID='
        env_val "${R2_ACCOUNT_ID:-}"; echo
        printf 'R2_ACCESS_KEY_ID='
        env_val "${R2_ACCESS_KEY_ID:-}"; echo
        printf 'R2_SECRET_ACCESS_KEY='
        env_val "${R2_SECRET_ACCESS_KEY:-}"; echo
        echo "R2_BUCKET=${R2_BUCKET:-}"
        printf 'R2_PUBLIC_DOMAIN='
        env_val "${R2_PUBLIC_DOMAIN:-}"; echo

        echo "BACKEND_HOST_PORT=${BACKEND_HOST_PORT:-8080}"

        # Volume 路径配置
        # 默认 named volume（production）；测试环境(Docker 19.03)用 bind mount 绕过 volume 权限 bug
        # 测试环境设置: PG_VOLUME=./data/pgdata, REDIS_VOLUME=./data/redis 等
        echo "PG_VOLUME=${PG_VOLUME:-pgdata-prod}"
        echo "REDIS_VOLUME=${REDIS_VOLUME:-redisdata-prod}"
        echo "UPLOADS_VOLUME=${UPLOADS_VOLUME:-uploads-data}"
        echo "REPORTS_VOLUME=${REPORTS_VOLUME:-reports-data}"
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
            grep -v -E '(SECRET_KEY|JWT_SECRET_KEY|PASSWORD|API_KEY|R2_SECRET|R2_ACCESS_KEY)' "$DEPLOY_PATH/.env" 2>/dev/null || true
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

    # ---- 异地备份:同步到 pve-01（仅 production 环境配置了 BACKUP_REMOTE_HOST 时执行） ----
    if [ -n "${BACKUP_REMOTE_HOST:-}" ] && [ -f "$DB_BACKUP_FILE" ]; then
        log_info "同步数据库备份到 ${BACKUP_REMOTE_HOST}..."
        local remote_dir="${BACKUP_REMOTE_DIR:-/opt/forklift-backups}"
        local ssh_key="${BACKUP_REMOTE_KEY:-/root/.ssh/pve01-sync}"
        if scp -i "$ssh_key" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
            "$DB_BACKUP_FILE" "${BACKUP_REMOTE_HOST}:${remote_dir}/" 2>/dev/null; then
            log_ok "异地备份完成: ${BACKUP_REMOTE_HOST}:${remote_dir}/$(basename "$DB_BACKUP_FILE")"
            # 清理远程旧备份（保留最近 10 份）
            ssh -i "$ssh_key" -o StrictHostKeyChecking=no "$BACKUP_REMOTE_HOST" \
                "ls -t ${remote_dir}/db_backup_*.sql.gz 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null" 2>/dev/null || true
        else
            log_warn "异地备份失败（远端不可达或 SSH key 未配置），继续部署"
        fi
    fi

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
# 确保镜像加速代理可用（本地 pull-through 缓存）
# ======================================================================
ensure_registry_proxy() {
    [ -z "$REGISTRY_PROXY" ] && return 0

    log_info ">>> 检查镜像加速代理: ${REGISTRY_PROXY} ..."

    # 仅当代理是本机回环地址时，自动创建/启动 registry:2 缓存容器
    if [ "$REGISTRY_PROXY" = "127.0.0.1:5000" ]; then
        if ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx 'ghcr-proxy'; then
            log_info "启动 ghcr pull-through 缓存容器 (registry:2)..."
            docker pull registry:2 >/dev/null 2>&1 || true
            docker run -d --name ghcr-proxy --restart unless-stopped \
                -p 127.0.0.1:5000:5000 \
                -e REGISTRY_PROXY_REMOTEURL=https://ghcr.io \
                -v ghcr-cache:/var/lib/registry \
                registry:2 >/dev/null 2>&1 || true
        fi
        if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'ghcr-proxy'; then
            docker start ghcr-proxy >/dev/null 2>&1 || true
        fi
    fi

    # 等待代理就绪（最多 10 秒），失败则回退直连 ghcr.io
    for i in $(seq 1 10); do
        if curl -s -o /dev/null "http://${REGISTRY_PROXY}/v2/"; then
            log_ok "镜像加速代理就绪: ${REGISTRY_PROXY}"
            return 0
        fi
        sleep 1
    done
    log_warn "镜像加速代理未就绪 (${REGISTRY_PROXY})，本次部署回退直连 ghcr.io"
    IMAGE_BACKEND="${IMAGE_BACKEND_ORIG:-$IMAGE_BACKEND}"
    IMAGE_FRONTEND="${IMAGE_FRONTEND_ORIG:-$IMAGE_FRONTEND}"
    IMAGE_LIBREOFFICE="${IMAGE_LIBREOFFICE_ORIG:-$IMAGE_LIBREOFFICE}"
    return 0
}

# ======================================================================
# 拉取 Docker 镜像
# ======================================================================
pull_images() {
    log_info ">>> 拉取最新镜像..."

    # 国内服务器拉取 ghcr.io 镜像易超时,配置客户端选项:
    # - max-concurrent-downloads=5: 并发拉取层数(默认 3,提高可加速)
    # - 通过环境变量 DOCKER_PULL_TIMEOUT 控制单次拉取超时(默认 600s=10min)
    local pull_opts=""
    if [ -n "${DOCKER_PULL_TIMEOUT:-}" ]; then
        pull_opts="--disable-content-trust=false"
    fi

    # 拉取后端镜像(含重试,应对 ghcr.io 国内访问不稳定)
    if [ -n "$IMAGE_BACKEND" ]; then
        local backend_image="${IMAGE_BACKEND}:${IMAGE_TAG}"
        # 本地已有该 tag 则跳过拉取（重复部署/代理缓存命中时显著提速）
        if docker image inspect "$backend_image" &>/dev/null; then
            log_ok "后端镜像已缓存，跳过拉取: $backend_image"
        else
            local pull_retries=3
            local pull_ok=false

            for attempt in $(seq 1 $pull_retries); do
                log_info "拉取后端镜像 (尝试 $attempt/$pull_retries): $backend_image"
                if docker pull "$backend_image"; then
                    pull_ok=true
                    break
                fi
                if [ $attempt -lt $pull_retries ]; then
                    log_warn "拉取失败,10 秒后重试..."
                    sleep 10
                fi
            done

            if [ "$pull_ok" = "true" ]; then
                log_ok "后端镜像: $backend_image"
            else
                log_error "后端镜像拉取失败(已重试 $pull_retries 次): $backend_image"
                exit 1
            fi
        fi
    fi

    # 拉取前端镜像(前端镜像小,通常无需重试)
    if [ -n "$IMAGE_FRONTEND" ]; then
        local frontend_image="${IMAGE_FRONTEND}:${IMAGE_TAG}"
        # 本地已有该 tag 则跳过拉取
        if docker image inspect "$frontend_image" &>/dev/null; then
            log_ok "前端镜像已缓存，跳过拉取: $frontend_image"
        else
            local pull_retries=3
            local pull_ok=false

            for attempt in $(seq 1 $pull_retries); do
                log_info "拉取前端镜像 (尝试 $attempt/$pull_retries): $frontend_image"
                if docker pull "$frontend_image"; then
                    pull_ok=true
                    break
                fi
                if [ $attempt -lt $pull_retries ]; then
                    log_warn "拉取失败,10 秒后重试..."
                    sleep 10
                fi
            done

            if [ "$pull_ok" = "true" ]; then
                log_ok "前端镜像: $frontend_image"
            else
                log_error "前端镜像拉取失败(已重试 $pull_retries 次): $frontend_image"
                exit 1
            fi
        fi
    fi

    # 拉取 LibreOffice sidecar 镜像(含 LibreOffice,体积大,需重试)
    if [ -n "$IMAGE_LIBREOFFICE" ]; then
        local lo_image="${IMAGE_LIBREOFFICE}:${IMAGE_TAG}"
        # 本地已有该 tag 则跳过拉取
        if docker image inspect "$lo_image" &>/dev/null; then
            log_ok "LibreOffice sidecar 镜像已缓存，跳过拉取: $lo_image"
        else
            local pull_retries=3
            local pull_ok=false

            for attempt in $(seq 1 $pull_retries); do
                log_info "拉取 LibreOffice sidecar 镜像 (尝试 $attempt/$pull_retries): $lo_image"
                if docker pull "$lo_image"; then
                    pull_ok=true
                    break
                fi
                if [ $attempt -lt $pull_retries ]; then
                    log_warn "拉取失败,10 秒后重试..."
                    sleep 10
                fi
            done

            if [ "$pull_ok" = "true" ]; then
                log_ok "LibreOffice sidecar 镜像: $lo_image"
            else
                log_error "LibreOffice sidecar 镜像拉取失败(已重试 $pull_retries 次): $lo_image"
                exit 1
            fi
        fi
    fi

    # ---- 拉取基础镜像（postgres + redis）----
    # compose up 时若缺少基础镜像会自动拉取，但国内访问 Docker Hub 易超时
    # 显式拉取可走 daemon.json 配置的国内镜像源，且能利用重试逻辑
    local base_images="postgres:15-alpine redis:7-alpine"
    for img in $base_images; do
        if docker image inspect "$img" &>/dev/null; then
            log_ok "基础镜像已缓存: $img"
            continue
        fi
        log_info "拉取基础镜像: $img"
        if docker pull "$img"; then
            log_ok "基础镜像: $img"
        else
            log_error "基础镜像拉取失败: $img"
            exit 1
        fi
    done
}

# ======================================================================
# 修复 dirty 数据库状态
# 当 schema_migrations.dirty=true 时,数据库因迁移中断进入脏状态,
# 后续 migrate up 会被拒绝。此函数自动检测并 force 到 version-1 清除 dirty。
# ======================================================================
fix_dirty_state() {
    log_info ">>> 检查数据库迁移状态..."

    local pg_id
    pg_id=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE" 2>/dev/null)
    if [ -z "$pg_id" ]; then
        log_warn "PostgreSQL 容器未运行,跳过 dirty 检查"
        return 0
    fi

    # 查询当前版本和 dirty 状态
    # 注意：全新数据库 schema_migrations 表尚未创建，psql 会报错返回非零；
    # 在 set -euo pipefail 下需用 || true 防止脚本退出
    local cur_ver dirty
    cur_ver=$(docker exec "$pg_id" psql -U "${DB_USER:-forklift}" -d forklift_training \
        -tAc "SELECT COALESCE(version,0) FROM schema_migrations LIMIT 1;" 2>/dev/null | tr -d ' \n' || true)
    dirty=$(docker exec "$pg_id" psql -U "${DB_USER:-forklift}" -d forklift_training \
        -tAc "SELECT COALESCE(dirty,false) FROM schema_migrations LIMIT 1;" 2>/dev/null | tr -d ' \n' || true)
    cur_ver=${cur_ver:-0}
    dirty=${dirty:-f}

    if [ "$dirty" != "t" ] && [ "$dirty" != "true" ]; then
        log_ok "数据库迁移状态正常 (version=$cur_ver, dirty=false)"
        return 0
    fi

    log_warn "数据库处于 dirty 状态 (version=$cur_ver, dirty=true)"
    log_info "自动修复:执行 migrate force $((cur_ver - 1)) 清除 dirty 标志"

    local force_ver=$((cur_ver - 1))
    if [ "$force_ver" -lt 0 ]; then
        force_ver=0
    fi

    local migrate_db_url
    migrate_db_url="postgres://${DB_USER:-forklift}:${DB_PASSWORD}@localhost:5432/forklift_training?sslmode=disable"

    if docker run --rm --network container:"$pg_id" \
        -e "DATABASE_URL=${migrate_db_url}" \
        "${IMAGE_BACKEND}:${IMAGE_TAG}" \
        /app/bin/migrate force "$force_ver" 2>&1; then
        log_ok "已清除 dirty 标志 (force 到版本 $force_ver)"
    else
        log_error "自动 force 失败,需手动修复"
        log_info "手动修复命令:"
        log_info "  docker exec $pg_id psql -U ${DB_USER:-forklift} -d forklift_training -c \"UPDATE schema_migrations SET version=$force_ver, dirty=false;\""
        return 1
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
    wait_postgres

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
# 等待数据库就绪
# ======================================================================
wait_postgres() {
    log_info ">>> 等待数据库就绪..."
    for i in $(seq 1 15); do
        if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
            pg_isready -U "${DB_USER:-forklift}" -d forklift_training &>/dev/null; then
            log_ok "数据库就绪"
            return 0
        fi
        sleep 2
    done
    log_warn "数据库在超时时间内未就绪，部分依赖数据库的步骤可能失败"
    return 1
}

# ======================================================================
# 迁移兼容性预检（防止旧镜像启动后因缺少迁移文件崩溃循环）
#   后端 cmd/server 启动时会自动执行 migrate up；若镜像内迁移文件版本
#   落后于数据库当前版本，会报 "no migration found for version N" 并退出。
#   预检在启动后端容器前拦截这种情况。
# 返回: 0=通过/可跳过, 1=镜像迁移版本落后，应终止部署
# ======================================================================
preflight_migration_check() {
    local image="${1:-}"
    [ -z "$image" ] && { log_warn "未指定镜像，跳过迁移预检"; return 0; }

    log_info ">>> 迁移兼容性预检: $image"

    # 1. 镜像内最大迁移版本（统计 /app/migrations/*.up.sql 的 6 位序号）
    local img_max="0"
    if docker image inspect "$image" &>/dev/null; then
        img_max=$(docker run --rm --entrypoint sh "$image" -c \
            'ls /app/migrations/*.up.sql 2>/dev/null | grep -oE "[0-9]{6}" | sort -n | tail -1' 2>/dev/null | tr -d ' \n')
        img_max=${img_max:-0}
    else
        log_warn "本地无镜像 $image，无法读取其迁移版本，跳过预检（请确认镜像已拉取）"
        return 0
    fi

    # 2. 数据库当前版本（schema_migrations.version）
    local db_ver="0"
    local pg_id
    pg_id=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE" 2>/dev/null)
    if [ -n "$pg_id" ]; then
        db_ver=$(docker exec "$pg_id" psql -U "${DB_USER:-forklift}" -d forklift_training \
            -tAc "SELECT COALESCE(version,0) FROM schema_migrations LIMIT 1;" 2>/dev/null | tr -d ' \n')
        db_ver=${db_ver:-0}
    fi

    log_info "镜像最大迁移版本=$img_max, 数据库当前版本=$db_ver"

    if [ "${db_ver:-0}" -gt "${img_max:-0}" ]; then
        log_error "❌ 镜像迁移版本($img_max)落后于数据库($db_ver)!"
        log_error "   该镜像启动后会因缺少迁移文件而崩溃循环 (no migration found for version ...)。"
        log_error "   请重新构建并推送包含最新迁移文件(直至 v${db_ver})的后端镜像，再部署/回滚。"
        return 1
    fi
    log_ok "迁移兼容性预检通过"
    return 0
}

# ======================================================================
# 重启服务
# ======================================================================
restart_services() {
    log_info ">>> 重启服务..."

    cd "$DEPLOY_PATH"

    # 移除可能残留的冲突容器（同名但非本 compose 项目管理的容器会导致 up 失败）
    for c in $KNOWN_CONTAINERS; do
        if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx "$c"; then
            log_warn "移除残留容器: $c"
            docker rm -f "$c" 2>/dev/null || true
        fi
    done

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

    log_ok "全栈服务已重启"
}

# ======================================================================
# 健康检查
# ======================================================================
health_check() {
    log_info ">>> 后端健康检查 (localhost:8080/api/health)..."

    RETRY=0
    while [ $RETRY -lt $HEALTH_CHECK_RETRIES ]; do
        # 检查容器状态
        BACKEND_STATUS=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null)
        if [ -z "$BACKEND_STATUS" ]; then
            log_error "后端容器未运行!"
            return 1
        fi

        # HTTP 健康检查（通过容器内部 wget，避免宿主机端口冲突）
        if docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" exec -T "$BACKEND_SERVICE" \
            wget -qO- http://localhost:8080/api/health 2>/dev/null | grep -q '"status":"ok"'; then
            HTTP_CODE="200"
        else
            HTTP_CODE="000"
        fi

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
    log_info ">>> 前端健康检查..."

    FRONTEND_STATUS=$(docker compose -f "$DEPLOY_PATH/$COMPOSE_FILE" ps -q "$FRONTEND_SERVICE" 2>/dev/null)
    if [ -z "$FRONTEND_STATUS" ]; then
        log_error "前端容器未运行!"
        return 1
    fi
    log_ok "前端容器运行中"
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

    # 清理旧版本镜像（保留最近 KEEP_IMAGES 个）
    prune_old_images

    # 清理旧备份（保留最近 10 个）
    if [ -d "$BACKUP_DIR" ]; then
        ls -t "$BACKUP_DIR"/backup_*.txt 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null || true
    fi

    # 清理 72 小时前的构建缓存
    docker builder prune -f --filter "until=72h" 2>/dev/null || true

    log_ok "清理完成"
}

# ======================================================================
# 清理旧版本镜像（按仓库保留最近 KEEP_IMAGES 个）
# ======================================================================
prune_old_images() {
    if [ "${KEEP_IMAGES:-3}" -le 0 ]; then
        log_info ">>> KEEP_IMAGES<=0，跳过旧镜像清理"
        return 0
    fi
    log_info ">>> 清理旧版本镜像 (每个仓库保留最近 ${KEEP_IMAGES} 个)..."

    local repo
    for repo in "${IMAGE_BACKEND}" "${IMAGE_FRONTEND}" "${IMAGE_LIBREOFFICE}" \
                "${IMAGE_BACKEND_ORIG:-}" "${IMAGE_FRONTEND_ORIG:-}" "${IMAGE_LIBREOFFICE_ORIG:-}"; do
        [ -z "$repo" ] && continue
        # 按创建时间倒序列出该仓库的镜像，保留前 KEEP_IMAGES 个，其余删除
        local kept=0
        local created id ref
        while IFS='|' read -r created id ref; do
            [ -z "$ref" ] && continue
            kept=$((kept + 1))
            if [ "$kept" -le "$KEEP_IMAGES" ]; then
                continue
            fi
            log_info "删除旧镜像: ${ref} (${id})"
            docker image rm "${id}" >/dev/null 2>&1 || true
        done < <(docker images --format '{{.CreatedAt}}|{{.ID}}|{{.Repository}}:{{.Tag}}' "$repo" 2>/dev/null | grep -v '|<none>' | sort -r)
    done

    log_ok "旧镜像清理完成"
}

# ======================================================================
# 回滚操作
# ======================================================================
do_rollback() {
    log_warn ">>> 执行回滚操作..."

    cd "$DEPLOY_PATH"

    # 读取上一个版本
    if [ ! -f "${BACKUP_DIR}/last-version.txt" ]; then
        log_error "未找到备份文件，无法回滚"
        exit 1
    fi

    # shellcheck disable=SC1090
    source "${BACKUP_DIR}/last-version.txt"

    # 记录当前实际运行的镜像（回滚失败时保持，避免 .env 被污染为不兼容镜像）
    local cur_be cur_fe
    cur_be=$(docker compose -f "$COMPOSE_FILE" ps -q "$BACKEND_SERVICE" 2>/dev/null | head -1)
    cur_be=$([ -n "$cur_be" ] && docker inspect --format='{{.Config.Image}}' "$cur_be" 2>/dev/null || echo "unknown")
    cur_fe=$(docker compose -f "$COMPOSE_FILE" ps -q "$FRONTEND_SERVICE" 2>/dev/null | head -1)
    cur_fe=$([ -n "$cur_fe" ] && docker inspect --format='{{.Config.Image}}' "$cur_fe" 2>/dev/null || echo "unknown")

    # 启动数据库以便迁移预检读取当前版本
    docker compose -f "$COMPOSE_FILE" up -d "$POSTGRES_SERVICE" 2>&1 | tail -3 || true
    wait_postgres

    # 回滚 backend（先做迁移兼容性预检，避免回滚到旧镜像后崩溃循环）
    if [ "${PREVIOUS_BACKEND_IMAGE:-unknown}" != "unknown" ]; then
        if preflight_migration_check "${PREVIOUS_BACKEND_IMAGE}"; then
            log_info "回滚后端到: $PREVIOUS_BACKEND_IMAGE"
            export BACKEND_IMAGE="${PREVIOUS_BACKEND_IMAGE}"
            write_env_file
            docker compose -f "$COMPOSE_FILE" down "$BACKEND_SERVICE" 2>&1 || true
            sleep 3
            docker compose -f "$COMPOSE_FILE" up -d "$BACKEND_SERVICE" 2>&1
        else
            log_error "❌ 拒绝回滚后端到 ${PREVIOUS_BACKEND_IMAGE}（迁移版本落后数据库，会崩溃循环）"
            log_error "   保持当前后端镜像: ${cur_be}"
            export BACKEND_IMAGE="${cur_be}"
            write_env_file
        fi
    else
        log_warn "无后端历史版本，跳过后端回滚"
    fi

    # 回滚 frontend
    if [ "${PREVIOUS_FRONTEND_IMAGE:-unknown}" != "unknown" ]; then
        log_info "回滚前端到: $PREVIOUS_FRONTEND_IMAGE"
        export FRONTEND_IMAGE="${PREVIOUS_FRONTEND_IMAGE}"
        write_env_file
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
            # host 网络模式下 nginx-host.conf 是 HTTP-only，不需要 SSL 证书
            # 若未来切回 bridge + HTTPS，可重新启用 write_ssl_certs
            # write_ssl_certs
            create_backup
            login_registry
            ensure_registry_proxy
            pull_images

            # 先拉起数据库与 Redis，供迁移预检读取当前版本
            docker compose -f "$COMPOSE_FILE" up -d "$POSTGRES_SERVICE" "$REDIS_SERVICE" 2>&1 | tail -5 || true
            wait_postgres

            # 迁移兼容性预检：若镜像迁移版本落后数据库，启动后会崩溃循环
            if ! preflight_migration_check "${IMAGE_BACKEND}:${IMAGE_TAG}"; then
                log_error "迁移预检失败，终止部署（避免后端因缺少迁移文件崩溃循环）"
                exit 1
            fi

            # 修复 dirty 数据库状态（必须在 restart_services 之前执行）
            # 否则 backend 启动时看到 dirty=true 会崩溃循环
            fix_dirty_state

            # 重启全栈（postgres/redis 已运行，此处拉起 backend + frontend）
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
