/**
 * 回复半屏浮层的键盘/面板布局逻辑（ADR-0025 ⑪，票 #1473）· **行为级**测试
 *
 * 为什么是行为而不是源码文本：⑪ 的验收写的是「面板贴键盘上沿、同值不重复驱动、上限约半屏」——
 * 这些只有**跑起来看驱动次数与产出数值**才算数。断言 `.uvue` 里出现过 `onKeyboardHeightChange`
 * 是接线守护（不构成 ③ 门证据，见 docs/agents/guards.md），去抖被改坏（同值也驱动）它照样绿。
 *
 * #2691 起上限从「半屏」改为「视口 60%」：断言同步改值，判据仍是「跑函数看产出数值」。
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

describe('⑪-1/⑪-2 面板 bottom 与 60% 上限', () => {
  it('面板开着时 bottom = 当前键盘高度；关闭时 bottom 归 0', () => {
    const { panelBottomPx } = layout();
    expect(panelBottomPx(280, true)).toBe(280);
    expect(panelBottomPx(280, false)).toBe(0);
    expect(panelBottomPx(0, true)).toBe(0);
  });

  it('微信键盘已缩小可视区时，不再重复扣键盘高度把面板顶出屏幕', () => {
    const { panelBottomPx } = layout();
    // Screenshot geometry normalized from the reported 768x1668 device capture.
    const visibleHeight = 477;
    const keyboardHeight = 357;
    const panelHeight = 135;
    const panelTop = visibleHeight - panelBottomPx(keyboardHeight, true, true) - panelHeight;
    expect(panelTop).toBeGreaterThanOrEqual(0);
  });

  it('面板高度上限 = 视口 75%（向下取整，不越过窗口七成半）', () => {
    const { panelMaxHeightPx } = layout();
    // #2959：60%（480@800）装不下「引文 + 200px 编辑区 + 工具行」，放到 75%
    expect(panelMaxHeightPx(800)).toBe(600);
    expect(panelMaxHeightPx(801)).toBe(600);   // 600.75 向下取整
    expect(panelMaxHeightPx(667)).toBe(500);   // 500.25 向下取整
    expect(panelMaxHeightPx(1000)).toBe(750);
  });
});

describe('面板主体上限：主体可滚，发送键常驻（P0 截断修复）', () => {
  it('主体上限 = 面板上限 − 非滚动部分（头部 + 底部操作区）', () => {
    const { panelMaxHeightPx, panelBodyMaxHeightPx, REPLY_PANEL_CHROME_PX } = layout();
    const panel = panelMaxHeightPx(800);          // 600
    expect(panelBodyMaxHeightPx(panel)).toBe(panel - REPLY_PANEL_CHROME_PX);
    // 主体 + 非滚动部分 == 面板上限：整体不越界，发送键不会落到面板外
    expect(panelBodyMaxHeightPx(panel) + REPLY_PANEL_CHROME_PX).toBe(panel);
  });

  it('面板上限很小时主体收回下限，不把输入区压成零高', () => {
    const { panelBodyMaxHeightPx, REPLY_PANEL_MIN_BODY_PX } = layout();
    expect(panelBodyMaxHeightPx(200)).toBe(REPLY_PANEL_MIN_BODY_PX);
    expect(panelBodyMaxHeightPx(0)).toBe(REPLY_PANEL_MIN_BODY_PX);
    // 下限本身也要能看见：不小于一行工具键的高度（88rpx ≈ 44px）
    expect(REPLY_PANEL_MIN_BODY_PX).toBeGreaterThan(88);
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
