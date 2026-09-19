#!/usr/bin/env pwsh
<#
.SYNOPSIS
    #1185 / #1187 的一次性收口清单：清理 5 个点目录 worktree + 收口命名纪律。

.DESCRIPTION
    这几步**必须由人在无会话占用时做** —— `git worktree move` / `remove` 在目录被别的进程
    持有句柄时一律 `Permission denied`（Windows 不允许改/删有打开句柄的目录）。agent 侧能做的
    已做完（闸门 new-worktree.ps1 已并入 #1185；2 个已改名）。

    本脚本把三件事做成**可机检**的：清点 → 执行 → 按验收标准判定。
    它**不会**为了删除去 kill 任何进程或别的会话；被句柄挡住的就报出来留给人。

.PARAMETER Execute
    真正执行删除与分支清理。**不带此开关时只做清点与可行性探测（dry-run）**，不改任何东西。

.PARAMETER Force
    连同未跟踪文件一起删。已核实这些未跟踪文件都是 `.tmp-*` 临时产物（PR 正文 / 取证中间件），
    但属**破坏性**动作：脚本会先打印它们，要求你带 -Force 明确确认。

.EXAMPLE
    pwsh scripts/cleanup-dot-worktrees.ps1
    # 看清单与探测结果；确认无误后再：
    pwsh scripts/cleanup-dot-worktrees.ps1 -Execute -Force

.NOTES
    依据：#1185（PR #1186）与 #1187 的方案；血账与机制见 #1144（PR #1150）。
    退出码：0 = 已达成验收（点目录 0 个）；1 = 仍有残留（或 dry-run 下有可执行项）；
            2 = 取不到仓库信息。
#>
[CmdletBinding()]
param(
    [switch]$Execute,
    [switch]$Force
)

$ErrorActionPreference = 'Continue'

# 判定表来自 #1187 的事实采集（全部 PR 已 MERGED、无未推独有工作）：
#   wt / 分支 / 独有提交数（供人工抽查用）
$targets = @(
    @{ wt = '.wt-1071';       branch = 'fix/1071-icon-glyph-restore';         ahead = 14; pr = '#1075' },
    @{ wt = '.wt-1087';       branch = 'fix/course-favorite-affordance';      ahead = 3;  pr = '#1114' },
    @{ wt = '.wt-1087b';      branch = 'fix/1087-cover-category';             ahead = 2;  pr = '#1129' },
    @{ wt = '.wt-1111';       branch = 'docs/0008-input-injection-correction'; ahead = 1;  pr = '#1127' },
    @{ wt = '.wt-ai-feature'; branch = $null;                                 ahead = 0;  pr = '(detached，停在 master 祖先)' }
)

# ── 仓库根（同 new-worktree.ps1：从 git 公共目录取，不从脚本位置上溯）
$commonDir = (& git -C $PSScriptRoot rev-parse --git-common-dir 2>$null | Select-Object -First 1)
if (-not $commonDir) { Write-Host '[cleanup] 取不到 --git-common-dir（不在 git 仓库里？）' -ForegroundColor Red; exit 2 }
$commonDir = $commonDir.Trim()
if (-not [System.IO.Path]::IsPathRooted($commonDir)) { $commonDir = Join-Path $PSScriptRoot $commonDir }
$RepoRoot = Split-Path -Parent (Resolve-Path $commonDir).Path

& git -C $RepoRoot fetch origin --prune 2>&1 | Out-Null

Write-Host "=== 1/3 清点（$RepoRoot）===" -ForegroundColor Cyan
$rows = foreach ($t in $targets) {
    $p = Join-Path $RepoRoot $t.wt
    if (-not (Test-Path $p)) {
        [pscustomobject]@{ wt = $t.wt; exists = $false; tracked = ''; untracked = ''; probe = '已不存在' }
        continue
    }
    $porc = & git -C $p status --porcelain 2>$null
    $tracked = ($porc | Where-Object { $_ -notmatch '^\?\?' } | Measure-Object).Count
    $untracked = ($porc | Where-Object { $_ -match '^\?\?' } | Measure-Object).Count

    # 可行性探测：**只读**。用 .NET 的目录改名试一下 —— 若目录树里有任何文件被其他进程持有句柄，
    # Windows 会抛 IOException（这正是 `git worktree move` 得到 Permission denied 的原因）。
    # ⚠️ 刻意**不**用 `git worktree move` 来探测：那会真的把目录移走，可能打断正在用它的会话。
    $probe = '可操作（无句柄占用）'
    $probeDir = Join-Path (Split-Path -Parent $p) ("__probe_" + [guid]::NewGuid().ToString('N').Substring(0, 8))
    try {
        [System.IO.Directory]::Move($p, $probeDir)
        [System.IO.Directory]::Move($probeDir, $p)   # 立刻移回，净效果为零
    } catch {
        $probe = '被句柄占用（需先关占用它的会话/进程）'
        if (Test-Path $probeDir) { try { [System.IO.Directory]::Move($probeDir, $p) } catch {} }
    }
    [pscustomobject]@{ wt = $t.wt; exists = $true; tracked = $tracked; untracked = $untracked; probe = $probe }
}
$rows | Format-Table -AutoSize | Out-String | Write-Host

Write-Host '=== 2/3 逐分支抽查（#1187 明文要求的「删前扫一眼」）===' -ForegroundColor Cyan
Write-Host '下面每个分支的独有提交都未推远端，删了就没了。确认它们只含已合并 PR 的内容：'
foreach ($t in $targets) {
    if (-not $t.branch) { Write-Host "  $($t.wt): (detached，无分支，跳过)"; continue }
    Write-Host "  ── $($t.wt)  $($t.branch)  PR $($t.pr)  [$($t.ahead) 个独有提交]"
    & git -C $RepoRoot log "origin/master..$($t.branch)" --oneline 2>$null | ForEach-Object { Write-Host "       $_" }
}

if (-not $Execute) {
    Write-Host ''
    Write-Host '=== 3/3（dry-run，未改任何东西）===' -ForegroundColor Yellow
    Write-Host '确认上面抽查无误后，执行：' -ForegroundColor Yellow
    Write-Host '    pwsh scripts/cleanup-dot-worktrees.ps1 -Execute -Force' -ForegroundColor Yellow
    Write-Host '（不带 -Force 时，含未跟踪文件的 worktree 会拒绝删除，只删干净的。）'
    exit 1
}

Write-Host ''
Write-Host '=== 3/3 执行 ===' -ForegroundColor Cyan
$blocked = @()
foreach ($t in $targets) {
    $p = Join-Path $RepoRoot $t.wt
    if (-not (Test-Path $p)) { Write-Host "SKIP  $($t.wt)（已不存在）"; continue }

    if (-not $Force) {
        $dirty = (& git -C $p status --porcelain 2>$null | Measure-Object).Count
        if ($dirty -gt 0) { Write-Host "SKIP  $($t.wt)（有 $dirty 项未提交，含未跟踪；需 -Force 明确确认）" -ForegroundColor Yellow; continue }
    }

    $out = & git -C $RepoRoot worktree remove $p --force 2>&1
    if ($LASTEXITCODE -eq 0) { Write-Host "OK    remove $($t.wt)" -ForegroundColor Green }
    else { Write-Host "FAIL  remove $($t.wt) :: $out" -ForegroundColor Red; $blocked += $t.wt }
}

Write-Host ''
Write-Host '── 清理已合并的本地分支 ──'
foreach ($t in $targets) {
    if (-not $t.branch) { continue }
    $exists = & git -C $RepoRoot branch --list $t.branch 2>$null
    if (-not $exists) { Write-Host "SKIP  分支 $($t.branch)（已不存在）"; continue }
    $out = & git -C $RepoRoot branch -D $t.branch 2>&1
    if ($LASTEXITCODE -eq 0) { Write-Host "OK    branch -D $($t.branch)" -ForegroundColor Green }
    else { Write-Host "FAIL  branch -D $($t.branch) :: $out" -ForegroundColor Red }
}

# ── 验收判定（#1185 验收标准 3）
Write-Host ''
Write-Host '── 验收判定 ──' -ForegroundColor Cyan
$dotDirs = & git -C $RepoRoot worktree list 2>$null | Where-Object { $_ -match '[\\/]\.' }
$n = ($dotDirs | Measure-Object).Count
if ($n -eq 0) {
    Write-Host '✅ 点开头目录数 = 0 ⇒ #1185 验收标准 3 满足，可据此收口 #1185 与本票。' -ForegroundColor Green
    exit 0
}
Write-Host "❌ 仍有 $n 个点开头目录：" -ForegroundColor Red
$dotDirs | ForEach-Object { Write-Host "     $_" }
if ($blocked.Count -gt 0) {
    Write-Host ''
    Write-Host "被句柄挡住的：$($blocked -join ', ')" -ForegroundColor Yellow
    Write-Host '处置：关掉占用它们的会话/进程后重跑本脚本；**不要**为此 kill 别人的会话。'
}
exit 1
