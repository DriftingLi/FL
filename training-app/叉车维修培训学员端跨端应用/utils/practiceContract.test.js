/**
 * practice 模块手术契约测试（T06，parent #644 / ADR-0007）
 *
 * 钉住 practice 手术交付的契约（编号是类别不是 describe 数；1) 与 4) 不是本文件的锁，
 * 是「执法点在别处」的口径登记）：
 * 1) 600 行软预算 / 模块目录 ≤2 层：**已由声明面执法**（`utils/modules.js` +
 *    `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件不再各写一遍（ADR-0023 票 C #1219）
 * 2) composable 接线：practice-do.uvue / practice.uvue 以显式 import 使用 composable
 * 3) 组件接线零孤儿：页面 import 的组件文件必须存在，组件文件必须被页面引用
 *    （#779 回归教训：practice.uvue 改为 import 四个组件却从未创建文件，master 编译中断）
 * 4) 域文件机械坑位（catch : any / .detail 直取）：本文件不再写这类锁——#654 起豁免面已删，
 *    执法点只有全工程守护 `utils/utsAndroidCompile.test.js` 规则 H/I 一处
 * 5) 零直发请求：页面层不直接 uni.request
 */
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

const PRACTICE_PAGES = ['pages/practice/practice.uvue', 'pages/practice/practice-do.uvue'];

describe('composable 接线契约（T06 拆分：显式 import composable）', () => {
  const doPage = read('pages/practice/practice-do.uvue');
  const mainPage = read('pages/practice/practice.uvue');

  it('practice-do.uvue import usePracticeSession composable', () => {
    expect(doPage).toContain('./composables/usePracticeSession');
  });

  it('practice-do.uvue import usePracticeComments composable', () => {
    expect(doPage).toContain('./composables/usePracticeComments');
  });

  it('practice-do.uvue import usePracticeNotes composable', () => {
    expect(doPage).toContain('./composables/usePracticeNotes');
  });

  it('practice.uvue import usePracticeOverview composable', () => {
    expect(mainPage).toContain('./composables/usePracticeOverview');
  });
});

describe('组件接线零孤儿（#779 回归锁：import 的组件文件必须存在）', () => {
  it('页面 import 的每个模块私有组件文件都真实存在', () => {
    const missing = [];
    for (const page of PRACTICE_PAGES) {
      const src = read(page);
      const re = /from\s+'\.\/components\/([^']+\.uvue)'/g;
      let m;
      while ((m = re.exec(src)) !== null) {
        if (!h.exists('pages/practice/components/' + m[1])) missing.push(page + ' -> components/' + m[1]);
      }
    }
    expect(missing).toEqual([]);
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const relDir = 'pages/practice/components';
    if (!h.exists(relDir)) return;
    const pagesSrc = PRACTICE_PAGES.map((p) => read(p)).join('\n');
    const orphans = h.sourceFilesIn(relDir)
      .map((rel) => rel.split('/').pop())
      .filter((f) => f.endsWith('.uvue'))
      .filter((f) => !pagesSrc.includes('./components/' + f));
    expect(orphans).toEqual([]);
  });
});

describe('零直发请求（页面层不直接 uni.request）', () => {
  it('practice-do.uvue 不直接调用 uni.request', () => {
    const src = read('pages/practice/practice-do.uvue');
    expect(src).not.toMatch(/uni\.request\s*\(/);
  });

  it('practice.uvue 不直接调用 uni.request', () => {
    const src = read('pages/practice/practice.uvue');
    expect(src).not.toMatch(/uni\.request\s*\(/);
  });
});

describe('域 api 收紧（T06 / ADR-0007）：DTO 函数经 mapper-callback 出口', () => {
  const api = read('api/practice.uts');

  it('引入 getMapped / postMapped 出口家族', () => {
    expect(api).toMatch(/import\s*\{[^}]*getMapped[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
  });

  const DTO_FNS = [
    'getFreePracticeApi',
    'getSequentialPracticeApi',
    'getTagPracticeApi',
    'getQuestionByIdApi',
    'submitAnswerApi',
    'getPracticeProgressApi',
    'getPracticeOverviewApi',
    'getPracticeStatsApi',
    'getPracticeHistoryApi',
  ];

  it.each(DTO_FNS)('%s 经 mapper 出口（getMapped/postMapped + 箭头包裹 build*）', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    // URL 允许「字面量」与「字面量 + id 变量拼接」两种形态（by-id 端点后者才有意义，先例 examContract）
    expect(body).toMatch(/(get|post)Mapped<[A-Za-z_$][\w$]*(?:\[\])?>\('[^']+'(?:\s*\+\s*[^,]+)?, [^,]+, \(data : UTSJSONObject\) : [A-Za-z_$][\w$]*(?:\[\])? => build[A-Za-z_$][\w$]*\(data\)\)/);
  });

  // raw 透传白名单：刻意保留裸 get/post（不硬套 identity map，T03/T05 口径）
  const RAW_PASSTHROUGH = [
    'getQuestionBankStatsApi',
    'getQuestionListApi',
    'savePracticeProgressApi',
    'getPracticeReportApi',
  ];

  it.each(RAW_PASSTHROUGH)('%s 保持 raw 透传（裸 get/post），未误加 DTO', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    expect(body).toMatch(/return (get|post)\('/);
    expect(body).not.toMatch(/Mapped</);
  });
});

describe('幻影路由锁（#662 口径）：api 层路由必须落在后端已注册清单内', () => {
  /** 抹掉注释（`://` 例外，防误杀 URL 字面量） */
  function stripComments(s) {
    return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
  }

  /** 从后端 <file>.go 的 rg.Group("<prefix>") + g.METHOD("<path>") 推出已注册路由 */
  function registeredRoutes(rel, prefix) {
    const src = stripComments(read(rel));
    const out = [];
    const re = /g\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)"[^)]*\)/g;
    let m;
    while ((m = re.exec(src)) !== null) out.push(prefix + m[2]);
    return out;
  }

  it('practice 域 api 的每个路由都在 practice_mode.go / question_bank.go 已注册', () => {
    const registered = [
      ...registeredRoutes('../../backend/internal/api/practice_mode.go', '/practice-mode'),
      ...registeredRoutes('../../backend/internal/api/question_bank.go', '/question-bank'),
    ];
    expect(registered.length).toBeGreaterThan(10);

    const api = stripComments(read('api/practice.uts'));
    // 拼接连形态：'/question-bank/questions/' + <id 变量> → /question-bank/questions/:question_id
    // （后端把路径参数写作 :question_id，故拼接段归一成同名占位符，先例 examContract 的 :mock_exam_id）
    const concatRe = /'(\/(?:practice-mode|question-bank)\/[^']*\/)'\s*\+\s*[A-Za-z_$][\w$.]*/g;
    const used = [];
    let c;
    while ((c = concatRe.exec(api)) !== null) used.push(c[1] + ':question_id');
    const rest = api.replace(concatRe, '');
    used.push(...[...rest.matchAll(/'(\/(?:practice-mode|question-bank)\/[^']*)'/g)].map((m) => m[1]));
    expect(used.length).toBeGreaterThan(8);

    const phantom = used.filter((u) => !registered.includes(u));
    expect(phantom).toEqual([]);
  });
});

describe('api 层零静默回退（失败要可见）', () => {
  const api = read('api/practice.uts');

  it('api/practice.uts 不含 mock 占位数据构造', () => {
    expect(api).not.toMatch(/mock/i);
  });

  it('api/practice.uts 不吞错：零 .catch(，失败直带上抛页面', () => {
    expect(api).not.toContain('.catch(');
  });
});
