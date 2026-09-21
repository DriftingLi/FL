/**
 * 契约 harness —— 模块结构事实的**纯出口**（ADR-0023 票 A / epic #1221）
 *
 * **只出事实、不做断言**（ADR-0023 ②④）：本文件不 `require('expect')`、不 `throw` 断言，
 * 只把「磁盘上是什么」算成可 `toEqual([])` / `toBeTruthy()` 的形状。
 * 为什么：断言进 harness 会让「为什么红」变成读 harness 源码，而失败信息质量正是本仓的门面；
 * 先例是 `gateCommonLibContract` 的纯函数 `scanContract(sources)`。
 *
 * ## 出口（消费方按需取用）
 *
 * - 路径与读取：`ROOT` / `absOf` / `relOf` / `exists` / `read` / `readText`（读取层归一复用
 *   `utils/utsHarness.js`，**不新写第二份 `normalizeEol`** —— ADR-0019 的硬约束）
 * - 枚举：`filesUnder` / `sourceFilesIn` / `moduleKeys` / `pagesModuleDirs` / `moduleDirs` /
 *   `moduleOnDisk` / `declaredFiles`
 * - 度量：`fileLines` / `moduleDepth`
 * - 接线：`importSpecifiers` / `resolveSpecifier` / `modulePrivateImports` / `orphanExtracts` / `deadImports`
 * - 消费者：`outOfDirFiles` / `consumerIndex` / `measuredConsumers` / `consumerFacts`
 * - 豁免：`allowlistPaths` / `allowlistOwnedBy`
 * - 预算：`BUDGET` / `MAX_DEPTH` / `budgetViolations` / `invalidOverrides`
 * - **一次拿全**：`reconcile(decls?)` —— 上面每一面的违规清单 + `ok`；`decls` 可注入
 *   （注入自检就是这么做的：把声明改坏一处，看对应对账是否判红）
 *
 * ## 两个平台无关的纪律
 *
 * - 所有枚举**排序后**返回：对账结论不得依赖 `readdir` 顺序（Windows / ubuntu 必须相同）。
 * - 不读工作树的 CRLF：文本一律经 `readText` 归一（ADR-0019）。
 */

const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');
const { BUDGET, MAX_DEPTH, MODULES, INFRA } = require('./modules');
const { GUARD_ALLOWLIST, allowlistPaths } = require('./guardAllowlist');

/** 移动端工程根（= `utils/` 的上一级）。与既有契约测试的 `path.join(__dirname, '..')` 同值。 */
const ROOT = path.join(__dirname, '..');

const SOURCE_RE = /\.(uvue|uts)$/;
const TEST_FILE_RE = /\.test\./;
/**
 * 依赖 / 构建产物 / 本地工具目录：枚举一律不进入（否则 `unpackage` 会把预算判红）。
 * ⚠️ 刻意**不含** `uni_modules` —— 它是 vendor 面（实测 5 个源文件），但「全仓只有一个声明点」
 * 这类判据要扫的是**整仓**，漏一个目录就是给假绿留门缝；模块归属面本来也不会指向它。
 */
const SKIP_DIRS = new Set([
  'node_modules', 'unpackage', 'dist', 'hybrid',
  '.git', '.hbuilderx', '.ci-verify', '.scratch', '.vscode', 'coverage',
]);

const absOf = (rel) => path.join(ROOT, rel);
const relOf = (abs) => path.relative(ROOT, abs).split(path.sep).join('/');
const exists = (rel) => fs.existsSync(absOf(rel));
/** 读一个相对路径的源文件（经 0019 的读取层归一） */
const read = (rel) => readText(absOf(rel));

/** 单文件行数。口径 = **总行数**，与既有模块契约的 600 预算一致（见 `modules.js` 文件头）。 */
function fileLines(rel) {
  return read(rel).split('\n').length;
}

/**
 * 目录下的文件（递归、排序稳定、跳过 `SKIP_DIRS` 与点目录）。
 * @param {string} relDir 仓库根下的相对目录（`.` = 整个工程）
 * @param {RegExp} extRe 扩展名筛选
 * @param {boolean} skipTestFiles 是否剔除 `*.test.*`
 */
function filesUnder(relDir, extRe = SOURCE_RE, skipTestFiles = false) {
  const out = [];
  const walk = (dir) => {
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch (e) {
      return;
    }
    for (const e of entries) {
      if (e.name.startsWith('.') || SKIP_DIRS.has(e.name)) continue;
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        walk(p);
        continue;
      }
      if (!extRe.test(e.name)) continue;
      if (skipTestFiles && TEST_FILE_RE.test(e.name)) continue;
      out.push(relOf(p));
    }
  };
  walk(absOf(relDir));
  return out.sort();
}

/**
 * 目录下的源文件（`.uvue` / `.uts`，递归、`*.test.*` 剔除、排序稳定）。
 * 目录不存在 ⇒ 返回 `[]`（与既有各契约测试的 walker 同口径）；「目录该在而不在」
 * 由 `reconcile().missingDirs` 报出来，不在这里抛。
 */
function sourceFilesIn(relDir) {
  return filesUnder(relDir, SOURCE_RE, true);
}

/** 声明的模块键（排序） */
function moduleKeys(decls = MODULES) {
  return Object.keys(decls).sort();
}

/** `pages/` 下的模块目录名（排序）= 模块键的事实来源（ADR-0023 ②：键 = 目录名） */
function pagesModuleDirs() {
  let entries;
  try {
    entries = fs.readdirSync(absOf('pages'), { withFileTypes: true });
  } catch (e) {
    return [];
  }
  return entries
    .filter((e) => e.isDirectory() && !e.name.startsWith('.'))
    .map((e) => e.name)
    .sort();
}

/** 模块归属目录：键目录 `pages/<key>` + 声明的 `extraDirs`（ADR-0023 ②） */
function moduleDirs(key, decls = MODULES) {
  const d = decls[key];
  if (!d) return [];
  return [`pages/${key}`, ...(d.extraDirs || [])];
}

/** 模块归属面的磁盘枚举一侧 */
function moduleOnDisk(key, decls = MODULES) {
  const out = new Set();
  for (const dir of moduleDirs(key, decls)) {
    for (const f of sourceFilesIn(dir)) out.add(f);
  }
  return [...out].sort();
}

/** 模块声明的必需文件一侧（原样、排序） */
function declaredFiles(key, decls = MODULES) {
  const d = decls[key];
  return d ? [...d.files].sort() : [];
}

/** 模块目录内最大相对层数（`pages/<key>/<文件>` = 1，`pages/<key>/<分组>/<文件>` = 2）；目录外的家不参与 */
function moduleDepth(key, decls = MODULES) {
  const base = `pages/${key}/`;
  let max = 0;
  for (const f of moduleOnDisk(key, decls)) {
    if (!f.startsWith(base)) continue;
    const segs = f.slice(base.length).split('/').length;
    if (segs > max) max = segs;
  }
  return max;
}

/**
 * 源码里的 import / export-from 说明符。
 * 逐行取「首个非空白不是注释起始」的行，再匹配 `from '…'` 或裸 `import '…'` ——
 * 于是注释里的 `from './x'` 不会被当接线（本仓的接线守护反复栽在这类误匹配上）。
 */
function importSpecifiers(src) {
  const out = [];
  for (const line of src.split('\n')) {
    const t = line.trim();
    if (t.startsWith('//') || t.startsWith('*') || t.startsWith('/*')) continue;
    for (const m of line.matchAll(/(?:\bfrom|\bimport)\s*['"]([^'"]+)['"]/g)) out.push(m[1]);
  }
  return out;
}

/**
 * 解析一个相对说明符为目标文件的仓库根相对路径（本仓省扩展名，故按候选表试）。
 * 落不到任何存在的候选时**仍返回规范化路径** —— 让「死引用」判据能把它报出来，
 * 而不是静默丢掉（丢掉的接线断点等于假绿）。
 */
function resolveSpecifier(relFrom, spec) {
  if (!spec.startsWith('.')) return null;
  const base = path.posix.normalize(path.posix.join(path.posix.dirname(relFrom), spec));
  if (base.startsWith('..')) return null;
  for (const c of [base, `${base}.uts`, `${base}.uvue`, `${base}/index.uts`, `${base}/index.uvue`]) {
    if (exists(c)) return c;
  }
  return base;
}

/** 模块内源文件 import 到**本模块归属面内**的每一条接线 */
function modulePrivateImports(key, decls = MODULES) {
  const dirs = moduleDirs(key, decls);
  const out = [];
  for (const from of moduleOnDisk(key, decls)) {
    for (const spec of importSpecifiers(read(from))) {
      const resolved = resolveSpecifier(from, spec);
      if (!resolved) continue;
      if (!dirs.some((d) => resolved === d || resolved.startsWith(`${d}/`))) continue;
      out.push({ from, spec, resolved });
    }
  }
  return out;
}

/** 拆出物零孤儿：`extractDirs` 里没被本模块面内任何源文件 import 的文件（ADR-0023 ④ 票 A） */
function orphanExtracts(key, decls = MODULES, precomputedImports) {
  const d = decls[key];
  const extractDirs = (d && d.extractDirs) || [];
  if (extractDirs.length === 0) return [];
  const imports = precomputedImports || modulePrivateImports(key, decls);
  const imported = new Set(imports.map((x) => x.resolved));
  const out = [];
  for (const dir of extractDirs) {
    for (const f of sourceFilesIn(dir)) if (!imported.has(f)) out.push(f);
  }
  return out.sort();
}

/** 死引用：模块内 import 指向本模块面内、但磁盘上不存在的目标 */
function deadImports(key, decls = MODULES, precomputedImports) {
  const imports = precomputedImports || modulePrivateImports(key, decls);
  return imports
    .filter((x) => !exists(x.resolved))
    .map((x) => ({ module: key, from: x.from, spec: x.spec, resolved: x.resolved }))
    .sort((a, b) => (a.from + a.spec).localeCompare(b.from + b.spec));
}

/** 模块「目录外的家」（归属面里不在 `pages/<key>/` 下的部分，如域 api / 共享 composable） */
function outOfDirFiles(key, decls = MODULES) {
  const prefix = `pages/${key}/`;
  return declaredFiles(key, decls).filter((f) => !f.startsWith(prefix));
}

/**
 * 一次遍历建「文件 → 消费它的模块键」索引（对账内部用）。
 * 为什么要有它：逐 owner 各扫一遍全仓是 23 × 全量文件读，票 A 的自检里会被调用几十次；
 * 建成索引后每次对账全文只读一遍。
 */
function consumerIndex(decls = MODULES) {
  const index = new Map();
  for (const key of moduleKeys(decls)) {
    for (const from of moduleOnDisk(key, decls)) {
      const seen = new Set();
      for (const spec of importSpecifiers(read(from))) {
        const resolved = resolveSpecifier(from, spec);
        if (!resolved || seen.has(resolved)) continue;
        seen.add(resolved);
        if (!index.has(resolved)) index.set(resolved, new Set());
        index.get(resolved).add(key);
      }
    }
  }
  return index;
}

/** 实测消费了 owner 目录外件的模块（键，排序） */
function measuredConsumers(owner, decls = MODULES) {
  const owned = outOfDirFiles(owner, decls);
  if (owned.length === 0) return [];
  const index = consumerIndex(decls);
  const out = new Set();
  for (const f of owned) {
    for (const k of index.get(f) || []) if (k !== owner) out.add(k);
  }
  return [...out].sort();
}

/**
 * 消费者对账：登记面（`crossModuleConsumers`）与实测面双向比。
 * - `unregistered`：实测消费了但没登记 ⇒ 判红（新消费者必须登记，否则模块边界静默漂移）
 * - `stale`：登记了但实测不消费 ⇒ 判红（假登记会让「谁在用我的件」变成一份过期名单）
 */
function consumerFacts(decls = MODULES, index = consumerIndex(decls)) {
  const unregistered = [];
  const stale = [];
  for (const key of moduleKeys(decls)) {
    const declared = new Set((decls[key] && decls[key].crossModuleConsumers) || []);
    const measured = new Set();
    for (const f of outOfDirFiles(key, decls)) {
      for (const c of index.get(f) || []) if (c !== key) measured.add(c);
    }
    for (const c of measured) if (!declared.has(c)) unregistered.push({ module: key, consumer: c });
    for (const c of declared) if (!measured.has(c)) stale.push({ module: key, consumer: c });
  }
  unregistered.sort((a, b) => `${a.module}|${a.consumer}`.localeCompare(`${b.module}|${b.consumer}`));
  stale.sort((a, b) => `${a.module}|${a.consumer}`.localeCompare(`${b.module}|${b.consumer}`));
  return { unregistered, stale };
}

/** 归属本模块的豁免文件（原样） */
function allowlistOwnedBy(key, decls = MODULES) {
  const d = decls[key];
  return d ? [...(d.allowlistOwned || [])].sort() : [];
}

/**
 * 预算违规（只对**数字**预算；`'pending'` = 登记不执法，ADR-0023 ②⑥）。
 * 逐文件判定：文件行数 > （`budgetOverrides[file].budget` ?? `budget`）即违规。
 */
function budgetViolations(key, decls = MODULES) {
  const d = decls[key];
  if (!d || typeof d.budget !== 'number') return [];
  const overrides = d.budgetOverrides || {};
  const out = [];
  for (const f of declaredFiles(key, decls)) {
    if (!exists(f)) continue; // 文件不存在由 phantom 面报，不在这里重复
    const ov = overrides[f];
    const cap = ov && typeof ov.budget === 'number' ? ov.budget : d.budget;
    const lines = fileLines(f);
    if (lines > cap) out.push({ module: key, file: f, lines, budget: cap, overridden: Boolean(ov) });
  }
  return out;
}

/** 非法覆盖：缺 `reason`/`issue`、预算不是数字、或指向本模块没声明的文件（ADR-0023 ⑤） */
function invalidOverrides(key, decls = MODULES) {
  const d = decls[key];
  if (!d) return [];
  const declared = new Set(declaredFiles(key, decls));
  const out = [];
  for (const [file, ov] of Object.entries(d.budgetOverrides || {})) {
    const problems = [];
    if (!ov || typeof ov.budget !== 'number') problems.push('预算不是数字');
    if (!ov || !ov.reason) problems.push('缺 reason');
    if (!ov || !ov.issue) problems.push('缺 issue');
    if (!declared.has(file)) problems.push('文件不在本模块声明里');
    if (problems.length > 0) out.push({ module: key, file, problems });
  }
  return out.sort((a, b) => a.file.localeCompare(b.file));
}

/**
 * 基础设施面：声明里 `dirs` 展开成源文件 + `files` 原样（排序、去重）。
 * @param {object} infra 基础设施声明（默认 `INFRA`；注入自检传改坏过的副本）
 */
function infraFiles(infra = INFRA) {
  const out = new Set();
  for (const d of infra.dirs || []) {
    for (const f of sourceFilesIn(d)) out.add(f);
  }
  for (const f of infra.files || []) out.add(f);
  return [...out].sort();
}

/**
 * 全表对账里基础设施那一侧的事实（票 D #1220）。**只出事实**，判红在测试里。
 * @param {object} decls 模块声明集合
 * @param {object} infra 基础设施声明
 * @returns {{dirs:number,files:number,phantomDirs:string[],phantomFiles:string[],overlaps:string[],unregistered:string[],oversized:string[],oversizedDrift:string[]}}
 *   - `unregistered`：树上有、既没归模块也没登记为 infra 的源文件（**这就是「隐形文件」的位置**）
 *   - `overlaps`：既被模块声明、又登记为 infra（归属不唯一）
 *   - `oversized` / `oversizedDrift`：超预算的 infra 文件；以及 `oversized` 登记的**双向**漂移
 */
function infraFacts(decls = MODULES, infra = INFRA) {
  const owned = new Set(moduleKeys(decls).flatMap((k) => declaredFiles(k, decls)));
  const inf = infraFiles(infra);
  const infSet = new Set(inf);
  const onDisk = filesUnder('.', SOURCE_RE, true);
  const oversized = inf.filter((f) => exists(f) && fileLines(f) > BUDGET);
  const declaredOversized = infra.oversized || [];
  return {
    dirs: (infra.dirs || []).length,
    files: inf.length,
    phantomDirs: (infra.dirs || []).filter((d) => !fs.existsSync(absOf(d))).sort(),
    phantomFiles: (infra.files || []).filter((f) => !exists(f)).sort(),
    overlaps: inf.filter((f) => owned.has(f)).sort(),
    unregistered: onDisk.filter((f) => !owned.has(f) && !infSet.has(f)).sort(),
    oversized,
    oversizedDrift: [
      ...oversized.filter((f) => !declaredOversized.includes(f)),
      ...declaredOversized.filter((f) => !oversized.includes(f)),
    ].sort(),
  };
}

/**
 * 一次拿全：声明面自检的全部事实。
 * @param {object} decls 声明集合（默认真源 `MODULES`；注入自检传改坏过的副本）
 * @param {object} infra 基础设施声明（同上，默认真源 `INFRA`）
 * @returns {object} 每一面一个违规清单 + `ok`（`pending` 是事实不是违规）
 */
function reconcile(decls = MODULES, infra = INFRA) {
  const keys = moduleKeys(decls);
  const missing = [];
  const missingDirs = [];
  const phantom = [];
  const depth = [];
  const budget = [];
  const orphan = [];
  const dead = [];
  const allowlistUnregistered = [];
  const allowlistStaleOwned = [];
  const invalid = [];
  const pending = [];
  const owners = new Map();

  // 模块键一级的双向对账：`pages/` 目录枚举 ⊆ 声明（漏登记红）、声明 ⊆ 目录（幽灵声明红）
  const dirs = pagesModuleDirs();
  const undeclaredModules = dirs.filter((d) => !(d in decls));
  const phantomModules = keys.filter((k) => !dirs.includes(k));

  for (const key of keys) {
    for (const dir of moduleDirs(key, decls)) {
      if (!fs.existsSync(absOf(dir))) missingDirs.push({ module: key, dir });
    }

    const declared = new Set(declaredFiles(key, decls));
    for (const f of moduleOnDisk(key, decls)) {
      if (!declared.has(f)) missing.push({ module: key, file: f });
    }
    for (const f of declared) {
      if (!exists(f)) phantom.push({ module: key, file: f });
      if (!owners.has(f)) owners.set(f, []);
      owners.get(f).push(key);
    }

    const actualDepth = moduleDepth(key, decls);
    const capDepth = typeof decls[key].maxDepth === 'number' ? decls[key].maxDepth : MAX_DEPTH;
    if (actualDepth > capDepth) depth.push({ module: key, actual: actualDepth, cap: capDepth });

    for (const v of budgetViolations(key, decls)) budget.push(v);
    if (decls[key].budget === 'pending') pending.push(key);
    invalid.push(...invalidOverrides(key, decls));

    // 本模块的内部接线只解析一次，孤儿与死引用两面共用（自检里 reconcile 会被调用几十次）
    const priv = modulePrivateImports(key, decls);
    for (const f of orphanExtracts(key, decls, priv)) orphan.push({ module: key, file: f });
    dead.push(...deadImports(key, decls, priv));

    const owned = new Set(allowlistOwnedBy(key, decls));
    const surface = declared;
    for (const p of allowlistPaths()) {
      if (surface.has(p) && !owned.has(p)) allowlistUnregistered.push({ module: key, file: p });
    }
    for (const p of owned) {
      if (!allowlistPaths().includes(p)) allowlistStaleOwned.push({ module: key, file: p });
    }
  }

  const duplicates = [...owners.entries()]
    .filter(([, list]) => list.length > 1)
    .map(([file, list]) => ({ file, modules: [...list].sort() }))
    .sort((a, b) => a.file.localeCompare(b.file));

  const consumers = consumerFacts(decls);
  const infrastructure = infraFacts(decls, infra);
  const sortBy = (a, b) =>
    `${a.module}|${a.file || a.consumer || a.dir || ''}`.localeCompare(`${b.module}|${b.file || b.consumer || b.dir || ''}`);

  const report = {
    modules: keys.length,
    moduleDirs: dirs,
    undeclaredModules,
    phantomModules,
    missing: missing.sort(sortBy),
    missingDirs: missingDirs.sort(sortBy),
    phantom: phantom.sort(sortBy),
    duplicates,
    depth: depth.sort(sortBy),
    budget: budget.sort(sortBy),
    orphanExtracts: orphan.sort(sortBy),
    deadImports: dead,
    allowlistUnregistered: allowlistUnregistered.sort(sortBy),
    allowlistStaleOwned: allowlistStaleOwned.sort(sortBy),
    invalidOverrides: invalid.sort(sortBy),
    unregisteredConsumers: consumers.unregistered,
    staleConsumers: consumers.stale,
    pending,
    // 全表对账（票 D）：基础设施那一侧
    infraFiles: infrastructure.files,
    unregisteredSourceFiles: infrastructure.unregistered,
    infraPhantomDirs: infrastructure.phantomDirs,
    infraPhantomFiles: infrastructure.phantomFiles,
    infraOverlaps: infrastructure.overlaps,
    infraOversizedDrift: infrastructure.oversizedDrift,
  };
  report.violations = [
    'undeclaredModules', 'phantomModules', 'missing', 'missingDirs', 'phantom', 'duplicates',
    'depth', 'budget', 'orphanExtracts', 'deadImports', 'allowlistUnregistered',
    'allowlistStaleOwned', 'invalidOverrides', 'unregisteredConsumers', 'staleConsumers',
    'unregisteredSourceFiles', 'infraPhantomDirs', 'infraPhantomFiles', 'infraOverlaps', 'infraOversizedDrift',
  ].filter((k) => report[k].length > 0);
  report.ok = report.violations.length === 0;
  return report;
}

module.exports = {
  // 路径与读取
  ROOT,
  absOf,
  relOf,
  exists,
  read,
  readText,
  // 枚举
  filesUnder,
  sourceFilesIn,
  moduleKeys,
  pagesModuleDirs,
  moduleDirs,
  moduleOnDisk,
  declaredFiles,
  // 度量
  fileLines,
  moduleDepth,
  // 接线
  importSpecifiers,
  resolveSpecifier,
  modulePrivateImports,
  orphanExtracts,
  deadImports,
  // 消费者
  outOfDirFiles,
  consumerIndex,
  measuredConsumers,
  consumerFacts,
  // 豁免
  allowlistPaths,
  allowlistOwnedBy,
  // 预算与总对账
  budgetViolations,
  invalidOverrides,
  // 基础设施（全表对账，票 D）
  infraFiles,
  infraFacts,
  reconcile,
  // 常量透出（消费方不必再 require 两个文件）
  BUDGET,
  MAX_DEPTH,
  MODULES,
  INFRA,
  GUARD_ALLOWLIST,
};
