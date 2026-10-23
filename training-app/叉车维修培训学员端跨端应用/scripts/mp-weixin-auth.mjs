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
// 两相（#1690 run12/run13）：--mode auth（缺省）= 注入会话给登录态页；--mode guest = **游客盖帽**给入口页。
//       入口页是 welcome 着陆页（index.uvue #1478：「已登录仍无条件 reLaunch dashboard」，业务不变量、
//       不得条件化）⇒ 有会话时其冷启动必落 dashboard 判 miss。guest 相用 setStorageSync 把 auth_user
//       写成非 JSON 盖帽（removeStorageSync 实测不跨 IDE 重启落盘、set 落盘——run13 实证不对称）：
//       应用启动 restoreFromStorage 走 uObj==null 分支 resetAuthState（stores/auth.uts:194，只动内存
//       不动 storage）⇒ welcome 页以游客态留在入口页。盖帽/注入都须发生在当页冷启动**之前**。
// 诚实降级：任何一步失败 ⇒ ok=false + kebab reason，由调用方记 SKIP（auth-session-unavailable），
//       绝不冒充取证成立。
// 输出：单行 `MP_WEIXIN_AUTH {json}`（调用方只看这行，不看退出码）。
//       json = {ok, status: existing|injected|cleared|none, user, reason, detail, elapsedMs} —— 不含令牌。
// 红线：不调 mp.close()（会话留给后续探针，同 mp-weixin-probe.mjs 纪律）；单步全部带超时
//       （冷启动后 app 侧未热时 callWxMethod 会「timeout waiting for automator response」，重试而不是挂死）。

import path from 'node:path';
import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420',
    requireRoot: '',
    timeoutMs: 90000,
    api: 'https://api.gdyidao.cn/api',
    stepTimeoutMs: 15000,
    attempts: 3,
    retryDelayMs: 4000,
    flushSettleMs: 45000, // 无 WeappStorage 目录时的盲等兜底
    flushTimeoutMs: 90000, // 落盘确认轮询上限（#1690 run13：写不确认就 quit ⇒ 重启后原样复活）
    flushIntervalMs: 2000,
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
  const GUEST_CAP = 'gate-guest-invalid-json'; // 非 JSON ⇒ getStorageJSON 走 catch 返 null（utils/storage.uts:44）

  // 0) guest 相（#1690 run13 终版）：游客盖帽给入口页。
  //    实证口径（cap-probe 实验）：callWxMethod 的 storage 写（含 REMOVE）只活 app 内存，IDE 异步刷盘、
  //    quit 不冲刷 ⇒ 删除会「复活」，写盖帽也必须**落盘确认**后才准进入冷启动。盖帽值含唯一时间戳，
  //    盘上出现该值 = 本轮写入已刷盘。auth_user 非 JSON ⇒ 应用启动 restoreFromStorage 判无用户 ⇒
  //    resetAuthState（#1124 纪律：只动内存不动 storage）⇒ 入口 welcome 页停游客态（#1478 不变量）。
  if (args.mode === 'guest') {
    try {
      const capVal = GUEST_CAP + '-' + Date.now(); // 唯一值：落盘轮询不被上一轮旧盖帽假确认
      await wxRetry(mp, 'setStorageSync', USER_KEY, capVal);
      await wxRetry(mp, 'setStorageSync', TOKEN_KEY, '');
      const uCap = String((await wxRetry(mp, 'getStorageSync', USER_KEY)) ?? '');
      if (uCap !== capVal) {
        finish({ ok: false, status: 'none', reason: 'storage-clear-failed', detail: 'guest cap not in place: readback=' + uCap.slice(0, 40) });
      }
      const fc = await flushConfirm(capVal); // 盖帽必须活过紧随其后的 quit ⇒ 落盘确认后才放行
      if (fc === 'timeout') {
        finish({ ok: false, status: 'none', reason: 'storage-flush-timeout', detail: 'guest cap not flushed to disk in ' + args.flushTimeoutMs + 'ms' });
      }
      finish({ ok: true, status: 'cleared', user: '', reason: '', detail: 'guest phase: auth_user capped, token emptied, flush=' + fc });
    } catch (e) {
      finish({ ok: false, status: 'none', reason: 'storage-clear-failed', detail: String((e && e.message) || e).slice(0, 200) });
    }
  }

  // 落盘确认（#1690 run13 实证）：storage 写只活 app 内存，IDE 以未公开节律异步刷盘、quit 不冲刷。
  // 判据：写含唯一时间戳的标记值，轮询 DevTools User Data/*/WeappSimulator/WeappStorage/storage_*.json
  // 直到标记出现在盘上（只读观测，不写 IDE 内部文件）。超时 ⇒ 诚实报 storage-flush-timeout。
  async function flushConfirm(marker) {
    const base = process.env.LOCALAPPDATA || '';
    const ud = base ? path.join(base, '微信开发者工具', 'User Data') : '';
    if (!ud || !existsSync(ud)) {
      await sleep(args.flushSettleMs); // 非默认安装面：盲等兜底（detail 记 settled-blind，不瞒）
      return 'settled-blind';
    }
    const deadline = Date.now() + args.flushTimeoutMs;
    for (;;) {
      try {
        for (const hash of readdirSync(ud)) {
          const dir = path.join(ud, hash, 'WeappSimulator', 'WeappStorage');
          let fl = [];
          try { fl = readdirSync(dir); } catch { continue; }
          for (const f of fl) {
            if (!f.startsWith('storage_') || !f.endsWith('.json')) continue;
            try {
              if (readFileSync(path.join(dir, f), 'utf8').includes(marker)) return 'flushed';
            } catch { /* 正被 IDE 重写 ⇒ 下一轮 */ }
          }
        }
      } catch { /* 目录竞态 ⇒ 下一轮 */ }
      if (Date.now() > deadline) return 'timeout';
      await sleep(args.flushIntervalMs);
    }
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
  // 6) 落盘确认（#1690 run13）：注入若没刷盘，冷启动重启后令牌丢失 ⇒ 本页白跑。
  //    stamp 键 = 门自有、应用不消费；值含唯一时间戳，盘上出现即证明本轮写入已随之落盘。
  const stamp = 'fs-' + Date.now();
  try {
    await wxRetry(mp, 'setStorageSync', 'gate_flush_stamp', stamp);
  } catch { /* stamp 写失败 ⇒ 走 flush timeout 路径，不另立错误 */ }
  const fc = await flushConfirm(stamp);
  if (fc === 'timeout') {
    finish({ ok: false, status: 'none', reason: 'storage-flush-timeout', detail: 'injection not flushed to disk in ' + args.flushTimeoutMs + 'ms; ' + apiDetail });
  }
  finish({ ok: true, status: 'injected', user: String(user.user_id), reason: '', detail: 'wx-login+storage ok, flush=' + fc + ' ' + apiDetail });
}

main().catch((e) => finish({ ok: false, status: 'none', reason: 'exception', detail: String((e && e.stack) || e).slice(0, 300) }));
