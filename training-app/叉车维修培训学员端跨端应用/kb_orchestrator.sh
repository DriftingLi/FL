#!/bin/bash
# kb_orchestrator.sh — 叉车知识库全量流水线编排（A 方案，幂等，每10分钟调用）
# 阶段链: parsing -> converting(无图注) -> embedding -> captioning(后台数周)
#         -> reconverting(带图注) -> reingesting(全量重灌) -> done
# 状态: /root/.kb_pipeline_state   完成标记: /root/.kb_pipeline_done
# 输出约定(供定时任务解析):
#   TRANSITION: <state> <中文说明>   阶段切换(需通知用户)
#   STATE=<s> PROGRESS ...           常规进度(简短汇报)
#   STATE=<s> RELAUNCH_* ...         进程死亡自动续跑
#   STATE=<s> ERROR ...              需人工介入
#   PIPELINE_DONE chunks=N           全部完成(通知+删除定时任务)
set -u
STATE_FILE=/root/.kb_pipeline_state
DONE_MARKER=/root/.kb_pipeline_done
MLOG=/root/mineru_run.log

cur=$(cat "$STATE_FILE" 2>/dev/null || echo "parsing")

alive() { pgrep -f "$1" >/dev/null 2>&1; }

attempt() {  # $1=phase；递增尝试计数并输出次数
  local f="/root/.kb_att_$1"
  local n
  n=$(cat "$f" 2>/dev/null || echo 0)
  n=$((n + 1))
  echo "$n" > "$f"
  echo "$n"
}

setstate() {  # $1=state $2=中文说明
  echo "$1" > "$STATE_FILE"
  echo "TRANSITION: $1 $2"
}

gpu_overhead() {  # 读取当前 ollama GPU_OVERHEAD
  docker inspect ollama --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | grep -o 'OLLAMA_GPU_OVERHEAD=[0-9]*' | cut -d= -f2
}

recreate_ollama() {  # $1=overhead
  docker rm -f ollama >/dev/null 2>&1
  docker run -d --name ollama --network host --gpus all --restart unless-stopped \
    -e OLLAMA_HOST=0.0.0.0:11434 -e OLLAMA_KEEP_ALIVE=-1 \
    -e OLLAMA_GPU_OVERHEAD="$1" \
    -v /root/ollama:/root/.ollama \
    ollama/ollama:latest serve >/dev/null
  sleep 8
}

stats_chunks() {
  curl -s --max-time 10 http://127.0.0.1:5000/stats | grep -oE '[0-9]+' | head -1
}

case "$cur" in

parsing)
  d=$(grep -c '\] DONE ' "$MLOG" 2>/dev/null); d=${d:-0}
  f=$(grep -c '\] FAIL ' "$MLOG" 2>/dev/null); f=${f:-0}
  s=$(grep -c '\] SKIP ' "$MLOG" 2>/dev/null); s=${s:-0}
  total=$((d + f + s))
  if alive 'run_mineru\.sh' || alive 'mineru-venv/bin/mineru'; then
    echo "STATE=parsing PROGRESS 解析中 $total/1588 (DONE=$d FAIL=$f SKIP=$s)"
  elif [ "$total" -ge 1588 ]; then
    # 解析全部完成：调大 ollama 显存预留(原任务使命) + 启动无图注转换
    if [ "$(gpu_overhead)" != "256" ]; then
      recreate_ollama 256
    fi
    rm -f /root/.kb_att_convert
    nohup python3 /root/mineru2kb.py /root/mineru-out --no-caption \
      > /root/kb_convert.log 2>&1 &
    setstate converting "解析完成(DONE=$d FAIL=$f SKIP=$s)。ollama显存预留已调至256(Dify恢复全速)；启动无图注转换+图片上传S3，预计2-4小时"
  else
    n=$(attempt parse)
    if [ "$n" -le 3 ]; then
      nohup bash /root/run_mineru.sh >> "$MLOG" 2>&1 &
      echo "STATE=parsing RELAUNCH_PARSE 解析进程中断($total/1588)，已续跑 attempt=$n"
    else
      echo "STATE=parsing ERROR 解析进程反复中断($total/1588)，需人工检查 $MLOG"
    fi
  fi
  ;;

converting)
  if alive 'mineru2kb\.py'; then
    echo "STATE=converting PROGRESS 转换中 $(tail -1 /root/kb_convert.log 2>/dev/null | cut -c1-110)"
  elif grep -q '^完成 ' /root/kb_convert.log 2>/dev/null; then
    rm -f /root/.kb_att_embed
    nohup docker exec kb-server python /app/data/ingest_direct.py /app/data --recursive \
      > /root/kb_ingest.log 2>&1 &
    setstate embedding "转换完成，启动全量切块+bge-m3嵌入，预计1-5天；期间检索服务正常"
  else
    n=$(attempt convert)
    if [ "$n" -le 3 ]; then
      nohup python3 /root/mineru2kb.py /root/mineru-out --no-caption \
        > /root/kb_convert.log 2>&1 &
      echo "STATE=converting RELAUNCH_CONVERT 转换中断，已续跑 attempt=$n"
    else
      echo "STATE=converting ERROR 转换反复中断，需人工检查 /root/kb_convert.log"
    fi
  fi
  ;;

embedding)
  if alive 'ingest_direct\.py'; then
    echo "STATE=embedding PROGRESS 嵌入中 $(tail -1 /root/kb_ingest.log 2>/dev/null | cut -c1-110)"
  elif grep -q '^完成:' /root/kb_ingest.log 2>/dev/null; then
    curl -s --max-time 10 -X POST http://127.0.0.1:5000/reload >/dev/null
    rm -f /root/.kb_att_caption
    nohup python3 /root/caption_runner.py > /root/kb_caption.log 2>&1 &
    setstate captioning "嵌入完成(chunks=$(stats_chunks))已reload——全量知识库(无图注版)正式可用；启动后台图注(qwen3-vl:2b，预计1-3周，期间Dify问答可能变慢)"
  else
    n=$(attempt embed)
    if [ "$n" -le 3 ]; then
      nohup docker exec kb-server python /app/data/ingest_direct.py /app/data --recursive \
        > /root/kb_ingest.log 2>&1 &
      echo "STATE=embedding RELAUNCH_EMBED 嵌入中断，已续跑 attempt=$n"
    else
      echo "STATE=embedding ERROR 嵌入反复中断，需人工检查 /root/kb_ingest.log"
    fi
  fi
  ;;

captioning)
  if alive 'caption_runner\.py'; then
    tgt=$(cat /root/.caption_target.txt 2>/dev/null || echo 0)
    cache_n=$(python3 -c "import json;print(len(json.load(open('/root/.caption_cache.json'))))" 2>/dev/null || echo 0)
    echo "STATE=captioning PROGRESS 图注中 cache=$cache_n/$tgt $(tail -1 /root/kb_caption.log 2>/dev/null | cut -c1-90)"
  elif [ -f /root/.caption_done ]; then
    rm -f /root/.kb_att_reconvert
    nohup python3 /root/mineru2kb.py /root/mineru-out --force \
      > /root/kb_convert2.log 2>&1 &
    setstate reconverting "图注完成($(cat /root/.caption_done))。启动最终转换(带图注，缓存命中较快)"
  else
    n=$(attempt caption)
    if [ "$n" -le 3 ]; then
      nohup python3 /root/caption_runner.py > /root/kb_caption.log 2>&1 &
      echo "STATE=captioning RELAUNCH_CAPTION 图注中断，已续跑 attempt=$n"
    else
      echo "STATE=captioning ERROR 图注反复中断，需人工检查 /root/kb_caption.log"
    fi
  fi
  ;;

reconverting)
  if alive 'mineru2kb\.py'; then
    echo "STATE=reconverting PROGRESS 最终转换中 $(tail -1 /root/kb_convert2.log 2>/dev/null | cut -c1-110)"
  elif grep -q '^完成 ' /root/kb_convert2.log 2>/dev/null; then
    docker exec kb-server sh -c \
      'rm -f /app/data/index/faiss.index /app/data/index/metadata.pkl /app/data/index/processed_sources.txt'
    rm -f /root/.kb_att_reingest
    nohup docker exec kb-server python /app/data/ingest_direct.py /app/data --recursive \
      > /root/kb_ingest2.log 2>&1 &
    setstate reingesting "最终转换完成，清空索引全量重灌(带图注)，预计1-5天；期间旧索引继续服务零停机"
  else
    n=$(attempt reconvert)
    if [ "$n" -le 3 ]; then
      nohup python3 /root/mineru2kb.py /root/mineru-out --force \
        > /root/kb_convert2.log 2>&1 &
      echo "STATE=reconverting RELAUNCH_RECONVERT 最终转换中断，已续跑 attempt=$n"
    else
      echo "STATE=reconverting ERROR 最终转换反复中断，需人工检查 /root/kb_convert2.log"
    fi
  fi
  ;;

reingesting)
  if alive 'ingest_direct\.py'; then
    echo "STATE=reingesting PROGRESS 重灌中 $(tail -1 /root/kb_ingest2.log 2>/dev/null | cut -c1-110)"
  elif grep -q '^完成:' /root/kb_ingest2.log 2>/dev/null; then
    curl -s --max-time 10 -X POST http://127.0.0.1:5000/reload >/dev/null
    chunks=$(stats_chunks)
    echo "PIPELINE_DONE chunks=$chunks" > "$DONE_MARKER"
    echo "STATE=done PIPELINE_DONE chunks=$chunks"
  else
    n=$(attempt reingest)
    if [ "$n" -le 3 ]; then
      nohup docker exec kb-server python /app/data/ingest_direct.py /app/data --recursive \
        > /root/kb_ingest2.log 2>&1 &
      echo "STATE=reingesting RELAUNCH_REINGEST 重灌中断，已续跑 attempt=$n"
    else
      echo "STATE=reingesting ERROR 重灌反复中断，需人工检查 /root/kb_ingest2.log"
    fi
  fi
  ;;

done)
  echo "STATE=done PIPELINE_DONE $(cat "$DONE_MARKER" 2>/dev/null)"
  ;;

*)
  echo "STATE=unknown ERROR 未知状态: $cur（可手动 echo parsing > $STATE_FILE 重置）"
  ;;
esac
