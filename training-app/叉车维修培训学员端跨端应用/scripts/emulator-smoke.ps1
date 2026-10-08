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
    ⚠️ **每张截图调用都有单次超时**（#1562，2026-10-07）：截图复用 `lib/auto-screenshot.ps1` 的有界执行器
    （`-AdbCallTimeoutSeconds`，默认 15 秒）。到点不返回 ⇒ 终止整棵进程树、该页记 FAIL 并**继续**跑后面的页，
    日志点名「哪一次调用 / 多大预算 / 有没有留下残帧」；截图先落 `.part`、「调用返回 + 帧非空」才改名归位
    —— 被杀调用的残帧可能删不掉（#1560 真链路现测），而本链按「存在且非空」判出图 ⇒ 归位靠命名，不靠删除成功。
    ⚠️ **有界覆盖到哪一行（2026-10-08，#1568 AC 第 1 + 2 条）**：截图那条走执行核（#1562）；`Get-AdbOutput`
    这一个点接上同一件唯一执行核（AC 第 1 条，它自己只有 1 行调用、却压着 **12 个调用方**）；AC 第 2 条把
    原先**没走收口点的 13 处直调**也逐个接上同一件执行核，并按调用形态分档 —— 文本档 `-AdbTextCallTimeoutSeconds`
    （默认 15 秒：`adb version` / `pm list packages` / `resolve-activity` / 两条 `dumpsys` / `mkdir` / `logcat -c`，
    现测最大 2.5 秒）、logcat 档 60 秒（现测 5.9 / 6.2 秒）、开机等待档 120 秒（现测 `wait-for-device` 29.2 / 32.0 秒）、
    install 档 180 秒（现测 44.6 MB 基座 8.5 / 12.3 秒）、push 档 120 秒（现测 69.7 / 111.1 ms 每 MB）、
    收尾档 60 秒（现测 `emu kill` 0.4 秒 / `wait-for-disconnect` 2.3 秒）。数与出处逐条见
    `docs/verification/tooling/1568/emulator-tier-readings.txt` 与 `-run2.txt`，档位理由写在 `param()`。
    ⇒ **本脚本代码面的 `& $AdbExe` 直调现为 0**（复算尺现测：本文件 13 → 0，全仓 24 → 11），
    长等没有被塞进 15 秒：本票要收的是「没有上限的等」，不是「等得久」。
    ⚠️ 一处既有的语义补齐：`logcat -d` 挂死时旧写法的下场是「空日志」，而空日志会被下面的计数读成
    「FATAL=0 ⇒ 无崩溃」——那是把挂死包装成通过。现在 `Get-LogcatSummary` 带出 `TimedOut`，该页据此**记失败**，
    汇总另落一行 `LOGCAT_INCONCLUSIVE`（形状与 #1562 给截图定的 `shotTimedOut ⇒ 该页 FAIL` 同律）。
    数法要说清，否则两个量会被读成一个：尺 = 剥注释（块注释与整行 `#` 注释）后按**所在函数**归属数直调，
    入仓件 `docs/verification/tooling/1568/adb-bounded-count.mjs`；本文件从 #1562 后的 14 行 → 13 行 → **0 行**
    （中间那一格是 AC 第 1 条收掉的收口点本体）。

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
    # **一次截图调用**的上限（#1562）。默认值理由与 `device-capture.ps1` 同名参数一致：同一条有界调用在
    # #1560 的真链路产物里返回 646 / 719 / 949 / 1217 ms（`docs/verification/tooling/1560/README.md:25` 与 :48-49），
    # 15 秒给了最慢那次的十倍余量，而它把「一次不返回就整条冒烟就地停住」钉死成**最多 15 秒**。
    # ⚠️ 限定词是实的：截图这一档只管 `Export-Screenshot` 那一条 `exec-out screencap`。本脚本其余 adb 调用
    #   分属下面几档（#1568 AC 第 1 条收了 `Get-AdbOutput` 那一个点 = 12 个调用方；AC 第 2 条把原先 13 处
    #   没走收口点的直调也接上同一件执行核，并按形态分档 —— 代码面现测 `& $AdbExe` 为 0）。
    [int]$AdbCallTimeoutSeconds = 15,
    # **一次取文本的 adb 调用**的上限（#1568，2026-10-08）：收的是 `Get-AdbOutput` 那一个点 —— 它自己只有
    # 1 行调用，却压着 **12 个调用方**（`sys.boot_completed` 轮询、三条 `settings put`、`input keyevent`、
    # 三条 `getprop`、`pidof`、`am start` / `am force-stop`），改一处就让它们一并有界。
    # 默认值理由指得到入库产物：#1560 真链路里同一条有界调用返回 646 / 719 / 949 / 1217 ms
    # （`docs/verification/tooling/1560/README.md:25` 与 :48-49），而那还是一张约 730 KB 的 PNG；本档要取的是
    # `getprop` / `settings` / `am start` 这类小文本，同机现测 `adb version` 178 ms、`adb devices` 106 ms
    # （`docs/verification/tooling/1568/latency-readings.txt`）⇒ 15 秒给了三个数量级的余量。
    # ⚠️ 这一档**只给读文本与小管理调用**：`install` / `push` / `wait-for-device` / `logcat -d` / 收尾是有意的长等，
    #   各走下面那几档 —— 把它们塞进 15 秒就是**把正常当挂死**（现测：wait-for-device 29.2/32.0 秒、
    #   install 8.5/12.3 秒、logcat -d 5.9/6.2 秒）；本票要收的是「没有上限的等」，不是「等得久」。
    [int]$AdbTextCallTimeoutSeconds = 15,
    # ── 长等三档 + logcat 一档 + 收尾一档（#1568 AC 第 2 条，2026-10-08）──────────────────────────
    # 每个默认值都指得到入库现测产物 `docs/verification/tooling/1568/emulator-tier-readings.txt`（两次冷起
    # Pixel_4a_API_30 的真读数，另有 `-run2.txt` 复测）。**同一形状的两列都实测过**，取大值再给余量：
    #   wait-for-device 29,160 / 32,049 ms ⇒ 120 秒（≈3.7 倍；且**必须小于** -BootTimeoutSeconds 300，
    #     否则外层起机上限反而管不住它）；
    #   install -r -t（44.6 MB 基座，流式安装）8,528 / 12,268 ms ⇒ 180 秒（≈15 倍；换更大 APK 也够）；
    #   push 69.7 / 111.1 ms 每 MB ⇒ 120 秒（按 111 ms/MB 外推到 200 MB 是 22 秒，给 ≈5 倍）；
    #   logcat -d 5,896 / 6,186 ms ⇒ 60 秒（≈10 倍；真会话的缓冲区比刚起机时更满，这一格不能贴读数定档）；
    #   emu kill 357 / 413 ms + wait-for-disconnect 2,069 / 2,289 ms ⇒ 60 秒（≈26 倍；收尾一挂死，
    #     下面的 Kill() 兜底就永远轮不到 ⇒ 这一档的意义是「让兜底可达」）。
    # 现测同时照出：本机 `install` 一直是 `INSTALL_FAILED_NO_MATCHING_ABIS`（x86 AVD 装不上基座的原生库），
    # 这解释了 #1562 三跑冒烟为何都带 -SkipInstallBaseApk；install 那一档量的仍是**真实流式传输**的耗时。
    [int]$AdbInstallTimeoutSeconds = 180,
    [int]$AdbPushTimeoutSeconds = 120,
    [int]$AdbBootWaitTimeoutSeconds = 120,
    [int]$AdbLogcatTimeoutSeconds = 60,
    [int]$AdbTeardownTimeoutSeconds = 60,
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
# 「一次 adb 调用怎么才有界」的**唯一真源**在 `scripts/lib/auto-screenshot.ps1`（#1560 落地的有界执行器，
# #1562 把本脚本这张图也接到它上面）。dot-source 复用、**不**在此再写一份「等待 + 杀树 + 残帧作废」：
# 复制而不是复用是 ADR-0008「adb 解析的唯一真源」记的那类分叉。该 lib 只有函数定义、加载无副作用，
# 且与本脚本无同名函数（若将来出现同名，后加载者胜 —— 本文件的定义在 dot-source 之后，以本文件为准）。
. (Join-Path $PSScriptRoot 'lib\auto-screenshot.ps1')

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
# 超时过的截图调用点名清单（#1562）：汇总与机检行用它
$script:ShotTimeouts = @()
# 超时过的**取文本**调用点名清单与调用计数（#1568）：同一件事的另一条通道，同一份机检行口径
$script:AdbTextTimeouts = @()
$script:AdbTextCalls = 0
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

# ── 有界 adb 调用的**唯一**委托层（#1568 AC 第 1 + 2 条，2026-10-08）──────────────────────────
# 「一次 adb 调用怎么才有界」（等待、到点杀整棵进程树、残文件不回收）不在本脚本重写 —— 那份只在
# `lib/auto-screenshot.ps1` 存在一件。本层只补三件这一族需要、而执行核不该管的事：
#   ① 档位：`-BudgetSeconds` 从 `param()` 的六个档取，**调用形态决定档**（AC 第 3 条）；
#   ② 点名：挂死必须落一行超时结论（哪一次调用、多大预算、实际等了多久、属于哪一档），
#      否则读日志的人只看到「设备没答」，分不清 adb 通道挂了与答了个空。是读数、不是判据；
#   ③ serial 的取舍：`version` 这类 **server 级**命令恒不带 `-s`（带了就换判据 —— AC 第 4 条），
#      设备级调用照旧带。
# 委托链两段：`Get-AdbOutput`（回文本，超时回空串 —— 既有语义）→ `Invoke-AdbTierCall`（回状态）。
function Invoke-AdbTierCall {
    param(
        [string[]]$AdbArgs,
        [int]$BudgetSeconds = $AdbTextCallTimeoutSeconds,
        [string]$Tier = 'text',
        [switch]$NoSerial
    )
    $script:AdbTextCalls = $script:AdbTextCalls + 1
    $adbSerial = $(if ($NoSerial) { '' } else { $Serial })
    # `-MergeStdErr` 是给「失败一律回空串/回原文」兜底的：本族的原始形状是 `2>&1`，
    # adb 的**失败原文全在 stderr**（`am start` 的 Error type、`pidof` 的空答、install 的 Failure 行），
    # 不合并就等于悄悄把判据原料换了。
    # 非 Windows 走直启分支：本脚本实践中只在 Windows 跑（要 WHPX/HAXM 与 emulator.exe），但它的
    # 行为守护在 CI 的 ubuntu 上真跑（#1562 的 `B12` 教训：那一格造不出来就会静默假绿）。
    $r = Invoke-BoundedAdbText -AdbExe $AdbExe -Serial $adbSerial -AdbArguments $AdbArgs -AdbArgv $AdbArgs `
        -DirectExec:(-not $IsWindows) -MergeStdErr -TimeoutSeconds $BudgetSeconds
    if ($r.TimedOut) {
        $script:AdbTextTimeouts = @($script:AdbTextTimeouts) + @(($AdbArgs -join ' '))
        Write-Log ('ADB_TEXT_TIMEOUT call=adb ' + $(if ($NoSerial) { '' } else { "-s $Serial " }) + ($AdbArgs -join ' ') +
            " callBudgetSeconds=$BudgetSeconds seconds=$($r.Seconds) tier=$Tier —— 已终止整棵进程树，本次按「没拿到」回，调用方各按自己的分支判")
    }
    return $r
}

function Get-AdbOutput {
    param([string[]]$AdbArgs, [int]$BudgetSeconds = $AdbTextCallTimeoutSeconds, [string]$Tier = 'text', [switch]$NoSerial)
    # 既有语义不许改：失败（含挂死）一律返回空串，调用方各按自己的分支判「空」算不算失败
    #（统一走 adb -s <serial>；要区分「答了空」与「没答上」的调用方直接读 Invoke-AdbTierCall 的 TimedOut ——
    #  logcat 那一格就是这么做的，因为空日志会被读成「无崩溃」）。
    $r = Invoke-AdbTierCall -AdbArgs $AdbArgs -BudgetSeconds $BudgetSeconds -Tier $Tier -NoSerial:$NoSerial
    if ($r.TimedOut) { return '' }
    return ([string]$r.Text).Trim()
}

# wait-for-device 单独成函数（AC 第 2 条）：它是本族里最长的等（现测 29.2 / 32.0 秒），也是唯一
# 「挂在那里连起没起机都判不出来」的一格 —— 收进可直调的函数，行为守护才真跑得到它（EMT7）。
function Wait-ForDeviceBounded {
    Write-Log 'adb wait-for-device …'
    $null = Get-AdbOutput @('wait-for-device') -BudgetSeconds $AdbBootWaitTimeoutSeconds -Tier 'boot-wait'
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
    # `adb version` 是 **server 级**命令：带 `-s <serial>` 就不是同一条调用了（#1568 AC 第 4 条「不改既有判据语义」
    # 在这一格最实 —— 起了模拟器之后再问 version 会答出别的东西），所以走 -NoSerial；档位给文本档（现测 165–200 ms）。
    $script:AdbVersion = ((Get-AdbOutput @('version') -NoSerial -Tier 'server') -split "`n" | Select-Object -First 1).Trim()
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

    Wait-ForDeviceBounded

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
    $pkgs = Get-AdbOutput @('shell', 'pm', 'list', 'packages')
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
    $out = Get-AdbOutput @('shell', 'cmd', 'package', 'resolve-activity', '--brief', '-a', 'android.intent.action.MAIN', '-c', 'android.intent.category.LAUNCHER', $Package)
    $m = [regex]::Match($out, '([A-Za-z0-9_.]+/[A-Za-z0-9_.$]+)')
    if ($m.Success) { return $m.Groups[1].Value }
    return "$Package/$Package.PandoraEntry"
}function Install-BaseApk {
    if ($SkipInstallBaseApk) {
        Write-Log '跳过装基座（-SkipInstallBaseApk）'
        return
    }
    Write-Log "安装基座 APK（$($script:BaseApkSizeMb) MB）：$BaseApk"
    # 流式安装是**有意的长等**（现测 44.6 MB 基座 8.5 / 12.3 秒，含失败那一次）⇒ 走 install 档，不套文本档。
    # 判据不变：只看输出里有没有 `Success`（本仓口径：不拿退出码当判据，见契约 C2）。
    $out = Get-AdbOutput @('install', '-r', '-t', $BaseApk) -BudgetSeconds $AdbInstallTimeoutSeconds -Tier 'install'
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
    $null = Get-AdbOutput @('shell', 'mkdir', '-p', $target)
    # push 是有意的长等（现测 69.7 / 111.1 ms 每 MB）⇒ 走 push 档。目标串逐字保持 `目录/.`（adb 的「只传内容」写法）。
    $out = Get-AdbOutput @('push', "$resolved/.", $target) -BudgetSeconds $AdbPushTimeoutSeconds -Tier 'push'
    if ($out -notmatch 'pushed|skipped') {
        Write-Log "资源 push 结果可疑：$($out.Trim())"
        return $false
    }
    Write-Log '资源 push 完成'
    return $true
}

# ================= 断言辅助 =================
function Get-ForegroundActivity {
    $out = Get-AdbOutput @('shell', 'dumpsys', 'activity', 'activities')
    $m = [regex]::Match($out, 'topResumedActivity[^\n]*?([A-Za-z0-9_.]+/[A-Za-z0-9_.]+)')
    if ($m.Success) { return $m.Groups[1].Value }
    $out2 = Get-AdbOutput @('shell', 'dumpsys', 'window')
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
    # ⚠️ 这一格**不能**用「回空串」的 Get-AdbOutput：挂死时它回空文本，而下面的计数会把空文本读成
    #   「FATAL=0 ⇒ 无崩溃」——那是把挂死包装成通过，比不收口更坏。所以这里直读状态位（#1568 AC 第 4 条：
    #   「不返回」要成为可见结论，且不改既有判据语义 —— 计数照旧，只是多带一个 TimedOut 让调用方落该页失败）。
    #   预算给 logcat 档（现测刚起机时 5.9 / 6.2 秒，真会话的缓冲区更满 ⇒ 60 秒，见 param() 的理由）。
    $r = Invoke-AdbTierCall @('logcat', '-d', '-v', 'brief') -BudgetSeconds $AdbLogcatTimeoutSeconds -Tier 'logcat'
    $dump = [string]$r.Text
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
        TimedOut        = [bool]$r.TimedOut
        BudgetSeconds   = $AdbLogcatTimeoutSeconds
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
    # ⚠️ 单次 adb 调用**有界**（#1562；同族缺陷的第三个落点，前两个由 #1560 收口）。
    #    旧写法 `& cmd.exe /c "… exec-out screencap -p > file" 2>&1 | Out-Null` 没有单次超时 ⇒ 一次不返回
    #    就既不产帧也不报错，整条冒烟就地停住（后面的页也跑不到）。现复用 `lib/auto-screenshot.ps1`
    #    那份有界执行核：PNG 是二进制，仍由 OS 把 stdout 直接写进文件、父进程一个字节都不读
    #    （PowerShell 的 `>` 会破坏字节流 —— 这条既有约束由执行核保住，headless 出图判据不变）。
    #    ⚠️ 先落 `.part`、成功才归位：被杀调用的残帧**可能删不掉**（#1560 真链路现测：`Kill` 之后句柄未放、
    #    删除与它竞争），而本链的判据是「文件存在且非空」⇒ 半张图绝不能落在最终名上被当成本次出图。
    $partPath = "$path.part"
    if (Test-Path -LiteralPath $partPath) { Remove-Item -LiteralPath $partPath -Force -ErrorAction SilentlyContinue }
    $shot = Invoke-BoundedAdbShot -AdbExe $AdbExe -Serial $Serial -OutFile $partPath -TimeoutSeconds $AdbCallTimeoutSeconds
    if ($shot.TimedOut) {
        # 到点给结论并**继续**跑后面的页：该页记失败（见主循环），最终名上不写文件。
        $partLeft = if (Test-Path -LiteralPath $partPath) { (Get-Item -LiteralPath $partPath).Length } else { 0 }
        Write-Log "截图调用未返回：$name —— 单次 exec-out screencap 未在 $AdbCallTimeoutSeconds 秒内返回（$($shot.Error)）"
        Write-Log ("SHOT_CALL_TIMEOUT page=$name callBudgetSeconds=$AdbCallTimeoutSeconds partBytes=$partLeft finalWritten=False")
        $script:ShotTimeouts = @($script:ShotTimeouts) + @($Page)
        Remove-Item -LiteralPath $partPath -Force -ErrorAction SilentlyContinue
        return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = 0; TimedOut = $true; CallBudgetSeconds = $AdbCallTimeoutSeconds }
    }
    if (-not (Test-Path -LiteralPath $partPath) -or (Get-Item -LiteralPath $partPath).Length -eq 0) {
        $shotWhy = if ($shot.ErrTail) { "（adb stderr: $($shot.ErrTail)）" } else { '' }
        Write-Log "截图失败：$name（文件未生成）$shotWhy"
        Remove-Item -LiteralPath $partPath -Force -ErrorAction SilentlyContinue
        return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = 0; TimedOut = $false; CallBudgetSeconds = $AdbCallTimeoutSeconds }
    }
    # 归位：只有「调用返回了」且「拿到非空帧」的图才算出图。失败只判这一页，不许掀翻整条冒烟。
    try {
        Move-Item -LiteralPath $partPath -Destination $path -Force
    } catch {
        Write-Log "截图归位失败：$name（$($_.Exception.Message)）"
        Remove-Item -LiteralPath $partPath -Force -ErrorAction SilentlyContinue
        return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = 0; TimedOut = $false; CallBudgetSeconds = $AdbCallTimeoutSeconds }
    }
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        $size = (Get-Item -LiteralPath $path).Length
        Write-Log "截图：$name ($([math]::Round($size / 1KB, 1)) KB)"
        return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = $size; TimedOut = $false; CallBudgetSeconds = $AdbCallTimeoutSeconds }
    }
    Write-Log "截图失败：$name（文件未生成）"
    return [pscustomobject]@{ Page = $Page; Path = $path; Name = $name; Bytes = 0; TimedOut = $false; CallBudgetSeconds = $AdbCallTimeoutSeconds }
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
    $null = Get-AdbOutput @('logcat', '-c')
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
        # 截图调用超时 ⇒ **该页记失败、循环继续**（#1562）。只加这一条判据：0 字节出图的老行为不动
        # （票面「不动判据本身」；本链原先并不按截图字节数判页，改成按它判会越界）。
        if ($shot.TimedOut) { $pageFail += "截图调用未在 $AdbCallTimeoutSeconds 秒内返回（adb 通道问题，非渲染问题）" }
        # logcat 读不到 = 崩溃计数不可判 ⇒ **该页记失败**（形状与上面那条截图超时同律，#1568 AC 第 4 条）。
        # 少了这一条，「adb 挂死 ⇒ 空日志 ⇒ FATAL=0」就会被读成「这页没崩」——那是本票最险的一格。
        if ($logcat.TimedOut) { $pageFail += "logcat 未在 $AdbLogcatTimeoutSeconds 秒内返回 ⇒ 崩溃与 ANR 计数不可判，该页记失败" }

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
        Write-Log "  进程存活=$alive 前台=$foreground FATAL=$($logcat.FatalCount) ANR=$($logcat.AnrCount) 截图=$($shot.Name) shotTimedOut=$($shot.TimedOut) logcatTimedOut=$($logcat.TimedOut) 判定=$status"

        $pageResults += [pscustomobject]@{ Page = $page; Status = $status; Bytes = $shot.Bytes; Fatal = $logcat.FatalCount; Anr = $logcat.AnrCount; Alive = $alive }
    }

    $final = Get-LogcatSummary
    $summary = @(
        '',
        '── 冒烟汇总（非门，不替代 ① 真机门）──',
        "AVD=$AvdName  model=$($script:DeviceModel)  api=$($script:DeviceSdk)  abi=$($script:DeviceAbi)",
        "硬件加速：$($script:Accel.Verdict)（$($script:Accel.Brief)）",
        "冷启动耗时=$($script:ColdBootSeconds) 秒（headless=$(-not $ShowWindow)）",
        # 「不返回」必须是一种**可见的结论**（#1562）：次数 / 预算 / 哪几页都点名，
        # 否则读日志的人只看到「少了一张图」，分不清 adb 通道问题与渲染问题。是读数、不是判据。
        "SHOT_CALL_BUDGET calls=$(@($shotRecords).Count) timeouts=$(@($script:ShotTimeouts).Count) callBudgetSeconds=$AdbCallTimeoutSeconds timedOutPages=$(if (@($script:ShotTimeouts).Count -gt 0) { $script:ShotTimeouts -join ',' } else { 'none' })",
        # 取文本那一档同口径点名（#1568）：挂过哪几次、预算多大都得写出来，读日志的人才分得清
        # 「adb 通道不返回」与「设备答了个空」——前者查 adb、后者查设备，处置人完全不同。同样是读数、不是判据。
        "ADB_TEXT_CALL_BUDGET calls=$($script:AdbTextCalls) timeouts=$(@($script:AdbTextTimeouts).Count) defaultBudgetSeconds=$AdbTextCallTimeoutSeconds hungCalls=$(if (@($script:AdbTextTimeouts).Count -gt 0) { $script:AdbTextTimeouts -join ';' } else { 'none' })",
        # 六档现值打进汇总（#1568 AC 第 3 条）：事后能从日志核对「当时生效的是哪一档」。
        # 没有这一行，读到一条 `seconds=60` 的超时的人分不清是「预算给小了」还是「设备真挂了」。
        "ADB_TIER_BUDGET text=$AdbTextCallTimeoutSeconds logcat=$AdbLogcatTimeoutSeconds boot_wait=$AdbBootWaitTimeoutSeconds install=$AdbInstallTimeoutSeconds push=$AdbPushTimeoutSeconds teardown=$AdbTeardownTimeoutSeconds shot=$AdbCallTimeoutSeconds",
        "基座 APK=$($script:BaseApkPackage)  安装=$(if ($SkipInstallBaseApk) { '已跳过' } else { "已安装 ($($script:BaseApkSizeMb) MB)" })",
        "本次 logcat 总行数=$($final.Lines)  FATAL EXCEPTION=$($final.FatalCount)  ANR=$($final.AnrCount)  AndroidRuntime=$($final.RuntimeCrashes)  进程死亡=$($final.ProcessDeaths)",
        "LOGCAT_INCONCLUSIVE=$(if ($final.TimedOut) { 1 } else { 0 })",
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
        # 收尾两格也有单次预算（teardown 档；现测 emu kill 0.36/0.41 秒、wait-for-disconnect 2.1/2.3 秒）。
        # 这一档的意义是**让下面的 Kill() 兜底可达**：旧写法一次不返回就永远轮不到兜底，
        # 「绝不留下跑着的模拟器」这句承诺就地失效（#1568 AC 第 4 条：不返回要成为可见结论）。
        try { $null = Get-AdbOutput @('emu', 'kill') -BudgetSeconds $AdbTeardownTimeoutSeconds -Tier 'teardown' } catch { Write-Log "emu kill 失败：$($_.Exception.Message)" }
        try { $null = Get-AdbOutput @('wait-for-disconnect') -BudgetSeconds $AdbTeardownTimeoutSeconds -Tier 'teardown' } catch { }
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












