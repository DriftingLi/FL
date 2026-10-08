/**
 * `device-capture.ps1` 那 6 处「直调 adb」按形态分档的有界性 —— 运行期行为守护（#1568 AC 第 2 条，2026-10-08）
 *
 * 为什么单独钉一件（而不是把 BSD / EMT 那两套复制一遍）：BSD 收的是**截图**那一格（二进制通道），
 * EMT 收的是**仿真机冒烟**的 13 处直调；本文件收的是 ①a 真机取证脚本里的 6 处**文本 / 管理类**直调，
 * 形态与本仓另两处都不同，且各有一条自己的下场：
 *   · `Get-DeviceList`（`adb devices -l`）是 **server 级**命令 —— 恒拼 `-s <serial>` 就不是同一条调用；
 *   · `Get-LogcatWindow`（`logcat -d` 全量）挂死时旧写法的下场是**空日志**⇒ 崩溃计数读成 0 =
 *     **把挂死读成无崩溃**（这是本族里唯一一处「挂死比无界更危险」的形状，故必须把 `TimedOut` 带出来）；
 *   · `Get-ForegroundInfo` **每页都跑**（只读模式 1 次、切页模式 N+1 次）⇒ 一次不返回就顶掉整次取证；
 *   · `Start-AppPage` / `Resolve-LauncherComponent` 是票面 AC 第 7 条点名的**既有路径**：只加预算，
 *     回退分支与 `am start` 的原文案一律不许动。
 *
 * 档位取值只引用指得到库内产物的现测（`docs/verification/tooling/1568/`）：
 *   `latency-readings.txt` 的 `READ_devices ms=106` / `READ_version ms=178` ⇒ server 级走文本档（15 秒）；
 *   `emulator-tier-readings.txt` 与 `-run2.txt` 的 `shell_dumpsys_activity_activities` 1,168 / 1,662 ms、
 *   `shell_cmd_package_resolve_activity` 457 / 336 ms ⇒ 单行读走文本档；
 *   同两份的 `logcat_dump` 5,896 / 6,186 ms ⇒ 全量 dump 走 logcat 档（60 秒）；基线 `-t 1` 是同形态里更小
 *   的那一个，**没有独立现测**，故与全量同档（这一格写在注释里，不假装量过）。
 *
 * 本套件只断言**产物**（落盘日志、返回值、子进程退出码、假 adb 记到的 argv），不断言源码文本；
 * 源码面由 `deviceCaptureContract.test.js` 的 D13 钉（函数体 scoped 的锚点 + 本文件 `& $AdbExe` 归零棘轮）。
 *   DCT1  server 挂死 ⇒ `Resolve-DeviceSerial` 走 `Fail-Environment`（exit 2），结论点名 `tier=server` 与该次预算
 *   DCT2  对照腿：devices 正常返回一台 ⇒ serial 与 model 照旧解析、日志里零超时
 *   DCT3  dumpsys 挂死 ⇒ `Get-ForegroundInfo` 带出 `TimedOut=True`（`Component` 确实是空 —— 险处就在这一格）
 *   DCT4  对照腿：dumpsys 正常返回 `topResumedActivity` ⇒ `TimedOut=False`、Component/Source/Raw 照旧
 *   DCT5  logcat 基线挂死 ⇒ 窗口起点 0（既有 `logcat=SKIP` 出口不变）**且**能分清「通道不返回」与「缓冲为空」
 *   DCT6  logcat 全量挂死 ⇒ `TimedOut=True` 且同一读数里 `FatalCount=0`（不带出 TimedOut 就是把挂死读成无崩溃）
 *   DCT7  对照腿：全量正常返回含 FATAL + 本包 ANR ⇒ `TimedOut=False` 且两个计数照旧（分档不许把判据换掉）
 *   DCT8  resolve-activity 挂死 ⇒ 仍回退到当前前台组件（AC 第 7 条：既有回退分支不变），结论点名 `tier=resolve`
 *   DCT9  `am start` 挂死 ⇒ `Start-AppPage` 不抛不吞、流程继续（既有路径只加预算）
 *   DCT10 预算是**参数**：文本档从 2 换到 5 ⇒ 日志里的 `callBudgetSeconds` 跟着变（写死在函数体里这条打不中）
 *   DCT11 挂死不得成为链路终点：基线挂一次 ⇒ 随后那次全量 dump 照样拿得到
 *   DCT12 多设备守卫没被接线换掉 ⇒ 两条 online 且未给 -Device 仍 exit 2，且把两条候选都点出来
 *
 * ⚠️ 手法沿用本仓先例：AST 抽出脚本里的函数定义后**直调**，不执行整篇门脚本（整篇跑会去连真机）；
 *   执行核用**真的那一份**（dot-source `lib/auto-screenshot.ps1`）⇒「有界」不是影子造出来的。
 *   载体是**可编程假 adb**（node）：真 adb 不能按需要挂死，而这一族要的正是「真的不返回」那一格。
 *   挂死桩刻意**不用 `ping`** —— `autoScreenshotStabilityBehavior` 的 B6 拿全局 `Get-Process ping` 计数当
 *   杀树判据，同批跑会把**别人的**套件顶红（2026-10-08 现测）。桩寿命 30 秒：有界腿会在预算内把树杀掉，
 *   只有改动前那几支**真无界**的腿会把它留到超时后被 execFileSync 收掉，30 秒足够且不长期占进程。
 *   判据 token 全 ASCII（pwsh 的中文在 OEM 码页下到 Node 侧会变乱码 —— 本仓血账）；要判中文文案时在
 *   PS 侧判完再回带布尔。
 *   运行前提：需要 `pwsh`；不可用时**红**，不 skip（DCT0 就是这一格的显式见证）。平台两支都真跑
 *   （Linux 走执行核的直启分支）—— AC 第 5 条明令禁止「非 Windows 静默跳过」。
 *
 * 在册性：`scripts/lib/contract-tests.ps1` 的 pattern 里 `deviceCapture` 是**子串**匹配，现测同时列出
 *   `deviceCaptureContract` 与本文件 ⇒ 不新增格子（裁定与成本读数写进 PR 正文）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const IS_WIN = process.platform === 'win32';
// 每条挂死腿都要真挂到点（档位在腿里给 2–3 秒）+ 真 cmd/node 启动开销；先例 utils/emulatorSmokeTierBehavior.test.js:42
jest.setTimeout(240000);

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'device-capture.ps1');
const LIB = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

/** 假 adb：行为与 argv 都由该腿 tmp 里的 state.json 驱动（要的是「按调用形态给不同答案 + 按需要真挂死」）。 */
const FAKE_ADB_JS = [
  "const fs = require('fs'); const path = require('path');",
  "const st = JSON.parse(fs.readFileSync(path.join(__dirname, 'adb-state.json'), 'utf8'));",
  "const argv = process.argv.slice(2);",
  "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
  "const joined = argv.join(' ');",
  "if ((st.hangFor || []).some((k) => joined.includes(k))) { setTimeout(() => {}, 30000); return; }",
  "const say = (s) => process.stdout.write(String(s) + '\\n');",
  // adb devices -l：状态列必须落在产品解析器的白名单里，否则会被当成噪声行丢掉（DCT2 就判不出来）
  "if (joined.includes('devices')) {",
  "  (st.devicesLines || ['List of devices attached',",
  "    'FAKE-SERIAL device product:wayland model:MI_2510DRK44C device:wayland transport_id:7']).forEach(say);",
  "  return;",
  "}",
  "if (joined.includes('dumpsys activity activities')) {",
  "  (st.foregroundLines || ['ACTIVITY MANAGER ACTIVITIES (dumpsys activity activities)',",
  "    ' topResumedActivity=ActivityRecord{1a2b3c u0 io.dcloud.uniappx/io.dcloud.uniappx.MainActivity t123}']).forEach(say);",
  "  return;",
  "}",
  // 基线与全量差一个 `-t 1`：两支必须能被分开挂死（DCT5 / DCT11 靠这条）
  "if (joined.includes('logcat -d -v epoch -t 1')) { (st.baselineLines || ['  1759000000.100  100  200 I Tag: seed']).forEach(say); return; }",
  "if (joined.includes('logcat -d -v epoch')) { (st.windowLines || ['--------- beginning of main']).forEach(say); return; }",
  "if (joined.includes('resolve-activity')) { say(st.resolveLine || 'io.dcloud.uniappx/io.dcloud.uniappx.PandoraEntry'); return; }",
  "if (joined.includes('am start')) { (st.amStartLines || ['Starting: Intent { act=android.intent.action.VIEW dat=uniapp://pages/login/login }']).forEach(say); return; }",
  "say('STUB-OUT');",
  '',
].join('\n');

const fwd = (p) => String(p).replace(/\\/g, '/');

function makeFixture(state) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-tier-'));
  fs.writeFileSync(path.join(dir, 'fake-adb.js'), FAKE_ADB_JS, 'utf8');
  const exe = path.join(dir, IS_WIN ? 'fake-adb.bat' : 'fake-adb');
  if (IS_WIN) fs.writeFileSync(exe, '@echo off\r\nnode "%~dp0fake-adb.js" %*\r\n', 'utf8');
  else fs.writeFileSync(exe, '#!/bin/sh\nexec node "$(dirname "$0")/fake-adb.js" "$@"\n', { mode: 0o755 });
  fs.writeFileSync(path.join(dir, 'adb-state.json'), JSON.stringify(Object.assign({ hangFor: [] }, state || {}), null, 2), 'utf8');
  return { dir: fwd(dir), exe: fwd(exe), rawDir: dir };
}

/**
 * 一条腿 = 一次 pwsh 子进程：真执行核 + AST 抽出的产品函数 + 该腿的档位与假 adb 状态。
 * `tiers` 给的是**参数**，不是源码常量 —— DCT10 就是靠换它来证明「预算是参数」。
 */
function leg(body, opts) {
  const o = opts || {};
  const fx = makeFixture(o.state);
  const t = Object.assign({ text: 3, logcat: 3 }, o.tiers || {});
  const extra = o.setup || '';
  const script = `
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
. "${fwd(LIB)}"
$src = Get-Content -LiteralPath "${fwd(SCRIPT)}" -Raw
$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)
$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))
foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }
Write-Output ("EXTRACTED=" + $fns.Count)

# ── 脚本级前置：与脚本顶部状态块**同形**（StrictMode 下「先声明再读」是硬要求；
#    少一行就会把「未声明」的报错当成功能红 —— 本仓 #1562 BSD 套件第一版正是这样暴露的）
$dir = '${fx.dir}'
$VerifyRoot = $dir
$LogPath = Join-Path $dir 'device-capture.log'
$AdbExe = '${fx.exe}'
$AdbPath = '${fx.exe}'
$AdbCallTimeoutSeconds = 3
$AdbTextCallTimeoutSeconds = ${t.text}
$AdbLogcatTimeoutSeconds = ${t.logcat}
$Device = ''
$Pages = 'pages/login/login'
$LogcatSeconds = 0
$script:Serial = 'FAKE-SERIAL'
$script:DeviceModel = ''
$script:ChildProcesses = @()
$script:DrawingReady = $false
$script:FinalStatus = ''
$script:Foreground = [pscustomobject]@{ Component = ''; Raw = ''; Source = ''; TimedOut = $false }
$script:TargetPackage = 'io.dcloud.uniappx'
$script:ShotRecords = @()
$script:ShotTimeouts = @()
$script:AdbTextCalls = 0
$script:AdbTextTimeouts = @()
$script:SkippedPages = @()
$script:WindowStart = 0.0
$script:WindowKnown = $false
$script:FinalTotals = [pscustomobject]@{
    Lines = 0; Total = 0; WindowLines = 0; WindowStart = 0.0; WindowKnown = $false
    FatalCount = 0; AnrAllCount = 0; AnrPkgCount = 0
    RuntimeCrashes = 0; ProcessDeaths = 0; FatalSamples = @(); AnrSamples = @(); TimedOut = $false
}
$script:PageResultsAll = @()
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding UTF8
${extra}
function LogHas([string]$Pattern) {
  $raw = Get-Content -LiteralPath $LogPath -Raw -Encoding UTF8
  return $(if ($raw -match $Pattern) { 'True' } else { 'False' })
}
function LastArgvLine([string]$Needle) {
  $p = Join-Path $dir 'calls.log'
  if (-not (Test-Path -LiteralPath $p)) { return 'none' }
  $lines = @(Get-Content -LiteralPath $p -Encoding UTF8 | Where-Object { $_ -like ('*' + $Needle + '*') })
  return $(if ($lines.Count -gt 0) { $lines[-1] } else { 'none' })
}
${body}
Write-Output 'DCT_PROBE_DONE=1'
`.trim();

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  const psArgs = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 Write-Host 的 InformationRecord 在 stderr 上被序列化成 `#< CLIXML>`，
  // 取证读数整段不可读（先例 utils/deviceCaptureBoundedShotBehavior.test.js:58）
  if (IS_WIN) psArgs.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  psArgs.push('-EncodedCommand', encoded);
  try {
    const stdout = execFileSync('pwsh', psArgs, {
      encoding: 'utf8',
      // 改动前这 6 支是**真无界**的：这个上界把红相的代价钉在几十秒内，而不是让 jest 挂在那里
      timeout: o.timeoutMs || 60000,
      windowsHide: true,
      maxBuffer: 16 * 1024 * 1024,
    });
    return { ok: true, code: 0, stdout: String(stdout), fx };
  } catch (e) {
    return { ok: false, code: e.status === undefined ? null : e.status, stdout: String(e.stdout || ''), fx };
  }
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp('^' + key + '=(.*)$', 'm'));
  return m ? m[1].trim() : null;
}

/** 从**落盘日志**取机检字段：子进程 `exit 2` 之后 stdout 什么都没有了，判据只能打在产物上。 */
function logField(fx, re) {
  const p = path.join(fx.rawDir, 'device-capture.log');
  if (!fs.existsSync(p)) return 'missing';
  const m = fs.readFileSync(p, 'utf8').match(re);
  return m ? m[1] : 'none';
}

describe('device-capture 的 6 处直调按形态分档有界（运行期，#1568 AC 第 2 条）', () => {
  it('DCT0 前置：pwsh 真的可用（不可用即红，不许静默 skip）', () => {
    const r = leg("Write-Output ('PWSH=' + $PSVersionTable.PSVersion.Major)", {});
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'PWSH')).toBe('7');
  }, 60000);

  it('DCT1 server 级挂死 ⇒ 结论落盘（tier=server + 该次预算），Resolve-DeviceSerial 照旧判 UNUSABLE 并 exit 2', () => {
    const r = leg(
      [
        "$s = Resolve-DeviceSerial -Requested ''",
        "Write-Output (\"SERIAL=\" + $s)",
      ].join('\n'),
      { state: { hangFor: ['devices'] }, tiers: { text: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(false);
    expect(r.code).toBe(2);
    expect(field(r.stdout, 'SERIAL')).toBeNull();
    expect(logField(r.fx, /DEVICE_CAPTURE_RESULT=(\w+)/)).toBe('UNUSABLE');
    expect(logField(r.fx, /ADB_TEXT_TIMEOUT.*tier=(\S+)/)).toBe('server');
    expect(logField(r.fx, /callBudgetSeconds=(\d+)/)).toBe('2');
    // server 级命令不得被拼上 -s：那已经不是同一条调用了（AC 第 4 条）
    const hung = logField(r.fx, /ADB_TEXT_TIMEOUT call=([^\n]*)/);
    expect(hung).not.toBe('none');
    expect(hung.includes('devices')).toBe(true);
    expect(hung.includes('-s')).toBe(false);
  }, 130000);

  it('DCT2 对照腿：devices 正常返回一台 ⇒ serial 与 model 照旧解析，日志里零超时', () => {
    const r = leg(
      [
        "$s = Resolve-DeviceSerial -Requested ''",
        "Write-Output (\"SERIAL=\" + $s)",
        "Write-Output (\"MODEL=\" + $script:DeviceModel)",
        "Write-Output (\"TIMEOUTS=\" + $(LogHas 'ADB_TEXT_TIMEOUT'))",
        "Write-Output (\"DEVICES_HAS_S=\" + $(LastArgvLine 'devices').Contains('-s'))",
      ].join('\n'),
      {},
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'SERIAL')).toBe('FAKE-SERIAL');
    expect(field(r.stdout, 'MODEL')).toBe('MI_2510DRK44C');
    expect(field(r.stdout, 'TIMEOUTS')).toBe('False');
    expect(field(r.stdout, 'DEVICES_HAS_S')).toBe('False');
  }, 130000);

  it('DCT3 dumpsys 挂死 ⇒ Get-ForegroundInfo 带出 TimedOut=True（同一格 Component 确实是空 —— 不带出就是把挂死读成「没前台」）', () => {
    const r = leg(
      [
        '$fg = Get-ForegroundInfo',
        'Write-Output ("FG_TIMEDOUT=" + [bool]$fg.TimedOut)',
        "Write-Output (\"FG_COMPONENT=\" + $fg.Component)",
        "Write-Output (\"TIER_READ=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=read'))",
      ].join('\n'),
      { state: { hangFor: ['dumpsys activity'] }, tiers: { text: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'FG_TIMEDOUT')).toBe('True');
    expect(field(r.stdout, 'FG_COMPONENT')).toBe('');
    expect(field(r.stdout, 'TIER_READ')).toBe('True');
  }, 130000);

  it('DCT4 对照腿：dumpsys 正常返回 topResumedActivity ⇒ TimedOut=False 且 Component/Source/Raw 照旧（分档不许把判据换掉）', () => {
    const r = leg(
      [
        '$fg = Get-ForegroundInfo',
        'Write-Output ("FG_TIMEDOUT=" + [bool]$fg.TimedOut)',
        "Write-Output (\"FG_COMPONENT=\" + $fg.Component)",
        "Write-Output (\"FG_SOURCE=\" + $fg.Source)",
        "Write-Output (\"FG_RAW_HAS_ACTIVITY=\" + $fg.Raw.Contains('topResumedActivity'))",
      ].join('\n'),
      { state: {} },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'FG_TIMEDOUT')).toBe('False');
    expect(field(r.stdout, 'FG_COMPONENT')).toBe('io.dcloud.uniappx/io.dcloud.uniappx.MainActivity');
    expect(field(r.stdout, 'FG_SOURCE')).toBe('topResumedActivity');
    expect(field(r.stdout, 'FG_RAW_HAS_ACTIVITY')).toBe('True');
  }, 130000);

  it('DCT5 logcat 基线挂死 ⇒ 窗口起点仍是 0（既有 logcat=SKIP 出口不变）但能点名「通道不返回」与「缓冲为空」是两件事', () => {
    const r = leg(
      [
        '$b = Get-LogcatBaseline',
        "Write-Output (\"BL_EPOCH=\" + $b.Epoch)",
        'Write-Output ("BL_TIMEDOUT=" + [bool]$b.TimedOut)',
        "Write-Output (\"TIER_LOGCAT=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=logcat'))",
      ].join('\n'),
      { state: { hangFor: ['-t 1'] }, tiers: { logcat: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'BL_EPOCH')).toBe('0');
    expect(field(r.stdout, 'BL_TIMEDOUT')).toBe('True');
    expect(field(r.stdout, 'TIER_LOGCAT')).toBe('True');
  }, 130000);

  it('DCT6 logcat 全量挂死 ⇒ TimedOut=True 且同一读数里 FatalCount=0（险处：空日志会被读成「无崩溃」）', () => {
    const r = leg(
      [
        "$w = Get-LogcatWindow -SinceEpoch 1759000000.0 -MaxSeconds 0 -Pkg 'io.dcloud.uniappx'",
        'Write-Output ("W_TIMEDOUT=" + [bool]$w.TimedOut)',
        "Write-Output (\"W_FATAL=\" + $w.FatalCount)",
        "Write-Output (\"W_LINES=\" + $w.WindowLines)",
        "Write-Output (\"TIER_LOGCAT=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=logcat'))",
      ].join('\n'),
      { state: { hangFor: ['logcat'] }, tiers: { logcat: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'W_TIMEDOUT')).toBe('True');
    expect(field(r.stdout, 'W_FATAL')).toBe('0');
    expect(field(r.stdout, 'TIER_LOGCAT')).toBe('True');
  }, 130000);

  it('DCT7 对照腿：全量正常返回含 FATAL 与本包 ANR ⇒ TimedOut=False 且两个计数照旧', () => {
    const r = leg(
      [
        "$w = Get-LogcatWindow -SinceEpoch 1759000000.0 -MaxSeconds 0 -Pkg 'io.dcloud.uniappx'",
        'Write-Output ("W_TIMEDOUT=" + [bool]$w.TimedOut)',
        "Write-Output (\"W_FATAL=\" + $w.FatalCount)",
        "Write-Output (\"W_ANR_PKG=\" + $w.AnrPkgCount)",
        "Write-Output (\"W_WINDOW_LINES=\" + $w.WindowLines)",
      ].join('\n'),
      {
        state: {
          windowLines: [
            '--------- beginning of main',
            '1759000001.100  100  200 E AndroidRuntime: FATAL EXCEPTION: main',
            '1759000002.100  100  210 E ActivityManager: ANR in io.dcloud.uniappx (io.dcloud.uniappx/.MainActivity)',
            '1758999999.050  100  220 I Tag: before-window-must-be-excluded',
          ],
        },
      },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'W_TIMEDOUT')).toBe('False');
    expect(field(r.stdout, 'W_FATAL')).toBe('1');
    expect(field(r.stdout, 'W_ANR_PKG')).toBe('1');
    // 窗口起点之前的行必须仍在窗口外（这条与超时无关，是既有判据的不变式）
    expect(field(r.stdout, 'W_WINDOW_LINES')).toBe('2');
  }, 130000);

  it('DCT8 resolve-activity 挂死 ⇒ 仍回退到当前前台组件（AC 第 7 条：既有回退分支不变），结论点名 tier=resolve', () => {
    const r = leg(
      [
        '$launcher = Resolve-LauncherComponent',
        "Write-Output (\"LAUNCHER=\" + $launcher)",
        "Write-Output (\"TIER_RESOLVE=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=resolve'))",
      ].join('\n'),
      {
        state: { hangFor: ['resolve-activity'] },
        tiers: { text: 2 },
        timeoutMs: 45000,
        setup:
          "$script:Foreground = [pscustomobject]@{ Component = 'io.dcloud.uniappx/io.dcloud.uniappx.MainActivity'; Raw = 'seed'; Source = 'topResumedActivity'; TimedOut = $false }",
      },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'LAUNCHER')).toBe('io.dcloud.uniappx/io.dcloud.uniappx.MainActivity');
    expect(field(r.stdout, 'TIER_RESOLVE')).toBe('True');
  }, 130000);

  it('DCT9 am start 挂死 ⇒ Start-AppPage 不抛不吞、流程继续（既有路径只加预算），结论点名 tier=am-start', () => {
    const r = leg(
      [
        "Start-AppPage -Page 'pages/login/login'",
        "Write-Output 'STARTAPP_RETURNED=1'",
        "Write-Output (\"TIER_AMSTART=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=am-start'))",
      ].join('\n'),
      { state: { hangFor: ['am start'] }, tiers: { text: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'STARTAPP_RETURNED')).toBe('1');
    expect(field(r.stdout, 'TIER_AMSTART')).toBe('True');
  }, 130000);

  it('DCT10 预算是参数：文本档 2 → 5 ⇒ 日志里的 callBudgetSeconds 跟着变（写死在函数体里这条打不中）', () => {
    const a = leg('$null = Get-ForegroundInfo', { state: { hangFor: ['dumpsys activity'] }, tiers: { text: 2 }, timeoutMs: 45000 });
    expect(a.ok).toBe(true);
    expect(logField(a.fx, /callBudgetSeconds=(\d+)/)).toBe('2');
    const b = leg('$null = Get-ForegroundInfo', { state: { hangFor: ['dumpsys activity'] }, tiers: { text: 5 }, timeoutMs: 45000 });
    expect(b.ok).toBe(true);
    expect(logField(b.fx, /callBudgetSeconds=(\d+)/)).toBe('5');
  }, 160000);

  it('DCT11 挂死不得成为链路终点：基线挂一次 ⇒ 随后那次全量 dump 照样拿得到', () => {
    const r = leg(
      [
        '$b = Get-LogcatBaseline',
        'Write-Output ("BL_TIMEDOUT=" + [bool]$b.TimedOut)',
        "$w = Get-LogcatWindow -SinceEpoch 1759000000.0 -MaxSeconds 0 -Pkg 'io.dcloud.uniappx'",
        'Write-Output ("W_TIMEDOUT=" + [bool]$w.TimedOut)',
        "Write-Output (\"AFTER_HANG_TOTAL=\" + $w.Total)",
        "Write-Output (\"TIMEOUT_COUNT=\" + @($script:AdbTextTimeouts).Count)",
      ].join('\n'),
      {
        state: {
          hangFor: ['-t 1'],
          windowLines: ['1759000001.100  100  200 I Tag: alive'],
        },
        tiers: { logcat: 2 },
        timeoutMs: 45000,
      },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'BL_TIMEDOUT')).toBe('True');
    expect(field(r.stdout, 'W_TIMEDOUT')).toBe('False');
    expect(field(r.stdout, 'AFTER_HANG_TOTAL')).toBe('1');
    expect(field(r.stdout, 'TIMEOUT_COUNT')).toBe('1');
  }, 130000);

  it('DCT12 多设备守卫没被接线换掉：两条 online 且未给 -Device ⇒ 仍 exit 2，且把两条候选都点出来', () => {
    const r = leg(
      [
        "$s = Resolve-DeviceSerial -Requested ''",
        "Write-Output (\"SERIAL=\" + $s)",
      ].join('\n'),
      {
        state: {
          devicesLines: [
            'List of devices attached',
            'FAKE-SERIAL-A device product:wayland model:MI_A device:wayland transport_id:1',
            'FAKE-SERIAL-B device product:wayland model:MI_B device:wayland transport_id:2',
          ],
        },
      },
    );
    expect(r.ok).toBe(false);
    expect(r.code).toBe(2);
    const raw = fs.readFileSync(path.join(r.fx.rawDir, 'device-capture.log'), 'utf8');
    expect(logField(r.fx, /DEVICE_CAPTURE_RESULT=(\w+)/)).toBe('UNUSABLE');
    expect(raw.includes('FAKE-SERIAL-B')).toBe(true);
    expect(raw.includes('-Device')).toBe(true);
  }, 130000);
});
