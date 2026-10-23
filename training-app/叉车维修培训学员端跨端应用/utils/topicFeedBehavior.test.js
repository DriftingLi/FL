/**
 * forum 帖子流（`composables/useTopicFeed.uts`）· **行为级**测试（#1733 / 移动端 ADR-0027 第一格内服）。
 *
 * 为什么是行为而不是源码文本：本票要修的是一件**只有跑起来才看得见**的事 ——
 * `pages/forum/forum.uvue:136-146` 切 Tab 时旧 Tab 的在飞响应能不能污染新 Tab 的列表。
 * 迁移前那版 `loadTopics` 第一行是 `if (loading.value) return`，把「装载中切 Tab」整个吞掉；
 * 源码文本断言（`utils/forumContract.test.js`）在那种缺陷下**照样绿**。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 把两个真件都当 JS 执行 —— 被测模块是真
 * `useTopicFeed.uts`，它内部持有的也是**真** `usePagedList.uts`（把前者的 `usePagedList`
 * 绑定指后者的导出）。注入的只有 Vue 反应式替身、api 桩与 `uni` 桩。
 *
 * 判据（每条都对应票面验收）：
 *   ① tab→query 映射与 `pageSize` 逐字照旧（迁的是装载协议，不是查询语义）；
 *   ② **切 Tab 换代**：旧 Tab 的响应迟到时不得写进新 Tab 的列表，也不得把新代的数据拼歪；
 *   ③ 触底追加递第 2 页、在飞重入只发一次；
 *   ④ 终止判据来自信封的 `hasMore`（`noMore` 是它的派生取反），到底后不再发请求；
 *   ⑤ 失败时保留已有条目、`loading` 归假、**toast 仍由本模块负责**（模块本体不碰 `uni.*`）。
 *
 * **成对取证**（ADR-0008 ③ 判据③：只跑通过的那一次不算验收）：末组用注入变异的副本证明判据有牙 ——
 *   ① 把 `reset` 分支从 `loadFirst` 换成 `onLoadMore` ⇒ ②「切 Tab 换代」判红；
 *   ② 把信封的 `hasMore` 写死成 `true` ⇒ ④「到底不再发」判红；
 *   ③ 删掉失败分支的 toast ⇒ ⑤ 判红；
 *   ④ 拆掉换代令牌 ⇒ ②「旧代失败不得弹 toast」判红。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const FEED_UTS = path.join(__dirname, '..', 'composables', 'useTopicFeed.uts');
const PAGED_UTS = path.join(__dirname, '..', 'composables', 'usePagedList.uts');

/** Vue 反应式的最小替身（本仓 node 侧解析不到 `vue`：它是 uni-app-x 编译期依赖） */
const vueStub = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/** 等微任务链跑完：被测模块是 `await api → .then/.catch` 两跳，单次 setImmediate 不够稳 */
const flush = async () => {
  for (let i = 0; i < 4; i++) {
    await new Promise((resolve) => setImmediate(resolve));
  }
};

/**
 * 组装一个帖子流实例。
 * `getForumTopicsApi` 每次调用返回**可控 deferred**：测试自己决定哪一代、哪一页先落地 ——
 * 「旧 Tab 迟到」这类时序判据只有可控时序才测得出来。
 * @param file 被执行的 `useTopicFeed.uts` 路径（成对取证时传变异副本）
 * @param tab  初始 Tab（`square` | `hot` | `experience` | `resource`）
 */
function harnessAt(file, tab) {
  const calls = [];
  const toasts = [];
  const getForumTopicsApi = (...args) => {
    let resolveFn = null;
    let rejectFn = null;
    const promise = new Promise((resolve, reject) => { resolveFn = resolve; rejectFn = reject; });
    calls.push({ args, resolve: resolveFn, reject: rejectFn });
    return promise;
  };
  // 真 usePagedList：被测模块内部持有它（不是替身 —— 代际守卫在它里面，替换掉就测不到真行为）
  const pagedMod = loadUts(PAGED_UTS, { ...vueStub() });
  const feedMod = loadUts(file, {
    ...vueStub(),
    getForumTopicsApi,
    usePagedList: pagedMod.usePagedList,
    uni: { showToast: (opt) => { toasts.push(opt); } },
  });
  const currentTab = { value: tab };
  return { api: feedMod.useTopicFeed(currentTab), calls, toasts, currentTab };
}

const harness = (tab = 'square') => harnessAt(FEED_UTS, tab);

/** 读真源 → 注入变异 → 落到临时目录 → 返回变异副本路径（工作树不动） */
function mutatedCopy(absFile, replacements, prefix) {
  let src = readText(absFile);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  const file = path.join(dir, path.basename(absFile));
  fs.writeFileSync(file, src);
  return file;
}

/** 一批成功响应（信封字段与 `Page<ForumTopic>` 同形：items / total / hasMore；#1737 起列表出口就是它） */
const page1 = (items, hasMore = true) => ({ items, total: items.length + (hasMore ? 10 : 0), hasMore });

// ===== ① 查询语义与 pageSize 逐字照旧 =====

describe('首屏：`loadTopics(true)` 换到第 1 页，查询语义不变', () => {
  it('广场 tab：scope=general、category=discussion，page=1、pageSize=10（页码与页宽都从本模块闭包走）', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    expect(calls.length).toBe(1);
    expect(calls[0].args).toEqual([1, 10, 'general', 'latest', 'desc', 'discussion', '', '', '']);
    calls[0].resolve(page1(['a']));
    await flush();
    expect(api.topics.value).toEqual(['a']);
    expect(api.loading.value).toBe(false);
  });

  it('知识问答 tab：scope=all、category=question、sort 由 qaSort 决定（recommend/hot ⇒ hot）', async () => {
    const { api, calls } = harness('hot');
    api.loadTopics(true);
    expect(calls[0].args).toEqual([1, 10, 'all', 'hot', 'desc', 'question', '', '', '']);
    calls[0].resolve(page1(['q']));
    await flush();
  });

  it('备考经验 tab：scope=all、category 清空、isExperience=true、默认排序 latest', async () => {
    const { api, calls } = harness('experience');
    api.loadTopics(true);
    expect(calls[0].args).toEqual([1, 10, 'all', 'latest', 'desc', '', '', '', 'true']);
    calls[0].resolve(page1(['e']));
    await flush();
  });

  it('精选筛选与问答状态经第 7/8 个参数透传（顺序即 api 契约，勿改）', async () => {
    const { api, calls } = harness('hot');
    api.onFeaturedChange('true');
    api.onQaStatusChange('resolved');
    expect(calls[calls.length - 1].args).toEqual([1, 10, 'all', 'hot', 'desc', 'question', 'resolved', 'true', '']);
    calls[calls.length - 1].resolve(page1(['q']));
    await flush();
  });
});

// ===== ② 切 Tab 换代（本票的承重判据）=====

describe('切 Tab 换代：旧 Tab 的迟到响应不得写进新 Tab 的列表', () => {
  it('广场请求在飞时切到问答并重载 ⇒ 旧响应到达后列表仍是问答那批', async () => {
    const { api, calls, currentTab } = harness('square');
    api.loadTopics(true);                       // 广场第 1 页在飞
    expect(calls.length).toBe(1);
    currentTab.value = 'hot';                   // 壳层 onSwitchTab
    api.loadTopics(true);                       // 换代：问答第 1 页
    expect(calls.length).toBe(2);
    expect(calls[1].args[5]).toBe('question');
    calls[1].resolve(page1(['q1'], false));     // 新代先落地
    await flush();
    expect(api.topics.value).toEqual(['q1']);
    calls[0].resolve(page1(['stale']));         // 旧代此刻才到
    await flush();
    expect(api.topics.value).toEqual(['q1']);   // 旧代不得覆盖、也不得被拼进来
    expect(api.noMore.value).toBe(true);        // 新代 hasMore=false 的派生没有被旧代打歪
  });

  it('旧代失败也不得冒泡成用户的 toast（否则切 Tab 会弹出上一屏的失败）', async () => {
    const { api, calls, currentTab, toasts } = harness('square');
    api.loadTopics(true);
    currentTab.value = 'hot';
    api.loadTopics(true);
    calls[1].resolve(page1(['q1'], false));
    await flush();
    calls[0].reject(new Error('stale boom'));
    await flush();
    expect(toasts).toEqual([]);
    expect(api.topics.value).toEqual(['q1']);
  });
});

// ===== ③ 触底追加 =====

describe('触底追加：递第 2 页并拼接；在飞重入只发一次', () => {
  it('`onLoadMore` 递 page=2，列表拼接、noMore 随新批次', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].resolve(page1(['a']));
    await flush();
    expect(api.noMore.value).toBe(false);       // hasMore=true ⇒ 还能翻
    api.onLoadMore();
    expect(calls.length).toBe(2);
    expect(calls[1].args[0]).toBe(2);           // 页码前进由 usePagedList 管
    calls[1].resolve(page1(['b'], false));
    await flush();
    expect(api.topics.value).toEqual(['a', 'b']);
    expect(api.noMore.value).toBe(true);
  });

  it('在飞时连续触底 ⇒ 只发一次请求（重入守卫仍在模块里生效）', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].resolve(page1(['a']));
    await flush();
    api.onLoadMore();
    api.onLoadMore();
    api.onLoadMore();
    expect(calls.length).toBe(2);
  });
});

// ===== ④ 终止判据 =====

describe('终止判据来自信封：`noMore` 是 `hasMore` 的派生取反', () => {
  it('首批 hasMore=false ⇒ noMore 立刻为真；再触底不再发请求', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].resolve(page1(['a'], false));
    await flush();
    expect(api.noMore.value).toBe(true);
    api.onLoadMore();
    expect(calls.length).toBe(1);
  });

  it('空批次即使信封说 hasMore=true 也判到底（空批次兜底在协议本体里，不在本模块重写）', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].resolve({ items: [], total: 0, hasMore: true });
    await flush();
    expect(api.noMore.value).toBe(true);
    api.onLoadMore();
    expect(calls.length).toBe(1);
  });

  it('刷新（再 `loadTopics(true)`）把 noMore 复位：新代 hasMore=true ⇒ 又能翻', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].resolve(page1(['a'], false));
    await flush();
    expect(api.noMore.value).toBe(true);
    api.loadTopics(true);
    calls[1].resolve(page1(['a2'], true));
    await flush();
    expect(api.noMore.value).toBe(false);
    api.onLoadMore();
    expect(calls.length).toBe(3);
    expect(calls[2].args[0]).toBe(2);
  });
});

// ===== ⑤ 失败态 =====

describe('失败：保留已有条目、loading 归假、toast 由本模块负责（模块本体不碰 uni.*）', () => {
  it('首屏失败 ⇒ 弹出「加载帖子失败」、列表不变、loading 归假', async () => {
    const { api, calls, toasts } = harness('square');
    api.loadTopics(true);
    calls[0].resolve(page1(['a']));
    await flush();
    api.onLoadMore();
    calls[1].reject(new Error('boom'));
    await flush();
    expect(toasts.map((t) => t.title)).toEqual(['加载帖子失败']);
    expect(api.topics.value).toEqual(['a']);    // 保留已有数据（ADR-0027 ③）
    expect(api.loading.value).toBe(false);
    expect(api.noMore.value).toBe(false);       // 失败不动终止判据：重试即 loadTopics(true)
  });

  it('失败后刷新仍能取到第 1 页（恢复路径不被失败态卡死）', async () => {
    const { api, calls } = harness('square');
    api.loadTopics(true);
    calls[0].reject(new Error('boom'));
    await flush();
    api.loadTopics(true);
    expect(calls[calls.length - 1].args[0]).toBe(1);
    calls[calls.length - 1].resolve(page1(['again'], false));
    await flush();
    expect(api.topics.value).toEqual(['again']);
  });
});

// ===== 成对取证（必红）：判据在坏实现上确实会红 =====

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必红 · `reset` 分支改走 `onLoadMore`（不换代）⇒ 「切 Tab 要换到第 1 页」判红', async () => {
    const broken = harnessAt(
      mutatedCopy(FEED_UTS, [[
        'paged.loadFirst()',
        'paged.onLoadMore()',
      ]], 'topic-feed-reset-'),
      'square',
    );
    broken.api.loadTopics(true);
    // 坏实现在没有 hasMore 时直接短路 ⇒ 首屏一个请求都不发，「page===1」判据必然判红
    expect(broken.calls.length).toBe(0);
  });

  it('必红 · 信封 `hasMore` 写死成 `true` ⇒ 「到底不再发请求」判红', async () => {
    const broken = harnessAt(
      mutatedCopy(FEED_UTS, [[
        'hasMore: result.hasMore,',
        'hasMore: true,',
      ]], 'topic-feed-hasmore-'),
      'square',
    );
    broken.api.loadTopics(true);
    broken.calls[0].resolve(page1(['a'], false));
    await flush();
    // 坏实现把「后端说没有了」吞掉 ⇒ 真源上 noMore===true 的断言判红
    expect(broken.api.noMore.value).toBe(false);
    broken.api.onLoadMore();
    expect(broken.calls.length).toBe(2);
  });

  it('必红 · 拆掉换代令牌（toast 不再分代）⇒ ②「旧代失败不得弹 toast」判红', async () => {
    const broken = harnessAt(
      mutatedCopy(FEED_UTS, [[
        'requestToken = requestToken + 1',
        'requestToken = requestToken',
      ]], 'topic-feed-token-'),
      'square',
    );
    broken.api.loadTopics(true);
    broken.currentTab.value = 'hot';
    broken.api.loadTopics(true);
    broken.calls[1].resolve(page1(['q1'], false));
    await flush();
    broken.calls[0].reject(new Error('stale boom'));
    await flush();
    // 坏实现在旧代失败时照样 toast ⇒ 真源上「换代后 toasts 为空」判红
    expect(broken.toasts.map((t) => t.title)).toEqual(['加载帖子失败']);
    expect(broken.api.topics.value).toEqual(['q1']);
  });

  it('必红 · 删掉失败分支的 toast ⇒ 用户可见的失败提示消失，⑤ 判红', async () => {
    const broken = harnessAt(
      mutatedCopy(FEED_UTS, [[
        "uni.showToast({ title: '加载帖子失败', icon: 'none' })",
        'null',
      ]], 'topic-feed-toast-'),
      'square',
    );
    broken.api.loadTopics(true);
    broken.calls[0].reject(new Error('boom'));
    await flush();
    // 坏实现静默失败 ⇒ 真源上 toasts 必有一条「加载帖子失败」
    expect(broken.toasts).toEqual([]);
    expect(broken.api.loading.value).toBe(false);
  });
});
