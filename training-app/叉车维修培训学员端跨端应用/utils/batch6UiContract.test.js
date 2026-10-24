/**
 * 批6 UI 改造契约（#1609：N06 无页头白名单 6 页收口 + 全站页面级五硬指标终态）
 *
 * 两层判据：
 *  A. **全站页面级终态**（§9 五硬指标，扫描面 = `pages/<页目录>/<页>.uvue` 入口件）
 *  B. **N06 专项**（#1592 终审表定义：页面自绘、含自垫状态栏、走 token）
 *
 * 扫描面口径（AC②「全站五硬指标机检终态」的落地口径，2026-10-15 用户裁定）：
 * 「全站」= **页面级 59 个入口件**。不含页面私有子组件（`pages/<页目录>/components/`）与
 * `components/` 共享件层，两处都是**明确延后项**、不是漏测：
 *  - ui-spec §10 :377 实测「8 件共享件里 6 件 0 引用」，死件的 lang="scss"/裸 hex
 *    问题判据原文写着「扶正时一并修」= 共享件层本就不绿，本批不假装替它兑现；
 *  - 页面私有子组件的收口归属各自功能票（批6 票面只点名 6 页 + 裁定 A 补入 mall）。
 * 这条口径写在套件里而不是只写在 PR 里，是为了让「谁把扫描面扩了 / 缩了」在机检层可见。
 *
 * 豁免名单正确性（AC② 点名项，判据源 #1592 决议）：
 *  - 「页头件引用 100%」的豁免名单 = N06 六页，**精确相等**——多一页（mall 曾混进来，
 *    本批已按裁定 A 归位 N03）或少一页都判红；
 *  - 「手写 .status-bar 归零」「裸 hex 归零」**无豁免**：N06 页自垫状态栏可以保留
 *    `.status-bar` 类，但高度必须取自 utils/system.uts 唯一真源（#1602），
 *    不得再出现 `useStatusBarHeight` / 页面自写 `statusBarHeight.value =` 的第二份实现。
 *
 * 判据来源：docs/ui-spec.md §7.1/§9/§10、#1592 终审表、#1609 票面验收标准。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** N06 无页头白名单六页（#1592 终审表 + ui-spec §7.1 :307） */
const N06 = [
  'pages/index/index.uvue',
  'pages/login/login.uvue',
  'pages/register/register.uvue',
  'pages/forgot-password/forgot-password.uvue',
  'pages/profile-setup/profile-setup.uvue',
  'pages/search/search.uvue',
];

/** 页头件经包装链成立的页（批5 先例：ai-assistant → ai-chat-nav → app-nav-bar），不占豁免名单 */
const AI_ASSISTANT = 'pages/ai-assistant/ai-assistant.uvue';

/** 页面级扫描面：`pages/<页目录>/<页>.uvue` 入口件（深度 2，天然排除页面私有 components 目录） */
const PAGE_ENTRY_RE = /^pages\/[^/]+\/[^/]+\.uvue$/;
const PAGES = h.filesUnder('pages', /\.(uvue)$/, true).filter((f) => PAGE_ENTRY_RE.test(f));

/** token 值集合（uni.scss 的 $-变量值，归一大写 6 位）——与批4/批5 套件同口径 */
const UNI_SCSS = read('uni.scss');
const tokenValues = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)].map((m) =>
    m[1].toUpperCase(),
  ),
);

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
  // 剥注释：issue 引用（#947/#1140）会伪装成 3 位 hex（§9 只判声明位）
  const style = styleBlock(src)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^\s*\/\/.*$/gm, '');
  const hexes = style.match(/#[0-9a-fA-F]{3,8}\b/g) || [];
  return hexes.filter((raw) => {
    const v = normalizeHex(raw);
    return !tokenValues.has(v) && !WHITELIST[v];
  });
}

describe('批6 页面级扫描面完整性（§9 扫描对象 pages/** 的入口件层）', () => {
  it('入口件枚举为 59 页（页面增删须同步本套件与 #1595 台账，不得静默漂移）', () => {
    expect(PAGES.length).toBe(59);
  });

  it('N06 六页都在扫描面内（漏一页 = 豁免名单失去真源）', () => {
    for (const p of N06) expect(PAGES).toContain(p);
  });
});

describe('批6 全站页面级五硬指标终态（§9，AC②）', () => {
  it('指标1 声明位裸 hex = 0（∈ token 值 ∪ §1.6 白名单，无页级豁免）', () => {
    const bad = PAGES.filter((p) => hexViolations(read(p)).length > 0)
      .map((p) => `${p}: ${[...new Set(hexViolations(read(p)))].join(' ')}`);
    expect(bad).toEqual([]);
  });

  it('指标2 `<style lang="scss">` 100%（无豁免）', () => {
    const bad = PAGES.filter((p) => !styleBlock(read(p)).match(/<style lang="scss">/));
    expect(bad).toEqual([]);
  });

  it('指标3 手写 .status-bar = 0（非 N06 页一律不得自绘；N06 自垫页由专项判唯一真源）', () => {
    // #1592 决议的读法：豁免名单只给「页头件引用 100%」这一半；「手写 .status-bar 归零」的
    // 判据落在**高度来源**上——N06 页自垫可以保留 `.status-bar` 类（§7.1 :307 的定义），
    // 但高度必须取自 utils/system.uts，不得再有页面自算的第二份（专项逐条钉死）。
    const bad = PAGES.filter((p) => !N06.includes(p) && read(p).match(/class="status-bar"/));
    expect(bad).toEqual([]);
  });

  it('指标3 页头件引用 100%，且豁免名单**精确等于** N06 六页（#1592 决议）', () => {
    const noNavBar = PAGES.filter((p) => !read(p).includes("from '../../components/app-nav-bar/app-nav-bar.uvue'"));
    // ai-assistant 经包装链满足（批5 先例），从豁免名单判据里剔除后必须与 N06 逐字相等
    const exempt = noNavBar.filter((p) => p !== AI_ASSISTANT).sort();
    expect(exempt).toEqual([...N06].sort());

    // 包装链本身也要成立，否则「不引用」是绕过而非归位
    const ai = read(AI_ASSISTANT);
    expect(ai).toContain("from '../../components/ai-chat/ai-chat-nav.uvue'");
    const nav = read('components/ai-chat/ai-chat-nav.uvue');
    expect(nav).toContain("from '../app-nav-bar/app-nav-bar.uvue'");
  });

  it('指标5 主色 #239EDD 字面值 = 0（必须写 $primary-color）', () => {
    const bad = PAGES.filter((p) => styleBlock(read(p)).match(/#239EDD/i));
    expect(bad).toEqual([]);
  });
});

describe('批6 N06 专项（#1592：页面自绘、自垫状态栏走唯一真源）', () => {
  it('带状态栏占位的 N06 页 = index / login / profile-setup / search 四页（另两页维持系统导航）', () => {
    const withBar = N06.filter((p) => read(p).includes('class="status-bar"'));
    expect(withBar.sort()).toEqual([
      'pages/index/index.uvue',
      'pages/login/login.uvue',
      'pages/profile-setup/profile-setup.uvue',
      'pages/search/search.uvue',
    ].sort());
  });

  it('register / forgot-password 无自绘页头、无状态栏占位（§7.1 :310 维持系统导航不动作）', () => {
    for (const p of ['pages/register/register.uvue', 'pages/forgot-password/forgot-password.uvue']) {
      const s = read(p);
      expect(s).not.toMatch(/class="status-bar"/);
      expect(s).not.toMatch(/<(app-nav-bar|AppNavBar)/);
    }
  });

  it('自垫状态栏高度取自 utils/system.uts 的 useSafeArea，不留第二份实现', () => {
    for (const p of N06) {
      const s = read(p);
      expect(s).not.toContain('useStatusBarHeight');
      expect(s).not.toMatch(/statusBarHeight\.value\s*=/);
      if (s.includes('class="status-bar"')) {
        expect(s).toContain('import { useSafeArea }');
        expect(s).toMatch(/height:\s*safeArea\.statusBarHeight\s*\+\s*'px'/);
      }
    }
  });

  it('自垫状态栏底色走 token（$bg-header / $primary-color）或 transparent，不写裸色值', () => {
    for (const p of N06) {
      const m = styleBlock(read(p)).match(/\.status-bar\s*\{[^}]*background-color:\s*([^;]+);/);
      if (!m) continue;
      expect(['$bg-header', '$primary-color', 'transparent']).toContain(m[1].trim());
    }
  });

  it('index 着陆页「已登录冷启动落 dashboard」不变量未被本批动到（#1478，判据 1）', () => {
    const s = read('pages/index/index.uvue');
    expect(s).toMatch(/if \(auth\.isLoggedIn\.value\)[\s\S]*?uni\.reLaunch\(\{[\s\S]*?\/pages\/dashboard\/dashboard/);
  });

  it('search 顶部行右端「取消」受 capsuleLeft 约束：右 padding = 750 − capsuleLeft + 24（#1845）', () => {
    const s = read('pages/search/search.uvue');
    // 动态让位只能走内联 style（uvue 无 var()/calc()，先例 app-nav-bar 的 rightSlotStyle）
    expect(s).toMatch(/<view class="search-header"\s+:style="searchHeaderStyle"/);
    // 值取自唯一测量层 useSafeArea 的 capsuleLeft；右缘 = 胶囊左边线 − 24rpx
    expect(s).toMatch(/safeArea\.value\.capsuleLeft/);
    expect(s).toMatch(/750\s*-\s*capsuleLeft\s*\+\s*CAPSULE_CLEARANCE/);
    // 间隙常量与页头件 RIGHT_SLOT_CLEARANCE 同值（24），两处漂移在此显形
    expect(s).toMatch(/CAPSULE_CLEARANCE\s*=\s*24/);
    expect(read('components/app-nav-bar/app-nav-bar.uvue')).toMatch(/RIGHT_SLOT_CLEARANCE\s*=\s*24/);
  });

  it('防回潮：N06 现形态里唯一「顶部行右端可点元素」的 search 必须保留让位结构（#1845）', () => {
    // 锚定现形态：search 是 N06 六页中唯一在自绘头部行右端放可点元素（取消按钮）的页。
    // 让位结构与锚点共存：取消按钮还在顶部行右端一天，searchHeaderStyle 就得在
    //（拆掉它 = 回到「右缘贴屏右缘 24rpx」的胶囊冲突形态）。几何验收留真机（ADR-0035 T4 口径）。
    const s = read('pages/search/search.uvue');
    expect(s).toMatch(/class="search-cancel"/);
    expect(s).toMatch(/:style="searchHeaderStyle"/);
  });

  it('N06 负面机检：其余自绘三页头部区不得长出可点元素形态（#1845 复审）', () => {
    // 头部段口径（按 2026-10-24 三页现状锚定）：自 class="status-bar" 占位起、至各自
    // 内容区起点类止（index=.hero、login=.form、profile-setup=.section）——三页该段现状
    // 为居中 logo/标题（index=.page-tag、login/profile-setup=.header），零可点元素。
    // 段内出现 @click/@tap 即红：三页是自绘头部、无页头件的胶囊让位保护，头部右端长出
    // 可点元素就会顶进胶囊矩形；将来真要长，正路是 search 先例的 capsuleLeft 内联让位
    //（见上两条 #1845 断言），届时须先改本断言补让位判据、不得静默长出。
    // 宽口径说明：不分左右端（@tap 仓内确有使用，如 personal-activity 的列表项），
    // 现三页头部段 0 命中、无误差代价；锚点缺失或错位同样判红（fail-closed，
    // 模板重构必须带着本断言一起改）。
    const zones = [
      ['pages/index/index.uvue', 'class="hero"'],
      ['pages/login/login.uvue', 'class="form"'],
      ['pages/profile-setup/profile-setup.uvue', 'class="section"'],
    ];
    const bad = [];
    for (const [p, contentMarker] of zones) {
      const src = read(p);
      const start = src.indexOf('class="status-bar"');
      const end = src.indexOf(contentMarker);
      if (start < 0 || end <= start) {
        bad.push(`${p}: 头部段锚点漂移（status-bar@${start} → ${contentMarker}@${end}）`);
      } else if (/@(click|tap)\b/.test(src.slice(start, end))) {
        bad.push(`${p}: 头部段出现 @click/@tap 可点元素`);
      }
    }
    expect(bad).toEqual([]);
  });
});

describe('批6 mall 补漏（裁定 A：#1595 台账行 25 判 N03，批3 清单漏收，本批归位）', () => {
  const MALL = 'pages/mall/mall.uvue';

  it('页头归位 N03：引用页头件 + 分隔线 + 左返回走 leftClick，不再自绘 .header/.status-bar', () => {
    const s = read(MALL);
    expect(s).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
    expect(s).toMatch(/<AppNavBar[^>]*:divider="true"/);
    expect(s).toMatch(/<AppNavBar[^>]*@left-click="toback"/);
    expect(s).not.toMatch(/class="status-bar"/);
    expect(styleBlock(s)).not.toMatch(/^\s*\.header\s*\{/m);
    expect(s).not.toContain('useStatusBarHeight');
  });

  it('右槽跨小程序 navigator 功能不丢（app-id 与素材路径原样迁入手 #right 槽）', () => {
    const s = read(MALL);
    expect(s).toMatch(/<template #right>[\s\S]*?target="miniProgram"[\s\S]*?app-id="wx56ce2775c2b3aea8"[\s\S]*?<\/template>/);
    expect(s).toContain('/static/icons/易拉宝-006 1-1.png');
  });

  it('商城私有组件接线与证件随 /courses 下发两处行为锁不受页头迁移影响（#1609 只动页头）', () => {
    const s = read(MALL);
    for (const c of ['mall-sort-bar', 'mall-category-sidebar', 'mall-float-actions']) {
      expect(s).toContain(`./components/${c}.uvue`);
    }
    expect(s).toContain('getCourseListApi(page.value, pageSize, 0, 0, browseCredentialId.value)');
  });
});
