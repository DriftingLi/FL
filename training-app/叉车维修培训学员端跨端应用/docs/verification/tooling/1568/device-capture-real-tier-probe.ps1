#Requires -Version 7
<#
.SYNOPSIS
    真机侧的**档位余量**现测：把 device-capture 那 6 个点位的同形态调用逐条计时。
.DESCRIPTION
    补的是哪一格：#1568 AC 第 2 条第二片定两档（文本 15 秒 / logcat 60 秒）时，能引用的入库读数**全是仿真机**
    （`emulator-tier-readings.txt`：dumpsys 1,168 / 1,662 ms、`logcat -d` 5,896 / 6,186 ms），
    ADR-0008 当时把这点写成「真机的 dumpsys / logcat 余量本仓没有读数」。这份件跑在维护者那台
    Redmi 23049RAD8C / Android 15 上，把那句话换成现测。

    三件只在这份件里量得到的事：
      1. `logcat -d -v epoch -t 1`（第一片登记为「没有独立现测」那一格）真机 **513 / 523 ms**；
      2. 全量 `logcat -d -v epoch` 真机 **17,909 / 16,673 ms**（同一份 dump 47 MB / 28.4 万行）
         ⇒ 套 15 秒文本档**必假红**，60 秒档的真机余量是 ≈3.3 倍而不是仿真机那 ≈10 倍；
      3. `devices -l` 不带 serial 的 server 级形态在真机上 89–155 ms ⇒ 文本档余量 ≈42 倍。

    ⚠️ **第一版把形参写成 `$args`** —— 那是 PowerShell 的自动变量，splat 出来是空数组 ⇒
       六次调用全变成裸 `adb`（各自 rc=1、`chars` 恒 8933 = usage 文本），只有不走该函数的截图那次是真的。
       现在改名 `$ArgList` 并加两条自证：`argv_n>=1` 与 `rc=0`（读数里都在，错了当场能看见）。
    ⚠️ 跑之前确保机器空闲：这份量的是**时延**，同机并发（仿真机腿、变异电池）会把数抬高 —— 见
       [[gate3-wallclock-flake-under-concurrency]] 同族教训。
.OUTPUTS
    docs/verification/tooling/1568/device-capture-real-tier-readings.txt
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Device,
    [string]$AdbExe = 'D:\android-sdk\platform-tools\adb.exe',
    [string]$Pkg = 'io.dcloud.uniappx',
    [int]$Rounds = 2,
    [string]$Out = ''
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $AdbExe)) { throw "ADB_ABSENT $AdbExe" }
if (-not $Out) { $Out = Join-Path $PSScriptRoot 'device-capture-real-tier-readings.txt' }
$script:Adb = $AdbExe
$script:OutFile = $Out

Set-Content -LiteralPath $Out -Value (
    "REAL_TIER serial=$Device model=" + ((& $AdbExe -s $Device shell getprop ro.product.model 2>$null | Out-String).Trim()) +
    " android=" + ((& $AdbExe -s $Device shell getprop ro.build.version.release 2>$null | Out-String).Trim()) +
    " adb=" + ((& $AdbExe version 2>$null | Out-String) -split "`r?`n")[0].Trim() +
    " now=" + (Get-Date -Format 's') + " rounds=$Rounds 形态=与 scripts/device-capture.ps1 的六个点位同形") -Encoding utf8

function Timed([string]$name, [string[]]$ArgList, [string]$tier) {
    # 两条自证：argv 非空（第一版就飘在这）+ 调用必须 rc=0，否则量的是错误路径不是时延
    if (@($ArgList).Count -lt 1) { throw ("EMPTY_ARGV name=" + $name) }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $res = & $script:Adb @ArgList 2>&1 | Out-String
    $sw.Stop()
    $rc = $LASTEXITCODE
    Add-Content -LiteralPath $script:OutFile -Value (
        "CALL $name tier=$tier rc=$rc ms=" + $sw.ElapsedMilliseconds + " chars=" + $res.Length +
        " argv_n=" + @($ArgList).Count) -Encoding utf8
    Write-Output ("$name tier=$tier rc=$rc ms=" + $sw.ElapsedMilliseconds)
}

for ($i = 1; $i -le $Rounds; $i++) {
    Add-Content -LiteralPath $Out -Value ("ROUND " + $i) -Encoding utf8
    Timed 'devices_l_no_serial'    @('devices', '-l') 'server'
    Timed 'shell_dumpsys_activity' @('-s', $Device, 'shell', 'dumpsys', 'activity', 'activities') 'read'
    Timed 'shell_resolve_activity' @('-s', $Device, 'shell', 'cmd', 'package', 'resolve-activity', '--brief',
        '-a', 'android.intent.action.MAIN', '-c', 'android.intent.category.LAUNCHER', $Pkg) 'resolve'
    Timed 'shell_pidof'            @('-s', $Device, 'shell', 'pidof', $Pkg) 'read'
    Timed 'logcat_t1_epoch'        @('-s', $Device, 'shell', 'logcat', '-d', '-v', 'epoch', '-t', '1') 'logcat'
    Timed 'logcat_dump_epoch'      @('-s', $Device, 'shell', 'logcat', '-d', '-v', 'epoch') 'logcat'
}

# 截图那一档（exec-out 二进制）：#1562 已有基线，这里补同机现值
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$png = Join-Path $env:TEMP 'real-tier-shot.png'
& $AdbExe -s $Device exec-out screencap -p > $png 2>$null
$sw.Stop()
Add-Content -LiteralPath $Out -Value ("CALL shot_execout tier=shot rc=$LASTEXITCODE ms=" + $sw.ElapsedMilliseconds +
    " bytes=" + (Get-Item -LiteralPath $png).Length) -Encoding utf8
Remove-Item -LiteralPath $png -Force -ErrorAction SilentlyContinue

# 体量解释：为什么真机这一格比仿真机大两个数量级（缓冲区可读字节直接抄回）
$g = (& $AdbExe -s $Device shell logcat -g 2>$null | Out-String)
($g -split "`r?`n" | Where-Object { $_.Trim() }) | ForEach-Object { Add-Content -LiteralPath $Out -Value ("BUFFER " + $_.Trim()) -Encoding utf8 }
Write-Output ("DONE " + $Out)
