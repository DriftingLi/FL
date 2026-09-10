/**
 * 首页（dashboard 模块）静态契约测试——对照设计稿截图的页面格式
 *
 * .uvue 无法在 jest 中渲染，故以源码文本断言三类契约：
 * 1) 格式契约：顶部图标可见、公告栏存在、四列宫格、新闻标题下划线、填充式筛选胶囊、绿色热销标签
 * 2) 跳转契约：写死的 url 均已在 pages.json 注册
 * 3) uvue 兼容性：样式仅使用 uni-app-x 原生渲染器支持的 CSS 子集，且无死样式
 *
 * T05 手术（#643）后为多编译单元：断言跨「页面 + pages/dashboard/components/**」聚合，
 * 模板契约经组件模板内联保持原有顺序与可见性（路径指向更新，断言强度不变，先例 profilePage）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const PAGE_PATH = path.join(ROOT, 'pages', 'dashboard', 'dashboard.uvue');
const COMPONENT_DIR = path.join(ROOT, 'pages', 'dashboard', 'components');
const PAGES_JSON = path.join(ROOT, 'pages.json');

const src = fs.readFileSync(PAGE_PATH, 'utf8');
const pagesConf = JSON.parse(fs.readFileSync(PAGES_JSON, 'utf8'));
const registeredPages = new Set(pagesConf.pages.map((p) => p.path));
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
    const csrc = fs.readFileSync(compPath, 'utf8');
    const open = csrc.indexOf('<template>');
    const close = csrc.lastIndexOf('</template>');
    if (open === -1 || close === -1) return whole;
    return csrc.slice(open + 10, close);
  });
}

const template = inlineComponents(src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>')));
const script = moduleFiles().map((f) => {
  const s = fs.readFileSync(f, 'utf8');
  const m = /<script[^>]*>([\s\S]*?)<\/script>/.exec(s);
  return m ? m[1] : '';
}).join('\n');
const styleBlock = moduleFiles().map((f) => {
  const s = fs.readFileSync(f, 'utf8');
  const m = /<style[^>]*>([\s\S]*?)<\/style>/.exec(s);
  return m ? m[1] : '';
}).join('\n');

/** 解析形如 { key, title, icon, color, path, available } 的宫格条目数组（T05 手术后住 MenuGrid 组件） */
function menuEntries() {
  const menuSrc = fs.readFileSync(path.join(COMPONENT_DIR, 'dashboard-menu-grid.uvue'), 'utf8');
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

describe('首页 uvue 兼容性（逐编译单元：页面 + 各组件独立样式作用域）', () => {
  const units = moduleFiles();

  it.each(units.map((f) => [path.basename(f), f]))('%s 不使用 uvue 不支持的 CSS 属性与单位', (_name, file) => {
    const s = fs.readFileSync(file, 'utf8');
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
    const s = fs.readFileSync(file, 'utf8');
    const st = s.slice(s.indexOf('<style'), s.lastIndexOf('</style>'));
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(st)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button)\s*\{/m.test(st)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(st)).toBe(false);
  });

  it.each(units.map((f) => [path.basename(f), f]))('%s 模板与样式中的 class 一一对应（无死样式、无裸 class）', (_name, file) => {
    const s = fs.readFileSync(file, 'utf8');
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
