/**
 * 「我的收藏」落点契约（#1089 PR-B / #1237）—— 断言**行为**，不断言源码片段。
 *
 * ## 为什么需要这条守护
 *
 * 症状（维护者真机复核 #1071 牵出）：`pages/profile/favorites.uvue` 里收藏的**章节点不开** ——
 * 点进去是空页；同时筛选 chip 里的「题目」「资讯」两类的条目**点了毫无反应**。
 *
 * 根因是**两张表各自合法、只有对起来才不成立**：
 *   ① 落点键错 —— 该处传 camelCase `?chapterId=`，而 `chapter-view` 读 `options['course_id']`
 *      + `options['chapter_id']`（ADR-0014 明载的落点键）；`chapterId` 是全仓唯一的 camelCase query 键。
 *   ② 数据缺失 —— 即使键名改对也打不开：`chapter-view` 需要**两个键都在**，而收藏条目的
 *      `course_id` 当年根本不随响应下发（已由 PR #1133 在生产补齐，本票消费它）。
 *   ③ 分支缺失 —— 落点表只有 course / topic / chapter 三个分支，而筛选 chip 有六项
 *      （「资讯」= `featured`、「题目」= `question`）⇒ 这两类**压根没有分支**，静默 no-op。
 *
 * 既有守护 `navQueryKeyContract.test.js` 只能抓「**传了**目标页不读的键」（方向①），
 * 抓不到方向③「压根没传、没有分支」。
 *
 * ## 断言方式（#1237 起的口径）
 *
 * 落点表已搬到 `utils/favoriteLanding.uts`（唯一实现），本守护经 `utils/utsHarness.js` 的
 * **`loadUts` 真跑那个模块**、断言它的对外决策（`url` / `notice`）—— 不再是自己取出页面函数体
 * 拿 `new Function` 求值的自搓执行器。
 *
 * 为什么必须搬（#1237）：`docs/agents/guards.md` 的分类器判据只看**代码级的执行调用**
 * （`child_process` / `execFileSync` / `spawnSync` / **`loadUts(`** / 动态 `import()`），且明确防
 * 「只 `require` 模块名」的绕法；而 `loadUts` 只吃 `.uts` 模块 —— 落点表住在 `.uvue` 里时
 * **永远进不了那条判据**，本文件只能算**接线守护**（不构成 ③ 门承重证据）。
 *
 * ## 回答 `guards.md` 末节三问（本文件）
 *
 * 1. **行为**守护（`node scripts/classify-guards.mjs` 判据：`loadUts(` + 引用仓内载体 `.uts`）。
 * 2. 成对断言：**必不红** = 本套件在未改动树上全绿；**必红** = 把 `utils/favoriteLanding.uts` 的
 *    chapter 分支改坏（如去掉 `course_id` 守卫、或回到 camelCase 键）⇒ 本套件判红。
 *    该成对取证按 #1237 的要求逐次执行、每轮之间**逐字节还原**（收据在 PR 正文）。
 * 3. 页面侧只剩**接线**：`favorites.uvue` 调用 `favoriteLanding(item)` 并把 `notice` → `showToast`、
 *    `url` → `navigateTo`；这条接线由本文件的「唯一口径 + 应用决策」组守着（行为由上面那条兜底）。
 *
 * ⚠️ 覆盖边界（2026-09-18 #1159 改判后）：本守护覆盖**全仓唯一**的收藏落点表 —— 原先并存的第二处
 *   （`pages/profile/personal-activity.uvue` 的 `onFavoriteClick`）已随「收藏的唯一列表承载面」裁定
 *   （根仓库 `ADR-0018` 补遗）**整体摘除**。
 *
 *   ⇒ **新增 `target_type` 时必须改 `utils/favoriteLanding.uts` 与本文件**：这里是全仓唯一会因
 *   「缺分支」判红的地方。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）；跑模块一律经共享执行器（#1237） */
const { readText, loadUts } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE = 'pages/profile/favorites.uvue';
/** 落点表的唯一实现（本守护的被测物） */
const MODULE = 'utils/favoriteLanding.uts';
const read = (rel) => readText(path.join(ROOT, rel));

/**
 * 取 `marker` 之后那个函数的**函数体**（花括号配平，与缩进风格 / 行尾无关）。
 * 配平而非「找下一个行首 `}`」：本仓空格与制表符混用（先例 #1111 契约测试记的三个坑）。
 * 解析失败返回 `''` —— 调用方的正向用例随即判红，不静默放过。
 *
 * ⚠️ 仅**坐标映射那一组**（`buildFavoriteItem` 未 `export`，`loadUts` 只回读导出名）仍用它。
 */
function fnBodyOf(src, marker) {
  const start = src.indexOf(marker);
  if (start < 0) return '';
  const open = src.indexOf('{', start);
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    const ch = src[i];
    if (ch === '{') depth++;
    else if (ch === '}') {
      depth--;
      if (depth === 0) return src.slice(open + 1, i);
    }
  }
  return '';
}

/** 剥掉 UTS 的标量类型标注（同样只服务坐标映射那一组的取体执行） */
function stripUtsTypes(body) {
  return body.replace(/:\s*(string|number|boolean|UTSJSONObject|FavoriteItem)\b/g, '');
}

/** 真跑共享落点模块，返回对外决策（每次调用都是**全新模块实例**，互不串状态） */
function landing(item) {
  const mod = loadUts(path.join(ROOT, MODULE), {});
  return mod.favoriteLanding(item);
}

/** 便捷：把决策投影成断言面 `{ navigations, toasts }`（与 #1089 的断言逐条同形） */
function click(item) {
  const d = landing(item);
  return {
    navigations: d.url.length > 0 ? [d.url] : [],
    toasts: d.notice.length > 0 ? [d.notice] : [],
  };
}

/** 收藏条目夹具（`course_id` 是 0 哨兵：仅 chapter 有意义，其余类型后端恒给 0） */
const fav = (targetType, targetId, courseId = 0) => ({
  favorite_id: 1,
  target_type: targetType,
  target_id: targetId,
  course_id: courseId,
  title: 't',
  cover: '',
  created_at: '',
});

describe('执行器自检：落点模块真的被 loadUts 跑起来了（防空跑假绿）', () => {
  it('模块导出 `favoriteLanding` 且返回决策对象', () => {
    const d = landing(fav('course', 7));
    expect(typeof d).toBe('object');
    expect(Object.prototype.hasOwnProperty.call(d, 'url')).toBe(true);
    expect(Object.prototype.hasOwnProperty.call(d, 'notice')).toBe(true);
    expect(d.url).toBe('/pages/courses/course-detail?id=7');
  });

  it('落点路径只出现在被测模块里（页面已无内联落点表）', () => {
    const page = read(PAGE);
    for (const seg of ['/pages/courses/course-detail', '/pages/courses/chapter-view',
      '/pages/forum/forum-detail', '/pages/featured/featured-detail', '/pages/practice/practice-do']) {
      expect(read(MODULE)).toContain(seg);
      expect(page).not.toContain(seg);
    }
  });
});

describe('#1089 我的收藏落点：五类条目各有正确落点', () => {
  it('chapter：带上 course_id 与 chapter_id 两键（与 course-detail / searchDisplay 同式）', () => {
    const c = click(fav('chapter', 31, 7));
    expect(c.navigations).toEqual(['/pages/courses/chapter-view?course_id=7&chapter_id=31']);
  });

  it('question：落到做题页（复用 searchDisplay 的既有路径，不新造参数）', () => {
    const c = click(fav('question', 99));
    expect(c.navigations).toEqual(['/pages/practice/practice-do?mode=single&question_id=99']);
  });

  it('featured：落到精选详情（收藏侧类型名是 featured，**不是**搜索侧的 content）', () => {
    const c = click(fav('featured', 12));
    expect(c.navigations).toEqual(['/pages/featured/featured-detail?id=12']);
  });

  it('course / topic：既有落点不回归', () => {
    expect(click(fav('course', 7)).navigations).toEqual(['/pages/courses/course-detail?id=7']);
    expect(click(fav('topic', 5)).navigations).toEqual(['/pages/forum/forum-detail?id=5']);
  });

  it('未知类型：不跳转（不猜落点），也不弹提示（不是缺陷，只是没落点）', () => {
    const c = click(fav('material', 1));
    expect(c.navigations).toEqual([]);
    expect(c.toasts).toEqual([]);
  });

  it('落点路由全部在 pages.json 注册（防幻影路由）', () => {
    const pagesJson = read('pages.json');
    const urls = [
      click(fav('chapter', 31, 7)).navigations[0],
      click(fav('question', 99)).navigations[0],
      click(fav('featured', 12)).navigations[0],
      click(fav('course', 7)).navigations[0],
      click(fav('topic', 5)).navigations[0],
    ];
    expect(urls.filter((u) => !u)).toEqual([]);
    for (const u of urls) {
      expect(pagesJson).toContain('"' + u.replace(/^\//, '').split('?')[0] + '"');
    }
  });
});

describe('#1089 章节拿不到所属课程：不跳转 + 可见提示（不得静默 no-op）', () => {
  it('course_id == 0 ⇒ 不跳转，且给出可见提示', () => {
    const c = click(fav('chapter', 31, 0));
    expect(c.navigations).toEqual([]);
    expect(c.toasts.length).toBeGreaterThan(0);
    expect(c.toasts[0].length).toBeGreaterThan(0);
  });

  it('course_id < 0（契约异常）⇒ 同样不跳转 + 提示', () => {
    const c = click(fav('chapter', 31, -3));
    expect(c.navigations).toEqual([]);
    expect(c.toasts.length).toBeGreaterThan(0);
  });

  it('提示文案不得与真实页面跳转共用（否则「不跳」不可观测）—— 提示非空且不含协议头', () => {
    const c = click(fav('chapter', 31, 0));
    expect(c.toasts[0]).not.toMatch(/^\/pages\//);
  });

  it('「没有落点」与「有落点但拿不到前提」不得混为一谈：前者静默、后者必提示', () => {
    expect(click(fav('material', 1)).toasts).toEqual([]);          // 表外类型：没落点
    expect(click(fav('chapter', 31, 0)).toasts.length).toBe(1);    // 章节缺课程：有落点但缺前提
  });
});

describe('#1089 camelCase 死键 `chapterId` 已摘除', () => {
  it('落点表与页面源码里都不再把 `chapterId` 当 query 键传（只判键位，不误伤散文提及）', () => {
    // 判据收紧到「出现在 query 键位」——写成 `/chapterId/` 会把日后注释里提到它也算红
    expect(read(MODULE)).not.toMatch(/[?&]chapterId\b/);
    expect(read(PAGE)).not.toMatch(/[?&]chapterId\b/);
  });

  it('不再出现「传了目标页不读」的章节落点（与 navQueryKeyContract 同一判据面）', () => {
    const url = click(fav('chapter', 31, 7)).navigations[0];
    const keys = url.slice(url.indexOf('?') + 1).split('&').map((kv) => kv.split('=')[0]);
    // chapter-view 的 onLoad 读这两键（ADR-0014）
    expect(keys).toEqual(['course_id', 'chapter_id']);
  });
});

describe('#1237 落点口径唯一：页面只剩「应用决策」的接线', () => {
  it('模板仍然把点击接线到 onItemClick（函数在、但没接 = 同样点不开）', () => {
    expect(read(PAGE)).toContain('@click="onItemClick(item)"');
  });

  it('页面 import 共享落点模块并调用它', () => {
    const page = read(PAGE);
    expect(page).toContain("import { favoriteLanding } from '../../utils/favoriteLanding'");
    expect(page).toContain('favoriteLanding(item)');
  });

  it('页面把决策应用到 uni：notice → showToast、url → navigateTo', () => {
    const page = read(PAGE);
    expect(page).toContain('uni.showToast({ title: landing.notice');
    expect(page).toContain('uni.navigateTo({ url: landing.url })');
  });

  it('页面不再内联落点分支（`target_type` 的比较只应出现在被测模块里）', () => {
    const page = read(PAGE);
    expect(page).not.toMatch(/item\.target_type\s*==/);
    expect(read(MODULE)).toMatch(/item\.target_type\s*==/);
  });
});

describe('#1089 落点的数据前提：FavoriteItem.course_id 真的从响应映射进来', () => {
  // 为什么与落点放同一个文件：落点表再好，只要 `buildFavoriteItem` 不读 `course_id`，
  // 真机上 `item.course_id` 恒为 `undefined`/0 ⇒ 章节永远走「不跳 + 提示」。
  // 上面那组用例的夹具是**手写的**，天然测不到这一层 —— 这里补上，否则删掉映射行仍全绿。
  //
  // ⚠️ 这一组仍用取体执行：`buildFavoriteItem` **未 export**，而 `loadUts` 只回读导出名。
  // 要把它也切到共享执行器，得先经 `getFavoritesApi` + 桩 `request` 间接测（另立票，不在 #1237）。

  const API = 'api/favorite.uts';
  const HELPERS = 'api/helpers.uts';

  /** 剥掉 TS/UTS 的 `as X` 断言（`} as FavoriteItem` 在 new Function 里是语法错误） */
  const stripAs = (body) => body.replace(/\s+as\s+[A-Za-z_$][\w$]*/g, '');

  /**
   * 把 `export function <name>(…): T { … }` 取出来，用**源码里的真实函数体**
   * 加注入的依赖构造一个可调用函数（不写镜像实现，避免两份实现悄悄分叉）。
   */
  function fnFromSource(rel, marker, deps, argNames) {
    const body = stripAs(stripUtsTypes(fnBodyOf(read(rel), marker)));
    const names = Object.keys(deps);
    return {
      body,
      call: (...args) => new Function(...names, ...argNames, body)(...names.map((n) => deps[n]), ...args),
    };
  }

  /** 真实的 toNumber / toStr（同样取源码函数体，不是镜像） */
  const helper = (marker) => fnFromSource(HELPERS, marker, {}, ['v', 'defaultVal']);
  const toNumberBody = helper('export function toNumber(').body;
  const toStrBody = helper('export function toStr(').body;
  const toNumber = (v, d = 0) => new Function('v', 'defaultVal', toNumberBody)(v, d);
  const toStr = (v, d = '') => new Function('v', 'defaultVal', toStrBody)(v, d);

  const build = fnFromSource(
    API,
    'function buildFavoriteItem(',
    { toNumber, toStr },
    ['obj'],
  );

  it('解析器自检：两个函数体都真的取到了（防空跑假绿）', () => {
    expect(toNumberBody).toContain('parseFloat');
    expect(toStrBody).toContain('return');
    expect(build.body).toContain('course_id');
  });

  it('后端给 `course_id`（number / 字符串）⇒ 原样映射进来', () => {
    expect(build.call({ target_type: 'chapter', target_id: 31, course_id: 7 }).course_id).toBe(7);
    expect(build.call({ target_type: 'chapter', target_id: 31, course_id: '7' }).course_id).toBe(7);
  });

  it('字段缺失 / null（非 chapter 类型的后端契约）⇒ 0 哨兵，不抛错', () => {
    expect(build.call({ target_type: 'chapter', target_id: 31 }).course_id).toBe(0);
    expect(build.call({ target_type: 'chapter', target_id: 31, course_id: null }).course_id).toBe(0);
    expect(build.call({ target_type: 'course', target_id: 7, course_id: null }).course_id).toBe(0);
  });

  it('映射出来的条目直接喂给落点模块 ⇒ 章节落点成立（数据前提与落点口径接得上）', () => {
    const mapped = build.call({ target_type: 'chapter', target_id: 31, course_id: 7 });
    expect(click(mapped).navigations).toEqual(['/pages/courses/chapter-view?course_id=7&chapter_id=31']);
    const missing = build.call({ target_type: 'chapter', target_id: 31 });
    expect(click(missing).navigations).toEqual([]);
    expect(click(missing).toasts.length).toBe(1);
  });

  it('类型声明里 `course_id` 是**非可空** number（可空会让 Kotlin 侧 `<=` 比较编译报错）', () => {
    const typeSrc = read('types/favorite.uts');
    // 取 FavoriteItem 这一段，避免误命中别的类型
    const block = typeSrc.slice(
      typeSrc.indexOf('export type FavoriteItem'),
      typeSrc.indexOf('export type FavoriteListResult'),
    );
    expect(block).toMatch(/course_id\s*:\s*number\b/);
    expect(block).not.toMatch(/course_id\s*:\s*number\s*\|\s*null/);
  });
});
