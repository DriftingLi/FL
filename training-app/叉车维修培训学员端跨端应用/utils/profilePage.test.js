/**
 * 「我的」页面静态契约测试（校验 pages/profile/profile.uvue）
 *
 * .uvue 无法在 jest 中渲染，故以源码文本断言三类契约：
 * 1) 与原型图一致的分区、宫格条目与顺序；原型没有的入口不得残留
 * 2) 所有跳转目标都已在 pages.json 注册，且 navigateTo 不指向 tabBar 页
 * 3) 样式仅使用 uni-app-x 原生渲染器支持的 CSS 子集（见 AGENTS.md），且无死样式
 */
const fs = require('fs');
const path = require('path');

const PAGE_PATH = path.join(__dirname, '..', 'pages', 'profile', 'profile.uvue');
const PAGES_JSON = path.join(__dirname, '..', 'pages.json');

const src = fs.readFileSync(PAGE_PATH, 'utf8');
const pagesConf = JSON.parse(fs.readFileSync(PAGES_JSON, 'utf8'));
const registeredPages = new Set(pagesConf.pages.map((p) => p.path));
const tabPages = new Set(
  pagesConf.tabBar && pagesConf.tabBar.list ? pagesConf.tabBar.list.map((t) => t.pagePath) : []
);

/** 模块编译单元：主页面 + ./components 下全部组件（T03 手术后各为独立 .uvue 编译单元） */
const COMPONENT_DIR = path.join(__dirname, '..', 'pages', 'profile', 'components');
function moduleFiles() {
  const out = [PAGE_PATH];
  if (fs.existsSync(COMPONENT_DIR)) {
    for (const e of fs.readdirSync(COMPONENT_DIR)) {
      if (e.endsWith('.uvue')) out.push(path.join(COMPONENT_DIR, e));
    }
  }
  return out;
}

/** 跨 <script>/<style> 抽取指定区块体：template 取页面模板并内联组件模板（保原型顺序），script/style 聚合全部单元 */
function section(name) {
  const files = moduleFiles();
  if (name === 'template') {
    const pageSrc = fs.readFileSync(PAGE_PATH, 'utf8');
    const tpl = pageSrc.slice(pageSrc.indexOf('<template>'), pageSrc.lastIndexOf('</template>'));
    return inlineComponents(tpl);
  }
  const re = name === 'script' ? /<script[^>]*>([\s\S]*?)<\/script>/ : /<style[^>]*>([\s\S]*?)<\/style>/;
  return files.map((f) => {
    const s = fs.readFileSync(f, 'utf8');
    const m = re.exec(s);
    return m ? m[1] : '';
  }).join('\n');
}

/**
 * 组件模板内联：把自闭合的 PascalCase 组件标签替换为组件文件自身的模板体。
 * 原型区块顺序契约跨越组件边界依然成立——此为路径指向更新，断言强度不变。
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

const template = section('template');
const script = section('script');

/** 解析形如 { key, title, icon, path, available } 的宫格条目数组 */
function entryList(varName) {
  const decl = src.indexOf('const ' + varName + ' :');
  if (decl === -1) throw new Error('未找到数组：' + varName);
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

const toolItems = entryList('toolItems');
const serviceItems = entryList('serviceItems');
const allEntries = toolItems.concat(serviceItems);

describe('「我的」页面原型图契约', () => {
  it('自上而下包含原型要求的全部区块', () => {
    const markers = [
      'class="nav-title">我的<',
      'class="checkin-pill"',
      '学习时长',
      '坚持天数',
      '可用积分',
      '我的课程',
      'class="section-title">工具<',
      'class="section-title">服务<',
    ];
    let last = -1;
    for (const marker of markers) {
      const at = template.indexOf(marker);
      expect(at).toBeGreaterThan(-1);
      expect(at > last).toBe(true);
      last = at;
    }
  });

  it('工具宫格条目与原型一致（含顺序）', () => {
    expect(toolItems.map((i) => i.title)).toEqual([
      '学习记录', '收藏夹', '错题本', '笔记本', '学练计划', '学习资料', '就业在线', '任务中心',
    ]);
  });

  it('服务宫格条目与原型一致（含顺序，设置落在第二行）', () => {
    expect(serviceItems.map((i) => i.title)).toEqual([
      '我的订单', '帮助中心', '地址管理', '消息中心', '设置',
    ]);
  });

  it('三张数据卡的标签与原型一致', () => {
    expect(template).toContain('学习时长');
    expect(template).toContain('坚持天数');
    expect(template).toContain('可用积分');
    // 原型是三张独立卡片，而非单卡片内的三列
    expect((template.match(/class="stats-card/g) || []).length).toBe(3);
  });

  it('原型没有的入口不残留', () => {
    for (const removed of ['我的钱包', '我的考试目标', '退出登录', '连续签到', '已打卡', '去签到']) {
      expect(src.includes(removed)).toBe(false);
    }
    // 「设置」只能作为服务宫格条目出现，不应再有独立菜单列表
    expect(template.includes('menu-item')).toBe(false);
  });

  it('每项入口都有图标与文案，签到胶囊保留今日签到状态', () => {
    expect(allEntries.length).toBe(13);
    for (const item of allEntries) {
      expect(item.icon.length).toBeGreaterThan(0);
      expect(item.title.length).toBeGreaterThan(0);
    }
    expect(src.includes('todayChecked')).toBe(true);
    expect(src.includes('getCheckInCalendarApi')).toBe(true);
  });
});

describe('「我的」页面跳转契约', () => {
  it('available 条目必须有已注册的 path，占位条目 path 必须为空', () => {
    expect(allEntries.length).toBeGreaterThan(0);
    for (const item of allEntries) {
      if (item.available) {
        expect(item.path.startsWith('/pages/')).toBe(true);
        expect(registeredPages.has(item.path.slice(1))).toBe(true);
      } else {
        expect(item.path).toBe('');
      }
    }
  });

  it('原型中尚无对应页的条目走占位提示而非死链', () => {
    const placeholder = allEntries.filter((i) => !i.available).map((i) => i.key);
    expect(placeholder.sort()).toEqual(
      ['address', 'help', 'materials', 'notebook', 'orders', 'study-plan'].sort()
    );
  });

  it('写死的跳转 url 均已注册，且区分 switchTab 与 navigateTo', () => {
    const re = /uni\.(navigateTo|switchTab|reLaunch|redirectTo)\(\{\s*url:\s*'([^']+)'/g;
    const found = [];
    let m;
    while ((m = re.exec(script)) !== null) found.push({ kind: m[1], url: m[2] });
    expect(found.length).toBeGreaterThan(0);
    for (const nav of found) {
      expect(nav.url.startsWith('/pages/')).toBe(true);
      const page = nav.url.slice(1);
      expect(registeredPages.has(page)).toBe(true);
      if (nav.kind === 'switchTab') {
        expect(tabPages.has(page)).toBe(true);
      } else {
        // navigateTo 到 tabBar 页面在 uni-app 中会失败
        expect(tabPages.has(page)).toBe(false);
      }
    }
  });
});

describe('「我的」页面 uvue 兼容性（逐编译单元：页面 + 各组件独立样式作用域）', () => {
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
    expect(st.includes('max-height')).toBe(false);
  });

  it.each(units.map((f) => [path.basename(f), f]))('%s 只使用 class 选择器', (_name, file) => {
    const s = fs.readFileSync(file, 'utf8');
    const st = s.slice(s.indexOf('<style'), s.lastIndexOf('</style>'));
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(st)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button)\s*\{/m.test(st)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(st)).toBe(false);
    expect(/^[^.@}\s][\w-]*\s*\{/m.test(st)).toBe(false);
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

  it('主页面 class 使用面保持规模（原型未缩水）', () => {
    const used = new Set();
    for (const m of template.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    expect(used.size).toBeGreaterThan(20);
  });
});

