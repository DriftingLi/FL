<#
.SYNOPSIS
    【非门（不替代 ① 验收门，也不写入 ## 验收证据）】真机无线调试统一入口：
    自动发现连接端口 + 心跳保活 + 掉线自愈，跑完直接给出门要用的 -Device 值。

    ⚠️ **它是内循环的载体，不是门**：本脚本的结论行叫 WIRELESS_DEBUG，**不得**被当成任何验收门的证据，
    也**不得**出现 gate-evidence: 标记（判据同 utils/emulatorSmokeContract.test.js 的 C4）。
    它只替人做两件机械事：从 mDNS 候选里试出能用的端口、以及把掉线后的重连接回来。

.DESCRIPTION
    解决两件重复发生的事：

    1. **端口轮换后要人读屏**。这台 PC 上 `adb mdns services` 是能出记录的（2026-10-07 现测：
       `adb-b32d8398-ul54S3  _adb-tls-connect._tcp  192.168.10.51:39527`，另有一条 37611 的**陈旧记录**），
       所以「IP 地址和端口」那一行不必再从手机屏幕上抄——本脚本把 mDNS 记录当候选表，逐个 `connect` 试探，
       再用 `shell echo ok` 验真。**陈旧记录恒回 10061**（现测），所以「记录里有」不等于「能用」，只能现测。
    2. **连着连着掉了 / 开关被关**。心跳只发无副作用命令保活 TCP；慢拍读 `global adb_wifi_enabled`，
       读到非 1 才写回 1（现测：值已是 1 时同值写回**不换端口、会话不断**，故可安全重复声明）。

    ### 防不住的部分（照实说，别当万能药）
    手机侧重置（重启、撤销授权、手动关掉开关）之后**一个 adb 通道都没有**，任何脚本都无法从零把开关打开。
    那一刻仍要人在手机上点一次；脚本保证的是「点完这一次之后不用读屏、不用再手抄端口」。

    ### 实测读数（2026-10-07，设备 192.168.10.51，Redmi marble / Android 15 / HyperOS V816）
    - 注入故障（把这台设备的全部 transport `adb disconnect`）后，心跳 **2 秒**内经 mDNS 自己接回同一端口，
      全程没读屏、没重启 adb server（`keep.log` 时间线：LOST 14:28:46 → SAMEPORT 14:28:47 → RECOVERED 14:28:48）。
    - 慢拍间隔实测 16–17 秒（hb=5s × 3 拍 + adb 调用开销）——判「间隔有没有真生效」只看相邻日志时间差。
    - 旧端口（37611）实测恒回 `10061 由于目标计算机积极拒绝` ⇒ **端口已经换了就不是重试问题**，
      只能人看一次手机；此时 mDNS 若已出新鲜记录，`ensure` 会自己挑到，仍不用抄端口。
    - `adb pair` 打到【连接端口】实测回 `protocol fault` —— pair 分支据此把失败分成三类回话，
      且失败的配对尝试实测**不影响已存在的会话**。

    ### 红线（与 scripts/device-capture.ps1 同源）
    - **绝不 `adb kill-server`**：adb server 与 HBuilderX 共享，杀掉会连带打断门。
    - 心跳只发 `shell echo` / `shell settings get|put`；**不发 `input`**（抢取证脚本的焦点判据），
      不截图、不 install / force-stop / logcat -c / push / reboot。
    - `status` 比心跳多读几样诊断（`dumpsys battery` / `dumpsys power` 全量取回后本地过滤、
      `ip -4 addr show`、`settings get`）——都只读，但**别在取证跑到一半**用它抢设备回话。
    - `stop` 只杀本脚本自己起的 keep 进程（校验命令行含本文件名），不碰 adb / HBuilderX / cli。
    - 同一时刻只允许一个 keep：已在跑就拒绝叠加（两条心跳对同一设备只会互抢）。

    ### 副作用边界
    唯一会写设备状态的路径是 `-Action on` 与慢拍的 `settings put global adb_wifi_enabled 1`，
    且**只写这一个 key 的“开”方向**——永不写 0，不改别的 setting。要纯只读就带 `-ReassertToggle:$false`。

.PARAMETER Action
    status 只读诊断 | ensure 连上就退出（默认，给门用）| probe 只报发现结果不改动 | keep 前台心跳循环
    | watch 后台心跳（默认存活 8 小时后自收口）| stop 收掉后台 keep
    | on 用现有通道把无线调试开关写回 1 | pair 配对（6 位码只能人从手机弹窗读）

.PARAMETER ReassertToggle
    慢拍是否执行「读到非 1 就写回 1」。默认开。关掉后全程只读，设备一个字节都不改。

.EXAMPLE
    $T = 'scripts/wireless-debug.ps1'
    pwsh -NoProfile -File $T                                   # 连上并给出 -Device 该填什么
    pwsh -NoProfile -File $T -Action watch                     # 开跑一整天的后台保活（8 小时后自己退出）
    pwsh -NoProfile -File $T -Action status                    # 出问题了先看这份诊断
    pwsh -NoProfile -File $T -Action stop                      # 收掉后台保活
    pwsh -NoProfile -File $T -Action pair -PairEndpoint 192.168.10.51:41203 -PairCode 123456
    # 喂门（只要 serial）：$dev = pwsh -NoProfile -File $T -Action ensure -Quiet
    # 然后：npm run hx:run -- -Device $dev

.OUTPUTS
    机读只认一行：`WIRELESS_DEBUG action=... result=... serial=... toggle=... extra=...`（纯 ASCII，可 grep）。
    中文全文在 UTF-8 三份文件里（上层按 GBK 解码 stdout 时中文必乱码，别拿控制台输出当文本源）：
      · last-run.txt —— 本次人类输出的副本
      · actions.log  —— 每次动作的结论 + Detail（追加）
      · keep.log     —— 心跳/掉线/自愈时间线（追加）
    三份都在状态目录里（默认 $env:LOCALAPPDATA\adb-wireless，可用 -StateDir 或 $env:ADB_WIRELESS_STATE_DIR 覆盖）。
#>
[CmdletBinding()]
param(
    [ValidateSet('status', 'ensure', 'keep', 'watch', 'stop', 'pair', 'on', 'probe')]
    [string]$Action = 'ensure',

    [string]$AdbExe = 'D:\android-sdk\platform-tools\adb.exe',

    # 已知的 ip:port；给了就先试它，失败仍回落到 mDNS 发现
    [string]$Serial,

    # pair 分支专用：弹窗里那一行「IP:端口」+ 6 位配对码
    [string]$PairEndpoint,
    [string]$PairCode,

    # 连接态心跳间隔
    [int]$IntervalSeconds = 20,
    # 每 N 拍做一次「读开关 + 必要时写回 1」的慢拍
    [int]$SlowBeatEvery = 12,
    # 掉线态重连/重发现的尝试间隔（比心跳密）
    [int]$ProbeSeconds = 8,
    # 掉线后最多等多久仍无可用端口才放弃（旧端口 10061 恒永久失效，此时只能人点手机）
    [int]$WaitMinutes = 25,
    # keep/watch 的总存活时长（小时）。默认 8 —— 不留无限存活的后台心跳；跑一天自己收口
    [int]$MaxHours = 8,

    # 机器级落点：这工具要在任意 worktree / 任意检出里用，**不得**写死宿主树路径（契约 W6 锁这条）。
    # 优先级：-StateDir > $env:ADB_WIRELESS_STATE_DIR > $env:LOCALAPPDATA\adb-wireless > $env:TEMP\adb-wireless
    [string]$StateDir = $(if ($env:ADB_WIRELESS_STATE_DIR) { $env:ADB_WIRELESS_STATE_DIR }
        elseif ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA 'adb-wireless' }
        else { Join-Path $env:TEMP 'adb-wireless' }) <# 入仓判据：这里不得出现宿主树绝对路径 #>,
    [switch]$ReassertToggle = $true,
    # 只打 serial，便于命令替换
    [switch]$Quiet,
    # 被别的脚本/agent 捕获输出时用：stdout 只出现 ASCII 字节（非 ASCII 换成 ?），
    # 中文全文留在 UTF-8 的 last-run.txt / actions.log，免得对面拿到一屏乱码还当读数。
    [switch]$Ascii
)

$ErrorActionPreference = 'Stop'
$StatePath = Join-Path $StateDir 'state.json'
$LogPath = Join-Path $StateDir 'keep.log'
$PidPath = Join-Path $StateDir 'watch.pid'
$ActLogPath = Join-Path $StateDir 'actions.log'
$TranscriptPath = Join-Path $StateDir 'last-run.txt'
$script:TranscriptStarted = $false
$script:ScriptFile = $PSCommandPath

# 现测坑：pwsh 的控制台输出按这台机器的码页走，被上层（Bash / 别的 pwsh 驱动）按 GBK 解码时中文成乱码。
# 对策三条：① 机读结论行只用 ASCII；② -Ascii 时把 stdout 里的非 ASCII 全部换成 ?；
# ③ 中文全文一律落 UTF-8 文件（last-run.txt 是人类输出副本，actions.log 是结论+Detail，keep.log 是时间线）。
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

if (-not (Test-Path -LiteralPath $AdbExe -PathType Leaf)) {
    Write-Host "WIRELESS_DEBUG abort=adb_missing exe=$AdbExe"
    exit 2
}
if (-not (Test-Path -LiteralPath $StateDir)) {
    New-Item -ItemType Directory -Path $StateDir -Force | Out-Null
}

# ---------- 基础层 ----------

function Invoke-Adb {
    param([string[]]$AdbArgs)
    $out = & $AdbExe @AdbArgs 2>&1 | Out-String
    return $out.Trim()
}

function Get-DeviceList {
    $list = @()
    foreach ($l in ((Invoke-Adb @('devices')) -split "`r?`n")) {
        if ($l -match '^\s*(\S+)\s+(device|offline|unauthorized|no permissions)\s*$') {
            $list += [pscustomobject]@{ Serial = $Matches[1]; State = $Matches[2] }
        }
    }
    return @($list)
}

function Get-MdnsCandidate {
    # `adb mdns services` 行形如：name<TAB>_adb-tls-connect._tcp<TAB>192.168.10.51:39527
    $found = @()
    foreach ($l in ((Invoke-Adb @('mdns', 'services')) -split "`r?`n")) {
        if ($l -match '_adb-tls-connect\._tcp\s+(\d{1,3}(?:\.\d{1,3}){3}:\d{2,5})\s*$') {
            $ep = $Matches[1]
            if (-not ($found.Serial -contains $ep)) {
                $found += [pscustomobject]@{ Serial = $ep; Source = 'mdns' }
            }
        }
    }
    return $found
}

function Test-LiveSerial {
    # connect 回 "already connected" 或 "connected to" 都不代表能用；
    # 唯一的真判据是这条 transport 在 adb devices 里 state=device 且 shell 有回话。
    param([string]$Ep)
    $out = Invoke-Adb @('connect', $Ep)
    $state = $null
    foreach ($d in (Get-DeviceList)) { if ($d.Serial -eq $Ep) { $state = $d.State } }
    if ($state -ne 'device') {
        return [pscustomobject]@{ Ok = $false; Detail = $out }
    }
    $echo = Invoke-Adb @('-s', $Ep, 'shell', 'echo', 'ok')
    if ($echo -notmatch '(?m)^ok$') {
        return [pscustomobject]@{ Ok = $false; Detail = "state=device 但 shell 无回话：$echo" }
    }
    return [pscustomobject]@{ Ok = $true; Detail = $out }
}

function Read-ToggleState {
    param([string]$Ep)
    $v = Invoke-Adb @('-s', $Ep, 'shell', 'settings', 'get', 'global', 'adb_wifi_enabled')
    if ($v -match '^\s*(\d+|null)\s*$') { return $Matches[1] }
    return "?($v)"
}

function Write-ToggleOn {
    # 只写“开”方向，永不写 0。同值写回实测不换端口（见 .DESCRIPTION）。
    param([string]$Ep)
    Invoke-Adb @('-s', $Ep, 'shell', 'settings', 'put', 'global', 'adb_wifi_enabled', '1') | Out-Null
    return (Read-ToggleState -Ep $Ep)
}

function Find-WorkingEndpoint {
    # 候选顺序：显式 -Serial → 上次成功的 state.json → mDNS 全部记录。
    # 每条都现测（10061 的旧记录会留在 mDNS 表里，不能信列表）。
    param([string]$Preferred)
    $ordered = @()
    $seen = @{}
    if ($Preferred) { $ordered += $Preferred; $seen[$Preferred] = $true }
    $st = Read-State
    if ($st -and $st.serial -and -not $seen.ContainsKey($st.serial)) {
        $ordered += $st.serial; $seen[$st.serial] = $true
    }
    foreach ($c in (Get-MdnsCandidate)) {
        if (-not $seen.ContainsKey($c.Serial)) { $ordered += $c.Serial; $seen[$c.Serial] = $true }
    }
    foreach ($ep in $ordered) {
        $r = Test-LiveSerial -Ep $ep
        if ($r.Ok) {
            return [pscustomobject]@{ Ok = $true; Serial = $ep; Tried = $ordered.Count }
        }
        Say ("  候选 {0} 不可用：{1}" -f $ep, $r.Detail)
        $t = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
        Add-Content -LiteralPath $ActLogPath -Value ('{0} candidate_reject endpoint={1} reason={2}' -f `
                $t, $ep, ($r.Detail -replace "`r?`n", ' ')) -Encoding utf8
        # 连上但 offline 的残条目会干扰后续判定，顺手摘掉（只动这一条，不 kill-server）
        if ($r.Detail -notmatch '10061') { Invoke-Adb @('disconnect', $ep) | Out-Null }
    }
    return [pscustomobject]@{ Ok = $false; Serial = $null; Tried = $ordered.Count }
}

function Read-State {
    if (-not (Test-Path -LiteralPath $StatePath)) { return $null }
    try { return (Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json) } catch { return $null }
}

function Write-State {
    param([string]$Ep, [string]$Note)
    $prev = Read-State
    $hist = @()
    if ($prev -and $prev.portHistory) { $hist = @($prev.portHistory) }
    if ($Ep -and (($hist | Select-Object -First 1) -ne $Ep)) {
        $hist = @($Ep) + ($hist | Select-Object -First 9)
    }
    $obj = [ordered]@{
        serial      = $Ep
        updatedAt   = (Get-Date).ToString('s')
        portHistory = $hist
        toggleState = $(if ($Ep) { Read-ToggleState -Ep $Ep } else { 'no-channel' })
        note        = $Note
    }
    ($obj | ConvertTo-Json -Depth 4) | Set-Content -LiteralPath $StatePath -Encoding utf8
}

function Get-ProcessCommandLine { param([int]$Id)
    # 归属判据要读「这个 pid 的命令行里有没有本脚本」，而读法两边平台不一样：
    # Windows 走 CIM；Linux **根本没有 CIM cmdlet**——现测（2026-10-07 CI run 37597381121，ubuntu-24.04）
    # 在 Linux 上调它会 CommandNotFound ⇒ 整条 watch 当场炸掉，连「读不到」都算不上。
    # 所以这里按平台分支：Linux 读 /proc/<pid>/cmdline（NUL 分隔），读不到就返回 $null，
    # 让上层走「不认这个 pid」那一支——宁可报 spawn_unconfirmed，也不猜某个进程是自家 keep。
    if ($IsWindows) {
        return (Get-CimInstance Win32_Process -Filter "ProcessId=$Id" -ErrorAction SilentlyContinue).CommandLine
    }
    $procFile = "/proc/$Id/cmdline"
    if (Test-Path -LiteralPath $procFile) {
        $bytes = [System.IO.File]::ReadAllBytes($procFile)
        return (([System.Text.Encoding]::UTF8.GetString($bytes)) -replace "`0", ' ')
    }
    return $null
}

function Get-WatcherPid {
    if (-not (Test-Path -LiteralPath $PidPath)) { return $null }
    # 自测踩到的坑：Start-Process -PassThru 在这台机器上回过空对象 ⇒ pid 文件写成空，
    # 于是 (Get-Content -Raw).Trim() 对 $null 调方法炸掉，后续 status/stop 全部 exit 1。
    # 现在 pid 由 keep 进程自己落盘（它拿得到 $PID），这里只做空值兜底。
    $raw = Get-Content -LiteralPath $PidPath -Raw -ErrorAction SilentlyContinue
    if (-not $raw) { return $null }
    $p = $raw.Trim()
    if ($p -match '^\d+$') {
        $proc = Get-Process -Id ([int]$p) -ErrorAction SilentlyContinue
        if ($proc) {
            $cl = Get-ProcessCommandLine -Id ([int]$p)
            # 归属判据取当前脚本自己的文件名（Get-WatcherPid 调用前已由 $script:ScriptFile 备好），不得写死字面量：
            # 入仓改名成 wireless-debug.ps1 后那个旧名再也不命中 —— 实测后果是 watch 永远报
            # spawn_unconfirmed、stop 永远报 not_running，而 keep 进程还在后台跑（留孤儿）。
            $mine = [System.IO.Path]::GetFileName($script:ScriptFile)
            if ($cl -and $mine -and $cl -match [regex]::Escape($mine)) { return [int]$p }
        }
    }
    return $null
}

function Set-WatcherPid { param([int]$Id) Set-Content -LiteralPath $PidPath -Value $Id -Encoding ascii }

function Clear-WatcherPid { Remove-Item -LiteralPath $PidPath -Force -ErrorAction SilentlyContinue }

function Say([object]$Msg) {
    if ($Quiet) { return }
    $s = [string]$Msg
    # -Ascii = 给管道/agent 用的：stdout 里不出现非 ASCII 字节（上层码页不是 UTF-8 时中文必乱码，
    # 与其给一屏乱码，不如把结构留下、中文只进 UTF-8 副本 last-run.txt）。
    $out = if ($Ascii) { $s -replace '[^\x20-\x7E]', '?' } else { $s }
    Write-Host $out
    if ($Action -eq 'keep') { return }
    try {
        if (-not $script:TranscriptStarted) {
            Set-Content -LiteralPath $TranscriptPath -Value '' -Encoding utf8
            $script:TranscriptStarted = $true
        }
        Add-Content -LiteralPath $TranscriptPath -Value $s -Encoding utf8
    } catch { }
}

function Write-Keep([string]$Msg) {
    $line = '{0} {1}' -f (Get-Date -Format 'HH:mm:ss'), $Msg
    Add-Content -LiteralPath $LogPath -Value $line -Encoding utf8
    Say $line
}

function Show-Verdict {
    param([string]$Result, [string]$Ep, [string]$Extra = '', [string]$Detail = '')
    # 机读行一律 ASCII —— 这台机器上 pwsh 的控制台输出经重定向会二次错码（现测：中文变乱码），
    # 拿它当 grep 目标会判错。中文说明走 Detail：打进 UTF-8 的 actions.log，并在终端说给人听。
    $t = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    $tg = $(if ($Ep) { Read-ToggleState -Ep $Ep } else { '-' })
    $ascii = ('{0} WIRELESS_DEBUG action={1} result={2} serial={3} toggle={4} extra={5}' -f `
            $t, $Action, $Result, $(if ($Ep) { $Ep } else { '-' }), $tg, $Extra) -replace '[^\x20-\x7E]', '?'
    Say $ascii
    if ($Detail) {
        # Detail 是给人看的；被管道捕获时按上层码页可能变乱码，-Ascii 就把它只写进日志。
        if (-not $Ascii) { Say "    → $Detail" }
    }
    $full = 'WIRELESS_DEBUG action={0} result={1} serial={2} toggle={3} extra={4} detail={5}' -f `
        $Action, $Result, $(if ($Ep) { $Ep } else { '-' }), $tg, $Extra, $Detail
    Add-Content -LiteralPath $ActLogPath -Value ('{0} {1}' -f $t, $full) -Encoding utf8
}

# ---------- 动作层 ----------

function Do-Status {
    Say '=== adb ==='
    Say ("exe={0}" -f $AdbExe)
    $ver = @((Invoke-Adb @('version')) -split "`r?`n" | Where-Object { $_ -match 'version' } | ForEach-Object { $_.Trim() }) -join ' / '
    Say $ver
    Say '=== adb devices ==='
    $devs = Get-DeviceList
    if ($devs.Count -eq 0) { Say '(空)' }
    foreach ($d in $devs) { Say ("  {0}  [{1}]" -f $d.Serial, $d.State) }
    Say '=== adb mdns services ==='
    Say ((Invoke-Adb @('mdns', 'services')) -replace "`r", '')
    Say '=== 设备侧真值（任取一条 device transport）==='
    $live = @($devs | Where-Object { $_.State -eq 'device' })
    if ($live.Count -ge 1) {
        $ep = $live[0].Serial
        Say ("  adb_wifi_enabled(global) = {0}" -f (Read-ToggleState -Ep $ep))
        # 多行输出必须先各自由变量收好再 -join：写在 "...{0}" -f (pipeline) -join ' ' 里
        # 会被运算符优先级吃掉，只打出第一条（2026-10-07 实测踩过）。
        $wlan = @((Invoke-Adb @('-s', $ep, 'shell', 'ip', '-4', 'addr', 'show', 'wlan0')) -split "`r?`n" |
            Where-Object { $_ -match 'inet ' } | ForEach-Object { $_.Trim() }) -join ' | '
        Say ("  wlan0 = {0}" -f $wlan)
        $batt = @((Invoke-Adb @('-s', $ep, 'shell', 'dumpsys', 'battery')) -split "`r?`n" |
            Where-Object { $_ -match 'AC powered|USB powered|Wireless powered|level' } |
            ForEach-Object { $_.Trim() }) -join ' / '
        Say ("  power/battery = {0}" -f $batt)
        $wake = @((Invoke-Adb @('-s', $ep, 'shell', 'dumpsys', 'power')) -split "`r?`n" |
            Where-Object { $_ -match 'mWakefulness=' } | Select-Object -First 1 | ForEach-Object { $_.Trim() }) -join ' '
        Say ("  screen = {0}" -f $wake)
        $sot = Invoke-Adb @('-s', $ep, 'shell', 'settings', 'get', 'system', 'screen_off_timeout')
        Say ("  screen_off_timeout(system) = {0}" -f $sot)
        Say '  读法：屏幕睡着不影响端口（端口轮换由开关/重启决定），但没插电时电池到 0 会直接断链'
    } else {
        Say '  没有 state=device 的 transport ⇒ 开关侧只能人点手机（脚本无法从零把开关打开）'
    }
    Say '=== 本机状态 ==='
    $st = Read-State
    if ($st) { Say ("  state.json serial={0} updatedAt={1} toggle={2}" -f $st.serial, $st.updatedAt, $st.toggleState) }
    else { Say '  state.json 无' }
    $wp = Get-WatcherPid
    Say ("  watcher={0}" -f $(if ($wp) { "pid=$wp 在跑" } else { 'none' }))
    Say ("  log={0}" -f $LogPath)
    Say ("  actions={0}" -f $ActLogPath)
    Say ("  transcript={0}" -f $TranscriptPath)
    Show-Verdict -Result 'done' -Ep $(if ($live.Count -ge 1) { $live[0].Serial } else { $null }) -Extra 'mode=status_readonly' `
        -Detail 'status 全程只读：只发 devices / mdns services / settings get / dumpsys，不写设备、不断连。'
}

function Do-Probe {
    $mdns = Get-MdnsCandidate
    Say ("mDNS 候选：{0}" -f $(if ($mdns.Count) { ($mdns.Serial -join ', ') } else { '无' }))
    $r = Find-WorkingEndpoint -Preferred $Serial
    if ($r.Ok) {
        if ($Quiet) { Write-Host $r.Serial }
        Show-Verdict -Result 'found' -Ep $r.Serial -Extra "tried=$($r.Tried)" `
            -Detail ("probe 只报告不改动；可用端口 {0}（候选共 {1} 条，含陈旧记录所以要逐个现测）" -f $r.Serial, $r.Tried)
        exit 0
    }
    Show-Verdict -Result 'none' -Ep $null -Extra "tried=$($r.Tried)" `
        -Detail '没有可用端口 ⇒ 无线调试多半被关或已重置，需在手机上点一次（跑 -Action on 会给出点哪一步）'
    exit 3
}

function Do-On {
    # 从任一活着的通道把开关写回 1：优先 USB transport（它不依赖无线调试本身）
    $devs = @(Get-DeviceList | Where-Object { $_.State -eq 'device' })
    $usb = @($devs | Where-Object { $_.Serial -notmatch '^\d+\.\d+\.\d+\.\d+:' -and $_.Serial -notmatch '_adb-tls-connect' })
    $ch = $null
    if ($usb.Count -ge 1) { $ch = $usb[0].Serial }
    elseif ($devs.Count -ge 1) { $ch = $devs[0].Serial }
    if (-not $ch) {
        Show-Verdict -Result 'no_channel' -Ep $null -Extra 'need_human_tap=1' `
            -Detail @'
没有任何 adb 通道 ⇒ 开关只能先在手机上点一次：
  设置 → 开发者选项 → 无线调试 → 打开
  要重新配对就点「使用配对码配对设备」，把弹窗里的 端口 和 6 位配对码 交给：
    pwsh -File <本脚本> -Action pair -PairEndpoint <IP:配对端口> -PairCode <6位码>
点完后再跑一次本动作，或直接把 -Action ensure 当入口。
注意：配对弹窗里的端口是【配对端口】，主页面「IP 地址和端口」那行是【连接端口】，两个不是一个数。
'@
        exit 3
    }
    $before = Read-ToggleState -Ep $ch
    $chan = $(if ($usb.Count -ge 1) { 'usb' } else { 'wireless' })
    if (-not $ReassertToggle) {
        # 没写就不能报 written —— 自测第 5 步抓到上一版把「只读」报成了「已写」
        Show-Verdict -Result 'readonly' -Ep $ch -Extra "channel=$chan toggle_get=$before write=skipped" `
            -Detail ("按 -ReassertToggle 关闭写入：只读到 global adb_wifi_enabled={0}，设备一个字节都没改" -f $before)
    } else {
        $after = Write-ToggleOn -Ep $ch
        Show-Verdict -Result 'written' -Ep $ch -Extra "channel=$chan toggle_set=$before->$after" `
            -Detail ("通过 {0} 通道把 global adb_wifi_enabled 声明为 1，读回 {1}（同值写回实测不换端口、不断会话）" -f $chan, $after)
    }
    $r = Find-WorkingEndpoint -Preferred $Serial
    if ($r.Ok) {
        Write-State -Ep $r.Serial -Note 'on'
        Show-Verdict -Result 'connected' -Ep $r.Serial -Extra "tried=$($r.Tried)" -Detail '开关声明完就把连接端口交给 mDNS 发现，无需读屏'
        exit 0
    }
    Show-Verdict -Result 'no_endpoint_after_on' -Ep $null -Extra 'retry_after_mdns_record' `
        -Detail '开关已声明为 1 但没有可用端口 ⇒ 等 mDNS 出记录后重试，或走 -Action pair 重新配对'
    exit 4
}

function Do-Pair {
    if (-not $PairEndpoint -or -not $PairCode) {
        Show-Verdict -Result 'need_args' -Ep $null -Extra 'pair_endpoint_and_6_digit_code_required' `
            -Detail 'pair 需要 -PairEndpoint <IP:配对端口> -PairCode <6位码>，两者都取自「使用配对码配对设备」弹窗里那一行'
        exit 2
    }
    if ($PairCode -notmatch '^\d{6}$') {
        Show-Verdict -Result 'bad_code' -Ep $null -Extra "code_len=$($PairCode.Length)" `
            -Detail "配对码是 6 位数字，收到 '$PairCode'（端口与配对码来自不同屏，别混着抄）"
        exit 2
    }
    $out = Invoke-Adb @('pair', $PairEndpoint, $PairCode)
    $raw = $out -replace "`r?`n", ' '
    if ($out -match 'Successfully paired') {
        $r = Find-WorkingEndpoint -Preferred $Serial
        if ($r.Ok) {
            Write-State -Ep $r.Serial -Note 'after-pair'
            Show-Verdict -Result 'paired_connected' -Ep $r.Serial -Extra "pair=$PairEndpoint" `
                -Detail ('配对成功，连接端口由 mDNS 自动发现（{0} 个候选里试出 {1}），不用读屏' -f $r.Tried, $r.Serial)
            exit 0
        }
        Show-Verdict -Result 'paired_no_endpoint' -Ep $null -Extra "pair=$PairEndpoint" `
            -Detail '配对成功但连接端口还没出现 ⇒ 让「无线调试」主页面停在前台再跑 ensure'
        exit 4
    }
    # 三种失败形态含义不同，分类回话比把 adb 原文丢给用户有用（现测过 protocol fault 那一支）
    $cls, $hint = switch -Regex ($out) {
        'protocol fault' { 'wrong_port_kind', 'TCP 连上了但答的不是配对服务 ⇒ 你给的极可能是【连接端口】，不是弹窗里的【配对端口】' }
        'unable to.*pairing|failed to read' { 'dialog_gone', '配对服务没答 ⇒ 弹窗可能已关掉或超时，重新打开「使用配对码配对设备」再看端口那行' }
        'Connection refused|10061' { 'no_listener', '该端口没有监听 ⇒ 弹窗已关或已换端口' }
        default { 'unknown', '按 adb 原文判断（见 actions.log 的 raw=）' }
    }
    Show-Verdict -Result 'pair_failed' -Ep $null -Extra "pair=$PairEndpoint class=$cls" `
        -Detail ("$hint ｜ adb 原文：$raw")
    Add-Content -LiteralPath $ActLogPath -Value ('{0} pair_raw endpoint={1} raw={2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $PairEndpoint, $raw) -Encoding utf8
    exit 5
}

function Do-Ensure {
    $r = Find-WorkingEndpoint -Preferred $Serial
    if (-not $r.Ok) {
        Show-Verdict -Result 'unconnected' -Ep $null -Extra "tried=$($r.Tried)" `
            -Detail '一条候选都没试活 ⇒ 跑 -Action on 看手机侧要点哪一步'
        exit 3
    }
    Write-State -Ep $r.Serial -Note 'ensure'
    if ($Quiet) { Write-Host $r.Serial }
    Show-Verdict -Result 'connected' -Ep $r.Serial -Extra "tried=$($r.Tried)" `
        -Detail ("接上了 {0}。跑门就带： -Device {0}" -f $r.Serial)
    # 同一台真机常以两条 transport 出现（ip:port 与 _adb-tls-connect._tcp），门要求显式 -Device
    $dup = @(Get-DeviceList | Where-Object { $_.State -eq 'device' -and $_.Serial -ne $r.Serial })
    if ($dup.Count -ge 1) {
        Show-Verdict -Result 'multi_transport' -Ep $r.Serial -Extra ("others={0}" -f (($dup | ForEach-Object { $_.Serial }) -join ',')) `
            -Detail ('本机另有 {0} 条 transport ⇒ env-check 会判「多个在线设备」，跑门时必须显式 -Device {1}' -f $dup.Count, $r.Serial)
    }
    exit 0
}

function Do-Keep {
    # keep 的心跳只保 TCP 与状态；端口轮换由发现逻辑处理（旧端口一旦 10061 就永久失效，重试同端口是无效的）
    $alive = Get-WatcherPid
    if ($alive -and $alive -ne $PID) {
        Write-Keep "REFUSE_STACK: 已有 keep/watch 在跑 pid=$alive ⇒ 不叠第二个循环（同一台设备两条心跳只会互抢）"
        exit 0
    }
    Set-WatcherPid -Id $PID   # 由子进程自己落盘：Start-Process -PassThru 在这台机器上回过空对象

    $deadline = if ($MaxHours -gt 0) { (Get-Date).AddHours($MaxHours) } else { $null }
    $absentSince = $null
    $beat = 0
    $switchCount = 0
    $ep = (Read-State).serial

    Write-Keep "start action=keep hb=${IntervalSeconds}s slow=${SlowBeatEvery}beats probe=${ProbeSeconds}s wait=${WaitMinutes}m max=${MaxHours}h reassert=$ReassertToggle pid=$PID prefer=$ep"

    try {
        while ($true) {
            if ($deadline -and (Get-Date) -ge $deadline) {
                Write-Keep "END: 到 ${MaxHours}h 存活上限，主动收口（不碰 adb server，不碰设备开关）"
                exit 0
            }

            $state = $null
            if ($ep) { foreach ($d in (Get-DeviceList)) { if ($d.Serial -eq $ep) { $state = $d.State } } }

            if ($ep -and $state -eq 'device') {
                if ($absentSince) {
                    $gap = [int]((Get-Date) - $absentSince).TotalSeconds
                    Write-Keep "RECOVERED serial=$ep 离线 ${gap}s（累计端口切换 $switchCount 次）"
                    $absentSince = $null
                }
                $null = Invoke-Adb @('-s', $ep, 'shell', 'echo', 'hb')
                $beat++
                if ($beat % $SlowBeatEvery -eq 0) {
                    $t = Read-ToggleState -Ep $ep
                    if ($ReassertToggle -and $t -ne '1') {
                        $t2 = Write-ToggleOn -Ep $ep
                        Write-Keep "slowbeat beat=$beat toggle=$t -> 重新声明 1 读回=$t2 serial=$ep"
                        Write-State -Ep $ep -Note 'reassert'
                    } else {
                        Write-Keep "slowbeat beat=$beat hb_ok=$ep toggle=$t"
                    }
                }
                Start-Sleep -Seconds $IntervalSeconds
                continue
            }

            # 掉线态（或刚启动还没端口）：不重试旧端口，直接重发现
            if (-not $absentSince) {
                $absentSince = Get-Date
                Write-Keep "LOST serial=$(if ($ep) { $ep } else { '-' }) state=$(if ($state) { $state } else { 'absent' }) -> 重发现（mDNS 候选 + 上次成功端口）"
                if ($ep) { $null = Invoke-Adb @('disconnect', $ep) }
            }
            if (((Get-Date) - $absentSince).TotalMinutes -ge $WaitMinutes) {
                Write-Keep "GIVEUP: ${WaitMinutes} 分钟内没有可用端口 ⇒ 端口已轮换且 mDNS 无记录，只能人在手机上看一次新端口（旧端口 10061 永久失效）"
                exit 3
            }
            $r = Find-WorkingEndpoint -Preferred $null
            if ($r.Ok) {
                # 找回的可能是同一个端口（掉线但端口没换），也可能是新端口（轮换）。
                # 早先只判「-ne $ep」，于是同端口接回来时被记成 no_endpoint —— 日志会说谎。
                if ($r.Serial -ne $ep) {
                    $switchCount++
                    Write-Keep "NEWPORT serial=$($r.Serial)（第 $switchCount 次端口切换，mDNS 自动接上，无需读屏）"
                    Write-State -Ep $r.Serial -Note 'keep-switch'
                } else {
                    Write-Keep "SAMEPORT serial=$($r.Serial)（端口没换，直接接回）"
                    Write-State -Ep $r.Serial -Note 'keep-heal'
                }
                $ep = $r.Serial
                Start-Sleep -Seconds 1
                continue
            }
            Write-Keep "no_endpoint waited=$([int]((Get-Date) - $absentSince).TotalSeconds)s tried=$($r.Tried) -> ${ProbeSeconds}s 后重发现"
            Start-Sleep -Seconds $ProbeSeconds
        }
    }
    finally {
        $now = Get-WatcherPid
        if ($now -eq $PID) { Clear-WatcherPid }
        Write-Keep "exit pid=$PID serial=$(if ($ep) { $ep } else { '-' }) beats=$beat switches=$switchCount"
    }
}

function Do-Watch {
    $alive = Get-WatcherPid
    if ($alive) {
        Show-Verdict -Result 'already_running' -Ep (Read-State).serial -Extra "watcher_pid=$alive" `
            -Detail "已在跑一个 keep 循环（pid=$alive）⇒ 不叠第二个；要停就 -Action stop"
        exit 0
    }
    # 重新起一个 pwsh 当后台心跳。两条跨平台约束（2026-10-07 CI 在 ubuntu 上现测踩到的）：
    # ① 可执行文件用 $PSHome 拼，别拿 (Get-Process -Id $PID).Path——那条在 Unix 上不保证有值；
    #    且 FilePath 一律传**不带引号**的原值（Windows 会剥引号，Unix 不会 ⇒ 带引号＝文件不存在）。
    # ② ArgumentList 传**数组**让 PowerShell 自己按平台加引号，别预先 join 成一个字符串再塞引号。
    $pwshName = if ($IsWindows) { 'pwsh.exe' } else { 'pwsh' }
    $hostExe = Join-Path $PSHome $pwshName
    if (-not (Test-Path -LiteralPath $hostExe)) { $hostExe = 'pwsh' }
    $childArgs = @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $script:ScriptFile,
        '-Action', 'keep',
        '-IntervalSeconds', $IntervalSeconds,
        '-SlowBeatEvery', $SlowBeatEvery,
        '-ProbeSeconds', $ProbeSeconds,
        '-WaitMinutes', $WaitMinutes,
        '-MaxHours', $MaxHours,
        '-StateDir', $StateDir,
        # -AdbExe 必须一起传下去：漏了它，后台 keep 会用默认 adb 路径而不是调用方指定的那份
        # （2026-10-07 写行为守护时现测：非默认 adb 下 watch 起的子进程碰不到被指定的设备）
        '-AdbExe', $AdbExe
    )
    if (-not $ReassertToggle) { $childArgs += '-ReassertToggle:$false' }
    $spawn = @{ FilePath = $hostExe; ArgumentList = $childArgs }
    if ($IsWindows) { $spawn['WindowStyle'] = 'Hidden' } # -WindowStyle 只在 Windows 上有意义
    $null = Start-Process @spawn

    # pid 由 keep 子进程自己落盘，这里等它写出来再回报（-PassThru 的返回值不可信，见 Get-WatcherPid 注释）
    $wp = $null
    for ($i = 0; $i -lt 20; $i++) {
        Start-Sleep -Milliseconds 500
        $wp = Get-WatcherPid
        if ($wp) { break }
    }
    if (-not $wp) {
        Show-Verdict -Result 'spawn_unconfirmed' -Ep $null -Extra "log=$LogPath" `
            -Detail '起了进程但 10 秒内没读到 pid 文件 ⇒ 看 keep.log 有没有 start 行，必要时 -Action keep 前台跑一次看报错'
        exit 4
    }
    Show-Verdict -Result 'spawned' -Ep $null -Extra "watcher_pid=$wp log=$LogPath" `
        -Detail "后台心跳已起（pid=$wp），日志：$LogPath；当前端口 $((Read-State).serial)"
    exit 0
}

function Do-Stop {
    $alive = Get-WatcherPid
    if (-not $alive) {
        Clear-WatcherPid
        Show-Verdict -Result 'not_running' -Ep $null -Extra 'no_own_watcher' `
            -Detail '没有本脚本起的 keep 进程（adb server 与 HBuilderX 都没被碰）'
        exit 0
    }
    Stop-Process -Id $alive -Force
    Clear-WatcherPid
    Show-Verdict -Result 'stopped' -Ep $null -Extra "killed_pid=$alive" `
        -Detail "只结束了本脚本自己的 keep 进程 pid=$alive；设备侧开关与 adb 连接都没动，需要连接就再跑 ensure"
    exit 0
}

switch ($Action) {
    'status' { Do-Status }
    'probe' { Do-Probe }
    'ensure' { Do-Ensure }
    'on' { Do-On }
    'pair' { Do-Pair }
    'keep' { Do-Keep }
    'watch' { Do-Watch }
    'stop' { Do-Stop }
}
