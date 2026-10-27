<#
.SYNOPSIS
    本地收口用「改动 ⇒ 行为腿受影响子集」的两步：取改动清单、调映射工具（#2034，ADR-0046 步骤 3）。

.DESCRIPTION
    ADR-0046 步骤 3 的裁定是「本地行为腿跑受影响子集、全量交 CI 4 片」。映射本身**不在本文件**：
    判据与四条兜底边住在 `scripts/behavior-suite-deps.mjs`（PR #2030 已合，覆盖率报告
    `docs/design/2019-行为腿受影响子集覆盖率报告.md`）。本文件只做两件事：

      1. `Get-ChangedPathsForSubset`：取本次改动（`git diff HEAD` + 未跟踪）——纯读、无副作用；
      2. `Get-BehaviorSubset`：把清单喂给工具（stdin，`--changed-json`），拿回 `total/subset/reasons/explain`。

    为什么改动清单要含**未跟踪**文件：`-Changed` 是收口前的本地动作，此时新写的 `.uts` / 新页
    常常还没 `git add`。`git diff HEAD` 看不见它们，于是「新加的页」这种最该跑的改动反而算不出子集 ——
    那正是报告 §8 记录过的缺口形态。

    为什么用 `--changed-json` 而不是解析人读文本：人读文本里有 `  <path>` 这样的行，解析它等于在
    PowerShell 里复制一份格式；工具把同一份文本作为 `explain` 一并给出后，本文件只**打印**它，
    人看见的与工具打出来的逐字相同（含「为什么是这个子集」的原因行）。

    失败语义（fail-closed）：路径不存在 / node 起不来 / 退出码非 0 / 输出不是 JSON / **子集为空**
    一律 `Ok = $false` 且带 `Error` 原文。子集为空**绝不能**被当成「跑完了且全绿」——
    「0 套件」在本仓长得和绿一模一样（AGENTS.md ③ 条记过这个形态）。

.NOTES
    纯读：不写文件、不跑测试、不取 HBuilderX 锁、不碰设备。
#>

function Get-ChangedPathsForSubset {
    <#
      本次改动清单（仓库根相对路径，一行一条）。含未跟踪文件，见 .SYNOPSIS 的理由。
      `core.quotepath=false`：中文目录名不做 octal 转义（与 dev-finish 取 porcelain 同一取法，#1037）。
      取不到（git 不可用 / 不在仓库里）⇒ throw，由调用点判「环境不可用」而不是「没有改动」。
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$ProjectDir)

    # 子进程（git）的 stdout 按 `[Console]::OutputEncoding` 解码：本机默认 **gb2312** ⇒ 中文目录名
    # 会变成乱码串；而乱码路径喂给映射工具**一条套件都匹配不到**（假绿的另一种形态，与 #1526 的
    # 「看得见但读不懂」同源）。dev-finish.ps1 文件头已全局设过 UTF-8；这里再设一次并在 finally 里
    # 恢复，是为了**脱离该脚本单独调用**（守护测试、排障）时也成立。
    $prevEncoding = $null
    try {
        $prevEncoding = [Console]::OutputEncoding
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    }
    catch {
        $prevEncoding = $null
    }

    try {
        $paths = [System.Collections.Generic.List[string]]::new()
        foreach ($spec in @(
                @('diff', '--name-only', 'HEAD'),
                @('ls-files', '--others', '--exclude-standard')
            )) {
            $gitArgs = @('-C', $ProjectDir, '-c', 'core.quotepath=false') + $spec
            $raw = @()
            try {
                $raw = @(& git @gitArgs 2>$null)
            }
            catch {
                throw "git $($spec -join ' ') 起不来：$($_.Exception.Message)"
            }
            if ($LASTEXITCODE -ne 0) { throw "git $($spec -join ' ') 失败（exit $LASTEXITCODE）" }
            foreach ($line in $raw) {
                $p = "$line".Trim()
                if (-not $p) { continue }
                # 少见形态：路径里带引号/反斜杠时 git 仍会加引号（quotepath=false 只管非 ASCII）
                if ($p -match '^"(.*)"$') { $p = $Matches[1] }
                if (-not $paths.Contains($p)) { $paths.Add($p) }
            }
        }
        return @($paths)
    }
    finally {
        if ($null -ne $prevEncoding) {
            try { [Console]::OutputEncoding = $prevEncoding } catch { }
        }
    }
}

function Get-BehaviorSubset {
    <#
      改动清单 ⇒ 应跑的行为腿套件。返回对象字段：
        Ok / ExitCode / Error          结论与失败原文（Ok=$false 时 Subset 无意义）
        ChangedCount / Total           改动数 / 行为腿套件总数（**现测**，不来自任何文档）
        Subset                         相对路径数组（`utils/xxxBehavior.test.js` 形态）
        Explain                        工具那份人读报告的逐行数组（调用方原样打印 ——
                                       原因行、未命中警告、尾行提示全在里面）

      子集为空 ⇒ Ok=$false（不把「没跑」当绿）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [string[]]$Changed = @(),
        [string]$ToolRelative = 'scripts/behavior-suite-deps.mjs'
    )

    $result = [pscustomobject]@{
        Ok           = $false
        ExitCode     = -1
        Error        = ''
        ChangedCount = @($Changed | Where-Object { $_ }).Count
        Total        = 0
        Subset       = @()
        Explain      = @()
    }

    $tool = Join-Path $ProjectDir $ToolRelative
    if (-not (Test-Path -LiteralPath $tool)) {
        $result.Error = "找不到映射工具：$tool"
        return $result
    }

    $utf8 = [System.Text.Encoding]::UTF8
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    # 执行器取 PATH 里的 node：工具是 ESM 脚本，本仓所有入口都这么起（不引第二个 node 路径真源）。
    $psi.FileName = 'node'
    # ArgumentList（不是拼一个字符串）：路径里有空格/中文也不用自己转义
    [void]$psi.ArgumentList.Add($tool)
    [void]$psi.ArgumentList.Add('--changed-json')
    [void]$psi.ArgumentList.Add('--changed-from')
    [void]$psi.ArgumentList.Add('stdin')
    $psi.WorkingDirectory = $ProjectDir
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardInputEncoding = $utf8
    $psi.StandardOutputEncoding = $utf8
    $psi.StandardErrorEncoding = $utf8

    try { $proc = [System.Diagnostics.Process]::Start($psi) }
    catch {
        $result.Error = "起不来 node（PATH 里没有）：$($_.Exception.Message)"
        return $result
    }

    # 先写 stdin 并关掉：工具读的是「读到底」的 `fs.readFileSync(0)`，不关它就永远等不到 EOF。
    $payload = (@($Changed | Where-Object { $_ } | ForEach-Object { "$_".Trim() }) -join "`n")
    try {
        $proc.StandardInput.Write($payload + "`n")
        $proc.StandardInput.Close()
    }
    catch {
        $result.Error = "往映射工具写 stdin 失败：$($_.Exception.Message)"
        try { $proc.Kill() } catch { }
        return $result
    }
    $out = $proc.StandardOutput.ReadToEnd()
    $err = $proc.StandardError.ReadToEnd()
    $proc.WaitForExit()
    $result.ExitCode = $proc.ExitCode

    if ($proc.ExitCode -ne 0) {
        $detail = ("$err`n$out").Trim() -replace '\s+', ' '
        if ($detail.Length -gt 300) { $detail = $detail.Substring(0, 300) }
        $result.Error = "映射工具 exit=$($proc.ExitCode)：$detail"
        return $result
    }

    try { $json = $out | ConvertFrom-Json }
    catch {
        $result.Error = "映射工具输出不是 JSON：$($_.Exception.Message)"
        return $result
    }

    $subset = @($json.subset | Where-Object { $_ })
    if ($subset.Count -eq 0) {
        $result.Error = '子集为空（0 个行为套件）—— 不以「没跑」当绿'
        return $result
    }

    $result.Total = [int]$json.total
    $result.ChangedCount = [int]$json.changedCount
    $result.Subset = $subset
    $result.Explain = @($json.explain | ForEach-Object { "$_" })
    $result.Ok = $true
    return $result
}
