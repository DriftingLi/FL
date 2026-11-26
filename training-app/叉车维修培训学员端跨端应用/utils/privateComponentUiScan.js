/**
 * 页面私有子件层扫描器（#2866，判据见 docs/design/20261125-页面私有子件层判据.md §8/§10/§11）。
 *
 * 口径（与 T4 #2835 实测账本一致，2026-11-26 复现）：
 * - 私有子件 = `pages/<族>/components/**\/*.uvue`（路径定义层，不看命名）；
 * - 只扫 `<style>` 块内裸 hex；先剥 `/* *\/` 与整行 `//`（二者是 SCSS 注释，不是字面值）；
 * - hex 正则钉 3/6/8 位并加「后一位不是 hex」断言——4 位是票号（#2520），历史曾误计 631 处；
 * - 裸 hex ∈ uni.scss token 值 ∪ utils/uiHexWhitelist.js，否则算违规；
 * - 违规按文件计数做两向棘轮（基线只减不增，新文件缺省 0）；
 * - 同族封闭：私有件的 import 方必须同族，且导入者必须在 `pages/` 下（否则是跨层消费）。
 *
 * 纯函数 + 只读文件系统，供接线套件与基线生成脚本复用（基线禁止手抄）。
 */
const fs = require('fs');
const path = require('path');

const PRIVATE_REL_RE = /^pages\/[^/]+\/components\/.+\.uvue$/;
const SKIP_DIRS = new Set(['node_modules', 'unpackage', 'uni_modules', 'docs']);
const HEX_RE = /#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3})(?![0-9a-fA-F])/g;
const IMPORT_RE = /from\s+['"]([^'"]+\.uvue)['"]/g;
const STYLE_RE = /<style([^>]*)>([\s\S]*?)<\/style>/g;

function normalizeHex(raw) {
  const value = String(raw).replace(/^#/, '').toUpperCase();
  if (value.length === 3) return value.split('').map((c) => c + c).join('');
  return value;
}

function readSource(absPath) {
  return fs.readFileSync(absPath, 'utf8').replace(/\r\n/g, '\n');
}

function toPosix(rel) {
  return rel.split(path.sep).join('/');
}

/** 递归收集 `.uvue`，rel 路径为 app 根相对的 POSIX 形式。 */
function walkUvue(appRoot, dir, acc) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (entry.name.startsWith('.') || SKIP_DIRS.has(entry.name)) continue;
      walkUvue(appRoot, path.join(dir, entry.name), acc);
      continue;
    }
    if (!entry.name.endsWith('.uvue')) continue;
    acc.push(toPosix(path.relative(appRoot, path.join(dir, entry.name))));
  }
  return acc;
}

function collectUvue(appRoot, dir = appRoot) {
  return walkUvue(appRoot, dir, []);
}

function extractStyleBlocks(source) {
  const blocks = [];
  for (const match of source.matchAll(STYLE_RE)) {
    blocks.push({ attrs: match[1], body: match[2] });
  }
  return blocks;
}

function stripStyleComments(body) {
  return body.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^[ \t]*\/\/.*$/gm, '');
}

function hasScssLang(attrs) {
  const match = attrs.match(/lang\s*=\s*["']?([\w-]+)/);
  return Boolean(match) && match[1].toLowerCase() === 'scss';
}

function hexLiterals(text) {
  const found = [];
  for (const match of text.matchAll(HEX_RE)) found.push(match[1]);
  return found;
}

/** 把字面值分成 token / 白名单 / 违规三堆（入参均为归一后无 `#` 的大写值集合）。 */
function classifyHex(literals, tokenValues, whitelistValues) {
  const result = { token: 0, whitelist: 0, violations: [] };
  for (const literal of literals) {
    const value = normalizeHex(literal);
    if (tokenValues.has(value)) result.token += 1;
    else if (whitelistValues.has(value)) result.whitelist += 1;
    else result.violations.push(`#${value}`);
  }
  return result;
}

/** uni.scss 里直接声明为 hex 的 token 值（别名 / 变量引用不展开，与 T4 账本同口径）。 */
function loadTokenValues(appRoot) {
  const values = new Set();
  for (const line of readSource(path.join(appRoot, 'uni.scss')).split('\n')) {
    const declaration = line.match(/^\s*\$[\w-]+\s*:\s*([^;]+);/);
    if (!declaration) continue;
    for (const literal of hexLiterals(declaration[1])) values.add(normalizeHex(literal));
  }
  return values;
}

function loadWhitelistValues(appRoot) {
  const table = require(path.join(appRoot, 'utils', 'uiHexWhitelist.js'));
  return new Set(Object.keys(table).map((key) => normalizeHex(key)));
}

function scanPrivateComponents(appRoot) {
  const tokenValues = loadTokenValues(appRoot);
  const whitelistValues = loadWhitelistValues(appRoot);
  const privateRels = collectUvue(appRoot, path.join(appRoot, 'pages'))
    .filter((rel) => PRIVATE_REL_RE.test(rel))
    .sort();

  const files = privateRels.map((rel) => {
    const source = readSource(path.join(appRoot, rel));
    const blocks = extractStyleBlocks(source);
    const langMissing = blocks.filter((block) => !hasScssLang(block.attrs)).length;
    const literals = blocks.flatMap((block) => hexLiterals(stripStyleComments(block.body)));
    const classified = classifyHex(literals, tokenValues, whitelistValues);
    return {
      rel,
      family: rel.split('/')[1],
      styleBlocks: blocks.length,
      langMissing,
      hexCount: literals.length,
      tokenCount: classified.token,
      whitelistCount: classified.whitelist,
      violations: classified.violations
    };
  });

  const totals = {
    files: files.length,
    styleBlocks: files.reduce((sum, file) => sum + file.styleBlocks, 0),
    langMissing: files.reduce((sum, file) => sum + file.langMissing, 0),
    hexCount: files.reduce((sum, file) => sum + file.hexCount, 0),
    tokenCount: files.reduce((sum, file) => sum + file.tokenCount, 0),
    whitelistCount: files.reduce((sum, file) => sum + file.whitelistCount, 0),
    violationCount: files.reduce((sum, file) => sum + file.violations.length, 0)
  };

  return { files, privateRels, tokenValues, whitelistValues, totals };
}

/** 逐文件违规计数 → 排序后的基线对象（0 值不入表）。 */
function ratchetBaseline(scanResult) {
  const baseline = {};
  for (const file of scanResult.files) {
    if (file.violations.length > 0) baseline[file.rel] = file.violations.length;
  }
  return Object.fromEntries(Object.entries(baseline).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
}

/** 两向棘轮：实际 > 基线 = 新增；基线 > 实际 = 清干净了但基线没跟。 */
function ratchetFindings(actual, baseline) {
  const findings = [];
  const rels = new Set([...Object.keys(actual), ...Object.keys(baseline)]);
  for (const rel of [...rels].sort()) {
    const now = actual[rel] || 0;
    const allowed = baseline[rel] || 0;
    if (now > allowed) findings.push(`${rel}: 实测 ${now} 处 > 基线 ${allowed} 处（新增裸 hex，须改回 token 或白名单）`);
    if (allowed > now) findings.push(`${rel}: 基线 ${allowed} 处 > 实测 ${now} 处（已清干净，须同步下调基线）`);
  }
  return findings;
}

function resolveImportTarget(importerRel, spec) {
  const importerDir = path.posix.dirname(importerRel);
  if (spec.startsWith('@/')) return path.posix.normalize(spec.slice(2));
  if (spec.startsWith('.')) return path.posix.normalize(path.posix.join(importerDir, spec));
  return null;
}

/** 同族封闭：返回 findings 与 import 边数（边数用于反重言活性断言）。 */
function familyClosureFindings(appRoot, privateRels) {
  const privateSet = new Set(privateRels);
  const findings = [];
  let edges = 0;
  for (const rel of collectUvue(appRoot)) {
    const source = readSource(path.join(appRoot, rel));
    for (const match of source.matchAll(IMPORT_RE)) {
      const target = resolveImportTarget(rel, match[1]);
      if (!target || !privateSet.has(target)) continue;
      edges += 1;
      const targetFamily = target.split('/')[1];
      const importerFamily = rel.startsWith('pages/') ? rel.split('/')[1] : null;
      if (importerFamily === targetFamily) continue;
      const where = importerFamily === null ? `来源不在 pages/ 下（${rel}）` : `跨族（${importerFamily} → ${targetFamily}）`;
      findings.push(`${target} 被 ${where} 导入：${rel}`);
    }
  }
  return { findings, edges };
}

/** 白名单死条自检：每个在册值都必须在全层（pages + components 的 .uvue style 块）真实出现（#2862 起 55 值）。 */
function whitelistDeadEntries(appRoot, whitelistValues) {
  const used = new Set();
  for (const rel of collectUvue(appRoot)) {
    for (const block of extractStyleBlocks(readSource(path.join(appRoot, rel)))) {
      for (const literal of hexLiterals(stripStyleComments(block.body))) used.add(normalizeHex(literal));
    }
  }
  return [...whitelistValues].filter((value) => !used.has(value)).sort().map((value) => `#${value}`);
}

module.exports = {
  PRIVATE_REL_RE,
  normalizeHex,
  collectUvue,
  extractStyleBlocks,
  stripStyleComments,
  hasScssLang,
  hexLiterals,
  classifyHex,
  loadTokenValues,
  loadWhitelistValues,
  scanPrivateComponents,
  ratchetBaseline,
  ratchetFindings,
  familyClosureFindings,
  whitelistDeadEntries
};
