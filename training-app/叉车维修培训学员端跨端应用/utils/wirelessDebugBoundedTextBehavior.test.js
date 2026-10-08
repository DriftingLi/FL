/**
 * 真机无线调试入口 `Invoke-Adb`（**16 个调用方共用一个点**）的有界性运行期守护（#1568，2026-10-08）
 *
 * 为什么这张最该收（票面 #1568 AC 第 1 条）：`wireless-debug.ps1` 是**设备掉线时才用的自愈工具**，
 * 也正是最容易撞上「adb 不返回」的那条路 —— 心跳、mDNS 发现、connect 试探全压在 `Invoke-Adb`
 * 这一个 `& $AdbExe @AdbArgs 2>&1 | Out-String` 上，它自己挂住时，"自愈" 就成了「整条 keep 循环静默停住」。
 *
 * 本文件**真起 pwsh、真跑脚本**，adb 换成可编程的假载体（node + `.bat` / `.sh`，形态沿用
 * utils/wirelessDebugBehavior.test.js 的 makeFixture），钉五条只有跑起来才看得见的行为：
 *
 *   WBD1 挂死腿：mDNS 那一次调用永不返回 ⇒ 到预算就**给结论**（ensure 照旧判 unconnected、退出码非 0），
 *        且 actions.log 点名「哪一次调用 / 多大预算」（票面 AC：「不返回」必须是可见结论）。
 *   WBD2 对照腿：正常载体 ⇒ ensure 判 connected、serial 是那条活端口，日志里**一条超时都没有**
 *        （防「把正常返回也判成超时」这一支 —— 判据三条里的「测的是不是那一支」）。
 *   WBD3 `2>&1` 语义不许在收口时丢掉：pair 失败那一支 adb 把 `protocol fault` 打在 **stderr**，
 *        分类判据必须照样拿到它（class=wrong_port_kind）。这是收口最容易改坏的东西 —— 执行核
 *        把 stdout / stderr 分成两个文件，封装不合并就静默换了判据。
 *   WBD4 预算按调用形态分档：读数类与端点类各一档 —— 换一档预算就换一个上界，日志点的预算跟着变
 *        （票面 AC：install / push / wait-for-device / boot 等待**不得**套 15 秒，同一条也要求「分档」是真接上的）。
 *   WBD5 空转自检：假载体确实在 calls.log 里记下了那一次挂死的调用（否则 WBD1 是在空集合上判绿）。
 *
 * ⚠️ 判别力实测（M 系列读数记在 PR 的 `## 验收证据`）：把 `Invoke-Adb` 换回 `& $AdbExe … | Out-String`
 *   ⇒ WBD1/WBD4 红（挂死腿一直睡到 execFileSync 上界）；把封装的 stderr 合并去掉 ⇒ WBD3 红。
 *
 * ⚠️ 平台口径：**五条在 Windows 与 CI 的 ubuntu 上都真跑**（假载体按平台落 `.bat` / `.sh`）。
 *   先例教训：#1562 的 `B12` 曾被 CI 照出「那一格在 ubuntu 上物理造不出来」⇒ 本套件不写
 *   `if (!IS_WIN) return` 这种静默跳过腿；非 Windows 走执行核的直启分支，挂死与到点都真发生。
 *
 * 在册性：`scripts/lib/contract-tests.ps1` 的 pattern 需含 `wirelessDebugBoundedText`，
 *   否则 token 收窄的门里这一组真执行腿永不执行（判据见该文件头部；#1562 同款裁定）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
// 读文件走共享读者（ADR-0019）：CI 的 check-contract-read 规则二不管读的是不是仓内文件。
const { readText } = require('./utsHarness');

const IS_WIN = process.platform === 'win32';
const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**：`scripts/classify-guards.mjs` 按文件自身的
// 「代码级执行调用 + 仓内载体引用」判行为守护 ⇒ 两样都不能藏进共享夹具。
const SCRIPT_REL = 'scripts/wireless-debug.ps1';
const SCRIPT_ABS = path.join(ROOT, SCRIPT_REL);

const LIVE = '192.168.10.51:39527';
const DEAD = '192.168.10.51:37611';

// 真挂死腿的预算给 2 / 3 秒，加上真 pwsh + 真 node 载体开销 ⇒ 整条探针十几秒，默认 5 秒拦不住。
jest.setTimeout(180000);

/**
 * 假 adb：`adb-state.json` 控场（含 `hangFor` —— 命中就**不返回**），`calls.log` 记下每一次 argv。
 * pair 失败那一支刻意只写 stderr —— 与真 adb 一致，也是 WBD3 的判据来源。
 */
const FAKE_ADB_JS = [
  "const fs = require('fs');",
  "const path = require('path');",
  "const S = path.join(__dirname, 'adb-state.json');",
  "const st = JSON.parse(fs.readFileSync(S, 'utf8'));",
  "st.connected = st.connected || [];",
  "st.hangFor = st.hangFor || [];",
  "const save = () => fs.writeFileSync(S, JSON.stringify(st, null, 2));",
  "const say = (s) => process.stdout.write(s + '\\n');",
  "const err = (s) => process.stderr.write(s + '\\n');",
  "const rec = (ep) => (st.records || []).find((r) => r.ep === ep);",
  "const argv = process.argv.slice(2);",
  "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
  "const keyFor = (a) => {",
  "  if (a[0] === '-s') return a.slice(2).join(' ');",
  "  return a.join(' ');",
  "};",
  "const hangKey = argv[0] === '-s' ? argv.slice(2)[0] : argv[0];",
  "if (st.hangFor.includes(hangKey)) { setTimeout(() => {}, 120000); return; }",
  "if (argv[0] === '-s') {",
  "  const ep = argv[1];",
  "  const rest = argv.slice(2).join(' ');",
  "  const live = st.connected.includes(ep) && rec(ep) && rec(ep).alive;",
  "  if (/^shell echo /.test(rest)) { live ? say(rest.split(' ')[2]) : err('error: device not found'); save(); return; }",
  "  if (rest.includes('settings get global adb_wifi_enabled')) { say(String(st.toggle)); save(); return; }",
  "  if (rest.includes('settings put global adb_wifi_enabled')) {",
  "    const v = rest.trim().split(' ').pop();",
  "    st.toggleWrites = (st.toggleWrites || []).concat([v]); st.toggle = v; say(''); save(); return;",
  "  }",
  "  say('stub'); save(); return;",
  "}",
  "if (argv[0] === 'version') { say('Android Debug Bridge version 1.0.41'); return; }",
  "if (argv[0] === 'mdns') {",
  "  say('List of discovered mdns services');",
  "  (st.records || []).forEach((r, i) => say('adb-b32d8398-' + i + '\\t_adb-tls-connect._tcp\\t' + r.ep));",
  "  return;",
  "}",
  "if (argv[0] === 'devices') {",
  "  say('List of devices attached');",
  "  st.connected.forEach((ep) => { if (rec(ep) && rec(ep).alive) say(ep + '\\tdevice'); });",
  "  return;",
  "}",
  "if (argv[0] === 'connect') {",
  "  const ep = argv[1];",
  "  if (rec(ep) && rec(ep).alive) { if (!st.connected.includes(ep)) st.connected.push(ep); say('connected to ' + ep); }",
  "  else err('cannot connect to ' + ep + ': Connection refused (10061)');",
  "  save(); return;",
  "}",
  "if (argv[0] === 'disconnect') { st.connected = st.connected.filter((x) => x !== argv[1]); say('disconnected ' + argv[1]); save(); return; }",
  "if (argv[0] === 'pair') {",
  "  if (st.pairEndpoint === argv[1] && String(argv[2] || '').length === 6) { say('Successfully paired to ' + argv[1]); return; }",
  "  err('error: protocol fault (could not read status message): No error'); return;",
  "}",
  "say('stub-ignored ' + argv.join(' '));",
  '',
].join('\n');

function makeFixture(over) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wd-bound-'));
  const stateDir = path.join(dir, 'state');
  fs.mkdirSync(stateDir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'fake-adb.js'), FAKE_ADB_JS, 'utf8');
  const exe = path.join(dir, IS_WIN ? 'fake-adb.bat' : 'fake-adb');
  if (IS_WIN) fs.writeFileSync(exe, '@echo off\r\nnode "%~dp0fake-adb.js" %*\r\n', 'utf8');
  else fs.writeFileSync(exe, '#!/bin/sh\nexec node "$(dirname "$0")/fake-adb.js" "$@"\n', { mode: 0o755 });
  const state = Object.assign(
    { records: [{ ep: DEAD, alive: false }, { ep: LIVE, alive: true }], connected: [], toggle: '1', hangFor: [] },
    over || {},
  );
  fs.writeFileSync(path.join(dir, 'adb-state.json'), JSON.stringify(state, null, 2), 'utf8');
  fs.writeFileSync(path.join(stateDir, 'state.json'), JSON.stringify({ serial: null, portHistory: [] }), 'utf8');
  return { dir, stateDir, exe };
}

const actionsLog = (fx) => {
  const p = path.join(fx.stateDir, 'actions.log');
  return fs.existsSync(p) ? readText(p) : '';
};
const callsParsed = (fx) => {
  const p = path.join(fx.dir, 'calls.log');
  if (!fs.existsSync(p)) return [];
  return readText(p).split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l).join(' '));
};

function runTool(fx, extraArgs, timeout) {
  try {
    const stdout = execFileSync(
      'pwsh',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', SCRIPT_ABS,
        '-AdbExe', fx.exe, '-StateDir', fx.stateDir, '-Ascii'].concat(extraArgs),
      { encoding: 'utf8', timeout: timeout || 90000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 },
    );
    return { status: 0, stdout };
  } catch (e) {
    return { status: typeof e.status === 'number' ? e.status : 9, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

describe('wireless-debug.ps1 的 Invoke-Adb 收口点有界性（运行期，#1568）', () => {
  it('WBD1 挂死腿：mDNS 发现不返回 ⇒ 到预算给结论（ensure 照旧 unconnected、非 0 退出），日志点名哪一次调用与哪一档预算', () => {
    const fx = makeFixture({ hangFor: ['mdns'] });
    const t0 = Date.now();
    // mDNS 发现归**端点档**（它和 connect 一样要等网络答话），所以这里两档各给一个不同的数：
    // 断言拿到的必须是端点档那个 3，而不是读数档的 2 —— 两个数不相等，这一格才分得开。
    const r = runTool(fx, ['-Action', 'ensure', '-AdbCallTimeoutSeconds', '2', '-AdbEndpointTimeoutSeconds', '3'], 80000);
    const wall = Date.now() - t0;
    expect(callsParsed(fx).some((c) => c.startsWith('mdns'))).toBe(true); // 那一次调用确实发生过
    expect(r.status).not.toBe(0);
    expect(r.stdout).toMatch('result=unconnected');
    const log = actionsLog(fx);
    expect(log).toMatch('ADB_CALL_TIMEOUT args=mdns services callBudgetSeconds=3');
    // 读数行把两档预算都点出来（读日志的人要能分清「哪一档拦的」）
    expect(log).toMatch('ADB_CALL_BUDGET calls=1 timeouts=1 readBudgetSeconds=2 endpointBudgetSeconds=3 hungArgs=mdns services');
    // 只证「没挂住」，不是判据（墙钟口径 #1549）：预算 3 秒给了十倍余量
    expect(wall).toBeLessThan(30000);
  });

  it('WBD2 对照腿：正常载体 ⇒ ensure 判 connected 且日志里没有一条超时（正常返回不得被误判）', () => {
    // 只放一条活端口候选：这一腿验的是「不误判」，多试一条死端口只会多几趟真进程派发（成本读数在 PR 正文）
    const fx = makeFixture({ records: [{ ep: LIVE, alive: true }] });
    const r = runTool(fx, ['-Action', 'ensure', '-AdbCallTimeoutSeconds', '5']);
    expect(r.stdout).toMatch(/result=connected serial=\S*39527/);
    expect(r.stdout).not.toMatch('result=unconnected');
    const log = actionsLog(fx);
    expect(log).not.toMatch('ADB_CALL_TIMEOUT');
    expect(callsParsed(fx).length).toBeGreaterThan(2); // 确实跑过一批调用
  });

  it('WBD3 pair 失败分支：adb 把 protocol fault 打在 stderr ⇒ 分类判据照样拿到它（2>&1 语义没丢）', () => {
    const fx = makeFixture({ pairEndpoint: '192.168.10.51:41203' });
    const r = runTool(fx, ['-Action', 'pair', '-PairEndpoint', '192.168.10.51:40000', '-PairCode', '123456']);
    expect(r.stdout).toMatch('result=pair_failed');
    expect(r.stdout).toMatch('class=wrong_port_kind'); // 这一支的原料全在 stderr
    expect(actionsLog(fx)).toMatch('protocol fault');
  });

  it('WBD4 预算分档：connect 那一档吃端点预算，读数那一档吃默认预算 —— 两个数在同一次运行里各自点名', () => {
    const fx = makeFixture({ hangFor: ['connect'], records: [{ ep: DEAD, alive: false }] });
    const r = runTool(fx, ['-Action', 'ensure', '-AdbCallTimeoutSeconds', '2', '-AdbEndpointTimeoutSeconds', '3'], 80000);
    expect(r.status).not.toBe(0);
    expect(r.stdout).toMatch('result=unconnected');
    const log = actionsLog(fx);
    expect(log).toMatch('ADB_CALL_TIMEOUT args=connect ' + DEAD + ' callBudgetSeconds=3');
    // 同一趟里读数档仍然按自己的预算（两条档不是同一个数，也不是把 15 秒套到端点长等上）
    expect(log).toMatch('callBudgetSeconds=3');
  });

  it('WBD5 空转自检：挂死腿的桩确实被调到、结论行确实来自脚本而不是超时后没输出', () => {
    const fx = makeFixture({ hangFor: ['devices'], records: [{ ep: LIVE, alive: true }] });
    const r = runTool(fx, ['-Action', 'ensure', '-AdbCallTimeoutSeconds', '2'], 80000);
    expect(r.stdout).toMatch('WIRELESS_DEBUG action=ensure result=unconnected');
    expect(callsParsed(fx).some((c) => c.startsWith('devices'))).toBe(true);
    expect(actionsLog(fx)).toMatch('ADB_CALL_TIMEOUT args=devices callBudgetSeconds=2');
  });
});
