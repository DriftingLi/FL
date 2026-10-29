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

    另有一条**批量出口**（-PruneOrphans，#2081）：清掉「磁盘上有 `wt-*` / `.wt-*` 目录、注册表里没有」的孤儿。
    这些目录不在 git 注册表里 ⇒ 没有 `worktree remove` 可跑、也不该动任何分支；唯一的危险是
    **指回主树的 node_modules junction**（lib/wt-bootstrap.ps1 建的就是这种）：跟随删除会把主树依赖一起删掉。
    所以顺序固定为「先摘链接、再删骨架」，并按「能不能安全删」分两档（判据真源 = lib/wt-orphans.ps1）：
      · orphan   —— 有文件或有链接 ⇒ 先逐条摘链接，再删目录；
      · skeleton —— 全树只有空目录 ⇒ 直接删。
    删完复核**每条链接的目标**原样还在（entries 前后一致），这是这条路径的承重判据。默认同样 dry-run。

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

.PARAMETER PruneOrphans
    批量清掉**未注册**的 `wt-*` / `.wt-*` 目录（孤儿），不接收 -Task。默认 dry-run，确认后加 -Execute。
    **不动任何分支**：孤儿目录里的分支归属不在本模式射程（分支残留走 check-branch-residue.ps1）。
    与 -Branch / -KeepBranch / -Force 互斥（给了就报错，免得以为它会按那些开关行事）。

.PARAMETER WtRoot
    -PruneOrphans 的扫描根，缺省 = 主树根。**只给守护测试注入临时目录用**；生产路径不传即行为不变。

.PARAMETER GitCli
    读 `git worktree list --porcelain` 用哪个 git 可执行文件，默认 `git`。**只给守护测试注桩用**
    （utils/removeWorktreePruneBehavior.test.js 注入一个「注册表永远为空」的桩来验 fail-closed）；
    生产路径不传即行为不变。

.PARAMETER KeepBranch
    只删工作树，保留本地分支。

.PARAMETER Force
    分支上有未合并进 origin/master 的提交时，仍允许删除该分支。破坏性动作，需显式确认。

.EXAMPLE
    pwsh scripts/remove-worktree.ps1 -Task 1727
    # 看计划与持有者；确认后：
    pwsh scripts/remove-worktree.ps1 -Task 1727 -Execute

.EXAMPLE
    pwsh scripts/remove-worktree.ps1 -PruneOrphans
    # 批量出口：列出所有未注册的 wt-* / .wt-* 目录与两档；确认后：
    pwsh scripts/remove-worktree.ps1 -PruneOrphans -Execute

.NOTES
    退出码：0 = 成功（或 dry-run 只读完成）；1 = 参数非法；2 = 取不到仓库根（-PruneOrphans 下另含
            「拿不到 worktree 注册表」⇒ 分不清孤儿与在用，拒绝删）；3 = 执行失败 / 仍有残留；
            4 = 分支有未合并提交且未带 -Force（拒绝破坏性删除）；
            5 = 调用方 shell 的 CWD 在目标树内（先 cd 到树外再执行）。
    机制与血账：docs/agents/multi-agent-git.md「worktree 收尾」段；#1144 / #1727。
#>
[CmdletBinding(DefaultParameterSetName = 'Single')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Single', Position = 0)][string]$Task,
    [Parameter(Mandatory = $true, ParameterSetName = 'Orphans')][switch]$PruneOrphans,
    [string]$Branch,
    [string]$HxCli,
    [string]$DevToolsCli,
    [switch]$Execute,
    [switch]$KeepBranch,
    [switch]$Force,
    [string]$WtRoot = '',
    [string]$GitCli = 'git'
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

# 路径归一化只有一份实现：lib/wt-orphans.ps1 的 Get-WtNormPath（注册表侧与磁盘侧必须同一规则）。
. (Join-Path $PSScriptRoot 'lib\wt-orphans.ps1')

# ⚠️ 单棵树那条的 -Task 校验只属于 'Single' 参数集：-PruneOrphans 没有 -Task（互斥由 param 声明保证）。
if ($PSCmdlet.ParameterSetName -eq 'Single') {
    if ([string]::IsNullOrWhiteSpace($Task)) { Fail '-Task 不能为空' 1 }
    if ($Task -match '[\\/]') { Fail "-Task 不得含路径分隔符：$Task" 1 }
}

$RepoRoot = Get-WtMainRoot -Base $PSScriptRoot
if (-not $RepoRoot) { Fail '取不到主树根（--git-common-dir 失败，不在 git 仓库里？）' 2 }

# 列出目录里的每条链接及其目标（计划与执行共用一份解析；DryRun 也要能看到「指向哪」，人才判得了）。
function Get-WtLinkTarget {
    param([Parameter(Mandatory = $true)][string]$Path)
    $out = @()
    foreach ($l in @((Measure-WtDirContent -Path $Path).links)) {
        $t = ''
        try { $t = [string](Get-Item -LiteralPath $l -Force -ErrorAction Stop).Target } catch { $t = '' }
        $label = if ($t) { Split-Path -Leaf $t } else { '(unresolved)' }
        $out += [pscustomobject]@{ link = $l; target = $t; label = $label; exists = [bool]($t -and (Test-Path -LiteralPath $t)) }
    }
    return $out
}

# ══════ 批量出口：孤儿 worktree 目录（#2081）══════
# 与单棵树那条的根本差别：这些目录**不在** git 注册表里 ⇒ 没有 worktree remove 可跑、也不该动任何分支。
# 唯一的危险是 node_modules 那种**指回主树的链接**（lib/wt-bootstrap.ps1 建的就是它）——
# 跟随删除会把主树依赖一起删掉，所以顺序固定为「先摘链接、再删骨架」。
if ($PruneOrphans) {
    . (Join-Path $PSScriptRoot 'lib\wt-orphans.ps1')
    if ($Branch -or $KeepBranch -or $Force) {
        Fail '-PruneOrphans 不接收 -Branch / -KeepBranch / -Force：批量出口不动任何分支' 1
    }

    $scanRoot = if ($WtRoot) { $WtRoot } else { $RepoRoot }
    $registry = Get-WtRegistry -RepoRoot $RepoRoot -GitCli $GitCli
    $v = Get-WtOrphanVerdict -ScanRoot $scanRoot -Registered $registry.registered

    # 分不清「孤儿」与「正在用」时**不删**：fail-closed（与 check-branch-residue 的 exit 5 同一取向）
    if (-not $v.ok) {
        Write-Host 'PRUNE_REFUSED  —— 拿不到 worktree 注册表（或扫描根不存在）：分不清孤儿与在用，本次不删任何东西' -ForegroundColor Red
        exit 2
    }

    $linked = Get-WtOrphanByTier -Items $v.items -Tier 'orphan'
    $skeleton = Get-WtOrphanByTier -Items $v.items -Tier 'skeleton'
    $totalLinks = (@($v.items | Measure-Object -Property linkCount -Sum).Sum)
    if ($null -eq $totalLinks) { $totalLinks = 0 }

    # 调用方 CWD 在待删目录内 ⇒ Windows 删不掉（进程当前目录占着它）；强删会留半截 ⇒ 直接拒
    $cwdNorm = Get-WtNormPath (Get-Location).Path
    $inside = @($v.items | Where-Object {
            $p = Get-WtNormPath $_.path
            $cwdNorm -eq $p -or $cwdNorm.StartsWith($p + '\', [System.StringComparison]::OrdinalIgnoreCase)
        })
    if ($inside.Count -gt 0) {
        Write-Host ("PRUNE_CWD_INSIDE  —— 当前目录在待删目录内（{0}）：先 cd 到树外再执行" -f $inside[0].name) -ForegroundColor Red
        if ($Execute) { exit 5 }
    }

    # 逐条列出档 / 链接数 / 文件数 / **每条链接指向哪**：人是拿这张计划逐条裁定「删不删」的，不是盲批。
    foreach ($i in $v.items) {
        $actions = if ($i.tier -eq 'orphan') { 'detach-links,rmdir' } else { 'rmdir' }
        Write-Host ("PRUNE_PLAN {0} tier={1} links={2} files={3} actions={4}" -f $i.name, $i.tier, $i.linkCount, $i.files, $actions)
        foreach ($t in @(Get-WtLinkTarget -Path $i.path)) {
            Write-Host ("  PRUNE_LINK {0} -> {1} exists={2}" -f $i.name, $t.label, ([int]$t.exists))
        }
    }
    $withFiles = @($v.items | Where-Object { $_.files -gt 0 })
    if ($withFiles.Count -gt 0) {
        Write-Host ("PRUNE_NOTICE {0} 个目录里有文件：删前确认 .ci-verify/ .scratch/ 这类被 gitignore 的证据产物不是只活在里面（multi-agent-git.md「删 worktree 之前先把证据拷出来」那条血账）。" -f $withFiles.Count) -ForegroundColor Yellow
    }

    if (-not $Execute) {
        Write-Host ("PRUNE_SUMMARY orphans={0} linked={1} skeleton={2} links={3}" -f $v.items.Count, $linked.Count, $skeleton.Count, $totalLinks)
        Write-Host '[remove-worktree] dry-run：未改任何东西（一条链接都没摘）。确认后加 -Execute。' -ForegroundColor Yellow
        exit 0
    }

    Write-Host ''
    Write-Host '=== 执行：先摘链接、再删骨架 ===' -ForegroundColor Cyan

    # 链接目标基线：摘之前数一次、删完再数一次，一致才算没伤到目标。
    # 目标本来就不存在（悬空 junction，如 #2061 摘掉的那条）记 existed=false、不参与复核 —— 不是我们弄没的。
    $targets = @{}

    $removed = 0
    $detached = 0
    $failed = @()
    foreach ($i in $v.items) {
        # 链接清单取自分档时那份枚举（lib/wt-orphans.ps1 Measure-WtDirContent）：数到几条就摘几条
        $links = @((Measure-WtDirContent -Path $i.path).links)
        $detachFailed = $false
        foreach ($l in $links) {
            $t = ''
            try { $t = [string](Get-Item -LiteralPath $l -Force -ErrorAction Stop).Target } catch { $t = '' }
            if ($t -and -not $targets.ContainsKey($t)) {
                $existed = Test-Path -LiteralPath $t
                $n = -1
                $digest = ''
                if ($existed) {
                    $names = @(Get-ChildItem -LiteralPath $t -Force -ErrorAction SilentlyContinue | Sort-Object Name | ForEach-Object { $_.Name })
                    $n = $names.Count
                    # 只比顶层条目数会漏「数没变、里面被削」那种损伤 ⇒ 再比一份名字集指纹
                    $digest = ($names -join '|')
                }
                $targets[$t] = @{ existed = $existed; entries = $n; names = $digest }
            }
            try {
                $attr = [System.IO.File]::GetAttributes($l)
                # **只删链接本身**：Directory.Delete(recursive=false) 对 junction 正是这个语义（#2061 同款手法）
                if (($attr -band [System.IO.FileAttributes]::Directory) -ne 0) { [System.IO.Directory]::Delete($l, $false) }
                else { [System.IO.File]::Delete($l) }
                $detached++
            } catch {
                Write-Host ("PRUNE_DETACH_FAIL {0} :: {1}" -f $i.name, $_.Exception.Message) -ForegroundColor Red
                $detachFailed = $true
            }
        }

        # 摘链没全成、或摘完仍能看到链接 / 枚举不完整 ⇒ **不许删**。
        # 「先摘链接再删骨架」若只靠注释保证，就等于赌「这个删除实现不跟随链接」——赌输的代价是主树依赖。
        $afterDetach = Measure-WtDirContent -Path $i.path
        $leftLinks = @($afterDetach.links).Count
        if ($detachFailed -or $leftLinks -gt 0 -or -not $afterDetach.complete) {
            Write-Host ("PRUNE_LINK_LEFT {0} :: 未摘净（left={1} complete={2}），跳过删除（fail-closed）" -f $i.name, $leftLinks, ([int]$afterDetach.complete)) -ForegroundColor Red
            $failed += $i.name
            continue
        }

        # 到这里才删：剩下的是一棵普通空骨架。先用 cmd rmdir（与单棵树同手法），仍删不掉再用 .NET 兜底
        $out = cmd /c rmdir /s /q "$($i.path)" 2>&1
        if (Test-Path -LiteralPath $i.path) {
            try { [System.IO.Directory]::Delete($i.path, $true) } catch { }
        }
        if (Test-Path -LiteralPath $i.path) {
            Write-Host ("PRUNE_FAIL {0} :: {1}" -f $i.name, (($out | Out-String).Trim())) -ForegroundColor Red
            $failed += $i.name
        } else {
            Write-Host ("PRUNE_OK {0} (links={1})" -f $i.name, $links.Count) -ForegroundColor Green
            $removed++
        }
    }

    Write-Host ''
    Write-Host '── 复核：链接目标原样还在（跟随链接的实现会在这里变红）──' -ForegroundColor Cyan
    $kept = 0
    $checked = 0
    foreach ($t in $targets.Keys) {
        if (-not $targets[$t].existed) { continue }
        $checked++
        $names = @(Get-ChildItem -LiteralPath $t -Force -ErrorAction SilentlyContinue | Sort-Object Name | ForEach-Object { $_.Name })
        $after = $names.Count
        $afterNames = ($names -join '|')
        $label = Split-Path -Leaf $t
        if ((Test-Path -LiteralPath $t) -and $after -eq $targets[$t].entries -and $afterNames -eq $targets[$t].names) {
            $kept++
            Write-Host ("LINK_TARGET_KEPT {0} entries={1} after={2}" -f $label, $targets[$t].entries, $after) -ForegroundColor Green
        } else {
            Write-Host ("LINK_TARGET_CHANGED {0} entries={1} after={2}" -f $label, $targets[$t].entries, $after) -ForegroundColor Red
        }
    }

    Write-Host ("PRUNE_SUMMARY orphans={0} removed={1} linked={2} skeleton={3} links={4} targetsKept={5}/{6}" -f $v.items.Count, $removed, $linked.Count, $skeleton.Count, $detached, $kept, $checked)
    if ($removed -eq $v.items.Count -and $kept -eq $checked -and $failed.Count -eq 0) {
        Write-Host '[remove-worktree] ✅ 孤儿目录已清掉，链接目标未受影响。' -ForegroundColor Green
        exit 0
    }
    Write-Host ("[remove-worktree] 仍有残留：{0}" -f (@($failed | Select-Object -Unique) -join ', ')) -ForegroundColor Red
    Write-Host '[remove-worktree] 处置：关掉占用它的 HBuilderX / 微信开发者工具 / 会话后重跑；不要为此 kill 别人的会话。' -ForegroundColor Yellow
    exit 3
}

# ══════ 单棵树：-Task <n> ══════
$name = "wt-$Task"
$dest = Join-Path $RepoRoot $name
if (-not $Branch) { $Branch = "feat/$Task" }

# ── 预检：注册状态、目录、移动端依赖 junction、已知持有者
$destNorm = Get-WtNormPath $dest

# 调用方 shell 的 CWD 若在树内，会以「进程当前目录」的形式占着目录（Windows 语义），删不掉。
# 本进程随后切到主树只是放掉自己这一份；调用方仍须先 cd 出去。
$cwdNorm = Get-WtNormPath (Get-Location).Path
$cwdInside = ($cwdNorm -eq $destNorm) -or $cwdNorm.StartsWith($destNorm + '\', [System.StringComparison]::OrdinalIgnoreCase)
if ($cwdInside) {
    Write-Host "[remove-worktree] 当前目录在目标树内（$cwdNorm）。先 cd 到树外再执行；调用方 shell 的 CWD 会挡住删除。" -ForegroundColor Yellow
    if ($Execute) { exit 5 }
}
Set-Location -LiteralPath $RepoRoot

$porcelain = & git -C $RepoRoot worktree list --porcelain 2>$null
$registered = @($porcelain | Where-Object { $_ -like 'worktree *' } | ForEach-Object { Get-WtNormPath ($_.Substring(9).Trim()) }) -contains $destNorm
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
$leftReg = @(& git -C $RepoRoot worktree list --porcelain 2>$null | Where-Object { $_ -like 'worktree *' } | ForEach-Object { Get-WtNormPath ($_.Substring(9).Trim()) }) -contains $destNorm

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
