#!/bin/sh
set -e

# Railway 挂载持久卷到 /data，卷属主为 root，app 用户无权创建子目录。
# 此入口脚本以 root 运行，创建所需子目录并修正属主，再切换到 app 用户执行主进程。
mkdir -p /data/reports /data/uploads/chapters /data/uploads/slides
chown -R app:app /data

exec su-exec app "$@"
