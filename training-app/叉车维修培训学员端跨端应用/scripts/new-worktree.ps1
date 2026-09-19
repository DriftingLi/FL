#!/usr/bin/env pwsh
<#
.SYNOPSIS
    建一个合规的会话 worktree，并当场用**实测判据**证明 jest 能看见测试。

.DESCRIPTION
    为什么需要这个入口（#1185 / 血账 #1144）：
      `docs/agents/multi-agent-git.md` 早已写明「会话 worktree 用 D:\FL\wt-<task>（无点），
      不要用 D:\FL\.wt-<task>」并给了判据，但新会话仍持续起 `.wt-<票号>` —— 因为命名是从
      **在场目录样例**照抄的（#1185 实测当时仍有 5 个点目录 worktree 改不动）。文档纠不动这个，闸门可以。

    本脚本的判据只有一条，且是**实测**的：
      建完立刻在目标里跑 `jest --listTests`，**必须真的列出套件**（计数 > 0）。
      点目录下该命令匹配 0 个套件（实测 `.wt-1071` = 0 个套件，合法名 = 82）。

      为什么不靠「校验目录名」：
        #1144 的机制是段首为 `.` 或 `{}()+?.^$` 之一才会踩坑（jest-util 的
        `replacePathSepForGlob` 故意不转换该处反斜杠）。默认命名 `wt-<task>` 的段首恒为 `w`，
        **不可能**触发 —— 实测 `wt-(wip)1185` / `wt-{wip}1185` 的 `--listTests` 分别是
        **104 / 104** 个套件，都正常。所以「枚举坏名字」在默认命名下是**恒不触发的死代码**，
        而**建完实测**对所有命名方案都成立。判据取后者。

.PARAMETER Task
    票号或任务名，用于生成目录名与分支名（例：1185 → 目录 wt-1185、分支 feat/1185）。

.PARAMETER Base
    起点 ref，默认 origin/master。

.PARAMETER Branch
    覆盖分支名（默认 feat/<Task>）。传 docs/<Task>、fix/<Task> 等时用。

.PARAMETER NoBranch
    只建 detached worktree，不建分支（临时复现用）。

.PARAMETER SkipVerify
    跳过建完后的 jest 判据（仅当你**明确**知道不需要单测面时用；默认不跳）。

.EXAMPLE
    pwsh scripts/new-worktree.ps1 -Task 1185
    pwsh scripts/new-worktree.ps1 -Task 1185 -Branch fix/1185-xxx

.NOTES
    退出码：0 = 建成且判据通过；1 = 参数非法；2 = 建失败；3 = 判据计数为 0（踩坑，须改名）。
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Task,
    [string]$Base = 'origin/master',
    [string]$Branch,
    [switch]$NoBranch,
    [switch]$SkipVerify
)

$ErrorActionPreference = 'Stop'

function Fail([string]$msg, [int]$code) {
    Write-Host "[new-worktree] $msg" -ForegroundColor Red
    exit $code
}

# ── 参数校验（只拦**确实**会出问题的）
# 分隔符会让目录名多出一段，那一段的段首就不可控了 ⇒ 必须拦。
if ($Task -match '[\\/]') { Fail "-Task 不得含路径分隔符：$Task" 1 }
if ([string]::IsNullOrWhiteSpace($Task)) { Fail "-Task 不能为空" 1 }

# ── 仓库根与主工作树的 mobile 目录
# ⚠️ 不能从 $PSScriptRoot 上溯：本脚本既可能在主树、也可能在某个 worktree 里被调用，
#    上溯会得到**那个 worktree**，于是新 worktree 被嵌进另一个 worktree 里（实测踩过：
#    建出了 D:\FL\wt-1185\wt-(wip)1185）。改用 git 的公共目录 —— 对所有 worktree 指向同一处。
$commonDir = (& git -C $PSScriptRoot rev-parse --git-common-dir 2>$null | Select-Object -First 1)
if (-not $commonDir) { Fail "取不到 --git-common-dir（不在 git 仓库里？）" 2 }
$commonDir = $commonDir.Trim()
if (-not [System.IO.Path]::IsPathRooted($commonDir)) { $commonDir = Join-Path $PSScriptRoot $commonDir }
$RepoRoot = Split-Path -Parent (Resolve-Path $commonDir).Path
$MobileRoot = Join-Path $RepoRoot (Join-Path 'training-app' '叉车维修培训学员端跨端应用')

$name = "wt-$Task"
$dest = Join-Path $RepoRoot $name
if (Test-Path $dest) { Fail "目标已存在：$dest" 2 }

# ── 建
& git -C $RepoRoot fetch origin --prune 2>&1 | Out-Null
$gitArgs = @('-C', $RepoRoot, 'worktree', 'add')
if ($NoBranch) {
    $gitArgs += @($dest, $Base)
} else {
    if (-not $Branch) { $Branch = "feat/$Task" }
    $gitArgs += @('-b', $Branch, $dest, $Base)
}
& git @gitArgs
if ($LASTEXITCODE -ne 0) { Fail "git worktree add 失败（exit $LASTEXITCODE）" 2 }
Write-Host "[new-worktree] 已建 $dest（分支 $Branch）" -ForegroundColor Green

# ── 唯一判据：实测 jest 能否看见套件
if ($SkipVerify) { Write-Host "[new-worktree] 已按 -SkipVerify 跳过判据"; exit 0 }

$proj = Join-Path $dest (Join-Path 'training-app' '叉车维修培训学员端跨端应用')
if (-not (Test-Path (Join-Path $proj 'jest.config.unit.js'))) {
    Write-Host "[new-worktree] $proj 下没有 jest.config.unit.js，跳过判据。" -ForegroundColor Yellow
    exit 0
}
$jest = Join-Path $MobileRoot 'node_modules\jest\bin\jest.js'
if (-not (Test-Path $jest)) {
    Write-Host "[new-worktree] 主树缺 jest（$jest），无法跑判据。先在主树 npm ci。" -ForegroundColor Yellow
    exit 0
}

Push-Location $proj
try {
    $out = & node $jest --config jest.config.unit.js -i --listTests 2>&1
    # 只数「看起来像测试路径」的行：点目录下 stderr 的报错行会被算成 1（实测），不能拿它当计数
    $count = ($out | Where-Object { $_ -match '\.test\.[jt]s\s*$' } | Measure-Object).Count
} finally { Pop-Location }

if ($count -eq 0) {
    Write-Host "[new-worktree] ❌ 判据失败：--listTests 列出 0 个套件 ⇒ 在这个 worktree 里跑 ③ 会假绿。" -ForegroundColor Red
    Write-Host "              目录名：$dest"
    Write-Host "              修法：git worktree move `"$dest`" `"<换一个不含点/元字符前缀的名字>`""
    exit 3
}
Write-Host "[new-worktree] ✅ 判据通过：--listTests 列出 $count 个套件。" -ForegroundColor Green
exit 0
