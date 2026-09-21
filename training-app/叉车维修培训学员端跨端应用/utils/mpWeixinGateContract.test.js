/**
 * ② 微信开发者工具门（scripts/mp-weixin-check.ps1 + scripts/mp-weixin-probe.mjs）契约守护
 *
 * 背景：#883 spike 证实 ② 可完全无人值守执行（HBuilderX `publish mp-weixin` 构建并自己拉起开发者工具
 * → `cli.bat auto` 开自动化端口 → `miniprogram-automator` 连上读 pageStack / console / exceptions 并截图）。
 * 2026-09-12 修订把 ② 从**人工门**降为**半自动门**（与 ④a 同款口径，#870 裁定 B）。它的价值全押在
 * 下面几条不变量上，本测试就是守这几条：
 *   C1 输入 = HBuilderX CLI 构建产物（unpackage\dist\build\mp-weixin），不另造构建；探针在同目录 scripts/
 *   C2 顺序写死 `cli.bat close` → `cli.bat auto`；**auto 必须等它跑完**（中途 kill ⇒ 端口永不监听，2026-09-12 实测）
 *   C3 「pageStack 非空」必须是**前置断言**；探针不得用元素级 API（page.$() 会挂起 15s，且会触发 `$(` 检测）
 *   C4 判成败只看输出：不得出现 $LASTEXITCODE；必须解析 MP_WEIXIN_PROBE / MP_WEIXIN_RESULT 与 publish 文案
 *   C5 端口监听面风险必须写明（实测绑 `::` 通配地址，CLI 无法收窄），且必须轮询端口延迟出现
 *   C6 结果评论格式 == `gate-evidence:②` + `commit: <sha>`；只在门通过（exit 0）分支贴
 *   C7 `cli.bat close` 在 finally 路径里执行 + 幂等保护（避免端口/会话残留）
 *   C8 非等价声明 + 挡不住的类别 + **源 manifest appid 前置断言**（HBuilderX 首次导入会回写 manifest 置 null）
 *   C9 截图入库纪律（P2）：WebP→JPEG 回退、宽 ≤720、单张 ≤150KB、合计 ≤1.5MB、-NoArchive、失败不阻塞门
 *   C10 注册与文档：package.json 脚本、ADR-0008、PR 模板、移动端 AGENTS.md 的人工门清单
 *   C11 HBuilderX 忙检测：必须 dot-source scripts/lib/hx-busy.ps1、等它给结论、**绝不 kill 主程序**
 *   C12 **② 的时序三步写死**（2026-09-12 复测）：`close → open --project <dist> → auto`；
 *       `open` 必须是**显式步骤 + 写进日志**，`-SkipBuild` 下同样能定位构建产物，且不得被 build 分支条件包住
 *   C13 **导航不可用时诚实降级**（2026-09-12 复测）：探针把「逐页导航 + 每页截图」记 SKIP
 *       （reason=navigation-api-unsupported）、SKIP **不得写成 PASS**、必须写进结果行与门评论；
 *       门只对可验证子集作结论（failures 只由该子集产生）；结果行字段清单落在门计划里（见 C18）
 *   C14 贴评论调用**不得断链**：语句续行符缺失会让 `-ArchivedRel …` 变成另一条命令（曾真实存在）。
 *       **2026-09-13 收窄（#914）**：只对**代码行**判 —— 帮助文本里的 bullet（`-Pages` / `-AllowAppStart`
 *       这类）根本不是续行参数，旧口径在 device-capture.ps1 上误报 3 条
 *   C15 **就绪闸门**（2026-09-13 实测定位）：`auto` 之后、探针之前必须先用 `scripts/mp-weixin-ready.mjs`
 *       等齐「`Tool.getInfo` 带 `SDKVersion`」与「`App.getPageStack` 开始应答」两个里程碑；探针超时抬到
 *       `-ProbeTimeoutMs`；结果行必须带 `ready=` / `sdk=` / `ide=`
 *   C16 失败**归因**：探针必须用 `connectError.kind` 把「会话未就绪」与「端口连不上」分开报；
 *       脚本不得再把「自动化端口连不上」当作「重试无益」——那一类恰恰是**短暂**的（实测 24–34s 才就绪）
 *   C17 `-Doctor` 体检模式：只体检、不构建、不跑探针、不贴评论、不入库、**不产出门的通过结论**；
 *       逐层 OK/WARN/FAIL + 独立退出码，报告必须在 finally 里打印（早期 exit 也要有结论）
 *   C18 **门计划是「步骤序列 / 每步参数与判据 / 超时预算与 exit 2 归属 / 结果行字段清单」的唯一真源**
 *       （#914 收束）：这四类不变量集中在脚本的 `New-GatePlan` 一处声明，执行路径按 id 取用
 *       （`Get-GateStep`），`-DryRun` 把同一份数据打成单行 `GATE_PLAN {json}`。守护**跑 `-DryRun` 并断言
 *       这份 JSON**（不再是调用点源码文本）⇒ 把调用点原文抽进共享库时守护不会失配。
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/kotlinAllGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 *
 * 为什么 C2 / C12 / C15 / C18 走「跑脚本 + 断言 JSON」而不是读源码文本（#914，ADR-0008「守护从断言文本
 * 改为断言行为」）：文本口径下，任何把被 pin 的调用点原文抽进共享库的重构都会让守护失配，而唯一的
 * 「修法」是放宽守护（等于把防护拆掉）⇒ 门脚本家族被自己的守护冻在重复状态。改成断言计划的 JSON 后，
 * 「顺序/参数/预算」是**直接观测**，搬运不再与守护冲突。
 * 代价（写实）：守护多了一个运行时依赖 —— 需要 `pwsh`（Windows 本机与 CI 的 ubuntu runner 都自带
 * PowerShell 7；CI 的 mobile-test job 已在跑 `npm run test:unit`）。拿不到 pwsh 或计划不可解析时
 * **fail-closed**：直接抛，不退化成「跳过」。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const DEVICE_CAPTURE_REL = 'scripts/device-capture.ps1';
const PROBE_REL = 'scripts/mp-weixin-probe.mjs';
const READY_REL = 'scripts/mp-weixin-ready.mjs';
const PKG_REL = 'package.json';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
const AGENTS_REL = 'AGENTS.md';
const PR_TEMPLATE_REL = path.join('..', '..', '.github', 'PULL_REQUEST_TEMPLATE.md');

// 门计划的步骤序列（② 的全部外部动作，按执行顺序；-DryRun 输出的 steps[].id 必须逐字等于它）
const PLAN_SEQUENCE = [
  'devtools-close',
  'devtools-open',
  'devtools-auto',
  'port-listening',
  'ready-gate',
  'automator-probe'
];
// 结果行必填字段（顺序即输出顺序；可选字段单列，见 C13）
const RESULT_MANDATORY = [
  'appid', 'pageStack', 'entry', 'errorsTotal', 'exceptionsTotal', 'logsTotal',
  'shots', 'navigation', 'ready', 'sdk', 'ide', 'log'
];
const RESULT_OPTIONAL = ['navigationSkipReason'];

/**
 * 取一个 PowerShell 函数的函数体（花括号配对）。
 * ⚠️ **不要用注释行当结束锚点**：这些判据跑在 stripCommentLines 之后的代码文本上，注释已变空行
 * （#1209 实测：拿 `# ---------- 主流程 ----------` 当锚点 ⇒ 永远定位不到函数体、判据空跑）。
 */
function psFunctionBody(code, decl) {
  const at = code.indexOf(decl);
  if (at === -1) return '';
  const open = code.indexOf('{', at);
  if (open === -1) return '';
  let depth = 0;
  for (let i = open; i < code.length; i++) {
    if (code[i] === '{') depth++;
    else if (code[i] === '}') { depth--; if (depth === 0) return code.slice(open, i + 1); }
  }
  return '';
}

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/**
 * 注释剥离：**只把注释行置空，行号保持不变**。
 * C14（续行判据）与 C18（调用点原文出现次数）都要「代码行」口径 —— 帮助文本里的 bullet 与
 * 注释里引用的原文都不是代码，参与判定只会制造误报。
 */
function stripCommentLines(text) {
  const lines = String(text).split(/\r?\n/);
  const out = [];
  let inBlock = false;
  for (const line of lines) {
    const t = line.trim();
    if (inBlock) {
      out.push('');
      if (t.includes('#>')) inBlock = false;
      continue;
    }
    if (t.startsWith('<#')) {
      out.push('');
      if (!t.includes('#>')) inBlock = true;
      continue;
    }
    if (t.startsWith('#')) { out.push(''); continue; }
    out.push(line);
  }
  return out;
}

// ---------- 门计划：跑 -DryRun 取 JSON（fail-closed，拿不到就抛，不退化成跳过）----------

const PLAN_PREFIX = 'GATE_PLAN ';

/** PowerShell 宿主：本仓 npm 脚本与门脚本都用 pwsh（Windows 本机与 CI ubuntu runner 都自带）。 */
function powershellExe() {
  return 'pwsh';
}

function dryRunArgs(scriptPath, projectDir) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass');
  args.push('-File', scriptPath, '-DryRun', '-Project', projectDir);
  return args;
}

function runDryRun(scriptPath, projectDir) {
  const exe = powershellExe();
  const args = dryRunArgs(scriptPath, projectDir);
  try {
    return execFileSync(exe, args, {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024
    });
  } catch (e) {
    throw new Error(
      `-DryRun 执行失败：${exe} ${args.join(' ')}\n`
      + `exit=${e.status} signal=${e.signal}\n`
      + `stdout=${e.stdout}\nstderr=${e.stderr}\n`
      + '（守护要跑脚本取门计划：pwsh 不可用时 fail-closed，不跳过）'
    );
  }
}

/** -DryRun 的输出必须**恰好一行** `GATE_PLAN {json}`，否则 fail-closed。 */
function parsePlan(stdout) {
  const all = String(stdout).split(/\r?\n/);
  const hits = all.filter((l) => l.startsWith(PLAN_PREFIX));
  if (hits.length !== 1) {
    throw new Error(`-DryRun 必须只输出一行以 "GATE_PLAN " 开头的 JSON，实际 ${hits.length} 行；完整 stdout：\n${stdout}`);
  }
  return JSON.parse(hits[0].slice(PLAN_PREFIX.length));
}

/**
 * 纯函数：门计划 JSON → 违规清单（C2 / C12 / C13 / C15 的计划面 + C18 计划完整性）。
 * 被「真实 -DryRun 输出」与「注入变形的计划」共用 —— 注入用例能检出违规，才证明这些判据不是空跑。
 */
function scanPlan(plan) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };
  const norm = (p) => String(p === undefined || p === null ? '' : p).replace(/\\/g, '/');
  const steps = plan && Array.isArray(plan.steps) ? plan.steps : [];
  const ids = steps.map((s) => s.id);
  const at = (id) => ids.indexOf(id);
  const step = (id) => steps.find((s) => s.id === id);
  const argvOf = (s) => (s && Array.isArray(s.argv) ? s.argv : []);
  const argAfter = (s, flag) => {
    const a = argvOf(s);
    const i = a.indexOf(flag);
    return i === -1 ? undefined : a[i + 1];
  };
  const P = (plan && plan.paths) || {};
  const T = (plan && plan.timeouts) || {};
  const E = (plan && plan.endpoint) || {};

  // ---- C18 计划完整性：接口版本 + 六步序列 + 每步都有 tag/判据/exit2 归属 ----
  must(plan && plan.schema === 'gate-plan/1', 'C18', '计划缺 schema = gate-plan/1（守护与脚本的接口版本，改了要同步）');
  must(ids.join(' → ') === PLAN_SEQUENCE.join(' → '), 'C18', `步骤序列不是写死的那条（应 ${PLAN_SEQUENCE.join(' → ')}，实际 ${ids.join(' → ') || '（空）'}）`);
  steps.forEach((s) => {
    must(Boolean(s.tag), 'C18', `步骤 ${s.id} 缺 tag（执行点与日志都要用它）`);
    must(Array.isArray(s.criteria) && s.criteria.length > 0, 'C18', `步骤 ${s.id} 缺判据（criteria）—— 「每步判据」是计划必须覆盖的四类内容之一`);
    must(Array.isArray(s.exit2), 'C18', `步骤 ${s.id} 没有 exit2 归属声明（数组，可以是空）`);
  });
  // 「谁决定 exit 2」：每步声明的 exit2 必须摊平进汇总视图，且每条都写明 reason
  const flat = [];
  steps.forEach((s) => (Array.isArray(s.exit2) ? s.exit2 : []).forEach((e) => flat.push(`${s.id}|${e.on}|${e.reason}`)));
  const summary = (Array.isArray(plan && plan.exit2) ? plan.exit2 : []).map((e) => `${e.step}|${e.on}|${e.reason}`);
  must(flat.join(',') === summary.join(','), 'C18', `exit 2 归属的汇总视图与各步骤声明不一致（步骤侧 ${flat.join(',')} / 汇总侧 ${summary.join(',')}）`);
  (Array.isArray(plan && plan.exit2) ? plan.exit2 : []).forEach((e) => {
    must(Boolean(e.reason), 'C18', `exit2 ${e.step}/${e.on} 没写 reason（「谁决定 exit 2」必须能机检）`);
  });
  // close 是「失败只警告」的步骤：它声明 exit 2 就意味着门的退出语义被改宽了
  const closeStep = step('devtools-close');
  must(Boolean(closeStep) && (closeStep.exit2 || []).length === 0, 'C18', 'close 是「失败只警告」的步骤，不该声明 exit 2');
  // 预算与步骤不能分叉：步骤上的超时值必须逐项等于预算里的对应项
  [['devtools-open', 'open', 'timeoutSeconds'],
    ['devtools-auto', 'auto', 'timeoutSeconds'],
    ['port-listening', 'portWait', 'waitSeconds'],
    ['ready-gate', 'readyGate', 'timeoutSeconds'],
    ['automator-probe', 'probe', 'timeoutSeconds']].forEach(([id, key, field]) => {
    const s = step(id);
    must(Boolean(s) && s[field] === T[key], 'C18', `步骤 ${id} 的 ${field}（${s ? s[field] : '缺步骤'}）与预算 timeouts.${key}（${T[key]}）不一致 —— 预算与步骤分叉了`);
  });
  // 路径面：执行点用的目录/脚本必须是计划里那几个（相对路径写死，绝对路径由项目根派生）
  must(P.distRelative === 'unpackage\\dist\\build\\mp-weixin', 'C18', '构建产物相对路径不是 unpackage\\dist\\build\\mp-weixin');
  must(P.probeRelative === 'scripts\\mp-weixin-probe.mjs', 'C18', '探针相对路径不是 scripts\\mp-weixin-probe.mjs');
  must(P.readyRelative === 'scripts\\mp-weixin-ready.mjs', 'C18', '就绪闸门相对路径不是 scripts\\mp-weixin-ready.mjs');
  must(norm(P.dist).endsWith(norm(P.distRelative)) && norm(P.dist).startsWith(norm(P.project)), 'C18', 'dist 不是「项目根 + 构建产物相对路径」派生的');
  must(norm(P.probe).endsWith(norm(P.probeRelative)), 'C18', 'probe 路径与 probeRelative 对不上');
  must(norm(P.ready).endsWith(norm(P.readyRelative)), 'C18', 'ready 路径与 readyRelative 对不上');
  must(Number.isFinite(E.port) && E.port > 0, 'C18', '自动化端口不是正整数');
  must(E.ws === `ws://127.0.0.1:${E.port}`, 'C18', '端点 ws 与端口对不上');

  // ---- C2：close → auto 的顺序；auto 必须跑完；参数与端口写死在计划里 ----
  const autoStep = step('devtools-auto');
  must(at('devtools-close') !== -1 && at('devtools-auto') !== -1 && at('devtools-close') < at('devtools-auto'),
    'C2', '`close` 未出现在 `auto` 之前（顺序错了会撞 pageStack 空）');
  must(Boolean(closeStep) && argvOf(closeStep)[0] === 'close' && argvOf(closeStep)[1] === '--project',
    'C2', 'close 的 argv 不是 `close --project <dist>`');
  must(Boolean(autoStep), 'C2', '计划里没有 auto 步骤（开自动化端口）');
  if (autoStep) {
    must(argvOf(autoStep)[0] === 'auto', 'C2', 'auto 的 argv 不以 `auto` 开头');
    must(argvOf(autoStep)[1] === '--project' && argvOf(autoStep)[2] === P.dist,
      'C2', 'auto 未以构建产物目录作为 --project');
    must(argAfter(autoStep, '--auto-port') === String(E.port), 'C2', 'auto 的 --auto-port 与计划端点端口不一致（端口写死在计划里）');
    must(argvOf(autoStep).includes('--trust-project'), 'C2', 'auto 缺 --trust-project');
    must(autoStep.awaitCompletion === true, 'C2', 'auto 未声明「必须等它跑完」（中途 kill ⇒ 端口永不监听）');
    must(Number.isFinite(autoStep.timeoutSeconds) && autoStep.timeoutSeconds > 0, 'C2', 'auto 缺超时兜底');
    must((autoStep.exit2 || []).some((e) => e.on === 'timeout'), 'C2', 'auto 未声明「超时 ⇒ exit 2」');
  }

  // ---- C12：close → open --project <dist> → auto；open 显式、写日志、-SkipBuild 下同样定位产物 ----
  const openStep = step('devtools-open');
  must(Boolean(openStep), 'C12', '计划里没有 `cli.bat open --project <dist>`（缺这步 pageStack 恒空）');
  must(at('devtools-close') !== -1 && at('devtools-open') !== -1 && at('devtools-auto') !== -1
    && at('devtools-close') < at('devtools-open') && at('devtools-open') < at('devtools-auto'),
    'C12', '顺序不是 `close → open --project <dist> → auto`');
  if (openStep) {
    must(argvOf(openStep)[0] === 'open' && argvOf(openStep)[1] === '--project', 'C12', 'open 的 argv 不是 `open --project <dist>`');
    must(argvOf(openStep)[2] === P.dist, 'C12', 'open 的项目目录不是构建产物目录（-SkipBuild 下会定位不到）');
    must(openStep.logged === true, 'C12', 'open 未声明要写进日志（须与 close/auto 一样可追溯）');
    must(Number.isFinite(openStep.timeoutSeconds) && openStep.timeoutSeconds > 0, 'C12', 'open 缺超时兜底');
    must((openStep.exit2 || []).some((e) => e.on === 'timeout'), 'C12', 'open 未声明「超时 ⇒ exit 2」（窗口没重开就没法继续）');
  }

  // ---- C15：auto 之后、探针之前跑就绪闸门；判据与预算写在计划里 ----
  const readyStep = step('ready-gate');
  must(at('devtools-auto') !== -1 && at('ready-gate') !== -1 && at('devtools-auto') < at('ready-gate'),
    'C15', '就绪闸门未出现在 auto 之后（顺序错了等于没等）');
  must(at('ready-gate') !== -1 && at('automator-probe') !== -1 && at('ready-gate') < at('automator-probe'),
    'C15', '就绪闸门未出现在探针之前（端点没就绪就交给了 automator）');
  if (readyStep) {
    must(argAfter(readyStep, '--ws') === E.ws, 'C15', '就绪闸门的 --ws 不是计划里的自动化端点');
    must(argAfter(readyStep, '--wait-seconds') === String(T.readyWait), 'C15', '就绪闸门的 --wait-seconds 与预算 timeouts.readyWait 不一致');
    must(argvOf(readyStep).includes('--require-stack'), 'C15', '就绪闸门未要求 pageStack 开始应答（只等 SDKVersion 不够）');
    must(argvOf(readyStep).includes('--require-root'), 'C15', '就绪闸门缺 --require-root（automator 解析不到）');
    const criterion = (readyStep.criteria || []).join(' ');
    must(criterion.includes('SDKVersion'), 'C15', '就绪闸门的判据未写明「`Tool.getInfo` 须带 SDKVersion」');
    must(criterion.includes('App.getPageStack'), 'C15', '就绪闸门的判据未写明「`App.getPageStack` 须开始应答」');
    must((readyStep.exit2 || []).some((e) => e.on === 'not-ready' && e.reason === 'automation-not-ready'),
      'C15', '就绪闸门未声明「未就绪 ⇒ exit 2 reason=automation-not-ready」');
  }
  must(Number.isFinite(T.probeTimeoutMs) && T.probeTimeoutMs >= 60000, 'C15', 'ProbeTimeoutMs 预算不足 60s（实测就绪窗口上界 34s）');
  must(argAfter(step('automator-probe'), '--timeout-ms') === String(T.probeTimeoutMs),
    'C15', '探针的 --timeout-ms 与预算 timeouts.probeTimeoutMs 不一致');

  // ---- C13（结果行字段清单）：必填字段逐字与顺序都对，可选字段只有 navigationSkipReason ----
  const rl = (plan && plan.resultLine) || {};
  must(rl.prefix === 'MP_WEIXIN_RESULT', 'C13', '结果行前缀不是 MP_WEIXIN_RESULT');
  const fields = Array.isArray(rl.fields) ? rl.fields : [];
  const mandatory = fields.filter((f) => !f.optional).map((f) => f.name);
  const optional = fields.filter((f) => f.optional).map((f) => f.name);
  must(mandatory.join(',') === RESULT_MANDATORY.join(','),
    'C13', `结果行必填字段须恰好为（含顺序）${RESULT_MANDATORY.join(',')}，实际 ${mandatory.join(',') || '（空）'}`);
  must(optional.join(',') === RESULT_OPTIONAL.join(','),
    'C13', `结果行可选字段须恰好为 ${RESULT_OPTIONAL.join(',')}，实际 ${optional.join(',') || '（空）'}`);
  must(fields.length > 0 && fields[fields.length - 1].name === 'log' && !fields[fields.length - 1].optional,
    'C18', 'log 应是结果行最后一个字段（与既有日志格式一致）');

  return violations;
}

/** 纯函数：七份源码文本 → 违规清单（文本面规则；C2/C12/C13/C15 的顺序·参数·预算面已搬去 scanPlan） */
function scanContract(sources) {
  const violations = [];
  const s = sources.script || '';
  const probe = sources.probe || '';
  const ready = sources.ready || '';
  const pkg = sources.pkg || '';
  const adr = sources.adr || '';
  const agents = sources.agents || '';
  const template = sources.template || '';
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  // C1 输入/产物路径
  must(s.includes("$DistRelative = 'unpackage\\dist\\build\\mp-weixin'"), 'C1', '未把输入口径写死为 unpackage\\dist\\build\\mp-weixin');
  must(s.includes("$ProbeRelative = 'scripts\\mp-weixin-probe.mjs'"), 'C1', '未引用同目录探针 scripts\\mp-weixin-probe.mjs');

  // C2 执行侧只留一件：超时兜底（顺序/参数/预算已由 scanPlan 断言门计划）。
  // 计划的 exit2 声明了「timeout ⇒ exit 2」，代码就得真的有 TimedOut 判据 —— 声明与实现两边都要有。
  must(s.includes('$auto.TimedOut'), 'C2', 'auto 缺超时兜底（执行侧必须有 TimedOut 判据，计划声明的 exit 2 才有落点）');

  // C3 pageStack 前置断言 + 禁止元素级 API
  must(probe.includes('pageStackNonEmpty'), 'C3', '探针缺 pageStack 非空前置断言');
  must(probe.includes('getPageMetaByWebviewId'), 'C3', '探针未记录 pageStack 空导致的误导弹错（#883 坑位 1）');
  probe.split(/\r?\n/).forEach((line, idx) => {
    if (/^\s*\/\//.test(line)) return; // 注释里提 API 名是允许的；本守护守的是「别用」
    if (/\$\(|getElementByXpath/.test(line) && !/pageStack|mp\.pageStack/.test(line)) {
      violations.push('C3 第 ' + (idx + 1) + ' 行用了元素级 API（page.$ 会挂起 15s，#883 坑位 3）');
    }
  });

  // C4 判成败只看输出
  must(!/\$LASTEXITCODE/.test(s), 'C4', '出现 $LASTEXITCODE：CLI 退出码恒为 0，判成败不得依赖退出码');
  must(s.includes('MP_WEIXIN_PROBE'), 'C4', '未解析探针的 MP_WEIXIN_PROBE 结果行（无输出判据）');
  must(s.includes('MP_WEIXIN_RESULT'), 'C4', '缺少 MP_WEIXIN_RESULT 结果行（无法机检成败）');
  must(s.includes('导出微信小程序成功'), 'C4', '未以构建产物文案（导出微信小程序成功）判定 publish 成败');

  // C5 端口监听面风险 + 端口延迟出现要轮询
  must(s.includes('通配'), 'C5', '未写明端口绑定通配地址的风险');
  must(s.includes('监听面'), 'C5', '未写明监听面无法从 CLI 侧收窄（不做加固的声明）');
  must(s.includes('$PortWaitSeconds'), 'C5', '缺端口轮询（auto 跑完后端口是延迟出现的）');

  // C6 评论格式 + 只在通过分支贴
  must(s.includes('gate-evidence:②'), 'C6', '未输出 gate-evidence:② 标记（pr-evidence 认不出这条评论证据）');
  must(s.includes('- commit: $sha'), 'C6', '评论缺 commit 行（无法与 PR head sha 绑定）');
  must(/\$PostToPr/.test(s), 'C6', '缺 -PostToPr 开关（门结果免手抄的入口）');
  const callAt = s.indexOf('Publish-GateComment -PrNumber');
  must(callAt !== -1, 'C6', '未调用 Publish-GateComment');
  if (callAt !== -1) must(s.indexOf('exit 1') < callAt, 'C6', 'exit 1 失败分支出现在贴评论调用之后（失败也会贴评论）');

  // C7 finally 路径 + 幂等
  const finallyAt = s.lastIndexOf('finally {');
  must(finallyAt !== -1, 'C7', '缺 finally 路径');
  if (finallyAt !== -1) must(s.slice(finallyAt).includes('Invoke-DevToolsClose'), 'C7', 'finally 路径里没有 `cli.bat close`（会残留端口/会话）');
  must(s.includes('$script:closed'), 'C7', 'close 未做幂等保护（$script:closed）');

  // C8 非等价声明 + 源 manifest appid 前置断言
  must(s.includes('非等价'), 'C8', '缺「非等价声明」（② ≠ ① 真机门 / ② ≠ ④b 云打包门）');
  must(s.includes('checkIsSupportSoterAuthentication'), 'C8', '未列出挡不住的类别（生物识别实测在开发者工具不可用）');
  must(s.includes('manifest-appid'), 'C8', '源 manifest appid 不匹配时未判门不过（fail-closed 缺一路径）');
  must(/mp-weixin"\\s\*:\\s\*\\{/.test(s) || s.includes('mp-weixin"'), 'C8', '缺源 manifest appid 前置断言（HBuilderX 首次导入会回写 manifest 置 null）');
  must(s.includes('不在 Page 上') || s.includes('Page 上'), 'C8', '未记录 screenshot 挂在 MiniProgram 而非 Page（#883 坑位 2）');

  // C9 截图入库纪律
  must(s.includes("$script:ArchiveMaxCount = 10"), 'C9', '入库张数上限未写死为 10');
  must(s.includes('$script:ArchiveMaxBytes = 150KB'), 'C9', '单张体积上限未写死为 150KB');
  must(s.includes('$script:ArchiveMaxTotal = 1.5MB'), 'C9', '合计体积上限未写死为 1.5MB');
  must(s.includes('$script:ArchiveMaxWidth = 720'), 'C9', '宽度上限未写死为 720');
  must(s.includes('docs/verification/'), 'C9', '入库落点不是 docs/verification/<模块>/<PR号>/');
  must(s.includes('-NoArchive'), 'C9', '缺 -NoArchive 逃生开关');
  must(/Get-WebpEncoder/.test(s) && /cwebp/.test(s) && /ffmpeg/.test(s) && /magick/.test(s), 'C9', '未探测 WebP 编码器（cwebp / ffmpeg / magick）');
  must(s.includes('System.Drawing') || s.includes('[System.Drawing.'), 'C9', '缺 JPEG 回退实现（System.Drawing）');
  must(/Save-Jpeg/.test(s) && /Quality/.test(s), 'C9', '缺 JPEG 质量控制（q75）');
  must(s.includes('不得让门因此失败') || s.includes('不影响门结论'), 'C9', '入库失败未声明「不得让门失败」');
  must(/^.*-after/s.test(s) || s.includes('-after'), 'C9', '产物命名缺 -after 后缀');
  must(s.includes('省了'), 'C9', '减图时未要求在评论里写明省了哪几页');
  must(s.includes('入库截图'), 'C9', '评论未用仓库内相对路径列出入库截图');

  // C11 HBuilderX 忙检测（单实例串行资源）：② 也必须接共享 helper，且不得 kill 主程序
  must(/(lib\/hx-busy\.ps1|lib\\hx-busy\.ps1)/.test(s), 'C11', '未 dot-source scripts/lib/hx-busy.ps1（HBuilderX 忙检测）');
  must(/Wait-HxFree/.test(s), 'C11', '未调用 Wait-HxFree（忙检测 + 等待上限）');
  must(/Release-HxLock/.test(s), 'C11', '未在 finally 里释放 agent 互斥锁');
  must(!/Stop-Process[^\r\n]*HBuilderX/.test(s), 'C11', '出现 kill 主程序的调用（HBuilderX 是共享单实例资源，绝不抢占）');
  must(s.includes('$HxWaitSeconds') && s.includes('$HxNoWait'), 'C11', '缺 -HxWaitSeconds / -HxNoWait 参数');

  // C12 执行侧只留一件：`open` 的**执行点**必须晚于构建分支的 no-dist 兜底（-SkipBuild 路径也要到得了它）。
  // 顺序/参数/日志由 scanPlan 断言计划，这里守的是「执行点在正确的位置」这一结构事实。
  // 锚点取**整条字面量**（不是裸 `no-dist`）：裸子串会被 `no-dist-X` 这类改名蒙过去（子串遮蔽，见 C17 的同类教训）。
  const openUseAt = s.indexOf("-Id 'devtools-open'");
  must(openUseAt !== -1, 'C12', '执行路径里没有按 id 取用 open 步骤（计划与执行脱节）');
  const lastDistGuard = s.indexOf("'MP_WEIXIN_RESULT errors=env reason=no-dist'");
  must(lastDistGuard !== -1, 'C12', '找不到 -SkipBuild 分支的 no-dist 兜底（open 的执行点晚于哪个锚点就判不出来了）');
  if (openUseAt !== -1 && lastDistGuard !== -1) {
    must(openUseAt > lastDistGuard, 'C12', 'open 的执行点落在构建分支条件内（-SkipBuild 路径到不了它）');
  }
  must(s.includes('$open.TimedOut'), 'C12', 'open 缺超时兜底（窗口没重开就没法继续；计划声明了 timeout ⇒ exit 2）');

  // C13 导航不可用时诚实降级：SKIP，不得写成 PASS；且必须写进结果行与门评论
  must(probe.includes('navigation-api-unsupported'), 'C13', '探针未记录降级原因 `navigation-api-unsupported`');
  must(/NAV_SKIP_ASSERTIONS/.test(probe), 'C13', '探针未声明「降级涉及哪几条断言」（allRoutesVisited / screenshotsProduced）');
  must(/'skip'/.test(probe), 'C13', '探针未把降级断言记为 SKIP（字符串 skip），有假绿风险');
  must(/if \(v === false\)/.test(probe), 'C13', '探针的 failures 不是「只由明确 false（可验证子集不过）」产生（SKIP 会被算成失败或 PASS 混入）');
  must(/isNavigationUnsupported/.test(probe), 'C13', '探针未区分「导航 API 不支持」与真失败（TIMEOUT 之类不得被降级吞掉）');
  must(probe.includes('currentPageScreenshot') || probe.includes('current.png'), 'C13', '探针降级后没有取「当前页」截图（可验证子集要求 ≥1 张）');
  must(s.includes('navigation = "$navField"'), 'C13', '结果行没有把 navigation 字段绑到 $navField（字段清单在计划里，取值在这里）');
  must(s.includes('navigation=SKIP reason='), 'C13', '日志缺 `navigation=SKIP reason=…` 行（降级必须写进日志）');
  must(!/navigation=PASS/.test(s), 'C13', '把导航降级写成了 PASS（降级不等于通过）');
  must(s.includes('降级（SKIP，非 PASS）'), 'C13', 'PR 门评论未显式写明降级（SKIP，非 PASS）');
  must(/navigation-api-unsupported/.test(s), 'C13', '脚本侧未写明降级原因（reason=navigation-api-unsupported）');
  must(/未证实/.test(s), 'C13', '脚本头未写明「未证实」（版本组合问题留给后续排查，不得宣称已定位根因）');
  must(/首跑/.test(s), 'C13', '脚本头未写明首跑校准要求');
  must(s.includes("$metricValues['navigationSkipReason']"), 'C13', '可选字段 navigationSkipReason 未做成「降级时才写」（不降级就不该出现）');

  // C14 贴评论调用不得断链（缺续行反引号会把参数变成另一条命令）。
  // **只对代码行判**（#914 收窄）：`<# … #>` 帮助文本与 `#` 注释里的 bullet 不是续行参数 ——
  // 旧口径在 device-capture.ps1 上误报 3 条（命中的全是 -Pages / -AllowAppStart 这类 help bullet）。
  const codeLines = stripCommentLines(s);
  codeLines.forEach((line, idx) => {
    if (!/^\s+-[A-Za-z][\w]*\b/.test(line)) return;
    if (idx === 0 || /`\s*$/.test(codeLines[idx - 1])) return;
    violations.push('C14 第 ' + (idx + 1) + ' 行以 `' + line.trim().split(/\s+/)[0] + '` 起行，但上一行没有续行反引号（语句断链，参数不会生效）');
  });
  if (callAt !== -1) {
    const callBlock = s.slice(callAt, s.indexOf('exit $exitCode'));
    must(callBlock.includes('-ArchivedRel') && callBlock.includes('-ArchiveNotes'), 'C14', 'Publish-GateComment 调用未传 -ArchivedRel / -ArchiveNotes（入库截图清单进不了评论）');
  }

  // C15 执行侧：就绪闸门脚本与结果行的取值绑定（顺序/参数/判据由 scanPlan 断言计划）
  must(s.includes("$ReadyRelative = 'scripts\\mp-weixin-ready.mjs'"), 'C15', '未引用就绪闸门 scripts/mp-weixin-ready.mjs');
  must(s.includes('MP_WEIXIN_READY'), 'C15', '未解析就绪闸门的 MP_WEIXIN_READY 结果行');
  must(s.includes('$ReadyWaitSeconds'), 'C15', '缺 -ReadyWaitSeconds 预算参数');
  must(s.includes("'--timeout-ms', \"$ProbeTimeoutMs\""), 'C15', '探针超时未由 -ProbeTimeoutMs 传入（30s 默认值正落在就绪窗口中间）');
  const ptm = s.match(/\$ProbeTimeoutMs = (\d+)/);
  must(ptm && Number(ptm[1]) >= 60000, 'C15', 'ProbeTimeoutMs 默认值不足 60s（实测就绪窗口上界 34s）');
  must(/ready\s*=\s*"\$\{readySeconds\}s"/.test(s), 'C15', '结果行缺 ready= /（会话就绪耗时不机检，下次红绿不定又只能靠猜）');
  must(/sdk\s*=\s*"\$sdkVersion"/.test(s) && /ide\s*=\s*"\$ideVersion"/.test(s), 'C15', '结果行缺 sdk= / ide=');
  must(s.includes('reason=automation-not-ready'), 'C15', '未就绪时没有独立 reason（会退化成笼统的 env 失败）');
  must(ready.includes('SDKVersion'), 'C15', '就绪闸门没有校验 Tool.getInfo 的 SDKVersion');
  must(ready.includes('App.getPageStack'), 'C15', '就绪闸门没有校验 App.getPageStack 开始应答');
  must(probe.includes('mp-weixin-ready.mjs'), 'C15', '探针未指向就绪闸门（归因说明进不了探针侧）');
  must(/24–34s/.test(s) && /24–34s/.test(probe), 'C15', '脚本与探针未记录实测就绪窗口（24–34s）——超时值会变成无来源的魔法数');

  // C16 失败归因：会话未就绪 ≠ 端口连不上（旧版把两者写成同一句，直接导致一轮误诊）
  must(probe.includes('connectError'), 'C16', '探针未输出 connectError（无法区分未就绪与连不上）');
  must(probe.includes('classifyConnectError'), 'C16', '探针未对连接失败做归因分类');
  must(probe.includes("'not-ready'") && probe.includes("'unreachable'"), 'C16', '探针缺 not-ready / unreachable 两个归因口径');
  must(s.includes('connectError.kind') || probe.includes('connectError.kind'), 'C16', '归因口径未写进结果（kind）');
  // 「自动化端口连不上」是**短暂**的（实测 24–34s 才就绪）⇒ 绝不能作为「放弃重试」的依据。
  // 只检查判据**那一行**：上面允许写注释解释这条历史教训。
  // 判据是 `if (...)` 这一行，`break` 在它的**下一行** —— 断言必须按行号取，不能指望同一行里出现 break。
  const sLines = s.split(/\r?\n/);
  const breakIdx = sLines.findIndex((l) => l.includes("'找不到 miniprogram-automator' }"));
  must(breakIdx !== -1, 'C16', '缺「只对缺模块提前放弃重试」的判据行');
  if (breakIdx !== -1) {
    must(!sLines[breakIdx].includes('自动化端口连不上'), 'C16', '放弃重试的判据行仍含「自动化端口连不上」（它正是短暂的那一类 ⇒ 门必红）');
    must(Boolean(sLines[breakIdx + 1]) && sLines[breakIdx + 1].includes('break'), 'C16', '放弃重试的判据之后没有 break（判据不生效）');
  }

  // C17 -Doctor 体检模式：只体检，不产出门结论
  // 必须带尾随逗号一起匹配：`s.includes('[switch]$Doctor')` 会被 `[switch]$DoctorX` 这类改名**蒙过去**
  // （子串仍在），注入用例实测就抓不到——这正是「防空跑假绿」那组用例存在的意义。
  must(/\[switch\]\$Doctor,/.test(s), 'C17', '缺 -Doctor 开关（须为 `[switch]$Doctor,` 参数）');
  must(s.includes('if ($Doctor) { $SkipBuild = $true }'), 'C17', '-Doctor 未强制跳过构建（会去接 HBuilderX 并改工作树）');
  must(s.includes('if ($Doctor) { Write-DoctorReport }'), 'C17', '体检报告未在 finally 里打印（早期 exit 就看不到结论）');
  must(/Add-DoctorRow/.test(s), 'C17', '缺逐层体检记录（Add-DoctorRow）');
  ['L1', 'L2', 'L3', 'L4', 'L5', 'L6', 'L7'].forEach((layer) => {
    must(s.includes("'" + layer + " "), 'C17', '体检缺 ' + layer + ' 层');
  });
  must(s.includes('doctor-islogin'), 'C17', '体检缺登录态检查（登录态过期需人补扫一次码）');
  must(s.includes('不产出 ② 门的通过结论'), 'C17', '体检未声明「不产出门的通过结论」（-Doctor 绿 ≠ ② 通过）');
  must(s.includes('Get-DoctorExitCode'), 'C17', '缺体检独立退出码');
  const doctorExitIdx = s.indexOf('exit (Get-DoctorExitCode)');
  must(doctorExitIdx !== -1, 'C17', '体检模式未在就绪判定后退出');
  if (doctorExitIdx !== -1) {
    const probeUseIdx = s.indexOf("-Id 'automator-probe'");
    const archiveCallIdx = s.indexOf('Publish-ScreenshotArchive -PrNumber');
    must(probeUseIdx !== -1, 'C17', '执行路径里没有按 id 取用探针步骤（取不到就判不出体检是否越界）');
    if (probeUseIdx !== -1) must(doctorExitIdx < probeUseIdx, 'C17', '-Doctor 会继续跑探针（体检不该产出门结论）');
    must(doctorExitIdx < callAt, 'C17', '-Doctor 会贴 PR 评论');
    must(archiveCallIdx === -1 || doctorExitIdx < archiveCallIdx, 'C17', '-Doctor 会做截图入库');
  }
  must(/"build:mp-weixin-doctor"\s*:\s*"[^"]*-Doctor"/.test(pkg), 'C17', 'package.json 未注册 build:mp-weixin-doctor');

  // C18 执行侧接线：门计划是唯一真源 —— 每个步骤都被按 id 取用一次，且取用点确实把计划里的
  // 参数/预算交给了 Invoke-Process；被 pin 的调用点原文**至多**在代码里出现一次（不得在执行路径再写一遍）。
  // 这是「搬运不再碰守护」的关键：判据是「执行读计划」，不是「调用点长什么样」。
  const code = stripCommentLines(s).join('\n');
  const STEP_USE = [
    { id: 'devtools-close', needs: ['.argv', '.timeoutSeconds'] },
    { id: 'devtools-open', needs: ['.argv', '.timeoutSeconds', 'Write-Log'] },
    { id: 'devtools-auto', needs: ['.argv', '.timeoutSeconds', 'Write-Log'] },
    { id: 'port-listening', needs: ['.waitSeconds'] },
    { id: 'ready-gate', needs: ['.argv', '.timeoutSeconds', 'Write-Log'] },
    { id: 'automator-probe', needs: ['.argv', '.timeoutSeconds'] }
  ];
  STEP_USE.forEach(({ id, needs }) => {
    const ref = `-Id '${id}'`;
    const refAt = code.indexOf(ref);
    must(refAt !== -1, 'C18', `计划步骤 ${id} 没有被执行路径按 id 取用（计划与执行脱节）`);
    if (refAt === -1) return;
    const window = code.slice(refAt, refAt + 400);
    needs.forEach((n) => {
      must(window.includes(n), 'C18', `步骤 ${id} 的取用点附近没有把 ${n} 用起来（计划里的参数/预算没被执行路径消费）`);
    });
  });
  [['close', "'close', '--project'"], ['open', "'open', '--project'"], ['auto', "'auto', '--project'"]].forEach(([verb, literal]) => {
    const n = (code.match(new RegExp(literal.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g')) || []).length;
    must(n <= 1, 'C18', `调用点原文 ${literal} 在代码里出现了 ${n} 次 —— 只允许出现在门计划里一次（执行路径不得再写死一份）`);
  });

  // C19 报告层不得毁掉门结论、也不得丢掉证据（#1209 血账）
  //   事故：Publish-ScreenshotArchive 只有成功路径带 `Rel`，调用点在 Set-StrictMode 下读 `$archive.Rel`
  //   ⇒ 「未入库」分支（如"工作树有本次之外的改动"）抛异常 ⇒ 贴 sha 绑定评论那步被打断：
  //   门明明通过却 exit 1，PR 上也没有证据评论（#1199 的 ② 就是这么被手工绕过去的）。
  const archiveBody = psFunctionBody(code, 'function Publish-ScreenshotArchive');
  must(archiveBody.length > 0, 'C19', '定位不到 Publish-ScreenshotArchive 函数体（无法核对返回形状）');
  if (archiveBody.length > 0) {
    const returns = archiveBody.match(/return @\{[^}]*\}/g) || [];
    must(returns.length >= 2, 'C19', `Publish-ScreenshotArchive 的 return 分支只有 ${returns.length} 个（判据可能已空跑）`);
    returns.forEach((r, i) => {
      must(/Rel\s*=/.test(r), 'C19',
        `Publish-ScreenshotArchive 第 ${i + 1} 个 return 缺 Rel ⇒ 未入库分支返回不同形状（Set-StrictMode 下调用点读取即抛异常，曾把「门通过」变成 exit 1 且贴不出证据评论）`);
    });
  }
  must(!/\$archive\.Rel\b/.test(code), 'C19', '调用点直接读 $archive.Rel（须走 Get-Prop，否则未入库分支抛异常）');
  must(/Get-Prop \$archive 'Rel'/.test(code), 'C19', '调用点未用 Get-Prop 安全取 Rel');
  must(/贴 ② 门评论失败（门结论不受影响/.test(code), 'C19', '贴评论失败时缺「不改结论」的响亮警告（报告层崩溃会静默吞掉证据）');
  const commentCallIdx = code.indexOf('Publish-GateComment -PrNumber $PostToPr');
  must(commentCallIdx !== -1, 'C19', '定位不到贴评论调用点（报告层是否受保护无法核对）');
  if (commentCallIdx !== -1) {
    const preWindow = code.slice(Math.max(0, commentCallIdx - 200), commentCallIdx);
    must(/try\s*\{/.test(preWindow), 'C19', '贴评论调用没被 try 包住（报告层一崩就会把「门通过」变成 exit 1）');
    must(/catch\s*\{/.test(code.slice(commentCallIdx, commentCallIdx + 1200)), 'C19', '贴评论调用缺 catch（崩了必须警告 + 记日志，不得静默）');
  }

  // C10 注册与文档
  must(/"build:mp-weixin-check"\s*:\s*"[^"]*scripts\/mp-weixin-check\.ps1"/.test(pkg), 'C10', 'package.json 未注册 build:mp-weixin-check');
  must(adr.includes('半自动'), 'C10', 'ADR-0008 未把 ② 记为半自动门');
  must(adr.includes('#883'), 'C10', 'ADR-0008 未引用 #883');
  must(adr.includes('mp-weixin-check.ps1'), 'C10', 'ADR-0008 未记录 ② 的脚本载体');
  must(template.includes('gate-evidence:②'), 'C10', 'PR 模板未说明 ② 可用 -PostToPr 评论承载');
  must(agents.includes('人工门只剩') || agents.includes('② 微信开发者工具门 = 半自动门'), 'C10', '移动端 AGENTS.md 未把 ② 移出人工门清单并写明运行前提');

  return violations;
}

describe('② 微信开发者工具门契约（#883 / 2026-09-12 半自动 / 2026-09-13 守护改断言门计划）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    probe: readSource(PROBE_REL),
    ready: readSource(READY_REL),
    pkg: readSource(PKG_REL),
    adr: readSource(ADR_REL),
    agents: readSource(AGENTS_REL),
    template: readSource(PR_TEMPLATE_REL)
  };

  // -DryRun 的实跑：项目目录用一个**空临时目录**（脚本在 -DryRun 下不该要求 manifest.json、不该写工作树）
  let tmpProject = '';
  let dryRunStdout = '';
  let realPlan = null;

  beforeAll(() => {
    tmpProject = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-dryrun-'));
    dryRunStdout = runDryRun(path.join(ROOT, SCRIPT_REL), tmpProject);
    realPlan = parsePlan(dryRunStdout);
  }, 180000);

  afterAll(() => {
    if (tmpProject) fs.rmSync(tmpProject, { recursive: true, force: true });
  });

  it('真实文件：-DryRun 输出可解析，门计划零违规', () => {
    expect(scanPlan(realPlan)).toEqual([]);
  });

  it('-DryRun：单行 GATE_PLAN、不写工作树、不执行任何外部命令', () => {
    // 「单行」是接口约定：守护按行取 JSON，多行/夹带其他输出会让解析面变脆
    const nonEmpty = dryRunStdout.split(/\r?\n/).filter((l) => l.trim() !== '');
    expect(nonEmpty).toHaveLength(1);
    expect(nonEmpty[0].startsWith(PLAN_PREFIX)).toBe(true);
    // 不写工作树：.ci-verify（日志目录）都不该被创建 —— 空临时项目目录跑完必须还是空的
    expect(fs.readdirSync(tmpProject)).toEqual([]);
    // 不执行外部命令：Invoke-Process 每次都会打 `>>> [tag] file args`，出现即说明跑了进程
    expect(dryRunStdout).not.toMatch(/^>>> \[/m);
  });

  it('C18：计划覆盖四类内容（步骤序列 / 每步参数与判据 / 预算与 exit 2 归属 / 结果行字段）', () => {
    expect(realPlan.steps.map((s) => s.id)).toEqual(PLAN_SEQUENCE);
    realPlan.steps.forEach((s) => {
      expect(Array.isArray(s.criteria) && s.criteria.length).toBeTruthy();
      expect(Array.isArray(s.exit2)).toBe(true);
    });
    // 每步的参数与判据：给出的是**值**，不是「有个变量」
    expect(realPlan.steps.find((s) => s.id === 'devtools-auto').argv).toEqual(
      ['auto', '--project', realPlan.paths.dist, '--auto-port', String(realPlan.endpoint.port), '--trust-project']
    );
    expect(realPlan.steps.find((s) => s.id === 'ready-gate').criteria.join(' ')).toContain('SDKVersion');
    expect(realPlan.steps.find((s) => s.id === 'port-listening').criteria.join(' ')).toContain('listening');
    // 预算与 exit 2 归属：谁决定 exit 2 要能机检
    expect(realPlan.timeouts.portWait).toBe(realPlan.steps.find((s) => s.id === 'port-listening').waitSeconds);
    expect(realPlan.exit2.map((e) => e.step + ':' + e.reason)).toEqual([
      'devtools-open:devtools-open-timeout',
      'devtools-auto:auto-timeout',
      'port-listening:port-not-listening',
      'ready-gate:no-ready-result',
      'ready-gate:automation-not-ready',
      'automator-probe:probe-timeout'
    ]);
    // 结果行字段清单
    expect(realPlan.resultLine.prefix).toBe('MP_WEIXIN_RESULT');
    expect(realPlan.resultLine.fields.filter((f) => !f.optional).map((f) => f.name)).toEqual(RESULT_MANDATORY);
    expect(realPlan.resultLine.fields.filter((f) => f.optional).map((f) => f.name)).toEqual(RESULT_OPTIONAL);
  });

  it('C2 / C12：三步顺序与参数（close → open --project <dist> → auto）在计划里写死', () => {
    const argv = (id) => realPlan.steps.find((s) => s.id === id).argv;
    expect(argv('devtools-close')).toEqual(['close', '--project', realPlan.paths.dist]);
    expect(argv('devtools-open')).toEqual(['open', '--project', realPlan.paths.dist]);
    expect(realPlan.steps.map((s) => s.id).indexOf('devtools-close'))
      .toBeLessThan(realPlan.steps.map((s) => s.id).indexOf('devtools-open'));
    expect(realPlan.steps.map((s) => s.id).indexOf('devtools-open'))
      .toBeLessThan(realPlan.steps.map((s) => s.id).indexOf('devtools-auto'));
    // auto 必须跑完（不得被 kill）：计划里显式声明，且有界（超时）
    const auto = realPlan.steps.find((s) => s.id === 'devtools-auto');
    expect(auto.awaitCompletion).toBe(true);
    expect(auto.timeoutSeconds).toBeGreaterThan(0);
  });

  it('C15：auto 之后、探针之前是就绪闸门，端点/预算/判据都取自计划', () => {
    const ready = realPlan.steps.find((s) => s.id === 'ready-gate');
    expect(ready.argv[0]).toBe(realPlan.paths.ready);
    expect(ready.argv).toContain('--require-stack');
    expect(ready.argv[ready.argv.indexOf('--wait-seconds') + 1]).toBe(String(realPlan.timeouts.readyWait));
    const probe = realPlan.steps.find((s) => s.id === 'automator-probe');
    expect(probe.argv[probe.argv.indexOf('--timeout-ms') + 1]).toBe(String(realPlan.timeouts.probeTimeoutMs));
    expect(realPlan.timeouts.probeTimeoutMs).toBeGreaterThanOrEqual(60000);
  });

  it('自检：注入**变形后的计划**必须被检出（防空跑假绿）', () => {
    const clone = () => JSON.parse(JSON.stringify(realPlan));
    const editStep = (plan, id, mutate) => {
      const s = plan.steps.find((x) => x.id === id);
      mutate(s);
      return plan;
    };
    const swap = (plan, a, b) => {
      const i = plan.steps.findIndex((x) => x.id === a);
      const j = plan.steps.findIndex((x) => x.id === b);
      const t = plan.steps[i];
      plan.steps[i] = plan.steps[j];
      plan.steps[j] = t;
      return plan;
    };
    const cases = [
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { s.argv[0] = 'autoX'; })],
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { s.argv.splice(s.argv.indexOf('--auto-port'), 2); })],
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { s.argv[s.argv.indexOf('--auto-port') + 1] = '1'; })],
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { s.awaitCompletion = false; })],
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { delete s.timeoutSeconds; })],
      ['C2', (p) => editStep(p, 'devtools-auto', (s) => { s.exit2 = []; })],
      ['C2', (p) => swap(p, 'devtools-close', 'devtools-auto')],
      ['C12', (p) => editStep(p, 'devtools-open', (s) => { s.argv[0] = 'openX'; })],
      ['C12', (p) => editStep(p, 'devtools-open', (s) => { s.argv[2] = 'D:/somewhere/else'; })],
      ['C12', (p) => editStep(p, 'devtools-open', (s) => { s.logged = false; })],
      ['C12', (p) => editStep(p, 'devtools-open', (s) => { s.exit2 = []; })],
      ['C12', (p) => swap(p, 'devtools-open', 'devtools-auto')],
      ['C12', (p) => { p.steps = p.steps.filter((s) => s.id !== 'devtools-open'); return p; }],
      ['C18', (p) => { p.paths.distRelative = 'unpackage/dist/build/other'; return p; }],
      ['C15', (p) => swap(p, 'devtools-auto', 'ready-gate')],
      ['C15', (p) => swap(p, 'ready-gate', 'automator-probe')],
      ['C15', (p) => editStep(p, 'ready-gate', (s) => { s.argv = s.argv.filter((a) => a !== '--require-stack'); })],
      ['C15', (p) => editStep(p, 'ready-gate', (s) => { s.criteria = ['等端口']; })],
      ['C15', (p) => editStep(p, 'ready-gate', (s) => { s.exit2 = []; })],
      ['C15', (p) => editStep(p, 'ready-gate', (s) => { s.argv[s.argv.indexOf('--wait-seconds') + 1] = '5'; })],
      ['C15', (p) => editStep(p, 'automator-probe', (s) => { s.argv[s.argv.indexOf('--timeout-ms') + 1] = '30000'; })],
      ['C15', (p) => { p.timeouts.probeTimeoutMs = 30000; return p; }],
      ['C13', (p) => { p.resultLine.fields = p.resultLine.fields.filter((f) => f.name !== 'navigation'); return p; }],
      ['C13', (p) => { p.resultLine.prefix = 'RESULT_X'; return p; }],
      ['C13', (p) => { p.resultLine.fields.find((f) => f.name === 'log').optional = true; return p; }],
      ['C13', (p) => { p.resultLine.fields.find((f) => f.name === 'navigationSkipReason').optional = false; return p; }],
      ['C13', (p) => { p.resultLine.fields.push({ name: 'extra' }); return p; }],
      ['C18', (p) => editStep(p, 'devtools-auto', (s) => { s.criteria = []; })],
      ['C18', (p) => editStep(p, 'port-listening', (s) => { delete s.tag; })],
      ['C18', (p) => editStep(p, 'devtools-auto', (s) => { s.timeoutSeconds = 1; })],
      ['C18', (p) => { p.exit2 = p.exit2.slice(1); return p; }],
      ['C18', (p) => { p.exit2[0].reason = ''; return p; }],
      ['C18', (p) => swap(p, 'devtools-close', 'devtools-open')],
      ['C18', (p) => editStep(p, 'devtools-close', (s) => { s.exit2 = [{ on: 'timeout', reason: 'x' }]; })],
      ['C18', (p) => editStep(p, 'devtools-close', (s) => { delete s.exit2; })],
      ['C18', (p) => { const log = p.resultLine.fields.find((f) => f.name === 'log'); p.resultLine.fields = [log].concat(p.resultLine.fields.filter((f) => f.name !== 'log')); return p; }],
      ['C18', (p) => { p.schema = 'gate-plan/2'; return p; }],
      ['C18', (p) => { p.endpoint.ws = 'ws://127.0.0.1:1'; return p; }],
      ['C18', (p) => { p.paths.probeRelative = 'scripts/other.mjs'; return p; }]
    ];
    cases.forEach(([rule, mutate], caseIndex) => {
      const found = scanPlan(mutate(clone()));
      expect({ caseIndex, rule, detected: found.some((v) => v.startsWith(rule)) })
        .toEqual({ caseIndex, rule, detected: true });
    });
  });

  it('自检：把脚本源码里的计划改坏，真跑 -DryRun 后必须被检出（证明判据挂在脚本上，不是硬编码的 JSON）', () => {
    const mutated = real.script.replace("argv = @('auto', '--project', $Dist", "argv = @('autoX', '--project', $Dist");
    expect(mutated).not.toBe(real.script); // 锚点必须在（否则这条自检会静默变成空跑）
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-dryrun-mut-'));
    const copy = path.join(dir, 'mp-weixin-check.ps1');
    try {
      fs.writeFileSync(copy, mutated, 'utf8');
      const plan = parsePlan(runDryRun(copy, tmpProject));
      const found = scanPlan(plan);
      expect(found.some((v) => v.startsWith('C2'))).toBe(true);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }, 180000);

  it('自检：注入违规必须被检出（文本面，防空跑假绿）', () => {
    const cases = [
      ['C1', { ...real, script: real.script.replace("$DistRelative = 'unpackage\\dist\\build\\mp-weixin'", "$DistRelative = 'x'") }],
      ['C1', { ...real, script: real.script.replace(/scripts\\mp-weixin-probe\.mjs/g, 'scripts\\other.mjs') }],
      ['C2', { ...real, script: real.script.replace(/\$auto\.TimedOut/g, '$autoX.TimedOut') }],
      ['C3', { ...real, probe: real.probe.replace(/pageStackNonEmpty/g, 'stackOk') }],
      ['C3', { ...real, probe: real.probe + '\nconst el = await page.$("view");\n' }],
      ['C4', { ...real, script: real.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' }],
      ['C4', { ...real, script: real.script.replace(/MP_WEIXIN_PROBE/g, 'PROBE_X') }],
      ['C4', { ...real, script: real.script.replace(/MP_WEIXIN_RESULT/g, 'RESULT_X') }],
      ['C5', { ...real, script: real.script.replace(/通配/g, '本机') }],
      ['C5', { ...real, script: real.script.replace(/监听面/g, 'x') }],
      ['C6', { ...real, script: real.script.replace(/gate-evidence:②/g, 'gate-evidence:x') }],
      ['C6', { ...real, script: real.script.replace(/- commit: \$sha/g, '- sha: $sha') }],
      ['C7', { ...real, script: real.script.replace(/\$script:closed/g, '$closedX') }],
      ['C8', { ...real, script: real.script.replace(/非等价/g, '等价性待定') }],
      ['C8', { ...real, script: real.script.replace(/manifest-appid/g, 'manifest-x') }],
      ['C9', { ...real, script: real.script.replace('$script:ArchiveMaxBytes = 150KB', '$script:ArchiveMaxBytes = 900KB') }],
      ['C9', { ...real, script: real.script.replace(/-NoArchive/g, '-ArchiveAll') }],
      ['C10', { ...real, pkg: real.pkg.replace('build:mp-weixin-check', 'build:mp-x') }],
      ['C10', { ...real, adr: real.adr.replace(/半自动/g, '全自动') }],
      ['C10', { ...real, template: real.template.replace(/gate-evidence:②/g, 'gate-evidence:x') }],
      ['C10', { ...real, agents: '人工门清单（无 ② 条目）' }],
      ['C11', { ...real, script: real.script.replace(/hx-busy\.ps1/g, 'other.ps1') }],
      ['C11', { ...real, script: real.script.replace(/Release-HxLock/g, 'ReleaseNothing') }],
      ['C11', { ...real, script: real.script.replace('Release-HxLock', 'Stop-Process -Name HBuilderX -Force') }],
      ['C12', { ...real, script: real.script.replace("-Id 'devtools-open'", "-Id 'devtools-openX'") }],
      ['C12', { ...real, script: real.script.replace(/\$open\.TimedOut/g, '$openX.TimedOut') }],
      ['C12', { ...real, script: real.script.replace("errors=env reason=no-dist", "errors=env reason=nodist") }],
      // C13：把降级写成 PASS / 让 SKIP 静默变成通过 / 把结果行的 navigation 绑错
      ['C13', { ...real, script: real.script.replace('navigation=SKIP reason=', 'navigation=PASS reason=') }],
      ['C13', { ...real, script: real.script.replace('navigation = "$navField"', 'navigation = "ok"') }],
      ['C13', { ...real, script: real.script.replace('降级（SKIP，非 PASS）', '结果说明') }],
      ['C13', { ...real, probe: real.probe.replace(/navigation-api-unsupported/g, 'x') }],
      ['C13', { ...real, probe: real.probe.replace(/'skip'/g, 'true') }],
      ['C13', { ...real, probe: real.probe.replace('if (v === false)', 'if (v === true)') }],
      // C14：续行反引号被删 ⇒ `-ArchivedRel …` 变成另一条命令（曾真实发生过）
      ['C14', { ...real, script: real.script.replace(/-ReproCommand 'npm run build:mp-weixin-check' `/g, "-ReproCommand 'npm run build:mp-weixin-check'") }],
      // C15：拆掉就绪闸门 / 把探针超时降回 30s / 结果行不再机检就绪耗时
      // （`--require-stack` 这类**参数**已搬到计划面，注入用例见上面那组 scanPlan 自检）
      ['C15', { ...real, script: real.script.replace("$ReadyRelative = 'scripts\\mp-weixin-ready.mjs'", "$ReadyRelative = 'scripts\\x.mjs'") }],
      ['C15', { ...real, script: real.script.replace(/\$ProbeTimeoutMs = 60000/, '$ProbeTimeoutMs = 30000') }],
      ['C15', { ...real, script: real.script.replace('ready = "${readySeconds}s"', 'ready = "${readySeconds}ms"') }],
      ['C15', { ...real, script: real.script.replace('reason=automation-not-ready', 'reason=env') }],
      ['C15', { ...real, ready: real.ready.replace(/SDKVersion/g, 'x') }],
      ['C15', { ...real, probe: real.probe.replace(/mp-weixin-ready\.mjs/g, 'x.mjs') }],
      // C16：把两类失败合并回去 / 取消归因分类
      ['C16', { ...real, probe: real.probe.replace(/classifyConnectError/g, 'classifyX') }],
      ['C16', { ...real, probe: real.probe.replace(/'not-ready'/g, "'x'") }],
      ['C16', { ...real, script: real.script.replace("'找不到 miniprogram-automator' }", "'找不到 miniprogram-automator|自动化端口连不上' }") }],
      // C17：体检模式失效（不跳构建 / 报告不打 / 声称产出门结论 / 注册丢失）
      ['C17', { ...real, script: real.script.replace('[switch]$Doctor', '[switch]$DoctorX') }],
      ['C17', { ...real, script: real.script.replace('if ($Doctor) { $SkipBuild = $true }', 'if ($Doctor) { }') }],
      ['C17', { ...real, script: real.script.replace('if ($Doctor) { Write-DoctorReport }', '') }],
      ['C17', { ...real, script: real.script.replace('不产出 ② 门的通过结论', '体检结论') }],
      ['C17', { ...real, pkg: real.pkg.replace('build:mp-weixin-doctor', 'build:mp-x') }],
      // C18：执行路径与计划脱节（不再按 id 取用 / 又把调用点原文写死一份）
      ['C18', { ...real, script: real.script.replace("-Id 'ready-gate'", "-Id 'ready-gateX'") }],
      ['C18', { ...real, script: real.script.replace("-Id 'automator-probe'", "-Id 'automator-probeX'") }],
      ['C18', { ...real, script: real.script + "\nInvoke-Process -FilePath $devTools -Arguments @('close', '--project', $dist) -TimeoutSeconds 120 -Tag 'x'\n" }],
      ['C18', { ...real, script: real.script.replace('-Arguments $readyStep.argv', '-Arguments $readyArgs') }],
      ['C18', { ...real, script: real.script.replace('$portStep.waitSeconds', '180') }],
      // C19：报告层（归档形状统一 / 安全取值 / try-catch / 响亮警告）—— #1209 血账
      ['C19', { ...real, script: real.script.replace("未入库：找不到 gh CLI'); Commands = @(); Rel = @() }", "未入库：找不到 gh CLI'); Commands = @() }") }],
      ['C19', { ...real, script: real.script + "\n$relProbe = $archive.Rel\n" }],
      ['C19', { ...real, script: real.script.replace("-ArchivedRel @(Get-Prop $archive 'Rel')", "-ArchivedRel @($archive.Rel)") }],
      ['C19', { ...real, script: real.script.replace("try {\n        Publish-GateComment -PrNumber $PostToPr", "Publish-GateComment -PrNumber $PostToPr") }],
      ['C19', { ...real, script: real.script.replace("贴 ② 门评论失败（门结论不受影响", "贴评论失败") }]
    ];
    cases.forEach(([rule, sources], caseIndex) => {
      const found = scanContract(sources);
      // 断言里带上序号与规则名：注入用例一旦失效，失败信息要能直接指出是**哪一条**（否则只能二分）
      expect({ caseIndex, rule, detected: found.some((v) => v.startsWith(rule)) })
        .toEqual({ caseIndex, rule, detected: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('C4：门结论只来自探针 JSON 输出，不来自任何退出码', () => {
    expect(real.script).toContain('MP_WEIXIN_PROBE');
    expect(real.script).not.toMatch(/\$LASTEXITCODE/);
    expect(real.probe).toContain('probeOk');
  });

  it('C19：报告层不改门结论、不丢证据（归档形状统一 / Get-Prop 取值 / try-catch / 响亮警告）', () => {
    const code = stripCommentLines(real.script).join('\n');
    expect(code).toMatch(/Get-Prop \$archive 'Rel'/);
    expect(code).not.toMatch(/\$archive\.Rel/);
    expect(code).toContain('贴 ② 门评论失败（门结论不受影响');
    const callIdx = code.indexOf('Publish-GateComment -PrNumber $PostToPr');
    expect(callIdx).toBeGreaterThan(-1);
    expect(code.slice(Math.max(0, callIdx - 200), callIdx)).toMatch(/try\s*\{/);
    const body = psFunctionBody(code, 'function Publish-ScreenshotArchive');
    const returns = body.match(/return @\{[^}]*\}/g) || [];
    expect(returns.length).toBeGreaterThanOrEqual(2);
    returns.forEach((r) => expect(r).toMatch(/Rel\s*=/));
  });

  it('C6：仅门通过（exit 0）分支贴 sha 绑定评论', () => {
    const callAt = real.script.indexOf('Publish-GateComment -PrNumber');
    expect(callAt).toBeGreaterThan(-1);
    expect(real.script.indexOf('exit 1')).toBeLessThan(callAt);
    expect(real.script.indexOf('exit $exitCode')).toBeGreaterThan(callAt);
  });

  it('C9：入库上限写死在脚本里（10 张 / 150KB / 1.5MB / 720px）', () => {
    expect(real.script).toContain('$script:ArchiveMaxCount = 10');
    expect(real.script).toContain('$script:ArchiveMaxBytes = 150KB');
    expect(real.script).toContain('$script:ArchiveMaxTotal = 1.5MB');
    expect(real.script).toContain('$script:ArchiveMaxWidth = 720');
  });

  it('C13：导航不可用时降级为 SKIP（不是 PASS），且写进结果行 / 日志 / 门评论', () => {
    expect(real.probe).toContain('navigation-api-unsupported');
    expect(real.probe).toContain('NAV_SKIP_ASSERTIONS');
    expect(real.probe).toContain("'skip'");
    expect(real.probe).toContain('if (v === false)');
    expect(real.script).toContain('navigation = "$navField"');
    expect(real.script).toContain('navigation=SKIP reason=');
    expect(real.script).toContain('navigation-api-unsupported');
    expect(real.script).toContain('降级（SKIP，非 PASS）');
    expect(real.script).not.toMatch(/navigation=PASS/);
    // 可验证子集写在脚本头里（门只对它作结论）
    expect(real.script).toContain('可验证子集');
    expect(real.script).toContain('currentPageScreenshot');
    expect(real.probe).toContain('currentPageScreenshot');
  });

  it('C13：SKIP 不产生 failures，failures 只由可验证子集（明确 false）产生', () => {
    // 静态守护：探针只把 `v === false` 收进 failures，且降级断言的值是字符串 'skip'
    const probe = real.probe;
    const finishBody = probe.slice(probe.indexOf('function finish('), probe.indexOf('const loaded = loadAutomator()'));
    expect(finishBody).toMatch(/if \(v === false\) result\.failures\.push/);
    expect(finishBody).toMatch(/:\s*'skip'/); // 降级时断言的值是字符串 'skip'（既不是 true 也不是 false）
    expect(finishBody).not.toMatch(/v\)\s*\{\s*result\.failures/); // 旧的 `if (!v)` 形态（会把 SKIP 算成失败）
  });

  it('C14：贴评论调用是一条语句（续行反引号不缺，-ArchivedRel/-ArchiveNotes 在语句内）—— 只对代码行判', () => {
    const lines = stripCommentLines(real.script);
    lines.forEach((line, idx) => {
      if (!/^\s+-[A-Za-z][\w]*\b/.test(line)) return;
      if (idx === 0) return;
      expect(lines[idx - 1]).toMatch(/`\s*$/); // 上一行必须以续行反引号结尾
    });
    const callAt = real.script.indexOf('Publish-GateComment -PrNumber');
    const callBlock = real.script.slice(callAt, real.script.indexOf('exit $exitCode'));
    expect(callBlock).toContain('-ArchivedRel');
    expect(callBlock).toContain('-ArchiveNotes');
  });

  it('C14：帮助文本里的 bullet（device-capture.ps1 那类）不再误报，真断链仍被抓', () => {
    // ① 收窄后的规则对真实文件零命中：device-capture.ps1 的 help bullet 曾让旧口径误报 3 条
    const deviceCapture = readSource(DEVICE_CAPTURE_REL);
    const deviceViolations = scanContract({ ...real, script: deviceCapture }).filter((v) => v.startsWith('C14'));
    expect(deviceViolations).toEqual([]);
    // 旧口径（不剥注释）在同一文件上确实会报 —— 这是这条收窄的现场证据，不是空谈
    const rawHits = [];
    deviceCapture.split(/\r?\n/).forEach((line, idx) => {
      if (!/^\s+-[A-Za-z][\w]*\b/.test(line)) return;
      if (idx === 0 || /`\s*$/.test(deviceCapture.split(/\r?\n/)[idx - 1])) return;
      rawHits.push(idx + 1);
    });
    expect(rawHits.length).toBeGreaterThan(0);
    // ② 合成样本：help bullet（-Pages / -AllowAppStart）与注释 bullet 不报，代码里的真断链要报
    const syntheticLines = [
      '<#',
      '.PARAMETER Pages',
      '    用法：',
      "      -Pages 'a,b'        只截这些页",
      '      -AllowAppStart        允许拉起 App',
      '#>',
      'param([string]$Pages)',
      '# 下面第一条是好的续行，第二条断了（少一个行尾反引号）',
      'Invoke-Process -FilePath $cli -Arguments @("x") `',
      '    -TimeoutSeconds 120',
      'Invoke-Process -FilePath $cli -Arguments @("y")',
      '    -TimeoutSeconds 120'
    ];
    const brokenLine = syntheticLines.findIndex((l, i) => l.trim().startsWith('-TimeoutSeconds') && !syntheticLines[i - 1].endsWith('`'));
    const syntheticViolations = scanContract({ ...real, script: syntheticLines.join('\n') }).filter((v) => v.startsWith('C14'));
    expect(syntheticViolations).toHaveLength(1);
    expect(syntheticViolations[0]).toContain(`第 ${brokenLine + 1} 行`);
  });

  it('C16：连接失败必须归因；「自动化端口连不上」不得再被当作「重试无益」', () => {
    const lines = real.script.split(/\r?\n/);
    const breakIdx = lines.findIndex((l) => l.includes("'找不到 miniprogram-automator' }"));
    expect(breakIdx).toBeGreaterThan(-1);
    expect(lines[breakIdx]).not.toContain('自动化端口连不上');
    expect(lines[breakIdx + 1]).toContain('break');
    expect(real.probe).toContain('connectError');
    expect(real.probe).toContain('classifyConnectError');
    expect(real.probe).toContain("'not-ready'");
    expect(real.probe).toContain("'unreachable'");
  });

  it('C17：-Doctor 只体检（不构建 / 不跑探针 / 不贴评论 / 不入库），报告在 finally 打印', () => {
    const s = real.script;
    const doctorExitIdx = s.indexOf('exit (Get-DoctorExitCode)');
    expect(s).toContain('[switch]$Doctor');
    expect(s).toContain('if ($Doctor) { $SkipBuild = $true }');
    expect(s).toContain('if ($Doctor) { Write-DoctorReport }');
    expect(doctorExitIdx).toBeGreaterThan(-1);
    expect(doctorExitIdx).toBeLessThan(s.indexOf("-Id 'automator-probe'"));
    expect(doctorExitIdx).toBeLessThan(s.indexOf('Publish-GateComment -PrNumber'));
    expect(doctorExitIdx).toBeLessThan(s.indexOf('Publish-ScreenshotArchive -PrNumber'));
    expect(s).toContain('不产出 ② 门的通过结论');
    expect(s).toContain('Get-DoctorExitCode');
  });
});
