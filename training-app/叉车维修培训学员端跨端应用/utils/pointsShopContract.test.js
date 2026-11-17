/**
 * 积分商城页契约（#1942，口径：货源 B / 验收 A / 入口 B —— 2026-10-29 维护者裁定）。
 *
 * 源码契约缝：`pages/points/points-shop.uvue` 与 `.uts` 都无法被 jest import（同 aiProGateContract 先例），
 * 而这条链路横跨四处——列表接口、商城页、两个入口、后端对账表：
 *   ① 列表只信服务端：owned 由后端按人算，客户端不本地推断（否则「列表说没拥有、兑换说已拥有」）；
 *   ② 可点性只由服务端事实决定：已拥有 / 售罄 ⇒ 按钮不可点；
 *   ③ 兑换必须过确认，且余额不足要明说（但仍由服务端裁定，不客户端预扣）；
 *   ④ 入口与落点：任务中心死按钮 + 积分明细 banner → 商城页；成功后落到权益页（AI 助手）。
 *   ⑤ 课程兑换导流（#2531）：余额卡之后、商品列表之前的一张**静态**卡；CTA = 写一次性跨页意图
 *      + switchTab 到课程列表（不是 navigateTo）；打开商城不多发任何证件 / 课程请求。
 *   —— #2617 T1（页面状态下沉 + 组件拆分，行为不变）后的读法调整：
 *   符号 / 函数名类断言读 SRC（页面 + pages/mall/composables/useMall.uts），模板类断言读 TPL_ALL
 *   （页面模板 + points-filter-bar 组件模板）；「不自持状态」「不探测服务端」这类判据仍只读 PAGE。
 *
 * 已知边界（如实声明）：本文件只钉客户端这一半；可兑性由后端对账表（shopSKUDecls）决定，
 * 那边由 backend/internal/points/sku_registry_test.go 与 shop_ai_pro_contract_test.go 钉住。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const REPO = path.join(ROOT, '..', '..');
const read = (p) => readText(path.join(ROOT, p));
/** pages.json 路由拍平（#2579 起非 tabBar 页整目录切进分包，分包 path 是相对 root 的短名） */
const { registeredRoutes } = require('../scripts/lib/pages-routes.js');

const PAGE = read('pages/points/points-shop.uvue');
/** #2617 T1：页面状态与动作下沉的双页共享 composable（积分商品路在这里） */
const MALL_STATE = read('pages/mall/composables/useMall.uts');
/** #2617 T1：筛选栏（含两套展开面板）拆出的组件，模板类断言读它的模板 */
const FILTER_BAR = read('pages/points/components/points-filter-bar.uvue');
/** 符号 / 函数名类断言读的并集：页面壳层 + 下沉后的家 */
const SRC = PAGE + '\n' + MALL_STATE;
/** 模板类断言读的并集：页面模板 + 筛选栏组件模板 */
const TPL_ALL = PAGE.slice(PAGE.indexOf('<template>'), PAGE.lastIndexOf('</template>'))
  + FILTER_BAR.slice(FILTER_BAR.indexOf('<template>'), FILTER_BAR.lastIndexOf('</template>'));
const API = read('api/points.uts');
const TYPES = read('types/points.uts');
const TASK_CENTER = read('pages/points/task-center.uvue');
const POINTS_DETAIL = read('pages/points/points-detail.uvue');
const PAGES_JSON = read('pages.json');
const REGISTRY = readText(path.join(REPO, 'backend', 'internal', 'points', 'sku_registry.go'));

describe('积分商城页（#1942）契约', () => {
  describe('① 列表接口（api/points.uts）', () => {
    it('走 GET /points/shop/items，不另造路由', () => {
      expect(API).toMatch(/export function getShopItemsApi\(\) : Promise<ShopItemsResult>/);
      expect(API).toMatch(/getMapped<ShopItemsResult>\('\/points\/shop\/items'/);
    });

    it('stock 用 toNumberOrNull（null 是不限库存，当 0 会把在售项标成售罄）', () => {
      expect(API).toMatch(/stock: toNumberOrNull\(obj\['stock'\]\)/);
      expect(TYPES).toMatch(/stock : number \| null/);
    });

    it('owned 照抄服务端，不在客户端重算', () => {
      expect(API).toMatch(/owned: toBool\(obj\['owned'\]\)/);
      expect(TYPES).toMatch(/owned : boolean/);
    });
  });

  describe('② 商城页（pages/points/points-shop.uvue）', () => {
    it('列表与余额都来自后端，空列表是合法结果（空态不是错误态）', () => {
      expect(SRC).toMatch(/getShopItemsApi\(\)/);
      expect(SRC).toMatch(/getPointsBalanceApi\(true\)/);
      expect(PAGE).toContain('暂无可兑换商品');
      expect(PAGE).toContain('商品加载失败');
    });

    it('已拥有 ⇒ 按钮不可点、文案「已拥有」（防重复兑换，服务端判据）', () => {
      expect(SRC).toMatch(/if \(item\.owned\) return true/);
      expect(SRC).toContain("if (item.owned) return '已拥有'");
      expect(SRC).toMatch(/if \(blocked\(item\) \|\| redeemingSku\.value\.length > 0\) return/);
    });

    it('兑换必过确认弹窗；余额不足要明说，但不客户端预扣', () => {
      expect(SRC).toContain('uni.showModal');
      expect(SRC).toMatch(/const short = pointsBalance\.value < item\.price/);
      expect(SRC).toContain('余额不足，继续兑换会被服务端拒绝。');
      expect(SRC).toMatch(/redeemShopItemApi\(item\.sku\)/);
      expect(SRC).not.toMatch(/pointsBalance\.value = pointsBalance\.value -/);
    });

    it('成功后按服务端回包刷新余额与列表，并落到权益页（AI 助手）', () => {
      expect(SRC).toMatch(/pointsBalance\.value = res\.balance/);
      expect(SRC).toMatch(/await loadItems\(\)/);
      expect(SRC).toMatch(/if \(sku == AI_PRO_SKU\)/);
      expect(SRC).toMatch(/uni\.switchTab\(\{ url: '\/pages\/ai-assistant\/ai-assistant' \}\)/);
    });

    it('页面已在 pages.json 登记（未登记 ⇒ 路由打不开）', () => {
      expect(registeredRoutes(PAGES_JSON)).toContain('pages/points/points-shop');
      expect(PAGES_JSON).toContain('"navigationBarTitleText": "积分商城"');
    });
  });

  describe('⑤ 课程兑换导流卡（#2531：只加一张静态卡，不动兑换链路）', () => {
    /** 只在模板里找位置，避免样式块里的同名选择器把顺序判据弄虚 */
    const tpl = PAGE.slice(PAGE.indexOf('<template>'), PAGE.lastIndexOf('</template>'));

    it('卡片位置固定：余额卡之后、商品列表之前（三件文案齐）', () => {
      const iBalance = tpl.indexOf('balance-card');
      const iEntry = tpl.indexOf('course-entry');
      const iShop = tpl.indexOf('shop-card');
      expect(iBalance).toBeGreaterThan(-1);
      expect(iEntry).toBeGreaterThan(iBalance);
      expect(iShop).toBeGreaterThan(iEntry);
      expect(tpl).toContain('课程兑换');
      expect(tpl).toContain('去兑换');
      // 与课程详情确认弹窗「用积分解锁付费课」同一口径：不写价格、不写营销话
      expect(tpl).toContain('用积分解锁付费课');
    });

    it('CTA = 写一次性意图 + switchTab 到课程列表（navigateTo 会开出与 tabBar 各活一份的第二页）', () => {
      const fn = PAGE.slice(PAGE.indexOf('function onGoCourseRedeem'));
      const body = fn.slice(0, fn.indexOf('\n    }'));
      expect(body).toMatch(/requestCourseRedeemIntent()/);
      expect(body).toContain("uni.switchTab({ url: '/pages/courses/courses' })");
      expect(body).not.toContain('uni.navigateTo');
      // 意图写的是导航模块里那一个（不是页面里自造的第三个状态）
      expect(PAGE).toContain("import { goBack, requestCourseRedeemIntent } from '../../utils/navigation'");
    });

    it('不探测服务端：打开商城仍然只发余额 + 列表两次请求，没有课程 / 证件请求', () => {
      const show = PAGE.slice(PAGE.indexOf('onShow('));
      expect(show.slice(0, 120)).toMatch(/loadBalance\(\)\s*loadItems\(\)/);
      expect(PAGE).not.toMatch(/getCourseListApi|getCurrentCredentialApi/);
    });

    it('导流只走课程列表：卡片里不出现实物商城 / mall 的入口', () => {
      const card = tpl.slice(tpl.indexOf('course-entry'), tpl.indexOf('shop-card'));
      expect(card).not.toMatch(/mall|实物商城|商品商城/);
    });
  });

  describe('③ 两个入口（口径 B）', () => {
    it('任务中心「去兑换」不再是 toast 死按钮', () => {
      const fn = TASK_CENTER.slice(TASK_CENTER.indexOf('function goExchange'));
      expect(fn.slice(0, 240)).toContain("uni.navigateTo({ url: '/pages/points/points-shop' })");
      expect(fn.slice(0, 240)).not.toContain('功能开发中');
    });

    it('积分明细页有「去积分商城」入口条，同落点', () => {
      // #2523 起入口条按原型 355:1725 收口为浅黄提示条（class notice-bar），文案同「去积分商城」
      expect(POINTS_DETAIL).toContain('notice-bar');
      expect(POINTS_DETAIL).toContain('去积分商城');
      expect(POINTS_DETAIL).toMatch(/function onGoShop\(\) : void \{\s*uni\.navigateTo\(\{ url: '\/pages\/points\/points-shop' \}\)/);
    });
  });

  describe('④ 后端对账表同源（不改它 ⇒ 商城恒空）', () => {
    it('ai_pro 登记了权益读者，键形状是 (ai_pro, ai_pro)', () => {
      expect(REGISTRY).toContain('const aiProSKU = "ai_pro"');
      expect(REGISTRY).toMatch(/EntitlementSKU: func\(_ string\) string \{ return aiProSKU \}/);
      expect(REGISTRY).toMatch(/aiProSKU: \{/);
    });
  });

  describe('⑥ 左栏三态与筛选两态（#2518 第 2 刀；原型 821:6396 主态 + 815:5332 筛选态，贴片 807:2035/807:2190）', () => {
    /** 模板切片（同 ⑤ 的口径：只在模板里找位置） */
    const tpl = PAGE.slice(PAGE.indexOf('<template>'), PAGE.lastIndexOf('</template>'));

    it('左栏三态齐：热门（原型无出边，只陈列）/ 图书系列 / 积分兑换（本页态）', () => {
      const side = tpl.slice(tpl.indexOf('shop-sidebar'), tpl.indexOf('content-col'));
      expect(side).toContain('热门');
      expect(side).toContain('图书系列');
      expect(side).toContain('积分兑换');
    });

    it('图书系列 → mall 路由（821:6678 → 824:6971 边；Q8 两路由不合并，navigateTo 不 switchTab）', () => {
      const fn = PAGE.slice(PAGE.indexOf('function onGoMall'));
      expect(fn.slice(0, 240)).toContain("uni.navigateTo({ url: '/pages/mall/mall' })");
      expect(fn.slice(0, 240)).not.toContain('uni.switchTab');
    });

    it('筛选两态：分类五档 enum 按 N1 #2548（book|material|course|other），积分范围档位按票面（≤500/500-1000/>1000）', () => {
      expect(TPL_ALL).toContain('商品分类');
      expect(TPL_ALL).toContain('积分范围');
      expect(TPL_ALL).toContain('可换商品');
      expect(SRC).toContain("value: 'book'");
      expect(SRC).toContain("value: 'material'");
      expect(SRC).toContain("value: 'course'");
      expect(SRC).toContain("value: 'other'");
      expect(SRC).toContain('≤500');
      expect(SRC).toContain('500-1000');
      expect(SRC).toContain('>1000');
    });

    it('筛选骨架：getShopItemsFilteredApi 三参下发同一 /points/shop/items，值走字符串（buildQuery 值强转 as string）', () => {
      expect(API).toMatch(/export function getShopItemsFilteredApi\(category : string, pointsMin : number, pointsMax : number\) : Promise<ShopItemsResult>/);
      const fn = API.slice(API.indexOf('function getShopItemsFilteredApi'));
      expect(fn.slice(0, 900)).toContain("query['category'] = category");
      expect(fn.slice(0, 900)).toContain("query['points_min'] = pointsMin.toString()");
      expect(fn.slice(0, 900)).toContain("query['points_max'] = pointsMax.toString()");
    });

    it('无筛选仍走原动词 getShopItemsApi()（既有契约调用形状不因筛选改道）', () => {
      const fn = SRC.slice(SRC.indexOf('async function loadItems'));
      expect(fn.slice(0, 900)).toContain('getShopItemsApi()');
      expect(fn.slice(0, 900)).toContain('getShopItemsFilteredApi(cat, range.min, range.max)');
    });

    it('「可换商品」→ 展开分类面板（821:6546 → 815:5332 边）；点蒙层回主态（815:5463 → 821:6396 边）', () => {
      const hot = SRC.slice(SRC.indexOf('function onHotRedeemable'));
      expect(hot.slice(0, 240)).toContain("filterPanel.value = 'category'");
      const close = SRC.slice(SRC.indexOf('function onClosePanel'));
      expect(close.slice(0, 240)).toMatch(/filterPanel\.value = ''/);
      expect(tpl).toContain('list-mask');
    });

    it('筛选态一次只展开一套（面板互斥），蒙层只在展开时出现', () => {
      expect(TPL_ALL).toMatch(/v-if="filterPanel == 'category'"/);
      expect(TPL_ALL).toMatch(/v-if="filterPanel == 'range'"/);
      expect(tpl).toMatch(/v-if="filterPanel != ''" class="list-mask"/);
    });
  });
  describe('⑦ #2617 T1 组件拆分与页面壳层零自持状态（行为不变，断言强度不变）', () => {
    it('页面壳层不自持状态：ref / reactive / computed 全在 useMall（口径同 pointsExchangeRecordsContract）', () => {
      expect(PAGE).not.toMatch(/import\s*\{[^}]*\bref\b[^}]*\}\s*from\s*'vue'/);
      expect(PAGE).not.toMatch(/\breactive\(/);
      expect(PAGE).not.toMatch(/\bcomputed\(/);
      expect(PAGE).toContain('useMall()');
    });

    it('余额卡 / 筛选栏 / 商品卡各有独立组件文件，页面显式 import 并在模板使用（kebab 标签，⑤ 的顺序断言读它）', () => {
      const COMPONENTS = ['points-balance-card', 'points-filter-bar', 'points-shop-card'];
      for (const name of COMPONENTS) {
        expect(read('pages/points/components/' + name + '.uvue')).toContain('<template>');
        expect(PAGE).toContain('./components/' + name + '.uvue');
        expect(PAGE).toContain('<' + name);
      }
    });
  });
});
