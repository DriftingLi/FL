#!/usr/bin/env pwsh
<#
.SYNOPSIS
    删除一张票的本地 worktree：先优雅收口 HBuilderX / 微信开发者工具，再判定能不能删，绝不 kill 进程。

.DESCRIPTION
    `git worktree remove` 在目录被别的进程持有句柄时一律失败（Windows 不允许删有打开句柄的目录）。
    移动端最常见的持有者是 HBuilderX 的语言服务 / 文件监视子进程，以及微信开发者工具的常驻会话。
    本脚本把「删之前该做什么」收成一个可机检的顺序：

        预检（注册状态 / junction / 持有者）
          → 优雅收口（cli project close / cli.bat quit，只在工具确实在跑时）
          → 拆 node_modules junction（移动端 worktree 的依赖是主树的链接）
          → git worktree remove --force
          → 目录兜底删除（git 元数据已 prune 的残留树）
          → 校验目录 / worktree 列表 / 分支
          → 仍被锁就报出持有者，非零退出，留给人处置

    默认是 dry-run：只打印计划与持有者，不改任何东西；确认后加 -Execute。

.PARAMETER Task
    票号或任务名，与 new-worktree.ps1 -Task 对应。目标目录 = <主树>/wt-<Task>。

.PARAMETER Branch
    分支名覆盖；缺省 feat/<Task>。

.PARAMETER HxCli
    HBuilderX cli.exe 路径。缺省从正在运行的 HBuilderX 同目录取，取不到就跳过「优雅收口」。
    显式给出即权威：路径不可用时不回退探测。

.PARAMETER DevToolsCli
    微信开发者工具 cli.bat 路径。缺省从正在运行的 wechatdevtools 同目录取；取不到就跳过。

.PARAMETER Execute
    真正执行删除；不带时只做 dry-run。

.PARAMETER KeepBranch
    只删工作树，保留本地分支。

.PARAMETER Force
    分支上有未合并进 origin/master 的提交时，仍允许删除该分支。破坏性动作，需显式确认。

.EXAMPLE
    pwsh scripts/remove-worktree.ps1 -Task 1727
    # 看计划与持有者；确认后：
    pwsh scripts/remove-worktree.ps1 -Task 1727 -Execute

.NOTES
    退出码：0 = 成功（或 dry-run 只读完成）；1 = 参数非法；2 = 取不到仓库根；
            3 = 执行失败 / 仍有残留；4 = 分支有未合并提交且未带 -Force（拒绝破坏性删除）；
            5 = 调用方 shell 的 CWD 在目标树内（先 cd 到树外再执行）。
    机制与血账：docs/agents/multi-agent-git.md「worktree 收尾」段；#1144 / #1727。
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Task,
    [string]$Branch,
    [string]$HxCli,
    [string]$DevToolsCli,
    [switch]$Execute,
    [switch]$KeepBranch,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

# junction / cmd rmdir / HBuilderX cli 都是 Windows 语义；POSIX 上直接说清楚，不静默降级。
if (-not $IsWindows) { Write-Host '[remove-worktree] 仅支持 Windows（junction / rmdir / HBuilderX cli）' -ForegroundColor Red; exit 1 }

function Fail([string]$msg, [int]$code) {
    Write-Host "[remove-worktree] $msg" -ForegroundColor Red
    exit $code
}

# 仓库根与 new-worktree.ps1 同源：git 的公共目录对所有 worktree 指向同一处，
# 从脚本位置上溯会得到调用方所在的那棵树（血账见 new-worktree.ps1「仓库根与主工作树」）。
. (Join-Path $PSScriptRoot 'lib\wt-bootstrap.ps1')

if ([string]::IsNullOrWhiteSpace($Task)) { Fail '-Task 不能为空' 1 }
if ($Task -match '[\\/]') { Fail "-Task 不得含路径分隔符：$Task" 1 }

$RepoRoot = Get-WtMainRoot -Base $PSScriptRoot
if (-not $RepoRoot) { Fail '取不到主树根（--git-common-dir 失败，不在 git 仓库里？）' 2 }

$name = "wt-$Task"
$dest = Join-Path $RepoRoot $name
if (-not $Branch) { $Branch = "feat/$Task" }

# ── 预检：注册状态、目录、移动端依赖 junction、已知持有者
# git 在 Windows 上把 worktree 路径打成正斜杠（`worktree D:/FL/wt-x`）⇒ 比较前必须归一化。
function Get-NormPath([string]$p) {
    try { return [System.IO.Path]::GetFullPath($p).TrimEnd([char[]]@('\', '/')) } catch { return $p }
}
$destNorm = Get-NormPath $dest

# 调用方 shell 的 CWD 若在树内，会以「进程当前目录」的形式占着目录（Windows 语义），删不掉。
# 本进程随后切到主树只是放掉自己这一份；调用方仍须先 cd 出去。
$cwdNorm = Get-NormPath (Get-Location).Path
$cwdInside = ($cwdNorm -eq $destNorm) -or $cwdNorm.StartsWith($destNorm + '\', [System.StringComparison]::OrdinalIgnoreCase)
if ($cwdInside) {
    Write-Host "[remove-worktree] 当前目录在目标树内（$cwdNorm）。先 cd 到树外再执行；调用方 shell 的 CWD 会挡住删除。" -ForegroundColor Yellow
    if ($Execute) { exit 5 }
}
Set-Location -LiteralPath $RepoRoot

$porcelain = & git -C $RepoRoot worktree list --porcelain 2>$null
$registered = @($porcelain | Where-Object { $_ -like 'worktree *' } | ForEach-Object { Get-NormPath ($_.Substring(9).Trim()) }) -contains $destNorm
$exists = Test-Path -LiteralPath $dest

$mobileSub = Join-Path 'training-app' '叉车维修培训学员端跨端应用'
$mobileDir = Join-Path $dest $mobileSub
$nodeModules = Join-Path $mobileDir 'node_modules'
$isJunction = $false
if (Test-Path -LiteralPath $nodeModules) {
    $item = Get-Item -LiteralPath $nodeModules -Force -ErrorAction SilentlyContinue
    $isJunction = [bool]($item -and (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0))
}

# 未提交改动默认拒绝（-Force 才删）：worktree remove --force 会连同它们一起丢。
$dirty = $false
if ($exists -and (Test-Path -LiteralPath (Join-Path $dest '.git'))) {
    $dirty = @(& git -C $dest status --porcelain 2>$null).Count -gt 0
}

# 持有者：HBuilderX / 微信开发者工具只要在跑就列为「可能持有」——它们的语言服务 / 文件监视
# 子进程命令行里往往不带项目路径，靠命令行匹配会漏报。node / pwsh 只在命令行提到本目录时才算。
$holders = New-Object System.Collections.Generic.List[object]
$procs = Get-CimInstance Win32_Process -Filter "Name='HBuilderX.exe' OR Name='wechatdevtools.exe' OR Name='wxfilewatcher_x64.exe' OR Name='node.exe' OR Name='pwsh.exe'" -ErrorAction SilentlyContinue
foreach ($p in @($procs)) {
    if ($p.ProcessId -eq $PID) { continue }
    $cmd = [string]$p.CommandLine
    $isTool = $p.Name -in @('HBuilderX.exe', 'wechatdevtools.exe', 'wxfilewatcher_x64.exe')
    $mentions = $cmd -and $cmd.IndexOf($dest, [StringComparison]::OrdinalIgnoreCase) -ge 0
    if (-not ($isTool -or $mentions)) { continue }
    $reason = if ($isTool) { '工具常驻（可能持有项目目录）' } else { '命令行提到本 worktree' }
    $holders.Add([ordered]@{
            pid    = $p.ProcessId
            name   = $p.Name
            path   = [string]$p.ExecutablePath
            reason = $reason
        })
}

# 优雅收口路径：优先用正在运行的进程旁的同名 CLI，与主程序版本必然匹配。
function Resolve-ToolCli([string]$explicit, [string]$processName, [string]$cliName) {
    if ($explicit) { return $explicit }
    $proc = Get-Process -Name $processName -ErrorAction SilentlyContinue | Where-Object { $_.Path } | Select-Object -First 1
    if ($proc) { return (Join-Path (Split-Path -Parent $proc.Path) $cliName) }
    return $null
}
$hxCliPath = Resolve-ToolCli $HxCli 'HBuilderX' 'cli.exe'
$wxCliPath = Resolve-ToolCli $DevToolsCli 'wechatdevtools' 'cli.bat'

$branchExists = @(& git -C $RepoRoot branch --list $Branch 2>$null).Count -gt 0
$branchMerged = $false
if ($branchExists) {
    $branchMerged = @(& git -C $RepoRoot branch --merged origin/master --list $Branch 2>$null).Count -gt 0
    if (-not $branchMerged) {
        # squash 合并后分支与 master 不再有祖先关系；单提交分支用 patch-id（git cherry）兜底：
        # 输出里没有 `+` 行 ⇒ 每个提交都在 upstream 有等价补丁。
        $cherry = @(& git -C $RepoRoot cherry origin/master $Branch 2>$null)
        $branchMerged = ($cherry.Count -gt 0) -and (@($cherry | Where-Object { -not $_.StartsWith('-') }).Count -eq 0)
    }
}

$status = if (-not $exists -and -not $registered) { 'not-found' }
elseif (-not $exists -and $registered) { 'stale-registration' }
else { 'ready' }

$actions = New-Object System.Collections.Generic.List[string]
if ($status -ne 'not-found') {
    if ($hxCliPath -and (Test-Path -LiteralPath $mobileDir)) { $actions.Add("hx-close: $hxCliPath project close --path $mobileDir") }
    if ($wxCliPath) { $actions.Add("wx-close: $wxCliPath quit") }
    if ($isJunction) { $actions.Add('remove-junction: node_modules') }
    if ($registered) { $actions.Add("git-worktree-remove: $dest") }
    if ($exists) { $actions.Add("fallback-rmdir: $dest") }
    if ($branchExists -and -not $KeepBranch) { $actions.Add("branch-delete: $Branch (merged=$branchMerged)") }
}

$plan = [ordered]@{
    task            = $Task
    worktree        = $dest
    branch          = $Branch
    exists          = $exists
    registered      = $registered
    nodeModulesLink = $isJunction
    branchExists    = $branchExists
    branchMerged    = $branchMerged
    dirty           = $dirty
    # 直接放 List：ConvertTo-Json 会序列化成数组；@($list) 在 [ordered] 字面量里会抛
    # "Argument types do not match"（PowerShell 7 实测），故不套数组子表达式。
    holders         = $holders
    status          = $status
    actions         = $actions
}
$plan | ConvertTo-Json -Depth 6

if (-not $Execute) {
    Write-Host '[remove-worktree] dry-run：未改任何东西。确认后加 -Execute。' -ForegroundColor Yellow
    exit 0
}

if ($status -eq 'not-found') {
    Write-Host '[remove-worktree] 目录与注册都不存在，无需清理。' -ForegroundColor Green
    exit 0
}

# worktree remove --force 会连未提交改动一起丢；清理入口不替人做这个决定。
if ($dirty) { Fail "worktree 有未提交改动（$dest）：先提交或 stash 再来，脚本不替你丢改动" 4 }

# ── 1. 优雅收口（只在工具确实在跑时；不做 Stop-Process）
if ($hxCliPath -and (Test-Path -LiteralPath $hxCliPath) -and (Test-Path -LiteralPath $mobileDir)) {
    Write-Host "[remove-worktree] HBuilderX project close …"
    & $hxCliPath project close --path $mobileDir 2>&1 | Out-Host
}
if ($wxCliPath -and (Test-Path -LiteralPath $wxCliPath)) {
    Write-Host '[remove-worktree] 微信开发者工具 quit …'
    & $wxCliPath quit 2>&1 | Out-Host
}

# ── 2. 先拆 junction：git worktree remove 遇到指向主树的链接时，顺序不对会伤到目标
if ($isJunction) {
    $out = cmd /c rmdir "$nodeModules" 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Host "[remove-worktree] junction 删除失败：$out" -ForegroundColor Yellow }
}

# ── 3. git 注销 + 删目录
if ($registered) {
    $out = & git -C $RepoRoot worktree remove $dest --force 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[remove-worktree] git worktree remove 失败：$out" -ForegroundColor Yellow
    }
}

# 兜底：git 元数据已 prune 的残留树（目录在、注册不在）或 remove 只删了元数据的情况。
if (Test-Path -LiteralPath $dest) {
    $out = cmd /c rmdir /s /q "$dest" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[remove-worktree] 目录删除失败：$out" -ForegroundColor Red
    }
}
& git -C $RepoRoot worktree prune 2>&1 | Out-Null

# ── 4. 分支：有未合并提交时拒绝（-Force 才放行），避免把没进主线的活删掉
$branchRemoved = $false
if ($branchExists -and -not $KeepBranch) {
    if (-not $branchMerged -and -not $Force) {
        Write-Host "[remove-worktree] 分支 $Branch 有未合并进 origin/master 的提交；保留。确需删除请加 -Force。" -ForegroundColor Yellow
    } else {
        & git -C $RepoRoot branch -D $Branch 2>&1 | Out-Host
        $branchRemoved = $LASTEXITCODE -eq 0
    }
}

# ── 5. 校验：目录消失、worktree 列表不再提到它、分支按请求处理
$leftDir = Test-Path -LiteralPath $dest
$leftReg = @(& git -C $RepoRoot worktree list --porcelain 2>$null | Where-Object { $_ -like 'worktree *' } | ForEach-Object { Get-NormPath ($_.Substring(9).Trim()) }) -contains $destNorm

$result = [ordered]@{
    task          = $Task
    worktree      = $dest
    dirRemoved    = -not $leftDir
    regRemoved    = -not $leftReg
    branchRemoved = $branchRemoved
}
$result | ConvertTo-Json -Depth 4

if ($leftDir -or $leftReg) {
    Write-Host '[remove-worktree] 仍有残留。当前持有者：' -ForegroundColor Red
    $holders | Format-Table -AutoSize | Out-String | Write-Host
    Write-Host '[remove-worktree] 处置：关掉占用它的 HBuilderX / 微信开发者工具 / 会话后重跑；不要为此 kill 别人的会话。' -ForegroundColor Yellow
    exit 3
}
Write-Host '[remove-worktree] ✅ 已清理。' -ForegroundColor Green
exit 0
