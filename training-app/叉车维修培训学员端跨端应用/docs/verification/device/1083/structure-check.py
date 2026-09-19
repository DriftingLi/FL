"""#1083 ①a 真机取证的像素结构断言（PIL + numpy，**非 OCR**）。

判据形状照先例 docs/verification/device/1080/structure-check.py。
颜色 / 尺寸事实取自 pages/profile/components/wrong-question-card.uvue 的 <style>：

    .record-question-img   height:320rpx、background-color:#f8f9fa   （题干图块）
    .card-action-btn       border:1rpx solid #2979ff（「查看答案与解析」，白底蓝描边）
    .card-action-primary   background-color:#2979ff（实心「重做」）
    .card-action-danger    border:1rpx solid #f44336（描边「移出」）
    .answer-section        background-color:#f8f9fa、padding:16rpx、margin-top:16rpx
    .answer-line/.answer-explain   color:#666666（空态 .answer-empty 为 #999999）

本机 rpx→px 系数 = 1156/750 = 1.54133 ⇒ 320rpx = 493.2 px。

⚠️ 面板色 #f8f9fa 与页面底色 #f5f5f5 只差 3 —— 判面板时 TOL 必须收到 2，
   否则两种底色互相污染（首版 TOL=6 就踩了这个坑，读数不可用）。

用法：
    python structure-check.py                 # 断言同目录下的 20/21/22 三张图
    python structure-check.py --dir <目录>

退出码：0 = 全部断言通过；1 = 有断言失败。
"""

import argparse
import os
import sys

import numpy as np
from PIL import Image

# 本机控制台是 GBK：不强制 UTF-8 时，输出里的 ⇒ 之类字符会直接抛 UnicodeEncodeError，
# 且 Tee 出去的机检文件会变成 GBK（入库产物必须是 UTF-8）。
try:
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')
except Exception:
    pass

BLUE = (0x29, 0x79, 0xFF)   # #2979ff
RED = (0xF4, 0x43, 0x36)    # #f44336
PANEL = (0xF8, 0xF9, 0xFA)  # #f8f9fa
TEXT = (0x66, 0x66, 0x66)   # #666666

RIGHT_HALF_X = 620          # 卡片操作行都在右半区；左半区是题干/标签
MIN_BAND_PX = 20
PANEL_TOL = 2               # 见文件头：必须收紧，否则与 #f5f5f5 混淆

FAILS = []
INFOS = []


def load(path):
    if not os.path.exists(path):
        FAILS.append(f'缺图：{os.path.basename(path)}')
        return None
    return np.array(Image.open(path).convert('RGB'))


def mask(a, rgb, tol=6):
    r, g, b = rgb
    return ((np.abs(a[:, :, 0].astype(int) - r) <= tol) &
            (np.abs(a[:, :, 1].astype(int) - g) <= tol) &
            (np.abs(a[:, :, 2].astype(int) - b) <= tol))


def bands(m, x_from=0, min_px=MIN_BAND_PX):
    sub = m[:, x_from:]
    rows = sub.sum(axis=1)
    out, inb, start = [], False, 0
    for y, c in enumerate(rows):
        if c >= min_px and not inb:
            inb, start = True, y
        elif c < min_px and inb:
            inb = False
            out.append((start, y - 1))
    if inb:
        out.append((start, len(rows) - 1))
    res = []
    for (y0, y1) in out:
        s = sub[y0:y1 + 1]
        xs = np.where(s.any(axis=0))[0]
        if len(xs):
            res.append((y0, y1, int(xs.min()) + x_from, int(xs.max()) + x_from, int(s.sum())))
    return res


def show(title, bs):
    print(f'  {title}: bands={len(bs)}')
    for (y0, y1, x0, x1, px) in bs:
        print(f'     y {y0:5d}-{y1:5d} (h={y1 - y0 + 1:4d})  x {x0:5d}-{x1:5d}  px={px}')


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--dir', default=os.path.dirname(os.path.abspath(__file__)))
    ap.add_argument('--groups', default='20-groups-view.png')
    ap.add_argument('--cards', default='21-card-list.png')
    ap.add_argument('--opened', default='22-answer-open.png')
    a = ap.parse_args()

    groups = load(os.path.join(a.dir, a.groups))
    cards = load(os.path.join(a.dir, a.cards))
    opened = load(os.path.join(a.dir, a.opened))
    if groups is None or cards is None or opened is None:
        print('\n'.join(FAILS))
        return 1

    for name, img in ((a.groups, groups), (a.cards, cards), (a.opened, opened)):
        print(f'image {name}: {img.shape[1]} x {img.shape[0]}')

    # ---- A. 分组视图：右半区没有卡片操作行 ----
    print('\n[A] 分组视图（默认态）右半区不得出现卡片操作行')
    gb = bands(mask(groups, BLUE), RIGHT_HALF_X)
    gr = bands(mask(groups, RED), RIGHT_HALF_X)
    show('blue #2979ff x>620', gb)
    show('red  #f44336 x>620', gr)
    if not gb and not gr:
        print('  PASS 分组视图右半区零操作行 ⇒ 默认确为分组视图')
    else:
        FAILS.append('分组视图右半区出现卡片操作行 ⇒ 这一帧不是分组视图')

    # ---- B. 下钻后卡片列表渲染出操作行 ----
    print('\n[B] 下钻后卡片列表：右半区应有实心主按钮 + 描边危险按钮')
    cb = bands(mask(cards, BLUE), RIGHT_HALF_X)
    cr = bands(mask(cards, RED), RIGHT_HALF_X)
    show('blue #2979ff x>620', cb)
    show('red  #f44336 x>620', cr)
    if cb and cr:
        print(f'  PASS 卡片列表渲染：实心/描边带 {len(cb)} / {len(cr)}')
    else:
        FAILS.append('下钻后右半区没有卡片操作行 ⇒ 卡片列表未渲染')

    # ---- C. 展开后出现答案面板（#f8f9fa）且内含 #666666 文字行 ----
    print('\n[C] 点「查看答案与解析」后：出现答案面板（#f8f9fa，TOL=2）且含 #666666 文字行')
    ob = bands(mask(opened, BLUE), RIGHT_HALF_X)
    show('blue #2979ff x>620（展开帧）', ob)
    pb = bands(mask(opened, PANEL, PANEL_TOL), 0)
    show('#f8f9fa 全宽（TOL=2）', pb)
    panel = [b for b in pb if 100 <= (b[1] - b[0] + 1) <= 420]
    txt = bands(mask(opened, TEXT), 0, min_px=5)
    show('#666666 全宽', txt)
    if panel and txt and any(b[0] <= t[0] <= b[1] for b in panel for t in txt):
        print(f'  PASS 答案面板 {len(panel)} 块（高度 100–420px），其内有 #666666 文字行 {len(txt)} 条')
    else:
        FAILS.append('展开后未见「高度 100–420px 的 #f8f9fa 面板 + 其内 #666666 文字行」')

    # ---- D. 卡片增高：第 2 张卡的操作行被面板推下 ----
    # 只认「操作行」形状的带（实心主按钮 h ≈ 72px = 12rpx+24rpx*1.5 量级）：
    # 帧里还有状态栏/角标之类的小蓝带（h ≤ 8），直接取 cb[1] 会取错（首版就踩了这个）。
    def action_rows(bs):
        return [b for b in bs if 60 <= (b[1] - b[0] + 1) <= 80]

    print('\n[D] 展开后卡片增高：第 2 张卡的操作行 y 应更大')
    cb_rows, ob_rows = action_rows(cb), action_rows(ob)
    print(f'  操作行数：折叠态 {len(cb_rows)} / 展开态 {len(ob_rows)}')
    if len(cb_rows) >= 2 and len(ob_rows) >= 2:
        y0, y1 = cb_rows[1][0], ob_rows[1][0]
        print(f'  第 2 张卡操作行 y: {y0} → {y1}  delta={y1 - y0}')
        if y1 > y0:
            print('  PASS 卡片增高')
        else:
            FAILS.append('展开后第 2 张卡未被推下 ⇒ 面板未插入')
    else:
        FAILS.append('帧内操作行不足 2，无法用「被推下」证明增高')

    # ---- E. 题干图块（报告项，不判红）----
    print('\n[E] 题干图块（320rpx = 493px，报告项，不参与判红）')
    img_bands = [b for b in bands(mask(cards, PANEL, PANEL_TOL), 0) if 450 <= (b[1] - b[0] + 1) <= 540]
    if img_bands:
        show('疑似题干图块', img_bands)
        INFOS.append(f'疑似题干图块 {len(img_bands)} 处')
    else:
        print('  本帧无高度 ≈493px 的 #f8f9fa 块')
        INFOS.append('本帧无带图错题 ⇒ 图块高度判据未主张')

    print('\n=== 汇总 ===')
    for i in INFOS:
        print(f'INFO {i}')
    for f in FAILS:
        print(f'FAIL {f}')
    print(f'structure-check: {"OK" if not FAILS else "FAILED"} ({len(FAILS)} 条断言未过)')
    return 1 if FAILS else 0


if __name__ == '__main__':
    sys.exit(main())
