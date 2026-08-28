#!/usr/bin/env bash
# ===== 每日数据库离线备份（分层备份策略的最后一层）=====
#   RBD 小时快照（crash-consistent）→ 误删/回滚
#   本脚本每日 pg_dump 逻辑备份 → RGW 私有 bucket + 异地 pve-01（防逻辑/灾难级问题）
# 依赖：/etc/forklift-backup.env（root 600）提供凭据与异地配置：
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / RGW_ENDPOINT /
#   REMOTE_HOST / REMOTE_DIR / REMOTE_KEY（可选，留空跳过异地同步）
set -euo pipefail
# shellcheck source=/dev/null
. /etc/forklift-backup.env

BKT=forklift-backups
EP="${RGW_ENDPOINT:-http://127.0.0.1:8088}"
BKDIR=/var/backups/forklift
KEEP_DAYS="${KEEP_DAYS:-14}"
TS=$(date +%Y%m%d_%H%M%S)
DB_BK="${BKDIR}/db_${TS}.sql.gz"
mkdir -p "$BKDIR"
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

echo "[backup] $(date '+%F %T') 开始每日备份..."

# 1. pg_dump + gzip（逻辑备份，可精确恢复单表）
if ! docker exec forklift-pg-prod pg_dump -U forklift -d forklift_training 2>/dev/null | gzip -9 > "$DB_BK"; then
    echo "[backup] pg_dump 失败" >&2
    rm -f "$DB_BK"
    exit 1
fi

# 2. 上传 RGW 私有 bucket
if ! aws --endpoint-url "$EP" s3api put-object --bucket "$BKT" \
    --key "backup/${TS}.sql.gz" --body "$DB_BK" >/dev/null; then
    echo "[backup] RGW 上传失败" >&2
    exit 1
fi

# 3. RGW 旧备份清理（对象按名称时间排序，保留最近 KEEP_DAYS 份）
mapfile -t OLD < <(aws --endpoint-url "$EP" s3api list-objects --bucket "$BKT" \
    --prefix backup/ --query "reverse(Contents[].Key)" --output text 2>/dev/null \
    | tr '\t' '\n' | tail -n +$((KEEP_DAYS + 1)))
for k in "${OLD[@]}"; do
    [ -n "$k" ] && aws --endpoint-url "$EP" s3api delete-object --bucket "$BKT" --key "$k" >/dev/null 2>&1 || true
done

# 4. 异地同步 pve-01（可选）
if [ -n "${REMOTE_HOST:-}" ]; then
    ssh -i "$REMOTE_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
        "$REMOTE_HOST" "mkdir -p ${REMOTE_DIR}" 2>/dev/null || true
    if scp -i "$REMOTE_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
        "$DB_BK" "${REMOTE_HOST}:${REMOTE_DIR}/" 2>/dev/null; then
        ssh -i "$REMOTE_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
            "$REMOTE_HOST" "find ${REMOTE_DIR} -name 'db_*.sql.gz' -mtime +${KEEP_DAYS} -delete" 2>/dev/null || true
        echo "[backup] 异地同步完成"
    else
        echo "[backup] 异地同步失败（忽略，RGW 仍有副本）" >&2
    fi
fi

# 5. 本地保留 KEEP_DAYS
find "$BKDIR" -name 'db_*.sql.gz' -mtime +"$KEEP_DAYS" -delete 2>/dev/null || true

echo "[backup] 完成: $DB_BK ($(du -h "$DB_BK" | cut -f1))"