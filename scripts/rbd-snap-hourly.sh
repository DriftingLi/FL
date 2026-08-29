#!/bin/bash
# ===== 每小时 RBD 快照：forklift-data/pgdata =====
# crash-consistent 快照（PG 有 WAL，恢复时自动回放）；误操作回滚用
# 离线逻辑备份由每日 pg_dump（存 RGW + 异地）承担，见 scripts/backup-daily
# 保留最近 KEEP 份，超出即删除最旧
IMG="forklift-data/pgdata"
KEEP=30

rbd snap create "${IMG}@snap-$(date +%Y%m%d-%H)" >/dev/null 2>&1 || exit 1

mapfile -t snaps < <(rbd snap ls "$IMG" | awk 'NR>1 {print $2}' | sort -r)
n=${#snaps[@]}
for ((i=KEEP; i<n; i++)); do
    rbd snap rm "${IMG}@${snaps[$i]}" 2>/dev/null || true
done
echo "[$(date '+%F %T')] snapshot ok, total=$(rbd snap ls "$IMG" | wc -l)"