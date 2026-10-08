<#
.SYNOPSIS
    #1568 AC 第 2 条的**真链路腿**：把 `emulator-smoke.ps1` 的分档接到真 adb + 真仿真机上跑四遍，
    读数落 `emulator-real-legs-readings.txt`（判据只取 ASCII token —— 本仓血账：pwsh 的中文到上层控制台会变乱码）。

.DESCRIPTION
    为什么必须有这四遍（票面 DoD：工具链改动的验收 = 它要达成的那个行为在真实链路上成立，不是「免四门」）：
    EMT 那九条用的是可编程假载体（要的是「真挂死」这一格 —— 真 adb 不能按需要挂死）。这一份补的是**载体那一头**：
      L1 正常冒烟（-SkipInstallBaseApk，与 #1562 的夹具同形）⇒ 六档现值打进汇总（ADB_TIER_BUDGET），
         且整趟**零** ADB_TEXT_TIMEOUT ⇒ 分档没把正常等误判成挂死（这是「不得套 15 秒」那格的反面证据）。
         ⚠️ 这条是**判据**不是保证：同日两轮同码同参，一轮零超时、另一轮 3 条文本调用到点（通道真挂住过
         一阵）⇒ 两轮对照与可复算边界记在 README §3c，别把「跑过一次零超时」读成「恒零超时」。
      L2 `-AdbLogcatTimeoutSeconds 0` ⇒ 真 logcat -d 到点：汇总必须落 LOGCAT_INCONCLUSIVE=1，
         而不是「FATAL=0 ⇒ 无崩溃」。
      L3 `-AdbInstallTimeoutSeconds 0` ⇒ 真 install 到点：结论必须点名 tier=install，且整条链以 UNUSABLE 收口
         （不永挂）。0 秒档是故意的：它把「预算是参数」这一格打在真载体上。
      L4 中文 APK 路径走新链路（cmd 包装 + stderr 合并）⇒ 必须仍拿到 adb 的失败原文
         （本机 AVD 装基座是 INSTALL_FAILED_NO_MATCHING_ABIS —— 那句原文在 stderr，正好当合并判据的原料）。

    ⚠️ 取证边界（写进 PR 正文，不藏，也别写成比代码宽）：逐页那条 `if ($logcat.TimedOut) { $pageFail += … }`
    在 L2 上**确实执行了** —— 它经 `$failures`（`:869 $passed = ($failures.Count -eq 0)`）把整趟判成 FAIL，
    也就是本腿 `exit=1` 的来源；逐页行带出 `logcatTimedOut=True`、汇总落 `LOGCAT_INCONCLUSIVE=1`。
    **没证到的是页标签本身成为 FAIL**：紧随其后的 `if (-not $hasResources) { $status = 'SKIP' }`（#1562 既有
    规则，票面 AC 第 4 条不许本票动它）把标签盖成 SKIP，而 `-ResourcesDir`（HBuilderX 产物
    `unpackage/resources/app-android`）本机与宿主树现测都 ABSENT ⇒ 「该页记失败」这一**效果**只由
    契约锚点 C11④ + 行为腿 EMT5 锁着；要真跑到它得先有一次 dev 编译产出资源目录。

.PARAMETER OutFile
    读数落盘路径（相对调用时的当前目录）。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/emulator-real-legs.ps1 `
        -OutFile docs/verification/tooling/1568/emulator-real-legs-readings.txt
#>
[CmdletBinding()]
param(
    [string]$AvdName = 'Pixel_4a_API_30',
    [string]$SdkRoot = 'D:\android-sdk',
    [string]$BaseApk = 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX\plugins\uniappx-launcher\base\android_base.apk',
    [string]$OutFile = 'docs/verification/tooling/1568/emulator-real-legs-readings.txt',
    [int]$LegTimeoutSeconds = 420
)

$ErrorActionPreference = 'Stop'
$script:LegTally = @()
# 本轮时间戳：腿的 stdout 按它分组（见 Read-Leg 里那条「取证件不许销毁证据」）
$RunStamp = (Get-Date).ToString('yyyyMMdd-HHmmss')
# RAW 的六条 pattern 提到脚本级，并在这里**逐条真编译一遍**（放在解析路径之前：最便宜的不变式先查）：
# 血账是 `'Failure ['`（Unterminated [] set）—— 它不在解析期报错，而是在第一轮 L1 跑完、进 RAW 循环时才炸，
# 于是白跑一轮仿真机（约 3.5 分钟）且 L2–L4 从未执行。取证驱动自己的错必须在开跑前就红，不能等产物。
$RawPats = @('ADB_TIER_BUDGET', 'ADB_TEXT_TIMEOUT', 'LOGCAT_INCONCLUSIVE', 'ADB_TEXT_CALL_BUDGET', 'INSTALL_FAILED', 'Failure \[')
foreach ($p in $RawPats) {
    try { [void][regex]::new('(?m)^.*' + $p + '.*$') }
    catch { throw ('BAD_RAW_PATTERN ' + $p + ' :: ' + $_.Exception.Message) }
}
Write-Output ('RAW_PATTERNS_COMPILED=' + $RawPats.Count)
# 上溯**四级**才是移动端工程根：1568 → tooling → verification → docs → 根（本文件在 docs/verification/tooling/1568/ 下）。
# ⚠️ 这条算术错过一次，代价记在案：写成三级会解析到 `docs/`，于是 $Smoke 指向不存在的文件，
# 四条腿全部 `exit=90 wall_seconds=0` 秒退、`.ci-verify/emulator-smoke.log` 零行，而读数件照样打满四行、
# 末尾还留一句 `LEGS_DONE=1` —— **取证驱动自己没跑起来，却产出了一份看着像成功的产物**（正是本票在修的那类事）。
# 所以两道兜底：① 下面那句 Test-Path 先 throw（不带 $Smoke 存在就别开始）；② 末尾的 LEGS_DONE 改成带分母与
# log_present 计数，秒退再也伪装不成「跑完了」。
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path
$Smoke = Join-Path $Root 'scripts\emulator-smoke.ps1'
$LogAbs = Join-Path $Root '.ci-verify\emulator-smoke.log'
$outAbs = if ([System.IO.Path]::IsPathRooted($OutFile)) { $OutFile } else { Join-Path (Get-Location).Path $OutFile }
if (-not (Test-Path -LiteralPath $Smoke)) { throw "找不到被验脚本：$Smoke" }
if (Test-Path -LiteralPath $outAbs) { Remove-Item -LiteralPath $outAbs -Force }

function Read-Leg {
    param([string]$Label, [string[]]$SmokeArgs)
    if (Test-Path -LiteralPath $LogAbs) { Remove-Item -LiteralPath $LogAbs -Force }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    # 子进程 stdout 按「本轮 + 标签」落名：先前用固定名，下一轮同标签的腿直接覆盖上一轮的原文 ——
    # 现测吃过一次：12:22 那轮 L1 真撞上一阵 adb 通道不应答（三条文本调用各 15.2/15.3 秒到点、该页 SKIP、
    # 整趟 FAIL），那三条 call= / tier= 的原文只在 real-leg-L1_normal.out 里，12:29 重跑就把它们冲掉了，
    # 读数件又因上面那条 RAW 抓行缺陷没抄全 ⇒ 只剩「timeout_count=3」这一个数。取证件自己不许销毁证据。
    $childLog = Join-Path $Root ('.ci-verify\real-leg-' + $RunStamp + '-' + $Label + '.out')
    # 子进程跑被验脚本：拿**真退出码**（0=PASS / 1=FAIL / 2=UNUSABLE），而不是让 PowerShell 的异常语义冒充结论
    $pi = Start-Process -FilePath 'pwsh' `
        -ArgumentList (@('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $Smoke) + $SmokeArgs) `
        -PassThru -NoNewWindow -RedirectStandardOutput $childLog -RedirectStandardError ($childLog + '.err')
    # 每条腿都有**硬上界**：本票修的正是「没有上界的等」，取证驱动自己不能无界（WaitForExit 带毫秒才是有界等）
    $gone = $pi.WaitForExit(($LegTimeoutSeconds + 60) * 1000)
    if (-not $gone) {
        try { $pi.Kill($true) } catch { }
        $sw.Stop()
        Add-Content -LiteralPath $outAbs -Value ('LEG ' + $Label + ' ABORTED_after=' + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
            ' —— 取证驱动自己的上界到点（这本身就是「还挂着」的结论）') -Encoding utf8
        $script:LegTally += [pscustomobject]@{ Label = $Label; Ran = $false; Why = 'aborted_by_driver_cap' }
        return
    }
    $sw.Stop()
    $rc = $pi.ExitCode
    $raw = if (Test-Path -LiteralPath $LogAbs) { Get-Content -LiteralPath $LogAbs -Raw -Encoding UTF8 } else { '' }
    $tailErr = ''
    $errPath = $childLog + '.err'
    if (Test-Path -LiteralPath $errPath) {
        $errTail = @(Get-Content -LiteralPath $errPath -Tail 3 -ErrorAction SilentlyContinue) -join ' | '
        $tailErr = ($errTail -replace '[^\x20-\x7E]', '?')
        if ($tailErr.Length -gt 200) { $tailErr = $tailErr.Substring(0, 200) }
    }
    $has = { param($p) if ($raw -match $p) { 'True' } else { 'False' } }
    $val = { param($p) $m = [regex]::Match($raw, $p); if ($m.Success) { $m.Groups[1].Value } else { 'none' } }
    $line = ('LEG ' + $Label + ' exit=' + $rc + ' wall_seconds=' + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
        ' log_present=' + $(if ($raw) { 'True' } else { 'False' }) +
        ' tier_budget_present=' + (& $has 'ADB_TIER_BUDGET') +
        ' install_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=install') +
        ' logcat_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=logcat') +
        ' push_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=push') +
        ' bootwait_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=boot-wait') +
        ' teardown_tier=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=teardown') +
        ' inconclusive=' + (& $val 'LOGCAT_INCONCLUSIVE=(\d+)') +
        ' result=' + (& $val 'EMULATOR_SMOKE_RESULT=(\w+)') +
        ' timeout_count=' + ([regex]::Matches($raw, 'ADB_TEXT_TIMEOUT')).Count +
        ' budget_lines=' + ([regex]::Matches($raw, '(?m)^.*ADB_.*BUDGET.*$')).Count +
        ' child_err_tail=' + '"' + $tailErr + '"')
    Add-Content -LiteralPath $outAbs -Value $line -Encoding utf8
    Write-Host $line
    # Ran = 被验脚本**有没有真写出日志**：秒退（路径不存在的 90、参数绑定失败）也是 exit 有值、日志零行，
    # 只看 exit 码会把「根本没开始」读成「跑完了」。末行的分母就是靠这一格撑住的。
    $script:LegTally += [pscustomobject]@{ Label = $Label; Ran = [bool]$raw; Why = 'exit=' + $rc }
    # ⚠️ 抓**整行**而不是抓到的 token 为止：`^.*TOKEN` 只到 token 就停，读数里的 ADB_TEXT_TIMEOUT 行没有
    # call= / tier= / callBudgetSeconds= —— 而这三样正是「哪一次调用、多大预算、哪一档」的全部内容
    # （L1 现测就吃过这一口：timeout_count=3 却答不出是哪三条，判据等于没留）。
    # 名单唯一真源在脚本级（那里自带逐条编译自检），这里只引用——别再抄第二份
    $pats = $RawPats
    foreach ($pat in $pats) {
        foreach ($m in ([regex]::Matches($raw, ('(?m)^.*' + $pat + '.*$')))) {
            $t = ($m.Value -replace '[^\x20-\x7E]', '?').Trim()
            if ($t.Length -gt 220) { $t = $t.Substring(0, 220) }
            Add-Content -LiteralPath $outAbs -Value ('  RAW ' + $Label + ': ' + $t) -Encoding utf8
        }
    }
}

$common = @('-AvdName', $AvdName, '-SdkRoot', $SdkRoot, '-BaseApk', $BaseApk,
            '-Pages', 'pages/index/index', '-Module', 'emulator', '-NoArchive')
Add-Content -LiteralPath $outAbs -Value ('# emulator-real-legs 读数（真 adb + 真仿真机 ' + $AvdName + '，' + (Get-Date -Format 's') + '） smoke=' + $Smoke) -Encoding utf8
Add-Content -LiteralPath $outAbs -Value ('BASE base_apk_bytes=' + (Get-Item -LiteralPath $BaseApk).Length + ' leg_timeout_seconds=' + $LegTimeoutSeconds) -Encoding utf8

Read-Leg -Label 'L1_normal' -SmokeArgs ($common + @('-SkipInstallBaseApk'))
Read-Leg -Label 'L2_logcat_zero_budget' -SmokeArgs ($common + @('-SkipInstallBaseApk', '-AdbLogcatTimeoutSeconds', '0'))
Read-Leg -Label 'L3_install_zero_budget' -SmokeArgs ($common + @('-AdbInstallTimeoutSeconds', '0'))
Read-Leg -Label 'L4_cjk_path_install' -SmokeArgs $common

# 收尾一行带分母：`LEGS_DONE=1` 单独存在时，「四条腿全秒退」与「四条腿全跑完」长得一模一样。
# 现测过的那次就是这样：路径算错 → 四行 exit=90 + 满屏 False/none + LEGS_DONE=1，差一点当证据引用。
$ranCount = @($script:LegTally | Where-Object { $_.Ran }).Count
$totalCount = @($script:LegTally).Count
$verdict = 'LEGS_DONE=' + $ranCount + '/' + $totalCount + ' legs_produced_log=' + $ranCount
if ($ranCount -lt $totalCount) { $verdict += ' LEGS_INCOMPLETE=1' }
Add-Content -LiteralPath $outAbs -Value $verdict -Encoding utf8
Write-Host $verdict
