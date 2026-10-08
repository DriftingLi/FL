// S2 落盘源：扫工作树里所有 .log/.txt，挑出含 jest 汇总行且 total>=100（全量档）的，
// 输出 大小/mtime/套件/用例/Time，用于给读数补「落盘字节」这一栏与漏计校准。
import { readdirSync, statSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
const ROOTS = ['D:/FL/.scratch', 'D:/FL/.ci-verify'];
const CUT = Date.parse('2026-09-22T00:00:00Z');
const out = [];
function walk(d, depth) {
  if (depth > 6) return;
  let es; try { es = readdirSync(d, { withFileTypes: true }); } catch { return; }
  for (const e of es) {
    const p = join(d, e.name);
    if (e.isDirectory()) { if (e.name !== 'node_modules' && e.name !== '.git') walk(p, depth + 1); continue; }
    if (!/\.(log|txt)$/i.test(e.name)) continue;
    let st; try { st = statSync(p); } catch { continue; }
    if (st.mtimeMs < CUT) continue;
    if (st.size > 40_000_000) continue;
    let s; try { s = readFileSync(p, 'utf8'); } catch { continue; }
    if (!/Test Suites:/.test(s)) continue;
    const m = s.match(/Test Suites:\s*(?:(\d+) failed,\s*)?(?:(\d+) skipped,\s*)?(\d+) passed,\s*(\d+) total/);
    const t = s.match(/Time:\s*([\d.]+)\s*s/);
    const tests = s.match(/Tests:\s*(?:(\d+) failed,\s*)?(?:[\d\s,a-z]*?)(\d+) total/);
    out.push({ p, size: st.size, mtime: new Date(st.mtimeMs).toISOString(), f: m?.[1] || '0', total: m ? +m[4] : NaN, sec: t ? +t[1] : NaN, tests: tests ? +tests[2] : NaN });
  }
}
for (const r of ROOTS) walk(r, 0);
const full = out.filter(o => o.total >= 100).sort((a, b) => Date.parse(a.mtime) - Date.parse(b.mtime));
console.log(`含 jest 汇总行的落盘文件（mtime>=2026-09-22）共 ${out.length} 个，其中全量档(total>=100) ${full.length} 个`);
console.log('mtime(UTC)\t字节\t失败套件\t套件total\t用例total\tTime_s\t路径');
for (const o of full) console.log(`${o.mtime}\t${o.size}\t${o.f}\t${o.total}\t${o.tests}\t${isNaN(o.sec)?'-':o.sec}\t${o.p}`);
console.log(`\n全量档落盘字节合计=${full.reduce((a,b)=>a+b.size,0)} Time合计=${full.reduce((a,b)=>a+(isNaN(b.sec)?0:b.sec),0).toFixed(3)}s 有Time的=${full.filter(o=>!isNaN(o.sec)).length}`);
// 非全量档的也列个计数，说明「落盘里也有单套件跑」
const byT = new Map();
for (const o of out) { const k = o.total >= 100 ? '全量档' : 'total<100'; byT.set(k, (byT.get(k)||0)+1); }
console.log('按档位: ' + [...byT].map(([k,v])=>`${k}=${v}`).join(' '));
