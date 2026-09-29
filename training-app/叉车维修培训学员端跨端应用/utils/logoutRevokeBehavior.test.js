/**
 * #1386 登出吊销行为守护（**跑真的 `api/auth.uts` / `stores/auth.uts`**，不断言源码文本）
 *
 * 契约来源：移动端 `docs/adr/0030-refresh令牌的通道归属与族判定口径.md` ④ 第 2 项 · issue #1386
 *
 * 钉住的不变量：
 *   L1 登出**必须把手上那支 refresh_token 发给后端**（`/auth/logout` 的 body）——
 *      不带 ⇒ 服务端吊销分支拿不到东西 ⇒ 登出后那支 7 天凭证仍可续（#1385 缺口 2）；
 *   L2 **顺序即语义**：rt 必须在清槽（`clearAuthData()` 会移除 `auth_refresh_token`）
 *      **之前**取到 —— 顺序反了 L1 就静默退化成「发空串」，故断言「非空」而不是只断言「被调用」；
 *   L3 登出仍**静默**：后端失败不弹 toast、本地照样清槽并跳登录页（`silent: true` 语义不得改）；
 *   L4 api 层带 rt 之后，**清槽清单一字不动**（#1380  owns 那三个键，本票只加请求体）。
 *
 * 为什么用 utsHarness 而不是源码文本断言：源码文本只能证明「某个字面量出现过」，
 * 证明不了「rt 是在清槽前还是清槽后读的」——那正是本票唯一的新风险。
 */
const path = require('path');
const fs = require('fs');
const { loadUts, importedNames } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const API_AUTH_UTS = path.join(ROOT, 'api', 'auth.uts');
const STORE_AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';
const KEY_CREDENTIALS = 'auth_secure_credentials';

const STUDENT = 'student';
const ACCESS = 'access-token-under-test';
const RT = 'refresh-token-under-test';
const USER_OBJ = { user_id: 7, username: 'u7', name: '同学', role: STUDENT };

/** 假 uni：storage 落 Map；reLaunch / showToast 只记录 */
function makeUni() {
  const kv = new Map();
  const toasts = [];
  const relaunches = [];
  return {
    kv,
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
    request: () => {},
    getSystemInfoSync: () => ({ statusBarHeight: 20, windowHeight: 800 }),
  };
}

/**
 * 起真的 `api/auth.uts`，捕获它经 `post` 出口发出的请求。
 * bindings 按该模块的 import 名单**自动补齐**（缺一个 harness 就抛「缺绑定」，
 * 自动补可避免本守护因无关 import 变动而连锁红）。
 */
function loadApiAuth(uni) {
  const src = fs.readFileSync(API_AUTH_UTS, 'utf8');
  const captured = [];
  const bindings = {};
  importedNames(src).forEach((n) => { bindings[n] = () => Promise.resolve(null); });
  bindings.post = (url, data, options) => {
    captured.push({ url, data, options });
    return Promise.resolve({});
  };
  bindings.uni = uni;
  const mod = loadUts(API_AUTH_UTS, bindings);
  return { mod, captured };
}

/**
 * 起真的 `stores/auth.uts`。
 * ⚠️ `logout()` 有一层「内存 access 非空才打后端」的守卫（`token` 是模块级 `ref('')`，
 * 不是 storage 派生值）⇒ 登录态必须经**真的** `setAuthData()` 落，直接种 storage 会让
 * 守卫判成未登录、整条吊销路径被静默跳过（本守护第一版就是这么误红的，记在这里防再犯）。
 */
function seedLogin(store, storage) {
  store.setAuthData(ACCESS, USER_OBJ, RT);
  // 冗余确认：access 与 refresh 都该在槽里（setAuthData 是单一落点）
  expect(storage.getStorage(KEY_REFRESH)).toBe(RT);
}
/** 起真的 `stores/auth.uts`（storage / authRole 用真模块，其余按 import 名单补桩） */
function loadStoreAuth(uni, opts) {
  const logoutFails = !!(opts && opts.logoutFails);
  const storage = loadUts(STORAGE_UTS, { uni });
  const storeSrc = fs.readFileSync(STORE_AUTH_UTS, 'utf8');
  const logoutApiCalls = [];
  const bindings = {};
  importedNames(storeSrc).forEach((n) => { bindings[n] = () => {}; });
  // vue 的 ref：与 concurrent401RefreshBehavior 同款最小实现
  bindings.ref = (v) => ({ value: v });
  bindings.registerRefreshTokenHandler = () => {};
  bindings.logoutApi = (rt) => {
    // 记录**调用那一刻**storage 里还在不在 rt —— 这就是 L2 的顺序判据
    logoutApiCalls.push({ rt, refreshStillInStorage: storage.getStorage(KEY_REFRESH).length > 0 });
    // L4：后端**失败**路径（网络错 / 401 都算）—— 静默语义要求本地照常清槽、不弹 toast
    return logoutFails ? Promise.reject(new Error('logout backend unavailable')) : Promise.resolve({});
  };
  bindings.setStorage = storage.setStorage;
  bindings.setStorageJSON = storage.setStorageJSON;
  bindings.getStorage = storage.getStorage;
  bindings.getStorageJSON = storage.getStorageJSON;
  bindings.removeStorage = storage.removeStorage;
  bindings.updateSecureToken = () => {};
  bindings.setActiveRole = () => {};
  bindings.clearActiveRole = () => {};
  bindings.errMsg = (_e, fallback) => fallback;
  bindings.STORAGE_KEY_TOKEN = KEY_TOKEN;
  bindings.STORAGE_KEY_REFRESH_TOKEN = KEY_REFRESH;
  bindings.STORAGE_KEY_USER = KEY_USER;
  bindings.STORAGE_KEY_LOGIN_PROVIDER = KEY_PROVIDER;
  bindings.STORAGE_KEY_CREDENTIALS = KEY_CREDENTIALS;
  bindings.ACTIVE_ROLE_RECRUITER = 'recruiter';
  bindings.uni = uni;
  const mod = loadUts(STORE_AUTH_UTS, bindings);
  return { mod, storage, logoutApiCalls };
}

describe('#1386 登出必须把 refresh_token 交给后端吊销', () => {
  test('L1/L3 api 层：logoutApi(rt) 经 post 发出 /auth/logout，body 带 refresh_token 且 silent', async () => {
    const uni = makeUni();
    const { mod, captured } = loadApiAuth(uni);

    await mod.logoutApi(RT);

    expect(captured).toHaveLength(1);
    const sent = captured[0];
    expect(sent.url).toBe('/auth/logout');
    expect(sent.data).not.toBeNull();
    expect(sent.data.refresh_token).toBe(RT);
    // 登出是用户主动行为：后端失败不得弹 toast 干扰（silent 语义逐字保留）
    expect(sent.options.silent).toBe(true);
    expect(uni.toasts).toHaveLength(0);
  });

  test('L2 顺序：store 的 logout() 在清槽之前就把手上那支 rt 交给 logoutApi', async () => {
    const uni = makeUni();
    const { mod, storage, logoutApiCalls } = loadStoreAuth(uni);
    const store = mod.useAuthStore();

    // 正常登录态：access + refresh 都在槽里
    seedLogin(store, storage);

    await store.logout();

    expect(logoutApiCalls).toHaveLength(1);
    const call = logoutApiCalls[0];
    expect(call.rt).toBe(RT);
    // 取 rt 的时刻 storage 里还得有它 —— 反序（先清槽再取）会静默退化成发空串
    expect(call.refreshStillInStorage).toBe(true);
  });

  test('L2b 吊销之后照常清槽并跳登录页（本票只加请求体，清槽清单一字不动）', async () => {
    const uni = makeUni();
    const { mod, storage, logoutApiCalls } = loadStoreAuth(uni);
    const store = mod.useAuthStore();

    seedLogin(store, storage);

    await store.logout();

    expect(logoutApiCalls).toHaveLength(1);
    expect(storage.getStorage(KEY_TOKEN)).toBe('');
    expect(storage.getStorage(KEY_REFRESH)).toBe('');
    expect(uni.relaunches.map((o) => o.url)).toContain('/pages/login/login');
    // 生物识别凭据故意保留（ADR-0004）—— 本票不得顺手改这条
    expect(storage.getStorage(KEY_CREDENTIALS)).toBeDefined();
  });

  test('L4 后端失败也照常清本地登录态、且吊销请求仍照发（静默语义不因带 rt 而变）', async () => {
    const uni = makeUni();
    const { mod, storage, logoutApiCalls } = loadStoreAuth(uni, { logoutFails: true });
    const store = mod.useAuthStore();
    seedLogin(store, storage);

    await store.logout();

    // 后端失败不改变「先把 rt 交出去」的尝试；失败被吞掉，不阻断本地登出
    expect(logoutApiCalls).toHaveLength(1);
    expect(logoutApiCalls[0].rt).toBe(RT);
    expect(storage.getStorage(KEY_TOKEN)).toBe('');
    expect(uni.relaunches.map((o) => o.url)).toContain('/pages/login/login');
    expect(uni.toasts).toHaveLength(0);
  });
});
