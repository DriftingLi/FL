/**
 * exam 模块手术契约测试（T07，parent #645 / ADR-0007）
 *
 * 钉住 exam（模拟考）手术交付的契约：
 * 1) 600 行软预算：pages/exam/** 全部源文件 ≤600 行，**新建 composable 与 section 组件同样计入**
 *    （维护者裁定 B 硬化口径：防「把 905 行页面挪成 900 行 composable」的假达标）；api/mockExam.uts 一并纳入
 * 2) 模块目录 ≤2 层
 * 3) composable 接线：两个页面以显式 import 使用模块私有 composable
 * 4) 组件接线零孤儿：页面 import 的组件文件必须存在，组件文件必须被页面引用
 *    （#779 回归教训：practice.uvue 改为 import 四个组件却从未创建文件，master 编译中断）
 * 5) allowlist 不回潮：exam 域文件不得出现在 GUARD_ALLOWLIST
 * 6) 零直发请求：页面层不直接 uni.request
 * 7) 域 api 收紧：4 个 DTO 函数经 mapper-callback 出口，2 个 void 语义函数保持 raw 透传白名单
 * 8) 幻影路由锁（#662 口径）：api 层每条 /mock-exam 路由都落在后端 mock_exam.go 已注册清单内
 * 9) 删除禁区「exam 不用删」的行为保持点：随机组卷 / 进度保存 / 断点续考 / 未完成询问 /
 *    计时（含超时自动交卷）/ 交卷（含未答题数提示）/ 成绩跳转 / 退出保存 / 失败重试逐项仍在，
 *    pages.json 两条路由与模块文件集合不减
 * 10) 展示纯函数唯一实现：exam 模块零 getTypeName / isMultiChoice 第二实现（消费 utils/wrongQuestionDisplay）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const EXAM_PAGES = ['pages/exam/mock-exam.uvue', 'pages/exam/mock-exam-result.uvue'];

function examSourceFiles() {
  const out = [];
  const walk = (d) => {
    if (!fs.existsSync(d)) return;
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, 'pages/exam'));
  return out;
}

describe('600 行软预算机检（pages/exam/** + 模块域 api 达标后锁定）', () => {
  it('exam 模块全部源文件 ≤600 行（含新建 composable 与 section 组件）', () => {
    const files = examSourceFiles().concat(path.join(ROOT, 'api/mockExam.uts'));
    const over = files.map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层', () => {
    const deep = examSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/exam'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });

  it('扫描面非空（防 walk 静默失效导致假绿）', () => {
    expect(examSourceFiles().length).toBeGreaterThanOrEqual(7);
  });
});

describe('composable 接线契约（T07 拆分：显式 import 模块私有 composable）', () => {
  it('mock-exam.uvue import useMockExamSession composable', () => {
    expect(read('pages/exam/mock-exam.uvue')).toContain('./composables/useMockExamSession');
  });

  it('mock-exam-result.uvue import useMockExamResult composable', () => {
    expect(read('pages/exam/mock-exam-result.uvue')).toContain('./composables/useMockExamResult');
  });

  it('两个 composable 均声明显式结果类型（Kotlin error18 规避，守护规则 U 同款口径）', () => {
    expect(read('pages/exam/composables/useMockExamSession.uts')).toMatch(/export function useMockExamSession\(\)\s*:\s*UseMockExamSessionResult/);
    expect(read('pages/exam/composables/useMockExamResult.uts')).toMatch(/export function useMockExamResult\(\)\s*:\s*UseMockExamResultResult/);
  });
});

describe('组件接线零孤儿（#779 回归锁：import 的组件文件必须存在）', () => {
  it('页面 import 的每个模块私有组件文件都真实存在', () => {
    const missing = [];
    for (const page of EXAM_PAGES) {
      const src = read(page);
      const re = /from\s+'\.\/components\/([^']+\.uvue)'/g;
      let m;
      while ((m = re.exec(src)) !== null) {
        const target = path.join(ROOT, 'pages/exam/components', m[1]);
        if (!fs.existsSync(target)) missing.push(page + ' -> components/' + m[1]);
      }
    }
    expect(missing).toEqual([]);
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const dir = path.join(ROOT, 'pages/exam/components');
    if (!fs.existsSync(dir)) return;
    const pagesSrc = EXAM_PAGES.map((p) => read(p)).join('\n');
    const orphans = fs.readdirSync(dir)
      .filter((f) => f.endsWith('.uvue'))
      .filter((f) => !pagesSrc.includes('./components/' + f));
    expect(orphans).toEqual([]);
  });
});

describe('allowlist 不回潮（exam 域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 exam 域文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/pages[/\\]exam/);
    expect(block).not.toMatch(/api[/\\]mockExam\.uts/);
  });
});

describe('零直发请求（页面层不直接 uni.request）', () => {
  it.each(EXAM_PAGES)('%s 不直接调用 uni.request', (page) => {
    expect(read(page)).not.toMatch(/uni\.request\s*\(/);
  });
});

describe('域 api 收紧（T07 / ADR-0007）：DTO 函数经 mapper-callback 出口', () => {
  const api = read('api/mockExam.uts');

  it('引入 getMapped / postMapped 出口家族', () => {
    expect(api).toMatch(/import\s*\{[^}]*getMapped[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
  });

  const DTO_FNS = [
    'startMockExamApi',
    'resumeMockExamApi',
    'getMockExamResultApi',
    'getMockExamHistoryApi',
  ];

  it.each(DTO_FNS)('%s 经 mapper 出口（getMapped/postMapped + 箭头包裹 build*）', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    expect(body).toMatch(/(get|post)Mapped<[A-Za-z_$][\w$]*(?:\[\])?>\([^,]+,\s*[^,]+,\s*\(data : UTSJSONObject\) : [A-Za-z_$][\w$]*(?:\[\])? => build[A-Za-z_$][\w$]*\(data\)\)/);
  });

  // raw 透传白名单：刻意保留裸 post（返回体被调用方忽略，void 语义，T03/T05 口径）
  const RAW_PASSTHROUGH = ['saveMockExamApi', 'submitMockExamApi'];

  it.each(RAW_PASSTHROUGH)('%s 保持 raw 透传（裸 post），未误加 DTO', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    expect(body).toMatch(/return (get|post)\(/);
    expect(body).not.toMatch(/Mapped</);
  });

  it('api 层唯一一处静默回退是 history 探活的既有空表回退（本票按删除禁区原样保留）', () => {
    expect([...api.matchAll(/\.catch\(/g)].length).toBe(1);
    const start = api.indexOf('export function getMockExamHistoryApi');
    expect(api.slice(start, api.indexOf('\n}', start))).toContain('.catch(');
  });
});

describe('幻影路由锁（#662 口径）：api 层路由必须落在后端已注册清单内', () => {
  /** 抹掉注释（`://` 例外，防误杀 URL 字面量） */
  function stripComments(s) {
    return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
  }

  it('api/mockExam.uts 的每条路由都在 backend mock_exam.go 已注册', () => {
    const backend = stripComments(read('../../backend/internal/api/mock_exam.go'));
    expect(backend).toContain('rg.Group("/mock-exam"');
    const registered = [];
    const re = /g\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)"/g;
    let m;
    while ((m = re.exec(backend)) !== null) registered.push('/mock-exam' + m[2]);
    expect(registered.length).toBe(6);

    const api = stripComments(read('api/mockExam.uts'));
    // 拼接连形态：'/mock-exam/' + <id 变量> + '/save'
    const concatRe = /'\/mock-exam\/'\s*\+\s*[A-Za-z_$][\w$.]*\s*\+\s*'(\/[^']*)'/g;
    const used = [];
    let c;
    while ((c = concatRe.exec(api)) !== null) used.push('/mock-exam/:mock_exam_id' + c[1]);
    const rest = api.replace(concatRe, '');
    const litRe = /'(\/mock-exam\/[^']+)'/g;
    let l;
    while ((l = litRe.exec(rest)) !== null) used.push(l[1]);
    expect(used.length).toBe(6);

    const phantom = used.filter((u) => !registered.includes(u));
    expect(phantom).toEqual([]);
  });
});

describe('删除禁区「exam 不用删」：行为保持点逐项仍在', () => {
  const session = read('pages/exam/composables/useMockExamSession.uts');
  const page = read('pages/exam/mock-exam.uvue');
  const resultComposable = read('pages/exam/composables/useMockExamResult.uts');
  const resultPage = read('pages/exam/mock-exam-result.uvue');

  it('随机组卷：startMockExamApi 仍由会话发起', () => {
    expect(session).toContain('startMockExamApi(0, initDuration.value)');
  });

  it('进度保存：/save 仍随答题与计时器周期触发', () => {
    expect(session).toContain('saveMockExamApi(');
    expect(session).toContain('now - lastSaveTime > 30000');
    expect(page).toContain('session.doSaveProgress(false)');
  });

  it('断点续考：resume 模式仍走后端已存答案回灌', () => {
    expect(session).toContain('resumeMockExamApi(');
    expect(session).toContain("mode.value == 'resume' && saved != null");
    expect(page).toContain('session.applyOptions(options)');
  });

  it('未完成考试询问：in_progress 探活 + 继续考试/开始新考试二选一仍在', () => {
    expect(session).toContain('getMockExamHistoryApi(1, 5)');
    expect(session).toContain("== 'in_progress'");
    expect(session).toContain('继续上一场考试？选择"开始新考试"将重新组卷。');
    expect(session).toContain("cancelText: '开始新考试'");
  });

  it('计时：倒计时/警告/危险/停止（卸载与交卷）逐项仍在', () => {
    expect(session).toContain('setInterval(');
    expect(session).toContain('clearInterval(timerId)');
    expect(session).toContain('remainingTime.value <= 300');
    expect(session).toContain('remainingTime.value <= 60');
    expect(page).toContain('session.stopTimer()');
  });

  it('超时自动交卷：autoSubmitFn 间接入口与提示仍在', () => {
    expect(session).toContain('autoSubmitFn = doSubmit');
    expect(session).toContain('考试时间已到，自动交卷');
  });

  it('交卷：确认弹窗（含未答题数）、交卷失败恢复计时、成绩页跳转仍在', () => {
    expect(session).toContain("title: '交卷确认'");
    expect(session).toContain("'还有 ' + unanswered + ' 题未作答，确定交卷吗？'");
    expect(session).toContain('startTimer()');
    expect(session).toContain("url: '/pages/exam/mock-exam-result?mock_exam_id=' + examId");
    expect(session).toContain('uni.redirectTo');
  });

  it('退出保存：返回键弹窗保存进度后 goBack 仍在', () => {
    expect(page).toContain('退出考试将自动保存进度，下次可继续。确定退出吗？');
    expect(page).toContain('confirmText: ' + "'保存并退出'");
    expect(page).toContain('goBack()');
  });

  it('加载失败可见 + 重试：loadError 与重试入口仍在（无假题回退）', () => {
    expect(session).toContain("let msg = '加载考试失败'");
    expect(session).toContain('loadError.value = msg');
    expect(page).toContain('@click="onRetry"');
    expect(page).toContain('session.loadExam()');
  });

  it('成绩页：汇总/逐题详情/查看历史/失败重试逐项仍在', () => {
    expect(resultComposable).toContain('getMockExamResultApi(');
    expect(resultComposable).toContain("errorMsg.value = '加载成绩失败'");
    expect(resultComposable).toContain('考试 ID 无效');
    expect(resultPage).toContain("url: '/pages/profile/mock-exam-records'");
    expect(resultPage).toContain('@click="onRetry"');
  });

  it('模块文件集合不减：两条路由与模块文件均在（不删页面/路由）', () => {
    const routes = [...read('pages.json').matchAll(/"path"\s*:\s*"([^"]+)"/g)].map((m) => m[1]);
    expect(routes).toContain('pages/exam/mock-exam');
    expect(routes).toContain('pages/exam/mock-exam-result');
    for (const rel of [
      'pages/exam/mock-exam.uvue',
      'pages/exam/mock-exam-result.uvue',
      'api/mockExam.uts',
      'pages/profile/mock-exam-records.uvue',
    ]) {
      expect(exists(rel)).toBe(true);
    }
  });
});

describe('展示纯函数唯一实现（T03/T06 口径）：exam 模块零题型判定第二实现', () => {
  it('exam 模块内不重新声明 getTypeName / isMultiChoice', () => {
    for (const f of examSourceFiles()) {
      const src = fs.readFileSync(f, 'utf8');
      expect(src).not.toContain('function getTypeName');
      expect(src).not.toContain('function isMultiChoice');
    }
  });

  it('两个 composable 均消费 utils/wrongQuestionDisplay 的唯一实现', () => {
    expect(read('pages/exam/composables/useMockExamSession.uts')).toMatch(/import\s*\{[^}]*getTypeName[^}]*\}\s*from\s*'\.\.\/\.\.\/\.\.\/utils\/wrongQuestionDisplay'/);
    expect(read('pages/exam/composables/useMockExamResult.uts')).toMatch(/import\s*\{[^}]*getTypeName[^}]*\}\s*from\s*'\.\.\/\.\.\/\.\.\/utils\/wrongQuestionDisplay'/);
  });
});
