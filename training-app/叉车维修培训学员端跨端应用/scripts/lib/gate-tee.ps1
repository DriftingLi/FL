<#
 .SYNOPSIS
    门输出转发器：子门的 stdout/stderr **实时**转发到控制台与日志，且**收流有界**（#2147）。

 .DESCRIPTION
    病根（#2147，2026-10-30 现场；来路 = #2131 的 F4 只修到内层）：
    `pr-gates.ps1` 曾写 `& $pwshExe @argv 2>&1 | Tee-Object -FilePath <log> -Append | ForEach-Object {...}`。
    子门进程正常退出后，**它拉起的微信开发者工具（常驻 GUI）仍持有这层管道的写句柄** ⇒ `Tee-Object`
    永远等不到 EOF ⇒ 整条 pr-gates 挂死（实测 1050 s，④c 根本没开始），现场读起来像「门还在跑」。
    `mp-weixin-check.ps1` 里那层 5 s 有界收流（#2131 F4）管的是它自己的 `ReadToEndAsync`，
    **管不到外面这一层** —— 两层 EOF 依赖是两处独立的洞。

    处置（与 #2131 F4 同一条判据，落在上一层）：自己起进程，**不经 PowerShell 管道**（那层的 EOF 由孙
    进程的句柄决定，我们控制不了），逐行 `ReadLineAsync` 转发；主进程退出后只给 DrainMs 的收尾窗口，
    收不到就明确打 [warn] 放弃剩余输出 —— 「进程已退出」这个事实不许被一条挂住的读取抹掉。

    转发**不改写、不重排、不加前缀**：两门结果行的「逐字节不变」靠「不碰」实现（见 pr-gates 头注释）。

 .PARAMETER FilePath
    子进程可执行文件（pr-gates 传自己的 pwsh）。

 .PARAMETER Arguments
    子进程参数。走 ArgumentList 逐项传，不拼命令行字符串（路径里的空格/中文不经转义层）。

 .PARAMETER LogPath
    日志文件；给了就**逐行**追加（与 #2131 起 Tee-Object -Append 的落盘形态一致）。

 .PARAMETER DrainMs
    主进程退出后允许输出流继续收尾的毫秒数。超时即放弃剩余输出并 DrainAbandoned=true。

 .PARAMETER WorkingDirectory
    子进程工作目录（缺省继承本进程）。

 .OUTPUTS
    [pscustomobject]：Lines（已转发的行，供调用方找结果行）/ ExitCode / DrainAbandoned。
#>
function Invoke-GateTee {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [AllowEmptyCollection()][string[]]$Arguments = @(),
        [string]$LogPath,
        [int]$DrainMs = 5000,
        [string]$WorkingDirectory
    )
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8
    if ($WorkingDirectory) { $psi.WorkingDirectory = $WorkingDirectory }
    foreach ($a in $Arguments) { [void]$psi.ArgumentList.Add($a) }

    $collected = [System.Collections.Generic.List[string]]::new()
    $p = [System.Diagnostics.Process]::Start($psi)
    # 两条流各持一个「下一行」任务：谁先完成就先转发谁，**不用管道**（EOF 归我们判）
    $outTask = $p.StandardOutput.ReadLineAsync()
    $errTask = $p.StandardError.ReadLineAsync()
    $drainSw = $null
    $abandoned = $false
    while ($true) {
        $pending = [System.Collections.Generic.List[System.Threading.Tasks.Task]]::new()
        if ($null -ne $outTask) { $pending.Add($outTask) }
        if ($null -ne $errTask) { $pending.Add($errTask) }
        if ($pending.Count -eq 0) { break }
        $idx = -1
        try { $idx = [System.Threading.Tasks.Task]::WaitAny([System.Threading.Tasks.Task[]]$pending.ToArray(), 250) } catch { $idx = -1 }
        if ($idx -ge 0) {
            $done = $pending[$idx]
            $line = $null
            try { $line = $done.Result } catch { $line = $null }
            if ([object]::ReferenceEquals($done, $outTask)) {
                $outTask = if ($null -ne $line) { $p.StandardOutput.ReadLineAsync() } else { $null }
            } elseif ([object]::ReferenceEquals($done, $errTask)) {
                $errTask = if ($null -ne $line) { $p.StandardError.ReadLineAsync() } else { $null }
            }
            if ($null -ne $line) {
                Write-Host "$line"
                $collected.Add("$line")
                if ($LogPath) { Add-Content -LiteralPath $LogPath -Encoding utf8 -Value "$line" }
            }
        }
        # 主进程退出 ⇒ 起一个有界收尾窗口；窗口用完就放弃剩余输出（#2147 的承重条款）
        if ($p.HasExited) {
            if ($null -eq $drainSw) { $drainSw = [System.Diagnostics.Stopwatch]::StartNew() }
            elseif ($drainSw.ElapsedMilliseconds -ge $DrainMs) { $abandoned = $true; break }
        } else {
            $drainSw = $null
        }
    }
    if ($abandoned) {
        Write-Host ("[warn] 子进程已退出（exit={0}）但输出流仍被其子进程（常驻 IDE 等）占用 ⇒ 放弃剩余输出，不影响退出码与门结论" -f $p.ExitCode) -ForegroundColor Yellow
    }
    return [pscustomobject]@{
        Lines = [string[]]$collected.ToArray()
        ExitCode = $p.ExitCode
        DrainAbandoned = $abandoned
    }
}
