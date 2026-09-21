/**
 * 导航 query 键契约守护（#1071 真机复核牵出）—— 源码契约测试缝。
 *
 * 为什么需要它：`pages/profile/records.uvue` 跳课程详情时传的是 `?course_id=<n>`，
 * 而 `pages/courses/course-detail.uvue` 的 `onLoad` 读的是 **`options['id']`**
 * （实测每个历史版本都读 `id`）⇒ `id` 恒缺失、页面走空态，用户表现为
 * 「学习记录里已完成的课程点不开」。同类问题还有一处（见下「已知例外」）。
 *
 * 这类缺陷的特征：**两端各自都合法、编译过、jest 绿、只有把它们对起来才不成立**
 * —— 静态契约测试正是能把它钉住的那道缝。
 *
 * 判据（只判「传了但目标页从不读」的键）：
 *   对每个 `navigateTo/redirectTo/reLaunch/switchTab` 的**字面量** URL，
 *   取其 query 的键集合，与**目标页 onLoad 实际读取的键集合**求差；
 *   差集非空 ⇒ 该键无效（页面拿不到它，行为静默退化）。
 *
 * 为什么只判这一个方向（传了没读），不判「该读但没传」：后者需要知道每页的**必填**参数，
 * 而仓里没有这份声明（`onLoad` 普遍写成 `if (x != null)` 兜底）—— 硬判会把合法调用判红。
 *
 * ⚠️ 已知边界（如实声明，勿把本守护当全量）：
 *   1. 只判**字面量** URL（含 `` `...?${x}` `` 的静态键名）；URL 由变量拼接的跳过 ——
 *      静态分析到此为止，那类需靠人工/真机兜（本仓现有 URL 均为字面量，实测覆盖 100%）。
 *   2. 页面把 `options` 整体**委托**给 composable 时（先例 exam：`session.applyOptions(options)`），
 *      键要从被调方解析。本守护按**函数名**解析这类委托（见 `collectOptionReaderKeys`）；
 *      若日后出现「转手两次」的委托链则解析不到，会**漏报**（不会误报）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const relOf = (p) => path.relative(ROOT, p).split(path.sep).join('/');

/** 递归收集源码文件（跳过构建产物与依赖） */
function collectSource(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (['node_modules', 'unpackage', '.git', '.ci-verify'].includes(e.name)) continue;
      collectSource(full, acc);
    } else if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) {
      acc.push(full);
    }
  }
  return acc;
}

/**
 * 从 `startIdx` 起截出 `onLoad(...)` 这次调用的完整文本（含实参与其后的函数体）。
 *
 * 实现：先按**圆括号**配平找到 `(` 的匹配 `)`（这解决 `onLoad(function () {…})` 与
 * `onLoad((options) => {…})` 两种形态的外层括号）；若紧跟着 `=>`，说明是箭头函数，
 * 再从 `=>` 起按**花括号**配平补上函数体。
 *
 * ⚠️ 为什么不用「找下一个行首 `}`」那种启发式：本仓缩进**空格与制表符混用**，
 * 实测 `pages/ai-assistant/ai-feature.uvue` 用 tab ⇒ 按 `\n    }`（四空格）切**永远找不到**，
 * 函数体会一路延伸到文件结尾（静默把整个 `<style>` 当成函数体）。
 * 配平则与缩进风格、行尾（本仓有 CRLF 文件）完全无关。
 */
function balancedCallBody(src, startIdx) {
  const open = src.indexOf('(', startIdx);
  if (open === -1) return '';
  let depth = 0;
  let end = -1;
  for (let i = open; i < src.length; i++) {
    const ch = src[i];
    if (ch === '(') depth++;
    else if (ch === ')') {
      depth--;
      if (depth === 0) { end = i; break; }
    }
  }
  if (end === -1) return src.slice(open + 1);
  let text = src.slice(open + 1, end);
  // 箭头函数体：`) => { … }` —— 从 `=>` 后按花括号配平补上
  const after = src.slice(end + 1);
  const arrow = /^\s*=>\s*/.exec(after);
  if (arrow !== null) {
    const braceStart = end + 1 + arrow[0].length;
    if (src[braceStart] === '{') {
      let d = 0;
      for (let i = braceStart; i < src.length; i++) {
        if (src[i] === '{') d++;
        else if (src[i] === '}') {
          d--;
          if (d === 0) { text += src.slice(braceStart, i + 1); break; }
        }
      }
    }
  }
  return text;
}

/** 取「调用表达式」的函数体（自 `startIdx` 处的调用起，按括号配平） */
function fnBody(src, startIdx) {
  return balancedCallBody(src, startIdx);
}

/**
 * 取「函数声明」的函数体 —— `function name(params) : RetType { … }`。
 *
 * ⚠️ 为什么不能用 `fnBody`（调用表达式版）：调用版从第一个 `(` 开始配平，对
 * `function applyOptions(options : UTSJSONObject) : void {` 会**把参数表当成函数体**
 * 返回（`"options : UTSJSONObject"`）⇒ 括号键恒为空 ⇒ 委托解析表全空
 * （实测：`collectOptionReaderKeys` 返回 0 个函数，导致 exam 三处委托被误报成无效键）。
 * 函数声明要：配平参数表 → 跳过返回类型 → 从 `{` 起按花括号配平。
 */
function fnDeclBody(src, startIdx) {
  const open = src.indexOf('(', startIdx);
  if (open === -1) return '';
  let depth = 0;
  let parenEnd = -1;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '(') depth++;
    else if (src[i] === ')') {
      depth--;
      if (depth === 0) { parenEnd = i; break; }
    }
  }
  if (parenEnd === -1) return '';
  // 跳过返回类型标注（`: void` / `: Promise<void>` 等），找到函数体的 `{`
  const brace = src.indexOf('{', parenEnd);
  if (brace === -1) return '';
  // 防御：`{` 与 `)` 之间若出现 `;`，说明不是函数定义（不应发生）
  if (src.slice(parenEnd, brace).includes(';')) return '';
  let d = 0;
  for (let i = brace; i < src.length; i++) {
    if (src[i] === '{') d++;
    else if (src[i] === '}') {
      d--;
      if (d === 0) return src.slice(brace + 1, i);
    }
  }
  return src.slice(brace + 1);
}

/**
 * `obj['x']` 形式的字符串键访问 —— 用于取 query 参数键。
 *
 * ⚠️ 为什么**不按变量名**（`options['x']`）匹配：实测本仓至少四种写法，按名字切会大面积漏
 * —— `options['x']`、`_options` + `const opts = _options as UTSJSONObject` 别名后 `opts['id']`
 * （先例 featured-detail / job-detail）、`onLoad((options : OnLoadOptions) => …)`（先例 ai-feature）、
 * 以及委托 `session.applyOptions(options)`。而 `onLoad` 体内的 `['字符串']` 访问，
 * 其键**就是** query 参数键（页面入参只从 query 来），与变量怎么命名无关 ⇒ 按「括号字符串键」收。
 *
 * 已知代价（如实声明）：若页面 script 里**别处**也用 `['某键']` 且该键恰好与某次导航传的键同名，
 * 本守护会把它当作「页面读得到」而**放过**（漏报）。方向是**安全**的（不会把合法调用判红），
 * 且实测本仓不存在这种遮蔽（否则 ④ 会漏掉 favorites 那处）。
 */
function bracketKeysIn(text) {
  const keys = new Set();
  for (const m of text.matchAll(/\[\s*'([^']+)'\s*\]/g)) keys.add(m[1]);
  return keys;
}

/**
 * 全仓「读取 options 的函数名 → 其读取的键集合」。
 * 用于解析页面把 options 委托给 composable 的情形（先例 exam 的 applyOptions）。
 * 键冲突时**并集**（保守：宁可多认几个键而不误报）。
 */
function collectOptionReaderKeys(files) {
  const byFn = new Map();
  for (const f of files) {
    const src = readText(f);
    const re = /function\s+([A-Za-z_$][\w$]*)\s*\(\s*options\s*(?::\s*[^)]*)?\)/g;
    let m;
    while ((m = re.exec(src)) !== null) {
      const keys = bracketKeysIn(fnDeclBody(src, m.index));
      if (keys.size === 0) continue;
      const name = m[1];
      const prev = byFn.get(name) || new Set();
      keys.forEach((k) => prev.add(k));
      byFn.set(name, prev);
    }
  }
  return byFn;
}

/**
 * 页面 → 该页 onLoad 实际会读到的 query 键集合。
 * 覆盖：`onLoad` 参数体内的括号字符串键 + `foo(options)` / `x.foo(options)` 形式的委托。
 */
function pageOptionKeys(src, readerKeys) {
  const keys = new Set();
  // ⚠️ 两个都踩过的坑，缺一就出「只读 []」的假阳性：
  //   ① 必须锚 `onLoad(` 的**调用**（`onLoad((…) =>` / `onLoad(function (…)`），
  //      不能只写 `onLoad` —— 否则命中 `import { onLoad } from '@dcloudio/uni-app'`；
  //   ② **绝不能**用 `src.replace(...)` 去掉 import 来消除 ① —— replace 会**移动字符位置**，
  //      而下面 `fnBody(src, onLoad.index)` 用的是**原串**的偏移 ⇒ index 错位到别处，
  //      截出的 body 是空的（实测把 3 处真缺陷误报成 19 处）。
  //      正确做法：直接在**原串**上用「后面必须跟 `(` 或 `function (`」这一个正则区分调用与导入。
  const onLoad = /onLoad\s*\(\s*(?:\(|function\s*\()/.exec(src);
  if (onLoad === null) return keys;
  const body = fnBody(src, onLoad.index);
  bracketKeysIn(body).forEach((k) => keys.add(k));
  // 委托：`xxx(options)` / `x.yyy(options)`
  for (const m of body.matchAll(/([A-Za-z_$][\w$.]*)\s*\(\s*options\s*\)/g)) {
    const callee = m[1].split('.').pop();
    const delegated = readerKeys.get(callee);
    if (delegated) delegated.forEach((k) => keys.add(k));
  }
  return keys;
}

/** pages.json → 路由 path → 页面源码 */
function routeToSource(files) {
  const pagesJson = JSON.parse(readText(path.join(ROOT, 'pages.json')));
  const byRoute = new Map();
  const index = new Map(files.map((f) => [relOf(f), f]));
  const all = [];
  for (const p of pagesJson.pages || []) all.push(p.path);
  for (const sp of pagesJson.subPackages || []) {
    for (const p of sp.pages || []) all.push(sp.root + '/' + p.path);
  }
  for (const route of all) {
    const src = index.get(route + '.uvue');
    if (src) byRoute.set(route, src);
  }
  return byRoute;
}

/** 所有导航调用里的字面量 URL（含静态键名） */
function navigations(files) {
  const out = [];
  const re = /(?:navigateTo|redirectTo|reLaunch|switchTab)\s*\(\s*\{[^}]*?url\s*:\s*(`[^`]*`|'[^']*'|"[^"]*")/g;
  for (const f of files) {
    const src = readText(f);
    let m;
    while ((m = re.exec(src)) !== null) {
      const raw = m[1].slice(1, -1);
      const q = raw.indexOf('?');
      if (q === -1) continue;
      const route = raw.slice(0, q).replace(/^\/+/, '');
      if (/\$\{/.test(route)) continue; // 路径本身动态 ⇒ 无法静态定目标页
      const keys = [];
      for (const kv of raw.slice(q + 1).split('&')) {
        const eq = kv.indexOf('=');
        if (eq === -1) continue;
        const k = kv.slice(0, eq).trim();
        if (!k || /\$\{/.test(k)) continue;
        keys.push(k);
      }
      if (keys.length === 0) continue;
      out.push({ file: relOf(f), route, keys, url: raw, line: src.slice(0, m.index).split('\n').length });
    }
  }
  return out;
}

/**
 * 已知例外（expand 侧；清掉一条必须附带「已修」的证据，禁静默删除）。
 * 本仓既有纪律：存量违例走 allowlist，各票清自己范围，epic 收尾删机制（ADR-0007「静态守护与契约测试纪律」）。
 *
 * **现为空表（2026-09-18，#1089 PR-B 摘除唯一一条）**：原例外是
 * `pages/profile/favorites.uvue` → `pages/courses/chapter-view` 传 camelCase `chapterId`。
 * 摘除的「已修」证据（同时满足本守护的两面）：
 *   ① 键名面：该处已改为 `?course_id=&chapter_id=`（与 ADR-0014 及
 *      `pages/courses/course-detail.uvue` 的 `onChapterClick` / `onStartLearn` 同式），
 *      本文件用例 ④ 在**无例外**时全仓全绿；
 *   ② 数据面：`course_id` 已由后端随条目下发（PR #1133 生产核验）并经
 *      `types/favorite.uts` + `api/favorite.uts` 的 `buildFavoriteItem` 落到
 *      `FavoriteItem.course_id`（走 `toNumber`，缺省 ⇒ 0）—— 即「改键名不足以修复」
 *      这个当时写进 `why` 的前提已经不再成立。
 * 摘除后用例 ⑥ 的循环体不再执行（空表天然通过）；新落点契约见
 * `utils/favoritesLandingContract.test.js`（补的正是本守护抓不到的「压根没分支」那一面）。
 */
/**
 * 本文件**自有**的导航 query 键豁免表（与「守护规则豁免」`GUARD_ALLOWLIST` 无关，别混淆）。
 * 2026-09-20 原名也叫 `GUARD_ALLOWLIST` —— 与 `utils/guardAllowlist.js` 的同名常量撞名，
 * 让「全仓 `GUARD_ALLOWLIST` 只有一个声明点」这条判据（ADR-0023 ⑧）变成假命题；改名区分。
 * 摘除上一条后本表为空，用例 ⑥ 的循环体不再执行（空表天然通过）。
 */
const NAV_QUERY_ALLOWLIST = [];

/** 纯函数：给定页面键表与导航清单 → 违规清单 */
function findUnreadQueryKeys(byRoute, readerKeys, navs) {
  const problems = [];
  const unresolved = [];
  for (const n of navs) {
    const src = byRoute.get(n.route);
    if (!src) { unresolved.push(n); continue; }
    const declared = pageOptionKeys(readText(src), readerKeys);
    const bad = n.keys.filter((k) => !declared.has(k));
    if (bad.length > 0) {
      const allowed = NAV_QUERY_ALLOWLIST.some((a) => a.file === n.file && a.route === n.route);
      if (!allowed) problems.push({ ...n, declared: [...declared], bad });
    }
  }
  return { problems, unresolved };
}

const FILES = collectSource(ROOT);
const READER_KEYS = collectOptionReaderKeys(FILES);
const BY_ROUTE = routeToSource(FILES);
const NAVS = navigations(FILES);

describe('导航 query 键契约（传了但目标页不读 ⇒ 静默失效）', () => {
  it('① 扫描面自检：路由表、导航清单、页面取参都必须真的抓到了（防空跑假绿）', () => {
    expect(BY_ROUTE.size).toBeGreaterThan(30);   // pages.json 的页面基本都能映射到源文件
    expect(NAVS.length).toBeGreaterThan(20);     // 带 query 的导航确实存在
    // 本票两处承载页必须都在扫描面内，且键集合非空
    for (const r of ['pages/courses/course-detail', 'pages/courses/chapter-view']) {
      const src = BY_ROUTE.get(r);
      expect(src).toBeDefined();
      expect(pageOptionKeys(readText(src), READER_KEYS).size).toBeGreaterThan(0);
    }
  });

  it('② 检测器自检：注入「传了但目标页不读」的键必须被判红（用真实页面源码驱动纯函数）', () => {
    // 用**真实** course-detail 页面当目标页（它只读 id），注入一条传 course_id 的导航 ⇒ 必须判红。
    // 这同时证明了本票那处真缺陷的形态能被抓住。
    const byRoute = new Map([['pages/courses/course-detail', BY_ROUTE.get('pages/courses/course-detail')]]);
    const injected = [{
      file: 'pages/somewhere.uvue', line: 1,
      route: 'pages/courses/course-detail',
      keys: ['course_id'], url: '/pages/courses/course-detail?course_id=8',
    }];
    const red = findUnreadQueryKeys(byRoute, READER_KEYS, injected);
    expect(red.problems).toHaveLength(1);
    expect(red.problems[0].bad).toEqual(['course_id']);

    // 反向自检：把键换成页面真正读的 id ⇒ 必须判绿（防「恒红」的假守护）
    const green = findUnreadQueryKeys(byRoute, READER_KEYS,
      [{ ...injected[0], keys: ['id'], url: '/pages/courses/course-detail?id=8' }]);
    expect(green.problems).toEqual([]);
  });

  it('③ 页面 onLoad 取参解析：直接取参与「委托给 composable」两种形态都要能解析', () => {
    // 直接取参：course-detail 读 id
    expect([...pageOptionKeys(readText(BY_ROUTE.get('pages/courses/course-detail')), READER_KEYS)])
      .toContain('id');
    // 委托取参：exam 的 session.applyOptions(options) ⇒ mock_exam_id 必须被解析出来
    const exam = BY_ROUTE.get('pages/exam/mock-exam-result');
    if (exam) {
      expect([...pageOptionKeys(readText(exam), READER_KEYS)])
        .toContain('mock_exam_id');
    }
  });

  it('④ 全仓导航不传「目标页从不读」的 query 键', () => {
    const { problems } = findUnreadQueryKeys(BY_ROUTE, READER_KEYS, NAVS);
    const report = problems
      .map((p) => p.file + ':' + p.line + '  ' + p.url +
        '\n        传了 [' + p.keys.join(', ') + ']，目标页 ' + p.route +
        ' 只读 [' + p.declared.join(', ') + '] ⇒ 无效键: ' + p.bad.join(', '))
      .join('\n');
    expect(report).toBe('');
  });

  it('⑤ 目标页缺失的导航（人工复核面）：必须在报告里可见，不得静默跳过', () => {
    const { unresolved } = findUnreadQueryKeys(BY_ROUTE, READER_KEYS, NAVS);
    // 允许为空（当前实测为 0），若有则打印出来便于人工核对
    if (unresolved.length > 0) {
      console.log('[导航守护] 未能定位目标页的导航：\n' +
        unresolved.map((u) => '  ' + u.file + ':' + u.line + ' ' + u.url).join('\n'));
    }
    expect(Array.isArray(unresolved)).toBe(true);
  });

  it('⑥ 例外表必须是「活」的（防 allowlist 掩盖一条已失效的守护）', () => {
    // 每个例外都必须**真的**在命中：去掉 allowlist 后该导航会被判红。
    // 否则（例如 favorites 那处被修好了）这条例外就是死代码，必须删掉。
    for (const a of NAV_QUERY_ALLOWLIST) {
      const navs = NAVS.filter((n) => n.file === a.file && n.route === a.route);
      expect(navs.length).toBeGreaterThan(0); // 例外指向的导航仍存在
      const stillBad = navs.some((n) => {
        const src = BY_ROUTE.get(n.route);
        if (!src) return false;
        const declared = pageOptionKeys(readText(src), READER_KEYS);
        return n.keys.some((k) => !declared.has(k));
      });
      expect(stillBad).toBe(true); // 例外仍然有存在的理由
    }
  });

  it('⑦ 红能力回归：把 records 那处改回错键 `course_id` ⇒ 必须被判红（本票修复的锁）', () => {
    // 直接喂纯函数，不依赖磁盘：目标页用真实的 course-detail（只读 id）
    const byRoute = new Map([['pages/courses/course-detail', BY_ROUTE.get('pages/courses/course-detail')]]);
    const bug = [{
      file: 'pages/profile/records.uvue', line: 229,
      route: 'pages/courses/course-detail',
      keys: ['course_id'], url: '/pages/courses/course-detail?course_id=8',
    }];
    const { problems } = findUnreadQueryKeys(byRoute, READER_KEYS, bug);
    expect(problems).toHaveLength(1);
    expect(problems[0].bad).toEqual(['course_id']);
  });
});
