/**
 * ② 微信开发者工具门（scripts/mp-weixin-check.ps1 + scripts/mp-weixin-probe.mjs）契约守护
 *
 * 背景：#883 spike 证实 ② 可完全无人值守执行（HBuilderX `publish mp-weixin` 构建并自己拉起开发者工具
 * → `cli.bat auto` 开自动化端口 → `miniprogram-automator` 连上读 pageStack / console / exceptions 并截图）。
 * 2026-09-12 修订把 ② 从**人工门**降为**半自动门**（与 ④a 同款口径，#870 裁定 B）。它的价值全押在
 * 下面几条不变量上，本测试就是守这几条：
 *   C1 输入 = HBuilderX CLI 构建产物（unpackage\dist\build\mp-weixin），不另造构建；探针在同目录 scripts/
 *   C2 顺序写死 `cli.bat close` → `cli.bat auto`；**auto 必须等它跑完**（中途 kill ⇒ 端口永不监听，2026-09-12 实测）
 *   C3 「pageStack 非空」必须是**前置断言**；禁令已收窄（#1917 裁决 + #1919 落锁）：page.waitFor(选择器) 全文件禁、
 *       导航 API 在锚点段内禁；page.$ + element.text() 锚点断言放行（正向自检锁「不再误伤」）
 *   C4 判成败只看输出：不得出现 $LASTEXITCODE；必须解析 MP_WEIXIN_PROBE / MP_WEIXIN_RESULT 与 publish 文案
 *   C5 端口监听面风险必须写明（实测绑 `::` 通配地址，CLI 无法收窄），且必须轮询端口延迟出现
 *   C6 结果评论格式 == `gate-evidence:②` + `commit: <sha>`；只在门通过（exit 0）分支贴
 *      ⚠️ A1（#2100）后**贴评论已退役**（Send-GateComment 降为「只打印可粘贴块」），本项的
 *      marker / `-PostToPr` 断言保护的是调用点形状而非投递行为 —— 仍能挡住「调用点被静默摘掉」，
 *      但已不能证明「有评论被贴出」。待随投递面清理一并对齐（见 PR #2131 遗留段）。
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
 *   C24 **任意页直达探针**（#1682 · ADR-0039 拆分步骤 2）：门计划必须声明 directPages 节（机制 =
 *       cold-start-direct，构建产物 app.json 首项临时换页 + plain 冷启动、finally 还原、SKIP reason 值域、
 *       探针 argv 模板）；执行路径必须消费 `<page>` 占位与 auto 页清单判据，且任何代码行不得回写源头
 *       pages.json（运行时面判据来源，ADR-0038 决策 2 红线）。核验补记（2026-10-13）后另钉两件事：
 *       **入库数据流**（直达截图 → 聚合 shots → probeJson.shots → 归档，AC1 的链条，拆了门照样绿）与
 *       **评论渲染的 null 过滤**（-NoArchive 时 @(Get-Prop $null 'Rel') 不得渲染成空条目假清单）。
 *   C25 **登录态会话注入**（#1690）：门计划必须有 auth-inject 步（ready-gate 之后、探针之前，exec=node，
 *   直达循环两相（run12/run13）：入口页 = guest 相（--mode guest 游客盖帽——auth_user 写非 JSON + token 清空，
 *       removeStorageSync 实测不跨重启落盘、setStorageSync 落盘，run13 实证不对称；#1478 welcome 有会话必弹 dashboard）、
 *   其余页 = auth 相（--mode auth 注入，当页冷启动**之前**执行）；
 *       exit 2 恒空——注入失败是诚实 SKIP（auth-session-unavailable），不是环境 blocked）；执行路径唯一
 *       取用点 `Get-GateStep -Id 'auth-inject'` + `Invoke-AuthInject` 解析 `MP_WEIXIN_AUTH` 结果行；
 *       注入脚本 scripts/mp-weixin-auth.mjs 走 callWxMethod('login') → POST /auth/wx-login →
 *       setStorageSync 四键（与 setAuthData 对齐），令牌绝不入结果行、脚本不得 mp.close（会话留给探针）。
 *
 *   C26 **publish 前秒级环境预探**（#1860）：门计划第一步是 env-preprobe（exec=none，**只读**不操控 IDE），
 *       判据库 scripts/lib/mp-weixin-preprobe.ps1；楔死态签名 = 「.ide 端口文件在但其服务端口试连不上」，
 *       是**否定性闸门**（只拦楔死，不肯定性放行）；exit 2 归属 = ide-port-unreachable（envReasons 在列）。
 *       #2147 补进程维度：进程数 0 的陈旧端口文件（stale-port-file）走放行，不算楔死。
 *   C27 **产物内容指纹自动 -SkipBuild**（#1860）：判据库 scripts/lib/mp-weixin-build-fingerprint.ps1，
 *       指纹键 = 输入集内容哈希 + 产物内容哈希 + HBuilderX 版本令牌（三组件全等才命中）；命中翻
 *       $SkipBuild 并重建门计划；落盘在产物合法/appid 断言都过之后；失配方向 fail 向重建。
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
const AUTH_REL = 'scripts/mp-weixin-auth.mjs';
const PKG_REL = 'package.json';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
const AGENTS_REL = 'AGENTS.md';
const PR_TEMPLATE_REL = path.join('..', '..', '.github', 'PULL_REQUEST_TEMPLATE.md');

// 门计划的步骤序列（② 的全部外部动作，按执行顺序；-DryRun 输出的 steps[].id 必须逐字等于它）
const PLAN_SEQUENCE = [
  'env-preprobe',
  'devtools-quit',
  'devtools-close',
  'devtools-open',
  'devtools-auto',
  'port-listening',
  'ready-gate',
  'auth-inject',
  'automator-probe'
];
// 结果行必填字段（顺序即输出顺序；可选字段单列，见 C13）
const RESULT_MANDATORY = [
  'appid', 'pageStack', 'entry', 'errorsTotal', 'exceptionsTotal', 'logsTotal',
  'shots', 'navigation', 'ready', 'sdk', 'ide', 'log'
];
// 可选字段（只在对应情形出现时才写；顺序 = 出现时的相对顺序）
// noise：平台层噪音白名单放行的 console error 条数（2026-10-29 F3）。**只在 noise>0 时出现** ——
// 过滤必须可见：某次跑次真被过滤过东西，读数行上一定看得到；没噪音的跑次结果行逐字节不变。
const RESULT_OPTIONAL = ['coverage', 'navigationSkipReason', 'auth', 'asserts', 'noise'];

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
  // quit 同样是「失败只警告」的步骤（pre-flight 清场不判环境不可用）
  const quitStep = step('devtools-quit');
  must(Boolean(quitStep) && (quitStep.exit2 || []).length === 0, 'C18', 'quit 是「失败只警告」的 pre-flight 步骤，不该声明 exit 2');
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

  // ---- C21：整段重试（2026-09-23 坑位 6）—— auto/端口起不来时**整段** close → open → auto 再来一遍 ----
  // 为什么是「整段」而不是「只重试 auto」：实测只重跑 auto 会绑到一个**没加载项目的新 IDE 实例**上 ——
  // 端口通了但 Tool.getInfo 缺 SDKVersion、App.getPageStack 永不应答（结果行 reason=sdk-version-missing）。
  const portStep = step('port-listening');
  if (autoStep) {
    must(Number.isInteger(autoStep.attempts) && autoStep.attempts >= 2, 'C21',
      `auto 未声明整段重试次数（attempts 须为 ≥2 的整数，实际 ${JSON.stringify(autoStep.attempts)}）—— 冷起点第一枪实测必空，单发会让门红在环境上`);
    must(autoStep.attempts === T.autoAttempts, 'C21',
      `auto 的 attempts（${autoStep.attempts}）与预算 timeouts.autoAttempts（${T.autoAttempts}）不一致 —— 预算与步骤分叉了`);
    must(Number.isInteger(autoStep.retryDelaySeconds) && autoStep.retryDelaySeconds >= 0
      && autoStep.retryDelaySeconds === T.autoRetryDelay, 'C21',
      `auto 的 retryDelaySeconds（${autoStep.retryDelaySeconds}）与预算 timeouts.autoRetryDelay（${T.autoRetryDelay}）不一致`);
    must((autoStep.criteria || []).join(' ').includes('整段'), 'C21',
      'auto 的判据未写明「重试单元是整段 close → open → auto」（只重试 auto 会绑到没加载项目的新 IDE 实例上）');
  }
  if (portStep) {
    must(portStep.attempts === T.autoAttempts, 'C21',
      `port-listening 的 attempts（${portStep.attempts}）与预算 timeouts.autoAttempts（${T.autoAttempts}）不一致 —— 端口预算必须与整段尝试同源`);
  }

  // ---- C22：pre-flight quit 清场（#1634）—— quit 必须先于一切会话步骤，且在 close 之前 ----
  // 病根：脚本启动（IPC 链）下，残留/堆叠的未初始化 IDE 实例会让 open 把项目窗口叠成白屏（连菜单/Console
  // 都没有）。close 只关项目窗口、对未初始化实例无能为力 ⇒ 必须先 quit 退出整个 IDE。判据（唯一真源仍是
  // 计划）：quit 先于 close/open/auto、argv 写死 `quit`、声明进程归零轮询（waitSeconds）与失败只警告的
  // 语义、不判 exit 2。#1860：计划第一步是 env-preprobe（**只读**预探，不操控 IDE）——「quit 是第一步」
  // 收窄为「quit 是第一个会话步骤，其前只允许只读的 env-preprobe」。
  must(Boolean(quitStep), 'C22', '计划里没有 pre-flight `cli.bat quit` 步（残留 IDE 实例会把项目窗口叠成白屏，#1634）');
  if (quitStep) {
    must(
      ids.indexOf('devtools-quit') === 1 && ids[0] === 'env-preprobe',
      'C22',
      'quit 前面只允许只读的 env-preprobe（#1860），清场本身必须发生在 close/open/auto 之前'
    );
    must(argvOf(quitStep)[0] === 'quit', 'C22', 'quit 的 argv 不是 `quit`（清场要退出整个 IDE，不是 close 项目窗口）');
    must(Number.isFinite(quitStep.waitSeconds) && quitStep.waitSeconds > 0, 'C22', 'quit 缺进程归零轮询预算（waitSeconds）');
    must(Number.isFinite(quitStep.timeoutSeconds) && quitStep.timeoutSeconds > 0, 'C22', 'quit 缺超时兜底');
    must(quitStep.awaitCompletion === true, 'C22', 'quit 未声明「等它跑完」（与 close/open 同款可追溯口径）');
    must(quitStep.logged === true, 'C22', 'quit 未声明「写进日志」（清场结果必须可追溯）');
    const qc = (quitStep.criteria || []).join(' ');
    must(qc.includes('白屏'), 'C22', 'quit 的判据未写明白屏病根（#1634）');
    must(qc.includes('归零'), 'C22', 'quit 的判据未写明「轮询 wechatdevtools 进程归零」');
    must(qc.includes('只警告') || qc.includes('[warn]'), 'C22', 'quit 的判据未写明「失败只警告，不判 exit 2」');
    must(!qc.includes('Stop-Process') || qc.includes('不得用 Stop-Process'), 'C22', 'quit 的判据须写明「不得用 Stop-Process 替代 quit」（kill IDE 是 ADR-0008 红线）');
    must((quitStep.exit2 || []).length === 0, 'C22', 'quit 是「失败只警告」的 pre-flight 步骤，不该声明 exit 2');
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

  // ---- C23（#1655 / ADR-0036 处置二）：三分结果行形状写进计划（branches 键） ----
  // 三分 = 通过 metric 行（无 errors= token，errorsTotal=0 承载通过语义）/ 门未过 errors=<N> failures=…
  // / 环境受阻 errors=env-blocked reason=<slug>。
  // 分支声明是**产物契约**：机检方（dev-loop、agent）从计划就知道「环境性失败长什么样、
  // 处置是什么」——不用再从 publish 文案猜。改 token/分支形状必须同步这里（改了要同步）。
  const B = (plan && plan.branches) || {};
  must(B.pass && B.pass.errors === '<none>', 'C23', 'branches.pass.errors 不是字面量 \'<none>\'（通过行**没有** errors= token——判别写在缺席上，别把它改成值）');
  must(B.pass && B.pass.errorsTotal === '0', 'C23', 'branches.pass.errorsTotal 不是 \'0\'（通过语义由 metric 行的 errorsTotal=0 承载）');
  must(B.pass && B.pass.exitCode === 0, 'C23', 'branches.pass.exitCode 不是 0（通过 = exit 0）');
  must(B.gateFailed && B.gateFailed.errors === '<N>', 'C23', 'branches.gateFailed.errors 不是 \'<N>\'（门未过行的 errors= 值是失败计数）');
  must(B.gateFailed && B.gateFailed.failures === true, 'C23', 'branches.gateFailed 未声明 failures= 字段（门未过行必须带 failures=）');
  must(B.gateFailed && B.gateFailed.exitCode === 1, 'C23', 'branches.gateFailed.exitCode 不是 1（门未过 = exit 1）');
  must(B.envBlocked && B.envBlocked.errors === 'env-blocked', 'C23', 'branches.envBlocked.errors 不是 \'env-blocked\'（环境受阻的一等 token，ADR-0036 处置二）');
  must(B.envBlocked && B.envBlocked.reason === true, 'C23', 'branches.envBlocked 未声明 reason= 字段（env 行必须带 reason=<slug>）');
  must(B.envBlocked && B.envBlocked.exitCode === 2, 'C23', 'branches.envBlocked.exitCode 不是 2（环境受阻 = exit 2）');
  must(B.envBlocked && B.envBlocked.classifier === 'Get-MpWeixinEnvClassification', 'C23',
    'branches.envBlocked.classifier 不是 Get-MpWeixinEnvClassification（分类判据的唯一真源在 scripts/lib/mp-weixin-env.ps1）');
  const catalog = (B.envBlocked && Array.isArray(B.envBlocked.envReasons)) ? B.envBlocked.envReasons : [];
  must(catalog.length > 0, 'C23', 'branches.envBlocked.envReasons 缺失（环境受阻的 reason 枚举必须写进计划，机检方按它判断处置）');
  must(new Set(catalog).size === catalog.length, 'C23', 'branches.envBlocked.envReasons 有重复项（枚举必须唯一）');
  catalog.forEach((r) => must(/^[a-z0-9-]+$/.test(r), 'C23', `envReasons 枚举值 ${r} 不是 kebab-case slug（reason= 值域纪律）`));
  // 计划 exit2 声明的 reason（脚本侧直接 emit 的环境性失败）必须全部落在 envReasons 枚举里
  (Array.isArray(plan && plan.exit2) ? plan.exit2 : []).forEach((e) => {
    must(catalog.includes(e.reason), 'C23', `exit2 的 reason ${e.reason}（${e.step}/${e.on}）不在 envReasons 枚举里 —— 直接 emit 的环境性失败必须能被枚举覆盖`);
  });

  // ---- C24（#1682 · ADR-0039 拆分步骤 2）：任意页直达探针（cold-start-direct）的计划面声明 ----
  //   机制由 ADR-0038 决策 1 裁定为唯一可靠：构建产物 app.json 首项临时换页 + plain 冷启动；
  //   源头 pages.json 是运行时面判据来源，绝不许动（决策 2 红线）。缺这节 ⇒ 执行路径退化成硬编码参数。
  const dp = plan && plan.directPages;
  must(Boolean(dp), 'C24', '门计划缺 directPages 节（#1682 直达探针没有唯一真源）');
  if (dp) {
    must(dp.mechanism === 'cold-start-direct', 'C24', 'directPages.mechanism 不是 cold-start-direct（ADR-0038 决策 1 的机制是唯一一个，勿另选）');
    must(dp.swapTarget === 'app.json', 'C24', 'directPages.swapTarget 不是 app.json（换页对象=构建产物 app.json 的 pages 首项）');
    must(String(dp.swapNote || '').includes('不动源头 pages.json'), 'C24', 'directPages.swapNote 未声明「不动源头 pages.json」（ADR-0038 决策 2 红线）');
    must(dp.restore === 'finally', 'C24', 'directPages.restore 不是 finally（换页必须在异常路径也还原）');
    const dpArgv = Array.isArray(dp.probeArgv) ? dp.probeArgv.map(String) : [];
    must(dpArgv.includes('--expect-page'), 'C24', '直达探针 argv 模板缺 --expect-page（页身份判据=落地的 pageStack[0] 等于目标路由）');
    must(dpArgv.includes('<page>'), 'C24', '直达探针 argv 模板缺 <page> 占位（页清单由执行路径传入，计划不写死页）');
    const wsAt = dpArgv.indexOf('--ws');
    must(wsAt >= 0 && dpArgv[wsAt + 1] === plan.endpoint.ws, 'C24', 'directPages.probeArgv 的 --ws 值与 endpoint.ws 不一致');
    const shotAt = dpArgv.indexOf('--shot-dir');
    must(shotAt >= 0 && dpArgv[shotAt + 1] === plan.paths.logDir, 'C24', 'directPages.probeArgv 的 --shot-dir 值与 paths.logDir 不一致');
    const rootAt = dpArgv.indexOf('--require-root');
    must(rootAt >= 0 && dpArgv[rootAt + 1] === plan.paths.automatorRoot, 'C24', 'directPages.probeArgv 的 --require-root 值与 paths.automatorRoot 不一致');
    const msAt = dpArgv.indexOf('--timeout-ms');
    must(msAt >= 0 && String(dpArgv[msAt + 1]) === String(plan.timeouts.probeTimeoutMs), 'C24', 'directPages.probeArgv 的 --timeout-ms 值与预算 probeTimeoutMs 不一致');
    must(Number.isFinite(dp.attempts) && dp.attempts >= 1, 'C24', 'directPages.attempts 缺失或 < 1（直达探针仍需重试预算，只针对会话级短暂失败）');
    const sr = Array.isArray(dp.skipReasons) ? dp.skipReasons.map(String) : [];
    must(sr.length > 0, 'C24', 'directPages.skipReasons 为空（SKIP reason 值域必须可枚举——诚实降级的前提）');
    sr.forEach((r) => must(/^[a-z0-9-]+$/.test(r), 'C24', `skipReasons 值 ${r} 不是 kebab-case slug`));
    must(sr.includes('direct-page-unsupported'), 'C24', 'skipReasons 缺 direct-page-unsupported（落地页≠目标路由必须记 SKIP）');
    // ---- 锚点元素断言（#1919）的计划面声明：锚点表是唯一真源，形状与首期锚点值都锁 ----
    const anchors = dp.anchors;
    must(Boolean(anchors) && Object.keys(anchors).length > 0, 'C24', 'directPages.anchors 缺失或为空（锚点表必须由计划声明，执行侧不得硬编码锚点）');
    if (anchors) {
      Object.keys(anchors).forEach((k) => {
        must(/^pages(\/[a-z0-9-]+)+$/.test(k), 'C24', `anchors 键 ${k} 不是页面路由形态（/^pages(\\/[a-z0-9-]+)+$/）`);
        const a = anchors[k] || {};
        must(typeof a.selector === 'string' && a.selector.startsWith('.'), 'C24', `anchors[${k}].selector 不是类选择器（uni-app x 不能跨组件选择，锚点必须页面级可达）`);
        must(typeof a.text === 'string' && a.text.length > 0, 'C24', `anchors[${k}].text 为空（element.text() 精确比对需要期望值）`);
      });
      const loginAnchor = anchors['pages/login/login'];
      // 首期锚点精确 pin（#1917 单验值）：改锚点必须重新单验并同步判据，不许静默漂移
      must(Boolean(loginAnchor) && loginAnchor.selector === '.app-title' && loginAnchor.text === '叉车维修培训系统', 'C24', 'login 锚点不是 {.app-title, 叉车维修培训系统}（#1917 单验值，改动须重新单验并同步判据）');
    }
    const dc = (dp.criteria || []).join('\n');
    must(dc.includes('冷启动'), 'C24', 'directPages.criteria 未写冷启动直达机制');
    must(dc.includes('还原'), 'C24', 'directPages.criteria 未写还原纪律');
    must(dc.includes('skip'), 'C24', 'directPages.criteria 未写 SKIP 诚实降级');
    const sr24 = Array.isArray(dp.skipReasons) ? dp.skipReasons.map(String) : [];
    must(sr24.includes('auth-session-unavailable'), 'C25', 'directPages.skipReasons 缺 auth-session-unavailable（#1690 会话注入失败的回退 reason，值域要可枚举）');
  }

  // ---- C25（#1690）：登录态会话注入步的计划面声明 ----
  //   登录态页直达冷启动无会话 ⇒ 401 被弹回登录页（#1610 缺口）。注入只发生在门运行环境
  //   （automator callWxMethod），不改业务代码；失败是诚实 SKIP，不是环境 blocked。
  const authStep = step('auth-inject');
  must(Boolean(authStep), 'C25', '门计划缺 auth-inject 步（#1690 会话注入没有唯一真源）');
  if (authStep) {
    must(authStep.exec === 'node', 'C25', 'auth-inject 不是 node 脚本步骤');
    const aArgv = argvOf(authStep).map(String);
    must(aArgv[0] === String(P.auth), 'C25', 'auth-inject 的 argv[0] 与 paths.auth 不一致（脚本路径要由计划派生）');
    const aWs = aArgv.indexOf('--ws');
    must(aWs >= 0 && aArgv[aWs + 1] === E.ws, 'C25', 'auth-inject 的 --ws 与 endpoint.ws 不一致');
    const aRoot = aArgv.indexOf('--require-root');
    must(aRoot >= 0 && aArgv[aRoot + 1] === P.automatorRoot, 'C25', 'auth-inject 的 --require-root 与 paths.automatorRoot 不一致');
    must(Array.isArray(authStep.exit2) && authStep.exit2.length === 0, 'C25', 'auth-inject 不该声明 exit 2（注入失败⇒诚实 SKIP，不得升级成环境 blocked）');
    const ac = (authStep.criteria || []).join('\n');
    must(ac.includes('MP_WEIXIN_AUTH'), 'C25', 'auth-inject 判据未写 MP_WEIXIN_AUTH 结果行');
    must(ac.includes('existing') && ac.includes('injected') && ac.includes('none'), 'C25', 'auth-inject 判据未枚举 status 值域（existing/injected/none）');
    must(ac.includes('令牌'), 'C25', 'auth-inject 判据未声明令牌不入结果行（凭据不得进日志/评论）');
  }
  must(P.authRelative === 'scripts\\mp-weixin-auth.mjs', 'C25', '注入脚本相对路径不是 scripts\\mp-weixin-auth.mjs');

  // ---- C26（#1816）：直达探针的**组级 navigation 形态**——部分达成不得塌缩成整组 skip ----
  //   2026-10-23 实测：2 页里 index 达成、login 截图 automator 超时 ⇒ 旧实现整组只剩 skip(reason=…)，
  //   已达成页的成功读数被一页 miss 吞掉。形态必须由计划声明（ok / partial / skip / n/a），
  //   partial 必须带 ok=/skip= 两个计数（只说「部分」等于没说缺几页）。
  if (dp) {
    const ns = dp.navigationShapes;
    must(Boolean(ns), 'C26', 'directPages 缺 navigationShapes 节（组级 navigation 形态没有唯一真源）');
    if (ns) {
      ['ok', 'partial', 'skip', 'envEarly'].forEach((k) => {
        must(String(ns[k] || '').length > 0, 'C26', `directPages.navigationShapes 缺 ${k} 形态（组级形态值域必须可枚举）`);
      });
      must(String(ns.partial || '').includes('partial(ok='), 'C26', 'navigationShapes.partial 必须形如 partial(ok=<N>,skip=<M>)（计数要出现在结果行）');
      must(String(ns.partial || '').includes('skip='), 'C26', 'navigationShapes.partial 缺 skip=<M> 计数（只说「部分达成」等于没说缺几页）');
      must(String(ns.skip || '').includes('skip(reason='), 'C26', 'navigationShapes.skip 必须是 skip(reason=<slug>) 形态（与 skipReasons 同源）');
      must(String(ns.envEarly || '') === 'n/a', 'C26', 'navigationShapes.envEarly 必须是 n/a（环境早退没走到导航，不得写成 ok/skip）');
    }
    must((dp.criteria || []).join('\n').includes('partial'), 'C26', 'directPages.criteria 未写 partial 形态（部分达成必须如实报告，不得塌缩成整组 skip）');
  }

  // ---- C27（#1817）：会话分相判据 = 入口页 ∪ 游客页族，不得用「是否入口页」单相判据 ----
  //   2026-10-23 实测：login 这类游客页被注 auth 会话后自己 reLaunch 到 dashboard，
  //   冷启动落地页 ≠ 目标页 ⇒ 截 miss-login.png 撞 automator 应答超时，页级 reason 还被冒充成
  //   direct-page-unsupported。判据的真源必须由计划声明（routes 清单 + rationale），不得散落。
  if (dp) {
    const gp = dp.guestPhase;
    must(Boolean(gp), 'C27', 'directPages 缺 guestPhase 节（会话分相判据没有唯一真源）');
    if (gp) {
      const gr = Array.isArray(gp.routes) ? gp.routes : [];
      must(gr.length > 0, 'C27', 'guestPhase.routes 为空（游客页族必须可枚举，不能靠执行侧猜）');
      ['pages/login/login', 'pages/register/register'].forEach((r) => {
        must(gr.includes(r), 'C27', `guestPhase.routes 缺 ${r}（已登录会自己跳走的页必须走 guest 相）`);
      });
      must(String(gp.rationale || '').includes('游客页族'), 'C27', 'guestPhase 缺 rationale（分相判据的来由必须写在计划里，否则后人会照「入口页」单相改回去）');
      must(String(gp.mode || '') === 'guest', 'C27', 'guestPhase.mode 必须是 guest（游客页族走 guest 相清会话）');
    }
    const gpc = (dp.criteria || []).join('\n');
    must(gpc.includes('游客页族'), 'C27', 'directPages.criteria 未写「游客页族」分相判据（#1817）');
    must(gpc.includes('landed-elsewhere'), 'C27', 'directPages.criteria 未写 landed-elsewhere 页级 reason（落地到别处必须点名落到哪）');
  }

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

  // C3 pageStack 前置断言 + 禁令收窄（#1917 裁决 + #1919 落锁）：
  //   page.waitFor(选择器) 全文件禁（#1917 实测挂起 8s，条件等待用轮询 page.$）；
  //   导航 API 在锚点段 [C3-anchor-begin]–[C3-anchor-end] 内禁（段标记缺失判红 = fail-closed）；
  //   page.$ + element.text() 锚点断言放行（正向自检锁「不再误伤」，见放行语义锁用例）。
  must(probe.includes('pageStackNonEmpty'), 'C3', '探针缺 pageStack 非空前置断言');
  must(probe.includes('getPageMetaByWebviewId'), 'C3', '探针未记录 pageStack 空导致的误导弹错（#883 坑位 1）');
  // 锚点参数入口被拆 ⇒ 红（--assert-selector / --assert-text 是锚点断言的唯一入口）
  must(probe.includes("'--assert-selector'") && probe.includes("'--assert-text'"), 'C3', '探针缺 --assert-selector/--assert-text 解析（#1919 锚点断言入口被拆）');

  // ---- C29（#2097）：逐屏取证的滚动机制必须走 scroll-view 元素 API ----
  // 血账（2026-10-29 真跑实测）：第一版判停读 getCurrentPages() 末页的 p.scrollTop —— 小程序 Page 没有
  // 这个属性，读不到即落到 0 ⇒ 首次滚动后立刻判「已到页尾」，三页全只出 -p1 且无任何报错。且本仓页面
  // 都在 <scroll-view> 里滚，页级 wx.pageScrollTo 恒无效。故：量 scrollHeight、scrollTo 驱动、property 回读。
  // ⚠️ 判据必须吃**块注释也抹净**的视图：血账原文写在函数文档里，只抹 // 会被自己的注释绊红。
  {
    const probeCode = probe.replace(/\/\*[\s\S]*?\*\//g, '');
    must(probeCode.includes('scrollHeight('), 'C29', '逐屏取证没量滚动容器内容高度（算不出该拍几屏）');
    must(probeCode.includes('.scrollTo('), 'C29', '逐屏取证没经 scroll-view 元素的 scrollTo 驱动滚动');
    must(probeCode.includes("property('scrollTop')"), 'C29', '逐屏取证没回读实际 scrollTop（证明不了真滚了）');
    must(!/getCurrentPages\(\)[\s\S]{0,120}?scrollTop/.test(probeCode), 'C29', '仍从 getCurrentPages() 末页读 scrollTop 判停（该属性不存在 ⇒ 恒判已到页尾，#2097 血账）');
    must(probeCode.includes('shotWarning'), 'C29', '没有滚动容器 / 滚动无效时的降级没有留名（会冒充「已逐屏」）');
  }
  probe.split(/\r?\n/).forEach((line, idx) => {
    if (/^\s*\/\//.test(line)) return; // 注释里提 API 名是允许的；本守护守的是「别用」
    if (/page\.waitFor\(\s*['"`]/.test(line)) {
      violations.push('C3 第 ' + (idx + 1) + ' 行用了 page.waitFor(选择器)（#1917 实测挂起 8s，条件等待用轮询 page.$）');
    }
  });
  // 锚点段内禁 waitFor/导航 API；段标记缺失本身判红（段扫失去范围不得静默放行）。
  // 终点标记从 begin 标记**所在行之后**起找：probe 头注里自述扫法时会含 [C3-anchor-end] 字面量，
  // 直接 indexOf 会命中注释行、把段扫范围塌缩成一行（#1919 实测踩过）。
  const anchorBeginAt = probe.indexOf('[C3-anchor-begin]');
  let anchorEndAt = -1;
  if (anchorBeginAt !== -1) {
    const beginLineEnd = probe.indexOf('\n', anchorBeginAt);
    anchorEndAt = probe.indexOf('[C3-anchor-end]', beginLineEnd === -1 ? anchorBeginAt : beginLineEnd);
  }
  if (anchorBeginAt === -1 || anchorEndAt === -1 || anchorEndAt <= anchorBeginAt) {
    violations.push('C3 探针缺 [C3-anchor-begin]/[C3-anchor-end] 标记（锚点段扫失去范围，fail-closed）');
  } else {
    probe.slice(anchorBeginAt, anchorEndAt).split(/\r?\n/).forEach((line, idx) => {
      if (/^\s*\/\//.test(line)) return;
      if (/page\.waitFor\(|\bmp\.(reLaunch|navigateTo)\(/.test(line)) {
        violations.push('C3 锚点段第 ' + (idx + 1) + ' 行用了 waitFor/导航 API（锚点段只许 page.$ + element.text()，#1917/#1919）');
      }
    });
  }

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

  // C9b 入库触发面（2026-10-29 A1 收尾）：入库必须是**显式** `-Archive` 才发生。
  // 为什么单独钉：A1 之后「拿证据块」必须传 `-PostToPr`，若入库仍由号码触发，正常取证会顺手
  // `git add`+`commit`+push ⇒ 门给分支造一个 commit、head sha 被挪走，正文 `跑于 <sha7>` 从 head 变祖先。
  // 三条一起钉（开关在 / 旧条件消失 / 守卫在调用之前）：任一条单独都能被「保留开关但仍由号码触发」绕过。
  must(/\[switch\]\$Archive\b/.test(s), 'C9b', '缺 [switch]$Archive（截图入库必须是显式动作，不能由号码顺带触发）');
  must(!s.includes('if ($PostToPr -gt 0 -and -not $NoArchive)'), 'C9b', '入库触发退回旧条件（只要有号码就 commit + push ⇒ 挪走 head sha）');
  const archGuardAt = s.indexOf('if ($Archive');
  const archCallAt = s.indexOf('Publish-ScreenshotArchive -PrNumber');
  must(archGuardAt !== -1 && archCallAt !== -1 && archGuardAt < archCallAt, 'C9b',
    '$Archive 守卫不在 Publish-ScreenshotArchive 调用之前（守卫与调用脱节 ⇒ 开关形同虚设）');

  // C30 覆盖判据（#2101 落库，2026-10-29）：`-Pages auto` 只认 `pages/**/*.uvue`，改 `components/` / `utils/*.uts` /
  // `App.uvue` / `pages.json` 时清单为空 ⇒ 只跑入口页一张 = **覆盖为零的绿**（#2099 盘点出的洞）。
  // 三分支语义（none 不退化冒烟集 / 空清单退冒烟集 / 改动页全跑）的**真执行**由
  // utils/mpWeixinPagesCoverageBehavior.test.js 钉住；这里只钉「接上了 + 读数不说谎」，因为文本断言
  // 测不到分支行为（C20 教训：判据被改成 `if ($false)` 时文本照样绿）。
  must(/smokePages\s*=\s*@\('pages\/dashboard\/dashboard',\s*'pages\/profile\/profile'\)/.test(s), 'C30',
    '计划未声明冒烟集两页（清单为空时只剩入口页 ⇒ 覆盖为零的绿）');
  must(s.includes('Resolve-MpWeixinDirectCoverage -AutoPages $autoPages'), 'C30',
    'auto 分支没有调用覆盖判据库（判据写了但没接线 = 死代码）');
  must(s.includes('$coverageField = "$($resolution.Coverage)"'), 'C30',
    '覆盖读数没有从判据库取（门脚本自己算第二份 = 两个真源）');
  must(/name = 'coverage'; optional = \$true/.test(s), 'C30',
    '结果行字段清单里没有 coverage（计划声明了而执行不填 = 假声明）');
  must(/\$metricValues\['coverage'\] = "\$coverageField"/.test(s), 'C30',
    'coverage 未注入结果行（字段声明了但永远是空的）');
  must(!template.includes('观测不到的能力面'), 'C30',
    'PR 模板仍留着 ①b 的能力面自报框（#2103 说删；勾了不判红、也不决定人签，纯格式税）');
  must(template.includes('能力面：'), 'C30',
    'PR 模板没给「命中能力面时写一行」的写法（删了框却没给替代，等于把该记的东西也删了）');

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
  // #1861 起锚点**不含前缀**：结果行前缀已收成 Get-GateResultPrefix（-BuildOnly 出独立 token），
  // 锚死 `MP_WEIXIN_RESULT ` 等于逼执行路径为过守护再把前缀硬写一次 —— 那正是这次重构要消掉的东西。
  const lastDistGuard = s.indexOf('errors=env-blocked reason=no-dist');
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

  // C26（#1816）结果行与日志面：组级 partial 必须真的写出来，且不得写成 PASS
  must(s.includes('"partial(ok=$navOkCount,skip=$navSkipCount)"'), 'C26', '结果行没有把组级部分达成写成 partial(ok=N,skip=M)（一页 miss 不许吞掉同组成功读数）');
  must(s.includes('navigation=PARTIAL'), 'C26', '日志缺 `navigation=PARTIAL ok=… skip=…` 行（部分达成必须进日志，不能只在内存里）');
  must(s.includes("Get-Prop $nav 'pages'"), 'C26', '未从 navigation.pages 读页级读数（未达成页的页级原因与已达成页的读数都必须可追溯）');

  // C27（#1817）执行侧面：分相判据必须真的接在「游客页族」上，且落地到别处要点名
  must(s.includes('$isGuestPage'), 'C27', '执行路径缺 $isGuestPage（分相判据必须是入口页 ∪ 游客页族，不能只看 $isEntry）');
  must(s.includes("Get-Prop $script:GatePlan['directPages'] 'guestPhase'"), 'C27', '执行路径未从计划 guestPhase 取游客页族（会手抄第二份清单）');
  must(s.includes('landed-elsewhere'), 'C27', '执行路径未记 landed-elsewhere(<落地页>) 页级 reason（落地到别处会被冒充成 direct-page-unsupported）');
  must(!/\$isEntry\)\s*\{\s*$/m.test(s) || s.includes('$isGuestPage'), 'C27', '分相仍以 $isEntry 单相判定（游客页会被注 auth 会话后自己跳走）');
  // Get-Prop 必须字典感知：计划是真源（[ordered]），而 [ordered] 的键不是 PSObject 属性。
  // 2026-10-23 实测：不改这一处时 $guestRoutes 恒为空、登录页照旧被注会话，只有真跑才照得出来。
  must(s.includes('$Object -is [System.Collections.IDictionary]'), 'C27', 'Get-Prop 不读字典键（[ordered] 计划被读成 $null，声明形同虚设）');

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

  // C22 pre-flight quit 清场的执行侧接线（#1634；计划面判据在 scanPlan）
  //   · 有按 id 取用 quit 步的执行点（计划与执行不脱节）；
  //   · 进程归零轮询按 ProcessName 双名匹配 wechatdevtools|微信开发者工具（#1690：本机 IDE 进程名是中文，
  //     只数英文名恒 0 ⇒ 归零轮询形同虚设、冷启动叠在垂死实例上；只观测，绝不 kill —— Stop-Process 是 ADR-0008 红线）；
  //   · 主流程（重试单元之前）真的调用了 pre-flight，且 -Doctor 跳过。
  must(s.includes("-Id 'devtools-quit'"), 'C22', '执行路径里没有按 id 取用 quit 步骤（计划与执行脱节）');
  must(/ProcessName -match 'wechatdevtools\|微信开发者工具'/.test(s), 'C22', '执行侧没有 IDE 进程归零双名轮询（wechatdevtools/微信开发者工具；quit 是异步的，不轮询等于没等清场，只数英文名在本机恒 0 —— #1690 实测）');
  must(!/Stop-Process\s+-/.test(stripCommentLines(s).join('\n')), 'C22', '执行侧出现 Stop-Process 调用（运行中 kill IDE 是 ADR-0008 红线，只能 quit；判据文本里「不得用 Stop-Process」的引用不算）');
  must(/if \(-not \$Doctor\) \{ Invoke-DevToolsPreflightQuit \}/.test(s), 'C22', '主流程没有在重试单元之前调用 pre-flight（或未做 -Doctor 跳过）');
  must(s.includes('function Invoke-DevToolsPreflightQuit'), 'C22', '缺 pre-flight quit 的执行函数');

  // ---- C24（#1682）执行侧：直达分支必须真实接线 ----
  must(s.includes("[string]$Pages = 'auto'"), 'C24', '门脚本缺 -Pages 参数声明（#1682 交付物载体；判到默认值 auto，防 $PagesMode 前缀掩盖缺席）');
  must(s.includes("'<page>'"), 'C24', '执行路径未消费 <page> 占位（直达 argv 未从 directPages.probeArgv 模板化）');
  must(s.includes('origin/master...HEAD'), 'C24', '-Pages auto 模式缺「本 PR 改动页」判据（git diff origin/master...HEAD）');
  must(s.includes('function Switch-DistAppJsonFirstPage'), 'C24', '缺构建产物 app.json 首项换页函数');
  must(s.includes('function Restore-DistAppJson'), 'C24', '缺 app.json 还原函数');
  const c24FinallyAt = s.lastIndexOf('} finally {');
  must(c24FinallyAt !== -1 && s.slice(c24FinallyAt).includes('Restore-DistAppJson'), 'C24', 'Restore-DistAppJson 不在 finally 路径（异常路径换页会留脏产物）');
  must(s.includes('page-not-in-build'), 'C24', '执行路径未记 page-not-in-build 的 SKIP');
  must(s.includes('direct-page-unsupported'), 'C24', '执行路径未记 direct-page-unsupported 的 SKIP');
  must(s.includes('cold-start-direct'), 'C24', '执行路径未携带机制标识 cold-start-direct');
  // #1690 终版归因：入口页 = welcome 着陆页（#1478 有会话必弹 dashboard）⇒ 入口页必须独立换页→冷启动，
  // 且冷启动前处于 guest 相（无会话）；「吃主会话」旧口径把主会话落地页冒充入口页取证。
  must(!s.includes('if ($pg -ne $entryRoute)'), 'C24', '直达循环回退成「入口页跳过换页吃主会话」旧口径（#1690：主会话落地页 ≠ 入口页取证）');
  must(s.includes('port-held-after-quit'), 'C24', '直达冷启动缺「端口被垂死实例占住」判据（#1690 run5：open 骑旧会话 ⇒ 假 miss；归零后必须等端口收摊）');
  must(probe.includes('shotErr'), 'C24', '直达截图缺重试壳（#1690 run5：冷启动后渲染服务未热，mp.screenshot 撞 automator 15s 应答超时 ⇒ 重试一次再记错）');
  must(s.includes('键序轮转'), 'C24', '换页写入缺「键序轮转」保证（#1690 run6：pages[0] 未变时字节不变 ⇒ IDE 不重编译、窗口停在上一页 ⇒ 入口页假 miss）');
  must(s.includes('-ne @($re.pages).Count'), 'C24', '换页读回复验缺「路由唯一性」判据（#1690 run7-10：重复路由让构建器拒启模拟器，症状伪装成 page-stack-not-answering）');
  must(s.includes("-Mode 'guest'"), 'C24', '直达循环缺 guest 相（入口页冷启动前必须清会话——#1478 welcome 页「已登录无条件落 dashboard」业务不变量，有会话的 index 冷启动必 miss，run4/6/11/12 实证）');
  must(s.includes('$isEntry = ($pg -eq $entryRoute)'), 'C24', '直达循环缺「入口页 == guest 相」分相判据（两相序真源，#1690 run12）');
  must(s.includes('入口漂移自愈'), 'C24', '缺「入口漂移自愈」判据（#1690 run14：上次强杀遗留换页 ⇒ 孤儿页被当入口静默取证，index 漏测还判 ✅）');
  // #1682 核验补记③：入库数据流必须钉住——AC1「每被改页 ≥1 张入仓截图」全靠直达截图并入聚合
  // shots、再写回 probeJson（Publish-ScreenshotArchive 只遍历 probeJson.shots）。拆掉这条接线，
  // 门照样绿、入库目录却静默退化成「只有入口页 1 张」——正是核验时抓到的那个缺口。
  must(s.includes('$agg.shots += @($pj.shots)'), 'C24', '直达组截图未并入聚合 shots ⇒ AC1 入库链断（Publish-ScreenshotArchive 只遍历 probeJson.shots）');
  must(s.includes('shots = @($agg.shots)'), 'C24', '聚合 shots 未写回 probeJson ⇒ 入库与结果行不再共用同一清单');
  // #1682 核验补记①：评论渲染必须过滤 null 条目——@(Get-Prop $null 'Rel') = @($null)（单元素= null），
  // 不过滤会把 -NoArchive / 未入库渲染成一条空反引号条目，兜底文案永不出现。
  const gateCommentBody = psFunctionBody(code, 'function Publish-GateComment');
  must(gateCommentBody.length > 0, 'C24', '定位不到 Publish-GateComment 函数体（无法核对入库清单渲染的 null 过滤）');
  must(gateCommentBody.includes('@(@($ArchivedRel) | Where-Object { $_ })'), 'C24', 'Publish-GateComment 未过滤 null 入库条目 ⇒ -NoArchive 渲染成空条目假清单（#1682 核验补记①回归）');
  stripCommentLines(s).filter((l) => /Set-Content|Out-File/.test(l)).forEach((l) => {
    must(!l.includes('pages.json'), 'C24', `代码行回写源头 pages.json（ADR-0038 决策 2 红线）：${l.trim()}`);
  });
  must(probe.includes("'--expect-page'"), 'C24', '探针未解析 --expect-page 参数');
  must(probe.includes('cold-start-direct'), 'C24', '探针未记录直达达成的机制标识');
  must(probe.includes('direct-page-unsupported'), 'C24', '探针未记录直达未达成的 SKIP reason');
  // ---- 锚点断言（#1919）执行侧两条文本 pin：计划消费 + 参数拼接（拆掉任一条 ⇒ 门静默不跑锚点）----
  must(s.includes("Get-Prop $DirectPages 'anchors'"), 'C24', '执行路径未消费 directPages.anchors（锚点表没进 argv，门静默不跑锚点）');
  must(s.includes("'--assert-selector'"), 'C24', '执行路径未拼接 --assert-selector（锚点断言没接进直达探针 argv）');

  // ---- C25（#1690）执行侧：会话注入必须真实接线 ----
  must(s.includes("[string]$Auth = 'auto'"), 'C25', '门脚本缺 -Auth 参数声明（#1690 交付物载体；判到默认值 auto，防前缀掩盖缺席）');
  must(s.includes("Get-GateStep -Plan $script:GatePlan -Id 'auth-inject'"), 'C25', '执行路径未消费 auth-inject 步（缺唯一 Get-GateStep 取用点）');
  must(s.includes('function Invoke-AuthInject'), 'C25', '缺会话注入的执行函数 Invoke-AuthInject');
  must(s.includes('MP_WEIXIN_AUTH'), 'C25', '执行路径未解析 MP_WEIXIN_AUTH 结果行');
  must(s.includes('auth-session-unavailable'), 'C25', '执行路径未记 auth-session-unavailable 回退');
  must(s.includes("-Mode 'guest'") && s.includes("-Mode 'auth'"), 'C25', '门脚本缺两相会话调用（guest 相清入口页会话 / auth 相注入登录态页——#1690 run12 两相，#1478 welcome 不变量）');
  const authSrc = sources.auth || '';
  must(authSrc.includes('MP_WEIXIN_AUTH'), 'C25', '注入脚本不输出 MP_WEIXIN_AUTH 结果行');
  must(authSrc.includes('/auth/wx-login'), 'C25', '注入脚本未走真实 /auth/wx-login 通道');
  ['auth_token', 'auth_user', 'auth_refresh_token', 'auth_login_provider'].forEach((k) => {
    must(authSrc.includes("'" + k + "'"), 'C25', `注入脚本 storage 键缺 ${k}（须与 setAuthData 四键对齐）`);
  });
  must(authSrc.includes('callWxMethod'), 'C25', '注入脚本未用 callWxMethod（注入只发生在门运行环境）');
  must(authSrc.includes("a === '--mode'"), 'C25', '注入脚本缺 --mode 解析（guest/auth 两相的唯一开关）');
  must(authSrc.includes('gate-guest-invalid-json'), 'C25', '注入脚本 guest 相缺游客盖帽（auth_user 写非 JSON 让 restoreFromStorage 判游客——remove 不跨重启落盘，run13 实证；不盖帽 ⇒ 入口页被 #1478 弹 dashboard）');
  must(authSrc.includes('flushSettleMs'), 'C25', '注入脚本缺落盘兜底窗口 flushSettleMs（非默认安装面的盲等路径，须存在且带 settled-blind 上报）');
  must(authSrc.includes('flushConfirm') && authSrc.includes('WeappSimulator'), 'C25', '注入脚本缺落盘确认（#1690 run13 实证：storage 写只活 app 内存、quit 不冲刷 ⇒ 盖帽/注入必须轮询 WeappStorage 盘上出现唯一标记后才放行冷启动，否则白写）');
  must(authSrc.includes('storage-flush-timeout'), 'C25', '注入脚本缺 flush 超时诚实失败（落盘确认超时 ⇒ ok=false reason=storage-flush-timeout，不得假放行）');
  must(authSrc.includes('gate_flush_stamp'), 'C25', 'auth 相缺落盘 stamp（注入写回后以唯一 stamp 键确认刷盘——门自有键，应用不消费）');
  must(authSrc.includes("'cleared'"), 'C25', '注入脚本 guest 相缺 status=cleared 上报');
  // 只判代码行：注释里「不调 mp.close()」的纪律声明不算违规（剥 // 行注释后查真调用）
  const authCode = authSrc.split(/\r?\n/).filter((l) => !l.trim().startsWith('//')).join('\n');
  must(!authCode.includes('mp.close'), 'C25', '注入脚本关闭了会话（须留给后续探针，与 mp-weixin-probe.mjs 同纪律）');
  must(authSrc.includes('MP_WEIXIN_AUTH'), 'C25', '注入脚本注释未声明 MP_WEIXIN_AUTH 纪律（防后人加 close 断会话）');

  // C10 注册与文档
  must(/"build:mp-weixin-check"\s*:\s*"[^"]*scripts\/mp-weixin-check\.ps1"/.test(pkg), 'C10', 'package.json 未注册 build:mp-weixin-check');
  // C28 构建面模式入口（#1861 第一段）：入口是 DoD 的一部分，被静默摘掉就等于「能力没有落地」——
  // 判据是「入口存在**且**它真的带 -BuildOnly」（只判脚本名会被一个不带开关的同名入口蒙过去）。
  must(/"build:mp-weixin-buildonly"\s*:\s*"[^"]*scripts\/mp-weixin-check\.ps1\s+-BuildOnly"/.test(pkg), 'C28', 'package.json 未注册 build:mp-weixin-buildonly（或入口未带 -BuildOnly）');
  must(adr.includes('半自动'), 'C10', 'ADR-0008 未把 ② 记为半自动门');
  must(adr.includes('#883'), 'C10', 'ADR-0008 未引用 #883');
  must(adr.includes('mp-weixin-check.ps1'), 'C10', 'ADR-0008 未记录 ② 的脚本载体');
  // A1（#2100）：模板的证据形态是「正文一行 `跑于 <sha7>` + 门读数行」，
  // 不再要求写 marker / `-PostToPr` 评论承载 —— 那套载体已随 A1 整体退役。
  must(template.includes('跑于'), 'C10', 'PR 模板未写明 A1 证据形态（正文一行 `跑于 <sha7>`）');
  must(agents.includes('人工门只剩') || agents.includes('② 微信开发者工具门 = 半自动门'), 'C10', '移动端 AGENTS.md 未把 ② 移出人工门清单并写明运行前提');

  return violations;
}

describe('② 微信开发者工具门契约（#883 / 2026-09-12 半自动 / 2026-09-13 守护改断言门计划）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    probe: readSource(PROBE_REL),
    ready: readSource(READY_REL),
    auth: readSource(AUTH_REL),
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
      'env-preprobe:ide-port-unreachable',
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
      ['C21', (p) => editStep(p, 'devtools-auto', (s) => { s.attempts = 1; })],
      ['C21', (p) => editStep(p, 'devtools-auto', (s) => { delete s.attempts; })],
      ['C21', (p) => { p.timeouts.autoAttempts = 5; return p; }],
      ['C21', (p) => editStep(p, 'devtools-auto', (s) => { s.retryDelaySeconds = 0; })],
      ['C21', (p) => { p.timeouts.autoRetryDelay = 999; return p; }],
      ['C21', (p) => editStep(p, 'devtools-auto', (s) => { s.criteria = ['auto 必须等它跑完']; })],
      ['C21', (p) => editStep(p, 'port-listening', (s) => { s.attempts = 1; })],
      ['C15', (p) => { swap(p, 'devtools-auto', 'ready-gate'); return p; }],
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
      ['C18', (p) => { p.paths.probeRelative = 'scripts/other.mjs'; return p; }],
      // C22：拆掉 pre-flight quit / 挪到 close 之后 / 把它改成 close 的别名 / 摘掉红线与「只警告」判据
      ['C22', (p) => { p.steps = p.steps.filter((s) => s.id !== 'devtools-quit'); return p; }],
      ['C22', (p) => swap(p, 'devtools-quit', 'devtools-close')],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.argv = ['close', '--project', 'x']; })],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.waitSeconds = 0; })],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.criteria = ['清场']; })],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.exit2 = [{ on: 'timeout', reason: 'quit-timeout' }]; })],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.awaitCompletion = false; })],
      ['C22', (p) => editStep(p, 'devtools-quit', (s) => { s.logged = false; })],
      // C23：三分声明被改坏（通过行的 token 缺席声明、env token、分类器真源、枚举纪律、exit2 覆盖）
      ['C23', (p) => { delete p.branches; return p; }],
      ['C23', (p) => { p.branches.pass.errors = '0'; return p; }],
      ['C23', (p) => { p.branches.pass.errorsTotal = '1'; return p; }],
      ['C23', (p) => { p.branches.gateFailed.errors = '0'; return p; }],
      ['C23', (p) => { p.branches.gateFailed.failures = false; return p; }],
      ['C23', (p) => { p.branches.gateFailed.exitCode = 2; return p; }],
      ['C23', (p) => { p.branches.envBlocked.errors = 'env'; return p; }],
      ['C23', (p) => { p.branches.envBlocked.reason = false; return p; }],
      ['C23', (p) => { p.branches.envBlocked.classifier = 'AnythingElse'; return p; }],
      ['C23', (p) => { p.branches.envBlocked.envReasons = []; return p; }],
      ['C23', (p) => { const c = p.branches.envBlocked.envReasons.slice(); c.push(c[0]); p.branches.envBlocked.envReasons = c; return p; }],
      ['C23', (p) => { const c = p.branches.envBlocked.envReasons.slice(); c[0] = 'Port_Not_Listening'; p.branches.envBlocked.envReasons = c; return p; }],
      ['C23', (p) => { p.exit2 = p.exit2.concat([{ step: 'x', on: 'y', reason: 'brand-new-env-reason' }]); return p; }],
      // C24：直达声明被改坏（机制/换页对象/源头红线/还原/argv 模板/预算/SKIP 值域/判据）
      ['C24', (p) => { delete p.directPages; return p; }],
      ['C24', (p) => { p.directPages.mechanism = 'navigate-to'; return p; }],
      ['C24', (p) => { p.directPages.swapTarget = 'app.js'; return p; }],
      ['C24', (p) => { p.directPages.swapNote = '换页'; return p; }],
      ['C24', (p) => { p.directPages.restore = 'main'; return p; }],
      ['C24', (p) => { p.directPages.probeArgv = p.directPages.probeArgv.filter((a) => a !== '--expect-page'); return p; }],
      ['C24', (p) => { p.directPages.probeArgv = p.directPages.probeArgv.filter((a) => a !== '<page>'); return p; }],
      ['C24', (p) => { p.directPages.probeArgv[p.directPages.probeArgv.indexOf('--ws') + 1] = 'ws://127.0.0.1:1'; return p; }],
      ['C24', (p) => { p.directPages.probeArgv[p.directPages.probeArgv.indexOf('--timeout-ms') + 1] = '1'; return p; }],
      ['C24', (p) => { p.directPages.attempts = 0; return p; }],
      ['C24', (p) => { p.directPages.skipReasons = []; return p; }],
      ['C24', (p) => { p.directPages.skipReasons = ['DirectPageUnsupported']; return p; }],
      ['C24', (p) => { p.directPages.criteria = ['换页']; return p; }],
      // C25：会话注入被改坏（缺步/exit2 加宽/端点漂移/脚本路径漂移/SKIP 值域缺失）
      ['C25', (p) => { p.steps = p.steps.filter((x) => x.id !== 'auth-inject'); return p; }],
      ['C25', (p) => { const x = p.steps.find((y) => y.id === 'auth-inject'); x.exec = 'pwsh'; return p; }],
      ['C25', (p) => { const x = p.steps.find((y) => y.id === 'auth-inject'); x.exit2 = [{ on: 'fail', reason: 'auth-failed' }]; return p; }],
      ['C25', (p) => { const x = p.steps.find((y) => y.id === 'auth-inject'); x.argv[x.argv.indexOf('--ws') + 1] = 'ws://127.0.0.1:1'; return p; }],
      ['C25', (p) => { const x = p.steps.find((y) => y.id === 'auth-inject'); x.argv[0] = 'scripts/nope.mjs'; return p; }],
      ['C25', (p) => { const x = p.steps.find((y) => y.id === 'auth-inject'); x.criteria = ['跑注入']; return p; }],
      ['C25', (p) => { p.paths.authRelative = 'scripts\\nope.mjs'; return p; }],
      ['C25', (p) => { p.directPages.skipReasons = p.directPages.skipReasons.filter((r) => r !== 'auth-session-unavailable'); return p; }],
      // C26：组级 navigation 形态被改坏（缺节/缺形态/partial 无计数/skip 形态漂移/envEarly 漂移/判据未写 partial）
      ['C26', (p) => { delete p.directPages.navigationShapes; return p; }],
      ['C26', (p) => { delete p.directPages.navigationShapes.partial; return p; }],
      ['C26', (p) => { p.directPages.navigationShapes.partial = 'partial'; return p; }],
      ['C26', (p) => { p.directPages.navigationShapes.partial = 'partial(ok=<N>)'; return p; }],
      ['C26', (p) => { p.directPages.navigationShapes.skip = 'skip'; return p; }],
      ['C26', (p) => { p.directPages.navigationShapes.envEarly = 'environ'; return p; }],
      ['C26', (p) => { p.directPages.criteria = p.directPages.criteria.filter((c) => !c.includes('partial')); return p; }],
  // C27：会话分相判据被改坏（缺节/空清单/缺关键页/mode 漂移/判据未写）
  ['C27', (p) => { delete p.directPages.guestPhase; return p; }],
  ['C27', (p) => { p.directPages.guestPhase.routes = []; return p; }],
  ['C27', (p) => { p.directPages.guestPhase.routes = p.directPages.guestPhase.routes.filter((r) => r !== 'pages/login/login'); return p; }],
  ['C27', (p) => { p.directPages.guestPhase.mode = 'auth'; return p; }],
  ['C27', (p) => { p.directPages.guestPhase.rationale = '随便写的'; return p; }],
  ['C27', (p) => { p.directPages.criteria = p.directPages.criteria.filter((c) => !c.includes('游客页族')); return p; }],
  ['C27', (p) => { p.directPages.criteria = p.directPages.criteria.filter((c) => !c.includes('landed-elsewhere')); return p; }],
  // C24 锚点表（#1919）：缺表 / login 锚点漂移 / 非类选择器 ⇒ 计划面红
  ['C24', (p) => { delete p.directPages.anchors; return p; }],
  ['C24', (p) => { p.directPages.anchors['pages/login/login'].selector = '.subtitle'; return p; }],
  ['C24', (p) => { p.directPages.anchors['pages/login/login'].text = '登录'; return p; }],
  ['C24', (p) => { p.directPages.anchors['pages/login/login'] = { selector: 'view', text: 'x' }; return p; }],
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
      // C3 禁令收窄后的变异面：waitFor 引号形态（全文件禁）/ 锚点段内塞导航 API / 拆 --assert-* 入口
      ['C3', { ...real, probe: real.probe + '\nawait page.waitFor(".app-title");\n' }],
      ['C3', { ...real, probe: real.probe.replace('// [C3-anchor-end]', "mp.reLaunch('/pages/index/index');\n// [C3-anchor-end]") }],
      ['C3', { ...real, probe: real.probe.replace(/'--assert-text'/g, "'--assert-tx'") }],
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
      // C9b：把入库触发退回「有号码就入库」（A1 后正常取证会挪走 head sha）/ 删掉显式开关本身
      ['C9b', { ...real, script: real.script.replace('if ($Archive -and -not $NoArchive -and $PostToPr -gt 0) {', 'if ($PostToPr -gt 0 -and -not $NoArchive) {') }],
      ['C9b', { ...real, script: real.script.replace(/\[switch\]\$Archive,/, '') }],
      // C30：冒烟集被清空 / 判据库没接线 / 读数自己算第二份 / 字段不注入 / 自报框回到模板
      ['C30', { ...real, script: real.script.replace("smokePages = @('pages/dashboard/dashboard', 'pages/profile/profile')", 'smokePages = @()') }],
      ['C30', { ...real, script: real.script.replace('Resolve-MpWeixinDirectCoverage -AutoPages $autoPages', 'Get-NoSuchResolver -AutoPages $autoPages') }],
      ['C30', { ...real, script: real.script.replace('$coverageField = "$($resolution.Coverage)"', "$coverageField = 'changed:0'") }],
      ['C30', { ...real, script: real.script.replace("name = 'coverage'; optional = $true", "name = 'coverageX'; optional = $true") }],
      ['C30', { ...real, script: real.script.replace("$metricValues['coverage'] = \"$coverageField\"", '') }],
      ['C30', { ...real, template: real.template + '\n- [ ] 本次改动触及「agent 观测不到的能力面」\n' }],
      ['C30', { ...real, template: real.template.replace(/能力面：/g, '能力面X：') }],
      ['C10', { ...real, pkg: real.pkg.replace('build:mp-weixin-check', 'build:mp-x') }],
      // C28 必红：入口在、但没带 -BuildOnly（那是一个「能跑 ② 全量」的同名入口，不是构建面模式）
      ['C28', { ...real, pkg: real.pkg.replace('scripts/mp-weixin-check.ps1 -BuildOnly', 'scripts/mp-weixin-check.ps1') }],
      // C29 必红四条：删掉内容高度测量 / 改掉回读属性名 / 把无效判停塞回代码 / 抹掉降级留名
      ['C29', { ...real, probe: real.probe.replace(/scrollHeight\(/g, 'scrollHeightX(') }],
      ['C29', { ...real, probe: real.probe.replace(/property\('scrollTop'\)/g, "property('scrollTopX')") }],
      ['C29', { ...real, probe: real.probe.replace('const actual = await readScrollTop(container);', 'const actual = (getCurrentPages().pop() || {}).scrollTop;') }],
      ['C29', { ...real, probe: real.probe.replace(/step\.shotWarning/g, 'step.warnX') }],
      ['C10', { ...real, adr: real.adr.replace(/半自动/g, '全自动') }],
      ['C10', { ...real, template: real.template.replace(/跑于/g, '跑过') }],
      ['C10', { ...real, agents: '人工门清单（无 ② 条目）' }],
      ['C11', { ...real, script: real.script.replace(/hx-busy\.ps1/g, 'other.ps1') }],
      ['C11', { ...real, script: real.script.replace(/Release-HxLock/g, 'ReleaseNothing') }],
      ['C11', { ...real, script: real.script.replace('Release-HxLock', 'Stop-Process -Name HBuilderX -Force') }],
      ['C12', { ...real, script: real.script.replace("-Id 'devtools-open'", "-Id 'devtools-openX'") }],
      ['C12', { ...real, script: real.script.replace(/\$open\.TimedOut/g, '$openX.TimedOut') }],
      ['C12', { ...real, script: real.script.replace("errors=env-blocked reason=no-dist", "errors=env-blocked reason=nodist") }],
      // C13：把降级写成 PASS / 让 SKIP 静默变成通过 / 把结果行的 navigation 绑错
      ['C13', { ...real, script: real.script.replace('navigation=SKIP reason=', 'navigation=PASS reason=') }],
      ['C13', { ...real, script: real.script.replace('navigation = "$navField"', 'navigation = "ok"') }],
      ['C13', { ...real, script: real.script.replace(/降级（SKIP，非 PASS）/g, '结果说明') }],
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
      ['C19', { ...real, script: real.script.replace("贴 ② 门评论失败（门结论不受影响", "贴评论失败") }],
      // C22：执行侧拆掉 pre-flight（不再按 id 取用 quit / 轮询改 kill —— kill 是 ADR-0008 红线）
      ['C22', { ...real, script: real.script.replace("-Id 'devtools-quit'", "-Id 'devtools-quitX'") }],
      ['C22', { ...real, script: real.script.replace(/ProcessName -match 'wechatdevtools\|微信开发者工具'/g, "ProcessName -match 'wechatdevtools'") }],
      ['C22', { ...real, script: real.script.replace('if (-not $Doctor) { Invoke-DevToolsPreflightQuit }', 'if ($false) { }') }],
      // 红线面：真的往代码里塞 Stop-Process 调用必须被抓（判据文本里的「不得用 Stop-Process」引用不算命中）
      ['C22', { ...real, script: real.script.replace('        $waited++', '        Stop-Process -Name wechatdevtools -Force\n        $waited++') }],
      // C24：拆掉直达接线（参数/占位/判据/还原 finally/源头红线/探针直达模式）
      ['C24', { ...real, script: real.script.replace("[string]$Pages = 'auto'", "[string]$PagesX = 'auto'") }],
      ['C24', { ...real, script: real.script.replace(/'<page>'/g, 'null') }],
      ['C24', { ...real, script: real.script.replace(/origin\/master\.\.\.HEAD/g, 'HEAD') }],
      ['C24', { ...real, script: real.script.replace('function Restore-DistAppJson {', 'function RestoreX {') }],
      ['C24', { ...real, script: real.script.replace('\n    Restore-DistAppJson', '') }],
      ['C24', { ...real, script: real.script.replace('Set-Content -LiteralPath $Path -Value $out -Encoding utf8NoBOM -NoNewline', "Set-Content -LiteralPath 'pages.json' -Value 1") }],
      ['C24', { ...real, probe: real.probe.replace("'--expect-page'", "'--other-page'") }],
      ['C24', { ...real, probe: real.probe.replace(/cold-start-direct/g, 'x') }],
      ['C24', { ...real, probe: real.probe.replace(/direct-page-unsupported/g, 'x') }],
      ['C24', { ...real, script: real.script.replace('foreach ($pg in $directList) {', 'foreach ($pg in $directList) { if ($pg -ne $entryRoute) {') }],
      ['C25', { ...real, auth: real.auth.replace(/gate-guest-invalid-json/g, 'x') }],
      ['C25', { ...real, auth: real.auth.replace(/flushSettleMs/g, 'x') }],
      ['C25', { ...real, auth: real.auth.replace(/flushConfirm/g, 'x') }],
      ['C25', { ...real, auth: real.auth.replace(/storage-flush-timeout/g, 'x') }],
      ['C25', { ...real, auth: real.auth.replace(/gate_flush_stamp/g, 'x') }],
      ['C25', { ...real, auth: real.auth.replace("a === '--mode'", 'false') }],
      ['C25', { ...real, script: real.script.replace(/-Mode 'auth'/g, "-Mode 'x'") }],
      ['C24', { ...real, script: real.script.replace(/port-held-after-quit/g, 'x') }],
      ['C24', { ...real, script: real.script.replace(/键序轮转/g, 'x').replace(/\$rot = @\(\$keys\[1\.\.\(\$keys\.Count - 1\)\]\) \+ @\(\$keys\[0\]\)/, '$rot = $keys') }],
      ['C24', { ...real, script: real.script.replace(/-Mode 'guest'/g, "-Mode 'auth'") }],
      ['C24', { ...real, script: real.script.replace('$isEntry = ($pg -eq $entryRoute)', '$isEntry = $false') }],
    ['C24', { ...real, script: real.script.replace(/入口漂移自愈/g, 'x') }],
      ['C24', { ...real, probe: real.probe.replace(/shotErr/g, 'x') }],
      ['C24', { ...real, script: real.script.replace("if (@($re.pages | Select-Object -Unique).Count -ne @($re.pages).Count) { return 'app-json-invalid' }").replace(/\n *# 唯一性复验[^\n]*/g, '') }],
      // C24 入库数据流（#1682 核验补记③）与评论渲染 null 过滤（核验补记①）：拆接线必须被抓
      ['C24', { ...real, script: real.script.replace('$agg.shots += @($pj.shots)', '') }],
      ['C24', { ...real, script: real.script.replace('shots = @($agg.shots)', 'shots = @()') }],
      ['C24', { ...real, script: real.script.replace('@(@($ArchivedRel) | Where-Object { $_ })', '@($ArchivedRel)') }],
      // C26：结果行/日志的 partial 形态被拆（部分达成退回「整组 skip 或说不清」）
      ['C26', { ...real, script: real.script.replace('partial(ok=$navOkCount,skip=$navSkipCount)', 'partial') }],
      ['C26', { ...real, script: real.script.replace('navigation=PARTIAL', 'navigation=SKIP') }],
      ['C26', { ...real, script: real.script.replace("Get-Prop $nav 'pages'", "Get-Prop $nav 'pagez'") }],
  // C27：执行侧分相判据被拆（退回 $isEntry 单相 / 手抄清单 / 落地原因被冒充）
  ['C27', { ...real, script: real.script.replace(/\$isGuestPage/g, '$isEntry') }],
  ['C27', { ...real, script: real.script.replace("Get-Prop $script:GatePlan['directPages'] 'guestPhase'", "Get-Prop $script:GatePlan['directPages'] 'guestPhaze'") }],
  ['C27', { ...real, script: real.script.replace(/landed-elsewhere/g, 'direct-page-unsupported') }],
  ['C27', { ...real, script: real.script.replace('$Object -is [System.Collections.IDictionary]', '$false') }],
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

  it('C3 放行语义锁：page.$+element.text() 锚点写法不得再被判违规（#1917/#1919 正向自检）', () => {
    // 禁令收窄的镜像断言：若有人把 C3 误改回「禁一切元素级 API」，这条必须先红——
    // 否则变异自检全绿也只证明「还禁着」，证明不了「放行了」。
    const withAnchor = { ...real, probe: real.probe + '\nconst el = await page.$(".app-title");\nconst t = await el.text();\n' };
    expect(scanContract(withAnchor).filter((v) => v.startsWith('C3'))).toEqual([]);
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

  it('C22：pre-flight quit 清场在计划里先于一切会话步骤（#1634 白屏病根；#1860 起计划第一步是只读预探）', () => {
    const quit = realPlan.steps.find((s) => s.id === 'devtools-quit');
    // #1860：env-preprobe 是新的计划第一步 —— 只读（.ide 端口文件 + TCP 试连），不操控 IDE，
    // 排在 quit 之前只是「先探后动」；**第一个会话步骤仍是 quit**（本条不变量的本意）。
    expect(realPlan.steps[0].id).toBe('env-preprobe');
    expect(realPlan.steps[1].id).toBe('devtools-quit');
    expect(quit.argv).toEqual(['quit']);
    expect(quit.exit2).toEqual([]); // 失败只警告，不判 exit 2
    expect(quit.criteria.join(' ')).toContain('白屏');
    expect(quit.criteria.join(' ')).toContain('#1860'); // quit 前先清点 IDE 进程（0 个 ⇒ 不 quit）
    // quit 在 close 之前：close 只关项目窗口，清残留 IDE 实例必须先 quit
    expect(realPlan.steps.map((s) => s.id).indexOf('devtools-quit'))
      .toBeLessThan(realPlan.steps.map((s) => s.id).indexOf('devtools-close'));
  });

  it('C26：publish 前秒级预探在计划里是第一步（#1860：否定性闸门，只拦楔死态）', () => {
    const pre = realPlan.steps.find((s) => s.id === 'env-preprobe');
    expect(pre.exec).toBe('none'); // 只读：不发命令、不操控 IDE
    expect(pre.argv).toEqual([]);
    const criteria = pre.criteria.join(' ');
    expect(criteria).toContain('否定性'); // 只拦楔死，不肯定性放行
    expect(criteria).toContain('ide-port-unreachable');
    // #2147：进程数 0 的陈旧端口文件是**放行**的第三态，必须写进计划判据（计划是判据的唯一真源；
    // 只写在判据库注释里 = 下一个人读计划时看不到「门为什么肯冷起点」）
    expect(criteria).toContain('stale-port-file');
    expect(pre.exit2).toEqual([{ on: 'unreachable', reason: 'ide-port-unreachable' }]);
    // exit 2 归属必须进 envReasons 白名单（结果行 reason= 的取值域）
    expect(realPlan.branches.envBlocked.envReasons).toContain('ide-port-unreachable');
    // 预算在计划里（timeouts 是唯一真源）
    expect(realPlan.timeouts.preprobeMs).toBeGreaterThan(0);
  });

  it('C26-wiring：门脚本真的调预探判据库（dot-source + 调用 + 计划预算取用），-Doctor 跳过', () => {
    const code = stripCommentLines(real.script).join('\n');
    expect(code).toContain("lib\\mp-weixin-preprobe.ps1'");
    expect(code).toContain('Test-MpWeixinDevToolsSessionReachable');
    expect(code).toContain("timeouts']['preprobeMs");
    expect(code).toContain('if (-not $Doctor) {');
    // 否定性闸门的 exit 2 走结果行构造器（形状由 mp-weixin-env.ps1 单点保证，不裸拼字符串）
    const body = code.slice(code.indexOf('Test-MpWeixinDevToolsSessionReachable'));
    expect(body).toContain("New-MpWeixinEnvResultLine -Prefix $($script:GatePlan.resultLine.prefix) -Reason 'ide-port-unreachable'");
    // #2147：进程维度这一支必须**真被门消费** —— 判据库数了进程、门里却不看，等于白写（且不可见）
    expect(body).toContain('$pre.StalePortFile');
    // 楔死早退路径跳过 finally 的 close：close 对死端口 .ide 会自启 IDE 再关（#1860 要消的自启族），
    // 且本路径从未创建自动化会话、无残留可收 —— 收尾哨兵必须真的被 close 函数消费。
    expect(code).toMatch(/\$script:skipFinalClose = \$true/);
    expect(psFunctionBody(code, 'function Invoke-DevToolsClose')).toContain('if ($script:skipFinalClose) { return }');
  });

  it('C27：产物内容指纹自动 -SkipBuild（#1860：三组件全等才命中，fail 向重建）', () => {
    const code = stripCommentLines(real.script).join('\n');
    expect(code).toContain("lib\\mp-weixin-build-fingerprint.ps1'");
    expect(code).toContain('Test-MpWeixinBuildFingerprintHit');
    expect(code).toContain('Save-MpWeixinBuildFingerprint');
    // 命中 ⇒ 翻 $SkipBuild 且重建计划（mode.skipBuild 是计划面字段，翻值不重建 = 计划与执行脱节）
    expect(code).toMatch(/\$SkipBuild = \$true\s*\$script:GatePlan = New-ThisGatePlan -UsePort \$Port/s);
    // 落盘只在「本轮真构建过」的分支（-SkipBuild 下不重写指纹）
    expect(code).toMatch(/if \(-not \$SkipBuild\) \{\s*try \{\s*\$fpSave = Save-MpWeixinBuildFingerprint/s);
    // 判据库的行为守护存在（三组件语义在 lib 测试里真执行，不在契约里抄第二份）
    expect(fs.existsSync(path.join(ROOT, 'utils/mpWeixinBuildFingerprintBehavior.test.js'))).toBe(true);
    expect(fs.existsSync(path.join(ROOT, 'utils/mpWeixinGatePreprobeBehavior.test.js'))).toBe(true);
  });

  it('C22：执行侧真的跑 pre-flight（取计划步骤 + 进程归零轮询 + -Doctor 跳过 + 绝不 kill）', () => {
    const s = real.script;
    expect(s).toContain('function Invoke-DevToolsPreflightQuit');
    expect(s).toContain("-Id 'devtools-quit'");
    expect(s).toContain("ProcessName -match 'wechatdevtools|微信开发者工具'");
    expect(s).toContain('if (-not $Doctor) { Invoke-DevToolsPreflightQuit }');
    expect(stripCommentLines(s).join('\n')).not.toMatch(/Stop-Process\s+-/);
  });

  it('C10：注册与文档面（package.json / ADR-0008 / PR 模板 / 移动端 AGENTS.md）', () => {
    expect(real.pkg).toMatch(/"build:mp-weixin-check"\s*:\s*"[^"]*scripts\/mp-weixin-check\.ps1"/);
    expect(real.adr).toContain('半自动');
    expect(real.adr).toContain('#883');
    expect(real.adr).toContain('mp-weixin-check.ps1');
    expect(real.template).toContain('跑于');
    expect(real.agents).toContain('② 微信开发者工具门 = 半自动门');
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

/**
 * C20（#1210）：② 门的**产物合法性**判据面。
 *
 * 票面病根：旧实现用**正则**从 `project.config.json` 抠 appid ⇒ **非法 JSON 也能通过 appid 校验**；
 * 这类坏产物的症状（白屏 / `pageStack` 永不应答 / IDE 日志反复 `routeTo appLaunch timeout`）在门上
 * 表现为**看起来像环境问题**的 `page-stack-not-answering`，把真正的病根藏起来。
 *
 * 本节的成对取证：
 *   · 必不红 = 合法产物夹具 ⇒ `ok=true` 且 appid 由**解析后取值**；
 *   · 必红 = 三条**各自定向**的坏产物（非法 JSON / 缺必需文件 / 缺页 `.js`）⇒ 各自命中对应的 `stage`。
 *
 * ⚠️ 判据库是**真执行**的（`pwsh` 跑 `scripts/lib/mp-weixin-product.ps1`），不是断言源码文本 ——
 * 文本断言在「判据被摘掉、只留一句字面量」时照样绿。接线面（门脚本真的调它了 / 旧正则真的没了）
 * 另由下面两条 wiring 用例守。
 */
describe('C20：产物合法性判据（#1210）', () => {
  const LIB_REL = path.join('scripts', 'lib', 'mp-weixin-product.ps1');

  /** 跑真判据库取报告（fail-closed：拿不到 pwsh 或输出不可解析即抛） */
  function runProductReport(dist) {
    const lib = path.join(ROOT, LIB_REL);
    const cmd = `. '${lib}'; (Get-MpWeixinProductReport -Dist '${dist}') | ConvertTo-Json -Compress`;
    const args = ['-NoProfile', '-NonInteractive'];
    if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass');
    args.push('-Command', cmd);
    let stdout = '';
    try {
      stdout = execFileSync(powershellExe(), args, {
        encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 4 * 1024 * 1024
      });
    } catch (e) {
      throw new Error(`产物判据库执行失败：${cmd}\n exit=${e.status}\n stdout=${e.stdout}\n stderr=${e.stderr}`);
    }
    const line = String(stdout).split(/\r?\n/).filter((l) => l.trim().startsWith('{')).pop();
    if (!line) throw new Error(`产物判据库没有输出 JSON：${stdout}`);
    return JSON.parse(line.trim());
  }

  /** 造一份产物夹具：{ 坏在哪儿 } ⇒ 临时目录 */
  function makeProduct(mutate) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-product-'));
    fs.writeFileSync(path.join(dir, 'project.config.json'), JSON.stringify({ appid: 'wxTESTAPPID' }));
    fs.writeFileSync(path.join(dir, 'app.json'), JSON.stringify({ pages: ['pages/index/index'] }));
    fs.writeFileSync(path.join(dir, 'app.js'), 'App({})');
    fs.writeFileSync(path.join(dir, 'app.wxss'), '/* wxss */');
    fs.mkdirSync(path.join(dir, 'pages', 'index'), { recursive: true });
    fs.writeFileSync(path.join(dir, 'pages', 'index', 'index.js'), 'Page({})');
    if (mutate) mutate(dir);
    return dir;
  }

  const dirs = [];
  const product = (mutate) => { const d = makeProduct(mutate); dirs.push(d); return d; };
  afterAll(() => { dirs.forEach((d) => { try { fs.rmSync(d, { recursive: true, force: true }); } catch (e) {} }); });

  it('C20a（必不红）：合法产物 ⇒ ok=true，appid 由解析后取值（不是从坏文件里抠）', () => {
    const r = runProductReport(product(null));
    expect(r.ok).toBe(true);
    expect(r.stage).toBe('product-ok');
    expect(r.appid).toBe('wxTESTAPPID');
    expect(r.pages).toBe(1);
  });

  it('C20b（必红①）：project.config.json 非法 JSON ⇒ fail-closed，且带出**位置原文**', () => {
    // 夹具即票面形态：根对象正常闭合，**后面多出一段游离片段**
    const dir = product((d) => {
      fs.writeFileSync(path.join(d, 'project.config.json'), '{\n  "appid": "wxTESTAPPID"\n}\n  "miniprogram": {\n    "current": -1\n  }\n');
    });
    const r = runProductReport(dir);
    expect(r.ok).toBe(false);
    expect(r.stage).toBe('product-json');
    expect(r.detail).toContain('project.config.json');
    // 位置原文（System.Text.Json 的 `line N, position M`）必须原样带出 —— 这是「让人一眼看到病根」那条
    expect(r.detail).toMatch(/line \d+/);
    expect(r.detail).toMatch(/position \d+/);
  });

  it('C20c（必红②）：缺必需文件（app.wxss）⇒ 点名缺了哪个', () => {
    const dir = product((d) => fs.rmSync(path.join(d, 'app.wxss')));
    const r = runProductReport(dir);
    expect(r.ok).toBe(false);
    expect(r.stage).toBe('product-missing');
    expect(r.detail).toContain('app.wxss');
  });

  it('C20d（必红③）：app.json.pages 声明的页面缺 .js ⇒ 点名缺哪个 .js', () => {
    const dir = product((d) => {
      fs.writeFileSync(path.join(d, 'app.json'), JSON.stringify({ pages: ['pages/index/index', 'pages/login/login'] }));
    });
    const r = runProductReport(dir);
    expect(r.ok).toBe(false);
    expect(r.stage).toBe('product-pages');
    expect(r.detail).toContain('pages/login/login.js');
  });

  it('C20e（必红④）：app.json.pages 路由重复 ⇒ 点名重复项（#1690 run7-10：重复页让构建器拒启模拟器，症状伪装成 page-stack-not-answering）', () => {
    // 夹具把每个声明页的 .js 都补齐 ⇒ 判据面唯一可能的失败分支就是「重复」。
    // （中文 detail 经子进程 stdout 会被 OEM 码页打成乱码，负向断言不牢靠 —— 结构上排除歧义，断言只用 ASCII）
    const dir = product((d) => {
      fs.writeFileSync(path.join(d, 'app.json'), JSON.stringify({ pages: ['pages/index/index', 'pages/login/login', 'pages/index/index'] }));
      fs.mkdirSync(path.join(d, 'pages', 'login'), { recursive: true });
      fs.writeFileSync(path.join(d, 'pages', 'login', 'login.js'), 'Page({})');
    });
    const r = runProductReport(dir);
    expect(r.ok).toBe(false);
    expect(r.stage).toBe('product-pages');
    expect(r.detail).toContain('pages/index/index');
    expect(r.pages).toBe(3);
  });

  it('C20-wiring①：门脚本真的调判据库（dot-source + 调用），且旧正则抠 appid 已删', () => {
    const s = readSource(SCRIPT_REL);
    expect(s).toContain(`'lib\\mp-weixin-product.ps1'`);
    expect(s).toContain('Get-MpWeixinProductReport -Dist $dist');
    // 旧写法（正则从产物里抠 appid）不得复活；判据只在**代码行**上判，避免注释引例自命中
    const code = stripCommentLines(s).join('\n');
    expect(code).not.toContain('$projConfig = Join-Path $dist');
    expect(code).not.toContain('[regex]::Match((Get-Content -LiteralPath $projConfig');
    // 产物坏了 ⇒ exit 1（不是 exit 2：与「环境不可用」分开）
    const idx = code.indexOf('if (-not $product.ok) {');
    expect(idx).toBeGreaterThan(-1);
    const block = code.slice(idx, code.indexOf('\n    }', idx));
    expect(block).toContain('exit 1');
    expect(block).not.toContain('exit 2');
  });

  it('C20-wiring②：ADR 写明产物合法性判据，模板写明 ④b 的去向（决策回写纪律）', () => {
    // A1（#2100）后 ④b 不判红、正文不写 ④b 行，只在 notes 里留提醒。
    // 所以「模板必须复述产物合法性判据」这条断言已过期 —— 但「这件事在文档里没消失」仍要守：
    // ADR 侧保留判据原文（口径的归档位置），模板侧保留 ④b 的去向说明（写的人知道它去哪儿了）。
    expect(readSource(ADR_REL)).toContain('产物合法性');
    expect(readSource(PR_TEMPLATE_REL)).toContain('④b');
  });
});
