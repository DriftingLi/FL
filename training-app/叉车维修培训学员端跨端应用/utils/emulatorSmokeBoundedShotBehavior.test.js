/**
 * `emulator-smoke.ps1`（仿真机前置冒烟）那张截图的**有界性**运行期守护（#1562，2026-10-07）
 *
 * 为什么需要（票面现测，同族缺陷的第三个落点）：它的 `Export-Screenshot` 与 `device-capture.ps1`
 * **逐字同形** —— `& cmd.exe /c "adb … exec-out screencap -p > file" 2>&1 | Out-Null`，
 * 前台同步等待、没有单次超时。#1560 已证明这个形状偶发但真实（同机同时刻手工 `screencap`
 * 4.8 秒返回，而链里那一次 32 分钟没回来）⇒ 一次不返回，整条冒烟就地停住。
 * 「同一份有界单次调用」在本仓需要三个消费点，本票把 2/3 两个落点也接到那一份上。
 *
 * 本套件钉的是**调用点这一侧**（断言产物，不断言源码文本）：
 *   ESD1 真执行核 + 真挂死桩 ⇒ 调用点在预算内给结论（有界不是影子造出来的）
 *   ESD2 超时**且残帧留盘** ⇒ 最终名不许有文件（归位靠 `.part` 命名，不靠删除成功）
 *   ESD3 对照腿：正常非空帧 ⇒ 出图判据照旧成立（headless 那张图本来就是出图判据，不许被放宽）
 *   ESD4 一页超时后**第二页照旧跑**（票面「该页记失败并继续」）
 *   ESD5 出口不混：「没有帧」不得被谎报成「通道不返回」
 *
 * ⚠️ 判别力实测（#1562 收口会话，见 PR 正文）：把 `Export-Screenshot` 换回旧的无界形状
 *   ⇒ ESD1/ESD2/ESD4 红；去掉 `.part` 归位直接写最终名 ⇒ ESD2 红。
 *
 * ⚠️ 手法沿用本仓先例：AST 抽出脚本里的函数定义后**直调**，不执行整篇脚本
 *   （整篇跑会真起模拟器 + 装基座）。影子替换靠「函数名在调用时解析」。
 *   判据 token 全 ASCII（pwsh 的中文在 OEM 码页下到 Node 侧会变乱码 —— 本仓血账）。
 *
 * ⚠️ 墙钟口径（#1549）：判据只取定性值与顺序值；`*_MS` 上界只用来证明「没挂住」，给 30 倍余量。
 *
 * 运行前提：需要 `pwsh`。不可用时 **fail-closed 抛错，不 skip**。
 *
 * 在册性（票面 AC5 现测后的裁定）：`scripts/lib/contract-tests.ps1` 原先**没有** `emulatorSmoke` token
 *   （现测 grep 0 命中），本票**补上了**。判据不是偏好而是该文件头部写明的那条：「新增运行期守护时
 *   必须把 token 加进本函数，否则该守护**永不执行**」—— 本套件属行为守护（门 3 承重那一类，
 *   见 `docs/agents/guards.md`），只在「全量」那一档跑而 token 收窄的门静默跳过，正是要防的假绿形态。
 *   时序：该文件此前正被 open PR #1561（#1543）改着 ⇒ 等它合入 master（`65d25f4f`）后才动它。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'emulator-smoke.ps1');
const LIB = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

function probeShot(ctx) {
  const script = `
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
# 真执行核先加载 —— ESD1 跑**真的**那一份，不是影子
. "${LIB}"
$src = Get-Content -LiteralPath "${SCRIPT}" -Raw
$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)
$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))
foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }
Write-Output ("EXTRACTED=" + $fns.Count)

$dir = '${ctx.tmp}'
# 脚本级前置：与脚本顶部状态块同形（StrictMode 下「先声明再读」是硬要求）
$VerifyDir = $dir
$AdbExe = 'unused'
$AdbCallTimeoutSeconds = 2
$Serial = 'emulator-5554'
$script:ShotTimeouts = @()
$script:Log = @()
function Write-Log { param([string]$Message) $script:Log = @($script:Log) + @($Message) }
function LogHit([string]$Pattern) {
  return [bool](@($script:Log | Where-Object { $_ -match $Pattern }).Count -ge 1)
}

Write-Output ("IS_WINDOWS=" + [bool]$IsWindows)

# ── ESD1：真启动器 + 真挂死桩 ⇒ 调用点在预算内给结论
$hang = Join-Path $dir 'hang-emu.cmd'
Set-Content -LiteralPath $hang -Encoding ascii -Value @('@echo off', 'echo PARTIAL-FRAME', 'ping -n 120 127.0.0.1 > nul')
$AdbExe = $hang
$script:Log = @()
$script:ShotTimeouts = @()
$sw1 = [System.Diagnostics.Stopwatch]::StartNew()
$r1 = Export-Screenshot -Page 'pages/probe/hang'
$sw1.Stop()
Write-Output ("ESD1_TIMEDOUT=" + [bool]$r1.TimedOut)
Write-Output ("ESD1_BYTES=" + $r1.Bytes)
Write-Output ("ESD1_NAME=" + $r1.Name)
Write-Output ("ESD1_FINAL_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'emulator-pages-probe-hang.png'))))
Write-Output ("ESD1_LOG_NAMES_CALL=" + $(LogHit 'SHOT_CALL_TIMEOUT page=emulator-pages-probe-hang\\.png callBudgetSeconds=2'))
Write-Output ("ESD1_MS=" + $sw1.ElapsedMilliseconds)

# ── 影子执行核
function Invoke-BoundedAdbShot {
    param([string]$AdbExe, [string]$Serial, [string]$OutFile, [int]$TimeoutSeconds = 15)
    if ($script:shadow -eq 'timeout_with_frame') {
        # 真链路现测到的竞争形状：被判超时，而残帧**就留在盘上**
        Set-Content -LiteralPath $OutFile -Encoding ascii -Value 'PARTIAL-FRAME-BYTES'
        return [pscustomobject]@{ TimedOut = $true; Exited = $false; ExitCode = -1; Seconds = [double]$TimeoutSeconds; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = "未在 $TimeoutSeconds 秒内返回（桩）" }
    }
    if ($script:shadow -eq 'ok') {
        Set-Content -LiteralPath $OutFile -Encoding ascii -Value 'FULL-FRAME-BYTES-LONGER-THAN-PARTIAL'
        return [pscustomobject]@{ TimedOut = $false; Exited = $true; ExitCode = 0; Seconds = 0.4; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = '' }
    }
    return [pscustomobject]@{ TimedOut = $false; Exited = $true; ExitCode = 1; Seconds = 0.1; OutFile = $OutFile; ErrFile = ''; ErrTail = 'error: device offline'; Error = '' }
}
$AdbExe = 'stub'

# ── ESD2：超时 + 残帧留盘 ⇒ 最终名不写文件
$script:shadow = 'timeout_with_frame'
$script:Log = @()
$script:ShotTimeouts = @()
$r2 = Export-Screenshot -Page 'pages/probe/a'
Write-Output ("ESD2_TIMEDOUT=" + [bool]$r2.TimedOut)
Write-Output ("ESD2_BYTES=" + $r2.Bytes)
Write-Output ("ESD2_FINAL_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'emulator-pages-probe-a.png'))))
$pb = [regex]::Match((($script:Log) -join ' '), 'partBytes=(\\d+)')
Write-Output ("ESD2_PARTBYTES=" + $(if ($pb.Success) { $pb.Groups[1].Value } else { 'none' }))
Write-Output ("ESD2_LOG_NAMES_BUDGET=" + $(LogHit 'callBudgetSeconds=2'))
Write-Output ("ESD2_ACCUMULATOR=" + (@($script:ShotTimeouts) -join ','))

# ── ESD3 对照腿：正常非空帧 ⇒ 出图判据照旧成立
$script:shadow = 'ok'
$script:Log = @()
$script:ShotTimeouts = @()
$r3 = Export-Screenshot -Page 'pages/probe/b'
Write-Output ("ESD3_TIMEDOUT=" + [bool]$r3.TimedOut)
Write-Output ("ESD3_BYTES_GT0=" + [bool]($r3.Bytes -gt 0))
Write-Output ("ESD3_FINAL_EXISTS=" + [bool](Test-Path -LiteralPath (Join-Path $dir 'emulator-pages-probe-b.png') -PathType Leaf))
Write-Output ("ESD3_PART_GONE=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'emulator-pages-probe-b.png.part'))))
Write-Output ("ESD3_NO_TIMEOUT_LOG=" + [bool](-not (LogHit 'SHOT_CALL_TIMEOUT')))

# ── ESD4：一页超时后第二页照旧跑
$script:shadow = 'timeout_with_frame'
$script:Log = @()
$script:ShotTimeouts = @()
$c1 = Export-Screenshot -Page 'pages/probe/c1'
$script:shadow = 'ok'
$c2 = Export-Screenshot -Page 'pages/probe/c2'
Write-Output ("ESD4_FIRST_TIMEOUT=" + [bool]$c1.TimedOut)
Write-Output ("ESD4_SECOND_BYTES_GT0=" + [bool]($c2.Bytes -gt 0))
Write-Output ("ESD4_SECOND_FINAL_EXISTS=" + [bool](Test-Path -LiteralPath (Join-Path $dir 'emulator-pages-probe-c2.png') -PathType Leaf))
Write-Output ("ESD4_ACCUMULATOR=" + (@($script:ShotTimeouts) -join ','))

# ── ESD5：没有帧 ≠ 超时
$script:shadow = 'noframe'
$script:Log = @()
$script:ShotTimeouts = @()
$r5 = Export-Screenshot -Page 'pages/probe/d'
Write-Output ("ESD5_TIMEDOUT=" + [bool]$r5.TimedOut)
Write-Output ("ESD5_BYTES=" + $r5.Bytes)
Write-Output ("ESD5_LOG_HAS_STDERR=" + $(LogHit 'device offline'))
Write-Output ("ESD5_NO_TIMEOUT_LOG=" + [bool](-not (LogHit 'SHOT_CALL_TIMEOUT')))

Write-Output ("STRAY_PNG=" + @(Get-ChildItem -LiteralPath $dir -Filter '*.png' | Where-Object { $_.Name -notmatch 'probe-b|probe-c2' }).Count)
Write-Output "EMUSHOT_PROBE_DONE=1"
`.trim();

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 180000,
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

describe('emulator-smoke 截图调用的有界性（运行期，#1562）', () => {
  let ctx;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'emushot-')) };
    probe = probeShot(ctx);
    if (probe.ok) {
      [
        'IS_WINDOWS', 'EXTRACTED', 'EMUSHOT_PROBE_DONE',
        'ESD1_TIMEDOUT', 'ESD1_BYTES', 'ESD1_NAME', 'ESD1_FINAL_ABSENT', 'ESD1_LOG_NAMES_CALL', 'ESD1_MS',
        'ESD2_TIMEDOUT', 'ESD2_BYTES', 'ESD2_FINAL_ABSENT', 'ESD2_PARTBYTES', 'ESD2_LOG_NAMES_BUDGET', 'ESD2_ACCUMULATOR',
        'ESD3_TIMEDOUT', 'ESD3_BYTES_GT0', 'ESD3_FINAL_EXISTS', 'ESD3_PART_GONE', 'ESD3_NO_TIMEOUT_LOG',
        'ESD4_FIRST_TIMEOUT', 'ESD4_SECOND_BYTES_GT0', 'ESD4_SECOND_FINAL_EXISTS', 'ESD4_ACCUMULATOR',
        'ESD5_TIMEDOUT', 'ESD5_BYTES', 'ESD5_LOG_HAS_STDERR', 'ESD5_NO_TIMEOUT_LOG', 'STRAY_PNG',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
    }
  });

  afterAll(() => {
    if (ctx) {
      try {
        fs.rmSync(ctx.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红
      }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      'emulator-smoke 截图有界性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  test('ESD1: 真挂死桩下调用点在预算内给结论，最终名不留文件，日志点名哪一次调用', () => {
    requireProbe();
    expect(Number(v.EXTRACTED)).toBeGreaterThanOrEqual(20);
    expect(v.ESD1_NAME).toBe('emulator-pages-probe-hang.png');
    expect(v.ESD1_BYTES).toBe('0');
    expect(v.ESD1_FINAL_ABSENT).toBe('True');
    if (v.IS_WINDOWS === 'True') {
      expect(v.ESD1_TIMEDOUT).toBe('True');
      expect(v.ESD1_LOG_NAMES_CALL).toBe('True');
      expect(Number(v.ESD1_MS)).toBeLessThanOrEqual(60000); // 只证没挂住，不是判据
    } else {
      // 起不了 cmd.exe ⇒ 走「没有帧」出口、同样不许挂住；「日志点名哪一次超时」在这一支根本不成立
      expect(v.ESD1_TIMEDOUT).toBe('False');
      expect(Number(v.ESD1_MS)).toBeLessThanOrEqual(60000);
    }
  });

  test('ESD2: 超时且残帧确实留盘 ⇒ 最终名不写文件（归位靠命名、不靠删除成功）', () => {
    requireProbe();
    expect(v.ESD2_TIMEDOUT).toBe('True');
    expect(v.ESD2_BYTES).toBe('0');
    expect(v.ESD2_FINAL_ABSENT).toBe('True');
    // 空转证明：残帧确实留在盘上（partBytes 由 PS 侧从日志里读出），否则「最终名不在」是恒真断言
    expect(Number(v.ESD2_PARTBYTES)).toBeGreaterThan(0);
    expect(v.ESD2_LOG_NAMES_BUDGET).toBe('True');
    expect(v.ESD2_ACCUMULATOR).toBe('pages/probe/a');
  });

  test('ESD3: 对照腿正常返回时出图判据照旧成立（headless 那张图不因收口被放宽）', () => {
    requireProbe();
    expect(v.ESD3_TIMEDOUT).toBe('False');
    expect(v.ESD3_BYTES_GT0).toBe('True');
    expect(v.ESD3_FINAL_EXISTS).toBe('True');
    expect(v.ESD3_PART_GONE).toBe('True');
    expect(v.ESD3_NO_TIMEOUT_LOG).toBe('True');
  });

  test('ESD4: 一页超时后第二页照样出图，超时清单只点名第一页', () => {
    requireProbe();
    expect(v.ESD4_FIRST_TIMEOUT).toBe('True');
    expect(v.ESD4_SECOND_BYTES_GT0).toBe('True');
    expect(v.ESD4_SECOND_FINAL_EXISTS).toBe('True');
    expect(v.ESD4_ACCUMULATOR).toBe('pages/probe/c1');
  });

  test('ESD5: 没有帧不谎报成超时，且把 adb stderr 带上；证据目录无游离 PNG', () => {
    requireProbe();
    expect(v.ESD5_TIMEDOUT).toBe('False');
    expect(v.ESD5_BYTES).toBe('0');
    expect(v.ESD5_LOG_HAS_STDERR).toBe('True');
    expect(v.ESD5_NO_TIMEOUT_LOG).toBe('True');
    expect(v.STRAY_PNG).toBe('0');
    expect(v.EMUSHOT_PROBE_DONE).toBe('1');
  });
});
