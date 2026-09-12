<#
.SYNOPSIS
    移动端验收门 ② 的载体：微信开发者工具无报错（**半自动门**，agent 可执行并附证据；合并仍由人）。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md；来源 spike issue #883（全链路实测，≈4.5 分钟/次）。
    2026-09-12 修订：② 从**人工门**降为**半自动门** —— 与 ④a 同款口径（#870 裁定 B）：
    agent 会话可执行本脚本并把结果贴成 sha 绑定评论；**「执行人」栏由人签收、合并仍由人执行**。

    全链路（每一步都无人工介入，#883 实测）：
      cli open → cli project open --path <项目>（HBuilderX 只认「已导入项目」，这一步是无人值守导入入口）
      → cli publish mp-weixin --project <项目>（构建**并自己拉起**开发者工具打开项目）
      → cli.bat close（清残留自动化会话）→ cli.bat auto --auto-port <Port> --trust-project（开自动化端口）
      → node scripts/mp-weixin-probe.mjs（miniprogram-automator 连上：读 pageStack / console / exceptions + 截图）

    **三条实测坑位（勿删，ADR-0008 已录）**：
      1. `pageStack` 为空会让**所有 page 级 API** 报同一个误导弹错
         `Cannot destructure property 'rawPath' of 't.getPageMetaByWebviewId(...)' as it is null`
         —— 根因是自动化会话里没有页面被打开，不是版本不兼容。处置 = 先 `cli.bat close` 再 `cli.bat auto`
         （本脚本把这一步写死），并把「pageStack 非空」作为**前置断言**（探针里判 exit 2 环境不可用）。
      2. `mp.screenshot()` 挂在 **MiniProgram** 上，不在 Page 上（`page.screenshot is not a function`），
         且不返回内容 ⇒ 产物只能靠落盘 PNG 取。
      3. `page.$()` 元素级断言在 0.12.1 + Stable v2.01.2510290 组合下**不可用**（挂起 15s 超时）
         ⇒ 本门只做 page 级导航 + console/exception 判定 + 整页截图，**不得**加元素级断言。

    **判成败一律解析输出，绝不看退出码**：HBuilderX CLI 失败时退出码恒为 0（如实测「项目不存在，请先导入」仍返回 0），
    开发者工具 `cli.bat` 与探针的退出码同样不作为门结论。

    **端口监听面的风险（照实记，未做加固）**：`cli.bat auto --auto-port <Port>` 的监听地址由开发者工具决定，
    实测 `Get-NetTCPConnection` 显示绑定在通配地址（`::`）而非仅 `127.0.0.1` ⇒ **自动化端口在局域网内可被访问**。
    本机为开发/CI 前置机时风险可接受；在**共享网络**上跑本门应先确认防火墙。本脚本无法从 CLI 侧收窄监听面
    （`cli auto -h` 无绑定地址参数），故只在此声明，不假装已处理。

    **非等价声明（勿删）**：② ≠ ① 真机门，也 ≠ ④b 云打包门。
      ② 只证明「小程序端在开发者工具里打开入口页 + 被改页时 console 无 error、无 exception」；
      **挡不住**的类别（#883 逐条实测）：`content://` 上传、生物识别门控（开发者工具明确回
      `checkIsSupportSoterAuthentication:fail … 请使用真机进行开发`）、第三方 SDK 回调、
      真机性能/内存/ANR/厂商 ROM 差异 ⇒ 这些仍只能靠 **① 真机门** 与发版前的全量逐页冒烟兜底。

    运行前提（硬前提）：HBuilderX CLI 靠与主程序的**本地 IPC**；受限沙箱下会报「与主程序的连接已中断」，
    须以全访问权限执行（同 ④a）。开发者工具需已登录（`cli.bat islogin` → `{"login":true}`），
    登录态过期时需人补扫一次码。

    退出码：0 = ② 通过；1 = 门未过（console error / exception / 页面未打开 / 截图缺失）；2 = 环境不可用（缺 CLI / 连不上 / 超时）。

.PARAMETER Project
    项目根目录。缺省为本脚本上一级目录（scripts/ 位于项目根下）。
    注意：HBuilderX 按**项目名（目录名）**解析，同名目录会被误命中 ⇒ 用唯一目录名跑本门（#883 坑位）。

.PARAMETER HBuilderX
    HBuilderX 安装根（需含 cli.exe 与 plugins/）。缺省按「正在运行的 HBuilderX → $env:HBuilderX_HOME → 常见安装位置」探测。

.PARAMETER Cli
    cli.exe 路径。**显式给出即权威**：不可用直接 exit 2，不再回退探测。

.PARAMETER DevToolsCli
    微信开发者工具 cli.bat 路径。缺省按常见安装位置探测。

.PARAMETER AppId
    期望的微信小程序 appid。**默认真实 appid**（与 manifest.json 的 mp-weixin.appid 同值）。
    用途有二：① 作为**门的前置断言**——校验构建产物 project.config.json 落盘的 appid 与之一致；
    ② 传给 `cli publish mp-weixin --appid <AppId>`。实测（#883 + 本次复测）：该参数对**构建产物无影响**，
    产物 appid 一律取自 manifest.json 的 `mp-weixin.appid`；manifest 缺该项时落成 `touristappid`（游客 appid）。
    传空串（-AppId ''）可跳过该断言。

.PARAMETER Port
    自动化端口（默认 9420）。传给 `cli.bat auto --auto-port <Port>`。

.PARAMETER Routes
    待验页面路由（逗号分隔）。**第一个 = 入口页**（用 reLaunch 打开），其余按 navigateTo 打开。
    缺省 `pages/index/index,pages/login/login`（与 #883 实测口径一致：入口页 + 一个被改页）。

.PARAMETER SkipBuild
    跳过 `cli publish mp-weixin`（构建）。适用于已有新鲜产物、只想重跑自动化判定的场合。
    注意：跳过构建后开发者工具里打开的仍是上一次的产物，门结论只对那份产物成立。

.PARAMETER PostToPr
    > 0 时，**仅在门通过（exit 0）分支**把结果贴成 PR 评论（② 门结果免手抄，与 ④ 门同款）。
    评论正文严格形如 `<!-- gate-evidence:② -->` + 「② 微信开发者工具无报错（半自动，agent 执行）」
    + `commit: <HEAD sha>` + 结论（含 MP_WEIXIN_RESULT）与产物路径 + 复现命令；
    `.github/workflows/pr-evidence.yml` 只认「带该标记且 sha 与 PR head 相等」的评论，不校真伪。
    gh 不可用/取不到 sha 时只打印警告，**不影响门的结论**。

.EXAMPLE
    npm run build:mp-weixin-check
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -SkipBuild
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -Routes 'pages/index/index,pages/login/login'
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -PostToPr 884
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$HBuilderX,
    [string]$Cli,
    [string]$DevToolsCli,
    [string]$AppId = 'wx38c3e31b16a7ced0',
    [int]$Port = 9420,
    [string]$Routes = 'pages/index/index,pages/login/login',
    [switch]$SkipBuild,
    [int]$PublishTimeoutSeconds = 900,
    [int]$AutoTimeoutSeconds = 180,
    [int]$PortWaitSeconds = 90,
    [string]$Module = 'mp-weixin',
    [switch]$NoArchive,
    [int]$HxWaitSeconds = 600,
    [switch]$HxNoWait,
    [int]$ProbeTimeoutSeconds = 180,
    [int]$ProbeAttempts = 6,
    [int]$ProbeRetryDelaySeconds = 10,
    [int]$PostToPr = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$DistRelative = 'unpackage\dist\build\mp-weixin'
$ProbeRelative = 'scripts\mp-weixin-probe.mjs'
$WxDevToolsPatterns = @(
    'E:\微信web开发者工具\cli.bat',
    'D:\微信web开发者工具\cli.bat',
    'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat',
    'C:\Program Files\Tencent\微信web开发者工具\cli.bat',
    "$env:LOCALAPPDATA\微信web开发者工具\cli.bat"
)
$script:closed = $false

function Test-PeHeader {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $fs = [System.IO.File]::OpenRead($Path)
        try { $buf = New-Object byte[] 2; $read = $fs.Read($buf, 0, 2) } finally { $fs.Dispose() }
    } catch { return $false }
    # 'MZ'：E:\HBuilderX 那份残留副本前 8 字节全为 00（文件损坏），必须排除
    return ($read -eq 2 -and $buf[0] -eq 0x4D -and $buf[1] -eq 0x5A)
}

function Resolve-HBuilderXRoot {
    param([string]$Explicit, [string]$ExplicitCli)
    if ($ExplicitCli) {
        if (-not (Test-PeHeader -Path $ExplicitCli)) { return $null }
        return (Split-Path (Resolve-Path -LiteralPath $ExplicitCli).Path -Parent)
    }
    if ($Explicit) {
        $probe = Join-Path $Explicit 'cli.exe'
        if (-not (Test-PeHeader -Path $probe)) { return $null }
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($proc) {
            $candidate = Split-Path $proc.Path -Parent
            if (Test-PeHeader -Path (Join-Path $candidate 'cli.exe')) { return $candidate }
        }
    } catch { }
    $candidates = @()
    if ($env:HBuilderX_HOME) { $candidates += $env:HBuilderX_HOME }
    foreach ($pattern in @(
            'D:\软件\HBuilderX*\HBuilderX', 'D:\HBuilderX*\HBuilderX', 'E:\HBuilderX*\HBuilderX',
            'C:\Program Files\HBuilderX*\HBuilderX', 'C:\Program Files\HBuilderX', "$env:LOCALAPPDATA\HBuilderX")) {
        $candidates += (Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | ForEach-Object { $_.FullName })
    }
    foreach ($c in $candidates) {
        if (Test-PeHeader -Path (Join-Path $c 'cli.exe')) { return (Resolve-Path -LiteralPath $c).Path }
    }
    return $null
}

function Resolve-DevToolsCli {
    param([string]$Explicit)
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit -PathType Leaf) { return (Resolve-Path -LiteralPath $Explicit).Path }
        return $null
    }
    foreach ($p in $WxDevToolsPatterns) {
        if (Test-Path -LiteralPath $p -PathType Leaf) { return (Resolve-Path -LiteralPath $p).Path }
    }
    return $null
}

# 统一进程入口：超时兜底 + UTF-8 解码。**不返回退出码作为判据**（只在日志里留痕）。
function Invoke-Process {
    param([string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds, [string]$Tag, [string]$WorkingDirectory)
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = if ($WorkingDirectory) { $WorkingDirectory } else { $script:workDir }
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8
    foreach ($a in $Arguments) { [void]$psi.ArgumentList.Add($a) }
    Write-Host ">>> [$Tag] $FilePath $($Arguments -join ' ')"
    $p = [System.Diagnostics.Process]::Start($psi)
    $stdout = $p.StandardOutput.ReadToEndAsync()
    $stderr = $p.StandardError.ReadToEndAsync()
    $exited = $p.WaitForExit($TimeoutSeconds * 1000)
    if (-not $exited) {
        try { $p.Kill($true) } catch { }
        return @{ Output = "[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true }
    }
    return @{ Output = ($stdout.Result + $stderr.Result); ExitCode = $p.ExitCode; TimedOut = $false }
}

# ---------- 项目与日志 ----------
if (-not $Project) { $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
$Project = (Resolve-Path -LiteralPath $Project).Path
$script:workDir = $Project
if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}
$logDir = Join-Path $Project '.ci-verify'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logPath = Join-Path $logDir 'mp-weixin.log'
$logRelative = '.ci-verify/mp-weixin.log'
Set-Content -LiteralPath $logPath -Encoding utf8 -Value @(
    "# mp-weixin-check (②) 开始 $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
    "# project = $Project",
    "# 期望 appid = $AppId ; 自动化端口 = $Port ; 路由 = $Routes ; SkipBuild = $([bool]$SkipBuild)",
    "# 判成败只看输出：HBuilderX CLI / cli.bat / 探针的退出码一律不作门结论"
)
function Write-Log { param([string]$Text) Add-Content -LiteralPath $logPath -Value $Text -Encoding utf8 }

$dist = Join-Path $Project $DistRelative
$probePath = Join-Path $Project $ProbeRelative
if (-not (Test-Path -LiteralPath $probePath)) {
    Write-Host "[error] 缺探针脚本：$probePath" -ForegroundColor Red
    Write-Log 'MP_WEIXIN_RESULT errors=env reason=no-probe'
    exit 2
}
$routeList = @($Routes -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($routeList.Count -eq 0) {
    Write-Host '[error] -Routes 为空：至少给一个入口页路由' -ForegroundColor Red
    Write-Log 'MP_WEIXIN_RESULT errors=env reason=no-routes'
    exit 2
}
$entryUrl = $routeList[0]

# ---------- 环境面 ----------
$hbxRoot = Resolve-HBuilderXRoot -Explicit $HBuilderX -ExplicitCli $Cli
$cliExe = if ($hbxRoot) { Join-Path $hbxRoot 'cli.exe' } else { $null }
$devTools = Resolve-DevToolsCli -Explicit $DevToolsCli
if (-not $SkipBuild) {
    if (-not $hbxRoot) {
        Write-Host '[error] 找不到可用的 HBuilderX（需含 cli.exe 与 plugins/）。用 -HBuilderX <安装根> 或 -Cli <cli.exe> 指定。' -ForegroundColor Red
        Write-Log 'MP_WEIXIN_RESULT errors=env reason=hbuilderx-not-found'
        exit 2
    }
} elseif ($Cli -and -not $hbxRoot) {
    Write-Host "[error] 指定的 cli.exe 不可用（不存在或 PE 头损坏）：$Cli" -ForegroundColor Red
    Write-Log 'MP_WEIXIN_RESULT errors=env reason=cli-unusable'
    exit 2
}
if (-not $devTools) {
    Write-Host '[error] 找不到微信开发者工具 cli.bat。用 -DevToolsCli <路径> 指定。' -ForegroundColor Red
    Write-Log 'MP_WEIXIN_RESULT errors=env reason=devtools-not-found'
    exit 2
}
$nodeExe = (Get-Command node -ErrorAction SilentlyContinue | Select-Object -First 1).Source
if (-not $nodeExe) {
    Write-Host '[error] 找不到 node（跑探针需要 Node.js + miniprogram-automator）。' -ForegroundColor Red
    Write-Log 'MP_WEIXIN_RESULT errors=env reason=node-not-found'
    exit 2
}

# 端口占用前置检查：残留会话会让 cli.bat auto 静默复用旧会话 ⇒ pageStack 空（坑位 1）
$occupied = @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
if ($occupied.Count -gt 0) {
    Write-Host ">>> 端口 $Port 已被占用（疑似残留自动化会话），先 close 清理" -ForegroundColor Yellow
    Write-Log "port $Port already listening by pid(s): $(($occupied.OwningProcess) -join ',')"
}

# ---------- 收尾：cli.bat close 必须在 finally 路径里执行，避免端口/会话残留 ----------
function Invoke-DevToolsClose {
    if ($script:closed) { return }
    $script:closed = $true
    if (-not $devTools) { return }
    try {
        $r = Invoke-Process -FilePath $devTools -Arguments @('close', '--project', $dist) -TimeoutSeconds 120 -Tag 'devtools-close'
        Write-Log "`n>>> devtools-close`n$($r.Output)"
        Write-Host $r.Output
    } catch {
        Write-Host "[warn] cli.bat close 失败（不影响门结论）：$_" -ForegroundColor Yellow
    }
}

# ---------- P1：门通过时把结果贴成「sha 绑定」的 PR 评论（② 门结果免手抄）----------
function Get-HeadSha {
    param([string]$ProjectDir)
    $attempts = @()
    if ($ProjectDir) { $attempts += , @('-C', $ProjectDir, 'rev-parse', 'HEAD') }
    $attempts += , @('rev-parse', 'HEAD')
    foreach ($a in $attempts) {
        try {
            $out = & git @a 2>$null | Select-Object -First 1
            if ($out -and "$out".Trim()) { return "$out".Trim() }
        } catch { }
    }
    return ''
}

function Publish-GateComment {
    param(
        [int]$PrNumber,
        [string]$ResultLine,
        [string]$LogRelative,
        [string]$ShotRelative,
        [string]$ReproCommand,
        [string[]]$ArchivedRel = @(),
        [string[]]$ArchiveNotes = @()
    )
    $sha = Get-HeadSha -ProjectDir $Project
    if (-not $sha) {
        Write-Host '[warn] 取不到 HEAD sha（git 不可用或不在仓库里）：跳过贴 PR 评论，门结论不受影响。' -ForegroundColor Yellow
        return
    }
    $short = $sha.Substring(0, [Math]::Min(7, $sha.Length))
    # 正文格式被 .github/workflows/pr-evidence.yml 认（标记 ② + commit sha），改动前先看那边的注释
    $archLines = @()
    if (@($ArchivedRel).Count -gt 0) {
        $archLines += '- 入库截图（仓库内相对路径，GitHub 可点开预览）：'
        foreach ($r in @($ArchivedRel)) { $archLines += ('  - ``' + $r + '``') }
    } else {
        $archLines += '- 入库截图：（无 —— 见脚本输出的 [archive] 说明，未入库不影响门结论）'
    }
    if (@($ArchiveNotes).Count -gt 0) { $archLines += ('- 入库说明：' + (@($ArchiveNotes) -join '；')) }
    $body = @(
        '<!-- gate-evidence:② -->',
        '**② 微信开发者工具无报错（半自动，agent 执行）**',
        "- commit: $sha",
        "- 结论（含产物）：``$ResultLine``；本地截图 ``$ShotRelative``；日志 ``$LogRelative``",
        '- 非等价声明：② ≠ ① 真机门，也 ≠ ④b 云打包门（content:// / 生物识别 / 第三方 SDK / 真机性能 挡不住）。'
    ) + $archLines + @(
        "- 复现：``$ReproCommand``"
    )
    $body = $body -join "`n"
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        Write-Host '[warn] 找不到 gh CLI：跳过贴 PR 评论，门结论不受影响（结果见上面日志）。' -ForegroundColor Yellow
        return
    }
    try {
        $out = (& gh pr comment $PrNumber --body $body 2>&1 | Out-String)
        if ($out -match 'github\.com/') {
            Write-Host "✅ 已贴 PR #$PrNumber 的 ② 门评论（sha 绑定 $short）。" -ForegroundColor Green
        } else {
            Write-Host "[warn] 贴 PR #$PrNumber 评论疑似失败（门结论不受影响）：$($out.Trim())" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "[warn] 贴 PR 评论失败（门结论不受影响）：$_" -ForegroundColor Yellow
    }
}
# ---------- 截图入库（P2：② 的截图自动压缩提交到 PR 分支）----------
# 约束（维护者 2026-09-12 追加）：优先 WebP q75（探测 cwebp / ffmpeg / magick，择一），没有就退 JPEG q75
# （.NET System.Drawing，Windows 自带）；宽度上限 720px（等比、不放大）；每 PR ≤10 张、单张 ≤150KB、合计 ≤1.5MB；
# 超限先降质 → 再缩尺 → 再减图（减图必须在评论里写明省了哪几页）。
# **不得让门因此失败**：任何一步出错只打印警告 + 给出待执行的 git 命令。
$script:ArchiveMaxCount = 10
$script:ArchiveMaxBytes = 150KB
$script:ArchiveMaxTotal = 1.5MB
$script:ArchiveMaxWidth = 720
$script:ArchiveFail = $null

function Get-ImageCodec { return [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' } | Select-Object -First 1 }

function Get-WebpEncoder {
    foreach ($t in @('cwebp', 'ffmpeg', 'magick')) {
        $c = Get-Command $t -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($c) { return @{ Name = $t; Path = $c.Source } }
    }
    return $null
}

function Save-Jpeg {
    param($Bitmap, [string]$Path, [int]$Quality)
    $codec = Get-ImageCodec
    $eps = [System.Drawing.Imaging.EncoderParameters]::new(1)
    $eps.Param[0] = [System.Drawing.Imaging.EncoderParameter]::new([System.Drawing.Imaging.Encoder]::Quality, [int64]$Quality)
    $bmp2 = [System.Drawing.Bitmap]::new($Bitmap.Width, $Bitmap.Height, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp2)
    $g.Clear([System.Drawing.Color]::White)   # JPEG 无 alpha：先把透明底填白，免得出现黑块
    $g.DrawImage($Bitmap, 0, 0, $Bitmap.Width, $Bitmap.Height)
    $g.Dispose()
    try { $bmp2.Save($Path, $codec, $eps) } finally { $bmp2.Dispose(); $eps.Dispose() }
}

function Invoke-WebpEncode {
    param([string]$Tool, [string]$EncoderPath, [string]$PngPath, [string]$OutPath, [int]$Quality)
    switch ($Tool) {
        'cwebp' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @('-q', "$Quality", '-quiet', $PngPath, '-o', $OutPath) -TimeoutSeconds 120 -Tag 'cwebp' }
        'ffmpeg' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @('-y', '-loglevel', 'error', '-i', $PngPath, '-quality', "$Quality", $OutPath) -TimeoutSeconds 120 -Tag 'ffmpeg' }
        'magick' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @($PngPath, '-quality', "$Quality", $OutPath) -TimeoutSeconds 120 -Tag 'magick' }
    }
    return (Test-Path -LiteralPath $OutPath)
}

function Export-ArchiveImage {
    param([string]$SourcePng, [string]$OutBase, [int]$MaxWidth, $Encoder, [int]$Quality)
    # 返回 @{ Path, Bytes, Width, Height, Format } 或 $null
    $src = $null
    try { $src = [System.Drawing.Image]::FromFile($SourcePng) } catch { return $null }
    try {
        $w = $src.Width; $h = $src.Height
        if ($w -gt $MaxWidth) { $h = [int][Math]::Round($h * ($MaxWidth / [double]$w)); $w = $MaxWidth }
        $scaled = [System.Drawing.Bitmap]::new($w, $h, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
        $g = [System.Drawing.Graphics]::FromImage($scaled)
        $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $g.Clear([System.Drawing.Color]::White)
        $g.DrawImage($src, 0, 0, $w, $h)
        $g.Dispose()
        $out = "$OutBase.jpg"; $format = 'jpeg'
        try {
            if ($Encoder) {
                $tmpPng = "$OutBase.tmp.png"
                $scaled.Save($tmpPng, [System.Drawing.Imaging.ImageFormat]::Png)
                $out = "$OutBase.webp"
                if (Invoke-WebpEncode -Tool $Encoder.Name -EncoderPath $Encoder.Path -PngPath $tmpPng -OutPath $out -Quality $Quality) {
                    $format = 'webp'
                } else {
                    $out = "$OutBase.jpg"; $format = 'jpeg'
                    Save-Jpeg -Bitmap $scaled -Path $out -Quality $Quality
                }
                Remove-Item -LiteralPath $tmpPng -Force -ErrorAction SilentlyContinue
            } else {
                Save-Jpeg -Bitmap $scaled -Path $out -Quality $Quality
            }
        } finally { $scaled.Dispose() }
        if (-not (Test-Path -LiteralPath $out)) { return $null }
        return @{ Path = $out; Bytes = (Get-Item -LiteralPath $out).Length; Width = $w; Height = $h; Format = $format }
    } finally { $src.Dispose() }
}

function Publish-ScreenshotArchive {
    param([int]$PrNumber, [string]$Module, [string]$ShotDir, $ProbeJson, [string]$RepoRoot)
    # 目标：docs/verification/<模块>/<PR号>/<页名>-after.<ext>；仅当「当前分支 = 该 PR 的 head」
    # 且「工作树除本次改动外无其他改动」时才提交；其余情况只警告 + 给命令。
    $notes = [System.Collections.Generic.List[string]]::new()
    $cmdLines = [System.Collections.Generic.List[string]]::new()
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { return @{ Notes = @('未入库：找不到 gh CLI'); Commands = @() } }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { return @{ Notes = @('未入库：找不到 git'); Commands = @() } }

    $root = $RepoRoot
    $branch = ''
    try { $branch = (& git -C $root rev-parse --abbrev-ref HEAD 2>$null | Select-Object -First 1) } catch { }
    $dirty = @()
    try { $dirty = @(& git -C $root status --porcelain 2>$null | Where-Object { $_ -and $_.Trim() }) } catch { }

    $prJson = $null
    try { $prJson = (& gh pr view $PrNumber --json headRefName,state 2>$null | Out-String | ConvertFrom-Json) } catch { }
    if (-not $prJson) { return @{ Notes = @("未入库：取不到 PR #$PrNumber 的信息（gh 不可用/无权限）"); Commands = @() } }
    if ($prJson.state -ne 'OPEN') { return @{ Notes = @("未入库：PR #$PrNumber 状态为 $($prJson.state)（非 OPEN）"); Commands = @() } }
    if ($branch -ne $prJson.headRefName) {
        return @{ Notes = @("未入库：当前分支 '$branch' ≠ PR #$PrNumber 的 head 分支 '$($prJson.headRefName)'（避免把截图提到错的分支）"); Commands = @() }
    }
    $allowed = @("docs/verification/$Module/$PrNumber/")
    $blocking = @($dirty | Where-Object { $p = ($_ -replace '^..\s+', '').Trim('"'); -not ($allowed | Where-Object { $p.StartsWith($_) }) })
    if ($blocking.Count -gt 0) {
        return @{ Notes = @("未入库：工作树有本次之外的改动（$($blocking.Count) 项），为免夹带不做提交"); Commands = @() }
    }

    $module = if ($Module) { $Module } else { 'mp-weixin' }
    $destDir = Join-Path $root ("docs/verification/$module/$PrNumber" -replace '/', '\')
    New-Item -ItemType Directory -Force -Path $destDir | Out-Null
    $encoder = Get-WebpEncoder
    if (-not $encoder) { $notes.Add('本机无 cwebp / ffmpeg / magick：按纪律退到 JPEG q75（System.Drawing）') }
    else { $notes.Add("WebP 编码器：$($encoder.Name)") }

    $shots = @($ProbeJson.shots | Where-Object { $_.isPng -and (Test-Path -LiteralPath $_.path) })
    # 超限处置顺序：先降质 → 再缩尺 → 再减图（减图写进评论）
    $plan = @(
        @{ MaxWidth = $script:ArchiveMaxWidth; Quality = 75 },
        @{ MaxWidth = $script:ArchiveMaxWidth; Quality = 55 },
        @{ MaxWidth = 560; Quality = 60 },
        @{ MaxWidth = 420; Quality = 55 }
    )
    $made = @()
    $skipped = @()
    foreach ($p in $plan) {
        if ($made.Count -ge [Math]::Min($shots.Count, $script:ArchiveMaxCount)) { break }
        Remove-Item -LiteralPath (Join-Path $destDir '*') -Force -ErrorAction SilentlyContinue
        $made = @()
        foreach ($s in $shots) {
            if ($made.Count -ge $script:ArchiveMaxCount) { break }
            $base = [System.IO.Path]::GetFileNameWithoutExtension($s.name)
            $outBase = Join-Path $destDir "$base-after"
            $r = Export-ArchiveImage -SourcePng $s.path -OutBase $outBase -MaxWidth $p.MaxWidth -Encoder $encoder -Quality $p.Quality
            if (-not $r) { continue }
            if ($r.Bytes -gt $script:ArchiveMaxBytes) { Remove-Item -LiteralPath $r.Path -Force -ErrorAction SilentlyContinue; continue }
            $made += $r
        }
        $total = ($made | Measure-Object -Property Bytes -Sum).Sum
        if ($total -le $script:ArchiveMaxTotal) { break }
    }
    # 仍超上限的图（按字节从大到小丢），并记下省了哪几页
    while ((($made | Measure-Object -Property Bytes -Sum).Sum) -gt $script:ArchiveMaxTotal -and $made.Count -gt 1) {
        $biggest = $made | Sort-Object -Property Bytes -Descending | Select-Object -First 1
        Remove-Item -LiteralPath $biggest.Path -Force -ErrorAction SilentlyContinue
        $skipped += ([System.IO.Path]::GetFileName($biggest.Path) + "（超合计上限）")
        $made = @($made | Where-Object { $_.Path -ne $biggest.Path })
    }
    if ($made.Count -eq 0) { return @{ Notes = @('未入库：压缩后没有任何图满足体积上限'); Commands = @() } }
    if ($shots.Count -gt $script:ArchiveMaxCount) {
        $skipped += ($shots[$script:ArchiveMaxCount..($shots.Count - 1)] | ForEach-Object { $_.name })
    }

    $rel = $made | ForEach-Object { ('docs/verification/' + $module + '/' + $PrNumber + '/' + [System.IO.Path]::GetFileName($_.Path)) }
    foreach ($m in $made) {
        $notes.Add(("入库图：{0} {1}x{2} {3} {4}B" -f [System.IO.Path]::GetFileName($m.Path), $m.Width, $m.Height, $m.Format, $m.Bytes))
    }
    if ($skipped.Count -gt 0) { $notes.Add('省了这些页（体积/数量上限）：' + ($skipped -join '、')) }

    # 提交（只 add 本次入库的目录）
    try {
        & git -C $root add -- ("docs/verification/$module/$PrNumber") 2>$null | Out-Null
        & git -C $root commit -m "docs(verification): ② 微信开发者工具门截图（PR #$PrNumber）" --no-verify 2>&1 | Out-Null
        $sha2 = (& git -C $root rev-parse HEAD 2>$null | Select-Object -First 1)
        $notes.Add("已提交入库：$($sha2.Substring(0,7))（未用 [skip ci]：CI 会为新 sha 重跑，PR head 仍需挂在成功 run 上）")
    } catch {
        $notes.Add("入库提交失败：$_")
        $cmdLines.Add("git add docs/verification/$module/$PrNumber")
        $cmdLines.Add("git commit -m `"docs(verification): ② 门截图（PR #$PrNumber）`"")
    }
    return @{ Notes = $notes; Commands = $cmdLines; Rel = $rel }
}

# ---------- 主流程 ----------
$exitCode = 2
try {
    # HBuilderX 忙检测（单实例串行资源，ADR-0008 坑位段）：-SkipBuild 下不接主程序
    if (-not $SkipBuild) {
        . (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
        $hx = Wait-HxFree -CliExe $cliExe -TimeoutSeconds $HxWaitSeconds -NoWait:$HxNoWait -LogPath $logPath
    }
    Write-Host "proj  = $Project"
    Write-Host "dist  = $dist"
    Write-Host "cli   = $cliExe"
    Write-Host "wxcli = $devTools"
    Write-Host "log   = $logPath"

    if (-not $SkipBuild) {
        # 0) **源 manifest 的 appid 前置断言（fail-closed）** —— 实测坑位（2026-09-12）：
        #    HBuilderX 在**首次导入 / 编译项目时会回写工作树的 manifest.json，把 mp-weixin.appid 置为 null**
        #    （实测 mtime 与 publish 启动同秒），于是产物 project.config.json 落成 touristappid，
        #    而 git 工作树里的 manifest 同时被改脏 —— 门会在「游客 appid」下静默跑通。
        #    ⚠️ 这条不止影响本门：manifest.json 是「运行时面」文件、正是 pr-evidence 的判据来源，
        #    任何 HBuilderX 门（④a / ④c / ②）在共享工作树里跑都可能静默改脏它 ⇒ 跑完请 `git status` 核对。
        #    故这里在动 CLI **之前**先校验源 manifest，不匹配即判门不过（而不是拿游客 appid 出个绿）。
        $manifestPath = Join-Path $Project 'manifest.json'
        $manifestRaw = Get-Content -LiteralPath $manifestPath -Raw
        $srcAppId = ''
        $mm = [regex]::Match($manifestRaw, '(?s)"mp-weixin"\s*:\s*\{.*?"appid"\s*:\s*"([^"]*)"')
        if ($mm.Success) { $srcAppId = $mm.Groups[1].Value }
        Write-Log "源 manifest mp-weixin.appid = '$srcAppId'（期望 '$AppId'）"
        Write-Host "源 manifest mp-weixin.appid = $srcAppId（期望 $AppId）"
        if ($AppId -and $srcAppId -ne $AppId) {
            Write-Host "[error] 源 manifest 的 mp-weixin.appid 与期望不一致：manifest='$srcAppId' 期望='$AppId'" -ForegroundColor Red
            Write-Host '        这正是 HBuilderX 回写 manifest.json 把 appid 置 null 的痕迹（2026-09-12 实测）；' -ForegroundColor Red
            Write-Host '        产物 appid 只会取自 manifest（CLI 的 --appid 不改产物）⇒ 此时跑出来的门是「游客 appid 下的门」，不能算数。' -ForegroundColor Red
            Write-Host "        处置：先 `git checkout -- manifest.json` 还原真实 appid，再重跑本门。" -ForegroundColor Red
            Write-Log "MP_WEIXIN_RESULT errors=1 stage=manifest-appid manifest=$srcAppId expected=$AppId"
            exit 1
        }

        # 1) 唤醒主程序 + 无人值守导入（HBuilderX 只认「已导入项目」，publish 未导入会报「项目不存在，请先导入」）
        foreach ($step in @(
                @{ Tag = 'cli-open'; Args = @('open') },
                @{ Tag = 'project-open'; Args = @('project', 'open', '--path', $Project) })) {
            $r = Invoke-Process -FilePath $cliExe -Arguments $step.Args -TimeoutSeconds $PublishTimeoutSeconds -Tag $step.Tag
            Write-Log "`n>>> $($step.Tag)`n$($r.Output)"
            Write-Host $r.Output
            if ($r.Output -match '与主程序的连接已中断|启动超时') {
                Write-Host '[error] HBuilderX CLI 连不上主程序：受限/沙箱会话会阻断 CLI↔主程序的本地 IPC。' -ForegroundColor Red
                Write-Log 'MP_WEIXIN_RESULT errors=env reason=cli-ipc-blocked'
                exit 2
            }
            if ($r.TimedOut) {
                Write-Host "[error] $($step.Tag) 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
                Write-Log 'MP_WEIXIN_RESULT errors=env reason=timeout'
                exit 2
            }
        }

        # 2) 构建 mp-weixin —— HBuilderX 构建成功后**自己**拉起开发者工具并打开项目（#883 实测）
        $publishArgs = @('publish', 'mp-weixin', '--project', $Project)
        if ($AppId) { $publishArgs += @('--appid', $AppId) }
        $pub = Invoke-Process -FilePath $cliExe -Arguments $publishArgs -TimeoutSeconds $PublishTimeoutSeconds -Tag 'publish-mp-weixin'
        Write-Log "`n>>> publish-mp-weixin`n$($pub.Output)"
        Write-Host $pub.Output
        if ($pub.Output -match '与主程序的连接已中断') {
            Write-Host '[error] HBuilderX CLI 连不上主程序（见日志）。' -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=cli-ipc-blocked'
            exit 2
        }
        if ($pub.TimedOut) {
            Write-Host "[error] publish 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=timeout'
            exit 2
        }
        # 成败一律解析输出（CLI 退出码不可用）
        if ($pub.Output -notmatch '导出微信小程序成功|编译成功') {
            Write-Host '[error] publish 未报「导出微信小程序成功」（见日志）：先解决构建失败，再跑 ②。' -ForegroundColor Red
            Write-Host '        提示：HBuilderX 按**项目名（目录名）**解析，同名目录会被误命中 ⇒ 请用唯一目录名（#883 坑位）。' -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=1 stage=publish reason=no-artifact'
            exit 1
        }
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 产物不完整（缺 app.json）：$dist" -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=1 stage=publish reason=incomplete-artifact'
            exit 1
        }
    } else {
        Write-Log "`n>>> -SkipBuild：跳过 publish，直接用现有产物 $dist"
        Write-Host ">>> -SkipBuild：跳过 publish（用现有产物；门结论只对这份产物成立）" -ForegroundColor Yellow
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 没有可用产物（缺 $dist\app.json）：去掉 -SkipBuild 或先跑一次构建。" -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=no-dist'
            exit 2
        }
    }

    # 3) 产物 appid 前置断言 —— 实测：产物 appid 一律取自 manifest.json 的 mp-weixin.appid；
    #    manifest 缺该项时落成 touristappid（游客 appid）。CLI 的 --appid **不改**产物。
    $productAppId = ''
    $projConfig = Join-Path $dist 'project.config.json'
    if (Test-Path -LiteralPath $projConfig) {
        $m = [regex]::Match((Get-Content -LiteralPath $projConfig -Raw), '"appid"\s*:\s*"([^"]*)"')
        if ($m.Success) { $productAppId = $m.Groups[1].Value }
    }
    Write-Log "产物 appid = '$productAppId'（期望 '$AppId'）"
    Write-Host "产物 appid = $productAppId（期望 $AppId）"
    if ($AppId -and $productAppId -ne $AppId) {
        Write-Host "[error] 产物 appid 与期望不一致：产物=$productAppId 期望=$AppId" -ForegroundColor Red
        Write-Host '        appid 来源是 manifest.json 的 mp-weixin.appid（不是本脚本的 -AppId，也不是 CLI 的 --appid）；' -ForegroundColor Red
        Write-Host '        产物落成 touristappid = manifest 的 mp-weixin.appid 为 null/缺失（游客态）。' -ForegroundColor Red
        Write-Log "MP_WEIXIN_RESULT errors=1 stage=appid product=$productAppId expected=$AppId"
        exit 1
    }

    # 4) 清残留自动化会话（坑位 1：不 close 会撞 pageStack 空 ⇒ page 级 API 全废且报误导错）
    Invoke-DevToolsClose

    # 5) 开自动化端口（无人值守，不需要人在 GUI 里点任何开关）
    #    ⚠️ 2026-09-12 实测最要紧的一条：auto **必须跑完**（它会派生子进程去起自动化服务；
    #    中途 kill 掉 auto 会让端口永远不监听），且跑完后端口是**延迟出现**的 ⇒ 之后必须轮询等待。
    $auto = Invoke-Process -FilePath $devTools -Arguments @('auto', '--project', $dist, '--auto-port', "$Port", '--trust-project') `
        -TimeoutSeconds $AutoTimeoutSeconds -Tag 'devtools-auto'
    Write-Log "`n>>> devtools-auto`n$($auto.Output)"
    Write-Host $auto.Output
    if ($auto.TimedOut) {
        Write-Host "[error] cli.bat auto 超时（$AutoTimeoutSeconds 秒）" -ForegroundColor Red
        Write-Log 'MP_WEIXIN_RESULT errors=env reason=auto-timeout'
        exit 2
    }
    $usingAppId = ''
    $ma = [regex]::Match($auto.Output, 'Using AppID:\s*(\S+)')
    if ($ma.Success) { $usingAppId = $ma.Groups[1].Value }
    Write-Log "cli auto 回显 AppID = '$usingAppId'"
    Write-Host "cli auto 回显 AppID = $usingAppId"
    if ($AppId -and $usingAppId -and $usingAppId -ne $AppId) {
        Write-Host "[error] cli.bat auto 使用的 AppID（$usingAppId）与期望（$AppId）不一致。" -ForegroundColor Red
        Write-Log "MP_WEIXIN_RESULT errors=1 stage=auto-appid using=$usingAppId expected=$AppId"
        exit 1
    }

    # 端口延迟出现：轮询到 listening（最长 $PortWaitSeconds 秒）
    $listening = $false
    for ($i = 0; $i -lt $PortWaitSeconds; $i++) {
        if (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue).Count -gt 0) { $listening = $true; break }
        Start-Sleep -Seconds 1
    }
    $addrs = ''
    if ($listening) {
        $addrs = (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) |
            ForEach-Object { "$($_.LocalAddress):$($_.LocalPort)" }) -join ','
    }
    Write-Log "端口 $Port listening=$listening addrs=$addrs"
    Write-Host "端口 $Port listening=$listening addrs=$addrs"
    if (-not $listening) {
        Write-Host "[error] 自动化端口 $Port 未监听：cli.bat auto 未真正生效。" -ForegroundColor Red
        Write-Log 'MP_WEIXIN_RESULT errors=env reason=port-not-listening'
        exit 2
    }

    # 6) 探针：pageStack 前置断言 + 入口页/被改页导航 + console/exception + 截图
    #    先确保 miniprogram-automator 可用：装到**项目外**的临时根（--no-save + --ignore-scripts），
    #    不动仓库的 package.json / package-lock.json；探针用 --require-root 指过去解析。
    $autoRoot = Join-Path $env:TEMP 'mp-weixin-automator'
    $autoModule = Join-Path $autoRoot 'node_modules\miniprogram-automator\package.json'
    if (-not (Test-Path -LiteralPath $autoModule)) {
        $npmCmd = (Get-Command npm.cmd -ErrorAction SilentlyContinue | Select-Object -First 1).Source
        if (-not $npmCmd) { $npmCmd = (Get-Command npm -ErrorAction SilentlyContinue | Select-Object -First 1).Source }
        if (-not $npmCmd) {
            Write-Host '[error] 找不到 npm：无法准备 miniprogram-automator。' -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=npm-not-found'
            exit 2
        }
        New-Item -ItemType Directory -Force -Path $autoRoot | Out-Null
        $npmStep = Invoke-Process -FilePath $npmCmd -WorkingDirectory $autoRoot `
            -Arguments @('i', '--no-save', '--ignore-scripts', '--no-audit', '--no-fund', '--loglevel=error', 'miniprogram-automator@0.12.1') `
            -TimeoutSeconds 600 -Tag 'npm-automator'
        Write-Log "`n>>> npm-automator（$autoRoot）`n$($npmStep.Output)"
        Write-Host $npmStep.Output
        if (-not (Test-Path -LiteralPath $autoModule)) {
            Write-Host '[error] miniprogram-automator 装不上（见日志）：探针无法连自动化端口。' -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=automator-install-failed'
            exit 2
        }
    }
    Write-Log "automator require-root = $autoRoot"
    $probeArgs = @($probePath, '--ws', "ws://127.0.0.1:$Port", '--routes', ($routeList -join ','),
        '--entry-url', $entryUrl, '--shot-dir', $logDir, '--require-root', $autoRoot)
    # 探针重试：开发者工具把项目/小程序拉起来是**异步**的（实测连接与 pageStack 常在第 2–3 次才就绪，
    # 首次报 "check if target project window is opened with automation enabled"）⇒ 必须重试而不是一次定生死。
    $probe = $null
    $probeJson = $null
    for ($attempt = 1; $attempt -le $ProbeAttempts; $attempt++) {
        Write-Log "`n>>> automator-probe（第 $attempt/$ProbeAttempts 次）"
        $probe = Invoke-Process -FilePath $nodeExe -Arguments $probeArgs -TimeoutSeconds $ProbeTimeoutSeconds -Tag "automator-probe#$attempt"
        Write-Log $probe.Output
        Write-Host $probe.Output
        if ($probe.TimedOut) {
            Write-Host "[error] 探针超时（$ProbeTimeoutSeconds 秒）：元素级 API 会静默挂起，本门不得引入它们（#883 坑位 3）。" -ForegroundColor Red
            Write-Log 'MP_WEIXIN_RESULT errors=env reason=probe-timeout'
            exit 2
        }
        # **只看输出**：解析探针的 MP_WEIXIN_PROBE JSON（退出码不作判据）
        foreach ($line in ($probe.Output -split "`r?`n")) {
            if ($line -match '^\s*MP_WEIXIN_PROBE\s+(\{.*\})\s*$') {
                try { $probeJson = ($Matches[1] | ConvertFrom-Json) } catch { $probeJson = $null }
            }
        }
        if ($probeJson -and $probeJson.probeOk) { break }
        if ($probeJson -and @($probeJson.failures | Where-Object { $_ -match '环境不可用：找不到 miniprogram-automator|自动化端口连不上' }).Count -gt 0 -and $attempt -ge 2) {
            break  # 环境问题重试无益
        }
        if ($attempt -lt $ProbeAttempts) {
            Write-Host ">>> 探针未就绪（第 $attempt 次），$ProbeRetryDelaySeconds 秒后重试 —— 开发者工具拉起项目是异步的" -ForegroundColor Yellow
            Start-Sleep -Seconds $ProbeRetryDelaySeconds
        }
    }

    if (-not $probeJson) {
        Write-Host '[error] 探针未输出可解析的 MP_WEIXIN_PROBE 结果行（见日志）。' -ForegroundColor Red
        Write-Log 'MP_WEIXIN_RESULT errors=env reason=no-probe-result'
        exit 2
    }

    $stackLen = @($probeJson.pageStack).Count
    $shotPng = @($probeJson.shots | Where-Object { $_.isPng })
    $shotNames = (@($shotPng | ForEach-Object { Split-Path $_.path -Leaf }) -join ',')
    $resultLine = "MP_WEIXIN_RESULT appid=$productAppId pageStack=$stackLen entry=$($probeJson.entryPage) " +
        "errorsTotal=$($probeJson.errorsTotal) exceptionsTotal=$($probeJson.exceptionsTotal) logsTotal=$($probeJson.logsTotal) " +
        "shots=$($shotPng.Count) log=$logRelative"
    Write-Log "`n$resultLine"
    Write-Log ("shots: " + (@($shotPng | ForEach-Object { "$($_.path) $($_.width)x$($_.height) $($_.bytes)B" }) -join ' | '))

    # P2：门通过时把截图**压缩入库**到 PR 分支（docs/verification/<模块>/<PR号>/<页名>-after.<ext>）
    # 任何失败都只警告 + 给命令，**不让门因此失败**（维护者 2026-09-12 要求）。
    $archive = $null
    if ($PostToPr -gt 0 -and -not $NoArchive) {
        try {
            $archive = Publish-ScreenshotArchive -PrNumber $PostToPr -Module $Module -ShotDir $logDir -ProbeJson $probeJson -RepoRoot $Project
        } catch {
            Write-Host "[warn] 截图入库失败（不影响门结论）：$_" -ForegroundColor Yellow
            $archive = @{ Notes = @("入库异常：$_"); Commands = @() }
        }
        foreach ($an in @($archive.Notes)) { Write-Host "   [archive] $an"; Write-Log "archive: $an" }
        if (@($archive.Commands).Count -gt 0) {
            Write-Host "   [archive] 请手动执行：" -ForegroundColor Yellow
            foreach ($ac in @($archive.Commands)) { Write-Host "     $ac" -ForegroundColor Yellow }
        }
    } elseif ($NoArchive) {
        Write-Host "   [archive] -NoArchive：跳过截图入库" -ForegroundColor Yellow
    }

    $failed = @($probeJson.failures)
    if ($probeJson.probeOk -and $failed.Count -eq 0) {
        Write-Host ''
        Write-Host "✅ ② 通过：$resultLine" -ForegroundColor Green
        Write-Host "   截图：$shotNames（日志 $logPath）" -ForegroundColor Green
        Write-Host '   非等价声明：② ≠ ① 真机门，也 ≠ ④b 云打包门；content:// / 生物识别 / 第三方 SDK / 真机性能 挡不住。' -ForegroundColor Yellow
        Write-Host '   注意：不得由 agent 代填 ② 的「执行人」、不得由 agent 写「已通过」——本脚本只产出证据。' -ForegroundColor Yellow
        $exitCode = 0
    } else {
        Write-Host ''
        Write-Host "❌ ② 未过：$($failed.Count) 条断言未过 ——" -ForegroundColor Red
        $failed | Select-Object -First 20 | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
        @($probeJson.steps | Where-Object { $_.error }) | Select-Object -First 10 |
            ForEach-Object { Write-Host "   [step] $($_.label) $($_.route)：$($_.error)" -ForegroundColor Red }
        Write-Host "完整日志：$logPath" -ForegroundColor Yellow
        Write-Log ("MP_WEIXIN_RESULT errors=$($failed.Count) failures=" + ($failed -join ' / '))
        $exitCode = if (@($failed | Where-Object { $_ -match '环境不可用' }).Count -gt 0) { 2 } else { 1 }
    }
} catch {
    Write-Host "[error] ② 脚本异常：$_" -ForegroundColor Red
    Write-Log "MP_WEIXIN_RESULT errors=env reason=exception detail=$_"
    $exitCode = 2
} finally {
    # 必须在 finally 路径里关：否则自动化会话与端口会残留（下次 pageStack 空 ⇒ 误导弹错）
    Invoke-DevToolsClose
    # 释放 agent 互斥锁（-SkipBuild 下未取锁，Release-HxLock 是安全的空操作）
    if (Get-Command Release-HxLock -ErrorAction SilentlyContinue) { Release-HxLock }
}

# P1：仅门通过时贴「sha 绑定」评论（门结果免手抄；gh 不可用/取不到 sha 只警告，不改门结论）
if ($exitCode -eq 0 -and $PostToPr -gt 0) {
    Publish-GateComment -PrNumber $PostToPr -ResultLine $resultLine -LogRelative $logRelative `
        -ShotRelative ".ci-verify/$shotNames" -ReproCommand 'npm run build:mp-weixin-check'
        -ArchivedRel $(if ($archive -and $archive.Rel) { @($archive.Rel) } else { @() }) `
        -ArchiveNotes $(if ($archive) { @($archive.Notes) } else { @() })
}
exit $exitCode
