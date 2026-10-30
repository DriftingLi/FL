#!/usr/bin/env node
/**
 * #2139 证据日志可解码性判据（库层，零副作用）。
 *
 * 为什么存在：A1 证据形态下证据是荣誉制（校验器不读图、不校验内容真伪），而 2026-10-30 实测
 * `.ci-verify` 下的 `*.log` 有 36/99 份读不出来、且损坏不可逆。没有任何门会发现手里的日志是坏的
 * ⇒ 这条判据补的正是「可解码性」这一维，挂在 preflight-pr（仓库根、PR 前本就跑、不占 HBuilderX）。
 *
 * 三类损坏（本文件按字节形态分，比诊断脚本原来的 A/B 两分更细；同名读数可对照）：
 *   A-gbk-mojibake    文件是合法 UTF-8，内容是「UTF-8 字节被当 GBK 解码后又写回 UTF-8」的乱码字。
 *   B1-raw-bytes      文件字节本身不是合法 UTF-8（原始非 UTF-8 字节直接落盘，如 cmd 内部重定向）。
 *   B2-replacement    文件是合法 UTF-8，但里面已经烤进了 U+FFFD（按块解码把多字节序列切断的产物 ——
 *                     #2139 实测：`test-behavior.log` 与 `2045-behavior.log` 各有 56 个、位置成簇
 *                     2376/2378/2379/2380 ⇒ 与「原始 GBK 落盘」不是同一个根因，修法也不同）。
 *
 * 编码自律：本文件与它的 CLI、测试**只用 ASCII 源码**，乱码签名以码位表表达。
 * 这不是洁癖 —— #2139 执行会话实测：把 U+FFFD 字面量写进源码会被写入链**整字吞掉**（当场得到空
 * 字符串），所以判据自身也不能依赖非 ASCII 字面量。钉在 evidence-encoding.test.mjs 的「源码纯 ASCII」用例上。
 *
 * 只读：不写任何文件、不改任何门的通过判据。
 */
import fs from 'node:fs';
import path from 'node:path';

/** UTF-8 中文被按 GBK 误读后高频出现的字（诊断脚本原表的 15 个码位，逐个来自现测样本）。 */
export const MOJIBAKE_CODES = [
  0x9359, 0x9352, 0x9428, 0x951B, 0x5BF0, 0x6D60, 0x93C4, 0x95C7,
  0x95C6, 0x935A, 0x5BEE, 0x93B4, 0x7F01, 0x59AB, 0x7481,
];

/** 正常中文技术日志几乎不会连续命中这些字；沿用诊断脚本的阈值，保证与历史读数可比。 */
export const MOJIBAKE_MIN = 3;

export const VERDICT = {
  CLEAN: 'clean',
  A_GBK_MOJIBAKE: 'A-gbk-mojibake',
  B1_RAW_BYTES: 'B1-raw-bytes',
  B2_REPLACEMENT: 'B2-replacement-char',
};

const MOJIBAKE_SET = new Set(MOJIBAKE_CODES);

/**
 * 对一个证据文件的字节做定性。纯函数，喂 Buffer 即可（测试用夹具直接构造字节）。
 * 判读顺序有意义：先定「字节是否合法 UTF-8」，再在合法文件里区分「已烤进的 U+FFFD」与「GBK 乱码字」。
 */
export function classifyBytes(buf) {
  let validUtf8 = true;
  try {
    new TextDecoder('utf-8', { fatal: true }).decode(buf);
  } catch {
    validUtf8 = false;
  }
  const text = buf.toString('utf8');
  let fffd = 0;
  let moji = 0;
  for (let i = 0; i < text.length; i += 1) {
    const code = text.charCodeAt(i);
    if (code === 0xFFFD) fffd += 1;
    else if (MOJIBAKE_SET.has(code)) moji += 1;
  }
  let verdict = VERDICT.CLEAN;
  if (!validUtf8) verdict = VERDICT.B1_RAW_BYTES;
  else if (fffd > 0) verdict = VERDICT.B2_REPLACEMENT;
  else if (moji >= MOJIBAKE_MIN) verdict = VERDICT.A_GBK_MOJIBAKE;
  return { verdict, validUtf8, fffd, moji, bytes: buf.length };
}

/**
 * 收集一棵树里的证据日志：任意 `.ci-verify` 目录下的 `*.log`。
 * 剪掉 node_modules / .git，并剪掉**别的会话的树**（`wt-*` / `wtx*` / `.scratch`）——
 * 一票一树一分支，本树只该为自己新产出的证据负责，不替并发会话背红。
 */
export function collectEvidenceLogs(root, pruneDirs = ['wt-', 'wtx', '.scratch']) {
  const out = [];
  const stack = [root];
  while (stack.length > 0) {
    const dir = stack.pop();
    let ents;
    try {
      ents = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      continue;
    }
    for (const e of ents) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name === 'node_modules' || e.name === '.git') continue;
        if (pruneDirs.some((pre) => e.name.startsWith(pre))) continue;
        stack.push(p);
      } else if (e.isFile() && e.name.endsWith('.log') && p.split(path.sep).includes('.ci-verify')) {
        out.push(p);
      }
    }
  }
  return out.sort();
}

/**
 * 扫描一棵树的证据日志，返回可读性结论。
 * `sinceMs`：只判**本次新产出**的文件（mtime 早于它的存量按票面裁定「不可复算、不救」豁免，
 * 豁免条数进 skippedStale，不静默丢弃）。缺省不豁免（全树判定，诊断形态）。
 */
export function scanEvidenceLogs({ root, sinceMs = 0 } = {}) {
  // fail-closed：root 不存在时**抛错**，不返回 files=0。
  // 「扫不到文件」和「文件都干净」是两件事 —— 把前者写成后者就是 guards.md 说的恒真判据
  // （本会话实测踩过：root 解析成多行垃圾 ⇒ files=0 ⇒ 判据看起来绿，而树上日志明明是坏的）。
  if (!fs.existsSync(root)) {
    throw new Error(`证据日志判据：树根不存在，无法判定（拒绝把「扫不到」读成「全干净」）：${root}`);
  }
  const files = collectEvidenceLogs(root);
  const rows = [];
  let scanned = 0;
  let skippedStale = 0;
  for (const f of files) {
    let mtimeMs = 0;
    try {
      mtimeMs = fs.statSync(f).mtimeMs;
    } catch {
      continue;
    }
    if (sinceMs > 0 && mtimeMs < sinceMs) {
      skippedStale += 1;
      continue;
    }
    scanned += 1;
    const buf = fs.readFileSync(f);
    const c = classifyBytes(buf);
    if (c.verdict !== VERDICT.CLEAN) {
      rows.push({ file: path.relative(root, f), ...c });
    }
  }
  return {
    root,
    sinceMs,
    files: scanned,
    corrupt: rows.length,
    clean: scanned - rows.length,
    skippedStale,
    rows,
    ok: rows.length === 0,
  };
}

/** 读一行读数，给 CLI 与 preflight 共用（同一份措辞，不抄第二份）。 */
export function formatScanSummary(scan) {
  const scope = scan.sinceMs > 0 ? `since=${new Date(scan.sinceMs).toISOString()}` : 'since=none';
  return `EVIDENCE_ENCODING files=${scan.files} corrupt=${scan.corrupt} clean=${scan.clean} skippedStale=${scan.skippedStale} ${scope}`;
}

/**
 * 扫描结果 → 预检 lint 形状（`{ ok, reds, notes }`），与 #2053 的 `lintIssueReferences` 同构。
 * `reds` 是给人直接看的字符串（已带类别与计数），preflight 只负责并进 errors、不二次加工。
 * `notes` 恒带一行读数（A1 形态下证据是荣誉制，读数行是「手里的日志到底能不能读」的唯一机读出口）。
 */
export function classifyForPreflight(scan) {
  return {
    ok: scan.corrupt === 0,
    reds: scan.rows.map((r) => `证据日志不可解码 · ${r.file} · ${r.verdict}（fffd=${r.fffd} moji=${r.moji} bytes=${r.bytes}）`),
    notes: [
      formatScanSummary(scan),
      ...(scan.skippedStale > 0
        ? [`证据日志存量豁免 ${scan.skippedStale} 份（mtime 早于分支点，已证不可逆、不救，登记在 docs/agents/ledger.md）`]
        : []),
    ],
  };
}
