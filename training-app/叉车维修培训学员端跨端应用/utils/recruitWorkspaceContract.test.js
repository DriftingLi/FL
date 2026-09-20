/**
 * 招聘者工作区骨架与三面契约（ADR-0021 ④ 线 4 / ADR-0022 ④ 步骤 **P2** / issue #1195）
 *
 * 缝：`.uvue` / `.uts` 不能在 jest 里 import（渲染层跑不了、UTS 是方言），沿用**源码契约测试**缝。
 * 但**只钉那些只能靠结构才能钉住的事**（唯一性、挂载数、请求账本、入口集合）；
 * 语义/边界（过期判定、计数、状态投影）由 `utils/recruitDisplayBehavior.test.js`
 * 用 `loadUts` 真跑起来验（源码契约证明「接线没断」，行为测试证明「判得对」，两者都要）。
 *
 * 钉住的判据（票面「判据（断言行为）」逐条）：
 *   A. 三个一级面渲染**同一个**分段件；全仓**仅此一份**；二级面（投递列表、我的）**无**分段；
 *   B. 首屏批量请求 = **2**（`/recruit/jobs` + `/recruit/contact-requests`）；
 *      **不逐职位拉 `unread_count`**（N+1，ADR-0021 ② 禁令）；
 *   C. 交换段：过期由客户端判、**过期项无任何操作入口**；徽标只计未过期 pending（行为在行为测试）；
 *   D. 标记不合适：二次确认 + **不可逆**提示 + 学员侧 30 天冷却；**无任何「撤销」入口**；
 *   E. 移动端**不存在**职位发布/编辑/上下架入口，也**不存在**任何「功能开发中」式占位；
 *   F. 状态词**两端同源**（与 Web 的两张 Record 逐字对账）+ 每个取值在消费面都有 `.tag-<status>`。
 *
 * 另加两组本仓硬约束锁：`pages.json` 注册闭环（无死链 / 无未注册页）与 uvue 语法禁令
 * （class-only 选择器、无 `gap` / `grid` / CSS 变量 / `vh|vw` / `transition`）。
 *
 * @note 读源码一律经 `utils/utsHarness.js` 的 `readText`（ADR-0019 的读取层 EOL 归一），
 *       Windows 检出的 CRLF 不会让多行锚点静默失配。
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const REPO = path.join(ROOT, '..', '..');

const read = (rel) => readText(path.join(ROOT, rel));

// ---- 本票的面 ----
const TAB_BAR = 'pages/recruiter/components/recruiter-tab-bar.uvue';
const PAGE_JOBS = 'pages/recruiter/jobs.uvue';
const PAGE_RESUMES = 'pages/recruiter/resumes.uvue';
const PAGE_CONTACTS = 'pages/recruiter/contacts.uvue';
const PAGE_APPLICATIONS = 'pages/recruiter/applications.uvue';
const PAGE_ME = 'pages/recruiter/me.uvue';
const PAGE_LOGIN = 'pages/recruiter/login.uvue';
const API_RECRUIT = 'api/recruit.uts';
const DISPLAY = 'utils/recruitDisplay.uts';
const GUARD = 'utils/recruitGuard.uts';

/** 一级面（挂分段件，互相 `redirectTo`） */
const FIRST_LEVEL = [PAGE_JOBS, PAGE_RESUMES, PAGE_CONTACTS];
/** 二级面（**无**分段件） */
const SECOND_LEVEL = [PAGE_APPLICATIONS, PAGE_ME];
/** 本票的完整源码面（用于「零内联 / 零入口」这类全量扫描） */
const WORKSPACE_PAGES = [PAGE_LOGIN].concat(FIRST_LEVEL).concat(SECOND_LEVEL);
const SURFACE = WORKSPACE_PAGES.concat([TAB_BAR, API_RECRUIT, DISPLAY, GUARD]);

const NEW_UVUE = [TAB_BAR].concat(FIRST_LEVEL).concat(SECOND_LEVEL);

// ---------------------------------------------------------------------------
// 源码工具（全部先剥注释：注释里写着「不做什么」不算做了）
// ---------------------------------------------------------------------------

const stripHtmlComments = (s) => s.replace(/<!--[\s\S]*?-->/g, ' ');
const stripBlockComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, ' ');
const stripLineComments = (s) => s.replace(/(^|\s)\/\/[^\n]*/g, '$1');
const stripComments = (s) => stripLineComments(stripBlockComments(stripHtmlComments(s)));

/** 模板段（含标签，剥 HTML 注释） */
function templateOf(src) {
  const i = src.indexOf('<template>');
  const j = src.indexOf('</template>');
  if (i === -1 || j === -1) return '';
  return stripHtmlComments(src.slice(i, j));
}

/** `<style ...>` 块正文（数组：一个页面可能有多段 style） */
function styleBlocksOf(src) {
  const out = [];
  const re = /<style([^>]*)>([\s\S]*?)<\/style>/g;
  let m;
  while ((m = re.exec(src)) !== null) out.push({ attrs: m[1], body: m[2] });
  return out;
}

/** 取函数体（`function name(` / `async function name(` 起，按花括号配平） */
function fnBody(src, name) {
  const re = new RegExp('(?:async\\s+)?function\\s+' + name + '\\s*\\(');
  const m = re.exec(src);
  if (!m) return '';
  const open = src.indexOf('{', src.indexOf(')', m.index));
  return balancedFrom(src, open);
}

/**
 * 取 `onLoad(` 回调体 —— 仓内两种形态都要认：
 * `onLoad(function (options) {…})` 与 `onLoad((options : OnLoadOptions) => {…})`（后者是主流，13/20）
 * @note 先在**剥注释**的副本上定位：本票的页面注释里写着 `onLoad(...)`（解释定义顺序契约），
 *       不剥就会命中文档而不是代码 —— 这正是本锁第一版踩到的坑
 */
function onLoadBody(src) {
  const clean = stripComments(src);
  const m = /onLoad\(\s*(?:\(|function)/.exec(clean);
  if (m === undefined || m === null) return '';
  const arrow = clean.indexOf('=>', m.index);
  if (arrow === -1) return fnBody(clean, 'onLoad');
  return balancedFrom(clean, clean.indexOf('{', arrow));
}

/** 从 `open` 处的 `{` 起按花括号配平截出块体 */
function balancedFrom(src, open) {
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  return '';
}

/** 递归收集文件（跳过依赖与构建产物） */
function collectFiles(dir, predicate, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (['node_modules', 'unpackage', '.git', '.ci-verify', 'dist'].includes(e.name)) continue;
      collectFiles(full, predicate, acc);
    } else if (predicate(e.name)) {
      acc.push(full);
    }
  }
  return acc;
}

const relOf = (p) => path.relative(ROOT, p).split(path.sep).join('/');

/** 面内出现的所有 `/pages/recruiter/<x>` 路由（去重排序） */
function recruiterRoutesIn(src) {
  const found = new Set();
  const re = /\/pages\/recruiter\/([A-Za-z0-9_-]+)/g;
  let m;
  while ((m = re.exec(src)) !== null) found.add(m[1]);
  return [...found].sort();
}

/** 面内调用到的 `xxxApi(` 名字集合 */
function apiCallsIn(src) {
  const found = new Set();
  const re = /\b([A-Za-z_][A-Za-z0-9_]*Api)\s*\(/g;
  let m;
  const clean = stripComments(src);
  while ((m = re.exec(clean)) !== null) found.add(m[1]);
  return [...found].sort();
}

/** 解析 `RECRUIT_*_STATUS_DESCRIPTORS` 表体里的行（只认表内行，形态锁死） */
function parseDescriptorRows(src, constName) {
  const clean = stripComments(src);
  const head = clean.indexOf(constName);
  if (head === -1) return null;
  const eq = clean.indexOf('=', head);
  if (eq === -1) return null;
  const open = clean.indexOf('[', eq);
  if (open === -1) return null;
  let depth = 0;
  let body = null;
  for (let i = open; i < clean.length; i++) {
    if (clean[i] === '[') depth++;
    else if (clean[i] === ']') {
      depth--;
      if (depth === 0) {
        body = clean.slice(open + 1, i);
        break;
      }
    }
  }
  if (body === null) return null;
  const rows = [];
  const re = /\{\s*status:\s*'([^']*)',\s*label:\s*'([^']*)'\s*\}\s*as\s+RecruitStatusDescriptor/g;
  let m;
  while ((m = re.exec(body)) !== null) rows.push({ status: m[1], label: m[2] });
  return rows;
}

/**
 * 解析 Web 侧同域单点的描述子表
 * （`frontend/src/utils/contactRequestStatus.ts` / `applicationStatus.ts`：
 *   `const DESCRIPTORS: Record<X, StatusDescriptor> = { key: { label: '…', tone: '…' }, … }`）
 *
 * ⚠️ **为什么不再读 `MyRequests.vue` 的内联 Record**：那两张 `const m: Record<string, string>`
 * 是 #1103 / ADR-0056 §8 收编**之前**的形态，收编后真源已搬到 `frontend/src/utils/*Status.ts`。
 * 本锁最初就按旧形态写，跑出来 `null` —— 正是 fail-closed 该有的表现（真源搬家要改锁，不是删锁）。
 * @returns Map<status, label>；文件/表都解析不到返回 null（判红，不静默放行）
 */
function parseWebDescriptorModule(absPath) {
  if (!fs.existsSync(absPath)) return null;
  const src = readText(absPath);
  const head = src.indexOf('const DESCRIPTORS');
  if (head === -1) return null;
  const eq = src.indexOf('=', head);
  const open = src.indexOf('{', eq);
  if (eq === -1 || open === -1) return null;
  const body = balancedFrom(src, open);
  if (body === '') return null;
  const out = new Map();
  const re = /([A-Za-z_$][\w$]*)\s*:\s*\{\s*label:\s*'([^']*)'/g;
  let m;
  while ((m = re.exec(body)) !== null) out.set(m[1], m[2]);
  return out.size > 0 ? out : null;
}

/**
 * 后端 Go 常量表的行匹配器（行首赋值形态，避开 `==` 比较；与 contributionStatusContract 同法）
 *
 * @param namePrefix 常量名前缀
 * @param typeName 常量**声明的类型名**（给出时按类型精确圈定取值域）
 * @note 为什么需要 `typeName`：同一个前缀下可能住着**另一个取值域**的常量 ——
 *       `contact_authz.go` 里 `ContactGrant` 前缀同时有 `ContactGrantState` 五态与
 *       `ContactGrantSource`（recruiter / application）两值。只按前缀扫会把 source 的两值
 *       算进状态域，对账锁于是凭空多出两个取值（本锁第一版就是这么错的）。
 * @note 形态有两种：带类型声明（`ContactGrantPending ContactGrantState = "pending"`）与不带
 *       （`ApplicationStatusApplied = "applied"`）⇒ 不给 `typeName` 时类型位是可选的
 */
const goStatusRe = (namePrefix, typeName = '') => {
  const typePart = typeName === '' ? '(?:\\w+\\s+)?' : typeName;
  return new RegExp('^\\s*' + namePrefix + '\\w*\\s+' + typePart + '\\s*=\\s*"([a-z_]+)"', 'gm');
};

/** 解析一个 Go 文件里的取值集合；文件/表都解析不到返回 null（判红，不静默放行） */
function parseGoStatuses(absPath, namePrefix, typeName = '') {
  if (!fs.existsSync(absPath)) return null;
  const src = stripComments(readText(absPath));
  const out = [];
  const re = goStatusRe(namePrefix, typeName);
  let m;
  while ((m = re.exec(src)) !== null) out.push(m[1]);
  return out.length > 0 ? out : null;
}

// ---------------------------------------------------------------------------
// A. 分段件的唯一性
// ---------------------------------------------------------------------------

describe('A. 分段件：全仓唯一一份，且只被三个一级面各挂一次', () => {
  it(`${TAB_BAR} 存在`, () => {
    expect(fs.existsSync(path.join(ROOT, TAB_BAR))).toBe(true);
  });

  it('全仓只有一个 recruiter-tab-bar.uvue（ADR-0022 ②「唯一一份」；ADR-0009 的病根是两份手画的栏）', () => {
    const hits = collectFiles(ROOT, (n) => n === 'recruiter-tab-bar.uvue').map(relOf);
    expect(hits).toEqual([TAB_BAR]);
  });

  it('三个一级面各挂一次（挂载数 === 1，不是「至少一次」）', () => {
    for (const rel of FIRST_LEVEL) {
      const n = (templateOf(read(rel)).match(/<RecruiterTabBar/g) || []).length;
      expect([rel, n]).toEqual([rel, 1]);
      expect(read(rel)).toContain("from './components/recruiter-tab-bar.uvue'");
    }
  });

  it('二级面（投递列表、我的）零分段：既不挂组件，也不出现段切换处理器', () => {
    for (const rel of SECOND_LEVEL) {
      const src = read(rel);
      expect([rel, (templateOf(src).match(/<RecruiterTabBar/g) || []).length]).toEqual([rel, 0]);
      expect([rel, src.includes('recruiter-tab-bar')]).toEqual([rel, false]);
      expect([rel, src.includes('onSwitchTab')]).toEqual([rel, false]);
    }
  });

  it('分段件之外没有任何一处挂它（挂载面恰好 = 三个一级面）', () => {
    const all = collectFiles(ROOT, (n) => n.endsWith('.uvue')).map(relOf);
    const mounters = all
      .filter((rel) => templateOf(readText(path.join(ROOT, rel))).includes('<RecruiterTabBar'))
      .sort();
    expect(mounters).toEqual([...FIRST_LEVEL].sort());
  });

  it('分段件的三段 key/label 是本票冻结值，且集中在这一个文件里', () => {
    const src = read(TAB_BAR);
    expect(src).toContain("{ key: 'jobs', label: '职位' }");
    expect(src).toContain("{ key: 'resumes', label: '简历库' }");
    expect(src).toContain("{ key: 'contacts', label: '交换' }");
  });

  it('分段件是纯展示件：零 api/store、零路由调用（切换决策留页面）', () => {
    const src = read(TAB_BAR);
    expect(src).not.toMatch(/import\s*\{[^}]*\}\s*from\s*'\.\.\/\.\.\/(api|stores)\//);
    expect(src).not.toMatch(/uni\.(navigateTo|redirectTo|reLaunch|request)/);
    expect(src).toContain("defineEmits(['switch'])");
  });
});

// ---------------------------------------------------------------------------
// B. 请求账本：首屏恰好 2 个批量请求，禁 N+1
// ---------------------------------------------------------------------------

describe('B. 首屏批量请求 = 2；禁逐职位拉 unread_count（N+1）', () => {
  const jobs = read(PAGE_JOBS);

  it('职位段的取数面恰好是这两个批量端点，各调一次', () => {
    expect(apiCallsIn(jobs)).toEqual(['getRecruitContactRequestsApi', 'getRecruitJobsApi']);
    expect((jobs.match(/getRecruitJobsApi\s*\(/g) || []).length).toBe(1);
    expect((jobs.match(/getRecruitContactRequestsApi\s*\(/g) || []).length).toBe(1);
  });

  it('职位段**不碰**按职位的投递端点、也不出现 unread 字样（JobPostingDTO 没有投递计数）', () => {
    const clean = stripComments(jobs);
    expect(clean).not.toContain('getRecruitJobApplicationsApi');
    expect(clean).not.toContain('/recruit/jobs/');
    expect(clean.toLowerCase()).not.toContain('unread');
  });

  it('按职位的投递端点全仓只有一个消费面（投递列表页）', () => {
    const sources = collectFiles(ROOT, (n) => /\.(uvue|uts)$/.test(n) && !/\.test\./.test(n)).map(relOf);
    const consumers = sources
      .filter((rel) => rel !== API_RECRUIT)
      .filter((rel) => read(rel).includes('getRecruitJobApplicationsApi'))
      .sort();
    expect(consumers).toEqual([PAGE_APPLICATIONS]);
  });

  it('api 层没有「遍历职位再逐个拉」的循环（N+1 的源码形态）', () => {
    const api = stripComments(read(API_RECRUIT));
    // 逐职位拉必然要有一个循环变量当 jobId；本文件里 getRecruitJobApplicationsApi 只有定义处
    expect((api.match(/getRecruitJobApplicationsApi/g) || []).length).toBe(1);
    expect(api).not.toMatch(/for\s*\([^)]*job/i);
  });

  it('徽标与紧迫行共用同一份 contact-requests（不为其中一处再发一次）', () => {
    // 两个函数体里同源：先算 count，再算 soonest，都消费同一个 items 形参
    const countFn = fnBody(jobs, 'applyContactSignals');
    expect(countFn).toContain('unexpiredPendingCount(items, now)');
    expect(countFn).toContain('soonestPendingIndex(items, now)');
    expect((jobs.match(/getRecruitContactRequestsApi\s*\(/g) || []).length).toBe(1);
  });

  it('一级面之间用 redirectTo 替换（不堆栈），二级面用 navigateTo 推入', () => {
    for (const rel of FIRST_LEVEL) {
      const src = read(rel);
      expect([rel, src.includes('uni.redirectTo')]).toEqual([rel, true]);
      // 一级面之间**不得**用 navigateTo（那会把段切换堆进返回栈）
      expect([rel, /uni\.navigateTo\(\{\s*url:\s*'\/pages\/recruiter\/(jobs|resumes|contacts)'/.test(src)]).toEqual([rel, false]);
      // 「我的」是一级面导航栏的常驻入口 → 推入二级面
      expect([rel, src.includes("uni.navigateTo({ url: '/pages/recruiter/me' })")]).toEqual([rel, true]);
    }
    expect(read(PAGE_ME)).toContain('uni.navigateBack()');
    expect(read(PAGE_APPLICATIONS)).toContain('uni.navigateBack()');
  });
});

// ---------------------------------------------------------------------------
// C. 交换段
// ---------------------------------------------------------------------------

describe('C. 交换段：客户端判过期、过期项无操作入口', () => {
  const contacts = read(PAGE_CONTACTS);

  it('过期判定来自 utils/recruitDisplay.uts（本地薄包装，模板不直调 import 函数 —— 守护规则 S）', () => {
    expect(contacts).toContain('contactExpired');
    expect(contacts).toContain('describeRecruitContactStatus');
    expect(contacts).toContain('function contactTagLabel(');
    expect(contacts).toContain('function contactTagClass(');
    expect(templateOf(contacts)).not.toContain('describeRecruitContactStatus(');
    expect(templateOf(contacts)).not.toMatch(/contactExpired\s*\(/);
  });

  it('行内零操作入口：整页只有导航的「我的」一个 @click，没有按钮、没有「标记不合适」', () => {
    const clicks = contacts.match(/@click=/g) || [];
    expect(clicks.length).toBe(1);
    expect(contacts).toContain('@click="onMe"');
    expect(contacts).not.toContain('<button');
    expect(contacts).not.toContain('标记不合适');
    expect(contacts).not.toContain('onReject');
  });

  it('徽标计数走同一个函数（徽标只计未过期 pending 的语义在行为测试里验）', () => {
    expect(contacts).toContain('unexpiredPendingCount(result.items, nowMs.value)');
    expect(contacts).toContain('contactRemainingText(remainingDays(row.expires_at, nowMs.value))');
  });
});

// ---------------------------------------------------------------------------
// D. 标记不合适
// ---------------------------------------------------------------------------

describe('D. 标记不合适：二次确认（不可逆 + 30 天冷却），全仓无撤销入口', () => {
  const apps = read(PAGE_APPLICATIONS);

  it('是一个 uni.showModal 二次确认，而不是直接执行', () => {
    const body = fnBody(apps, 'onReject');
    expect(body).toContain('uni.showModal(');
    expect(body).toContain('success: (res) =>');
    expect(body).toContain('if (res.confirm)');
    // 真正的拒绝调用只允许出现在 confirm 分支（即 doReject 里）
    const between = body.slice(body.indexOf('res.confirm'), body.indexOf('doReject(row)'));
    expect(between).toContain('res.confirm');
    expect(fnBody(apps, 'doReject')).toContain('rejectRecruitApplicationApi(row.id)');
  });

  it('确认文案同时给出**不可撤销**与**学员侧 30 天冷却**（票面两个提示都不能省）', () => {
    const body = fnBody(apps, 'onReject');
    expect(body).toContain('不可撤销');
    expect(body).toContain('30 天');
    expect(body).toContain('标记为不合适');
  });

  it('全仓无「撤销 / 恢复」入口：`撤销` 的每一次出现都必须是否定式「不可撤销」', () => {
    const offenders = [];
    for (const rel of SURFACE) {
      const clean = stripComments(read(rel));
      const all = clean.match(/撤销/g) || [];
      const negated = clean.match(/不可撤销/g) || [];
      if (all.length !== negated.length) offenders.push(`${rel} → 撤销 ${all.length} / 不可撤销 ${negated.length}`);
      // 判据是「有没有回退**入口**」⇒ 扫动作标识符，不扫 `restore` 这个词：
      // `restoreFromStorage()`（P1 登录页的既有调用）与「恢复已删数据」无关，扫它只会误报
      if (/\brevert\b|\bunrevert\b|\bunreject\b|\bundelete\b|\breopen\w*/i.test(clean)) {
        offenders.push(`${rel} → 出现 revert/unrevert/unreject/undelete/reopen`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('api 层的写操作恰好是两个具名 POST（reject + 发起交换），没有第三个写端点', () => {
    // ⚠️ 收口（#1195 + #1196 叠加）后本断言由「恰好 1 个」改成「恰好 2 个」——这是**事实修正**，
    // 不是放宽：P3（#1196）的「发起交换」`POST /recruit/contact-requests` 是本文件里第二个
    // 合法写端点（两票并行各自写了自己的 `api/recruit.uts`，合一后两个都必须在）。
    // 强制的部分（**没有多余的写端点 / 没有回退端点 / 没有 PUT|DELETE|PATCH**）一条未减，
    // 且两条端点路径都逐字钉死 —— 笔误端点名或端点搬家都会被判红。
    const api = stripComments(read(API_RECRUIT));
    const posts = api.match(/\bpost\s*\(/g) || [];
    expect(posts.length).toBe(2);
    // P2 的 reject：终态、且后端**无回退端点** ⇒ 全仓只有这一个写它的地方
    expect(api).toContain("'/recruit/applications/' + applicationId.toString() + '/reject'");
    // P3 的发起交换（另见 `recruiterResumeBehavior` D1 的运行期断言）
    expect(api).toContain("post('/recruit/contact-requests', payload)");
  });

  it('标记不合适的前置是 status == applied（后端只允许 applied 被拒）', () => {
    expect(templateOf(apps)).toContain(`v-if="row.status == 'applied'"`);
  });
});

// ---------------------------------------------------------------------------
// E. 无职位发布入口 / 无「功能开发中」占位
// ---------------------------------------------------------------------------

describe('E. 移动端不存在职位发布入口，也不存在「功能开发中」式占位', () => {
  it('招聘者面零「功能开发中 / 敬请期待 / UNAVAILABLE_TOAST」', () => {
    const offenders = SURFACE.filter((rel) => {
      const clean = stripComments(read(rel));
      return clean.includes('功能开发中') || clean.includes('敬请期待') || clean.includes('UNAVAILABLE_TOAST');
    });
    expect(offenders).toEqual([]);
  });

  it('零发布 / 编辑 / 上下架：面内不出现 toggle-status，也没有职位写端点', () => {
    for (const rel of SURFACE) {
      const clean = stripComments(read(rel));
      expect([rel, clean.includes('toggle-status')]).toEqual([rel, false]);
      expect([rel, clean.includes('发布职位')]).toEqual([rel, false]);
      expect([rel, clean.includes('编辑职位')]).toEqual([rel, false]);
      expect([rel, clean.includes("post('/recruit/jobs'")]).toEqual([rel, false]);
    }
    // api 层只有读端点 + 一个写端点（reject）；PUT / DELETE / PATCH 一个都没有
    const api = stripComments(read(API_RECRUIT));
    expect(api).not.toMatch(/\bput\s*\(/);
    expect(api).not.toMatch(/\bdel\s*\(/);
    expect(api).not.toMatch(/\bpatch\s*\(/);
    expect(api).not.toMatch(/\/recruit\/jobs'/);
  });

  it('移动端无任何职位编辑路由（与 worker/recruiter 路由集对比：路由名一眼可见）', () => {
    const routes = [];
    for (const rel of WORKSPACE_PAGES.concat([GUARD])) {
      for (const r of recruiterRoutesIn(read(rel))) routes.push(r);
    }
    // ⚠️ 收口（#1195 + #1196 叠加）后本集合由 6 个变成 7 个：`resume-detail` 是 P3 的**二级面**
    // （简历详情，被简历库的列表项 `navigateTo` 推入），它把「简历库」这一级接成了一个完整的面。
    // **判据方向未变**：仍然是「面内出现的招聘者路由**逐个列名**、不许多出编辑/发布类路由」——
    // 加进白名单的是这个已被 ADR-0022 ④ 步骤 P3 明文授权的详情面，不是放宽。
    expect([...new Set(routes)].sort()).toEqual(['applications', 'contacts', 'jobs', 'login', 'me', 'resume-detail', 'resumes']);
  });
});

// ---------------------------------------------------------------------------
// F. 状态词两端同源 + 消费面 class 覆盖
// ---------------------------------------------------------------------------

const CONTACT_ROWS = parseDescriptorRows(read(DISPLAY), 'RECRUIT_CONTACT_STATUS_DESCRIPTORS');
const APPLICATION_ROWS = parseDescriptorRows(read(DISPLAY), 'RECRUIT_APPLICATION_STATUS_DESCRIPTORS');
const JOB_ROWS = parseDescriptorRows(read(DISPLAY), 'RECRUIT_JOB_STATUS_DESCRIPTORS');

describe('F. 状态词单点 + 两端同源（ADR-0018 口径）', () => {
  const WEB_CONTACT = path.join(REPO, 'frontend/src/utils/contactRequestStatus.ts');
  const WEB_APPLICATION = path.join(REPO, 'frontend/src/utils/applicationStatus.ts');
  const GO_CONTACT = path.join(REPO, 'backend/internal/service/contact_authz.go');
  const GO_APPLICATION = path.join(REPO, 'backend/internal/service/job_application_service.go');

  it('三张表都解析得出来且非空（fail-closed：解析不到 = 锁失效，不是通过）', () => {
    expect(CONTACT_ROWS).not.toBeNull();
    expect(APPLICATION_ROWS).not.toBeNull();
    expect(JOB_ROWS).not.toBeNull();
    expect(CONTACT_ROWS.length).toBe(5);
    expect(APPLICATION_ROWS.length).toBe(3);
    expect(JOB_ROWS.length).toBe(3);
  });

  it('对账源本身可解析（Web 已把这两域收编成 `frontend/src/utils/*Status.ts` 单点 —— 搬家要改锁，不要删锁）', () => {
    const web = parseWebDescriptorModule(WEB_CONTACT);
    expect(web).not.toBeNull();
    expect(web.size).toBe(5);
    const webApp = parseWebDescriptorModule(WEB_APPLICATION);
    expect(webApp).not.toBeNull();
    expect(webApp.size).toBe(3);
    expect(parseGoStatuses(GO_CONTACT, 'ContactGrant', 'ContactGrantState')).not.toBeNull();
    expect(parseGoStatuses(GO_APPLICATION, 'ApplicationStatus')).not.toBeNull();
  });

  it('对照①：交换申请取值集合 == 后端 Go 常量表（漏一个取值即红；`ContactGrantSource` 两值不属状态域）', () => {
    const all = parseGoStatuses(GO_CONTACT, 'ContactGrant');
    // 先钉住「前缀下确实住着第二个取值域」这个前提，否则下面按类型圈定的写法就是多余的
    expect(all).toContain('recruiter');
    const go = [...new Set(parseGoStatuses(GO_CONTACT, 'ContactGrant', 'ContactGrantState'))].sort();
    expect(go).toEqual(['approved', 'expired', 'pending', 'rejected', 'revoked']);
    expect(CONTACT_ROWS.map((r) => r.status).sort()).toEqual(go);
  });

  it('对照①：投递取值集合 == 后端 Go 常量表', () => {
    const go = [...new Set(parseGoStatuses(GO_APPLICATION, 'ApplicationStatus'))].sort();
    expect(go).toEqual(['applied', 'rejected', 'withdrawn']);
    expect(APPLICATION_ROWS.map((r) => r.status).sort()).toEqual(go);
  });

  it('对照②：交换申请 label 与 Web 单点逐字相等（两端同源）', () => {
    const web = parseWebDescriptorModule(WEB_CONTACT);
    expect(CONTACT_ROWS.map((r) => `${r.status}:${r.label}`)).toEqual(
      CONTACT_ROWS.map((r) => `${r.status}:${web.get(r.status)}`)
    );
  });

  it('对照②：投递 label 与 Web 单点逐字相等（两端同源）', () => {
    const web = parseWebDescriptorModule(WEB_APPLICATION);
    expect(APPLICATION_ROWS.map((r) => `${r.status}:${r.label}`)).toEqual(
      APPLICATION_ROWS.map((r) => `${r.status}:${web.get(r.status)}`)
    );
  });

  it('每个取值在消费面都有 `.tag-<status>` class（含 unknown 兜底）', () => {
    const pairs = [
      [CONTACT_ROWS, PAGE_CONTACTS],
      [APPLICATION_ROWS, PAGE_APPLICATIONS],
      [JOB_ROWS, PAGE_JOBS],
    ];
    const missing = [];
    for (const [rows, page] of pairs) {
      const style = styleBlocksOf(read(page)).map((b) => b.body).join('\n');
      for (const row of rows) {
        if (!style.includes('.tag-' + row.status)) missing.push(`${page} 缺 .tag-${row.status}`);
      }
      if (!style.includes('.tag-unknown')) missing.push(`${page} 缺 .tag-unknown（兜底取值也要有样式）`);
    }
    expect(missing).toEqual([]);
  });

  it('词表取值不回流消费面：招聘者页面零内联状态文案（唯一允许的一处见下）', () => {
    const labels = [
      ...CONTACT_ROWS.map((r) => r.label),
      ...APPLICATION_ROWS.map((r) => r.label),
      ...JOB_ROWS.map((r) => r.label),
    ];
    /**
     * 收口（#1195 + #1196 叠加）后的**唯一**豁免：P3 的 `resume-detail.uvue` 里有
     * `'登录已过期，请重新登录'` —— 它是**带鉴权下载**在 401 时的失败文案（登录态过期），
     * 与 `contact grant` 词表的 `expired` 标签「已过期」同字不同义。
     * 豁免按**长串**扣掉这次出现，再逐词扫剩下的文本：这样 ① 该文件里任何**别的**状态词
     * 内联仍然会被判红；② 词表自己漂移（label 不再包含于该串）也不会静默放过。
     */
    const LOGIN_EXPIRY_IN_DETAIL = '登录已过期，请重新登录';
    const offenders = [];
    for (const abs of collectFiles(path.join(ROOT, 'pages/recruiter'), (n) => n.endsWith('.uvue'))) {
      const rel = relOf(abs);
      const raw = stripComments(readText(abs));
      const clean = rel === 'pages/recruiter/resume-detail.uvue'
        ? raw.split(LOGIN_EXPIRY_IN_DETAIL).join('')
        : raw;
      for (const label of labels) {
        if (clean.includes(label)) offenders.push(`${rel} → ${label}`);
      }
    }
    // 允许的一处（P2 自有）：投递列表的确认框必须原样说出学员会收到的结果名「不合适」，
    // 而它同时是投递词表里 rejected 的 label —— 同一事实、同一措辞，不是漂移。
    expect(offenders).toEqual([`${PAGE_APPLICATIONS} → 不合适`]);
    // 豁免的前提现测：该长串确实存在（否则上面那次 split 是空操作、豁免形同虚设）
    expect(read('pages/recruiter/resume-detail.uvue')).toContain(LOGIN_EXPIRY_IN_DETAIL);
    expect(CONTACT_ROWS.map((r) => r.label)).toContain('已过期');
  });
});

// ---------------------------------------------------------------------------
// G. pages.json 注册闭环
// ---------------------------------------------------------------------------

describe('G. pages.json：新增路由注册闭环（无死链 / 无未注册页）', () => {
  const pagesJson = read('pages.json');

  it('本票新增的 5 条注册齐备，且历史登录页仍在', () => {
    for (const route of ['jobs', 'resumes', 'contacts', 'applications', 'me', 'login']) {
      expect([route, pagesJson.includes(`"pages/recruiter/${route}"`)]).toEqual([route, true]);
    }
  });

  it('pages/recruiter 下每个页面文件都有对应注册（无死链），每条注册都有文件', () => {
    const files = collectFiles(path.join(ROOT, 'pages/recruiter'), (n) => n.endsWith('.uvue'))
      .filter((abs) => !abs.includes(`${path.sep}components${path.sep}`))
      .map((abs) => relOf(abs).replace(/\.uvue$/, ''));
    const parsed = JSON.parse(pagesJson);
    const registered = parsed.pages
      .map((p) => p.path)
      .filter((p) => p.startsWith('pages/recruiter/'))
      .sort();
    expect(files.sort()).toEqual(registered);
  });

  it('新增注册一律 navigationStyle=custom（本仓 46/48 页的主流做法，ADR-0022 ⑤）', () => {
    const parsed = JSON.parse(pagesJson);
    const mine = parsed.pages.filter((p) => p.path.startsWith('pages/recruiter/'));
    const notCustom = mine.filter((p) => p.style.navigationStyle !== 'custom').map((p) => p.path);
    expect(notCustom).toEqual([]);
  });

  it('没有把 HBuilderX 的 condition 段带进来（本地开发配置，禁止提交）', () => {
    expect(Object.keys(JSON.parse(pagesJson))).not.toContain('condition');
  });
});

// ---------------------------------------------------------------------------
// H. 会话守卫 / 骨架边界 / uvue 硬约束
// ---------------------------------------------------------------------------

describe('H. 会话守卫与骨架边界', () => {
  it('本票 5 个工作区页面 onLoad 都先过守卫，且守卫返回 false 时**立即停止取数**', () => {
    for (const rel of FIRST_LEVEL.concat(SECOND_LEVEL)) {
      const src = read(rel);
      const body = onLoadBody(src);
      expect([rel, body.length > 0]).toEqual([rel, true]);
      // 守卫的**语义**是「onLoad 开头先判，false 即 return 停止取数」。直接内联调用
      // `ensureRecruiterSession()`，或经本页一层命名包装（`guardRecruiter()`）都满足；
      // 但**包装体里必须仍然调用 `ensureRecruiterSession()`** —— 见下面那条等价性断言，
      // 它堵住「包装一个不看会话的弱守卫」这条绕过路径。
      // （简历库一级面在收口后由 P2 壳 + P3 正文合成，其 onLoad 用的是命名包装。）
      const inlineGuard = body.includes('if (!ensureRecruiterSession()) return');
      const wrappedGuard = body.includes('if (!guardRecruiter()) return');
      expect([rel, inlineGuard || wrappedGuard]).toEqual([rel, true]);
      expect([rel, /from '\.\.\/\.\.\/utils\/recruitGuard'/.test(src)]).toEqual([rel, true]);
      expect([rel, src.includes('ensureRecruiterSession')]).toEqual([rel, true]);
    }
  });

  it('守卫不新增 401 出口：不 import request/api，也不认识 HTTP 状态码', () => {
    const guard = stripComments(read(GUARD));
    expect(guard).not.toMatch(/from\s*'\.\.\/(api|stores)\//);
    expect(guard).not.toMatch(/401|statusCode/);
    expect(guard).toContain('isRecruiterActive()');
    expect(guard).toContain('STORAGE_KEY_TOKEN');
  });

  it('命名的页面守卫（如有）必须仍以 `ensureRecruiterSession()` 收口，不是另立一套弱判定', () => {
    // 上一条允许 onLoad 走 `guardRecruiter()` 这层包装；这一条钉住包装体不许**绕开**会话守卫。
    // 现测面：收口后的简历库一级面（P3 的正文并入 P2 的壳时保留了 P3 的命名守卫）。
    const src = read(PAGE_RESUMES);
    if (!src.includes('guardRecruiter')) return;
    const body = fnBody(src, 'guardRecruiter');
    expect(body).toContain('ensureRecruiterSession()');
    // 包装体只允许「角色判定 + 会话守卫」两件事：不得自己认识 HTTP 状态码或清凭据
    expect(body).not.toMatch(/401|statusCode/);
    expect(body).not.toContain('removeStorage');
    expect(apiCallsIn(body)).toEqual([]);
  });

  it('简历库一级面（P2 壳 + P3 正文合一后）：壳的徽标 + 正文的简历域端点都在，且面子面唯一', () => {
    // ⚠️ 本用例在收口前是「本票只落骨架、不碰简历域端点」。收口后 P3 的正文并入同一个面，
    // 那句断言已与事实相反 ⇒ 换成**合一后**的判据：壳（徽标取数）与正文（列表取数）都在，
    // 且没有第二个页面承担简历库这一级。
    const resumesRaw = read(PAGE_RESUMES);
    const resumes = stripComments(resumesRaw);
    // 正文在：简历域列表端点 + 列表渲染 + 「加载更多」
    expect(resumes).toContain('getRecruitResumesApi');
    expect(templateOf(resumesRaw)).toContain('v-for');
    expect(templateOf(resumesRaw)).toContain('@scrolltolower="onLoadMore"');
    // 壳仍在：分段件 + 徽标（ADR-0022 ② 徽标常驻、三段可见）
    expect(templateOf(resumesRaw)).toContain('<RecruiterTabBar');
    expect(apiCallsIn(resumesRaw)).toEqual(['getRecruitContactRequestsApi', 'getRecruitResumesApi']);
    // 徽标与紧迫行共用同一份 ⇒ 本页 `contact-requests` 只拉一次
    expect((resumesRaw.match(/getRecruitContactRequestsApi\s*\(/g) || []).length).toBe(1);
    // 「面子面唯一」：全仓不再有第二个页面承担简历库这一级
    const libraryFaces = collectFiles(path.join(ROOT, 'pages/recruiter'), (n) => n.endsWith('.uvue'))
      .map(relOf)
      .filter((rel) => !rel.includes('/components/'))
      .filter((rel) => templateOf(readText(path.join(ROOT, rel))).includes('<RecruiterTabBar'));
    expect(libraryFaces.sort()).toEqual([...FIRST_LEVEL].sort());
  });

  it('「我的」推入页：复用招聘者专用退出，不用学员语义的 logout()', () => {
    const me = read(PAGE_ME);
    expect(me).toContain('auth.logoutRecruiter()');
    expect(stripComments(me)).not.toContain('auth.logout()');
    expect(read('stores/auth.uts')).toContain('function logoutRecruiter() : void {');
    expect(read('stores/auth.uts')).toContain('clearIdentityForSwitch()');
  });

  it('招聘者登录页落到工作区首页（首页即职位段），不再落到学员 dashboard', () => {
    const login = read(PAGE_LOGIN);
    expect(login).toContain("uni.reLaunch({ url: '/pages/recruiter/jobs' })");
    expect(login).not.toContain("'/pages/dashboard/dashboard'");
  });
});

describe('H2. uvue / UTS 硬约束（本票新增文件）', () => {
  it('每个 .uvue 的 style 都声明 lang="scss"', () => {
    const bad = [];
    for (const rel of NEW_UVUE) {
      const blocks = styleBlocksOf(read(rel));
      expect([rel, blocks.length > 0]).toEqual([rel, true]);
      for (const b of blocks) {
        if (!/lang\s*=\s*"scss"/.test(b.attrs)) bad.push(rel);
      }
    }
    expect(bad).toEqual([]);
  });

  it('样式禁令：无 gap / grid / CSS 变量 / vh|vw / transition|animation', () => {
    const rules = [
      [/(^|[^-])\bgap\s*:/m, 'gap'],
      [/row-gap\s*:|column-gap\s*:/, 'row/column-gap'],
      [/display\s*:\s*grid/, 'grid'],
      [/var\(--/, 'CSS 变量'],
      [/\d\s*(vh|vw)\b/, 'vh/vw'],
      [/transition\s*:|animation\s*:/, 'transition/animation'],
    ];
    const offenders = [];
    for (const rel of NEW_UVUE) {
      const style = styleBlocksOf(read(rel)).map((b) => b.body).join('\n');
      for (const [re, name] of rules) {
        if (re.test(style)) offenders.push(`${rel} → ${name}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('选择器只用 class（不出现 view/text/image/scroll-view/button 这类 tag 选择器）', () => {
    const tag = /(^|[\s,{}])(view|text|image|scroll-view|button|input)(\s*[,{])/;
    const offenders = [];
    for (const rel of NEW_UVUE) {
      for (const b of styleBlocksOf(read(rel))) {
        // 去掉属性值里的内容：只在「选择器行」上判（行首到 { 之间）
        const selectorish = b.body
          .split('\n')
          .map((line) => line.split('{')[0])
          .join('\n');
        if (tag.test(selectorish)) offenders.push(rel);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('UTS 侧禁用写法：无 undefined、无 Record<…> 字面量、无交叉类型', () => {
    const sources = [API_RECRUIT, DISPLAY, GUARD].concat(NEW_UVUE);
    const offenders = [];
    for (const rel of sources) {
      const clean = stripComments(read(rel));
      if (/\bundefined\b/.test(clean)) offenders.push(`${rel} → undefined`);
      if (/Record\s*</.test(clean)) offenders.push(`${rel} → Record<…>`);
      if (/& \{/.test(clean)) offenders.push(`${rel} → 交叉类型`);
    }
    expect(offenders).toEqual([]);
  });

  it('滚动承载：绑了固定视口高度的**页面**必须有 scroll-view（#1134 的同族判据，本票自持一份）', () => {
    // 分段件是 components/ 下的子组件（没有根容器契约，同族守护明确跳过它），故只查页面；
    // P1 的登录页根容器不绑视口高度（本条判据对它不适用），也不在扫描面里。
    for (const rel of FIRST_LEVEL.concat(SECOND_LEVEL)) {
      const tpl = templateOf(read(rel));
      const rootIsViewport = /<view[^>]*:style="\{ height: windowHeight/.test(tpl);
      expect([rel, rootIsViewport, /<scroll-view[^>]*scroll-y/.test(tpl)]).toEqual([rel, true, true]);
    }
  });

  it('api 层数组一律先落 UTSJSONObject[] 再逐项取字段（ADR-0015 实体解码边界）', () => {
    const api = read(API_RECRUIT);
    expect(api).not.toMatch(/as\s+Recruit\w+\[\]/);
    expect((api.match(/as UTSJSONObject\[\]/g) || []).length).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// I. 锁自检（防假绿：检测器本身必须判得出来）
// ---------------------------------------------------------------------------

describe('I. 锁自检：合成违规必须被判出来（否则本文件是空跑）', () => {
  it('分段件唯一性：判据是对全仓 basename 的**精确相等**（`toContain` 那种写法漏掉第二份）', () => {
    const hits = collectFiles(ROOT, (n) => n === 'recruiter-tab-bar.uvue').map(relOf);
    expect(hits).toEqual([TAB_BAR]);
    expect(hits.concat(['pages/other/components/recruiter-tab-bar.uvue'])).not.toEqual([TAB_BAR]);
  });

  it('挂载计数：合成多挂一次，计数从 1 变 2', () => {
    const one = '<template><RecruiterTabBar :active="activeKey" /></template>';
    const two = one + one;
    expect((one.match(/<RecruiterTabBar/g) || []).length).toBe(1);
    expect((two.match(/<RecruiterTabBar/g) || []).length).toBe(2);
  });

  it('请求账本：合成「逐职位拉」的形态会被 api 扫描与消费者计数同时看见', () => {
    const bad = "for (const job of jobs) { getRecruitJobApplicationsApi(job.id) }";
    expect(apiCallsIn(bad)).toContain('getRecruitJobApplicationsApi');
    expect(/for\s*\([^)]*job/i.test(bad)).toBe(true);
  });

  it('撤销入口：合成一个不带「不可」的「撤销」必须被扫出来', () => {
    const bad = stripComments('function onRevert() : void { uni.showToast({ title: "撤销" }) }');
    const all = bad.match(/撤销/g) || [];
    const negated = bad.match(/不可撤销/g) || [];
    expect(all.length).not.toBe(negated.length);
  });

  it('描述子解析器：改散行形态后解析为零（fail-closed，不放宽成模糊匹配）', () => {
    const reformatted = read(DISPLAY).replace("{ status: 'pending',", '{\n    status: "pending",');
    const rows = parseDescriptorRows(reformatted, 'RECRUIT_CONTACT_STATUS_DESCRIPTORS');
    expect(rows.map((r) => r.status)).not.toContain('pending');
  });

  it('Web 描述子解析器：路径不存在时返回 null（锁不静默放行）', () => {
    expect(parseWebDescriptorModule(path.join(REPO, 'frontend/src/utils/__not_exist__.ts'))).toBeNull();
  });

  it('Web 描述子解析器：合成一份单点文件能解析出 label', () => {
    const synth = "const DESCRIPTORS: Record<X, StatusDescriptor> = {\n  pending: { label: '待同意', tone: 'warning' }\n}";
    const head = synth.indexOf('const DESCRIPTORS');
    const body = balancedFrom(synth, synth.indexOf('{', synth.indexOf('=', head)));
    const re = /([A-Za-z_$][\w$]*)\s*:\s*\{\s*label:\s*'([^']*)'/g;
    const out = new Map();
    let m;
    while ((m = re.exec(body)) !== null) out.set(m[1], m[2]);
    expect(out.get('pending')).toBe('待同意');
  });

  it('Go 常量表解析器：两种声明形态（带类型 / 不带类型）都解析得出，且类型可用来圈定取值域', () => {
    const synth = [
      '\tContactGrantPending ContactGrantState = "pending"',
      '\tContactGrantSourceRecruiter ContactGrantSource = "recruiter"',
      '\tApplicationStatusApplied = "applied"',
    ].join('\n');
    const grab = (prefix, typeName) => {
      const re = goStatusRe(prefix, typeName);
      const out = [];
      let m;
      while ((m = re.exec(synth)) !== null) out.push(m[1]);
      return out;
    };
    expect(grab('ContactGrant', 'ContactGrantState')).toEqual(['pending']);
    expect(grab('ContactGrant')).toEqual(['pending', 'recruiter']);
    expect(grab('ApplicationStatus')).toEqual(['applied']);
  });

  it('Go 解析器：前缀过宽会凭空多算取值（本锁第一版正是这样错的，故留这条自检）', () => {
    const synth = '\tContactGrantPending ContactGrantState = "pending"\n\tContactGrantSourceRecruiter ContactGrantSource = "recruiter"\n';
    const grab = (typeName) => {
      const re = goStatusRe('ContactGrant', typeName);
      const out = [];
      let m;
      while ((m = re.exec(synth)) !== null) out.push(m[1]);
      return out;
    };
    expect(grab('ContactGrantState').length).toBe(1);
    expect(grab('').length).toBe(2);
  });

  it('uvue 禁令检测器：合成 gap / grid / vh 样本必须命中', () => {
    const rules = [[/(^|[^-])\bgap\s*:/m, 'gap'], [/display\s*:\s*grid/, 'grid'], [/\d\s*(vh|vw)\b/, 'vh/vw']];
    expect(rules.filter(([re]) => re.test('.a { gap: 12rpx; }\n.b { display: grid; }\n.c { height: 50vh; }')).length).toBe(3);
  });

  it('tag 选择器检测器：合成 view{} 必须命中，class 选择器不命中', () => {
    const tag = /(^|[\s,{}])(view|text|image|scroll-view|button|input)(\s*[,{])/;
    expect(tag.test('\nview { flex: 1; }')).toBe(true);
    expect(tag.test('\n.row-main { flex: 1; }')).toBe(false);
  });
});
