#!/bin/bash
# 搜索知识库中与 Toyota 8FD 41-1 相关的 chunk，查看图片链接格式
docker exec kb-server python3 -c "
import sys
sys.path.insert(0, '/app/data')
from vector_store import VectorStore
vs = VectorStore('/app/data')
results = vs.search('Toyota 8FD 41-1 故障码', k=5)
for i, r in enumerate(results):
    print(f'--- chunk {i} (score={r[\"score\"]:.3f}) ---')
    print(r['text'][:500])
    print()
"