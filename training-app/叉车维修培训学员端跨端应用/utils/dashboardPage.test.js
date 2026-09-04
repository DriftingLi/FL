/**
 * 首页（dashboard.uvue）静态契约测试——对照设计稿截图的页面格式
 *
 * .uvue 无法在 jest 中渲染，故以源码文本断言三类契约：
 * 1) 格式契约：顶部图标可见、公告栏存在、四列宫格、新闻标题下划线、填充式筛选胶囊、绿色热销标签
 * 2) 跳转契约：写死的 url 均已在 pages.json 注册
 * 3) uvue 兼容性：样式仅使用 uni-app-x 原生渲染器支持的 CSS 子集，且无死样式
 */
const fs = require('fs');
const path = require('path');

const PAGE_PATH = path.join(__dirname, '..', 'pages', 'dashboard', 'dashboard.uvue');
const PAGES_JSON = path.join(__dirname, '..', 'pages.json');

const src = fs.readFileSync(PAGE_PATH, 'utf8');
const pagesConf = JSON.parse(fs.readFileSync(PAGES_JSON, 'utf8'));
const registeredPages = new Set(pagesConf.pages.map((p) => p.path));
const tabPages = new Set(
  pagesConf.tabBar && pagesConf.tabBar.list ? pagesConf.tabBar.list.map((t) => t.pagePath) : []
);

const template = src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>'));
const script = src.slice(src.indexOf('<script'), src.lastIndexOf('</script>'));
const styleBlock = src.slice(src.indexOf('<style>'), src.lastIndexOf('</style>'));

/** 解析形如 { key, title, icon, color, path, available } 的宫格条目数组 */
function menuEntries() {
  const decl = src.indexOf('const menuItems :');
  if (decl === -1) throw new Error('未找到 menuItems');
  const blockStart = src.indexOf('[', decl);
  const blockEnd = src.indexOf('\n    ]', blockStart);
  const block = src.slice(blockStart, blockEnd);
  const re = /\{\s*key:\s*'([^']+)'[^}]*?title:\s*'([^']+)'[^}]*?icon:\s*'([^']*)'[^}]*?path:\s*'([^']*)'[^}]*?available:\s*(true|false)/g;
  const out = [];
  let m;
  while ((m = re.exec(block)) !== null) {
    out.push({ key: m[1], title: m[2], icon: m[3], path: m[4], available: m[5] === 'true' });
  }
  return out;
}

describe('首页格式契约（对照设计稿截图）', () => {
  it('顶部导航两个图标必须可见（历史 bug：icon 文本为空导致图标消失）', () => {
    const icons = [...template.matchAll(/class="nav-icon-text">([^<]*)</g)].map((m) => m[1]);
    expect(icons.length).toBe(2);
    for (const icon of icons) {
      expect(icon.trim().length).toBeGreaterThan(0);
    }
  });

  it('导航下方有公告栏（icon + 文案 + 箭头）', () => {
    expect(template).toContain('class="notice-bar"');
    expect(template).toContain('class="notice-icon"');
    expect(template).toContain('class="notice-text"');
    expect(template).toContain('class="notice-arrow"');
  });

  it('功能宫格为四列（25% 宽），且没有占位死入口', () => {
    expect(template).toContain('menu-item--col4');
    expect(styleBlock).toContain('.menu-item--col4');
    expect(styleBlock).toMatch(/\.menu-item--col4\s*\{[^}]*width:\s*25%/);
    const entries = menuEntries();
    expect(entries.length).toBe(4);
    for (const item of entries) {
      expect(item.available).toBe(true);
      expect(item.path.length).toBeGreaterThan(0);
      expect(item.icon.length).toBeGreaterThan(0);
    }
  });

  it('新闻资讯标题下有蓝色短划线', () => {
    expect(template).toContain('class="section-underline"');
    expect(styleBlock).toMatch(/\.section-underline\s*\{[^}]*background-color:\s*#239EDD/);
  });

  it('筛选胶囊为填充式（无描边）：默认灰底、激活浅蓝底蓝字', () => {
    expect(styleBlock).toMatch(/\.filter-tag\s*\{[^}]*background-color:\s*#ECECEC/);
    expect(styleBlock).toMatch(/\.filter-tag-active\s*\{[^}]*background-color:\s*#EBF9FF/);
    expect(styleBlock).toMatch(/\.filter-tag-text-active\s*\{[^}]*color:\s*#239EDD/);
    expect(styleBlock).not.toMatch(/\.filter-tag\s*\{[^}]*border:\s*1rpx solid/);
  });

  it('热销标签为绿色胶囊，课程小标签为无底灰字，价格红色', () => {
    expect(styleBlock).toMatch(/\.course-tag-hot\s*\{[^}]*background-color:\s*#00A443/);
    expect(styleBlock).toMatch(/\.course-tag-hot\s*\{[^}]*border-radius:\s*999rpx/);
    expect(styleBlock).not.toMatch(/\.course-tag-sm\s*\{[^}]*background-color/);
    expect(styleBlock).toMatch(/\.course-price\s*\{[^}]*color:\s*#FF2424/);
  });

  it('页面底部为 tabBar 预留留白', () => {
    expect(template).toContain('class="page-footer-space"');
    expect(styleBlock).toMatch(/\.page-footer-space\s*\{[^}]*height:\s*120rpx/);
  });

  it('banner 两侧留白为 40rpx', () => {
    expect(styleBlock).toMatch(/\.banner\s*\{[^}]*margin:\s*24rpx 40rpx/);
  });
});

describe('首页跳转契约', () => {
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

describe('首页 uvue 兼容性', () => {
  it('不使用 uvue 不支持的 CSS 属性与单位', () => {
    expect(/(^|[;{\s])gap\s*:/.test(styleBlock)).toBe(false);
    expect(styleBlock.includes('row-gap')).toBe(false);
    expect(styleBlock.includes('column-gap')).toBe(false);
    expect(styleBlock.includes('var(--')).toBe(false);
    expect(styleBlock.includes('currentColor')).toBe(false);
    expect(styleBlock.includes('calc(')).toBe(false);
    expect(/\d+(vh|vw)\b/.test(styleBlock)).toBe(false);
    expect(styleBlock.includes('display: grid')).toBe(false);
    expect(/transition|animation/.test(styleBlock)).toBe(false);
  });

  it('只使用 class 选择器（无标签/伪类/ID 选择器）', () => {
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(styleBlock)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button)\s*\{/m.test(styleBlock)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(styleBlock)).toBe(false);
  });

  it('模板与样式中的 class 一一对应（无死样式、无裸 class）', () => {
    const used = new Set();
    for (const m of template.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    for (const m of template.matchAll(/:class="([^"]+)"/g)) {
      for (const q of m[1].matchAll(/'([^']+)'/g)) used.add(q[1]);
    }
    const defined = new Set();
    for (const m of styleBlock.matchAll(/\.([A-Za-z][A-Za-z0-9_-]*)/g)) defined.add(m[1]);

    const undefinedClasses = [...used].filter((c) => !defined.has(c));
    const deadClasses = [...defined].filter((c) => !used.has(c));
    expect(undefinedClasses).toEqual([]);
    expect(deadClasses).toEqual([]);
    expect(used.size).toBeGreaterThan(20);
  });
});
