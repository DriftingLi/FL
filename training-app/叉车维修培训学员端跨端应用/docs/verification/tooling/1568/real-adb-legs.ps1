#!/usr/bin/env pwsh
<#
.SYNOPSIS
    【非门】#1568 两个收口点在**真 adb** 上的读数腿：证明「有界」不是拿假载体自证，
    而是真 cmd.exe + 真 adb.exe + 真进程树下成立。

.DESCRIPTION
    为什么还要这一份（根 `docs/agents/release.md` 的 DoD：工具链改动的验收判据是
    「它要达成的那个行为在**真实链路**上成立」，只跑 test:unit 写「免」不算验收）：
    ETD / WBD 两族用的是可编程假载体（要的是「真挂死」这一格 —— 真 adb 不可按需要挂死），
    本文件补的是**另一头**：载体换成真 adb.exe，读回、stderr 合并、预算到点三件事都得在真链路上一样成立。

    红线：全程只读 —— 只发 `version` / `devices` / `mdns services` / `settings get` / `dumpsys`，
    不 connect、不 disconnect、不 install / push / kill-server、不发 input。StateDir 指到临时目录，
    不碰维护者在用的 `%LOCALAPPDATA%\adb-wireless`。

    设备侧那一档本轮现测时**没有 state=device 的 transport** ⇒ `Do-Status` 的 shell 段自然跳过，
    这不是缺陷而是环境；设备在场时同一份脚本会自动多打几条读数。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/real-adb-legs.ps1 -OutFile docs/verification/tooling/1568/real-adb-readings.txt
#>
[CmdletBinding()]
param(
    [string]$AdbExe = 'D:\android-sdk\platform-tools\adb.exe',
    [string]$OutFile = ''
)

$ErrorActionPreference = 'Stop'
$lines = @()
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path  # <项目根>
$tool = Join-Path $root 'scripts\wireless-debug.ps1'
$smoke = Join-Path $root 'scripts\emulator-smoke.ps1'
$lib = Join-Path $root 'scripts\lib\auto-screenshot.ps1'

function Add-Line([string]$s) { $script:lines += $s; Write-Host $s }

if (-not (Test-Path -LiteralPath $AdbExe -PathType Leaf)) {
    Add-Line 'RL_ABORT adb_missing'
    exit 2
}
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('rl1568-' + $PID)
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

# ── R1：默认预算下的真链路 —— 工具必须照常给出结论，且 adb 的 stdout 真读回来了
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$out1 = (& pwsh -NoProfile -ExecutionPolicy Bypass -File $tool -Action status -AdbExe $AdbExe -StateDir $tmp 2>&1 | Out-String)
$sw.Stop()
$act1 = Join-Path $tmp 'actions.log'
$budget1 = ''
if (Test-Path -LiteralPath $act1) {
    $budget1 = ((Get-Content -LiteralPath $act1 -Encoding utf8 | Where-Object { $_ -match 'ADB_CALL_BUDGET' }) | Select-Object -Last 1)
}
Add-Line ('RL1_concluded=' + [bool]($out1 -match 'WIRELESS_DEBUG action=status result=done'))
Add-Line ('RL1_stdout_has_adb_version=' + [bool]($out1 -match 'Android Debug Bridge version'))
Add-Line ('RL1_budget_line=' + ($budget1 -replace '[^\x20-\x7E]', '?'))
Add-Line ('RL1_wall_ms=' + $sw.ElapsedMilliseconds)

# ── R2：预算 0 的强制腿（先例 #1560「强制腿只能用预算 0」）—— 真 adb 通道上也必须「到点就给结论」
$sw2 = [System.Diagnostics.Stopwatch]::StartNew()
$out2 = (& pwsh -NoProfile -ExecutionPolicy Bypass -File $tool -Action status -AdbExe $AdbExe -StateDir $tmp -AdbCallTimeoutSeconds 0 2>&1 | Out-String)
$sw2.Stop()
$tmo2 = @()
if (Test-Path -LiteralPath $act1) {
    $tmo2 = @((Get-Content -LiteralPath $act1 -Encoding utf8 | Where-Object { $_ -match 'ADB_CALL_TIMEOUT' }))
}
Add-Line ('RL2_concluded=' + [bool]($out2 -match 'WIRELESS_DEBUG action=status result=done'))
Add-Line ('RL2_timeouts=' + $tmo2.Count)
Add-Line ('RL2_first=' + ($(if ($tmo2.Count) { ($tmo2[0] -replace '[^\x20-\x7E]', '?') } else { 'none' })))
Add-Line ('RL2_wall_ms=' + $sw2.ElapsedMilliseconds)

# ── R3：真 adb 的 stderr 合并 —— 取一个不存在的 serial，adb 的「device not found」原文在 **stderr** 上
#     手法沿用行为守护那族：AST 抽出 Get-AdbOutput 直调，不跑整篇冒烟脚本
$probe = @'
$ErrorActionPreference = "Stop"
. "@@LIB@@"
$src = Get-Content -LiteralPath "@@SMOKE@@" -Raw
$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)
$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))
foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }
$VerifyDir = "@@TMP@@"
$LogPath = Join-Path $VerifyDir 'emulator-smoke.log'
$AdbExe = "@@ADB@@"
$Serial = 'emulator-5556'
$AdbCallTimeoutSeconds = 15
$AdbTextCallTimeoutSeconds = 15
$script:AdbTextTimeouts = @()
$script:AdbTextCalls = 0
$txt = Get-AdbOutput @('shell', 'echo', 'rl3')
Write-Output ("RL3_nonempty=" + [bool]("$txt".Trim().Length -gt 0))
Write-Output ("RL3_HAS_NOT_FOUND=" + [bool]("$txt" -match 'not found'))
Write-Output ("RL3_raw_text=" + ("$txt" -replace '[^\x20-\x7E]', '?'))
'@
$probe = $probe.Replace('@@LIB@@', $lib).Replace('@@SMOKE@@', $smoke).Replace('@@TMP@@', $tmp).Replace('@@ADB@@', $AdbExe)
# 落到临时 .ps1 再 -File 执行：不经 -Command 就不会被外层引号/码页二次加工（本仓踩过的传参坑）
$probeFile = Join-Path $tmp 'rl3-probe.ps1'
Set-Content -LiteralPath $probeFile -Value $probe -Encoding utf8
$out3 = (& pwsh -NoProfile -ExecutionPolicy Bypass -File $probeFile 2>&1 | Out-String)
foreach ($k in 'RL3_nonempty', 'RL3_HAS_NOT_FOUND') {
    $m = [regex]::Match($out3, "(?m)^$k=(\S+)")
    Add-Line ("$k=" + $(if ($m.Success) { $m.Groups[1].Value } else { 'missing' }))
}
Add-Line ('RL3_raw=' + ((($out3 -split "`r?`n" | Where-Object { $_ -match 'not found|no devices|RL3_' }) -join ' | ') -replace '[^\x20-\x7E]', '?'))
Add-Line ('RL3_carrier=real_adb stderr-only-answer serial=emulator-5556(no such transport)')

Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
$text = ($lines -join "`r`n") + "`r`n"
if ($OutFile) {
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    Set-Content -LiteralPath $OutFile -Value $text -Encoding ascii
}
