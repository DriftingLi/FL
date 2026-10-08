// 终版对账：S1(转录) × S2(本机落盘，已剔除 CI 日志/评论回读/手写汇总)
import { readFileSync, writeFileSync } from 'node:fs';
const s2 = readFileSync('D:/FL/.scratch/research/1545-s2-local.tsv','utf8').split('\n').filter(Boolean);
const sh = s2.shift().split('\t');
const S2 = s2.map(l=>{const c=l.split('\t');const o={};sh.forEach((k,i)=>o[k]=c[i]);return {...o, mt:Date.parse(o.mtime), size:+o.size, suitesFailed:+o.suitesFailed, total:+o.suitesTotal, sec:o.seconds===''?NaN:+o.seconds};});
const T = readFileSync('D:/FL/.scratch/research/1545-runs-v4.tsv','utf8').split('\n').filter(Boolean);
const h = T.shift().split('\t');
const S1 = T.map(l=>{const c=l.split('\t');const o={};h.forEach((k,i)=>o[k]=c[i]);return o;})
  .filter(r=>r.bucket==='全量③')
  .map(r=>({ ts:Date.parse(r.ts), sec:r.seconds===''?NaN:+r.seconds, total:+r.total, f:+r.fSuites||0, verdict:r.verdict, ticket:r.ticket, sid:r.sid, redirect:r.redirect, cmd:r.cmd, chars:+r.resultChars||0, used:false, how:'' }));
for (const a of S1) { if (isNaN(a.sec)) continue; const b = S2.find(x=>!x._u && x.total===a.total && Math.abs(x.sec-a.sec)<0.5); if (b) { b._u=true; a.used=true; a.how='P1'; a.b=b; } }
const p2=[];
for (const b of S2.filter(x=>!x._u)) {
  const cand = S1.filter(a=>!a.used && a.total===b.total && a.f===b.suitesFailed && (b.mt-a.ts)>=30000 && (b.mt-a.ts)<=1800000);
  if (cand.length) { cand.sort((x,y)=>Math.abs(x.ts-b.mt)-Math.abs(y.ts-b.mt)); const a=cand[0]; a.used=true; a.how='P2'; a.b=b; b._u=true; b.s1=a; p2.push(b); }
}
const s1only = S1.filter(a=>!a.used), s2only = S2.filter(b=>!b._u);
const sum=(a,f)=>a.reduce((x,y)=>x+f(y),0);
console.log(`S1 转录全量 ${S1.length} 次（有 Time ${S1.filter(a=>!isNaN(a.sec)).length}，Time 合计 ${sum(S1.filter(a=>!isNaN(a.sec)),a=>a.sec).toFixed(1)} s）`);
console.log(`S2 本机落盘全量 ${S2.length} 次（全有 Time，Time 合计 ${sum(S2.filter(b=>!isNaN(b.sec)),b=>b.sec).toFixed(1)} s，字节 ${sum(S2,b=>b.size)}）`);
console.log(`\nP1 精确对上（Time+套件数全等）= ${S1.filter(a=>a.how==='P1').length}`);
console.log(`P2 极可能同一次（套件数与失败数全等、落盘 mtime 落在命令 ts + 30s..1800s）= ${p2.length}`);
console.log(`仅 S2 可证（转录里完全没有对应行 ⇒ S1 漏计）= ${s2only.length} · 字节 ${sum(s2only,b=>b.size)} · Time ${sum(s2only.filter(b=>!isNaN(b.sec)),b=>b.sec).toFixed(1)} s`);
console.log(`仅 S1 可证（跑了没落盘，或产物已清/在窗口外）= ${s1only.length} · 有 Time ${s1only.filter(a=>!isNaN(a.sec)).length} · Time ${sum(s1only.filter(a=>!isNaN(a.sec)),a=>a.sec).toFixed(1)} s`);
const union = S1.length + s2only.length;
const usec = sum(S1.filter(a=>!isNaN(a.sec)),a=>a.sec) + sum(s2only.filter(b=>!isNaN(b.sec)),b=>b.sec) + sum(p2.filter(b=>!isNaN(b.sec)&&isNaN(b.s1.sec)),b=>b.sec);
const nTime = S1.filter(a=>!isNaN(a.sec)).length + s2only.filter(b=>!isNaN(b.sec)).length + p2.filter(b=>!isNaN(b.sec)&&isNaN(b.s1.sec)).length;
console.log(`\n=== 并集（下界）===\n全量 ③ 至少 ${union} 次 · 可测墙钟 ${usec.toFixed(1)} s = ${(usec/60).toFixed(1)} min（有 Time ${nTime} 次，覆盖率 ${(nTime/union*100).toFixed(1)} 个百分点）`);
console.log(`\n--- 仅 S2 可证的 ${s2only.length} 次 ---`);
for (const b of s2only.sort((x,y)=>x.mt-y.mt)) console.log(`${new Date(b.mt).toISOString()}  ${String(b.size).padStart(7)}B  ${b.total}套件/${b.suitesFailed}失败  ${String(b.sec).padStart(7)}s  ${b.path}`);
console.log(`\n--- P2 的 ${p2.length} 对（S1 行拿到了落盘的 Time）---`);
for (const b of p2.sort((x,y)=>x.mt-y.mt)) console.log(`${b.path.split('FL').pop()}  ${b.size}B  ${b.sec}s  ←票 ${b.s1.ticket||'-'} ts=${new Date(b.s1.ts).toISOString()} Δ=${((b.mt-b.s1.ts)/1000).toFixed(0)}s`);
writeFileSync('D:/FL/.scratch/research/1545-union.tsv',
  'src\tts_utc\tticket\tsid\tsuitesTotal\tsuitesFailed\tseconds\tbytes\tverdict\n' +
  S1.map(a=>`S1\t${new Date(a.ts).toISOString()}\t${a.ticket||''}\t${a.sid}\t${a.total}\t${a.f}\t${isNaN(a.sec)?'':a.sec}\t${a.how==='P1'||a.how==='P2'?a.b.size:''}\t${a.verdict}`).join('\n') + '\n' +
  s2only.map(b=>`S2only\t${new Date(b.mt).toISOString()}\t\t\t${b.total}\t${b.suitesFailed}\t${isNaN(b.sec)?'':b.sec}\t${b.size}\t${b.suitesFailed>0?'红':'绿'}`).join('\n') + '\n');
console.log('\n已写 1545-union.tsv');
