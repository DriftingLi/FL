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
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');
const RECRUITER_LOGIN_UVUE = path.join(ROOT, 'pages', 'recruiter', 'login.uvue');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';
const KEY_CREDENTIALS = 'auth_secure_credentials';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const ROLE_STUDENT = 'student';
const ROLE_RECRUITER = 'recruiter';

const EXPIRED = 'expired-access-token';
const FRESH = 'fresh-access-token';
const REFRESH_1 = 'refresh-token-1';
const REFRESH_2 = 'refresh-token-2';
const RECRUITER_ACCESS = 'recruiter-access-token';

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
  // 身份角色真源（#1194）：`request.uts` 的 401 出口与 `auth.uts` 都从它取「当前是谁」。
  // 用**真的** authRole.uts 而不是注桩 —— 否则「角色分支」这条判据就退化成了接线断言。
  const authRole = loadUts(AUTH_ROLE_UTS, {
    getStorage: storage.getStorage,
    setStorage: storage.setStorage,
    removeStorage: storage.removeStorage,
    STORAGE_KEY_ACTIVE_ROLE: KEY_ACTIVE_ROLE,
    ACTIVE_ROLE_STUDENT: ROLE_STUDENT,
    ACTIVE_ROLE_RECRUITER: ROLE_RECRUITER,
  });
  const request = loadUts(REQUEST_UTS, {
    gateRefresh: gate.gateRefresh,
    API_BASE_URL: 'http://127.0.0.1:8080',
    REQUEST_TIMEOUT: 15000,
    ENABLE_DEBUG_LOG: false,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    STORAGE_KEY_USER: KEY_USER,
    isRecruiterActive: authRole.isRecruiterActive,
    getStorage: storage.getStorage,
    removeStorage: storage.removeStorage,
    isContentUri: () => false,
    buildTempFilePath: () => '',
    UPLOAD_TMP_DIR: 'upload-tmp',
    uni,
    getCurrentPages: () => pages,
  });

  const refreshCalls = [];
  const recruiterLoginCalls = [];
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
    // 招聘者登录（#1194）：记录调用并返回与学员登录同构的响应
    recruiterLoginApi: (params) => {
      recruiterLoginCalls.push(params);
      return Promise.resolve({
        token: RECRUITER_ACCESS,
        refresh_token: '',
        user: { user_id: 11, username: params.username, name: params.username, role: ROLE_RECRUITER },
      });
    },
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
    setActiveRole: authRole.setActiveRole,
    clearActiveRole: authRole.clearActiveRole,
    errMsg: (_e, fallback) => fallback,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    STORAGE_KEY_REFRESH_TOKEN: KEY_REFRESH,
    STORAGE_KEY_USER: KEY_USER,
    STORAGE_KEY_LOGIN_PROVIDER: KEY_PROVIDER,
    STORAGE_KEY_CREDENTIALS: KEY_CREDENTIALS,
    ACTIVE_ROLE_RECRUITER: ROLE_RECRUITER,
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
    storage,
    authRole,
    request,
    refreshCalls,
    recruiterLoginCalls,
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

describe('登出后遗留 refresh_token 不得悄悄复活会话', () => {
  test('I5: 内存已归零时，无凭据的 401 不刷新、也不写回 token/user', async () => {
    const app = buildApp({ refreshOk: true }); // refresh_token 仍有效 —— 故意诱使「复活」

    // 造出「401 登出 + 登录页 onLoad 已归零内存」的现场：
    // handleUnauthorized 只清 access 侧凭据（refresh_token 按 ADR-0004 保留）
    app.uni.removeStorageSync(KEY_TOKEN);
    app.uni.removeStorageSync(KEY_USER);
    expect(app.store.restoreFromStorage()).toBe(false); // 登录页 onLoad
    expect(app.store.isLoggedIn.value).toBe(false);
    expect(app.store.user.value).toBe(null);

    // 此刻来一个**无凭据**的受保护请求（`pages/resume/resume.uvue` 这类 onLoad 无守卫的页面会发）
    const p = app.request.get('/api/courses');
    expect(app.stormAll()).toBe(1); // 无凭据 ⇒ 401
    await expect(p).rejects.toThrow('登录已过期');

    // 关键：rt 有效也不许用它刷新，更不许把 token/user 写回来
    // （写回 user 只会是 JSON.stringify(null) = "null" 的脏值；Kotlin 侧则是 `user.value!` 的 !! NPE）
    expect(app.refreshCalls.length).toBe(0);
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
    expect(app.uni.kv.has(KEY_USER)).toBe(false);
    expect(app.store.isLoggedIn.value).toBe(false);
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

/**
 * #1194 招聘者身份面（移动端 ADR-0022 ④ 步骤 P1 / ⑥ 验收标准）
 *
 * 这些用例跑的是**真的** `stores/auth.uts` + `api/request.uts` + `utils/authRole.uts`：
 * 身份互斥清槽有没有清干净、401 出口到底跳去了哪一页、招聘者态有没有去调 `/auth/refresh`，
 * 都是行为事实而不是源码文本。学员路径的既有断言（上面 I2/I3/I4）**一行未改** —— 那正是
 * ADR-0022 ⑤「学员路径行为逐条不变」的回归面。
 */
describe('招聘者身份面（#1194）：互斥清槽与角色化 401', () => {
  test('R1: 招聘者登入后学员三键与安全凭据为空，auth_active_role = recruiter', async () => {
    const app = buildApp({ refreshOk: true });
    // 先把「上一重身份」摆满：学员凭据（refresh_token 已由 buildApp 的 setAuthData 写入）
    // + 生物识别安全凭据槽
    app.storage.setStorage(KEY_CREDENTIALS, '{"u":"u7","p":"p7","has":true}');
    expect(app.uni.kv.has(KEY_CREDENTIALS)).toBe(true);
    expect(app.storage.getStorage(KEY_REFRESH)).toBe(REFRESH_1);

    await app.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });

    // 登录请求真的打到了招聘者登录入口（不是一个被 mock 掉的空转）
    expect(app.recruiterLoginCalls.length).toBe(1);
    expect(app.recruiterLoginCalls[0].username).toBe('hr001');

    // 学员三键为空：token 被招聘者凭据顶替，refresh_token / user 必须清掉
    expect(app.storage.getStorage(KEY_REFRESH)).toBe('');
    expect(app.uni.kv.has(KEY_REFRESH)).toBe(false);
    expect(app.storage.getStorage(KEY_USER)).not.toContain('"role":"student"');
    // 安全凭据槽为空（ADR-0021 ②：复用单槽 ⇒ 不能留着另一方的凭据）
    expect(app.uni.kv.has(KEY_CREDENTIALS)).toBe(false);
    // 角色标记 = recruiter，且招聘者凭据已落到共用槽里
    expect(app.storage.getStorage(KEY_ACTIVE_ROLE)).toBe(ROLE_RECRUITER);
    expect(app.storage.getStorage(KEY_TOKEN)).toBe(RECRUITER_ACCESS);
    expect(app.authRole.isRecruiterActive()).toBe(true);
    // 内存态与 storage 一致（登录页 onLoad 的 isLoggedIn 守卫据此放行）
    expect(app.store.isLoggedIn.value).toBe(true);
  });

  test('R2: 学员身份下招聘者态不可恢复（无角色标记 ⇒ 判定回落学员；清槽后无 refresh_token 可续）', async () => {
    const app = buildApp({ refreshOk: true });
    await app.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });
    expect(app.authRole.isRecruiterActive()).toBe(true);
    // 招聘者态下 refresh_token 槽是空的 —— 这是「不调 /auth/refresh」的结构前提
    expect(app.storage.getStorage(KEY_REFRESH)).toBe('');

    // 回到学员缺省态（无角色标记）
    app.authRole.clearActiveRole();
    expect(app.authRole.isRecruiterActive()).toBe(false);
    expect(app.authRole.getActiveRole()).toBe(ROLE_STUDENT);

    // 脏值兜底：角色标记被写成非合法值时，判定必须是学员（宁可多跳一次学员登录页）
    app.storage.setStorage(KEY_ACTIVE_ROLE, 'admin');
    expect(app.authRole.getActiveRole()).toBe(ROLE_STUDENT);
    expect(app.authRole.isRecruiterActive()).toBe(false);
  });

  test('R3: 401 学员态 → /pages/login/login，且只 reLaunch 一次（既有学员语义不变）', async () => {
    const app = buildApp({ refreshOk: false });
    expect(app.storage.getStorage(KEY_ACTIVE_ROLE)).toBe(''); // 学员是缺省身份

    const p = app.request.get('/auth/me');
    app.stormAll();
    // 必须消费 reject：否则是未处理拒绝（既有 I2/I3/I4 用 Promise.allSettled 同理）
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/login/login');
  });

  test('R4: 401 招聘者态 → /pages/recruiter/login，只 reLaunch 一次，且**不调** /auth/refresh', async () => {
    const app = buildApp({ refreshOk: true }); // refresh 后端可用 —— 故意诱使「拿招聘者 token 去刷新」
    await app.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });
    // 制造「招聘者 access token 已失效」的现场（结构前提：招聘者态没有 refresh_token）
    expect(app.storage.getStorage(KEY_REFRESH)).toBe('');
    app.storage.setStorage(KEY_TOKEN, EXPIRED);

    const p = app.request.get('/api/recruit/jobs');
    expect(app.stormAll()).toBe(1);
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    // 跳到招聘者登录页，且**恰好一次**
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/recruiter/login');
    // 招聘者态**禁用刷新链**：无 refresh token ⇒ 不得让闸门拿着招聘者 token 去调 /auth/refresh
    expect(app.refreshCalls.length).toBe(0);
  });
});

/**
 * #1194 收口：**招聘者登录页的离开路径**（真机复现的死胡同 + 「取消即登出」语义纠正）
 *
 * 现测事实（真机，2026-09-20）：招聘者页守卫 `utils/recruitGuard.uts`（P2 分支）与
 * `api/request.uts` 的 401 角色化出口都用 **`uni.reLaunch`** 进 `/pages/recruiter/login`
 * ⇒ **页面栈被清空**，本页原有的 `uni.navigateBack()` 与「取消」分支都是死胡同，
 * 人被困在本页（`pages/recruiter/contacts` → `/pages/recruiter/login` 反复出现）。
 *
 * 出口只做对了一半还不够 —— 另一半是**角色标记**：`auth_active_role` 只在
 * `loginAsRecruiter()` 里被写，学员侧任何登录路径都不写它 ⇒ 不清槽就回学员端，下一次 401
 * 仍按角色分支把人送回招聘者登录页（**来回弹**）。R5 拿**真的** `request.uts` 把这条链路走一遍；
 * R7 是这条判断的**判别力对照**（不清槽 ⇒ 必红），不是为了凑绿。
 *
 * ⚠️ **反向的错法（本组用例的第二条不变量）**：把**所有**出口都接到「清槽 + reLaunch」上，
 * 就把「点到招聘者入口又点取消」变成了「把当前学员登出」—— 本页 P1 的原始语义是
 * 「取消 ⇒ `navigateBack()`，什么都不动」。所以出口必须**按页面栈分流**，且清槽的判据是
 * **「当前身份是招聘者」**而不是「栈空」：栈空但身份是**学员**时（守卫把一个在册学员从招聘者页
 * 弹到了这里）清槽就是**误登出**。R6 / R6b / R8 锁出口形状与分流接线，
 * R9 在**真 store** 上锁「学员会话只在招聘者态才被清」这条行为。
 *
 * 分类（docs/agents/guards.md）：R6 / R6b / R8 是**接线守护**（读 `.uvue` 源文本；判别力靠注入取得，
 * 读数见 PR 正文），它们的**行为兜底** = R5（清槽后 401 落回学员登录页）/ R7（不清槽 ⇒ 弹回）
 * / R9（不清槽 ⇒ 学员三键与安全凭据逐键保留）。
 */
describe('#1194 收口：招聘者登录页的离开路径', () => {
  const EXIT_FN = 'leaveRecruiterLogin';

  /** 取出分流函数体（缩进锚定：函数闭合括号是 4 空格，体内最内层是 8 空格） */
  function exitFnBody() {
    const src = readText(RECRUITER_LOGIN_UVUE);
    const hit = src.match(new RegExp(`function ${EXIT_FN}\\(\\)[\\s\\S]*?\\n {4}\\}`));
    expect(hit).not.toBeNull();
    return hit[0];
  }

  test('R6: 出口存在，且是学员登录页招聘者入口的镜像（纯文字 link，无说明性文案）', () => {
    const src = readText(RECRUITER_LOGIN_UVUE);

    expect(src).toContain('class="student-exit"');
    expect(src).toContain('class="student-exit-link"');
    expect(src).toContain(`@click="${EXIT_FN}"`);

    // 复用既有 link 外观（与 `pages/login/login.uvue` 的 `.back-password-link` 逐属性同形）
    expect(src).toContain('border-bottom-width: 1rpx;');
    expect(src).toContain('border-bottom-color: #2979ff;');

    // 无说明性 hint：出口块里只有那一行文字（多一段说明文案 = 违反「无说明性文案」）
    const block = src.match(/<view class="student-exit"[\s\S]*?<\/view>/);
    expect(block).not.toBeNull();
    expect((block[0].match(/<text/g) || []).length).toBe(1);
    expect(block[0]).toContain('返回学员登录');
  });

  test('R6b: 三个出口（左上「返回」/ 模态「取消」/ 底部文字）收敛到同一条分流', () => {
    const src = readText(RECRUITER_LOGIN_UVUE);

    // 模板两处 @click：左上「返回」与底部出口
    expect((src.match(new RegExp(`@click="${EXIT_FN}"`, 'g')) || []).length).toBe(2);
    // 模态「取消」分支也走同一条路；「确定」分支（退出当前账号）语义未动
    const modal = src.match(/uni\.showModal\(\{[\s\S]*?\n {12}\}\)/);
    expect(modal).not.toBeNull();
    expect(modal[0]).toContain('auth.clearAuthData()');
    expect(modal[0]).toContain(`${EXIT_FN}()`);
    // 旧的「所有出口都清槽」形态必须消失（它正是「取消即登出」的实现）
    expect(src).not.toContain('goStudentLogin');
  });

  test('R8: 按页面栈分流 —— 有页只回退（不清槽）；栈空且是招聘者态才清槽 + reLaunch', () => {
    const body = exitFnBody();

    // ① 判据是页面栈（与 `utils/navigation.uts` / `pages/search/search.uvue` 同形）
    expect(body).toContain('getCurrentPages()');
    expect(body).toContain('pages != null && pages.length > 1');

    const backAt = body.indexOf('uni.navigateBack()');
    const retAt = body.indexOf('return', backAt);
    const roleAt = body.indexOf('isRecruiterActive()');
    const clearAt = body.indexOf('clearIdentityForSwitch()');
    const relaunchAt = body.indexOf("uni.reLaunch({ url: '/pages/login/login' })");
    expect(backAt).toBeGreaterThan(-1);
    expect(retAt).toBeGreaterThan(backAt);
    // ② 清槽与 reLaunch 都在有页分支 `return` **之后** ⇒ 「有页 ⇒ 不清槽」由收口保证
    expect(roleAt).toBeGreaterThan(retAt);
    expect(clearAt).toBeGreaterThan(roleAt);
    // ③ 顺序：清槽必须**先于** reLaunch（反了就留下「新页 + 旧角色」，401 仍会把人送回本页）
    expect(relaunchAt).toBeGreaterThan(clearAt);

    // ④ 整页只有这一个返回出口与这一个「学员登录页」跳转，且都在分流函数体内
    const src = readText(RECRUITER_LOGIN_UVUE);
    expect((src.match(/uni\.navigateBack\(/g) || []).length).toBe(1);
    expect((src.match(/\/pages\/login\/login/g) || []).length).toBe(1);
    expect(src).not.toContain('function goBack');
  });

  test('R5: 出口清槽之后，后续 401 落回学员登录页（不再弹回招聘者登录页）', async () => {
    const app = buildApp({ refreshOk: true });
    await app.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });
    expect(app.authRole.isRecruiterActive()).toBe(true);

    // 复现「点出口」的**行为**（页面函数本身在 R6 里由源码断言钉住其形状）：
    app.store.clearIdentityForSwitch();
    app.uni.relaunches.length = 0;

    // 出口之后进学员登录页：角色标记必须已经不在
    expect(app.uni.kv.has(KEY_ACTIVE_ROLE)).toBe(false);
    expect(app.authRole.isRecruiterActive()).toBe(false);
    // 出口顺手清掉招聘者凭据（这正是清槽函数而非「只跳转」的理由）
    expect(app.storage.getStorage(KEY_TOKEN)).toBe('');

    // 学员登录页之后的第一次 401：必须落回**学员**登录页
    const p = app.request.get('/auth/me');
    app.stormAll();
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/login/login');
  });

  test('R7（判别力对照）: 出口**只跳转不清槽**时，下一次 401 会把人弹回招聘者登录页', async () => {
    const app = buildApp({ refreshOk: true });
    await app.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });

    // 反事实：只跳转、不清槽（等价于「出口写成 uni.reLaunch 而不调 clearIdentityForSwitch」）
    app.uni.relaunches.length = 0;
    expect(app.authRole.isRecruiterActive()).toBe(true);

    const p = app.request.get('/auth/me');
    app.stormAll();
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    // 这就是真机上看到的那件事：来回弹
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/recruiter/login');
  });

  /**
   * R9 是**行为守护**（真跑 `stores/auth.uts` + `utils/authRole.uts`）：它锁的是分流**两条分支的后果**，
   * 也就是接线守护 R8 的行为兜底 —— 「有页 ⇒ 不清槽」之所以必须成立，是因为清槽会让一个在册学员
   * 被登出（乙面就是那个伤害面）；而「招聘者态 ⇒ 清槽」之所以必须成立，是因为不清就会来回弹（R7）。
   */
  test('R9（行为）: 学员会话只在招聘者态才被清 —— 「取消/返回」不得把学员登出（逐键对照）', async () => {
    const KEYS = [KEY_TOKEN, KEY_REFRESH, KEY_USER, KEY_CREDENTIALS];

    // 甲：身份是**学员**（出口的清槽条件 `isRecruiterActive()` 为假）⇒ 离开路径只 reLaunch，
    //     认证槽逐键原样 —— 学员登录页的 onLoad 会因 isLoggedIn 直接把人送回工作区，不需要重新登录。
    const keep = buildApp({ refreshOk: true });
    keep.storage.setStorage(KEY_CREDENTIALS, '{"u":"u7","p":"p7","has":true}');
    const before = KEYS.map((k) => keep.storage.getStorage(k));
    expect(before.every((v) => v.length > 0)).toBe(true); // 前提：学员会话与安全凭据确实是「摆满」的
    expect(keep.authRole.isRecruiterActive()).toBe(false);

    keep.uni.reLaunch({ url: '/pages/login/login' }); // 学员态下出口只做这一步
    expect(KEYS.map((k) => keep.storage.getStorage(k))).toEqual(before);
    expect(keep.store.isLoggedIn.value).toBe(true);
    expect(keep.store.restoreFromStorage()).toBe(true); // 回来还能接着用，没有被登出

    // 乙（伤害面 / 判别力对照）：同一现场若**无条件**清槽 ⇒ 一个只是走错路的学员被登出。
    //     这就是被纠正掉的语义；也是「清槽条件必须收窄到 isRecruiterActive()」的反例。
    const harmed = buildApp({ refreshOk: true });
    harmed.storage.setStorage(KEY_CREDENTIALS, '{"u":"u7","p":"p7","has":true}');
    harmed.store.clearIdentityForSwitch();
    expect(KEYS.map((k) => harmed.storage.getStorage(k))).toEqual(['', '', '', '']);
    expect(harmed.store.isLoggedIn.value).toBe(false);
    expect(harmed.authRole.isRecruiterActive()).toBe(false);

    // 丙：身份是**招聘者**（栈空的死胡同）⇒ 这时才轮到清槽；角色标记必须一起没掉，
    //     否则下一次 401 仍按角色分支把人送回招聘者登录页（R5 / R7 已锁这条链路）。
    const recruiter = buildApp({ refreshOk: true });
    recruiter.storage.setStorage(KEY_CREDENTIALS, '{"u":"hr","p":"p","has":true}');
    await recruiter.store.loginAsRecruiter({ username: 'hr001', password: 'pass1234' });
    expect(recruiter.authRole.isRecruiterActive()).toBe(true);
    recruiter.store.clearIdentityForSwitch();
    expect(recruiter.uni.kv.has(KEY_ACTIVE_ROLE)).toBe(false);
    expect(recruiter.authRole.isRecruiterActive()).toBe(false);
    expect(recruiter.storage.getStorage(KEY_CREDENTIALS)).toBe('');
  });
});
