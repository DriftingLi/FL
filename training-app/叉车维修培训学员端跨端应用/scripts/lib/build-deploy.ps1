<#
.SYNOPSIS
    构建部署模块：按级别决定「只编译诊断」还是「真运行到设备」。

.DESCRIPTION
    **Q-2 口径（2026-09-14 用户裁定）**：
      · quick           → **不部署**（编译诊断由 test-compile 的 `-CompileOnly` 负责；本模块明报 Deployed=false）
      · standard / full → 调 `hx-run.ps1` **真运行**（编译 + 推送 + 启动 + 设备侧基线判据）

    **不自己实现 launch**：真运行路径已由 `hx-run.ps1` 实测验证（236–287 秒、设备侧资源目录 mtime 前进判据、
    分段计时、分失败语义）。本模块第一版**重复实现且漏掉了整个 launch 调用**，并把 Deployed 无条件写成真值
    ⇒ 假绿（`dev:finish` 报「✅ 已到设备（耗时 0s）」而实际什么都没做）。本版改为**解析 hx-run 的机检行**取结论：
      HX_RUN mode=… compile=… deploy=… total=… exit=ok|fail|env
      HX_RUN_DEPLOY deployed=true|false www_before=… www_after=… pid_before=… pid_after=… reason=…

    **fail-closed（B4/B7）**：`$deployed` 默认 `$false`；只有 `HX_RUN_DEPLOY deployed=true` 才置真；
    **机检行缺失**（hx-run 未正常收口）⇒ 判 `Ok=$false`，绝不假装成功。

    > 契约测试 B8 断言本文件**不得出现**「Deployed 直接赋真值」那个字面量 —— **注释里也不行**
    > （仓库惯例，同 `hx-run.ps1` 的 C1 守护说明）。

.EXAMPLE
    . scripts/lib/build-deploy.ps1
    $r = Invoke-BuildAndDeploy -Level 'standard' -Device '192.168.10.51:39181' -ProjectDir 'D:\FL\...'
#>

function Invoke-BuildAndDeploy {
    [CmdletBinding()]
    param(
        [string]$CliPath,
        [string]$Device,
        [string]$ProjectDir,
        [ValidateSet('quick', 'standard', 'full')]
        [string]$Level = 'quick',
        [switch]$Full,
        # 透传给 hx-run 的 `-TimeoutSeconds`（真运行模式下 = 部署轮询上限）。
        # 默认 1800：2026-09-14 实测本项目编译段可远超 900 秒（冷缓存下 901 秒仍在编译）。
        [int]$RunTimeoutSeconds = 1800
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }

    $ErrorActionPreference = 'Stop'
    $started = Get-Date

    # ---- fail-closed 默认值：没拿到证据就一直是 false ----
    $ok = $false
    $deployed = $false
    $reason = ''
    $runLine = ''
    $deployVerdict = ''
    $manifestRestored = $false
    $pagesRestored = $false

    # ---- 记录构建前状态（用于 HBuilderX 回写检测）----
    $manifestPath = Join-Path $ProjectDir 'manifest.json'
    $pagesPath = Join-Path $ProjectDir 'pages.json'
    $manifestBefore = ''
    $pagesBefore = ''
    if (Test-Path -LiteralPath $manifestPath) { $manifestBefore = (Get-Content -LiteralPath $manifestPath -Raw) }
    if (Test-Path -LiteralPath $pagesPath) { $pagesBefore = (Get-Content -LiteralPath $pagesPath -Raw) }

    $hxRun = Join-Path $ProjectDir 'scripts\hx-run.ps1'

    if ($Level -eq 'quick') {
        # 🟢 快速模式：不部署（Q-2）
        Write-Host '[build-deploy] 🟢 快速模式：不部署（编译诊断由 test-compile 的 -CompileOnly 负责）' -ForegroundColor Gray
        $ok = $true
        $deployed = $false
        $reason = 'quick 模式：不部署（只做编译期诊断）'
    }
    else {
        if (-not (Test-Path -LiteralPath $hxRun)) {
            # fail-closed：脚本都不在 ⇒ 不假装成功
            $ok = $false
            $deployed = $false
            $reason = "找不到 hx-run.ps1：$hxRun"
            Write-Host "[build-deploy] ❌ $reason" -ForegroundColor Red
        }
        else {
            Write-Host "[build-deploy] 调 hx-run.ps1 真运行（$Level 模式）..." -ForegroundColor Cyan

            $runExtra = @('-Project', $ProjectDir, '-TimeoutSeconds', "$RunTimeoutSeconds")
            if ($Device) { $runExtra += @('-Device', $Device) }
            if ($CliPath) { $runExtra += @('-Cli', $CliPath) }
            if ($Full) { $runExtra += @('-Full') }

            try {
                $runOutput = & pwsh -NoProfile -ExecutionPolicy Bypass -File $hxRun @runExtra 2>&1 | Out-String
            }
            catch {
                $runOutput = "$($_.Exception.Message)"
            }
            $runExit = $LASTEXITCODE

            # ---- 解析机检行（唯一判据来源）----
            foreach ($line in ($runOutput -split "`r?`n")) {
                if ($line -match 'HX_RUN_DEPLOY\s+deployed=(true|false)') { $deployVerdict = $Matches[1] }
                if ($line -match '^HX_RUN\s+mode=') { $runLine = $line.Trim() }
            }

            if (-not $runLine) {
                # fail-closed：机检行缺失 ⇒ hx-run 未正常收口，判未部署（绝不放过）
                $ok = $false
                $deployed = $false
                $reason = "机检行缺失（hx-run 未正常收口，exit=$runExit）⇒ 判未部署"
            }
            elseif ($deployVerdict -eq 'true') {
                $ok = $true
                $deployed = $true
                $reason = "hx-run 报 deployed=true（$runLine）"
            }
            else {
                $ok = $false
                $deployed = $false
                $reason = "hx-run 报 deployed=$deployVerdict（$runLine）⇒ 判未部署"
            }

            # 把 hx-run 的输出落盘，便于排障
            $logDir = Join-Path $ProjectDir '.ci-verify'
            New-Item -ItemType Directory -Force -Path $logDir | Out-Null
            Add-Content -LiteralPath (Join-Path $logDir 'build-deploy.log') -Encoding utf8 `
                -Value "`n# ---- hx-run（$Level）exit=$runExit ----`n$runOutput" -ErrorAction SilentlyContinue
        }
    }

    # ---- 还原 HBuilderX 回写的配置文件 ----
    if ($manifestBefore) {
        $manifestAfter = ''
        if (Test-Path -LiteralPath $manifestPath) { $manifestAfter = (Get-Content -LiteralPath $manifestPath -Raw) }
        if ($manifestAfter -ne $manifestBefore) {
            Set-Content -LiteralPath $manifestPath -Value $manifestBefore -Encoding utf8
            $manifestRestored = $true
            Write-Host '[build-deploy] 已还原 manifest.json' -ForegroundColor Yellow
        }
    }

    if ($pagesBefore) {
        $pagesAfter = ''
        if (Test-Path -LiteralPath $pagesPath) { $pagesAfter = (Get-Content -LiteralPath $pagesPath -Raw) }
        if ($pagesAfter -ne $pagesBefore) {
            Set-Content -LiteralPath $pagesPath -Value $pagesBefore -Encoding utf8
            $pagesRestored = $true
            Write-Host '[build-deploy] 已还原 pages.json' -ForegroundColor Yellow
        }
    }

    return [pscustomobject]@{
        Ok               = $ok
        Deployed         = $deployed
        Reason           = $reason
        RunLine          = $runLine
        Duration         = [int]((Get-Date) - $started).TotalSeconds
        ManifestRestored = $manifestRestored
        PagesRestored    = $pagesRestored
        Error            = $(if ($ok) { '' } else { $reason })
    }
}
