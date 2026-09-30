/**
 * #1404 401 顶出必须清证件域三键 —— **行为级**守护（真跑 `api/request.uts` → 回调接缝 → `stores/auth.uts`）
 *
 * 病根（票面）：`handleUnauthorized()` 只移 `auth_token` / `auth_user` 两条，不经 `stores/auth.uts`
 * 的任何清槽函数（循环依赖，:17 注释）⇒ 证件域三键留着。后果与 #1380 同形：401 顶出 → 登录页 →
 * **换另一个账号登录**（`login()` 只做 `setAuthData()`，不清任何槽）→ `GET /me/credential` 恰好失败时
 * 首页离线回退读 `readCurrentCredentialCache()`，供出**上一个账号**的证件 id + name。
 * 与 #1380 的差别只在触发条件：那条要「主动退出 / 身份切换」，本条**被动**发生在任何一次 401 之后。
 *
 * 接缝（票面裁定：全局回调通道，`registerRefreshTokenHandler` 同款）：`request.uts` 触发回调、
 * `auth.uts` 注册 `clearCredentialStorage()` —— 不在 `request.uts` 抄第三份键清单（那正是 #1380
 * 要消灭的「两处各写一份」形态）。行为面在本文件（只有跑起来才看得见：谁被清、谁被留下），
 * 接线面归 `utils/concurrent401RefreshContract.test.js` 的 C6 族。
 *
 * 同时钉住**不变量**（I2/I3/I4 与 ADR-0022 ⑤ 的既有登出口径，本票一动不许）：
 * 一次跳转、`auth_refresh_token` 保留、`auth_secure_credentials` 保留、内存登录态归零。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');
const CRED_UTS = path.join(ROOT, 'api', 'credential.uts');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';
const KEY_CREDENTIALS = 'auth_secure_credentials';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const ROLE_STUDENT = 'student';
const ROLE_RECRUITER = 'recruiter';

/** 证件域三键（#1380 清单；`selected_cert` 是 #1349 之前的既有残留） */
const CERT_KEYS = ['selected_cert', 'selected_cert_id', 'selected_cert_name'];

const EXPIRED = 'expired-access-token';
const ACCOUNT_A_RT = 'refresh-token-of-account-A';
const ACCOUNT_B_ACCESS = 'fresh-access-token-of-account-B';
const ACCOUNT_B_RT = 'refresh-token-of-account-B';

/** 上一个账号留下的证件数据（三键全部有值 = 缺陷发生的那个前置态，同 credentialGroupedBehavior） */
const prevAccountCert = () => ({ selected_cert: 'forklift_n1', selected_cert_id: '7', selected_cert_name: '上一账号的证' });

const toNumber = (v, d = 0) => (v == null ? d : (Number.isNaN(parseFloat(`${v}`)) ? d : parseFloat(`${v}`)));
const toNumberOrNull = (v) => (v == null ? null : (Number.isNaN(parseFloat(`${v}`)) ? null : parseFloat(`${v}`)));
const toStr = (v, d = '') => (v == null ? d : `${v}`);

/** 「清」的两种合法形态都算（写空串 / 移除键），判据只取生产读方看得到的那个：`as string` 后为空 */
const isCleared = (v) => v === undefined || v === '';

/** 假 `uni`：storage 用 Map 落地（语义同 `utils/storage.uts`），request 只**捕获**不自动回应 */
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
 * 装配一棵真实的依赖树（同 `concurrent401RefreshBehavior.test.js` 的配方）：
 * `refreshGate.uts` + `storage.uts` → `request.uts` →（两条回调：刷新 + 本票的清理）→ `auth.uts`；
 * 证件缓存的**读出口**用真的 `api/credential.uts` 接在同一片 storage 上 —— 断言「缓存读回 null」
 * 用的就是生产里那段读代码，不是测试自己另搭一份。
 * @param {{ certSeed?: Record<string, string> }} [opts] 上一账号留下的证件数据
 */
function buildApp(opts = {}) {
  const uni = makeUni();
  const pages = [{ route: 'pages/dashboard/dashboard' }];
  Object.entries(opts.certSeed || {}).forEach(([k, v]) => uni.setStorageSync(k, v));

  const storage = loadUts(STORAGE_UTS, { uni });
  const gate = loadUts(GATE_UTS, {});
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

  // 本票的接缝必须是**接线好的**（不是占位桩）：auth.uts 注册进 request.uts 的那个回调被调了几次可观测
  const unauthorizedCleanupCalls = [];
  const refreshCalls = [];
  const noopApi = () => Promise.resolve(null);
  const authMod = loadUts(AUTH_UTS, {
    ref: (v) => ({ value: v }),
    registerRefreshTokenHandler: request.registerRefreshTokenHandler,
    registerUnauthorizedCredentialCleanup: (handler) => {
      request.registerUnauthorizedCredentialCleanup(() => {
        unauthorizedCleanupCalls.push(1);
        return handler();
      });
    },
    loginApi: () => Promise.resolve({
      token: ACCOUNT_B_ACCESS,
      refresh_token: ACCOUNT_B_RT,
      user: { user_id: 8, username: 'u8', name: '乙同学', role: ROLE_STUDENT },
    }),
    phoneLoginApi: noopApi,
    emailLoginApi: noopApi,
    mpWechatLoginApi: noopApi,
    logoutApi: () => Promise.resolve({}),
    getUserInfoApi: noopApi,
    recruiterLoginApi: noopApi,
    // 甲账号的 refresh_token 也无效 ⇒ 401 走「真失败」顶出（本票的触发面）
    refreshTokenApi: (rt) => {
      refreshCalls.push(rt);
      return Promise.reject(new Error('refresh token 已失效'));
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

  // 真的证件缓存读出口（生产离线回退读的就是这段代码）
  const cred = loadUts(CRED_UTS, {
    getMapped: () => Promise.resolve({}),
    requestMapped: () => Promise.resolve({}),
    toNumber,
    toNumberOrNull,
    toStr,
    uni,
  });

  const store = authMod.useAuthStore();
  // 甲账号在册：access 已过期、refresh 有效但后端会拒（复现配方同 concurrent401RefreshBehavior 的 buildApp）
  store.setAuthData(EXPIRED, { user_id: 7, username: 'u7', name: '甲同学', role: ROLE_STUDENT }, ACCOUNT_A_RT);

  /** 回应全部挂起请求：带新 token 的回 200，其余回 401 */
  function stormAll() {
    const pending = uni.requests.splice(0, uni.requests.length);
    pending.forEach((o) => {
      if (o.header['Authorization'] === `Bearer ${ACCOUNT_B_ACCESS}`) {
        o.success({ statusCode: 200, data: { code: 200, message: 'ok', data: { user_id: 8 } } });
        return;
      }
      o.success({ statusCode: 401, data: { code: 401, message: 'Token无效或已过期，请重新登录' } });
    });
    return pending.length;
  }

  return { uni, store, storage, request, cred, refreshCalls, unauthorizedCleanupCalls, stormAll };
}

describe('#1404 401 顶出 → 换账号登录 → 证件缓存读回 null（可见后果面）', () => {
  test('反事实前提：三键有值时缓存确实读得到上一账号的证件（下面的断言不是假绿）', () => {
    const app = buildApp({ certSeed: prevAccountCert() });
    const cached = app.cred.readCurrentCredentialCache();
    expect(cached).not.toBeNull();
    expect(cached.id).toBe(7);
    expect(cached.name).toBe('上一账号的证');
  });

  test('主案：refresh 真失败被 401 顶出 → 乙账号 login() → 缓存读回 null（离线回退不得供出甲账号的证）', async () => {
    const app = buildApp({ certSeed: prevAccountCert() });

    const p = app.request.get('/api/courses');
    expect(app.stormAll()).toBe(1);
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    // 顶出即清：三键为空（写空串形态由 C6 族接线锁管，这里两种合法形态都算）
    for (const k of CERT_KEYS) expect(isCleared(app.uni.kv.get(k))).toBe(true);
    expect(app.cred.readCurrentCredentialCache()).toBeNull();

    // 换账号登录（生产路径：登录页 → auth store 的 login()，只做 setAuthData，不清任何槽）
    await app.store.login({ username: 'u8', password: 'whatever' });
    expect(app.store.isLoggedIn.value).toBe(true);

    // 本票的可见后果：新会话的首页若 `GET /me/credential` 恰好失败，回退读到的必须是 null
    expect(app.cred.readCurrentCredentialCache()).toBeNull();
  });

  test('登出口径不变量逐条守住（I2/I3/I4、ADR-0022 ⑤）：一次跳转、rt 保留、安全凭据保留、内存归零', async () => {
    const app = buildApp({ certSeed: prevAccountCert() });
    app.uni.setStorageSync(KEY_CREDENTIALS, 'secure-envelope');

    const p = app.request.get('/api/courses');
    expect(app.stormAll()).toBe(1);
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    // 恰好一次跳转（isHandlingUnauthorized 去重不受本票影响）
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/login/login');
    // 顶出仍只清 access 侧：refresh_token 保留（既有裁定，本票不动它）
    expect(app.uni.kv.get(KEY_REFRESH)).toBe(ACCOUNT_A_RT);
    // 生物识别凭据保留（ADR-0004，登出面从来不清它）
    expect(app.uni.kv.get(KEY_CREDENTIALS)).toBe('secure-envelope');
    // 登录页 onLoad 归零内存登录态，不弹回首页（#1124 语义不变）
    expect(app.store.restoreFromStorage()).toBe(false);
    expect(app.store.isLoggedIn.value).toBe(false);
    expect(app.uni.relaunches.length).toBe(1);
  });

  test('判别力对照：接缝未触发时三键必留着（证明上面的红-绿差异确实来自这条链）', async () => {
    const app = buildApp({ certSeed: prevAccountCert() });
    // 不经 401：任何请求都没发生 ⇒ 三键原样（清理只挂在 401 顶出这一个点上）
    expect(app.unauthorizedCleanupCalls.length).toBe(0);
    for (const k of CERT_KEYS) expect(isCleared(app.uni.kv.get(k))).toBe(false);
  });

  test('招聘者态 401 同样清三键（handleUnauthorized 单点角色化，清理在跳转前对两角色一致）', async () => {
    const app = buildApp({ certSeed: prevAccountCert() });
    app.uni.setStorageSync(KEY_ACTIVE_ROLE, ROLE_RECRUITER);
    // 招聘者态无 refresh_token ⇒ 401 直接终态登出（不调刷新链）
    app.uni.removeStorageSync(KEY_REFRESH);

    const p = app.request.get('/api/recruit/jobs');
    expect(app.stormAll()).toBe(1);
    await expect(p).rejects.toThrow('登录已过期');
    await sleep(30);

    expect(app.uni.relaunches[0].url).toBe('/pages/recruiter/login');
    for (const k of CERT_KEYS) expect(isCleared(app.uni.kv.get(k))).toBe(true);
    expect(app.cred.readCurrentCredentialCache()).toBeNull();
    expect(app.refreshCalls.length).toBe(0);
  });
});
