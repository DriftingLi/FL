<#
.SYNOPSIS
    用**真 adb + 真仿真机**跑 `scripts/device-capture.ps1` 的分档收口腿（#1568 AC 第 2 条第二片的真链路取证）。

.DESCRIPTION
    为什么还要这一篇：`utils/deviceCaptureTierBehavior.test.js`（DCT1–DCT13）的载体是**可编程假 adb** ——
    它证明的是「到点给结论、杀整棵进程树、把 `TimedOut` 带进返回值」这套机制，但证明不了两件事：
      ① 真 adb 的答复（`devices -l` / `dumpsys activity activities` / `logcat -d -v epoch`）在换成
         「落盘再读回 + `-MergeStdErr`」之后**仍被原有解析器读懂** —— 假 adb 的那些行是我照着真形状写的；
      ② 预算到点时整条取证链给出的是**一条可见结论**而不是停住 —— 这要真进程树才看得见。
    仿真机是为这一篇准备的载体：`device-capture.ps1` 的红线是「只对设备只读」，而 `emulator-*` serial 上发的
    `devices` / `exec-out screencap` / `shell dumpsys` / `shell logcat -d` **全是只读命令**，不需要真机，
    也不碰维护者的调试会话。判据：serial 必须以 `emulator-` 开头，否则拒绝跑（真机那一侧由人开无线调试）。

    三条腿各量什么（读数在 `device-capture-emulator-readings.txt`）：
      L1_ref      默认预算 ⇒ 正常跑完，核对新 token 在真日志里的**实际形状**与 `ADB_TEXT_CALL_BUDGET timeouts=0`
                  （对照腿：分档没把正常当挂死）。
      L2_logcat0  只把 **logcat 档给 0** ⇒ 基线与全量窗口都到点：应看到 `LOGCAT_INCONCLUSIVE=1`、
                  「崩溃判据不成立」的失败条目，而**整条链照跑完并出图**。这一腿是本族最险那格的真链路见证：
                  旧写法在这里给出空日志 ⇒ 零计数被写成「无崩溃」。
      L3_text0    把**文本档给 0** ⇒ 第一条 `adb devices` 就到点：应 `exit 2` 且**原因行自己点名** `tier=server`，
                  墙钟是个位数秒。旧写法在这一格是**永不返回**，没有对照可给 —— 这一条只能靠「有界后几秒出结论」
                  与改动前「等满驱动上界后 ABORTED」两种产物形态对照（见下面那条 ABORTED 出口）。
    ⚠️ 预算给 0 是**把预算当探针用**（"这次调用不许等"），不是给人用的默认值；默认值与其现测出处只写在
       `scripts/device-capture.ps1` 的 `param()` 注释里，这里不抄数字（抄过来就是第二真源）。

    取证驱动自己的纪律（四条，血账都在同目录 `emulator-real-legs.ps1` 头部与 README §3c）：
      · 上溯**四级**才是移动端工程根（本文件在 docs/verification/tooling/1568/ 下），开跑前先 `Test-Path`
        被验脚本 —— 少一级会让每条腿秒退（exit=90）、产物却看着像跑完了。
      · 每条腿都有**硬上界**：`WaitForExit(ms)` 带毫秒才是有界等；本票修的正是「没有上界的等」，
        驱动自己不能无界。到点打 `ABORTED_after=` 并记 `Ran=false`。
      · 子进程 stdout 按「本轮时间戳 + 标签」落名：固定名会让下一轮冲掉上一轮原文（现测吃过一次）。
      · 收尾结论带**分母**（`LEGS_DONE=n/3` + `log_present=`）：只打 DONE 无法区分「腿都红」与「一条没跑」。
    RAW 抓的是**整行**而不是 token（`call=` / `tier=` / `callBudgetSeconds=` 三样才是「哪一次调用、多大预算、
    哪一档」的全部内容），且 pattern 名单在开跑前**逐条真编译一遍**——非法 pattern 会在第一条腿之后才炸，
    白烧一轮仿真机（血账：`'Failure ['` 是 Unterminated [] set）。

.PARAMETER AvdName / SdkRoot / Port / BootTimeoutSeconds
    与 `scripts/emulator-smoke.ps1` 及同目录 `emulator-tier-probe.ps1` 的默认值逐字对齐 ——
    量的才是「那条链路」的数，换 AVD 得到的读数不可比。

.PARAMETER LegTimeoutSeconds
    单条腿的产品侧上界（驱动再给它 +60 秒的硬上界）。默认 240 秒 = 一次正常取证（截图 + 数条文本调用）
    的最坏外推；到点即判该腿 ABORTED，绝不让取证驱动变成第二个无界等待点。

.PARAMETER OutFile
    读数落盘路径（相对调用时的当前目录）。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/device-capture-emulator-legs.ps1
#>
[CmdletBinding()]
param(
    [string]$AvdName = 'Pixel_4a_API_30',
    [string]$SdkRoot = 'D:\android-sdk',
    [int]$Port = 5554,
    [int]$BootTimeoutSeconds = 300,
    [int]$LegTimeoutSeconds = 240,
    [string]$OutFile = 'docs/verification/tooling/1568/device-capture-emulator-readings.txt'
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# 四级上溯：1568 → tooling → verification → docs → 移动端工程根（scripts/ 与 .ci-verify/ 在那儿）
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path
$Capture = Join-Path $Root 'scripts\device-capture.ps1'
$AdbExe = Join-Path $SdkRoot 'platform-tools\adb.exe'
$EmulatorExe = Join-Path $SdkRoot 'emulator\emulator.exe'
$Serial = "emulator-$Port"
$CaptureLog = Join-Path $Root '.ci-verify\device-capture.log'

# 开跑前的三道存在性断言：缺任何一条就别开始产出产物（「零行日志 + DONE」是最坏的一种假绿）
foreach ($p in @($AdbExe, $EmulatorExe)) { if (-not (Test-Path -LiteralPath $p)) { throw "环境缺件：$p" } }
if (-not (Test-Path -LiteralPath $Capture)) { throw "找不到被验脚本：$Capture（根路径算错？本文件应上溯四级）" }
if ($Serial -notmatch '^emulator-') { throw "本驱动只允许仿真机 serial，实测=$Serial（真机红线：不得拿去打断维护者的会话）" }

$outAbs = if ([System.IO.Path]::IsPathRooted($OutFile)) { $OutFile } else { Join-Path (Get-Location).Path $OutFile }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outAbs) | Out-Null
if (Test-Path -LiteralPath $outAbs) { Remove-Item -LiteralPath $outAbs -Force }
function Add-Reading([string]$Line) {
    Write-Host $Line
    Add-Content -LiteralPath $outAbs -Value $Line -Encoding utf8
}

# RAW 的名单提到脚本级并**逐条真编译**（非法 pattern 必须在开跑前红，不能等第一轮之后）
$RawPats = @('ADB_TEXT_TIMEOUT', 'ADB_TEXT_CALL_BUDGET', 'ADB_TIER_BUDGET', 'LOGCAT_INCONCLUSIVE', 'DEVICE_CAPTURE_RESULT')
foreach ($p in $RawPats) {
    try { [void][regex]::new('(?m)^.*' + $p + '.*$') }
    catch { throw ('BAD_RAW_PATTERN ' + $p + ' :: ' + $_.Exception.Message) }
}
Write-Output ('RAW_PATTERNS_COMPILED=' + $RawPats.Count)

$work = Join-Path ([System.IO.Path]::GetTempPath()) ('dc-emu-legs-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Force -Path $work | Out-Null
$emuOut = Join-Path $work 'emu-stdout.log'
$emuErr = Join-Path $work 'emu-stderr.log'
$emuProc = $null
$script:LegTally = @()
# 本轮时间戳：腿的 stdout 按它分组（取证件不许销毁证据 —— 见头部第三条纪律）
$RunStamp = (Get-Date).ToString('yyyyMMdd-HHmmss')

function Read-Leg {
    param([string]$Label, [string[]]$CaptureArgs)
    if (Test-Path -LiteralPath $CaptureLog) { Remove-Item -LiteralPath $CaptureLog -Force }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $childLog = Join-Path $Root ('.ci-verify\dc-leg-' + $RunStamp + '-' + $Label + '.out')
    # 子进程跑被验脚本：拿**真退出码**（0=PASS / 1=FAIL / 2=UNUSABLE），不让 PowerShell 的异常语义冒充结论
    $pi = Start-Process -FilePath 'pwsh' `
        -ArgumentList (@('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $Capture,
            '-Device', $Serial, '-Pages', 'pages/index/index', '-NoArchive') + $CaptureArgs) `
        -PassThru -NoNewWindow -RedirectStandardOutput $childLog -RedirectStandardError ($childLog + '.err')
    # 硬上界：到点就是「还挂着」的结论本身，并杀掉整棵进程树（本票修的正是没有上界的等）
    $gone = $pi.WaitForExit(($LegTimeoutSeconds + 60) * 1000)
    $sw.Stop()
    if (-not $gone) {
        try { $pi.Kill($true) } catch { }
        Add-Reading ('LEG ' + $Label + ' ABORTED_after=' + [math]::Round($sw.Elapsed.TotalSeconds, 1) +
            ' —— 取证驱动自己的上界到点（这本身就是「还挂着」的结论）')
        $script:LegTally = @($script:LegTally) + @([pscustomobject]@{ Label = $Label; Ran = $false; Why = 'aborted_by_driver_cap' })
        return
    }
    $rc = $pi.ExitCode
    $raw = if (Test-Path -LiteralPath $CaptureLog) { Get-Content -LiteralPath $CaptureLog -Raw -Encoding UTF8 } else { '' }
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
        ' result=' + (& $val 'DEVICE_CAPTURE_RESULT=(\w+)') +
        ' inconclusive=' + (& $val 'LOGCAT_INCONCLUSIVE=(\d)') +
        ' text_calls=' + (& $val 'ADB_TEXT_CALL_BUDGET calls=(\d+)') +
        ' text_timeouts=' + (& $val 'ADB_TEXT_CALL_BUDGET calls=\d+ timeouts=(\d+)') +
        ' hang_server=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=server') +
        ' hang_logcat=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=logcat') +
        ' hang_read=' + (& $has 'ADB_TEXT_TIMEOUT[^\r\n]*tier=read') +
        ' shot_bytes_nonzero=' + (& $has '截图：current-screen|截图：pages-index-index-current') +
        ' timeout_count=' + ([regex]::Matches($raw, 'ADB_TEXT_TIMEOUT')).Count +
        ' child_err_tail=' + '"' + $tailErr + '"')
    Add-Reading $line
    # Ran = 被验脚本**有没有真写出日志**：秒退（路径不存在 / 参数绑定失败）也是 exit 有值而日志零行
    $script:LegTally = @($script:LegTally) + @([pscustomobject]@{ Label = $Label; Ran = [bool]$raw; Why = 'exit=' + $rc })
    foreach ($pat in $RawPats) {
        foreach ($m in ([regex]::Matches($raw, ('(?m)^.*' + $pat + '.*$')))) {
            $t = ($m.Value -replace '[^\x20-\x7E]', '?').Trim()
            if ($t.Length -gt 240) { $t = $t.Substring(0, 240) }
            Add-Reading ('  RAW ' + $Label + ': ' + $t)
        }
    }
}

try {
    $emuArgs = @('-avd', $AvdName, '-port', "$Port", '-no-snapshot-load', '-no-audio', '-no-boot-anim', '-gpu', 'auto')
    Add-Reading '# device-capture 分档收口的仿真机腿读数（每行一条 LEG 记录，RAW 是整行）'
    Add-Reading ('DRIVER avd=' + $AvdName + ' port=' + $Port + ' capture=' + $Capture +
        ' leg_cap_seconds=' + ($LegTimeoutSeconds + 60) + ' at=' + (Get-Date).ToString('yyyy-MM-ddTHH:mm:ss'))
    $swBoot = [System.Diagnostics.Stopwatch]::StartNew()
    $emuProc = Start-Process -FilePath $EmulatorExe -ArgumentList $emuArgs -PassThru `
        -RedirectStandardOutput $emuOut -RedirectStandardError $emuErr
    # 起机这一段**照 emulator-tier-probe.ps1 的同形步骤走**（无界 wait-for-device 冷起实测 29.2 / 32.0 秒，
    # 是已知长等形态，本驱动的射程是 device-capture 那 6 处，不在此处收口）
    $null = & $AdbExe -s $Serial wait-for-device 2>&1 | Out-String
    $booted = $false
    while ($swBoot.Elapsed.TotalSeconds -lt $BootTimeoutSeconds) {
        $v = (& $AdbExe -s $Serial shell getprop sys.boot_completed 2>&1 | Out-String)
        if ($v -match '1') { $booted = $true; break }
        Start-Sleep -Seconds 2
    }
    $swBoot.Stop()
    $devLine = ((& $AdbExe devices 2>&1 | Out-String) -split "`r?`n" | Where-Object { $_ -match $Serial } | Select-Object -First 1)
    Add-Reading ('DRIVER booted=' + $booted + ' boot_ms=' + $swBoot.ElapsedMilliseconds +
        ' devices_line=' + ($devLine -replace '[^\x20-\x7E]', '?'))
    if (-not $booted) { throw "仿真机在 $BootTimeoutSeconds 秒内没起完（sys.boot_completed 没读到 1）—— 腿不开，别产半份产物" }

    Read-Leg -Label 'L1_ref' -CaptureArgs @()
    Read-Leg -Label 'L2_logcat0' -CaptureArgs @('-AdbLogcatTimeoutSeconds', '0')
    Read-Leg -Label 'L3_text0' -CaptureArgs @('-AdbTextCallTimeoutSeconds', '0')
} finally {
    # 收尾自己起的那个仿真机：全程只读命令 + 自己起的进程自己收，不动 adb server、不碰别人的设备
    try { $null = (& $AdbExe -s $Serial emu kill 2>&1 | Out-String) } catch { }
    try { $null = (& $AdbExe -s $Serial wait-for-disconnect 2>&1 | Out-String) } catch { }
    if ($emuProc -and -not $emuProc.HasExited) { try { $emuProc.Kill($true) } catch { } }
    $ranCount = @($script:LegTally | Where-Object { $_.Ran }).Count
    $totalCount = @($script:LegTally).Count
    Add-Reading ('TEARDOWN emulator_stopped=' + $(if ($emuProc -and $emuProc.HasExited) { 'True' } else { 'already_exited_or_null' }))
    Add-Reading ('LEGS_DONE=' + $ranCount + '/3 legs_ran=' + $totalCount +
        ' labels=' + (($script:LegTally | ForEach-Object { $_.Label + '(' + $_.Why + ')' }) -join ','))
}
