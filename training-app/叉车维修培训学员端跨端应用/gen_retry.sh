#!/bin/bash
# 提取 FAIL 的 PDF 路径并生成补跑清单
# 从 mineru_run.log 提取 [N/1588] FAIL 格式的行
grep -E '\[[0-9]+/1588\] FAIL ' /root/mineru_run.log | while IFS= read -r line; do
  # 提取数字部分 [N/1588]
  n=$(echo "$line" | grep -oE '\[[0-9]+/1588\]' | tr -d '[]' | cut -d/ -f1)
  if [ -n "$n" ]; then
    sed -n "${n}p" /root/mineru_todo.txt
  fi
done > /root/mineru_retry.txt
wc -l /root/mineru_retry.txt
echo "=====前5行====="
head -5 /root/mineru_retry.txt
echo "=====后5行====="
tail -5 /root/mineru_retry.txt