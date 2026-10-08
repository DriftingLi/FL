#Requires -Version 7
<#
.SYNOPSIS
    #1568 AC 第 2 条第二片的**真机**腿驱动：对 Redmi（Android 15）跑 device-capture 的三条腿。
.DESCRIPTION
    为什么要有这份件（补的是哪一格）：本片的仿真机三腿（`device-capture-emulator-legs.ps1`）能证
    「真 adb 的答复换成落盘读回 + `-MergeStdErr` 后仍被原有解析器读懂」与「到点给可见结论」，
    但 PR #1576 与移动端 ADR-0008 都把一句话登记成**没证到**：厂商 ROM 的 `dumpsys` 字段差异
    （Android 14+ 只报 `topResumedActivity`）与真会话的 logcat 体量。这一份就是去量那两格。

    三条腿与判据（与仿真机腿同一套形状，只换载体）：
      L1 默认预算            ⇒ 期望 exit=0 / RESULT=PASS / `ADB_TEXT_CALL_BUDGET … timeouts=0`
                              / `LOGCAT_INCONCLUSIVE=0`，且前台 Activity 从 **topResumedActivity** 解析出来
      L2 只给 logcat 档 0    ⇒ 期望 exit=1 / 两条 `ADB_TEXT_TIMEOUT tier=logcat` / `LOGCAT_INCONCLUSIVE=1`
                              / RESULT=**FAIL**（不许读成「无崩溃」的 PASS）
      L3 只给文本档 0        ⇒ 期望 exit=2 / `ADB_TEXT_TIMEOUT call=adb devices -l … tier=server`
                              （argv 里**没有 -s**：server 级形态在真机上成立）

    ⚠️ 三条腿全部**只读**：不给 `-AllowAppStart`（切页默认关闭，AC 第 7 条），驱动自己也只调
       `device-capture.ps1` 与 `adb devices`，不重启 adb server、不动设备上任何东西。
    ⚠️ 取证驱动自己的三处洞（根路径级数 / RAW 抓整行 / pattern 开跑前真编译）都按 §3c 的修法实现，
       收尾行带**分母**：`LEGS_DONE=n/3`，产出日志数与之同列 —— 别信一个没有分母的 DONE。
.OUTPUTS
    docs/verification/tooling/1568/device-capture-real-readings.txt（汇总读数件）
    子日志按运行戳分组：docs/verification/tooling/1568/legs/<RunStamp>/<leg>.out
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Device,
    [string]$AdbPath = 'D:\android-sdk\platform-tools\adb.exe',
    [int]$LegTimeoutSeconds = 1200,
    [switch]$SkipL1
)

$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path
$Capture = Join-Path $Root 'scripts\device-capture.ps1'
# 前置条件先 throw（§3c 第 1 条洞的修法）：路径不存在就别把四条腿跑成 exit=90
if (-not (Test-Path -LiteralPath $Capture)) { throw "CAPTURE_ABSENT $Capture" }
if (-not (Test-Path -LiteralPath $AdbPath)) { throw "ADB_ABSENT $AdbPath" }

$outDir = Join-Path $PSScriptRoot 'legs'
$RunStamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$legDir = Join-Path $outDir ($RunStamp + '-real')
New-Item -ItemType Directory -Force -Path $legDir | Out-Null

$outFile = Join-Path $PSScriptRoot 'device-capture-real-readings.txt'
$rows = @()

function Rec([string]$line) {
    Write-Host $line
    $script:rows += $line
}

Rec ("CAPTURE blob=" + (git -C $Root rev-parse HEAD).Trim() + " run_stamp=" + $RunStamp)
Rec ("DEVICE " + $Device + " model=" + ((& $AdbPath -s $Device shell getprop ro.product.model 2>$null | Out-String).Trim()) +
    " android=" + ((& $AdbPath -s $Device shell getprop ro.build.version.release 2>$null | Out-String).Trim()))
# 只读性自证：驱动只调 device-capture 与下面这一条 devices
Rec ("ADB_SERVER_LEVEL_PROBE argv=devices -l（无 -s，驱动自身也不重启 server）")

# RAW 六条 pattern：开跑前逐条真编译（§3c 第 3 条洞 —— 'Failure [' 不转义会等跑完第一条腿才炸）
$pats = @(
    'ADB_TEXT_TIMEOUT',
    'ADB_TEXT_CALL_BUDGET',
    'ADB_TIER_BUDGET',
    'LOGCAT_INCONCLUSIVE',
    'DEVICE_CAPTURE_RESULT',
    'Failure \['
)
foreach ($p in $pats) { $null = [regex]::new($p) }
Rec ("RAW_PATTERNS_COMPILED=" + $pats.Count)

$legs = @()
if (-not $SkipL1) { $legs += , @{ Tag = 'L1_ref'; Extra = @() } }
$legs += , @{ Tag = 'L2_logcat0'; Extra = @('-AdbLogcatTimeoutSeconds', '0') }
$legs += , @{ Tag = 'L3_text0'; Extra = @('-AdbTextCallTimeoutSeconds', '0') }

$produced = 0
$withRaw = 0
foreach ($leg in $legs) {
    $out = Join-Path $legDir ($leg.Tag + '.out')
    $errF = Join-Path $legDir ($leg.Tag + '.err')
    $psi = @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $Capture,
        '-Device', $Device, '-NoArchive', '-Module', 'device',
        '-OutDir', ('.ci-verify-real-' + $leg.Tag)
    ) + $leg.Extra
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $p = Start-Process -FilePath 'pwsh' -ArgumentList $psi -PassThru -NoNewWindow `
        -RedirectStandardOutput $out -RedirectStandardError $errF -WorkingDirectory (Split-Path -Parent $Capture)
    $exited = $p.WaitForExit($LegTimeoutSeconds * 1000)
    $sw.Stop()
    if (-not $exited) {
        try { $p.Kill($true) } catch { }
        Rec ("LEG " + $leg.Tag + " exit=TIMEOUT_CAP wall_seconds=" + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
            " cap_seconds=$LegTimeoutSeconds —— 上界自己也要能被点名")
        continue
    }
    $hasLog = Test-Path -LiteralPath $out
    if ($hasLog -and (Get-Item -LiteralPath $out).Length -gt 0) { $produced++ }
    Rec ("LEG " + $leg.Tag + " exit=" + $p.ExitCode + " wall_seconds=" + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
        " log_present=" + $hasLog)

    # 判据原料取**产品自己落盘的那份 UTF-8 日志**（`device-capture.log`）：
    # 腿的 stdout 走 console 编码，中文尾巴在这台机器上会被读成乱码（第一版入库件就是这样，
    # ASCII 字段完好但整行不可读）⇒ 换源之后 RAW 行与屏显中文同形，指得住。
    $prodLog = Join-Path (Join-Path $Root ('.ci-verify-real-' + $leg.Tag)) 'device-capture.log'
    $src = if (Test-Path -LiteralPath $prodLog) { $prodLog } else { $out }
    Rec ("RAW_SOURCE " + $leg.Tag + " " + $(if ($src -eq $prodLog) { 'product_log(UTF-8)' } else { 'leg_stdout' }) + " " + $src)
    $text = Get-Content -LiteralPath $src -Raw -Encoding UTF8
    foreach ($pat in $pats) {
        # 抓**整行**（§3c 第 2 条洞：'^.*TOKEN' 会把 call=/tier= 截掉）
        $m = [regex]::Matches($text, '(?m)^.*' + $pat + '.*$')
        foreach ($one in $m) { Rec ("RAW " + $leg.Tag + " :: " + $one.Value.Trim()) }
    }
    $errRaw = if (Test-Path -LiteralPath $errF) { (Get-Content -LiteralPath $errF -Raw -Encoding UTF8) } else { '' }
    # 空文件时 Get-Content -Raw 给的是 $null —— 直接 .Trim() 就是「对 Null 值表达式调用方法」，
    # 而这一抛会让整个 readings 件写不出来（首轮真机腿就是这么断在 L1 之后）。
    if ($errRaw -and $errRaw.Trim()) {
        $errTxt = $errRaw.Trim()
        Rec ("STDERR " + $leg.Tag + " bytes=" + $errTxt.Length + " 首行=" + (($errTxt -split "`r?`n")[0]))
    }
}

Rec ("LEGS_DONE=" + $legs.Count + "/" + $legs.Count + " legs_ran=" + $legs.Count +
    " legs_produced_log=" + $produced + " readings=" + $outFile)
Set-Content -LiteralPath $outFile -Value ($rows -join "`r`n") -Encoding UTF8
Write-Host ("DONE " + $outFile)
