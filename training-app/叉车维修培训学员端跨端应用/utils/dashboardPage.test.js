/**
 * 首页（dashboard 模块）静态契约测试——对照设计稿截图的页面格式
 *
 * .uvue 无法在 jest 中渲染，故以源码文本断言三类契约：
 * 1) 格式契约：顶部品牌与操作区可见、无公告栏、四列宫格、新闻标题下划线、填充式筛选胶囊、绿色热销标签
 * 2) 跳转契约：写死的 url 均已在 pages.json 注册
 * 3) uvue 兼容性：样式仅使用 uni-app-x 原生渲染器支持的 CSS 子集，且无死样式
 *
 * T05 手术（#643）后为多编译单元：断言跨「页面 + pages/dashboard/components/**」聚合，
 * 模板契约经组件模板内联保持原有顺序与可见性（路径指向更新，断言强度不变，先例 profilePage）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE_PATH = path.join(ROOT, 'pages', 'dashboard', 'dashboard.uvue');
const COMPONENT_DIR = path.join(ROOT, 'pages', 'dashboard', 'components');
const PAGES_JSON = path.join(ROOT, 'pages.json');

const src = readText(PAGE_PATH);
const pagesConf = JSON.parse(readText(PAGES_JSON));
// 注册面 = pages ∪ subPackages（subPackages 的 path 相对各自 root）——#2579 起非 tabBar 页整目录切出主包，
// 只认 pages 段会把分包页误判成死链（同 #2430 招聘端切分包时的口径）。
const registeredPages = new Set([
  ...pagesConf.pages.map((p) => p.path),
  ...(pagesConf.subPackages || []).flatMap((sp) => (sp.pages || []).map((p) => `${sp.root}/${p.path}`)),
]);
const tabPages = new Set(
  pagesConf.tabBar && pagesConf.tabBar.list ? pagesConf.tabBar.list.map((t) => t.pagePath) : []
);

/** 模块编译单元：主页面 + ./components 下全部组件（T05 手术后各为独立 .uvue 编译单元） */
function moduleFiles() {
  const out = [PAGE_PATH];
  if (fs.existsSync(COMPONENT_DIR)) {
    for (const e of fs.readdirSync(COMPONENT_DIR)) {
      if (e.endsWith('.uvue')) out.push(path.join(COMPONENT_DIR, e));
    }
  }
  return out;
}

/**
 * 组件模板内联：把自闭合的 PascalCase 组件标签替换为组件文件自身的模板体。
 * 区块顺序契约跨越组件边界依然成立——此为路径指向更新，断言强度不变。
 */
function inlineComponents(tpl) {
  return tpl.replace(/<([A-Z][A-Za-z0-9]*)\b[^>]*\/>/g, (whole, tag) => {
    const kebab = tag.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
    const compPath = path.join(COMPONENT_DIR, kebab + '.uvue');
    if (!fs.existsSync(compPath)) return whole;
    const csrc = readText(compPath);
    const open = csrc.indexOf('<template>');
    const close = csrc.lastIndexOf('</template>');
    if (open === -1 || close === -1) return whole;
    return csrc.slice(open + 10, close);
  });
}

const template = inlineComponents(src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>')));
const script = moduleFiles().map((f) => {
  const s = readText(f);
  const m = /<script[^>]*>([\s\S]*?)<\/script>/.exec(s);
  return m ? m[1] : '';
}).join('\n');
const styleBlock = moduleFiles().map((f) => {
  const s = readText(f);
  const m = /<style[^>]*>([\s\S]*?)<\/style>/.exec(s);
  return m ? m[1] : '';
}).join('\n');

/** 解析形如 { key, title, icon, path, available } 的宫格条目数组（T05 手术后住 MenuGrid 组件；icon = static/icons/dashboard 下文件名） */
function menuEntries() {
  const menuSrc = readText(path.join(COMPONENT_DIR, 'dashboard-menu-grid.uvue'));
  const decl = menuSrc.indexOf('const menuItems :');
  if (decl === -1) throw new Error('未找到 menuItems');
  const blockStart = menuSrc.indexOf('[', decl);
  const blockEnd = menuSrc.indexOf('\n    ]', blockStart);
  const block = menuSrc.slice(blockStart, blockEnd);
  const re = /\{\s*key:\s*'([^']+)'[^}]*?title:\s*'([^']+)'[^}]*?icon:\s*'([^']*)'[^}]*?path:\s*'([^']*)'[^}]*?available:\s*(true|false)/g;
  const out = [];
  let m;
  while ((m = re.exec(block)) !== null) {
    out.push({ key: m[1], title: m[2], icon: m[3], path: m[4], available: m[5] === 'true' });
  }
  return out;
}

describe('首页格式契约（对照设计稿截图）', () => {
  it('HRWAI 品牌切图落在共享页头左槽，证件与动作仍留在业务栏', () => {
    const left = /<template #left>([\s\S]*?)<\/template>/.exec(template);
    expect(left).not.toBeNull();
    expect(left[1]).toContain('class="dashboard-brand"');
    expect(left[1]).toContain('src="/static/icons/dashboard/hrwai-brand.png"');
    expect(template).not.toMatch(/nav-bell/);
    expect(template).not.toMatch(/onNotificationClick/);
    expect(template).not.toMatch(/:right-icons="/);
    expect(template).toContain('class="credential-bar"');
    expect(template).toContain('class="credential-picker"');
    expect(template).toContain('@click="onHeaderSearch"');
    expect(template).toContain('@click="onHeaderMall"');
    expect(styleBlock).toMatch(/\.dashboard-brand\s*\{[^}]*width:\s*58rpx/);
    expect(styleBlock).toMatch(/\.dashboard-brand\s*\{[^}]*height:\s*68rpx/);
    expect(styleBlock).toMatch(/\.dashboard-brand\s*\{[^}]*margin-left:\s*16rpx/);
    const asset = path.join(ROOT, 'static', 'icons', 'dashboard', 'hrwai-brand.png');
    expect(fs.existsSync(asset)).toBe(true);
    expect(fs.statSync(asset).size).toBeGreaterThan(0);
  });

  it('证件栏为原型纯文本形态：88rpx 命中行、无胶囊底色、下拉箭头用切图', () => {
    expect(template).toContain('class="credential-arrow"');
    expect(template).toContain('src="/static/icons/dashboard/credential-arrow.png"');
    expect(template).not.toContain('class="credential-arrow">⌄');
    expect(styleBlock).toMatch(/\.credential-picker\s*\{[^}]*height:\s*88rpx/);
    expect(styleBlock).not.toMatch(/\.credential-picker\s*\{[^}]*background-color/);
    expect(styleBlock).not.toMatch(/\.credential-picker\s*\{[^}]*border-radius/);
    expect(styleBlock).not.toMatch(/\.credential-picker\s*\{[^}]*padding/);
    expect(styleBlock).toMatch(/\.credential-name\s*\{[^}]*font-size:\s*32rpx/);
    expect(styleBlock).toMatch(/\.credential-name\s*\{[^}]*color:\s*\$text-color/);
    expect(styleBlock).toMatch(/\.credential-arrow\s*\{[^}]*width:\s*24rpx/);
    expect(styleBlock).toMatch(/\.credential-arrow\s*\{[^}]*height:\s*24rpx/);
    expect(styleBlock).toMatch(/\.credential-arrow\s*\{[^}]*margin-left:\s*14rpx/);
    const asset = path.join(ROOT, 'static', 'icons', 'dashboard', 'credential-arrow.png');
    expect(fs.existsSync(asset)).toBe(true);
    expect(fs.statSync(asset).size).toBeGreaterThan(0);
  });

  it('搜索与商城使用原型线性切图，88rpx 命中区不重叠且图标边缘归属正确', () => {
    expect(template).toContain('src="/static/icons/dashboard/header-search.png"');
    expect(template).toContain('src="/static/icons/dashboard/header-mall.png"');
    expect(template).toContain('class="header-action header-action--search"');
    expect(template).not.toContain('/static/icons/mall/search.png');
    expect(template).not.toContain('/static/icons/dashboard/grid-course-mall.png');
    expect(styleBlock).toMatch(/\.header-action\s*\{[^}]*position:\s*relative/);
    expect(styleBlock).toMatch(/\.header-action\s*\{[^}]*width:\s*88rpx/);
    expect(styleBlock).toMatch(/\.header-action\s*\{[^}]*height:\s*88rpx/);
    expect(styleBlock).toMatch(/\.header-action-icon\s*\{[^}]*position:\s*absolute/);
    expect(styleBlock).toMatch(/\.header-action-icon\s*\{[^}]*top:\s*28rpx/);
    expect(styleBlock).toMatch(/\.header-action-icon\s*\{[^}]*width:\s*32rpx/);
    expect(styleBlock).toMatch(/\.header-action-icon\s*\{[^}]*height:\s*32rpx/);
    expect(styleBlock).toMatch(/\.header-actions\s*\{[^}]*margin-right:\s*-13rpx/);
    expect(styleBlock).toMatch(/\.header-action--search\s+\.header-action-icon\s*\{[^}]*right:\s*0/);
    expect(styleBlock).toMatch(/\.header-action--mall\s+\.header-action-icon\s*\{[^}]*left:\s*18rpx/);
    expect(styleBlock).not.toMatch(/\.header-action--mall\s*\{[^}]*margin-left/);
    for (const name of ['header-search.png', 'header-mall.png']) {
      const asset = path.join(ROOT, 'static', 'icons', 'dashboard', name);
      expect(fs.existsSync(asset)).toBe(true);
      expect(fs.statSync(asset).size).toBeGreaterThan(0);
    }
  });

  it('原型第二行后无公告栏，Banner 是滚动内容首项', () => {
    expect(template).not.toMatch(/notice-bar|notice-icon|notice-text|notice-arrow|onNoticeClick/);
    expect(styleBlock).not.toMatch(/\.notice-/);
    expect(script).not.toContain('function onNoticeClick()');
    expect(template.indexOf('class="banner"')).toBeGreaterThan(template.indexOf('class="credential-bar"'));
  });

  it('功能宫格为四列（25% 宽），不包含首页全局搜索入口', () => {
    expect(template).toContain('menu-item--col4');
    expect(styleBlock).toContain('.menu-item--col4');
    expect(styleBlock).toMatch(/\.menu-item--col4\s*\{[^}]*width:\s*25%/);
    const entries = menuEntries();
    expect(entries.length).toBe(4);
    expect(entries.map((e) => e.path)).not.toContain('/pages/search/search');
    for (const item of entries) {
      expect(item.available).toBe(true);
      expect(item.path.length).toBeGreaterThan(0);
      expect(item.icon.length).toBeGreaterThan(0);
    }
  });

  it('宫格图标为原型 404_2195 切图（#2609）：包内 PNG 字面量路径、emoji 兜底退场、切图真在包里', () => {
    const entries = menuEntries();
    // 只看宫格件本身（整页模板含其他组件的 emoji，如课程区兜底）
    const gridTpl = readText(path.join(COMPONENT_DIR, 'dashboard-menu-grid.uvue'));
    // 原型四格 3D 插画 + 全局搜索入口使用同系列立体放大镜
    expect(gridTpl).toContain('class="menu-icon-img"');
    expect(gridTpl).toMatch(/<image[^>]*:src="'\/static\/icons\/dashboard\/' \+ item\.icon \+ '\.png'"/);
    // 包内图一律 PNG（ADR-0066 ①）；不再有 emoji 圆角块兜底
    expect(gridTpl).not.toContain('menu-icon-box');
    for (const emoji of ['🛒', '📚', '📣', '🔍']) {
      expect(gridTpl).not.toContain(emoji);
    }
    for (const item of entries) {
      const asset = path.join(ROOT, 'static', 'icons', 'dashboard', item.icon + '.png');
      expect(fs.existsSync(asset)).toBe(true);
      expect(fs.statSync(asset).size).toBeGreaterThan(0);
    }
  });

  it('新闻资讯标题下有蓝色短划线', () => {
    expect(template).toContain('class="section-underline"');
    // #2919 私有子件裸 hex 归零：断言钉 token 名（$primary-color canon #239EDD），不再钉字面值
    expect(styleBlock).toMatch(/\.section-underline\s*\{[^}]*background-color:\s*\$primary-color/);
  });

  it('新闻资讯：四枚分类 chips + 白卡包列表 + 序号与 MM.DD 日期（#2581 原型 404_2195）', () => {
    expect(template).toContain('class="news-filter-row"');
    expect(template).toContain('class="news-card"');
    expect(template).toContain('class="news-index"');
    // chips 文案单点在 utils/dashboardNews（组件只引用，不另抄一份）
    expect(script).toContain('DASHBOARD_NEWS_FILTERS');
    // #2919：白卡 / 激活浅蓝底 / 蓝字均走 token（canon 与旧字面值逐位同值，像素面零变化）
    expect(styleBlock).toMatch(/\.news-card\s*\{[^}]*background-color:\s*\$card-bg/);
    expect(styleBlock).toMatch(/\.news-filter-tag-active\s*\{[^}]*background-color:\s*\$primary-tint/);
    expect(styleBlock).toMatch(/\.news-filter-tag-text-active\s*\{[^}]*color:\s*\$primary-color/);
  });

  it('筛选胶囊为填充式（无描边）：默认灰底、激活浅蓝底蓝字', () => {
    expect(styleBlock).toMatch(/\.filter-tag\s*\{[^}]*background-color:\s*\$tag-bg-neutral/);
    expect(styleBlock).toMatch(/\.filter-tag-active\s*\{[^}]*background-color:\s*\$primary-tint/);
    expect(styleBlock).toMatch(/\.filter-tag-text-active\s*\{[^}]*color:\s*\$primary-color/);
    expect(styleBlock).not.toMatch(/\.filter-tag\s*\{[^}]*border:\s*1rpx solid/);
  });

  it('热销标签为绿色胶囊，课程小标签为无底灰字，价格红色', () => {
    // #2861 语义色收敛：热销绿吃 $success-color（canon #14AE50），不再钉字面值
    expect(styleBlock).toMatch(/\.course-tag-hot\s*\{[^}]*background-color:\s*\$success-color/);
    expect(styleBlock).toMatch(/\.course-tag-hot\s*\{[^}]*border-radius:\s*999rpx/);
    expect(styleBlock).not.toMatch(/\.course-tag-sm\s*\{[^}]*background-color/);
    expect(styleBlock).toMatch(/\.course-price\s*\{[^}]*color:\s*\$color-price/);
  });

  it('页面底部为 tabBar 预留留白', () => {
    expect(template).toContain('class="page-footer-space"');
    expect(styleBlock).toMatch(/\.page-footer-space\s*\{[^}]*height:\s*120rpx/);
  });

  it('banner 顶部贴住第二行、两侧留白 37rpx、圆角走大号 token', () => {
    expect(styleBlock).toMatch(/\.banner\s*\{[^}]*margin:\s*0 37rpx/);
    expect(styleBlock).toMatch(/\.banner\s*\{[^}]*border-radius:\s*\$radius-xl/);
  });
});

describe('首页跳转契约', () => {
  it('页头证件栏保留搜索与商城入口路由', () => {
    expect(script).toContain("function onHeaderSearch()");
    expect(script).toContain("url: '/pages/search/search'");
    expect(script).toContain("function onHeaderMall()");
    expect(script).toContain("url: '/pages/mall/mall'");
  });

  it('写死的跳转 url 均已注册，且区分 switchTab 与 navigateTo', () => {
    const re = /uni\.(navigateTo|switchTab|reLaunch|redirectTo)\(\{\s*url:\s*'([^']+)'/g;
    const found = [];
    let m;
    while ((m = re.exec(script)) !== null) found.push({ kind: m[1], url: m[2] });
    expect(found.length).toBeGreaterThan(0);
    for (const nav of found) {
      expect(nav.url.startsWith('/pages/')).toBe(true);
      const page = nav.url.split('?')[0].slice(1);
      expect(registeredPages.has(page)).toBe(true);
      if (nav.kind === 'switchTab') {
        expect(tabPages.has(page)).toBe(true);
      } else {
        expect(tabPages.has(page)).toBe(false);
      }
    }
  });

  it('宫格条目 path 必须是已注册页面', () => {
    for (const item of menuEntries()) {
      expect(registeredPages.has(item.path.slice(1))).toBe(true);
    }
  });
});

describe('首页 uvue 兼容性（逐编译单元：页面 + 各组件独立样式作用域）', () => {
  const units = moduleFiles();

  it.each(units.map((f) => [path.basename(f), f]))('%s 不使用 uvue 不支持的 CSS 属性与单位', (_name, file) => {
    const s = readText(file);
    const st = s.slice(s.indexOf('<style'), s.lastIndexOf('</style>'));
    expect(/(^|[;{\s])gap\s*:/.test(st)).toBe(false);
    expect(st.includes('row-gap')).toBe(false);
    expect(st.includes('column-gap')).toBe(false);
    expect(st.includes('var(--')).toBe(false);
    expect(st.includes('currentColor')).toBe(false);
    expect(st.includes('calc(')).toBe(false);
    expect(/\d+(vh|vw)\b/.test(st)).toBe(false);
    expect(st.includes('display: grid')).toBe(false);
    expect(/transition|animation/.test(st)).toBe(false);
  });

  it.each(units.map((f) => [path.basename(f), f]))('%s 只使用 class 选择器（无标签/伪类/ID 选择器）', (_name, file) => {
    const s = readText(file);
    const st = s.slice(s.indexOf('<style'), s.lastIndexOf('</style>'));
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(st)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button)\s*\{/m.test(st)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(st)).toBe(false);
  });

  it.each(units.map((f) => [path.basename(f), f]))('%s 模板与样式中的 class 一一对应（无死样式、无裸 class）', (_name, file) => {
    const s = readText(file);
    const tpl = s.slice(s.indexOf('<template>'), s.lastIndexOf('</template>'));
    const st = s.slice(s.indexOf('<style'), s.lastIndexOf('</style>'));
    const used = new Set();
    for (const m of tpl.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    for (const m of tpl.matchAll(/:class="([^"]+)"/g)) {
      for (const q of m[1].matchAll(/'([^']+)'/g)) used.add(q[1]);
    }
    const defined = new Set();
    for (const m of st.matchAll(/\.([A-Za-z][A-Za-z0-9_-]*)/g)) defined.add(m[1]);

    const undefinedClasses = [...used].filter((c) => !defined.has(c));
    const deadClasses = [...defined].filter((c) => !used.has(c));
    expect({ file: path.basename(file), undefinedClasses }).toEqual({ file: path.basename(file), undefinedClasses: [] });
    expect({ file: path.basename(file), deadClasses }).toEqual({ file: path.basename(file), deadClasses: [] });
  });

  it('模块 class 使用面保持规模（原型未缩水）', () => {
    const used = new Set();
    for (const m of template.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    expect(used.size).toBeGreaterThan(20);
  });
});
