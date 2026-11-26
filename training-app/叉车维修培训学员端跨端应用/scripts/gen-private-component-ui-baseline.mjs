/**
 * 生成 utils/privateComponentUiBaseline.json（#2866）。
 *
 * 基线必须由扫描器产出、禁止手抄（判据笔记 §11）：本脚本与接线套件共用
 * utils/privateComponentUiScan.js 的同一份口径，只写有违规的文件、键排序、0 值删条目。
 *
 * 用法：node scripts/gen-private-component-ui-baseline.mjs
 */
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const APP_ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
const scan = require(path.join(APP_ROOT, 'utils', 'privateComponentUiScan.js'));

const result = scan.scanPrivateComponents(APP_ROOT);
const baseline = scan.ratchetBaseline(result);
const target = path.join(APP_ROOT, 'utils', 'privateComponentUiBaseline.json');
fs.writeFileSync(target, `${JSON.stringify(baseline, null, 2)}\n`);

console.log(
  `PRIVATE_COMPONENT_UI_BASELINE_RESULT files=${result.totals.files} hex=${result.totals.hexCount} ` +
    `token=${result.totals.tokenCount} whitelist=${result.totals.whitelistCount} ` +
    `violations=${result.totals.violationCount} baselineFiles=${Object.keys(baseline).length} out=${path.relative(APP_ROOT, target)}`
);
