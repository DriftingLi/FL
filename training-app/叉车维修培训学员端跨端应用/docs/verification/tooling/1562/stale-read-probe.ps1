# 旧档读回探针（#1562 · 直调真执行核，产物随本目录入库）
#
# 它证明的那一格：`Invoke-BoundedAdbText` 的输出文件按 PID 命名，而 `-RedirectStandardOutput` 只在**这次调用
# 起得来**时才截断它 ⇒ 盘上若留着**上一次调用**的文本，只判「没被判超时」就会把它读成本次结论
# （等于一次 adb 都没发却报告「屏幕亮着」、照样截图）。发货件的判据是 `Exited`。
#
# 握法是现测挑出来的，另两种会造出**空转断言**（M6 第一次跑就是这样照出来的）：
#   · 锁 stderr ⇒ .NET 先按 `FileMode.Create` 截断 stdout ⇒ 旧代码也读不到旧档；
#   · 锁 stdout 但不给 Read 共享 ⇒ 自己那侧 `Get-Content` 一起失败，同样读不到；
#   · ✅ `FileAccess.Write` + `FileShare.Read` ⇒ 写者（重定向）开不进来、读者（`Get-Content`）开得上。
# ⚠️ 只在 Windows 成立：Linux 的 `unlink` 允许删正被打开的文件 ⇒ 起调用前那句清档会成功、旧档留不下
#    （CI 首跑读数 `B12_STALE_STILL_THERE=False`，本机 `True`）。
#
# 用法（在移动项目目录下）：
#   pwsh docs/verification/tooling/1562/stale-read-probe.ps1
#   pwsh docs/verification/tooling/1562/stale-read-probe.ps1 -Lib <path\to\auto-screenshot.ps1>
# 读数含义与两侧对照（发货件 vs 临时把判据退回 `-not TimedOut`）见同目录 README.md 的「M6 这一格为什么值得单独记」。
# 默认随仓解析：本文件在 <项目>/docs/verification/tooling/1562/ ⇒ 上溯四层才是项目目录，再进 scripts/lib
# （少算一层会解析到 docs/scripts 而不报错，只在 dot-source 时才炸——先 Resolve-Path 让它当场点名）
param(
    [string]$Lib = ''
)
if (-not $Lib) {
    $Lib = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..\..\scripts\lib\auto-screenshot.ps1'))
}
if (-not (Test-Path -LiteralPath $Lib -PathType Leaf)) {
    Write-Output ("PROBE_LIB_MISSING=" + $Lib)
    exit 2
}
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
. $Lib
Write-Output ("PROBE_LIB=" + $Lib)
$dir = Join-Path ([System.IO.Path]::GetTempPath()) ('stale-read-probe-' + $PID)
New-Item -ItemType Directory -Path $dir | Out-Null
# 与 Invoke-BoundedAdbText 同名的落档路径（PID 命名 ⇒ 必须算准它才打得到那一格）
$outFile = Join-Path $dir ('adb-call-stdout-{0}.txt' -f $PID)
Set-Content -LiteralPath $outFile -Encoding ascii -Value 'STALE mWakefulness=Awake'
$lock = [System.IO.File]::Open($outFile, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Write, [System.IO.FileShare]::Read)
try {
    $r = Invoke-BoundedAdbText -AdbExe 'cmd.exe' -Serial 'FAKE' -AdbArguments @('shell', 'dumpsys', 'power') `
        -WorkDir $dir -TimeoutSeconds 5
    Write-Output ("P_EXITED=" + [bool]$r.Exited)
    Write-Output ("P_TIMEDOUT=" + [bool]$r.TimedOut)
    Write-Output ("P_HAS_ERROR=" + [bool]($r.Error -ne ''))
    Write-Output ("P_TEXT_HAS_STALE=" + [bool]($r.Text -match 'STALE'))
    $a = Test-ScreenAwake -AdbExe 'cmd.exe' -Serial 'FAKE' -WorkDir $dir -TimeoutSeconds 5
    Write-Output ("P_AWAKE_OK=" + [bool]$a.Ok)
    Write-Output ("P_AWAKE_STATE=" + $a.State)
    Write-Output ("P_STALE_STILL_THERE=" + [bool](Test-Path -LiteralPath $outFile))
} finally {
    $lock.Dispose()
    Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue
}
