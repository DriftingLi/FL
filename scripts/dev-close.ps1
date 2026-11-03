#!/usr/bin/env pwsh
<#
.SYNOPSIS
    一条命令收完「第 8 步」：拆树 + 分支残留核对 + 一行机检读数（#2174）。

.DESCRIPTION
    薄封装，不是新机制。拆树的全部判据仍在 remove-worktree.ps1，远端分支残留的全部判据仍在
    check-branch-residue.ps1 —— 本文件只把它们按根 docs/agents/release.md 第 7 步的顺序串起来，
    并把**决定**（准不准拆 / 要不要透传 -Execute）交给同目录 lib/dev-close.mjs 判：那里能被
    node --test 真执行地测，而本文件里的分支逻辑 CI 测不到（CI 全线 ubuntu），所以刻意不在这里留判据。

    树身份（#2247）：认树 = 注册表里路径名为 wt-<Task> 的条目，身份 = 该条目实际登记的分支。
    本文件**不再自己拼 feat/<Task>**——那个写死曾是血账：树 wt-2230 挂在 fix/2230-dead-subtab、
    wt-2189 挂在 fix/2189-lint-brace-expansion，旧判据一律判「无树」而静默跳过全部动作。
    现在第 1 步先问判定层 `--resolve` 要身份分支，PR 头查询／未合并对账／拆树／分支残留核对
    四处全部改用它。

    并发边界（本票第一性约束）：只动 -Task 自己那一棵树。默认不 checkout、不 pull、不改主树分支状态 ——
    主树是多会话共享面（manifest.json / postcss.config.js 常年挂着别的会话的脏）。`git fetch --prune`
    只动远端跟踪引用、不碰工作区，属安全对账，默认就做；ff-only 同步改的是主树工作区，是树的主人
    自己的动作，要它请加 -SyncMaster（且仅在 -Yes 时生效）。

    编码：入口钉 [Console]::OutputEncoding 为 UTF-8（同 preflight-pr.ps1 入口那条），并把 $OutputEncoding 设成**无 BOM** 的
    UTF8Encoding —— 用 [Text.Encoding]::UTF8 会给子进程 stdin 冠一个 U+FEFF，JSON.parse 当场炸，
    整条判定链读不回 facts 而恒红（#2174 实测：Unexpected token '\uFEFF'）。真跑取证见票评论。
    若某条带非 ASCII 的路径在 PS→node 管道里受损，后果是**判红不是判绿**：判定层读不回就标 unknown，
    unknown ⇒ verdict=red，DECIDE 行读不回 allow=true 也直接不准拆（本链不接受 fail-open；#2161 同族）。

.PARAMETER Task
    票号（纯数字）。认树入口 = <主树>/wt-<Task>；树身份 = 该条目登记的分支（由判定层解析，不由本文件假设）。

.PARAMETER Branch
    可选的覆盖：声明「我以为这棵树的身份分支是 X」。与登记分支不一致时**不猜、不二选一**，
    判定层直接判红 reason=branch-mismatch —— 错配的收尾比没做更坏。

.PARAMETER Yes
    真正执行；不带时全程 dry-run（-Execute 一律不透传）。

.PARAMETER SyncMaster
    仅在 -Yes 时生效：在主树追加一次 `merge --ff-only origin/master`（不做 checkout，理由见 .DESCRIPTION）。

#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)][string]$Task,
    [string]$Branch,
    [switch]$Yes,
    [switch]$SyncMaster
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

if (-not $IsWindows) { Write-Host '[dev-close] 仅支持 Windows（拆的是 junction / worktree 目录）' -ForegroundColor Red; exit 1 }
if ($Task -notmatch '^[0-9]+$') { Write-Host '[dev-close] -Task 只接票号（纯数字）' -ForegroundColor Red; exit 2 }

# 主树根问 git（--git-common-dir），不从 $PSScriptRoot 上溯：后者得到的是**调用方所在的那棵树**，
# 于是 `wt-<Task>` 目录会对着 worktree 自己找、-SyncMaster 也会合错树（同 new-worktree.ps1 的血账）。
# 判据不在这里重写一遍 —— dot-source 移动端 lib/wt-bootstrap.ps1 的 Get-WtMainRoot（唯一实现）。
# 定位同样按枚举、不写死中文工程目录名（理由见下方 $mobileScripts 那段）。
$wtBootstrap = Get-ChildItem -LiteralPath (Join-Path (Split-Path -Parent $PSScriptRoot) 'training-app') -Directory -ErrorAction SilentlyContinue |
    ForEach-Object { Join-Path $_.FullName 'scripts\lib\wt-bootstrap.ps1' } |
    Where-Object { Test-Path -LiteralPath $_ } |
    Select-Object -First 1
$root = $null
if ($wtBootstrap) {
    . $wtBootstrap
    $root = Get-WtMainRoot -Base $PSScriptRoot
}
if (-not $root) { Write-Host '[dev-close] 反推主树失败（git rev-parse --git-common-dir 无输出）' -ForegroundColor Red; exit 2 }
$core = Join-Path $PSScriptRoot 'lib\dev-close.mjs'
if (-not (Test-Path -LiteralPath $core)) { Write-Host "[dev-close] 判定层缺失：$core" -ForegroundColor Red; exit 2 }

# 移动端 scripts 目录按枚举定位，不写死中文目录名（写死=第二份口径，#1845 那类两口径并存的教训）
$mobileScripts = Get-ChildItem -LiteralPath (Join-Path $root 'training-app') -Directory -ErrorAction SilentlyContinue |
    ForEach-Object { Join-Path $_.FullName 'scripts' } |
    Where-Object { Test-Path -LiteralPath (Join-Path $_ 'remove-worktree.ps1') } |
    Select-Object -First 1
if (-not $mobileScripts) { Write-Host '[dev-close] 找不到 remove-worktree.ps1，无法拆树' -ForegroundColor Red; exit 2 }

function Invoke-Core([string]$mode, $factsObj) {
    $json = $factsObj | ConvertTo-Json -Compress -Depth 6
    $out = $json | & node $core $mode 2>&1 | Out-String
    return [pscustomobject]@{ Text = ('' + $out).Trim(); Code = $LASTEXITCODE }
}
function FactOf([string]$text, [string]$key) {
    $m = [regex]::Match($text, ('\b' + $key + '=([^ ]+)'))
    if (-not $m.Success) { return '' }
    return $m.Groups[1].Value
}

# ---------- 1) 采集事实（只采集，不判断）----------
$wtList = '' + (& git -C $root worktree list --porcelain 2>&1 | Out-String)
$unmerged = ''

# 认树与树身份都问判定层（#2247）：本文件不再自己扫注册表、也不再自己拼 feat/<Task>。
# 两处写死的分支名（认树那一处与 PR 头查询那一处）就是 wt-2230 / wt-2189 每次收尾必漏的根因。
# 目录是否存在只采一次、两处复用同一个事实（同一件事在两个地方各测一遍就是第二份口径）。
$taskDirExists = [bool](Test-Path -LiteralPath (Join-Path $root ("wt-" + $Task)))
$rq = Invoke-Core '--resolve' ([ordered]@{
    task = $Task; worktreeList = $wtList; branch = $Branch; treeDirExists = $taskDirExists
})
Write-Host $rq.Text
$treeState = FactOf $rq.Text 'state'
$treePathHint = FactOf $rq.Text 'path'
if ($treePathHint -eq 'na') { $treePathHint = '' }
# 身份分支只取登记分支：没认出树就没有身份（行内 branch=na），别拿探测分支冒充（#2247）。
$identBranch = FactOf $rq.Text 'branch'
if ($identBranch -eq 'na') { $identBranch = '' }
# 探测分支只喂采集（PR 头 / 未合并对账 / 残留核对 / 本地分支），不参与认树、也不进 branch= 字段。
$probeBranch = $identBranch
if (-not $probeBranch) { $probeBranch = FactOf $rq.Text 'probe_branch' }
if ($probeBranch -eq 'na') { $probeBranch = '' }

# squash 合并仓里，功能分支的提交**永远不是** master 的祖先（GitHub squash 会造一个新提交），
# 所以「有没有还没合的活」不能问 origin/master，只能问 PR 自己的 head sha。
Push-Location $root
try {
    if ($probeBranch) {
        $prJsonText = '' + (& gh pr list --state all --head $probeBranch --json state,headRefOid 2>&1 | Out-String)
    } else {
        $prJsonText = ''
    }
} finally { Pop-Location }
$prHead = ''
$mh = [regex]::Match($prJsonText, '"headRefOid"\s*:\s*"([0-9a-fA-F]{7,40})"')
if ($mh.Success) { $prHead = $mh.Groups[1].Value }
if ($treePathHint -and (Test-Path -LiteralPath $treePathHint)) {
    if ($prHead) {
        # 只问「PR 头之后还有没有提交」：合并后被本地补提交的活，就是没进 PR 的活
        $unmerged = '' + (& git -C $treePathHint log --oneline "$prHead..HEAD" 2>&1 | Out-String)
    } else {
        # 读不到 PR 头就不许拆（fail-closed）：拿不到比对基准时，「看起来没活」不是放行理由
        $unmerged = 'pr-head-unreadable'
    }
}

$dec = Invoke-Core '--decide' ([ordered]@{
    task = $Task; execute = [bool]$Yes; cwd = ('' + (Get-Location).Path)
    worktreeList = $wtList; unmergedLog = $unmerged; prJson = $prJsonText; branch = $Branch
    treeDirExists = $taskDirExists
})
Write-Host $dec.Text
$reason = FactOf $dec.Text 'reason'
if ((FactOf $dec.Text 'allow') -ne 'true') {
    $v = Invoke-Core '--verdict' ([ordered]@{
        task = $Task; prState = (FactOf $dec.Text 'pr'); mode = $(if ($Yes) { 'execute' } else { 'dryrun' }); fetch = 'skipped'
        treeBefore = 'present'; blockedReason = $(if ($reason) { $reason } else { 'decide-unreadable' })
        branch = $identBranch; legacy = ($treeState -eq 'legacy')
        remove = [ordered]@{ ran = $false }; residue = [ordered]@{ ran = $false; state = 'na' }
    })
    Write-Host $v.Text
    exit 1
}

# ---------- 2) 对账（fetch 安全：只动远端跟踪引用）----------
$fetchState = 'ok'
$fr = & git -C $root fetch --prune 2>&1 | Out-String
if ($LASTEXITCODE -ne 0) { $fetchState = 'failed'; Write-Host "[dev-close] fetch --prune 失败：$fr" -ForegroundColor Yellow }
if ($Yes -and $SyncMaster) {
    $mr = & git -C $root merge --ff-only origin/master 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { Write-Host "[dev-close] ff-only 同步未成（主树可能正被别人用着）：$mr" -ForegroundColor Yellow }
}

# ---------- 3) 按判定行动：拆树 → 分支残留核对 ----------
$passExec = (FactOf $dec.Text 'pass_execute') -eq 'true'
$skipRemove = (FactOf $dec.Text 'skip_remove') -eq 'true'
$treeBefore = $(if ($skipRemove) { 'absent' } else { 'present' })
$removeText = ''; $removeCode = 0
if (-not $skipRemove) {
    # 拆树调用带身份分支（#2247）：remove-worktree.ps1 的 -Branch 决定它核哪条分支的合并状态与删除尾部，
    # 目录仍由 -Task 唯一定（remove-worktree.ps1 里 $name = "wt-$Task"），所以 fix/* 形态的树也能被正确拆掉 —— 判据内部不动。
    $argvR = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $mobileScripts 'remove-worktree.ps1'), '-Task', $Task)
    if ($identBranch) { $argvR += @('-Branch', $identBranch) }
    if ($passExec) { $argvR += '-Execute' }
    $removeText = '' + (& pwsh @argvR 2>&1 | Out-String)
    $removeCode = $LASTEXITCODE
    Write-Host $removeText.Trim()
}
# 分支残留核对同样按身份分支（#2247）：残留永远是「这条分支还留着没」，不是「feat/<票号> 还留着没」。
# 没有身份分支时只做全量扫描（residue 只认 -Branch，不接受 -Task），绝不退回写死的 feat/<票号>。
$argvC = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $mobileScripts 'check-branch-residue.ps1'))
if ($identBranch) { $argvC += @('-Branch', $identBranch) }
if ($passExec) { $argvC += '-Execute' }
$residueText = '' + (& pwsh @argvC 2>&1 | Out-String)
$residueCode = $LASTEXITCODE
Write-Host $residueText.Trim()

# ---------- 4) 读数：原始产物交回判定层，这里不解析 ----------
# local_branch 必须与 `git branch --list` 一致（#2188 验收第 3 条）：remove 跑完后现采事实，交判定层兜底。
# 采集放这里不放别处——此刻分支若还在，就是「真没删掉」，早采晚采都会把中间态读成假话。
# 核的仍是身份分支（#2247）：问 feat/<票号> 会恒答 absent，把「没删掉的 fix/* 分支」谎报成已清干净。
$lbText = ''
if ($identBranch) { $lbText = '' + (& git -C $root branch --list "$identBranch" 2>&1 | Out-String) }
$v = Invoke-Core '--verdict' ([ordered]@{
    task = $Task; prState = (FactOf $dec.Text 'pr'); mode = $(if ($Yes) { 'execute' } else { 'dryrun' })
    fetch = $fetchState; treeBefore = $treeBefore
    localBranch = $(if ($lbText.Trim()) { 'kept' } else { 'absent' })
    branch = $identBranch; legacy = ($treeState -eq 'legacy')
    remove = [ordered]@{ ran = [bool](-not $skipRemove); exitCode = $removeCode; raw = $removeText }
    residue = [ordered]@{ ran = $true; exitCode = $residueCode; raw = $residueText }
})
Write-Host $v.Text
exit $v.Code