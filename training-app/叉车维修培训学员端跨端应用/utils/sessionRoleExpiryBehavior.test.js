/**
 * 会话收尾与身份落地的行为级回归（真跑 `request.uts` / `auth.uts` / `loginOutlet.uts`）
 *
 * 钉住两条用户可见症状的根因：
 *
 *   E1「退出后仍弹『登录已过期』」—— 401 到达时**会话已被用户主动结束**（或请求发出去时
 *      压根没带凭据）。此时它不是「登录态过期」，不得再弹提示、更不得清一次已经清过的登录态。
 *   E2「登录进入学员端偶发『权限不足』」—— 招聘者与学员共用 `auth_token` 槽，会话恢复只判
 *      「有没有 token」不判「这槽属于谁」。带招聘者 token 被送进学员首页后，学员能力端点
 *      （`CapStudentAccess` 等）全部 403「权限不足」。学员登录路径还必须把残留的
 *      `auth_active_role=recruiter` 归位，否则下一次 401 会按旧角色跳错登录页。
 *
 * 为什么是行为守护：E1 的判据是「toast 次数」「token 是否被误清」「跳转次数」，源码文本
 * 断言证明不了；E2 的判据是「登录后角色标记指向谁」「落地路径选哪条」。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');
const { requestBindings } = require('./requestUtsHarness');

const ROOT = path.join(__dirname, '..');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const LOGIN_OUTLET_UTS = path.join(ROOT, 'utils', 'loginOutlet.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');
const LANDING_PAGE = path.join(ROOT, 'pages', 'index', 'index.uvue');
const LOGIN_PAGE = path.join(ROOT, 'pages', 'login', 'login.uvue');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';
const KEY_CREDENTIALS = 'auth_secure_credentials';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const ROLE_STUDENT = 'student';
const ROLE_RECRUITER = 'recruiter';

const OLD_ACCESS = 'old-access-token';
const REFRESH_1 = 'refresh-token-1';

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
 * 装配真实的依赖树。
 * @param {{ refreshOk?: boolean, deferRefresh?: boolean }} opts
 *   refreshOk=false ⇒ 刷新真失败；deferRefresh=true ⇒ 刷新挂在飞，由调用方 `settleRefresh()` 决定结果
 */
function buildApp(opts) {
  const refreshOk = opts.refreshOk !== false;
  const deferRefresh = opts.deferRefresh === true;
  const uni = makeUni();
  const pages = [{ route: 'pages/dashboard/dashboard' }];

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
    ...requestBindings({ uni, getStorage: storage.getStorage, API_BASE_URL: 'http://127.0.0.1:8080', STORAGE_KEY_TOKEN: KEY_TOKEN }),
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

  let releaseRefresh = null;
  const refreshCalls = [];
  const noopApi = () => Promise.resolve(null);
  const authMod = loadUts(AUTH_UTS, {
    ref: (v) => ({ value: v }),
    registerRefreshTokenHandler: request.registerRefreshTokenHandler,
    registerUnauthorizedCredentialCleanup: request.registerUnauthorizedCredentialCleanup,
    loginApi: (params) => Promise.resolve({
      token: 'student-access-token',
      refresh_token: 'student-refresh-token',
      user: { user_id: 7, username: params.username, name: '同学', role: 'hrwai_user' },
    }),
    phoneLoginApi: noopApi,
    emailLoginApi: noopApi,
    mpWechatLoginApi: noopApi,
    logoutApi: () => Promise.resolve({}),
    getUserInfoApi: noopApi,
    recruiterLoginApi: (params) => Promise.resolve({
      token: 'recruiter-access-token',
      refresh_token: '',
      user: { user_id: 11, username: params.username, name: params.username, role: ROLE_RECRUITER },
    }),
    refreshTokenApi: (rt) => {
      refreshCalls.push(rt);
      if (deferRefresh) {
        return new Promise((resolve, reject) => {
          releaseRefresh = (ok) => {
            if (ok) resolve({ token: 'fresh-access-token', refresh_token: 'refresh-token-2' });
            else reject(new Error('refresh token 已吊销'));
          };
        });
      }
      if (!refreshOk) return Promise.reject(new Error('refresh token 已失效'));
      return Promise.resolve({ token: 'fresh-access-token', refresh_token: 'refresh-token-2' });
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
    ACTIVE_ROLE_STUDENT: ROLE_STUDENT,
    ACTIVE_ROLE_RECRUITER: ROLE_RECRUITER,
    uni,
  });

  const store = authMod.useAuthStore();
  store.setAuthData(OLD_ACCESS, { user_id: 7, username: 'u7', name: '同学', role: 'hrwai_user' }, REFRESH_1);

  /** 回应匹配的挂起请求；带新 token 的给 200，其余 401 */
  function respondWhere(pred) {
    const hit = [];
    const rest = [];
    uni.requests.forEach((o) => { (pred(o) ? hit : rest).push(o); });
    uni.requests.length = 0;
    rest.forEach((o) => uni.requests.push(o));
    hit.forEach((o) => {
      if (o.header['Authorization'] === 'Bearer fresh-access-token') {
        o.success({ statusCode: 200, data: { code: 200, message: 'ok', data: { user_id: 7 } } });
        return;
      }
      o.success({ statusCode: 401, data: { code: 401, message: 'Token无效或已过期，请重新登录' } });
    });
    return hit.length;
  }

  return { uni, store, storage, authRole, request, refreshCalls, respondWhere, releaseRefresh: () => releaseRefresh };
}

function expiredToasts(app) {
  return app.uni.toasts.filter((t) => String(t.title).indexOf('登录已过期') >= 0);
}

describe('E1 401 只对「当前会话」生效', () => {
  test('E1a 用户主动退出后在飞的 401 不弹「登录已过期」、不重复跳转', async () => {
    const app = buildApp({ refreshOk: true });
    const pending = app.request.get('/auth/me');
    expect(app.uni.requests.length).toBe(1);
    expect(app.uni.requests[0].header['Authorization']).toBe(`Bearer ${OLD_ACCESS}`);

    app.store.clearAuthData(); // 用户点退出：清内存 + 清 access 侧凭据（不发跳转）
    expect(app.store.isLoggedIn.value).toBe(false);
    expect(app.uni.relaunches.length).toBe(0);

    app.respondWhere(() => true); // 旧会话的 401 此刻才到
    await expect(pending).rejects.toThrow('登录已过期');

    expect(expiredToasts(app).length).toBe(0);
    expect(app.uni.relaunches.length).toBe(0);
    expect(app.refreshCalls.length).toBe(0); // 没有登录态就别拿 refresh_token 去复活
  });

  test('E1b 退出时刷新在飞且随后失败，也不弹「登录已过期」', async () => {
    const app = buildApp({ deferRefresh: true });
    const pending = app.request.get('/auth/me');

    app.respondWhere(() => true); // 401 ⇒ 起一次刷新（挂在飞）
    await sleep(10);
    expect(app.refreshCalls.length).toBe(1);

    app.store.clearAuthData(); // 刷新在飞期间用户退出（服务端也会吊销那支 refresh_token）
    app.releaseRefresh()(false); // 刷新随后真的失败

    await expect(pending).rejects.toThrow('登录已过期');
    expect(expiredToasts(app).length).toBe(0);
    expect(app.uni.relaunches.length).toBe(0);
  });

  test('E1c 当前会话刷新真失败时仍按原语义提示并跳转（防过度静默）', async () => {
    const app = buildApp({ refreshOk: false });
    const pending = app.request.get('/auth/me');

    app.respondWhere(() => true);
    await expect(pending).rejects.toThrow('登录已过期');

    expect(expiredToasts(app).length).toBe(1);
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
  });
});

describe('E2 会话恢复与登录落地按身份分流', () => {
  test('E2a 学员登录必须把残留的招聘者身份标记归位', async () => {
    const app = buildApp({ refreshOk: true });
    await app.store.loginAsRecruiter({ username: 'hr001', password: 'secret1' });
    expect(app.authRole.isRecruiterActive()).toBe(true);

    await app.store.login({ username: 'u7', password: 'secret1' });
    expect(app.authRole.getActiveRole()).toBe(ROLE_STUDENT);
    expect(app.authRole.isRecruiterActive()).toBe(false);
  });

  test('E2b 已登录启动时的落地路径按身份选择（招聘者不得进学员首页）', () => {
    const app = buildApp({ refreshOk: true });
    const outlet = loadUts(LOGIN_OUTLET_UTS, {
      uni: app.uni,
      setTimeout,
      isRecruiterActive: app.authRole.isRecruiterActive,
    });
    expect(typeof outlet.landingPathForActiveRole).toBe('function');

    app.authRole.setActiveRole(ROLE_RECRUITER);
    expect(outlet.landingPathForActiveRole()).toBe('/pages/recruiter/home');

    app.authRole.clearActiveRole();
    expect(outlet.landingPathForActiveRole()).toBe('/pages/dashboard/dashboard');
  });

  test('E2c 两个「已登录即跳转」的入口共用同一条角色分流', () => {
    const landing = readText(LANDING_PAGE);
    const login = readText(LOGIN_PAGE);
    expect(landing).toContain('landingPathForActiveRole');
    expect(login).toContain('landingPathForActiveRole');
  });
});
