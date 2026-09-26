<#
.SYNOPSIS
    进程采集层（`Invoke-Process`）—— ④c 门与守护共用的子进程执行器。

.DESCRIPTION
    为什么单独成库（#1285）：采集层必须能被**真执行**地测到 ——
    `utils/kotlinAllProcessCaptureBehavior.test.js` dot-source 本文件后直接驱动它
    （先例 `scripts/lib/publish-freshness.ps1` + `utils/kotlinAllStaleExportBehavior.test.js`）。
    留在门脚本里就只能读源码文本断言，而接线守护**不构成 ③ 证据**（`docs/agents/guards.md`）。
    本文件零副作用（只有函数定义），dot-source 它不会跑门。
    两条契约（#1285 的 期望 a）：① **总时长受 TimeoutSeconds 约束、含读段** —— `WaitForExit($ms)` 之后，
    两个读任务还必须在剩余预算内排空（退出 ≠ 读全）；② **排不空 ⇒ TimedOut=true** ⇒ 门照旧走
    `reason=timeout`（fail-closed：绝不拿没排空的流去喂正向标记判据，超时语义一分不松）。
#>
function Invoke-Process {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [int]$TimeoutSeconds,
        [string]$Tag
    )
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8
    foreach ($a in $Arguments) { [void]$psi.ArgumentList.Add($a) }

    Write-Host ">>> [$Tag] $FilePath"
    $p = [System.Diagnostics.Process]::Start($psi)
    $stdout = $p.StandardOutput.ReadToEndAsync()
    $stderr = $p.StandardError.ReadToEndAsync()
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $exited = $p.WaitForExit($TimeoutSeconds * 1000)
    if (-not $exited) {
        try { $p.Kill($true) } catch { }
        return @{ Output = "[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true }
    }
    # #1285：进程已退出 ≠ 输出已读全 —— 读段也必须**有界**。`WaitForExit(int)` 不等异步读任务排空
    # （.NET 文档明示；本机探针实测：子脚本秒退、后代握 stdout 时，旧实现总耗时 41137 ms / 预算 4000 ms
    #  且 TimedOut=False ⇒ 超时语义对读段完全失效）。故在剩余预算内 `Task.WaitAll` 排空两个读任务：
    # 排不空 ⇒ TimedOut=true ⇒ 门照旧走 `reason=timeout`（fail-closed：绝不拿没排空的流去喂正向标记判据）。
    # 进程既已退出，正常 EOF 是毫秒级 ⇒ 给 1 秒下限，避免预算刚好耗尽时把「已排空」误判成超时。
    $remainingMs = $TimeoutSeconds * 1000 - [int]$sw.ElapsedMilliseconds
    if ($remainingMs -lt 1000) { $remainingMs = 1000 }
    $drained = [System.Threading.Tasks.Task]::WaitAll([System.Threading.Tasks.Task[]]@($stdout, $stderr), $remainingMs)
    if (-not $drained) {
        # 不取 `.Result` —— 那正是旧的无界阻塞点；也不 catch 聚合异常：读任务 faulted 时这里照旧抛
        # （与旧行为一致的 fail-closed：脚本退出，而不是拿残缺输出继续判）。
        return @{ Output = "[timeout] $Tag 输出未在预算内排空（进程已退出但 stdout/stderr 未读完）"; ExitCode = -1; TimedOut = $true }
    }
    $out = $stdout.Result + $stderr.Result
    return @{ Output = $out; ExitCode = $p.ExitCode; TimedOut = $false }
}
