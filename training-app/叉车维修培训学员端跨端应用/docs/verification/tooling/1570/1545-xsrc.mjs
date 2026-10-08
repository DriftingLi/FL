// S1(会话转录 JSONL) × S2(落盘 .log) 交叉对账：谁漏了谁
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
    S2.push({ p, size: st.size, mt: st.mtimeMs, f: m?.[1] ? +m[1] : 0, total: m ? +m[4] : NaN, sec: t ? +t[1] : NaN });
  }
}
for (const r of ['D:/FL/.scratch', 'D:/FL/.ci-verify']) walk(r, 0);
const s2full = S2.filter(o => o.total >= 100);

// S2 去重：同 total + 同 Time 秒 + 字节差 < 5% ⇒ 判为「同一次跑的副本」，保留字节大的那份
const groups = [];
for (const o of s2full.sort((a,b)=>b.size-a.size)) {
  const g = groups.find(g => g[0].total === o.total && !isNaN(o.sec) && Math.abs(g[0].sec - o.sec) < 0.001);
  if (g) g.push(o); else groups.push([o]);
}
const s2uniq = groups.map(g => g[0]);
const s2copies = groups.filter(g => g.length > 1);
console.log(`S2 落盘全量档：文件 ${s2full.length} 个 ⇒ 去重后 ${s2uniq.length} 次跑（${s2copies.length} 组存在副本，共 ${s2copies.reduce((a,g)=>a+g.length-1,0)} 份副本）`);
for (const g of s2copies) console.log('  副本组: ' + g.map(o=>`${o.p.split('FL').pop()}(${o.size}B)`).join(' | '));

// S1
const T = readFileSync('D:/FL/.scratch/research/1545-runs-v4.tsv','utf8').split('\n').filter(Boolean);
const h = T.shift().split('\t');
const rows = T.map(l=>{const c=l.split('\t');const o={};h.forEach((k,i)=>o[k]=c[i]);return o;});
const s1 = rows.filter(r=>r.bucket==='全量③').map(r=>({ ts:Date.parse(r.ts), sec:r.seconds===''?NaN:+r.seconds, total:+r.total, f:+r.fSuites||0, ticket:r.ticket, sid:r.sid, redirect:r.redirect, matched:null }));
console.log(`S1 转录全量档：${s1.length} 次（有 Time ${s1.filter(r=>!isNaN(r.sec)).length} 次）`);

// 匹配：|Δsec| < 0.5 且 total 相同 ⇒ 同一次跑
for (const a of s1) {
  if (isNaN(a.sec)) continue;
  const cand = s2uniq.filter(b => b.total === a.total && Math.abs(b.sec - a.sec) < 0.5 && !b._used);
  if (cand.length) { const b = cand[0]; b._used = true; a.matched = b; }
}
const both = s1.filter(r=>r.matched);
const s1only = s1.filter(r=>!r.matched);
const s2only = s2uniq.filter(b=>!b._used);
console.log(`\n=== 交叉对账 ===`);
console.log(`两源都读到（sec+total 对上）：${both.length} 次`);
console.log(`只在 S1（跑了但没落盘，或落盘文件已不在/mtime 早于窗口）：${s1only.length} 次，其中有 Time 的 ${s1only.filter(r=>!isNaN(r.sec)).length} 次`);
console.log(`只在 S2（落盘在，但汇总行没进上下文 ⇒ S1 漏计）：${s2only.length} 次`);
console.log(`\nS1 漏计的那 ${s2only.length} 次（S2-only）：`);
console.log('mtime(UTC)\t字节\t失败套件\ttotal\tTime_s\t路径');
for (const b of s2only.sort((x,y)=>x.mt-y.mt)) console.log(`${new Date(b.mt).toISOString()}\t${b.size}\t${b.f}\t${b.total}\t${isNaN(b.sec)?'-':b.sec}\t${b.p}`);
const sum = (a,f)=>a.reduce((x,y)=>x+f(y),0);
console.log(`\nS2-only 字节合计=${sum(s2only,o=>o.size)} Time合计=${sum(s2only.filter(o=>!isNaN(o.sec)),o=>o.sec).toFixed(3)}s`);
console.log(`S1-only 中「命令带落盘重定向」的 ${s1only.filter(r=>r.redirect==='Y').length} 次`);
// 两源并集下界
const unionSec = sum(both,r=>r.sec) + sum(s1only.filter(r=>!isNaN(r.sec)),r=>r.sec) + sum(s2only.filter(o=>!isNaN(o.sec)),o=>o.sec);
console.log(`\n并集口径（去重后）：次数下界 ${both.length + s1only.length + s2only.length}；可测墙钟合计 ${unionSec.toFixed(1)} s = ${(unionSec/60).toFixed(1)} min`);
console.log(`S1 单源口径：111 次 / 有 Time ${s1.filter(r=>!isNaN(r.sec)).length} 次 / ${sum(s1.filter(r=>!isNaN(r.sec)),r=>r.sec).toFixed(1)} s`);
console.log(`S2 单源口径：${s2uniq.length} 次 / 有 Time ${s2uniq.filter(o=>!isNaN(o.sec)).length} 次 / ${sum(s2uniq.filter(o=>!isNaN(o.sec)),o=>o.sec).toFixed(1)} s / 字节 ${sum(s2uniq,o=>o.size)}`);
