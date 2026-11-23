/**
 * AI 助手抽屉契约 —— 左抽屉（唯一入口面）的类型来源、样式载体与菜单承载。
 *
 * 为什么需要它（都有现场证据，都是"编译能过、真机才炸/才报"）：
 *
 *  ① **类型名义重复 = 运行时崩溃**：右抽屉（ai-chat-drawer-right.uvue，已随 #2786 删除）原来自带一份
 *     type MenuItem，而页面传进来的是 pages/ai-assistant/ai-assistant-constants.uts 导出的同名类型。
 *     两个结构相同、**名义不同**的类型编译到 Kotlin 后是**两个类** ⇒ 真机抛
 *     java.lang.ClassCastException: MenuItem cannot be cast to MenuItem（栈顶就是该文件的 v-for 行）
 *     ⇒ 抽屉一渲染就炸、连遮罩都画不出来，真机表现为「点了没有反应」（issue #947）。
 *     #2786 把菜单搬进**左抽屉**后，同一条坑随入口搬过来 —— 本条守护因此改锚左抽屉。
 *
 *  ② **<view> 上不许用文字类样式**：ai-chat-drawer-left.uvue 的 .empty-tip 曾在 view 上用
 *     text-align / font-size / color，真机报
 *     style property text-align|font-size|color is only supported on <text>|<button>|<input>|<textarea>
 *     （全项目 20 多处 .empty-tip 里只有它这么写）。这条会让 hx-run 的 error 扫描判 exit=fail。
 *
 *  ③ **抽屉必须显式 left/right 锚定**：真机「点 ⋯ 无反应」在补上 .drawer-right { left: 30% } 之后消失
 *     （维护者 2026-09-13 18:45 截图确认抽屉正常弹出）。⚠️ 承重者未定（未单独二分）⇒ 本测试锁的是
 *     "**实测可用的那套组合**"，不是某一处的因果；将来若要精简，必须先在真机上验证。#2786 后锚点
 *     只剩左抽屉的 .drawer-left { left: 0 }。
 *
 *  ④ **入口面单一化（#2786，移动端 ADR-0009 2026-11-23 修订）**：⋯ 与右抽屉退役，菜单（含替代横幅的
 *     「解锁专业版」）唯一落在左抽屉「设置」分组。守护随之改写：右入口可见性判据作废，换成
 *     「左抽屉承载菜单」—— 常量表四项齐、左抽屉真的渲染并抛 key、页面分派四个动作。
 *     ⚠️ 删掉「解锁专业版」= 基础版彻底没有兑换入口（ADR-0009 2026-09-13 修订否决过这个中间态），
 *     所以这项必须报红而不是静默。
 *
 *  ⚠️ **别再犯的错**：①那条（类型重复）当年被我当成"点不开"的根因 —— 但 18:45 那个**可用**构建里
 *     它仍然存在 ⇒ 它只是**编译期诊断**（会让 hx-run 判红），**不是**点不开的原因。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const LEFT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-left.uvue');
const RIGHT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-right.uvue');
const NAV = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-nav.uvue');
const NAVBAR = path.join(__dirname, '..', 'components', 'app-nav-bar', 'app-nav-bar.uvue');
const CONSTS = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant-constants.uts');
const PAGE_PATH = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const COMPOSABLE = path.join(__dirname, '..', 'composables', 'useAiChat.uts');
const read = (p) => readText(p);

/** 取函数体（大括号配平）——防止切片越界到文件尾造成假绿（沿用 #921 的 fnBody）。 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) return '';
  const open = src.indexOf('{', start);
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  return '';
}

/**
 * 左入口**可见性**判据（2026-10-27）—— 左抽屉是会话列表入口。
 *
 * 现场：左抽屉自 #947 起就是好的，@left-click="toggleSidebarLeft" 也接好了，但页头左槽的 leftIcon
 * 默认空串 ⇒ 左槽内 text 不渲染 ⇒ 槽 view 零宽零高 ⇒ **命中区不存在**（#1677 A 的「有动作、无可点面」
 * 形态）。真机表现就是「AI 助手页没有任何地方能打开历史会话」。
 *
 * 三件串成一条：① 左槽真有字形；② 页面把它接到开左抽屉；③ 抽屉真的在页面里渲染并绑会话列表。
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
 * 左抽屉**承载菜单**判据（#2786）—— 取代退役的 rightEntryVisible。
 *
 * 三件串成一条：① 常量表四项齐（含「解锁专业版」，且不许把「新对话」混回来）；
 * ② 左抽屉真的渲染分组标题 + menuItems，并把点击 key 抛给页面；③ 页面把菜单传进去、
 * 接 action、且 onMenuAction 四个分支齐（pro 必须接 openProSheet）。
 * 缺任一件，用户要么看不到入口，要么点了没反应。
 */
function drawerCarriesMenu(consts, left, page) {
  const v = [];
  const keys = ['settings', 'custom-models', 'profile', 'pro'];
  keys.forEach((k) => {
    if (consts.indexOf("key: '" + k + "'") < 0) v.push('常量表缺少菜单项 ' + k);
  });
  if (consts.indexOf("label: '解锁专业版'") < 0) {
    v.push('常量表没有「解锁专业版」项 ⇒ 横幅已删，基础版没有任何兑换入口');
  }
  if (/key: 'new-chat'/.test(consts)) {
    v.push('菜单表又混进「新对话」（它由左抽屉白卡承载，两处会被读成两个动作）');
  }
  if (!/v-for="item in menuItems"/.test(left)) v.push('左抽屉未渲染 menuItems（设置分组是空的）');
  if (!/class="drawer-group-title"/.test(left)) {
    v.push('左抽屉缺分组标题 ⇒ 设置项会被读成「最近对话里混进来的行」');
  }
  if (!/@click="onMenu\(item\.key\)"/.test(left)) v.push('菜单行未接 onMenu（点了没反应）');
  if (!/emit\('action', key\)/.test(left)) v.push('左抽屉未把菜单 key 抛给页面（点了没反应）');
  if (!/<AiChatDrawerLeft[^>]*:menu-items="drawerMenuItems"/.test(page)) {
    v.push('页面未把菜单传给左抽屉（设置分组会渲染成空卡）');
  }
  if (!/@action="onMenuAction"/.test(page)) v.push('页面未接左抽屉的 action 事件');
  const fn = fnBody(page, 'function onMenuAction');
  if (fn.length === 0) {
    v.push('页面没有 onMenuAction（菜单点击无处分派）');
  } else {
    keys.forEach((k) => {
      if (fn.indexOf("key == '" + k + "'") < 0) v.push('onMenuAction 缺少 ' + k + ' 分支');
    });
    if (!/key == 'pro'[\s\S]*?openProSheet\(\)/.test(fn)) {
      v.push('「解锁专业版」未接到 openProSheet（点了打不开兑换面板）');
    }
  }
  return v;
}

describe('AI 助手抽屉契约（类型单一来源 / view 不承载文字样式 / 抽屉锚定）', () => {
  const left = read(LEFT);
  const nav = read(NAV);

  it('① 左抽屉不自带 MenuItem 类型，必须与页面同源（否则 Kotlin 里是两个类 ⇒ ClassCastException）', () => {
    expect(left).not.toMatch(/^\s*type\s+MenuItem\s*=/m);
    expect(left).toContain(
      "import type { MenuItem } from '../../pages/ai-assistant/ai-assistant-constants'"
    );
    // prop 仍按该类型声明（改回 any/UTSJSONObject 就等于放弃这层约束）
    expect(left).toMatch(/menuItems\?:\s*MenuItem\[\]/);
  });

  it('② 左侧抽屉的会话类型走共享 types（对照项：它一直是对的）', () => {
    expect(left).not.toMatch(/^\s*type\s+AiSession\s*=/m);
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

  it('④ 抽屉显式 left 锚定（#947 实测组合；#2786 后只剩左抽屉这一处）', () => {
    expect(left).toMatch(/\.drawer-left\s*\{[^}]*left:\s*0/);
  });

  it('⑤ 页头不再承载菜单入口：无 #toolbar 插槽 / 无字形节点 / 无 menuClick（#2786 单一入口面）', () => {
    // 按结构判、不按字形字符判：注释里提到旧字形/旧属性不算违规，真违规是「插槽/节点/事件还在」
    const navCode = nav.replace(/<!--[\s\S]*?-->/g, '');
    expect(navCode).not.toMatch(/<template #toolbar>/);
    expect(navCode).not.toMatch(/chat-menu-glyph/);
    expect(navCode).not.toMatch(/:show-toolbar/);
    expect(navCode).not.toMatch(/menuClick/);
    // 页头件的 #toolbar 能力本身保留（别处可能还在用），删的是本页的消费
    expect(read(NAVBAR)).toMatch(/<slot name="toolbar">/);
  });

  it('⑥ 右抽屉真的退役：文件已删、页面不再 import/mount、composable 不再有右侧状态', () => {
    expect(fs.existsSync(RIGHT)).toBe(false);
    const page = read(PAGE_PATH);
    expect(page).not.toMatch(/AiChatDrawerRight/);
    expect(page).not.toMatch(/ai-chat-drawer-right/);
    expect(read(COMPOSABLE)).not.toMatch(/sidebarRight|toggleSidebarRight/);
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
   * ⑤ 标题恒居中（2026-10-27 建立；#2761 改判）：左槽一旦有字形，旧页头件就把标题切左对齐 ——
   * 「☰ AI 助手 ⋯」全挤在左上角。**#2761 起全站页头恒居中**，centerTitle 已从页头件与包装件一并删除 ——
   * 这条锁的是「透传与 prop 都不许复活」，否则又会退回「某些页居中、某些页左对齐」的分裂。
   */
  it('⑤ 标题恒居中：centerTitle 透传与 prop 均已删除（#2761 起全站恒居中）', () => {
    expect(nav).not.toMatch(/:center-title=/);
    expect(nav).not.toMatch(/centerTitle/);
    expect(read(NAVBAR)).not.toMatch(/centerTitle/);
  });
});

/** 左抽屉承载菜单（#2786）：取代退役的「右入口可见性」一组。 */
describe('左抽屉承载菜单（#2786 单一入口面；ADR-0009 2026-11-23 修订）', () => {
  const consts = read(CONSTS);
  const left = read(LEFT);
  const page = read(PAGE_PATH);

  it('① 四件齐备：常量表四项 + 左抽屉渲染并抛 key + 页面分派四个动作，零违规', () => {
    expect(drawerCarriesMenu(consts, left, page)).toEqual([]);
  });

  it('② 判别力：删「解锁专业版」/ 抽掉分组标题 / 拆 action 接线 / 不传菜单 / 缺 pro 分支 ⇒ 各自报红', () => {
    const noPro = consts.replace(/\n\s*\{ key: 'pro', label: '解锁专业版' \},/, '');
    expect(noPro).not.toBe(consts);
    expect(drawerCarriesMenu(noPro, left, page).join()).toMatch(/没有「解锁专业版」项/);
    expect(
      drawerCarriesMenu(consts, left.replace('class="drawer-group-title"', ''), page).join()
    ).toMatch(/缺分组标题/);
    expect(
      drawerCarriesMenu(consts, left.replace('v-for="item in menuItems"', ''), page).join()
    ).toMatch(/未渲染 menuItems/);
    expect(
      drawerCarriesMenu(consts, left, page.replace('@action="onMenuAction"', '')).join()
    ).toMatch(/未接左抽屉的 action/);
    expect(
      drawerCarriesMenu(consts, left, page.replace(':menu-items="drawerMenuItems"', '')).join()
    ).toMatch(/未把菜单传给左抽屉/);
    // 「新对话」混回菜单表（两个入口 = 两个动作）
    expect(
      drawerCarriesMenu(consts.replace("{ key: 'settings'", "{ key: 'new-chat', label: '新对话' },\n    { key: 'settings'"), left, page).join()
    ).toMatch(/又混进「新对话」/);
  });
});
