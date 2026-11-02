# ④c 出口的证据自查与 JVM 编码参数（#2161）
#
# 零副作用 lib：只有函数定义与注释，**不写文件、不改全局状态**；node 子进程只在
# Get-EvidenceLogVerdict 被显式调用时启动。判据要在 ③ 门行为腿**真执行**（真采集 + 断言 fffd），
# 留在门脚本里就只能读源码文本断言，而接线守护不构成 ③ 证据（同 #1285 采集层、#2045 错误行判据）。
# 行为举证 = utils/evidenceSelfcheckBehavior.test.js；接线由 utils/kotlinAllGateContract.test.js C11 钉。
#
# 本文件**只调用** #2139 的判据（.github/scripts/check-evidence-encoding.mjs），不改它、不放宽阈值、
# 也不动 docs/agents/ledger.md 的 94 份存量豁免 —— 判据唯一真源在那两个文件里，这里一个字节也没有。

function Get-KotlinJvmUtf8Args {
    # 为什么钉这一条（现测 2026-11-01/11-02，本机中文工作树）：
    #   Corretto 17 在中文路径下把 file.encoding 取成 OEM 码页（本机现测 GB18030），kotlinc 的
    #   中文诊断行就按那套码页写 stdout；采集层（lib/process-capture.ps1）按 UTF-8 解码 ⇒
    #   不可解码字节当场烤成 U+FFFD，而证据日志一旦落盘**不可逆**。
    #   只加这一个参数 ⇒ kotlin-all.log 的 fffd 由 18 降到 0。
    # 为什么不是别的两条路（票面「待确认」已现测裁定）：
    #   -Dstdout.encoding / -Dstderr.encoding —— JDK 18（JEP 400）才认，Corretto 17 不认；
    #   JAVA_TOOL_OPTIONS —— 会往子进程 stderr 打「Picked up JAVA_TOOL_OPTIONS: ...」，污染采集产物。
    # 采集层再加 GBK 兜底解码属「第二套编码知识」，另需架构裁定，不在本票射程。
    return @('-Dfile.encoding=UTF-8')
}

function Resolve-EvidenceEncodingCli {
    param([string]$StartDir)

    # 判据 CLI 在仓库根的 .github/scripts/ 下，而门脚本住在 training-app/<中文名>/scripts/ 里
    # ⇒ 逐层 Join-Path 往上找，不写反斜杠字面量（③ 门的 ubuntu runner 上反斜杠是文件名的一部分）。
    $dir = $StartDir
    for ($i = 0; $i -lt 8; $i++) {
        if ([string]::IsNullOrWhiteSpace($dir)) { break }
        $candidate = Join-Path (Join-Path (Join-Path $dir '.github') 'scripts') 'check-evidence-encoding.mjs'
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
        $parent = Split-Path -Parent $dir
        if ([string]::IsNullOrWhiteSpace($parent) -or $parent -eq $dir) { break }
        $dir = $parent
    }
    return $null
}

function Get-EvidenceLogVerdict {
    <#
        对 $Root 下的证据日志跑一次 #2139 分类判据，回结构体：
          Status  = 'clean' | 'corrupt' | 'unknown'
          Ok      = 仅 clean 为 true（fail-closed：跑不成绝不等于干净）
          Bad     = 坏件点名行（KOTLIN_ALL_READABLE_BAD file=… verdict=… fffd=… moji=…）
          Summary = 单行读数 KOTLIN_ALL_READABLE status=… files=… corrupt=… stale=… fffd=… since=…
        $SinceMs > 0 时只判该时刻之后写过的日志 —— 早于它的属 #2139 的存量豁免，不在本门射程。
        读数行不写 U+FFFD 字面量、不自己数坏字节：数字全部来自判据 CLI 的 JSON（第二真源）。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [long]$SinceMs = 0,
        [string]$CliPath,
        [string]$NodePath
    )

    function unknown([string]$reason) {
        return [pscustomobject]@{
            Status = 'unknown'; Ok = $false; Files = 0; Corrupt = 0; SkippedStale = 0; Fffd = 0
            Bad = @(); Summary = "KOTLIN_ALL_READABLE status=unknown reason=$reason"; Detail = ''
        }
    }

    $cli = if ($CliPath) { $CliPath } else { Resolve-EvidenceEncodingCli -StartDir $Root }
    if (-not $cli -or -not (Test-Path -LiteralPath $cli -PathType Leaf)) { return unknown 'cli-not-found' }

    $node = if ($NodePath) { $NodePath } else { (Get-Command node -ErrorAction SilentlyContinue).Source }
    if (-not $node) { return unknown 'node-not-found' }
    # 显式传进来的 node 也可能是假路径（行为守护专门造这一支）：查无此文件一样收成 unknown。
    # 门不许因为环境缺件而抛异常崩掉，更不许把「没跑成」读成「干净」（#2139 同一条 fail-closed）。
    if ($NodePath -and -not (Test-Path -LiteralPath $node -PathType Leaf)) { return unknown 'node-not-found' }

    # 不用 $LASTEXITCODE 判成败（同 C4 口径）：判据结论在它的 JSON 里，退出码只是附带信号。
    # $ErrorActionPreference 只在函数作用域内降级 —— node 的原生 stderr 不该把门带崩。
    $ErrorActionPreference = 'Continue'
    $cliArgs = @($cli, '--root', $Root, '--json')
    if ($SinceMs -gt 0) { $cliArgs += @('--since', [string]$SinceMs) }
    # catch 的只是「连子进程都起不来」（PS 侧 CommandNotFoundException）；判据 CLI 自己抛错
    # （root 不存在那支）走的是 native stderr，落到下面的 bad-json 分支，两者都收敛为 unknown。
    $raw = $null
    try { $raw = & $node @cliArgs 2>&1 } catch { return unknown 'node-invocation-failed' }
    $text = (@($raw | ForEach-Object { "$_" }) -join "`n").Trim()

    $scan = $null
    try { $scan = ConvertFrom-Json -InputObject $text } catch { $scan = $null }
    if ($null -eq $scan) {
        $r = unknown 'bad-json'
        $r.Detail = $text
        return $r
    }
    $props = @($scan.PSObject.Properties.Name)
    if (-not ('ok' -in $props) -or -not ('corrupt' -in $props)) {
        $r = unknown 'unexpected-json'
        $r.Detail = $text
        return $r
    }

    $files = [long]$scan.files
    $corrupt = [long]$scan.corrupt
    $stale = [long]$scan.skippedStale
    $bad = @()
    $fffd = [long]0
    if ('rows' -in $props) {
        foreach ($row in @($scan.rows)) {
            if ($null -eq $row) { continue }
            $fffd += [long]$row.fffd
            $bad += ("KOTLIN_ALL_READABLE_BAD file=$($row.file) verdict=$($row.verdict) fffd=$($row.fffd) moji=$($row.moji)")
        }
    }

    # 只认判据的两个信号，其余一律 unknown（例如 corrupt=0 但 ok=false —— 判据自己说它没跑完）
    # ok=false 且 corrupt=0（判据自己说没跑完）也算 unknown，不给「干净」放行
    $status = if ($corrupt -gt 0) { 'corrupt' } elseif ($scan.ok -eq $true -and $files -ge 1) { 'clean' } else { 'unknown' }

    return [pscustomobject]@{
        Status = $status
        Ok = ($status -eq 'clean')
        Files = $files
        Corrupt = $corrupt
        SkippedStale = $stale
        Fffd = $fffd
        Bad = $bad
        Summary = "KOTLIN_ALL_READABLE status=$status files=$files corrupt=$corrupt stale=$stale fffd=$fffd since=$SinceMs"
        Detail = ''
    }
}
