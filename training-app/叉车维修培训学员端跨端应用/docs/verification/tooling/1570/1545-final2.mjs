// #1545 v4 · 全量 ③ 执行台账（细化「两次全量之间到底动了什么」）
// 相比 v2 多三列：editsCode / editsDoc / editsOther（按 Edit|Write 的 file_path 分面）、gitOps（两次之间改树的 git/建树命令数）
// 源：C:/Users/ZHENG/.qoder/projects/d--FL/*.jsonl + */subagents/*.jsonl + Temp/qoder-cli/D--FL/<sid>/tasks/*.output
import { readdirSync, createWriteStream, readFileSync, existsSync, statSync } from 'node:fs';
import { createReadStream } from 'node:fs';
import { join, basename } from 'node:path';
import { createInterface } from 'node:readline';

const PROJ = 'C:/Users/ZHENG/.qoder/projects/d--FL';
const TASKS = 'C:/Users/ZHENG/AppData/Local/Temp/qoder-cli/D--FL';
const OUT = 'D:/FL/.scratch/research/1545-runs-v4.tsv';

const RE_JESTCMD = /(npm (?:run )?test(?::unit)?(?::local)?\b)|(npx jest\b)|(--config jest\.config\.unit\.js)|(dev-finish\.ps1)|(dev:finish)/;
const RE_LISTTESTS = /--listTests/;
const RE_PATTERN = /--testPathPattern/;
const RE_DEVFINISH = /dev-finish\.ps1|dev:finish/;
const RE_REDIRECT = /(>|>>|\*\>|Tee-Object|Out-File)\s*[^ ]*\.(log|txt)/;
const RE_GITOP = /git (?:merge|rebase|pull|checkout|reset|stash|cherry-pick|worktree)|new-worktree\.ps1|wt-bootstrap\.ps1|npm (?:ci|install)\b/;
const RE_CODE = /(training-app[\\/])|(utils[\\/])|\.(uts|uvue|test\.js|js|mjs|ps1|go|ts|vue|json)$/i;
const RE_DOC = /\.md$/i;

const num = (s, re) => { const m = String(s || '').match(re); return m ? +m[1] : NaN; };
function parseSummary(t) {
  t = String(t || '');
  const suites = (t.match(/^\s*Test Suites:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  if (!suites) return null;
  const tests = (t.match(/^\s*Tests:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  const time = (t.match(/^\s*Time:.*$/m) || [''])[0].replace(/\s+/g, ' ').trim();
  const fails = [...t.matchAll(/^\s*FAIL\s+(\S+)/gm)].map((m) => m[1]).slice(0, 8).join(' ');
  return {
    suites, tests, time, fails,
    total: num(suites, /(\d+)\s+total/), fSuites: num(suites, /(\d+)\s+failed/) || 0,
    tTotal: num(tests, /(\d+)\s+total/), fTests: num(tests, /(\d+)\s+failed/) || 0,
    seconds: num(time, /Time:\s*([\d.]+)\s*s/), chars: t.length,
  };
}

const sessMeta = new Map();
const meta = (sid) => { if (!sessMeta.has(sid)) sessMeta.set(sid, { tickets: new Map(), firstUser: '', firstTs: '', lastTs: '', branches: new Set(), cwds: new Set() }); return sessMeta.get(sid); };
function addTicket(m, n, w) {
  if (!/^\d{3,4}$/.test(String(n))) return; const v = Number(n); if (v < 600 || v > 1600) return;
  m.tickets.set(v, (m.tickets.get(v) || 0) + w);
}

const taskIndex = new Map();
for (const sid of readdirSync(TASKS)) {
  const d = join(TASKS, sid, 'tasks'); if (!existsSync(d)) continue;
  for (const g of readdirSync(d)) if (g.endsWith('.output')) taskIndex.set(g.replace(/\.output$/, ''), { path: join(d, g), mtime: statSync(join(d, g)).mtime.toISOString() });
}

const pairs = [];
for (const f of readdirSync(PROJ).filter((x) => x.endsWith('.jsonl'))) {
  const p = join(PROJ, f), sid = basename(f, '.jsonl');
  pairs.push({ path: p, sid, side: 'main' });
  const sub = join(p.replace(/\.jsonl$/, ''), 'subagents');
  if (existsSync(sub)) for (const g of readdirSync(sub)) if (g.endsWith('.jsonl')) pairs.push({ path: join(sub, g), sid, side: 'sub' });
}

const runs = [];
for (const { path, sid, side } of pairs) {
  const m = meta(sid);
  const rl = createInterface({ input: createReadStream(path, { encoding: 'utf8' }), crlfDelay: Infinity });
  const pending = new Map();
  let eCode = 0, eDoc = 0, eOther = 0, gOps = 0;
  for await (const line of rl) {
    let o; try { o = JSON.parse(line); } catch { continue; }
    const ts = o.timestamp || '';
    if (ts) { if (!m.firstTs) m.firstTs = ts; m.lastTs = ts; }
    if (o.cwd) m.cwds.add(o.cwd);
    if (o.gitBranch) m.branches.add(o.gitBranch);
    const content = o?.message?.content;
    const raw = typeof content === 'string' ? content : JSON.stringify(content || '');
    if (o.type === 'user' && !m.firstUser && Array.isArray(content)) {
      const t = content.map((x) => x?.text || '').join(' '); if (t.trim()) m.firstUser = t.replace(/\s+/g, ' ').slice(0, 260);
    }
    for (const g of raw.matchAll(/issues\/(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/(?:^|[^0-9])#(\d{3,4})(?:[^0-9]|$)/g)) addTicket(m, g[1], 2);
    for (const g of raw.matchAll(/wt-(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/-Task\s+(\d{3,4})/g)) addTicket(m, g[1], 3);
    for (const g of raw.matchAll(/(?:feat|fix|docs|refactor|chore)\/(\d{3,4})/g)) addTicket(m, g[1], 3);
    if (!Array.isArray(content)) continue;
    for (const c of content) {
      if (c?.type === 'tool_use') {
        if (c.name === 'Edit' || c.name === 'Write' || c.name === 'NotebookEdit') {
          const fp = String(c.input?.file_path || '');
          if (RE_DOC.test(fp)) eDoc++; else if (RE_CODE.test(fp)) eCode++; else eOther++;
        }
        if (c.name !== 'Bash' && c.name !== 'Shell') continue;
        const cmd = String(c.input?.command || '');
        if (RE_GITOP.test(cmd)) gOps++;
        if (!RE_JESTCMD.test(cmd)) continue;
        pending.set(c.id, { ts, cmd, side, eCode, eDoc, eOther, gOps });
      } else if (c?.type === 'tool_result') {
        const p = pending.get(c.tool_use_id); if (!p) continue; pending.delete(c.tool_use_id);
        let text = typeof c.content === 'string' ? c.content
          : Array.isArray(c.content) ? c.content.map((x) => (typeof x === 'string' ? x : x?.text || '')).join('\n') : '';
        let sum = parseSummary(text), src = 'inline', bgMtime = '';
        const bg = text.match(/background with ID:\s*(\w+)/);
        if (!sum && bg) { const ti = taskIndex.get(bg[1]); if (ti) { sum = parseSummary(readFileSync(ti.path, 'utf8')); src = 'bg:' + bg[1]; if (sum) bgMtime = ti.mtime; } }
        const kind = RE_LISTTESTS.test(p.cmd) ? 'listTests' : RE_DEVFINISH.test(p.cmd) ? 'dev-finish' : RE_PATTERN.test(p.cmd) ? 'narrowed' : 'jest';
        runs.push({ sid, side: p.side, ts: p.ts, kind, redirect: RE_REDIRECT.test(p.cmd) ? 'Y' : 'N', eCode: p.eCode, eDoc: p.eDoc, eOther: p.eOther, gOps: p.gOps, src, bgMtime, sum, cmd: p.cmd.replace(/[\t\r\n]+/g, ' ').slice(0, 180) });
        if (sum && sum.total >= 100) { eCode = 0; eDoc = 0; eOther = 0; gOps = 0; }
      }
    }
  }
}

function ticketOf(sid) {
  const m = meta(sid); const arr = [...m.tickets].sort((a, b) => b[1] - a[1] || a[0] - b[0]);
  if (!arr.length) return { t: '', why: '无票号线索', ratio: '' };
  const ratio = arr.length > 1 && arr[1][1] ? (arr[0][1] / arr[1][1]).toFixed(1) : 'inf';
  const inFirst = m.firstUser.includes(String(arr[0][0]));
  return { t: String(arr[0][0]), why: `${arr.slice(0, 3).map((x) => x[0] + ':' + x[1]).join(',')} firstUser含=${inFirst ? 'Y' : 'N'} ratio=${ratio}`, ratio };
}

const out = createWriteStream(OUT);
out.write(['sid', 'side', 'ticket', 'ticketWhy', 'ts', 'kind', 'bucket', 'verdict', 'total', 'fSuites', 'tTotal', 'fTests', 'seconds', 'resultChars', 'redirect', 'eCode', 'eDoc', 'eOther', 'gOps', 'src', 'bgMtime', 'fails', 'branch', 'cwd', 'cmd'].join('\t') + '\n');
for (const r of runs) {
  const { t, why } = ticketOf(r.sid); const m = meta(r.sid); const s = r.sum;
  const bucket = !s ? '无汇总行' : s.total >= 100 ? '全量③' : s.total >= 10 ? '收窄③' : '单套件③';
  out.write([r.sid, r.side, t, why, r.ts, r.kind, bucket,
    s ? ((s.fSuites > 0 || s.fTests > 0) ? '红' : '绿') : '', s ? s.total : '', s ? s.fSuites : '', s ? s.tTotal : '', s ? s.fTests : '',
    s && !isNaN(s.seconds) ? s.seconds : '', s ? s.chars : '', r.redirect, r.eCode, r.eDoc, r.eOther, r.gOps, r.src, r.bgMtime,
    s ? s.fails : '', [...m.branches].join('|'), [...m.cwds].join('|'), r.cmd].join('\t') + '\n');
}
out.end();

// 会话表（含 firstUser，供归属可信度人工复核）
const so = createWriteStream('D:/FL/.scratch/research/1545-sessions-v4.tsv');
so.write(['sid', 'ticket', 'ticketWhy', 'firstTs', 'lastTs', 'branches', 'cwds', 'firstUser'].join('\t') + '\n');
for (const [sid, m] of sessMeta) {
  const { t, why } = ticketOf(sid);
  so.write([sid, t, why, m.firstTs, m.lastTs, [...m.branches].join('|'), [...m.cwds].join('|'), (m.firstUser || '').replace(/[\t\r\n]+/g, ' ')].join('\t') + '\n');
}
so.end();

const full = runs.filter((r) => r.sum && r.sum.total >= 100);
console.log(`会话=${sessMeta.size} jest调用=${runs.length} 全量=${full.length} 后台解析=${runs.filter((r) => r.src.startsWith('bg:') && r.sum).length}`);
console.log(`全量里 eCode=0 的=${full.filter((r) => r.eCode === 0).length}  其中 eDoc>0 的=${full.filter((r) => r.eCode === 0 && r.eDoc > 0).length}  其中全零(什么都没改)=${full.filter((r) => r.eCode === 0 && r.eDoc === 0 && r.eOther === 0 && r.gOps === 0).length}`);
console.log(`全量里 gOps>0 的=${full.filter((r) => r.gOps > 0).length}`);
