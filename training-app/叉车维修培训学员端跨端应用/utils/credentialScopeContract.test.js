/**
 * #1107 移动端证件下发口径契约锁 —— 「哪些端点必须显式传 `credential_id`」的冻结清单。
 *
 * 事实源：`docs/agents/credential-scope.md`（本文件是它的机检面）。锁四件事：
 *   1. **scoped 端点消费面 = 冻结清单**：移动端 api 层里所有落在「按证件分区」路由族上的
 *      字面量请求点（方法 + 路径）逐条冻结，逐文件计数冻结 —— 新增消费面/多写一处调用都要
 *      回来登记，否则判红（这就是「漏传面为零」的机械判据：新面不可能悄悄绕过分区）。
 *   2. **必须显式传的面真的在传**：`/search`、`/courses`、`/tags` 走 query，
 *      `POST /practice-mode/progress` 走 body；逐个断言「形参声明 + 赋值点」成对存在。
 *   3. **依赖服务端兜底的面不许传**：JWT + CredentialScoped 的路由族一旦显式传参，
 *      语义就从「跟随当前」变成「浏览指定」，必须回来登记，故此处反向锁。
 *   4. **不分区面不许加参数**：`getCatalogTreeSpecialties` 只消费 `specialties`（全局共享字典），
 *      传证件是 no-op（服务端只分区课程节点）—— 别「顺手补一个参数」。
 *
 * ⚠️ 扫描面与边界（老实写清）：jest 无法 import `.uts` / `.uvue`（本仓契约测试一贯用源码缝），
 * 且只认**字面量**请求点 ⇒ **看不见**「先把路径拼进变量再发请求」的写法（如
 * `const url = '/wrong-questions/' + id + '/redo'`）与 `uploadFile(...)`。那些面的运行期证据
 * 由 ①a 真机 logcat 的 `[request] >>> GET <url>` 机检行兜（见口径表 §四）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 抹掉注释（`://` 例外，防误杀 URL 字面量） */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/** 取某个 api 函数的函数体（到行首 `}` 为止，与 practiceContract 同缝；兼容非导出的帮助函数） */
function fnBody(rel, name) {
  const src = read(rel);
  let start = src.indexOf('export function ' + name + '(');
  if (start < 0) start = src.indexOf('function ' + name + '(');
  if (start < 0) return '';
  return src.slice(start, src.indexOf('\n}', start) + 2);
}

/** 后端「按当前证件分区」的路由族前缀（`CredentialScoped` 注册面 + handler 自读 query 的公开面） */
const SCOPED_PREFIX = /^\/(search|courses|tags|question-bank|practice-mode|wrong-questions|mock-exam|real-exam|favorites|contributions)([/?]|$)/;

/** api 层的字面量请求点：{ file, method, path, fn } */
function literalRequestSites() {
  const apiDir = path.join(ROOT, 'api');
  const out = [];
  for (const f of fs.readdirSync(apiDir).filter((x) => x.endsWith('.uts')).sort()) {
    const rel = 'api/' + f;
    const src = stripComments(read(rel));
    const fnRe = /(?:export\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(/g;
    const bounds = [];
    let m;
    while ((m = fnRe.exec(src)) !== null) bounds.push({ name: m[1], start: m.index });
    for (let i = 0; i < bounds.length; i++) bounds[i].end = i + 1 < bounds.length ? bounds[i + 1].start : src.length;
    const reqRe = /(?:^|[^\w.])(get|post|put|del|patch)(?:Mapped\s*<[^>]*>)?\s*\(\s*'([^']+)'/g;
    while ((m = reqRe.exec(src)) !== null) {
      const fn = bounds.filter((b) => b.start <= m.index && m.index < b.end).pop();
      out.push({ file: rel, method: m[1].toUpperCase(), path: m[2], fn: fn ? fn.name : '(top-level)' });
    }
  }
  return out;
}

const ALL_SITES = literalRequestSites();
const SCOPED_SITES = ALL_SITES.filter((s) => SCOPED_PREFIX.test(s.path));

/**
 * 冻结：scoped 端点消费面（方法 + 路径，去重，25 处）。
 * 新增一行 = 新端点在移动端有了消费面 ⇒ 先判它该显式传还是靠服务端兜底，登记口径表再改这里。
 */
const SCOPED_REQUEST_SITES = [
  'DEL /favorites/',
  'GET /contributions/mine',
  'GET /courses',
  'GET /favorites',
  'GET /favorites/check',
  'GET /mock-exam/history',
  'GET /practice-mode/free',
  'GET /practice-mode/history',
  'GET /practice-mode/practice-stats',
  'GET /practice-mode/progress',
  'GET /practice-mode/sequential',
  'GET /practice-mode/stats',
  'GET /practice-mode/tag',
  'GET /question-bank/questions',
  'GET /question-bank/questions/',
  'GET /question-bank/stats',
  'GET /search',
  'GET /tags',
  'GET /wrong-questions',
  'GET /wrong-questions/stats',
  'POST /contributions',
  'POST /favorites',
  'POST /mock-exam/start',
  'POST /practice-mode/progress',
  'POST /practice-mode/submit'
].sort();

/** 冻结：逐文件 scoped 请求点计数（防「同一路径多写一处调用」绕过去重集合） */
const SCOPED_REQUEST_COUNTS = {
  'api/contribution.uts': 2,
  'api/course.uts': 2,
  'api/favorite.uts': 4,
  'api/mockExam.uts': 2,
  'api/practice.uts': 13,
  'api/search.uts': 2,
  'api/wrongQuestion.uts': 2
};

/**
 * 必须显式传（query）：公开端点，服务端兜底永不生效。
 * `pass` = 该函数体里必须出现的「把 credentialId 交出去」的落点。
 * 搜索侧把赋值收在 `withCredential` 帮助函数里（两个出口共用），故单独锁那个帮助函数。
 */
const EXPLICIT_QUERY = [
  { api: 'api/search.uts', fn: 'searchAllApi', pass: ['credentialId : number = 0', ', credentialId)'] },
  { api: 'api/search.uts', fn: 'searchByTypeApi', pass: ['credentialId : number = 0', ', credentialId)'] },
  { api: 'api/course.uts', fn: 'getCourseListApi', pass: ['credentialId : number = 0', "params['credential_id'] = credentialId.toString()"] },
  { api: 'api/course.uts', fn: 'getTagsApi', pass: ['credentialId : number = 0', 'credential_id: credentialId.toString()'] }
];

/** 依赖服务端兜底（JWT 面）：函数体内不得出现 credential_id */
const FALLBACK_FNS = {
  'api/practice.uts': [
    'getQuestionBankStatsApi', 'getQuestionListApi', 'getQuestionByIdApi',
    'getFreePracticeApi', 'getSequentialPracticeApi', 'getTagPracticeApi',
    'submitAnswerApi', 'getPracticeProgressApi', 'getPracticeReportApi',
    'getPracticeOverviewApi', 'getPracticeStatsApi', 'getPracticeHistoryApi'
  ],
  'api/wrongQuestion.uts': ['getWrongQuestionsApi', 'getWrongQuestionStatsApi'],
  'api/mockExam.uts': ['startMockExamApi', 'getMockExamHistoryApi'],
  'api/favorite.uts': ['getFavoritesApi', 'addFavoriteApi', 'removeFavoriteApi', 'checkFavoriteApi'],
  'api/contribution.uts': ['getMyContributionsApi']
};

describe('#1107 scoped 端点消费面 = 冻结清单', () => {
  it('字面量请求点里落在 scoped 路由族的集合与冻结表一致', () => {
    const actual = [...new Set(SCOPED_SITES.map((s) => s.method + ' ' + s.path))].sort();
    expect(actual).toEqual(SCOPED_REQUEST_SITES);
  });

  it('逐文件计数一致（同一路径的第二处调用也要登记）', () => {
    const counts = {};
    for (const s of SCOPED_SITES) counts[s.file] = (counts[s.file] || 0) + 1;
    expect(counts).toEqual(SCOPED_REQUEST_COUNTS);
  });

  it('判据不放过坏实现：扫描器确实看得见已知的传递点（探针）', () => {
    // 若正则被改坏，上面两条会「两边都空」而假绿 —— 用两处已知点做判别力前置
    const probe = SCOPED_SITES.filter((s) => (s.file === 'api/search.uts' && s.method === 'GET') || s.method === 'POST' && s.path === '/practice-mode/progress');
    expect(probe.length).toBe(3);
    expect(ALL_SITES.length).toBeGreaterThan(100);
  });
});

describe('#1107 必须显式传的面（漏传面为零）', () => {
  it.each(EXPLICIT_QUERY)('$api#$fn：形参 + 赋值成对存在（query）', ({ api, fn, pass }) => {
    const body = fnBody(api, fn);
    expect(body.length).toBeGreaterThan(0);
    for (const token of pass) expect(body).toContain(token);
  });

  it('api/search.uts：赋值收在 withCredential 单点，且只在 >0 时下发', () => {
    const helper = fnBody('api/search.uts', 'withCredential');
    expect(helper).toContain('if (credentialId > 0)');
    expect(helper).toContain("params['credential_id'] = credentialId.toString()");
  });

  it('POST /practice-mode/progress：证件写进 body，且只在 sequential 下写', () => {
    const body = fnBody('api/practice.uts', 'savePracticeProgressApi');
    expect(body).toContain('credentialId : number = 0');
    expect(body).toContain("if (practiceMode == 'sequential' && credentialId > 0)");
    expect(body).toContain("payload['credential_id'] = credentialId");
  });

  it('POST /contributions：创建投稿的 body 带归属证件（既有行为不变）', () => {
    const body = fnBody('api/contribution.uts', 'createContributionApi');
    expect(body).toContain('credential_id: credentialId');
  });
});

describe('#1107 依赖兜底的面不许传（传了就是「浏览指定」，需另登记）', () => {
  it.each(Object.entries(FALLBACK_FNS))('%s 的 JWT 面函数体不含 credential_id', (api, fns) => {
    const missing = [];
    const offenders = [];
    for (const fn of fns) {
      const body = fnBody(api, fn);
      if (body === '') missing.push(fn);
      if (body.indexOf('credential_id') >= 0) offenders.push(fn);
    }
    expect(missing).toEqual([]);
    expect(offenders).toEqual([]);
  });

  it('不分区面不得「顺手补参数」：/catalog/tree 只消费 specialties', () => {
    const body = fnBody('api/course.uts', 'getCatalogTreeSpecialties');
    expect(body).not.toContain('credential');
  });
});

describe('#1107 消费面接线：页面/组合式函数把证件交给 api', () => {
  it('搜索页：浏览器指定名 browseCredentialId 随两个出口下发', () => {
    const page = read('pages/search/search.uvue');
    expect(page).toContain('const browseCredentialId = ref<number>(0)');
    expect(page).toContain('searchAllApi(kw, browseCredentialId.value)');
    expect(page).toContain('searchByTypeApi(kw, tab.value, requestPage, pageSize.value, browseCredentialId.value)');
  });

  it('课程页：解析当前证件 + 随 /courses 下发 + 证件切换后回到第一页重取', () => {
    const page = read('pages/courses/courses.uvue');
    expect(page).toContain('const browseCredentialId = ref<number>(0)');
    expect(page).toContain('await ensureBrowseCredential()');
    expect(page).toContain('getCourseListApi(page.value, pageSize, 0, 0, browseCredentialId.value)');
    expect(page).toContain('if (browseCredentialId.value != prevCredentialId)');
  });

  it('商城页：进入前解析当前证件并随 /courses 下发', () => {
    const page = read('pages/mall/mall.uvue');
    expect(page).toContain('const browseCredentialId = ref<number>(0)');
    expect(page).toContain('loadBrowseCredential().then(');
    expect(page).toContain('getCourseListApi(page.value, pageSize, 0, 0, browseCredentialId.value)');
  });

  it('练习总览：标签题数按当前证件取（与抽题池同口径）', () => {
    const src = read('pages/practice/composables/usePracticeOverview.uts');
    expect(src).toContain('const cred = await getCurrentCredentialApi()');
    expect(src).toContain('getTagsApi(browseCredentialId)');
  });

  it('练习会话：顺序练习保存进度时把当前证件写进 body', () => {
    const src = read('pages/practice/composables/usePracticeSession.uts');
    expect(src).toContain('let browseCredentialId = 0');
    expect(src).toContain('savePracticeProgressApi(currentIndex.value + 1, saveMode, questions.value.length, a, browseCredentialId)');
  });
});

describe('#1107 口径表（事实源）在位且覆盖决策面', () => {
  const doc = read('docs/agents/credential-scope.md');

  it.each(['`GET /search`', '`GET /courses`', '`GET /tags`', '`POST /practice-mode/progress`'])(
    '口径表登记了必须显式传的端点 %s',
    (token) => {
      expect(doc).toContain(token);
    }
  );

  it('口径表引用了根仓例外登记与决策落点（避免与根仓口径脱钩）', () => {
    expect(doc).toContain('ADR-0056');
    expect(doc).toContain('credential_scope.go:24-51');
    expect(doc).toContain('roleOf(c) == student');
  });

  it('口径表声明「不分区」判据：/catalog/tree 的消费字段不随证件变', () => {
    expect(doc).toContain('`GET /catalog/tree`');
    expect(doc).toContain('no-op');
  });
});
