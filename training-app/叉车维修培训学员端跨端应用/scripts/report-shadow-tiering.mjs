#!/usr/bin/env node
// ADR-0052 波 3a（#2181）：影子分档的**可复算**回放器。
//
// 为什么需要它：Shadow 注记只进 pr-evidence 的 `core.info` 日志、不落盘，
// 影子窗口本身可能一个样本都没有（实测：波 2 合并后 5 次 run、命中运行时面 0 个）。
// 分档判据 `classifyTiering` 是纯函数（文件清单 + 名单 + 正文），所以历史已合并 PR
// 可以离线回放——这是唯一能给出分布的样本来源。
//
// **判据只有一份**：直接 import `.github/scripts/lib/gate-validator.mjs`，
// 绝不在此复制阶梯实现（两份口径正是 #1144 那一类血账的形态）。
//
// 自 #2199（ADR-0053 P0）起每次运行都产出一份 manifest（默认 `.scratch/3a/manifest.json`）：
//   - validatorBlobSha / validatorCommitSha / coreListHash / switches：证明两次运行用的是
//     同一套判据（#2193 §6-1）；
//   - capturedAtUtc / sourceCommand / window：样本窗口显式闭合、可复核（§6-3）；
//   - prCount / runtimeHitCount / shadowLineCount / patchMissingCount / anomalyCount /
//     cacheHitCount / cacheRebuiltCount：一律显式落盘，**0 也写**（§6-4）——「没跑异常检查」
//     与「异常 0」从此可区分。
// 缓存判据是 (validatorBlobSha, coreListHash, prNumber) 三元组（§6-2）：判据模块或名单一变，
// 旧档即失效重拉，堵住「判据改了还在用旧档」。
// 先拉后算（§6-6）：阶段 A 把全部 PR 的 files 原始档落到 raw/，阶段 B 才计算；原料在
// 90 天天花板内被搬离 GitHub 侧。
//
// 窗口排序：gh pr list 没有 --sort/--direction（2.97 实测；gh search prs 的 --sort 也不含
// merged），所以顺序在本地显式做——按 mergedAt 降序、同刻按 PR 号降序；窗口首末两端随
// manifest.window 落盘，两次运行的样本顺序因此可 diff。
//
// 用法：
//   node scripts/report-shadow-tiering.mjs [--limit 200] [--out <json>] [--cache <dir>] [--manifest <json>]
// 输出：摘要打 stdout；逐 PR 明细写 replay.json、版本戳写 manifest.json（默认都落 .scratch/3a/，不入仓）。

import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const argOf = (name, dflt) => {
  const i = process.argv.indexOf(`--${name}`);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : dflt;
};

const REPO = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
const LIMIT = Number(argOf('limit', '200'));
const OUT = path.resolve(argOf('out', path.join(REPO, '.scratch/3a/replay.json')));
const CACHE = path.resolve(argOf('cache', path.join(REPO, '.scratch/3a/raw')));
const MANIFEST = path.resolve(argOf('manifest', path.join(REPO, '.scratch/3a/manifest.json')));
const CORE_LIST = path.join(REPO, '.github/scripts/lib/mobile-gate-core-list.json');
const VALIDATOR = path.join(REPO, '.github/scripts/lib/gate-validator.mjs');

// sourceCommand 只记 gh 出站命令（逐条原样，含 --limit 等参数）；git 本地读不算数据面。
const sourceCommands = [];
const gh = (args) => {
  sourceCommands.push(['gh', ...args].join(' '));
  return execFileSync('gh', args, { cwd: REPO, maxBuffer: 64 * 1024 * 1024, encoding: 'utf8' });
};
const gitOut = (args) => execFileSync('git', args, { cwd: REPO, encoding: 'utf8' }).trim();

// —— 判据基（#2193 §6-1）：实际被 import 的模块内容哈希 + 它在 HEAD 的最后改动提交 ——
// hash-object 算的是**工作区文件内容**的 blob id：本地改了判据但没提交，缓存也会失效。
const validatorBlobSha = gitOut(['hash-object', VALIDATOR]).slice(0, 12);
const validatorCommitSha = (() => {
  try {
    return gitOut(['log', '-1', '--format=%h', '--', '.github/scripts/lib/gate-validator.mjs']);
  } catch {
    return null;
  }
})();

const sha256 = (buf) => crypto.createHash('sha256').update(buf).digest('hex');
const coreListBuf = fs.readFileSync(CORE_LIST);
const coreList = JSON.parse(coreListBuf.toString('utf8'));
const coreListHash = sha256(coreListBuf).slice(0, 12);
const switches = {
  exemptionActive: coreList?.switches?.exemptionActive ?? null,
  logicLiteTierOpen: coreList?.switches?.logicLiteTierOpen ?? null,
};

const { classifyTiering, isRuntimeFile, validatePrEvidence } = await import(pathToFileURL(VALIDATOR).href);

// —— 与 gate-validator 的 hitsMpWeixinFace 同口径：三份 json，或运行时面 diff 里有 MP-WEIXIN 条件编译指令行 ——
const MP_WEIXIN_FILES = ['manifest.json', 'platformConfig.json'];
const MP_DIRECTIVE = /^\s*(?:\/{2,}|\/\*|<!--|["'])?\s*#\s*(?:ifdef|ifndef|if)\s+[A-Za-z0-9_\s|&()!-]*\bMP-WEIXIN\b/;
const needs2Files = (raw) =>
  raw
    .filter((f) => {
      if (f.filename.startsWith('training-app/') && MP_WEIXIN_FILES.includes(f.filename.split('/').pop())) return true;
      if (!isRuntimeFile(f.filename)) return false;
      return String(f.patch || '')
        .split('\n')
        .filter((l) => /^[+-]/.test(l) && !/^(?:\+\+\+|---)/.test(l))
        .map((l) => l.slice(1))
        .some((c) => MP_DIRECTIVE.test(c));
    })
    .map((f) => f.filename);

// ② 读数行只认「token 后紧跟 key=value」的真读数；正文里提到 token 的散文不算（#1916 实测踩过）。
const GATE2_READING = /MP_WEIXIN_RESULT\s+[a-z][\w-]*=/i;
const gate2Class = (line) =>
  !line ? '无' : /env-blocked/.test(line) ? 'env-blocked' : /errorsTotal=0/.test(line) ? 'errorsTotal=0' : '其他';

const tally = (xs) => xs.reduce((m, k) => ((m[k] = (m[k] || 0) + 1), m), {});

// —— 缓存（#2193 §6-2）：三元组 (validatorBlobSha, coreListHash, prNumber) 全等才复用。
// 旧版裸数组档没有版本戳，一律视为未命中（重建），不静默沿用 —— 那正是本票要堵的洞。
function readCache(prNumber) {
  const file = path.join(CACHE, `pr-${prNumber}.json`);
  if (!fs.existsSync(file)) return null;
  try {
    const doc = JSON.parse(fs.readFileSync(file, 'utf8'));
    if (doc?.version !== 1) return null;
    if (doc.prNumber !== prNumber) return null;
    if (doc.validatorBlobSha !== validatorBlobSha) return null;
    if (doc.coreListHash !== coreListHash) return null;
    return Array.isArray(doc.files) ? doc.files : null;
  } catch {
    return null;
  }
}

function writeCache(prNumber, files) {
  fs.mkdirSync(CACHE, { recursive: true });
  const file = path.join(CACHE, `pr-${prNumber}.json`);
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify({
    version: 1,
    prNumber,
    validatorBlobSha,
    coreListHash,
    capturedAtUtc: new Date().toISOString(),
    files,
  }));
  fs.renameSync(tmp, file);
}

// —— 阶段 A：先拉（原始档先落盘，#2193 §6-6） ——
const capturedAtUtc = new Date().toISOString();
const repoSlug = gh(['repo', 'view', '--json', 'nameWithOwner', '--jq', '.nameWithOwner']).trim();
const prs = JSON.parse(gh([
  'pr', 'list',
  '--state', 'merged',
  '--limit', String(LIMIT),
  '--json', 'number,title,mergedAt,body',
]));

const rawByPr = new Map();
let cacheHitCount = 0;
let cacheRebuiltCount = 0;
let patchMissingCount = 0;
for (const pr of prs) {
  let raw = readCache(pr.number);
  if (raw) cacheHitCount += 1;
  else {
    raw = JSON.parse(gh(['api', `repos/${repoSlug}/pulls/${pr.number}/files`, '--paginate']));
    writeCache(pr.number, raw);
    cacheRebuiltCount += 1;
  }
  if (!Array.isArray(raw)) throw new Error(`PR #${pr.number} 的 files 原始档不是数组`);
  rawByPr.set(pr.number, raw);
  // 缺 patch = 二进制 / 超大 diff：分档对这些文件只能看文件名与状态，必须计数（#2193 §6-4）。
  for (const f of raw) if (f.patch == null) patchMissingCount += 1;
}

// —— 阶段 B：后算（只读阶段 A 落下的原始档；判据与影子注记都取自 gate-validator 本体） ——
const rows = [];
let shadowLineCount = 0;
let anomalyCount = 0;
for (const pr of prs) {
  const raw = rawByPr.get(pr.number);
  const files = raw.map((f) => ({ filename: f.filename, status: f.status, patch: f.patch }));
  // PR 档 = 各文件最坏值；单文件再走一遍分档器即得该文件自己的档位（判据仍是同一份）。
  const perFile = files
    .filter((f) => isRuntimeFile(f.filename))
    .map((f) => {
      const one = classifyTiering({ files: [f], coreList, bodyText: '' });
      return { filename: f.filename, status: f.status, tier: one.tier, why: (one.triggers[0] || '').split('：').slice(1).join('：') };
    });
  const body = pr.body || '';
  // 影子行数与分档结论都从 validatePrEvidence 本体取：CI 到时打出的就是这些 SHADOW-0052 行（#2193 §6-5）。
  const checked = await validatePrEvidence({ files: raw, body, headSha: '', fetchCompare: null, coreList });
  const tiering = checked.tiering;
  const shadowLines = checked.notes.filter((n) => n.includes('SHADOW-0052'));
  shadowLineCount += shadowLines.length;
  anomalyCount += tiering.anomalies.length;
  const gate2 = (body.match(/MP_WEIXIN_RESULT[^\n]*/g) || []).map((s) => s.trim()).filter((s) => GATE2_READING.test(s));
  rows.push({
    number: pr.number,
    title: pr.title,
    mergedAt: pr.mergedAt,
    fileCount: files.length,
    runtimeFiles: files.filter((f) => isRuntimeFile(f.filename)).map((f) => f.filename),
    needs2Files: needs2Files(raw),
    tier: tiering.tier,
    declared: tiering.declared,
    verified: tiering.verified,
    planned: tiering.planned,
    triggers: tiering.triggers,
    perFile,
    anomalies: tiering.anomalies,
    shadowLineCount: shadowLines.length,
    gate2,
    gate2Class: gate2Class(gate2[0]),
    changedLines: (body.match(/TEST_(?:UNIT|BEHAVIOR)_CHANGED_RESULT[^\n]*/g) || []).map((s) => s.trim()),
  });
}

// 本地显式排序（gh pr list 无 --sort）：mergedAt 降序、同刻按 PR 号降序 ⇒ 窗口可复现。
rows.sort((a, b) => String(b.mergedAt || '').localeCompare(String(a.mergedAt || '')) || b.number - a.number);

const newest = rows[0] || null;
const oldest = rows[rows.length - 1] || null;
const manifest = {
  version: 1,
  validatorBlobSha,
  validatorCommitSha,
  coreListHash,
  switches,
  capturedAtUtc,
  sourceCommand: sourceCommands,
  window: {
    firstPrNumber: oldest ? oldest.number : null,
    firstMergedAt: oldest ? oldest.mergedAt : null,
    lastPrNumber: newest ? newest.number : null,
    lastMergedAt: newest ? newest.mergedAt : null,
  },
  prCount: rows.length,
  runtimeHitCount: rows.filter((r) => r.runtimeFiles.length > 0).length,
  shadowLineCount,
  patchMissingCount,
  anomalyCount,
  cacheHitCount,
  cacheRebuiltCount,
};

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, JSON.stringify(rows, null, 2));
fs.mkdirSync(path.dirname(MANIFEST), { recursive: true });
fs.writeFileSync(MANIFEST, JSON.stringify(manifest, null, 2));

// —— 摘要（报告里的每个数字都出自这一段）——
const rt = rows.filter((r) => r.runtimeFiles.length > 0);
const allFiles = rt.flatMap((r) => r.perFile);
const withGate2 = rt.filter((r) => r.gate2.length > 0);
const span = rows.length ? `${oldest.mergedAt} → ${newest.mergedAt}` : '空样本';
console.log(`样本：${rows.length} 个已合并 PR（${span}）`);
console.log(`命中运行时面：${rt.length}/${rows.length}；② 实际触发面（needs2）：${rt.filter((r) => r.needs2Files.length).length}/${rt.length}`);
console.log(`PR 档分布（运行时面内）：${JSON.stringify(tally(rt.map((r) => r.tier)))}`);
console.log(`逐文件档位（${allFiles.length} 个运行时文件）：${JSON.stringify(tally(allFiles.map((f) => f.tier)))}`);
console.log(`全门原因直方图：${JSON.stringify(tally(allFiles.filter((f) => f.tier === 'full').map((f) => f.why)))}`);
console.log(`正文档位声明：${JSON.stringify(tally(rt.map((r) => r.declared || '未声明')))}`);
console.log(`planned 双证状态：${JSON.stringify(tally(rt.map((r) => r.planned.state)))}；changed 读数行 PR 数：${rt.filter((r) => r.changedLines.length).length}`);
console.log(`② 读数行（真读数）：${withGate2.length}/${rt.length}；分类：${JSON.stringify(tally(withGate2.map((r) => r.gate2Class)))}`);
console.log(`② 必过却无读数行：${rt.filter((r) => r.needs2Files.length && !r.gate2.length).map((r) => `#${r.number}`).join(' ') || '无'}`);
console.log(`若转正（3b）少跑必过 ② 的 PR：${rt.filter((r) => r.tier !== 'full' && r.needs2Files.length).length}；少跑任意 ② 的 PR：${rt.filter((r) => r.tier !== 'full').length}`);
console.log(`含公共目录改动的 PR：${rt.filter((r) => r.perFile.some((f) => f.why.includes('公共目录'))).length}；含新增/删除文件的 PR：${rt.filter((r) => r.perFile.some((f) => /状态 (?:added|removed)/.test(f.why))).length}`);
const switchesText = `exemptionActive:${switches.exemptionActive},logicLiteTierOpen:${switches.logicLiteTierOpen}`;
console.log(`manifest：${MANIFEST}`);
console.log(`REPLAY_MANIFEST validatorBlobSha=${validatorBlobSha} validatorCommitSha=${validatorCommitSha} coreListHash=${coreListHash} switches=${switchesText} prCount=${manifest.prCount} runtimeHitCount=${manifest.runtimeHitCount} shadowLineCount=${manifest.shadowLineCount} patchMissingCount=${manifest.patchMissingCount} anomalyCount=${manifest.anomalyCount} cacheHit=${manifest.cacheHitCount} cacheRebuilt=${manifest.cacheRebuiltCount}`);
console.log(`明细：${OUT}`);
