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
# 各镜像独立标签（CD 传入内容哈希标签：未变更的镜像跳过构建/拉取）；缺省统一用 IMAGE_TAG
IMAGE_TAG_BACKEND="${IMAGE_TAG_BACKEND:-$IMAGE_TAG}"
IMAGE_TAG_FRONTEND="${IMAGE_TAG_FRONTEND:-$IMAGE_TAG}"
IMAGE_TAG_LIBREOFFICE="${IMAGE_TAG_LIBREOFFICE:-$IMAGE_TAG}"
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

# ghcr 国内镜像源（南大 ghcr.nju.edu.cn，透传 ghcr 认证可拉私有镜像）：
# 晚高峰 ghcr CDN 限速时代理回源/直连均易超时（曾致 testing CD 连续失败），
# 作为拉取回退链路的第一回退（代理 → 镜像源 → 直连 ghcr.io，见 pull_one）。
# 置空禁用；仅对 ghcr.io 镜像生效。
REGISTRY_MIRROR="${REGISTRY_MIRROR:-ghcr.nju.edu.cn}"

# 健康检查
HEALTH_CHECK_RETRIES="${HEALTH_CHECK_RETRIES:-20}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-6}"

# 跳过部署期备份（测试环境专用加速开关：回滚保障由 RBD 快照 + 每日离线备份承担，
# 不再在每次部署关键路径上做 pg_dump；production 保持 false）
SKIP_BACKUP="${SKIP_BACKUP:-false}"

# compose up --wait 超时（秒）：容器就绪等待上限，超时后由 health_check() 轮询兜底
UP_WAIT_TIMEOUT="${UP_WAIT_TIMEOUT:-60}"

# compose profile 参数：production 用 COMPOSE_PROFILES=full 启用 libreoffice sidecar；
# 留空（testing）则跳过 500MB 大镜像服务。转成 CLI --profile（部分 compose 版本
# 不识别 COMPOSE_PROFILES 环境变量）
COMPOSE_PROFILE_ARGS=""
if [ -n "${COMPOSE_PROFILES:-}" ]; then
    COMPOSE_PROFILE_ARGS="--profile ${COMPOSE_PROFILES}"
fi

# 后台备份 join 超时（秒）：迁移前等待备份完成的上限
BACKUP_WAIT_TIMEOUT="${BACKUP_WAIT_TIMEOUT:-120}"

# SSL 证书目录（固定路径，由 write_ssl_certs() 写入，frontend 容器挂载）
SSL_CERT_DIR="${DEPLOY_PATH}/nginx/ssl"

# 迁移
SKIP_MIGRATION="${SKIP_MIGRATION:-false}"

# 清理策略：保留最近 N 个镜像
KEEP_IMAGES="${KEEP_IMAGES:-3}"

# ghcr pull-through 缓存上限（MB）：缓存超阈值即清空重建（清空后拉取按需回源）。
# 默认 15GB——服务器磁盘 30G（约 24G 可用），6GB 旧阈值导致清空过频、
# 每次清空后 testing 部署需全量回源（实测拉取 43 分钟）
CACHE_LIMIT_MB="${CACHE_LIMIT_MB:-15000}"

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

        # SMTP 邮件（验证码通道；腾讯企业邮 SSL 465）
        printf 'SMTP_HOST='
        env_val "${SMTP_HOST:-}"; echo
        echo "SMTP_PORT=${SMTP_PORT:-465}"
        printf 'SMTP_USERNAME='
        env_val "${SMTP_USERNAME:-${SMTP_FROM:-}}"; echo
        printf 'SMTP_PASSWORD='
        env_val "${SMTP_PASSWORD:-}"; echo
        printf 'SMTP_FROM='
        env_val "${SMTP_FROM:-}"; echo
        echo "SMTP_FROM_NAME=${SMTP_FROM_NAME:-和润天下}"
        # 腾讯云短信（手机验证码通道；SecretId/SecretKey 为云 API 密钥，非控制台 AppKey）
        printf 'TENCENT_SMS_SECRET_ID='
        env_val "${TENCENT_SMS_SECRET_ID:-}"; echo
        printf 'TENCENT_SMS_SECRET_KEY='
        env_val "${TENCENT_SMS_SECRET_KEY:-}"; echo
        printf 'TENCENT_SMS_SDK_APP_ID='
        env_val "${TENCENT_SMS_SDK_APP_ID:-}"; echo
        printf 'TENCENT_SMS_SIGN_NAME='
        env_val "${TENCENT_SMS_SIGN_NAME:-}"; echo
        echo "TENCENT_SMS_TEMPLATE_REGISTER=${TENCENT_SMS_TEMPLATE_REGISTER:-}"
        echo "TENCENT_SMS_TEMPLATE_LOGIN=${TENCENT_SMS_TEMPLATE_LOGIN:-}"
        echo "TENCENT_SMS_TEMPLATE_PASSWORD=${TENCENT_SMS_TEMPLATE_PASSWORD:-}"
        echo "TENCENT_SMS_TEMPLATE_BIND_PHONE=${TENCENT_SMS_TEMPLATE_BIND_PHONE:-}"
        echo "TENCENT_SMS_REGION=${TENCENT_SMS_REGION:-ap-guangzhou}"
        # 微信登录凭证（小程序 / 开放平台严格区分）
        printf 'WECHAT_MINI_PROGRAM_APP_ID='
        env_val "${WECHAT_MINI_PROGRAM_APP_ID:-}"; echo
        printf 'WECHAT_MINI_PROGRAM_APP_SECRET='
        env_val "${WECHAT_MINI_PROGRAM_APP_SECRET:-}"; echo
        printf 'WECHAT_OPEN_PLATFORM_APP_ID='
        env_val "${WECHAT_OPEN_PLATFORM_APP_ID:-}"; echo
        printf 'WECHAT_OPEN_PLATFORM_APP_SECRET='
        env_val "${WECHAT_OPEN_PLATFORM_APP_SECRET:-}"; echo

        printf 'SECRET_KEY='
        env_val "${SECRET_KEY:-}"; echo
        printf 'JWT_SECRET_KEY='
        env_val "${JWT_SECRET_KEY:-}"; echo
        echo "JWT_EXPIRES_HOURS=${JWT_EXPIRES_HOURS:-2}"
        echo "JWT_REFRESH_EXPIRES_DAYS=${JWT_REFRESH_EXPIRES_DAYS:-7}"

        # 登录态 Cookie（父域名共享登录）
        printf 'AUTH_COOKIE_NAME='
        env_val "${AUTH_COOKIE_NAME:-hrwai_token}"; echo
        printf 'AUTH_COOKIE_DOMAIN='
        env_val "${AUTH_COOKIE_DOMAIN:-}"; echo
        echo "AUTH_COOKIE_SECURE=${AUTH_COOKIE_SECURE:-true}"

        printf 'ADMIN_DEFAULT_PASSWORD='
        env_val "${ADMIN_DEFAULT_PASSWORD:-}"; echo
        printf 'TUTOR_DEFAULT_PASSWORD='
        env_val "${TUTOR_DEFAULT_PASSWORD:-}"; echo
        printf 'STUDENT_DEFAULT_PASSWORD='
        env_val "${STUDENT_DEFAULT_PASSWORD:-}"; echo

        printf 'CORS_ORIGINS='
        env_val "${CORS_ORIGINS:-}"; echo

        echo "BACKEND_IMAGE=${IMAGE_BACKEND}:${IMAGE_TAG_BACKEND}"
        echo "FRONTEND_IMAGE=${IMAGE_FRONTEND}:${IMAGE_TAG_FRONTEND}"
        echo "LIBREOFFICE_IMAGE=${IMAGE_LIBREOFFICE:-forklift-libreoffice}:${IMAGE_TAG_LIBREOFFICE}"
        echo "DOMAIN=${DOMAIN:-localhost}"

        # SSL 证书目录（compose 挂载到 frontend 容器 /etc/nginx/ssl，nginx-host.conf 引用固定路径）
        echo "SSL_CERT_DIR=${SSL_CERT_DIR:-${DEPLOY_PATH}/nginx/ssl}"

        echo "UPLOAD_FOLDER=/data/uploads"
        echo "VOLUME_MOUNT_PATH=/data"
        echo "MAX_CONTENT_LENGTH_MB=250"
        echo "VALUATION_PDF_OUTPUT_DIR=/data/reports"
        echo "REDIS_PASSWORD=${REDIS_PASSWORD:-}"
        echo "REDIS_DB=${REDIS_DB:-0}"
        echo "REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-10}"
        echo "REDIS_KEY_PREFIX=${REDIS_KEY_PREFIX:-fl:}"

        echo "# S3 兼容对象存储（STORAGE_DRIVER=r2；R2_ENDPOINT 空=Cloudflare R2，非空=自建 RGW）"
        echo "STORAGE_DRIVER=${STORAGE_DRIVER:-local}"
        printf 'R2_ENDPOINT='
        env_val "${R2_ENDPOINT:-}"; echo
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
    } > "${DEPLOY_PATH}/.env.tmp.$$"
    # 唯一临时文件（$$）：并发部署时两条进程写各自 tmp，避免共享 .env.tmp 的
    # rm/mv 交错把 .env 删丢（曾致 compose 回退默认镜像名去拉 Docker Hub 超时）
    rm -f "${DEPLOY_PATH}/.env"
    mv "${DEPLOY_PATH}/.env.tmp.$$" "${DEPLOY_PATH}/.env"
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

    # 设置权限：chown 到 nginx:alpine 固定 UID 101，保证 nginx 用户可读
    # （master 以 root 运行绑 80/443，worker 降权为 nginx；证书文件可读，私钥仅 owner 可读）
    chown 101:101 "${SSL_CERT_DIR}/fullchain.pem" "${SSL_CERT_DIR}/privkey.pem"
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
        echo "=== 镜像标签: backend=${IMAGE_TAG_BACKEND} frontend=${IMAGE_TAG_FRONTEND} libreoffice=${IMAGE_TAG_LIBREOFFICE} ==="
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
            grep -v -E '(SECRET_KEY|JWT_SECRET_KEY|PASSWORD|API_KEY|R2_SECRET|R2_ACCESS_KEY|TENCENT_SMS_SECRET_ID)' "$DEPLOY_PATH/.env" 2>/dev/null || true
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
        # 先确保远端备份目录存在（目录缺失会导致 scp 静默失败）
        ssh -i "$ssh_key" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
            "$BACKUP_REMOTE_HOST" "mkdir -p ${remote_dir}" >/dev/null 2>&1 || true
        if scp -i "$ssh_key" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
            "$DB_BACKUP_FILE" "${BACKUP_REMOTE_HOST}:${remote_dir}/" 2>/tmp/backup_scp.err; then
            log_ok "异地备份完成: ${BACKUP_REMOTE_HOST}:${remote_dir}/$(basename "$DB_BACKUP_FILE")"
            # 清理远程旧备份（保留最近 10 份）
            ssh -i "$ssh_key" -o StrictHostKeyChecking=no "$BACKUP_REMOTE_HOST" \
                "ls -t ${remote_dir}/db_backup_*.sql.gz 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null" 2>/dev/null || true
        else
            local scp_err
            scp_err=$(tail -1 /tmp/backup_scp.err 2>/dev/null || echo "未知错误")
            log_warn "异地备份失败: ${scp_err}，继续部署"
        fi
        rm -f /tmp/backup_scp.err
    fi

    log_ok "备份完成: $BACKUP_FILE"
}

# ======================================================================
# 等待后台备份完成（备份与早期无依赖步骤并行，任何 DB 写操作前 join）
# ======================================================================
wait_backup() {
    local pid="$1"
    local waited=0
    log_info ">>> 等待后台备份完成..."
    while kill -0 "$pid" 2>/dev/null; do
        sleep 2
        waited=$((waited + 2))
        if [ "$waited" -ge "${BACKUP_WAIT_TIMEOUT:-120}" ]; then
            log_warn "备份超时（${waited}s），继续部署（本次部署窗口的回滚保障可能不完整）"
            return 1
        fi
    done
    wait "$pid" 2>/dev/null || log_warn "备份进程异常退出（详见上方日志）"
    log_ok "备份已完成"
    return 0
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
# 启动 ghcr pull-through 缓存容器（registry:2）
# 先清理同名残留容器再创建（缓存卷 ghcr-cache 保留，除非超阈值另行清空）
# ======================================================================
start_registry_proxy() {
    docker rm -f ghcr-proxy >/dev/null 2>&1 || true
    log_info "启动 ghcr pull-through 缓存容器 (registry:2)..."
    docker pull registry:2 >/dev/null 2>&1 || true
    local proxy_env=(-e REGISTRY_PROXY_REMOTEURL=https://ghcr.io)
    local proxy_mount=(-v ghcr-cache:/var/lib/registry)
    # 上游认证（read:packages PAT）：避免 ghcr.io 匿名 IP 限流导致拉取爬行/挂起
    # 凭据文件：${DEPLOY_PATH}/.ghcr-pull-token（root 600，ops 维护轮换）
    # 注：registry 2.8.x 不支持 *_FILE 后缀 env，须在容器创建时读文件注入
    GHCR_PULL_TOKEN_FILE="${GHCR_PULL_TOKEN_FILE:-$DEPLOY_PATH/.ghcr-pull-token}"
    if [ -f "$GHCR_PULL_TOKEN_FILE" ]; then
        proxy_env+=(-e REGISTRY_PROXY_USERNAME=oauth2 \
            -e "REGISTRY_PROXY_PASSWORD=$(cat "$GHCR_PULL_TOKEN_FILE")")
    fi
    docker run -d --name ghcr-proxy --restart unless-stopped \
        -p 127.0.0.1:5000:5000 \
        "${proxy_env[@]}" \
        "${proxy_mount[@]}" \
        registry:2 >/dev/null 2>&1 || true
    if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'ghcr-proxy'; then
        docker start ghcr-proxy >/dev/null 2>&1 || true
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
        # 缓存卷只增不减（pull-through 无自动回收），超阈值即清空重建，
        # 否则长时间积累会撑满磁盘（曾导致 testing 部署失败：磁盘 0GB < 2GB）
        CACHE_DIR="/var/lib/docker/volumes/ghcr-cache/_data"
        if [ -d "$CACHE_DIR" ]; then
            CACHE_MB=$(du -sm "$CACHE_DIR" 2>/dev/null | awk '{print $1}')
            if [ "${CACHE_MB:-0}" -gt "${CACHE_LIMIT_MB:-15000}" ]; then
                log_warn "ghcr-cache 缓存 ${CACHE_MB}MB 超过 ${CACHE_LIMIT_MB:-15000}MB 阈值，清空缓存卷（拉取按需回源）"
                docker rm -f ghcr-proxy >/dev/null 2>&1 || true
                docker volume rm ghcr-cache >/dev/null 2>&1 || true
            fi
        fi
        if ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx 'ghcr-proxy'; then
            start_registry_proxy
        elif ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'ghcr-proxy'; then
            docker start ghcr-proxy >/dev/null 2>&1 || true
        fi
    fi

    # 等待代理就绪（最多 10 秒），失败则改走国内镜像源（镜像源仍失败由 pull_one 回退直连兜底）
    for i in $(seq 1 10); do
        if curl -s -o /dev/null "http://${REGISTRY_PROXY}/v2/"; then
            log_ok "镜像加速代理就绪: ${REGISTRY_PROXY}"
            return 0
        fi
        sleep 1
    done
    log_warn "镜像加速代理未就绪 (${REGISTRY_PROXY})，本次部署改走国内镜像源 ${REGISTRY_MIRROR:-（未配置）}"
    IMAGE_BACKEND="${IMAGE_BACKEND_ORIG:-$IMAGE_BACKEND}"
    IMAGE_FRONTEND="${IMAGE_FRONTEND_ORIG:-$IMAGE_FRONTEND}"
    IMAGE_LIBREOFFICE="${IMAGE_LIBREOFFICE_ORIG:-$IMAGE_LIBREOFFICE}"
    rewrite_to_mirror
    return 0
}

# ghcr.io/* 镜像改写为国内镜像源前缀（代理不可用时改走镜像源加速；
# 镜像源再失败由 pull_one 回退直连 ghcr.io 兜底）。非 ghcr.io 镜像不动。
rewrite_to_mirror() {
    [ -z "$REGISTRY_MIRROR" ] && return 0
    case "$IMAGE_BACKEND" in
        ghcr.io/*) IMAGE_BACKEND="${REGISTRY_MIRROR}/${IMAGE_BACKEND#ghcr.io/}" ;;
    esac
    case "$IMAGE_FRONTEND" in
        ghcr.io/*) IMAGE_FRONTEND="${REGISTRY_MIRROR}/${IMAGE_FRONTEND#ghcr.io/}" ;;
    esac
    case "$IMAGE_LIBREOFFICE" in
        ghcr.io/*) IMAGE_LIBREOFFICE="${REGISTRY_MIRROR}/${IMAGE_LIBREOFFICE#ghcr.io/}" ;;
    esac
}

# ======================================================================
# 拉取 Docker 镜像
# ======================================================================

# 代理健康检查：探测超时则重启（registry:2 上游连接可能挂起导致拉取卡死）
# 重启后仍不可用则删除重建（缓存卷保留）；仍失败返回非 0，由调用方走镜像源/直连回退链路
ensure_proxy_healthy() {
    [ -z "$REGISTRY_PROXY" ] && return 0
    if timeout 5 curl -s -o /dev/null "http://${REGISTRY_PROXY}/v2/"; then
        return 0
    fi
    log_warn "镜像加速代理探测超时，重启 ghcr-proxy ..."
    docker restart ghcr-proxy >/dev/null 2>&1 || true
    sleep 3
    if timeout 5 curl -s -o /dev/null "http://${REGISTRY_PROXY}/v2/"; then
        log_ok "镜像加速代理重启成功"
        return 0
    fi
    log_warn "镜像加速代理重启后仍不可用，删除重建（缓存卷保留）..."
    start_registry_proxy
    sleep 3
    if timeout 5 curl -s -o /dev/null "http://${REGISTRY_PROXY}/v2/"; then
        log_ok "镜像加速代理重建成功"
        return 0
    fi
    log_error "镜像加速代理重建后仍不可用，本次拉取走镜像源/直连回退链路"
    return 1
}

# 拉取单个镜像：3 次重试；回退链路：国内镜像源（透传 ghcr 认证）→ 直连 ghcr.io 认证拉取
pull_one() {
    local name="$1"
    local image="$2"
    local retries=3
    for attempt in $(seq 1 $retries); do
        log_info "拉取${name}镜像 (尝试 $attempt/$retries): $image"
        # docker pull 加超时（默认 600s）：registry 上游挂起时不再无限等待，
        # 超时后按失败处理 → 重启代理重试 / 走回退链路
        if timeout "${DOCKER_PULL_TIMEOUT:-600}" docker pull "$image"; then
            log_ok "${name}镜像: $image"
            return 0
        fi
        if [ $attempt -lt $retries ]; then
            log_warn "拉取失败,10 秒后重试..."
            sleep 10
            # 重试前确保代理健康（registry:2 挂起是拉取卡死的常见原因，重启后再重试）
            ensure_proxy_healthy || true
        fi
    done

    # ghcr 核心路径（<org>/<image>:<tag>）：从代理/镜像源/直连任一前缀提取，供回退引用。
    # 注意：去掉 registry 前缀后的镜像名（如 driftingli/fl-backend:tag）会被 docker
    # 解析到 Docker Hub（registry-1.docker.io）——回退引用必须补对应 registry 前缀。
    local core=""
    case "$image" in
        "${REGISTRY_PROXY}/"*) core="${image#${REGISTRY_PROXY}/}" ;;
        "${REGISTRY_MIRROR}/"*) core="${image#${REGISTRY_MIRROR}/}" ;;
        "${REGISTRY}/"*) core="${image#${REGISTRY}/}" ;;
    esac

    # 回退 1：国内镜像源（仅 ghcr.io；镜像源路径本身失败时跳过，不重复尝试）
    if [ -n "$core" ] && [ "$REGISTRY" = "ghcr.io" ] && [ -n "$REGISTRY_MIRROR" ] &&
        [ -n "$GITHUB_TOKEN" ] && [ "$image" != "${REGISTRY_MIRROR}/${core}" ]; then
        local mirror_ref="${REGISTRY_MIRROR}/${core}"
        log_warn "代理拉取失败，回退国内镜像源 ${REGISTRY_MIRROR} 认证拉取: ${mirror_ref}"
        echo "$GITHUB_TOKEN" | docker login "$REGISTRY_MIRROR" -u oauth2 --password-stdin >/dev/null 2>&1 || true
        if timeout "${DOCKER_PULL_TIMEOUT:-600}" docker pull "$mirror_ref"; then
            docker tag "$mirror_ref" "$image"
            docker rmi "$mirror_ref" >/dev/null 2>&1 || true
            log_ok "镜像源拉取成功并补 tag: $image"
            return 0
        fi
    fi

    # 回退 2：直连 ghcr.io 认证拉取（已是直连路径时跳过；最终兜底）
    if [ -n "$core" ] && [ -n "$GITHUB_TOKEN" ] && [ "$image" != "${REGISTRY}/${core}" ]; then
        local ghcr_ref="${REGISTRY}/${core}"
        log_warn "回退直连 ${REGISTRY} 认证拉取: ${ghcr_ref}"
        echo "$GITHUB_TOKEN" | docker login "$REGISTRY" -u oauth2 --password-stdin >/dev/null 2>&1 || true
        if timeout "${DOCKER_PULL_TIMEOUT:-600}" docker pull "$ghcr_ref"; then
            docker tag "$ghcr_ref" "$image"
            docker rmi "$ghcr_ref" >/dev/null 2>&1 || true
            log_ok "直连拉取成功并补 tag: $image"
            return 0
        fi
    fi
    log_error "${name}镜像拉取失败(已重试 $retries 次 + 镜像源/直连回退): $image"
    return 1
}

pull_images() {
    log_info ">>> 拉取最新镜像..."
    ensure_proxy_healthy || true

    # 单次拉取超时由 pull_one 中的 timeout "${DOCKER_PULL_TIMEOUT:-600}" 控制
    # （registry 上游挂起时自动失败重试，不再无限等待）

    # 拉取后端镜像（本地已有该 tag 则跳过：内容标签命中即零传输）
    if [ -n "$IMAGE_BACKEND" ]; then
        local backend_image="${IMAGE_BACKEND}:${IMAGE_TAG_BACKEND}"
        if docker image inspect "$backend_image" &>/dev/null; then
            log_ok "后端镜像已缓存，跳过拉取: $backend_image"
        elif ! pull_one "后端" "$backend_image"; then
            exit 1
        fi
    fi

    # 拉取前端镜像
    if [ -n "$IMAGE_FRONTEND" ]; then
        local frontend_image="${IMAGE_FRONTEND}:${IMAGE_TAG_FRONTEND}"
        if docker image inspect "$frontend_image" &>/dev/null; then
            log_ok "前端镜像已缓存，跳过拉取: $frontend_image"
        elif ! pull_one "前端" "$frontend_image"; then
            exit 1
        fi
    fi

    # 拉取 LibreOffice sidecar 镜像（体积大）
    if [ -n "$IMAGE_LIBREOFFICE" ]; then
        local lo_image="${IMAGE_LIBREOFFICE}:${IMAGE_TAG_LIBREOFFICE}"
        if docker image inspect "$lo_image" &>/dev/null; then
            log_ok "LibreOffice sidecar 镜像已缓存，跳过拉取: $lo_image"
        elif ! pull_one "LibreOffice sidecar" "$lo_image"; then
            exit 1
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

    # 启动服务并等待容器就绪：
    # - --wait：compose v2 原生等待全部服务 healthy/running（事件驱动，替代固定 sleep + 轮询）
    # - --wait-timeout 超时后 up 返回非 0，由下方 health_check() HTTP 轮询兜底确认
    # 注：host 网络模式 frontend 无 healthcheck（compose 中 disable），--wait 按 running 处理
    if ! docker compose -f "$COMPOSE_FILE" ${COMPOSE_PROFILE_ARGS} up -d --wait --wait-timeout "${UP_WAIT_TIMEOUT:-60}" --remove-orphans 2>&1 | tail -10; then
        log_warn "compose --wait 超时或失败，由后续健康检查兜底确认"
    fi

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

    # 附加清理仓库：国内镜像源前缀（回退拉取遗留的镜像源 tag）与门户镜像
    # （hrwai-portal 独立流水线部署，不在 IMAGE_* 清单内，不清理会无限堆积）。
    local mirror_repos=()
    if [ -n "${REGISTRY_MIRROR:-}" ]; then
        local orig
        for orig in "${IMAGE_BACKEND_ORIG:-}" "${IMAGE_FRONTEND_ORIG:-}" "${IMAGE_LIBREOFFICE_ORIG:-}"; do
            [ -z "$orig" ] && continue
            case "$orig" in
                ghcr.io/*) mirror_repos+=("${REGISTRY_MIRROR}/${orig#ghcr.io/}") ;;
            esac
        done
    fi
    local portal_repo="${PRUNE_PORTAL_REPO:-127.0.0.1:5000/driftingli/hrwai-portal}"

    local repo
    for repo in "${IMAGE_BACKEND}" "${IMAGE_FRONTEND}" "${IMAGE_LIBREOFFICE}" \
                "${IMAGE_BACKEND_ORIG:-}" "${IMAGE_FRONTEND_ORIG:-}" "${IMAGE_LIBREOFFICE_ORIG:-}" \
                "${mirror_repos[@]:-}" "${portal_repo:-}"; do
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
            # -f 必需：同一镜像常被 代理/镜像源/直连 多前缀引用（同 ID 多 tag），
            # 不带 -f 时 rmi 报 "referenced in multiple repositories" 且被 || true
            # 静默吞掉——曾致旧版本镜像永不清理、无限堆积（testing 堆了 10 天 ~3GB）
            docker image rm -f "${id}" >/dev/null 2>&1 || true
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
            # 写入 SSL 证书（host 网络模式 + nginx-host.conf 已在 80/443 提供 HTTPS，
            # 证书内容来自 GitHub Secrets SSL_FULLCHAIN/SSL_PRIVKEY，写入 $SSL_CERT_DIR）
            write_ssl_certs
            # 备份与后续无依赖步骤（登录/镜像拉取）并行执行，迁移前 join——
            # 隐藏备份耗时（pg_dump + 异地同步），同时保证备份先于任何 DB 写操作完成。
            # SKIP_BACKUP=true（testing 加速）：跳过备份，回滚保障由 RBD 快照+每日备份承担
            if [ "${SKIP_BACKUP}" != "true" ]; then
                create_backup &
                BACKUP_PID=$!
            fi
            login_registry
            ensure_registry_proxy
            pull_images

            # 先拉起数据库与 Redis，供迁移预检读取当前版本
            docker compose -f "$COMPOSE_FILE" up -d "$POSTGRES_SERVICE" "$REDIS_SERVICE" 2>&1 | tail -5 || true
            wait_postgres

            # 迁移前 join 后台备份（回滚保障：备份必先于 fix_dirty/迁移落盘）
            if [ "${SKIP_BACKUP}" != "true" ] && [ -n "${BACKUP_PID:-}" ]; then
                wait_backup "$BACKUP_PID"
            fi

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
