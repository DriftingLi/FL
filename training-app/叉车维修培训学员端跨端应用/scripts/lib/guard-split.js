/**
 * 守护分类的**机械判据**（唯一一份）+ ③ 门两腿的**切分函数**（#1702）。
 *
 * 两个消费者，共用同一份判据：
 *   ① `scripts/classify-guards.mjs` —— 人读分类表 / `--json` / `utils/guardClassification.test.js`；
 *   ② `jest.config.unit.js` 与 `jest.config.behavior.js` —— ③ 门拆腿后的 `testMatch`。
 *
 * 为什么必须只有一份（#1702）：③ 门拆腿的**切分轴**就是守护分类本身（行为守护承重、接线守护不承重）。
 * 若两处各写一套「什么算真的执行了被测面」，拆出来的两腿会与 `docs/agents/guards.md` 的分类表**静默漂移**
 * —— 那正是本仓血账 #974 的形态（同一件事两份真源，谁也没红）。
 *
 * ⚠️ 本文件的判据文本**逐字搬自** `scripts/classify-guards.mjs`（含 #1178 那条注释）；
 * 搬家时以 `node scripts/classify-guards.mjs --json` 前后逐字节相同为验证（#1702 实施记录）。
 *
 * 判据是**代码级事实**，不是文件名约定：叫 `*Behavior.test.js` 却只 `expect(src).toContain(...)` 的，
 * 照样归接线守护（见 classify-guards.mjs 文件头「第一版栽在这里」那段）。
 */
'use strict';

const fs = require('node:fs');
const path = require('node:path');

/**
 * ① 执行判据：源码里有没有**代码级**的「跑起来」调用。
 * 调用方必须先用 stripComments 去注释再喂进来 —— 注释里提到 `spawnSync(` 不算执行。
 */
const EXECUTION_MARKERS = [
  { re: /require\(\s*['"]child_process['"]\s*\)/, why: 'require(child_process)：起子进程真跑载体' },
  { re: /\bfrom\s+['"]node:child_process['"]/, why: 'import node:child_process' },
  { re: /\bexecFileSync\s*\(/, why: 'execFileSync：同步起子进程' },
  { re: /\bspawnSync\s*\(/, why: 'spawnSync：同步起子进程' },
  { re: /\bexecSync\s*\(/, why: 'execSync：同步起子进程' },
  // ⚠️ `utsHarness` 这条必须锚在**执行调用**上，不能只认模块名（#1178 实测踩过）：
  // 读取层归一（ADR-0019）落地后，一批**接线守护**也 `require('./utsHarness')` 取共享读者 `readText` ——
  // 但它们**并不执行** `.uts`，只是用它读文本。只认模块名会把 6 个接线守护误升成行为守护
  // （H5「点名的手抄镜像仍是接线守护」当场转红，而它们的断言一行没改）。
  // 故只认真正的执行出口：`loadUts(`（把 .uts 当 JS 跑）。
  { re: /\bloadUts\s*\(/, why: 'utsHarness.loadUts：把 .uts 当 JS 真执行' },
  { re: /import\(\s*/, why: '动态 import()：真加载被执行物' },
];

/** 去注释：块注释 + 行注释（`//` 起）。字符串里的 `//` 极少见，本仓的断言文本里没有 http:// 之类 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
}

/** 已去注释的源码 ⇒ 命中的执行判据（`why` 串，空数组 = 接线守护）。 */
function matchExecutionMarkers(code) {
  return EXECUTION_MARKERS.filter((m) => m.re.test(code)).map((m) => m.why);
}

/** 源码 ⇒ 是不是「现场执行了被测面」的行为守护。 */
function isBehaviorSource(src) {
  return matchExecutionMarkers(stripComments(src)).length > 0;
}

/** `utils/` 下的测试文件名（排序稳定 ⇒ 两腿的 testMatch 可 diff）。 */
function listTestFiles(utilsDir) {
  return fs
    .readdirSync(utilsDir)
    .filter((f) => f.endsWith('.test.js'))
    .sort();
}

/** 文件名 ⇒ jest `testMatch` 的 glob（`<rootDir>` 前缀，正斜杠，与拆分前那条 glob 同根）。 */
function toTestMatch(files) {
  return files.map((f) => `<rootDir>/utils/${path.basename(f)}`);
}

/**
 * 把 `utils/` 下的套件按判据切成两腿。
 * fail-closed：任一条腿为空就抛（空集合上判绿正是本仓防的那件事；「0 个套件」比「红」更坏，
 * 它长得和绿一模一样）。
 */
function splitSuites(utilsDir) {
  const files = listTestFiles(utilsDir);
  const behavior = [];
  const wiring = [];
  for (const name of files) {
    const src = fs.readFileSync(path.join(utilsDir, name), 'utf8');
    (isBehaviorSource(src) ? behavior : wiring).push(path.join(utilsDir, name));
  }
  if (files.length === 0) {
    throw new Error(`guard-split: ${utilsDir} 下没有任何 *.test.js —— 拒绝在空集合上切分（fail-closed）`);
  }
  if (behavior.length === 0 || wiring.length === 0) {
    throw new Error(
      `guard-split: 切分出一条空腿（behavior=${behavior.length} wiring=${wiring.length}）—— 判据或目录形态变了，先查 scripts/classify-guards.mjs`
    );
  }
  return {
    utilsDir,
    all: files.map((f) => path.join(utilsDir, f)),
    behavior,
    wiring,
  };
}

module.exports = {
  EXECUTION_MARKERS,
  stripComments,
  matchExecutionMarkers,
  isBehaviorSource,
  listTestFiles,
  toTestMatch,
  splitSuites,
};
