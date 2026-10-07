#!/usr/bin/env pwsh
<#
  #1560 真链路取证（一次性脚本，不入库：落 .scratch/，产物另拷 docs/verification）
  三条腿都走**真 adb + 真设备 + 真 HBuilderX 派发**；不改任何判据默认值（只显式传参）。
    A 对照腿（green）：默认 15 秒单次预算 ⇒ 正常返回**不被误杀**；真 PNG 落盘（字节完整性由这条锁）
    B 挂死腿：-AdbCallTimeoutSeconds 1（低于现测 949 ms / 0.5 s 的正常返回）⇒ 每轮必挂；
      并把 -NavigateMinSeconds 60 顶到大于 -NavigateTimeoutSeconds 8 ⇒ **Skipped 与运气无关**（结构必达），
      读数是「callTimeouts 前进 + 到点判未落定 + 链继续收口」，而不是「等满 420 秒还没挂死」
    C 第二调用点：让导航落定，页面截图那一次吃 1 秒预算 ⇒ 期望点名 SHOT_CALL_TIMEOUT 且继续下一页
  锁：Test-BuildEnv 取 HBuilderX 互斥锁，finally 里 Release-HxLock。
      现测锁持有者 pid=27880 已不存在（2026-10-06 20:06 那次挂死会话的残留）⇒ Acquire-HxLock 按 #974
      的「进程已不存在」判陈旧并抢占，日志会写明原因。
  ⚠️ 启动 HBuilderX 主程序与唤醒屏幕都是**取证前置**（本会话现测 count=0 时必须先起，否则 cli 的本地 IPC 不可达）；
     lib 自己坚持「不注入 input」那条判据没有被本脚本改写。
#>
$ErrorActionPreference = 'Stop'

$scratch = $PSScriptRoot
# .scratch/1560 在 worktree **根**下 ⇒ 上溯两级是树根，移动端项目目录还要再进 training-app\叉车…
$repoRoot = Split-Path -Parent (Split-Path -Parent $scratch)
$proj = Join-Path $repoRoot (Join-Path 'training-app' '叉车维修培训学员端跨端应用')
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$log = Join-Path $scratch "real-chain-$stamp.log"
$outDir = Join-Path $scratch "shots-$stamp"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

. (Join-Path $proj 'scripts\lib\env-check.ps1')
. (Join-Path $proj 'scripts\lib\hx-busy.ps1')
. (Join-Path $proj 'scripts\lib\auto-screenshot.ps1')

function Line([string]$t) { Write-Host "RC | $t" }
function Wake-Device($adb, $dev) {
    $a = Test-ScreenAwake -AdbExe $adb -Serial $dev
    if ($a.Ok) { Line "WAKE state=$($a.State)"; return }
    & $adb -s $dev shell input keyevent KEYCODE_WAKEUP | Out-Null
    Start-Sleep -Seconds 2
    $a2 = Test-ScreenAwake -AdbExe $adb -Serial $dev
    Line "WAKE_BEFORE=$($a.State) WAKE_AFTER=$($a2.State) (取证前置：只唤醒屏幕)"
}

Start-Transcript -Path $log | Out-Null
$fail = ''
try {
    $adb = Resolve-AdbExeLocal
    Line "ADB path=$adb"
    $dev = (Get-OnlineDevices -AdbExe $adb | Select-Object -First 1).Serial
    if (-not $dev) { throw '没有在线设备' }
    Line "DEVICE=$dev"
    Wake-Device $adb $dev

    # 取证前置 2：HBuilderX 主程序必须在跑，否则 cli 的本地 IPC 不可达（本会话现测 count=0）
    if (@(Get-Process HBuilderX -ErrorAction SilentlyContinue).Count -eq 0) {
        $cli0 = Resolve-CliPathLocal
        $exe = Join-Path (Split-Path $cli0) 'HBuilderX.exe'
        Line "HXB_NOT_RUNNING starting exe=$exe"
        Start-Process -FilePath $exe
        for ($i = 1; $i -le 30; $i++) {
            Start-Sleep -Seconds 5
            $c = @(Get-Process HBuilderX -ErrorAction SilentlyContinue).Count
            $resp = Test-HxResponsive -CliExe $cli0 -ProbeTimeoutSeconds 8
            Line "HXB_WAIT try=$i proc=$c responsive=$($resp.Responsive) $($resp.Output)"
            if ($c -gt 0 -and $resp.Responsive) { break }
        }
    } else { Line 'HXB_RUNNING already' }

    $be = Test-BuildEnv -ProjectDir $proj -Device $dev -HxWaitSeconds 1800
    if (-not $be.Ok) { throw "Test-BuildEnv 不通过：$($be.Error)" }
    Set-HxLockOwnerEnv
    Line "LOCK_HELD cli=$($be.CliPath)"

    $res = (Get-Content -LiteralPath (Join-Path $proj 'pages.json') -Raw) | ConvertFrom-Json
    # ⚠️ 页选择照 ADR-0008:370 的实测约束：`pages/index/index` **启动即自跳转**到 dashboard ⇒
    #    按 S17 的页身份判据它本来就该 fail-closed（「请求页 X，实际进入 Y」），拿它当「落定腿」会把
    #    判据行为读成 bug。故对照腿请求 `pages/dashboard/dashboard`（该 ADR 记的通过页），
    #    第二页留 index 只为证「超时/跳页之后循环继续走下一页」。
    $p1 = 'pages/dashboard/dashboard'
    $p2 = $res.pages[0].path
    if ($p1 -notin ($res.pages | ForEach-Object { $_.path })) { throw "pages.json 里没有 $p1" }
    Line "PAGES p1=$p1 (不自跳转) p2=$p2 (ADR-0008:370 记的自跳转页)"

    function ReportPng([string]$path, [string]$tag) {
        if (-not (Test-Path -LiteralPath $path)) { Line "$tag PNG absent path=$path"; return }
        $bytes = [System.IO.File]::ReadAllBytes($path)
        $magic = ($bytes[0..3] | ForEach-Object { $_.ToString('X2') }) -join ''
        $blank = Test-ScreenBlank -Path $path
        Line ("{0} PNG bytes={1} magic={2} blank={3} min={4} max={5} mean={6} sha256={7}" -f `
                $tag, $bytes.Length, $magic, $blank.Blank, $blank.Min, $blank.Max, $blank.Mean,
            (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash)
    }

    # ── 腿 A：默认 15 秒单次预算；导航上限放到 900 秒以吸收首次编译
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rA = Invoke-AutoScreenshot -Device $dev -CliPath $be.CliPath -ProjectDir $proj -OutputDir $outDir `
        -Pages $p1 -NavigateMinSeconds 20 -NavigateTimeoutSeconds 900 -MaxPages 1
    $sw.Stop()
    Line ("LEG_A ok={0} shots={1} skipped={2} wall_seconds={3}" -f `
            $rA.Ok, ($rA.Screenshots -join '|'), ($rA.Skipped -join '|'), [int]$sw.Elapsed.TotalSeconds)
    $pngA = Join-Path $outDir ((Split-Path $p1 -Leaf) + '.png')
    ReportPng $pngA 'LEG_A'
    if ($rA.Skipped.Count -gt 0) {
        # 冷编译可能吃掉第一次导航预算 ⇒ 第二次同参重跑（此时已热），读数记 LEG_A2
        Wake-Device $adb $dev
        $sw2 = [System.Diagnostics.Stopwatch]::StartNew()
        $rA2 = Invoke-AutoScreenshot -Device $dev -CliPath $be.CliPath -ProjectDir $proj -OutputDir $outDir `
            -Pages $p1 -NavigateMinSeconds 20 -NavigateTimeoutSeconds 900 -MaxPages 1
        $sw2.Stop()
        Line ("LEG_A2 ok={0} shots={1} skipped={2} wall_seconds={3}" -f `
                $rA2.Ok, ($rA2.Screenshots -join '|'), ($rA2.Skipped -join '|'), [int]$sw2.Elapsed.TotalSeconds)
        ReportPng $pngA 'LEG_A2'
    }

    # ── 腿 B：单次预算 1 秒 + MinSeconds 60 > TimeoutSeconds 8 ⇒ Skipped 结构必达
    Wake-Device $adb $dev
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rB = Invoke-AutoScreenshot -Device $dev -CliPath $be.CliPath -ProjectDir $proj -OutputDir $outDir `
        -Pages $p1 -AdbCallTimeoutSeconds 1 -NavigateMinSeconds 60 -NavigateTimeoutSeconds 8 -MaxPages 1
    $sw.Stop()
    Line ("LEG_B ok={0} shots={1} skipped={2} wall_seconds={3} (期望：到点 Skipped 且链继续收口，不是挂住)" -f `
            $rB.Ok, ($rB.Screenshots -join '|'), ($rB.Skipped -join '|'), [int]$sw.Elapsed.TotalSeconds)

    # ── 腿 C：导航落定后，页面截图那一次吃 1 秒预算 ⇒ SHOT_CALL_TIMEOUT，且两页都跑完
    Wake-Device $adb $dev
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $rC = Invoke-AutoScreenshot -Device $dev -CliPath $be.CliPath -ProjectDir $proj -OutputDir $outDir `
        -Pages "$p1,$p2" -AdbCallTimeoutSeconds 1 -NavigateMinSeconds 20 -NavigateTimeoutSeconds 600 -MaxPages 2
    $sw.Stop()
    Line ("LEG_C ok={0} shots={1} skipped={2} stale={3} wall_seconds={4}" -f `
            $rC.Ok, ($rC.Screenshots -join '|'), ($rC.Skipped -join '|'), ($rC.StaleShots -join '|'), [int]$sw.Elapsed.TotalSeconds)

    Line 'REALCHAIN_DONE=1'
}
catch {
    $fail = $_.Exception.Message
    Line "REALCHAIN_FAIL $fail"
}
finally {
    Clear-HxLockOwnerEnv
    Release-HxLock
    Line 'HX_LOCK_RELEASED'
    Stop-Transcript | Out-Null
}
Line "LOG=$log"
if ($fail) { exit 2 } else { exit 0 }
