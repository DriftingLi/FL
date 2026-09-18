/**
 * 「个人动态 · 收藏」落点契约（#1159）—— 断言**行为**，不断言源码片段。
 *
 * ## 为什么需要这条守护
 *
 * 症状（票面现测）：`pages/profile/personal-activity.uvue` 的「赞 / 收藏 → 收藏」子页里，
 * **`chapter` 与 `question` 两类点了没反应**（静默 no-op），`topic` / `course` / `featured` 正常。
 *
 * 根因是**落点表缺分支**：`onFavoriteClick` 只有 topic / course / featured 三支，
 * 而收藏的 `target_type` 是五类（ADR-0018 多态收藏）。
 *
 * ## 与 #1089 / PR #1147 的关系（同族）
 *
 * `pages/profile/favorites.uvue` 的 `onItemClick` 是**同一族**形态，已由 PR #1147 修好；
 * 本页是「模块化之前的落点表被照抄」的第二处。故本守护：
 *   - **复用** `utils/favoriteLandingHarness.js`（拆自 #1147 的执行器，唯一实现，不写第二套）；
 *   - 额外拿 `utils/searchDisplay.uts` 的 `searchItemPath` 当**权威口径**做交叉断言 ——
 *     同一类条目在两个入口必须落到同一个 url（`featured` ↔ 搜索侧 `content` 只共用路径、
 *     不共用分支 key，故只对 `content` 的**路径形状**做断言，不照抄那边的 switch key）。
 *
 * ## 断言方式（照移动端 ADR-0008）
 *
 * 从 `.uvue` 源码取出 `onFavoriteClick` 的函数体，注入桩 `uni` **真的执行**它，
 * 断言它对外做的事（`navigateTo` 的 url / `showToast` 的 title）。
 *
 * 反向自检（防「提取失败即假绿」）：把**修复前**那段真实坏代码喂进同一执行器，
 * 必须复现「chapter / question 静默无反应」这一形态；提取失败时空函数体不得让正向用例变绿。
 */
const {
  readSource,
  fnBodyOf,
  runLandingBody,
  landingClick,
  fnFromSource,
  favoriteFixture,
} = require('./favoriteLandingHarness');

const PAGE = 'pages/profile/personal-activity.uvue';
const MARKER = 'function onFavoriteClick(';

/** 直接跑页面源码里的 `onFavoriteClick`（每次调用重读源码） */
const click = landingClick(PAGE, MARKER);
/** 收藏夹具：`fav(type, id, courseId)` */
const fav = favoriteFixture;

/** 权威口径：搜索侧的落点函数（同样取源码函数体真跑，不写镜像实现） */
const searchItemPath = fnFromSource('utils/searchDisplay.uts', 'export function searchItemPath(', {}, ['item']);
/** 搜索条目夹具（`searchItemPath` 读 id / type / parent_id） */
const searchItem = (type, id, parentId = 0) => ({ id, type, parent_id: parentId });

describe('#1159 个人动态·收藏落点：五类条目各有正确落点', () => {
  it('chapter：带上 course_id 与 chapter_id 两键（ADR-0014；不是 camelCase chapterId）', () => {
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
    const pagesJson = readSource('pages.json');
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

describe('#1159 与权威口径同源（searchDisplay.searchItemPath）', () => {
  it('解析器自检：`searchItemPath` 的函数体真的取到了（防空跑假绿）', () => {
    expect(searchItemPath.body).toContain('chapter_id');
    expect(searchItemPath.body).toContain('question_id');
  });

  it.each([
    ['course', 'course', 7, 0],
    ['chapter', 'chapter', 31, 7],
    ['question', 'question', 99, 0],
    ['topic', 'topic', 5, 0],
  ])('%s：本页落点与搜索落点**逐字相同**', (label, type, id, parentId) => {
    const mine = click(fav(type, id, parentId)).navigations[0];
    const theirs = searchItemPath.call(searchItem(type, id, parentId));
    expect(mine).toBe(theirs);
  });

  it('featured：只共用**路径**，不照抄搜索侧的 `content` 分支 key（类型名不同是刻意的）', () => {
    const mine = click(fav('featured', 12)).navigations[0];
    // 搜索侧同名落点由 `content` 驱动 —— 拿它与本页比，证明差异只在 key、不在路径
    expect(mine).toBe(searchItemPath.call(searchItem('content', 12)));
    // 反面：本页若照抄 `content` 当 key，`featured` 条目会静默 no-op（本票要防的正是这个）
    expect(click(fav('content', 12)).navigations).toEqual([]);
  });

  it('章节落点键集合恒为 [course_id, chapter_id]（防 camelCase 死键回潮）', () => {
    const url = click(fav('chapter', 31, 7)).navigations[0];
    const keys = url.slice(url.indexOf('?') + 1).split('&').map((kv) => kv.split('=')[0]);
    expect(keys).toEqual(['course_id', 'chapter_id']);
    expect(readSource(PAGE)).not.toMatch(/[?&]chapterId\b/);
  });
});

describe('#1159 章节拿不到所属课程：不跳转 + 可见提示（不得静默 no-op）', () => {
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

  it('提示只在 chapter 缺课程时出现：其余类型的正常落点不得被它污染', () => {
    for (const [type, id, courseId] of [['course', 7, 0], ['topic', 5, 0], ['question', 99, 0], ['featured', 12, 0]]) {
      const c = click(fav(type, id, courseId));
      expect(c.toasts).toEqual([]);
      expect(c.navigations).toHaveLength(1);
    }
  });
});

describe('#1159 模板仍接线（函数在、但没接 = 同样点不开）', () => {
  it('收藏列表把点击接到 onFavoriteClick(item)', () => {
    expect(readSource(PAGE)).toContain('@tap="onFavoriteClick(item)"');
  });

  it('落点函数确实存在于页面（提取成功，非空函数体）', () => {
    expect(fnBodyOf(readSource(PAGE), MARKER)).toContain('target_type');
  });
});

describe('#1159 落点的数据前提：FavoriteItem.course_id 真的从响应映射进来', () => {
  // 为什么与落点放同一个文件：落点表再好，只要 `buildFavoriteItem` 不读 `course_id`，
  // 真机上 `item.course_id` 恒为 0 ⇒ 章节永远走「不跳 + 提示」。
  // 上面那组用例的夹具是**手写的**，天然测不到这一层 —— 这里补上，否则删掉映射行仍全绿。
  const API = 'api/favorite.uts';
  const HELPERS = 'api/helpers.uts';

  /** 真实的 toNumber / toStr（同样取源码函数体，不是镜像） */
  const helper = (marker) => fnFromSource(HELPERS, marker, {}, ['v', 'defaultVal']);
  const toNumberBody = helper('export function toNumber(').body;
  const toStrBody = helper('export function toStr(').body;
  const toNumber = (v, d = 0) => new Function('v', 'defaultVal', toNumberBody)(v, d); // eslint-disable-line no-new-func
  const toStr = (v, d = '') => new Function('v', 'defaultVal', toStrBody)(v, d); // eslint-disable-line no-new-func

  const build = fnFromSource(API, 'function buildFavoriteItem(', { toNumber, toStr }, ['obj']);

  it('解析器自检：三个函数体都真的取到了（防空跑假绿）', () => {
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
    const typeSrc = readSource('types/favorite.uts');
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
  /** 修复前 `personal-activity.uvue` 的真实 `onFavoriteClick` 函数体（抄自页面现测） */
  const OLD_BUGGY_BODY = `
        if (item.target_type === 'topic') {
            uni.navigateTo({ url: '/pages/forum/forum-detail?id=' + item.target_id })
        } else if (item.target_type === 'course') {
            uni.navigateTo({ url: '/pages/courses/course-detail?id=' + item.target_id })
        } else if (item.target_type === 'featured') {
            uni.navigateTo({ url: '/pages/featured/featured-detail?id=' + item.target_id })
        }
  `;

  it('坏代码：chapter 静默无反应（本票要修的形态之一）', () => {
    expect(runLandingBody(OLD_BUGGY_BODY, fav('chapter', 31, 7)).navigations).toEqual([]);
  });

  it('坏代码：question 静默无反应（本票要修的形态之二）', () => {
    expect(runLandingBody(OLD_BUGGY_BODY, fav('question', 99)).navigations).toEqual([]);
  });

  it('坏代码：既有三类仍然正常（证明红的是缺分支，不是执行器坏了）', () => {
    expect(runLandingBody(OLD_BUGGY_BODY, fav('topic', 5)).navigations).toEqual(['/pages/forum/forum-detail?id=5']);
    expect(runLandingBody(OLD_BUGGY_BODY, fav('course', 7)).navigations).toEqual(['/pages/courses/course-detail?id=7']);
  });

  it('提取失败必须是**红**而非空跑：空函数体下正向用例不成立', () => {
    expect(runLandingBody('', fav('chapter', 31, 7)).navigations).toEqual([]);
    expect(runLandingBody('', fav('question', 99)).navigations).toEqual([]);
    expect(runLandingBody('', fav('chapter', 31, 0)).toasts).toEqual([]);
  });
});
