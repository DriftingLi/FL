/**
 * 回复点赞 · **行为级**测试（#2332 首批 / #1713 的回归钉子）
 *
 * 为什么这里必须是行为而不是源码文本：本域栽过一次 —— #1713 那条测试把**错误的存储键**
 * （\`'forum_reply_like_'\`）断言成了规范，于是「本地态优先于服务端」这个 bug 被绿锁住，
 * 后来删本地态时反倒要改测试。源码文本锁抓不到这一类：字面量还在、行为已经错了。
 *
 * 缝：\`utils/utsHarness.js\` 的 \`loadUts\` 把 \`composables/useTopicDetail.uts\` 当 JS 真跑。
 * 票面原来写「.uts 在 node 里不可 import ⇒ 删掉 + 登记真机冒烟」—— 那一半实测不成立
 * （去类型后即可执行，注入 14 条 import 绑定 + Vue 反应式替身即可）；**只有 .uvue 模板/像素**真的
 * 只能归 ①a 发版冒烟。故本套件承载行为，\`utils/forumContract.test.js\` 只留调用点接线锁。
 *
 * 判据只问一句：**把实现改坏，这条会红吗？** 末组用注入变异的副本证明这一点（成对取证，
 * 同 \`utils/pagedListBehavior.test.js\` 先例）：
 *   ① 把 \`isReplyLiked\` 改回「本地存储优先」⇒ #1713 回归断言必红；
 *   ② 拆掉失败回滚 ⇒ 回滚断言必红；
 *   ③ 拆掉 \`like_count\` 回落 ⇒ 计数回落断言必红。
 *
 * 与接线腿的分工（别混）：本套件自己搭依赖，**接线断了它照样绿** —— 「壳层还在调
 * \`onToggleReplyLike\`」由 \`utils/forumContract.test.js\` 的接线锁守。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const DETAIL_UTS = path.join(__dirname, '..', 'composables', 'useTopicDetail.uts');

/** Vue 反应式的最小替身（node 侧解析不到 \`vue\`：它是 uni-app-x 编译期依赖） */
const vueStub = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

const flush = () => new Promise((resolve) => setImmediate(resolve));

/** 可自己决定何时落地的 promise（乐观态只有在接口返回**之前**才看得见） */
const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
};

/**
 * 组装一个详情实例 + 调用记录。
 * @param file 被执行的 \`.uts\` 路径（成对取证时传变异副本）
 * @param storage 本地存储替身内容（#1713 的「残留键不得复活」用它）
 */
function harnessAt(file, storage) {
  const calls = { like: [], unlike: [], toast: [] };
  const likeReplyApi = (id) => {
    const d = deferred();
    calls.like.push({ id, resolve: d.resolve, reject: d.reject });
    return d.promise;
  };
  const unlikeReplyApi = (id) => {
    const d = deferred();
    calls.unlike.push({ id, resolve: d.resolve, reject: d.reject });
    return d.promise;
  };
  const store = storage || {};
  const uni = {
    getStorageSync: (k) => (k in store ? store[k] : ''),
    setStorageSync: (k, v) => { store[k] = v; },
    removeStorageSync: (k) => { delete store[k]; },
    showToast: (o) => { calls.toast.push(o); },
  };
  const notUsed = () => { throw new Error('本套件不该走到这条依赖'); };
  const mod = loadUts(file, {
    ...vueStub(),
    uni,
    useAuthStore: () => ({ user: { value: null }, token: { value: '' } }),
    goBack: () => {},
    getForumTopicDetailApi: notUsed,
    deleteForumTopicApi: notUsed,
    deleteForumReplyApi: notUsed,
    likeTopicApi: notUsed,
    unlikeTopicApi: notUsed,
    acceptReplyApi: notUsed,
    cancelAcceptApi: notUsed,
    checkFavoriteApi: notUsed,
    addFavoriteApi: notUsed,
    removeFavoriteApi: notUsed,
    likeReplyApi,
    unlikeReplyApi,
  });
  return { api: mod.useTopicDetail(), calls };
}

const harness = (storage) => harnessAt(DETAIL_UTS, storage);

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

// ===== ① 判据读后端权威字段 =====

describe('判定口径：已赞与否读后端权威字段，不读任何本地态', () => {
  it('服务端下发 liked_by_me ⇒ 已赞；两个别名任一为真都算已赞', () => {
    const { api } = harness();
    expect(api.isReplyLiked({ id: 1, liked: true, liked_by_me: false })).toBe(true);
    expect(api.isReplyLiked({ id: 1, liked: false, liked_by_me: true })).toBe(true);
    expect(api.isReplyLiked({ id: 1, liked: false, liked_by_me: false })).toBe(false);
  });

  it('#1713 回归：本地存储残留 forum_reply_like_<id> 键**不影响**判定（错误存储键不得复活成口径）', () => {
    // #1713 的病根就是这条键被当成真源：谁再把它读回来，这里立刻红
    const { api } = harness({ 'forum_reply_like_7': true, 'forum_reply_like_8': 1 });
    expect(api.isReplyLiked({ id: 7, liked: false, liked_by_me: false })).toBe(false);
    expect(api.isReplyLiked({ id: 8, liked: false })).toBe(false);
  });

  it('计数以 likes_count 为准；为 0 时才回落到 like_count（旧字段别名）', () => {
    const { api } = harness();
    expect(api.getReplyLikesCount({ id: 1, likes_count: 5, like_count: 9 })).toBe(5);
    expect(api.getReplyLikesCount({ id: 1, likes_count: 0, like_count: 2 })).toBe(2);
    expect(api.getReplyLikesCount({ id: 1, likes_count: 0, like_count: 0 })).toBe(0);
  });
});

// ===== ② 乐观更新 / 服务端对账 / 失败回滚 =====

describe('点赞交互：乐观生效、以服务端返回值对账、失败就地回滚', () => {
  it('点赞 ⇒ 接口返回前就已显示 +1；返回后以服务端 likes_count 覆盖（不是本地自增数）', async () => {
    const { api, calls } = harness();
    const reply = { id: 7, liked: false, liked_by_me: false, likes_count: 3 };
    const pending = api.onToggleReplyLike(reply);
    expect([reply.liked, reply.likes_count]).toEqual([true, 4]); // 乐观：不等接口
    expect(calls.like.length).toBe(1);
    expect(calls.like[0].id).toBe(7);
    calls.like[0].resolve({ liked: true, likes_count: 9 });
    await pending;
    expect([reply.liked, reply.likes_count]).toEqual([true, 9]); // 服务端对账
  });

  it('取消赞 ⇒ 立即 -1 且不为负；返回后以服务端返回值覆盖', async () => {
    const { api, calls } = harness();
    const reply = { id: 9, liked: true, likes_count: 1 };
    const pending = api.onToggleReplyLike(reply);
    expect([reply.liked, reply.likes_count]).toEqual([false, 0]); // 下限 0，不是 -1
    calls.unlike[0].resolve({ liked: false, likes_count: 0 });
    await pending;
    expect([reply.liked, reply.likes_count]).toEqual([false, 0]);
  });

  it('接口失败 ⇒ 已赞态与计数一起回滚，并把后端 message 弹一次 toast', async () => {
    const { api, calls } = harness();
    const reply = { id: 7, liked: false, likes_count: 3 };
    const pending = api.onToggleReplyLike(reply);
    expect(reply.liked).toBe(true);
    calls.like[0].reject(new Error('boom'));
    await pending;
    expect([reply.liked, reply.likes_count]).toEqual([false, 3]); // 回滚到操作前
    expect(calls.toast.length).toBe(1);
    expect(calls.toast[0].title).toBe('boom');
  });
});

// ===== 成对取证（必红）：判据在坏实现上确实会红 =====

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红（对照）：真源上本地键无影响、失败回滚干净、计数回落生效', async () => {
    const { api, calls } = harness({ 'forum_reply_like_7': true });
    expect(api.isReplyLiked({ id: 7, liked: false })).toBe(false);
    const reply = { id: 7, liked: false, likes_count: 3 };
    const pending = api.onToggleReplyLike(reply);
    calls.like[0].reject(new Error('boom'));
    await pending;
    expect([reply.liked, reply.likes_count]).toEqual([false, 3]);
    expect(api.getReplyLikesCount({ id: 1, likes_count: 0, like_count: 2 })).toBe(2);
  });

  it('必红 · 把 isReplyLiked 改回「本地存储优先」⇒ #1713 回归断言必红', () => {
    const broken = harnessAt(mutatedCopy(DETAIL_UTS, [[
      "        if (reply.liked == true) return true\n        if (reply.liked_by_me == true) return true",
      "        if (uni.getStorageSync('forum_reply_like_' + reply.id) == true) return true\n        if (reply.liked == true) return true\n        if (reply.liked_by_me == true) return true",
    ]], 'forum-reply-like-local-'), { 'forum_reply_like_7': true });
    // 坏实现真的把残留键读成了已赞 ⇒ 「本地键不得影响判定」那条必然判红
    expect(broken.api.isReplyLiked({ id: 7, liked: false, liked_by_me: false })).toBe(true);
  });

  it('必红 · 拆掉失败回滚 ⇒ 失败后仍显示已赞且计数停在 +1（回滚断言必红）', async () => {
    const broken = harnessAt(mutatedCopy(DETAIL_UTS, [[
      '        } catch (e) {\n            reply.liked = wasLiked\n            reply.likes_count = oldCount\n',
      '        } catch (e) {\n',
    ]], 'forum-reply-like-rollback-'));
    const reply = { id: 7, liked: false, likes_count: 3 };
    const pending = broken.api.onToggleReplyLike(reply);
    broken.calls.like[0].reject(new Error('boom'));
    await pending;
    // 坏实现把乐观态留住了 ⇒ 「回滚到 false / 3」必然判红
    expect([reply.liked, reply.likes_count]).toEqual([true, 4]);
  });

  it('必红 · 拆掉 like_count 回落 ⇒ 旧字段口径下的计数丢失（计数回落断言必红）', () => {
    const broken = harnessAt(mutatedCopy(DETAIL_UTS, [[
      '        const lc = reply.like_count\n        if (lc != null && (lc as number) > 0) return lc as number\n',
      '',
    ]], 'forum-reply-like-count-'));
    // 坏实现对 like_count 视而不见 ⇒ 「为 0 时回落到 2」必然判红
    expect(broken.api.getReplyLikesCount({ id: 1, likes_count: 0, like_count: 2 })).toBe(0);
  });
});
