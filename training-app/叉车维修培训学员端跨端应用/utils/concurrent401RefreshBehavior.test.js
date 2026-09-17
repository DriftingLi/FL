/**
 * #1124 并发 401 行为级守护（**跑真的 `request.uts` / `auth.uts`**，不是断言源码文本）
 *
 * 契约来源：移动端 `docs/adr/0017-冷启动并发401误判登出缺口.md`
 *
 * 钉住的不变量（对应票面验收 1/3/5）：
 *   I1「另一次刷新正在进行」**绝不等于**「刷新失败」—— N 个并发 401 只发 **1** 次刷新，
 *      刷新成功后各自带新 token 重试并 resolve；
 *   I1b 刷新**已 settle** 之后才到达的迟到 401（它带的仍是那个已被替换的旧 token）
 *      同样**不得**产生第二次刷新（去重认 token，不认时间盒）；
 *   I2 只有刷新**真的**失败才登出：恰好**一次**跳转、storage 内 `auth_token`/`auth_user` 清空；
 *   I3 登出后登录页**不弹回**首页（`restoreFromStorage()` 归零内存登录态）；
 *   I4 登出只清 access 侧凭据：`auth_refresh_token` 保留（ADR-0004 生物识别快捷登录依赖它）。
 *
 * 为什么不用源码文本断言：那条路只能证明「某个字面量出现过」，证明不了
 * 「刷新被调用了几次」「并发请求最终是 resolve 还是 reject」。所以这里用
 * `utils/utsHarness.js` 把 `.uts` 真正执行起来，`uni.request` 由测试驱动。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';

const EXPIRED = 'expired-access-token';
const FRESH = 'fresh-access-token';
const REFRESH_1 = 'refresh-token-1';
const REFRESH_2 = 'refresh-token-2';

/** 假 `uni`：storage 用 Map 落地（语义与 `utils/storage.uts` 相同），request 只**捕获**不自动回应 */
function makeUni() {
  const kv = new Map();
  const requests = [];
  const toasts = [];
  const relaunches = [];
  return {
    kv,
    requests,
    toasts,
    relaunches,
    setStorageSync: (k, v) => { kv.set(k, v); },
    getStorageSync: (k) => (kv.has(k) ? kv.get(k) : ''),
    removeStorageSync: (k) => { kv.delete(k); },
    clearStorageSync: () => { kv.clear(); },
    getStorageInfoSync: () => ({ currentSize: 0 }),
    showToast: (o) => { toasts.push(o); },
    reLaunch: (o) => { relaunches.push(o); },
    showLoading: () => {},
    hideLoading: () => {},
    request: (o) => { requests.push(o); },
    getSystemInfoSync: () => ({ statusBarHeight: 20, windowHeight: 800 }),
  };
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * 装配一棵真实的依赖树：`refreshGate.uts` + `storage.uts` → `request.uts` →（回调注册）→ `auth.uts`
 * @param {{ refreshOk: boolean }} opts refreshOk=false 走「refresh_token 也无效」的真失败路径
 */
function buildApp(opts) {
  const refreshOk = opts.refreshOk;
  const uni = makeUni();
  const pages = [{ route: 'pages/dashboard/dashboard' }];

  const storage = loadUts(STORAGE_UTS, { uni });
  const gate = loadUts(GATE_UTS, {});
  const request = loadUts(REQUEST_UTS, {
    gateRefresh: gate.gateRefresh,
    API_BASE_URL: 'http://127.0.0.1:8080',
    REQUEST_TIMEOUT: 15000,
    ENABLE_DEBUG_LOG: false,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    getStorage: storage.getStorage,
    removeStorage: storage.removeStorage,
    isContentUri: () => false,
    buildTempFilePath: () => '',
    UPLOAD_TMP_DIR: 'upload-tmp',
    uni,
    getCurrentPages: () => pages,
  });

  const refreshCalls = [];
  const noopApi = () => Promise.resolve(null);
  const authMod = loadUts(AUTH_UTS, {
    ref: (v) => ({ value: v }),
    registerRefreshTokenHandler: request.registerRefreshTokenHandler,
    loginApi: noopApi,
    phoneLoginApi: noopApi,
    emailLoginApi: noopApi,
    mpWechatLoginApi: noopApi,
    logoutApi: () => Promise.resolve({}),
    getUserInfoApi: noopApi,
    refreshTokenApi: (rt) => {
      refreshCalls.push(rt);
      if (!refreshOk) return Promise.reject(new Error('refresh token 已失效'));
      return Promise.resolve({ token: FRESH, refresh_token: REFRESH_2 });
    },
    setStorage: storage.setStorage,
    setStorageJSON: storage.setStorageJSON,
    getStorage: storage.getStorage,
    getStorageJSON: storage.getStorageJSON,
    removeStorage: storage.removeStorage,
    updateSecureToken: () => {},
    errMsg: (_e, fallback) => fallback,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    STORAGE_KEY_REFRESH_TOKEN: KEY_REFRESH,
    STORAGE_KEY_USER: KEY_USER,
    STORAGE_KEY_LOGIN_PROVIDER: KEY_PROVIDER,
    uni,
  });

  const store = authMod.useAuthStore();
  // 正常登录一次：access 已过期（复现配方里的「签名无效」等价构造），refresh_token 仍有效
  store.setAuthData(EXPIRED, { user_id: 7, username: 'u7', name: '同学', role: 'student' }, REFRESH_1);

  /**
   * 回应**匹配**的挂起请求（按该请求实际带的 bearer 决定 401/200），返回回应条数。
   * 用谓词而不是「取前 N 条」，是为了能精确模拟「迟到的那个 401」——队列顺序不可靠。
   */
  function respondWhere(pred) {
    const hit = [];
    const rest = [];
    uni.requests.forEach((o) => { (pred(o) ? hit : rest).push(o); });
    uni.requests.length = 0;
    rest.forEach((o) => uni.requests.push(o));
    hit.forEach((o) => {
      if (o.header['Authorization'] === `Bearer ${FRESH}`) {
        o.success({ statusCode: 200, data: { code: 200, message: 'ok', data: { user_id: 7 } } });
        return;
      }
      o.success({ statusCode: 401, data: { code: 401, message: 'Token无效或已过期，请重新登录' } });
    });
    return hit.length;
  }

  return {
    uni,
    store,
    request,
    refreshCalls,
    respondWhere,
    /** 全部挂起请求一起回应（模拟「同一秒内一起到达的 401」，即一次风暴） */
    stormAll: () => respondWhere(() => true),
    /** 回应带新 token 的重试 */
    respondFresh: () => respondWhere((o) => o.header['Authorization'] === `Bearer ${FRESH}`),
  };
}

describe('refreshGate 单飞语义（api/refreshGate.uts）', () => {
  test('G1: N 个并发调用（同一 token）只跑一次 runner，且都拿到同一次的真实结果', async () => {
    const gate = loadUts(GATE_UTS, {});
    let calls = 0;
    const runner = () => {
      calls += 1;
      return sleep(30).then(() => true);
    };
    const results = await Promise.all([
      gate.gateRefresh(runner, 'old-token'),
      gate.gateRefresh(runner, 'old-token'),
      gate.gateRefresh(runner, 'old-token'),
      gate.gateRefresh(runner, 'old-token'),
    ]);
    expect(calls).toBe(1);
    expect(results).toEqual([true, true, true, true]);
  });

  test('G2: 真失败对所有并发者都是 false（不是「别人在刷」被折成失败）', async () => {
    const gate = loadUts(GATE_UTS, {});
    let calls = 0;
    const runner = () => {
      calls += 1;
      return sleep(20).then(() => false);
    };
    const results = await Promise.all([
      gate.gateRefresh(runner, 'old-token'),
      gate.gateRefresh(runner, 'old-token'),
    ]);
    expect(calls).toBe(1);
    expect(results).toEqual([false, false]);
  });

  test('G3: runner 抛错或拒绝折成 false（真失败），不向上冒泡', async () => {
    const gate = loadUts(GATE_UTS, {});
    const results = await Promise.all([
      gate.gateRefresh(() => Promise.reject(new Error('boom')), 't'),
      gate.gateRefresh(() => { throw new Error('sync boom'); }, 't'),
    ]);
    expect(results).toEqual([false, false]);
  });

  test('G4: 去重认 token 不认时间（刷新已 settle 后，带同一旧 token 的迟到 401 仍不重发）', async () => {
    const gate = loadUts(GATE_UTS, {});
    let calls = 0;
    const runner = () => {
      calls += 1;
      return Promise.resolve(true);
    };
    const first = await gate.gateRefresh(runner, 'old-token');
    // 关键：这里**不等待任何时间窗**。刷新已 settle，但迟到成员带的还是被替换掉的旧 token
    const late = await gate.gateRefresh(runner, 'old-token');
    expect([first, late]).toEqual([true, true]);
    expect(calls).toBe(1);
    // 换了 token（新的 access 又过期了）⇒ 必须重新真跑，不能被旧结果吞掉
    expect(await gate.gateRefresh(runner, 'new-token')).toBe(true);
    expect(calls).toBe(2);
    // 无凭据（''）不参与合并：登出后的请求必须能重新尝试刷新
    expect(await gate.gateRefresh(runner, '')).toBe(true);
    expect(calls).toBe(3);
  });

  test('G5: 迟到成员拿到的是那一次的真实结果（失败也一并复用，不重发、也不假装成功）', async () => {
    const gate = loadUts(GATE_UTS, {});
    let calls = 0;
    const runner = () => {
      calls += 1;
      return Promise.resolve(false);
    };
    expect(await gate.gateRefresh(runner, 'old-token')).toBe(false);
    expect(await gate.gateRefresh(runner, 'old-token')).toBe(false);
    expect(calls).toBe(1);
  });
});

describe('并发 401（刷新成功）—— 只发一次刷新，各自重试并 resolve', () => {
  test('I1: 4 个并发 401 只产生 1 次刷新，4 个请求全部 resolve 且不登出', async () => {
    const app = buildApp({ refreshOk: true });

    const inflight = [
      app.request.get('/auth/me'),
      app.request.get('/api/courses'),
      app.request.get('/api/me/credential'),
      app.request.get('/api/notifications'),
    ];
    expect(app.stormAll()).toBe(4); // 4 个 401「同秒」到达
    expect(app.refreshCalls.length).toBe(1); // 刷新**此刻**已在飞（第一个 401 发起，其余并入）

    await sleep(30);
    expect(app.respondFresh()).toBe(4); // 4 个带新 token 的重试

    const values = await Promise.all(inflight);
    expect(values.map((v) => v['user_id'])).toEqual([7, 7, 7, 7]);

    expect(app.refreshCalls.length).toBe(1); // 验收 5：并发场景刷新计数恒为 1（轮换不被重复消费）
    expect(app.uni.relaunches.length).toBe(0);
    expect(app.uni.toasts.filter((t) => String(t.title).indexOf('登录已过期') >= 0).length).toBe(0);
    expect(app.uni.kv.get(KEY_TOKEN)).toBe(FRESH);
  });

  test('I1b: 刷新 settle 之后才到的 401（同一次风暴的迟到成员）不产生第二次刷新', async () => {
    const app = buildApp({ refreshOk: true });

    // 两个请求同批发出 ⇒ 都带旧 token；但只让第一个的 401 先到，第二个的 401 故意压后
    const first = app.request.get('/auth/me');
    const straggler = app.request.get('/api/courses');

    expect(app.respondWhere((o) => o.url.endsWith('/auth/me'))).toBe(1);
    await sleep(30); // 刷新 settle，第一次请求已完成「刷新 + 重试」
    expect(app.respondFresh()).toBe(1);
    await expect(first).resolves.toEqual({ user_id: 7 });
    expect(app.refreshCalls.length).toBe(1);

    // 现在才把那个迟到的 401 放进来 —— 此刻刷新早已 settle（时间盒去重在这里会漏）
    expect(app.respondWhere((o) => o.url.endsWith('/api/courses'))).toBe(1);
    await sleep(30);
    expect(app.respondFresh()).toBe(1); // 它自己照常带新 token 重试并 resolve
    await expect(straggler).resolves.toEqual({ user_id: 7 });

    expect(app.refreshCalls.length).toBe(1); // 关键：没有第二次 POST /auth/refresh
    expect(app.uni.relaunches.length).toBe(0);
    expect(app.uni.toasts.length).toBe(0);
  });
});

describe('并发 401（refresh_token 也无效）—— 恰好一次登出，登录页不弹回首页', () => {
  test('I2/I3/I4: 一次跳转、access 侧凭据清空、refresh_token 保留、内存登录态归零', async () => {
    const app = buildApp({ refreshOk: false });

    const inflight = [
      app.request.get('/auth/me'),
      app.request.get('/api/courses'),
      app.request.get('/api/me/credential'),
      app.request.get('/api/notifications'),
    ];
    app.stormAll(); // 4 个 401「同秒」到达
    const settled = await Promise.allSettled(inflight);

    // 真失败：4 个请求都 reject（resolve/reject 可观测），且刷新只发了一次
    expect(settled.map((s) => s.status)).toEqual(['rejected', 'rejected', 'rejected', 'rejected']);
    expect(app.refreshCalls.length).toBe(1);
    expect(app.refreshCalls[0]).toBe(REFRESH_1);

    // 恰好一次登出跳转（isHandlingUnauthorized 去重，二阶循环的「来回跳」不复现）
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/login/login');

    // 登出只清 access 侧凭据；refresh_token 保留（ADR-0004 生物识别快捷登录依赖它）
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
    expect(app.uni.kv.has(KEY_USER)).toBe(false);
    expect(app.uni.kv.get(KEY_REFRESH)).toBe(REFRESH_1);

    // 登录页 onLoad：restoreFromStorage() 必须归零内存登录态，否则会 reLaunch 弹回首页
    expect(app.store.restoreFromStorage()).toBe(false);
    expect(app.store.isLoggedIn.value).toBe(false);
    expect(app.store.token.value).toBe('');
    expect(app.store.user.value).toBe(null);
    expect(app.uni.relaunches.length).toBe(1); // 没有再弹回首页
  });
});
