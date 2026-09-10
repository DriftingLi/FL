/**
 * dashboard 模块手术契约测试（refs #643 / refactor epic #638 T05）
 *
 * 沿用源码契约缝（.uvue/.uts 不可 jest import）。先例：profileContract（T03 模块汇总契约）、
 * forumContract（T04 五类锁 + 行为保持点）。本文件五类锁：
 * ① 本模块域 api 收紧（出口家族 + catch/降级保持 + 零裸 .then 残留）
 * ② 600 行软预算模块全量复检 + 目录 ≤2 层
 * ③ 拆出物接线收口（显式 import + 模板挂载 + 零孤儿）
 * ④ 页面层零直发请求
 * ⑤ allowlist 模块级清零
 * 外加行为保持点锁（轮询/乐观回滚/存储同步/静态数据/筛选抽屉）。
 *
 * 本票域 api 收紧口径（同 profileContract）：仅「有 DTO 映射」的函数经 *Mapped 出口家族；
 * 返回 void 的透传函数（viewFeaturedContentApi / markNotificationReadApi / markAllReadApi）不硬套 identity map。
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

/** dashboard 模块全部源文件（.uvue/.uts，排除测试） */
function dashboardSourceFiles() {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, 'pages/dashboard'));
  return out;
}

/* ══ ① 本模块域 api 收紧（featured / notification / credential；student 域已由 T03 收紧，此处复检） ══ */
describe('featured 域收紧（dashboard 新闻资讯数据源）', () => {
  const src = read('api/featured.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getFeaturedContentsApi / getFeaturedContentDetailApi 经 getMapped + build* 提取（箭头包裹，error17 先例）', () => {
    const list = fnBodyOf(src, 'getFeaturedContentsApi');
    expect(list).toContain('getMapped<FeaturedContentListResult>');
    expect(list).toMatch(/\(data : UTSJSONObject\) : FeaturedContentListResult => buildFeaturedListResult\(data\)/);
    const detail = fnBodyOf(src, 'getFeaturedContentDetailApi');
    expect(detail).toContain('getMapped<FeaturedContentDetail>');
    expect(detail).toMatch(/\(data : UTSJSONObject\) : FeaturedContentDetail => buildFeaturedDetailResult\(data\)/);
    expect(src).toMatch(/function buildFeaturedListResult/);
    expect(src).toMatch(/function buildFeaturedDetailResult/);
  });
  it('viewFeaturedContentApi（void 透传）保持原 post() 通路不硬套 mapper', () => {
    expect(fnBodyOf(src, 'viewFeaturedContentApi')).toContain('post(');
  });
});

describe('notification 域收紧（dashboard 未读轮询数据源）', () => {
  const src = read('api/notification.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getNotificationsApi / getUnreadCountApi 经 getMapped，字段读取保持（items/total/page/page_size/unread_count/count）', () => {
    const list = fnBodyOf(src, 'getNotificationsApi');
    expect(list).toContain('getMapped<NotificationListResult>');
    expect(list).toMatch(/\(data : UTSJSONObject\) : NotificationListResult => buildNotificationListResult\(data\)/);
    expect(src).toMatch(/function buildNotificationListResult/);
    const unread = fnBodyOf(src, 'getUnreadCountApi');
    expect(unread).toContain("getMapped<number>('/notifications/unread-count'");
    expect(unread).toContain("toNumber(data['count'])");
  });
  it('mark*（void 透传）保持原 post() 通路', () => {
    expect(fnBodyOf(src, 'markNotificationReadApi')).toContain('post(');
    expect(fnBodyOf(src, 'markAllReadApi')).toContain('post(');
  });
});

describe('credential 域收紧（dashboard 证件切换数据源）', () => {
  const src = read('api/credential.uts');
  it('引入 getMapped 与 requestMapped（PATCH 无便捷面，走 opts 同通路）', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*requestMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getCurrentCredentialApi 经 getMapped 且保留 catch + mock 列表回退（错误行为不得变）', () => {
    const body = fnBodyOf(src, 'getCurrentCredentialApi');
    expect(body).toContain('getMapped<CredentialItem | null>');
    expect(body).toContain(".catch((e) : CredentialItem | null =>");
    expect(body).toContain('getMockCredentialList()');
    expect(body).toContain("uni.getStorageSync('selected_cert')");
  });
  it('switchCredentialApi 经 requestMapped，PATCH 经 opts.method 赋值（与 patch() 逐字同通路）且保留 catch + mock 模拟切换', () => {
    const body = fnBodyOf(src, 'switchCredentialApi');
    expect(body).toContain('requestMapped<CredentialSwitchResult>');
    expect(body).toContain("opts.method = 'PATCH'");
    expect(body).toContain('opts.data = body');
    expect(body).toContain('.catch((e) : CredentialSwitchResult =>');
    expect(body).toContain('getMockCredentialList()');
  });
});

describe('raw .then 收紧完成度（本票域 DTO 函数零残留，student 已由 T03 达标一并复检）', () => {
  const targets = ['api/featured.uts', 'api/notification.uts', 'api/credential.uts', 'api/student.uts'];
  it.each(targets)('%s 不再出现 get/post/patch(...).then((data : UTSJSONObject) : DTO 形态', (rel) => {
    const src = read(rel);
    expect(src).not.toMatch(/return (get|post|patch)\([^)]*\)\s*\.then\(\(data : UTSJSONObject\) : (?!UTSJSONObject)/);
  });
});

/* ══ ② 600 行软预算 + 目录深度 ══ */
describe('600 行软预算机检（模块全量：pages/dashboard/** 全部源文件）', () => {
  it('走查范围非空（防路径断链导致空集合假绿：页面 + 5 组件 + 2 composable = 8）', () => {
    expect(dashboardSourceFiles().length).toBe(8);
  });

  it('主页面预算落袋锁（手术前 1423 行）', () => {
    const lines = read('pages/dashboard/dashboard.uvue').split('\n').length;
    expect(lines).toBeLessThanOrEqual(600);
    expect(lines).toBeLessThan(1000);
  });

  it('模块全部源文件 ≤600 行（含 components/composables，达标后锁住防回潮，先例 profileContract）', () => {
    const over = dashboardSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层（pages/dashboard/<页 或 <子目录>/<文件>）', () => {
    const deep = dashboardSourceFiles()
      .filter((f) => path.relative(path.join(ROOT, 'pages/dashboard'), f).split(/[\\/]/).length > 2)
      .map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

/* ══ ③ 拆出物接线收口 ══ */
describe('组件接线汇总（T05 拆出物 5 组件 + 2 composable：显式 import + 模板挂载/调用）', () => {
  const WIRING = [
    ['DashboardCertDropdown', './components/dashboard-cert-dropdown.uvue'],
    ['DashboardMenuGrid', './components/dashboard-menu-grid.uvue'],
    ['DashboardContinueCard', './components/dashboard-continue-card.uvue'],
    ['DashboardNewsSection', './components/dashboard-news-section.uvue'],
    ['DashboardCourseSection', './components/dashboard-course-section.uvue'],
  ];
  const page = read('pages/dashboard/dashboard.uvue');

  it.each(WIRING)('<%s>：页面显式 import %s 且模板挂载', (tag, importRel) => {
    expect(page).toContain(importRel);
    expect(page).toMatch(new RegExp(`<${tag}[\\s/>]`));
  });

  it('两个 composable 经模块内显式相对 import 且被调用（ADR-0007 模块私有安置）', () => {
    expect(page).toContain('./composables/use-dashboard-credential.uts');
    expect(page).toContain('./composables/use-dashboard-feeds.uts');
    expect(page).toContain('useDashboardCredential()');
    expect(page).toContain('useDashboardFeeds()');
  });

  it('拆出物零孤儿：components/ 与 composables/ 每个文件都被页面 import（新增拆出物必须接线）', () => {
    const pageSrc = page;
    const orphanOf = (dir, prefix) => fs.readdirSync(path.join(ROOT, 'pages/dashboard', dir))
      .filter((n) => /\.(uvue|uts)$/.test(n))
      .filter((n) => !pageSrc.includes(`${prefix}/${n}`))
      .sort();
    const orphans = [
      ...orphanOf('components', './components').map((n) => `components/${n}`),
      ...orphanOf('composables', './composables').map((n) => `composables/${n}`),
    ];
    expect(orphans).toEqual([]);
  });
});

/* ══ ④ 页面层零直发请求 ══ */
describe('页面层零直发请求（网络一律经域 api 函数，#643 收紧口径）', () => {
  it('pages/dashboard/** 无源文件 import api/request 或裸调 uni.request', () => {
    const hits = [];
    for (const f of dashboardSourceFiles()) {
      const src = fs.readFileSync(f, 'utf8');
      const rel = path.relative(ROOT, f);
      if (/from\s*'[^']*api\/request(\.uts)?'/.test(src)) hits.push(`${rel}: import api/request`);
      if (/uni\.request\s*\(/.test(src)) hits.push(`${rel}: uni.request 裸调`);
      if (/uni\.(upload|download)File\s*\(/.test(src)) hits.push(`${rel}: uni.uploadFile/downloadFile 裸调`);
    }
    expect(hits).toEqual([]);
  });
});

/* ══ ⑤ allowlist 模块级清零（dashboard 手术前即零豁免，本锁防回潮 + 域 api 口径） ══ */
describe('allowlist 模块级清零（dashboard 域 catch/detail 违例豁免不存在）', () => {
  function guardAllowlistBlock() {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const end = guardSrc.indexOf('};', start);
    expect(end).toBeGreaterThan(start);
    return guardSrc.slice(start, end);
  }

  it('GUARD_ALLOWLIST 不含任何 pages/dashboard 文件（模块全量，先例 profileContract）', () => {
    expect(guardAllowlistBlock()).not.toMatch(/pages[/\\]dashboard/);
  });

  it.each(['featured', 'notification', 'credential', 'student'])('本模块域 api %s.uts 无 allowlist 条目', (domain) => {
    expect(guardAllowlistBlock()).not.toMatch(new RegExp(`api[/\\\\]${domain}\\.uts`));
  });
});

/* ══ 行为保持点锁（先例 forumContract：跳转语义/乐观回滚/状态位置钉成断言） ══ */
describe('行为保持点（手术偏离与回退风险的显式钉锁）', () => {
  const page = read('pages/dashboard/dashboard.uvue');
  const cred = read('pages/dashboard/composables/use-dashboard-credential.uts');
  const feeds = read('pages/dashboard/composables/use-dashboard-feeds.uts');

  it('生命周期编排留在页面：onShow 四连刷 + onHide 停轮询', () => {
    const show = page.slice(page.indexOf('onShow('), page.indexOf('onHide('));
    expect(show).toContain('loadCredentials()');
    expect(show).toContain('loadMyCourses()');
    expect(show).toContain('loadFeaturedContents()');
    expect(show).toContain('startUnreadPolling()');
    const hide = page.slice(page.indexOf('onHide('));
    expect(hide).toContain('stopUnreadPolling()');
  });

  it('未读轮询 30s 间隔与先拉后轮语义保持', () => {
    const start = fnBodyOf2(feeds, 'startUnreadPolling');
    expect(start).toContain('stopUnreadPolling()');
    expect(start).toContain('loadUnreadCount()');
    expect(start).toContain('30000');
  });

  it('证件切换乐观更新 + 失败回滚（prevId/prevName）与错误文案保持', () => {
    const body = cred.slice(cred.indexOf('async function onSelectCredential'));
    expect(body).toContain('const prevId = currentCredentialId.value');
    expect(body).toContain('const prevName = currentCert.value');
    expect(body).toContain('currentCredentialId.value = prevId');
    expect(body).toContain('currentCert.value = prevName');
    expect(body).toContain('切换失败，请重试');
    expect(body).toContain('isSwitching.value = false');
  });

  it("selected_cert 存储同步点保持（load 1 + switch 2 + composable getStorageSync 回退 2）", () => {
    const syncWrites = (cred.match(/uni\.setStorageSync\('selected_cert'/g) || []).length;
    expect(syncWrites).toBe(3);
    const syncReads = (cred.match(/uni\.getStorageSync\('selected_cert'/g) || []).length;
    expect(syncReads).toBe(2);
  });

  it('筛选抽屉留页面（选中态重开保持）；组件不引入跨层 v-model（ADR-0007 T04 教训）', () => {
    expect(page).toContain('class="filter-mask"');
    expect(page).toContain('const selectedPrice = ref(0)');
    expect(page).toContain('const selectedType = ref(0)');
    expect(page).toContain('filter applied');
    const course = read('pages/dashboard/components/dashboard-course-section.uvue');
    expect(course).toContain("defineEmits(['openFilter'])");
    expect(page).toContain('@open-filter="onFilterBtnClick"');
    // 页面与抽屉之间不存在 modelValue/v-model
    expect(page).not.toMatch(/v-model[:.\\w]*=/);
  });

  it('死代码证书名回退映射表已删除（全模块零命中，消费面核对与删除说明见 composable 头注释）', () => {
    const all = dashboardSourceFiles().map((f) => fs.readFileSync(f, 'utf8')).join('\n');
    expect(all).not.toContain('certNameMap');
  });

  it('静态数据完整性：宫格 4 入口 / 课程 4 卡 / Tab 2 / 筛选标签 3 / 热门考证 2', () => {
    const menu = read('pages/dashboard/components/dashboard-menu-grid.uvue');
    for (const t of ['课程商城', '题库练习', '学习资料', '考情资讯']) expect(menu).toContain(t);
    const course = read('pages/dashboard/components/dashboard-course-section.uvue');
    for (const t of ['2026年叉车基础理论课程', '叉车实操技能强化班', '叉车安全规范专题课', '叉车维修高级进阶课']) expect(course).toContain(t);
    expect(course).toContain("['热门课程', '精品课程']");
    expect(course).toContain("['全部', '实操技能', '综合评审']");
    const dd = read('pages/dashboard/components/dashboard-cert-dropdown.uvue');
    expect(dd).toContain('入门组合');
    expect(dd).toContain('电动叉车维修优选');
    for (const lv of ['五级', '四级', '三级', '二级', '一级']) expect(dd).toContain(lv);
  });

  it('跳转语义保持：宫格 reLaunch、课程/新闻 navigateTo、公告与更多同路 featured-list', () => {
    const menu = read('pages/dashboard/components/dashboard-menu-grid.uvue');
    expect(menu).toContain('uni.reLaunch({ url: item.path })');
    const course = read('pages/dashboard/components/dashboard-course-section.uvue');
    expect(course).toContain('/pages/courses/course-detail?id=${course.id}');
    expect(page).toContain("url: '/pages/featured/featured-detail?id=' + news.content_id.toString()");
    expect((page.match(/\/pages\/featured\/featured-list/g) || []).length).toBe(2);
  });
});

/** composable 内局部函数体提取（非 export，缩进 "\n    }" 收尾） */
function fnBodyOf2(fileSrc, name) {
  const start = fileSrc.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  const end = fileSrc.indexOf('\n    }', start);
  return fileSrc.slice(start, end);
}
