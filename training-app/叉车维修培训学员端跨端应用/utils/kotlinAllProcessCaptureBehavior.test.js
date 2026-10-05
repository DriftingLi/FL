/**
 * ④c 采集层（`scripts/lib/process-capture.ps1` 的 `Invoke-Process`）· **行为级**守护（票 #1285；判据形态 #1549）
 *
 * ## 为什么是行为守护
 *
 * #1285 的缺陷只有**跑起来**才看得见：
 *   ① 读段**没有有界预算** —— `WaitForExit($ms)` 只看住「进程退出」这一段，旧实现随后无界地阻塞在
 *      `.Result` 上：本机探针实测（2026-09-26）预算 4 秒、子脚本秒退、后代握着 stdout 管道 40 秒
 *      ⇒ 旧实现总耗时 **41137 ms（预算的 10.3 倍）且 TimedOut=False** —— 超时语义对读段完全失效；
 *   ② 采集结果是 publish 判据（#1272 的 fail-closed 正向标记）的**唯一输入** —— 「读全 stdout」没守住，
 *      判据再对也是在没排空的流上判。
 * 断言「源码里出现 WaitAll」对这两条毫无判别力（`docs/agents/guards.md`：接线守护不构成 ③ 证据）。
 *
 * ## 本文件没有耗时阈值（#1549，2026-10-05）
 *
 * 旧版有四条 `expect(Number(elapsed)).toBe*Than(...)`（原 `:194` / `:206` / `:220` / `:246`），押的是
 * **本机 CPU 供给**而不是被测行为：#1547 执行期实测**空闲单跑也红**（`3 failed / 4 passed`，`Time 237 s`），
 * 同一 head 的 CI `mobile-test` 全绿 ⇒ 红的是墙钟不是逻辑，而族一那条「重跑即绿」的退路对本族**不成立**。
 * 现判据只有两类观测量：
 *   - **定性值**：`TimedOut`、输出是否 `[timeout]` 开头、退出码、行数与两处标记行；
 *   - **两条顺序判据**（本文件的核心，一段一维）：
 *     · 退出段 —— 「返回时子进程已经走到退出那一步了吗」：子进程睡满后**先落 mark 再退出**，
 *       mark 在 ⇔ 它已走到那一步；真件把睡过预算的子进程 `Kill` 掉 ⇒ mark 恒不在。
 *     · 读段 —— 「返回时后代还握着 stdout 吗」：后代由**确定性信号**释放（见 `holderBody`），不是靠睡眠计时。
 * 于是「无界」在因果上**只能**表现为「返回时对方已经松手」—— 不需要比较耗时就能把旧实现判红。
 *
 * ⚠️ 撤掉阈值不等于自动免疫负载：**预算自己也会变成判据**。本轮 A/B 同窗实测把这条抓出来了 ——
 * 旧版在 36-worker 饱和下红的那两条，红的都不是耗时阈值而是旁边的**定性**断言
 * （`BEFORE1549:189` 的 `timedOut` 读到 True：10 s 预算被子进程冷启吃光；`BEFORE1549:245` 同理，4 s 预算）。
 * 本机现测子脚本冷启：空载 **0.88–0.92 s**，饱和 **14.3–17.6 s**（`.scratch/childstart.txt`）。
 * 所以 CAP 预算取 60 s、HOLD 预算取 30 s、后代自放余量取 90 s / 15 s —— 这些数字全部只兜「挂死」与
 * 「挤不挤得进那条要验的分支」、都不参与断言。同一条纪律在 #1544 里记作「等待预算 ≠ 断言」。
 * 形态同源：#1544 的 D1（存活探针）+ D5（探针自检），与本仓正面先例 `utils/hxTimingBehavior.test.js`
 * 的 `Simulate-DeployTicks`（把秒数换成注入的假采样 ⇒ 断言落点是结论字符串与采样序号）。
 * **放宽阈值不是修法**（只是把假红推迟），#1544 的裁定在本族同理。
 *
 * ## 怎么真执行
 *
 * 采集层住在 `scripts/lib/process-capture.ps1`（零副作用），本文件用 pwsh dot-source 它后**直接驱动**：
 * 真起子进程、真喂合成流、真读逐行输出与超时标志（先例 `utils/kotlinAllStaleExportBehavior.test.js`）。
 * 被测物是**仓内真件**，变异只落临时副本（不改工作树、不进仓）。
 *
 * ## 判据（③ 门三问）
 *
 * - **判别力（①「我故意弄坏被测物，它会不会红？」）**：③ 组 HOLD 在旧实现（无界 `.Result`）上**实测判红**，
 *   判据是顺序而非耗时；④ 组五条变异腿逐条弄坏「收全 / 恒不排空 / 退出段有界 / 有界排空 / 超时标志」，
 *   并断言观测量**真的翻转** ⇒ ①②③ 三组的断言都有牙。其中「恒不排空」与「退出段无界」两条是本轮新加的：
 *   被撤掉的那两条耗时断言原本名义上承担的事，现在改由定性值/顺序值承担，并且被量过了。
 * - **测的是该测的那一支（②）**：驱的是真实子进程与真实管道（不是 mock 字符串），退出码 / 超时标志 /
 *   逐行输出 / 释放顺序全部从真执行里取。
 * - **成对取证（③「只跑通过的那一次不算验收」）**：收全 + 超时语义保持 + 读段有界三条必不红，
 *   对 ④ 组五条镜像腿（变异后必须是相反的那组值）。顺序判据的正控也在组内：同一维在 ③ 取 True、
 *   在 ④ 的变异腿取 False —— 形同 #1544 的 D5，防探针退化成恒真。
 *
 * ## 已知边界（写实，勿读成「已覆盖」）
 *
 * - 票面字段里那次「中间丢行」在合成流上**复现不了**（本机实测：子进程退出即关写端，300 行含中间标记行
 *   全量返回）—— 实证复现的缺陷是**读段无界**（上文 41137 ms）。门端到端（真跑 publish）需要 HBuilderX
 *   的本地 IPC，受限会话跑不了（与 #1272 守护记的边界同类）⇒ 字段那次丢行的机制由票面字节级记录承担；
 *   本守护锁的是**采集契约**：读段必须有界（排不完 ⇒ TimedOut ⇒ 门 reason=timeout，fail-closed）、
 *   多行必须收全、超时必须保持。
 * - **比现测更饿的机器上，③ 会退到「退出段超时」那一支**：③ 的读段预算 30 s 是按本机饱和最慢读数
 *   17.6 s 的 1.7 倍定的。若某台机器上子脚本冷启久过 30 s，③ 仍判绿（两个出口的 fail-closed 语义相同、
 *   断言也同为 True），但它那一轮验的是退出段而不是读段 —— **这是本文件目前唯一已知的 vacuous 形态**，
 *   登记在此而不是当它不存在。反向的危险（变异腿假红）已由 `MUT` 腿的预算同一口径消掉：本轮 A/B 里
 *   旧版 `:245` 那条假红就是这个形态的实证。
 * - 变异腿里 `WaitForExit` 的替代写法有个坑：只把 `$exited = $p.WaitForExit($ms)` 换成
 *   `$p.WaitForExit()` 注入的**不是**「无界等待」—— 该重载返回 void ⇒ PowerShell 赋成 `$null` ⇒
 *   下一句 `if (-not $exited)` 恒真 ⇒ 实际注入的是「恒判超时」，与要仿的缺陷反向（2026-10-05 实测踩过）。
 *   故变异必须写成「等到真退出 + 当作成功」两步。
 * - 同款写法还存在于 `scripts/mp-weixin-check.ps1` 与 `scripts/lib/hx-busy.ps1`（② 门与忙探测各自的
 *   副本）—— **本票不改**（票面只 mandate ④c），此处写明以免被读成「全仓已收口」。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（与仓库先例一致；
 * 静默跳过等于假绿）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'process-capture.ps1');
const GATE_REL = 'scripts/kotlin-all-check.ps1';

/** HOLD 腿传给被测物的读段预算（秒）。它是**预算**不是断言 —— 判据看超时标志与释放顺序。
 *  取值判据（2026-10-05 本机现测，`.scratch/childstart.txt`）：子脚本「一次 pwsh 冷启 + 起后代 + 退出」
 *  空载 **0.88–0.92 s**，36-worker 饱和（CPU 100%）下 **14.3–17.6 s**。预算必须**大于**子脚本冷启，
 *  否则饥饿会把这一腿挤到「退出段超时」那条支上 —— 空载时 4 s 够用，饱和时就成了 #1544 里
 *  「默认 5 s 预算本身就是一条墙钟判据」的同一个坑（本轮 A/B 实测：旧版 `:245` 腿正是这样假红的，
 *  `timedOut` 在变异体上读到了 True）。取 **30 s** = 饱和最慢读数 17.6 s 的 1.7 倍。
 *  这一腿的耗时 ≈ 预算本身（被测物就是要在预算处收手），所以这是**用行为换覆盖**、不是给判据松绑。 */
const HOLD_BUDGET_S = 30;
/** CAP 腿的预算（秒）。旧值 10 贴着空闲耗时给，于是预算自己成了一条墙钟判据：本轮 A/B 实测饱和下
 *  300 行合成流跑不完 10 s ⇒ `timedOut` 读到 True ⇒ **定性断言**被机器负载判红（`BEFORE1549:189`）。
 *  抬到 60 = 饱和下子脚本冷启 17.6 s 的 3.4 倍，且不花空闲路径的钱（何时返回取决于子进程排空完成，
 *  不取决于预算），只兜挂死。 */
const CAP_BUDGET_S = 60;
/** 超时腿：子进程睡眠与预算（秒）。两者关系是**因果**的 —— 睡满才落 mark，而预算短到必然先收手，
 *  所以「返回时 mark 在不在」就是「退出段有没有预算」的顺序读数（真件 Kill 掉睡过预算的子进程 ⇒
 *  mark 恒不在；饥饿只会让睡眠更长、不会更短 ⇒ 这一维随负载单调安全）。 */
const TMO_BUDGET_S = 3;
const TMO_SLEEP_S = 12;
/** 后代的**自放余量**（秒）：见不到释放信号时的兜底，防夹具挂死。
 *  HOLD 组（真件）取 90 —— 必须长过 `HOLD_BUDGET_S`，否则真件会在预算耗尽前先等到松手、把「排不完」
 *  读成「排完了」；90 = 30 的 3 倍，且真件在 ~30 s 就返回，命中这一支需要再饿 60 s。
 *  变异「无界排空」腿取 15 —— 该腿断的是「返回时后代已松手」，无界实现在因果上**只能**在松手后返回，
 *  所以这里的余量不参与判据，只决定那条路多慢。同形先例见 #1544：子进程寿命当余量、不当阈值。 */
const HOLDER_MARGIN_S = { real: 90, unbounded: 15 };
/** `drive` 的 execFileSync 上限（毫秒）。必须长于上面几个上限之和，否则「驱动自己超时」会顶替
 *  「断言判红」，把失败原因吞掉（#1544 的同一条坑）。 */
const DRIVE_TIMEOUT_MS = 180000;

/** jest.setTimeout 是**每条用例**的预算，只兜「挂死」，本文件不拿它比快慢。取本文件最坏那条的内部
 *  硬上限（CAP 预算 60 s / HOLD 预算 30 s / 无界变异阻塞 15 s / 超时腿子进程 12 s，各再加驱动层 pwsh
 *  冷启）与 `DRIVE_TIMEOUT_MS` 之上留余量 ⇒ 300 s。⚠️ 默认 5 s 本身就是一条墙钟判据（同 #1544），必须显式抬高。 */
jest.setTimeout(300000);

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 单引号转义（PowerShell 字面量）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

/**
 * dot-source 指定副本的采集层并执行语句，回读 stdout。
 * @param {string} libAbs 要 dot-source 的库路径（真实库或**变异副本**）
 */
function drive(libAbs, statements, timeout = DRIVE_TIMEOUT_MS) {
  const prelude = [
    '$ErrorActionPreference = "Stop"',
    // 必须先把编码钉成 UTF-8：脚本与断言里都有中文，被管道捕获时可能落成 OEM 代码页 ⇒ 恒假
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    `. ${q(libAbs)}`,
  ];
  const script = prelude.concat(statements).join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return String(stdout);
  } catch (e) {
    throw new Error(
      '驱动采集层失败（pwsh 不可用、库抛错，或驱动上限被命中 —— 后者说明被测物阻塞得比预算久得多）：\n'
      + `exit=${e.status}\nstdout=${e.stdout}\nstderr=${e.stderr || e.message}`
    );
  }
}

/** 取 `KEY=value` 行 */
function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  if (m === null) throw new Error(`输出里没有 ${key}：\n${stdout}`);
  return m[1].trim();
}

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function mutatedLib(replacements) {
  let src = readText(path.join(ROOT, LIB_REL));
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-cap-mut-')), 'process-capture.ps1');
  fs.writeFileSync(file, src);
  return file;
}

const REAL_LIB = path.join(ROOT, LIB_REL);

/** 脚本落到指定路径；首行钉 UTF-8 —— 与真实 CLI 一致（管道上输出 UTF-8，票面字节级记录） */
function writeScript(file, body) {
  fs.writeFileSync(file, ['[Console]::OutputEncoding = [System.Text.Encoding]::UTF8'].concat(body).join('\n'), 'utf8');
  return file;
}

/** 新建一个临时目录并交出其中的文件路径（信号文件与脚本同目录：一次夹具一眼看全，不必跨目录对号） */
function tmpIn(name) {
  return path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-cap-')), name);
}

/** 子脚本写到独立临时目录 */
function writeChild(name, body) {
  return writeScript(tmpIn(name), body);
}

/** 「多行 + 末行后立刻退出」的合成流：300 行，第 150 行是中间标记行，末行也是标记行（票面期望 c） */
const LINES_BODY = [
  'for ($i = 1; $i -le 300; $i++) {',
  "    if ($i -eq 150) { Write-Output 'MIDDLE-MARKER 导出 android 成功，路径为：D:\\x' }",
  '    Write-Output "LINE-$i"',
  '}',
  "Write-Output 'LAST-MARKER 导出 android 成功，路径为：D:\\y'",
];

function capStatements(child, key) {
  return [
    '$pwsh = (Get-Process -Id $PID).Path',
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(child)}) -TimeoutSeconds ${CAP_BUDGET_S} -Tag 'CAP'`,
    "$ln = ([regex]::Matches($r.Output, '(?m)^LINE-\\d+\\r?$')).Count",
    "$mid = $r.Output -match 'MIDDLE-MARKER.*导出 android 成功'",
    "$last = $r.Output -match 'LAST-MARKER.*导出 android 成功'",
    "$tmo = $r.Output.StartsWith('[timeout]')",
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $r.ExitCode + "|" + $ln + "|" + $mid + "|" + $last)`,
  ];
}

/**
 * 超时腿：子进程睡过预算，**睡满后先落 mark 再退出** ⇒ mark 存在 ⇔ 子进程已走到退出那一步
 * （被 `Kill` 掉的那一支走不到 `Set-Content`）。这就是退出段的顺序判据。
 */
function tmoStatements(key, markPath, opts = {}) {
  const sleepS = opts.sleepS === undefined ? TMO_SLEEP_S : opts.sleepS;
  const budgetS = opts.budgetS === undefined ? TMO_BUDGET_S : opts.budgetS;
  const childCmd = `Start-Sleep ${sleepS}; Set-Content -LiteralPath ${q(markPath)} -Value exited`;
  return [
    '$pwsh = (Get-Process -Id $PID).Path',
    `if (Test-Path -LiteralPath ${q(markPath)}) { Remove-Item -LiteralPath ${q(markPath)} -Force }`,
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-Command',${q(childCmd)}) -TimeoutSeconds ${budgetS} -Tag 'TMO'`,
    '$exitedAlready = Test-Path -LiteralPath ' + q(markPath),
    "$tmo = $r.Output.StartsWith('[timeout]')",
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $r.ExitCode + "|" + $exitedAlready)`,
  ];
}

/**
 * 后代的**确定性释放协议**（代替旧版的 `Start-Sleep 12`）：握着继承来的 stdout 管道，直到看见 flag
 * （由驱动脚本在 `Invoke-Process` **返回之后**才创建）才松手；松手前先落 mark。
 * 于是「驱动侧看到 mark」⇔「管道在返回前已经松开」—— 这是一个**顺序**观测量，与耗时无关。
 * `marginS` 只在「见不到 flag」那条兜底支上生效（防夹具挂死），正常路径用不到它。
 */
function holderBody(flagPath, markPath, marginS) {
  return [
    `$flag = ${q(flagPath)}`,
    `$mark = ${q(markPath)}`,
    `$deadline = (Get-Date).AddSeconds(${marginS})`,
    'while ((Get-Date) -lt $deadline -and -not (Test-Path -LiteralPath $flag)) { Start-Sleep -Milliseconds 100 }',
    'Set-Content -LiteralPath $mark -Value released',
    'exit 0',
  ];
}

/**
 * 子脚本：打印一行 ⇒ 起一个握着 stdout 管道的后代 ⇒ **立刻退出**。
 * 「秒退」是关键：进程退出 ≠ 管道 EOF，真正会卡住的是读段（#1285 的缺陷本体）。
 */
function holdChildBody(holderPath) {
  return [
    "Write-Output 'BEFORE-HOLD 导出 android 成功'",
    '$psi = [System.Diagnostics.ProcessStartInfo]::new()',
    "$psi.FileName = 'pwsh'",
    '$psi.UseShellExecute = $false',
    `foreach ($a in @('-NoProfile', '-File', ${q(holderPath)})) { [void]$psi.ArgumentList.Add($a) }`,
    '[void][System.Diagnostics.Process]::Start($psi)',
    "Write-Output 'AFTER-HOLD'",
    'exit 0',
  ];
}

/** HOLD 组夹具：子脚本、后代脚本、flag/mark 两个信号文件同在一个临时目录 */
function holdHarness(marginS) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-cap-hold-'));
  const flag = path.join(dir, 'release.flag');
  const mark = path.join(dir, 'released.mark');
  const holder = writeScript(path.join(dir, 'holder.ps1'), holderBody(flag, mark, marginS));
  const child = writeScript(path.join(dir, 'child-hold.ps1'), holdChildBody(holder));
  return { child, flag, mark };
}

function holdStatements(h, key) {
  return [
    '$pwsh = (Get-Process -Id $PID).Path',
    `foreach ($f in @(${q(h.flag)}, ${q(h.mark)})) { if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f -Force } }`,
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(h.child)}) -TimeoutSeconds ${HOLD_BUDGET_S} -Tag 'HOLD'`,
    // 顺序判据（本组的核心）：返回**那一刻** mark 还不存在 ⇒ 后代仍握着管道 ⇒ 读段确实没等排空。
    `$held = -not (Test-Path -LiteralPath ${q(h.mark)})`,
    // 释放信号留在最后发，两个理由：它是顺序判据的另一半（发早了后代就没得握），且旧写法那 12 s
    // 固定睡眠同时被 execFileSync 的 stdout 继承链拖住 —— 2026-10-05 实测那两条腿各 13.9 / 14.0 s，
    // 换成信号后真件这条降到 ~5 s。
    `Set-Content -LiteralPath ${q(h.flag)} -Value release`,
    "$tmo = $r.Output.StartsWith('[timeout]')",
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $held)`,
  ];
}

/** 机检行 ⇒ 具名字段对象：断言写成字段，失败时读得出是哪一维翻车 */
function fields(line, names) {
  const values = line.split('|');
  return names.reduce((acc, n, i) => ({ ...acc, [n]: values[i] }), {});
}

const CAP_FIELDS = ['timedOut', 'timeoutHead', 'exitCode', 'lines', 'mid', 'last'];
const TMO_FIELDS = ['timedOut', 'timeoutHead', 'exitCode', 'childExitedAtReturn'];
const HOLD_FIELDS = ['timedOut', 'timeoutHead', 'heldAtReturn'];

// ===== ① 多行 + 末行后立刻退出 ⇒ 收全（票面期望 c 的字面要求）=====

describe('采集契约：多行 + 末行后立刻退出 ⇒ 收全（期望 c 的合成流）', () => {
  it('必不红半：300 行（中间含标记行、末行也是标记行）全部收齐，退出码 0、不超时', () => {
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(REAL_LIB, capStatements(child, 'CAP'));
    // 「在预算内排完」由 {timedOut, timeoutHead} 承担。旧版在这里还叠了一条耗时阈值，而 ④ 的
    // 「读段恒不排空」腿实测它**在同一个缺陷上照样绿**（1552 ms，远在该阈值内）⇒ 撤掉不丢语义。
    expect(fields(pick(out, 'CAP'), CAP_FIELDS)).toEqual({
      timedOut: 'False', timeoutHead: 'False', exitCode: '0', lines: '300', mid: 'True', last: 'True',
    });
  });
});

// ===== ② 超时语义保持（票面期望 a：超时仍走 reason=timeout 那个出口）=====

describe('采集契约：超时语义保持（期望 a：超时仍 TimedOut=true ⇒ 门 reason=timeout）', () => {
  it('必不红半：子进程睡过预算 ⇒ TimedOut=True、输出以 [timeout] 开头、退出码 -1、且**子进程还没走到退出**', () => {
    const mark = tmpIn('tmo-exit.mark');
    const out = drive(REAL_LIB, tmoStatements('TMO', mark));
    // childExitedAtReturn=True 就是「退出段没预算」的形态（等到进程真退出再判定）。旧版那条耗时阈值
    // 在这里独家承担的只有「返回得快」= 性能；「退出段有界」改由这一维 + timedOut 承担，
    // 且 ④ 的「退出段无界」腿量过它真的会翻。
    expect(fields(pick(out, 'TMO'), TMO_FIELDS)).toEqual({
      timedOut: 'True', timeoutHead: 'True', exitCode: '-1', childExitedAtReturn: 'False',
    });
  });
});

// ===== ③ 读段有界（票面期望 a 的本体：**顺序**判据，不量耗时）=====

describe('采集契约：读段有界（期望 a：排不空 ⇒ TimedOut，且返回时后代仍握着管道）', () => {
  it('必不红半：子脚本秒退、后代握 stdout ⇒ TimedOut=True 且 **返回那一刻管道还没松开**（旧实现做不到）', () => {
    const h = holdHarness(HOLDER_MARGIN_S.real);
    const out = drive(REAL_LIB, holdStatements(h, 'HOLD'));
    const f = fields(pick(out, 'HOLD'), HOLD_FIELDS);
    // heldAtReturn=True ⇔ Invoke-Process 没等管道排空就返回了。旧实现（无界 `.Result`）在因果上
    // 只能拿到 False（它必然在后代松手之后才返回）⇒ 这一维就是「有界」的非计时表达。
    // ④ 的「无界排空」腿读的是同一维的相反值 ⇒ 顺带给探针自己做了正控（防它退化成恒 True）。
    expect(f).toEqual({ timedOut: 'True', timeoutHead: 'True', heldAtReturn: 'True' });
  });
});

// ===== ④ 成对取证（变异后观测量必须翻转）：把库注入变异，证明上面的断言有牙 =====

describe('成对取证（变异后观测量必须翻转）：把采集层注入变异，证明上面三组的断言有牙', () => {
  const WAIT_ALL = '$drained = [System.Threading.Tasks.Task]::WaitAll([System.Threading.Tasks.Task[]]@($stdout, $stderr), $remainingMs)';

  it('变异「收全」（丢掉 stdout 那半）⇒ 300 行变 0、两处标记行全丢（① 的断言随之判红）', () => {
    const broken = mutatedLib([['$out = $stdout.Result + $stderr.Result', '$out = $stderr.Result']]);
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(broken, capStatements(child, 'MUT'));
    const f = fields(pick(out, 'MUT'), CAP_FIELDS);
    expect({ lines: f.lines, mid: f.mid, last: f.last }).toEqual({ lines: '0', mid: 'False', last: 'False' });
  });

  it('变异「读段恒不排空」（WaitAll 的返回值改恒假）⇒ 收全腿的 timedOut 翻成 True（① 撤掉阈值后仍有牙）', () => {
    const broken = mutatedLib([[WAIT_ALL, '$drained = $false']]);
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(broken, capStatements(child, 'MUT'));
    const f = fields(pick(out, 'MUT'), CAP_FIELDS);
    // 这一腿同时是「耗时阈值没有独家语义」的证据：缺陷形态是「没排空」，而耗时照样很小 ——
    // 判红的是定性值，不是墙钟。
    expect({ timedOut: f.timedOut, timeoutHead: f.timeoutHead })
      .toEqual({ timedOut: 'True', timeoutHead: 'True' });
  });

  it('变异「退出段无界」（等到进程真退出并当作成功）⇒ mark 在返回时已存在（② 的顺序维随之判红）', () => {
    // ⚠️ 变异必须是「无界地等 + 当作成功」，不是只把 `$exited = $p.WaitForExit($ms)` 换成
    //    `$p.WaitForExit()`：后者返回 void ⇒ PowerShell 赋成 `$null` ⇒ 下一句 `if (-not $exited)` 恒真
    //    ⇒ 注入的是「恒判超时」，与要仿的缺陷反向（2026-10-05 实测就是这么红的）。
    const broken = mutatedLib([
      ['$exited = $p.WaitForExit($TimeoutSeconds * 1000)', '$p.WaitForExit(); $exited = $true'],
    ]);
    const mark = tmpIn('tmo-exit.mark');
    const out = drive(broken, tmoStatements('MUT', mark));
    const f = fields(pick(out, 'MUT'), TMO_FIELDS);
    // **只断顺序维**：变异体走到读段时 `remainingMs` 必然已是负数 ⇒ 落到 1 s 下限，而那一秒在饱和机器上
    // 不保证排得完 ⇒ `timedOut/exitCode` 可能多给出「排不空」那个出口。顺序维不受影响（子进程真退出过
    // ⇒ mark 必在），所以这一维才是这一腿的牙。
    expect(f.childExitedAtReturn).toBe('True');
  });

  it('变异「有界排空」（WaitAll 恒真 ⇒ 退回无界 .Result）⇒ 返回时后代已松手（③ 的顺序判据随之判红）', () => {
    const broken = mutatedLib([[WAIT_ALL, '$drained = $true']]);
    const h = holdHarness(HOLDER_MARGIN_S.unbounded);
    const out = drive(broken, holdStatements(h, 'MUT'));
    const f = fields(pick(out, 'MUT'), HOLD_FIELDS);
    // 与 ③ 那组**逐字段相反**：这就是「③ 有牙」的镜像证据，且两维都不涉及耗时。
    expect(f).toEqual({ timedOut: 'False', timeoutHead: 'False', heldAtReturn: 'False' });
  });

  it('变异「超时标志」⇒ 超时场景 TimedOut=False（② 的 timedOut 随之判红）', () => {
    const broken = mutatedLib([[
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true',
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $false',
    ]]);
    const mark = tmpIn('tmo-exit.mark');
    const out = drive(broken, tmoStatements('MUT', mark));
    const f = fields(pick(out, 'MUT'), TMO_FIELDS);
    expect({ timedOut: f.timedOut, timeoutHead: f.timeoutHead, exitCode: f.exitCode })
      .toEqual({ timedOut: 'False', timeoutHead: 'True', exitCode: '-1' });
  });
});

// ===== ⑤ 接线：库被门脚本真的用上（**不构成 ③ 证据**，行为面由上面三组承重）=====

describe('接线（本组是接线守护，不构成 ③ 证据）：门脚本确实消费这个库', () => {
  const gate = readText(path.join(ROOT, GATE_REL));
  const lib = readText(path.join(ROOT, LIB_REL));

  it('门脚本 dot-source 采集库并调用它，且不再自带一份定义（防重复长回来）', () => {
    expect(gate).toContain('lib\\process-capture.ps1');
    expect(gate).toContain('Invoke-Process -FilePath');
    expect(gate).not.toContain('function Invoke-Process');
    expect((lib.match(/function Invoke-Process/g) || []).length).toBe(1);
  });
});
