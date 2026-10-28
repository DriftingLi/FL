/**
 * `-BuildOnly`（#1861 第一段）**行为守护** —— 真跑门脚本，判据取产物（日志里的结果行 + 假开发者工具的调用序列）。
 *
 * 为什么必须行为守护：这一格的价值全在「**真跑到切点**、且**真不跑会话段**」——
 *   判据要是写成源码文本断言（「文件里有 Start-GateStage -Name 'automation' 之前 exit」），
 *   把 `exit 0` 挪到 automation 之后、或把切点抬到产物判据之前，守护照样绿（C20 先例：判据看着在、实际静默失效）。
 * 四组用例各钉一面：
 *   G1 计划面：`-DryRun -BuildOnly` ⇒ 结果行换 token、字段换成本轮真能读的六项、会话段步骤整段不在计划里。
 *      列着「这轮不跑」的步骤就是两份真相，故这条断言的是**计划的步骤集合**，不是注释。**跨平台**（-DryRun
 *      在端口/环境检查之前退出）。
 *   G2 收口面：`-BuildOnly -SkipBuild` 真跑到切点（产物合法性之后）⇒ exit 0、
 *      结果行 `MP_WEIXIN_BUILDONLY_RESULT … build=skipped(explicit)`（**不假装构建过**）、
 *      假开发者工具只收到一次 `close`（finally 收尾）—— 没有 quit / open / auto ⇒ 会话段真没跑。
 *   G3 门未过面：产物不合法 ⇒ exit 1，结果行**仍是** BUILDONLY token（前缀走门计划的唯一取值点，
 *      不靠调用点自觉；先例：异常兜底行曾硬写 MP_WEIXIN_RESULT）。
 *   G4 publish 面（Windows）：把 node 复本当 `cli.exe`、在项目根放一份 `publish.js` 冒充「导出成功」
 *      ⇒ 真走 publish 分支：`build=publish`（而不是 skipped）、`elapsed=<N>s`、产物判据照跑、退出码 0，
 *      且会话段仍未跑（假开发者工具只有 close）。
 *
 * ⚠️ **为什么 G2–G4 是 Windows 专属、且非 Windows 有一条独立的真断言**（2026-10-28 CI 实测纠正）：
 *   `scripts/mp-weixin-check.ps1` 的端口面用 Windows 专有的 `Get-NetTCPConnection`，它排在环境检查
 *   与日志初始化**之前** ⇒ 在 Linux（CI 的 ubuntu-latest）上任何一次 ② 链路都在那里以 exit 1 结束、
 *   **一行结果行都写不出来**。这不是本守护的缺陷，是 ② 门在 POSIX 不可跑（ADR-0008 托管 CI 段）。
 *   早前本文件曾断言「POSIX 会走 exit 2 reason=hbuilderx-not-found」—— 那是错的（CI 实测 exit 1、
 *   日志为空），已改成下面这条 fail-closed 真断言（既有守护同款口径，不静默跳过）。
 *
 * 执行调用留在本文件（③ 门分类器按文件自身判行为守护 —— H4 纪律，见 mpWeixinGateHarness 头注释）。
 */

const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const H = require('./mpWeixinGateHarness');

// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身判类）
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const SCRIPT = path.join(H.ROOT, SCRIPT_REL);

/** 真跑门（超时预算压到秒级；夹具路径即项目根）。 */
function runGate(fixture, fakeCli, port, extraArgs) {
  const args = H.gateArgs(SCRIPT, fixture, fakeCli, port, extraArgs);
  try {
    const stdout = execFileSync('pwsh', args, {
      cwd: H.ROOT, encoding: 'utf8', timeout: 300000, windowsHide: true, maxBuffer: 16 * 1024 * 1024
    });
    return { status: 0, stdout };
  } catch (e) {
    return { status: e.status, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

/** 结果行只取日志（stdout 在中文 Windows 上受控制台代码页影响会乱码 —— harness 头注释的口径）。 */
function resultLine(fixture, token) {
  const log = H.gateLog(fixture);
  const re = new RegExp('^' + token + ' [^\\r\\n]*', 'm');
  const m = log.match(re);
  return m ? m[0] : '';
}

/** 清理夹具；EPERM 只降级为警告，不掩盖断言结果。 */
function rmFixture(dir) {
  try {
    fs.rmSync(dir, { recursive: true, force: true });
  } catch (e) {
    process.stderr.write('[warn] 夹具清理失败（不掩盖断言）: ' + e.message + '\n');
  }
}

describe('-BuildOnly（#1861）：只跑构建面，会话段真不跑', () => {
  jest.setTimeout(600000);

  it('G1 计划面：-DryRun -BuildOnly 换 token / 换字段 / 会话段步骤不进计划（跨平台）', () => {
    const fixture = H.makeFixture();
    try {
      const out = execFileSync('pwsh', [
        '-NoProfile', '-NonInteractive', '-OutputFormat', 'Text', '-File', SCRIPT,
        '-Project', fixture.dir, '-DryRun', '-BuildOnly'
      ], { cwd: H.ROOT, encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
      const line = out.split(/\r?\n/).find((l) => l.startsWith('GATE_PLAN '));
      expect(line).toBeTruthy();
      const plan = JSON.parse(line.slice('GATE_PLAN '.length));

      // ① token：build-only 与 ② 必须**可判读分流**（同 TEST_UNIT_LOCAL_RESULT 先例）
      expect(plan.resultLine.prefix).toBe('MP_WEIXIN_BUILDONLY_RESULT');
      // ② 字段：只声明这一轮真能读到的六项（会话段那些读数本轮根本不存在，列着就是逼执行路径填假值）
      expect(plan.resultLine.fields.map((f) => f.name)).toEqual(['appid', 'pages', 'product', 'build', 'elapsed', 'log']);
      // ③ 步骤：只剩构建面仍要用的两步；会话段（quit / open / auto / 就绪闸门 / 探针）整段不在计划里
      expect(plan.steps.map((s) => s.id)).toEqual(['env-preprobe', 'devtools-close']);
      // ④ exit 2 归属只声明给计划里真有的步骤（摊平后自动收敛，不是另写一份清单）
      expect(plan.exit2.map((e) => e.step)).toEqual(['env-preprobe']);
      // ⑤ 模式位：守护在 -DryRun 上就能区分三种 mode，不用真跑
      expect(plan.mode.buildOnly).toBe(true);
      expect(plan.mode.skipBuild).toBe(false);
    } finally { rmFixture(fixture.dir); }
  });

  it('G2 收口面：-BuildOnly -SkipBuild 跑到切点后退出，结果行如实写 build=skipped(explicit)', async () => {
    if (!H.IS_WIN) return; // POSIX 由下一条 fail-closed 用例真断言（见文件头「为什么 G2–G4 是 Windows 专属」）
    const fixture = H.makeFixture();
    try {
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);
      const line = resultLine(fixture, 'MP_WEIXIN_BUILDONLY_RESULT');

      expect(r.status).toBe(0);
      // build= 只报事实：显式 -SkipBuild ⇒ skipped(explicit)（**不得**写成 publish —— 那等于假称构建过）
      expect(line).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT appid=wx38c3e31b16a7ced0 pages=1 product=ok build=skipped\(explicit\) elapsed=n\/a log=\.ci-verify\/mp-weixin\.log$/m);
      // ② 的 token 一行都不许出现在 build-only 的日志里（分流靠 token，不靠行内容）
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
      // 会话段真没跑：假开发者工具只收到 finally 那一次 close（quit / open / auto 一个都没有）
      expect(H.verbsOf(fakeCli)).toEqual(['close']);
    } finally { rmFixture(fixture.dir); }
  });

  it('G3 门未过面：产物不合法 ⇒ exit 1，且结果行仍是 BUILDONLY token', async () => {
    if (!H.IS_WIN) return; // 同上
    const fixture = H.makeFixture();
    try {
      // 非法 JSON 产物（#1210 判据面：project.config.json 不可解析 ⇒ stage=product-json）
      fs.writeFileSync(path.join(fixture.dist, 'project.config.json'), '{ "appid": ', 'utf8');
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);

      expect(r.status).toBe(1);
      expect(log).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=product-json detail=/m);
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
      expect(H.verbsOf(fakeCli)).toEqual(['close']);
    } finally { rmFixture(fixture.dir); }
  });

  it('G4 publish 面：build=publish 且会话段仍不跑', async () => {
    if (!H.IS_WIN) return; // 同上
    const fixture = H.makeFixture();
    try {
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      // 假 HBuilderX：node 复本充 cli.exe（Windows 上 node.exe 有 MZ 头 ⇒ 过 Test-PeHeader；
      // POSIX 上该判据不可能过，且 ② 链路本身跑不起来 —— 故本用例只在 Windows 上执行）。
      const hbxDir = path.join(fixture.dir, 'hbx');
      fs.mkdirSync(hbxDir, { recursive: true });
      const cli = path.join(hbxDir, 'cli.exe');
      fs.copyFileSync(process.execPath, cli);
      // 门用 `node <verb> …`（cwd = 项目根）解析动词 ⇒ 放一份 publish.js 冒充「导出微信小程序成功」。
      // 产物不在这里重造：夹具已有合法产物，这条用例要证的是**publish 分支被走到**、不是产物生成。
      fs.writeFileSync(path.join(fixture.dir, 'publish.js'), "console.log('导出微信小程序成功');\n", 'utf8');

      // -BuildOnly 但要真构建 ⇒ 去掉 harness 基底里的 -SkipBuild，再显式给 -Cli
      const base = H.gateArgs(SCRIPT, fixture, fakeCli, port, []);
      const skipAt = base.indexOf('-SkipBuild');
      const args = base.slice(0, skipAt).concat(base.slice(skipAt + 1), ['-BuildOnly', '-Cli', cli]);
      let status;
      try {
        execFileSync('pwsh', args, { cwd: H.ROOT, encoding: 'utf8', timeout: 600000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
        status = 0;
      } catch (e) {
        status = e.status;
      }

      const line = resultLine(fixture, 'MP_WEIXIN_BUILDONLY_RESULT');
      const log = H.gateLog(fixture);
      expect(status).toBe(0);
      // build=publish（真构建过）+ elapsed 是本轮 publish 墙钟，不是 n/a
      expect(line).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT appid=wx38c3e31b16a7ced0 pages=1 product=ok build=publish elapsed=\d+s log=\.ci-verify\/mp-weixin\.log$/m);
      // 会话段仍未跑：只有 finally 的 close（quit / open / auto 一次都没有）
      expect(H.verbsOf(fakeCli)).toEqual(['close']);
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
    } finally { rmFixture(fixture.dir); }
  });

  it('非 Windows：build-only 在 POSIX 上 fail-closed（真断言，不静默跳过）', async () => {
    if (H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      // 真断言的是「在 POSIX 上 ② 链路根本走不到结果行」：端口面用 Windows 专有的 Get-NetTCPConnection，
      // 它排在环境检查与日志初始化之前 ⇒ 非零退出、且 build-only 与 ② 两个 token 都不出现。
      // 这条不是「跳过」：一旦脚本将来在 POSIX 上真能跑通（例如端口面被跨平台化），它会当场提醒改写成真判据。
      expect(r.status).not.toBe(0);
      const log = H.gateLog(fixture);
      expect(log).not.toMatch(/^MP_WEIXIN_BUILDONLY_RESULT /m);
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
    } finally { rmFixture(fixture.dir); }
  });
});
