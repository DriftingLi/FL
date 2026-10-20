# 孤儿 worktree 清理脚本 - 精准匹配已合并 PR
param([switch]$ForceRun)

#
# ⛔ STALE GUARD（#1723，2026-10-20）：本脚本的 $mergedBranches / $worktrees 两张表
# 是 2026-10-14 前后的一次性快照，与磁盘现状已脱节（wt-1514-8/1543/1570/1602/1603/1604
# 等树早已不在；目标里没有后来出现的 wt-1718 / wt-1713-verify 孤儿壳）。
# 它还带 --force 裸删与硬编码路径。**重跑前必须按 `git worktree list` 现状重写两张表**；
# 直接跑 = 对着不存在的树做无效功，或在未来树名撞上时误删。默认在此拦停。
if (-not $ForceRun) {
    Write-Host "[STALE-GUARD] targets 表已过期（#1723）；按现状重写两张表后，用 -ForceRun 显式覆盖本拦截。" -ForegroundColor Red
    return
}

$repo = "DriftingLi/FL"
$gitRoot = "d:/FL"

# 已合并 PR 对应的 headRefName（从 gh pr list 获取）
$mergedBranches = @(
    "feat/1602", "feat/1603", "feat/1604", "feat/1606", 
    "feat/1543", "docs/1543", "feat/1570",
    "refactor/1514-valuation-flatten-8"
)

# 当前所有非主树 worktree
$worktrees = @(
    @{ Path="D:/FL/wt-1514-8"; Branch="refactor/1514-valuation-flatten-8" },
    @{ Path="D:/FL/wt-1543"; Branch="feat/1543" },
    @{ Path="D:/FL/wt-1543-docs"; Branch="docs/1543" },
    @{ Path="D:/FL/wt-1570"; Branch="feat/1570" },
    @{ Path="D:/FL/wt-1602"; Branch="feat/1602" },
    @{ Path="D:/FL/wt-1603"; Branch="feat/1603" },
    @{ Path="D:/FL/wt-1604"; Branch="feat/1604" },
    @{ Path="D:/FL/wt-1606"; Branch="feat/1606" },
    @{ Path="D:/FL/wt-1610"; Branch="feat/1610" },
    @{ Path="D:/FL/wt-1634"; Branch="feat/1634" },
    @{ Path="D:/FL/wt-1650"; Branch="feat/1650" },
    @{ Path="D:/FL/wt-1661"; Branch="feat/1661" },
    @{ Path="D:/FL/wt-1677"; Branch="feat/1677" }
)

$orphans = @()
$kept = @()

foreach ($wt in $worktrees) {
    if ($mergedBranches -contains $wt.Branch) {
        Write-Host "[ORPHAN] $($wt.Branch) - PR merged" -ForegroundColor Red
        $orphans += $wt
    } else {
        Write-Host "[ACTIVE] $($wt.Branch) - no merged PR" -ForegroundColor Green
        $kept += $wt
    }
}

# 清理孤儿 worktrees
Write-Host "`n`n========== CLEANUP REPORT ==========" -ForegroundColor Magenta
Write-Host "Orphans to remove: $($orphans.Count)" -ForegroundColor Red
Write-Host "Active branches kept: $($kept.Count)" -ForegroundColor Green

if ($orphans.Count -gt 0) {
    foreach ($orphan in $orphans) {
        Write-Host "`n--- Removing $($orphan.Path) ---" -ForegroundColor Red
        
        # a) 检查 node_modules junction
        $nodeModulesPath = Join-Path $orphan.Path "training-app/叉车维修培训学员端跨端应用/node_modules"
        if (Test-Path $nodeModulesPath) {
            $item = Get-Item $nodeModulesPath
            $isJunction = $item.Attributes -band [System.IO.FileAttributes]::ReparsePoint
            if ($isJunction) {
                Write-Host "Detected junction at: $nodeModulesPath" -ForegroundColor Yellow
                try {
                    [System.IO.Directory]::Delete($nodeModulesPath, $false) | Out-Null
                    Write-Host "Removed junction only" -ForegroundColor Gray
                } catch {
                    Write-Host "[WARNING] Could not delete junction: $_" -ForegroundColor Yellow
                }
            }
        }
        
        # b) remove worktree
        Write-Host "Executing: git worktree remove '$($orphan.Path)' --force" -ForegroundColor Gray
        try {
            & git -C $gitRoot worktree remove "$($orphan.Path)" --force 2>&1 | Out-String | ForEach-Object { Write-Host $_ }
        } catch {
            Write-Host "[ERROR] Failed to remove worktree: $_" -ForegroundColor Red
            continue
        }
        
        # c) verify directory gone
        if (Test-Path $orphan.Path) {
            Write-Host "[WARNING] Directory still exists!" -ForegroundColor Red
        } else {
            Write-Host "[OK] Directory verified removed" -ForegroundColor Green
        }
        
        # d) delete local branch
        Write-Host "Cleaning local branch: $($orphan.Branch)" -ForegroundColor Gray
        try {
            & git -C $gitRoot branch -D $($orphan.Branch) 2>&1 | Out-String | ForEach-Object { Write-Host $_ }
        } catch {
            Write-Host "[INFO] Branch deletion skipped or already gone" -ForegroundColor Gray
        }
    }
    
    # e) prune
    Write-Host "`nPruning git worktree registry..." -ForegroundColor Magenta
    & git -C $gitRoot worktree prune
    Write-Host "Prune complete." -ForegroundColor Green
} else {
    Write-Host "`nNo orphan worktrees found. Skipping cleanup." -ForegroundColor Green
}

# 汇报结果
Write-Host "`n========== SUMMARY ==========" -ForegroundColor Magenta
Write-Host "Removed: $($orphans.Count) orphan worktree(s)" -ForegroundColor Red
Write-Host "Kept: $($kept.Count) active branch(es):" -ForegroundColor Green
foreach ($k in $kept) {
    Write-Host "  - $($k.Path) ($($k.Branch))" -ForegroundColor Cyan
}
