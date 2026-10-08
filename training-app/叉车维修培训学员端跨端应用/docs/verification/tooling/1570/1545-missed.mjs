// 漏计探测：bucket=无汇总行 但命令形态是「全量面」⇒ 汇总行没进上下文（多半被重定向进文件）
import { readFileSync } from 'node:fs';
const T = readFileSync('D:/FL/.scratch/research/1545-runs-v4.tsv','utf8').split('\n').filter(Boolean);
const h = T.shift().split('\t');
const rows = T.map(l=>{const c=l.split('\t');const o={};h.forEach((k,i)=>o[k]=c[i]);return o;});
const FULLISH = /npm run test:unit|npm test(?!\s*:unit)|jest --config jest\.config\.unit\.js\s*(-i)?\s*(2>&1|\||$|;)/;
const NARROW = /--testPathPattern|jest\.config\.unit\.js\s+\S*utils|dev-finish/;
const noSum = rows.filter(r=>r.bucket==='无汇总行');
const susp = noSum.filter(r=>FULLISH.test(r.cmd||'') && !NARROW.test(r.cmd||''));
console.log(`总调用=${rows.length} 无汇总行=${noSum.length} 其中疑似全量面=${susp.length} 其中带重定向=${susp.filter(r=>r.redirect==='Y').length}`);
const byTicket=new Map();
for(const r of susp){const k=r.ticket||'(未归属)';byTicket.set(k,(byTicket.get(k)||0)+1);}
console.log('疑似漏计·逐票: '+[...byTicket].sort((a,b)=>b[1]-a[1]).map(([k,v])=>`${k}=${v}`).join(' '));
// 命令里出现的落盘文件名
const files=new Set();
for(const r of susp){for(const m of (r.cmd||'').matchAll(/([A-Za-z]:[\/][^\s;"'|]+\.log)/g)) files.add(m[1]);}
console.log(`疑似漏计里点名的 .log 落盘路径 ${files.size} 个:`);
[...files].sort().forEach(f=>console.log('  '+f));
