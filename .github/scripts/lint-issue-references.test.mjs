import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import { lintIssueReferences, makeFsFacts, makeGhNumberResolver } from './lib/lint-issue-references.mjs';
import { mergeRefLint } from './preflight-pr.mjs';

// #2053 行为守护：票面引用完整性 lint。夹具**真执行** lint 函数（票/PR 正文 + 注入仓内事实），
// 不打网络、不依赖真实仓；三类漂移（missing-file / adr / issue-number）各钉各的，
// 合规形态（「编号 = 执行时最大号 + 1」、占位示意、白名单外前缀）必须放行；小节号（§6.3 类）显式不判。

const EMPTY_ADR = { root: new Set(), mobile: new Set() };

function makeDeps({ files = [], adr = EMPTY_ADR, numbers = null } = {}) {
  return {
    existsPath: (p) => files.includes(p),
    adrNumbers: (scope) => adr[scope] || new Set(),
    ...(numbers === null ? {} : { resolveNumber: async (n) => numbers.includes(n) }),
  };
}

const byCat = (r, c) => r.reds.filter((x) => x.category === c);

test('切片1：反引号白名单前缀路径不存在 ⇒ missing-file 红；存在 ⇒ 不红', async () => {
  const body = '一手引用 `docs/design/20261024-验收门成本fix与证据复用实践调研.md`，接线 `scripts/preflight-pr.ps1`。';
  const r = await lintIssueReferences(body, makeDeps({ files: ['scripts/preflight-pr.ps1'] }));
  const reds = byCat(r, 'missing-file');
  assert.equal(reds.length, 1, '只判缺失的那个');
  assert.equal(reds[0].ref, 'docs/design/20261024-验收门成本fix与证据复用实践调研.md');
  assert.equal(r.ok, false);
});

test('切片1：行号后缀 \`:35\` 剥掉再判；剥后存在不红', async () => {
  const body = '曾引 `docs/design/20261025-微信门二截图采集方案调研.md:35`。';
  const r = await lintIssueReferences(body, makeDeps({ files: ['docs/design/20261025-微信门二截图采集方案调研.md'] }));
  assert.deepEqual(r.reds, []);
  assert.equal(r.ok, true);
});

test('切片1：目录路径按目录存在判（docs/adr/ 现 47 份 类写法）', async () => {
  const r1 = await lintIssueReferences('目录 `docs/adr/` 现 47 份。', makeDeps({ files: ['docs/adr'] }));
  assert.deepEqual(r1.reds, []);
  const r2 = await lintIssueReferences('目录 `docs/adr/`。', makeDeps({ files: [] }));
  assert.equal(byCat(r2, 'missing-file').length, 1);
});

test('切片1：占位/示意形态不判（<NNNN>、…、通配 *、含空格）', async () => {
  const body = ['模板 `docs/adr/NNNN-中文标题.md`', '省略 `docs/design/…`', '通配 `scripts/*.mjs`', '空格 `docs/design/a b.md`'].join('\n');
  const r = await lintIssueReferences(body, makeDeps({ files: [] }));
  assert.deepEqual(r.reds, [], '示意形态一律放行（NNNN 占位见 adr 切片另钉）');
});

test('切片1：白名单外前缀与外链不判', async () => {
  const body = '约定 `docs/agents/issue-tracker.md`，前端 `frontend/src/x.ts`，外链 `https://example.com/docs/design/nope.md`，裸 `lib/mp-weixin-publish.ps1`。';
  const r = await lintIssueReferences(body, makeDeps({ files: [] }));
  assert.deepEqual(r.reds, []);
});

test('切片2：ADR 号位——裸 ADR-00NN 判根仓号位；有文件不红、无文件红', async () => {
  const adr = { root: new Set(['0043', '0088']), mobile: new Set(['0043']) };
  const ok = await lintIssueReferences('教训见 ADR-0043 决策 8。', makeDeps({ adr }));
  assert.deepEqual(ok.reds, []);
  const bad = await lintIssueReferences('按 ADR-0089 执行。', makeDeps({ adr }));
  const reds = byCat(bad, 'adr');
  assert.equal(reds.length, 1);
  assert.equal(reds[0].ref, '0089');
});

test('切片2：「移动端 ADR-00NN」判移动端号位（独立编号体系，根仓同号不豁免）', async () => {
  const adr = { root: new Set(['0043']), mobile: new Set() };
  const r = await lintIssueReferences('并在移动端 ADR-0043 决策 3 句加指路注。', makeDeps({ adr }));
  assert.equal(byCat(r, 'adr').length, 1, '移动端侧 0043 不在库 ⇒ 红');
});

test('切片2：新 ADR 写死号且号位已被占 ⇒ 红（#1861 形态：写死「0044」，三日内被占）', async () => {
  const adr = { root: new Set(), mobile: new Set(['0044']) };
  const r = await lintIssueReferences('按规划的「新移动端 ADR 0044」落门化判据。', makeDeps({ adr }));
  const reds = byCat(r, 'adr');
  assert.equal(reds.length, 1);
  assert.match(reds[0].message, /已被占用/);
});

test('切片2：合规形态不判红——「编号 = 执行时 docs/adr 最大号 + 1」写法不在捕获面', async () => {
  const adr = { root: new Set(['0043']), mobile: new Set(['0044', '0045', '0046', '0047']) };
  const body = '落**新移动端 ADR**（编号 = 执行时 docs/adr 最大号 + 1，2026-10-27 实测 0044–0047 已占、以现测为准）';
  const r = await lintIssueReferences(body, makeDeps({ adr }));
  assert.deepEqual(r.reds, [], '裸 0044–0047（无 ADR 词头）与不写死号位的写法都不该红');
});

test('切片3：#N 票号在 DriftingLi/FL 解析不到 ⇒ issue-number 红；可解析不红', async () => {
  const body = '前置 #1861 已合并，另引 #9001（不存在）。范围写法 #1845-#1848 都要能解析。';
  const r = await lintIssueReferences(body, makeDeps({ numbers: [1861, 1845, 1846, 1847, 1848] }));
  const reds = byCat(r, 'issue-number');
  assert.equal(reds.length, 1);
  assert.equal(reds[0].ref, '9001');
});

test('切片3：HTML 注释与围栏代码块里的历史票号不进判据（#1770 教训）；§小节号显式不判', async () => {
  const body = ['<!-- 模板残留 #1770 / #883… -->', '详见姊妹调研 §6.3 与 `docs/agents/release.md:13`。', '```', 'gh issue view 9999', '```'].join('\n');
  const r = await lintIssueReferences(body, makeDeps({ numbers: [] }));
  assert.deepEqual(r.reds, [], '注释/代码块/小节号都不该产出红项');
});

test('切片3：hex 色值 #0d9458/#41ba41 形态不误捕为票号', async () => {
  const body = "splitColor [[0.25, '#0d9458'], [0.75, '#41ba41'], [1, '#f43146']]";
  const r = await lintIssueReferences(body, makeDeps({ numbers: [] }));
  assert.deepEqual(byCat(r, 'issue-number'), []);
});

test('血账夹具：#1861 修复前形态 ⇒ 命中 ADR 号位 + 文档不存在两类（≥2 红项），§6.3 仍不判', async () => {
  const preFixBody = [
    '## What to build',
    '1. 按规划的「新移动端 ADR 0044」落证据复用判据；',
    '2. 一手引用 `docs/design/20261024-验收门成本fix与证据复用实践调研.md` §7.2；',
    '3. 口径对齐姊妹调研 §6.3。',
  ].join('\n');
  const adr = { root: new Set(), mobile: new Set(['0044']) };
  const r = await lintIssueReferences(preFixBody, makeDeps({ adr, files: [] }));
  const cats = new Set(r.reds.map((x) => x.category));
  assert.ok(cats.has('adr'), '号位占用要红');
  assert.ok(cats.has('missing-file'), '未入库文档要红');
  assert.ok(r.reds.length >= 2);
  assert.ok(!r.reds.some((x) => x.message.includes('§')), '小节号不许成为红项');
});

test('合规夹具：#2049 形态正文跑绿（执行时最大号+1、已占号陈述、既有门引用都放行）', async () => {
  const ok2049 = [
    '## What to build',
    '1. `.github/scripts/lib/gate-validator.mjs` 门清单加 `{ key: \'⑤\', requires: 运行时面命中且 ② 未命中 }`；',
    '2. PR 证据表加独立行，评论 marker `gate-evidence:⑤`；② 行措辞白名单一字不动；',
    '3. 落**新移动端 ADR**（编号 = 执行时 docs/adr 最大号 + 1，2026-10-27 实测 0044–0047 已占、以现测为准）：',
    '   并在移动端 ADR-0043 决策 3 句加指路注。',
    '## 边界',
    '- 票面引用的规则只引已合库 ADR（issue-tracker 2026-10-24 立规）；前置 #1861 已合并。',
  ].join('\n');
  const deps = makeDeps({
    files: ['docs/adr', 'scripts'],
    adr: { root: new Set(['0043']), mobile: new Set(['0043', '0044', '0045', '0046', '0047']) },
    numbers: [1861],
  });
  const r = await lintIssueReferences(ok2049, deps);
  assert.deepEqual(r.reds.map((x) => x.message), []);
});

test('切片4：makeFsFacts——真实仓根读 docs/adr 号位与目录存在性（本仓现状即事实源）', async () => {
  const root = fileURLToPath(new URL('../../', import.meta.url));
  const facts = makeFsFacts(root);
  assert.ok(facts.adrNumbers('root').has('0043'), '根仓 ADR-0043 在库');
  assert.ok(facts.adrNumbers('mobile').has('0043'), '移动端 0043 在库');
  assert.ok(facts.existsPath('docs/adr'));
  assert.ok(!facts.existsPath('docs/design/不存在的文件.md'));
  const r = await lintIssueReferences('见 `docs/adr/` 与 ADR-0043、移动端 ADR-0043。', facts);
  assert.deepEqual(r.reds, []);
});

test('切片4：makeGhNumberResolver——注入 exec 桩，404 ⇒ false、命中 ⇒ true、复用同号只查一次', async () => {
  let calls = 0;
  const run = (cmd, args) => {
    calls += 1;
    if (String(args[1]).endsWith('/9001')) { const e = new Error('HTTP 404: Not Found'); e.status = 404; throw e; }
    return JSON.stringify({ number: Number(String(args[1]).split('/').pop()) });
  };
  const resolve = makeGhNumberResolver('DriftingLi/FL', { run });
  assert.equal(await resolve(9001), false);
  assert.equal(await resolve(1861), true);
  assert.equal(await resolve(1861), true);
  assert.equal(calls, 2, '缓存：同号不重查');
});

test('切片5：mergeRefLint——红项并进 errors、ok 联动、notes 透传；gates 原样不动（不新增门）', () => {
  const base = { ok: true, errors: [], notes: ['原注'], gates: { '①': { bound: true }, '③': { required: false } } };
  const lint = {
    ok: false,
    reds: [{ category: 'adr', ref: '0044', message: '新 移动端 ADR 0044 写死号已被占用（合规写法：编号 = 执行时 docs/adr 最大号 + 1）' }],
    notes: ['未注入 resolveNumber，跳过票号可解析判据'],
  };
  const m = mergeRefLint(base, lint);
  assert.equal(m.ok, false);
  assert.equal(m.errors.length, 1);
  assert.match(m.errors[0], /^引用漂移 · 新 移动端 ADR 0044/);
  assert.deepEqual(m.notes, ['原注', '未注入 resolveNumber，跳过票号可解析判据']);
  assert.deepEqual(m.gates, base.gates, '门清单不许被动');
  const m2 = mergeRefLint(base, { ok: true, reds: [], notes: [] });
  assert.equal(m2.ok, true);
  assert.deepEqual(m2.errors, [], 'lint 全绿时原证据结论不变');
});
