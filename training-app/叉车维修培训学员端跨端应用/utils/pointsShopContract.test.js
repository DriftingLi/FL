/**
 * 积分商城页契约（#1942，口径：货源 B / 验收 A / 入口 B —— 2026-10-29 维护者裁定）。
 *
 * 源码契约缝：`pages/points/points-shop.uvue` 与 `.uts` 都无法被 jest import（同 aiProGateContract 先例），
 * 而这条链路横跨四处——列表接口、商城页、两个入口、后端对账表：
 *   ① 列表只信服务端：owned 由后端按人算，客户端不本地推断（否则「列表说没拥有、兑换说已拥有」）；
 *   ② 可点性只由服务端事实决定：已拥有 / 售罄 ⇒ 按钮不可点；
 *   ③ 兑换必须过确认，且余额不足要明说（但仍由服务端裁定，不客户端预扣）；
 *   ④ 入口与落点：任务中心死按钮 + 积分明细 banner → 商城页；成功后落到权益页（AI 助手）。
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

const PAGE = read('pages/points/points-shop.uvue');
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
      expect(PAGE).toMatch(/getShopItemsApi\(\)/);
      expect(PAGE).toMatch(/getPointsBalanceApi\(true\)/);
      expect(PAGE).toContain('暂无可兑换商品');
      expect(PAGE).toContain('商品加载失败');
    });

    it('已拥有 ⇒ 按钮不可点、文案「已拥有」（防重复兑换，服务端判据）', () => {
      expect(PAGE).toMatch(/if \(item\.owned\) return true/);
      expect(PAGE).toContain("if (item.owned) return '已拥有'");
      expect(PAGE).toMatch(/if \(blocked\(item\) \|\| redeemingSku\.value\.length > 0\) return/);
    });

    it('兑换必过确认弹窗；余额不足要明说，但不客户端预扣', () => {
      expect(PAGE).toContain('uni.showModal');
      expect(PAGE).toMatch(/const short = pointsBalance\.value < item\.price/);
      expect(PAGE).toContain('余额不足，继续兑换会被服务端拒绝。');
      expect(PAGE).toMatch(/redeemShopItemApi\(item\.sku\)/);
      expect(PAGE).not.toMatch(/pointsBalance\.value = pointsBalance\.value -/);
    });

    it('成功后按服务端回包刷新余额与列表，并落到权益页（AI 助手）', () => {
      expect(PAGE).toMatch(/pointsBalance\.value = res\.balance/);
      expect(PAGE).toMatch(/await loadItems\(\)/);
      expect(PAGE).toMatch(/if \(sku == AI_PRO_SKU\)/);
      expect(PAGE).toMatch(/uni\.switchTab\(\{ url: '\/pages\/ai-assistant\/ai-assistant' \}\)/);
    });

    it('页面已在 pages.json 登记（未登记 ⇒ 路由打不开）', () => {
      expect(PAGES_JSON).toContain('"path": "pages/points/points-shop"');
      expect(PAGES_JSON).toContain('"navigationBarTitleText": "积分商城"');
    });
  });

  describe('③ 两个入口（口径 B）', () => {
    it('任务中心「去兑换」不再是 toast 死按钮', () => {
      const fn = TASK_CENTER.slice(TASK_CENTER.indexOf('function goExchange'));
      expect(fn.slice(0, 240)).toContain("uni.navigateTo({ url: '/pages/points/points-shop' })");
      expect(fn.slice(0, 240)).not.toContain('功能开发中');
    });

    it('积分明细页有「去积分商城」入口条，同落点', () => {
      expect(POINTS_DETAIL).toContain('shop-entry');
      expect(POINTS_DETAIL).toContain('去积分商城挑好物');
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
});
