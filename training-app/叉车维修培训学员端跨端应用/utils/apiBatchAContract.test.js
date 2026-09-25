/**
 * 域 api 批量收紧 A 契约（refs #652，refactor epic #638 T14）
 *
 * 本票票面写「六个域 api 收紧」，实测收口现状是**只有两域还需要动**，锁按三类事实分别钉：
 *
 * ① 本票新收紧（此前仍是裸 get + 映射内联在出口）：
 *    - api/search.uts：两出口 get(...) .then → getMapped<T> + 内联映射收敛为具名 build*
 *    - api/job.uts：有载荷的 GET（详情 / 列表）→ getMapped<T> + build* 去重；DTO 内联声明迁至 types/job.uts
 * ② 前序票**已经**收紧，本票只做「不回潮」复检（同批次口径齐平，防后续误改）：
 *    - api/points.uts（#709）、api/featured.uts / api/notification.uts（T05 #643，主体锁在 dashboardContract）
 * ③ 票面与实际不符的两项，登记为事实而非按票面补做（避免为凑 AC 造无收益改动）：
 *    - **guide 域根本没有 api 文件**：pages/guide 只有 choose-cert.uvue 一个文件，它的依赖面是
 *      stores/auth + utils/storage + utils/navigation（切证件后 uni.reLaunch 到 dashboard / login），
 *      不触 api 层；后端亦无 guide 端点。⇒ 钉「零 api + 页面零 api import」防隐性回潮，不新建空文件。
 *    - **api 层豁免在本票范围内早已清零**（既成事实，不是本票动作）。票面写的「catch/detail 条目」
 *      = GUARD_ALLOWLIST 的规则 H（`catch (e : any)`）与规则 I（`: any` 参数访问 `.detail`，见
 *      AGENTS.md 坑位表），两者现状：规则 I 键已不存在，规则 H 只剩两条 ——
 *      api/checkin.uts（归 forum）与 pages/notifications/notifications.uvue —— 后者**确属本票六域
 *      之一**（notifications），但它是**页面**豁免，不在这条 AC「api/ 层」的射程内，故六域 **api 文件**
 *      对 allowlist 的净贡献是 0，本票无需删除动作，只钉「五域 api 文件永不出现在豁免名单里」防回潮。
 *      （那两条 H 项实测规则命中数已为 0，属**过期豁免**；删除要动 guardAllowlist.js + modules.js
 *      且跨两个模块，其中 modules.js 的 `allowlistOwned` 与模块归属面同批变 ⇒ 归 **#654**（T16 epic 收尾
 *      的票面就是「删除 allowlist 机制、四条规则无豁免全量执法」），不在本票顺手动。）
 *
 * void 透传出口（reportJobApi / applyJobApi / viewFeaturedContentApi / mark* 等）按 ADR-0007
 * 「两问判据」保持裸 post，不硬套 identity map —— 本锁显式承认其为合法留裸，不算未收紧。
 *
 * .uvue/.uts 无法被 jest 直接 import，沿用仓库既有源码契约缝（先例 pointsRealApiContract / dashboardContract）。
 */
/** harness：读取层归一 + 模块归属面 + 豁免名单单点（本文件不自建 ROOT / read / walker） */
const h = require('./contractHarness');
const read = h.read;
const exists = h.exists;
const allowlistPaths = h.allowlistPaths;

/**
 * 抹掉块注释与行注释（`://` 例外，防误杀 URL 字面量）。
 * 历史说明注释会提到裸 get、.then、.catch 这些「已退役」名词，断言只针对代码本体。
 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/**
 * 「未收紧」的判据形态：`return get|post|patch(...).then((data : UTSJSONObject) : DTO ...`
 * —— 出口拿裸 HTTP 函数 + 内联 `.then` 直接映射成 DTO。收紧后应改走 *Mapped<T>。
 * void 透传的 `.then(() : void => {...})` 无 `(data : UTSJSONObject)` 形参，天然不被本式命中。
 */
const RAW_THEN_TO_DTO = /return\s+(get|post|patch)\([^)]*\)\s*\.then\(\s*\(\s*data\s*:\s*UTSJSONObject\s*\)/;

/**
 * 域 api 里「裸 GET」的判据：`\bget\s*\(`。收紧后六域（有 api 文件的五域）**一条不剩**——
 * 它们所有 GET 都有载荷，全走 getMapped。比 RAW_THEN_TO_DTO 更严：后者只抓「内联 .then 映射成
 * DTO」这一种未收紧形态，`get(url).then(x => x)` 之类从它缝里漏过。
 * 不误伤 getMapped / getJobListApi / getUnreadCountApi（`get` 后紧跟字母而非 `(`）。
 * 裸 `post` 不在靶内：void 透传按「两问判据」合法留裸（见文件头）。
 */
const RAW_GET = /\bget\s*\(/;

/** 出口体内「直接读响应字段」的判据：`data['x']`。收紧后出口只剩一行委托，故应零命中 */
const FIELD_READ = /\bdata\['/;

/**
 * 「DTO 类型经 `types/index` 消费」的判据。写成 `[^}]+` 会让 `import type { }` 这种**空花括号**
 * 也算命中（本文件自检当场抓到），故要求花括号内至少有一个非空白字符。
 */
const TYPES_INDEX_IMPORT = /import\s+type\s*\{\s*[^}\s][^}]*\}\s*from\s*'\.\.\/types\/index'/;

/**
 * 本票六域里**有 api 文件**的五域。第六域 guide 无 api 文件（后端无 guide 端点，
 * 见下方 ③-a 的登记），故不出现在这里的 it.each 里 —— 不是漏项。
 */
const DOMAINS = ['points', 'featured', 'notification', 'search', 'job'];

/** 取某个顶层函数（`marker` 起）的函数体，到下一个行首 `}` 为止 */
function topFn(fileSrc, marker) {
  const start = fileSrc.indexOf(marker);
  expect(start).toBeGreaterThan(-1);
  const end = fileSrc.indexOf('\n}', start);
  expect(end).toBeGreaterThan(-1);
  return fileSrc.slice(start, end);
}

const searchApi = stripComments(read('api/search.uts'));
const jobApi = stripComments(read('api/job.uts'));
const typesJob = read('types/job.uts');
const typesIndex = read('types/index.uts');
const jobListPage = read('pages/jobs/job-list.uvue');
const jobDetailPage = read('pages/jobs/job-detail.uvue');

/* ══ ①-a search 域出口收紧（#652 T14） ══ */
describe('search 域收紧：两出口经 getMapped<T>，请求形态与字段读取不变', () => {
  it('import 只引入 getMapped（不再引入裸 get）', () => {
    expect(searchApi).toMatch(/import\s*\{\s*getMapped\s*\}\s*from\s*'\.\/request'/);
  });

  it('searchAllApi / searchByTypeApi 各经 getMapped<SearchResult> / getMapped<SearchPagedResult>', () => {
    expect(topFn(searchApi, 'export function searchAllApi')).toContain("getMapped<SearchResult>('/search'");
    expect(topFn(searchApi, 'export function searchByTypeApi')).toContain("getMapped<SearchPagedResult>('/search'");
  });

  /**
   * 「导出显式 DTO + **手动映射函数**」（本票 AC 原话）的落地形态：与 #643 T05 已收紧域同构——
   * 出口一行委托具名 build*，出口体内零 `data['…']`。字段读取一旦回到出口里，就意味着映射又开始
   * 各出口复制一份（job 域收紧前正是这个病：详情与列表两份同形映射，补一处漏一处）。
   */
  it('两出口一行委托具名映射，出口体内零字段读取（映射单一实现，与 T05 同构）', () => {
    for (const [marker, delegation] of [
      ['export function searchAllApi', '=> buildSearchResult(data, keyword)'],
      ['export function searchByTypeApi', '=> buildSearchPagedResult(data, keyword, type)'],
    ]) {
      const body = topFn(searchApi, marker);
      expect(body).toContain(delegation);
      expect(body).not.toMatch(FIELD_READ);
    }
    // keyword/type 是回退缺省值，只能由出口传给映射（不是映射自己去猜）
    expect(topFn(searchApi, 'function buildSearchResult')).toContain("keyword: toStr(data['keyword'], keyword)");
    expect(topFn(searchApi, 'function buildSearchPagedResult')).toContain("type: toStr(data['type'], type)");
  });

  it('字段读取保持（chapters 分区、credential_id 显式下发未被收紧动作改坏）', () => {
    expect(searchApi).toContain("data['chapters']");
    expect(searchApi).toContain("params['credential_id'] = credentialId.toString()");
  });

  it('零裸 get/post(...).then → DTO 残留，零 .catch(（失败上抛页面）', () => {
    expect(searchApi).not.toMatch(RAW_THEN_TO_DTO);
    expect(searchApi).not.toContain('.catch(');
  });
});

/* ══ ①-b job 域出口收紧 + DTO 迁 types/（#652 T14） ══ */
describe('job 域收紧：有载荷 GET 经 getMapped<T> + build* 去重', () => {
  it('import 引入 getMapped 与 post（post 供 void 透传出口）', () => {
    expect(jobApi).toMatch(/import\s*\{[^}]*\bgetMapped\b[^}]*\bpost\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('详情 / 列表收敛到 buildJobPosting（此前两处同形映射去重），列表经 buildJobListResult', () => {
    expect(jobApi).toMatch(/function buildJobPosting\(/);
    expect(jobApi).toMatch(/function buildJobListResult\(/);
    expect(topFn(jobApi, 'export function getJobDetailApi')).toContain('getMapped<JobPosting>');
    expect(topFn(jobApi, 'export function getJobDetailApi')).toMatch(/\(data : UTSJSONObject\) : JobPosting => buildJobPosting\(data\)/);
    expect(topFn(jobApi, 'export function getJobListApi')).toContain('getMapped<JobListResult>');
    expect(topFn(jobApi, 'export function getJobListApi')).toMatch(/\(data : UTSJSONObject\) : JobListResult => buildJobListResult\(data\)/);
    // 同 search 口径：去重后映射只有一份，出口不得再自己读字段
    for (const marker of ['export function getJobDetailApi', 'export function getJobListApi']) {
      expect(topFn(jobApi, marker)).not.toMatch(FIELD_READ);
    }
  });

  it('字段读取逐行不变（apply_state / cooldown_days / forced_offline 等 DTO 字段仍被映射）', () => {
    const builder = topFn(jobApi, 'function buildJobPosting');
    for (const f of ['id', 'title', 'salary_min', 'apply_state', 'cooldown_days', 'forced_offline']) {
      expect(builder).toContain(`obj['${f}']`);
    }
  });

  /**
   * 请求形态锁（ADR-0007「params 沿用手动 query 序列化…收紧不得改变请求形态」）。
   * 列表把查询串自己拼进 url、params 传 null —— 这是收紧**前后同一形态**，不是新写法：
   * getMapped 内部正是 `url + (url.includes('?') ? '&' : '?') + pairs.join('&')`，
   * 与旧的 `'/jobs' + (qs.length > 0 ? '?' + qs : '')` 逐字节同解。本条钉住它不被
   * 「顺手改成 params 传对象」（那会改序列化通路，属本票范围外的另一件事）。
   */
  it('列表请求形态不变：手动 query 串进 url、params 传 null', () => {
    const body = topFn(jobApi, 'export function getJobListApi');
    expect(body).toContain("const url = '/jobs' + (qs.length > 0 ? '?' + qs : '')");
    expect(body).toContain("encodeURIComponent(k) + '=' + encodeURIComponent(query[k] as string)");
    expect(body).toContain('getMapped<JobListResult>(url, null,');
  });

  it('reportJobApi / applyJobApi（void 透传）保持裸 post，不硬套 mapper', () => {
    expect(topFn(jobApi, 'export function reportJobApi')).toMatch(/return post\(/);
    expect(topFn(jobApi, 'export function applyJobApi')).toMatch(/return post\(/);
  });

  it('零裸 get/post(...).then → DTO 残留，零 .catch(', () => {
    expect(jobApi).not.toMatch(RAW_THEN_TO_DTO);
    expect(jobApi).not.toContain('.catch(');
  });
});

describe('job DTO 迁出：类型落 types/job.uts 并经 types/index 消费', () => {
  it('types/job.uts 定义四个 export type（api 层不再内联声明）', () => {
    for (const t of ['JobPosting', 'JobListParams', 'JobListResult', 'ReportReason']) {
      expect(typesJob).toContain(`export type ${t} = {`);
    }
    expect(jobApi).not.toMatch(/export type JobPosting/);
  });

  it('types/index barrel re-export job 类型（UTS 合法形态：inline type modifier）', () => {
    expect(typesIndex).toMatch(/export\s*\{[^}]*\btype JobPosting\b[^}]*\}\s*from\s*'\.\/job'/);
    for (const t of ['JobListParams', 'JobListResult', 'ReportReason']) {
      expect(typesIndex).toContain(`type ${t}`);
    }
  });

  it('job 页面 type import 指向 types/index，api/job 只剩值 import（机械重定向，零行为改动）', () => {
    expect(jobListPage).toMatch(/import type \{[^}]*JobPosting[^}]*\} from '\.\.\/\.\.\/types\/index'/);
    expect(jobDetailPage).toMatch(/import type \{[^}]*JobPosting[^}]*ReportReason[^}]*\} from '\.\.\/\.\.\/types\/index'/);
    // 页面仍从 api/job 取出口函数与常量（值 import 不动）
    expect(jobDetailPage).toContain('from \'../../api/job\'');
    // 但不再从 api/job 里 import 任何类型
    expect(jobListPage).not.toMatch(/import type \{[^}]*\} from '\.\.\/\.\.\/api\/job'/);
    expect(jobDetailPage).not.toMatch(/import type \{[^}]*\} from '\.\.\/\.\.\/api\/job'/);
  });
});

/* ══ ② 前序票已收紧域：不回潮复检（与本批口径齐平） ══ */
describe('points / featured / notification 域出口家族不回潮', () => {
  it.each(DOMAINS)('api/%s.uts import 含 getMapped（出口家族一致性）', (domain) => {
    const src = stripComments(read(`api/${domain}.uts`));
    expect(src).toMatch(/import\s*\{[^}]*\bgetMapped\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it.each(DOMAINS)('api/%s.uts 零裸 HTTP(...).then → DTO 残留', (domain) => {
    const src = stripComments(read(`api/${domain}.uts`));
    expect(src).not.toMatch(RAW_THEN_TO_DTO);
  });

  /**
   * 比 RAW_THEN_TO_DTO 更严的一致性面：这五域的**每一个** GET 都有载荷，收紧后全仓五域
   * 零裸 `get(`。抓的是 RAW_THEN_TO_DTO 漏掉的那类回潮（`get(url).then(x => x)`、
   * `get(url).then((d : any) => …)` —— 没标注 `data : UTSJSONObject` 就从它的缝里过去）。
   */
  it.each(DOMAINS)('api/%s.uts 零裸 get(（所有 GET 一律走 getMapped）', (domain) => {
    const src = stripComments(read(`api/${domain}.uts`));
    expect(src).not.toMatch(RAW_GET);
  });

  /** 票面 AC「六个域 api 全部导出显式 DTO 类型」的类型单一来源面 */
  it.each(DOMAINS)('api/%s.uts 的 DTO 来自 types/index，api 层零内联 `export type`', (domain) => {
    const src = stripComments(read(`api/${domain}.uts`));
    expect(src).toMatch(TYPES_INDEX_IMPORT);
    expect(src).not.toMatch(/^export type /m);
  });
});

/* ══ ③-a guide 域零 api（登记 + 不回潮） ══ */
/**
 * guide 页面「不得触 api 层」的判据本体。抽出来给锁与注入自检**共用同一份**——
 * 若自检里另写一遍正则，自检绿只证明自检那份正则没坏，证明不了锁没坏。
 */
const GUIDE_API_IMPORT = /from\s*'[^']*\/api\//;
const GUIDE_BARE_REQUEST = /uni\.request\s*\(/;
function guideApiHits(src) {
  const hits = [];
  if (GUIDE_API_IMPORT.test(src)) hits.push('import api/*');
  if (GUIDE_BARE_REQUEST.test(src)) hits.push('uni.request 裸调');
  return hits;
}

describe('guide 域零 api（无域 api 文件，页面不触 api 层）', () => {
  it('不存在 api/guide.uts（后端无 guide 端点，域本就无数据 api）', () => {
    expect(exists('api/guide.uts')).toBe(false);
  });

  it('pages/guide/** 无源文件 import api/ 或裸调 uni.request（不得回潮出隐性 api 面）', () => {
    const files = h.sourceFilesIn('pages/guide');
    // **下限断言**：目录空 / 路径写错时下面的循环零次执行、hits 恒为 [] ⇒ 假绿。
    expect(files.length).toBeGreaterThan(0);
    const hits = [];
    for (const rel of files) {
      for (const why of guideApiHits(stripComments(read(rel)))) hits.push(`${rel}: ${why}`);
    }
    expect(hits).toEqual([]);
  });
});

/* ══ ③-b api 层豁免（catch/detail）本票范围内清零，不回潮 ══ */
/** 豁免名单命中判据本体（同样与注入自检共用，理由见上） */
const apiEntryRe = (domain) => new RegExp(`api[/\\\\]${domain}\\.uts$`);

describe('api 层 allowlist 不回潮（本批五域 api 文件均无豁免条目）', () => {
  it.each(DOMAINS)('%s.uts 不在任何豁免名单', (domain) => {
    expect(allowlistPaths().filter((p) => apiEntryRe(domain).test(p))).toEqual([]);
  });
});

/* ══ 注入违规自检：证明上面每条判据真能判红（防正则失配导致假绿） ══ */
describe('判据自检（red-capable）：五条判据各命中植入的违规、不误伤合法形态', () => {
  it('RAW_THEN_TO_DTO：命中植入的裸 get(...).then → DTO 违规', () => {
    const planted = "export function foo() : Promise<Foo> {\n    return get('/x').then((data : UTSJSONObject) : Foo => buildFoo(data))\n}";
    expect(RAW_THEN_TO_DTO.test(planted)).toBe(true);
  });

  it('RAW_THEN_TO_DTO：不误伤 void 透传（无 data : UTSJSONObject 形参）与 getMapped 合法出口', () => {
    const voidPost = "return post(url, payload).then(() : void => {\n    })";
    const mapped = "return getMapped<Foo>('/x', params, (data : UTSJSONObject) : Foo => buildFoo(data))";
    expect(RAW_THEN_TO_DTO.test(voidPost)).toBe(false);
    expect(RAW_THEN_TO_DTO.test(mapped)).toBe(false);
  });

  /** 本条正是 RAW_GET 存在的理由：这类回潮从 RAW_THEN_TO_DTO 的缝里过去（形参没标注类型） */
  it('RAW_GET：命中 RAW_THEN_TO_DTO 漏掉的裸 get（无 DTO 标注的 .then、裸透传）', () => {
    for (const planted of [
      "return get('/x').then((d) : Foo => buildFoo(d))",
      "return get('/x').then((x : any) => x)",
      "return get('/x', params)",
    ]) {
      expect(RAW_GET.test(planted)).toBe(true);
      expect(RAW_THEN_TO_DTO.test(planted)).toBe(false);
    }
  });

  it('RAW_GET：不误伤 getMapped 出口与 get* 命名的函数（`get` 后紧跟字母不是 `(`）', () => {
    const legal = "return getMapped<Foo>('/x', params, (data : UTSJSONObject) : Foo => buildFoo(data))"
      + "\nexport function getJobListApi(params : P) : Promise<R> {"
      + "\nexport function getUnreadCountApi() : Promise<number> {";
    expect(RAW_GET.test(legal)).toBe(false);
  });

  it('FIELD_READ：命中「映射又复制回出口里」，不误伤委托形态与 params 手动序列化', () => {
    expect(FIELD_READ.test("const a = 1\nreturn { id: data['id'] }")).toBe(true);
    expect(FIELD_READ.test("return getMapped<Foo>('/x', params, (data : UTSJSONObject) : Foo => buildFoo(data, keyword))")).toBe(false);
    // job 列表出口体里有 query['page'] 与 params.page!，都不是响应字段读取
    expect(FIELD_READ.test("if (params.page != null) query['page'] = params.page!.toString()")).toBe(false);
  });

  /**
   * guide 判据的自检。植入的两条正是它要抓的两种回潮形态（显式 import 一个域 api、
   * 或绕过 api 层直接 uni.request）；合法形态拿**真实页面**当样本，避免自说自话。
   */
  it('guideApiHits：命中植入的 api import 与裸 uni.request，不误伤 choose-cert 现状', () => {
    expect(guideApiHits("import { searchAllApi } from '../../api/search'")).toEqual(['import api/*']);
    expect(guideApiHits("uni.request({ url: '/x' })")).toEqual(['uni.request 裸调']);
    expect(guideApiHits("import { getItemList } from '../../components/list'")).toEqual([]);
    for (const rel of h.sourceFilesIn('pages/guide')) {
      expect(guideApiHits(stripComments(read(rel)))).toEqual([]);
    }
  });

  /**
   * allowlist 判据的自检：`filter(...)` 命中空名单时**永远返回 []**，与「名单里有但没命中」长得一模一样。
   * 于是本条先钉名单本身非空（读者活着），再钉判据对植入路径确实命中、对页面同名文件不命中。
   */
  it('apiEntryRe：名单非空、植入的 api/<域>.uts 条目确实命中、不误伤同名页面路径', () => {
    expect(allowlistPaths().length).toBeGreaterThan(0);
    expect(apiEntryRe('job').test('training-app/app/api/job.uts')).toBe(true);
    expect(apiEntryRe('job').test('training-app\\api\\job.uts')).toBe(true);
    expect(apiEntryRe('job').test('training-app/app/pages/jobs/job.uts')).toBe(false);
    expect(apiEntryRe('job').test('training-app/app/api/jobby.uts')).toBe(false);
  });

  /** DTO 归位判据的自检：内联声明与「类型来自 types/index」两条各判一次 */
  it('DTO 归位判据：植入的内联 export type 确实命中、合法 import type 形态确实命中、空花括号不算数', () => {
    expect(/^export type /m.test("export function a() {}\nexport type JobPosting = {\n}")).toBe(true);
    expect(/^export type /m.test("import type { JobPosting } from '../types/index'")).toBe(false);
    expect(TYPES_INDEX_IMPORT.test("import type { JobPosting, JobListResult } from '../types/index'")).toBe(true);
    // 空花括号（含带空格的 `{ }`）不满足判据 —— 旧写法 `[^}]+` 在这里会假绿
    expect(TYPES_INDEX_IMPORT.test("import type { } from '../types/index'")).toBe(false);
    expect(TYPES_INDEX_IMPORT.test("import type {} from '../types/index'")).toBe(false);
  });
});
