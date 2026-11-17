/**
 * mall 试点手术契约测试（TDD: RED → GREEN，refs #640 / refactor epic #638 T02）
 *
 * 钉住试点交付的两类契约（1) 2)；编号是类别不是 describe 数，3) 4) 是「执法点在别处」的口径登记），
 * 采用项目既有源码契约缝（.uvue/.uts 不可被 jest import）：
 * 1) api 收紧：getCourseListApi 经 requestMapped mapper-callback 出口（#639 出口首个真实消费者）
 * 2) 组件接线：mall.uvue 以显式 import 使用五个模块私有组件（Q17 安置规则）
 * 3) 600 软预算 / 目录 ≤2 层：**已由声明面执法**（`utils/modules.js` +
 *    `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件不再各写一遍（ADR-0023 票 C #1219）
 * 4) mall 域文件机械坑位（catch : any / .detail 直取，即试点验收的「清零」项）：本文件不再写这类锁
 *    ——#654 起豁免面已删，执法点只有全工程守护 `utils/utsAndroidCompile.test.js` 规则 H/I 一处
 */
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

describe('api 收紧契约（getCourseListApi 经 requestMapped 出口家族）', () => {
  const src = read('api/course.uts');
  const req = read('api/request.uts');

  it('request.uts 提供 GET 便捷面 getMapped：委派 requestMapped，query 序列化收在传输层 buildQuery（不交平台自动序列化，防跨端漂移）', () => {
    const start = req.indexOf('export function getMapped');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n}', start));
    expect(body).toContain('requestMapped<T>(');
    // #1734 问题 3：序列化本体下沉 api/transport.uts（buildQuery），getMapped 只经 withQuery 组 URL。
    // 判据仍锁「不交平台自动序列化」这层意图：getMapped 必须在**自己**拼 query，
    // 只是拼接的实现在传输层 —— 故一侧钉委派、另一侧钉 buildQuery 逐字含 encodeURIComponent。
    expect(body).toContain('withQuery(');
    const transport = read('api/transport.uts');
    const bs = transport.indexOf('export function buildQuery');
    expect(bs).toBeGreaterThan(-1);
    const buildQueryBody = transport.slice(bs, transport.indexOf('\n}', bs));
    expect(buildQueryBody).toContain('encodeURIComponent');
    expect(buildQueryBody).toContain('pairs.join');
  });

  it('course.uts 引入 getMapped 出口', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('getCourseListApi 函数体经 getMapped，映射走信封归一出口 decodePage（ADR-0037 PR-B）', () => {
    const start = src.indexOf('export function getCourseListApi');
    expect(start).toBeGreaterThan(-1);
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('getMapped<Page<CourseItem>>');
    expect(body).toMatch(/decodePage<CourseItem>\(data/);
    expect(body).toContain('COURSE_LIST_KEYS');
    // #1331：mock 降级按 ADR-0007「掩盖面」判据整体退役 —— 原「UI 冻结：无网络时仍出 mock 数据」反转：
    // 失败一律上抛，由页面呈现可见错误态 + 重试（courses / mall 的 loadError 三态）。锁随之反转为「不得复活」。
    expect(body).not.toContain('getMockCourseList()');
  });
});

describe('模块私有组件接线契约（Q17 安置：pages/<module>/components/ 显式 import）', () => {
  const page = read('pages/mall/mall.uvue');
  const COMPONENTS = ['mall-sort-bar', 'mall-category-sidebar', 'mall-float-actions', 'mall-course-card', 'mall-hot-section'];

  it.each(COMPONENTS)('组件文件存在于 pages/mall/components/%s.uvue', (name) => {
    expect(h.exists(`pages/mall/components/${name}.uvue`)).toBe(true);
  });

  it.each(COMPONENTS)('mall.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用五个组件标签（非只 import 不用；PascalCase 先例同 AiChatNav）', () => {
    expect(page).toMatch(/<MallSortBar[\s/>]/);
    expect(page).toMatch(/<MallCategorySidebar[\s/>]/);
    expect(page).toMatch(/<MallFloatActions[\s/>]/);
    expect(page).toMatch(/<MallCourseCard[\s/>]/);
    expect(page).toMatch(/<MallHotSection[\s/>]/);
  });
});

describe('#2518 第 4 刀（商城逐像素收口 824:6971）：默认书封 · 搜索切图 · 积分兑换反向边', () => {
  const page = read('pages/mall/mall.uvue');
  const card = read('pages/mall/components/mall-course-card.uvue');

  it('默认书封：cover_url 兜底走包内路径拼接（表达式含 /static/ 才过 imageWebpAttrContract 豁免），五种书封文件在位', () => {
    expect(card).toMatch(/:src="'\/static\/icons\/mall\/cover-' \+ defaultCoverName\(item\) \+ '\.png'"/);
    for (const f of ['cover-ebook', 'cover-material-pro', 'cover-material-aux', 'cover-material-skill', 'cover-textbook']) {
      expect(h.exists(`static/icons/mall/${f}.png`)).toBe(true);
    }
    // 商品卡纯 CSS 灰块兜底退役（必买清单区不在本刀射程，其占位保留）
    expect(page).not.toContain('product-img-placeholder');
    expect(card).not.toContain('product-img-placeholder');
  });

  it('搜索位吃切图（排序栏 + 悬浮菜单共用一个 32×32 资产），🔍 emoji 兜底退场', () => {
    expect(h.exists('static/icons/mall/search.png')).toBe(true);
    expect(read('pages/mall/components/mall-sort-bar.uvue')).toContain('/static/icons/mall/search.png');
    expect(read('pages/mall/components/mall-float-actions.uvue')).toContain('/static/icons/mall/search.png');
    expect(read('pages/mall/components/mall-sort-bar.uvue')).not.toContain('🔍');
    expect(read('pages/mall/components/mall-float-actions.uvue')).not.toContain('🔍');
  });

  it('左栏「积分兑换」→ 积分商城（824:7014 → 821:6396 边；Q8 两路由不合并，互跳即两路由互跳）；VIP 套餐位退役', () => {
    const side = read('pages/mall/components/mall-category-sidebar.uvue');
    expect(side).toContain('积分兑换');
    expect(side).not.toContain('VIP套餐');
    expect(side).not.toContain('isVip');
    expect(page).toContain("uni.navigateTo({ url: '/pages/points/points-shop' })");
    // 选中「积分兑换」不是本页态：不落 activeCategory（分支内 return）
    expect(page).toMatch(/if \(idx == 2\) \{\s*\n\s*onGoPointsShop\(\)\s*\n\s*return/);
  });

describe('#2617 T1 页面状态下沉与组件拆分（预重构，行为不变）', () => {
  const page = read('pages/mall/mall.uvue');
  const state = read('pages/mall/composables/useMall.uts');

  it('页面壳层零自持状态：ref / reactive / computed 全在 useMall（口径同 pointsExchangeRecordsContract）', () => {
    expect(page).not.toMatch(/import\s*\{[^}]*\bref\b[^}]*\}\s*from\s*'vue'/);
    expect(page).not.toMatch(/\breactive\(/);
    expect(page).not.toMatch(/\bcomputed\(/);
    expect(page).toContain('useMall()');
  });

  it('课程路装载下沉到 composable，且打开商城不多打积分商品请求（两路都靠显式调用）', () => {
    expect(state).toMatch(/export function useMall\(\)/);
    expect(state).toContain('getCourseListApi(page.value, pageSize, 0, 0, browseCredentialId.value)');
    expect(page).toContain('startCourses()');
    expect(page).not.toMatch(/getShopItemsApi|getPointsBalanceApi/);
  });

  it('热门区显示条件与三态文案仍在页面（拆组件不改显示时机）', () => {
    expect(page).toMatch(/v-if="activeCategory === 0"/);
    expect(page).toContain('商品加载失败，请检查网络后重试');
    expect(page).toContain('暂无商品');
  });
});
});
