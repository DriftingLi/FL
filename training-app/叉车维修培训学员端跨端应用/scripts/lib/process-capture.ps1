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

<#
.SYNOPSIS
    跨块**有状态**的 UTF-8 增量解码（#2139）：读一个还在增长的文件，块边界切在多字节序列中间也不产 U+FFFD。

.DESCRIPTION
    为什么住这里（与 `Invoke-Process` 同一条理由）：解码层必须能被**真执行**地测到 —— 留在门脚本里就只能
    读源码文本断言，而接线守护不构成 ③ 证据（`docs/agents/guards.md`）。
    行为守护：`utils/evidenceCaptureEncodingBehavior.test.js`（真件 + 旧写法变异腿，成对取证）。

    坏在哪：`[System.Text.Encoding]::UTF8.GetString($buf, 0, $read)` 是**无状态**一次性解码 —— 给一段
    被切断的多字节序列就当场替换成 U+FFFD，且不可逆（原文拼不回来）。2026-10-30 实测：③ 门行为腿的
    `test-behavior.log` 与 `2045-behavior.log` 各有 56 个 U+FFFD、位置成簇（2376/2378/2379/2380…）
    ⇒ 是块边界切断，不是票面原先写的「原始 GBK 字节落盘」，两者修法不同。

    为什么 `Decoder` 修得了：`System.Text.Decoder` 自己留存「没凑齐的尾字节」，下次调用接着用（不 flush
    就不丢）。所以解码器由**调用方持有**、**一条流一个** —— stdout 与 stderr 共用一个会把两条流的尾字节
    混在一起。`throwOnInvalidBytes = $false`：真·非法字节走替换字符，与判据的 B1/B2 语义一致 ——
    证据可读性由 `.github/scripts/check-evidence-encoding.mjs` 判红，不由采集层抛异常炸门。
#>
function Get-Utf8CompletePrefixLength {
    <#
      返回「最后一个 UTF-8 序列已凑齐」的最大前缀长度。只有末尾那一段可能是半截：
      往回最多看 4 个字节找引导字节，按引导字节定该序列应有的总长；没凑齐就把它退回给下一次。
      非 UTF-8 的乱字节不在这里管（照旧由解码器出替换字符 —— 坏就报坏，交判据判红，不在采集层藏）。
    #>
    param(
        # AllowEmptyCollection：PowerShell 对 Mandatory 参数**拒绝空数组**绑定（本会话实测报错），
        # 而「空缓冲 ⇒ 前缀 0」是这个函数该有的值 —— 不加这条它就不是全函数，调用方得自己挡空。
        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [byte[]]$Bytes,
        [switch]$Flush
    )
    $arr = @($Bytes)
    $len = $arr.Length
    if ($len -eq 0) { return 0 }
    if ($Flush) { return $len }
    for ($back = 1; $back -le 4; $back += 1) {
        $idx = $len - $back
        if ($idx -lt 0) { return 0 }
        $b = [int]$arr[$idx]
        if (($b -band 0xC0) -eq 0x80) { continue }   # 延续字节：再往左找引导字节
        $need = if ($b -lt 0x80) { 1 } elseif ($b -lt 0xE0) { 2 } elseif ($b -lt 0xF0) { 3 } else { 4 }
        if ($back -lt $need) { return $idx }          # 没凑齐 ⇒ 这段留给下一次轮询
        break
    }
    return $len
}

function Read-Utf8Chunk {
    <#
      从 $Offset 读到文件当前末尾，只解码**凑齐了的前缀**；没凑齐的尾字节留在文件里（不推进偏移），
      下一次轮询自然连着新到的字节一起重读 ⇒ 跨块不产生 U+FFFD，且不需要任何跨块状态。
      返回 Text（本次新增文本，可为空）与 BytesRead（可安全推进的字节数，半截尾巴不算在内）。
      文件不可读 / 还没长出来 ⇒ Text=''、BytesRead=0（不把「读不到」伪装成「读到空」）。
      -Flush：文件末尾真截断时用，把剩下的字节全解码 —— 宁可留一个替换字符也不**丢字**
      （丢字比 U+FFFD 更坏：读不出来的行至少还看得出是读不出来）。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][long]$Offset,
        [switch]$Flush
    )
    $empty = [pscustomobject]@{ Text = ''; BytesRead = 0 }
    if (-not (Test-Path -LiteralPath $Path)) { return $empty }
    try {
        # 共享读 + 允许写端删除：子进程可能还在写、也可能被 kill 后清临时文件（旧实现同形）。
        $fs = [System.IO.File]::Open(
            $Path,
            [System.IO.FileMode]::Open,
            [System.IO.FileAccess]::Read,
            ([System.IO.FileShare]::ReadWrite -bor [System.IO.FileShare]::Delete))
    } catch { return $empty }
    try {
        if ($fs.Length -le $Offset) { return $empty }
        [void]$fs.Seek($Offset, [System.IO.SeekOrigin]::Begin)
        $buf = [byte[]]::new($fs.Length - $Offset)
        $read = $fs.Read($buf, 0, $buf.Length)
        if ($read -le 0) { return $empty }
        $safe = Get-Utf8CompletePrefixLength -Bytes $buf[0..($read - 1)] -Flush:$Flush.IsPresent
        if ($safe -le 0) { return $empty }   # 整段都是半截（例如只到了 1 个引导字节）⇒ 原样留给下次
        return [pscustomobject]@{ Text = [System.Text.Encoding]::UTF8.GetString($buf, 0, $safe); BytesRead = $safe }
    } finally { $fs.Dispose() }
}
