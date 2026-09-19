/**
 * courses 模块手术契约测试（T08，parent #646 / ADR-0007）
 *
 * 照 profile / forum / dashboard / practice / exam 先例的「模块汇总契约」口径，钉住 courses 手术交付：
 * 1) 600 行软预算：pages/courses/** 全部源文件 ≤600 行，**新建 section 组件与 composable 同样计入**
 *    （维护者裁定 B：防「把 872 行页面挪成 860 行 composable」的假达标）；api/course.uts 一并纳入
 * 2) 模块目录 ≤2 层；模块必需源文件清单完整（删任一即红）
 * 3) 页面 ↔ 组件接口对账：prop / 事件双向（改名即红，无孤儿 prop、无孤儿 emit）
 * 4) composable 接线：chapter-view 以显式 import 使用模块私有 composable，且该 composable 显式结果类型
 * 5) 组件接线零孤儿（#779 回归锁）：页面 import 的组件文件必须存在，组件目录不得有孤儿文件
 * 6) allowlist 不回潮：courses 域文件不得出现在 GUARD_ALLOWLIST
 * 7) 零直发请求：页面层不直接 uni.request
 * 8) 域 api 收紧：6 个 DTO 函数经 mapper-callback 出口（箭头包裹 build*），
 *    updateCourseProgressApi 保持 raw post 白名单；api 层 .catch 静默回退计数与术前一致（本票按删除禁区不退役回退）
 * 9) 幻影路由锁（#662 口径）：api/course.uts 的每条路由字面量都必须落在后端已注册清单内
 * 10) 删除禁区行为保持点：章节学习的计时/上报/切章补报/预览/附件下载、课程详情的收藏/学习状态/
 *     继续学习/章节跳转/失败重试逐项仍在；三条路由仍在 pages.json
 * 11) 展示纯函数唯一实现：文件图标三件套与课程分类图标/底色各只有一处实现；
 *     formatDuration 两处是**刻意不同语义**（空值兜底 '未知' vs '-'），锁住不被「顺手合并」
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const COURSES_PAGES = [
  'pages/courses/chapter-view.uvue',
  'pages/courses/course-detail.uvue',
  'pages/courses/courses.uvue',
];

/** 模块必需源文件清单（删任一即红；新增文件不触发假红，但预算/守护仍覆盖到） */
const REQUIRED_SOURCE_FILES = [
  'pages/courses/courses.uvue',
  'pages/courses/course-detail.uvue',
  'pages/courses/chapter-view.uvue',
  'pages/courses/components/chapter-markdown.uvue',
  'pages/courses/components/chapter-file-list.uvue',
  'pages/courses/components/chapter-nav.uvue',
  'pages/courses/components/course-cover-section.uvue',
  'pages/courses/components/course-progress-section.uvue',
  'pages/courses/components/course-info-grid.uvue',
  'pages/courses/components/course-chapter-list.uvue',
  'pages/courses/composables/useChapterStudy.uts',
];

/** 页面 ↔ 组件接口对账表（本票唯一新增接口面，改名必须红） */
const WIRING = [
  { page: 'pages/courses/chapter-view.uvue', tag: 'ChapterMarkdown', comp: 'pages/courses/components/chapter-markdown.uvue' },
  { page: 'pages/courses/chapter-view.uvue', tag: 'ChapterFileList', comp: 'pages/courses/components/chapter-file-list.uvue' },
  { page: 'pages/courses/chapter-view.uvue', tag: 'ChapterNav', comp: 'pages/courses/components/chapter-nav.uvue' },
  { page: 'pages/courses/course-detail.uvue', tag: 'CourseCoverSection', comp: 'pages/courses/components/course-cover-section.uvue' },
  { page: 'pages/courses/course-detail.uvue', tag: 'CourseProgressSection', comp: 'pages/courses/components/course-progress-section.uvue' },
  { page: 'pages/courses/course-detail.uvue', tag: 'CourseInfoGrid', comp: 'pages/courses/components/course-info-grid.uvue' },
  { page: 'pages/courses/course-detail.uvue', tag: 'CourseChapterList', comp: 'pages/courses/components/course-chapter-list.uvue' },
];

/** 域 api 收紧面：经 mapper-callback 出口的 DTO 函数 + 保持 raw 透传的白名单 */
const DTO_FNS = [
  'getCourseListApi',
  'getCatalogTreeSpecialties',
  'getLevelsApi',
  'getCourseDetailApi',
  'getChapterDetailApi',
  'getTagsApi',
];
const RAW_PASSTHROUGH = ['updateCourseProgressApi'];

function definePropsNames(src) {
  const m = /defineProps<\{([\s\S]*?)\}>/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/([A-Za-z_$][\w$]*)\s*\??\s*:/g)].map((x) => x[1]);
}

function defineEmitsNames(src) {
  const m = /defineEmits\(\[([^\]]*)\]\)/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/'([^']+)'/g)].map((x) => x[1]);
}

/** 页面里某个组件标签的属性区（自闭合标签） */
function usageAttrs(pageSrc, tag) {
  const m = new RegExp('<' + tag + '\\b([\\s\\S]*?)/>').exec(pageSrc);
  return m === null ? null : m[1];
}

const camelToKebab = (s) => s.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase());
const kebabToCamel = (s) => s.replace(/-([a-z])/g, (_, c) => c.toUpperCase());

/** 页面上绑定的 prop 名（`:x` / `v-bind:x`，kebab 归一为 camel） */
function boundProps(attrs) {
  const out = new Set();
  for (const m of attrs.matchAll(/(?:^|\s)(?::|v-bind:)([a-z][a-z0-9-]*)\s*=/g)) out.add(kebabToCamel(m[1]));
  return [...out];
}

/** 页面上监听的事件名（`@x`，kebab 归一为 camel） */
function boundEvents(attrs) {
  const out = new Set();
  for (const m of attrs.matchAll(/(?:^|\s)@([a-z][a-z0-9-]*)\s*=/g)) out.add(kebabToCamel(m[1]));
  return [...out];
}

function coursesSourceFiles() {
  const out = [];
  const walk = (d) => {
    if (!fs.existsSync(d)) return;
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, 'pages/courses'));
  return out;
}

const relOf = (abs) => path.relative(ROOT, abs).split(path.sep).join('/');

/** 取某个顶层导出函数的函数体（到下一个行首 `}` 为止，同 pointsRealApiContract 口径） */
function fnBody(src, marker) {
  const start = src.indexOf(marker);
  expect(start).toBeGreaterThan(-1);
  return src.slice(start, src.indexOf('\n}', start));
}

function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

describe('600 行软预算机检（pages/courses/** + 模块域 api 达标后锁定）', () => {
  it('courses 模块全部源文件 ≤600 行（含新建 composable 与 section 组件）', () => {
    const files = coursesSourceFiles().concat(path.join(ROOT, 'api/course.uts'));
    const over = files.map((f) => ({
      file: relOf(f),
      lines: readText(f).split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层', () => {
    const deep = coursesSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/courses'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map(relOf);
    expect(deep).toEqual([]);
  });

  it('模块必需源文件清单完整（删任一文件即红，扫描面不靠数量下限兜底）', () => {
    for (const rel of REQUIRED_SOURCE_FILES) expect(exists(rel)).toBe(true);
    // 扫描面有效性：目录遍历必须真的扫到全部必需文件（防 walker 静默失效）
    const scanned = new Set(coursesSourceFiles().map(relOf));
    for (const rel of REQUIRED_SOURCE_FILES) expect(scanned.has(rel)).toBe(true);
  });
});

describe('页面 ↔ 组件接口对账（本票唯一新增接口面：prop/事件改名即红）', () => {
  it.each(WIRING)('$page ↔ $tag：页面绑定的每个 prop 都在组件 defineProps 内', ({ page, tag, comp }) => {
    const attrs = usageAttrs(read(page), tag);
    expect(attrs).not.toBeNull();
    const declared = definePropsNames(read(comp));
    for (const p of boundProps(attrs)) expect(declared).toContain(p);
  });

  it.each(WIRING)('$page ↔ $tag：组件声明的每个 prop 都被页面绑定（无孤儿 prop）', ({ page, tag, comp }) => {
    const attrs = usageAttrs(read(page), tag);
    const bound = boundProps(attrs);
    for (const p of definePropsNames(read(comp))) expect(bound).toContain(p);
  });

  it.each(WIRING)('$page ↔ $tag：页面监听的每个事件都在组件 defineEmits 内', ({ page, tag, comp }) => {
    const attrs = usageAttrs(read(page), tag);
    const declared = defineEmitsNames(read(comp));
    for (const e of boundEvents(attrs)) expect(declared).toContain(e);
  });

  it.each(WIRING)('$page ↔ $tag：组件声明的每个事件都被页面监听（无孤儿 emit）', ({ page, tag, comp }) => {
    const attrs = usageAttrs(read(page), tag);
    const bound = boundEvents(attrs);
    for (const e of defineEmitsNames(read(comp))) expect(bound).toContain(e);
  });

  it('对账表本身有效（7 个组件、prop 与事件总数非零，防解析器静默失效）', () => {
    let props = 0;
    let emits = 0;
    for (const w of WIRING) {
      props += definePropsNames(read(w.comp)).length;
      emits += defineEmitsNames(read(w.comp)).length;
    }
    expect(props).toBe(26);
    expect(emits).toBe(5);
  });

  it('对账锁具备红能力（注入改名必须被抓到，防「解析器失效即假绿」）', () => {
    const attrs = usageAttrs(read(WIRING[0].page), WIRING[0].tag);
    expect(boundProps(attrs)).toContain('blocks');
    const renamed = attrs.replace(':blocks=', ':blocksRenamed=');
    expect(boundProps(renamed)).not.toContain('blocks');
    expect(definePropsNames(read(WIRING[0].comp))).toContain('blocks');
  });
});

describe('composable 接线契约（T08 拆分：显式 import 模块私有 composable）', () => {
  it('chapter-view.uvue 以显式 import 使用 useChapterStudy composable', () => {
    const src = read('pages/courses/chapter-view.uvue');
    expect(src).toMatch(/import\s*\{\s*useChapterStudy\s*\}\s*from\s*'\.\/composables\/useChapterStudy'/);
    expect(src).toContain('useChapterStudy()');
  });

  it('useChapterStudy 声明显式结果类型（Kotlin error18 规避，守护规则 U 同款口径）', () => {
    const src = read('pages/courses/composables/useChapterStudy.uts');
    expect(src).toMatch(/export\s+function\s+useChapterStudy\s*\(\s*\)\s*:\s*UseChapterStudyResult/);
    expect(src).toContain('as UseChapterStudyResult');
  });

  it('返回对象里的函数字段一律箭头包裹（守护规则 T：禁裸函数引用）', () => {
    const src = read('pages/courses/composables/useChapterStudy.uts');
    const body = fnBody(src, 'return {');
    for (const name of ['beginStudy', 'stopStudy', 'reportIncremental']) {
      expect(body).toMatch(new RegExp(name + '\\s*:\\s*\\('));
    }
  });
});

describe('组件接线零孤儿（#779 回归锁：import 的组件文件必须存在）', () => {
  it('页面 import 的每个模块私有组件文件都真实存在', () => {
    for (const page of COURSES_PAGES) {
      const src = read(page);
      for (const m of src.matchAll(/from\s*'\.\/components\/([^']+)'/g)) {
        expect(exists('pages/courses/components/' + m[1])).toBe(true);
      }
    }
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const dir = path.join(ROOT, 'pages/courses/components');
    // 目录不存在 ⇒ 直接判红（否则「目录没了」会让这条静默通过）
    expect(fs.existsSync(dir)).toBe(true);
    const pagesSrc = COURSES_PAGES.map(read).join('\n');
    for (const name of fs.readdirSync(dir)) {
      if (!/\.uvue$/.test(name)) continue;
      expect(pagesSrc).toMatch(new RegExp("'\\./components/" + name.replace(/\./g, '\\.') + "'"));
    }
  });
});

describe('allowlist 不回潮（courses 域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 courses 域文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/pages[/\\]courses/);
    expect(block).not.toMatch(/api[/\\]course\.uts/);
  });

  it('courses 域源文件零 catch-any / 零 e.detail 直取（规则 H / I 全量执法）', () => {
    const files = coursesSourceFiles().concat([path.join(ROOT, 'api/course.uts')]);
    for (const f of files) {
      const src = readText(f);
      expect(src).not.toMatch(/catch\s*\(\s*\(?\s*[A-Za-z_$][\w$]*\s*:\s*any\b(?!\s*\|)/);
      expect(src).not.toContain('.detail');
    }
  });
});

describe('零直发请求（页面层不直接 uni.request）', () => {
  it.each(COURSES_PAGES)('%s 不直接调用 uni.request', (rel) => {
    expect(read(rel)).not.toContain('uni.request');
  });
});

describe('域 api 收紧（T08 / ADR-0007）：DTO 函数经 mapper-callback 出口', () => {
  const src = read('api/course.uts');

  it('引入 getMapped 出口家族（且不再裸 import get）', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).not.toMatch(/import\s*\{\s*get\s*,\s*getMapped/);
  });

  it.each(DTO_FNS)('%s 经 mapper 出口（getMapped + 箭头包裹 build*）', (name) => {
    const body = fnBody(src, 'export function ' + name);
    expect(body).toContain('getMapped<');
    // 箭头包裹（守护规则 M 禁裸 builder 引用作 mapper）：箭头之后紧邻的就是 build* 调用
    expect(body).toMatch(/=>[\s\S]{0,240}?build[A-Za-z]*\(/);
  });

  it.each(RAW_PASSTHROUGH)('%s 保持 raw 透传（裸 post），未误加 DTO', (name) => {
    const body = fnBody(src, 'export function ' + name);
    expect(body).toContain("post('/course/'");
    expect(body).not.toContain('getMapped');
    expect(body).not.toContain('postMapped');
  });

  it('api 层裸 get 已清零（收紧后不应再有裸 GET 出口）', () => {
    const code = stripComments(src);
    expect(code).not.toMatch(/[^a-zA-Z]get\(/);
  });

  it('api 层静默回退计数与术前一致（本票按删除禁区不退役 mock 回退）', () => {
    // 5 处：courses 列表 / catalog 专业方向 / levels / 课程详情 / 章节详情（getTagsApi 本就无回退）
    expect((src.match(/\.catch\(/g) || []).length).toBe(5);
    // T02 试点锁定的那条回退保持原样（mallPilotContract 亦钉此点）
    expect(fnBody(src, 'export function getCourseListApi')).toContain('getMockCourseList()');
  });
});

describe('幻影路由锁（#662 口径）：api 层路由必须落在后端已注册清单内', () => {
  const apiSrc = stripComments(read('api/course.uts'));

  /** 后端已注册路由的唯一事实源（courses.go + training_catalog.go 的 .GET/.POST 注册行） */
  function backendRoutes() {
    const out = [];
    for (const rel of ['../../backend/internal/api/courses.go', '../../backend/internal/api/training_catalog.go']) {
      for (const m of read(rel).matchAll(/\.(?:GET|POST|PUT|DELETE)\("([^"]+)"/g)) out.push(m[1]);
    }
    return out;
  }

  it('course.uts 的路由字面量非空，且每条都被后端注册清单覆盖', () => {
    // 只取「拼接链的起点」字面量：`'/course/' + id + '/progress'` 这类后续片段不是独立路由
    const routes = [];
    for (const m of apiSrc.matchAll(/'(\/[A-Za-z0-9\-/:]*)'/g)) {
      const before = apiSrc.slice(0, m.index).replace(/\s+$/, '');
      if (before.endsWith('+')) continue;
      routes.push(m[1]);
    }
    expect(routes.length).toBeGreaterThan(0);
    const registered = backendRoutes();
    expect(registered.length).toBeGreaterThan(0);
    const phantom = [];
    for (const lit of routes) {
      const ok = registered.some((r) => {
        const pattern = new RegExp('^' + r.replace(/:[A-Za-z_]+/g, '[^/]+') + '$');
        if (pattern.test(lit)) return true;
        // 拼接形态：api 只写了前缀（如 '/course/'），后端把 id 拼在同一段
        return lit.endsWith('/') && r.replace(/:[A-Za-z_]+/g, '').startsWith(lit);
      });
      if (!ok) phantom.push(lit);
    }
    expect(phantom).toEqual([]);
  });

  it('后端确实注册了本域用到的 7 个端点（防「清单读空 ⇒ 全判幻影」的反向假绿）', () => {
    const registered = backendRoutes();
    for (const r of ['/courses', '/catalog/tree', '/levels', '/tags', '/course/:course_id', '/course/:course_id/chapter/:chapter_id', '/course/:course_id/progress']) {
      expect(registered).toContain(r);
    }
  });
});

describe('删除禁区「courses 不用删」：行为保持点逐项仍在', () => {
  it('章节学习：计时/增量上报走模块私有 composable，心跳与上报间隔逐字保持', () => {
    const src = read('pages/courses/composables/useChapterStudy.uts');
    expect(src).toContain('AUTO_REPORT_INTERVAL = 60');
    expect(src).toContain('setInterval');
    expect(src).toContain('updateCourseProgressApi(reportCourseId, reportChapterId, durationMinutes)');
    expect(src).toContain('if (!isFinal && incremental < 60) return');
  });

  it('章节学习：onHide / onUnload / 切章前都先停表并补报剩余时长', () => {
    const src = read('pages/courses/chapter-view.uvue');
    expect(src).toMatch(/onHide\(\(\) : void => \{\s*\n\s*study\.stopStudy\(\)\s*\n\s*study\.reportIncremental\(true\)/);
    expect(src).toMatch(/onUnload\(\(\) : void => \{\s*\n\s*study\.stopStudy\(\)\s*\n\s*study\.reportIncremental\(true\)/);
    const prev = fnBody(src, 'function onPrevChapter');
    const next = fnBody(src, 'function onNextChapter');
    for (const body of [prev, next]) {
      expect(body).toContain('study.stopStudy()');
      expect(body).toContain('study.reportIncremental(true).then(');
    }
  });

  it('章节学习：进入章节后开始计时（beginStudy 带上 courseId 与当前章节）', () => {
    const src = read('pages/courses/chapter-view.uvue');
    expect(src).toContain('study.beginStudy(courseId.value, data.chapter_id)');
  });

  it('章节学习：图片预览与附件下载/打开仍在页面（组件 emit 回壳层）', () => {
    const src = read('pages/courses/chapter-view.uvue');
    expect(src).toContain('uni.previewImage');
    expect(src).toContain('uni.downloadFile');
    expect(src).toContain('uni.openDocument');
    expect(src).toContain('@image-click="onImageClick"');
    expect(src).toContain('@file-click="onFileClick"');
  });

  it('章节学习：上下章标题计算与返回仍在；缺省档解析仍在页面', () => {
    const src = read('pages/courses/chapter-view.uvue');
    expect(src).toContain('updateNavTitles()');
    expect(src).toContain('goBack()');
    expect(src).toContain('parseMarkdown(detail.value!.content)');
    expect(src).toContain('getChapterDetailApi(courseId.value, chapterId.value)');
  });

  it('章节正文渲染：8 个渲染分支与附件资料区块逐项仍在', () => {
    const md = read('pages/courses/components/chapter-markdown.uvue');
    for (const t of ["'heading'", "'paragraph'", "'code'", "'list'", "'quote'", "'divider'", "'image'", "'table'"]) {
      expect(md).toContain('block.type == ' + t);
    }
    const files = read('pages/courses/components/chapter-file-list.uvue');
    expect(files).toContain('附件资料');
    expect(files).toContain('@click="onFileTap(file)"');
  });

  it('章节导航：四个文案与禁用态判据仍在', () => {
    const nav = read('pages/courses/components/chapter-nav.uvue');
    for (const t of ['上一章节', '下一章节', '没有上一章节', '没有下一章节']) expect(nav).toContain(t);
    expect(nav).toContain(':class="{ disabled: prevChapterId == 0 }"');
    expect(nav).toContain(':class="{ disabled: nextChapterId == 0 }"');
  });

  it('课程详情：收藏三连、学习状态、继续学习章节计算仍在页面', () => {
    const src = read('pages/courses/course-detail.uvue');
    for (const t of ['checkFavoriteApi', 'addFavoriteApi', 'removeFavoriteApi', 'getStudentCourseDetailApi', 'computeContinueChapterId', 'onStartLearn']) {
      expect(src).toContain(t);
    }
    expect(src).toContain("{{ continueChapterId > 0 ? '继续学习' : '开始学习' }}");
  });

  it('章节页：收藏三连接线在（#1140），且 target_type 是 chapter 不是 course', () => {
    const src = read('pages/courses/chapter-view.uvue');
    for (const t of ['checkFavoriteApi', 'addFavoriteApi', 'removeFavoriteApi']) {
      expect(src).toContain(t);
    }
    // 目标类型必须逐字是 'chapter'：从 course-detail 抄接线而忘改 target_type 时，
    // 页面「看起来能用」但收藏的是课程 —— 静态面必须拦下（行为面见
    // utils/chapterFavoriteContract.test.js：一个守接线，一个守行为，两者都要）
    expect(src).toMatch(/checkFavoriteApi\(\s*'chapter'/);
    expect(src).toMatch(/addFavoriteApi\(\s*'chapter'/);
    expect(src).toContain('removeFavoriteApi(favoriteId.value)');
    // 模板仍把点击接线到控件（函数在、但没接 = 同样点不到）
    expect(src).toContain('@click="toggleFavorite"');
  });

  it('课程详情：章节点击跳转的 URL 形态逐字保持', () => {
    const src = read('pages/courses/course-detail.uvue');
    expect(src).toContain("'/pages/courses/chapter-view?course_id=' + courseId.value + '&chapter_id=' + chapterId");
  });

  it('课程详情：封面/进度/信息网格/章节列表四个区块的关键文案逐项仍在组件里', () => {
    expect(read('pages/courses/components/course-cover-section.uvue')).toContain('cover-section');
    const prog = read('pages/courses/components/course-progress-section.uvue');
    expect(prog).toContain('学习进度');
    expect(prog).toContain('已完成 {{ completed }}/{{ totalChapters }} 章');
    const info = read('pages/courses/components/course-info-grid.uvue');
    for (const t of ['理论学时', '实操学时', '章节数', '课程时长', '关联证书', '前置课程']) expect(info).toContain(t);
    const list = read('pages/courses/components/course-chapter-list.uvue');
    for (const t of ['课程内容', '个章节', '暂无章节内容', 'chapter-arrow']) expect(list).toContain(t);
  });

  it('课程详情：加载失败可见 + 重试仍在页面', () => {
    const src = read('pages/courses/course-detail.uvue');
    expect(src).toContain('课程详情加载失败');
    expect(src).toContain('@click="loadDetail"');
  });

  it('模块文件集合不减：三条路由与三个页面均在（不删页面/路由）', () => {
    const pagesJson = read('pages.json');
    for (const p of ['pages/courses/courses', 'pages/courses/course-detail', 'pages/courses/chapter-view']) {
      expect(pagesJson).toMatch(new RegExp('"path"\\s*:\\s*"' + p + '"'));
      expect(exists(p + '.uvue')).toBe(true);
    }
  });
});

describe('展示纯函数唯一实现（T03/T06 口径）：courses 模块零第二实现', () => {
  const files = coursesSourceFiles();

  const declFiles = (name) => files.filter((f) => new RegExp('function\\s+' + name + '\\s*\\(').test(read(relOf(f)))).map(relOf);

  it.each(['getFileIcon', 'getFileIconClass', 'getFileTypeName'])('%s 全模块只有一处实现', (name) => {
    expect(declFiles(name)).toHaveLength(1);
  });

  it.each(['getSpecialtyIcon', 'getSpecialtyCoverColor'])('%s 全模块只有一处实现', (name) => {
    expect(declFiles(name)).toHaveLength(1);
  });

  it('文件图标三件套落在拆分出的附件组件里（跟消费面走）', () => {
    const owner = 'pages/courses/components/chapter-file-list.uvue';
    for (const name of ['getFileIcon', 'getFileIconClass', 'getFileTypeName']) {
      expect(declFiles(name)).toEqual([owner]);
    }
  });

  it('formatDuration 两处是刻意不同语义，各自空值兜底不得被「顺手合并」', () => {
    // 章节侧（分钟→'未知'）与课程详情侧（分钟→'-'）文案不同，合并会改掉其中一页的显示
    expect(declFiles('formatDuration').sort()).toEqual([
      'pages/courses/chapter-view.uvue',
      'pages/courses/course-detail.uvue',
    ]);
    expect(read('pages/courses/chapter-view.uvue')).toContain("if (minutes <= 0) return '未知'");
    expect(read('pages/courses/course-detail.uvue')).toContain("if (minutes <= 0) return '-'");
  });

  it('封面图标/底色的映射表仍只有页面那一份（组件只吃扁平 props）', () => {
    const comp = read('pages/courses/components/course-cover-section.uvue');
    expect(comp).not.toContain('getSpecialtyCoverColor');
    expect(comp).not.toContain('getSpecialtyIcon');
    expect(comp).toContain('coverColor');
    expect(comp).toContain('specialtyIcon');
  });
});

describe('页面侧派生状态（T05「同一事实不两处持有」）：封面与时长由 detail 派生', () => {
  it('coverColor 不再是可写的 ref，而是 computed（消除一处失效风险）', () => {
    const src = read('pages/courses/course-detail.uvue');
    expect(src).not.toContain("const coverColor = ref<string>");
    expect(src).not.toContain('coverColor.value =');
    expect(src).toMatch(/const coverColor = computed<string>\(/);
    expect(src).toMatch(/const specialtyIcon = computed<string>\(/);
    expect(src).toMatch(/const durationText = computed<string>\(/);
  });

  it('computed 的空值兜底：底色 #e3f2fd / 图标 📚（真字形）/ 时长 -', () => {
    const src = read('pages/courses/course-detail.uvue');
    expect(src).toContain("if (detail.value == null) return '#e3f2fd'");
    expect(src).toContain("if (detail.value == null) return '📚'");
    expect(src).toContain("if (detail.value == null) return '-'");
  });
});

/**
 * #1087 契约：封面分类语义由**真实字段**驱动，且退化不了。
 *
 * 票面成因是「函数吃 category（后端已退役 ⇒ 恒空）⇒ 永远走默认分支」，所以这里**断言产物**：
 * 每个 specialty_id 各自解析出**互不相同**的真字形与真底色。旧实现下这 4 个断言会同时红
 * （4 个 id 全落在同一个默认分支上）—— 这正是本票要防的回归形态。
 *
 * 边界如实声明：本文件是静态契约测试（node 环境，不跑 uvue 运行时），断言的是映射表的**取值为真**，
 * 不是「真机上渲染出像素」。渲染面由 ①a 真机逐页截图取证。
 */
describe('#1087 封面分类语义由 specialty_id 驱动（真实字段，非退役 category）', () => {
  const PAGE = 'pages/courses/course-detail.uvue';
  const src = read(PAGE);

  /**
   * 取 `<marker>` 之后的第一个函数体（首个独立成行的 `}` 即为收尾）。
   * 不复用下层 fnBody 的原因如实记：该 helper 靠 `indexOf('\n}', …)` 判尾，
   * 只在「函数尾 `}` 后紧跟空行」的写法下正确；本页 `const x = computed<string>(() : string => {`
   * 的收尾是 `\n    })`（无空行），会被截短。
   */
  const sliceBody = (marker) => {
    const start = src.indexOf(marker);
    // 找不到标记（函数被删/改名）时返回空串，让**测试逐条判红**并点名，
    // 而不是在 describe 体里抛异常把整个 suite 变成 "failed to run"（那样红得没信息）。
    if (start < 0) return '';
    const end = src.indexOf('\n    }', start);
    if (end < 0) return '';
    return src.slice(start, end);
  };

  const iconBody = sliceBody('function getSpecialtyIcon');
  const colorBody = sliceBody('function getSpecialtyCoverColor');

  /** 解析 `if (<ident> == N) return 'X'` 形式的常量映射表 */
  const parseMap = (body, ident) => {
    const out = new Map();
    for (const m of body.matchAll(new RegExp('if\\s*\\(\\s*' + ident + '\\s*==\\s*(\\d+)\\s*\\)\\s*return\\s*\'([^\']*)\'', 'g'))) {
      out.set(Number(m[1]), m[2]);
    }
    return out;
  };

  const iconMap = parseMap(iconBody, 'specialtyId');
  const colorMap = parseMap(colorBody, 'specialtyId');

  it('四个专业方向（1 操作 / 2 维修 / 3 安全 / 4 电池）都有显式分支', () => {
    for (const id of [1, 2, 3, 4]) {
      expect(iconMap.has(id)).toBe(true);
      expect(colorMap.has(id)).toBe(true);
    }
  });

  it('每个方向解出的都是**真字形**：非空串、非孤立 U+FE0F、非 ASCII 占位', () => {
    for (const id of [1, 2, 3, 4]) {
      const glyph = iconMap.get(id);
      expect(glyph.length).toBeGreaterThan(0);
      // 不得只剩变体选择符（#1071 的第一族坏形态：孤立 U+FE0F）
      expect(glyph.replace(/[\uFE0E\uFE0F]/g, '')).not.toBe('');
      // 图标必须是非 ASCII 的真实字形（空串 / 纯 ASCII 占位属坏形态）
      expect(/[^\x00-\x7F]/.test(glyph)).toBe(true);
      expect(glyph).not.toContain('\uFFFD');
      // 码位必须落在图标区（emoji / dingbat），挡掉"看着像图标其实是别的东西"
      const cps = Array.from(glyph).map((c) => c.codePointAt(0));
      expect(cps.some((cp) => (cp >= 0x2190 && cp <= 0x2BFF) || (cp >= 0x1F000 && cp <= 0x1FAFF))).toBe(true);
    }
  });

  it('图标两两互不相同（旧实现下 4 个 id 恒等 ⇒ 此断言即本票的回归锁）', () => {
    const glyphs = [1, 2, 3, 4].map((id) => iconMap.get(id));
    expect(new Set(glyphs).size).toBe(4);
  });

  it('底色两两互不相同且都是合法十六进制色值', () => {
    const colors = [1, 2, 3, 4].map((id) => colorMap.get(id));
    expect(new Set(colors).size).toBe(4);
    for (const c of colors) expect(c).toMatch(/^#[0-9a-fA-F]{6}$/);
  });

  it('未知 / 未设置方向有兜底，且兜底不返回空字形', () => {
    // 末行兜底：`return '<非空字形>';`
    const iconFallback = [...iconBody.matchAll(/return\s+'([^']*)'/g)].pop();
    expect(iconFallback).toBeDefined();
    expect(iconFallback[1].length).toBeGreaterThan(0);
    expect(/[^\x00-\x7F]/.test(iconFallback[1])).toBe(true);
    // 底色兜底必须是合法十六进制色值
    expect(colorBody).toMatch(/return\s*'#[0-9a-fA-F]{6}'\s*;?\s*$/);
  });

  it('页面模板把图标与底色接到 specialty 派生的 computed 上（不是退役 category）', () => {
    expect(src).toContain(':specialty-icon="specialtyIcon"');
    expect(src).toContain(':cover-color="coverColor"');
    // 接线不得回退到退役字段
    expect(src).not.toContain('detail.value!.category');
    expect(src).not.toContain('detail!.category');
    expect(src).not.toContain('data.category');
  });

  it('旧 10 类枚举（theory/safety/practice/advanced/structure/hydraulic/driving/maintenance/cargo/troubleshooting）在本页彻底清零', () => {
    const stale = [
      'CATEGORY_01', 'CATEGORY_02', 'CATEGORY_03', 'CATEGORY_04',
      "'theory'", "'safety'", "'practice'", "'advanced'", "'structure'",
      "'hydraulic'", "'driving'", "'maintenance'", "'cargo'", "'troubleshooting'",
    ];
    for (const token of stale) expect(src).not.toContain(token);
  });
});

/**
 * #1087 Q3 裁定：退役残留的 category 字段面。
 * 后端 CourseDTO 无 category（backend/internal/service/course_service.go 的 category 属 CredentialBriefDTO）
 * ⇒ 前端映射吃空值。经全仓核对**无第二消费面**后退役；本组断言防它被重新引入。
 */
describe('#1087 Q3：课程域 category 残留面已退役', () => {
  it('types/course.uts 无 category 字段、无 CourseCategory 类型', () => {
    const src = read('types/course.uts');
    expect(src).not.toMatch(/^\s*category\s*:/m);
    expect(src).not.toContain('CourseCategory');
  });

  it('types/index.uts 不再导出 CourseCategory', () => {
    expect(read('types/index.uts')).not.toContain('CourseCategory');
  });

  it('api/course.uts 不再读 / 写 category（含 mock 数据）', () => {
    const src = read('api/course.uts');
    expect(src).not.toContain("obj['category']");
    expect(src).not.toContain("infoObj['category']");
    expect(src).not.toMatch(/\bcategory\s*:/);
  });

  it('courses 模块页面与组件不再读 course.category（属性访问，非 CSS 类名 / 非注释）', () => {
    // 只锁**可执行代码**里的属性访问形态：
    // ① CSS 类名 `.category-row` 是本页合法的视觉元素，不是数据消费面；
    // ② 说明退役原因的注释里会出现 `course.category` 字样，属解释、非消费。
    //    注意：本仓既有的 stripComments 只吃 JS 注释，.uvue 的模板注释是 HTML 形态（<!-- -->），
    //    故此处自带一个三形态都吃的剥离器（已有实测踩中：模板注释漏网导致假红）。
    const stripAll = (s) =>
      s
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:])\/\/[^\n]*/g, '$1');
    const consumers = ['course.category', 'detail.category', 'data.category', 'item.category'];
    for (const f of coursesSourceFiles()) {
      const code = stripAll(read(relOf(f)));
      for (const c of consumers) expect(code).not.toContain(c);
    }
  });
});
