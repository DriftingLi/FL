/**
 * 批5 UI 改造契约（#1608：N05 透明页头 7 页收口）
 *
 * 形态沿用批4 的五硬指标机检，落位真源 = #1595 台账 + #1608 票面 + #1592 终审表：
 *
 * 1. 声明位裸 hex ∈ token 值 ∪ §1.6 白名单（utils/uiHexWhitelist.js）
 * 2. `<style lang="scss">` 100%
 * 3. 手写 `.status-bar` = 0 且页头件引用 100%（N05：transparent、页面自垫底色）
 * 4. 页头件带 transparent（N05 判据；ai-assistant 走 ai-chat-nav 包装链，特判见下）
 * 5. 主色 #239EDD 字面值 = 0
 *
 * 随批收口（§8⑤ dashboard 筛选单轴）的行为钉点在 dashboardContract.test.js /
 * coursesContract.test.js（数据流归属在那两处），本文件不重复钉。
 *
 * 判据来源：docs/ui-spec.md §6.1/§7.1/§9；#1608 票面验收标准。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** 批5 七页（N05 透明套，#1592 落位表：G2×5 + G6 recruiter/login + G7 resume） */
const PAGES = [
  'pages/dashboard/dashboard.uvue',
  'pages/forum/forum.uvue',
  'pages/ai-assistant/ai-assistant.uvue',
  'pages/courses/courses.uvue',
  'pages/profile/profile.uvue',
  'pages/recruiter/login.uvue',
  'pages/resume/resume.uvue',
];

/** token 值集合（uni.scss 的 $-变量值，归一成大写 6 位） */
const UNI_SCSS = read('uni.scss');
const tokenValues = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)].map((m) =>
    m[1].toUpperCase(),
  ),
);

/** 归一：3 位 hex 折成 6 位、大写 */
function normalizeHex(hex) {
  const h6 = hex.length === 4
    ? hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3]
    : hex.slice(1, 7);
  return `#${h6}`.toUpperCase();
}

function styleBlock(src) {
  const start = src.indexOf('<style');
  if (start < 0) return '';
  return src.slice(start, src.indexOf('</style>'));
}

function hexViolations(src) {
  // 先剥注释（/* */ 与 // 行）——issue 引用如 #947/#1140 会伪装成 3 位 hex（ui-spec §9 只判声明位）
  const style = styleBlock(src)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^\s*\/\/.*$/gm, '');
  const hexes = style.match(/#[0-9a-fA-F]{3,8}\b/g) || [];
  return hexes.filter((raw) => {
    const v = normalizeHex(raw);
    return !tokenValues.has(v) && !WHITELIST[v];
  });
}

for (const p of PAGES) {
  const src = read(p);

  describe(`${p}（ui-spec §9 机检）`, () => {
    it('H1 样式块声明位裸 hex ∈ token 值 ∪ §1.6 白名单', () => {
      expect(hexViolations(src)).toEqual([]);
    });

    it('H2 样式块声明 lang="scss"', () => {
      expect(styleBlock(src)).toMatch(/<style lang="scss">/);
    });

    it('H5 主色不出现字面值（#239EDD 必须写 $primary-color）', () => {
      expect(styleBlock(src)).not.toMatch(/#239EDD/i);
    });
  });

  describe(`${p}（页头归位 §7.1 N05 透明套）`, () => {
    it('引用页头件且 transparent，不再手写 .status-bar / 手写状态栏高度', () => {
      const s = read(p);
      expect(s).not.toMatch(/class="status-bar"/);
      expect(s).not.toContain('useStatusBarHeight');
      if (p === 'pages/ai-assistant/ai-assistant.uvue') {
        // N05 经包装链成立：页面 import ai-chat-nav，包装件内部引用 app-nav-bar 且默认 transparent
        expect(s).toContain("from '../../components/ai-chat/ai-chat-nav.uvue'");
        const nav = read('components/ai-chat/ai-chat-nav.uvue');
        expect(nav).toContain("from '../app-nav-bar/app-nav-bar.uvue'");
        expect(nav).toMatch(/transparent\s*:\s*true/);
        // 页面对包装件的状态栏死绑定已删（占位 prop 仅供旧调用方，抽屉两处绑定不在此列）
        expect(s).not.toMatch(/<AiChatNav[^>]*status-bar-height/);
      } else {
        expect(s).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
        expect(s).toMatch(/<(app-nav-bar|AppNavBar)[^>]*transparent/);
      }
    });
  });
}

describe('批5 N05 页面自垫底色（页头透明后背景由页面承担，§6.1/§7.1）', () => {
  it('G2 渐变三页（dashboard/courses/forum）容器走 $gradient-start/$gradient-end token', () => {
    for (const p of [
      'pages/dashboard/dashboard.uvue',
      'pages/courses/courses.uvue',
      'pages/forum/forum.uvue',
    ]) {
      const style = styleBlock(read(p));
      expect(style).toMatch(/linear-gradient\(to bottom,\s*\$gradient-start,\s*\$gradient-end\)/);
      expect(style).toMatch(/background-color:\s*\$gradient-end/);
    }
  });

  it('profile 页自垫蓝渐变对（白名单在册，#1592 保留视觉不并 token）', () => {
    const style = styleBlock(read('pages/profile/profile.uvue'));
    expect(style).toMatch(/linear-gradient\(to bottom,\s*#6ECCFD,\s*#0CA2EF\)/i);
  });

  it('resume 页自垫灰底 $bg-header 语义值 #F5F6FA（白名单在册）；login 自垫 $bg-header token', () => {
    expect(styleBlock(read('pages/resume/resume.uvue'))).toMatch(/#f5f6fa/i);
    expect(styleBlock(read('pages/recruiter/login.uvue'))).toMatch(/background(-color)?:\s*\$bg-header/);
  });

  it('ai-assistant 页自垫 #DFEFFB（白名单在册，批2 先例）', () => {
    expect(styleBlock(read('pages/ai-assistant/ai-assistant.uvue'))).toMatch(/#dfeffb/i);
  });
});

describe('批5 特殊页头形态（app-nav-bar 槽位承载）', () => {
  it('forum 右槽仅保留消息徽标入口（#1846 收敛：发帖 pill 下沉页内搜索栏右端；✉️ 单图标 64rpx 落在典型可用槽宽约 164rpx 内）', () => {
    const s = read('pages/forum/forum.uvue');
    // 右槽只剩 ✉️（含未读徽标）；发帖 pill 不得回流页头（右槽超宽会顶进胶囊矩形，spec 20261023 §4.2）
    const right = /<template #right>([\s\S]*?)<\/template>/.exec(s);
    expect(right).not.toBeNull();
    expect(right[1]).toMatch(/msg-icon-wrap[\s\S]*?msg-badge/);
    expect(right[1]).not.toMatch(/create-pill|header-right|发帖/);
    // 移出页头的入口页内可达（#1846）：搜索栏右端 pill 与搜索栏本体同走 onCreate（触发行为不变）
    expect(s).toMatch(/v-if="currentTab !== 'experience'" class="res-search-bar"/);
    expect(s).toMatch(/class="create-pill"[\s\S]{0,120}?@click="onCreate"/);
  });

  it('dashboard 页头保留证件下拉标题与单图标右槽（#1846 收敛：🔍 移出页头进功能宫格，3→2；🛒 移出页头——商城入口已在宫格「课程商城」，右槽 2→1；#1602 形态其余不动）', () => {
    const s = read('pages/dashboard/dashboard.uvue');
    expect(s).toMatch(/:right-icons="navRightIcons"/);
    expect(s).toMatch(/navRightIcons\s*:\s*string\[\]\s*=\s*\['🔔'\]/);
    expect(s).not.toContain("'🛒'");
    expect(s).toMatch(/<AppNavBar[^>]*@left-click="onCertTap"/);
    // 移出页头的入口页内可达（#1846）：宫格补全局搜索格（静态面与跳转语义另由 dashboardContract / searchContract M2 钉住）
    expect(read('pages/dashboard/components/dashboard-menu-grid.uvue')).toContain("path: '/pages/search/search'");
  });

  it('resume 页头右槽 🔔 通知入口 + 左返回（title 空）', () => {
    const s = read('pages/resume/resume.uvue');
    expect(s).toMatch(/<app-nav-bar[^>]*@left-click="onBack"[^>]*:right-icons="\['🔔'\]"[^>]*@right-click="onNotifications"/);
  });

  it('recruiter/login 页头返回走 leftClick（#1604 自绘头豁免已于本批清零）', () => {
    const s = read('pages/recruiter/login.uvue');
    expect(s).toMatch(/<app-nav-bar[^>]*@left-click="leaveRecruiterLogin"/);
    expect(s).not.toMatch(/class="nav-back/);
  });
});
