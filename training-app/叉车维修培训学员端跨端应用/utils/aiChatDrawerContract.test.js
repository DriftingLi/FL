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

/**
 * 右入口**可见性**判据（2026-11-10，#2412；#2479 改锚点：页头右槽 → 工具栏行）。
 *
 * 现场：#1649（8a95e05b）修融合冲突时把页头件的单图标分支删了，只留 rightIcons 与 #right 插槽，
 * 而 ai-chat-nav 还在传 :right-icon ⇒ 右槽退回空插槽：MP-WEIXIN 端「看不见、点得动」
 * （rightSlotWidth > 0 且挂着 @click），App / H5 端零宽 ⇒ 入口彻底消失。
 *
 * #2479 改锚点：一级页标题要居中于**屏幕中心** ⇒ 页头第一行右槽必须为空（不空则长标题会钻到
 * 浮动的右槽下面），⋯ 下沉到标题栏下方工具栏行（app-nav-bar 的 #toolbar）。入口的**链路**没变
 * （页面 API 仍是 menuClick、右抽屉仍由页面渲染），变的只是字形落点。
 *
 * 三件串成一条：① ⋯ 在 #toolbar 里有字形、且 ai-chat-nav 打开了工具栏行；
 * ② 点击经 onMenuClick → 页面 @menu-click 接到开右抽屉；③ 抽屉真的在页面里渲染并绑菜单项。
 * 另守两条：页头件的 rightIcons 分支仍在（其他页面仍在用）、同义 rightIcon prop 不得复活。
 */
function rightEntryVisible(nav, navBar, page) {
  const v = [];
  if (!/v-if="rightIcons\.length > 0"/.test(navBar)) {
    v.push('页头件没有 rightIcons 渲染分支 ⇒ 右槽只剩空插槽，入口无可见字形');
  }
  // 同义 prop 不得复活：它是 #2412 的成因（#1649 删了分支却留下 prop），收编后右槽只有
  // rightIcons / #right 两条路径（ADR 0063）；再长出第三条只会让下一个调用方再被骗一次
  if (/rightIcon\s*\?:\s*string/.test(navBar)) {
    v.push('页头件又声明了 rightIcon prop（#2412 已收编为 rightIcons 一条路径，同义 prop 不得复活，见 ADR 0063）');
  }
  if (!/<slot name="toolbar">/.test(navBar)) {
    v.push('页头件没有 #toolbar 插槽 ⇒ ⋯ 无处渲染（#2479：入口下沉标题栏下方工具栏行）');
  }
  if (/:right-icons="/.test(nav)) {
    v.push('ai-chat-nav 又把右槽字形传回页头（#2479：一级页页头第一行右槽必须为空，否则长标题钻到浮动右槽下面）');
  }
  if (!/:show-toolbar="true"/.test(nav)) {
    v.push('ai-chat-nav 未打开页头工具栏行（:show-toolbar）⇒ ⋯ 不渲染，入口看不见');
  }
  const toolbar = (nav.match(/<template #toolbar>([\s\S]*?)<\/template>/) || [])[1] || '';
  if (toolbar.length === 0) {
    v.push('ai-chat-nav 未把 ⋯ 放进 #toolbar（#2479：右抽屉入口落在工具栏行）');
  } else {
    if (!/⋯/.test(toolbar)) {
      v.push('工具栏行无 ⋯ 字形 ⇒ 右抽屉入口不可见');
    }
    if (!/@click="onMenuClick"/.test(toolbar)) {
      v.push('工具栏行 ⋯ 未接到 onMenuClick（点得动也开不了右抽屉）');
    }
  }
  if (!/@menu-click="toggleSidebarRight"/.test(page)) {
    v.push('页面未把 menu-click 接到 toggleSidebarRight（点了也开不了右抽屉）');
  }
  if (!/<AiChatDrawerRight[^>]*:menu-items="rightMenuItems"/.test(page)) {
    v.push('页面未渲染右侧菜单抽屉，或抽屉未绑菜单项（开了也是空的）');
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

  it('④ 右侧入口可点：⋯ 的命中区由 #toolbar 行承载（#2479 改锚点：页头第一行右槽清空）', () => {
    // #1602：AiChatNav 独立形态消失，页头结构归 app-nav-bar；此处守三件事：
    // a) nav 包装件仍发出 menuClick（页面 API 不变）
    expect(nav).toMatch(/onMenuClick/);
    // b) #2479：一级页标题居中于屏幕中心 ⇒ 入口下沉标题栏下方工具栏行 —— 页头件必须有该插槽，
    //    且工具栏节点真的渲染它（组件侧几何由 appNavBarCapsuleToolbarContract 另看一道）。
    const navBar = read(NAVBAR);
    expect(navBar).toMatch(/<slot name="toolbar">/);
    expect(navBar).toMatch(/v-if="showToolbar"[\s\S]*?slot name="toolbar"/);
    // c) 页头第一行右槽不再承载该入口（清空正是标题能居中的前提）
    expect(nav).not.toMatch(/:right-icons=/);
    // 组件侧右槽仍可点（其他页面 —— resume 等 —— 仍在用 rightIcons / #right 两条路径）
    expect(navBar).toMatch(/class="nav-bar-side nav-bar-right"[\s\S]*?@click="onRightClick"/);
    expect(navBar).toMatch(/capsuleLeft/);
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

/** 右入口（右侧菜单抽屉）：补上右槽与 leftEntryVisible 对称的兜底（#2412）。 */
describe('右入口可见性（右侧菜单抽屉，2026-11-10 #2412；#2479 锚点移到工具栏行）', () => {
  const nav = read(NAV);
  const navBar = read(NAVBAR);
  const page = read(PAGE_PATH);

  it('① 三件齐备：组件画字形 + 点击接到开右抽屉 + 抽屉绑菜单项，零违规', () => {
    expect(rightEntryVisible(nav, navBar, page)).toEqual([]);
  });

  it('② 判别力：字形置空 / 抽掉工具栏 / 拆接线 / 传回右槽 / 不渲染右抽屉 ⇒ 各自报红', () => {
    // 字形置空：工具栏行里的 ⋯ 被掏空
    const noGlyph = nav.replace(/(<text class="chat-menu-glyph">)[^<]*(<\/text>)/, '$1$2');
    expect(noGlyph).not.toBe(nav);
    expect(rightEntryVisible(noGlyph, navBar, page).join()).toMatch(/工具栏行无 ⋯ 字形/);
    // 整块 #toolbar 抽掉（回到「页头第一行是唯一入口」的旧形态）
    const noToolbar = nav.replace(/<template #toolbar>[\s\S]*?<\/template>\s*/, '');
    expect(noToolbar).not.toBe(nav);
    expect(rightEntryVisible(noToolbar, navBar, page).join()).toMatch(/未把 ⋯ 放进 #toolbar/);
    // 拆接线：nav 侧 / 页面侧各一处
    expect(rightEntryVisible(nav.replace('@click="onMenuClick"', ''), navBar, page).join()).toMatch(/未接到 onMenuClick/);
    expect(rightEntryVisible(nav, navBar, page.replace('@menu-click="toggleSidebarRight"', '')).join()).toMatch(/未把 menu-click 接到 toggleSidebarRight/);
    // 关掉工具栏行（⋯ 无处渲染）
    expect(rightEntryVisible(nav.replace(':show-toolbar="true"', ''), navBar, page).join()).toMatch(/未打开页头工具栏行/);
    // 右槽字形又传回来（本票的成因形态）
    expect(rightEntryVisible(nav.replace('<template #toolbar>', ':right-icons="[]"\n\t\t\t<template #toolbar>'), navBar, page).join()).toMatch(/又把右槽字形传回页头/);
    // 不渲染右抽屉
    const noDrawer = page.replace(/<AiChatDrawerRight[\s\S]*?\/>/, '');
    expect(noDrawer).not.toBe(page);
    expect(rightEntryVisible(nav, navBar, noDrawer).join()).toMatch(/未渲染右侧菜单抽屉/);
    // 页头件丢了字形分支（回归原形：右槽只剩插槽）
    expect(rightEntryVisible(nav, navBar.replace('v-if="rightIcons.length > 0"', 'v-if="false"'), page).join()).toMatch(/没有 rightIcons 渲染分支/);
    // 同义 prop 复活（本票原形：prop 在、分支没了）
    const resurrected = navBar.replace('rightIcons ?: string[]', 'rightIcon ?: string\n        rightIcons ?: string[]');
    expect(resurrected).not.toBe(navBar);
    expect(rightEntryVisible(nav, resurrected, page).join()).toMatch(/又声明了 rightIcon prop/);
    // 页头件丢了 #toolbar 插槽（⋯ 无处渲染）
    expect(rightEntryVisible(nav, navBar.replace('<slot name="toolbar">', ''), page).join()).toMatch(/没有 #toolbar 插槽/);
  });
});
