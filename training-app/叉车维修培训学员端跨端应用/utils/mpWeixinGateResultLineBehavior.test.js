/**
 * ② 门「环境性失败三分」**端到端行为守护**（#1655 / ADR-0036 处置二）。
 *
 * 上一道守护（mpWeixinGateEnvBehavior）真执行分类库本身；这一道**真跑门脚本**，
 * 用假开发者工具 + 桩探针/桩就绪闸门逼出**三种结果行**在真实执行路径上的形状与互斥：
 *   ① 通过     → 日志出现 `MP_WEIXIN_RESULT <metric 行>`（errorsTotal=0 无 failures），exit 0；
 *   ② 门未过   → `errors=<N> failures=…`，且**永不**出现 env-blocked，exit 1；
 *   ③ 环境受阻 → `errors=env-blocked reason=<slug>[ failures=… <metric>]`，exit 2。
 * ③ 有两条真实路径，各有独立用例：
 *   ③a 直接 emit 点（端口不监听 ⇒ reason=port-not-listening，裸 token 行、无 failures=/metric）；
 *   ③b 探针失败带「环境不可用：」前缀 ⇒ 尾部分类器（Get-MpWeixinEnvClassification）
 *      判 env ⇒ New-MpWeixinEnvResultLine 拼 `reason=… failures=… <metric>`（带 failures 与 metric）。
 *
 * 为什么要端到端：三分互斥是**产物形状**的契约 —— 分类库对了但门脚本没接上（比如失败分支
 * 还在用旧的裸 `errors=env` token、或把 env 行拼进了门未过行）时，只有真跑门才拦得住。
 * 对抗式自查：断言的是**日志产物**（结果行），不是源码片段；把门脚本的分类接线改坏
 * （env 失败拼成门未过行 / 门未过拼成 env 行）时，② 或 ③ 会当场翻红。
 *
 * 夹具口径（v4）：门脚本在**早期预检**就检查端口占用并在被占时换端口（Get-FreeAutoPort），
 * 所以**不能**在 runGate 前持端口（v3 教训：预检把端口换掉 ⇒ 轮询落空 ⇒ 误判 env）。
 * 真实 cli.bat auto 的行为正是「auto 之后自动化服务在端口上监听」—— 因此让假 CLI 的
 * auto 动词**派生一个分离监听器**（detached，cwd=tmpdir 不持夹具句柄 ⇒ 无 v2 的 EPERM），
 * 预检时端口仍空闲 ⇒ 不换端口 ⇒ 轮询在监听器就绪后通过。需要「端口永不监听」的用例
 * （③a）用普通假 CLI（不派生监听器）即可。
 *
 * 执行调用留在本文件（③ 门分类器按文件自身判行为守护 —— H4 纪律，见 harness 头注释）。
 */

const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const H = require('./mpWeixinGateHarness');

// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身判类）
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';

/**
 * 「auto 后端口会监听」的假 CLI（① 通过 / ② 门未过 / ③b 探针前缀 env 用）：
 * 复用 harness 的假 CLI（.bat/.sh 包装不变），把 fake-devtools.js 重写成
 * auto 动词派生分离监听器（绑 --auto-port，60s 后自灭）再回显成功文案。
 * 监听器 cwd=os.tmpdir()：不持夹具目录句柄 ⇒ finally rmSync 不会 EPERM（v2 教训）。
 */
function makeListeningDevTools(dir) {
  const cli = H.makeFakeDevTools(dir);
  const listenerPath = path.join(dir, 'fake-listener.js');
  fs.writeFileSync(listenerPath, [
    "const net = require('net');",
    'const port = Number(process.argv[2] || 0);',
    'if (!port) process.exit(0);',
    'const srv = net.createServer(() => {});',
    "srv.listen(port, '127.0.0.1', () => {});",
    'setTimeout(() => process.exit(0), 60000);'
  ].join('\n'), 'utf8');
  const appId = JSON.stringify(H.APPID);
  const listenerArg = JSON.stringify(listenerPath);
  const toolPath = path.join(dir, 'fake-devtools.js');
  const src = [
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
    '  if (port) spawn(process.execPath, [' + listenerArg + ', String(port)], { detached: true, stdio: \'ignore\', cwd: os.tmpdir() }).unref();',
    "  process.stdout.write('- Fetching AppID () permissions\\n√ Using AppID: ' + " + appId + " + '\\n√ auto\\n');",
    '} else {',
    "  process.stdout.write('√ ' + verb + '\\n');",
    '}'
  ].join('\n');
  fs.writeFileSync(toolPath, src, 'utf8');
  return cli;
}

/**
 * 「端口永不监听」的假 CLI（③a 直接 emit 点用）：普通回显假 CLI，不派生监听器 ——
 * 门轮询 portStep.waitSeconds 秒全落空 ⇒ 整段重试用尽 ⇒ errors=env-blocked
 * reason=port-not-listening（PortFallback 守护同款口径；这里只复用形状，断言三分产物行）。
 */
function makePortDeadDevTools(dir) {
  return H.makeFakeDevTools(dir);
}

// 探针/就绪闸门桩与两份合格 JSON 已收进共享夹具（#1655 与 #1863 两套直达循环守护共用，防复制品漂移）
const { writeProbeStub, writeReadyStub, probeFixture, readyFixture } = H;

/** 真跑门（起子进程；超时预算压到秒级，探针只跑一次）。 */
function runGate(dir, fakeCli, port, extraArgs) {
  const args = H.gateArgs(path.join(H.ROOT, SCRIPT_REL), { dir }, fakeCli, port, extraArgs);
  // harness 基底已带 `-PortWaitSeconds 1`：同一参数**重复传**会被 pwsh 判参数绑定错误直接退出
  // （连日志都不落盘 ⇒ 断言只能看到空串，2026-10-12 实测踩到）。要改值就**改基底**，别再追加。
  // 1s 对「auto 派生的分离监听器」偏紧（node 冷启 ~0.3–0.5s），统一抬到 5s。
  if (!extraArgs || !extraArgs.includes('-PortWaitSeconds')) {
    const i = args.indexOf('-PortWaitSeconds');
    if (i >= 0) args[i + 1] = '5';
  }
  try {
    const stdout = execFileSync('pwsh', args, { cwd: H.ROOT, encoding: 'utf8', timeout: 300000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
    return { status: 0, stdout };
  } catch (e) {
    return { status: e.status, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

/** 清理夹具；EPERM（分离监听器偶发句柄）只降级为警告，不掩盖断言结果（v2 教训）。 */
function rmFixture(dir) {
  try {
    fs.rmSync(dir, { recursive: true, force: true });
  } catch (e) {
    process.stderr.write('[warn] 夹具清理失败（不掩盖断言）: ' + e.message + '\n');
  }
}

describe('② 结果行三分端到端（真跑门脚本 + 桩探针/桩就绪闸门）', () => {
  jest.setTimeout(600000);

  it('非 Windows：fail-closed（真断言，不静默跳过）', async () => {
    if (H.IS_WIN) return;
    const fixture = H.makeFixture();
    const fakeCli = makePortDeadDevTools(fixture.dir);
    writeReadyStub(fixture.dir, readyFixture());
    writeProbeStub(fixture.dir, probeFixture([]));
    const port = await H.freePort();
    const r = runGate(fixture.dir, fakeCli, port, ['-ProbeAttempts', '1']);
    expect(r.status).not.toBe(0);
    expect(r.stdout).not.toMatch(/MP_WEIXIN_RESULT errorsTotal=0/);
    rmFixture(fixture.dir);
  });

  it('通过分支：prefix + errorsTotal=0 metric 行（无 errors=/failures=/env-blocked），exit 0', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = makeListeningDevTools(fixture.dir);
      writeReadyStub(fixture.dir, readyFixture());
      writeProbeStub(fixture.dir, probeFixture([]));
      const port = await H.freePort();
      // v4 口径：不预持端口（预检会换端口绕开）——auto 派生的分离监听器让轮询通过。
      const r = runGate(fixture.dir, fakeCli, port, ['-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      // 断言收窄到结果行：直达分支的逐页读数行（direct-page … errors=0 …）是 #1682 的新日志面，
      // 全 log 的 not.toMatch(/errors=/) 会被它顶穿——三分互斥只对结果行成立。
      const resultLine = (log.match(/MP_WEIXIN_RESULT [^\r\n]*/) || [''])[0];
      // #1690：结果行新增可选字段 auth=。桩环境没有真会话（假 CLI 的端口不应答 automator 协议），
      // 注入脚本诚实报 none(status+reason) ⇒ 这里把 auth=none(…) 钉进通过分支行形（三分互斥口径不变）。
      // #2101：新增可选字段 coverage=，夹具是**非 git 目录** ⇒ git 判据不可用 ⇒ 如实写 none（不是「覆盖为零」）。
      // 这一条同时钉住降级面：`none` 必须出现在行上，不能被静默省略（否则读的人以为覆盖判据可用）。
      expect(resultLine).toMatch(/appid=wx38c3e31b16a7ced0 pageStack=1 entry=pages\/index\/index errorsTotal=0 exceptionsTotal=0 logsTotal=0 shots=1 navigation=ok ready=\d+s sdk=3\.8\.10 ide=1\.06\.2412050 coverage=none auth=none(\([a-z0-9-]+\))? log=\.ci-verify\/mp-weixin\.log/);
      expect(resultLine).not.toMatch(/errors=/);
      expect(resultLine).not.toMatch(/env-blocked/);
      expect(r.status).toBe(0);
    } finally { rmFixture(fixture.dir); }
  });

  it('门未过分支：errors=1 failures=断言未过：…，exit 1，且永不写 env-blocked', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = makeListeningDevTools(fixture.dir);
      writeReadyStub(fixture.dir, readyFixture());
      writeProbeStub(fixture.dir, probeFixture(['断言未过：entry（期望 login）']));
      const port = await H.freePort();
      const r = runGate(fixture.dir, fakeCli, port, ['-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      expect(log).toMatch(/MP_WEIXIN_RESULT errors=1 failures=断言未过：entry（期望 login） appid=/);
      expect(log).not.toMatch(/env-blocked/);
      expect(r.status).toBe(1);
    } finally { rmFixture(fixture.dir); }
  });

  it('环境受阻·端口直接 emit：errors=env-blocked reason=port-not-listening（裸 token 行），exit 2', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = makePortDeadDevTools(fixture.dir);
      writeReadyStub(fixture.dir, readyFixture());
      writeProbeStub(fixture.dir, probeFixture([]));
      const port = await H.freePort();
      const r = runGate(fixture.dir, fakeCli, port, ['-AutoAttempts', '1', '-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      expect(log).toMatch(/MP_WEIXIN_RESULT errors=env-blocked reason=port-not-listening/);
      // 互斥：env 分支不产门未过行（errors=1 failures=…）也不产通过行（appid= 开头的 metric 行）。
      expect(log).not.toMatch(/errors=1 failures=/);
      expect(log).not.toMatch(/MP_WEIXIN_RESULT appid=/);
      expect(r.status).toBe(2);
    } finally { rmFixture(fixture.dir); }
  });

  it('环境受阻·探针前缀走尾部分类器：reason=page-stack-empty + failures= + metric，exit 2', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = makeListeningDevTools(fixture.dir);
      writeReadyStub(fixture.dir, readyFixture());
      writeProbeStub(fixture.dir, probeFixture(['环境不可用：pageStack 为空（自动化会话没有打开页面）。']));
      const port = await H.freePort();
      const r = runGate(fixture.dir, fakeCli, port, ['-ProbeAttempts', '1']);
      const log = H.gateLog({ dir: fixture.dir });
      expect(log).toMatch(/MP_WEIXIN_RESULT errors=env-blocked reason=page-stack-empty failures=环境不可用：pageStack 为空.*appid=/);
      // 互斥：探针前缀 env 也绝不写成门未过行。
      expect(log).not.toMatch(/errors=1 failures=/);
      expect(r.status).toBe(2);
    } finally { rmFixture(fixture.dir); }
  });
});
