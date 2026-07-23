#!/bin/sh
set -e

# 入口脚本 — 以 root 创建 /data 子目录并修正属主，再切换到 app 用户执行 CMD

# 创建运行时数据目录（如果不存在）
DATA_DIRS="/data/uploads /data/reports /data/backups"
for dir in $DATA_DIRS; do
    if [ ! -d "$dir" ]; then
        mkdir -p "$dir"
    fi
done

# 修正 /data 的属主（Railway 卷挂载可能覆盖镜像层的权限）
chown -R app:app /data 2>/dev/null || true

# 切换到 app 用户并执行 CMD
exec su-exec app "$@"
