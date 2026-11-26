/**
 * 移动端 .ps1 里「git 路径输出」调用点的扫描与归属审计（#2874）
 *
 * 为什么需要它：本仓位于中文目录名下，core.quotepath 默认 true ⇒ git diff --name-only /
 * ls-files / status --porcelain 输出的路径是「双引号 + 八进制转义」形态；任一调用点漏解码，
 * 下游按 ^pages/ 锚定就恒失配（#1853：改动页清单恒空、门静默判绿）。
 *
 * 本模块只做两件事：
 *   1. 扫出候选调用点 —— PS1 注释先剥。票面那条 Select-String 量到的 37 是注释与说明文案行
 *      （37 行全是散文，零真实调用：本仓真实调用一律写成参数数组，字面量 "git diff" 只出现在
 *      注释里），故这里必须先剥注释再匹配，否则会对着注释反复分诊。
 *   2. 拿一张「归属表」对账 —— 每个调用点要么被解码 / 相对化 / 关 quotepath 保护，要么进允许
 *      列表并写明理由。归属表在契约测试里（utils/levelDetectContract.test.js），本模块不持有它。
 *
 * 解码器不在这里：唯一真源是 scripts/lib/level-detect.ps1 的 ConvertFrom-GitQuotedPath。
 */
'use strict';

const fs = require('fs');
const path = require('path');

/** 相对移动端项目根扫哪些目录。本仓 .ps1 只在这两处（scripts/*.ps1、scripts/lib/*.ps1）。 */
const SCAN_DIRS = ['scripts', 'scripts/lib'];

/**
 * 会产出「可能被 core.quotepath 转义」的路径的 git 输出开关。
 * 实测（2026-11-26，中文文件名 + 默认 quotepath）：
 *   会转义 → diff --name-only / --name-status / --numstat、ls-files、status --porcelain
 *   不转义 → rev-parse --show-toplevel|--git-common-dir、worktree list --porcelain
 * ⇒ 扫前五类；worktree list 由归属表按 allow-quotepath-immune 豁免（实测依据见测试注释）。
 */
const PATH_OUTPUT_TOKENS = ['--name-only', '--name-status', '--numstat', 'ls-files', '--porcelain'];

/** 归属表允许的 verdict。 */
const VERDICTS = [
  'quotepath-off',
  'decoded',
  'allow-count',
  'allow-quotepath-immune',
  'allow-narrative',
];

/** 说明文案（打印命令）不是调用点：它的第一个词是打印/抛错 cmdlet。 */
const PRINT_PREFIX = /^\s*(Write-(Host|Output|Warning|Information|Verbose|Error|Debug)|throw)\b/;

/**
 * 剥掉 PowerShell 注释，保留字符串内容（'--name-only' 这类引号里的 token 必须留下）。
 *
 * 状态机而非逐行正则：块注释 <# ... #> 可跨行、行注释 # 可在字符串里出现（'#'）、
 * 双引号里有反引号转义、单引号里 '' 是转义。
 * 注释内容替换成空格（保留换行），行号因此与原文一一对应。
 */
function stripPsComments(src) {
  const out = [];
  let state = 'code';
  for (let i = 0; i < src.length; i += 1) {
    const c = src[i];
    const n = src[i + 1];
    if (state === 'blockcomment') {
      if (c === '#' && n === '>') {
        out.push(' ', ' ');
        i += 1;
        state = 'code';
      } else {
        out.push(c === '\n' ? '\n' : ' ');
      }
      continue;
    }
    if (state === 'linecomment') {
      if (c === '\n') {
        out.push('\n');
        state = 'code';
      } else {
        out.push(' ');
      }
      continue;
    }
    if (state === 'squote') {
      out.push(c);
      if (c === "'" && n === "'") {
        out.push(n);
        i += 1;
      } else if (c === "'") {
        state = 'code';
      }
      continue;
    }
    if (state === 'dquote') {
      out.push(c);
      if (c === '`') {
        out.push(n === undefined ? '' : n);
        i += 1;
      } else if (c === '"') {
        state = 'code';
      }
      continue;
    }
    // state === 'code'
    if (c === '<' && n === '#') {
      out.push(' ', ' ');
      i += 1;
      state = 'blockcomment';
    } else if (c === '#') {
      out.push(' ');
      state = 'linecomment';
    } else if (c === "'") {
      out.push(c);
      state = 'squote';
    } else if (c === '"') {
      out.push(c);
      state = 'dquote';
    } else {
      out.push(c);
    }
  }
  return out.join('');
}

/** 列出一棵树里要扫的 .ps1（相对路径一律正斜杠，便于在断言里写死与跨平台比较）。 */
function listPs1Files(root, dirs = SCAN_DIRS) {
  const files = [];
  for (const dir of dirs) {
    const abs = path.join(root, dir);
    if (!fs.existsSync(abs)) continue;
    for (const name of fs.readdirSync(abs).sort()) {
      if (!name.endsWith('.ps1')) continue;
      files.push({ file: path.posix.join(dir.split(path.sep).join('/'), name), abs: path.join(abs, name) });
    }
  }
  return files;
}

/**
 * 扫出所有路径输出调用点。
 * 每条命中带 kind：'call'（真调用）或 'narrative'（说明文案里的示例命令）。
 */
function scanGitPathOutputs(root, dirs = SCAN_DIRS) {
  const hits = [];
  for (const { file, abs } of listPs1Files(root, dirs)) {
    const code = stripPsComments(fs.readFileSync(abs, 'utf8'));
    code.split('\n').forEach((line, i) => {
      const token = PATH_OUTPUT_TOKENS.find((t) => line.includes(t));
      if (!token) return;
      const text = line.trim();
      hits.push({ file, line: i + 1, token, text, kind: PRINT_PREFIX.test(text) ? 'narrative' : 'call' });
    });
  }
  return hits;
}

/**
 * 归属表对账。registry 每条形如
 *   { file, kind, sites, note, evidence? }
 * 断言四件事：
 *   ① 扫描到的文件集合 === 归属表的文件集合（新增/删掉一个调用点都会打破它）
 *   ② 每个文件的调用点数 === 该文件各条 sites 之和（数量漂移 ⇒ 需要逐点重新分诊）
 *   ③ narrative 命中只能记在 allow-narrative 名下
 *   ④ evidence 串必须仍能在该文件（剥注释后）里找到 —— 归属不能变成「写着写着就与代码脱钩」的纸条
 * 返回 { ok, violations, hits }；violations 为空才算绿（红夹具正是靠它证伪）。
 */
function auditGitPathOutputs({ root, registry, dirs = SCAN_DIRS }) {
  const hits = scanGitPathOutputs(root, dirs);
  const violations = [];

  const scanned = new Map();
  for (const h of hits) {
    const s = scanned.get(h.file) || { call: 0, narrative: 0, total: 0 };
    s[h.kind] += 1;
    s.total += 1;
    scanned.set(h.file, s);
  }

  const registered = new Map();
  for (const e of registry) {
    if (!VERDICTS.includes(e.kind)) violations.push(e.file + ': 未知归属 verdict「' + e.kind + '」');
    if (!Number.isInteger(e.sites) || e.sites <= 0) violations.push(e.file + ': sites 必须是正整数');
    if (!e.note || !String(e.note).trim()) violations.push(e.file + ': 缺理由（note）');
    if (!e.evidence) violations.push(e.file + ': 缺 evidence（无法自证归属）');
    registered.set(e.file, [...(registered.get(e.file) || []), e]);
  }

  for (const [file, s] of scanned) {
    if (!registered.has(file)) {
      violations.push(file + ': 扫描到 ' + s.total + ' 处路径输出调用，归属表里没有它（新增调用点未分诊）');
      continue;
    }
    const entries = registered.get(file);
    const sum = entries.reduce((a, e) => a + (Number(e.sites) || 0), 0);
    if (sum !== s.total) {
      violations.push(file + ': 扫描到 ' + s.total + ' 处，归属表记 ' + sum + ' 处（调用点漂移，需逐点重新分诊）');
    }
    const narSites = entries
      .filter((e) => e.kind === 'allow-narrative')
      .reduce((a, e) => a + (Number(e.sites) || 0), 0);
    if (narSites !== s.narrative) {
      violations.push(file + ': narrative 命中 ' + s.narrative + ' 处，allow-narrative 记 ' + narSites + ' 处');
    }
  }

  for (const [file, entries] of registered) {
    if (!scanned.has(file)) {
      violations.push(file + ': 归属表记了调用点，扫描却没找到（已删/已改，登记该同步删）');
      continue;
    }
    const abs = path.join(root, file.split('/').join(path.sep));
    const code = fs.existsSync(abs) ? stripPsComments(fs.readFileSync(abs, 'utf8')) : '';
    for (const e of entries) {
      if (!e.evidence) continue;
      if (!code.includes(e.evidence)) {
        violations.push(file + ': 归属 ' + e.kind + ' 的依据「' + e.evidence + '」在该文件里找不到');
      }
    }
  }

  return { ok: violations.length === 0, violations, hits };
}

module.exports = {
  SCAN_DIRS,
  PATH_OUTPUT_TOKENS,
  VERDICTS,
  stripPsComments,
  listPs1Files,
  scanGitPathOutputs,
  auditGitPathOutputs,
};
