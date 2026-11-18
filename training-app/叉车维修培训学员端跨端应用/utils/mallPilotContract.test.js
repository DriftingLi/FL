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
 * #2620 T4 起再守「入口收敛 + 文案统一 + 词表」：四处入口同一深链 tab=points、路由表标题 / 页头 /
 * 首页宫格为「商城」、「课程商城」旧名与一次性跨页意图两半清零。
 *
 * #2621 T5 是整件合并的收口：旧「积分商城」路由与旧页下线（无兼容重定向）、页私有组件随页退役，
 * 原 `utils/pointsShopContract.test.js` 的有效断言（列表接口 / 筛选骨架 / 页内兑换管线 / 后端对账表）
 * 迁到本文件末尾——被删页的套件不再单留一份，也不新建第二个测试文件。
 */
const path = require('path');
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;
/** pages.json 路由拍平（#2579 起非一级页整目录切进分包，分包 path 是相对 root 的短名）——不硬写路由表字面量 */
const { registeredRoutes } = require('../scripts/lib/pages-routes.js');

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
  // #2619 T3：排序栏整体退役（四档无后端支撑），筛选收成「积分范围」单控件 ⇒ 清单随之换人
  // #2621 T5：余额卡随旧页下线迁入本模块（它只剩合并页一个消费者）⇒ 清单补一件
  const COMPONENTS = ['mall-balance-card', 'mall-range-filter', 'mall-category-sidebar', 'mall-float-actions', 'mall-course-card', 'mall-hot-section', 'mall-shop-card'];

  it.each(COMPONENTS)('组件文件存在于 pages/mall/components/%s.uvue', (name) => {
    expect(h.exists(`pages/mall/components/${name}.uvue`)).toBe(true);
  });

  it.each(COMPONENTS)('mall.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用七个组件标签（非只 import 不用；PascalCase 先例同 AiChatNav）', () => {
    expect(page).toMatch(/<MallBalanceCard[\s/>]/);
    expect(page).toMatch(/<MallRangeFilter[\s/>]/);
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

  it('搜索位吃切图（搜索钮共用一个 32×32 资产），🔍 emoji 兜底退场；排序栏已随 T3 退役', () => {
    expect(h.exists('static/icons/mall/search.png')).toBe(true);
    expect(read('pages/mall/components/mall-float-actions.uvue')).toContain('/static/icons/mall/search.png');
    expect(read('pages/mall/components/mall-float-actions.uvue')).not.toContain('🔍');
    expect(h.exists('pages/mall/components/mall-sort-bar.uvue')).toBe(false);
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
    // #2621 T5：旧页下线后余额卡只剩本页一个消费者 ⇒ 迁入 mall 的组件面（mall-balance-card）
    const m = page.match(/<MallBalanceCard[^>]*>/);
    expect(m).not.toBeNull();
    expect(m[0]).not.toContain('v-if');
    const float = read('pages/mall/components/mall-float-actions.uvue');
    expect(float).toContain('搜索');
    expect(float).not.toContain('分享功能开发中');
    expect(float).not.toContain('showFloatMenu');
    expect(float).not.toMatch(/\/pages\/index\/index/);
  });

  it('⑥ 两源按类型取源：课程走课程列表动词、图书系列走 category=book、积分兑换吃 points_price 非空', () => {
    // #2619 T3 起「积分范围」两档参数进图书系列取源（图书系列也显筛选控件）
    expect(state).toContain("getShopItemsFilteredApi('book', range.min, range.max)");
    const start = state.indexOf('function redeemableList');
    expect(start).toBeGreaterThan(-1);
    expect(state.slice(start, state.indexOf('\n    }', start))).toContain('it.points_price != null');
    // 兑换调用只在状态层（卡只回抛 redeem 事件，卡内不直连 api）
    const shopCard = read('pages/mall/components/mall-shop-card.uvue');
    expect(shopCard).not.toContain('redeemShopItemApi');
    expect(shopCard).not.toContain('getPointsBalanceApi');
  });
});

describe('#2619 T3 积分商品分区 + 兑换三态 + 单筛选控件 + 排序退役', () => {
  const page = read('pages/mall/mall.uvue');
  const state = read('pages/mall/composables/useMall.uts');
  const card = read('pages/mall/components/mall-shop-card.uvue');
  const filter = read('pages/mall/components/mall-range-filter.uvue');
  const tpl = page.slice(page.indexOf('<template>'), page.lastIndexOf('</template>'));
  /** 标签类判据只在模板里找：注释（含「XX 已退役」说明）不算标签（本仓既有口径同此，见 coursesContract stripAll） */
  const tplOf = (src) => src
    .slice(src.indexOf('<template>'), src.lastIndexOf('</template>'))
    .replace(/<!--[\s\S]*?-->/g, '');

  it('① 积分兑换分区 = points_price 非空课程段 + 积分商品全量；图书系列只看 category=book', () => {
    // 分区 3：课程段（points_price 非空）+ 商品段（全量 items）同屏，两个小标题都在
    const part3 = tpl.slice(tpl.indexOf('v-else class="partition"'));
    expect(part3).toContain('积分课程');
    expect(part3).toContain('积分商品');
    expect(part3).toContain('<MallCourseCard');
    expect(part3).toContain('<MallShopCard');
    // 图书系列仍只看 category=book（服务端参数，DTO 无 category 字段 ⇒ 客户端切不出来）
    expect(state).toContain("getShopItemsFilteredApi('book', range.min, range.max)");
    // 积分兑换的商品段是「全量」：loadItems 不带 category 参数
    expect(state).toContain("getShopItemsFilteredApi('', range.min, range.max)");
  });

  it('② 积分商品卡页内兑换：三态（已拥有 / 已售罄 / 兑换中）+ 确认弹窗 + 余额不足明说 + 不预扣', () => {
    // 卡只呈现（按钮文案与配色由状态层算好流入），回抛 redeem 事件
    expect(card).toContain('btnLabel');
    expect(card).toContain(':class="[\'shop-btn\', btnClass]"');
    expect(card).toContain(':class="[\'shop-btn-text\', btnTextClass]"');
    expect(card).toContain("emit('redeem')");
    // 三态单点：btnText 的判定顺序 = 已拥有 → 已售罄 → 兑换中 → 兑换
    expect(state).toContain("if (item.owned) return '已拥有'");
    expect(state).toContain("if (s != null && s == 0) return '已售罄'");
    expect(state).toContain("if (redeemingSku.value == item.sku) return '兑换中'");
    // 确认弹窗 + 余额不足明说，仍由服务端裁定、不预扣
    expect(state).toContain('uni.showModal');
    expect(state).toContain('余额不足，继续兑换会被服务端拒绝。');
    expect(state).toMatch(/redeemShopItemApi\(item\.sku\)/);
    expect(state).not.toMatch(/pointsBalance\.value = pointsBalance\.value -/);
    // 两个分区都把三态接上（按钮文案/配色/事件三件齐）
    const bindings = tpl.match(/<MallShopCard[\s\S]*?\/>/g) || [];
    expect(bindings.length).toBeGreaterThan(1);
    for (const b of bindings) {
      expect(b).toContain(':btn-label="btnText(');
      expect(b).toContain(':btn-class="btnClass(');
      expect(b).toContain(':btn-text-class="btnTextClass(');
      expect(b).toContain('@redeem="onRedeem(');
    }
    // 按钮命中区同样撑到 88rpx（视觉胶囊 60rpx），@click 挂命中区
    expect(card).toMatch(/\.shop-btn-hit \{[^}]*height:\s*88rpx/);
    expect(card).toContain('@click="onRedeem"');
  });

  it('③a 两路积分商品失败不伪装成空货架：可见错误 + 重试位（#1331 口径）', () => {
    // 档位变化会重取，重取也会失败 ⇒ 两个积分商品分区都要有错误分支，不能只留「暂无商品」
    expect(tpl).toContain('商品加载失败，请检查网络后重试');
    // 两路各有独立的失败标记：图书系列失败不能被积分商品路的成败冒充
    expect(tpl).toMatch(/v-if="bookLoadFailed && bookItems\.length == 0 && !bookLoading"/);
    expect(tpl).toMatch(/v-if="loadFailed && items\.length == 0 && !loading"/);
    expect(state).toContain('bookLoadFailed : Ref<boolean>');
    expect(tpl.match(/@click="onRetryItems"/g) || []).toHaveLength(2);
    // 重试位重载两路（只刷一路会让另一分区停在失败态）
    expect(page).toMatch(/function onRetryItems\(\) : void \{[\s\S]{0,120}?loadItems\(\)[\s\S]{0,80}?loadBookItems\(\)/);
  });

  it('③ 筛选栏只剩「积分范围」单控件：三档 + 「不限」复位，只在图书系列 / 积分兑换显示', () => {
    // 组件里只有一套档位（无分类触发器、无「可换商品」快捷入口）
    expect(filter).toContain('积分范围');
    expect(tplOf(filter)).not.toContain('商品分类');
    expect(tplOf(filter)).not.toContain('可换商品');
    // 三档 + 不限：档位表只此一份（rangeOptions），标签逐字，且标签读作闭区间
    //（500 两侧都算：把 500 只算进「≤500」会让「500-1000」这个标签少一档）
    expect(state).toContain("label: '不限'");
    expect(state).toContain("label: '≤500'");
    expect(state).toContain("label: '500-1000'");
    expect(state).toContain("label: '>1000'");
    expect(state).toContain("{ label: '500-1000', value: '500-1000', min: 500, max: 1000 }");
    expect(state).not.toContain('categoryOptions');
    // 触摸目标：视觉胶囊可以更小，命中区按 docs/ui-spec.md §5.11 撑到 88rpx 且 @click 挂命中区
    expect(filter).toMatch(/\.range-pill-hit \{[^}]*height:\s*88rpx/);
    expect(filter).toContain('<view v-for="(opt, i) in rangeOptions" :key="opt.value" class="range-pill-hit" @click="onPick(i)">');
    // 分区可见性：只在 2 / 3 两个含积分商品的分区出现
    const m = tpl.match(/<MallRangeFilter[^>]*v-if="([^"]+)"/);
    expect(m).not.toBeNull();
    expect(m[1]).toContain('activeCategory === 2');
    expect(m[1]).toContain('activeCategory === 3');
    // 切分区高度不跳：控件槽常驻、固定高度（占位不随分区增减）
    expect(tpl).toMatch(/<view class="filter-slot">[\s\S]*?<MallRangeFilter/);
    const slotStyle = page.slice(page.indexOf('.filter-slot {'));
    expect(slotStyle.slice(0, slotStyle.indexOf('}'))).toMatch(/height:\s*88rpx/);
  });

  it('④ 退役清零：商品分类 / 可换商品 / 课程兑换导流卡 / 四档排序栏 标签与实现不存；搜索仍在悬浮钮', () => {
    expect(h.exists('pages/mall/components/mall-sort-bar.uvue')).toBe(false);
    expect(h.exists('pages/points/components/points-filter-bar.uvue')).toBe(false);
    expect(page).not.toContain('MallSortBar');
    // 退役标签在商城页 / 筛选栏的模板里都不许复活（头注里的退役说明不算标签）。
    // #2621 T5：旧积分商城页已删，它的「标签不许复活」由下文 T5 ② 的全工程源文件面扫兜住
    for (const src of [page, filter]) {
      const t = tplOf(src);
      expect(t).not.toContain('商品分类');
      expect(t).not.toContain('可换商品');
      expect(t).not.toContain('用积分解锁付费课');
    }
    // 实现面同日清零（分类档位表 / 面板态 / 排序态都不在状态层）
    expect(state).not.toContain('categoryOptions');
    expect(state).not.toContain('filterPanel');
    expect(state).not.toContain('activeSort');
    expect(state).not.toContain('setSort');
    // 一次性跨页意图（导流卡唯一的消费者）两半都退役
    const nav = read('utils/navigation.uts');
    expect(nav).not.toContain('CourseRedeemIntent');
    expect(read('pages/courses/courses.uvue')).not.toContain('CourseRedeemIntent');
    // 搜索能力不丢：仍在右下角悬浮钮（排序栏右端那一位已随排序栏退役）
    expect(read('pages/mall/components/mall-float-actions.uvue')).toContain("emit('search')");
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

describe('#2620 T4 入口收敛到 tab=points + 词表文案统一（#2518 Q8 反转的收尾一刀）', () => {
  const page = read('pages/mall/mall.uvue');
  const state = read('pages/mall/composables/useMall.uts');
  const task = read('pages/points/task-center.uvue');
  const detail = read('pages/points/points-detail.uvue');
  const records = read('pages/points/points-exchange-records.uvue');
  const grid = read('pages/dashboard/components/dashboard-menu-grid.uvue');
  /** 合并页「积分兑换」分区的深链（ADR-0072 决策 3：初始分区由 query 决定） */
  const MERGED = '/pages/mall/mall?tab=points';
  /** 模板切片（剥注释：头注里的退役说明不是学员可见文案） */
  const tplOf = (src) => src
    .slice(src.indexOf('<template>'), src.lastIndexOf('</template>'))
    .replace(/<!--[\s\S]*?-->/g, '');

  it('① 任务中心两条路由定义与「去兑换」都落合并页的积分兑换分区', () => {
    // browse_mall / exchange_gift 两项奖励都发生在货架页 ⇒ 落点带 tab=points，点完就走
    expect(task).toContain("if (code == 'browse_mall' || code == 'exchange_gift') return { url: '/pages/mall/mall?tab=points', tab: false } as TaskRoute");
    const go = task.slice(task.indexOf('function goExchange'));
    expect(go.slice(0, 240)).toContain("uni.navigateTo({ url: '/pages/mall/mall?tab=points' })");
  });

  it('② 积分明细提示条与兑换记录返回入口同落点（看余额顺势去花分、看完记录接着挑）', () => {
    const detailGo = detail.slice(detail.indexOf('function onGoShop'));
    expect(detailGo.slice(0, 200)).toContain("uni.navigateTo({ url: '/pages/mall/mall?tab=points' })");
    const recordsGo = records.slice(records.indexOf('function onGoShop'));
    expect(recordsGo.slice(0, 200)).toContain("uni.navigateTo({ url: '/pages/mall/mall?tab=points' })");
    // 三个文件里出现的每个商城深链都必须是同一份 query（新增入口自动纳入，多带参数即红）
    const urls = [task, detail, records].flatMap((src) => [...src.matchAll(/'(\/pages\/mall\/mall[^']*)'/g)].map((m) => m[1]));
    expect(urls.length).toBeGreaterThan(0);
    for (const u of urls) expect(u).toBe(MERGED);
  });

  it('③ 生产端与消费端同一深链口径：入口带 tab=points，合并页 onLoad 读 options[\'tab\']', () => {
    expect(page).toContain("options['tab']");
    expect(state).toMatch(/tab == 'points' \? 3 : 0/);
  });

  it('④ 文案统一：路由表标题 / 页头 / 首页宫格都是「商城」', () => {
    const conf = JSON.parse(read('pages.json'));
    const mallPkg = (conf.subPackages || []).find((s) => s.root === 'pages/mall');
    expect(mallPkg).toBeTruthy();
    expect(mallPkg.pages[0].style.navigationBarTitleText).toBe('商城');
    expect(page).toContain('title="商城"');
    expect(grid).toContain("title: '商城'");
    expect(grid).toContain("path: '/pages/mall/mall'");
  });

  it('⑤ 「课程商城」旧名清零：学员可见处不再出现该叫法', () => {
    // 无 <template> 的文件（纯脚本件）读原文，不让 tplOf 在缺模板时切片成空串静默过
    const visible = (src) => (src.includes('<template>') ? tplOf(src) : src);
    const offenders = [...h.sourceFilesIn('pages'), ...h.sourceFilesIn('components')]
      .filter((f) => visible(read(f)).includes('课程商城'));
    expect(offenders).toEqual([]);
  });

  it('⑥ 词表口径落进可见文案：「积分商城」只指兑换管线，不再当页面名出现在入口里', () => {
    // 入口文案（提示条 / 空态按钮）写的是落点那个页面的名字：商城
    for (const src of [detail, records]) expect(tplOf(src)).not.toContain('积分商城');
    expect(tplOf(detail)).toContain('商城');
    expect(tplOf(records)).toContain('商城');
  });

  it('⑦ 跨页意图机制两半清零：全工程源文件无 courseRedeemIntent 引用（写入 + 消费都无）', () => {
    // 射程 = 整个工程（`sourceFilesIn('.')` 已跳过 node_modules / unpackage 等构建面，见 contractHarness），
    // 不按目录白名单挑：意图机制的写入方落过 utils、消费方落过页面，漏扫的目录就是下一次复活的藏身处
    const offenders = h.sourceFilesIn('.').filter((f) => /courseredeemintent/i.test(read(f)));
    expect(offenders).toEqual([]);
  });
});

describe('#2621 T5 旧路由与旧页下线 + 契约迁移（整件合并的收口票）', () => {
  const page = read('pages/mall/mall.uvue');
  const state = read('pages/mall/composables/useMall.uts');
  const api = read('api/points.uts');
  const types = read('types/points.uts');
  const conf = read('pages.json');
  const routes = registeredRoutes(conf);
  /** 后端可兑性对账表（真源不在本端，只钉「同源」这一层）：仓库根 = 移动端工程根上溯两级 */
  const REGISTRY = h.readText(path.join(ROOT, '..', '..', 'backend', 'internal', 'points', 'sku_registry.go'));

  it('① 旧路由从路由表消失、旧页与它的私有卡片文件一并删除（无兼容重定向）', () => {
    expect(routes).not.toContain('pages/points/points-shop');
    expect(h.exists('pages/points/points-shop.uvue')).toBe(false);
    // 卡片唯一消费者就是被删的页 ⇒ 随页下线，不在 points 域里留第二份三态渲染器
    expect(h.exists('pages/points/components/points-shop-card.uvue')).toBe(false);
    // 不做旧路径兼容重定向：无外部深链，运行时也没有路由重定向机制
    expect(conf).not.toContain('redirect');
  });

  it('② 退役清零：全工程源文件无旧路由字面量残留（入口 / 深链 / 重定向 / 注释引用都不许复活）', () => {
    // 射程同 ⑦：整个工程源文件面，不按目录白名单挑（漏扫的目录就是下一次复活的藏身处）
    const offenders = h.sourceFilesIn('.').filter((f) => read(f).includes('points-shop'));
    expect(offenders).toEqual([]);
  });

  it('③ 合并页是唯一货架路由（路由表里再没有第二个通向货架的登记）', () => {
    expect(routes).toContain('pages/mall/mall');
    expect(routes.filter((r) => /(^|\/)mall$/.test(r))).toEqual(['pages/mall/mall']);
  });

  it('④ 契约迁移·列表接口：单动词 /points/shop/items；stock 与 owned 都照抄服务端', () => {
    expect(api).toMatch(/export function getShopItemsApi\(\) : Promise<ShopItemsResult>/);
    expect(api).toMatch(/getMapped<ShopItemsResult>\('\/points\/shop\/items'/);
    // stock 用 toNumberOrNull（null 是不限库存，当 0 会把在售项标成售罄）
    expect(api).toMatch(/stock: toNumberOrNull\(obj\['stock'\]\)/);
    expect(types).toMatch(/stock : number \| null/);
    // owned 照抄服务端，不在客户端重算（否则「列表说没拥有、兑换说已拥有」）
    expect(api).toMatch(/owned: toBool\(obj\['owned'\]\)/);
    expect(types).toMatch(/owned : boolean/);
  });

  it('⑤ 契约迁移·筛选骨架：三参下发同一个动词，值走字符串；无档走原动词（商品段是全量）', () => {
    expect(api).toMatch(/export function getShopItemsFilteredApi\(category : string, pointsMin : number, pointsMax : number\) : Promise<ShopItemsResult>/);
    const fn = api.slice(api.indexOf('function getShopItemsFilteredApi'));
    expect(fn.slice(0, 900)).toContain("query['category'] = category");
    expect(fn.slice(0, 900)).toContain("query['points_min'] = pointsMin.toString()");
    expect(fn.slice(0, 900)).toContain("query['points_max'] = pointsMax.toString()");
    const load = state.slice(state.indexOf('async function loadItems'));
    expect(load.slice(0, 900)).toContain('getShopItemsApi()');
    expect(load.slice(0, 900)).toContain("getShopItemsFilteredApi('', range.min, range.max)");
  });

  it('⑥ 契约迁移·档位变化即重取两路（不是「只换高亮再重载同一份数据」的假控件）', () => {
    const fn = state.slice(state.indexOf('function onPickRange'));
    const body = fn.slice(0, fn.indexOf('\n    }'));
    expect(body).toContain('activeRangeIdx.value = idx');
    expect(body).toContain('loadItems()');
    expect(body).toContain('loadBookItems()');
  });

  it('⑦ 契约迁移·可点性只由服务端事实决定；成功后按服务端回包刷新余额与列表并落权益页', () => {
    // 已拥有 / 售罄 ⇒ 不可点，且防重入（兑换请求在途时再点无效）
    expect(state).toMatch(/if \(item\.owned\) return true/);
    expect(state).toMatch(/if \(blocked\(item\) \|\| redeemingSku\.value\.length > 0\) return/);
    // 成功回流：余额只认服务端回包，列表重拉（owned 态以服务端为准），不客户端预扣
    expect(state).toMatch(/pointsBalance\.value = res\.balance/);
    expect(state).toMatch(/await loadItems\(\)/);
    expect(state).toMatch(/if \(sku == AI_PRO_SKU\)/);
    expect(state).toMatch(/uni\.switchTab\(\{ url: '\/pages\/ai-assistant\/ai-assistant' \}\)/);
    expect(state).not.toMatch(/pointsBalance\.value = pointsBalance\.value -/);
  });

  it('⑧ 契约迁移·后端对账表同源：ai_pro 登记了权益读者，键形状是 (ai_pro, ai_pro)', () => {
    expect(REGISTRY).toContain('const aiProSKU = "ai_pro"');
    expect(REGISTRY).toMatch(/EntitlementSKU: func\(_ string\) string \{ return aiProSKU \}/);
    expect(REGISTRY).toMatch(/aiProSKU: \{/);
  });

  it('⑨ 契约迁移·余额卡随页下线迁入 mall 组件面（消费者归位），页面用它的 PascalCase 标签', () => {
    expect(h.exists('pages/mall/components/mall-balance-card.uvue')).toBe(true);
    expect(page).toContain("'./components/mall-balance-card.uvue'");
    expect(page).toMatch(/<MallBalanceCard[\s/>]/);
    // 旧家（points 域的组件目录）随旧页下线一并消失，不留第二份
    expect(h.exists('pages/points/components')).toBe(false);
  });
});
