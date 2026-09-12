<#
.SYNOPSIS
    Android SDK 一次性装机（command-line tools + platform-tools/adb + emulator + API 30 x86 系统镜像）。

.DESCRIPTION
    这是**非门辅助**工具链的准备脚本，服务于 scripts/emulator-smoke.ps1（仿真机前置冒烟）。
    它**不是验收门**，也不替代 ① 真机门：仿真机抓不到生物识别/厂商 ROM/真机上传路径类问题。
    口径见 docs/adr/0008-移动端验收门与证据.md（「非门辅助：仿真机前置冒烟」，#883 / O2）。

    做什么
      - 下载 Android command-line tools 并摊平到 <SdkRoot>\cmdline-tools\latest（sdkmanager 只认这个布局）
      - 选中一个 JDK 17+（sdkmanager 硬要求；本机 PATH 上通常没有 java）
      - sdkmanager 安装 platform-tools / emulator / system-images;android-30;google_apis;x86
      - 接受 licenses（--licenses 需要交互，**不**由本脚本代跑；见 .PARAMETER AcceptLicenses）
      - 修正**复用**的既有 AVD：config.ini 里的绝对 sdk/皮肤路径指向已删除的旧 SDK，重指到本 SdkRoot

    幂等：已存在的组件跳过（sdkmanager 自身也是幂等安装），脚本可反复跑。

    装机成本（实测）：约 3-4 GB 下载、20-40 分钟（视网络）。

.PARAMETER SdkRoot
    SDK 根目录。默认 D:\android-sdk —— **非系统盘且路径不含空格**（空格路径对 sdkmanager/emulator
    的部分调用是已知雷区，故刻意不用默认的 D:\Program Files\android）。

.PARAMETER ApiLevel
    系统镜像的 API 级别。默认 30，与机器上既有 AVD（Pixel_4a_API_30 / Pixel_XL_API_30）一致。

.PARAMETER Abi
    系统镜像 ABI。默认 x86 —— 与既有 AVD 的 abi.type 一致，且 HBuilderX 自带基座 APK
    含 x86/x86_64，x86 模拟器装得上。

.PARAMETER CommandLineToolsUrl
    command-line tools zip 下载地址（要换版本时改这里）。

.PARAMETER CommandLineToolsZip
    已下载好的 command-line tools zip 本地路径。给了就复用、不再联网下载（换 SDK 根目录重跑时省 150 MB）。

.PARAMETER SkipSystemImage
    只装 tools/platform-tools/emulator，不下载系统镜像（镜像约 1 GB）。

.PARAMETER Force
    即使探测到组件已齐也重跑安装步骤（默认已齐则跳过，用于幂等复跑）。

.EXAMPLE
    pwsh -NoProfile -File scripts/android-sdk-setup.ps1
    pwsh -NoProfile -File scripts/android-sdk-setup.ps1 -SdkRoot 'D:\android-sdk' -ApiLevel 30
    pwsh -NoProfile -File scripts/android-sdk-setup.ps1 -Force          # 幂等复跑验证

.NOTES
    退出码：0 = 就绪；2 = 环境不可用（下载失败 / 无 JDK 17+ / sdkmanager 安装失败）。
#>
[CmdletBinding()]
param(
    [string]$SdkRoot = 'D:\android-sdk',
    [int]$ApiLevel = 30,
    [string]$Abi = 'x86',
    [string]$Package = 'google_apis',
    [string]$CommandLineToolsUrl = 'https://dl.google.com/android/repository/commandlinetools-win-11076708_latest.zip',
    [string]$CommandLineToolsZip = '',
    [switch]$SkipSystemImage,
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$SdkRoot = [System.IO.Path]::GetFullPath($SdkRoot)
$PlatformTools = Join-Path $SdkRoot 'platform-tools'
$EmulatorDir = Join-Path $SdkRoot 'emulator'
$CmdlineToolsDir = Join-Path $SdkRoot 'cmdline-tools'
$LatestDir = Join-Path $CmdlineToolsDir 'latest'
$SdkManager = Join-Path $LatestDir 'bin\sdkmanager.bat'
$SystemImage = "system-images;android-$ApiLevel;$Package;$Abi"
$ImageSysdir = "system-images\android-$ApiLevel\$Package\$Abi"

function Write-Step { param([string]$Message) Write-Host "[sdk-setup] $Message" }
function Write-Warn2 { param([string]$Message) Write-Host "[sdk-setup][warn] $Message" -ForegroundColor Yellow }

function Write-Result {
    param([string]$Status, [string]$Detail)
    Write-Host ''
    Write-Host "ANDROID_SDK_SETUP_RESULT=$Status"
    if ($Detail) { Write-Host "  $Detail" }
}

# --- 环境变量：本进程内先设好，sdkmanager/emulator 才认得 ---
function Set-SdkEnvironment {
    $env:ANDROID_HOME = $SdkRoot
    $env:ANDROID_SDK_ROOT = $SdkRoot
    $env:ANDROID_USER_HOME = if ($env:ANDROID_USER_HOME) { $env:ANDROID_USER_HOME } else { Join-Path $env:USERPROFILE '.android' }
    $pathParts = @($PlatformTools, $EmulatorDir, (Join-Path $LatestDir 'bin'))
    $env:Path = ($pathParts -join ';') + ';' + $env:Path
}

# --- JDK 17+：sdkmanager 硬要求；本机 PATH 上一般没有 java ---
function Resolve-JavaHome {
    $candidates = @()
    if ($env:JAVA_HOME) { $candidates += $env:JAVA_HOME }
    $candidates += @(
        "$env:ProgramFiles\android studio\jbr"        # Android Studio 自带 JBR（实测 21.0.9）
        "$env:ProgramFiles\Eclipse Adoptium\jdk-21*"
        "$env:ProgramFiles\Microsoft\jdk-17*"
        "$env:ProgramFiles\Java\jdk-21*"
        "$env:ProgramFiles\Java\jdk-17*"
        "$env:LOCALAPPDATA\Programs\Eclipse Adoptium\jdk-21*"
    )
    foreach ($pattern in $candidates) {
        if (-not $pattern) { continue }
        $expanded = @()
        if ($pattern -match '[\*\?]') {
            $expanded = @(Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName)
        } elseif (Test-Path -LiteralPath $pattern -PathType Container) {
            $expanded = @($pattern)
        }
        foreach ($jdkHome in $expanded) {
            $java = Join-Path $jdkHome 'bin\java.exe'
            if (-not (Test-Path -LiteralPath $java -PathType Leaf)) { continue }
            $version = Get-JavaMajorVersion -JavaExe $java
            if ($version -ge 17) {
                Write-Step "JDK: $jdkHome (java $version)"
                return $jdkHome
            }
        }
    }
    return $null
}

function Get-JavaMajorVersion {
    param([string]$JavaExe)
    try {
        $raw = (& $JavaExe -version 2>&1 | Out-String)
    } catch { return 0 }
    $m = [regex]::Match($raw, 'version "(\d+)(?:\.(\d+))?')
    if (-not $m.Success) { return 0 }
    $major = [int]$m.Groups[1].Value
    if ($major -eq 1 -and $m.Groups[2].Success) { return [int]$m.Groups[2].Value }  # 1.8 形态
    return $major
}

function Install-CommandLineTools {
    if (Test-Path -LiteralPath $SdkManager -PathType Leaf) {
        Write-Step "command-line tools 已就位，跳过下载：$SdkManager"
        return
    }
    $stage = Join-Path $env:TEMP ('android-cmdline-tools-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    $zip = Join-Path $stage 'cmdline-tools.zip'
    # 复用缓存：换个 SDK 根目录重跑时不必再下一次 150 MB（可用 -CommandLineToolsZip 显式指定）
    if ($CommandLineToolsZip -and (Test-Path -LiteralPath $CommandLineToolsZip -PathType Leaf)) {
        Copy-Item -LiteralPath $CommandLineToolsZip -Destination $zip -Force
        Write-Step "复用缓存的 command-line tools zip：$CommandLineToolsZip"
    } else {
        Write-Step "下载 command-line tools：$CommandLineToolsUrl"
        try {
            Invoke-WebRequest -Uri $CommandLineToolsUrl -OutFile $zip -UseBasicParsing -TimeoutSec 1800
        } catch {
            Write-Result 'UNUSABLE' "command-line tools 下载失败：$($_.Exception.Message)"
            exit 2
        }
    }
    Write-Step ("zip 大小：" + [math]::Round((Get-Item $zip).Length / 1MB, 1) + ' MB')
    $extract = Join-Path $stage 'x'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($zip, $extract)

    # zip 里是 cmdline-tools/ ；sdkmanager 只认 <SdkRoot>\cmdline-tools\latest 布局
    New-Item -ItemType Directory -Force -Path $CmdlineToolsDir | Out-Null
    if (Test-Path -LiteralPath $LatestDir) { Remove-Item -LiteralPath $LatestDir -Recurse -Force }
    Move-Item -LiteralPath (Join-Path $extract 'cmdline-tools') -Destination $LatestDir
    Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    Write-Step "command-line tools 已摊平到：$LatestDir"
}

function Test-SdkComponentInstalled {
    param(
        [string]$Relative,
        [string]$PathType = 'Container'
    )
    return (Test-Path -LiteralPath (Join-Path $SdkRoot $Relative) -PathType $PathType)
}

function Install-SdkPackages {
    $wanted = @('platform-tools', 'emulator')
    if (-not $SkipSystemImage) { $wanted += $SystemImage }

    $allPresent = (Test-SdkComponentInstalled 'platform-tools\adb.exe' -PathType Leaf) -and
                  (Test-SdkComponentInstalled 'emulator\emulator.exe' -PathType Leaf) -and
                  ($SkipSystemImage -or (Test-SdkComponentInstalled $ImageSysdir))
    if ($allPresent -and -not $Force) {
        Write-Step '所需组件已齐，跳过 sdkmanager 安装（幂等）'
        return
    }

    Write-Step "sdkmanager 安装：$($wanted -join ', ')"
    Write-Step '（系统镜像约 1 GB，首次会跑几分钟到几十分钟）'

    # sdkmanager 是个 .bat，**非零退出码在 PowerShell 里会变成终止错误**（本脚本 ErrorActionPreference=Stop），
    # 且它会把下载/解压交给子进程 —— 必须临时放宽偏好，否则父进程退出早于落盘时整个脚本会硬崩（实测 exit 1）。
    $previousEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        # sdkmanager 需要交互式输入 y/n 接受 licenses；用 yes 流喂它
        $answer = ('y' + [Environment]::NewLine) * 200
        $out = $answer | & $SdkManager --sdk_root="$SdkRoot" @wanted 2>&1
    } finally {
        $ErrorActionPreference = $previousEap
    }
    $text = ($out | Out-String)
    if ($text -match '(?m)(ERROR:|Failed to install|Failed to read)') {
        Write-Warn2 'sdkmanager 输出含错误行：'
        ($text -split "`n" | Where-Object { $_ -match 'ERROR:|Error|Failed' } | Select-Object -First 20) | ForEach-Object { Write-Host "    $_" }
    }

    # sdkmanager 的子进程可能比父进程活得久：给它一点时间落盘再判「组件是否真在了」，避免误报
    $deadline = (Get-Date).AddSeconds(120)
    $missing = @()
    while ($true) {
        $missing = @()
        if (-not (Test-SdkComponentInstalled 'platform-tools\adb.exe' -PathType Leaf)) { $missing += 'platform-tools' }
        if (-not (Test-SdkComponentInstalled 'emulator\emulator.exe' -PathType Leaf)) { $missing += 'emulator' }
        if (-not $SkipSystemImage -and -not (Test-SdkComponentInstalled $ImageSysdir)) { $missing += $SystemImage }
        if ($missing.Count -eq 0) { break }
        if ((Get-Date) -ge $deadline) { break }
        Write-Step ("等待落盘，仍缺：" + ($missing -join ', ') + ' …')
        Start-Sleep -Seconds 5
    }
    if ($missing.Count -gt 0) {
        Write-Result 'UNUSABLE' ("组件缺失：" + ($missing -join ', ') + "；见上方 sdkmanager 输出")
        exit 2
    }
    Write-Step 'sdkmanager 安装完成'
}

# --- 复用既有 AVD：config.ini 里的旧 SDK 绝对路径（skin.path）会让 emulator 直接起不来 ---
# 实测（emulator 37.1.11）：这些 AVD 的 skin.path 指向已删除的 `D:\Program Files\android\SDK\skins\pixel_4a`，
# 或指向本 SDK 并不存在的皮肤名，emulator 会在启动检查后直接 `ERROR | unknown skin name` 退出。
# 新版 emulator 不再随 SDK 分发 pixel_* 皮肤（本机只带 resources\skins\android-36），所以正解是
# **删掉这两行**，让 emulator 用 AVD 自己的设备档案（hw.lcd.width/height/density）决定屏幕尺寸。
# 只在「皮肤确实解析不到」时删，指向本 SDK 且真实存在的 skin 保持不动。
function Repair-AvdConfigs {
    $avdHome = if ($env:ANDROID_AVD_HOME) { $env:ANDROID_AVD_HOME } else { Join-Path $env:ANDROID_USER_HOME 'avd' }
    if (-not (Test-Path -LiteralPath $avdHome -PathType Container)) {
        Write-Step "无既有 AVD 目录（$avdHome），跳过 AVD 修正"
        return
    }
    $skinsRels = @('skins', 'emulator\resources\skins')
    $fixed = @()
    Get-ChildItem -LiteralPath $avdHome -Directory -Filter '*.avd' -ErrorAction SilentlyContinue | ForEach-Object {
        $configPath = Join-Path $_.FullName 'config.ini'
        if (-not (Test-Path -LiteralPath $configPath -PathType Leaf)) { return }
        $lines = Get-Content -LiteralPath $configPath

        $skinPathLine = $lines | Where-Object { $_ -match '^\s*skin\.path\s*=' } | Select-Object -First 1
        $skinNameLine = $lines | Where-Object { $_ -match '^\s*skin\.name\s*=' } | Select-Object -First 1
        if (-not $skinPathLine) { return }

        $current = ($skinPathLine -split '=', 2)[1].Trim()
        $leaf = Split-Path -Leaf $current
        $resolves = $false
        foreach ($rel in $skinsRels) {
            if (Test-Path -LiteralPath (Join-Path $SdkRoot (Join-Path $rel $leaf)) -PathType Container) { $resolves = $true; break }
        }
        if ($resolves) { return }   # 皮肤在本 SDK 里真实存在：不动

        $removed = @()
        if ($skinPathLine) { $removed += "skin.path=$current" }
        if ($skinNameLine) { $removed += ('skin.name=' + ($skinNameLine -split '=', 2)[1].Trim()) }
        $newLines = $lines | Where-Object { $_ -notmatch '^\s*skin\.(path|name)\s*=' }
        Set-Content -LiteralPath $configPath -Value $newLines -Encoding UTF8
        $fixed += "$($_.Name): 删除不可解析的 [$($removed -join ', ')]（改用设备档案 hw.lcd.*）"
    }
    if ($fixed.Count -eq 0) {
        Write-Step 'AVD 皮肤配置无需修正'
    } else {
        $fixed | ForEach-Object { Write-Step "AVD 修正：$_" }
    }
}

function Write-EnvironmentHints {
    Write-Host ''
    Write-Host '把以下环境变量持久化（当前会话已生效，新开终端需自行设置或写进用户环境变量）：'
    Write-Host "  ANDROID_HOME=$SdkRoot"
    Write-Host "  ANDROID_SDK_ROOT=$SdkRoot"
    Write-Host "  PATH 追加：$PlatformTools;$EmulatorDir;$(Join-Path $LatestDir 'bin')"
    Write-Host ''
    Write-Host '手工修正 AVD（若自动修正不适用）：'
    Write-Host "  编辑 %USERPROFILE%\.android\avd\<AVD>.avd\config.ini，**删掉 skin.path / skin.name 两行**"
    Write-Host '  （新版 emulator 不再分发 pixel_* 皮肤，留在那里会直接 `ERROR | unknown skin name` 退出；'
    Write-Host '   删掉后 emulator 按 AVD 的设备档案 hw.lcd.width/height/density 决定屏幕尺寸）。image.sysdir.1 是相对路径，无需改。'
    Write-Host ''
    Write-Host '第一次用还需接受 licenses（交互式，本脚本不代跑）：'
    Write-Host "  `"$SdkManager`" --sdk_root=`"$SdkRoot`" --licenses"
}

# ================= main =================
Write-Step "SDK 根目录：$SdkRoot"
Write-Step "目标组件：platform-tools / emulator / $SystemImage"

$javaHome = Resolve-JavaHome
if (-not $javaHome) {
    Write-Result 'UNUSABLE' '找不到 JDK 17+（sdkmanager 硬要求）。装 JDK 17+ 或设 JAVA_HOME 后重跑。'
    exit 2
}
$env:JAVA_HOME = $javaHome

New-Item -ItemType Directory -Force -Path $SdkRoot | Out-Null
Set-SdkEnvironment
Install-CommandLineTools
Install-SdkPackages
Repair-AvdConfigs
Set-SdkEnvironment   # platform-tools/emulator 装好后把 PATH 补齐

$adb = Join-Path $PlatformTools 'adb.exe'
$emu = Join-Path $EmulatorDir 'emulator.exe'
$adbVersion = if (Test-Path -LiteralPath $adb) { ((& $adb version 2>&1 | Out-String) -split "`n" | Select-Object -First 1).Trim() } else { 'MISSING' }
$emuVersion = if (Test-Path -LiteralPath $emu) { ((& $emu -version 2>&1 | Out-String) -split "`n" | Where-Object { $_ -match 'Emulator version' } | Select-Object -First 1).Trim() } else { 'MISSING' }

Write-Host ''
Write-Step "adb      : $adbVersion"
Write-Step "emulator : $emuVersion"
Write-Step "sdkmanager: $SdkManager"
Write-EnvironmentHints
Write-Result 'READY' "SDK 就绪：$SdkRoot"
exit 0

