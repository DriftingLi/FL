// 13 次「命令全量面但无汇总行」的孤儿跑：它们的 tool_result 到底留下了什么
import { readFileSync, readdirSync } from 'node:fs';
const D = 'C:/Users/ZHENG/.qoder/projects/d--FL/';
const rd = (p) => { const t = readFileSync(p, 'utf8').split('\n').filter(Boolean); const h = t.shift().split('\t'); return t.map((l) => { const c = l.split('\t'); const o = {}; h.forEach((k, i) => (o[k] = c[i])); return o; }); };
const v4 = rd('D:/FL/.scratch/research/1545-runs-v4.tsv');
const F = /jest --config jest\.config\.unit\.js/, N = /testPathPattern|dev-finish|-t ["']|--listTests|\.test\.js/;
const orphans = v4.filter((r) => r.bucket === '无汇总行' && F.test(r.cmd || '') && !N.test(r.cmd || ''));
const cache = new Map();
const load = (sid) => {
  if (cache.has(sid)) return cache.get(sid);
  const f = readdirSync(D).filter((x) => x.startsWith(sid) && x.endsWith('.jsonl'))[0];
  const t = f ? readFileSync(D + f, 'utf8').split('\n').filter(Boolean) : [];
  cache.set(sid, t); return t;
};
let withTime = 0, sumSec = 0, withSuites = 0;
const rows = [];
for (const o of orphans) {
  const t = load(o.sid);
  const i = t.findIndex((l) => l.includes(o.ts));
  let res = null;
  if (i >= 0) {
    const use = JSON.parse(t[i]);
    const callId = ((use.message?.content || []).find((x) => x.type === 'tool_use' && JSON.stringify(x.input || '').includes('jest --config jest.config.unit.js')) || {}).id;
    for (let j = i + 1; j < Math.min(i + 14, t.length); j++) {
      const r = JSON.parse(t[j]);
      const c = r.message && r.message.content;
      if (!Array.isArray(c)) continue;
      const tr = c.find((x) => x.type === 'tool_result' && (!callId || x.tool_use_id === callId));
      if (tr) { res = JSON.stringify(tr.content); break; }
    }
  }
  const sec = res ? parseFloat((res.match(/Time:\s*([\d.]+)\s*s/) || [])[1] || 'NaN') : NaN;
  const suites = res ? parseFloat((res.match(/Test Suites:[^\n]*?(\d+)\s+total/) || [])[1] || 'NaN') : NaN;
  if (Number.isFinite(sec)) { withTime++; sumSec += sec; }
  if (Number.isFinite(suites)) withSuites++;
  rows.push({ ts: o.ts, ticket: o.ticket || '-', sid: (o.sid || '').slice(0, 8), sec: Number.isFinite(sec) ? sec : null, suites: Number.isFinite(suites) ? suites : null, len: res ? res.length : null, head: res ? res.replace(/\\r\\n|\\n/g, ' ⏎ ').slice(2, 150) : '(未找到配对的 tool_result)' });
}
console.log(`13 次孤儿跑（实数 ${orphans.length}）：结果块里留有 Time: 行的 ${withTime} 次 · 留有 Test Suites: 汇总的 ${withSuites} 次`);
console.log(`留有 Time 的那些合计 ${sumSec.toFixed(1)} s`);
for (const r of rows) console.log(`| ${r.ts.slice(0, 19)} | ${r.ticket} | ${r.sid} | Time=${r.sec === null ? '无' : r.sec} | total=${r.suites === null ? '无' : r.suites} | 块长=${r.len} | ${r.head} |`);
