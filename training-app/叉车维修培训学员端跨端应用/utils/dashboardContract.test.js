/**
 * dashboard 模块手术契约测试（refs #643 / refactor epic #638 T05）
 *
 * 沿用源码契约缝（.uvue/.uts 不可 jest import）。先例：profileContract（T03 模块汇总契约）、
 * forumContract（T04 五类锁 + 行为保持点）。本文件五类锁：
 * ① 本模块域 api 收紧（出口家族 + catch/降级保持 + 零裸 .then 残留）
 *    ⚠️ credential 域两条口径已被推翻：#1346 去掉 switch 的 mock 假成功；**#1349** 把证件列表
 *    真源切到后端 `GET /credentials/grouped`、硬编码字典整删、离线回退改读「后端确认值」缓存，
 *    并同批新建 credential 域**幻影路由锁**（#662 口径，此前独缺）—— 见下面两个 describe。
 * ② 页面预算落袋锁（模块全量预算 / 目录 ≤2 层已由声明面执法，见 ADR-0023 票 C #1219）
 * ③ 拆出物接线收口（显式 import + 模板挂载 + 零孤儿）
 * ④ 页面层零直发请求
 * ⑤ 机械坑位零命中：执法点在全工程守护（`utils/utsAndroidCompile.test.js` 规则 H/I，#654 起无豁免），
 *    本文件不再各写一遍
 * 外加行为保持点锁（轮询/乐观回滚/存储同步/静态数据/筛选抽屉）。
 *
 * 本票域 api 收紧口径（同 profileContract）：仅「有 DTO 映射」的函数经 *Mapped 出口家族；
 * 返回 void 的透传函数（viewFeaturedContentApi / markNotificationReadApi / markAllReadApi）不硬套 identity map。
 */
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

/** 提取 export function 函数体（从声明到顶层 "\n}"，先例 quickLoginContract） */
function fnBodyOf(fileSrc, name) {
  const start = fileSrc.indexOf(`export function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  const end = fileSrc.indexOf('\n}', start);
  return fileSrc.slice(start, end);
}

/**
 * 抹注释（`://` 例外，防误杀 URL 字面量）。先例 coursesContract。
 * #1349 起 credential 段的**反向锁**一律走它：那些注释里引用着被删掉的旧代码
 * （`getMockCredentialList()` / `Promise.resolve(...)`），拿原文判「不得出现」会自己绊自己。
 */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
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
  // 注释里的 getMock* 是**解释性文字**（说明为什么删），不是回退 —— 判据一律先抹注释
  const code = stripComments(src);
  it('引入 getMapped 与 requestMapped（PATCH 无便捷面，走 opts 同通路）', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*requestMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getCurrentCredentialApi 经 getMapped 且保留 catch（GET 面「读不到」不当业务失败），但不再查硬编码字典（#1349）', () => {
    const body = fnBodyOf(src, 'getCurrentCredentialApi');
    const bare = stripComments(body);
    expect(body).toContain('getMapped<CredentialItem | null>');
    expect(body).toContain(".catch((e) : CredentialItem | null =>");
    // #1349：离线回退上移到使用方（读「后端确认值」缓存）；本层折进回退会让调用方
    // 把缓存项误当后端回显、拿空 code 去写 selected_cert
    expect(bare).not.toContain('getMockCredentialList');
    expect(bare).not.toContain("uni.getStorageSync('selected_cert')");
  });
  it('getAllCredentialsApi 走后端 grouped、失败上抛（#1349 反转 #643 的 mock 列表兜底）', () => {
    const body = fnBodyOf(src, 'getAllCredentialsApi');
    const bare = stripComments(body);
    expect(body).toContain('getMapped<CredentialGroup[]>');
    expect(body).toContain('/credentials/grouped');
    expect(body).toContain('buildCredentialGroups');
    // 兜底的两种形态都不许在：catch 回落、以及 Promise.resolve(mock) 的同步假列表
    expect(bare).not.toContain('.catch(');
    expect(bare).not.toContain('Promise.resolve(');
    expect(bare).not.toContain('getMockCredentialList');
  });
  it('硬编码证件字典整删（含定义处）—— 锁的是「mock 绝迹」，不是「catch 归零」（与 coursesContract 的差别写在里面）', () => {
    expect(code).not.toContain('getMockCredentialList');
    expect(code).not.toContain('groupCredentials');
    // 与 coursesContract.test.js:239-243 的口径差别写明，免得后人「顺手对齐」：
    // #1331 那处把 .catch 一并清零；本票按裁定**保留** getCurrentCredentialApi 的 .catch
    // （GET 读面的「读不到」不当业务失败上抛），所以这里的数字锁是 1 而不是 0。
    expect((code.match(/\.catch\(/g) || []).length).toBe(1);
  });
  it('缓存读写出口在位（#1349 离线回退的真源，两键都写字符串）', () => {
    expect(src).toContain('export function readCurrentCredentialCache');
    expect(src).toContain('export function writeCurrentCredentialCache');
    expect(code).toContain("uni.setStorageSync('selected_cert_id'");
    expect(code).toContain("uni.setStorageSync('selected_cert_name'");
  });
  it('credential.level 走 toNumberOrNull（后端该字段可为 null，裸 as number 抛 Kotlin NPE）', () => {
    expect(src).toContain("level: toNumberOrNull(obj['level'])");
    expect(src).not.toMatch(/obj\['level'\]\s*as\s+number/);
    expect(src).toMatch(/import\s*\{[^}]*toNumberOrNull[^}]*\}\s*from\s*'\.\/helpers'/);
  });
  it('switchCredentialApi 经 requestMapped，PATCH 经 opts.method 赋值；失败上抛、不再 mock 假成功（#1346 反转 #643 的「后端未实现」兜底）', () => {
    const body = fnBodyOf(src, 'switchCredentialApi');
    expect(body).toContain('requestMapped<CredentialSwitchResult>');
    expect(body).toContain("opts.method = 'PATCH'");
    expect(body).toContain('opts.data = body');
    // 后端 PATCH /me/credential 已实现 ⇒ 失败必须上抛，不得用 .catch + mock 兜成 success:true
    expect(body).not.toContain('.catch(');
    expect(body).not.toContain('getMockCredentialList');
  });
});

/* ══ 幻影路由锁（#662 口径，先例 coursesContract / resumeContract）══ */
describe('幻影路由锁（#662）：credential 域路由必须落在后端已注册清单内（#1349 新建）', () => {
  /** 后端注册面：training_catalog.go 学员端段直接挂 `rg.`（无子组前缀） */
  function registeredCredentialRoutes() {
    const go = stripComments(read('../../backend/internal/api/training_catalog.go'));
    return [...go.matchAll(/\brg\.(GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/g)]
      .map((m) => `${m[1]} ${m[2]}`);
  }

  /** 前端请求点：method 由出口形态定（getMapped→GET / opts.method 赋值→该值） */
  function frontendCredentialRoutes() {
    const code = stripComments(read('api/credential.uts'));
    const out = [];
    for (const m of code.matchAll(/getMapped<[^(]*\(\s*TRAINING_API_BASE \+ '([^']+)'/g)) {
      out.push(`GET ${m[1]}`);
    }
    const opts = /opts\.method = '([A-Z]+)'/.exec(code);
    const patchPath = /opts : RequestOptions = \{ url: TRAINING_API_BASE \+ '([^']+)' \}/.exec(code);
    if (opts != null && patchPath != null) out.push(`${opts[1]} ${patchPath[1]}`);
    return out;
  }

  it('credential.uts 的请求点非空（防判据空跑），且每条都被后端注册面覆盖', () => {
    const used = [...new Set(frontendCredentialRoutes())];
    expect(used.length).toBe(3);
    expect(used).toContain('GET /credentials/grouped');
    const registered = registeredCredentialRoutes();
    expect(registered).toContain('GET /credentials/grouped');
    expect(used.filter((u) => !registered.includes(u))).toEqual([]);
  });

  it('判据具备红能力：注入一条未注册路由必须被列出', () => {
    const registered = registeredCredentialRoutes();
    expect(registered.filter((r) => r.includes('credentials/grouped'))).toHaveLength(1);
    expect(['GET /credentials/grouped-x'].filter((u) => !registered.includes(u)))
      .toEqual(['GET /credentials/grouped-x']);
  });
});

describe('raw .then 收紧完成度（本票域 DTO 函数零残留，student 已由 T03 达标一并复检）', () => {
  const targets = ['api/featured.uts', 'api/notification.uts', 'api/credential.uts', 'api/student.uts'];
  it.each(targets)('%s 不再出现 get/post/patch(...).then((data : UTSJSONObject) : DTO 形态', (rel) => {
    const src = read(rel);
    expect(src).not.toMatch(/return (get|post|patch)\([^)]*\)\s*\.then\(\(data : UTSJSONObject\) : (?!UTSJSONObject)/);
  });
});

/* ══ ② 页面预算落袋锁（模块全量预算 / 目录深度已由声明面执法，#1219） ══ */
describe('页面预算落袋锁（手术目标本身也断言，防「只挪注释」的假达标）', () => {
  it('走查范围非空（防路径断链导致空集合假绿：页面 + 5 组件 + 2 composable = 8）', () => {
    expect(h.sourceFilesIn('pages/dashboard').length).toBe(8);
  });

  it('主页面预算落袋锁（手术前 1423 行）', () => {
    const lines = read('pages/dashboard/dashboard.uvue').split('\n').length;
    expect(lines).toBeLessThanOrEqual(600);
    expect(lines).toBeLessThan(1000);
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
    const orphanOf = (dir, prefix) => h.sourceFilesIn(`pages/dashboard/${dir}`)
      .map((rel) => rel.split('/').pop())
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
    for (const rel of h.sourceFilesIn('pages/dashboard')) {
      const src = read(rel);
      if (/from\s*'[^']*api\/request(\.uts)?'/.test(src)) hits.push(`${rel}: import api/request`);
      if (/uni\.request\s*\(/.test(src)) hits.push(`${rel}: uni.request 裸调`);
      if (/uni\.(upload|download)File\s*\(/.test(src)) hits.push(`${rel}: uni.uploadFile/downloadFile 裸调`);
    }
    expect(hits).toEqual([]);
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
    // #1346：未确认切换（success:false）不得再当成功写本地存储 ⇒ else 分支不再有 item.code 写入
    expect(body).not.toContain("uni.setStorageSync('selected_cert', item.code)");
  });

  it("selected_cert 存储同步点（#1349 改写：写 2 保持，读 2→0 —— 回退改读「后端确认值」缓存）", () => {
    // 沿革：#1346 把写从 3 调到 2（未确认切换不写）；#1349 把**读**清零 —— 原两处读拿 code
    // 去查硬编码字典（后端字典一漂移就供出后端没有的 id），现由 selected_cert_id / _name 两键
    // 承接。code 键本身仍有写（choose-cert 页面在消费它），其退役在 #1349 跟进票里议。
    const syncWrites = (cred.match(/uni\.setStorageSync\('selected_cert'/g) || []).length;
    expect(syncWrites).toBe(2);
    const syncReads = (cred.match(/uni\.getStorageSync\('selected_cert'/g) || []).length;
    expect(syncReads).toBe(0);
    // 缓存出口的接线：1 读（回退）+ 3 写（后端确认 ×2、确认无证件清空 ×1）
    expect((cred.match(/readCurrentCredentialCache\(\)/g) || []).length).toBe(1);
    expect((cred.match(/writeCurrentCredentialCache\(/g) || []).length).toBe(3);
    // 清空必须显式传 null（供旧值冒充当前证件是 #1349 要堵的第二处漂移）
    expect(cred).toContain('writeCurrentCredentialCache(null)');
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
    const all = h.sourceFilesIn('pages/dashboard').map(read).join('\n');
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
