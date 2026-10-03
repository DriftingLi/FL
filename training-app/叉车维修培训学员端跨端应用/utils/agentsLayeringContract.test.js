/**
 * 移动端 `AGENTS.md` 的「根线式分层」守护（#1509）
 *
 * **要防的那件事**：常驻文件（每次会话都被整份注入的 `AGENTS.md`）把**按需读的参考表**、
 * 以及**别的文件里已经存在的同源副本**抄在身上 ⇒ 它一年比一年长，而长出来的部分全是漂移源。
 * 根 `AGENTS.md` 自己写过这条判据：「本处**不重复那些表**——它们靠指路而非抄写，
 * 抄过来正是历史上漂移的来源」。本守护把「根线式」钉成七条**跨真源**的不变式：
 *
 *   L1 常驻预算  —— `AGENTS.md` 长度 ≤ `residentMax`，**只降不升**的棘轮（同 guardClassification H2 的写法）
 *   L2 不抄写    —— `AGENTS.md` 不得含与根 `AGENTS.md` / 根 `docs/agents/*.md` **逐字相同**的长行；
 *                   例外只有 `DUP_ALLOWED` 那几条「指向同一真源的指针句」，且登记必须精确
 *                   （多抄一条即红、登记的行不再重复也即红）。先例：ADR-0020 ④-6（同源模板残骸）
 *   L3 命令必注册 —— `AGENTS.md` 点名的每条 `npm run X` 必须真存在于某个 `package.json` 的 scripts 里
 *                   （移动端自己的或 `frontend/` 的——本文档会点到别的栈的命令）。
 *                   先例：ADR-0020 ④-6 校正的「**本项目没有这个脚本**」那类残骸
 *   L4 引用可达  —— 活文件（`scripts/` 与 `scripts/lib/` 的 .ps1、`utils/` 的 .test.js、
 *                   `docs/agents/` 的 .md）里
 *                   「`AGENTS.md`「小节名」」形式的引用，其小节名必须仍是某侧 `AGENTS.md` 的标题。
 *                   依据本仓约定「引用技能文档引小节名不引行号」⇒ 小节名就是锚点，改名或搬走即悬空。
 *                   **不扫 `docs/adr/**`、`docs/verification/**` 与 `*.uvue`/`*.uts` 注释**：那些按
 *                   #927 收口时立的规矩属**历史记录，一律不改**（拿不可改的历史当判据 = 判据失效）
 *   L5 指针必解析 —— markdown 相对链接必须指向存在的文件；裸写的 `docs/agents|adr/*.md` 必须在
 *                   移动端树或仓根存在（ADR 命名规约里的 `NNNN-中文标题.md` 是**占位符**，不算引用）。
 *                   先例：#1485（仓内留下指向已删内容的悬空引用）
 *   L6 搬走锚点  —— 被搬进按需层的判据，必须真能在其新家里 grep 到（登记在 `MOVED_ANCHORS`）。
 *                   依据 #1509 验收 ④：「被搬走的每条判据在其新文件里可 grep 到」——登记失守 = 指针指向空地址。
 *   L7 别条线路径 —— 移动端 `AGENTS.md` 不得含别条线（Web / 后端 / 运维）的路径与工具名
 *                   （`FOREIGN_PATHS` = `frontend/`、`src/components/ui`、`gofmt`、`docker-compose`、`vue-tsc`，
 *                   票面点名）。先例：#1509「删作失真」——旧文件那些路径在本仓根本不存在，留着既占量又是误导。
 *
 * **形状**沿用本仓既有守护（见 utils/hxRunContract.test.js、utils/guardClassification.test.js）：
 * 先对**合成样本**注入违规、断言检测有效（防空跑假绿），再对真实文件断言零命中。
 * 合成样本是手写的，不从真文件派生期望值 —— 否则就是拿被测物算被测物的镜像断言。
 *
 * **分类**：接线守护（断言「文档 ↔ 清单/文件树」的一致性，不执行任何被测物）⇒ **不构成 ③ 行为证据**。
 */

const fs = require('fs');
const path = require('path');

/** 读文本一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const SELF = path.join(__dirname, 'agentsLayeringContract.test.js');

/** L1 预算：2026-10-03 实测常驻 30,795 字符（票 #1509 的目标是砍到 ≤16,000）。棘轮只许往下调。 */
const RESIDENT_MAX = 16000;

/** L2 只认「长行」——短行（表头分隔线、列表符号）逐字相同不构成抄写。 */
const DUP_MIN_LEN = 20;

/**
 * L2 例外登记：与根层逐字相同**且应当**逐字相同的指针句（一句话把读者送去真源）。
 * 判据是「它是路由、不是内容」——抄路牌不开第二真源。新增必须写明理由。
 */
const DUP_ALLOWED = [
  'Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.',
  '五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.',
  'AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.',
];

/**
 * L6 被搬走判据的落点登记：每项 = [新家路径, 必须能 grep 到的锚点们]。
 * 锚点取**判据原文片段**（与验收 ④ 的「可 grep 到」同口径），不是标题全称——
 * 标题会被追加日期后缀，片段不会因此失配。
 */
const MOVED_ANCHORS = [
  ['docs/agents/uvue-css.md', ['lang="scss"', '选择器限制（严格）', '`gap` 替换模式', 'UTS 类型系统限制', 'Kotlin 编译期常见错误', '编译验证']],
  ['docs/agents/concurrency.md', ['HX_LOCK_OWNER', 'HX_BUSY wait=', '先写代码、后集中上机', '1800 秒']],
  ['docs/agents/dev-loop.md', ['时间账与三样慢', '三条原因的实测原文', '成本账反模式（2026-10-03 立）', '语义坑位（血账 #917']],
  ['docs/adr/0008-移动端验收门与证据.md', ['islogin', '唯一名']],
];

/** L7 别条线路径黑名单：票 #1509 票面点名的 5 项，逐字。 */
const FOREIGN_PATHS = ['frontend/', 'src/components/ui', 'gofmt', 'docker-compose', 'vue-tsc'];

const HEADING_RE = /^#{1,6}[ \t]+(.+?)[ \t]*$/gm;
const LINK_RE = /\]\(([^)\s#]+)(?:#[^)]*)?\)/g;
const DOC_RE = /docs\/(?:agents|adr)\/[\w\u4e00-\u9fff.\-/]+\.md/g;
const NPM_RE = /npm run ([A-Za-z][\w:\-]*)/g;
/** `\x60?` = 反引号可选（真文里两种写法都有）；「」用码点写，躲开写入链路对字形的破坏。 */
const CITE_RE = /AGENTS\.md\x60?\u300c([^\u300d]+)\u300d/g;

/** 取每条匹配的第 1 捕获组（无组则取整条），去重保序。 */
function uniq(re, text) {
  const out = [];
  for (const m of text.matchAll(new RegExp(re.source, 'gm'))) {
    const v = m[1] !== undefined ? m[1] : m[0];
    if (!out.includes(v)) out.push(v);
  }
  return out;
}

/**
 * 纯函数：吃一份 sources，吐违规清单（`'L1: …'`～`'L7: …'`）。不做 IO，好让测试喂合成样本。
 * sources = { agents, rootAgents, rootDocs, scripts, cites, residentMax, dupAllowed,
 *             movedAnchors, readMoved(rel), foreignPaths,
 *             existsMobile(rel), existsRepo(rel) }
 */
function scan(s) {
  const v = [];
  const allowed = s.dupAllowed || DUP_ALLOWED;

  // ---- L1 常驻预算
  if (s.agents.length > s.residentMax) {
    v.push(`L1: 常驻超限 ${s.agents.length} > 预算 ${s.residentMax}（把参考表与同源副本挪去按需读的那一层）`);
  }

  // ---- L2 不抄写
  const origin = new Map();
  for (const [name, text] of Object.entries(s.rootDocs)) {
    for (const raw of text.split('\n')) {
      const t = raw.trim();
      if (t.length >= DUP_MIN_LEN && !origin.has(t)) origin.set(t, name);
    }
  }
  const dup = new Set();
  for (const raw of s.agents.split('\n')) {
    const t = raw.trim();
    if (t.length >= DUP_MIN_LEN && origin.has(t)) dup.add(t);
  }
  for (const t of dup) {
    if (!allowed.includes(t)) v.push(`L2: 与 ${origin.get(t)} 逐字同行的长句（第二真源）：${t.slice(0, 48)}`);
  }
  for (const t of allowed) {
    if (!dup.has(t)) v.push(`L2: 例外登记失效（该行不再与根层重复，从 DUP_ALLOWED 划掉）：${t.slice(0, 48)}`);
  }

  // ---- L3 命令必注册
  for (const tok of uniq(NPM_RE, s.agents)) {
    if (!s.scripts.includes(tok)) v.push(`L3: 点名了 npm run ${tok}，但没有 package.json 注册它`);
  }

  // ---- L4 引用可达
  const heads = new Set(uniq(HEADING_RE, s.agents));
  const rootHeads = new Set(uniq(HEADING_RE, s.rootAgents));
  for (const c of s.cites) {
    for (const name of uniq(CITE_RE, c.text)) {
      if (!heads.has(name) && !rootHeads.has(name)) {
        v.push(`L4: ${c.rel} 引用 AGENTS.md 的「${name}」，两侧 AGENTS.md 都没有这个小节`);
      }
    }
  }

  // ---- L5 指针必解析
  for (const t of uniq(LINK_RE, s.agents)) {
    if (/^[a-z][a-z0-9+.-]*:\/\//i.test(t) || t.startsWith('mailto:')) continue;
    if (!s.existsMobile(path.posix.normalize(path.posix.join('.', t)))) v.push(`L5: 链接目标不存在：${t}`);
  }
  for (const m of uniq(DOC_RE, s.agents)) {
    if (m.includes('NNNN')) continue; // ADR 命名规约里的占位符，不是引用
    if (!s.existsMobile(m) && !s.existsRepo(m)) v.push(`L5: 点名的文档不存在：${m}`);
  }

  // ---- L6 搬走锚点（#1509 验收 ④ 的机检化）
  for (const [rel, anchors] of s.movedAnchors) {
    const text = s.readMoved(rel);
    if (text === undefined || text === null) {
      v.push(`L6: 按需层落点不存在：${rel}`);
      continue;
    }
    for (const a of anchors) {
      if (!text.includes(a)) v.push(`L6: ${rel} 缺被搬走的锚点「${a}」`);
    }
  }

  // ---- L7 别条线路径
  for (const t of (s.foreignPaths || FOREIGN_PATHS)) {
    if (s.agents.includes(t)) v.push(`L7: 常驻层混入别条线路径「${t}」`);
  }
  return v;
}

// ---------- 真实文件装载（只喂给「真实文件」那组断言） ----------

/** 仓根靠 `.git` 上溯找，不写死层数：工作树里 `../../` 与主树里 `../../` 不是同一个地方。 */
function findRepoRoot(from) {
  let d = from;
  for (let i = 0; i < 6; i += 1) {
    if (fs.existsSync(path.join(d, '.git'))) return d;
    d = path.dirname(d);
  }
  throw new Error('找不到 git 根（L5 要按两层落点解析文档路径）');
}

const REPO = findRepoRoot(ROOT);

function scriptsRegistered() {
  const out = [];
  for (const rel of ['package.json', path.join('frontend', 'package.json')]) {
    for (const base of [ROOT, REPO]) {
      const p = path.join(base, rel);
      if (!fs.existsSync(p)) continue;
      out.push(...Object.keys(JSON.parse(readText(p)).scripts || {}));
    }
  }
  return out;
}

/** L4 的射程 = **活文件**：脚本 / 守护测试 / 移动端按需文档。历史面按头注的规矩排除。 */
function citeSources() {
  const files = [];
  for (const d of ['scripts', path.join('scripts', 'lib'), 'utils', path.join('docs', 'agents')]) {
    let names = [];
    try {
      names = fs.readdirSync(path.join(ROOT, d));
    } catch (e) {
      continue; // 目录不存在 ⇒ 空射程
    }
    for (const n of names) {
      const abs = path.join(ROOT, d, n);
      if (!fs.statSync(abs).isFile() || abs === SELF) continue; // 本文件注释里点名小节，不算外部引用
      if (/\.(ps1|test\.js|md)$/.test(n)) files.push(abs);
    }
  }
  return files.map((abs) => ({ rel: path.relative(ROOT, abs).replace(/\\/g, '/'), text: readText(abs) }));
}

function loadReal() {
  const rootDocs = { 'AGENTS.md': readText(path.join(REPO, 'AGENTS.md')) };
  for (const n of fs.readdirSync(path.join(REPO, 'docs', 'agents'))) {
    if (n.endsWith('.md')) rootDocs[`docs/agents/${n}`] = readText(path.join(REPO, 'docs', 'agents', n));
  }
  return {
    agents: readText(path.join(ROOT, 'AGENTS.md')),
    rootAgents: rootDocs['AGENTS.md'],
    rootDocs,
    scripts: scriptsRegistered(),
    cites: citeSources(),
    residentMax: RESIDENT_MAX,
    movedAnchors: MOVED_ANCHORS,
    readMoved: (rel) => {
      const p = path.join(ROOT, rel);
      return fs.existsSync(p) ? readText(p) : undefined;
    },
    existsMobile: (rel) => fs.existsSync(path.join(ROOT, rel)),
    existsRepo: (rel) => fs.existsSync(path.join(REPO, rel)),
  };
}

// ---------- 合成样本：手写，不从真文件派生期望值 ----------

const SPINE_DUP = 'Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.';
const BRANCH_ONLY = '根层长句甲：这一整句只属于按需层，常驻抄过来就是第二真源。';

function synth(over) {
  return {
    agents: [
      '# spine',
      '## 反模式',
      '正文里跑 npm run gate:a 与 npm run gate:b。',
      '链接 [x](docs/agents/branch.md) 与文档 docs/adr/0001-a.md。',
      SPINE_DUP,
    ].join('\n'),
    rootAgents: '# 根\n## 只属于根的小节\n',
    rootDocs: {
      'AGENTS.md': `# 根\n${SPINE_DUP}\n`,
      'docs/agents/checks.md': `# 检查\n${BRANCH_ONLY}\n`,
    },
    scripts: ['gate:a', 'gate:b'],
    cites: [{ rel: 'scripts/x.ps1', text: '见 `AGENTS.md`「反模式」。\n' }],
    residentMax: 500,
    dupAllowed: [SPINE_DUP],
    movedAnchors: [['docs/agents/branch.md', ['锚点甲']]],
    readMoved: (rel) => (rel === 'docs/agents/branch.md' ? '锚点甲躺在新家里。' : undefined),
    existsMobile: (rel) => ['docs/agents/branch.md', 'docs/adr/0001-a.md'].includes(rel),
    existsRepo: (rel) => rel === 'docs/agents/issue-tracker.md',
    ...over,
  };
}

const REAL = loadReal();
const of = (list, id) => list.filter((x) => x.startsWith(id));

describe('AGENTS.md 分层守护：检测有效性（注入违规必红，防空跑假绿）', () => {
  test('合成样本本身零违规（否则下面那批「变红」全在测空集）', () => {
    expect(scan(synth())).toEqual([]);
  });

  const cases = [
    ['L1', '常驻超预算', (s) => ({ residentMax: 10 })],
    ['L2', '把按需层的长句抄进常驻', (s) => ({ agents: `${s.agents}\n${BRANCH_ONLY}` })],
    ['L2', '登记的指针句不再重复（登记该划掉）', (s) => ({ agents: s.agents.replace(SPINE_DUP, '一句根层没有的话。') })],
    ['L3', '点名了没注册的命令', (s) => ({ scripts: ['gate:a'] })],
    ['L4', '被引用的标题改名或被搬走', (s) => ({ agents: s.agents.replace('## 反模式', '## 注意事项') })],
    ['L4', '新活文件引用了不存在的小节', (s) => ({
      cites: [...s.cites, { rel: 'utils/new.test.js', text: 'AGENTS.md「没这个小节」' }],
    })],
    ['L5', '链接指向不存在的文件', (s) => ({ existsMobile: () => false })],
    ['L5', '点名的按需文档从没创建过', (s) => ({ agents: `${s.agents}\n判据见 docs/agents/never-created.md。` })],
    ['L6', '被搬走的锚点在新家里丢了（落点文件没了）', (s) => ({ readMoved: () => undefined })],
    ['L6', '落点文件在、锚点却不在（搬丢了内容）', (s) => ({ readMoved: () => '这里只剩一段别的文字。' })],
    ['L7', '常驻层混入别条线路径', (s) => ({ agents: `${s.agents}\n入口在 frontend/ 的脚手架里。` })],
  ];

  test.each(cases)('%s：%s', (id, _why, mutate) => {
    const s0 = synth();
    expect(of(scan(synth(mutate(s0))), id).length).toBeGreaterThan(0);
  });

  test('L5：ADR 命名规约的 NNNN 占位符不算悬空引用', () => {
    const s0 = synth();
    expect(of(scan(synth({ agents: `${s0.agents}\nADR 写成 docs/adr/NNNN-中文标题.md。` })), 'L5')).toEqual([]);
  });

  test('L4：根 AGENTS.md 的小节名不属移动端锚点面 ⇒ 不判红', () => {
    expect(of(scan(synth({
      cites: [{ rel: 'scripts/x.ps1', text: '见根 `AGENTS.md`「只属于根的小节」' }],
    })), 'L4')).toEqual([]);
  });

  test('L7：登记黑名单只认票面 5 项，别的词不误伤', () => {
    expect(of(scan(synth({ agents: `${synth().agents}\n提到 golangci-lint 与 Swagger 不算别条线路径。` })), 'L7')).toEqual([]);
  });
});

describe('AGENTS.md 分层守护：真实文件', () => {
  test('L1 常驻预算', () => {
    expect(of(scan(REAL), 'L1')).toEqual([]);
  });

  test('L2 不抄写（例外登记须精确）', () => {
    expect(of(scan(REAL), 'L2')).toEqual([]);
  });

  test('L3 命令必注册', () => {
    expect(of(scan(REAL), 'L3')).toEqual([]);
  });

  test('L4 引用可达', () => {
    expect(of(scan(REAL), 'L4')).toEqual([]);
  });

  test('L5 指针必解析', () => {
    expect(of(scan(REAL), 'L5')).toEqual([]);
  });

  test('L6 搬走锚点（被搬走的判据在其新文件里可 grep 到，#1509 验收 ④）', () => {
    expect(of(scan(REAL), 'L6')).toEqual([]);
  });

  test('L7 别条线路径不进常驻层（#1509「删作失真」）', () => {
    expect(of(scan(REAL), 'L7')).toEqual([]);
  });
});
