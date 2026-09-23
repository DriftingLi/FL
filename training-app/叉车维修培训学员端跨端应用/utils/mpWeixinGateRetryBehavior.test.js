/**
 * ② 门「整段重试」行为守护（2026-09-23 实测定位，坑位 6）。
 *
 * 病根（本机实测，两次独立复现）：冷起点 / IDE 重启窗口下 **第一次** `cli.bat auto` 会
 * 「回显 `√ auto` 但自动化端口在预算内始终不监听」或「直接挂住 >180s」；**同一条命令**等 IDE 稳定后
 * 重跑 1–8s 即起。故执行路径把 `close → open → auto → 端口` 当**一个可重试的单元**跑 attempts 次
 * （`scripts/mp-weixin-check.ps1`，计划面判据 C21）。
 *
 * 为什么守护要**真跑门**而不是断言源码文本：文本断言在「循环被摘掉、只留一句字面量」时照样绿。
 * 这里用**假开发者工具**（`node` 冒充 `cli.bat`：只记账 + 回显，**永不**监听端口 = 冷起点第一枪的形态）
 * 把门真跑一遍，判据是**调用序列**与**结果行**：
 *   - 必不红（attempts=2）：`close → open → auto` 必须**整段**出现两遍（不是只重试 auto），
 *     且两次都用尽才 `exit 2` / `reason=port-not-listening`；
 *   - 必红（attempts=1）：只允许出现一遍 —— 这条同时证明「次数真的来自 `-AutoAttempts`」，
 *     而不是写死的两遍（否则本条会红）。
 *
 * **平台边界写实**（与 `utils/autoScreenshotContract.test.js` 的 B 套件同款口径）：
 * 门的运行时路径依赖 Windows 专有的 `Get-NetTCPConnection` 与 `cli.bat` ⇒ Linux CI（mobile-test 跑在
 * ubuntu-latest）上验的是**另一支**：拿不到证据的环境里门必须 **fail-closed**（非 0 退出、且**不得**打出
 * 通过形态的结果行），**不是**静默跳过。
 */
const fs = require('fs');
const os = require('os');
const net = require('net');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const APPID = 'wx38c3e31b16a7ced0';
const IS_WIN = process.platform === 'win32';

function writeFile(p, text, mode) {
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, text, 'utf8');
  if (mode) fs.chmodSync(p, mode);
}

/** 造一份**合法**的 mp-weixin 产物夹具（判据面：`scripts/lib/mp-weixin-product.ps1`）。 */
function makeFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-retry-'));
  writeFile(path.join(dir, 'manifest.json'), JSON.stringify({ name: 'fixture', 'mp-weixin': { appid: APPID } }));
  // 门只做存在性校验；本用例在端口面就退出，探针不会被执行
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
 * 假开发者工具：把每个子命令追加到 `calls.log`，回显成功文案，**永不**监听端口。
 * @returns 可执行入口（Windows = `.bat`，POSIX = 带 shebang 的可执行文件）
 */
function makeFakeDevTools(dir) {
  const js = [
    "const fs = require('fs');",
    "const path = require('path');",
    "const verb = process.argv[2] || '';",
    "fs.appendFileSync(path.join(__dirname, 'calls.log'), verb + '\\n');",
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

/** 取一个当前空闲的端口（避免夹具端口被占用 ⇒ 端口轮询立刻为真、走到另一条路径去）。 */
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

function runGate(fixture, fakeCli, port, extraArgs) {
  const args = [
    '-NoProfile', '-NonInteractive', '-File', path.join(ROOT, SCRIPT_REL),
    '-Project', fixture.dir, '-SkipBuild', '-DevToolsCli', fakeCli,
    '-Port', String(port), '-PortWaitSeconds', '1', '-OpenTimeoutSeconds', '20',
    '-AutoTimeoutSeconds', '20', '-AutoRetryDelaySeconds', '0', '-HxNoWait'
  ].concat(extraArgs || []);
  try {
    const stdout = execFileSync('pwsh', args, {
      encoding: 'utf8', timeout: 300000, windowsHide: true, maxBuffer: 16 * 1024 * 1024
    });
    return { status: 0, stdout };
  } catch (e) {
    return { status: e.status, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

function callsOf(fakeCli) {
  const p = path.join(path.dirname(fakeCli), 'calls.log');
  if (!fs.existsSync(p)) return [];
  return fs.readFileSync(p, 'utf8').split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
}

/**
 * 读门自己写的日志（`<项目>/.ci-verify/mp-weixin.log`，UTF-8）。
 * 结论行（`MP_WEIXIN_RESULT`）与 `整段重试` 行只进日志、不进 stdout，且 stdout 在中文 Windows 上
 * 受子进程控制台代码页影响会出现乱码 ⇒ **判据一律取日志文件**，不对 stdout 断言中文。
 */
function gateLog(fixture) {
  const p = path.join(fixture.dir, '.ci-verify', 'mp-weixin.log');
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
}

describe('② 整段重试行为（坑位 6：auto 回显 √ 但端口不起 ⇒ close → open → auto 整段再来）', () => {
  let fixture = null;
  let fakeCli = '';
  let port = 0;

  beforeAll(async () => {
    fixture = makeFixture();
    fakeCli = makeFakeDevTools(fixture.dir);
    port = await freePort();
  }, 60000);

  afterAll(() => {
    if (fixture) fs.rmSync(fixture.dir, { recursive: true, force: true });
  });

  it('非 Windows：门在拿不到证据的环境里 fail-closed（不得产出绿）', () => {
    if (IS_WIN) return; // Windows 上由下面两条真跑（本条只钉平台边界那一支）
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '1']);
    expect(r.status).not.toBe(0);
    expect(r.stdout).not.toMatch(/MP_WEIXIN_RESULT errors=0/);
  });

  it('必不红：attempts=2 ⇒ close → open → auto 整段出现两遍，用尽才 exit 2', () => {
    if (!IS_WIN) return; // 平台边界由上一条真断言
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '2']);
    const calls = callsOf(fakeCli);
    // **整段**：不是 close/open/auto 各来两次的任意组合，而是两遍完整的 close → open → auto
    expect(calls).toEqual(['close', 'open', 'auto', 'close', 'open', 'auto']);
    // 全部用尽才判环境不可用（不是第一枪就红）
    expect(r.status).toBe(2);
    const log = gateLog(fixture);
    expect(log).toMatch(/整段重试第 2\/2 次/);
    expect(log).toMatch(/MP_WEIXIN_RESULT errors=env reason=port-not-listening/);
  }, 300000);

  it('必红：attempts=1 ⇒ 只跑一遍（证明次数真的来自 -AutoAttempts，不是写死的）', () => {
    if (!IS_WIN) return;
    fs.rmSync(path.join(fixture.dir, 'calls.log'), { force: true });
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '1']);
    expect(callsOf(fakeCli)).toEqual(['close', 'open', 'auto']);
    expect(r.status).toBe(2);
    const log = gateLog(fixture);
    expect(log).not.toMatch(/整段重试/);
    expect(log).toMatch(/MP_WEIXIN_RESULT errors=env reason=port-not-listening/);
  }, 300000);
});
