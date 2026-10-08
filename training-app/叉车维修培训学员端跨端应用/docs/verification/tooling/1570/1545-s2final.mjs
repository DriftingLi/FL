// S2 落盘源定档：把「本机 jest 全量跑的真产物」与「CI job 日志 / 评论回读 / 手写汇总」分开
import { readdirSync, statSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
const CUT = Date.parse('2026-09-22T00:00:00Z');
const acc = [];
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
    const head = s.slice(0, 4000);
    let cls = '本机跑';
    if (/Current runner version|##\[group\]Runner Image|^mobile-test\t/m.test(head) || /Hosted Compute Agent/.test(head)) cls = 'CI job 日志';
    else if (st.size < 5000) cls = '摘要/回读（非跑产物）';
    else if (/^npm warn Unknown user config/.test(s) === false && !/PASS|FAIL|console\.log|jest/.test(s.slice(0, 20000))) cls = '可疑（无 jest 逐套件行）';
    acc.push({ p, size: st.size, mt: st.mtimeMs, f: m && m[1] ? +m[1] : 0, total: m ? +m[4] : NaN, sec: t ? +t[1] : NaN, cls });
  }
}
for (const r of ['D:/FL/.scratch', 'D:/FL/.ci-verify']) walk(r, 0);
const byCls = new Map();
for (const o of acc) { if (!byCls.has(o.cls)) byCls.set(o.cls, []); byCls.get(o.cls).push(o); }
for (const [k, v] of byCls) {
  const full = v.filter(o => o.total >= 100);
  console.log(`${k}: 文件 ${v.length} 个（全量档 ${full.length}）· 字节 ${v.reduce((a,b)=>a+b.size,0)}`);
  if (k !== '本机跑') full.forEach(o => console.log(`   - ${o.p}  ${o.size}B  ${o.total}套件  Time=${isNaN(o.sec)?'-':o.sec}`));
}
// 本机跑·全量档 去重（同 total + 同 Time 秒 ⇒ 副本）
const local = byCls.get('本机跑') || [];
const lf = local.filter(o => o.total >= 100).sort((a,b)=>b.size-a.size);
const groups = [];
for (const o of lf) { const g = groups.find(g => g[0].total === o.total && !isNaN(o.sec) && Math.abs(g[0].sec - o.sec) < 0.001); if (g) g.push(o); else groups.push([o]); }
const uniq = groups.map(g => g[0]);
console.log(`\n本机跑·全量档：文件 ${lf.length} ⇒ 去重后 ${uniq.length} 次 · 字节 ${uniq.reduce((a,b)=>a+b.size,0)} · 有 Time ${uniq.filter(o=>!isNaN(o.sec)).length} · Time 合计 ${uniq.filter(o=>!isNaN(o.sec)).reduce((a,b)=>a+b.sec,0).toFixed(1)} s`);
console.log(`其中红跑（失败套件>0）${uniq.filter(o=>o.f>0).length} 次`);
// 1410 并行/串行实验那一批单独标出
const exp = uniq.filter(o => o.p.split('1410').length > 1);
console.log(`\n#1410「并行 vs 串行」实验批：${exp.length} 次 · Time ${exp.reduce((a,b)=>a+(isNaN(b.sec)?0:b.sec),0).toFixed(1)} s · 字节 ${exp.reduce((a,b)=>a+b.size,0)}`);
exp.forEach(o=>console.log(`   - ${o.p}  ${o.size}B  Time=${o.sec}`));
import { writeFileSync } from 'node:fs';
writeFileSync('D:/FL/.scratch/research/1545-s2-local.tsv',
  'mtime\tsize\tsuitesFailed\tsuitesTotal\tseconds\tpath\n' +
  uniq.sort((a,b)=>a.mt-b.mt).map(o=>`${new Date(o.mt).toISOString()}\t${o.size}\t${o.f}\t${o.total}\t${isNaN(o.sec)?'':o.sec}\t${o.p}`).join('\n')+'\n');
console.log('\n已写 1545-s2-local.tsv');
