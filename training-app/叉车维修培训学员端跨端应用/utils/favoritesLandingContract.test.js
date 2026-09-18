/**
 * 「我的收藏」落点契约（#1089 PR-B）—— 断言**行为**，不断言源码片段。
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
 *   ③ 分支缺失 —— `onItemClick` 只有 course / topic / chapter 三个分支，而筛选 chip 有六项
 *      （「资讯」= `featured`、「题目」= `question`）⇒ 这两类**压根没有分支**，静默 no-op。
 *
 * 既有守护 `navQueryKeyContract.test.js` 只能抓「**传了**目标页不读的键」（方向①），
 * 抓不到方向③「压根没传、没有分支」。本文件补的正是这一面。
 *
 * ## 断言方式（照 ADR-0008「守护从断言源码文本改为断言行为」）
 *
 * 从 `.uvue` 源码里**取出 `onItemClick` 的函数体**，注入桩 `uni` 后**真的执行**它，
 * 断言它对外做的事（navigateTo 的 url / showToast 的 title）—— 而不是断言源码里
 * 出现过某个字符串（那是坏实现的超集：写对字符串、接错分支照样绿）。
 *
 * 反向自检（防「提取失败即假绿」）：把**修复前**那段真实坏代码喂进同一个执行器，
 * 必须复现「章节跳到 camelCase 键」+「question / featured 静默无反应」两种形态。
 * 提取函数返回空串时，正向用例会**判红**（而不是空跑变绿）。
 *
 * ⚠️ 覆盖边界（2026-09-18 更新，#1159 改判后）：本守护覆盖**全仓唯一**的收藏落点表 ——
 *   `favorites.uvue` 的 `onItemClick`。原先并存的第二处
 *   （`pages/profile/personal-activity.uvue` 的 `onFavoriteClick`）已随「收藏的唯一列表承载面」
 *   裁定（根仓库 `ADR-0018` 补遗）**整体摘除**；那一侧改由 `personalActivityContract.test.js`
 *   反向钉住「收藏面不得回潮」。
 *
 *   ⇒ **新增 `target_type` 时必须改本文件**：这里是全仓唯一会因「缺分支」判红的地方。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const PAGE = 'pages/profile/favorites.uvue';
/** 归一 CRLF（工作树里的 .uvue 是 CRLF，不归一则按 `\n` 锚定的逻辑会错位） */
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');

/**
 * 取 `marker` 之后那个函数的**函数体**（花括号配平，与缩进风格 / 行尾无关）。
 * 配平而非「找下一个行首 `}`」：本仓空格与制表符混用（先例 #1111 契约测试记的三个坑）。
 * 解析失败返回 `''` —— 调用方的正向用例随即判红，不静默放过。
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

/**
 * 剥掉 UTS 的标量类型标注（`const url : string = …`），让函数体能在 Node 里直接跑。
 * 本仓 UTS 写法允许局部变量带标注 —— 不剥会在 `new Function` 里变成语法错误（判红，
 * 是安全方向），但会让守护变成「改个写法就红」的噪音源，故显式剥掉。
 */
function stripUtsTypes(body) {
  return body.replace(/:\s*(string|number|boolean|UTSJSONObject|FavoriteItem)\b/g, '');
}

/** 在沙箱里执行一段 `onItemClick` 函数体，返回它对外做的事 */
function runOnItemClickBody(body, item) {
  const calls = { navigations: [], toasts: [] };
  const uni = {
    navigateTo: (o) => calls.navigations.push(o.url),
    showToast: (o) => calls.toasts.push(o.title),
  };
  // 只跑从源码里取出的函数体；不引入被测页面的任何其它代码
  new Function('item', 'uni', stripUtsTypes(body))(item, uni);
  return calls;
}

/** 便捷：直接跑页面源码里的 `onItemClick` */
function click(item) {
  return runOnItemClickBody(fnBodyOf(read(PAGE), 'function onItemClick('), item);
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
});

describe('#1089 camelCase 死键 `chapterId` 已摘除', () => {
  it('页面源码里不再把 `chapterId` 当 query 键传（只判键位，不误伤散文提及）', () => {
    // 判据收紧到「出现在 query 键位」——写成 `/chapterId/` 会把日后注释里提到它也算红
    expect(read(PAGE)).not.toMatch(/[?&]chapterId\b/);
  });

  it('模板仍然把点击接线到 onItemClick（函数在、但没接 = 同样点不开）', () => {
    expect(read(PAGE)).toContain('@click="onItemClick(item)"');
  });

  it('不再出现「传了目标页不读」的章节落点（与 navQueryKeyContract 同一判据面）', () => {
    const url = click(fav('chapter', 31, 7)).navigations[0];
    const keys = url.slice(url.indexOf('?') + 1).split('&').map((kv) => kv.split('=')[0]);
    // chapter-view 的 onLoad 读这两键（ADR-0014）
    expect(keys).toEqual(['course_id', 'chapter_id']);
  });
});

describe('#1089 落点的数据前提：FavoriteItem.course_id 真的从响应映射进来', () => {
  // 为什么与落点放同一个文件：落点表再好，只要 `buildFavoriteItem` 不读 `course_id`，
  // 真机上 `item.course_id` 恒为 `undefined`/0 ⇒ 章节永远走「不跳 + 提示」。
  // 上面那组用例的夹具是**手写的**，天然测不到这一层 —— 这里补上，否则删掉映射行仍全绿。

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

describe('红能力自检：修复前的真实坏代码必须被同一执行器抓出来（防提取失败即假绿）', () => {
  /** 修复前 favorites.uvue 的真实 `onItemClick` 函数体（抄自 #1089 正文） */
  const OLD_BUGGY_BODY = `
        if (item.target_type == 'course') {
            uni.navigateTo({ url: '/pages/courses/course-detail?id=' + item.target_id })
        } else if (item.target_type == 'topic') {
            uni.navigateTo({ url: '/pages/forum/forum-detail?id=' + item.target_id })
        } else if (item.target_type == 'chapter') {
            uni.navigateTo({ url: '/pages/courses/chapter-view?chapterId=' + item.target_id })
        }
  `;

  it('坏代码：章节落到 camelCase `chapterId`（缺 course_id）', () => {
    const c = runOnItemClickBody(OLD_BUGGY_BODY, fav('chapter', 31, 7));
    expect(c.navigations).toEqual(['/pages/courses/chapter-view?chapterId=31']);
    expect(c.navigations[0]).toContain('chapterId');
    expect(c.navigations[0]).not.toContain('course_id');
  });

  it('坏代码：question / featured 静默无反应（本票要修的形态同源）', () => {
    expect(runOnItemClickBody(OLD_BUGGY_BODY, fav('question', 99)).navigations).toEqual([]);
    expect(runOnItemClickBody(OLD_BUGGY_BODY, fav('featured', 12)).navigations).toEqual([]);
  });

  it('坏代码：course_id == 0 时仍然跳转（本票要求「不跳 + 提示」的反面）', () => {
    const c = runOnItemClickBody(OLD_BUGGY_BODY, fav('chapter', 31, 0));
    expect(c.navigations).toHaveLength(1);
    expect(c.toasts).toEqual([]);
  });

  it('提取失败必须是**红**而非空跑：空函数体下正向用例不成立', () => {
    expect(runOnItemClickBody('', fav('chapter', 31, 7)).navigations).toEqual([]);
    expect(runOnItemClickBody('', fav('chapter', 31, 0)).toasts).toEqual([]);
  });
});
