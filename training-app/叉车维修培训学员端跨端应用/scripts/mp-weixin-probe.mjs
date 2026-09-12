// 微信开发者工具自动化探针（② 半自动门的取证核心）——由 scripts/mp-weixin-check.ps1 调用。
//
// 背景：docs/adr/0008-移动端验收门与证据.md；spike issue #883 的三条实测坑位（勿删）：
//   1. `pageStack` 为空会让**所有 page 级 API** 报同一个误导弹错
//      `Cannot destructure property 'rawPath' of 't.getPageMetaByWebviewId(...)' as it is null`
//      —— 根因是自动化会话里没有页面被打开，不是版本不兼容。处置 = 调用方先 `cli.bat close` 再 `cli.bat auto`。
//   2. `mp.screenshot()` 挂在 **MiniProgram** 上，不在 Page 上（`page.screenshot is not a function`）；
//      它**不返回内容**，产物只能靠落盘文件取（本脚本读 PNG IHDR 自证）。
//   3. `page.$()` 元素级断言在当前版本组合下**不可用**（实测挂起 15s 超时）⇒ 本探针只用
//      **page 级**导航（`mp.reLaunch` / `mp.navigateTo`，注意都在 MiniProgram 上）+ console/exception 读取，
//      **不得**引入元素级断言。
//
// 用法：
//   node mp-weixin-probe.mjs --ws ws://127.0.0.1:9420 --routes pages/index/index,pages/login/login \
//        --entry-url pages/index/index --shot-dir <dir> [--timeout-ms 30000] [--page-timeout-ms 10000]
// 输出：最后一行固定为 JSON（前缀 `MP_WEIXIN_PROBE `），由调用方解析 —— **只看输出，不看退出码**。
// 退出码：0 = 断言全过；1 = 有断言未过（真报错）；2 = 环境不可用（连不上 / 缺 automator / pageStack 空）。
// 说明：断言结果同时写进 JSON 的 assertions/failures/probeOk，调用方以 JSON 为准。

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420', routes: [], shotDir: '', entryUrl: '', requireRoot: '',
    timeoutMs: 30000, pageTimeoutMs: 10000, settleMs: 2000, tailMax: 40,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const val = () => argv[++i];
    if (a === '--ws') out.ws = val();
    else if (a === '--routes') out.routes = String(val() || '').split(',').map((s) => s.trim()).filter(Boolean);
    else if (a === '--shot-dir') out.shotDir = val();
    else if (a === '--entry-url') out.entryUrl = val();
    else if (a === '--require-root') out.requireRoot = val();
    else if (a === '--timeout-ms') out.timeoutMs = Number(val());
    else if (a === '--page-timeout-ms') out.pageTimeoutMs = Number(val());
  }
  return out;
}
const args = parseArgs(process.argv.slice(2));

// 单步等待不得无限挂起（#883：元素级 API 会静默挂 15s）
function withTimeout(promise, ms, label) {
  return Promise.race([
    Promise.resolve(promise),
    new Promise((_, rej) => {
      const t = setTimeout(() => rej(new Error('TIMEOUT(' + ms + 'ms) @ ' + label)), ms);
      if (typeof t.unref === 'function') t.unref();
    }),
  ]);
}

function loadAutomator() {
  // 解析顺序：调用方指定的 require 根（脚本把 automator 装到项目外，避免动 package-lock）→
  // 项目自身 node_modules → 探针所在目录上一级 → require 链
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

function pngInfo(file) {
  try {
    const fd = fs.openSync(file, 'r');
    const buf = Buffer.alloc(24);
    fs.readSync(fd, buf, 0, 24, 0);
    fs.closeSync(fd);
    const sig = buf.subarray(0, 8).toString('hex');
    return {
      path: file, bytes: fs.statSync(file).size, sig, isPng: sig === '89504e470d0a1a0a',
      width: buf.readUInt32BE(16), height: buf.readUInt32BE(20),
    };
  } catch (e) {
    return { path: file, error: String((e && e.message) || e) };
  }
}

// console 消息在 automator 里是**原始 JSON 帧**（不是 Element）：直接取 type/args，兼容两种 args 形态
function logEntry(msg) {
  const raw = msg || {};
  const type = typeof raw.type === 'function' ? raw.type() : raw.type;
  const rawArgs = typeof raw.args === 'function' ? raw.args() : raw.args;
  const list = Array.isArray(rawArgs) ? rawArgs : [];
  return {
    type: String(type || 'unknown'),
    args: list.map((a) => {
      if (a && typeof a.value === 'function') { try { return a.value(); } catch { return String(a); } }
      if (a && typeof a === 'object' && 'value' in a) return a.value;
      return a;
    }).map((v) => (typeof v === 'string' ? v.slice(0, 300) : v)),
  };
}

const rel = (p) => String(p || '').replace(/^\//, '');
const started = Date.now();
const result = {
  ws: args.ws, connected: false, automatorFrom: '', routes: args.routes,
  pageStack: [], entryPage: '', steps: [], shots: [],
  logsTotal: 0, errorsTotal: 0, exceptionsTotal: 0, consoleTail: [], elapsedMs: 0,
  assertions: {}, failures: [],
};

function finish(envUnavailable) {
  result.elapsedMs = Date.now() - started;
  const stack = result.pageStack;
  result.assertions = {
    connected: result.connected === true,
    // 前置断言：pageStack 非空（空栈会让 page 级 API 报完全误导的错，见文件头坑位 1）
    pageStackNonEmpty: stack.length > 0,
    entryPageInStack: Boolean(result.entryPage) && stack.map(rel).includes(rel(result.entryPage)),
    allRoutesVisited: args.routes.length > 0 && args.routes.every((r) => stack.map(rel).includes(rel(r))),
    screenshotsProduced: args.routes.length > 0 && result.shots.filter((s) => s.isPng).length >= args.routes.length,
    noConsoleErrors: result.errorsTotal === 0,
    noExceptions: result.exceptionsTotal === 0,
  };
  Object.entries(result.assertions).forEach(([k, v]) => {
    if (!v) result.failures.push('断言未过：' + k);
  });
  const probeOk = result.failures.length === 0;
  console.log('MP_WEIXIN_PROBE ' + JSON.stringify({ ...result, probeOk }));
  process.exit(probeOk ? 0 : envUnavailable ? 2 : 1);
}

const loaded = loadAutomator();
if (!loaded.mod) {
  result.failures.push('环境不可用：找不到 miniprogram-automator（' + (loaded.error || '未知') + '）');
  result.triedRoots = loaded.triedRoots || [];
  console.error('[error] ' + result.failures[0]);
  finish(true);
}
result.automatorFrom = loaded.from;
const automator = loaded.mod;

let mp;
try {
  mp = await withTimeout(automator.connect({ wsEndpoint: args.ws }), args.timeoutMs, 'automator.connect');
  result.connected = true;
} catch (e) {
  result.failures.push('环境不可用：自动化端口连不上（' + String((e && e.message) || e) + '）');
  console.error('[error] ' + result.failures[0]);
  finish(true);
}

// console / exception 全程收集：必须在任何导航之前挂上，否则漏掉入口页的报错
mp.on('console', (msg) => {
  const entry = logEntry(msg);
  if (entry.type === 'error') result.errorsTotal += 1;
  result.logsTotal += 1;
  if (result.consoleTail.length < args.tailMax) result.consoleTail.push(entry);
});
mp.on('exception', (e) => {
  result.exceptionsTotal += 1;
  if (result.consoleTail.length < args.tailMax) {
    result.consoleTail.push({ type: 'exception', args: [String((e && e.message) || e).slice(0, 300)] });
  }
});

if (args.shotDir && !fs.existsSync(args.shotDir)) fs.mkdirSync(args.shotDir, { recursive: true });

// pageStack 前置断言：读到空栈直接判「环境不可用」（exit 2），不继续做 page 级 API
try {
  const stack = await withTimeout(mp.pageStack(), args.timeoutMs, 'mp.pageStack');
  result.pageStack = (stack || []).map((p) => rel(p.path));
} catch (e) {
  result.failures.push('pageStack 读取失败：' + String((e && e.message) || e));
}
if (result.pageStack.length === 0) {
  result.failures.push('环境不可用：pageStack 为空（自动化会话没有打开页面）。处置 = 先 `cli.bat close` 再 `cli.bat auto`（#883 坑位 1）。');
  console.error('[error] ' + result.failures[result.failures.length - 1]);
  finish(true);
}
result.entryPage = result.pageStack[0];

// 逐页：第一个用 reLaunch（当入口页），其余用 navigateTo（当被改页）
for (let i = 0; i < args.routes.length; i += 1) {
  const route = rel(args.routes[i]);
  const label = i === 0 ? 'entry' : 'page-' + i;
  const step = { label, route, navigate: i === 0 ? 'reLaunch' : 'navigateTo', ok: false };
  try {
    const page = await withTimeout(
      i === 0 ? mp.reLaunch(route) : mp.navigateTo(route),
      args.timeoutMs, step.navigate + '(' + route + ')',
    );
    step.currentPage = rel(page && page.path);
    step.ok = step.currentPage === route;
    if (!step.ok) step.error = '导航后当前页不是目标页';
    if (args.shotDir) {
      const shotFile = path.join(args.shotDir, label + '.png');
      await withTimeout(mp.screenshot({ path: shotFile }), args.timeoutMs, 'mp.screenshot(' + label + ')');
      if (fs.existsSync(shotFile)) {
        const info = pngInfo(shotFile);
        result.shots.push({ name: label, route: step.currentPage, ...info });
        step.shot = shotFile;
      } else {
        step.shotError = 'screenshot 未落盘（该 API 不返回内容，只能看文件）';
      }
    }
  } catch (e) {
    step.error = String((e && e.message) || e);
  }
  result.steps.push(step);
  // 让页面把异步 console（网络回调等）打完再取下一张
  await new Promise((r) => setTimeout(r, args.settleMs / Math.max(1, args.routes.length)));
}

// 收尾：给被改页的异步日志一点时间（#883 实测入口页有请求回调日志）
await new Promise((r) => setTimeout(r, args.settleMs));
finish(false);
