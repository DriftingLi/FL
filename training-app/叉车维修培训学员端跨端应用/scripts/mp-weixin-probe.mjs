// 微信开发者工具自动化探针——由 scripts/mp-weixin-check.ps1 调用，仅作**可选手动诊断**。
// 2026-11-05（#2320）：② 微信开发者工具 PR 门已整体退役，本探针不再作为验收门、不再截图取证；
// 仍可手动运行以查看 console error / exception / 白屏这类运行时读数，结果不参与判绿。
//
// 背景：spike issue #883 的实测坑位（勿删）：
//   1. `pageStack` 为空会让**所有 page 级 API** 报同一个误导弹错
//      `Cannot destructure property 'rawPath' of 't.getPageMetaByWebviewId(...)' as it is null`
//      —— 根因是自动化会话里没有页面被打开，不是版本不兼容。处置 = 调用方先 `cli.bat close` 再 `cli.bat auto`。
//   2. `page.$()` 元素级断言 2026-09-18 原测判不可用（就绪闸门早跑窗口的产物）→ 2026-10-25 #1917 裁决证伪
//      （`page.$('.app-title')` 8ms 可用、`element.text()` 精确匹配可用）→ #1919 落锁收窄：
//      `page.waitFor(选择器)` 与导航 API 仍禁；`page.$` + `element.text()` 锚点断言放行（仅直达模式）。
//
// 2026-09-12 复测（**诚实降级，不许假绿**）：本机 `miniprogram-automator@0.12.1` 下 `mp.reLaunch` / `mp.navigateTo`
//   **恒报 `Uncaught [object Object]`**，而 `mp.connect` / `mp.pageStack` 正常 ⇒「逐页导航」这组断言在该环境
//   **不可能成立**。处置：
//     - 捕获导航异常后**不把它算门通过、也不算门失败**，而是把这组断言记成 **SKIP（reason=navigation-api-unsupported）**，
//       写进 JSON 的 `navigation` 字段、`assertions`（值取字符串 `'skip'`）与结果行（调用方渲染 `navigation=skip(unsupported)`）；
//     - 可验证子集 = `connected` + `pageStack` 非空 + 入口页在栈内 + `errorsTotal==0 && exceptionsTotal==0`；
//     - `failures` 只由**可验证子集**产生 ⇒ 降级本身不产生 failures。
//   **未证实**：是否为版本组合（automator 0.12.1 × 开发者工具 Stable v2.01.2510290）问题，留给后续排查。
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
//   node mp-weixin-probe.mjs --ws ws://127.0.0.1:9420 --routes pages/index/index,pages/login/login
//        --entry-url pages/index/index [--timeout-ms 30000] [--page-timeout-ms 10000]
//   任意页直达（#1682 · ADR-0038 决策1）：node mp-weixin-probe.mjs --ws <端点> --expect-page <路由> ——
//        调用方已把构建产物 app.json 首项临时换成目标页并冷启动；本模式**不调用任何导航 API**，
//        只断言「冷启动落地的当前页 == 目标页」+ console/exception 读数。
//   达成 ⇒ navigation=ok(detail=cold-start-direct)；未达成 ⇒ navigation=skip(reason=direct-page-unsupported)
//   —— 诚实降级：SKIP 既不冒充门失败，也不冒充逐页取证成立。
// 输出：最后一行固定为 JSON（前缀 `MP_WEIXIN_PROBE `），由调用方解析 —— **只看输出，不看退出码**。
// 退出码：0 = 断言全过；1 = 有断言未过（真报错）；2 = 环境不可用（连不上 / 缺 automator / pageStack 空）。
// 说明：断言结果同时写进 JSON 的 assertions/failures/probeOk，调用方以 JSON 为准。

import path from 'node:path';
import { createRequire } from 'node:module';

function parseArgs(argv) {
  const out = {
    ws: 'ws://127.0.0.1:9420', routes: [], entryUrl: '', requireRoot: '',
    timeoutMs: 30000, pageTimeoutMs: 10000, settleMs: 2000, tailMax: 40, expectPage: '',
    assertSelectors: [], assertTexts: [],
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const val = () => argv[++i];
    if (a === '--ws') out.ws = val();
    else if (a === '--routes') out.routes = String(val() || '').split(',').map((s) => s.trim()).filter(Boolean);
    else if (a === '--entry-url') out.entryUrl = val();
    else if (a === '--require-root') out.requireRoot = val();
    else if (a === '--timeout-ms') out.timeoutMs = Number(val());
    else if (a === '--page-timeout-ms') out.pageTimeoutMs = Number(val());
    else if (a === '--expect-page') out.expectPage = String(val() || '').trim();
    else if (a === '--assert-selector') out.assertSelectors.push(String(val() || '').trim());
    else if (a === '--assert-text') out.assertTexts.push(String(val() || '').trim());
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
const NAV_SKIP_ASSERTIONS = ['allRoutesVisited'];
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

// ---------- 平台层噪音白名单（2026-10-29 立；2026-11-02 #2164 扩充跨条共现）----------
// ⚠️ 回滚单位自 #2164 起**不再是**「只动这一段」：= 两段常量（PATTERNS + CONTEXT_RULES）
//   + platformNoiseOf 签名（第二参 runNoiseIds）+ F3 守护的源码切片，两处一起撤才干净。
//   来路与裁定全文见票面 #2164（维护者 2026-11-02 Q1/Q2/Q3=(a)）。
// 病根：开发者工具在「page stack 非空时又 appLaunch」会向 console 打一条 **error**。cold-start-direct
// （#1682 逐页直达）每次直达到非入口页都必然触发，且实测**三页逐字相同** ⇒ `noConsoleErrors` 恒假、
// ② 在新形态下**永远拿不到绿**（读数见 #2107 全量实测：② 834 s 且判红；#2099 盘点的判据快照）。
// 它不是被测代码的问题（平台自身行为），所以不计入 `errorsTotal`。
//
// ⚠️ 过滤**必须可见**（这是本改动的承重条款，删等于重新引入「覆盖为零的绿」）：
//   命中的条数进 `noiseTotal`、原文进 `noiseTail`，读数行里与 errorsTotal 并列输出 `noise=<N>`；
//   消费者（mp-weixin-check.ps1）把它写进 metric 行的 optional `noise` 字段。
//   某天这条白名单吞掉了真错误 ⇒ 读数里 noise 非 0，一眼能看出「这一跑过滤过东西」。
//
// 判据刻意**窄**：只认 appLaunch 这一族，且必须在锚词之后 200 字符的**同一个窗口**里同时出现
// 「non-empty」与「page stack」——这两个词的**语序不固定**（实测读到过 `non-empty page stack` 与
// `page stack is non-empty` 两种形态，写死语序会让白名单在换个工具版本后静默失效 ⇒ 又变回恒假红）。
// 另：只对 `type === 'error'` 生效。宁可漏（漏了就还是红，安全默认），不可宽（宽了就静默吞真错）。
const PLATFORM_NOISE_PATTERNS = [
  {
    id: 'appLaunchNonEmptyStack',
    anchor: /appLaunch/i,
    also: [/non-?empty/i, /page\s*stack/i],
    window: 200,
  },
  // #2211（2026-11-02 维护者裁定 Q1/Q2，两族各自最窄共现，不合并成一条宽判据）：
  //   A. `routeDone with a webviewId 5 is not found` —— 逐页直达后旧 webview 已销毁，
  //      路由回调仍打到它身上。窄口径：**锚 routeDone ∧ 同窗同时出现 webviewId 与 is not found**。
  //      「is not found」单独出现（如文件没找到）缺 webviewId ⇒ 不放行。
  //   B. `SystemError (appServiceSDKScriptError) … reading 'screenHeight'` —— 平台在无活跃
  //      webview 时读到的空对象上取属性。窄口径：**锚 appServiceSDKScriptError ∧ 同窗同时出现
  //      `reading '<某属性>'` 与该属性名 screenHeight**。screenHeight 只是**共现词**（不是锚），
  //      业务代码里自己读 screenHeight 报的 TypeError 不带 appServiceSDKScriptError ⇒ 照旧红。
  //   两族都只对 `type === 'error'` 生效，且都要在锚之后 200 字符的同一窗口内凑齐，宁可漏不可宽。
  {
    id: 'routeDoneWebviewMissing',
    anchor: /routeDone/i,
    also: [/webviewId/i, /is\s*not\s*found/i],
    window: 200,
  },
  {
    id: 'appServiceNullScreenHeight',
    anchor: /appServiceSDKScriptError/i,
    also: [/reading\s*'[^']+'/i, /screenHeight/i],
    window: 200,
  },
];

// #2164 跨条共现规则（2026-11-02 维护者裁定 Q1=(a)，最窄形态，Q2=(a) 不扩背书）：
// 同页还会打一条 {"type":"error","args":[{}]} —— Error 的 message/stack 非可枚举，
// 原帧经 JSON 序列化被抹平成空对象 ⇒ 探针侧可锚文本只剩 '[object Object]'，零区分度，
// 多次红跑逐字节录恒定同一形态、不会多出可锚信息（#2107 留档 + #2164 三页红跑）。
// 单看它谁也可能是真错 ⇒ 放行不靠文本靠**同页同跑的跨条共现**：本次跑此前已放行过
// appLaunchNonEmptyStack，且下列条件逐条全成立才算噪音：
//   error 级（函数入口判）∧ args 恰好一个 ∧ 该参是非字符串的对象 ∧ 渲染文本恰为 [object Object]
//   ∧ 同跑已放行过 requires 背书的噪音 id。
// **条序不入判据**：这条 error 与 appLaunch 是否恒成对/恒有序，属未证事实（AC7）——真跑取证
// 若发现不同页不总是成对 ⇒ 停下回报，不自行放宽或收紧。routeDone / screenHeight 两族**不并入本条**
// ——它们各有独立锚词，已由 #2211 在上表按维护者裁定的窄共现放行（不跨条背书、不看条序）。
// 宁可漏（漏了照旧红），不可宽（宽了静默吞真错）。
const PLATFORM_NOISE_CONTEXT_RULES = [
  {
    id: 'appLaunchCompanionEmptyObject',
    requires: 'appLaunchNonEmptyStack',
    textEquals: '[object Object]',
  },
];

function platformNoiseOf(entry, runNoiseIds) {
  if (!entry || entry.type !== 'error') return null;
  const text = entry.args.map((v) => (typeof v === 'string' ? v : String(v))).join(' ');
  for (const p of PLATFORM_NOISE_PATTERNS) {
    const m = p.anchor.exec(text);
    if (!m) continue;
    const near = text.slice(m.index, m.index + (p.window || 200));
    if (p.also.every((re) => re.test(near))) return { id: p.id, text: text.slice(0, 200) };
  }
  const seen = Array.isArray(runNoiseIds) ? runNoiseIds : [];
  for (const r of PLATFORM_NOISE_CONTEXT_RULES) {
    if (!seen.includes(r.requires)) continue;
    if (entry.args.length !== 1) continue;
    const a0 = entry.args[0];
    if (typeof a0 !== 'object' || a0 === null || Array.isArray(a0)) continue;
    if (text !== r.textEquals) continue;
    return { id: r.id, text: text + ' (co-noise=' + r.requires + ')' };
  }
  return null;
}

const rel = (p) => String(p || '').replace(/^\//, '');
const started = Date.now();
const result = {
  ws: args.ws, connected: false, automatorFrom: '', routes: args.routes,
  pageStack: [], entryPage: '', steps: [],
  logsTotal: 0, errorsTotal: 0, exceptionsTotal: 0, consoleTail: [], elapsedMs: 0,
  // noiseTotal / noiseTail：被平台噪音白名单放行的条数与原文（2026-10-29，见 PLATFORM_NOISE_PATTERNS 注释）。
  // 与 errorsTotal **并列**上报——过滤可见，不是静默吞掉。
  noiseTotal: 0, noiseTail: [],
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
  };
  // 「逐页导航」这组：导航 API 不可用时**记 SKIP**（值 = 字符串 'skip'）——
  // 既不写 true（那是假绿），也不写 false（那不是被测代码的问题）；SKIP 不进 failures。
  // 判据取**导航本身的结果**（每一步的 ok = 导航后当前页 == 目标页），不拿「导航前读到的 pageStack」
  // 去比所有路由——后者恒假（pageStack 在导航前只读一次），会让健康机器上的门永远过不去。
  const navSteps = result.steps.filter((s) => s.navigate !== 'none');
  // 直达模式（#1682）：routes 为空、navigate 全 'none' ⇒ 逐页组判据换成「直达达成」
  const directMode = Boolean(args.expectPage);
  assertions.allRoutesVisited = navSupported
    ? (directMode
      ? result.steps.some((s) => s.direct === true && s.ok === true)
      : (args.routes.length > 0 && navSteps.length === args.routes.length && navSteps.every((s) => s.ok === true)))
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
// #2164 共现上下文：本跑已放行的噪音 id。cold-start-direct 一个进程 = 一个页 ⇒「本跑」即「同页」。
// 内部状态：不进 result、不给读数行加字段；只供 platformNoiseOf 判跨条共现，回滚随两段常量一起撤。
const runNoiseIds = [];
mp.on('console', (msg) => {
  const entry = logEntry(msg);
  result.logsTotal += 1;
  if (entry.type === 'error') {
    // 平台层噪音先分类，再决定是否计数（2026-10-29 F3）：命中白名单 ⇒ 记 noise、**不进** errorsTotal。
    // 尾部日志仍保留这条（type 改标 platform-noise），便于事后逐条核对到底放行了什么。
    const noise = platformNoiseOf(entry, runNoiseIds);
    if (noise) {
      result.noiseTotal += 1;
      runNoiseIds.push(noise.id);
      if (result.noiseTail.length < args.tailMax) result.noiseTail.push({ type: 'platform-noise', id: noise.id, args: [noise.text] });
      entry.type = 'platform-noise';
      entry.noiseId = noise.id;
    } else {
      result.errorsTotal += 1;
    }
  }
  if (result.consoleTail.length < args.tailMax) result.consoleTail.push(entry);
});
mp.on('exception', (e) => {
  result.exceptionsTotal += 1;
  if (result.consoleTail.length < args.tailMax) {
    result.consoleTail.push({ type: 'exception', args: [String((e && e.message) || e).slice(0, 300)] });
  }
});



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
// `Uncaught [object Object]`，见文件头 2026-09-12 复测段），只判「落地页 == 目标页」+ 读数。
// 达成 ⇒ navigation=ok(detail=cold-start-direct)；未达成 ⇒ SKIP(reason=direct-page-unsupported)：
// 机制不成立**不冒充门失败**（failures 不产生），也**不冒充逐页取证成立**（不写 true）。
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
  // 锚点断言（#1919）：锚点挂起 8s 只影响本次判定，不产生额外的证据面。
  result.anchors = await runAnchorAssertions(mp, anchorPairs, reached, result);
  result.steps.push(step);
  result.navigation = reached
    ? { status: 'ok', reason: '', detail: 'cold-start-direct', at: 'direct:' + target, skippedAssertions: [], skippedSteps: [] }
    : { status: 'skip', reason: 'direct-page-unsupported', detail: 'direct-page-unsupported', at: 'direct:' + target, skippedAssertions: [], skippedSteps: [] };
  console.log('[direct] ' + (reached ? 'ok' : 'skip') + ' page=' + target + ' current=' + step.currentPage
    + ' errors=' + result.errorsTotal + ' noise=' + result.noiseTotal
    + ' exceptions=' + result.exceptionsTotal
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
    + ' ⇒ 逐页导航记 SKIP（reason=' + NAV_SKIP_DETAIL + '），不计入门失败；门只对可验证子集作结论。');
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
  // 让页面把异步 console（网络回调等）打完再进下一步
  await new Promise((r) => setTimeout(r, args.settleMs / Math.max(1, args.routes.length)));
}

if (!navSkipped) result.navigation = { status: 'ok', reason: '', detail: '', at: '', skippedAssertions: [], skippedSteps: [] };

// 收尾：给被改页的异步日志一点时间（#883 实测入口页有请求回调日志）
await new Promise((r) => setTimeout(r, args.settleMs));
finish(false);
