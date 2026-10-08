/**
 * `emulator-smoke.ps1` 的文本收口点 `Get-AdbOutput` 的**有界性**运行期守护（#1568，2026-10-08）
 *
 * 为什么需要（票面 #1568 现测）：`Get-AdbOutput` 是 `& $AdbExe -s $Serial @AdbArgs 2>&1 | Out-String`
 * —— 前台同步等一次 adb、**没有单次超时**，而它一个点就压着 **12 个调用方**（`sys.boot_completed` 轮询、
 * `settings put`、`input keyevent`、`getprop`、`pidof`、`am start` / `am force-stop`）。
 * 一次不返回就既不产结论也不报错，后面的页根本跑不到 —— 与 #1560/#1562 已修的三处同源，只是通道是文本。
 *
 * 本套件钉的是**调用点这一侧**（断言产物 = 落盘的 `emulator-smoke.log` 与返回值，不断言源码文本）：
 *   ETD1 真执行核 + 真挂死桩 ⇒ 调用点在预算内给结论、回空串、日志点名哪一次调用与多大预算
 *   ETD2 对照腿：正常返回 ⇒ 文本照旧回给调用方，且 stdout 与 stderr **都在**（`2>&1` 的语义不许在收口时丢掉）
 *   ETD3 非零退出 ⇒ 仍回 adb 的原文（旧形状在 `$ErrorActionPreference='Stop'` 下会把非零退出抛出去；
 *        收口后不许抛 —— 抛了调用方就拿不到「设备答了什么」这条信息）
 *   ETD4 预算是**参数**：换一个预算就换一个上界（默认值写在 `param()`，档位按调用形态分）
 *
 * ⚠️ 判别力实测（M 系列读数记在 PR 的 `## 验收证据` 与 `docs/verification/tooling/1568/`）：
 *   把 `Get-AdbOutput` 换回旧的无界形状 ⇒ ETD1/ETD4 红；去掉 stderr 合并 ⇒ ETD2 红。
 *
 * ⚠️ 手法沿用本仓先例：AST 抽出脚本里的函数定义后**直调**，不执行整篇门脚本（整篇跑会真起模拟器 + 装基座）；
 *   执行核用**真的那一份**（dot-source `lib/auto-screenshot.ps1`），不套影子 —— 有界不是影子造出来的。
 *   判据 token 全 ASCII（pwsh 的中文在 OEM 码页下到 Node 侧会变乱码 —— 本仓血账）。
 *
 * ⚠️ 墙钟口径（#1549）：判据只取定性值与顺序值；`*_MS` 上界只用来证明「没挂住」，给足余量。
 *
 * 运行前提：需要 `pwsh`。不可用时 **fail-closed 抛错，不 skip**。
 *   与 #1562 那两条截屏腿不同，本套件的桩是**平台自配的载体**（Windows `.cmd` / Unix `.sh`），
 *   非 Windows 走执行核的直启分支 ⇒ **两条平台都真挂死、真到点**（CI 的 ubuntu 上这一格不是空转腿，
 *   先例教训：#1562 的 `B12` 曾被 CI 照出「那一格在 ubuntu 上物理造不出来」）。
 *
 * 在册性：`scripts/lib/contract-tests.ps1` 的 pattern 需含 `emulatorSmokeBoundedText`，
 *   否则 token 收窄的门里这一组真执行腿**永不执行**（判据与理由见该文件头部与 #1562 的同款裁定）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const IS_WIN = process.platform === 'win32';
// 真挂死腿按预算到点（默认预算 2 / 5 秒）+ 真启动器开销 ⇒ 一条探针就要跑十几秒，默认 5 秒拦不住
// （先例 utils/hxLaunchDetachBehavior.test.js:47、utils/evidenceGenContract.test.js:26）。
jest.setTimeout(180000);
const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'emulator-smoke.ps1');
const LIB = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (IS_WIN) args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

/**
 * 挂死桩 / 正常桩 / 非零退出桩：前两条都往 stdout 与 stderr **各**写一行，用来同时验「有界」与「`2>&1` 语义没丢」。
 * 形态与 utils/emulatorSmokeBoundedShotBehavior.test.js 的 ESD1 桩同源（真 cmd + 真到点），
 * 差别只在：这一族的输出是文本，且必须两条平台都能真跑 ⇒ 载体按平台落 `.cmd` / `.sh`
 * （Linux 上走执行核的直启分支，那一格在 CI 里照样真挂死、真到点 —— 不是静默跳过的空转腿）。
 */
function writeStub(dir, name, mode) {
  const L = [];
  L.push(IS_WIN ? '@echo off' : '#!/bin/sh');
  if (mode === 'fail') {
    L.push('echo ERROR-ON-STDERR 1>&2');
    L.push(IS_WIN ? 'exit /b 1' : 'exit 1');
  } else {
    L.push('echo STDOUT-MARKER');
    L.push('echo STDERR-MARKER 1>&2');
    if (mode === 'hang') {
      // ⚠️ 挂死载体刻意**不用 `ping`**：`utils/autoScreenshotStabilityBehavior.test.js` 的 B6 用
      // 「全局 `Get-Process ping` 计数」当杀树判据，同一趟 jest 里任何别的 ping 都会把它顶红
      // （2026-10-08 现测：本套件用 ping 当桩时 B6 收到 1；换成 node 常驻后 B6 回到 0）。
      // node 本来就是本仓测试的执行体，不额外引依赖。
      L.push(IS_WIN ? 'node -e "setTimeout(()=>{},120000)"' : 'sleep 120');
    } else {
      L.push(IS_WIN ? 'exit /b 0' : 'exit 0');
    }
  }
  const file = path.join(dir, name + (IS_WIN ? '.cmd' : '.sh'));
  fs.writeFileSync(file, L.join(IS_WIN ? '\r\n' : '\n') + (IS_WIN ? '\r\n' : '\n'), 'utf8');
  if (!IS_WIN) fs.chmodSync(file, 0o755);
  return file.replace(/\\/g, '/');
}

function probeText(ctx) {
  const stubHang = writeStub(ctx.tmp, 'stub-hang', 'hang');
  const stubOk = writeStub(ctx.tmp, 'stub-ok', 'ok');
  const stubFail = writeStub(ctx.tmp, 'stub-fail', 'fail');
  const script = `
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
# 真执行核先加载 —— ETD1/ETD2 跑**真的**那一份，不是影子
. "${LIB}"
$src = Get-Content -LiteralPath "${SCRIPT}" -Raw
$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)
$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))
foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }
Write-Output ("EXTRACTED=" + $fns.Count)

# ── 脚本级前置：与脚本顶部状态块同形（StrictMode 下「先声明再读」是硬要求）
$dir = '${ctx.tmp}'
$VerifyDir = $dir
$LogPath = Join-Path $dir 'emulator-smoke.log'
$AdbExe = 'unused'
$Serial = 'emulator-5554'
$AdbCallTimeoutSeconds = 15
$AdbTextCallTimeoutSeconds = 2
$script:AdbTextTimeouts = @()
$script:AdbTextCalls = 0
$script:ShotTimeouts = @()
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding utf8
function LogField([string]$Key) {
  $raw = Get-Content -LiteralPath $LogPath -Raw -Encoding UTF8
  $m = [regex]::Match($raw, ('(?m)^.*{0}=(\\S+).*\$' -f [regex]::Escape($Key)))
  return $(if ($m.Success) { $m.Groups[1].Value } else { 'none' })
}
function LogHas([string]$Pattern) {
  $raw = Get-Content -LiteralPath $LogPath -Raw -Encoding UTF8
  return [bool]($raw -match $Pattern)
}

Write-Output ("IS_WINDOWS=" + [bool]$IsWindows)

# ── ETD1：真启动器 + 真挂死桩 ⇒ 调用点在预算内给结论，回空串，日志点名哪一次调用与多大预算
$AdbExe = '${stubHang}'
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding utf8
$script:AdbTextTimeouts = @()
$script:AdbTextCalls = 0
$sw1 = [System.Diagnostics.Stopwatch]::StartNew()
$t1 = Get-AdbOutput @('shell', 'getprop', 'sys.boot_completed')
$sw1.Stop()
Write-Output ("ETD1_EMPTY=" + [bool]([string]$t1 -eq ''))
Write-Output ("ETD1_LOG_NAMES_CALL=" + $(LogHas 'ADB_TEXT_TIMEOUT call='))
Write-Output ("ETD1_LOG_NAMES_BUDGET=" + $(LogHas 'callBudgetSeconds=2'))
Write-Output ("ETD1_LOG_NAMES_ARGS=" + $(LogHas 'sys\\.boot_completed'))
Write-Output ("ETD1_ACCUMULATOR=" + (@($script:AdbTextTimeouts) -join ','))
Write-Output ("ETD1_MS=" + $sw1.ElapsedMilliseconds)

# ── ETD2：对照腿，正常返回 ⇒ 文本照旧回、stdout 与 stderr 都在、不误判成超时
$AdbExe = '${stubOk}'
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding utf8
$script:AdbTextTimeouts = @()
$t2 = Get-AdbOutput @('shell', 'getprop', 'ro.product.model')
Write-Output ("ETD2_STDOUT=" + [bool]("$t2" -match 'STDOUT-MARKER'))
Write-Output ("ETD2_STDERR=" + [bool]("$t2" -match 'STDERR-MARKER'))
Write-Output ("ETD2_TRIMMED=" + [bool]("$t2" -eq ("$t2".Trim())))
Write-Output ("ETD2_NO_TIMEOUT_LOG=" + [bool](-not (LogHas 'ADB_TEXT_TIMEOUT')))
Write-Output ("ETD2_ACCUMULATOR_EMPTY=" + [bool](@($script:AdbTextTimeouts).Count -eq 0))

# ── ETD3：非零退出 ⇒ 仍把 adb 原文回给调用方，且**不抛**（旧形状在 EAP=Stop 下会抛 NativeCommandError）
$AdbExe = '${stubFail}'
$threw = $false
$t3 = ''
try { $t3 = Get-AdbOutput @('shell', 'pidof', 'io.dcloud.HBuilder') } catch { $threw = $true }
Write-Output ("ETD3_THREW=" + [bool]$threw)
Write-Output ("ETD3_HAS_STDERR_TEXT=" + [bool]("$t3" -match 'ERROR-ON-STDERR'))

# ── ETD4：预算是**参数** —— 换一个预算就换一个上界（档位按调用形态分，写死在函数体里就红）
$AdbExe = '${stubHang}'
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding utf8
$script:AdbTextTimeouts = @()
$sw4 = [System.Diagnostics.Stopwatch]::StartNew()
$null = Get-AdbOutput @('shell', 'echo', 'budget-five') -BudgetSeconds 5
$sw4.Stop()
Write-Output ("ETD4_LOG_BUDGET_FIVE=" + $(LogHas 'callBudgetSeconds=5'))
Write-Output ("ETD4_DEFAULT_NOT_USED=" + $(if ((LogField 'callBudgetSeconds') -eq '5') { 'True' } else { 'False' }))
Write-Output ("ETD4_MS=" + $sw4.ElapsedMilliseconds)

Write-Output ("CALLS_COUNTER=" + $(LogHas 'ADB_TEXT_CALL_BUDGET'))
Write-Output "EMUTEXT_PROBE_DONE=1"
`.trim();

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      // 挂死腿的桩睡 120 秒 ⇒ 这一格的上界就是「探针不许跟着睡」：改造前它 60 秒到点被杀（红），
      // 改造后到点自己收口（绿，实测十几秒）。
      timeout: 60000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

describe('emulator-smoke 文本收口点 Get-AdbOutput 的有界性（运行期，#1568）', () => {
  let ctx;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'emutext-')) };
    probe = probeText(ctx);
    if (probe.ok) {
      [
        'IS_WINDOWS', 'EXTRACTED', 'EMUTEXT_PROBE_DONE',
        'ETD1_EMPTY', 'ETD1_LOG_NAMES_CALL', 'ETD1_LOG_NAMES_BUDGET', 'ETD1_LOG_NAMES_ARGS', 'ETD1_ACCUMULATOR', 'ETD1_MS',
        'ETD2_STDOUT', 'ETD2_STDERR', 'ETD2_TRIMMED', 'ETD2_NO_TIMEOUT_LOG', 'ETD2_ACCUMULATOR_EMPTY',
        'ETD3_THREW', 'ETD3_HAS_STDERR_TEXT',
        'ETD4_LOG_BUDGET_FIVE', 'ETD4_DEFAULT_NOT_USED', 'ETD4_MS', 'CALLS_COUNTER',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
    }
  });

  afterAll(() => {
    if (ctx) {
      try { fs.rmSync(ctx.tmp, { recursive: true, force: true }); } catch { /* 清理失败不判红 */ }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      'Get-AdbOutput 有界性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  test('ETD1: 真挂死桩下调用点在预算内给结论、回空串，日志点名哪一次调用与多大预算', () => {
    requireProbe();
    expect(Number(v.EXTRACTED)).toBeGreaterThanOrEqual(20);
    expect(v.ETD1_EMPTY).toBe('True');
    expect(v.ETD1_LOG_NAMES_CALL).toBe('True');
    expect(v.ETD1_LOG_NAMES_BUDGET).toBe('True');
    expect(v.ETD1_LOG_NAMES_ARGS).toBe('True');
    expect(v.ETD1_ACCUMULATOR).toBe('shell getprop sys.boot_completed');
    // 只证「没挂住」，不是判据（墙钟口径 #1549）：预算 2 秒给了 15 倍余量
    expect(Number(v.ETD1_MS)).toBeLessThanOrEqual(30000);
  });

  test('ETD2: 对照腿正常返回时文本照旧回给调用方，stdout 与 stderr 都在（2>&1 语义不许在收口时丢掉）', () => {
    requireProbe();
    expect(v.ETD2_STDOUT).toBe('True');
    expect(v.ETD2_STDERR).toBe('True');
    expect(v.ETD2_TRIMMED).toBe('True');
    expect(v.ETD2_NO_TIMEOUT_LOG).toBe('True');
    expect(v.ETD2_ACCUMULATOR_EMPTY).toBe('True');
  });

  test('ETD3: adb 非零退出仍回原文且不抛（收口后调用方拿得到「设备答了什么」）', () => {
    requireProbe();
    expect(v.ETD3_THREW).toBe('False');
    expect(v.ETD3_HAS_STDERR_TEXT).toBe('True');
  });

  test('ETD4: 单次预算是可调参数 —— 给 5 秒就用 5 秒，日志点的预算跟着变', () => {
    requireProbe();
    expect(v.ETD4_LOG_BUDGET_FIVE).toBe('True');
    expect(v.ETD4_DEFAULT_NOT_USED).toBe('True');
    expect(Number(v.ETD4_MS)).toBeLessThanOrEqual(40000);
  });

  test('ETD5: 探针跑完且全套件非空转（每条断言都真取到了读数）', () => {
    requireProbe();
    expect(v.EMUTEXT_PROBE_DONE).toBe('1');
    Object.keys(v).forEach((k) => expect(v[k]).not.toBeNull());
  });
});
