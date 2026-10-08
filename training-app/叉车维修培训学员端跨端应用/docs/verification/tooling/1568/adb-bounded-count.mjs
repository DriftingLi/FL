#!/usr/bin/env node
/**
 * 「一次 adb 调用有没有单次超时」的**复算尺**（#1568 票面 AC 最后一条：开票与收票用同一把尺）
 *
 * 为什么要有这个文件（票面「现测」段那条方法说明的可执行版）：
 *   #1562 收口时那句「每次 adb 调用都有单次超时」对整脚本不成立，而**登记票自己也飘过两次**
 *   （裸 grep 把块注释里的反面教材数成调用点；只数两条门脚本漏了三个文件）。
 *   ⇒ 结论：任何「还剩 N 处」的报价必须由**与契约守护同一套剥注释法**跑出来，且可复算。
 *
 * 三条判据（每条都是本仓实际飘过的地方，实现里逐条对应）：
 *   1. 先删 `<# … #>` 块、再删整行 `#` —— 与 `deviceCaptureContract` / `emulatorSmokeContract`
 *      的 `replace(/<#[\s\S]*?#>/g,'').replace(/^\s*#.*$/gm,'')` 同一把尺；掩码时**保留行号**
 *      （`wirelessDebugContract.maskDocBlocks` 的做法），否则报出来的行号对不上原文。
 *   2. 「所在函数」按**大括号深度**归属，不能拿「最后见到的一行 function」当归属 ——
 *      后者会把函数闭合之后的顶层脚本行算进那个函数名下（#1568 第一版就这样把 3 行记错了）。
 *      数括号前把**字符串字面量内部**掩掉，`"a{b"` 不算深度。
 *   3. 调用点特征取 `&\s*\$[aA]db[eE]xe`，**不**用 `Out-String` 当特征 ——
 *      `emulator-smoke.ps1` 那批直调里有 5 处是 `| Out-Null`，拿它当尺会少数。
 *
 * 用法：node docs/verification/tooling/1568/adb-bounded-count.mjs [scripts 目录]
 * 输出：每文件一行机读 `ADB_UNBOUNDED file=… calls=… funcs=…` + 逐函数明细 + 合计行。
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
// 本文件在 <项目根>/docs/verification/tooling/1568/ ⇒ 上溯四层才是项目根（scripts/ 在那儿）
const ROOT = path.join(__dirname, '..', '..', '..', '..');
const SCRIPTS_DIR = process.argv[2] ? path.resolve(process.argv[2]) : path.join(ROOT, 'scripts');

/** 掩掉块注释与整行注释，**行号不变**（与两条门脚本守护同一把尺：先 `<#…#>`、再整行 `#`）。 */
function maskComments(text) {
  const src = text.replace(/\r\n/g, '\n');
  const chars = src.split('');
  const blockRe = /<#[\s\S]*?#>/g;
  let m;
  while ((m = blockRe.exec(src)) !== null) {
    for (let i = m.index; i < m.index + m[0].length; i++) {
      if (chars[i] !== '\n') chars[i] = ' ';
    }
  }
  return chars.join('').split('\n').map((l) => (/^[ \t]*#/.test(l) ? '' : l));
}

/** 把字符串字面量的**内容**掩掉（引号留着），只用于数大括号深度。 */
function maskStringsForDepth(line) {
  return line
    .replace(/'([^']*)'/g, (_all, inner) => "'" + inner.replace(/[^']/g, ' ') + "'")
    .replace(/"([^"]*)"/g, (_all, inner) => '"' + inner.replace(/[^"]/g, ' ') + '"');
}

/** 一行的深度增量与「它在深度 0 上登记了哪个函数名」。 */
function depthDelta(line) {
  const s = maskStringsForDepth(line);
  let delta = 0;
  for (const ch of s) {
    if (ch === '{') delta++;
    else if (ch === '}') delta--;
  }
  return delta;
}

const CALL_RE = /&\s*\$[aA]db[eE]xe/;
const FUNC_RE = /^[ \t]*function[ \t]+([A-Za-z0-9_.-]+)/;

function scanFile(file) {
  const lines = maskComments(fs.readFileSync(file, 'utf8'));
  let depth = 0;
  let current = null;
  const hits = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const depthBefore = depth;
    const fn = depthBefore === 0 ? FUNC_RE.exec(line) : null;
    if (fn) current = fn[1];
    if (CALL_RE.test(line)) {
      // 归属：深度 0 上的函数头行算它自己；有当前函数名算那个函数；**其余一律算顶层脚本段**
      // （顶层多行结构如 `@{ … }` 散在多行时深度也 >0，这时 current 是 null —— 别把它记成上一个函数）
      const owner = current || '<top-level>';
      hits.push({ line: i + 1, owner });
    }
    depth += depthDelta(line);
    if (depth <= 0) { depth = Math.max(depth, 0); current = null; }
  }
  return hits;
}

const targets = fs.readdirSync(SCRIPTS_DIR, { recursive: true })
  .filter((p) => p.endsWith('.ps1'))
  .map((p) => path.join(SCRIPTS_DIR, p))
  .filter((p) => !p.includes(`${path.sep}node_modules${path.sep}`))
  .sort();

const perFile = [];
for (const file of targets) {
  const hits = scanFile(file);
  if (!hits.length) continue;
  const byFn = {};
  hits.forEach((h) => { byFn[h.owner] = (byFn[h.owner] || 0) + 1; });
  perFile.push({ rel: path.relative(ROOT, file).split(path.sep).join('/'), hits, byFn });
}

const total = perFile.reduce((n, f) => n + f.hits.length, 0);
perFile.forEach((f) => {
  const funcs = Object.keys(f.byFn).length;
  console.log(`ADB_UNBOUNDED file=${f.rel} calls=${f.hits.length} funcs=${funcs}`);
  Object.keys(f.byFn).forEach((k) => {
    const rows = f.hits.filter((h) => h.owner === k).map((h) => h.line).join(',');
    console.log(`  FUNC name=${k} calls=${f.byFn[k]} lines=${rows}`);
  });
});
console.log(`ADB_UNBOUNDED_TOTAL calls=${total} files=${perFile.length}`);
