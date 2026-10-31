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
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

/**
 * 抹掉块注释与行注释（`://` 例外，防误杀 URL 字面量）。
 * 历史说明注释里会提到 /points/records、mock、.catch() 这些「已退役」的名词，
 * 断言只针对代码本体，不针对注释——注释越写越清楚不该把测试搞红。
 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

const apiSrc = stripComments(read('api/points.uts'));
const typesSrc = read('types/points.uts');
const detailSrc = read('pages/points/points-detail.uvue');
const rulesSrc = read('pages/points/points-rules.uvue');
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

  /**
   * 判别力注入（#1421：新增课程兑换动词的字面量必须真的在射程内）。
   *
   * ⚠️ 如实处理「拼接起点字面量」这件事：api 里 `'/points/shop/course/' + id + '/redeem'`
   * 的首段 `'/points/shop/course/'` 只是**前缀**，光拿它比对后端整路由（`.../:courseId/redeem`）
   * 永远不中 —— 所以判据要先把**动词函数体**里的三段（前缀 + 参数槽 + 后缀）拼回整路径，
   * 再用 `:param` 展开成模式的正则去命中。这样才既抓得住「漏了 /redeem / 走错子路径」，
   * 又不会把合法 SKU 名（如恰好叫 courses 的商品）误杀（那本该走通用 shop redeem，不算幻影）。
   */
  it('判据具备红能力：课程兑换拼回整路径命中注册模式，漏 /redeem 后缀必须出清单', () => {
    const patterns = [
      '/points/balance',
      '/points/ledger',
      '/points/tasks',
      '/points/tasks/:code/claim',
      '/points/shop/course/:courseId/redeem',
      '/points/shop/:sku/redeem',
    ].map((p) => new RegExp('^' + p.replace(/:[A-Za-z_]+/g, '[^/]+') + '$'));
    const shapeMatch = (full) => patterns.some((re) => re.test(full));
    // 动词函数体真的把三段拼成了后端那条整路径
    const courseBody = fnBody(apiSrc, 'export function redeemCoursePointsApi');
    const prefix = /'(\/points\/shop\/course\/)'/.exec(courseBody)[1];
    expect(prefix + '7/redeem').toBe('/points/shop/course/7/redeem');
    expect(shapeMatch('/points/shop/course/7/redeem')).toBe(true);
    // 反向：漏掉 /redeem 后缀（拼错管线）必须被判幻影
    expect(shapeMatch(prefix + '7')).toBe(false);
    // 历史幻影仍出清单
    expect(shapeMatch('/points/records')).toBe(false);
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
    // PR-F #1666：流水列表改走信封归一出口 getPaged（统一 Page<PointsLedgerItem>）
    expect(apiSrc).toContain("getPaged<PointsLedgerItem>('/points/ledger', page, pageSize");
    expect(apiSrc).toContain('getMapped<PointsTasksResult>(');
    expect(apiSrc).toContain('postMapped<PointsClaimResult>(');
  });
});

describe('流水接口对齐后端 /points/ledger 契约（#512 direction）', () => {
  it('解析 items 而非 records，页参由信封出口注入', () => {
    const body = fnBody(apiSrc, 'export function getPointsLedgerApi');
    expect(body).toContain("'/points/ledger'");
    // 页参（page / page_size）不再手拼，由 getPaged 统一注入；数组键 items 由 keys 声明承载
    expect(body).toContain('getPaged<PointsLedgerItem>');
    expect(body).toContain("['items']");
    expect(body).not.toContain("params['page_size']");
    const itemBuilder = fnBody(apiSrc, 'function buildPointsLedgerItem');
    expect(itemBuilder).toContain("obj['delta']");
    expect(itemBuilder).toContain("obj['reason']");
    expect(itemBuilder).toContain("obj['expires_at']");
    // 旧「构造 PointsLedgerResult 再拆包」的中间层已删（PR-F #1666）
    expect(apiSrc).not.toMatch(/function\s+buildPointsLedgerResult/);
    expect(apiSrc).not.toContain('PointsLedgerResult');
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
    for (const t of ['PointsLedgerItem', 'PointsTaskItem', 'PointsTasksResult', 'PointsClaimResult']) {
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

describe('课程兑换动词（#1421 / 移动端 ADR-0031：接后端 points.go 的 shop/course/:courseId/redeem）', () => {
  it('导出 redeemCoursePointsApi，经 postMapped DTO 出口、复用 PointsRedeemResult（不新造响应形状）', () => {
    expect(apiSrc).toContain('export function redeemCoursePointsApi');
    const body = fnBody(apiSrc, 'export function redeemCoursePointsApi');
    expect(body).toContain("'/points/shop/course/'");
    expect(body).toContain("'/redeem'");
    expect(body).toContain('postMapped<PointsRedeemResult>');
    // encodeURIComponent 纪律与 claim/redeemShopItem 同款（id 是数字仍需显式过一遍，防调用方塞串）
    expect(body).toContain('encodeURIComponent');
  });

  it('出口家族同步：PointsRedeemResult 的两个动词共用同一 builder（兑换管线唯一、形状唯一）', () => {
    expect((apiSrc.match(/buildPointsRedeemResult/g) || []).length).toBe(3); // 1 定义 + 2 消费
    expect(apiSrc).toContain("postMapped<PointsRedeemResult>(url, null, (data : UTSJSONObject) : PointsRedeemResult => buildPointsRedeemResult(data))");
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

  it('「即将过期」筛选整体退役（根 ADR-0082：积分暂不过期，#1943）', () => {
    expect(detailSrc).not.toContain('expiringWithinDays');
    expect(detailSrc).not.toContain('只看即将过期');
  });

  it('「积分规则」死标签接上跳转（#1943 静态规则页）', () => {
    expect(detailSrc).toContain('/pages/points/points-rules');
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

describe('points-rules.uvue 静态规则页（#1943：分值对齐后端配置 + 明示暂不过期）', () => {
  it('获取段 10 项任务逐条对齐现行后端配置（000002 种子 + 000021/000034 调整）', () => {
    const rows = [
      ['每日登录', '5'],
      ['每日答题 1 次', '10'],
      ['浏览 3 篇帖子', '5'],
      ['发布 1 篇帖子', '10'],
      ['回复 3 次', '10'],
      ['完成 1 次模考', '20'],
      ['完善基础资料', '10'],
      ['完善联系资料', '10'],
      ['选定目标证件', '10'],
      ['完成首节课程', '20'],
    ];
    for (const [name, points] of rows) {
      const i = rulesSrc.indexOf('rule-name">' + name + '</text>');
      expect(i).toBeGreaterThan(-1);
      const j = rulesSrc.indexOf('rule-points">+' + points + '<', i);
      expect(j).toBeGreaterThan(i);
      expect(j - i).toBeLessThan(220);
    }
  });

  it('每日组写明 0 点北京时间重置口径（FAQ 000036 points 类）', () => {
    expect(rulesSrc).toContain('每天 0 点北京时间重置');
  });

  it('有效期段明示暂不过期；模板可见文案无「即将过期」、无返点条款（根 ADR-0082 决策 4/5）', () => {
    const tpl = tplOf(rulesSrc);
    expect(tpl).toContain('积分暂不过期');
    expect(tpl).not.toContain('即将过期');
    expect(tpl).not.toContain('每实付');
  });

  it('pages.json 注册规则页路由', () => {
    expect(read('pages.json')).toContain('pages/points/points-rules');
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

describe('task-center.uvue 分组收敛（ADR-0054：growth 组退役，任务 code 保留）', () => {
  /** 取源码里的 order 白名单 —— 分组值域与顺序的唯一来源 */
  function groupOrder(src) {
    const m = src.match(/const\s+order\s*:\s*string\[\]\s*=\s*\[([^\]]*)\]/);
    expect(m).not.toBeNull();
    return m[1].split(',').map((s) => s.trim().replace(/'/g, '')).filter((s) => s.length > 0);
  }

  it('growth 分组死代码绝迹：label / desc / icon 三处分支 + order 白名单 + .icon-growth CSS', () => {
    // 'growth' 带引号 ⇒ 只可能来自分组字面量，不会误伤 'growth_post' 这类任务 code
    expect(taskSrc).not.toMatch(/'growth'/);
    expect(taskSrc).not.toContain('icon-growth'); // groupIconClass 的返回分支及其 CSS 规则
    expect(taskSrc).not.toContain('成长任务');
    expect(taskSrc).not.toContain('每日达成每日领');
  });

  it('三个 growth_* 任务 code 未被误删（分组改判 daily，code 是流水里的历史标识）', () => {
    for (const code of ['growth_post', 'growth_reply', 'growth_mock']) {
      expect(taskSrc).toContain(`'${code}'`);
    }
  });

  it('order 白名单只剩 daily / newbie；迁移 000034 + 000045 后的真实形状落成两段且兜底段永不命中', () => {
    const order = groupOrder(taskSrc);
    expect(order).toEqual(['daily', 'newbie']);
    // 夹具与迁移 000034 后的生产种子同形：三条 growth_* 的 group 已改判 daily
    // browse_course = 迁移 000045（后端 #2065）新增的 daily 任务；端上渲染仍全按后端返回，此夹具只锁「形状不凭空多段」
    const tasks = ['daily_login', 'daily_quiz', 'daily_browse', 'browse_course', 'growth_post', 'growth_reply', 'growth_mock']
      .map((code) => ({ code, group: 'daily' }))
      .concat(['newbie_profile_basic', 'newbie_profile_contact', 'newbie_credential', 'newbie_first_course']
        .map((code) => ({ code, group: 'newbie' })));
    // 空段判据：order 内每个分组都非空（buildGroups 只在 bucket.length > 0 时 push 一段）
    expect(order.map((g) => tasks.filter((t) => t.group === g).length)).toEqual([7, 4]);
    // 兜底判据：没有任务落在 order 之外 ⇒ rest 恒空，「其他任务」段不会渲染
    expect(tasks.filter((t) => order.indexOf(t.group) < 0)).toEqual([]);
  });
});

describe('#1944 browse_course：图标 / 「去完成」落点 / 流水事由（后端 #2065 新任务码，不复用 daily_browse）', () => {
  const displaySrc = stripComments(read('utils/pointsDisplay.uts'));

  it('任务中心有独立图标，且不与论坛的 daily_browse 共用一条分支', () => {
    expect(taskSrc).toContain("if (code == 'browse_course') return '🎓'");
    expect(taskSrc).toContain("if (code == 'daily_browse') return '📖'");
  });

  it('「去完成」落课程列表 tab，不落论坛（两个码语义不同，#2065 评论点名别串台）', () => {
    expect(taskSrc).toContain("if (code == 'browse_course') return { url: '/pages/courses/courses', tab: true } as TaskRoute");
    for (const line of taskSrc.split(/\r?\n/).filter((l) => l.includes("'browse_course'"))) {
      expect(line).not.toContain('forum');
    }
  });

  it('领取后流水事由映射「浏览课程」（后端 Claim 落账 Reason = 任务 code，缺映射会把裸 code 甩给用户）', () => {
    expect(displaySrc).toContain("if (reason == 'browse_course') return '浏览课程'");
  });

  it('端上不写死任务文案与分值：标题/描述/points 全部读 GET /points/tasks 返回（Q11 定案）', () => {
    expect(taskSrc).not.toContain('浏览 3 门课程'); // 文案唯一来源是后端下发
    expect(taskSrc).not.toMatch(/今日可得[^\n]*6[05]/);
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
