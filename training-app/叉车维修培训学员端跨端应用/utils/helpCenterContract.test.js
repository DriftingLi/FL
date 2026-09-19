/**
 * 帮助中心契约测试（#1081）
 *
 * 票面（#1081）：移动端帮助中心 —— 接上 `pages/profile/profile.uvue` 里已存在的**禁用占位**
 * （`{ key: 'help', title: '帮助中心', icon: '🎧', available: false }`），消费 Web 侧新建的
 * `GET /api/faq`（#1079，随 PR #1088 合并）。
 *
 * 本文件钉住的契约：
 * 1) 新页在且已注册（`pages/profile/help-center.uvue` + `pages.json`）
 * 2) 入口已接线：`help` 不再是 `available: false` 死格子，path 指向已注册路由
 * 3) 数据源走 `api/faq`：页面层零直发请求；`GET /api/faq` **一次取回**（无分页、无搜索接口参数）
 * 4) 搜索**端上过滤**：页面消费 `utils/faqDisplay` 的纯函数（口径不在页面里另写一份）
 * 5) 三态齐备：加载 / 失败（带重试）/ 空态，且失败态不伪装成空态
 * 6) 折叠 Q&A：展开态数据 + 切换函数存在
 * 7) 遵守 uvue CSS 约束（只用 class 选择器、无 gap、无 CSS 变量、`<style lang="scss">`），
 *    且模板 class 与样式定义一一对应（无死样式、无裸 class）
 * 8) 答案口径：原样换行、不引 Markdown 渲染器
 *
 * 断言强度说明（ADR-0007「守护从断言文本改为断言行为」）：搜索/筛选/分组的**行为**由
 * `utils/faqDisplay.test.js` 用镜像实现断言；本文件只断言「页面确实接了那一层、没有另写一份」，
 * 两者合起来才有意义 —— 单独一条源码文本断言会放过「页面自己另实现一套坏的过滤」。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const PAGE_REL = 'pages/profile/help-center.uvue';
const API_REL = 'api/faq.uts';
const TYPES_REL = 'types/faq.uts';
const DISPLAY_REL = 'utils/faqDisplay.uts';
const PROFILE_REL = 'pages/profile/profile.uvue';

const page = read(PAGE_REL);
const api = read(API_REL);
const types = read(TYPES_REL);
const display = read(DISPLAY_REL);
const profile = read(PROFILE_REL);

/** 模板 / 脚本 / 样式三段（.uvue 单文件，按标签切） */
const tplOf = (src) => src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>'));
const scriptOf = (src) => src.slice(src.indexOf('<script'), src.lastIndexOf('</script>'));
const styleOf = (src) => src.slice(src.indexOf('<style'), src.lastIndexOf('</style>'));

describe('帮助中心页：存在与注册', () => {
  it('页面文件存在，且已在 pages.json 注册', () => {
    expect(exists(PAGE_REL)).toBe(true);
    expect(read('pages.json')).toContain('"pages/profile/help-center"');
  });

  it('pages.json 有该路由的标题与自定义导航（与 profile 模块其余页同形）', () => {
    const conf = JSON.parse(read('pages.json'));
    const route = conf.pages.find((p) => p.path === 'pages/profile/help-center');
    expect(route).toBeDefined();
    expect(route.style.navigationBarTitleText).toBe('帮助中心');
    expect(route.style.navigationStyle).toBe('custom');
  });
});

describe('入口接线：profile 服务宫格「帮助中心」', () => {
  it('help 条目 available:true 且 path 指向已注册的新页', () => {
    expect(profile).toMatch(
      /\{\s*key: 'help',\s*title: '帮助中心',\s*icon: '[^']+',\s*path: '\/pages\/profile\/help-center',\s*available: true\s*\}/,
    );
  });

  it('help 不再是死格子（`available: false` 或空 path 的残留必须为零）', () => {
    expect(profile).not.toMatch(/key: 'help'[^}]*available: false/);
    expect(profile).not.toMatch(/key: 'help'[^}]*path: ''/);
  });

  it('跳转目标已在 pages.json 注册（防幻影路由）：从 profile 点得进去', () => {
    const routes = [...read('pages.json').matchAll(/"path"\s*:\s*"([^"]+)"/g)].map((m) => m[1]);
    expect(routes).toContain('pages/profile/help-center');
    expect(profile).toContain("path: '/pages/profile/help-center'");
  });

  it('help-center 不是 tabBar 页（profile 用 navigateTo，指到 tabBar 页会失败）', () => {
    const conf = JSON.parse(read('pages.json'));
    const tabs = (conf.tabBar && conf.tabBar.list ? conf.tabBar.list : []).map((t) => t.pagePath);
    expect(tabs).not.toContain('pages/profile/help-center');
  });
});

describe('数据源：消费 GET /api/faq（一次取回，无分页无搜索接口）', () => {
  it('页面走 api/faq，页面层零直发请求（不 import request、不裸调 uni.request）', () => {
    const pageSrc = read(PAGE_REL);
    expect(pageSrc).toMatch(/from '\.\.\/\.\.\/api\/faq'/);
    expect(pageSrc).not.toMatch(/from '[^']*api\/request/);
    expect(pageSrc).not.toMatch(/uni\.request\s*\(/);
  });

  it('api/faq.uts 请求的是学员面 `/faq`，且经 getMapped 出口（域 api 统一出口）', () => {
    expect(api).toContain("getMapped<FaqCategory[]>('/faq', null");
    expect(api).toContain('export function getFaqApi');
  });

  it('不分页：请求不带 page / page_size 参数（#1079 定案：一次返回全部）', () => {
    expect(api).not.toContain('page_size');
    expect(api).not.toMatch(/page:/);
  });

  it('新增搜索接口零残留（搜索走端上过滤，ADR-0049）', () => {
    // 端上不得出现任何「搜索」语义的服务端端点
    expect(api).not.toMatch(/\/faq\/search/);
    expect(read(PAGE_REL)).not.toMatch(/\/faq\/search/);
  });

  it('响应形状按后端契约逐字段映射（ADR-0003 手动映射，不用 as 强转整体）', () => {
    for (const field of ['code', 'id', 'sort_order', 'title', 'entries']) {
      expect(api).toContain(`obj['${field}']`);
    }
    for (const field of ['id', 'question', 'answer', 'sort_order']) {
      expect(api).toContain(`obj['${field}']`);
    }
    expect(api).toContain("data['categories']");
  });

  it('空分类 entries 缺失 / null 都兜底成空数组（后端返回 []，但不假设只有一种形态）', () => {
    expect(api).toContain("const rawEntries = obj['entries']");
    expect(api).toContain('if (rawEntries != null)');
    expect(api).toContain('entries: entries');
  });
});

describe('端上搜索：页面消费 utils/faqDisplay（口径不在页面里另写一份）', () => {
  it('页面 import 了纯函数层', () => {
    expect(page).toMatch(/from '\.\.\/\.\.\/utils\/faqDisplay'/);
    for (const fn of ['buildFaqNav', 'buildFaqGroups', 'faqEmptyText', 'faqEmptyIcon']) {
      expect(page).toContain(fn);
    }
  });

  it('页面不再自己实现匹配 / 计数（防「另写一份坏过滤」）', () => {
    const script = scriptOf(page);
    expect(script).not.toContain('indexOf(k)');
    expect(script).not.toMatch(/function\s+matches\s*\(/);
    expect(script).not.toMatch(/function\s+hitCount\s*\(/);
  });

  it('搜索是端上的：input 双向绑定 + 清空 + 切分类清词', () => {
    expect(tplOf(page)).toMatch(/v-model="keyword"/);
    expect(scriptOf(page)).toContain('onClearKeyword');
    expect(scriptOf(page)).toContain('onSelectCategory');
    // 切分类即清关键词：搜索态忽略分类选择，两者互斥
    expect(scriptOf(page)).toMatch(/activeCode\.value = code\s*\n\s*keyword\.value = ''/);
  });

  it('faqDisplay 是搜索口径的唯一实现（问题或答案命中即算、大小写无关）', () => {
    expect(display).toContain('export function faqMatches');
    expect(display).toContain('export function faqHitCount');
    expect(display).toContain('entry.question.toLowerCase().indexOf(k) >= 0');
    expect(display).toContain('entry.answer.toLowerCase().indexOf(k) >= 0');
  });
});

describe('三态齐备：加载 / 失败（重试）/ 空态', () => {
  it('加载态存在', () => {
    expect(tplOf(page)).toContain('加载中...');
    expect(scriptOf(page)).toContain('loading.value = true');
  });

  it('失败态与空态**分开**（失败不伪装成「暂无内容」）', () => {
    const tpl = tplOf(page);
    expect(scriptOf(page)).toContain('failed.value = true');
    expect(tpl).toMatch(/v-if="!loading && failed"/);
    expect(tpl).toMatch(/v-if="!loading && !failed && groups\.length == 0"/);
  });

  it('失败态有可点的重试位（不是只有一句报错）', () => {
    const tpl = tplOf(page);
    expect(tpl).toContain('帮助内容加载失败');
    expect(tpl).toMatch(/@click="reload"/);
    expect(scriptOf(page)).toMatch(/function reload\(\)/);
  });

  it('空态文案由纯函数给出（搜索无命中 ≠ 内容未就绪），页面不硬编码两份文案', () => {
    expect(scriptOf(page)).toContain('faqEmptyText(keyword.value)');
    expect(scriptOf(page)).toContain('faqEmptyIcon(keyword.value)');
  });

  it('重试不重入：loading 中再次进入被拦下', () => {
    expect(scriptOf(page)).toMatch(/if \(loading\.value\) return/);
  });
});

describe('折叠 Q&A', () => {
  it('展开态是数据驱动的数组（uvue 无 CSS 过渡，靠 v-if 切换）', () => {
    expect(scriptOf(page)).toContain('expandedIds');
    expect(scriptOf(page)).toContain('function isExpanded');
    expect(scriptOf(page)).toContain('function onToggle');
    expect(tplOf(page)).toMatch(/v-if="isExpanded\(e\.id\)"/);
    expect(tplOf(page)).toMatch(/@click="onToggle\(e\.id\)"/);
  });

  it('展开是幂等集合语义：重复 toggle 同一 id 回到收起（不是累加重复项）', () => {
    expect(scriptOf(page)).toContain('if (!removed) next.push(id)');
  });

  it('展开指示器随状态变化（▾ / ▸）', () => {
    expect(tplOf(page)).toMatch(/isExpanded\(e\.id\) \? '▾' : '▸'/);
  });
});

describe('答案口径：纯文本、原样换行（不引 Markdown 渲染器）', () => {
  it('答案直接以文本渲染，不经 markdown 渲染链路', () => {
    expect(tplOf(page)).toContain('{{ e.answer }}');
    expect(page).not.toMatch(/utils\/markdown/);
    expect(page).not.toContain('parseMarkdown');
  });

  it('answer 在 api / 纯函数层都不被改写（只取值与命中判定）', () => {
    expect(api).toContain("answer: toStr(obj['answer'])");
    expect(api).not.toMatch(/\.answer\.(replace|split|trim)/);
    expect(display).not.toMatch(/entry\.answer\.(replace|split)/);
  });
});

describe('类型与契约来源', () => {
  it('types/faq.uts 定义四个形状并从 barrel 再导出', () => {
    for (const t of ['export type FaqEntry', 'export type FaqCategory', 'export type FaqNavItem', 'export type FaqGroup']) {
      expect(types).toContain(t);
    }
    expect(read('types/index.uts')).toContain(
      "export { type FaqEntry, type FaqCategory, type FaqNavItem, type FaqGroup } from './faq'",
    );
  });

  it('字段名与 Web 侧生成契约逐字一致（faq.ts 的 FaqEntryDTO / FaqCategoryDTO）', () => {
    const web = read('../../frontend/src/api/generated/faq.ts');
    for (const t of ['export interface FaqEntryDTO', 'export interface FaqCategoryDTO']) {
      expect(web).toContain(t);
    }
    // 学员面条目字段：id / question / answer / sort_order（无 published / category_id —— 那些属管理面）
    const webEntry = web.slice(web.indexOf('export interface FaqEntryDTO'), web.indexOf('export interface FaqResult'));
    for (const f of ['id', 'question', 'answer', 'sort_order']) {
      expect(webEntry).toContain(`${f}:`);
    }
    expect(webEntry).not.toContain('published');
  });
});

describe('uvue CSS 约束（逐条对齐 AGENTS.md 的兼容性表）', () => {
  /** 去掉 CSS 注释后再判：注释里的属性名 / 类名不是实现（否则「说明为什么禁它」的注释会把守护本身判红） */
  const stripCssComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '');
  const style = stripCssComments(styleOf(page));

  it('style 块声明 lang="scss"', () => {
    expect(page).toMatch(/<style lang="scss">/);
  });

  it('不使用 uvue 不支持的 CSS 属性与单位', () => {
    expect(/(^|[;{\s])gap\s*:/.test(style)).toBe(false);
    expect(style).not.toContain('row-gap');
    expect(style).not.toContain('column-gap');
    expect(style).not.toContain('var(--');
    expect(style).not.toContain('currentColor');
    expect(style).not.toContain('calc(');
    expect(/\d+(vh|vw)\b/.test(style)).toBe(false);
    expect(style).not.toContain('display: grid');
    expect(style).not.toContain('grid-template');
    expect(/transition|animation/.test(style)).toBe(false);
    expect(style).not.toContain('max-height');
    expect(style).not.toContain('text-decoration');
    expect(style).not.toContain('align-items: baseline');
  });

  it('只使用 class 选择器（无 tag / id / 伪类 / 属性选择器）', () => {
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(style)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button|input)\s*\{/m.test(style)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(style)).toBe(false);
    expect(/^[^.@}\s][\w-]*\s*\{/m.test(style)).toBe(false);
  });

  it('不写 `white-space`：uvue 原生端只支持 `<text>` / `<button>`（真机日志实测的错行）', () => {
    // 2026-09-17 真机（①a）实测：`.category-scroll`（scroll-view）上的 `white-space: nowrap`
    // 被渲染层判错并忽略 —— `style property white-space is only supported on <text>|<button>`。
    // 本条是按**设备侧日志**补的守护：AGENTS.md 的 uvue CSS 黑名单里没有这一项，静态表查不出来。
    // 横滑行改由 `flex-direction: row` + 子项 `flex-shrink: 0` 撑出溢出（已同批真机复验）。
    expect(style).not.toContain('white-space');
  });

  it('模板 class 与样式定义一一对应（无死样式、无裸 class）', () => {
    const tpl = tplOf(page);
    const script = scriptOf(page);
    const used = new Set();
    for (const m of tpl.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    for (const m of tpl.matchAll(/:class="([^"]+)"/g)) {
      const expr = m[1];
      for (const q of expr.matchAll(/'?([A-Za-z][A-Za-z0-9_-]*)'?\s*:/g)) used.add(q[1]);
      for (const q of expr.matchAll(/'([^']+)'/g)) {
        const after = expr.slice(q.index + q[0].length);
        const before = expr.slice(0, q.index).trimEnd();
        if (q[1].length > 0 && !after.trimStart().startsWith(':') && !/(==|!=|===|!==)$/.test(before)) used.add(q[1]);
      }
      for (const fn of expr.matchAll(/([A-Za-z_$][\w$]*)\s*\(/g)) {
        const fstart = script.indexOf('function ' + fn[1] + '(');
        if (fstart === -1) continue;
        const fend = script.indexOf('\n    }', fstart);
        const body = script.slice(fstart, fend === -1 ? undefined : fend);
        for (const q of body.matchAll(/'([^'\n]+)'/g)) if (q[1].length > 0) used.add(q[1]);
      }
    }
    const defined = new Set();
    for (const m of style.matchAll(/\.([A-Za-z][A-Za-z0-9_-]*)/g)) defined.add(m[1]);

    expect({ undefinedClasses: [...used].filter((c) => !defined.has(c)) }).toEqual({ undefinedClasses: [] });
    expect({ deadClasses: [...defined].filter((c) => !used.has(c)) }).toEqual({ deadClasses: [] });
  });

  it('注入自检：检测器对坏样本判红（防空跑假绿）', () => {
    const bad = [
      ['gap 简写', '.a { gap: 8rpx; }', (s) => /(^|[;{\s])gap\s*:/.test(s)],
      ['row-gap', '.a { row-gap: 8rpx; }', (s) => s.includes('row-gap')],
      ['CSS 变量', '.a { color: var(--x); }', (s) => s.includes('var(--')],
      ['calc', '.a { width: calc(100% - 20rpx); }', (s) => s.includes('calc(')],
      ['vh 单位', '.a { height: 50vh; }', (s) => /\d+(vh|vw)\b/.test(s)],
      ['grid', '.a { display: grid; }', (s) => s.includes('display: grid')],
      ['transition', '.a { transition: all .2s; }', (s) => /transition|animation/.test(s)],
      ['tag 选择器', 'view { flex: 1; }', (s) => /^\s*(view|text|image|scroll-view|button|input)\s*\{/m.test(s)],
    ];
    for (const [name, sample, detector] of bad) {
      expect([name, detector(sample)]).toEqual([name, true]);
    }
  });
});
