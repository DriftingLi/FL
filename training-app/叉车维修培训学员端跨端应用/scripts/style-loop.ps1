<#
.SYNOPSIS
    样式内循环入口（issue #1543）：改完样式后一条命令走完「编译期诊断 → 逐页截图 → 与基线做像素比」，
    终端**只回一行**机检判据。

.DESCRIPTION
    为什么要有它（票面素材来源的那句裁决）：样式类改动此前要么跑
    `npm run dev:finish -- -Level standard`（那会把契约测试与整模块编译门一起拖进来，还要人读满屏步骤日志），
    要么手工串 `hx:compile-only` → `hx:run` → `capture:device` → `png-diff` 三四条命令并逐条读输出 ——
    两种形态都在同一件事上加价：**上下文膨胀**与**执行很慢**是同一个出口。
    本入口把这条链收成一个进程、收成一个 `STYLE_LOOP` 行：会话默认只读那一行，
    日志仍在 `.ci-verify/style-loop.log` 里全量落盘，要看细节再去看它。

    动作顺序（每步判据都来自既有 lib，本文件**不重写任何判据**）：

      1. 预检「有没有活」（`lib/style-loop.ps1` 的 Get-StyleLoopWorkState）——没改动页面就直接红，
         **不取锁、不启动 HBuilderX**（一轮启动是分钟级，白跑就是白跑）。
      2. 环境（`lib/env-check.ps1` 的 Test-BuildEnv：adb 设备 + HBuilderX cli + 忙探测）。
      3. **一个 HBuilderX 锁临界区**覆盖 4–6（同 `dev-finish.ps1` 的 A-3 裁决：后两步共享设备状态，
         中途被别的会话插进来会截到别人的页面）。锁交接走 `$env:HX_LOCK_OWNER`，失败路径经 `finally` 释放。
      4. 编译期诊断 = `hx-run.ps1 -CompileOnly`（经 `lib/test-compile.ps1`）。**它不碰设备**，
         且正是常驻层点名的那条：类型名义不一致、uvue 样式规则违反、模板编译错误**都是编译期诊断**，
         不该拖到真机上验。放在装机之前还有一条只有在这里才成立的理由：**编译没过时设备上跑的是
         上一次构建的页面**，截它做像素比会把「没生效」读成「没变化」。
      5. 真运行到设备（`lib/build-deploy.ps1`，判定 = 设备侧事实相对基线前进）。
      6. 逐页截图（`lib/auto-screenshot.ps1`）——**只截改动页**；「这一轮截哪些页」的唯一真源仍是它
         （它从 git diff 推导并按 pages.json 校验），本入口只决定「要不要开始跑」。
      7. 像素比（`lib/screenshot-diff.ps1` + `lib/screenshot-gate.ps1` 的 Get-PngDiffVerdict，
         与 `dev:finish` 步骤 7 同一个判据、同一个「本轮产物」起点）。

    退出码（沿用本仓既有语义）：0 = 绿；1 = 红（判据判出的问题）；2 = 环境不可用。

    明确**不做**（票面「不在本票射程」与门归属四条）：

      · 不进 CI，不是门：机检行恒带 `gate=none`。它不改任何验收门的触发面与归属 ——
        真机门 / 微信门 / 契约门 / 编译门的结论仍只由各自的门脚本产出。
      · 微信小程序自动化截图不进这条链（现读三票 12 次调用 9 次失败，成功的 3 次全部逐页导航被跳过）。
      · 不跑契约测试、不跑整模块编译门 —— 那是中循环「每个视觉满意点一次」的活，
        把它们塞进每次样式迭代就是本票要消灭的串行成本。
      · 不新增串行资源，也不额外取 HBuilderX 之外的锁。

    工作树纪律（票面第 6 条）：进临界区之前先给 `manifest.json` / `pages.json` / `platformConfig.json`
    拍**字节**快照，退出时（含异常与 `exit` 路径 —— 实测 PowerShell 的 `finally` 在 `exit` 时仍执行）
    把被回写的项写回原字节。`lib/build-deploy.ps1` 自己那层还原比的是**文本**、还原用 `Set-Content`，
    BOM 或行尾变了它看不出来而 git 看得出来，所以这层字节还原是它的兜底而不是重复。

.OUTPUTS
    终端：恰好一行 `STYLE_LOOP gate=none verdict=… phase=… changed=… shot=… log=… pages=… shots=…
    reason=… secs=…`（≤200 B，恒 ASCII，判红时点名至少一页）。其余全部进 `.ci-verify/style-loop.log`。

.NOTES
    为什么分「包装层（父进程）」与「链体（子进程 `-Inner`）」两层，而不是一个进程把活全干完：
      `lib/hx-busy.ps1` 的 `Wait-HxFree` 在超时或锁被占时是**直接 `exit 2`**（本仓既有语义，不许为了迁就入口
      去改并发纪律）。链体内联在父进程里就会被它**就地截断** —— 那一轮既没有机检行也没有日志，
      正是本入口要消灭的形态。分成两层后父进程只认「子进程打出了什么 + 退出码是多少」，
      子进程怎么死的（超时 exit / 崩溃 / 被 kill）都拿得出一行结论；判据在
      `lib/style-loop.ps1` 的 `Resolve-StyleLoopChildOutput`（纯函数，被守护成对真跑：有行⇒原样 relay、
      无行⇒合成并按红）。副作用是包装层必须钉 `[Console]::OutputEncoding = UTF8` ——
      不钉就是把子进程的中文输出按 GBK 解成 mojibake 落进日志（#1542 缺陷 5、auto-screenshot 的 S20 同源血账）。

.EXAMPLE
    npm run style:loop
    npm run style:loop -- -Device b32d8398
    npm run style:loop -- -Pages 'pages/profile/settings'      # 首轮建基线（改动集里还没有 .uvue 时）
    npm run style:loop -- -UpdateBaseline                      # 人看过 diff、确认是有意改动之后刷基线
    npm run style:loop -- -DryRun                              # 只看计划与参数，不执行
#>
[CmdletBinding()]
param(
    [string]$Device,
    [string]$CliPath,
    [string]$ProjectDir,
    # 显式页面清单（逗号分隔的项目内页面路径）。给了它就绕开预检 —— 首轮建基线时改动集里
    # 还没有 `.uvue`，靠推导会得到「没活」，而人就是要建这一轮的基线。
    [string]$Pages = '',
    [int]$MaxScreenshotPages = 5,
    # 像素判据参数：与 dev:finish 同一套默认值（0.5% 阈值 + 可选忽略状态栏行），不在这里另定口径。
    [ValidateRange(0.0, 1.0)]
    [double]$PixelThreshold = 0.005,
    [ValidateRange(0, 10000)]
    [int]$IgnoreTopRows = 0,
    [ValidateRange(0, 10000)]
    [int]$IgnoreBottomRows = 0,
    [switch]$UpdateBaseline,
    [int]$HxWaitSeconds = 1800,
    [int]$HxRunTimeoutSeconds = 1800,
    # 机检行字节预算（票面第 3 条：≤200 B）。**上限就是那条承诺本身** ⇒ 用参数范围锁死，
    # 而不是只写在注释里等人遵守（评审自查 2026-10-05：原 `ValidateRange(40, 4096)` 让调用方
    # 一句 `-MaxLineBytes 4096` 就把 ≤200 B 的承诺放宽成 2 KB）。下限 40 留给守护验收缩阶梯。
    [ValidateRange(40, 200)]
    [int]$MaxLineBytes = 200,
    [switch]$DryRun,
    # 内部形态：本脚本被自己以子进程方式拉起时走链体分支（见上方 .NOTES）。不是给人用的开关。
    [switch]$Inner
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# 子进程输出解码用 UTF-8（本仓先例：dev-finish.ps1 / frontier-run.ps1 / mp-weixin-check.ps1）。
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

$started = Get-Date
$logRel = '.ci-verify/style-loop.log'

. (Join-Path $PSScriptRoot 'lib\style-loop.ps1')

# ============================================================
# 父进程侧的收口：**任何**在链体之前/之外失败的形态，也必须是「恰好一行 + 非零退出」。
#   自查轮（2026-10-05 code review）实测两种形态：`-ProjectDir` 指错目录、`.ci-verify` 被占成文件
#   ⇒ `Resolve-Path` / `Add-Content` 直接抛，终端上出来的是一坨 PowerShell 错误栈、
#   **一行判据都没有** —— 正是本入口要消灭的那个形态。`log=unwritten` 是诚实的：
#   落不了盘就不许在行里声称产物存在。
# ============================================================
function Complete-StyleLoopAbort {
    param([string]$Phase, [string]$Reason, [int]$Exit, [int]$BudgetBytes)
    $v = [pscustomobject]@{
        Verdict = 'env'; Phase = $Phase; Changed = 0; Pages = 0; Shots = 0
        Reason  = $Reason; ChangedPages = @()
    }
    $fmt = Format-StyleLoopLine -Verdict $v -LogPath 'unwritten' -Seconds 0 -MaxBytes $BudgetBytes
    Write-Host $fmt.Line
    exit $Exit
}

if (-not $ProjectDir) { $ProjectDir = (Join-Path $PSScriptRoot '..') }
if (-not (Test-Path -LiteralPath $ProjectDir -PathType Container)) {
    Complete-StyleLoopAbort -Phase 'usage' -Reason 'bad-project-dir' -Exit 2 -BudgetBytes $MaxLineBytes
}
$ProjectDir = (Resolve-Path -LiteralPath $ProjectDir).Path
$logPath = Join-Path $ProjectDir $logRel
$logWritable = $true
try {
    $null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $logPath)
}
catch {
    $logWritable = $false
}

# ============================================================
# 链体的唯一出口：算结论 ⇒ 打那一行 ⇒ 把退出码交回调用方。
#   **任何阶段**都从这里收口，所以「半路就红」与「走完全程」共用同一种产出形状
#   —— 形状由 lib 的纯函数保证，被 utils/styleLoopBehavior.test.js 真跑断言。
# ============================================================
function Complete-StyleLoopRun {
    param($Inputs, [datetime]$Started, [int]$MaxLineBytes, [string]$LogRelativePath)

    $seconds = [int]((Get-Date) - $Started).TotalSeconds
    $verdict = Get-StyleLoopVerdict @Inputs
    $fmt = Format-StyleLoopLine -Verdict $verdict -LogPath $LogRelativePath -Seconds $seconds -MaxBytes $MaxLineBytes
    Write-Host "      结论字节数=$($fmt.Bytes) 收缩档=$($fmt.Ladder) 点名=$(@($fmt.ShotNames) -join '|')"
    Write-Host $fmt.Line
    return [int]$verdict.ExitCode
}

# ============================================================
# 链体（子进程形态）
#   这里 Write-Host 的自由输出会被包装层整段收进日志，只有那一行 STYLE_LOOP 是结论。
# ============================================================
function Invoke-StyleLoopChain {
    param(
        [string]$Device,
        [string]$CliPath,
        [string]$ProjectDir,
        [string]$Pages,
        [int]$MaxScreenshotPages,
        [double]$PixelThreshold,
        [int]$IgnoreTopRows,
        [int]$IgnoreBottomRows,
        [bool]$UpdateBaseline,
        [int]$HxWaitSeconds,
        [int]$HxRunTimeoutSeconds,
        [int]$MaxLineBytes,
        [string]$LogRelativePath
    )

    . (Join-Path $PSScriptRoot 'lib\env-check.ps1')
    . (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
    . (Join-Path $PSScriptRoot 'lib\test-compile.ps1')
    . (Join-Path $PSScriptRoot 'lib\build-deploy.ps1')
    . (Join-Path $PSScriptRoot 'lib\auto-screenshot.ps1')
    . (Join-Path $PSScriptRoot 'lib\screenshot-diff.ps1')

    # 阶段结果先落「什么都没做」的形状：任何一阶段没走到，结论就按那条判红（fail-closed），
    # 而不是留一个空字段让渲染层去猜。
    $inputs = @{
        HasWork      = $true; EnvError = ''; CompileOk = $null; CompileExit = $null
        Deployed     = $false; ShotError = ''
        PageCount    = 0; ShotCount = 0; ChangedCount = 0; ChangedPages = @()
        DiffVerdict  = $null
    }

    # ---------- 1) 预检：有没有活 ----------
    Write-Host '[1/7] 预检改动集'
    $work = if ($Pages) { Get-StyleLoopWorkState -PagesExplicit } else { Get-StyleLoopWorkState -ProjectDir $ProjectDir }
    Write-Host "      haswork=$($work.HasWork) pages-under-diff=$($work.PageFileCount) source=$($work.Source)"
    $inputs.HasWork = [bool]$work.HasWork
    if (-not $work.HasWork) {
        # 没活就直接收口：**不取锁、不启动 HBuilderX** —— 这是预检存在的全部理由
        return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
    }

    # 配置字节快照：早于任何平台交互，且**外层 finally 无条件还原**（含 Wait-HxFree 的 exit 路径与异常路径）
    $configFp = Get-StyleLoopConfigFingerprint -ProjectDir $ProjectDir
    $lockTaken = $false

    try {
        # ---------- 2) 环境 ----------
        Write-Host '[2/7] 环境检测'
        $envResult = Test-BuildEnv -ProjectDir $ProjectDir -Device $Device -CliPath $CliPath -HxWaitSeconds 120
        if (-not $envResult.Ok) {
            $inputs.EnvError = [string]$envResult.Error
            Write-Host "      环境不可用：$($envResult.Error)"
            return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
        }
        $Device = $envResult.Device
        Write-Host "      设备 $Device 在线，HBuilderX cli 可用（$($envResult.CliPath)）"

        # ---------- 3) HBuilderX 锁临界区（覆盖 4–6）----------
        $hxLog = Join-Path $ProjectDir '.ci-verify/style-loop-hx.log'
        Write-Host '[3/7] 进入 HBuilderX 锁临界区'
        # Wait-HxFree 超时/锁被占时自己 `exit 2`（既有并发语义，不在此拦截 —— 拦就是给并发纪律开第二条口径）；
        # 届时包装层会合成结论行，链体的 finally 仍会执行（实测 exit 会走完 finally）。
        $null = Wait-HxFree -CliExe $envResult.CliPath -TimeoutSeconds $HxWaitSeconds -LogPath $hxLog
        $lockTaken = $true
        Set-HxLockOwnerEnv
        try {
            # ---------- 4) 编译期诊断（仅编译，不碰设备）----------
            Write-Host '[4/7] 编译期诊断（hx-run -CompileOnly）'
            $compile = Invoke-TestAndCompile -Level 'quick' -ProjectDir $ProjectDir -CliPath $envResult.CliPath `
                -SkipTests -QuickCompile -HxRunTimeoutSeconds $HxRunTimeoutSeconds
            $inputs.CompileOk = [bool]$compile.Ok
            $inputs.CompileExit = [int]$compile.ExitCode
            Write-Host "      exit=$($compile.ExitCode) ok=$($compile.Ok) line=$($compile.CompileResult) err=$($compile.Error)"
            if (-not [bool]$compile.Ok) {
                # 编译没过 ⇒ 设备上的画面还是上一次构建的 ⇒ 装机与截图都不必做
                return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
            }

            # ---------- 5) 真运行到设备 ----------
            Write-Host '[5/7] 真运行到设备'
            $deploy = Invoke-BuildAndDeploy -CliPath $envResult.CliPath -Device $Device -ProjectDir $ProjectDir `
                -Level 'standard' -RunTimeoutSeconds $HxRunTimeoutSeconds
            # 判「到了设备」只认 `Deployed`（设备侧事实相对基线前进），**不认 `Ok`**：
            # `Ok` 是「调用成立」，`lib/build-deploy.ps1` 的 quick 那一支明写 `Ok=$true / Deployed=$false`，
            # 拿 `Ok` 顶位就是把「没部署」读成「部署了」（自查轮 2026-10-05 抓到的一处方向性错误）。
            $inputs.Deployed = [bool]$deploy.Deployed
            Write-Host "      ok=$($deploy.Ok) deployed=$($deploy.Deployed) $($deploy.RunLine) reason=$($deploy.Reason)"
            if (-not $deploy.Ok) {
                return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
            }

            # ---------- 6) 逐页截图（只截改动页）----------
            Write-Host '[6/7] 逐页截图'
            $shot = Invoke-AutoScreenshot -Device $Device -CliPath $envResult.CliPath -ProjectDir $ProjectDir `
                -MaxPages $MaxScreenshotPages -RunStartedAt $started -Pages $Pages
            $inputs.PageCount = @($shot.TargetPages).Count
            $inputs.ShotCount = @($shot.Screenshots).Count
            if (-not $shot.Ok) { $inputs.ShotError = [string]$shot.Error }
            if (@($shot.Screenshots).Count -eq 0 -and [string]::IsNullOrWhiteSpace($inputs.ShotError)) {
                # 「一张都没截到」即使 Ok 为真也不往下比 —— 没有本轮截图就没有判据
                $inputs.ShotError = '没有本轮截图'
            }
            Write-Host "      target=$($inputs.PageCount) shots=$($inputs.ShotCount) skipped=$(@($shot.Skipped).Count) error=$($shot.Error)"
        }
        finally {
            if ($lockTaken) {
                Clear-HxLockOwnerEnv
                Release-HxLock
                Write-Host '[3/7] 已退出锁临界区'
            }
        }

        if (-not [string]::IsNullOrWhiteSpace($inputs.ShotError)) {
            return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
        }

        # ---------- 7) 像素比（锁外，纯本地）----------
        Write-Host '[7/7] 与基线做像素比'
        $diff = Compare-ScreenshotBaseline -ProjectDir $ProjectDir -UpdateBaseline:$UpdateBaseline `
            -PixelThreshold $PixelThreshold -IgnoreTopRows $IgnoreTopRows -IgnoreBottomRows $IgnoreBottomRows `
            -RunStartedAt $started
        $changedPages = @()
        foreach ($entry in @($diff.Diff)) {
            $parts = "$entry" -split ':'
            if (@($parts).Count -ge 2 -and $parts[1] -ne '无变化') { $changedPages += $parts[0] }
        }
        $inputs.DiffVerdict = Get-PngDiffVerdict -DiffResult $diff -UpdateBaseline:$UpdateBaseline
        $inputs.ChangedCount = [int]$diff.ChangedCount
        $inputs.ChangedPages = $changedPages
        if ([string]$inputs.DiffVerdict.Action -eq 'write-baseline') {
            # 判定表里 write-baseline 那一支的**动作**归调用方执行（与 dev-finish 步骤 7 同一口径）；
            # 只转述结论不执行，基线就永远是空的 ⇒ 每轮都「首次运行」、判据永不咬（ADR-0008:419）。
            $seed = Copy-StyleLoopBaselineShot -ProjectDir $ProjectDir -RunStartedAt $started
            Write-Host "      基线已建立：$(@($seed.Copied) -join ',')（跳过非本轮 $(@($seed.Skipped).Count) 张）"
        }
        Write-Host "      changed=$($diff.ChangedCount) mode=$($diff.Mode) ok=$($inputs.DiffVerdict.Ok) action=$($inputs.DiffVerdict.Action) msg=$($inputs.DiffVerdict.Message)"

        return (Complete-StyleLoopRun -Inputs $inputs -Started $started -MaxLineBytes $MaxLineBytes -LogRelativePath $LogRelativePath)
    }
    finally {
        $restore = Restore-StyleLoopConfig -Fingerprint $configFp
        Write-Host "      配置字节还原 restored=$(@($restore.Restored) -join ',') appeared=$(@($restore.Appeared) -join ',') unchanged=$(@($restore.Unchanged) -join ',')"
    }
}

# ============================================================
# 转发实参只有**一张表**：链体的入参与子进程的命令行都从它派生（一份实参、两处消费）。
#   为什么值得改这一刀：这里原本把同一串参数抄了**三遍**（入口 param 块 / 链体 param 块 /
#   `$childArgs` 手工清单），而手工清单**漏抄一个不会报错** —— 子进程安静地用默认值跑，
#   于是人传的 `-IgnoreTopRows 80` 到了链体变成 0，判据就不再是人以为的那个判据。
#   Duplicated Code 里最贵的那一类：漏抄即静默改语义。
#   再加一道形状自检：表里的键必须是链体参数名，打错键名当场出一行结论，而不是静默走默认值。
# ============================================================
$loopOpts = @{
    Device              = $Device
    CliPath             = $CliPath
    ProjectDir          = $ProjectDir
    Pages               = $Pages
    MaxScreenshotPages  = $MaxScreenshotPages
    PixelThreshold      = $PixelThreshold
    IgnoreTopRows       = $IgnoreTopRows
    IgnoreBottomRows    = $IgnoreBottomRows
    UpdateBaseline      = [bool]$UpdateBaseline
    HxWaitSeconds       = $HxWaitSeconds
    HxRunTimeoutSeconds = $HxRunTimeoutSeconds
    MaxLineBytes        = $MaxLineBytes
}
$chainParamNames = @((Get-Command Invoke-StyleLoopChain).Parameters.Keys)
$unknownOpts = @($loopOpts.Keys | Where-Object { $chainParamNames -notcontains $_ })
if ($unknownOpts.Count -gt 0) {
    Complete-StyleLoopAbort -Phase 'usage' -Reason "unknown-opt-$($unknownOpts -join ',')" -Exit 2 -BudgetBytes $MaxLineBytes
}

# ============================================================
# -DryRun：只打计划（**不是判据**，故 verdict=plan；不建子进程、不取锁、不碰设备）
# ============================================================
if ($DryRun) {
    Write-Host '=== style:loop 计划（-DryRun：不执行任何操作）===' -ForegroundColor Cyan
    Write-Host '  1. 预检改动集里有没有 pages/ 下的改动（无 ⇒ 直接红，不启动 HBuilderX）'
    Write-Host '  2. 环境检测（adb 设备 + HBuilderX cli + 忙探测）'
    Write-Host '  3. HBuilderX 锁临界区（4–6 共享一把锁：仅编译 → 真运行 → 逐页截图）'
    Write-Host '  4. 编译期诊断：hx-run -CompileOnly（不推送 / 不启动 / 不需要设备）'
    Write-Host '  5. 真运行到设备：hx-run（判定 = 设备侧事实相对基线前进）'
    Write-Host ("  6. 逐页截图：auto-screenshot（只截改动页，最多 {0} 页）" -f $MaxScreenshotPages)
    Write-Host '  ── 退出锁临界区，按字节还原 manifest.json / pages.json / platformConfig.json ──'
    Write-Host '  7. 与基线做像素比：screenshot-diff + Get-PngDiffVerdict（与 dev:finish 步骤 7 同源）'
    Write-Host ''
    Write-Host ("  Device='{0}'  Pages='{1}'  PixelThreshold={2}  IgnoreTopRows={3}  IgnoreBottomRows={4}  UpdateBaseline={5}" -f `
        $Device, $Pages, $PixelThreshold, $IgnoreTopRows, $IgnoreBottomRows, $UpdateBaseline)
    Write-Host ("  HxWaitSeconds={0}  HxRunTimeoutSeconds={1}  MaxLineBytes={2}" -f $HxWaitSeconds, $HxRunTimeoutSeconds, $MaxLineBytes)
    Write-Host '  本入口不是门：机检行恒带 gate=none，不改任何验收门的触发面与归属。' -ForegroundColor Yellow

    $planVerdict = [pscustomobject]@{
        Verdict = 'plan'; Phase = 'dryrun'; Changed = 0; Pages = 0; Shots = 0
        Reason  = 'dry-run'; ChangedPages = @()
    }
    $planLine = Format-StyleLoopLine -Verdict $planVerdict -LogPath $logRel -Seconds 0 -MaxBytes $MaxLineBytes
    Write-Host $planLine.Line
    exit 0
}

# ============================================================
# 链体形态（子进程）
# ============================================================
if ($Inner) {
    $code = Invoke-StyleLoopChain @loopOpts -LogRelativePath $logRel
    exit $code
}

# ============================================================
# 包装层（父进程）：拉起子进程 ⇒ 子进程全部输出进日志 ⇒ 终端只 relay 那一行
# ============================================================
$childArgs = @('-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', $PSCommandPath, '-Inner')
foreach ($key in @($loopOpts.Keys | Sort-Object)) {
    $val = $loopOpts[$key]
    if ($val -is [bool]) {
        # switch 参数只在真值时传（传 `-Foo:$false` 也行，但清单里出现的就是「真做了的事」）
        if ($val) { $childArgs += "-$key" }
        continue
    }
    $text = "$val"
    if (-not $text) { continue }   # 空串 = 没给 ⇒ 让子进程用自己的默认值
    if ($val -is [double]) {
        # 阈值用 InvariantCulture 写：中文/德语区域设置下 `"$PixelThreshold"` 会变成 `0,005`，
        # 传出去就是把默认阈值静默改成别的数（同 lib/screenshot-diff.ps1 传 CLI 参数时的那条理由）。
        $text = $val.ToString('0.######', [System.Globalization.CultureInfo]::InvariantCulture)
    }
    $childArgs += @("-$key", $text)
}

$childExit = 0
$spawnFailed = $false
try {
    $captured = & pwsh @childArgs 2>&1 | Out-String
}
catch {
    $captured = "包装层拉起子进程失败：$($_.Exception.Message)"
    $spawnFailed = $true
    $childExit = 2
}
# 只在**真的拉起了子进程**时才认 `$LASTEXITCODE`：它是原生命令的自动变量，
# spawn 失败时里面还是**上一条命令**留下的陈旧值（可能是 0）—— 拿它覆盖上面那个 2，
# 就会把「子进程根本没起来」伪装成「子进程干净地退了 0」，正对本入口的反面承诺。
if (-not $spawnFailed -and $null -ne $LASTEXITCODE) { $childExit = [int]$LASTEXITCODE }

$elapsed = [int]((Get-Date) - $started).TotalSeconds
$resolved = Resolve-StyleLoopChildOutput -Captured $captured -ChildExitCode $childExit `
    -LogPath $logRel -Seconds $elapsed -MaxBytes $MaxLineBytes

# 日志：表头绑定本次 head 与工作树脏净，再加子进程的完整输出与 relay 的那一行。
# 「会话侧只读一行」的前提是**这一行以外**的东西仍在磁盘上 —— 出得去、复算得了。
$headShort = ''
$dirtyCount = 0
try { $headShort = "$(& git -C $ProjectDir rev-parse --short HEAD 2>$null)".Trim() } catch { }
try { $dirtyCount = @(& git -C $ProjectDir status --porcelain 2>$null | Where-Object { $_ }).Count } catch { }
$logBlock = @(
    "# ---- style:loop $($started.ToString('yyyy-MM-dd HH:mm:ss')) 用时 ${elapsed}s head=$headShort dirty_files=$dirtyCount from_child=$($resolved.FromChild) exit=$($resolved.ExitCode) ----"
    "$($resolved.Body)".TrimEnd()
    $resolved.Line
)
$logOk = $logWritable
if ($logOk) {
    try {
        Add-Content -LiteralPath $logPath -Value $logBlock -Encoding utf8
    }
    catch {
        $logOk = $false
    }
}
$finalLine = [string]$resolved.Line
if (-not $logOk) {
    # 落不了盘 ⇒ 那一行里的 `log=` 必须跟着改口：行不能指着一个不存在的产物当证据。
    # 细节走 **stderr**（终端 stdout 仍恰好一行 —— 会话侧读的就是那一行，这里不新增噪音），
    # 静默丢弃才是本仓反复防的形态。
    try { [Console]::Error.WriteLine("style:loop 日志未落盘：$logPath") } catch { }
    $finalLine = ($finalLine -replace 'log=\S+', 'log=unwritten')
}

Write-Host $finalLine
exit $resolved.ExitCode
