/**
 * 裸 pwsh 调用防回归棘轮守护（#2256，spec C 父票 #2234）—— 全仓测试文件静态扫描。
 *
 * ## 为什么需要它
 *
 * spec C（#2234）把台账上 66 处「裸 execFileSync(pwsh|powershell…)」逐条核销到 0（C-1 建账 / C-3 清零，
 * 票 #2254）。清零是一次性成果；下一个新测试再手写裸调用，破坏的是同一条纪律：Windows 控制台按 OEM
 * 代码页吐字节，调用方的 utf8 解码烤出 U+FFFD（#2139 / #2078 的病根）。本守护把「0」变成**永久约束**：
 * 出现一处 ⇒ 当场红，违例文案逐文件:行点名（照着能直接改）。
 *
 * ## 判据（口径与 C-1 台账一致，#2234 评论）
 *
 * 代码级解析 execFileSync( 的完整实参表；首参 = pwsh | powershell | powershell.exe |
 * powershellExe()（字面量/别名/回查最近赋值）。命中且**无** UTF-8 自律证据 ⇒ 判红。
 * 自律标记（PIN）与 utils/probeSpawn.js 的工具形态同款：encodeCommand( / withUtf8Discipline( /
 * UTF8_DISCIPLINE / [Console]::OutputEncoding / $OutputEncoding=。
 *
 * ## 豁免（四型，全部是**结构性**判据，不是文件名登记表）
 *
 *   1. **-File 临时 harness 型**：自律行写在 writeFileSync 的模板体里（devFinish / hxLaunch /
 *      GateDrainAndNoise / deviceCaptureHardening 的形态）⇒ writeFileSync 的内容变量纳入回查池。
 *      连带覆盖「已钉脚本的变异副本」：副本只 replace 业务行，自律行仍在。
 *   2. **encodeCommand 装填局部变量型**：数组 push('-EncodedCommand', encoded) ⇒ push 实参纳入
 *      回查池（deviceCaptureTier 的形态）。
 *   3. **-File 指向仓内已钉脚本型**：实参表解析出 .ps1 载体路径 ⇒ 读仓内该文件验 PIN
 *      （mpWeixin 系 gateArgs + scripts/mp-weixin-check.ps1 的形态）。
 *   4. **探针工具自身守护型**：probeSpawnBehavior 的裸调用是**必红腿的设计目的**（头注明说）⇒
 *      文件 require('./probeSpawn.js') ⇒ 整文件豁免。
 *
 * -File 调用点本身不改（C-3 口径：26 处 -File 改被调脚本），故 -File 形态的判红证据落在**载体文件**
 * 上，不在调用点 —— 这是 3 型豁免存在的原因，也是为什么豁免要读仓内文件验钉。
 *
 * ## 形态
 *
 * 沿用全仓守护先例（utils/uvueFontCarrierContract.test.js）：先对**注入的违规样本**断言检测有效
 * （防空跑假绿），再对真实文件断言零命中 + 扫描面自检。helper 自带重复、不抽公共模块（仓规）。
 *
 * ## 已知边界（写实，不宣称覆盖）
 *
 *   · 扫描面 = 本目录 utils/ 下全部 *.test.js（行为腿 + 接线腿都在这里，两腿都要锁）。
 *   · 只认 execFileSync 首参为 pwsh 系的形态；spawnSync / exec 回调系不在 C-1 台账口径内
 *     （台账只收 execFileSync），出现时由后续票扩口径。
 *   · 双缓冲扫描：调用点判在 codeOnly（注释与字符串已 blank 化 ⇒ 注释里的调用不算）。
 *     实参表的**内容**（首参字面量、.ps1 路径、模板串里的自律行）回 full 缓冲取 —— 字符串 blank 化
 *     只是让「注释里的调用」不响，不是把字符串内容当空气。
 *   · 实参表解析用括号配对，跨行调用照常；apply / 展开运算符形态不判（仓内现测 0 处）。
 *   · 豁免判据是「回查池里有 PIN 证据」，不是「文件里任何地方有 PIN」—— 回查池 = full 实参表 +
 *      标识符最近赋值回查（深度 2，codeOnly）+ writeFileSync 内容变量（full）+ push 实参（full）。
 *      精度不够的位会假红，届时优先扩回查池，不动豁免判据。
 */

'use strict';

const fs = require('fs');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const UTILS = __dirname;

/* ------------------------------------------------------------------ 扫描器 */

/** 自律标记（PIN）：与 utils/probeSpawn.js 的工具形态同款。 */
const PIN_RE = /encodeCommand\s*\(|withUtf8Discipline\s*\(|UTF8_DISCIPLINE|\[Console\]::OutputEncoding|\$OutputEncoding\s*=/;

/** 首参判 pwsh 系（在 full 缓冲的实参表上做：字面量/别名/powershellExe() 形态） */
const PWSH_FIRST_RE = /^(?:(?:pwsh|powershell|powershell\.exe)['"\s]|powershellExe\s*\(\s*\)\s*[,)]|[A-Za-z_$][\w$]*\s*[,)]|$)/;

// 首参为「裸标识符」时（别名/局部变量），是否判 pwsh 系由 traceAssign 回查其赋值决定；
// 上面的正则只吃「能当场定案」的三类：pwsh 系字面量开头 / powershellExe() 整体 / 标识符待回查。

/**
 * 双缓冲：
 *   codeOnly —— 注释与字符串整体 blank 化（换行保留）：调用点判在这里 ⇒ 注释里的调用不响。
 *   full     —— 原文：实参表内容（首参字面量、.ps1 路径、模板串自律行）从这里取。
 * 行号从 codeOnly 下标换算（blank 化保留换行 ⇒ 天然对齐源文件）。
 */
function codeBuffer(src) {
  let out = '';
  let i = 0;
  const n = src.length;
  while (i < n) {
    const c = src[i];
    if (c === '/' && src[i + 1] === '*') {
      const end = src.indexOf('*/', i + 2);
      const stop = end === -1 ? n : end + 2;
      out += src.slice(i, stop).replace(/[^\n]/g, ' ');
      i = stop;
      continue;
    }
    if (c === '/' && src[i + 1] === '/') {
      const end = src.indexOf('\n', i);
      const stop = end === -1 ? n : end;
      out += src.slice(i, stop).replace(/[^\n]/g, ' ');
      i = stop;
      continue;
    }
    if (c === '\'' || c === '"' || c === '`') {
      const quote = c;
      let j = i + 1;
      while (j < n) {
        if (src[j] === '\\') { j += 2; continue; }
        if (src[j] === quote) { j += 1; break; }
        j += 1;
      }
      out += src.slice(i, j).replace(/[^\n]/g, ' ');
      i = j;
      continue;
    }
    out += c;
    i += 1;
  }
  return out;
}

/** 某个下标所在行号（1 起，codeOnly 与 full 换行一致 ⇒ 通用） */
const lineAt = (text, index) => text.slice(0, index).split('\n').length;

/** 括号配对取 openParen 起的完整实参表文本（跨行照常） */
function argTextOf(buf, openParen) {
  let depth = 0;
  for (let i = openParen; i < buf.length; i++) {
    const c = buf[i];
    if (c === '(') depth += 1;
    if (c === ')') {
      depth -= 1;
      if (depth === 0) return buf.slice(openParen + 1, i);
    }
  }
  return '';
}

/** 顶层逗号切分（结构性括号要躲；字符串已在 codeOnly blank 化，full 版本带引号但引号内逗号需躲引号） */
function splitArgs(argText) {
  const parts = [];
  let depth = 0;
  let quote = null;
  let start = 0;
  for (let i = 0; i < argText.length; i++) {
    const c = argText[i];
    if (quote) {
      if (c === '\\') { i += 1; continue; }
      if (c === quote) quote = null;
      continue;
    }
    if (c === '\'' || c === '"' || c === '`') { quote = c; continue; }
    if (c === '(' || c === '[' || c === '{') depth += 1;
    if (c === ')' || c === ']' || c === '}') depth -= 1;
    if (c === ',' && depth === 0) {
      parts.push(argText.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(argText.slice(start));
  return parts.map((p) => p.trim()).filter(Boolean);
}
/**
 * 标识符回查：从 pos 往文件头找该标识符**最近一次**赋值/定义的右侧文本（codeOnly 上做）。
 * 覆盖 const/let/var/function/参数重赋值；右侧引用的标识符再递归回查（深度 2）。
 */
function traceAssign(buf, name, pos, depth) {
  const maxDepth = depth === undefined ? 2 : depth;
  if (maxDepth <= 0 || !name) return '';
  const esc = name.replace(/\$/g, '\\$');
  const re = new RegExp('(?:const|let|var)\\s+' + esc + '\\s*=|function\\s+' + esc + '\\s*\\(|\\b' + esc + '\\s*=[^=]', 'g');
  let last = null;
  let m;
  while ((m = re.exec(buf)) !== null) {
    if (m.index >= pos) break;
    last = m;
  }
  if (!last) return '';
  const eqOff = buf.indexOf('=', last.index);
  if (eqOff === -1) return '';
  const from = eqOff + 1;
  let depth2 = 0;
  let end = from;
  for (let i = from; i < Math.min(buf.length, from + 500); i++) {
    const c = buf[i];
    if (c === '(' || c === '[' || c === '{') depth2 += 1;
    if (c === ')' || c === ']' || c === '}') {
      if (depth2 === 0) break;
      depth2 -= 1;
    }
    if (depth2 === 0 && c === ';') { end = i; break; }
    end = i + 1;
  }
  const text = buf.slice(from, end);
  const refs = text.match(/[A-Za-z_$][\w$]*/g) || [];
  let extra = '';
  for (const r of refs) {
    if (r === name) continue;
    const t = traceAssign(buf, r, last.index, maxDepth - 1);
    if (t) extra += '\n' + t;
  }
  return text + extra;
}

/** 全文 writeFileSync( 的第 2 实参（内容表达式，full 缓冲：模板串里的自律行在这里才看得见） */
function writeFileSyncContents(fullBuf) {
  const out = [];
  const re = /writeFileSync\s*\(/g;
  let m;
  while ((m = re.exec(fullBuf)) !== null) {
    const argText = argTextOf(fullBuf, m.index + m[0].length - 1);
    const args = splitArgs(argText);
    if (args.length >= 2) out.push(args[1]);
  }
  return out;
}

/** 全文 .push( 的实参（full 缓冲：push('-EncodedCommand', encoded) 在这里） */
function pushArgsFull(fullBuf) {
  const out = [];
  const re = /\.\s*push\s*\(/g;
  let m;
  while ((m = re.exec(fullBuf)) !== null) {
    const argText = argTextOf(fullBuf, m.index + m[0].length - 1);
    if (argText) out.push(argText);
  }
  return out;
}

/** 文本里抠 .ps1 相对路径字面量 */
function ps1PathsOf(text) {
  const out = [];
  const re = /['"]([^'"\n]*?\.ps1)['"]/g;
  let m;
  while ((m = re.exec(text)) !== null) out.push(m[1]);
  return out;
}

/** 读仓内 .ps1 验 PIN（文件级豁免判据；读不到 ⇒ false，fail-closed） */
function ps1Pinned(relPath) {
  try {
    return PIN_RE.test(readText(path.resolve(ROOT, relPath)));
  } catch (e) {
    return false;
  }
}

/* ------------------------------------------------------------------ 主扫描 */

/**
 * 扫一个测试文件，返回裸调用清单 [{ line, args }]。
 * 判定流程：codeOnly 找调用点（注释/字符串里的不响）→ full 取实参表内容判首参 →
 * 组回查池验 PIN → -File 载体文件级验钉。
 */
function scanFile(src) {
  // 4 型豁免：probeSpawn 自身守护的必红腿（头注明说是设计目的）
  if (/require\s*\(\s*['"]\.\/probeSpawn\.js['"]\s*\)/.test(src)) return [];

  const code = codeBuffer(src);
  const full = src;
  // 全文共享的豁免素材：writeFileSync 内容表达式与 push 实参（full）
  const wContents = writeFileSyncContents(full).join('\n');
  const pArgs = pushArgsFull(full).join('\n');
  // 全文 .ps1 字面量 → 是否已钉（逐文件缓存）
  const ps1Pins = {};
  for (const p of ps1PathsOf(full)) {
    if (!(p in ps1Pins)) ps1Pins[p] = ps1Pinned(p);
  }

  const out = [];
  const re = /execFileSync\s*\(/g;
  let m;
  while ((m = re.exec(code)) !== null) {
    const line = lineAt(code, m.index);
    const openParen = m.index + m[0].length - 1;
    const argCode = argTextOf(code, openParen);
    if (!argCode) continue;
    const argFull = argTextOf(full, openParen);
    const argsFull = splitArgs(argFull);
    if (argsFull.length === 0) continue;

    // ---- 首参判 pwsh 系 ----
    const first = argsFull[0];
    let isPwsh = false;
    const lit = first.match(/^['"]([^'"]+)['"]$/);
    if (lit && /^(pwsh|powershell|powershell\.exe)$/.test(lit[1])) isPwsh = true;
    if (/^powershellExe\s*\(\s*\)$/.test(first.replace(/\s+/g, ''))) isPwsh = true;
    if (!isPwsh) {
      const id = first.match(/^[A-Za-z_$][\w$]*$/);
      if (id) {
        // 别名/变量：回查最近赋值，赋值右侧以 pwsh 系字面量或 powershellExe() 开头 ⇒ pwsh 系
        const t = traceAssign(code, id[0], m.index, 3);
        if (/^\s*['"](?:pwsh|powershell|powershell\.exe)['"]|^\s*powershellExe\s*\(\s*\)/.test(t)) isPwsh = true;
      }
    }
    if (!isPwsh) continue;

    // ---- 回查池（PIN 证据池）----
    let pool = argFull + '\n' + argCode;
    // 实参表上的标识符（codeOnly 回查；exclude 'File' 之类不必要）
    for (const a of splitArgs(argCode)) {
      const idents = a.match(/[A-Za-z_$][\w$]*/g) || [];
      for (const id of idents) {
        const t = traceAssign(code, id, m.index, 3);
        if (t) pool += '\n' + t;
      }
    }
    // 1 型 + 已钉副本：writeFileSync 内容（含模板串自律行）
    pool += '\n' + wContents;
    // 2 型：push 实参
    pool += '\n' + pArgs;
    // 豁免素材里出现的标识符在 **full 缓冲**上回查：1 型的自律行藏在字符串字面量里
    // （codeOnly 把字符串 blank 掉了，code 回查看不见），2 型的 PIN 证据在 push 实参指向的
    // 赋值行（encoded = encodeCommand(script)）上。full 不 blank ⇒ 两型一次覆盖。
    // 只用于豁免判定，过度收集无害（多拼几行不改变 PIN_RE 的判定面）。
    const waiveExprs = wContents + '\n' + pArgs;
    for (const id of new Set(waiveExprs.match(/[A-Za-z_$][\w$]*/g) || [])) {
      // 锚在调用点：同名变量可能定义多次（hxLaunchDetachBehavior 的 body :126 harness 模板 /
      // :214 另一用例），取「调用点前最近赋值」才对 —— 用全文最后赋值会豁免错位。
      const t = traceAssign(full, id, m.index, 3);
      if (t) pool += '\n' + t;
    }

    if (PIN_RE.test(pool)) continue;

    // ---- 3 型：-File 载体文件级验钉 ----
    // .ps1 字面量常藏在变量链里（args = H.gateArgs(path.join(H.ROOT, SCRIPT_REL), …)，
    // SCRIPT_REL = 'scripts/mp-weixin-check.ps1'）—— codeOnly 回查把字符串 blank 掉 ⇒
    // 只扫 argFull 拿不到。对实参表标识符在 full 上再回查一轮，从回查文本里解析 .ps1。
    let fullTrace = '';
    for (const a of splitArgs(argFull)) {
      const idents = a.match(/[A-Za-z_$][\w$]*/g) || [];
      for (const id of idents) {
        // 深度 4：157 行形态 args→base→SCRIPT→SCRIPT_REL 要四层才落到 .ps1 字面量
        const t = traceAssign(full, id, m.index, 4);
        if (t) fullTrace += '\n' + t;
      }
    }
    const ps1s = ps1PathsOf(argFull + '\n' + fullTrace);
    let waivedByFile = false;
    for (const p of ps1s) {
      if (ps1Pins[p]) { waivedByFile = true; break; }
    }
    if (waivedByFile) continue;

    out.push({ line, args: argFull.replace(/\s+/g, ' ').slice(0, 120) });
  }
  return out;
}

/** 全仓测试文件收集（点目录跳；与先例 collectUvue 同款纪律） */
function collectTests(dir, acc) {
  const list = acc || [];
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (e) {
    return list;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name.startsWith('.')) continue;
      if (e.name === 'node_modules' || e.name === 'uni_modules') continue;
      collectTests(full, list);
    } else if (e.name.endsWith('.test.js')) {
      list.push(full);
    }
  }
  return list;
}

const FILES = collectTests(UTILS).map((p) => path.relative(UTILS, p).replace(/\\/g, '/')).sort();
const readSource = (rel) => readText(path.join(UTILS, rel));

/* ------------------------------------------------------------------ 样本入口 */

// 本文件也在扫描面里 ⇒ 样本与违例文案里的调用形态一律字符串拼接，避免自扫描命中。
const BARE = 'exec' + 'FileSync';
const PWSH = 'pw' + 'sh';
const scanSample = (sampleSrc) => scanFile(sampleSrc);
/* ------------------------------------------------------------------ 测试 */

describe('裸 pwsh 调用防回归棘轮（#2256）', () => {
  /* ---------------- ① 违规样本必须判红（防空跑假绿） ---------------- */
  describe('① 检测器自检：注入的违规样本必须判红', () => {
    const bare = BARE; // 样本里用拼接后的变量名 ⇒ 生成出的文本就是真调用形态
    const mk = (lines) => lines.join('\n');

    test.each([
      [
        '字面量首参、无自律 ⇒ 判红',
        mk(['const r = ' + bare + "('" + PWSH + "', ['-Command', 'dir'], { encoding: 'utf8' });"]),
      ],
      [
        'powershellExe() 首参同样判红',
        mk([
          "const exe = powershellExe();",
          "const r = " + bare + "(exe, ['-Command', 'dir'], { encoding: 'utf8' });",
        ]),
      ],
      [
        '-EncodedCommand 但自律不进池 ⇒ 仍判红（写错位置的自律不算数）',
        mk([
          "const args = ['-NoProfile', '-NonInteractive'];",
          "const r = " + bare + "('" + PWSH + "', args, {});",
        ]),
      ],
      [
        '跨行调用也判（实参表跨行）',
        mk([
          'const r = ' + bare + '(',
          "  '" + PWSH + "',",
          "  ['-Command', 'dir'],",
          "  { encoding: 'utf8' }",
          ');',
        ]),
      ],
    ])('%s', (_name, sampleSrc) => {
      const v = scanSample(sampleSrc);
      expect(v).toHaveLength(1);
      // 违例文案逐文件:行点名（③ 节真实文件用同一格式）
      const msg = 'sample.test.js:' + v[0].line + ' 裸 pwsh 调用（' + bare + '(' + v[0].args + ')）：'
        + ' 首参 pwsh 系且回查池无 UTF-8 自律证据。修法：走 utils/probeSpawn.js 的'
        + ' withUtf8Discipline/encodeCommand，或 -File 指向已钉自律行的脚本（先例见 #2254 台账）。';
      expect(msg).toMatch(/裸 pwsh 调用/);
      expect(msg).toMatch(/修法/);
    });

    it('行号落在调用那一行（不是文件头）', () => {
      const pad = Array.from({ length: 7 }, () => '// pad').join('\n');
      const v = scanSample(pad + '\nconst r = ' + bare + "('" + PWSH + "', ['-Command', 'x'], {});");
      expect(v).toHaveLength(1);
      expect(v[0].line).toBe(8);
    });
  });

  /* ---------------- ② 合规样本必须判绿（防误报：四型豁免各自要能命中） ---------------- */
  describe('② 检测器自检：合规形态必须判绿', () => {
    const bare = BARE;
    const pwsh = PWSH;
    const mk = (lines) => lines.join('\n');

    it('1 型：writeFileSync 模板体里带自律行 ⇒ 豁免', () => {
      const body = mk([
        "$ErrorActionPreference = 'Stop'",
        'try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }',
        '$OutputEncoding = [System.Text.Encoding]::UTF8',
      ]);
      const sample = mk([
        "const harness = 'C:/tmp/h.ps1';",
        'const body = ' + JSON.stringify(body) + ';',
        "fs.writeFileSync(harness, body, 'utf8');",
        "const r = " + bare + "('" + pwsh + "', ['-File', harness], { encoding: 'utf8' });",
      ]);
      expect(scanSample(sample)).toEqual([]);
    });

    it('2 型：encodeCommand 装填局部数组 ⇒ 豁免', () => {
      const sample = mk([
        'const encoded = encodeCommand(script);',
        "const psArgs = ['-NoProfile', '-NonInteractive'];",
        "psArgs.push('-EncodedCommand', encoded);",
        "const r = " + bare + "('" + pwsh + "', psArgs, { encoding: 'utf8' });",
      ]);
      expect(scanSample(sample)).toEqual([]);
    });

    it('3 型：-File 指向仓内已钉脚本 ⇒ 豁免（载体没钉 ⇒ 该红，防豁免判据静默失效）', () => {
      const pinnedTarget = ps1Pinned('scripts/dev-finish.ps1');
      const sample = "const r = " + bare + "('" + pwsh + "', ['-File', 'scripts/dev-finish.ps1'], {});";
      if (pinnedTarget) {
        expect(scanSample(sample)).toEqual([]);
      } else {
        // 载体未钉 ⇒ 豁免判据必须放它过不判 —— 3 型豁免依赖「载体真的有自律」这一事实
        expect(scanSample(sample)).toHaveLength(1);
      }
    });

    it('4 型：require probeSpawn.js 的守护文件整文件豁免', () => {
      const sample = mk([
        "const { UTF8_DISCIPLINE } = require('./probeSpawn.js');",
        "const r = " + bare + "('" + pwsh + "', ['-Command', 'x'], {});",
      ]);
      expect(scanSample(sample)).toEqual([]);
    });

    it('自律直接写在实参表上 ⇒ 豁免', () => {
      const sample = "const r = " + bare + "('" + pwsh + "', ['-EncodedCommand', encodeCommand(body)], {});";
      expect(scanSample(sample)).toEqual([]);
    });

    it('注释与字符串里的调用不算（双缓冲防空红）', () => {
      const sample = mk([
        '// 注释提到 ' + bare + "('" + pwsh + "', …) 不算",
        "const label = 'call " + bare + " here';",
        'const r = otherFn(1, 2);',
      ]);
      expect(scanSample(sample)).toEqual([]);
    });

    it('首参不是 pwsh 系 ⇒ 不判（本守护只锁 C-1 台账口径）', () => {
      const sample = "const r = " + bare + "('node', ['x.js'], {});";
      expect(scanSample(sample)).toEqual([]);
    });
  });

  /* ---------------- ③ 真实文件（全仓 utils 下的 .test.js） ---------------- */
  describe('③ 真实文件零命中（基线 0，只减不增）', () => {
    const scanned = FILES.map((file) => ({ file, violations: scanFile(readSource(file)) }));

    it('扫描面自检：测试文件数必须真的在工作（防空跑假绿）', () => {
      expect(FILES.length).toBeGreaterThan(100);
    });

    it('扫描面自检：全仓 pwsh 系调用点必须 ≥ 已知下界（解析塌掉 ⇒ 「零命中」恒真假绿）', () => {
      // C-3 收口后现测 79 个 pwsh 调用点；execFileSync 总数更多（还有非 pwsh 的）。锁保守下界。
      let total = 0;
      for (const file of FILES) {
        const code = codeBuffer(readSource(file));
        const re = /execFileSync\s*\(/g;
        let m;
        while ((m = re.exec(code)) !== null) total += 1;
      }
      expect(total).toBeGreaterThan(30);
    });

    it('全仓零裸调用（新测试手写裸 pwsh ⇒ 这里逐文件:行点名判红）', () => {
      const found = scanned.filter((s) => s.violations.length > 0).map((s) =>
        s.file + '\n  ' + s.violations.map((v) =>
          ':' + v.line + ' 裸 pwsh 调用 ' + BARE + '(' + v.args + ') —— 修法：走 utils/probeSpawn.js'
            + ' 的 withUtf8Discipline/encodeCommand，或 -File 指向已钉自律行的脚本'
        ).join('\n  ')
      );
      expect(found.join('\n')).toBe('');
    });
  });
});