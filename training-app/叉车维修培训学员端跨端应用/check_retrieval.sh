#!/bin/bash
# 检索知识库 chunk 内容
cat > /tmp/query2.json << 'JSONEOF'
{"knowledge_id":"all","query":"Toyota 8FD 41-1 故障码","retrieval_setting":{"top_k":3}}
JSONEOF
curl -s -X POST http://127.0.0.1:5000/retrieval \
  -H 'Content-Type: application/json' \
  -d @/tmp/query2.json > /tmp/ret2.json
python3 << 'PYEOF'
import json
with open("/tmp/ret2.json") as f:
    data = json.load(f)
if isinstance(data, dict) and "records" in data:
    for i, r in enumerate(data["records"]):
        s = r.get("score", "?")
        print("--- 结果", i, "(score =", s, ") ---")
        text = r.get("content", r.get("text", ""))
        print(text[:600])
        print()
else:
    print("UNKNOWN:", json.dumps(data, ensure_ascii=False)[:500])
PYEOF