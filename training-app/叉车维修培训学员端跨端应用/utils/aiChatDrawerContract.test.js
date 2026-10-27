/**
 * AI 助手抽屉契约 —— 两条**真机实测踩到的**坑，一条一断言。
 *
 * 为什么需要它（都有现场证据，都是"编译能过、真机才炸/才报"）：
 *
 *  ① **类型名义重复 = 运行时崩溃**：`components/ai-chat/ai-chat-drawer-right.uvue` 原来自带一份
 *     `type MenuItem = { key: string; label: string }`，而页面传进来的是
 *     `pages/ai-assistant/ai-assistant-constants.uts` 导出的同名类型。两个结构相同、**名义不同**的类型
 *     编译到 Kotlin 后是**两个类** ⇒ 真机抛
 *     `java.lang.ClassCastException: MenuItem cannot be cast to MenuItem`
 *     （栈顶就是该文件第 9 行的 `v-for` 行）⇒ 右侧抽屉**一渲染就炸、连遮罩都画不出来**，
 *     真机表现为「点右上角 ⋯ 没有反应」（issue #947）。
 *     **左侧抽屉**的类型来自 `types/index`（单一来源）⇒ 不炸 —— 这就是左右不对称的全部原因，
 *     也是当初把根因猜成"点击/定位"的原因（猜错了，实测日志直接给了答案）。
 *
 *  ② **`<view>` 上不许用文字类样式**：`ai-chat-drawer-left.uvue` 的 `.empty-tip` 曾在 `view` 上用
 *     `text-align / font-size / color`，真机报
 *     `style property text-align|font-size|color is only supported on <text>|<button>|<input>|<textarea>`
 *     （全项目 20 多处 `.empty-tip` 里只有它这么写；其余都把字体样式放在子元素 `.empty-text` 上）。
 *     这条会让 `hx-run` 的 error 扫描判 `exit=fail`，挡住开发内循环的干净收口。
 *
 *  ③ **右抽屉必须用 `left` 锚定 + 右侧入口要有显式尺寸的命中盒**：真机「点 ⋯ 无反应」在补上
 *     `.drawer-right { left: 30% }`（以及右侧入口的 `.nav-more-box`）之后消失（维护者 2026-09-13 18:45
 *     截图确认抽屉正常弹出）。⚠️ 这两处里**承重者未定**（未单独二分）⇒ 本测试锁的是"**实测可用的那套
 *     组合**"，不是某一处的因果；将来若要精简，必须先在真机上验证。
 *
 *  ⚠️ **别再犯的错**：①那条（类型重复）当年被我当成"点不开"的根因 —— 但 18:45 那个**可用**构建里
 *     它仍然存在 ⇒ 它只是**编译期诊断**（会让 hx-run 判红），**不是**点不开的原因。看到编译期诊断时，
 *     不要直接把它当成用户可见现象的根因。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const RIGHT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-right.uvue');
const LEFT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-left.uvue');
const NAV = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-nav.uvue');
const NAVBAR = path.join(__dirname, '..', 'components', 'app-nav-bar', 'app-nav-bar.uvue');
const PAGE_PATH = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const read = (p) => readText(p);

/**
 * 左入口**可见性**判据（2026-10-27）—— 与上面 ④（右入口可点）同源的左半边。
 *
 * 现场：左抽屉（最近对话）自 #947 起就是好的，`@left-click="toggleSidebarLeft"` 也接好了，
 * 但页头左槽的 `leftIcon` 默认空串 ⇒ AppNavBar 的 `.nav-bar-left` 内 text 不渲染 ⇒ 槽 view
 * 零宽零高 ⇒ **命中区不存在**（#1677 A 的「有动作、无可点面」形态）。真机表现就是「AI 助手页
 * 没有任何地方能打开历史会话」，只有点标题碰巧代发（左槽空置时的代发，见 appNavBarLeftEntryContract）——
 * 那属于兜底，不能当入口。
 *
 * 三件串成一条：① 左槽真有字形；② 页面把它接到开左抽屉；③ 抽屉真的在页面里渲染并绑会话列表。
 * 缺任一件，用户都打不开会话列表。
 */
function leftEntryVisible(nav, page) {
  const v = [];
  const m = nav.match(/leftIcon:\s*'([^']*)'/);
  const glyph = m ? m[1] : '';
  if (glyph.length === 0) {
    v.push('页头左槽无字形（leftIcon 为空）⇒ 左槽零宽零高、无命中区，会话列表打不开');
  }
  if (!/@left-click="toggleSidebarLeft"/.test(page)) {
    v.push('页面未把 left-click 接到 toggleSidebarLeft（点了也开不了抽屉）');
  }
  if (!/<AiChatDrawerLeft[^>]*:sessions="sessions"/.test(page)) {
    v.push('页面未渲染左侧会话抽屉，或抽屉未绑会话列表（开了也是空的）');
  }
  return v;
}

describe('AI 助手抽屉契约（类型单一来源 / view 不承载文字样式）', () => {
  const right = read(RIGHT);
  const left = read(LEFT);
  const nav = read(NAV);

  it('① 右侧抽屉不自带 MenuItem 类型，必须与页面同源（否则 Kotlin 里是两个类 ⇒ ClassCastException）', () => {
    expect(right).not.toMatch(/^\s*type\s+MenuItem\s*=/m);
    expect(right).toContain(
      "import type { MenuItem } from '../../pages/ai-assistant/ai-assistant-constants'"
    );
    // prop 仍按该类型声明（改回 any/UTSJSONObject 就等于放弃这层约束）
    expect(right).toMatch(/menuItems\?:\s*MenuItem\[\]/);
  });

  it('② 左侧抽屉同样不自带类型，走共享 types（对照项：它一直是对的）', () => {
    expect(left).not.toMatch(/^\s*type\s+(AiSession|MenuItem)\s*=/m);
    expect(left).toContain("import type { AiSession } from '../../types/index'");
  });

  it('③ empty-tip 的字体样式落在 text 上，不能在 view 上（uvue 只允许 <text> 等元素）', () => {
    const at = left.indexOf('.empty-tip {');
    expect(at).toBeGreaterThan(-1);
    const block = left.slice(at, left.indexOf('}', at));
    expect(block).not.toMatch(/font-size|color|text-align/);
    expect(left).toContain('<text class="empty-text">');
    expect(left).toMatch(/\.empty-text\s*\{[^}]*font-size/);
  });

  it('④ 右侧入口可点：命中区由页头件右槽承载（#1602 归一：AiChatNav→AppNavBar，#947 经验收编进 app-nav-bar 右槽）', () => {
    // #1602：AiChatNav 独立形态消失，页头右槽归 app-nav-bar；此处守两件事：
    // a) nav 包装件仍发出 menuClick（页面 API 不变）
    expect(nav).toMatch(/onMenuClick/);
    // b) 页头件右槽是显式可点元素（右槽宽度由 useSafeArea().rightSlotWidth 驱动，胶囊让位）。
    //    #1604：右槽 class 归一为 nav-bar-side nav-bar-right（左右槽共用 .nav-bar-side 基类），
    //    并开放 <slot name="right">（批1 页面右槽内容经 slot 注入）——可点性与宽度约束不变。
    const navBar = read(NAVBAR);
    expect(navBar).toMatch(/class="nav-bar-side nav-bar-right"[\s\S]*?@click="onRightClick"/);
    expect(navBar).toMatch(/rightSlotWidth/);
    // 右抽屉 left 锚定继续保留（#947 实测组合）
    expect(right).toMatch(/\.drawer-right\s*\{[^}]*left:\s*30%/);
  });
});

describe('左入口可见性（最近对话 / 会话列表，2026-10-27）', () => {
  const nav = read(NAV);
  const page = read(PAGE_PATH);

  it('① 左槽有字形：leftIcon 默认非空（空 ⇒ 零宽零高、无命中区）', () => {
    expect(nav).toMatch(/leftIcon:\s*'[^']+'/);
  });

  it('② 字形仍是 BMP 符号（非 BMP 代理对曾被吃成空串，血账见 recruiter-tab-bar 注释）', () => {
    const glyph = (nav.match(/leftIcon:\s*'([^']*)'/) || [])[1];
    // 全部码位都 ≤ 0xFFFF ⇒ 不是代理对
    const allBmp = Array.from(glyph).every((ch) => ch.codePointAt(0) <= 0xffff);
    expect(allBmp).toBe(true);
  });

  it('③ 三件齐备：字形 + 点击接到开左抽屉 + 抽屉绑会话列表，零违规', () => {
    expect(leftEntryVisible(nav, page)).toEqual([]);
  });

  it('④ 判别力：把 leftIcon 改回空串 / 拆掉点击接线 / 不渲染抽屉 ⇒ 各自报红', () => {
    expect(leftEntryVisible(nav.replace(/leftIcon:\s*'[^']*'/, "leftIcon: ''"), page).join()).toMatch(
      /左槽无字形/
    );
    expect(leftEntryVisible(nav, page.replace('@left-click="toggleSidebarLeft"', '')).join()).toMatch(
      /未把 left-click 接到 toggleSidebarLeft/
    );
    expect(
      leftEntryVisible(nav, page.replace(/<AiChatDrawerLeft[\s\S]*?\/>/, '')).join()
    ).toMatch(/未渲染左侧会话抽屉/);
  });

  /**
   * ⑤ 标题恒居中（2026-10-27）：左槽一旦有字形，AppNavBar 的 §5.1 就把标题切左对齐 ——
   * 「☰ AI 助手 ⋯」全挤在左上角。聊天页要三段式居中，故透传 centerTitle，默认开。
   * 这条锁两件事：prop 真的透传到 AppNavBar（不是写在 props 里没人用）、且默认值是 true
   * （默认 false ⇒ 页头又退回左对齐，而这不会报任何错）。
   */
  it('⑤ 标题恒居中：center-title 透传到页头且默认开', () => {
    expect(nav).toMatch(/:center-title="centerTitle"/);
    expect(nav).toMatch(/centerTitle:\s*true/);
    // 判别力：默认值翻成 false 或拆掉透传 ⇒ 两条断言各自失配
    expect(nav.replace(/centerTitle:\s*true/, 'centerTitle: false')).not.toMatch(/centerTitle:\s*true/);
    expect(nav.replace(':center-title="centerTitle"', '')).not.toMatch(/:center-title="centerTitle"/);
  });
});
