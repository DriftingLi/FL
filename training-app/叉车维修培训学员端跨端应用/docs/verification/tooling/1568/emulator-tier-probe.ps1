<#
.SYNOPSIS
    量 emulator-smoke.ps1 那 13 处「没界的 adb 直调」在真仿真机上的**真实耗时**，给 #1568 AC 第 2 条的
    分档预算提供可入库的数（AC 第 3 条：每个数指得到入库产物；install / push / wait-for-device / boot
    等待**不得**套 15 秒那一档）。

.DESCRIPTION
    为什么现测：仓内 2026-10-07 的三份冒烟产物（`docs/verification/tooling/1562/1562-emu-ref.txt`、
    `-hang.txt`、`-ref2.txt`）都带 `-SkipInstallBaseApk` ⇒ 冷启动那一格有数（9.6 / 12.7 / 12.8 秒），
    但 `install` / `push` / 收尾的 `emu kill` + `wait-for-disconnect` **一次都没测过**。没有这几个数就定不出
    「长等档」，只能拍脑袋——而拍脑袋正是本票要防的。

    形态与本仓既有探针同源：只读仿真机、自己起自己收（finally 里 `adb emu kill` + `wait-for-disconnect`），
    不碰真机（判据：serial 必须以 `emulator-` 开头，否则拒绝跑），不动 adb server（不 kill-server），
    不发 `input`（不抢 ①a 的焦点判据）。

.PARAMETER AvdName / SdkRoot / Port
    与 `scripts/emulator-smoke.ps1` 的默认值逐字对齐，量出来的才是「那条链路」的数。

.PARAMETER BaseApk
    要 install 的基座 APK（真实大小就是那条调用的自变量；默认取冒烟脚本用的同一个）。

.PARAMETER PushProxyMb
    `push` 那一格没有现成的 `unpackage/resources/app-android`（本树与宿主树都 ABSENT，实测见票面评论），
    所以用「指定大小的临时目录」当代理，并把 MB 数一起写进读数——**代理就是代理，读数里标 PROXY**。

.PARAMETER OutFile
    读数落盘路径（相对调用时的当前目录）。

.EXAMPLE
    pwsh -NoProfile -File docs/verification/tooling/1568/emulator-tier-probe.ps1 `
        -OutFile docs/verification/tooling/1568/emulator-tier-readings.txt -PushProxyMb 48
#>
[CmdletBinding()]
param(
    [string]$AvdName = 'Pixel_4a_API_30',
    [string]$SdkRoot = 'D:\android-sdk',
    [int]$Port = 5554,
    [string]$BaseApk = 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX\plugins\uniappx-launcher\base\android_base.apk',
    [int]$PushProxyMb = 48,
    [int]$BootTimeoutSeconds = 300,
    [string]$OutFile = 'docs/verification/tooling/1568/emulator-tier-readings.txt'
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$AdbExe = Join-Path $SdkRoot 'platform-tools\adb.exe'
$EmulatorExe = Join-Path $SdkRoot 'emulator\emulator.exe'
$Serial = "emulator-$Port"
foreach ($p in @($AdbExe, $EmulatorExe)) {
    if (-not (Test-Path -LiteralPath $p)) { throw "环境缺件：$p" }
}
if ($Serial -notmatch '^emulator-') { throw "探针只允许仿真机 serial，实测=$Serial" }

$work = Join-Path ([System.IO.Path]::GetTempPath()) ("emu-tier-" + [Guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Force -Path $work | Out-Null
$emuOut = Join-Path $work 'emu-stdout.log'
$emuErr = Join-Path $work 'emu-stderr.log'
$outAbs = if ([System.IO.Path]::IsPathRooted($OutFile)) { $OutFile } else { Join-Path (Get-Location).Path $OutFile }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outAbs) | Out-Null
Set-Content -LiteralPath $outAbs -Value '# emulator-tier-probe 读数（每行一条 TIME 记录）' -Encoding utf8

function Rec([string]$tier, [string]$label, [scriptblock]$body, [hashtable]$extra = @{}) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rc = -1
    $text = ''
    $threw = ''
    try {
        $text = (& $body 2>&1 | Out-String)
        $rc = $LASTEXITCODE
    } catch {
        $threw = $_.Exception.Message
    }
    $sw.Stop()
    $ms = $sw.ElapsedMilliseconds
    $one = ($text -split "`r?`n" | Where-Object { $_.Trim() } | Select-Object -First 1)
    if ($one) { $one = $one.Trim() }
    $tail = ($one -replace '[^\x20-\x7E]', '?')
    if ($tail.Length -gt 70) { $tail = $tail.Substring(0, 70) }
    $line = "TIME tier=$tier call=$label ms=$ms rc=$rc"
    foreach ($k in $extra.Keys) { $line += " $k=$($extra[$k])" }
    if ($threw) { $line += " threw=$($threw -replace '[^\x20-\x7E]', '?')" }
    $line += " first=`"$tail`""
    Add-Content -LiteralPath $outAbs -Value $line -Encoding utf8
    Write-Host $line
    return [pscustomobject]@{ Ms = $ms; Rc = $rc; Text = $text; Threw = $threw }
}

$emuProc = $null
try {
    $emuArgs = @('-avd', $AvdName, '-no-snapshot', '-no-boot-anim', '-no-audio', '-port', "$Port",
                 '-gpu', 'swiftshader_indirect', '-no-window')
    Add-Content -LiteralPath $outAbs -Value ("PROBE avd=$AvdName port=$Port base_apk_bytes=" +
        $(if (Test-Path -LiteralPath $BaseApk) { (Get-Item -LiteralPath $BaseApk).Length } else { 'absent' }) +
        " push_proxy_mb=$PushProxyMb emulator=" + (Get-Item -LiteralPath $EmulatorExe).VersionInfo.ProductVersion) -Encoding utf8

    # 冷 case：先记下「设备不在」，再起机
    $null = Rec 'read' 'adb_version_no_serial' { & $AdbExe version }
    $emuProc = Start-Process -FilePath $EmulatorExe -ArgumentList $emuArgs -PassThru `
        -RedirectStandardOutput $emuOut -RedirectStandardError $emuErr

    $cold = Rec 'boot-wait' 'wait_for_device_cold' { & $AdbExe -s $Serial wait-for-device }
    Add-Content -LiteralPath $outAbs -Value ("COLD wait_for_device_ms=" + $cold.Ms) -Encoding utf8

    # boot_completed 轮询的总时长（外层已有 BootTimeoutSeconds，这里量的是它实际用掉多少）
    $swBoot = [System.Diagnostics.Stopwatch]::StartNew()
    $booted = $false
    while ($swBoot.Elapsed.TotalSeconds -lt $BootTimeoutSeconds) {
        $v = (Rec 'read' 'getprop_sys_boot_completed' { & $AdbExe -s $Serial shell getprop sys.boot_completed })
        if ($v.Text -match '1') { $booted = $true; break }
        Start-Sleep -Seconds 2
    }
    $swBoot.Stop()
    Add-Content -LiteralPath $outAbs -Value ("BOOT booted=$booted boot_completed_poll_ms=" + $swBoot.ElapsedMilliseconds) -Encoding utf8

    $null = Rec 'read' 'shell_pm_list_packages' { & $AdbExe -s $Serial shell pm list packages }
    $null = Rec 'read' 'shell_cmd_package_resolve_activity' {
        & $AdbExe -s $Serial shell cmd package resolve-activity --brief -a android.intent.action.MAIN -c android.intent.category.LAUNCHER android
    }
    $null = Rec 'read' 'shell_dumpsys_activity_activities' { & $AdbExe -s $Serial shell dumpsys activity activities }
    $null = Rec 'read' 'shell_dumpsys_window' { & $AdbExe -s $Serial shell dumpsys window }
    $null = Rec 'read' 'logcat_dump' { & $AdbExe -s $Serial logcat -d -v brief }
    $null = Rec 'read' 'logcat_clear' { & $AdbExe -s $Serial logcat -c }
    $null = Rec 'read' 'shell_mkdir_p' { & $AdbExe -s $Serial shell mkdir -p /data/local/tmp/tierprobe }

    # 长等两格：install（真基座 APK）与 push（代理目录，MB 数写进读数）
    if (Test-Path -LiteralPath $BaseApk) {
        $apkMb = [math]::Round((Get-Item -LiteralPath $BaseApk).Length / 1MB, 1)
        $i1 = Rec 'install' 'install_r_t_base_apk' { & $AdbExe -s $Serial install -r -t $BaseApk } @{ apk_mb = $apkMb }
        Add-Content -LiteralPath $outAbs -Value ("INSTALL first_ms=" + $i1.Ms) -Encoding utf8
        # 整段原文（成败与失败码都在这里面）：本机首跑实测 rc=1，光看 first= 那行判不出为什么
        $iLines = @($i1.Text -split "`r?`n" | Where-Object { $_.Trim() } | Select-Object -First 8)
        for ($n = 0; $n -lt $iLines.Count; $n++) {
            Add-Content -LiteralPath $outAbs -Value ("INSTALL_TEXT_" + ($n + 1) + "=" + $iLines[$n].Trim()) -Encoding utf8
        }
        $i2 = Rec 'install' 'install_r_t_repeat' { & $AdbExe -s $Serial install -r -t $BaseApk } @{ apk_mb = $apkMb }
        Add-Content -LiteralPath $outAbs -Value ("INSTALL repeat_ms=" + $i2.Ms) -Encoding utf8
    } else {
        Add-Content -LiteralPath $outAbs -Value 'INSTALL skipped base_apk_absent' -Encoding utf8
    }

    $proxy = Join-Path $work 'pushsrc'
    New-Item -ItemType Directory -Force -Path $proxy | Out-Null
    $bytes = [long]$PushProxyMb * 1MB
    $chunk = New-Object byte[] (1MB)
    (New-Object Random).NextBytes($chunk)
    $fs = [System.IO.File]::Create((Join-Path $proxy 'blob.bin'))
    for ($i = 0; $i -lt $PushProxyMb; $i++) { $fs.Write($chunk, 0, $chunk.Length) }
    $fs.Close()
    $pr = Rec 'push' 'push_dir_proxy' { & $AdbExe -s $Serial push "$proxy/." '/data/local/tmp/tierprobe' } @{ proxy_mb = $PushProxyMb }
    Add-Content -LiteralPath $outAbs -Value ("PUSH proxy note=unpackage_resources_absent 见 PROBE 行；ms_per_mb=" +
        [math]::Round($pr.Ms / $PushProxyMb, 1)) -Encoding utf8

    # 别在 AVD 上留 48 MB 垃圾与临时包
    $null = Rec 'read' 'shell_rm_rf_probe_dir' { & $AdbExe -s $Serial shell rm -rf /data/local/tmp/tierprobe }
    if (Test-Path -LiteralPath $BaseApk) {
        $pk = (& $AdbExe -s $Serial shell pm list packages) 2>&1 | Out-String
        Add-Content -LiteralPath $outAbs -Value ('NOTE base_apk_left_installed pm_list_has_dcloud=' +
            ($pk -match 'dcloud|HBuilder')) -Encoding utf8
    }

    # 挂死风险那一格：设备已被 kill 时 wait-for-device 会不会永不返回（下面收尾之后再测）
} finally {
    if ($emuProc) {
        $k = Rec 'teardown' 'adb_emu_kill' { & $AdbExe -s $Serial emu kill }
        Add-Content -LiteralPath $outAbs -Value ("TEARDOWN emu_kill_ms=" + $k.Ms) -Encoding utf8
        $d = Rec 'teardown' 'adb_wait_for_disconnect' { & $AdbExe -s $Serial wait-for-disconnect }
        Add-Content -LiteralPath $outAbs -Value ("TEARDOWN wait_for_disconnect_ms=" + $d.Ms) -Encoding utf8
        if (-not $emuProc.HasExited) {
            try { $emuProc.WaitForExit(30000) | Out-Null } catch { }
            if (-not $emuProc.HasExited) { try { $emuProc.Kill() } catch { } }
        }
    }
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    Add-Content -LiteralPath $outAbs -Value ('DEVICES_AFTER=' + ((& $AdbExe devices) -join ' | ')) -Encoding utf8
    Add-Content -LiteralPath $outAbs -Value 'PROBE_DONE=1' -Encoding utf8
}
