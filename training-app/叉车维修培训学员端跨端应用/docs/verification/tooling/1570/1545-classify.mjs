// #1545 v3 · 归因分类：把 1545-runs-v2.tsv 里的每次全量 ③ 执行按「触发原因」分档
// 规则全部可从 TSV 列复算；规则不命中的记「归因不可得」，不猜。
import { readFileSync } from 'node:fs';

const T = readFileSync('D:/FL/.scratch/research/1545-runs-v2.tsv', 'utf8').split('\n').filter(Boolean);
const head = T.shift().split('\t');
const ix = (n) => head.indexOf(n);
const rows = T.map((l) => { const c = l.split('\t'); const o = {}; head.forEach((h, i) => o[h] = c[i]); return o; });

const full = rows.filter((r) => r.bucket === '全量③').map((r) => ({
  ...r, seconds: r.seconds === '' ? NaN : +r.seconds, chars: +r.resultChars || 0,
  editsBefore: +r.editsBefore || 0, t: Date.parse(r.ts),
})).sort((a, b) => a.t - b.t);

// ---- 并发重叠：同一时刻有别的会话也在跑全量 ③ ----
for (let i = 0; i < full.length; i++) {
  const a = full[i]; const end = isNaN(a.seconds) ? a.t + 120000 : a.t + a.seconds * 1000;
  a.overlap = full.filter((b, j) => j !== i && b.sid !== a.sid && b.t < end && (isNaN(b.seconds) ? b.t + 120000 : b.t + b.seconds * 1000) > a.t).length;
}

// ---- 同会话内排序，取「上一次全量」与「下一次全量」----
const bySess = new Map();
for (const r of full) { if (!bySess.has(r.sid)) bySess.set(r.sid, []); bySess.get(r.sid).push(r); }
for (const [, arr] of bySess) for (let i = 0; i < arr.length; i++) { arr[i].prev = arr[i - 1] || null; arr[i].next = arr[i + 1] || null; }

const WALLCLOCK_SUITES = /kotlinAllProcessCaptureBehavior|hxLaunchDetachBehavior/;
function cause(r) {
  const red = r.verdict === '红';
  const gapMin = r.prev ? (r.t - r.prev.t) / 60000 : NaN;
  // C3 并发假红：红 + 失败面命中墙钟套件，或 红 + 与其它会话跑重叠
  if (red && WALLCLOCK_SUITES.test(r.fails || '')) return 'C3 并发假红（墙钟套件命中）';
  if (red && r.overlap > 0 && (r.seconds > 400 || isNaN(r.seconds))) return 'C3 并发假红（重叠 + 墙钟膨胀）';
  // C1 为了重看失败原因：红跑但失败详情没进上下文（结果字符少且没抓到 FAIL 行），且随后又跑了一次
  if (red && !WALLCLOCK_SUITES.test(r.fails || '') && r.next && (r.chars < 500 || !(r.fails || '').trim())) return 'C1 重看失败原因（红跑详情未入上下文 ⇒ 又跑一次）';
  // C4 写错返工 / 正常迭代：两次全量之间有编辑动作
  if (r.editsBefore > 0) return 'C4 改完复验（其间有 Edit/Write）';
  // C2 无改动重跑：与上一次全量之间零编辑
  if (r.prev && r.editsBefore === 0) return 'C2 无本地改动又跑一次（sha 前进 / 取证重贴 / 习惯）';
  if (!r.prev) return 'C0 会话内首次全量（无对照）';
  return '归因不可得';
}
for (const r of full) r.cause = cause(r);

console.log(`全量 ③ 执行数=${full.length}  红=${full.filter((r) => r.verdict === '红').length}  绿=${full.filter((r) => r.verdict === '绿').length}`);
console.log(`有 Time: 秒数=${full.filter((r) => !isNaN(r.seconds)).length}  秒数合计=${full.reduce((a, r) => a + (isNaN(r.seconds) ? 0 : r.seconds), 0).toFixed(1)}`);
console.log(`结果字符合计=${full.reduce((a, r) => a + r.chars, 0)}  中位数=${(() => { const a = full.map((r) => r.chars).sort((x, y) => x - y); return a[Math.floor(a.length / 2)]; })()}`);
console.log(`与其它会话重叠的全量跑=${full.filter((r) => r.overlap > 0).length}`);
console.log(`落盘重定向的命令=${full.filter((r) => r.redirect === 'Y').length}  由后台 output 解析=${full.filter((r) => r.src.startsWith('bg:')).length}`);

console.log('\n--- 按原因分档 ---');
console.log('cause\truns\tred\tsumSeconds\trunsWithTime\tsumResultChars\tmeanSeconds');
const perC = new Map();
for (const r of full) {
  if (!perC.has(r.cause)) perC.set(r.cause, { n: 0, red: 0, sec: 0, nSec: 0, chars: 0 });
  const o = perC.get(r.cause); o.n++; if (r.verdict === '红') o.red++;
  if (!isNaN(r.seconds)) { o.sec += r.seconds; o.nSec++; } o.chars += r.chars;
}
for (const [k, o] of [...perC].sort((a, b) => b[1].n - a[1].n)) {
  console.log([k, o.n, o.red, o.sec.toFixed(1), o.nSec, o.chars, o.nSec ? (o.sec / o.nSec).toFixed(1) : '-'].join('\t'));
}

console.log('\n--- 逐次全量 ③（按时间）---');
console.log('ts\tticket\tverdict\tsuites\tseconds\tchars\teditsBefore\toverlap\tredirect\tsrc\tcause\tfails');
for (const r of full) {
  console.log([r.ts, r.ticket || '-', r.verdict, `${r.fSuites}f/${r.total}`, isNaN(r.seconds) ? '-' : r.seconds, r.chars, r.editsBefore, r.overlap, r.redirect, r.src, r.cause, (r.fails || '').slice(0, 90)].join('\t'));
}

// ---- 票号归属可信度：从 ticketWhy 里解析 top1/top2 权重比 ----
console.log('\n--- 逐票汇总（带归属可信度）---');
console.log('ticket\truns\tred\tsumSeconds\trunsWithTime\tsumChars\tsessions\ttop1/top2比\tfirst\tlast');
const perT = new Map();
for (const r of full) {
  const k = r.ticket || '(未归属)';
  if (!perT.has(k)) perT.set(k, { n: 0, red: 0, sec: 0, nSec: 0, chars: 0, sids: new Set(), ratios: new Set(), first: r.ts, last: r.ts });
  const o = perT.get(k); o.n++; if (r.verdict === '红') o.red++;
  if (!isNaN(r.seconds)) { o.sec += r.seconds; o.nSec++; }
  o.chars += r.chars; o.sids.add(r.sid);
  const w = (r.ticketWhy || '').match(/权重(\d+)\(([^)]*)\)/);
  if (w) { const parts = w[2].split(',').map((x) => +x.split(':')[1]); o.ratios.add(parts[1] ? (parts[0] / parts[1]).toFixed(1) : 'inf'); }
  if (r.ts < o.first) o.first = r.ts; if (r.ts > o.last) o.last = r.ts;
}
for (const [k, o] of [...perT].sort((a, b) => b[1].n - a[1].n)) {
  console.log([k, o.n, o.red, o.sec.toFixed(1), o.nSec, o.chars, o.sids.size, [...o.ratios].join('/'), o.first, o.last].join('\t'));
}
