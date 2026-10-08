import { readdirSync, statSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
const CUT = Date.parse('2026-09-22T00:00:00Z');
const S2 = [];
function walk(d, depth) {
  if (depth > 6) return;
  let es; try { es = readdirSync(d, { withFileTypes: true }); } catch { return; }
  for (const e of es) {
    const p = join(d, e.name);
    if (e.isDirectory()) { if (e.name !== 'node_modules' && e.name !== '.git') walk(p, depth + 1); continue; }
    if (!/\.(log|txt)$/i.test(e.name)) continue;
    let st; try { st = statSync(p); } catch { continue; }
    if (st.mtimeMs < CUT || st.size > 40_000_000) continue;
    let s; try { s = readFileSync(p, 'utf8'); } catch { continue; }
    if (!/Test Suites:/.test(s)) continue;
    const m = s.match(/Test Suites:\s*(?:(\d+) failed,\s*)?(?:(\d+) skipped,\s*)?(\d+) passed,\s*(\d+) total/);
    const t = s.match(/Time:\s*([\d.]+)\s*s/);
    S2.push({ p, size: st.size, mt: st.mtimeMs, f: m && m[1] ? +m[1] : 0, total: m ? +m[4] : NaN, sec: t ? +t[1] : NaN });
  }
}
for (const r of ['D:/FL/.scratch', 'D:/FL/.ci-verify']) walk(r, 0);
const s2full = S2.filter(o => o.total >= 100).sort((a,b)=>b.size-a.size);
const groups = [];
for (const o of s2full) {
  const g = groups.find(g => g[0].total === o.total && !isNaN(o.sec) && Math.abs(g[0].sec - o.sec) < 0.001);
  if (g) g.push(o); else groups.push([o]);
}
const s2uniq = groups.map(g => g[0]);

const T = readFileSync('D:/FL/.scratch/research/1545-runs-v4.tsv','utf8').split('\n').filter(Boolean);
const h = T.shift().split('\t');
const rows = T.map(l=>{const c=l.split('\t');const o={};h.forEach((k,i)=>o[k]=c[i]);return o;});
const s1 = rows.filter(r=>r.bucket==='全量③').map(r=>({ ts:Date.parse(r.ts), sec:r.seconds===''?NaN:+r.seconds, total:+r.total, f:+r.fSuites||0, verdict:r.verdict, ticket:r.ticket, sid:r.sid, redirect:r.redirect, cmd:r.cmd, used:false }));

// pass1: sec+total 精确对
for (const a of s1) {
  if (isNaN(a.sec)) continue;
  const b = s2uniq.find(x => !x._u && x.total === a.total && Math.abs(x.sec - a.sec) < 0.5);
  if (b) { b._u = true; a.used = true; a.how = 'P1'; }
}
// pass2: total 同 + fSuites 同 + mtime 落在 [ts+30s, ts+1500s] ⇒ 极可能是同一次跑（S1 无 Time 行）
const p2 = [];
for (const b of s2uniq.filter(x=>!x._u)) {
  const cand = s1.filter(a => !a.used && a.total === b.total && a.f === b.f && (b.mt - a.ts) >= 30000 && (b.mt - a.ts) <= 1500000);
  if (cand.length) {
    cand.sort((x,y)=>Math.abs(x.ts-b.mt)-Math.abs(y.ts-b.mt));
    const a = cand[0]; a.used = true; b._u = true; b.how = 'P2'; b.s1 = a; p2.push(b);
  }
}
const s1only = s1.filter(a=>!a.used);
const s2only = s2uniq.filter(b=>!b._u);
console.log(`S2 去重后 ${s2uniq.length} 次 · S1 ${s1.length} 次`);
console.log(`pass1（sec+total 精确对上）= ${s1.filter(a=>a.how==='P1').length}`);
console.log(`pass2（total+失败套件数对上、mtime 落在 ts+30s..ts+1500s ⇒ 同一次跑，S1 只是没 Time 行）= ${p2.length}`);
console.log(`真·S1 漏计（S2 有落盘、S1 完全无对应行）= ${s2only.length}`);
console.log(`真·S2 缺（S1 有行、落盘文件不在窗口内/已被清）= ${s1only.length}（其中有 Time ${s1only.filter(a=>!isNaN(a.sec)).length}）`);
console.log('\n--- pass2 明细（S1 行 × S2 落盘）---');
for (const b of p2.sort((x,y)=>x.mt-y.mt)) console.log(`${b.p.split('FL').pop()}  ${b.size}B  ${b.total}套件 ${b.f}失败  Time=${b.sec}s  ←S1 ${new Date(b.s1.ts).toISOString()} 票=${b.s1.ticket||'-'} sid=${b.s1.sid.slice(0,8)} redirect=${b.s1.redirect} Δ=${((b.mt-b.s1.ts)/1000).toFixed(0)}s`);
console.log('\n--- 真·S1 漏计明细 ---');
for (const b of s2only.sort((x,y)=>x.mt-y.mt)) console.log(`${new Date(b.mt).toISOString()}  ${b.size}B  ${b.total}套件 ${b.f}失败  Time=${isNaN(b.sec)?'-':b.sec}s  ${b.p}`);
const sum=(a,f)=>a.reduce((x,y)=>x+f(y),0);
console.log(`\n真·S1 漏计：字节 ${sum(s2only,o=>o.size)} · Time ${sum(s2only.filter(o=>!isNaN(o.sec)),o=>o.sec).toFixed(1)}s · 有 Time ${s2only.filter(o=>!isNaN(o.sec)).length}/${s2only.length}`);
console.log(`pass2 那批的落盘字节 ${sum(p2,o=>o.size)} · Time ${sum(p2.filter(o=>!isNaN(o.sec)),o=>o.sec).toFixed(1)}s`);
const u = s1.length + s2only.length;
const usec = sum(s1.filter(a=>!isNaN(a.sec)),a=>a.sec) + sum(s2only.filter(o=>!isNaN(o.sec)),o=>o.sec) + sum(p2.filter(o=>!isNaN(o.sec)&&isNaN(o.s1.sec)),o=>o.sec);
console.log(`\n并集：全量 ③ 至少 ${u} 次（S1 ${s1.length} + 仅落盘可证 ${s2only.length}）`);
console.log(`并集可测墙钟 ${usec.toFixed(1)} s = ${(usec/60).toFixed(1)} min（有 Time 的 ${s1.filter(a=>!isNaN(a.sec)).length + s2only.filter(o=>!isNaN(o.sec)).length + p2.filter(o=>!isNaN(o.sec)&&isNaN(o.s1.sec)).length} 次）`);
