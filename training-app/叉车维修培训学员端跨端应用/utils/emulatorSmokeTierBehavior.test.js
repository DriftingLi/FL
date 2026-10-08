/**
 * `emulator-smoke.ps1` 那 13 处「直调 adb」按形态分档的有界性 —— 运行期行为守护（#1568 AC 第 2 条，2026-10-08）
 *
 * 为什么这一族单独钉（不是把 ETD 那套复制一遍）：AC1 收的是 `Get-AdbOutput` 那**一个点**（12 个调用方），
 * 本文件收的是它**外面**的 13 处直调 —— 形态完全不同，风险也不同：
 *   · 长等是真的长：`wait-for-device` 冷起实测 **29.2 / 32.0 秒**、`install -r -t`（44.6 MB 基座）**8.5 / 12.3 秒**、
 *     `push` 每 MB **69.7 / 111.1 ms**、`logcat -d` **5.9 / 6.2 秒**
 *     （`docs/verification/tooling/1568/emulator-tier-readings.txt` 与 `-run2.txt`）
 *     ⇒ 套 15 秒的文本档就是**把正常当挂死**（票面 AC 第 3 条明令禁止），完全不套就是「一次不返回整条冒烟就地停住」。
 *   · `adb version` 是 server 级命令：恒拼 `-s <serial>` 会让它答得不一样 —— AC 第 4 条「不改既有判据语义」在这一格最险。
 *   · `logcat -d` 挂死时旧写法的下场是「空日志」⇒ 崩溃计数读成 0 = **把挂死读成无崩溃**。所以 `Get-LogcatSummary`
 *     必须把 `TimedOut` 带出来给页面判据（形状同 C9 给截图定的 `if ($shot.TimedOut) { $pageFail += …`）。
 *
 * 本套件只断言**产物**（落盘日志、返回值、子进程退出码、假 adb 记到的 argv），不断言源码文本；源码面由 C10/C11 钉。
 *   EMT1 install 真挂死 ⇒ 结论落盘（`tier=install` + 该次预算）且 `Install-BaseApk` 照旧判 UNUSABLE / exit 2
 *   EMT2 对照腿：install 正常返回 Success ⇒ 不判失败、日志里零超时
 *   EMT3 push 真挂死 ⇒ `Push-Resources` 回 $false（既有「结果可疑 ⇒ 省页」分支不变），结论点名 push 档
 *   EMT4 对照腿：push 正常返回「1 file pushed」⇒ 回 $true 且无超时
 *   EMT5 logcat 真挂死 ⇒ `Get-LogcatSummary.TimedOut=True`（同一次读数里 FatalCount 确实是 0 —— 险处就在这一格）
 *   EMT6 对照腿：logcat 正常返回含 FATAL ⇒ `TimedOut=False` 且计数照旧（分档不许把判据换掉）
 *   EMT7 boot-wait 档到点后**流程继续**：下一次取文本照样拿得到（挂死不能变成整条链路的终点）
 *   EMT8 server 级命令不带 serial：`version` 那条 argv 里没有 `-s`，而 `shell` 那条有
 *   EMT9 预算是**参数**：install 档从 2 换到 5 ⇒ 日志里的 `callBudgetSeconds` 跟着变（写死在函数体里这条打不中）
 *
 * ⚠️ 手法沿用本仓先例：AST 抽出脚本里的函数定义后**直调**，不执行整篇门脚本（整篇跑会真起模拟器 + 装基座）；
 *   执行核用**真的那一份**（dot-source `lib/auto-screenshot.ps1`）⇒「有界」不是影子造出来的。
 *   载体是**可编程假 adb**（node）：真 adb 不能按需要挂死，而这一族要的正是「真的不返回」那一格。
 *   挂死桩刻意**不用 `ping`** —— `autoScreenshotStabilityBehavior` 的 B6 拿全局 `Get-Process ping` 计数当杀树判据，
 *   同批跑会把**别人的**套件顶红（2026-10-08 现测：用 ping 时 B6 收到 1）。
 *   判据 token 全 ASCII（pwsh 的中文在 OEM 码页下到 Node 侧会变乱码 —— 本仓血账）。
 *   运行前提：需要 `pwsh`；不可用时 fail-closed 抛错，不 skip。平台两支都真跑（Linux 走执行核直启分支）。
 *
 * 在册性：`scripts/lib/contract-tests.ps1` 的 pattern 要能选中本文件（裁定与成本读数写进 PR 正文）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const IS_WIN = process.platform === 'win32';
// 每条腿都要真挂死到点（档位实参给 2–3 秒）+ 真 cmd/node 启动开销；先例 utils/hxLaunchDetachBehavior.test.js:47
jest.setTimeout(240000);

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'emulator-smoke.ps1');
const LIB = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

/** 假 adb：行为与 argv 都由该腿 tmp 里的 state.json 驱动（要的是「按调用形态给不同答案 + 按需要真挂死」）。 */
const FAKE_ADB_JS = [
  "const fs = require('fs'); const path = require('path');",
  "const st = JSON.parse(fs.readFileSync(path.join(__dirname, 'adb-state.json'), 'utf8'));",
  "const argv = process.argv.slice(2);",
  "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
  "const joined = argv.join(' ');",
  "if ((st.hangFor || []).some((k) => joined.includes(k))) { setTimeout(() => {}, 120000); return; }",
  "const say = (s) => process.stdout.write(String(s) + '\\n');",
  "if (joined.includes('install')) { (st.installLines || ['Performing Streamed Install', 'Success']).forEach(say); return; }",
  "if (joined.includes(' push ')) { say(st.pushLine || 'C:/tmp/src/.: 1 file pushed, 0 skipped.'); return; }",
  "if (joined.includes('logcat -d')) { (st.logcatLines || ['--------- beginning of main', 'E AndroidRuntime: FATAL EXCEPTION: main']).forEach(say); return; }",
  "if (joined.includes('logcat -c')) { say('LOGCAT-CLEARED'); return; }",
  "if (joined.includes('wait-for-device')) { say('WAIT-DONE'); return; }",
  "if (joined.includes('version')) { say('Android Debug Bridge version 1.0.41'); return; }",
  "say('STUB-OUT');",
  '',
].join('\n');

const fwd = (p) => String(p).replace(/\\/g, '/');

function makeFixture(state) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'emu-tier-'));
  fs.writeFileSync(path.join(dir, 'fake-adb.js'), FAKE_ADB_JS, 'utf8');
  const exe = path.join(dir, IS_WIN ? 'fake-adb.bat' : 'fake-adb');
  if (IS_WIN) fs.writeFileSync(exe, '@echo off\r\nnode "%~dp0fake-adb.js" %*\r\n', 'utf8');
  else fs.writeFileSync(exe, '#!/bin/sh\nexec node "$(dirname "$0")/fake-adb.js" "$@"\n', { mode: 0o755 });
  fs.writeFileSync(path.join(dir, 'adb-state.json'), JSON.stringify(Object.assign({ hangFor: [] }, state || {}), null, 2), 'utf8');
  const res = path.join(dir, 'res');
  fs.mkdirSync(res, { recursive: true });
  return { dir: fwd(dir), exe: fwd(exe), res: fwd(res), rawDir: dir };
}

/**
 * 一条腿 = 一次 pwsh 子进程：真执行核 + AST 抽出的产品函数 + 该腿的档位与状态。
 * `tiers` 给的是**参数**，不是源码常量 —— EMT9 就是靠换它来证明「预算是参数」。
 */
function leg(body, opts) {
  const o = opts || {};
  const fx = makeFixture(o.state);
  const t = Object.assign({ text: 3, logcat: 3, install: 3, push: 3, bootWait: 3, teardown: 3 }, o.tiers || {});
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

# ── 脚本级前置：与脚本顶部状态块同形（StrictMode 下「先声明再读」是硬要求）
$dir = '${fx.dir}'
$VerifyDir = $dir
$LogPath = Join-Path $dir 'emulator-smoke.log'
$AdbExe = '${fx.exe}'
$Serial = 'emulator-5554'
$Port = 5554
$BootTimeoutSeconds = 30
$PageSettleSeconds = 1
$AdbCallTimeoutSeconds = 3
$AdbTextCallTimeoutSeconds = ${t.text}
$AdbLogcatTimeoutSeconds = ${t.logcat}
$AdbInstallTimeoutSeconds = ${t.install}
$AdbPushTimeoutSeconds = ${t.push}
$AdbBootWaitTimeoutSeconds = ${t.bootWait}
$AdbTeardownTimeoutSeconds = ${t.teardown}
$BaseApk = Join-Path $dir 'fake-base.apk'
Set-Content -LiteralPath $BaseApk -Value 'x' -Encoding utf8
$SkipInstallBaseApk = $false
$ResourcesDir = '${fx.res}'
$ProjectRoot = $dir
$script:AdbTextTimeouts = @()
$script:AdbTextCalls = 0
$script:ShotTimeouts = @()
$script:BaseApkSizeMb = 1
$script:BaseApkPackage = 'io.dcloud.HBuilder'
$script:BootedEmulator = $true
Set-Content -LiteralPath $LogPath -Value 'seed' -Encoding utf8

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
Write-Output 'EMUTIER_PROBE_DONE=1'
`.trim();

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-EncodedCommand', encoded], {
      encoding: 'utf8',
      // 改动前 install 那一支是**真无界**的：靠这个上界把红相的代价钉在几十秒内，而不是让 jest 挂着
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

/** 从**落盘日志**取机检字段：子进程退出（exit 2）之后 stdout 什么都没有了，判据只能打在产物上。 */
function logField(fx, re) {
  const p = path.join(fx.rawDir, 'emulator-smoke.log');
  if (!fs.existsSync(p)) return 'missing';
  const m = fs.readFileSync(p, 'utf8').match(re);
  return m ? m[1] : 'none';
}

describe('emulator-smoke 的 13 处直调按形态分档有界（运行期，#1568 AC 第 2 条）', () => {
  it('EMT1 install 真挂死 ⇒ 结论落盘（tier=install + 该次预算），Install-BaseApk 照旧判 UNUSABLE 并 exit 2', () => {
    const r = leg("Install-BaseApk\nWrite-Output 'INSTALL_RETURNED=1'", {
      state: { hangFor: ['install'] },
      tiers: { install: 2 },
      timeoutMs: 45000,
    });
    expect(r.ok).toBe(false);
    expect(field(r.stdout, 'INSTALL_RETURNED')).toBeNull();
    expect(logField(r.fx, /EMULATOR_SMOKE_RESULT=(\w+)/)).toBe('UNUSABLE');
    expect(logField(r.fx, /ADB_TEXT_TIMEOUT.*tier=(\S+)/)).toBe('install');
    expect(logField(r.fx, /callBudgetSeconds=(\d+)/)).toBe('2');
  }, 130000);

  it('EMT2 对照腿：install 正常返回 Success ⇒ 不判失败、日志里零超时', () => {
    const r = leg(
      [
        'Install-BaseApk',
        "Write-Output 'INSTALL_RETURNED=1'",
        "Write-Output (\"UNUSABLE=\" + $(LogHas 'EMULATOR_SMOKE_RESULT=UNUSABLE'))",
        "Write-Output (\"TIMEOUTS=\" + $(LogHas 'ADB_TEXT_TIMEOUT'))",
      ].join('\n'),
      { state: { installLines: ['Performing Streamed Install', 'Success'] } },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'INSTALL_RETURNED')).toBe('1');
    expect(field(r.stdout, 'UNUSABLE')).toBe('False');
    expect(field(r.stdout, 'TIMEOUTS')).toBe('False');
  }, 130000);

  it('EMT3 push 真挂死 ⇒ Push-Resources 回 $false（既有「结果可疑 ⇒ 省页」分支不变），结论点名 push 档', () => {
    const r = leg(
      [
        '$ok = Push-Resources',
        "Write-Output (\"PUSH_RETURNED=\" + $ok)",
        "Write-Output (\"TIER_PUSH=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=push'))",
      ].join('\n'),
      { state: { hangFor: ['push'] }, tiers: { push: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'PUSH_RETURNED')).toBe('False');
    expect(field(r.stdout, 'TIER_PUSH')).toBe('True');
  }, 130000);

  it('EMT4 对照腿：push 正常返回「1 file pushed」⇒ Push-Resources 回 $true 且无超时', () => {
    const r = leg(
      [
        '$ok = Push-Resources',
        "Write-Output (\"PUSH_RETURNED=\" + $ok)",
        "Write-Output (\"TIMEOUTS=\" + $(LogHas 'ADB_TEXT_TIMEOUT'))",
      ].join('\n'),
      { state: { pushLine: 'C:/tmp/src/.: 1 file pushed, 0 skipped. 48.6 MB/s (8388608 bytes in 0.165s)' } },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'PUSH_RETURNED')).toBe('True');
    expect(field(r.stdout, 'TIMEOUTS')).toBe('False');
  }, 130000);

  it('EMT5 logcat 真挂死 ⇒ Get-LogcatSummary 带出 TimedOut=True（同一次读数里 FatalCount=0 —— 不带出 TimedOut 就是把挂死读成无崩溃）', () => {
    const r = leg(
      [
        '$s = Get-LogcatSummary',
        "Write-Output (\"LOGCAT_TIMEDOUT=\" + $s.TimedOut)",
        "Write-Output (\"LOGCAT_FATAL=\" + $s.FatalCount)",
        "Write-Output (\"TIER_LOGCAT=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=logcat'))",
      ].join('\n'),
      { state: { hangFor: ['logcat -d'] }, tiers: { logcat: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'LOGCAT_TIMEDOUT')).toBe('True');
    expect(field(r.stdout, 'LOGCAT_FATAL')).toBe('0');
    expect(field(r.stdout, 'TIER_LOGCAT')).toBe('True');
  }, 130000);

  it('EMT6 对照腿：logcat 正常返回含 FATAL ⇒ TimedOut=False 且计数照旧（分档不许把判据换掉）', () => {
    const r = leg(
      [
        '$s = Get-LogcatSummary',
        "Write-Output (\"LOGCAT_TIMEDOUT=\" + $s.TimedOut)",
        "Write-Output (\"LOGCAT_FATAL=\" + $s.FatalCount)",
      ].join('\n'),
      { state: { logcatLines: ['--------- beginning of main', 'E AndroidRuntime: FATAL EXCEPTION: main'] } },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'LOGCAT_TIMEDOUT')).toBe('False');
    expect(field(r.stdout, 'LOGCAT_FATAL')).toBe('1');
  }, 130000);

  it('EMT7 boot-wait 档到点后流程继续：下一次取文本照样拿得到（挂死不得成为整条链路的终点）', () => {
    const r = leg(
      [
        "Wait-ForDeviceBounded | Out-Null",
        "Write-Output (\"AFTER_HANG=\" + (Get-AdbOutput @('shell', 'getprop', 'sys.boot_completed')))",
        "Write-Output (\"TIER_BOOTWAIT=\" + $(LogHas 'ADB_TEXT_TIMEOUT.*tier=boot-wait'))",
      ].join('\n'),
      { state: { hangFor: ['wait-for-device'] }, tiers: { bootWait: 2 }, timeoutMs: 45000 },
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'TIER_BOOTWAIT')).toBe('True');
    expect(field(r.stdout, 'AFTER_HANG')).toContain('STUB-OUT');
  }, 130000);

  it('EMT8 server 级命令不带 serial：version 那条 argv 没有 -s，而 shell 那条有（恒拼 -s 就是换判据）', () => {
    const r = leg(
      [
        "Write-Output (\"VERSION_TEXT=\" + (Get-AdbOutput @('version') -NoSerial -Tier 'server'))",
        "Write-Output (\"SHELL_TEXT=\" + (Get-AdbOutput @('shell', 'getprop', 'sys.boot_completed')))",
        "Write-Output (\"VERSION_HAS_S=\" + $(LastArgvLine 'version').Contains('-s'))",
        "Write-Output (\"SHELL_HAS_S=\" + $(LastArgvLine 'getprop').Contains('-s'))",
      ].join('\n'),
      {},
    );
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'VERSION_TEXT')).toContain('Android Debug Bridge version');
    expect(field(r.stdout, 'VERSION_HAS_S')).toBe('False');
    expect(field(r.stdout, 'SHELL_HAS_S')).toBe('True');
  }, 130000);

  it('EMT9 预算是参数：install 档 2 → 5 ⇒ 日志里的 callBudgetSeconds 跟着变（写死在函数体里这条打不中）', () => {
    const a = leg('Install-BaseApk', { state: { hangFor: ['install'] }, tiers: { install: 2 }, timeoutMs: 45000 });
    expect(a.ok).toBe(false);
    expect(logField(a.fx, /callBudgetSeconds=(\d+)/)).toBe('2');
    const b = leg('Install-BaseApk', { state: { hangFor: ['install'] }, tiers: { install: 5 }, timeoutMs: 45000 });
    expect(b.ok).toBe(false);
    expect(logField(b.fx, /callBudgetSeconds=(\d+)/)).toBe('5');
  }, 160000);
});
