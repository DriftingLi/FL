/**
 * ③ 门单测配置。`-i`（串行）语义的现值记录（#1410，2026-09-30 评估）：
 * - 共享资源面干净：146 套件无 temp/端口/git/env 跨套件竞争（temp 写全落 mkdtempSync
 *   唯一目录；端口 listen(0) ephemeral；git 只读 `worktree list` 或临时仓库内操作）。
 *   `git log -S` 考古：`-i` 系 #494 脚手架初始设定一次性铺给全部 jest 脚本，非压 flaky。
 * - `-i` 实际在挡的是**CPU 争抢下的 5s 默认超时抖动**：loadUts 系套件在测试体内同步
 *   转译 .uts，jest 并行（workers≈核数）偶发超时红（12 核实测 5 次并行出现 1 次，
 *   见 #1410 评论勘误）。红的是墙钟不是逻辑，单套件重跑即绿。
 * - CI `mobile-test`（2 核 ubuntu-latest）⇒ workers≈1，本就无收益且有 thrash 风险
 *   ⇒ ③ 门与 CI 继续 `-i`（稳、可复现）。
 * - 本地多核内循环用 `npm run test:unit:local` / `npm run test:behavior:local`（并行，
 *   偶发墙钟红按门失败原样重跑一次即可）。
 *
 * ---- ③ 门拆腿（#1702，2026-10-18）----
 * 本文件此前 `testMatch` 是 utils 下的全量 glob（`*.test.[jt]s?(x)`，含子目录），于是**名字（unit）与实际内容
 * （半数是行为守护）不符**：单套件最长静默曾达 10 分钟、整轮零进度输出 ⇒「静默」被反复读成
 * 「卡死」（≥3 次，见 #1702）。现按 `scripts/lib/guard-split.js` 的**同一条判据**
 * （= `docs/agents/guards.md` 的守护分类）切成两腿：
 *   · 本文件（`test:unit`）= **接线守护** —— 读源码文本断言调用点 / 常量 / 顺序，不起子进程、不跑 .uts；
 *   · `jest.config.behavior.js`（`test:behavior`）= **行为守护** —— 现场执行被测面（跑 pwsh、utsHarness 真跑 .uts）。
 * 判据仍只有一份：两腿的 testMatch 都由 `splitSuites()` 现算，没有第二份手写清单（血账 #974 的形态）。
 * 单跑某一个套件时要选对 `--config`（套件落在哪一腿由 `node scripts/classify-guards.mjs` 回答）。
 */

const path = require('node:path');
const { splitSuites, toTestMatch } = require('./scripts/lib/guard-split.js');

const { wiring } = splitSuites(path.join(__dirname, 'utils'));

module.exports = {
  testEnvironment: 'node',
  // 接线守护（第二腿见 jest.config.behavior.js）。glob 由判据现算 ⇒ 新增套件自动落进该落的那一腿。
  testMatch: toTestMatch(wiring),
  testPathIgnorePatterns: ['/node_modules/'],
  moduleFileExtensions: ['js', 'json']
};
