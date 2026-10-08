#!/usr/bin/env pwsh
<#
.SYNOPSIS
    【非门】#1568 预算默认值的**现测探针**：量一次 adb 调用在各调用形态上的真实耗时，
    产出一份可复算的读数（`latency-readings.txt`），供门脚本 `param()` 里那句预算默认值指得到产物。

.DESCRIPTION
    为什么要有这一份（#1568 票面 AC 第 3 条）：
      「每个预算数必须能指到一份入库产物（本仓纪律：读数要可复现，不能只写在注释里）」。
      本票给两个收口点定预算，其中**管理类**调用（connect / mdns）与**读数类**（devices / version /
      shell dumpsys）的耗时形状不同：前者会踩 TCP 握手超时（连不上且不拒绝的端口要等 SYN 重传跑完），
      拿同一个 15 秒去套会把「还能连上但慢」的连接判死 —— 票面明确「本票要收的是没有上限的等，不是等得久」。
      ⇒ 档位必须由现测定，不能凭感觉。

    红线（与 scripts/wireless-debug.ps1 同源）：全程只读 —— 不 kill-server、不 install / push / reboot、
      不发 input、不截图。connect 只在本地与非可路由地址上试（**不会**真接上任何设备）；
      万一连上了才 disconnect 那一条，不动别的 transport。

    设备侧那一档：本轮现测时**没有 state=device 的 transport**（`adb devices` 空），
      所以 shell / dumpsys 那几条只报 no_device 而不谎称测过 —— 它们的默认值改为引用
      #1560 的入库产物（`docs/verification/tooling/1560/README.md` 里同一条有界调用真机返回
      646 / 719 / 949 / 1217 ms，且那还是一张约 730 KB 的 PNG）。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/adb-latency-probe.ps1 -OutFile docs/verification/tooling/1568/latency-readings.txt
#>
[CmdletBinding()]
param(
    [string]$AdbExe = 'D:\android-sdk\platform-tools\adb.exe',
    [string]$OutFile = '',
    # 非可路由地址（RFC5737 保留段），用来量「TCP 一直不应答」那一档的真实上界
    [string]$BlackholeHost = '10.255.255.1',
    [string]$RefusedHost = '127.0.0.1'
)

$ErrorActionPreference = 'Continue'
$lines = @()

function Invoke-Timed {
    param([string]$Label, [string[]]$AdbArgs)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $out = & $AdbExe @AdbArgs 2>&1 | Out-String
    $sw.Stop()
    $first = (($out -split "`r?`n" | Where-Object { $_.Trim() }) | Select-Object -First 1)
    if (-not $first) { $first = '(empty)' }
    $script:lines += ('{0} ms={1} rc={2} out={3}' -f $Label, $sw.ElapsedMilliseconds, $LASTEXITCODE, ($first -replace '[^\x20-\x7E]', '?'))
    return $out
}

$lines += ('PROBE adb_exe_exists=' + (Test-Path -LiteralPath $AdbExe -PathType Leaf) + ' at=' + (Get-Date).ToString('s'))
$lines += ('OS=' + [System.Environment]::OSVersion.VersionString + ' ps=' + $PSVersionTable.PSVersion.ToString())

# ── 读数类（本票默认 15 秒那一档的对象）
Invoke-Timed 'READ_version' @('version') | Out-Null
Invoke-Timed 'READ_devices' @('devices') | Out-Null
Invoke-Timed 'READ_mdns_services' @('mdns', 'services') | Out-Null

# ── 管理类：端口不应答 / 端口拒绝（connect 那一档）
Invoke-Timed 'MGMT_connect_refused' @('connect', ('{0}:9' -f $RefusedHost)) | Out-Null
Invoke-Timed 'MGMT_connect_blackhole' @('connect', ('{0}:5555' -f $BlackholeHost)) | Out-Null

# ── 设备侧读数类：有 transport 才测，没有就照实说没测
$dev = @(& $AdbExe devices 2>&1 | Out-String)
$live = @(($dev -split "`r?`n") | ForEach-Object { if ($_ -match '^(\S+)\s+device\s*$') { $Matches[1] } })
if ($live.Count -ge 1) {
    $s = $live[0]
    Invoke-Timed 'DEV_getprop' @('-s', $s, 'shell', 'getprop', 'ro.product.model') | Out-Null
    Invoke-Timed 'DEV_settings_get' @('-s', $s, 'shell', 'settings', 'get', 'global', 'adb_wifi_enabled') | Out-Null
    Invoke-Timed 'DEV_dumpsys_power' @('-s', $s, 'shell', 'dumpsys', 'power') | Out-Null
} else {
    $lines += 'DEV_* no_device —— 本轮现测没有 state=device 的 transport，设备侧那一档未测（默认值引用 #1560 产物）'
}

$text = ($lines -join "`r`n") + "`r`n"
Write-Host $text
if ($OutFile) {
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    Set-Content -LiteralPath $OutFile -Value $text -Encoding ascii
    Write-Host ('PROBE wrote=' + $OutFile)
}
