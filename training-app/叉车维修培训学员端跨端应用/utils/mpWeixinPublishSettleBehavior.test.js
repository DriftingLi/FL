/**
 * ② 门构建段「产物已完整而进程不退」的收流判据守护（#2163）。
 *
 * 病根现场（票面 + `.scratch/2163/handoff.md`）：`cli publish mp-weixin` 在产物**写满之后**仍然不退出
 * （实测产物 23:32:45 齐、cli 到 23:4x 还活着），而构建段的收敛信号只有 `WaitForExit(预算)` 一条
 * ⇒ 门只能烧到预算到点、以 `errors=env-blocked reason=timeout` 收场，整轮 ② 白烧。
 * 默认预算 900 s 的新树首编译更是直接「到点被杀且无产物」。
 *
 * 为什么这里真执行、不读源码文本（`docs/agents/guards.md` 判据 ①）：这条洞的后果不是红，而是
 * **门静默不收敛**；文本断言（「 Invoke-Process 里有没有那几行」）在判据被摘掉时照样绿。
 *
 * 断言面（成对：必不红 / 必红各一组，guards.md 新增守护第 2 问）
 *   S1 必不红：产物合法 + 本次写盘 + 已静默 ⇒ 收流成立（Settled=true）。
 *   S2 必红：产物不完整（app.json 声明的页面缺 .js）⇒ 不收流 —— 半途产物绝不能当成功。
 *   S3 必红：产物全是上一次的（无一文件晚于基准）⇒ 不收流 —— 这正是 #1272 的假绿族。
 *   S4 必红：产物在写、还没静默 ⇒ 不收流 —— 早收等于拿写了一半的产物出绿。
 *   S5 必红：只有指纹文件是新的、产物本体是旧的 ⇒ 不收流（指纹文件不得当作「本次写过」的证据）。
 *   S6 必不红：**真进程**——stub cli 写满产物后不退出，收流必须在预算前返回，且**没有杀任何进程**。
 *   S7 必红：stub cli 不退出且产物始终不完整 ⇒ 照旧走到预算用尽（fail-closed 与三分语义不变）。
 *
 * S6/S7 用真实进程：病根就是「进程不退 + 句柄被继承」，mock 掉等于什么都没测（F4 同律）。
 * 不吃 HBuilderX / 开发者工具 / 网络（guards.md 第 4 问：环境缺失分支必须本机造得出）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync, spawn } = require('child_process');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/mp-weixin-publish-settle.ps1';
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const { readText } = require('./utsHarness');
const { makeFixture } = require('./mpWeixinGateHarness');

const LIB = path.join(ROOT, LIB_REL);
/** 本文件的基准时刻：所有夹具 mtime 相对它摆位。 */
const SINCE_ISO = () => new Date(Date.now() - 60000).toISOString();

function pwsh(file, extraArgs) {
  const args = ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', file];
  if (extraArgs) args.push(...extraArgs);
  return execFileSync('pwsh', args, {
    encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 8 * 1024 * 1024,
  });
}

/** 取一个 PowerShell 函数的函数体（花括号配对）—— 与 mpWeixinGateDrainAndNoiseBehavior 同法。 */
function psFunctionBody(code, decl) {
  const at = code.indexOf(decl);
  if (at === -1) return '';
  const open = code.indexOf('{', at);
  if (open === -1) return '';
  let depth = 0;
  for (let i = open; i < code.length; i++) {
    if (code[i] === '{') depth++;
    else if (code[i] === '}') { depth--; if (depth === 0) return code.slice(open, i + 1); }
  }
  return '';
}

/** 把产物目录里所有文件的 mtime 拨到 `date`（含子目录）。 */
function setDistMtimes(dist, date) {
  for (const name of fs.readdirSync(dist)) {
    const p = path.join(dist, name);
    const st = fs.lstatSync(p);
    if (st.isDirectory()) { setDistMtimes(p, date); continue; }
    fs.utimesSync(p, date, date);
  }
}

function touch(p, date) { fs.utimesSync(p, date, date); }

const now = () => new Date();
const secondsAgo = (s) => new Date(Date.now() - s * 1000);

// ---------------------------------------------------------------- 判据面（纯函数）
describe('S1–S5：产物收流判据（复用 mp-weixin-product 单点真源 + 新鲜度/静默量）', () => {
  let harnessDir = '';
  beforeAll(() => {
    expect(fs.existsSync(LIB)).toBe(true); // 判据库不存在 ⇒ 下面每条都无从执行，直接红
    harnessDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-settle-'));
  });

  function settle(dist, sinceIso, quietSeconds) {
    const runner = path.join(harnessDir, 'settle.ps1');
    fs.writeFileSync(runner, [
      "$ErrorActionPreference = 'Stop'",
      `. '${LIB.replace(/\\/g, '\\')}'`,
      '$r = Get-MpWeixinPublishSettle -Dist $args[0] -Since ([datetime]::Parse($args[1])) -QuietSeconds ([int]$args[2])',
      "Write-Output ('SETTLERESULT ' + ($r | ConvertTo-Json -Compress))",
    ].join('\n'), 'utf8');
    const raw = pwsh(runner, [dist, sinceIso, String(quietSeconds)]);
    const line = raw.split(/\r?\n/).find((l) => l.startsWith('SETTLERESULT '));
    if (!line) throw new Error(`判据没出结果行：\n${raw}`);
    return JSON.parse(line.slice('SETTLERESULT '.length));
  }

  it('S1 合法 + 本次写盘 + 已静默 ⇒ 收流成立', () => {
    const { dist } = makeFixture();
    // 全部文件 30 s 前写完（晚于基准 60 s 前？否 ⇒ 让基准更早，确保 fresh）
    const since = new Date(Date.now() - 120000).toISOString();
    setDistMtimes(dist, secondsAgo(30));
    const r = settle(dist, since, 10);
    expect(r.Settled).toBe(true);
    expect(r.Reason).toBe('settled');
    expect(typeof r.Files).toBe('number');
    expect(r.Files).toBeGreaterThan(0);
  });

  it('S2 产物不完整（页面缺 .js）⇒ 不收流，且 reason 指向产物', () => {
    const { dist } = makeFixture();
    fs.unlinkSync(path.join(dist, 'pages', 'index', 'index.js'));
    const since = new Date(Date.now() - 120000).toISOString();
    setDistMtimes(dist, secondsAgo(30));
    const r = settle(dist, since, 10);
    expect(r.Settled).toBe(false);
    expect(r.Reason).toMatch(/product/);
  });

  it('S3 产物全是上一次的（无一文件晚于基准）⇒ 不收流（#1272 假绿族）', () => {
    const { dist } = makeFixture();
    setDistMtimes(dist, secondsAgo(3600));
    const r = settle(dist, SINCE_ISO(), 10);
    expect(r.Settled).toBe(false);
    expect(r.Reason).toMatch(/stale/);
  });

  it('S4 还在写（最新文件比静默窗口更近）⇒ 不收流', () => {
    const { dist } = makeFixture();
    const since = new Date(Date.now() - 120000).toISOString();
    setDistMtimes(dist, secondsAgo(30));
    touch(path.join(dist, 'app.js'), secondsAgo(1)); // 刚被写过 ⇒ 未静默
    const r = settle(dist, since, 10);
    expect(r.Settled).toBe(false);
    expect(r.Reason).toMatch(/writing|quiet/);
  });

  it('S5 只有指纹文件是新的、产物本体是旧的 ⇒ 不收流（指纹不得充当「本次写过」的证据）', () => {
    const { dist } = makeFixture();
    setDistMtimes(dist, secondsAgo(3600));
    const fp = path.join(dist, '.mp-weixin-build-fingerprint.json');
    fs.writeFileSync(fp, JSON.stringify({ schema: 'mp-weixin-build-fingerprint/2' }));
    const r = settle(dist, SINCE_ISO(), 10);
    expect(r.Settled).toBe(false);
    expect(r.Reason).toMatch(/stale/);
  });
});

// ---------------------------------------------------------------- 进程面（真执行）
describe('S6–S7：构建段等待的收流必须有界且不杀进程', () => {
  const gateSrc = readText(path.join(ROOT, SCRIPT_REL));
  const body = psFunctionBody(gateSrc, 'function Invoke-Process');
  let dir = '';

  beforeAll(() => {
    expect(body).not.toBe('');
    dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-settle-proc-'));
  });

  /** stub cli：写满一份合法产物 → 拉起一个继承 stdout 句柄的子进程 → 自己睡到被叫停。 */
  function writeStubCli(mode) {
    const dist = mode === 'hang-no-product'
      ? path.join(dir, 'empty-dist')
      : makeFixture().dist;
    fs.mkdirSync(dist, { recursive: true });
    const settleFile = path.join(dir, 'settle-marker');
    const stub = path.join(dir, 'stub-cli.js');
    fs.writeFileSync(stub, [
      'const fs = require("fs");',
      'const path = require("path");',
      'const { spawn } = require("child_process");',
      `const DIST = ${JSON.stringify(dist)};`,
      `const SETTLE = ${JSON.stringify(settleFile)};`,
      `const MODE = ${JSON.stringify(mode)};`,
      'if (MODE === "hang-with-product") {',
      '  // 模拟「本轮 publish 落盘」：把产物文件 mtime 全部推到**现在**（必须晚于调用方在发命令前',
      '  // 取的 Since —— 实测产物是构建末尾一次性写盘，本 stub 的写盘就是那一次）。',
      '  const t = new Date();',
      '  const bump = (d) => fs.readdirSync(d).forEach((n) => {',
      '    const p = path.join(d, n); const st = fs.statSync(p);',
      '    if (st.isDirectory()) bump(p); else fs.utimesSync(p, t, t);',
      '  });',
      '  bump(DIST);',
      '  fs.writeFileSync(SETTLE, "dist=" + DIST);',
      '} else {',
      '  fs.writeFileSync(SETTLE, "dist=" + DIST);',
      '}',
      // 句柄继承：与 #2107 的常驻 IDE 同族形态，子进程活着 ⇒ stdout 不 EOF
      "spawn(process.execPath, ['-e', 'setTimeout(()=>{}, 15000)'], { stdio: 'inherit', detached: false });",
      'setTimeout(() => {}, 15000);',
    ].join('\n'), 'utf8');
    return { stub, dist, settleFile };
  }

  function runHarness(mode, timeoutSeconds) {
    const { stub, dist } = writeStubCli(mode);
    const runner = path.join(dir, `harness-${mode}.ps1`);
    fs.writeFileSync(runner, [
      "$ErrorActionPreference = 'Stop'",
      '$script:workDir = (Get-Location).Path',
      `. '${LIB.replace(/\\/g, '\\')}'`,
      'function Invoke-Process ' + body,
      // Since 必须在**发命令之前**取（与门里同源）：晚取会把本轮写的产物读成「早于基准」。
      '$since = Get-Date',
      '$t0 = Get-Date',
      '$r = Invoke-Process -FilePath $args[0] -Arguments @($args[1]) -TimeoutSeconds ([int]$args[2]) ' +
        "-Tag settle-probe -SettleDist $args[3] -SettleSince $since -SettleQuietSeconds 2 -SettleGraceSeconds 5 -SettlePollSeconds 1",
      '$ms = [int]((Get-Date) - $t0).TotalMilliseconds',
      // 红线在**收流那一刻**取样，不留到事后：事后查任务表会被 stub 的自然退场混清 ——
      // execFileSync 要等继承句柄全断才拿到 EOF，那已在 stub 寿命之后，查到的「进程没了」是
      // 「自己到点退了」而不是「被门杀了」。取样点必须落在门返回与 stub 退场之间。
      '$alive = $null -ne (Get-Process -Id $r.ChildPid -ErrorAction SilentlyContinue)',
      '$out = [ordered]@{',
      '  timedOut = [bool]$r.TimedOut',
      '  settled = [bool]$r.Settled',
      '  ms = $ms',
      '  childPid = [int]$r.ChildPid',
      '  aliveAtReturn = [bool]$alive',
      '  reason = "$($r.SettleReason)"',
      '}',
      "Write-Output ('PROCRESULT ' + ($out | ConvertTo-Json -Compress))",
    ].join('\n'), 'utf8');
    let raw = '';
    try {
      raw = pwsh(runner, [process.execPath, stub, String(timeoutSeconds), dist]);
    } catch (e) {
      const msg = `${e.message}\n${e.stdout || ''}\n${e.stderr || ''}`;
      // 收流参数还不存在（修复未落地）⇒ 这是一条**真红**，不掩盖
      if (/SettleDist|找不到与参数/.test(msg)) throw new Error(`构建段尚不支持按产物收流（#2163 的病根）：${msg.slice(0, 400)}`);
      throw e;
    }
    const line = raw.split(/\r?\n/).find((l) => l.startsWith('PROCRESULT '));
    if (!line) throw new Error(`进程面没出结果行：\n${raw}`);
    return JSON.parse(line.slice('PROCRESULT '.length));
  }

  it('S6 产物已完整而进程不退 ⇒ 预算前收流、且 stub 进程仍活着（红线：不 kill）', () => {
    const budget = 40;
    const r = runHarness('hang-with-product', budget);
    expect(r.settled).toBe(true);
    expect(r.timedOut).toBe(false);
    // 远小于预算才叫收流；跑到预算 = 旧行为
    expect(r.ms).toBeLessThan(budget * 1000 * 0.6);
    expect(r.reason).toBe('settled');
    // 红线核验：收流返回的那一刻，cli 还活着 ⇒ 门没有终止它（对照 timeout 分支的 Kill($true)）
    expect(r.aliveAtReturn).toBe(true);
  }, 120000);

  it('S7 产物始终不完整 ⇒ 照旧烧到预算并判 timeout（fail-closed 与三分语义不变）', () => {
    const budget = 12;
    const r = runHarness('hang-no-product', budget);
    expect(r.settled).toBe(false);
    expect(r.timedOut).toBe(true);
    expect(r.ms).toBeGreaterThanOrEqual(budget * 1000 * 0.9);
  }, 120000);
});
