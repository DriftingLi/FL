#!/usr/bin/env node
/**
 * 生成 docs/2332-断言处置表.md（#2332 的产出物）。
 *
 * 数据源只有两处，避免「文档漂移」：
 *   ① utils/forumContract.test.js 源码 —— 用例顺序、行号、断言形态由 extractCases 现抽；
 *   ② utils/forumContractAudit.js 的 AUDIT —— 逐条 kind / verdict / note（人工判读的结论）。
 * 两者行键 = desc::title，1:1 覆盖由 forumContract.test.js 末组元守护锁住。
 *
 * 用法（在移动端项目根）：node scripts/gen-2332-assertion-audit.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const CASE_FILE = path.join(ROOT, 'utils', 'forumContract.test.js');
const OUT_FILE = path.join(ROOT, 'docs', '2332-断言处置表.md');
const TICK = String.fromCharCode(96);

const { AUDIT, extractCases } = require(path.join(ROOT, 'utils', 'forumContractAudit.js'));

const src = fs.readFileSync(CASE_FILE, 'utf8').replace(/\r\n/g, '\n');
const lines = src.split('\n');
// 末组「断言分类登记」是本表自己的元守护（4 条），不属于被处置的断言 ⇒ 排除；
// 但保留它在全量列表里的位置，用来切「上一条用例源码块的结尾」。
const META_DESC = '断言分类登记';
const allCases = extractCases(src);
const entries = allCases.map((c, i) => ({ c, i })).filter((e) => !e.c.desc.includes(META_DESC));
const cases = entries.map((e) => e.c);

if (cases.length !== 80) {
  console.error('[2332] 声明单元应为 80，现抽到 ' + cases.length + '（已排除元守护组）；先对齐 utils/forumContractAudit.js');
  process.exit(1);
}

const auditByKey = new Map();
for (const row of AUDIT) auditByKey.set(row.desc + '::' + row.title, row);

function blockOf(index) {
  const pos = entries[index].i;
  const start = cases[index].line - 1;
  const endAt = pos + 1 < allCases.length ? allCases[pos + 1].line - 1 : lines.length;
  return lines.slice(start, endAt).join('\n');
}

const MATCHERS = /\.(not\.)?(toContain|toMatch|toEqual|toStrictEqual|toBeTruthy|toBeFalsy|toBeLessThanOrEqual|toBeGreaterThanOrEqual|toBeLessThan|toBeGreaterThan|toBeNull|toBeUndefined|toBeDefined|toBe|toHaveLength|toHaveProperty|toMatchObject|toHaveBeenCalledTimes|toHaveBeenCalledWith)\b/g;
function shapeOf(index) {
  const seen = [];
  for (const m of blockOf(index).matchAll(MATCHERS)) {
    const name = (m[1] ? 'not.' : '') + m[2];
    if (!seen.includes(name)) seen.push(name);
  }
  return seen.length ? seen.join(' ') : '（无 matcher）';
}

/** 这条断言盯的是哪个源文件（源码块里第一处形如 pages/… 或 composables/… 的字符串字面量） */
const LOCK_TARGET = /['"]([A-Za-z0-9_.\/-]+\.(?:uvue|uts|ts|js))['"]/;
function lockOf(index) {
  const m = blockOf(index).match(LOCK_TARGET);
  return m ? m[1] : '—';
}

function cell(s) {
  return String(s == null ? '' : s).replace(/\|/g, '\\|').replace(/\n/g, ' ');
}

function landing(r) {
  if (r.verdict === 'rewrite-behavior') return '接线锁（本文件）+ 行为判据（utils/forumReplyLikeBehavior.test.js）';
  if (r.verdict === 'retitle-wiring') return '接线锁（本文件），标题不再自称行为';
  return '接线锁（本文件）';
}

const rows = cases.map((c, i) => {
  const r = auditByKey.get(c.desc + '::' + c.title);
  if (!r) {
    console.error('[2332] 登记表缺项：' + c.desc + '::' + c.title);
    process.exit(1);
  }
  const kindLabel = c.kind === 'each' ? 'it.each' : 'it';
  return '| ' + (i + 1) + ' | ' + cell(c.desc) + ' | ' + cell(c.title) + ' | L' + c.line + ' (' + kindLabel + ') | '
    + cell(shapeOf(i)) + ' | ' + cell(lockOf(i)) + ' | ' + r.kind + ' | ' + r.verdict + ' | ' + cell(landing(r)) + ' | ' + cell(r.note) + ' |';
});

const stat = (pred) => cases.filter((c, i) => pred(auditByKey.get(c.desc + '::' + c.title), c, i)).length;
const nEach = cases.filter((c) => c.kind === 'each').length;
const nWiring = stat((r) => r.kind === 'wiring');
const nClaim = stat((r) => r.kind === 'behavior-claim');
const nKeep = stat((r) => r.verdict === 'keep-wiring');
const nRetitle = stat((r) => r.verdict === 'retitle-wiring');
const nRewrite = stat((r) => r.verdict === 'rewrite-behavior');
const descs = [...new Set(cases.map((c) => c.desc))];

const out = [
  '# #2332 断言处置表 · forumContract.test.js（接线 / 自称行为 两分法）',
  '',
  '> 生成物：' + TICK + 'node scripts/gen-2332-assertion-audit.mjs' + TICK + ' 现抽源码与登记表，**勿手改本文件**；改断言先改 ' + TICK + 'utils/forumContract.test.js' + TICK + ' 与 ' + TICK + 'utils/forumContractAudit.js' + TICK + '，再重跑生成。',
  '> 元守护：' + TICK + 'forumContract.test.js' + TICK + ' 末组「断言分类登记」锁住 1:1 覆盖、取值受控与本表读数。',
  '',
  '## 1. 判据（A 口径，2026-11-07 裁定）',
  '',
  '对每条断言问两问：',
  '',
  '1. **把被测行为改坏，它会红吗？** 不红 ⇒ 假绿（#1713 型）。',
  '2. **把实现等价改写（改写法、不改行为），它会红吗？** 红了 ⇒ 锁写法。',
  '',
  '裁定：**判据 ② 只施加于「自称验证行为」的断言**。接线锁（职责只有一条：声明还在 / 调用点还在 / 字面量还在）不按判据 ② 判，只要求它**不冒充行为证据**。故接线族保留，但必须显式标注，且不构成 ③ 门证据（见 ' + TICK + 'docs/agents/guards.md' + TICK + '）。',
  '',
  '三种处置取值：',
  '',
  '| 处置 | 含义 |',
  '| --- | --- |',
  '| ' + TICK + 'keep-wiring' + TICK + ' | 保留接线锁，断言与标题都不动 |',
  '| ' + TICK + 'retitle-wiring' + TICK + ' | 保留断言，标题改成不再自称行为 |',
  '| ' + TICK + 'rewrite-behavior' + TICK + ' | 行为判据移出到行为腿（真跑 ' + TICK + '.uts' + TICK + '），此处只留调用点锁 |',
  '',
  '## 2. 读数（本次现测）',
  '',
  '| 指标 | 值 |',
  '| --- | --- |',
  '| 声明单元（' + TICK + 'it(' + TICK + ' 76 + ' + TICK + 'it.each(' + TICK + ' 4） | **' + cases.length + '** |',
  '| 运行时用例（' + TICK + 'it.each' + TICK + ' 展开 8+4） | **' + (cases.length - nEach + 12) + '** |',
  '| describe 分组 | ' + descs.length + ' |',
  '| 类别 wiring / behavior-claim | ' + nWiring + ' / ' + nClaim + ' |',
  '| 处置 keep / retitle / rewrite | ' + nKeep + ' / ' + nRetitle + ' / ' + nRewrite + ' |',
  '',
  '分组清单：' + descs.map((d) => TICK + d + TICK).join(' · ') + '。',
  '',
  '> 票面写「覆盖全部 76 用例」—— 76 指 ' + TICK + 'it(' + TICK + ' 声明；另有 4 条 ' + TICK + 'it.each' + TICK + '（分别展开 8 与 4）同样是声明单元，故本表按 **80 条声明 / 88 条运行时用例** 逐条登记，取值只多不少。',
  '',
  '## 3. 处置表（逐条）',
  '',
  '| # | describe | 用例标题 | 源码位置 | 断言形态 | 锁对象 | 类别 | 处置 | 判据落点 | 备注 |',
  '| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |',
  ...rows,
  '',
  '## 4. 行为债（本次未还，已登记）',
  '',
  '以下断言仍是接线锁，但背后有**真行为**待钉；它们不冒充行为证据，也不进 ③ 门读数：',
  '',
  '- **回复分页三条**（' + TICK + 'detail：分页态…' + TICK + ' / ' + TICK + 'loadDetail 以第 1 页替换整列' + TICK + ' / ' + TICK + 'loadMoreReplies 按 hasMore 追加并去重' + TICK + '）：真实语义（首屏替换、追加去重、到底不再请求）目前**无行为套件**；' + TICK + 'utils/pagedListBehavior.test.js' + TICK + ' 覆盖的是另一层纯函数，不能替代。三条已改写标题为「声明仍在」，不再自称验证行为。',
  '- **头像点击 → 个人动态**：属 ' + TICK + '.uvue' + TICK + ' 模板路由，node 侧不可达，归入 ①a 发版冒烟（移动端 ADR-0043 固定人工场景清单），本表只保留去重缩进锁。',
  '',
  '## 5. 与行为腿的分工（别混）',
  '',
  '- 本文件（接线腿）：回答「壳层还在调这些函数吗、字面量还在吗」。**依赖断了它挡得住**，实现改坏它挡不住。',
  '- ' + TICK + 'utils/forumReplyLikeBehavior.test.js' + TICK + '（行为腿）：回答「回复点赞到底怎么算」。它自己搭依赖，接线断了它照样绿；判据由文件末组「成对取证」注入变异自证（3 条必红）。',
  '- #1713 的回归钉子只有一处：行为腿里「本地存储残留 ' + TICK + 'forum_reply_like_<id>' + TICK + ' 键不影响判定」，非 ' + TICK + 'toContain' + TICK + ' 负向文本锁。',
  '',
].join('\n');

fs.writeFileSync(OUT_FILE, out, 'utf8');
console.log('[2332] 已写 ' + path.relative(ROOT, OUT_FILE) + ' · 声明 ' + cases.length + ' / 运行时 ' + (cases.length - nEach + 12)
  + ' · wiring ' + nWiring + ' / claim ' + nClaim + ' · keep ' + nKeep + ' / retitle ' + nRetitle + ' / rewrite ' + nRewrite);
