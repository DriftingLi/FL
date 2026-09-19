/**
 * 章节页收藏接线契约（#1140）—— 断言**行为**，不断言源码片段。
 *
 * ## 为什么需要这条守护
 *
 * `chapter` 是通用收藏（ADR-0018）里唯一**没有创建点**的类型：两端章节页都没有收藏控件，
 * 而两端都在消费它（移动端 `pages/profile/favorites.uvue` 有「章节」筛选 chip）⇒
 * 缺入口时该 chip **恒空、无声无息**（没有任何东西会变红，#1132 的诊断）。
 *
 * 本票在 `pages/courses/chapter-view.uvue` 补入口。最容易复现的坏形态是
 * **从同域先例 `course-detail.uvue` 抄接线而忘改 `target_type` / 忘换 id**：
 * 页面会「看起来能用」（有心形、点击有 toast），但收藏的是**课程**、不是当前章节 ——
 * 而课程收藏早有入口 ⇒ 症状是「章节页点了收藏，我的收藏·章节里什么都没有」。
 * 只断言「源码里出现过 checkFavoriteApi」抓不到这一面（那是坏实现的超集）。
 *
 * ## 断言方式（照 移动端 docs/adr/0008「守护从断言源码文本改为断言行为」）
 *
 * 从 `.uvue` 源码里**取出 `loadFavoriteStatus` / `toggleFavorite` / `loadDetail` 的函数体**，
 * 注入桩（ref 用 `{ value }` 模拟）后**真的执行**，断言它对外做的事：
 * 调了哪个 API、参数是什么、状态与 toast 变成什么。不写镜像实现。
 *
 * 反向自检（防「提取失败即假绿」）：把**修复前**的真实坏代码（course 版接线 / 谎报成功）
 * 喂进同一个执行器，必须复现坏形态；提取失败时正向用例判红。
 *
 * ⚠️ 已知边界（如实声明）：本守护不覆盖**模板渲染**（`favorite-icon` 两态字形由
 * `utils/courseFavoriteAffordanceContract.test.js` 对课程详情守，章节页同形）。
 * 「真机上确实点了能收藏、列表里出现、点开渲染正文」由 ①a 真机取证承担。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE = 'pages/courses/chapter-view.uvue';
const read = (rel) => readText(path.join(ROOT, rel));

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

/** 剥掉 UTS 的标量类型标注，让函数体能在 Node 里直接跑（写法变了不至于变噪音红） */
function stripUtsTypes(body) {
  return body.replace(/:\s*(string|number|boolean|UTSJSONObject|FavoriteItem|Promise<void>)\b/g, '');
}

/** 直接执行一段函数体（正文用例与「坏代码自检」共用同一执行器） */
function runBody(body, deps, argNames = [], args = []) {
  const names = Object.keys(deps);
  const src = `return (async () => {${stripUtsTypes(body)}})()`;
  const fn = new Function(...names, ...argNames, src);
  return fn(...names.map((n) => deps[n]), ...args);
}

/** 从页面源码取函数体并包成可调用函数 */
function fnFromPage(marker, deps) {
  const body = fnBodyOf(read(PAGE), marker);
  return { body, call: () => runBody(body, deps) };
}

/** 一次「页面执行」的完整夹具：三个真实函数体 + 记录对外副作用的桩 */
function makePage(overrides = {}) {
  const calls = { checks: [], adds: [], removes: [], toasts: [], warns: [], errors: [] };
  const state = {
    courseId: { value: overrides.courseId != null ? overrides.courseId : 7 },
    chapterId: { value: overrides.chapterId != null ? overrides.chapterId : 31 },
    loading: { value: true },
    detail: { value: null },
    courseDetail: { value: null },
    isFavorited: { value: overrides.isFavorited === true },
    favoriteId: { value: overrides.favoriteId != null ? overrides.favoriteId : 0 },
  };

  const deps = {
    courseId: state.courseId,
    chapterId: state.chapterId,
    loading: state.loading,
    detail: state.detail,
    courseDetail: state.courseDetail,
    isFavorited: state.isFavorited,
    favoriteId: state.favoriteId,
    checkFavoriteApi: (type, id) => {
      calls.checks.push([type, id]);
      return overrides.check != null
        ? overrides.check(type, id)
        : Promise.resolve({ favorited: false, favorite_id: 0 });
    },
    addFavoriteApi: (type, id) => {
      calls.adds.push([type, id]);
      return overrides.add != null
        ? overrides.add(type, id)
        : Promise.resolve({ favorite_id: 55 });
    },
    removeFavoriteApi: (id) => {
      calls.removes.push(id);
      return overrides.remove != null ? overrides.remove(id) : Promise.resolve();
    },
    uni: { showToast: (o) => calls.toasts.push(o.title) },
    console: { warn: (...a) => calls.warns.push(a.join(' ')), error: (...a) => calls.errors.push(a.join(' ')) },
    study: { stopStudy: () => {}, beginStudy: () => {}, reportIncremental: () => Promise.resolve(), reportCompleted: () => Promise.resolve() },
    getChapterDetailApi: overrides.getChapterDetailApi != null
      ? overrides.getChapterDetailApi
      : () => Promise.resolve({ chapter_id: state.chapterId.value }),
    getCourseDetailApi: () => Promise.resolve({ chapters: [] }),
    updateNavTitles: () => {},
  };

  const loadFavoriteStatus = fnFromPage('async function loadFavoriteStatus(', deps);
  // loadDetail 里那句 loadFavoriteStatus() 指向**同一个真实函数体**（不注入空桩，
  // 否则「切章后重查」就会退化成「loadDetail 里出现过这个名字」）
  deps.loadFavoriteStatus = loadFavoriteStatus.call;
  const loadDetail = fnFromPage('async function loadDetail(', deps);
  const toggleFavorite = fnFromPage('async function toggleFavorite(', deps);

  return { calls, state, loadFavoriteStatus, loadDetail, toggleFavorite, deps };
}

describe('#1140 解析器自检：三个函数体真的都取到了（防空跑假绿）', () => {
  const page = makePage();

  it('loadFavoriteStatus / toggleFavorite / loadDetail 函数体非空且特征正确', () => {
    expect(page.loadFavoriteStatus.body).toContain('checkFavoriteApi');
    expect(page.toggleFavorite.body).toContain('addFavoriteApi');
    expect(page.toggleFavorite.body).toContain('removeFavoriteApi');
    expect(page.loadDetail.body).toContain('getChapterDetailApi');
  });
});

describe('#1140 进页 / 切章后重查：状态查询的 target_type 与 id', () => {
  it('查询用 `chapter` + **当前章节 id**（不是课程 id）', async () => {
    const page = makePage({ courseId: 7, chapterId: 31, check: () => Promise.resolve({ favorited: true, favorite_id: 9 }) });
    await page.loadFavoriteStatus.call();
    expect(page.calls.checks).toEqual([['chapter', 31]]);
    expect(page.state.isFavorited.value).toBe(true);
    expect(page.state.favoriteId.value).toBe(9);
  });

  it('未收藏的章节 ⇒ 两态归零（不残留上一章的收藏态）', async () => {
    const page = makePage({ chapterId: 32, isFavorited: true, favoriteId: 9, check: () => Promise.resolve({ favorited: false, favorite_id: 0 }) });
    await page.loadFavoriteStatus.call();
    expect(page.calls.checks).toEqual([['chapter', 32]]);
    expect(page.state.isFavorited.value).toBe(false);
    expect(page.state.favoriteId.value).toBe(0);
  });

  it('查询失败 ⇒ 不冒泡（await 不 reject）、有告警，且不改写状态 —— 不阻断正文渲染', async () => {
    const page = makePage({ check: () => Promise.reject(new Error('boom')) });
    await expect(page.loadFavoriteStatus.call()).resolves.toBeUndefined();
    expect(page.calls.warns.length).toBeGreaterThan(0);
    expect(page.state.isFavorited.value).toBe(false);
  });

  it('章号非法（<= 0）⇒ 不发请求（不打扰后端）', async () => {
    const page = makePage({ chapterId: 0 });
    await page.loadFavoriteStatus.call();
    expect(page.calls.checks).toEqual([]);
  });

  it('切章后重查：loadDetail 跑完会**真的**查一次当前章节（不是在源码里出现个名字）', async () => {
    const page = makePage({ chapterId: 32, check: () => Promise.resolve({ favorited: true, favorite_id: 7 }) });
    await page.loadDetail.call();
    expect(page.calls.checks).toEqual([['chapter', 32]]);
    expect(page.state.isFavorited.value).toBe(true);
  });

  it('连点上一章 / 下一章时状态跟着走：每次 loadDetail 都用**新的**章节 id 重查', async () => {
    const page = makePage({ chapterId: 31, check: () => Promise.resolve({ favorited: false, favorite_id: 0 }) });
    await page.loadDetail.call();
    page.state.chapterId.value = 32;
    await page.loadDetail.call();
    expect(page.calls.checks).toEqual([['chapter', 31], ['chapter', 32]]);
  });

  it('竞态：旧章节的查询迟到 ⇒ 不得覆盖当前章节的状态', async () => {
    let resolveOld = null;
    const page = makePage({
      chapterId: 31,
      check: () => new Promise((res) => { resolveOld = res; }),
    });
    const pending = page.loadFavoriteStatus.call();
    // 用户在响应回来之前切到了下一章
    page.state.chapterId.value = 32;
    resolveOld({ favorited: true, favorite_id: 9 });
    await pending;
    expect(page.state.isFavorited.value).toBe(false);
    expect(page.state.favoriteId.value).toBe(0);
  });
});

describe('#1140 点击收藏 / 取消：调哪个 API、参数是什么、状态与提示是什么', () => {
  it('未收藏 ⇒ add(`chapter`, 当前章节 id) 恰一次，置为已收藏并记下 favorite_id', async () => {
    const page = makePage({ courseId: 7, chapterId: 31, add: () => Promise.resolve({ favorite_id: 55 }) });
    await page.toggleFavorite.call();
    expect(page.calls.adds).toEqual([['chapter', 31]]);
    expect(page.calls.removes).toEqual([]);
    expect(page.state.isFavorited.value).toBe(true);
    expect(page.state.favoriteId.value).toBe(55);
    expect(page.calls.toasts).toEqual(['收藏成功']);
  });

  it('已收藏 ⇒ remove(favorite_id) 恰一次并归零（不再发 add）', async () => {
    const page = makePage({ isFavorited: true, favoriteId: 55 });
    await page.toggleFavorite.call();
    expect(page.calls.removes).toEqual([55]);
    expect(page.calls.adds).toEqual([]);
    expect(page.state.isFavorited.value).toBe(false);
    expect(page.state.favoriteId.value).toBe(0);
    expect(page.calls.toasts).toEqual(['已取消收藏']);
  });

  it('收藏失败 ⇒ 提示「操作失败」，且**不得谎报成功**（状态仍为未收藏）', async () => {
    const page = makePage({ add: () => Promise.reject(new Error('HTTP 400')) });
    await page.toggleFavorite.call();
    expect(page.calls.toasts).toEqual(['操作失败']);
    expect(page.state.isFavorited.value).toBe(false);
    expect(page.state.favoriteId.value).toBe(0);
    expect(page.calls.errors.length).toBeGreaterThan(0);
  });

  it('取消收藏失败 ⇒ 状态仍为已收藏（不得谎报已取消）', async () => {
    const page = makePage({ isFavorited: true, favoriteId: 55, remove: () => Promise.reject(new Error('HTTP 500')) });
    await page.toggleFavorite.call();
    expect(page.calls.toasts).toEqual(['操作失败']);
    expect(page.state.isFavorited.value).toBe(true);
    expect(page.state.favoriteId.value).toBe(55);
  });

  it('add 用的是**章节 id**、remove 用的是 favorite_id（两张 id 不得串）', async () => {
    const page = makePage({ courseId: 7, chapterId: 31, isFavorited: true, favoriteId: 55 });
    await page.toggleFavorite.call();
    expect(page.calls.removes).toEqual([55]);
    page.state.isFavorited.value = false;
    await page.toggleFavorite.call();
    // 第二次是「已取消后再收藏」：仍必须是 chapter + 章节 id，不得落到 courseId
    expect(page.calls.adds).toEqual([['chapter', 31]]);
  });

  it('章号非法（<= 0）⇒ 不发请求（不产生悬挂的收藏行）', async () => {
    const page = makePage({ chapterId: 0 });
    await page.toggleFavorite.call();
    expect(page.calls.adds).toEqual([]);
    expect(page.calls.removes).toEqual([]);
  });
});

describe('#1140 红能力自检：坏接线必须被同一执行器抓出来（防提取失败即假绿）', () => {
  /** 坏形态 ①：照抄课程详情的接线 —— `target_type` 是 `course`、id 用 courseId */
  const COURSE_COPY_BODY = `
        try {
            if (isFavorited.value) {
                await removeFavoriteApi(favoriteId.value)
                isFavorited.value = false
                favoriteId.value = 0
                uni.showToast({ title: '已取消收藏', icon: 'none' })
            } else {
                const result = await addFavoriteApi('course', courseId.value)
                isFavorited.value = true
                favoriteId.value = result.favorite_id
                uni.showToast({ title: '收藏成功', icon: 'none' })
            }
        } catch (e) {
            console.error('[chapter-view] toggleFavorite failed:', e)
            uni.showToast({ title: '操作失败', icon: 'none' })
        }
  `;

  /** 坏形态 ②：失败也把状态置成「已收藏」——谎报成功 */
  const LIE_ON_FAILURE_BODY = `
        try {
            const result = await addFavoriteApi('chapter', chapterId.value)
            isFavorited.value = true
            favoriteId.value = result.favorite_id
            uni.showToast({ title: '收藏成功', icon: 'none' })
        } catch (e) {
            isFavorited.value = true
            uni.showToast({ title: '收藏成功', icon: 'none' })
        }
  `;

  const stubs = (over) => {
    const calls = { adds: [], removes: [], toasts: [] };
    const state = {
      courseId: { value: 7 }, chapterId: { value: 31 },
      isFavorited: { value: false }, favoriteId: { value: 0 },
    };
    const deps = {
      courseId: state.courseId, chapterId: state.chapterId,
      isFavorited: state.isFavorited, favoriteId: state.favoriteId,
      addFavoriteApi: (t, i) => { calls.adds.push([t, i]); return over.add ? over.add() : Promise.resolve({ favorite_id: 55 }); },
      removeFavoriteApi: (i) => { calls.removes.push(i); return Promise.resolve(); },
      uni: { showToast: (o) => calls.toasts.push(o.title) },
      console: { error: () => {}, warn: () => {} },
    };
    return { calls, state, deps };
  };

  it('坏形态①：收藏落到 `course` + courseId（本票要防的抄错），正向判据必然不成立', async () => {
    const { calls, deps } = stubs({});
    await runBody(COURSE_COPY_BODY, deps);
    expect(calls.adds).toEqual([['course', 7]]);
    expect(calls.adds).not.toEqual([['chapter', 31]]);
  });

  it('坏形态②：add 失败仍报「收藏成功」且状态为已收藏（正向判据必然不成立）', async () => {
    const { calls, state, deps } = stubs({ add: () => Promise.reject(new Error('HTTP 400')) });
    await runBody(LIE_ON_FAILURE_BODY, deps);
    expect(state.isFavorited.value).toBe(true);
    expect(calls.toasts).toEqual(['收藏成功']);
  });

  it('提取失败（空函数体）⇒ 正向用例不成立，不空跑变绿', async () => {
    const { calls, state, deps } = stubs({});
    await runBody('', deps);
    expect(calls.adds).toEqual([]);
    expect(state.isFavorited.value).toBe(false);
    expect(fnBodyOf(read(PAGE), 'async function doesNotExist(')).toBe('');
  });
});
