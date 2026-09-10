#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""估算：解析剩余时间 + 入库(切块/嵌入/图注)耗时。"""
import os
import re
import subprocess
import time
from datetime import datetime

LOG = '/root/mineru_run.log'
OUT = '/root/mineru-out'

now = datetime.now()
print('now:', now.strftime('%m-%d %H:%M'))

# 日志里的全量 START/DONE 行
starts, dones, fails = [], [], []
for line in open(LOG, encoding='utf-8', errors='ignore'):
    m = re.match(r'\[(\d+)/1588\] (START|DONE|FAIL) (.*?)\s+(\d\d:\d\d:\d\d)?', line)
    if m:
        idx, kind, stem = int(m.group(1)), m.group(2), m.group(3)
        ts = m.group(4)
        if kind == 'START':
            starts.append((idx, ts, stem))
        elif kind == 'DONE':
            dones.append((idx, ts, stem))
        elif kind == 'FAIL':
            fails.append((idx, ts, stem))

print('START 行数:', len(starts), ' DONE:', len(dones), ' FAIL:', len(fails))
if starts:
    print('首条全量 START:', starts[0])
    print('末条 START     :', starts[-1])

# 用 DONE 时间戳(带秒表)估算近期速率：取日志尾部 DONE 行的耗时
# 若 DONE 行不含耗时，则用最近1小时的 DONE 数量
last_done = dones[-5:] if dones else []
print('尾部 DONE 样例:', last_done)

# 已完成目录数
dirs = [d for d in os.listdir(OUT) if os.path.isdir(os.path.join(OUT, d))]
print('mineru-out 目录数:', len(dirs))

# md 总量
total_bytes = 0
n_md = 0
for root, _, files in os.walk(OUT):
    for f in files:
        if f.endswith('.md'):
            total_bytes += os.path.getsize(os.path.join(root, f))
            n_md += 1
print(f'md 文件数: {n_md}, 总字节: {total_bytes/1e6:.1f} MB')

# 图片统计：数量 + 大小分布(抽样2000)
imgs = []
for root, _, files in os.walk(OUT):
    for f in files:
        if f.endswith(('.jpg', '.jpeg', '.png')):
            imgs.append(os.path.join(root, f))
print('图片总数:', len(imgs))
import random
random.seed(1)
sample = random.sample(imgs, min(2000, len(imgs)))
sizes = sorted(os.path.getsize(p) for p in sample)
if sizes:
    n = len(sizes)
    q = lambda p: sizes[min(int(n * p), n - 1)]
    print(f'图片字节分布(抽样{n}): min={q(0)} p25={q(0.25)} 中位={q(0.5)} p75={q(0.75)} p95={q(0.95)} max={q(1.0)}')
    big = sum(1 for s in sizes if s >= 15000)
    print(f'>=15KB 占比: {big/n*100:.1f}%  → 全量约 {int(big/n*len(imgs))} 张')

# 图注缓存现状
cc = '/root/.caption_cache.json'
if os.path.exists(cc):
    print('caption 缓存:', os.path.getsize(cc), 'bytes')
else:
    print('caption 缓存: 不存在')
