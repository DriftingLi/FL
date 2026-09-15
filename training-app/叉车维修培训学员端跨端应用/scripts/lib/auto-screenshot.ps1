<#
.SYNOPSIS
    自动截图模块：只截**本次改动涉及的页面**，真翻页、带反假绿判据。

.DESCRIPTION
    **Q3 口径（2026-09-14 用户裁定）**：只截改动页面 + `-Pages` 参数。
    为什么要限范围：每页一次 `cli launch app-android --pagePath <页>` 都要过一遍 HBuilderX
    （编译 + 推送），本仓 `pages.json` 有 50 页 ⇒ 全量截图成本不可接受
    （`device-capture.ps1` 早已把「逐页 launch 成本高」标为待裁定，本次裁定即此）。

    **页面集合的确定顺序**：
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
        且画面**连续两次采样一致**；到 `-NavigateTimeoutSeconds`（默认 420s）仍未落定 ⇒ 该页记 `Skipped`
        且**不产截图**（fail-closed：宁可可核地明报，也不拿上一页/白屏当本次证据）。
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
    #>
    param([string]$AdbExe, [string]$Serial)
    $out = ''
    try { $out = (& $AdbExe -s $Serial shell dumpsys power 2>$null | Out-String) } catch { }
    if ($out -notmatch 'mWakefulness=(\w+)') { return [pscustomobject]@{ Ok = $false; State = 'unknown' } }
    return [pscustomobject]@{ Ok = ($Matches[1] -eq 'Awake'); State = $Matches[1] }
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

function Wait-NavSettled {
    <#
      有界的「导航已落定」判据（**设备侧事实**，不靠退出码、不靠固定 sleep）。
      **2026-09-15 修订（血账）**：旧版只要求「画面连续两次采样一致」——实测被**全黑帧**骗过：
      灭屏 / 编译期间的黑屏天然稳定，16 秒就「落定」，两张黑图当成了本次证据。
      现在三条**同时**满足才算落定：
        ① 应用日志里出现**目标页**的页面进入行（`进入页面:"<目标页>"`）—— 页身份的唯一设备侧证据；
        ② 采样帧**不是全黑 / 无内容**（`Test-ScreenBlank`）；
        ③ 已等满 -MinSeconds 且画面连续两次采样一致。
      到 -TimeoutSeconds 仍不满足 ⇒ Settled=$false，调用方记 Skipped 且**不产截图**（fail-closed）。
      日志里进的是**别的页**时，Reason 会点名「请求页 X，实际进入 Y」—— 这就是 ADR-0008 要的**页身份**判据。
    #>
    param(
        [string]$AdbExe,
        [string]$Serial,
        [string]$ProbeFile,
        [string]$NavOutFile,
        [string]$ExpectedPage,
        [int]$MinSeconds = 15,
        [int]$TimeoutSeconds = 420,
        [int]$PollSeconds = 5
    )
    $start = Get-Date
    $prev = ''
    $stable = 0
    $sampled = 0
    $entered = ''
    $blankSeen = 0
    while ($true) {
        Start-Sleep -Seconds $PollSeconds
        $elapsed = [int]((Get-Date) - $start).TotalSeconds
        $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $AdbExe, $Serial, $ProbeFile
        & cmd.exe /c $cmd 2>&1 | Out-Null
        $blankNow = $false
        if ((Test-Path -LiteralPath $ProbeFile) -and (Get-Item -LiteralPath $ProbeFile).Length -gt 0) {
            $sampled++
            $blankNow = (Test-ScreenBlank -Path $ProbeFile).Blank
            if ($blankNow) { $blankSeen++ }
            $h = (Get-FileHash -LiteralPath $ProbeFile -Algorithm SHA256).Hash
            if ($h -eq $prev) { $stable++ } else { $stable = 0 }
            $prev = $h
        }
        $enter = Get-NavEnteredPage -NavOutFile $NavOutFile
        if ($enter) { $entered = $enter }

        if ($entered -and $entered -eq $ExpectedPage -and -not $blankNow -and $elapsed -ge $MinSeconds -and $stable -ge 1) {
            return [pscustomobject]@{ Settled = $true; Seconds = $elapsed; Samples = $sampled; EnteredPage = $entered; Reason = '已进入目标页且画面稳定（连续两次采样一致）' }
        }
        if ($elapsed -ge $TimeoutSeconds) {
            $why = if ($entered -and $entered -ne $ExpectedPage) {
                "请求页 $ExpectedPage，日志里最后进入的是 $entered（页身份不符）"
            }
            elseif ($sampled -gt 0 -and $blankSeen -ge $sampled) {
                "到上限 $TimeoutSeconds 秒画面始终是全黑（采样 $sampled 次）——设备可能灭屏"
            }
            elseif (-not $entered) {
                "到上限 $TimeoutSeconds 秒日志里始终没有页面进入行（launch 未真正跑起来）"
            }
            else {
                "到上限 $TimeoutSeconds 秒未同时满足「进入目标页 + 非全黑 + 画面稳定」"
            }
            return [pscustomobject]@{ Settled = $false; Seconds = $elapsed; Samples = $sampled; EnteredPage = $entered; Reason = $why }
        }
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
        [int]$MaxPages = 5,
        [int]$WaitSeconds = 3,
        # 导航落定的有界判据（见 Wait-NavSettled）：等满 MinSeconds 且画面连续两次采样一致才算落定；
        # 到 TimeoutSeconds 仍未落定 ⇒ fail-closed 记 Skipped（不产截图，避免拿上一页当本次证据）。
        [int]$NavigateMinSeconds = 15,
        [int]$NavigateTimeoutSeconds = 420,
        [int]$PollSeconds = 5
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputDir) {
        $OutputDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }

    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

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

    if ($Pages) {
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
    if ($targetPages.Count -eq 0) {
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
    $capped = @()
    if ($targetPages.Count -gt $MaxPages) {
        $capped = @($targetPages[$MaxPages..($targetPages.Count - 1)])
        $targetPages = @($targetPages[0..($MaxPages - 1)])
        Write-Host "[auto-screenshot] 目标页 $($targetPages.Count + $capped.Count) 个超过 -MaxPages=$MaxPages，裁掉：$($capped -join ', ')" -ForegroundColor Yellow
    }

    # ---------- 逐页导航 + 截图 ----------
    $screenshots = @()
    $skipped = @($capped)
    $hashConflicts = @()
    $staleShots = @()
    $seenHashes = @{}
    $prevHash = ''

    # 本次运行的起点：用于「截图必须是本次新落的」判据（见下方陈旧截图检查）。
    # ⚠️ 必须在**进入循环之前**取，否则每页各取一次会让判据退化成恒真。
    $runStarted = Get-Date

    # 前置（fail-closed）：设备必须**亮屏**才能截图 —— 灭屏时 screencap 只会给全黑帧，
    # 而全黑帧天然「稳定」⇒ 会被当成本次证据（2026-09-15 实测踩到）。唤醒设备是**人**的动作，脚本不注入 input。
    $awake = Test-ScreenAwake -AdbExe $adbExe -Serial $Device
    if (-not $awake.Ok) {
        foreach ($p in $targetPages) { $skipped += ($p -split '/')[-1] }
        return [pscustomobject]@{
            Ok            = $false
            Screenshots   = @()
            Skipped       = $skipped
            HashConflicts = @()
            StaleShots    = @()
            TargetPages   = $targetPages
            Error         = "设备屏幕未唤醒（mWakefulness=$($awake.State)）：灭屏时 screencap 只会得到全黑帧 ⇒ 拒绝截图（fail-closed）。请先唤醒设备（或打开「充电时保持唤醒」）再跑。"
        }
    }

    $pageNum = 0
    foreach ($page in $targetPages) {
        $pageNum++
        $pageName = ($page -split '/')[-1]
        $outputFile = Join-Path $OutputDir "$pageName.png"

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
            $settle = Wait-NavSettled -AdbExe $adbExe -Serial $Device -ProbeFile $probeFile `
                -NavOutFile $nav.OutFile -ExpectedPage $page `
                -MinSeconds $NavigateMinSeconds -TimeoutSeconds $NavigateTimeoutSeconds -PollSeconds $PollSeconds
            if (-not $settle.Settled) {
                $skipped += $pageName
                Write-Host "  ❌ $pageName 导航未在 $NavigateTimeoutSeconds 秒内落定（$($settle.Reason)；launch 日志：$($nav.OutFile)）——不截这张图" -ForegroundColor Red
                continue
            }
            Write-Host "  · 导航已落定（$($settle.Seconds)s，$($settle.Reason)）" -ForegroundColor DarkGray

            Start-Sleep -Seconds $WaitSeconds

            # 截图（exec-out 保 PNG 字节流）
            $cmd = '"{0}" -s {1} exec-out screencap -p > "{2}"' -f $adbExe, $Device, $outputFile
            & cmd.exe /c $cmd 2>&1 | Out-Null

            if (-not (Test-Path -LiteralPath $outputFile) -or (Get-Item -LiteralPath $outputFile).Length -eq 0) {
                $skipped += $pageName
                Write-Host "  ⚠️ $pageName 截图失败" -ForegroundColor Yellow
                continue
            }

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
    }

    $allOk = ($screenshots.Count -gt 0 -and $skipped.Count -eq 0 -and $hashConflicts.Count -eq 0)
    $errorMsg = ''
    if ($hashConflicts.Count -gt 0) {
        $errorMsg = "hash 冲突（切页未生效）: $($hashConflicts -join ', ')"
    }
    elseif ($screenshots.Count -eq 0) {
        $errorMsg = '没有任何页面截图成功'
    }

    return [pscustomobject]@{
        Ok            = $allOk
        Screenshots   = $screenshots
        Skipped       = $skipped
        HashConflicts = $hashConflicts
        StaleShots    = $staleShots
        TargetPages   = $targetPages
        Error         = $errorMsg
    }
}
