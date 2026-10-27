/**
 * 真机无线调试入口的**行为**守护（scripts/wireless-debug.ps1，#1564）—— #1998 拆分件 1/4
 *
 * 原单套件 utils/wirelessDebugBehavior.test.js（202.5s，run 40111032652）是 CI 行为腿四片的关键路径，
 * 按用例形态拆成四件，本件 = ensure 选路与开关（B6 fail-closed / B1 死记录在前 / B2 回落 mDNS / B4 开关写回）。拆分口径：
 * - 用例与断言**逐字沿用**原套件，只重组不重写；四件合计覆盖原 B1–B7 全部七条（不重叠不遗漏）。
 * - 夹具段逐字复制到每个拆分件——这是有意的：scripts/classify-guards.mjs 按文件自身的
 *   「代码级执行调用 + 仓内载体字面量」判行为腿，藏进共享夹具会被判成接线腿。
 *   全程在 ADR-0042 边界内：不动 -i、不动分片机制、不删改被测断言、不新建第三份被测面清单。
 * - 本件单列理由：B6/B1/B2 同属 ensure 的三条选路语义（fail-closed / 死记录在前 / 回落 mDNS），B4 开关写回同为「单发命令、无后台进程」形态且共享 no_channel / fail-closed 边界。
 * - 其余三件：wirelessDebugKeepRecoverBehavior.test.js（B3 keep 自愈）、wirelessDebugStopReapBehavior.test.js（B5 按 pid 收 stop）、
 *   wirelessDebugStopHangBehavior.test.js（B7 收挂在飞的 adb 调用）
 * - 拆分后估摊耗时与推导读 utils/jestSuiteTimings.json 的 note（CI run 40443976624 重采；StopReap/StopHang 两键按本机逐用例占比保和拆分）。
 *
 *   B6 一条候选都试不活 ⇒ ensure 判 unconnected、退出码非 0、对设备零写入（fail-closed 真断言）。
 *      防的是「连不上却报 connected」——门照着那个假 serial 去跑就是白跑一轮真机。
 *
 *   B1 候选表里**死记录排在前面** ⇒ 结论必须是那个活端口。
 *      防的是「信第一条 mDNS 记录」。现测（2026-10-07）：这台机器的表里活端口与一个恒 10061 的旧端口并列。
 *
 *   B2 显式 `-Serial` 与 state.json 存的都是**死端口** ⇒ 必须回落到 mDNS 的活端口。
 *      防的是「拿到已知端口就当结论、连不上就直接退出」。B1 管「选错候选」，B2 管「不回落」——两支不同的失败模式。
 *
 *   B4 开关读到 0 ⇒ `on` 写回 1；关掉写入时 ⇒ 一个字都不写。后半支防「无条件改设备」那类退化。
 *
 * 平台口径（2026-10-07 修订，**六条在 Windows 与 CI 的 ubuntu 上都真跑**）：假载体按平台落成
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

describe('wireless-debug.ps1 行为守护（#1564）—— ensure 选路与开关（#1998 拆分件 1/4）', () => {
  // #1718：每条用例结束都按夹具目录把漏下的桩进程收干净。这不是「顺手打扫」——
  // 漏下任何一个，它都可能握着本进程（jest -i）继承出去的 stdout 管道，整条行为腿就永远等不到 EOF。
  afterEach(() => {
    for (const fx of FIXTURES.splice(0)) killStubProcesses(fx.dir);
  });

  it('B6 无候选可试 ⇒ ensure 判 unconnected、退出码非 0、对设备零写入（fail-closed，两侧平台都跑）', () => {
    const fx = makeFixture({ records: [], connected: [] }); // mDNS 空表 + 没有已知端口
    const r = runTool(fx, ['-Action', 'ensure'], 60000);
    expect(r.status).not.toBe(0); // 现测（2026-10-07 Windows）exit=3
    expect(r.stdout).toMatch('result=unconnected');
    expect(r.stdout).not.toMatch(/result=connected|result=paired_connected/);
    const seq = callsParsed(fx);
    expect(seq.some((c) => c.startsWith('connect '))).toBe(false); // 没候选就不许盲试端口
    expect(seq.join('|')).not.toMatch('kill-server');
    expect(adbState(fx).toggleWrites || []).toEqual([]); // 失败路径一个字节都不写设备
    expect(toolState(fx).serial).toBeNull(); // 不把失败当成功落盘
  }, 60000);

  it('B1 候选表死记录在前 ⇒ 结论必须是活端口（不得信第一条 mDNS 记录）', () => {
    const fx = makeFixture({ records: [{ ep: DEAD, alive: false }, { ep: LIVE, alive: true }] });
    const r = runTool(fx, ['-Action', 'ensure']);
    const v = verdict(r.stdout, 'ensure').find((x) => x.result === 'connected');
    expect(v).toBeDefined();
    expect(v.serial).toBe(LIVE);
    const seq = callsParsed(fx);
    expect(seq.join('|')).toMatch('mdns services'); // 候选表来源确实是 mDNS
    const connects = seq.filter((c) => c.startsWith('connect '));
    expect(connects[0]).toContain(DEAD); // 先试了排在前面那条
    expect(connects.some((c) => c.includes(LIVE))).toBe(true);
    expect(seq.join('|')).not.toMatch('kill-server');
  }, 60000);

  it('B2 -Serial 与 state.json 都是死端口 ⇒ 必须回落到 mDNS 的活端口', () => {
    const fx = makeFixture({ records: [{ ep: LIVE, alive: true }] });
    seedToolState(fx, GONE); // 上次成功、如今已死的端口
    const r = runTool(fx, ['-Action', 'ensure', '-Serial', DEAD]);
    const v = verdict(r.stdout, 'ensure').find((x) => x.result === 'connected');
    expect(v).toBeDefined();
    expect(v.serial).toBe(LIVE);
    expect(r.stdout).not.toContain('result=connected serial=' + DEAD); // 死端口不得当结论
    expect(toolState(fx).serial).toBe(LIVE); // 回落结果要落盘，下次冷启动别再试死端口
  }, 60000);

  it('B4 开关读到 0 ⇒ on 写回 1；关掉写入 ⇒ 设备一个字节都不改', () => {
    // 前提：`on` 只在**已有 adb 通道**时才谈得上写开关——没有通道时它必须判 no_channel（那是产品的边界，
    // 不是缺陷：无通道意味着手机侧重置过，任何脚本都开不了那个开关）。所以这里先接上一条 live transport。
    const a = makeFixture({ toggle: '0', connected: [LIVE] });
    const r = runTool(a, ['-Action', 'on']);
    expect(verdict(r.stdout, 'on').map((x) => x.result)).toContain('written');
    expect(adbState(a).toggleWrites || []).toEqual(['1']); // 只写过「开」方向，且只写这一次

    const noChannel = makeFixture({ toggle: '0', connected: [] });
    const r0 = runTool(noChannel, ['-Action', 'on']);
    expect(verdict(r0.stdout, 'on').map((x) => x.result)).toContain('no_channel');
    expect(adbState(noChannel).toggleWrites || []).toEqual([]); // 没通道就绝不能留下半次写入

    const b = makeFixture({ toggle: '1', connected: [LIVE] });
    const r2 = runTool(b, ['-Action', 'on', '-ReassertToggle:$false']);
    expect(verdict(r2.stdout, 'on').map((x) => x.result)).toContain('readonly');
    expect(adbState(b).toggleWrites || []).toEqual([]);
  }, 60000);
});
