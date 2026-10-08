// #1545 · 从 Qoder 会话 JSONL 里抽「全量单测门（③）」的每一次执行
// 判据源：宿主会话转录 C:/Users/ZHENG/.qoder/projects/d--FL/*.jsonl（每行一条 JSON，带 timestamp / gitBranch / cwd / sessionId / isSidechain）
// 只读；输出两份：TSV 明细 + 汇总。
import { readdirSync, statSync, createWriteStream } from 'node:fs';
import { createReadStream } from 'node:fs';
import { join } from 'node:path';
import { createInterface } from 'node:readline';

const ROOT = 'C:/Users/ZHENG/.qoder/projects/d--FL';

// 命中「跑了一次 jest」的命令形态
const RE_JEST = /(npm (?:run )?test(?::unit)?(?::local)?\b)|(npx jest\b)|(jest --config jest\.config\.unit\.js)|(--config jest\.config\.unit\.js)/;
// 全量 vs 收窄 vs 单套件
const RE_PATTERN = /--testPathPattern/;
const RE_SINGLE = /(utils\/[A-Za-z0-9_.\-]+\.test\.[jt]sx?)|(\.test\.[jt]sx?\b)/;
const RE_LISTTESTS = /--listTests/;
const RE_DEVFINISH = /dev-finish\.ps1|dev:finish/;

function classify(cmd) {
  if (RE_LISTTESTS.test(cmd)) return 'listTests';           // 只列套件，不跑用例（建工作树闸门）
  if (RE_DEVFINISH.test(cmd)) return 'dev-finish(收窄③)';    // 步骤 3 用 --testPathPattern 的收窄面
  if (RE_PATTERN.test(cmd)) return '收窄③(testPathPattern)';
  if (/:local\b|test:unit:local/.test(cmd)) return '全量③(并行)';
  if (RE_SINGLE.test(cmd) && !/npm (?:run )?test/.test(cmd)) return '单套件③';
  if (RE_JEST.test(cmd)) return '全量③(串行)';
  return 'other';
}

function summaryOf(text) {
  const pick = (re) => { const m = String(text).match(re); return m ? m[0].replace(/\s+/g, ' ').trim() : ''; };
  return {
    suites: pick(/^\s*Test Suites:.*$/m),
    tests: pick(/^\s*Tests:.*$/m),
    time: pick(/^\s*Time:.*$/m),
  };
}

const files = readdirSync(ROOT).filter((f) => f.endsWith('.jsonl')).map((f) => join(ROOT, f));
const subFiles = [];
for (const f of files) {
  const dir = f.replace(/\.jsonl$/, '');
  try {
    const sub = join(dir, 'subagents');
    for (const g of readdirSync(sub)) if (g.endsWith('.jsonl')) subFiles.push(join(sub, g));
  } catch { /* 无 subagents 目录 */ }
}

const out = createWriteStream('D:/FL/.scratch/research/1545-jsonl-jest-runs.tsv');
out.write(['sessionId', 'sidechain', 'timestamp', 'gitBranch', 'cwd', 'kind', 'resultChars', 'suitesLine', 'testsLine', 'timeLine', 'cmdHead'].join('\t') + '\n');

let scanned = 0, hits = 0;
const perKind = new Map();
const perBranch = new Map();

async function scanFile(path, sidechainLabel) {
  const rl = createInterface({ input: createReadStream(path, { encoding: 'utf8' }), crlfDelay: Infinity });
  const pending = new Map(); // tool_use_id -> {ts, branch, cwd, sid, kind, cmd}
  for await (const line of rl) {
    scanned++;
    if (!line.includes('tool_use') && !line.includes('tool_result')) continue;
    let o; try { o = JSON.parse(line); } catch { continue; }
    const content = o?.message?.content;
    if (!Array.isArray(content)) continue;
    for (const c of content) {
      if (c?.type === 'tool_use' && (c.name === 'Bash' || c.name === 'Shell')) {
        const cmd = String(c.input?.command || '');
        if (!RE_JEST.test(cmd) && !RE_DEVFINISH.test(cmd)) continue;
        const kind = classify(cmd);
        pending.set(c.id, {
          ts: o.timestamp || '', branch: o.gitBranch || '', cwd: o.cwd || '',
          sid: o.sessionId || '', kind, cmd,
        });
      } else if (c?.type === 'tool_result') {
        const p = pending.get(c.tool_use_id);
        if (!p) continue;
        pending.delete(c.tool_use_id);
        let text = '';
        if (typeof c.content === 'string') text = c.content;
        else if (Array.isArray(c.content)) text = c.content.map((x) => (typeof x === 'string' ? x : x?.text || '')).join('\n');
        const s = summaryOf(text);
        const row = [p.sid, sidechainLabel, p.ts, p.branch, p.cwd, p.kind, String(text.length),
          s.suites, s.tests, s.time, p.cmd.replace(/[\t\r\n]+/g, ' ').slice(0, 160)];
        out.write(row.join('\t') + '\n');
        hits++;
        perKind.set(p.kind, (perKind.get(p.kind) || 0) + 1);
        const b = p.branch || '(unknown)';
        if (!perBranch.has(b)) perBranch.set(b, new Map());
        const m = perBranch.get(b);
        m.set(p.kind, (m.get(p.kind) || 0) + 1);
      }
    }
  }
}

for (const f of files) await scanFile(f, 'main');
for (const f of subFiles) await scanFile(f, 'subagent');
out.end();

console.log(`scanned lines=${scanned} files=${files.length}+${subFiles.length}sub hits=${hits}`);
console.log('--- per kind ---');
for (const [k, v] of [...perKind].sort((a, b) => b[1] - a[1])) console.log(`${v}\t${k}`);
console.log('--- per branch (只列命中≥1 的) ---');
for (const [b, m] of [...perBranch].sort()) {
  const parts = [...m].map(([k, v]) => `${k}=${v}`).join(' ');
  console.log(`${b}\t${parts}`);
}
