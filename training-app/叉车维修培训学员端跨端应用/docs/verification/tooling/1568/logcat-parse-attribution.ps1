#Requires -Version 7
<#
.SYNOPSIS
    归因实验：真机 L1 那 ~4 分钟墙钟**不是** adb 那一格花掉的 —— 用规模缩放把它定位到 `Get-LogcatWindow`。
.DESCRIPTION
    起因（2026-10-08 真机腿现测）：`device-capture.ps1` 在 Redmi 23049RAD8C 上默认预算跑一整趟要
    241–574 秒，而同一趟里单次 `logcat -d -v epoch` 的 **adb 调用只 16.7–17.9 秒**
    （`device-capture-real-tier-readings.txt`，47 MB / 28.4 万行）⇒ 大头在把这份 dump 变成对象之后的处理。

    为什么不能直接说「就是 `$stamped +=`」：本脚本对同一份 dump 跑两种写法 × 四个规模
    （满量 / 1/2 / 1/4 / 1/8），看的是**缩放**而不是单次耗时。现测结论（`logcat-parse-attribution.txt`）：
    两种写法在 35k→284k 上都超线性（`+=` 29.8→60.4→143.8→509.3 秒；List.Add 19.2→43.1→124.8→375.7 秒）
    ⇒ **`+=` 只解释其中约四分之一到三分之一**，主体成本是「把 28 万行**全部物化成 pscustomobject 数组**」
    这件事本身（工作集实测 650 MB→1.2 GB，GC 压力跟着来）。写成「就是 += 」会比代码宽。

    ⚠️ 这一格**不在 #1568 的射程**：AC 第 2 条要收的是「没有上限的等」，物化成本是 #1562 就存在的形状
       （改前 base `704fd336` 的 `:359` 同一行就是 `$stamped +=`，本票一行没动），且修它要动判据原料的形状。
       ⇒ 处置：读数与判据入库，另立 follow-up 票，本票不改。
    ⚠️ 跑之前确保机器空闲：同机并发（真机腿、变异电池）会把两族数一起抬高，缩放比反而失真。
.PARAMETERS
    Dump 一份真机全量 logcat（`-v epoch`）的落盘副本；行数从文件现数，不写死。
.OUTPUTS
    docs/verification/tooling/1568/logcat-parse-attribution.txt
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Dump,
    [string]$Out = '',
    [int]$MaxLines = 0
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Dump)) { throw "DUMP_ABSENT $Dump" }
if (-not $Out) { $Out = Join-Path $PSScriptRoot 'logcat-parse-attribution.txt' }

# 只留带 epoch 时间戳的行（与 Get-LogcatWindow 的过滤同形），行号与体量都在这里现数
$all = @(Get-Content -LiteralPath $Dump -Force | Where-Object { $_ -match '^\s*\d{9,}\.\d+\s' })
if ($MaxLines -gt 0 -and $all.Count -gt $MaxLines) { $all = @($all | Select-Object -First $MaxLines) }
if ($all.Count -lt 8) { throw ("SAMPLE_TOO_SMALL count=" + $all.Count) }

Set-Content -LiteralPath $Out -Value (
    "QUADRATIC source_lines=" + $all.Count + " dump=" + $Dump +
    " bytes=" + (Get-Item -LiteralPath $Dump).Length + " now=" + (Get-Date -Format 's') +
    " 两种写法各跑四档规模（1/8、1/4、1/2、满量），判据看缩放不看单次") -Encoding utf8

foreach ($div in @(8, 4, 2, 1)) {
    $n = [int][math]::Floor($all.Count / $div)
    $sub = @($all | Select-Object -First $n)

    # A 组：与产品代码同形的 `$acc += [pscustomobject]…`
    $swA = [System.Diagnostics.Stopwatch]::StartNew()
    $acc = @()
    foreach ($l in $sub) {
        $m = [regex]::Match($l, '^\s*(\d{9,}\.\d+)\s')
        if (-not $m.Success) { continue }
        $acc += [pscustomobject]@{ Ts = [double]$m.Groups[1].Value; Line = $l }
    }
    $swA.Stop()

    # B 组：同一份输入、同一件事，只把累加换成 List.Add —— 差值才是 += 的成本
    $swB = [System.Diagnostics.Stopwatch]::StartNew()
    $list = New-Object 'System.Collections.Generic.List[object]'
    foreach ($l in $sub) {
        $m = [regex]::Match($l, '^\s*(\d{9,}\.\d+)\s')
        if (-not $m.Success) { continue }
        [void]$list.Add([pscustomobject]@{ Ts = [double]$m.Groups[1].Value; Line = $l })
    }
    $swB.Stop()

    Add-Content -LiteralPath $Out -Value ("SCALE div=" + $div + " n=" + $n +
        " plus_assign_ms=" + $swA.ElapsedMilliseconds + " produced_a=" + $acc.Count +
        " list_add_ms=" + $swB.ElapsedMilliseconds + " produced_b=" + $list.Count +
        " ws_mb=" + [math]::Round((Get-Process -Id $PID).WorkingSet64 / 1MB, 0)) -Encoding utf8
    $acc = $null; $list = $null; $sub = $null
    [GC]::Collect()
}
Write-Output ("DONE " + $Out)
