<#
.SYNOPSIS
    自动截图模块：只截**本次改动涉及的页面**，真翻页、带反假绿判据。

.DESCRIPTION
    **Q3 口径（2026-09-14 用户裁定）**：只截改动页面 + `-Pages` 参数。
    为什么要限范围：每页一次 `cli launch app-android --pagePath <页>` 都要过一遍 HBuilderX
    （编译 + 推送），本仓 `pages.json` 现数 60 页 ⇒ 全量截图成本不可接受
    （`device-capture.ps1` 早已把「逐页 launch 成本高」标为待裁定，本次裁定即此）。
    ⚠️ 那个 60 是 **2026-10-28 现测底数**（`pages.json` 的 `pages` 段 60 项、`subPackages` 0 项）；
    页数的真源永远是 `pages.json` 本身而不是这句话 ⇒ 页数再变也不必回来改这里（写死的数字会漂，#1861 同族）。

    **`-FullRepo <发版票号>`（#2078，仅发版冒烟语境）**：上面「只截改动页」是**日常 PR 的口径**，
    它管不住发版冒烟要的「全仓逐页 + 首轮基线库」（移动端 ADR-0043 决策 2 与决策 5）。
    `-FullRepo` 就是那枚**显式授权位**，取值必须是发版票号（纯数字）—— 票号既是授权凭证又是归档目录名，
    **不留「只给开关不给票号」的通道**：没有票号的产物无处归档，那就是一次说不出主键的全仓截图。
    开启后：
      · 页清单 = `pages.json` 全量（含 `subPackages` 的页，现数 0 项），**不走** git diff 推导、**不受** `-MaxPages` 裁剪；
      · 产物与首轮基线落 `docs/verification/device/<票号>/`（与 `device-capture.ps1` 的归档主键同一处）；
      · 截图文件名改按**整条路径**生成（`pages_login_login`，不是 `login`）。这条不是美化：本仓 60 条路径的
        末段只有 **59** 个唯一值（`pages/login/login` 与 `pages/recruiter/login` 末段同为 `login`）⇒ 沿用末段命名
        会让 60 页**只落 59 张 PNG**、后一张覆盖前一张，「页清单与产物计数对得上」当场落空，而「hash 与上一页
        相同 ⇒ 切页未生效」还会把这次覆盖读成故障。#2078 实施时现测出来，不是假想边界。
      · **不带 `-FullRepo` 时本文件逐字行为不变**（末段命名、git 推导、`-MaxPages` 裁剪、0 页不回退全量，全部照旧）。
    设备侧 fail-closed 一条都不放宽：未落定的页仍记 `Skipped` 且**不产图**，`Skipped` / `HashConflicts` / 零产物
    照样让 `Ok=$false`（⑥ 的断路腿靠的就是它）。像素那一侧不同：跨发版比对阈值要第二次发版才标定得了
    （决策 5），故首轮基线按「未改页从宽转人看」—— 见 `style-loop.ps1` 的 `-FullRepo` 分支只**转述**像素结论、
    不拿同帧 0.5% 阈值冒充跨天判据（`docs/agents/dev-loop.md:417` 血账：零改动树跨天读到 0.919%）。

    **页面集合的确定顺序**：
      0. `-FullRepo <发版票号>` 给定 → 页集合 = `pages.json` **全量**（`pages` + 各 `subPackages`），
         **不裁剪、不推导**；这一支只由显式授权位进入，日常 PR 走不到（见上 `-FullRepo` 一节）
      1. `-Pages <a,b>` 显式给定 → 用它（最高优先级）
      2. 否则**从 git diff 推导**：把 `pages/**/*.uvue` 的改动映射回 pages.json 里的 page path
         （`-ChangedOnly` 是这一行为的显式声明；两者同给时以 `-Pages` 为准）
      3. 推导出 **0 页 ⇒ 明报「无改动页面」并返回**，**绝不回退到全量**（fail-safe）
      4. `-MaxPages`（默认 5）兜底裁剪，被裁掉的页记入 `Skipped`

    导航与判据：
      · 切页用 `cli launch app-android --pagePath <页>`（`device-capture.ps1` #898 spike 实测：
        `am start` 的 uniapp 深链**不被 App 处理**，会停在原页）。
        ⚠️ 派发**必须分离**（`Start-NavLaunchDetached`）：那个 cli 会话**永不自己收口**，
        前台同步等待会让步骤 6 **永久挂住**（2026-09-15 实测：18 分钟零输出、CPU 0.08 秒）。
      · 「导航已落定」= 有界设备侧判据（`Wait-NavSettled`）：等满 `-NavigateMinSeconds`（默认 15s）
        且画面**连续两次采样内容一致**（`Compare-ScreenFrames` 的**宽容**比较：差异像素占比 ≤
        `-StableMaxDiffPercent`，默认 0.5%）—— **不得**用全帧 hash 相等：系统状态栏里的实时读数
        （MIUI「显示实时网速」）每 1–2 秒就变，会让全帧 hash 恒不同 ⇒ 判据永远不可能满足（2026-09-15 真机实测）。
        到 `-NavigateTimeoutSeconds`（默认 420s）仍未落定 ⇒ 该页记 `Skipped`
        且**不产截图**（fail-closed：宁可可核地明报，也不拿上一页/白屏当本次证据）。
        ⚠️ **单次 adb 调用也必须要有界**（#1560，2026-10-06）：上面那条 420 秒上限**原先不可达** ——
        每轮的 `& cmd.exe /c "adb … > probe.png"` 没有单次超时，一次不返回就永远回不到上限判据
        （票面现测：32 分钟 `.ci-verify` 零写入，而同时刻手工 `screencap` 4.8 秒返回 ⇒ 卡点不是 adb 坏，
        是**那次调用在链里没被约束**）。现在两处 adb 调用（导航采样 + 页面截图）都走 `Invoke-BoundedAdbShot`
        （`-AdbCallTimeoutSeconds`，默认 15s）：超时 ⇒ 终止整棵进程树 + 该轮**不计入采样** + 残帧尽力删
        ⇒ 「不返回」从「整条链挂死」变成「到点 `Settled=false` ⇒ 该页 `Skipped`」，也就是把本文件一直承诺、
        但代码没做到的那条 fail-closed 补实。
        ⚠️ **同一件「有界单次调用」在本仓只有一个实现**（#1562，2026-10-07）：等待/杀树/残帧作废三段
        收成执行核 `Invoke-BoundedAdbCall`，`Invoke-BoundedAdbShot`（截屏，二进制）与
        `Invoke-BoundedAdbText`（取文本，如 `dumpsys power`）是它的两个薄封装 —— 三个消费点
        （本文件的采样与页面截图、本文件的灭屏前置 `Test-ScreenAwake`、①a 取证与仿真机冒烟的那两张图）
        各抄一份正是 ADR-0008「adb 解析的唯一真源」记的那类分叉。后两个消费点在 `device-capture.ps1` /
        `emulator-smoke.ps1`，它们 **dot-source 本文件复用**、不自定义第二份。
        ⚠️ **删除不能当判据**：半张 PNG 若进了「画面稳定」的比较就伪装成一次成功采样（比挂死更坏 —— 谎报落定），
        而真链路现测（2026-10-07）证明 `Kill` 之后删除与子进程句柄有竞争、**可能删不掉** ⇒ 防线是两条逻辑闸门：
        采样轮用 `-not $shot.TimedOut` 不计数；页面截图先落 `.part`、成功才改名成 `*.png`
        （步骤 7 按 `*.png` + 本轮时间戳扫目录，半张图绝不能进证据目录）。
      · 每页截图算 **SHA256**；与上一页**相同 ⇒ 判切页未生效**（记入 `HashConflicts`，令 `Ok=$false`）
      · **截图文件的时间戳必须晚于本次运行起点**：`$OutputDir` 不清理、文件名按页名固定，
        故 adb 静默失败时**上一次运行的同名残留 PNG** 会让「存在且非空」照样通过 ——
        那正是把陈旧截图当本次证据的路径。早于起点的记入 `StaleShots`，并按跳过处理。

    `Ok=true` 仅当：至少截到 1 页、无跳过、无 hash 冲突。

    **adb 解析的唯一真源**（2026-09-15，#1027 血账）：本文件**不得**自带候选集 ——
    这里曾内联第二份（只查 `$env:ANDROID_SDK_ROOT` / `$env:ANDROID_HOME` / `Get-Command adb`），
    比 `lib/env-check.ps1` 的 `Resolve-AdbExeLocal` **少了 `D:\android-sdk\platform-tools\adb.exe` 兜底**
    ⇒ 本机（两个 env 都没设、`adb` 不在 PATH、adb 在 D 盘）**步骤 2/5 能过、步骤 6 恒报「找不到 adb.exe」**；
    而步骤 4/5 已经真跑完并产生副作用（编译 + 部署），步骤 7–9（对比 / 证据 / 还原）永远到不了。
    现统一复用 `env-check.ps1` 的解析器（详因与裁定见
    `docs/adr/0008-移动端验收门与证据.md`「adb 解析的唯一真源」）。

.EXAMPLE
    . scripts/lib/auto-screenshot.ps1
    # 默认：从 git diff 推导改动页面
    $r = Invoke-AutoScreenshot -Device '192.168.10.51:39181' -CliPath 'D:\...\cli.exe' -ProjectDir 'D:\FL\...'
    # 显式指定
    $r = Invoke-AutoScreenshot -Pages 'pages/index/index,pages/profile/profile' -Device '...' -CliPath '...'
    # 仅发版冒烟：全仓逐页 + 首轮基线入库到 docs/verification/device/<票号>/
    $r = Invoke-AutoScreenshot -FullRepo '1864' -Device '...' -CliPath '...' -ProjectDir 'D:\FL\...'
#>

function Start-NavLaunchDetached {
    <#
      切页步专用：**派发后不等待**。
      为什么必须这样（2026-09-15 实测血账）：不带 `--compile` 的真运行 cli **永不自己收口** ——
      实测一个会话存活 18 分钟、CPU 累计 0.08 秒。先前这里是 `& $CliPath @launchArgs 2>&1 | Out-String`
      （前台同步等待、无超时）⇒ 步骤 6 **永久挂住**：截图不落盘，HashConflicts / TargetPages 永远算不出来
      （「截图→对比→证据」那一段从未跑通的直接原因之一）。
      做法与 `hx-run.ps1` 的 `Start-CliLaunchDetached` **同一套**（那边 2026-09-15 由 #1013 落地）：
        包装脚本自己把 cli 输出写进日志文件（`*>`，不经 cmd 重解析）+ `UseShellExecute = $true`
        让子进程拿到**自己的进程树**（不继承本进程的 stdout/stderr 句柄 ⇒ 调用链上任何祖先的
        `| Out-String` / `*> logfile` 都不会被那个常驻会话拖住）。
      **不 kill、不等待、不设超时**：那个会话什么时候结束由下一次 launch 顶掉（实测约 10 秒）。
    #>
    param([string[]]$CliArgs, [string]$CliExe, [string]$LogDir, [string]$LogFile)
    $stamp = [guid]::NewGuid().ToString('N')
    $outFile = Join-Path $LogDir "launch-nav-$stamp.out"
    $errFile = Join-Path $LogDir "launch-nav-$stamp.err"
    $display = "$CliExe " + ($CliArgs -join ' ')
    if ($LogFile) { Add-Content -LiteralPath $LogFile -Value ">>> $display（分离派发，不等收口）" -Encoding utf8 }
    Write-Host ">>> $display"

    $pwshExe = (Get-Process -Id $PID).Path
    if (-not $pwshExe) { $pwshExe = 'pwsh' }
    $wrapFile = Join-Path $LogDir "launch-nav-$stamp.wrap.ps1"
    $argLine = ($CliArgs | ForEach-Object { if ("$_" -match '\s') { '"' + $_ + '"' } else { "$_" } }) -join ' '
    $wrapLines = @(
        '$ErrorActionPreference = ''Continue'''
        # pin child-output decoding to UTF-8: the DCloud CLI writes UTF-8, while pwsh otherwise decodes
        # with the OEM/ACP code page (GBK here) => the Chinese in the log becomes double-encoded mojibake
        # and the app-side page-entry line (`进入页面:"pages/..."`) can never be matched.
        # Keep this wrapper body ASCII-only (it is written as UTF-8 no-BOM and re-parsed by pwsh).
        '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8'
        "& '$CliExe' $argLine *> '$outFile'"
    )
    # 脚本体只含 ASCII（cli 路径 + 参数），按 UTF-8 无 BOM 落盘，避开编码坑
    [System.IO.File]::WriteAllLines($wrapFile, $wrapLines, (New-Object System.Text.UTF8Encoding($false)))

    $si = New-Object System.Diagnostics.ProcessStartInfo
    $si.FileName = $pwshExe
    $si.Arguments = '-NoProfile -ExecutionPolicy Bypass -File "' + $wrapFile + '"'
    $si.UseShellExecute = $true          # ← 关键：新进程树，**不继承**本进程的 stdout/stderr 句柄
    $si.WindowStyle = 'Hidden'
    $si.WorkingDirectory = (Split-Path -Parent $LogDir)
    try {
        $proc = [System.Diagnostics.Process]::Start($si)
    }
    catch {
        # 派发失败也要让调用方拿到对象（fail-closed：Proc=$null 时后面的落定判据自然会判未落定）
        $proc = $null
        if ($LogFile) { Add-Content -LiteralPath $LogFile -Value "[error] 派发导航 launch 失败：$($_.Exception.Message)" -Encoding utf8 }
    }
    return [pscustomobject]@{ Proc = $proc; OutFile = $outFile; ErrFile = $errFile; WrapFile = $wrapFile; Started = (Get-Date) }
}

function Test-ScreenAwake {
    <#
      设备是否**亮屏**（`mWakefulness=Awake`）。
      为什么必须有这条前置：灭屏时 `adb exec-out screencap` 返回的是**全黑帧**，而全黑帧
      **天然满足「画面稳定」** ⇒ 会让「导航已落定」判据瞬间通过、把黑图当成本次证据
      （2026-09-15 实测：两次采样 16 秒即「落定」，两张 PNG 字节完全相同、内容全黑）。
      fail-closed：不亮屏就**不截图**。唤醒设备是**人**的动作，本脚本不注入 input。

      ⚠️ **这次调用同样必须有单次超时**（#1562，2026-10-07；#1560 收口时现测出的第三处无界 adb 调用）：
      旧写法 `& $AdbExe -s $Serial shell dumpsys power | Out-String` 是**前台同步等待、无上限**，
      而它跑在**每页之前** ⇒ 挂住的时机比 #1560 修掉的采样循环**更早**，卡的是「能不能开始截屏」这一步。
      超时后的结论只有一个：**拿不到唤醒状态**（`Ok=$false` + `State='unknown'`），走既有的 fail-closed
      出口拒绝截图 —— **不得**新增第三种状态把「没亮屏」与「不知道亮没亮」变成两条不同后果，
      那会让调用方的判断从「截不截」漂成「截，因为看起来没亮」。
      `TimedOut` 只是**读数**（写进机检行，让人一眼分清「通道不返回」与「设备真没亮」），不改判定。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        [string]$WorkDir = '',
        # 单次 `dumpsys power` 的上限；默认值理由见 `Invoke-BoundedAdbText` 的参数注释（读数指得到入库产物）。
        [int]$TimeoutSeconds = 15
    )
    $r = Invoke-BoundedAdbText -AdbExe $AdbExe -Serial $Serial -AdbArguments @('shell', 'dumpsys', 'power') `
        -WorkDir $WorkDir -TimeoutSeconds $TimeoutSeconds
    if ($r.TimedOut) {
        # 机检行（ASCII token，供守护与取证 grep）：**是读数不是判据** —— 判定仍是下面那句 `Ok=$false`。
        # 先例 `lib/capability-surface.ps1`「最后一行是机检行……它不是判据」与 `NAV_SAMPLE`。
        Write-Host ('AWAKE_PROBE ok=False state=unknown timedOut=True callBudgetSeconds={0}' -f $TimeoutSeconds) -ForegroundColor DarkGray
        return [pscustomobject]@{ Ok = $false; State = 'unknown'; TimedOut = $true; Seconds = $r.Seconds; Error = $r.Error }
    }
    $out = $r.Text
    if ($out -notmatch 'mWakefulness=(\w+)') {
        Write-Host ('AWAKE_PROBE ok=False state=unknown timedOut=False callBudgetSeconds={0}' -f $TimeoutSeconds) -ForegroundColor DarkGray
        return [pscustomobject]@{ Ok = $false; State = 'unknown'; TimedOut = $false; Seconds = $r.Seconds; Error = $r.Error }
    }
    $awakeNow = [bool]($Matches[1] -eq 'Awake')
    Write-Host ('AWAKE_PROBE ok={0} state={1} timedOut=False callBudgetSeconds={2}' -f $awakeNow, $Matches[1], $TimeoutSeconds) -ForegroundColor DarkGray
    return [pscustomobject]@{ Ok = $awakeNow; State = $Matches[1]; TimedOut = $false; Seconds = $r.Seconds; Error = '' }
}

function Get-NavEnteredPage {
    <#
      从一次导航的 launch 日志里取**应用自己打出的页面进入行**（形如 `进入页面:"pages/xxx/yyy"`）。
      这是**页身份**的唯一设备侧证据 —— `dumpsys` 只能给到 Activity / 包名，而 uniapp 全页面同一个 Activity。
      只取**最后一条**；没找到返回空串。日志按 UTF-8 读（DCloud CLI 的输出就是 UTF-8）。
    #>
    param([string]$NavOutFile)
    if (-not $NavOutFile -or -not (Test-Path -LiteralPath $NavOutFile)) { return '' }
    $text = ''
    try { $text = (Get-Content -LiteralPath $NavOutFile -Raw -Encoding utf8 -ErrorAction Stop) } catch { return '' }
    $last = ''
    foreach ($line in ($text -split "`r?`n")) {
        if ($line -notmatch '进入页面') { continue }
        if ($line -match '(pages/[A-Za-z0-9_\-/]+)') { $last = $Matches[1] }
    }
    return $last
}

function Test-ScreenBlank {
    <#
      整帧是否**近似全黑 / 无内容**（动态范围极小且均值很低）。
      为什么要它：灭屏、白屏、切页过渡都可能给出「稳定但无信息」的帧，把它当证据就是假绿
      （2026-09-15 实测：17208 字节的全黑 PNG 一路通过「存在且非空 + 时间戳新 + hash 不变」）。
      采样 24×24 网格；判定 = (max - min) < 16 且 mean < 24。
      取色失败（缺 System.Drawing）⇒ Blank=$false 并回带 Error：**不**因环境缺件判失败，但会在日志留痕。
    #>
    param([string]$Path)
    try {
        Add-Type -AssemblyName System.Drawing -ErrorAction Stop
        $img = [System.Drawing.Image]::FromFile($Path)
        try {
            $bmp = New-Object System.Drawing.Bitmap $img
            $min = 255; $max = 0; $sum = 0; $n = 0
            $stepX = [Math]::Max(1, [int]($bmp.Width / 24))
            $stepY = [Math]::Max(1, [int]($bmp.Height / 24))
            for ($x = 0; $x -lt $bmp.Width; $x += $stepX) {
                for ($y = 0; $y -lt $bmp.Height; $y += $stepY) {
                    $c = $bmp.GetPixel($x, $y)
                    $l = [int](0.299 * $c.R + 0.587 * $c.G + 0.114 * $c.B)
                    if ($l -lt $min) { $min = $l }
                    if ($l -gt $max) { $max = $l }
                    $sum += $l; $n++
                }
            }
            $bmp.Dispose()
            $mean = if ($n -gt 0) { [int]($sum / $n) } else { 0 }
            return [pscustomobject]@{ Blank = (($max - $min) -lt 16 -and $mean -lt 24); Min = $min; Max = $max; Mean = $mean; Sampled = $n }
        }
        finally { $img.Dispose() }
    }
    catch {
        return [pscustomobject]@{ Blank = $false; Min = -1; Max = -1; Mean = -1; Sampled = 0; Error = $_.Exception.Message }
    }
}

function Compare-ScreenFrames {
    <#
      两帧是否**内容一致**（宽容比较，而不是逐字节相同）。
      为什么不能再用全帧 SHA256（2026-09-15 真机实测，血账）：系统状态栏里有**应用控制不了**的实时读数
      （MIUI「显示实时网速」的 KB/s），它每 1–2 秒就变。实测连拍三帧（间隔 5 秒，1080×2400）：
        frame-1 / -2 / -3 的整帧 sha256 **两两不同**；按 100px 横带逐带比较发现
        **只有顶部 0–99px（状态栏）有 >8 的像素差（MaxDiff 188），其余整幅逐字节相同**。
      ⇒ 全帧 hash 恒不同 ⇒ 「画面稳定」判据**在该设备上永远不可能满足** ⇒ 步骤 6 必然 420 秒超时、
        步骤 7–9（对比 / 证据 / 还原）永远跑不到 —— 而 app 画面其实早就落定了。
      现口径：按与 `Test-ScreenBlank` 同一套网格采样，统计**差异像素占比**；≤ `-MaxDiffPercent` 即判内容一致。
        真切页 / 滚动 / 过渡帧是大面积变化（占比远高于阈值）⇒ 该拦的照旧拦，fail-closed 语义不放宽。
      **写实边界（不许假称它更严）**：面积小于阈值的动态元素（例如某个 100×100 的动画角标）会被判成「稳定」。
      取色失败（缺 System.Drawing）⇒ `Same=$false` 且回带 Error：**不**因环境缺件判「稳定」（宁可不落定）。
    #>
    param(
        [string]$Path,
        [string]$PrevPath,
        # 单个采样点被判「变了」的通道差阈值（0–255）
        [int]$DiffThreshold = 8,
        # 差异像素占比上限（%）：超过即判「画面未落定」
        [double]$MaxDiffPercent = 0.5
    )
    if (-not (Test-Path -LiteralPath $Path) -or -not (Test-Path -LiteralPath $PrevPath)) {
        return [pscustomobject]@{ Same = $false; DiffPercent = -1; Sampled = 0; Error = '缺帧文件（首帧无可比对象）' }
    }
    try {
        Add-Type -AssemblyName System.Drawing -ErrorAction Stop
        $imgA = [System.Drawing.Image]::FromFile($Path)
        $imgB = [System.Drawing.Image]::FromFile($PrevPath)
        try {
            $bmpA = New-Object System.Drawing.Bitmap $imgA
            $bmpB = New-Object System.Drawing.Bitmap $imgB
            try {
                if ($bmpA.Width -ne $bmpB.Width -or $bmpA.Height -ne $bmpB.Height) {
                    return [pscustomobject]@{ Same = $false; DiffPercent = -1; Sampled = 0; Error = "两帧尺寸不同（$($bmpA.Width)x$($bmpA.Height) vs $($bmpB.Width)x$($bmpB.Height)）" }
                }
                $stepX = [Math]::Max(1, [int]($bmpA.Width / 24))
                $stepY = [Math]::Max(1, [int]($bmpA.Height / 24))
                $diff = 0; $n = 0
                for ($x = 0; $x -lt $bmpA.Width; $x += $stepX) {
                    for ($y = 0; $y -lt $bmpA.Height; $y += $stepY) {
                        $ca = $bmpA.GetPixel($x, $y); $cb = $bmpB.GetPixel($x, $y)
                        $d = [Math]::Max([Math]::Abs($ca.R - $cb.R), [Math]::Max([Math]::Abs($ca.G - $cb.G), [Math]::Abs($ca.B - $cb.B)))
                        if ($d -gt $DiffThreshold) { $diff++ }
                        $n++
                    }
                }
                $pct = if ($n -gt 0) { [Math]::Round(100.0 * $diff / $n, 3) } else { -1 }
                return [pscustomobject]@{ Same = ($n -gt 0 -and $pct -le $MaxDiffPercent); DiffPercent = $pct; Sampled = $n }
            }
            finally { $bmpA.Dispose(); $bmpB.Dispose() }
        }
        finally { $imgA.Dispose(); $imgB.Dispose() }
    }
    catch {
        return [pscustomobject]@{ Same = $false; DiffPercent = -1; Sampled = 0; Error = $_.Exception.Message }
    }
}

function Invoke-BoundedAdbCall {
    <#
      单次 adb 调用的**有界执行核**（#1560，2026-10-06；#1562 收成一件、供三个消费点共用）：
      一次调用不返回 ⇒ 到点终止整棵进程树并回报 `TimedOut`。

      为什么把「一次调用」收成一件（#1562）：同一个「前台同步等一次 adb、没有单次超时」的形状在本仓有
      **三个**落点（导航采样、页面截图 = #1560 已修；`Test-ScreenAwake` 的 `dumpsys power` = 本票；
      ①a 取证与仿真机冒烟各自的那张 `screencap` = 本票）。等待/杀树/残帧这三段逻辑若各抄一份，
      漂移就是早晚的事 —— ADR-0008「adb 解析的唯一真源」记的血账正是「复制而不是复用」。
      故本核只处理「起一个进程、有界地等它、不放就杀树、产物按 `TimedOut` 作废」，
      具体是哪条 adb 子命令由 `$AdbSubCommand` 传入；两个调用形态各有一个**薄封装**
      （`Invoke-BoundedAdbShot` 截屏、`Invoke-BoundedAdbText` 取文本），封装里没有第二次等待逻辑。

      为什么单独成一个函数：`Wait-NavSettled` 那条 420 秒上限**只在两轮之间**检查一次，而真正可能不返回的是
      那一轮的 adb 调用本身 ⇒ 调用挂住就永远回不到判据（票面现测：32 分钟 `.ci-verify` 零写入，既不落定也不报超时）。
      把「一次调用」收成有界动作，`-TimeoutSeconds` 才**真的可达** —— 本函数就是那条 fail-closed 出口的承重件。

      为什么是 `Start-Process -PassThru` + `WaitForExit(ms)` + `Kill($true)`（票面另有两个候选，各自的不选理由）：
        · `Start-Job`：本仓 `scripts/` **零处先例**，且每次派发多起一个 PowerShell 宿主进程；
        · 复用 `lib/process-capture.ps1` 的 `Invoke-Process`：它把 stdout 按 UTF-8 **读成字符串**
          （`process-capture.ps1:28-29` 设编码、`:55` 取 `.Result`）⇒ 拿它截屏会**改坏 PNG 字节**；
          而它「等异步读段排空」那一段正是 `#1285` 记的坑（`:42-48`）——adb 有**常驻 server**，
          客户端退了句柄未必放 ⇒ 读段会造出第二个无界等待点。
        ⇒ 本函数**让 OS 把 stdout 直接写进文件**（父进程一个字节都不读），只剩「有界等待 + 杀树」，
          读段那条挂死路径从根上不存在。`cmd.exe` 那层保留（它是既有被验证过的重定向载体），
          但**内层 `>` 去掉**、改由 `-RedirectStandardOutput` 落盘：现测同一形状 5 次调用 **2703 ms**，
          旧的 `& cmd.exe /c` 要 **5958 ms** ⇒ 不但不加开销，还省一半。
      2026-10-06 探针实测（`\.scratch\1560\probe-bounded-call.ps1`，九条假设逐条证伪或证实）：
      `WaitForExit(2000)` 对挂死调用返回 False 且 2113 ms 收口 / `Kill($true)` 后 cmd 的孙进程 `ping` 剩 **0** 个 /
      重定向文件**启动即截断**（跑完不写 = 0 字节）/ Kill 之后文件**立刻可删** / 中文目录（`.ci-verify` 在
      `叉车维修…` 下）可用 / 真机 `b32d8398` screencap **1217 ms** 返回 103,678 字节且 PNG 头 `89504E47` 完好。

      **残帧规则（本函数的产物契约，被下面两处调用点依赖）**：被终止的调用**不得被当成一次采样**。
      本函数会尽力删掉残帧（先 `WaitForExit(1500)` 收尸、`Dispose()` 放句柄，再删，删不动重试一次），
      但**删除只是卫生，不是判据** —— 真链路现测（2026-10-07，强制挂死腿预算 0）里 `Kill` 之后子进程仍握着
      重定向句柄，`Remove-Item` 当场失败、文件留在盘上。所以防线在调用方两处：
      `Wait-NavSettled` 用 `-not $shot.TimedOut` 让超时轮**逻辑上**不计数；页面截图那侧先落 `.part`
      再在成功时改名归位（步骤 7 按 `-Filter '*.png'` + 本轮时间戳扫目录，半张图绝不能进证据目录）。
      为什么危险：半张 PNG 拿去 `Compare-ScreenFrames` 极可能判「两帧一致」⇒ **谎报落定**，比原来的挂死更坏。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        # 交给 adb 的子命令原文（不含 `-s <serial>`，那部分由本核拼）。截屏封装传 `exec-out screencap -p`。
        [string]$AdbSubCommand,
        [string]$OutFile,
        # stdout 的落点名可以换，但**不能**与 stdout 同文件（`Start-Process` 直接拒绝）。
        # 默认名沿用 #1560 的两个避讳：不叫 `*.png`、不叫 `*.log`（见下方注释）。
        [string]$StderrName = 'adb-screencap-stderr.txt',
        # argv 各段：**直启分支**（`-DirectExec`）用的那份，与 `$AdbSubCommand` 同义、只是不拼成一条 cmd 串。
        # 为什么要有直启分支：cmd 那一层在 **Linux / macOS 上根本不存在** ⇒ 非 Windows 的调用方（#1568 的
        # `wireless-debug.ps1`：它是机器级工具，行为守护在 CI 的 ubuntu 上真跑）经它就只剩「起不来」一条出口，
        # 看似的「有界」其实是「永远拿不到结果」。等待与杀树仍然只有下面那一份 —— 两条分支共用同一个收口。
        [string[]]$AdbArgv = @(),
        # 直启开关：调用方明确「这一条不必经 cmd.exe」。默认**关** —— 截屏那条 PNG 通道走的仍是
        # #1560 / #1562 已在真机验过的 cmd 重定向载体，本票不把那条已验证的链路换掉。
        [switch]$DirectExec,
        # 单次调用的上限。现测健康返回 **1217 ms**（2026-10-06 真机 b32d8398；票面记的是 4.8 s ⇒ 两个数都留档，
        # 别只引用对默认值有利的那一个）。默认 15 秒 = 现测的 12 倍余量、票面数的 3 倍；一轮最坏耗时
        # （`PollSeconds` 5 + 本值 15）相对 420 秒上限可忽略，而它把「32 分钟零写入」钉死成**最多 15 秒**。
        [int]$TimeoutSeconds = 15
    )
    # 相对路径按**当前 PowerShell 位置**归一：`Start-Process -Redirect*` 得拿到可解析的路径。
    # 两个调用点给的都已是绝对路径（`.ci-verify` 由 `$ProjectDir` 派生），这里只给独立调用者兜底。
    if (-not [System.IO.Path]::IsPathRooted($OutFile)) {
        $OutFile = Join-Path (Get-Location).Path $OutFile
    }
    # stderr 单独一份，且**不能与 stdout 同文件**（`Start-Process` 直接拒绝）。
    # 命名刻意避开两个判据来源：不叫 `*.png`（步骤 7 按 `-Filter '*.png'` 扫目录，`screenshot-diff.ps1:235`）、
    # 不叫 `*.log`（避免被别处的日志扫描捡走当成判据输入）。
    $errFile = Join-Path (Split-Path -Parent $OutFile) $StderrName
    # `-s <serial>` 只在**给了 serial** 时才拼：`adb devices` / `mdns services` / `connect` 这一类是
    # server 级命令，带 `-s` 会被 adb 当参数吃掉（#1568 的 `wireless-debug.ps1` 全走这一类，serial 由
    # 调用方在自己的 argv 里带）。截屏与 `Test-ScreenAwake` 恒有 serial ⇒ 拼出来的串与之前逐字相同。
    $cmd = if ($Serial) { '"{0}" -s {1} {2}' -f $AdbExe, $Serial, $AdbSubCommand }
    else { '"{0}" {1}' -f $AdbExe, $AdbSubCommand }
    try {
        if ($DirectExec) {
            $argList = @()
            if ($Serial) { $argList += @('-s', $Serial) }
            $argList += @($AdbArgv)
            $p = Start-Process -FilePath $AdbExe -ArgumentList $argList `
                -RedirectStandardOutput $OutFile -RedirectStandardError $errFile -NoNewWindow -PassThru
        }
        else {
            $p = Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', $cmd) `
                -RedirectStandardOutput $OutFile -RedirectStandardError $errFile -NoNewWindow -PassThru
        }
    }
    catch {
        # 起不来（路径/权限）≠ 超时：`TimedOut=$false` + `Error` 点名，调用方按「没有采样」处理。
        return [pscustomobject]@{ TimedOut = $false; Exited = $false; ExitCode = -1; Seconds = 0; OutFile = $OutFile; ErrFile = $errFile; ErrTail = ''; Error = "启动 adb 截图调用失败：$($_.Exception.Message)" }
    }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $exited = $p.WaitForExit($TimeoutSeconds * 1000)
    if (-not $exited) {
        try { $p.Kill($true) } catch { }
        $sw.Stop()
        # ⚠️ `Kill` 只是**请求**终止：子进程握着的重定向文件句柄未必已释放 ⇒ 删除与它存在竞争。
        #    真链路现测（2026-10-07，强制挂死腿预算 0）：`Remove-Item` 当场失败，报「残帧未删净」，
        #    且导航采样循环里 `nav-probe.png` 也留在盘上 ⇒ **删除不能当判据用**，它只是卫生。
        #    真正的防线在调用方：超时轮一律按「没有采样」处理（见 `Wait-NavSettled` 里 `-not $shot.TimedOut`）。
        #    这里的收尸等待**仍然有界**（1.5 秒）—— 不得为删干净而无限等，那正是本函数要消灭的形状。
        $null = $p.WaitForExit(1500)
        try { $p.Dispose() } catch { }
        Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
        if (Test-Path -LiteralPath $OutFile) {
            Start-Sleep -Milliseconds 200
            Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
        }
        $left = if (Test-Path -LiteralPath $OutFile) {
            '残帧未删净（句柄竞争；调用方按「没有采样」处理，不依赖本次删除成功）'
        } else { '残帧已作废' }
        return [pscustomobject]@{
            TimedOut = $true; Exited = $false; ExitCode = -1
            Seconds  = [Math]::Round($sw.ElapsedMilliseconds / 1000.0, 1)
            OutFile  = $OutFile; ErrFile = $errFile; ErrTail = (Get-Content -LiteralPath $errFile -TotalCount 1 -ErrorAction SilentlyContinue)
            Error    = "单次 adb 调用未在 $TimeoutSeconds 秒内返回（已终止整棵进程树；$left）"
        }
    }
    $sw.Stop()
    $code = -1
    try { $code = $p.ExitCode } catch { }
    [pscustomobject]@{
        TimedOut = $false; Exited = $true; ExitCode = $code
        Seconds  = [Math]::Round($sw.ElapsedMilliseconds / 1000.0, 1)
        OutFile  = $OutFile; ErrFile = $errFile; ErrTail = (Get-Content -LiteralPath $errFile -TotalCount 1 -ErrorAction SilentlyContinue)
        Error    = ''
    }
}

function Invoke-BoundedAdbShot {
    <#
      一次**截屏**调用的有界封装（#1560 发货件，签名与返回形状一字未动）：
      实现已收成 `Invoke-BoundedAdbCall` 那份有界执行核，本函数只钉「哪条子命令」。
      为什么保留这个名字而不让调用方直调执行核：`exec-out screencap -p` 是**二进制**通道，
      「PNG 必须走 OS 重定向、父进程一个字节都不读」这条约束是它专属的坑位（#1285 / `process-capture.ps1`
      按 UTF-8 读 stdout 会改坏字节），封装在就把坑位绑在调用形态上，而不是散在三个调用点里各记一遍。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        [string]$OutFile,
        [int]$TimeoutSeconds = 15
    )
    return Invoke-BoundedAdbCall -AdbExe $AdbExe -Serial $Serial -AdbSubCommand 'exec-out screencap -p' `
        -OutFile $OutFile -StderrName 'adb-screencap-stderr.txt' -TimeoutSeconds $TimeoutSeconds
}

function Invoke-BoundedAdbText {
    <#
      一次**取文本**调用的有界封装（#1562，2026-10-07）。为什么不能直接套用截屏封装：它写死了
      `exec-out screencap -p`，而 `Test-ScreenAwake` 要的是 `shell dumpsys power` 的**文本**输出。
      为什么仍走同一个执行核（而不是新写一份「有界等待 + 杀树」）：那三段逻辑抄第二份就是分叉
      —— ADR-0008「adb 解析的唯一真源」的血账正是「复制而不是复用」。
      ⚠️ 取文本也**先落盘再读回**，不在父进程读 stdout：读段那条挂死路径（`#1285`：adb 有常驻
      server，客户端退了句柄未必放）在两个形态里是同一条，不能因为「这次是文本」就换回
      `… | Out-String` —— 那正是本票要消灭的形状。
      返回：`TimedOut / Exited / ExitCode / Seconds / Text / Error / OutFile`。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        # adb 子命令的**各段**（例：@('shell','dumpsys','power')），由本函数拼成一条子命令。
        [string[]]$AdbArguments,
        # 落盘目录：调用方不给就用系统临时目录。默认值刻意**不**落在 `.ci-verify` ——
        # 那是证据目录，步骤 7 按 `-Filter '*.png'` + 本轮时间戳扫它，中间产物不该进射程。
        [string]$WorkDir = '',
        # 单次调用上限。默认值理由只引用**指得到入库产物**的读数：#1560 收口会话真链路里同一条有界调用
        # 返回 646 / 719 / 949 / 1217 ms（`docs/verification/tooling/1560/README.md:25` 与 :48-49），
        # 而那出的是一张 ~730 KB 的 PNG ⇒ 本封装要取的一页 `dumpsys power` 文本只会更快，15 秒只会更宽。
        # ⚠️ 本会话另有两次 `dumpsys power` 的 ms 读数**没有落进任何产物** ⇒ 不抄在这里（同一份 ADR 那条
        #   「不抄数字，抄过来就是第二真源」的纪律；注释里的数字必须能被读者复算）。
        # 与截屏共用同一个默认值是为了让「一次**有界** adb 调用」在本仓只有一个预算口径，而不是每个子命令各定一个。
        # ⚠️ 「有界」这个限定是实的：本文件之外仍有文本与管理类 `& $AdbExe … | Out-String` 走无界等待，
        #   数法 = 剥注释 + 按所在函数归属（尺入仓在 `docs/verification/tooling/1568/adb-bounded-count.mjs`，
        #   「26 → 剩余 N」的每一格都在那张票的 PR 证据里用同一把尺复算）。
        #   别把本句读成「全仓每次 adb 调用都有界」，剩余面登记在 #1568。
        [int]$TimeoutSeconds = 15,
        # argv 各段 + 直启开关：透传给执行核的**同一份**等待逻辑（见 `Invoke-BoundedAdbCall` 那两条注释）。
        # 为什么由调用方而不是本封装判平台：判「这台机器有没有 cmd 那一层」是调用方的既有语境
        # （`wireless-debug.ps1` 在非 Windows 上要真跑），而 `Test-ScreenAwake` 那条已验证的形状不该被动到。
        [string[]]$AdbArgv = @(),
        [switch]$DirectExec,
        # stderr 合并：`2>&1` 那一格的等价物。为什么必须留在封装这一侧、且必须显式给：
        # 执行核把 stdout / stderr 落成**两个**文件（`Start-Process` 不允许同文件），而本仓有两条判据
        # 的原料本来就在 stderr 上（`wireless-debug.ps1` 的 pair 失败分类 `protocol fault`、
        # `emulator-smoke.ps1` 的 `Get-AdbOutput` 12 个调用方都吃过 adb 的 stderr 原文）——
        # 默认不合并是为了保住 #1562 已发货的 `Test-ScreenAwake`（它只看 stdout 里的 `mWakefulness=`）。
        [switch]$MergeStdErr
    )
    $dir = if ($WorkDir) { $WorkDir } else { [System.IO.Path]::GetTempPath() }
    # 文件名带 PID：同机并发会话（多 worktree / 多个取证链同时跑）不互相覆盖同一份 stdout。
    $outFile = Join-Path $dir ('adb-call-stdout-{0}.txt' -f $PID)
    $errName = ('adb-call-stderr-{0}.txt' -f $PID)
    # 含空格的段（`-d "uniapp://…"`、带空格的路径）必须逐段加引号，否则经 cmd 那一层会被拆成两个 argv——
    # 旧的 `& $AdbExe @args` 形态由 PowerShell 传参、不存在这一格，收口时不补就等于把坑位换了个位置。
    $joined = ($AdbArguments | ForEach-Object {
        if ($_ -match '\s') { '"{0}"' -f $_ } else { [string]$_ }
    }) -join ' '
    # ⚠️ 起调用**前先清旧档**（#1562 自审发现的窄缝，2026-10-07）：上一轮被 `Kill` 的调用可能因句柄竞争而删不掉，
    #   旧文本就留在同一个 PID 路径上；而 `-RedirectStandardOutput` 只在**这次调用起得来**时才截断文件
    #   （文件被别的句柄握着 ⇒ `Start-Process` 直接抛 ⇒ 执行核走 catch：`TimedOut=$false` + `Exited=$false`）。
    #   ⇒ 只判 `TimedOut` 就会把**上一次调用的文本**读成本次结论。删除在这里仍然只是卫生
    #   （#1560 的不变式：删除不能当判据），真正的闸门在下面那条 `Exited`。
    Remove-Item -LiteralPath $outFile -Force -ErrorAction SilentlyContinue
    # stderr 走同一条纪律：同一个 PID 路径 ⇒ 起调用前清档 + 只在 `Exited` 时读回（#1568 合并那一格的新入口）
    Remove-Item -LiteralPath (Join-Path $dir $errName) -Force -ErrorAction SilentlyContinue
    $call = Invoke-BoundedAdbCall -AdbExe $AdbExe -Serial $Serial -AdbSubCommand $joined `
        -AdbArgv $AdbArgv -DirectExec:$DirectExec `
        -OutFile $outFile -StderrName $errName -TimeoutSeconds $TimeoutSeconds
    $text = ''
    # 读回判据 = `Exited`（进程**真的退出了**）而不是「没被判超时」：`Exited=$false` 同时覆盖两种
    # 「盘上那份不是本次调用的字节」——被判超时、以及 `Start-Process` 起不来（catch 那条）。
    if ($call.Exited -and (Test-Path -LiteralPath $outFile)) {
        $text = (Get-Content -LiteralPath $outFile -Raw -ErrorAction SilentlyContinue)
        if ($null -eq $text) { $text = '' }
    }
    if ($MergeStdErr -and $call.Exited) {
        # 合并顺序 = 先 stdout 再 stderr，与调用方原先 `2>&1` 拿到的**内容**等价（行序可能不同：
        # 两条判据都按 `-match` 取原料，不按行序，实测见 #1568 的 WBD3 腿）。
        $errText = ''
        if (Test-Path -LiteralPath $call.ErrFile) {
            $errText = (Get-Content -LiteralPath $call.ErrFile -Raw -ErrorAction SilentlyContinue)
            if ($null -eq $errText) { $errText = '' }
        }
        if ($errText.Trim()) { $text = if ($text.Trim()) { ($text.Trim() + "`n" + $errText.Trim()) } else { $errText } }
    }
    # 超时 ⇒ 残帧（这里是半份文本）同样作废：`Text` 留空，由调用方按「没拿到」处理。
    Remove-Item -LiteralPath $outFile -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $call.ErrFile -Force -ErrorAction SilentlyContinue
    [pscustomobject]@{
        TimedOut = $call.TimedOut; Exited = $call.Exited; ExitCode = $call.ExitCode
        Seconds  = $call.Seconds; Text = $text; OutFile = $outFile
        ErrFile = $call.ErrFile; ErrTail = $call.ErrTail; Error = $call.Error
    }
}

function Wait-NavSettled {
    <#
      有界的「导航已落定」判据（**设备侧事实**，不靠退出码、不靠固定 sleep）。
      **2026-09-15 修订一（血账）**：旧版只要求「画面连续两次采样一致」——实测被**全黑帧**骗过：
      灭屏 / 编译期间的黑屏天然稳定，16 秒就「落定」，两张黑图当成了本次证据。
      **2026-09-15 修订二（#1027 收尾，真机实测）**：那条「一致」原先是**全帧 hash 相等**，
      在系统状态栏带实时读数的设备上**永远不可能满足**（详见 `Compare-ScreenFrames`）⇒ 改宽容比较。
      现在三条**同时**满足才算落定：
        ① 应用日志里出现**目标页**的页面进入行（`进入页面:"<目标页>"`）—— 页身份的唯一设备侧证据；
        ② 采样帧**不是全黑 / 无内容**（`Test-ScreenBlank`）；
        ③ 已等满 -MinSeconds 且相邻两次采样**内容一致**（`Compare-ScreenFrames`，差异占比 ≤ `-StableMaxDiffPercent`）。
      到 -TimeoutSeconds 仍不满足 ⇒ Settled=$false，调用方记 Skipped 且**不产截图**（fail-closed）。
      日志里进的是**别的页**时，Reason 会点名「请求页 X，实际进入 Y」—— 这就是 ADR-0008 要的**页身份**判据。

      **2026-10-06 修订三（#1560，票面现测：32 分钟 `.ci-verify` 零写入）**：上面那条「到 -TimeoutSeconds 仍未满足
      ⇒ Settled=$false」**原先不可达** —— 每轮那次 adb 调用没有单次超时，一次不返回就永远回不到上限判据
      （票面同时刻手工 `screencap` 4.8 秒返回 ⇒ 卡点不是 adb 坏，是**那次调用在链里没被约束**）。
      现在单次调用由 `Invoke-BoundedAdbShot` 兜住：超时的轮**逻辑上不计入采样**（不靠残帧删得掉），并以 `CallTimeouts` 计数回报；
      四条不满足原因之外多一条**最优先**的「通道无返回」（`Samples=0` 且 `CallTimeouts>0`）——
      「adb 通道不返回」与「画面没稳定」的处置人完全不同，混进通用文案会把前者读成后者。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        [string]$ProbeFile,
        [string]$NavOutFile,
        [string]$ExpectedPage,
        # 中转页白名单的落点（#2078）：给了就表示「进入这些页」也算落到目标页，但结论记 **Redirected**
        # 而不是「普通落定」—— 画面是落点的画面，不能拿它冒充请求页的证据（⑥ 要的就是这个区分）。
        # 白名单**只由数据文件提供**，不在代码里写死页名（写死会漂且不可审计）。
        [string[]]$AllowedLandingPages = @(),
        [int]$MinSeconds = 15,
        [int]$TimeoutSeconds = 420,
        [int]$PollSeconds = 5,
        # 单次 adb 采样调用的上限（#1560，见 `Invoke-BoundedAdbShot`）：`$TimeoutSeconds` 是**整轮导航**的预算，
        # 本值是**一次调用**的预算 —— 缺了后者，前者不可达（挂死的那次调用永远回不到 `:elapsed -ge $TimeoutSeconds`）。
        [int]$CallTimeoutSeconds = 15,
        # 「画面稳定」的宽容阈值（差异像素占比 %），见 Compare-ScreenFrames。
        # 默认 0.5：系统状态栏的实时网速读数约占 0.05%（实测），真切页 / 滚动远高于它。
        [double]$StableMaxDiffPercent = 0.5
    )
    $start = Get-Date
    # 宽容比较需要**两帧同时在盘**：探针文件每轮被覆盖，故留一份上一帧的副本。
    $prevFile = Join-Path (Split-Path -Parent $ProbeFile) 'nav-probe-prev.png'
    if (Test-Path -LiteralPath $prevFile) { Remove-Item -LiteralPath $prevFile -Force -ErrorAction SilentlyContinue }
    $stable = 0
    $sampled = 0
    $callTimeouts = 0
    $entered = ''
    $blankSeen = 0
    $lastDiff = -1
    $lastShotError = ''
    # 机检行格式（ASCII token，供守护与取证 grep）：它**不是判据**，不影响 `Settled` 结论与任何退出码
    # —— 先例 `lib/capability-surface.ps1`「最后一行是机检行……它不是判据」。
    $navLineFmt = 'NAV_SAMPLE settled={0} seconds={1} samples={2} callTimeouts={3} callBudgetSeconds={4} budgetSeconds={5}'
    while ($true) {
        Start-Sleep -Seconds $PollSeconds
        # 单次采样调用**有界**（#1560）：旧写法 `& cmd.exe /c $cmd 2>&1 | Out-Null` 没有单次超时，
        # 一次不返回就永远回不到下面那条 `$elapsed -ge $TimeoutSeconds` ⇒ 420 秒上限形同虚设。
        $shot = Invoke-BoundedAdbShot -AdbExe $AdbExe -Serial $Serial -OutFile $ProbeFile -TimeoutSeconds $CallTimeoutSeconds
        # 用时改在**调用之后**才算：`Seconds` 从此含本次调用的耗时，那句「到上限 N 秒」不再撒谎。
        $elapsed = [int]((Get-Date) - $start).TotalSeconds
        $blankNow = $false
        $sameNow = $false
        if ($shot.TimedOut) {
            # 超时的轮**不计入采样**：不 ++、不比较、不写 prev。
            # 半张 PNG 拿去 `Compare-ScreenFrames` 极可能判「两帧一致」⇒ 未落定的页会被谎报成落定。
            $callTimeouts++
            $lastShotError = $shot.Error
        }
        # ⚠️ `-not $shot.TimedOut` 是**逻辑闸门**，不是对「删除已成功」的重复检查：`Invoke-BoundedAdbShot`
        #    会尽力删残帧，但真链路现测（2026-10-07，强制挂死腿预算 0）证明 `Kill` 之后句柄未放、删除当场失败
        #    ⇒ 计数若只看「文件存在且非空」，就会把被杀的那半帧当成一次成功采样。
        if (-not $shot.TimedOut -and (Test-Path -LiteralPath $ProbeFile) -and (Get-Item -LiteralPath $ProbeFile).Length -gt 0) {
            $sampled++
            $blankNow = (Test-ScreenBlank -Path $ProbeFile).Blank
            if ($blankNow) { $blankSeen++ }
            # ⚠️ **不得**退回「全帧 hash 相等」：那正是被系统状态栏实时读数钉死的写法（见 Compare-ScreenFrames）。
            $cmp = Compare-ScreenFrames -Path $ProbeFile -PrevPath $prevFile -MaxDiffPercent $StableMaxDiffPercent
            $sameNow = [bool]$cmp.Same
            $lastDiff = $cmp.DiffPercent
            if ($sameNow) { $stable++ } else { $stable = 0 }
            Copy-Item -LiteralPath $ProbeFile -Destination $prevFile -Force -ErrorAction SilentlyContinue
        }
        $enter = Get-NavEnteredPage -NavOutFile $NavOutFile
        if ($enter) { $entered = $enter }

        $isExpected = ($entered -and $entered -eq $ExpectedPage)
        $isAllowedLanding = ($entered -and (@($AllowedLandingPages) -contains $entered))
        # 稳定性 / 非全黑 / 最短等待三条判据**一条都不放宽**；白名单只替换「页身份等于请求页」这一条，
        # 且结论明确带 RedirectedTo ⇒ 调用方不会把落点画面当成请求页的证据。
        if (($isExpected -or $isAllowedLanding) -and -not $blankNow -and $elapsed -ge $MinSeconds -and $stable -ge 1) {
            Write-Host ($navLineFmt -f 'True', $elapsed, $sampled, $callTimeouts, $CallTimeoutSeconds, $TimeoutSeconds) -ForegroundColor DarkGray
            $redir = if (-not $isExpected) { [string]$entered } else { '' }
            $why = if ($redir) {
                "已进入白名单落点 $redir（请求页 $ExpectedPage 为中介页）且画面稳定（相邻两次采样差异 $lastDiff% ≤ $StableMaxDiffPercent%）"
            } else {
                "已进入目标页且画面稳定（相邻两次采样差异 $lastDiff% ≤ $StableMaxDiffPercent%）"
            }
            return [pscustomobject]@{ Settled = $true; RedirectedTo = $redir; Seconds = $elapsed; Samples = $sampled; CallTimeouts = $callTimeouts; EnteredPage = $entered; DiffPercent = $lastDiff; Reason = $why }
        }
        if ($elapsed -ge $TimeoutSeconds) {
            $why = if ($sampled -eq 0 -and $callTimeouts -gt 0) {
                "到上限 $TimeoutSeconds 秒，$callTimeouts 次 adb 采样调用**全部未在 $CallTimeoutSeconds 秒内返回**（残帧已当场作废，一次采样都没拿到）——设备侧通道疑似不可用，末次：$lastShotError"
            }
            elseif ($entered -and $entered -ne $ExpectedPage) {
                "请求页 $ExpectedPage，日志里最后进入的是 $entered（页身份不符）"
            }
            elseif ($sampled -gt 0 -and $blankSeen -ge $sampled) {
                "到上限 $TimeoutSeconds 秒画面始终是全黑（采样 $sampled 次）——设备可能灭屏"
            }
            elseif (-not $entered) {
                "到上限 $TimeoutSeconds 秒日志里始终没有页面进入行（launch 未真正跑起来）"
            }
            else {
                $tail = if ($callTimeouts -gt 0) { "；其间 $callTimeouts 次采样调用未在 $CallTimeoutSeconds 秒内返回" } else { '' }
                "到上限 $TimeoutSeconds 秒未同时满足「进入目标页 + 非全黑 + 画面稳定（相邻帧差异 ≤ $StableMaxDiffPercent%）」（最后一次差异 $lastDiff%）$tail"
            }
            Write-Host ($navLineFmt -f 'False', $elapsed, $sampled, $callTimeouts, $CallTimeoutSeconds, $TimeoutSeconds) -ForegroundColor DarkGray
            return [pscustomobject]@{ Settled = $false; RedirectedTo = ''; Seconds = $elapsed; Samples = $sampled; CallTimeouts = $callTimeouts; EnteredPage = $entered; DiffPercent = $lastDiff; Reason = $why }
        }
    }
}

function Get-FullRepoPageList {
    <#
    .SYNOPSIS
        纯函数：`pages.json` 对象 → **全量**页路径清单（`pages` + 各 `subPackages`）。
    .DESCRIPTION
        为什么不复用已有的 `$allPages`（= `$pagesJson.pages`）：那是 git diff **映射校验**用的集合，
        本票 ③ 明写「不带 -FullRepo 时行为逐字不变」⇒ 动它会顺带改掉日常 PR 的推导射程。
        全仓逐页要的却是「一个页都不漏」，所以 `subPackages` 必须进来（现数 0 项，但清单来源不能写死成 0）。
        `subPackages` 的 `path` 是**相对各自 root** 的 ⇒ 真正的 pagePath = `root/path`，直接取 `path`
        会把 `--pagePath` 指到一个不存在的页（fail-closed 那边只会记 Skipped，白烧一轮机时）。
        Set-StrictMode 兼容：属性存在性先问 `PSObject.Properties.Name`，不裸取（缺属性在 Latest 下直接抛）。
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)]$PagesJson)

    function Get-PathOf($obj) {
        if (-not $obj) { return '' }
        if (@($obj.PSObject.Properties.Name) -notcontains 'path') { return '' }
        return "$($obj.path)".Trim()
    }

    # 属性存在性必须**先问再取**（#2078 行为腿 FR1 当场抓出来的真实缺陷）：
    # 调用方 dev:finish / style-loop 都在 Set-StrictMode -Version Latest 下跑，而 Latest 对
    # 「对象上没有这个属性」是直接抛、不是给 $null。本仓真实 pages.json **根本没有 subPackages 这个键**
    # （现测 keys = pages, globalStyle, tabBar, uniIdRouter）⇒ 裸写 @($PagesJson.subPackages) 会让
    # -FullRepo 在真实项目上第一步就炸，而在任何带该键的夹具上都不会 —— 这种「只在真数据上坏」
    # 的形状，接线守护看不见，只有真读真文件的行为守护抓得到。
    $names = @($PagesJson.PSObject.Properties.Name)
    $list = @()
    if ($names -notcontains 'pages') { return @() }
    foreach ($p in @($PagesJson.pages)) {
        $one = Get-PathOf $p
        if ($one) { $list += $one }
    }
    # 没有 subPackages 键（本仓现状）⇒ 到此就是全量，不再碰那个不存在的属性
    if ($names -notcontains 'subPackages') { return @($list | Where-Object { $_ } | Select-Object -Unique) }
    foreach ($sp in @($PagesJson.subPackages)) {
        if (-not $sp) { continue }
        $root = ''
        if (@($sp.PSObject.Properties.Name) -contains 'root') { $root = "$($sp.root)".Trim().Trim('/') }
        foreach ($p in @($sp.pages)) {
            $one = Get-PathOf $p
            if (-not $one) { continue }
            $list += $(if ($root) { "$root/$one" } else { $one })
        }
    }
    return @($list | Where-Object { $_ } | Select-Object -Unique)
}

function Get-FullRepoShotName {
    <#
    .SYNOPSIS
        纯函数：页路径 → 截图文件基名（整条路径拍平，**不用末段**）。
    .DESCRIPTION
        这条是本票实施时**现测**出来的坑，不是风格偏好：主树 60 条页路径的末段只有 **59** 个唯一值
        （`pages/login/login` 与 `pages/recruiter/login` 末段同为 `login`）。沿用日常形态的末段命名，
        60 页只会落 59 张 PNG（后一张覆盖前一张），于是
          · ⑥「60 页清单与产物计数相符」当场落空，且
          · 「hash 与上一页相同 ⇒ 切页未生效」会把这次覆盖读成一次故障（HashConflicts 假红）。
        只在 -FullRepo 形态改命名 ⇒ 日常 PR 的产物名逐字不变（票 ③）。
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$Page)
    $flat = ($Page -replace '[\\/]', '_') -replace '[^\w.\-]', '_'
    return ($flat -replace '^[_.]+', '' -replace '[_.]+$', '')
}

function Get-RedirectMap {
    <#
    .SYNOPSIS
        读中转页白名单（`scripts/lib/auto-screenshot-redirects.json`），返回 hashtable：请求页 -> 允许落点数组。
    .DESCRIPTION
        为什么是**数据文件**而不是代码里的 if：(a) 60 页里哪些页是中介页是项目事实、会随产品变；
        (b) 判据要可审计 —— 人打开文件就能看到「这一页允许落到哪」，而不是 diff 代码去猜；
        (c) 漏补一条的后果必须是 fail-closed（该页 Skipped），不能是假绿。
        文件缺失 / 解析失败 ⇒ 返回空表并**明报**（不是静默降级）：那等于「本轮不认任何中转页」，
        做法上安全，但人会莫名其妙地看到中介页 Skipped，所以必须说一句。
    #>
    [CmdletBinding()]
    param([string]$Path)
    $map = @{}
    if (-not $Path -or -not (Test-Path -LiteralPath $Path)) {
        Write-Host "[auto-screenshot] 中转页白名单不存在：$Path（本轮不认任何中转页，中介页会记 Skipped）" -ForegroundColor Yellow
        return $map
    }
    try {
        $json = Get-Content -LiteralPath $Path -Raw -Encoding utf8 | ConvertFrom-Json
        $names = @($json.PSObject.Properties.Name)
        if ($names -notcontains 'redirects') { return $map }
        foreach ($k in @($json.redirects.PSObject.Properties.Name)) {
            $map[$k] = @($json.redirects.$k | Where-Object { $_ })
        }
    }
    catch {
        Write-Host "[auto-screenshot] 中转页白名单解析失败（$($_.Exception.Message)）⇒ 本轮不认任何中转页" -ForegroundColor Yellow
        return @{}
    }
    return $map
}

function Select-FullRepoAttempted {
    <#
    .SYNOPSIS
        纯函数：从全量清单取**本轮**要跑的页（-RampPages 的分流处）。
    .DESCRIPTION
        为什么需要它：票 ⑥ 要求「先一页起跑通、再逐步扩」，而票 ①/④.1 又要求 -FullRepo 是全量、不裁剪。
        两者不打架的前提是分清**两个数**：*本轮跑几页*（可以 ramp）与*全仓清单有几页*（恒为全量，用于累计对账）。
        把「起跑」实现成「裁掉全仓清单」是错的 —— 那样 manifest 里的全仓计划也跟着变小，
        「还差 59 页」这件事就从数据里消失了，分夜跑与 #2077 续跑都失去输入。
    #>
    [CmdletBinding()]
    param(
        [string[]]$PlannedPages = @(),
        [int]$RampPages = 0
    )
    $all = @($PlannedPages | Where-Object { $_ })
    if ($RampPages -le 0) { return $all }
    if ($RampPages -ge @($all).Count) { return $all }
    return @($all[0..($RampPages - 1)])
}

function Get-FullRepoCounts {
    <#
    .SYNOPSIS
        纯函数：页清单 ⇄ 产物计数的**对账**（⑥ 那条判据的机器形态）。
    .DESCRIPTION
        「计数相符」判的是*每个计划页都有一个同名产物*，不是「截到的张数 ≤ 计划数」——后者在
        重名覆盖那一形下 59 ≤ 60 照样绿，正是被这条判据要挡住的假绿。
        Missing 点名到页，不只给数（4 小时无人值守的夜里，一行「缺哪几页」决定这轮能不能续）。
    #>
    [CmdletBinding()]
    param(
        [string[]]$PlannedPages = @(),
        [string[]]$CapturedNames = @(),
        [string[]]$SkippedNames = @(),
        # 中转页（拍了也没有自己的画面）：算「已交代」但**不产出** —— 必须与 Captured 分开传，
        # 否则「已交代」会被读成「有图」，正是这条判据要挡的假绿方向。
        [string[]]$RedirectedNames = @()
    )
    $expected = @($PlannedPages | ForEach-Object { Get-FullRepoShotName $_ })
    $missing = @($expected | Where-Object {
        (@($CapturedNames) -notcontains $_) -and (@($RedirectedNames) -notcontains $_)
    })
    $captured = @($CapturedNames | Where-Object { @($expected) -contains $_ })
    return [pscustomobject]@{
        PlannedCount  = @($expected).Count
        CapturedCount = @($captured).Count
        SkippedCount  = @($SkippedNames).Count
        Missing       = $missing
        CountsMatch   = (@($missing).Count -eq 0)
    }
}

function Get-FullRepoManifestData {
    <#
    .SYNOPSIS
        纯函数：把一轮全仓逐页的输入拼成 manifest 对象（含 ⑥ 要的对账结论与逐页时间账）。
    .DESCRIPTION
        拆出来的理由：这一段既要在**循环里每页**调（中断也留有已产清单），又要在结尾收口时调一次，
        两处各拼一遍字段必然漂（漂的结果是「跑完的 manifest 与中途的不是同一份形状」，#2077 续跑读它就错开）。
        计数对账不在这里重写判据 —— 直接调 `Get-FullRepoCounts`。
    #>
    [CmdletBinding()]
    param(
        [string]$Ticket = '',
        [string[]]$PlannedPages = @(),
        # 本轮实际要跑的页（-RampPages 起跑时是 PlannedPages 的前缀）。不给即本轮=全量。
        # 分成两个字段是本形态的判据骨架：CountsMatch 判**本轮**是否都产出，
        # DirCountsMatch 判**全仓**是否已经齐 —— 跑第 1 页的夜里前者绿、后者差 59 页，两者都得看得见。
        [string[]]$AttemptedPages = @(),
        [string[]]$Screenshots = @(),
        [string[]]$Skipped = @(),
        # 中转页记录（Page / Name / To / Seconds）：进 manifest 做审计，并在计数里当「已交代」
        $Redirected = @(),
        [string[]]$HashConflicts = @(),
        [string[]]$StaleShots = @(),
        $PageSeconds = @(),
        [Nullable[datetime]]$RunStartedAt = $null,
        # 归档目录（给了就连目录一起算**累计**进度）：一轮约 3.9 小时、必须可分夜跑，
        # 目录里的 PNG 是**跨轮累积**的集合。只报「本轮已产」会把第二夜读成「几乎从零开始」，
        # 而只报「目录里有几张」又会把上一夜的失败页算进本轮 —— 所以两个都报，各标各的语义。
        [string]$OutputDir = ''
    )
    $attempted = $(if (@($AttemptedPages).Count -gt 0) { @($AttemptedPages) } else { @($PlannedPages) })
    $redirectedNames = @($Redirected | Where-Object { $_ } | ForEach-Object { [string]$_.Name })
    $counts = Get-FullRepoCounts -PlannedPages $attempted -CapturedNames $Screenshots -SkippedNames $Skipped -RedirectedNames $redirectedNames
    $dirCaptured = @()
    $dirMissing = @()
    $dirPngCount = 0
    if ($OutputDir -and (Test-Path -LiteralPath $OutputDir)) {
        $dirNames = @(Get-ChildItem -Path $OutputDir -Filter '*.png' -File | ForEach-Object { $_.BaseName })
        $dirPngCount = @($dirNames).Count
        $expectedAll = @($PlannedPages | ForEach-Object { Get-FullRepoShotName -Page $_ })
        $dirCaptured = @($expectedAll | Where-Object { $dirNames -contains $_ })
        $dirMissing = @($expectedAll | Where-Object { $dirNames -notcontains $_ })
    }
    $sumSecs = 0
    foreach ($r in @($PageSeconds)) { if ($r) { $sumSecs += [int]$r.Seconds } }
    return [pscustomobject]@{
        Schema         = 'fullrepo-manifest/1'
        ReleaseTicket  = $Ticket
        RunStartedAt   = $(if ($null -ne $RunStartedAt) { [datetime]$RunStartedAt } else { $null })
        PlannedCount   = @(@($PlannedPages) | ForEach-Object { Get-FullRepoShotName -Page $_ } | Select-Object -Unique).Count
        AttemptedCount = $counts.PlannedCount
        RampActive     = (@($attempted).Count -lt @($PlannedPages).Count)
        CapturedCount  = $counts.CapturedCount
        SkippedCount   = $counts.SkippedCount
        Missing        = @($counts.Missing)
        Redirected     = @($Redirected)
        RedirectedCount = @($redirectedNames).Count
        # CountsMatch 只判**本轮**（attempted）是否都产出；全仓齐不齐看下面的 DirCountsMatch
        # 累计面（跨轮）：目录里现在有几张 / 缺哪几页 —— 连跑与收口取这一组
        DirPngCount    = $dirPngCount
        DirCaptured    = @($dirCaptured)
        DirMissing     = @($dirMissing)
        CountsMatch    = [bool]$counts.CountsMatch
        DirCountsMatch = (@($dirMissing).Count -eq 0 -and @($PlannedPages).Count -gt 0)
        HashConflicts  = @($HashConflicts)
        StaleShots     = @($StaleShots)
        TotalPageSeconds = $sumSecs
        Pages          = @($PageSeconds)
        WrittenAt      = (Get-Date)
    }
}

function Write-FullRepoManifest {
    <#
    .SYNOPSIS
        把一轮全仓逐页的清单 / 产物 / 计数 / 时间账落成归档目录里的 `fullrepo-manifest.json`。
    .DESCRIPTION
        为什么每页都重写一遍而不是只在结尾写：一轮约 3.9 小时纯机时，**中途被断电或被 kill 是常态路径**
        （#2077 的断点续跑就是为这件事建的）。结尾才落盘的 manifest 在挂掉的运行里等于没有；
        每页重写让它同时充当「已产清单」，续跑才有输入可读。落盘失败**明报但不改判据**（Ok 仍由截图事实决定）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)]$Manifest
    )
    try {
        $json = ($Manifest | ConvertTo-Json -Depth 6)
        [System.IO.File]::WriteAllText($Path, $json, (New-Object System.Text.UTF8Encoding($false)))
        return [pscustomobject]@{ Ok = $true; Path = $Path; Error = '' }
    }
    catch {
        return [pscustomobject]@{ Ok = $false; Path = $Path; Error = $_.Exception.Message }
    }
}

function Invoke-AutoScreenshot {
    [CmdletBinding()]
    param(
        [string]$Device,
        [string]$CliPath,
        [string]$ProjectDir,
        [string]$OutputDir,
        [string]$Pages = '',
        [switch]$ChangedOnly,
        # **仅发版冒烟语境**的显式授权位（#2078）：值 = 发版票号（纯数字）。给了它才走全仓逐页，
        # 且产物落 `docs/verification/device/<票号>/`。不给 ⇒ 本文件行为逐字不变（票 ③）。
        [string]$FullRepo = '',
        # 起跑档位，**仅在 -FullRepo 下生效**（⑥「先一页起跑通、再逐步扩」的入口）：>0 即本轮只跑全量清单的
        # 前 N 页，但 manifest 里的**全仓计划仍是 60 页**（累计对账不变），「还差几页」一直可算。
        # 不实现成「裁掉全仓清单」：那会让全仓计划跟着变小，分夜跑与 #2077 续跑就失去输入。
        [int]$RampPages = 0,
        [int]$MaxPages = 5,
        [int]$WaitSeconds = 3,
        # 导航落定的有界判据（见 Wait-NavSettled）：等满 MinSeconds 且相邻两次采样**内容一致**才算落定；
        # 到 TimeoutSeconds 仍未落定 ⇒ fail-closed 记 Skipped（不产截图，避免拿上一页当本次证据）。
        # StableMaxDiffPercent 是「内容一致」的宽容阈值（差异像素占比 %）—— 不得退回全帧 hash 相等。
        [int]$NavigateMinSeconds = 15,
        [int]$NavigateTimeoutSeconds = 420,
        [int]$PollSeconds = 5,
        # **一次** adb 调用的上限（#1560）：`-NavigateTimeoutSeconds` 是整轮导航的预算，本值管单次调用 ——
        # 两个调用点（`Wait-NavSettled` 的采样、下面那张页面截图）共用它。默认理由与现测数字见 `Invoke-BoundedAdbShot`。
        [int]$AdbCallTimeoutSeconds = 15,
        [double]$StableMaxDiffPercent = 0.5,
        # 本次运行的起点（issue #1158）：由调用方传入**同一个**值，好让步骤 7 的「本轮产物」过滤
        # 与步骤 6 这里的陈旧截图判据用**同一把尺子**（不各取一次 `Get-Date`，那样两个判据会漂）。
        # 不传 ⇒ 本函数自己取（旧行为，向后兼容）。
        [Nullable[datetime]]$RunStartedAt = $null
    )

    # ---------- -FullRepo 授权位校验（**任何副作用之前**）----------
    # 校验排在建目录、探测 adb、点 cli 之前：一个写错的票号不该在树里留下目录，更不该白烧一次编译。
    # 「不给 -FullRepo 留无票号通道」是票 ③ 的原话 ⇒ 只认纯数字票号。
    # 与 -Pages 互斥：一个是「按票号归档的全仓逐页」，一个是「人指定的几页」，两种语义混着给
    # 就没法回答「这一轮到底截了哪些页」⇒ 拒（拒得看得见，不静默取其一）。
    $fullRepoMode = -not [string]::IsNullOrWhiteSpace($FullRepo)
    # 全仓计划（全量）在此先给默认值，只在 -FullRepo 分支里被真正赋值：
    # 别在后面的「循环前」段再赋一次 @($targetPages) —— 那会把起跑档的 1 页覆盖成全仓计划（本会话踩过）。
    $fullRepoPlannedPages = @()
    if ($fullRepoMode -and $FullRepo -notmatch '^\d+$') {
        return [pscustomobject]@{
            Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @()
            Error = "-FullRepo 要的是**发版票号**（纯数字，如 -FullRepo 1864），收到 '$FullRepo' ⇒ 拒绝开始（不建目录、不碰设备）。"
        }
    }
    if ($RampPages -lt 0) {
        return [pscustomobject]@{
            Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @()
            Error = "-RampPages 不得为负（$RampPages）⇒ 拒绝开始。"
        }
    }
    if ($RampPages -ne 0 -and -not $fullRepoMode) {
        return [pscustomobject]@{
            Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @()
            Error = "-RampPages 只在 -FullRepo 下有意义（日常形态本就按改动页跑）⇒ 拒绝开始，不静默忽略。"
        }
    }
    if ($fullRepoMode -and $Pages) {
        return [pscustomobject]@{
            Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @()
            Error = "-FullRepo 与 -Pages 互斥（前者=按票号归档全仓逐页，后者=人指定的几页）⇒ 二选一，本次拒绝开始。"
        }
    }

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputDir) {
        # 全仓形态的产物**默认就落在票号目录**（归档主键 = 票号，与 device-capture.ps1 同一处口径）；
        # 日常形态逐字不变（仍是 .ci-verify 那棵树，随工作树回收消失）。
        $OutputDir = if ($fullRepoMode) {
            Join-Path $ProjectDir (Join-Path 'docs\verification\device' $FullRepo)
        } else {
            Join-Path $ProjectDir '.ci-verify\screenshots'
        }
    }

    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

    # 全仓形态的「已产清单」落点：与截图同目录，续跑（#2077）读的就是它。
    $fullRepoManifestPath = Join-Path $OutputDir 'fullrepo-manifest.json'

    # ---------- adb ----------
    # ⚠️ **解析器的唯一真源在 `lib/env-check.ps1`**（`Resolve-AdbExeLocal`）：本文件**不得**再自带候选集。
    #    2026-09-15 血账（#1027）：这里曾内联第二份（只查 `$env:ANDROID_SDK_ROOT` / `$env:ANDROID_HOME` /
    #    `Get-Command adb`），比 env-check 那份**少了 `D:\android-sdk\platform-tools\adb.exe` 兜底**
    #    ⇒ 本机（两个 env 都未设、adb 不在 PATH、adb 在 D 盘）步骤 2/5 能过、**步骤 6 恒报「找不到 adb.exe」**，
    #    而步骤 4/5 已真跑完并产生副作用（编译 + 部署），步骤 7–9 永远到不了 ——
    #    这正是 ADR-0008「门脚本共享载体」记的那类分叉：解析器被复制而不是复用，两份悄悄漂移。
    #    env-check.ps1 只含函数定义（dot-source 无副作用）；本模块**独立调用**时补加载，
    #    在 dev:finish 里它已在步骤 2 被 dot-source ⇒ 这里不会重复加载（与下面 `ConvertFrom-GitQuotedPath`
    #    同一套懒加载写法，故 `$PSScriptRoot` 同源指向 `scripts/lib`）。
    if (-not (Get-Command Resolve-AdbExeLocal -ErrorAction SilentlyContinue)) {
        . (Join-Path $PSScriptRoot 'env-check.ps1')
    }
    $adbExe = Resolve-AdbExeLocal
    if (-not $adbExe) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 adb.exe' }
    }

    # ---------- HBuilderX cli ----------
    if (-not $CliPath) { $CliPath = $env:HBuilderX_CLI }
    if (-not $CliPath) {
        $proc = $null
        try { $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1 } catch { }
        if ($proc) {
            try {
                $runningCli = Join-Path (Split-Path $proc.Path) 'cli.exe'
                if (Test-Path -LiteralPath $runningCli) { $CliPath = $runningCli }
            } catch { }
        }
    }
    if (-not $CliPath -or -not (Test-Path -LiteralPath $CliPath)) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 HBuilderX cli.exe' }
    }

    # ---------- pages.json（取全量清单，用于映射与校验）----------
    $pagesPath = Join-Path $ProjectDir 'pages.json'
    if (-not (Test-Path -LiteralPath $pagesPath)) {
        return [pscustomobject]@{ Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @(); Error = '找不到 pages.json' }
    }
    $pagesJson = Get-Content -LiteralPath $pagesPath -Raw | ConvertFrom-Json
    $allPages = @($pagesJson.pages | ForEach-Object { $_.path })

    # ---------- 1) 确定目标页集合 ----------
    $targetPages = @()

    if ($fullRepoMode) {
        # 授权位已确 ⇒ 全量清单直接来自 pages.json（含 subPackages），**不走 git 推导**、
        # 也**不受 -MaxPages 裁剪**（裁剪是日常 PR 的成本闸门，不是发版冒烟的判据）。
        # 这里用 Get-FullRepoPageList 而不是复用 $allPages：后者只含 pages 段，
        # 动它等于顺手改掉日常 PR 的推导射程（票 ③ 要的是逐字不变）。
        # 两个集合分开存 —— 这是本形态的判据骨架：
        #   $fullRepoPlannedPages = 全仓计划（恒为全量，累计对账与续跑的分母）
        #   $targetPages          = 本轮要跑的页（-RampPages 只裁这一份）
        $fullRepoPlannedPages = @(Get-FullRepoPageList -PagesJson $pagesJson)
        $targetPages = @(Select-FullRepoAttempted -PlannedPages $fullRepoPlannedPages -RampPages $RampPages)
        $plannedAll = @($fullRepoPlannedPages).Count
        if (@($targetPages).Count -lt $plannedAll) {
            $rampMsg = "[auto-screenshot] -FullRepo {0} 起跑档：本轮 {1}/{2} 页（全仓计划仍按 {2} 页对账，RampPages={3}）"
            Write-Host ($rampMsg -f $FullRepo, @($targetPages).Count, $plannedAll, $RampPages) -ForegroundColor Magenta
        } else {
            Write-Host "[auto-screenshot] -FullRepo $FullRepo ⇒ 全仓逐页 $($targetPages.Count) 页（不推导、不裁剪）" -ForegroundColor Yellow
        }
    }
    elseif ($Pages) {
        # 显式清单优先
        $targetPages = @($Pages -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    }
    else {
        # 默认（含 -ChangedOnly）：从 git diff 推导改动涉及的页面
        # ⚠️ `--relative` 不能省（2026-09-15 实测）：git 默认输出**仓库根相对**路径，而本项目整套源码位于
        #    `training-app/<中文目录名>/` 之下 ⇒ 路径前面挂着一串非 ASCII 段，下面的映射按 `^pages/` 锚定
        #    ⇒ 一条也匹配不上 ⇒ **推导恒为 0 页** ⇒ 步骤 6 必然报「无改动页面」并 exit 1
        #    ⇒ HashConflicts / TargetPages 永远取不到真值（这正是长跑不出来的那条验证债）。
        #    实测：临时改 pages/index/index.uvue，raw 是 "training-app/\345.../pages/index/index.uvue"，
        #    推导 = 0 页；加 --relative 后得到 `pages/index/index.uvue`，与下面的锚定同形状。
        # ⚠️ 解码同样不能省：`core.quotepath`（git 默认 true）把非 ASCII 路径整条加引号 + `\ooo` 转义
        #    ⇒ 末尾是 `uvue"` 而不是 `uvue`，后缀/锚定匹配一律静默失配（level-detect.ps1 踩过同一个陷阱）。
        if (-not (Get-Command ConvertFrom-GitQuotedPath -ErrorAction SilentlyContinue)) {
            # 解码器的**唯一真源**在 level-detect.ps1（纯函数定义、无副作用）；独立调用本模块时补加载，
            # 在 dev:finish 里它已在步骤 1 被 dot-source ⇒ 这里不会重复加载。
            . (Join-Path $PSScriptRoot 'level-detect.ps1')
        }
        $diffFiles = @()
        foreach ($gitArgs in @(@('diff', '--name-only', '--relative', 'HEAD'), @('diff', '--name-only', '--relative', 'origin/master...HEAD'))) {
            try {
                $raw = & git -C $ProjectDir @gitArgs 2>$null
                if ($raw) { $diffFiles += @($raw | Where-Object { $_ -and "$_".Trim() } | ForEach-Object { ConvertFrom-GitQuotedPath "$_".Trim() }) }
            }
            catch { }
        }
        $diffFiles = @($diffFiles | Select-Object -Unique)

        $derived = @()
        foreach ($f in $diffFiles) {
            # pages/<name>.uvue  →  pages/<name>
            if ($f -match '^pages/(.+)\.uvue$') {
                $candidate = "pages/$($Matches[1])"
                if ($allPages -contains $candidate) { $derived += $candidate }
            }
        }
        $targetPages = @($derived | Select-Object -Unique)
    }

    # ---------- 3) 0 页 ⇒ 明报并返回，绝不回退全量 ----------
    # -FullRepo 形态下这条**照样有效**，但方向相反且更硬：那一支的唯一入参是票号，
    # 页清单空 ⇒ 「全仓」这个前提 itself 不成立（pages.json 读空 / 结构变了），
    # 必须红着装不了事，而不是返回一个「0 页全仓截图」的绿。
    if ($targetPages.Count -eq 0) {
        if ($fullRepoMode) {
            return [pscustomobject]@{
                Ok = $false; Screenshots = @(); Skipped = @(); HashConflicts = @(); TargetPages = @()
                Error = "-FullRepo $FullRepo：pages.json 全量清单解析出 **0 页** ⇒ 无页可截，不返回绿（全仓形态没有「静默零产物」这条出口）。"
            }
        }
        $why = if ($Pages) { '指定的 -Pages 为空' } else { 'git diff 里没有 pages/**/*.uvue 改动' }
        return [pscustomobject]@{
            Ok           = $false
            Screenshots  = @()
            Skipped      = @()
            HashConflicts = @()
            TargetPages  = @()
            Error        = "无改动页面（$why）⇒ 不截图（**不回退到全量**）。要显式指定请用 -Pages 'pages/xxx/xxx'。"
        }
    }

    # ---------- 4) -MaxPages 兜底裁剪 ----------
    # 全仓形态**不裁剪**：票号就是授权，裁掉任何一页都让「全仓基线库」名不副实（决策 5）。
    $capped = @()
    if (-not $fullRepoMode -and $targetPages.Count -gt $MaxPages) {
        $capped = @($targetPages[$MaxPages..($targetPages.Count - 1)])
        $targetPages = @($targetPages[0..($MaxPages - 1)])
        Write-Host "[auto-screenshot] 目标页 $($targetPages.Count + $capped.Count) 个超过 -MaxPages=$MaxPages，裁掉：$($capped -join ', ')" -ForegroundColor Yellow
    }

    # ---------- 逐页导航 + 截图 ----------
    $screenshots = @()
    $skipped = @($capped)
    $hashConflicts = @()
    $staleShots = @()
    # 中转页（请求页被立刻跳到白名单落点）：**不产图、不算 Skipped、算「已交代」**。
    # 与 Skipped 的区别是处置不同：Skipped 要人查（切页/亮屏/通道），Redirected 是产品事实、无需处置。
    $redirected = @()
    # 白名单来源唯一：scripts/lib/auto-screenshot-redirects.json（见 Get-RedirectMap 的理由段）
    $redirectMap = Get-RedirectMap -Path (Join-Path $PSScriptRoot 'auto-screenshot-redirects.json')
    $seenHashes = @{}
    $prevHash = ''

    # 本次运行的起点：用于「截图必须是本次新落的」判据（见下方陈旧截图检查）。
    # ⚠️ 必须在**进入循环之前**取，否则每页各取一次会让判据退化成恒真。
    # 调用方给了就用它（issue #1158：步骤 7 的「本轮产物」过滤必须与这里同源）。
    $runStarted = if ($null -ne $RunStartedAt) { [datetime]$RunStartedAt } else { Get-Date }

    # 循环**之前**先落一次 manifest（仅全仓形态）：这样「设备没亮 / adb 通道坏 / 中途被 kill」这些
    # 一页都没截到的形态，归档里也留有「计划 N 页、已产 0 页」的凭据，而不是一个空目录让人猜。
    # ⑥ 断路腿要的「明报 INCOMPLETE、已产页不丢、零产物不冒充绿」全靠这份早落盘的清单兑现。
    if ($fullRepoMode) {
        $mfPre = Get-FullRepoManifestData -Ticket $FullRepo -PlannedPages $fullRepoPlannedPages -AttemptedPages $targetPages -RunStartedAt $runStarted -OutputDir $OutputDir
        $null = Write-FullRepoManifest -Path $fullRepoManifestPath -Manifest $mfPre
    }

    # 前置（fail-closed）：设备必须**亮屏**才能截图 —— 灭屏时 screencap 只会给全黑帧，
    # 而全黑帧天然「稳定」⇒ 会被当成本次证据（2026-09-15 实测踩到）。唤醒设备是**人**的动作，脚本不注入 input。
    # 单次调用有界（#1562）：这一处原先是**无上限**的同步等待，而它在每页之前跑 ⇒ 挂住时连「开始截屏」都到不了。
    # 「拿不到唤醒状态」有两种，判定相同（都不截）但**文案必须分开**——处置人不同：
    # 前者查 adb 通道（设备可能好着），后者是**人**去亮屏。混成一句会把通道问题读成没亮屏。
    $awake = Test-ScreenAwake -AdbExe $adbExe -Serial $Device -TimeoutSeconds $AdbCallTimeoutSeconds
    if (-not $awake.Ok) {
        # 灭屏这一支也要按**本轮形态**记名：全仓形态记拍平名，否则 manifest 里的 Skipped
        # 与截图文件名（以及 #2077 续跑要读的键）是两套名字，对账当场对不上。
        foreach ($p in $targetPages) {
            $skipped += $(if ($fullRepoMode) { Get-FullRepoShotName -Page $p } else { ($p -split '/')[-1] })
        }
        $awakeWhy = if ($awake.TimedOut) {
            "单次 adb 调用未在 $AdbCallTimeoutSeconds 秒内返回（adb 通道问题，不是没亮屏）"
        } else { "mWakefulness=$($awake.State)" }
        return [pscustomobject]@{
            Ok            = $false
            Screenshots   = @()
            Skipped       = $skipped
            HashConflicts = @()
            StaleShots    = @()
            TargetPages   = $targetPages
            Error         = "设备屏幕未唤醒（$awakeWhy）：灭屏时 screencap 只会得到全黑帧 ⇒ 拒绝截图（fail-closed）。请先唤醒设备（或打开「充电时保持唤醒」）再跑。"
            # 全仓形态的收口字段**这条早退也必须带**（#2078 现测：⑥ 的断路腿正是落到这一支）：
            # 少了它们，调用方在 Set-StrictMode 下读 $r.Verdict 直接抛 —— 「明报 INCOMPLETE」就报不出来。
            # 零产物 ⇒ Verdict=FAIL（零产物不冒充绿）；已产页仍留在归档目录里、manifest 已在循环前落过一次。
            FullRepo      = $(if ($fullRepoMode) { $FullRepo } else { '' })
            Verdict       = $(if ($fullRepoMode) { 'FAIL' } else { '' })
            Counts        = $null
            ManifestPath  = $(if ($fullRepoMode -and $fullRepoManifestPath) { $fullRepoManifestPath } else { '' })
            PlannedPages  = $(if ($fullRepoMode) { @($fullRepoPlannedPages) } else { @() })
            AttemptedPages = $(if ($fullRepoMode) { @($targetPages) } else { @() })
            Redirected    = @()
            PageSeconds   = @()
        }
    }

    # 全仓形态的逐页**时间账与心跳**（票 ④.2 / ④.4）：一轮约 3.9 小时，缺「现在第几页、这页花了多久、
    # 累计多少」就等于无人值守时**没有任何可判读的东西** —— 挂死与慢得看不出区别（#1560 那条血账的形状）。
    $fullRepoPageSeconds = @()
    $loopStarted = Get-Date
    $prevPageSeconds = 0

    $pageNum = 0
    foreach ($page in $targetPages) {
        $pageNum++
        # 文件名：日常形态按末段（**逐字不变**），全仓形态按整条路径拍平。
        # 不是风格问题：主树 60 页末段只有 59 个唯一值，按末段会少一张图、并把覆盖读成切页故障。
        $pageName = if ($fullRepoMode) { Get-FullRepoShotName -Page $page } else { ($page -split '/')[-1] }
        $outputFile = Join-Path $OutputDir "$pageName.png"
        $pageStarted = Get-Date

        Write-Host "[auto-screenshot] ($pageNum/$($targetPages.Count)) 导航: $page" -ForegroundColor Cyan

        try {
            # 切页：cli launch app-android --pagePath <页> --project <项目> [--deviceId <设备>]
            # ⚠️ **必须分离派发**（2026-09-15 实测血账）：不带 `--compile` 的真运行 cli **永不自己收口**
            #    —— 实测一个会话存活 18 分钟、CPU 累计 0.08 秒。先前这里是
            #    `$null = & $CliPath @launchArgs 2>&1 | Out-String`（前台同步等待、无超时）
            #    ⇒ 步骤 6 **永久挂住**：截图不落盘，HashConflicts / TargetPages 永远算不出来。
            #    现在与 hx-run.ps1 的 launch 步同一套做法（包装脚本 + UseShellExecute 新进程树）。
            $launchArgs = @('launch', 'app-android', '--pagePath', $page, '--project', $ProjectDir)
            if ($Device) { $launchArgs += @('--deviceId', $Device) }
            $navLogDir = Join-Path $ProjectDir '.ci-verify'
            New-Item -ItemType Directory -Force -Path $navLogDir | Out-Null
            $navLogFile = Join-Path $navLogDir 'auto-screenshot-nav.log'
            $nav = Start-NavLaunchDetached -CliArgs $launchArgs -CliExe $CliPath -LogDir $navLogDir -LogFile $navLogFile

            # 「导航是否落定」= 有界的**设备侧**判据（不靠退出码、不靠固定 sleep）：
            #   每页一次 launch 要走一遍 HBuilderX（编译 + 推送，分钟级），固定 3 秒必然截到旧画面。
            $probeFile = Join-Path $navLogDir 'nav-probe.png'
            $allowedLanding = @()
            if ($redirectMap.ContainsKey($page)) { $allowedLanding = @($redirectMap[$page]) }
            $settle = Wait-NavSettled -AdbExe $adbExe -Serial $Device -ProbeFile $probeFile `
                -NavOutFile $nav.OutFile -ExpectedPage $page -AllowedLandingPages $allowedLanding `
                -MinSeconds $NavigateMinSeconds -TimeoutSeconds $NavigateTimeoutSeconds -PollSeconds $PollSeconds `
                -CallTimeoutSeconds $AdbCallTimeoutSeconds -StableMaxDiffPercent $StableMaxDiffPercent
            if (-not $settle.Settled) {
                $skipped += $pageName
                Write-Host "  ❌ $pageName 导航未在 $NavigateTimeoutSeconds 秒内落定（$($settle.Reason)；launch 日志：$($nav.OutFile)）——不截这张图" -ForegroundColor Red
                continue
            }
            if ($settle.RedirectedTo) {
                # 中转页：画面是落点的画面 ⇒ 绝不能存成请求页的图（那正是「名不副实」的证据）
                $redirected += [pscustomobject]@{ Page = $page; Name = $pageName; To = [string]$settle.RedirectedTo; Seconds = [int]$settle.Seconds }
                Write-Host "  ↔ $pageName 是中转页：已进入白名单落点 $($settle.RedirectedTo)（不产图，记 Redirected；$($settle.Seconds)s）" -ForegroundColor Yellow
                continue
            }
            Write-Host "  · 导航已落定（$($settle.Seconds)s，$($settle.Reason)）" -ForegroundColor DarkGray

            Start-Sleep -Seconds $WaitSeconds

            # 截图（exec-out 保 PNG 字节流；**单次调用有界**，#1560）。
            # 旧写法是 `& cmd.exe /c "… > file" 2>&1 | Out-Null`：没有单次超时 ⇒ 挂住时既不产帧也不报错，
            # 而票面点名「只修采样循环等于把挂死从『等落定』挪到『等截图』」——这个调用点就是那第二个形状。
            # ⚠️ 先落**暂存名** `.part`（不是 `.png`），成功才改名归位。真链路现测（2026-10-07，强制挂死腿）证明
            #    被杀调用的残帧**可能删不掉**（子进程还握着句柄，删除与终止有竞争），而步骤 7 是按
            #    `-Filter '*.png'` + 本轮时间戳扫目录的（`screenshot-diff.ps1:235`）⇒ 半张图若直接落在证据目录，
            #    会被下一步当成本轮截图去比基线。票面那句「到点 ⇒ 该页 Skipped、**不产截图**」要靠命名兑现，
            #    不能靠删除成功。
            $partFile = "$outputFile.part"
            $shot = Invoke-BoundedAdbShot -AdbExe $adbExe -Serial $Device -OutFile $partFile -TimeoutSeconds $AdbCallTimeoutSeconds
            if ($shot.TimedOut) {
                # 走既有的「该页不计入证据」出口（调用方语义不变，只多点名）；`.part` 不进 `*.png` 射程。
                $skipped += $pageName
                Write-Host "  ❌ $pageName 的截图调用未在 $AdbCallTimeoutSeconds 秒内返回（$($shot.Error)）⇒ 不计入证据" -ForegroundColor Red
                Write-Host ("SHOT_CALL_TIMEOUT page={0} callBudgetSeconds={1}" -f $pageName, $AdbCallTimeoutSeconds) -ForegroundColor DarkGray
                continue
            }

            if (-not (Test-Path -LiteralPath $partFile) -or (Get-Item -LiteralPath $partFile).Length -eq 0) {
                $skipped += $pageName
                $shotWhy = if ($shot.ErrTail) { "（adb stderr: $($shot.ErrTail)）" } else { '' }
                Write-Host "  ⚠️ $pageName 截图失败$shotWhy" -ForegroundColor Yellow
                continue
            }
            # 归位：只有「调用返回了」且「拿到非空帧」的图才进证据目录
            Move-Item -LiteralPath $partFile -Destination $outputFile -Force

            # 反假绿：整帧**全黑 / 无内容** ⇒ 不是证据（灭屏、白屏、切页过渡帧都会这样）。
            # 2026-09-15 实测：17208 字节的全黑 PNG 一路通过「存在且非空 + 时间戳新 + hash 不变」三条判据。
            $blankInfo = Test-ScreenBlank -Path $outputFile
            if ($blankInfo.Blank) {
                $skipped += $pageName
                Write-Host "  ❌ $pageName 截图整帧全黑（min=$($blankInfo.Min) max=$($blankInfo.Max) mean=$($blankInfo.Mean)）⇒ 判无效，不计入证据" -ForegroundColor Red
                continue
            }

            # 反假绿：截图**必须是本次运行新落的**。
            # ⚠️ 为什么必须有这条：`$OutputDir` 默认 `.ci-verify\screenshots`，目录**不清理**，
            #    文件名按页名固定。若 adb 截图静默失败（写入 0 字节或没写），上面那条只查
            #    「文件存在且非空」——**上一次运行残留的同名 PNG 会让它照样通过**，
            #    于是把陈旧截图当成本次证据。时间戳判据把这种情形钉死。
            $shotWritten = (Get-Item -LiteralPath $outputFile).LastWriteTime
            if ($shotWritten -lt $runStarted) {
                $staleShots += $pageName
                $skipped += $pageName
                Write-Host "  ❌ $pageName 的截图时间戳早于本次运行起点（陈旧文件，疑似本次截图未真正落盘）" -ForegroundColor Red
                continue
            }

            $hash = (Get-FileHash -LiteralPath $outputFile -Algorithm SHA256).Hash

            # 反假绿：与上一页 hash 相同 ⇒ 切页没生效
            if ($prevHash -and $hash -eq $prevHash) {
                $hashConflicts += $pageName
                $skipped += $pageName
                Write-Host "  ❌ $pageName 的 hash 与上一页相同 ⇒ 切页未生效" -ForegroundColor Red
                continue
            }

            $seenHashes[$pageName] = $hash
            $prevHash = $hash
            $screenshots += $pageName
            $short = if ($hash.Length -ge 16) { $hash.Substring(0, 16) } else { $hash }
            Write-Host "  ✅ $pageName.png (sha256=$short…)" -ForegroundColor Green
        }
        catch {
            $skipped += $pageName
            Write-Host "  ⚠️ $pageName 异常: $($_.Exception.Message)" -ForegroundColor Yellow
        }
        finally {
            # 逐页时间账 + 心跳写在 finally：本循环有 6 条 continue 出口（未落定 / 调用超时 / 截图失败 /
            # 全黑帧 / 陈旧 / hash 冲突）加一条 catch，挨个补记账必漏一处。本会话实测 pwsh 7：
            # finally 在 continue、throw、正常路径**三条都走** ⇒ 一处覆盖全部出口。
            $pageSecs = [int]((Get-Date) - $pageStarted).TotalSeconds
            if ($fullRepoMode) {
                $rec = [pscustomobject]@{ Page = $page; Name = $pageName; Seconds = $pageSecs; Index = $pageNum }
                $fullRepoPageSeconds += $rec
                $cum = [int]((Get-Date) - $loopStarted).TotalSeconds
                $doneCount = @($fullRepoPageSeconds).Count
                $sumSecs = [int](($fullRepoPageSeconds | Measure-Object -Property Seconds -Sum).Sum)
                $avg = if ($doneCount -gt 0) { [int]($sumSecs / $doneCount) } else { $pageSecs }
                $remain = [int](($targetPages.Count - $pageNum) * $avg)
                # 剩余预估按本轮已有页的均值算（第 1 页用本页自身）；不给经验常数——常数正是票 ④.4 要拿实测换掉的东西
                $fmt = "  [心跳] {0}/{1} {2} 本页 {3}s（上一页 {4}s）累计 {5}s 剩约 {6}s"
                Write-Host ($fmt -f $pageNum, $targetPages.Count, $pageName, $pageSecs, $prevPageSeconds, $cum, $remain) -ForegroundColor DarkCyan
                $prevPageSeconds = $pageSecs
                $mfArgs = @{ Ticket = $FullRepo; PlannedPages = $fullRepoPlannedPages; AttemptedPages = $targetPages; Screenshots = $screenshots }
                $mfArgs.Skipped = $skipped
                $mfArgs.Redirected = $redirected
                $mfArgs.HashConflicts = $hashConflicts
                $mfArgs.StaleShots = $staleShots
                $mfArgs.PageSeconds = $fullRepoPageSeconds
                $mfArgs.RunStartedAt = $runStarted
                $mfArgs.OutputDir = $OutputDir
                $null = Write-FullRepoManifest -Path $fullRepoManifestPath -Manifest (Get-FullRepoManifestData @mfArgs)
            }
        }
    }

    # `Ok` 的判据**一字未改**（票 ③：日常形态逐字不变；全仓形态也不放宽）：
    # 零产物、有跳过、有 hash 冲突都算不通过。全仓形态只在这之上**多带一个 Verdict**，
    # 用来把「截到了但没截全」（INCOMPLETE，处置指向工具）与「一页都没截到」（FAIL，阻断）分开
    # —— 移动端 ADR-0043 决策 6 的三态语义，不是新发明的第四态。
    $allOk = ($screenshots.Count -gt 0 -and $skipped.Count -eq 0 -and $hashConflicts.Count -eq 0)
    $errorMsg = ''
    if ($hashConflicts.Count -gt 0) {
        $errorMsg = "hash 冲突（切页未生效）: $($hashConflicts -join ', ')"
    }
    elseif ($screenshots.Count -eq 0) {
        $errorMsg = '没有任何页面截图成功'
    }

    $fullRepoCounts = $null
    $verdict = ''
    if ($fullRepoMode) {
        $redirectedNamesFinal = @($redirected | ForEach-Object { [string]$_.Name })
        $fullRepoCounts = Get-FullRepoCounts -PlannedPages $targetPages -CapturedNames $screenshots -SkippedNames $skipped -RedirectedNames $redirectedNamesFinal
        $mfPost = Get-FullRepoManifestData -Ticket $FullRepo -PlannedPages $fullRepoPlannedPages -AttemptedPages $targetPages -Screenshots $screenshots `
            -Skipped $skipped -HashConflicts $hashConflicts -StaleShots $staleShots `
            -PageSeconds $fullRepoPageSeconds -RunStartedAt $runStarted -OutputDir $OutputDir -Redirected $redirected
        $null = Write-FullRepoManifest -Path $fullRepoManifestPath -Manifest $mfPost
        if ($allOk -and [bool]$fullRepoCounts.CountsMatch) { $verdict = 'PASS' }
        elseif (@($screenshots).Count -eq 0) { $verdict = 'FAIL' }
        else { $verdict = 'INCOMPLETE' }
        $totalSecs = [int]((Get-Date) - $loopStarted).TotalSeconds
        # 一行里同时给**两个分母**：attempted（本轮该产出的）与 repo（全仓计划）。
        # 起跑档下 verdict=PASS 只说明「本轮这几页成了」，不代表全仓基线建成 —— 所以
        # dirMissing（全仓还差哪几页）必须与它同行出现，否则 PASS 会被读成收口完成。
        $fmt = '[fullrepo] verdict={0} attempted={1} captured={2} skipped={3} missing={4} repo={5} dirMissing={6} 本轮机时={7}s manifest={8}'
        $dirMissingN = @($mfPost.DirMissing).Count
        Write-Host ($fmt -f $verdict, $fullRepoCounts.PlannedCount, $fullRepoCounts.CapturedCount, `
            $fullRepoCounts.SkippedCount, (@($fullRepoCounts.Missing) -join ','), `
            $mfPost.PlannedCount, $dirMissingN, $totalSecs, $fullRepoManifestPath) `
            -ForegroundColor $(if ($verdict -eq 'PASS') { 'Green' } else { 'Red' })
    }

    return [pscustomobject]@{
        Ok            = $allOk
        Screenshots   = $screenshots
        Skipped       = $skipped
        HashConflicts = $hashConflicts
        StaleShots    = $staleShots
        TargetPages   = $targetPages
        # 本次运行起点（issue #1158）：步骤 7 的「本轮产物」过滤要**同一个**值 —— 由本对象回传给调用方，
        # 不各自 `Get-Date`（两个判据各取一次会漂，先取的那次会把后落的截图误判成陈旧）。
        RunStartedAt  = $runStarted
        Error         = $errorMsg
        # ---------- -FullRepo 形态专有 ----------
        # 日常形态（不带 -FullRepo）这几个字段恒为 '' / $null / 空集：调用方照旧只读上面那六个，
        # 不会被新字段牵到（票 ③「逐字不变」也包括结论对象的形状不变）。
        # Verdict = 移动端 ADR-0043 决策 6 的三态（PASS / INCOMPLETE / FAIL），不是新发明的第四态。
        # Counts 是给 ⑥「页清单与产物计数相符」那条判据的**结构化依据** —— 不让人去数目录里的图。
        FullRepo      = $(if ($fullRepoMode) { $FullRepo } else { '' })
        Verdict       = $verdict
        Counts        = $fullRepoCounts
        ManifestPath  = $(if ($fullRepoMode) { $fullRepoManifestPath } else { '' })
        # PlannedPages = 全仓计划（恒为全量）；AttemptedPages = 本轮实际跑的页（起跑档是其前缀）。
        # 两个都给调用方，是因为「本轮 PASS」与「全仓齐了」是两个结论，不能靠一个数组猜。
        PlannedPages  = $(if ($fullRepoMode) { @($fullRepoPlannedPages) } else { @() })
        AttemptedPages = $(if ($fullRepoMode) { @($targetPages) } else { @() })
        # 中转页（请求页被立刻跳到白名单落点）：与 Captured **分开**给调用方 ——
        # 它是「已交代但没产图」，写进 Screenshots 就是假绿（本会话真机轮实测过这个坑）。
        Redirected    = @($redirected)
        PageSeconds   = @($fullRepoPageSeconds)
    }
}
