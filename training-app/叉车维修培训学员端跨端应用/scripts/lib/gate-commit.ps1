<#
.SYNOPSIS
    门评论的「sha 绑定」口径：只绑 GitHub 上点得开的提交（#1695）。

.DESCRIPTION
    背景（#1690 run15/run16 实证）：② 门的归档链路是 `git add` + `git commit`（**仅本地**），
    随后贴出的证据评论把 `- commit: <sha>` 指向那个本地提交；而 `.github/workflows/pr-evidence.yml`
    只核「sha 是不是 PR head 的祖先」，**核不出「远端可达」** —— 于是任何一次 rebase / force-push /
    分支删除都会让既往门评论里的 sha 在 GitHub 上 404，证据链悬空（run15 @32e0eed、run16 @d4fc737）。

    本文件的成员把口径收成**一处**（各门脚本只调函数，不各写一份 git 逻辑）：

      已入库成员（调用方只准用这四枚）
      - `Test-ShaReachable`    ：某 sha 是否已落在远端分支上（`git branch -r --contains`，离线）
      - `Push-ArchiveCommit`   ：把当前 HEAD 推上 PR head 分支（推得动就推，推不动**如实回报**）
      - `Resolve-CommitBinding`：给出评论 `- commit:` 该绑哪个 sha，以及**必须如实附上的标注行**
      - `Send-GateComment`     ：把**同一份**正文贴到 commit（判据面）+ 号码（展示副本）上（#1770）

    #1770（证据主键从 PR 号改成 commit sha）：判据评论必须**贴在 commit 上**，校验器按 sha 找证据；
    号码评论只是给人看的展示副本 —— 于是「贴评论」与「绑哪个 sha」是同一件事的两半，都收在本文件。

    绑定优先级（fail-closed：宁可标「不可点开」，绝不把点不开的 sha 伪装成证据锚点）：

      A) HEAD 远端可达   ⇒ 绑 HEAD，无标注行。
      B) HEAD 不可达、但远端分支 tip 是 HEAD 的祖先，且 `tip..HEAD` 只动 `docs/verification/`
                         ⇒ 绑**远端 tip**（点得开），并加一行「本地归档 sha（未推送）」；
                            这正是「归档提交只活在本地」那一形态的兜底。
      C) 其余             ⇒ 绑 HEAD 并标注「未推送/点不开」。此形态 pr-evidence 的祖先核对会**如实判红**，
                            不静默放过 —— 悬空证据宁可红，不可假绿。

    准入门槛（与 `lib/gate-common.ps1` 同一条）：本文件只做**纯 git 查询 + 一次显式 push + 评论投递**
    （后者仅 `Send-GateComment` 两处 `gh api` POST：commit 评论 / 号码评论），不读设备、不解析评论正文、
    不判断门结论。push 只推当前分支到同名远端分支
    （`git push <remote> HEAD:refs/heads/<branch>`）：**不带 force、不动 tag、不推其它 ref**。

    判据面守护：`utils/gateCommitBindingBehavior.test.js`（真执行本文件的函数，含「远端可达 / 仅本地」
    两种形态各一腿 + 推不动时的降级一腿）。本文件**不引** `lib/gate-common.ps1`：HEAD sha 的取法只有
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
        先解析 `origin` 的 URL（离线、确定），解析不出来才问 `gh repo view`（要 gh 已登录）。
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
        把**同一份**评论正文贴到两个落点：commit（判据面）+ 号码（展示副本）。不抛异常，失败如实回报。
    .DESCRIPTION
        #1770：验收证据的主键从 PR 号改成 commit sha —— 校验器按 sha 找证据，所以**判据评论必须贴在
        commit 上**（`POST /repos/{owner}/{repo}/commits/{sha}/comments`）；号码评论（`/issues/{N}/comments`，
        PR 号在 REST 里就是 issue 号）只是给人看的副本，**不再假设 PR 存在**（新流程是证据齐了再开 PR）。

        正文一律走**临时文件 + `--input`**：正文里有反引号 / 中文 / 多行，内联进命令行会被 PowerShell 的
        引号规则改写。任何一步失败只回报（Errors），**绝不抛异常改门结论**（#1209 血账：报告层崩了不许把
        「门通过」变成 exit 1）。
    .OUTPUTS
        [pscustomobject] @{ CommitOk = <bool>; CommitUrl = <string>; IssueOk = <bool>; IssueUrl = <string>; Errors = <string[]> }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Sha,
        # 绑定口径算出来的可达性：不可达（口径 C）时不贴判据评论 —— 贴上去就是悬空证据（#1695）
        [bool]$Reachable = $true,
        [int]$IssueNumber = 0,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Body
    )
    $errors = New-Object System.Collections.Generic.List[string]
    $commitOk = $false
    $commitUrl = ''
    $issueOk = $false
    $issueUrl = ''
    $tmp = ''
    try {
        if (-not $Body) {
            $errors.Add('正文为空，跳过投递')
        }
        elseif (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
            $errors.Add('找不到 gh CLI')
        }
        else {
            $slug = Get-GateRepoSlug -ProjectDir $ProjectDir
            if (-not $slug) {
                $errors.Add('取不到 origin 的 owner/repo（gh api 端点无从拼起）')
            }
            else {
                $tmp = Join-Path $env:TEMP ('gate-comment-' + [guid]::NewGuid().ToString('N') + '.json')
                [System.IO.File]::WriteAllText($tmp, (@{ body = $Body } | ConvertTo-Json -Compress), [System.Text.UTF8Encoding]::new($false))
                if (-not $Sha) {
                    $errors.Add('没有可绑定的 sha：判据评论不贴')
                }
                elseif (-not $Reachable) {
                    $errors.Add("sha $Sha 不在远端可达（GitHub 上点不开）：判据评论不贴，避免制造悬空证据（#1695 口径 C）")
                }
                else {
                    $out1 = (& gh api --method POST "repos/$slug/commits/$Sha/comments" --input $tmp -q '.html_url' 2>&1 | Out-String)
                    $url1 = $out1.Trim()
                    if ($LASTEXITCODE -eq 0 -and $url1 -match '^https?://') {
                        $commitOk = $true
                        $commitUrl = $url1
                    }
                    else {
                        $errors.Add("commit 评论失败：$url1")
                    }
                }
                if ($IssueNumber -gt 0) {
                    $out2 = (& gh api --method POST "repos/$slug/issues/$IssueNumber/comments" --input $tmp -q '.html_url' 2>&1 | Out-String)
                    $url2 = $out2.Trim()
                    if ($LASTEXITCODE -eq 0 -and $url2 -match '^https?://') {
                        $issueOk = $true
                        $issueUrl = $url2
                    }
                    else {
                        $errors.Add("展示副本（#$IssueNumber）失败：$url2")
                    }
                }
            }
        }
    }
    catch {
        $errors.Add("投递异常：$($_.Exception.Message)")
    }
    finally {
        if ($tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
    }
    return [pscustomobject]@{
        CommitOk  = $commitOk
        CommitUrl = $commitUrl
        IssueOk   = $issueOk
        IssueUrl  = $issueUrl
        Errors    = @($errors)
    }
}
