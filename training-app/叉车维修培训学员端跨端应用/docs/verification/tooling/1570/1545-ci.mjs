// S4 CI 侧：从逐 run 的 jobs JSON 里挑 mobile-test，算次数 / 结论 / 墙钟
import { readdirSync, readFileSync } from 'node:fs';
const D = 'D:/FL/.scratch/research/cijobs/';
const runs = JSON.parse(readFileSync('D:/FL/.scratch/research/1545-ci-window.json', 'utf8'));
const byId = new Map(runs.map(r => [String(r.databaseId), r]));
const rows = [];
let nofile = 0, nojob = 0;
for (const f of readdirSync(D)) {
  if (!f.endsWith('.json')) continue;
  const id = f.replace(/\.json$/, '');
  let j; try { j = JSON.parse(readFileSync(D + f, 'utf8')); } catch { nofile++; continue; }
  const jobs = (j.jobs || []).filter(x => x.name === 'mobile-test');
  if (!jobs.length) { nojob++; continue; }
  for (const job of jobs) {
    const st = job.started_at ? Date.parse(job.started_at) : NaN;
    const en = job.completed_at ? Date.parse(job.completed_at) : NaN;
    const r = byId.get(id) || {};
    rows.push({
      id, branch: job.head_branch || r.headBranch || '', title: r.displayTitle || job.head_sha?.slice(0, 8) || '',
      sha: (job.head_sha || '').slice(0, 8), created: r.createdAt || '', start: job.started_at, end: job.completed_at,
      sec: Number.isFinite(st) && Number.isFinite(en) ? (en - st) / 1000 : NaN,
      concl: job.conclusion, event: r.event || '',
      npm: (job.steps || []).find(s => s.name === '安装依赖'),
      unit: (job.steps || []).find(s => s.name && s.name.includes('单元测试')),
    });
  }
}
rows.sort((a, b) => Date.parse(a.start || 0) - Date.parse(b.start || 0));
const sum = (a, f) => a.reduce((x, y) => x + f(y), 0);
const withT = rows.filter(r => Number.isFinite(r.sec));
console.log(`## S4 · CI mobile-test`);
console.log(`取回 jobs 文件 ${readdirSync(D).filter(f => f.endsWith('.json')).length} / 窗口内 run ${runs.length}；解析失败 ${nofile}；其中没有 mobile-test job（= changes.mobile 未命中 ⇒ skip）${nojob}`);
const byC = new Map(); for (const r of rows) byC.set(r.concl, (byC.get(r.concl) || 0) + 1);
console.log('按结论: ' + [...byC].sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k}=${v}`).join(' · '));
// 「实跑」= conclusion 是 success 或 failure（skipped/cancelled 不算跑了门）
const ran = rows.filter(r => r.concl === 'success' || r.concl === 'failure');
const ranT = ran.filter(r => Number.isFinite(r.sec) && r.sec > 0);
const badT = withT.filter(r => r.sec <= 0);
console.log(`其中真跑了门（success/failure）${ran.length} 次；skipped ${byC.get('skipped') || 0} 次（= changes.mobile 未命中该 PR 的改动集，job 建了但没跑）；cancelled ${byC.get('cancelled') || 0} 次`);
console.log(`可计时的真跑 ${ranT.length} 次；起止时间算出 ≤0 s 的 ${badT.length} 次（skipped/cancelled 的 started_at 与 completed_at 同刻或倒挂，已排除）`);
console.log(`真跑 job 墙钟合计 ${sum(ranT, r => r.sec).toFixed(1)} s = ${(sum(ranT, r => r.sec) / 60).toFixed(1)} min = ${(sum(ranT, r => r.sec) / 3600).toFixed(2)} h（均值 ${(sum(ranT, r => r.sec) / ranT.length).toFixed(1)} s，中位 ${ranT.map(r => r.sec).sort((a, b) => a - b)[Math.floor(ranT.length / 2)].toFixed(1)} s，区间 ${Math.min(...ranT.map(r => r.sec)).toFixed(1)}–${Math.max(...ranT.map(r => r.sec)).toFixed(1)} s）`);
// 「单元测试」这一步单独的墙钟（= ③ 门在 CI 上的净时间）
const u = ran.filter(r => r.unit && r.unit.started_at && r.unit.completed_at)
  .map(r => ({ ...r, usec: (Date.parse(r.unit.completed_at) - Date.parse(r.unit.started_at)) / 1000 }))
  .filter(r => r.usec > 0);
console.log(`其中「单元测试」这一步可单独计时 ${u.length} 次 · 合计 ${sum(u, r => r.usec).toFixed(1)} s = ${(sum(u, r => r.usec) / 60).toFixed(1)} min = ${(sum(u, r => r.usec) / 3600).toFixed(2)} h · 均值 ${(sum(u, r => r.usec) / u.length).toFixed(1)} s · 中位 ${u.map(r => r.usec).sort((a, b) => a - b)[Math.floor(u.length / 2)].toFixed(1)} s · 区间 ${Math.min(...u.map(r => r.usec)).toFixed(1)}–${Math.max(...u.map(r => r.usec)).toFixed(1)} s`);
const fail = u.filter(r => r.unit.conclusion !== 'success');
console.log(`「单元测试」这一步非 success 的 ${fail.length} 次：`);
for (const r of fail) console.log(`  ${(r.start || '').slice(0, 16)}  ${r.sha}  step=${r.unit.conclusion}  ${Math.round(r.usec)}s  ${r.branch}  ${r.title.slice(0, 50)}`);
console.log(`job 里「安装依赖」这一步可计时 ${ran.filter(r => r.npm && r.npm.started_at && r.npm.completed_at).length} 次（对照：npm ci 占 job 墙钟的大头）`);
// 同一 sha 上重复跑（= CI 侧的「重跑」）
const bySha = new Map(); for (const r of ran) { const k = r.sha; if (!bySha.has(k)) bySha.set(k, []); bySha.get(k).push(r); }
const rep = [...bySha].filter(([, v]) => v.length > 1);
console.log(`同一 head sha 上 mobile-test 跑了 ≥2 次的 sha 数 ${rep.length}（涉及 ${sum(rep, ([, v]) => v.length)} 次跑）；最多的一次 sha 跑了 ${Math.max(...rep.map(([, v]) => v.length))} 次`);
console.log('\n--- 同 sha 重复跑明细（≥2 次）---');
for (const [sha, v] of rep.sort((a, b) => b[1].length - a[1].length)) {
  console.log(`${sha} ×${v.length}  ${v.map(r => `${(r.start || '').slice(5, 16)}[${r.concl}]${Number.isFinite(r.sec) ? Math.round(r.sec) + 's' : ''}`).join(' ')}  ${v[0].branch}  ${v[0].title.slice(0, 60)}`);
}
console.log('\n--- 逐次真跑（前 20 与后 20）---');
const line = r => `${(r.start || '').slice(0, 16)}  ${r.sha}  ${r.concl.padEnd(9)} ${Number.isFinite(r.sec) ? Math.round(r.sec) + 's' : '-'}  ${r.event.padEnd(9)} ${r.branch.slice(0, 34).padEnd(34)} ${r.title.slice(0, 56)}`;
ran.slice(0, 20).forEach(r => console.log(line(r)));
console.log('  ...');
ran.slice(-20).forEach(r => console.log(line(r)));
// 按日
const byDay = new Map();
for (const r of ran) { const d = (r.start || '').slice(0, 10); if (!byDay.has(d)) byDay.set(d, { n: 0, s: 0, f: 0, u: 0, un: 0 }); const o = byDay.get(d); o.n++; if (Number.isFinite(r.sec) && r.sec > 0) o.s += r.sec; if (r.concl !== 'success') o.f++; if (r.unit && r.unit.started_at && r.unit.completed_at) { const x = (Date.parse(r.unit.completed_at) - Date.parse(r.unit.started_at)) / 1000; if (x > 0) { o.u += x; o.un++; } } }
console.log('\n--- 按日（只数真跑）---');
for (const [d, o] of [...byDay].sort()) console.log(`${d}  跑 ${o.n} 次  非success ${o.f}  job墙钟 ${o.s.toFixed(0)} s  单元测试步 ${o.un} 次 / ${o.u.toFixed(0)} s`);
// 按分支（= 按票）：CI 侧每票跑了几次门
const byBr = new Map();
for (const r of ran) { const b = r.branch || '(未知)'; if (!byBr.has(b)) byBr.set(b, { n: 0, s: 0, f: 0, shas: new Set() }); const o = byBr.get(b); o.n++; if (Number.isFinite(r.sec) && r.sec > 0) o.s += r.sec; if (r.concl !== 'success') o.f++; o.shas.add(r.sha); }
const brs = [...byBr].sort((a, b) => b[1].n - a[1].n);
console.log(`\n--- 按分支（真跑次数降序，共 ${brs.length} 个分支）---`);
console.log('| 分支 | 真跑次数 | 非success | 不同 sha 数 | job 墙钟 s |');
console.log('|---|---|---|---|---|');
for (const [b, o] of brs) console.log(`| ${b} | ${o.n} | ${o.f} | ${o.shas.size} | ${o.s.toFixed(0)} |`);
console.log(`\n分支数 ${brs.length}；真跑次数 top5：${brs.slice(0, 5).map(([b, o]) => `${b}=${o.n}`).join(' · ')}`);
console.log(`每个分支平均真跑 ${(ran.length / brs.length).toFixed(2)} 次；跑了 ≥2 次的分支 ${brs.filter(([, o]) => o.n >= 2).length} 个`);
console.log(`真跑次数 = 不同 sha 数 的分支 ${brs.filter(([, o]) => o.n === o.shas.size).length} 个（⇒ CI 侧的重跑几乎全部由「推了新 commit」触发，不是同一 commit 重跑）`);
