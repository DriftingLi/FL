<#
.SYNOPSIS
    PR 面一次取锁串 ② 与 ④c：一个 `Wait-HxFree` 覆盖两道需要 HBuilderX 主程序的门（#2112）。

.DESCRIPTION
    来路：地图 #2098 / 票「机器时间：PR 粒度选档」(#2104) 的工程半句。

    **改的是什么**：现在 ② (`mp-weixin-check.ps1`) 与 ④c (`kotlin-all-check.ps1`) 各取一次
    `scripts/lib/hx-busy.ps1` 的 agent 互斥锁（两门的 `publish` 段都要 HBuilderX 主程序可达）
    ⇒ 每张运行时面 PR 取锁两次。本脚本把这两次收成**一次**：父进程取锁 → `Set-HxLockOwnerEnv`
    交接 → 依次跑两门。

    **形态照抄先例，不新造协议**：
      · 单租约串多步 = `hx-run.ps1` 的「仅编译 → 真运行 → 逐页截图」（ADR-0011 的 A-3 临界区）；
      · 锁交接 = `$env:HX_LOCK_OWNER`（`hx-busy.ps1` 的 H10/H11）：子进程的 `Wait-HxFree` 认到
        「同一个持有者」就直接复用，打 `HX_BUSY wait=0 result=inherited`，**不重复加锁、不排队**。

    **判据（票面原文）**：日志里**只有一次**取锁记录；两门各自结果行**逐字节不变**
    （`MP_WEIXIN_RESULT` / `KOTLIN_ALL_RESULT`）；排队行只出现一次。本脚本的对应做法：
      · 取锁记录 = `HX_BUSY wait=<秒> result=free`。父进程只调一次 `Wait-HxFree`，把它的
        `-LogPath` 指向本脚本的**同一份**日志 ⇒ 那条记录在日志里只有一处；结果行再复述成
        `locks=<n>`（n = 日志里 `result=free` 的条数，期望 1）。
      · 两门的结果行**原样**落在同一份日志里：本脚本只用 `scripts/lib/gate-tee.ps1` 逐行转发子进程
        输出（**不是** PowerShell 管道 + `Tee-Object`，见 #2147），**不改写、不重排、不加前缀**
        （「逐字节不变」靠「不碰」实现，不靠事后比对）。
      · 排队行（`等待另一个 agent 会话释放 HBuilderX 锁`）只可能来自父进程这一次；子进程走
        `inherited` 分支，早于排队逻辑返回。

    **顺序与失败语义**：永远 ② → ④c，**两门都跑**。第一门红也继续跑第二门 —— 否则「没跑」
    会被读成「这一半没问题」（同 `dev-finish.ps1 -UnitGate` 两腿的先例）。退出码取**最重者**：
    2（环境受阻）> 1（门未过）> 0。

    **边界（票面「不要做」）**：不改两门内部判据、不改 ③ 的 `-i` 串行、不碰移动端 ADR-0046 的
    选档规则、不投工具链大修。本脚本只决定**先后与取锁次数**。

    **收益要写实**：中位收益 ≈ 0（#2112 票面：125 次取锁实测 89 次 wait=0、中位 0 s）；真收益在
    尾部（454 / 501 / 592 s 各一次、120 s 超时一次）—— 取锁次数减半 = 这四种尾部事件的暴露面减半。
    **代价也写实**：一次租约的**持有时间 = 两门之和**；旧形态里两门之间那个几秒的空档，现在不再
    让给别人排队。这不是新增占用（活还是那些活），只是让出的窗口消失。若将来真观察到「排队明显
    变长」，先改这里、别动门。

.PARAMETER Project
    项目根。缺省为本脚本上一级目录（`scripts/` 位于项目根下）。

.PARAMETER Cli
    HBuilderX `cli.exe`。显式给出即用它做忙探测并原样透传给两门；缺省按
    `lib/hx-busy.ps1` 的 `Resolve-HxCliExe` 探测（环境变量 → 正在运行的 HBuilderX → 常见安装位置）。

.PARAMETER HBuilderX
    HBuilderX 安装根（透传给两门）。与 `-Cli` 二选一，`-Cli` 优先。

.PARAMETER Pages
    透传给 ② 的 `-Pages`（缺省 `auto` = 入口页 + 本 PR 改动页；见 #2101 / ADR-0039）。

.PARAMETER HxWaitSeconds
    唯一那次 `Wait-HxFree` 的等待上限（默认 1800 秒）。**比单门默认值宽**：这一次租约要覆盖两门，
    上限应覆盖「两门之和」而不是单门。

.PARAMETER MpArgs / KotlinArgs
    追加给对应门的参数（原样拼在末尾，可覆盖前面的同名参数，例如 `-MpArgs '-SkipBuild'`）。

.PARAMETER TempRoot
    **测试/排障专用**：把锁目录换成该根（`$env:TEMP` 在取锁之前被改写，两门作为子进程继承同一个
    根）。用途是让行为测试用**自己的** `hx-agent.lock`，不去撞维护者正在跑的 HBuilderX 会话。
    日常运行不要给。

.PARAMETER MpScript / KotlinScript
    **测试/排障专用**：门载体覆盖（缺省即真的两门）。同 `dev-finish.ps1 -BehaviorExecutor` 的先例，
    供行为守护用假门验证「一次取锁 + 两门串联」这条接线，不必真编译。

.PARAMETER LogPath
    本脚本的日志（默认 `<项目>/.ci-verify/pr-gates.log`）。两门的 stdout 都 tee 到这里。

.PARAMETER DryRun
    只打印将要跑的两条命令行，**不取锁、不调 CLI、不写日志**（任何机器上都能跑）。

.EXAMPLE
    npm run gates:pr
    pwsh -NoProfile -File scripts/pr-gates.ps1 -DryRun
    pwsh -NoProfile -File scripts/pr-gates.ps1 -Pages pages/dashboard/dashboard,pages/profile/profile
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$HBuilderX,
    [string]$Cli,
    [string]$Pages = 'auto',
    [int]$HxWaitSeconds = 1800,
    [switch]$HxNoWait,
    [string[]]$MpArgs = @(),
    [string[]]$KotlinArgs = @(),
    [string]$TempRoot,
    [string]$MpScript,
    [string]$KotlinScript,
    [string]$LogPath,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# 日志乱码的唯一真源（#2027）：不钉这一行，重定向捕获子进程的 UTF-8 输出会按 GBK(cp936) 解码。
# 非控制台环境设置可能抛 ⇒ try 兜住（先例 = dev-finish.ps1 / hx-run.ps1 顶部同款写法）。
try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }

$MpToken = 'MP_WEIXIN_RESULT'
$KotlinToken = 'KOTLIN_ALL_RESULT'

# ---------- 定位项目与载体 ----------
if (-not $Project) { $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}
if (-not $MpScript) { $MpScript = Join-Path $PSScriptRoot 'mp-weixin-check.ps1' }
if (-not $KotlinScript) { $KotlinScript = Join-Path $PSScriptRoot 'kotlin-all-check.ps1' }
foreach ($p in @($MpScript, $KotlinScript)) {
    if (-not (Test-Path -LiteralPath $p -PathType Leaf)) {
        Write-Host "[error] 门载体不存在：$p" -ForegroundColor Red
        exit 2
    }
}
if (-not $LogPath) { $LogPath = Join-Path (Join-Path $Project '.ci-verify') 'pr-gates.log' }

# 子进程入口：本进程自己的 pwsh（与 hx-run 的 Start-CliLaunchDetached 同款取法）。
$pwshExe = (Get-Process -Id $PID).Path
if (-not $pwshExe) { $pwshExe = 'pwsh' }

# 子门输出转发器（#2147）。**不再用 PowerShell 管道 + Tee-Object**：子门拉起的微信开发者工具（常驻 GUI）
# 会继承那层管道的写句柄 ⇒ Tee 永远等不到 EOF ⇒ 整条 pr-gates 挂死（2026-10-30 实测 1050 s，④c 没开始）。
# 判据库 scripts/lib/gate-tee.ps1（行为守护 utils/prGatesTeeBehavior.test.js 真执行）。
. (Join-Path $PSScriptRoot 'lib\gate-tee.ps1')
$TeeDrainMs = 5000

# ---------- 两门的命令行（**唯一的参数拼装处**：DryRun 与真跑共用这一份）----------
$shared = @{}
if ($Cli) { $shared['Cli'] = $Cli }
if ($HBuilderX) { $shared['HBuilderX'] = $HBuilderX }

$mpArgv = @('-Project', $Project, '-Pages', $Pages) + @($MpArgs)
$kotlinArgv = @('-Project', $Project) + @($KotlinArgs)
foreach ($k in @($shared.Keys)) {
    # 同一份共享参数插在**自定义参数之前** ⇒ -MpArgs / -KotlinArgs 里显式给的能覆盖它（后写者胜）。
    $mpArgv = @('-' + $k, $shared[$k]) + $mpArgv
    $kotlinArgv = @('-' + $k, $shared[$k]) + $kotlinArgv
}

function Format-Plan {
    param([string]$Name, [string]$ScriptPath, [string[]]$Argv)
    $parts = @($Name, ':', $ScriptPath) + $Argv
    return ($parts -join ' ')
}

# ---------- -DryRun：只打印计划，零副作用 ----------
# 必须排在取锁之前：看一眼要跑什么这件事不能先占住 HBuilderX 的锁（同 hx-run 的 C7 顺序要求）。
if ($DryRun) {
    $planCli = if ($Cli) { $Cli } else { '<未探测：正式运行按 Resolve-HxCliExe 探测，或用 -Cli 指定>' }
    Write-Host '=== pr-gates 计划（-DryRun：不取锁、不调 CLI、不写日志）===' -ForegroundColor Cyan
    Write-Host ("project  = {0}" -f $Project)
    Write-Host ("cli      = {0}" -f $planCli)
    Write-Host ("log      = {0}   # 正式运行才写" -f $LogPath)
    Write-Host ("wait     = 唯一一次 Wait-HxFree，上限 {0} 秒（覆盖两门之和）" -f $HxWaitSeconds)
    Write-Host ("mp       : {0}" -f (Format-Plan -Name 'mp-weixin-check.ps1' -ScriptPath $MpScript -Argv $mpArgv))
    Write-Host ("kotlin   : {0}" -f (Format-Plan -Name 'kotlin-all-check.ps1' -ScriptPath $KotlinScript -Argv $kotlinArgv))
    Write-Host '  （顺序固定 ② → ④c；两门都跑，退出码取最重者 2 > 1 > 0）'
    Write-Host 'PR_GATES mode=dryrun exit=ok'
    exit 0
}

# ---------- 锁目录（测试/排障：先换根，再 dot-source —— hx-busy 在加载时就算好锁路径）----------
if ($TempRoot) {
    New-Item -ItemType Directory -Force -Path $TempRoot | Out-Null
    $env:TEMP = $TempRoot
    Write-Host "[warn] -TempRoot：锁目录已换到 $TempRoot/hx-agent.lock（只在测试/排障用）" -ForegroundColor Yellow
}

# ---------- 解析 cli 并取**这一次**锁 ----------
. (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')

$cliExe = Resolve-HxCliExe -Explicit $Cli -HxDir $HBuilderX
if (-not $cliExe) {
    Write-Host '[error] 找不到可用的 HBuilderX cli.exe（用 -Cli <路径> / -HBuilderX <安装根> / $env:HBuilderX_CLI 指定）。' -ForegroundColor Red
    Write-Host '        本脚本不降级跑门：两门都要主程序可达，猜一个路径只会把它们一起判成环境受阻。' -ForegroundColor Red
    Write-Host 'PR_GATES mode=run exit=env mp=skipped kotlin=skipped locks=0 reason=cli-not-found'
    exit 2
}

$logDir = Split-Path -Parent $LogPath
if ($logDir) { New-Item -ItemType Directory -Force -Path $logDir | Out-Null }
Set-Content -LiteralPath $LogPath -Encoding utf8 -Value @(
    "# pr-gates 开始 $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')（#2112：PR 面一次取锁串 ② 与 ④c）",
    "# project = $Project",
    "# cli = $cliExe",
    "# 两门的 stdout 原样 tee 在本文件；结果行不加前缀、不改写"
)

Write-Host ''
Write-Host '>>> 取唯一一次 HBuilderX 锁（两门共用这一份租约）' -ForegroundColor DarkGray
$hx = Wait-HxFree -CliExe $cliExe -TimeoutSeconds $HxWaitSeconds -NoWait:$HxNoWait -LogPath $LogPath
Set-HxLockOwnerEnv

$startedAt = Get-Date
$results = [ordered]@{}
$exitState = 'ok'

function Invoke-Gate {
    <#
      跑一门：子进程 `pwsh -File <载体> @argv`，stdout/stderr 原样 tee 进本脚本日志。
      判成败**不看退出码单独一项**（两门的语义都是 0 过 / 1 门未过 / 2 环境受阻，但也要求结果行
      真的打出来了）：0 而缺结果行 ⇒ no-line（fail-closed，别把「崩了」读成「过了」）。
    #>
    param([string]$Name, [string]$ScriptPath, [string[]]$Argv, [string]$Token, [string]$LogPath)

    Write-Host ''
    Write-Host (">>> [{0}] {1}" -f $Name, (Format-Plan -Name (Split-Path -Leaf $ScriptPath) -ScriptPath $ScriptPath -Argv $Argv)) -ForegroundColor Cyan
    Add-Content -LiteralPath $LogPath -Encoding utf8 -Value @(
        "",
        (">>> pr-gates gate={0} start={1}" -f $Name, (Get-Date -Format 'HH:mm:ss'))
    )

    $argv = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ScriptPath) + $Argv
    # ⚠️ 两条都不能写：
    #   ① `$out = @(& ... | Tee-Object ...)` —— Tee 的产物被赋值**吞掉**，控制台一行看不到（门跑完了却静默）；
    #   ② `& $pwshExe @argv 2>&1 | Tee-Object ... | ForEach-Object` —— 子门拉起的常驻 IDE 继承这层管道的
    #      写句柄，EOF 永不到来 ⇒ **整条 pr-gates 挂死**（#2147：② 已出结果、门却挂到被外部 killed，④c 没开始）。
    # 现在是自带的有界收流器：逐行回显 + 逐行落盘 + 主进程退出后只收 DrainMs 就放手（判据库 gate-tee.ps1）。
    $tee = Invoke-GateTee -FilePath $pwshExe -Arguments $argv -LogPath $LogPath -DrainMs $TeeDrainMs
    $code = $tee.ExitCode
    $out = @($tee.Lines)

    $found = @($out | ForEach-Object { "$_" } | Where-Object { $_ -match ("^" + $Token + "(\s|$)") })
    $line = if ($found.Count -gt 0) { $found[$found.Count - 1] } else { '' }

    $state = if ($code -eq 2) { 'env' }
             elseif ($code -eq 1) { 'fail' }
             elseif ($code -eq 0 -and $line) { 'ok' }
             else { 'no-line' }

    if (-not $line) {
        Write-Host ("⚠️ [{0}] 退出码 {1} 但没有打出 {2} 结果行 —— 记 state={3}（有结果行才谈得上「原样」）" -f $Name, $code, $Token, $state) -ForegroundColor Yellow
    }
    Add-Content -LiteralPath $LogPath -Encoding utf8 -Value (">>> pr-gates gate={0} exit={1} state={2} seconds={3}" -f $Name, $code, $state, [int]((Get-Date) - $startedAt).TotalSeconds)

    return [pscustomobject]@{ Name = $Name; State = $state; Code = $code; Line = $line }
}

try {
    $mp = Invoke-Gate -Name 'mp' -ScriptPath $MpScript -Argv $mpArgv -Token $MpToken -LogPath $LogPath
    $results['mp'] = $mp.State

    $kotlin = Invoke-Gate -Name 'kotlin' -ScriptPath $KotlinScript -Argv $kotlinArgv -Token $KotlinToken -LogPath $LogPath
    $results['kotlin'] = $kotlin.State

    # 退出码取最重者：环境受阻压过门未过，门未过压过通过。no-line 按环境受阻算（别把「崩了」读成「过了」）。
    $states = @($mp.State, $kotlin.State)
    $exitState = if ($states -contains 'env' -or $states -contains 'no-line') { 'env' }
                 elseif ($states -contains 'fail') { 'fail' }
                 else { 'ok' }
}
catch {
    # 子进程都起不来（pwsh 缺失 / 权限）时也要**留下结果行**：否则调用方只看到「脚本没输出」，
    # 分不清「门没过」与「工具链坏了」，与本仓最忌讳的「静默」同形。
    $exitState = 'env'
    Write-Host "[error] 跑门时抛异常：$($_.Exception.Message)" -ForegroundColor Red
    Add-Content -LiteralPath $LogPath -Encoding utf8 -Value ("[error] pr-gates 抛异常：{0}" -f $_.Exception.Message)
}
finally {
    # 锁与 env 的清理必须在 finally：红跑 / 抛异常 / 中途 Ctrl+C 都不能把锁留给下一个会话。
    Clear-HxLockOwnerEnv
    Release-HxLock
}

# ---------- 取锁次数自证（票面判据之一）----------
# 期望 1：只有父进程那一次。子进程走 `inherited`（不打 free），所以这个数是「真取锁次数」。
# **只报不改退出码**：门的结论由门定；取锁次数是本文具的不变量，异常时喊出来给人看，别连坐门的绿。
$freeCount = -1
try {
    $freeCount = @(Get-Content -LiteralPath $LogPath | Where-Object { $_ -match 'HX_BUSY wait=\d+ result=free' }).Count
}
catch { $freeCount = -1 }

$expected = if ($hx.Result -eq 'inherited') { 0 } else { 1 }
if ($freeCount -ne $expected) {
    Write-Host ("⚠️ 取锁记录数 = {0}（期望 {1}，acquire={2}）—— 本门的价值就在「只有一次取锁」，先查为什么。" -f $freeCount, $expected, $hx.Result) -ForegroundColor Yellow
}

$seconds = [int]((Get-Date) - $startedAt).TotalSeconds
# 缺字段落 skipped 而不是空串：结果行是机器读的，「没跑到」必须有自己的取值（空字段会被读成「忘了填」）。
$mpState = if ($results.Contains('mp')) { $results['mp'] } else { 'skipped' }
$kotlinState = if ($results.Contains('kotlin')) { $results['kotlin'] } else { 'skipped' }
$line = 'PR_GATES mode=run mp={0} kotlin={1} exit={2} locks={3} acquire={4} seconds={5} log={6}' -f `
    $mpState, $kotlinState, $exitState, $freeCount, $hx.Result, $seconds, $LogPath
Write-Host ''
Write-Host $line
Add-Content -LiteralPath $LogPath -Encoding utf8 -Value @("", $line)

if ($exitState -eq 'ok') { exit 0 }
if ($exitState -eq 'fail') { exit 1 }
exit 2
