/**
 * 真机无线调试入口的**行为**守护（scripts/wireless-debug.ps1，#1564）—— #1998 拆分件 4/4
 *
 * 原单套件 utils/wirelessDebugBehavior.test.js（202.5s，run 40111032652）是 CI 行为腿四片的关键路径，
 * 按用例形态拆成四件，本件 = stop 收挂在飞的 adb 调用（B7）。拆分口径：
 * - 用例与断言**逐字沿用**原套件，只重组不重写；四件合计覆盖原 B1–B7 全部七条（不重叠不遗漏）。
 * - 夹具段逐字复制到每个拆分件——这是有意的：scripts/classify-guards.mjs 按文件自身的
 *   「代码级执行调用 + 仓内载体字面量」判行为腿，藏进共享夹具会被判成接线腿。
 *   全程在 ADR-0042 边界内：不动 -i、不动分片机制、不删改被测断言、不新建第三份被测面清单。
 * - 本件单列理由：B7 与 B5 同属 stop 生命周期，但 B7 的 hang 口径要先让在飞调用停半空、stop 后再轮询桩进程清零，CI 实测远比 B5 贵
 *   （run 40443976624：B5+B7 合并件 185.6s 超腿内总/4，不单独成件则四片配平无解）；其回归钉语义见下。
 * - 其余三件：wirelessDebugEnsureBehavior.test.js（B6/B1/B2/B4）、wirelessDebugKeepRecoverBehavior.test.js（B3）、
 *   wirelessDebugStopReapBehavior.test.js（B5 按 pid 收 stop）
 * - 拆分后各件耗时与推导读 utils/jestSuiteTimings.json 的 note（CI run 40452269933 重采）。
 *
 *   B7 `stop` 收 keep 时必须连**正在跑的那次 adb 调用**一起收（#1718）。
 *      血账：只杀 keep 的 pid 时，Windows 上那次调用还多一层 cmd.exe；桩进程活下来并握着 keep 从
 *      调用方（jest）继承的 stdout 管道 ⇒ 汇总行永远不打、进程挂 20+ 分钟。这条腿先让 `shell echo hb`
 *      停在半空证明确有调用在飞，再断言 stop 之后夹具目录名不再出现在任何进程命令行里。
 * 平台口径（2026-10-07 修订，**1条在 Windows 与 CI 的 ubuntu 上都真跑**）：假载体按平台落成
 * `.bat` / `sh` 两种壳，pwsh 两侧都在。上一版把 B1–B5 写成「非 Windows 直接 return」⇒ CI 里那五条是
 * **永不执行的空体**（正是 `docs/agents/guards.md` #1156 要防的「全绿可以恒真」），而当时补的那条
 * 「非 Windows 不得报成功」断的是**代码里并不存在的平台禁令**：CI 现测 ubuntu 上脚本照跑、照报
 * `result=connected` ⇒ 该断言判红（run 37595419950，`Tests: 1 failed, 3073 passed, 3074 total`）。
 * 真机上的 Windows 实链读数（受控断链 2 秒自愈那一段）记在 #1564 的 PR 正文。
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync, spawn } = require('child_process');
// 读文件一律经共享读者（ADR-0019 的读取层归一真源）：CI 的 `check-contract-read.mjs` 规则二
// 判的是「读者助手体内裸读 fs.readFileSync」，**不问读的是不是仓内文件**（本夹具在 os.tmpdir() 里，
// 一样要归一）。2026-10-07 现测：run 37598814948 的 mobile-test 就红在这四条助手上（3074 条用例全绿，
// 挂的是那一步独立检查）。顺带这也是真需求——Windows 上 keep.log 由 Add-Content 写成 CRLF。
const { readText } = require('./utsHarness');

const IS_WIN = process.platform === 'win32';
const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**：`scripts/classify-guards.mjs` 按文件自身的
// 「代码级执行调用 + 仓内载体引用」判行为守护 ⇒ 两样都不能藏进共享夹具（藏了就被判成接线守护）。
const SCRIPT_REL = 'scripts/wireless-debug.ps1';
const SCRIPT_ABS = path.join(ROOT, SCRIPT_REL);

const LIVE = '192.168.10.51:39527';
const DEAD = '192.168.10.51:37611';
const GONE = '192.168.10.51:41999'; // 记录表里根本没有、只出现在 state.json / -Serial 里的死端口

/** 本文件起过的夹具目录（#1718：afterEach 要按目录名把漏下的桩进程收干净）。 */
const FIXTURES = [];

/** 同步睡一小段：用 SharedArrayBuffer 阻塞，不再额外起进程（计时不该进判据）。 */
const sleep = (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);

/** 假 adb：adb-state.json 控场，calls.log 记下每一次 argv。 */
const FAKE_ADB_JS = [
  "const fs = require('fs');",
  "const path = require('path');",
  "const S = path.join(__dirname, 'adb-state.json');",
  "const st = JSON.parse(fs.readFileSync(S, 'utf8'));",
  "st.connected = st.connected || [];",
  "st.devicesCalls = st.devicesCalls || 0;",
  "const save = () => fs.writeFileSync(S, JSON.stringify(st, null, 2));",
  "const say = (s) => process.stdout.write(s + '\\n');",
  "const rec = (ep) => (st.records || []).find((r) => r.ep === ep);",
  "const argv = process.argv.slice(2);",
  "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
  "if (argv[0] === '-s') {",
  "  const ep = argv[1];",
  "  const rest = argv.slice(2).join(' ');",
  "  const live = st.connected.includes(ep) && rec(ep) && rec(ep).alive;",
  // #1718：让某一次 `shell echo hb` 停在半空（真机上的「正跑到一半」），用来复现「keep 被杀、调用还在飞」
  "  if (/^shell echo / .test(rest) && st.hangEchoMs) { Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, st.hangEchoMs); }",
  "  if (/^shell echo / .test(rest)) { live ? say(rest.split(' ')[2]) : say('error: device not found'); save(); return; }",
  "  if (rest.includes('settings get global adb_wifi_enabled')) { say(String(st.toggle)); save(); return; }",
  "  if (rest.includes('settings put global adb_wifi_enabled')) {",
  "    const v = rest.trim().split(' ').pop();",
  "    st.toggleWrites = (st.toggleWrites || []).concat([v]); st.toggle = v; say(''); save(); return;",
  "  }",
  "  say('stub'); save(); return;",
  "}",
  "if (argv[0] === 'version') { say('Android Debug Bridge version 1.0.41'); return; }",
  "if (argv[0] === 'mdns' && argv[1] === 'services') {",
  "  say('List of discovered mdns services');",
  "  (st.records || []).forEach((r, i) => say('adb-b32d8398-' + i + '\\t_adb-tls-connect._tcp\\t' + r.ep));",
  "  return;",
  "}",
  "if (argv[0] === 'devices') {",
  "  st.devicesCalls += 1;",
  "  say('List of devices attached');",
  "  if (st.devicesCalls > (st.emptyDevicesFirst || 0)) {",
  "    st.connected.forEach((ep) => { if (rec(ep) && rec(ep).alive) say(ep + '\\tdevice'); });",
  "  }",
  "  save(); return;",
  "}",
  "if (argv[0] === 'connect') {",
  "  const ep = argv[1];",
  "  if (rec(ep) && rec(ep).alive) { if (!st.connected.includes(ep)) st.connected.push(ep); say('connected to ' + ep); }",
  "  else say('cannot connect to ' + ep + ': Connection refused (10061)');",
  "  save(); return;",
  "}",
  "if (argv[0] === 'disconnect') { st.connected = st.connected.filter((x) => x !== argv[1]); say('disconnected ' + argv[1]); save(); return; }",
  "if (argv[0] === 'pair') {",
  "  if (st.pairEndpoint === argv[1] && String(argv[2] || '').length === 6) { say('Successfully paired to ' + argv[1]); return; }",
  "  say('error: protocol fault (could not read status message): No error'); return;",
  "}",
  "say('stub-ignored ' + argv.join(' '));",
  '',
].join('\n');

function makeFixture(over) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wd-adb-'));
  const stateDir = path.join(dir, 'state');
  fs.mkdirSync(stateDir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'fake-adb.js'), FAKE_ADB_JS, 'utf8');
  const bat = path.join(dir, IS_WIN ? 'fake-adb.bat' : 'fake-adb');
  if (IS_WIN) fs.writeFileSync(bat, '@echo off\r\nnode "%~dp0fake-adb.js" %*\r\n', 'utf8');
  else fs.writeFileSync(bat, '#!/bin/sh\nexec node "$(dirname "$0")/fake-adb.js" "$@"\n', { mode: 0o755 });
  const state = Object.assign(
    { records: [{ ep: DEAD, alive: false }, { ep: LIVE, alive: true }], connected: [], toggle: '1', emptyDevicesFirst: 0, devicesCalls: 0 },
    over || {},
  );
  fs.writeFileSync(path.join(dir, 'adb-state.json'), JSON.stringify(state, null, 2), 'utf8');
  fs.writeFileSync(path.join(stateDir, 'state.json'), JSON.stringify({ serial: null, portHistory: [] }), 'utf8');
  const fx = { dir, stateDir, bat };
  FIXTURES.push(fx);
  return fx;
}

const adbState = (fx) => JSON.parse(readText(path.join(fx.dir, 'adb-state.json')));
const callsParsed = (fx) => {
  const p = path.join(fx.dir, 'calls.log');
  if (!fs.existsSync(p)) return [];
  return readText(p).split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l).join(' '));
};
const toolState = (fx) => {
  const p = path.join(fx.stateDir, 'state.json');
  return fs.existsSync(p) ? JSON.parse(readText(p)) : null;
};
const keepLog = (fx) => {
  const p = path.join(fx.stateDir, 'keep.log');
  return fs.existsSync(p) ? readText(p) : '';
};
const seedToolState = (fx, serial) => fs.writeFileSync(
  path.join(fx.stateDir, 'state.json'),
  JSON.stringify({ serial, portHistory: serial ? [serial] : [] }),
  'utf8',
);
/** 进程还在不在（Windows 上问 pwsh，不用 process.kill 的 0 信号——它对已死/无权都抛）。 */
const pidAlive = (pid) => {
  const out = execFileSync('pwsh', ['-NoProfile', '-Command',
    `if (Get-Process -Id ${pid} -ErrorAction SilentlyContinue) { 'ALIVE' } else { 'GONE' }`],
  { encoding: 'utf8', windowsHide: true });
  return String(out).trim().includes('ALIVE');
};

/**
 * 命令行里带夹具目录的进程 = 这个夹具派生出来的桩进程（#1718）。
 * 覆盖 keep 自己、Windows 上包 `fake-adb.bat` 的 cmd.exe、跑 `fake-adb.js` 的 node。
 */
function stubProcessPids(dir) {
  if (IS_WIN) {
    // `-ne $PID`：查询进程自己的命令行里也带着 `$n = '<dir>'`，不排除它就会把自己数成活口
    const cmd = `$n = '${dir.replace(/'/g, "''")}'; `
      + 'Get-CimInstance Win32_Process | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($n) } '
      + '| ForEach-Object { $_.ProcessId }';
    const out = execFileSync('pwsh', ['-NoProfile', '-Command', cmd], { encoding: 'utf8', windowsHide: true });
    return out.split(/\s+/).filter(Boolean).map(Number).filter((n) => Number.isInteger(n) && n > 0);
  }
  const out = execFileSync('ps', ['-eo', 'pid=,args='], { encoding: 'utf8' });
  return out.split('\n')
    .filter((l) => l.includes(dir))
    .map((l) => Number(l.trim().split(/\s+/)[0]))
    .filter((n) => Number.isInteger(n) && n > 0);
}

/**
 * 兜底清扫：把夹具目录派生出来的桩进程按**进程树**收掉，轮询到清空（最多 5 秒）。
 * 为什么不只靠 `child.kill()`：kill 只打 keep 的 pid，keep 正在跑的那次 adb 调用会活下来，
 * 并握着 keep 从本进程继承的 stdout 管道 ⇒ jest 永远等不到 EOF（#1718 的 20+ 分钟挂起）。
 * Windows 用 `taskkill /T`（一次收整棵树）；非 Windows 逐个 SIGKILL（父进程先死，子进程仍可按 pid 杀）。
 */
function killStubProcesses(dir) {
  const t0 = Date.now();
  while (Date.now() - t0 < 5000) {
    const pids = stubProcessPids(dir);
    if (!pids.length) return true;
    for (const pid of pids) {
      try {
        if (IS_WIN) execFileSync('taskkill', ['/PID', String(pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true });
        else process.kill(pid, 'SIGKILL');
      } catch (e) { /* 已自行退出 */ }
    }
    sleep(100);
  }
  return stubProcessPids(dir).length === 0;
}

function runTool(fx, extraArgs, timeout) {
  try {
    const stdout = execFileSync(
      'pwsh',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', SCRIPT_ABS,
        '-AdbExe', fx.bat, '-StateDir', fx.stateDir, '-Ascii'].concat(extraArgs),
      { encoding: 'utf8', timeout: timeout || 90000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 },
    );
    return { status: 0, stdout };
  } catch (e) {
    return { status: typeof e.status === 'number' ? e.status : 9, stdout: String(e.stdout || '') + String(e.stderr || ''), message: e.message };
  }
}
const verdict = (out, action) => {
  const re = new RegExp('WIRELESS_DEBUG action=' + action + ' result=(\\S+) serial=(\\S+)', 'g');
  return [...out.matchAll(re)].map((m) => ({ result: m[1], serial: m[2] }));
};

describe('wireless-debug.ps1 行为守护（#1564）—— stop 收挂在飞的 adb 调用（#1998 拆分件 4/4）', () => {
  // #1718：每条用例结束都按夹具目录把漏下的桩进程收干净。这不是「顺手打扫」——
  // 漏下任何一个，它都可能握着本进程（jest -i）继承出去的 stdout 管道，整条行为腿就永远等不到 EOF。
  afterEach(() => {
    for (const fx of FIXTURES.splice(0)) killStubProcesses(fx.dir);
  });

  it('B7 stop 收 keep 时连它正在跑的那次 adb 调用一起收掉（#1718：残留会吊住调用方管道）', () => {
    // 复现口径：让 `shell echo hb` 停在半空（真机上的「正跑到一半」），这时 stop 只杀 keep 的 pid
    // 就会留下 cmd.exe / node 两个桩进程 —— 实测它们握着本进程继承出去的 stdout 管道，jest 于是永不退出。
    const fx = makeFixture({ connected: [LIVE], hangEchoMs: 30000 });
    seedToolState(fx, LIVE); // 直接进连接态：省掉重发现，让 `shell echo hb` 尽快在飞
    const w = runTool(fx, ['-Action', 'watch', '-IntervalSeconds', '1', '-SlowBeatEvery', '10', '-MaxHours', '1']);
    const m = /watcher_pid=(\d+)/.exec(w.stdout);
    if (!m) throw new Error('watch 没回 watcher_pid ⇒ exit=' + w.status + ' out=' + JSON.stringify(w.stdout.slice(-600)));
    const t0 = Date.now();
    while (Date.now() - t0 < 20000) {
      if (callsParsed(fx).some((c) => c.includes('shell echo hb'))) break;
      sleep(200);
    }
    // 先证明「确实有一次 adb 调用在飞」——否则这条用例会退化成空跑（hang 没生效时它恒绿）
    expect(callsParsed(fx).some((c) => c.includes('shell echo hb'))).toBe(true);
    expect(stubProcessPids(fx.dir).length).toBeGreaterThan(0); // 在飞的那次调用此刻一定有一个桩进程

    const s = runTool(fx, ['-Action', 'stop']);
    expect(s.stdout).toContain('killed_pid=' + m[1]);
    const t1 = Date.now();
    let left = stubProcessPids(fx.dir);
    while (left.length && Date.now() - t1 < 5000) {
      sleep(100);
      left = stubProcessPids(fx.dir);
    }
    expect(left).toEqual([]); // keep 的整棵子树都收了，没有握着调用方管道的孤儿
  }, 120000);
});
