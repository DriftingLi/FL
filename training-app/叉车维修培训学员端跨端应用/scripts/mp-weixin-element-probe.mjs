// 微信小程序「元素级 API 可用性」分层探针 —— 裁决 #883 探针输出 × ADR-0008:134/137 的矛盾。
//
// 为什么存在这个文件（勿删）：
//   仓库里对同一件事有两份**互相矛盾**的实测记录：
//     · #883（issue 正文的探针 JSON）：`currentPage` ✅ / `reLaunch` ✅ / `navigateTo` ✅
//       （返回 `pages/login/login`）/ `errorsTotal=0` / `exceptionsTotal=0`；
//       **只有** `entryPage.$("view")` 报 `TIMEOUT(15000ms)`。
//     · `docs/adr/0008-移动端验收门与证据.md:137` 的转述：「`mp.reLaunch` / `mp.navigateTo`
//       **恒报** `Uncaught [object Object]`」⇒ 逐页导航组降级 SKIP(reason=navigation-api-unsupported)；
//       :134 进一步把「元素级断言」**整体**判为不可用。
//   而 #883 测的是**标签选择器** `$("view")`；ADR 把结论推广到了**元素级 API 全体**。
//   仓库里唯一那个页面用例用的是**类选择器**（`pages/index/index.test.js` 的 `page.$('.title')`）——
//   **类选择器从未被实测过**。而任何真实用例（「点了按钮没跳转」）都必须 `$('.btn').tap()`。
//   ⇒ 「那 15s 挂起绑在元素级 API 上，还是绑在标签选择器上」，就是本次要裁决的问题。
//   ADR:137 自己也写着「**⚠️ 未证实**：是否为版本组合问题，留给后续排查」——本探针就是那次排查。
//
// 与 ② 门的关系（**刻意解耦，勿接回去**）：
//   `scripts/mp-weixin-probe.mjs` 是 ② 门的取证核心，受 `utils/mpWeixinGateContract.test.js` 的
//   C3 守护（「探针不得用元素级 API」，理由正是 15s 挂起会让门时好时坏）。本文件是 **spike 工具，不是门**：
//   它不产出任何门的通过结论，也**不得**被 `scripts/mp-weixin-check.ps1` 引用
//   （`utils/mpWeixinElementProbeContract.test.js` 的 E5 会断言这一点）。
//
// 判据预登记（**事前写死，事后不得挪门槛**）：见 computeVerdict() 与 VERDICT_IMPACT。
//
// 反永绿（本仓的失败模式：让绿被写死而不是被证出来）：
//   本探针自带**已知必败对照项** `control.absent` —— 用一个**不存在**的 class 去 `$()`，
//   它必须**取不到**元素（`resolved-null`）。若它取到了元素，说明元素解析恒真 ⇒ `trustworthy=false`，
//   **其余测量一律不作结论**（`verdict=inconclusive-control-failed`）。
//   没有这一步的探针，等于一行绿字。
//   另注：对照项**超时**不算对照失败，那本身是一个测量结果（说明 `$()` 连"找不到"这条路径都不返回）。
//
// 运行前提（人只需确认开发者工具处于**已登录**状态；登录态过期时由人补扫一次码）：
//   1. 产物已构建且开发者工具已打开、自动化端口已开：
//        cli.bat auto --project <dist> --auto-port 9420 --trust-project
//   2. **先跑就绪闸门**（#883 血账：`auto` 返回 ≠ 会话可用，`App.getPageStack` 要等 24–34s）：
//        node scripts/mp-weixin-ready.mjs --ws ws://127.0.0.1:9420 --require-stack --require-root <automatorRoot>
//   3. 再跑本探针：
//        node scripts/mp-weixin-element-probe.mjs --ws ws://127.0.0.1:9420 --require-root <automatorRoot>
//
// 靶点选择（**登录态无关**）：默认打 `pages/login/login`，因为它可用 `mp.navigateTo` 直接到达，
//   不像入口页 `pages/index/index` 在已登录时会 `onLoad` 里 `reLaunch` 到 dashboard（那样靶点会消失）。
//   默认两步测量：`.app-title` 的文本（类选择器解析 + 文本读取），以及 `.pwd-toggle` 的
//   **点击前后文本变化**（`@click="togglePasswordVisible"`，显示 ↔ 隐藏）——
//   后者是「点击 → 状态更新」的真断言，不需要跳转，因此不掺入导航 API 的可用性。
//
// 输出：最后一行固定 `MP_WEIXIN_ELEMENT_PROBE {json}`；人可读分步行打在它之前。
// 退出码：0 = 已得出结论（结论本身可能是「不可用」）；2 = 环境不可用；3 = 对照项失败（结论不可信）。

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420',
    requireRoot: '',
    targetRoute: 'pages/login/login',
    classSelector: '.app-title',
    classExpectText: '叉车维修培训系统',
    tapSelector: '.pwd-toggle',
    absentSelector: '.zzz-probe-absent-class-9f3a',
    tagSelector: 'view',
    connectTimeoutMs: 30000,
    // 默认步超时必须 **< 15000**：这样 #883 那类挂起会被判成 TIMEOUT(8000) 而不是拖死整个探针，
    // 「挂起」与「报错」才分得开（这是本探针的核心可观测性）。
    stepTimeoutMs: 8000,
    settleMs: 1500,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const val = () => argv[++i];
    if (a === '--ws') out.ws = val();
    else if (a === '--require-root') out.requireRoot = val();
    else if (a === '--target-route') out.targetRoute = val();
    else if (a === '--class-selector') out.classSelector = val();
    else if (a === '--class-expect-text') out.classExpectText = val();
    else if (a === '--tap-selector') out.tapSelector = val();
    else if (a === '--absent-selector') out.absentSelector = val();
    else if (a === '--tag-selector') out.tagSelector = val();
    else if (a === '--connect-timeout-ms') out.connectTimeoutMs = Number(val());
    else if (a === '--step-timeout-ms') out.stepTimeoutMs = Number(val());
    else if (a === '--settle-ms') out.settleMs = Number(val());
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

// 原始错误三元组：裁决 ADR 的「恒报 `Uncaught [object Object]`」需要**原始串**，不能只留一句归纳。
function rawError(e) {
  const err = e || {};
  let stringified = '';
  try { stringified = JSON.stringify(err); } catch { stringified = '(JSON.stringify 失败)'; }
  return {
    name: String(err.name || ''),
    message: String(err.message || ''),
    str: String(err),
    stringified,
  };
}

// 统一记录一步：outcome ∈ resolved | resolved-null | timeout | error | skipped
function makeRecorder() {
  const steps = [];
  return {
    steps,
    rec(id, kind, target, outcome, extra) {
      const step = { id, kind, target, outcome, ...(extra || {}) };
      steps.push(step);
      const detail = step.value !== undefined ? ' value=' + JSON.stringify(step.value)
        : step.error ? ' error=' + step.error.message
          : step.note ? ' note=' + step.note : '';
      console.log(`[step] ${id} kind=${kind} target=${target} outcome=${outcome} ${step.elapsedMs}ms${detail}`);
      return step;
    },
    async run(id, kind, target, fn) {
      const t0 = Date.now();
      try {
        const value = await withTimeout(fn(), args.stepTimeoutMs, id + '(' + target + ')');
        const outcome = (value === null || value === undefined) ? 'resolved-null' : 'resolved';
        return this.rec(id, kind, target, outcome, {
          elapsedMs: Date.now() - t0,
          value: outcome === 'resolved' ? summarizeElement(value) : null,
          _value: value,
        });
      } catch (e) {
        const isTimeout = /^TIMEOUT\(/.test(String((e && e.message) || e));
        return this.rec(id, kind, target, isTimeout ? 'timeout' : 'error', {
          elapsedMs: Date.now() - t0,
          error: rawError(e),
        });
      }
    },
  };
}

// value 只留可读摘要：Element / Page 都不可安全 JSON 序列化（含循环引用与原生句柄），
// 硬塞进结果行会把那一行打爆或直接抛 —— 所以这里**只取白名单字段**，取不到就退化成类型名。
function summarizeElement(v) {
  if (Array.isArray(v)) return { type: 'Array', length: v.length };
  if (v && typeof v === 'object') {
    if (typeof v.path === 'string') return { type: 'Page', path: String(v.path).slice(0, 200) };
    const out = {};
    for (const k of ['tagName', 'id', 'innerHTML', 'innerText']) {
      try {
        const got = typeof v[k] === 'function' ? v[k]() : v[k];
        if (got !== undefined && got !== null && String(got) !== '') out[k] = String(got).slice(0, 200);
      } catch { /* 取不到就跳过 */ }
    }
    return Object.keys(out).length > 0 ? out : { type: 'Element' };
  }
  if (typeof v === 'string') return v.slice(0, 200);
  return v === undefined ? null : v;
}

function loadAutomator() {
  // 解析顺序与 mp-weixin-probe.mjs 一致：调用方指定的 require 根（门脚本把 automator 装到项目外，
  // 不动仓库的 package.json / package-lock.json）→ 项目自身 node_modules → 本脚本上一级 → require 链。
  const roots = [];
  if (args.requireRoot) roots.push(args.requireRoot);
  roots.push(process.cwd());
  if (import.meta.dirname) roots.push(path.join(import.meta.dirname, '..'));
  for (const root of roots) {
    try {
      const mod = createRequire(path.join(root, 'noop.js'))('miniprogram-automator');
      return { mod, from: root, version: readPkgVersion(root) };
    } catch { /* 换下一个根 */ }
  }
  try {
    const mod = createRequire(import.meta.url)('miniprogram-automator');
    return { mod, from: 'require-chain', version: '' };
  } catch (e) {
    return { mod: null, error: String((e && e.message) || e), triedRoots: roots };
  }
}

// 「版本组合」假设（ADR:137 未证实的那条）需要把 automator 版本写进证据里
function readPkgVersion(root) {
  try {
    const p = path.join(root, 'node_modules', 'miniprogram-automator', 'package.json');
    return String(JSON.parse(fs.readFileSync(p, 'utf8')).version || '');
  } catch { return ''; }
}

// 就绪闸门报告的 IDE / SDK 版本（存在就带上——裁决「版本组合」假设要用）
function readReadySidecar() {
  try {
    const p = path.join(process.cwd(), '.ci-verify', 'mp-weixin-element-ready.json');
    if (!fs.existsSync(p)) return null;
    return JSON.parse(fs.readFileSync(p, 'utf8'));
  } catch { return null; }
}

// ---------- 判据预登记（**唯一真源，事后不得挪门槛**）----------
const VERDICT_IMPACT = {
  'class-selector-and-tap-flow-usable': 'Q15：类选择器 + 点击流程可用 ⇒ mp-weixin 可作流程验证载体（Q5 改为「迁移到 mp-weixin 通道」，不撤销）',
  'class-selector-usable-text-read-failed': 'Q15：类选择器可解析但文本读取失败 ⇒ 元素级可用性存疑，需补测后再判',
  'class-selector-usable-tap-flow-unproven': 'Q15：类选择器可用、点击流程未证实 ⇒ 只能做 page 级断言；流程验证不成立 ⇒ 走 ③ + 撤销 Q5',
  'class-selector-unusable': 'Q15：类选择器亦不可用 ⇒ 元素级断言整体不可用，ADR:134 的推广成立 ⇒ 走 ③ + 撤销 Q5',
  'inconclusive-control-failed': 'Q15：**不判** —— 对照项失败，本探针自身不可信，先修探针',
  'env-unavailable': 'Q15：**不判** —— 环境不可用（连接/会话/pageStack），与 API 可用性无关',
};

function computeVerdict(m, trust) {
  if (!trust.ok) return 'inconclusive-control-failed';
  if (m.classResolve !== 'resolved') return 'class-selector-unusable';
  if (!m.textRead || m.textRead.matched !== true) return 'class-selector-usable-text-read-failed';
  if (!m.tapToggled || m.tapToggled.changed !== true) return 'class-selector-usable-tap-flow-unproven';
  return 'class-selector-and-tap-flow-usable';
}

// 裁决 ADR:137 的「恒报 Uncaught [object Object]」：以**原始错误串**为准，不以归纳为准
function classifyNavigation(relaunch, navigateTo) {
  const unsupported = (s) => /Uncaught \[object Object\]/.test((s && (s.str || s.message)) || '');
  const a = unsupported(relaunch);
  const b = unsupported(navigateTo);
  if (a && b) return 'confirmed-adr-0008';       // 两个导航 API 都报 Uncaught ⇒ ADR 的转述成立
  if (!a && !b && relaunch.outcome === 'resolved' && navigateTo.outcome === 'resolved') {
    return 'confirmed-probe-883';                // 都能用 ⇒ #883 的探针输出成立，ADR 的转述是过度推广
  }
  return 'mixed';
}

// ---------- 主流程 ----------
const started = Date.now();
const recorder = makeRecorder();
const result = {
  ws: args.ws,
  probe: 'mp-weixin-element-probe',
  isGate: false,                                  // 显式声明：这不是门证据
  connected: false,
  automatorFrom: '',
  automatorVersion: '',
  ready: readReadySidecar(),                      // 就绪闸门的 sidecar（有则带上 ide/sdk 版本）
  pageStack: [],
  controls: {},
  measurements: {},
  trustworthy: false,
  trustReason: '',
  navigationContradiction: 'unknown',
  verdict: '',
  channelImpact: '',
  elapsedMs: 0,
  error: null,
};

function finish(envUnavailable) {
  result.elapsedMs = Date.now() - started;
  // `_value` 是给判定用的原生句柄（Element / Page / 数组），**不得**进入结果行：
  // 它不可 JSON 序列化（循环引用 + 原生句柄），混进去会让那一行直接抛或打爆。
  result.steps = recorder.steps.map(({ _value, ...rest }) => rest);
  console.log('MP_WEIXIN_ELEMENT_PROBE ' + JSON.stringify(result));
  if (envUnavailable) process.exit(2);
  process.exit(result.trustworthy ? 0 : 3);
}

const loaded = loadAutomator();
if (!loaded.mod) {
  result.error = '找不到 miniprogram-automator（' + (loaded.error || '未知') + '）';
  result.triedRoots = loaded.triedRoots || [];
  result.verdict = 'env-unavailable';
  result.channelImpact = VERDICT_IMPACT[result.verdict];
  console.error('[error] ' + result.error);
  finish(true);
}
result.automatorFrom = loaded.from;
result.automatorVersion = loaded.version;
const automator = loaded.mod;

// 与 mp-weixin-probe.mjs 同款归因：区分「会话未就绪」与「端口连不上」（2026-09-13 实测定位）
function classifyConnectError(e) {
  const raw = String((e && e.message) || e || '');
  if (/reading 'split'|cmpVersion|checkVersion|SDKVersion/i.test(raw)) return 'not-ready';
  if (/Failed connecting to|ECONNREFUSED|socket hang up|Connection closed|ETIMEDOUT/i.test(raw)) return 'unreachable';
  if (/^TIMEOUT\(/.test(raw)) return 'timeout';
  return 'unknown';
}

let mp;
{
  const t0 = Date.now();
  try {
    mp = await withTimeout(automator.connect({ wsEndpoint: args.ws }), args.connectTimeoutMs, 'automator.connect');
    result.connected = true;
    recorder.rec('connect', 'env', args.ws, 'resolved', { elapsedMs: Date.now() - t0 });
  } catch (e) {
    const kind = classifyConnectError(e);
    recorder.rec('connect', 'env', args.ws, 'error', { elapsedMs: Date.now() - t0, error: rawError(e), note: 'kind=' + kind });
    result.error = '环境不可用：连接失败（kind=' + kind + '）—— 若 kind=not-ready 请先跑就绪闸门';
    result.verdict = 'env-unavailable';
    result.channelImpact = VERDICT_IMPACT[result.verdict];
    console.error('[error] ' + result.error);
    finish(true);
  }
}

// pageStack 前置断言：空栈会让所有 page 级 API 报完全误导的错（#883 坑位 1）
{
  const step = await recorder.run('pageStack', 'env', 'mp.pageStack', () => mp.pageStack());
  if (step.outcome === 'resolved') result.pageStack = (step._value || []).map((p) => String(p.path || '').replace(/^\//, ''));
  if (result.pageStack.length === 0) {
    result.error = '环境不可用：pageStack 为空（会话没打开页面）—— 处置：先 cli.bat close 再 cli.bat open + auto';
    result.verdict = 'env-unavailable';
    result.channelImpact = VERDICT_IMPACT[result.verdict];
    console.error('[error] ' + result.error);
    finish(true);
  }
}

// 显式导航到靶页：不依赖会话当前停在哪一页（靶页登录态无关）
{
  const step = await recorder.run('target.reLaunch', 'nav', args.targetRoute, () => mp.reLaunch(args.targetRoute));
  result.measurements.navigateToTarget = { outcome: step.outcome, raw: step.error || null };
  await new Promise((r) => setTimeout(r, args.settleMs));
}

// 取当前页必须兜住：抛出去就没有结果行，等于这次取证白跑（fail-closed 也要有 JSON）
let page = null;
{
  const t0 = Date.now();
  try {
    page = await withTimeout(mp.currentPage(), args.stepTimeoutMs, 'mp.currentPage');
    result.measurements.currentPage = String((page && page.path) || '').replace(/^\//, '');
    recorder.rec('currentPage', 'env', args.targetRoute, page ? 'resolved' : 'resolved-null', {
      elapsedMs: Date.now() - t0,
      value: result.measurements.currentPage,
    });
  } catch (e) {
    recorder.rec('currentPage', 'env', args.targetRoute, 'error', { elapsedMs: Date.now() - t0, error: rawError(e) });
    result.error = '环境不可用：currentPage 取不到（' + ((e && e.message) || e) + '）';
    result.verdict = 'env-unavailable';
    result.channelImpact = VERDICT_IMPACT[result.verdict];
    console.error('[error] ' + result.error);
    finish(true);
  }
}

// —— 对照项（反永绿）：不存在的 class 必须取不到元素 ——
{
  const step = await recorder.run('control.absent', 'control', args.absentSelector, () => page.$(args.absentSelector));
  result.controls.absent = step.outcome;
  // 对照项**超时**是测量结果（连"找不到"都不返回），不是对照失败；只有"取到了元素"才说明解析恒真。
  result.trustworthy = step.outcome !== 'resolved';
  result.trustReason = result.trustworthy
    ? 'control-absent 未取到元素（outcome=' + step.outcome + '）⇒ 解析不是恒真'
    : 'control-absent **取到了元素** ⇒ 元素解析恒真，其余测量一律不作结论';
}

// —— #883 复现：标签选择器 ——
{
  const step = await recorder.run('control.tagSelect', 'control', args.tagSelector, () => page.$(args.tagSelector));
  result.controls.tagSelect = step.outcome;
}

// —— 测量 1：类选择器解析 ——
let classEl = null;
{
  const step = await recorder.run('measure.classResolve', 'measure', args.classSelector, () => page.$(args.classSelector));
  classEl = step._value || null;
  result.measurements.classResolve = step.outcome;
}

// —— 测量 2：元素文本读取 ——
if (classEl) {
  const step = await recorder.run('measure.textRead', 'measure', args.classSelector, () => classEl.text());
  const actual = step.outcome === 'resolved' ? String(step._value) : '';
  result.measurements.textRead = {
    selector: args.classSelector,
    expected: args.classExpectText,
    actual,
    matched: step.outcome === 'resolved' && actual.trim() === args.classExpectText,
    outcome: step.outcome,
  };
} else {
  recorder.rec('measure.textRead', 'measure', args.classSelector, 'skipped', { note: '类选择器未解析出元素' });
  result.measurements.textRead = { selector: args.classSelector, expected: args.classExpectText, actual: '', matched: false, outcome: 'skipped' };
}

// —— 测量 3：点击 → 状态变化（不掺导航 API 的可用性）——
{
  const before = await recorder.run('measure.tapBefore', 'measure', args.tapSelector, async () => {
    const el = await withTimeout(page.$(args.tapSelector), args.stepTimeoutMs, 'tapBefore($)');
    if (!el) return null;
    return withTimeout(el.text(), args.stepTimeoutMs, 'tapBefore(text)');
  });
  let afterText = '';
  let tapStep = null;
  if (before.outcome === 'resolved') {
    tapStep = await recorder.run('measure.tap', 'measure', args.tapSelector, async () => {
      const el = await withTimeout(page.$(args.tapSelector), args.stepTimeoutMs, 'tap($)');
      if (!el) throw new Error('tap 前重新查询未取到元素');
      await withTimeout(el.tap(), args.stepTimeoutMs, 'tap()');
      return 'tapped';
    });
    if (tapStep.outcome === 'resolved') {
      await new Promise((r) => setTimeout(r, args.settleMs));
      const after = await recorder.run('measure.tapAfter', 'measure', args.tapSelector, async () => {
        const el = await withTimeout(page.$(args.tapSelector), args.stepTimeoutMs, 'tapAfter($)');
        if (!el) return null;
        return withTimeout(el.text(), args.stepTimeoutMs, 'tapAfter(text)');
      });
      afterText = after.outcome === 'resolved' ? String(after._value) : '';
    }
  } else if (before.outcome === 'resolved-null') {
    recorder.rec('measure.tap', 'measure', args.tapSelector, 'skipped', { note: '点击靶点未解析出元素（可能是 v-if 未渲染）' });
  }
  const beforeText = before.outcome === 'resolved' ? String(before._value) : '';
  result.measurements.tapToggled = {
    selector: args.tapSelector,
    before: beforeText,
    after: afterText,
    changed: Boolean(beforeText) && Boolean(afterText) && beforeText !== afterText,
    beforeOutcome: before.outcome,
    tapOutcome: tapStep ? tapStep.outcome : 'skipped',
  };
}

// —— 测量 4/5：导航 API 原始行为（裁决 ADR:137 的「恒报 Uncaught [object Object]」）——
{
  const rl = await recorder.run('measure.reLaunch', 'nav', args.targetRoute, () => mp.reLaunch(args.targetRoute));
  result.measurements.reLaunch = { outcome: rl.outcome, error: rl.error || null };
  await new Promise((r) => setTimeout(r, args.settleMs));
  const nt = await recorder.run('measure.navigateTo', 'nav', args.targetRoute, () => mp.navigateTo(args.targetRoute));
  result.measurements.navigateTo = { outcome: nt.outcome, error: nt.error || null };
  result.navigationContradiction = classifyNavigation(
    { outcome: rl.outcome, ...(rl.error || {}) },
    { outcome: nt.outcome, ...(nt.error || {}) },
  );
}

result.verdict = computeVerdict(result.measurements, { ok: result.trustworthy });
result.channelImpact = VERDICT_IMPACT[result.verdict];
console.log('[verdict] ' + result.verdict + ' | ' + result.channelImpact);
finish(false);
