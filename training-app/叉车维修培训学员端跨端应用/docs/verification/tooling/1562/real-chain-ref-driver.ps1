# 取证腿二（#1562）：只跑对照腿，把真图留在盘上并当场验字节（前一轮的强制腿会把同名文件删掉）
$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$proj = 'D:\FL\wt-1562\training-app\叉车维修培训学员端跨端应用'
$adb = 'D:\android-sdk\platform-tools\adb.exe'
$emu = 'D:\android-sdk\emulator\emulator.exe'
$out = Join-Path $proj '.ci-verify'
$dst = Join-Path $proj 'docs\verification\tooling\1562'

function Show-Shot([string]$Path, [string]$Tag) {
    if (-not (Test-Path -LiteralPath $Path)) { Write-Output ("SHOT " + $Tag + " present=False"); return }
    $len = (Get-Item -LiteralPath $Path).Length
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $magic = (($bytes[0..3] | ForEach-Object { $_.ToString('X2') }) -join '')
    $sha = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    Write-Output ("SHOT " + $Tag + " present=True bytes=" + $len + " magic=" + $magic + " sha256=" + $sha)
    # 出图判据：真图必须**能解码**（headless 下 screencap 出的是有效 PNG，AC4 那一条）
    try {
        Add-Type -AssemblyName System.Drawing -ErrorAction Stop
        $img = [System.Drawing.Image]::FromFile($Path)
        Write-Output ("DECODE " + $Tag + " ok=True " + $img.Width + "x" + $img.Height)
        $img.Dispose()
    } catch {
        Write-Output ("DECODE " + $Tag + " ok=False err=" + $_.Exception.Message)
    }
    $leaf = Split-Path -Leaf $Path
    Copy-Item -LiteralPath $Path -Destination (Join-Path $dst $leaf) -Force
    Write-Output ("COPIED " + $leaf)
}

Write-Output "=== R1 emulator-smoke 对照腿（预算 15，跑完图留在盘上）==="
$log1 = Join-Path $out '1562-emu-ref2.txt'
Start-Process -FilePath 'pwsh' -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $proj 'scripts\emulator-smoke.ps1'), '-Pages', 'pages/index/index,pages/login/login', '-NoArchive', '-PageSettleSeconds', '5', '-SkipInstallBaseApk') -RedirectStandardOutput $log1 -RedirectStandardError ($log1 + '.err') -NoNewWindow -Wait
Write-Output ("BUDGET " + ((Get-Content -LiteralPath $log1 -Raw) -split "`r?`n" | Where-Object { $_ -match 'SHOT_CALL_BUDGET|EMULATOR_SMOKE_RESULT' }) -join ' || ')
Show-Shot (Join-Path $out 'emulator-pages-index-index.png') 'EMU_INDEX'

Write-Output "=== R2 手工起仿真机 + device-capture 对照腿 ==="
$bootOut = Join-Path $out '1562-emu-boot2.txt'
$null = Start-Process -FilePath $emu -ArgumentList @('-avd', 'Pixel_4a_API_30', '-no-snapshot', '-no-boot-anim', '-no-audio', '-port', '5558', '-gpu', 'swiftshader_indirect', '-no-window') -RedirectStandardOutput $bootOut -RedirectStandardError ($bootOut + '.err') -NoNewWindow -PassThru
& $adb wait-for-device 2>&1 | Out-Null
$deadline = (Get-Date).AddSeconds(300)
$booted = $false
while ((Get-Date) -lt $deadline) {
    $bc = (& $adb -s emulator-5558 shell getprop sys.boot_completed 2>&1 | Out-String)
    if ($bc -match '1') { $booted = $true; break }
    Start-Sleep -Seconds 5
}
Write-Output ("BOOTED=" + [bool]$booted)
if ($booted) {
    $log2 = Join-Path $out '1562-cap-ref2.txt'
    Start-Process -FilePath 'pwsh' -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $proj 'scripts\device-capture.ps1'), '-Device', 'emulator-5558', '-Pages', 'pages/login/login', '-NoArchive') -RedirectStandardOutput $log2 -RedirectStandardError ($log2 + '.err') -NoNewWindow -Wait
    Write-Output ("BUDGET " + ((Get-Content -LiteralPath $log2 -Raw) -split "`r?`n" | Where-Object { $_ -match 'SHOT_CALL_BUDGET|DEVICE_CAPTURE_RESULT|截图：' }) -join ' || ')
    Show-Shot (Join-Path $out 'pages-login-login-current.png') 'CAP_CURRENT'
    & $adb -s emulator-5558 emu kill 2>&1 | Out-Null
}
Write-Output "R2_DONE=1"
