/**
 * `device-capture.ps1`（①a 真机只读取证）那张截图的**有界性**运行期守护（#1562，2026-10-07）
 *
 * 为什么需要（票面现测，同族缺陷的第二个落点）：`Export-Screenshot` 原先是
 *   `& cmd.exe /c "adb … exec-out screencap -p > file" 2>&1 | Out-Null`
 * —— 前台同步等待、**没有单次超时**。#1560 已经证明这个形状是**偶发但真实**的
 * （同机同时刻手工 `adb exec-out screencap` 4.8 秒返回，而链里那一次 32 分钟没回来）
 * ⇒ 「设备健康」不构成豁免理由。挂住时这条取证链**既不产帧也不报错**，后面的页也跑不到 ——
 * 而 ①a 的全部产物就是「逐页的图 + 一行机检结论」，就地停住等于无声地交不了活。
 *
 * 现在它复用 `lib/auto-screenshot.ps1` 那份有界执行核（唯一真源，本票刻意**不**复制第二份）。
 * 本套件钉的是**调用点这一侧**的三件事，全都断言**产物**而不是源码文本：
 *   BSD1 真执行核 + 真「写半帧就永不返回」的桩 ⇒ 调用点在预算内给出结论（有界不是影子造出来的）
 *   BSD2 超时**且残帧确实留在盘上** ⇒ 最终名上不许有文件（归位靠 `.part` 命名兑现，不靠删除成功）
 *   BSD3 对照腿：正常返回的非空帧 ⇒ 归位、算 sha、不产出任何超时机检行（证明 BSD2 不是「逢截必红」）
 *   BSD4 一页超时之后**后面的页照旧跑**：第二页出图归位，超时清单只点名第一页
 *   BSD5 出口不混：拿到「没有帧」时不得谎报成超时（`TimedOut=False` + `Bytes=0`），并把 adb stderr 带上
 *
 * ⚠️ 判别力实测（#1562 收口会话）：把 `Export-Screenshot` 换回旧的无界形状（`& cmd.exe /c "… > 最终名"`）
 *   ⇒ BSD1/BSD2/BSD4 同时红；只删掉 `.part` 归位、直接往最终名写 ⇒ BSD2 红（半张图进了证据名）。
 *
 * ⚠️ 手法沿用本仓先例：AST 抽出脚本里的函数定义后**直调**，不执行整篇脚本
 *   （`hxLaunchDetachBehavior.test.js` 同一手法 —— 整篇跑会去连真机与 adb）。
 *   判据 token **全 ASCII**：pwsh 的 stdout 在 Windows 上按 OEM 码页写，中文到 Node 侧会变乱码
 *   （本仓血账），所以中文文案一律在 **PS 侧**判完再回带布尔。
 *
 * ⚠️ 墙钟口径（#1549 的教训）：**判据只取定性值与顺序值**。下面的 `*_MS` 上界只用来证明
 *   「没挂住」，给的是预算的 30 倍余量；结论对不对看 `TimedOut` / `Bytes` / 文件在不在。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**
 *   （仓库先例：本目录 `autoScreenshotStabilityBehavior.test.js` / `hxLaunchDetachBehavior.test.js`）。
 *
 * 在册性（票面 AC5 现测后的裁定，写进 PR 正文）：本套件文件名匹配不到
 *   `scripts/lib/contract-tests.ps1` 的任何 token（那里现测**没有** `deviceCapture`），
 *   所以它**只由全量 ③ 门（`npm run test:unit`）兜**，不在 token 收窄的门里跑。
 *   刻意不补 token：`contract-tests.ps1` 正被 open PR #1561（#1543）改着，别并行动同一文件。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'device-capture.ps1');
const LIB = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

/**
 * 一次 pwsh 会话跑完五腿：抽函数 → 真执行核腿 → 影子腿（确定性）→ 连续两页 → 空帧出口。
 * 影子替换靠「PowerShell 函数名**在调用时**解析」（先例 B7），因此不改产品代码就有接缝。
 */
function probeShot(ctx) {
  const script = `
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
# 真执行核（有界的那一件）先加载 —— BSD1 要跑**真的**那一份，不是影子
. "${LIB}"
# 再把 device-capture.ps1 的函数定义整批抽出来（不执行整篇脚本：那会去连真机）
$src = Get-Content -LiteralPath "${SCRIPT}" -Raw
$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)
$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))
foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }
Write-Output ("EXTRACTED=" + $fns.Count)

$dir = '${ctx.tmp}'
# 脚本级前置：与脚本顶部状态块同形（StrictMode -Version Latest 下「先声明再读」是硬要求，
# 本探针第一版就是被 $script:ShotTimeouts 未声明照出来的 —— 那条报错正是这条纪律的证据）
$VerifyRoot = $dir
$AdbExe = 'unused'
$AdbCallTimeoutSeconds = 2
$script:Serial = 'FAKE-SERIAL'
$script:ShotTimeouts = @()
$script:Log = @()
# Write-Log 换成收集器（后定义者胜）：BSD2/BSD4/BSD5 判的是**日志里点没点名**，不是盘上的日志文件
function Write-Log { param([string]$Message) $script:Log = @($script:Log) + @($Message) }
function LogHit([string]$Pattern) {
  return [bool](@($script:Log | Where-Object { $_ -match $Pattern }).Count -ge 1)
}

Write-Output ("IS_WINDOWS=" + [bool]$IsWindows)

# ── BSD1：真启动器 + 真「先吐 14 字节就永不返回」的桩 ⇒ 调用点在预算内给结论
$hang = Join-Path $dir 'hang.cmd'
Set-Content -LiteralPath $hang -Encoding ascii -Value @('@echo off', 'echo PARTIAL-FRAME', 'ping -n 120 127.0.0.1 > nul')
$AdbExe = $hang
$script:Log = @()
$sw1 = [System.Diagnostics.Stopwatch]::StartNew()
$r1 = Export-Screenshot -Name 'bsd1.png'
$sw1.Stop()
Write-Output ("BSD1_TIMEDOUT=" + [bool]$r1.TimedOut)
Write-Output ("BSD1_BYTES=" + $r1.Bytes)
Write-Output ("BSD1_FINAL_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'bsd1.png'))))
Write-Output ("BSD1_LOG_NAMES_CALL=" + $(LogHit 'SHOT_CALL_TIMEOUT page=bsd1\\.png callBudgetSeconds=2'))
Write-Output ("BSD1_MS=" + $sw1.ElapsedMilliseconds)

# ── 影子执行核：把「不返回」与「有帧」收成确定性的读数（BSD2–BSD5 用）
function Invoke-BoundedAdbShot {
    param([string]$AdbExe, [string]$Serial, [string]$OutFile, [int]$TimeoutSeconds = 15)
    if ($script:shadow -eq 'timeout_with_frame') {
        # 真链路现测到的竞争形状：被判超时，而残帧**就留在盘上**（删除与句柄释放有竞争）
        Set-Content -LiteralPath $OutFile -Encoding ascii -Value 'PARTIAL-FRAME-BYTES'
        return [pscustomobject]@{ TimedOut = $true; Exited = $false; ExitCode = -1; Seconds = [double]$TimeoutSeconds; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = "未在 $TimeoutSeconds 秒内返回（桩）" }
    }
    if ($script:shadow -eq 'ok') {
        Set-Content -LiteralPath $OutFile -Encoding ascii -Value 'FULL-FRAME-BYTES-LONGER-THAN-PARTIAL'
        return [pscustomobject]@{ TimedOut = $false; Exited = $true; ExitCode = 0; Seconds = 0.4; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = '' }
    }
    return [pscustomobject]@{ TimedOut = $false; Exited = $true; ExitCode = 1; Seconds = 0.1; OutFile = $OutFile; ErrFile = ''; ErrTail = 'error: device not found'; Error = '' }
}
$AdbExe = 'stub'

# ── BSD2：超时 + **残帧留在盘上** ⇒ 最终名不许有文件（防线是命名归位，不是删除成功）
$script:shadow = 'timeout_with_frame'
$script:Log = @()
$script:ShotTimeouts = @()
$r2 = Export-Screenshot -Name 'bsd2.png'
Write-Output ("BSD2_TIMEDOUT=" + [bool]$r2.TimedOut)
Write-Output ("BSD2_BYTES=" + $r2.Bytes)
Write-Output ("BSD2_HASH_EMPTY=" + [bool]($r2.Hash -eq ''))
Write-Output ("BSD2_FINAL_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'bsd2.png'))))
# 「残帧确实留下了」必须是**实测**的，否则上面那条「不在盘上」是空转断言（guards.md 判据③）
$partBytes = [regex]::Match((($script:Log) -join ' '), 'partBytes=(\\d+)')
Write-Output ("BSD2_PARTBYTES=" + $(if ($partBytes.Success) { $partBytes.Groups[1].Value } else { 'none' }))
Write-Output ("BSD2_LOG_NAMES_BUDGET=" + $(LogHit 'callBudgetSeconds=2'))
Write-Output ("BSD2_LOG_NAMES_FINAL=" + $(LogHit 'finalWritten=False'))
Write-Output ("BSD2_ACCUMULATOR=" + (@($script:ShotTimeouts) -join ','))

# ── BSD3 对照腿：正常返回的非空帧 ⇒ 归位 + 算 sha + 不出超时机检行
$script:shadow = 'ok'
$script:Log = @()
$script:ShotTimeouts = @()
$r3 = Export-Screenshot -Name 'bsd3.png'
Write-Output ("BSD3_TIMEDOUT=" + [bool]$r3.TimedOut)
Write-Output ("BSD3_BYTES_GT0=" + [bool]($r3.Bytes -gt 0))
Write-Output ("BSD3_HASHLEN=" + $r3.Hash.Length)
Write-Output ("BSD3_FINAL_EXISTS=" + [bool](Test-Path -LiteralPath (Join-Path $dir 'bsd3.png') -PathType Leaf))
Write-Output ("BSD3_PART_GONE=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'bsd3.png.part'))))
Write-Output ("BSD3_NO_TIMEOUT_LOG=" + [bool](-not (LogHit 'SHOT_CALL_TIMEOUT')))
Write-Output ("BSD3_ACCUMULATOR_EMPTY=" + [bool](@($script:ShotTimeouts).Count -eq 0))

# ── BSD4：一页超时之后**后面的页照旧跑**（票面「该页记失败并继续」）
$script:shadow = 'timeout_with_frame'
$script:Log = @()
$script:ShotTimeouts = @()
$c1 = Export-Screenshot -Name 'bsd4-a.png'
$script:shadow = 'ok'
$c2 = Export-Screenshot -Name 'bsd4-b.png'
Write-Output ("BSD4_FIRST_TIMEOUT=" + [bool]$c1.TimedOut)
Write-Output ("BSD4_FIRST_BYTES=" + $c1.Bytes)
Write-Output ("BSD4_SECOND_TIMEOUT=" + [bool]$c2.TimedOut)
Write-Output ("BSD4_SECOND_FINAL_EXISTS=" + [bool](Test-Path -LiteralPath (Join-Path $dir 'bsd4-b.png') -PathType Leaf))
Write-Output ("BSD4_ACCUMULATOR=" + (@($script:ShotTimeouts) -join ','))

# ── BSD5：没有帧 ≠ 超时 —— 两个出口不许混（否则「adb 报错」会被读成「通道不返回」）
$script:shadow = 'noframe'
$script:Log = @()
$script:ShotTimeouts = @()
$r5 = Export-Screenshot -Name 'bsd5.png'
Write-Output ("BSD5_TIMEDOUT=" + [bool]$r5.TimedOut)
Write-Output ("BSD5_BYTES=" + $r5.Bytes)
Write-Output ("BSD5_FINAL_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'bsd5.png'))))
Write-Output ("BSD5_LOG_HAS_STDERR=" + $(LogHit 'device not found'))
Write-Output ("BSD5_NO_TIMEOUT_LOG=" + [bool](-not (LogHit 'SHOT_CALL_TIMEOUT')))

# 证据目录里**不许**留下半张图的最终名（步骤 7 按 *.png 扫目录比基线）
Write-Output ("STRAY_PNG=" + @(Get-ChildItem -LiteralPath $dir -Filter '*.png' | Where-Object { $_.Name -notmatch 'bsd3|bsd4-b' }).Count)
Write-Output "SHOT_PROBE_DONE=1"
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

describe('device-capture 截图调用的有界性（运行期，#1562）', () => {
  let ctx;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'devshot-')) };
    probe = probeShot(ctx);
    if (probe.ok) {
      [
        'IS_WINDOWS', 'EXTRACTED', 'SHOT_PROBE_DONE',
        'BSD1_TIMEDOUT', 'BSD1_BYTES', 'BSD1_FINAL_ABSENT', 'BSD1_LOG_NAMES_CALL', 'BSD1_MS',
        'BSD2_TIMEDOUT', 'BSD2_BYTES', 'BSD2_HASH_EMPTY', 'BSD2_FINAL_ABSENT', 'BSD2_PARTBYTES',
        'BSD2_LOG_NAMES_BUDGET', 'BSD2_LOG_NAMES_FINAL', 'BSD2_ACCUMULATOR',
        'BSD3_TIMEDOUT', 'BSD3_BYTES_GT0', 'BSD3_HASHLEN', 'BSD3_FINAL_EXISTS', 'BSD3_PART_GONE',
        'BSD3_NO_TIMEOUT_LOG', 'BSD3_ACCUMULATOR_EMPTY',
        'BSD4_FIRST_TIMEOUT', 'BSD4_FIRST_BYTES', 'BSD4_SECOND_TIMEOUT', 'BSD4_SECOND_FINAL_EXISTS', 'BSD4_ACCUMULATOR',
        'BSD5_TIMEDOUT', 'BSD5_BYTES', 'BSD5_FINAL_ABSENT', 'BSD5_LOG_HAS_STDERR', 'BSD5_NO_TIMEOUT_LOG',
        'STRAY_PNG',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
    }
  });

  afterAll(() => {
    if (ctx) {
      try {
        fs.rmSync(ctx.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红（BSD1 的桩可能还挂着 ping；先例同 B6）
      }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      'device-capture 截图有界性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  // BSD1：真执行核 —— 有界不是影子造出来的性质
  test('BSD1: 真挂死桩下调用点在预算内给结论，最终名上不留文件，日志点名是哪一次调用', () => {
    requireProbe();
    expect(v.EXTRACTED).not.toBeNull();
    expect(Number(v.EXTRACTED)).toBeGreaterThanOrEqual(20);
    expect(v.BSD1_BYTES).toBe('0');
    expect(v.BSD1_FINAL_ABSENT).toBe('True');
    if (v.IS_WINDOWS === 'True') {
      expect(v.BSD1_TIMEDOUT).toBe('True');
      expect(v.BSD1_LOG_NAMES_CALL).toBe('True');
      // 只证「没挂住」：预算 2 秒，这里给 60 秒（30 倍余量）。上界不是判据（见文件头墙钟口径）。
      expect(Number(v.BSD1_MS)).toBeLessThanOrEqual(60000);
    } else {
      // Linux（本仓 CI runner）没有 cmd.exe ⇒ 起进程失败也**不许挂住**，且走的是「没有帧」而非「超时」出口。
      // 这一支断不了「日志点名是哪一次超时调用」（根本没有超时发生），所以那条断言只在 Windows 那支跑。
      expect(v.BSD1_TIMEDOUT).toBe('False');
      expect(Number(v.BSD1_MS)).toBeLessThanOrEqual(60000);
    }
  });

  // BSD2：残帧留在盘上时，防线必须是「先 .part、成功才归位」这条**命名**闸门
  test('BSD2: 超时且残帧确实留盘 ⇒ 最终名不写文件（归位靠命名、不靠删除成功）', () => {
    requireProbe();
    expect(v.BSD2_TIMEDOUT).toBe('True');
    expect(v.BSD2_BYTES).toBe('0');
    expect(v.BSD2_HASH_EMPTY).toBe('True'); // 没有帧就不进页身份哈希（半张图的 sha 会伪装成一张图）
    expect(v.BSD2_FINAL_ABSENT).toBe('True');
    // 空转证明：残帧**确实**留在了盘上（partBytes>0 由 PS 侧从日志里读出来），
    // 否则上面那条「最终名不在」只是「本来就没写」，守不住任何东西（guards.md 判据③）
    expect(Number(v.BSD2_PARTBYTES)).toBeGreaterThan(0);
    expect(v.BSD2_LOG_NAMES_BUDGET).toBe('True');
    expect(v.BSD2_LOG_NAMES_FINAL).toBe('True');
    expect(v.BSD2_ACCUMULATOR).toBe('bsd2.png');
  });

  // BSD3 对照腿：正常帧照旧归位、算 sha、不出超时机检行
  test('BSD3: 对照腿正常返回时仍按原判据出图（证明 BSD2 不是「逢截必红」）', () => {
    requireProbe();
    expect(v.BSD3_TIMEDOUT).toBe('False');
    expect(v.BSD3_BYTES_GT0).toBe('True');
    expect(Number(v.BSD3_HASHLEN)).toBe(64);
    expect(v.BSD3_FINAL_EXISTS).toBe('True');
    expect(v.BSD3_PART_GONE).toBe('True'); // .part 已被归位吃掉，不留在证据目录
    expect(v.BSD3_NO_TIMEOUT_LOG).toBe('True');
    expect(v.BSD3_ACCUMULATOR_EMPTY).toBe('True');
  });

  // BSD4：该页失败、**后面的页照旧跑**（票面「记失败并继续」）
  test('BSD4: 一页超时后第二页照样出图，超时清单只点名第一页', () => {
    requireProbe();
    expect(v.BSD4_FIRST_TIMEOUT).toBe('True');
    expect(v.BSD4_FIRST_BYTES).toBe('0');
    expect(v.BSD4_SECOND_TIMEOUT).toBe('False');
    expect(v.BSD4_SECOND_FINAL_EXISTS).toBe('True');
    expect(v.BSD4_ACCUMULATOR).toBe('bsd4-a.png');
  });

  // BSD5：出口不许混 —— 「adb 报错没给帧」不能被读成「通道不返回」（处置人不同）
  test('BSD5: 没有帧不谎报成超时，且把 adb stderr 带上；证据目录无游离 PNG', () => {
    requireProbe();
    expect(v.BSD5_TIMEDOUT).toBe('False');
    expect(v.BSD5_BYTES).toBe('0');
    expect(v.BSD5_FINAL_ABSENT).toBe('True');
    expect(v.BSD5_LOG_HAS_STDERR).toBe('True');
    expect(v.BSD5_NO_TIMEOUT_LOG).toBe('True');
    // 除 BSD3/BSD4 那两张**成功**的图，目录里不许有别的 *.png（半帧绝不进 `*.png` 射程）
    expect(v.STRAY_PNG).toBe('0');
    expect(v.SHOT_PROBE_DONE).toBe('1');
  });
});
