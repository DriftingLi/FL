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

    **锁**：`$env:TEMP\hx-agent.lock`，内容为两行（pid / 起始时间）。**陈旧判定：持有超过 30 分钟视为失效可抢占**
    （防止上一个会话被强杀后留下死锁）。取锁后必须用 `Release-HxLock` 释放，且**要在 `finally` 路径里释放**。

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

function Test-HxLockStale {
    param($Info)
    if (-not $Info) { return $true }
    try { return ((Get-Date) - $Info.MTime).TotalMinutes -gt $script:HxLockStaleMinutes } catch { return $true }
}

function Acquire-HxLock {
    # 返回 $true 表示拿到锁（含抢占陈旧锁）；$false 表示锁被别人持有且未陈旧
    for ($i = 0; $i -lt 2; $i++) {
        $info = Get-HxLockInfo
        if ($info -and -not (Test-HxLockStale -Info $info)) { return $false }
        if ($info) {
            Write-Host ">>> [hx-busy] 抢占陈旧锁（持有者 pid=$($info.Pid)，起始 $($info.Started)，已超 $script:HxLockStaleMinutes 分钟）" -ForegroundColor Yellow
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

