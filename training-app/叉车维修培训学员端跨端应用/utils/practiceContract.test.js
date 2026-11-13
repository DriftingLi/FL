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

const PRACTICE_PAGES = ['pages/practice/practice.uvue', 'pages/practice/practice-do.uvue', 'pages/practice/real-exam-papers.uvue'];

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

describe('离开页面停表（#1727：系统返回/手势返回不走 onExit）', () => {
  const doPage = read('pages/practice/practice-do.uvue');

  function hookBody(name) {
    const re = new RegExp(name + '\\s*\\(\\s*\\(\\s*\\)\\s*:\\s*void\\s*=>\\s*\\{([\\s\\S]*?)\\n\\s*\\}\\s*\\)');
    const m = doPage.match(re);
    return m ? m[1] : '';
  }

  it('导入 onHide / onUnload 生命周期钩子', () => {
    expect(doPage).toMatch(/import\s*\{[^}]*\bonHide\b[^}]*\bonUnload\b[^}]*\}\s*from\s*'@dcloudio\/uni-app'/);
  });

  it('onHide 停表（页面隐藏/进后台）', () => {
    expect(hookBody('onHide')).toContain('session.stopTimer()');
  });

  it('onUnload 停表（页面销毁兜底）', () => {
    expect(hookBody('onUnload')).toContain('session.stopTimer()');
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
    // 形态一：整体箭头包裹 build*(data)（原状）
    // 形态二（ADR-0037 PR-B）：装载改走信封归一出口 decodeList/decodePage，仍由箭头包裹的 build*(obj) 逐项映射
    expect(body).toMatch(/(get|post)Mapped<[A-Za-z_$][\w$]*(?:<[A-Za-z_$][\w$]*>)?(?:\[\])?>\('[^']+'(?:\s*\+\s*[^,]+)?, [^,]+, \(data : UTSJSONObject\) : [A-Za-z_$][\w$]*(?:\[\])? => build[A-Za-z_$][\w$]*\(data\)\)|(get|post)Mapped<[A-Za-z_$][\w$]*(?:<[A-Za-z_$][\w$]*>)?(?:\[\])?>\('[^']+'(?:\s*\+\s*[^,]+)?, [^,]+, \(data : UTSJSONObject\) : [^=]+=> decode(?:List|Page)<[A-Za-z_$][\w$]*>\(data, '[^']+', [^;]*build[A-Za-z_$][\w$]*\(obj\)/);
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

  it('practice 域 api 的每个路由都在 practicemode/handler.go / questionbank/handler.go 已注册', () => {
    const registered = [
      ...registeredRoutes('../../backend/internal/practicemode/handler.go', '/practice-mode'),
      ...registeredRoutes('../../backend/internal/questionbank/handler.go', '/question-bank'),
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

describe('考点正确率接线（#1948：摘 mockAccuracy，改接 stats 的 by_tag）', () => {
  const list = read('pages/practice/components/practice-module-list.uvue');
  const comp = read('pages/practice/composables/usePracticeOverview.uts');
  const page = read('pages/practice/practice.uvue');
  const apiSrc = read('api/practice.uts');
  const types = read('types/practice.uts');

  it('组件里 mock 彻底离场（函数、调用、派生式三者都不得残留）', () => {
    expect(list).not.toContain('mockAccuracy');
    expect(list).not.toMatch(/60 \+ \(tagId/);
  });

  it('副标题文案来自 utils/practiceModuleDisplay 唯一实现（组件不自带第二份文案）', () => {
    expect(list).toContain("import { moduleMetaText } from '../../../utils/practiceModuleDisplay'");
    expect(list).toContain('return moduleMetaText(m.completed, m.total, m.accuracy, m.has_accuracy)');
    expect(list).not.toMatch(/\n\s*function accuracyText\(/);
  });

  it('api 层导出 by_tag 装载出口并读后端新键', () => {
    expect(apiSrc).toContain('export function getPracticeTagAccuracyApi');
    expect(apiSrc).toContain("data['by_tag']");
  });

  it('正确率与题数落同一证件分区（#702 口径：不传则两处会错开）', () => {
    expect(apiSrc).toContain("params['credential_id']");
    expect(comp).toContain('getPracticeTagAccuracyApi(browseCredentialId)');
  });

  it('composable 声明新两字段并消费装载出口（悬空 import 是编译盲区，#779 同类）', () => {
    expect(comp).toContain('accuracy : number; has_accuracy : boolean');
    expect(comp).toContain('getPracticeTagAccuracyApi');
    expect(comp).toContain('has_accuracy: false');
  });

  it('类型面有 PracticeTagAccuracy 且 index 有出口（缺出口则消费侧 error18 找不到名称）', () => {
    expect(types).toContain('export type PracticeTagAccuracy');
    expect(read('types/index.uts')).toContain('type PracticeTagAccuracy');
  });

  it('回本页轻量刷正确率，不重跑逐标签进度面', () => {
    expect(comp).toContain('loadTagAccuracy');
    expect(page).toContain('loadTagAccuracy()');
  });
});

describe('积分解锁兑换接线（#2462：二次确认 → 兑换 → 以 entitled 为准重拉）', () => {
  const papersPage = read('pages/practice/real-exam-papers.uvue');

  /** 取函数体：从声明处到下一个 8 空格缩进的 `}`（页面里没有嵌套到同级的块） */
  function bodyOf(signature) {
    const start = papersPage.indexOf(signature);
    expect(start).toBeGreaterThan(-1);
    return papersPage.slice(start, papersPage.indexOf('\n    }', start));
  }

  const redeem = bodyOf('async function doRedeemPaper');
  const click = bodyOf('function onRedeem');

  it('只有未解锁卷渲染解锁按钮，已解锁卷不留按钮', () => {
    expect(papersPage).toMatch(/<app-button v-if="!p\.entitled"[^>]*@click="onRedeem\(p\)"><\/app-button>/);
  });

  it('按钮文案与在途文案来自 utils/realExamPapersDisplay（页面不留第二份字面量）', () => {
    expect(papersPage).toContain('paperRedeemButtonText(redeemingPaperId.value == p.paper_id)');
    expect(papersPage).not.toMatch(/'积分解锁'/);
  });

  it('在途期间所有解锁按钮禁用、且点击守卫再拦一次（AC④ 不可重复点击）', () => {
    expect(papersPage).toMatch(/<app-button v-if="!p\.entitled"[^>]*:disabled="redeemingPaperId != 0"/);
    expect(click).toMatch(/redeemingPaperId\.value != 0\)\s*\{\s*\n\s*return/);
  });

  it('点击先出二次确认，确认文案含该卷价格；仅确认才发请求（取消不发，AC①）', () => {
    expect(click).toContain('uni.showModal(');
    expect(click).toMatch(/content:\s*paperRedeemConfirmText\(p\.price\)/);
    expect(click).toMatch(/if \(res\.confirm\) \{\s*\n\s*doRedeemPaper\(p\)/);
    expect(click).not.toContain('redeemPaperApi');
  });

  it('兑换走 api/realExam 的 redeemPaperApi（页面不直连请求，AC⑥ 无后端改动）', () => {
    expect(papersPage).toMatch(/import\s*\{[^}]*redeemPaperApi[^}]*\}\s*from\s*'\.\.\/\.\.\/api\/realExam'/);
    expect(redeem).toContain('await redeemPaperApi(p.paper_id)');
    expect(papersPage).not.toMatch(/uni\.request\s*\(/);
  });

  it('成败都重拉列表：catch 只记原因、不 return 不提前收尾，await loadPapers(true) 在 catch 之后（AC②③）', () => {
    expect(redeem).toMatch(/catch[\s\S]*?errMsg\(e, '兑换失败，请稍后重试'\)/);
    expect(redeem).not.toMatch(/catch[\s\S]*?\breturn\b/);
    expect(redeem.indexOf('await loadPapers(true)')).toBeGreaterThan(redeem.indexOf('catch'));
  });

  it('解锁状态只认服务端：entitled / price 一概不本地赋值（AC⑤）', () => {
    expect(papersPage).not.toMatch(/\.entitled\s*=(?!=)/);
    expect(papersPage).not.toMatch(/\.price\s*=(?!=)/);
  });

  it('全链路没有按响应文案判「已兑换」的字符串比对（AC⑤ / ADR-0024）', () => {
    expect(papersPage).not.toMatch(/(indexOf|includes|match|test)\(\s*['"]已兑换/);
    expect(papersPage).not.toMatch(/['"]已兑换['"]/);
  });

  it('失败提示 = 后端原因 + 重拉后的真实状态，并给出下一步落点（AC③）', () => {
    const fail = bodyOf('function notifyRedeemFailed');
    expect(fail).toMatch(/content:\s*paperRedeemFailText\(reason, stillLocked\)/);
    expect(fail).toContain("uni.navigateTo({ url: '/pages/points/task-center' })");
  });
});
