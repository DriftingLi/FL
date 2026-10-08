// S4 扩窗：把窗前（cijobs2）与窗内（cijobs）两批 jobs 合起来算 CI 侧 mobile-test
import { readdirSync, readFileSync } from 'node:fs';
const DIRS = ['D:/FL/.scratch/research/cijobs2/', 'D:/FL/.scratch/research/cijobs/'];
const WIN_A = Date.parse('2026-09-22T07:48:39.133Z');
const files = new Map();
for (const D of DIRS) for (const f of readdirSync(D)) if (f.endsWith('.json')) files.set(f.replace(/\.json$/, ''), D + f);
const rows = []; let nofile = 0, nojob = 0, nfiles = files.size;
for (const [id, path] of files) {
  let j; try { j = JSON.parse(readFileSync(path, 'utf8')); } catch { nofile++; continue; }
    const jobs = (j.jobs || []).filter((x) => x.name === 'mobile-test');
    if (!jobs.length) { nojob++; continue; }
    for (const job of jobs) {
      const st = Date.parse(job.started_at || ''), en = Date.parse(job.completed_at || '');
      const u = (job.steps || []).find((s) => s.name && s.name.includes('单元测试'));
      const usec = u && u.started_at && u.completed_at ? (Date.parse(u.completed_at) - Date.parse(u.started_at)) / 1000 : NaN;
      rows.push({
        id, sha: (job.head_sha || '').slice(0, 8), branch: job.head_branch || '',
        start: job.started_at, sec: Number.isFinite(st) && Number.isFinite(en) ? (en - st) / 1000 : NaN,
        concl: job.conclusion, usec, uconcl: u ? u.conclusion : null,
        pre: Date.parse(job.started_at || '0') < WIN_A,
      });
    }
}
rows.sort((a, b) => Date.parse(a.start || 0) - Date.parse(b.start || 0));
const ran = rows.filter((r) => r.concl === 'success' || r.concl === 'failure');
const sum = (a, f) => a.reduce((x, y) => x + (Number.isFinite(f(y)) ? f(y) : 0), 0);
const byC = new Map(); for (const r of rows) byC.set(r.concl, (byC.get(r.concl) || 0) + 1);
const seg = (p) => ran.filter(p);
console.log(`## S4 · CI mobile-test（扩窗）`);
console.log(`run 级去重后 ${nfiles} 个 run（两目录并集；重叠 ${readdirSync(DIRS[0]).length + readdirSync(DIRS[1]).length - nfiles} 个）；解析失败 ${nofile}；该 run 里没有 mobile-test job 的 ${nojob} 个`);
console.log(`mobile-test job 共 ${rows.length} 个 ⇒ ` + [...byC].sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k}=${v}`).join(' · '));
console.log(`真跑（success/failure）${ran.length} 次；skipped ${byC.get('skipped') || 0} · cancelled ${byC.get('cancelled') || 0}`);
for (const [nm, p] of [['窗前（起 < 2026-09-22T07:48Z）', (r) => r.pre], ['窗内（≥ 2026-09-22T07:48Z）', (r) => !r.pre], ['合起来', () => true]]) {
  const a = seg(p); const t = a.filter((r) => Number.isFinite(r.sec) && r.sec > 0);
  const s = sum(t, (r) => r.sec);
  const u = a.filter((r) => Number.isFinite(r.usec) && r.usec > 0);
  const us = sum(u, (r) => r.usec);
  const med = (arr) => arr.slice().sort((x, y) => x - y)[Math.floor(arr.length / 2)];
  console.log(`${nm}：真跑 ${a.length} 次（红 ${a.filter((r) => r.concl === 'failure').length}）· 可计时 ${t.length} · job 墙钟 ${s.toFixed(1)} s = ${(s / 3600).toFixed(2)} h · 中位 ${med(t.map((r) => r.sec)).toFixed(1)} s · 「单元测试」步可计时 ${u.length} · 净墙钟 ${us.toFixed(1)} s = ${(us / 3600).toFixed(2)} h · 中位 ${med(u.map((r) => r.usec)).toFixed(1)} s · 该步非 success ${u.filter((r) => r.uconcl !== 'success').length} 次`);
}
const bySha = new Map(); for (const r of ran) { if (!bySha.has(r.sha)) bySha.set(r.sha, []); bySha.get(r.sha).push(r); }
const rep = [...bySha].filter(([, v]) => v.length > 1);
console.log(`同一 head sha 真跑 ≥2 次的 sha ${rep.length} 个（涉及 ${sum(rep, ([, v]) => v.length)} 次）；最多的 sha 跑了 ${rep.length ? Math.max(...rep.map(([, v]) => v.length)) : 0} 次`);
for (const [sha, v] of rep.sort((a, b) => b[1].length - a[1].length).slice(0, 12)) {
  console.log(`  ${sha} ×${v.length}  ${v.map((r) => `${(r.start || '').slice(5, 16)}[${r.concl}]${Number.isFinite(r.sec) ? Math.round(r.sec) + 's' : ''}`).join(' ')}  ${v[0].branch}`);
}
const byBr = new Map();
for (const r of ran) { const b = r.branch || '?'; if (!byBr.has(b)) byBr.set(b, { n: 0, s: 0, f: 0, shas: new Set() }); const o = byBr.get(b); o.n++; if (Number.isFinite(r.sec) && r.sec > 0) o.s += r.sec; if (r.concl !== 'success') o.f++; o.shas.add(r.sha); }
const brs = [...byBr].sort((a, b) => b[1].n - a[1].n);
console.log(`\n分支（真跑口径）共 ${brs.length} 个；top15：`);
for (const [b, o] of brs.slice(0, 15)) console.log(`  ${String(o.n).padStart(3)} 次  红 ${o.f}  不同 sha ${o.shas.size}  job墙钟 ${o.s.toFixed(0)} s  ${b}`);
console.log(`真跑次数 = 不同 sha 数的分支 ${brs.filter(([, o]) => o.n === o.shas.size).length} / ${brs.length} 个`);
console.log(`跑了 ≥2 次的分支 ${brs.filter(([, o]) => o.n >= 2).length} 个；≥5 次 ${brs.filter(([, o]) => o.n >= 5).length} 个；≥8 次 ${brs.filter(([, o]) => o.n >= 8).length} 个`);
// 「单元测试」步失败的套件名（判是否墙钟假红族）
const badStep = ran.filter((r) => r.uconcl && r.uconcl !== 'success');
console.log(`\n「单元测试」步非 success 的 ${badStep.length} 次（sha 列表，日志另存）：${badStep.map((r) => r.sha).join(' ')}`);
