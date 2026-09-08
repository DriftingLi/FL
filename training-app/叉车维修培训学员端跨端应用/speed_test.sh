#!/bin/bash
# 测试 ollama 推理速度
result=$(curl -s --max-time 30 -X POST http://localhost:11434/api/generate \
  -d '{"model":"qwen3-kb","prompt":"1+1等于几？请直接回答数字。","options":{"num_predict":10}}')
eval_count=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('eval_count',0))")
eval_duration=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('eval_duration',0))")
if [ "$eval_duration" != "0" ]; then
  tokens_per_sec=$(echo "scale=1; $eval_count / ($eval_duration / 1000000000)" | bc)
  echo "tokens/s: $tokens_per_sec"
else
  echo "N/A"
fi
echo "GPU:"
nvidia-smi --query-gpu=memory.used,memory.total,utilization.gpu --format=csv,noheader