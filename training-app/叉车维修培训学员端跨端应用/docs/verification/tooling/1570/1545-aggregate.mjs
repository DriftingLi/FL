// #1545 · 把 1545-jsonl-jest-runs.tsv 收敛成「有 jest 汇总行作证据」的执行记录，并按规模分档
// 判据：只认 tool_result 里真的出现 `Test Suites:` 汇总行的那些调用（命令文本里提到命令名的假阳性由此被剔掉）
import { readFileSync } from 'node:fs';

const rows = readFileSync('D:/FL/.scratch/research/1545-jsonl-jest-runs.tsv', 'utf8')
  .split('\n').filter(Boolean).slice(1)
  .map((l) => {
    const c = l.split('\t');
    return { sid: c[0], side: c[1], ts: c[2], branch: c[3], cwd: c[4], kind: c[5], chars: +c[6] || 0, suites: c[7], tests: c[8], time: c[9], cmd: c[10] };
  });

const num = (s, re) => { const m = String(s).match(re); return m ? +m[1] : NaN; };
const withSum = rows.filter((r) => r.suites && /Test Suites:/.test(r.suites));

for (const r of withSum) {
  r.total = num(r.suites, /(\d+)\s+total/);
  r.failedSuites = num(r.suites, /(\d+)\s+failed/);
  r.testsTotal = num(r.tests, /(\d+)\s+total/);
  r.testsFailed = num(r.tests, /(\d+)\s+failed/);
  r.seconds = num(r.time, /Time:\s*([\d.]+)\s*s/);
  // 档位：全量 = 100+ 套件；收窄 = 10..99；单套件 = <10
  r.bucket = r.total >= 100 ? '全量③' : (r.total >= 10 ? '收窄③' : '单套件③');
  r.verdict = (r.failedSuites > 0 || r.testsFailed > 0) ? '红' : '绿';
  // 归属：worktree 目录名里的票号优先，其次分支名里的票号
  const wt = (r.cwd || '').match(/wt[-_]?(\d{3,4})/i);
  const br = (r.branch || '').match(/(\d{3,4})/);
  r.ticket = wt ? wt[1] : (br ? br[1] : '');
  r.attr = wt ? `cwd:${wt[0]}` : (br ? `branch:${r.branch}` : 'cwd+branch 无票号');
}

console.log(`总命中行=${rows.length}  其中有 jest 汇总行=${withSum.length}  被剔掉的假阳性=${rows.length - withSum.length}`);
const byBucket = new Map();
for (const r of withSum) byBucket.set(r.bucket, (byBucket.get(r.bucket) || 0) + 1);
console.log('--- 按规模档位 ---');
for (const [k, v] of [...byBucket].sort((a, b) => b[1] - a[1])) console.log(`${v}\t${k}`);

const full = withSum.filter((r) => r.bucket === '全量③');
console.log(`\n--- 全量③ 明细（${full.length} 次）---`);
console.log('ts\tseconds\tverdict\tsuites\ttests\tresultChars\tticket\tattr\tside\tcwd\tbranch');
for (const r of full.sort((a, b) => a.ts.localeCompare(b.ts))) {
  console.log([r.ts, r.seconds, r.verdict, `${r.failedSuites || 0}f/${r.total}`, `${r.testsFailed || 0}f/${r.testsTotal}`, r.chars, r.ticket || '-', r.attr, r.side, r.cwd, r.branch].join('\t'));
}

console.log('\n--- 全量③ 按票号汇总 ---');
const perTicket = new Map();
for (const r of full) {
  const k = r.ticket || '(无票号)';
  if (!perTicket.has(k)) perTicket.set(k, { n: 0, red: 0, sec: 0, chars: 0, first: r.ts, last: r.ts, secs: [] });
  const o = perTicket.get(k);
  o.n++; if (r.verdict === '红') o.red++;
  if (!isNaN(r.seconds)) { o.sec += r.seconds; o.secs.push(r.seconds); }
  o.chars += r.chars;
  if (r.ts < o.first) o.first = r.ts;
  if (r.ts > o.last) o.last = r.ts;
}
console.log('ticket\truns\tred\tsumSeconds\tsumResultChars\tfirst\tlast\tsecondsList');
for (const [k, o] of [...perTicket].sort((a, b) => b[1].n - a[1].n)) {
  console.log([k, o.n, o.red, o.sec.toFixed(1), o.chars, o.first, o.last, o.secs.map((x) => x.toFixed(1)).join(',')].join('\t'));
}

console.log('\n--- 时间窗 ---');
const all = withSum.map((r) => r.ts).sort();
console.log(`最早=${all[0]}  最晚=${all[all.length - 1]}`);
const fullTs = full.map((r) => r.ts).sort();
console.log(`全量③ 最早=${fullTs[0]}  最晚=${fullTs[fullTs.length - 1]}`);
