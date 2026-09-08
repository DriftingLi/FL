#!/bin/bash
# 补跑 35 本 FAIL 的 PDF
# 备份原 todo，用 retry 清单替换，启动 run_mineru.sh
# 补跑完成后自动恢复

cp /root/mineru_todo.txt /root/mineru_todo.txt.bak.full
cp /root/mineru_retry.txt /root/mineru_todo.txt

echo "=====试跑前检查====="
echo "TODO 条数: $(wc -l < /root/mineru_todo.txt)"
echo "MinerU 进程:"
pgrep -af mineru 2>/dev/null || echo "无"

# 检查 MinerU fast_api 是否在运行
if curl -s http://127.0.0.1:55001/health >/dev/null 2>&1; then
  echo "MinerU 推理服务运行中，直接启动"
else
  echo "MinerU 推理服务未运行，启动脚本会自行拉起"
fi

# 清理旧日志，重新跑
mv /root/mineru_run.log /root/mineru_run.log.bak.full 2>/dev/null
nohup bash /root/run_mineru.sh > /root/mineru_retry_run.log 2>&1 &
echo "补跑已启动 PID=$!"