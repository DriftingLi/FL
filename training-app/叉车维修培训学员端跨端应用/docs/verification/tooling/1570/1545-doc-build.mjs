// 把 1545-doc-prose.md 里的占位符换成从台账生成的表，产出 docs/design/ 的终件
import { readFileSync, writeFileSync, readdirSync } from 'node:fs';

const rd = (p) => {
  const t = readFileSync(p, 'utf8').split('\n').filter(Boolean);
  const h = t.shift().split('\t');
  return t.map((l) => { const c = l.split('\t'); const o = {}; h.forEach((k, i) => (o[k] = c[i])); return o; });
};
const R = 'D:/FL/.scratch/research/';
const s1 = rd(R + '1545-s1-enriched.tsv');
const s2only = rd(R + '1545-union2-s2only.tsv');
const matched = rd(R + '1545-union2-matched.tsv');
const s2all = rd(R + '1545-s2v2.tsv');

// ---- 附录 B：仅落盘的 36 次
const B = ['| mtime (UTC) | total 套件 | 失败套件 | jest Time s | B | 路径（相对 D:/FL） |', '|---|---|---|---|---|---|']
  .concat(s2only.sort((a, b) => Date.parse(a.mtime) - Date.parse(b.mtime))
    .map((r) => `| ${r.mtime.slice(0, 19).replace('T', ' ')} | ${r.total} | ${r.fail || 0} | ${r.sec || '—'} | ${r.size} | ${r.rel} |`));

// ---- §4 逐票
const per = new Map();
for (const r of s1) {
  const k = r.ticket || '(未归属)';
  if (!per.has(k)) per.set(k, { a: [], s2: [], bytes: 0, sids: new Set() });
  per.get(k).a.push(r); per.get(k).sids.add(r.sid);
}
for (const r of s2only) {
  const cand = (r.rel.match(/\d{3,4}/g) || []).map(Number).filter((n) => n >= 600 && n <= 1600);
  const k = cand.length ? String(cand[0]) : '(未归属)';
  if (!per.has(k)) per.set(k, { a: [], s2: [], bytes: 0, sids: new Set() });
  per.get(k).s2.push(r); per.get(k).bytes += +r.size;
}
const CA = ['C4', 'C2a', 'C2b', 'C2c', 'C2d', 'C1', 'C3'];
const num = (x) => (Number.isFinite(parseFloat(x)) ? parseFloat(x) : NaN);
const row = (k, o) => {
  const a = o.a;
  const sec = a.reduce((x, r) => x + (Number.isFinite(num(r.seconds)) ? num(r.seconds) : 0), 0);
  const ch = a.reduce((x, r) => x + (+r.chars || 0), 0);
  const c = (k2) => a.filter((r) => r.cause === k2).length;
  const hasS2 = o.s2.length > 0;
  return `| ${k}${hasS2 && !a.length ? '†' : ''} | ${a.length || '—'} | ${a.filter((r) => r.verdict === '红').length || (a.length ? 0 : '—')} | ${CA.map((x) => (a.length ? c(x) : '—')).join(' | ')} | ${a.length ? sec.toFixed(1) : '—'} | ${a.length ? ch : '—'} | ${hasS2 ? o.s2.length : '—'} | ${o.bytes ? o.bytes : '—'} | ${o.sids.size || '—'} |`;
};
const sorted = [...per].sort((a, b) => (b[1].a.length + b[1].s2.length) - (a[1].a.length + a[1].s2.length));
const T4 = [`| 票 | S1 全量 | 红 | C4 | C2a | C2b | C2c | C2d | C1 | C3 | S1 墙钟 s | 进上下文字符 | 仅落盘 | 落盘字节 | 会话数 |`,
  `|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|`]
  .concat(sorted.map(([k, o]) => row(k, o)))
  .concat([`| **合计** | ${s1.length} | ${s1.filter((r) => r.verdict === '红').length} | ${CA.map((x) => s1.filter((r) => r.cause === x).length).join(' | ')} | ${s1.reduce((x, r) => x + (Number.isFinite(num(r.seconds)) ? num(r.seconds) : 0), 0).toFixed(1)} | ${s1.reduce((x, r) => x + (+r.chars || 0), 0)} | ${s2only.length} | ${s2only.reduce((x, r) => x + +r.size, 0)} | ${new Set(s1.map((r) => r.sid)).size} |`]);

// ---- §5 #1472 六次
const m1472 = matched.filter((r) => r.rel.includes('wt1472'));
const all1472 = s2all.filter((r) => r.rel.includes('wt1472-full') && r.real === 'Y' && r.CI === 'N');
const T5 = ['| # | 落盘文件 | 字节 | 套件 total | 失败套件 | jest Time s | 两源对账 | S1 命令 ts (UTC) / 会话 |', '|---|---|---|---|---|---|---|---|'];
all1472.sort((a, b) => Date.parse(a.iso) - Date.parse(b.iso)).forEach((r, i) => {
  const p = m1472.find((x) => x.rel === r.rel);
  T5.push(`| ${i + 1} | ${r.rel} | ${r.size} | ${r.total} | ${r.fail || 0} | ${r.sec} | ${p ? p.why + '（' + p.ticket + ' / ' + (p.ts || '').slice(0, 19) + '）' : '仅 S2（S1 无汇总行）'} | ${p ? (p.ts || '').slice(0, 23) : '—'} |`);
});
T5.push(`| 合计 | ${all1472.length} 份 | ${all1472.reduce((x, r) => x + +r.size, 0)} B | — | ${all1472.reduce((x, r) => x + (+r.fail || 0), 0)} | ${all1472.reduce((x, r) => x + num(r.sec), 0).toFixed(3)} s | — | — |`);

// ---- 附录 A：S1 逐次台账
const A = ['| # | ts (UTC) | 票 | 会话 | 红绿 | 失败/total 套件 | Time s | 字符 | eCode | eDoc | eOther | gOps | 重叠 | 因 | 失败套件 |',
  '|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|'];
s1.slice().sort((a, b) => Date.parse(a.ts) - Date.parse(b.ts)).forEach((r, i) => {
  A.push(`| ${i + 1} | ${r.ts.slice(0, 19).replace('T', ' ')} | ${r.ticket || '-'} | ${(r.sid || '').slice(0, 8)} | ${r.verdict} | ${r.failed || 0}/${r.suites} | ${r.seconds || '—'} | ${r.chars || 0} | ${r.eCode} | ${r.eDoc} | ${r.eOther} | ${r.gOps} | ${r.overlap} | ${r.cause} | ${(r.fails || '').replace(/\|/g, '/').slice(0, 58) || '—'} |`);
});

// ---- 附录 C：CI 窗内按分支真跑 top20
const DIRS = [R + 'cijobs/', R + 'cijobs2/'];
const WIN_A = Date.parse('2026-09-22T07:48:39.133Z');
const byBr = new Map();
for (const D of DIRS) for (const f of readdirSync(D)) {
  if (!f.endsWith('.json')) continue;
  let j; try { j = JSON.parse(readFileSync(D + f, 'utf8')); } catch { continue; }
  for (const job of (j.jobs || [])) {
    if (job.name !== 'mobile-test') continue;
    if (job.conclusion !== 'success' && job.conclusion !== 'failure') continue;
    if (Date.parse(job.started_at) < WIN_A) continue;
    const b = job.head_branch || '?';
    if (!byBr.has(b)) byBr.set(b, { n: 0, s: 0, f: 0, shas: new Set(), u: 0, un: 0 });
    const o = byBr.get(b);
    o.n++; o.s += (Date.parse(job.completed_at) - Date.parse(job.started_at)) / 1000; o.shas.add((job.head_sha || '').slice(0, 8));
    if (job.conclusion !== 'success') o.f++;
    const st = (job.steps || []).find((x) => x.name && x.name.includes('单元测试'));
    if (st && st.started_at && st.completed_at) { o.u += (Date.parse(st.completed_at) - Date.parse(st.started_at)) / 1000; o.un++; }
  }
}
const cbr = [...byBr].sort((a, b) => b[1].n - a[1].n);
const C = ['| 分支（窗内真跑 ≥ 2 次，降序，取满 20 行） | 真跑 | 红 | 不同 sha | job 墙钟 s | 「单元测试」步 s（次数） |', '|---|---|---|---|---|---|']
  .concat(cbr.filter(([, o]) => o.n >= 2).slice(0, 20).map(([b, o]) => `| ${b} | ${o.n} | ${o.f} | ${o.shas.size} | ${o.s.toFixed(0)} | ${o.u.toFixed(0)}（${o.un}） |`));
C.push(`| **窗内合计 ${byBr.size} 个分支** | ${cbr.reduce((x, [, o]) => x + o.n, 0)} | ${cbr.reduce((x, [, o]) => x + o.f, 0)} | — | ${cbr.reduce((x, [, o]) => x + o.s, 0).toFixed(0)} | ${cbr.reduce((x, [, o]) => x + o.u, 0).toFixed(0)}（${cbr.reduce((x, [, o]) => x + o.un, 0)}） |`);

// ---- 注入
let doc = readFileSync(R + '1545-doc-prose.md', 'utf8');
const put = (k, v) => { if (!doc.includes(k)) throw new Error('占位符缺失 ' + k); doc = doc.split(k).join(v.join('\n')); };
put('{{TICKET_TABLE}}', T4); put('{{T1472_TABLE}}', T5); put('{{LEDGER_TABLE}}', A); put('{{S2ONLY_TABLE}}', B); put('{{CI_TABLE}}', C);
const OUT = 'D:/FL/wt-1545/docs/design/1545-mobile-gate3-rerun-reading.md';
writeFileSync(OUT, doc);
console.log(`已写 ${OUT}（${doc.length} 字符 / ${doc.split('\n').length} 行）`);
console.log(`自检：S1 ${s1.length} · 仅落盘 ${s2only.length} · 并集 ${s1.length + s2only.length} · 对上匹配 ${matched.length} · 逐票行 ${sorted.length} · 附录A ${A.length - 2} 行 · 附录B ${B.length - 2} 行 · 附录C ${C.length - 2} 行`);
console.log(`自检墙钟：S1 ${s1.reduce((x, r) => x + (Number.isFinite(num(r.seconds)) ? num(r.seconds) : 0), 0).toFixed(1)} + 仅落盘 ${s2only.reduce((x, r) => x + (Number.isFinite(num(r.sec)) ? num(r.sec) : 0), 0).toFixed(1)} = ${(s1.reduce((x, r) => x + (Number.isFinite(num(r.seconds)) ? num(r.seconds) : 0), 0) + s2only.reduce((x, r) => x + (Number.isFinite(num(r.sec)) ? num(r.sec) : 0), 0)).toFixed(1)} s`);
console.log(`自检字符：${s1.reduce((x, r) => x + (+r.chars || 0), 0)}`);
