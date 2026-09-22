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

    退出码：0 = 通过；1 = 编译报错 / **导出未刷新**（含导出无产物）；2 = 环境不可用（缺 CLI/编译器、publish 未成立、超时）。

    假绿的补丁（#1272，2026-09-22 实测）
      - 旧版两条判据都漏：(a) publish 步只按 `与主程序的连接已中断|启动超时` 两条文案判失败，
        于是 `-1:cli:命令'publish app-android'不存在或缺少参数`（**主程序忙/未就绪时的通用文案**）被放过；
        (b) 后续「成败以产物为准」只看**有没有 .kt**、不看新鲜度 ⇒ 只要磁盘上留着一份旧导出就判 ✅。
        实测的坏读数：`KOTLIN_ALL_RESULT errors=0 classes=1526 files=120` + ✅，而导出是第一棵树的
        （工作树里当时的改动在导出里搜不到）⇒ 那次读数**不覆盖当次改动**。
      - 现在两道判据都补上：publish 步要求输出里出现正向标记（`$PublishSuccessMarker`），
        且导出目录里 .kt 的最新 mtime **必须晚于**本次 publish 的基准（`Test-AppResourceFreshness`）。
        两者任一不成立 ⇒ **exit 非 0 且不打印 ✅**。
      - **根因写实**：票面原写「publish 命令在 v5.24 已不存在」是**误诊** —— 实测该命令存在且可用
        （同一台机成功导出 119 个 .kt）；这恰恰说明**不能靠文案猜命令是否存在**，只能判「这一步有没有成立」。
      - `-SkipPublish` 是受限会话的逃生门，**不适用**新鲜度判据 ⇒ 结果行显式记
        `freshness=skipped(skip-publish)`，不假装判过。

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

# ---------- publish 步与新鲜度的判据 ----------
# 两条判据（`Get-PublishVerdict` / `Test-AppResourceFreshness`）住在 `scripts/lib/publish-freshness.ps1`：
# 那里零副作用（只有函数与常量），故 `utils/kotlinAllStaleExportBehavior.test.js` 可以 dot-source 它
# 并**真执行**这两条判据 —— 判据留在本脚本里就只能读源码文本断言，而接线守护不构成 ③ 证据（#1272）。
. (Join-Path $PSScriptRoot 'lib\publish-freshness.ps1')

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
# 新鲜度判据的结论（结果行带上它）。预置成**响亮**的值而不是 `skipped(skip-publish)`：后者只属于
# `-SkipPublish` 分支；若将来有路径绕过裁决直接走到结果行，读出来是 `not-judged` —— 记账要 fail-closed
# （评审发现：原先预置成 `skipped(skip-publish)`，等于让机器读的那一栏默认取「无害」值）。
$freshnessVerdict = 'not-judged'
if ($SkipPublish) {
    $freshnessVerdict = 'skipped(skip-publish)'
    Write-Host '>>> -SkipPublish：不接 HBuilderX，跳过忙检测与互斥锁（kotlinc 段不依赖主程序）' -ForegroundColor Yellow
    Write-Host '    ⚠️ 同时**跳过新鲜度判据**：本次编译的产物是否覆盖当前树，由调用方保证 —— 结果行记 freshness=skipped(skip-publish)。' -ForegroundColor Yellow
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
    # 新鲜度判据的基准时刻：**发第一条 CLI 命令之前**（判据 = 导出目录里 .kt 的最新 mtime 必须晚于它）。
    # 见 Test-AppResourceFreshness 的说明：实测 publish 是全量重写，故该判据不会误杀「没改动」的合法运行。
    $publishStartedAt = Get-Date
    $publishCommand = ''
    # publish 步的输出/超时留到循环外统一裁决（见下面的 Get-PublishStageVerdict）。
    # 初始值不是 null：StrictMode 下取 `$null.X` 会抛，而「压根没拿到 publish 输出」这件事
    # 本来就等价于「没看到成功标记」⇒ 让它走 no-success-marker 那一支即可。
    $publishResult = @{ Output = ''; TimedOut = $false; ExitCode = -1 }
    foreach ($step in @(
            @{ Tag = 'cli-open'; Args = @('open') },
            @{ Tag = 'project-open'; Args = @('project', 'open', '--path', $Project) },
            @{ Tag = 'publish-appresource'; Args = @('publish', 'app-android', '--type', 'appResource', '--project', $Project) })) {
        $r = Invoke-Process -FilePath $cliExe -Arguments $step.Args -TimeoutSeconds $PublishTimeoutSeconds -Tag $step.Tag
        Write-Log "`n>>> $($step.Tag)"
        if ($step.Tag -eq 'publish-appresource') {
            $publishCommand = "$cliExe $($step.Args -join ' ')"
        }
        Write-Log $r.Output
        Write-Host $r.Output
        # publish 步的成败**不在这一层判**：整段裁决收在循环外（`Get-PublishStageVerdict`），
        # 为的是把「用了哪条命令 / 产物 mtime / 新鲜度判据」三件事**先写进日志再退** ——
        # 失败路径才最需要这三行（评审发现：原先它们只在成功路径齐，失败路径什么都没有）。
        if ($step.Tag -eq 'publish-appresource') {
            $publishResult = $r
            continue
        }
        # 注：这条字面量与库里的 `$PublishFailurePattern` **有意重复** ——
        # `utils/kotlinAllGateContract.test.js` 的 C7 要求「门脚本文本里必须有 CLI↔主程序 IPC 阻断的判据」，
        # 而库那份只服务于 publish 步的裁决；这条守的是前两步（cli-open / project-open）。
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
    # 导出可能是异步的 ⇒ 有界轮询等它变新鲜。publish 本身没成立时**不必等**，
    # 但下面仍会量一次导出目录 —— 失败日志同样需要 mtime 这一行。
    $exportProbeDir = Join-Path $Project $AppResourceRelative
    $publishPre = Get-PublishVerdict -Output $publishResult.Output -TimedOut $publishResult.TimedOut
    if ($publishPre.Ok) {
        for ($i = 0; $i -lt 40; $i++) {
            if ((Test-AppResourceFreshness -ExportDir $exportProbeDir -Since $publishStartedAt).Fresh) { break }
            Start-Sleep -Seconds 3
        }
    }
    # 整段裁决（门与守护**共用同一个函数** ⇒ 连「陈旧 ⇒ exit 1」这条决策序列本身也有证据）
    $publishEval = Get-PublishStageVerdict -PublishOutput $publishResult.Output -PublishTimedOut $publishResult.TimedOut -ExportDir $exportProbeDir -Since $publishStartedAt
    # 日志三件事：**所有路径都打**（成功与失败都要能看出「用了哪条命令 / 产物 mtime / 新鲜度判据结论」）
    Write-Log "publish 命令 = $publishCommand"
    Write-Log "publish 产物 mtime = $($publishEval.Newest)（.kt 数 = $($publishEval.KtCount)）"
    Write-Log "新鲜度判据 = $($publishEval.Freshness)（基准 = $publishStartedAt；判据 = 最新 .kt mtime 晚于基准）"
    Write-Log "publish 判据 = $($publishEval.Reason)（cli exitcode=$($publishResult.ExitCode)）"
    if ($publishEval.ExitCode -ne 0) {
        if ($publishEval.Reason -eq 'stale-export') {
            Write-Host "[error] appResource 导出**未刷新**：导出目录里有 $($publishEval.KtCount) 个 .kt，但最新 mtime = $($publishEval.Newest) **不晚于**本次基准 $publishStartedAt。" -ForegroundColor Red
            Write-Host '        ⇒ 这份产物**不覆盖当前树**，编译它得出的结论无意义（这正是 #1272 的假绿形态）。' -ForegroundColor Red
            Write-Host '        处置：先确认上面「publish 判据」那一行；或删掉导出目录后重跑。' -ForegroundColor Red
        } elseif ($publishEval.Reason -eq 'no-artifact') {
            Write-Host '[error] appResource 导出未产出 .kt（见日志）：先解决导出失败，再跑 ④c。' -ForegroundColor Red
            Write-Host '        提示：若 HBuilderX 中已导入同名项目，CLI 可能把 publish 指向那个目录（实测遇到）—— 请用唯一目录名，或先在 HBuilderX 里关闭同名项目。' -ForegroundColor Red
        } else {
            Write-Host "[error] publish 步**没有成立**（判据：$($publishEval.Reason)）⇒ ④c 不得据此判 ✅。" -ForegroundColor Red
            Write-Host '        三种可能：① HBuilderX 忙 / 未就绪（重试即可）；② 命令或参数在本版本不可用（对照 `cli publish app-android --help`）；③ CLI↔主程序 IPC 被断。' -ForegroundColor Red
            Write-Host '        若本版 CLI 的**成功文案**变了（正向标记未命中），改 scripts/lib/publish-freshness.ps1 的 $PublishSuccessMarker —— 判据是 fail-closed：宁可判红，不可假绿。' -ForegroundColor Red
        }
        # errors= 这一栏在失败路径写 `gate`（判据不成立）或 `env`（环境），**不写数字** ——
        # 旧写法写 `errors=1`，读起来像「有 1 条编译错误」，而那条路径根本没跑到编译（评审发现）。
        $errorTag = if ($publishEval.ExitCode -eq 1) { 'gate' } else { 'env' }
        Write-Log "KOTLIN_ALL_RESULT errors=$errorTag stage=publish reason=$($publishEval.Reason) freshness=$($publishEval.Freshness)"
        exit $publishEval.ExitCode
    }
    $freshnessVerdict = $publishEval.Freshness
    Write-Log "publish 产物 .kt = $($publishEval.KtCount)（新鲜度 $($publishEval.Freshness)）"
}

# ---------- 2) 收集 appResource 产物里的 .kt ----------
$appResourceDir = Join-Path $Project $AppResourceRelative
if (-not (Test-Path -LiteralPath $appResourceDir)) {
    Write-Host "[error] 找不到 appResource 产物：$appResourceDir" -ForegroundColor Red
    Write-Host '        先在 HBuilderX 执行「发行 → 原生App-本地打包 → 生成本地打包App资源」，或去掉 -SkipPublish。' -ForegroundColor Red
    Write-Log 'KOTLIN_ALL_RESULT errors=env reason=no-appresource'
    exit 2
}
# 产物口径的**唯一定义**在 scripts/lib/publish-freshness.ps1 的 `Get-AppResourceKtFiles`
# （评审发现：这里原先抄了一份「递归找 .kt + 排除 \www\」，而只有库那份进了守护）。
$ktFiles = @(Get-AppResourceKtFiles -ExportDir $appResourceDir | ForEach-Object { $_.FullName })
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
$summary = "KOTLIN_ALL_RESULT errors=$($errorLines.Count) classes=$classCount files=$($ktFiles.Count) input=$AppResourceRelative freshness=$freshnessVerdict log=$logPath"
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
# Get-HeadSha 已上移到 scripts/lib/gate-common.ps1（三份**逐字节相同**、且无任何守护 pin ⇒ 唯一合格的零风险
# 切片）。共享库头部写明了准入门槛：只有「逐字节相同」**且**「未被 utils/*Contract.test.js 作为字面量锚点
# pin 住」的代码才允许搬进去 —— 本仓守护断言的是**源码文本**，搬走被 pin 的代码等于逼着后续放宽守护。
. (Join-Path $PSScriptRoot 'lib\gate-common.ps1')

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
