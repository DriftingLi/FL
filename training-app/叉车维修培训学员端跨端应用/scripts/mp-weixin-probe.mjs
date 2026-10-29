// 微信开发者工具自动化探针（② 半自动门的取证核心）——由 scripts/mp-weixin-check.ps1 调用。
//
// 背景：docs/adr/0008-移动端验收门与证据.md；spike issue #883 的三条实测坑位（勿删）：
//   1. `pageStack` 为空会让**所有 page 级 API** 报同一个误导弹错
//      `Cannot destructure property 'rawPath' of 't.getPageMetaByWebviewId(...)' as it is null`
//      —— 根因是自动化会话里没有页面被打开，不是版本不兼容。处置 = 调用方先 `cli.bat close` 再 `cli.bat auto`。
//   2. `mp.screenshot()` 挂在 **MiniProgram** 上，不在 Page 上（`page.screenshot is not a function`）；
//      它**不返回内容**，产物只能靠落盘文件取（本脚本读 PNG IHDR 自证）。
//   3. `page.$()` 元素级断言 2026-09-18 原测判不可用（就绪闸门早跑窗口的产物）→ 2026-10-25 #1917 裁决证伪
//      （`page.$('.app-title')` 8ms 可用、`element.text()` 精确匹配可用）→ #1919 落锁收窄：
//      `page.waitFor(选择器)` 与导航 API 仍禁；`page.$` + `element.text()` 锚点断言放行（仅直达模式）。
//
// 2026-09-12 复测（**诚实降级，不许假绿**）：本机 `miniprogram-automator@0.12.1` 下 `mp.reLaunch` / `mp.navigateTo`
//   **恒报 `Uncaught [object Object]`**，而 `mp.connect` / `mp.pageStack` / `mp.screenshot` 正常 ⇒
//   「逐页导航 + 每页截图」这组断言在该环境**不可能成立**。处置：
//     - 捕获导航异常后**不把它算门通过、也不算门失败**，而是把这组断言记成 **SKIP（reason=navigation-api-unsupported）**，
//       写进 JSON 的 `navigation` 字段、`assertions`（值取字符串 `'skip'`）与结果行（调用方渲染 `navigation=skip(unsupported)`）；
//     - 门仍按**可验证子集**判过：`connected` + `pageStack` 非空 + 入口页在栈内 + `errorsTotal==0 && exceptionsTotal==0`
//       + **至少 1 张当前页截图**（导航不可用时当前页仍是 `pageStack[0]`，照样截图取证）；
//     - `failures` 只由**可验证子集**产生 ⇒ 降级本身不产生 failures。
//   **未证实**：是否为版本组合（automator 0.12.1 × 开发者工具 Stable v2.01.2510290）问题，留给后续排查；
//   首跑校准（见 docs/adr/0008-移动端验收门与证据.md ② 段）要求人工核对 `navigation=` 字段。
//   ⚠️ 只认「API 不支持」这一族签名；`TIMEOUT(...)`（元素级 API 挂起那类）仍按**真失败**处理，绝不静默降级。
//
// 2026-09-13 定位（**② 时好时坏的真正原因**，勿删）：本探针此前把「会话未就绪」报成了「自动化端口连不上」。
//   实测三个里程碑：端口接受连接 t≈1s → `Tool.getInfo` 带 `SDKVersion` t≈1–3s → `App.getPageStack` 开始应答
//   t≈24–34s。`automator.connect()` 内部的 `checkVersion()` 紧跟在 ws 打开之后，落在第 2 个里程碑之前就会抛
//   `Cannot read properties of undefined (reading 'split')`；而 `mp.pageStack()` 原本的 30s 超时**正好落在
//   24–34s 这段窗口中间** ⇒ 同一份代码有时绿有时红。处置 = 调用方先跑 `scripts/mp-weixin-ready.mjs` 就绪闸门
//   （等齐两个里程碑再 connect），并且本探针必须用 `connectError.kind` 把两类失败分开报。
//
// 用法：
//   node mp-weixin-probe.mjs --ws ws://127.0.0.1:9420 --routes pages/index/index,pages/login/login \
//        --entry-url pages/index/index --shot-dir <dir> [--timeout-ms 30000] [--page-timeout-ms 10000]
//   任意页直达（#1682 · ADR-0038 决策1）：node mp-weixin-probe.mjs --ws <端点> --expect-page <路由> \
//        --shot-dir <dir> —— 调用方已把构建产物 app.json 首项临时换成目标页并冷启动；本模式**不调用任何
//        导航 API**，只断言「冷启动落地的当前页 == 目标页」+ 截图 + console/exception 读数。
//   达成 ⇒ navigation=ok(detail=cold-start-direct)；未达成 ⇒ navigation=skip(reason=direct-page-unsupported)
//   —— 诚实降级：SKIP 既不冒充门失败，也不冒充逐页取证成立。
// 输出：最后一行固定为 JSON（前缀 `MP_WEIXIN_PROBE `），由调用方解析 —— **只看输出，不看退出码**。
// 退出码：0 = 断言全过；1 = 有断言未过（真报错）；2 = 环境不可用（连不上 / 缺 automator / pageStack 空）。
// 说明：断言结果同时写进 JSON 的 assertions/failures/probeOk，调用方以 JSON 为准。

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420', routes: [], shotDir: '', entryUrl: '', requireRoot: '',
    timeoutMs: 30000, pageTimeoutMs: 10000, settleMs: 2000, tailMax: 40, expectPage: '',
    assertSelectors: [], assertTexts: [], scrollTop: 0, shotsMode: 'single', shotsMax: 5,
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
    else if (a === '--expect-page') out.expectPage = String(val() || '').trim();
    else if (a === '--assert-selector') out.assertSelectors.push(String(val() || '').trim());
    else if (a === '--assert-text') out.assertTexts.push(String(val() || '').trim());
    // #2097：折叠区取证 —— 目标区在首屏之下时，先滚到该位置再截图；
    // 不滚的话「截图拍的是标题、改动在屏幕外」，那样的证据不成立（已踩过一次）。
    else if (a === '--scroll-top') out.scrollTop = Number(val()) || 0;
    // #2097：'full' = 逐屏滚到页尾各截一张（-p1 / -p2 / …）。改动落在哪一屏都拍得到，
    // 不必按票手填滚动值 —— 这正是「截图截错位置」那一类问题的通用解。
    else if (a === '--shots') out.shotsMode = String(val() || 'single');
    else if (a === '--shots-max') out.shotsMax = Number(val()) || 5;
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

// ---------- 锚点元素断言（#1919 落锁 · #1917 裁决承接）----------
// [C3-anchor-begin]（C3 守护扫本标记到 [C3-anchor-end] 之间的非注释行：禁 waitFor/导航 API）
const ANCHOR_STEP_TIMEOUT_MS = 8000; // element-probe 同款步超时（E3 判据 <15000：挂起与报错分得开）
// pairs = [[selector, expectedText], ...]；返回 { total, passed, skipped, items }。
// 失败文案以「锚点断言」开头、**绝不带 `环境不可用：` 前缀**（门侧直达循环对该前缀整组转 exit 2）。
async function runAnchorAssertions(mp, pairs, reached, result) {
  const out = { total: 0, passed: 0, skipped: 0, items: [] };
  if (!pairs || pairs.length === 0) return out;
  if (!reached) {
    // 页身份都没建立 ⇒ 与直达 SKIP 同口径：全 skip、不产生 failures
    for (const [sel] of pairs) {
      out.total += 1; out.skipped += 1;
      out.items.push({ selector: sel, skipped: true, skipReason: 'direct-page-unsupported' });
    }
    return out;
  }
  let page = null;
  try {
    page = await withTimeout(mp.currentPage(), ANCHOR_STEP_TIMEOUT_MS, 'mp.currentPage');
  } catch (e) {
    const msg = String((e && e.message) || e);
    for (const [sel] of pairs) {
      out.total += 1;
      out.items.push({ selector: sel, error: '拿不到当前页：' + msg });
      result.failures.push('锚点断言无法执行（拿不到当前页：' + msg + '）');
    }
    return out;
  }
  for (const [sel, expected] of pairs) {
    out.total += 1;
    const item = { selector: sel, expected, elapsedMs: 0 };
    const t0 = Date.now();
    try {
      const el = await withTimeout(page.$(sel), ANCHOR_STEP_TIMEOUT_MS, 'page.$(' + sel + ')');
      if (!el) {
        // uni-app x 不能跨组件选择 ⇒ 锚点必须页面级可达
        item.error = '锚点元素不可达（page.$ 解析为 null：uni-app x 不能跨组件选择，锚点必须页面级可达）';
        result.failures.push('锚点断言未过：' + sel + ' 元素不可达（page.$ 解析为 null）');
      } else {
        const actual = await withTimeout(el.text(), ANCHOR_STEP_TIMEOUT_MS, 'element.text(' + sel + ')');
        item.actual = String(actual);
        item.ok = item.actual === expected; // 精确匹配（#1917 裁决口径）
        if (item.ok) out.passed += 1;
        else result.failures.push('锚点断言未过：' + sel + ' 期望「' + expected + '」实得「' + item.actual + '」');
      }
    } catch (e) {
      item.error = String((e && e.message) || e);
      result.failures.push('锚点断言未过：' + sel + ' 执行失败（' + item.error + '）');
    }
    item.elapsedMs = Date.now() - t0;
    out.items.push(item);
  }
  return out;
}
// [C3-anchor-end]

// 导航降级的判定（**诚实降级**：SKIP 不等于 PASS，也不等于 FAIL）
const NAV_SKIP_REASON = 'unsupported';
const NAV_SKIP_DETAIL = 'navigation-api-unsupported';
const NAV_SKIP_ASSERTIONS = ['allRoutesVisited', 'screenshotsProduced'];
// 只认「API 不支持」这一族签名：实测本机 automator 0.12.1 的 reLaunch/navigateTo 抛 `Uncaught [object Object]`
// （Error 无 message ⇒ String(err) 只剩这个壳）。`TIMEOUT(...)` 是元素级 API 挂起那类，**不得**被降级吞掉。
const NAV_UNSUPPORTED_RE = /Uncaught \[object Object\]|\bis not a function\b|not supported|不支持/;
function isNavigationUnsupported(err) {
  const msg = typeof err === 'string' ? err : String((err && err.message) || err || '');
  if (/^TIMEOUT\(/.test(msg)) return false;
  return NAV_UNSUPPORTED_RE.test(msg);
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
  // connectError：连接失败时的**归因口径**（{kind: not-ready|unreachable|timeout|unknown, raw}），
  // 调用方靠它区分「会话未就绪」与「端口连不上」——两者以前都写成同一句，导致误诊（2026-09-13）。
  connectError: null,
  // navigation：ok = 逐页导航断言可用；skip = 导航 API 不可用 ⇒ 该组断言记 SKIP（reason=navigation-api-unsupported）
  navigation: { status: 'pending', reason: '', detail: '', at: '', skippedAssertions: [], skippedSteps: [] },
  assertions: {}, failures: [],
  // anchors：锚点元素断言结果（#1919）——JSON 恒有该字段，门侧聚合好读
  anchors: { total: 0, passed: 0, skipped: 0, items: [] },
};

function finish(envUnavailable) {
  result.elapsedMs = Date.now() - started;
  const stack = result.pageStack;
  const pngShots = result.shots.filter((s) => s.isPng).length;
  const navStatus = result.navigation.status;
  // 早退（连不上 / pageStack 空）根本没走到导航 ⇒ 记 n/a：既不是 ok 也不是 skip(unsupported)
  if (navStatus === 'pending') result.navigation = { ...result.navigation, status: 'n/a', reason: 'not-run', detail: 'navigation-not-attempted' };
  const navSkip = result.navigation.status === 'skip';
  const navSupported = result.navigation.status === 'ok';
  // **可验证子集**（降级时只有这几条能判门过/不过）：
  const assertions = {
    connected: result.connected === true,
    // 前置断言：pageStack 非空（空栈会让 page 级 API 报完全误导的错，见文件头坑位 1）
    pageStackNonEmpty: stack.length > 0,
    entryPageInStack: Boolean(result.entryPage) && stack.map(rel).includes(rel(result.entryPage)),
    noConsoleErrors: result.errorsTotal === 0,
    noExceptions: result.exceptionsTotal === 0,
    // 至少 1 张**当前页**截图（导航不可用时当前页 = pageStack[0]，照样截图取证）
    currentPageScreenshot: pngShots >= 1,
  };
  // 「逐页导航 + 每页截图」这组：导航 API 不可用时**记 SKIP**（值 = 字符串 'skip'）——
  // 既不写 true（那是假绿），也不写 false（那不是被测代码的问题）；SKIP 不进 failures。
  // 判据取**导航本身的结果**（每一步的 ok = 导航后当前页 == 目标页），不拿「导航前读到的 pageStack」
  // 去比所有路由——后者恒假（pageStack 在导航前只读一次），会让健康机器上的门永远过不去。
  const navSteps = result.steps.filter((s) => s.navigate !== 'none');
  // 直达模式（#1682）：routes 为空、navigate 全 'none' ⇒ 逐页组判据换成「直达达成 + 至少 1 张页截图」
  const directMode = Boolean(args.expectPage);
  assertions.allRoutesVisited = navSupported
    ? (directMode
      ? result.steps.some((s) => s.direct === true && s.ok === true)
      : (args.routes.length > 0 && navSteps.length === args.routes.length && navSteps.every((s) => s.ok === true)))
    : 'skip';
  assertions.screenshotsProduced = navSupported
    ? (directMode ? pngShots >= 1 : (args.routes.length > 0 && pngShots >= args.routes.length))
    : 'skip';
  result.assertions = assertions;
  // 只有**明确 false**（可验证子集不过，或导航可用时逐页组不过）才算失败；'skip' 不算
  Object.entries(assertions).forEach(([k, v]) => {
    if (v === false) result.failures.push('断言未过：' + k);
  });
  if (navSkip) {
    result.navigation.skippedAssertions = NAV_SKIP_ASSERTIONS.slice();
    result.navigation.skippedSteps = result.steps
      .filter((s) => s.skipped).map((s) => s.label + ':' + s.route);
  }
  const probeOk = result.failures.length === 0;
  console.log('MP_WEIXIN_PROBE ' + JSON.stringify({ ...result, probeOk }));
  process.exit(probeOk ? 0 : envUnavailable ? 2 : 1);
}

// 锚点参数校验（#1919，parseArgs 之后、finish 可用处）：
// 两数不等 ⇒ 参数错配 fail-closed 早退（exit 1，不连会话就红）；
// 有锚点但无 --expect-page ⇒ 锚点断言只在直达模式受支持——**不早退**（照跑原取证面，收尾时 failures 判红）。
const anchorPairs = [];
for (let i = 0; i < Math.max(args.assertSelectors.length, args.assertTexts.length); i += 1) {
  anchorPairs.push([args.assertSelectors[i] || '', args.assertTexts[i] || '']);
}
if (args.assertSelectors.length !== args.assertTexts.length) {
  const msg = '参数不配对：--assert-selector 与 --assert-text 数目不一致（'
    + args.assertSelectors.length + ' vs ' + args.assertTexts.length + '）';
  console.error('[error] ' + msg);
  result.failures.push(msg);
  finish(false);
}
if (anchorPairs.length > 0 && !args.expectPage) {
  result.failures.push('锚点断言只在直达模式（--expect-page）受支持：非直达模式下锚点不执行，判未过（锚点断言）');
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

// 「连不上」与「会话未就绪」**必须分开报**（2026-09-13 实测定位，勿合并回去）。
// automator 的 `Launcher.connect` 在 ws 打开后**立刻**调 `MiniProgram.checkVersion()`，它读
// `(await send('Tool.getInfo')).SDKVersion`；该字段在会话就绪前缺失 ⇒ `licia/cmpVersion` 对 undefined
// 调 `.split('.')` ⇒ 抛 `Cannot read properties of undefined (reading 'split')`，
// 再被 `connectTool` 的 catch 换成 “Failed connecting to …” 文案 —— 看上去完全像端口问题。
// 旧版探针把这条一律写成「自动化端口连不上」，把排查引向了端口/时序（真实发生在 #883 复测里），
// 而真实原因是**会话未就绪**：见 scripts/mp-weixin-ready.mjs 的就绪闸门与 ADR-0008 ② 段。
function classifyConnectError(e) {
  const raw = String((e && e.message) || e || '');
  if (/reading 'split'|cmpVersion|checkVersion|SDKVersion/i.test(raw)) {
    return { kind: 'not-ready', detail: '自动化会话未就绪：Tool.getInfo 未返回 SDKVersion（automator 的 checkVersion 崩溃）' };
  }
  if (/Failed connecting to|ECONNREFUSED|socket hang up|Connection closed|ETIMEDOUT/i.test(raw)) {
    return { kind: 'unreachable', detail: '自动化端口连不上（ws 层）' };
  }
  if (/^TIMEOUT\(/.test(raw)) return { kind: 'timeout', detail: '连接超时（未在预算内拿到连接）' };
  return { kind: 'unknown', detail: '连接失败' };
}

let mp;
try {
  mp = await withTimeout(automator.connect({ wsEndpoint: args.ws }), args.timeoutMs, 'automator.connect');
  result.connected = true;
} catch (e) {
  const c = classifyConnectError(e);
  result.connectError = { kind: c.kind, raw: String((e && e.message) || e) };
  result.failures.push('环境不可用：' + c.detail + '（口径 connectError.kind=' + c.kind + '；原始：' + result.connectError.raw + '）');
  console.error('[error] ' + result.failures[0]);
  if (c.kind === 'not-ready') {
    console.error('[error] 这不是端口问题：跑就绪闸门 `node scripts/mp-weixin-ready.mjs --ws <端点> --require-stack`');
    console.error('[error] 它会等 Tool.getInfo 带 SDKVersion、且 App.getPageStack 开始应答，再把端点交给 automator。');
  }
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

// 任意页直达模式（#1682 · ADR-0038 决策1）：调用方已把构建产物 app.json 首项临时换成目标页并冷启动，
// 会话落地页就应是目标页 ⇒ **不调用任何导航 API**（reLaunch/navigateTo 在本机 automator 0.12.1 恒报
// `Uncaught [object Object]`，见文件头 2026-09-12 复测段），只判「落地页 == 目标页」+ 截图 + 读数。
// 达成 ⇒ navigation=ok(detail=cold-start-direct)；未达成 ⇒ SKIP(reason=direct-page-unsupported)：
// 机制不成立**不冒充门失败**（failures 不产生），也**不冒充逐页取证成立**（不写 true）。
/**
 * #2097：滚动到目标位置（折叠区取证的必需前置）。
 *
 * 走 mp.evaluate 在**页面上下文**里调 wx.pageScrollTo：automator 的页面级包装 API 在本机 0.12.1
 * 不稳，而 evaluate 是仓内既有可行通道（先例 scripts/cert-fix-evidence.mjs）。滚动失败不静默吞：
 * 记 scrollError，让读证据的人知道「这张图不代表目标区」。
 */
/**
 * #2097：逐屏整页取证。改动可能落在任何一屏；只拍首屏就等于「没拍」。
 *
 * ⚠️ **2026-10-29 实测的血账（本函数第一版为什么恒只出 -p1）**：第一版靠
 * `getCurrentPages()` 末页对象的 `p.scrollTop` 判「滚到头了没」——小程序 Page 实例**没有**这个属性，
 * 读不到就落到真值 0 ⇒ 首次滚动后立刻 `actual <= lastTop` 成立 ⇒ break。三页全只出 `-p1`、
 * 且**没有** scrollError，绿得毫无痕迹。同时本仓页面把内容放在 `<scroll-view>` 里（dashboard 与
 * featured-detail 都是 `.content-scroll`），**页级 `wx.pageScrollTo` 在 scroll-view 页面上恒无效**。
 * 现在的判据全部走 automator 的 ScrollViewElement：`scrollHeight()` 量内容高度、`scrollTo(x, y)` 驱动、
 * `property('scrollTop')` 回读实际位置。**没有可驱动容器时只拍首屏并写 `shotWarning`** —— 宁可少拍、
 * 不许冒充「已逐屏」（同 ② 门一贯的「不假绿」纪律）。
 */
const SHOT_SCROLL_CANDIDATES = ['.content-scroll', '.page-scroll', 'scroll-view'];
/** 滚动后等渲染稳定的时长（滚动是同步 IPC 但视图刷新是下一帧，太快会拍到大半旧屏） */
const SHOT_SCROLL_SETTLE_MS = 350;

/**
 * 找一个**真能滚**的容器：必须有内容高度且 > 0。
 *
 * @returns {Promise<null | { el: any, sel: string, contentH: number }>} null = 本页没有可驱动的滚动容器
 */
async function findScrollContainer(page) {
  if (!page) return null;
  for (let i = 0; i < SHOT_SCROLL_CANDIDATES.length; i += 1) {
    const sel = SHOT_SCROLL_CANDIDATES[i];
    let el = null;
    try {
      el = await withTimeout(page.$(sel), args.timeoutMs, 'page.$(' + sel + ')');
    } catch (e) {
      el = null;
    }
    if (!el || typeof el.scrollHeight !== 'function' || typeof el.scrollTo !== 'function') continue;
    let contentH = 0;
    try {
      contentH = Number(await withTimeout(el.scrollHeight(), args.timeoutMs, 'scrollHeight(' + sel + ')')) || 0;
    } catch (e) {
      contentH = 0;
    }
    if (contentH <= 0) continue;
    return { el, sel, contentH };
  }
  return null;
}

/** 回读容器实际 scrollTop；失败返回 -1（与真值 0 区分开，别把「读不到」当「在顶部」） */
async function readScrollTop(container) {
  try {
    return Number(await withTimeout(container.el.property('scrollTop'), args.timeoutMs, 'scrollTop(' + container.sel + ')')) || 0;
  } catch (e) {
    return -1;
  }
}

async function captureShots(name, shotDir, step) {
  if (args.shotsMode !== 'full') {
    const shotFile = path.join(shotDir, name + '.png');
    await withTimeout(mp.screenshot({ path: shotFile }), args.timeoutMs, 'mp.screenshot(' + name + ')');
    if (fs.existsSync(shotFile)) {
      const info = pngInfo(shotFile);
      result.shots.push({ name, route: step.currentPage, ...info });
      step.shot = shotFile;
    } else {
      step.shotError = 'screenshot 未落盘（该 API 不返回内容，只能看文件）';
    }
    return;
  }

  let winH = 0;
  try {
    winH = Number(await withTimeout(mp.evaluate(() => wx.getSystemInfoSync().windowHeight), args.timeoutMs, 'mp.evaluate(windowHeight)')) || 0;
  } catch (e) {
    winH = 0;
  }
  if (winH <= 0) winH = 600;
  // 步长取窗口高度的 80%（留重叠，避免正好切掉某一行）
  const stepPx = Math.max(200, Math.round(winH * 0.8));

  step.shots = [];
  let page = null;
  try {
    page = await withTimeout(mp.currentPage(), args.timeoutMs, 'mp.currentPage(shots)');
  } catch (e) {
    page = null;
  }
  const container = await findScrollContainer(page);

  const shootOnce = async (shotName) => {
    const shotFile = path.join(shotDir, shotName + '.png');
    await withTimeout(mp.screenshot({ path: shotFile }), args.timeoutMs, 'mp.screenshot(' + shotName + ')');
    if (!fs.existsSync(shotFile)) {
      step.shotError = 'screenshot 未落盘（该 API 不返回内容，只能看文件）';
      return null;
    }
    return { shotFile, info: pngInfo(shotFile) };
  };

  if (!container) {
    // 诚实降级：没有可驱动的滚动容器 ⇒ 只拍首屏，并留名（不冒充「已逐屏」）
    step.shotWarning = 'no-scroll-container';
    const one = await shootOnce(name + '-p1');
    if (one) {
      result.shots.push({ name: name + '-p1', route: step.currentPage, scrollTop: -1, scroller: null, ...one.info });
      step.shots.push(one.shotFile);
      step.shot = one.shotFile;
    }
    return;
  }

  step.scroller = container.sel;
  step.scrollHeight = container.contentH;
  const maxTop = Math.max(0, container.contentH - winH);
  const planned = Math.max(1, Math.min(args.shotsMax, Math.ceil(maxTop / stepPx) + 1));
  step.shotsPlanned = planned;

  for (let i = 0; i < planned; i += 1) {
    const want = Math.min(i * stepPx, maxTop);
    if (i > 0) {
      try {
        await withTimeout(container.el.scrollTo(0, want), args.timeoutMs, 'scrollTo(' + container.sel + ',' + want + ')');
      } catch (e) {
        step.scrollError = String((e && e.message) || e);
        break;
      }
      await new Promise((r) => setTimeout(r, SHOT_SCROLL_SETTLE_MS));
    }
    const actual = await readScrollTop(container);
    const one = await shootOnce(name + '-p' + (i + 1));
    if (!one) break;
    result.shots.push({ name: name + '-p' + (i + 1), route: step.currentPage, scrollTop: actual, scroller: container.sel, ...one.info });
    step.shots.push(one.shotFile);
  }

  // 诚实标记：计划多屏但每屏字节完全相同 ⇒ 滚动没生效（别把一张图当多屏证据）
  if (step.shots.length > 1) {
    const first = fs.readFileSync(step.shots[0]);
    let allSame = true;
    for (let i = 1; i < step.shots.length; i += 1) {
      if (!first.equals(fs.readFileSync(step.shots[i]))) { allSame = false; break; }
    }
    if (allSame) step.shotWarning = 'scroll-ineffective';
  }
  if (step.shots.length > 0) step.shot = step.shots[0];
}

async function applyScroll(y) {
  if (!y || y <= 0) return '';
  try {
    await withTimeout(mp.evaluate((top) => { wx.pageScrollTo({ scrollTop: top, duration: 0 }); }, y), 8000, 'wx.pageScrollTo');
    await new Promise((r) => setTimeout(r, 400));
    return '';
  } catch (e) {
    return String((e && e.message) || e);
  }
}

if (args.expectPage) {
  const target = rel(args.expectPage);
  const leaf = target.split('/').pop() || 'direct';
  const reached = rel(result.pageStack[0]) === target;
  const step = { label: 'direct-' + leaf, route: target, navigate: 'none', direct: true, ok: reached, currentPage: rel(result.pageStack[0]) };
  if (!reached) {
    step.error = '冷启动落地的当前页不是目标页（直达未达成）';
    step.skipped = true;
    step.skipReason = 'direct-page-unsupported';
  }
  if (args.shotDir) {
    // 达成截目标页（文件名=页名）；未达成也截——但命名 miss- 前缀，**不冒充被改页证据**
    const shotName = reached ? leaf : 'miss-' + leaf;
    // 先滚到目标区，再截图（#2097；失败只记读数，不冒充拍到了）
    const scrollErr = await applyScroll(args.scrollTop);
    if (scrollErr) step.scrollError = scrollErr;
    // #1690 run5 实测：冷启动后 app 侧渲染服务未热 ⇒ mp.screenshot 会撞 automator 内部
    // 15s 应答超时（dashboard 直达 reached=True 却截不到图判红）。重试一次（隔 3s 让服务
    // 热起来），两次都失败才记错——不把「试过一次」冒充成「截到了」。
    let shotErr = null;
    for (let attempt = 1; attempt <= 2; attempt++) {
      shotErr = null;
      try {
        // #2097：取证入口统一走 captureShots —— single 模式等价旧行为，full 模式逐屏拍到页尾。
        await captureShots(shotName, args.shotDir, step);
        if (step.shot) break;
        shotErr = 'screenshot 未落盘（该 API 不返回内容，只能看文件）';
      } catch (e) {
        shotErr = String((e && e.message) || e);
      }
      if (attempt < 2) await new Promise((r2) => setTimeout(r2, 3000));
    }
    if (shotErr) {
      if (shotErr.includes('未落盘')) step.shotError = shotErr;
      else step.error = shotErr;
    }
  }
  // 锚点断言（#1919）：截图在前、锚点在后——锚点挂起 8s 也不吃掉截图证据（截图=人证，锚点=判定面）
  result.anchors = await runAnchorAssertions(mp, anchorPairs, reached, result);
  result.steps.push(step);
  result.navigation = reached
    ? { status: 'ok', reason: '', detail: 'cold-start-direct', at: 'direct:' + target, skippedAssertions: [], skippedSteps: [] }
    : { status: 'skip', reason: 'direct-page-unsupported', detail: 'direct-page-unsupported', at: 'direct:' + target, skippedAssertions: [], skippedSteps: [] };
  console.log('[direct] ' + (reached ? 'ok' : 'skip') + ' page=' + target + ' current=' + step.currentPage
    + ' errors=' + result.errorsTotal + ' exceptions=' + result.exceptionsTotal + ' shot=' + (step.shot || '-')
    + ' anchors=' + result.anchors.passed + '/' + result.anchors.total);
  await new Promise((r) => setTimeout(r, args.settleMs));
  finish(false);
}

// 逐页：第一个用 reLaunch（当入口页），其余用 navigateTo（当被改页）
let navSkipped = false;
const markNavSkip = (label, route, err) => {
  navSkipped = true;
  result.navigation = {
    status: 'skip', reason: NAV_SKIP_REASON, detail: NAV_SKIP_DETAIL,
    at: label + ':' + route, skippedAssertions: [], skippedSteps: [],
  };
  console.error('[skip] 导航 API 不可用（' + label + ' -> ' + route + '）：' + String((err && err.message) || err)
    + ' ⇒ 逐页导航 + 每页截图记 SKIP（reason=' + NAV_SKIP_DETAIL + '），不计入门失败；门只对可验证子集作结论。');
};
for (let i = 0; i < args.routes.length; i += 1) {
  const route = rel(args.routes[i]);
  const label = i === 0 ? 'entry' : 'page-' + i;
  const step = { label, route, navigate: i === 0 ? 'reLaunch' : 'navigateTo', ok: false };
  if (navSkipped) {
    // 导航 API 已证不可用：后续路由不再逐个重试，统一记 SKIP（省时间且不制造误导性的逐个失败）
    step.skipped = true;
    step.skipReason = NAV_SKIP_DETAIL;
    result.steps.push(step);
    continue;
  }
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
    const msg = String((e && e.message) || e);
    if (isNavigationUnsupported(e)) {
      // **诚实降级**：既不算通过、也不算失败 —— 记 SKIP，并让调用方把 navigation=skip(...) 写进结果行与评论
      markNavSkip(label, route, e);
      step.skipped = true;
      step.skipReason = NAV_SKIP_DETAIL;
      step.error = msg;
    } else {
      step.error = msg;
    }
  }
  result.steps.push(step);
  // 让页面把异步 console（网络回调等）打完再取下一张
  await new Promise((r) => setTimeout(r, args.settleMs / Math.max(1, args.routes.length)));
}

// 降级路径仍要满足「至少 1 张当前页截图」：导航不可用时当前页仍是 pageStack 里那一页，直接截它。
if (navSkipped && args.shotDir && result.shots.filter((s) => s.isPng).length === 0) {
  const route = result.pageStack[0] || result.entryPage;
  const step = { label: 'current', route, navigate: 'none', ok: false, currentPageOnly: true };
  const shotFile = path.join(args.shotDir, 'current.png');
  try {
    await withTimeout(mp.screenshot({ path: shotFile }), args.timeoutMs, 'mp.screenshot(current)');
    if (fs.existsSync(shotFile)) {
      const info = pngInfo(shotFile);
      result.shots.push({ name: 'current', route, ...info });
      step.shot = shotFile;
      step.ok = info.isPng === true;
      if (!step.ok) step.error = '当前页截图不是 PNG（见 sig）';
    } else {
      step.shotError = 'screenshot 未落盘（该 API 不返回内容，只能看文件）';
    }
  } catch (e) {
    step.error = String((e && e.message) || e);
  }
  result.steps.push(step);
}
if (!navSkipped) result.navigation = { status: 'ok', reason: '', detail: '', at: '', skippedAssertions: [], skippedSteps: [] };

// 收尾：给被改页的异步日志一点时间（#883 实测入口页有请求回调日志）
await new Promise((r) => setTimeout(r, args.settleMs));
finish(false);
