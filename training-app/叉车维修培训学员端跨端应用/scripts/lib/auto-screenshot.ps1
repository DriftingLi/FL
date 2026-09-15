<#
.SYNOPSIS
    自动截图模块：只截**本次改动涉及的页面**，真翻页、带反假绿判据。

.DESCRIPTION
    **Q3 口径（2026-09-14 用户裁定）**：只截改动页面 + `-Pages` 参数。
    为什么要限范围：每页一次 `cli launch app-android --pagePath <页>` 都要过一遍 HBuilderX
    （编译 + 推送），本仓 `pages.json` 有 50 页 ⇒ 全量截图成本不可接受
    （`device-capture.ps1` 早已把「逐页 launch 成本高」标为待裁定，本次裁定即此）。

    **页面集合的确定顺序**：
      1. `-Pages <a,b>` 显式给定 → 用它（最高优先级）
      2. 否则**从 git diff 推导**：把 `pages/**/*.uvue` 的改动映射回 pages.json 里的 page path
         （`-ChangedOnly` 是这一行为的显式声明；两者同给时以 `-Pages` 为准）
      3. 推导出 **0 页 ⇒ 明报「无改动页面」并返回**，**绝不回退到全量**（fail-safe）
      4. `-MaxPages`（默认 5）兜底裁剪，被裁掉的页记入 `Skipped`

    导航与判据：
      · 切页用 `cli launch app-android --pagePath <页>`（`device-capture.ps1` #898 spike 实测：
        `am start` 的 uniapp 深链**不被 App 处理**，会停在原页）
      · 每页截图算 **SHA256**；与上一页**相同 ⇒ 判切页未生效**（记入 `HashConflicts`，令 `Ok=$false`）
      · **截图文件的时间戳必须晚于本次运行起点**：`$OutputDir` 不清理、文件名按页名固定，
        故 adb 静默失败时**上一次运行的同名残留 PNG** 会让「存在且非空」照样通过 ——
        那正是把陈旧截图当本次证据的路径。早于起点的记入 `StaleShots`，并按跳过处理。

    `Ok=true` 仅当：至少截到 1 页、无跳过、无 hash 冲突。

.EXAMPLE
    . scripts/lib/auto-screenshot.ps1
    # 默认：从 git diff 推导改动页面
    $r = Invoke-AutoScreenshot -Device '192.168.10.51:39181' -CliPath 'D:\...\cli.exe' -ProjectDir 'D:\FL\...'
    # 显式指定
    $r = Invoke-AutoScreenshot -Pages 'pages/index/index,pages/profile/profile' -Device '...' -CliPath '...'
#>

function Invoke-AutoScreenshot {
    [CmdletBinding()]
    param(
        [string]$Device,
        [string]$CliPath,
        [string]$ProjectDir,
        [string]$OutputDir,
        [string]$Pages = '',
        [switch]$ChangedOnly,
        [int]$MaxPages = 5,
        [int]$WaitSeconds = 3
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputDir) {
        $OutputDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }

    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

    # ---------- adb ----------
    $adbExe = $null
    foreach ($root in @($env:ANDROID_SDK_ROOT, $env:ANDROID_HOME)) {
        if ($root) {
            $c = Join-Path $root 'platform-tools\adb.exe'
            if (Test-Path -LiteralPath $c) { $adbExe = $c; break }
        }
    }
    if (-not $adbExe) {
        $cmd = Get-Command adb -ErrorAction SilentlyContinue
        if ($cmd) { $adbExe = $cmd.Source }
    }
    if (-not $adbExe) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 adb.exe' }
    }

    # ---------- HBuilderX cli ----------
    if (-not $CliPath) { $CliPath = $env:HBuilderX_CLI }
    if (-not $CliPath) {
        $proc = $null
        try { $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1 } catch { }
        if ($proc) {
            try {
                $runningCli = Join-Path (Split-Path $proc.Path) 'cli.exe'
                if (Test-Path -LiteralPath $runningCli) { $CliPath = $runningCli }
            } catch { }
        }
    }
    if (-not $CliPath -or -not (Test-Path -LiteralPath $CliPath)) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 HBuilderX cli.exe' }
    }

    # ---------- pages.json（取全量清单，用于映射与校验）----------
    $pagesPath = Join-Path $ProjectDir 'pages.json'
    if (-not (Test-Path -LiteralPath $pagesPath)) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 pages.json' }
    }
    $pagesJson = Get-Content -LiteralPath $pagesPath -Raw | ConvertFrom-Json
    $allPages = @($pagesJson.pages | ForEach-Object { $_.path })

    # ---------- 1) 确定目标页集合 ----------
    $targetPages = @()

    if ($Pages) {
        # 显式清单优先
        $targetPages = @($Pages -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    }
    else {
        # 默认（含 -ChangedOnly）：从 git diff 推导改动涉及的页面
        $diffFiles = @()
        foreach ($gitArgs in @(@('diff', '--name-only', 'HEAD'), @('diff', '--name-only', 'origin/master...HEAD'))) {
            try {
                $raw = & git -C $ProjectDir @gitArgs 2>$null
                if ($raw) { $diffFiles += @($raw | Where-Object { $_ -and "$_".Trim() } | ForEach-Object { "$_".Trim() }) }
            }
            catch { }
        }
        $diffFiles = @($diffFiles | Select-Object -Unique)

        $derived = @()
        foreach ($f in $diffFiles) {
            # pages/<name>.uvue  →  pages/<name>
            if ($f -match '^pages/(.+)\.uvue$') {
                $candidate = "pages/$($Matches[1])"
                if ($allPages -contains $candidate) { $derived += $candidate }
            }
        }
        $targetPages = @($derived | Select-Object -Unique)
    }

    # ---------- 3) 0 页 ⇒ 明报并返回，绝不回退全量 ----------
    if ($targetPages.Count -eq 0) {
        $why = if ($Pages) { '指定的 -Pages 为空' } else { 'git diff 里没有 pages/**/*.uvue 改动' }
        return [pscustomobject]@{
            Ok           = $false
            Screenshots  = @()
            Skipped      = @()
            HashConflicts = @()
            TargetPages  = @()
            Error        = "无改动页面（$why）⇒ 不截图（**不回退到全量**）。要显式指定请用 -Pages 'pages/xxx/xxx'。"
        }
    }

    # ---------- 4) -MaxPages 兜底裁剪 ----------
    $capped = @()
    if ($targetPages.Count -gt $MaxPages) {
        $capped = @($targetPages[$MaxPages..($targetPages.Count - 1)])
        $targetPages = @($targetPages[0..($MaxPages - 1)])
        Write-Host "[auto-screenshot] 目标页 $($targetPages.Count + $capped.Count) 个超过 -MaxPages=$MaxPages，裁掉：$($capped -join ', ')" -ForegroundColor Yellow
    }

    # ---------- 逐页导航 + 截图 ----------
    $screenshots = @()
    $skipped = @($capped)
    $hashConflicts = @()
    $staleShots = @()
    $seenHashes = @{}
    $prevHash = ''

    # 本次运行的起点：用于「截图必须是本次新落的」判据（见下方陈旧截图检查）。
    # ⚠️ 必须在**进入循环之前**取，否则每页各取一次会让判据退化成恒真。
    $runStarted = Get-Date

    $pageNum = 0
    foreach ($page in $targetPages) {
        $pageNum++
        $pageName = ($page -split '/')[-1]
        $outputFile = Join-Path $OutputDir "$pageName.png"

        Write-Host "[auto-screenshot] ($pageNum/$($targetPages.Count)) 导航: $page" -ForegroundColor Cyan

        try {
            # 切页：cli launch app-android --pagePath <页> --project <项目> [--deviceId <设备>]
            $launchArgs = @('launch', 'app-android', '--pagePath', $page, '--project', $ProjectDir)
            if ($Device) { $launchArgs += @('--deviceId', $Device) }
            $null = & $CliPath @launchArgs 2>&1 | Out-String

            Start-Sleep -Seconds $WaitSeconds

            # 截图（exec-out 保 PNG 字节流）
            $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $adbExe, $Device, $outputFile
            & cmd.exe /c $cmd 2>&1 | Out-Null

            if (-not (Test-Path -LiteralPath $outputFile) -or (Get-Item -LiteralPath $outputFile).Length -eq 0) {
                $skipped += $pageName
                Write-Host "  ⚠️ $pageName 截图失败" -ForegroundColor Yellow
                continue
            }

            # 反假绿：截图**必须是本次运行新落的**。
            # ⚠️ 为什么必须有这条：`$OutputDir` 默认 `.ci-verify\screenshots`，目录**不清理**，
            #    文件名按页名固定。若 adb 截图静默失败（写入 0 字节或没写），上面那条只查
            #    「文件存在且非空」——**上一次运行残留的同名 PNG 会让它照样通过**，
            #    于是把陈旧截图当成本次证据。时间戳判据把这种情形钉死。
            $shotWritten = (Get-Item -LiteralPath $outputFile).LastWriteTime
            if ($shotWritten -lt $runStarted) {
                $staleShots += $pageName
                $skipped += $pageName
                Write-Host "  ❌ $pageName 的截图时间戳早于本次运行起点（陈旧文件，疑似本次截图未真正落盘）" -ForegroundColor Red
                continue
            }

            $hash = (Get-FileHash -LiteralPath $outputFile -Algorithm SHA256).Hash

            # 反假绿：与上一页 hash 相同 ⇒ 切页没生效
            if ($prevHash -and $hash -eq $prevHash) {
                $hashConflicts += $pageName
                $skipped += $pageName
                Write-Host "  ❌ $pageName 的 hash 与上一页相同 ⇒ 切页未生效" -ForegroundColor Red
                continue
            }

            $seenHashes[$pageName] = $hash
            $prevHash = $hash
            $screenshots += $pageName
            $short = if ($hash.Length -ge 16) { $hash.Substring(0, 16) } else { $hash }
            Write-Host "  ✅ $pageName.png (sha256=$short…)" -ForegroundColor Green
        }
        catch {
            $skipped += $pageName
            Write-Host "  ⚠️ $pageName 异常: $($_.Exception.Message)" -ForegroundColor Yellow
        }
    }

    $allOk = ($screenshots.Count -gt 0 -and $skipped.Count -eq 0 -and $hashConflicts.Count -eq 0)
    $errorMsg = ''
    if ($hashConflicts.Count -gt 0) {
        $errorMsg = "hash 冲突（切页未生效）: $($hashConflicts -join ', ')"
    }
    elseif ($screenshots.Count -eq 0) {
        $errorMsg = '没有任何页面截图成功'
    }

    return [pscustomobject]@{
        Ok            = $allOk
        Screenshots   = $screenshots
        Skipped       = $skipped
        HashConflicts = $hashConflicts
        StaleShots    = $staleShots
        TargetPages   = $targetPages
        Error         = $errorMsg
    }
}
