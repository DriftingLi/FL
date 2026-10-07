/**
 * 真机无线调试入口的**行为**守护（scripts/wireless-debug.ps1，#1564）
 *
 * 为什么不能只有静态契约（utils/wirelessDebugContract.test.js）：源码文本断言在
 * 「算法被改坏但字面量还在」时**不会红**——本仓已经实测过「全绿可以恒真」这件事。
 * 本文件真起 pwsh、真跑脚本，把 adb 换成一份**可编程的假载体**（node + .bat，
 * 形态沿用 utils/mpWeixinGateHarness.js 的 makeFakeDevTools），钉六条只有跑起来才看得见的行为：
 *
 *   B1 候选表里**死记录排在前面** ⇒ 结论必须是那个活端口。
 *      防的是「信第一条 mDNS 记录」。现测（2026-10-07）：这台机器的表里活端口与一个恒 10061 的旧端口并列。
 *   B2 显式 `-Serial` 与 state.json 存的都是**死端口** ⇒ 必须回落到 mDNS 的活端口。
 *      防的是「拿到已知端口就当结论、连不上就直接退出」。B1 管「选错候选」，B2 管「不回落」——两支不同的失败模式。
 *   B3 设备从 `adb devices` 里消失 ⇒ keep 自己经 mDNS 重发现并接回（LOST → SAMEPORT/NEWPORT → RECOVERED），
 *      且全程不发 kill-server（adb server 与 HBuilderX 共享，行为层再钉一次）。
 *   B4 开关读到 0 ⇒ `on` 写回 1；关掉写入时 ⇒ 一个字都不写。后半支防「无条件改设备」那类退化。
 *   B5 `watch` 起了后台 keep ⇒ `stop` 必须**按同一个 pid** 收掉、收完进程真没了。
 *      血账：入仓改名后归属判据里写死的旧文件名不再命中 ⇒ watch 恒报 spawn_unconfirmed、
 *      stop 恒报 not_running，而 keep 进程还在后台跑（留孤儿）。这条就是那起事故的回归钉。
 *      同一条腿还顺带钉住「-AdbExe 必须传给子进程」：漏传时子进程根本不碰指定载体，calls.log 会是空的。
 *
 *   B6 一条候选都试不活 ⇒ ensure 判 unconnected、退出码非 0、对设备零写入（fail-closed 真断言）。
 *      防的是「连不上却报 connected」——门照着那个假 serial 去跑就是白跑一轮真机。
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

const IS_WIN = process.platform === 'win32';
const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**：`scripts/classify-guards.mjs` 按文件自身的
// 「代码级执行调用 + 仓内载体引用」判行为守护 ⇒ 两样都不能藏进共享夹具（藏了就被判成接线守护）。
const SCRIPT_REL = 'scripts/wireless-debug.ps1';
const SCRIPT_ABS = path.join(ROOT, SCRIPT_REL);

const LIVE = '192.168.10.51:39527';
const DEAD = '192.168.10.51:37611';
const GONE = '192.168.10.51:41999'; // 记录表里根本没有、只出现在 state.json / -Serial 里的死端口

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
  return { dir, stateDir, bat };
}

const adbState = (fx) => JSON.parse(fs.readFileSync(path.join(fx.dir, 'adb-state.json'), 'utf8'));
const callsParsed = (fx) => {
  const p = path.join(fx.dir, 'calls.log');
  if (!fs.existsSync(p)) return [];
  return fs.readFileSync(p, 'utf8').split(/\r?\n/).filter((l) => l.trim()).map((l) => JSON.parse(l).join(' '));
};
const toolState = (fx) => {
  const p = path.join(fx.stateDir, 'state.json');
  return fs.existsSync(p) ? JSON.parse(fs.readFileSync(p, 'utf8')) : null;
};
const keepLog = (fx) => {
  const p = path.join(fx.stateDir, 'keep.log');
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
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
    return { status: typeof e.status === 'number' ? e.status : 9, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}
const verdict = (out, action) => {
  const re = new RegExp('WIRELESS_DEBUG action=' + action + ' result=(\\S+) serial=(\\S+)', 'g');
  return [...out.matchAll(re)].map((m) => ({ result: m[1], serial: m[2] }));
};

describe('wireless-debug.ps1 行为守护（#1564）', () => {
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

  it('B3 设备从表里消失 ⇒ keep 经 mDNS 自己接回（LOST→SAMEPORT/NEWPORT→RECOVERED），全程不 kill-server', () => {
    const fx = makeFixture({ emptyDevicesFirst: 3 }); // 前 3 次 devices 报空 = 链路掉了
    seedToolState(fx, LIVE);
    const child = spawn('pwsh', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', SCRIPT_ABS,
      '-AdbExe', fx.bat, '-StateDir', fx.stateDir, '-Action', 'keep',
      '-IntervalSeconds', '1', '-SlowBeatEvery', '2', '-ProbeSeconds', '1',
      '-WaitMinutes', '1', '-MaxHours', '1'], { windowsHide: true });
    let log = '';
    const t0 = Date.now();
    try {
      while (Date.now() - t0 < 45000) {
        log = keepLog(fx);
        if (/RECOVERED/.test(log)) break;
        sleep(500);
      }
    } finally {
      try { child.kill(); } catch (e) { /* 已自行收口 */ }
    }
    expect(log).toMatch('LOST');
    expect(log).toMatch(/SAMEPORT|NEWPORT/);
    expect(log).toMatch('RECOVERED');
    const seq = callsParsed(fx).join('|');
    expect(seq).toMatch('mdns services'); // 重发现走 mDNS，不是原地重试同端口
    expect(seq).not.toMatch('kill-server');
  }, 90000);

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

  it('B5 watch 起的 keep ⇒ stop 按同一 pid 收掉、进程真没了（且子进程用的是指定 adb）', () => {
    const fx = makeFixture();
    const w = runTool(fx, ['-Action', 'watch', '-IntervalSeconds', '1', '-SlowBeatEvery', '2', '-MaxHours', '1']);
    const m = /watcher_pid=(\d+)/.exec(w.stdout);
    expect(m).toBeTruthy(); // 归属判据写死旧名时这里恒报 spawn_unconfirmed
    const pid = Number(m[1]);
    const w2 = runTool(fx, ['-Action', 'watch', '-IntervalSeconds', '1']);
    expect(w2.stdout).toMatch('already_running'); // 不叠第二个循环
    sleep(2500); // 等子进程至少跑出一次 adb 调用
    const s = runTool(fx, ['-Action', 'stop']);
    expect(s.stdout).toContain('killed_pid=' + pid);
    expect(pidAlive(pid)).toBe(false); // 真消失，而不是「报了个 not_running」
    expect(fs.existsSync(path.join(fx.stateDir, 'watch.pid'))).toBe(false);
    // 漏传 -AdbExe 的话，calls.log 里只会有父进程那次 watch 的 version 检查，keep 全在打真 adb
    const seq = callsParsed(fx);
    expect(seq.some((c) => c.includes('echo hb') || c.startsWith('devices') || c.startsWith('connect '))).toBe(true);
  }, 90000);
});
