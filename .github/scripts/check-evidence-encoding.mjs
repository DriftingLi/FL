#!/usr/bin/env node
/**
 * #2139 证据日志可解码性判据（CLI 入口）。判据本体在 `lib/evidence-encoding.mjs`，这里不抄第二份。
 *
 * 用法：
 *   node .github/scripts/check-evidence-encoding.mjs                       # 全树判定（诊断形态）
 *   node .github/scripts/check-evidence-encoding.mjs --root <dir>          # 指定树根
 *   node .github/scripts/check-evidence-encoding.mjs --since-branch-point  # 只判本次新产出（预检形态）
 *   node .github/scripts/check-evidence-encoding.mjs --since <ISO|epochms>
 *   node .github/scripts/check-evidence-encoding.mjs --json                # 机读
 *
 * 退出码：0 = 干净；1 = 有受污染文件（逐行点名文件 + 类别）；2 = 用法 / 环境错误。
 * 只读：不改任何文件。存量豁免走 mtime，并在读数里如实报 `skippedStale=`，不静默丢弃。
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { scanEvidenceLogs, formatScanSummary } from './lib/evidence-encoding.mjs';

const USAGE = [
  '用法: node .github/scripts/check-evidence-encoding.mjs [选项]',
  '',
  '  --root <dir>             树根（默认本文件所在仓库的顶层）',
  '  --since <ISO|epochms>    只判 mtime 不早于该时刻的文件',
  '  --since-branch-point     用 merge-base(HEAD, origin/master) 的提交时刻作为「本次新产出」基线',
  '  --json                   输出 JSON',
  '  -h, --help               本帮助',
].join('\n');

class UsageError extends Error {}

function parseArgs(argv) {
  const out = { root: '', since: 0, json: false, help: false };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === '-h' || a === '--help') out.help = true;
    else if (a === '--json') out.json = true;
    else if (a === '--root') out.root = need(argv, ++i, a);
    else if (a === '--since') out.since = parseSince(need(argv, ++i, a));
    else if (a === '--since-branch-point') out.since = branchPointMs();
    else throw new UsageError(`未知参数：${a}`);
  }
  return out;
}

function need(argv, i, flag) {
  const v = argv[i];
  if (v === undefined || v.startsWith('--')) throw new UsageError(`${flag} 缺参数`);
  return v;
}

function parseSince(raw) {
  if (/^\d+$/.test(raw)) return Number(raw);
  const t = Date.parse(raw);
  if (Number.isNaN(t)) throw new UsageError(`--since 不是 epochms 也不是 ISO 时间：${raw}`);
  return t;
}

function gitRun(args) {
  return execFileSync('git', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
}

function repoTopLevel() {
  // 只准用无 pathspec 的形态：`rev-parse --show-toplevel -- <path>` 会把 pathspec **原样多行回显**
  // （本会话实测返回 "D:/FL/wt-2139\n--\n.github/scripts"）⇒ root 成多行垃圾 ⇒ 扫到 0 个文件、
  // 判据静默假绿。取第一行并实测目录存在；两处都不成立就抛，宁可 exit 2 也不出「干净」的结论。
  const here = path.dirname(fileURLToPath(import.meta.url));
  let candidate = '';
  try {
    candidate = gitRun(['rev-parse', '--show-toplevel']).trim().split(/\r?\n/)[0].trim();
  } catch {
    candidate = '';
  }
  if (!candidate || !fs.existsSync(candidate)) candidate = path.resolve(here, '..', '..');
  if (!fs.existsSync(candidate)) throw new UsageError(`找不到树根，无法扫描证据日志：${candidate}`);
  return candidate;
}

function branchPointMs() {
  let base = '';
  for (const ref of ['origin/master', 'master']) {
    try {
      base = gitRun(['merge-base', 'HEAD', ref]).trim();
      break;
    } catch { /* 试下一个 ref */ }
  }
  if (!base) throw new UsageError('--since-branch-point 需要能解析 origin/master 或 master 的 git 仓库');
  const secs = Number(gitRun(['show', '-s', '--format=%ct', base]).trim());
  if (!Number.isFinite(secs) || secs <= 0) throw new UsageError(`读不到分支点提交时刻：${base}`);
  return secs * 1000;
}

function main(argv) {
  const args = parseArgs(argv);
  if (args.help) {
    console.log(USAGE);
    return 0;
  }
  const root = args.root || repoTopLevel();
  const scan = scanEvidenceLogs({ root, sinceMs: args.since });
  if (args.json) {
    console.log(JSON.stringify({ ...scan, summary: formatScanSummary(scan) }, null, 2));
  } else {
    for (const r of scan.rows) {
      console.log(`${r.verdict}\tfffd=${r.fffd}\tmoji=${r.moji}\t${r.file}`);
    }
    console.log(formatScanSummary(scan));
  }
  return scan.ok ? 0 : 1;
}

try {
  process.exitCode = main(process.argv.slice(2));
} catch (err) {
  if (err instanceof UsageError) {
    process.stderr.write(`${err.message}\n\n${USAGE}\n`);
    process.exitCode = 2;
  } else {
    process.stderr.write(`判据出错：${err && err.message ? err.message : err}\n`);
    process.exitCode = 2;
  }
}
