// S2v2：全仓兜底扫描（我自己的口径），对账子代理的 74 次本机全量
// 判据：文件里出现 "Test Suites:" ⇒ 是 jest 产物；total 套件 ≥ 100 ⇒ 记为「全量档」
//       （与 1545-report.mjs 的全量档判据同一条，保持可对照）
// 排除：node_modules / .git / unpackage / kotlin-class / dist / .qoder
import { readdirSync, statSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = 'D:/FL';
const SKIP = new Set(['node_modules', '.git', 'unpackage', 'kotlin-class', 'dist', '.qoder', 'kotlin-build', '.gradle']);
const files = [];
(function walk(d, depth) {
  if (depth > 9) return;
  let ents; try { ents = readdirSync(d, { withFileTypes: true }); } catch { return; }
  for (const e of ents) {
    if (SKIP.has(e.name)) continue;
    const p = join(d, e.name);
    if (e.isDirectory()) walk(p, depth + 1);
    else if (e.isFile()) {
      let st; try { st = statSync(p); } catch { continue; }
      if (st.size === 0 || st.size > 40 * 1024 * 1024) continue;
      files.push({ p, size: st.size, mtime: st.mtimeMs });
    }
  }
})(ROOT, 0);

const hits = [];
for (const f of files) {
  let buf;
  try { buf = readFileSync(f.p); } catch { continue; }
  if (!buf.includes('Test Suites:')) continue;
  const txt = buf.toString('utf8');
  // 真跑产物判据：文件里有 jest 的逐套件结果行（PASS/FAIL utils/xxx.test.js）。
  // 引文类（machine-lines.txt / PR 正文 json / md 摘录）只抄末尾汇总，没有逐套件行。
  const banner = /forklift-training-app@1\.0\.0 test:unit|(?:^|\n)> jest --config/m.test(txt);
  const real = /(?:^|\n)(?:PASS|FAIL) utils\/[A-Za-z0-9_.-]+\.test\.js/.test(txt);
  const lines = txt.split(/\r?\n/);
  // 取最后一次汇总（一个文件可能有多次跑的拼接）
  const sums = [];
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/Test Suites:\s*(.*)$/);
    if (!m) continue;
    const seg = lines.slice(i, i + 6).join('\n');
    const tot = (seg.match(/(\d+)\s+total/) || [])[1];
    const tm = (seg.match(/Time:\s*([\d.]+)\s*s/) || [])[1];
    const fl = (m[1].match(/(\d+)\s+failed/) || [])[1];
    sums.push({ line: m[1].trim(), total: tot ? +tot : NaN, sec: tm ? +tm : NaN, fail: fl ? +fl : 0, idx: i });
  }
  if (!sums.length) continue;
  hits.push({ ...f, n: sums.length, real, banner, last: sums[sums.length - 1], sums, fail: sums[sums.length - 1].fail, rel: f.p.replace(/\\/g, '/').replace('D:/FL/', '') });
}

const WIN_A = Date.parse('2026-09-22T07:48:39.133Z');
const WIN_B = Date.parse('2026-10-04T16:21:59.177Z');
const isCI = (t) => /Current runner version|##\[group\]Runner Image|Hosted Compute Agent|^mobile-test\t/m.test(t.slice(0, 4000));

console.log(`## S2v2 全仓兜底扫描（我自己的口径）`);
console.log(`扫了 ${files.length} 个非空文件（排除 node_modules/.git/unpackage/kotlin-class/dist/.qoder/kotlin-build/.gradle，深度 ≤9，≤40 MB）`);
console.log(`含 "Test Suites:" 的文件 ${hits.length} 个`);
const multi = hits.filter((h) => h.n > 1);
console.log(`其中同一文件里出现 ≥2 次汇总行的 ${multi.length} 个（拼接型，逐次计入）`);
const runs = [];
for (const h of hits) for (const s of h.sums) runs.push({ rel: h.rel, size: h.size, mtime: h.mtime, total: s.total, sec: s.sec, line: s.line, n: h.n, real: h.real, banner: h.banner, fail: s.fail, head: isCI(readFileSync(h.p, 'utf8').slice(0, 4000)) });
console.log(`汇总行合计 ${runs.length} 条（一次跑产一条汇总 ⇒ 按条计次）`);
const CI = runs.filter((r) => r.head);
const QUOTE = runs.filter((r) => !r.head && !r.real);
const LO = runs.filter((r) => !r.head && r.real);
console.log(`按开头判 CI job 日志的 ${CI.length} 条（不可与本机混算）；无 npm 横幅判为引文/摘录的 ${QUOTE.length} 条（分布在 ${new Set(QUOTE.map((r) => r.rel)).size} 个文件）；本机真跑产物 ${LO.length} 条（${new Set(LO.map((r) => r.rel)).size} 个文件）`);
const full = LO.filter((r) => Number.isFinite(r.total) && r.total >= 100);
const inwin = full.filter((r) => r.mtime >= WIN_A && r.mtime <= WIN_B);
console.log(`本机全量档（total ≥ 100 套件）${full.length} 次；其中落在观察窗 2026-09-22T07:48:39Z..2026-10-04T16:21:59Z 内的 ${inwin.length} 次`);
const sum = (a) => a.reduce((x, y) => x + y, 0);
const withT = (a) => a.filter((r) => Number.isFinite(r.sec));
console.log(`窗内全量档：有 Time 行 ${withT(inwin).length} 条 · Σ Time ${sum(withT(inwin).map((r) => r.sec)).toFixed(1)} s = ${(sum(withT(inwin).map((r) => r.sec)) / 60).toFixed(1)} min = ${(sum(withT(inwin).map((r) => r.sec)) / 3600).toFixed(2)} h · 字节合计 ${sum(inwin.map((r) => r.size))}`);
console.log(`窗外（更早）全量档 ${full.length - inwin.length} 条 · Σ Time ${sum(withT(full.filter((r) => !inwin.includes(r))).map((r) => r.sec)).toFixed(1)} s`);
console.log(`本机非全量档（total < 100 或无 total）${LO.length - full.length} 条`);
writeFileSync('D:/FL/.scratch/research/1545-s2v2.tsv', ['mtime\tiso\treal\tbanner\tCI\ttotal\tfail\tsec\tsize\tn\trel'].concat(runs.map((r) => [r.mtime.toFixed(0), new Date(r.mtime).toISOString(), r.real ? 'Y' : 'N', r.banner ? 'Y' : 'N', r.head ? 'Y' : 'N', Number.isFinite(r.total) ? r.total : '', r.fail || '', Number.isFinite(r.sec) ? r.sec : '', r.size, r.n, r.rel.replace(/\t/g, ' ')].join('\t'))).join('\n') + '\n');
console.log(`已写 1545-s2v2.tsv（逐条汇总行 + 分类标记，供对账）`);
console.log(`\n--- 窗内全量档逐条（按 mtime）---`);
console.log('| mtime (UTC) | total 套件 | 失败套件 | Time s | B | 路径 |');
console.log('|---|---|---|---|---|---|');
for (const r of inwin.sort((a, b) => a.mtime - b.mtime)) {
  console.log(`| ${new Date(r.mtime).toISOString().slice(5, 19).replace('T', ' ')} | ${r.total} | ${r.fail || 0} | ${Number.isFinite(r.sec) ? r.sec : '—'} | ${r.size} | ${r.rel} |`);
}
console.log(`\n--- 窗外（2026-09-22T07:48Z 之前）全量档逐条 ---`);
for (const r of full.filter((x) => !inwin.includes(x)).sort((a, b) => a.mtime - b.mtime)) {
  console.log(`| ${new Date(r.mtime).toISOString().slice(0, 19).replace('T', ' ')} | ${r.total} | ${r.fail || 0} | ${Number.isFinite(r.sec) ? r.sec : '—'} | ${r.size} | ${r.rel} |`);
}
console.log(`\n--- CI job 日志（${CI.length} 条）---`);
for (const r of CI.sort((a, b) => a.mtime - b.mtime)) console.log(`| ${new Date(r.mtime).toISOString().slice(0, 19).replace('T', ' ')} | ${r.total} | ${Number.isFinite(r.sec) ? r.sec : '—'} | ${r.size} | ${r.rel} |`);
console.log(`\n--- 拼接型文件（一个文件里 ≥2 次汇总）---`);
for (const h of multi) console.log(`| ${new Date(h.mtime).toISOString().slice(0, 19).replace('T', ' ')} | ${h.n} 次 | ${h.sums.map((s) => s.total + '套件/' + (Number.isFinite(s.sec) ? s.sec + 's' : '—')).join(' · ')} | ${h.rel} |`);
