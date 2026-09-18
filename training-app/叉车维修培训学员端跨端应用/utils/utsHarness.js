/**
 * UTS 行为级测试缝：把 `.uts` 模块**真正跑起来**，而不只是断言源码文本
 *
 * 背景：#1124 的验收要求「能观测到刷新请求被调用了几次、并发 401 最终是 resolve 还是 reject」，
 * 纯源码契约测试（`requestOutletContract` / `quickLoginContract` 那一类）做不到这一点 ——
 * 它只能证明「某个字面量出现过」，证明不了「并发只发一次刷新」。
 *
 * 做法：`.uts` 是 TypeScript 方言，本仓用到的语法子集可以被机械地去掉类型后当 JS 执行：
 *   ① Babel 解析（`@babel/plugin-syntax-typescript`，随 jest 依赖树一同安装）
 *   ② 去类型注解 / 去 `import`（依赖由调用方以 `bindings` 注入）/ 去 `export`
 *   ③ `new Function('bindings', …)` 求值，回读 `export` 出的名字
 * 于是「模块级状态 + 并发 Promise 语义」这类**只有跑起来才看得见**的行为可以被机检。
 *
 * 与源码契约测试的分工：这里跑行为（次数 / resolve 还是 reject / 存储与跳转副作用），
 * 源码契约测试守结构（函数签名、调用点、key 清单）。两者都要 —— 前者证明行为对，
 * 后者证明**接线没断**（行为测试会自己搭依赖，接线断了它照样绿）。
 *
 * 边界（写实，不假装通用）：
 *   - 只支持本仓 `.uts` 实际用到的 TS 子集；遇到没覆盖的语法（如 `enum` / 装饰器）会抛错，不静默降级。
 *   - 遇到无法识别的 `import` 形态直接 FAIL（fail-closed）—— 否则会静默少注入依赖。
 *   - `.uvue` 的模板/样式不参与（那是渲染层，node 里跑不了）；只跑 `.uts` 逻辑模块。
 */
const fs = require('fs');
const path = require('path');

/** 去类型 / 去 import / 去 export（`.uts` 的 JS 可执行子集） */
function stripUtsPlugin() {
  return {
    visitor: {
      TSTypeAnnotation(p) { p.remove(); },
      TSTypeParameterInstantiation(p) { p.remove(); },
      TSTypeParameterDeclaration(p) { p.remove(); },
      TSAsExpression(p) { p.replaceWith(p.node.expression); },
      TSNonNullExpression(p) { p.replaceWith(p.node.expression); },
      TSTypeAliasDeclaration(p) { p.remove(); },
      TSInterfaceDeclaration(p) { p.remove(); },
      TSDeclareFunction(p) { p.remove(); },
      ImportDeclaration(p) { p.remove(); },
      ExportNamedDeclaration(p) {
        if (p.node.declaration) p.replaceWith(p.node.declaration);
        else p.remove();
      },
      ExportDefaultDeclaration(p) { p.remove(); },
    },
  };
}

/** `import { a, b as c } from 'x'` 的运行时绑定名（`import type` 是纯类型，不需要绑定） */
function importedNames(src) {
  const names = [];
  const re = /^[ \t]*import\s+(type\s+)?\{([^}]*)\}\s+from\s+['"][^'"]+['"]/gm;
  let m;
  let matched = 0;
  while ((m = re.exec(src)) !== null) {
    matched += 1;
    if (m[1]) continue;
    m[2]
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
      .forEach((s) => names.push(s.split(/\s+as\s+/).pop().trim()));
  }
  // fail-closed：有 import 语句没被本正则识别（默认导入 / 命名空间导入 / 无花括号形态）就报错，
  // 不能静默少注入一个依赖 —— 那样测试会绿得毫无意义。
  const total = (src.match(/^[ \t]*import\s/gm) || []).length;
  if (total !== matched) {
    throw new Error(`utsHarness: 有 ${total - matched} 条 import 不被支持（仅支持 import { … } / import type { … }）`);
  }
  return names;
}

/** `export function f` / `export const c` 的名字（`export type` 是纯类型，不回读） */
function exportedNames(src) {
  const names = [];
  const re = /^[ \t]*export\s+(?:async\s+)?function\s+([A-Za-z0-9_$]+)|^[ \t]*export\s+const\s+([A-Za-z0-9_$]+)/gm;
  let m;
  let matched = 0;
  while ((m = re.exec(src)) !== null) {
    matched += 1;
    names.push(m[1] || m[2]);
  }
  // fail-closed（与 import 那条对称）：有 export 语句没被本正则识别（`export default` /
  // `export { a, b }` 列表形态 / `export class`）就报错，否则那个名字会被**静默**从回读结果里丢掉，
  // 测试拿到的模块少了一个导出却不报错。
  const total = (src.match(/^[ \t]*export\s/gm) || []).length;
  const typeOnly = (src.match(/^[ \t]*export\s+type\s/gm) || []).length;
  if (matched !== total - typeOnly) {
    throw new Error(`utsHarness: 有 ${total - typeOnly - matched} 条 export 不被支持（仅支持 export function / export const，export type 忽略）`);
  }
  return names;
}

/** 依赖缺了就 fail-loud 报清缺哪条，而不是抛一句 MODULE_NOT_FOUND（二者都随 jest 依赖树安装） */
function loadBabel() {
  try {
    return {
      babel: require('@babel/core'),
      syntaxTypescript: require('@babel/plugin-syntax-typescript').default,
    };
  } catch (e) {
    throw new Error(
      'utsHarness 需要 @babel/core 与 @babel/plugin-syntax-typescript'
      + '（随 jest 的转译依赖树安装；若本机 node_modules 不完整，先在项目目录跑 npm ci）：'
      + String(e && e.message)
    );
  }
}

/**
 * 载入并执行一个 `.uts` 模块
 * @param {string} file 模块绝对路径
 * @param {Record<string, unknown>} bindings 该模块 import 的依赖 + 它引用的全局量（如 `uni`、`getCurrentPages`）
 * @returns {Record<string, unknown>} 模块 `export` 出的名字 → 值（每次调用都是**全新模块实例**，模块级状态互不串）
 */
function loadUts(file, bindings) {
  const src = fs.readFileSync(file, 'utf8');
  const need = importedNames(src);
  const missing = need.filter((n) => !(n in bindings));
  if (missing.length > 0) {
    throw new Error(`utsHarness: ${path.basename(file)} 缺绑定：${missing.join(', ')}`);
  }
  const { babel, syntaxTypescript } = loadBabel();
  const out = babel.transformSync(src, {
    filename: path.basename(file) + '.ts',
    plugins: [syntaxTypescript, stripUtsPlugin],
    babelrc: false,
    configFile: false,
  });
  const exps = exportedNames(src);
  const names = [...new Set(need.concat(Object.keys(bindings)))];
  const body = [
    `const { ${names.join(', ')} } = bindings;`,
    out.code,
    `return { ${exps.map((n) => `${n}: ${n}`).join(', ')} };`,
  ].join('\n');
  return new Function('bindings', body)(bindings); // eslint-disable-line no-new-func
}

module.exports = { loadUts, importedNames, exportedNames };
