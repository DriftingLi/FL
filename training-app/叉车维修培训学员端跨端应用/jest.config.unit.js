/**
 * ③ 门单测配置。`-i`（串行）语义的现值记录（#1410，2026-09-30 评估）：
 * - 共享资源面干净：146 套件无 temp/端口/git/env 跨套件竞争（temp 写全落 mkdtempSync
 *   唯一目录；端口 listen(0) ephemeral；git 只读 `worktree list` 或临时仓库内操作）。
 *   `git log -S` 考古：`-i` 系 #494 脚手架初始设定一次性铺给全部 jest 脚本，非压 flaky。
 * - `-i` 实际在挡的是**CPU 争抢下的 5s 默认超时抖动**：loadUts 系套件在测试体内同步
 *  转译 .uts，jest 并行（workers≈核数）偶发超时红（12 核实测 5 次并行出现 1 次，
 *   见 #1410 评论勘误）。红的是墙钟不是逻辑，单套件重跑即绿。
 * - CI `mobile-test`（2 核 ubuntu-latest）⇒ workers≈1，本就无收益且有 thrash 风险
 *   ⇒ ③ 门与 CI 继续 `npm run test:unit`（`-i`，稳、可复现）。
 * - 本地多核内循环用 `npm run test:unit:local`（并行，实测 ~60-90s vs 串行 ~236s），
 *   偶发墙钟红按门失败原样重跑一次即可。
 */
module.exports = {
  testEnvironment: 'node',
  testMatch: ['<rootDir>/utils/**/*.test.[jt]s?(x)'],
  testPathIgnorePatterns: ['/node_modules/'],
  moduleFileExtensions: ['js', 'json']
};