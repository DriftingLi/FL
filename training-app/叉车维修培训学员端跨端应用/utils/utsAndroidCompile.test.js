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
});
