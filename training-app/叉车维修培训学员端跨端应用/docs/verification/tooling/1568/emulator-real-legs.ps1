<#
.SYNOPSIS
    #1568 AC 第 2 条的**真链路腿**：把 `emulator-smoke.ps1` 的分档接到真 adb + 真仿真机上跑四遍，
    读数落 `emulator-real-legs-readings.txt`（判据只取 ASCII token —— 本仓血账：pwsh 的中文到上层控制台会变乱码）。

.DESCRIPTION
    为什么必须有这四遍（票面 DoD：工具链改动的验收 = 它要达成的那个行为在真实链路上成立，不是「免四门」）：
    EMT 那九条用的是可编程假载体（要的是「真挂死」这一格 —— 真 adb 不能按需要挂死）。这一份补的是**载体那一头**：
      L1 正常冒烟（-SkipInstallBaseApk，与 #1562 的夹具同形）⇒ 六档现值打进汇总（ADB_TIER_BUDGET），
         且整趟**零** ADB_TEXT_TIMEOUT ⇒ 分档没把正常等误判成挂死（这是「不得套 15 秒」那格的反面证据）。
      L2 `-AdbLogcatTimeoutSeconds 0` ⇒ 真 logcat -d 到点：汇总必须落 LOGCAT_INCONCLUSIVE=1，
         而不是「FATAL=0 ⇒ 无崩溃」。
      L3 `-AdbInstallTimeoutSeconds 0` ⇒ 真 install 到点：结论必须点名 tier=install，且整条链以 UNUSABLE 收口
         （不永挂）。0 秒档是故意的：它把「预算是参数」这一格打在真载体上。
      L4 中文 APK 路径走新链路（cmd 包装 + `-MergeStdErr`）⇒ 必须仍拿到 adb 的失败原文
         （本机 AVD 装基座是 INSTALL_FAILED_NO_MATCHING_ABIS —— 那句原文在 stderr，正好当合并判据的原料）。

    ⚠️ 已知取证缺口（写进 PR 正文，不藏）：逐页那条 `if ($logcat.TimedOut) { $pageFail += … }` 需要
    `-ResourcesDir`（HBuilderX 编译产物 `unpackage/resources/app-android`），本机与宿主树都 ABSENT（实测见
    `docs/verification/tooling/1568/README.md`）⇒ 该分支由契约锚点 C11④ + 行为腿 EMT5 锁，未在此处真跑。

.PARAMETER Smoke
    被验的那份脚本路径（默认取仓内 `scripts/emulator-smoke.ps1`）。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/emulator-real-legs.ps1 `
        -OutFile docs/verification/tooling/1568/emulator-real-legs-readings.txt
#>
[CmdletBinding()]
param(
    [string]$AvdName = 'Pixel_4a_API_30',
    [string]$SdkRoot = 'D:\android-sdk',
    [string]$BaseApk = 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX\plugins\uniappx-launcher\base\android_base.apk',
    [string]$LogFile = '.ci-verify/emulator-smoke.log',
    [string]$OutFile = 'docs/verification/tooling/1568/emulator-real-legs-readings.txt'
)

$ErrorActionPreference = 'Stop'
$script:Root = Join-Path $PSScriptRoot '..\..\..'
$Smoke = Join-Path $script:Root 'scripts\emulator-smoke.ps1'
$outAbs = if ([System.IO.Path]::IsPathRooted($OutFile)) { $OutFile } else { Join-Path (Get-Location).Path $OutFile }
$logAbs = if ([System.IO.Path]::IsPathRooted($LogFile)) { $LogFile } else { Join-Path (Get-Location).Path $LogFile }

function Read-Leg([string]$label, [string]$expectExit0, [string[]]$args) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rc = 0
    try {
        & $Smoke @args *>$null
    } catch {
        $rc = if ($_.Exception.ErrorCode) { [int]$_.Exception.ErrorCode } else { 90 }
    }
    $sw.Stop()
    $raw = if (Test-Path -LiteralPath $logAbs) { Get-Content -LiteralPath $logAbs -Raw -Encoding UTF8 } else { '' }
    $has = { param($p) if ($raw -match $p) { 'True' } else { 'False' } }
    $val = { param($p) $m = [regex]::Match($raw, $p); if ($m.Success) { $m.Groups[1].Value } else { 'none' } }
    $line = ('LEG ' + $label + ' exit=' + $rc + ' wall_seconds=' + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
        ' tier_budget_present=' + (& $has 'ADB_TIER_BUDGET') +
        ' install_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=install') +
        ' logcat_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=logcat') +
        ' push_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=push') +
        ' bootwait_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=boot-wait') +
        ' any_timeout=' + (& $has 'ADB_TEXT_TIMEOUT') +
        ' inconclusive=' + (& $val 'LOGCAT_INCONCLUSIVE=(\d+)') +
        ' result=' + (& $val 'EMULATOR_SMOKE_RESULT=(\w+)') +
        ' timeout_count=' + ([regex]::Matches($raw, 'ADB_TEXT_TIMEOUT')).Count +
        ' exit0_expected=' + $expectExit0)
    Add-Content -LiteralPath $outAbs -Value $line -Encoding utf8
    Write-Host $line
    # 汇总行与超时行原文（ASCII 段截出来，中文按本仓口径不进判据）
    foreach ($pat in @('ADB_TIER_BUDGET.*', 'ADB_TEXT_TIMEOUT.*', 'LOGCAT_INCONCLUSIVE=.*')) {
        foreach ($m in ([regex]::Matches($raw, ('(?m)^.*' + $pat)))) {
            $t = $m.Value -replace '[^\x20-\x7E]', '?'
            if ($t.Length -gt 200) { $t = $t.Substring(0, 200) }
            Add-Content -LiteralPath $outAbs -Value ('  RAW ' + $label + ': ' + $t) -Encoding utf8
        }
    }
}

Set-Content -LiteralPath $outAbs -Value ('# emulator-real-legs 读数（真 adb + 真仿真机 ' + $AvdName + '，' + (Get-Date -Format 's') + '）') -Encoding utf8
Add-Content -LiteralPath $outAbs -Value ('BASE base_apk_bytes=' + (Get-Item -LiteralPath $BaseApk).Length + ' smoke=' + $Smoke) -Encoding utf8

Read-Leg 'L1_normal' 'yes' @('-AvdName', $AvdName, '-SdkRoot', $SdkRoot, '-BaseApk', $BaseApk,
    '-SkipInstallBaseApk', '-Pages', 'pages/index/index', '-Module', 'emulator', '-NoArchive')
Read-Leg 'L2_logcat_zero_budget' 'no' @('-AvdName', $AvdName, '-SdkRoot', $SdkRoot, '-BaseApk', $BaseApk,
    '-SkipInstallBaseApk', '-Pages', 'pages/index/index', '-Module', 'emulator', '-NoArchive',
    '-AdbLogcatTimeoutSeconds', '0')
Read-Leg 'L3_install_zero_budget' 'no' @('-AvdName', $AvdName, '-SdkRoot', $SdkRoot, '-BaseApk', $BaseApk,
    '-Pages', 'pages/index/index', '-Module', 'emulator', '-NoArchive',
    '-AdbInstallTimeoutSeconds', '0')
Read-Leg 'L4_cjk_path_install' 'no' @('-AvdName', $AvdName, '-SdkRoot', $SdkRoot, '-BaseApk', $BaseApk,
    '-Pages', 'pages/index/index', '-Module', 'emulator', '-NoArchive')

Add-Content -LiteralPath $outAbs -Value 'LEGS_DONE=1' -Encoding utf8
Write-Host 'LEGS_DONE=1'
