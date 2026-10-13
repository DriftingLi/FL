/**
 * .uvue 重复 import 守护（refs #1675；事故源 #1670 / #1673）
 *
 * 事故：#1670（批2 N03 页头收口）在 11 个 .uvue 页面引入了同文件重复 import——同一标识符
 * 被多条 import 声明，`vue/compiler-sfc` 报 `Identifier 'xxx' has already been declared`，
 * mp-weixin 发布面编译失败（settings.uvue:90/92/94/96 同一 import 重复 4 次）。
 * 为什么漏网：② 门只对命中 MP-WEIXIN 面的 PR 必跑（#1670 的 ② 行写「免」），
 * ④c Kotlin 工具链不走 compiler-sfc，③ 门只扫 utils/ 下的 .test 文件——页面源码的语法错误
 * 在四门里没有落点。本守护把这条失效面补上：**重复 import 在 ③ 门就地变红**。
 *
 * 判据口径：同一 script 块内，同一标识符在同一声明空间（value / type）被 ≥2 条 import
 * 声明 ⇒ 违规。`import type { A }` 与 `import { A }` 并存**不算**（TS 类型空间与值空间
 * 分离，合法）；两条 `import type { A }` 算（TS2300）。别名的本地名才是声明名
 * （`import { goBack as g2 }` 声明的是 g2，不与 goBack 冲突）。
 *
 * 分类：接线守护（读源码文本断言），不构成 ③ 门证据；判别力由注入违规样本的必红面自证。
 * 扫描面 = 工程全部 .uvue（App.uvue / pages / components 一并覆盖；.uts 走 UTS 编译器由 ④ 兜）。
 */
const h = require('./contractHarness');

/** #1670 的实际炸点（发布面第一个报错的页面）——载体锚点兼回归坐标 */
const SETTINGS_PAGE = 'pages/profile/settings.uvue';

/** 提取 <script> 块内容（setup 与普通 script 都扫） */
function scriptBlocks(src) {
  const out = [];
  const re = /<script\b[^>]*>([\s\S]*?)<\/script>/g;
  let m;
  while ((m = re.exec(src)) !== null) out.push(m[1]);
  return out;
}

/** 抹 JS 注释（块注释 + 行注释，`://` 例外防误杀 URL）。先例 certSelectorOverlayContract.stripComments */
function stripJsComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/**
 * 解析一条 import 子句（`import` 与 `from '...'` 之间的文本）声明的本地标识符。
 * 返回 [{ name, space }]；space = 'value' | 'type'。
 */
function bindingNamesOf(clause) {
  const names = [];
  let body = clause.trim();
  let space = 'value';
  const tm = body.match(/^type\s+/);
  if (tm) {
    space = 'type';
    body = body.slice(tm[0].length);
  }
  // 默认导出名（`import X ...` / `import X, {...} ...`）
  const dm = body.match(/^([\w$]+)\s*($|,)/);
  if (dm) {
    names.push({ name: dm[1], space });
    body = body.slice(dm[1].length).replace(/^[\s,]+/, '');
  }
  // 命名空间（`import * as NS ...`）
  const nm = body.match(/^\*\s*as\s+([\w$]+)/);
  if (nm) {
    names.push({ name: nm[1], space });
    body = '';
  }
  // 具名列表（`{ a, b as c, type d }`，跨行一并吃）
  const bm = body.match(/\{([\s\S]*?)\}/);
  if (bm) {
    for (const raw of bm[1].split(',')) {
      const piece = raw.trim();
      if (!piece) continue;
      const am = piece.match(/^(?:type\s+)?([\w$]+)(?:\s+as\s+([\w$]+))?$/);
      if (am) names.push({ name: am[2] || am[1], space });
    }
  }
  return names;
}

/** 单个 .uvue 源的重复 import 违规清单（空 = 守约） */
function duplicateImportViolations(src) {
  const v = [];
  scriptBlocks(src).forEach((raw, blockIdx) => {
    const code = stripJsComments(raw);
    const seen = new Map();
    const IMPORT_RE = /\bimport\b([\s\S]*?)from\s*['"][^'"]*['"]/g;
    let m;
    while ((m = IMPORT_RE.exec(code)) !== null) {
      for (const { name, space } of bindingNamesOf(m[1])) {
        const key = space + '·' + name;
        const n = (seen.get(key) || 0) + 1;
        seen.set(key, n);
        if (n === 2) {
          v.push(
            `标识符 ${name} 被 2 条 import 重复声明（${space === 'type' ? 'type ' : ''}import，script 块 #${blockIdx + 1}）`
              + '—— vue/compiler-sfc: Identifier has already been declared ⇒ mp-weixin 发布面编译失败（#1673）'
          );
        }
      }
    }
  });
  return v;
}

/** 全工程 .uvue 扫描；违规行带文件名前缀 */
function sweepUvueDuplicates() {
  const out = [];
  for (const f of h.sourceFilesIn('.').filter((p) => p.endsWith('.uvue'))) {
    for (const v of duplicateImportViolations(h.read(f))) {
      out.push(f + ' → ' + v);
    }
  }
  return out;
}

/* ══ 真实文件（必不红面） ══ */
describe('全仓 .uvue 重复 import 守护（真实源码，#1675）', () => {
  it('全工程 .uvue 零重复 import', () => {
    expect(sweepUvueDuplicates()).toEqual([]);
  });

  it('#1670 实际炸点 settings.uvue 零重复（回归坐标）', () => {
    expect(duplicateImportViolations(h.read(SETTINGS_PAGE))).toEqual([]);
  });
});

/* ══ 判别力（必红面）：注入违规样本，检测器必须红 ══ */
describe('检测器判别力（注入违规样本必红）', () => {
  it('#1670 形状：同一 import 同文本重复两条 → 红', () => {
    // 注入点必须在 script 块内部（追加到整文件末尾会落在 </style> 之后，检测器看不见）；
    // 锚点 = 该页真实开标签形状（<script setup lang="uts">）
    const injected = h
      .read(SETTINGS_PAGE)
      .replace('<script setup lang="uts">', '<script setup lang="uts">\nimport { ref } from \'vue\'\nimport { ref } from \'vue\'');
    expect(injected).not.toBe(h.read(SETTINGS_PAGE));
    const v = duplicateImportViolations(injected);
    expect(v.some((x) => x.includes('标识符 ref'))).toBe(true);
  });

  it('跨行 import 重复 → 红（import { 换行 ... } 形态不被漏检）', () => {
    const dup = [
      'import {',
      '  getCourseListApi,',
      '  getLevelsApi,',
      "} from '../../api/course'",
      "import { getLevelsApi } from '../../api/course'",
    ].join('\n');
    const v = duplicateImportViolations('<script setup>\n' + dup + '\n</script>');
    expect(v.some((x) => x.includes('标识符 getLevelsApi'))).toBe(true);
  });

  it('两条 import type 同名 → 红（type 空间同样不容重复声明）', () => {
    const dup = [
      "import type { CourseItem } from '../../types/index'",
      "import type { CourseItem } from '../../types/index'",
    ].join('\n');
    const v = duplicateImportViolations('<script setup>\n' + dup + '\n</script>');
    expect(v.some((x) => x.includes('type import'))).toBe(true);
  });

  it('别名本地名不同 → 不红（import { goBack as g2 } 不与 goBack 冲突）', () => {
    const src = [
      '<script setup>',
      "import { goBack } from '../../utils/navigation'",
      "import { goBack as g2 } from '../../utils/navigation'",
      '</script>',
    ].join('\n');
    expect(duplicateImportViolations(src)).toEqual([]);
  });
});
