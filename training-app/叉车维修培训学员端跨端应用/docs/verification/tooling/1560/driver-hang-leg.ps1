#!/usr/bin/env pwsh
<#
  #1560 真链路**强制覆盖**腿（一次性，不入库）
  第一轮（real-chain.ps1）的真机读数说明本机热截屏稳定低于 1 秒（949 ms / 1217 ms 是冷通道口径），
  所以「1 秒预算」在真链路上**不保证每轮必挂** ⇒ 那条超时面改用**预算 0**强制照出来，并如实标注为强制覆盖：
    D1 直接调发货的 Invoke-BoundedAdbShot -TimeoutSeconds 0 ⇒ 证 WaitForExit(0) 是**非阻塞**（不是无限等），
       且判 TimedOut 时残帧**不在盘上**
    D2 同一函数 -TimeoutSeconds 1 ⇒ 边际带读数（可能返回也可能判超时，两个结果都是真数据，不挑）
    B2 整条 Invoke-AutoScreenshot -AdbCallTimeoutSeconds 0，两页 ⇒ 期望每页都「callTimeouts 前进 + samples=0
       + 到点 Skipped + 循环继续下一页」，跑完链子自己收口（这就是票面 AC4 的靶心）
  锁与前置同第一轮：Test-BuildEnv 取 HBuilderX 互斥锁，finally 释放。
#>
$ErrorActionPreference = 'Stop'
$scratch = $PSScriptRoot
$repoRoot = Split-Path -Parent (Split-Path -Parent $scratch)
$proj = Join-Path $repoRoot (Join-Path 'training-app' '叉车维修培训学员端跨端应用')
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$log = Join-Path $scratch "real-chain2-$stamp.log"
$outDir = Join-Path $scratch "shots2-$stamp"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

. (Join-Path $proj 'scripts\lib\env-check.ps1')
. (Join-Path $proj 'scripts\lib\hx-busy.ps1')
. (Join-Path $proj 'scripts\lib\auto-screenshot.ps1')

function Line([string]$t) { Write-Host "RC2 | $t" }

Start-Transcript -Path $log | Out-Null
$fail = ''
try {
    $adb = Resolve-AdbExeLocal
    $dev = (Get-OnlineDevices -AdbExe $adb | Select-Object -First 1).Serial
    $a = Test-ScreenAwake -AdbExe $adb -Serial $dev
    Line "DEVICE=$dev ADB=$adb WAKE=$($a.State)"
    if (-not $a.Ok) { & $adb -s $dev shell input keyevent KEYCODE_WAKEUP | Out-Null; Start-Sleep -Seconds 2 }

    # ── D1：预算 0（强制不返回）
    $f1 = Join-Path $outDir 'd1.png'
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $r1 = Invoke-BoundedAdbShot -AdbExe $adb -Serial $dev -OutFile $f1 -TimeoutSeconds 0
    $sw.Stop()
    Line ("D1_budget0 wall_ms={0} TimedOut={1} Exit={2} Seconds={3} file_exists_after={4} err={5}" -f `
            $sw.ElapsedMilliseconds, $r1.TimedOut, $r1.ExitCode, $r1.Seconds, (Test-Path -LiteralPath $f1), $r1.Error)

    # ── D2：预算 1 秒（边际带）
    $f2 = Join-Path $outDir 'd2.png'
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $r2 = Invoke-BoundedAdbShot -AdbExe $adb -Serial $dev -OutFile $f2 -TimeoutSeconds 1
    $sw.Stop()
    $bytes2 = if ((Test-Path -LiteralPath $f2)) { (Get-Item -LiteralPath $f2).Length } else { 0 }
    Line ("D2_budget1 wall_ms={0} TimedOut={1} Exit={2} Seconds={3} bytes={4}" -f `
            $sw.ElapsedMilliseconds, $r2.TimedOut, $r2.ExitCode, $r2.Seconds, $bytes2)

    # ── D3：预算 15 秒（默认值，真机对照 + 字节完整性）
    $f3 = Join-Path $outDir 'd3.png'
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $r3 = Invoke-BoundedAdbShot -AdbExe $adb -Serial $dev -OutFile $f3 -TimeoutSeconds 15
    $sw.Stop()
    if (Test-Path -LiteralPath $f3) {
        $b = [System.IO.File]::ReadAllBytes($f3)
        $magic = ($b[0..3] | ForEach-Object { $_.ToString('X2') }) -join ''
        $bl = Test-ScreenBlank -Path $f3
        Line ("D3_budget15 wall_ms={0} TimedOut={1} Exit={2} bytes={3} magic={4} blank={5} sha256={6}" -f `
                $sw.ElapsedMilliseconds, $r3.TimedOut, $r3.ExitCode, $b.Length, $magic, $bl.Blank,
            (Get-FileHash -LiteralPath $f3 -Algorithm SHA256).Hash)
    }
    else { Line "D3_budget15 TimedOut=$($r3.TimedOut) no-file" }

    # ── B2：整条链 + 预算 0，两页 ⇒ 每页都到点 Skipped 且循环继续
    $be = Test-BuildEnv -ProjectDir $proj -Device $dev -HxWaitSeconds 1800
    if (-not $be.Ok) { throw "Test-BuildEnv 不通过：$($be.Error)" }
    Set-HxLockOwnerEnv
    Line "LOCK_HELD cli=$($be.CliPath)"
    $navProbe = Join-Path (Join-Path $proj '.ci-verify') 'nav-probe.png'
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rB = Invoke-AutoScreenshot -Device $dev -CliPath $be.CliPath -ProjectDir $proj -OutputDir $outDir `
        -Pages 'pages/dashboard/dashboard,pages/index/index' -AdbCallTimeoutSeconds 0 `
        -NavigateMinSeconds 20 -NavigateTimeoutSeconds 30 -MaxPages 2
    $sw.Stop()
    Line ("B2 ok={0} shots={1} skipped={2} wall_seconds={3} nav_probe_left={4}" -f `
            $rB.Ok, ($rB.Screenshots -join '|'), ($rB.Skipped -join '|'), [int]$sw.Elapsed.TotalSeconds,
        (Test-Path -LiteralPath $navProbe))
    Line 'REALCHAIN2_DONE=1'
}
catch {
    $fail = $_.Exception.Message
    Line "REALCHAIN2_FAIL $fail"
}
finally {
    Clear-HxLockOwnerEnv
    Release-HxLock
    Line 'HX_LOCK_RELEASED'
    Stop-Transcript | Out-Null
}
Line "LOG=$log"
if ($fail) { exit 2 } else { exit 0 }
