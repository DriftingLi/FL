// #1545 v2 · 会话转录 → 全量 ③ 执行台账（含归因判据所需的上下文列）
// 源：C:/Users/ZHENG/.qoder/projects/d--FL/*.jsonl（主会话）+ */subagents/*.jsonl（子代理）
//     + C:/Users/ZHENG/AppData/Local/Temp/qoder-cli/D--FL/<sid>/tasks/<taskId>.output（后台跑的落盘输出）
// 只读；输出 TSV 到 D:/FL/.scratch/research/1545-runs-v2.tsv
import { readdirSync, createWriteStream, readFileSync, existsSync, statSync } from 'node:fs';
import { createReadStream } from 'node:fs';
import { join, basename } from 'node:path';
import { createInterface } from 'node:readline';

const PROJ = 'C:/Users/ZHENG/.qoder/projects/d--FL';
const TASKS = 'C:/Users/ZHENG/AppData/Local/Temp/qoder-cli/D--FL';

const RE_JESTCMD = /(npm (?:run )?test(?::unit)?(?::local)?\b)|(npx jest\b)|(--config jest\.config\.unit\.js)|(dev-finish\.ps1)|(dev:finish)/;
const RE_LISTTESTS = /--listTests/;
const RE_PATTERN = /--testPathPattern/;
const RE_DEVFINISH = /dev-finish\.ps1|dev:finish/;
const RE_REDIRECT = /(>|>>|\*\>|Tee-Object|Out-File)\s*[^ ]*\.(log|txt)/;

const num = (s, re) => { const m = String(s || '').match(re); return m ? +m[1] : NaN; };
function parseSummary(text) {
  const t = String(text || '');
  const suites = (t.match(/^\s*Test Suites:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  const tests = (t.match(/^\s*Tests:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  const time = (t.match(/^\s*Time:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  if (!suites) return null;
  const fails = [...t.matchAll(/^\s*FAIL\s+(\S+)/gm)].map((m) => m[1]).slice(0, 8).join(' ');
  return {
    suites, tests, time, fails,
    total: num(suites, /(\d+)\s+total/),
    fSuites: num(suites, /(\d+)\s+failed/) || 0,
    fTests: num(tests, /(\d+)\s+failed/) || 0,
    tTotal: num(tests, /(\d+)\s+total/),
    seconds: num(time, /Time:\s*([\d.]+)\s*s/),
    chars: t.length,
  };
}

// ---------- 会话 → 票号 ----------
const sessMeta = new Map(); // sid -> {tickets:Map, cwds:Set, branches:Set, firstUser:string, firstTs:string}
function meta(sid) {
  if (!sessMeta.has(sid)) sessMeta.set(sid, { tickets: new Map(), cwds: new Set(), branches: new Set(), firstUser: '', firstTs: '', lastTs: '' });
  return sessMeta.get(sid);
}
function addTicket(m, n, w) {
  if (!/^\d{3,4}$/.test(String(n))) return;
  const v = Number(n); if (v < 600 || v > 1600) return;   // 本仓票号区间（现读：#638 epic … #1547）
  m.tickets.set(v, (m.tickets.get(v) || 0) + w);
}

const mainFiles = readdirSync(PROJ).filter((f) => f.endsWith('.jsonl')).map((f) => join(PROJ, f));
const pairs = []; // {path, sid, side}
for (const f of mainFiles) {
  pairs.push({ path: f, sid: basename(f, '.jsonl'), side: 'main' });
  const sub = join(f.replace(/\.jsonl$/, ''), 'subagents');
  if (existsSync(sub)) for (const g of readdirSync(sub)) if (g.endsWith('.jsonl')) pairs.push({ path: join(sub, g), sid: basename(f, '.jsonl'), side: 'sub' });
}

const runs = [];       // 每次 jest 调用（含无汇总行的）
const taskIndex = new Map(); // taskId -> {path, mtime}
for (const sid of readdirSync(TASKS)) {
  const d = join(TASKS, sid, 'tasks');
  if (!existsSync(d)) continue;
  for (const g of readdirSync(d)) if (g.endsWith('.output')) {
    const p = join(d, g);
    taskIndex.set(g.replace(/\.output$/, ''), { path: p, sid, mtime: statSync(p).mtime.toISOString() });
  }
}

for (const { path, sid, side } of pairs) {
  const m = meta(sid);
  const rl = createInterface({ input: createReadStream(path, { encoding: 'utf8' }), crlfDelay: Infinity });
  const pending = new Map();
  let editsSinceLastFull = 0;
  for await (const line of rl) {
    let o; try { o = JSON.parse(line); } catch { continue; }
    const ts = o.timestamp || '';
    if (ts) { if (!m.firstTs) m.firstTs = ts; m.lastTs = ts; }
    if (o.cwd) m.cwds.add(o.cwd);
    if (o.gitBranch) m.branches.add(o.gitBranch);
    const content = o?.message?.content;
    // 票号候选：正文里的 issue 链接 / #NNNN / wt-NNNN / -Task NNNN
    const raw = typeof content === 'string' ? content : JSON.stringify(content || '');
    if (o.type === 'user' && !m.firstUser && typeof content !== 'string') {
      const t = (Array.isArray(content) ? content.map((x) => x?.text || '').join(' ') : '');
      if (t.trim()) m.firstUser = t.replace(/\s+/g, ' ').slice(0, 300);
    }
    for (const g of raw.matchAll(/issues\/(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/(?:^|[^0-9])#(\d{3,4})(?:[^0-9]|$)/g)) addTicket(m, g[1], 2);
    for (const g of raw.matchAll(/wt-(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/-Task\s+(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/(?:feat|fix|docs|refactor|chore)\/(\d{3,4})/g)) addTicket(m, g[1], 3);

    if (!Array.isArray(content)) continue;
    for (const c of content) {
      if (c?.type === 'tool_use') {
        const nm = c.name;
        if (nm === 'Edit' || nm === 'Write' || nm === 'NotebookEdit') editsSinceLastFull++;
        if (nm !== 'Bash' && nm !== 'Shell') continue;
        const cmd = String(c.input?.command || '');
        if (!RE_JESTCMD.test(cmd)) continue;
        pending.set(c.id, { ts, cmd, side, editsBefore: editsSinceLastFull });
      } else if (c?.type === 'tool_result') {
        const p = pending.get(c.tool_use_id); if (!p) continue;
        pending.delete(c.tool_use_id);
        let text = typeof c.content === 'string' ? c.content
          : Array.isArray(c.content) ? c.content.map((x) => (typeof x === 'string' ? x : x?.text || '')).join('\n') : '';
        let sum = parseSummary(text);
        let src = 'inline';
        // 后台跑：结果里没有汇总行，但给了 taskId ⇒ 去 tasks/<id>.output 里取
        const bg = text.match(/background with ID:\s*(\w+)/);
        if (!sum && bg) {
          const ti = taskIndex.get(bg[1]);
          if (ti) { const ft = readFileSync(ti.path, 'utf8'); sum = parseSummary(ft); src = 'bg:' + bg[1]; if (sum) sum.mtime = ti.mtime; }
        }
        const kind = RE_LISTTESTS.test(p.cmd) ? 'listTests'
          : RE_DEVFINISH.test(p.cmd) ? 'dev-finish'
            : RE_PATTERN.test(p.cmd) ? 'narrowed' : 'jest';
        runs.push({
          sid, side: p.side, ts: p.ts, kind, cmd: p.cmd.replace(/[\t\r\n]+/g, ' ').slice(0, 200),
          redirect: RE_REDIRECT.test(p.cmd) ? 'Y' : 'N', editsBefore: p.editsBefore,
          src, sum,
        });
        if (sum && sum.total >= 100) editsSinceLastFull = 0;
      }
    }
  }
}

// 会话票号裁定：权重最高者；并列记「歧义」
function ticketOf(sid) {
  const m = meta(sid);
  const arr = [...m.tickets].sort((a, b) => b[1] - a[1] || a[0] - b[0]);
  if (!arr.length) return { t: '', why: '无票号线索' };
  if (arr.length > 1 && arr[0][1] === arr[1][1]) return { t: String(arr[0][0]), why: `歧义(${arr.slice(0, 3).map((x) => x[0] + ':' + x[1]).join(',')})` };
  return { t: String(arr[0][0]), why: `权重${arr[0][1]}(${arr.slice(0, 3).map((x) => x[0] + ':' + x[1]).join(',')})` };
}

const out = createWriteStream('D:/FL/.scratch/research/1545-runs-v2.tsv');
out.write(['sid', 'side', 'ticket', 'ticketWhy', 'ts', 'kind', 'bucket', 'verdict', 'total', 'fSuites', 'tTotal', 'fTests', 'seconds', 'resultChars', 'redirect', 'editsBefore', 'src', 'bgMtime', 'fails', 'branch', 'cwd', 'cmd'].join('\t') + '\n');
for (const r of runs) {
  const { t, why } = ticketOf(r.sid);
  const m = meta(r.sid);
  const s = r.sum;
  const bucket = !s ? '无汇总行' : s.total >= 100 ? '全量③' : s.total >= 10 ? '收窄③' : '单套件③';
  out.write([r.sid, r.side, t, why, r.ts, r.kind, bucket,
    s ? ((s.fSuites > 0 || s.fTests > 0) ? '红' : '绿') : '',
    s ? s.total : '', s ? s.fSuites : '', s ? s.tTotal : '', s ? s.fTests : '',
    s && !isNaN(s.seconds) ? s.seconds : '', s ? s.chars : '', r.redirect, r.editsBefore, r.src,
    (s && s.mtime) || '', s ? s.fails : '', [...m.branches].join('|'), [...m.cwds].join('|'), r.cmd].join('\t') + '\n');
}
out.end();

const full = runs.filter((r) => r.sum && r.sum.total >= 100);
console.log(`会话数=${sessMeta.size}  jest 调用=${runs.length}  有汇总行=${runs.filter((r) => r.sum).length}  全量=${full.length}`);
console.log(`后台任务 output 文件=${taskIndex.size}  由 bg 解析出汇总的调用=${runs.filter((r) => r.src.startsWith('bg:') && r.sum).length}`);
const perT = new Map();
for (const r of full) {
  const { t } = ticketOf(r.sid); const k = t || '(未归属)';
  if (!perT.has(k)) perT.set(k, { n: 0, red: 0, sec: 0, nSec: 0, chars: 0, sids: new Set(), first: r.ts, last: r.ts });
  const o = perT.get(k); o.n++; o.sids.add(r.sid);
  const s = r.sum;
  if (s.fSuites > 0 || s.fTests > 0) o.red++;
  if (!isNaN(s.seconds)) { o.sec += s.seconds; o.nSec++; }
  o.chars += s.chars;
  if (r.ts < o.first) o.first = r.ts; if (r.ts > o.last) o.last = r.ts;
}
console.log('\nticket\tfullRuns\tred\tsumSeconds\trunsWithTime\tsumResultChars\tsessions\tfirst\tlast');
for (const [k, o] of [...perT].sort((a, b) => b[1].n - a[1].n)) {
  console.log([k, o.n, o.red, o.sec.toFixed(1), o.nSec, o.chars, o.sids.size, o.first, o.last].join('\t'));
}
console.log('\n--- 会话→票号裁定（只列有全量跑的会话）---');
const sids = [...new Set(full.map((r) => r.sid))];
for (const s of sids) {
  const m = meta(s); const { t, why } = ticketOf(s);
  console.log(`${s}\t票=${t || '-'}\t${why}\tfirstUser=${(m.firstUser || '').slice(0, 110)}`);
}
