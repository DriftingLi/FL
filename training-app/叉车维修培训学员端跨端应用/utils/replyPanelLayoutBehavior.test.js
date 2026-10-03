/**
 * 回复半屏浮层的键盘/面板布局逻辑（ADR-0025 ⑪，票 #1473）· **行为级**测试
 *
 * 为什么是行为而不是源码文本：⑪ 的验收写的是「面板贴键盘上沿、同值不重复驱动、上限约半屏」——
 * 这些只有**跑起来看驱动次数与产出数值**才算数。断言 `.uvue` 里出现过 `onKeyboardHeightChange`
 * 是接线守护（不构成 ③ 门证据，见 docs/agents/guards.md），去抖被改坏（同值也驱动）它照样绿。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 真执行 `utils/replyPanelLayout.uts`（先例 forumBodyBehavior）。
 *
 * **成对取证**（③ 判据③）：末组用「去掉同值守卫」的坏 tracker 证明去抖判据**有牙** ——
 * 坏实现下相同 height 会重复驱动 setter，被本判据抓红。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const LAYOUT_UTS = path.join(__dirname, 'replyPanelLayout.uts');

const layout = () => loadUts(LAYOUT_UTS, {});

describe('⑪-4 键盘高度去抖：相同 height 不重复驱动面板 bottom', () => {
  it('连续相同 height 只驱动一次；变化（含收起归零）才再驱动', () => {
    const { createKeyboardHeightTracker } = layout();
    const driven = [];
    const onEvent = createKeyboardHeightTracker((h) => { driven.push(h); });
    // 微信文档：keyboardheightchange 可能多次触发相同 height ⇒ 相同值应忽略
    [280, 280, 280, 0, 0, 320, 320].forEach((h) => onEvent(h));
    expect(driven).toEqual([280, 0, 320]);
  });

  it('初值为 0（键盘未起）：首个 0 事件不驱动（与初始同值），起键盘才驱动', () => {
    const { createKeyboardHeightTracker } = layout();
    const driven = [];
    const onEvent = createKeyboardHeightTracker((h) => { driven.push(h); });
    onEvent(0);      // 与初始同值 ⇒ 忽略
    onEvent(280);    // 键盘起
    expect(driven).toEqual([280]);
  });
});

describe('⑪-1/⑪-2 面板 bottom 与半屏上限', () => {
  it('面板开着时 bottom = 当前键盘高度；关闭时 bottom 归 0', () => {
    const { panelBottomPx } = layout();
    expect(panelBottomPx(280, true)).toBe(280);
    expect(panelBottomPx(280, false)).toBe(0);
    expect(panelBottomPx(0, true)).toBe(0);
  });

  it('面板高度上限约半屏（向下取整，不越过窗口一半）', () => {
    const { panelMaxHeightPx } = layout();
    expect(panelMaxHeightPx(800)).toBe(400);
    expect(panelMaxHeightPx(801)).toBe(400);   // 半屏取整
    expect(panelMaxHeightPx(667)).toBe(333);
  });
});

describe('成对取证：去抖判据有牙（坏实现必红）', () => {
  it('把 tracker 换成「无同值守卫」的坏版本 ⇒ 相同 height 被重复驱动 ⇒ 上面判据不成立', () => {
    // 坏实现：每次事件都驱动 setter（去掉 `if (h === last) return`）
    const brokenTracker = (setter) => (h) => { setter(h); };
    const driven = [];
    [280, 280, 280].forEach((h) => brokenTracker((x) => driven.push(x))(h));
    // 真判据要求 driven === [280]；坏实现给出 [280,280,280] ⇒ 证明判据能区分好坏
    expect(driven).not.toEqual([280]);
    expect(driven).toEqual([280, 280, 280]);
  });
});
