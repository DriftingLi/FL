#!/usr/bin/env node
/**
 * #1770 本地预检：对「将要推的那个 commit」跑与 CI 完全同一份判据。
 *
 * 为什么要有它：证据主键改成 commit sha 之后，证据在开 PR 之前就能贴；但这只解决「贴哪」，
 * 不解决「贴得对不对」——首推即绿的前提是本机能先拿到与 CI 相同的结论。判据不抄第二份：
 * 直接 import .github/scripts/lib/gate-validator.mjs（pr-evidence.yml / mobile-gates.yml 也是
 * 用 repos.getContent 取同一个文件执行，见 #1770 S5）。
 *
 * 用法：
 *   node .github/scripts/preflight-pr.mjs --body-file .scratch/pr-body.md
 *   node .github/scripts/preflight-pr.mjs --pr 1234      # PR 已存在时把它的评论也算进来
 *   node .github/scripts/preflight-pr.mjs --json          # 给 submit-pr.ps1 消费
 *
 * 退出码：0 = 绿（可以开 PR）；1 = 红（附缺哪几门）；2 = 用法 / 环境错误。
 * 只读：只有 gh api 的 GET 与 git 的读操作，不写任何远端状态。
 */
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { validatePrEvidence, issueNumbersInBody, GATE_EVIDENCE } from './lib/gate-validator.mjs';

const USAGE = [
  '用法: node .github/scripts/preflight-pr.mjs [选项]',
  '',
  '  --body-file <path>  PR 正文文件（未给且未给 --pr 时按空正文判）',
  '  --pr <number>       PR 已存在时把 PR 评论并入 gate 评论候选',
  '  --base <ref>        比较基线（默认 origin/master，本地 git log 用它，API 自动剥 origin/）',
  '  --commit <sha>      被检 commit（默认 HEAD）',
  '  --fill-body         把 ci-summary 的 run 链接补进正文 ③ 行并写回 --body-file（不开 PR，只补链接）',
  '  --json              输出 JSON（供 submit-pr.ps1 消费）',
  '  -h, --help          本帮助',
].join('\n');

export function parseArgs(argv) {
  const out = { base: 'origin/master', commit: '', bodyFile: '', pr: 0, json: false, help: false, fillBody: false };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '-h' || arg === '--help') out.help = true;
    else if (arg === '--json') out.json = true;
    else if (arg === '--fill-body') out.fillBody = true;
    else if (arg === '--base') out.base = need(argv, ++i, arg);
    else if (arg === '--commit') out.commit = need(argv, ++i, arg);
    else if (arg === '--body-file') out.bodyFile = need(argv, ++i, arg);
    else if (arg === '--pr') out.pr = Number(need(argv, ++i, arg));
    else throw new UsageError(`未知参数：${arg}`);
  }
  if (!Number.isInteger(out.pr)) throw new UsageError('--pr 只能是整数');
  if (out.fillBody && !out.bodyFile) throw new UsageError('--fill-body 需要 --body-file');
  return out;
}

class UsageError extends Error {}

function need(argv, i, flag) {
  const v = argv[i];
  if (v === undefined || v.startsWith("--")) throw new UsageError(`${flag} 缺参数`);
  return v;
}

function run(cmd, args) {
  return execFileSync(cmd, args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
    maxBuffer: 64 * 1024 * 1024,
  });
}

const git = (...args) => run("git", args);
const gh = (...args) => run("gh", args);

function ghJson(args) {
  const text = gh(...args);
  return text && text.trim() ? JSON.parse(text) : null;
}

function repoSlug() {
  const view = ghJson(['repo', 'view', '--json', 'nameWithOwner']);
  if (!view || !view.nameWithOwner) throw new Error('拿不到仓库名（gh repo view 失败）');
  return view.nameWithOwner;
}

/** compare API → 校验器要的 files（带 patch，② 门的条件编译指令判据靠它）。 */
export function toValidatorFiles(payload) {
  const raw = payload && Array.isArray(payload.files) ? payload.files : [];
  const files = raw.map((f) => ({ filename: f.filename, patch: f.patch }));
  // compare API 最多给 300 个文件；到顶就按截断处理，让校验器 fail-closed 而不是假装看全了。
  return { files, truncated: raw.length >= 300 };
}

function compareOf(repo, base, head) {
  const payload = ghJson(['api', '-H', 'Accept: application/vnd.github+json', `repos/${repo}/compare/${base}...${head}`]);
  const { files, truncated } = toValidatorFiles(payload);
  return { status: (payload && payload.status) || '', mergeBaseSha: (payload && payload.merge_base_commit && payload.merge_base_commit.sha) || '', files, truncated };
}

function commitRangeShas(base, commit) {
  const text = git('log', '--format=%H', `${base}..${commit}`);
  return text.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
}

function commitCommentBodies(repo, sha) {
  const arr = ghJson(['api', `repos/${repo}/commits/${sha}/comments?per_page=100`]) || [];
  return arr.map((c) => (c && c.body) || '');
}

function prCommentBodies(repo, pr) {
  const arr = ghJson(['api', `repos/${repo}/issues/${pr}/comments?per_page=100`]) || [];
  return arr.map((c) => (c && c.body) || '');
}

/** 票（issue）评论正文——#1841 起与 PR 评论同进候选（commit 评论端点恒 404 时的降级落点）。 */
function issueCommentBodies(repo, n) {
  const arr = ghJson(['api', `repos/${repo}/issues/${n}/comments?per_page=100`]) || [];
  return arr.map((c) => (c && c.body) || '');
}

function prBody(pr) {
  const view = ghJson(['pr', 'view', String(pr), '--json', 'body']);
  return (view && view.body) || '';
}

function currentLogin() {
  try {
    const me = ghJson(['api', 'user']);
    return (me && me.login) || '';
  } catch {
    return '';
  }
}

/**
 * ③ 门的 run 链接自动发现（#1770 S4）：这条线上 commit 的 ci-summary 就是两腿全绿的运行记录，
 * 以前要人手去 Actions 页面复制链接——手抄的链接最容易抄成别的 run（看不出错、但证据对不上）。
 * 纯函数，便于单向测（runs 由调用方取）。
 */
export function pickCiSummaryRun(runs, headSha) {
  const list = (Array.isArray(runs) ? runs : []).filter((r) => r && r.name === 'ci-summary');
  if (list.length === 0) return null;
  const forSha = headSha ? list.filter((r) => r.head_sha === headSha) : [];
  const pool = forSha.length > 0 ? forSha : list;
  const green = pool.filter((r) => r.conclusion === 'success');
  const pick = (green.length > 0 ? green : pool).slice().sort((a, b) => String(b.started_at || '').localeCompare(String(a.started_at || '')))[0];
  return { conclusion: pick.conclusion || pick.status || '', url: pick.details_url || pick.html_url || '', sha: pick.head_sha || '', name: pick.name };
}

function checkRuns(repo, sha) {
  const payload = ghJson(['api', `repos/${repo}/commits/${sha}/check-runs?per_page=100`]);
  return (payload && payload.check_runs) || [];
}

// 判据形状同样取自描述符（#1920）：CI run 链接、占位词表、行前缀、结论字段、门的顺序都不在这里另写一份。
const re = (spec) => new RegExp(spec.source, spec.flags || '');
const CI_RUN_LINK = re(GATE_EVIDENCE.artifacts['ci-run']);
// 与校验器的 PLACEHOLDER 同词表：这些值代表「还没填」，往里补链接会把它从「如实判红」变成「假绿」。
const PLACEHOLDER_VALUE = re(GATE_EVIDENCE.placeholder);
const LIST_ITEM_PREFIX = new RegExp(GATE_EVIDENCE.listItem.prefixStrip);
const CONCLUSION_FIELD = new RegExp(
  `(${GATE_EVIDENCE.conclusionField}(?:[（(][^）)]*[）)])?\\s*[：:]\\s*)(.*)$`,
);
const GATE_KEYS = GATE_EVIDENCE.gates.map((g) => g.key);

/**
 * 把 ci-summary 的 run 链接补进正文 ③ 行（#1770 S4）——只补链接，不改结论文字。
 * 三种情况拒绝补（返回 filled:false + reason，让调用方打印人话而不是造出假证据）：
 *   - ③ 行已经有 run 链接（already-has-run，无事可做）
 *   - ③ 行没有「结论：」字段（no-conclusion-field，结构不对，该按模板重写）
 *   - 结论是占位/空（conclusion-placeholder，链接补上去会让占位词判据失效 ⇒ 必须先写字）
 * 纯函数（body + url 进，body 出），便于单向测。
 */
export function fillCiLine(body, url) {
  const text = String(body || '');
  const link = String(url || '');
  if (!CI_RUN_LINK.test(link)) return { body: text, filled: false, reason: 'url-invalid' };
  const lines = text.split(/\r?\n/);
  const idx = lines.findIndex((l) => l.replace(LIST_ITEM_PREFIX, '').startsWith('③'));
  if (idx === -1) return { body: text, filled: false, reason: 'no-gate-3-line' };
  if (CI_RUN_LINK.test(lines[idx])) return { body: text, filled: false, reason: 'already-has-run' };
  const m = lines[idx].match(CONCLUSION_FIELD);
  if (!m) return { body: text, filled: false, reason: 'no-conclusion-field' };
  const value = m[2].trim().replace(/[（(]\s*[）)]\s*$/, '').trim();
  if (PLACEHOLDER_VALUE.test(value)) return { body: text, filled: false, reason: 'conclusion-placeholder' };
  lines[idx] = `${lines[idx].trimEnd()}（${link}）`;
  return { body: lines.join('\n'), filled: true, reason: 'filled', line: lines[idx] };
}

function render(ctx) {
  const { args, commit, author, compare, gateComments, commitComments, issueComments, shas, ciRun, result } = ctx;
  const lines = [];
  lines.push('#1770 预检（与 CI 同一份判据：.github/scripts/lib/gate-validator.mjs）');
  lines.push(`  commit   ${commit.slice(0, 12)}`);
  lines.push(`  base     ${args.base}（compare ${compare.status}，${compare.files.length} 个文件${compare.truncated ? '，已截断' : ''}）`);
  lines.push(`  author   ${author ? "@" + author : "（未知）"}`);
  lines.push(`  证据     正文 ${ctx.hasBody ? '有' : '无'} · PR 评论 ${gateComments.length} 条 · commit 评论 ${commitComments.length} 条（${shas.length} 个 commit） · 票评论 ${issueComments.length} 条（正文关联的票）`);
  if (ciRun) lines.push(`  ci-summary  ${ciRun.conclusion || '?'} ${ciRun.url || '（无链接）'}`);
  lines.push("");
  for (const key of GATE_KEYS) {
    const gate = result.gates && result.gates[key];
    if (!gate) continue;
    const state = gate.required === false ? '— 免' : gate.bound ? '✅ 已绑' : '❌ 未绑';
    lines.push(`  ${key} ${state}`);
  }
  if (result.errors.length > 0) {
    lines.push("");
    for (const e of result.errors) lines.push(`  ❌ ${e}`);
  }
  if (result.notes.length > 0) {
    lines.push("");
    for (const n of result.notes) lines.push(`  · ${n}`);
  }
  lines.push("");
  lines.push(result.ok ? "结论：绿 —— 可以开 PR（提交前再确认 HEAD 已推到 origin）" : "结论：红 —— 先补齐上面的缺项，再重跑预检（绿了才开 PR）");
  // S4：③ 还没绑上且已经找到 ci-summary ⇒ 直接把可粘贴的行打出来，省掉手抄 run 链接这一步。
  const g3 = result.gates && result.gates['③'];
  if (g3 && g3.required !== false && !g3.bound) {
    lines.push("");
    if (ciRun && ciRun.conclusion === 'success' && ciRun.url) {
      lines.push(`  ③ 行可直接引用（ci-summary=success）：`);
      lines.push(`    ③ 结论：两腿全绿（${ciRun.url}）`);
    } else if (ciRun) {
      lines.push(`  ⚠ ci-summary 当前是 ${ciRun.conclusion || '未完成'}，此时贴 ③ 行会构成虚假证据——等它绿。`);
    } else {
      lines.push("  ⚠ 这个 commit 上还没有 ci-summary check run（先把分支推到 origin，等 CI 跑完再预检）。");
    }
  }
  return lines.join("\n");
}

async function main(argv) {
  const args = parseArgs(argv);
  if (args.help) {
    console.log(USAGE);
    return 0;
  }
  const repo = repoSlug();
  const commit = (args.commit || git('rev-parse', 'HEAD')).trim();
  if (args.fillBody) {
    // 只补链接，不开 PR：submit-pr.ps1 先调这一支把 ③ 行补齐，再跑完整预检。
    const ciRun = pickCiSummaryRun(checkRuns(repo, commit), commit);
    const body = readFileSync(args.bodyFile, 'utf8');
    const out = ciRun && ciRun.conclusion === 'success' ? fillCiLine(body, ciRun.url) : { body, filled: false, reason: ciRun ? 'ci-not-green' : 'no-ci-run' };
    if (out.filled) writeFileSync(args.bodyFile, out.body, 'utf8');
    console.log(JSON.stringify({ filled: out.filled, reason: out.reason, line: out.line || '', ciRun }, null, 2));
    return out.filled || out.reason === 'already-has-run' ? 0 : 1;
  }
  const baseApi = args.base.replace(/^origin\//, '');
  const compare = compareOf(repo, baseApi, commit);
  const hasBody = Boolean(args.bodyFile || args.pr);
  const body = args.bodyFile ? readFileSync(args.bodyFile, 'utf8') : args.pr ? prBody(args.pr) : '';
  const shas = [...new Set([...commitRangeShas(args.base, commit), commit])].slice(-30);
  const commitComments = shas.flatMap((sha) => commitCommentBodies(repo, sha));
  const gateComments = args.pr ? prCommentBodies(repo, args.pr) : [];
  // #1841：票评论也进候选（本仓 commit 评论端点恒 404 ⇒ 门脚本降级把同一份证据贴到票上）。
  // 「哪些票」由校验器里同一个函数定（正文显式关联，≤5 张）——本地预检与 CI 不许两套口径。
  const issueComments = issueNumbersInBody(body).flatMap((n) => issueCommentBodies(repo, n));
  const author = currentLogin();
  const compareCache = new Map();
  const fetchCompare = (from, to) => {
    const key = `${from}...${to}`;
    if (compareCache.has(key)) return compareCache.get(key);
    let value = null;
    try {
      const c = compareOf(repo, from.replace(/^origin\//, ''), to);
      value = { status: c.status, mergeBaseSha: c.mergeBaseSha, files: c.files.map((f) => f.filename), filesTruncated: c.truncated };
    } catch (e) {
      // 与 CI 同口径：compare 拿不到就收敛成 null，让校验器 fail-closed（不当成「没改」）。
      process.stderr.write(`compare ${key} 失败，按 fail-closed 处理：${e.message}\n`);
      value = null;
    }
    compareCache.set(key, value);
    return value;
  };
  const result = await validatePrEvidence({ files: compare.files, body, author, gateComments, commitComments, issueComments, headSha: commit, fetchCompare });
  const ciRun = pickCiSummaryRun(checkRuns(repo, commit), commit);
  if (args.json) {
    console.log(JSON.stringify({ ok: result.ok, errors: result.errors, notes: result.notes, gates: result.gates, commit, base: args.base, author, ciRun }, null, 2));
  } else {
    console.log(render({ args, commit, author, compare, gateComments, commitComments, issueComments, shas, ciRun, result, hasBody }));
  }
  return result.ok ? 0 : 1;
}

const invokedDirectly = Boolean(process.argv[1]) && process.argv[1].replace(/\\/g, "/").endsWith(".github/scripts/preflight-pr.mjs");
if (invokedDirectly) {
  main(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (err) => {
      if (err instanceof UsageError) {
        process.stderr.write(`${err.message}\n\n${USAGE}\n`);
        process.exit(2);
      }
      process.stderr.write(`预检出错：${err && err.message ? err.message : err}\n`);
      process.exit(2);
    },
  );
}