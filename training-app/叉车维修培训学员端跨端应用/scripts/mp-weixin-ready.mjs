// ② 自动化会话「就绪闸门」——由 scripts/mp-weixin-check.ps1 在 `cli.bat auto` 之后、探针之前调用。
//
// 为什么需要它（2026-09-13 实测定位，勿删）：
//   `cli.bat auto` 返回、端口开始接受连接，**不等于**自动化会话可用。实测（本机
//   开发者工具 Stable v2.01.2510290 × miniprogram-automator@0.12.1）三个里程碑分得很开：
//     · 端口接受连接             t ≈ 1s        （auto 返回后仍会有一小段 ECONNREFUSED）
//     · Tool.getInfo 带 SDKVersion  t ≈ 1–3s   ← 早于此窗口时 result 里**只有 version、没有 SDKVersion**
//     · App.getPageStack 开始应答  t ≈ 24–34s  （两次实测 24s / 34s，波动大）
//   ② 门的两条红都是这两个里程碑造成的，且**都被误判成了「端口连不上」**：
//     1) `miniprogram-automator` 的 `Launcher.connect` 在 ws 打开后**立刻**调 `MiniProgram.checkVersion()`，
//        它读 `(await send('Tool.getInfo')).SDKVersion`；该字段缺失时 `licia/cmpVersion` 对 `undefined`
//        调 `.split('.')` ⇒ 抛 `Cannot read properties of undefined (reading 'split')`，
//        被 `connectTool` 的 catch 换成 “Failed connecting to …” 文案 ⇒ 看上去像端口问题。
//     2) 探针的 `mp.pageStack()` 超时原本取 30s，正好**落在 24–34s 这段就绪窗口中间** ⇒ 同一份代码
//        有时绿有时红。这不是竞态，是**就绪等待不足**。
//
// 因此本闸门在把端点交给 automator **之前**先用裸 ws 把两个里程碑等出来：
//   `Tool.getInfo` 必须带回 `SDKVersion`（否则 automator 一连就崩），`App.getPageStack` 必须开始应答
//   （否则探针的 pageStack 前置断言必挂）。它只读不写、用完即关。实测**不影响**随后的 automator 会话：
//   同一端口上先跑本闸门、再 `automator.connect()`，`pageStack=["pages/index/index"]` 正常。
//
// 用法：
//   node mp-weixin-ready.mjs --ws ws://127.0.0.1:9420 [--wait-seconds 120] [--require-stack]
//        [--require-root <装了 ws 的目录>] [--poll-timeout-ms 4000]
// 输出：最后一行固定为 `MP_WEIXIN_READY {json}`，由调用方解析——**只看输出，不看退出码**。
// 退出码：0 = 就绪；2 = 未就绪（超预算仍缺里程碑）。

import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420', waitSeconds: 120, requireStack: false,
    requireRoot: '', pollTimeoutMs: 4000,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const val = () => argv[++i];
    if (a === '--ws') out.ws = val();
    else if (a === '--wait-seconds') out.waitSeconds = Number(val());
    else if (a === '--require-root') out.requireRoot = val();
    else if (a === '--poll-timeout-ms') out.pollTimeoutMs = Number(val());
    else if (a === '--require-stack') out.requireStack = true;
  }
  return out;
}
const args = parseArgs(process.argv.slice(2));

// ws 来自调用方指定的 require 根（脚本把 automator 装到项目外，`ws` 是它的依赖）→
// 项目自身 node_modules → 本脚本上一级 → require 链（与探针同款解析顺序）。
function loadWs() {
  const tried = [];
  const roots = [];
  if (args.requireRoot) roots.push(args.requireRoot);
  roots.push(process.cwd());
  if (import.meta.dirname) roots.push(path.join(import.meta.dirname, '..'));
  for (const root of roots) {
    tried.push(root);
    try { return { mod: createRequire(path.join(root, 'noop.js'))('ws'), from: root }; } catch { /* 换下一个根 */ }
  }
  try { return { mod: createRequire(import.meta.url)('ws'), from: 'require-chain' }; } catch (e) {
    return { mod: null, error: String((e && e.message) || e), tried };
  }
}

const loaded = loadWs();
const result = {
  ws: args.ws, ok: false, reason: '',
  portReadyAt: null, sdkReadyAt: null, pageStackAt: null,
  ideVersion: '', sdkVersion: '', pageStack: null,
  elapsedMs: 0, timeline: [], requireStack: args.requireStack,
};
const started = Date.now();
const elapsed = () => Math.round((Date.now() - started) / 1000);

function finish() {
  result.elapsedMs = Date.now() - started;
  console.log('MP_WEIXIN_READY ' + JSON.stringify(result));
  process.exit(result.ok ? 0 : 2);
}

if (!loaded.mod) {
  result.reason = 'ws-unavailable';
  result.detail = '找不到 ws 模块（' + (loaded.error || '未知') + '）：先在调用方准备好 miniprogram-automator 依赖';
  result.triedRoots = loaded.tried || [];
  console.error('[error] ' + result.detail);
  finish();
}
const WebSocket = loaded.mod;
result.wsFrom = loaded.from;

function connectOnce() {
  const w = new WebSocket(args.ws);
  return new Promise((resolve) => {
    const t = setTimeout(() => { try { w.terminate(); } catch { /* 已关 */ } resolve(null); }, 3000);
    w.on('open', () => { clearTimeout(t); resolve(w); });
    w.on('error', () => { clearTimeout(t); try { w.terminate(); } catch { /* 已关 */ } resolve(null); });
  });
}

// 单条请求：按 id 收敛回复；IDE 也会推无 id 的事件帧（App.logAdded 之类），必须忽略。
// 结果三态：replied（有 result）/ error（IDE 明确报错，如 unimplemented）/ no-reply（超时未答）。
function ask(ws, id, method, params) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => { ws.off('message', h); resolve({ verdict: 'no-reply' }); }, args.pollTimeoutMs);
    const h = (data) => {
      let obj = null;
      try { obj = JSON.parse(String(data)); } catch { return; }
      if (!obj || obj.id !== id) return;
      clearTimeout(timer); ws.off('message', h);
      if (obj.error) resolve({ verdict: 'error', error: String(obj.error.message || obj.error) });
      else resolve({ verdict: 'replied', result: obj.result });
    };
    ws.on('message', h);
    try { ws.send(JSON.stringify({ id, method, params })); } catch (e) {
      clearTimeout(timer); ws.off('message', h);
      resolve({ verdict: 'no-reply', error: String((e && e.message) || e) });
    }
  });
}

// —— 里程碑 1：端口真的开始接受连接（auto 返回后仍有一段 ECONNREFUSED）——
let ws = null;
while (elapsed() <= args.waitSeconds) {
  ws = await connectOnce();
  if (ws) { result.portReadyAt = elapsed(); break; }
  console.log(`[ready] t=${elapsed()}s 端口尚未接受连接（auto 返回后端口是延迟出现的）`);
  await new Promise((r) => setTimeout(r, 1000));
}
if (!ws) {
  result.reason = 'port-not-connectable';
  result.detail = `等待 ${args.waitSeconds}s 仍无法连上 ${args.ws}（端口未监听 / 防火墙 / auto 未生效）`;
  console.error('[error] ' + result.detail);
  finish();
}
console.log(`[ready] t=${result.portReadyAt}s 端口已接受连接`);

// —— 里程碑 2/3：SDKVersion 出现（automator 一连就崩的那个字段）+ pageStack 开始应答 ——
let id = 1;
while (elapsed() <= args.waitSeconds) {
  const t = elapsed();
  const info = await ask(ws, id++, 'Tool.getInfo', {});
  const stack = await ask(ws, id++, 'App.getPageStack', {});
  const sdk = info.verdict === 'replied' && info.result ? info.result.SDKVersion : undefined;
  const ver = info.verdict === 'replied' && info.result ? info.result.version : undefined;
  const len = stack.verdict === 'replied' && stack.result
    ? (Array.isArray(stack.result.pageStack) ? stack.result.pageStack.length : null) : null;
  result.timeline.push({
    t, ideVersion: ver, sdkVersion: sdk, pageStack: len, getInfo: info.verdict, getPageStack: stack.verdict,
  });
  if (ver) result.ideVersion = ver;
  if (sdk) { result.sdkVersion = sdk; if (result.sdkReadyAt === null) result.sdkReadyAt = t; }
  if (len !== null && result.pageStackAt === null) result.pageStackAt = t;
  result.pageStack = len;
  console.log(`[ready] t=${t}s Tool.getInfo=${info.verdict} SDKVersion=${sdk || '(缺失)'} App.getPageStack=${stack.verdict} pageStack=${len === null ? '(未答)' : len}`);
  const stackOk = !args.requireStack || (len !== null && len > 0);
  if (sdk && stackOk) {
    result.ok = true;
    console.log(`[ready] 就绪：端口 t=${result.portReadyAt}s、SDKVersion t=${result.sdkReadyAt}s、pageStack t=${result.pageStackAt}s`);
    try { ws.close(); } catch { /* 已关 */ }
    finish();
  }
  // SDKVersion 的缺失是**短暂**的（实测 1–3s），不必等满再问；pageStack 未答则每次要等满超时，天然有间隔。
  await new Promise((r) => setTimeout(r, sdk ? 1000 : 300));
}

// 超预算：把「缺哪个里程碑」说清楚——这正是旧脚本含糊成「端口连不上」的地方
if (!result.sdkVersion) result.reason = 'sdk-version-missing';
else if (args.requireStack && !(result.pageStack > 0)) result.reason = 'page-stack-not-answering';
else result.reason = 'unknown';
result.detail = `等待 ${args.waitSeconds}s 未就绪（reason=${result.reason}；SDKVersion=${result.sdkVersion || '(缺失)'}，pageStack=${result.pageStack === null ? '(未答)' : result.pageStack}）`;
console.error('[error] ' + result.detail);
console.error('[error] 不要把这条当成「端口连不上」——端口在 t=' + result.portReadyAt + 's 就已接受连接。');
try { ws.close(); } catch { /* 已关 */ }
finish();
