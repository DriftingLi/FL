#!/usr/bin/env node
/**
 * #1770 本地预检：对「将要推的那个 commit」跑与 CI 完全同一份判据。
 *
 * 为什么要有它：证据在开 PR 之前就能写（A1：写进正文一行 `跑于 <sha7>` + 门读数行），但那只解决
 * 「写哪里」，不解决「写对没有」——首开即绿的前提是本机能先拿到与 CI 相同的结论。判据不抄第二份：
 * 直接 import .github/scripts/lib/gate-validator.mjs（pr-evidence.yml 也是用 repos.getContent 取同一个文件执行）。
 *
 * **A1 形态（2026-10-29，#2100）**：不再取任何评论（PR / commit / 票三路全退役），
 *   也不再补 ③ 的 CI run 链接（③ 判据已判纯税）。所以本脚本的取数面只剩三样：
 *   改动文件清单（compare）、PR 正文、以及 ci-summary 的绿/红（给人看，不参与判据）。
 *
 * #2053：同层再对 PR 正文跑引用完整性 lint（.github/scripts/lib/lint-issue-references.mjs），
 * 红项并进 errors、共决 ok——不新增门、不动 gate-validator 的门清单与行措辞。
 *
 * 用法：
 *   node .github/scripts/preflight-pr.mjs --body-file .scratch/pr-body.md
 *   node .github/scripts/preflight-pr.mjs --pr 1234      # PR 已存在时读它的正文
 *   node .github/scripts/preflight-pr.mjs --json          # 给 submit-pr.ps1 消费
 *
 * 退出码：0 = 绿（可以开 PR）；1 = 红（附缺哪几门）；2 = 用法 / 环境错误。
 * 只读：只有 gh api 的 GET 与 git 的读操作，不写任何远端状态。
 */
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { validatePrEvidence, GATE_EVIDENCE } from './lib/gate-validator.mjs';
import { lintIssueReferences, makeFsFacts, makeGhNumberResolver } from './lib/lint-issue-references.mjs';
import { scanEvidenceLogs, classifyForPreflight } from './lib/evidence-encoding.mjs';

const USAGE = [
  '用法: node .github/scripts/preflight-pr.mjs [选项]',
  '',
  '  --body-file <path>  PR 正文文件（未给且未给 --pr 时按空正文判）',
  '  --pr <number>       PR 已存在时读它的正文',
  '  --base <ref>        比较基线（默认 origin/master，本地 git 用它，API 自动剥 origin/）',
  '  --commit <sha>      被检 commit（默认 HEAD）',
  '  --json              输出 JSON（供 submit-pr.ps1 消费）',
  '  -h, --help          本帮助',
].join('\n');

export function parseArgs(argv) {
  const out = { base: 'origin/master', commit: '', bodyFile: '', pr: 0, json: false, help: false };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '-h' || arg === '--help') out.help = true;
    else if (arg === '--json') out.json = true;
    else if (arg === '--base') out.base = need(argv, ++i, arg);
    else if (arg === '--commit') out.commit = need(argv, ++i, arg);
    else if (arg === '--body-file') out.bodyFile = need(argv, ++i, arg);
    else if (arg === '--pr') out.pr = Number(need(argv, ++i, arg));
    else throw new UsageError(`未知参数：${arg}`);
  }
  if (!Number.isInteger(out.pr)) throw new UsageError('--pr 只能是整数');
  return out;
}

class UsageError extends Error {}

function need(argv, i, flag) {
  const v = argv[i];
  if (v === undefined || v.startsWith('--')) throw new UsageError(`${flag} 缺参数`);
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

/**
 * 注入校验器的 fetchCompare：导出工厂（#2044 TDD 提取 seam）。
 * 参数形态是契约面：校验器按 fetchCompare({ base, head }) 调用、CI 注入实现也按 ({ base, head }) 收，
 * 本地预检必须同构 —— 测试面在 preflight-pr.test.mjs 钉三方一致性。
 * compareOfImpl / warn 仅测试注桩用，默认走生产路径（gh api 与 stderr）。
 */
export function makeFetchCompare(repo, { compareOfImpl = compareOf, warn } = {}) {
  const write = warn || ((m) => process.stderr.write(m));
  const compareCache = new Map();
  return ({ base, head }) => {
    const key = `${base}...${head}`;
    if (compareCache.has(key)) return compareCache.get(key);
    let value = null;
    try {
      const c = compareOfImpl(repo, String(base).replace(/^origin\//, ''), String(head));
      value = { status: c.status, mergeBaseSha: c.mergeBaseSha, files: c.files.map((f) => f.filename), filesTruncated: c.truncated };
    } catch (e) {
      // 与 CI 同口径：compare 拿不到就收敛成 null，让校验器 fail-closed（不当成「没改」）。
      write(`compare ${key} 失败，按 fail-closed 处理：${e.message}\n`);
      value = null;
    }
    compareCache.set(key, value);
    return value;
  };
}

function prBody(pr) {
  const view = ghJson(['pr', 'view', String(pr), '--json', 'body']);
  return (view && view.body) || '';
}

/**
 * ci-summary 的 run 状态（#1770 S4 的遗留用途，A1 后**不参与判据**）：
 * 它只在输出里给人看一句「这个 commit 的 CI 绿了没」——开 PR 前的自查，
 * 以及 submit-pr.ps1 的「等 CI 绿再建 PR」。
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

const GATE_KEYS = GATE_EVIDENCE.gates.map((g) => g.key);

/**
 * #2053：引用完整性 lint 结果并进预检——红项进 errors、ok 共决、notes 透传；gates 原样不动。
 * 纯函数；行为钉在 lint-issue-references.test.mjs 切片5（含「全绿时原结论不变」与「不新增门」两条）。
 */
export function mergeRefLint(result, lint) {
  const reds = lint.reds || [];
  return {
    ...result,
    ok: result.ok && reds.length === 0,
    errors: [...result.errors, ...reds.map((x) => '引用漂移 · ' + x.message)],
    notes: [...result.notes, ...(lint.notes || [])],
  };
}

/**
 * #2139：证据日志可解码性并进预检——与 #2053 同构（红项进 errors、ok 共决、读数行进 notes）。
 * 不新增门、不动 gate-validator 的门清单与行措辞：本票只治「证据文件可读性」这一维。
 * 纯函数；行为钉在 evidence-encoding.test.mjs 判据六 / 七（必红一条、必不红一条，成对）。
 */
export function mergeEncodingLint(result, lint) {
  const reds = lint.reds || [];
  return {
    ...result,
    ok: result.ok && reds.length === 0,
    errors: [...result.errors, ...reds],
    notes: [...result.notes, ...(lint.notes || [])],
  };
}

/** 分支点时刻 = merge-base(HEAD, base) 的提交时刻；读不到返回 0，调用方据此跳过并在 notes 说明。 */
function branchPointMs(baseApi) {
  for (const ref of [baseApi, `origin/${baseApi}`]) {
    try {
      const sha = git('merge-base', 'HEAD', ref).trim();
      const secs = Number(git('show', '-s', '--format=%ct', sha).trim());
      if (Number.isFinite(secs) && secs > 0) return secs * 1000;
    } catch { /* 换下一个 ref 再试 */ }
  }
  return 0;
}

function render(ctx) {
  const { args, commit, compare, ciRun, result, hasBody } = ctx;
  const lines = [];
  lines.push('#1770 预检（与 CI 同一份判据：.github/scripts/lib/gate-validator.mjs；A1 形态）');
  lines.push(`  commit   ${commit.slice(0, 12)}`);
  lines.push(`  base     ${args.base}（compare ${compare.status}，${compare.files.length} 个文件${compare.truncated ? '，已截断' : ''}）`);
  lines.push(`  证据     正文 ${hasBody ? '有' : '无'}（A1：只认正文，不读任何评论）`);
  if (ciRun) lines.push(`  ci-summary  ${ciRun.conclusion || '?'} ${ciRun.url || '（无链接）'}（仅提示，不参与判据）`);
  lines.push("");
  for (const key of GATE_KEYS) {
    const gate = result.gates && result.gates[key];
    if (!gate) continue;
    const state = gate.required === false ? '— 免' : gate.bound ? `✅ 已绑 ${gate.commit}` : '❌ 未绑';
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
  const baseApi = args.base.replace(/^origin\//, '');
  const compare = compareOf(repo, baseApi, commit);
  const hasBody = Boolean(args.bodyFile || args.pr);
  const body = args.bodyFile ? readFileSync(args.bodyFile, 'utf8') : args.pr ? prBody(args.pr) : '';
  const fetchCompare = makeFetchCompare(repo);
  const result = await validatePrEvidence({ files: compare.files, body, headSha: commit, fetchCompare });
  // #2053：对 PR 正文跑引用完整性 lint（与证据段判据同层）；空正文无从判起，直接放行分支。
  const refLint = body
    ? await lintIssueReferences(body, { ...makeFsFacts(git('rev-parse', '--show-toplevel').trim()), resolveNumber: makeGhNumberResolver(repo, { run }) })
    : { ok: true, reds: [], notes: [] };
  const refMerged = mergeRefLint(result, refLint);
  // #2139：证据日志可解码性 —— 只判**本次新产出**（mtime 不早于分支点）的 `.ci-verify` 日志。
  // 读不到分支点就不判（不替别的会话的存量背红），但把「跳过」说出来，不留静默绿。
  const sinceMs = branchPointMs(baseApi);
  const encLint = sinceMs > 0
    ? classifyForPreflight(scanEvidenceLogs({ root: git('rev-parse', '--show-toplevel').trim(), sinceMs }))
    : { ok: true, reds: [], notes: ['证据日志可读性：跳过（读不到分支点基线）'] };
  const final = mergeEncodingLint(refMerged, encLint);
  const ciRun = pickCiSummaryRun(checkRuns(repo, commit), commit);
  if (args.json) {
    console.log(JSON.stringify({ ok: final.ok, errors: final.errors, notes: final.notes, gates: final.gates, commit, base: args.base, ciRun }, null, 2));
  } else {
    console.log(render({ args, commit, compare, ciRun, result: final, hasBody }));
  }
  return final.ok ? 0 : 1;
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
