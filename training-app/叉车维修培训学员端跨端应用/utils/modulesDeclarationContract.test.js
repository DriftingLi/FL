/**
 * 模块声明面自检（ADR-0023 票 A / epic #1221）
 *
 * 这个套件守的是**声明本身**，不是模块里的业务：`utils/modules.js` 说「模块由哪些文件组成、
 * 预算多少、拆出物接没接线」，本文件负责证明**那份声明与磁盘一致**，而且
 * **这份一致性判据有判别力**（改坏一处必红 —— 只跑通过的那一次不算验收，ADR-0008 的 ③ 判据）。
 *
 * C 组还守第二件事：**存量豁免机制不得被带回来**（#654 删掉的不仅是那份文件，还有那条「允许存在
 * 豁免」的口径；口径活在注释与登记位里，所以锁的是代码面形态而不是文件存在性）。
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
/**
 * 那个常量的名字。⚠️ 标识符一律**拼出来**：本文件要被自己的扫描面扫，
 * 写成连续字面量就是守护命中自己（同一类坑见本仓 `contractTestPatternBehavior` 的 pattern 自指）。
 */
const MECH = 'GUARD_' + 'ALLOWLIST';
/** 豁免机制的登记面形态：`utils/modules.js` 的 `allowlistOwned` 字段（键或赋值都算） */
const OWNED_RE = /allowlistOwned\s*[:=]/;
/** 豁免名单那份文件本身（路径也拼开，理由同上） */
const MECH_FILE = 'utils/' + 'guardAllowlist.js';
/**
 * 被点名禁止的三种形态：**声明它**、**读它那份文件**、**在源码文本里抠它的字面量**。
 *
 * - `NEEDLES` = 判据（扫代码面用的探针）；
 * - `FORMS` = 该形态**真会长什么样**的样本，只喂给 C3 的注入自检。
 *
 * ⚠️ 两者必须分开：C3 若直接把 `NEEDLES[k]` 拼进被测文本，那一步等于断言「文本里含有我刚刚放进去的子串」，
 * **恒过**、换不来任何判别力（#654 首版就栽在这里）。分开之后 C3 多出一条**真可失败**的断言：
 * `FORMS[k]` 必须包含 `NEEDLES[k]` —— 探针被写歪（改成一个现实中不存在的前缀）当场判红。
 */
const NEEDLES = {
  decl: MECH,
  file: 'utils/' + 'guardAllowlist',
  textParse: "indexOf('" + 'const ' + MECH + "'",
};
const FORMS = {
  decl: 'const ' + MECH + ' = { H: new Set([\'api/checkin.uts\']) };',
  file: "require('./" + 'guardAllowlist' + "'); // 见 utils/" + 'guardAllowlist' + '.js',
  textParse: "if (src.indexOf('" + 'const ' + MECH + "') >= 0) throw new Error('dup');",
};
/** 判据本体：给定「文件 → 代码面」表与探针，返回命中文件。C2 走真表、C3 走污染表，**同一条判据**。 */
const hitsIn = (codeMap, needle) => [...codeMap.keys()].filter((f) => codeMap.get(f).includes(needle));
/** 命中 needle 的文件（只看代码面，注释不算） */
const hitting = (needle) => hitsIn(SCAN_CODE, needle);
/** 判别力自检用的**必不红对照**：合法的分写形态（本文件自己就是这种写法）不得被探针抓到 */
const SPLIT_FORM = "'GUARD_' + 'ALLOWLIST'";
/**
 * 取守护脚本里某条规则的用例体（`it('F：…` 到该 `it` 的收尾 `});`）。
 * ⚠️ 终止符必须显式验：`slice` 拿到 `-1` 会静默扫到文件末尾 —— 那是本仓点过名的老坑
 * （ADR-0023 决策 ⑧ 当年正是「抠源码文本」的形态栽在这上面），判据会宽到看不见。
 */
function ruleBody(src, id) {
  const start = src.indexOf("it('" + id + "：");
  if (start < 0) throw new Error('rule ' + id + ' 的用例体找不到');
  const terminator = '\n  });';
  const end = src.indexOf(terminator, start);
  if (end < 0) throw new Error('规则 ' + id + ' 的用例体没有收尾终止符');
  return src.slice(start, end + terminator.length);
}

/** 扫描面的代码面全文（只读一次：C 组要在这张表上跑多轮，重复读 200+ 个文件不值当） */
const SCAN_CODE = new Map(SCAN.map((f) => [f, codeOnly(h.read(f))]));

/** 真源对账**只算一次**（每次对账要读全量源码；注入用例各算各的） */
const REAL = h.reconcile();

describe('A. 真源声明与磁盘一致（双向对账 / 跨模块唯一 / 接线 / 消费者 / 预算）', () => {
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

  // A8（豁免归属对账）已随 #654 删除豁免机制一并摘除；A9 起编号不重排 ——
  // 历史 ③ 门输出与 ADR 引用里的用例名要继续查得到（同一口径见 ADR-0007「不可改名的用例名」）。
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
      // B10 / B11（两条豁免注入自检）随 #654 删除机制一并摘除；B12 起编号不重排，理由同 A8 处注释。
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

/**
 * C 组 —— **存量豁免机制不得回来**（#654 / T16 的收口锁）。
 *
 * 原 C 组（ADR-0023 决策 ⑧，其「单一声明点」判据已被 #654 的收尾节取代）守的是「那份名单只有一个声明点、
 * 消费方一律 `require` 取用」。名单清空后这条判据**反过来才有牙**：名单一旦为空，17 份模块契约里
 * 那 27 条「本域不在豁免名单里」的用例声明（三条是 `it.each`，展开 **37 例**）同时变恒真 ——
 * `filter` 命中空名单永远返回 `[]`，与「名单里有但没命中」长得一模一样。于是「机制已退役」不能只靠
 * 删掉那个文件来保证 —— 有人加回一份**空**名单，整套不回潮锁就能静默复活成空判据。
 * 所以这里锁的是**代码面形态**：声明它、读它那份文件、抠它的字面量，三者各自判红；
 * 再加上登记位（`allowlistOwned`）与「规则体内不许有跳过某文件的查询」两格。
 *
 * 判据一律落在**代码面**（`codeOnly`）：注释里提到它不算引用 —— 历史 ADR 与 `navQueryKeyContract`
 * 的撞名说明都还要写这个名字，一条会因为解释自己为什么存在而判红的锁活不过下一次改写。
 * 名字（`MECH` / `NEEDLES` / `FORMS`）在本文件里也是拼出来的，否则守护命中自己。
 *
 * ⚠️ 本组的**残留射程**（有意不覆盖，写明白而不是假装没有）：
 * ① **改名绕开**：另起一套 `EXEMPT_PATHS` / 换一份文件名，C2 的三支探针抓不到。C 组锁的是**这个机制的名字**，
 *    不是「任何形似豁免的东西」—— 后者文本判据做不到，靠评审与注入自检兜。
 * ② 规则体内**手写常量比较**跳过某文件（`if (f === 'x') continue`）不在 C4 的判据里；
 *    但**路径片段式**跳过（`u.file.includes('…')`）自 #654 起被 C4 逐条枚举钉住：全表只许 `app-ios` 一处
 *    （`app-ios` 目录走 Swift 编译，是既有旁路，不是本次新开的豁免面）。
 */
describe('C. 存量豁免机制零声明点（#654 退役后不许回来）', () => {
  /** C3 注入用的靶文件：真实存在、在扫描面内、且不属于任何判据的豁免位 */
  const VICTIM = 'utils/format.uts';
  /** 把 `VICTIM` 在内存里污染成「含该真实形态」，再走**同一条判据**（不落盘 ⇒ 不动工作树） */
  const contaminatedHits = (form, needle) => {
    const polluted = new Map(SCAN_CODE);
    polluted.set(VICTIM, form + '\n' + SCAN_CODE.get(VICTIM));
    return hitsIn(polluted, needle);
  };
  /**
   * 规则体内「按文件跳过」的判据本体（C4 与 C5 共用同一条，注入自检因此真在测判据而不是测自己）。
   * 返回违规清单：空 = 干净。
   */
  function skipViolations(body, id) {
    const v = [];
    for (const m of body.match(/\.has\s*\(/g) || []) v.push('集合式跳过 .has(');
    for (const m of body.matchAll(/\.includes\(\s*'([^']*)'\s*\)/g)) {
      if (id !== 'F' || m[1] !== 'app-ios') v.push(`路径式跳过 includes('${m[1]}')`);
    }
    return v;
  }

  it('C1: 扫描面本身非空且不塌方（防路径断链导致的空集合假绿）', () => {
    // 2026-09-25 现测：整面 409 份、顶层 17 个目录。取整写死阈值：宁可红一次让人重测，不可静默少扫半棵树
    expect(SCAN.length).toBeGreaterThan(300);
    expect(new Set(SCAN.map((f) => f.split(/[\\/]/)[0])).size).toBeGreaterThan(12);
    expect(SCAN).toContain('utils/utsAndroidCompile.test.js');
    expect(SCAN).toContain('utils/modulesDeclarationContract.test.js');
    expect(SCAN).toContain(VICTIM);
  });

  it('C2: 全仓代码面零声明点 —— 声明它 / 读它那份文件 / 抠它的字面量，三种形态各一侧', () => {
    expect(hitting(NEEDLES.decl)).toEqual([]);
    expect(hitting(NEEDLES.file)).toEqual([]);
    expect(hitting(NEEDLES.textParse)).toEqual([]);
    expect(h.exists(MECH_FILE)).toBe(false);
  });

  it.each(Object.keys(NEEDLES))('C3: 判别力 —— 注入「%s」的真实形态必被抓到，合法分写形态必不红', (which) => {
    // ① 探针与真实形态的覆盖关系：探针被写歪（改成现实中不存在的前缀）这一条就先红
    expect(FORMS[which]).toContain(NEEDLES[which]);
    // ② 必红一侧：把**真实写法**植进去，判据必须抓到
    expect(contaminatedHits(FORMS[which], NEEDLES[which])).toContain(VICTIM);
    // ③ 必不红一侧（对照组）：本文件自用的分写形态不得命中，否则是守护抓自己
    expect(contaminatedHits(SPLIT_FORM, NEEDLES[which])).not.toContain(VICTIM);
    expect(hitting(NEEDLES[which])).toEqual([]);
  });

  it('C4: 五条机械坑位规则判的是产物为空，体内既无集合式豁免跳过、路径式跳过也只有 app-ios 一处', () => {
    const src = codeOnly(h.read('utils/utsAndroidCompile.test.js'));
    for (const id of ['F', 'G', 'H', 'I', 'J']) {
      const body = ruleBody(src, id);
      expect(body).toContain('expect(violations).toEqual([])'); // 判据 = 扫描产物
      expect(skipViolations(body, id)).toEqual([]);
    }
  });

  it.each(['集合式', '路径式'])('C5: 判别力 —— 往规则 H 体内注入一条%s跳过，C4 的判据必红', (which) => {
    const src = codeOnly(h.read('utils/utsAndroidCompile.test.js'));
    // 注的是「整行」而不是函数调用片段：植入物得长得像真会提交的那种代码，否则自检只证明替换发生了
    const anchor = '      for (const h of scanCatchAnyParam(u.code)';
    const implant =
      which === '集合式'
        ? '      if (exempt.has(u.file)) continue;\n'
        : "      if (u.file.includes('pages/notifications/notifications')) continue;\n";
    const broken = src.replace(anchor, implant + anchor);
    expect(broken).not.toBe(src); // 锚点真的在（防判据因拼错而空跑）
    expect(skipViolations(ruleBody(broken, 'H'), 'H')).toEqual(
      which === '集合式' ? ['集合式跳过 .has('] : [`路径式跳过 includes('pages/notifications/notifications')`]
    );
  });

  it('C6: 登记位也不许回来：`utils/modules.js` 代码面不含 `allowlistOwned` 字段', () => {
    const decl = codeOnly(h.read('utils/modules.js'));
    expect(decl).not.toMatch(OWNED_RE);
    // 必红一侧：把字段加回一份声明的副本里，判据抓得到（含空数组形态 —— 空名单正是静默复活的样子）
    const broken = decl.replace('maxDepth: MAX_DEPTH,', 'allowlistOwned: [],\n    maxDepth: MAX_DEPTH,');
    expect(broken).not.toBe(decl);
    expect(broken).toMatch(OWNED_RE);
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

/**
 * E 组 —— **全表对账**（票 D #1220）：把「隐形的目录外文件」这条缝堵死。
 *
 * A 组对账的是**模块归属面**（`pages/<键>/**` + 登记的目录外的家）；于是**没进任何表的目录外文件**
 * 在 A 组眼下是隐形的 —— `api/forum.uts` 当年就是这样（654 行、躺在自称达标的 forum 模块里）。
 * E 组要求：**树上的每个源文件，要么归某个模块、要么登记为基础设施**，没有第三种。
 * 基础设施**登记不执法**（行数不进模块预算面），但「谁在表上」这件事本身是硬判据。
 * 判别力同样成对取证（E4–E8：真实声明必不红 + 改坏必红）。
 */
describe('E. 全表对账：每个源文件都有归属（模块 或 基础设施）', () => {
  it('E1: 全表覆盖面非空（模块面 / 基础设施面 / 全树源文件都成规模，防空集合假绿）', () => {
    const onDisk = h.filesUnder('.', /\.(uvue|uts)$/, true);
    const declared = h.moduleKeys().reduce((n, k) => n + h.declaredFiles(k).length, 0);
    const infra = h.infraFiles();
    expect(onDisk.length).toBeGreaterThan(200);
    expect(declared).toBeGreaterThan(150);
    expect(infra.length).toBeGreaterThan(50);
    // 两面加起来的**去重并集**必须恰好等于全树源文件数（既不多也不少）
    expect(new Set([...h.moduleKeys().flatMap((k) => h.declaredFiles(k)), ...infra]).size).toBe(onDisk.length);
  });

  it('E2: 零隐形文件：树上每个源文件都归了模块或登记为基础设施', () => {
    expect(REAL.unregisteredSourceFiles).toEqual([]);
  });

  it('E3: 基础设施登记项都真实存在，且不与模块面重叠（归属唯一）', () => {
    expect(REAL.infraPhantomDirs).toEqual([]);
    expect(REAL.infraPhantomFiles).toEqual([]);
    expect(REAL.infraOverlaps).toEqual([]);
  });

  it('E4: 超预算的跨界文件双向对账（登记不执法，但不许隐形、也不许留过期登记）', () => {
    expect(REAL.infraOversizedDrift).toEqual([]);
    // 事实可见：现在确实有一条（`api/request.uts`）—— 这条断言防「oversized 恒空 ⇒ 判据空跑」
    const facts = h.infraFacts();
    expect(facts.oversized.length).toBeGreaterThan(0);
    expect(facts.oversized.every((f) => h.fileLines(f) > h.BUDGET)).toBe(true);
  });

  const INFRA_INJECTIONS = [
    {
      id: 'E5',
      name: '整目录从基础设施面里漏登记（`utils`）',
      key: 'unregisteredSourceFiles',
      apply: (infra) => { infra.dirs = infra.dirs.filter((d) => d !== 'utils'); },
    },
    {
      id: 'E6',
      name: '幽灵登记：基础设施里写一个不存在的文件',
      key: 'infraPhantomFiles',
      apply: (infra) => { infra.files.push('api/ghost-api.uts'); },
    },
    {
      id: 'E7',
      name: '归属重叠：把一个已归模块的文件也登记成基础设施',
      key: 'infraOverlaps',
      apply: (infra) => { infra.files.push('api/course.uts'); },
    },
    {
      id: 'E8',
      name: '超预算跨界文件漏登记（清空 oversized）',
      key: 'infraOversizedDrift',
      apply: (infra) => { infra.oversized = []; },
    },
    {
      id: 'E9',
      name: '过期登记：oversized 里留一个没超预算的文件',
      key: 'infraOversizedDrift',
      apply: (infra) => { infra.oversized.push('api/auth.uts'); },
    },
    {
      id: 'E10',
      name: '幽灵基础设施目录',
      key: 'infraPhantomDirs',
      apply: (infra) => { infra.dirs.push('utils/ghost-dir'); },
    },
  ];

  it.each(INFRA_INJECTIONS)('$id: $name ⇒ $key 判红（真实声明必不红）', ({ key, apply }) => {
    expect(REAL[key]).toEqual([]); // 必不红一侧
    const brokenInfra = JSON.parse(JSON.stringify(h.INFRA));
    apply(brokenInfra);
    expect(h.reconcile(h.MODULES, brokenInfra)[key].length).toBeGreaterThan(0); // 必红一侧
  });

  it('E11: 漏登记的信息能直接指出是哪个文件（可照抄进声明）', () => {
    const brokenInfra = JSON.parse(JSON.stringify(h.INFRA));
    brokenInfra.dirs = brokenInfra.dirs.filter((d) => d !== 'utils');
    const missing = h.reconcile(h.MODULES, brokenInfra).unregisteredSourceFiles;
    expect(missing).toContain('utils/format.uts');
    expect(missing).toContain('utils/system.uts');
  });
});
