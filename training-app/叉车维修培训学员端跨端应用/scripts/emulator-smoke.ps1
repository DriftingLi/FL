<#
.SYNOPSIS
    【非门（不替代 ① 真机门）——仅前置冒烟】仿真机起机 + 装基座 + 逐页打开 + 断言 + 截图。

.DESCRIPTION
    ⚠️ **非门（不替代 ① 真机门）——仅前置冒烟：仿真机截图与无异常证据，鉴权/生物识别/厂商 ROM/真机上传路径类问题仿真机抓不到。**

    这是 agent 跑的**辅助**手段，不是验收门，**不进** .github/workflows/pr-evidence.yml 的校验器。
    口径见 docs/adr/0008-移动端验收门与证据.md（「非门辅助：仿真机前置冒烟」，2026-09-11，#883 / O2）。
    ① 真机门**仍由人在真机执行**，本脚本只负责把「人要看哪几页」缩小成一组已过滤的候选页 + 截图。

    为什么仿真机替代不了真机（#883 spike 实测）
      - 开发者工具 console 实测：`checkIsSupportSoterAuthentication:fail … 请使用真机进行开发`
      - ADR-0008 记的两次实害都是真机独有失败模式：content:// 上传 602001、证件 level NPE、生物识别强转崩溃

    抓得到：布局/渲染崩坏、页面加载期 UTS/JS 异常、导航状态、截图看版式
    抓不到：生物识别、运行时权限弹窗行为、厂商 ROM 差异、真机上传路径（content://）

    行为：起模拟器（**默认 headless**，-ShowWindow 才开窗）→ adb wait-for-device + sys.boot_completed → 安装基座 APK
    → 拉起应用并逐页打开页面清单 → 断言（进程未崩溃、无 FATAL EXCEPTION/ANR、目标页在前台）
    → 逐页截图 .ci-verify/emulator-<page>.png → 日志 .ci-verify/emulator-smoke.log
    → finally 里 adb emu kill 收尾（**无条件执行**，异常/断言失败/超时都走这条路）。

    判成败**只看输出与 logcat，不看退出码**（沿用本仓口径，见 scripts/kotlin-all-check.ps1 的同款声明）。

.PARAMETER AvdName
    要起的 AVD 名。默认 Pixel_4a_API_30（复用既有 AVD，保留其 userdata）。

.PARAMETER SdkRoot
    Android SDK 根目录（含 platform-tools/emulator）。默认 D:\android-sdk；由 scripts/android-sdk-setup.ps1 装出。

.PARAMETER Pages
    逗号分隔的页面清单（pages.json 里的 path），如 "pages/index/index,pages/login/login"。默认取入口页。

.PARAMETER ShowWindow
    **默认无窗口（headless）**：模拟器跑在 `-no-window -no-audio -no-boot-anim` 下，不弹 GUI 窗口、不抢焦点
    （这台是维护者在用的工作机，冒烟不得打断人）。仅当显式给 `-ShowWindow` 才开窗。
    截图不受影响：仍走 `adb exec-out screencap -p`（headless 下可用，已实测出图）。

.PARAMETER BaseApk
    基座 APK。默认走 HBuilderX 自带那份（含 x86/x86_64，故 x86 模拟器装得上）。

.PARAMETER SkipInstallBaseApk
    跳过装基座（假设已装过）。

.PARAMETER ResourcesDir
    已编译的 5+ 应用资源目录（HBuilderX「运行到手机或模拟器」产出的 app-android 资源），会 push 进设备让基座加载本项目。
    不给则只做「起机 + 装基座 + 起基座」级别的冒烟，逐页打开会被标记为 SKIP（不假装跑过）。

.PARAMETER PostToPr
    > 0 时把结果贴成 PR 评论，标记 `<!-- prefilter:emulator-smoke -->`（**刻意不用 gate-evidence: 前缀**，
    避免被验收门校验器误当作门证据）。

.EXAMPLE
    pwsh -NoProfile -File scripts/emulator-smoke.ps1
    pwsh -NoProfile -File scripts/emulator-smoke.ps1 -Pages "pages/index/index,pages/login/login"
    pwsh -NoProfile -File scripts/emulator-smoke.ps1 -ResourcesDir unpackage\resources\app-android -PostToPr 883

.NOTES
    退出码：0 = 通过；1 = 断言失败；2 = 环境不可用（缺 SDK/adb/AVD/APK）。
#>
[CmdletBinding()]
param(
    [string]$AvdName = 'Pixel_4a_API_30',
    [string]$SdkRoot = 'D:\android-sdk',
    [string]$Pages = 'pages/index/index',
    [switch]$ShowWindow,
    [string]$BaseApk = 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX\plugins\uniappx-launcher\base\android_base.apk',
    [switch]$SkipInstallBaseApk,
    [string]$ResourcesDir = '',
    [string]$Module = 'emulator',
    [switch]$NoArchive,
    [int]$PostToPr = 0,
    [int]$BootTimeoutSeconds = 300,
    [int]$PageSettleSeconds = 8,
    [int]$Port = 5554
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ⚠️ 非门标注（三处之一：脚本头部；另两处＝日志首行、可选 PR 评论）。改这行前先读 ADR-0008。
$NonGateBanner = '非门（不替代 ① 真机门）——仅前置冒烟：仿真机截图与无异常证据，鉴权/生物识别/厂商 ROM/真机上传路径类问题仿真机抓不到'

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$VerifyDir = Join-Path $ProjectRoot '.ci-verify'
$LogPath = Join-Path $VerifyDir 'emulator-smoke.log'
$PlatformTools = Join-Path $SdkRoot 'platform-tools'
$EmulatorExe = Join-Path $SdkRoot 'emulator\emulator.exe'
$AdbExe = Join-Path $PlatformTools 'adb.exe'
$Serial = "emulator-$Port"

# script 作用域状态：Set-StrictMode 下必须先声明再赋值，否则读未赋值变量会抛错
$script:BootedEmulator = $false
$script:EmulatorProcess = $null
$script:BaseApkPackage = ''
$script:BaseApkPackageFallback = 'io.dcloud.HBuilder'
$script:BaseApkSizeMb = 0
$script:AdbVersion = ''
$script:EmuVersion = ''
$script:ColdBootSeconds = 0
$script:DeviceModel = ''
$script:DeviceSdk = ''
$script:DeviceAbi = ''
$script:FinalStatus = ''
$script:Accel = [pscustomobject]@{ Verdict = 'UNKNOWN'; Raw = ''; Brief = '' }
$script:DrawingReady = $false
$script:SkippedPages = @()
$script:ShotRecords = @()
$script:FinalTotals = [pscustomobject]@{ Lines = 0; FatalCount = 0; AnrCount = 0; RuntimeCrashes = 0; ProcessDeaths = 0 }
$script:PageResultsAll = @()

function Write-Log {
    param([string]$Message)
    $line = "[emulator-smoke] $Message"
    Write-Host $line
    if (Test-Path -LiteralPath $VerifyDir -PathType Container) {
        Add-Content -LiteralPath $LogPath -Value $line -Encoding UTF8
    }
}

function Write-Result {
    param([string]$Status, [string]$Detail)
    Write-Log "EMULATOR_SMOKE_RESULT=$Status"
    if ($Detail) { Write-Log "  $Detail" }
}

function Fail-Environment {
    param([string]$Detail)
    Write-Result 'UNUSABLE' $Detail
    exit 2
}

function Convert-PageToFileName {
    param([string]$Page)
    return 'emulator-' + ($Page -replace '[^A-Za-z0-9._-]', '-') + '.png'
}

function Get-AdbOutput {
    param([string[]]$AdbArgs)
    # 统一走 adb -s <serial>；失败一律返回空串，让调用方自行判定
    $out = & $AdbExe -s $Serial @AdbArgs 2>&1
    return (($out | Out-String).Trim())
}

# ================= 环境自检（缺 SDK/adb/AVD/APK ⇒ exit 2） =================
function Assert-Environment {
    if (-not (Test-Path -LiteralPath $AdbExe -PathType Leaf)) {
        Fail-Environment "缺 adb：$AdbExe。先跑 scripts/android-sdk-setup.ps1 装 SDK。"
    }
    if (-not (Test-Path -LiteralPath $EmulatorExe -PathType Leaf)) {
        Fail-Environment "缺 emulator：$EmulatorExe。先跑 scripts/android-sdk-setup.ps1 装 SDK。"
    }
    $env:ANDROID_HOME = $SdkRoot
    $env:ANDROID_SDK_ROOT = $SdkRoot

    $avds = (& $EmulatorExe -list-avds 2>&1 | Out-String)
    if ($avds -notmatch [regex]::Escape($AvdName)) {
        Fail-Environment "找不到 AVD「$AvdName」。现有：" + (($avds -split "`n" | Where-Object { $_.Trim() }) -join ', ')
    }
    $global:BaseApkPath = $BaseApk
    if (-not $SkipInstallBaseApk) {
        if (-not (Test-Path -LiteralPath $BaseApk -PathType Leaf)) {
            Fail-Environment "缺基座 APK：$BaseApk"
        }
        $script:BaseApkSizeMb = [math]::Round((Get-Item -LiteralPath $BaseApk).Length / 1MB, 1)
    }
    $script:AdbVersion = ((& $AdbExe version 2>&1 | Out-String) -split "`n" | Select-Object -First 1).Trim()
    $script:EmuVersion = ((& $EmulatorExe -version 2>&1 | Out-String) -split "`n" | Where-Object { $_ -match 'Emulator version|Android emulator' } | Select-Object -First 1).Trim()
    $script:Accel = Get-AccelerationStatus
}

# ================= 加速能力探测（Windows 上需要 WHPX/Hyper-V 或 HAXM） =================
# 尽早探测：不可用时**立即**报告「需要人做什么」，不要让 agent 在那儿等一个起不来的模拟器。
# 实测结论会写进日志汇总与 PR 正文（一次性人工前置清单）。
function Get-AccelerationStatus {
    $out = (& $EmulatorExe -accel-check 2>&1 | Out-String)
    $verdict = 'UNKNOWN'
    if ($out -match 'is installed and usable') { $verdict = 'OK' }
    elseif ($out -match 'not installed|is not usable|HAXM is not installed') { $verdict = 'UNAVAILABLE' }
    $firstLine = (($out -split "`r?`n" | Where-Object { $_.Trim() }) | Select-Object -First 2) -join ' / '
    # 摘要要带上有信息量的那行（实测形如 `WHPX(10.0.26200) is installed and usable.`），
    # 否则只取前两行会得到无意义的 `accel: / 0`
    $detail = (($out -split "`r?`n" | Where-Object { $_ -match 'usable|HAXM|WHPX|Hyper-V|not installed' }) | Select-Object -First 1)
    if ($detail) { $firstLine = $detail.Trim() }
    return [pscustomobject]@{ Verdict = $verdict; Raw = $out.Trim(); Brief = $firstLine }
}
# ================= 模拟器起机 =================
function Start-Emulator {
    $stdout = Join-Path $VerifyDir 'emulator-stdout.log'
    $stderr = Join-Path $VerifyDir 'emulator-stderr.log'
    $emuArgs = @('-avd', $AvdName, '-no-snapshot', '-no-boot-anim', '-no-audio', '-port', "$Port",
                 '-gpu', 'swiftshader_indirect')
    if ($ShowWindow) { Write-Log '-ShowWindow：本次开 GUI 窗口（默认是 headless）' } else { $emuArgs += '-no-window' }

    Write-Log "启动模拟器：$EmulatorExe $($emuArgs -join ' ')"
    $proc = Start-Process -FilePath $EmulatorExe -ArgumentList $emuArgs -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $script:EmulatorProcess = $proc
    $script:BootedEmulator = $true

    Write-Log 'adb wait-for-device …'
    & $AdbExe -s $Serial wait-for-device 2>&1 | Out-Null

    Write-Log "等待 sys.boot_completed=1（上限 $BootTimeoutSeconds 秒）…"
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $booted = $false
    while ($sw.Elapsed.TotalSeconds -lt $BootTimeoutSeconds) {
        if ($proc.HasExited) {
            $tail = (Get-Content -LiteralPath $stderr -Tail 15 -ErrorAction SilentlyContinue) -join ' | '
            Fail-Environment "模拟器进程提前退出（exit $($proc.ExitCode)）。stderr 尾部：$tail"
        }
        $bc = (Get-AdbOutput @('shell', 'getprop', 'sys.boot_completed'))
        if ($bc -match '1') { $booted = $true; break }
        Start-Sleep -Seconds 3
    }
    if (-not $booted) {
        Fail-Environment "等 sys.boot_completed 超时（$BootTimeoutSeconds 秒）。见 $stdout / $stderr"
    }
    $sw.Stop()
    $script:ColdBootSeconds = [math]::Round($sw.Elapsed.TotalSeconds, 1)
    Write-Log "冷启动完成，耗时 $($script:ColdBootSeconds) 秒"

    # 关掉窗口/动画等对截图的干扰（失败不阻断）
    Get-AdbOutput @('shell', 'settings', 'put', 'global', 'window_animation_scale', '0') | Out-Null
    Get-AdbOutput @('shell', 'settings', 'put', 'global', 'transition_animation_scale', '0') | Out-Null
    Get-AdbOutput @('shell', 'settings', 'put', 'global', 'animator_duration_scale', '0') | Out-Null
    Get-AdbOutput @('shell', 'input', 'keyevent', '82') | Out-Null   # 解锁：菜单键

    $model = Get-AdbOutput @('shell', 'getprop', 'ro.product.model')
    $sdk = Get-AdbOutput @('shell', 'getprop', 'ro.build.version.sdk')
    $abi = Get-AdbOutput @('shell', 'getprop', 'ro.product.cpu.abi')
    $script:DeviceModel = $model
    $script:DeviceSdk = $sdk
    $script:DeviceAbi = $abi
    Write-Log "设备：model=$model api=$sdk abi=$abi serial=$Serial"
}

# ================= 基座 APK =================
# 基座包名/入口一律**从设备上问**（adb），不依赖 aapt2/build-tools：
# 少一份工具链依赖，也不用解析二进制 AndroidManifest。装不上才回落到已知常量。
function Resolve-BaseApkOnDevice {
    $pkgs = (& $AdbExe -s $Serial shell pm list packages 2>&1 | Out-String)
    # 基座与「已装的本项目 5+ 应用」可能不同包名；优先带 dcloud/HBuilder 字样的，其次回落常量
    $candidates = @()
    foreach ($line in ($pkgs -split "`r?`n")) {
        $m = [regex]::Match($line, '^package:(\S+)$')
        if (-not $m.Success) { continue }
        $name = $m.Groups[1].Value
        if ($name -match 'dcloud|HBuilder' -or $name -eq $script:BaseApkPackageFallback) { $candidates += $name }
    }
    if ($candidates.Count -eq 0) {
        Write-Log "设备上未找到基座包（回落 $($script:BaseApkPackageFallback)）：$($pkgs.Trim() -split "`n" | Select-Object -First 5)"
        return $script:BaseApkPackageFallback
    }
    $chosen = $candidates[0]
    if ($chosen -ne $script:BaseApkPackageFallback) {
        Write-Log "设备上基座包名=$chosen（与回落常量 $($script:BaseApkPackageFallback) 不同，以设备为准）"
    }
    return $chosen
}

function Resolve-LauncherComponent {
    param([string]$Package)
    # 先问系统的 resolved activity，拿不到再猜 PandoraEntry（DCloud 基座的入口类名）
    $out = (& $AdbExe -s $Serial shell cmd package resolve-activity --brief -a android.intent.action.MAIN -c android.intent.category.LAUNCHER $Package 2>&1 | Out-String)
    $m = [regex]::Match($out, '([A-Za-z0-9_.]+/[A-Za-z0-9_.$]+)')
    if ($m.Success) { return $m.Groups[1].Value }
    return "$Package/$Package.PandoraEntry"
}function Install-BaseApk {
    if ($SkipInstallBaseApk) {
        Write-Log '跳过装基座（-SkipInstallBaseApk）'
        return
    }
    Write-Log "安装基座 APK（$($script:BaseApkSizeMb) MB）：$BaseApk"
    $out = (& $AdbExe -s $Serial install -r -t $BaseApk 2>&1 | Out-String)
    if ($out -notmatch 'Success') {
        Fail-Environment "基座 APK 安装失败：$($out.Trim())"
    }
    Write-Log "基座安装结果：$($out.Trim())"
}

function Push-Resources {
    if (-not $ResourcesDir) {
        Write-Log '未给 -ResourcesDir：跳过资源 push，逐页打开将标记 SKIP（不假装跑过）'
        return $false
    }
    $resolved = if ([System.IO.Path]::IsPathRooted($ResourcesDir)) { $ResourcesDir } else { Join-Path $ProjectRoot $ResourcesDir }
    if (-not (Test-Path -LiteralPath $resolved -PathType Container)) {
        Write-Log "资源目录不存在，跳过：$resolved"
        return $false
    }
    $target = '/sdcard/Android/data/' + $script:BaseApkPackage + '/apps/__UNI__1C1D180/www'
    Write-Log "push 应用资源：$resolved → $target"
    & $AdbExe -s $Serial shell mkdir -p $target 2>&1 | Out-Null
    $out = (& $AdbExe -s $Serial push $resolved/. $target 2>&1 | Out-String)
    if ($out -notmatch 'pushed|skipped') {
        Write-Log "资源 push 结果可疑：$($out.Trim())"
        return $false
    }
    Write-Log '资源 push 完成'
    return $true
}

# ================= 断言辅助 =================
function Get-ForegroundActivity {
    $out = (& $AdbExe -s $Serial shell dumpsys activity activities 2>&1 | Out-String)
    $m = [regex]::Match($out, 'topResumedActivity[^\n]*?([A-Za-z0-9_.]+/[A-Za-z0-9_.]+)')
    if ($m.Success) { return $m.Groups[1].Value }
    $out2 = (& $AdbExe -s $Serial shell dumpsys window 2>&1 | Out-String)
    $m2 = [regex]::Match($out2, 'mCurrentFocus[^\n]*?([A-Za-z0-9_.]+/[A-Za-z0-9_.]+)')
    if ($m2.Success) { return $m2.Groups[1].Value }
    return ''
}

function Test-ProcessAlive {
    $out = Get-AdbOutput @('shell', 'pidof', $script:BaseApkPackage)
    return ($out -match '\d')
}

# logcat 摘要：只统计「本次冒烟期间」的崩溃/无响应证据（脚本开头已 logcat -c 清过）
function Get-LogcatSummary {
    $dump = (& $AdbExe -s $Serial logcat -d -v brief 2>&1 | Out-String)
    $lines = $dump -split "`r?`n"
    $fatal = @($lines | Where-Object { $_ -match 'FATAL EXCEPTION' })
    # ANR 只在**打给基座自己**时才算失败：`ANR in <pkg>` 里的 <pkg> 是卡住的进程。
    # 实测（swiftshader 软件渲染）系统进程/输入法也会被记 ANR；把它们算到本应用头上是假阳性。
    $allAnr = @($lines | Where-Object { $_ -match 'ANR in ' })
    $anr = @($allAnr | Where-Object { $_ -match ('ANR in ' + [regex]::Escape($script:BaseApkPackage) + '($|[\s(])') })
    $anrOther = @($allAnr | Where-Object { $anr -notcontains $_ })
    $runtimeCrash = @($lines | Where-Object { $_ -match 'E AndroidRuntime' })
    $died = @($lines | Where-Object { $_ -match 'has died' })
    return [pscustomobject]@{
        Lines           = $lines.Count
        FatalCount      = $fatal.Count
        AnrCount        = $anr.Count
        AnrOtherCount   = $anrOther.Count
        RuntimeCrashes  = $runtimeCrash.Count
        ProcessDeaths   = $died.Count
        FatalSamples    = @($fatal | Select-Object -First 5)
        AnrSamples      = @($anr | Select-Object -First 5)
        AnrOtherSamples = @($anrOther | Select-Object -First 5)
        Raw             = $dump
    }
}

function Export-Screenshot {
    param([string]$Page)
    $name = Convert-PageToFileName -Page $Page
    $path = Join-Path $VerifyDir $name
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
    # PNG 是二进制：必须走 cmd 重定向，PowerShell 的 > 会破坏字节流
    $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $AdbExe, $Serial, $path
    & cmd.exe /c $cmd 2>&1 | Out-Null
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        $size = (Get-Item -LiteralPath $path).Length
        Write-Log "截图：$name ($([math]::Round($size / 1KB, 1)) KB)"
        return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = $size }
    }
    Write-Log "截图失败：$name（文件未生成）"
    return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = 0 }
}


# ================= 截图入库（非门证据，可选） =================
# 上限（维护者硬性要求）：宽度 ≤720px（等比、不放大）、单张 ≤150 KB、每 PR ≤10 张且合计 ≤1.5 MB。
# 优先 WebP q75（探测 cwebp / ffmpeg / magick），没有就退 JPEG q75（.NET System.Drawing）。
$ArchiveMaxWidth = 720
$ArchiveMaxBytes = 150 * 1024
$ArchiveMaxCount = 10
$ArchiveMaxTotalBytes = 1536 * 1024

function Get-WebpEncoder {
    foreach ($name in @('cwebp', 'ffmpeg', 'magick')) {
        $cmd = Get-Command $name -ErrorAction SilentlyContinue
        if ($cmd) { return @{ Name = $name; Path = $cmd.Source } }
    }
    return $null
}

function Load-Drawing {
    if ($script:DrawingReady) { return $true }
    try {
        Add-Type -AssemblyName System.Drawing
        $script:DrawingReady = $true
        return $true
    } catch {
        Write-Log "System.Drawing 不可用，跳过入库：$($_.Exception.Message)"
        return $false
    }
}

function Resize-Bitmap {
    param([System.Drawing.Image]$Image, [int]$MaxWidth)
    $w = $Image.Width
    $h = $Image.Height
    if ($w -gt $MaxWidth) {
        $h = [int][math]::Round($h * ($MaxWidth / [double]$w))
        $w = $MaxWidth
    }
    $bmp = New-Object System.Drawing.Bitmap $w, $h
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.DrawImage($Image, 0, 0, $w, $h)
    $g.Dispose()
    return $bmp
}

function Save-JpegWithQuality {
    param([System.Drawing.Bitmap]$Bitmap, [string]$Path, [int]$Quality)
    $codec = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' }
    $encoderParams = New-Object System.Drawing.Imaging.EncoderParameters 1
    $encoderParams.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter ([System.Drawing.Imaging.Encoder]::Quality), ([int]$Quality)
    $Bitmap.Save($Path, $codec, $encoderParams)
    $encoderParams.Dispose()
}

function Save-WebpViaTool {
    param([System.Drawing.Bitmap]$Bitmap, [string]$PngTemp, [string]$OutPath, [hashtable]$Enc, [int]$Quality)
    $Bitmap.Save($PngTemp, [System.Drawing.Imaging.ImageFormat]::Png)
    switch ($Enc.Name) {
        'cwebp' { & $Enc.Path -q $Quality -quiet $PngTemp -o $OutPath 2>&1 | Out-Null }
        'ffmpeg' { & $Enc.Path -y -loglevel error -i $PngTemp -quality $Quality $OutPath 2>&1 | Out-Null }
        'magick' { & $Enc.Path $PngTemp -quality $Quality $OutPath 2>&1 | Out-Null }
    }
    return (Test-Path -LiteralPath $OutPath -PathType Leaf)
}

# 超限顺序：降质 → 缩尺 → 减图（减图由调用方按顺序裁）
function Compress-Screenshot {
    param([string]$SourcePath, [string]$OutDir, [string]$BaseName, [hashtable]$Encoder)
    $attempts = @(
        @{ Quality = 75; MaxWidth = $ArchiveMaxWidth; Label = 'q75/720' },
        @{ Quality = 60; MaxWidth = $ArchiveMaxWidth; Label = 'q60/720' },
        @{ Quality = 75; MaxWidth = 560;            Label = 'q75/560' },
        @{ Quality = 60; MaxWidth = 420;            Label = 'q60/420' }
    )
    $img = $null
    try { $img = [System.Drawing.Image]::FromFile($SourcePath) } catch {
        Write-Log "  读数失败，跳过入库：$($_.Exception.Message)"
        return $null
    }
    $result = $null
    foreach ($attempt in $attempts) {
        $bmp = $null
        try {
            $bmp = Resize-Bitmap -Image $img -MaxWidth $attempt.MaxWidth
            $suffix = $attempt.Label -replace '[/\\]', '-'
            $ext = if ($Encoder) { 'webp' } else { 'jpg' }
            $candidate = Join-Path $OutDir ("$BaseName-$suffix.$ext")
            $ok = $false
            if ($Encoder) {
                $pngTemp = Join-Path $env:TEMP ("smoke-shot-" + [guid]::NewGuid().ToString('N') + '.png')
                $ok = Save-WebpViaTool -Bitmap $bmp -PngTemp $pngTemp -OutPath $candidate -Enc $Encoder -Quality $attempt.Quality
                Remove-Item -LiteralPath $pngTemp -Force -ErrorAction SilentlyContinue
            } else {
                Save-JpegWithQuality -Bitmap $bmp -Path $candidate -Quality $attempt.Quality
                $ok = $true
            }
            if ($ok -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
                $bytes = (Get-Item -LiteralPath $candidate).Length
                if ($bytes -le $ArchiveMaxBytes) {
                    $result = [pscustomobject]@{
                        Name = (Split-Path -Leaf $candidate); Path = $candidate
                        Width = $bmp.Width; Height = $bmp.Height; Bytes = $bytes
                        Format = $ext; Attempt = $attempt.Label
                    }
                    break
                }
                Remove-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
            }
        } catch {
            Write-Log "  压缩失败（$($attempt.Label)）：$($_.Exception.Message)"
        } finally {
            if ($bmp) { $bmp.Dispose() }
        }
    }
    $img.Dispose()
    return $result
}

function Invoke-Git {
    param([string[]]$GitArgs)
    $out = & git -C $ProjectRoot @GitArgs 2>&1
    return (($out | Out-String).Trim())
}

function Archive-Screenshots {
    param([array]$Shots, [int]$PrNumber)
    if ($NoArchive) { Write-Log '入库：已按 -NoArchive 跳过'; return @() }
    if (-not $Shots -or @($Shots).Count -eq 0) { Write-Log '入库：没有截图可存'; return @() }

    $usable = @($Shots | Where-Object { $_.Bytes -gt 0 })
    if (@($usable).Count -eq 0) { Write-Log '入库：截图均为 0 字节，跳过'; return @() }

    # 硬前提：当前分支是该 PR 的 head 且工作树无其他改动；否则只警告，不提交
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Write-Log '入库：无 git，跳过'; return @() }
    $status = Invoke-Git @('status', '--porcelain')
    if ($status) {
        Write-Log "入库：工作树有未提交改动，跳过入库（不影响冒烟结论）。待提交：$(($status -split "`n" | Select-Object -First 5) -join '; ')"
        return @()
    }
    $branch = Invoke-Git @('rev-parse', '--abbrev-ref', 'HEAD')
    $prHead = ''
    try { $prHead = ((& gh pr view $PrNumber --json headRefName -q .headRefName 2>&1 | Out-String).Trim()) } catch { }
    if (-not $prHead) {
        Write-Log "入库：取不到 PR #$PrNumber 的 head 分支，跳过"
        return @()
    }
    if ($branch -ne $prHead) {
        Write-Log "入库：当前分支 $branch ≠ PR head $prHead，跳过入库"
        return @()
    }

    $encoder = Get-WebpEncoder
    if ($encoder) { Write-Log "入库编码器：$($encoder.Name)（WebP）" }
    else { Write-Log '入库编码器：无 cwebp/ffmpeg/magick ⇒ 退 JPEG q75（.NET System.Drawing）' }

    $targetDir = Join-Path $ProjectRoot (Join-Path 'docs\verification' (Join-Path $Module "$PrNumber"))
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

    # 减图：最多 $ArchiveMaxCount 张（按页面顺序取前 N），被省掉的要写明
    $skippedPages = @()
    if ($usable.Count -gt $ArchiveMaxCount) {
        $skippedPages = @($usable[$ArchiveMaxCount..($usable.Count - 1)] | ForEach-Object { $_.Page })
        $usable = @($usable[0..($ArchiveMaxCount - 1)])
    }

    $entries = @()
    foreach ($shot in $usable) {
        $base = 'smoke-' + ($shot.Page -replace '[^A-Za-z0-9._-]', '-')
        $compressed = Compress-Screenshot -SourcePath $shot.Path -OutDir $targetDir -BaseName $base -Encoder $encoder
        if (-not $compressed) {
            Write-Log "  入库失败（超限或编码失败），跳过：$($shot.Page)"
            continue
        }
        $entries += $compressed
        Write-Log ("  入库：" + $compressed.Name + "  $($compressed.Width)x$($compressed.Height)  $([math]::Round($compressed.Bytes / 1KB, 1)) KB  " + $compressed.Format + "  [" + $compressed.Attempt + "]")
    }

    # 合计超限：从尾部继续减图
    while ((@($entries | Measure-Object -Property Bytes -Sum).Sum) -gt $ArchiveMaxTotalBytes -and $entries.Count -gt 1) {
        $dropped = $entries[$entries.Count - 1]
        Remove-Item -LiteralPath $dropped.Path -Force -ErrorAction SilentlyContinue
        $entries = @($entries[0..($entries.Count - 2)])
        $skippedPages += ($dropped.Name -replace '^smoke-', '')
        Write-Log "  合计超 1.5 MB，减图移除：$($dropped.Name)"
    }

    if ($entries.Count -eq 0) { Write-Log '入库：压缩后没有可用图，跳过'; return @() }

    try {
        $rel = @()
        foreach ($e in $entries) {
            $r = $e.Path.Substring($ProjectRoot.Length).TrimStart('\', '/') -replace '\\', '/'
            $rel += $r
        }
        Invoke-Git (@('add') + $rel) | Out-Null
        $msg = "chore(verify): 归档仿真机冒烟截图（非门，不替代 ① 真机门）PR #$PrNumber"
        Invoke-Git (@('commit', '-m', $msg)) | Out-Null
        $push = Invoke-Git @('push', 'origin', $branch)
        Write-Log "入库：已提交并推送 $(@($rel).Count) 张到 $branch"
    } catch {
        Write-Log "入库提交/推送失败（不影响结论）：$($_.Exception.Message)"
    }
    if (@($skippedPages).Count -gt 0) {
        Write-Log "入库：因上限省掉 $($skippedPages.Count) 张：$($skippedPages -join ', ')"
    }
    $script:SkippedPages = $skippedPages
    return $entries
}


function Publish-PrefilterComment {
    param([int]$PrNumber, [string]$Body)
    try {
        $sha = (git -C $ProjectRoot rev-parse HEAD 2>&1 | Out-String).Trim()
        $body = $Body + "`n`n- commit: $sha"
        # 刻意用 prefilter: 前缀，而不是验收门证据那套标记，避免被校验器当成门证据
        $body = "<!-- prefilter:emulator-smoke -->`n" + $body
        $tmp = Join-Path $env:TEMP ('emulator-smoke-comment-' + [guid]::NewGuid().ToString('N') + '.md')
        [System.IO.File]::WriteAllText($tmp, $body, [System.Text.UTF8Encoding]::new($false))
        $out = (gh pr comment $PrNumber --body-file $tmp 2>&1 | Out-String)
        Write-Log "PR 评论结果：$($out.Trim())"
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    } catch {
        Write-Log "贴 PR 评论失败（不影响结论）：$($_.Exception.Message)"
    }
}

# ================= main =================
New-Item -ItemType Directory -Force -Path $VerifyDir | Out-Null
# 日志首行＝非门标注（三处之二；另两处＝脚本头部、可选 PR 评论）
Set-Content -LiteralPath $LogPath -Value ("[emulator-smoke] " + $NonGateBanner) -Encoding UTF8
Write-Log ('运行时间：' + (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'))
Write-Log "AVD=$AvdName SDK=$SdkRoot 页数=$(($Pages -split ',').Count)"

$exitCodes = @{ Pass = 0; Assert = 1; Unusable = 2 }
$pageResults = @()
$failures = @()
$skips = @()

try {
    Assert-Environment
    Write-Log "adb      : $($script:AdbVersion)"
    Write-Log "emulator : $($script:EmuVersion)"

    Write-Log "基座 APK 来源：$BaseApk"

    Start-Emulator
    & $AdbExe -s $Serial logcat -c 2>&1 | Out-Null
    Write-Log 'logcat 已清空（崩溃计数只统计本次冒烟期间）'

    Install-BaseApk

    # 装完才问设备要包名/入口（基座包名以设备为准，不靠猜）
    $script:BaseApkPackage = Resolve-BaseApkOnDevice
    $launcherComponent = Resolve-LauncherComponent -Package $script:BaseApkPackage
    Write-Log "基座 package=$($script:BaseApkPackage) launcher=$launcherComponent"
    $alive = Test-ProcessAlive
    if (-not $alive) {
        # 基座装完先冷启一次，确认「能起」这一档本身是通的
        Get-AdbOutput @('shell', 'am', 'start', '-n', $launcherComponent) | Out-Null
        Start-Sleep -Seconds 5
        Write-Log "基座冷启后进程存活=$(Test-ProcessAlive)"
    }

    $hasResources = Push-Resources
    if ($hasResources) {
        Get-AdbOutput @('shell', 'am', 'force-stop', $script:BaseApkPackage) | Out-Null
    }

    $pageList = @($Pages -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    $shotRecords = @()
    foreach ($page in $pageList) {
        Write-Log "── 打开页面：$page"
        $intentArgs = @('shell', 'am', 'start', '-n', $launcherComponent, '-a', 'android.intent.action.VIEW',
                        '-d', "uniapp://$page", '--ez', 'dcloud_open_url', 'true', '--es', 'dcloud_page', $page)
        $startOut = Get-AdbOutput $intentArgs
        Write-Log "am start：$(($startOut -split "`n" | Select-Object -First 2) -join ' / ')"

        Start-Sleep -Seconds $PageSettleSeconds

        $foreground = Get-ForegroundActivity
        $alive = Test-ProcessAlive
        $logcat = Get-LogcatSummary
        $shot = Export-Screenshot -Page $page
        $shotRecords += $shot

        $pageFail = @()
        if (-not $alive) { $pageFail += '基座进程不存活（进程已退出）' }
        if ($logcat.FatalCount -gt 0) { $pageFail += "logcat 有 FATAL EXCEPTION × $($logcat.FatalCount)" }
        if ($logcat.AnrCount -gt 0) { $pageFail += "logcat 有 ANR × $($logcat.AnrCount)" }
        if ($logcat.RuntimeCrashes -gt 0) { $pageFail += "logcat 有 AndroidRuntime 崩溃 × $($logcat.RuntimeCrashes)" }
        if ($foreground -and $foreground -notmatch [regex]::Escape($script:BaseApkPackage)) {
            $pageFail += "前台 Activity 不是基座（$foreground）"
        }

        $status = 'PASS'
        if (@($pageFail).Count -gt 0) {
            $status = 'FAIL'
            $failures += "$page → $($pageFail -join '；')"
            $pageFail | ForEach-Object { Write-Log "  ✗ $_" }
        }
        if (-not $hasResources) {
            $status = 'SKIP'
            $skips += "$page（未提供 -ResourcesDir，未验证页面打开）"
            Write-Log '  ⚠ SKIP：未提供 -ResourcesDir，本次只验证到「基座能起、无崩溃」'
        }
        Write-Log "  进程存活=$alive 前台=$foreground FATAL=$($logcat.FatalCount) ANR=$($logcat.AnrCount) 截图=$($shot.Name) 判定=$status"

        $pageResults += [pscustomobject]@{ Page = $page; Status = $status; Bytes = $shot.Bytes; Fatal = $logcat.FatalCount; Anr = $logcat.AnrCount; Alive = $alive }
    }

    $final = Get-LogcatSummary
    $summary = @(
        '',
        '── 冒烟汇总（非门，不替代 ① 真机门）──',
        "AVD=$AvdName  model=$($script:DeviceModel)  api=$($script:DeviceSdk)  abi=$($script:DeviceAbi)",
        "硬件加速：$($script:Accel.Verdict)（$($script:Accel.Brief)）",
        "冷启动耗时=$($script:ColdBootSeconds) 秒（headless=$(-not $ShowWindow)）",
        "基座 APK=$($script:BaseApkPackage)  安装=$(if ($SkipInstallBaseApk) { '已跳过' } else { "已安装 ($($script:BaseApkSizeMb) MB)" })",
        "本次 logcat 总行数=$($final.Lines)  FATAL EXCEPTION=$($final.FatalCount)  ANR=$($final.AnrCount)  AndroidRuntime=$($final.RuntimeCrashes)  进程死亡=$($final.ProcessDeaths)",
        "页面判定：PASS=$(@($pageResults | Where-Object { $_.Status -eq 'PASS' }).Count)  FAIL=$(@($pageResults | Where-Object { $_.Status -eq 'FAIL' }).Count)  SKIP=$(@($pageResults | Where-Object { $_.Status -eq 'SKIP' }).Count)",
        "截图目录=$VerifyDir",
        "日志=$LogPath"
    )
    $summary | ForEach-Object { Write-Log $_ }

    $script:FinalTotals = $final
    $script:PageResultsAll = $pageResults
    $passed = ($failures.Count -eq 0)
    $statusWord = 'PASS'
    if (-not $passed) { $statusWord = 'FAIL' }
    Write-Result $statusWord ("页面 " + (($pageResults | ForEach-Object { "$($_.Page)=$($_.Status)" }) -join ', '))
    $script:FinalStatus = $statusWord
} catch {
    Write-Log "脚本异常：$($_.Exception.Message)"
    Write-Result 'UNUSABLE' $_.Exception.Message
    $script:FinalStatus = 'UNUSABLE'
} finally {
    # 收尾**无条件**执行：断言失败、超时、异常都走这里，绝不留下跑着的模拟器
    if ($script:BootedEmulator) {
        Write-Log '收尾：adb emu kill'
        try { & $AdbExe -s $Serial emu kill 2>&1 | Out-Null } catch { Write-Log "emu kill 失败：$($_.Exception.Message)" }
        try { & $AdbExe -s $Serial wait-for-disconnect 2>&1 | Out-Null } catch { }
    }
    if ($script:EmulatorProcess -and -not $script:EmulatorProcess.HasExited) {
        try { $script:EmulatorProcess.WaitForExit(30000) | Out-Null } catch { }
        if (-not $script:EmulatorProcess.HasExited) {
            try { $script:EmulatorProcess.Kill() } catch { }
        }
    }
    Write-Log '收尾完成'
}

if ($PostToPr -gt 0) {
    # 截图入库（非门证据）：压缩后提交到当前 PR 分支 docs/verification/<模块>/<PR号>/
    $archived = @()
    if ($script:FinalStatus -eq 'PASS' -or $script:FinalStatus -eq 'FAIL') {
        $archived = Archive-Screenshots -Shots $script:ShotRecords -PrNumber $PostToPr
    }
    $shotLines = @()
    if (@($archived).Count -gt 0) {
        $shotLines += "- 截图（已入库，仓库内相对路径）："
        foreach ($a in $archived) {
            $r = $a.Path.Substring($ProjectRoot.Length).TrimStart('\', '/') -replace '\\', '/'
            $shotLines += "  - ``$r`` — $($a.Width)x$($a.Height)，$([math]::Round($a.Bytes / 1KB, 1)) KB，$($a.Format)"
        }
    } else {
        $shotLines += '- 截图：未入库（本地 .ci-verify/，原因见 .ci-verify/emulator-smoke.log）'
    }
    if (@($script:SkippedPages).Count -gt 0) {
        $shotLines += "- 因上限（≤10 张 / ≤150 KB / 合计 ≤1.5 MB）省掉的页：$($script:SkippedPages -join ', ')"
    }

    $totals = $script:FinalTotals
    $bodyLines = @(
        "**$NonGateBanner**",
        '',
        '### 仿真机前置冒烟（非门 · prefilter · agent 执行）',
        '',
        '本结果**不是**验收门证据，**不替代 ① 真机门**；① 仍由人在真机执行，本项只用来缩小人要看哪几页。',
        '下列截图是**非门**前置冒烟产物，**不替代 ① 真机截图**。',
        '',
        "- AVD：$AvdName（model=$($script:DeviceModel) api=$($script:DeviceSdk) abi=$($script:DeviceAbi)）",
        "- 硬件加速：$($script:Accel.Verdict)（$($script:Accel.Brief)）",
        "- 冷启动耗时：$($script:ColdBootSeconds) 秒（headless=$(-not $ShowWindow)，截图走 adb exec-out screencap -p）",
        "- logcat：FATAL EXCEPTION=$($totals.FatalCount) / ANR=$($totals.AnrCount) / AndroidRuntime=$($totals.RuntimeCrashes)（详见 .ci-verify/emulator-smoke.log）",
        "- 页面判定：$(($script:PageResultsAll | ForEach-Object { "$($_.Page)=$($_.Status)" }) -join '，')",
        "- 复现：pwsh -NoProfile -File scripts/emulator-smoke.ps1 -Pages `"$Pages`""
    )
    $bodyLines += $shotLines
    $bodyLines += @('', '抓不到：生物识别、运行时权限弹窗行为、厂商 ROM 差异、真机上传路径（content://）。')
    $body = ($bodyLines -join "`n")
    Publish-PrefilterComment -PrNumber $PostToPr -Body $body
}

if ($script:FinalStatus -eq 'FAIL') { exit $exitCodes.Assert }
if ($script:FinalStatus -eq 'UNUSABLE') { exit $exitCodes.Unusable }
exit $exitCodes.Pass












