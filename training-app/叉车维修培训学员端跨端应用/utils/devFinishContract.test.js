/**
 * 主脚本契约守护（scripts/dev-finish.ps1）
 *
 * 背景（2026-09-14，A-3 裁决）：HBuilderX 是**单实例串行资源**，而 `dev:finish` 有**三个**步骤需要它
 * （编译 → 真运行部署 → 逐页截图），且其中后两步**共享设备状态**：若分三次各拿一次锁，
 * 别的会话可能在「部署到目标页」与「截图」之间插进来 launch 到别的页 ⇒ **截到别人的页面**。
 * 故改为**一个临界区**覆盖步骤 4–6；步骤 1–3（不碰 HBuilderX）与 7–9（纯本地）留在锁外，
 * 尽量缩短持有时间。
 *
 * **锁交接**：`hx-busy.ps1` 的锁**按 PID 判定且不可重入** ⇒ 父进程持锁后再调子脚本会**自死锁**。
 * 故持锁后调 `Set-HxLockOwnerEnv`（写 `$env:HX_LOCK_OWNER`）交接，子脚本认到同一持有者就复用；
 * `finally` 里 `Clear-HxLockOwnerEnv` + `Release-HxLock`。
 *
 * 守护的不变量：
 *   F1  参数定义正确（Device, Level, DryRun, Distribute, UpdateBaseline, HxWaitSeconds）
 *   F2  步骤顺序正确（环境→级别→测试→编译→部署→截图→对比→证据→还原）
 *   F3  退出码语义正确（env=2, fail=1, ok=0）
 *   F4  -DryRun 不执行实际操作
 *   F5  dot-source 所有子模块
 *   F6  【A-3】dot-source hx-busy 并调 Wait-HxFree（自己持锁）
 *   F7  【A-3】锁在 finally 路径里释放（Release-HxLock）
 *   F8  【A-3】临界区**只**覆盖步骤 4–6（步骤 1–3 在锁前，7–9 在锁后）
 *   F9  【锁交接】设置并清理 $env:HX_LOCK_OWNER（Set-/Clear-HxLockOwnerEnv）
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const { execFileSync } = require('child_process');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/dev-finish.ps1';

function readSource() {
  return readText(path.join(ROOT, SCRIPT_REL));
}

describe('dev-finish.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('F1: parameters defined correctly', () => {
    ['[string]$Device', '[string]$Level', '[switch]$DryRun', '[switch]$Distribute', '[switch]$UpdateBaseline', '$HxWaitSeconds'].forEach((p) => {
      expect(src).toContain(p);
    });
  });

  test('F2: steps in correct order', () => {
    const order = ['Step 1', 'Step 2', 'Step 3', 'Step 4', 'Step 5', 'Step 6', 'Step 7', 'Step 8', 'Step 9'];
    const idx = order.map((s) => src.indexOf(s));
    idx.forEach((v, i) => expect(v).toBeGreaterThan(-1));
    for (let i = 1; i < idx.length; i++) {
      expect(idx[i]).toBeGreaterThan(idx[i - 1]);
    }
  });

  test('F3: exit codes correct (env=2, fail=1, ok=0)', () => {
    expect(src).toContain('exit 2');
    expect(src).toContain('exit 1');
    expect(src).toContain('exit 0');
  });

  test('F4: DryRun mode', () => {
    expect(src).toContain('DryRun');
    expect(src).toContain('不执行任何操作');
  });

  test('F5: dot-sources all sub-modules', () => {
    ['env-check.ps1', 'level-detect.ps1', 'test-compile.ps1', 'build-deploy.ps1', 'auto-screenshot.ps1', 'screenshot-diff.ps1', 'evidence-gen.ps1', 'contract-tests.ps1'].forEach((m) => {
      expect(src).toContain(m);
    });
  });

  test('F6: 【A-3】holds the HBuilderX lock itself (hx-busy + Wait-HxFree)', () => {
    expect(src).toMatch(/lib[\\/]hx-busy\.ps1/);
    expect(src).toContain('Wait-HxFree');
  });

  test('F7: 【A-3】releases the lock in a finally path', () => {
    const finallyAt = src.lastIndexOf('} finally {');
    expect(finallyAt).toBeGreaterThan(-1);
    expect(src.slice(finallyAt)).toContain('Release-HxLock');
  });

  test('F8: 【A-3】critical section covers ONLY steps 4–6', () => {
    const lockAt = src.indexOf('Wait-HxFree');
    const releaseAt = src.lastIndexOf('Release-HxLock');
    expect(lockAt).toBeGreaterThan(-1);
    expect(releaseAt).toBeGreaterThan(lockAt);

    // 步骤 1–3（不碰 HBuilderX）必须在取锁**之前**
    ['Write-Step 1 9', 'Write-Step 2 9', 'Write-Step 3 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(-1);
      expect(i).toBeLessThan(lockAt);
    });

    // 步骤 4–6（需要 HBuilderX）必须在锁**之内**
    ['Write-Step 4 9', 'Write-Step 5 9', 'Write-Step 6 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(lockAt);
      expect(i).toBeLessThan(releaseAt);
    });

    // 步骤 7–9（纯本地）必须在锁**之后**
    ['Write-Step 7 9', 'Write-Step 8 9', 'Write-Step 9 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(releaseAt);
    });
  });

  test('F9: 【锁交接】sets and clears $env:HX_LOCK_OWNER', () => {
    expect(src).toContain('Set-HxLockOwnerEnv');
    expect(src).toContain('Clear-HxLockOwnerEnv');
    // 清理必须在 finally 里（否则失败路径会留下陈旧交接）
    const finallyAt = src.lastIndexOf('} finally {');
    expect(src.slice(finallyAt)).toContain('Clear-HxLockOwnerEnv');
  });

  // F10（2026-09-14 Q-2 修正）：Q-A（quick 且未 -Compile）**不取锁**。
  // 否则别的会话持锁时，「秒级静态守护」会白等到超时 ⇒ 定位名不副实。
  test('F10: 【Q-A】lock acquisition is conditional on needing HBuilderX', () => {
    expect(src).toContain('$needsHx');
    // 取锁必须在 $needsHx 分支内
    const hxAt = src.indexOf('Wait-HxFree');
    expect(hxAt).toBeGreaterThan(-1);
    const guard = src.slice(Math.max(0, hxAt - 600), hxAt);
    expect(guard).toMatch(/if\s*\(\s*\$needsHx\s*\)/);
    // Q-A 路径必须走轻量检查（不要求设备）
    expect(src).toContain('Test-StaticEnv');
  });

  // F11（2026-09-14 Q-2 修正）：quick 的编译诊断由**显式开关**开启（Q-B），是默认行为之外的选择。
  test('F11: 【Q-B】-Compile switch exists and gates the quick compile', () => {
    expect(src).toMatch(/\[switch\]\$Compile\b/);
    expect(src).toContain('QuickCompile:$Compile');
    // $needsHx 必须把「quick + -Compile」算作需要 HBuilderX
    expect(src).toMatch(/\$needsHx\s*=\s*\(\$detectedLevel -ne 'quick'\)\s*-or\s*\$Compile/);
  });
});

// ============================================================
// S 系列（#1526，2026-10-04）：`New-TestFailureSummary` 的**行为级**守护
//
// 为什么是行为守护而不是文本断言（本仓口径：docs/agents/guards.md）：
//   本票要守的是「打开那个文件看得见什么」—— 失败用例名、`Tests:` / `Time:` 行、≤2 KB 的承诺、
//   绿跑不产。这四件事**全都不体现在源码字面量上**：把 `^\s*Tests:` 那条 break 删掉、
//   把预算判断写反、把「绿跑直接返回」写成「照样落盘」，源码里该有的字一个都没少，
//   产物却已经不是那件东西。⇒ 只能**真执行它、真读它写出来的文件**。
//
// 怎么执行（先例：scripts/lib/process-capture.ps1 + utils/hxLaunchDetachBehavior.test.js）：
//   用 AST 把 `scripts/dev-finish.ps1` 里的**函数定义**抽出来 `Invoke-Expression` 进一个临时
//   驱动脚本，然后**直接调** `New-TestFailureSummary`。
//   ⚠️ 为什么不能像 capabilitySurfaceBehavior 的 H7 那样「跑真脚本」：H7 走的是 `-DryRun`，
//   它在单元测试那一步**之前**就 `exit 0`。真跑到那一步就会 `npx jest --testPathPattern <真源 pattern>`，
//   而 pattern 里有 `devFinish` ⇒ 嵌套 jest 会再跑**本文件** ⇒ 本守护再 spawn 一次 dev-finish ⇒
//   自递归直到超时。这不是「偷懒用 AST」，是这条路径**结构上不可用作守护**：
//   调用点（`exit 1` 之前落盘、`Write-Host` 出路径、绿跑不产）的端到端验收由本票的**成对人工取证**
//   承担（必红一次真跑到退出码 1 并留下摘要文件；必绿一次证明不落盘），见 PR 正文。
//
// 判别力：**逐条改坏被测物、跑本套件取红**，2026-10-04 三批实测（每批做完即还原，还原前后 sha256
// 逐字一致：一、二批针对去重前的文件形态，BASE=65E401B6…7206；第三批针对去重后的形态，
// BASE=0FD358B8…67F9；取证脚本 `.scratch/1526/mutation.ps1` / `mutation2.ps1` / `mutation3.ps1`）。
// 下面每行左边是被改坏的那一处，右边是**实际报红的用例**（不是推演）：
//   去掉 `$ExitCode -eq 0` 的提前返回        ⇒ S1 红（绿跑产出了文件）
//   删掉汇总行的那条 break                   ⇒ S2 红（`Tests:` 在文件里出现两次）
//   字节预算的 while 写死成假                 ⇒ S3 红（超预算且不吭声）
//   去掉「丢了多少条」的说明                  ⇒ S3 红（静默截断）
//   把 Missing 的第三项登记成空串             ⇒ S4 红（缺了什么不再有名有姓）
//   去掉超预算后的硬裁剪兜底                  ⇒ S5 红（预算承诺被破）
//   汇总段整段不产出                          ⇒ S2 / S3 / S6 同时红（总量面在三处被断言）
//   把汇总段挪到用例段之后（内容一字不动）      ⇒ **只有 S2 红**（顺序断言在 S2；S6 不红 ——
//     条目裁剪循环丢的是套件与用例，汇总段不在可丢集合里，所以「挪动」动不到 S6 的判据；
//     第三批随 assemble 结构改动**重跑过**这条，读数不变）
//   失败套件行不做去重                        ⇒ **只有 S7 红**（写成「共 2 个失败套件」）
//   失败用例条目不做去重                      ⇒ **只有 S7 红**（同块两份 ⇒ 5 条 vs `Tests: 2 failed`）
//   `● Console` 不分流、算进失败用例           ⇒ **只有 S7 红**（noteCount 归零、caseCount 变 3）
//   ⚠️ 一批里有一条**改不动语义**的变异（把 break 换成 continue）跑成全绿：夹具中汇总块之后
//     只剩空行，两种写法的采集结果相同 ⇒ 那是**等价变异**，不能算「守护漏了」，
//     但也说明「全绿」本身不证明变异生效 —— 所以每条都按名字取了红。
//   ⚠️ 反向的坑（改**脚本**时踩到，2026-10-04）：`dev-finish.ps1` 里任何注释写了「Step + 步骤号」
//     那个字面量，都会让**同文件的 F2**（判 `Step N` 串首次出现位置单调递增）报红，红因是
//     「步骤顺序错了」而实际只动了注释。⇒ 指代步骤请写步骤名（「单元测试」），别写字面量。
//
// 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**。
// ============================================================

/** 字形用字符码拼，不放裸字形进源码（编辑链会吃掉；与脚本里 `\uXXXX` 同一条纪律）。 */
const BULLET = String.fromCharCode(0x25cf); // jest 默认 reporter 的失败用例标记
const VERBOSE_X = String.fromCharCode(0x2715); // verbose reporter 的失败标记
const SUIT = String.fromCharCode(0x203a); // 套件名与用例名之间的分隔符

const CASE_ZH = `签到日历契约 ${SUIT} 中文用例名：月末格子应落在本周`;
const SUITE_FAIL_LINE = 'FAIL utils/checkinCalendar.test.js (8.405 s)';
// 第七个场景（dupAndConsole）里被**重复两次**的那条 FAIL 行；提到模块作用域是让 S7 能拿同一串去数。
const DUP_FAIL_LINE = 'FAIL utils/contractTestPatternBehavior.test.js (6.565 s)';
const DUP_CASE_ONE = `Q-A 契约测试 pattern 唯一真源 ${SUIT} P1: 逐字一致`;
const TESTS_LINE = 'Tests:       22 failed, 979 passed, 1001 total';
const TIME_LINE = 'Time:        140.697 s';

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 七个场景的输入。**全部喂合成输出**（ADR-0019 §⑤：判据不读工作树真源），但形状取自
 *  真跑一次 jest 抓到的输出（`.ci-verify/unit-*.log`）：`FAIL <路径>` / `  ● <套件> › <用例>` /
 *  `Test Suites:` / `Tests:` / `Time:`，且实测**无 ANSI 转义**（非 TTY 下 jest 不上色）。
 *  第七个（`dupAndConsole`）的形状不是编的：它是 #1526 真链路必红那次抓到的输出形态 ——
 *  同一份输出里**一个套件的整块详情出现两遍**（`.scratch/1526/full-raw.txt` 第 15 与 189 行是同一条
 *  `FAIL` 行），而 `Tests: 2 failed` 只有一遍 ⇒ 去重前摘要会写「共 2 个失败套件 / 5 条失败用例」，
 *  同场还混进一条 `● Console`（来自**通过**的套件的日志块），去重/分流前被当成失败用例计数。 */
function scenarios(dir) {
  const failOutput = [
    SUITE_FAIL_LINE,
    `${BULLET} ${CASE_ZH}`,
    '      expect(received).toBe(expected) // Object.is equality',
    '      Expected: 31',
    `${BULLET} 另一个套件 ${SUIT} 另一条用例`,
    '',
    '      Expected: true',
    `${VERBOSE_X} 仅 verbose 形态的用例名`,
    'Test Suites: 1 failed, 96 passed, 97 total',
    TESTS_LINE,
    TIME_LINE,
  ].join('\r\n');

  const manyCases = [];
  for (let i = 1; i <= 60; i++) {
    manyCases.push(`${BULLET} 套件${i} ${SUIT} 第 ${i} 条失败用例，名字刻意取得很长以便把预算撑爆，看是否从尾部丢弃并明说`);
  }
  manyCases.push(TESTS_LINE, TIME_LINE);

  const manySuites = [];
  for (let i = 1; i <= 97; i++) {
    manySuites.push(`FAIL utils/someQuiteLongSuiteName${i}.test.js (${i}.${i} s)`);
  }
  manySuites.push('Test Suites: 97 failed, 97 total', TESTS_LINE, TIME_LINE);

  const oneCase = [`${BULLET} 某套件 ${SUIT} 某用例`, TESTS_LINE, TIME_LINE].join('\r\n');

  // 真链路抓到的形态：FAIL 行 + 一条 `● Console`（来自别的**通过**套件）+ 两条失败用例块，
  // 然后整块（FAIL 行与两个用例块，**不含** Console）再来一遍，最后汇总三行只有一遍。
  const dupFailLine = DUP_FAIL_LINE;
  const dupConsoleBlock = [
    `${BULLET} Console`,
    '      console.warn',
    '      [refreshGate] refresh runner failed: Error: boom',
  ];
  const dupCaseOne = [
    `${BULLET} ${DUP_CASE_ONE}`,
    '      expect(received).toBe(expected) // Object.is equality',
    '      Expected: "levelDetect|envCheck"',
  ];
  const dupCaseTwo = [
    `${BULLET} Q-A 契约测试 pattern 唯一真源 ${SUIT} P2: 无空格`,
    '      - Expected  - 1',
  ];
  const dupAndConsole = [
    dupFailLine, ...dupConsoleBlock, ...dupCaseOne, ...dupCaseTwo,
    dupFailLine, ...dupCaseOne, ...dupCaseTwo,
    'Test Suites: 1 failed, 31 passed, 32 total',
    TESTS_LINE,
    TIME_LINE,
  ].join('\r\n');

  const spec = {
    green: { output: failOutput, exitCode: 0, maxBytes: 2048 },
    fail: { output: failOutput, exitCode: 1, maxBytes: 2048 },
    manyCases: { output: manyCases.join('\r\n'), exitCode: 1, maxBytes: 2048 },
    manySuites: { output: manySuites.join('\r\n'), exitCode: 1, maxBytes: 2048 },
    noStructure: { output: `E${'x'.repeat(6000)}`, exitCode: 1, maxBytes: 2048 },
    tinyBudget: { output: oneCase, exitCode: 1, maxBytes: 160 },
    dupAndConsole: { output: dupAndConsole, exitCode: 1, maxBytes: 2048 },
  };
  for (const [name, sc] of Object.entries(spec)) {
    sc.path = path.join(dir, `${name}.txt`); // eslint-disable-line no-param-reassign
  }
  // 绿跑那条**故意**放进一个不存在的子目录：这样「不碰文件系统」是可断言的事实
  // （父目录有没有被建出来），而不是只靠「文件不在」这种目录本来就在的弱断言。
  spec.green.path = path.join(dir, 'green-dir-should-not-exist', 'green.txt');
  return spec;
}

/**
 * 一次性真跑七个场景：AST 抽函数定义 ⇒ 调 `New-TestFailureSummary` ⇒ 把返回对象**与落盘文件内容**
 * 写成 JSON 交给 node 断言。
 * 为什么一趟跑完而不是每条用例起一个 pwsh：③ 门是 `jest -i` 串行全量，进程启动成本按套件计更划算，
 * 而七个场景共享同一份 AST 抽取 ⇒ 判别力不变、白等的时间省掉。
 */
function runSummaryScenarios() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'devfinish-1526-'));
  const inputPath = path.join(dir, 'input.json');
  const resultPath = path.join(dir, 'result.json');
  const harness = path.join(dir, 'harness.ps1');
  fs.writeFileSync(inputPath, JSON.stringify({
    script: path.join(ROOT, SCRIPT_REL),
    cases: scenarios(dir),
  }), 'utf8');

  const body = [
    '$ErrorActionPreference = \'Stop\'',
    'Set-StrictMode -Version Latest',
    `$spec = [System.IO.File]::ReadAllText('${inputPath}', [System.Text.Encoding]::UTF8) | ConvertFrom-Json`,
    '$src = [System.IO.File]::ReadAllText($spec.script, [System.Text.Encoding]::UTF8)',
    '$tokens = $null',
    '$errors = $null',
    '$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$tokens, [ref]$errors)',
    'if (@($errors).Count -gt 0) { throw (\'dev-finish.ps1 解析失败（行号）：\' + ((@($errors) | ForEach-Object { $_.Extent.StartLineNumber }) -join \',\')) }',
    '$fns = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)',
    'if (@($fns | Where-Object { $_.Name -eq \'New-TestFailureSummary\' }).Count -ne 1) { throw \'New-TestFailureSummary 没有被抽到（被改名 / 被拆走 / 定义形态变了）\' }',
    'foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }',
    '$out = [ordered]@{}',
    'foreach ($p in $spec.cases.PSObject.Properties) {',
    '  $sc = $p.Value',
    '  $r = $null',
    '  $thrown = $null',
    '  try {',
    '    $r = New-TestFailureSummary -Output $sc.output -Path $sc.path -ExitCode ([int]$sc.exitCode) -MaxBytes ([int]$sc.maxBytes)',
    '  } catch {',
    '    $thrown = $_.Exception.Message',
    '  }',
    '  $exists = Test-Path -LiteralPath $sc.path',
    '  $bytes = -1',
    '  $content = \'\'',
    '  if ($exists) { $bytes = ([System.IO.File]::ReadAllBytes($sc.path)).Length; $content = [System.IO.File]::ReadAllText($sc.path, [System.Text.Encoding]::UTF8) }',
    '  $out[$p.Name] = @{',
    '    thrown = $thrown',
    '    isNull = ($null -eq $r)',
    '    exists = $exists',
    '    fileBytes = $bytes',
    '    content = $content',
    '    bytes = $(if ($null -ne $r) { $r.Bytes } else { -1 })',
    '    truncated = $(if ($null -ne $r) { $r.Truncated } else { $false })',
    '    suiteCount = $(if ($null -ne $r) { $r.SuiteCount } else { -1 })',
    '    caseCount = $(if ($null -ne $r) { $r.CaseCount } else { -1 })',
    '    noteCount = $(if ($null -ne $r) { $r.NoteCount } else { -1 })',
    '    keptSuites = $(if ($null -ne $r) { $r.KeptSuites } else { -1 })',
    '    keptCases = $(if ($null -ne $r) { $r.KeptCases } else { -1 })',
    '    keptNotes = $(if ($null -ne $r) { $r.KeptNotes } else { -1 })',
    '    droppedSuites = $(if ($null -ne $r) { $r.DroppedSuites } else { -1 })',
    '    droppedCases = $(if ($null -ne $r) { $r.DroppedCases } else { -1 })',
    // 去重计数的**取值面**要在守护里可读：产品缺陷（同一 FAIL 块被算两遍）只有靠这两个数才能
    // 和「摘要写了几套几例」区分开 —— 只断言最终数字，就把「去重生效」和「夹具本来就只有一份」
    // 混成同一个读数了（S7 的判别力依赖这两项）。
    '    dupSuites = $(if ($null -ne $r) { $r.DupSuites } else { -1 })',
    '    dupEntries = $(if ($null -ne $r) { $r.DupEntries } else { -1 })',
    // Missing 走 `|` 连接的**字符串**而不是 JSON 数组：PowerShell 的 `ConvertTo-Json` 会把空数组
    // 序列化成 `null`（实测踩过 —— 于是断言 `toEqual([])` 对着 `null` 红，而红的是序列化形状、
    // 不是被测函数）。判据面要判「缺了哪几项」，就不该让判据依赖 JSON 数组形状这一层。
    '    missing = $(if ($null -ne $r) { (@($r.Missing) -join \'|\') } else { \'\' })',
    '  }',
    '}',
    `$json = ConvertTo-Json -InputObject $out -Depth 6 -Compress`,
    `[System.IO.File]::WriteAllText('${resultPath}', $json, [System.Text.UTF8Encoding]::new($false))`,
    '$out.Keys -join \',\'',
  ].join('\n');
  fs.writeFileSync(harness, body, 'utf8');

  try {
    execFileSync(powershellExe(), psArgs(['-File', harness]), {
      cwd: ROOT,
      timeout: 180000,
      windowsHide: true,
      maxBuffer: 32 * 1024 * 1024,
    });
  } catch (e) {
    // fail-closed：pwsh 不可用 / 驱动脚本抛错都不能 skip（skip = 本守护在 CI 上永不执行）
    throw new Error(
      'New-TestFailureSummary 驱动脚本跑不起来（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
      + `status=${e.status === undefined ? 'n/a' : e.status}\n`
      + `stdout=${String(e.stdout || '')}\nstderr=${String(e.stderr || e.message || '')}`
    );
  }
  if (!fs.existsSync(resultPath)) {
    throw new Error(`驱动脚本跑完了但没有产出 ${resultPath}（fail-closed：判据面不存在就等于没判）`);
  }
  const parsed = JSON.parse(readText(resultPath));
  return { dir, parsed };
}

describe('New-TestFailureSummary 失败摘要（#1526，行为级：真执行被测函数、断言它写出的文件）', () => {
  let ctx;

  beforeAll(() => {
    ctx = runSummaryScenarios();
  }, 200000);

  afterAll(() => {
    // 常驻/句柄未释放时 rmSync 会 EPERM —— 这是测试自身的收尾问题，不该把用例判红（先例同文件）
    if (!ctx) return;
    try {
      fs.rmSync(ctx.dir, { recursive: true, force: true });
    } catch {
      // 留给 OS 临时目录清理
    }
  });

  test('S1: 绿跑不落盘（exit 0 ⇒ 返回空且**不碰文件系统**，连父目录都不建）', () => {
    const g = ctx.parsed.green;
    expect(g.thrown).toBe(null);
    expect(g.isNull).toBe(true);
    // 绿跑产文件 = 造一份「看起来像证据」的镜像，正是本仓防的形态
    expect(g.exists).toBe(false);
    expect(g.fileBytes).toBe(-1);
    // 父目录也没被建出来（`New-Item -Force` 只在真要落盘的那条路径上）
    expect(fs.existsSync(path.join(ctx.dir, 'green-dir-should-not-exist'))).toBe(false);
  });

  test('S2: 失败摘要里真有「哪个套件 / 哪条用例 / 为什么红」+ Tests: 行 + Time: 行', () => {
    const f = ctx.parsed.fail;
    expect(f.thrown).toBe(null);
    expect(f.isNull).toBe(false);
    expect(f.exists).toBe(true);
    expect(f.content).toContain(CASE_ZH); // 中文用例名整条活着（裁剪/编码链没把它变成乱码）
    // `›`（U+203A）是这条链上最容易被换码的一个字，单独钉一下**函数本体**的读写面：
    // 夹具从 node 写成 UTF-8 JSON ⇒ pwsh 按 UTF-8 读 ⇒ 函数按 UTF-8 无 BOM 写文件，三段里
    // 任何一段按本机代码页（gb2312）走，这里拿到的就是形近字而不是 `›`。
    // ⚠️ 这一条**守不到**脚本顶部那行 `[Console]::OutputEncoding` —— 它守的是**子进程输出进内存**
    // 那一刻的解码，而本守护是 AST 抽函数、输入来自 JSON 文件，不经过控制台码页。那一半由
    // 本票的**真链路人工取证**兜（真跑一次 dev-finish 到失败分支，摘要文件里的 `›` 与中文必须完好，
    // 产物随 PR 附上）；写在这里当已守 = 把没判的东西报成判了。
    expect(f.content).toContain(SUIT);
    expect(f.content).toContain(SUITE_FAIL_LINE);
    expect(f.content).toContain('Expected: 31'); // 「为什么红」那一层
    expect(f.content).toContain(TESTS_LINE);
    expect(f.content).toContain(TIME_LINE);
    // 汇总行**只能出现一次**：判据行的采集若把汇总块也吃掉，这里就是两次（S2 的变异面）
    expect((f.content.match(/Tests:/g) || []).length).toBe(1);
    expect(f.suiteCount).toBe(1);
    expect(f.caseCount).toBe(3); // 默认 reporter 两条 + verbose 一条
    expect(f.missing).toBe('');
    // 段落顺序是**判据**不是排版，两条机制各管一头：①条目裁剪循环（超预算时先丢用例、再丢套件，
    // 汇总段不在可丢集合里 ⇒ S6 用 97 个套件的洪水验这一端）；②硬裁剪兜底只保头不保尾
    // （汇总排在最前 ⇒ `Tests:` / `Time:` 不会被字节裁剪砍掉，S5 验这一端）。
    // 变异实测：把汇总段挪到用例段之后（内容一字不动）⇒ 本条红；S6 不红（机制①压根不看顺序）。
    expect(f.content.indexOf('汇总：')).toBeLessThan(f.content.indexOf('失败套件'));
    expect(f.content.indexOf('失败套件')).toBeLessThan(f.content.indexOf('失败用例'));
    // 落盘字节数 = 返回对象报的字节数（无 BOM、无编码链加的字）
    expect(f.bytes).toBe(f.fileBytes);
    expect(f.bytes).toBeLessThanOrEqual(2048);
  });

  test('S3: 60 条长用例名 ⇒ 仍 ≤2 KB，且**明说**丢了多少条（静默截断＝换一种看不见）', () => {
    const m = ctx.parsed.manyCases;
    expect(m.caseCount).toBe(60);
    expect(m.bytes).toBeLessThanOrEqual(2048);
    expect(m.bytes).toBe(m.fileBytes);
    expect(m.keptCases).toBeLessThan(m.caseCount);
    expect(m.droppedCases).toBe(m.caseCount - m.keptCases);
    expect(m.truncated).toBe(false); // 是靠整条丢弃收敛的，不是硬裁字符
    expect(m.content).toContain('未列出');
    expect(m.content).toContain(`共 ${m.caseCount} 条`);
    // 被丢弃不能把总量面一起丢掉：Tests:/Time: 必须还在
    expect(m.content).toContain(TESTS_LINE);
    expect(m.content).toContain(TIME_LINE);
  });

  test('S4: 输出里没有可抓的结构 ⇒ 文件里就写着「没抓到什么」，不许静默产出一份空表头', () => {
    const n = ctx.parsed.noStructure;
    expect(n.exists).toBe(true);
    expect(n.suiteCount).toBe(0);
    expect(n.caseCount).toBe(0);
    expect(n.missing.split('|')).toEqual(['Tests 汇总行', 'Time 汇总行', '失败套件 / 失败用例名']);
    expect(n.content).toContain('警告');
    expect(n.content).toContain('没抓到');
  });

  test('S5: 预算小到连表头都装不下 ⇒ 硬裁剪保头不保尾，≤N 字节是承诺不是目标', () => {
    const t = ctx.parsed.tinyBudget;
    expect(t.exists).toBe(true);
    expect(t.truncated).toBe(true);
    expect(t.bytes).toBeLessThanOrEqual(160);
    expect(t.bytes).toBe(t.fileBytes);
    // 砍的是尾部：「这份文件在说什么」的表头必须还在，砍完剩一串用例碎片就没意义了
    expect(t.content).toContain('dev-finish 单元测试失败摘要');
  });

  test('S6: 97 个套件一起红（FAIL 行就 ≈4 KB）⇒ 汇总段不在可丢集合里，Tests:/Time: 必须活着', () => {
    const s = ctx.parsed.manySuites;
    expect(s.suiteCount).toBe(97);
    expect(s.bytes).toBeLessThanOrEqual(2048);
    expect(s.keptSuites).toBeLessThan(s.suiteCount);
    expect(s.content).toContain(TESTS_LINE);
    expect(s.content).toContain(TIME_LINE);
  });

  // S7：真链路抓到的那份输出里**同一个套件整块出现两遍**，还混着一条来自通过套件的 `● Console`。
  // 这一条守的是摘要的**计数诚实**：写出来的「共几个套件 / 几条用例」必须对得上 `Tests:` 行说的量，
  // 且 Console 不能被当失败用例 —— 二者都是 #1526 真链路必红那次的**实测缺陷**（去重前该场景报
  // 「共 2 个失败套件 / 5 条失败用例」，而同一份输出的 `Tests: 2 failed` 说只有两条）。
  test('S7: 同一块详情出现两遍 + 一条 Console ⇒ 套件/用例各计一次，Console 单列不进用例段', () => {
    const d = ctx.parsed.dupAndConsole;
    expect(d.thrown).toBe(null);
    expect(d.exists).toBe(true);
    // 采集面：重复的那一份要**被看见**（计数为 1），否则「去重生效」与「夹具只有一份」读不出区别
    expect(d.suiteCount).toBe(1);
    expect(d.dupSuites).toBe(1);
    expect(d.caseCount).toBe(2);
    expect(d.dupEntries).toBe(2);
    // Console 归到「套件级说明」，不占用例名额
    expect(d.noteCount).toBe(1);
    expect(d.keptNotes).toBe(1);
    // 产物面：整份文件里 FAIL 行与每条判据行都只剩一遍
    expect((d.content.match(/FAIL utils\/contractTestPatternBehavior\.test\.js/g) || []).length).toBe(1);
    expect((d.content.match(/Tests:/g) || []).length).toBe(1);
    expect((d.content.match(/levelDetect\|envCheck/g) || []).length).toBe(1);
    expect(d.content).toContain(DUP_CASE_ONE);
    // 写给人看的总量措辞用的是**去重后**的数
    expect(d.content).toContain('共 1 个');
    expect(d.content).toContain('共 2 条');
    // Console 那条的**说明行**还在（不是只剩个标题），且它排在用例段之后 ⇒ 用例段里没有它
    expect(d.content).toContain('[refreshGate] refresh runner failed: Error: boom');
    expect(d.content.indexOf('套件级说明')).toBeGreaterThan(d.content.indexOf('失败用例'));
    expect(d.content.indexOf('[refreshGate]')).toBeGreaterThan(d.content.indexOf('套件级说明'));
    expect(d.content.indexOf('套件级说明')).toBeGreaterThan(d.content.indexOf('失败套件'));
    expect(d.truncated).toBe(false);
    expect(d.bytes).toBe(d.fileBytes);
    expect(d.bytes).toBeLessThanOrEqual(2048);
    expect(d.missing).toBe('');
  });
});

// ============================================================
// U 系列：跑**全量单测门**那一刻在磁盘上留下什么（#1546，2026-10-05）
//
// 被测物 = `Write-UnitGateArtifacts`（`scripts/dev-finish.ps1`）。它存在的理由：#1526 只守住了
// 「单元测试（契约子集）」那一步，而全量那一条（`npm run test:unit`）至今没有落盘脚本 —— 跑完只剩
// 一个 exit code ⇒ 「刚才哪个红了」只能再花几分钟重跑（#1545 读数里的 C1 那一档）。
//
// 为什么只能真执行（与 S 系列同一条理由，且更强）：本票的四条口径**全部不体现在源码字面量上** ——
//   · 绿跑只留日志、不留摘要；
//   · 日志落的是**原文**而不是内容 hash（#1522 那条「同 sha 只留一份日志」的候选已撤销并在册）；
//   · 机检行里的数字能从日志复算；
//   · 退出码原样透传，不夹紧也不改写。
// 把 `Select-Object -Last 1` 改成 `-First 1`、把 `exit=$ExitCode` 写成 `exit=1`、把日志内容换成
// `$Output.Length` —— 源码该有的字一个不少，产物已经不是那件东西。
//
// ⚠️ 不能像 ②④ 那样「跑真脚本」取这个证据：`-UnitGate` 分支跑的就是 `npm run test:unit`，
//    而本文件在它的射程里 ⇒ 嵌套 jest 会自递归（同文件 S 系列头部记过的同一个坑）。
//    执行面（`npm run test:unit` 这一条命令、`exit $art.ExitCode`、`-PostToPr` 的位置）由 U8 作
//    **接线**断言钉住，端到端由本票的**真链路成对取证**承担（绿跑一次、红跑一次，产物随 PR 附上）。
//
// 判别力：**逐条改坏被测物、跑本套件取红**，2026-10-05 实测九条全部报红，每条做完即还原，
// 还原后 sha256(dev-finish.ps1) 与基线逐字一致（基线 656E58A6…753E；脚本 `.scratch/1546/mutation.js`，
// 它按 `jest --json` 取**用例名**而不是按人眼读终端 —— 终端码页会把 `›` 与中文打成形近字）。
// 左边是被改坏的那一处，右边是**实际报红的用例**（不是推演）：
//   取第一条汇总而不是最后一条            ⇒ U7 红（把 `● Console` 里打印出来的噪声读成总量）
//   机检行把退出码写死成 1               ⇒ **U1 + U4** 红（绿跑被写成 exit=1；127 被夹成 1）
//   读不出汇总行时静默补 0               ⇒ U3 红（「读不出」伪装成「没红」）
//   日志只留体量不留原文（内容折叠）      ⇒ **U1 + U2 + U5** 红（事后复核没有物证；复算无从下手）
//   绿跑也产摘要                        ⇒ U1 红（多出一份长得像证据的镜像文件，连目录都被建出来）
//   摘要写失败时不吭声                   ⇒ U6 红（人读出口坏了却静默）
//   机检行不引用日志路径                 ⇒ **U4 + U5** 红（结论行指向虚无）
//   调用点自己抄第二份门参数              ⇒ U8 红（「门」与「被落盘的那一次」分家，#974 同形）
//   分支末尾改成无条件 exit 0            ⇒ U8 红（造出第二个绿面）
// ⚠️ U8 是**接线**断言，且它对「分支的形状」敏感（右花括号、缩进、语句顺序）—— 三条不变式里
//    只有「跑的是那一条命令」与「贴评论在 exit 之前」有真链路取证兜行为，见本票 PR 正文。
// ============================================================

const GATE_GREEN_OUTPUT = [
  '> forklift-training-app@1.0.0 test:unit',
  '> jest --config jest.config.unit.js -i',
  '',
  'PASS utils/format.test.js',
  'PASS utils/checkinCalendar.test.js',
  '',
  'Test Suites: 158 passed, 158 total',
  'Tests:       3000 passed, 3000 total',
  'Snapshots:   0 total',
  'Time:        214.365 s',
  'Ran all test suites.',
].join('\r\n');

const GATE_RED_OUTPUT = [
  '> forklift-training-app@1.0.0 test:unit',
  '> jest --config jest.config.unit.js -i',
  '',
  'PASS utils/format.test.js',
  SUITE_FAIL_LINE,
  `${BULLET} ${CASE_ZH}`,
  '',
  '      expect(received).toBe(expected) // Object.is equality',
  '',
  '      Expected: 31',
  '      Received: 30',
  '',
  'Test Suites: 1 failed, 157 passed, 158 total',
  'Tests:       2 failed, 2998 passed, 3000 total',
  'Snapshots:   0 total',
  'Time:        219.884 s',
  'Ran all test suites.',
].join('\r\n');

// 「输出里引用过一份汇总行」的形态：某条用例把 jest 自己的汇总行**打印**出来了（`● Console` 里）。
// 取**最后**一条才是 reporter 收尾那一份；取第一条就会把这条噪声读成总量（U7 的变异面）。
const GATE_QUOTED_SUMMARY_OUTPUT = [
  SUITE_FAIL_LINE,
  `${BULLET} Console`,
  '      console.log',
  '        Test Suites: 3 failed, 3 total',
  '        Tests:       9 failed, 12 passed, 21 total',
  '        Time:        9.000 s',
  `${BULLET} ${CASE_ZH}`,
  '      Expected: 31',
  'Test Suites: 1 failed, 157 passed, 158 total',
  'Tests:       2 failed, 2998 passed, 3000 total',
  'Time:        219.884 s',
].join('\r\n');

// 完全没有可解析结构的一跑（npm 自己炸了 / jest 没起来）：判定照旧走退出码，但机检行**必须点名**
// 读不出来的是哪几项 —— 补 0 等于把「读不出」伪装成「没红」。
const GATE_NOISE_OUTPUT = [
  'npm ERR! code EJSONPARSE',
  'npm ERR! While resolving: forklift-training-app@1.0.0',
  'npm ERR! command failed',
].join('\r\n');

// jest 的 `Time:` 行**带预估尾巴**的那一形（2026-10-05 真链路抓到：一次 287.673 s 的绿跑挂着
// `estimated 814 s`，因为上一跑真的跑了 813.602 s）。预估不是本次耗时，也不该挤进一个 `key=value`
// 的机检 token —— U10 判的就是「只取测量值本体，且取出来的仍是日志那一行的前缀」。
const GATE_ESTIMATED_TIME_OUTPUT = [
  '> forklift-training-app@1.0.0 test:unit',
  '> jest --config jest.config.unit.js -i',
  '',
  'PASS utils/format.test.js',
  '',
  'Test Suites: 158 passed, 158 total',
  'Tests:       3017 passed, 3017 total',
  'Snapshots:   0 total',
  'Time:        287.673 s, estimated 814 s',
  'Ran all test suites.',
].join('\r\n');

/** 八个场景。形状取自真跑一次 `npm run test:unit` 的输出（`.ci-verify/test-unit.log`），
 *  但**全部喂合成输出**（ADR-0019 §⑤：判据不读工作树真源）。 */
function gateScenarios(dir) {
  const refLog = '.ci-verify/test-unit.log';
  const refSummary = '.ci-verify/test-unit-failure.txt';
  const spec = {
    green: { output: GATE_GREEN_OUTPUT, exitCode: 0, logRef: refLog, summaryRef: refSummary, durationSeconds: 222 },
    red: { output: GATE_RED_OUTPUT, exitCode: 1, logRef: refLog, summaryRef: refSummary, durationSeconds: 231 },
    // 没给 ref ⇒ 机检行退回绝对路径（终端里那一行要能直接打开）；退出码刻意取 127（不夹紧的变异面）
    noRefs: { output: GATE_RED_OUTPUT, exitCode: 127, logRef: '', summaryRef: '', durationSeconds: 0 },
    noise: { output: GATE_NOISE_OUTPUT, exitCode: 1, logRef: refLog, summaryRef: refSummary, durationSeconds: 3 },
    quotedSummary: { output: GATE_QUOTED_SUMMARY_OUTPUT, exitCode: 1, logRef: refLog, summaryRef: refSummary, durationSeconds: 4 },
    blockedSummary: { output: GATE_RED_OUTPUT, exitCode: 1, logRef: refLog, summaryRef: refSummary, durationSeconds: 5 },
    estimatedTime: { output: GATE_ESTIMATED_TIME_OUTPUT, exitCode: 0, logRef: refLog, summaryRef: refSummary, durationSeconds: 291 },
    // 绿跑，但磁盘上还留着**上一跑**的红摘要（路径固定 ⇒ 绿跑不会覆盖它）。U9 判「指名而不装看不见」。
    greenWithStaleSummary: { output: GATE_GREEN_OUTPUT, exitCode: 0, logRef: refLog, summaryRef: refSummary, durationSeconds: 6 },
  };
  for (const [name, sc] of Object.entries(spec)) {
    sc.logPath = path.join(dir, `${name}-test-unit.log`); // eslint-disable-line no-param-reassign
    sc.summaryPath = path.join(dir, `${name}-test-unit-failure.txt`); // eslint-disable-line no-param-reassign
  }
  // 绿跑的摘要路径**故意**放进不存在的子目录 ⇒ 「不碰文件系统」断言到目录面，而不是只靠「文件不在」
  // （S1 同法）。
  spec.green.summaryPath = path.join(dir, 'green-gate-dir-should-not-exist', 'test-unit-failure.txt');
  // 摘要写不进去的那一跑：摘要路径本身是一个**已经存在的目录** —— 两个平台上「往目录路径写文件」
  // 都必失败，不靠平台特性（拿「父目录是普通文件」当夹具在 Windows 上会被 New-Item 的 -Force 绕过去）。
  spec.blockedSummary.summaryPath = path.join(dir, 'blocked-summary-is-a-directory');
  return spec;
}

/**
 * 一次性真跑六个场景：AST 抽函数定义 ⇒ 调 `Write-UnitGateArtifacts` ⇒ 把返回对象**与两个产物的内容**
 * 写成 JSON 交给 node 断言。形态与 `runSummaryScenarios` 一致（一趟跑完共享同一份 AST 抽取）。
 */
function runUnitGateScenarios() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'devfinish-1546-'));
  fs.mkdirSync(path.join(dir, 'blocked-summary-is-a-directory'));
  const inputPath = path.join(dir, 'input.json');
  const resultPath = path.join(dir, 'result.json');
  const harness = path.join(dir, 'harness.ps1');
  const cases = gateScenarios(dir);
  // 给 U9 种一份**上一跑**的红摘要（内容刻意与本次无关）：绿跑既不该覆盖它、也不该装作没看见它
  fs.writeFileSync(cases.greenWithStaleSummary.summaryPath, '上一次红跑的摘要正文，本次是绿跑 ⇒ 不是本次结论\r\n', 'utf8');
  fs.writeFileSync(inputPath, JSON.stringify({
    script: path.join(ROOT, SCRIPT_REL),
    cases,
  }), 'utf8');

  const body = [
    '$ErrorActionPreference = \'Stop\'',
    'Set-StrictMode -Version Latest',
    `$spec = [System.IO.File]::ReadAllText('${inputPath}', [System.Text.Encoding]::UTF8) | ConvertFrom-Json`,
    '$src = [System.IO.File]::ReadAllText($spec.script, [System.Text.Encoding]::UTF8)',
    '$tokens = $null',
    '$errors = $null',
    '$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$tokens, [ref]$errors)',
    'if (@($errors).Count -gt 0) { throw (\'dev-finish.ps1 解析失败（行号）：\' + ((@($errors) | ForEach-Object { $_.Extent.StartLineNumber }) -join \',\')) }',
    '$fns = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)',
    'if (@($fns | Where-Object { $_.Name -eq \'Write-UnitGateArtifacts\' }).Count -ne 1) { throw \'Write-UnitGateArtifacts 没有被抽到（被改名 / 被拆进别的文件 / 定义形态变了）\' }',
    'if (@($fns | Where-Object { $_.Name -eq \'New-TestFailureSummary\' }).Count -ne 1) { throw \'New-TestFailureSummary 没有被一起抽到：全量门摘要复用不了 #1526 那份，落点变成第二真源\' }',
    'foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }',
    '$out = [ordered]@{}',
    'foreach ($p in $spec.cases.PSObject.Properties) {',
    '  $sc = $p.Value',
    '  $r = $null',
    '  $thrown = $null',
    '  try {',
    '    $r = Write-UnitGateArtifacts -Output $sc.output -LogPath $sc.logPath -SummaryPath $sc.summaryPath -ExitCode ([int]$sc.exitCode) -DurationSeconds ([int]$sc.durationSeconds) -LogRef $sc.logRef -SummaryRef $sc.summaryRef',
    '  } catch {',
    '    $thrown = $_.Exception.Message',
    '  }',
    '  $logExists = Test-Path -LiteralPath $sc.logPath',
    '  $logBytes = -1',
    '  $logContent = \'\'',
    '  if ($logExists) { $logBytes = ([System.IO.File]::ReadAllBytes($sc.logPath)).Length; $logContent = [System.IO.File]::ReadAllText($sc.logPath, [System.Text.Encoding]::UTF8) }',
    '  $summaryExists = Test-Path -LiteralPath $sc.summaryPath',
    '  $out[$p.Name] = @{',
    '    thrown = $thrown',
    '    resultLine = $(if ($null -ne $r) { $r.ResultLine } else { \'nosuch\' })',
    '    exitCodeEcho = $(if ($null -ne $r) { $r.ExitCode } else { -1 })',
    '    logExists = $logExists',
    '    logBytes = $logBytes',
    '    logContent = $logContent',
    '    summaryExists = $summaryExists',
    '    summaryPathEcho = $(if ($null -ne $r) { $r.SummaryPath } else { \'nosuch\' })',
    '    summaryBytes = $(if ($null -ne $r) { $r.SummaryBytes } else { -1 })',
    '    summaryError = $(if ($null -ne $r) { $r.SummaryError } else { \'nosuch\' })',
    '    staleSummary = $(if ($null -ne $r) { $r.StaleSummary } else { \'nosuch\' })',
    '    suitesTotal = $(if ($null -ne $r) { $r.SuitesTotal } else { \'nosuch\' })',
    '    suitesFailed = $(if ($null -ne $r) { $r.SuitesFailed } else { \'nosuch\' })',
    '    testsTotal = $(if ($null -ne $r) { $r.TestsTotal } else { \'nosuch\' })',
    '    testsFailed = $(if ($null -ne $r) { $r.TestsFailed } else { \'nosuch\' })',
    '    time = $(if ($null -ne $r) { $r.Time } else { \'nosuch\' })',
    // Missing 走 `|` 连接的字符串而不是 JSON 数组：同 S 系列记过的坑（空数组会被序列化成 null，
    // 于是断言对着 `null` 红，红的是序列化形状不是被测函数）。
    '    missing = $(if ($null -ne $r) { (@($r.Missing) -join \'|\') } else { \'nosuch\' })',
    '  }',
    '}',
    '$json = ConvertTo-Json -InputObject $out -Depth 6 -Compress',
    `[System.IO.File]::WriteAllText('${resultPath}', $json, [System.Text.UTF8Encoding]::new($false))`,
    '$out.Keys -join \',\'',
  ].join('\n');
  fs.writeFileSync(harness, body, 'utf8');

  try {
    execFileSync(powershellExe(), psArgs(['-File', harness]), {
      cwd: ROOT,
      timeout: 180000,
      windowsHide: true,
      maxBuffer: 32 * 1024 * 1024,
    });
  } catch (e) {
    throw new Error(
      'Write-UnitGateArtifacts 驱动脚本跑不起来（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
      + `status=${e.status === undefined ? 'n/a' : e.status}\n`
      + `stdout=${String(e.stdout || '')}\nstderr=${String(e.stderr || e.message || '')}`
    );
  }
  if (!fs.existsSync(resultPath)) {
    throw new Error(`驱动脚本跑完了但没有产出 ${resultPath}（fail-closed：判据面不存在就等于没判）`);
  }
  return { dir, parsed: JSON.parse(readText(resultPath)) };
}

/** 从日志**原文**里把三条汇总行重新数一遍 —— 「机检行可复算」的复算方就是这段：判据不靠把同一串
 *  数字抄第二遍，而是从产物里现取。取最后一条与函数本体同口径。 */
function recomputeFromLog(logContent) {
  const lines = String(logContent).split(/\r?\n/);
  const last = (re) => {
    for (let i = lines.length - 1; i >= 0; i--) if (re.test(lines[i])) return lines[i];
    return '';
  };
  const num = (line, label) => {
    const m = new RegExp(`(\\d+)\\s+${label}`).exec(line);
    return m ? Number(m[1]) : null;
  };
  const suiteLine = last(/^\s*Test Suites?:\s/);
  const testLine = last(/^\s*Tests:\s/);
  const timeLine = last(/^\s*Time:\s/);
  return {
    suitesTotal: suiteLine ? num(suiteLine, 'total') : null,
    suitesFailed: suiteLine ? num(suiteLine, 'failed') : null,
    testsTotal: testLine ? num(testLine, 'total') : null,
    testsFailed: testLine ? num(testLine, 'failed') : null,
    time: timeLine ? timeLine.replace(/^\s*Time:\s*/, '').trim().replace(/\s+/g, '') : '',
  };
}

describe('Write-UnitGateArtifacts 全量单测门落盘（#1546，行为级：真执行、断言它写出的两个文件）', () => {
  let ctx;

  beforeAll(() => {
    ctx = runUnitGateScenarios();
  }, 200000);

  afterAll(() => {
    if (!ctx) return;
    try {
      fs.rmSync(ctx.dir, { recursive: true, force: true });
    } catch {
      // 留给 OS 临时目录清理
    }
  });

  test('U1: 绿跑只留日志不留摘要（连摘要的父目录都不许被建出来），机检行的数字与日志汇总行一致', () => {
    const g = ctx.parsed.green;
    expect(g.thrown).toBe(null);
    expect(g.logExists).toBe(true);
    expect(g.summaryExists).toBe(false);
    expect(g.staleSummary).toBe(''); // 干净环境里的绿跑：无摘要、也无陈旧摘要可指名
    expect(fs.existsSync(path.join(ctx.dir, 'green-gate-dir-should-not-exist'))).toBe(false);
    // 绿跑的日志是「这道门真的跑过、结论确实是 0」的唯一物证 ⇒ 必须在，且是原文
    expect(g.logContent.startsWith(GATE_GREEN_OUTPUT)).toBe(true);
    expect(g.resultLine).toContain('TEST_UNIT_RESULT');
    expect(g.resultLine).toMatch(/\bexit=0\b/);
    expect(g.resultLine).toContain('summary=none');
    expect(g.resultLine).not.toContain('missing=');
    expect(g.suitesTotal).toBe(158);
    expect(g.testsTotal).toBe(3000);
    // jest **省略零值类别**（绿跑只打 `3000 passed, 3000 total`）⇒ 读成 0，不是 '?'、也不是猜
    expect(g.suitesFailed).toBe(0);
    expect(g.testsFailed).toBe(0);
    expect(g.time).toBe('214.365s');
    expect(g.resultLine).toContain('time=214.365s');
    expect(g.resultLine).toContain('wall=222s');
  });

  test('U2: 红跑留下日志 + 摘要两件产物，且机检行能从日志原文复算（票面「机检行可复算」）', () => {
    const r = ctx.parsed.red;
    expect(r.thrown).toBe(null);
    expect(r.logExists).toBe(true);
    expect(r.summaryExists).toBe(true);
    // 摘要复用 #1526 那一件产物 ⇒ 「哪个套件 / 哪条用例 / 为什么红」三件都在（不是又一个空壳文件）。
    // 读的是**函数自己报的那条路径**，不是测试按约定拼出来的第二条 —— 路径本身也在射程里。
    expect(typeof r.summaryPathEcho).toBe('string');
    expect(r.summaryPathEcho).not.toBe('');
    const summaryText = fs.readFileSync(r.summaryPathEcho, 'utf8');
    expect(summaryText).toContain(SUITE_FAIL_LINE);
    expect(summaryText).toContain(CASE_ZH);
    expect(summaryText).toContain('Expected: 31');
    // 复算：从日志里重新数一遍，与机检行报的逐项相等
    const back = recomputeFromLog(r.logContent);
    expect(r.resultLine).toContain(`suites=${back.suitesTotal}`);
    expect(r.resultLine).toContain(`suitesFailed=${back.suitesFailed}`);
    expect(r.resultLine).toContain(`tests=${back.testsTotal}`);
    expect(r.resultLine).toContain(`testsFailed=${back.testsFailed}`);
    expect(r.resultLine).toContain(`time=${back.time}`);
    expect(r.missing).toBe('');
    expect(back.suitesTotal).toBe(158);
    expect(back.testsFailed).toBe(2);
    // 机检行**追加在日志尾部**：只拿这一份文件就能同时读到完整输出与结论行
    expect(r.logContent).toContain(r.resultLine);
    expect(r.resultLine).toMatch(/\bexit=1\b/);
  });

  test('U3: 输出形态变了（没有汇总行）⇒ 机检行点名缺哪几项，不静默补 0', () => {
    const n = ctx.parsed.noise;
    expect(n.thrown).toBe(null);
    expect(n.logExists).toBe(true); // 越是读不出来越要有原文
    expect(n.suitesTotal).toBe(null);
    expect(n.testsTotal).toBe(null);
    expect(n.time).toBe('');
    expect(n.missing.split('|')).toEqual(['suites', 'tests', 'time']);
    expect(n.resultLine).toContain('suites=?');
    expect(n.resultLine).toContain('tests=?');
    expect(n.resultLine).toContain('time=?');
    expect(n.resultLine).toContain('missing=suites|tests|time');
  });

  test('U4: 退出码原样透传（不夹成 1、不改写成 0），且它来自被测物而不是第二处真源', () => {
    const w = ctx.parsed.noRefs;
    expect(w.thrown).toBe(null);
    expect(w.exitCodeEcho).toBe(127);
    expect(w.resultLine).toMatch(/\bexit=127\b/);
    expect(w.resultLine).not.toMatch(/\bexit=1\b/); // 127 没被夹成 1（第二个红面）
    expect(w.logExists).toBe(true);
    // 没给 ref ⇒ 退回绝对路径（终端里那一行要能直接打开）。两侧都归一分隔符再比：
    // Windows 上 PS 给 `\`、node 给 `\`，而断言串在两种平台上都得成立。
    const norm = (s) => String(s).replace(/\\/g, '/');
    expect(norm(w.resultLine)).toContain(`log=${norm(path.join(ctx.dir, 'noRefs-test-unit.log'))}`);
    expect(w.summaryExists).toBe(true);
    expect(norm(w.resultLine)).toContain(`summary=${norm(path.join(ctx.dir, 'noRefs-test-unit-failure.txt'))}`);
  });

  test('U5: 日志落的是原文，机检行引用的是**路径** —— 不被内容 hash 折叠（撤销在册的那一档）', () => {
    for (const key of ['green', 'red']) {
      const c = ctx.parsed[key];
      const fixture = key === 'green' ? GATE_GREEN_OUTPUT : GATE_RED_OUTPUT;
      // 折叠的形态有三种：整段换成摘要值 / 只留长度 / 只留 hash —— 逐条都判不过「原文在」这一关
      expect(c.logContent.startsWith(fixture)).toBe(true);
      expect(c.logBytes).toBeGreaterThanOrEqual(Buffer.byteLength(fixture, 'utf8'));
      expect(c.resultLine).not.toMatch(/\b(?:sha|hash|digest|md5|sha256)=/);
      expect(c.resultLine).not.toMatch(/\b[0-9a-f]{40}\b/);
      // 引用的是路径：绿跑也必须有 log=，否则「事后复核结论」没有物证可查
      expect(c.resultLine).toMatch(/\blog=\S+\.log\b/);
    }
  });

  test('U6: 摘要写不进去（摘要路径本身是个目录）⇒ 日志照写、机检行照出、summaryError 有名有姓，退出码不受影响', () => {
    const b = ctx.parsed.blockedSummary;
    // 人读出口坏了**不能**改门的结果：不外抛（否则调用点的 exit 都到不了），也不静默
    expect(b.thrown).toBe(null);
    expect(b.logExists).toBe(true);
    // 函数自己报「没有摘要产物」—— 而 `summaryExists` 这一跑是 **true**（那里种着一个目录）：
    // 两行合起来才证明「产物在不在」看的是函数写没写，不是路径存不存在。
    expect(b.summaryPathEcho).toBe('');
    expect(b.summaryBytes).toBe(0);
    expect(b.summaryExists).toBe(true);
    expect(b.exitCodeEcho).toBe(1);
    expect(b.resultLine).toContain('summary=none');
    expect(b.resultLine).toContain('summaryError=');
    expect(b.summaryError).not.toBe('');
    // 「写不进去」与「上一次红过」是两件事：这里不许顺带报出 staleSummary（那条路径刻意种成**目录**）
    expect(b.resultLine).not.toContain('staleSummary=');
    expect(b.staleSummary).toBe('');
  });

  test('U7: 输出里引用过一份汇总行 ⇒ 取 reporter 收尾那一份（最后一条），不读成噪声', () => {
    const q = ctx.parsed.quotedSummary;
    expect(q.thrown).toBe(null);
    expect(q.suitesTotal).toBe(158);
    expect(q.testsTotal).toBe(3000);
    expect(q.testsFailed).toBe(2);
    expect(q.time).toBe('219.884s');
    expect(q.resultLine).toContain('suites=158');
    expect(q.resultLine).not.toContain('suites=3');
    expect(q.resultLine).not.toContain('time=9.000s');
  });

  test('U9: 绿跑时磁盘上还留着上一跑的红摘要 ⇒ 机检行指名 staleSummary，且不删不改那份产物', () => {
    const s = ctx.parsed.greenWithStaleSummary;
    expect(s.thrown).toBe(null);
    expect(s.logExists).toBe(true);
    // 本次确实没产摘要（绿跑）—— 与「文件不存在」区分开：文件在，但它不是本次的
    expect(s.summaryPathEcho).toBe('');
    expect(s.resultLine).toContain('summary=none');
    expect(s.resultLine).toMatch(/\bstaleSummary=\d{8}-\d{6}\b/);
    expect(s.staleSummary).toMatch(/^\d{8}-\d{6}$/);
    // **不删**：`.ci-verify/` 里的东西在本仓是判据输入（删了不可复算），指名而不是销毁
    const kept = fs.readFileSync(path.join(ctx.dir, 'greenWithStaleSummary-test-unit-failure.txt'), 'utf8');
    expect(kept).toContain('上一次红跑的摘要正文');
    // 绿跑也不许覆盖它 —— 内容与种下去时逐字相同
    expect(kept.trim()).toBe('上一次红跑的摘要正文，本次是绿跑 ⇒ 不是本次结论');
  });

  test('U10: `Time:` 行挂着 jest 的预估尾巴 ⇒ time= 只取测量值本体，且它仍是日志那一行的前缀（可复算）', () => {
    const e = ctx.parsed.estimatedTime;
    expect(e.thrown).toBe(null);
    expect(e.time).toBe('287.673s');
    expect(e.resultLine).toContain('time=287.673s');
    // 预估是给下一跑看的，不是本次耗时 ⇒ 不许挤进 key=value 的机检 token
    expect(e.resultLine).not.toContain('estimated');
    // 复算口径仍然成立：机检行那串是日志里 `Time:` 行去掉标签后的**前缀**
    const back = recomputeFromLog(e.logContent);
    expect(back.time.startsWith(e.time)).toBe(true);
    expect(e.suitesTotal).toBe(158);
    expect(e.testsTotal).toBe(3017);
    expect(e.resultLine).toMatch(/\bexit=0\b/);
  });

  test('U8: 调用点是接线正确的 —— 跑的就是 `npm run test:unit` 那一条、判定只看退出码、贴评论在 exit 之前', () => {
    // 本条是**接线**断言（读源码文本），它守的三条不变式各由谁兜行为：
    //   · 「跑的是 package.json 那一条门命令、不抄第二份 jest 参数」⇒ 行为面由本票真链路取证兜
    //     （真跑一次 `-UnitGate`，日志里必须是全量 158 套件那一跑，不是契约子集那一跑）；
    //   · 「产物由 `Write-UnitGateArtifacts` 产出、退出码透传」⇒ U1–U7 真执行；
    //   · 「贴评论只在 -PostToPr 且发生在 exit 之前」⇒ 真链路取证（贴出的评论带 sha 绑定且门仍按码退出）。
    const scriptSrc = readSource();
    const branchStart = scriptSrc.indexOf('if ($UnitGate)');
    const step1At = scriptSrc.indexOf('# Step 1: 级别判定');
    expect(branchStart).toBeGreaterThan(-1);
    expect(step1At).toBeGreaterThan(branchStart);
    const branch = scriptSrc.slice(branchStart, step1At);
    expect(branch).toContain('npm run test:unit');
    // 第二份门参数 = 门与「被落盘的那一次」分家（#974 的 pattern 漂移同形）
    expect(branch).not.toContain('--testPathPattern');
    expect(branch).not.toContain('jest.config.unit.js');
    // 判定只看退出码：分支的**最后一条语句**就是那一个 exit，后面不再有别的评价逻辑。
    // ⚠️ 不能用 `indexOf('exit $art.ExitCode')` —— 那串字面量也出现在给人看的那行
    //    「全量单测门未过（exit $art.ExitCode）」里，取到的第一处不是语句而是文案。
    const exitMatch = branch.match(/^ {4}exit \$art\.ExitCode$/m);
    expect(exitMatch).not.toBeNull();
    const callAt = branch.indexOf('Publish-UnitGateComment -PrNumber $PostToPr');
    const exitAt = branch.indexOf(exitMatch[0]);
    expect(callAt).toBeGreaterThan(-1);
    expect(callAt).toBeLessThan(exitAt);
    expect(branch.slice(exitAt + exitMatch[0].length)).toMatch(/^\n\}\n/); // 其后只剩分支的右花括号
    expect(branch.slice(0, callAt)).toContain('if ($PostToPr -gt 0)');
    // Get-HeadSha 用共享库那份，本文件不自定义（`gateCommonLibContract` 的 G2 同一条纪律）
    expect(branch).toContain("lib\\gate-common.ps1");
    expect(scriptSrc).not.toMatch(/function Get-HeadSha/);
    // 摘要只由复用件产出：全量门这一支不许再抄一份裁剪逻辑（第二真源）
    expect(branch).not.toContain('New-TestFailureSummary -Output');
    expect(branchStart).toBeGreaterThan(scriptSrc.indexOf('if ($DryRun)')); // -DryRun 的承诺优先
  });
});

