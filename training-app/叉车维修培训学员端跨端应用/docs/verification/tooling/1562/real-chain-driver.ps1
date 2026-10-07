# 真链路取证驱动（#1562 · 本地跑，不入库）
# 四条腿：仿真机冒烟对照腿 / 仿真机冒烟强制腿（预算 0）/ ①a 取证对照腿 / ①a 取证强制腿
# 每腿自己把完整输出落盘并打印结论行 —— 判据取定性读数（timeouts / SHOT_CALL_TIMEOUT / 文件在不在）。
$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$proj = 'D:\FL\wt-1562\training-app\叉车维修培训学员端跨端应用'
$adb = 'D:\android-sdk\platform-tools\adb.exe'
$emu = 'D:\android-sdk\emulator\emulator.exe'
$out = Join-Path $proj '.ci-verify'
New-Item -ItemType Directory -Force -Path $out | Out-Null

function Run-Leg([string]$Tag, [string[]]$ScriptArgs) {
    $logFile = Join-Path $out ("1562-" + $Tag + ".txt")
    Write-Output ("LEG_START " + $Tag)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $p = Start-Process -FilePath 'pwsh' -ArgumentList (@('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File') + $ScriptArgs) `
        -RedirectStandardOutput $logFile -RedirectStandardError ($logFile + '.err') -NoNewWindow -Wait -PassThru
    $sw.Stop()
    $txt = Get-Content -LiteralPath $logFile -Raw -ErrorAction SilentlyContinue
    if ($null -eq $txt) { $txt = '' }
    $budget = ($txt -split "`r?`n" | Where-Object { $_ -match 'SHOT_CALL_BUDGET' }) -join ' || '
    $timeouts = ($txt -split "`r?`n" | Where-Object { $_ -match 'SHOT_CALL_TIMEOUT' }).Count
    $result = ($txt -split "`r?`n" | Where-Object { $_ -match 'EMULATOR_SMOKE_RESULT=|DEVICE_CAPTURE_RESULT=' }) -join ' || '
    $shotLines = ($txt -split "`r?`n" | Where-Object { $_ -match '^\[.+\] 截图：' }) -join ' || '
    Write-Output ("LEG " + $Tag + " exit=" + $p.ExitCode + " seconds=" + [int]$sw.Elapsed.TotalSeconds)
    Write-Output ("  BUDGET " + $budget)
    Write-Output ("  TIMEOUT_LINES=" + $timeouts)
    Write-Output ("  VERDICT " + $result)
    Write-Output ("  SHOTS " + $shotLines)
}

Write-Output "=== L1 emulator-smoke 对照腿（默认预算 15；x86 AVD 装不上 arm 基座 => 带 -SkipInstallBaseApk，只取截图那一步）==="
Run-Leg 'emu-ref' @((Join-Path $proj 'scripts\emulator-smoke.ps1'), '-Pages', 'pages/index/index,pages/login/login', '-NoArchive', '-PageSettleSeconds', '5', '-SkipInstallBaseApk')
Write-Output "=== L2 emulator-smoke 强制腿（预算 0：每次调用必挂）==="
Run-Leg 'emu-hang' @((Join-Path $proj 'scripts\emulator-smoke.ps1'), '-Pages', 'pages/index/index,pages/login/login', '-NoArchive', '-PageSettleSeconds', '5', '-SkipInstallBaseApk', '-AdbCallTimeoutSeconds', '0')

# ── 手工起一台仿真机给 device-capture 用（①a 链的只读取证）
Write-Output "=== boot for device-capture ==="
$bootOut = Join-Path $out '1562-emu-boot.txt'
$bp = Start-Process -FilePath $emu -ArgumentList @('-avd', 'Pixel_4a_API_30', '-no-snapshot', '-no-boot-anim', '-no-audio', '-port', '5556', '-gpu', 'swiftshader_indirect', '-no-window') `
    -RedirectStandardOutput $bootOut -RedirectStandardError ($bootOut + '.err') -NoNewWindow -PassThru
& $adb wait-for-device 2>&1 | Out-Null
$deadline = (Get-Date).AddSeconds(300)
$booted = $false
while ((Get-Date) -lt $deadline) {
    $bc = (& $adb -s emulator-5556 shell getprop sys.boot_completed 2>&1 | Out-String)
    if ($bc -match '1') { $booted = $true; break }
    Start-Sleep -Seconds 5
}
Write-Output ("BOOTED=" + [bool]$booted + " pid=" + $bp.Id)
if ($booted) {
    Write-Output "=== L3 device-capture 对照腿（默认预算 15）==="
    Run-Leg 'cap-ref' @((Join-Path $proj 'scripts\device-capture.ps1'), '-Device', 'emulator-5556', '-Pages', 'pages/login/login', '-NoArchive')
    Write-Output "=== L4 device-capture 强制腿（预算 0）==="
    Run-Leg 'cap-hang' @((Join-Path $proj 'scripts\device-capture.ps1'), '-Device', 'emulator-5556', '-Pages', 'pages/login/login', '-NoArchive', '-AdbCallTimeoutSeconds', '0')
    # 证据名上不许有半帧（.part 归位判据）：列出目录里的 png / png.part
    $pngs = @(Get-ChildItem -LiteralPath $out -Filter '*.png' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name })
    $parts = @(Get-ChildItem -LiteralPath $out -Filter '*.png.part' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name })
    Write-Output ("EVIDENCE_PNG=" + ($pngs -join ','))
    Write-Output ("STRAY_PART=" + ($parts -join ','))
    & $adb -s emulator-5556 emu kill 2>&1 | Out-Null
}
Write-Output "DRIVER_DONE=1"
