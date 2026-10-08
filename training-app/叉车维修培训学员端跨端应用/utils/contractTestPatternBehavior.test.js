/**
 * Q-A 契约测试 **pattern 唯一真源** 的行为级守护（运行期）—— 漂移缺陷（2026-09-14）
 *
 * 为什么需要这个文件：
 *   `dev-finish.ps1` 步骤 3 与 `lib/test-compile.ps1` **各抄了一份 `$testPattern`**，并且**漂移了** ——
 *   步骤 3 那份少了 `hxRun` / `hxTimingBehavior` / `hxError` 三个 token。后果不是「少跑几个测试」：
 *   步骤 3 **先跑且失败即 `exit 1`** ⇒ **真正起门禁作用的是范围更窄的那份**，而步骤 4 用的是更宽的
 *   ⇒ 同一套 Q-A 出现两个范围，新增的运行期守护可能只在其中一条路径上生效。
 *   而 `testCompileContract` T13 当时是**源码文本**断言（`expect(src).toContain('hxRun')`），
 *   它只能证明「字面量里出现过 hxRun」，证明不了「dev-finish 与 test-compile 用的是同一份」。
 *
 *   ⇒ 与 ADR-0008「守护从断言文本改为断言行为」的方向一致：本文件**运行期**断言
 *     ① 真源能被 dot-source 且返回的 pattern 与调用方实际使用的完全一致（逐字面量钉住，含 token 顺序）；
 *     ② 每个 token 都能在**本仓库里真的选中至少一个测试套件**（子串匹配，`hxRun` 匹配不到
 *        `hxErrorLinesBehavior` 这类坑必须被钉住 —— 否则守护「永不执行」）；
 *     ③ 两个调用方都**没有**再抄一份自己的 pattern 字面量（漂移的来源）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与仓库先例 mpWeixinGateContract.test.js:143 一致；静默跳过等于假绿。
 */
const { execFileSync } = require('child_process');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'contract-tests.ps1');
const CALLERS = [
  path.join('scripts', 'dev-finish.ps1'),
  path.join('scripts', 'lib', 'test-compile.ps1'),
];

/** ⚠️ 真源的权威内容：本用例**逐字**钉住它（顺序敏感）。
 *  pattern 变动必须同时改这里 —— 这是故意的：pattern 决定「哪些守护真的会跑」，
 *  静默增删 token 等于静默改变门禁范围，必须留下显式痕迹。
 *  （`hxRun` 覆盖 hxRunContract，故不需要单独的 `hxRunContract` token。） */
const EXPECTED_PATTERN =
  'levelDetect|envCheck|testCompile|buildDeploy|autoScreenshot|screenshotDiff|evidenceGen|devFinish|hxBusyGate|hxRun|hxTimingBehavior|hxError|hxLaunchDetach|capabilitySurface|concurrent401Refresh|contractTestPattern|contractReaderEol|resumeAttachment|jobApplyState|recruiterResume|wtBootstrap|frontier|aiAnswerBody|frontierRun|styleLoop|deviceCapture|emulatorSmoke|wirelessDebugBoundedText';

/** 每个 token 必须能真的选中套件（子串匹配，逐个独立验证，不靠另一个 token 顺带命中）。
 *  ⚠️ 注意最后一个 token：**pattern 必须自指** —— 本守护自己也要能被这个 pattern 选中，
 *  否则它就成了「永不执行」的那个守护（正是本文件要防的形态）。 */
const TOKENS_REQUIRED_TO_SELECT = [
  'levelDetect',
  'envCheck',
  'testCompile',
  'buildDeploy',
  'autoScreenshot',
  'screenshotDiff',
  'evidenceGen',
  'devFinish',
  'hxBusyGate',
  'hxRun',
  'hxTimingBehavior',
  'hxError',
  'hxLaunchDetach',
  'capabilitySurface',
  'concurrent401Refresh',
  'contractTestPattern',
  'contractReaderEol',
  'resumeAttachment',
  'jobApplyState',
  'recruiterResume',
  'wtBootstrap',
  'frontier',
  'aiAnswerBody',
  'frontierRun',
  'styleLoop',
  'deviceCapture',
  'emulatorSmoke',
  'wirelessDebugBoundedText',
];

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** dot-source 真源并回读 pattern（fail-closed） */
function readPatternFromLib() {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    `. "${path.join(ROOT, LIB_REL)}"`,
    'Write-Output ("PATTERN=" + (Get-ContractTestPattern))',
  ].join('\n');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    const m = String(stdout).match(/^PATTERN=(.*)$/m);
    return { ok: true, pattern: m ? m[1].trim() : null, stdout: String(stdout) };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

/** 一次性取全量套件清单（`--listTests`，无 pattern）。P3 逐 token 在**进程内**过滤它，
 *  不再每 token 起一个 jest 子进程。原形态 21 token = 21 次 node/jest 冷启 ≈ 30s，是 ③ 门最贵的
 *  单件；且那个 token 清单只增（docs/adr/0027：每加一个行为守护就 +1 token ⇒ 每加一个永久
 *  +~1.5s），改成列全一次后与 token 数无关。**等价性**：jest 的 `--testPathPattern <token>`
 *  本就是「把 token 当正则测绝对路径」，与下面 `new RegExp(token).test(path)` 同语义——已用一次性
 *  对照脚本对当前全部 token 逐个实测**集合全等**（含 `contractTestPattern` 自指那条）后才落地，
 *  免得改行为守护内部却悄悄改了它度量的东西（本仓最恨的静默漂移）。 */
// ⚠️ 直接用 `process.execPath` 跑 jest 入口，**不要** spawn `npx` / `npx.cmd`：
//    Node 24 下 spawn `.cmd` 不带 shell 会 EINVAL，而 `npx`（无扩展名）从 Node 视角又 ENOENT
//    （它只在 PowerShell 的 PATH 解析里存在）。直连 node + jest 入口是唯一稳的形态。
const JEST_ENTRY = require.resolve('jest/bin/jest');

function listAllTestSuites() {
  const out = execFileSync(
    process.execPath,
    [JEST_ENTRY, '--config', 'jest.config.unit.js', '--listTests'],
    { cwd: ROOT, encoding: 'utf8', timeout: 180000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 }
  );
  return String(out)
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.endsWith('.test.js'));
}

// 整个 describe 内只起一次子进程（缓存）；null = 尚未取
let cachedAllSuites = null;
function allSuites() {
  if (cachedAllSuites === null) cachedAllSuites = listAllTestSuites();
  return cachedAllSuites;
}
function suitesMatchedBy(token, list) {
  const re = new RegExp(token);
  return list.filter((p) => re.test(p));
}

describe('Q-A 契约测试 pattern 唯一真源（运行期）', () => {
  test('P1: 真源可 dot-source，且返回的 pattern 与权威内容逐字一致', () => {
    const r = readPatternFromLib();
    if (!r.ok) {
      throw new Error(
        `dot-source ${LIB_REL} 失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
        + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
      );
    }
    expect(r.pattern).toBe(EXPECTED_PATTERN);
  });

  test('P2: pattern 无空格、无重复 token（jest 参数与可读性）', () => {
    const r = readPatternFromLib();
    if (!r.ok) throw new Error('真源不可读，见 P1');
    const tokens = r.pattern.split('|');
    expect(r.pattern).not.toMatch(/\s/);
    expect(new Set(tokens).size).toBe(tokens.length);
    expect(tokens).toEqual(TOKENS_REQUIRED_TO_SELECT);
  });

  test('P3: 每个 token 都能真的选中至少一个套件（子串匹配坑位）', () => {
    // 列全一次（缓存），逐 token 在进程内按与 jest --testPathPattern 相同的正则语义过滤。
    const list = allSuites();
    // 防空转：若这份全量清单静默为空，下面每个 token 都「0 命中」会全红 —— 但那是「清单没取到」
    // 而非「守护没接线」，两种成因要分得开（本仓反复踩过「判不了当无变化」那半岛）。取不到即非零抛。
    if (list.length === 0) throw new Error('--listTests 返回空清单 ⇒ 取数失灵，无法判 token 命中（fail-closed，不静默放行）');
    const failed = [];
    TOKENS_REQUIRED_TO_SELECT.forEach((token) => {
      if (suitesMatchedBy(token, list).length === 0) failed.push(token);
    });
    // 一个 token 都选不中 ⇒ 该守护永不执行（本仓反复踩过的假绿形态）
    expect(failed).toEqual([]);
  }, 600000);

  test('P4: 两个调用方都没有再抄一份自己的 pattern 字面量（漂移的来源）', () => {
    const offenders = CALLERS.filter((rel) => {
      const src = readText(path.join(ROOT, rel));
      return /\$testPattern\s*=\s*'/.test(src);
    });
    expect(offenders).toEqual([]);
  });

  test('P5: 两个调用方都真的用了真源（不是留着旧变量没人用）', () => {
    const missing = CALLERS.filter((rel) => {
      const src = readText(path.join(ROOT, rel));
      return !src.includes('Get-ContractTestPattern');
    });
    expect(missing).toEqual([]);
  });
});
