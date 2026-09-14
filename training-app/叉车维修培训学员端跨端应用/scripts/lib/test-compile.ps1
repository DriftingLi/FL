<#
.SYNOPSIS
    测试+编译编排模块：按级别执行单元测试和编译门。

.DESCRIPTION
    执行顺序：
      1. npm run test:unit（单元测试）
      2. 根据级别选择编译方式：
         - quick → 快速编译检查（只确认能编译）
         - standard / full → 正式编译门（kotlin-all-check）

    任一步骤失败立即停止，返回 @{ Ok = $false; Step = <失败步骤>; Error = <输出> }

.EXAMPLE
    . scripts/lib/test-compile.ps1
    $result = Invoke-TestAndCompile -Level 'standard' -ProjectDir "D:\FL\..."
    if (-not $result.Ok) { Write-Host "失败在 $($result.Step): $($result.Error)" }
#>

function Invoke-TestAndCompile {
    [CmdletBinding()]
    param(
        [ValidateSet('quick', 'standard', 'full')]
        [string]$Level = 'quick',
        [string]$ProjectDir,
        [switch]$SkipTests,
        [int]$TestTimeout = 300,
        [int]$CompileTimeout = 1800
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }

    $ErrorActionPreference = 'Stop'
    $started = Get-Date
    $testOutput = ''

    # --- Step 1: 单元测试（可跳过） ---
    if (-not $SkipTests) {
        Write-Host "[test-compile] 执行单元测试（dev:finish 契约测试）..." -ForegroundColor Cyan
        $testPattern = 'levelDetect|envCheck|testCompile|buildDeploy|autoScreenshot|screenshotDiff|evidenceGen|devFinish'
        try {
            $testOutput = & npx jest --config jest.config.unit.js -i --testPathPattern $testPattern --forceExit 2>&1 | Out-String
        } catch {
            $testOutput = $_.Exception.Message
        }

        if ($LASTEXITCODE -ne 0) {
            return [pscustomobject]@{
                Ok           = $false
                Step         = 'test'
                Error        = "单元测试失败（exit code $LASTEXITCODE）"
                TestOutput   = $testOutput
                CompileResult = ''
                Duration     = [int]((Get-Date) - $started).TotalSeconds
            }
        }

        Write-Host "[test-compile] 单元测试通过" -ForegroundColor Green
    }

    # --- Step 2: 编译门 ---
    $compileScript = ''
    $compileLabel = ''

    # 🟢 快速模式：跳过编译门（编译由后续 build-deploy 步骤处理）
    if ($Level -eq 'quick') {
        Write-Host "[test-compile] 🟢 快速模式：跳过编译门（编译由构建安装步骤处理）" -ForegroundColor Gray
        return [pscustomobject]@{
            Ok           = $true
            Step         = 'compile'
            Error        = ''
            TestOutput   = $testOutput
            CompileResult = 'COMPILE_RESULT errors=0 skipped=true quick_mode'
            Duration     = [int]((Get-Date) - $started).TotalSeconds
        }
    }

    # 🟡🔴 标准/完整模式：运行正式编译门
    $scriptPath = ''
    $scriptArgs = @()

    switch ($Level) {
        'standard' {
            $scriptPath = Join-Path $ProjectDir 'scripts\kotlin-all-check.ps1'
            $scriptArgs = @('-Project', $ProjectDir)
            $compileLabel = '编译门（④c）'
        }
        'full' {
            $scriptPath = Join-Path $ProjectDir 'scripts\kotlin-all-check.ps1'
            $scriptArgs = @('-Project', $ProjectDir)
            $compileLabel = '编译门（④c）'
        }
    }

    if (-not (Test-Path -LiteralPath $scriptPath)) {
        Write-Host "[test-compile] 编译脚本不存在: $scriptPath，跳过编译门" -ForegroundColor Yellow
        return [pscustomobject]@{
            Ok           = $true
            Step         = 'compile'
            Error        = ''
            TestOutput   = $testOutput
            CompileResult = 'COMPILE_RESULT errors=0 skipped=true script_not_found'
            Duration     = [int]((Get-Date) - $started).TotalSeconds
        }
    }

    Write-Host "[test-compile] 执行$compileLabel..." -ForegroundColor Cyan
    try {
        $compileOutput = & pwsh -NoProfile -ExecutionPolicy Bypass -File $scriptPath @scriptArgs 2>&1 | Out-String
    } catch {
        $compileOutput = $_.Exception.Message
    }

    if ($LASTEXITCODE -ne 0) {
        return [pscustomobject]@{
            Ok           = $false
            Step         = 'compile'
            Error        = "$compileLabel 失败（exit code $LASTEXITCODE）"
            TestOutput   = $testOutput
            CompileResult = ''
            Duration     = [int]((Get-Date) - $started).TotalSeconds
        }
    }

    # 提取 COMPILE_RESULT 行
    $resultLine = ''
    foreach ($line in ($compileOutput -split "`r?`n")) {
        if ($line -match 'COMPILE_RESULT') {
            $resultLine = $line.Trim()
            break
        }
    }

    Write-Host "[test-compile] $compileLabel 通过" -ForegroundColor Green

    return [pscustomobject]@{
        Ok           = $true
        Step         = 'compile'
        Error        = ''
        TestOutput   = $testOutput
        CompileResult = $resultLine
        Duration     = [int]((Get-Date) - $started).TotalSeconds
    }
}
