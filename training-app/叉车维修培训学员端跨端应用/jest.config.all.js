/**
 * 「全部套件」配置（#1702）—— **不是 ③ 门**，也不是第三份清单。
 *
 * `testMatch` 逐字保留**拆腿前** `jest.config.unit.js` 的那一条 glob：它只服务**按 `--testPathPattern`
 * 收窄**的跑法（`scripts/dev-finish.ps1` 步骤 3、`scripts/lib/test-compile.ps1`，以及
 * `utils/contractTestPatternBehavior.test.js` 里那条「pattern 必须覆盖到套件」的现测）。
 * 那份 pattern 是横跨两腿的契约子集，收窄到任一腿都会**静默漏掉另一半** ——
 * jest 对 testMatch 之外的套件当不存在，红的是「0 个套件」而不是「跑错了」，那是本仓最怕的假绿形态。
 *
 * ③ 门的判据仍是两腿（`npm run test:unit` + `npm run test:behavior`，或入口 `npm run test:unit:gate`）。
 */
module.exports = {
  ...require('./jest.config.unit.js'),
  testMatch: ['<rootDir>/utils/**/*.test.[jt]s?(x)'],
};
