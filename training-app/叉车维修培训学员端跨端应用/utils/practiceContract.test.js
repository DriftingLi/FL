/**
 * practice 模块手术契约测试（T06，parent #644 / ADR-0007）
 *
 * 钉住 practice 手术交付的六类契约：
 * 1) 600 行软预算：pages/practice/** 全部源文件 ≤600 行
 * 2) 模块目录 ≤2 层
 * 3) composable 接线：practice-do.uvue / practice.uvue 以显式 import 使用 composable
 * 4) 组件接线零孤儿：页面 import 的组件文件必须存在，组件文件必须被页面引用
 *    （#779 回归教训：practice.uvue 改为 import 四个组件却从未创建文件，master 编译中断）
 * 5) allowlist 不回潮：practice 域文件不得出现在 GUARD_ALLOWLIST
 * 6) 零直发请求：页面层不直接 uni.request
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
/** 豁免名单从单点读（ADR-0023 ⑧）：不再解析守护脚本源码文本取常量 */
const { allowlistPaths } = require('./contractHarness');

const PRACTICE_PAGES = ['pages/practice/practice.uvue', 'pages/practice/practice-do.uvue'];

function practiceSourceFiles(dir = 'pages/practice') {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, dir));
  return out;
}

describe('600 行软预算机检（pages/practice/** 达标后锁定）', () => {
  it('practice 模块全部源文件 ≤600 行', () => {
    const over = practiceSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: readText(f).split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层', () => {
    const deep = practiceSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/practice'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

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
        const target = path.join(ROOT, 'pages/practice/components', m[1]);
        if (!fs.existsSync(target)) missing.push(page + ' -> components/' + m[1]);
      }
    }
    expect(missing).toEqual([]);
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const dir = path.join(ROOT, 'pages/practice/components');
    if (!fs.existsSync(dir)) return;
    const pagesSrc = PRACTICE_PAGES.map((p) => read(p)).join('\n');
    const orphans = fs.readdirSync(dir)
      .filter((f) => f.endsWith('.uvue'))
      .filter((f) => !pagesSrc.includes('./components/' + f));
    expect(orphans).toEqual([]);
  });
});

describe('allowlist 不回潮（practice 域违例清零的锁）', () => {
  it('豁免面不含 practice 域文件', () => {
    const hits = allowlistPaths().filter((p) => /pages[/\\]practice|api[/\\]practice\.uts/.test(p));
    expect(hits).toEqual([]);
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
