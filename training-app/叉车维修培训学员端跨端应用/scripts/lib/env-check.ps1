<#
.SYNOPSIS
    环境检测模块：adb 设备 + HBuilderX CLI + 忙检测。

.DESCRIPTION
    检测项：
      1. adb 设备是否在线
      2. HBuilderX CLI 是否可用
      3. HBuilderX 主程序是否忙（复用 hx-busy.ps1）

    全部通过返回 @{ Ok = $true; Device = <serial>; CliPath = <path> }
    任一失败返回 @{ Ok = $false; Error = <描述> }

.EXAMPLE
    . scripts/lib/env-check.ps1
    $env = Test-BuildEnv -ProjectDir "D:\FL\training-app\叉车维修培训学员端跨端应用"
    if (-not $env.Ok) { Write-Host "环境不可用: $($env.Error)" }
#>

# 复用 hx-run.ps1 的探测函数
function Resolve-AdbExeLocal {
    $cands = @()
    foreach ($root in @($env:ANDROID_SDK_ROOT, $env:ANDROID_HOME)) {
        if ($root) { $cands += (Join-Path $root 'platform-tools\adb.exe') }
    }
    $cands += 'D:\android-sdk\platform-tools\adb.exe'
    foreach ($c in $cands) {
        if ($c -and (Test-Path -LiteralPath $c -PathType Leaf)) { return $c }
    }
    $cmd = Get-Command adb -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    return $null
}

function Resolve-CliPathLocal {
    param([string]$Explicit)
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit -PathType Leaf) {
            return (Resolve-Path -LiteralPath $Explicit).Path
        }
        return $null
    }
    # 正在运行的 HBuilderX 同目录
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($proc) {
            $runningCli = Join-Path (Split-Path $proc.Path) 'cli.exe'
            if (Test-Path -LiteralPath $runningCli -PathType Leaf) {
                return (Resolve-Path -LiteralPath $runningCli).Path
            }
        }
    } catch { }
    # 常见安装位置
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
        if (Test-Path -LiteralPath $c -PathType Leaf) {
            return (Resolve-Path -LiteralPath $c).Path
        }
    }
    return $null
}

function Get-OnlineDevices {
    param([string]$AdbExe)
    $out = (& $AdbExe devices 2>&1 | Out-String)
    $list = @()
    foreach ($line in ($out -split "`r?`n")) {
        if ($line -match '^\s*(\S+)\s+(device|offline|unauthorized)\s*$') {
            $list += [pscustomobject]@{ Serial = $Matches[1]; State = $Matches[2] }
        }
    }
    return @($list | Where-Object { $_.State -eq 'device' })
}

function Test-BuildEnv {
    [CmdletBinding()]
    param(
        [string]$ProjectDir,
        [string]$CliPath,
        [string]$Device,
        [int]$HxWaitSeconds = 600
    )

    # 1. 检测 adb
    $adbExe = Resolve-AdbExeLocal
    if (-not $adbExe) {
        return [pscustomobject]@{
            Ok    = $false
            Error = '找不到 adb.exe。设 $env:ANDROID_SDK_ROOT / $env:ANDROID_HOME，或跑 scripts/android-sdk-setup.ps1'
        }
    }

    # 2. 检测设备
    $online = Get-OnlineDevices -AdbExe $adbExe
    if ($Device) {
        $hit = @($online | Where-Object { $_.Serial -eq $Device })
        if ($hit.Count -eq 0) {
            return [pscustomobject]@{
                Ok    = $false
                Error = "指定设备 $Device 不在 adb 设备列表里"
            }
        }
        $targetDevice = $Device
    } else {
        if ($online.Count -eq 0) {
            return [pscustomobject]@{
                Ok    = $false
                Error = '没有在线安卓设备（adb devices 无 device 行）：先连真机或用 -Device 指定'
            }
        }
        if ($online.Count -gt 1) {
            return [pscustomobject]@{
                Ok    = $false
                Error = "有 $($online.Count) 个在线设备，必须显式指定 -Device"
            }
        }
        $targetDevice = $online[0].Serial
    }

    # 3. 检测 HBuilderX CLI
    $cliExe = Resolve-CliPathLocal -Explicit $CliPath
    if (-not $cliExe) {
        if ($env:HBuilderX_CLI -and (Test-Path -LiteralPath $env:HBuilderX_CLI -PathType Leaf)) {
            $cliExe = (Resolve-Path -LiteralPath $env:HBuilderX_CLI).Path
        }
    }
    if (-not $cliExe) {
        return [pscustomobject]@{
            Ok    = $false
            Error = '找不到可用的 HBuilderX cli.exe。用 -CliPath <路径> 或设 $env:HBuilderX_CLI'
        }
    }

    # 4. HBuilderX 忙检测（复用 hx-busy.ps1）
    $busyHelper = Join-Path $PSScriptRoot 'hx-busy.ps1'
    if (Test-Path -LiteralPath $busyHelper) {
        . $busyHelper
        $logDir = if ($ProjectDir) { Join-Path $ProjectDir '.ci-verify' } else { Join-Path $PSScriptRoot '..\.ci-verify' }
        $logPath = Join-Path $logDir 'env-check.log'
        New-Item -ItemType Directory -Force -Path $logDir | Out-Null
        $hx = Wait-HxFree -CliExe $cliExe -TimeoutSeconds $HxWaitSeconds -LogPath $logPath
        try {
            # 通过
        } finally {
            Release-HxLock
        }
    }

    return [pscustomobject]@{
        Ok      = $true
        Device  = $targetDevice
        CliPath = $cliExe
        AdbExe = $adbExe
    }
}
