<#
.SYNOPSIS
    一键完成：编译 → 部署 → 截图 → 对比 → 生成证据。

.DESCRIPTION
    按改动复杂度自动判定级别（🟢快速 / 🟡标准 / 🔴完整），执行对应流程：

      🟢 快速（默认 **Q-A 静态守护**）：契约测试已跑；**秒级**、不取锁、不占设备、**未做编译诊断**
      🟢 快速 `-Compile`（**Q-B 编译诊断**）：追加 `hx-run -CompileOnly`；**热缓存 257 秒**，
         **冷 / 上次被打断后 826–901+ 秒**（2026-09-14 两次实测，48 页工程）
         —— **勿再引用旧口径「约 3 分钟起 / 冷 >15 分钟」**，两者都已被实测推翻
      🟡 标准：④c 编译门 + 真运行到设备 + 只截改动页 + 简要证据
      🔴 完整：④c 编译门 + 真运行到设备 + 只截改动页 + 完整证据

    **Q-2 修正（2026-09-14）**：原本让 🟢 默认跑 `hx-run -CompileOnly` 当「快速」，但**实测推翻了前提** ——
    冷 / 失效缓存下 compile-only **>901 秒**，**比真运行（4–5 分钟）还慢**。故 🟢 默认降级为 **Q-A 静态守护**
    （秒级、不取锁、不占设备），编译诊断改为**显式 `-Compile`**。要看真机效果请用 🟡，或用 HBuilderX GUI 热刷新（秒级）。
    **Q-A 必须明说「未做编译诊断」** —— 静默跳过正是最早那版「什么都没做却报成功」的假绿。

    **A-3 临界区**：HBuilderX 是**单实例串行资源**，而本脚本有**三个**步骤需要它（编译 / 部署 / 截图），
    且后两步**共享设备状态**。若各步各拿一次锁，别的会话可能在「部署到目标页」与「截图」之间插进来
    launch 到别的页 ⇒ **截到别人的页面**。故改为**一个临界区覆盖步骤 4–6**。
    **Q-A 根本不进临界区**（它不使用主程序）—— 否则别的会话持锁时，秒级路径会白等到超时。

    **锁交接**：`hx-busy.ps1` 的锁**按 PID 判定且不可重入** ⇒ 父进程持锁后再调子脚本（子进程 PID 不同）
    会**自死锁**。故持锁后调 `Set-HxLockOwnerEnv` 写 `$env:HX_LOCK_OWNER`；子脚本的**取锁函数**
    认到「同一持有者」就复用、不再加锁。失败/超时路径经 `finally` 释放锁并清理 env。

    **并发会话会排队**（不是冲突）：等待上限 `-HxWaitSeconds`（默认 1800 秒，因临界区较长），
    超时 ⇒ `exit 2`（环境不可用），**绝不抢占、绝不 kill 主程序**。真机步骤请**集中到一个会话**执行。

.EXAMPLE
    npm run dev:finish
    npm run dev:finish -- -Level standard -Device 192.168.10.51:39181
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
    # Q-2 修正（2026-09-14 用户裁定）：🟢 quick **默认只做静态守护（Q-A）**，秒级、不占设备、不取锁。
    # 需要编译期诊断时显式加本开关 ⇒ Q-B（hx-run -CompileOnly；冷/失效缓存下实测 >901 秒，成本写实）。
    [switch]$Compile,
    [int]$MaxScreenshotPages = 5,
    [int]$HxWaitSeconds = 1800,
    # 透传给 hx-run 的超时（编译段 / 部署轮询）。2026-09-14 实测本项目冷缓存下
    # 编译可超 900 秒 ⇒ 默认取 1800，避免把「慢」误判成「环境不可用」。
    [int]$HxRunTimeoutSeconds = 1800
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$started = Get-Date

# ============================================================
# 辅助
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
# DryRun：只打印计划
# ============================================================
if ($DryRun) {
    Write-Host "=== dev:finish 计划（-DryRun：不执行任何操作）===" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "步骤："
    Write-Host "  1. 级别判定（根据 git diff）"
    Write-Host "  2. 环境检测（Q-A 只验 git+项目根；其余验设备 + HBuilderX cli）"
    Write-Host "  3. 单元测试（本工具链契约测试）"
    Write-Host "  ── 以下 4-6 仅在实际需要 HBuilderX 时才进入锁临界区（Q-A 直接跳过）──"
    Write-Host "  4. 编译：quick 默认 **不编译**（Q-A）；-Compile 或 standard/full 才编译"
    Write-Host "  5. 部署：quick → 不部署；standard/full → hx-run 真运行"
    Write-Host "  6. 截图：只截改动页面（🟡🔴；最多 $MaxScreenshotPages 页）"
    Write-Host "  ── 临界区结束，锁已释放 ──"
    Write-Host "  7. 截图对比基线（🟡🔴）"
    Write-Host "  8. 生成验收证据（🟡🔴）"
    Write-Host "  9. 还原 manifest.json / pages.json"
    Write-Host ""
    Write-Host "参数："
    Write-Host "  -Device:              ${Device:-<自动检测>}"
    Write-Host "  -Level:               ${Level:-<自动判定>}"
    Write-Host "  -Compile:             $Compile   # quick 下是否做编译期诊断（Q-B）"
    Write-Host "  -Distribute:          $Distribute"
    Write-Host "  -UpdateBaseline:      $UpdateBaseline"
    Write-Host "  -MaxScreenshotPages:  $MaxScreenshotPages"
    Write-Host "  -HxWaitSeconds:       $HxWaitSeconds"
    Write-Host "  -HxRunTimeoutSeconds: $HxRunTimeoutSeconds"
    exit 0
}

# ============================================================
# Step 1: 级别判定
#   提前到环境检测之前 —— 「要不要 HBuilderX / 设备」取决于级别与 -Compile，
#   而 Q-A（quick 且未 -Compile）**既不需要主程序也不需要设备**，若仍走
#   Test-BuildEnv 就会去取 HBuilderX 互斥锁 ⇒ 别的会话持锁时「秒级」变「等 120 秒后 exit 2」。
# ============================================================
Write-Step 1 9 '级别判定'
. (Join-Path $PSScriptRoot 'lib\level-detect.ps1')
$levelResult = Get-DetectLevel -ProjectDir $ProjectDir -ForceLevel $Level
$detectedLevel = $levelResult.Level
$levelIcon = switch ($detectedLevel) { 'quick' { '🟢' } 'standard' { '🟡' } 'full' { '🔴' } }
Write-Result $true "$levelIcon $detectedLevel — $($levelResult.Reason)"

# 本次是否需要 HBuilderX / 设备（Q-A 不需要）
$needsHx = ($detectedLevel -ne 'quick') -or $Compile
$quickTier = ''
if ($detectedLevel -eq 'quick') {
    $quickTier = if ($Compile) { 'Q-B 编译诊断' } else { 'Q-A 静态守护' }
}

# ============================================================
# Step 2: 环境检测（按需求分级）
# ============================================================
Write-Step 2 9 '环境检测'
. (Join-Path $PSScriptRoot 'lib\env-check.ps1')
if ($needsHx) {
    $envResult = Test-BuildEnv -ProjectDir $ProjectDir -Device $Device -HxWaitSeconds 120
    if (-not $envResult.Ok) {
        Write-Result $false $envResult.Error
        exit 2
    }
    Write-Result $true "设备 $($envResult.Device) 在线，HBuilderX cli 可用"
    $Device = $envResult.Device
}
else {
    $envResult = Test-StaticEnv -ProjectDir $ProjectDir
    if (-not $envResult.Ok) {
        Write-Result $false $envResult.Error
        exit 2
    }
    Write-Result $true "$quickTier：只验 git + 项目根（不取锁、不要求设备）"
}

# ============================================================
# Step 3: 单元测试
# ============================================================
Write-Step 3 9 '单元测试'
# ⚠️ pattern 取自**唯一真源** `lib\contract-tests.ps1`，本文件**不得**再抄一份自己的字面量
#    （曾有第二份、且与 test-compile 的那份漂移：少了 hxRun/hxTimingBehavior/hxError，
#     而步骤 3 先跑且失败即 exit 1 ⇒ 实际门禁是**更窄的那份**）。守护：contractTestPatternBehavior.test.js
. (Join-Path $PSScriptRoot 'lib\contract-tests.ps1')
$testPattern = Get-ContractTestPattern
try {
    $testOutput = & npx jest --config jest.config.unit.js -i --testPathPattern $testPattern --forceExit 2>&1 | Out-String
}
catch {
    $testOutput = "$($_.Exception.Message)"
}
if ($LASTEXITCODE -ne 0) {
    Write-Result $false "单元测试失败（exit code $LASTEXITCODE）"
    exit 1
}
Write-Result $true '单元测试通过'

# ============================================================
# 临界区：步骤 4-6 一次持锁（A-3）
#   HBuilderX 是单实例串行资源；且部署与截图共享设备状态，
#   中途被别的会话 launch 插进来会截到别人的页面。
# ============================================================
. (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
$hxLog = Join-Path $ProjectDir '.ci-verify\dev-finish.log'
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $hxLog) | Out-Null

Write-Host ''
if ($needsHx) {
    Write-Host '>>> 进入 HBuilderX 锁临界区（步骤 4-6）' -ForegroundColor DarkGray
    # 取锁：忙就等（上限 $HxWaitSeconds），超时 ⇒ exit 2 且**不抢占、不 kill**
    $null = Wait-HxFree -CliExe $envResult.CliPath -TimeoutSeconds $HxWaitSeconds -LogPath $hxLog
    # 交接给子脚本（子进程会继承该 env；父持锁时子脚本不再自己加锁 ⇒ 避免自死锁）
    Set-HxLockOwnerEnv
}
else {
    # Q-A 不碰 HBuilderX ⇒ **不取锁**（否则别的会话持锁时这里会白等，把秒级变分钟级）
    Write-Host ">>> $quickTier：不取 HBuilderX 锁（本路径不使用主程序）" -ForegroundColor DarkGray
}

$screenshotResult = $null
$deployResult = $null

try {
    # ===== Step 4: 编译 =====
    Write-Step 4 9 '编译'
    . (Join-Path $PSScriptRoot 'lib\test-compile.ps1')
    $compileResult = Invoke-TestAndCompile -Level $detectedLevel -ProjectDir $ProjectDir -CliPath $envResult.CliPath -SkipTests -QuickCompile:$Compile -HxRunTimeoutSeconds $HxRunTimeoutSeconds
    if (-not $compileResult.Ok) {
        Write-Result $false "$($compileResult.Step) 失败: $($compileResult.Error)"
        exit 1
    }
    if ($detectedLevel -eq 'quick' -and -not $Compile) {
        Write-Result $true "$quickTier：契约测试已过；**未做编译诊断**（需要编译请加 -Compile）"
    }
    elseif ($detectedLevel -eq 'quick') {
        Write-Result $true "编译期诊断为空（hx-run -CompileOnly，未占设备）"
    }
    else {
        Write-Result $true "编译门通过（$($compileResult.CompileResult)）"
    }

    # ===== Step 5: 部署 =====
    Write-Step 5 9 '部署'
    . (Join-Path $PSScriptRoot 'lib\build-deploy.ps1')
    $deployResult = Invoke-BuildAndDeploy -CliPath $envResult.CliPath -Device $Device -ProjectDir $ProjectDir -Level $detectedLevel -RunTimeoutSeconds $HxRunTimeoutSeconds
    if (-not $deployResult.Ok) {
        Write-Result $false "部署失败: $($deployResult.Error)"
        exit 1
    }
    if ($deployResult.Deployed) {
        Write-Result $true "已到设备（耗时 $($deployResult.Duration)s）"
    }
    else {
        Write-Host "  ⏭️ 未部署：$($deployResult.Reason)" -ForegroundColor Gray
    }
    if ($deployResult.ManifestRestored) { Write-Host '  📝 manifest.json 已还原' -ForegroundColor Yellow }
    if ($deployResult.PagesRestored) { Write-Host '  📝 pages.json 已还原' -ForegroundColor Yellow }

    # ===== Step 6: 截图 =====
    Write-Step 6 9 '自动截图'
    if ($detectedLevel -eq 'quick') {
        Write-Host '  ⏭️ 快速模式未部署到设备，截图无意义 ⇒ 跳过' -ForegroundColor Gray
    }
    else {
        . (Join-Path $PSScriptRoot 'lib\auto-screenshot.ps1')
        $screenshotResult = Invoke-AutoScreenshot -Device $Device -CliPath $envResult.CliPath `
            -ProjectDir $ProjectDir -MaxPages $MaxScreenshotPages
        if ($screenshotResult.Ok) {
            Write-Result $true "$($screenshotResult.Screenshots.Count) 页截图完成（$($screenshotResult.Screenshots -join ', ')）"
        }
        else {
            Write-Result $false "截图未通过: $($screenshotResult.Error)"
            exit 1
        }
    }
} finally {
    # 收尾**无条件**执行：断言失败 / exit / 异常都走这里
    # 只有真取过锁的路径才需要释放（Q-A 从未取锁，Release-HxLock 本就是 no-op，但仍显式分流以免误读）
    if ($needsHx) {
        Clear-HxLockOwnerEnv
        Release-HxLock
        Write-Host '>>> 已退出锁临界区' -ForegroundColor DarkGray
    }
}

# ============================================================
# Step 7: 截图对比（🟡🔴）
# ============================================================
$diffResult = $null
$evidenceResult = $null

Write-Step 7 9 '截图对比'
if ($detectedLevel -eq 'quick') {
    Write-Host '  ⏭️ 快速模式，跳过对比' -ForegroundColor Gray
}
else {
    . (Join-Path $PSScriptRoot 'lib\screenshot-diff.ps1')
    $diffResult = Compare-ScreenshotBaseline -ProjectDir $ProjectDir -UpdateBaseline:$UpdateBaseline
    if ($diffResult.NewBaseline) {
        # ⚠️ 不许再无条件写「已建立基线」：`Compare-ScreenshotBaseline` 只在 **-UpdateBaseline** 时才把本轮
        #    截图拷进基线目录，否则只建了个**空目录**。原文案会把「空基线」说成「已建立」（2026-09-15 实测：
        #    步骤 7 通过但下次运行仍会报「新增」）。文案据实分流。
        if ($UpdateBaseline) {
            Write-Result $true '首次运行，已建立基线（本轮截图已写入基线）'
        }
        else {
            Write-Result $true '首次运行：基线目录为空，**未**建立基线（要以此为本轮基线请加 -UpdateBaseline）'
        }
    }
    elseif ($diffResult.ChangedCount -eq 0) {
        Write-Result $true '所有页面无变化'
    }
    else {
        Write-Host "  ⚠️ $($diffResult.ChangedCount) 个文件有变化" -ForegroundColor Yellow
    }
}

# ============================================================
# Step 8: 生成证据（🟡🔴）
# ============================================================
Write-Step 8 9 '生成证据'
if ($detectedLevel -eq 'quick') {
    Write-Host '  ⏭️ 快速模式，无需写证据' -ForegroundColor Gray
}
else {
    . (Join-Path $PSScriptRoot 'lib\evidence-gen.ps1')
    $evidenceResult = New-Evidence -Level $detectedLevel `
        -CompileResult $compileResult.CompileResult `
        -ScreenshotDiff $diffResult.Diff `
        -ScreenshotChangedCount $diffResult.ChangedCount `
        -ProjectDir $ProjectDir
    if ($evidenceResult.Generated) {
        Write-Result $true "已写入 $($evidenceResult.Path)"
    }
    else {
        Write-Result $false '证据生成失败'
    }
}

# ============================================================
# Step 9: 还原配置
# ============================================================
Write-Step 9 9 '还原配置'
if ($deployResult -and ($deployResult.ManifestRestored -or $deployResult.PagesRestored)) {
    Write-Result $true '配置文件已还原'
}
else {
    Write-Result $true '配置文件未被改脏'
}

# ============================================================
# 摘要
# ============================================================
$totalSeconds = [int]((Get-Date) - $started).TotalSeconds
$minutes = [int]($totalSeconds / 60)
$seconds = $totalSeconds % 60

Write-Host ''
Write-Host '========================================' -ForegroundColor Green
Write-Host "  dev:finish 完成！总耗时 $minutes 分 $seconds 秒" -ForegroundColor Green
Write-Host "  级别: $levelIcon $detectedLevel" -ForegroundColor Green
if ($deployResult) { Write-Host "  部署: $(if ($deployResult.Deployed) { '已到设备' } else { '未部署' })" -ForegroundColor Green }
Write-Host '========================================' -ForegroundColor Green

if ($evidenceResult -and $evidenceResult.Generated) {
    Write-Host ''
    Write-Host '📋 验收证据已生成，粘贴到 PR 正文的 ## 验收证据 段：' -ForegroundColor Cyan
    Write-Host $evidenceResult.Content
}
elseif ($detectedLevel -eq 'quick') {
    Write-Host ''
    Write-Host '📝 快速模式：PR 验收证据段写「免（低风险运行时面：仅 .uvue 样式/文案改动）」' -ForegroundColor Gray
}

exit 0
