/**
 * ③ 行为腿看门狗 / 腿级总超时 / 断点续跑（#1994）—— 行为守护。
 *
 * 为什么真执行而不是断言源码：本票要的是「假套件 sleep 5s 时阈值 3s 必判 STALL 且腿继续」这种
 * **时序行为**，源码里 `-ge $StallSeconds` 写得再对，kill 错进程 / 不继续 / 汇总补 0 也都看不出来。
 * 所以：
 *   · 假 jest = 临时目录里的 node 脚本（实现 --listTests / --runTestsByPath 两个协议）；
 *   · 假套件 = 普通文件（内容 SLEEP4 / FAIL / OK），不碰仓内真实套件；
 *   · 被测物 = `scripts/dev-finish.ps1` 的 `Invoke-BehaviorLeg` + `Write-UnitGateArtifacts`（AST 抽出来真调）。
 * 断言落在**产物**上：STALL 行形状、下一套件是否真的起过、续跑是否跳过了前缀、机检行读数。
 *
 * 不验的：跨进程「真 jest 卡死」场景（CI 上真跑行为腿才有），以及 dev-finish 分支本身（由既有 U8
 * 接线断言 + 真链路取证承担）。
 */
'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

jest.setTimeout(180000);

const ROOT = path.resolve(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'dev-finish.ps1');

// ⚠️ Windows-only：被测物 dev-finish.ps1 用 ComSpec 起子进程（cmd /d /s /c），且 ubuntu CI 上没有 pwsh
//    ⇒ 平台不满足时整组 skip 并打印原因。fail-closed 只保留给「pwsh 在、但脚本抛错」。
const hasPwsh = (() => {
  if (process.platform !== 'win32') return false;
  try {
    execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', 'exit 0'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
})();
const describeWin = hasPwsh ? describe : describe.skip;
if (!hasPwsh) {
  // eslint-disable-next-line no-console
  console.warn('[#1994] 跳过看门狗守护：platform=' + process.platform + '（dev-finish.ps1 是 Windows 工具）');
}

const FAKE_JEST = [
  "'use strict';",
  "const fs = require('fs');",
  "const path = require('path');",
  'const dir = __dirname;',
  "if (process.argv.includes('--listTests')) {",
  "  fs.readdirSync(dir).filter((f) => f.endsWith('.test.js')).sort()",
  "    .forEach((f) => console.log(path.join(dir, f)));",
  '  process.exit(0);',
  '}',
  "const at = process.argv.indexOf('--runTestsByPath');",
  'const suite = process.argv[at + 1];',
  "fs.appendFileSync(path.join(dir, 'calls.log'), path.basename(suite) + '\\n');",
  "const mode = fs.readFileSync(suite, 'utf8').trim();",
  "console.log('RUN ' + path.basename(suite));",
  "if (mode === 'SLEEP4') {",
  '  setTimeout(() => {',
  "    console.log('Test Suites: 1 passed, 1 total');",
  "    console.log('Tests: 2 passed, 2 total');",
  "    console.log('Time: 4.0 s');",
  '    process.exit(0);',
  '  }, 4000);',
  "} else if (mode === 'STDERR') {",
  "  console.error('Test Suites: 1 passed, 1 total');",
  "  console.error('Tests: 2 passed, 2 total');",
  "  console.error('Time: 0.05 s');",
  "  process.exit(0);",
  "} else if (mode === 'FAIL') {",
  "  console.log('Test Suites: 1 failed, 1 total');",
  "  console.log('Tests: 1 failed, 1 total');",
  "  console.log('Time: 0.1 s');",
  '  process.exit(1);',
  '} else {',
  "  console.log('Test Suites: 1 passed, 1 total');",
  "  console.log('Tests: 1 passed, 1 total');",
  "  console.log('Time: 0.05 s');",
  '  process.exit(0);',
  '}',
  '',
].join('\n');

function makeScenario(root, name, suites) {
  const dir = path.join(root, name);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'fake-jest.cjs'), FAKE_JEST, 'utf8');
  for (const [file, mode] of Object.entries(suites)) fs.writeFileSync(path.join(dir, file), mode + '\n', 'utf8');
  return dir;
}

/** 一次真跑四个场景：假 jest 起进程 ⇒ Invoke-BehaviorLeg ⇒ Write-UnitGateArtifacts ⇒ 读产物。 */
function runScenarios() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1994-'));
  const defs = {
    stall: { suites: { 'a.test.js': 'OK', 'b.test.js': 'SLEEP4', 'c.test.js': 'OK' }, stall: 2, timeout: 0, resume: 0 },
    resume: { suites: { 'a.test.js': 'OK', 'b.test.js': 'OK', 'c.test.js': 'OK' }, stall: 30, timeout: 0, resume: 2 },
    legTimeout: { suites: { 'a.test.js': 'SLEEP4', 'b.test.js': 'OK', 'c.test.js': 'OK' }, stall: 60, timeout: 2, resume: 0 },
    allGreen: { suites: { 'a.test.js': 'OK', 'b.test.js': 'OK' }, stall: 30, timeout: 0, resume: 0 },
    stderrSummary: { suites: { 'a.test.js': 'STDERR', 'b.test.js': 'STDERR' }, stall: 30, timeout: 0, resume: 0 },
    failed: { suites: { 'a.test.js': 'FAIL', 'b.test.js': 'OK' }, stall: 30, timeout: 0, resume: 0 },
  };
  const cases = {};
  for (const [name, def] of Object.entries(defs)) {
    const dir = makeScenario(root, name, def.suites);
    cases[name] = {
      dir,
      runner: `node "${path.join(dir, 'fake-jest.cjs')}"`,
      stall: def.stall,
      timeout: def.timeout,
      resume: def.resume,
      logPath: path.join(dir, 'behavior.log'),
      summaryPath: path.join(dir, 'behavior-failure.txt'),
    };
  }
  const inputPath = path.join(root, 'input.json');
  const resultPath = path.join(root, 'result.json');
  const harness = path.join(root, 'harness.ps1');
  fs.writeFileSync(inputPath, JSON.stringify({ script: SCRIPT, cases }), 'utf8');

  const body = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `$spec = [System.IO.File]::ReadAllText('${inputPath}', [System.Text.Encoding]::UTF8) | ConvertFrom-Json`,
    '$src = [System.IO.File]::ReadAllText($spec.script, [System.Text.Encoding]::UTF8)',
    '$tokens = $null',
    '$errors = $null',
    '$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$tokens, [ref]$errors)',
    "if (@($errors).Count -gt 0) { throw ('dev-finish.ps1 解析失败（行号）：' + ((@($errors) | ForEach-Object { $_.Extent.StartLineNumber }) -join ',')) }",
    '$fns = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)',
    "foreach ($name in @('Invoke-BehaviorLeg', 'Write-UnitGateArtifacts', 'New-TestFailureSummary')) {",
    "  if (@($fns | Where-Object { $_.Name -eq $name }).Count -ne 1) { throw \"$name 没有被抽到（被改名 / 被拆走 / 定义形态变了）\" }",
    '}',
    'foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }',
    '$out = [ordered]@{}',
    'foreach ($p in $spec.cases.PSObject.Properties) {',
    '  $sc = $p.Value',
    '  $callsLog = Join-Path $sc.dir \'calls.log\'',
    '  Remove-Item -LiteralPath $callsLog -ErrorAction SilentlyContinue',
    '  $r = Invoke-BehaviorLeg -ProjectDir $sc.dir -Runner $sc.runner -StallSeconds ([int]$sc.stall) -TotalTimeoutSeconds ([int]$sc.timeout) -ResumeFrom ([int]$sc.resume)',
    '  $art = Write-UnitGateArtifacts -Output $r.Output -LogPath $sc.logPath -SummaryPath $sc.summaryPath -ExitCode $r.ExitCode -Leg behavior -DurationSeconds 1 -PlannedSuites $r.Planned -CompletedSuites $r.Completed -ResumeFrom ([int]$sc.resume) -StalledSuite $r.StalledSuite -StalledReason $r.StalledReason -Stalls $r.Stalls -StallLimitSeconds ([int]$sc.stall) -LegTimeoutSeconds ([int]$sc.timeout) -LogRef \'behavior.log\' -HeadSha \'a1b2c3d\' -WorktreeState clean',
    '  $calls = \'\'',
    '  if (Test-Path -LiteralPath $callsLog) { $calls = ((Get-Content -LiteralPath $callsLog) -join \',\') }',
    '  $logText = [System.IO.File]::ReadAllText($sc.logPath, [System.Text.Encoding]::UTF8)',
    '  $out[$p.Name] = @{ output = $r.Output; resultLine = $art.ResultLine; calls = $calls; exitCode = $r.ExitCode; planned = $r.Planned; completed = $r.Completed; stalls = $r.Stalls; stalled = $r.StalledSuite; reason = $r.StalledReason; logText = $logText }',
    '}',
    'ConvertTo-Json -InputObject $out -Depth 6 -Compress | Set-Content -LiteralPath \'' + resultPath + '\' -Encoding UTF8',
    "$out.Keys -join ','",
  ].join('\n');
  fs.writeFileSync(harness, body, 'utf8');

  let stdout = '';
  try {
    stdout = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', harness], {
      cwd: ROOT,
      encoding: 'utf8',
      timeout: 170000,
    });
  } catch (err) {
    throw new Error('Invoke-BehaviorLeg 驱动脚本跑不起来（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n' + (err.stdout || '') + '\n' + (err.stderr || '') + '\n' + err.message);
  }
  // Invoke-BehaviorLeg 会把逐套件心跳 / STALL 行打到控制台 ⇒ 只要求**最后一行**是场景名清单
  expect(stdout.trim().endsWith(Object.keys(cases).join(','))).toBe(true);
  return JSON.parse(fs.readFileSync(resultPath, 'utf8'));
}

describeWin('行为腿看门狗 / 总超时 / 断点续跑（#1994，假 jest + 假套件，真执行真产物）', () => {
  let ctx;
  beforeAll(() => { ctx = runScenarios(); });

  test('W1: 5s 套件在 2s 阈值下判 STALL（i=k/N 行如实），腿继续跑下一套件，已完成读数照常累加', () => {
    const s = ctx.stall;
    expect(s.exitCode).toBe(1);
    expect(s.calls).toBe('a.test.js,b.test.js,c.test.js'); // 掐掉 b 之后 c 真的起了 ⇒ 腿继续
    expect(s.output).toMatch(/JEST_SUITE i=2\/3 status=STALL secs=[\d.]+(?: reason=stall)? file=b\.test\.js/);
    expect(s.output).toMatch(/RUN c\.test\.js/);
    expect(s.planned).toBe(3);
    expect(s.completed).toBe(2); // b 被 kill ⇒ 只有 a/c 是「跑完」的
    expect(s.stalls).toBe(1);
    expect(s.stalled).toBe('b.test.js');
    expect(s.resultLine).toContain('stalled=b.test.js');
    expect(s.resultLine).toContain('stalls=1');
    expect(s.resultLine).toContain('planned=3');
    expect(s.resultLine).toContain('completed=2');
    expect(s.resultLine).toMatch(/\bexit=1\b/);
    expect(s.resultLine).toContain('suites=3');
    expect(s.resultLine).toContain('suitesFailed=1');
    // 被 kill 的套件拿不到 Tests 行 ⇒ 总数不可知：不补 0，写 ? + missing=tests
    // `?` 之后没有 \b（两侧都非单词字符）⇒ 用空白收尾判定
    expect(s.resultLine).toMatch(/\btests=\?(?:\s|$)/);
    expect(s.resultLine).toContain('missing=tests');
  });

  test('W2: -BehaviorResumeFrom 2 从第 2 个套件开始、1 不重跑；拼接读数写 resumedFrom=', () => {
    const r = ctx.resume;
    expect(r.calls).toBe('b.test.js,c.test.js'); // a 一次都没起
    expect(r.output).toMatch(/JEST_SUITE_RESUME from=2 total=3 skipped=1/);
    expect(r.resultLine).toContain('resumedFrom=2');
    expect(r.resultLine).toContain('planned=3');
    expect(r.resultLine).toContain('completed=2');
    expect(r.resultLine).toContain('stalled=none');
    expect(r.exitCode).toBe(0);
  });

  test('W3: 腿级总超时到点即停，机检行带 stalled=<当前套件> 且非绿；后续套件不再起', () => {
    const t = ctx.legTimeout;
    expect(t.calls).toBe('a.test.js'); // 超时后 b/c 没起
    expect(t.output).toMatch(/JEST_SUITE i=1\/3 status=STALL secs=[\d.]+ reason=leg-timeout file=a\.test\.js/);
    expect(t.resultLine).toContain('stalled=a.test.js');
    expect(t.resultLine).toContain('stalledReason=leg-timeout');
    expect(t.resultLine).toMatch(/\bexit=1\b/);
  });

  test('W4: 全绿时字段仍在、读数为 0/none，旧字段语义不变（suites/tests/time 可复算）', () => {
    const g = ctx.allGreen;
    expect(g.exitCode).toBe(0);
    expect(g.calls).toBe('a.test.js,b.test.js');
    expect(g.resultLine).toMatch(/\bexit=0\b/);
    expect(g.resultLine).toContain('suites=2');
    expect(g.resultLine).toContain('tests=2');
    expect(g.resultLine).toContain('stalled=none');
    expect(g.resultLine).toContain('stalls=0');
    expect(g.resultLine).toContain('resumedFrom=0');
    expect(g.resultLine).toContain('planned=2');
    expect(g.resultLine).toContain('completed=2');
    // 守住「不给卡住算绿」的另一侧：没卡也不许把字段省略
    expect(g.resultLine).toMatch(/\bsuitesFailed=0\b/);
  });

  test('W5: 普通失败套件照常算红（看门狗不吞失败），并继续后面的套件', () => {
    const f = ctx.failed;
    expect(f.exitCode).toBe(1);
    expect(f.calls).toBe('a.test.js,b.test.js');
    expect(f.output).toMatch(/Test Suites: 1 failed, 1 passed, 2 total/);
    expect(f.resultLine).toContain('suitesFailed=1');
    expect(f.resultLine).toMatch(/\bexit=1\b/);
    expect(f.resultLine).toContain('stalled=none');
  });

  test('W7: 汇总行写 stderr（真 jest 形态）时 tests 仍能解析，不得退化成 ?', () => {
    const s = ctx.stderrSummary;
    expect(s.exitCode).toBe(0);
    expect(s.resultLine).toContain('tests=4');
    expect(s.resultLine).not.toContain('missing=tests');
    expect(s.output).toMatch(/Test Suites: 2 passed, 2 total/);
  });
});

describe('reporter 的全局序号接线（#1994）', () => {
  test('W6: env 给出 offset/total 时 JEST_SUITE 行按全局序号打印，PLAN 用全局总数；env 不在时退回本进程读数', () => {
    const Reporter = require('../scripts/jest-reporter-suite-progress.js');
    const writes = [];
    const origWrite = process.stdout.write;
    const origOffset = process.env.FL_BEHAVIOR_SUITE_OFFSET;
    const origTotal = process.env.FL_BEHAVIOR_SUITE_TOTAL;
    process.stdout.write = (chunk) => { writes.push(String(chunk)); return true; };
    try {
      process.env.FL_BEHAVIOR_SUITE_OFFSET = '41';
      process.env.FL_BEHAVIOR_SUITE_TOTAL = '75';
      const rep = new Reporter();
      rep.onRunStart({ numTotalTestSuites: 1 });
      rep.onTestResult({ path: path.join(__dirname, 'a.test.js') }, { numPassingTests: 2, numFailingTests: 0, perfStats: { runtime: 900 } });
      delete process.env.FL_BEHAVIOR_SUITE_OFFSET;
      delete process.env.FL_BEHAVIOR_SUITE_TOTAL;
      const local = new Reporter();
      local.onRunStart({ numTotalTestSuites: 2 });
      local.onTestResult({ path: path.join(__dirname, 'b.test.js') }, { numPassingTests: 1, numFailingTests: 0, perfStats: { runtime: 100 } });
    } finally {
      process.stdout.write = origWrite;
      if (origOffset === undefined) delete process.env.FL_BEHAVIOR_SUITE_OFFSET; else process.env.FL_BEHAVIOR_SUITE_OFFSET = origOffset;
      if (origTotal === undefined) delete process.env.FL_BEHAVIOR_SUITE_TOTAL; else process.env.FL_BEHAVIOR_SUITE_TOTAL = origTotal;
    }
    const text = writes.join('');
    expect(text).toContain('JEST_SUITE_PLAN total=75 ');
    expect(text).toMatch(/JEST_SUITE i=42\/75 status=PASS/);
    // 没有 env 时与拆分前逐字同形
    expect(text).toContain('JEST_SUITE_PLAN total=2 ');
    expect(text).toMatch(/JEST_SUITE i=1\/2 status=PASS/);
  });
});
