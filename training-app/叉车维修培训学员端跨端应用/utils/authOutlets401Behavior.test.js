/**
 * #1704 三条 401 出口归一 —— **行为级**守护（真跑 `api/request.uts` + `api/aiAssistant.uts`）
 *
 * 契约来源：移动端 `docs/adr/0017-冷启动并发401误判登出缺口.md`
 *
 * #1124/#1131 把「并发 401 共享同一次刷新」钉在了 `doSingleRequest` 上，但**两条旁路出口**
 * 不在它的射程内，仍各写了一份「看见 401 就登出」：
 *   - `uploadFile`（`uni.uploadFile` 不走统一 request 封装）⇒ 一次上传就登出；
 *   - `streamChatApi`（SSE 整包接收）⇒ 报错但**不跳登录页**，与「已登出」状态不一致。
 * 两者都会让「access token 恰好过期但 refresh_token 仍有效」被误判成真失效。
 *
 * 钉住的不变量（与 `doSingleRequest` 同一份规则）：
 *   U1/S1 首次 401 **不得**登出：先续期（一次刷新），成功则带新 token **重发一次**并正常返回；
 *   U2/S2 续期**真的**失败才登出：恰好**一次**跳转 + access 侧凭据清空；
 *   U3/S3 重发**仍** 401 ⇒ 直接终态，**不得**再触发第二次刷新（否则每次重发都白刷一轮）。
 *
 * 为什么不是源码文本断言：文本只能证明某个字面量出现过，证明不了「刷新被调了几次」
 * 「最终 resolve 还是 reject」「跳转了几次」。所以用 `utils/utsHarness.js` 把 `.uts` 真执行起来。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const AI_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');

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

const AUTH_EXPIRED = '登录已过期，请重新登录';
const SSE_URL = 'http://127.0.0.1:8080/ai-assistant/chat';

/** 假 `uni`：storage 用 Map 落地，request / uploadFile 只**捕获**不自动回应 */
function makeUni() {
  const kv = new Map();
  const requests = [];
  const uploads = [];
  const toasts = [];
  const relaunches = [];
  return {
    kv,
    requests,
    uploads,
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
    uploadFile: (o) => { uploads.push(o); },
    getSystemInfoSync: () => ({ statusBarHeight: 20, windowHeight: 800 }),
  };
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** 上传接口的成功信封（`uni.uploadFile` 的 `res.data` 是**字符串**） */
function uploadBody(data) {
  return JSON.stringify({ code: 200, message: 'ok', data: data });
}

/**
 * 装配真实依赖树：refreshGate + storage → request →（回调注册）→ auth store → aiAssistant
 * @param {{ refreshOk: boolean }} opts refreshOk=false 走「refresh_token 也无效」的真失败路径
 */
function buildApp(opts) {
  const refreshOk = opts.refreshOk;
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
  const noopApi = () => Promise.resolve(null);
  const authMod = loadUts(AUTH_UTS, {
    ref: (v) => ({ value: v }),
    registerRefreshTokenHandler: request.registerRefreshTokenHandler,
    registerUnauthorizedCredentialCleanup: request.registerUnauthorizedCredentialCleanup,
    loginApi: noopApi,
    phoneLoginApi: noopApi,
    emailLoginApi: noopApi,
    mpWechatLoginApi: noopApi,
    logoutApi: () => Promise.resolve({}),
    getUserInfoApi: noopApi,
    recruiterLoginApi: noopApi,
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
  // 已登录、access 已过期、refresh_token 仍有效（复现配方里的等价构造）
  store.setAuthData(EXPIRED, { user_id: 7, username: 'u7', name: '同学', role: ROLE_STUDENT }, REFRESH_1);

  const ai = loadUts(AI_UTS, {
    getMapped: request.getMapped,
    postMapped: request.postMapped,
    requestMapped: request.requestMapped,
    del: request.del,
    uploadFile: request.uploadFile,
    // 归一后的单点（#1704）：aiAssistant 不复刻 401 规则，只用 request.uts 的决策函数
    renewOrExpire: request.renewOrExpire,
    AUTH_EXPIRED_MESSAGE: request.AUTH_EXPIRED_MESSAGE,
    decodeList: (d) => d,
    decodePage: (d) => d,
    toNumber: (v) => Number(v),
    toStr: (v) => String(v),
    API_BASE_URL: 'http://127.0.0.1:8080',
    AI_REQUEST_TIMEOUT: 60000,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    getStorage: storage.getStorage,
    uni,
  });

  /** 把队列里**匹配**的条目挑出来交给 respond 回应，返回回应条数 */
  function respondFrom(queue, pred, respond) {
    const hit = [];
    const rest = [];
    queue.forEach((o) => { (pred(o) ? hit : rest).push(o); });
    queue.length = 0;
    rest.forEach((o) => queue.push(o));
    hit.forEach(respond);
    return hit.length;
  }

  return {
    uni,
    store,
    storage,
    request,
    ai,
    refreshCalls,
    bearer: (o) => o.header['Authorization'],
    sentExpired: (o) => o.header['Authorization'] === `Bearer ${EXPIRED}`,
    sentFresh: (o) => o.header['Authorization'] === `Bearer ${FRESH}`,
    /** 上传：回应挂起的上传（按是否带新 token 分派，与 request 侧同构） */
    respondUploadWhere: (pred) => respondFrom(uni.uploads, pred, (o) => {
      if (o.header['Authorization'] === `Bearer ${FRESH}`) {
        o.success({ statusCode: 200, data: uploadBody({ url: 'https://cdn.example.com/f.jpg' }) });
      } else {
        o.success({ statusCode: 401, data: JSON.stringify({ code: 401, message: 'Token无效或已过期，请重新登录' }) });
      }
    }),
    respondUpload401: () => respondFrom(uni.uploads, () => true, (o) => {
      o.success({ statusCode: 401, data: JSON.stringify({ code: 401, message: 'Token无效或已过期，请重新登录' }) });
    }),
    respondUploadOk: () => respondFrom(uni.uploads, () => true, (o) => {
      o.success({ statusCode: 200, data: uploadBody({ url: 'https://cdn.example.com/f.jpg' }) });
    }),
    /** SSE：流式接口的响应体是**原始文本**，不是 JSON 信封 */
    respondSseOk: () => respondFrom(uni.requests, () => true, (o) => {
      o.success({ statusCode: 200, data: 'event: done\ndata: {}\n\n' });
    }),
    respondSse401: () => respondFrom(uni.requests, () => true, (o) => {
      o.success({ statusCode: 401, data: '{"code":401,"message":"Token无效或已过期，请重新登录"}' });
    }),
  };
}

describe('U 上传出口（api/request.uts uploadFile）', () => {
  test('U1: 首次 401 不登出 —— 续期一次后带新 token 重发并 resolve', async () => {
    const app = buildApp({ refreshOk: true });
    const p = app.request.uploadFile('/upload/image', '/tmp/a.jpg', 'file');

    await sleep(10);
    expect(app.uni.uploads.length).toBe(1);
    expect(app.sentExpired(app.uni.uploads[0])).toBe(true);

    app.respondUpload401();
    await sleep(50);

    // 续期发生且只发生一次
    expect(app.refreshCalls.length).toBe(1);
    // 重发：带的是**新** token
    expect(app.uni.uploads.length).toBe(1);
    expect(app.sentFresh(app.uni.uploads[0])).toBe(true);

    app.respondUploadOk();
    const res = await p;
    expect(res['url']).toBe('https://cdn.example.com/f.jpg');
    // 关键：全程没有登出、没有跳转
    expect(app.uni.relaunches.length).toBe(0);
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(true);
  });

  test('U2: 续期真的失败才登出 —— 恰好一次跳转 + access 侧凭据清空', async () => {
    const app = buildApp({ refreshOk: false });
    const p = app.request.uploadFile('/upload/image', '/tmp/a.jpg', 'file');

    await sleep(10);
    app.respondUpload401();
    await expect(p).rejects.toThrow(AUTH_EXPIRED);

    expect(app.refreshCalls.length).toBe(1);
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.relaunches[0].url).toBe('/pages/login/login');
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
    // 不再重发（刷新失败没有可用的新 token）
    expect(app.uni.uploads.length).toBe(0);
  });

  test('U3: 重发仍 401 ⇒ 直接终态，不得触发第二次刷新', async () => {
    const app = buildApp({ refreshOk: true });
    const p = app.request.uploadFile('/upload/image', '/tmp/a.jpg', 'file');

    await sleep(10);
    app.respondUpload401();          // 第一次：过期 token
    await sleep(50);
    app.respondUpload401();          // 第二次：新 token 仍 401（服务端真失效）
    await expect(p).rejects.toThrow(AUTH_EXPIRED);

    expect(app.refreshCalls.length).toBe(1);   // ← 不许刷第二轮
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
  });
});

describe('S SSE 出口（api/aiAssistant.uts streamChatApi）', () => {
  function collect() {
    const seen = { errors: [], done: 0, deltas: [] };
    return {
      seen,
      callbacks: {
        onDelta: (t) => { seen.deltas.push(t); },
        onError: (m) => { seen.errors.push(m); },
        onDone: () => { seen.done += 1; },
      },
    };
  }

  test('S1: 首次 401 不报错 —— 续期一次后重发并正常完成', async () => {
    const app = buildApp({ refreshOk: true });
    const { seen, callbacks } = collect();

    app.ai.streamChatApi({ message: '你好' }, callbacks);
    await sleep(10);
    expect(app.sentExpired(app.uni.requests[0])).toBe(true);

    app.respondSse401();
    await sleep(50);

    expect(app.refreshCalls.length).toBe(1);
    expect(app.uni.requests.length).toBe(1);
    expect(app.sentFresh(app.uni.requests[0])).toBe(true);

    app.respondSseOk();
    await sleep(30);

    expect(seen.done).toBe(1);
    expect(seen.errors).toEqual([]);
    expect(app.uni.relaunches.length).toBe(0);
  });

  test('S2: 续期真的失败 ⇒ onError 拿到统一文案，且登出恰好一次', async () => {
    const app = buildApp({ refreshOk: false });
    const { seen, callbacks } = collect();

    app.ai.streamChatApi({ message: '你好' }, callbacks);
    await sleep(10);
    app.respondSse401();
    await sleep(50);

    expect(app.refreshCalls.length).toBe(1);
    expect(seen.errors).toEqual([AUTH_EXPIRED]);
    expect(app.uni.relaunches.length).toBe(1);
    expect(app.uni.kv.has(KEY_TOKEN)).toBe(false);
  });

  test('S3: 重发仍 401 ⇒ 终态，不得触发第二次刷新', async () => {
    const app = buildApp({ refreshOk: true });
    const { seen, callbacks } = collect();

    app.ai.streamChatApi({ message: '你好' }, callbacks);
    await sleep(10);
    app.respondSse401();
    await sleep(50);
    app.respondSse401();
    await sleep(30);

    expect(app.refreshCalls.length).toBe(1);
    expect(seen.errors).toEqual([AUTH_EXPIRED]);
    expect(app.uni.relaunches.length).toBe(1);
  });
});
