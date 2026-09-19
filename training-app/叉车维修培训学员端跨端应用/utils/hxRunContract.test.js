/**
 * 「日常增量运行」契约守护（scripts/hx-run.ps1 + 三层节奏文档）
 *
 * 背景（2026-09-12）：移动端 UI 迭代的痛点不是编译慢，而是**每次微调都走全量那一圈**——
 * 全量编译实测 4.8–8 分钟（`compile-check.ps1` 就是这条门，默认值不动），增量约 1–2 分钟；
 * 资源导出 publish 98 秒–3 分钟；基座 APK 95.7 MB **只需装一次**。于是新增 `scripts/hx-run.ps1`
 * 承担**日常增量**：默认无干净缓存重建、不重装基座、忙就等、绝不 kill。
 * 这个脚本的全部风险在于**被后续会话"顺手优化"成全量**，或**为了"清干净"去 kill 主程序**
 * （违反 ADR-0008 的「HBuilderX 是单实例串行资源」坑位），所以用本测试把这几条锁死：
 *   C1 默认增量：干净缓存重建开关**只允许**出现在文件唯一的 `if ($Full)` 块里；`-Full` 是显式开关
 *   C2 不得出现任何强杀调用（Stop-Process / taskkill / 进程对象 .Kill()）——共享主程序绝不抢占
 *   C3 必须 dot-source `scripts/lib/hx-busy.ps1`、调用 `Wait-HxFree`，且锁在 `finally` 里释放
 *   C4 忙等待上限是 `-WaitSeconds`（默认 600），环境不可用一律 `exit 2`
 *   C5 必须输出机检行 `HX_RUN mode=… compile=… deploy=… total=… exit=…`，且三种结论都有出口
 *   C6 判成败只解析 stdout：不得出现 `$LASTEXITCODE` / `ExitCode`（CLI 退出码恒 0）
 *   C7 `-DryRun` 存在、在任何 HBuilderX 交互之前 exit 0，且该分支内不派发 cli / 不做忙检测 / 不启进程；
 *      真正派发 cli（`Invoke-CliStep -Name`）必须在忙检测（`Wait-HxFree -CliExe`）之后
 *   C8 `-Device` 缺/多 ⇒ exit 2、日志默认 `.ci-verify/hx-run.log`、脚本**不做** adb 装基座
 *   C9 头注释写明与 `compile-check.ps1` 的分工（门 = 全量，本脚本 = 日常增量）与「绝不 kill 主程序」
 *   C10 文档落锁：移动端 `AGENTS.md`「开发内循环（移动端 UI 迭代）」三层节奏 + 反模式、
 *       ADR-0008 非门辅助段的一行指针、`package.json` 注册 `hx:run`
 *   C11 **部署后置断言**（2026-09-12 假绿教训 v1 → 2026-09-13 v2 升级）：必须有 `Get-DeviceDeployFacts` /
 *       `Get-DeployVerdict`，主判据是**基线相对的设备侧事实**（资源目录 mtime 相对基线前进，`stat -c %Y`），
 *       `topResumedActivity` 只作辅助说明（**曾单独作判据 ⇒ 重复运行到同一台机器时恒真 ⇒ v2 假绿**）；
 *       必须输出 `HX_RUN_DEPLOY deployed=… www=… pid_after=…` 机检行；`deployed=false` ⇒ `exit=env` + `exit 2`，
 *       且该判定必须出现在报成功（`-Exit 'ok'`）**之前**
 *   C12 **真运行语义**（2026-09-13 事故根因）：
 *       `launch app-android` 的 `--compile` 官方语义是「**仅编译代码**」⇒ 本脚本的 launch 参数里**不得出现** `--compile`
 *       （它只属于全量编译门 `compile-check.ps1`）；必须显式传 `--deviceId`；launch 步必须走
 *       `Start-CliLaunchDetached`（真运行会话**不会自己收口**，走会等待的路径必然假超时）；
 *       且 `AGENTS.md` / ADR-0008 都必须写明「仅编译」这条坑位
 *   C13 **`-CompileOnly` 诊断模式 + 快速失败**（2026-09-13，#949：慢的不是编译，是返工/排队/卡死）：
 *       必须有 `-CompileOnly` 开关，其参数**只在这个分支里**传 `--compile true`（官方语义的「仅编译」）、
 *       **不传 `--deviceId`**、并跳过设备解析（⇒ 不接设备的机器上也能拿到编译期诊断）；
 *       必须有部署停滞判据（`Test-HxCompileFinished` + `-DeployStallSeconds`）；
 *       忙等待默认 120（忙就快速 exit 2，不无声等十分钟）、轮询上限默认 900；
 *       `package.json` 注册 `hx:compile-only`；`AGENTS.md` 写明分层内循环
 *   C14 **最短部署观察窗**（2026-09-14，#974 假阴性）：部署判定必须有 `-DeployDeadlineSeconds`（默认 ≥60 秒）——
 *       设备侧事实一旦前进就立即 PASS；**未前进时必须等满这个窗**才允许因「会话退出 / 停滞 / 到顶」收手。
 *       依据：实测资源落盘比推送晚 **7–21 秒**，窗口比它短就会把「稍后落盘」误判成未部署
 *       （当日实测 3 秒就下结论 ⇒ 白重跑一轮约 7 分钟编译）。收手判定必须来自
 *       `scripts/lib/hx-deploy.ps1` 的纯函数 `Test-DeployObservationStop`（可被运行期断言），
 *       且**不得**退回「`$launch.Proc.HasExited` 一为真就 break」的旧形态
 *
 * 设计沿用本仓既有守护测试形态（见 utils/emulatorSmokeContract.test.js、utils/hxBusyGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 *
 * **CRLF 坑位**：`.gitattributes` 已把 `*.ps1` / `*.js` / `*.md` 钉成 LF；本文件里的锚点按 **LF** 写。
 * 若哪天有人把 `hx-run.ps1` 存成 CRLF，本测试的多行锚点会失配（这正是想要的：先红再改）。
 *
 * **首跑校准已发生（2026-09-12，照实记）**：`hx-run.ps1` 已真机首跑，标记表命中了 `编译成功`。
 * 而这次首跑暴露的**不是计时不准，而是假绿** —— HBuilderX 打出 `编译成功` → `ready in …` → `已停止运行...`，
 * **什么都没到达设备**（设备上目标包 `lastUpdateTime` 仍是旧日期），旧版脚本却仍报 `exit=ok` / 退出码 0，
 * 于是有人据它宣布「已编译并运行到设备」，把**旧构建的截图**当成 ①a 取证入库（证据污染，已在对应 PR 撤回）。
 * 故新增 **C11**，把「**编译成功 ≠ 运行成功**」这条锁死。
 */

const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/hx-run.ps1';
const DEPLOY_LIB_REL = 'scripts/lib/hx-deploy.ps1';
const AGENTS_REL = 'AGENTS.md';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
const PKG_REL = 'package.json';

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/**
 * 掩掉 `<# ... #>` 文档块（块内字符替换成空格，偏移/行号不变）。
 * 头注释本来就允许出现 `Stop-Process`、`if ($Full)`、`adb install` 这类**叙述性**字面量
 * （它们在解释规则本身），所以涉及「代码里不许有」的规则只查掩码后的正文。
 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 纯函数：源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };
  const raw = sources.script || '';
  const code = maskDocBlocks(raw);
  const agents = sources.agents || '';
  const adr = sources.adr || '';
  const pkg = sources.pkg || '';
  const lib = sources.deployLib || '';

  // ---- C1 默认增量：干净缓存重建开关只允许出现在唯一的 if ($Full) 块里 ----
  const blocks = code.match(/if\s*\(\s*\$Full\s*\)\s*\{[\s\S]*?\n\}/g) || [];
  must(blocks.length === 1, 'C1', `应恰有 1 个 if ($Full) 块（实得 ${blocks.length} 个）——全量开关必须集中一处`);
  const totalSw = (code.match(/cleanCache/g) || []).length;
  const inBlockSw = blocks.reduce((n, b) => n + (b.match(/cleanCache/g) || []).length, 0);
  must(totalSw >= 1, 'C1', '未见干净缓存重建开关（-Full 分支必须给出，否则 -Full 是空开关）');
  must(totalSw === inBlockSw, 'C1', `干净缓存重建开关出现在 if ($Full) 块之外（全文 ${totalSw} 处，块内 ${inBlockSw} 处）⇒ 默认会变全量`);
  must(/\[switch\]\$Full/.test(code), 'C1', '缺 -Full 开关（全量必须显式给，不能默认）');
  must(/\$mode = 'incremental'/.test(code), 'C1', '默认 mode 不是 incremental');

  // ---- C2 绝不强杀（共享主程序绝不抢占）----
  ['Stop-Process', 'taskkill', 'pkill', 'kill.exe'].forEach((token) => {
    must(!code.includes(token), 'C2', `正文出现强杀调用字面量：${token}（HBuilderX 是单实例串行资源，忙就等）`);
  });
  must(!/\.Kill\s*\(/i.test(code), 'C2', '正文出现进程对象 .Kill() 调用（不得强杀任何进程）');

  // ---- C3 忙检测 helper + 锁 ----
  must(/lib[\\/]hx-busy\.ps1/.test(code), 'C3', '未 dot-source scripts/lib/hx-busy.ps1');
  must(/Wait-HxFree\s+-CliExe/.test(code), 'C3', '未调用 Wait-HxFree（缺 agent 互斥锁 + 忙探测 + 等待上限）');
  const finallyAt = code.lastIndexOf('} finally {');
  must(finallyAt !== -1, 'C3', '缺 finally 路径（锁必须在那里释放）');
  if (finallyAt !== -1) {
    must(/Release-HxLock/.test(code.slice(finallyAt)), 'C3', 'finally 路径里没有 Release-HxLock');
  }

  // ---- C4 忙/超时 = exit 2 ----
  must(/\[int\]\$WaitSeconds\s*=\s*120/.test(code), 'C4', '-WaitSeconds 默认不是 120（#949 要求：忙就快速 exit 2，不无声等十分钟）');
  const exit2 = (code.match(/exit 2/g) || []).length;
  must(exit2 >= 3, 'C4', `环境不可用路径未判 exit 2（应 ≥3 处：cli/adb 缺失、设备缺或多、连接中断或超时；实得 ${exit2} 处）`);

  // ---- C5 机检行 ----
  must(/HX_RUN mode=/.test(code), 'C5', '缺 HX_RUN mode= 机检行');
  must(
    /HX_RUN mode=\{0\} compile=\{1\} deploy=\{2\} total=\{3\} exit=\{4\}/.test(code),
    'C5',
    '机检行格式不符（应为 HX_RUN mode=… compile=… deploy=… total=… exit=…）'
  );
  ["-Exit 'ok'", "-Exit 'fail'", "-Exit 'env'"].forEach((token) => {
    must(code.includes(token), 'C5', `缺结论出口 ${token}（判成败的每条路径都要打机检行）`);
  });

  // ---- C6 不看退出码 ----
  must(!/\$LASTEXITCODE/.test(code), 'C6', '出现 $LASTEXITCODE：CLI 退出码恒 0，判成败只解析 stdout');
  must(!/ExitCode/.test(code), 'C6', '出现 ExitCode：CLI 退出码恒 0，判成败只解析 stdout');
  must(/\$errorLines/.test(code), 'C6', '缺 stdout 判据（error 行扫描）');

  // ---- C7 -DryRun 零副作用 + 派发顺序 ----
  must(/\[switch\]\$DryRun/.test(code), 'C7', '缺 -DryRun 开关（唯一可端到端验证的路径）');
  const dryAt = code.indexOf('if ($DryRun) {');
  must(dryAt !== -1, 'C7', '未找到 `if ($DryRun) {` 分支');
  const dryExitAt = dryAt === -1 ? -1 : code.indexOf('exit 0', dryAt);
  must(dryExitAt > dryAt, 'C7', '-DryRun 分支没有在自己那段里 exit 0');
  if (dryAt !== -1 && dryExitAt > dryAt) {
    const drySlice = code.slice(dryAt, dryExitAt);
    must(!drySlice.includes('Invoke-CliStep'), 'C7', '-DryRun 分支里派发了 cli（Invoke-CliStep）');
    must(!/Wait-HxFree\s+-CliExe/.test(drySlice), 'C7', '-DryRun 分支里做了忙检测（会碰主程序）');
    must(!drySlice.includes('Start-Process'), 'C7', '-DryRun 分支里启动了进程');
    must(!/Resolve-TargetDevice|Resolve-AdbExe/.test(drySlice), 'C7', '-DryRun 分支里探测了设备（会调 adb）');
  }
  const waitCallAt = code.indexOf('Wait-HxFree -CliExe');
  const firstDispatchAt = code.indexOf('Invoke-CliStep -Name');
  must(dryAt !== -1 && waitCallAt !== -1 && dryAt < waitCallAt, 'C7', '-DryRun 的早退不在忙检测之前 ⇒ 计划输出也可能碰到主程序');
  must(waitCallAt !== -1 && firstDispatchAt !== -1 && waitCallAt < firstDispatchAt, 'C7', '派发 cli 早于忙检测（Wait-HxFree 必须在 Invoke-CliStep -Name 之前）');

  // ---- C8 设备前置判定 / 日志 / 不自装基座 ----
  must(/\[string\]\$Device/.test(code), 'C8', '缺 -Device 参数');
  // 「多设备必须显式指定」锚在**守卫代码**上（只查散文会让"把守卫删了但留着提示"蒙混过关）
  must(
    /if\s*\(\s*\$online\.Count\s*-gt\s*1\s*\)\s*\{[\s\S]{0,500}?exit 2/.test(code),
    'C8',
    '缺「多设备未显式指定 ⇒ exit 2」的守卫（$online.Count -gt 1 分支必须 exit 2）'
  );
  must(/\$online\.Count\s*-eq\s*0/.test(code), 'C8', '缺「无在线设备 ⇒ exit 2」的守卫（$online.Count -eq 0 分支）');
  must(/hx-run\.log/.test(code), 'C8', '日志默认路径不是 .ci-verify/hx-run.log');
  must(/\.ci-verify/.test(code), 'C8', '日志未落 .ci-verify/（该目录已在 .gitignore）');
  must(!/install/i.test(code), 'C8', '正文出现 install：脚本不得自装基座（95.7 MB 只装一次，装机交给 HBuilderX）');

  // ---- C9 头注释的分工与纪律（查原始文本：这些声明写在文档块里）----
  must(raw.includes('compile-check.ps1'), 'C9', '头注释未写明与 compile-check.ps1 的分工');
  must(raw.includes('门') && raw.includes('全量'), 'C9', '头注释未写明「门 = 全量」');
  must(raw.includes('日常增量'), 'C9', '头注释未写明本脚本 = 日常增量');
  must(raw.includes('绝不 kill'), 'C9', '头注释未写明「绝不 kill 主程序」');
  must(raw.includes('scripts/lib/hx-busy.ps1'), 'C9', '头注释未指向 scripts/lib/hx-busy.ps1');
  must(raw.includes('不是门'), 'C9', '头注释未声明本脚本不是门（不进验收证据）');

  // ---- C11 部署后置断言：编译成功 ≠ 运行成功（v1 2026-09-12；判据必须**基线相对**，v2 2026-09-13）----
  must(/function\s+Get-DeviceDeployFacts/.test(code), 'C11', '缺设备侧取数函数 Get-DeviceDeployFacts（只扫 error 行会报出假绿）');
  must(/function\s+Get-DeployVerdict/.test(code), 'C11', '缺部署判定函数 Get-DeployVerdict');
  must(/stat -c %Y/.test(code), 'C11', '未取「资源真落盘」这个设备侧事实（应有 stat -c %Y 取资源目录 mtime）');
  must(/\$deployed = Get-DeployVerdict/.test(code), 'C11', '判定结果不是来自 Get-DeployVerdict（写成常量就是假绿）');
  must(
    /-Before \$factsBefore -After \$factsAfter/.test(code),
    'C11',
    '判定不是**基线相对**的（缺 -Before/-After 比对）—— 没有基线时恒真的判据会报出 v2 假绿'
  );
  must(/HX_RUN_DEPLOY deployed=/.test(code), 'C11', '缺 HX_RUN_DEPLOY 机检行（「到底有没有到设备」必须可机检）');
  must(
    /www=\$\(Format-WwwPair -Before \$factsBefore -After \$factsAfter\)/.test(code) && /pid_after=/.test(code),
    'C11',
    '机检行未同时报基线与现值（应为 … www=$(Format-WwwPair -Before $factsBefore -After $factsAfter) … pid_after=…）'
  );
  must(/已停止运行/.test(code), 'C11', '未纳入实测停止标记「已停止运行」（2026-09-12 真机首跑的校准点）');
  must(/io\.dcloud\.uniappx/.test(code), 'C11', '候选包名缺 HBuilderX 标准基座（dev 运行的常见承载）');
  must(/topResumedActivity/.test(code), 'C11', '未取前台包名（取它只为辅助说明；**不得单独作判据**）');
  const verdictAt = code.indexOf('if (-not $deployed.Deployed)');
  must(verdictAt !== -1, 'C11', '缺「未部署」分支（必须显式判未部署，不能只报成功）');
  const okAt = verdictAt === -1 ? -1 : code.indexOf("-Exit 'ok'", verdictAt);
  if (verdictAt !== -1) {
    const verdictSlice = code.slice(verdictAt, verdictAt + 1600);
    must(/-Exit 'env'/.test(verdictSlice), 'C11', '未部署时未打 exit=env 的结果行（不得报 ok）');
    must(/exit 2/.test(verdictSlice), 'C11', '未部署时未 exit 2（以 0 退出就是假绿）');
    must(okAt !== -1 && verdictAt < okAt, 'C11', '部署后置断言必须在报成功（-Exit \'ok\'）之前 —— 否则成功先被报出去了');
  }

  // ---- C12 真运行语义（2026-09-13 事故根因：--compile true 的官方语义是「仅编译代码」）----
  const buildAt = code.indexOf("$launchArgs = @('launch', 'app-android'");
  must(buildAt !== -1, 'C12', '未找到 launch 参数构造行（无法核对真运行语义）');
  // 「真运行路径」= 从参数构造行到 -CompileOnly 分支之前（#949 加了那只分支，它**该**传 --compile）
  const coStart = buildAt === -1 ? -1 : code.indexOf('if ($CompileOnly) {', buildAt);
  must(coStart !== -1 && coStart > buildAt, 'C12', '未找到 -CompileOnly 分支（无法把真运行参数与仅编译参数分开核对）');
  if (buildAt !== -1 && coStart > buildAt) {
    const normalSlice = code.slice(buildAt, coStart);
    must(
      !/--compile/.test(normalSlice),
      'C12',
      '真运行路径的 launch 参数里出现 --compile（官方语义是「仅编译代码」⇒ 只编译不运行；它只属于 -CompileOnly 分支与全量编译门）'
    );
  }
  must(
    /--cleanCache/.test(code.slice(buildAt, buildAt + 1200)),
    'C12',
    'launch 参数构造处未见 -Full 分支的干净缓存重建开关（C1 的落点应在这里）'
  );
  must(/--deviceId/.test(code), 'C12', 'launch 未显式传 --deviceId（官方文档：不指定时默认使用第一个设备；显式传才无歧义）');
  must(/Start-CliLaunchDetached/.test(code), 'C12', 'launch 步未走「派发后不等待」路径（真运行会话不返回，等待必然假超时）');
  must(
    !/Invoke-CliStep -Name 'launch'/.test(code),
    'C12',
    'launch 步仍在用会等待收口的 Invoke-CliStep（真运行会话不返回 ⇒ 必然假超时）'
  );
  must(/\[int\]\$StepTimeoutSeconds/.test(code), 'C12', '缺 -StepTimeoutSeconds（open / project-open 两小步的超时）');
  must(agents.includes('仅编译'), 'C12', 'AGENTS.md 未写明「--compile 的语义是仅编译」（下个会话会再踩同一个坑）');
  must(adr.includes('仅编译'), 'C12', 'ADR-0008 未写明「仅编译」坑位（ADR 才是冷启动会话的必读面）');

  // ---- C13 -CompileOnly 诊断模式 + 快速失败（2026-09-13，#949）----
  must(/\[switch\]\$CompileOnly/.test(code), 'C13', '缺 -CompileOnly 开关（编译期诊断必须在真机链路之前能拿到）');
  const coAt = code.indexOf('if ($CompileOnly) {');
  must(coAt !== -1, 'C13', '未找到 `if ($CompileOnly) {` 分支（无法核对「仅编译」参数的落点）');
  if (coAt !== -1) {
    const coSlice = code.slice(coAt, coAt + 700);
    must(/--compile/.test(coSlice), 'C13', '-CompileOnly 分支未传 --compile（那才是官方语义的「仅编译代码」）');
    must(!/--deviceId/.test(coSlice), 'C13', '-CompileOnly 分支不该传 --deviceId（仅编译不需要设备）');
  }
  must(
    /\$target = @\{ Serial = ''; Mode = 'compile-only' \}/.test(code),
    'C13',
    '-CompileOnly 未跳过设备解析（会让「不接设备也能跑诊断」落空）'
  );
  must(/Test-HxCompileFinished/.test(code), 'C13', '缺部署停滞判据（编译段已结束 + 无前进 ⇒ 提前判环境不可用）');
  // ⚠️ 每个 Get-HxErrorLines 调用点都必须包 @(...)：函数返回空数组会退化成 $null，
  //    而 `$null.Count` 在 Set-StrictMode -Latest 下直接抛错（2026-09-13 首次真跑踩到）
  const ghcCalls = (code.match(/Get-HxErrorLines -Output/g) || []).length;
  const ghcSafe = (code.match(/@\(Get-HxErrorLines -Output/g) || []).length;
  must(ghcCalls >= 2, 'C13', `Get-HxErrorLines 调用点应 ≥2（仅编译 + 真运行；实得 ${ghcCalls}）`);
  must(
    ghcCalls === ghcSafe,
    'C13',
    `有 Get-HxErrorLines 调用点没包 @()（${ghcSafe}/${ghcCalls}）—— 空数组退化成 $null 会让 StrictMode 抛错`
  );
  must(/\[int\]\$DeployStallSeconds\s*=\s*300/.test(code), 'C13', '缺 -DeployStallSeconds（默认 300）');
  must(/\[int\]\$TimeoutSeconds\s*=\s*900/.test(code), 'C13', '部署轮询上限默认不是 900（#949 要求由 1800 下调）');
  must(/"hx:compile-only"\s*:\s*"[^"]*hx-run\.ps1\s+-CompileOnly"/.test(pkg), 'C13', 'package.json 未注册 hx:compile-only');
  must(agents.includes('CompileOnly') || agents.includes('compile-only'), 'C13', 'AGENTS.md 未写分层内循环（诊断走 CompileOnly）');


  // ---- C10 文档落锁 ----
  must(agents.includes('## 开发内循环（移动端 UI 迭代）'), 'C10', 'AGENTS.md 缺「开发内循环（移动端 UI 迭代）」小节');
  ['内循环', '中循环', '外循环'].forEach((token) => {
    must(agents.includes(token), 'C10', `AGENTS.md 三层节奏缺「${token}」`);
  });
  must(agents.includes('hx-run.ps1'), 'C10', 'AGENTS.md 未指向 scripts/hx-run.ps1');
  must(agents.includes('npm run hx:run'), 'C10', 'AGENTS.md 未给出 npm run hx:run 入口');
  must(agents.includes('反模式'), 'C10', 'AGENTS.md 缺「反模式」清单');
  must(agents.includes('一个视觉主题一个 commit'), 'C10', 'AGENTS.md 未写「一个视觉主题一个 commit」的提交粒度');
  must(agents.includes('56d0fe3'), 'C10', 'AGENTS.md 未给出混提交的反例（56d0fe3）');
  must(agents.includes('不重装基座') || agents.includes('基座只装一次'), 'C10', 'AGENTS.md 未写「不重装基座」');
  must(adr.includes('开发内循环'), 'C10', 'ADR-0008 非门辅助段缺一行指针（应指向该小节与本脚本）');
  must(adr.includes('hx-run.ps1'), 'C10', 'ADR-0008 指针未指向 scripts/hx-run.ps1');
  must(/"hx:run"\s*:\s*"[^"]*scripts\/hx-run\.ps1"/.test(pkg), 'C10', 'package.json 未注册 hx:run → scripts/hx-run.ps1');

  // ---- C14 最短部署观察窗（#974 假阴性：落盘比推送晚 7–21 秒，窗口比它短就会误判未部署）----
  const dds = code.match(/\[int\]\$DeployDeadlineSeconds\s*=\s*(\d+)/);
  must(dds !== null, 'C14', '缺 -DeployDeadlineSeconds（最短部署观察窗；没有它「会话一退出 / 首采样无前进」就会直接判 env）');
  if (dds) {
    must(
      Number(dds[1]) >= 60,
      'C14',
      `-DeployDeadlineSeconds 默认 ${dds[1]} 秒 < 60 —— 实测资源落盘要 7–21 秒，窗口更短会把「稍后落盘」误判成未部署`
    );
  }
  must(/lib[\\/]hx-deploy\.ps1/.test(code), 'C14', '未 dot-source scripts/lib/hx-deploy.ps1（收手判定必须来自可做运行期断言的纯函数）');
  must(/Test-DeployObservationStop\s+-Deployed/.test(code), 'C14', '主循环未调用 Test-DeployObservationStop（收手判定没接线）');
  must(
    code.includes('-DeployDeadlineSeconds $DeployDeadlineSeconds'),
    'C14',
    '未把 -DeployDeadlineSeconds 透传给收手判定（参数形同虚设）'
  );
  must(
    !/\$launch\.Proc\.HasExited\)\s*\{\s*\$launchExitedEarly\s*=\s*\$true;\s*break\s*\}/.test(code),
    'C14',
    '收手判定退回「会话一退出就 break」—— 资源落盘比推送晚 7–21 秒，那会把「稍后落盘」误判成未部署（#974 的原始缺陷）'
  );
  // 纯函数自身的不变量：① 前进即 PASS；② 「会话退出」必须被最短观察窗挡住，否则窗口形同虚设
  must(lib.includes('function Test-DeployObservationStop'), 'C14', DEPLOY_LIB_REL + ' 缺 Test-DeployObservationStop');
  // ⚠️ 锚点刻意取**代码形状**（`Stop = $true; Outcome = 'advanced'`），不取裸的 `Outcome = 'advanced'`：
  //    后者在纯函数的注释（返回值说明）里也出现 ⇒ 守卫会被注释满足，删掉真正的代码行也不报（本仓踩过多次）
  must(
    lib.includes("Stop = $true; Outcome = 'advanced'"),
    'C14',
    DEPLOY_LIB_REL + ' 缺「一旦前进就 PASS」的收手代码（advanced 收手分支）'
  );
  must(
    /\$windowElapsed\s*-and\s*\$Exited/.test(lib),
    'C14',
    DEPLOY_LIB_REL + ' 的「会话退出」收手未被最短观察窗挡住（等于没加窗口）'
  );

  return violations;
}

describe('日常增量运行契约（scripts/hx-run.ps1，2026-09-12）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    agents: readSource(AGENTS_REL),
    adr: readSource(ADR_REL),
    pkg: readSource(PKG_REL),
    deployLib: readSource(DEPLOY_LIB_REL)
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['C1', '干净缓存开关被挪出 -Full 块', (s) => ({
        ...s,
        script: s.script + "\n$launchArgs += @('--cleanCache', 'true')\n"
      })],
      ['C1', '-Full 块被改名（全量变成不可控）', (s) => ({
        ...s,
        script: s.script.replace('if ($Full) {', 'if ($true) {')
      })],
      ['C1', '-Full 开关被删', (s) => ({ ...s, script: s.script.replace('[switch]$Full,', '') })],
      ['C2', '混入强杀主程序', (s) => ({ ...s, script: s.script + '\nStop-Process -Name HBuilderX -Force\n' })],
      ['C2', '混入 taskkill', (s) => ({ ...s, script: s.script + '\ntaskkill /IM HBuilderX.exe /F\n' })],
      ['C2', '混入进程对象 .Kill()', (s) => ({ ...s, script: s.script + '\n$proc.Kill()\n' })],
      ['C3', 'dot-source 被删', (s) => ({ ...s, script: s.script.replace(/lib\\hx-busy\.ps1/g, 'other.ps1') })],
      ['C3', 'Wait-HxFree 被删', (s) => ({ ...s, script: s.script.replace(/Wait-HxFree -CliExe/g, 'WaitNothing -CliExe') })],
      ['C3', '锁没在 finally 里释放', (s) => ({ ...s, script: s.script.replace(/Release-HxLock/g, 'ReleaseNothing') })],
      ['C4', '忙等待上限被改大', (s) => ({ ...s, script: s.script.replace('[int]$WaitSeconds = 120', '[int]$WaitSeconds = 3600') })],
      ['C4', '环境不可用不再 exit 2', (s) => ({ ...s, script: s.script.replace(/exit 2/g, 'exit 9') })],
      ['C5', '机检行格式被改', (s) => ({
        ...s,
        script: s.script.replace('HX_RUN mode={0} compile={1}', 'HX_RUN compile={1}')
      })],
      ['C5', 'fail 出口被删', (s) => ({ ...s, script: s.script.replace(/-Exit 'fail'/g, "-Exit 'bad'") })],
      ['C6', '拿退出码当判据', (s) => ({ ...s, script: s.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' })],
      ['C6', '改用 ExitCode 判成败', (s) => ({ ...s, script: s.script + '\nif ($proc.ExitCode -ne 0) { exit 1 }\n' })],
      ['C7', '-DryRun 分支被改名', (s) => ({ ...s, script: s.script.replace('if ($DryRun) {', 'if ($false) {') })],
      ['C7', '-DryRun 分支里派发 cli', (s) => ({
        ...s,
        script: s.script.replace('if ($DryRun) {', "if ($DryRun) {\n    Invoke-CliStep -Name 'open'\n")
      })],
      ['C8', '多设备守卫被删', (s) => ({ ...s, script: s.script.replace('if ($online.Count -gt 1) {', 'if ($false) {') })],
      ['C8', '无设备守卫被删', (s) => ({ ...s, script: s.script.replace('if ($online.Count -eq 0) {', 'if ($false) {') })],
      ['C8', '日志改名', (s) => ({ ...s, script: s.script.replace(/hx-run\.log/g, 'run.log') })],
      ['C8', '混入自装基座', (s) => ({ ...s, script: s.script + '\n& $adbExe install -r base.apk\n' })],
      ['C9', '头注释不再声明绝不 kill', (s) => ({ ...s, script: s.script.replace(/绝不 kill/g, '尽量避免 kill') })],
      ['C9', '头注释不再提 compile-check 分工', (s) => ({
        ...s,
        script: s.script.replace(/compile-check\.ps1/g, 'some-gate.ps1')
      })],
      ['C10', 'AGENTS.md 小节被删', (s) => ({
        ...s,
        agents: s.agents.replace('## 开发内循环（移动端 UI 迭代）', '## 开发循环')
      })],
      ['C10', 'AGENTS.md 反模式清单被删', (s) => ({ ...s, agents: s.agents.replace(/反模式/g, '注意事项') })],
      ['C10', 'ADR 指针被删', (s) => ({ ...s, adr: s.adr.replace(/hx-run\.ps1/g, 'other.ps1') })],
      ['C10', 'package.json 未注册', (s) => ({ ...s, pkg: s.pkg.replace('"hx:run"', '"hxrun"') })],
      ['C11', '取数函数被删（退回只看 error 行）', (s) => ({
        ...s,
        script: s.script.replace(/function Get-DeviceDeployFacts/g, 'function X')
      })],
      ['C11', '判定函数被删', (s) => ({
        ...s,
        script: s.script.replace(/function Get-DeployVerdict/g, 'function Y')
      })],
      ['C11', '资源 mtime 取数被删（不再看「资源真落盘」）', (s) => ({
        ...s,
        script: s.script.replace(/stat -c %Y/g, 'cat')
      })],
      ['C11', '判定被写成常量（不再来自判定函数）', (s) => ({
        ...s,
        script: s.script.replace('$deployed = Get-DeployVerdict', '$deployed = @{ Deployed = $true }')
      })],
      ['C11', '判据不再与基线比对（退回恒真）', (s) => ({
        ...s,
        script: s.script
          .replace(/-Before \$factsBefore -After \$factsAfter/g, '-Before $factsAfter -After $factsAfter')
      })],
      ['C11', 'HX_RUN_DEPLOY 机检行被删', (s) => ({
        ...s,
        script: s.script.replace(/HX_RUN_DEPLOY deployed=/g, 'X')
      })],
      ['C11', '机检行不再报基线（只报现值）', (s) => ({
        ...s,
        script: s.script.replace('www=$(Format-WwwPair -Before $factsBefore -After $factsAfter)', 'www=none')
      })],
      ['C11', '不再查设备侧事实', (s) => ({ ...s, script: s.script.replace(/topResumedActivity/g, 'X') })],
      ['C11', '实测停止标记被删', (s) => ({ ...s, script: s.script.replace(/已停止运行/g, 'X') })],
      ['C11', '标准基座候选被删', (s) => ({ ...s, script: s.script.replace(/io\.dcloud\.uniappx/g, 'X') })],
      ['C11', '未部署分支被删（退回只报成功）', (s) => ({
        ...s,
        script: s.script.replace('if (-not $deployed.Deployed) {', 'if ($false) {')
      })],
      ['C12', 'launch 参数里塞回 --compile true（只编译不运行）', (s) => ({
        ...s,
        script: s.script.replace(
          "$launchArgs = @('launch', 'app-android', '--project', $Project)",
          "$launchArgs = @('launch', 'app-android', '--project', $Project, '--compile', 'true')"
        )
      })],
      ['C12', '--deviceId 被删', (s) => ({ ...s, script: s.script.replace(/--deviceId/g, '--dev') })],
      ['C12', 'launch 步退回会等待收口的路径', (s) => ({
        ...s,
        script: s.script.replace(/Start-CliLaunchDetached/g, 'Invoke-CliStep')
      })],
      ['C12', '两小步超时开关被删', (s) => ({
        ...s,
        script: s.script.replace('[int]$StepTimeoutSeconds = 180,', '')
      })],
      ['C12', 'AGENTS.md 不再写「仅编译」', (s) => ({
        ...s,
        agents: s.agents.replace(/仅编译/g, '只编译')
      })],
      ['C12', 'ADR 不再写「仅编译」', (s) => ({ ...s, adr: s.adr.replace(/仅编译/g, '只编译') })],
      ['C13', '-CompileOnly 开关被删', (s) => ({ ...s, script: s.script.replace('[switch]$CompileOnly,', '') })],
      ['C13', '-CompileOnly 分支不再传 --compile（退化成真运行）', (s) => ({
        ...s,
        script: s.script.replace("$launchArgs += @('--compile', 'true')", '$null = $null')
      })],
      ['C13', '-CompileOnly 分支又去解析设备（"不接设备也能跑"落空）', (s) => ({
        ...s,
        script: s.script.replace(
          "$target = @{ Serial = ''; Mode = 'compile-only' }",
          '$target = Resolve-TargetDevice -Requested $Device -AdbExe $adbExe'
        )
      })],
      ['C13', '部署停滞判据被删', (s) => ({ ...s, script: s.script.replace(/Test-HxCompileFinished/g, 'X') })],
      ['C13', '停滞阈值被调到超过轮询上限（等于没有快速失败）', (s) => ({
        ...s,
        script: s.script.replace('[int]$DeployStallSeconds = 300', '[int]$DeployStallSeconds = 0')
      })],
      ['C13', '轮询上限被调回 1800', (s) => ({ ...s, script: s.script.replace('[int]$TimeoutSeconds = 900', '[int]$TimeoutSeconds = 1800') })],
      ['C13', 'package.json 未注册 hx:compile-only', (s) => ({ ...s, pkg: s.pkg.replace('"hx:compile-only"', '"hxcompileonly"') })],
      ['C13', 'AGENTS.md 不再写 CompileOnly 分层', (s) => ({
        ...s,
        agents: s.agents.replace(/CompileOnly|compile-only/g, 'XXX')
      })],
      ['C13', 'Get-HxErrorLines 调用点丢了 @()（空数组退化成 $null ⇒ StrictMode 抛错）', (s) => ({
        ...s,
        script: s.script.replace(/@\(Get-HxErrorLines -Output/g, 'Get-HxErrorLines -Output')
      })],
      // C14：最短部署观察窗（#974）—— 每条各自注入，避免某条失效被其余掩盖
      ['C14', '观察窗默认值被压到实测落盘时间以下', (s) => ({
        ...s,
        script: s.script.replace('[int]$DeployDeadlineSeconds = 60', '[int]$DeployDeadlineSeconds = 5')
      })],
      ['C14', '观察窗参数被删（退回「首采样无前进就判 env」）', (s) => ({
        ...s,
        script: s.script.replace(/\[int\]\$DeployDeadlineSeconds\s*=\s*60,\r?\n/, '')
      })],
      ['C14', '收手判定没接线', (s) => ({
        ...s,
        script: s.script.replace('Test-DeployObservationStop -Deployed', 'Get-SomethingElse -Deployed')
      })],
      ['C14', '观察窗没透传给判定（参数形同虚设）', (s) => ({
        ...s,
        script: s.script.replace('-DeployDeadlineSeconds $DeployDeadlineSeconds', '-DeployDeadlineSeconds 0')
      })],
      ['C14', '收手判定退回「会话一退出就 break」', (s) => ({
        ...s,
        script: s.script.replace(
          '    if ($stop.Stop) {',
          '    if ($launch.Proc.HasExited) { $launchExitedEarly = $true; break }\n    if ($stop.Stop) {'
        )
      })],
      ['C14', '纯函数库的 dot-source 被删', (s) => ({
        ...s,
        script: s.script.replace(/lib\\hx-deploy\.ps1/, 'lib\\other.ps1')
      })],
      ['C14', '「会话退出」不再被窗口挡住（窗口形同虚设）', (s) => ({
        ...s,
        deployLib: s.deployLib.replace('$windowElapsed -and $Exited', '$Exited')
      })],
      ['C14', '纯函数丢了「前进即 PASS」', (s) => ({
        ...s,
        deployLib: s.deployLib.replace("Stop = $true; Outcome = 'advanced'", "Stop = $true; Outcome = 'x'")
      })]
    ];
    cases.forEach(([rule, label, mutate]) => {
      const found = scanContract(mutate(real));
      expect({ label, hit: found.some((v) => v.startsWith(rule)) }).toEqual({ label, hit: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('C1：默认增量的 launch 参数里没有干净缓存重建开关，-Full 才给', () => {
    const code = maskDocBlocks(real.script);
    const blocks = code.match(/if\s*\(\s*\$Full\s*\)\s*\{[\s\S]*?\n\}/g) || [];
    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toContain('--cleanCache');
    expect(code).toContain("$launchArgs = @('launch', 'app-android', '--project', $Project)");
  });

  it('C12：launch 是真运行（真运行路径参数里没有 --compile；带 --deviceId；走不等待的派发路径）', () => {
    const code = maskDocBlocks(real.script);
    const at = code.indexOf("$launchArgs = @('launch', 'app-android'");
    expect(at).toBeGreaterThan(-1);
    // 这条曾把错参数锁成「必须」：C1 的 happy-path 原来断言参数里**必须**有 --compile true。
    // `--compile true` 的官方语义是「仅编译代码」⇒ 断言它等于断言「从不运行」。
    // #949 之后它只允许出现在 -CompileOnly 分支里，所以核对范围是「到该分支之前」（真运行路径）。
    const coAt = code.indexOf('if ($CompileOnly) {', at);
    expect(coAt).toBeGreaterThan(at);
    expect(code.slice(at, coAt)).not.toMatch(/--compile/);
    expect(code).toContain('--deviceId');
    expect(code).toContain('Start-CliLaunchDetached');
    expect(code).not.toContain("Invoke-CliStep -Name 'launch'");
  });

  it('C2：正文不含任何强杀调用（头注释里的说明性字面量已被掩码排除）', () => {
    const code = maskDocBlocks(real.script);
    expect(code).not.toMatch(/Stop-Process|taskkill|\.Kill\s*\(/i);
    // 头注释本身必须写明这条纪律（否则下个会话看不见为什么不能 kill）
    expect(real.script).toContain('绝不 kill');
  });

  it('C7：-DryRun 在任何 HBuilderX 交互之前 exit 0，且真派发在忙检测之后', () => {
    const code = maskDocBlocks(real.script);
    const dryAt = code.indexOf('if ($DryRun) {');
    const waitAt = code.indexOf('Wait-HxFree -CliExe');
    const dispatchAt = code.indexOf('Invoke-CliStep -Name');
    expect(dryAt).toBeGreaterThan(-1);
    expect(dryAt).toBeLessThan(waitAt);
    expect(waitAt).toBeLessThan(dispatchAt);
  });

  it('C11：部署判定存在、基线相对、未部署时 exit=env/2 且在报成功之前', () => {
    const code = maskDocBlocks(real.script);
    expect(code).toMatch(/function\s+Get-DeviceDeployFacts/);
    expect(code).toMatch(/function\s+Get-DeployVerdict/);
    expect(code).toContain('HX_RUN_DEPLOY deployed=');
    expect(code).toContain('stat -c %Y');
    expect(code).toContain('-Before $factsBefore -After $factsAfter');
    expect(code).toContain('topResumedActivity');
    expect(code).toContain('已停止运行');
    expect(code).toContain('io.dcloud.uniappx');
    const verdictAt = code.indexOf('if (-not $deployed.Deployed)');
    // -Exit 'ok' 在 #949 之后也出现在 -CompileOnly 分支里 ⇒ 必须从「部署判定」之后开始找
    const okAt = code.indexOf("-Exit 'ok'", verdictAt);
    expect(verdictAt).toBeGreaterThan(-1);
    expect(okAt).toBeGreaterThan(-1);
    expect(verdictAt).toBeLessThan(okAt);
    expect(code.slice(verdictAt, verdictAt + 1600)).toContain('exit 2');
  });

  it('C13：-CompileOnly 只编译拿诊断（不传 --deviceId、跳过设备解析）+ 快速失败参数就位', () => {
    const code = maskDocBlocks(real.script);
    const at = code.indexOf('if ($CompileOnly) {');
    expect(at).toBeGreaterThan(-1);
    expect(code.slice(at, at + 700)).toContain('--compile');
    expect(code.slice(at, at + 700)).not.toContain('--deviceId');
    expect(code).toContain("$target = @{ Serial = ''; Mode = 'compile-only' }");
    expect(code).toContain('Test-HxCompileFinished');
    expect(code).toMatch(/\[int\]\$DeployStallSeconds\s*=\s*300/);
    expect(code).toMatch(/\[int\]\$TimeoutSeconds\s*=\s*900/);
    expect(code).toMatch(/\[int\]\$WaitSeconds\s*=\s*120/);
    expect(real.pkg).toMatch(/"hx:compile-only"\s*:\s*"[^"]*hx-run\.ps1\s+-CompileOnly"/);
  });

  it('C14：部署收手不再早于最短观察窗（会话退出也必须等满 60 秒）', () => {
    const code = maskDocBlocks(real.script);
    const dds = code.match(/\[int\]\$DeployDeadlineSeconds\s*=\s*(\d+)/);
    expect(dds).not.toBeNull();
    expect(Number(dds[1])).toBeGreaterThanOrEqual(60);
    expect(code).toContain('Test-DeployObservationStop -Deployed');
    expect(code).toContain('-DeployDeadlineSeconds $DeployDeadlineSeconds');
    // 旧形态（会话一退出就 break）不得回来 —— 它正是 #974 的假阴性
    expect(code).not.toMatch(/\$launch\.Proc\.HasExited\)\s*\{\s*\$launchExitedEarly\s*=\s*\$true;\s*break\s*\}/);
    // 纯函数的不变量：前进即 PASS；「会话退出」被最短观察窗挡住
    expect(real.deployLib).toContain('function Test-DeployObservationStop');
    expect(real.deployLib).toContain("Stop = $true; Outcome = 'advanced'");
    expect(real.deployLib).toMatch(/\$windowElapsed\s*-and\s*\$Exited/);
  });

  it('C10：AGENTS.md 的三层节奏与 ADR-0008 的一行指针都在', () => {
    expect(real.agents).toContain('## 开发内循环（移动端 UI 迭代）');
    expect(real.agents).toContain('npm run hx:run');
    expect(real.adr).toContain('开发内循环');
    expect(real.adr).toContain('scripts/hx-run.ps1');
  });
});