/**
 * ③ 门第二腿：**行为守护**（读取者：`npm run test:behavior`）。
 *
 * 与第一腿（`jest.config.unit.js`）的分工见该文件头「③ 门拆腿」段。这一腿是 ③ 门里**承重**的那半：
 * 只有它会因为「算法被改坏」而红（接线守护在「字面量还在」时不红，见 `docs/agents/guards.md`）。
 *
 * 为什么带 `-i` 串行：同一套判据（#1410）—— 这一腿里 loadUts 系套件在并行下会因 CPU 争抢撞 5s 默认超时。
 * 串行 + 单套件最长可达数分钟 ⇒ **整轮静默与卡死不可区分**（#1656 的误判链、本票 #1702）。
 * 故挂一个自定义 reporter 打**逐套件心跳行**（`JEST_SUITE i=N/total ...`）：
 * 它不改任何判据、不改超时、不动 `-i`，只把「进度」这个事实写进终端与日志。
 */
const path = require('node:path');
const { splitSuites, toTestMatch } = require('./scripts/lib/guard-split.js');

const { behavior } = splitSuites(path.join(__dirname, 'utils'));

module.exports = {
  ...require('./jest.config.unit.js'),
  testMatch: toTestMatch(behavior),
  reporters: ['default', '<rootDir>/scripts/jest-reporter-suite-progress.js'],
};