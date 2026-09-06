/**
 * uni-app x App-Android（Kotlin）编译静态守护
 *
 * 背景：HBuilderX 5.24 无线调试全量编译 kotlin 失败，五类根因 H5 端不报、
 * 只会在 App 端暴露（error 编号对应 DCloud 编译器已知问题文档）：
 *   A. <script setup lang="uts"> 顶层函数编译为 Kotlin 局部函数，无提升——
 *      调用出现在定义之前即 error18「找不到名称」（error13 同源）
 *   B. uni_modules interface.uts / types 中定义的类型，使用文件未显式
 *      import——类型不参与该平台编译即 error3/18
 *   C. Kotlin toByteArray/toString 只有 Charset 重载，传字符串字面量即
 *      error17（String ≠ Charset）
 *   D. interface.uts 中无函数体的 ambient 函数声明，被平台文件 value-import
 *      后整体编译时因无实现而失败
 *   E. 原生互操作文件里无注解的数字常量——UTS 推断为 number（Kotlin
 *      Number），流入 Java/Kotlin 的 Int 形参即 error17，须显式注解 Int
 *
 * 设计：每类扫描都是纯函数，先各跑一个「注入违规」自检用例证明检测有效
 * （防止空跑假绿），再对全工程源码跑断言零命中（真正守护）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCAN_DIRS = ['pages', 'components', 'utils', 'composables', 'api', 'stores', 'constants', 'types', 'uni_modules'];
const SKIP = new Set(['node_modules', 'unpackage', '.git', 'dist', 'hybrid']);

function walk(dir, out = []) {
  if (!fs.existsSync(dir)) return out;
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name.startsWith('.') || SKIP.has(e.name)) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(uvue|uts)$/.test(e.name)) out.push(p);
  }
  return out;
}

const ALL_FILES = [];
for (const d of SCAN_DIRS) walk(path.join(ROOT, d), ALL_FILES);

/** 抹掉字符串与注释内容（保留行结构），避免文本误匹配 */
function blank(src) {
  const chars = src.split('');
  let i = 0;
  let state = 'code';
  while (i < chars.length) {
    const c = src[i];
    const c2 = src.slice(i, i + 2);
    if (state === 'code') {
      if (c2 === '//') { state = 'line'; chars[i] = ' '; chars[i + 1] = ' '; i += 2; continue; }
      if (c2 === '/*') { state = 'block'; chars[i] = ' '; chars[i + 1] = ' '; i += 2; continue; }
      if (c === '"' || c === "'" || c === '`') { state = c === '`' ? 'tpl' : c === '"' ? 'dq' : 'sq'; chars[i] = ' '; i++; continue; }
      i++;
    } else if (state === 'line') {
      if (c === '\n') state = 'code'; else if (c !== '\r') chars[i] = ' ';
      i++;
    } else if (state === 'block') {
      if (c2 === '*/') { chars[i] = ' '; chars[i + 1] = ' '; state = 'code'; i += 2; continue; }
      if (c !== '\n' && c !== '\r') chars[i] = ' ';
      i++;
    } else {
      const q = state === 'tpl' ? '`' : state === 'dq' ? '"' : "'";
      if (c === '\\') { chars[i] = ' '; if (chars[i + 1] !== '\n') chars[i + 1] = ' '; i += 2; continue; }
      if (c === q) { state = 'code'; chars[i] = ' '; i++; continue; }
      if (c !== '\n' && c !== '\r') chars[i] = ' ';
      i++;
    }
  }
  return chars.join('');
}

function scriptBlocks(text) {
  const out = [];
  const re = /<script\b[^>]*>([\s\S]*?)<\/script>/g;
  let m;
  while ((m = re.exec(text)) !== null) {
    out.push({ code: m[1], tag: m[0].slice(0, m[0].indexOf('>') + 1) });
  }
  return out;
}

/** A：script setup 顶层函数「调用早于定义」 */
function scanUseBeforeDefine(scriptCode) {
  const lines = blank(scriptCode).split('\n');
  let depth = 0;
  const defs = [];
  for (let li = 0; li < lines.length; li++) {
    const at = depth;
    for (const ch of lines[li]) { if (ch === '{') depth++; else if (ch === '}') depth--; }
    if (at === 0) {
      const m = /(?:^|\s)(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(/.exec(lines[li]);
      if (m) defs.push({ name: m[1], line: li });
    }
  }
  const hits = [];
  for (const d of defs) {
    const callRe = new RegExp('(?<![\\w$.])' + d.name + '\\s*\\(');
    for (let li = 0; li < d.line; li++) {
      if (callRe.test(lines[li])) { hits.push(d.name); break; }
    }
  }
  return hits;
}

/** C：字符串 charset 传给 Kotlin Charset 重载 */
function scanCharsetLiteral(text) {
  const hits = [];
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/\.toByteArray\(\s*['"][^'"]+['"]\s*\)/.test(lines[i]) || /\.toString\(\s*['"][^'"]+['"]\s*\)/.test(lines[i])) {
      hits.push(lines[i].trim());
    }
  }
  return hits;
}

/** D：interface.uts 里无函数体的 ambient 声明 */
function scanAmbientFunction(cleanText) {
  const hits = [];
  const lines = cleanText.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/^\s*export\s+(?:async\s+)?function\s+\w+[^{]*$/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** E：原生互操作文件里无注解的数字常量（推断为 number，流入 Int 形参即 error17） */
function scanUntypedNumericConst(cleanText) {
  const hits = [];
  const lines = cleanText.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/^\s*(?:export\s+)?(?:const|let)\s+[A-Za-z_$][\w$]*\s*=\s*-?\d+(\.\d+)?\s*;?\s*$/.test(lines[i])) {
      hits.push(lines[i].trim());
    }
  }
  return hits;
}

/** B：跨文件 export type/interface 未 import 就引用（需全工程导出表） */
function buildExportedTypeMap() {
  const exported = new Map();
  for (const file of ALL_FILES.filter((f) => f.endsWith('.uts'))) {
    const clean = blank(fs.readFileSync(file, 'utf8'));
    let m;
    const re = /export\s+(?:type|interface)\s+([A-Za-z_$][\w$]*)/g;
    while ((m = re.exec(clean)) !== null) {
      if (!exported.has(m[1])) exported.set(m[1], file);
    }
  }
  return exported;
}

/** 全工程代码单元：.uts 取整文件，.uvue 取 script 块（模板表达式不走 Kotlin 严格检查，不参与脚本规则扫描） */
function allCodeUnits() {
  const units = [];
  for (const file of ALL_FILES) {
    const text = fs.readFileSync(file, 'utf8');
    if (file.endsWith('.uts')) units.push({ file, code: text });
    else for (const s of scriptBlocks(text)) units.push({ file, code: s.code });
  }
  return units;
}

/** 顶层逗号切分（深度感知 () [] {}，字符串与注释已被 blank 抹除） */
function splitTopLevel(text) {
  const parts = [];
  let depth = 0;
  let cur = '';
  for (const c of text) {
    if ('([{'.includes(c)) { depth++; cur += c; continue; }
    if (')]}'.includes(c)) { depth--; cur += c; continue; }
    if (c === ',' && depth === 0) { parts.push(cur); cur = ''; continue; }
    cur += c;
  }
  if (cur.trim() !== '') parts.push(cur);
  return parts;
}

/** 形参串 → 必选形参数；`=` 默认值或 `?` 可选记为可选；含 `...` 变长返回 null（无法静态计数，跳过执法） */
function parseRequiredParams(paramText) {
  if (paramText.trim() === '') return 0;
  let required = 0;
  for (const p of splitTopLevel(paramText)) {
    if (p.includes('...')) return null;
    if (/[=]/.test(p) || /\?\s*:/.test(p)) continue;
    required++;
  }
  return required;
}

/** 全局函数签名表：.uts 的 function 定义与「name : (params) =>」箭头属性（composable 返回面）、.uvue script 局部 function；同名取最小必选数 */
function buildFunctionSignatureMap() {
  const table = new Map();
  const consider = (name, paramText) => {
    const required = parseRequiredParams(paramText);
    if (required === null) return;
    const prev = table.get(name);
    table.set(name, prev === undefined ? required : Math.min(prev, required));
  };
  const fnRe = /(?:^|\n)\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(([^)]*)\)/g;
  const arrowRe = /(?<![\w$])([A-Za-z_$][\w$]*)\s*:\s*\(([^)]*)\)\s*=>/g;
  let m;
  for (const file of ALL_FILES.filter((f) => f.endsWith('.uts'))) {
    const clean = blank(fs.readFileSync(file, 'utf8'));
    while ((m = fnRe.exec(clean)) !== null) consider(m[1], m[2]);
    while ((m = arrowRe.exec(clean)) !== null) consider(m[1], m[2]);
  }
  for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
    for (const s of scriptBlocks(fs.readFileSync(file, 'utf8'))) {
      const clean = blank(s.code);
      while ((m = fnRe.exec(clean)) !== null) consider(m[1], m[2]);
    }
  }
  return table;
}

/** 实参数统计：从 openIdx 的 `(` 做深度游走，depth=1 的顶层逗号计数；未闭合返回 null */
function countCallArgs(clean, openIdx) {
  let depth = 0;
  let args = 1;
  for (let i = openIdx; i < clean.length; i++) {
    const c = clean[i];
    if ('([{'.includes(c)) { depth++; continue; }
    if (')]}'.includes(c)) { depth--; if (depth === 0) return args; continue; }
    if (c === ',' && depth === 1) args++;
  }
  return null;
}

/** J：调用实参少于必选形参（Kotlin error: No value passed for parameter）。
 *  函数定义行与方法调用（前置 .）不报；变长签名不入表不执法 */
function scanCallArity(code, sigTable) {
  const clean = blank(code);
  const hits = [];
  const callRe = /(?<![\w$.])([A-Za-z_$][\w$]*)\s*\(/g;
  let m;
  while ((m = callRe.exec(clean)) !== null) {
    const name = m[1];
    const required = sigTable.get(name);
    if (required === undefined || required === 0) continue;
    const before = clean.slice(Math.max(0, m.index - 12), m.index);
    if (/function\s+$/.test(before)) continue;
    const args = countCallArgs(clean, m.index + m[0].length - 1);
    if (args !== null && args < required) hits.push(`${name}(...) 实参 ${args} < 必选 ${required}`);
  }
  return hits;
}

/** 全工程类型字段表：.uts 的 export type / interface / type 声明体，取深度 1 的字段名集合（嵌套内层不取，宁漏勿误） */
function buildTypeFieldMap() {
  const fields = new Map();
  const headRe = /(?:export\s+)?(?:type|interface)\s+([A-Za-z_$][\w$]*)[^{]*\{/g;
  let m;
  for (const file of ALL_FILES.filter((f) => f.endsWith('.uts'))) {
    const clean = blank(fs.readFileSync(file, 'utf8'));
    while ((m = headRe.exec(clean)) !== null) {
      let depth = 0;
      let body = '';
      for (let i = m.index + m[0].length - 1; i < clean.length; i++) {
        const c = clean[i];
        if (c === '{') { depth++; if (depth === 1) continue; }
        else if (c === '}') { depth--; if (depth === 0) break; }
        if (depth >= 1) body += c;
      }
      const set = new Set();
      depth = 0;
      for (const rawLine of body.split('\n')) {
        const fm = /^\s*([A-Za-z_$][\w$]*)\s*\??\s*:/.exec(rawLine);
        if (fm !== null && depth === 0) set.add(fm[1]);
        for (const c of rawLine) {
          if ('{(['.includes(c)) depth++;
          else if ('})]'.includes(c)) depth--;
        }
      }
      if (!fields.has(m[1])) fields.set(m[1], set);
    }
  }
  return fields;
}

/** L：类型化参数访问未声明字段（Kotlin error18: 找不到名称）。
 *  仅执法字段表可解析的单类型参数；可选链 ?. 与未知类型不扫；方法调用（后随括号）不报 */
function scanTypedParamFieldAccess(code, fieldMap) {
  const clean = blank(code);
  const hits = [];
  const fnRe = /(?:^|\n)\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(/g;
  let m;
  while ((m = fnRe.exec(clean)) !== null) {
    const openIdx = clean.indexOf('(', m.index + m[0].length - 1);
    if (openIdx === -1) continue;
    let depth = 0;
    let closeIdx = -1;
    for (let i = openIdx; i < clean.length; i++) {
      if ('([{'.includes(clean[i])) depth++;
      else if (')]}'.includes(clean[i])) { depth--; if (depth === 0) { closeIdx = i; break; } }
    }
    if (closeIdx === -1) continue;
    const tracked = [];
    for (const p of splitTopLevel(clean.slice(openIdx + 1, closeIdx))) {
      const pm = /^\s*([A-Za-z_$][\w$]*)\s*:\s*([A-Za-z_$][\w$]*)\s*$/.exec(p);
      if (pm === null) continue;
      const set = fieldMap.get(pm[2]);
      if (set !== undefined) tracked.push([pm[1], pm[2], set]);
    }
    if (tracked.length === 0) continue;
    let bodyStart = -1;
    for (let i = closeIdx + 1; i < clean.length; i++) {
      if (clean[i] === '{') { bodyStart = i; break; }
      if (clean[i] === ';') break;
    }
    if (bodyStart === -1) continue;
    depth = 0;
    let body = '';
    for (let i = bodyStart; i < clean.length; i++) {
      const c = clean[i];
      if (c === '{') { depth++; if (depth === 1) continue; }
      else if (c === '}') { depth--; if (depth === 0) break; }
      if (depth >= 1) body += c;
    }
    const accessRe = /(?<![\w$])([A-Za-z_$][\w$]*)\s*\.\s*([A-Za-z_$][\w$]*)\b(?!\s*\()/g;
    while ((m = accessRe.exec(body)) !== null) {
      for (const [pname, typeName, set] of tracked) {
        if (m[1] === pname && !set.has(m[2])) hits.push(`参数 ${pname} : ${typeName} 访问未声明字段 ${m[2]}`);
      }
    }
  }
  return hits;
}

// ── 注入违规自检（证明检测函数有效，不是空跑假绿）────────────────────────
describe('守护自检：检测逻辑对已知违规样本必须报出', () => {
  it('A 能报出 script setup 调用早于定义', () => {
    const sample = `
<template><view/></template>
<script setup lang="uts">
  function caller() : void { callee() }
  function callee() : void { }
</script>`;
    const code = scriptBlocks(sample)[0].code;
    expect(scanUseBeforeDefine(code)).toContain('callee');
  });

  it('A 不误报先定义后使用（对照组）', () => {
    const sample = `
<template><view/></template>
<script setup lang="uts">
  function callee() : void { }
  function caller() : void { callee() }
</script>`;
    const code = scriptBlocks(sample)[0].code;
    expect(scanUseBeforeDefine(code)).toEqual([]);
  });

  it('C 能报出 toByteArray("UTF-8")', () => {
    expect(scanCharsetLiteral('const x = value.toByteArray("UTF-8")')).toHaveLength(1);
    expect(scanCharsetLiteral('const x = value.toByteArray(Charset.forName("UTF-8"))')).toEqual([]);
  });

  it('D 能报出 ambient 函数声明', () => {
    expect(scanAmbientFunction('export function setSecureItem(key : string) : SecureSetResult')).toHaveLength(1);
    expect(scanAmbientFunction('export function setSecureItem(key : string) : SecureSetResult {')).toEqual([]);
  });

  it('E 能报出无注解数字常量', () => {
    expect(scanUntypedNumericConst('const GCM_TAG_BITS = 128;')).toHaveLength(1);
    expect(scanUntypedNumericConst('const GCM_TAG_BITS : Int = 128;')).toEqual([]);
    expect(scanUntypedNumericConst('const GCM_TAG_BITS : number = 128;')).toEqual([]);
  });

  it('B 检测表能识别导出类型（导出表非空）', () => {
    const map = buildExportedTypeMap();
    expect(map.has('SecureSetResult')).toBe(true);
    expect(map.has('StoredCredentials')).toBe(true);
  });

  it('J 能报出实参少于必选形参（对照组：实参齐全与方法调用不报）', () => {
    const table = new Map([['sendInput', 3]]);
    const code = 'function sendInput(a : string, b : string, c : string[]) : void {}\nsendInput("x", "y")';
    expect(scanCallArity(code, table)).toHaveLength(1);
    expect(scanCallArity('sendInput("x", "y", [])', table)).toEqual([]);
    expect(scanCallArity('obj.sendInput("x")', table)).toEqual([]);
  });

  it('J 函数定义行自身不计入调用（定义形参可少于全局表同名签名）', () => {
    const table = new Map([['helper', 2]]);
    const code = 'function helper(a : string) : void { }\nconst r = helper("only-one")';
    expect(scanCallArity(code, table)).toHaveLength(1);
  });

  it('L 能报出类型化参数访问未声明字段（对照组：已声明字段与未知类型不报）', () => {
    const fields = new Map([['ForumReply', new Set(['id', 'likes_count', 'liked'])]]);
    const code = 'function likes(r : ForumReply) : number {\n  return r.like_count\n}';
    expect(scanTypedParamFieldAccess(code, fields)).toHaveLength(1);
    expect(scanTypedParamFieldAccess('function likes(r : ForumReply) : number {\n  return r.likes_count\n}', fields)).toEqual([]);
    expect(scanTypedParamFieldAccess('function f(o : SomeUnknown) : void {\n  return o.whatever\n}', fields)).toEqual([]);
  });
});

// ── 全工程真实扫描（守护本体，回归即红）────────────────────────────────
describe('全工程守护：五类 Kotlin 编译地雷零命中', () => {
  const scopeInfo = { uvue: ALL_FILES.filter((f) => f.endsWith('.uvue')).length, uts: ALL_FILES.filter((f) => f.endsWith('.uts')).length };

  it('扫描范围非空（.uvue/.uts 全覆盖）', () => {
    expect(scopeInfo.uvue).toBeGreaterThan(50);
    expect(scopeInfo.uts).toBeGreaterThan(20);
  });

  it('A：无 script setup 顶层函数前向引用', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
      const text = fs.readFileSync(file, 'utf8');
      for (const s of scriptBlocks(text)) {
        if (!/setup/.test(s.tag)) continue;
        for (const name of scanUseBeforeDefine(s.code)) {
          violations.push(`${path.relative(ROOT, file)}: "${name}()" 调用早于定义`);
        }
      }
    }
    expect(violations).toEqual([]);
  });

  it('B：无未 import 的跨文件类型引用', () => {
    const exported = buildExportedTypeMap();
    const violations = [];
    for (const file of ALL_FILES) {
      const isUts = file.endsWith('.uts');
      const text = fs.readFileSync(file, 'utf8');
      const blocks = isUts ? [{ code: text }] : scriptBlocks(text);
      for (const s of blocks) {
        const clean = blank(s.code);
        const lines = clean.split('\n');
        for (const [name, defFile] of exported) {
          if (path.resolve(file) === path.resolve(defFile)) continue;
          if (new RegExp('(?:^|\\n)[ \\t]*(?:export\\s+)?(?:type|interface)\\s+' + name + '\\b').test(clean)) continue;
          if (new RegExp('import\\s+(?:type\\s+)?\\{[^}]*\\b' + name + '\\b[^}]*\\}', 's').test(clean)) continue;
          if (new RegExp('import\\s+(?:type\\s+)?' + name + '\\b').test(clean)) continue;
          const useRe = new RegExp('(?<![\\w$])' + name + '(?![\\w$])');
          if (lines.some((ln) => useRe.test(ln))) {
            violations.push(`${path.relative(ROOT, file)}: "${name}" 未 import（导出于 ${path.relative(ROOT, defFile)}）`);
          }
        }
      }
    }
    expect(violations).toEqual([]);
  });

  it('C：无字符串 charset 传参', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uts'))) {
      for (const h of scanCharsetLiteral(fs.readFileSync(file, 'utf8'))) {
        violations.push(`${path.relative(ROOT, file)}: ${h}`);
      }
    }
    expect(violations).toEqual([]);
  });

  it('D：interface.uts 无 ambient 函数声明', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('interface.uts'))) {
      for (const h of scanAmbientFunction(blank(fs.readFileSync(file, 'utf8')))) {
        violations.push(`${path.relative(ROOT, file)}: ${h}`);
      }
    }
    expect(violations).toEqual([]);
  });

  it('E：原生互操作文件无未注解数字常量（number 流入 Int 形参 → error17）', () => {
    const violations = [];
    const isNativeFile = (f) => f.includes('utssdk') && (f.includes('app-android') || f.includes('app-ios') || f.includes('app-harmony'));
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uts') && isNativeFile(f))) {
      for (const h of scanUntypedNumericConst(blank(fs.readFileSync(file, 'utf8')))) {
        violations.push(`${path.relative(ROOT, file)}: ${h}`);
      }
    }
    expect(violations).toEqual([]);
  });
  it('J：无调用实参少于必选形参（Kotlin error: No value passed for parameter）', () => {
    const violations = [];
    const table = buildFunctionSignatureMap();
    for (const u of allCodeUnits()) {
      for (const h of scanCallArity(u.code, table)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });
  it('L：类型化参数无未声明字段访问（Kotlin error18: 找不到名称）', () => {
    const violations = [];
    const fieldMap = buildTypeFieldMap();
    for (const u of allCodeUnits()) {
      for (const h of scanTypedParamFieldAccess(u.code, fieldMap)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });
});
