<#
.SYNOPSIS
    孤儿 worktree 目录的**判据真源**：磁盘上有、注册表里没有 ⇒ 孤儿，并按「能不能安全删」分两档。

.DESCRIPTION
    为什么单独抽一份（#2081）：读写同一件事的有两个入口 ——
    `check-branch-residue.ps1`（只报）与 `remove-worktree.ps1 -PruneOrphans`（报完再删）。
    判据（哪些目录算孤儿、算哪一档）若在两个入口各写一遍，迟早一边改一边忘（本仓「别开第二真源」的惯犯形态）。

    **两档的判据是「会不会伤到别处」，不是「看起来像不像残留」**（2026-10-28 主树实测 48 个孤儿：39 挂活链接 / 9 纯骨架）：
      · orphan   —— 目录里有文件**或**有 reparse point（junction / 符号链接）。
                    移动端 worktree 的 node_modules 是指回**主树**的 junction（lib/wt-bootstrap.ps1）⇒
                    这类目录不能直接递归删：跟随链接会把主树 188 条依赖一起删掉。安全顺序 = 先摘链接、再删骨架。
      · skeleton —— 全树只有空目录（无文件、无链接），且**枚举完整**。删它不可能伤到别处。

    ⚠️ 遍历**绝不跟随** reparse point：链接只计数、不进去（跟随会把主树依赖数成孤儿的文件，分档全错）。
    ⚠️ **枚举不到就不许当骨架**（fail-closed）：任何一层子目录读不进去 ⇒ 该目录判 orphan。
       把「读不到」当成「没有」会让人拿到「可直接删」的结论去删一棵自己没看清的树。

    ⚠️ 本文件**只读**：不删目录、不摘链接、不跑 `git worktree remove`。副作用全在入口脚本。
    形态同 lib/wt-bootstrap.ps1（纯逻辑可 dot-source，被 utils/*Behavior.test.js 经真起 pwsh 锁住）。
#>

function Get-WtNormPath {
    <#
      注册表路径与磁盘路径的**唯一**归一化：绝对路径 → 去尾分隔符 → 小写。
      git 在 Windows 上把 worktree 路径打成正斜杠（worktree D:/FL/wt-x），磁盘上取到的是反斜杠 ⇒
      不归一对不上（这是「磁盘对账」这个判据最容易静默失效的地方）。
      ⚠️ 不要在别处再写一份（remove-worktree.ps1 曾另留一个不小写的 Get-NormPath ⇒ 两份判据）。
    #>
    [OutputType([string])]
    param([string]$Path)
    try {
        return [System.IO.Path]::GetFullPath($Path).TrimEnd([char[]]@('\', '/')).ToLowerInvariant()
    } catch {
        return $Path.TrimEnd([char[]]@('\', '/')).ToLowerInvariant()
    }
}

function Get-WtRegistry {
    <#
      git worktree list --porcelain 的一次解析，同时给出两件事实：
        registered —— 全部注册路径（不只挂在分支上的：detached / bare 也算）
        branchOf   —— 分支名 → 该分支所在的注册路径
      给两个入口共用：check-branch-residue.ps1 两件都要（目录对账 + 收口顺序），remove-worktree.ps1 只要 registered。

      ⚠️ 取不到（不在 git 仓库 / git 报错）时 registered 为空表 —— 调用方**必须**把它当成
      「对账做不成」而不是「磁盘干净」（fail-closed）。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [string]$GitCli = 'git'
    )
    $registered = @{}
    $branchOf = @{}
    $cur = $null
    foreach ($line in (& $GitCli -C $RepoRoot worktree list --porcelain 2>$null)) {
        if ($line -like 'worktree *') {
            $cur = $line.Substring(9).Trim()
            if ($cur) { $registered[(Get-WtNormPath $cur)] = $true }
        } elseif ($line -like 'branch refs/heads/*' -and $cur) {
            $branchOf[$line.Substring(18).Trim()] = $cur
        }
    }
    return @{ registered = $registered; branchOf = $branchOf }
}

function Measure-WtDirContent {
    <#
      数一棵目录树里的 文件 / 链接 / 子目录。**不跟随 reparse point**（见文件头）。

      links 给的是**路径清单**而不是个数：摘链接的那一侧（remove-worktree.ps1 -PruneOrphans）必须
      摘的正是这里数出来的那些 —— 一侧按个数、另一侧自己再枚举一遍，两份实现迟早对不上，
      而这里对不上就是「该摘的没摘，然后递归删跟着链接打到主树」。

      complete = $false ⇒ 有子目录读不进去（权限 / 竞态）⇒ 调用方不得把结果当「空」用。
    #>
    param([Parameter(Mandatory = $true)][string]$Path)
    $files = 0
    $dirs = 0
    $links = @()
    $complete = $true
    $stack = New-Object System.Collections.Stack
    $stack.Push($Path)
    while ($stack.Count -gt 0) {
        $cur = $stack.Pop()
        try { $entries = [System.IO.Directory]::GetFileSystemEntries($cur) } catch { $complete = $false; continue }
        foreach ($e in $entries) {
            $attr = [System.IO.File]::GetAttributes($e)
            if (($attr -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { $links += $e; continue }
            if (($attr -band [System.IO.FileAttributes]::Directory) -ne 0) { $dirs++; $stack.Push($e) } else { $files++ }
        }
    }
    return @{ files = $files; links = $links; dirs = $dirs; complete = $complete }
}

function Get-WtOrphanByTier {
    <# 按档过滤（两个入口的 4 处分组都走这里，别各写一遍 Where-Object）。 #>
    param(
        [object[]]$Items,
        [Parameter(Mandatory = $true)][ValidateSet('orphan', 'skeleton')][string]$Tier
    )
    return @(@($Items) | Where-Object { $_ -and $_.tier -eq $Tier })
}

function Get-WtOrphanVerdict {
    <#
      扫描根下的 wt-* / .wt-* 目录 vs 注册表 → 分档清单。
      返回 @{ ok; registered; disk; items = @( @{ name; path; tier; files; linkCount } ) }
        ok = $false ⇒ 对账**没做成**（注册表空 / 扫描根不存在），调用方不得据此判干净。
      目录名只认 wt-* / .wt-*：默认命名（不带点）不会踩 ③ 门的假绿坑（#1144）。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$ScanRoot,
        [Parameter(Mandatory = $true)][hashtable]$Registered
    )
    $v = @{ ok = $false; registered = $Registered.Count; disk = 0; items = @() }
    if ($Registered.Count -eq 0) { return $v }
    if (-not (Test-Path -LiteralPath $ScanRoot)) { return $v }

    $v.ok = $true
    $dirs = @(Get-ChildItem -LiteralPath $ScanRoot -Directory -Force |
        Where-Object { $_.Name -like 'wt-*' -or $_.Name -like '.wt-*' })
    $v.disk = $dirs.Count

    # ⚠️ 用普通数组累加，不用 List + @($items) 赋值：后者在 PowerShell 7 上对哈希表成员会抛
    #    "Argument types do not match"，而且**异常发生在赋值处**、$v 照样返回 ⇒ 判据静默变成「零孤儿」
    #    （2026-10-28 实测踩到；remove-worktree.ps1 也有同款注记）。
    $items = @()
    foreach ($d in ($dirs | Sort-Object Name)) {
        if ($Registered.ContainsKey((Get-WtNormPath $d.FullName))) { continue }
        $m = Measure-WtDirContent -Path $d.FullName
        $nLinks = @($m.links).Count
        # 骨架要同时满足：枚举完整（读不到就不算）且 无文件 且 无链接。其余一律按 orphan（宁可保守）。
        $tier = if ($m.complete -and $m.files -eq 0 -and $nLinks -eq 0) { 'skeleton' } else { 'orphan' }
        $items += [pscustomobject]@{
            name = $d.Name; path = $d.FullName; tier = $tier;
            files = $m.files; linkCount = $nLinks; complete = $m.complete
        }
    }
    $v.items = $items
    return $v
}
