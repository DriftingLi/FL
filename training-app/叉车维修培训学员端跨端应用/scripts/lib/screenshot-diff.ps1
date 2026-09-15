<#
.SYNOPSIS
    截图对比模块：当前截图与基线逐张对比，输出差异报告。

.DESCRIPTION
    对比方式：文件大小 + hash 比较（简单实现）。
    输出差异报告：无变化 / 有变化 / 新增 / 缺失。
    -UpdateBaseline 用当前截图覆盖基线。

.EXAMPLE
    . scripts/lib/screenshot-diff.ps1
    $result = Compare-ScreenshotBaseline -CurrentDir ".ci-verify/screenshots" -BaselineDir ".ci-verify/baseline"
#>

function Compare-ScreenshotBaseline {
    [CmdletBinding()]
    param(
        [string]$CurrentDir,
        [string]$BaselineDir,
        [string]$ProjectDir,
        [switch]$UpdateBaseline
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $CurrentDir) {
        $CurrentDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }
    if (-not $BaselineDir) {
        $BaselineDir = Join-Path $ProjectDir '.ci-verify\baseline'
    }

    # 确保基线目录存在
    $baselineExists = Test-Path -LiteralPath $BaselineDir
    if (-not $baselineExists) {
        New-Item -ItemType Directory -Force -Path $BaselineDir | Out-Null
        Write-Host "[screenshot-diff] 基线目录不存在，已创建: $BaselineDir" -ForegroundColor Yellow
    }

    # 获取当前截图列表
    # ⚠️ 必须用 `@(...)` 包住：`Get-ChildItem` 只命中**一个**文件时会退化成**标量**，而调用方
    #    `dev-finish.ps1` 是以 `Set-StrictMode -Version Latest` 跑的 ⇒ 标量取 `.Count` 直接抛
    #    「在此对象上找不到属性 Count」。2026-09-15 真机实测（#1027 收尾）：基线目录不存在且
    #    **恰好一页**改动时，步骤 7 就崩在这里 —— 而「单页改动 + 首次运行」恰恰是最常见的形态。
    $currentFiles = @()
    if (Test-Path -LiteralPath $CurrentDir) {
        $currentFiles = @(Get-ChildItem -Path $CurrentDir -Filter '*.png' -File)
    }

    if ($currentFiles.Count -eq 0) {
        return [pscustomobject]@{
            Diff         = @()
            ChangedCount = 0
            NewBaseline  = (-not $baselineExists)
            Error        = '无当前截图可对比'
        }
    }

    $diff = @()
    $changedCount = 0
    $isNewBaseline = -not $baselineExists

    foreach ($file in $currentFiles) {
        $baselineFile = Join-Path $BaselineDir $file.Name
        $status = ''

        if (-not (Test-Path -LiteralPath $baselineFile)) {
            # 新增截图（无基线）
            $status = '新增'
            $changedCount++
        } else {
            # 对比文件大小和 hash
            $currentHash = (Get-FileHash -Path $file.FullName -Algorithm MD5).Hash
            $baselineHash = (Get-FileHash -Path $baselineFile -Algorithm MD5).Hash

            if ($currentHash -eq $baselineHash) {
                $status = '无变化'
            } else {
                $status = '⚠️ 有变化'
                $changedCount++
            }
        }

        $diff += "$($file.Name):$status"
        $icon = if ($status -eq '无变化') { '📄' } elseif ($status -eq '新增') { '🆕' } else { '⚠️' }
        Write-Host "[screenshot-diff] $icon $($file.Name) — $status" -ForegroundColor $(if ($status -eq '无变化') { 'Gray' } elseif ($status -eq '新增') { 'Yellow' } else { 'Yellow' })
    }

    # 检查基线中有但当前没有的文件（缺失）
    if (Test-Path -LiteralPath $BaselineDir) {
        # 同上的 `@(...)` 理由：单文件时若退化成标量，后续任何 `.Count` 都会在 StrictMode 下抛错
        $baselineFiles = @(Get-ChildItem -Path $BaselineDir -Filter '*.png' -File)
        foreach ($bFile in $baselineFiles) {
            $currentFile = Join-Path $CurrentDir $bFile.Name
            if (-not (Test-Path -LiteralPath $currentFile)) {
                $diff += "$($bFile.Name):缺失"
                $changedCount++
                Write-Host "[screenshot-diff] ❌ $($bFile.Name) — 缺失" -ForegroundColor Red
            }
        }
    }

    # 更新基线
    if ($UpdateBaseline) {
        New-Item -ItemType Directory -Force -Path $BaselineDir | Out-Null
        foreach ($file in $currentFiles) {
            $dest = Join-Path $BaselineDir $file.Name
            Copy-Item -LiteralPath $file.FullName -Destination $dest -Force
        }
        Write-Host "[screenshot-diff] ✅ 基线已更新" -ForegroundColor Green
    }

    return [pscustomobject]@{
        Diff         = $diff
        ChangedCount = $changedCount
        NewBaseline  = $isNewBaseline
        Error        = ''
    }
}
