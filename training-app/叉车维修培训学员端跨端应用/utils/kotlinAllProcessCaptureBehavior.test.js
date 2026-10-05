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
 * 改前（origin/master `36e0d5ff`）有四条 `expect(Number(elapsed)).toBe*Than(...)`，押的是**本机 CPU
 * 供给**而不是被测行为：#1547 执行期实测**空闲单跑也红**（`3 failed / 4 passed`，`Time 237 s`），
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
 * 改前那版在 36-worker 饱和下红的两条，红的都不是耗时阈值而是旁边的**定性**断言：改前 `:189` 的
 * `timedOut` 读到 True（10 s 预算被子进程冷启吃光）、改前 `:245` 同理（4 s 预算）。本机 12 核现测
 * （2026-10-05，原始日志随 PR 正文附出）：子脚本「一次 pwsh 冷启 + 起后代 + 退出」空载 **0.88–0.92 s**、
 * 36-worker 饱和下 **14.3–17.6 s**；300 行合成流那条腿饱和下整腿 **30.3 / 42.8 s**。
 *
 * 预算因此分两类给（这条区分是本文件最容易读错的地方）：
 *   - **给宽不花钱的**（返回由被测物驱动，不由预算驱动）：CAP 的 120 s、④-4 变异腿的 90 s —— 前者一有
 *     EOF 就返回（空闲实测 ~1.9 s），后者的返回由**后代松手**驱动。这类数字只兜挂死。
 *   - **给宽要花钱的**：③ 的 30 s —— 这一腿的耗时**就是**它要验的那个「到点收手」，抬到 120 s 就给每次
 *     门跑多塞 90 s。取 30 s = 饱和最慢冷启 17.6 s 的 1.7 倍；代价与残留风险登记在「已知边界」。
 * 纪律同源：#1544 的 D2 注释（「它不是断言…命中即提前退出 ⇒ 给宽不花钱」）。形态同源：#1544 的 D1
 * （存活探针）+ D5（探针自检），与本仓正面先例 `utils/hxTimingBehavior.test.js` 的
 * `Simulate-DeployTicks`（秒数换成注入的假采样 ⇒ 断言落点是结论字符串与采样序号）。**放宽阈值不是修法**（只是把假红推迟），#1544 的裁定在本族同理 ——
 * 本文件换的是**观测量**，不是数字。
 *
 * ## 怎么真执行
 *
 * 采集层住在 `scripts/lib/process-capture.ps1`（零副作用），本文件用 pwsh dot-source 它后**直接驱动**：
 * 真起子进程、真喂合成流、真读逐行输出与超时标志（先例 `utils/kotlinAllStaleExportBehavior.test.js`）。
 * 被测物是**仓内真件**，变异只落临时副本（不改工作树、不进仓）。
 *
 * ## 判据（③ 门三问）
 *
 * - **判别力（①「我故意弄坏被测物，它会不会红？」）**：③ 组 HOLD 在被弄坏的真件上**实测判红**（本轮两种
 *   载体变异各跑一遍，判红落点与读数随 PR 正文附出），判据是顺序而非耗时；④ 组五条变异腿逐条弄坏
 *   「收全 / 恒不排空 / 退出段有界 / 有界排空 / 超时标志」并断言观测量**真的翻转** ⇒ ①②③ 三组的断言都
 *   有牙。其中「恒不排空」与「退出段无界」两条是本轮新加的：被撤掉的那两条耗时断言原本名义上承担的事，
 *   现在改由定性值/顺序值承担，并且被量过了。
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
 * - **③ 有一条已知的 vacuous 支，本轮把它做成可见而不是藏起来**：若某台机器上子脚本冷启久过 30 s，
 *   ③ 会走「退出段超时」而不是「读段超时」—— 三个断言字段照样全 True（两个出口的 fail-closed 语义相同），
 *   但那一轮没验到读段。机检行因此多带一个 `readBranch` 字段（`[timeout]` 后面是不是「未在预算内排空」），
 *   它**不进断言**（进断言就等于把「机器够快」重新写成判据，本票要消灭的正是这个），但失败时
 *   `objectContaining` 会把整行 received 打出来 ⇒ 「这一轮验的是哪一段」看得见。
 * - ④-4「有界排空」变异腿**不能**沿用 ③ 的 30 s：它断的是变异体 `timedOut=False`，而变异体要先等到子脚本
 *   退出才走得到读段 —— 30 s 在饱和下会被两层冷启（子脚本 + 后代）吃穿，那条腿就会**假红**（改前 `:245`
 *   正是这个形态的实证）。给它 90 s 不花钱：变异体的返回由后代松手（`HOLDER_MARGIN_S.unbounded`）驱动。
 *   ⇒ ③ 与 ④-4 的预算**故意不同**，理由是「谁驱动返回」不同，不是漂移。
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

/** ③ 读段腿的预算（秒）。它是**预算**不是断言 —— 判据看超时标志与释放顺序。取值判据与「给宽要花钱」
 *  的取舍见文件头；30 s = 饱和最慢子脚本冷启 17.6 s 的 1.7 倍（本机 12 核 2026-10-05 现测）。 */
const HOLD_BUDGET_S = 30;
/** ④-4（有界排空变异）腿的预算（秒）：比 ③ 宽，因为变异体的返回由后代松手驱动、不由预算驱动 ⇒ 给宽
 *  不花钱，而给窄会让这条**成对取证的牙**在饱和下假红（机制见文件头「已知边界」第 3 条）。 */
const MUT_HOLD_BUDGET_S = 90;
/** CAP 腿的预算（秒）。改前的 10 s 贴着空闲耗时给 ⇒ 预算自己成了墙钟判据（饱和下 `timedOut` 读到 True）。
 *  给到 120 不花空闲路径的钱（排空一完成就返回，实测 ~1.9 s），只兜挂死。 */
const CAP_BUDGET_S = 120;
/** 超时腿：子进程睡眠与预算（秒）。两者关系是**因果**的 —— 睡满才落 mark，而预算短到必然先收手，
 *  所以「返回时 mark 在不在」就是「退出段有没有预算」的顺序读数（真件 Kill 掉睡过预算的子进程 ⇒
 *  mark 恒不在；饥饿只会让睡眠更长、不会更短 ⇒ 这一维随负载单调安全）。 */
const TMO_BUDGET_S = 3;
const TMO_SLEEP_S = 12;
/** 后代的**自放余量**（秒）：见不到释放信号时的兜底，防夹具挂死。
 *  真件腿取 90 —— 必须长过 `HOLD_BUDGET_S`，否则「排不完」会被读成「排完了」；90 = 30 的 3 倍。
 *  变异「无界排空」腿取 15 —— 该腿断的是「返回时后代已松手」，无界实现在因果上**只能**在松手后返回，
 *  所以这里的余量不参与判据，只决定那条路多慢。同形先例见 #1544：子进程寿命当余量、不当阈值。 */
const HOLDER_MARGIN_S = { real: 90, unbounded: 15 };
/** `drive` 的 execFileSync 上限（毫秒）—— **本文件唯一的硬止停**。必须长于上面几个上限之和
 *  （最坏 = CAP 预算 120 s + 驱动层冷启），否则「驱动自己超时」会顶替「断言判红」，把失败原因吞掉
 *  （#1544 的同一条坑）。为什么只有它能止停：见下面 `jest.setTimeout` 那段 —— 同步 spawn 期间
 *  jest 的计时器根本跑不到，所以挂死只会由 `execFileSync` 自己的 timeout 收口。 */
const DRIVE_TIMEOUT_MS = 300000;

/** jest.setTimeout 在**本文件这类同步守护上拦不住任何东西**，别把它当保险：每条腿都是同步
 *  `execFileSync`，测试体把事件循环占死 ⇒ 计时器轮不到执行。本机实测（head `0570eec5`，2026-10-05）：
 *  把这条临时改成 `15000` 再跑，`Tests: 9 passed` / `Time: 80.106 s`，其中 ③ 单条 **31 128 ms**、
 *  ④-4 **17 937 ms** 全绿 —— 15 s 预算对 31 s 的用例**没有产生任何超时红**（同仓另一会话在
 *  `utils/styleLoopBehavior.test.js` 上量到过同一个现象）。
 *  那为什么还抬到 600 s：① 与 #1544 的文件同形（那条也这么写）；② 真实挂死由 `DRIVE_TIMEOUT_MS`
 *  收口，本值只要**不小于**任何一条腿的正常耗时就不会添乱。**它的取值不构成判据**，别照着它推预算。 */
jest.setTimeout(600000);

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 单引号转义（PowerShell 字面量）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

/** 三条驱动脚本共用的两行：取宿主 pwsh 路径、把「输出是不是 `[timeout]` 开头」取成具名布尔。
 *  共用是为了**不让三条腿各自漂** —— 超时标志的读法必须与超时语义那条腿一字不差。 */
const PS_SELF = '$pwsh = (Get-Process -Id $PID).Path';
const PS_IS_TIMEOUT = "$tmo = $r.Output.StartsWith('[timeout]')";

/**
 * dot-source 指定副本的采集层并执行语句，回读 stdout。
 * @param {string} libAbs 要 dot-source 的库路径（真实库或**变异副本**）
 */
function drive(libAbs, statements) {
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
      timeout: DRIVE_TIMEOUT_MS,
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

/** 新建一个临时目录、交出其中的文件路径（信号文件与脚本同目录：一次夹具一眼看全，不必跨目录对号） */
function newTmpFile(name) {
  return path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-cap-')), name);
}

/** 子脚本写到独立临时目录 */
function writeChild(name, body) {
  return writeScript(newTmpFile(name), body);
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
    PS_SELF,
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(child)}) -TimeoutSeconds ${CAP_BUDGET_S} -Tag 'CAP'`,
    "$ln = ([regex]::Matches($r.Output, '(?m)^LINE-\\d+\\r?$')).Count",
    "$mid = $r.Output -match 'MIDDLE-MARKER.*导出 android 成功'",
    "$last = $r.Output -match 'LAST-MARKER.*导出 android 成功'",
    PS_IS_TIMEOUT,
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $r.ExitCode + "|" + $ln + "|" + $mid + "|" + $last)`,
  ];
}

/**
 * 超时腿：子进程睡过预算，**睡满后先落 mark 再退出** ⇒ mark 存在 ⇔ 子进程已走到退出那一步
 * （被 `Kill` 掉的那一支走不到 `Set-Content`）。这就是退出段的顺序判据。
 */
function tmoStatements(key, markPath) {
  const childCmd = `Start-Sleep ${TMO_SLEEP_S}; Set-Content -LiteralPath ${q(markPath)} -Value exited`;
  return [
    PS_SELF,
    `if (Test-Path -LiteralPath ${q(markPath)}) { Remove-Item -LiteralPath ${q(markPath)} -Force }`,
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-Command',${q(childCmd)}) -TimeoutSeconds ${TMO_BUDGET_S} -Tag 'TMO'`,
    '$childExited = Test-Path -LiteralPath ' + q(markPath),
    PS_IS_TIMEOUT,
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $r.ExitCode + "|" + $childExited)`,
  ];
}

/**
 * 后代的**确定性释放协议**（代替改前的 `Start-Sleep 12`）：握着继承来的 stdout 管道，直到看见 flag
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

function holdStatements(h, key, budgetS) {
  return [
    PS_SELF,
    `foreach ($f in @(${q(h.flag)}, ${q(h.mark)})) { if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f -Force } }`,
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(h.child)}) -TimeoutSeconds ${budgetS} -Tag 'HOLD'`,
    // 顺序判据（本组的核心）：返回**那一刻** mark 还不存在 ⇒ 后代仍握着管道 ⇒ 读段确实没等排空。
    `$held = -not (Test-Path -LiteralPath ${q(h.mark)})`,
    // 分支可见性（**不进断言**，理由见文件头「已知边界」第 2 条）：两个超时出口的文案不同 ——
    // 「未在预算内排空」= 读段超时（要验的那一支），「未返回，已终止」= 退出段超时。
    `$readBranch = $r.Output -match ${q('未在预算内排空')}`,
    // 释放信号留在最后发：它是顺序判据的另一半（发早了后代就没得握）；同时它取代了改前那 12 s
    // 固定睡眠 —— 旧睡眠还被 execFileSync 的 stdout 继承链拖住，改前两条 HOLD 腿实测各 13.9 / 14.0 s。
    `Set-Content -LiteralPath ${q(h.flag)} -Value release`,
    PS_IS_TIMEOUT,
    `Write-Output ("${key}=" + $r.TimedOut + "|" + $tmo + "|" + $held + "|" + $readBranch)`,
  ];
}

/** 机检行 ⇒ 具名字段对象：断言写成字段，失败时读得出是哪一维翻车。
 *  ⚠️ 段数与字段名数必须相等 —— 不等就当场红，否则缺的那一维会变成 `undefined`，
 *  在 `toEqual` 里读起来像「值不对」而不是「行被截了」。 */
function fields(line, names) {
  const values = line.split('|');
  expect({ segments: values.length, names: names.length }).toEqual({ segments: names.length, names: names.length });
  return names.reduce((acc, n, i) => ({ ...acc, [n]: values[i] }), {});
}

const CAP_FIELDS = ['timedOut', 'timeoutHead', 'exitCode', 'lines', 'mid', 'last'];
const TMO_FIELDS = ['timedOut', 'timeoutHead', 'exitCode', 'childExitedAtReturn'];
const HOLD_FIELDS = ['timedOut', 'timeoutHead', 'heldAtReturn', 'readBranch'];

// ===== ① 多行 + 末行后立刻退出 ⇒ 收全（票面期望 c 的字面要求）=====

describe('采集契约：多行 + 末行后立刻退出 ⇒ 收全（期望 c 的合成流）', () => {
  it('必不红半：300 行（中间含标记行、末行也是标记行）全部收齐，退出码 0、不超时', () => {
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(REAL_LIB, capStatements(child, 'CAP'));
    // 「在预算内排完」由 {timedOut, timeoutHead} 承担。改前这里还叠了一条耗时阈值，而 ④-2
    // 「读段恒不排空」腿实测它**在同一个缺陷上照样绿** ⇒ 撤掉不丢语义（读数随 PR 正文附出）。
    expect(fields(pick(out, 'CAP'), CAP_FIELDS)).toEqual({
      timedOut: 'False', timeoutHead: 'False', exitCode: '0', lines: '300', mid: 'True', last: 'True',
    });
  });
});

// ===== ② 超时语义保持（票面期望 a：超时仍走 reason=timeout 那个出口）=====

describe('采集契约：超时语义保持（期望 a：超时仍 TimedOut=true ⇒ 门 reason=timeout）', () => {
  it('必不红半：子进程睡过预算 ⇒ TimedOut=True、输出以 [timeout] 开头、退出码 -1、且**子进程还没走到退出**', () => {
    const mark = newTmpFile('tmo-exit.mark');
    const out = drive(REAL_LIB, tmoStatements('TMO', mark));
    // childExitedAtReturn=True 就是「退出段没预算」的形态（等到进程真退出再判定）。改前那条耗时阈值
    // 在这里独家承担的只有「返回得快」= 性能；「退出段有界」改由这一维 + timedOut 承担，
    // 且 ④-3「退出段无界」腿量过它真的会翻。
    expect(fields(pick(out, 'TMO'), TMO_FIELDS)).toEqual({
      timedOut: 'True', timeoutHead: 'True', exitCode: '-1', childExitedAtReturn: 'False',
    });
  });
});

// ===== ③ 读段有界（票面期望 a 的本体：**顺序**判据，不量耗时）=====

describe('采集契约：读段有界（期望 a：排不空 ⇒ TimedOut，且返回时后代仍握着管道）', () => {
  it('必不红半：子脚本秒退、后代握 stdout ⇒ TimedOut=True 且 **返回那一刻管道还没松开**（旧实现做不到）', () => {
    const h = holdHarness(HOLDER_MARGIN_S.real);
    const out = drive(REAL_LIB, holdStatements(h, 'HOLD', HOLD_BUDGET_S));
    const f = fields(pick(out, 'HOLD'), HOLD_FIELDS);
    // heldAtReturn=True ⇔ Invoke-Process 没等管道排空就返回了。旧实现（无界 `.Result`）在因果上
    // 只能拿到 False（它必然在后代松手之后才返回）⇒ 这一维就是「有界」的非计时表达。
    // ④-4 读的是同一维的相反值 ⇒ 顺带给探针自己做了正控（防它退化成恒 True）。
    // 用 objectContaining 而不是 toEqual：`readBranch` 不参与判定（理由见文件头「已知边界」第 2 条），
    // 但失败时 jest 会把整行 received 打出来 ⇒ 「这一轮走的是读段还是退出段」看得见。
    expect(f).toEqual(expect.objectContaining({
      timedOut: 'True', timeoutHead: 'True', heldAtReturn: 'True',
    }));
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
    // 这一腿同时是「耗时阈值没有独家语义」的证据：缺陷形态是「没排空」，而耗时照样落在阈值内 ——
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
    const mark = newTmpFile('tmo-exit.mark');
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
    const out = drive(broken, holdStatements(h, 'MUT', MUT_HOLD_BUDGET_S));
    const f = fields(pick(out, 'MUT'), HOLD_FIELDS);
    // 与 ③ 那组**逐字段相反**（`readBranch` 也相反：变异体拿到的是子脚本的真实输出，不是排空超时文案）。
    // 这就是「③ 有牙」的镜像证据，且四维都不涉及耗时。预算与 ③ 故意不同，理由见文件头「已知边界」第 3 条。
    expect(f).toEqual({ timedOut: 'False', timeoutHead: 'False', heldAtReturn: 'False', readBranch: 'False' });
  });

  it('变异「超时标志」⇒ 超时场景 TimedOut=False（② 的 timedOut 随之判红）', () => {
    const broken = mutatedLib([[
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true',
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $false',
    ]]);
    const mark = newTmpFile('tmo-exit.mark');
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
