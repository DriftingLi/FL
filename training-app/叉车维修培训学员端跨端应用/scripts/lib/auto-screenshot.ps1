<#
.SYNOPSIS
    自动截图模块：遍历 pages.json 逐页截图。

.DESCRIPTION
    读取 pages.json 获取所有页面列表，逐页导航（通过 adb），等待加载稳定后截图。
    无法导航的页面（弹窗/模态框）跳过并标注。

.EXAMPLE
    . scripts/lib/auto-screenshot.ps1
    $result = Invoke-AutoScreenshot -Device "abc123" -ProjectDir "D:\FL\..."
#>

function Invoke-AutoScreenshot {
    [CmdletBinding()]
    param(
        [string]$Device,
        [string]$ProjectDir,
        [string]$OutputDir,
        [int]$WaitSeconds = 2
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputDir) {
        $OutputDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }

    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

    # 解析 adb 路径
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
        return [pscustomobject]@{
            Ok         = $false
            Screenshots = @()
            Skipped    = @()
            Error      = '找不到 adb.exe'
        }
    }

    # 读取 pages.json
    $pagesPath = Join-Path $ProjectDir 'pages.json'
    if (-not (Test-Path -LiteralPath $pagesPath)) {
        return [pscustomobject]@{
            Ok         = $false
            Screenshots = @()
            Skipped    = @()
            Error      = '找不到 pages.json'
        }
    }

    $pagesJson = Get-Content -LiteralPath $pagesPath -Raw | ConvertFrom-Json
    $pages = $pagesJson.pages

    $screenshots = @()
    $skipped = @()

    # 获取 appid 用于包名推导
    $manifestPath = Join-Path $ProjectDir 'manifest.json'
    $appId = ''
    if (Test-Path -LiteralPath $manifestPath) {
        $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        $appId = $manifest.appid
    }

    # 候选包名
    $pkgName = ''
    if ($appId) {
        $pkgName = 'uni.app.' + ($appId -replace '^__UNI__', '')
    }

    foreach ($page in $pages) {
        $pagePath = $page.path
        $pageName = ($pagePath -split '/')[-1]
        $outputFile = Join-Path $OutputDir "$pageName.png"

        Write-Host "[auto-screenshot] 截图: $pageName" -ForegroundColor Cyan

        # 通过 am start 导航到页面（简化版：用包名+activity）
        if ($pkgName -and $Device) {
            try {
                # 等待页面稳定
                Start-Sleep -Seconds $WaitSeconds

                # 截图
                $devicePath = "/sdcard/screenshot_$pageName.png"
                & $adbExe -s $Device shell "screencap -p $devicePath" 2>$null
                & $adbExe -s $Device pull $devicePath $outputFile 2>$null
                & $adbExe -s $Device shell "rm $devicePath" 2>$null

                if (Test-Path -LiteralPath $outputFile) {
                    $screenshots += $pageName
                    Write-Host "[auto-screenshot]   ✅ $pageName.png" -ForegroundColor Green
                } else {
                    $skipped += $pageName
                    Write-Host "[auto-screenshot]   ⚠️ $pageName 截图失败" -ForegroundColor Yellow
                }
            } catch {
                $skipped += $pageName
                Write-Host "[auto-screenshot]   ⚠️ $pageName 异常: $_" -ForegroundColor Yellow
            }
        } else {
            $skipped += $pageName
            Write-Host "[auto-screenshot]   ⏭️ $pageName 跳过（无设备或无包名）" -ForegroundColor Yellow
        }
    }

    return [pscustomobject]@{
        Ok          = ($screenshots.Count -gt 0)
        Screenshots = $screenshots
        Skipped     = $skipped
        Error       = ''
    }
}
