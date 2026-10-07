/**
 * `Compare-ScreenFrames` 的**宽容比较**语义，运行期守护（#1027 收尾，2026-09-15 真机实测）
 *
 * 为什么需要（真机实测，血账）：
 *   `Wait-NavSettled` 的「画面稳定」原先是**全帧 hash 相等**。而系统状态栏里有**应用控制不了**的
 *   实时读数（MIUI「显示实时网速」的 KB/s，每 1–2 秒就变）⇒ 连拍三帧的整帧 sha256 **两两不同**
 *   （实测差异**只在顶部 0–99px 状态栏**、MaxDiff 188，其余整幅逐字节相同）⇒ 判据**永远不可能满足**
 *   ⇒ 步骤 6 必然 420 秒超时、步骤 7–9（对比 / 证据 / 还原）永远跑不到，而 app 画面早就落定了。
 *   ⇒ 改为**采样网格上的差异像素占比**（≤ 阈值即判内容一致）。本文件钉住这个语义：
 *     ① 微小的系统 UI churn **必须**判「一致」（否则又退回永远不落定）；
 *     ② 真切页 / 滚动那种大面积变化**必须**判「不一致」（fail-closed 语义不得被放宽）；
 *     ③ 缺上一帧 / 取色不可用**必须**判「不一致」并把原因回带（宁可不落定，绝不假稳定）。
 *
 * 标定（同机实测，1080×2400）：状态栏量级 churn 在 24/48/96 网格上 = 0%、192 网格 = 0.011%；
 *   真切页（对照全黑帧）= ~100% ⇒ 默认阈值 0.5% 在噪声上方 ~45×、真变化下方 ~200×。
 *
 * ⚠️ **判别力的平台边界（写实，不许假称处处有判别力）**：本函数靠 `System.Drawing` 取色，而它
 *   **只在 Windows 可用**（PowerShell 7 的 System.Drawing.Common 是 Windows-only；注意 **Linux 上
 *   `Add-Type -AssemblyName System.Drawing` 是成功的**，真正抛错的是构造 `Bitmap` ⇒ 能力探测必须真建一次位图）。所以每个用例
 *   都显式分两支、**两支都断言**（没有静默跳过），只是各自在自己平台上验该验的那一面：
 *     · Windows（本工具链的运行平台）：验「微变宽容 + 巨变拦得住」；
 *     · Linux（本仓 CI 的 ubuntu runner）：验**缺件时的 fail-closed 语义**（判不出 ⇒ 不判稳定，
 *       且回带的 `DiffPercent=-1` 哨兵值必须出现 —— 它是「没比较」与「比过了但不同」的区分点）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（仓库先例：
 *   `hxLaunchDetachBehavior.test.js` / `contractTestPatternBehavior.test.js`）。
 *
 * ── 本文件第二组（B6–B8，2026-10-06，#1560）：adb 单次调用的**有界性**
 *   为什么放这里而不是新开套件（票面 AC5 的措辞按「不新增 token」解释）：
 *     · `contract-tests.ps1:62` 的 `autoScreenshot` token 是**子串**匹配，本文件已在册 ⇒ 零注册改动，
 *       不新开第二真源；
 *     · 本文件的夹具现成——已 dot-source `auto-screenshot.ps1`（函数级直调，不碰 HBuilderX），
 *       已在 `mkdtemp` 里合成 PNG，判据 token 全 ASCII。
 *   三层各锁一件事（**PNG 字节完整性不放进 ③ 门**——cmd 往 stdout 吐二进制不可靠，那一条由 #1560
 *   的**真链路腿**承载：真机 screencap 走同一形状，断言 PNG 头与可解码，读数在 PR 正文）：
 *     B6 有界执行器本身：真启动器 + 「写半帧就永不返回」的 `.cmd` 桩 ⇒ 预算内返回 / 杀到孙进程 /
 *        残帧尽力删（并先证明「被杀的调用确实会在盘上留下非空残帧」，否则删除断言是空转）；
 *     B7 采样循环：把有界执行器**影子替换**成「永不返回」（PowerShell 函数名在调用时解析 ⇒ 无需改产品代码）
 *        ⇒ 到点 `Settled=False`、`Samples=0`（超时轮不计入采样）、`CallTimeouts>0`、Reason 点名是调用超时；
 *        Windows 那支另跑一遍**真桩**版本（不靠影子），证整条链在真启动器下同样收口；
 *     B7W 「**残帧留在盘上**还不算采样」——真链路现测（2026-10-07，强制挂死腿预算 0）照出 `Kill` 之后
 *        删除与子进程句柄有竞争、**可能删不掉**（日志原文「残帧未删净」/ `nav_probe_left=True`）⇒
 *        「不计入采样」必须是 `-not $shot.TimedOut` 这条**逻辑闸门**，不能是删除的成功率。
 *        这一支断言夹具里文件确实在盘上且非空（`B7W_FILE_BYTES > 0`）而 `Samples=0`，并验半帧没被写进
 *        「上一帧」（`B7W_PREV_ABSENT`）—— 否则下一轮拿半帧一比就判「稳定」。
 *        同源的第二处防线（页面截图先落 `.part` 成功才归位）由 S23 的 ④b 在结构面钉住。
 *     B8 对照腿：影子执行器正常写帧 + 日志给页身份行 ⇒ `Settled=True`（三条同时满足那一条真走到）。
 *   ⚠️ 平台边界同 B1–B5：取色不可用时「落定」在结构上不可能（`Compare-ScreenFrames` fail-closed 判不一致）
 *     ⇒ Linux 那支断言的是「**仍然有界**」而不是「能落定」，两支都断言、无静默跳过。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const AUTO_SHOT = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 造四张合成帧（600×800，24 步网格 ⇒ stepX=25 / stepY=33 / 共 600 个采样点）并回读判定：
 *  · base      基准帧（纯色 + 一个白块）
 *  · identical 与 base **逐字节相同**（复制）
 *  · tiny      只改 **6×6**、恰好命中**一个**采样点（1/600 = 0.167% ⇒ 应判「一致」但仍 >0）
 *  · big       改掉上半幅（真切页量级 ⇒ 应判「不一致」）
 *  判据 token 全是 ASCII（True/False/数字），JS 侧**不匹配中文**（本仓血账：OEM 码页会把中文变乱码）。
 */
function probeFrames(ctx) {
  const q = (s) => String(s).replace(/'/g, "''");
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. "${AUTO_SHOT}"`,
    // ⚠️ 能力探测**必须真建一次位图**：`Add-Type -AssemblyName System.Drawing` 在 Linux 上**会成功**
    //    （程序集能加载），真正抛错的是构造 Bitmap（`The type initializer for 'Windows.Win32.PInvokeGdiPlus'
    //    threw an exception`）—— 本用例第一版只 try 了 Add-Type ⇒ 在 ubuntu runner 上判成「可用」，
    //    接着在建图处 abort，整条探针没有任何 token（CI 实测就这么红的）。
    'try {',
    '  Add-Type -AssemblyName System.Drawing -ErrorAction Stop',
    '  $probeBmp = New-Object System.Drawing.Bitmap 2, 2',
    '  $probeBmp.Dispose()',
    '  $drawing = $true',
    '} catch { $drawing = $false }',
    'Write-Output ("DRAWING_AVAILABLE=" + [bool]$drawing)',
    `$dir = '${q(ctx.tmp)}'`,
    'if ($drawing) {',
    '  $w = 600; $h = 800',
    '  $bmp = New-Object System.Drawing.Bitmap $w, $h',
    '  $g = [System.Drawing.Graphics]::FromImage($bmp)',
    '  $g.Clear([System.Drawing.Color]::FromArgb(255, 210, 226, 245))',
    '  $g.FillRectangle([System.Drawing.Brushes]::White, 20, 300, 560, 200)',
    '  $g.Dispose()',
    "  $bmp.Save((Join-Path $dir 'base.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $bmp.Dispose()',
    "  Copy-Item (Join-Path $dir 'base.png') (Join-Path $dir 'identical.png') -Force",
    '  $b = New-Object System.Drawing.Bitmap (Join-Path $dir "base.png")',
    '  $g2 = [System.Drawing.Graphics]::FromImage($b)',
    '  $g2.FillRectangle([System.Drawing.Brushes]::Black, 498, 31, 6, 6)',
    '  $g2.Dispose()',
    "  $b.Save((Join-Path $dir 'tiny.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $b.Dispose()',
    '  $c = New-Object System.Drawing.Bitmap (Join-Path $dir "base.png")',
    '  $g3 = [System.Drawing.Graphics]::FromImage($c)',
    '  $g3.FillRectangle([System.Drawing.Brushes]::Black, 0, 0, $w, [int]($h / 2))',
    '  $g3.Dispose()',
    "  $c.Save((Join-Path $dir 'big.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $c.Dispose()',
    '}',
    'function Emit([string]$tag, $r) {',
    '  Write-Output ($tag + "_SAME=" + [bool]$r.Same)',
    '  Write-Output ($tag + "_PCT=" + $r.DiffPercent)',
    // ⚠️ 成功路径的返回对象**没有** Error 属性 ⇒ StrictMode 下 `$r.Error` 会直接抛错（本用例第一版就栽在这），
    //    必须用 PSObject.Properties 判「属性在不在」，而不是判值是否为 $null。
    '  Write-Output ($tag + "_HAS_ERROR=" + [bool]($null -ne $r.PSObject.Properties["Error"]))',
    '}',
    "Emit 'IDENTICAL' (Compare-ScreenFrames -Path (Join-Path $dir 'identical.png') -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'TINY'      (Compare-ScreenFrames -Path (Join-Path $dir 'tiny.png')      -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'BIG'       (Compare-ScreenFrames -Path (Join-Path $dir 'big.png')       -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'MISSING'   (Compare-ScreenFrames -Path (Join-Path $dir 'base.png')      -PrevPath (Join-Path $dir 'no-such-prev.png'))",
    'Write-Output "PROBE_DONE=1"',
  ].join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
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

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

/**
 * B6–B8 的探针（#1560）：adb 单次调用的**有界性**，一次 pwsh 会话里跑完三层。
 * 判据 token 全 ASCII（True/False/数字）——JS 侧不匹配中文（本仓血账：OEM 码页会把中文变乱码）；
 * 需要判中文的地方（Reason 文案）在 **PS 侧**判完再回带布尔，跨语言只传 ASCII。
 */
function probeBounded(ctx) {
  const script = `
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
. "${AUTO_SHOT}"
$dir = '${ctx.tmp}'
Write-Output ("IS_WINDOWS=" + [bool]$IsWindows)
try {
  Add-Type -AssemblyName System.Drawing -ErrorAction Stop
  $pb = New-Object System.Drawing.Bitmap 2, 2
  $pb.Dispose()
  $drawing = $true
} catch { $drawing = $false }
Write-Output ("DRAWING_AVAILABLE=" + [bool]$drawing)

# 假 adb 桩：先把 14 字节写到 stdout（=「被杀的调用会留下非空残帧」的前提），再挂 120 秒不返回
$hang = Join-Path $dir 'hang.cmd'
Set-Content -LiteralPath $hang -Encoding ascii -Value @('@echo off', 'echo PARTIAL-FRAME', 'ping -n 120 127.0.0.1 > nul')
$quick = Join-Path $dir 'quick.cmd'
Set-Content -LiteralPath $quick -Encoding ascii -Value @('@echo off', 'echo OK')
$navLog = Join-Path $dir 'nav.log'
Set-Content -LiteralPath $navLog -Encoding utf8 -Value 'probe log without page line'

if ($IsWindows) {
  # ── B6：真启动器 + 挂死桩 ⇒ 预算内返回 + 杀到孙进程 + 残帧不在盘上
  $p6 = Join-Path $dir 'b6.png'
  $sw6 = [System.Diagnostics.Stopwatch]::StartNew()
  $r6 = Invoke-BoundedAdbShot -AdbExe $hang -Serial 'FAKE-SERIAL' -OutFile $p6 -TimeoutSeconds 2
  $sw6.Stop()
  Write-Output ("B6_TIMEDOUT=" + [bool]$r6.TimedOut)
  Write-Output ("B6_MS=" + $sw6.ElapsedMilliseconds)
  Write-Output ("B6_FILE_GONE=" + [bool](-not (Test-Path -LiteralPath $p6)))
  Start-Sleep -Milliseconds 300
  Write-Output ("B6_PING_LEFT=" + @(Get-Process -Name 'ping' -ErrorAction SilentlyContinue).Count)
  # 空转证明：不走本函数、同样 Kill 一次挂死调用 ⇒ 盘上**确实**留着非空残帧 ⇒ 上面那条删除不是空转
  $raw = Join-Path $dir 'b6raw.png'
  $rp = Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', ('"{0}" -s X exec-out screencap -p' -f $hang)) -RedirectStandardOutput $raw -NoNewWindow -PassThru
  $null = $rp.WaitForExit(1000)
  if (-not $rp.HasExited) { try { $rp.Kill($true) } catch { } }
  Write-Output ("B6C_RAW_BYTES=" + $(if (Test-Path -LiteralPath $raw) { (Get-Item -LiteralPath $raw).Length } else { 0 }))
  $r6b = Invoke-BoundedAdbShot -AdbExe $quick -Serial 'FAKE-SERIAL' -OutFile (Join-Path $dir 'b6ok.png') -TimeoutSeconds 10
  Write-Output ("B6B_TIMEDOUT=" + [bool]$r6b.TimedOut)
  Write-Output ("B6B_EXIT=" + $r6b.ExitCode)

  # ── B7R：整条采样循环 + **真桩**（不经影子）⇒ 到点未落定、一次采样都没拿到、盘上无残帧
  $swR = [System.Diagnostics.Stopwatch]::StartNew()
  $rR = Wait-NavSettled -AdbExe $hang -Serial 'FAKE-SERIAL' -ProbeFile (Join-Path $dir 'p7r.png') -NavOutFile $navLog -ExpectedPage 'pages/probe/page' -MinSeconds 1 -TimeoutSeconds 6 -PollSeconds 1 -CallTimeoutSeconds 2
  $swR.Stop()
  Write-Output ("B7R_SETTLED=" + [bool]$rR.Settled)
  Write-Output ("B7R_SAMPLES=" + $rR.Samples)
  Write-Output ("B7R_CALLTIMEOUTS=" + $rR.CallTimeouts)
  Write-Output ("B7R_MS=" + $swR.ElapsedMilliseconds)
  Write-Output ("B7R_PROBE_GONE=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'p7r.png'))))
  Write-Output ("B7R_REASON_NAMES_CALL_TIMEOUT=" + [bool]($rR.Reason -match '未在 2 秒内返回'))
} else {
  # Linux（本仓 CI runner）：没有 cmd.exe ⇒ 本函数**也不能挂住**——有界性是「拿不到帧时的唯一出路」
  $sw6 = [System.Diagnostics.Stopwatch]::StartNew()
  $r6 = Invoke-BoundedAdbShot -AdbExe '/bin/echo' -Serial 'FAKE-SERIAL' -OutFile (Join-Path $dir 'b6.png') -TimeoutSeconds 2
  $sw6.Stop()
  Write-Output ("B6_BOUNDED=" + [bool]($sw6.ElapsedMilliseconds -lt 10000))
  Write-Output ("B6_HAS_ERROR=" + [bool]($r6.Error -ne ''))
  Write-Output ("B6_TIMEDOUT=" + [bool]$r6.TimedOut)
}

# ── 影子替换有界执行器（PowerShell 函数名**在调用时解析** ⇒ 无需改产品代码就有接缝）
$script:shadowMode = 'timeout'
$script:shadowCalls = 0
$basePng = Join-Path $dir 'base.png'
function Invoke-BoundedAdbShot {
  param([string]$AdbExe, [string]$Serial, [string]$OutFile, [int]$TimeoutSeconds = 15)
  $script:shadowCalls = $script:shadowCalls + 1
  if ($script:shadowMode -eq 'ok') {
    if (Test-Path -LiteralPath $basePng) { Copy-Item -LiteralPath $basePng -Destination $OutFile -Force }
    if ($script:shadowCalls -eq 1) { Add-Content -LiteralPath $navLog -Value '进入页面:"pages/probe/page"' -Encoding utf8 }
    return [pscustomobject]@{ TimedOut = $false; Exited = $true; ExitCode = 0; Seconds = 0.1; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = '' }
  }
  if ($script:shadowMode -eq 'timeout_write') {
    # 真链路现测的形状：调用被判超时，而**残帧确实留在盘上**（Kill 与句柄释放有竞争，删除可能失败）
    Set-Content -LiteralPath $OutFile -Encoding ascii -Value 'PARTIAL-FRAME-BYTES'
    return [pscustomobject]@{ TimedOut = $true; Exited = $false; ExitCode = -1; Seconds = $TimeoutSeconds; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = '探针桩：留残帧并判超时' }
  }
  return [pscustomobject]@{ TimedOut = $true; Exited = $false; ExitCode = -1; Seconds = $TimeoutSeconds; OutFile = $OutFile; ErrFile = ''; ErrTail = ''; Error = '探针桩：未在 ' + $TimeoutSeconds + ' 秒内返回' }
}

# ── B7：每次调用都永不返回 ⇒ 循环必须**到点**收口（420 秒那条上限在此真的可达）
$sw7 = [System.Diagnostics.Stopwatch]::StartNew()
$r7 = Wait-NavSettled -AdbExe 'stub' -Serial 'FAKE-SERIAL' -ProbeFile (Join-Path $dir 'p7.png') -NavOutFile $navLog -ExpectedPage 'pages/probe/page' -MinSeconds 1 -TimeoutSeconds 6 -PollSeconds 1 -CallTimeoutSeconds 2
$sw7.Stop()
Write-Output ("B7_SETTLED=" + [bool]$r7.Settled)
Write-Output ("B7_SAMPLES=" + $r7.Samples)
Write-Output ("B7_CALLTIMEOUTS=" + $r7.CallTimeouts)
Write-Output ("B7_MS=" + $sw7.ElapsedMilliseconds)
Write-Output ("B7_REASON_NAMES_CALL_TIMEOUT=" + [bool]($r7.Reason -match '未在 2 秒内返回'))

# ── B7W：超时**且残帧留在盘上**（真链路现测到的形状）⇒ 依然不得计入采样。
#    这一支防的是「把删除当判据」：文件确实在盘上、且非空，靠的是「-not $shot.TimedOut」这条逻辑闸门。
$p7w = Join-Path $dir 'p7w.png'
$script:shadowMode = 'timeout_write'
$swW = [System.Diagnostics.Stopwatch]::StartNew()
$rW = Wait-NavSettled -AdbExe 'stub' -Serial 'FAKE-SERIAL' -ProbeFile $p7w -NavOutFile $navLog -ExpectedPage 'pages/probe/page' -MinSeconds 1 -TimeoutSeconds 6 -PollSeconds 1 -CallTimeoutSeconds 2
$swW.Stop()
Write-Output ("B7W_SETTLED=" + [bool]$rW.Settled)
Write-Output ("B7W_SAMPLES=" + $rW.Samples)
Write-Output ("B7W_CALLTIMEOUTS=" + $rW.CallTimeouts)
Write-Output ("B7W_MS=" + $swW.ElapsedMilliseconds)
Write-Output ("B7W_FILE_ON_DISK=" + [bool](Test-Path -LiteralPath $p7w))
Write-Output ("B7W_FILE_BYTES=" + $(if (Test-Path -LiteralPath $p7w) { (Get-Item -LiteralPath $p7w).Length } else { 0 }))
Write-Output ("B7W_PREV_ABSENT=" + [bool](-not (Test-Path -LiteralPath (Join-Path $dir 'nav-probe-prev.png'))))

# ── B8 对照腿：控制腿必须真走到「三条同时满足」那一条（Windows 才有 PNG 可写）
if ($drawing) {
  $bmp = New-Object System.Drawing.Bitmap 600, 800
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.Clear([System.Drawing.Color]::FromArgb(255, 210, 226, 245))
  $g.FillRectangle([System.Drawing.Brushes]::White, 20, 300, 560, 200)
  $g.Dispose()
  $bmp.Save($basePng, [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Dispose()
}
$script:shadowCalls = 0
$script:shadowMode = 'ok'
$sw8 = [System.Diagnostics.Stopwatch]::StartNew()
$r8 = Wait-NavSettled -AdbExe 'stub' -Serial 'FAKE-SERIAL' -ProbeFile (Join-Path $dir 'p8.png') -NavOutFile $navLog -ExpectedPage 'pages/probe/page' -MinSeconds 1 -TimeoutSeconds 8 -PollSeconds 1 -CallTimeoutSeconds 2
$sw8.Stop()
Write-Output ("B8_SETTLED=" + [bool]$r8.Settled)
Write-Output ("B8_SAMPLES=" + $r8.Samples)
Write-Output ("B8_CALLTIMEOUTS=" + $r8.CallTimeouts)
Write-Output ("B8_MS=" + $sw8.ElapsedMilliseconds)
Write-Output ("BOUNDED_DONE=" + $script:shadowCalls)
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

describe('Compare-ScreenFrames 宽容比较语义（运行期，#1027 收尾）', () => {
  let ctx;
  let probe;
  let drawing;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'stability-')) };
    probe = probeFrames(ctx);
    if (probe.ok) {
      drawing = field(probe.stdout, 'DRAWING_AVAILABLE');
      ['IDENTICAL', 'TINY', 'BIG', 'MISSING'].forEach((tag) => {
        v[tag] = {
          same: field(probe.stdout, `${tag}_SAME`),
          pct: field(probe.stdout, `${tag}_PCT`),
          hasError: field(probe.stdout, `${tag}_HAS_ERROR`),
        };
      });
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

  const canDraw = () => drawing === 'True';

  // B1：探针必须真跑完、判据 token 齐备（fail-closed，不 skip）
  test('B1: 探针跑完，判据 token 齐备（pwsh 可用）', () => {
    if (!probe.ok) {
      throw new Error(
        'Compare-ScreenFrames 探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
          + `exit=${probe.status}\nstdout=${probe.stdout}\nstderr=${probe.stderr}`
      );
    }
    expect(field(probe.stdout, 'PROBE_DONE')).toBe('1');
    expect(['True', 'False']).toContain(drawing);
    ['IDENTICAL', 'TINY', 'BIG', 'MISSING'].forEach((tag) => {
      expect(['True', 'False']).toContain(v[tag].same);
      expect(['True', 'False']).toContain(v[tag].hasError);
    });
  });

  // B2：逐字节相同的两帧 ⇒ 必须判「一致」（差异 0%），否则连静止页面都落定不了
  test('B2: 完全相同的两帧 ⇒ 判内容一致（差异 0%）', () => {
    if (!canDraw()) {
      // Linux CI：取色不可用 ⇒ 只能是 fail-closed 那一支，且必须回带 -1 哨兵（「没比较」）
      expect(v.IDENTICAL.same).toBe('False');
      expect(v.IDENTICAL.hasError).toBe('True');
      expect(Number(v.IDENTICAL.pct)).toBe(-1);
      return;
    }
    expect(v.IDENTICAL.same).toBe('True');
    expect(v.IDENTICAL.hasError).toBe('False');
    expect(Number(v.IDENTICAL.pct)).toBe(0);
  });

  // B3：只改一个采样点（6×6，状态栏实时读数量级）⇒ **仍判一致**，但占比必须**大于 0**
  //     （大于 0 才证明「差异被真的看见了、只是被宽容掉了」，而不是网格恰好漏掉 ⇒ 弱断言）。
  //     这条就是本 bug 的回归钉：退回全帧 hash 相等时它会红（现实中它让步骤 7–9 永远跑不到）。
  test('B3: 命中 1/600 个采样点的微变 ⇒ 判内容一致（宽容生效，且差异确实被看见）', () => {
    if (!canDraw()) {
      expect(v.TINY.same).toBe('False');
      expect(v.TINY.hasError).toBe('True');
      return;
    }
    expect(v.TINY.same).toBe('True');
    const pct = Number(v.TINY.pct);
    expect(pct).toBeGreaterThan(0);
    expect(pct).toBeLessThanOrEqual(0.5);
  });

  // B4：真切页量级的大面积变化（上半幅）⇒ **必须**判不一致（宽容不放宽 fail-closed 语义）
  test('B4: 半幅画面变化 ⇒ 判内容不一致（该拦的照旧拦）', () => {
    if (!canDraw()) {
      expect(v.BIG.same).toBe('False');
      expect(v.BIG.hasError).toBe('True');
      return;
    }
    expect(v.BIG.same).toBe('False');
    expect(Number(v.BIG.pct)).toBeGreaterThan(0.5);
  });

  // B5：缺上一帧（首轮采样）⇒ 判不一致且回带原因，绝不假稳定
  test('B5: 缺上一帧 ⇒ 判不一致并回带原因（fail-closed）', () => {
    expect(v.MISSING.same).toBe('False');
    expect(v.MISSING.hasError).toBe('True');
  });
});

describe('adb 单次调用的有界性（运行期，#1560）', () => {
  let ctx2;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx2 = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'bounded-')) };
    probe = probeBounded(ctx2);
    if (probe.ok) {
      [
        'IS_WINDOWS', 'DRAWING_AVAILABLE',
        'B6_TIMEDOUT', 'B6_MS', 'B6_FILE_GONE', 'B6_PING_LEFT', 'B6C_RAW_BYTES', 'B6B_TIMEDOUT', 'B6B_EXIT',
        'B6_BOUNDED', 'B6_HAS_ERROR',
        'B7R_SETTLED', 'B7R_SAMPLES', 'B7R_CALLTIMEOUTS', 'B7R_MS', 'B7R_PROBE_GONE', 'B7R_REASON_NAMES_CALL_TIMEOUT',
        'B7_SETTLED', 'B7_SAMPLES', 'B7_CALLTIMEOUTS', 'B7_MS', 'B7_REASON_NAMES_CALL_TIMEOUT',
        'B7W_SETTLED', 'B7W_SAMPLES', 'B7W_CALLTIMEOUTS', 'B7W_MS', 'B7W_FILE_ON_DISK', 'B7W_FILE_BYTES', 'B7W_PREV_ABSENT',
        'B8_SETTLED', 'B8_SAMPLES', 'B8_CALLTIMEOUTS', 'B8_MS', 'BOUNDED_DONE',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
      v.navLines = String(probe.stdout).split('\n').filter((l) => l.indexOf('NAV_SAMPLE') >= 0);
    }
  });

  afterAll(() => {
    if (ctx2) {
      try {
        fs.rmSync(ctx2.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红
      }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      '有界性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  // B6：**有界执行器本身**（真 cmd 启动器 + 真「写半帧就永不返回」的桩）
  test('B6: 挂死的单次调用必须在预算内被终止 —— 杀到孙进程、残帧不留在盘上', () => {
    requireProbe();
    if (v.IS_WINDOWS !== 'True') {
      // Linux（本仓 CI runner）：没有 cmd.exe ⇒ 能断言的是「启动器缺失同样不许挂住」——
      // 有界性这条性质不该依赖「能不能起进程」。
      expect(v.B6_BOUNDED).toBe('True');
      expect(v.B6_HAS_ERROR).toBe('True');
      return;
    }
    expect(v.B6_TIMEDOUT).toBe('True');
    expect(Number(v.B6_MS)).toBeLessThanOrEqual(6000); // 预算 2 秒 + 派发开销
    expect(v.B6_FILE_GONE).toBe('True'); // 残帧当场删
    expect(v.B6_PING_LEFT).toBe('0'); // Kill 杀到 cmd 的孙进程
    // 上面那条删除**不是空转**：不走本函数、同样杀一次挂死调用 ⇒ 盘上确实留着非空残帧
    expect(Number(v.B6C_RAW_BYTES)).toBeGreaterThan(0);
    // 成对的对照腿：正常返回的调用不得被误判成超时（否则「有界」退化成「把所有调用都杀掉」）
    expect(v.B6B_TIMEDOUT).toBe('False');
    expect(Number(v.B6B_EXIT)).toBe(0);
  });

  // B7：**采样循环**——每次调用都不返回 ⇒ 那条 TimeoutSeconds 上限必须真的可达（票面的靶心）
  test('B7: 调用永不返回时循环到点判未落定，超时轮不计入采样，Reason 点名是调用超时', () => {
    requireProbe();
    expect(v.B7_SETTLED).toBe('False');
    // 超时轮**不计入采样**：半张 PNG 若进了 Compare-ScreenFrames 会伪装成「两帧一致」⇒ 谎报落定
    expect(v.B7_SAMPLES).toBe('0');
    expect(Number(v.B7_CALLTIMEOUTS)).toBeGreaterThanOrEqual(2);
    expect(v.B7_REASON_NAMES_CALL_TIMEOUT).toBe('True');
    // 用时上界 = TimeoutSeconds 6 + PollSeconds 1 + CallTimeoutSeconds 2 + 3 秒松弛（**不是判据**，只证没挂住）
    expect(Number(v.B7_MS)).toBeLessThanOrEqual(12000);
    expect(v.navLines.some((l) => l.indexOf('NAV_SAMPLE settled=False') >= 0)).toBe(true);

    // B7W：**残帧确实留在盘上**（真链路现测到的竞争形状）⇒ 「不计入采样」必须是**逻辑闸门**而不是删除的成功率
    expect(v.B7W_FILE_ON_DISK).toBe('True');
    expect(Number(v.B7W_FILE_BYTES)).toBeGreaterThan(0);
    expect(v.B7W_SETTLED).toBe('False');
    expect(v.B7W_SAMPLES).toBe('0');
    expect(Number(v.B7W_CALLTIMEOUTS)).toBeGreaterThanOrEqual(2);
    expect(v.B7W_PREV_ABSENT).toBe('True'); // 超时轮也不得把半帧写进「上一帧」，否则下一轮拿它比出「稳定」
    expect(Number(v.B7W_MS)).toBeLessThanOrEqual(12000);

    if (v.IS_WINDOWS === 'True') {
      // 同一条循环再用**真桩**（不经影子）跑一遍 ⇒ 「有界」不是夹具造出来的性质
      expect(v.B7R_SETTLED).toBe('False');
      expect(v.B7R_SAMPLES).toBe('0');
      expect(Number(v.B7R_CALLTIMEOUTS)).toBeGreaterThanOrEqual(1);
      expect(v.B7R_PROBE_GONE).toBe('True');
      expect(v.B7R_REASON_NAMES_CALL_TIMEOUT).toBe('True');
      expect(Number(v.B7R_MS)).toBeLessThanOrEqual(12000);
    }
  });

  // B8：**对照腿**——调用正常返回 + 页身份行 + 画面稳定 ⇒ 判落定（证明 B7 不是「夹具坏了才红」）
  test('B8: 对照腿正常返回时仍按原判据落定（页身份 + 非全黑 + 相邻帧一致）', () => {
    requireProbe();
    expect(v.B8_CALLTIMEOUTS).toBe('0');
    expect(v.BOUNDED_DONE).not.toBeNull(); // 探针跑到最后一行
    if (v.IS_WINDOWS === 'True' && v.DRAWING_AVAILABLE === 'True') {
      expect(v.B8_SETTLED).toBe('True');
      expect(Number(v.B8_SAMPLES)).toBeGreaterThanOrEqual(2);
      expect(v.navLines.some((l) => l.indexOf('NAV_SAMPLE settled=True') >= 0)).toBe(true);
    } else {
      // Linux：取色不可用 ⇒ Compare-ScreenFrames fail-closed 判「不一致」，「落定」在结构上不可能；
      // 这一支验的是「即使如此，循环照旧在预算内收口、且一次调用超时都没有」
      expect(v.B8_SETTLED).toBe('False');
      expect(Number(v.B8_MS)).toBeLessThanOrEqual(14000); // TimeoutSeconds 8 + Poll 1 + Call 2 + 3
    }
  });
});

/**
 * B9–B11 的探针（#1562）：**灭屏前置**那一次 adb 调用的有界性。
 *
 * 为什么必须有这一组（票面现测，同族缺陷的第三个落点）：`Test-ScreenAwake` 原先是
 * `& $AdbExe -s $Serial shell dumpsys power | Out-String` —— 前台同步等待、**没有单次超时**，
 * 而它在**每一页之前**跑 ⇒ 挂住的时机比 #1560 修掉的采样循环**更早**，卡的是「能不能开始截屏」。
 * 它取的还是**文本**（不是 PNG），所以复用不了 `Invoke-BoundedAdbShot` 的调用形态，
 * 但「等待 / 杀树 / 结果作废」那三段必须只有**一份**实现 —— 本探针验的就是这一件新封装
 * （`Invoke-BoundedAdbText`）真的接在执行核上，而不是又抄了一份。
 *
 * 判据口径（沿 #1549 的教训）：**只取定性值与顺序值**。墙钟上界只用来证明「没挂住」，
 * 给的是预算的 10 倍以上余量；结论对不对一律看 `TimedOut` / `Ok` / `State` 这三个定性读数。
 */
function probeAwake(ctx) {
  const script = `
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
. "${AUTO_SHOT}"
$dir = '${ctx.tmp}'
Write-Output ("IS_WINDOWS=" + [bool]$IsWindows)

# 三种桩：亮屏 / 灭屏 / 永不返回（先吐半行再挂 120 秒）
$awakeCmd = Join-Path $dir 'awake.cmd'
Set-Content -LiteralPath $awakeCmd -Encoding ascii -Value @('@echo off', 'echo   mWakefulness=Awake')
$asleepCmd = Join-Path $dir 'asleep.cmd'
Set-Content -LiteralPath $asleepCmd -Encoding ascii -Value @('@echo off', 'echo   mWakefulness=Asleep')
$hangCmd = Join-Path $dir 'hang-awake.cmd'
Set-Content -LiteralPath $hangCmd -Encoding ascii -Value @('@echo off', 'echo PARTIAL', 'ping -n 120 127.0.0.1 > nul')

if ($IsWindows) {
  # ── B9 对照腿：正常返回的 dumpsys **不得**被误判成超时（否则「有界」退化成「把所有调用都杀掉」）
  $a1 = Test-ScreenAwake -AdbExe $awakeCmd -Serial 'FAKE-SERIAL' -WorkDir $dir -TimeoutSeconds 20
  Write-Output ("B9_OK=" + [bool]$a1.Ok)
  Write-Output ("B9_STATE=" + $a1.State)
  Write-Output ("B9_TIMEDOUT=" + [bool]$a1.TimedOut)
  $a2 = Test-ScreenAwake -AdbExe $asleepCmd -Serial 'FAKE-SERIAL' -WorkDir $dir -TimeoutSeconds 20
  Write-Output ("B9B_OK=" + [bool]$a2.Ok)
  Write-Output ("B9B_STATE=" + $a2.State)
  Write-Output ("B9B_TIMEDOUT=" + [bool]$a2.TimedOut)

  # ── B10 挂死腿：预算 2 秒 ⇒ 必须**到点给结论**，且给的是「拿不到唤醒状态」而不是「判成没亮屏之外的第三种后果」
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  $a3 = Test-ScreenAwake -AdbExe $hangCmd -Serial 'FAKE-SERIAL' -WorkDir $dir -TimeoutSeconds 2
  $sw.Stop()
  Write-Output ("B10_OK=" + [bool]$a3.Ok)
  Write-Output ("B10_STATE=" + $a3.State)
  Write-Output ("B10_TIMEDOUT=" + [bool]$a3.TimedOut)
  Write-Output ("B10_HAS_ERROR=" + [bool]($a3.Error -ne ''))
  Write-Output ("B10_MS=" + $sw.ElapsedMilliseconds)
  # 残帧（这里是半份 stdout 文本）不许留在盘上：WorkDir 就是本探针目录，数一下就知道
  Write-Output ("B10_LEFT_TEXT=" + @(Get-ChildItem -LiteralPath $dir -Filter 'adb-call-stdout-*.txt').Count)
  Write-Output ("B10_LEFT_PNG=" + @(Get-ChildItem -LiteralPath $dir -Filter '*.png').Count)
} else {
  # Linux（本仓 CI runner）：没有 cmd.exe ⇒ 断言的是「启动器缺失同样不许挂住」+ 仍走 fail-closed
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  $a3 = Test-ScreenAwake -AdbExe '/bin/echo' -Serial 'FAKE-SERIAL' -WorkDir $dir -TimeoutSeconds 2
  $sw.Stop()
  Write-Output ("B10_OK=" + [bool]$a3.Ok)
  Write-Output ("B10_STATE=" + $a3.State)
  Write-Output ("B10_TIMEDOUT=" + [bool]$a3.TimedOut)
  Write-Output ("B10_BOUNDED=" + [bool]($sw.ElapsedMilliseconds -lt 60000))
  Write-Output ("B10_MS=" + $sw.ElapsedMilliseconds)
}
Write-Output "AWAKE_SENTINEL_DONE=1"
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

describe('灭屏前置那次 adb 调用的有界性（运行期，#1562）', () => {
  let ctx3;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx3 = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'awake-')) };
    probe = probeAwake(ctx3);
    if (probe.ok) {
      [
        'IS_WINDOWS', 'B9_OK', 'B9_STATE', 'B9_TIMEDOUT', 'B9B_OK', 'B9B_STATE', 'B9B_TIMEDOUT',
        'B10_OK', 'B10_STATE', 'B10_TIMEDOUT', 'B10_HAS_ERROR', 'B10_BOUNDED', 'B10_MS',
        'B10_LEFT_TEXT', 'B10_LEFT_PNG',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
      v.awakeLines = String(probe.stdout).split('\n').filter((l) => l.indexOf('AWAKE_PROBE ok=') >= 0);
      v.sentinel = field(probe.stdout, 'AWAKE_SENTINEL_DONE');
    }
  });

  afterAll(() => {
    if (ctx3) {
      try {
        fs.rmSync(ctx3.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红
      }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      '灭屏前置有界性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  // B9：对照腿 —— 真给了 mWakefulness 的两次调用都必须**按原判据**给结论，且都不被当成超时
  test('B9: 正常返回的 dumpsys 不被误判为超时（亮屏判亮、灭屏判灭，两条都真走到）', () => {
    requireProbe();
    if (v.IS_WINDOWS !== 'True') return; // Linux 那支由 B10 断言（没有 cmd.exe 就没有文本可读）
    expect(v.B9_OK).toBe('True');
    expect(v.B9_STATE).toBe('Awake');
    expect(v.B9_TIMEDOUT).toBe('False');
    // 成对的「必不红」面：灭屏必须仍判 Ok=False，且**不是**因为超时（那是另一条出口）
    expect(v.B9B_OK).toBe('False');
    expect(v.B9B_STATE).toBe('Asleep');
    expect(v.B9B_TIMEDOUT).toBe('False');
  });

  // B10：挂死腿 —— 到点必须给结论，且结论是「拿不到唤醒状态」（Ok=False / State=unknown）
  test('B10: 调用永不返回时到点给结论并 fail-closed，残帧（半份文本）不留盘', () => {
    requireProbe();
    expect(v.B10_OK).toBe('False'); // 判定只有「截 / 不截」两档：超时归到不截，不开第三种后果
    expect(v.B10_STATE).toBe('unknown');
    if (v.IS_WINDOWS === 'True') {
      expect(v.B10_TIMEDOUT).toBe('True');
      expect(v.B10_HAS_ERROR).toBe('True');
      // 上界只用来证明「没挂住」，不是判据（预算 2 秒 + 派发开销，这里给 60 秒 = 30 倍余量）
      expect(Number(v.B10_MS)).toBeLessThanOrEqual(60000);
      expect(v.B10_LEFT_TEXT).toBe('0');
      expect(v.B10_LEFT_PNG).toBe('0');
    } else {
      expect(v.B10_BOUNDED).toBe('True');
    }
  });

  // B11：机检行 —— 「通道不返回」与「设备没亮」在日志里必须分得开（是读数、不是判据）
  test('B11: AWAKE_PROBE 机检行按出口打得出，且点名单次预算', () => {
    requireProbe();
    expect(v.sentinel).toBe('1'); // 探针跑到最后一行（否则下面的「按出口」断言可能只是没跑到）
    const lines = v.awakeLines;
    // 每条都必须**整行符合**这个格式（ASCII token：跨语言只传 ASCII，本仓血账是 OEM 码页会把中文变乱码）
    expect(lines.length).toBeGreaterThanOrEqual(1);
    lines.forEach((l) => {
      expect(l.trim()).toMatch(
        /^AWAKE_PROBE ok=(True|False) state=\w+ timedOut=(True|False) callBudgetSeconds=\d+$/,
      );
    });
    if (v.IS_WINDOWS === 'True') {
      // 三条出口各一行：判亮 / 判灭 / 拿不到（超时）
      expect(lines.length).toBeGreaterThanOrEqual(3);
      expect(lines.some((l) => l.indexOf('ok=True state=Awake timedOut=False') >= 0)).toBe(true);
      expect(lines.some((l) => l.indexOf('ok=False state=Asleep timedOut=False') >= 0)).toBe(true);
      expect(lines.some((l) => l.indexOf('ok=False state=unknown timedOut=True') >= 0)).toBe(true);
    } else {
      // Linux：起不了 cmd.exe ⇒ 只有「拿不到唤醒状态」这一条出口可达，且它必须**不是**超时出口
      expect(lines.some((l) => l.indexOf('ok=False state=unknown timedOut=False') >= 0)).toBe(true);
    }
  });
});
