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
// 用法：
//   node scripts/report-shadow-tiering.mjs [--limit 200] [--out <json>] [--cache <dir>]
// 输出：摘要打 stdout；逐 PR 明细写 JSON（默认落 .scratch/3a/replay.json，不入仓）。

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const argOf = (name, dflt) => {
  const i = process.argv.indexOf(`--${name}`);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : dflt;
};

const REPO = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
const LIMIT = Number(argOf('limit', '200'));
const OUT = path.resolve(argOf('out', path.join(REPO, '.scratch/3a/replay.json')));
const CACHE = path.resolve(argOf('cache', path.join(REPO, '.scratch/3a/raw')));
const CORE_LIST = path.join(REPO, '.github/scripts/lib/mobile-gate-core-list.json');

const gh = (args) => execFileSync('gh', args, { cwd: REPO, maxBuffer: 64 * 1024 * 1024, encoding: 'utf8' });

const coreList = JSON.parse(fs.readFileSync(CORE_LIST, 'utf8'));
const { classifyTiering, isRuntimeFile } = await import(
  `file://${path.join(REPO, '.github/scripts/lib/gate-validator.mjs').replace(/\\/g, '/')}`
);

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

/** 逐 PR 回放：files 取自 GitHub，正文取自 PR。CLI 打网络，命中缓存则不重取。 */
function replayOne(pr) {
  const cacheFile = path.join(CACHE, `pr-${pr.number}.json`);
  let raw;
  if (fs.existsSync(cacheFile)) raw = JSON.parse(fs.readFileSync(cacheFile, 'utf8'));
  else {
    raw = JSON.parse(gh(['api', `repos/${REPO_SLUG()}/pulls/${pr.number}/files`, '--paginate']));
    fs.mkdirSync(CACHE, { recursive: true });
    fs.writeFileSync(cacheFile, JSON.stringify(raw));
  }
  const files = raw.map((f) => ({ filename: f.filename, status: f.status, patch: f.patch }));
  // PR 档 = 各文件最坏值；单文件再走一遍分档器即得该文件自己的档位（判据仍是同一份）。
  const perFile = files
    .filter((f) => isRuntimeFile(f.filename))
    .map((f) => {
      const one = classifyTiering({ files: [f], coreList, bodyText: '' });
      return { filename: f.filename, status: f.status, tier: one.tier, why: (one.triggers[0] || '').split('：').slice(1).join('：') };
    });
  const body = pr.body || '';
  const tiering = classifyTiering({ files, coreList, bodyText: body });
  const gate2 = (body.match(/MP_WEIXIN_RESULT[^\n]*/g) || []).map((s) => s.trim()).filter((s) => GATE2_READING.test(s));
  return {
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
    gate2,
    gate2Class: gate2Class(gate2[0]),
    changedLines: (body.match(/TEST_(?:UNIT|BEHAVIOR)_CHANGED_RESULT[^\n]*/g) || []).map((s) => s.trim()),
  };
}

let repoSlug = null;
function REPO_SLUG() {
  if (!repoSlug) repoSlug = execFileSync('gh', ['repo', 'view', '--json', 'nameWithOwner', '--jq', '.nameWithOwner'], { cwd: REPO, encoding: 'utf8' }).trim();
  return repoSlug;
}

const prs = JSON.parse(gh(['pr', 'list', '--state', 'merged', '--limit', String(LIMIT), '--json', 'number,title,mergedAt,body']));
const rows = prs.map(replayOne);
fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, JSON.stringify(rows, null, 2));

// —— 摘要（报告里的每个数字都出自这一段）——
const rt = rows.filter((r) => r.runtimeFiles.length > 0);
const allFiles = rt.flatMap((r) => r.perFile);
const withGate2 = rt.filter((r) => r.gate2.length > 0);
console.log(`样本：${rows.length} 个已合并 PR（${rows.at(-1).mergedAt} → ${rows[0].mergedAt}）`);
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
console.log(`明细：${OUT}`);
