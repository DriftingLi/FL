#!/usr/bin/env node
/**
 * 守护分类器 —— 「这个测试是**行为守护**还是**接线守护**？」
 *
 * 为什么需要它（2026-09-18，issue #1156）：移动端 ③ 门（`npm run test:unit` 全绿）此前**只有一句「全绿」、
 * 没有判据**。而 2026-09-18 实测证明那句「全绿」**可以恒真**：把 `utils/format.uts` 的 `return '0s'`
 * 改坏 ⇒ `format.test.js` 照样 8 passed（它是手抄的镜像实现，连「镜像同步」那段都没有）。
 * ⇒ 「全绿」本身不是证据；**「改坏被测物会不会红」才是**。
 *
 * 本脚本把「哪些守护真的跑了被测物」变成**可机检的事实**，而不是作者的声明：
 *
 *   - **行为守护（behavior）**：**执行了**被测对象 —— 起子进程跑 CLI / 真跑 pwsh /
 *     用 `utsHarness` 把 `.uts` 当 JS 跑 / 真跑 PNG 像素差。**只有这一类对 ③ 门承重。**
 *   - **接线守护（wiring）**：读源码文本断言调用点、常量、命名、顺序。它防的是「接线被悄悄拆掉」，
 *     **不构成 ③ 门证据** —— 源码文本断言在「算法被改坏但字面量还在」时**不会红**
 *     （2026-09-18 实测：坏掉的 `checkin` 文案与 `format.uts` 的 `0s`，都没有任何用例变红）。
 *
 * 判据是**代码级事实**，不是文件名约定、不是注释里提到过什么、也不是作者自报：
 * 文件名叫 `*Behavior.test.js` 而实际只 `expect(src).toContain(...)` 的，照样归接线守护。
 * （本脚本的第一版就栽在这里：`screenshotDiffContract` / `devFinishContract` / `hxRunContract` 等 9 个
 * 文件只在**注释或路径字符串**里出现 `.ps1`，被误判成行为守护 ⇒ 现在要求「可执行调用 + 被执行的载体」。）
 *
 * 用法：
 *   node scripts/classify-guards.mjs                      # 人读的表 + 类别统计
 *   node scripts/classify-guards.mjs --json               # 机读（供 utils/guardClassification.test.js 断言）
 *   node scripts/classify-guards.mjs --class=behavior --files   # 只要一类，只要文件名
 *
 * 退出码：0 = 分类完成；2 = 一个测试文件都没有（fail-closed，不在空集合上判绿）。
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

/**
 * ① 执行判据：文件里有没有**代码级**的「跑起来」调用。
 * 只认代码行，不认注释（先用 stripComments 去注释再匹配）。
 * `[^)]*` 而不是 `['"]child_process['"]`：本仓两种引号都在用。
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

/** 被执行的载体：说明这个「跑起来」跑的是**仓内的东西**，不是自说自话 */
const SUBJECT_RE = /\.(?:uts|uvue|ps1|mjs|json)\b/;

/** 去注释：块注释 + 行注释（`//` 起）。字符串里的 `//` 极少见，本仓的断言文本里没有 http:// 之类 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
}

/** 读取的文件（真实存在于同一模块目录里的 `.uts`）——两类守护都该做的事 */
function readsRealFiles(srcDir, src) {
  const out = new Set();
  const re = /['"]([^'"]*\.(?:uts|uvue|ps1|mjs|json))['"]/g;
  let m;
  while ((m = re.exec(src)) !== null) {
    const p = m[1];
    if (p.includes('.test.')) continue;
    out.add(p);
  }
  return [...out];
}

export function classify(utilsDir) {
  return fs
    .readdirSync(utilsDir)
    .filter((f) => f.endsWith('.test.js'))
    .sort()
    .map((name) => {
      const src = fs.readFileSync(path.join(utilsDir, name), 'utf8');
      const code = stripComments(src);
      const exec = EXECUTION_MARKERS.filter((m) => m.re.test(code)).map((m) => m.why);
      // 分类只看**一件事**：有没有代码级的执行调用。
      //   ⚠️ 不要把「有没有被执行的载体」并进分类条件（2026-09-18 血账）：那样一个**真执行**的文件
      //   一旦失去载体引用，就会**掉进 wiring** 而不是被报成「行为但无载体」—— 于是
      //   `utils/guardClassification.test.js` 的 H3（行为守护必须引用仓内载体）**永远没有反例可抓**，
      //   变成一条恒真的断言（正是本票要消灭的形态）。载体引用由 H3 单独断言。
      const isBehavior = exec.length > 0;
      return {
        name,
        cls: isBehavior ? 'behavior' : 'wiring',
        why: isBehavior ? exec : [],
        // 接线守护也应该在读真源；`refs` 为空 = 既不跑也不读，值得人看一眼（H4 钉住名单）。
        // H3 也读它：行为守护的 `refs` 不得为空 —— 否则「跑起来了」但跑的是自己搭的假货。
        refs: readsRealFiles(utilsDir, code),
      };
    });
}

function main() {
  const argv = process.argv.slice(2);
  const asJson = argv.includes('--json');
  const onlyClass = (argv.find((a) => a.startsWith('--class=')) || '').split('=')[1];
  const filesOnly = argv.includes('--files');

  const utilsDir = path.join(HERE, '..', 'utils');
  const rows = classify(utilsDir);

  if (rows.length === 0) {
    // fail-closed：在空集合上判绿正是本仓防的那件事
    console.log(JSON.stringify({ ok: false, reason: 'no-test-files', utilsDir }));
    process.exit(2);
  }

  const behavior = rows.filter((r) => r.cls === 'behavior');
  const wiring = rows.filter((r) => r.cls === 'wiring');
  const filtered = onlyClass ? rows.filter((r) => r.cls === onlyClass) : rows;

  if (asJson) {
    console.log(
      JSON.stringify(
        {
          ok: true,
          total: rows.length,
          behaviorCount: behavior.length,
          wiringCount: wiring.length,
          behavior: behavior.map((r) => r.name),
          wiring: wiring.map((r) => r.name),
          rows,
        },
        null,
        2
      )
    );
    return;
  }

  if (filesOnly) {
    filtered.forEach((r) => console.log(r.name));
    return;
  }

  console.log('守护分类（真源；判据 = 代码级事实，见本文件头）');
  console.log(`  行为守护 behavior：${behavior.length}  ——**对 ③ 门承重**`);
  console.log(`  接线守护 wiring  ：${wiring.length}  ——防接线被拆，**不构成 ③ 门证据**`);
  console.log('');
  for (const r of filtered) {
    const tag = r.cls === 'behavior' ? '行为' : '接线';
    const why = r.cls === 'behavior' ? ` ← ${r.why.join('；')}` : '';
    console.log(`  [${tag}] ${r.name}${why}`);
  }
  const noRefs = rows.filter((r) => r.refs.length === 0);
  if (noRefs.length > 0) {
    console.log('');
    console.log(`既不起子进程、也不引用仓内载体文件（${noRefs.length} 个，值得人看一眼）：`);
    noRefs.forEach((r) => console.log(`  · ${r.name}`));
  }
}

const isMain = process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href;
if (isMain) main();
