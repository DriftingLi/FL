<#
.SYNOPSIS
    一键完成脚本：编译 → 安装 → 截图 → 对比 → 生成证据

.DESCRIPTION
    按改动复杂度自动判定级别（🟢快速 / 🟡标准 / 🔴完整），执行对应流程：

    🟢 快速：只改样式/文案 → 编译+到设备 → 完事
    🟡 标准：改了逻辑 → 编译门+截图对比+证据
    🔴 完整：改了配置/新增页面 → 全部四门+完整证据

    用法：
      npm run dev:finish                    # 自动判定级别
      npm run dev:finish -- -Level quick    # 强制快速模式
      npm run dev:finish -- -DryRun         # 只看计划
      npm run dev:finish -- -UpdateBaseline # 更新截图基线

.EXAMPLE
    npm run dev:finish
    npm run dev:finish -- -Level standard -Device emulator-5554
    npm run dev:finish -- -DryRun
#>
[CmdletBinding()]
param(
    [string]$Device,
    [ValidateSet('quick', 'standard', 'full', '')]
    [string]$Level = '',
    [switch]$DryRun,
    [switch]$Distribute,
    [switch]$UpdateBaseline,
    [int]$HxWaitSeconds = 600
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$started = Get-Date

# ============================================================
# 辅助函数
# ============================================================
function Write-Step {
    param([int]$Num, [int]$Total, [string]$Label)
    Write-Host ""
    Write-Host "[$Num/$Total] $Label" -ForegroundColor Cyan
}

function Write-Result {
    param([bool]$Ok, [string]$Msg)
    if ($Ok) { Write-Host "  ✅ $Msg" -ForegroundColor Green }
    else { Write-Host "  ❌ $Msg" -ForegroundColor Red }
}

# ============================================================
# DryRun 模式：只打印计划
# ============================================================
if ($DryRun) {
    Write-Host "=== dev:finish 计划（-DryRun：不执行任何操作）===" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "步骤："
    Write-Host "  1. 环境检测（adb 设备 + HBuilderX CLI）"
    Write-Host "  2. 级别判定（根据 git diff）"
    Write-Host "  3. 单元测试（npm run test:unit）"
    Write-Host "  4. 编译门（kotlin-all-check）"
    Write-Host "  5. 构建+安装到真机"
    Write-Host "  6. 自动截图（🟡🔴 模式）"
    Write-Host "  7. 截图对比基线（🟡🔴 模式）"
    Write-Host "  8. 生成验收证据（🟡🔴 模式）"
    Write-Host "  9. 还原 manifest.json / pages.json"
    Write-Host ""
    Write-Host "参数："
    Write-Host "  -Device:        ${Device:-<自动检测>}"
    Write-Host "  -Level:         ${Level:-<自动判定>}"
    Write-Host "  -Distribute:    $Distribute"
    Write-Host "  -UpdateBaseline: $UpdateBaseline"
    Write-Host "  -HxWaitSeconds: $HxWaitSeconds"
    exit 0
}

# ============================================================
# Step 1: 环境检测
# ============================================================
Write-Step 1 9 '环境检测'
. (Join-Path $PSScriptRoot 'lib\env-check.ps1')
$envResult = Test-BuildEnv -ProjectDir $ProjectDir -Device $Device -HxWaitSeconds $HxWaitSeconds
if (-not $envResult.Ok) {
    Write-Result $false $envResult.Error
    exit 2
}
Write-Result $true "设备 $($envResult.Device) 在线，HBuilderX CLI 可用"
$Device = $envResult.Device

# ============================================================
# Step 2: 级别判定
# ============================================================
Write-Step 2 9 '级别判定'
. (Join-Path $PSScriptRoot 'lib\level-detect.ps1')
$levelResult = Get-DetectLevel -ProjectDir $ProjectDir -ForceLevel $Level
$detectedLevel = $levelResult.Level
$levelIcon = switch ($detectedLevel) { 'quick' { '🟢' } 'standard' { '🟡' } 'full' { '🔴' } }
Write-Result $true "$levelIcon $detectedLevel — $($levelResult.Reason)"

# ============================================================
# Step 3: 单元测试（只跑 dev:finish 相关的契约测试，不跑全量）
# ============================================================
Write-Step 3 9 '单元测试'
$testPattern = 'levelDetect|envCheck|testCompile|buildDeploy|autoScreenshot|screenshotDiff|evidenceGen|devFinish'
try {
    $testOutput = & npx jest --config jest.config.unit.js -i --testPathPattern $testPattern --forceExit 2>&1 | Out-String
} catch {
    $testOutput = $_.Exception.Message
}
if ($LASTEXITCODE -ne 0) {
    Write-Result $false "单元测试失败（exit code $LASTEXITCODE）"
    exit 1
}
Write-Result $true "单元测试通过"

# ============================================================
# Step 4: 编译门
# ============================================================
Write-Step 4 9 '编译门'
. (Join-Path $PSScriptRoot 'lib\test-compile.ps1')
$compileResult = Invoke-TestAndCompile -Level $detectedLevel -ProjectDir $ProjectDir -SkipTests
if (-not $compileResult.Ok) {
    Write-Result $false "$($compileResult.Step) 失败: $($compileResult.Error)"
    exit 1
}
Write-Result $true "编译通过 ($($compileResult.CompileResult))"

# ============================================================
# Step 5: 构建+安装
# ============================================================
Write-Step 5 9 '构建+安装'
. (Join-Path $PSScriptRoot 'lib\build-deploy.ps1')
$deployResult = Invoke-BuildAndDeploy -CliPath $envResult.CliPath -Device $Device -ProjectDir $ProjectDir -Level $detectedLevel
if (-not $deployResult.Ok) {
    Write-Result $false "构建安装失败: $($deployResult.Error)"
    exit 1
}
Write-Result $true "已到设备（耗时 $($deployResult.Duration)s）"
if ($deployResult.ManifestRestored) { Write-Host "  📝 manifest.json 已还原" -ForegroundColor Yellow }
if ($deployResult.PagesRestored) { Write-Host "  📝 pages.json 已还原" -ForegroundColor Yellow }

# ============================================================
# Step 6: 自动截图（🟡🔴 模式）
# ============================================================
$screenshotResult = $null
$diffResult = $null
$evidenceResult = $null

if ($detectedLevel -ne 'quick') {
    Write-Step 6 9 '自动截图'
    . (Join-Path $PSScriptRoot 'lib\auto-screenshot.ps1')
    $screenshotResult = Invoke-AutoScreenshot -Device $Device -ProjectDir $ProjectDir
    if ($screenshotResult.Ok) {
        Write-Result $true "$($screenshotResult.Screenshots.Count)/$($screenshotResult.Screenshots.Count + $screenshotResult.Skipped.Count) 页面截图完成"
    } else {
        Write-Result $false "截图失败: $($screenshotResult.Error)"
    }
} else {
    Write-Step 6 9 '自动截图'
    Write-Host "  ⏭️ 快速模式，跳过截图" -ForegroundColor Gray
}

# ============================================================
# Step 7: 截图对比（🟡🔴 模式）
# ============================================================
if ($detectedLevel -ne 'quick') {
    Write-Step 7 9 '截图对比'
    . (Join-Path $PSScriptRoot 'lib\screenshot-diff.ps1')
    $diffResult = Compare-ScreenshotBaseline -ProjectDir $ProjectDir -UpdateBaseline:$UpdateBaseline
    if ($diffResult.NewBaseline) {
        Write-Result $true "首次运行，已建立基线"
    } elseif ($diffResult.ChangedCount -eq 0) {
        Write-Result $true "所有页面无变化"
    } else {
        Write-Host "  ⚠️ $($diffResult.ChangedCount) 个文件有变化" -ForegroundColor Yellow
    }
} else {
    Write-Step 7 9 '截图对比'
    Write-Host "  ⏭️ 快速模式，跳过对比" -ForegroundColor Gray
}

# ============================================================
# Step 8: 生成证据（🟡🔴 模式）
# ============================================================
if ($detectedLevel -ne 'quick') {
    Write-Step 8 9 '生成证据'
    . (Join-Path $PSScriptRoot 'lib\evidence-gen.ps1')
    $evidenceResult = New-Evidence -Level $detectedLevel `
        -CompileResult $compileResult.CompileResult `
        -ScreenshotDiff $diffResult.Diff `
        -ScreenshotChangedCount $diffResult.ChangedCount `
        -ProjectDir $ProjectDir
    if ($evidenceResult.Generated) {
        Write-Result $true "已写入 $($evidenceResult.Path)"
    } else {
        Write-Result $false "证据生成失败"
    }
} else {
    Write-Step 8 9 '生成证据'
    Write-Host "  ⏭️ 快速模式，无需写证据" -ForegroundColor Gray
    Write-Host "  📝 PR 验收证据段写：免（低风险运行时面：仅 .uvue 样式/文案改动）" -ForegroundColor Gray
}

# ============================================================
# Step 9: 还原配置文件
# ============================================================
Write-Step 9 9 '还原配置'
if ($deployResult.ManifestRestored -or $deployResult.PagesRestored) {
    Write-Result $true "配置文件已还原"
} else {
    Write-Result $true "配置文件未被改脏"
}

# ============================================================
# 摘要
# ============================================================
$totalSeconds = [int]((Get-Date) - $started).TotalSeconds
$minutes = [int]($totalSeconds / 60)
$seconds = $totalSeconds % 60

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  dev:finish 完成！总耗时 $minutes 分 $seconds 秒" -ForegroundColor Green
Write-Host "  级别: $levelIcon $detectedLevel" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

if ($evidenceResult -and $evidenceResult.Generated) {
    Write-Host ""
    Write-Host "📋 验收证据已生成，粘贴到 PR 正文的 ## 验收证据 段：" -ForegroundColor Cyan
    Write-Host $evidenceResult.Content
} elseif ($detectedLevel -eq 'quick') {
    Write-Host ""
    Write-Host "📝 快速模式：PR 验收证据段写「免（低风险运行时面：仅 .uvue 样式/文案改动）」" -ForegroundColor Gray
}

exit 0
