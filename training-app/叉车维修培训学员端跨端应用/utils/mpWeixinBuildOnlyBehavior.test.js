/**
 * `-BuildOnly`（#1861 第一段）**行为守护** —— 真跑门脚本，判据取产物（日志里的结果行 + 假开发者工具的调用序列）。
 *
 * 为什么必须行为守护：这一格的价值全在「**真跑到切点**、且**真不跑会话段**」——
 *   判据要是写成源码文本断言（「文件里有 Start-GateStage -Name 'automation' 之前 exit」），
 *   把 `exit 0` 挪到 automation 之后、或把切点抬到产物判据之前，守护照样绿（C20 先例：判据看着在、实际静默失效）。
 * 四组用例各钉一面：
 *   G1 计划面：`-DryRun -BuildOnly` ⇒ 结果行换 token、字段换成本轮真能读的十一项、会话段步骤整段不在计划里。
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
      // ② 字段：只声明这一轮真能读到的读数（会话段那些本轮根本不存在，列着就是逼执行路径填假值）。
      //    #2417 起补三枚**产物开关**读数 + 主包字节数与 2 MiB 硬限判定：票面要「结果行直接读出」。
      expect(plan.resultLine.fields.map((f) => f.name)).toEqual([
        'appid', 'pages', 'product', 'lazyCodeLoading', 'minified', 'es6',
        'build', 'elapsed', 'mainBytes', 'main2m', 'log'
      ]);
      // ③ 步骤：只剩构建面仍要用的收尾 close；会话段（quit / open / auto / 就绪闸门 / 探针）整段已随
      //    #2326 删除，env-preprobe（IDE 楔死预探）也随之删除
      expect(plan.steps.map((s) => s.id)).toEqual(['devtools-close']);
      // ④ exit 2 归属只声明给计划里真有的步骤：devtools-close 失败只警告 ⇒ 归属清单为空
      expect(plan.exit2).toEqual([]);
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
      expect(line).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT appid=wx38c3e31b16a7ced0 pages=1 product=ok lazyCodeLoading=requiredComponents minified=true es6=false build=skipped\(explicit\) elapsed=n\/a mainBytes=\d+ main2m=ok log=\.ci-verify\/mp-weixin\.log$/m);
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
      expect(line).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT appid=wx38c3e31b16a7ced0 pages=1 product=ok lazyCodeLoading=requiredComponents minified=true es6=false build=publish elapsed=\d+s mainBytes=\d+ main2m=ok log=\.ci-verify\/mp-weixin\.log$/m);
      // 会话段仍未跑：只有 finally 的 close（quit / open / auto 一次都没有）
      expect(H.verbsOf(fakeCli)).toEqual(['close']);
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
    } finally { rmFixture(fixture.dir); }
  });

  // ---------- #2417：产物两开关 + 主包字节数 ----------
  // 判据一律取**产物**，不取仓根配置：仓根改对而产物没带（构建没重来 / HBuilderX 没把仓根那份复制进
  // 产物）时，上传面板的「JS 压缩」「组件按需注入」照样红 ⇒ 断言源码只会给出假绿。
  // 三条失败各自一个 stage + actual= 实读值 ⇒ 看失败行就知道坏在哪一项，不用翻日志找上下文。

  /** 改产物里的某份 JSON（按相对路径）。 */
  function mutateProductJson(fixture, relPath, mutate) {
    const p = path.join(fixture.dist, ...relPath.split('/'));
    const obj = JSON.parse(fs.readFileSync(p, 'utf8'));
    mutate(obj);
    fs.writeFileSync(p, JSON.stringify(obj), 'utf8');
  }

  /** 目录文件字节总和（期望值的**独立算法** —— 不复用被测脚本的读数实现）。 */
  function sumBytes(dir, excludeRoots = []) {
    let total = 0;
    const walk = (d, rel) => {
      for (const e of fs.readdirSync(d, { withFileTypes: true })) {
        const p = path.join(d, e.name);
        const r = rel ? rel + '/' + e.name : e.name;
        if (e.isDirectory()) { if (!excludeRoots.includes(r)) walk(p, r); } else { total += fs.statSync(p).size; }
      }
    };
    walk(dir, '');
    return total;
  }

  /** 往产物里塞一份指定字节数的文件（造体积形态用，内容无意义）。 */
  function writeBlob(dir, relPath, bytes) {
    const p = path.join(dir, ...relPath.split('/'));
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, Buffer.alloc(bytes, 0x20));
  }

  it('G6a 开关面：产物 lazyCodeLoading 缺失 / 值不对 ⇒ 都判失败，失败行指名是哪一项', async () => {
    if (!H.IS_WIN) return; // 同 G2–G4：构建面在 POSIX 上走不到结果行
    const CASES = [
      { label: '缺字段', mutate: (o) => { delete o.lazyCodeLoading; }, actual: 'missing' },
      // 值在、但不对 = 等于没开：只断言「字段存在性」的守护会在这一形上假绿
      { label: '值不对', mutate: (o) => { o.lazyCodeLoading = 'required'; }, actual: 'required' }
    ];
    for (const c of CASES) {
      const fixture = H.makeFixture();
      try {
        mutateProductJson(fixture, 'app.json', c.mutate);
        const fakeCli = H.makeFakeDevTools(fixture.dir);
        const port = await H.freePort();
        const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
        const log = H.gateLog(fixture);

        expect({ label: c.label, status: r.status }).toEqual({ label: c.label, status: 1 });
        expect(log).toMatch(new RegExp('^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=lazy-code-loading actual=' + c.actual + '(\\s|$)', 'm'));
        expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
        expect(H.verbsOf(fakeCli)).toEqual(['close']);
      } finally { rmFixture(fixture.dir); }
    }
  });

  it('G6b 开关面：产物 setting.minified 不为 JSON 布尔 true ⇒ 判失败（含「字符串 true」这一形）', async () => {
    if (!H.IS_WIN) return;
    const CASES = [
      { label: 'false', mutate: (o) => { o.setting.minified = false; } },
      { label: '缺字段', mutate: (o) => { delete o.setting.minified; } },
      // ⚠️ 这一形是**故意**钉的：产物写 "minified": "true" 时字符串长得像真值，微信按布尔读 = 没开。
      //    故判据必须是 stage（严格布尔判定），不是「actual 看起来对不对」。
      { label: '字符串 true', mutate: (o) => { o.setting.minified = 'true'; } }
    ];
    for (const c of CASES) {
      const fixture = H.makeFixture();
      try {
        mutateProductJson(fixture, 'project.config.json', c.mutate);
        const fakeCli = H.makeFakeDevTools(fixture.dir);
        const port = await H.freePort();
        const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
        const log = H.gateLog(fixture);

        expect({ label: c.label, status: r.status }).toEqual({ label: c.label, status: 1 });
        expect(log).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=minified(\s|$)/m);
        expect(H.verbsOf(fakeCli)).toEqual(['close']);
      } finally { rmFixture(fixture.dir); }
    }
  });

  it('G6c 开关面：产物 setting.es6 不为 JSON 布尔 false ⇒ 判失败（降级成 ES5 只会让包更大）', async () => {
    if (!H.IS_WIN) return;
    const CASES = [
      { label: 'true', mutate: (o) => { o.setting.es6 = true; } },
      { label: "字符串 'false'", mutate: (o) => { o.setting.es6 = 'false'; } },
      { label: '缺字段', mutate: (o) => { delete o.setting.es6; } }
    ];
    for (const c of CASES) {
      const fixture = H.makeFixture();
      try {
        mutateProductJson(fixture, 'project.config.json', c.mutate);
        const fakeCli = H.makeFakeDevTools(fixture.dir);
        const port = await H.freePort();
        const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
        const log = H.gateLog(fixture);

        expect({ label: c.label, status: r.status }).toEqual({ label: c.label, status: 1 });
        expect(log).toMatch(/^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=es6(\s|$)/m);
        expect(H.verbsOf(fakeCli)).toEqual(['close']);
      } finally { rmFixture(fixture.dir); }
    }
  });

  it('G7a 体积面：主包字节数进结果行；超 1.5 MiB 建议线只提示不判红', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      // 无分包 ⇒ 主包 = 全量。1.6 MB 落在 1.5 MiB 建议线与 2 MiB 硬限之间：这是本票最要紧的一格 ——
      // 真实产物（1,595,414 B）就在这里，判红等于每次构建都失败。
      writeBlob(fixture.dist, 'static/blob.bin', 1600000);
      const expectedMain = sumBytes(fixture.dist);
      expect(expectedMain).toBeGreaterThan(1572864);
      expect(expectedMain).toBeLessThan(2097152);

      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);
      const line = resultLine(fixture, 'MP_WEIXIN_BUILDONLY_RESULT');

      expect(r.status).toBe(0);
      expect(line).toContain('mainBytes=' + expectedMain);
      expect(line).toContain('main2m=ok');
      // 建议线只写读数与「权威读数是上传面板」的注明，不判红（判红 = 每次构建必失败）
      expect(log).toMatch(/^main-package mainBytes=\d+ .* advisory1\.5m=over hard2m=ok(\s|$)/m);
    } finally { rmFixture(fixture.dir); }
  });

  it('G7b 体积面：分包目录不计入主包（全量超 2 MiB 而主包没过线 ⇒ 仍通过）', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      mutateProductJson(fixture, 'app.json', (o) => { o.subPackages = [{ root: 'pkg-a', pages: ['pages/index/index'] }]; });
      writeBlob(fixture.dist, 'pkg-a/blob.bin', 3000000);
      const expectedMain = sumBytes(fixture.dist, ['pkg-a']);
      expect(sumBytes(fixture.dist)).toBeGreaterThan(2097152);
      expect(expectedMain).toBeLessThan(1572864);

      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);
      const line = resultLine(fixture, 'MP_WEIXIN_BUILDONLY_RESULT');

      expect(r.status).toBe(0);
      expect(line).toContain('mainBytes=' + expectedMain);
      expect(log).toMatch(/^main-package .*subPackages=1 .*advisory1\.5m=under hard2m=ok(\s|$)/m);
    } finally { rmFixture(fixture.dir); }
  });

  it('G8 体积面：主包超 2 MiB 硬限 ⇒ exit 1，失败行带读数与限值', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      writeBlob(fixture.dist, 'static/blob.bin', 2100000);
      const expectedMain = sumBytes(fixture.dist);

      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);

      expect(r.status).toBe(1);
      expect(log).toMatch(new RegExp('^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=main-package mainBytes=' + expectedMain + ' limit=2097152(\\s|$)', 'm'));
      expect(H.verbsOf(fakeCli)).toEqual(['close']);
    } finally { rmFixture(fixture.dir); }
  });

  // ---------- #2418：工作树回写判据（manifest.json / pages.json） ----------
  // 判据**刻意不对称**：仅工作区脏 ⇒ 只提示（正常构建必然脏，判红等于每次构建都红）；
  // 进暂存区 ⇒ exit 1 并指名（那才是「会被并入本次提交」的真实风险）。
  // 夹具默认**不是 git 仓库**（其余用例都跑在这个形态上）⇒ 读数 git=unavailable、不判红；
  // 这里显式 git init 造出「干净 / 仅脏 / 已暂存」三种树形态。

  /** 把夹具变成 git 仓库，并把两处本地配置提交进去（干净树起点）。 */
  function initRepo(fixture) {
    const git = (args) => execFileSync('git', args, { cwd: fixture.dir, encoding: 'utf8', stdio: 'pipe' });
    fs.writeFileSync(path.join(fixture.dir, 'pages.json'), JSON.stringify({ pages: [] }), 'utf8');
    git(['init', '-q']);
    git(['config', 'user.email', 'fixture@example.com']);
    git(['config', 'user.name', 'fixture']);
    git(['add', 'manifest.json', 'pages.json']);
    git(['-c', 'commit.gpgsign=false', 'commit', '-q', '-m', 'fixture']);
    return git;
  }

  it('G9a 回写面：干净树 ⇒ 读数 clean/clean、exit 0、不打脏改提示（不误报）', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      initRepo(fixture);
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);

      expect(r.status).toBe(0);
      expect(log).toMatch(/^worktree-rewrite manifest\.json=clean pages\.json=clean git=ok$/m);
      // 干净树不打提示（提示里的还原命令是 ASCII，可跨控制台代码页断言；'>>>' 不唯一 ——
      // -SkipBuild 与 devtools-close 也用它）
      expect(r.stdout).not.toContain('git checkout -- manifest.json');
    } finally { rmFixture(fixture.dir); }
  });

  it('G9b 回写面：仅工作区脏（未暂存）⇒ 读数 dirty、只提示不判红', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      initRepo(fixture);
      // HBuilderX 回写形态：appid 被置 null
      fs.writeFileSync(path.join(fixture.dir, 'manifest.json'), JSON.stringify({ name: 'fixture', 'mp-weixin': { appid: null } }), 'utf8');
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);

      expect(r.status).toBe(0);
      expect(log).toMatch(/^worktree-rewrite manifest\.json=dirty pages\.json=clean git=ok$/m);
      // 只提示：脏改提示照打（含还原命令），但结果行照出、退出码 0
      expect(r.stdout).toContain('git checkout -- manifest.json');
      expect(resultLine(fixture, 'MP_WEIXIN_BUILDONLY_RESULT')).toContain('build=skipped(explicit)');
    } finally { rmFixture(fixture.dir); }
  });

  it('G9c 回写面：回写脏改进暂存区 ⇒ exit 1 并指名是哪一处', async () => {
    if (!H.IS_WIN) return;
    const CASES = [
      { file: 'manifest.json', mutate: (dir) => fs.writeFileSync(path.join(dir, 'manifest.json'), JSON.stringify({ name: 'fixture', 'mp-weixin': { appid: null } }), 'utf8') },
      { file: 'pages.json', mutate: (dir) => fs.writeFileSync(path.join(dir, 'pages.json'), JSON.stringify({ pages: [], condition: { current: 0, list: [{ name: 'dev', path: 'pages/index/index' }] } }), 'utf8') }
    ];
    for (const c of CASES) {
      const fixture = H.makeFixture();
      try {
        const git = initRepo(fixture);
        c.mutate(fixture.dir);
        git(['add', c.file]);
        const fakeCli = H.makeFakeDevTools(fixture.dir);
        const port = await H.freePort();
        const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
        const log = H.gateLog(fixture);

        expect({ file: c.file, status: r.status }).toEqual({ file: c.file, status: 1 });
        // 指名：staged= 后面就是那一处（两处同时暂存时是逗号分隔的清单）
        expect(log).toMatch(new RegExp('^MP_WEIXIN_BUILDONLY_RESULT errors=1 stage=worktree-rewrite staged=' + c.file.replace('.', '\\.') + '(\\s|$)', 'm'));
        expect(log).toMatch(new RegExp('^worktree-rewrite manifest\\.json=(clean|staged) pages\\.json=(clean|staged) git=ok$', 'm'));
        // 判失败 ⇒ 不写通过结果行（与其它 exit 1 出口同形）
        expect(log).not.toMatch(/^MP_WEIXIN_BUILDONLY_RESULT appid=/m);
      } finally { rmFixture(fixture.dir); }
    }
  });

  it('G9d 回写面：非 git 仓库 ⇒ 读数 git=unavailable、不判红（判不出 ≠ 判失败）', async () => {
    if (!H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const r = runGate(fixture, fakeCli, port, ['-BuildOnly']);
      const log = H.gateLog(fixture);

      expect(r.status).toBe(0);
      expect(log).toMatch(/^worktree-rewrite manifest\.json=unknown pages\.json=unknown git=unavailable$/m);
    } finally { rmFixture(fixture.dir); }
  });

  it('G5a 静态：构建面用到的顶层常量必须有顶格定义（装配漏抽常量块的直接后验）', () => {
    // 为什么存在（#2326 实测）：装配器按**行区间**从旧文件抽构建面段落，旧 340–353 的常量块整段落在
    // 所有切割区间之外 ⇒ `$WxDevToolsPatterns` 的定义被丢掉，而 `Resolve-DevToolsCli` 仍在 foreach 它。
    // Set-StrictMode 下这一支只在**不传 -DevToolsCli** 时走到；而 G2–G4 每条都传 ⇒ 套件全绿、
    // 真实 `npm run build:mp-weixin-buildonly` 当场 exit 1。⇒ 光跑套件不够，得把常量清单钉住。
    const lines = fs.readFileSync(SCRIPT, 'utf8').split(/\r?\n/);
    const definedAtTopLevel = (name) => lines.some((l) => l.startsWith('$' + name + ' ='));
    // 清单 = 构建面（publish / 产物 / 指纹 / 结果行 / 收尾 close）真用到的顶层常量，逐字对照 HEAD 的定义点
    const REQUIRED = [
      'DistRelative',        // 产物目录（旧 342 行）
      'WxDevToolsPatterns',  // 开发者工具候选路径（旧 346 行）⇒ Resolve-DevToolsCli 的枚举源
      'Doctor',              // 已退役开关的常量兜底（指纹段判定式仍取反它）
      'MainHardLimitBytes',  // 主包 2 MiB 硬限（#2417，3.6 段判定式）
      'MainAdvisoryBytes',   // 主包 1.5 MiB 建议线（#2417，只打读数不判红）
      'WorktreeRewritePaths' // 回写判据的两处本地配置（#2418，3.7 段与 finally 共用）
    ];
    for (const name of REQUIRED) {
      expect({ name, defined: definedAtTopLevel(name) }).toEqual({ name, defined: true });
    }
    // 定义存在还必须真被使用，否则就是长回来的第二份真相
    expect(fs.readFileSync(SCRIPT, 'utf8')).toContain('foreach ($p in $WxDevToolsPatterns)');
  });

  it('G5b 真实入口形态：`-BuildOnly -DryRun` 必须能跑起来（npm 那条 argv 的形状）', () => {
    // 为什么这条必须存在：npm 入口是 `pwsh -File scripts/mp-weixin-check.ps1 -BuildOnly`——**只有这一枚**
    // 开关，不带 -Project / -DevToolsCli。而 G1–G4 全部经 H.gateArgs 传 -Project 与 -DevToolsCli ⇒
    // 「默认项目根 + 候选枚举」这条真实形状从来没被执行过：装配器漏抽旧 340–353 常量块时套件照样全绿，
    // 真实入口却当场炸（#2326 实测）。⇒ 这里按真实 argv 形状跑一次。
    //
    // 为什么用 -DryRun 而不是真跑到 no-dist：后者会命中本机**真装着**的 cli.bat，使 finally 的 close
    // 把微信开发者工具拉起来再关掉（实测「IDE server has started」）；在无头管道下那还会把 stdout 写满、
    // 卡到超时。启动常驻 IDE 不属于本套件的职责（② 已退役、日常不启工具，#2320）。
    const out = execFileSync('pwsh', [
      '-NoProfile', '-NonInteractive', '-OutputFormat', 'Text', '-File', SCRIPT, '-BuildOnly', '-DryRun'
    ], { cwd: H.ROOT, encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
    const line = out.split(/\r?\n/).find((l) => l.startsWith('GATE_PLAN '));
    expect(line).toBeTruthy();
    const plan = JSON.parse(line.slice('GATE_PLAN '.length));
    // 计划能装配出来 = 顶层常量全部就位；root 取的是**默认项目根**（不传 -Project 的那一支）
    expect(plan.mode.buildOnly).toBe(true);
    expect(plan.project.root).toBe(path.resolve(H.ROOT));
    expect(plan.resultLine.prefix).toBe('MP_WEIXIN_BUILDONLY_RESULT');
  });

  it('非 Windows：构建面在 POSIX 上 fail-closed（真断言，不静默跳过）', async () => {
    if (H.IS_WIN) return;
    const fixture = H.makeFixture();
    try {
      const fakeCli = H.makeFakeDevTools(fixture.dir);
      const port = await H.freePort();
      const base = H.gateArgs(SCRIPT, fixture, fakeCli, port, ['-BuildOnly']);
      const skipAt = base.indexOf('-SkipBuild');
      const args = base.slice(0, skipAt).concat(base.slice(skipAt + 1));
      let status;
      try {
        execFileSync('pwsh', args, { cwd: H.ROOT, encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
        status = 0;
      } catch (e) {
        status = e.status;
      }
      // 判据原文由 CI 实测钉住（2026-10-28 纠正）：POSIX 上 ②/构建面链路**走不到结果行** ⇒
      // 非零退出 + 两个 token 都不出现。#2326 曾把它改回「exit 2 + reason=hbuilderx-not-found」，
      // 那是本会话在 Windows 上的推断、未经 Linux 复验 ⇒ CI 当场判红（2026-11-06 实测 Expected 2 /
      // Received 1，并伴随 gate-stages「无未闭段」异常）⇒ 恢复被 CI 验过的那条，不再拿推断当判据。
      // 这条不是「跳过」：一旦端口面/构建面真被跨平台化、在 POSIX 上能跑出结果行，它会当场提醒改写成真判据。
      expect(status).not.toBe(0);
      const log = H.gateLog(fixture);
      expect(log).not.toMatch(/^MP_WEIXIN_BUILDONLY_RESULT /m);
      expect(log).not.toMatch(/^MP_WEIXIN_RESULT /m);
    } finally { rmFixture(fixture.dir); }
  });
});
