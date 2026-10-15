// mp-weixin-auth.mjs — ② 门 DevTools 会话注入（#1690）
// 目的：直达探针（#1682）冷启动登录态页时，微信开发者工具里没有登录会话 ⇒ 首屏 401 ⇒
//       handleUnauthorized reLaunch 到 /pages/login/login ⇒ pageStack[0] ≠ 目标页 ⇒ reached=False。
// 机制：用 automator 的 MiniProgram.callWxMethod 在**门运行环境**里走一遍真实登录链路：
//       wx.login 取 code → POST {API}/auth/wx-login 换双令牌 → wx.setStorageSync 写回与
//       stores/auth.uts setAuthData 完全同键同形的 4 个凭据（auth_token / auth_refresh_token /
//       auth_user(JSON) / auth_login_provider）。**不改任何业务代码**：注入只发生在开发者工具
//       的 storage 里，app 冷启动后 restoreFromStorage 照常消费（access 过期还有 refresh_token 续期兜底）。
// 幂等：先读现状 —— token 与 user 都在 ⇒ status=existing 直接返回（每页冷启动前都会调本脚本，
//       重复调用必须便宜）。实测 storage 跨 IDE quit→reopen 持久（#1690 前置实验）。
// 两相（#1690 run12）：--mode auth（缺省）= 注入会话给登录态页；--mode guest = **清会话**给入口页。
//       入口页是 welcome 着陆页（index.uvue #1478：「已登录仍无条件 reLaunch dashboard」，业务不变量、
//       不得条件化）⇒ 有会话时其冷启动必落 dashboard 判 miss。guest 相 removeStorageSync 四键 + 读回
//       确认 ⇒ status=cleared。清/注入都须发生在当页冷启动**之前**（冷启动后 app 已按旧 storage 落地）。
// 诚实降级：任何一步失败 ⇒ ok=false + kebab reason，由调用方记 SKIP（auth-session-unavailable），
//       绝不冒充取证成立。
// 输出：单行 `MP_WEIXIN_AUTH {json}`（调用方只看这行，不看退出码）。
//       json = {ok, status: existing|injected|cleared|none, user, reason, detail, elapsedMs} —— 不含令牌。
// 红线：不调 mp.close()（会话留给后续探针，同 mp-weixin-probe.mjs 纪律）；单步全部带超时
//       （冷启动后 app 侧未热时 callWxMethod 会「timeout waiting for automator response」，重试而不是挂死）。

import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420',
    requireRoot: '',
    timeoutMs: 90000,
    api: 'https://www.gccsmile.com/api',
    stepTimeoutMs: 15000,
    attempts: 3,
    retryDelayMs: 4000,
    mode: 'auth',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const val = () => argv[++i];
    if (a === '--ws') out.ws = val();
    else if (a === '--require-root') out.requireRoot = val();
    else if (a === '--timeout-ms') out.timeoutMs = Number(val());
    else if (a === '--api') out.api = String(val() || '').replace(/\/+$/, '');
    else if (a === '--step-timeout-ms') out.stepTimeoutMs = Number(val());
    else if (a === '--attempts') out.attempts = Number(val());
    else if (a === '--mode') out.mode = String(val()) === 'guest' ? 'guest' : 'auth';
  }
  return out;
}
const args = parseArgs(process.argv.slice(2));

const startedAt = Date.now();
const deadline = startedAt + args.timeoutMs;

function withTimeout(promise, ms, label) {
  return Promise.race([
    Promise.resolve(promise),
    new Promise((_, rej) => {
      const t = setTimeout(() => rej(new Error('TIMEOUT(' + ms + 'ms) @ ' + label)), ms);
      if (typeof t.unref === 'function') t.unref();
    }),
  ]);
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function loadAutomator() {
  // 解析顺序与 mp-weixin-probe.mjs 同源：--require-root → cwd → 脚本上级 → require 链
  const roots = [];
  if (args.requireRoot) roots.push(args.requireRoot);
  roots.push(process.cwd());
  if (import.meta.dirname) roots.push(path.join(import.meta.dirname, '..'));
  for (const root of roots) {
    try {
      return { mod: createRequire(path.join(root, 'noop.js'))('miniprogram-automator'), from: root };
    } catch { /* 换下一个根 */ }
  }
  try {
    return { mod: createRequire(import.meta.url)('miniprogram-automator'), from: 'require-chain' };
  } catch (e) {
    return { mod: null, error: String((e && e.message) || e), triedRoots: roots };
  }
}

function finish(obj) {
  const payload = { ...obj, elapsedMs: Date.now() - startedAt };
  console.log('MP_WEIXIN_AUTH ' + JSON.stringify(payload));
  process.exit(obj.ok ? 0 : 1);
}

// callWxMethod 重试壳：冷启动后 app 侧服务未热 ⇒ 「timeout waiting for automator response」是常态而非环境坏
async function wxRetry(mp, method, ...callArgs) {
  let lastErr = '';
  for (let i = 1; i <= args.attempts; i += 1) {
    if (Date.now() > deadline) break;
    try {
      return await withTimeout(mp.callWxMethod(method, ...callArgs), args.stepTimeoutMs, method);
    } catch (e) {
      lastErr = String((e && e.message) || e);
      if (i < args.attempts) await sleep(args.retryDelayMs);
    }
  }
  throw new Error(method + ' failed after retries: ' + lastErr.slice(0, 160));
}

async function main() {
  const { mod: automator, from, error, triedRoots } = loadAutomator();
  if (!automator) {
    finish({ ok: false, status: 'none', reason: 'automator-missing', detail: String(error || '').slice(0, 200) + ' roots=' + JSON.stringify(triedRoots) });
  }
  let mp = null;
  try {
    mp = await withTimeout(automator.connect({ wsEndpoint: args.ws }), args.stepTimeoutMs, 'connect');
  } catch (e) {
    finish({ ok: false, status: 'none', reason: 'connect-failed', detail: String((e && e.message) || e).slice(0, 200) });
  }

  const TOKEN_KEY = 'auth_token'; // 键名与 constants/app.uts 存储键一一对应
  const USER_KEY = 'auth_user';
  const REFRESH_KEY = 'auth_refresh_token';
  const PROVIDER_KEY = 'auth_login_provider';

  // 0) guest 相（#1690 run12）：清会话给入口页（welcome 页有会话必弹 dashboard，见文件头）。
  //    remove 不存在的键可能报错 ⇒ 逐键吞错，以读回为准判「已清」。
  if (args.mode === 'guest') {
    for (const k of [TOKEN_KEY, REFRESH_KEY, USER_KEY, PROVIDER_KEY]) {
      try { await wxRetry(mp, 'removeStorageSync', k); } catch { /* 键本就不存在 ⇒ 缺席即游客态 */ }
    }
    let tGone = '';
    let uGone = '';
    try {
      tGone = String((await wxRetry(mp, 'getStorageSync', TOKEN_KEY)) ?? '');
      uGone = String((await wxRetry(mp, 'getStorageSync', USER_KEY)) ?? '');
    } catch (e) {
      finish({ ok: false, status: 'none', reason: 'storage-clear-failed', detail: String((e && e.message) || e).slice(0, 200) });
    }
    if (tGone.length > 20 || uGone.length > 2) {
      finish({ ok: false, status: 'none', reason: 'storage-clear-failed', detail: 'token/user still present after removeStorageSync' });
    }
    finish({ ok: true, status: 'cleared', user: '', reason: '', detail: 'guest phase: session keys removed' });
  }

  // 1) 现状检查（幂等便宜路径）
  let token = '';
  let userRaw = '';
  try {
    token = String((await wxRetry(mp, 'getStorageSync', TOKEN_KEY)) ?? '');
    userRaw = String((await wxRetry(mp, 'getStorageSync', USER_KEY)) ?? '');
  } catch (e) {
    finish({ ok: false, status: 'none', reason: 'storage-read-failed', detail: String((e && e.message) || e).slice(0, 200) });
  }
  if (token.length > 20 && userRaw.length > 2) {
    let uid = '';
    try { uid = String(JSON.parse(userRaw).user_id ?? ''); } catch { uid = ''; }
    if (uid) {
      finish({ ok: true, status: 'existing', user: uid, reason: '', detail: 'session already in devtools storage' });
    }
  }

  // 2) wx.login 取 code（IDE 已登录 + 项目 appid 有效才有 code）
  let code = '';
  try {
    const lr = await wxRetry(mp, 'login');
    code = String((lr && lr.code) || '');
  } catch (e) {
    finish({ ok: false, status: 'none', reason: 'no-login-code', detail: String((e && e.message) || e).slice(0, 200) });
  }
  if (!code) {
    finish({ ok: false, status: 'none', reason: 'no-login-code', detail: 'wx.login returned empty code' });
  }

  // 3) 后端换令牌（与 api/auth.uts mpWechatLoginApi 同一端点同一形状）
  let data = null;
  let apiDetail = '';
  try {
    const res = await withTimeout(fetch(args.api + '/auth/wx-login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code }),
    }), args.stepTimeoutMs, 'wx-login-fetch');
    const body = await res.json().catch(() => null);
    apiDetail = 'http=' + res.status + ' biz=' + (body && body.code);
    if (res.ok && body && body.code === 200 && body.data && body.data.token) {
      data = body.data;
    }
  } catch (e) {
    apiDetail = String((e && e.message) || e).slice(0, 160);
  }
  if (!data) {
    finish({ ok: false, status: 'none', reason: 'wx-login-api-failed', detail: apiDetail });
  }

  // 4) 写回凭据（形状对齐 stores/auth.uts setAuthData + api/auth.uts buildUserInfo：
  //    auth_user = {user_id, username, name, role, avatar_url}，name 缺省取 username，
  //    avatar 兼容 /auth/wx-login 的 avatar 字段名）
  const user = {
    user_id: data.user_id,
    username: data.username ?? '',
    name: data.name ?? data.username ?? '',
    role: data.role ?? '',
    avatar_url: data.avatar_url || data.avatar || '',
  };
  try {
    await wxRetry(mp, 'setStorageSync', TOKEN_KEY, String(data.token));
    if (data.refresh_token) await wxRetry(mp, 'setStorageSync', REFRESH_KEY, String(data.refresh_token));
    await wxRetry(mp, 'setStorageSync', USER_KEY, JSON.stringify(user));
    await wxRetry(mp, 'setStorageSync', PROVIDER_KEY, 'wechat');
  } catch (e) {
    finish({ ok: false, status: 'none', reason: 'set-storage-failed', detail: String((e && e.message) || e).slice(0, 200) });
  }

  // 5) 读回复验（callWxMethod 返回值形状 = 原始值本身，非 {data} 包）
  let rt = '';
  let ru = '';
  try {
    rt = String((await wxRetry(mp, 'getStorageSync', TOKEN_KEY)) ?? '');
    ru = String((await wxRetry(mp, 'getStorageSync', USER_KEY)) ?? '');
  } catch (e) {
    finish({ ok: false, status: 'none', reason: 'readback-mismatch', detail: String((e && e.message) || e).slice(0, 200) });
  }
  let uidOk = false;
  try { uidOk = String(JSON.parse(ru).user_id) === String(user.user_id); } catch { uidOk = false; }
  if (rt !== String(data.token) || !uidOk) {
    finish({ ok: false, status: 'none', reason: 'readback-mismatch', detail: apiDetail });
  }
  finish({ ok: true, status: 'injected', user: String(user.user_id), reason: '', detail: 'wx-login+storage ok ' + apiDetail });
}

main().catch((e) => finish({ ok: false, status: 'none', reason: 'exception', detail: String((e && e.stack) || e).slice(0, 300) }));
