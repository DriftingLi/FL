/**
 * mall 试点手术契约测试（TDD: RED → GREEN，refs #640 / refactor epic #638 T02）
 *
 * 钉住试点交付的两类契约（1) 2)；编号是类别不是 describe 数，3) 4) 是「执法点在别处」的口径登记），
 * 采用项目既有源码契约缝（.uvue/.uts 不可被 jest import）：
 * 1) api 收紧：getCourseListApi 经 requestMapped mapper-callback 出口（#639 出口首个真实消费者）
 * 2) 组件接线：mall.uvue 以显式 import 使用模块私有组件（Q17 安置规则）
 * 3) 600 软预算 / 目录 ≤2 层：**已由声明面执法**（`utils/modules.js` +
 *    `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件不再各写一遍（ADR-0023 票 C #1219）
 * 4) mall 域文件机械坑位（catch : any / .detail 直取，即试点验收的「清零」项）：本文件不再写这类锁
 *    ——#654 起豁免面已删，执法点只有全工程守护 `utils/utsAndroidCompile.test.js` 规则 H/I 一处
 *
 * #2618 T2 起本文件同时守「四态货架骨架」：四态结构、按类型取源、课程卡进详情、余额卡常驻、
 * 悬浮收敛为单搜索钮、热门固定条数混排、页内切换不跳转（票面验收 ①-⑦ 的机器可判部分）。
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
  const COMPONENTS = ['mall-sort-bar', 'mall-category-sidebar', 'mall-float-actions', 'mall-course-card', 'mall-hot-section', 'mall-shop-card'];

  it.each(COMPONENTS)('组件文件存在于 pages/mall/components/%s.uvue', (name) => {
    expect(h.exists(`pages/mall/components/${name}.uvue`)).toBe(true);
  });

  it.each(COMPONENTS)('mall.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用六个组件标签（非只 import 不用；PascalCase 先例同 AiChatNav）', () => {
    expect(page).toMatch(/<MallSortBar[\s/>]/);
    expect(page).toMatch(/<MallCategorySidebar[\s/>]/);
    expect(page).toMatch(/<MallFloatActions[\s/>]/);
    expect(page).toMatch(/<MallCourseCard[\s/>]/);
    expect(page).toMatch(/<MallHotSection[\s/>]/);
    expect(page).toMatch(/<MallShopCard[\s/>]/);
  });
});

describe('#2518 第 4 刀（商城逐像素收口 824:6971）：默认书封 · 搜索切图 · 左栏四态', () => {
  const page = read('pages/mall/mall.uvue');
  const card = read('pages/mall/components/mall-course-card.uvue');

  it('默认书封：cover_url 兜底走包内路径拼接（表达式含 /static/ 才过 imageWebpAttrContract 豁免），五种书封文件在位', () => {
    expect(card).toMatch(/:src="'\/static\/icons\/mall\/cover-' \+ defaultCoverName\(title\) \+ '\.png'"/);
    for (const f of ['cover-ebook', 'cover-material-pro', 'cover-material-aux', 'cover-material-skill', 'cover-textbook']) {
      expect(h.exists(`static/icons/mall/${f}.png`)).toBe(true);
    }
    // 商品卡纯 CSS 灰块兜底退役（热门区/积分商品卡的占位不在本锁射程，其类名不叫 product-img-placeholder）
    expect(page).not.toContain('product-img-placeholder');
    expect(card).not.toContain('product-img-placeholder');
  });

  it('搜索位吃切图（排序栏 + 悬浮搜索共用一个 32×32 资产），🔍 emoji 兜底退场', () => {
    expect(h.exists('static/icons/mall/search.png')).toBe(true);
    expect(read('pages/mall/components/mall-sort-bar.uvue')).toContain('/static/icons/mall/search.png');
    expect(read('pages/mall/components/mall-float-actions.uvue')).toContain('/static/icons/mall/search.png');
    expect(read('pages/mall/components/mall-sort-bar.uvue')).not.toContain('🔍');
    expect(read('pages/mall/components/mall-float-actions.uvue')).not.toContain('🔍');
  });

  it('左栏四态齐备（热门/课程/图书系列/积分兑换），VIP 套餐位退役', () => {
    const side = read('pages/mall/components/mall-category-sidebar.uvue');
    expect(side).toContain('热门');
    expect(side).toContain('课程');
    expect(side).toContain('图书系列');
    expect(side).toContain('积分兑换');
    expect(side).not.toContain('VIP套餐');
    expect(side).not.toContain('isVip');
  });
});

describe('#2618 T2 四态货架骨架（合并为单一货架页，ADR-0072）', () => {
  const page = read('pages/mall/mall.uvue');
  const state = read('pages/mall/composables/useMall.uts');

  it('① 四态左栏页内切换：不跳积分商城、不重发请求；四态顺序与分区号同口径', () => {
    const side = read('pages/mall/components/mall-category-sidebar.uvue');
    // 顺序即分区号：0 热门 / 1 课程 / 2 图书系列 / 3 积分兑换
    expect(side).toMatch(/\{ name: '热门', id: 0 \}[\s\S]*?\{ name: '课程', id: 1 \}[\s\S]*?\{ name: '图书系列', id: 2 \}[\s\S]*?\{ name: '积分兑换', id: 3 \}/);
    // Q8 反转：本页不再有跳积分商城的分支（旧 navigateTo + if (idx == 2) 早返形态退役）
    expect(page).not.toContain("uni.navigateTo({ url: '/pages/points/points-shop' })");
    expect(page).not.toMatch(/if \(idx == 2\)/);
    expect(page).toMatch(/function onCategoryChange\(idx : number\) : void \{[\s\S]{0,200}?setCategory\(idx\)/);
    // 切换分区只改 activeCategory，不触发装载
    const start = state.indexOf('function setCategory');
    expect(start).toBeGreaterThan(-1);
    const body = state.slice(start, state.indexOf('\n    }', start));
    expect(body).toContain('activeCategory.value = idx');
    expect(body).not.toContain('loadCourses(');
    expect(body).not.toContain('loadItems(');
  });

  it('② tab 深链：onLoad 读 options[\'tab\']，tab=points 落积分兑换、缺省/未知落热门', () => {
    expect(page).toMatch(/onLoad\(\(options : OnLoadOptions\)/);
    expect(page).toContain("options['tab']");
    expect(page).toContain('startMall(');
    expect(state).toMatch(/activeCategory\.value = tab == 'points' \? 3 : 0/);
    // 首屏四路一次装载（课程 + 余额 + 积分商品 + 图书系列），分区之间不再各自打请求
    const start = state.indexOf('function startMall');
    const body = state.slice(start, state.indexOf('\n    }', start));
    for (const call of ['loadBalance()', 'loadItems()', 'loadBookItems()', 'startCourses()']) {
      expect(body).toContain(call);
    }
  });

  it('③ 热门 = 课程前 6 + 积分商品前 6，两段各带「查看全部」；固定条数、不做无限加载', () => {
    const hot = read('pages/mall/components/mall-hot-section.uvue');
    expect(hot).toContain('热门课程');
    expect(hot).toContain('热门积分商品');
    expect(hot).toContain('查看全部');
    expect(hot).toContain("emit('moreCourses')");
    expect(hot).toContain("emit('morePoints')");
    // 切片口径：两路各前 6（HOT_SIZE 单点）
    expect(state).toContain('courses.value.slice(0, 6)');
    expect(state).toContain('res.items.slice(0, 6)');
    // 加载更多只对课程分区开放 ⇒ 热门没有第二页
    const start = state.indexOf('function loadMoreCourses');
    const body = state.slice(start, state.indexOf('\n    }', start));
    expect(body).toContain('if (activeCategory.value != 1) return');
    // 两段「查看全部」都走页内切换（不跳转）
    expect(page).toMatch(/function onShowCourses\(\) : void \{[\s\S]{0,120}?setCategory\(1\)/);
    expect(page).toMatch(/function onShowPoints\(\) : void \{[\s\S]{0,120}?setCategory\(3\)/);
  });

  it('④ 课程分区列全部课程；整卡进课程详情、卡右下「查看详情」；「必买清单」退役', () => {
    expect(page).toMatch(/v-else-if="activeCategory === 1"/);
    expect(page).toMatch(/loadCourses\(true\)/);
    expect(page).toContain('商品加载失败，请检查网络后重试');
    expect(page).toContain('暂无商品');
    // 整卡点击 → 课程详情（列表里点不扣分，兑换确认在详情页）
    expect(page).toMatch(/function onProductClick\(item : CourseItem\) : void \{[\s\S]{0,140}?\/pages\/courses\/course-detail\?id=/);
    const card = read('pages/mall/components/mall-course-card.uvue');
    expect(card).toContain('查看详情');
    // 旧「必买清单」区退役并入热门课程段（ADR-0072 决策 4）：页面与热门区都不得再出现该区名
    expect(page).not.toContain('必买清单');
    expect(read('pages/mall/components/mall-hot-section.uvue')).not.toContain('必买清单');
  });

  it('⑤ 余额卡常驻页头下方（四态可见）；悬浮只余单搜索钮', () => {
    // 常驻 = 组件标签上不挂 v-if（用「标签内无 v-if」判，避免误伤同页其他 v-if）
    const m = page.match(/<PointsBalanceCard[^>]*>/);
    expect(m).not.toBeNull();
    expect(m[0]).not.toContain('v-if');
    const float = read('pages/mall/components/mall-float-actions.uvue');
    expect(float).toContain('搜索');
    expect(float).not.toContain('分享功能开发中');
    expect(float).not.toContain('showFloatMenu');
    expect(float).not.toMatch(/\/pages\/index\/index/);
  });

  it('⑥ 两源按类型取源：课程走课程列表动词、图书系列走 category=book、积分兑换吃 points_price 非空', () => {
    expect(state).toContain('getShopItemsFilteredApi(\'book\', 0, 0)');
    const start = state.indexOf('function redeemableList');
    expect(start).toBeGreaterThan(-1);
    expect(state.slice(start, state.indexOf('\n    }', start))).toContain('it.points_price != null');
    // 只读商品卡：T2 不带兑换按钮（三态是 T3 的面）
    const shopCard = read('pages/mall/components/mall-shop-card.uvue');
    expect(shopCard).not.toContain("emit('redeem')");
    expect(shopCard).not.toContain('redeemShopItemApi');
  });
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

  it('课程路装载下沉到 composable，且打开商城不多打积分商品请求（四路都靠 startMall 的显式调用）', () => {
    expect(state).toMatch(/export function useMall\(\)/);
    expect(state).toContain('getCourseListApi(page.value, pageSize, 0, 0, browseCredentialId.value)');
    expect(page).toContain('startMall(');
    expect(page).not.toMatch(/getShopItemsApi|getPointsBalanceApi/);
  });

  it('四态结构仍在页面（拆组件不改显示时机）', () => {
    expect(page).toMatch(/v-if="activeCategory === 0"/);
    expect(page).toMatch(/v-else-if="activeCategory === 2"/);
  });
});
