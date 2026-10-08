// #1545 · 报表生成：读 1545-runs-v4.tsv，产出调研件里要贴的全部表（markdown）
// 归因规则（优先级从上到下，命中即止；全部只依赖 TSV 里的列，可复算）：
//   C3 并发假红   = verdict=红 且 (fails 命中墙钟套件 或 (overlap>0 且 seconds>400))
//   C1 失败原因未入上下文 = verdict=红 且 fails 为空（结果里没有 FAIL 行 ⇒ 只知道红了、不知道谁红）
//   C4 改完复验   = eCode>0（两次全量之间动过被测面代码：training-app/ 或 utils/ 或 .uts/.uvue/.test.js/.ps1/.js/...）
//   C2a 只改文档  = eCode=0 且 eDoc>0
//   C2b 只有 git/建树动作 = eCode=0 且 eDoc=0 且 eOther=0 且 gOps>0
//   C2c 什么都没动 = eCode=0 且 eDoc=0 且 eOther=0 且 gOps=0
//   C2d 只动了其它面 = eCode=0 且 eDoc=0 且 eOther>0
//   C0 会话内首次全量（同会话无上一次全量作对照）→ 仍按上面规则归类，另在首跑列标记
import { readFileSync } from 'node:fs';

const P = 'D:/FL/.scratch/research/1545-runs-v4.tsv';
const T = readFileSync(P, 'utf8').split('\n').filter(Boolean);
const head = T.shift().split('\t');
const rows = T.map((l) => { const c = l.split('\t'); const o = {}; head.forEach((h, i) => o[h] = c[i]); return o; });
const N = (x) => (x === '' || x === undefined ? NaN : +x);

const all = rows.map((r) => ({ ...r, seconds: N(r.seconds), chars: N(r.resultChars) || 0, t: Date.parse(r.ts) }));
const full = all.filter((r) => r.bucket === '全量③').sort((a, b) => a.t - b.t);

// 并发重叠（跨会话，区间 = [ts, ts+seconds]；无 seconds 的按 120 s 保守占位）
const endOf = (r) => r.t + (isNaN(r.seconds) ? 120000 : r.seconds * 1000);
for (const a of full) a.overlap = full.filter((b) => b.sid !== a.sid && b.t < endOf(a) && endOf(b) > a.t).length;

// 同会话上一次全量
const bySess = new Map();
for (const r of full) { if (!bySess.has(r.sid)) bySess.set(r.sid, []); bySess.get(r.sid).push(r); }
for (const [, arr] of bySess) arr.forEach((r, i) => { r.prev = arr[i - 1] || null; r.firstInSess = i === 0; });

const WALL = /kotlinAllProcessCaptureBehavior|hxLaunchDetachBehavior/;
function cause(r) {
  const red = r.verdict === '红';
  if (red && WALL.test(r.fails || '')) return 'C3';
  if (red && r.overlap > 0 && (r.seconds > 400)) return 'C3';
  if (red && !(r.fails || '').trim()) return 'C1';
  const ec = N(r.eCode) || 0, ed = N(r.eDoc) || 0, eo = N(r.eOther) || 0, g = N(r.gOps) || 0;
  if (ec > 0) return 'C4';
  if (ed > 0) return 'C2a';
  if (eo > 0) return 'C2d';
  if (g > 0) return 'C2b';
  return 'C2c';
}
for (const r of full) r.cause = cause(r);

const CAUSE_LABEL = {
  C1: 'C1 红跑但失败原因没进上下文（要重看 ⇒ 再跑）',
  C2a: 'C2a 其间只改了文档，全量与被改动无因果',
  C2b: 'C2b 其间只有 git/建树动作（sha 前进那一族）',
  C2c: 'C2c 其间什么都没动（纯重跑）',
  C2d: 'C2d 其间只动了其它面（非被测代码、非文档）',
  C3: 'C3 并发假红（墙钟断言 / 并发膨胀）',
  C4: 'C4 改完复验（被测面代码动过）',
};
const stat = (arr) => {
  const s = arr.map((r) => r.seconds).filter((x) => !isNaN(x)).sort((a, b) => a - b);
  const sum = s.reduce((a, b) => a + b, 0);
  return {
    n: arr.length, red: arr.filter((r) => r.verdict === '红').length,
    nTime: s.length, sum: sum, mean: s.length ? sum / s.length : NaN,
    median: s.length ? s[Math.floor(s.length / 2)] : NaN,
    min: s.length ? s[0] : NaN, max: s.length ? s[s.length - 1] : NaN,
    chars: arr.reduce((a, r) => a + r.chars, 0),
    charsMax: arr.length ? Math.max(...arr.map((r) => r.chars)) : 0,
  };
};

const fmt = (x, d = 1) => (isNaN(x) ? '—' : Number(x).toFixed(d));

console.log('## 窗口与源');
const allTs = all.map((r) => r.ts).filter(Boolean).sort();
const sessTsv = readFileSync('D:/FL/.scratch/research/1545-sessions-v4.tsv', 'utf8').split('\n').filter(Boolean);
console.log(`- 会话转录：扫了 ${sessTsv.length - 1} 个会话文件（主 + 子代理），其中 ${new Set(all.map((r) => r.sid)).size} 个含 jest 调用；最早 tool_use ${allTs[0]}，最晚 ${allTs[allTs.length - 1]}`);
console.log(`- jest 相关调用共 ${all.length} 次；其中真出现汇总行（bucket ≠ 无汇总行）${all.filter((r) => r.bucket !== '无汇总行').length} 次`);
const kb = new Map();
for (const r of all) kb.set(r.bucket, (kb.get(r.bucket) || 0) + 1);
console.log('- 按规模档位：' + [...kb].sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k}=${v}`).join(' · '));
const kk = new Map();
for (const r of all) kk.set(r.kind, (kk.get(r.kind) || 0) + 1);
console.log('- 按命令形态：' + [...kk].sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k}=${v}`).join(' · '));
console.log(`- 全量档判据：汇总行 total ≥ 100 套件（实测 total 区间 ${Math.min(...full.map((r) => N(r.total)))}–${Math.max(...full.map((r) => N(r.total)))}）`);

const S = stat(full);
console.log('\n## 全量 ③ 总账');
console.log(`执行 ${S.n} 次（红 ${S.red} / 绿 ${S.n - S.red}）· 有 Time: 行 ${S.nTime} 次 · 秒数合计 ${fmt(S.sum)} s（${fmt(S.sum / 60)} min）· 均值 ${fmt(S.mean)} s · 中位 ${fmt(S.median)} s · 区间 ${fmt(S.min)}–${fmt(S.max)} s`);
console.log(`进上下文的结果字符合计 ${S.chars}（单次中位 ${full.map((r) => r.chars).sort((a, b) => a - b)[Math.floor(full.length / 2)]}，单次最大 ${S.charsMax}）`);
console.log(`与其它会话时间区间重叠的全量跑 ${full.filter((r) => r.overlap > 0).length} 次；命令带落盘重定向 ${full.filter((r) => r.redirect === 'Y').length} 次；由宿主后台 output 解析出汇总 ${full.filter((r) => r.src.startsWith('bg:')).length} 次`);
console.log(`会话内首次全量 ${full.filter((r) => r.firstInSess).length} 次`);

console.log('\n## 按原因分档');
console.log('| 原因 | 次数 | 其中红 | 有 Time 的次数 | 秒数合计 | 均值 s | 中位 s | 结果字符合计 |');
console.log('|---|---|---|---|---|---|---|---|');
const perC = new Map();
for (const r of full) { if (!perC.has(r.cause)) perC.set(r.cause, []); perC.get(r.cause).push(r); }
for (const [k, arr] of [...perC].sort((a, b) => b[1].length - a[1].length)) {
  const s = stat(arr);
  console.log(`| ${CAUSE_LABEL[k]} | ${s.n} | ${s.red} | ${s.nTime} | ${fmt(s.sum)} | ${fmt(s.mean)} | ${fmt(s.median)} | ${s.chars} |`);
}
const t = stat(full.filter((r) => r.cause !== 'C4'));
console.log(`| **非 C4 小计（＝可质疑的那部分）** | ${t.n} | ${t.red} | ${t.nTime} | ${fmt(t.sum)} | ${fmt(t.mean)} | ${fmt(t.median)} | ${t.chars} |`);

console.log('\n## 逐票（按全量次数降序）');
console.log('| 票 | 全量次数 | 红 | C4 | C2a | C2b | C2c | C2d | C1 | C3 | 秒数合计 | 有Time次数 | 结果字符 | 会话数 | 归属比 top1/top2 | 首 | 末 |');
console.log('|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|');
const perT = new Map();
for (const r of full) {
  const k = r.ticket || '(未归属)';
  if (!perT.has(k)) perT.set(k, { arr: [], sids: new Set(), ratios: new Set() });
  const o = perT.get(k); o.arr.push(r); o.sids.add(r.sid);
  const w = (r.ticketWhy || '').match(/^([\d:,]+) firstUser含=([YN]) ratio=(\S+)$/);
  if (w) o.ratios.add(w[3] + (w[2] === 'Y' ? '*' : ''));
}
for (const [k, o] of [...perT].sort((a, b) => b[1].arr.length - a[1].arr.length)) {
  const s = stat(o.arr);
  const c = (x) => o.arr.filter((r) => r.cause === x).length;
  const ts = o.arr.map((r) => r.ts).sort();
  console.log(`| ${k} | ${s.n} | ${s.red} | ${c('C4')} | ${c('C2a')} | ${c('C2b')} | ${c('C2c')} | ${c('C2d')} | ${c('C1')} | ${c('C3')} | ${fmt(s.sum)} | ${s.nTime} | ${s.chars} | ${o.sids.size} | ${[...o.ratios].join('/')} | ${ts[0].slice(5, 16)} | ${ts[ts.length - 1].slice(5, 16)} |`);
}

console.log('\n## 逐次台账（全量 ③，按时间）');
console.log('| # | ts (UTC) | 票 | 红绿 | 套件 total | Time s | 结果字符 | eCode | eDoc | eOther | gOps | 重叠会话数 | 落盘 | 源 | 原因 | 失败套件 |');
console.log('|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|');
full.forEach((r, i) => {
  console.log(`| ${i + 1} | ${r.ts.slice(5, 19).replace('T', ' ')} | ${r.ticket || '-'} | ${r.verdict} | ${r.fSuites}f/${r.total} | ${isNaN(r.seconds) ? '—' : r.seconds} | ${r.chars} | ${r.eCode} | ${r.eDoc} | ${r.eOther} | ${r.gOps} | ${r.overlap} | ${r.redirect} | ${r.src.startsWith('bg:') ? 'bg' : 'inline'} | ${r.cause} | ${(r.fails || '').replace(/\|/g, '/') .slice(0, 60) || '—'} |`);
});
