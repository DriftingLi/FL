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
      - `Send-GateComment`     ：把**同一份**正文贴到 commit（优先落点）+ 号码（展示副本/降级落点）上（#1770/#1841）

      另：`Get-CommitCommentKind` 是「commit 评论投递结果」的分类判据，只给 `Send-GateComment`
      与守护测试用，**不对外**（调用方不自己解析 `gh` 的输出）。

    #1770（证据主键从 PR 号改成 commit sha）：判据评论**优先贴在 commit 上**，校验器按 sha 找证据；
    号码评论是给人看的展示副本；#1841 起 commit 端点回 404 时，号码评论升格为正式证据落点 ——
    于是「贴评论」与「绑哪个 sha」是同一件事的两半，都收在本文件。

    #1841（404 降级常态）：GitHub 对本仓 `POST /repos/<owner>/<repo>/commits/<sha>/comments` 长期返回
    **HTTP 404**（同 token 在别的仓正常，#1838），commit 评论贴不上去。处置**不是**静默 warn、也不是把门
    判红，而是**显式降级**：仍把**同一份**正文（marker 与 `- commit: <sha>` 都不变）贴到票
    （`-PostToIssue <票号>` 那个落点）当正式证据落点，并由 `Send-GateComment` 在返回值里如实回报
    `CommitKind=commit-unavailable` / `CommitUnavailable=$true` / `EvidenceUrl=<票评论 URL>` /
    `Notes`（含落点 URL）；六个调用方据此打印机检行 `GATE_COMMENT_DEGRADED tier=issue url=<URL>` 与
    一行人读说明。**为什么不是错误**：#1209 血账规定报告层崩了不许把「门通过」改成 exit 1；404 是
    **端点能力**缺失，证据本身仍完整（同一份正文、同一 marker、同一 sha），只是落点换了。校验器侧把
    票评论纳入候选（与 PR / commit 评论同构）由 A 轨改，本文件不碰 `.github/**`。

    与 #1695 fail-closed 的关系：#1695 管的是**贴之前**的绑定判定（sha 在 GitHub 上点不开就**不贴**
    commit 评论，口径 C）；#1841 管的是**贴之时**端点拒绝。两者不互相放宽：口径 C 仍然不贴；只有
    「已判定可贴、且真的发了请求、端点回 404」才降级，401/403/5xx/网络错误一律仍是硬失败
    （`CommitKind=failed`，调用方打印 `GATE_COMMENT_FAILED`）。

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
    两种形态各一腿 + 推不动时的降级一腿 + commit 评论投递 200/404/500 各一腿）。本文件**不引**
    `lib/gate-common.ps1`：HEAD sha 的取法只有
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

function Get-CommitCommentKind {
    <#
    .SYNOPSIS
        commit 评论投递结果的分类判据（#1841）：`ok` / `commit-unavailable` / `failed`。
    .NOTES
        判据只有三条，刻意不看 `push` / `gh` 的大小写与输出里的多余空白，也不解析 JSON：
          ① exit 0 且输出以 `http(s)://` 开头 ⇒ ok（`-q '.html_url'` 打出来的就是评论链接）
          ② 输出含 `(HTTP 404)` ⇒ commit-unavailable（本仓该端点的长期形态，见 #1838）
          ③ 其余（401/403/5xx/网络错误/空输出/输出不成链接） ⇒ failed
        404 判据**只准出现在这里**：调用方与 `Send-GateComment` 都不自己看 `gh` 的错误文本。
    #>
    [CmdletBinding()]
    param(
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Output,
        [Parameter(Mandatory = $true)][int]$ExitCode
    )
    $text = "$Output"
    if ($ExitCode -eq 0 -and $text.Trim() -match '^https?://') { return 'ok' }
    if ($text -match '\(HTTP 404\)') { return 'commit-unavailable' }
    return 'failed'
}

function Send-GateComment {
    <#
    .SYNOPSIS
        把**同一份**评论正文贴到两个落点：commit（优先判据面）+ 号码（展示副本/降级落点）。不抛异常，失败如实回报。
    .DESCRIPTION
        #1770：验收证据的主键从 PR 号改成 commit sha —— 校验器按 sha 找证据，所以**判据评论优先贴在
        commit 上**（`POST /repos/{owner}/{repo}/commits/{sha}/comments`）；号码评论（`/issues/{N}/comments`，
        PR 号在 REST 里就是 issue 号）是给人看的副本，**不再假设 PR 存在**（新流程是证据齐了再开 PR）。
        #1841：commit 端点回 404 时号码评论就是**正式证据落点**（返回值 EvidenceUrl 指向它）。

        正文一律走**临时文件 + `--input`**：正文里有反引号 / 中文 / 多行，内联进命令行会被 PowerShell 的
        引号规则改写。任何一步失败只回报（Errors），**绝不抛异常改门结论**（#1209 血账：报告层崩了不许把
        「门通过」变成 exit 1）。

        #1841：commit 端点回 404 时**不提前 return、不把票评论删掉** —— 同一份正文照旧贴票，返回值里
        用 CommitKind / CommitUnavailable / EvidenceUrl / Notes 区分「降级」与「硬失败」；404 不记进
        Errors（降级不是错误），硬失败（401/403/5xx/网络错误）才记。
    .OUTPUTS
        [pscustomobject] @{
            CommitOk          = <bool>    # 向后兼容：commit 评论是否成功
            CommitUrl         = <string>  # 向后兼容
            IssueOk           = <bool>    # 向后兼容：票/号码评论是否成功
            IssueUrl          = <string>  # 向后兼容
            Errors            = <string[]># 向后兼容：硬失败原因（404 降级不进这里）
            CommitKind        = <'ok'|'commit-unavailable'|'failed'|'skipped'>  # 跳过 = 没有 sha 或 Reachable=false
            CommitUnavailable = <bool>    # 端点 404（已降级）
            EvidenceUrl       = <string>  # CommitUrl；降级且票评论成功 ⇒ IssueUrl；否则空
            Notes             = <string[]># 降级时的如实说明行（含落点 URL）
        }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Sha,
        # 绑定口径算出来的可达性：不可达（口径 C）时不贴判据评论 —— 贴上去就是悬空证据（#1695）
        [bool]$Reachable = $true,
        [int]$IssueNumber = 0,
        [AllowEmptyString()][Parameter(Mandatory = $true)][string]$Body,
        # 仅供守护测试注桩（行为测试要真执行本函数的投递分支）；默认值就是生产路径的 gh。
        [string]$GhCli = 'gh'
    )
    $errors = New-Object System.Collections.Generic.List[string]
    $commitOk = $false
    $commitUrl = ''
    $issueOk = $false
    $issueUrl = ''
    # 默认 failed：连投递都没走到的输入错误也算硬失败；`skipped` 只留给「没有 sha / 不可达」两种形态
    $commitKind = 'failed'
    $commitUnavailable = $false
    $tmp = ''
    try {
        if (-not $Body) {
            $errors.Add('正文为空，跳过投递')
        }
        elseif (-not (Get-Command $GhCli -ErrorAction SilentlyContinue)) {
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
                    $commitKind = 'skipped'
                    $errors.Add('没有可绑定的 sha：判据评论不贴')
                }
                elseif (-not $Reachable) {
                    $commitKind = 'skipped'
                    $errors.Add("sha $Sha 不在远端可达（GitHub 上点不开）：判据评论不贴，避免制造悬空证据（#1695 口径 C）")
                }
                else {
                    $out1 = (& $GhCli api --method POST "repos/$slug/commits/$Sha/comments" --input $tmp -q '.html_url' 2>&1 | Out-String)
                    $commitKind = Get-CommitCommentKind -Output $out1 -ExitCode $LASTEXITCODE
                    if ($commitKind -eq 'ok') {
                        $commitOk = $true
                        $commitUrl = $out1.Trim()
                    }
                    elseif ($commitKind -eq 'commit-unavailable') {
                        # 降级不是错误：不写 Errors（硬失败才写）；落点与说明在下方 Notes 里如实给出（#1841）
                        $commitUnavailable = $true
                    }
                    else {
                        $errors.Add("commit 评论失败：$($out1.Trim())")
                    }
                }
                # #1841：404 降级时**仍然**把同一份正文贴票 —— 正文本来就同时投递两个落点，这里不提前 return。
                if ($IssueNumber -gt 0) {
                    $out2 = (& $GhCli api --method POST "repos/$slug/issues/$IssueNumber/comments" --input $tmp -q '.html_url' 2>&1 | Out-String)
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
    # 降级结论必须写进 Notes（调用方逐行打印）——「静默 warn」正是本票要消灭的形态（#1841）
    $notes = New-Object System.Collections.Generic.List[string]
    if ($commitUnavailable) {
        if ($issueOk) {
            $notes.Add("- ⚠️ commit 评论端点不可用（HTTP 404）：证据主键降级为票评论 $issueUrl（#1841）")
        }
        else {
            $notes.Add('- ⚠️ commit 评论端点不可用（HTTP 404）：本应降级为票评论，但票评论也投递失败（#1841）')
        }
    }
    $evidenceUrl = ''
    if ($commitUrl) { $evidenceUrl = $commitUrl }
    elseif ($commitUnavailable -and $issueOk) { $evidenceUrl = $issueUrl }
    return [pscustomobject]@{
        CommitOk          = $commitOk
        CommitUrl         = $commitUrl
        IssueOk           = $issueOk
        IssueUrl          = $issueUrl
        Errors            = @($errors)
        CommitKind        = $commitKind
        CommitUnavailable = $commitUnavailable
        EvidenceUrl       = $evidenceUrl
        Notes             = @($notes)
    }
}
