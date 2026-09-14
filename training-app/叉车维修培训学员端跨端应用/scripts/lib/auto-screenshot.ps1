<#
.SYNOPSIS
    自动截图模块：遍历 pages.json 逐页截图（真翻页，非假绿）。

.DESCRIPTION
    读取 pages.json 获取所有页面列表，通过 HBuilderX CLI --pagePath 逐页导航，
    等待加载稳定后截图。每页截图计算 SHA256，连续相同 hash 判定切页失效（fail-closed）。

    已知限制（来自 device-capture.ps1 #898 spike 实测）：
      - adb shell am start with uniapp:// deep link 不被 App 处理，会停在原页
      - 唯一可靠切页方式是 cli launch app-android --pagePath <页>

    退出码语义：
      Ok=true  → 所有可导航页面截图成功且 hash 不重复
      Ok=false → 存在失败或 hash 冲突（假绿风险）

.EXAMPLE
    . scripts/lib/auto-screenshot.ps1
    $result = Invoke-AutoScreenshot -Device "192.168.10.51:39181" -CliPath "D:\...\cli.exe" -ProjectDir "D:\FL\..."
#>

function Invoke-AutoScreenshot {
    [CmdletBinding()]
    param(
        [string]$Device,
        [string]$CliPath,
        [string]$ProjectDir,
        [string]$OutputDir,
        [int]$WaitSeconds = 3,
        [int]$NavTimeoutSeconds = 30
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputDir) {
        $OutputDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }

    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

    # ---------- 解析 adb 路径 ----------
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
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); Error = '找不到 adb.exe' }
    }

    # ---------- 解析 HBuilderX CLI ----------
    if (-not $CliPath) { $CliPath = $env:HBuilderX_CLI }
    if (-not $CliPath) {
        # 复用 env-check.ps1 的探测逻辑
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
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); Error = '找不到 HBuilderX cli.exe' }
    }

    # ---------- 读取 pages.json ----------
    $pagesPath = Join-Path $ProjectDir 'pages.json'
    if (-not (Test-Path -LiteralPath $pagesPath)) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); Error = '找不到 pages.json' }
    }

    $pagesJson = Get-Content -LiteralPath $pagesPath -Raw | ConvertFrom-Json
    $pages = @($pagesJson.pages)

    # ---------- tabBar 页面（可直接导航）vs 非 tabBar 页面（可能需要特定入口）----------
    $tabBarPages = @()
    if ($pagesJson.tabBar -and $pagesJson.tabBar.list) {
        $tabBarPages = @($pagesJson.tabBar.list | ForEach-Object { $_.pagePath })
    }

    $screenshots = @()
    $skipped = @()
    $hashConflicts = @()
    $seenHashes = @{}

    # ---------- 逐页导航 + 截图 ----------
    $pageNum = 0
    foreach ($page in $pages) {
        $pageNum++
        $pagePath = $page.path
        $pageName = ($pagePath -split '/')[-1]
        $outputFile = Join-Path $OutputDir "$pageName.png"

        Write-Host "[auto-screenshot] ($pageNum/$($pages.Count)) 导航: $pagePath" -ForegroundColor Cyan

        try {
            # 用 HBuilderX CLI 导航到目标页
            # cli launch app-android --pagePath <页> --project <项目> --deviceId <设备>
            $launchArgs = @('launch', 'app-android', '--pagePath', $pagePath, '--project', $ProjectDir)
            if ($Device) { $launchArgs += @('--deviceId', $Device) }

            $argLine = ($launchArgs | ForEach-Object { if ("$_" -match '\s') { '"' + $_ + '"' } else { "$_" } }) -join ' '
            $output = & $CliPath @launchArgs 2>&1 | Out-String

            # 等待页面加载稳定
            Start-Sleep -Seconds $WaitSeconds

            # 截图（exec-out screencap -p 保持 PNG 字节流）
            $devicePath = "/sdcard/screenshot_$pageName.png"
            $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $adbExe, $Device, $outputFile
            & cmd.exe /c $cmd 2>&1 | Out-Null

            if (-not (Test-Path -LiteralPath $outputFile) -or (Get-Item -LiteralPath $outputFile).Length -eq 0) {
                $skipped += "$pageName (截图失败)"
                Write-Host "  ⚠️ $pageName 截图失败" -ForegroundColor Yellow
                continue
            }

            # 计算 SHA256
            $hash = (Get-FileHash -LiteralPath $outputFile -Algorithm SHA256).Hash

            # 反假绿判据：连续截图 hash 相同 ⇒ 切页没生效
            $prevHash = ''
            if ($seenHashes.Count -gt 0) {
                $prevHash = ($seenHashes.Values | Select-Object -Last 1)
            }
            if ($prevHash -and $hash -eq $prevHash) {
                $hashConflicts += $pageName
                $skipped += "$pageName (hash 与上一页相同，切页可能未生效)"
                Write-Host "  ❌ $pageName hash 与上一页相同，切页可能未生效" -ForegroundColor Red
                continue
            }

            $seenHashes[$pageName] = $hash
            $screenshots += $pageName
            $shortHash = if ($hash.Length -ge 16) { $hash.Substring(0, 16) } else { $hash }
            Write-Host "  ✅ $pageName.png (sha256=$shortHash…)" -ForegroundColor Green

        } catch {
            $skipped += "$pageName (异常: $($_.Exception.Message))"
            Write-Host "  ⚠️ $pageName 异常: $($_.Exception.Message)" -ForegroundColor Yellow
        }
    }

    $allOk = ($skipped.Count -eq 0 -and $hashConflicts.Count -eq 0)
    $errorMsg = ''
    if ($hashConflicts.Count -gt 0) {
        $errorMsg = "hash 冲突（切页未生效）: $($hashConflicts -join ', ')"
    }

    return [pscustomobject]@{
        Ok           = $allOk
        Screenshots  = $screenshots
        Skipped      = $skipped
        HashConflicts = $hashConflicts
        Error        = $errorMsg
    }
}
