/**
 * 商品详情页契约（#2518 第 5 刀，口径：G2 #2512 G-01 + 原型 810:3399）
 *
 * 源码契约缝：`pages/points/points-goods-detail.uvue` 与 `.uts` 都无法被 jest import
 * （同 pointsExchangeRecordsContract 先例），所以钉源码文本。这条链路横跨四处：
 *   ① 数据面：商品 = GET /points/shop/items 全量按 sku 对号（后端无单条路由，不另造）；
 *      image_url / description 是 N1 #2548 加列的**调用骨架**——列上线前恒 null，
 *      页面渲染空态（票面「不必等 N1 上线，即插即用」）；
 *   ② G-01：「使用期限」写进 description 文案，**不新增 validity 字段** ⇒ 页面不得出现
 *      独立的「使用期限」数据行（注释里的裁定解释不算渲染内容，剥注释后判）；
 *   ③ 兑换链路：与 points-shop 同一管线（showModal 预判 → redeemShopItemApi，守卫全在服务端）；
 *      落点 ai_pro 走 #1942 权益落点，SKU 常量单点消费 useAiPro、不写字面量；
 *   ④ 切图落位：image 27（商品图兜底）与 主要事项（注意事项图标）包内字面量路径、文件在位。
 *
 * 已知边界（如实声明）：本文件只钉客户端这一半；N1 四列的迁移与查询参数由后端票 #2548 钉住。
 */
const path = require('path');
const fs = require('fs');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const PAGE = read('pages/points/points-goods-detail.uvue');
const COMPOSABLE = read('pages/points/composables/usePointsGoodsDetail.uts');
const API = read('api/points.uts');
const TYPES = read('types/points.uts');
const PAGES_JSON = read('pages.json');

/** 剥注释后再判语义：注释里提到「使用期限」是裁定解释，不是渲染内容 */
const PAGE_NO_COMMENT = PAGE
  .replace(/<!--[\s\S]*?-->/g, '')
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .replace(/^\s*\/\/.*$/gm, '');
const COMPOSABLE_NO_COMMENT = COMPOSABLE
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .replace(/^\s*\/\/.*$/gm, '');

describe('商品详情页（#2518 第 5 刀）契约', () => {
  describe('① 路由与页面形状', () => {
    it('页面已在 pages.json 登记（未登记 ⇒ 路由打不开），custom 导航照兑换记录先例', () => {
      expect(PAGES_JSON).toContain('"path": "pages/points/points-goods-detail"');
      expect(PAGES_JSON).toContain('"navigationBarTitleText": "商品详情"');
    });

    it('页面壳层零自持状态：不 import ref / reactive，只解构 composable', () => {
      expect(PAGE).not.toMatch(/import \{[^}]*\bref\b[^}]*\} from 'vue'/);
      expect(PAGE).not.toMatch(/\breactive\(/);
      expect(PAGE).toMatch(/usePointsGoodsDetail\(\)/);
    });

    it('sku 从 onLoad 取参，重试复发同一请求（普通 let 回声，不是响应式状态）', () => {
      expect(PAGE).toMatch(/onLoad\(\(options : UTSJSONObject\) => \{/);
      expect(PAGE).toMatch(/const raw = options\['sku'\]/);
      expect(PAGE).toMatch(/let currentSku = ''/);
    });
  });

  describe('② 数据面：列表对号 + N1 #2548 调用骨架', () => {
    it('商品走 GET /points/shop/items 全量按 sku 对号，不另造单条路由', () => {
      expect(COMPOSABLE).toMatch(/getShopItemsApi\(\)/);
      expect(COMPOSABLE).not.toMatch(/getShopItemsFilteredApi/);
    });

    it('ShopItem 带 N1 骨架位 image_url / description（可空，列上线前恒 null）', () => {
      expect(TYPES).toMatch(/image_url : string \| null/);
      expect(TYPES).toMatch(/description : string \| null/);
    });

    it('buildShopItem 对骨架位做 null 回退（expires_at 设计位同款读法），上线即插即用', () => {
      expect(API).toMatch(/const imageRaw = obj\['image_url'\]/);
      expect(API).toMatch(/const descRaw = obj\['description'\]/);
      expect(API).toMatch(/image_url: imageRaw != null \? toStr\(imageRaw\) : null/);
      expect(API).toMatch(/description: descRaw != null \? toStr\(descRaw\) : null/);
    });

    it('description 直出商品详情卡，未上线渲染空态（空态不是错误态）', () => {
      expect(PAGE).toContain('暂无商品详情');
      expect(PAGE).toMatch(/v-if="goodsDescText\.length > 0"/);
      expect(PAGE).toMatch(/v-else class="desc-empty"/);
    });

    it('余额 silent 读（失败不挡详情，ADR-0031 决策 3），展示走千分位', () => {
      expect(COMPOSABLE).toMatch(/getPointsBalanceApi\(true\)/);
      expect(COMPOSABLE).toMatch(/formatCountFull\(pointsBalance\.value\)/);
    });
  });

  describe('③ G-01：使用期限不设独立 UI 行', () => {
    it('页面不出现「使用期限 / 有效期」渲染内容（信息按裁定写进 description）', () => {
      expect(PAGE_NO_COMMENT).not.toContain('使用期限');
      expect(PAGE_NO_COMMENT).not.toContain('有效期');
    });

    it('注意事项卡静态行只用系统真话：库存有限才说「数量有限」（不限库存不撒谎）', () => {
      expect(PAGE).toMatch(/v-if="hasStockLimit" class="notice-line">数量有限，兑完即止/);
      expect(COMPOSABLE).toMatch(/return g != null && g\.stock != null/);
    });
  });

  describe('④ 兑换链路：与 points-shop 同一管线', () => {
    it('CTA 可点性只由服务端事实决定（已拥有 / 售罄），停用词四态齐', () => {
      expect(COMPOSABLE).toMatch(/if \(item\.owned\) return true/);
      expect(COMPOSABLE).toMatch(/if \(s != null && s == 0\) return true/);
      expect(COMPOSABLE).toContain("'已拥有'");
      expect(COMPOSABLE).toContain("'已售罄'");
      expect(COMPOSABLE).toContain("'兑换中'");
      expect(COMPOSABLE).toContain("'立即兑换'");
    });

    it('兑换必过确认弹窗；余额不足要明说，但不客户端预扣', () => {
      expect(COMPOSABLE).toMatch(/uni\.showModal\(\{/);
      expect(COMPOSABLE).toMatch(/'余额不足，继续兑换会被服务端拒绝。'/);
      expect(COMPOSABLE).toMatch(/redeemShopItemApi\(item\.sku\)/);
      expect(COMPOSABLE).not.toMatch(/pointsBalance\.value\s*-=|pointsBalance\.value\s*=\s*pointsBalance\.value\s*-/);
    });

    it('成功后按服务端回包刷新余额并重拉商品（owned 态以服务端为准）', () => {
      expect(COMPOSABLE).toMatch(/pointsBalance\.value = res\.balance/);
      expect(COMPOSABLE).toMatch(/await loadGoods\(\)/);
    });

    it('SKU 常量单点消费 useAiPro，不写 ai_pro 字面量（#1942 落点判定）', () => {
      expect(COMPOSABLE).toContain("import { AI_PRO_SKU } from '../../../composables/useAiPro'");
      expect(COMPOSABLE_NO_COMMENT).not.toMatch(/'ai_pro'/);
      expect(COMPOSABLE).toMatch(/if \(s == AI_PRO_SKU\) \{/);
    });

    it('落点：ai_pro → switchTab AI 助手（#1942）；其余 SKU → 兑换记录页（原型边 811:3685 → 608:9111，G-00 改语义）', () => {
      expect(COMPOSABLE).toMatch(/uni\.switchTab\(\{ url: '\/pages\/ai-assistant\/ai-assistant' \}\)/);
      expect(COMPOSABLE).toMatch(/uni\.navigateTo\(\{ url: '\/pages\/points\/points-exchange-records' \}\)/);
    });
  });

  describe('⑤ 切图落位（image-assets.md：包内字面量路径、禁 webp）', () => {
    it('商品图兜底与注意事项图标都写包内字面量路径', () => {
      expect(PAGE).toContain('src="/static/icons/points-goods/goods-default.png"');
      expect(PAGE).toContain('src="/static/icons/points-goods/notice.png"');
    });

    it('两件调色板 PNG 真的在包里', () => {
      for (const f of ['goods-default.png', 'notice.png']) {
        const p = path.join(ROOT, 'static', 'icons', 'points-goods', f);
        expect(fs.existsSync(p)).toBe(true);
        expect(fs.statSync(p).size).toBeGreaterThan(0);
      }
    });

    it('网络图分支显式 :webp="true"（服务端 filestore 全转 webp，漏写即真机空白）', () => {
      expect(PAGE).toMatch(/:src="goodsImageUrl" class="goods-img"\s*\n\s*mode="aspectFill" :webp="true"/);
    });
  });
});
