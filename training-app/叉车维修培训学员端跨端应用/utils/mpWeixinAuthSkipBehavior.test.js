/**
 * ② 门「auth 会话注入失败 ⇒ 登录态页不判 reached」**端到端行为守护**（#1863）。
 *
 * 病根（#1845 r3 实证，票面更正评论）：mp-weixin-auth.mjs 头注承诺「任何一步失败 ⇒ ok=false，
 * 由调用方记 SKIP（auth-session-unavailable），绝不冒充取证成立」，但调用方（mp-weixin-check.ps1）
 * 没执行这条——注入失败时直达探针照常冷启动，路由级判据（pageStack[0] == 目标路由）可通过、
 * 门照判「通过」：devtools storage 里躺着旧过期 token，冷启动恢复后业务层 401 弹「登录已过期」Toast
 * 但**路由停在目标页** ⇒ 结果行 `navigation=ok` 全绿、截图实拍的是 401 态而非改动页内容。
 *
 * 修法（票内裁定）也在本守护的射程内：
 *   1. auth 相注入 ok=false ⇒ 该页**立即记 SKIP(reason=auth-session-unavailable)**，不换页不冷启动
 *      不探针——路由判据不得独自放行；组级照 #1816 出 partial（已达成页读数照常计入，不得连累）。
 *      guest 相**不在此列**（游客页无会话才是合法落地态，由落地判据兜，#1817）——入口页的 guest 注入
 *      失败只告警，门照常推进（本测试的入口页正是走这条路径到「达成」的，顺带钉住 guest 相不受影响）。
 *   2. auth 失败 detail 透传进结果行：`auth=none(<reason>:<detail-slug>)`（#1845 的
 *      wx-login-api-failed 无 detail 可归因，是本票要顺带修的登记项）。
 *
 * 为什么要端到端真跑门：降级发生在**直达循环的消费侧**（脚本聚合层），探针桩报 ok、注入桩报失败，
 * 只有真跑门才能暴露「路由判据独自放行」——桩探针 + 桩注入 + 假开发者工具（auto 派生分离监听器）
 * 把门真推进直达循环，断言的是**日志产物**（页级读数行 + 结果行），不是源码片段。
 * 正反两例各真跑一遍门：注入失败 ⇒ 页级 SKIP + partial（必绿的新契约）；注入成功 ⇒ 同一页
 * reached=True + navigation=ok（既有绿路径零回归）——拆掉降级接线时前者当场翻红。
 *
 * 监听器口径（与 mpWeixinGateResultLineBehavior 的差别）：本测试的假 CLI 在**quit 动词时杀掉监听器**
 —— 真实 `cli.bat quit` 本就连自动化服务一起退出 IDE ⇒ 每次直达冷启动的「端口收摊」判据立即通过，
 * 不必像三分守护那样等监听器 60s 自灭后被 port-held-after-quit 重试一轮。直达循环要冷启动多次
 * （GREEN 用例两页两次冷启动），这个差着 ~70s/次。
 *
 * 夹具与桩的真源在 `utils/mpWeixinGateHarness.js`（writeAuthStub / writeProbeStub / writeReadyStub /
 * probeFixture / readyFixture）；执行调用留在本文件（③ 门分类器按文件自身判行为守护，H4 纪律）。
 */

const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const H = require('./mpWeixinGateHarness');

// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身判类）
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
// auth 相页：不在游客页族（guestPhase.routes）、也非入口页 ⇒ 走 auth 相（分相判据 #1817）。
const AUTH_PAGE = 'pages/dashboard/dashboard';
const ENTRY_PAGE = 'pages/index/index';

/**
 * 「auto 后端口会监听、quit 会收摊」的假 CLI：复用 harness 假 CLI 的 .bat/.sh 包装与 calls.log 记账，
 * 把 auto 动词重写成「派生分离监听器」，quit 动词被监听器盯到即退出（真实 quit 连自动化服务一起带走）。
 * 监听器 cwd=os.tmpdir()：不持夹具目录句柄 ⇒ finally rmSync 不会 EPERM（三分守护 v2 教训同款）。
 */
function makeQuitAwareDevTools(dir) {
  const cli = H.makeFakeDevTools(dir);
  const listenerPath = path.join(dir, 'fake-listener.js');
  fs.writeFileSync(listenerPath, [
    "const net = require('net');",
    "const fs = require('fs');",
    'const port = Number(process.argv[2] || 0);',
    'const logPath = process.argv[3] || "";',
    'if (!port) process.exit(0);',
    'const srv = net.createServer(() => {});',
    "srv.listen(port, '127.0.0.1', () => {});",
    // 真实 cli.bat quit 会连自动化服务一起带走 ⇒ 监听器盯 calls.log：本轮启动之后出现 quit 条目即退出
    'const base = logPath && fs.existsSync(logPath) ? fs.statSync(logPath).size : 0;',
    'const poll = setInterval(() => {',
    '  try {',
    '    if (!logPath || !fs.existsSync(logPath)) return;',
    '    if (fs.statSync(logPath).size <= base) return;',
    '    const tail = fs.readFileSync(logPath, "utf8").slice(base);',
    '    if (tail.indexOf(\'"quit"\') !== -1) { clearInterval(poll); srv.close(() => process.exit(0)); }',
    '  } catch (e) { /* 读取竞态：下一轮 */ }',
    '}, 200);',
    'setTimeout(() => process.exit(0), 120000);'
  ].join('\n'), 'utf8');
  const appId = JSON.stringify(H.APPID);
  const listenerArg = JSON.stringify(listenerPath);
  const logArg = JSON.stringify(path.join(dir, 'calls.log'));
  const toolPath = path.join(dir, 'fake-devtools.js');
  fs.writeFileSync(toolPath, [
    "const fs = require('fs');",
    "const path = require('path');",
    "const os = require('os');",
    "const { spawn } = require('child_process');",
    'const argv = process.argv.slice(2);',
    "fs.appendFileSync(path.join(__dirname, 'calls.log'), JSON.stringify(argv) + '\\n');",
    'const verb = argv[0];',
    "if (verb === 'auto') {",
    "  const i = argv.indexOf('--auto-port');",
    "  const port = i >= 0 ? argv[i + 1] : '';",
    '  if (port) spawn(process.execPath, [' + listenerArg + ", String(port), " + logArg + "], { detached: true, stdio: 'ignore', cwd: os.tmpdir() }).unref();",
    "  process.stdout.write('- Fetching AppID () permissions\\n√ Using AppID: ' + " + appId + " + '\\n√ auto\\n');",
    '} else {',
    "  process.stdout.write('√ ' + verb + '\\n');",
    '}'
  ].join('\n'), 'utf8');
  return cli;
}

/**
 * 夹具 = harness 合法产物 + auth 相页进构建（产物合法性判据：app.json.pages 每页必须有 <page>.js，
 * scripts/lib/mp-weixin-product.ps1）——缺 dashboard.js 会在门前置断言处判「产物坏了」（exit 1），
 * 根本走不到直达循环。
 */
function makeFixtureWithAuthPage() {
  const fixture = H.makeFixture();
  const dist = path.join(fixture.dir, 'unpackage', 'dist', 'build', 'mp-weixin');
  fs.writeFileSync(path.join(dist, 'app.json'), JSON.stringify({ pages: [ENTRY_PAGE, AUTH_PAGE] }));
  fs.mkdirSync(path.join(dist, 'pages', 'dashboard'), { recursive: true });
  fs.writeFileSync(path.join(dist, 'pages', 'dashboard', 'dashboard.js'), '// fixture\n');
  return fixture;
}

/**
 * 注入桩 payload：#1845 r3 的形态（换令牌 POST 一步失败，detail 带 HTTP 状态可归因）/ 正常注入态。
 */
const AUTH_FAIL = { ok: false, status: 'none', reason: 'wx-login-api-failed', detail: 'http=500 biz=1401', user: '', elapsedMs: 5 };
const AUTH_OK = { ok: true, status: 'injected', reason: '', detail: 'wx-login+storage ok, flush=flushed', user: '2', elapsedMs: 5 };

/** 真跑门（起子进程；参数由夹具拼，执行留在本文件）。 */
function runGate(dir, fakeCli, port, extraArgs) {
  const args = H.gateArgs(path.join(H.ROOT, SCRIPT_REL), { dir }, fakeCli, port, extraArgs);
  // harness 基底已带 `-PortWaitSeconds 1`：同一参数**重复传**会被 pwsh 判参数绑定错误直接退出
  // （三分守护 2026-10-12 实测踩到）。要改值就**改基底**：1s 对「auto 派生的分离监听器」偏紧，抬到 5s。
  if (!extraArgs || !extraArgs.includes('-PortWaitSeconds')) {
    const i = args.indexOf('-PortWaitSeconds');
    if (i >= 0) args[i + 1] = '5';
  }
  try {
    // 600s：本机开着真实开发者工具时，门每次 quit 后的「进程归零轮询」要等满两轮 120s（假 CLI 的 quit
    // 不会真退真实 IDE，#1695 同款环境耦合）——GREEN 用例 3 次 quit-poll（pre-flight + 两次冷启动），
    // 预算必须盖得住 ~450s 的最坏墙钟。
    const stdout = execFileSync('pwsh', args, { cwd: H.ROOT, encoding: 'utf8', timeout: 600000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
    return { status: 0, stdout };
  } catch (e) {
    return { status: e.status, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

/** 清理夹具；EPERM（分离监听器偶发句柄）只降级为警告，不掩盖断言结果（三分守护同款）。 */
function rmFixture(dir) {
  try {
    fs.rmSync(dir, { recursive: true, force: true });
  } catch (e) {
    process.stderr.write('[warn] 夹具清理失败（不掩盖断言）: ' + e.message + '\n');
  }
}

describe('② auth 注入失败 ⇒ 登录态页不判 reached（#1863 端到端：真跑门 + 桩注入/桩探针）', () => {
  jest.setTimeout(600000);

  it('非 Windows：fail-closed（真断言，不静默跳过）', async () => {
    if (H.IS_WIN) return;
    const fixture = H.makeFixture();
    const fakeCli = H.makeFakeDevTools(fixture.dir); // 端口永不监听 ⇒ 门在端口面退出
    H.writeReadyStub(fixture.dir, H.readyFixture());
    H.writeProbeStub(fixture.dir, H.probeFixture([]));
    const port = await H.freePort();
    const r = runGate(fixture.dir, fakeCli, port, ['-ProbeAttempts', '1']);
    expect(r.status).not.toBe(0);
    expect(r.stdout).not.toMatch(/MP_WEIXIN_RESULT appid=/);
    rmFixture(fixture.dir);
  });

  it('注入失败 ⇒ auth 相页记 SKIP(auth-session-unavailable)、不判 reached，组级 partial 而非 ok；detail 进结果行', async () => {
    if (!H.IS_WIN) return;
    const fixture = makeFixtureWithAuthPage();
    try {
      const fakeCli = makeQuitAwareDevTools(fixture.dir);
      H.writeReadyStub(fixture.dir, H.readyFixture());
      H.writeProbeStub(fixture.dir, H.probeFixture([])); // 路由判据「通过」——正是 #1845 的假绿形态
      H.writeAuthStub(fixture.dir, AUTH_FAIL);
      const port = await H.freePort();
      const r = runGate(fixture.dir, fakeCli, port, ['-Pages', AUTH_PAGE, '-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      const resultLine = (log.match(/MP_WEIXIN_RESULT [^\r\n]*/) || [''])[0];
      // 页级：登录态页不被判 reached——页级读数是 SKIP（reason 可归因到缺会话），且不出现 reached=True
      expect(log).toContain('direct-page ' + AUTH_PAGE + ' reached=skip(reason=auth-session-unavailable)');
      expect(log).toContain('reason=wx-login-api-failed detail=http=500 biz=1401');
      expect(log).not.toMatch(new RegExp('direct-page ' + AUTH_PAGE.replace(/\//g, '\\/') + ' reached=True'));
      // 结果行：组级不得 navigation=ok 全绿 —— 部分达成（#1816 形态：已达成页照常计入）
      expect(resultLine).toMatch(/navigation=partial\(ok=1,skip=1\)/);
      expect(resultLine).not.toMatch(/navigation=ok/);
      // detail 透传（#1863）：auth=none(<reason>:<HTTP 状态 slug>)
      expect(resultLine).toMatch(/auth=none\(wx-login-api-failed:http=500_biz=1401\)/);
      // partial 仍 exit 0（#1816：已达成页读数照常计入门结论，未达成页如实标注）——
      // 拆掉降级接线时本条照旧 0，但上面两条会红；钉 0 是为了挡「过度矫正成整门失败」。
      expect(r.status).toBe(0);
    } finally { rmFixture(fixture.dir); }
  }, 600000);

  it('零回归：注入成功（injected）⇒ 同一 auth 相页 reached=True、navigation=ok（既有绿路径不变）', async () => {
    if (!H.IS_WIN) return;
    const fixture = makeFixtureWithAuthPage();
    try {
      const fakeCli = makeQuitAwareDevTools(fixture.dir);
      H.writeReadyStub(fixture.dir, H.readyFixture());
      H.writeProbeStub(fixture.dir, H.probeFixture([]));
      H.writeAuthStub(fixture.dir, AUTH_OK);
      const port = await H.freePort();
      const r = runGate(fixture.dir, fakeCli, port, ['-Pages', AUTH_PAGE, '-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      const resultLine = (log.match(/MP_WEIXIN_RESULT [^\r\n]*/) || [''])[0];
      expect(log).toMatch(new RegExp('direct-page ' + AUTH_PAGE.replace(/\//g, '\\/') + ' reached=True'));
      expect(resultLine).toMatch(/navigation=ok/);
      expect(resultLine).toMatch(/auth=injected/);
      expect(r.status).toBe(0);
    } finally { rmFixture(fixture.dir); }
  }, 600000);
});
