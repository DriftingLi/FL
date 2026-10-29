<#
.SYNOPSIS
    门证据「跑在哪个提交上」的口径 + 证据块打印（A1 形态，#2100）。

.DESCRIPTION
    背景（#1690 run15/run16 实证）：② 门的归档链路是 `git add` + `git commit`（**仅本地**），
    随后写出的证据把 sha 指向那个本地提交；远端点不开的 sha 等于悬空证据。

    ## A1（2026-10-29，#2100）改了什么

    证据载体从「sha 绑定评论」改成「PR 正文一行 `跑于 <sha7>` + 门读数行」，于是本文件两处变动：

      · **投递退役**：`Send-GateComment` 不再 POST 任何评论，只把该粘进正文的块打印出来。
        「commit 端点 404 → 降级贴票评论」那条链（#1841）随之消失 —— 它买不到任何判定
        （#2099 实读：pr-evidence 不在 ruleset 必检，评论也从来不是判据的唯一来源）。
      · **绑定判定保留**：脚本仍要知道「这次证据是在哪个 sha 上跑的」，因为那一行要写进正文。
        `Resolve-CommitBinding` 的三档与 fail-closed 语义不变：#1695 的「不把点不开的 sha 伪装成
        证据锚点」仍然成立，A1 下它体现为「`跑于` 该写哪个 sha」。

    已入库成员（调用方只准用这四枚）

      - `Test-ShaReachable`     ：某 sha 是否已落在远端分支上（`git branch -r --contains`，离线）
      - `Push-ArchiveCommit`    ：把当前 HEAD 推上 PR head 分支（推得动就推，推不动**如实回报**）
      - `Resolve-CommitBinding` ：给出 `跑于` 该写的 sha，以及**必须如实附上的标注行**
      - `Send-GateComment`      ：**只打印**证据块（A1：不投递；函数名与签名保留，调用方零改动）

    绑定优先级（fail-closed：宁可标「不可点开」，绝不把点不开的 sha 伪装成证据锚点）：

      A) HEAD 远端可达   ⇒ 绑 HEAD，无标注行。
      B) HEAD 不可达、但远端分支 tip 是 HEAD 的祖先，且 `tip..HEAD` 只动 `docs/verification/`
                         ⇒ 绑**远端 tip**（点得开），并加一行「本地归档 sha（未推送）」；
                            这正是「归档提交只活在本地」那一形态的兜底。
      C) 其余             ⇒ 绑 HEAD 并标注「未推送/点不开」。此形态 pr-evidence 的祖先核对会**如实判红**，
                            不静默放过 —— 悬空证据宁可红，不可假绿。

    准入门槛（与 `lib/gate-common.ps1` 同一条）：本文件只做**纯 git 查询 + 一次显式 push + 打印证据块**，
    不读设备、不解析评论正文、不判断门结论、**不发任何网络写请求**。push 只推当前分支到同名远端分支
    （`git push <remote> HEAD:refs/heads/<branch>`）：**不带 force、不动 tag、不推其它 ref**。

    判据面守护：`utils/gateCommitBindingBehavior.test.js`（真执行本文件的函数，覆盖远端可达 / 仅本地
    两种形态 + 推不动时的降级）。本文件**不引** `lib/gate-common.ps1`：HEAD sha 的取法只有
    gate-common 那一份，调用方自己取好再喂进来（`-HeadSha`），这里不抄第二份。
#>

function Test-ShaReachable {
    <#
    .SYNOPSIS
        sha 是否已落在某个远端分支上（用本地 remote-tracking ref 判，不联网）。
    .NOTES
        `git branch -r --contains` 读的是 `refs/remotes/**`：本仓的 push 会顺带更新对应
        remote-tracking ref，所以「刚推上去」也能立刻判为可达；远端 tip 被别人改过而本地没
        fetch 时可能读到过期快照 —— 那种偏差只会让 A->B/C 的判定偏保守（宁可标不可点开）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Sha,
        [string]$Remote = 'origin'
    )
    if (-not $Sha) { return $false }
    $out = (& git -C $ProjectDir branch -r --contains $Sha 2>$null | Out-String)
    if ($LASTEXITCODE -ne 0) { return $false }
    foreach ($line in ($out -split "`r?`n")) {
        $t = $line.Trim()
        if ($t -and $t.StartsWith("$Remote/") -and $t -ne "$Remote/HEAD") { return $true }
    }
    return $false
}

function Push-ArchiveCommit {
    <#
    .SYNOPSIS
        把当前 HEAD 推上 `<remote>/<branch>`；返回 @{ Pushed = <bool>; Reason = <string> }，不抛异常。
    .NOTES
        只推当前分支（`HEAD:refs/heads/<branch>`）—— 不是 `--all`、不带 `--force`。
        推不动（分支保护 / 无权限 / 非快进 / 离线）时如实返回 Pushed=$false + 原因末行，
        由调用方决定怎么标注（口径见文件头 A/B/C）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [Parameter(Mandatory = $true)][string]$Branch,
        [string]$Remote = 'origin'
    )
    if (-not $Branch -or $Branch -eq 'HEAD') {
        return @{ Pushed = $false; Reason = '取不到当前分支名（detached HEAD？）' }
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        return @{ Pushed = $false; Reason = 'git 不可用' }
    }
    $out = (& git -C $ProjectDir push $Remote "HEAD:refs/heads/$Branch" 2>&1 | Out-String)
    if ($LASTEXITCODE -eq 0) { return @{ Pushed = $true; Reason = '' } }
    $tail = (@($out -split "`r?`n") | Where-Object { $_.Trim() } | Select-Object -Last 1)
    return @{ Pushed = $false; Reason = ("$tail").Trim() }
}

function Resolve-CommitBinding {
    <#
    .SYNOPSIS
        算出评论 `- commit:` 该绑的 sha，以及必须如实附在正文里的标注行（口径见文件头 A/B/C）。
    .OUTPUTS
        [pscustomobject] @{ Sha = <string>; Reachable = <bool>; Mode = <'remote-head'|'remote-tip'|'local-head'|'none'>; NoteLines = <string[]> }
    .NOTES
        `-HeadSha` 由调用方喂（各门脚本自己那份取法：gate-common 的 `Get-HeadSha` 或既有的
        `git rev-parse HEAD`）—— 本文件刻意**不定义** sha 取法，避免出现第二真源。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$HeadSha,
        [string]$Branch = '',
        [string]$Remote = 'origin'
    )
    $unpushedLabel = '本地归档 sha（未推送）'
    if (-not $HeadSha) {
        # 取不到 sha（git 失败 / 空仓）不是「偶发异常」，而是**拿不到可核验绑定**这一事实：
        # 如实回 none + 一句说明，让调用方的评论正文写明「本评论不带 commit 绑定」。
        return [pscustomobject]@{
            Sha       = ''
            Reachable = $false
            Mode      = 'none'
            NoteLines = @("- ⚠️ 取不到 HEAD sha（git 取值失败）：本评论**不带可核验的 commit 绑定**（#1695）")
        }
    }
    $short = $HeadSha.Substring(0, [Math]::Min(7, $HeadSha.Length))

    if (Test-ShaReachable -ProjectDir $ProjectDir -Sha $HeadSha -Remote $Remote) {
        return [pscustomobject]@{ Sha = $HeadSha; Reachable = $true; Mode = 'remote-head'; NoteLines = @() }
    }

    if (-not $Branch) {
        $Branch = (& git -C $ProjectDir rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
        if ($Branch -eq 'HEAD') { $Branch = '' }
    }
    if ($Branch) {
        $tip = (& git -C $ProjectDir rev-parse --verify --quiet "refs/remotes/$Remote/$Branch" 2>$null | Out-String).Trim()
        if ($tip -and $tip -ne $HeadSha) {
            & git -C $ProjectDir merge-base --is-ancestor $tip $HeadSha 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) {
                # `--relative` 让路径以 $ProjectDir 为基准（本仓是 monorepo，仓库根相对路径会多一层前缀）
                $names = (& git -C $ProjectDir diff --name-only --relative "$tip..$HeadSha" -- . 2>$null | Out-String)
                $onlyArchive = $true
                foreach ($n in ($names -split "`r?`n")) {
                    $t = $n.Trim()
                    if (-not $t) { continue }
                    if ($t -notlike 'docs/verification/*') { $onlyArchive = $false; break }
                }
                if ($onlyArchive) {
                    $tipShort = $tip.Substring(0, [Math]::Min(7, $tip.Length))
                    return [pscustomobject]@{
                        Sha       = $tip
                        Reachable = $true
                        Mode      = 'remote-tip'
                        NoteLines = @("- ⚠️ ${unpushedLabel}：``$short``（未推到远端；本评论绑定的 ``$tipShort`` 是远端分支 ``$Remote/$Branch`` 的 tip，点得开 —— #1695）")
                    }
                }
            }
        }
    }

    return [pscustomobject]@{
        Sha       = $HeadSha
        Reachable = $false
        Mode      = 'local-head'
        NoteLines = @("- ⚠️ ${unpushedLabel}：``$short`` —— GitHub 上点不开（#1695）；把它推上 ``$Remote`` 的同名分支后重跑本门，即可让评论绑到远端可达的 sha")
    }
}

function Get-GateRepoSlug {
    <#
    .SYNOPSIS
        取 GitHub REST 端点要的那一段 `owner/repo`；取不到就回空串（不抛异常）。
    .NOTES
        A1（#2100）后本文件不再发任何 gh 写请求，本函数因此**没有生产调用方**。
        暂留一版是为了「万一要回滚投递」时不必重写 URL 解析（回滚路径见 #2100 票面）；
        下一个版本若仍无人用，随 `-PostToIssue` 参数一起删。
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$ProjectDir)
    $url = ''
    try { $url = (& git -C $ProjectDir remote get-url origin 2>$null | Out-String).Trim() } catch { $url = '' }
    if ($url) {
        # https://github.com/OWNER/REPO(.git) / git@github.com:OWNER/REPO(.git) / ssh://git@github.com/OWNER/REPO(.git)
        $m = [regex]::Match($url, 'github\.com[:/]+([^/\s]+)/([^/\s]+?)(?:\.git)?/?$')
        if ($m.Success) { return ('{0}/{1}' -f $m.Groups[1].Value, $m.Groups[2].Value) }
    }
    try {
        $nwo = (& gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>$null | Select-Object -First 1)
        if ($nwo) { return ("$nwo").Trim() }
    } catch { }
    return ''
}

function Send-GateComment {
    <#
    .SYNOPSIS
        **A1 起只打印可粘贴的证据块，不再投递任何评论**（2026-10-29，#2100 决策 3/6）。

    .DESCRIPTION
        退役原因：校验器只读 PR 正文（`跑于 <sha7>` + 门读数行），评论不再是证据载体；
        于是「贴 commit 评论 → 404 → 降级贴票评论」这条链买不到任何判定，只剩维护成本。
        保留函数名与签名，是为了让五个调用方（mp-weixin-check / kotlin-all-check / compile-check /
        device-capture / emulator-smoke）**不改一行调用代码**就能停下投递 —— 改函数体比改五个脚本安全。

        行为：把准备好的正文原样打印出来（它就是该粘进 PR `## 验收证据` 段的那段），
        返回同形状对象供调用方既有分支消费：`CommitKind='retired'`、
        `CommitOk/IssueOk=$false`、`Errors` 为空（**退役不是失败**，别让调用方打红字）、
        `Notes` 里留一行去路说明。

        `-Sha` / `-Reachable` / `-IssueNumber` 仍然收（签名兼容），但不再产生任何副作用：
        不校验可达性、不拼 API 端点、不写临时文件、不调用 gh。
        正文里的 `commit: <sha>` 行不带用 —— A1 的行内已有 `跑于 <sha7>`。

    .OUTPUTS
        [pscustomobject] @{
            CommitOk          = $false
            CommitUrl         = ''
            IssueOk           = $false
            IssueUrl          = ''
            Errors            = @()
            CommitKind        = 'retired'
            CommitUnavailable = $false
            EvidenceUrl       = ''
            Notes             = <string[]>
            Printed           = <bool>
            Body              = <string>
        }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Sha,
        [bool]$Reachable = $true,
        [int]$IssueNumber = 0,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Body,
        # 保留仅为签名兼容与守护测试读形状；A1 下不再调用它。
        [string]$GhCli = 'gh'
    )
    $notes = New-Object System.Collections.Generic.List[string]
    $printed = $false
    if ("$Body".Trim()) {
        Write-Host ''
        Write-Host '===== 证据块（A1：直接粘进 PR 的 ## 验收证据 段，不再贴评论）=====' -ForegroundColor Cyan
        Write-Host $Body
        Write-Host '===== 证据块结束 =====' -ForegroundColor Cyan
        Write-Host ''
        $printed = $true
        $notes.Add('- A1（#2100）：门证据改为 PR 正文一行 `跑于 <sha7>` + 读数行；评论投递（commit / 票）已退役，上面的块粘进正文即可。')
    }
    else {
        $notes.Add('- 正文为空：没有可打印的证据块。')
    }
    return [pscustomobject]@{
        CommitOk          = $false
        CommitUrl         = ''
        IssueOk           = $false
        IssueUrl          = ''
        Errors            = @()
        CommitKind        = 'retired'
        CommitUnavailable = $false
        EvidenceUrl       = ''
        Notes             = @($notes)
        Printed           = $printed
        Body              = $Body
    }
}
