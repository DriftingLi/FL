import { readFileSync } from 'node:fs';
const D = 'C:/Users/ZHENG/.qoder/projects/d--FL/';
const f = process.argv[2] || '0d943f13-b332-467f-8f45-881a6256014a.jsonl';
const from = +(process.argv[3] || 2303), to = +(process.argv[4] || 2312);
const t = readFileSync(D + f, 'utf8').split('\n').filter(Boolean);
for (let i = from; i < Math.min(to, t.length); i++) {
  const o = JSON.parse(t[i]);
  const c = o.message && o.message.content;
  let txt = '';
  if (Array.isArray(c)) {
    txt = c.map((x) => {
      if (x.type === 'text') return 'TEXT:' + String(x.text).slice(0, 60).replace(/\n/g, ' ');
      if (x.type === 'tool_use') return 'USE id=' + x.id + ' ' + JSON.stringify(x.input).slice(0, 90);
      if (x.type === 'tool_result') { const s = JSON.stringify(x.content); return 'RES of=' + String(x.tool_use_id) + ' len=' + s.length + ' hasTestSuites=' + /Test Suites:/.test(s) + ' head=' + s.slice(0, 200); }
      return x.type;
    }).join(' || ');
  }
  console.log('---', i, o.type, String(o.timestamp).slice(11, 23), txt.slice(0, 320));
}
