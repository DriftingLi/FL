/**
 * ④c 采集层（`scripts/lib/process-capture.ps1` 的 `Invoke-Process`）· **行为级**守护（票 #1285）
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
 * ## 怎么真执行
 *
 * 采集层住在 `scripts/lib/process-capture.ps1`（零副作用），本文件用 pwsh dot-source 它后**直接驱动**：
 * 真起子进程、真喂合成流、真量总耗时与逐行输出（先例 `utils/kotlinAllStaleExportBehavior.test.js`）。
 * 被测物是**仓内真件**，不是手抄镜像。
 *
 * ## 判据（③ 门三问）
 *
 * - **判别力（①「我故意弄坏被测物，它会不会红？」）**：H3「读段有界」在旧实现上**实测判红**
 *   （预算 4 秒量到 >8 秒）；末组把库注入变异（拆排空 / 丢 stdout / 丢超时标志），证明三组断言
 *   **真的有牙**。
 * - **测的是该测的那一支（②）**：驱的是真实子进程与真实管道（不是 mock 字符串），退出码 / 超时标志 /
 *   总耗时 / 逐行输出全部从真执行里量。
 * - **成对取证（③「只跑通过的那一次不算验收」）**：多行收全（必不红）+ 超时语义保持（必不红）各一条，
 *   读段有界一条（在旧实现上必红过）；变异组再给三条必红。
 *
 * ## 已知边界（写实，勿读成「已覆盖」）
 *
 * - 票面字段里那次「中间丢行」在合成流上**复现不了**（本机实测：子进程退出即关写端，300 行含中间标记行
 *   全量返回）—— 实证复现的缺陷是**读段无界**（上文 41137 ms）。门端到端（真跑 publish）需要 HBuilderX
 *   的本地 IPC，受限会话跑不了（与 #1272 守护记的边界同类）⇒ 字段那次丢行的机制由票面字节级记录承担；
 *   本守护锁的是**采集契约**：读段必须有界（排不完 ⇒ TimedOut ⇒ 门 reason=timeout，fail-closed）、
 *   多行必须收全、超时必须保持。
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

// 单个用例真起子进程 + 真等超时预算（最坏 ~14 s），默认 5 s 一定不够
jest.setTimeout(120000);

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
function drive(libAbs, statements, timeout = 90000) {
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
      '驱动采集层失败（pwsh 不可用或库抛错，fail-closed 不跳过）：\n'
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

/** 子脚本写到临时目录；首行钉 UTF-8 —— 与真实 CLI 一致（管道上输出 UTF-8，票面字节级记录） */
function writeChild(name, body) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-cap-'));
  const file = path.join(dir, name);
  fs.writeFileSync(file, ['[Console]::OutputEncoding = [System.Text.Encoding]::UTF8'].concat(body).join('\n'), 'utf8');
  return file;
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
    '$sw = [System.Diagnostics.Stopwatch]::StartNew()',
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(child)}) -TimeoutSeconds 10 -Tag 'CAP'`,
    '$sw.Stop()',
    "$ln = ([regex]::Matches($r.Output, '(?m)^LINE-\\d+\\r?$')).Count",
    "$mid = $r.Output -match 'MIDDLE-MARKER.*导出 android 成功'",
    "$last = $r.Output -match 'LAST-MARKER.*导出 android 成功'",
    `Write-Output ("${key}=" + $sw.ElapsedMilliseconds + "|" + $r.TimedOut + "|" + $r.ExitCode + "|" + $ln + "|" + $mid + "|" + $last)`,
  ];
}

function tmoStatements(key) {
  return [
    '$pwsh = (Get-Process -Id $PID).Path',
    '$sw = [System.Diagnostics.Stopwatch]::StartNew()',
    "$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-Command','Start-Sleep 60') -TimeoutSeconds 3 -Tag 'TMO'",
    '$sw.Stop()',
    `Write-Output ("${key}=" + $sw.ElapsedMilliseconds + "|" + $r.TimedOut + "|" + $r.Output.Substring(0, 9))`,
  ];
}

/** 子脚本秒退、但后代进程继承 stdout 并握 12 秒 —— 读段必须在预算内收口（否则就是旧实现的无界阻塞） */
const HOLD_BODY = [
  "Write-Output 'BEFORE-HOLD 导出 android 成功'",
  '$psi = [System.Diagnostics.ProcessStartInfo]::new()',
  "$psi.FileName = 'pwsh'",
  "$psi.Arguments = '-NoProfile -Command Start-Sleep 12'",
  '$psi.UseShellExecute = $false',
  '[void][System.Diagnostics.Process]::Start($psi)',
  "Write-Output 'AFTER-HOLD'",
  'exit 0',
];

function holdStatements(child, key) {
  return [
    '$pwsh = (Get-Process -Id $PID).Path',
    '$sw = [System.Diagnostics.Stopwatch]::StartNew()',
    `$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',${q(child)}) -TimeoutSeconds 4 -Tag 'HOLD'`,
    '$sw.Stop()',
    `Write-Output ("${key}=" + $sw.ElapsedMilliseconds + "|" + $r.TimedOut + "|" + $r.Output.Substring(0, 9))`,
  ];
}

// ===== ① 多行 + 末行后立刻退出 ⇒ 收全（票面期望 c 的字面要求）=====

describe('采集契约：多行 + 末行后立刻退出 ⇒ 收全（期望 c 的合成流）', () => {
  it('必不红半：300 行（中间含标记行、末行也是标记行）全部收齐，退出码 0、不超时', () => {
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(REAL_LIB, capStatements(child, 'CAP'));
    const [elapsed, timedOut, exitCode, lines, mid, last] = pick(out, 'CAP').split('|');
    expect(timedOut).toBe('False');
    expect(exitCode).toBe('0');
    expect(lines).toBe('300'); // 中间行掉了就在这里红（票面丢行形态）
    expect(mid).toBe('True');
    expect(last).toBe('True');
    expect(Number(elapsed)).toBeLessThan(9000);
  });
});

// ===== ② 超时语义保持（票面期望 a：超时仍走 reason=timeout 那个出口）=====

describe('采集契约：超时语义保持（期望 a：超时仍 TimedOut=true ⇒ 门 reason=timeout）', () => {
  it('必不红半：子进程睡过预算 ⇒ 在预算附近返回、TimedOut=True、输出以 [timeout] 开头', () => {
    const out = drive(REAL_LIB, tmoStatements('TMO'));
    const [elapsed, timedOut, head] = pick(out, 'TMO').split('|');
    expect(timedOut).toBe('True');
    expect(head).toBe('[timeout]');
    expect(Number(elapsed)).toBeLessThan(8000);
  });
});

// ===== ③ 读段有界（票面期望 a 的本体：总时长受 TimeoutSeconds 约束）=====

describe('采集契约：读段有界（期望 a：总时长受 TimeoutSeconds 约束，排不完 ⇒ TimedOut）', () => {
  it('必红半（旧实现实测 41137ms/预算 4000ms）：子脚本秒退、后代握 stdout ⇒ 总耗时仍在预算附近且 TimedOut=True', () => {
    const child = writeChild('child-hold.ps1', HOLD_BODY);
    const out = drive(REAL_LIB, holdStatements(child, 'HOLD'));
    const [elapsed, timedOut, head] = pick(out, 'HOLD').split('|');
    // 旧实现：子脚本退出后 WaitForExit 即返回 true，随后 .Result 无界阻塞到后代松开管道 ⇒ 越界且 False
    expect(timedOut).toBe('True');
    expect(head).toBe('[timeout]');
    expect(Number(elapsed)).toBeLessThan(8000);
  });
});

// ===== ④ 成对取证（必红）：把库注入变异，证明上面的断言有牙 =====

describe('成对取证（必红）：把采集层注入变异，证明上面的断言有牙', () => {
  it('变异「收全」（丢掉 stdout 那半）⇒ 300 行变 0、两处标记行全丢（① 的断言随之判红）', () => {
    const broken = mutatedLib([['$out = $stdout.Result + $stderr.Result', '$out = $stderr.Result']]);
    const child = writeChild('child-lines.ps1', LINES_BODY);
    const out = drive(broken, capStatements(child, 'MUT'));
    const [, , , lines, mid, last] = pick(out, 'MUT').split('|');
    expect(lines).toBe('0');
    expect(mid).toBe('False');
    expect(last).toBe('False');
  });

  it('变异「有界排空」（WaitAll 恒真 ⇒ 退回无界 .Result）⇒ 总耗时越界且丢超时标志（③ 的断言随之判红）', () => {
    const broken = mutatedLib([[
      '$drained = [System.Threading.Tasks.Task]::WaitAll([System.Threading.Tasks.Task[]]@($stdout, $stderr), $remainingMs)',
      '$drained = $true',
    ]]);
    const child = writeChild('child-hold.ps1', HOLD_BODY);
    const out = drive(broken, holdStatements(child, 'MUT'));
    const [elapsed, timedOut] = pick(out, 'MUT').split('|');
    expect(timedOut).toBe('False');
    expect(Number(elapsed)).toBeGreaterThan(8000);
  });

  it('变异「超时标志」⇒ 超时场景 TimedOut=False（② 的断言随之判红）', () => {
    const broken = mutatedLib([[
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true',
      '"[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $false',
    ]]);
    const out = drive(broken, tmoStatements('MUT'));
    const [, timedOut] = pick(out, 'MUT').split('|');
    expect(timedOut).toBe('False');
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
