// AC⑤：写码段（非门时间）到底能不能测 —— 会话消息间隙账
// 输入：C:/Users/ZHENG/.qoder/projects/d--FL/*.jsonl（主会话 + 子代理会话）
// 判据：同一 JSONL 文件内相邻两条带 timestamp 的记录之差 = 一个「间隙」；
//        一个间隙里可能是「模型在思考」/「工具在跑」/「人走开」三者之一 ⇒ 不可纯归因
import { readdirSync, readFileSync, statSync } from 'node:fs';
const D = 'C:/Users/ZHENG/.qoder/projects/d--FL';
const files = readdirSync(D).filter((f) => f.endsWith('.jsonl'));
const WIN_A = Date.parse('2026-09-22T07:48:39.133Z');
const WIN_B = Date.parse('2026-10-04T16:21:59.177Z');
const gaps = [];
const spans = [];
let spanTotal = 0, nSess = 0;
for (const f of files) {
  const ts = [];
  for (const line of readFileSync(D + '/' + f, 'utf8').split('\n')) {
    if (!line) continue;
    let o; try { o = JSON.parse(line); } catch { continue; }
    const t = o.timestamp ? Date.parse(o.timestamp) : NaN;
    if (Number.isFinite(t)) ts.push(t);
  }
  if (ts.length < 2) continue;
  ts.sort((a, b) => a - b);
  nSess++;
  spans.push({ sess: f, h: (ts[ts.length - 1] - ts[0]) / 3600000, from: ts[0], to: ts[ts.length - 1] });
  spanTotal += ts[ts.length - 1] - ts[0];
  for (let i = 1; i < ts.length; i++) gaps.push({ sess: f, d: (ts[i] - ts[i - 1]) / 1000, at: ts[i] });
}
const win = gaps.filter((g) => g.at >= WIN_A && g.at <= WIN_B);
const sum = (a) => a.reduce((x, y) => x + y.d, 0);
const q = (a, p) => { const s = a.map((g) => g.d).sort((x, y) => x - y); return s[Math.floor(s.length * p)]; };
const bucket = (lo, hi) => win.filter((g) => g.d >= lo && g.d < hi);
console.log('## AC⑤ 写码段间隙账（会话转录相邻记录之差）');
console.log(`扫了 ${files.length} 个 JSONL，其中 ${nSess} 个有 ≥2 条带 timestamp 的记录`);
console.log(`会话跨度合计 ${Math.round(spanTotal / 1000)} s = ${(spanTotal / 3600000).toFixed(2)} h（96 个会话各算首末记录之差再相加，会话之间重叠部分不相减）`);
console.log(`间隙总数 ${gaps.length}；落在观察窗内的 ${win.length}`);
console.log(`窗内间隙分布：p50 ${q(win, 0.5).toFixed(2)} s · p90 ${q(win, 0.9).toFixed(2)} s · p99 ${q(win, 0.99).toFixed(2)} s · max ${(Math.max(...win.map((g) => g.d)) / 3600).toFixed(2)} h`);
for (const [nm, lo, hi] of [['< 1 min', 0, 60], ['1–10 min', 60, 600], ['≥ 10 min', 600, Infinity]]) {
  const b = bucket(lo, hi);
  console.log(`  ${nm}：${b.length} 个 · 合计 ${(sum(b) / 3600).toFixed(2)} h · 占窗内间隙总时长 ${win.length ? ((sum(b) / sum(win)) * 100).toFixed(1) : '—'} 个百分点`);
}
console.log(`窗内间隙总时长 ${(sum(win) / 3600).toFixed(2)} h`);
console.log(`对照：全量 ③ 可测墙钟 29297.8 s = 8.14 h ⇒ 占窗内间隙总时长的 ${((29297.8 / sum(win)) * 100).toFixed(2)} 个百分点`);
const act = sum(bucket(0, 600)) / 3600;
console.log(`「活跃段」（间隙 < 10 min 之和）= ${act.toFixed(2)} h ⇒ 全量 ③ 8.14 h 占活跃段 ${((8.14 / act) * 100).toFixed(2)} 个百分点`);
const long3 = win.filter((g) => g.d >= 3600).sort((a, b) => b.d - a.d).slice(0, 8);
console.log(`≥ 1 h 的间隙 ${win.filter((g) => g.d >= 3600).length} 个，最长的几个：`);
for (const g of long3) console.log(`  ${(g.d / 3600).toFixed(2)} h @ ${new Date(g.at).toISOString().slice(0, 19)} (${g.sess.slice(0, 8)})`);
spans.sort((a, b) => b.h - a.h);
console.log(`会话跨度（首末记录之差）最长的 5 个：`);
for (const s of spans.slice(0, 5)) console.log(`  ${s.h.toFixed(1)} h  ${s.sess.slice(0, 8)}  ${new Date(s.from).toISOString().slice(0, 16)} .. ${new Date(s.to).toISOString().slice(0, 16)}`);
const iv = spans.map((s) => [s.from, s.to]).sort((a, b) => a[0] - b[0]);
let u = 0, ce = -1, cs = -1;
for (const [a, b] of iv) { if (a > ce) { if (cs >= 0) u += ce - cs; cs = a; ce = b; } else if (b > ce) ce = b; }
if (cs >= 0) u += ce - cs;
console.log(`会话跨度并集（把重叠会话合并后）= ${(u / 3600000).toFixed(2)} h = ${(u / 1000).toFixed(0)} s —— 这才可与「窗内间隙总时长」对照，前者是墙上时间、后者是间隙相加`);
console.log(`全量 ③ 可测墙钟 8.14 h 占并集 ${(u / 3600000).toFixed(2)} h 的 ${((29297.8 / 3600) / (u / 3600000) * 100).toFixed(2)} 个百分点`);
