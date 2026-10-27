/**
 * 运行时依赖捕获（#2019 的一次性审计夹具，非门禁路径）
 *
 * 目的：给 `fs.readFileSync` 打桩，记录**套件真实读到的仓内文件**，用来对照
 * `scripts/behavior-suite-deps.mjs` 的静态抽取有没有漏边。
 *
 * 只记 `mobileRoot` 底下的路径（jest 自身与 node_modules 不记）。
 * 输出：`$DEP_CAPTURE_OUT/<套件 basename>.json`
 *
 * ⚠️ 这是研究夹具，不进任何门。生产判据不依赖它 —— 它只用来证伪「静态抽取 vs 真读」。
 */
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.resolve(__dirname, '..', '..');
const OUT = process.env.DEP_CAPTURE_OUT;

if (!OUT) {
  throw new Error('dep-capture: 需要环境变量 DEP_CAPTURE_OUT（输出目录）');
}

const seen = new Set();
const orig = fs.readFileSync;

fs.readFileSync = function patched(file, ...rest) {
  try {
    const abs = typeof file === 'string' ? path.resolve(file) : '';
    if (abs.startsWith(ROOT + path.sep)) seen.add(path.relative(ROOT, abs).replace(/\\/g, '/'));
  } catch {
    /* 记录失败不影响被测行为 */
  }
  return orig.call(this, file, ...rest);
};

afterAll(() => {
  const testPath = (expect.getState() || {}).testPath || 'unknown';
  const key = path.basename(testPath).replace(/\.test\.js$/, '');
  fs.mkdirSync(OUT, { recursive: true });
  // 用原始 readFileSync 以外的方式写，避免把自己写进捕获集
  const target = path.join(OUT, `${key}.json`);
  require('node:fs').writeFileSync(target, JSON.stringify([...seen].sort(), null, 2), 'utf8');
});
