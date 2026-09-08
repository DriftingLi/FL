#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os
import time

OUT = '/root/mineru-out'
mts = []
for d in os.listdir(OUT):
    p = os.path.join(OUT, d, 'auto', d + '.md')
    if os.path.exists(p):
        mts.append(os.path.getmtime(p))
mts.sort()
span = time.time() - mts[0]
n = len(mts)
rate = n / span * 3600
rem = (1588 - n) / rate
print(f'已完成 {n} 本, 时间跨度 {span/3600:.1f}h, 速率 {rate:.1f} 本/h, 单本均 {span/n/60:.1f} min')
print(f'剩余 {1588-n} 本 ≈ {rem:.1f} h ≈ {rem/24:.1f} 天')
print('server now:', time.strftime('%m-%d %H:%M'))
