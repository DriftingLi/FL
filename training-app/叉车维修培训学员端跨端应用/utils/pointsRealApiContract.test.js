/**
 * 积分前端接真接口契约（refs #709，来源 #642 分歧⑤ / #662 缺口清单）
 *
 * 本票把「积分域三处假数据」一次收口，测试即收口的锁：
 * 1) 路由真实：api/points.uts 只打后端已注册路由（points.go RegisterPointsRoutes），
 *    /points/records 与 /points/course-view 是从未注册的幻影路由（#662 实测 404），必须绝迹。
 * 2) 零 mock 静默回退：幻影 404 之所以没露馅，全靠 api 层 .catch() 回落占位数据；
 *    回退一退役，失败必须上抛到页面（「占位数据不是后端异常的正确呈现」，同 #408/#409 口径）。
 * 3) 页面接线：明细页走 /points/ledger 的 direction 收支筛选（#512），任务中心走
 *    /points/tasks + claim 三态，看课上报调用整体静默删除。
 *
 * .uvue/.uts 无法被 jest 直接 import，沿用仓库既有源码契约缝（见 mallPilotContract.test.js）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/**
 * 抹掉块注释与行注释（`://` 例外，防误杀 URL 字面量）。
 * 历史说明注释里会提到 /points/records、mock、.catch() 这些「已退役」的名词，
 * 断言只针对代码本体，不针对注释——注释越写越清楚不该把测试搞红。
 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

const apiSrc = stripComments(read('api/points.uts'));
const typesSrc = read('types/index.uts');
const detailSrc = read('pages/points/points-detail.uvue');
const taskSrc = stripComments(read('pages/points/task-center.uvue'));
const chapterSrc = stripComments(read('pages/courses/chapter-view.uvue'));
const profileSrc = read('pages/profile/profile.uvue');

/** 取 .uvue 的 <template> 段 */
function tplOf(src) {
  const start = src.indexOf('<template>');
  const end = src.lastIndexOf('</template>');
  return src.slice(start, end);
}

/** 取某个顶层函数的函数体（到下一个行首 `}` 为止） */
function fnBody(src, marker) {
  const start = src.indexOf(marker);
  expect(start).toBeGreaterThan(-1);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('幻影路由绝迹（#662：后端从未注册，404 被 catch-mock 掩盖）', () => {
  it('api/points.uts 不再请求 /points/records 与 /points/course-view', () => {
    expect(apiSrc).not.toContain('/points/records');
    expect(apiSrc).not.toContain('/points/course-view');
    expect(apiSrc).not.toContain('reportCourseViewPointsApi');
  });

  it('api/points.uts 请求的每个积分路由都在后端 points.go 已注册清单内', () => {
    const registered = [
      '/points/balance',
      '/points/ledger',
      '/points/tasks',
      '/points/shop/course/',
      '/points/shop/',
    ];
    const routes = [...apiSrc.matchAll(/['"](\/points\/[^'"]*)['"]/g)];
    expect(routes.length).toBeGreaterThan(0);
    for (const m of routes) {
      expect(registered.some((r) => m[1].startsWith(r))).toBe(true);
    }
  });
});

describe('catch-mock 静默回退退役（失败要可见）', () => {
  it('api/points.uts 无 mock 构造器与占位数据', () => {
    expect(apiSrc).not.toMatch(/mock/i);
  });

  it('api/points.uts 不吞错：全文件零 .catch(，失败直带上抛页面', () => {
    expect(apiSrc).not.toContain('.catch(');
  });

  it('出口全部经 mapper-callback 家族（#639 出口，与 #652 T14 同文件收口）', () => {
    expect(apiSrc).toMatch(/import\s*\{[^}]*getMapped[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(apiSrc).toContain("getMapped<PointsBalance>('/points/balance'");
    expect(apiSrc).toContain('getMapped<PointsLedgerResult>(');
    expect(apiSrc).toContain('getMapped<PointsTasksResult>(');
    expect(apiSrc).toContain('postMapped<PointsClaimResult>(');
  });
});

describe('流水接口对齐后端 /points/ledger 契约（#512 direction）', () => {
  it('解析 items 而非 records，页码参数为 page / page_size', () => {
    const body = fnBody(apiSrc, 'export function getPointsLedgerApi');
    expect(body).toContain("'/points/ledger'");
    expect(body).toContain('page_size');
    const itemBuilder = fnBody(apiSrc, 'function buildPointsLedgerItem');
    expect(itemBuilder).toContain("obj['delta']");
    expect(itemBuilder).toContain("obj['reason']");
    expect(itemBuilder).toContain("obj['expires_at']");
    const resultBuilder = fnBody(apiSrc, 'function buildPointsLedgerResult');
    expect(resultBuilder).toContain("obj['items']");
    expect(resultBuilder).not.toContain("obj['records']");
  });

  it('direction 仅在收入/支出时透传（全部不传；后端无 source_type 维度，幻影筛选退役）', () => {
    const body = fnBody(apiSrc, 'export function getPointsLedgerApi');
    expect(body).toContain('direction');
    expect(body).toMatch(/if\s*\(\s*direction\s*!=\s*''\s*\)/);
    expect(body).not.toContain('source_type');
    expect(body).not.toContain('type:');
  });
});

describe('任务中心接口：GET /points/tasks + POST /points/tasks/{code}/claim', () => {
  it('api 层导出任务列表与领取两个出口，字段逐一对齐后端 DTO', () => {
    expect(apiSrc).toContain('export function getPointsTasksApi');
    const claim = fnBody(apiSrc, 'export function claimPointsTaskApi');
    expect(claim).toContain("'/points/tasks/'");
    expect(claim).toContain("'/claim'");
    expect(claim).toContain('postMapped<PointsClaimResult>');
    const taskBuilder = fnBody(apiSrc, 'function buildPointsTaskItem');
    for (const f of ['code', 'group', 'title', 'desc', 'points', 'status', 'progress', 'total']) {
      expect(taskBuilder).toContain(`obj['${f}']`);
    }
    const claimBuilder = fnBody(apiSrc, 'function buildPointsClaimResult');
    expect(claimBuilder).toContain("obj['task_status']");
  });

  it('types 层按后端 DTO 定义积分类型，幻影 records 类型退役', () => {
    for (const t of ['PointsLedgerItem', 'PointsLedgerResult', 'PointsTaskItem', 'PointsTasksResult', 'PointsClaimResult']) {
      expect(typesSrc).toContain(`export type ${t} = {`);
    }
    expect(typesSrc).not.toContain('PointsRecordListResult');
    expect(typesSrc).not.toContain('export type PointsRecord =');
    const bal = typesSrc.slice(typesSrc.indexOf('export type PointsBalance = {'));
    const balBody = bal.slice(0, bal.indexOf('}') + 1);
    expect(balBody).toContain('balance : number');
    expect(balBody).toContain('total_earned : number');
    expect(balBody).toContain('total_spent : number');
    expect(balBody).not.toContain('total_points');
    expect(balBody).not.toContain('today_earned');
  });
});

describe('points-detail.uvue 接线（切实路由 + 失败可见）', () => {
  it('只 import 真接口出口，三 tab 走 direction 查询', () => {
    expect(detailSrc).toContain('getPointsLedgerApi');
    expect(detailSrc).not.toContain('getPointsRecordListApi');
    expect(detailSrc).toMatch(/'in'/);
    expect(detailSrc).toMatch(/'out'/);
    expect(detailSrc).toMatch(/'all'/);
  });

  it('「即将过期」按条目 expires_at 客户端筛（后端过期维度仍待补，见 #709 待后端清单）', () => {
    expect(detailSrc).toContain('expiringWithinDays');
  });

  it('加载失败呈现可见错误态并可重试（不再静默显示占位流水）', () => {
    expect(detailSrc).toMatch(/loadFailed/);
    expect(detailSrc).toMatch(/重试/);
  });

  it('列表渲染消费 ledger 真实字段 delta / reason，mock 字段 action / description 退役', () => {
    const tpl = tplOf(detailSrc);
    expect(tpl).toMatch(/item\.delta/);
    expect(tpl).toMatch(/item\.reason/);
    expect(tpl).not.toMatch(/item\.action/);
    expect(tpl).not.toMatch(/item\.description/);
    expect(tpl).not.toMatch(/item\.source_type/);
  });
});

describe('task-center.uvue 接线（真任务 + 真余额 + 三态领取）', () => {
  it('import 三个真接口出口', () => {
    expect(taskSrc).toContain('getPointsTasksApi');
    expect(taskSrc).toContain('claimPointsTaskApi');
    expect(taskSrc).toContain('getPointsBalanceApi');
  });

  it('硬编码余额 42 / 今日 80 与占位任务数组全部退役', () => {
    expect(taskSrc).not.toMatch(/ref<number>\(42\)/);
    expect(taskSrc).not.toMatch(/ref<number>\(80\)/);
    expect(taskSrc).not.toContain('const dailyTasks = ref<');
    expect(taskSrc).not.toContain('const welfareTasks = ref<');
  });

  it('任务分组与三态取自后端 group / status，按钮含领取动作', () => {
    expect(taskSrc).toMatch(/'daily'/);
    expect(taskSrc).toMatch(/'newbie'/);
    expect(taskSrc).toMatch(/'growth'/);
    expect(taskSrc).toMatch(/'claimable'/);
    expect(taskSrc).toMatch(/'claimed'/);
    expect(taskSrc).toMatch(/'todo'/);
    expect(taskSrc).toContain('领取');
  });

  it('模板渲染后端真实任务字段（title/desc/points/status/progress/total 全上阵）', () => {
    const tpl = tplOf(taskSrc);
    for (const f of ['task.title', 'task.desc', 'task.points', 'task.status', 'task.progress', 'task.total']) {
      expect(tpl).toContain(f);
    }
  });
});

describe('chapter-view.uvue：看课上报调用静默删除', () => {
  it('既无 import 也无调用，不留死函数（删除须静默：不得新增 404/报错提示）', () => {
    expect(chapterSrc).not.toContain('reportCourseViewPoints');
    expect(chapterSrc).not.toContain("api/points'");
  });
});

describe('profile.uvue 跟随余额字段收口', () => {
  it('可用积分读后端 balance 字段', () => {
    expect(profileSrc).toMatch(/data\.balance/);
    expect(profileSrc).not.toMatch(/data\.total_points/);
  });
});
