#!/bin/sh
set -e

# 入口脚本 — 创建 /data 子目录并执行 CMD
# 默认以非 root 用户(app)运行，直接执行 CMD
# 若通过 compose user: root override 以 root 运行，先修正 /data 属主再切换到 app 用户

DATA_DIRS="/data/uploads /data/reports /data/backups"
for dir in $DATA_DIRS; do
    mkdir -p "$dir" 2>/dev/null || true
done

if [ "$(id -u)" = "0" ]; then
    # root 运行：修正属主后切换到 app 用户
    chown -R app:app /data 2>/dev/null || true
    exec su-exec app "$@"
else
    # 非 root 运行：直接执行
    exec "$@"
fi
