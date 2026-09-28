/**
 * 分页异步列表（`composables/usePagedList.uts`）· **行为级**测试（移动端 ADR-0027）。
 *
 * 为什么是行为而不是源码文本：本模块的判据全是**跑起来才看得见**的东西 ——
 * 旧代响应到达时会不会写状态、空批次会不会把 `hasMore` 兜住、失败时 `items` 有没有被清空、
 * 在飞时重入会不会真的少发一次请求。源码文本断言抓不到其中任何一条（这正是
 * `docs/agents/guards.md` 写的「接线守护在算法被改坏但字面量还在时不会红」）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 把真件当 JS 执行；注入的只有 Vue 反应式替身
 * （同 `utils/forumImagePickerBehavior.test.js:33-36` 的先例 —— 被测的是编排，不是 Vue 的调度）。
 *
 * **成对取证**（ADR-0008 ③ 判据③：只跑通过的那一次不算验收）：末组用注入变异的副本证明判据有牙 ——
 *   ① 拆掉代际守卫 ⇒ 旧代响应会覆盖新代数据；
 *   ② 拆掉空批次兜底 ⇒ 空批次会让 `hasMore` 留在 `true`；
 *   ③ 拆掉重入守卫里的 `loading` 那半 ⇒ 在飞时会多发一次请求。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const PAGED_UTS = path.join(__dirname, '..', 'composables', 'usePagedList.uts');

/** Vue 反应式的最小替身（本仓 node 侧解析不到 `vue`：它是 uni-app-x 编译期依赖） */
const vueStub = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/** 等微任务链跑完（模块用 `.then/.catch`，测试要等它们落地） */
const flush = () => new Promise((resolve) => setImmediate(resolve));

/**
 * 组装一个列表实例 + 取数记录。
 * 每次 `fetchPage` 都返回**可控的 deferred**：测试自己决定哪一代、第几页先落地 ——
 * 「旧代迟到」这类时序判据只有可控时序才测得出来。
 * @param file 被执行的 `.uts` 路径（成对取证时传变异副本）
 */
function harnessAt(file) {
  const calls = [];
  const fetchPage = (page) => {
    let resolveFn = null;
    let rejectFn = null;
    const promise = new Promise((resolve, reject) => { resolveFn = resolve; rejectFn = reject; });
    calls.push({ page, resolve: resolveFn, reject: rejectFn });
    return promise;
  };
  const mod = loadUts(file, { ...vueStub() });
  return { api: mod.usePagedList(fetchPage), calls };
}

const harness = () => harnessAt(PAGED_UTS);

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

// ===== ① 首屏与追加 =====

describe('首屏：`loadFirst` 取第 1 页并**替换**列表', () => {
  it('成功 ⇒ items 替换、hasMore 取批次、loading 归假、failed 归假', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    expect([calls.length, calls[0].page]).toEqual([1, 1]); // 页码从 1 起
    expect(api.loading.value).toBe(true);
    calls[0].resolve({ items: ['a', 'b'], hasMore: true });
    await flush();
    expect(api.items.value).toEqual(['a', 'b']);
    expect(api.hasMore.value).toBe(true);
    expect(api.loading.value).toBe(false);
    expect(api.failed.value).toBe(false);
    expect(api.isEmpty.value).toBe(false);
  });

  it('再次 `loadFirst`（刷新）⇒ 替换而不是拼接，页码回到 1', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a', 'b'], hasMore: true });
    await flush();
    api.loadFirst();
    expect(calls[1].page).toBe(1);
    calls[1].resolve({ items: ['c'], hasMore: false });
    await flush();
    expect(api.items.value).toEqual(['c']);
    expect(api.hasMore.value).toBe(false);
  });
});

describe('追加：`onLoadMore` 取下一页并**拼接**', () => {
  it('第二次取数收到页码 2，items 拼接、页码随之前进', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a'], hasMore: true });
    await flush();
    api.onLoadMore();
    expect(calls[1].page).toBe(2);
    calls[1].resolve({ items: ['b'], hasMore: false });
    await flush();
    expect(api.items.value).toEqual(['a', 'b']);
    expect(api.hasMore.value).toBe(false);
  });

  it('在飞时重入 `onLoadMore` ⇒ 只发一次请求（重入守卫）', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a'], hasMore: true });
    await flush();
    api.onLoadMore();
    api.onLoadMore();
    api.onLoadMore();
    expect(calls.length).toBe(2); // 首屏 1 次 + 追加 1 次，三次调用只落地一次
  });

  it('`hasMore` 为假时 `onLoadMore` 直接返回，不发请求', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a'], hasMore: false });
    await flush();
    api.onLoadMore();
    expect(calls.length).toBe(1);
  });
});

// ===== ② 空批次兜底 =====

describe('空批次兜底：批次为空 ⇒ `hasMore` 强制为假（调用方漏判不会变成死循环）', () => {
  it('批次 `items` 为空且调用方给了 `hasMore: true` ⇒ 模块仍判到底', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: [], hasMore: true });
    await flush();
    expect(api.hasMore.value).toBe(false);
    expect(api.items.value).toEqual([]);
    expect(api.isEmpty.value).toBe(true); // 成功且空 ⇒ 空态成立
  });
});

// ===== ③ 失败：保留数据、与空态可分辨 =====

describe('失败：翻 `failed`、**不清空**已取到的 items、不动 hasMore', () => {
  it('追加失败 ⇒ 已取到的数据仍在，页面据此渲染错误态 + 重试', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a'], hasMore: true });
    await flush();
    api.onLoadMore();
    calls[1].reject(new Error('boom'));
    await flush();
    expect(api.failed.value).toBe(true);
    expect(api.loading.value).toBe(false);
    expect(api.items.value).toEqual(['a']); // 判据③：不清空
    expect(api.hasMore.value).toBe(true);   // 判据③：不动终止条件（重试还能继续）
    expect(api.isEmpty.value).toBe(false);  // 失败 ≠ 空：两态可分
  });

  it('首屏就失败 ⇒ items 为空但 `isEmpty` 仍为假（空态与失败态不分家就会误报「暂无数据」）', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].reject(new Error('boom'));
    await flush();
    expect(api.failed.value).toBe(true);
    expect(api.items.value).toEqual([]);
    expect(api.isEmpty.value).toBe(false);
  });

  it('失败后 `loadFirst`（重试）成功 ⇒ `failed` 归假', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].reject(new Error('boom'));
    await flush();
    api.loadFirst();
    calls[1].resolve({ items: ['z'], hasMore: false });
    await flush();
    expect(api.failed.value).toBe(false);
    expect(api.items.value).toEqual(['z']);
  });
});

// ===== ① 代际守卫（本仓此前只有两页手写过） =====

describe('代际守卫：过期响应不得写任何状态', () => {
  it('旧代迟到 ⇒ 不得覆盖新代数据（先慢后快）', async () => {
    const { api, calls } = harness();
    api.loadFirst();                       // 第 1 代（慢，稍后才回）
    api.loadFirst();                       // 第 2 代（快，先回）
    expect(calls.length).toBe(2);
    calls[1].resolve({ items: ['new'], hasMore: false });
    await flush();
    calls[0].resolve({ items: ['stale'], hasMore: true });   // 旧代此刻才到
    await flush();
    expect(api.items.value).toEqual(['new']);
    expect(api.hasMore.value).toBe(false);
    expect(api.loading.value).toBe(false);
  });

  it('追加在飞时切筛选（`loadFirst`）⇒ 旧追加响应作废，不得被拼进来', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    calls[0].resolve({ items: ['a'], hasMore: true });
    await flush();
    api.onLoadMore();                      // 第 2 页在飞
    api.loadFirst();                       // 切筛选：换代
    calls[2].resolve({ items: ['f'], hasMore: false });
    await flush();
    calls[1].resolve({ items: ['leak'], hasMore: true });    // 旧追加此刻才到
    await flush();
    expect(api.items.value).toEqual(['f']); // 旧代的追加不得混进新列表
  });

  it('旧代失败也不得翻 `failed`（否则新代成功会被旧代的失败标红）', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    api.loadFirst();
    calls[1].resolve({ items: ['ok'], hasMore: false });
    await flush();
    calls[0].reject(new Error('stale boom'));
    await flush();
    expect(api.failed.value).toBe(false);
    expect(api.items.value).toEqual(['ok']);
  });
});

// ===== 成对取证（必红）：判据在坏实现上确实会红 =====

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红（对照）：真源上旧代覆盖不了新代、空批次兜得住、重入只发一次', async () => {
    const { api, calls } = harness();
    api.loadFirst();
    api.loadFirst();
    calls[1].resolve({ items: ['new'], hasMore: false });
    await flush();
    calls[0].resolve({ items: ['stale'], hasMore: true });
    await flush();
    expect(api.items.value).toEqual(['new']);
    const empty = harness();
    empty.api.loadFirst();
    empty.calls[0].resolve({ items: [], hasMore: true });
    await flush();
    expect(empty.api.hasMore.value).toBe(false);
  });

  it('必红 · 拆掉 `loadFirst` 里的代际守卫 ⇒ 旧代响应会覆盖新代（①「旧代迟到不得覆盖」判红）', async () => {
    const broken = harnessAt(mutatedCopy(PAGED_UTS, [[
      'if (mine != generation) return\n                items.value = batch.items',
      'items.value = batch.items',
    ]], 'paged-list-stale-'));
    broken.api.loadFirst();
    broken.api.loadFirst();
    broken.calls[1].resolve({ items: ['new'], hasMore: false });
    await flush();
    broken.calls[0].resolve({ items: ['stale'], hasMore: true });
    await flush();
    // 坏实现真的把旧代写了进去 ⇒ ① 的 `items == ['new']` 断言必然判红
    expect(broken.api.items.value).toEqual(['stale']);
  });

  it('必红 · 拆掉空批次兜底 ⇒ 空批次的 `hasMore` 留在真（②「空批次判到底」判红）', async () => {
    const broken = harnessAt(mutatedCopy(PAGED_UTS, [[
      'return batch.hasMore && batch.items.length > 0',
      'return batch.hasMore',
    ]], 'paged-list-empty-'));
    broken.api.loadFirst();
    broken.calls[0].resolve({ items: [], hasMore: true });
    await flush();
    expect(broken.api.hasMore.value).toBe(true); // 坏实现真的留在了真 ⇒ ② 的断言判红
  });

  it('必红 · 拆掉重入守卫里的 `loading` 那一半 ⇒ 在飞时会多发一次请求（重入判据判红）', async () => {
    const broken = harnessAt(mutatedCopy(PAGED_UTS, [[
      'if (loading.value || !hasMore.value) return',
      'if (!hasMore.value) return',
    ]], 'paged-list-reentry-'));
    broken.api.loadFirst();
    broken.calls[0].resolve({ items: ['a'], hasMore: true });
    await flush();
    broken.api.onLoadMore();
    broken.api.onLoadMore();
    broken.api.onLoadMore();
    expect(broken.calls.length).toBe(4); // 首屏 1 + 三次追加都发出去了 ⇒「只发一次」判红
  });
});
