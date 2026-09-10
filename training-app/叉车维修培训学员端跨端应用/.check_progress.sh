#!/bin/bash
pgrep -f selective_caption.py
echo "SEPARATOR"
tail -3 /root/selective_caption.log 2>/dev/null
echo "SEPARATOR"
cat /root/.caption_high_done 2>/dev/null
echo "SEPARATOR"
python3 -c 'import json; d=json.load(open("/root/.caption_cache.json")); print(f"{len(d)} 条图注缓存")' 2>/dev/null
echo "SEPARATOR"
tail -3 /root/after_mineru2kb.log 2>/dev/null
echo "SEPARATOR"
echo "ALL_DONE"