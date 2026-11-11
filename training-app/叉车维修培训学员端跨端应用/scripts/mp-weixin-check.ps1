#!/usr/bin/env pwsh
<#
.SYNOPSIS
    mp-weixin **构建面**检查：publish（HBuilderX）→ 产物合法性 → 构建指纹 → 结果行。

.DESCRIPTION
    ⚠️ **不是验收门**。② 微信开发者工具门已于 2026-11-05 整体退役（#2320），本脚本不再做任何
    会话段动作：没有 cli quit / close→open→auto / 就绪闸门 / 探针 / auth 注入 / 逐页冷启动，
    也不启动开发者工具做取证。会话段 9 条守护与 4 个 mjs 已随本票删除（#2326）。

    为什么还留这一支（#2320 阶段一「保留必要构建命令」）：cli publish mp-weixin 是仓内唯一
    「微信小程序能否编译出来」的机检（#1670 重复 import 事故的反例面）。它是**可选检查** ——
    移动端 AGENTS.md ⑤ 不门化：PR 前建议跑一次、不判红。

    仍然保留 cli.bat close 的一次调用：HBuilderX publish 尾段会自启开发者工具（#2320 前实测），
    收掉那个自启实例靠 finally 里这一次 close。本脚本**不 kill IDE、不启停会话**。

.PARAMETER Project
    项目根（默认本脚本上一级 = uni-app x 项目根）。

.PARAMETER BuildOnly
    兼容保留：本脚本现在**只有构建面**这一种模式，给不给都是构建面。结果行的
    MP_WEIXIN_BUILDONLY_RESULT token 不受该开关影响。

.PARAMETER SkipBuild
    跳过 publish，只用现有产物（产物判据照跑）。指纹命中时脚本会**自动**翻成这个模式。

.PARAMETER DryRun
    只打印一行 GATE_PLAN {json} 后退出：不跑外部命令、不写工作树、不接 HBuilderX。

.NOTES
    退出码：0 = 构建面通过；1 = 产物/构建不过（门未过语义）；2 = 环境不可用。
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$AppId = 'wx38c3e31b16a7ced0',
    [string]$HBuilderX,
    [string]$Cli,
    [string]$DevToolsCli,
    [switch]$BuildOnly,
    [switch]$SkipBuild,
    [switch]$DryRun,
    [int]$PublishTimeoutSeconds = 900,
    [int]$PublishSettleQuietSeconds = 15,
    [int]$PublishSettleGraceSeconds = 120,
    [int]$HxWaitSeconds = 600,
    [switch]$HxNoWait
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
$OutputEncoding = [System.Text.Encoding]::UTF8

$DistRelative = 'unpackage\dist\build\mp-weixin'
# 主包体积两条线（#2417）：2 MiB 是微信**硬限**（超了上传直接拒）⇒ 判失败；1.5 MiB 只是**建议**线
# （上传面板口径）。为什么建议线不判红：minified 是**上传时**压缩（#2415 同树 A/B 实测：开关 ON/OFF
# 产物字节只差 42 B、.js 分毫不差）⇒ 本地产物字节反映不了上传后的读数，1.5 M 的权威读数只在面板上。
$MainHardLimitBytes = 2097152
$MainAdvisoryBytes = 1572864
# 开发者工具 cli.bat 的自动探测候选（逐字取自 HEAD 346–352 行）。
# ⚠️ 必须留在顶层：Resolve-DevToolsCli 在不传 -DevToolsCli 时枚举它（= npm 入口的真实形态），
# 而 Set-StrictMode 下「使用早于定义」当场抛「无法检索变量」——装配器按行区间抽取时把旧常量块整段跳过了（#2326 实测：
# 真跑 build:mp-weixin-buildonly 直接 exit 1，而套件因为每次都传 -DevToolsCli 而全绿）。守护 G5a/G5b 钉住这一类。
$WxDevToolsPatterns = @(
    'D:\微信web开发者工具\cli.bat',
    'D:\软件\微信web开发者工具\cli.bat',
    'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat',
    'C:\Program Files\Tencent\微信web开发者工具\cli.bat',
    "$env:LOCALAPPDATA\微信web开发者工具\cli.bat"
)
$script:closed = $false
$script:skipFinalClose = $false
# -Doctor 已随 ② 退役删除（#2326）；保留常量是因为指纹段与旧判定式里还按它取反。
$Doctor = $false
# 指纹命中而自动跳过 publish 的哨兵（#1861）：结果行要如实区分
# build=publish / build=skipped(fingerprint) / build=skipped(explicit) —— 不能把「没构建」写成「构建过」。
# ⚠️ 必须先初始化（旧脚本 369 行同位）：显式 -SkipBuild 的路径**不进**指纹块 ⇒ 收口处读它就是读未设置
# 变量，Set-StrictMode 下当场抛「无法检索变量」⇒ catch 写成 env-blocked、把通过的运行判成 exit 2
#（#2326 G2 实测踩到，红在 build=skipped(explicit) 那条断言上）。
$script:buildSkippedByFingerprint = $false
. (Join-Path $PSScriptRoot 'lib\gate-stages.ps1')

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
            'D:\软件\HBuilderX*\HBuilderX', 'D:\HBuilderX*\HBuilderX',
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
#
# 可选的「按产物收流」（#2163）：只给 -SettleDist / -SettleSince 时启用，服务构建段。
# 为什么要它（实测 2026-10-31，新树 5 s 采样）：产物写满（643 文件，14:53:54）与 cli 退出
# （14:55:29）之间**连健康跑都有 ≈90 s 空洞**；票面 B 形态是同一个空洞失去上界（产物 170 s 齐、
# cli 12 分钟不退）。旧实现只认 `WaitForExit(预算)` 一个信号 ⇒ 空洞一无界就必然烧到预算，
# 以 `errors=env-blocked reason=timeout` 白烧整轮 ②。
# 次序刻意是「判据成立后**再**等 cli 自然退出 GraceSeconds」，不是判据一成立就返回：
#   健康跑（实测 ≈90 s）落在 grace 窗口内 ⇒ 仍由 cli 自己退出、仍拿得到「导出微信小程序成功」
#   文案 ⇒ 行为与改动前一致；只有 cli 赖着不退（B 形态）才改走产物依据。
# 红线（票面第 5 条 / 移动端 ADR-0008）：收流**不 kill 任何进程** —— 与 timeout 分支的
# `Kill($true)` 相反，这里留下活着的 cli 并把它的 pid 记进门日志。
function Invoke-Process {
    param(
        [string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds, [string]$Tag, [string]$WorkingDirectory,
        [string]$SettleDist = '', [datetime]$SettleSince = (Get-Date),
        [int]$SettleQuietSeconds = 15, [int]$SettleGraceSeconds = 120, [int]$SettlePollSeconds = 2
    )
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

    # ---------- 有界等待 +（可选）按产物收流 ----------
    # 旧写法一次 `WaitForExit(整个预算)`：预算期内除「进程退出」没有别的信号，产物早写满也得干等。
    # 这里切成小片轮询，片间问一次收流判据；不给 SettleDist 时 sliceMs = 整个剩余预算，与旧行为同形。
    $settled = $false
    $settleInfo = $null
    $settledAt = $null
    $exited = $false
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ($true) {
        $remainMs = [int](($deadline - (Get-Date)).TotalMilliseconds)
        if ($remainMs -le 0) { break }
        $sliceMs = if ($SettleDist) { [Math]::Min($SettlePollSeconds * 1000, $remainMs) } else { $remainMs }
        if ($p.WaitForExit($sliceMs)) { $exited = $true; break }
        if (-not $SettleDist) { continue }
        $settleInfo = Get-MpWeixinPublishSettle -Dist $SettleDist -Since $SettleSince -QuietSeconds $SettleQuietSeconds
        if (-not $settleInfo.Settled) { $settledAt = $null; continue }
        if ($null -eq $settledAt) {
            $settledAt = Get-Date
            Write-Host "[settle] [$Tag] 产物已完整（$($settleInfo.Files) 文件 / $($settleInfo.Pages) 页），再等 cli 自然退出至多 $SettleGraceSeconds s" -ForegroundColor DarkGray
        }
        if (((Get-Date) - $settledAt).TotalSeconds -ge $SettleGraceSeconds) { $settled = $true; break }
    }

    if (-not $exited) {
        if ($settled) {
            # 不 kill：留下活着的 cli（其进程树里还有 HBuilderX 拉起的开发者工具），只放弃输出流。
            Write-Host "[warn] [$Tag] cli 未在 grace 窗口（$SettleGraceSeconds s）内退出（pid=$($p.Id)）⇒ 按产物收流，**未终止任何进程**；输出流放弃读取" -ForegroundColor Yellow
            return @{
                Output = ''; ExitCode = -1; TimedOut = $false; DrainAbandoned = $true
                Settled = $true; ChildPid = $p.Id; SettleReason = $settleInfo.Reason
                SettleDetail = $settleInfo.Detail; SettleFiles = $settleInfo.Files; SettlePages = $settleInfo.Pages
            }
        }
        try { $p.Kill($true) } catch { }
        # 到点被杀时把「为什么没收流」的实测量留在输出里：票面 A 形态要求日志能分清
        # 「没产物」与「有产物没退出」—— 旧实现两种都只打 reason=timeout，读不出差别。
        $why = 'settle=off'
        $s = @{ Reason = ''; Files = 0; NewestAgeS = -1 }
        if ($SettleDist) {
            $s = Get-MpWeixinPublishSettle -Dist $SettleDist -Since $SettleSince -QuietSeconds $SettleQuietSeconds
            $why = "settleReason=$($s.Reason) files=$($s.Files) newestAgeS=$($s.NewestAgeS) dist=$SettleDist"
        }
        return @{ Output = "[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止（$why）"; ExitCode = -1; TimedOut = $true; DrainAbandoned = $false; Settled = $false; ChildPid = 0; SettleReason = $s.Reason; SettleDetail = $why; SettleFiles = $s.Files; SettlePages = 0 }
    }
    # ---------- F4（2026-10-29）：主进程已退出 ≠ 输出流 EOF ----------
    # 病根（#2107 量尺实测，1050 s 纯挂死的一段）：`cli.bat close` 回显 `✔` 后 cmd.exe 正常退场，
    # 但**它拉起的开发者工具进程还活着，并继承了 stdout 句柄** ⇒ `ReadToEndAsync` 永远等不到 EOF，
    # 下方 `.Result` 会一直挂着。而超时判据**刚刚已经被 WaitForExit 走过了**（它返回 true），
    # 于是没有任何兜底：门其实早就出了结论，命令却不返回，现场读起来像「还在跑」。
    # 处置：主进程退出后给输出流一个**有界收尾窗口**（5 s，正常命令的数据早已写完）；
    # 收不到就明确放弃剩余输出并打 [warn]，绝不让一条挂住的读取抹掉「进程已退出」这个事实。
    # 判据因此只认**进程真退出**（WaitForExit 的返回值），不认 `✔` 之类的回显文案。
    $drainMs = 5000
    $drained = $false
    try { $drained = [System.Threading.Tasks.Task]::WaitAll(@($stdout, $stderr), $drainMs) } catch { $drained = $false }
    if (-not $drained) {
        Write-Host "[warn] [$Tag] 主进程已退出（exit=$($p.ExitCode)）但输出流仍被占用（子进程继承了 stdout 句柄）⇒ 放弃剩余输出，不影响退出码与门结论" -ForegroundColor Yellow
    }
    # 只在真的完成时取值：未完成的 Task 上取 .Result 就是上面那条挂死的复现方式
    $outText = ''; $errText = ''
    try { if ($stdout.IsCompleted) { $outText = $stdout.Result } } catch { $outText = '' }
    try { if ($stderr.IsCompleted) { $errText = $stderr.Result } } catch { $errText = '' }
    return @{ Output = ($outText + $errText); ExitCode = $p.ExitCode; TimedOut = $false; DrainAbandoned = (-not $drained); Settled = $false; ChildPid = 0; SettleReason = ''; SettleDetail = ''; SettleFiles = 0; SettlePages = 0 }
}

# 探针 JSON 的安全取值：Set-StrictMode 下访问不存在的属性会抛异常，
# 而「navigation 字段是否存在」正是降级判据，不能因为字段缺失把脚本炸成 env 失败。
function Get-Prop {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]
    if ($p) { return $p.Value }
    # 字典（`[ordered]@{}` / `@{}`）的键**不是** PSObject 属性：#1817 实测 `Get-Prop $plan 'routes'`
    # 静默回 $null，计划里声明的游客页族被读成空清单（游客页照旧被注 auth 会话），
    # 而源码文本面的守护照样全绿 —— 读不到配置比读错配置更难发现。字典按 key 取。
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) { return $Object[$Name] }
    return $null
}

function Get-GateStep {
    param($Plan, [string]$Id)
    foreach ($s in @($Plan['steps'])) {
        if ("$($s['id'])" -eq $Id) { return $s }
    }
    throw "门计划里没有步骤 '$Id'（Get-GateStep）—— 计划与执行脱节了"
}

# 结果行：按计划声明的字段清单拼（必填字段缺值即抛；可选字段无值就跳过）⇒ 清单不会与实际输出分叉。
function Format-GateMetricLine {
    param($Plan, [System.Collections.IDictionary]$Values)
    $parts = @()
    foreach ($f in @($Plan['resultLine']['fields'])) {
        $name = "$($f['name'])"
        $optional = if ($f.Contains('optional')) { [bool]$f['optional'] } else { $false }
        if (-not $Values.Contains($name)) {
            if ($optional) { continue }
            throw "结果行字段 $name 在 Values 里没有取值（计划与执行脱节）"
        }
        $parts += "$name=$($Values[$name])"
    }
    return ($parts -join ' ')
}

# -DryRun 的唯一输出：**单行** `GATE_PLAN {json}`。强制 UTF-8 输出，免得中文判据在重定向下变乱码。
function Write-GatePlanJson {
    param($Plan)
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $json = $Plan | ConvertTo-Json -Depth 12 -Compress
    Write-Output ('GATE_PLAN ' + $json)
}

# ---------- 项目与日志 ----------
if (-not $Project) { $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
$Project = (Resolve-Path -LiteralPath $Project).Path
$script:workDir = $Project

$dist = Join-Path $Project $DistRelative
$logDir = Join-Path $Project '.ci-verify'
$logPath = Join-Path $logDir 'mp-weixin.log'
$script:logPath = $logPath
$logRelative = '.ci-verify/mp-weixin.log'

# ---------- 门计划：只剩构建面（计划仍是「这轮真会跑什么」的唯一真源）----------
# 步骤集合故意只有 devtools-close：publish 由 HBuilderX 的 publish 命令本体承担（不是「步骤表里的一个 argv」），
# 收尾 close 是唯一还需要 argv 的外部调用。会话段步骤（env-preprobe / quit / open / auto / port / ready /
# probe / direct）全部随 ② 退役删除 —— 列着「这轮不跑」的步骤就是两份真相（#1861 的教训）。
function New-GatePlan {
    param(
        [string]$Project,
        [string]$Dist,
        [string]$LogRelative,
        [int]$PublishTimeoutSeconds,
        [bool]$SkipBuild,
        [bool]$DryRun
    )
    $steps = @(
        [ordered]@{
            id             = 'devtools-close'
            tag            = 'devtools-close'
            argv           = @('close')
            timeoutSeconds = 120
            criteria       = @('收掉 HBuilderX publish 尾段自启的开发者工具实例（幂等；失败只警告，不改结论）', '不 quit、不 open、不 auto —— 会话段已随 ② 退役（#2320/#2326）')
            exit2          = @()
        }
    )
    return [ordered]@{
        schema   = 'mp-weixin-gate/1'
        mode     = [ordered]@{ dryRun = $DryRun; skipBuild = $SkipBuild; doctor = $false; buildOnly = $true }
        project  = [ordered]@{ root = $Project; dist = $Dist }
        steps    = $steps
        timeouts = [ordered]@{ publish = $PublishTimeoutSeconds; close = 120 }
        # exit 2 归属只声明给计划里真有的步骤：这一版没有「环境不可用」归属的步骤
        # （devtools-close 失败只警告）⇒ 空数组，摊平口径与守护断言同源。
        exit2    = @()
        resultLine = [ordered]@{
            prefix = 'MP_WEIXIN_BUILDONLY_RESULT'
            # 字段 = 这一轮真能读到的读数（列着读不到的字段就是逼执行路径填假值）。
            # #2417 起补三枚产物开关读数 + 主包字节数与 2 MiB 硬限判定：票面要的是「结果行直接读出」——
            # 开关只写在日志正文里时，人读结果行看不出产物到底带没带，等于没读。
            fields = @(
                [ordered]@{ name = 'appid' },
                [ordered]@{ name = 'pages' },
                [ordered]@{ name = 'product' },
                [ordered]@{ name = 'lazyCodeLoading' },
                [ordered]@{ name = 'minified' },
                [ordered]@{ name = 'es6' },
                [ordered]@{ name = 'build' },
                [ordered]@{ name = 'elapsed' },
                [ordered]@{ name = 'mainBytes' },
                [ordered]@{ name = 'main2m' },
                [ordered]@{ name = 'log' }
            )
        }
    }
}
function New-ThisGatePlan {
    return New-GatePlan -Project $Project -Dist $dist -LogRelative $logRelative -PublishTimeoutSeconds $PublishTimeoutSeconds -SkipBuild ([bool]$SkipBuild) -DryRun ([bool]$DryRun)
}
$script:GatePlan = New-ThisGatePlan

# -DryRun：只打计划即退出。之前不许有任何外部命令 / 写工作树 / 接 HBuilderX（守护断言这一点）。
if ($DryRun) {
    Write-GatePlanJson -Plan $script:GatePlan
    exit 0
}

function Write-Log {
    param([string]$Text)
    Add-Content -LiteralPath $logPath -Value $Text -Encoding utf8
    # #2162：行首结果行**原样透传** stdout —— 通过分支的裸行此前被「✅ ② 通过：<行>」的中文前缀
    # 遮住（行首判据匹配不上），环境受阻分支则一个字都不打 ⇒ 按行首认读数的入口只能读成 no-line
    # （原调用方 pr-gates 的 `mp=` 自 #2320 起不再跑本门，但「裸行才可读」这条契约对手动运行同样成立）。
    # 覆盖三种结果行（通过/门未过/env-blocked，互斥由构造器保证）与 -BuildOnly 的独立 token；
    # 日志文件仍是字节级真源，透传逐字节 = 日志里那行，非结果行维持「只进文件」原语义。
    # 排除 `comment=`（投递诊断行，#2830 先例）：它同样以裸 token 开头但**不是读数行** ——
    # 调用方取**最后**一条行首匹配（原 pr-gates 口径），绿跑后若投递失败，诊断行会顶掉真读数。
    foreach ($ln in ($Text -split "`r?`n")) {
        if ($ln -match '^MP_WEIXIN(_BUILDONLY)?_RESULT(\s|$)' -and $ln -notmatch ' comment=') { Write-Host $ln }
    }
}
# -BuildOnly 的脏改提示（#1861 2026-10-28 裁定「跑完打印脏改提示」）：publish 经 HBuilderX 会回写工作树 ——
# manifest.json 的 mp-weixin.appid 可能被置 null、pages.json 可能被写入 condition（GUI 选的启动页，属本地开发
# 配置、禁止提交）。脚本**不自动改工作树**（自动 checkout 会把用户真实的改动一起抹掉），只把核对与还原动作印出来。
function Write-BuildOnlyWorktreeHint {
    Write-Host ''
    Write-Host '>>> 脏改提示（build-only 不自动改工作树，请人工核对）：' -ForegroundColor Yellow
    Write-Host '    git status --porcelain -- manifest.json pages.json' -ForegroundColor Yellow
    Write-Host '    · manifest.json 的 mp-weixin.appid 被置 null（HBuilderX 回写）⇒ git checkout -- manifest.json' -ForegroundColor Yellow
    Write-Host '    · pages.json 的 condition 段是 GUI 选的启动页（本地开发配置）⇒ 禁止提交，git checkout -- pages.json' -ForegroundColor Yellow
}
# 结果行前缀的**唯一取值点**（#1861）：门计划按 mode 声明（`-BuildOnly` ⇒ MP_WEIXIN_BUILDONLY_RESULT，
# 与 ② 的 MP_WEIXIN_RESULT 可判读分流，同 TEST_UNIT_LOCAL_RESULT 先例）。早期 exit 路径（计划已建但
# 未走主流程）与异常兜底都从这里取 —— 不靠各调用点自觉，否则分流会被「某处硬写前缀」破坏
# （先例：脚本异常兜底行曾硬写 MP_WEIXIN_RESULT，-BuildOnly 下异常就会打成 ② 的 token）。
function Get-GateResultPrefix {
    if ($script:GatePlan -and $script:GatePlan['resultLine']) { return [string]$script:GatePlan['resultLine']['prefix'] }
    return 'MP_WEIXIN_RESULT'
}
# 阶段收尾的**幂等**入口（#1861）：只在真有未闭段时才收尾（否则录制器会抛「无未闭段」）。
# 为什么要这一层而不是让调用点各自判断：-BuildOnly 有两条退出路径 —— 真跑过 HBuilderX 段（wait-hx /
# publish 已 Start，有未闭段）与 -SkipBuild（一段都没 Start）。「有没有未闭段」是录制器自己的状态，
# 不该由调用点各自猜：猜错要么丢掉该出的 STAGE_SUMMARY，要么把一次正常退出抛成 exit 2。
# ② 的主通过路径同样走这一层（语义等价），收尾点因此仍只有一处字面量（接线守护 gateStagesContract C6）。
function Stop-GateStageIfOpen {
    $open = Get-Variable -Name GateStageOpen -Scope Script -ErrorAction SilentlyContinue
    if ($null -ne $open -and $null -ne $open.Value) { Stop-GateStage }
}
# 日志目录按需创建 —— 必须排在 -DryRun 退出**之后**（-DryRun 的承诺是「不写工作树」）。
if (-not (Test-Path -LiteralPath $logDir)) { $null = New-Item -ItemType Directory -Force -Path $logDir }
# #2001 阶段时间戳：录制器指向本门日志（纯追加 [STAGE]/STAGE_SUMMARY 行，不改门判据）
. (Join-Path $PSScriptRoot 'lib\gate-stages.ps1')
Initialize-GateStageRecorder -LogPath $logPath

# ---------- 环境面 ----------
$hbxRoot = Resolve-HBuilderXRoot -Explicit $HBuilderX -ExplicitCli $Cli
$cliExe = if ($hbxRoot) { Join-Path $hbxRoot 'cli.exe' } else { $null }
if (-not $SkipBuild) {
    if (-not $hbxRoot) {
        Write-Host '[error] 找不到可用的 HBuilderX（需含 cli.exe 与 plugins/）。用 -HBuilderX <安装根> 或 -Cli <cli.exe> 指定。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=hbuilderx-not-found"
        exit 2
    }
} elseif ($Cli -and -not $hbxRoot) {
    Write-Host "[error] 指定的 cli.exe 不可用（不存在或 PE 头损坏）：$Cli" -ForegroundColor Red
    Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-unusable"
    exit 2
}
# 开发者工具**不再是构建面前置**（#2326）：会话段已随 ② 退役删除，这里留着 cli.bat 只为 finally
# 那一次 close —— publish 尾段会自启开发者工具，收掉自启实例用的。找不到就只警告：
# 没有工具 = 没有自启实例可收，不该让「能不能编译出来」这项检查因为没装工具而判环境不可用。
$devTools = Resolve-DevToolsCli -Explicit $DevToolsCli
if (-not $devTools) {
    Write-Host "[warn] 未找到微信开发者工具 cli.bat —— 构建面照跑，收尾 close 跳过（无自启实例可收）。" -ForegroundColor Yellow
    Write-Log 'devtools-not-found close=skipped（构建面不依赖开发者工具，#2326）'
}

# ---------- publish 参数：本轮唯一构造点（#2048）----------
# 为什么要提前到这里构造：指纹的第四道比对必须在**判命中之前**就知道本轮会用哪条命令 —— publish
# 参数住在 .ps1 里，而 .ps1 与 scripts/ 都被构建输入集排除（#2048 G1：改命令不动任何哈希组件、旧产物
# 原样在盘 ⇒ 三组件照样全等 ⇒ 命中 ⇒ 跳过 publish ⇒ 新命令从未执行、门在旧命令产物上出绿）。
# 构造点唯一 ⇒ 比对侧与执行侧用同一份数组，不存在「比 A 跑 B」的两份真相。
. (Join-Path $PSScriptRoot 'lib\mp-weixin-build-fingerprint.ps1')
$publishArgs = @('publish', 'mp-weixin', '--project', $Project)
if ($AppId) { $publishArgs += @('--appid', $AppId) }
$buildCommandKey = Get-MpWeixinBuildCommandKey -Arguments $publishArgs

# ---------- 产物内容指纹 ⇒ 自动 -SkipBuild（#1860）----------
# 白烧场景 = 同树重跑（#1494 §0.1 失败轮 / #1443 实测 -SkipBuild 省 ≈2–3 min）：环境失败轮重跑时
# publish 会重建一模一样的产物。「产物未变」机判化 —— 指纹键三组件**全等才命中**（判据库
# scripts/lib/mp-weixin-build-fingerprint.ps1，范式依据调研 §1.2 条 2）：
#   inputs   = 构建输入集内容哈希 —— 源码变了必不命中（防「改了源码却拿旧产物出绿」的假证据）；
#   product  = 产物目录内容哈希（排除指纹文件自身）—— 产物被换/被改必不命中（「产物或版本变化 ⇒ 必然重建」）；
#   hbuilderx = HBuilderX 版本令牌（目录版本号 + cli.exe 内容哈希；姊妹篇坑位 6b：工具版本会翻结论）。
# 失配方向一律 **fail 向重建**：任一组件算不出 / 指纹缺失或损坏 ⇒ 重建，绝不猜命中。
# 作用域 = 当前树：指纹落产物目录内（.mp-weixin-build-fingerprint.json），跨 worktree 不共享，
# 产物被清时自然失效。「门结论只对这份产物成立」语义不变 —— 指纹只是把「产物没变」机判化。
# 命中即翻 $SkipBuild 并**重建门计划**（mode.skipBuild 是计划面字段，翻值不重建 = 计划与执行脱节；
# 同款先例：端口被占换端口后重建计划）。-Doctor 跳过（体检不构建）；显式 -SkipBuild 无指纹事。
if (-not $SkipBuild -and -not $Doctor) {
    $fpVersion = Get-MpWeixinHBuilderXVersionToken -HbxRoot $hbxRoot -CliExe $cliExe
    $fpHit = $null
    if ("$fpVersion") { $fpHit = Test-MpWeixinBuildFingerprintHit -Project $Project -Dist $dist -HBuilderXVersion $fpVersion -BuildCommand $buildCommandKey }
    if ($fpHit -and $fpHit.Hit) {
        Write-Host ">>> 产物指纹命中 ⇒ 自动 -SkipBuild（$($fpHit.Detail)）" -ForegroundColor Yellow
        Write-Log "AUTO_SKIP_BUILD $($fpHit.Reason) version=$($fpVersion -replace 'cli=[0-9a-f]+', 'cli=<sha256>')"
        $SkipBuild = $true
        $script:GatePlan = New-ThisGatePlan
        # 如实记账（#1861）：-BuildOnly 的结果行要区分「本轮真构建过」与「指纹命中跳过」，不许含混。
        # 写在计划重建之后，保持「翻 $SkipBuild ⇒ 立刻重建计划」这条不变量的相邻性（守护 C27 钉的就是它）。
        $script:buildSkippedByFingerprint = $true
    } elseif ($fpHit) {
        Write-Log "build-fingerprint miss reason=$($fpHit.Reason)（$($fpHit.Detail)）"
    } else {
        Write-Log 'build-fingerprint check skipped reason=hbuilderx-version-unavailable（版本令牌算不出 ⇒ 必重建，fail 向重建）'
    }
}

# 端口占用已在**门计划构造之前**处理掉（换空闲端口 + 重建计划）—— 见上面 `Get-FreeAutoPort` 段。
# 这里不再判一次：残留会话会让 `cli.bat auto` **静默复用它**（端口在监听、ws 接得上，但 `Tool.getInfo`
# 无应答 ⇒ 就绪闸门 reason=sdk-version-missing），只 warning 等于把「复用死会话」当成正常路径
# （2026-09-23 实测定位，ADR-0008 坑位 6 续；ADR 坑位 1 讲的 pageStack 空是同一族的另一形态）。

function Invoke-DevToolsClose {
    if ($script:closed) { return }
    $script:closed = $true
    if (-not $devTools) { return }
    # #1860：楔死早退路径不收尾 —— close 对死端口 .ide 会自启 IDE 再关（cli 自启同族），且本路径
    # 从未创建自动化会话、无残留可收；处置已指向人工，跳过（哨兵见 $script:skipFinalClose 声明处）。
    if ($script:skipFinalClose) { return }
    # 参数（含 --project 与超时）一律取自门计划，不再写死在调用点（#914：判据面 = 计划，不是源码文本）
    $closeStep = Get-GateStep -Plan $script:GatePlan -Id 'devtools-close'
    try {
        $r = Invoke-Process -FilePath $devTools -Arguments $closeStep.argv -TimeoutSeconds $closeStep.timeoutSeconds -Tag $closeStep.tag
        # F4（2026-10-29）：close 后开发者工具**本来就会活着**（它只关项目窗口），继承 stdout 句柄是常态。
        # 把这层预期写进日志，免得下一个人把 Invoke-Process 的 [warn] 读成故障。
        if ($r.DrainAbandoned) { Write-Log ">>> $($closeStep.tag)：主进程已退出，输出流被常驻 IDE 继承（DrainAbandoned=true，预期行为，不影响结论）" }
        Write-Log "`n>>> $($closeStep.tag)`n$($r.Output)"
        Write-Host $r.Output
    } catch {
        Write-Host "[warn] cli.bat close 失败（不影响门结论）：$_" -ForegroundColor Yellow
    }
}

# ---------- 主流程 ----------
$exitCode = 2
# publish 段墙钟（#1861 ④）：-BuildOnly 的结果行读它；-SkipBuild / 指纹跳过时保持 0（结果行写 n/a）。
$publishSeconds = 0
try {
    # HBuilderX 忙检测（单实例串行资源，ADR-0008 坑位段）：-SkipBuild 下不接主程序
    if (-not $SkipBuild) {
        . (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
        Start-GateStage -Name 'wait-hx'
        $hx = Wait-HxFree -CliExe $cliExe -TimeoutSeconds $HxWaitSeconds -NoWait:$HxNoWait -LogPath $logPath
    }
    Write-Host "proj  = $Project"
    Write-Host "dist  = $dist"
    Write-Host "cli   = $cliExe"
    Write-Host "wxcli = $devTools"
    Write-Host "log   = $logPath"

    if (-not $SkipBuild) {
        Start-GateStage -Name 'publish'
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
            if ($BuildOnly) {
                # -BuildOnly **跳过**这条断言（#1861 2026-10-28 裁定）：本模式不上传 ⇒ 产物落 touristappid 无害；
                # 而 HBuilderX 首次导入/编译会把工作树 manifest.json 的 mp-weixin.appid 置 null（正上方坑位），
                # 保留断言 ⇒ **第二次起必假红**。实读值照常进日志与结果行，不藏（下面的脏改提示负责提醒还原）。
                Write-Host ">>> -BuildOnly：源 manifest appid='$srcAppId' 与期望 '$AppId' 不一致 —— 本模式不做 appid 断言（不上传，产物落 touristappid 无害）" -ForegroundColor Yellow
                Write-Log "build-only: manifest-appid 不断言 manifest='$srcAppId' expected='$AppId'"
            } else {
                Write-Host "[error] 源 manifest 的 mp-weixin.appid 与期望不一致：manifest='$srcAppId' 期望='$AppId'" -ForegroundColor Red
                Write-Host '        这正是 HBuilderX 回写 manifest.json 把 appid 置 null 的痕迹（2026-09-12 实测）；' -ForegroundColor Red
                Write-Host '        产物 appid 只会取自 manifest（CLI 的 --appid 不改产物）⇒ 此时跑出来的门是「游客 appid 下的门」，不能算数。' -ForegroundColor Red
                Write-Host "        处置：先 `git checkout -- manifest.json` 还原真实 appid，再重跑本门。" -ForegroundColor Red
                Write-Log "$(Get-GateResultPrefix) errors=1 stage=manifest-appid manifest=$srcAppId expected=$AppId"
                exit 1
            }
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
                Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-ipc-blocked"
                exit 2
            }
            if ($r.TimedOut) {
                Write-Host "[error] $($step.Tag) 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
                Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=timeout"
                exit 2
            }
        }

        # 2) 构建 mp-weixin —— HBuilderX 构建成功后**自己**拉起开发者工具并打开项目（#883 实测）。
        #    $publishArgs 在指纹判定之前唯一构造（#2048），这里**不得再拼一次**：拼两次就是两份真相，
        #    第四道比对会退化成「比对用 A、执行用 B」的假保险。
        #    墙钟单独计（#1861 ④）：-BuildOnly 的结果行要报 publish 段秒数（参照值 161 s / 48 页、227 s / 59 页），
        #    不从 [STAGE] 行反推 —— 那些行在早退路径上可能缺席。
        #
        # 「按产物收流」（#2163）：基准时刻取在**发 publish 命令之前** ⇒ 上一次跑留下的产物必然判
        # stale（本票实测：中途终止的构建根本不写 dist，写完的才会新）。判据本体在
        # `lib/mp-weixin-publish-settle.ps1`，它复用 #1210 的产物合法性单点真源 —— 这里不复述第二份。
        . (Join-Path $PSScriptRoot 'lib\mp-weixin-publish-settle.ps1')
        $settleSince = Get-Date
        $pubSw = [System.Diagnostics.Stopwatch]::StartNew()
        $pub = Invoke-Process -FilePath $cliExe -Arguments $publishArgs -TimeoutSeconds $PublishTimeoutSeconds -Tag 'publish-mp-weixin' `
            -SettleDist $dist -SettleSince $settleSince -SettleQuietSeconds $PublishSettleQuietSeconds -SettleGraceSeconds $PublishSettleGraceSeconds
        $pubSw.Stop()
        $publishSeconds = [int]$pubSw.Elapsed.TotalSeconds
        Write-Log "`n>>> publish-mp-weixin`n$($pub.Output)"
        Write-Host $pub.Output
        if ($pub.Output -match '与主程序的连接已中断') {
            Write-Host '[error] HBuilderX CLI 连不上主程序（见日志）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-ipc-blocked"
            exit 2
        }
        if ($pub.TimedOut) {
            Write-Host "[error] publish 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
            # 读数把两种形态分开钉住：没产物（settle=product-missing files=0）与
            # 「产物齐了但 cli 不退」（旧实现只能等预算到点）处置完全不同，而旧的那行两种都只打
            # reason=timeout，现场读不出差别（票面 AC1 要的正是能被区分出来的两种形态）。
            $sr = if ($pub.SettleReason) { $pub.SettleReason } else { 'off' }
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=timeout settle=$sr files=$($pub.SettleFiles) budget=${PublishTimeoutSeconds}s"
            exit 2
        }
        if ($pub.Settled) {
            # 收流成立：cli 没退 ⇒ 输出流没 EOF，**没有文案可读**。依据换成产物判据（同 #1381 的
            # 教训：别让单条文案成为必要条件，采集面丢一条就把整扇门变恒红）。
            # 门结论依旧由下面的产物合法性 + appid 断言与探针跑出，不是「大概成功了」。
            # 红线：此处不终止任何进程（cli pid 记进日志，交下面的 close 段处置）。
            Write-Host "[settle] publish 按产物收流：$($pub.SettleDetail)（cli pid=$($pub.ChildPid) 未终止）" -ForegroundColor Green
            Write-Log "publish=settled-by-artifact cli_pid=$($pub.ChildPid) files=$($pub.SettleFiles) pages=$($pub.SettlePages) reason=$($pub.SettleReason) secs=$publishSeconds"
        }
        # 成败一律解析输出（CLI 退出码不可用）
        if (-not $pub.Settled -and $pub.Output -notmatch '导出微信小程序成功|编译成功') {
            Write-Host '[error] publish 未报「导出微信小程序成功」（见日志）：先解决构建失败，再跑 ②。' -ForegroundColor Red
            Write-Host '        提示：HBuilderX 按**项目名（目录名）**解析，同名目录会被误命中 ⇒ 请用唯一目录名（#883 坑位）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=publish reason=no-artifact"
            exit 1
        }
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 产物不完整（缺 app.json）：$dist" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=publish reason=incomplete-artifact"
            exit 1
        }
    } else {
        Write-Log "`n>>> -SkipBuild：跳过 publish，直接用现有产物 $dist"
        Write-Host ">>> -SkipBuild：跳过 publish（用现有产物；门结论只对这份产物成立）" -ForegroundColor Yellow
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 没有可用产物（缺 $dist\app.json）：去掉 -SkipBuild 或先跑一次构建。" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-dist"
            exit 2
        }
    }

    # 3) **产物合法性 + appid 前置断言（#1210）** —— 判据面单点真源在 `scripts/lib/mp-weixin-product.ps1`。
    #    票面病根：旧实现用**正则**从 `project.config.json` 抠 appid ⇒ **非法 JSON 也能通过 appid 校验**；
    #    而这类坏产物会让开发者工具白屏 / `pageStack` 永不应答 / IDE 日志反复 `routeTo appLaunch timeout`，
    #    门最终只报一个**看起来像环境问题**的 `page-stack-not-answering`，把真正的病根藏起来。
    #    判据：必需文件齐（project.config.json / app.json / app.js / app.wxss）+ 两份 JSON 可解析
    #    （非法即打印**位置原文**）+ appid 由**解析后取值** + `app.json.pages` 每页有 `<page>.js`。
    #    失败 = **产物坏了 ⇒ 门未过（exit 1）**，与「环境不可用（exit 2）」严格分开。
    . (Join-Path $PSScriptRoot 'lib\mp-weixin-product.ps1')
    $product = Get-MpWeixinProductReport -Dist $dist
    Write-Log "产物合法性 = $($product.detail)"
    Write-Host "产物合法性 = $($product.detail)"
    if (-not $product.ok) {
        Write-Host "[error] 产物不合法（stage=$($product.stage)）：$($product.detail)" -ForegroundColor Red
        Write-Host '        这是**产物坏了**（构建被中断 / 写盘不完整），不是环境不可用 ⇒ 判门未过（exit 1）。' -ForegroundColor Red
        Write-Host '        处置：先 `Remove-Item -Recurse -Force <产物目录>` 干净重建（`npm run build:mp-weixin-buildonly`），再重跑本检查。' -ForegroundColor Red
        Write-Host '        排查提示：产物坏掉时开发者工具常见白屏 + `routeTo appLaunch timeout`，那不是「环境/锁/端口」问题。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=$($product.stage) detail=$($product.detail)"
        exit 1
    }

    # 3.5) appid 断言 —— 实测：产物 appid 一律取自 manifest.json 的 mp-weixin.appid；
    #      manifest 缺该项时落成 touristappid（游客 appid）。CLI 的 --appid **不改**产物。
    $productAppId = [string]$product.appid
    Write-Log "产物 appid = '$productAppId'（期望 '$AppId'）"
    Write-Host "产物 appid = $productAppId（期望 $AppId）"
    if ($AppId -and $productAppId -ne $AppId) {
        if ($BuildOnly) {
            # -BuildOnly 下这条也**不断言**（与源 manifest 那条同源同因）：产物 appid 由 manifest 决定，
            # 回写后必然 touristappid；本模式不上传 ⇒ 无害，且产物结构判据（#1210）已照跑。
            Write-Host ">>> -BuildOnly：产物 appid='$productAppId' 与期望 '$AppId' 不一致 —— 本模式不做 appid 断言（不上传；产物结构判据已过）" -ForegroundColor Yellow
            Write-Log "build-only: product-appid 不断言 product='$productAppId' expected='$AppId'"
        } else {
            Write-Host "[error] 产物 appid 与期望不一致：产物=$productAppId 期望=$AppId" -ForegroundColor Red
            Write-Host '        appid 来源是 manifest.json 的 mp-weixin.appid（不是本脚本的 -AppId，也不是 CLI 的 --appid）；' -ForegroundColor Red
            Write-Host '        产物落成 touristappid = manifest 的 mp-weixin.appid 为 null/缺失（游客态）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=appid product=$productAppId expected=$AppId"
            exit 1
        }
    }

    # 3.6) **发行开关 + 主包字节数（#2417）** —— 判据取**产物**，不取仓根配置。
    #      为什么不取仓根：仓根 project.config.json / manifest.json 改对了、产物却没带（构建没重来 /
    #      HBuilderX 没把仓根那份复制进产物）时，上传面板的「JS 压缩」「组件按需注入」照样红 ⇒
    #      断言源码只会给出假绿。为什么排在这里：产物合法性过了才谈得上读开关（坏产物已在 3) 拦掉）。
    #      失败行各自一个 stage（lazy-code-loading / minified / es6 / main-package）＋ actual= 实读值
    #      ⇒ 一眼分辨是哪一条坏的，不用去翻日志找上下文。
    $artifact = Get-MpWeixinArtifactReadings -Dist $dist
    if (-not $artifact.ok) {
        Write-Host "[error] 产物开关读数失败（stage=$($artifact.stage)）：$($artifact.detail)" -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=$($artifact.stage) detail=$($artifact.detail)"
        exit 1
    }
    Write-Log "产物读数 = $($artifact.detail)"
    Write-Host "产物读数 = $($artifact.detail)"

    # ① 组件按需注入：值必须是 requiredComponents（缺字段与写错值都判失败 —— 缺了等于没开）
    if ("$($artifact.lazyCodeLoading)" -ne 'requiredComponents') {
        $lazyActual = Format-ProductSwitchValue $artifact.lazyCodeLoading
        Write-Host "[error] 产物 app.json 的 lazyCodeLoading 为 $lazyActual（期望 requiredComponents）" -ForegroundColor Red
        Write-Host '        组件按需注入是上传面板的【必须】项；开关写在 manifest.json 的 mp-weixin 节点内才会进产物。' -ForegroundColor Red
        Write-Host '        处置：改 manifest.json 的 mp-weixin.lazyCodeLoading 后重跑一次发行构建（产物没重来就别指望变）。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=lazy-code-loading actual=$lazyActual detail=产物 app.json 缺 lazyCodeLoading 或值不是 requiredComponents（须在 manifest.json 的 mp-weixin 节点内声明）"
        exit 1
    }

    # ② JS 压缩：minified 必须是 JSON 布尔 true（字符串 "true" 会被微信按布尔读成没开 ⇒ 判失败）
    if (-not (Test-ProductSwitchBool $artifact.minified $true)) {
        $minActual = Format-ProductSwitchValue $artifact.minified
        Write-Host "[error] 产物 project.config.json 的 setting.minified 为 $minActual（期望 true）" -ForegroundColor Red
        Write-Host '        JS 压缩是上传面板的【必须】项，且只在**上传时**生效（本地产物字节不因此变化，故必须看开关本身）。' -ForegroundColor Red
        Write-Host '        处置：改仓根 project.config.json 的 setting.minified（HBuilderX 会原样复制进产物）后重跑构建。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=minified actual=$minActual detail=产物 project.config.json 的 setting.minified 不为 true（仓根 project.config.json 须为 true，HBuilderX 原样复制进产物）"
        exit 1
    }

    # ③ es6 降级必须关：开成 true 会把代码降级成 ES5（体积反而涨），而面板只查「JS 是否被压缩」
    if (-not (Test-ProductSwitchBool $artifact.es6 $false)) {
        $es6Actual = Format-ProductSwitchValue $artifact.es6
        Write-Host "[error] 产物 project.config.json 的 setting.es6 为 $es6Actual（期望 false）" -ForegroundColor Red
        Write-Host '        es6 打开会把代码降级编译成 ES5：体积涨、面板的「JS 压缩」也不看它 —— 本仓裁定保持 false。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=es6 actual=$es6Actual detail=产物 project.config.json 的 setting.es6 不为 false（本仓保持 es6=false，降级成 ES5 只会让包更大）"
        exit 1
    }

    # ④ 主包字节数：2 MiB 硬限判失败；1.5 MiB 只是**建议**线 ⇒ 只打印读数与「权威读数是上传面板」，不判红
    $mainVerdict = if ($artifact.mainBytes -gt $MainHardLimitBytes) { 'over' } else { 'ok' }
    $advisory = if ($artifact.mainBytes -gt $MainAdvisoryBytes) { 'over' } else { 'under' }
    Write-Log "main-package mainBytes=$($artifact.mainBytes) totalBytes=$($artifact.totalBytes) files=$($artifact.files) subPackages=$($artifact.subPackages) advisory1.5m=$advisory hard2m=$mainVerdict limit=$MainHardLimitBytes"
    Write-Host "主包字节数 = $($artifact.mainBytes) B（全量 $($artifact.totalBytes) B / $($artifact.files) 文件 / 分包 $($artifact.subPackages) 个；1.5 MiB 建议线 $MainAdvisoryBytes / 2 MiB 硬限 $MainHardLimitBytes）"
    # 建议线口径**无条件**写进日志与输出（AC ④）：不超线时也要说明「这条线不判红、权威读数在面板」，
    # 否则读者只看到 advisory1.5m=under 会以为本地读数就是面板读数。
    $advisoryNote = '1.5 MiB 是建议线、不判红：1.5 M 的权威读数是上传面板（minified 上传时才压缩，本地产物字节不变），本地读数只作参照'
    Write-Log "main-package advisory1.5m=$advisory note=$advisoryNote"
    if ($advisory -eq 'over') {
        Write-Host "        已超 1.5 MiB 建议线 —— $advisoryNote" -ForegroundColor Yellow
    } else {
        Write-Host "        $advisoryNote"
    }
    if ($mainVerdict -ne 'ok') {
        Write-Host "[error] 主包 $($artifact.mainBytes) B 超过 2 MiB 硬限 $MainHardLimitBytes B —— 微信上传会直接拒" -ForegroundColor Red
        Write-Host '        处置：分包（把非必需页移出主包）或裁剪主包静态资源；本检查不做分包动作。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=main-package mainBytes=$($artifact.mainBytes) limit=$MainHardLimitBytes detail=主包超过 2 MiB 硬限（微信上传会拒）须先分包或裁剪资源"
        exit 1
    }

    # 指纹落盘（#1860）：只有**本轮真构建过**（走到这里时 -SkipBuild 仍为 false = 没被指纹/人工跳过）
    # 且 3)–3.6) 的 exit 1 断言（产物合法性 / appid / 三枚开关 / 主包硬限）都过了才写 —— 产物坏不写（下次必然重建）。
    # 写失败只警告，不影响门结论（fail 向重建：没写成 = 下次没有指纹 = 重建）。
    if (-not $SkipBuild) {
        try {
            $fpSave = Save-MpWeixinBuildFingerprint -Project $Project -Dist $dist -HBuilderXVersion $fpVersion -BuildCommand $buildCommandKey
            if ($fpSave.Saved) {
                Write-Log "build-fingerprint saved $($fpSave.Detail)"
                Write-Host ">>> 产物指纹已落盘（下次同树重跑命中即自动 -SkipBuild）"
            } else {
                Write-Log "build-fingerprint NOT saved reason=$($fpSave.Reason)（$($fpSave.Detail)）"
                Write-Host "[warn] 产物指纹未落盘（$($fpSave.Reason)）—— 下次重跑将重建（fail 向重建）" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "[warn] 产物指纹落盘异常（不影响门结论，下次将重建）：$_" -ForegroundColor Yellow
            Write-Log "build-fingerprint save exception: $_"
        }
    }

# ---------- 构建面收口 ----------
# 走到这里说明构建面的判据都成立：publish 成功行已解析、产物合法性（#1210）已过、指纹已落盘。
# 会话段已于 2026-11-05 整体退役（#2320），其后不再有任何「之后整段」⇒ 这里就是脚本唯一的收口点。
# 原「if ($BuildOnly) 收口壳」随会话段一并退役（#2326）：条件已恒真；留着反而会让不带 -BuildOnly
# 的调用跳过收口、落到 finally 之后的 exit $exitCode（= 初值 2），把「构建面通过」判成 env-blocked。
# exit 0 位于 try 体内：实测照常触发 finally 的 Invoke-DevToolsClose + Release-HxLock（旧红线继承）。
$buildMode = if (-not $SkipBuild) { 'publish' } elseif ($script:buildSkippedByFingerprint) { 'skipped(fingerprint)' } else { 'skipped(explicit)' }
$buildValues = [ordered]@{
    appid   = "$productAppId"
    pages   = "$($product.pages)"
    product = 'ok'
    # 三枚开关读数取**产物实读值**（不是仓根配置值）：只有产物带着才算数（见 3.6 段）
    lazyCodeLoading = (Format-ProductSwitchValue $artifact.lazyCodeLoading)
    minified = (Format-ProductSwitchValue $artifact.minified)
    es6      = (Format-ProductSwitchValue $artifact.es6)
    build   = $buildMode
    elapsed = $(if (-not $SkipBuild) { "${publishSeconds}s" } else { 'n/a' })
    mainBytes = "$($artifact.mainBytes)"
    # 硬限判定取 3.6 段算出的同一枚 $mainVerdict（超限在那里就 exit 1）—— 不写死 'ok'，
    # 否则 3.6 段一旦移位/条件化，结果行会替一个没做过的判定作证。
    main2m  = $mainVerdict
    log     = $logRelative
}
$buildLine = "$($script:GatePlan.resultLine.prefix) " + (Format-GateMetricLine -Plan $script:GatePlan -Values $buildValues)
Write-Log ([string][char]10 + $buildLine)
Stop-GateStageIfOpen
Write-Host ''
Write-Host "✅ mp-weixin 构建面通过：$buildLine" -ForegroundColor Green
Write-Host '   本检查只证明「publish 成功 + 产物合法」，**不含**运行时取证（console error / exception / 页面栈一概不取证）。' -ForegroundColor Yellow
Write-Host '   ② 微信开发者工具门已于 2026-11-05 整体退役（#2320）：需要看运行时请手动诊断，不作为验收证据。' -ForegroundColor Yellow
exit 0
} catch {
    Write-Host "[error] mp-weixin 构建面脚本异常：$_" -ForegroundColor Red
    Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=exception"
    $exitCode = 2
} finally {
    # publish 尾段会自启开发者工具：收掉那个自启实例靠这一次 close（幂等；失败只警告）。
    Invoke-DevToolsClose
    if (Get-Command Release-HxLock -ErrorAction SilentlyContinue) { Release-HxLock }
    Write-BuildOnlyWorktreeHint
}
exit $exitCode
