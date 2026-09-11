<#
.SYNOPSIS
    移动端验收门 ④a：HBuilderX 全量编译（dev 编译）—— 半自动编译门。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md：第 ④a 门 = 「可复现命令 + 可核验日志」，
    证据产物固定为 <项目>/.ci-verify/build.log（被 .gitignore 的 *.log 覆盖，不入库）。

    三件必须知道的事：
      1. HBuilderX CLI 是**驱动主程序**的，不是独立无头工具：必须先 `cli open` 且主程序可达；
         若在受限会话（禁命名管道/本地套接字）里跑，会得到「与主程序的连接已中断」。
      2. CLI **失败时退出码恒为 0**（实测四处失败全返回 0），所以本脚本一律**解析 stdout** 判定成败，
         绝不看退出码。
      3. CLI 在连不上主程序时会**挂住不返回**（实测 >2 分钟无输出），所以每一步都有超时兜底。

    退出码：0 = 编译门通过；1 = 发现 error 行（编译失败）；2 = 环境不可用（cli 缺失/连不上/超时）。

.PARAMETER Cli
    cli.exe 路径。**显式给出即权威**：不可用就报错退出，不再回退自动探测。
    缺省时按 $env:HBuilderX_CLI → 常见安装位置探测（校验 PE 头，跳过损坏副本）。

.PARAMETER Project
    项目根目录。缺省为本脚本上一级目录（scripts/ 位于项目根下）。

.PARAMETER Clean
    是否 `--cleanCache true` 强制干净重建（默认 true，即真正的「全量编译」）。

.PARAMETER TimeoutSeconds
    编译步骤的超时秒数（默认 1800）。

.EXAMPLE
    npm run build:compile
    pwsh -NoProfile -File scripts/compile-check.ps1 -Clean:$false -TimeoutSeconds 900
#>
[CmdletBinding()]
param(
    [string]$Cli,
    [string]$Project,
    [bool]$Clean = $true,
    [int]$TimeoutSeconds = 1800
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Test-PeHeader {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $fs = [System.IO.File]::OpenRead($Path)
        try {
            $buf = New-Object byte[] 2
            $read = $fs.Read($buf, 0, 2)
        } finally {
            $fs.Dispose()
        }
    } catch {
        return $false
    }
    # 'MZ'：E:\HBuilderX 那份残留副本读回来前 8 字节全为 00（文件损坏），必须排除
    return ($read -eq 2 -and $buf[0] -eq 0x4D -and $buf[1] -eq 0x5A)
}

function Find-CliPath {
    # 1) 优先用**正在运行的 HBuilderX** 同目录下的 cli.exe —— 与主程序版本必然匹配
    $proc = $null
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
    } catch {
        $proc = $null
    }
    if ($proc) {
        try {
            $runningCli = Join-Path (Split-Path $proc.Path) 'cli.exe'
            if (Test-PeHeader -Path $runningCli) { return (Resolve-Path -LiteralPath $runningCli).Path }
        } catch {
            # 进程路径读不到（权限）就继续走位置探测
        }
    }

    $found = @()
    foreach ($pattern in @(
            'D:\软件\HBuilderX*\HBuilderX\cli.exe',
            'D:\HBuilderX*\HBuilderX\cli.exe',
            'C:\Program Files\HBuilderX*\HBuilderX\cli.exe',
            'C:\Program Files\HBuilderX\cli.exe',
            "$env:LOCALAPPDATA\HBuilderX\cli.exe")) {
        $found += (Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | ForEach-Object { $_.FullName })
    }
    foreach ($c in $found) {
        if (Test-PeHeader -Path $c) { return (Resolve-Path -LiteralPath $c).Path }
        Write-Host "[warn] 跳过不可用的 cli.exe 候选：$c（缺失或 PE 头损坏）" -ForegroundColor Yellow
    }
    return $null
}

# ---------- 定位项目与 CLI ----------

if (-not $Project) {
    $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
}
if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}

$explicitCli = $Cli
if (-not $explicitCli) { $explicitCli = $env:HBuilderX_CLI }
if ($explicitCli) {
    # 显式给出即权威：不可用直接失败，避免「以为用的是这个、其实用了另一个」
    if (-not (Test-PeHeader -Path $explicitCli)) {
        Write-Host "[error] 指定的 cli.exe 不可用（不存在或 PE 头损坏）：$explicitCli" -ForegroundColor Red
        exit 2
    }
    $CliPath = (Resolve-Path -LiteralPath $explicitCli).Path
} else {
    $CliPath = Find-CliPath
}
if (-not $CliPath) {
    Write-Host '[error] 找不到可用的 HBuilderX cli.exe。用 -Cli <路径> 或设 $env:HBuilderX_CLI 指定。' -ForegroundColor Red
    exit 2
}

$logDir = Join-Path $Project '.ci-verify'
$logPath = Join-Path $logDir 'build.log'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$started = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
Set-Content -LiteralPath $logPath -Value "# compile-check 开始 $started`n# cli = $CliPath`n# project = $Project`n# clean = $Clean" -Encoding utf8

$script:timedOutStep = $null

function Invoke-CliStep {
    param([string[]]$CliArgs, [int]$StepTimeout = 180)
    $display = "cli.exe $($CliArgs -join ' ')"
    Add-Content -LiteralPath $logPath -Value "`n>>> $display" -Encoding utf8
    Write-Host ">>> $display"

    $argLine = ($CliArgs | ForEach-Object { if ($_ -match '\s') { '"' + $_ + '"' } else { $_ } }) -join ' '
    $stamp = [guid]::NewGuid().ToString('N')
    $outFile = Join-Path $logDir "step-$stamp.out"
    $errFile = Join-Path $logDir "step-$stamp.err"

    $proc = Start-Process -FilePath $CliPath -ArgumentList $argLine -NoNewWindow -PassThru `
        -RedirectStandardOutput $outFile -RedirectStandardError $errFile
    $killed = $false
    try {
        Wait-Process -Id $proc.Id -Timeout $StepTimeout -ErrorAction Stop
    } catch {
        $killed = $true
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }

    $out = ''
    if (Test-Path -LiteralPath $outFile) { $out += (Get-Content -LiteralPath $outFile -Raw) }
    if (Test-Path -LiteralPath $errFile) { $out += (Get-Content -LiteralPath $errFile -Raw) }
    Remove-Item -LiteralPath $outFile, $errFile -Force -ErrorAction SilentlyContinue

    if ($killed) {
        $out += "`n[timeout] 该步超过 $StepTimeout 秒未返回，已终止（HBuilderX 主程序可能不可达）"
        $script:timedOutStep = $display
    }
    Add-Content -LiteralPath $logPath -Value $out -Encoding utf8
    Write-Host $out
    return $out
}

Write-Host "cli   = $CliPath"
Write-Host "proj  = $Project"
Write-Host "log   = $logPath"

Invoke-CliStep @('open') -StepTimeout 180 | Out-Null
Invoke-CliStep @('project', 'open', '--path', $Project) -StepTimeout 180 | Out-Null

$launchArgs = @('launch', 'app-android', '--project', $Project, '--compile', 'true')
if ($Clean) { $launchArgs += @('--cleanCache', 'true') }
Invoke-CliStep $launchArgs -StepTimeout $TimeoutSeconds | Out-Null

$all = Get-Content -LiteralPath $logPath -Raw

if ($script:timedOutStep -or $all -match '与主程序的连接已中断|启动超时') {
    Write-Host ''
    Write-Host '[error] HBuilderX CLI 连不上主程序（见日志）：请在普通终端里先手动启动 HBuilderX 再重试；' -ForegroundColor Red
    Write-Host '        受限/沙箱会话会阻断 CLI↔主程序的本地 IPC，CLI 在这种情况下会挂住或报「连接已中断」。' -ForegroundColor Red
    Add-Content -LiteralPath $logPath -Value "`nCOMPILE_RESULT errors=env log=$logPath" -Encoding utf8
    exit 2
}

$errorLines = @(Get-Content -LiteralPath $logPath |
    Where-Object { $_ -match '(?i)\berror\b|unresolved reference|cannot infer type|找不到名称|类型不匹配|编译失败' } |
    Where-Object { $_ -notmatch '(?i)0\s*error|errors?\s*[:=]\s*0|no errors?|error count\s*[:=]\s*0' })

$result = "COMPILE_RESULT errors=$($errorLines.Count) clean=$Clean log=$logPath"
Add-Content -LiteralPath $logPath -Value "`n$result" -Encoding utf8

if ($errorLines.Count -gt 0) {
    Write-Host ''
    Write-Host "❌ 编译门未过：$($errorLines.Count) 行含 error/编译错误 ——" -ForegroundColor Red
    $errorLines | Select-Object -First 40 | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
    Write-Host "完整日志：$logPath" -ForegroundColor Yellow
    Write-Host "把 $result 与关键行粘进 PR 的「验收证据」④a 行。" -ForegroundColor Yellow
    exit 1
}

Write-Host ''
Write-Host "✅ 编译门通过：未发现 error 行（$result）" -ForegroundColor Green
Write-Host '   把这一行与日志尾部若干行粘进 PR 的「验收证据」④a 行。' -ForegroundColor Green
exit 0
