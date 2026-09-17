from PIL import Image
import numpy as np

path = r'D:\FL\wt-1080\training-app\叉车维修培训学员端跨端应用-1080\docs\verification\device\1080\task-center.png'
im = Image.open(path).convert('RGB')
a = np.array(im)
H, W, _ = a.shape
print('image', W, 'x', H)

targets = {
    'daily  icon bg #FFF3E0': (0xFF, 0xF3, 0xE0),   # .icon-daily  -> 每日任务行
    'newbie icon bg #E8F5E9': (0xE8, 0xF5, 0xE9),   # .icon-newbie -> 新手任务行
    'underline    #2979FF': (0x29, 0x79, 0xFF),     # .section-underline 每段一条
}
TOL = 4
for name, (r, g, b) in targets.items():
    m = ((np.abs(a[:, :, 0].astype(int) - r) <= TOL) &
         (np.abs(a[:, :, 1].astype(int) - g) <= TOL) &
         (np.abs(a[:, :, 2].astype(int) - b) <= TOL))
    cols = m.sum(axis=0)
    rows = m.sum(axis=1)
    bands = []
    inb = False
    start = 0
    for y, c in enumerate(rows):
        if c >= 20 and not inb:
            inb, start = True, y
        elif c < 20 and inb:
            inb = False
            bands.append((start, y - 1))
    if inb:
        bands.append((start, H - 1))
    print(f'\n{name}: total_px={int(m.sum())}  vertical_bands={len(bands)}')
    for (y0, y1) in bands:
        sub = m[y0:y1 + 1]
        xs = np.where(sub.any(axis=0))[0]
        print(f'   y {y0:5d}-{y1:5d} (h={y1-y0+1:4d})  x {xs.min():5d}-{xs.max():5d}  px={int(sub.sum())}')
