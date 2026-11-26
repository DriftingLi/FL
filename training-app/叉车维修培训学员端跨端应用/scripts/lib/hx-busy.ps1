<#
.SYNOPSIS
    HBuilderX「忙」检测 + 等待上限 + agent 互斥锁（供全部需要 HBuilderX 的门脚本 dot-source）。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md（「HBuilderX 单实例 / 串行」坑位段）。

    **为什么需要它**：CLI（`cli.exe`）只是**驱动同一个 HBuilderX 主程序**的客户端，`publish` / `launch` /
    `pack` 等重活都要排到主程序的编译队列里。维护者用 GUI 编译/运行时，agent 的门脚本若并发发起，
    既会**拖慢维护者**，也会因为排队而**产生假失败**（实测：publish 被排在另一个会话的
    `launch app-android --compile true` 之后，输出停在「正在编译中...」不返回）。
    ⇒ 本仓据此把「同一时刻只跑一个 CLI 步骤」定为**串行口径（「单实例串行」是本仓库的叫法、编排约定，
    不是 HBuilderX 的平台上限**：官方未记载实例数量与并发行为，只记载同一程序内多窗体/多项目）。
    本机制是 **锁 + 探测 + 上限** 的 **fail-safe**：宁可 `exit 2`（环境不可用）也**绝不抢占主程序**。

    **限制要写实（不要假装能探测 GUI）**：机械上**无法可靠探测「维护者的 GUI 是否正在编译」**。
    本机制的探测手段是「一次轻量 CLI 调用（`project list`）带硬超时」：
      - **能**区分「主程序可达且响应」与「连不上 / 无响应」两种状态（实测可达时 `project list` 约 0.5–2 秒返回）；
      - **不能**区分「主程序空闲」与「主程序正在为别人编译」——实测在主程序正跑另一个编译任务时，
        `project list` **仍然毫秒级返回**（它不进编译队列）。所以探测**只用于挡「无响应/未启动」**，
        真正的并发保护靠**跨进程文件锁**（`$env:TEMP\hx-agent.lock`）＋**等待上限**。
      若能接受更保守的默认，可加 `-NoWait`：探测到「不可响应」立即 `exit 2`，不做等待。

    **锁**：`$env:TEMP\hx-agent.lock`，内容为两行（pid / 起始时间）。**陈旧判定有两条（#974 起）**：

      ① **持有进程已不存在** ⇒ 立即视为陈旧、可抢占 —— **这正是 30 分钟线本来要防的那个场景**
         （上一个会话被强杀 / 崩溃后留下的死锁）。2026-09-14 实测：锁里 pid 已死、锁只放了 3 分钟，
         旧判据手里**有 pid 却只看时间** ⇒ 调用方只能反复 `exit 2`，白等到 30 分钟（当日实测白等 ~25 分钟）。
      ② 否则回落到**原有时间线**：持有超过 30 分钟视为失效可抢占。

    **两条的优先级不可颠倒（fail-safe）**：只有**确定**持有者已退出（`Get-Process` 明确报
    「Cannot find a process with the process identifier …」）才走 ①；**pid 非数字 / 拿不到 pid /
    查询报其它错（如权限不足）一律当作「未知」**，回落到 ② 的时间判据 ⇒ 新判据**只可能提前放行死锁，
    绝不放行任何活会话**。取锁后必须用 `Release-HxLock` 释放，且**要在 `finally` 路径里释放**。

    **锁交接（H10/H11，2026-09-14 加）**：本锁**按 PID 判定且不可重入** —— 若 `dev-finish.ps1` 自己持锁、
    再去调需要 HBuilderX 的子脚本（`kotlin-all-check.ps1` / `hx-run.ps1` / `mp-weixin-check.ps1`），
    子进程会看到**父进程的新鲜锁**而一直等到超时 ⇒ **自死锁**。故引入 `$env:HX_LOCK_OWNER` 交接：
      父进程：`Wait-HxFree` 拿到锁后调 `Set-HxLockOwnerEnv`（写 `$PID`）；
      子进程：`Wait-HxFree` 开头调 `Test-HxLockInherited`，认到「同一持有者」就**直接复用、不重复加锁**。
    **fail-safe（H11）**：只有 `$env:HX_LOCK_OWNER` 与**锁文件里的 PID 一致**才算交接成立 ——
    父进程崩溃后 env 残留或锁已被释放时 PID 对不上 ⇒ 照常加锁，**绝不放开互斥**。
    **子进程不得释放父进程的锁**：`Release-HxLock` 只在 `$script:HxLockOwned` 为真时动作，而该标志
    只在**本进程**真正 `Acquire` 成功时才置真 ⇒ 交接场景下 `Release-HxLock` 天然 no-op（无需特判）。

    **现场快照（2026-11-26 加，用户 M2 ①）**：`Get-HxBusyStatus` + `Format-HxBusyStatusLine` 给出一行只读机读行
    `HX_STATUS lock=<held|free> holder=<pid|none> age=<秒>s|none alive=<true|false|unknown|none> stale=<…> cli=<路径|none> hx=<running|absent>`
    —— **不取锁、不调 cli、不写文件**，因此可随时跑（`npm run hx:run -- -Status`）。`Wait-HxFree` 也在**进入等待的第 0 秒**
    打这一行并给出三条出路（等 / `-NoWait` 立刻退 / 改做不需要 HBuilderX 的事），不再让人盯「已等 N 秒」到超时才知道被谁挡。
    职责分工：`HX_STATUS` 是**现场**，`HX_BUSY wait=… result=…` 是**结果**；两者都不参与任何门的判据。

    **失败语义**：等待超时 ⇒ 打印明确提示并 `exit 2`（环境不可用），提示里给出不依赖 HBuilderX 的替代门。
    **绝不 kill 主程序进程**、**绝不抢占项目**；本文件内**不得**出现任何按名字强杀 HBuilderX 主进程的调用（契约测试 H3 会拦截字面量）。

.EXAMPLE
    . "$PSScriptRoot/lib/hx-busy.ps1"
    $hx = Wait-HxFree -TimeoutSeconds 600          # 返回 @{ Waited = <秒>; Result = 'free' }
    try { ...HBuilderX 步骤... } finally { Release-HxLock }
#>

# 锁目录：优先 `$env:TEMP`（Windows 常规路径，文档与 ADR-0008 都这么写）；**为空时回落到 .NET 的
# GetTempPath()**。Linux（本仓单测跑在 CI 的 ubuntu-latest 上）与被裁剪的环境里 `$env:TEMP` 不存在，
# 此时 `Join-Path $env:TEMP …` 会抛「Cannot bind argument to parameter 'Path' because it is null」
# —— 2026-09-14 CI 实测：dot-source 本模块即失败，整套契约测试 fail-closed 转红。
$script:HxLockDir = if ($env:TEMP) { $env:TEMP } else { [System.IO.Path]::GetTempPath() }
$script:HxLockPath = Join-Path $script:HxLockDir 'hx-agent.lock'
$script:HxLockStaleMinutes = 30
$script:HxLockOwned = $false
$script:HxBusyWaitSeconds = 0

function Get-HxLockInfo {
    if (-not (Test-Path -LiteralPath $script:HxLockPath)) { return $null }
    try {
        $lines = @(Get-Content -LiteralPath $script:HxLockPath -ErrorAction Stop)
        if ($lines.Count -lt 2) { return $null }
        return @{ Pid = "$($lines[0])".Trim(); Started = "$($lines[1])".Trim(); MTime = (Get-Item -LiteralPath $script:HxLockPath).LastWriteTime }
    } catch { return $null }
}

function Test-HxProcessAlive {
    <#
      锁持有者存活探测（#974 现象一）。**刻意返回三态而不是布尔** ——
      因为「拿不准」必须与「确定已死」分开，否则新判据会放行活会话：
        $true  = 进程存在（活会话）
        $false = **确定**不存在（`Get-Process` 明确报「Cannot find a process …」）
        $null  = **未知**（pid 非数字 / 越界 / 查询报其它错，如权限不足）
      调用方只允许在 $false 时走「持有者已死 ⇒ 陈旧」这条快路；$null 必须回落时间判据（fail-safe）。
      用 .NET 的 `GetProcessById` 而**不是** `Get-Process` 这条 cmdlet：「进程不存在」在前者是
      **跨平台稳定**的 `ArgumentException`（.NET 文档 + Windows 实测，2026-09-14），
      后者抛的 `ProcessCommandException` 是实现细节 —— 本仓单测跑在 ubuntu-latest 上，必须跨平台可靠。
    #>
    param([string]$PidText)
    $t = "$PidText".Trim()
    if ($t -notmatch '^\d+$') { return $null }
    try {
        $p = [System.Diagnostics.Process]::GetProcessById([int]$t)
        if ($null -ne $p) { return $true }
        return $null
    } catch [System.ArgumentException] {
        return $false
    } catch {
        return $null
    }
}

function Test-HxLockStale {
    <#
      陈旧判定（#974 起两条，优先级见文件头）：
        ① 持有进程**已确定不存在** ⇒ 陈旧（被强杀/崩溃的会话留下的死锁，不该再等满 30 分钟）；
        ② 否则（拿不准，或持有者仍活）回落到时间线：持有超过 HxLockStaleMinutes 分钟。
      **顺序不可颠倒**：① 只在 Test-HxProcessAlive 明确返回 $false 时成立。
    #>
    param($Info)
    if (-not $Info) { return $true }
    if ((Test-HxProcessAlive -PidText $Info.Pid) -eq $false) { return $true }
    try { return ((Get-Date) - $Info.MTime).TotalMinutes -gt $script:HxLockStaleMinutes } catch { return $true }
}

function Get-HxBusyStatus {
    <#
      只读占用快照（用户 M2 ①：**默认要能快速识别「HBuilderX 是否被其他会话占用」**）。
      **不取锁、不调 cli、不写任何文件、不建目录** —— 因此可以在任何时刻跑，也不改变任何一个门的结果。
      「谁占着」的唯一可靠来源是仓内锁文件（锁里存 pid + 起始时间）；HBuilderX 进程是否存在只是旁证
      （它不代表有人在驱动它，也不代表本仓锁被占 —— 维护者用 GUI 时就是「进程在、锁不在」）。
      Alive 沿用 Test-HxProcessAlive 的**三态**：拿不准一律 unknown，绝不把「未知」写成「已死」。
    #>
    param([string]$CliExe)
    $info = Get-HxLockInfo
    $age = -1
    $alive = $null
    $stale = $null
    if ($info) {
        try { $age = [int]((Get-Date) - $info.MTime).TotalSeconds } catch { $age = -1 }
        $alive = Test-HxProcessAlive -PidText $info.Pid
        $stale = [bool](Test-HxLockStale -Info $info)
    }
    $hxProc = $null
    try { $hxProc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1 } catch { $hxProc = $null }
    $hxPid = 0
    $hxPath = $null
    if ($hxProc) {
        $hxPid = [int]$hxProc.Id
        try { $hxPath = "$($hxProc.Path)" } catch { $hxPath = $null }
    }
    $holder = $null
    $started = $null
    if ($info) { $holder = "$($info.Pid)"; $started = "$($info.Started)" }
    return @{
        LockPresent = [bool]$info
        Holder      = $holder
        Started     = $started
        AgeSeconds  = $age
        Alive       = $alive
        Stale       = $stale
        CliExe      = $CliExe
        HxRunning   = [bool]$hxProc
        HxPid       = $hxPid
        HxPath      = $hxPath
    }
}

function Format-HxTriState {
    # 三态渲染：$true / $false / $null(未知) ⇒ true / false / unknown。未知绝不渲染成 false。
    param($Value)
    if ($Value -eq $true) { return 'true' }
    if ($Value -eq $false) { return 'false' }
    return 'unknown'
}

function Format-HxBusyStatusLine {
    <#
      机读行（一行、稳定形状，便于人眼与脚本同时消费）：
        HX_STATUS lock=<held|free> holder=<pid|none> age=<秒>s|none alive=<true|false|unknown|none>
                  stale=<true|false|none> cli=<路径|none> hx=<running|absent>
      与 HX_BUSY 的分工：HX_BUSY 是**结果**（wait=… result=free|timeout|inherited），
      HX_STATUS 是**现场**（谁占着、占多久、是否陈旧、cli 解析到哪）—— 两者都不含 exit 语义。
    #>
    param($Status)
    $lock = 'free'; $holder = 'none'; $age = 'none'; $alive = 'none'; $stale = 'none'
    if ($Status.LockPresent) {
        $lock = 'held'
        $holder = "$($Status.Holder)"
        if ($Status.AgeSeconds -ge 0) { $age = "$($Status.AgeSeconds)s" }
        $alive = Format-HxTriState $Status.Alive
        $stale = Format-HxTriState $Status.Stale
    }
    $cli = 'none'
    if ($Status.CliExe) { $cli = "$($Status.CliExe)" }
    $hx = if ($Status.HxRunning) { 'running' } else { 'absent' }
    return "HX_STATUS lock=$lock holder=$holder age=$age alive=$alive stale=$stale cli=$cli hx=$hx"
}

function Write-HxBusyStatusLine {
    param($Status, [string]$LogPath)
    $line = Format-HxBusyStatusLine -Status $Status
    Write-Host $line
    if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value $line -ErrorAction SilentlyContinue }
    return $line
}

function Write-HxBusyChooseHint {
    <#
      占用时**当场**给出的可诊断说明 + 三条出路（用户 M2 ②：明确、可诊断，避免无意义长等）。
      口径必须写实：串行是**本仓的编排选择**，不是 HBuilderX 的平台上限（官方未记载并发行为）。
    #>
    param($Status, [int]$TimeoutSeconds)
    $who = "持有者 pid=$($Status.Holder)（起始 $($Status.Started)，已持有 $($Status.AgeSeconds) 秒"
    if ($Status.Alive -eq $true) { $who += '，进程仍在运行' }
    elseif ($Status.Alive -eq $false) { $who += '，进程已不存在 ⇒ 属陈旧锁，下一次取锁会自动抢占' }
    else { $who += '，进程是否存在**未知**（权限/pid 不可解析）⇒ 按 fail-safe 回落 30 分钟时间线' }
    $who += '）'
    Write-Host ">>> [hx-busy] 锁被占用：$who" -ForegroundColor Yellow
    Write-Host ">>> [hx-busy] 这不是 HBuilderX 的平台上限，而是本仓的单实例串行编排（并发行为未实测）；别据此判断「HBuilderX 不能并行」。" -ForegroundColor Yellow
    Write-Host ">>> [hx-busy] 现在可以选：① 等（-WaitSeconds <秒>，本次上限 $TimeoutSeconds 秒）② 立刻退（-NoWait，exit 2 环境不可用）③ 先做不需要 HBuilderX 的事（npm run test:unit / ④c -SkipPublish / npm run audit:*）" -ForegroundColor Yellow
}

function Acquire-HxLock {
    # 返回 $true 表示拿到锁（含抢占陈旧锁）；$false 表示锁被别人持有且未陈旧
    for ($i = 0; $i -lt 2; $i++) {
        $info = Get-HxLockInfo
        if ($info -and -not (Test-HxLockStale -Info $info)) { return $false }
        if ($info) {
            # 把**为什么**判陈旧写进日志：#974 的两条判据对应两种完全不同的处置，日志里分不清就没法复盘
            $why = if ((Test-HxProcessAlive -PidText $info.Pid) -eq $false) {
                "持有进程 pid=$($info.Pid) 已不存在（#974：不必再等满 $script:HxLockStaleMinutes 分钟）"
            } else {
                "已超 $script:HxLockStaleMinutes 分钟"
            }
            Write-Host ">>> [hx-busy] 抢占陈旧锁（pid=$($info.Pid)，起始 $($info.Started)：$why）" -ForegroundColor Yellow
            Remove-Item -LiteralPath $script:HxLockPath -Force -ErrorAction SilentlyContinue
        }
        try {
            Set-Content -LiteralPath $script:HxLockPath -Encoding utf8 -Value @("$PID", (Get-Date -Format 'yyyy-MM-dd HH:mm:ss')) -ErrorAction Stop
            $script:HxLockOwned = $true
            return $true
        } catch { return $false }
    }
    return $false
}

function Release-HxLock {
    if (-not $script:HxLockOwned) { return }
    $script:HxLockOwned = $false
    try {
        $info = Get-HxLockInfo
        # 只删自己持有的锁，避免误删别人刚抢到的
        if ($info -and "$($info.Pid)" -eq "$PID") { Remove-Item -LiteralPath $script:HxLockPath -Force -ErrorAction SilentlyContinue }
    } catch { }
}

function Test-HxLockInherited {
    <#
      锁交接（H10）：本会话的**父进程**已持锁时返回 $true（详见文件头「锁交接」段）。
      **fail-safe（H11）**：只有 $env:HX_LOCK_OWNER 与锁文件里的 PID **一致**才算成立 ——
      父进程崩溃 / 锁已被抢占 / env 被子进程继承到无关场景时 PID 对不上 ⇒ 返回 $false，调用方照常加锁。
    #>
    if (-not $env:HX_LOCK_OWNER) { return $false }
    $info = Get-HxLockInfo
    if (-not $info) { return $false }
    return ("$($info.Pid)" -eq "$($env:HX_LOCK_OWNER)")
}

function Set-HxLockOwnerEnv {
    <# 父进程：自己持锁后调用，把 PID 写进 env 供**本会话派生**的子脚本认（子进程会继承 env）。 #>
    $env:HX_LOCK_OWNER = "$PID"
}

function Clear-HxLockOwnerEnv {
    <# 父进程：释放锁时一并清掉，避免后代进程继承到陈旧交接（fail-safe 的第二道）。 #>
    if (Test-Path Env:HX_LOCK_OWNER) { Remove-Item Env:HX_LOCK_OWNER -ErrorAction SilentlyContinue }
}

function Test-HxResponsive {
    param([string]$CliExe, [int]$ProbeTimeoutSeconds = 5)
    # 返回 @{ Responsive = <bool>; Output = <string>; ElapsedMs = <int> }
    if (-not $CliExe -or -not (Test-Path -LiteralPath $CliExe -PathType Leaf)) {
        return @{ Responsive = $false; Output = 'cli.exe 缺失'; ElapsedMs = 0 }
    }
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $CliExe
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    foreach ($a in @('project', 'list')) { [void]$psi.ArgumentList.Add($a) }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try { $p = [System.Diagnostics.Process]::Start($psi) } catch {
        return @{ Responsive = $false; Output = "启动 cli 失败：$_"; ElapsedMs = 0 }
    }
    $so = $p.StandardOutput.ReadToEndAsync(); $se = $p.StandardError.ReadToEndAsync()
    $exited = $p.WaitForExit($ProbeTimeoutSeconds * 1000)
    $sw.Stop()
    if (-not $exited) {
        # 只终止**这一次探测进程**，绝不碰主程序
        try { $p.Kill($true) } catch { }
        return @{ Responsive = $false; Output = "探测未在 $ProbeTimeoutSeconds 秒内返回（判为「主程序忙/无响应」）"; ElapsedMs = [int]$sw.Elapsed.TotalMilliseconds }
    }
    return @{ Responsive = $true; Output = ($so.Result + $se.Result); ElapsedMs = [int]$sw.Elapsed.TotalMilliseconds }
}

function Get-HxRegValue {
    <#
      安全读注册表条目属性（2026-11-26 加，真跑踩出来的）：
      本仓脚本统一 `Set-StrictMode -Version Latest`，此时读一个**不存在的属性**会直接抛
      「在此对象上找不到属性"DisplayName"」——卸载项里有的条目确实没有 DisplayName / InstallLocation
      （非安装器写入的键、或权限受限读不全），于是整条 CLI 解析链断掉。
      ⇒ 一律走 PSObject.Properties 显式判存在，缺了就返回空串，绝不抛错。
    #>
    param($Item, [string]$Name)
    if (-not $Item) { return '' }
    $prop = $Item.PSObject.Properties[$Name]
    if (-not $prop) { return '' }
    if ($null -eq $prop.Value) { return '' }
    return [string]$prop.Value
}

function Get-HxCliCandidatesFromRegistry {
    <#
      安装信息的**第一手来源**（用户 M2 ④：CLI 解析不能只靠「当前运行中的 HBuilderX 进程反查」）。
      从三项卸载注册表取 DisplayName 含 `HBuilderX` 的条目，按下述顺序产出候选 `cli.exe` 路径：
        InstallLocation / InstallDir → `<loc>\cli.exe`、`<loc>\HBuilderX\cli.exe`
        DisplayIcon / UninstallString 指向的 exe 目录 → `<dir>\cli.exe`
      **只产出候选，不做判断**（存在性 / PE 头由调用方查）—— 注册表里有路径 ≠ 装好了，这条在本仓有先例。
      读不到 / 非 Windows ⇒ 返回空数组、绝不抛错（ubuntu CI 与本机受限会话都要能跑）。
    #>
    $out = New-Object System.Collections.ArrayList
    foreach ($root in @(
            'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
            'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
            'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*')) {
        $items = $null
        try { $items = Get-ItemProperty -Path $root -ErrorAction Stop } catch { continue }
        if (-not $items) { continue }
        foreach ($it in @($items)) {
            $displayName = Get-HxRegValue -Item $it -Name 'DisplayName'
            if ($displayName -notmatch 'HBuilderX') { continue }
            foreach ($loc in @((Get-HxRegValue -Item $it -Name 'InstallLocation'), (Get-HxRegValue -Item $it -Name 'InstallDir'))) {
                if (-not $loc) { continue }
                [void]$out.Add((Join-Path $loc 'cli.exe'))
                [void]$out.Add((Join-Path (Join-Path $loc 'HBuilderX') 'cli.exe'))
            }
            foreach ($raw in @((Get-HxRegValue -Item $it -Name 'DisplayIcon'), (Get-HxRegValue -Item $it -Name 'UninstallString'))) {
                if (-not $raw) { continue }
                $exe = ($raw -split ',')[0].Trim().Trim('"')
                if ($exe -notmatch '\.exe$') { continue }
                try { $dir = Split-Path -Path $exe -Parent } catch { continue }
                if (-not $dir) { continue }
                [void]$out.Add((Join-Path $dir 'cli.exe'))
            }
        }
    }
    # `,` 不能省：PowerShell 把空数组展开成「无输出」，调用方拿到的会是 $null（`Set-StrictMode` 下
    # 随后 `.Count` / `foreach` 就踩空）。函数契约是「读不到 ⇒ 空数组」，这里用 `,` 把数组本身原样返回。
    return ,@($out | Select-Object -Unique)
}

function Resolve-HxCliExe {
    <#
      找一个**能跑**的 HBuilderX `cli.exe`，供 `Wait-HxFree` 的一次忙探测用（#2112）。
      顺序与各门脚本既有的解析一致：显式 → `$env:HBuilderX_CLI` → 正在运行的 HBuilderX 同目录
      （版本必然匹配）→ 常见安装位置（新→旧）。
      **只判「存在且像可执行」，不做 PE 头校验**：那一步留给真正会用它跑 publish 的门（② / ④c
      各自解析），这里的目标只有一个 —— 给忙探测一个能立刻返回的目标。
      找不到返回 `$null`，**不猜、不回退到随便一个 exe**：猜错会把「没装 HBuilderX」误判成
      「主程序忙」，调用方就该在探测超时与「没装」之间来回折腾（这类误诊在本仓有前例）。
    #>
    param([string]$Explicit, [string]$HxDir)

    # 1) 显式即权威：给出就只认它（不可用 ⇒ 调用方判环境不可用，不回退）
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit -PathType Leaf) { return (Resolve-Path -LiteralPath $Explicit).Path }
        return $null
    }
    # 2) 安装根
    if ($HxDir) {
        foreach ($rel in @('cli.exe', 'HBuilderX\cli.exe')) {
            $c = Join-Path $HxDir $rel
            if (Test-Path -LiteralPath $c -PathType Leaf) { return (Resolve-Path -LiteralPath $c).Path }
        }
    }
    # 3) 环境变量
    if ($env:HBuilderX_CLI -and (Test-Path -LiteralPath $env:HBuilderX_CLI -PathType Leaf)) {
        return (Resolve-Path -LiteralPath $env:HBuilderX_CLI).Path
    }
    # 4) 正在运行的 HBuilderX 同目录
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($proc) {
            $runningCli = Join-Path (Split-Path $proc.Path -Parent) 'cli.exe'
            if (Test-Path -LiteralPath $runningCli -PathType Leaf) { return (Resolve-Path -LiteralPath $runningCli).Path }
        }
    }
    catch {
        # 进程路径读不到（权限）就继续走位置探测
    }
    # 5) 注册表卸载项（不依赖「HBuilderX 此刻是否在跑」，用户 M2 ④）
    foreach ($cand in (Get-HxCliCandidatesFromRegistry)) {
        if (Test-Path -LiteralPath $cand -PathType Leaf) { return (Resolve-Path -LiteralPath $cand).Path }
    }
    # 6) 常见安装位置（env 为空就跳过，别拼出相对路径去扫当前目录）
    $patterns = New-Object System.Collections.ArrayList
    [void]$patterns.Add('D:\软件\HBuilderX*\HBuilderX\cli.exe')
    [void]$patterns.Add('D:\HBuilderX*\HBuilderX\cli.exe')
    foreach ($root in @($env:ProgramFiles, ${env:ProgramFiles(x86)}, 'C:\Program Files', 'D:\Program Files', "$env:LOCALAPPDATA\Programs")) {
        if (-not $root) { continue }
        [void]$patterns.Add((Join-Path $root 'HBuilderX*\HBuilderX\cli.exe'))
        [void]$patterns.Add((Join-Path $root 'HBuilderX\cli.exe'))
    }
    if ($env:LOCALAPPDATA) { [void]$patterns.Add((Join-Path $env:LOCALAPPDATA 'HBuilderX\cli.exe')) }
    foreach ($pattern in $patterns) {
        $hit = Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | Select-Object -First 1
        if ($hit) { return $hit.FullName }
    }
    return $null
}

function Wait-HxFree {
    <#
      取 agent 互斥锁 + 探测主程序可响应性，直到可用或超时。
      返回 @{ Waited = <实际等待秒>; Result = 'free' }；超时/忙则直接 exit 2（环境不可用）。
      -NoWait：不做等待，锁被占或探测到不可响应立即 exit 2。
      **锁由调用方在自己的 finally 里通过 Release-HxLock 释放**（HBuilderX 步骤在本函数返回之后才跑）。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$CliExe,
        [int]$TimeoutSeconds = 600,
        [int]$PollSeconds = 10,
        [int]$ProbeTimeoutSeconds = 5,
        [switch]$NoWait,
        [string]$LogPath
    )
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $lockWaited = 0

    # 锁交接（H10/H11）：父会话已持锁 ⇒ 直接复用，**不重复加锁、不等待**（否则父自己等自己的锁 ⇒ 自死锁）
    if (Test-HxLockInherited) {
        $script:HxBusyWaitSeconds = 0
        $line = 'HX_BUSY wait=0 result=inherited'
        Write-Host $line
        if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value $line -ErrorAction SilentlyContinue }
        return @{ Waited = 0; Result = 'inherited' }
    }

    # 现场快照（用户 M2 ①）：**进等待的第 0 秒**就把「谁占着、占多久、cli 解析到哪」打出来，
    # 而不是让人盯着每 10 秒一行的「已等 N 秒」直到超时才知道发生了什么。
    $entry = Get-HxBusyStatus -CliExe $CliExe
    Write-HxBusyStatusLine -Status $entry -LogPath $LogPath | Out-Null
    if ($entry.LockPresent) { Write-HxBusyChooseHint -Status $entry -TimeoutSeconds $TimeoutSeconds }

    while (-not (Acquire-HxLock)) {
        if ($NoWait -or $lockWaited -ge $TimeoutSeconds) {
            Write-HxBusyHint -Reason "锁被另一个 agent 会话持有：$script:HxLockPath" -Waited $lockWaited -LogPath $LogPath -Status (Get-HxBusyStatus -CliExe $CliExe)
            exit 2
        }
        # 每轮带上持有者 pid 与起始时间：不同会话在同一个终端里等待时，只看「已等 N 秒」分不清是谁在挡。
        $who = ''
        $cur = Get-HxLockInfo
        if ($cur) { $who = "，持有者 pid=$($cur.Pid)（起始 $($cur.Started)）" }
        Write-Host ">>> [hx-busy] 等待另一个 agent 会话释放 HBuilderX 锁$who（已等 $lockWaited 秒，上限 $TimeoutSeconds 秒）" -ForegroundColor Yellow
        # 只睡到「余量」为止：否则 -WaitSeconds 6 会睡满一个 10 秒轮询，把「上限 6 秒」跑成 10 秒 —— 预算必须诚实。
        $sleepSec = [Math]::Min($PollSeconds, [Math]::Max(1, $TimeoutSeconds - $lockWaited))
        Start-Sleep -Seconds $sleepSec
        $lockWaited += $sleepSec
    }
    while ($true) {
        $probe = Test-HxResponsive -CliExe $CliExe -ProbeTimeoutSeconds $ProbeTimeoutSeconds
        if ($probe.Responsive) {
            if ($probe.Output -match '与主程序的连接已中断') {
                Write-Host '[warn] cli 报「与主程序的连接已中断」：本脚本的 HBuilderX 段落会自己 `cli.exe open` 唤醒主程序。' -ForegroundColor Yellow
            }
            $waited = [int]$sw.Elapsed.TotalSeconds
            $script:HxBusyWaitSeconds = $waited
            $line = "HX_BUSY wait=$waited result=free"
            Write-Host $line
            if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value $line -ErrorAction SilentlyContinue }
            return @{ Waited = $waited; Result = 'free' }
        }
        $waited = [int]$sw.Elapsed.TotalSeconds
        if ($NoWait -or $waited -ge $TimeoutSeconds) {
            Write-HxBusyHint -Reason 'cli.exe 探测未在超时内返回（主程序无响应，可能正忙）' -Waited $waited -LogPath $LogPath -Status (Get-HxBusyStatus -CliExe $CliExe)
            exit 2
        }
        $sleepSec = [Math]::Min($PollSeconds, [Math]::Max(1, $TimeoutSeconds - $waited))
        Write-Host ">>> [hx-busy] 主程序暂不可响应，$sleepSec 秒后重试（已等 $waited 秒，上限 $TimeoutSeconds 秒）" -ForegroundColor Yellow
        Start-Sleep -Seconds $sleepSec
    }
}

function Write-HxBusyHint {
    param([string]$Reason, [int]$Waited, [string]$LogPath, $Status)
    Write-Host "[error] HBuilderX 门暂不可执行：$Reason" -ForegroundColor Red
    # 超时这一行必须能自解释（用户 M2 ②）：谁占着、占多久、是陈旧锁还是活会话 —— 否则「等 120 秒再退」读起来像偶发。
    if ($Status -and $Status.LockPresent) {
        Write-HxBusyStatusLine -Status $Status -LogPath $LogPath | Out-Null
        Write-Host "        持有者 pid=$($Status.Holder)（起始 $($Status.Started)，已持有 $($Status.AgeSeconds) 秒，存活=$(Format-HxTriState $Status.Alive)，陈旧=$(Format-HxTriState $Status.Stale)）" -ForegroundColor Red
        Write-Host '        若要等更久：给入口传更大的 -WaitSeconds（hx:run 默认 120 秒）；要立刻退：-NoWait。' -ForegroundColor Red
    }
    Write-Host '        维护者可能正在用 HBuilderX（**GUI 优先**）；可稍后重试，或改跑不需要 HBuilderX 的门：' -ForegroundColor Red
    Write-Host '        仿真机冒烟（纯 adb/emulator）/ ④c `-SkipPublish` / `npm run test:unit`。' -ForegroundColor Red
    Write-Host '        本脚本**绝不抢占主程序、绝不 kill 主程序**（HBuilderX 是单实例串行资源，本仓编排选择）。' -ForegroundColor Red
    $script:HxBusyWaitSeconds = $Waited
    if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value "HX_BUSY wait=$Waited result=timeout" -ErrorAction SilentlyContinue }
    Write-Host "HX_BUSY wait=$Waited result=timeout"
}

