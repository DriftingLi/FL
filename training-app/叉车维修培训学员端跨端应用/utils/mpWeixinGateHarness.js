/**
 * ② 门**行为守护**的共享夹具（`utils/mpWeixinGate*Behavior.test.js` 共用）。
 *
 * 为什么抽出来：行为守护要「真跑门」，而真跑门需要三样东西 —— ① 一份**合法**的 mp-weixin 产物夹具、
 * ② 一个**假开发者工具**（记下每次被调用的 argv，永不监听端口）、③ 空闲/被占端口。两份测试各抄一遍
 * 会漂移（本仓的天条：判据/夹具只留一处真源，复制品会悄悄分叉）⇒ 收在这里。
 *
 * ⚠️ **执行调用（`execFileSync`）刻意留在各测试文件里，不搬进本夹具**：
 * ③ 门判据的可机检真源是 `scripts/classify-guards.mjs` —— 它按**文件自身**有没有代码级的执行调用
 * 判「行为守护 / 接线守护」。把执行搬进共享夹具 ⇒ 测试文件变成「零载体引用的接线守护」，
 * `utils/guardClassification.test.js` 的 H4 当场转红（本轮实测踩到过）。本夹具只提供
 * 「夹具 + 参数拼装（`gateArgs`）+ 记录读取」，**谁跑子进程谁自己 `execFileSync`**。
 *
 * 平台边界：门的运行时路径依赖 Windows 专有的 `Get-NetTCPConnection` 与 `cli.bat` ⇒ 夹具在 Windows 上
 * 造 `.bat`，在 POSIX 上造带 shebang 的可执行文件；**非 Windows 分支由各测试自己真断言**（不静默跳过）。
 */
const fs = require('fs');
const os = require('os');
const net = require('net');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const APPID = 'wx38c3e31b16a7ced0';
const IS_WIN = process.platform === 'win32';

function writeFile(p, text, mode) {
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, text, 'utf8');
  if (mode) fs.chmodSync(p, mode);
}

/** 造一份**合法**的 mp-weixin 产物夹具（判据面：`scripts/lib/mp-weixin-product.ps1`）。 */
function makeFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-gate-'));
  writeFile(path.join(dir, 'manifest.json'), JSON.stringify({ name: 'fixture', 'mp-weixin': { appid: APPID } }));
  // 门只做存在性校验；行为守护在端口面就退出，探针不会被执行
  writeFile(path.join(dir, 'scripts', 'mp-weixin-probe.mjs'), '// fixture\n');
  const dist = path.join(dir, 'unpackage', 'dist', 'build', 'mp-weixin');
  writeFile(path.join(dist, 'project.config.json'), JSON.stringify({ appid: APPID, projectname: 'fixture' }));
  writeFile(path.join(dist, 'app.json'), JSON.stringify({ pages: ['pages/index/index'] }));
  writeFile(path.join(dist, 'app.js'), '// fixture\n');
  writeFile(path.join(dist, 'app.wxss'), '/* fixture */\n');
  writeFile(path.join(dist, 'pages', 'index', 'index.js'), '// fixture\n');
  return { dir, dist };
}

/**
 * 假开发者工具：把每次调用的 argv 以**一行一个 JSON 数组**追加到 `calls.log`，回显成功文案，
 * **永不**监听端口（= 冷起点「auto 回显 √ 但端口不起」的形态）。
 * @returns 可执行入口（Windows = `.bat`，POSIX = 带 shebang 的可执行文件）
 */
function makeFakeDevTools(dir) {
  const js = [
    "const fs = require('fs');",
    "const path = require('path');",
    'const argv = process.argv.slice(2);',
    "const verb = argv[0] || '';",
    "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
    "if (verb === 'auto') {",
    `  process.stdout.write('- Fetching AppID () permissions\\n\\u221a Using AppID: ${APPID}\\n\\u221a auto\\n');`,
    '} else {',
    "  process.stdout.write('\\u221a ' + verb + '\\n');",
    '}',
    ''
  ].join('\n');
  writeFile(path.join(dir, 'fake-devtools.js'), js);
  if (IS_WIN) {
    const bat = path.join(dir, 'fake-devtools.bat');
    writeFile(bat, '@echo off\r\nnode "%~dp0fake-devtools.js" %*\r\n');
    return bat;
  }
  const sh = path.join(dir, 'fake-devtools');
  writeFile(sh, '#!/bin/sh\nexec node "$(dirname "$0")/fake-devtools.js" "$@"\n', 0o755);
  return sh;
}

/** 取一个当前空闲的端口（避免请求的端口恰好被占，把门引到另一条路径上）。 */
function freePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const p = srv.address().port;
      srv.close(() => resolve(p));
    });
  });
}

/** 占住一个端口（模拟「残留自动化会话」），返回 { port, close }。 */
function occupyPort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      resolve({ port: srv.address().port, close: () => new Promise((r) => srv.close(r)) });
    });
  });
}

/**
 * 拼门脚本的调用参数（`-SkipBuild`，不接 HBuilderX、不构建）。超时预算压到秒级：
 * 行为守护只关心调用序列与结果行。**只拼参数、不起进程**（见文件头那条：执行留在测试文件里）。
 */
function gateArgs(scriptPath, fixture, fakeCli, port, extraArgs) {
  return [
    '-NoProfile', '-NonInteractive', '-File', scriptPath,
    '-Project', fixture.dir, '-SkipBuild', '-DevToolsCli', fakeCli,
    '-Port', String(port), '-PortWaitSeconds', '1', '-OpenTimeoutSeconds', '20',
    '-AutoTimeoutSeconds', '20', '-AutoRetryDelaySeconds', '0', '-HxNoWait'
  ].concat(extraArgs || []);
}

/** 假开发者工具收到的调用（按时间顺序，每项是该次调用的 argv 数组）。 */
function callsOf(fakeCli) {
  const p = path.join(path.dirname(fakeCli), 'calls.log');
  if (!fs.existsSync(p)) return [];
  return fs.readFileSync(p, 'utf8')
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean)
    .map((l) => JSON.parse(l));
}

/** 调用序列的动词（`close` / `open` / `auto` …）。 */
function verbsOf(fakeCli) {
  return callsOf(fakeCli).map((a) => a[0]);
}

/** 取某动词最后一次收到的 `--flag` 取值。 */
function lastFlagValue(fakeCli, verb, flag) {
  const calls = callsOf(fakeCli).filter((a) => a[0] === verb);
  if (calls.length === 0) return undefined;
  const last = calls[calls.length - 1];
  const i = last.indexOf(flag);
  return i === -1 ? undefined : last[i + 1];
}

/** 清掉调用记录（同一个夹具连跑多轮时用）。 */
function resetCalls(fakeCli) {
  fs.rmSync(path.join(path.dirname(fakeCli), 'calls.log'), { force: true });
}

/**
 * 读门自己写的日志（`<项目>/.ci-verify/mp-weixin.log`，UTF-8）。
 * 结论行（`MP_WEIXIN_RESULT`）与门自己打印的说明行只进日志、不进 stdout；且 stdout 在中文 Windows 上
 * 受子进程控制台代码页影响会乱码 ⇒ **判据一律取日志文件**，不对 stdout 断言中文。
 */
function gateLog(fixture) {
  const p = path.join(fixture.dir, '.ci-verify', 'mp-weixin.log');
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
}

module.exports = {
  ROOT, APPID, IS_WIN,
  makeFixture, makeFakeDevTools, freePort, occupyPort,
  gateArgs, callsOf, verbsOf, lastFlagValue, resetCalls, gateLog
};
