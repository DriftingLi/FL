"""#1011 ①a 机检：从 a11y dump 断言「入口存在 + 顺序在末尾 + 跳转后确实到达目标页」。

为什么用 a11y 而不是像素：本票的判据是**文案级**（宫格项「练习记录」、目标页标题与统计卡标签），
a11y 逐字给出即可判；像素仅作截图留证。（#1083 用像素是因为那一页的 a11y 会间歇性返回 0 文本节点。）

用法：python check-1011.py            # 断言同目录下 01/02 两个 dump
退出码：0 = 全部断言通过；1 = 有失败
"""

import os
import re
import sys

try:
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')
except Exception:
    pass

FAILS = []
INFOS = []


def texts(path):
    if not os.path.exists(path):
        FAILS.append(f'缺文件：{os.path.basename(path)}')
        return []
    xml = open(path, encoding='utf-8').read()
    out = []
    for m in re.finditer(r'(?:text|content-desc)="([^"]*)"', xml):
        v = m.group(1)
        if v and v not in out:
            out.append(v)
    return out


def main():
    d = os.path.dirname(os.path.abspath(__file__))
    grid = texts(os.path.join(d, '01-profile-grid-a11y-dump.xml'))
    page = texts(os.path.join(d, '02-practice-records-a11y-dump.xml'))
    if not grid or not page:
        print('\n'.join(FAILS))
        return 1

    # A. 入口存在 + 顺序（每个宫格项是「图标实体 + 文案」两个节点 ⇒ 不能拿相邻节点当顺序判据）
    print('[A] 「我的」宫格出现第 9 项「练习记录」，且排在原有 8 项之后')
    ORDER = ['学习记录', '收藏夹', '错题本', '笔记本', '学练计划', '学习资料', '就业在线', '任务中心', '练习记录']
    if '练习记录' not in grid:
        FAILS.append('「我的」宫格里没有「练习记录」')
    else:
        idx = {w: (grid.index(w) if w in grid else -1) for w in ORDER}
        seq = [idx[w] for w in ORDER]
        if -1 in seq:
            FAILS.append('宫格项缺失：' + '、'.join(w for w in ORDER if idx[w] == -1))
        elif seq == sorted(seq):
            print(f'  PASS 9 项顺序成立（节点下标 {seq[0]} → {seq[-1]}），「练习记录」为末项')
        else:
            FAILS.append(f'宫格顺序不对：{list(zip(ORDER, seq))}')

    # B. 原有 8 项一个都不能少（防「新增一项、顶掉一项」）
    print('\n[B] 工具宫格原 8 项仍在（新增而非替换）')
    want = ['学习记录', '收藏夹', '错题本', '笔记本', '学练计划', '学习资料', '就业在线', '任务中心']
    missing = [w for w in want if w not in grid]
    if not missing:
        print('  PASS 8 项全在')
    else:
        FAILS.append('原有工具项缺失：' + '、'.join(missing))

    # C. 跳转成立：目标页身份 + 统计卡 + 列表头
    print('\n[C] 点入口后到达 practice-records（页身份 + 统计卡 + 列表）')
    ident = ['练习记录', '今日做题', '累计做题', '正确率', '连续天数']
    miss2 = [w for w in ident if w not in page]
    if not miss2:
        print('  PASS 页身份与统计卡齐全：' + '、'.join(ident))
    else:
        FAILS.append('目标页缺少：' + '、'.join(miss2))

    # D. 列表/空态：二者必居其一，且要如实报告是哪一个
    print('\n[D] 列表态还是空态（如实报告）')
    has_list = any(t == '没有更多了' for t in page)
    has_empty = '当前证件下暂无练习记录' in page
    if has_empty:
        INFOS.append('本帧为空态：已观察到新文案「当前证件下暂无练习记录」')
        print('  INFO 空态文案已观察到')
    elif has_list:
        INFOS.append('本帧为列表态（本账号当前证件下有练习记录）⇒ 空态文案本批未观察')
        print('  INFO 列表态：本批未观察到空态文案')
    else:
        FAILS.append('目标页既没有列表结束标记，也没有空态文案')

    print('\n=== 汇总 ===')
    for i in INFOS:
        print(f'INFO {i}')
    for f in FAILS:
        print(f'FAIL {f}')
    print(f'check-1011: {"OK" if not FAILS else "FAILED"} ({len(FAILS)} 条断言未过)')
    return 1 if FAILS else 0


if __name__ == '__main__':
    sys.exit(main())
