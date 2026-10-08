// 并集上界：S1 里「命令是全量面、但汇总行没进上下文」的跑，能否用落盘补上
import { readFileSync } from 'node:fs';
const rd = (p) => { const t = readFileSync(p, 'utf8').split('\n').filter(Boolean); const h = t.shift(); return t.map((l) => { const c = l.split('\t'); const o = {}; h.split('\t').forEach((k, i) => (o[k] = c[i])); return o; }); };
const v4 = rd('D:/FL/.scratch/research/1545-runs-v4.tsv');
const s2 = rd('D:/FL/.scratch/research/1545-s2v2.tsv');
const s1 = rd('D:/FL/.scratch/research/1545-s1-enriched.tsv');
const WIN_A = Date.parse('2026-09-22T07:48:39.133Z'), WIN_B = Date.parse('2026-10-04T16:21:59.177Z');
// 已被并集下界用掉的落盘行（P1/P2）——用 union2 的 matched 文件表排除
const usedRel = new Set(readFileSync('D:/FL/.scratch/research/1545-union2-matched.tsv', 'utf8').split('\n').slice(1).filter(Boolean).map((l) => l.split('\t')[10]));
const freeS2 = s2.filter((r) => r.real === 'Y' && r.CI === 'N' && +r.total >= 100 && Date.parse(r.iso) >= WIN_A && Date.parse(r.iso) <= WIN_B && !usedRel.has(r.rel));
const FULLFACE = /jest --config jest\.config\.unit\.js/;
const NARROW = /testPathPattern|dev-finish|-t ["']|--listTests|\.test\.js/;
const orphan = v4.filter((r) => r.bucket === '无汇总行' && FULLFACE.test(r.cmd || '') && !NARROW.test(r.cmd || ''));
console.log(`S1 侧「命令＝全量面（jest --config jest.config.unit.js，且不带 -t / testPathPattern / 位置参数 *.test.js / --listTests）但汇总行没进上下文」${orphan.length} 次`);
const byT0 = {};
for (const o of orphan) byT0[o.ticket || '(未归属)'] = (byT0[o.ticket || '(未归属)'] || 0) + 1;
console.log(`逐票：${Object.entries(byT0).sort((a, b) => b[1] - a[1]).map(([k, v]) => k + '=' + v).join(' ')}`);
const taken = new Set();
let m = 0;
for (const o of orphan) {
  const at = Date.parse(o.ts);
  const c = freeS2.find((b) => !taken.has(b.rel) && Date.parse(b.iso) >= at + 30000 && Date.parse(b.iso) <= at + 1800000);
  if (c) { taken.add(c.rel); m++; console.log(`  对上：${o.ts} 票${o.ticket || '-'} ← ${c.iso} ${c.rel}（total ${c.total} / ${c.sec || '—'} s）`); }
}
console.log(`可按时间对上落盘的 ${m} 次 ⇒ 计入并集；剩下 ${orphan.length - m} 次既无汇总行又对不上落盘 ⇒ 只进上界`);
console.log(`并集 = [${s1.length + freeS2.length - m + m}, ${s1.length + freeS2.length + (orphan.length - m)}] —— 下界 ${s1.length + freeS2.length}（S1 有汇总行 + 仅落盘），上界再加对不上盘的 ${orphan.length - m} 次`);
const un = orphan.filter((o) => { const at = Date.parse(o.ts); return !freeS2.some((b) => Date.parse(b.iso) >= at + 30000 && Date.parse(b.iso) <= at + 1800000); });
const byT = {};
for (const o of un) byT[o.ticket || '(未归属)'] = (byT[o.ticket || '(未归属)'] || 0) + 1;
console.log(`对不上落盘的逐票：${Object.entries(byT).sort((a, b) => b[1] - a[1]).map(([k, v]) => k + '=' + v).join(' ')}`);
console.log(`时间跨度 ${un.length ? un[0].ts : '-'} .. ${un.length ? un[un.length - 1].ts : '-'}`);
