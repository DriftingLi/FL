// 终版并集：S1（会话转录 111 次全量）× S2v2（全仓兜底 55 次窗内本机全量产物）
// 匹配规则与 1545-final-account.mjs 一致：
//   P1 精确：|Δsec| < 0.5 且 total 相同
//   P2 极可能：total 与失败套件数全等，且落盘 mtime 落在 S1 命令 ts + 30 s .. + 1800 s
import { readFileSync, writeFileSync } from 'node:fs';
const rd = (p) => { const t = readFileSync(p, 'utf8').split('\n').filter(Boolean); const h = t.shift(); return t.map((l) => { const c = l.split('\t'); const o = {}; h.split('\t').forEach((k, i) => (o[k] = c[i])); return o; }); };
const s1 = rd('D:/FL/.scratch/research/1545-s1-enriched.tsv');
const s2all = rd('D:/FL/.scratch/research/1545-s2v2.tsv');
const WIN_A = Date.parse('2026-09-22T07:48:39.133Z');
const WIN_B = Date.parse('2026-10-04T16:21:59.177Z');
const s2 = s2all.filter((r) => r.real === 'Y' && r.CI === 'N' && +r.total >= 100 && Date.parse(r.iso) >= WIN_A && Date.parse(r.iso) <= WIN_B);
console.log(`S1 全量（窗内、有汇总行进上下文）${s1.length} 次`);
console.log(`S2v2 全量（窗内、本机真跑产物、total ≥ 100）${s2.length} 条（文件 ${new Set(s2.map((r) => r.rel)).size} 个）`);
const used = new Set();
const pairs = [];
for (const a of s1) {
  const at = Date.parse(a.ts);
  const secA = parseFloat(a.seconds);
  let best = null;
  for (const b of s2) {
    if (used.has(b)) continue;
    const bt = Date.parse(b.iso), secB = parseFloat(b.sec || 'NaN');
    const sameTotal = String(a.suites) === b.total;
    if (Number.isFinite(secA) && Number.isFinite(secB) && Math.abs(secA - secB) < 0.5 && sameTotal) { best = { b, why: 'P1' }; break; }
    if (sameTotal && String(a.failed || '0') === String(b.fail || '0') && bt >= at + 30000 && bt <= at + 1800000) best = best && best.why === 'P1' ? best : { b, why: 'P2' };
  }
  if (best) { used.add(best.b); pairs.push({ a, b: best.b, why: best.why }); }
}
const p1 = pairs.filter((p) => p.why === 'P1'), p2 = pairs.filter((p) => p.why === 'P2');
const s2only = s2.filter((b) => !used.has(b));
console.log(`匹配：P1 精确 ${p1.length} · P2 极可能 ${p2.length} · 仅 S1 ${s1.length - pairs.length} · 仅 S2v2 ${s2only.length}`);
console.log(`并集下界 = ${s1.length} + ${s2only.length} = ${s1.length + s2only.length} 次`);
const sumS1 = s1.reduce((x, r) => x + (Number.isFinite(parseFloat(r.seconds)) ? parseFloat(r.seconds) : 0), 0);
const sumS2only = s2only.reduce((x, r) => x + (Number.isFinite(parseFloat(r.sec)) ? parseFloat(r.sec) : 0), 0);
console.log(`墙钟：S1 ${sumS1.toFixed(1)} s + 仅 S2v2 ${sumS2only.toFixed(1)} s = ${(sumS1 + sumS2only).toFixed(1)} s = ${((sumS1 + sumS2only) / 3600).toFixed(2)} h`);
console.log(`落盘字节：窗内本机全量产物合计 ${s2.reduce((x, r) => x + (+r.size), 0)} B（其中仅 S2v2 那 ${s2only.length} 条 ${s2only.reduce((x, r) => x + (+r.size), 0)} B）`);
console.log(`进上下文字符（只有 S1 有）：${s1.reduce((x, r) => x + (+r.chars || 0), 0)}`);
writeFileSync('D:/FL/.scratch/research/1545-union2-matched.tsv', ['why\tts\tticket\tcause\tverdict\ttotal\ts1sec\tchars\tmtime\tsize\trel'].concat(pairs.map((p) => [p.why, p.a.ts, p.a.ticket, p.a.cause, p.a.verdict, p.a.suites, p.a.seconds, p.a.chars, p.b.iso, p.b.size, p.b.rel].join('\t'))).join('\n') + '\n');
writeFileSync('D:/FL/.scratch/research/1545-union2-s2only.tsv', ['mtime\ttotal\tfail\tsec\tsize\trel'].concat(s2only.map((b) => [b.iso, b.total, b.fail, b.sec, b.size, b.rel].join('\t'))).join('\n') + '\n');
console.log('\n--- 仅 S2v2（落盘可证、转录里没这跑）逐条 ---');
for (const b of s2only.sort((x, y) => Date.parse(x.iso) - Date.parse(y.iso))) console.log(`| ${b.iso.slice(5, 19).replace('T', ' ')} | ${b.total} | ${b.fail || 0} | ${b.sec || '—'} | ${b.size} | ${b.rel} |`);
// S1 里 bucket 判定用的 chars 分布（供字符栏算式）
const chars = s1.map((r) => +r.chars || 0).sort((a, b) => a - b);
console.log(`\nS1 单次进上下文字符：中位 ${chars[Math.floor(chars.length / 2)]} · p90 ${chars[Math.floor(chars.length * 0.9)]} · 最大 ${chars[chars.length - 1]}`);
