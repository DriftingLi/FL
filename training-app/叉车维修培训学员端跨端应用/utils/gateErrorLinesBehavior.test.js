/**
 * ④ 本地编译门错误行判据的**题库**（#2045，防「门瞎了」）—— 行为守护。
 *
 * 这个文件是「烟雾报警器的测试按钮」：门判据搬进 `scripts/lib/gate-error-lines.ps1` 之后，
 * 本文件把它**真执行**一遍 —— 喂历史错题，看它判没判红。判据留在门脚本里时做不到这件事
 * （dot-source 门脚本会整篇执行），于是「改坏判据」只能等几周后真机或发版冒烟才暴露（血账 #4/#5）。
 *
 * 样本来源（全部有案可查，一条都不许发明）：
 *   · 血账 #4 原文族：`docs/adr/0008-移动端验收门与证据.md` 的「2026-09-20 修订」段与
 *     「已知盲区与临时三条件」段的对照表；`docs/verification/recruiter-workspace/1207/README.md`
 *     的「同一批日志行对两条 pattern 的现测对照」表（四行原文 + 逐行命中数）。
 *   · 血账 #5：publish 段与新鲜度的错题住在 `utils/kotlinAllStaleExportBehavior.test.js`
 *     （#1272 已落地，本票只把**判据族**入册，不重复那半边）。
 *   · 设备 console `[Error]` 假失败族：`lib/hx-errors.ps1` 头注 + ADR-0012；它的行为守护在
 *     `utils/hxErrorLinesBehavior.test.js`（第三形态，本票射程外，只在那边加样品）。
 *
 * ⚠️ 归腿现测（不是作者自报）：本文件起 `pwsh` 子进程真跑判据 ⇒ `node scripts/classify-guards.mjs`
 *    归**行为腿**（`test:behavior`）。票面原写「判定 fixture 进接线腿」，同一段同时规定
 *    「按 `classify-guards.mjs` 现测归腿，不手判」—— 现测结果是行为腿，故以现测为准；
 *    接线那一侧（门脚本真的 dot-source 并调用本库）由 `utils/gateErrorLinesWiringContract.test.js`
 *    承担，那一个文件不起子进程 ⇒ 接线腿。裁定与实测见移动端 `docs/adr/0012-…` 的「实施回记（#2045）」。
 *
 * 防恒绿的写法：每一族都配**两半** —— 合成错题必须判红 + 干净样本必须判绿。
 * 只写前者，判据退化成「什么都算错误」照样全绿；只写后者，判据退化成「什么都不算」也全绿。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与 `utils/hxErrorLinesBehavior.test.js` / `mpWeixinGateContract.test.js` 同律（静默跳过等于假绿）。
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB = path.join(ROOT, 'scripts', 'lib', 'gate-error-lines.ps1');

// ---------- ④a：dev 全量编译门的样本 ----------

/** 血账 #4 的原始漏网行（两种实测形态：`build.log` 里的插件诊断、带时间戳的同族行）。 */
const BLEED4_LINES = [
  '[plugin:uni:app-uvue] Could not resolve "../../api/recruit"  at pages/recruiter/components/recruiter-filter-drawer.uvue:84:5',
  '18:31:03.286 [plugin:uni:app-uvue] Could not resolve "../../api/helpers"',
];

/** ④a 判据本来就认得的族 —— 每族一条保险题（编译器升级后换长相时，先在这里暴露）。 */
const G1_FAMILIES = [
  'unresolved reference: bar at utils/secureStorage.uts:12',
  'Cannot infer type for this parameter. Please specify it explicitly.',
  '找不到名称 undefined',
  '类型不匹配: Any? vs Any',
  '[plugin:uni:app-uts] kotlin编译失败',
  'error: 找不到名称“region”。',
];

/** 否证行：门自己写的结论行与「零错误」陈述 —— **必须不判红**，否则这道门恒红。 */
const G1_CLEAN_LINES = [
  '编译完成: 0 error, 0 warning',
  'COMPILE_RESULT errors=0 clean=True log=.ci-verify/build.log',
  'errors: 0',
  'no errors found',
  'error count: 0',
];

/** 非错误行：编译中断的收尾文案 —— 它不是诊断，判红就是把「环境/中断」混进「编译失败」。 */
const G1_NEUTRAL_LINES = [
  '已停止运行...',
  '正在编译...',
];

// ---------- ④c：整模块 kotlinc 门的样本 ----------

/** kotlinc 的两种错误行形（血账 #5 那轮的实测形状：`e: ` 前缀与 `路径:行:列: error:`）。 */
const G5_MATCH_SAMPLES = [
  { line: 'e: /tmp/app-android/Foo.kt:12:3: unresolved reference: bar', fragments: 1 },
  { line: '/tmp/app-android/Foo.kt:9:1: error: type mismatch', fragments: 1 },
  // 计数判据返回的是**片段**、不是整行 —— 一行两处 `error:` ⇒ 2。这条语义钉住，
  // 免得日后有人「顺手改成整行」把 KOTLIN_ALL_RESULT 的 errors= 读数悄悄减半。
  { line: 'e: a.kt:1:1: error: first: error: second', fragments: 2 },
];

/** ④c 的干净面：warning 行与门自己的结论行不得计入 error。 */
const G7_CLEAN_LINES = [
  'w: /tmp/app-android/Foo.kt:2:2: warning: something unused',
  'KOTLIN_ALL_RESULT errors=0 classes=1526 files=120',
];

// ---------- 探针（形状照抄 utils/hxErrorLinesBehavior.test.js，含它踩过的三个坑）----------

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/**
 * 把一组行喂给**真正的 lib 判据**，回读 pwsh 的原始输出（失败即抛，不返回降级值）。
 * @param {string} fn lib 函数名（Get-CompileGateErrorLines / Get-KotlincGateErrorMatches / Select-KotlincGateErrorLines）
 * @param {string[]} lines 合成日志行
 */
function probe(fn, lines) {
  // ⚠️ 必须显式用 "`n" 拼成**一个多行字符串**再传：形参是 [string]，
  //    传数组会被 PowerShell 用**空格**拼接（实测把多行压成一行 ⇒ 判据整体失效）。
  const psLines = lines.length
    ? lines.map((l) => `'${String(l).replace(/'/g, "''")}'`).join(',\n')
    : '';
  const joinArgs = lines.length ? ' -join "`n"' : '';
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    `. "${LIB}"`,
    `$in = @(\n${psLines})${joinArgs}`,
    `$hits = @(${fn} -Output $in)`,
    'Write-Output ("COUNT=" + $hits.Count)',
    'Write-Output ("ISARRAY=" + ($hits -is [array]))',
    'foreach ($h in $hits) { Write-Output ("HIT=" + $h) }',
  ].join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return String(stdout);
  } catch (e) {
    throw new Error(
      `判据探针执行失败（pwsh 不可用或 lib 抛错，fail-closed 不跳过）：\n`
      + `exit=${e.status}\nstdout=${e.stdout || ''}\nstderr=${e.stderr || e.message}`
    );
  }
}

function read(stdout) {
  const grab = (key) => {
    const hit = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
    if (!hit) {
      // fail-closed：探针没打出这一行 ⇒ 读数不可信，**不许当 0 用**。
      // 把「没读到」读成「判据没命中」正是本票要消灭的那类假绿（恒 0 与恒真同样危险）。
      throw new Error(
        `探针输出缺少 ${key}= 行（读数不可信，不降级成 0）：\n`
        + `stdout=${JSON.stringify(String(stdout))}`
      );
    }
    return hit[1].trim();
  };
  return {
    count: Number(grab('COUNT')),
    isArray: grab('ISARRAY') === 'True',
    hits: String(stdout)
      .split(/\r?\n/)
      .filter((l) => l.startsWith('HIT='))
      .map((l) => l.slice(4)),
  };
}

const COMPILE_JUDGE = 'Get-CompileGateErrorLines';
const KOTLIN_COUNT_JUDGE = 'Get-KotlincGateErrorMatches';
const KOTLIN_LINES_JUDGE = 'Select-KotlincGateErrorLines';

describe('④a 判据题库：dev 全量编译门的错误行判据（血账 #4 销账证据）', () => {
  test('G0: 血账 #4 的历史漏网行 ⇒ **必须判红**（0 = 门瞎着，这条就是那个报警器）', () => {
    const r = read(probe(COMPILE_JUDGE, BLEED4_LINES));
    expect(r.count).toBe(BLEED4_LINES.length);
    // 判红还要求**报出的是那一行本身**，不是随便一行别的（防「计数对但内容错」）
    expect(r.hits.some((h) => h.includes('Could not resolve "../../api/recruit"'))).toBe(true);
  });

  test('G1: 判据认得的每一族都有保险题 ⇒ 全部判红', () => {
    const r = read(probe(COMPILE_JUDGE, G1_FAMILIES));
    expect(r.count).toBe(G1_FAMILIES.length);
  });

  test('G2: 否证行与门自己的结论行 ⇒ 一条都不判红（判据不退化成「什么都算错误」）', () => {
    const r = read(probe(COMPILE_JUDGE, G1_CLEAN_LINES));
    expect({ count: r.count, hits: r.hits }).toEqual({ count: 0, hits: [] });
  });

  test('G3: 「已停止运行…」一类中断收尾文案不是编译诊断 ⇒ 不判红', () => {
    const r = read(probe(COMPILE_JUDGE, G1_NEUTRAL_LINES));
    expect(r.count).toBe(0);
  });

  test('G4: 错题与干净行混在一份日志里 ⇒ 只报错题行', () => {
    const mixed = [...BLEED4_LINES, ...G1_CLEAN_LINES, ...G1_NEUTRAL_LINES];
    const r = read(probe(COMPILE_JUDGE, mixed));
    expect(r.count).toBe(BLEED4_LINES.length);
    r.hits.forEach((h) => expect(h).toContain('Could not resolve'));
  });
});

describe('④c 判据题库：整模块 kotlinc 门的两形错误行', () => {
  test('G5: `e: ` 前缀与 `路径:行:列: error:` 都判红，且计数返回**片段**（一行两处 = 2）', () => {
    G5_MATCH_SAMPLES.forEach((s) => {
      const r = read(probe(KOTLIN_COUNT_JUDGE, [s.line]));
      expect({ line: s.line, fragments: r.count }).toEqual({ line: s.line, fragments: s.fragments });
    });
  });

  test('G6: 打印判据返回整行，内容与计数判据同源 ⇒ 不许一个说有、一个说没有', () => {
    const lines = G5_MATCH_SAMPLES.map((s) => s.line);
    const countOut = read(probe(KOTLIN_COUNT_JUDGE, lines));
    const lineOut = read(probe(KOTLIN_LINES_JUDGE, lines));
    expect(countOut.count).toBeGreaterThan(0);
    expect(lineOut.hits).toEqual(lines);
  });

  test('G7: warning 行与 KOTLIN_ALL_RESULT 结论行 ⇒ 不判红', () => {
    const r = read(probe(KOTLIN_COUNT_JUDGE, G7_CLEAN_LINES));
    expect(r.count).toBe(0);
    const l = read(probe(KOTLIN_LINES_JUDGE, G7_CLEAN_LINES));
    expect(l.hits).toEqual([]);
  });

  test('G8: ④c 对插件形态**不命中**是已知边界（输入面是纯 kotlinc 文本，别把它读成已覆盖）', () => {
    // 实测对照见 docs/verification/recruiter-workspace/1207/README.md 的对照表最后两行：
    // 那两行属 CLI/插件输出面，不在 ④c 的输入面上 ⇒ 本票按「拿不准的宁缺毋滥」不改 ④c 措辞。
    // 这条钉的是「边界还在」：哪天输入面变了、它开始命中，本用例会红，逼着回来改口径。
    const r = read(probe(KOTLIN_COUNT_JUDGE, ['[plugin:uni:app-uts] kotlin编译失败']));
    expect(r.count).toBe(0);
  });
});

describe('两判据共同的形状不变式（StrictMode 退化坑，票面 User Story 6）', () => {
  test('G9: 空输入 ⇒ 返回**数组**而不是 $null（$null.Count 在 StrictMode 下会抛）', () => {
    [COMPILE_JUDGE, KOTLIN_COUNT_JUDGE, KOTLIN_LINES_JUDGE].forEach((fn) => {
      const r = read(probe(fn, []));
      expect({ fn, count: r.count, isArray: r.isArray }).toEqual({ fn, count: 0, isArray: true });
    });
  });

  test('G10: 判据函数零副作用 —— 同一进程里连续两次调用输出全等（不读文件、不碰 HBuilderX）', () => {
    const once = read(probe(COMPILE_JUDGE, G1_FAMILIES));
    const twice = read(probe(COMPILE_JUDGE, G1_FAMILIES));
    expect(twice.hits).toEqual(once.hits);
    expect(twice.count).toBe(once.count);
  });
});
