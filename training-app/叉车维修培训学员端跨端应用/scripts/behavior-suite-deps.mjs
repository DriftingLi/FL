#!/usr/bin/env node
/**
 * 行为腿「改动路径 → 受影响套件」映射生成器（#2019，ADR-0046 步骤 2）
 *
 * 用途：给定一组改动文件，算出本地该跑哪些行为腿套件。全量交 CI 4 片，
 *       本地只跑受影响子集 —— ADR-0046 的裁定前提就是本表够不够用。
 *
 * ── 为什么是**故意过覆盖** ──
 * 判据是「预测集合 ⊇ 实际失败集合」（#2019 票面）：多跑只浪费时间，漏跑会让
 * 「本地绿、CI 红」发生两次，然后整件事被放弃。所以抽取器一律朝**宽**的方向错：
 *   · 不区分「调用点字面量」与「const 里定义的字面量」——路径在本仓大量写成
 *     `const ENTRY = path.join(ROOT, 'scripts', 'x.ps1')`，只抽调用点会漏；
 *   · 不区分「断言用的路径」与「真读的路径」——注释先被剥掉，剩下的字符串里
 *     只要长得像仓内路径就算一条边；
 *   · 按 **basename** 建反查索引，同名文件跨目录时一起命中（宁可多跑）。
 *
 * ── 三条边（缺一条就会漏） ──
 * 1. **直接边**：套件源码里出现的路径字面量（`.uts` / `.ps1` / `.mjs` / `.uvue` / …）。
 * 2. **引用闭包边**：`.uts` 模块之间会 `import`（`forumDisplay.uts → forumBody.uts`）。
 *    改被 import 的那个，测 import 方 的套件也可能红 —— 而套件里只有 import 方的路径。
 *    故先把改动文件按 import 图反向闭包，再映射到套件。
 * 3. **全局边（票面要求，不在本表内自动判定，由 --changed 强制附加）**：
 *    · 门脚本 / workflow 改动（`.github/**`、`scripts/**`）⇒ 全跑 —— d 类套件测的是
 *      门脚本自身行为，「改了哪个业务文件」这个维度对它们失效；
 *    · b / b′ 名单（`--audit` 里 text-coupled 的那些）⇒ 只要命中就全跑。
 *
 * ── 用法 ──
 *   node scripts/behavior-suite-deps.mjs --audit
 *       全量分类：每个行为套件的执行形态（a/d/e）、文本耦合（b/b′）、抽到的路径边。
 *   node scripts/behavior-suite-deps.mjs --json > map.json
 *       机器可读的完整映射（含反向索引与闭包）。
 *   node scripts/behavior-suite-deps.mjs --changed <file> [<file>…]
 *       给改动文件，打印应跑套件（含被触发的全局边与原因）。
 *   node scripts/behavior-suite-deps.mjs --changed-from stdin
 *       同上，但改动清单从 stdin 读（每行一个路径，`git diff --name-only` 直接喂）。
 *   node scripts/behavior-suite-deps.mjs --changed-json --changed <file> [<file>…]
 *   node scripts/behavior-suite-deps.mjs --changed-json --changed-from stdin
 *       与上面两条同义，但把结果打成 JSON（`total` / `subset` / `reasons` / `uncovered` / `explain`）。
 *       `explain` 就是 `--changed` 那份人读文本的逐行数组 ⇒ 脚本打印它即可，不必重排格式。
 *       只多一种**输出形态**，判据与人类输出一字不动（#2034 接进 dev-finish 用）。
 *       ⚠️ `--changed-json` 必须与 `--changed` / `--changed-from` 之一同给（单独给它没有改动清单）。
 *
 * ── 不做的事 ──
 * 不执行任何测试、不联网、不写文件（只打 stdout）。只读。
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// 判据源与去注释**只有一份**，住在 `scripts/lib/guard-split.js`（#1702）。本脚本不复制判据，
// 只消费它切出来的行为腿列表 —— 否则「哪些套件是行为腿」会出现第二份真源。
import guardSplit from './lib/guard-split.js';

const { splitSuites, stripComments } = guardSplit;
const HERE = path.dirname(fileURLToPath(import.meta.url));

/** 仓内路径可能出现的扩展名（含 `.uvue`/`.uts` —— 运行时面本体）。 */
const PATH_EXT = /\.(uts|uvue|nvue|vue|js|mjs|cjs|ts|ps1|json|md|css|scss|sql|go|yml|yaml|png|jpg|jpeg|webp)$/i;

/** 字符串字面量：单引号 / 双引号 / 反引号（不跨行，路径不会跨行）。 */
const STRING_LITERAL = /(['"`])((?:(?!\1)[^\\\n]|\\.){1,200})\1/g;

/** d 类判据：真起子进程。与 `guard-split.js` 的 EXECUTION_MARKERS 同源，此处只要落类。 */
const SPAWN_RE = /\b(?:spawnSync|execFileSync|execSync|spawn|exec)\s*\(|require\(\s*['"]child_process['"]\s*\)|from\s+['"]node:child_process['"]/;
/** a 类判据：把 `.uts` 当 JS 真执行。 */
const LOAD_UTS_RE = /\bloadUts\s*\(/;
/** e 类判据：动态 import()。 */
const DYN_IMPORT_RE = /\bimport\s*\(/;
/** 文本耦合：读源码文本。 */
const READ_TEXT_RE = /\breadText\s*\(|\breadFileSync\s*\(/g;
/** 变异体注入的真实写法（本仓）：读真源 → split/join 或 replace 注入 → 落临时目录 → 重新 loadUts。 */
const MUTATE_SIGNALS = [/\.split\s*\(/, /\.replace(?:All)?\s*\(/];
const TMP_WRITE_RE = /\bmkdtempSync\s*\(|\bwriteFileSync\s*\(/;
/** 对源码文本做形状断言（b 类的特征）。 */
const TEXT_ASSERT_RE = /\.(?:toContain|toMatch|toMatchInlineSnapshot)\s*\(/;

/** 取 basename 作为反查键（跨目录同名一起命中 ⇒ 过覆盖方向）。 */
function key(p) {
  return path.basename(String(p).replace(/\\/g, '/'));
}

/** 一个字符串字面量是否像仓内路径。 */
function looksLikePath(s) {
  if (!s || s.length > 160) return false;
  const t = s.trim();
  if (!t || /\s/.test(t)) return false;
  if (!PATH_EXT.test(t)) return false;
  // 纯扩展名片段（如 `'.js'`）不算
  if (/^\.[a-z0-9]+$/i.test(t)) return false;
  return true;
}

/** 套件源码 ⇒ 路径字面量集合（键 = basename）。 */
function extractTokens(src) {
  const code = stripComments(src);
  const out = new Map(); // basename -> 原始字面量集合
  let m;
  STRING_LITERAL.lastIndex = 0;
  while ((m = STRING_LITERAL.exec(code)) !== null) {
    const raw = m[2];
    if (!looksLikePath(raw)) continue;
    const norm = raw.replace(/\\/g, '/').replace(/^\.\//, '');
    const k = key(norm);
    if (!out.has(k)) out.set(k, new Set());
    out.get(k).add(norm);
  }
  return out;
}

/**
 * 模块级常量 ⇒ 它指向的路径。
 * 支持多跳（`const UTS = path.join(ROOT, SRC_REL)`，`SRC_REL` 才是字面量所在）——
 * 单层解析会漏掉本仓最常见的两级写法（实测 `aiSourcesDisplay` / `forumBody` 都栽在这）。
 * 猜错的方向是多算一条边（过覆盖），不是漏边。
 */
function buildConstMap(code) {
  const raw = new Map();
  const re = /(?:^|\n)[ \t]*(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*([^;]{1,400});/g;
  let m;
  while ((m = re.exec(code)) !== null) raw.set(m[1], m[2]);

  const cache = new Map();
  const resolve = (name, depth = 0) => {
    if (cache.has(name)) return cache.get(name);
    if (depth > 3) return '';
    const init = raw.get(name);
    if (init === undefined) return '';
    let best = '';
    STRING_LITERAL.lastIndex = 0;
    let s;
    while ((s = STRING_LITERAL.exec(init)) !== null) {
      if (looksLikePath(s[2])) best = s[2];
    }
    if (!best) {
      for (const id of init.match(/[A-Za-z_$][\w$]*/g) || []) {
        if (id === name) continue;
        best = resolve(id, depth + 1);
        if (best) break;
      }
    }
    const out = best ? best.replace(/\\/g, '/') : '';
    cache.set(name, out);
    return out;
  };

  const map = new Map();
  for (const name of raw.keys()) {
    const v = resolve(name);
    if (v) map.set(name, v);
  }
  return map;
}

/**
 * 读源码文本的调用解析成「读的是不是运行时面文件」（`.uts` / `.uvue`）。
 * 参数是标识符 ⇒ 查常量表；参数是字面量 ⇒ 直接用。
 */
function readsSurfaceSource(code, consts) {
  const hits = [];
  const re = /\b(?:readText|readFileSync)\s*\(\s*([A-Za-z_$][\w$.]*)\s*[),]/g;
  let m;
  while ((m = re.exec(code)) !== null) {
    const arg = m[1];
    const resolved = consts.get(arg) || (looksLikePath(arg) ? arg : '');
    if (/\.(uts|uvue)$/i.test(resolved)) hits.push(resolved);
  }
  // 参数直接是字面量的形态
  const re2 = /\b(?:readText|readFileSync)\s*\(\s*['"`]([^'"`]+)['"`]/g;
  while ((m = re2.exec(code)) !== null) {
    if (/\.(uts|uvue)$/i.test(m[1])) hits.push(m[1]);
  }
  return [...new Set(hits)];
}

/**
 * 模块之间的 import 图（只收相对路径与 `@/` 前缀，包名不算）。
 *
 * 节点 id = 文件相对 mobileRoot 的路径（posix）。**不能用 basename**：本仓有大量同名文件
 * （`auth.uts` / `forum.uts` / `index.uts` …），以 basename 为节点会让两条依赖边互相覆盖
 * （第一版实测 `api/request.uts` 的边被吞掉，导致 `transport.uts` 的闭包少了一路）。
 *
 * ⚠️ import 必须容忍**多行写法**（`import {\n  a,\n} from './x'`）——单行正则会漏掉它们，
 * 第一版就是这样漏了 `api/request.uts -> ./transport`。
 *
 * 解析必须真实落盘（依次试原样 / `.uts` / `.js` / `.ts` / `/index.*`）。
 * 返回 { graph: Map(rel → Set(rel)), unresolved: string[], byBasename: Map(base → Set(rel)) }
 */
function buildImportGraph(root) {
  const graph = new Map();
  const unresolved = [];
  const byBasename = new Map();
  const EXTS = ['', '.uts', '.js', '.ts', '/index.uts', '/index.ts', '/index.js'];
  const rel = (abs) => path.relative(root, abs).replace(/\\/g, '/');

  const resolveSpec = (fromFile, spec) => {
    const base = spec.startsWith('@/')
      ? path.join(root, spec.slice(2))
      : path.resolve(path.dirname(fromFile), spec);
    for (const ext of EXTS) {
      const cand = base + ext;
      try {
        if (fs.statSync(cand).isFile()) return cand;
      } catch {
        /* 继续试下一个 */
      }
    }
    return '';
  };

  const scan = (dir) => {
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const e of entries) {
      const full = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (['node_modules', '.git', 'unpackage', 'dist', 'build'].includes(e.name)) continue;
        scan(full);
        continue;
      }
      if (!/\.(uts|js|mjs)$/.test(e.name)) continue;
      if (/\.test\.[cm]?js$/.test(e.name)) continue;
      const self = rel(full);
      if (!byBasename.has(e.name)) byBasename.set(e.name, new Set());
      byBasename.get(e.name).add(self);

      const src = stripComments(fs.readFileSync(full, 'utf8'));
      const deps = new Set();
      // import / export-from / require。花括号里用 `[^}]*`（跨行安全，不受「多少字符」上限影响）——
      // 本仓 `api/request.uts` 的 import 块跨 13 行，按字符数设上限会静默漏边。
      const SPECS = [
        /^[ \t]*(?:import|export)\s+(?:type\s+)?(?:\{[^}]*\}|[\w$*]+(?:\s*,\s*\{[^}]*\})?)\s*from\s*['"]([^'"]+)['"]/gm,
        /^[ \t]*import\s*['"]([^'"]+)['"]/gm,
        /require\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
      ];
      for (const re of SPECS) {
        let m;
        while ((m = re.exec(src)) !== null) {
          const spec = m[1];
          if (!/^\.{1,2}\//.test(spec) && !spec.startsWith('@/')) continue;
          const resolved = resolveSpec(full, spec);
          if (!resolved) {
            unresolved.push(`${self} -> ${spec}`);
            continue;
          }
          deps.add(rel(resolved));
        }
      }
      if (deps.size) graph.set(self, deps);
    }
  };
  scan(root);
  return { graph, unresolved, byBasename };
}

/** import 图反向：被 import 的模块名 -> 谁 import 它（直接）。 */
function invert(graph) {
  const inv = new Map();
  for (const [mod, deps] of graph) {
    for (const d of deps) {
      if (!inv.has(d)) inv.set(d, new Set());
      inv.get(d).add(mod);
    }
  }
  return inv;
}

/** 移动端子项目的目录名（改动路径里出现它就可以切成相对路径）。 */
const MOBILE_SEG = '叉车维修培训学员端跨端应用/';

/** 改动文件 ⇒ 图上的种子节点（相对路径）+ 同名节点的全部实例。 */
function seedsOf(changed, map) {
  const seeds = new Set();
  for (const c of changed) {
    const p = String(c).replace(/\\/g, '/');
    const i = p.indexOf(MOBILE_SEG);
    const rel = i >= 0 ? p.slice(i + MOBILE_SEG.length) : p;
    if (rel && !rel.startsWith('..')) seeds.add(rel);
    for (const n of map.importByBasename.get(key(c)) || []) seeds.add(n);
  }
  return seeds;
}
/**
 * 种子节点沿 import 图**反向**闭包（含自身）：改 `forumBody.uts` ⇒ 把
 * `forumDisplay.uts` / `forumDetailDisplay.uts` 等 import 方也拉进来。
 * 节点是相对路径（不是 basename —— 同名文件会让闭包串味）。
 */
function importClosure(seeds, inv) {
  const seen = new Set();
  const queue = [...seeds];
  while (queue.length) {
    const cur = queue.shift();
    if (seen.has(cur)) continue;
    seen.add(cur);
    for (const up of inv.get(cur) || []) {
      if (!seen.has(up)) queue.push(up);
    }
  }
  return seen;
}

/**
 * 全局边：命中即全跑。
 * 判据与 #2019 票面一致，可机检（不写「看情况」）。
 */
function globalEdges(changed) {
  const reasons = [];
  const norm = changed.map((c) => c.replace(/\\/g, '/'));
  const gateScripts = norm.filter(
    (c) => /(^|\/)\.github\//.test(c) || /(^|\/)scripts\/.*\.(mjs|js|ps1)$/.test(c)
  );
  if (gateScripts.length) {
    reasons.push({
      edge: 'gate-script',
      why: '门脚本 / workflow 改动 ⇒ 行为腿全跑（d 类套件测的是脚本自身行为，与业务文件无关）',
      files: gateScripts,
    });
  }
  // 改测试文件本身：`jestSplitContract` 读的是**全部** `utils/*.test.js`（两腿切分、心跳契约、
  // 套件并集完备都建立在「有哪些套件」上）。增删改一个套件就会改这张表 ⇒ 全跑。
  // 这条边同时消掉「目录读取者」里最难判的一类：读取发生在子进程里，静态扫不到。
  const suiteFiles = norm.filter((c) => /(^|\/)utils\/[^/]+\.test\.js$/.test(c));
  if (suiteFiles.length) {
    reasons.push({
      edge: 'suite-set',
      why: '改动 utils/*.test.js ⇒ 全跑（两腿切分与心跳契约以套件集合为输入）',
      files: suiteFiles,
    });
  }
  // 页面集合变了：一批 d 类套件在**子进程**里扫 `pages/`（capabilitySurface / mpWeixinGatePages /
  // mpWeixinBuildFingerprint / levelDetect*），静态 token 只能覆盖「既有页面名」，
  // 新增页不会命中任何 token ⇒ 这里按结构性改动全跑。
  const pageSet = norm.filter((c) => /(^|\/)(pages|manifest)\.json$/.test(c));
  if (pageSet.length) {
    reasons.push({
      edge: 'page-set',
      why: '改 pages.json / manifest.json ⇒ 全跑（页面集合或应用配置变了，扫页面的套件与路由契约同时失效）',
      files: pageSet,
    });
  }
  return reasons;
}

function loadSuites(mobileRoot) {
  const utilsDir = path.join(mobileRoot, 'utils');
  const { behavior } = splitSuites(utilsDir);
  return behavior.map((f) => path.relative(mobileRoot, f).replace(/\\/g, '/'));
}

/**
 * 目录读取者的**目录名**（`readdirSync(UTILS)` ⇒ `utils`）。
 * 这类套件依赖的是「目录里有什么」，单个文件映射对它们无效 —— 改成「改动落在该目录下 ⇒ 纳入」。
 * 目录参数常写成 `path.join(__dirname, '..', 'utils')`（**无扩展名**），故不能复用 buildConstMap。
 */
function dirTokens(code) {
  const out = new Set();
  const re = /\breaddirSync\s*\(\s*([A-Za-z_$][\w$.]*)\s*[,)]/g;
  let m;
  const segs = new Map();
  const cre = /(?:^|\n)[ \t]*(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*([^;]{1,300});/g;
  let c;
  while ((c = cre.exec(code)) !== null) {
    const lits = [...c[2].matchAll(/(['"`])([^'"`]{1,80})\1/g)].map((x) => x[2]);
    const dirish = lits.filter((l) => /^[\w.-]+$/.test(l) && l !== '..' && l !== '.');
    if (dirish.length) segs.set(c[1], dirish[dirish.length - 1]);
  }
  while ((m = re.exec(code)) !== null) {
    const arg = m[1];
    if (segs.has(arg)) out.add(segs.get(arg));
    else if (/^[A-Za-z_$][\w$]*$/.test(arg) && /^[\w-]+$/.test(arg)) out.add(arg);
  }
  return out;
}

/** 单个套件的画像。 */
function profileSuite(mobileRoot, rel) {
  const src = fs.readFileSync(path.join(mobileRoot, rel), 'utf8');
  const code = stripComments(src);
  const consts = buildConstMap(code);

  const spawn = SPAWN_RE.test(code);
  const loadUts = LOAD_UTS_RE.test(code);
  const dyn = DYN_IMPORT_RE.test(code);
  const readsText = READ_TEXT_RE.test(code);
  READ_TEXT_RE.lastIndex = 0;
  const surfaceReads = readsSurfaceSource(code, consts);
  const hasSurgery = MUTATE_SIGNALS.some((r) => r.test(code));
  const assertsText = TEXT_ASSERT_RE.test(code);
  const tmpWrite = TMP_WRITE_RE.test(code);
  // 执行形态：d（起子进程）> a（loadUts 真执行）> e（动态 import）
  const mode = spawn ? 'd' : loadUts ? 'a' : dyn ? 'e' : '?';

  // 文本耦合：
  //   b  = 读了运行时面源码文本，并对文本形状做断言（等价重构会红）
  //   b' = 把源码文本复制一份、注入变异后再执行（锚点原文一变，变异可能没注入而断言照跑 —— 静默失效）
  //
  // b′ 用**结构信号**判，不做全数据流：写临时副本 + 读源码 + 真执行，这三件事同时出现
  // 就是本仓的变异体夹具形态（`mutatedCopy` / `loadMutated`）。形参传入的读法
  // （`readText(absFile)`）靠数据流追不到，但这条信号能覆盖。
  const readsSurfaceText = surfaceReads.length > 0;
  const tempCopy = TMP_WRITE_RE.test(code) && readsText;
  const mutated = tempCopy && loadUts;

  return {
    suite: rel,
    mode,
    spawn,
    loadUts,
    dyn,
    readsText,
    readsSurfaceText,
    surfaceReads,
    assertsText,
    hasSurgery,
    tempCopy,
    mutated,
    dirs: dirTokens(code),
    tokens: extractTokens(src),
  };
}

function buildMap(mobileRoot) {
  const suites = loadSuites(mobileRoot);
  const profiles = suites.map((s) => profileSuite(mobileRoot, s));

  const index = new Map(); // basename -> Set(suite)
  for (const p of profiles) {
    for (const k of p.tokens.keys()) {
      if (!index.has(k)) index.set(k, new Set());
      index.get(k).add(p.suite);
    }
  }
  const graph = buildImportGraph(mobileRoot);
  return {
    profiles,
    index,
    importGraph: graph.graph,
    importByBasename: graph.byBasename,
    unresolvedImports: graph.unresolved,
    importInverse: invert(graph.graph),
  };
}

/**
 * 改动 → 应跑套件。返回 {subset, reasons, uncovered}。
 *
 * @param {object} opts
 * @param {boolean} opts.bprimeAlways  b′ 名单是否**常驻**（见下方注释）
 */
function select(map, changed, opts = {}) {
  const reasons = new Map(); // suite -> Set(why)
  const add = (suite, why) => {
    if (!reasons.has(suite)) reasons.set(suite, new Set());
    reasons.get(suite).add(why);
  };

  // 闭包在「相对路径」空间里算，回到 index 时换回 basename（index 是 basename 索引，故意过覆盖）
  const polluted = importClosure(seedsOf(changed, map), map.importInverse);
  const viaClosure = new Set([...polluted].map((x) => key(x)));

  for (const c of changed) {
    const k = key(c);
    for (const s of map.index.get(k) || []) add(s, `direct:${k}`);
  }
  for (const m of viaClosure) {
    for (const s of map.index.get(m) || []) add(s, `import-closure:${m}`);
  }

  // 目录边：套件读的是「目录里有什么」（readdirSync），单文件映射对它无效。
  // 判据：改动路径里出现该目录段 ⇒ 纳入。
  for (const p of map.profiles) {
    for (const d of p.dirs) {
      if (changed.some((c) => c.replace(/\\/g, '/').split('/').includes(d))) {
        add(p.suite, `dir:${d}`);
      }
    }
  }

  // b′ 常驻：它们对「源码文本长什么样」下判断（读真源 → 字符串手术 → 再执行），
  // 「改了哪个文件」不足以判定相关性 —— 锚点原文一变，变异可能没注入而断言照跑。
  // 名单制（不是启发式），代价是恒定 +21 个套件；`--no-bprime-always` 可关掉做对照。
  if (opts.bprimeAlways !== false) {
    for (const p of map.profiles) {
      if (p.mutated) add(p.suite, "bprime-always:结构信号(临时副本+读真源+真执行)");
    }
  }

  for (const g of globalEdges(changed)) {
    for (const p of map.profiles) add(p.suite, `${g.edge}:${g.files.map(key).join(',')}`);
  }

  const subset = [...reasons.keys()].sort();
  // 未被任何边覆盖的改动（本地不会因此跑任何套件 —— 需要人看一眼）
  const uncovered = changed.filter((c) => {
    const k = key(c);
    return !(map.index.get(k) || new Set()).size && !viaClosure.has(k);
  });
  return { subset, reasons, uncovered, viaClosure };
}

function fmtPct(n, d) {
  return d === 0 ? '0.0%' : `${((n / d) * 100).toFixed(1)}%`;
}

// ─────────────────────────── CLI ───────────────────────────

const argv = process.argv.slice(2);
const rootArg = argv.find((a) => a.startsWith('--root='));
const mobileRoot = rootArg
  ? path.resolve(rootArg.slice('--root='.length))
  : path.resolve(HERE, '..');

const has = (f) => argv.includes(`--${f}`);

if (has('audit')) {
  const map = buildMap(mobileRoot);
  const { profiles } = map;
  const counts = { a: 0, d: 0, e: 0, '?': 0 };
  for (const p of profiles) counts[p.mode] += 1;

  console.log(`# 行为腿套件分类（${profiles.length} 个，现测；判据源 scripts/lib/guard-split.js）\n`);
  console.log(`执行形态：a(loadUts 真执行) ${counts.a} · d(起子进程) ${counts.d} · e(动态 import) ${counts.e} · 未判定 ${counts['?']}`);
  console.log(`文本耦合：读运行时面源码 ${profiles.filter((p) => p.readsSurfaceText).length} · 其中做变异体注入(b′) ${profiles.filter((p) => p.mutated).length}\n`);

  for (const p of profiles) {
    const tags = [p.mode];
    if (p.readsSurfaceText) tags.push('readSrc');
    if (p.assertsText) tags.push('assertText');
    if (p.mutated) tags.push("b'");
    const toks = [...p.tokens.keys()].sort();
    console.log(`${p.suite}\t${tags.join('+')}\t${toks.length}\t${toks.join(' ')}`);
  }
  process.exit(0);
}

if (has('json')) {
  const map = buildMap(mobileRoot);
  const out = {
    generatedFrom: 'scripts/behavior-suite-deps.mjs',
    suiteCount: map.profiles.length,
    profiles: map.profiles.map((p) => ({
      suite: p.suite,
      mode: p.mode,
      flags: {
        spawn: p.spawn,
        loadUts: p.loadUts,
        dynImport: p.dyn,
        readsSurfaceText: p.readsSurfaceText,
        mutatesSource: p.mutated,
        dirs: [...p.dirs],
      },
      tokens: [...p.tokens.keys()].sort(),
    })),
    reverseIndex: Object.fromEntries(
      [...map.index.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => [k, [...v].sort()])
    ),
    importGraph: Object.fromEntries(
      [...map.importGraph.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => [k, [...v].sort()])
    ),
  };
  console.log(JSON.stringify(out, null, 2));
  process.exit(0);
}

if (has('health')) {
  const map = buildMap(mobileRoot);
  const { profiles } = map;

  // ① 目录读取者：依赖的是「目录里有什么」，不是某个文件 ⇒ 单文件映射对它们无效。
  // 与 `select()` 的目录边**同一套判据**（`dirTokens`），否则报告里会出现两个数：
  // 裸正则会把 `readdirSync(tmpProject)`（断言临时目录为空，不是仓库依赖）也算进来。
  const dirReaders = profiles.filter((p) => p.dirs.size > 0).map((p) => `${p.suite}  →  ${[...p.dirs].join(', ')}`);

  // ② import 图解析完整性：每个相对 spec 都要能落到盘上，否则闭包边是假的
  const missing = map.unresolvedImports;

  // ③ 漂移 token：套件引用但仓里不存在的文件名（多半是拼错或已删）
  const dangling = [];
  for (const [tok, suites] of map.index) {
    if (!/\.(uts|ps1|mjs|uvue)$/.test(tok)) continue;
    const found =
      fs.existsSync(path.join(mobileRoot, 'utils', tok)) ||
      fs.existsSync(path.join(mobileRoot, 'scripts', tok)) ||
      fs.existsSync(path.join(mobileRoot, 'scripts', 'lib', tok)) ||
      fs.existsSync(path.join(mobileRoot, 'pages', tok));
    if (!found) {
      // 全仓宽搜一次（含根仓库 —— 门脚本住在 `.github/scripts/` 与根 `scripts/`，
      // 只看移动端目录会把 `gate-validator.mjs` / `preflight-pr.mjs` 误报成不存在）
      let hit = false;
      const repoRoot = path.resolve(mobileRoot, '..', '..');
      const walk = (dir, depth) => {
        if (hit || depth > 5) return;
        let es;
        try {
          es = fs.readdirSync(dir, { withFileTypes: true });
        } catch {
          return;
        }
        for (const e of es) {
          if (hit) return;
          if (e.isDirectory()) {
            if (['node_modules', '.git', 'unpackage', 'dist', 'build'].includes(e.name)) continue;
            walk(path.join(dir, e.name), depth + 1);
          } else if (e.name === tok) hit = true;
        }
      };
      walk(mobileRoot, 0);
      if (!hit) walk(repoRoot, 0);
      if (!hit) dangling.push(`${tok} ← ${[...suites].join(', ')}`);
    }
  }

  // ④ 零覆盖运行时面：`utils/*.uts` 里没有任何行为套件引用的（映射为空 ≠ 映射漏了）
  const uts = fs.readdirSync(path.join(mobileRoot, 'utils')).filter((f) => f.endsWith('.uts'));
  const orphans = uts.filter((f) => !(map.index.get(f) || new Set()).size);

  console.log(`# 健全性审计（生成物，可重跑）\n`);
  console.log(`① 目录读取者（单文件映射对它们无效，需按目录名匹配或全跑）：${dirReaders.length}`);
  for (const d of dirReaders) console.log(`   ${d}`);
  console.log(`\n② import 图里解析不到的 spec：${missing.length}${missing.length ? '' : '（全部可解析 ⇒ 闭包边成立）'}`);
  for (const m of missing) console.log(`   ${m}`);
  console.log(`\n③ 静态抽到但仓里找不到的文件名：${dangling.length}`);
  for (const d of dangling) console.log(`   ${d}`);
  console.log(`\n④ utils/ 下零引用的 .uts（映射为空；全量跑对它们同样没有信息量）：${orphans.length}`);
  for (const o of orphans) console.log(`   utils/${o}`);
  process.exit(0);
}

if (has('compare-capture')) {
  const capDir = argv[argv.indexOf('--compare-capture') + 1];
  if (!capDir) {
    console.error('用法：--compare-capture <dep-capture 输出目录>');
    process.exit(2);
  }
  const map = buildMap(mobileRoot);
  const NOISE = /^(package\.json|package-lock\.json|\.browserslistrc|babel\.config\.js|tsconfig\.json|jest\.config\.)/;
  // 只比对源码面；配置/锁文件是工具链读的，与套件依赖无关
  const SOURCE = /\.(uts|uvue|js|mjs|ps1|json|md|css|scss)$/;

  const rows = [];
  for (const p of map.profiles) {
    const name = path.basename(p.suite).replace(/\.test\.js$/, '');
    const file = path.join(capDir, `${name}.json`);
    if (!fs.existsSync(file)) {
      rows.push({ suite: p.suite, status: 'no-capture' });
      continue;
    }
    const captured = JSON.parse(fs.readFileSync(file, 'utf8'))
      .filter((f) => SOURCE.test(f) && !NOISE.test(f) && !/^docs\//.test(f));
    const statics = new Set(p.tokens.keys());
    const missed = captured.filter((c) => !statics.has(path.basename(c)));
    rows.push({ suite: p.suite, status: missed.length ? 'MISS' : 'ok', captured: captured.length, missed });
  }

  const noCapture = rows.filter((r) => r.status === 'no-capture');
  const miss = rows.filter((r) => r.status === 'MISS');
  const ok = rows.filter((r) => r.status === 'ok');
  console.log(`# 静态抽取 vs 运行时真读（捕获目录 ${capDir}）\n`);
  console.log(`可比对 ${rows.length - noCapture.length} 个套件：一致 ${ok.length} · 漏边 ${miss.length} · 无捕获 ${noCapture.length}\n`);
  for (const r of miss) {
    console.log(`MISS ${r.suite}  （真读 ${r.captured} 个，静态集漏了 ${r.missed.length} 个）`);
    for (const m of r.missed) console.log(`       ${m}`);
  }
  if (noCapture.length) {
    console.log('\n无捕获（多为 d 类：依赖在子进程里发生，进程内捕获看不到）：');
    for (const r of noCapture) console.log(`   ${r.suite}`);
  }
  process.exit(miss.length ? 1 : 0);
}

/**
 * 健全性验证：**「若这个文件改了，这个套件会不会被选中？」**
 *
 * 这是映射方案唯一真正要回答的问题（比「静态 token 里有没有这个文件名」更重要）：
 * 静态集漏了什么无所谓，只要**闭包边 / 目录边**能把它补回来。
 * 输入是 dep-capture 的真读集，逐条 (套件, 依赖) 反查 select()。
 */
if (has('verify-capture')) {
  const capDir = argv[argv.indexOf('--verify-capture') + 1];
  if (!capDir) {
    console.error('用法：--verify-capture <dep-capture 输出目录>');
    process.exit(2);
  }
  const map = buildMap(mobileRoot);
  // 工具链读的配置文件不算套件依赖（babel 对每个套件都读一遍，与改动相关性无关）
  const NOISE = /^(package\.json|package-lock\.json|\.browserslistrc|babel\.config\.js|tsconfig\.json)$/;

  const cases = [];
  for (const p of map.profiles) {
    const name = path.basename(p.suite).replace(/\.test\.js$/, '');
    const file = path.join(capDir, `${name}.json`);
    if (!fs.existsSync(file)) continue;
    for (const dep of JSON.parse(fs.readFileSync(file, 'utf8'))) {
      if (NOISE.test(dep) || !/\.(uts|uvue|js|mjs|ps1|json|md)$/.test(dep)) continue;
      cases.push({ suite: p.suite, dep });
    }
  }

  const deps = [...new Set(cases.map((c) => c.dep))];
  const sel = new Map();
  for (const d of deps) sel.set(d, new Set(select(map, [d]).subset));

  const bad = cases.filter((c) => !sel.get(c.dep).has(c.suite));
  const pairs = cases.length;

  // 按套件汇总：边级会被「读全部套件」的那一个套件主导（jestSplitContract 占 200 条边），
  // 套件级才说明「有多少个套件是真健全的」。
  const bySuite = new Map();
  for (const p of map.profiles) bySuite.set(p.suite, { total: 0, miss: 0 });
  for (const c of cases) {
    const b = bySuite.get(c.suite);
    b.total += 1;
    if (!sel.get(c.dep).has(c.suite)) b.miss += 1;
  }
  const withDeps = [...bySuite.entries()].filter(([, v]) => v.total > 0);
  const sound = withDeps.filter(([, v]) => v.miss === 0);
  const vacuum = [...bySuite.entries()].filter(([, v]) => v.total === 0).map(([k]) => k);

  console.log(`# 健全性验证（${pairs} 条「套件 ← 真读依赖」边，${deps.length} 个不同依赖文件）\n`);
  console.log(`边级：若依赖改动，套件被选中 ${pairs - bad.length}/${pairs} = ${fmtPct(pairs - bad.length, pairs)}`);
  console.log(
    `套件级：${sound.length}/${withDeps.length} 个套件**全部**依赖都能被选中 = ${fmtPct(sound.length, withDeps.length)}`
  );
  console.log(`捕获真空（读发生在子进程里，本证据对它们无效）：${vacuum.length}\n`);
  for (const b of bad) console.log(`  漏选  ${b.suite}\n        ← 依赖 ${b.dep}`);
  if (vacuum.length) {
    console.log('\n真空套件（靠「门脚本改动 ⇒ 全跑」兜底，不能宣称被本验证覆盖）：');
    for (const v of vacuum) console.log(`  ${v}`);
  }
  process.exit(bad.length ? 1 : 0);
}

/**
 * 批量：`--batch <json>`，输入 `[{id, files:[…]}, …]`，一次建图算多个改动集的子集。
 * 用于「拿历史改动回归一遍」——每次起进程都要重建映射，批量才测得动。
 */
if (has('batch')) {
  const file = argv[argv.indexOf('--batch') + 1];
  if (!file) {
    console.error('用法：--batch <json 文件，[{id, files}]>');
    process.exit(2);
  }
  const cases = JSON.parse(fs.readFileSync(file, 'utf8'));
  const map = buildMap(mobileRoot);
  const total = map.profiles.length;
  console.log(`# 批量子集（${cases.length} 个改动集，行为腿全量 ${total} 个套件）\n`);
  console.log('case\t改动数\t子集\t占比');
  const sizes = [];
  for (const c of cases) {
    const { subset } = select(map, c.files, { bprimeAlways: !has('no-bprime-always') });
    sizes.push(subset.length);
    console.log(`${c.id}\t${c.files.length}\t${subset.length}\t${fmtPct(subset.length, total)}`);
  }
  const avg = sizes.reduce((a, b) => a + b, 0) / sizes.length;
  console.log(`\n平均子集 ${avg.toFixed(1)}/${total} = ${fmtPct(avg, total)}`);
  console.log(`中位数 ${sizes.slice().sort((a, b) => a - b)[Math.floor(sizes.length / 2)]}/${total}`);
  console.log(`等于全量（无收益）的改动集：${sizes.filter((s) => s === total).length}/${sizes.length}`);
  process.exit(0);
}

/**
 * `--changed` 的人读报告（#2034）。抽成函数只为「同一份格式只有一个来源」：
 * `--changed` 逐行打它、`--changed-json` 把它整段放进 `explain`（调用方原样打印，
 * 不必在 PowerShell 里再抄一遍格式）。
 */
function formatChangedReport({ changedCount, subset, reasons, uncovered, total }) {
  const lines = [];
  lines.push(`改动 ${changedCount} 个文件 ⇒ 应跑 ${subset.length}/${total} 个行为套件 (${fmtPct(subset.length, total)})`);
  lines.push('');
  for (const s of subset) {
    lines.push(`  ${s}`);
    lines.push(`      ← ${reasons[s].join(' | ')}`);
  }
  if (uncovered.length) {
    lines.push('');
    lines.push('⚠️ 以下改动未命中任何套件（本地不会跑任何东西 —— 确认是否真的没有行为守护覆盖）：');
    for (const u of uncovered) lines.push(`  ${u}`);
  }
  lines.push('');
  lines.push('# 直接喂给 jest：');
  lines.push(`  npx jest --config jest.config.behavior.js --runTestsByPath ${subset.join(' ')}`);
  return lines;
}

if (has('changed') || has('changed-from')) {
  let changed;
  if (has('changed-from')) {
    const src = fs.readFileSync(0, 'utf8');
    changed = src.split('\n').map((s) => s.trim()).filter(Boolean);
  } else {
    changed = argv.slice(argv.indexOf('--changed') + 1).filter((a) => !a.startsWith('--'));
  }
  if (!changed.length) {
    console.error('用法：--changed <file> [<file>…]  或  --changed-from stdin');
    process.exit(2);
  }
  const map = buildMap(mobileRoot);
  const { subset, reasons, uncovered } = select(map, changed, {
    bprimeAlways: !has('no-bprime-always'),
  });
  const report = {
    changedCount: changed.length,
    total: map.profiles.length,
    subset,
    reasons: Object.fromEntries(subset.map((s) => [s, [...reasons.get(s)]])),
    uncovered,
  };
  const explain = formatChangedReport(report);
  if (has('changed-json')) {
    // 机器可读面（#2034）：只有**输出形态**不同，判据与人类输出一字不动。
    // `explain` 与 `--changed` 是同一个数组 ⇒ 脚本不需要自己重排格式，也不会与它漂移。
    console.log(JSON.stringify({ ...report, explain }, null, 2));
  } else {
    console.log(explain.join('\n'));
  }
  process.exit(0);
}

console.error(
  [
    '用法：',
    '  node scripts/behavior-suite-deps.mjs --audit',
    '  node scripts/behavior-suite-deps.mjs --json',
    '  node scripts/behavior-suite-deps.mjs --changed <file> [<file>…]',
    '  node scripts/behavior-suite-deps.mjs --changed-from stdin',
    '  node scripts/behavior-suite-deps.mjs --changed-json --changed <file> [<file>…]',
  ].join('\n')
);
process.exit(2);
