import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  classifyBytes,
  collectEvidenceLogs,
  scanEvidenceLogs,
  formatScanSummary,
  classifyForPreflight,
  VERDICT,
  MOJIBAKE_CODES,
  MOJIBAKE_MIN,
} from './lib/evidence-encoding.mjs';
import { mergeEncodingLint } from './preflight-pr.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));

const FFFD = String.fromCharCode(0xFFFD);
const moji = (n) => String.fromCharCode(...MOJIBAKE_CODES.slice(0, n));

// ---- 夹具：全部用码位构造，源码不写任何非 ASCII 字面量（本票实测：U+FFFD 字面量会被写入链吞掉）----
const fixtures = {
  // 干净：正常 UTF-8 中文技术日志（码位 U+4E2D U+6587 =「中文」）
  clean: Buffer.from(`${String.fromCharCode(0x4e2d, 0x6587)} gate=ok exit=0\n`, 'utf8'),
  // A：合法 UTF-8 + 达到阈值的 GBK 误读字
  aMojibake: Buffer.from(`${moji(MOJIBAKE_MIN)} log line\n`, 'utf8'),
  // A 的下界另一侧：差一个乱码字不判红（钉住 MOJIBAKE_MIN 不是恒真判据）
  aUnderThreshold: Buffer.from(`${moji(MOJIBAKE_MIN - 1)} log line\n`, 'utf8'),
  // B1：字节本身不是合法 UTF-8（GBK 的「中文」两字直落盘）
  b1RawBytes: Buffer.from([0xd6, 0xd0, 0xce, 0xc4, 0x0a]),
  // B2：合法 UTF-8 但已烤进 U+FFFD（按块解码切断多字节序列的产物）
  b2Replacement: Buffer.from(`npm warn ${FFFD}${FFFD} suite done\n`, 'utf8'),
};

test('#2139 判据一：四类夹具各判对所判的类（必红的那几条真红）', () => {
  assert.equal(classifyBytes(fixtures.clean).verdict, VERDICT.CLEAN);
  assert.equal(classifyBytes(fixtures.aMojibake).verdict, VERDICT.A_GBK_MOJIBAKE);
  assert.equal(classifyBytes(fixtures.b1RawBytes).verdict, VERDICT.B1_RAW_BYTES);
  assert.equal(classifyBytes(fixtures.b2Replacement).verdict, VERDICT.B2_REPLACEMENT);
});

test('#2139 判据二：B1 与 B2 必须是两个类（诊断脚本原表把两者混成一类）', () => {
  const b1 = classifyBytes(fixtures.b1RawBytes);
  const b2 = classifyBytes(fixtures.b2Replacement);
  assert.equal(b1.validUtf8, false, 'B1 的字节不是合法 UTF-8');
  assert.equal(b2.validUtf8, true, 'B2 的字节是合法 UTF-8，坏在已烤进的替换字符');
  assert.ok(b1.fffd > 0 && b2.fffd > 0, '两者都带 U+FFFD ⇒ 只数 U+FFFD 分不开，必须再看合法性');
});

test('#2139 判据三：A 类阈值有下界（少一个乱码字不判红）', () => {
  assert.equal(classifyBytes(fixtures.aUnderThreshold).verdict, VERDICT.CLEAN);
  assert.equal(classifyBytes(fixtures.aUnderThreshold).moji, MOJIBAKE_MIN - 1);
});

// ---- 扫描面：临时树，验「本次新产出」的范围口径 ----
function makeTree() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-2139-'));
  const ciVerify = path.join(root, '.ci-verify');
  const app = path.join(root, 'training-app', 'app-x', '.ci-verify');
  const other = path.join(root, 'wt-9999', '.ci-verify');
  const noise = path.join(root, 'node_modules', 'pkg', '.ci-verify');
  fs.mkdirSync(ciVerify, { recursive: true });
  fs.mkdirSync(app, { recursive: true });
  fs.mkdirSync(other, { recursive: true });
  fs.mkdirSync(noise, { recursive: true });
  const put = (dir, name, buf, mtimeMs = 0) => {
    const p = path.join(dir, name);
    fs.writeFileSync(p, buf);
    if (mtimeMs) fs.utimesSync(p, new Date(mtimeMs), new Date(mtimeMs));
    return p;
  };
  return { root, ciVerify, app, other, noise, put };
}

test('#2139 判据四：扫描只收本树的 .ci-verify 日志，剪掉别的会话的树与 node_modules', () => {
  const t = makeTree();
  t.put(t.ciVerify, 'a.log', fixtures.clean);
  t.put(t.app, 'b.log', fixtures.aMojibake);
  t.put(t.other, 'c.log', fixtures.aMojibake);
  t.put(t.noise, 'd.log', fixtures.aMojibake);
  const found = collectEvidenceLogs(t.root).map((p) => path.basename(p)).sort();
  assert.deepEqual(found, ['a.log', 'b.log']);
  const scan = scanEvidenceLogs({ root: t.root });
  assert.equal(scan.files, 2);
  assert.equal(scan.corrupt, 1, '本树里那份 A 类要被点名');
  assert.equal(scan.rows[0].file, path.join('training-app', 'app-x', '.ci-verify', 'b.log'));
});

test('#2139 判据五：--since 只判本次新产出，豁免条数如实报（不静默丢）', () => {
  const t = makeTree();
  const stale = 1_000;
  const fresh = Date.now();
  t.put(t.ciVerify, 'stale.log', fixtures.b2Replacement, stale);
  t.put(t.ciVerify, 'fresh.log', fixtures.clean, fresh);
  const scan = scanEvidenceLogs({ root: t.root, sinceMs: fresh - 10_000 });
  assert.equal(scan.files, 1, '存量那份不进判定数');
  assert.equal(scan.skippedStale, 1, '但必须报出来，不能静默丢弃');
  assert.equal(scan.corrupt, 0);
  assert.equal(scan.ok, true);
});

test('#2139 读数行带 corrupt 与 clean 两个数（AC1 的「clean == files」要能机械判）', () => {
  const t = makeTree();
  t.put(t.ciVerify, 'ok.log', fixtures.clean, Date.now());
  const line = formatScanSummary(scanEvidenceLogs({ root: t.root }));
  assert.match(line, /EVIDENCE_ENCODING files=\d+ corrupt=\d+ clean=\d+ skippedStale=\d+/);
  assert.match(line, /files=1 corrupt=0 clean=1/);
});

// ---- 预检接线：同 #2053 的形状 —— 并进 errors、共决 ok、不动 gates、不新增门 ----
const baseResult = {
  ok: true,
  errors: [],
  notes: ['原有提示'],
  gates: { '\u2461': { key: '\u2461', required: true, bound: true, commit: 'abc1234' } },
};

test('#2139 判据六：受污染时预检转红并点名类别，且 gates 原样不动', () => {
  const lint = classifyForPreflight({ files: 3, corrupt: 1, clean: 2, sinceMs: 0, skippedStale: 0, rows: [{ file: 'x.log', verdict: VERDICT.B2_REPLACEMENT, fffd: 56, moji: 0, bytes: 91089 }], ok: false });
  const merged = mergeEncodingLint(baseResult, lint);
  assert.equal(merged.ok, false, '判红即不绿');
  assert.equal(merged.errors.length, 1);
  assert.match(merged.errors[0], /x\.log/);
  assert.match(merged.errors[0], /B2-replacement-char/, '必须点名类别');
  assert.deepEqual(merged.gates, baseResult.gates, '不动任何门的通过判据');
  assert.deepEqual(merged.notes.slice(0, 1), baseResult.notes, '原有 notes 保留在前');
});

test('#2139 判据七：全干净时不转红、gates 不动，读数行进 notes', () => {
  const lint = classifyForPreflight({ files: 3, corrupt: 0, clean: 3, sinceMs: 0, skippedStale: 7, rows: [], ok: true });
  const merged = mergeEncodingLint(baseResult, lint);
  assert.equal(merged.ok, baseResult.ok, '全干净不得把绿判成红');
  assert.deepEqual(merged.errors, baseResult.errors);
  assert.deepEqual(merged.gates, baseResult.gates, '不新增门');
  assert.match(merged.notes[1], /EVIDENCE_ENCODING files=3 corrupt=0 clean=3/);
  assert.match(merged.notes[2], /存量豁免 7 份/, '豁免必须说出来，不静默丢');
});

test('#2139 判据九：树根不存在必须抛错，不得退化成 files=0 的假绿（本会话实测踩过的形态）', () => {
  const missing = path.join(path.sep, 'fl-2139-no-such-tree');
  assert.throws(() => scanEvidenceLogs({ root: missing }), /树根不存在/);
  // 并且抛的是 Error（CLI 据此走 exit 2），不是返回一个 ok:true 的空结论
  assert.notEqual(classifyForPreflight, undefined);
});

test('#2139 判据八：判据自身不吃非 ASCII 字面量（本票实测 U+FFFD 字面量会被写入链整字吞掉）', () => {
  // 口径不是「文件必须纯 ASCII」（本仓注释写中文是常态），而是两条可机械判的：
  //   ① 源码里不许出现 U+FFFD 字面量 —— 它正是会被吞掉、吞掉后静默变成空串的那个字符；
  //   ② 判定所依赖的乱码签名字只许以码位表存在，不许以字面量散落在源码里。
  const forbidden = new Set([...MOJIBAKE_CODES, 0xFFFD]);
  for (const rel of ['lib/evidence-encoding.mjs', 'check-evidence-encoding.mjs', 'evidence-encoding.test.mjs']) {
    const text = fs.readFileSync(path.join(HERE, rel), 'utf8');
    const hits = [...text].filter((c) => forbidden.has(c.charCodeAt(0)));
    assert.equal(hits.length, 0, `${rel} 里出现了 ${hits.length} 个判据字面量（应改用码位）：${JSON.stringify(hits.slice(0, 3))}`);
  }
  // 码位表本身不许退化成空表 —— 空表会让判据恒绿（guards.md 判据①）
  assert.equal(MOJIBAKE_CODES.length, 15);
  assert.ok(MOJIBAKE_CODES.every((c) => c >= 0x2000), '码位都得落在真会误读的区块里');
});
