<#
.SYNOPSIS
    HBuilderX「忙」检测 + 等待上限 + agent 互斥锁（供全部需要 HBuilderX 的门脚本 dot-source）。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md（「HBuilderX 单实例 / 串行」坑位段）。

    **为什么需要它**：HBuilderX 是**单实例串行资源**——CLI（`cli.exe`）只是**驱动同一个主程序**的客户端，
    `publish` / `launch` / `pack` 等重活都要排到主程序的编译队列里。维护者用 GUI 编译/运行时，
    agent 的门脚本若并发发起，既会**拖慢维护者**，也会因为排队而**产生假失败**（实测：publish 被排在
    另一个会话的 `launch app-android --compile true` 之后，输出停在「正在编译中...」不返回）。
    ⇒ 本机制是 **锁 + 探测 + 上限** 的 **fail-safe**：宁可 `exit 2`（环境不可用）也**绝不抢占主程序**。

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

    **失败语义**：等待超时 ⇒ 打印明确提示并 `exit 2`（环境不可用），提示里给出不依赖 HBuilderX 的替代门。
    **绝不 kill 主程序进程**、**绝不抢占项目**；本文件内**不得**出现任何按名字强杀 HBuilderX 主进程的调用（契约测试 H3 会拦截字面量）。

.EXAMPLE
    . "$PSScriptRoot/lib/hx-busy.ps1"
    $hx = Wait-HxFree -TimeoutSeconds 600          # 返回 @{ Waited = <秒>; Result = 'free' }
    try { ...HBuilderX 步骤... } finally { Release-HxLock }
#>

$script:HxLockPath = Join-Path $env:TEMP 'hx-agent.lock'
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
      实测（PowerShell 7.6.5，2026-09-14）：pid 不存在时 `Get-Process -Id … -ErrorAction Stop`
      抛的正是 `Microsoft.PowerShell.Commands.ProcessCommandException`。
    #>
    param([string]$PidText)
    $t = "$PidText".Trim()
    if ($t -notmatch '^\d+$') { return $null }
    try {
        $p = Get-Process -Id ([int]$t) -ErrorAction Stop
        if ($null -ne $p) { return $true }
        return $null
    } catch [Microsoft.PowerShell.Commands.ProcessCommandException] {
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

    while (-not (Acquire-HxLock)) {
        if ($NoWait -or $lockWaited -ge $TimeoutSeconds) {
            Write-HxBusyHint -Reason "锁被另一个 agent 会话持有：$script:HxLockPath" -Waited $lockWaited -LogPath $LogPath
            exit 2
        }
        Write-Host ">>> [hx-busy] 等待另一个 agent 会话释放 HBuilderX 锁（已等 $lockWaited 秒，上限 $TimeoutSeconds 秒）" -ForegroundColor Yellow
        Start-Sleep -Seconds $PollSeconds
        $lockWaited += $PollSeconds
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
            Write-HxBusyHint -Reason 'cli.exe 探测未在超时内返回（主程序无响应，可能正忙）' -Waited $waited -LogPath $LogPath
            exit 2
        }
        Write-Host ">>> [hx-busy] 主程序暂不可响应，$PollSeconds 秒后重试（已等 $waited 秒，上限 $TimeoutSeconds 秒）" -ForegroundColor Yellow
        Start-Sleep -Seconds $PollSeconds
    }
}

function Write-HxBusyHint {
    param([string]$Reason, [int]$Waited, [string]$LogPath)
    Write-Host "[error] HBuilderX 门暂不可执行：$Reason" -ForegroundColor Red
    Write-Host '        维护者可能正在用 HBuilderX（**GUI 优先**）；可稍后重试，或改跑不需要 HBuilderX 的门：' -ForegroundColor Red
    Write-Host '        仿真机冒烟（纯 adb/emulator）/ ④c `-SkipPublish` / `npm run test:unit`。' -ForegroundColor Red
    Write-Host '        本脚本**绝不抢占主程序、绝不 kill 主程序**（HBuilderX 是单实例串行资源）。' -ForegroundColor Red
    $script:HxBusyWaitSeconds = $Waited
    if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value "HX_BUSY wait=$Waited result=timeout" -ErrorAction SilentlyContinue }
    Write-Host "HX_BUSY wait=$Waited result=timeout"
}

