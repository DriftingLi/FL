/**
 * ③ 门「两腿」的形状守护（#1702）—— 拆腿之后，**漏套件**才是新的假绿形态。
 *
 * 为什么需要它：拆腿之前 ③ 门只有一条 glob，套件不会漏；拆腿之后两个 config 各自一份 testMatch，
 * 于是出现了一种新的、**看起来完全正常**的坏法：某个套件被两个 testMatch 都排除在外
 * （判据改了一行、文件名带了子目录、手写清单漂移）⇒ 两条腿都绿、那个套件却从门里消失。
 * 这与 #974 同形（同一件事两份真源，谁也没红），也正是 docs/agents/dev-loop.md 明令禁止
 * 「新建第三份被测面清单」的原因：清单只能由 scripts/lib/guard-split.js **现算**。
 *
 * 本守护钉住的是**集合关系**（不是当前数字 —— 数字在 utils/guardClassification.test.js 的 H2 下界里）：
 *
 *   P1 分割完整性：wiring ∪ behavior = utils 下**全部** *.test.js，两两不相交，且两腿都非空
 *   P2 unit 腿的 testMatch 逐项等于分类器算出的 wiring 集（config 不许自说自话）
 *   P3 behavior 腿的 testMatch 逐项等于 behavior 集，且**只有它**挂心跳 reporter
 *   P4 jest.config.all.js 必须逐字保留拆腿前那条全量 glob —— 它是 --testPathPattern 收窄跑法的
 *      底盘，收窄成任一腿都会让另一半「静默不存在」（#1702 实施中实测过这个坑）
 *   P5 **jest 自己的口径**与分类器一致：真起子进程跑 --listTests，两腿套件数 = 分类器读数，
 *      且两腿清单的并集 = 全量清单、无重复（P2/P3 只验字符串，P5 验 jest 真认）
 *   P6 心跳行格式（JEST_SUITE i=k/N status=… secs=… elapsed=… file=…）由 reporter 自己产出：
 *      喂合成结果，断言三行 token 齐全、PASS/FAIL 与失败数如实 —— 钉产物，不钉源码文本。
 *      合成场景前必须隔离 `FL_BEHAVIOR_SUITE_*`（驱动 env，语义由 behaviorLegWatchdogBehavior 的 W6
 *      单独守着）：dev-finish 逐套件驱动时这两个变量就在进程里，reporter 正确地优先用全局数，
 *      不隔离会让本条在驱动腿里假装变红（total=78 而非 2，#2013）
 *
 * 运行前提：需要 node（起 jest 子进程做 P5）。不可用时 fail-closed 抛错，不 skip
 * （本仓先例：utils/guardClassification.test.js / pngDiffBehavior.test.js）。
 */
const { execFileSync } = require('child_process');
const path = require('path');

// 读仓内源码/配置一律走共享读者（ADR-0019 读取层归一）——裸 readFileSync 会被
// scripts/check-contract-read.mjs 判违规（CRLF 下多行锚点静默失配，守护形同不存在）。
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const UTILS = __dirname;
const JEST_ENTRY = path.join(ROOT, 'node_modules', 'jest', 'bin', 'jest.js');

const { splitSuites, toTestMatch } = require('../scripts/lib/guard-split.js');

/** 用 jest 自己的 --listTests 取某份 config 认得的套件（绝对路径清单）。 */
function listTests(configName) {
  const stdout = execFileSync('node', [JEST_ENTRY, '--config', configName, '--listTests'], {
    cwd: ROOT,
    encoding: 'utf8',
    timeout: 120000,
    windowsHide: true,
    maxBuffer: 16 * 1024 * 1024,
  });
  return String(stdout)
    .split(/\r?\n/)
    .filter((l) => l.trim().length > 0)
    .map((l) => l.trim());
}

const base = (abs) => path.basename(abs);

let split;

beforeAll(() => {
  split = splitSuites(UTILS);
});

describe('③ 门两腿的形状（#1702）', () => {
  test('P1: 两腿是 utils 全部套件的一个分割（不漏、不重、都非空）', () => {
    const all = split.all.map(base).sort();
    const union = [...split.wiring.map(base), ...split.behavior.map(base)].sort();
    expect(union).toEqual(all);
    expect(new Set(union).size).toBe(union.length);
    expect(split.behavior.length).toBeGreaterThan(0);
    expect(split.wiring.length).toBeGreaterThan(0);
    expect(all.every((n) => n.endsWith('.test.js'))).toBe(true);
  });

  test('P2/P3: 两腿的 testMatch 由分类器现算，且只有 behavior 腿挂心跳 reporter', () => {
    const unit = require('../jest.config.unit.js');
    const behavior = require('../jest.config.behavior.js');

    // jest 的 testMatch 是**有序**数组（splitSuites 按文件名排序 ⇒ 两端可比）
    expect(unit.testMatch).toEqual(toTestMatch(split.wiring));
    expect(behavior.testMatch).toEqual(toTestMatch(split.behavior));

    // 两腿的公共底盘必须同一份（否则「两腿」其实是两套环境）
    expect(behavior.testEnvironment).toBe(unit.testEnvironment);
    expect(behavior.moduleFileExtensions).toEqual(unit.moduleFileExtensions);

    // 心跳 reporter 只挂行为腿：它是「串行 + 单套件可达数分钟」的解药，接线腿没有这个症状
    expect(behavior.reporters).toEqual(['default', '<rootDir>/scripts/jest-reporter-suite-progress.js']);
    expect(unit.reporters).toBeUndefined();
  });

  test('P4: jest.config.all.js 逐字保留拆腿前的全量 glob（收窄跑法的底盘）', () => {
    const all = require('../jest.config.all.js');
    const unit = require('../jest.config.unit.js');
    expect(all.testMatch).toEqual(['<rootDir>/utils/**/*.test.[jt]s?(x)']);
    expect(all.testEnvironment).toBe(unit.testEnvironment);
    expect(all.reporters).toBeUndefined();
  });

  test('P5: jest 自己的 --listTests 与分类器读数一致（并集 = 全量、两两不重）', () => {
    const fromUnit = listTests('jest.config.unit.js');
    const fromBehavior = listTests('jest.config.behavior.js');
    const fromAll = listTests('jest.config.all.js');

    expect(fromUnit.length).toBe(split.wiring.length);
    expect(fromBehavior.length).toBe(split.behavior.length);
    expect(fromAll.length).toBe(split.all.length);

    const union = [...fromUnit.map(base), ...fromBehavior.map(base)];
    expect(new Set(union).size).toBe(union.length);
    expect(union.sort()).toEqual(fromAll.map(base).sort());
  });

  test('P6: 心跳行三行齐、PASS/FAIL 与失败数如实（钉产物，不钉源码文本）', () => {
    const SuiteProgressReporter = require('../scripts/jest-reporter-suite-progress.js');
    const writes = [];
    const original = process.stdout.write;
    // #2013：本用例断言的是**合成场景**（喂 numTotalTestSuites=2 ⇒ total=2）。而 dev-finish 逐套件
    // 驱动行为腿时会把全局 planned/offset 注入子进程 env（本仓内部契约，语义由 W6 单独守着）——
    // 驱动形态下本套件必然继承那一对 env，reporter 正确地优先用全局数 ⇒ P6 会在驱动腿里假装变红
    // （total=78），而同机直跑是绿的。隔离点只能放测试侧：reporter 的 env 优先级语义不许为测试开分支。
    const origOffset = process.env.FL_BEHAVIOR_SUITE_OFFSET;
    const origTotal = process.env.FL_BEHAVIOR_SUITE_TOTAL;
    delete process.env.FL_BEHAVIOR_SUITE_OFFSET;
    delete process.env.FL_BEHAVIOR_SUITE_TOTAL;
    process.stdout.write = (chunk) => {
      writes.push(String(chunk));
      return true;
    };
    try {
      const reporter = new SuiteProgressReporter();
      reporter.onRunStart({ numTotalTestSuites: 2 });
      reporter.onTestResult(
        { path: path.join(UTILS, 'aaa.test.js') },
        { numPassingTests: 3, numFailingTests: 0, perfStats: { runtime: 1200 } }
      );
      reporter.onTestResult(
        { path: path.join(UTILS, 'bbb.test.js') },
        { numPassingTests: 1, numFailingTests: 2, perfStats: { runtime: 3400 }, testExecError: null }
      );
      reporter.onRunComplete({}, { numTotalTestSuites: 2, numPassedTestSuites: 1, numFailedTestSuites: 1 });
    } finally {
      process.stdout.write = original;
      if (origOffset === undefined) delete process.env.FL_BEHAVIOR_SUITE_OFFSET; else process.env.FL_BEHAVIOR_SUITE_OFFSET = origOffset;
      if (origTotal === undefined) delete process.env.FL_BEHAVIOR_SUITE_TOTAL; else process.env.FL_BEHAVIOR_SUITE_TOTAL = origTotal;
    }

    const lines = writes.join('').split('\n').filter((l) => l.startsWith('JEST_SUITE'));
    expect(lines.length).toBe(4);

    expect(lines[0]).toMatch(/^JEST_SUITE_PLAN total=2 started=\d{2}:\d{2}:\d{2} /);
    expect(lines[1]).toMatch(
      /^JEST_SUITE i=1\/2 status=PASS secs=1\.2 elapsed=\d+\.\d tests=3 failed=0 file=utils\/aaa\.test\.js$/
    );
    expect(lines[2]).toMatch(
      /^JEST_SUITE i=2\/2 status=FAIL secs=3\.4 elapsed=\d+\.\d tests=3 failed=2 file=utils\/bbb\.test\.js$/
    );
    expect(lines[3]).toMatch(/^JEST_SUITE_DONE total=2 passed=1 failed=1 elapsed=\d+\.\d$/);
  });

  test('P7: 四条 npm 入口逐字面量 —— 腿与 config 的对应关系不许在 package.json 里漂', () => {
    // 为什么钉这里：P2/P3 钉的是 config 的内容，可「哪条命令跑哪条腿」这一层在 package.json。
    // 手改一条（比如把 test:behavior 指回 unit 的 config）会让两腿都「绿」，而行为腿其实一次没跑 ——
    // 与 #974 同形的第二真源漂移。devFinishContract 的 L3 钉的是脚本里那四个调用点，这一条钉被调的命令本身。
    const pkg = JSON.parse(readText(path.join(ROOT, 'package.json')));
    const s = pkg.scripts;
    expect(s['test:unit']).toBe('jest --config jest.config.unit.js -i');
    expect(s['test:behavior']).toBe('jest --config jest.config.behavior.js -i');
    expect(s['test:unit:local']).toBe('jest --config jest.config.unit.js');
    expect(s['test:behavior:local']).toBe('jest --config jest.config.behavior.js');
    // 两腿顺序跑才是全量：npm test 不许退化成只跑一半
    expect(s.test).toBe('npm run test:unit && npm run test:behavior');
    // 门入口两条仍指向同一个脚本的 -UnitGate / -UnitLocal（拆腿没有多开入口）
    expect(s['test:unit:gate']).toContain('scripts/dev-finish.ps1 -UnitGate');
    expect(s['test:unit:gate:local']).toContain('scripts/dev-finish.ps1 -UnitLocal');
  });
});
