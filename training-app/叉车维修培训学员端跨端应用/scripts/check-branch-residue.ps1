#!/usr/bin/env pwsh
<#
.SYNOPSIS
    机检「已合并 PR 的 head 分支是否仍留在远端」—— 分支残留的收口判据。

.DESCRIPTION
    为什么需要这个判据（2026-09-19 实测：本仓 14 个远端分支残留，13 个对应已 MERGED 的 PR）：
    删除动作有两条**静默**失败路径，两条都不报错：

      1) 命令启动时该 PR 已经是 MERGED（重试 / 网页合并 / REST 兜底 / auto-merge）：
         gh 2.97.0 `pkg/cmd/pr/merge/merge.go` 的 `deleteRemoteBranch()` 里，
         远端删除包在 `if !m.merged { api.BranchDeleteRemote(...) }` 内，
         而 `m.merged` 在构造时按 `pr.State == MERGED` 定死 ⇒ 整段跳过，
         之后**仍然无条件打印** `Deleted remote branch <name>`。
      2) 本地分支被 worktree 占用：调用顺序是 `deleteLocalBranch()` → 出错即 `return err`
         → 才 `deleteRemoteBranch()`，`git branch -D` 一失败，远端删除整段跑不到。

    仓库级 `delete_branch_on_merge` 关闭时没有任何服务端兜底 ⇒ 残留永久留下。

    常规判据在 squash-only 仓库里同样失效：多提交分支被 squash 后 patch-id 与任一原提交都不同，
    `git cherry` / `merge-base --is-ancestor` 都会把它读成「未合并」（本次 12/13 的 squash 提交
    其实已是 master 祖先，内容早已落地）。所以本脚本不用 git 的祖先关系，改用**事实源**：
    对每个远端分支问 GitHub「有没有关联 PR、什么状态」。

    判定（只看 PR 状态，不看本地）：
      · OPEN    ⇒ 有活跃 PR，**不动**（绝不删）
      · MERGED  ⇒ 残留（内容已落地，可删）
      · CLOSED  ⇒ 未合并即关闭，需人裁定（-IncludeClosed 时计入退出码）
      · 无关联 PR ⇒ 孤儿分支，需人裁定（同 -IncludeClosed；**永不**自动删）

    ⚠️ 本脚本**不跑** `git fetch`／`git ls-remote`：远端分支的存在性只取 GitHub API，
    本地 remote-tracking ref 可能过期（例：`git fetch --prune` 在受限环境里会 `ssh: signal pipe`
    失败，本地视图就永远不刷新）。本地信息只用来告诉你**收口顺序**（worktree → 本地分支 → 远端）。

.PARAMETER Branch
    只检查这一条分支。绿色面自测用：指向一条已清理干净的分支应 exit 0 并打印 ABSENT。

.PARAMETER IncludeClosed
    把「CLOSED 未合并」与「无关联 PR」的孤儿分支也计入退出码（默认只对 MERGED 残留判红）。

.PARAMETER Execute
    真正删除 MERGED 残留分支（`gh api -X DELETE repos/<nwo>/git/refs/heads/<branch>`）。
    不带此开关一律 dry-run。**永不**删默认分支，**永不**删 OPEN / CLOSED / 无 PR 的分支。
    删完只对**本次要删的那些**分支重新查 API 复核 —— 按设计不动的那几类本来就在远端，
    拿它们当判据会得到假红（2026-09-19 实测）。

.EXAMPLE
    pwsh scripts/check-branch-residue.ps1
    # 清点 + 判定 + 给出收口顺序；不改任何东西。

.EXAMPLE
    pwsh scripts/check-branch-residue.ps1 -Execute
    # 删掉所有 MERGED 残留分支，然后重新核对并打印结论。

.EXAMPLE
    pwsh scripts/check-branch-residue.ps1 -Branch chore/1137-gitattributes-uts-uvue-lf
    # 单分支核对（已清理的分支 ⇒ ABSENT、exit 0）。

.NOTES
    依据：2026-09-19 的 14 个残留分支归因（gh 2.97.0 源码 + GitHub timeline 的
    head_ref_deleted 事件对照取证）。上游设置项：仓库 Settings → Automatically delete head branches
    （`delete_branch_on_merge=true`）—— 打开后本脚本的 -Execute 就只是兜底。
    退出码：0 = 无 MERGED 残留；1 = 检出 MERGED 残留（或带 -IncludeClosed 时有待裁定项）；
            2 = 取不到仓库信息；3 = gh 不可用/未登录。
#>
[CmdletBinding()]
param(
    [string]$Branch,
    [switch]$IncludeClosed,
    [switch]$Execute
)

$ErrorActionPreference = 'Continue'

function Fail([string]$msg, [int]$code) {
    Write-Host "[branch-residue] $msg" -ForegroundColor Red
    exit $code
}

# gh 读写都套重试：本仓 2026-09-18 的 handoff 记过 TLS handshake timeout / EOF 随机出现。
function Invoke-Gh {
    param([string[]]$GhArgs, [int]$Retry = 3)
    $out = ''
    for ($i = 1; $i -le $Retry; $i++) {
        $out = & gh @GhArgs 2>&1
        if ($LASTEXITCODE -eq 0) { return @{ ok = $true; out = @($out) } }
        if ($i -lt $Retry) { Start-Sleep -Seconds 2 }
    }
    return @{ ok = $false; out = @($out) }
}

# ── 仓库根（同 new-worktree.ps1 / cleanup-dot-worktrees.ps1：从 git 公共目录取，不从脚本位置上溯）
$commonDir = (& git -C $PSScriptRoot rev-parse --git-common-dir 2>$null | Select-Object -First 1)
if (-not $commonDir) { Fail '取不到 --git-common-dir（不在 git 仓库里？）' 2 }
$commonDir = $commonDir.Trim()
if (-not [System.IO.Path]::IsPathRooted($commonDir)) { $commonDir = Join-Path $PSScriptRoot $commonDir }
$RepoRoot = Split-Path -Parent (Resolve-Path $commonDir).Path

$probe = & gh --version 2>&1
if ($LASTEXITCODE -ne 0) { Fail 'gh 不可用（未安装或不在 PATH）' 3 }

$nwo = (& gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>$null | Select-Object -First 1)
if (-not $nwo) { Fail 'gh repo view 失败：不在仓库里、或 gh 未登录' 3 }
$nwo = $nwo.Trim()

$default = (& gh repo view --json defaultBranchRef --jq '.defaultBranchRef.name' 2>$null | Select-Object -First 1)
if (-not $default) { Fail '取不到默认分支名' 2 }
$default = $default.Trim()

# 远端分支存在性：exists / absent / unknown。
# ⚠️ **不能**把「API 调用失败」当成「已删除」——那是假绿（判据本身会恒真）。
function Test-RemoteBranch([string]$b) {
    $r = Invoke-Gh @('api', "repos/$nwo/branches/$b")
    if ($r.ok) { return 'exists' }
    if (($r.out -join "`n") -match 'HTTP 404|Not Found') { return 'absent' }
    return 'unknown'
}

# ── 远端分支（事实源 = API；本地 ref 可能过期）
if ($Branch) {
    switch (Test-RemoteBranch $Branch) {
        'absent' {
            Write-Host "ABSENT  $Branch  —— 远端已无此分支（本判据的目标已达成）" -ForegroundColor Green
            Write-Host 'SUMMARY branches=0 mergedResidue=0 needHuman=0'
            exit 0
        }
        'unknown' { Fail "查远端分支失败（$Branch）：API 报错而非 404 ⇒ 不能当成已删除" 2 }
    }
    $branches = @($Branch)
} else {
    $r = Invoke-Gh @('api', "repos/$nwo/branches?per_page=100", '--paginate', '--jq', '.[].name')
    if (-not $r.ok) { Fail "取不到远端分支列表：$($r.out -join '; ')" 2 }
    $branches = @($r.out | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne $default })
}

# ── 本地事实：分支是否存在、被哪个 worktree 占着（只用于给出收口顺序）
$wtOfBranch = @{}
$curWt = $null
foreach ($line in (& git -C $RepoRoot worktree list --porcelain 2>$null)) {
    if ($line -like 'worktree *') { $curWt = $line.Substring(9).Trim() }
    elseif ($line -like 'branch refs/heads/*') { $wtOfBranch[$line.Substring(18).Trim()] = $curWt }
}
function Get-LocalFacts([string]$b) {
    & git -C $RepoRoot show-ref --verify --quiet "refs/heads/$b" 2>$null
    $hasLocal = ($LASTEXITCODE -eq 0)
    $wt = if ($wtOfBranch.ContainsKey($b)) { $wtOfBranch[$b] } else { '' }
    return @{ hasLocal = $hasLocal; wt = $wt }
}

Write-Host "=== 分支残留清点（$nwo，默认分支 $default）===" -ForegroundColor Cyan
Write-Host '判据：按关联 PR 的状态分类（不看 git 祖先关系 —— squash 会让它误判）。'
Write-Host ''

$residue = @()      # MERGED
$needHuman = @()    # CLOSED 未合并 / 无 PR
$active = @()       # OPEN

foreach ($b in $branches) {
    if ($b -eq $default) { continue }

    $pr = Invoke-Gh @('pr', 'list', '--repo', $nwo, '--head', $b, '--state', 'all', '--limit', '20',
                      '--json', 'number,state,mergedAt', '--jq', '.[] | "\(.number) \(.state) \(.mergedAt)"')
    if (-not $pr.ok) { Fail "查 PR 失败（$b）：$($pr.out -join '; ')" 2 }

    $rows = @()
    foreach ($line in $pr.out) {
        if ($line -match '^\s*(\d+)\s+(OPEN|CLOSED|MERGED)\s+(\S+)?\s*$') {
            $rows += [pscustomobject]@{ number = [int]$Matches[1]; state = $Matches[2]; mergedAt = $Matches[3] }
        }
    }

    $merged = @($rows | Where-Object { $_.state -eq 'MERGED' } | Sort-Object number -Descending)
    $open = @($rows | Where-Object { $_.state -eq 'OPEN' } | Sort-Object number -Descending)
    $closed = @($rows | Where-Object { $_.state -eq 'CLOSED' } | Sort-Object number -Descending)
    $facts = Get-LocalFacts $b
    $localNote = @()
    if ($facts.hasLocal) { $localNote += '本地分支=有' } else { $localNote += '本地分支=无' }
    if ($facts.wt) { $localNote += "worktree=$($facts.wt)" }
    $localNote = $localNote -join ' '

    if ($open.Count -gt 0) {
        $active += $b
        Write-Host ("OPEN      {0,-52} PR#{1}（活跃 PR，不动）  {2}" -f $b, $open[0].number, $localNote)
    } elseif ($merged.Count -gt 0) {
        $residue += [pscustomobject]@{ branch = $b; pr = $merged[0].number; mergedAt = $merged[0].mergedAt; wt = $facts.wt; hasLocal = $facts.hasLocal }
        Write-Host ("RESIDUE   {0,-52} PR#{1} merged {2}  {3}" -f $b, $merged[0].number, $merged[0].mergedAt, $localNote) -ForegroundColor Yellow
    } elseif ($closed.Count -gt 0) {
        $needHuman += [pscustomobject]@{ branch = $b; kind = 'CLOSED 未合并即关闭'; pr = $closed[0].number; wt = $facts.wt; hasLocal = $facts.hasLocal }
        Write-Host ("NEED-HUMAN {0,-51} PR#{1} CLOSED（未合并即关闭，需裁定）  {2}" -f $b, $closed[0].number, $localNote) -ForegroundColor Magenta
    } else {
        $needHuman += [pscustomobject]@{ branch = $b; kind = '无关联 PR（孤儿分支）'; pr = $null; wt = $facts.wt; hasLocal = $facts.hasLocal }
        Write-Host ("NEED-HUMAN {0,-51} 无关联 PR（孤儿分支，需裁定）  {1}" -f $b, $localNote) -ForegroundColor Magenta
    }
}

Write-Host ''
Write-Host ("SUMMARY branches={0} mergedResidue={1} needHuman={2} openPR={3}" -f $branches.Count, $residue.Count, $needHuman.Count, $active.Count)

if ($residue.Count -eq 0 -and $needHuman.Count -eq 0) {
    Write-Host '✅ 无 MERGED 残留分支。' -ForegroundColor Green
    exit 0
}

# ── 收口顺序：worktree → 本地分支 → 远端（顺序反了就会重演 ②：「本地删失败，远端删除跑不到」）
if ($residue.Count -gt 0) {
    Write-Host ''
    Write-Host '── MERGED 残留的收口顺序（顺序不可颠倒）──' -ForegroundColor Cyan
    foreach ($x in $residue) {
        Write-Host "  $($x.branch)  (PR #$($x.pr))"
        if ($x.wt) {
            Write-Host "    1) git -C `"$RepoRoot`" worktree remove `"$($x.wt)`""
            Write-Host "    2) git -C `"$RepoRoot`" branch -D $($x.branch)"
            Write-Host "    3) git -C `"$RepoRoot`" push origin --delete $($x.branch)   # 或本脚本 -Execute"
        } elseif ($x.hasLocal) {
            Write-Host "    1) git -C `"$RepoRoot`" branch -D $($x.branch)"
            Write-Host "    2) git -C `"$RepoRoot`" push origin --delete $($x.branch)   # 或本脚本 -Execute"
        } else {
            Write-Host "    git -C `"$RepoRoot`" push origin --delete $($x.branch)   # 或本脚本 -Execute"
        }
    }
}

if ($needHuman.Count -gt 0) {
    Write-Host ''
    Write-Host '── 需人裁定（脚本不会自动处理）──' -ForegroundColor Magenta
    foreach ($x in $needHuman) { Write-Host "  $($x.branch)  [$($x.kind)]" }
}

if (-not $Execute) {
    Write-Host ''
    Write-Host '（dry-run，未改任何东西）确认后用：' -ForegroundColor Yellow
    Write-Host '    pwsh scripts/check-branch-residue.ps1 -Execute' -ForegroundColor Yellow
    Write-Host '⚠️ 但真正的根治是上游设置：Settings → Automatically delete head branches'
    Write-Host '   （delete_branch_on_merge=true）—— 否则下次合并仍会复现同类残留。'
    if ($residue.Count -gt 0) { exit 1 }
    if ($IncludeClosed -and $needHuman.Count -gt 0) { exit 1 }
    exit 0
}

# ── -Execute：只删 MERGED 残留
Write-Host ''
Write-Host '=== 执行：删除 MERGED 残留分支 ===' -ForegroundColor Cyan
$toDelete = @($residue | ForEach-Object { $_.branch })
$failed = @()
foreach ($x in $residue) {
    if ($x.branch -eq $default) { Write-Host "SKIP  默认分支 $($x.branch)" -ForegroundColor Yellow; continue }
    $out = & gh api -X DELETE "repos/$nwo/git/refs/heads/$($x.branch)" 2>&1
    if ($LASTEXITCODE -eq 0) { Write-Host "OK    删除 $($x.branch)" -ForegroundColor Green }
    else { Write-Host "FAIL  删除 $($x.branch) :: $out" -ForegroundColor Red; $failed += $x.branch }
}

# ── 复核：**只核本次要删的那些分支**，不看命令回显（gh 会打印假的 Deleted remote branch）。
# ⚠️ 别拿「所有远端分支」去核：按设计不动的那几类（孤儿 / CLOSED 未合并 / 有活跃 PR）本来就在远端，
#    那样会被报成「仍在远端」⇒ **假红 + exit 1**（2026-09-19 实测：13 个全删成功，却因一条孤儿分支
#    打印 ❌ 并 exit 1，看上去像删除失败）。
Write-Host ''
Write-Host '── 复核（只核本次要删的分支；重新查 API，不信命令回显）──' -ForegroundColor Cyan
$stillThere = @()
$unknownBranches = @()
foreach ($b in $toDelete) {
    switch (Test-RemoteBranch $b) {
        'exists' { $stillThere += $b }
        'unknown' { $unknownBranches += $b }
    }
}
if ($stillThere.Count -eq 0 -and $unknownBranches.Count -eq 0) {
    if ($toDelete.Count -eq 0) {
        Write-Host "✅ 本次没有要删的 MERGED 残留（待裁定项 $($needHuman.Count) 个按设计未动）。" -ForegroundColor Green
    } else {
        Write-Host "✅ 本次要删的 $($toDelete.Count) 个已合并分支已不在远端；待裁定项 $($needHuman.Count) 个按设计未动。" -ForegroundColor Green
    }
    exit 0
}
if ($stillThere.Count -gt 0) { Write-Host "❌ 仍在远端：$($stillThere -join ', ')" -ForegroundColor Red }
if ($unknownBranches.Count -gt 0) { Write-Host "⚠️ 读不到（API 报错，未当成已删除）：$($unknownBranches -join ', ')" -ForegroundColor Yellow }
if ($failed.Count -gt 0) { Write-Host "   删除失败的：$($failed -join ', ')" -ForegroundColor Yellow }
exit 1
