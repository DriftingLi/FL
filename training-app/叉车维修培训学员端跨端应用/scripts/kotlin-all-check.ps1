<#
.SYNOPSIS
    移动端验收门 ④ 的默认载体：本地整模块 Kotlin 编译（半自动门，口径 ④c）——输入取 appResource 产物。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md；来源 issue #859（spike）/ #870（T2）。
    2026-09-11 修订：④a 与 ④c 合并为「④ 本地编译门」——④c（本脚本）为**默认**，
    dev 专属面（pages.json / manifest.json / platformConfig.json 改动、新增页面、收口 PR）追加 ④a（`npm run build:compile`）。

    为什么需要它
      - ④a（`npm run build:compile`）走 HBuilderX 的**增量**编译：只把本次变更的 .kt 交给 kotlinc，
        并把上一次的 `class/` 目录放进 classpath 以补齐符号（uni-uts-v1/dist/uvue/kotlin.js 的
        runUVueKotlinDev / parseKotlinChangedFiles）。它抓不到「整模块一起编译才暴露」的问题。
      - ④b（云打包）是唯一真跑 Gradle compileReleaseKotlin 的门，但按次消耗云资源、需账号、非每 PR。

    ④c 做什么：把 `cli publish app-android --type appResource` 导出的 .kt（app 源 + uni_modules 源）
    一次性交给 HBuilderX **自带**的 kotlinc 编译，复刻「整模块」这一关键语义；
    不需要 Android SDK / Gradle / keystore / appkey / 云账号。

    **非等价声明（勿删）**：④c ≠ compileReleaseKotlin。工具链不同（HBuilderX 自带 kotlinc 2.2.0
    + 自带 Corretto 17 + lib2 运行时 jar vs 离线 SDK 的 Gradle / AGP / 插件版本），
    ④c 只作**本地加固**：不得据此声称「云打包已通过」，也不得改写 ④b 的定义。

    边界（2026-09-11 实测）
      - **输入必须是 appResource 产物**（unpackage/resources/app-android/**）；**不得**用 dev 编译缓存
        （unpackage/cache/.app-android/src）：该缓存是增量产物，被引用的类型可能只存在于陈旧
        class/*.class 里，整模块编译必然报大量 unresolved reference。
      - publish 那一步仍需 HBuilderX 主程序可达（CLI 靠与主程序的本地 IPC）；若已有 appResource
        产物，用 -SkipPublish 可让**受限会话**只跑 kotlinc 部分（这一步不依赖主程序）。
      - 判成败一律解析输出，**不看退出码**：HBuilderX CLI 退出码恒为 0（实测缺 --project 失败仍 0）。

    退出码：0 = 通过；1 = 编译报错（或 publish 未成功）；2 = 环境不可用（缺 CLI/编译器、无产物、超时）。

.PARAMETER PostToPr
    > 0 时，**仅在门通过（exit 0）分支**把结果贴成 PR 评论（P1：编译门结果免手抄）。
    评论正文严格形如 `<!-- gate-evidence:④ -->` + 「④ 本地编译门（④c 整模块 Kotlin 编译，agent 执行）」
    + `commit: <HEAD sha>` + 结论（含 KOTLIN_ALL_RESULT）与日志路径 + 复现命令；
    `.github/workflows/pr-evidence.yml` 只认「带该标记且 sha 与 PR head 相等」的评论，不校真伪。
    gh 不可用/取不到 sha 时只打印警告，**不影响门的结论**。

.EXAMPLE
    npm run build:kotlin-all
    pwsh -NoProfile -File scripts/kotlin-all-check.ps1 -SkipPublish
    pwsh -NoProfile -File scripts/kotlin-all-check.ps1 -HBuilderX 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX'
    pwsh -NoProfile -File scripts/kotlin-all-check.ps1 -PostToPr 859   # 通过后把结果贴成 sha 绑定评论
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$HBuilderX,
    [string]$Cli,
    [switch]$SkipPublish,
    [int]$HxWaitSeconds = 600,
    [switch]$HxNoWait,
    [int]$PublishTimeoutSeconds = 900,
    [int]$KotlincTimeoutSeconds = 900,
    [int]$PostToPr = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# 出错行判据：kotlinc 的「路径:行:列: error:」与 `e: ` 前缀两种形态
$ErrorLinePattern = '(?m)(^\s*e: )|(:\d+:\d+: *error:)'
$AppResourceRelative = 'unpackage\resources\app-android'
$DevCacheRelative = 'unpackage\cache\.app-android'

function Test-PeHeader {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $bytes = [System.IO.File]::ReadAllBytes($Path)
    } catch { return $false }
    return ($bytes.Length -gt 2 -and $bytes[0] -eq 0x4D -and $bytes[1] -eq 0x5A)
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

    # 1) 正在运行的 HBuilderX 同目录（版本必然匹配）
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($proc) {
            $candidate = Split-Path $proc.Path -Parent
            if (Test-PeHeader -Path (Join-Path $candidate 'cli.exe')) { return $candidate }
        }
    } catch { }

    # 2) 环境变量 3) 常见安装位置（校验 PE 头，跳过损坏副本）
    $candidates = @()
    if ($env:HBuilderX_HOME) { $candidates += $env:HBuilderX_HOME }
    foreach ($pattern in @(
            'D:\软件\HBuilderX*\HBuilderX',
            'D:\HBuilderX*\HBuilderX',
            'E:\HBuilderX*\HBuilderX',
            'C:\Program Files\HBuilderX*\HBuilderX',
            'C:\Program Files\HBuilderX',
            "$env:LOCALAPPDATA\HBuilderX")) {
        $candidates += (Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | ForEach-Object { $_.FullName })
    }
    foreach ($c in $candidates) {
        if (Test-PeHeader -Path (Join-Path $c 'cli.exe')) { return (Resolve-Path -LiteralPath $c).Path }
    }
    return $null
}

function Invoke-Process {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [int]$TimeoutSeconds,
        [string]$Tag
    )
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8
    foreach ($a in $Arguments) { [void]$psi.ArgumentList.Add($a) }

    Write-Host ">>> [$Tag] $FilePath"
    $p = [System.Diagnostics.Process]::Start($psi)
    $stdout = $p.StandardOutput.ReadToEndAsync()
    $stderr = $p.StandardError.ReadToEndAsync()
    $exited = $p.WaitForExit($TimeoutSeconds * 1000)
    if (-not $exited) {
        try { $p.Kill($true) } catch { }
        return @{ Output = "[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止"; ExitCode = -1; TimedOut = $true }
    }
    $out = $stdout.Result + $stderr.Result
    return @{ Output = $out; ExitCode = $p.ExitCode; TimedOut = $false }
}

# ---------- 项目与日志 ----------
if (-not $Project) { $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}
$logDir = Join-Path $Project '.ci-verify'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logPath = Join-Path $logDir 'kotlin-all.log'
Set-Content -LiteralPath $logPath -Encoding utf8 -Value @(
    "# kotlin-all-check (④c) 开始 $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
    "# project = $Project",
    "# 输入 = $AppResourceRelative（appResource 产物）；严禁改用 $DevCacheRelative（dev 缓存不是自洽编译单元）"
)
function Write-Log { param([string]$Text) Add-Content -LiteralPath $logPath -Value $Text -Encoding utf8 }

# ---------- HBuilderX 忙检测（单实例串行资源，ADR-0008 坑位段）----------
. (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
if ($SkipPublish) {
    Write-Host '>>> -SkipPublish：不接 HBuilderX，跳过忙检测与互斥锁（kotlinc 段不依赖主程序）' -ForegroundColor Yellow
} else {
    $hx = Wait-HxFree -CliExe (Join-Path (Resolve-HBuilderXRoot -Explicit $HBuilderX -ExplicitCli $Cli) 'cli.exe') -TimeoutSeconds $HxWaitSeconds -NoWait:$HxNoWait -LogPath $logPath
}
try {
# ---------- 1) 可选：导出 appResource 产物 ----------
$hbxRoot = Resolve-HBuilderXRoot -Explicit $HBuilderX -ExplicitCli $Cli
if (-not $SkipPublish) {
    if (-not $hbxRoot) {
        Write-Host '[error] 找不到可用的 HBuilderX（需含 cli.exe 与 plugins/）。用 -HBuilderX <安装根> 指定。' -ForegroundColor Red
        Write-Log 'KOTLIN_ALL_RESULT errors=env reason=hbuilderx-not-found'
        exit 2
    }
    $cliExe = Join-Path $hbxRoot 'cli.exe'
    foreach ($step in @(
            @{ Tag = 'cli-open'; Args = @('open') },
            @{ Tag = 'project-open'; Args = @('project', 'open', '--path', $Project) },
            @{ Tag = 'publish-appresource'; Args = @('publish', 'app-android', '--type', 'appResource', '--project', $Project) })) {
        $r = Invoke-Process -FilePath $cliExe -Arguments $step.Args -TimeoutSeconds $PublishTimeoutSeconds -Tag $step.Tag
        Write-Log "`n>>> $($step.Tag)"
        Write-Log $r.Output
        Write-Host $r.Output
        if ($r.Output -match '与主程序的连接已中断|启动超时') {
            Write-Host '[error] HBuilderX CLI 连不上主程序：受限/沙箱会话会阻断 CLI↔主程序的本地 IPC。' -ForegroundColor Red
            Write-Host '        处置：以全访问权限重跑；或先由人执行 HBuilderX 发行 → 本地打包资源，再用 -SkipPublish 重跑本脚本。' -ForegroundColor Red
            Write-Log 'KOTLIN_ALL_RESULT errors=env reason=cli-ipc-blocked'
            exit 2
        }
        if ($r.TimedOut) {
            Write-Host "[error] $($step.Tag) 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
            Write-Log 'KOTLIN_ALL_RESULT errors=env reason=timeout'
            exit 2
        }
    }
    # 成败以**产物**为准（比匹配文案可靠）；导出可能是异步的，故轮询等待
    $produced = 0
    for ($i = 0; $i -lt 40; $i++) {
        $produced = @(Get-ChildItem -LiteralPath (Join-Path $Project $AppResourceRelative) -Recurse -Filter *.kt -File -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -notmatch '\\www\\' }).Count
        if ($produced -gt 0) { break }
        Start-Sleep -Seconds 3
    }
    if ($produced -eq 0) {
        Write-Host '[error] appResource 导出未产出 .kt（见日志）：先解决导出失败，再跑 ④c。' -ForegroundColor Red
        Write-Host '        提示：若 HBuilderX 中已导入同名项目，CLI 可能把 publish 指向那个目录（实测遇到）—— 请用唯一目录名，或先在 HBuilderX 里关闭同名项目。' -ForegroundColor Red
        Write-Log 'KOTLIN_ALL_RESULT errors=1 stage=publish reason=no-artifact'
        exit 1
    }
    Write-Log "publish 产物 .kt = $produced"
}

# ---------- 2) 收集 appResource 产物里的 .kt ----------
$appResourceDir = Join-Path $Project $AppResourceRelative
if (-not (Test-Path -LiteralPath $appResourceDir)) {
    Write-Host "[error] 找不到 appResource 产物：$appResourceDir" -ForegroundColor Red
    Write-Host '        先在 HBuilderX 执行「发行 → 原生App-本地打包 → 生成本地打包App资源」，或去掉 -SkipPublish。' -ForegroundColor Red
    Write-Log 'KOTLIN_ALL_RESULT errors=env reason=no-appresource'
    exit 2
}
$ktFiles = @(Get-ChildItem -LiteralPath $appResourceDir -Recurse -Filter *.kt -File -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -notmatch '\\www\\' } | ForEach-Object { $_.FullName })
if ($ktFiles.Count -eq 0) {
    Write-Host "[error] $appResourceDir 下没有 .kt（导出物不完整）" -ForegroundColor Red
    Write-Log 'KOTLIN_ALL_RESULT errors=env reason=no-kt'
    exit 2
}

# ---------- 3) 定位自带工具链 ----------
if (-not $hbxRoot) {
    Write-Host '[error] 找不到 HBuilderX 安装根（需要 plugins/ 下的 kotlinc、Corretto、UTS 插件、lib2）。' -ForegroundColor Red
    Write-Log 'KOTLIN_ALL_RESULT errors=env reason=hbuilderx-not-found'
    exit 2
}
$javaExe = Join-Path $hbxRoot 'plugins\amazon-corretto\bin\java.exe'
$kotlincHome = Join-Path $hbxRoot 'plugins\uniapp-runextension\kotlinc'
$preloaderJar = Join-Path $kotlincHome 'lib\kotlin-preloader.jar'
$compilerJar = Join-Path $kotlincHome 'lib\kotlin-compiler.jar'
$lib2Dir = Join-Path $hbxRoot 'plugins\uniapp-runextension\lib2'
$utsPluginJar = Join-Path $hbxRoot 'plugins\uniapp-uts-v1\node_modules\@dcloudio\uni-uts-v1\lib\kotlin\lib\uts-kotlin-compiler-plugin.jar'
foreach ($p in @($javaExe, $preloaderJar, $compilerJar, $lib2Dir, $utsPluginJar)) {
    if (-not (Test-Path -LiteralPath $p)) {
        Write-Host "[error] HBuilderX 自带工具链缺失：$p" -ForegroundColor Red
        Write-Log "KOTLIN_ALL_RESULT errors=env reason=missing-toolchain path=$p"
        exit 2
    }
}
$classPath = (Get-ChildItem -LiteralPath $lib2Dir -Filter *.jar -File | ForEach-Object { $_.FullName }) -join ';'
$classOutDir = Join-Path $logDir 'kotlin-class'
if (Test-Path -LiteralPath $classOutDir) { Remove-Item -LiteralPath $classOutDir -Recurse -Force }
New-Item -ItemType Directory -Force -Path $classOutDir | Out-Null

# ---------- 4) 整模块编译（HBuilderX 自带 kotlinc + UTS 编译器插件）----------
$javaArgs = @(
    '-Xmx2g', '--add-opens', 'java.base/java.util=ALL-UNNAMED',
    '-cp', $preloaderJar,
    'org.jetbrains.kotlin.preloading.Preloader',
    '-cp', $compilerJar,
    'org.jetbrains.kotlin.cli.jvm.K2JVMCompiler'
) + $ktFiles + @(
    '-cp', $classPath,
    '-d', $classOutDir,
    '-kotlin-home', $kotlincHome,
    "-Xplugin=$utsPluginJar",
    '-P', 'plugin:io.dcloud.uts.kotlin:tag=UTS',
    '-P', 'plugin:io.dcloud.uts.kotlin:console=true'
)
$result = Invoke-Process -FilePath $javaExe -Arguments $javaArgs -TimeoutSeconds $KotlincTimeoutSeconds -Tag 'kotlinc-all'
Write-Log "`n>>> kotlinc-all（整模块，$($ktFiles.Count) 个 .kt）"
Write-Log $result.Output

if ($result.TimedOut) {
    Write-Host "[error] kotlinc 超时（$KotlincTimeoutSeconds 秒）" -ForegroundColor Red
    Write-Log 'KOTLIN_ALL_RESULT errors=env reason=kotlinc-timeout'
    exit 2
}

$errorLines = @([regex]::Matches($result.Output, $ErrorLinePattern) | ForEach-Object { $_.Value })
$classCount = @(Get-ChildItem -LiteralPath $classOutDir -Recurse -Filter *.class -File -ErrorAction SilentlyContinue).Count
$summary = "KOTLIN_ALL_RESULT errors=$($errorLines.Count) classes=$classCount files=$($ktFiles.Count) input=$AppResourceRelative log=$logPath"
Write-Log "`n$summary"

if ($errorLines.Count -gt 0) {
    Write-Host ''
    Write-Host "❌ ④c 未过：整模块编译报 $($errorLines.Count) 条 error ——" -ForegroundColor Red
    ($result.Output -split "`n" | Where-Object { $_ -match $ErrorLinePattern } | Select-Object -First 20) |
        ForEach-Object { Write-Host "   $($_.Trim())" -ForegroundColor Red }
    Write-Host "完整日志：$logPath" -ForegroundColor Yellow
    exit 1
}

Write-Host ''
Write-Host "✅ ④c 通过：整模块编译无 error（$summary）" -ForegroundColor Green
Write-Host '   注意：④c ≠ compileReleaseKotlin（工具链不同），只作本地加固，不能替代 ④b 的结论。' -ForegroundColor Yellow

# ---------- 5) P1：门通过时把结果贴成「sha 绑定」的 PR 评论（编译门结果免手抄）----------
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
        [string]$GateLabel,
        [string]$ResultLine,
        [string]$LogRelative,
        [string]$ReproCommand
    )
    $sha = Get-HeadSha -ProjectDir $Project
    if (-not $sha) {
        Write-Host '[warn] 取不到 HEAD sha（git 不可用或不在仓库里）：跳过贴 PR 评论，门结论不受影响。' -ForegroundColor Yellow
        return
    }
    $short = $sha.Substring(0, [Math]::Min(7, $sha.Length))
    # 正文格式被 .github/workflows/pr-evidence.yml 认（标记 + commit sha），改动前先看那边的注释
    $body = @(
        '<!-- gate-evidence:④ -->',
        "**④ 本地编译门（$GateLabel，agent 执行）**",
        "- commit: $sha",
        "- 结论（含产物）：``$ResultLine``；日志 ``$LogRelative``",
        "- 复现：``$ReproCommand``"
    ) -join "`n"
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        Write-Host '[warn] 找不到 gh CLI：跳过贴 PR 评论，门结论不受影响（结果见上面日志）。' -ForegroundColor Yellow
        return
    }
    try {
        $out = (& gh pr comment $PrNumber --body $body 2>&1 | Out-String)
        if ($out -match 'github\.com/') {
            Write-Host "✅ 已贴 PR #$PrNumber 的 ④ 门评论（sha 绑定 $short）。" -ForegroundColor Green
        } else {
            Write-Host "[warn] 贴 PR #$PrNumber 评论疑似失败（门结论不受影响）：$($out.Trim())" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "[warn] 贴 PR 评论失败（门结论不受影响）：$_" -ForegroundColor Yellow
    }
}

if ($PostToPr -gt 0) {
    Publish-GateComment -PrNumber $PostToPr -GateLabel '④c 整模块 Kotlin 编译' `
        -ResultLine $summary -LogRelative '.ci-verify/kotlin-all.log' -ReproCommand 'npm run build:kotlin-all'
}
exit 0


} finally {
    # 释放 agent 互斥锁（-SkipPublish 下未取锁，Release-HxLock 是安全的空操作）
    Release-HxLock
}
