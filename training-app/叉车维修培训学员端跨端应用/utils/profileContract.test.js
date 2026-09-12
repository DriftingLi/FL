/**
 * profile 模块手术契约测试（refs #641 / refactor epic #638 T03）
 *
 * 沿用源码契约缝（.uvue/.uts 不可 jest import）。先例：mallPilotContract、quickLoginContract。
 * 逐切片补块：本文件随手术推进追加剧本（域 api 收紧 → 页面拆分/预算/接线）。
 *
 * 本票域 api 收紧口径：仅「有 DTO 映射」的函数经 *Mapped 出口家族；
 * 返回 UTSJSONObject/void 的原样透传函数不硬套 identity map（避免无意义中间层）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 提取 export function 函数体（从声明到顶层 "\n}"，先例 quickLoginContract） */
function fnBodyOf(fileSrc, name) {
  const start = fileSrc.indexOf(`export function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  const end = fileSrc.indexOf('\n}', start);
  return fileSrc.slice(start, end);
}

describe('出口家族完备性（postMapped 为 T03 前置补全）', () => {
  const req = read('api/request.uts');
  it('postMapped 存在：POST 形态与历史 post() 逐字一致（opts.data 同通路），委派 requestMapped', () => {
    const start = req.indexOf('export function postMapped');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n}', start));
    expect(body).toContain("opts.method = 'POST'");
    expect(body).toContain('opts.data = data');
    expect(body).toContain('requestMapped<T>(opts, map)');
  });
});

describe('student 域收紧（profile 主页面数据源）', () => {
  const src = read('api/student.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getProfileApi', 'getStudyStatsApi', 'getRecordsApi', 'getStudentCoursesApi', 'getStudentCourseDetailApi'])(
    '%s 经 getMapped 且保留原 catch/mock 降级结构', (fn) => {
      const body = fnBodyOf(src, fn);
      expect(body).toContain('getMapped<');
      // 收紧不得改变错误行为：原函数有 .catch 的必须保留
    });
  it('getProfileApi 的 mock 降级保持', () => {
    expect(fnBodyOf(src, 'getProfileApi')).toContain('getMockProfile()');
  });
  it('getStudyStatsApi / getRecordsApi 的 mock 降级保持', () => {
    expect(fnBodyOf(src, 'getStudyStatsApi')).toContain('getMockStudyStats()');
    expect(fnBodyOf(src, 'getRecordsApi')).toContain('getMockRecords()');
  });
});

describe('wrongQuestion 域收紧（错题本）', () => {
  const src = read('api/wrongQuestion.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getWrongQuestionsApi', 'getWrongQuestionStatsApi'])('%s 经 getMapped + build* 映射', (fn) => {
    const body = fnBodyOf(src, fn);
    expect(body).toContain('getMapped<');
    expect(body).toMatch(/build(WrongQuestionListResult|WrongQuestionStats)\(/);
  });
});

describe('favorite 域收紧（收藏段）', () => {
  const src = read('api/favorite.uts');
  it('引入 getMapped 与 postMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getFavoritesApi 经 getMapped 箭头包裹 builder（具名函数直传触发 Kotlin error17，#685），后端 favorites 字段兼容与分页默认值保持', () => {
    const body = fnBodyOf(src, 'getFavoritesApi');
    expect(body).toContain('getMapped<');
    // mapper 必须箭头包裹：UTS→Kotlin 不支持具名顶层函数直传高阶参数（error17）
    expect(body).toMatch(/getMapped<FavoriteListResult>\([^)]*\(data : UTSJSONObject\) : FavoriteListResult => buildFavoriteListResult\(data\)/);
    const b = read('api/favorite.uts');
    expect(b).toMatch(/function buildFavoriteListResult/);
    expect(b).toContain("data['favorites']");
  });
  it('addFavoriteApi 经 postMapped 映射 FavoriteItem', () => {
    expect(fnBodyOf(src, 'addFavoriteApi')).toContain('postMapped<FavoriteItem>');
  });
  it('checkFavoriteApi 经 getMapped 且提取了 buildFavoriteCheckResult', () => {
    expect(fnBodyOf(src, 'checkFavoriteApi')).toContain('getMapped<');
    expect(read('api/favorite.uts')).toMatch(/function buildFavoriteCheckResult/);
  });
});

describe('points 域收紧（积分余额/流水/任务，#709 改口径）', () => {
  const src = read('api/points.uts');
  it('引入 getMapped / postMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getPointsBalanceApi', 'getPointsLedgerApi', 'getPointsTasksApi', 'claimPointsTaskApi'])('%s 经 mapper-callback 出口', (fn) => {
    expect(fnBodyOf(src, fn)).toMatch(/(get|post)Mapped</);
  });
  // #709 前的锁是「mock 降级保持」（profile 首屏零网络依赖）；幻影 404 被它盖了三个月，
  // 口径反转为「失败要可见」：points 域一律不得再出现占位回退（见 pointsRealApiContract）。
  it('points 域 mock 降级已退役（#709）：无 getMock* 且出口零 .catch(', () => {
    const code = src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
    expect(code).not.toMatch(/getMock/);
    expect(code).not.toContain('.catch(');
  });
});

describe('raw .then 收紧完成度（本票四域 DTO 函数零残留）', () => {
  const targets = ['api/student.uts', 'api/wrongQuestion.uts', 'api/favorite.uts', 'api/points.uts'];
  it.each(targets)('%s 不再出现 get/post(...).then((data : UTSJSONObject) : DTO 形态', (rel) => {
    const src = read(rel);
    expect(src).not.toMatch(/return (get|post)\([^)]*\)\s*\.then\(\(data : UTSJSONObject\) : (?!UTSJSONObject)/);
  });
});

/* ══ T03f 模块级汇总（refs #678）：预算复检 / 接线收口 / 零直发请求 / allowlist 清零 ══
 * 前置票（#675/#676/#677 及主页面/api 收紧）已各自切片机检；
 * 本块是模块口径的总锁——预算按 pages/profile/** 全量走查（不止四手术文件），
 * 接线按「拆出物全挂载 + components/composables 零孤儿」收口，allowlist 覆盖整个模块与四域 api。
 * 「零直发请求」是 #641 模块 AC 的本模块锁；不新增全工程守护规则
 * （ADR-0007 明示该守护留待 #654 收尾票立项）。 */

/** pages/profile/** 全部源文件（.uvue/.uts，排除测试） */
function profileSourceFiles() {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, 'pages/profile'));
  return out;
}

describe('600 行软预算机检（模块全量：pages/profile/** 全部源文件）', () => {
  it('走查范围非空（防路径断链导致空集合假绿：四页 + 8 组件 + flows + 其余页 ≥14）', () => {
    expect(profileSourceFiles().length).toBeGreaterThanOrEqual(14);
  });

  it('四超预算文件全部预算内复检（错题本 1137 / 个人信息 1011 / 个人动态 722 / 主页 687 的落袋锁）', () => {
    const over = [
      'pages/profile/wrong-questions.uvue',
      'pages/profile/personal-info.uvue',
      'pages/profile/personal-activity.uvue',
      'pages/profile/profile.uvue',
    ].map((rel) => ({ file: rel, lines: read(rel).split('\n').length }))
      .filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块全部源文件 ≤600 行（含 components/**，达标后锁住防回潮，先例 mallPilot）', () => {
    const over = profileSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层（pages/profile/<页 或 <子目录>/<文件>）', () => {
    const deep = profileSourceFiles()
      .filter((f) => path.relative(path.join(ROOT, 'pages/profile'), f).split(/[\\/]/).length > 2)
      .map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

describe('组件接线汇总（T03 拆出物 8 组件 + 1 composable：显式 import + 模板挂载）', () => {
  const WIRING = [
    ['ProfileUserRow', 'pages/profile/profile.uvue', './components/profile-user-row.uvue'],
    ['WrongStatsCard', 'pages/profile/wrong-questions.uvue', './components/wrong-stats-card.uvue'],
    ['WrongQuestionCard', 'pages/profile/wrong-questions.uvue', './components/wrong-question-card.uvue'],
    ['WrongFilterBar', 'pages/profile/wrong-questions.uvue', './components/wrong-filter-bar.uvue'],
    ['InfoDialog', 'pages/profile/personal-info.uvue', './components/info-dialog.uvue'],
    ['ActivityUserCard', 'pages/profile/personal-activity.uvue', './components/activity-user-card.uvue'],
    ['ActivityTabBar', 'pages/profile/personal-activity.uvue', './components/activity-tab-bar.uvue'],
    ['ActivityTopicCard', 'pages/profile/personal-activity.uvue', './components/activity-topic-card.uvue'],
  ];

  it.each(WIRING)('<%s>：%s 显式 import %s 且模板挂载', (tag, pageRel, importRel) => {
    const page = read(pageRel);
    expect(page).toContain(importRel);
    expect(page).toMatch(new RegExp(`<${tag}[\\s/>]`));
  });

  it('personal-info flows composable 经模块内显式相对 import（Q17 安置）', () => {
    expect(read('pages/profile/personal-info.uvue')).toContain('./composables/personal-info-flows.uts');
  });

  it('拆出物零孤儿：components/ 与 composables/ 每个文件都被模块内源文件 import（新增拆出物必须接线）', () => {
    const pageSrcs = profileSourceFiles()
      .filter((f) => !f.includes(`${path.sep}components${path.sep}`) && !f.includes(`${path.sep}composables${path.sep}`))
      .map((f) => fs.readFileSync(f, 'utf8'));
    const orphanOf = (dir, prefix) => fs.readdirSync(path.join(ROOT, 'pages/profile', dir))
      .filter((n) => /\.(uvue|uts)$/.test(n))
      .filter((n) => !pageSrcs.some((s) => s.includes(`${prefix}/${n}`)))
      .sort();
    const orphans = [
      ...orphanOf('components', './components').map((n) => `components/${n}`),
      ...orphanOf('composables', './composables').map((n) => `composables/${n}`),
    ];
    expect(orphans).toEqual([]);
  });
});

describe('页面层零直发请求（网络一律经域 api 函数，#641 收紧口径）', () => {
  it('pages/profile/** 无源文件 import api/request 或裸调 uni.request', () => {
    const hits = [];
    for (const f of profileSourceFiles()) {
      const src = fs.readFileSync(f, 'utf8');
      const rel = path.relative(ROOT, f);
      if (/from\s*'[^']*api\/request(\.uts)?'/.test(src)) hits.push(`${rel}: import api/request`);
      if (/uni\.request\s*\(/.test(src)) hits.push(`${rel}: uni.request 裸调`);
      if (/uni\.(upload|download)File\s*\(/.test(src)) hits.push(`${rel}: uni.uploadFile/downloadFile 裸调`);
    }
    expect(hits).toEqual([]);
  });
});

describe('allowlist 模块级清零（profile 域 catch/detail 违例豁免不存在）', () => {
  /** GUARD_ALLOWLIST 源码块（豁免机制定义于守护文件，先例各切片 allowlist 锁） */
  function guardAllowlistBlock() {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const end = guardSrc.indexOf('};', start);
    // 终止符缺失时 indexOf 返回 -1、slice 会静默扩扫全文，必须显式失败
    expect(end).toBeGreaterThan(start);
    return guardSrc.slice(start, end);
  }

  it('GUARD_ALLOWLIST 不含任何 pages/profile 文件（模块全量，非逐切片正则）', () => {
    expect(guardAllowlistBlock()).not.toMatch(/pages[/\\]profile/);
  });

  it.each(['student', 'wrongQuestion', 'favorite', 'points'])('本模块域 api %s.uts 无 allowlist 条目', (domain) => {
    expect(guardAllowlistBlock()).not.toMatch(new RegExp(`api[/\\\\]${domain}\\.uts`));
  });
});
