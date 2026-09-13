<#
.SYNOPSIS
    移动端 UI 迭代的「日常增量运行」载体（快内循环）：HBuilderX 编译 -> 运行到手机 -> 分段计时。

.DESCRIPTION
    **分工（与既有「门」脚本划清边界）**
      - `scripts/compile-check.ps1`（④a）＝ **门**：加干净缓存重建开关的全量 dev 编译，实测 4.8–8 分钟；**默认值不要改**。
      - `scripts/kotlin-all-check.ps1`（④c）＝ 门：整模块 kotlinc（本地加固，不替代 ④b）。
      - `scripts/mp-weixin-check.ps1`（②）＝ 门：微信开发者工具无报错。
      - **本脚本＝日常增量**：只为「改一行样式/模板后想马上在真机上看到」服务，**不是门、不进 `## 验收证据`**。
        实测口径：全量 4.8–8 分钟 vs 增量约 1–2 分钟；资源导出 publish 98 秒–3 分钟；
        基座 APK（95.7 MB）**只需装一次**，所以本脚本**不重装基座**。
      - 三层节奏（内 / 中 / 外循环）的规范见移动端 `AGENTS.md`「开发内循环（移动端 UI 迭代）」；
        ADR-0008 只在该段留一行指针。

    **默认行为（每条都是刻意的）**
      0. **真运行，不是「仅编译」**：`launch app-android` 的 `--compile` 官方语义是「**仅编译代码**」（默认 false），
         所以本脚本**默认一律不传 `--compile`**——传了它就只编译、不运行（`--compile true` 只允许出现在
         `-CompileOnly` 分支里；契约测试 C13 守护，全量编译门 `compile-check.ps1` 另有一份）。
         **但「仅编译」正是拿编译期诊断的捷径** ⇒ 见下面的 `-CompileOnly` 与「日常循环分层」。
      1. **默认增量**：不加「干净缓存重建」开关——该字面量**只出现在 `if ($Full)` 分支里**（契约测试 hxRunContract C1 守护）。
      2. **不重装基座**：脚本里没有任何 `adb install`；「运行到手机」时的安装由 HBuilderX 自己处理。
      3. **忙就等，但只等一小会儿**：dot-source `scripts/lib/hx-busy.ps1` 后调 `Wait-HxFree`（agent 互斥锁 +
         主程序忙探测 + 等待上限 `-WaitSeconds`，**默认 120 秒**）；**超时 ⇒ exit 2（环境不可用）**，
         由调用者决定何时重试 —— **不无声等十分钟**（2026-09-13 实测：某次纯排队等了 ~10 分钟）。
      4. **绝不 kill 任何进程**：本脚本**不含** `Stop-Process` / `taskkill` / 任何强杀调用（契约测试 C2 守护）；
         HBuilderX 是单实例串行资源，**GUI 优先**（ADR-0008 坑位段）。某步超时也**不终止**已派生的 cli 子进程
         （cli 只是驱动主程序的客户端，会自行退出），只判「环境不可用」并 exit 2。

    **日常循环分层（2026-09-13 定，起因见 #949）**
      | 目的 | 载体 | 代价 |
      | --- | --- | --- |
      | 拿**编译期诊断**（类型、uvue 样式规则、模板编译错误） | **`-CompileOnly`** | ≈ 编译时间（实测 205–261 秒），**不推送/不启动/不轮询/不需要设备** |
      | **肉眼看样式** | 你在 HBuilderX GUI 里跑「运行到手机」+ 热刷新 | 秒级～1 分钟（`AGENTS.md` 原记；本会话未实测） |
      | **① 真机取证 / 收口** | 本脚本**默认模式**（真运行 + 设备侧基线判据） | 编译 + 推送 + 启动，实测 236–287 秒 |
      **为什么要分层**：2026-09-13 实测那次会话，慢的**不是编译** —— 是两次「编译期诊断拖到真机才发现」的返工、
      一次 ~10 分钟的锁排队、一次 ~20 分钟的 launch 卡死。编译（205–261 秒）是编译器的固定成本，
      **返工却是可以消除的**：那两条诊断（`MenuItem` 类型名义重复 → `ClassCastException`；`<view>` 上用了
      `text-align|font-size|color`）**用 `-CompileOnly` 第一次就会红在本地**。

    **判成败只解析 stdout**：HBuilderX CLI 失败时**退出码恒为 0**（ADR-0008 实测四处失败全返回 0），
    所以本脚本一律看输出文本，不依赖退出码。

    **分段计时**：从 launch 步的 stdout 里抽「带时间戳的行」，算「编译段 / 部署段」；再加各步墙钟。
    机检格式（便于日志 / 评论里核对时间花在哪）：
        HX_RUN mode=incremental|full compile=<s> deploy=<s> total=<s> exit=ok|fail|env|dryrun
    **分段计时的诚实声明（2026-09-12 已首跑）**：标记表（`$script:HxCompileEndMarkers`）在 2026-09-12 的真机
    首跑里命中了 `编译成功`；未命中时仍退化为「launch 步墙钟即编译段、部署段 0」并打印 `segment_source=wallclock`。
    **但分段计时只说明时间花在哪，不构成「已部署」的证据**（见下条）。

    **假绿教训 v1（2026-09-12；勿删）**：**编译成功 ≠ 运行成功**。实测 HBuilderX 会连续打出
        `… 编译成功。` → `ready in 223549ms.` → `已停止运行...`
    整条链路**没有任何东西到达设备**（设备上目标包的 `lastUpdateTime` 仍是旧日期），而旧版脚本因为
    「没扫到 error 行」就报 `exit=ok` + 退出码 0 ⇒ 有人据它宣布「已编译并运行到设备」，把**旧构建的截图**
    当成 ①a 取证入库（证据污染，已在对应 PR 里撤回）。**这就是本仓最忌讳的假绿。**

    **假绿教训 v2（2026-09-13；根因是**本脚本自己**，v1 的处置反而又造了一个假绿，勿删）**：
      ① **根因**：本脚本从 v1 起就把 `--compile true` 写死在 launch 参数里（照抄全量编译门），而该参数的官方语义是
         「**仅编译代码**」⇒ 它**从未请求过运行**。于是 `编译成功 → ready in … → 已停止运行...` 不是失败，是
         **一次仅编译调用的正常收场**；「运行到手机在编译成功后立刻停止」这个事故描述从头就建立在错前提上。
         单变量实验（只去掉这一个参数）：设备 `…/apps/<appid>/www` 的 mtime 由 13:11 前进到 **14:10**、基座进程重启
         ⇒ 部署成立。**脚本参数的语义要按官方帮助逐字读，不要按字面猜。**
      ② **v1 的处置失效**：`Get-DeployedState` 拿 `topResumedActivity` 判「部署成功」——可是**重复运行到同一台机器时
         基座本来就在前台**（本次它从 11:38 一直开着）⇒ 在**最常见**的场景下该判据恒为真，一个字节没推也报
         `deployed=true` + `exit=ok`。**这就是 v2 的假绿：判据没有基线。**
      ③ **处置**：判据改成**基线相对**的设备侧事实 —— 运行前先记基线（资源目录 mtime / 目标包进程 pid），
         运行后比对，只有**发生变化**才算部署；并且**主判据是「资源真落盘」**（推送会发生），
         `topResumedActivity` 一律只作**辅助说明**、不作判据。机检行同时报基线与现值：
        HX_RUN_DEPLOY deployed=true|false www_before=<epoch> www_after=<epoch> pid_before=<pid> pid_after=<pid> reason=<说明>
    `deployed=false` ⇒ 判**未部署**：`exit=env` + 退出码 2（既不是成功，也不是「代码写错了」）。
    候选包名 = `manifest.json` 的 appid（`__UNI__XXXX` → `uni.app.XXXX`）+ HBuilderX 标准基座 `io.dcloud.uniappx`
    （dev 运行的常见承载）；资源目录候选由这些包名派生，取**实际存在**的那些。

    **真运行会话常驻，但下一次 launch 会顶掉旧的（2026-09-13 实测，勿删）**：不传 `--compile` 的**真运行** `cli.exe`
    **不会自己返回**（实测一个会话活了 **51 分钟**，期间每约 10 分钟重试一次 `wakeUpDevice`；那次重试被设备以
    `INJECT_EVENTS` 拒绝，**非致命**，部署照常完成）；它最终是在**下一次 `launch` 发出后约 10 秒**打出
    `已停止运行...` 才收口 ⇒ **不需要手工清理**。只有「仅编译」调用才会自己立刻收口。
    ⇒ 本脚本对 launch 步**不等待进程退出**：发出去 → **有界轮询设备侧事实**（`-TimeoutSeconds`，默认 900 秒）→
    判完即返回，**既不等待也不终止**那个会话（要提前停就由人在 HBuilderX 里点停止）。
    **快速失败（2026-09-13 加，起因 #949）**：编译段已结束（命中 `$script:HxCompileEndMarkers`）之后，
    若设备侧事实在 `-DeployStallSeconds`（默认 300 秒）内**毫无前进** ⇒ **提前判环境不可用**，
    不再等满 `-TimeoutSeconds`（2026-09-13 实测某次 launch 卡死，白等了 ~20 分钟才到上限）。
    部署耗时实测：冷启 HBuilderX + 增量编译 205–610 秒，资源推送到落盘 15 秒–9 分钟（视缓存与设备而定）。

    **退出码**：0 = 运行到手机成功（设备侧事实已前进）/ 仅编译干净（无编译期诊断行）；
    1 = 输出含 error 行（编译 / 运行失败）；2 = 环境不可用（cli 或 adb 缺失、多设备未显式指定、
    与主程序连接中断、忙等待超时、部署停滞后判环境不可用、轮询超时仍未见到部署事实）。

.PARAMETER Project
    项目根目录。缺省为本脚本上一级目录（scripts/ 位于项目根下）。

.PARAMETER Device
    目标设备序列号（`adb devices` 第一列）。缺省自动取**唯一在线设备**；**有多个设备时必须显式指定，否则 exit 2**。
    ⚠️ **同一台机器可能以两个入口出现**（TCP `<ip>:<port>` 与 mDNS `adb-<serial>-…._adb-tls-connect._tcp`，
    `serialno` 相同）⇒ 那种情况下必须显式 `-Device`，否则会被本守卫判成"多设备"而 exit 2（2026-09-13 实测踩到）。
    本参数解析出的序列号会**显式传给** `launch --deviceId`（2026-09-13 实测该参数存在且有效）。

.PARAMETER HBuilderX
    HBuilderX 安装目录（在其中找 `cli.exe`）。与 `-Cli` 二选一，`-Cli` 优先。

.PARAMETER Cli
    `cli.exe` 路径。**显式给出即权威**：不可用就 exit 2，不再回退自动探测。
    缺省按 `$env:HBuilderX_CLI` -> 正在运行的 HBuilderX 同目录 -> 常见安装位置探测（校验 PE 头，跳过损坏副本）。

.PARAMETER Full
    **只有显式给**才走全量（加干净缓存重建开关）。日常迭代**不要**给：全量 4.8–8 分钟，
    而且那是全量门 `compile-check.ps1` 的活，不是本脚本的。与 `-CompileOnly` 互斥。

.PARAMETER CompileOnly
    **只编译、拿编译期诊断**（2026-09-13 加，起因 #949）：底层是 `--compile true` 的官方语义「仅编译代码」——
    **不推送、不启动、不轮询、不需要设备**，且该调用**会自己收口**（见脚本头「假绿教训 v2」①）。
    用途：**在真机链路之前**把类型 / uvue 样式规则 / 模板编译错误拦在本地（实测 ≈ 编译时间）。
    机检行 `mode=compile-only`；退出码 0 = 无诊断行、1 = 有诊断行、2 = 环境不可用。

.PARAMETER WaitSeconds
    `Wait-HxFree` 的忙等待上限（**默认 120 秒**，2026-09-13 由 600 下调）。超时 ⇒ exit 2；
    **绝不抢占主程序、绝不 kill**。下调理由：忙时无声等 10 分钟比"快速失败 + 调用者决定何时重试"更贵。

.PARAMETER DryRun
    只打印将要执行的命令行，**完全不调用 HBuilderX、不调 adb、不写日志**（零副作用，任何机器上都能跑）。

.PARAMETER StepTimeoutSeconds
    `open` / `project open` 两小步的超时秒数（默认 180）。超时**不强杀**，判环境不可用并 exit 2。

.PARAMETER TimeoutSeconds
    部署轮询上限（**默认 900 秒**，2026-09-13 由 1800 下调）：launch 发出后最多等这么久去观察**设备侧事实**是否前进。
    超时**不强杀**任何进程（真运行会话本来就不返回），判未部署 ⇒ 环境不可用并 exit 2。
    `-CompileOnly` 模式下它是"仅编译"那一步的超时。

.PARAMETER DeployStallSeconds
    **部署停滞阈值（默认 300 秒）**：编译段已结束 + 该秒数内设备侧事实无前进 ⇒ **提前判环境不可用**（exit 2），
    不等满 `-TimeoutSeconds`。实测正常推送只需 15–21 秒，所以 300 秒已很宽松。

.PARAMETER PollSeconds
    部署轮询间隔（默认 10 秒）。

.PARAMETER LogPath
    日志路径（默认 `<项目>/.ci-verify/hx-run.log`，被 .gitignore 的 `.ci-verify/` 覆盖，不入库）。

.EXAMPLE
    npm run hx:run                                            # 日常增量（推荐）
    npm run hx:compile-only                                   # 只编译：拿编译期诊断，不碰设备（#949）
    pwsh -NoProfile -File scripts/hx-run.ps1 -DryRun           # 只看将要跑什么，零副作用
    pwsh -NoProfile -File scripts/hx-run.ps1 -Device emulator-5554
    pwsh -NoProfile -File scripts/hx-run.ps1 -Full             # 偶尔要全量时（仍是「运行」，不是门）
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$Device,
    [string]$HBuilderX,
    [string]$Cli,
    [switch]$Full,
    [int]$WaitSeconds = 120,
    [switch]$DryRun,
    [switch]$CompileOnly,
    [int]$StepTimeoutSeconds = 180,
    [int]$TimeoutSeconds = 900,
    [int]$DeployStallSeconds = 300,
    [int]$PollSeconds = 10,
    [string]$LogPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------- 常量 ----------

# 机检行：契约测试 hxRunContract C5 按 `HX_RUN mode=` 锚点守护
$script:HxRunLineFormat = 'HX_RUN mode={0} compile={1} deploy={2} total={3} exit={4}'

# 「编译段结束」标记表。**未用真实 HBuilderX stdout 校准**（本 PR 未跑 HBuilderX）；
# 未命中即退化 wallclock 口径（见头注释「诚实声明」），真机首跑后按真实输出校准这里即可。
$script:HxCompileEndMarkers = @(
    '编译完成', '编译结束', '编译成功', '正在安装', '开始安装', '安装中',
    '运行到手机', '正在运行', '启动App', 'compile success', 'compile finished', 'built in'
)

function Test-HxCompileFinished {
    <# 编译段是否已结束（命中结束标记表）—— 用于部署停滞的快速失败判据（#949）。 #>
    param([string]$Output)
    foreach ($m in $script:HxCompileEndMarkers) {
        if ("$Output".Contains($m) -or "$Output".ToLower().Contains($m.ToLower())) { return $true }
    }
    return $false
}

function Get-HxErrorLines {
    <#
      判成败只解析 stdout（CLI 退出码恒 0）：扫 error / 编译失败 一类字样，再排除「0 error」这类否证行。
      仅编译与真运行两条路径共用同一判据（#949 抽函数，避免两处漂移）。
    #>
    param([string]$Output)
    return @("$Output" -split "`r?`n" |
        Where-Object { $_ -match '(?i)\berror\b|unresolved reference|cannot infer type|找不到名称|类型不匹配|编译失败|运行失败' } |
        Where-Object { $_ -notmatch '(?i)0\s*error|errors?\s*[:=]\s*0|no errors?|error count\s*[:=]\s*0' })
}

# ---------- 通用函数 ----------

function Test-PeHeader {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $fs = [System.IO.File]::OpenRead($Path)
        try { $buf = New-Object byte[] 2; $read = $fs.Read($buf, 0, 2) } finally { $fs.Dispose() }
    } catch {
        return $false
    }
    # 'MZ'：损坏的残留副本读回来前两字节常为 00，必须排除
    return ($read -eq 2 -and $buf[0] -eq 0x4D -and $buf[1] -eq 0x5A)
}

function Resolve-CliPath {
    param([string]$Explicit, [string]$HxDir, [switch]$SkipRunningProbe)
    # 1) -Cli 显式即权威
    if ($Explicit) {
        if (Test-PeHeader -Path $Explicit) { return (Resolve-Path -LiteralPath $Explicit).Path }
        return $null
    }
    # 2) -HBuilderX 安装目录
    if ($HxDir) {
        foreach ($rel in @('cli.exe', 'HBuilderX\cli.exe')) {
            $c = Join-Path $HxDir $rel
            if (Test-PeHeader -Path $c) { return (Resolve-Path -LiteralPath $c).Path }
        }
    }
    # 3) 环境变量
    if ($env:HBuilderX_CLI -and (Test-PeHeader -Path $env:HBuilderX_CLI)) {
        return (Resolve-Path -LiteralPath $env:HBuilderX_CLI).Path
    }
    # 4) 正在运行的 HBuilderX 同目录（版本必然匹配）。-SkipRunningProbe 时不查进程（DryRun 零副作用）
    if (-not $SkipRunningProbe) {
        try {
            $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        } catch {
            $proc = $null
        }
        if ($proc) {
            try {
                $runningCli = Join-Path (Split-Path $proc.Path) 'cli.exe'
                if (Test-PeHeader -Path $runningCli) { return (Resolve-Path -LiteralPath $runningCli).Path }
            } catch {
                # 进程路径读不到（权限）就继续走位置探测
            }
        }
    }
    # 5) 常见安装位置
    $found = @()
    foreach ($pattern in @(
            'D:\软件\HBuilderX*\HBuilderX\cli.exe',
            'D:\HBuilderX*\HBuilderX\cli.exe',
            'C:\Program Files\HBuilderX*\HBuilderX\cli.exe',
            'C:\Program Files\HBuilderX\cli.exe',
            "$env:LOCALAPPDATA\HBuilderX\cli.exe")) {
        $found += (Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | ForEach-Object { $_.FullName })
    }
    foreach ($c in $found) {
        if (Test-PeHeader -Path $c) { return (Resolve-Path -LiteralPath $c).Path }
        Write-Host "[warn] 跳过不可用的 cli.exe 候选：$c（缺失或 PE 头损坏）" -ForegroundColor Yellow
    }
    return $null
}

function Resolve-AdbExe {
    $cands = @()
    foreach ($root in @($env:ANDROID_SDK_ROOT, $env:ANDROID_HOME)) {
        if ($root) { $cands += (Join-Path $root 'platform-tools\adb.exe') }
    }
    $cands += 'D:\android-sdk\platform-tools\adb.exe'
    foreach ($c in $cands) {
        if ($c -and (Test-Path -LiteralPath $c -PathType Leaf)) { return $c }
    }
    $cmd = Get-Command adb -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    return $null
}

function Get-OnlineDeviceList {
    param([string]$AdbExe)
    $out = (& $AdbExe devices 2>&1 | Out-String)
    $list = @()
    foreach ($line in ($out -split "`r?`n")) {
        if ($line -match '^\s*(\S+)\s+(device|offline|unauthorized|no permissions)\s*$') {
            $list += [pscustomobject]@{ Serial = $Matches[1]; State = $Matches[2] }
        }
    }
    return @($list)
}

function Resolve-TargetDevice {
    # 缺设备 / 多设备未指定 ⇒ exit 2（环境不可用），与 hx-busy 的失败语义一致
    param([string]$Requested, [string]$AdbExe)
    $all = Get-OnlineDeviceList -AdbExe $AdbExe
    $online = @($all | Where-Object { $_.State -eq 'device' })
    if ($Requested) {
        $hit = @($all | Where-Object { $_.Serial -eq $Requested })
        if ($hit.Count -eq 0) {
            Write-Host "[error] -Device $Requested 不在 adb 设备列表里（当前：$(@($all | ForEach-Object { $_.Serial }) -join ', ')）" -ForegroundColor Red
            exit 2
        }
        if ($hit[0].State -ne 'device') {
            Write-Host "[error] -Device $Requested 状态是 $($hit[0].State)（不是 device）：先解授权/重连再跑。" -ForegroundColor Red
            exit 2
        }
        return @{ Serial = $Requested; Mode = 'explicit' }
    }
    if ($online.Count -eq 0) {
        Write-Host '[error] 没有在线安卓设备（adb devices 无 device 行）：先连真机/开仿真机，或用 -Device 指定。' -ForegroundColor Red
        exit 2
    }
    if ($online.Count -gt 1) {
        Write-Host "[error] 有 $($online.Count) 个在线设备，**必须显式指定** -Device（否则可能装到错的设备上）：" -ForegroundColor Red
        Write-Host "        候选：$(@($online | ForEach-Object { $_.Serial }) -join ' ')" -ForegroundColor Red
        exit 2
    }
    return @{ Serial = $online[0].Serial; Mode = 'auto' }
}

function Format-ArgvLine {
    param([string]$Exe, [string[]]$CliArgs)
    $parts = @('"' + $Exe + '"')
    foreach ($a in $CliArgs) {
        if ("$a" -match '\s') { $parts += '"' + $a + '"' } else { $parts += "$a" }
    }
    return ($parts -join ' ')
}

function Get-HxSegment {
    <#
      从 launch 步 stdout 抽带时间戳的行，算编译段 / 部署段。
      返回 @{ Compile = <秒>; Deploy = <秒>; Source = 'stdout' | 'wallclock' }
      未命中（无时间戳或未命中结束标记）⇒ 退化为「launch 步墙钟即编译段、部署段 0」。
    #>
    param([string]$Output, [int]$LaunchSeconds)
    $stamped = @()
    foreach ($line in ("$Output" -split "`r?`n")) {
        if ($line -match '^\s*\[?(\d{1,2}):(\d{2}):(\d{2})(?:[.,](\d{1,3}))?\]?\s') {
            $frac = 0.0
            if ($Matches[4]) { $frac = [double]("0." + $Matches[4].PadRight(3, '0')) }
            $t = [int]$Matches[1] * 3600 + [int]$Matches[2] * 60 + [int]$Matches[3] + $frac
            $stamped += [pscustomobject]@{ T = $t; Line = $line }
        }
    }
    $fallback = @{ Compile = $LaunchSeconds; Deploy = 0; Source = 'wallclock' }
    if ($stamped.Count -lt 2) { return $fallback }
    $endMark = $null
    foreach ($s in $stamped) {
        foreach ($m in $script:HxCompileEndMarkers) {
            if ($s.Line -like "*$m*" -or $s.Line.ToLower().Contains($m.ToLower())) { $endMark = $s.T; break }
        }
        if ($null -ne $endMark) { break }
    }
    if ($null -eq $endMark) { return $fallback }
    $first = $stamped[0].T
    $last = $stamped[$stamped.Count - 1].T
    $compile = [int][Math]::Round($endMark - $first)
    $deploy = [int][Math]::Round($last - $endMark)
    if ($compile -lt 0) { $compile = 0 }
    if ($deploy -lt 0) { $deploy = 0 }
    return @{ Compile = $compile; Deploy = $deploy; Source = 'stdout' }
}

function Invoke-CliStep {
    <#
      派发一步 cli 调用：只读 stdout（含 stderr），带超时。
      **超时不强杀**：只把 Exited 置 false 让调用方判「环境不可用」。
      判成败一律看返回的 Output 文本，不看退出码。
    #>
    param([string]$Name, [string[]]$CliArgs, [int]$StepTimeout, [string]$CliExe, [string]$LogDir, [string]$LogFile)
    $display = Format-ArgvLine -Exe $CliExe -CliArgs $CliArgs
    Add-Content -LiteralPath $LogFile -Value "`n>>> $display" -Encoding utf8
    Write-Host ">>> $display"

    $argLine = ($CliArgs | ForEach-Object { if ("$_" -match '\s') { '"' + $_ + '"' } else { "$_" } }) -join ' '
    $stamp = [guid]::NewGuid().ToString('N')
    $outFile = Join-Path $LogDir "step-$stamp.out"
    $errFile = Join-Path $LogDir "step-$stamp.err"

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $proc = Start-Process -FilePath $CliExe -ArgumentList $argLine -NoNewWindow -PassThru `
        -RedirectStandardOutput $outFile -RedirectStandardError $errFile
    $exited = $proc.WaitForExit($StepTimeout * 1000)
    $sw.Stop()
    $seconds = [int]$sw.Elapsed.TotalSeconds

    $out = ''
    if (Test-Path -LiteralPath $outFile) { $out += (Get-Content -LiteralPath $outFile -Raw) }
    if (Test-Path -LiteralPath $errFile) { $out += (Get-Content -LiteralPath $errFile -Raw) }
    Remove-Item -LiteralPath $outFile, $errFile -Force -ErrorAction SilentlyContinue

    if (-not $exited) {
        $out += "`n[timeout] 该步超过 $StepTimeout 秒未返回（HBuilderX 主程序可能不可达）。本脚本**不终止**该 cli 子进程（绝不 kill）。"
    }
    Add-Content -LiteralPath $LogFile -Value $out -Encoding utf8
    Write-Host $out
    return [pscustomobject]@{ Name = $Name; Output = "$out"; Exited = $exited; Seconds = $seconds; Display = $display }
}

function Start-CliLaunchDetached {
    <#
      launch 步专用：**派发后不等待**（真运行会话不返回，见脚本头「真运行会话常驻」）。
      输出重定向到日志目录里的文件，返回该文件路径，供后续扫 error 行 / 抽时间戳。
      **不 kill、不等待、不设超时** —— 那个会话什么时候结束，由人在 HBuilderX 里决定。
    #>
    param([string[]]$CliArgs, [string]$CliExe, [string]$LogDir, [string]$LogFile)
    $display = Format-ArgvLine -Exe $CliExe -CliArgs $CliArgs
    Add-Content -LiteralPath $LogFile -Value "`n>>> $display`n（本步**不等待收口**：真运行会话常驻；判据改为设备侧事实的相对变化）" -Encoding utf8
    Write-Host ">>> $display"
    $stamp = [guid]::NewGuid().ToString('N')
    $outFile = Join-Path $LogDir "launch-$stamp.out"
    $errFile = Join-Path $LogDir "launch-$stamp.err"
    $argLine = ($CliArgs | ForEach-Object { if ("$_" -match '\s') { '"' + $_ + '"' } else { "$_" } }) -join ' '
    $proc = Start-Process -FilePath $CliExe -ArgumentList $argLine -NoNewWindow -PassThru `
        -RedirectStandardOutput $outFile -RedirectStandardError $errFile
    return [pscustomobject]@{ Proc = $proc; OutFile = $outFile; ErrFile = $errFile; Display = $display; Started = (Get-Date) }
}

function Get-CapturedOutput {
    <# 读回被重定向的 stdout/stderr（会话仍在跑时也读得动，读到多少算多少）。#>
    param([string]$OutFile, [string]$ErrFile)
    $out = ''
    foreach ($f in @($OutFile, $ErrFile)) {
        if (-not $f) { continue }
        if (-not (Test-Path -LiteralPath $f)) { continue }
        try { $out += (Get-Content -LiteralPath $f -Raw -ErrorAction Stop) } catch { }
    }
    return "$out"
}

function Write-HxRunResult {
    param([string]$Mode, [int]$Compile, [int]$Deploy, [int]$Total, [string]$Exit, [string]$LogFile)
    $line = $script:HxRunLineFormat -f $Mode, $Compile, $Deploy, $Total, $Exit
    Write-Host $line
    if ($LogFile) { Add-Content -LiteralPath $LogFile -Value $line -Encoding utf8 -ErrorAction SilentlyContinue }
}

function Get-WwwCandidatePaths {
    <#
      资源落盘候选路径：由候选包名 + appid 派生（实测标准基座为 /sdcard/Android/data/<包名>/apps/<appid>/www）。
      取不到/不存在就为空 —— 空表示「无从判断」，不是「已部署」。
    #>
    param([string[]]$Packages, [string]$AppId)
    $paths = @()
    foreach ($pkg in @($Packages)) {
        if ($pkg -and $AppId) { $paths += ("/sdcard/Android/data/$pkg/apps/$AppId/www") }
    }
    return @($paths | Select-Object -Unique)
}

function Get-DeviceDeployFacts {
    <#
      基线相对判据的**取数**部分（2026-09-13 加，起因见脚本头「假绿教训 v2」）。
      取**设备侧事实**，与 HBuilderX 的措辞无关：
        - Www：候选资源目录的 mtime（epoch 秒，`stat -c %Y`）
        - Pid：候选包名对应进程的 pid
        - Foreground：当前前台包名 —— **仅作辅助说明，永不单独作判据**
          （重复运行到同一台机器时基座本来就在前台，拿它当判据会恒真 ⇒ 就是 v1 的假绿）
      运行前后各取一次，由 Get-DeployVerdict 比对。
    #>
    param([string]$AdbExe, [string]$Serial, [string[]]$WwwPaths, [string[]]$Packages)
    $www = @{}
    foreach ($w in @($WwwPaths)) {
        try {
            $out = ((& $AdbExe -s $Serial shell "stat -c %Y '$w'") 2>&1 | Out-String).Trim()
            if ($out -match '^\d+$') { $www["$w"] = [long]$out }
        } catch { }
    }
    $pids = @{}
    try {
        $psOut = ((& $AdbExe -s $Serial shell 'ps -A -o PID,NAME') 2>&1 | Out-String)
        foreach ($pkg in @($Packages)) {
            if (-not $pkg) { continue }
            $m = [regex]::Match($psOut, '(?m)^\s*(\d+)\s+' + [regex]::Escape($pkg) + '\s*$')
            if ($m.Success) { $pids["$pkg"] = "$($m.Groups[1].Value)" }
        }
    } catch { }
    $fg = ''
    try {
        $dump = ((& $AdbExe -s $Serial shell 'dumpsys activity activities') 2>&1 | Out-String)
        $m = [regex]::Match($dump, 'topResumedActivity=[^\r\n]*?\s([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)+)/')
        if ($m.Success) { $fg = $m.Groups[1].Value }
    } catch { }
    return [pscustomobject]@{ Www = $www; Pid = $pids; Foreground = $fg }
}

function Get-DeployVerdict {
    <#
      **基线相对**判定（v2 假绿教训）：只有与基线相比**发生变化**才算部署。
      主判据（唯一能判真的）：资源目录 mtime 前进 —— 实测**与内容是否变化无关**（13:11 与 14:10 两次推送，
      本地导出目录都没动，设备侧目录 mtime 照样前进），所以「没前进」只能解释成没有真推。
      旁证（**不判真**，只写进 reason）：目标包进程 pid 变化、`已停止运行` 一类停止标记。
      **失败方向刻意偏向「判未部署」**：误判未部署的代价是 exit 2（人再看一眼），误判已部署的代价是假绿
      —— 后者才是本仓最忌讳的。所以旁证一律不参与判真。
      返回 @{ Deployed=<bool>; Reason=<说明>; WwwAdvanced=@(); Restarted=@(); StopMarkers=@() }
    #>
    param($Before, $After, [string]$Stdout)
    $wwwAdvanced = @()
    foreach ($k in @($After.Www.Keys)) {
        if (-not $Before.Www.ContainsKey($k)) { $wwwAdvanced += $k; continue }
        if ([long]$After.Www[$k] -gt [long]$Before.Www[$k]) { $wwwAdvanced += $k }
    }
    $restarted = @()
    foreach ($k in @($After.Pid.Keys)) {
        if (-not $Before.Pid.ContainsKey($k)) { $restarted += $k; continue }
        if ("$($Before.Pid[$k])" -ne "$($After.Pid[$k])") { $restarted += $k }
    }
    $stopMarkers = @('已停止运行', '运行失败', '安装失败', '同步失败')
    $hitStop = @($stopMarkers | Where-Object { $Stdout -match [regex]::Escape($_) })
    $deployed = ($wwwAdvanced.Count -gt 0)
    $reason = if ($deployed) { "资源已落到设备：$($wwwAdvanced -join '、') 的 mtime 相对基线前进" }
              elseif ($restarted.Count -gt 0) { "只见进程重启（$($restarted -join '、')）、未见资源 mtime 前进 ⇒ 不判部署（旁证不判真）" }
              elseif ($hitStop.Count -gt 0) { "会话打出停止标记（$($hitStop -join '、')），且设备侧事实与基线完全一致（前台=$($After.Foreground)）" }
              else { "设备侧事实与基线完全一致（资源 mtime 未前进、进程 pid 未变；前台=$($After.Foreground)）" }
    return [pscustomobject]@{
        Deployed = $deployed
        Reason = $reason
        WwwAdvanced = $wwwAdvanced
        Restarted = $restarted
        StopMarkers = $hitStop
    }
}

function Format-WwwPair {
    <# 把基线/现值摘要成一行（机检行用）：`<路径>=<epoch>`，缺失写 none。#>
    param($Before, $After)
    $paths = @(@($Before.Www.Keys) + @($After.Www.Keys)) | Select-Object -Unique
    $parts = @()
    foreach ($p in @($paths)) {
        $b = if ($Before.Www.ContainsKey($p)) { $Before.Www[$p] } else { 'none' }
        $a = if ($After.Www.ContainsKey($p)) { $After.Www[$p] } else { 'none' }
        $parts += ("{0}={1}->{2}" -f $p, $b, $a)
    }
    if ($parts.Count -eq 0) { return 'none->none' }
    return ($parts -join ',')
}


function Write-SegmentTable {
    param($Steps, $Segment, [int]$Total)
    Write-Host ''
    Write-Host "分段计时（segment_source=$($Segment.Source)）：" -ForegroundColor Cyan
    foreach ($s in $Steps) { Write-Host ("  {0,-14} = {1}s" -f $s.Name, $s.Seconds) }
    Write-Host ("  {0,-14} = {1}s" -f 'compile', $Segment.Compile)
    Write-Host ("  {0,-14} = {1}s" -f 'deploy', $Segment.Deploy)
    Write-Host ("  {0,-14} = {1}s" -f 'total', $Total)
    if ($Segment.Source -eq 'wallclock') {
        Write-Host '  [note] 未从 stdout 命中带时间戳的编译段边界 ⇒ 编译段退化为 launch 步墙钟、部署段记 0。' -ForegroundColor Yellow
        Write-Host '         真机首跑后按真实输出校准 $script:HxCompileEndMarkers（见脚本头「诚实声明」）。' -ForegroundColor Yellow
    }
}

# ---------- 定位项目与参数 ----------

if (-not $Project) {
    $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
}
if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}
if (-not $LogPath) {
    $LogPath = Join-Path (Join-Path $Project '.ci-verify') 'hx-run.log'
}

$mode = 'incremental'

$explicitCli = $Cli
if (-not $explicitCli) { $explicitCli = $env:HBuilderX_CLI }
$cliPath = Resolve-CliPath -Explicit $explicitCli -HxDir $HBuilderX -SkipRunningProbe:$DryRun

if ($CompileOnly -and $Full) {
    Write-Host '[error] -CompileOnly 与 -Full 互斥：「只编译拿诊断」与「全量重编」不是一回事（全量门是 compile-check.ps1）。' -ForegroundColor Red
    exit 2
}

# launch 参数：**真运行**（默认一律不传 `--compile` —— 官方语义是「仅编译代码」，传了就只编译不运行）；
# `--deviceId` 在解析出目标设备之后追加（见「目标设备前置判定」之后那一行）。
# `--compile` **只允许出现在 `-CompileOnly` 分支里**（契约测试 C13 守护）；
# 干净缓存重建开关只允许出现在本文件唯一那个 `if ($Full)` 块里（契约测试 C1 守护 —— 所以块外连注释都别写那个字面量）。
$launchArgs = @('launch', 'app-android', '--project', $Project)
if ($CompileOnly) {
    # -CompileOnly（#949）：官方语义的「仅编译代码」——**不推送、不启动、不需要设备**（下面会跳过设备解析），
    # 且该调用**会自己收口**（脚本头「假绿教训 v2」①实测）⇒ 用带超时的同步步跑它即可。
    $mode = 'compile-only'
    $launchArgs += @('--compile', 'true')
}
if ($Full) {
    # 只有显式 -Full 才切全量并加「干净缓存重建」开关；日常迭代不加（全量门的活归 compile-check.ps1）
    $mode = 'full'
    $launchArgs += @('--cleanCache', 'true')
}

# ---------- -DryRun：只打印计划，零副作用 ----------
# 本分支在任何 HBuilderX 交互（Wait-HxFree）与任何 cli 派发（Invoke-CliStep）之前就 exit 0；
# 也不探测进程、不调 adb、不建目录、不写日志。契约测试 hxRunContract C7 守护这个顺序。
if ($DryRun) {
    $planCli = '<未探测到：正式运行需 -Cli <路径> 或设 $env:HBuilderX_CLI>'
    if ($cliPath) { $planCli = $cliPath }
    Write-Host '=== hx-run 计划（-DryRun：不调用 HBuilderX、不调 adb、不写日志）===' -ForegroundColor Cyan
    Write-Host ("mode     = {0}   # incremental = 默认增量；full 才加干净缓存重建开关" -f $mode)
    Write-Host ("project  = {0}" -f $Project)
    Write-Host ("cli      = {0}" -f $planCli)
    Write-Host ("log      = {0}   # 正式运行才写" -f $LogPath)
    if ($Device) {
        Write-Host ("device   = {0}   # 显式指定" -f $Device)
    } else {
        Write-Host 'device   = <自动：唯一在线设备；有多个在线设备时必须显式 -Device，否则 exit 2>'
    }
    Write-Host ("wait     = Wait-HxFree -TimeoutSeconds {0}   # 忙就等，超时 exit 2，绝不 kill" -f $WaitSeconds)
    Write-Host ("timeout  = 部署轮询上限 {0}s，每 {1}s 看一次设备侧事实；不等待也不强杀那个会话" -f $TimeoutSeconds, $PollSeconds)
    Write-Host '将执行的分步命令行：'
    Write-Host ("  step 1  : {0}" -f (Format-ArgvLine -Exe $planCli -CliArgs @('open')))
    Write-Host ("  step 2  : {0}" -f (Format-ArgvLine -Exe $planCli -CliArgs @('project', 'open', '--path', $Project)))
    Write-Host ("  step 3  : {0}" -f (Format-ArgvLine -Exe $planCli -CliArgs $launchArgs))
    if ($CompileOnly) {
        Write-Host '  （-CompileOnly：**只编译**，不追加 --deviceId、不推送、不启动；该调用会自己收口）'
    } else {
        Write-Host '  （step 3 另追加 --deviceId <解析出的设备序列号>；真运行会话常驻，本脚本不等待其收口，改判设备侧事实）'
    }
    Write-Host '  （脚本不自装基座：基座只装一次，装机由 HBuilderX 自己处理）'
    Write-HxRunResult -Mode $mode -Compile 0 -Deploy 0 -Total 0 -Exit 'dryrun' -LogFile $null
    exit 0
}

if (-not $cliPath) {
    Write-Host '[error] 找不到可用的 HBuilderX cli.exe。用 -Cli <路径> 或 -HBuilderX <安装目录> 或设 $env:HBuilderX_CLI 指定。' -ForegroundColor Red
    exit 2
}

if ($CompileOnly) {
    # 仅编译**不需要设备**：不碰 adb、不做设备前置判定（也让本模式能在不接设备的机器上跑，#949）
    $target = @{ Serial = ''; Mode = 'compile-only' }
} else {
    # ---------- 目标设备前置判定（缺/多 ⇒ exit 2，避免装到错的设备）----------
    $adbExe = Resolve-AdbExe
    if (-not $adbExe) {
        Write-Host '[error] 找不到 adb.exe。设 $env:ANDROID_SDK_ROOT / $env:ANDROID_HOME，或跑 scripts/android-sdk-setup.ps1 装到 D:\android-sdk。' -ForegroundColor Red
        exit 2
    }
    $target = Resolve-TargetDevice -Requested $Device -AdbExe $adbExe

    # 把解析出的设备显式传给 launch（官方文档：不指定时默认使用第一个设备）。
    # 本脚本已断言「多设备必须显式 -Device」，所以这里传的就是那个唯一/显式的目标，语义无歧义。
    $launchArgs += @('--deviceId', $target.Serial)
}

# ---------- 日志 ----------
$logDir = Split-Path -Parent $LogPath
if ($logDir) { New-Item -ItemType Directory -Force -Path $logDir | Out-Null }
Set-Content -LiteralPath $LogPath -Encoding utf8 -Value @(
    "# hx-run 开始 $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
    "# mode = $mode（本脚本不是门：门 = ④a compile-check.ps1 全量 / ④c kotlin-all-check.ps1）",
    "# cli = $cliPath",
    "# project = $Project",
    "# device = $($target.Serial)（$($target.Mode)）",
    "# wait = $WaitSeconds 秒上限；本脚本绝不 kill HBuilderX / cli / 主程序"
)

# ---------- HBuilderX 忙检测（单实例串行资源，ADR-0008 坑位段）----------
. (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
$hx = Wait-HxFree -CliExe $cliPath -TimeoutSeconds $WaitSeconds -LogPath $LogPath
try {

$runClock = [System.Diagnostics.Stopwatch]::StartNew()

# ---------- -CompileOnly：只编译、拿编译期诊断（不推送 / 不启动 / 不轮询 / 不需要设备；#949）----------
# 为什么要有它：2026-09-13 实测那次会话，慢的不是编译，而是两次「编译期诊断拖到真机才发现」的返工
# （`MenuItem` 类型名义重复 → ClassCastException；`<view>` 上用了 text-align|font-size|color）。
# 这两条都是**编译期诊断** ⇒ 用「仅编译」就能在本地、在不接设备的机器上拦住。
if ($CompileOnly) {
    Write-Host '=== -CompileOnly：只编译（不推送 / 不启动 / 不轮询 / 不需要设备）===' -ForegroundColor Cyan
    $steps = New-Object System.Collections.ArrayList
    # [void] 是必须的：ArrayList.Add() 会把索引打到输出流，污染后面的文本解析
    [void]$steps.Add((Invoke-CliStep -Name 'open' -CliArgs @('open') -StepTimeout $StepTimeoutSeconds -CliExe $cliPath -LogDir $logDir -LogFile $LogPath))
    [void]$steps.Add((Invoke-CliStep -Name 'project-open' -CliArgs @('project', 'open', '--path', $Project) -StepTimeout $StepTimeoutSeconds -CliExe $cliPath -LogDir $logDir -LogFile $LogPath))
    # 「仅编译」调用**会自己收口**（脚本头「假绿教训 v2」①实测）⇒ 用带超时的同步步，不像真运行那样常驻
    [void]$steps.Add((Invoke-CliStep -Name 'compile-only' -CliArgs $launchArgs -StepTimeout $TimeoutSeconds -CliExe $cliPath -LogDir $logDir -LogFile $LogPath))

    $compileStep = @($steps | Where-Object { $_.Name -eq 'compile-only' })[0]
    $allOutput = (@($steps | ForEach-Object { $_.Output }) -join "`n")
    $totalSeconds = [int]$runClock.Elapsed.TotalSeconds
    $segment = Get-HxSegment -Output $compileStep.Output -LaunchSeconds $compileStep.Seconds
    Write-SegmentTable -Steps $steps -Segment $segment -Total $totalSeconds

    $notExited = @($steps | Where-Object { -not $_.Exited })
    if ($notExited.Count -gt 0 -or $allOutput -match '与主程序的连接已中断|启动超时') {
        Write-Host ''
        Write-Host '[error] HBuilderX CLI 连不上主程序 / 某步未在超时内返回 ⇒ 环境不可用（exit 2）。' -ForegroundColor Red
        Write-Host '        受限/沙箱会话会阻断 CLI 与主程序的本地 IPC；请在**全访问权限**的普通终端里跑。' -ForegroundColor Red
        Write-Host '        本脚本**绝不 kill** HBuilderX / cli / 主程序（忙就等，超时就退出）。' -ForegroundColor Red
        Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy 0 -Total $totalSeconds -Exit 'env' -LogFile $LogPath
        exit 2
    }

    # ⚠️ 调用点必须包 `@(...)`：函数 `return @(...)` 在空数组时会退化成 $null，
    #    而 `Set-StrictMode -Latest` 下 `$null.Count` 直接抛错（2026-09-13 首次真跑踩到）
    $errorLines = @(Get-HxErrorLines -Output $allOutput)
    if ($errorLines.Count -gt 0) {
        Write-Host ''
        Write-Host "❌ 仅编译发现 $($errorLines.Count) 行编译期诊断 ——" -ForegroundColor Red
        $errorLines | Select-Object -First 20 | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
        Write-Host "完整日志：$LogPath" -ForegroundColor Yellow
        Write-Host '   这类诊断**不该拖到真机**（#949）：先在本地清干净，再走真运行 / ① 取证。' -ForegroundColor Yellow
        Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy 0 -Total $totalSeconds -Exit 'fail' -LogFile $LogPath
        exit 1
    }

    Write-Host ''
    Write-Host '✅ 仅编译干净：没有编译期诊断行（本次未推送、未启动、未占用设备）。' -ForegroundColor Green
    Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy 0 -Total $totalSeconds -Exit 'ok' -LogFile $LogPath
    exit 0
}

# ---------- 部署基线（判据是「相对基线是否前进」，见脚本头「假绿教训 v2」）----------
# 候选包名：manifest 的 appid（__UNI__XXXX → uni.app.XXXX）+ HBuilderX 标准基座（dev 运行的常见承载）；
# 资源目录候选由包名 + appid 派生。**取不到就是空** —— 空表示「无从判断」，绝不表示「已部署」。
$appidMatch = [regex]::Match((Get-Content -LiteralPath (Join-Path $Project 'manifest.json') -Raw), '"appid"\s*:\s*"(__UNI__[0-9A-Za-z]+)"')
$appId = ''
if ($appidMatch.Success) { $appId = $appidMatch.Groups[1].Value }
$candidatePkgs = @()
if ($appId) { $candidatePkgs += ('uni.app.' + ($appId -replace '^__UNI__', '')) }
$candidatePkgs += 'io.dcloud.uniappx'
$wwwPaths = Get-WwwCandidatePaths -Packages $candidatePkgs -AppId $appId
$factsBefore = Get-DeviceDeployFacts -AdbExe $adbExe -Serial $target.Serial -WwwPaths $wwwPaths -Packages $candidatePkgs
Write-Host ">>> [deploy] 基线：$(Format-WwwPair -Before $factsBefore -After $factsBefore)"

$steps = New-Object System.Collections.ArrayList
# [void] 是必须的：ArrayList.Add() 会把索引打到输出流，污染后面的文本解析
[void]$steps.Add((Invoke-CliStep -Name 'open' -CliArgs @('open') -StepTimeout $StepTimeoutSeconds -CliExe $cliPath -LogDir $logDir -LogFile $LogPath))
[void]$steps.Add((Invoke-CliStep -Name 'project-open' -CliArgs @('project', 'open', '--path', $Project) -StepTimeout $StepTimeoutSeconds -CliExe $cliPath -LogDir $logDir -LogFile $LogPath))

$preOutput = (@($steps | ForEach-Object { $_.Output }) -join "`n")
$notExited = @($steps | Where-Object { -not $_.Exited })
if ($notExited.Count -gt 0 -or $preOutput -match '与主程序的连接已中断|启动超时') {
    Write-Host ''
    Write-Host '[error] HBuilderX CLI 连不上主程序 / 某步未在超时内返回 ⇒ 环境不可用（exit 2）。' -ForegroundColor Red
    Write-Host '        受限/沙箱会话会阻断 CLI 与主程序的本地 IPC；请在**全访问权限**的普通终端里跑，' -ForegroundColor Red
    Write-Host '        或改跑不需要 HBuilderX 的检查：npm run test:unit / npm run build:kotlin-all -SkipPublish / npm run smoke:emulator。' -ForegroundColor Red
    Write-Host '        本脚本**绝不 kill** HBuilderX / cli / 主程序（忙就等，超时就退出）。' -ForegroundColor Red
    Write-HxRunResult -Mode $mode -Compile 0 -Deploy 0 -Total ([int]$runClock.Elapsed.TotalSeconds) -Exit 'env' -LogFile $LogPath
    exit 2
}

# ---------- launch：**派发后不等待**（真运行会话常驻），改观察设备侧事实是否前进 ----------
$launch = Start-CliLaunchDetached -CliArgs $launchArgs -CliExe $cliPath -LogDir $logDir -LogFile $LogPath
$factsAfter = $factsBefore
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$polledSeconds = 0
$launchExitedEarly = $false
$stalledEarly = $false
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds $PollSeconds
    $polledSeconds += $PollSeconds
    $factsAfter = Get-DeviceDeployFacts -AdbExe $adbExe -Serial $target.Serial -WwwPaths $wwwPaths -Packages $candidatePkgs
    $probe = Get-DeployVerdict -Before $factsBefore -After $factsAfter -Stdout (Get-CapturedOutput -OutFile $launch.OutFile -ErrFile $launch.ErrFile)
    if ($probe.Deployed) { break }
    if ($launch.Proc.HasExited) { $launchExitedEarly = $true; break }
    # 快速失败（#949）：**编译段已结束** + 该秒数内设备侧无前进 ⇒ 不必等满 -TimeoutSeconds
    # （2026-09-13 实测某次 launch 卡死，白等了 ~20 分钟才到上限）
    if ($polledSeconds -ge $DeployStallSeconds -and (Test-HxCompileFinished -Output (Get-CapturedOutput -OutFile $launch.OutFile -ErrFile $launch.ErrFile))) {
        $stalledEarly = $true
        break
    }
    if (($polledSeconds % 60) -eq 0) {
        Write-Host ">>> [deploy] 已等 $polledSeconds 秒（上限 $TimeoutSeconds 秒），设备侧事实仍未前进…" -ForegroundColor Yellow
    }
}

$launchOutput = Get-CapturedOutput -OutFile $launch.OutFile -ErrFile $launch.ErrFile
$allOutput = $preOutput + "`n" + $launchOutput
$totalSeconds = [int]$runClock.Elapsed.TotalSeconds
$segment = Get-HxSegment -Output $launchOutput -LaunchSeconds $polledSeconds
Write-SegmentTable -Steps $steps -Segment $segment -Total $totalSeconds
Add-Content -LiteralPath $LogPath -Value "`n$launchOutput" -Encoding utf8

# 判成败只解析 stdout（CLI 退出码恒 0）；判据与 -CompileOnly 共用同一个函数（#949，避免两处漂移）
# ⚠️ 必须包 `@(...)`：函数 `return @(...)` 空数组会退化成 $null，`$null.Count` 在 StrictMode 下抛错
$errorLines = @(Get-HxErrorLines -Output $allOutput)

if ($errorLines.Count -gt 0) {
    Write-Host ''
    Write-Host "❌ 运行没干净收口：$($errorLines.Count) 行含 error/失败字样 ——" -ForegroundColor Red
    $errorLines | Select-Object -First 20 | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
    Write-Host "完整日志：$LogPath" -ForegroundColor Yellow
    Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy $segment.Deploy -Total $totalSeconds -Exit 'fail' -LogFile $LogPath
    exit 1
}

# ---------- 部署判定（**基线相对**：只有设备侧事实前进才算部署；旁证不判真）----------
$deployed = Get-DeployVerdict -Before $factsBefore -After $factsAfter -Stdout $launchOutput
$deployedText = if ($deployed.Deployed) { 'true' } else { 'false' }
$pidBeforeText = (@($factsBefore.Pid.Keys) | ForEach-Object { "$_=$($factsBefore.Pid[$_])" }) -join ','
$pidAfterText = (@($factsAfter.Pid.Keys) | ForEach-Object { "$_=$($factsAfter.Pid[$_])" }) -join ','
if (-not $pidBeforeText) { $pidBeforeText = 'none' }
if (-not $pidAfterText) { $pidAfterText = 'none' }
$deployLine = "HX_RUN_DEPLOY deployed=$deployedText www=$(Format-WwwPair -Before $factsBefore -After $factsAfter) pid_before=$pidBeforeText pid_after=$pidAfterText foreground=$($factsAfter.Foreground) reason=$($deployed.Reason)"
Write-Host $deployLine
Add-Content -LiteralPath $LogPath -Value $deployLine -Encoding utf8

if (-not $deployed.Deployed) {
    Write-Host ''
    Write-Host '❌ 判定为**未部署**：设备侧事实与基线完全一致 —— 这次运行不算成功。' -ForegroundColor Red
    Write-Host "   依据：$($deployed.Reason)" -ForegroundColor Red
    if ($launchExitedEarly) {
        Write-Host '   注意：那个会话**自己提前退出了**（真运行会话不会自己收口）—— 与「仅编译」调用同形，' -ForegroundColor Red
        Write-Host '         先确认本脚本的 launch 参数里**没有** `--compile`（官方语义是「仅编译代码」）。' -ForegroundColor Red
    }
    if ($stalledEarly) {
        Write-Host "   注意：**编译段已结束、部署阶段 $polledSeconds 秒无前进** ⇒ 提前判环境不可用（不等满 $TimeoutSeconds 秒）。" -ForegroundColor Yellow
        Write-Host '         实测正常推送只需 15–21 秒，所以这种"停滞"多半是会话卡住 / 设备被别的 App 占着 / 装机弹窗没人点。' -ForegroundColor Yellow
    }
    if ($deployed.StopMarkers.Count -gt 0) {
        Write-Host "   会话的停止/失败标记：$($deployed.StopMarkers -join '、')" -ForegroundColor Red
    }
    Write-Host '   常见原因：设备屏幕锁着或被别的 App 占着、装机确认弹窗没人点、基座未就绪、' -ForegroundColor Yellow
    Write-Host '             或**距上次部署无代码变更**（保守起见一律判未部署，宁可多看一眼也不报假绿）。' -ForegroundColor Yellow
    Write-Host '   注意：这里判的是「有没有到设备」，不是「代码对不对」—— 编译错误另有上面的 error 行分支。' -ForegroundColor Yellow
    Write-Host "   完整日志：$LogPath" -ForegroundColor Yellow
    Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy $segment.Deploy -Total $totalSeconds -Exit 'env' -LogFile $LogPath
    exit 2
}

Write-Host ''
Write-Host "✅ 已 $mode 运行到 $($target.Serial)（device_mode=$($target.Mode)）：设备侧事实相对基线前进。" -ForegroundColor Green
Write-Host "   那个运行会话**仍然活着**（真运行不自己收口）；要停就在 HBuilderX 里点「停止」—— 本脚本绝不 kill。" -ForegroundColor Green
Write-Host "   本脚本不是门：收口前记得跑中循环（npm run test:unit + npm run build:kotlin-all）。" -ForegroundColor Green
Write-HxRunResult -Mode $mode -Compile $segment.Compile -Deploy $segment.Deploy -Total $totalSeconds -Exit 'ok' -LogFile $LogPath
exit 0

} finally {
    # 释放 agent 互斥锁（必须，否则下一个会话会一直等）
    Release-HxLock
}