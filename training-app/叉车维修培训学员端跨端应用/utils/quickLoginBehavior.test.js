/**
 * #1398 快捷登录成功信号 —— **行为级**守护（真跑 `stores/auth.uts` 的 `quickLogin`）
 *
 * 病根（票面第 2 条）：`quickLogin` 旧实现以 `result.refresh_token` 非空作返回值、调用方据此判成功。
 * 而 `loginApi` 的映射 `toStr(data['refresh_token'], '')` 显式容忍后端不下发 rt ⇒
 * 「登录成功但 rt 为空」时 `setAuthData` 已落盘（会话已建立），返回却是空串（假失败），
 * UI 会停在登录页再弹认证框（「已登录却停在登录页」的错配）。
 *
 * 裁定（#1398）：成功信号 = 会话建立（access token 落地），rt 只作回写值。
 * 这里跑真的 store.quickLogin 证明：rt 为空仍返回 true 且 access token 落盘。
 */
const path = require('path');

/** 真跑 stores/auth.uts（utsHarness 去类型当 JS 执行） */
const { loadUts } = require('./utsHarness');
const AUTH_UTS = path.join(__dirname, '..', 'stores', 'auth.uts');

const KEY_TOKEN = 'auth_token';
const KEY_REFRESH = 'auth_refresh_token';
const KEY_USER = 'auth_user';
const KEY_PROVIDER = 'auth_login_provider';

/**
 * 装配一棵 store：loginApi 由用例给定，storage 记录写入，其余依赖给占位。
 * @param {Function} loginApiImpl 覆盖 `loginApi`（返回 LoginResult 或 reject）
 */
function buildStore(loginApiImpl) {
  const writes = {}; // storage 快照：key → 值
  const updateSecureTokenCalls = [];
  const mod = loadUts(AUTH_UTS, {
    ref: (v) => ({ value: v }),
    loginApi: loginApiImpl,
    phoneLoginApi: () => Promise.resolve(null),
    emailLoginApi: () => Promise.resolve(null),
    mpWechatLoginApi: () => Promise.resolve(null),
    logoutApi: () => Promise.resolve({}),
    getUserInfoApi: () => Promise.resolve(null),
    refreshTokenApi: () => Promise.resolve(null),
    recruiterLoginApi: () => Promise.resolve(null),
    registerRefreshTokenHandler: () => {},
    setStorage: (k, v) => { writes[k] = v; },
    setStorageJSON: (k, v) => { writes[k] = v; },
    getStorage: (k) => (writes[k] === undefined ? '' : writes[k]),
    getStorageJSON: (k) => writes[k],
    removeStorage: (k) => { delete writes[k]; },
    updateSecureToken: (rt) => { updateSecureTokenCalls.push(rt); },
    setActiveRole: () => {},
    clearActiveRole: () => {},
    errMsg: (_e, fallback) => fallback,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    STORAGE_KEY_REFRESH_TOKEN: KEY_REFRESH,
    STORAGE_KEY_USER: KEY_USER,
    STORAGE_KEY_LOGIN_PROVIDER: KEY_PROVIDER,
    STORAGE_KEY_CREDENTIALS: 'auth_secure_credentials',
    ACTIVE_ROLE_RECRUITER: 'recruiter',
  });
  return { store: mod.useAuthStore(), writes, updateSecureTokenCalls };
}

const USER = { user_id: 7, username: 'u7', name: '同学', role: 'student' };

describe('quickLogin 成功信号（#1398：与会话建立绑定、与 rt 解绑）', () => {
  test('登录成功但后端不下发 rt ⇒ 会话已建立即返回 true，access token 落盘', async () => {
    const { store, writes } = buildStore(() =>
      Promise.resolve({ token: 'ACCESS', refresh_token: '', user: USER })
    );
    const ok = await store.quickLogin('u7', 'pw');
    expect(ok).toBe(true); // 旧实现此处返回 ''（假失败）—— 这就是要修的错配
    expect(writes[KEY_TOKEN]).toBe('ACCESS'); // 会话确实建立了
  });

  test('登录成功且下发 rt ⇒ true，rt 照常被回写（既有单一同步点不回退）', async () => {
    const { store, writes, updateSecureTokenCalls } = buildStore(() =>
      Promise.resolve({ token: 'ACCESS', refresh_token: 'ROTATED_RT', user: USER })
    );
    const ok = await store.quickLogin('u7', 'pw');
    expect(ok).toBe(true);
    expect(writes[KEY_TOKEN]).toBe('ACCESS');
    expect(updateSecureTokenCalls).toContain('ROTATED_RT');
  });

  test('真实失败（loginApi 抛错）⇒ false，调用方据此降级回填', async () => {
    const { store } = buildStore(() => Promise.reject(new Error('账号或密码错误')));
    const ok = await store.quickLogin('u7', 'pw');
    expect(ok).toBe(false);
  });

  test('返回体缺 token（会话没建立）⇒ false，不能把「没登录」报成成功', async () => {
    const { store, writes } = buildStore(() =>
      Promise.resolve({ token: '', refresh_token: 'RT', user: USER })
    );
    const ok = await store.quickLogin('u7', 'pw');
    expect(ok).toBe(false);
    expect(writes[KEY_TOKEN]).toBeUndefined(); // 没落 token
  });
});
