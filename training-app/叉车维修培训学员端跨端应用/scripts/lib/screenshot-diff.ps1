<#
.SYNOPSIS
    截图对比模块：当前截图与基线逐张对比，输出差异报告。

.DESCRIPTION
    对比方式（Q17，2026-09-18）：**像素差 + 阈值**为默认判据，MD5 退化为**回退路径**。
      - `scripts/lib/png-diff.mjs`（纯 Node、零依赖）逐像素比，默认阈值 0.5%（`-PixelThreshold`）。
      - 像素层**给不出结论**时（node 不在 / 非 PNG / 尺寸不同 / 不支持的 PNG 子集）⇒ **回退 MD5**
        并把 `Mode = 'md5'` 如实记进每条 diff 与返回对象 —— **不把「判不了」当「无变化」**。
      - `-Mode md5` 可强制只走 MD5（CI 无 node 的极端场景 / 对照实验用）。

    为什么不能只比 MD5：截图噪声是**必然**的（字体栅格化、抗锯齿、状态栏时钟/网速读数），
    MD5 只能回答「字节是否完全相同」⇒ 要么每页恒「有变化」，要么人为忽略它（那这层就白建）。
    判据选型的完整理由见 `scripts/lib/png-diff.mjs` 头部与 issue #1139。

    输出差异报告：无变化 / 有变化 / 新增 / 缺失。
    -UpdateBaseline 用当前截图覆盖基线。

.EXAMPLE
    . scripts/lib/screenshot-diff.ps1
    $result = Compare-ScreenshotBaseline -CurrentDir ".ci-verify/screenshots" -BaselineDir ".ci-verify/baseline"

.EXAMPLE
    # 只走 MD5（对照实验；证明像素层真的在起作用）
    $r = Compare-ScreenshotBaseline -Mode md5
#>

# dot-source 幂等：`screenshotDiffBehavior.test.js` 与本模块的调用方都会 dot-source
if (-not (Get-Command Get-PngDiffVerdict -ErrorAction SilentlyContinue)) {
    . (Join-Path $PSScriptRoot 'screenshot-gate.ps1')
}

function Test-IsPngFile {
    <#
    .SYNOPSIS
        文件头是不是 PNG 签名（8 字节）。

    .DESCRIPTION
        为什么需要它：把**非 PNG**的成对文件（既有守护 C1–C4 喂的就是 `[byte[]](1,2,3,4,5)`）
        直接丢给 `png-diff.mjs` 会走一条「必然 unsupported」的 node 子进程往返（约 50ms/张）。
        先验签名可以省掉这次往返，行为**完全不变**（仍然回退 MD5、仍然如实记 mode）。
        判据只在**都给不出结论**时才回退 —— 与「不把判不了当无变化」同一条纪律。
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$Path)

    try {
        $fs = [System.IO.File]::OpenRead($Path)
        try {
            if ($fs.Length -lt 8) { return $false }
            $sig = New-Object byte[] 8
            [void]$fs.Read($sig, 0, 8)
        }
        finally { $fs.Dispose() }
    }
    catch { return $false }

    $png = [byte[]](0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A)
    for ($i = 0; $i -lt 8; $i++) { if ($sig[$i] -ne $png[$i]) { return $false } }
    return $true
}

function Compare-ScreenshotFile {
    <#
    .SYNOPSIS
        单张截图 vs 其基线的判据（像素差优先、MD5 回退）。返回判定对象。

    .DESCRIPTION
        返回 `[pscustomobject]`：
          Status   '无变化' | '有变化' | '新增'      —— 文案契约沿用（D3 / C1–C4 / evidence-gen 都读它）
          Mode     'pixel' | 'md5'                   —— **本条**用的是哪条判据（如实记）
          Ratio    像素差比例（mode=pixel 时有效，否则为 $null）
          Different/Total、FallbackReason（回退原因，不回退时为空）
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$CurrentPath,
        [Parameter(Mandatory = $true)][string]$BaselinePath,
        [Parameter(Mandatory = $false)][string]$PngDiffScript,
        [double]$PixelThreshold = 0.005,
        [int]$IgnoreTopRows = 0,
        [int]$IgnoreBottomRows = 0,
        [ValidateSet('auto', 'md5', 'pixel')]
        [string]$Mode = 'auto'
    )

    if (-not (Test-Path -LiteralPath $BaselinePath)) {
        return [pscustomobject]@{
            Status = '新增'; Mode = 'none'; Ratio = $null; Different = $null; Total = $null; FallbackReason = ''
        }
    }

    # ---- 回退路径一：显式要求 MD5（-Mode md5）----
    if ($Mode -eq 'md5') {
        return (Compare-ScreenshotFileByMd5 -CurrentPath $CurrentPath -BaselinePath $BaselinePath -FallbackReason 'mode-md5')
    }

    # ---- 回退路径二：判据工具不存在 / 非 PNG（省一次注定失败的 node 往返）----
    if (-not $PngDiffScript -or -not (Test-Path -LiteralPath $PngDiffScript)) {
        return (Compare-ScreenshotFileByMd5 -CurrentPath $CurrentPath -BaselinePath $BaselinePath -FallbackReason 'png-diff-mjs-missing')
    }
    if (-not (Test-IsPngFile -Path $CurrentPath) -or -not (Test-IsPngFile -Path $BaselinePath)) {
        return (Compare-ScreenshotFileByMd5 -CurrentPath $CurrentPath -BaselinePath $BaselinePath -FallbackReason 'not-a-png')
    }

    # ---- 像素差（真实形态：spawn CLI，守的契约是 stdout JSON）----
    # 写成 [double]$PixelThreshold.ToString(...) 是为了**不受区域设置影响**（中文/德语环境下
    # `"$PixelThreshold"` 可能是 `0,005`，而 CLI 用 Number() 解析只会得到 0 —— 静默变成「零阈值」）。
    $thresholdText = $PixelThreshold.ToString('0.######', [System.Globalization.CultureInfo]::InvariantCulture)
    $outFile = [System.IO.Path]::GetTempFileName()
    $errFile = [System.IO.Path]::GetTempFileName()
    try {
        $cliArgs = @($PngDiffScript, '--a', $BaselinePath, '--b', $CurrentPath, '--threshold', $thresholdText)
        if ($IgnoreTopRows -gt 0) { $cliArgs += @('--ignore-top-rows', "$IgnoreTopRows") }
        if ($IgnoreBottomRows -gt 0) { $cliArgs += @('--ignore-bottom-rows', "$IgnoreBottomRows") }

        $json = ''
        $spawnReason = ''
        try {
            & node @cliArgs 1>$outFile 2>$errFile
            $json = (Get-Content -LiteralPath $outFile -Raw -ErrorAction SilentlyContinue)
        }
        catch {
            $spawnReason = 'node-spawn-failed'
        }

        $verdict = $null
        if (-not [string]::IsNullOrWhiteSpace($json)) {
            try { $verdict = $json | ConvertFrom-Json } catch { $verdict = $null }
        }

        if (-not $verdict -or ($verdict.ok -ne $true)) {
            $reason = if ($spawnReason) { $spawnReason } else { 'unsupported' }
            if ($verdict -and $verdict.reason) { $reason = "${reason}:$($verdict.reason)" }
            return (Compare-ScreenshotFileByMd5 -CurrentPath $CurrentPath -BaselinePath $BaselinePath -FallbackReason $reason)
        }

        $status = '无变化'
        if ([bool]$verdict.changed) { $status = '有变化' }
        return [pscustomobject]@{
            Status         = $status
            Mode           = 'pixel'
            Ratio          = $verdict.ratio
            Different      = $verdict.different
            Total          = $verdict.total
            FallbackReason = ''
        }
    }
    finally {
        Remove-Item -LiteralPath $outFile, $errFile -Force -ErrorAction SilentlyContinue
    }
}

function Compare-ScreenshotFileByMd5 {
    <#
    .SYNOPSIS
        回退判据：MD5 逐字节相等（**只在像素层给不出结论时**才允许到达这里）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$CurrentPath,
        [Parameter(Mandatory = $true)][string]$BaselinePath,
        [string]$FallbackReason = ''
    )

    $currentHash = (Get-FileHash -Path $CurrentPath -Algorithm MD5).Hash
    $baselineHash = (Get-FileHash -Path $BaselinePath -Algorithm MD5).Hash

    $status = '有变化'
    if ($currentHash -eq $baselineHash) { $status = '无变化' }

    return [pscustomobject]@{
        Status         = $status
        Mode           = 'md5'
        Ratio          = $null
        Different      = $null
        Total          = $null
        FallbackReason = $FallbackReason
    }
}

function Compare-ScreenshotBaseline {
    [CmdletBinding()]
    param(
        [string]$CurrentDir,
        [string]$BaselineDir,
        [string]$ProjectDir,
        [switch]$UpdateBaseline,
        # Q17（2026-09-18）：像素差阈值，默认 0.5%。截图噪声（字体栅格化 / 抗锯齿 / 状态栏读数）
        # 是**必然**的 ⇒ 无阈值等于每页恒红。0..1 之外没有意义，直接参数级拒绝（不静默钳位）。
        [ValidateRange(0.0, 1.0)]
        [double]$PixelThreshold = 0.005,
        # 顶部/底部忽略行数：真机建议取状态栏高度（时钟 / 网速读数是实时变化的）。
        [ValidateRange(0, 10000)]
        [int]$IgnoreTopRows = 0,
        [ValidateRange(0, 10000)]
        [int]$IgnoreBottomRows = 0,
        # 「本轮产物」的起点（issue #1158）。**给值即启用过滤**：**当前侧**早于该时刻的截图
        # 不算本轮产物 —— 目录从不清理、文件名按页名固定 ⇒ 上一轮失败运行的残留会污染「页数 / 变化数」。
        # ⚠️ **只过滤当前侧**（2026-09-18 修正）：基线是上一轮收口时的参考图，mtime **必然**早于本轮起点，
        #    按起点过滤基线会把每个有基线的页面全部跳过（详见 lib/screenshot-gate.ps1 的 Select-ThisRunShots）。
        # 判据的真源是调用方（`dev:finish` 与步骤 6 **同一个**运行起点）；不传 = 不过滤（向后兼容）。
        [Nullable[datetime]]$RunStartedAt = $null,
        # 强制只走 MD5（对照实验 / 无 node 的极端场景）
        [ValidateSet('auto', 'md5', 'pixel')]
        [string]$Mode = 'auto'
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $CurrentDir) {
        $CurrentDir = Join-Path $ProjectDir '.ci-verify\screenshots'
    }
    if (-not $BaselineDir) {
        $BaselineDir = Join-Path $ProjectDir '.ci-verify\baseline'
    }

    $pngDiffScript = Join-Path $PSScriptRoot 'png-diff.mjs'

    # 确保基线目录存在
    $baselineExists = Test-Path -LiteralPath $BaselineDir
    if (-not $baselineExists) {
        New-Item -ItemType Directory -Force -Path $BaselineDir | Out-Null
        Write-Host "[screenshot-diff] 基线目录不存在，已创建: $BaselineDir" -ForegroundColor Yellow
    }

    # 获取当前截图列表
    # ⚠️ 必须用 `@(...)` 包住：`Get-ChildItem` 只命中**一个**文件时会退化成**标量**，而调用方
    #    `dev-finish.ps1` 是以 `Set-StrictMode -Version Latest` 跑的 ⇒ 标量取 `.Count` 直接抛
    #    「在此对象上找不到属性 Count」。2026-09-15 真机实测（#1027 收尾）：基线目录不存在且
    #    **恰好一页**改动时，步骤 7 就崩在这里 —— 而「单页改动 + 首次运行」恰恰是最常见的形态。
    $currentFiles = @()
    if (Test-Path -LiteralPath $CurrentDir) {
        $currentFiles = @(Get-ChildItem -Path $CurrentDir -Filter '*.png' -File)
    }

    # 「本轮产物」过滤（issue #1158）：判据不在这里重写 —— 它是 `Select-ThisRunShots`（纯函数，与步骤 6
    # 的「截图时间戳必须晚于本次运行起点」同源），调用方把**同一个**运行起点传进来。
    # 跳过必须**可见**：静默丢弃 = 另一种假绿。
    $skippedStale = @()
    $inScope = @()
    $baselineFiles = @()
    if (Test-Path -LiteralPath $BaselineDir) {
        $baselineFiles = @(Get-ChildItem -Path $BaselineDir -Filter '*.png' -File)
    }
    if ($null -ne $RunStartedAt) {
        $scope = Select-ThisRunShots -RunStartedAt $RunStartedAt -CurrentDir $CurrentDir -BaselineDir $BaselineDir
        $skippedStale = @($scope.Skipped)
        $inScope = @($scope.InScope)
        $currentFiles = @($currentFiles | Where-Object { $inScope -contains $_.Name })
        foreach ($s in $skippedStale) {
            Write-Host "[screenshot-diff] ⏭️ $($s.Name) — 跳过（非本轮产物：$($s.Side) 侧早于本轮运行起点）" -ForegroundColor DarkYellow
        }
    }

    if ($currentFiles.Count -eq 0) {
        # 形状必须与正常路径**一致**：调用方以 StrictMode 跑，读一个不存在的属性会**直接抛**
        # （2026-09-18 实测：漏了 Fallback 就让整个探测脚本挂在这条早退分支上）。
        $errText = '无当前截图可对比'
        if ($skippedStale.Count -gt 0) {
            $errText = "无本轮截图可对比（目录里有 $($skippedStale.Count) 张陈旧残留，已跳过）"
        }
        return [pscustomobject]@{
            Diff         = @()
            ChangedCount = 0
            NewBaseline  = (-not $baselineExists)
            Error        = $errText
            Mode         = 'none'
            Detail       = @()
            PixelRan     = $false
            Fallback     = @()
            SkippedStale = $skippedStale
        }
    }

    $diff = @()
    $detail = @()
    $changedCount = 0
    $pixelRan = $false
    $fallbackReasons = @()

    foreach ($file in $currentFiles) {
        $baselineFile = Join-Path $BaselineDir $file.Name
        $verdict = Compare-ScreenshotFile -CurrentPath $file.FullName -BaselinePath $baselineFile `
            -PngDiffScript $pngDiffScript -PixelThreshold $PixelThreshold `
            -IgnoreTopRows $IgnoreTopRows -IgnoreBottomRows $IgnoreBottomRows -Mode $Mode

        $status = [string]$verdict.Status
        if ($status -ne '无变化') { $changedCount++ }
        if ($verdict.Mode -eq 'pixel') { $pixelRan = $true }
        elseif ($verdict.Mode -eq 'md5' -and -not [string]::IsNullOrWhiteSpace([string]$verdict.FallbackReason)) {
            $fallbackReasons += [string]$verdict.FallbackReason
        }

        $diff += "$($file.Name):$status"
        $detail += $verdict

        $icon = if ($status -eq '无变化') { '📄' } elseif ($status -eq '新增') { '🆕' } else { '⚠️' }
        $suffix = ''
        if ($verdict.Mode -eq 'pixel' -and $verdict.Ratio -ne $null) {
            $suffix = "（像素差 $([math]::Round([double]$verdict.Ratio * 100, 3))% / 阈值 $([math]::Round($PixelThreshold * 100, 3))%）"
        }
        elseif ($verdict.Mode -eq 'md5') {
            $suffix = "（MD5 回退：$($verdict.FallbackReason)）"
        }
        Write-Host "[screenshot-diff] $icon $($file.Name) — $status$suffix" -ForegroundColor $(if ($status -eq '无变化') { 'Gray' } else { 'Yellow' })
    }

    # 检查基线中有但当前没有的文件（缺失）
    # ⚠️ 过滤生效时（#1158），**陈旧的一侧不参与判据、也不替对方造出「缺失」**：
    #    上一轮失败运行留下的残留基线图，不得把本轮报成「缺失」（那正是 ADR-0008 记的那半岛）。
    if (Test-Path -LiteralPath $BaselineDir) {
        # 同上的 `@(...)` 理由：单文件时若退化成标量，后续任何 `.Count` 都会在 StrictMode 下抛错
        $baselineFiles = @(Get-ChildItem -Path $BaselineDir -Filter '*.png' -File)
        if ($null -ne $RunStartedAt) {
            $baselineFiles = @($baselineFiles | Where-Object { $inScope -contains $_.Name })
        }
        foreach ($bFile in $baselineFiles) {
            $currentFile = Join-Path $CurrentDir $bFile.Name
            if (-not (Test-Path -LiteralPath $currentFile)) {
                $diff += "$($bFile.Name):缺失"
                $changedCount++
                Write-Host "[screenshot-diff] ❌ $($bFile.Name) — 缺失" -ForegroundColor Red
            }
        }
    }

    # 更新基线
    if ($UpdateBaseline) {
        New-Item -ItemType Directory -Force -Path $BaselineDir | Out-Null
        foreach ($file in $currentFiles) {
            $dest = Join-Path $BaselineDir $file.Name
            Copy-Item -LiteralPath $file.FullName -Destination $dest -Force
        }
        Write-Host "[screenshot-diff] ✅ 基线已更新" -ForegroundColor Green
    }

    # NewBaseline 的口径（2026-09-18 收紧）：**基线目录里一张 PNG 都没有**才算首次。
    #   旧口径只看「目录存在与否」⇒ 上一轮未带 -UpdateBaseline 时会留下一个**空目录**，
    #   下一轮就自称「不是首次」而把全部页面报成「新增」（ADR-0008:419 记的就是这个坑）。
    $baselinePngCount = 0
    if (Test-Path -LiteralPath $BaselineDir) {
        $baselinePngCount = @(Get-ChildItem -Path $BaselineDir -Filter '*.png' -File).Count
    }
    $isNewBaseline = ($baselinePngCount -eq 0)

    $modeLabel = 'none'
    if ($pixelRan) { $modeLabel = 'pixel' } elseif ($diff.Count -gt 0) { $modeLabel = 'md5' }

    return [pscustomobject]@{
        Diff         = $diff
        ChangedCount = $changedCount
        NewBaseline  = $isNewBaseline
        Error        = ''
        Mode         = $modeLabel
        Detail       = $detail
        PixelRan     = $pixelRan
        Fallback     = @($fallbackReasons | Select-Object -Unique)
        # 被跳过的陈旧残留（可见，不静默）—— 调用方与 evidence 都能引用
        SkippedStale = $skippedStale
    }
}
