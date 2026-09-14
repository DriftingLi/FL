<#
.SYNOPSIS
    测试+编译编排模块：按级别执行单元测试与编译验证。

.DESCRIPTION
    **Q-2 口径（2026-09-14 用户裁定）**：
      · quick           → 调 `hx-run.ps1 -CompileOnly` 拿**编译期诊断**
                          （官方语义的「仅编译」：不推送 / 不启动 / 不轮询 / **不需要设备**，
                            且该调用会自己收口、不留常驻会话）
      · standard / full → 调 `kotlin-all-check.ps1`（④c 整模块编译门）

    旧口径让 quick **直接跳过这一步**（等于不做任何编译验证）—— **已废止**。
    quick 与 standard 的差别现在是「**诊断强度**」，而不是「做不做」。

    判成败：quick 取 `hx-run.ps1 -CompileOnly` 的**退出码**（0 = 无诊断行 / 1 = 有诊断行 / 2 = 环境不可用）；
    standard/full 取 `kotlin-all-check.ps1` 的退出码并解析其 `COMPILE_RESULT` 行。

    **fail-closed**：`$ok` 默认 `$false`；环境不可用（exit 2）与诊断非空（exit 1）都判 `Ok=$false`。

.EXAMPLE
    . scripts/lib/test-compile.ps1
    $r = Invoke-TestAndCompile -Level 'quick' -ProjectDir 'D:\FL\...' -SkipTests
#>

function Invoke-TestAndCompile {
    [CmdletBinding()]
    param(
        [ValidateSet('quick', 'standard', 'full')]
        [string]$Level = 'quick',
        [string]$ProjectDir,
        [string]$CliPath,
        [string]$Device,
        [switch]$SkipTests,
        # Q-2 修正（2026-09-14 用户裁定）：quick 默认只做**静态守护（Q-A）**，
        # 需要编译期诊断时由调用方显式开启本开关（Q-B）。
        # 理由：实测本项目 compile-only 在冷/失效缓存下 **>901 秒**，比真运行（4–5 分钟）还慢 ⇒
        # 把「编译」塞进默认的 quick 路径等于谎称「快速」。
        [switch]$QuickCompile,
        [int]$TestTimeout = 300,
        # hx-run 的 `-TimeoutSeconds`：CompileOnly 模式下它是「仅编译」那一步的超时。
        # 默认 1800（30 分钟）而非 hx-run 自身的 900 —— 2026-09-14 实测本项目
        # `compile-only` 跑到 **901 秒仍在大面积编译**（48 页工程、冷缓存），900 秒会**过早判环境不可用**。
        [int]$HxRunTimeoutSeconds = 1800
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }

    $ErrorActionPreference = 'Stop'
    $started = Get-Date

    # fail-closed 默认值
    $ok = $false
    $step = ''
    $error = ''
    $testOutput = ''
    $compileResult = ''

    # ================= Step 1: 单元测试（默认只跑本工具链的契约测试） =================
    if (-not $SkipTests) {
        Write-Host '[test-compile] 执行单元测试（dev:finish 契约测试）...' -ForegroundColor Cyan
        # #974：把**部署判定的时序守护**也纳入 Q-A —— `hxRun`（C1–C14，文本层）与
        # `hxTimingBehavior`（运行期）。一个 dev:finish **永远不跑**的守护等于空跑，
        # 正是本仓反复踩过的假绿形态（守护 T13）。
        # ADR-0012：再加 `hxError` —— 覆盖**错误行判据**的运行期守护 `hxErrorLinesBehavior`。
        # ⚠️ 注意 `--testPathPattern` 是**子串**匹配：`hxRun` **匹配不到** `hxErrorLinesBehavior`
        #    （已用 `jest --listTests` 实证），故必须显式加这个 token，否则该守护**永不执行**。
        $testPattern = 'levelDetect|envCheck|testCompile|buildDeploy|autoScreenshot|screenshotDiff|evidenceGen|devFinish|hxBusyGate|hxRun|hxTimingBehavior|hxError'
        try {
            $testOutput = & npx jest --config jest.config.unit.js -i --testPathPattern $testPattern --forceExit 2>&1 | Out-String
        }
        catch {
            $testOutput = "$($_.Exception.Message)"
        }

        if ($LASTEXITCODE -ne 0) {
            return [pscustomobject]@{
                Ok            = $false
                Step          = 'test'
                Error         = "单元测试失败（exit code $LASTEXITCODE）"
                TestOutput    = $testOutput
                CompileResult = ''
                Duration      = [int]((Get-Date) - $started).TotalSeconds
            }
        }
        Write-Host '[test-compile] 单元测试通过' -ForegroundColor Green
    }

    # ================= Step 2: 按级别做编译验证 =================
    if ($Level -eq 'quick' -and -not $QuickCompile) {
        # 🟢 Q-A 静态守护：契约测试已在 Step 1 跑过；**显式声明未做编译诊断**
        # （不是静默跳过 —— 静默跳过才是旧口径被废止的原因）
        Write-Host '[test-compile] 🟢 Q-A 静态守护：契约测试已通过；未做编译诊断（需要编译请加 -Compile 或 -Level standard）' -ForegroundColor Gray
        return [pscustomobject]@{
            Ok            = $true
            Step          = 'compile'
            Error         = ''
            TestOutput    = $testOutput
            CompileResult = 'STATIC_ONLY skipped=true quick_static_only（未做编译诊断）'
            Duration      = [int]((Get-Date) - $started).TotalSeconds
        }
    }

    if ($Level -eq 'quick') {
        # 🟢 Q-B 编译诊断：hx-run -CompileOnly（不推送 / 不启动 / 不需要设备）
        # 成本写实：冷/失效缓存下实测 >901 秒 —— 这是调用方显式要的，不是默认。
        $hxRun = Join-Path $ProjectDir 'scripts\hx-run.ps1'
        if (-not (Test-Path -LiteralPath $hxRun)) {
            return [pscustomobject]@{
                Ok            = $false
                Step          = 'compile'
                Error         = "找不到 hx-run.ps1：$hxRun"
                TestOutput    = $testOutput
                CompileResult = ''
                Duration      = [int]((Get-Date) - $started).TotalSeconds
            }
        }

        Write-Host '[test-compile] 执行编译期诊断（hx-run -CompileOnly）...' -ForegroundColor Cyan
        $extra = @('-Project', $ProjectDir, '-CompileOnly', '-TimeoutSeconds', "$HxRunTimeoutSeconds")
        if ($CliPath) { $extra += @('-Cli', $CliPath) }
        try {
            $compileOutput = & pwsh -NoProfile -ExecutionPolicy Bypass -File $hxRun @extra 2>&1 | Out-String
        }
        catch {
            $compileOutput = "$($_.Exception.Message)"
        }
        $compileExit = $LASTEXITCODE

        # 解析 hx-run 机检行
        foreach ($line in ($compileOutput -split "`r?`n")) {
            if ($line -match '^HX_RUN\s+mode=') { $compileResult = $line.Trim() }
        }

        # 落盘便于排障
        $logDir = Join-Path $ProjectDir '.ci-verify'
        New-Item -ItemType Directory -Force -Path $logDir | Out-Null
        Add-Content -LiteralPath (Join-Path $logDir 'test-compile.log') -Encoding utf8 `
            -Value "`n# ---- hx-run -CompileOnly exit=$compileExit ----`n$compileOutput" -ErrorAction SilentlyContinue

        switch ($compileExit) {
            0 { $ok = $true }
            1 {
                $ok = $false; $step = 'compile'
                $error = '编译期诊断非空（hx-run -CompileOnly exit=1）'
            }
            default {
                $ok = $false; $step = 'compile'
                $error = "环境不可用（hx-run -CompileOnly exit=$compileExit）"
            }
        }

        if ($ok) { Write-Host '[test-compile] 编译期诊断为空（无 error）' -ForegroundColor Green }
    }
    else {
        # 🟡🔴 标准/完整：kotlin-all-check.ps1（④c 整模块编译门）
        $kotlinScript = Join-Path $ProjectDir 'scripts\kotlin-all-check.ps1'
        if (-not (Test-Path -LiteralPath $kotlinScript)) {
            return [pscustomobject]@{
                Ok            = $false
                Step          = 'compile'
                Error         = "找不到 kotlin-all-check.ps1：$kotlinScript"
                TestOutput    = $testOutput
                CompileResult = ''
                Duration      = [int]((Get-Date) - $started).TotalSeconds
            }
        }

        Write-Host '[test-compile] 执行编译门（④c 整模块）...' -ForegroundColor Cyan
        $extra = @('-Project', $ProjectDir)
        if ($CliPath) { $extra += @('-Cli', $CliPath) }
        try {
            $compileOutput = & pwsh -NoProfile -ExecutionPolicy Bypass -File $kotlinScript @extra 2>&1 | Out-String
        }
        catch {
            $compileOutput = "$($_.Exception.Message)"
        }
        $compileExit = $LASTEXITCODE

        foreach ($line in ($compileOutput -split "`r?`n")) {
            if ($line -match 'COMPILE_RESULT') { $compileResult = $line.Trim(); break }
        }

        $logDir = Join-Path $ProjectDir '.ci-verify'
        New-Item -ItemType Directory -Force -Path $logDir | Out-Null
        Add-Content -LiteralPath (Join-Path $logDir 'test-compile.log') -Encoding utf8 `
            -Value "`n# ---- kotlin-all-check exit=$compileExit ----`n$compileOutput" -ErrorAction SilentlyContinue

        if ($compileExit -eq 0) {
            $ok = $true
            Write-Host '[test-compile] 编译门通过（④c）' -ForegroundColor Green
        }
        else {
            $ok = $false; $step = 'compile'
            $error = "编译门未过（kotlin-all-check exit=$compileExit）"
        }
    }

    return [pscustomobject]@{
        Ok            = $ok
        Step          = 'compile'
        Error         = $error
        TestOutput    = $testOutput
        CompileResult = $compileResult
        Duration      = [int]((Get-Date) - $started).TotalSeconds
    }
}
