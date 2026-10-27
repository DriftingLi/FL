/**
 * 行为腿「受影响子集」的接线与判据（#2034，ADR-0046 步骤 3）—— 行为守护。
 *
 * 为什么真执行：本票要的不是「源码里出现了 `--changed-json`」，而是
 *   ① 映射真能算出子集、四条兜底边真能把「结构性改动」放大成全跑；
 *   ② 子集交给 `Invoke-BehaviorLeg -OnlySuites` 后**只起这几个套件**、且名字对不上时**当场判非绿**；
 *   ③ 子集跑法的机检行**不是** ③ 门全量那一行（`mode=changed` 独立首 token）；
 *   ④ 改动清单的取法能看见**未跟踪**文件（新写的 `.uts` 常常还没 `git add`）。
 * 这些每一条都能「源码写得对但行为不对」，所以：映射部分真跑 node 工具读 JSON，
 * 腿部分用假 jest + 真 `Invoke-BehaviorLeg`（与 #1994 的看门狗守护同一套夹具思路）。
 *
 * 不验的：真 jest 跑真套件的墙钟（那是收口取证，见票面「子集墙钟 vs 全量墙钟」），
 * 以及「子集是否覆盖了该跑的套件」这张映射表本身（由 `docs/design/2019-…报告.md` §6.2 承担）。
 */
'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync, spawnSync } = require('node:child_process');
const { readText } = require('./utsHarness');

const ROOT = path.resolve(__dirname, '..');
const TOOL = path.join(ROOT, 'scripts', 'behavior-suite-deps.mjs');
const SCRIPT = path.join(ROOT, 'scripts', 'dev-finish.ps1');
const SUBSET_LIB = path.join(ROOT, 'scripts', 'lib', 'behavior-subset.ps1');

jest.setTimeout(180000);

/** 跑一次映射工具（JSON 面）。改动清单走 stdin —— 与 dev-finish 里的调用形态一致。 */
function toolJson(changed) {
  const r = spawnSync(process.execPath, [TOOL, '--changed-json', '--changed-from', 'stdin'], {
    cwd: ROOT,
    input: changed.join('\n') + '\n',
    encoding: 'utf8',
  });
  return { status: r.status, stdout: r.stdout, stderr: r.stderr, json: r.status === 0 ? JSON.parse(r.stdout) : null };
}

// ⚠️ Windows-only：`Invoke-BehaviorLeg` 用 ComSpec 起子进程、`Get-ChangedPathsForSubset` 要 git；
//    ubuntu CI 上没有 pwsh ⇒ 整组 skip 并打印原因（映射部分的两条与平台无关，仍跑）。
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
  console.warn('[#2034] 跳过子集接线守护：platform=' + process.platform + '（dev-finish.ps1 是 Windows 工具）');
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
  "console.log('Test Suites: 1 passed, 1 total');",
  "console.log('Tests: 2 passed, 2 total');",
  "console.log('Time: 0.05 s');",
  'process.exit(0);',
  '',
].join('\n');

/**
 * 一次真跑四件事（同一个 harness 进程，省掉四次 pwsh 启动）：
 *   legSubset  —— 假 jest 列 3 个套件、只交 2 个 ⇒ 只这 2 个起进程、planned=2、退出码 0
 *   legMissing —— 交一个清单里没有的名字 ⇒ 非绿 + 逐点点名 + **一个套件都没起**
 *   resultLine —— `Write-UnitGateArtifacts -Mode changed` 的首 token / mode 字段；默认模式逐字不变
 *   subset     —— 真调 `Get-BehaviorSubset`（与 node 工具同源；改动清单固定一份，**不读工作树**）；工具路径不存在 ⇒ Ok=$false
 *   changedGit —— 临时 git 仓库：已跟踪改动 + **未跟踪** 文件都要在清单里
 */
function runHarness() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-2034-'));
  const legDir = path.join(root, 'leg');
  fs.mkdirSync(legDir, { recursive: true });
  fs.writeFileSync(path.join(legDir, 'fake-jest.cjs'), FAKE_JEST, 'utf8');
  for (const f of ['a.test.js', 'b.test.js', 'c.test.js']) fs.writeFileSync(path.join(legDir, f), 'OK\n', 'utf8');

  const inputPath = path.join(root, 'input.json');
  const resultPath = path.join(root, 'result.json');
  const harnessPath = path.join(root, 'harness.ps1');
  fs.writeFileSync(
    inputPath,
    JSON.stringify({ script: SCRIPT, subsetLib: SUBSET_LIB, legDir, projectDir: ROOT, tmp: root }),
    'utf8'
  );

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
    '. $spec.subsetLib',
    '$runner = "node `"$(Join-Path $spec.legDir \'fake-jest.cjs\')`""',
    "$out = [ordered]@{}",
    '',
    '# ---- ① 只跑交给我的子集 ----',
    '$log = Join-Path $spec.legDir \'calls.log\'',
    "Remove-Item -LiteralPath $log -ErrorAction SilentlyContinue",
    '$r = Invoke-BehaviorLeg -ProjectDir $spec.legDir -Runner $runner -StallSeconds 30 -TotalTimeoutSeconds 0 -OnlySuites @(\'a.test.js\', \'c.test.js\')',
    "$calls = ''; if (Test-Path -LiteralPath $log) { $calls = ((Get-Content -LiteralPath $log) -join ',') }",
    '$out[\'legSubset\'] = @{ output = $r.Output; calls = $calls; planned = $r.Planned; completed = $r.Completed; exitCode = $r.ExitCode }',
    '',
    '# ---- ② 名字对不上就 fail-closed ----',
    "Remove-Item -LiteralPath $log -ErrorAction SilentlyContinue",
    '$r = Invoke-BehaviorLeg -ProjectDir $spec.legDir -Runner $runner -StallSeconds 30 -TotalTimeoutSeconds 0 -OnlySuites @(\'a.test.js\', \'zzz-missing.test.js\')',
    "$calls = ''; if (Test-Path -LiteralPath $log) { $calls = ((Get-Content -LiteralPath $log) -join ',') }",
    '$out[\'legMissing\'] = @{ output = $r.Output; calls = $calls; planned = $r.Planned; exitCode = $r.ExitCode }',
    '',
    '# ---- ③ 机检行身份：changed 与默认门必须可判读地区分 ----',
    "$logPath = Join-Path $spec.tmp 'r.log'; $sumPath = Join-Path $spec.tmp 'r.txt'",
    '$text = "Test Suites: 2 passed, 2 total`r`nTests: 4 passed, 4 total`r`nTime: 0.4 s`r`n"',
    '# -Mode changed 时行为腿发 TEST_BEHAVIOR_CHANGED_RESULT + mode=changed；不给 Mode 的那次逐字仍是旧形态',
    "$artChanged = Write-UnitGateArtifacts -Output $text -LogPath $logPath -SummaryPath $sumPath -ExitCode 0 -Mode changed -Leg behavior -DurationSeconds 1 -PlannedSuites 2 -CompletedSuites 2 -HeadSha 'a1b2c3d' -WorktreeState clean",
    "$artGate = Write-UnitGateArtifacts -Output $text -LogPath $logPath -SummaryPath $sumPath -ExitCode 0 -Leg behavior -DurationSeconds 1 -PlannedSuites 2 -CompletedSuites 2 -HeadSha 'a1b2c3d' -WorktreeState clean",
    '$out[\'resultLine\'] = @{ changed = $artChanged.ResultLine; gate = $artGate.ResultLine }',
    '',
    '# ---- ④ 子集计算的接线（真调工具 / 工具缺失 fail-closed）----',
    '# 改动清单**写死一份**，不从工作树取：收口时工作树通常是干净的，而「此刻恰好有未提交改动」的守护',
    '# 在全量跑里必红、单跑时又恰好绿（2026-10-27 全量实测踩到，红在 @($real.Subset)[0] 越界）。',
    '# 取改动清单这条接线由 ⑤ 的临时仓库承担 —— 那里脏/未跟踪状态是自己造的，可控。',
    "$changed = @('utils/markdown.uts')",
    '$real = Get-BehaviorSubset -ProjectDir $spec.projectDir -Changed $changed',
    "$bogus = Get-BehaviorSubset -ProjectDir $spec.projectDir -Changed $changed -ToolRelative 'scripts/no-such-tool.mjs'",
    "$out['subset'] = @{ changedCount = $real.ChangedCount; ok = $real.Ok; total = $real.Total; subsetCount = @($real.Subset).Count; first = (@($real.Subset) | Select-Object -First 1); explainLines = @($real.Explain).Count; explainHead = (@($real.Explain) | Select-Object -First 1); bogusOk = $bogus.Ok; bogusError = $bogus.Error }",
    '',
    '# ---- ⑤ 改动清单要看见未跟踪文件 ----',
    "$repo = Join-Path $spec.tmp 'gitrepo'",
    'New-Item -ItemType Directory -Force -Path $repo | Out-Null',
    '& git -C $repo init -q 2>$null',
    '& git -C $repo config user.email a@b.c',
    '& git -C $repo config user.name t',
    "[System.IO.File]::WriteAllText((Join-Path $repo 'tracked.txt'), 'x')",
    '& git -C $repo add tracked.txt 2>$null',
    '& git -C $repo commit -qm init 2>$null',
    "[System.IO.File]::WriteAllText((Join-Path $repo 'tracked.txt'), 'y')",
    "[System.IO.File]::WriteAllText((Join-Path $repo '新页.uvue'), 'z')",
    '$gitPaths = @(Get-ChangedPathsForSubset -ProjectDir $repo)',
    "$out['changedGit'] = @{ paths = ($gitPaths -join '|') }",
    '',
    'ConvertTo-Json -InputObject $out -Depth 6 -Compress | Set-Content -LiteralPath \'' + resultPath + '\' -Encoding UTF8',
    "'DONE'",
  ].join('\n');
  fs.writeFileSync(harnessPath, body, 'utf8');

  let stdout = '';
  try {
    stdout = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', harnessPath], {
      cwd: ROOT,
      encoding: 'utf8',
      timeout: 170000,
    });
  } catch (err) {
    throw new Error('子集接线驱动跑不起来（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n' + (err.stdout || '') + '\n' + (err.stderr || '') + '\n' + err.message);
  }
  // 产物缺失 = 驱动「跑过了但什么都没写」，比抛错更该被点名（fail-closed，不静默返回空对象）。
  if (!fs.existsSync(resultPath)) {
    throw new Error('harness 没产出 result.json（stdout=' + stdout + '）\nharness 原文：\n' + fs.readFileSync(harnessPath, 'utf8'));
  }
  return JSON.parse(fs.readFileSync(resultPath, 'utf8'));
}

describe('#2034 行为腿受影响子集：映射（node 工具真跑，JSON 面）', () => {
  test('M1: 单个 .uts ⇒ 子集严格小于全量，每条套件都带原因，且含「直接读到这个文件」的边', () => {
    const { status, json } = toolJson(['utils/markdown.uts']);
    expect(status).toBe(0);
    expect(json.total).toBeGreaterThan(0);
    expect(json.subset.length).toBeGreaterThan(0);
    // 不写死比例/数量：只钉「这是子集不是全量」（报告 §7.1 的 21–22 是当次读数，不是契约）
    expect(json.subset.length).toBeLessThan(json.total);
    // 头行是「应跑 x/y」的现测形态
    expect(json.explain[0]).toContain(`${json.subset.length}/${json.total}`);
    for (const s of json.subset) expect((json.reasons[s] || []).length).toBeGreaterThan(0);
    const flat = Object.values(json.reasons).flat();
    expect(flat.some((r) => r.includes('direct:markdown.uts') || r.includes('import-closure:markdown.uts'))).toBe(true);
  });

  test('M2: 三条结构性兜底边逐条放大成全跑（pages.json / 门脚本 / 套件集合），且原因行点名是哪条边', () => {
    const cases = [
      ['pages.json', 'page-set'],
      ['scripts/dev-finish.ps1', 'gate-script'],
      ['utils/jestSplitContract.test.js', 'suite-set'],
    ];
    for (const [file, edge] of cases) {
      const { status, json } = toolJson([file]);
      expect(`${file}:${status}`).toBe(`${file}:0`);
      // 兜底边 = 全跑（不是「几乎全跑」）：少一个就是映射漏洞
      expect(`${file}:${json.subset.length}`).toBe(`${file}:${json.total}`);
      const flat = Object.values(json.reasons).flat();
      expect(`${file}:${flat.some((r) => r.startsWith(edge + ':'))}`).toBe(`${file}:true`);
    }
  });

  test('M3: 空改动清单 ⇒ 非零退出且 stdout 为空（0 套件不许长得像绿）', () => {
    const r = spawnSync(process.execPath, [TOOL, '--changed-json', '--changed-from', 'stdin'], {
      cwd: ROOT,
      input: '\n',
      encoding: 'utf8',
    });
    expect(r.status).not.toBe(0);
    expect(r.stdout.trim()).toBe('');
    expect(r.stderr).toContain('--changed');
  });

  test('M4: `--changed` 的人读文本与 `--changed-json` 的 explain 逐行同一份（尾行命令 = subset）', () => {
    const changed = ['docs/2034-note.md', 'utils/markdown.uts'];
    const human = spawnSync(process.execPath, [TOOL, '--changed', ...changed], { cwd: ROOT, encoding: 'utf8' });
    const { json } = toolJson(changed);
    expect(human.status).toBe(0);
    const lines = human.stdout.replace(/\r\n/g, '\n').replace(/\n$/, '').split('\n');
    expect(lines).toEqual(json.explain);
    expect(lines[lines.length - 1].trim()).toBe(
      `npx jest --config jest.config.behavior.js --runTestsByPath ${json.subset.join(' ')}`
    );
  });
});

describeWin('#2034 行为腿受影响子集：接线（假 jest / 真 Invoke-BehaviorLeg，真产物）', () => {
  let ctx;
  beforeAll(() => {
    ctx = runHarness();
  });

  test('S1: -OnlySuites 只起交给它的套件，planned 按子集算，退出码仍由 jest 给', () => {
    const s = ctx.legSubset;
    expect(s.calls).toBe('a.test.js,c.test.js'); // b 一次都没起
    expect(s.planned).toBe(2);
    expect(s.completed).toBe(2);
    expect(s.exitCode).toBe(0);
    expect(s.output).toMatch(/JEST_SUITE_SUBSET requested=2 listed=3 selected=2 missing=0/);
    // 汇总行按**本次子集**累加（2 个套件），不是按 --listTests 的 3 个
    expect(s.output).toMatch(/Test Suites: 2 passed, 2 total/);
  });

  test('S2: 子集里的名字不在 --listTests 清单里 ⇒ 非绿 + 逐个点名 + 一个套件都不起', () => {
    const s = ctx.legMissing;
    expect(s.exitCode).not.toBe(0);
    expect(s.calls).toBe(''); // fail-closed：不跑「能跑的」那些来凑一个绿
    expect(s.planned).toBe(0);
    expect(s.output).toContain('JEST_SUITE_SUBSET_MISSING file=zzz-missing.test.js');
    expect(s.output).toMatch(/missing=1/);
  });

  test('S3: 机检行身份 —— changed 发独立首 token + mode=changed，默认门那行逐字仍是旧形态', () => {
    expect(ctx.resultLine.changed.split(' ')[0]).toBe('TEST_BEHAVIOR_CHANGED_RESULT');
    expect(ctx.resultLine.changed).toContain('mode=changed');
    // 成对腿：不给 -Mode 时既有读者读到的东西一个字都没变（不是「也带 mode=」）
    expect(ctx.resultLine.gate.split(' ')[0]).toBe('TEST_BEHAVIOR_RESULT');
    expect(ctx.resultLine.gate).not.toContain('mode=');
    expect(ctx.resultLine.gate).not.toContain('CHANGED');
  });

  test('S4: Get-BehaviorSubset 的取数与工具同源；工具缺失 ⇒ Ok=$false 且 Error 点名（不静默 0 套件）', () => {
    const s = ctx.subset;
    expect(s.changedCount).toBe(1); // 交给它 1 条改动 ⇒ 工具回的 changedCount 就是 1（不是从工作树推的）
    expect(s.ok).toBe(true);
    expect(s.total).toBeGreaterThan(0);
    expect(s.subsetCount).toBeGreaterThan(0);
    expect(s.subsetCount).toBeLessThanOrEqual(s.total);
    expect(s.explainHead).toContain(`${s.subsetCount}/${s.total}`);
    expect(s.explainLines).toBeGreaterThan(s.subsetCount); // 每条套件两行 + 头尾，不是光秃秃一个列表
    expect(s.bogusOk).toBe(false);
    expect(s.bogusError).toContain('no-such-tool.mjs');
  });

  test('S5: 改动清单含未跟踪文件（新写的 .uts/.uvue 还没 git add 时不能被漏掉）', () => {
    const paths = ctx.changedGit.paths.split('|');
    expect(paths).toContain('tracked.txt'); // 已跟踪改动（git diff HEAD）
    expect(paths).toContain('新页.uvue'); // 未跟踪新增（git ls-files --others）
  });
});

describe('#2034 行为腿受影响子集：脚本接线（读源码，钉「不抄第二份参数」）', () => {
  const src = readText(SCRIPT);
  const branchStart = src.indexOf('if ($Changed)');
  const branchEnd = src.indexOf('if ($UnitLocal)');
  const branch = branchStart > -1 && branchEnd > branchStart ? src.slice(branchStart, branchEnd) : '';

  test('W1: 参数与 npm 入口在（-Changed / test:unit:changed 指向同一处）', () => {
    expect(src).toMatch(/\[switch\]\$Changed/);
    const pkg = JSON.parse(readText(path.join(ROOT, 'package.json')));
    expect(pkg.scripts['test:unit:changed']).toContain('scripts/dev-finish.ps1 -Changed');
    // 本模式不新增行为腿/接线腿的 jest 参数 —— 跑的还是 package.json 那两条
    expect(branch).toContain('& npm run test:unit');
    expect(branch).toContain("npm run test:behavior --");
    expect(branch).not.toContain('jest.config.unit.js');
    expect(branch).not.toContain('jest.config.behavior.js');
    // 工具尾行那条并行命令只作为「不是本次跑法」的提示出现，不许真被当命令抄进来执行
    expect(branch).not.toMatch(/npx jest --config/);
  });

  test('W2: 分支位置在 -DryRun 之后、-UnitLocal 之前（既有分支切片边界一个字不动）', () => {
    const dryRunAt = src.indexOf('if ($DryRun)');
    expect(branchStart).toBeGreaterThan(dryRunAt);
    expect(branchEnd).toBeGreaterThan(branchStart);
    // 子集算在**跑测试之前**（前置步骤），且子集现算、不落盘成映射文件
    expect(branch.indexOf('Get-BehaviorSubset')).toBeGreaterThan(-1);
    expect(branch.indexOf('Get-BehaviorSubset')).toBeLessThan(branch.indexOf('Invoke-BehaviorLeg'));
    expect(branch).toContain('-OnlySuites $subsetInfo.Subset');
    // 不贴 sha 绑定评论、不进 CI 证据面
    expect(branch).not.toContain('Publish-UnitGateComment');
    // 兜底：子集为空/算不出来 ⇒ 非零退出（不以「没跑」当绿）
    expect(branch).toMatch(/if \(-not \$subsetInfo\.Ok\)/);
  });
});
