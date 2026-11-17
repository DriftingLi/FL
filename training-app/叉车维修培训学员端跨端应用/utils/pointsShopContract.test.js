/**
 * 积分商城页契约（#1942，口径：货源 B / 验收 A / 入口 B —— 2026-10-29 维护者裁定）。
 *
 * 源码契约缝：`pages/points/points-shop.uvue` 与 `.uts` 都无法被 jest import（同 aiProGateContract 先例），
 * 而这条链路横跨四处——列表接口、商城页、两个入口、后端对账表：
 *   ① 列表只信服务端：owned 由后端按人算，客户端不本地推断（否则「列表说没拥有、兑换说已拥有」）；
 *   ② 可点性只由服务端事实决定：已拥有 / 售罄 ⇒ 按钮不可点；
 *   ③ 兑换必须过确认，且余额不足要明说（但仍由服务端裁定，不客户端预扣）；
 *   ④ 入口与落点：任务中心死按钮 + 积分明细 banner → 商城页；成功后落到权益页（AI 助手）。
 *   ⑤ 课程兑换导流卡（#2531）已于 #2619 T3 退役（连同它依赖的一次性跨页意图两半）；本页此刻是
 *      合并页的过渡形态，入口收敛与整页下线分别在 #2620 / #2621。
 *   —— #2617 T1（页面状态下沉 + 组件拆分，行为不变）后的读法调整：
 *   符号 / 函数名类断言读 SRC（页面 + pages/mall/composables/useMall.uts），模板类断言读 TPL_ALL
 *   （页面模板 + 共用的「积分范围」筛选栏模板）；「不自持状态」「不探测服务端」这类判据仍只读 PAGE。
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
/** #2619 T3：筛选栏收成「积分范围」单控件，两页共用 mall 模块那一份（分类面板与「可换商品」退役） */
const FILTER_BAR = read('pages/mall/components/mall-range-filter.uvue');
/** 符号 / 函数名类断言读的并集：页面壳层 + 下沉后的家 */
const SRC = PAGE + '\n' + MALL_STATE;
/** 模板类断言读的并集：页面模板 + 共用的「积分范围」筛选栏模板（注释剥离：头注里的退役说明不算标签） */
const stripTplComments = (s) => s.replace(/<!--[\s\S]*?-->/g, '');
const TPL_ALL = stripTplComments(PAGE.slice(PAGE.indexOf('<template>'), PAGE.lastIndexOf('</template>')))
  + stripTplComments(FILTER_BAR.slice(FILTER_BAR.indexOf('<template>'), FILTER_BAR.lastIndexOf('</template>')));
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

  describe('③ 两个入口（口径 B）', () => {
    it('任务中心「去兑换」不再是 toast 死按钮（#2620 起落合并页的积分兑换分区）', () => {
      const fn = TASK_CENTER.slice(TASK_CENTER.indexOf('function goExchange'));
      expect(fn.slice(0, 240)).toContain("uni.navigateTo({ url: '/pages/mall/mall?tab=points' })");
      expect(fn.slice(0, 240)).not.toContain('功能开发中');
    });

    it('积分明细页有「去商城」入口条，同落点', () => {
      // #2523 起入口条按原型 355:1725 收口为浅黄提示条（class notice-bar）；
      // #2620 起文案不再把「积分商城」当页面名（词表：它只指兑换管线）
      expect(POINTS_DETAIL).toContain('notice-bar');
      expect(POINTS_DETAIL).toContain('去商城');
      expect(POINTS_DETAIL).not.toContain('去积分商城');
      expect(POINTS_DETAIL).toMatch(/function onGoShop\(\) : void \{\s*uni\.navigateTo\(\{ url: '\/pages\/mall\/mall\?tab=points' \}\)/);
    });
  });

  describe('④ 后端对账表同源（不改它 ⇒ 商城恒空）', () => {
    it('ai_pro 登记了权益读者，键形状是 (ai_pro, ai_pro)', () => {
      expect(REGISTRY).toContain('const aiProSKU = "ai_pro"');
      expect(REGISTRY).toMatch(/EntitlementSKU: func\(_ string\) string \{ return aiProSKU \}/);
      expect(REGISTRY).toMatch(/aiProSKU: \{/);
    });
  });

  describe('⑥ 左栏三态与单筛选控件（#2518 第 2 刀的三态 + #2619 T3 的筛选收口）', () => {
    /** 模板切片（同既有口径：只在模板里找位置，避免样式块里的同名选择器把判据弄虚） */
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

    it('筛选只剩「积分范围」单控件（三档 + 不限复位）：分类五档 / 可换商品 / 展开面板 / 蒙层全退役', () => {
      expect(TPL_ALL).toContain('积分范围');
      expect(TPL_ALL).not.toContain('商品分类');
      expect(TPL_ALL).not.toContain('可换商品');
      // 面板与蒙层两件随分类面板一起退役：单控件常驻，没有第二层可展开
      expect(TPL_ALL).not.toContain('list-mask');
      expect(TPL_ALL).not.toContain('filter-panel');
      expect(SRC).not.toContain('categoryOptions');
      expect(SRC).not.toContain('filterPanel');
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

    it('无筛选仍走原动词 getShopItemsApi()；带范围档时 category 传空串（商品段是「全量」）', () => {
      const fn = SRC.slice(SRC.indexOf('async function loadItems'));
      expect(fn.slice(0, 900)).toContain('getShopItemsApi()');
      expect(fn.slice(0, 900)).toContain("getShopItemsFilteredApi('', range.min, range.max)");
    });

    it('档位变化即重取两路（不是「只换高亮再重载同一份数据」的假控件）', () => {
      const fn = SRC.slice(SRC.indexOf('function onPickRange'));
      const body = fn.slice(0, fn.indexOf('\n    }'));
      expect(body).toContain('activeRangeIdx.value = idx');
      expect(body).toContain('loadItems()');
      expect(body).toContain('loadBookItems()');
    });
  });

  describe('⑦ #2617 T1 组件拆分与页面壳层零自持状态（行为不变，断言强度不变）', () => {
    it('页面壳层不自持状态：ref / reactive / computed 全在 useMall（口径同 pointsExchangeRecordsContract）', () => {
      expect(PAGE).not.toMatch(/import\s*\{[^}]*\bref\b[^}]*\}\s*from\s*'vue'/);
      expect(PAGE).not.toMatch(/\breactive\(/);
      expect(PAGE).not.toMatch(/\bcomputed\(/);
      expect(PAGE).toContain('useMall()');
    });

    it('余额卡 / 商品卡各有独立组件文件（筛选栏 #2619 T3 起与合并页共用 mall 模块那一份）', () => {
      const COMPONENTS = ['points-balance-card', 'points-shop-card'];
      for (const name of COMPONENTS) {
        expect(read('pages/points/components/' + name + '.uvue')).toContain('<template>');
        expect(PAGE).toContain('./components/' + name + '.uvue');
        expect(PAGE).toContain('<' + name);
      }
      expect(PAGE).toContain("'../mall/components/mall-range-filter.uvue'");
      expect(PAGE).toContain('<MallRangeFilter');
    });
  });
});
