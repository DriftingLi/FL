/**
 * 模块声明面自检（ADR-0023 票 A / epic #1221）
 *
 * 这个套件守的是**声明本身**，不是模块里的业务：`utils/modules.js` 说「模块由哪些文件组成、
 * 预算多少、拆出物接没接线、豁免归谁」，本文件负责证明**那份声明与磁盘一致**，而且
 * **这份一致性判据有判别力**（改坏一处必红 —— 只跑通过的那一次不算验收，ADR-0008 的 ③ 判据）。
 *
 * 守护分类（`node scripts/classify-guards.mjs`）：**接线守护** —— 读源码文本与文件清单，
 * 不执行被测物。所以它**不构成 ③ 门证据**（`docs/agents/guards.md`）；它守的接线是
 * 「模块边界是一份显式声明」，由本套件自己的注入自检兜底行为面（改坏声明 ⇒ 对账判红）。
 *
 * 为什么注入自检写在这里而不是 harness 里：harness **只出事实、不做断言**（ADR-0023 ②④）；
 * 断言留在测试里，于是「为什么红」读的是用例名，不是 harness 源码。
 */
const path = require('path');

const h = require('./contractHarness');
const { MODULES } = h;
const { GUARD_ALLOWLIST, allowlistPaths } = require('./guardAllowlist');
const utsHarness = require('./utsHarness');

/** 声明集合的深拷贝：注入自检用的「改坏版」，绝不动真源 */
const brokenCopy = () => JSON.parse(JSON.stringify(MODULES));

/**
 * 只留**代码面**：砍掉块注释与行注释。
 * 为什么必须这样：判据要落在代码事实上，「注释里提到」不算 —— `scripts/classify-guards.mjs`
 * 第一版就栽在这一条上（9 个纯接线守护因为注释里提到 `.ps1` 被误判成行为守护，见 `docs/agents/guards.md`）。
 * 这里的实现刻意只做「砍注释」这一件事，不尝试理解字符串（本仓这两个文件的字符串里不含 `//` / `/*`）。
 */
function codeOnly(src) {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .split('\n')
    .map((l) => l.replace(/\/\/.*$/, ''))
    .join('\n');
}

/** 全仓扫描面：源码 + 脚本 + 测试（跳过 node_modules / unpackage / .scratch 等） */
const SCAN = h.filesUnder('.', /\.(js|mjs|ts|uts|uvue|ps1)$/);
/** 声明行的形态：`const GUARD_ALLOWLIST = …`（let / var 同罪） */
const DECL_RE = /(?:const|let|var)\s+GUARD_ALLOWLIST\s*=/;
/**
 * 被点名禁止的形态：**在源码文本里定位那个常量**（`indexOf` + 字面量）。
 * ⚠️ 本文件自己要扫这个 needle，所以**拼出来**而不是写成连续字面量 ——
 * 否则守护会命中自己（本仓 `contractTestPatternBehavior` 的 pattern 自指是同一类坑）。
 */
const TEXT_PARSE_NEEDLE = 'indexOf(' + "'const " + 'GUARD_ALLOWLIST' + "'";

/** 扫描面的代码面全文（只读一次：C 组要在这张表上跑多轮，重复读 200+ 个文件不值当） */
const SCAN_CODE = new Map(SCAN.map((f) => [f, codeOnly(h.read(f))]));

/** 真源对账**只算一次**（每次对账要读全量源码；注入用例各算各的） */
const REAL = h.reconcile();

describe('A. 真源声明与磁盘一致（双向对账 / 跨模块唯一 / 接线 / 豁免归属 / 消费者 / 预算）', () => {
  it('A1: 扫描面非空（防空集合假绿：声明 23 个、目录 23 个、枚举到的文件成规模）', () => {
    expect(h.moduleKeys()).toHaveLength(23);
    expect(h.pagesModuleDirs()).toHaveLength(23);
    const onDisk = h.moduleKeys().reduce((n, k) => n + h.moduleOnDisk(k).length, 0);
    const declared = h.moduleKeys().reduce((n, k) => n + h.declaredFiles(k).length, 0);
    expect(onDisk).toBeGreaterThan(100);
    expect(declared).toBeGreaterThan(onDisk); // 声明面含目录外的家（域 api / 共享件）
    // 接线扫描面确实抓到了接线（否则「零孤儿 / 零死引用」是空跑）
    const wiring = h.moduleKeys().reduce((n, k) => n + h.modulePrivateImports(k).length, 0);
    expect(wiring).toBeGreaterThan(20);
    expect(allowlistPaths().length).toBeGreaterThan(0);
  });

  it('A2: 模块键双向对账：`pages/` 目录枚举 ⊆ 声明，声明 ⊆ 目录（漏登记 / 幽灵声明各一侧）', () => {
    expect(REAL.undeclaredModules).toEqual([]);
    expect(REAL.phantomModules).toEqual([]);
  });

  it('A3: 文件双向对账：目录枚举 ⊆ 声明（漏登记）、声明 ⊆ 磁盘（幽灵声明）', () => {
    expect(REAL.missing).toEqual([]);
    expect(REAL.phantom).toEqual([]);
    expect(REAL.missingDirs).toEqual([]);
  });

  it('A4: 跨模块唯一：同一文件不得被两个模块声明（ADR-0023 ⑦）', () => {
    expect(REAL.duplicates).toEqual([]);
  });

  it('A5: 模块目录深度不超过各自声明上限', () => {
    expect(REAL.depth).toEqual([]);
    expect(REAL.moduleDirs.map((k) => h.moduleDepth(k)).every((d) => d <= h.MAX_DEPTH)).toBe(true);
  });

  it('A6: 拆出物零孤儿：`extractDirs` 里每个文件都被模块面内源文件 import', () => {
    expect(REAL.orphanExtracts).toEqual([]);
  });

  it('A7: 模块内零死引用：import 指向的本模块文件都真实存在', () => {
    expect(REAL.deadImports).toEqual([]);
  });

  it('A8: 豁免归属：模块面内出现的 `GUARD_ALLOWLIST` 条目都已登记，登记的都还在名单里', () => {
    expect(REAL.allowlistUnregistered).toEqual([]);
    expect(REAL.allowlistStaleOwned).toEqual([]);
    // 非空性：名单里确实有条目落在模块面内，A8 不是空跑
    const owned = h.moduleKeys().flatMap((k) => h.allowlistOwnedBy(k));
    expect(owned).toEqual(allowlistPaths());
  });

  it('A9: 消费者登记面与实测面一致（新消费者必须登记；不得留假登记）', () => {
    expect(REAL.unregisteredConsumers).toEqual([]);
    expect(REAL.staleConsumers).toEqual([]);
  });

  it('A10: 数字预算的模块全部达标；`pending` 只出现在确有超预算文件的模块', () => {
    expect(REAL.budget).toEqual([]);
    // pending 不是豁免：每个 pending 模块都必须能在磁盘上指出超预算的文件
    const wrong = REAL.pending.filter((key) => {
      const cap = h.BUDGET;
      return !h.declaredFiles(key).some((f) => h.exists(f) && h.fileLines(f) > cap);
    });
    expect(wrong).toEqual([]);
    expect(REAL.pending.length).toBeGreaterThan(0); // 存量未达标确实存在（本票不执法，只登记）
  });

  it('A11: 预算覆盖登记合法：每条都带 budget + reason + issue，且指向本模块声明的文件', () => {
    expect(REAL.invalidOverrides).toEqual([]);
  });

  it('A12: 总对账 `ok`，且违规清单为空（一次拿全的那一面）', () => {
    expect(REAL.violations).toEqual([]);
    expect(REAL.ok).toBe(true);
  });
});

/**
 * 判别力：**成对取证** —— 每条注入都先证明「真实文件必不红」，再证明「改坏必红」。
 * 只跑通过的那一次不算验收（ADR-0008 ③ / `docs/agents/guards.md` 判据 ③）。
 */
describe('B. 判别力：注入违规必须判红（成对取证：改坏必红 + 真实文件必不红）', () => {
  const INJECTIONS = [
    {
      id: 'B1',
      name: '缺登记：从声明删掉一个归属目录下的真实文件',
      key: 'missing',
      apply: (d) => { d.courses.files = d.courses.files.filter((f) => f !== 'pages/courses/course-detail.uvue'); },
    },
    {
      id: 'B2',
      name: '幽灵声明：声明里写一个不存在的文件',
      key: 'phantom',
      apply: (d) => { d.courses.files.push('pages/courses/ghost-page.uvue'); },
    },
    {
      id: 'B3',
      name: '重复声明：同一文件被两个模块声明',
      key: 'duplicates',
      apply: (d) => { d.mall.files.push('api/course.uts'); },
    },
    {
      id: 'B4',
      name: '预算改小到低于实况',
      key: 'budget',
      apply: (d) => { d.courses.budget = 100; },
    },
    {
      id: 'B5',
      name: '模块漏登记：删掉一个模块键',
      key: 'undeclaredModules',
      apply: (d) => { delete d.courses; },
    },
    {
      id: 'B6',
      name: '幽灵模块：声明一个没有对应目录的模块键',
      key: 'phantomModules',
      apply: (d) => { d['ghost-module'] = { ...brokenCopy().courses, files: [] }; },
    },
    {
      id: 'B7',
      name: '目录深度超限：把模块深度上限调小',
      key: 'depth',
      apply: (d) => { d.courses.maxDepth = 1; },
    },
    {
      id: 'B8',
      name: '拆出物孤儿：extractDirs 指向一个本模块没接线的真目录',
      key: 'orphanExtracts',
      apply: (d) => { d.courses.extractDirs = ['components/app-card']; },
    },
    {
      id: 'B9',
      name: '归属目录不存在：extraDirs 写一个幽灵目录',
      key: 'missingDirs',
      apply: (d) => { d.forum.extraDirs = ['pages/forum/ghost-dir']; },
    },
    {
      id: 'B10',
      name: '豁免未登记：模块面内的 `GUARD_ALLOWLIST` 条目被清掉',
      key: 'allowlistUnregistered',
      apply: (d) => { d.forum.allowlistOwned = []; },
    },
    {
      id: 'B11',
      name: '假登记豁免：登记一个不在 `GUARD_ALLOWLIST` 里的文件',
      key: 'allowlistStaleOwned',
      apply: (d) => { d.courses.allowlistOwned = ['api/course.uts']; },
    },
    {
      id: 'B12',
      name: '预算覆盖非法：缺 reason / issue',
      key: 'invalidOverrides',
      apply: (d) => { d.courses.budgetOverrides = { 'pages/courses/courses.uvue': { budget: 700 } }; },
    },
    {
      id: 'B13',
      name: '消费者未登记 / 假登记：登记面与实测面不一致',
      key: 'unregisteredConsumers',
      apply: (d) => { d.courses.crossModuleConsumers = ['exam']; },
    },
  ];

  it.each(INJECTIONS)('$id: $name ⇒ $key 判红（真实文件必不红）', ({ key, apply }) => {
    expect(REAL[key]).toEqual([]); // 必不红一侧（真源只算一次，见文件头 REAL）
    const broken = brokenCopy();
    apply(broken);
    expect(h.reconcile(broken)[key].length).toBeGreaterThan(0); // 必红一侧
  });

  it('B14: 缺登记的报错信息直接指出该加哪一行（模块键 + 文件路径）', () => {
    const broken = brokenCopy();
    broken.courses.files = broken.courses.files.filter((f) => f !== 'pages/courses/course-detail.uvue');
    expect(h.reconcile(broken).missing).toContainEqual({
      module: 'courses',
      file: 'pages/courses/course-detail.uvue',
    });
  });

  it('B15: 假登记消费者同时被两侧抓到（stale + 未登记 exam）', () => {
    const broken = brokenCopy();
    broken.courses.crossModuleConsumers = ['exam'];
    const r = h.reconcile(broken);
    expect(r.staleConsumers).toContainEqual({ module: 'courses', consumer: 'exam' });
    expect(r.unregisteredConsumers).toContainEqual({ module: 'courses', consumer: 'jobs' });
  });
});

describe('C. `GUARD_ALLOWLIST` 单点（ADR-0023 ⑧）', () => {
  it('C1: 扫描面本身非空（防路径断链导致空集合假绿）', () => {
    expect(SCAN.length).toBeGreaterThan(200);
    expect(SCAN).toContain('utils/utsAndroidCompile.test.js');
    expect(SCAN).toContain('utils/guardAllowlist.js');
  });

  it('C2: 全仓只有一个声明点（`utils/guardAllowlist.js`）', () => {
    const decls = SCAN.filter((f) => DECL_RE.test(SCAN_CODE.get(f)));
    expect(decls).toEqual(['utils/guardAllowlist.js']);
  });

  it('C3: 全仓不再有解析该常量源码文本的读取', () => {
    const parsing = SCAN.filter((f) => SCAN_CODE.get(f).includes(TEXT_PARSE_NEEDLE));
    expect(parsing).toEqual([]);
  });

  it('C4: 守护脚本从单点取用（不再自带一份）', () => {
    const src = codeOnly(h.read('utils/utsAndroidCompile.test.js'));
    expect(src).toMatch(/require\(['"]\.\/guardAllowlist['"]\)/);
    expect(DECL_RE.test(src)).toBe(false);
  });

  it('C5: 提到该常量的测试一律从单点 / harness 取用（不再各抠一份文本）', () => {
    const self = ['utils/modulesDeclarationContract.test.js'];
    // 只看**代码面**：注释里提到它（例如「别与 GUARD_ALLOWLIST 混淆」）不算引用
    const testFiles = SCAN.filter((f) => f.endsWith('.test.js') && SCAN_CODE.get(f));
    const mention = testFiles.filter((f) => SCAN_CODE.get(f).includes('GUARD_ALLOWLIST'));
    const offenders = mention.filter((f) => {
      if (self.includes(f)) return false;
      return !/require\(['"]\.\/(guardAllowlist|contractHarness)['"]\)/.test(SCAN_CODE.get(f));
    });
    expect(offenders).toEqual([]);
    expect(mention.length).toBeGreaterThan(0); // 非空性：确实有测试在代码面引用它
    expect(testFiles.length).toBeGreaterThan(50); // 扫描面本身非空
  });
});

describe('D. harness 自己的约束（ADR-0023 ②④ / ⑤）', () => {
  it('D1: `utils/utsHarness.js` 既有导出签名零改动（5 个名字都在且仍是函数）', () => {
    for (const name of ['loadUts', 'importedNames', 'exportedNames', 'normalizeEol', 'readText']) {
      expect(typeof utsHarness[name]).toBe('function');
    }
  });

  it('D2: 读取层归一只有一份：harness 的 readText 就是 0019 那一份（对象同一性）', () => {
    expect(h.readText).toBe(utsHarness.readText);
    expect(h.ROOT).toBe(path.join(__dirname, '..'));
  });

  it('D3: harness 只出事实、不做断言（不 require expect / jest，不出现 expect( 调用）', () => {
    const code = codeOnly(h.read('utils/contractHarness.js'));
    expect(code).not.toMatch(/require\(['"]expect['"]\)/);
    expect(code).not.toMatch(/\bexpect\s*\(/);
    expect(code).not.toMatch(/\bjest\./);
    // 非空性：确实读到了代码面（防空串假绿）
    expect(code.length).toBeGreaterThan(2000);
  });

  it('D4: 声明只放数据（`utils/modules.js` 不读磁盘、不认识 jest）', () => {
    const code = codeOnly(h.read('utils/modules.js'));
    expect(code).not.toMatch(/require\(['"]fs['"]\)/);
    expect(code).not.toMatch(/require\(['"]path['"]\)/);
    expect(code).not.toMatch(/\bexpect\s*\(/);
    expect(code).not.toMatch(/\bfs\./);
  });
});
