<#
.SYNOPSIS
    【①a 预置 · 非门（不替代 ①b：指纹等关键交互仍由人走）】真机**只读取证**：逐页截图 + logcat 断言 + 前台 Activity。

.DESCRIPTION
    ⚠️ **本脚本是「①a 真机自动取证」的预置件（2026-09-12，为 PR #624 备料），不是验收门，也不替代 ①b。**

    ⚠️ **非门（不替代 ①b：指纹等关键交互仍由人走）——它只替人省掉「逐页截图与抓日志」这一步。**

    ① 真机门已决定拆成两半（口径见 docs/adr/0008-移动端验收门与证据.md「①a / ①b」节）：

    - **①a** = agent 用 adb 自动逐页截图 + logcat 断言 + 记录前台 Activity（**本脚本**）
    - **①b** = **人**走关键交互（生物识别指纹、运行时权限弹窗、真机上传路径）——**本脚本机械上无法替代**

    本脚本**只**替代 ① 的「截图与日志取证」，**不**替代 ①b。

    ### 机械上无法替代真机生物识别的理由（#883 spike 实测，引用其 devtools console 证据）
    spike #883 的开发者工具 console 实测报 `checkIsSupportSoterAuthentication:fail … 请使用真机进行开发`；
    更根本的是：指纹要**人把手指按上去**才产生一次成功/失败事件，而 adb 里**没有任何只读命令**能代替这次按压
    （本脚本一律禁用的 adb shell input 类事件注入也不会触发指纹传感器）。因而不放宽 ①：本脚本产出的是
    **候选页截图 + 无异常证据**，指纹那一步仍由人按，结论仍由人写。

    ## 只读禁令（本脚本的红线；改之前先读这一段）
    本脚本可能正跑在维护者**正在调试的真机上**（先例：#781 的 ai-basic UI 对齐调试），任何写操作都会打断他的会话。故**绝不**：

    - 不 kill-server：会顶掉维护者与其它工具共享的 adb server 连接（**不要**改用 HBuilderX 自带的那份 adb，同理）
    - 不 install / 不 uninstall：改设备上的应用状态
    - 不 force-stop / 默认不 am start：杀进程或抢前台会让调试会话断开
    - 不 logcat -c：会清掉维护者正在看的崩溃输出
    - 不 push / pull / reboot / input：改设备状态或注入输入事件

    原因一句话：**取证不该改变被取证的对象**——而维护者的调试会话本身正是要被取证的对象。

    只用只读命令：adb devices、adb -s <dev> exec-out screencap -p、adb -s <dev> shell dumpsys …、
    adb -s <dev> shell logcat -d（-d = dump 后立即退出，**不清**缓冲）。

    ### 只读带来的连带结论：无法清 logcat 缓冲 ⇒ 不能「清缓冲后再统计」
    处置（**不假装能清**）：脚本启动时先取**设备自己**最新日志行的 epoch 时间戳作窗口起点
    （logcat -d -v epoch -t 1），结束时只统计大于等于起点的行；-LogcatSeconds > 0 时再取
    「最近 N 秒」与「启动以来」的**交集**。起点取自设备本身，故设备时钟与宿主时钟不同步不影响结果。
    若连起点都取不到（缓冲为空），本次**不做崩溃判据**并在结果里写 logcat=SKIP——不拿历史日志冒充本次。

    ## 切页默认关闭
    **切页动作默认关闭：会打断维护者的调试会话。** 只有显式给 -AllowAppStart 才发 am start；
    给了 -NoStartApp / -SkipAppStart 则**无论如何**都不发（即便同时给了 -AllowAppStart，**以禁止为准**，fail-safe）。

    默认（只读）模式的行为：不切页，仅对**当前前台**采 1 张图（-Pages 只有一页时用 该页-current 命名），
    -Pages 里每一页在报告里标 SKIP（未切页：切页默认关闭）——**不假装跑过**。

    ## 判成败（只看输出里的 DEVICE_CAPTURE_RESULT=）
    断言（不满足 ⇒ exit 1）：前台 Activity 可取得、截图落盘且非 0 字节、窗口内无 FATAL EXCEPTION、
    窗口内无 ANR in 前台包名。
    环境不满足（缺 adb / 无设备 / 多设备未指定 -Device / 设备非 online）⇒ exit 2。
    **截图压缩与入库失败一律只警告、不影响结论**（-NoArchive 可整体跳过）。

    ## 用法
    pwsh -NoProfile -File scripts/device-capture.ps1 -Device 192.168.1.26:46701 -Pages "pages/login/login"
    pwsh -NoProfile -File scripts/device-capture.ps1 -PostToPr 624
    pwsh -NoProfile -File scripts/device-capture.ps1 -AllowAppStart -Package io.dcloud.uniappx -Pages "pages/login/login,pages/index/index"

.PARAMETER Device
    目标设备 serial（adb devices 第一列）。**多设备时必须显式给**，否则 exit 2（不做自动猜测）。
    注意：同一台真机可能同时以 <ip:port> 与 adb-<hash>._adb-tls-connect._tcp **两条** transport 出现
    （无线调试 + mDNS 自动发现），实测本机就是这种形态 ⇒ 显式指定才不会误判为「多设备」。
    不给时要求恰好**一个** online 设备，否则 exit 2。

.PARAMETER Pages
    逗号分隔的页面清单（pages.json 里的 path），如 "pages/login/login,pages/index/index"。默认登录页。

.PARAMETER AdbPath
    adb 路径。默认 D:\android-sdk\platform-tools\adb.exe（**与 HBuilderX 共用同一个 server**）。
    ⚠️ 不要改成 HBuilderX 自带的那份 adb：会起第二个 server 顶掉维护者的连接。

.PARAMETER OutDir
    本地取证输出目录（日志 + 原始 PNG）。默认 .ci-verify（相对项目根）。

.PARAMETER AllowAppStart
    显式开启 am start 切页。**默认关闭**。会打断维护者正在进行的调试会话，慎用。

.PARAMETER NoStartApp
.PARAMETER SkipAppStart
    两个等价开关：**禁止一切 am start**。优先级高于 -AllowAppStart（同时给则以禁止为准，fail-safe）。

.PARAMETER Package
    切页时目标包名。不给则用**当前前台**的包名（即维护者正在调试的那个应用）；仍取不到才 exit 2。

.PARAMETER LogcatSeconds
    logcat 时间窗上限（秒）。0（默认）= 从脚本启动那一刻起；大于 0 = 取「最近 N 秒」与「启动以来」的交集。

.PARAMETER PageSettleSeconds
    -AllowAppStart 切页后等待页面稳定的秒数（默认 3）。只读模式不等待。

.PARAMETER Module
    报告与归档子目录名。默认 device ⇒ docs/verification/device/<PR号>/。

.PARAMETER ArchiveModule
    单独覆盖归档子目录名；不给则用 -Module。

.PARAMETER NoArchive
    逃生开关：跳过截图压缩入库。

.PARAMETER PostToPr
    大于 0 时把结果贴成 PR 评论，标记 <!-- prefilter:device-capture -->（**刻意不用验收门证据那套前缀**，
    避免被 .github/workflows/pr-evidence.yml 当成验收门证据——本次**不改**门校验器）。

.NOTES
    退出码：0 = 通过；1 = 断言失败；2 = 环境不可用。
    实测状态：**设备探测 / 截图 / dumpsys / logcat -d 只读路径已实测**（2026-09-12，见 PR 正文）；
    -AllowAppStart 切页分支与归档入库**待跑**（维护者当时正在用这台真机调试 #781，不得打断）。
#>
[CmdletBinding()]
param(
    [string]$Device = '',
    [string]$Pages = 'pages/login/login',
    [string]$AdbPath = 'D:\android-sdk\platform-tools\adb.exe',
    [string]$OutDir = '.ci-verify',
    [switch]$AllowAppStart,
    [switch]$NoStartApp,
    [switch]$SkipAppStart,
    [string]$Package = '',
    [int]$LogcatSeconds = 0,
    [int]$PageSettleSeconds = 3,
    [string]$Module = 'device',
    [string]$ArchiveModule = '',
    [switch]$NoArchive,
    [int]$PostToPr = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# 非门标注（三处之一：脚本头部；另两处＝日志首行、可选 PR 评论）。改这行前先读 ADR-0008。
$NonGateBanner = '非门（不替代 ①b：指纹等关键交互仍由人走）——仅真机只读取证，机械上无法替代指纹按压'

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$VerifyRoot = if ([System.IO.Path]::IsPathRooted($OutDir)) { $OutDir } else { Join-Path $ProjectRoot $OutDir }
$LogPath = Join-Path $VerifyRoot 'device-capture.log'
$AdbExe = $AdbPath
$ArchiveModuleName = if ($ArchiveModule) { $ArchiveModule } else { $Module }

# 切页闸门：默认关闭；-NoStartApp / -SkipAppStart 等价且优先级更高（fail-safe）
$ForbidStart = ($NoStartApp -or $SkipAppStart)
$CanStart = ($AllowAppStart -and -not $ForbidStart)

# 截图入库上限（与 emulator-smoke / mp-weixin-check 同一套纪律；改这里要同步改 utils/deviceCaptureContract.test.js）
$ArchiveMaxWidth = 720
$ArchiveMaxBytes = 150 * 1024
$ArchiveMaxCount = 10
$ArchiveMaxTotalBytes = 1536 * 1024

# script 作用域状态：Set-StrictMode 下必须先声明再赋值，否则读未赋值变量会抛错
$script:Serial = ''
$script:DeviceModel = ''
$script:ChildProcesses = @()
$script:DrawingReady = $false
$script:FinalStatus = ''
$script:Foreground = [pscustomobject]@{ Component = ''; Raw = ''; Source = '' }
$script:TargetPackage = ''
$script:ShotRecords = @()
$script:SkippedPages = @()
$script:WindowStart = 0.0
$script:WindowKnown = $false
$script:FinalTotals = [pscustomobject]@{
    Lines = 0; Total = 0; WindowLines = 0; WindowStart = 0.0; WindowKnown = $false
    FatalCount = 0; AnrAllCount = 0; AnrPkgCount = 0
    RuntimeCrashes = 0; ProcessDeaths = 0; FatalSamples = @(); AnrSamples = @()
}
$script:PageResultsAll = @()

function Write-Log {
    param([string]$Message)
    $line = "[device-capture] $Message"
    Write-Host $line
    if (Test-Path -LiteralPath $VerifyRoot -PathType Container) {
        Add-Content -LiteralPath $LogPath -Value $line -Encoding UTF8
    }
}

function Write-Result {
    param([string]$Status, [string]$Detail)
    Write-Log "DEVICE_CAPTURE_RESULT=$Status"
    if ($Detail) { Write-Log "  $Detail" }
}

function Fail-Environment {
    param([string]$Detail)
    Write-Result 'UNUSABLE' $Detail
    exit 2
}

# ================= 设备探测（只读：adb devices） =================
function Get-DeviceList {
    $raw = (& $AdbExe devices -l 2>&1 | Out-String)
    $list = @()
    $known = @('device', 'offline', 'unauthorized', 'bootloader', 'recovery', 'sideload')
    foreach ($line in ($raw -split "`r?`n")) {
        $t = $line.Trim()
        if (-not $t) { continue }
        if ($t -match '^List of devices attached') { continue }
        if ($t.StartsWith('*')) { continue }
        $parts = $t -split '\s+'
        if (@($parts).Count -lt 2) { continue }
        $state = $parts[1]
        if ($known -notcontains $state) { continue }
        $mdl = ''
        $m = [regex]::Match($t, 'model:(\S+)')
        if ($m.Success) { $mdl = $m.Groups[1].Value }
        $list += [pscustomobject]@{ Serial = $parts[0]; State = $state; Model = $mdl; Raw = $t }
    }
    return $list
}

# 多设备 ⇒ 必须显式 -Device，否则 exit 2（绝不自动猜：猜错会去取证别人的设备/会话）
function Resolve-DeviceSerial {
    param([string]$Requested)
    $devices = @(Get-DeviceList)
    if ($devices.Count -eq 0) {
        Fail-Environment 'adb devices 未列出任何设备（不重启 adb server，故不做任何自动重连；请先恢复无线连接）'
    }
    if ($Requested) {
        $hit = @($devices | Where-Object { $_.Serial -eq $Requested })
        if ($hit.Count -eq 0) {
            Fail-Environment ("-Device $Requested 不在 adb devices 列表里。现有：" + (($devices | ForEach-Object { "$($_.Serial)[$($_.State)]" }) -join ', '))
        }
        if ($hit[0].State -ne 'device') {
            Fail-Environment "-Device $Requested 的状态是 $($hit[0].State)（不是 device）"
        }
        $script:DeviceModel = $hit[0].Model
        return $Requested
    }
    $online = @($devices | Where-Object { $_.State -eq 'device' })
    if ($online.Count -gt 1) {
        Fail-Environment ("adb devices 有 $($online.Count) 个 online 设备，无法自动判定 ⇒ 请显式 -Device <serial>。候选：" + (($online | ForEach-Object { "$($_.Serial)(model=$($_.Model))" }) -join ', ') + '。注意同一台真机可能同时以 <ip:port> 与 _adb-tls-connect._tcp 两条 transport 出现。')
    }
    if ($online.Count -eq 0) {
        Fail-Environment ('没有 state=device 的设备。现有：' + (($devices | ForEach-Object { "$($_.Serial)[$($_.State)]" }) -join ', '))
    }
    $script:DeviceModel = $online[0].Model
    return $online[0].Serial
}

# ================= 前台 Activity（只读：dumpsys activity activities） =================
# 口径要求记录 mResumedActivity；实测 Android 14+ 的 ROM 只报 topResumedActivity（本机 = annibale / 2510DRK44C），
# 故两者都认、优先 mResumedActivity。同时把命中的**原始行**整行留证（等于 dumpsys 管道 grep mResumedActivity 的记录）。
function Get-ForegroundInfo {
    $out = (& $AdbExe -s $script:Serial shell dumpsys activity activities 2>&1 | Out-String)
    foreach ($key in @('mResumedActivity', 'topResumedActivity')) {
        $rx = $key + '[^\n]*?([A-Za-z0-9_.]+/[A-Za-z0-9_.$]+)'
        $m = [regex]::Match($out, $rx)
        if ($m.Success) {
            $rawLine = (($out -split "`r?`n") | Where-Object { $_ -match $key } | Select-Object -First 1)
            return [pscustomobject]@{ Component = $m.Groups[1].Value; Raw = ([string]$rawLine).Trim(); Source = $key }
        }
    }
    return [pscustomobject]@{ Component = ''; Raw = ''; Source = '' }
}

function Get-PackageFromComponent {
    param([string]$Component)
    if (-not $Component) { return '' }
    $parts = $Component -split '/'
    return $parts[0]
}# ================= logcat 时间窗（只读：logcat -d；不清缓冲） =================
function Get-LogcatBaseline {
    # 窗口起点 = 设备自己最新日志行的 epoch 时间戳（不依赖宿主时钟）
    $out = (& $AdbExe -s $script:Serial shell logcat -d -v epoch -t 1 2>&1 | Out-String)
    $ms = [regex]::Matches($out, '(?m)^\s*(\d{9,}\.\d+)\s')
    if ($ms.Count -eq 0) { return 0.0 }
    return [double]$ms[$ms.Count - 1].Groups[1].Value
}

function Get-LogcatWindow {
    param([double]$SinceEpoch, [int]$MaxSeconds, [string]$Pkg)
    $dump = (& $AdbExe -s $script:Serial shell logcat -d -v epoch 2>&1 | Out-String)
    $lines = @($dump -split "`r?`n")
    $stamped = @()
    $latest = 0.0
    foreach ($l in $lines) {
        $m = [regex]::Match($l, '^\s*(\d{9,}\.\d+)\s')
        if (-not $m.Success) { continue }
        $ts = [double]$m.Groups[1].Value
        $stamped += [pscustomobject]@{ Ts = $ts; Line = $l }
        if ($ts -gt $latest) { $latest = $ts }
    }
    $start = $SinceEpoch
    if ($MaxSeconds -gt 0) {
        $floor = $latest - [double]$MaxSeconds
        if ($floor -gt $start) { $start = $floor }
    }
    $window = @()
    if ($start -gt 0) {
        $window = @($stamped | Where-Object { $_.Ts -ge $start } | ForEach-Object { $_.Line })
    }
    $fatal = @($window | Where-Object { $_ -match 'FATAL EXCEPTION' })
    # ANR 只在**打给前台那个包**时才算失败：ANR in <pkg> 里的 <pkg> 是卡住的进程；
    # 系统进程/输入法被记 ANR 是常见噪声，算到本应用头上会假阳性。
    $anrAll = @($window | Where-Object { $_ -match 'ANR in ' })
    $anrPkg = @()
    if ($Pkg) {
        $anrPkg = @($anrAll | Where-Object { $_ -match ('ANR in ' + [regex]::Escape($Pkg) + '($|[\s(])') })
    }
    $runtimeCrash = @($window | Where-Object { $_ -match 'E AndroidRuntime' })
    $died = @($window | Where-Object { $_ -match 'has died' })
    return [pscustomobject]@{
        Lines          = $lines.Count
        Total          = $stamped.Count
        WindowLines    = $window.Count
        WindowStart    = $start
        WindowKnown    = ($start -gt 0)
        FatalCount     = $fatal.Count
        AnrAllCount    = $anrAll.Count
        AnrPkgCount    = $anrPkg.Count
        RuntimeCrashes = $runtimeCrash.Count
        ProcessDeaths  = $died.Count
        FatalSamples   = @($fatal | Select-Object -First 5)
        AnrSamples     = @($anrPkg | Select-Object -First 5)
    }
}

# ================= 截图（只读：exec-out screencap -p） =================
function Convert-PageToFileName {
    param([string]$Page, [string]$Suffix)
    return (($Page -replace '[^A-Za-z0-9._-]', '-') + '-' + $Suffix + '.png')
}

function Export-Screenshot {
    param([string]$Name)
    $path = Join-Path $VerifyRoot $Name
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
    # PNG 是二进制：必须走 cmd 重定向，PowerShell 的 > 会破坏字节流
    $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $AdbExe, $script:Serial, $path
    & cmd.exe /c $cmd 2>&1 | Out-Null
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        $size = (Get-Item -LiteralPath $path).Length
        Write-Log "截图：$Name（$([math]::Round($size / 1KB, 1)) KB）"
        return [pscustomobject]@{ Page = $Name; Path = $path; Name = $Name; Bytes = $size }
    }
    Write-Log "截图失败：$Name（文件未生成）"
    return [pscustomobject]@{ Page = $Name; Path = $path; Name = $Name; Bytes = 0 }
}

# ================= 切页（写操作：**仅** -AllowAppStart 显式开启时可达） =================
function Resolve-LauncherComponent {
    $pkg = $script:TargetPackage
    $out = (& $AdbExe -s $script:Serial shell cmd package resolve-activity --brief -a android.intent.action.MAIN -c android.intent.category.LAUNCHER $pkg 2>&1 | Out-String)
    $m = [regex]::Match($out, '([A-Za-z0-9_.]+/[A-Za-z0-9_.$]+)')
    if ($m.Success) { return $m.Groups[1].Value }
    if ($script:Foreground.Component) { return $script:Foreground.Component }
    return ("$pkg/$pkg.PandoraEntry")
}

function Start-AppPage {
    param([string]$Page)
    $launcher = Resolve-LauncherComponent
    $intentArgs = @('shell', 'am', 'start', '-n', $launcher, '-a', 'android.intent.action.VIEW',
        '-d', "uniapp://$Page", '--ez', 'dcloud_open_url', 'true', '--es', 'dcloud_page', $Page)
    $out = (& $AdbExe -s $script:Serial @intentArgs 2>&1 | Out-String)
    Write-Log "am start：$((($out -split "`r?`n") | Select-Object -First 2) -join ' / ')"
}

# ================= 截图入库（可选；任何失败只警告、不改结论） =================
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
        @{ Quality = 75; MaxWidth = 560; Label = 'q75/560' },
        @{ Quality = 60; MaxWidth = 420; Label = 'q60/420' }
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
                $pngTemp = Join-Path $env:TEMP ("device-shot-" + [guid]::NewGuid().ToString('N') + '.png')
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
    if (-not (Load-Drawing)) { return @() }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Write-Log '入库：无 git，跳过'; return @() }

    # 硬前提：当前分支是该 PR 的 head 且工作树无其他改动；否则只警告，不提交
    $status = Invoke-Git @('status', '--porcelain')
    if ($status) {
        Write-Log "入库：工作树有未提交改动，跳过入库（不影响取证结论）。待提交：$((($status -split "`n") | Select-Object -First 5) -join '; ')"
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
        Write-Log "入库：当前分支 $branch 不等于 PR head $prHead，跳过入库（有意为之：绝不把截图提交到别的分支）"
        return @()
    }

    $encoder = Get-WebpEncoder
    if ($encoder) { Write-Log "入库编码器：$($encoder.Name)（WebP）" }
    else { Write-Log '入库编码器：无 cwebp/ffmpeg/magick ⇒ 退 JPEG q75（.NET System.Drawing）' }

    $targetDir = Join-Path $ProjectRoot (Join-Path 'docs\verification' (Join-Path $ArchiveModuleName "$PrNumber"))
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

    $skippedPages = @()
    if ($usable.Count -gt $ArchiveMaxCount) {
        $skippedPages = @($usable[$ArchiveMaxCount..($usable.Count - 1)] | ForEach-Object { $_.Page })
        $usable = @($usable[0..($ArchiveMaxCount - 1)])
    }

    $entries = @()
    foreach ($shot in $usable) {
        $base = 'device-' + ($shot.Page -replace '[^A-Za-z0-9._-]', '-')
        $compressed = Compress-Screenshot -SourcePath $shot.Path -OutDir $targetDir -BaseName $base -Encoder $encoder
        if (-not $compressed) {
            Write-Log "  入库失败（超限或编码失败），跳过：$($shot.Page)"
            continue
        }
        $entries += $compressed
        Write-Log ("  入库：" + $compressed.Name + "  $($compressed.Width)x$($compressed.Height)  $([math]::Round($compressed.Bytes / 1KB, 1)) KB  " + $compressed.Format + "  [" + $compressed.Attempt + "]")
    }

    while ((@($entries | Measure-Object -Property Bytes -Sum).Sum) -gt $ArchiveMaxTotalBytes -and $entries.Count -gt 1) {
        $dropped = $entries[$entries.Count - 1]
        Remove-Item -LiteralPath $dropped.Path -Force -ErrorAction SilentlyContinue
        $entries = @($entries[0..($entries.Count - 2)])
        $skippedPages += ($dropped.Name -replace '^device-', '')
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
        $msg = "chore(verify): 归档真机只读取证截图（非门，不替代 ①b 人工关键交互）PR #$PrNumber"
        Invoke-Git (@('commit', '-m', $msg)) | Out-Null
        Invoke-Git @('push', 'origin', $branch) | Out-Null
        Write-Log "入库：已提交并推送 $(@($rel).Count) 张到 $branch"
    } catch {
        Write-Log "入库提交/推送失败（不影响结论）：$($_.Exception.Message)"
    }
    if (@($skippedPages).Count -gt 0) {
        Write-Log "入库：因上限省掉 $(@($skippedPages).Count) 张：$($skippedPages -join ', ')"
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
        $body = "<!-- prefilter:device-capture -->`n" + $body
        $tmp = Join-Path $env:TEMP ('device-capture-comment-' + [guid]::NewGuid().ToString('N') + '.md')
        [System.IO.File]::WriteAllText($tmp, $body, [System.Text.UTF8Encoding]::new($false))
        $out = (gh pr comment $PrNumber --body-file $tmp 2>&1 | Out-String)
        Write-Log "PR 评论结果：$($out.Trim())"
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    } catch {
        Write-Log "贴 PR 评论失败（不影响结论）：$($_.Exception.Message)"
    }
}

# 兜底钩子：本脚本全程只用**同步**调用（& $AdbExe …），正常不会残留子进程；
# 这里保留一个无条件收尾点，防止日后有人改成 Start-Process 后忘记收尾（契约测试锁的就是这一点）。
function Stop-LeftoverChildren {
    if (@($script:ChildProcesses).Count -eq 0) { return }
    foreach ($p in @($script:ChildProcesses)) {
        try {
            if ($p -and -not $p.HasExited) {
                $p.Kill()
                Write-Log "收尾：已停止本脚本启动的子进程 pid=$($p.Id)"
            }
        } catch { Write-Log "收尾：停止子进程失败（忽略）：$($_.Exception.Message)" }
    }
    $script:ChildProcesses = @()
}# ================= main =================
New-Item -ItemType Directory -Force -Path $VerifyRoot | Out-Null
# 日志首行＝非门标注（三处之二；另两处＝脚本头部、可选 PR 评论）
Set-Content -LiteralPath $LogPath -Value ("[device-capture] " + $NonGateBanner) -Encoding UTF8
Write-Log ('运行时间：' + (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'))
Write-Log "adb=$AdbExe  Device=$(if ($Device) { $Device } else { '(自动取唯一在线设备)' })  Pages=$Pages  切页=$CanStart"
if ($ForbidStart) { Write-Log '-NoStartApp/-SkipAppStart 已生效：本次不发任何 am start（优先级高于 -AllowAppStart，fail-safe）' }

$exitCodes = @{ Pass = 0; Assert = 1; Unusable = 2 }
$pageResults = @()
$failures = @()
$notes = @()

try {
    if (-not (Test-Path -LiteralPath $AdbExe -PathType Leaf)) {
        Fail-Environment "缺 adb：$AdbExe（可用 -AdbPath 指定；默认应为 D:\android-sdk\platform-tools\adb.exe）"
    }
    $script:Serial = Resolve-DeviceSerial -Requested $Device
    Write-Log "设备：serial=$($script:Serial) model=$($script:DeviceModel)"

    # 窗口起点必须先于一切取证动作取（否则本次动作产生的日志会落在窗口外）
    $script:WindowStart = Get-LogcatBaseline
    $script:WindowKnown = ($script:WindowStart -gt 0)
    if ($script:WindowKnown) {
        Write-Log "logcat 窗口起点（设备时钟）= $($script:WindowStart)（-LogcatSeconds=$LogcatSeconds）"
    } else {
        Write-Log 'logcat 窗口起点取不到（缓冲为空？）⇒ 本次不做崩溃判据（logcat=SKIP），不拿历史日志冒充本次'
    }

    $script:Foreground = Get-ForegroundInfo
    Write-Log "前台 Activity：$($script:Foreground.Component)（来源 $($script:Foreground.Source)）"
    if ($script:Foreground.Raw) { Write-Log "  mResumedActivity/topResumedActivity 原始行：$($script:Foreground.Raw)" }
    $script:TargetPackage = if ($Package) { $Package } else { Get-PackageFromComponent -Component $script:Foreground.Component }
    Write-Log "目标包名：$($script:TargetPackage)$(if (-not $Package) { '（取自当前前台，即维护者正在调试的应用）' })"

    $pageList = @($Pages -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if (@($pageList).Count -eq 0) { $pageList = @('(未指定页面)') }

    if (-not $CanStart) {
        # ---------------- 只读模式：不切页（默认路径） ----------------
        Write-Log '切页动作默认关闭：会打断维护者的调试会话（要切页请显式加 -AllowAppStart；-NoStartApp/-SkipAppStart 则永久禁止）'
        $shotName = 'current-screen.png'
        if (@($pageList).Count -eq 1) { $shotName = (Convert-PageToFileName -Page $pageList[0] -Suffix 'current') }
        Write-Log "只读取证：不切页，仅对**当前前台**采 1 张图 -> $shotName"
        $shot = Export-Screenshot -Name $shotName
        $script:ShotRecords = @($shot)
        foreach ($page in $pageList) {
            Write-Log "  SKIP：$page —— 未切页：切页默认关闭"
            $pageResults += [pscustomobject]@{ Page = $page; Status = 'SKIP'; Detail = '未切页：切页默认关闭'; Shot = $shot.Name }
        }
        if ($shot.Bytes -le 0) { $failures += "截图未落盘或为 0 字节（$shotName）" }
    } else {
        # ---------------- 显式切页模式（只有 -AllowAppStart 才可达） ----------------
        if (-not $script:TargetPackage) {
            Fail-Environment '切页模式需要目标包名：既没给 -Package，也取不到当前前台包名'
        }
        foreach ($page in $pageList) {
            Write-Log "── 切页：$page（-AllowAppStart 已显式开启）"
            Start-AppPage -Page $page
            Start-Sleep -Seconds $PageSettleSeconds
            $fg = Get-ForegroundInfo
            $script:Foreground = $fg
            $shot = Export-Screenshot -Name (Convert-PageToFileName -Page $page -Suffix 'after')
            $script:ShotRecords += $shot
            $pageFail = @()
            if (-not $fg.Component) { $pageFail += '取不到前台 Activity' }
            elseif ($fg.Component -notmatch [regex]::Escape($script:TargetPackage)) { $pageFail += "前台不是目标包（$($fg.Component)）" }
            if ($shot.Bytes -le 0) { $pageFail += '截图未落盘或为 0 字节' }
            $status = 'PASS'
            if (@($pageFail).Count -gt 0) {
                $status = 'FAIL'
                $failures += "$page -> $($pageFail -join '；')"
                $pageFail | ForEach-Object { Write-Log "  FAIL $_" }
            }
            Write-Log "  前台=$($fg.Component) 截图=$($shot.Name) 判定=$status"
            $pageResults += [pscustomobject]@{ Page = $page; Status = $status; Detail = ($pageFail -join '；'); Shot = $shot.Name }
        }
    }

    # ---------------- logcat 窗口断言 ----------------
    $final = Get-LogcatWindow -SinceEpoch $script:WindowStart -MaxSeconds $LogcatSeconds -Pkg $script:TargetPackage
    $script:FinalTotals = $final
    if (-not $final.WindowKnown) {
        $notes += 'logcat=SKIP（窗口起点未知，未做崩溃判据）'
    } else {
        Write-Log "logcat 窗口：本次窗口行数=$($final.WindowLines)（缓冲总行数=$($final.Total)）"
        if ($final.FatalCount -gt 0) {
            $failures += "窗口内 FATAL EXCEPTION x $($final.FatalCount)"
            $final.FatalSamples | ForEach-Object { Write-Log "  FAIL $_" }
        }
        if ($final.AnrPkgCount -gt 0) {
            $failures += "窗口内 ANR in $($script:TargetPackage) x $($final.AnrPkgCount)"
            $final.AnrSamples | ForEach-Object { Write-Log "  FAIL $_" }
        }
    }

    # 前台 Activity 可取得（只读模式下的核心断言）
    if (-not $script:Foreground.Component) { $failures += '取不到前台 Activity（dumpsys 无 mResumedActivity/topResumedActivity）' }

    $summary = @(
        '',
        '── 真机只读取证汇总（非门，不替代 ①b 人工关键交互）──',
        "设备：serial=$($script:Serial) model=$($script:DeviceModel)",
        "前台 Activity：$($script:Foreground.Component)（来源 $($script:Foreground.Source)）",
        "目标包名：$($script:TargetPackage)",
        "切页：$(if ($CanStart) { '已显式开启（-AllowAppStart）' } else { '关闭（只读；未切页）' })",
        "logcat：窗口起点=$($final.WindowStart) 窗口行数=$($final.WindowLines)  FATAL EXCEPTION=$($final.FatalCount)  ANR in 包=$($final.AnrPkgCount)  ANR 合计=$($final.AnrAllCount)  E AndroidRuntime=$($final.RuntimeCrashes)  进程死亡=$($final.ProcessDeaths)",
        "页面判定：PASS=$(@($pageResults | Where-Object { $_.Status -eq 'PASS' }).Count)  FAIL=$(@($pageResults | Where-Object { $_.Status -eq 'FAIL' }).Count)  SKIP=$(@($pageResults | Where-Object { $_.Status -eq 'SKIP' }).Count)",
        "截图：$(($script:ShotRecords | ForEach-Object { "$($_.Name)=$($_.Bytes)B" }) -join ', ')",
        "目录=$VerifyRoot  日志=$LogPath"
    )
    $summary | ForEach-Object { Write-Log $_ }
    if (@($notes).Count -gt 0) { $notes | ForEach-Object { Write-Log "提示：$_" } }
    if (@($failures).Count -gt 0) { $failures | ForEach-Object { Write-Log "断言失败：$_" } }

    $script:PageResultsAll = $pageResults
    $statusWord = 'PASS'
    if (@($failures).Count -gt 0) { $statusWord = 'FAIL' }
    Write-Result $statusWord (("页面 " + (($pageResults | ForEach-Object { "$($_.Page)=$($_.Status)" }) -join ', ')) + "  前台=$($script:Foreground.Component)  logcat窗口行数=$($final.WindowLines)  FATAL=$($final.FatalCount)  ANRin包=$($final.AnrPkgCount)")
    $script:FinalStatus = $statusWord
} catch {
    Write-Log "脚本异常：$($_.Exception.Message)"
    Write-Result 'UNUSABLE' $_.Exception.Message
    $script:FinalStatus = 'UNUSABLE'
} finally {
    # 收尾**无条件**执行：断言失败、超时、异常都走这里；且**不对设备做任何写操作**，故无需回滚
    Stop-LeftoverChildren
    Write-Log '收尾完成：全程只读，未对设备做任何写操作'
}

if ($PostToPr -gt 0) {
    $archived = @()
    if ($script:FinalStatus -eq 'PASS' -or $script:FinalStatus -eq 'FAIL') {
        $archived = Archive-Screenshots -Shots $script:ShotRecords -PrNumber $PostToPr
    }
    $shotLines = @()
    if (@($archived).Count -gt 0) {
        $shotLines += '- 截图（已入库，仓库内相对路径）：'
        foreach ($a in $archived) {
            $r = $a.Path.Substring($ProjectRoot.Length).TrimStart('\', '/') -replace '\\', '/'
            $shotLines += "  - ``$r`` — $($a.Width)x$($a.Height)，$([math]::Round($a.Bytes / 1KB, 1)) KB，$($a.Format)"
        }
    } else {
        $shotLines += "- 截图：未入库（本地 $VerifyRoot，原因见 $LogPath）"
    }
    if (@($script:SkippedPages).Count -gt 0) {
        $shotLines += "- 因上限（<=10 张 / <=150 KB / 合计 <=1.5 MB）省掉的页：$($script:SkippedPages -join ', ')"
    }
    $totals = $script:FinalTotals
    $bodyLines = @(
        "**$NonGateBanner**",
        '',
        '### ①a 真机只读取证（非门 · prefilter · agent 执行）',
        '',
        '本结果**不是**验收门证据，**不替代 ①b**：指纹、权限弹窗、真机上传路径这三类由**人**走。',
        '本项只替代 ① 的「逐页截图 + 抓日志」这一步。',
        '',
        "- 设备：serial=$($script:Serial) model=$($script:DeviceModel)",
        "- 前台 Activity：$($script:Foreground.Component)（来源 $($script:Foreground.Source)）",
        "- 切页：$(if ($CanStart) { '已显式开启（-AllowAppStart）' } else { '关闭（只读；未切页，-Pages 各项记 SKIP）' })",
        "- logcat：窗口行数=$($totals.WindowLines) / FATAL EXCEPTION=$($totals.FatalCount) / ANR in 包=$($totals.AnrPkgCount)（详见 $LogPath）",
        "- 页面判定：$(($script:PageResultsAll | ForEach-Object { "$($_.Page)=$($_.Status)" }) -join '，')",
        "- 只读保证：全程只用 adb devices / exec-out screencap / shell dumpsys / shell logcat -d，未做任何写操作、未清 logcat、未重启 adb server",
        "- 复现：pwsh -NoProfile -File scripts/device-capture.ps1 -Device $($script:Serial) -Pages `"$Pages`""
    )
    $bodyLines += $shotLines
    $bodyLines += @('', '抓不到：生物识别指纹按压、运行时权限弹窗行为、厂商 ROM 差异、真机上传路径（content://）。')
    $body = ($bodyLines -join "`n")
    Publish-PrefilterComment -PrNumber $PostToPr -Body $body
}

if ($script:FinalStatus -eq 'FAIL') { exit $exitCodes.Assert }
if ($script:FinalStatus -eq 'UNUSABLE') { exit $exitCodes.Unusable }
exit $exitCodes.Pass