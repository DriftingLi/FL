// #1545 终版账：S1 富化（P1/P2 从落盘补 Time 与字节）+ S2-only 单列 + 逐档「若消除可省多少」
import { readFileSync, writeFileSync } from 'node:fs';
const rd = (p) => { const T = readFileSync(p, 'utf8').split('\n').filter(Boolean); const h = T.shift().split('\t'); return T.map(l => { const c = l.split('\t'); const o = {}; h.forEach((k, i) => o[k] = c[i]); return o; }); };
const s2 = rd('D:/FL/.scratch/research/1545-s2-local.tsv').map(o => ({ ...o, mt: Date.parse(o.mtime), size: +o.size, sf: +o.suitesFailed, tot: +o.suitesTotal, sec: o.seconds === '' ? NaN : +o.seconds }));
const s1raw = rd('D:/FL/.scratch/research/1545-runs-v4.tsv').filter(r => r.bucket === '全量③').map(r => ({
  ts: Date.parse(r.ts), sec: r.seconds === '' ? NaN : +r.seconds, tot: +r.total, f: +r.fSuites || 0,
  verdict: r.verdict, ticket: r.ticket, sid: r.sid, redirect: r.redirect, chars: +r.resultChars || 0,
  fails: r.fails || '', eCode: +r.eCode || 0, eDoc: +r.eDoc || 0, eOther: +r.eOther || 0, gOps: +r.gOps || 0,
  src: r.src, used: false, how: '', bytes: '', bfile: '',
}));
for (const a of s1raw) {
  if (isNaN(a.sec)) continue;
  const b = s2.find(x => !x._u && x.tot === a.tot && Math.abs(x.sec - a.sec) < 0.5);
  if (b) { b._u = true; a.used = true; a.how = 'P1'; a.bytes = b.size; a.bfile = b.path; }
}
for (const b of s2.filter(x => !x._u)) {
  const cand = s1raw.filter(a => !a.used && a.tot === b.tot && a.f === b.sf && (b.mt - a.ts) >= 30000 && (b.mt - a.ts) <= 1800000);
  if (cand.length) {
    cand.sort((x, y) => Math.abs(x.ts - b.mt) - Math.abs(y.ts - b.mt));
    const a = cand[0]; a.used = true; a.how = 'P2'; a.bytes = b.size; a.sec = b.sec; a.bfile = b.path; b._u = true; b.s1 = a;
  }
}
const s2only = s2.filter(b => !b._u);
const endOf = r => r.ts + (isNaN(r.sec) ? 120000 : r.sec * 1000);
for (const a of s1raw) a.overlap = s1raw.filter(b => b.sid !== a.sid && b.ts < endOf(a) && endOf(b) > a.ts).length;
const WALL = /kotlinAllProcessCaptureBehavior|hxLaunchDetachBehavior/;
function cause(r) {
  const red = r.verdict === '红';
  if (red && WALL.test(r.fails)) return 'C3';
  if (red && r.overlap > 0 && r.sec > 400) return 'C3';
  if (red && !r.fails.trim()) return 'C1';
  if (r.eCode > 0) return 'C4';
  if (r.eDoc > 0) return 'C2a';
  if (r.eOther > 0) return 'C2d';
  if (r.gOps > 0) return 'C2b';
  return 'C2c';
}
for (const r of s1raw) r.cause = cause(r);
const sum = (a, f) => a.reduce((x, y) => x + f(y), 0);
const stat = a => {
  const s = a.map(r => r.sec).filter(x => !isNaN(x)).sort((p, q) => p - q);
  const tot = s.reduce((p, q) => p + q, 0);
  const wb = a.filter(r => r.bytes !== '');
  return {
    n: a.length, red: a.filter(r => r.verdict === '红').length, nT: s.length, sum: tot,
    mean: s.length ? tot / s.length : NaN, med: s.length ? s[Math.floor(s.length / 2)] : NaN,
    min: s.length ? s[0] : NaN, max: s.length ? s[s.length - 1] : NaN,
    chars: sum(a, r => r.chars), nBytes: wb.length, bytes: wb.length ? sum(wb, r => +r.bytes) : 0,
  };
};
const f1 = x => (isNaN(x) ? '—' : x.toFixed(1));
const LAB = {
  C1: 'C1 红跑但失败原因没进上下文 ⇒ 只能再跑一次去看',
  C2a: 'C2a 其间只改了文档（全量与被改动无因果）',
  C2b: 'C2b 其间只有 git / 建树动作（sha 前进那一族）',
  C2c: 'C2c 其间什么都没动（纯重跑）',
  C2d: 'C2d 其间只动了其它面（非被测代码、非文档）',
  C3: 'C3 并发假红（墙钟断言 / 5s 默认超时抖动）',
  C4: 'C4 改完复验（被测面代码动过 ⇒ 该跑）',
};
const order = ['C4', 'C2a', 'C2b', 'C2c', 'C2d', 'C1', 'C3'];
const perC = new Map();
for (const r of s1raw) { if (!perC.has(r.cause)) perC.set(r.cause, []); perC.get(r.cause).push(r); }
const L = [];
const P = (...x) => L.push(...x);

P('## A. 并集总账');
const U = s1raw.length + s2only.length;
const usec = sum(s1raw.filter(r => !isNaN(r.sec)), r => r.sec) + sum(s2only.filter(b => !isNaN(b.sec)), b => b.sec);
const nT = s1raw.filter(r => !isNaN(r.sec)).length + s2only.filter(b => !isNaN(b.sec)).length;
P(`全量 ③ 至少 ${U} 次 = S1（会话转录）${s1raw.length} + 仅落盘可证 ${s2only.length}`);
P(`可测墙钟 ${usec.toFixed(1)} s = ${(usec / 60).toFixed(1)} min = ${(usec / 3600).toFixed(2)} h（有 Time 行 ${nT}/${U} = ${(nT / U * 100).toFixed(1)} 个百分点）`);
P(`进上下文的 tool_result 字符 ${sum(s1raw, r => r.chars)}（只有 S1 有这一栏；单次中位 ${s1raw.map(r => r.chars).sort((a, b) => a - b)[Math.floor(s1raw.length / 2)]}）`);
P(`落盘字节 ${sum(s2, b => b.size)}（S2 共 ${s2.length} 份本机全量产物；其中 ${s2.length - s2only.length} 份与 S1 对上）`);
P(`两源对账：P1 精确 ${s1raw.filter(r => r.how === 'P1').length} · P2 极可能 ${s1raw.filter(r => r.how === 'P2').length} · 仅 S1 ${s1raw.filter(r => !r.used).length} · 仅 S2 ${s2only.length}`);

P('', '## B. 归因分档（只对 S1 的 ' + s1raw.length + ' 次可归因；S2-only ' + s2only.length + ' 次没有转录 ⇒ 归因不可证）');
P('| 档 | 次数 | 红 | 有 Time | 墙钟合计 s | 均值 s | 中位 s | 进上下文字符 | 对上落盘的次数 | 落盘字节 |');
P('|---|---|---|---|---|---|---|---|---|---|');
for (const k of order) { const a = perC.get(k); if (!a) continue; const s = stat(a); P(`| ${LAB[k]} | ${s.n} | ${s.red} | ${s.nT} | ${f1(s.sum)} | ${f1(s.mean)} | ${f1(s.med)} | ${s.chars} | ${s.nBytes} | ${s.bytes || '—'} |`); }
const nc = stat(s1raw.filter(r => r.cause !== 'C4'));
P(`| **非 C4 小计** | ${nc.n} | ${nc.red} | ${nc.nT} | ${f1(nc.sum)} | ${f1(nc.mean)} | ${f1(nc.med)} | ${nc.chars} | ${nc.nBytes} | ${nc.bytes || '—'} |`);
const s2s = { n: s2only.length, red: s2only.filter(b => b.sf > 0).length, nT: s2only.filter(b => !isNaN(b.sec)).length, sum: sum(s2only.filter(b => !isNaN(b.sec)), b => b.sec), bytes: sum(s2only, b => b.size) };
P(`| S2-only（归因不可证） | ${s2s.n} | ${s2s.red} | ${s2s.nT} | ${f1(s2s.sum)} | — | — | — | ${s2s.n} | ${s2s.bytes} |`);

P('', '### 每档「若消除可省多少」算式（墙钟 + 字符量两栏；现读代理口径；不给百分比、不给金额）');
P('| 档 | 墙钟算式 | 字符量算式 |');
P('|---|---|---|');
for (const k of order) {
  const a = perC.get(k); if (!a) continue; const s = stat(a);
  const wall = k === 'C4'
    ? '0 s —— C4 是该跑的。能省的只有「同一道门跑得更快 / 更少跑几遍」，那属 #1410（去 -i）与 #1544（拆墙钟断言）射程，不属本档'
    : `${a.filter(r => !isNaN(r.sec)).length} 次有 Time，合计 ${f1(s.sum)} s；÷ ${a.length} 次 = 每次 ${f1(s.mean)} s。该档 ${a.length} 次全消除 ⇒ 省 ${f1(s.sum)} s = ${f1(s.sum / 60)} min`;
  const ch = `${s.chars} 字符 = 该档 ${a.length} 次进上下文的 tool_result 之和（单次中位 ${a.map(r => r.chars).sort((p, q) => p - q)[Math.floor(a.length / 2)]}）`;
  P(`| ${LAB[k]} | ${wall} | ${ch} |`);
}
P(`| **非 C4 小计** | ${f1(nc.sum)} s = ${f1(nc.sum / 60)} min（${nc.nT}/${nc.n} 次有 Time） | ${nc.chars} 字符 |`);
P(`| S2-only | ${f1(s2s.sum)} s = ${f1(s2s.sum / 60)} min（${s2s.nT}/${s2s.n} 次有 Time） | 无此栏（汇总行没进上下文）；落盘 ${s2s.bytes} B 只能作体量代理，与「进上下文字符」不同源、不可相加 |`);

P('', '## C. 逐票（S1 归因 + 仅落盘可证的次数）');
P('| 票 | S1 全量 | 红 | C4 | C2a | C2b | C2c | C2d | C1 | C3 | S1 墙钟 s | 进上下文字符 | 仅落盘 | 落盘字节 | 会话数 |');
P('|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|');
const perT = new Map();
const get = k => { if (!perT.has(k)) perT.set(k, { a: [], sids: new Set(), s2n: 0, s2b: 0, s2sec: 0, guess: false }); return perT.get(k); };
for (const r of s1raw) { const o = get(r.ticket || '(未归属)'); o.a.push(r); o.sids.add(r.sid); }
for (const b of s2only) {
  const m = b.path.match(/(\d{3,4})/g);
  const k = m && m.length ? m[m.length - 1] : '(未归属)';
  const o = get(k); o.s2n++; o.s2b += b.size; if (!isNaN(b.sec)) o.s2sec += b.sec; o.guess = true;
}
for (const [k, o] of [...perT].sort((p, q) => (q[1].a.length + q[1].s2n) - (p[1].a.length + p[1].s2n))) {
  const s = stat(o.a); const c = x => o.a.filter(r => r.cause === x).length;
  P(`| ${k}${o.guess && !o.a.length ? '†' : ''} | ${o.a.length || '—'} | ${o.a.length ? s.red : '—'} | ${o.a.length ? c('C4') : '—'} | ${o.a.length ? c('C2a') : '—'} | ${o.a.length ? c('C2b') : '—'} | ${o.a.length ? c('C2c') : '—'} | ${o.a.length ? c('C2d') : '—'} | ${o.a.length ? c('C1') : '—'} | ${o.a.length ? c('C3') : '—'} | ${o.a.length ? f1(s.sum) : '—'} | ${o.a.length ? s.chars : '—'} | ${o.s2n || '—'} | ${o.s2n ? o.s2b : '—'} | ${o.sids.size || '—'} |`);
}
P('');
P('† 该票号是从落盘路径里的数字段猜的（无转录可核），只作线索、不承重。');

P('', '## D. #1472 六次全量样本（票面 AC① 点名的那一批）');
P('| # | 落盘文件 | 字节 | 套件 | 失败套件 | jest Time s | 两源对账 | S1 命令 ts (UTC) | 票 / 会话 |');
P('|---|---|---|---|---|---|---|---|---|');
const six = ['wt1472-full1.log', 'wt1472-full2.log', 'wt1472-full3.log', 'wt1472-full4.log', 'wt1472-full-3.log', 'wt1472-full-3b.log'];
let tb = 0, tt = 0;
six.forEach((n, i) => {
  const b = s2.find(x => x.path.endsWith(n));
  const a = s1raw.find(r => r.bfile && r.bfile.endsWith(n));
  const how = a ? (a.how === 'P1' ? 'P1 精确' : 'P2 极可能') : '仅 S2（S1 漏计）';
  if (b) { tb += b.size; tt += isNaN(b.sec) ? 0 : b.sec; }
  P(`| ${i + 1} | .scratch/${n} | ${b ? b.size : '?'} | ${b ? b.tot : '?'} | ${b ? b.sf : '?'} | ${b ? b.sec : '?'} | ${how} | ${a ? new Date(a.ts).toISOString() : '—'} | ${a ? (a.ticket || '-') + ' / ' + a.sid.slice(0, 8) : '—'} |`);
});
P(`| 合计 | 6 份 | ${tb} B | — | — | ${tt.toFixed(3)} s = ${(tt / 60).toFixed(1)} min | — | — | — |`);

writeFileSync('D:/FL/.scratch/research/1545-s1-enriched.tsv',
  'ts\tticket\tsid\tcause\tverdict\tsuites\tfailed\tseconds\tbytes\tchars\teCode\teDoc\teOther\tgOps\toverlap\thow\tbfile\tfails\n' +
  s1raw.sort((p, q) => p.ts - q.ts).map(r => [new Date(r.ts).toISOString(), r.ticket, r.sid, r.cause, r.verdict, r.tot, r.f, isNaN(r.sec) ? '' : r.sec, r.bytes, r.chars, r.eCode, r.eDoc, r.eOther, r.gOps, r.overlap, r.how, r.bfile, r.fails.replace(/\t/g, ' ')].join('\t')).join('\n') + '\n');
P('', '已写 1545-s1-enriched.tsv');
console.log(L.join('\n'));
