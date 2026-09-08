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
 *
 * 2026-09 增补（refactor epic #638 T01 + 编译门红修）：AGENTS.md 坑位表机械化规则 F–L——
 *   F. 裸 String() 强转（Kotlin 无此重载，error17；app-ios Swift 互操作文件除外）；
 *   G. undefined 字面量；H. catch 参数显式 : any；
 *   I. `: any` 注解参数访问 .detail（类型化事件对象自带 detail 属合法，按组合判定）
 *   J. 调用实参少于必选形参（No value passed for parameter；函数定义行与方法调用不报）
 *   K. 模板 v-for 别名字段越界（error18；标签游走做块级作用域归属，页面本地类型声明优先于全局表）
 *   L. 类型化参数访问未声明字段（error18；script 侧，字段表可解析的单类型参数）
 *   M. *Mapped 出口传 build* 具名函数引用（Kotlin error17；箭头包裹才合法）
 *   N. as unknown as 双重强转（Kotlin error18 类型污染；对象 prop 走工厂默认值）
 *   O. 模板 {{ 裸函数名 }} 插值（uni-app x 不自动调用无参 function，静默渲染源码）
 *   P. 可选对象 prop 成员直读（Kotlin error18 nullable 接收者；扁平原始 props 或局部 val+判空）
 *   Q. ref<any> 类型擦除声明（Kotlin error18 any 无成员；组件实例引用需类型化）
 *   R. async 函数返回类型声明 : void（Kotlin 无法推断 UTSPromise 类型参数，级联编译错；须 Promise<void>）
 *   S. 模板直调 import 函数（uvue 模板 import 调用编译为 .invoke() = error18「找不到名称 invoke」；本地包装）
 * 存量违例走 GUARD_ALLOWLIST 豁免，由后续工单在各自范围清零（见常量注释）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCAN_DIRS = ['pages', 'components', 'utils', 'composables', 'api', 'stores', 'constants', 'types', 'uni_modules'];
const SKIP = new Set(['node_modules', 'unpackage', '.git', 'dist', 'hybrid']);

/**
 * 已知存量违例豁免（expand–contract 的 expand 侧，refactor epic #638 T01 引入）：
 * 规则上线时已存在的违例按「规则 → 文件」豁免，由后续工单在各自范围清零；
 * 全部清零后由 epic 收尾票（#654）删除本机制。
 * 仅存量不为零的规则入表（F/G/I 在 master 树上实测零存量，全量执法无豁免）；
 * 键 = 规则标识，值 = 豁免文件的相对路径集合
 */
const GUARD_ALLOWLIST = {
  H: new Set([
    'api/forum.uts',
    'api/checkin.uts',
    'pages/notifications/notifications.uvue',
  ]),
};

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

/** F：裸 String() 强转——Kotlin 无 String(x) 重载（error17），应为 x.toString() */
function scanBareStringCall(code) {
  const hits = [];
  const lines = blank(code).split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/(?<![A-Za-z0-9_$.])String\s*\(/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** G：undefined 字面量——UTS 空值统一 null，Kotlin 无 undefined（找不到名称） */
function scanUndefinedLiteral(code) {
  const hits = [];
  const lines = blank(code).split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/\bundefined\b/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** H：catch 参数显式注解 any——Kotlin 无非空 Any 收窄（error17），应 (e) 或 (e : any | null) */
function scanCatchAnyParam(code) {
  const hits = [];
  const lines = blank(code).split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/catch\s*\(\s*\(?\s*[A-Za-z_$][\w$]*\s*:\s*any\b(?!\s*\|)/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** I：`: any` 注解参数访问 .detail——Kotlin 非空 Any 无 detail 成员（Unresolved reference），应 as UTSJSONObject 后 ['detail'] 索引。
 *  类型化事件对象（如 InputEvent）自带 detail 属合法用法，故按「any 参数 + 同名接收者」组合判定而非裸扫 .detail */
function scanAnyParamDetailAccess(code) {
  const clean = blank(code);
  const anyParams = new Set();
  let m;
  const paramRe = /[(,]\s*([A-Za-z_$][\w$]*)\s*:\s*any\b(?!\s*\|)/g;
  while ((m = paramRe.exec(clean)) !== null) anyParams.add(m[1]);
  const hits = [];
  const accessRe = /(?<![\w$])([A-Za-z_$][\w$]*)\s*\.\s*detail\b/g;
  while ((m = accessRe.exec(clean)) !== null) {
    if (anyParams.has(m[1])) hits.push(m[1] + '.detail');
  }
  return hits;
}

/** M：getMapped/postMapped 出口传 build* 具名函数引用作 mapper
 *  ——UTS→Kotlin 不支持具名顶层函数直传高阶参数（error17: 参数类型不匹配 / Function invocation expected）；
 *  应包成箭头 `(data : UTSJSONObject) : T => buildX(data)`。函数型参数（如 map）透传合法，故仅锁 build* 命名。 */
function scanBareFnRefMapper(code) {
  const clean = blank(code);
  const hits = [];
  const callRe = /(?:get|post)Mapped\s*<[^>]*>\s*\(/g;
  let m;
  while ((m = callRe.exec(clean)) !== null) {
    const openIdx = clean.indexOf('(', m.index + m[0].length - 1);
    if (openIdx === -1) continue;
    let depth = 0;
    let closeIdx = -1;
    for (let i = openIdx; i < clean.length; i++) {
      const c = clean[i];
      if ('([{'.includes(c)) depth++;
      else if (')]}'.includes(c)) { depth--; if (depth === 0) { closeIdx = i; break; } }
    }
    if (closeIdx === -1) continue;
    const args = splitTopLevel(clean.slice(openIdx + 1, closeIdx));
    if (args.length === 0) continue;
    const last = args[args.length - 1].trim();
    if (/^build[A-Z][\w$]*$/.test(last)) hits.push('mapper 裸引用 ' + last);
  }
  return hits;
}

/** N：as unknown as 双重强转——Kotlin 侧把属性/参数类型污染为 unknown（error18 找不到成员），
 *  对象 prop 默认值应走工厂 `() => ({...} as T)`（先例 ai-chat sessions），非 null 强转 */
function scanUnknownDoubleCast(code) {
  const clean = blank(code);
  const hits = [];
  const lines = clean.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/\bas\s+unknown\s+as\b/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** O：模板 {{ 裸函数名 }} 插值——引用 script 里 function 声明却无括号调用。
 *  uni-app x 模板对无参 function 不会自动调用（渲染出函数源码，静默 bug）；
 *  应改 computed 或模板内 `fn(args)` 调用。仅扫 .uvue（需 template+script 同文件） */
function scanBareFnInterpolation(text) {
  const scriptIdx = text.indexOf('<script');
  if (scriptIdx === -1) return [];
  const template = text.slice(0, scriptIdx);
  const script = blank(text.slice(scriptIdx));
  const hits = [];
  const fns = new Set();
  let m;
  const fnRe = /(?:^|\n)\s*(?:export\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(/g;
  while ((m = fnRe.exec(script)) !== null) fns.add(m[1]);
  const interpRe = /\{\{\s*([A-Za-z_$][\w$]*)\s*\}\}/g;
  while ((m = interpRe.exec(template)) !== null) {
    if (fns.has(m[1])) hits.push(`模板裸插值 {{ ${m[1]} }}（function 未调用）`);
  }
  return hits;
}

/** P：可选对象类型 prop 的成员直读——Kotlin 对 nullable 接收者解析成员失败（error18 找不到名称）。
 *  合法姿势：原始类型 props 扁平传递（全仓先例），或局部 val + null 检查后访问（先例 redoVal）。
 *  数组型可选 prop 豁免（v-for 元素非空，先例 ai-chat sessions）。仅扫 .uvue */
function scanOptionalObjectPropAccess(text) {
  const scriptIdx = text.indexOf('<script');
  if (scriptIdx === -1) return [];
  const template = text.slice(0, scriptIdx);
  const script = blank(text.slice(scriptIdx));
  const dpIdx = script.indexOf('defineProps<');
  if (dpIdx === -1) return [];
  const blockEnd = script.indexOf('}>', dpIdx);
  if (blockEnd === -1) return [];
  const block = script.slice(dpIdx, blockEnd + 2);
  const objectProps = new Set();
  const optRe = /(\w+)\s*\?:\s*([A-Z][\w$]*)(\s*\[\s*\])?/g;
  let m;
  while ((m = optRe.exec(block)) !== null) {
    if (m[3]) continue;
    if (['UTSJSONObject', 'Any', 'Object', 'String', 'Number', 'Boolean'].includes(m[2])) continue;
    objectProps.add(m[1]);
  }
  if (objectProps.size === 0) return [];
  const hits = [];
  const zones = [['script', script], ['template', template]];
  for (const p of objectProps) {
    for (const [where, src] of zones) {
      const re = new RegExp('(?<![\\w$.])(?:props\\.)?' + p + '\\s*\\.\\s*[A-Za-z_$][\\w$]*', 'g');
      const seen = new Set();
      let mm;
      while ((mm = re.exec(src)) !== null) {
        if (!seen.has(mm[0])) { seen.add(mm[0]); hits.push(where + ' 直读可选对象 prop ' + mm[0]); }
      }
    }
  }
  return hits;
}

/** Q：ref<any> 类型擦除声明——Kotlin 侧 any 无成员（error18 找不到名称），
 *  组件实例引用应定义 interface 或用 defineExpose 的类型化包装，不裸 any。全树 0 存量，全量执法 */
function scanRefAnyDeclaration(code) {
  const clean = blank(code);
  const hits = [];
  const lines = clean.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/\bref\s*<[^>]*\bany\b/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** R：async 函数返回类型声明 : void——UTS 编译到 Kotlin 时 async 函数须返回 Promise<T>，
 *  写 : void 使返回类型成 UTSPromise<uninferred T>，报「Not enough information to infer
 *  type argument for 'T'」级联三连错（HBuilderX 5.24 编译门实测，wrong-questions loadStats）。
 *  同步函数 : void 合法不在靶内。全树 0 存量，全量执法 */
function scanAsyncVoidReturn(code) {
  const clean = blank(code);
  const hits = [];
  const lines = clean.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (/async\s+(?:function\s+[A-Za-z_$][\w$]*\s*)?\([^)]*\)\s*:\s*void\b/.test(lines[i])) hits.push(lines[i].trim());
  }
  return hits;
}

/** S：模板直调 import 函数——uvue 模板绑定走 $setup/$imports 解析，import 的顶层函数在模板里
 *  以函数调用形态出现时 Kotlin 编译成 .invoke()，报 error18「找不到名称 invoke」（#677 编译门实测：
 *  activity-topic-card 模板 {{ formatDateStr(createdDate) }}，import 自 utils/format）。
 *  本地 function / const 箭头在模板中直调合法，不在靶内；修复 = script 内本地薄包装。
 *  输入为 .uvue 原文（模板段 + script 段）。全树 0 存量，全量执法 */
function scanImportedFnTemplateCall(raw) {
  const hits = [];
  if (!raw.includes('<template>') || !raw.includes('</template>')) return hits;
  const tpl = raw.slice(raw.indexOf('<template>'), raw.lastIndexOf('</template>') + 11);
  const sm = /<script[^>]*>([\s\S]*?)<\/script>/.exec(raw);
  if (!sm) return hits;
  const script = sm[1];
  const names = new Set();
  for (const im of script.matchAll(/import\s+(?:type\s+)?\{([^}]*)\}\s+from/g)) {
    for (const part of im[1].split(',')) {
      let n = part.trim().replace(/^type\s+/, '');
      const asM = /\bas\s+([A-Za-z_$][\w$]*)\s*$/.exec(n);
      if (asM) n = asM[1];
      else n = n.split(/[\s]+/)[0];
      if (/^[A-Za-z_$][\w$]*$/.test(n)) names.add(n);
    }
  }
  for (const dm of script.matchAll(/import\s+([A-Za-z_$][\w$]*)\s+from/g)) names.add(dm[1]);
  for (const c of tpl.matchAll(/(?<![\w$.])([A-Za-z_$][\w$]*)\s*\(/g)) {
    if (names.has(c[1])) hits.push('模板直调 import 函数 ' + c[1] + '()');
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

/** 从类型头匹配处提取声明体的深度 1 字段名集合（嵌套内层不取——宁漏勿误） */
function extractTypeFieldSet(clean, headMatch) {
  const set = new Set();
  let depth = 0;
  let body = '';
  for (let i = headMatch.index + headMatch[0].length - 1; i < clean.length; i++) {
    const c = clean[i];
    if (c === '{') { depth++; if (depth === 1) continue; }
    else if (c === '}') { depth--; if (depth === 0) break; }
    if (depth >= 1) body += c;
  }
  depth = 0;
  for (const rawLine of body.split('\n')) {
    const startsAtZero = depth === 0;
    if (startsAtZero) {
      // 单行多字段（`a : X, b : Y` / `a : X; b : Y`）按深度 0 的逗号/分号切段逐段取
      const segs = [];
      let d = 0;
      let cur = '';
      for (const c of rawLine) {
        if ('([{'.includes(c)) d++;
        else if (')]}'.includes(c)) d--;
        if ((c === ',' || c === ';') && d === 0) { segs.push(cur); cur = ''; continue; }
        cur += c;
      }
      segs.push(cur);
      for (const seg of segs) {
        const fm = /^\s*([A-Za-z_$][\w$]*)\s*\??\s*:/.exec(seg);
        if (fm !== null) set.add(fm[1]);
      }
    }
    for (const c of rawLine) {
      if ('{(['.includes(c)) depth++;
      else if ('})]'.includes(c)) depth--;
    }
  }
  return set;
}

/** 全工程类型字段表：.uts 的 export type / interface / type 声明体（.uvue 本地类型由规则 K 按页覆盖） */
function buildTypeFieldMap() {
  const fields = new Map();
  const headRe = /(?:export\s+)?(?:type|interface)\s+([A-Za-z_$][\w$]*)[^{]*\{/g;
  let m;
  for (const file of ALL_FILES.filter((f) => f.endsWith('.uts'))) {
    const clean = blank(fs.readFileSync(file, 'utf8'));
    while ((m = headRe.exec(clean)) !== null) {
      if (!fields.has(m[1])) fields.set(m[1], extractTypeFieldSet(clean, m));
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

/** K：模板 v-for 别名访问未声明字段（Kotlin error18——模板属性访问同样走 Kotlin 类型检查）。
 *  标签游走维护 v-for 绑定栈做块级作用域归属（同名别名各归各块）；元素类型取 script 的
 *  `const src = ref<ET[]>` / `const src : ET[] =`；来源或类型不可解析则跳过（宁漏勿误） */
function scanTemplateFields(text, fieldMap) {
  const scriptIdx = text.indexOf('<script');
  if (scriptIdx === -1) return [];
  const template = text.slice(0, scriptIdx);
  const script = blank(text.slice(scriptIdx));
  // 页面本地 type/interface 声明优先于全局表（同名遮蔽场景，如 dashboard 本地 CourseItem）
  const effFields = new Map(fieldMap);
  const localHeadRe = /(?:^|\n)\s*(?:export\s+)?(?:type|interface)\s+([A-Za-z_$][\w$]*)[^{]*\{/g;
  let hm;
  while ((hm = localHeadRe.exec(script)) !== null) {
    effFields.set(hm[1], extractTypeFieldSet(script, hm));
  }
  const elemTypeOf = (src) => {
    let dm;
    let et = null;
    const declRefRe = new RegExp('const\\s+' + src + '\\s*=\\s*ref\\s*<\\s*([\\w$.]+)\\s*\\[\\s*\\]\\s*>');
    const declAnnotRe = new RegExp('const\\s+' + src + '\\s*:\\s*([\\w$.]+)\\s*\\[\\s*\\]\\s*=');
    if ((dm = declRefRe.exec(script)) !== null) et = dm[1];
    if (et === null && (dm = declAnnotRe.exec(script)) !== null) et = dm[1];
    return et;
  };
  const hits = [];
  const stack = [];
  const checkChunk = (chunk) => {
    for (const entry of stack) {
      if (entry.binding === null || entry.binding.fields === undefined) continue;
      const b = entry.binding;
      const accRe = new RegExp('(?<![\\w$])' + b.alias + '\\s*\\.\\s*([A-Za-z_$][\\w$]*)\\b(?!\\s*\\()', 'g');
      let am;
      while ((am = accRe.exec(chunk)) !== null) {
        if (!b.fields.has(am[1])) hits.push(`v-for 别名 ${b.alias} : ${b.type} 访问未声明字段 ${am[1]}`);
      }
    }
  };
  const tagRe = /<(\/?)([\w-]+)((?:[^>"']|"[^"]*"|'[^']*')*?)(\/?)>/g;
  let m;
  let lastEnd = 0;
  while ((m = tagRe.exec(template)) !== null) {
    checkChunk(template.slice(lastEnd, m.index));
    lastEnd = m.index + m[0].length;
    const isClosing = m[1] === '/';
    const isSelfClosing = m[4] === '/';
    const tag = m[2];
    const attrs = m[3] || '';
    if (!isClosing) {
      let binding = null;
      const vf = /v-for=(["'])\s*\(?\s*([A-Za-z_$][\w$]*)\s*(?:,\s*[^"']*?)?\s*\)?\s+in\s+([A-Za-z_$][\w$]*)\s*\1/.exec(attrs);
      if (vf !== null) {
        const et = elemTypeOf(vf[3]);
        if (et !== null) binding = { alias: vf[2], type: et, fields: effFields.get(et) };
      }
      stack.push({ tag, binding });
      checkChunk(attrs);
      if (isSelfClosing) stack.pop();
    } else {
      for (let i = stack.length - 1; i >= 0; i--) {
        if (stack[i].tag === tag) { stack.splice(i, 1); break; }
      }
    }
  }
  checkChunk(template.slice(lastEnd));
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

  it('F 能报出裸 String() 强转（对照组：toString 与标识符内 String 不报）', () => {
    expect(scanBareStringCall('const a = String(123)')).toHaveLength(1);
    expect(scanBareStringCall('const b = x.toString()')).toEqual([]);
    expect(scanBareStringCall('const c = "String(x) in string"')).toEqual([]);
    expect(scanBareStringCall('const d = UTSCString(x)')).toEqual([]);
  });

  it('G 能报出 undefined 字面量（对照组：null 与注释/字符串内的 undefined 不报）', () => {
    expect(scanUndefinedLiteral('let x = undefined')).toHaveLength(1);
    expect(scanUndefinedLiteral('let x = obj ?? undefined')).toHaveLength(1);
    expect(scanUndefinedLiteral('let x = null')).toEqual([]);
    expect(scanUndefinedLiteral('// 空值传 undefined 会编译失败')).toEqual([]);
    expect(scanUndefinedLiteral('const s = "undefined value"')).toEqual([]);
  });

  it('H 能报出 catch 参数显式 : any（对照组：无注解与 any | null 不报）', () => {
    expect(scanCatchAnyParam('.catch((e : any) => { fail(e) })')).toHaveLength(1);
    expect(scanCatchAnyParam('try { x } catch (err : any) { log(err) }')).toHaveLength(1);
    expect(scanCatchAnyParam('.catch((e) => { fail(e) })')).toEqual([]);
    expect(scanCatchAnyParam('.catch((e : any | null) => { fail(e) })')).toEqual([]);
  });

  it('I 能报出 : any 参数访问 .detail（对照组：类型化事件对象与其他成员不报）', () => {
    expect(scanAnyParamDetailAccess('onChange((e : any) => { const v = e.detail.value })')).toHaveLength(1);
    expect(scanAnyParamDetailAccess('function h(e : any) : void {\n  const v = e.detail\n}')).toHaveLength(1);
    expect(scanAnyParamDetailAccess('onInput((e) => { const v = e.detail.value })')).toEqual([]);
    expect(scanAnyParamDetailAccess('onChange((e : any) => { const v = e.value })')).toEqual([]);
    expect(scanAnyParamDetailAccess('.catch((err : any | null) => { log(err) })')).toEqual([]);
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

  it('K 能报出 v-for 别名访问未声明模板字段（对照组：已声明字段与未解析来源不报）', () => {
    const fields = new Map([['ForumTopic', new Set(['id', 'likes_count'])]]);
    const text = '<template><view v-for="item in topics">{{ item.like_count }} {{ item.likes_count }}</view></template><script setup lang="uts">\nconst topics = ref<ForumTopic[]>([])\n</script>';
    expect(scanTemplateFields(text, fields)).toHaveLength(1);
    const unknown = '<template><view v-for="x in mystuff">{{ x.whatever }}</view></template><script setup lang="uts">\nconst mystuff = ref<any[]>([])\n</script>';
    expect(scanTemplateFields(unknown, fields)).toEqual([]);
  });

  it('M 能报出 getMapped 传 build* 裸函数引用（对照组：箭头包裹 / 末参非 build* / 嵌套括号不误报）', () => {
    expect(scanBareFnRefMapper("return getMapped<Foo>('/x', params, buildFooListResult)")).toHaveLength(1);
    expect(scanBareFnRefMapper("return postMapped<Foo>('/x', payload, buildFooItem)")).toHaveLength(1);
    expect(scanBareFnRefMapper("return getMapped<Foo>('/x', params, (data : UTSJSONObject) : Foo => buildFoo(data))")).toEqual([]);
    expect(scanBareFnRefMapper("return getMapped<Foo>('/x', null, map)")).toEqual([]);
    expect(scanBareFnRefMapper("return getMapped<Foo>('/x' + id.toString(), null, buildFoo)")).toHaveLength(1);
  });

  it('N 能报出 as unknown as 双重强转（对照组：单重 as 不报）', () => {
    expect(scanUnknownDoubleCast("item: null as unknown as WrongQuestionItem")).toHaveLength(1);
    expect(scanUnknownDoubleCast("x = obj as UTSJSONObject")).toEqual([]);
  });

  it('O 能报出模板裸插值 function 名（对照组：computed/带括号调用/未定义名不报）', () => {
    const bare = '<template><text>{{ displayTypeName }}</text></template>\n<script setup lang="uts">\n    function displayTypeName() : string { return \'\' }\n</script>';
    expect(scanBareFnInterpolation(bare)).toHaveLength(1);
    const called = '<template><text>{{ getTypeName(t) }}</text></template>\n<script setup lang="uts">\n    function getTypeName(t : string) : string { return t }\n</script>';
    expect(scanBareFnInterpolation(called)).toEqual([]);
    const computedCase = '<template><text>{{ displayDate }}</text></template>\n<script setup lang="uts">\n    const displayDate = computed(() : string => \'\')\n</script>';
    expect(scanBareFnInterpolation(computedCase)).toEqual([]);
  });

  it('P 能报出可选对象 prop 成员直读（对照组：扁平原始 props/数组 prop/局部val 不报）', () => {
    const bad = '<template><text>{{ item.wrong_count }}</text></template>\n<script setup lang="uts">\n    withDefaults(defineProps<{\n        item?: WrongQuestionItem\n        expanded?: boolean\n    }>(), {})\n</script>';
    expect(scanOptionalObjectPropAccess(bad)).toHaveLength(1);
    const scriptBad = '<template><text>x</text></template>\n<script setup lang="uts">\n    const props = withDefaults(defineProps<{\n        item?: WrongQuestionItem\n    }>(), {})\n    function f() : string { return props.item.type }\n</script>';
    expect(scanOptionalObjectPropAccess(scriptBad)).toHaveLength(1);
    const flat = '<template><text>{{ wrongCount }}</text></template>\n<script setup lang="uts">\n    withDefaults(defineProps<{\n        wrongCount?: number\n        type?: string\n    }>(), { wrongCount: 0 })\n</script>';
    expect(scanOptionalObjectPropAccess(flat)).toEqual([]);
    const arrayProp = '<template><view v-for="s in sessions">{{ s.title }}</view></template>\n<script setup lang="uts">\n    withDefaults(defineProps<{\n        sessions?: AiSession[]\n    }>(), {})\n</script>';
    expect(scanOptionalObjectPropAccess(arrayProp)).toEqual([]);
    const localVal = '<template><text>x</text></template>\n<script setup lang="uts">\n    const props = withDefaults(defineProps<{\n        redoResult?: RedoResult | null\n    }>(), {})\n    function r2() : RedoResult | null { return props.redoResult }\n    function f() : boolean { const r = r2(); if (r == null) return false; return r.correct }\n</script>';
    expect(scanOptionalObjectPropAccess(localVal)).toEqual([]);
  });

  it('Q 能报出 ref<any> 类型擦除声明（对照组：具体类型 ref / any | null 形参不报）', () => {
    expect(scanRefAnyDeclaration('const statsCardRef = ref<any | null>(null)')).toHaveLength(1);
    expect(scanRefAnyDeclaration('const x = ref<any>(1)')).toHaveLength(1);
    expect(scanRefAnyDeclaration('const stats = ref<WrongQuestionStats>(s)')).toEqual([]);
    expect(scanRefAnyDeclaration('function f(e : any | null) : void {}')).toEqual([]);
  });

  it('R 能报出 async 函数声明 : void 返回类型（对照组：Promise<void> / 泛型 Promise<T> / 同步 : void 不报）', () => {
    expect(scanAsyncVoidReturn('async function loadStats() : void { }')).toHaveLength(1);
    expect(scanAsyncVoidReturn('const go = async (e : UTSJSONObject) : void => { }')).toHaveLength(1);
    expect(scanAsyncVoidReturn('async function loadStats() : Promise<void> { }')).toEqual([]);
    expect(scanAsyncVoidReturn('async function loadStats() : Promise<Stats> { }')).toEqual([]);
    expect(scanAsyncVoidReturn('function reset() : void { }')).toEqual([]);
  });

  it('S 能报出模板直调 import 函数（对照组：本地函数直调/import 仅在 script 消费/as 别名按本地名 不报）', () => {
    const bad = "<template><text>{{ fmtDate(d) }}</text></template>\n<script setup lang=\"uts\">\n    import { fmtDate } from '../../../utils/format'\n    const d = ref<string>('')\n</script>";
    expect(scanImportedFnTemplateCall(bad)).toHaveLength(1);
    const badAttr = "<template><view :class=\"cls(x)\"></view></template>\n<script setup lang=\"uts\">\n    import { cls } from '../../utils/x'\n</script>";
    expect(scanImportedFnTemplateCall(badAttr)).toHaveLength(1);
    const localOk = "<template><text>{{ fmt(d) }}</text></template>\n<script setup lang=\"uts\">\n    function fmt(s : string) : string { return s }\n    const d = ref<string>('')\n</script>";
    expect(scanImportedFnTemplateCall(localOk)).toEqual([]);
    const scriptOnly = "<template><text>{{ x }}</text></template>\n<script setup lang=\"uts\">\n    import { computed } from 'vue'\n    const x = computed(() : string => '')\n</script>";
    expect(scanImportedFnTemplateCall(scriptOnly)).toEqual([]);
    const memberOk = "<template><text>{{ obj.fmt(d) }}</text></template>\n<script setup lang=\"uts\">\n    import { fmt } from '../../utils/x'\n    const obj = { fmt: (s : string) : string => s }\n</script>";
    expect(scanImportedFnTemplateCall(memberOk)).toEqual([]);
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

  it('F：无裸 String() 强转（Kotlin 无 String(x) 重载，应为 x.toString()）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      // app-ios 原生互操作文件用 Swift 编译：String(data:encoding:) 是合法初始化器，不在本规则靶内
      if (u.file.includes('app-ios')) continue;
      for (const h of scanBareStringCall(u.code)) violations.push(`${path.relative(ROOT, u.file)}: ${h}`);
    }
    expect(violations).toEqual([]);
  });

  it('G：无 undefined 字面量（UTS 空值统一 null，Kotlin 找不到名称 undefined）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanUndefinedLiteral(u.code)) violations.push(`${path.relative(ROOT, u.file)}: ${h}`);
    }
    expect(violations).toEqual([]);
  });

  it('H：catch 参数无显式 : any 注解（allowlist 外零命中，应 (e) 或 any | null）', () => {
    const violations = [];
    const exempt = GUARD_ALLOWLIST.H;
    for (const u of allCodeUnits()) {
      if (exempt.has(path.relative(ROOT, u.file))) continue;
      for (const h of scanCatchAnyParam(u.code)) violations.push(`${path.relative(ROOT, u.file)}: ${h}`);
    }
    expect(violations).toEqual([]);
  });

  it('I：script 内无 : any 参数访问 .detail（类型化事件对象自带 detail 属合法，按组合判定）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanAnyParamDetailAccess(u.code)) violations.push(`${path.relative(ROOT, u.file)}: ${h}`);
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

  it('K：模板 v-for 别名无未声明字段访问（Kotlin error18: 找不到名称）', () => {
    const violations = [];
    const fieldMap = buildTypeFieldMap();
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
      for (const h of scanTemplateFields(fs.readFileSync(file, 'utf8'), fieldMap)) violations.push(path.relative(ROOT, file) + ': ' + h);
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

  it('M：*Mapped 出口无 build* 裸函数引用 mapper（Kotlin error17，箭头包裹才合法）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanBareFnRefMapper(u.code)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('N：全工程无 as unknown as 双重强转（Kotlin error18，对象 prop 走工厂默认值）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanUnknownDoubleCast(u.code)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('O：模板无裸插值引用 function 名（uni-app x 不自动调用无参 function，静默渲染源码）', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
      for (const h of scanBareFnInterpolation(fs.readFileSync(file, 'utf8'))) violations.push(path.relative(ROOT, file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('P：无可选对象 prop 成员直读（Kotlin error18 找不到名称；扁平原始 props 或局部 val+判空）', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
      for (const h of scanOptionalObjectPropAccess(fs.readFileSync(file, 'utf8'))) violations.push(path.relative(ROOT, file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('Q：无 ref<any> 类型擦除声明（Kotlin error18 any 无成员；组件引用用 interface 或类型化包装）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanRefAnyDeclaration(u.code)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('R：async 函数返回类型无 : void 声明（Kotlin 推断不出 UTSPromise 类型参数，级联编译错；须 Promise<void>）', () => {
    const violations = [];
    for (const u of allCodeUnits()) {
      for (const h of scanAsyncVoidReturn(u.code)) violations.push(path.relative(ROOT, u.file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });

  it('S：模板零直调 import 函数（uvue 模板 import 调用 = Kotlin error18 invoke；须 script 本地包装）', () => {
    const violations = [];
    for (const file of ALL_FILES.filter((f) => f.endsWith('.uvue'))) {
      for (const h of scanImportedFnTemplateCall(fs.readFileSync(file, 'utf8'))) violations.push(path.relative(ROOT, file) + ': ' + h);
    }
    expect(violations).toEqual([]);
  });
});
