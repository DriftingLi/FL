/**
 * credential.uts 证件列表切后端 —— **行为层**测试（#1349，#1346 评审跟进）
 *
 * 守护两件事，都是「只有跑起来才看得见」的：
 * 1) `getAllCredentialsApi` 的列表真源是后端 `GET /api/credentials/grouped`
 *    （`training_catalog.go:39`，2026-09-28 实测生产 200），**不再是** `getMockCredentialList()`；
 *    失败必须 reject（同 #1346 对 `switchCredentialApi` 的反转口径），不得兜成假列表。
 * 2) 当前证件的离线回退不再查硬编码字典，改查「上一次后端确认值」的缓存
 *    （`selected_cert_id` / `selected_cert_name`）；后端确认无证件 ⇒ 缓存必须清空，不得供旧值。
 *
 * 分工（`utsHarness` 文件头口径）：本文件跑**行为**，接线（谁调谁、存储键字面量）由
 * `dashboardContract.test.js` 的源码锁守。
 *
 * 第三段（#1380）把**真的 `stores/auth.uts`** 也起起来：证件域三键在登出 / 身份切换时
 * 必须一并清空 —— 那三个键是「上一个账号」的数据，留着就会在下一个账号的会话里、
 * 且恰好在 `GET /me/credential` 失败时被 `readCurrentCredentialCache()` 顶显出来。
 */
const path = require('path');
const { loadUts, importedNames, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const CRED = path.join(ROOT, 'api', 'credential.uts');
const COMPOSABLE = path.join(ROOT, 'pages', 'dashboard', 'composables', 'use-dashboard-credential.uts');
const AUTH_STORE = path.join(ROOT, 'stores', 'auth.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');

/** 后端 CredentialDict 的线上形态（字段序按 training_catalog_types.go:107 的字母序声明） */
const dict = (over) => ({
  category: 'special_operation', code: 'forklift_n1', created_at: '2026-08-27T22:03:10+08:00',
  description: '场内专用机动车辆作业-叉车司机N1', id: 1, level: null, name: '叉车司机N1证',
  sort_order: 1, status: 1, updated_at: '2026-08-27T22:03:10+08:00', ...over,
});

/** 最小 uni：storage 走 map（缺键回 ''，与 UTS 侧 `as string` 语义一致），toast / reLaunch 收集 */
function makeUni(init = {}) {
  const store = { ...init };
  const toasts = [];
  const relaunches = [];
  const uni = {
    getStorageSync: (k) => (store[k] === undefined ? '' : store[k]),
    setStorageSync: (k, v) => { store[k] = v; },
    removeStorageSync: (k) => { delete store[k]; },
    showToast: (o) => { toasts.push(o.title); },
    reLaunch: (o) => { relaunches.push(o); },
  };
  return { uni, store, toasts, relaunches };
}

const toNumber = (v, d = 0) => (v == null ? d : (Number.isNaN(parseFloat(`${v}`)) ? d : parseFloat(`${v}`)));
const toNumberOrNull = (v) => (v == null ? null : (Number.isNaN(parseFloat(`${v}`)) ? null : parseFloat(`${v}`)));
const toStr = (v, d = '') => (v == null ? d : `${v}`);

/** 起真实的 api/credential.uts：getMapped / requestMapped 由用例注入 */
function loadApi({ storage = {}, getMapped, requestMapped } = {}) {
  const u = makeUni(storage);
  const calls = [];
  const gM = (url, params, map) => {
    calls.push({ method: 'GET', url });
    return getMapped == null ? Promise.resolve(map({})) : getMapped(url, params, map);
  };
  const rM = (opts, map) => {
    calls.push({ method: opts.method, url: opts.url, data: opts.data });
    return requestMapped == null ? Promise.resolve(map({})) : requestMapped(opts, map);
  };
  const mod = loadUts(CRED, { getMapped: gM, requestMapped: rM, toNumber, toNumberOrNull, toStr, uni: u.uni });
  return { mod, calls, store: u.store, toasts: u.toasts };
}

describe('getAllCredentialsApi：列表来自后端 grouped（#1349 反转 #643 的 mock 列表）', () => {
  test('两桶映射成两组，顺序固定为 special_operation → skill_level（不随响应键序）', async () => {
    const payload = {
      skill_level: [dict({ category: 'skill_level', code: 'maintenance_L5', id: 4, level: 5, name: '五级', sort_order: 4 })],
      special_operation: [dict({}), dict({ id: 2, code: 'low_voltage_electrician', name: '低压电工证' })],
    };
    const { mod, calls } = loadApi({ getMapped: (url, params, map) => Promise.resolve(map(payload)) });
    const groups = await mod.getAllCredentialsApi();

    expect(groups.map((g) => g.category)).toEqual(['special_operation', 'skill_level']);
    expect(groups[0].group_name).toBe('特种作业配套上岗证');
    expect(groups[0].icon).toBe('🔧');
    expect(groups[1].group_name).toBe('工程机械维修工（叉车维修方向技能等级证）');
    expect(groups[1].icon).toBe('🚜');
    expect(groups[0].items.map((i) => i.id)).toEqual([1, 2]);
    expect(groups[1].items[0].level).toBe(5);
    // 防「无条件返回假列表」式假绿：必须真的发出这一条 GET
    expect(calls).toEqual([{ method: 'GET', url: expect.stringContaining('/credentials/grouped') }]);
  });

  test('空桶不出组（后端空集也回 []，不能渲染出空分组头）', async () => {
    const payload = { skill_level: [], special_operation: [dict({})] };
    const { mod } = loadApi({ getMapped: (url, params, map) => Promise.resolve(map(payload)) });
    const groups = await mod.getAllCredentialsApi();
    expect(groups.map((g) => g.category)).toEqual(['special_operation']);
  });

  test('单桶缺键不抛（响应里没有 skill_level 时只出一组）', async () => {
    const payload = { special_operation: [dict({})] };
    const { mod } = loadApi({ getMapped: (url, params, map) => Promise.resolve(map(payload)) });
    const groups = await mod.getAllCredentialsApi();
    expect(groups.map((g) => g.category)).toEqual(['special_operation']);
  });

  test('归组以「桶」为准：条目自带 category 与桶不一致时仍按桶归组（钉住后端两桶的现行约定）', async () => {
    const payload = { special_operation: [dict({ category: 'skill_level' })] };
    const { mod } = loadApi({ getMapped: (url, params, map) => Promise.resolve(map(payload)) });
    const groups = await mod.getAllCredentialsApi();
    expect(groups.map((g) => g.category)).toEqual(['special_operation']);
  });

  test('请求失败必须 reject（不得兜成 mock 列表；#1346 同一口径）', async () => {
    const { mod, calls } = loadApi({ getMapped: () => Promise.reject(new Error('500 backend reject')) });
    await expect(mod.getAllCredentialsApi()).rejects.toBeInstanceOf(Error);
    expect(calls).toHaveLength(1);
  });
});

describe('getCurrentCredentialApi：失败/未选择仍返回 null，缓存上移到 composable（#1349）', () => {
  test('后端确认无证件（credential:null）返回 null，不再内部查 mock 列表', async () => {
    const { mod, calls } = loadApi({
      storage: { selected_cert: 'forklift_n1' },
      getMapped: (url, params, map) => Promise.resolve(map({ credential: null })),
    });
    expect(await mod.getCurrentCredentialApi()).toBeNull();
    // 只发这一条请求：api 层不再为了回退去拉列表
    expect(calls).toHaveLength(1);
  });

  test('请求异常返回 null 而不是抛（GET 面「读不到」不当业务失败，语义与 #643 一致）', async () => {
    const { mod } = loadApi({ getMapped: () => Promise.reject(new Error('network down')) });
    await expect(mod.getCurrentCredentialApi()).resolves.toBeNull();
  });

  test('后端返回证件时透传（level 走 null 容忍）', async () => {
    const { mod } = loadApi({ getMapped: (url, params, map) => Promise.resolve(map({ credential: dict({}) })) });
    const c = await mod.getCurrentCredentialApi();
    expect(c.id).toBe(1);
    expect(c.level).toBeNull();
  });
});

describe('当前证件缓存（selected_cert_id / selected_cert_name）读写出口', () => {
  test('有 name 时读回 item（id 取缓存值，code 留空——缓存没有 code，不猜）', () => {
    const { mod } = loadApi({ storage: { selected_cert_id: '4', selected_cert_name: '五级' } });
    const c = mod.readCurrentCredentialCache();
    expect(c.id).toBe(4);
    expect(c.name).toBe('五级');
    expect(c.code).toBe('');
  });

  test('name 为空 ⇒ null（id 为空/非数字都不算可用的缓存）', () => {
    for (const init of [{}, { selected_cert_name: '' }, { selected_cert_id: '9' }]) {
      expect(loadApi({ storage: init }).mod.readCurrentCredentialCache()).toBeNull();
    }
    // id 非数字但 name 在 ⇒ 仍读回（页面只用 name 时不该整条作废），id 折 0
    const c = loadApi({ storage: { selected_cert_id: 'abc', selected_cert_name: '五级' } }).mod.readCurrentCredentialCache();
    expect(c.name).toBe('五级');
    expect(c.id).toBe(0);
  });

  test('写入 id + name 两键；传 null ⇒ 两键一起清空（后端确认无证件不得留旧值）', () => {
    // #1395 沿革：清空原为「三键」（含 selected_cert code 键）；code 键随引导页接线退役
    // （全仓写 0 读 0），这里只锁缓存自己的两键。
    const { mod, store } = loadApi({ storage: {} });
    mod.writeCurrentCredentialCache({ id: 4, code: 'maintenance_L5', name: '五级' });
    expect(store['selected_cert_id']).toBe('4');
    expect(store['selected_cert_name']).toBe('五级');

    mod.writeCurrentCredentialCache(null);
    expect(store['selected_cert_id']).toBe('');
    expect(store['selected_cert_name']).toBe('');
  });
});

describe('use-dashboard-credential：当前证件与列表各自失败各自呈现（#1349）', () => {
  /** 起真 composable：三个 api 出口 + 两个缓存出口注入替身，只测编排 */
  function loadComposable({ getCurrent, getAll, writeCache, readCache, switchApi }) {
    const { uni, toasts } = makeUni();
    const writes = [];
    const mod = loadUts(COMPOSABLE, {
      ref: (v) => ({ value: v }),
      getCurrentCredentialApi: getCurrent == null ? () => Promise.resolve(null) : getCurrent,
      getAllCredentialsApi: getAll == null ? () => Promise.resolve([]) : getAll,
      switchCredentialApi: switchApi == null ? () => Promise.resolve({ success: true, credential: null }) : switchApi,
      readCurrentCredentialCache: readCache == null ? () => null : readCache,
      writeCurrentCredentialCache: (c) => { writes.push(c); if (writeCache) writeCache(c); },
      uni,
    });
    return { face: mod.useDashboardCredential(), toasts, writes };
  }

  const group = (cat, id, name) => ({ group_name: cat, icon: 'x', category: cat, items: [{ id, code: 'c' + id, name }] });

  test('列表失败不污染已取到的当前证件：currentCert 保持后端值，分组留空，提示一次', async () => {
    const { face, toasts } = loadComposable({
      getCurrent: () => Promise.resolve({ id: 7, code: 'maintenance_L2', name: '二级' }),
      getAll: () => Promise.reject(new Error('list 500')),
    });
    await face.loadCredentials();
    expect(face.currentCert.value).toBe('二级');
    expect(face.currentCredentialId.value).toBe(7);
    expect(face.credentialGroups.value).toEqual([]);
    expect(toasts).toEqual(['证件列表加载失败，请重试']);
  });

  test('后端确认无证件 + 缓存命中 ⇒ 用缓存显示，且不回写缓存', async () => {
    const { face, writes } = loadComposable({
      getCurrent: () => Promise.resolve(null),
      getAll: () => Promise.resolve([group('special_operation', 1, 'N1')]),
      readCache: () => ({ id: 1, code: '', name: '叉车司机N1证' }),
    });
    await face.loadCredentials();
    expect(face.currentCert.value).toBe('叉车司机N1证');
    expect(face.currentCredentialId.value).toBe(1);
    expect(writes).toEqual([]);
  });

  test('后端确认无证件 + 无缓存 ⇒ 清空缓存三键并显示「选择证件」', async () => {
    const { face, writes } = loadComposable({
      getCurrent: () => Promise.resolve(null),
      getAll: () => Promise.resolve([]),
      readCache: () => null,
    });
    await face.loadCredentials();
    expect(face.currentCert.value).toBe('选择证件');
    expect(writes).toEqual([null]);
  });

  test('同一错误不重复弹 toast（onShow 高频重进首页），错误变化才再弹', async () => {
    let msg = 'list A';
    const { face, toasts } = loadComposable({
      getCurrent: () => Promise.resolve({ id: 7, code: 'x', name: '二级' }),
      getAll: () => Promise.reject(new Error(msg)),
    });
    await face.loadCredentials();
    await face.loadCredentials();
    expect(toasts).toEqual(['证件列表加载失败，请重试']);
    msg = 'list B';
    await face.loadCredentials();
    expect(toasts).toEqual(['证件列表加载失败，请重试', '证件列表加载失败，请重试']);
  });

  test('列表成功 ⇒ 分组落位到 credentialGroups', async () => {
    const gs = [group('special_operation', 1, 'N1')];
    const { face, toasts } = loadComposable({
      getCurrent: () => Promise.resolve({ id: 1, code: 'forklift_n1', name: 'N1' }),
      getAll: () => Promise.resolve(gs),
    });
    await face.loadCredentials();
    expect(face.credentialGroups.value).toEqual(gs);
    expect(toasts).toEqual([]);
  });

  test('切换成功 ⇒ 后端确认值进缓存（离线回退读的就是它）', async () => {
    const cred = { id: 3, code: 'welder', name: '焊工证' };
    const { face, writes } = loadComposable({
      switchApi: () => Promise.resolve({ success: true, credential: cred }),
    });
    await face.onSelectCredential({ id: 3, code: 'welder', name: '焊工证' });
    expect(writes).toEqual([cred]);
  });

  test('后端 200 没回凭证（未确认切换）⇒ 不回滚外不加写缓存（#1346 口径不回潮）', async () => {
    const { face, writes } = loadComposable({
      switchApi: () => Promise.resolve({ success: false, credential: null }),
    });
    await face.onSelectCredential({ id: 3, code: 'welder', name: '焊工证' });
    expect(writes).toEqual([]);
    expect(face.currentCert.value).toBe('加载中...');
  });
});

// ===== #1380：登出 / 身份切换必须清掉证件域三键 =====

/** auth.uts 清槽清单里那几个键的真值（真源在 constants/app.uts；本票的接线锁 = `concurrent401RefreshContract.test.js` 的 C6） */
const KEY = {
  token: 'auth_token',
  refreshToken: 'auth_refresh_token',
  user: 'auth_user',
  loginProvider: 'auth_login_provider',
  credentials: 'auth_secure_credentials',
};
/** 证件域三键：`selected_cert` 是 #1349 之前的既有残留，两外键由 #1349 新增 */
const CERT_KEYS = ['selected_cert', 'selected_cert_id', 'selected_cert_name'];
const ACCESS = 'access-under-test';
const RT = 'refresh-under-test';
const USER_OBJ = { user_id: 7, username: 'u7', name: '同学', role: 'student' };

/** 「清」的两种合法形态都算（写空串 / 移除键），判据只取生产读方看得到的那个：`as string` 后为空 */
const isCleared = (v) => v === undefined || v === '';

/**
 * 起**真的** `stores/auth.uts`（连同真的 `utils/storage.uts`），并把**真的**
 * `api/credential.uts` 的缓存读出口接在同一片 storage 上 —— 断言「退出后缓存读回 null」
 * 用的就是生产里那段读代码，不是测试自己另搭一份。其余 import 按名单补桩。
 * 读源一律走 `readText`（ADR-0019：契约/行为测试的 EOL 归一读取层）。
 */
function loadAuthFace(init = {}) {
  const u = makeUni(init);
  const storage = loadUts(STORAGE_UTS, { uni: u.uni });
  const bindings = {};
  importedNames(readText(AUTH_STORE)).forEach((n) => { bindings[n] = () => {}; });
  bindings.ref = (v) => ({ value: v });
  bindings.registerRefreshTokenHandler = () => {};
  bindings.logoutApi = () => Promise.resolve({});
  bindings.setStorage = storage.setStorage;
  bindings.setStorageJSON = storage.setStorageJSON;
  bindings.getStorage = storage.getStorage;
  bindings.getStorageJSON = storage.getStorageJSON;
  bindings.removeStorage = storage.removeStorage;
  bindings.updateSecureToken = () => {};
  bindings.setActiveRole = () => {};
  bindings.clearActiveRole = () => {};
  bindings.errMsg = (_e, fallback) => fallback;
  bindings.STORAGE_KEY_TOKEN = KEY.token;
  bindings.STORAGE_KEY_REFRESH_TOKEN = KEY.refreshToken;
  bindings.STORAGE_KEY_USER = KEY.user;
  bindings.STORAGE_KEY_LOGIN_PROVIDER = KEY.loginProvider;
  bindings.STORAGE_KEY_CREDENTIALS = KEY.credentials;
  bindings.ACTIVE_ROLE_RECRUITER = 'recruiter';
  bindings.uni = u.uni;
  const cred = loadUts(CRED, { getMapped: () => Promise.resolve({}), requestMapped: () => Promise.resolve({}), toNumber, toNumberOrNull, toStr, uni: u.uni });
  return { store: loadUts(AUTH_STORE, bindings).useAuthStore(), kv: u.store, uni: u, readCache: () => cred.readCurrentCredentialCache() };
}

/** 上一个账号留下的证件数据（三键全部有值 = 缺陷发生的那个前置态） */
const prevAccountCert = () => ({ selected_cert: 'forklift_n1', selected_cert_id: '7', selected_cert_name: '上一账号的证' });

describe('登出与身份切换必须清掉证件域三键（#1380）', () => {
  test('反事实前提：三键有值时缓存确实读得到上一账号的证件（下面两条不是假绿）', () => {
    const face = loadAuthFace(prevAccountCert());
    const cached = face.readCache();
    expect(cached).not.toBeNull();
    expect(cached.id).toBe(7);
    expect(cached.name).toBe('上一账号的证');
  });

  test('学员登出（走真的 logout()）后三键为空、缓存读回 null', async () => {
    const face = loadAuthFace({ ...prevAccountCert(), [KEY.credentials]: 'secure-envelope', [KEY.loginProvider]: 'password' });
    face.store.setAuthData(ACCESS, USER_OBJ, RT);
    // 前提：登录态与证件数据同时在槽里
    expect(face.kv[KEY.token]).toBe(ACCESS);
    expect(face.readCache()).not.toBeNull();

    await face.store.logout();

    for (const k of CERT_KEYS) expect(isCleared(face.kv[k])).toBe(true);
    expect(face.readCache()).toBeNull();
    // auth 四键照旧清干净（新增三键不该挤掉既有清理项）
    expect(isCleared(face.kv[KEY.token])).toBe(true);
    expect(isCleared(face.kv[KEY.refreshToken])).toBe(true);
    expect(isCleared(face.kv[KEY.loginProvider])).toBe(true);
    // 保留项逐条不变：生物识别凭据按 ADR-0004 留下，本票**只增清理项**
    expect(face.kv[KEY.credentials]).toBe('secure-envelope');
    expect(face.uni.relaunches.map((o) => o.url)).toContain('/pages/login/login');
  });

  test('身份互斥清槽（学员↔招聘者、招聘者退出）后三键为空、缓存读回 null', () => {
    const face = loadAuthFace({ ...prevAccountCert(), [KEY.credentials]: 'secure-envelope', [KEY.token]: ACCESS });

    face.store.clearIdentityForSwitch();

    for (const k of CERT_KEYS) expect(isCleared(face.kv[k])).toBe(true);
    expect(face.readCache()).toBeNull();
    // 切换语义逐条不变：安全凭据在切换中**必须**清（ADR-0021 ② 单槽）
    expect(isCleared(face.kv[KEY.credentials])).toBe(true);
    expect(isCleared(face.kv[KEY.token])).toBe(true);
  });

  test('清后不留「可顶显的脏缓存」：即使 GET /me/credential 挂了也不得供出上一账号（本票的可见后果）', async () => {
    const face = loadAuthFace(prevAccountCert());
    face.store.setAuthData(ACCESS, USER_OBJ, RT);

    await face.store.logout();

    // 首页离线回退分支读的就是这个：null ⇒ 页面走「选择证件」，不会把上一账号的证顶显
    expect(face.readCache()).toBeNull();
  });
});
