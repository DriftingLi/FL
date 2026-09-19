/**
 * #1124 并发 401 单飞刷新 —— 接线契约（源码文本断言，守「行为测试自己搭依赖」看不到的接线）
 *
 * 与 `utils/concurrent401RefreshBehavior.test.js` 的分工：
 *   - 行为测试跑真的 `request.uts` / `auth.uts`（次数、resolve/reject、副作用），但它**自己注入依赖**
 *     ⇒ 接线断了它照样可能绿；
 *   - 本文件断言生产侧的**调用点与 key 清单**，证明接线没断。
 * 两个都要：一个证明「行为对」，一个证明「生产代码真的走那条路」。
 *
 * 守护（对应票面「修什么」步骤 1/2/3）：
 *   C1 刷新去重不再是「布尔量 + 把『别人在刷』折成 false」
 *   C2 登出只由**真实刷新失败**（或终态 401）触发，且跳转只此一处
 *   C3 `restoreFromStorage()` 判 storage 不可用时同步归零内存登录态
 *   C4「清理缓存」不丢 `auth_refresh_token` / `auth_login_provider`
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const AUTH_UTS = path.join(ROOT, 'stores', 'auth.uts');
const SETTINGS_UVUE = path.join(ROOT, 'pages', 'profile', 'settings.uvue');

const requestSrc = readText(REQUEST_UTS);
const gateSrc = readText(GATE_UTS);
const authSrc = readText(AUTH_UTS);
const settingsSrc = readText(SETTINGS_UVUE);

/** 提取函数体：从函数声明到顶层 `\n}`（先例：utils/quickLoginContract.test.js） */
function fnBody(src, name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

/** 提两个锚点之间的源码（用于 .uvue 里缩进的函数体） */
function between(src, from, to) {
  const start = src.indexOf(from);
  if (start === -1) throw new Error(`未找到锚点 ${from}`);
  const end = src.indexOf(to, start);
  if (end === -1) throw new Error(`未找到锚点 ${to}`);
  return src.slice(start, end);
}

describe('C1 并发刷新去重：共享 in-flight，而不是布尔量', () => {
  test('api/refreshGate.uts 存在，且并发时返回**同一个在飞 Promise**', () => {
    const body = fnBody(gateSrc, 'gateRefresh');
    expect(gateSrc).toMatch(/let\s+inflight\s*:\s*Promise<boolean>\s*\|\s*null\s*=\s*null/);
    // 关键语义：非空时**返回它**（共享结果），不是返回 false
    expect(body).toMatch(/if\s*\(\s*existing\s*!=\s*null\s*\)\s*return\s+existing/);
    expect(body).toContain('inflight = started');
    expect(fnBody(gateSrc, 'gateRefresh')).toContain('inflight = null'); // settle 后放行下一次
  });

  test('去重认 token 不认时间盒：以「触发 401 的那个 access token」并入同一次结果', () => {
    // 签名必须收 forToken（时间盒版本没有这个参数）
    expect(gateSrc).toMatch(/export\s+function\s+gateRefresh\s*\(\s*runner\s*:\s*RefreshRunner\s*,\s*forToken\s*:\s*string\s*\)/);
    const body = fnBody(gateSrc, 'gateRefresh');
    expect(body).toMatch(/forToken\.length\s*>\s*0\s*&&\s*forToken\s*==\s*settledForToken\s*&&\s*settled\s*!=\s*null/);
    // 时间盒（HOLD_MS / setTimeout）与代际号（generation）都是被否掉的实现，不得回来
    expect(gateSrc).not.toContain('HOLD_MS');
    expect(gateSrc).not.toContain('generation');
    expect(gateSrc).not.toContain('setTimeout');
  });

  test('request.uts 接上闸门：import 且把「本次实际发出的 token」传进去', () => {
    expect(requestSrc).toContain("import { gateRefresh } from './refreshGate'");
    const body = fnBody(requestSrc, 'tryAutoRefresh');
    expect(body).toContain('gateRefresh(_refreshTokenHandler!, forToken)');
    expect(fnBody(requestSrc, 'doSingleRequest')).toContain('tryAutoRefresh(sentTokenOf(reqHeader))');
    // sentTokenOf 必须从请求头取本次真正发出的 token（storage 里的可能已被刷新换掉）
    const picker = fnBody(requestSrc, 'sentTokenOf');
    expect(picker).toContain('reqHeader[HEADER_AUTHORIZATION]');
    expect(picker).toContain('BEARER_PREFIX');
  });

  test('回归钉死：`isRefreshing` 布尔量及其「别人在刷 ⇒ 返回 false」读法已删除', () => {
    expect(requestSrc).not.toContain('isRefreshing');
    const body = fnBody(requestSrc, 'tryAutoRefresh');
    expect(body).not.toMatch(/if\s*\(\s*isRefreshing\s*\)\s*return\s+false/);
    // false 只允许有一个来源：没有刷新回调（真失败的一种）
    const falseCount = (body.match(/return\s+Promise\.resolve\(false\)/g) || []).length;
    expect(falseCount).toBe(1);
    expect(body).toMatch(/if\s*\(\s*_refreshTokenHandler\s*==\s*null\s*\)\s*return\s+Promise\.resolve\(false\)/);
  });

  test('回归钉死：100ms 延迟复位标记已删除（去重不再靠时间盒）', () => {
    expect(requestSrc).not.toMatch(/setTimeout\(\(\)\s*=>\s*\{\s*isRefreshing\s*=\s*false/);
  });
});

describe('C2 登出只由真实失败触发', () => {
  test('401 分支：只有 `refreshed` 为真才重试，否则登出', () => {
    const body = fnBody(requestSrc, 'doSingleRequest');
    expect(body).toContain('if (refreshed && getStorage(STORAGE_KEY_TOKEN).length > 0)');
    const retryIdx = body.indexOf('doSingleRequest(url, method, data, buildHeader(options.header, silent), timeout, options, true, resolve, reject)');
    // 取**最后**一处登出：doSingleRequest 内的 else 分支（缩进不参与匹配，避免被 12/16 空格绊倒）
    const logoutIdx = body.lastIndexOf('logoutAndReject(silent, reject)');
    expect(retryIdx).toBeGreaterThan(-1);
    expect(logoutIdx).toBeGreaterThan(retryIdx);
  });

  test('终态判定不变：重试仍 401 / 刷新接口自身 401 / 无刷新回调 ⇒ 直接登出', () => {
    const body = fnBody(requestSrc, 'doSingleRequest');
    expect(body).toContain("if (isRetry || options.url.includes('/auth/refresh') || _refreshTokenHandler == null)");
  });

  test('跳转只有一个出口：handleUnauthorized 只被 logoutAndReject 调用', () => {
    const callers = (requestSrc.match(/handleUnauthorized\(\)/g) || []).length;
    // 1 次定义在 logoutAndReject 里调用 + 0 次定义声明本身（声明是 `function handleUnauthorized()`）
    expect(callers).toBe(2); // `function handleUnauthorized()` 声明 + logoutAndReject 里的调用
    expect(fnBody(requestSrc, 'logoutAndReject')).toContain('handleUnauthorized()');
  });
});

describe('C3 restoreFromStorage 在 storage 不可用时归零内存登录态', () => {
  test('resetAuthState 是单一重置点，clearAuthData 复用它', () => {
    expect(fnBody(authSrc, 'resetAuthState')).toContain("token.value = ''");
    expect(fnBody(authSrc, 'resetAuthState')).toContain('syncDerived()');
    expect(fnBody(authSrc, 'clearAuthData')).toContain('resetAuthState()');
  });

  test('两个提前 return 分支都先归零内存（否则登录页会拿陈旧 isLoggedIn 弹回首页）', () => {
    const body = fnBody(authSrc, 'restoreFromStorage');
    expect(body).not.toMatch(/if\s*\(\s*t\.length\s*==\s*0\s*\)\s*\{\s*return\s+false\s*\}/);
    expect(body).not.toMatch(/if\s*\(\s*uObj\s*==\s*null\s*\)\s*\{\s*return\s+false\s*\}/);
    const resets = (body.match(/resetAuthState\(\)\s*\n?\s*return false/g) || []).length;
    expect(resets).toBe(2);
  });

  test('只重置内存、不动 storage：登出后 refresh_token 仍留给生物识别快捷登录（ADR-0004）', () => {
    const body = fnBody(authSrc, 'restoreFromStorage');
    expect(body).not.toContain('removeStorage(');
  });
});

describe('C4 清理缓存不削掉续期能力', () => {
  test('settings.uvue 的恢复清单含 refresh_token 与 login_provider', () => {
    const body = between(settingsSrc, 'function onClearCache', 'function onFeature');
    const restoreBlock = body.slice(body.indexOf('uni.clearStorageSync()'));
    expect(restoreBlock).toContain('setStorageSync(STORAGE_KEY_TOKEN');
    expect(restoreBlock).toContain('setStorageSync(STORAGE_KEY_USER');
    expect(restoreBlock).toContain('setStorageSync(STORAGE_KEY_REFRESH_TOKEN');
    expect(restoreBlock).toContain('setStorageSync(STORAGE_KEY_LOGIN_PROVIDER');
    expect(restoreBlock).toContain("setStorageSync('care_mode'");
  });

  test('所需 key 常量都已 import（否则 Kotlin 侧 error18 找不到名称）', () => {
    const importLine = settingsSrc.match(/import \{[^}]*\} from '\.\.\/\.\.\/constants\/app'/);
    expect(importLine).not.toBe(null);
    ['STORAGE_KEY_TOKEN', 'STORAGE_KEY_REFRESH_TOKEN', 'STORAGE_KEY_USER', 'STORAGE_KEY_LOGIN_PROVIDER'].forEach((k) => {
      expect(importLine[0]).toContain(k);
    });
  });
});

describe('C5 已登出时不得用遗留的 refresh_token 悄悄复活会话', () => {
  test('tryRefreshToken 先判内存 user 为空即拒绝，之后才可以用 user.value!', () => {
    const body = fnBody(authSrc, 'tryRefreshToken');
    const guardIdx = body.indexOf('if (user.value == null) return false');
    const derefIdx = body.indexOf('setAuthData(result.token, user.value!');
    expect(guardIdx).toBeGreaterThan(-1);
    expect(derefIdx).toBeGreaterThan(guardIdx);
  });
});
