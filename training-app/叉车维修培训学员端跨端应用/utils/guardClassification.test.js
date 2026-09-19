/**
 * ③ 门判据的守护 —— 「这个测试到底有没有在**执行**被测物？」(issue #1156，2026-09-18)
 *
 * 为什么需要它：移动端 ③ 门（`npm run test:unit` 全绿）此前**只有一句「全绿」、没有判据**。
 * 而已证的事实是那句「全绿」**可以恒真** —— 2026-09-18 实测：
 *
 *     把 utils/format.uts 的 `return '0s'` 改坏   ⇒ format.test.js 照样 8 passed（绿）
 *     把 utils/checkinCalendar.uts 的算法改坏     ⇒ checkinCalendar.test.js 照样绿
 *
 * 两个文件都是**手抄的镜像实现**（`format` 连「镜像同步」那段都没有）。⇒ 「全绿」本身不是证据；
 * **「改坏被测物会不会红」才是**。本守护守的是那条判据的**可机检形状**：
 *
 *   H1 分类器真跑得起来（fail-closed：跑不起来就抛，不 skip）
 *   H2 比例稳定：`behavior / total` 不得低于**已经实测为真**的下界（下降 = 有守护被降级成接线、或被执行物被换掉）
 *   H3 **每个行为守护都必须引用至少一个仓内载体文件**（`.uts` / `.uvue` / `.ps1` / `.mjs` / `.json`）
 *      —— 否则它「跑起来了」但跑的是自己搭的假货，不构成 ③ 门证据
 *   H4 「零载体引用的接线守护」**有名有姓地钉住**（今天恰好是那三个手抄镜像）—— 集合一变就要人看
 *   H5 点名的那几个手抄镜像模块**今天都还是接线守护**（供「迁移到真执行」那批票当**迁移前**的基线）
 *
 * **H2/H4 取值都经过本机实测**（2026-09-18，origin/master `bee21af4` + 本守护自身）：99 个测试文件、
 * 13 个行为 / 86 个接线。数字不是抄来的、是跑出来的；它们**故意允许上升**（把镜像迁成真执行会让
 * behavior 变多）—— 本守护挡的是**下降**与**悄悄换集合**，这正是永绿的反面。
 *
 * ⚠️ 取数纪律（2026-09-18 血账）：**必须在新进程里取**（`node scripts/classify-guards.mjs --json`）。
 * 本守护的第一版把常量写成 12/98 —— 那是同一进程里 `require()` 了**更早一份** JSON 得到的过时读数
 * （该文件当时还没被写进去）。**过时的下界会让守护对着一个不存在的世界判绿。**
 *
 * 运行前提：需要 `node`（跑 `.mjs` 分类器）。**不可用时 fail-closed 抛错，不 skip**
 * （仓库先例：`screenshotDiffSingleFileBehavior.test.js` / `pngDiffBehavior.test.js`）。
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const CLI = path.join(ROOT, 'scripts', 'classify-guards.mjs');

/** 本机实测下界（2026-09-18，origin/master `bee21af4` + 本守护自身）：不允许下降 */
const BEHAVIOR_MIN = 13;
const TOTAL_MIN = 99;

/** H4：允许「既不执行、也不引用仓内载体」的接线守护 —— 必须显式登记，新增即红 */
const ZERO_REF_WIRING_ALLOWED = [
  'checkinCalendar.test.js', // 纯镜像手抄、无源码锚点（改坏 .uts 不红：本机实测）
  'format.test.js',          // 纯镜像手抄、连「镜像同步」段都没有（改坏 .uts 不红：本机实测）
  'profileMetrics.test.js',  // 纯镜像，无任何文件引用
];

/** H5：点名的手抄镜像模块 —— 迁移前后都要能被这条断言区分 */
const KNOWN_MIRROR_MODULES = [
  'aiSourcesDisplay.test.js',
  'checkinCalendar.test.js',
  'faqDisplay.test.js',
  'format.test.js',
  'notebookDisplay.test.js',
  'pointsDisplay.test.js',
  'searchDisplay.test.js',
  'secureStorage.test.js',
];

describe('守护分类（③ 门判据的可机检形状，#1156）', () => {
  let report;

  beforeAll(() => {
    try {
      const stdout = execFileSync('node', [CLI, '--json'], {
        encoding: 'utf8',
        timeout: 60000,
        windowsHide: true,
        maxBuffer: 16 * 1024 * 1024,
      });
      report = JSON.parse(String(stdout));
    } catch (e) {
      throw new Error(
        'classify-guards.mjs 跑不起来（fail-closed 不跳过）：\n'
          + `status=${e.status}\nstdout=${String(e.stdout || '')}\nstderr=${String(e.stderr || e.message || '')}`
      );
    }
  });

  test('H1: 分类器跑完且至少认出一个测试文件（空集合上判绿 = 本仓防的那件事）', () => {
    expect(report.ok).toBe(true);
    expect(report.total).toBeGreaterThan(0);
    expect(report.behavior.length + report.wiring.length).toBe(report.total);
  });

  test(`H2: 行为守护不得少于实测下界（${BEHAVIOR_MIN} 个 / 共 ≥${TOTAL_MIN}）——下降 = 有守护被降级成接线`, () => {
    // 允许上升（迁移镜像会让 behavior 变多）；挡的是下降与集合缩水
    expect(report.behaviorCount).toBeGreaterThanOrEqual(BEHAVIOR_MIN);
    expect(report.total).toBeGreaterThanOrEqual(TOTAL_MIN);
  });

  test('H3: 每个行为守护都必须引用至少一个仓内载体（否则它跑的是自己搭的假货）', () => {
    const offenders = report.rows
      .filter((r) => r.cls === 'behavior' && r.refs.length === 0)
      .map((r) => r.name);
    expect(offenders).toEqual([]);
  });

  test('H4: 「零载体引用」的接线守护必须显式登记（集合一变就要人看）', () => {
    const zeroRef = report.rows.filter((r) => r.cls === 'wiring' && r.refs.length === 0).map((r) => r.name);
    expect(zeroRef.sort()).toEqual([...ZERO_REF_WIRING_ALLOWED].sort());
  });

  test('H5: 点名的手抄镜像模块今天都还是接线守护（「迁移到真执行」那批票的迁移前基线）', () => {
    const wiringNames = new Set(report.wiring);
    const stillWiring = KNOWN_MIRROR_MODULES.filter((n) => wiringNames.has(n));
    // 迁移推进时这条会红 —— 那正是要的：迁移 PR 必须把已迁走的模块从这里划掉并说明
    expect(stillWiring).toEqual(KNOWN_MIRROR_MODULES);
  });
});
