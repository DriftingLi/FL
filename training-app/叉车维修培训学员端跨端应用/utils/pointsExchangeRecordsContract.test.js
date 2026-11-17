/**
 * 积分兑换记录页契约（#2518，口径：G2 #2512 的 G-00 + E-1 裁定）
 *
 * 源码契约缝：`pages/points/points-exchange-records.uvue` 与 `.uts` 都无法被 jest import
 * （同 pointsShopContract 先例），所以钉源码文本。这条链路横跨四处：
 *   ① 语义：这是**积分兑换记录，不是订单** —— 订单号 / 六态状态机 / 物流 / 发货态 / 售后 / 晒单 / 收货人
 *      全部被 G-00 裁定取消（不建 order / address / logistics / aftersale 四张表）；
 *   ② 数据面：`GET /points/ledger?direction=out` 取支出流水，客户端按 `reason` 前缀 `redeem_` 收窄
 *      （后端 ledger 没有 reason 过滤参数，`ai_tokens` 等其它支出混在同一方向里）；
 *   ③ 装载协议：必须走 `composables/usePagedList.uts`（ADR-0027），不得手写页码游标
 *      —— 否则 `utils/pagedListAdoptionContract.test.js` 的手写存量棘轮 +1 判红；
 *   ④ 商品名：`GET /points/shop/items` 把 sku 翻成标题，失败**不阻断列表**（退回流水事由文案）。
 *
 * 已知边界（如实声明）：本文件只钉客户端这一半；`redeem_*` 事由由后端写入，
 * 那边由 backend/internal/points 的兑换用例钉住。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));
/** pages.json 路由拍平（#2579 起非 tabBar 页整目录切进分包，分包 path 是相对 root 的短名） */
const { registeredRoutes } = require('../scripts/lib/pages-routes.js');

const PAGE = read('pages/points/points-exchange-records.uvue');

/** 剥注释后再判语义：注释里写「不是订单」是解释，不是渲染内容 */
const PAGE_NO_COMMENT = PAGE
  .replace(/<!--[\s\S]*?-->/g, '')
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .replace(/^\s*\/\/.*$/gm, '');
const COMPOSABLE = read('pages/points/composables/usePointsExchangeRecords.uts');
const PAGES_JSON = read('pages.json');

describe('积分兑换记录页（#2518）契约', () => {
  describe('① 语义：是兑换记录，不是订单（G-00 裁定）', () => {
    it('页面不出现任何订单 / 物流 / 售后 / 晒单 / 收货人语义', () => {
      const forbidden = [
        '订单编号',
        '订单号',
        '待发货',
        '待收货',
        '待付款',
        '申请售后',
        '去晒单',
        '收货人',
        '物流',
        '快递',
        '发货',
      ];
      for (const word of forbidden) {
        expect(PAGE_NO_COMMENT).not.toContain(word);
      }
    });

    it('页面标题与路由标题都是「兑换记录」', () => {
      expect(PAGE).toContain('title="兑换记录"');
      expect(registeredRoutes(PAGES_JSON)).toContain('pages/points/points-exchange-records');
      expect(PAGES_JSON).toContain('"navigationBarTitleText": "兑换记录"');
    });

    it('到账状态只有两态（已到账 / 待确认），不引入六态状态机', () => {
      expect(COMPOSABLE).toContain("'已到账'");
      expect(COMPOSABLE).toContain("'待确认'");
      expect(COMPOSABLE).not.toMatch(/statusText\s*=\s*'待发货'/);
    });
  });

  describe('② 数据面：支出流水 + 客户端按 reason 收窄', () => {
    it('只取 direction=out 的流水，不另造路由', () => {
      expect(COMPOSABLE).toMatch(/getPointsLedgerApi\(at, PAGE_SIZE, 'out'\)/);
    });

    it('兑换判定是 reason 前缀 redeem_（后端无 reason 过滤参数，只能客户端筛）', () => {
      expect(COMPOSABLE).toMatch(/function isRedeemReason\(reason : string\) : boolean/);
      expect(COMPOSABLE).toMatch(/reason\.indexOf\('redeem_'\) == 0/);
      expect(COMPOSABLE).toMatch(/if \(!isRedeemReason\(it\.reason\)\) continue/);
    });

    it('筛空且还有下一页时继续取（空批次兜底会把 hasMore 强制 false，不能就此收工）', () => {
      expect(COMPOSABLE).toMatch(/out\.length == 0 && data\.hasMore && depth < MAX_CHAIN/);
      expect(COMPOSABLE).toMatch(/return fetchBatch\(depth \+ 1\)/);
    });

    it('商品名取商城目录，取不到退回流水事由文案（不显示 sku 原文）', () => {
      expect(COMPOSABLE).toMatch(/ledgerReasonLabel\(item\.reason, item\.delta\)/);
      expect(COMPOSABLE).toMatch(/title = shop\.title/);
      expect(COMPOSABLE).not.toMatch(/title = item\.ref_id/);
    });

    it('商品目录失败不阻断列表（catch 后只 warn）', () => {
      const fn = COMPOSABLE.slice(COMPOSABLE.indexOf('function loadShopCatalog'));
      const body = fn.slice(0, 500);
      expect(body).toMatch(/\.catch\(\(e\) => \{/);
      expect(body).toContain('console.warn');
      expect(body).not.toMatch(/throw /);
    });
  });

  describe('③ 装载协议：走 usePagedList，不手写页码游标（ADR-0027）', () => {
    it('复用 composables/usePagedList.uts，不自己写 page 游标 ref', () => {
      expect(COMPOSABLE).toContain("from '../../../composables/usePagedList'");
      expect(COMPOSABLE).toMatch(/usePagedList<ExchangeRecord>/);
      // 手写存量棘轮的判据：page = ref 同时命中 loader 函数
      expect(COMPOSABLE).not.toMatch(/\b\w*[Pp]age\s*=\s*ref/);
    });

    it('后端游标是普通 let（不是 ref），首屏与重试都归 1', () => {
      expect(COMPOSABLE).toMatch(/let backendPage = 1/);
      const fn = COMPOSABLE.slice(COMPOSABLE.indexOf('function loadFirst'));
      expect(fn.slice(0, 120)).toMatch(/backendPage = 1/);
      expect(fn.slice(0, 120)).toMatch(/list\.loadFirst\(\)/);
    });

    it('页面零自持状态：不 import ref / reactive，只解构 composable', () => {
      expect(PAGE).not.toMatch(/import \{[^}]*\bref\b[^}]*\} from 'vue'/);
      expect(PAGE).not.toMatch(/\breactive\(/);
      expect(PAGE).toMatch(/usePointsExchangeRecords\(\)/);
    });
  });

  describe('④ 三态与页脚（空态不是错误态）', () => {
    it('错误态、空态、列表三态齐，且空态给出去商城的路', () => {
      expect(PAGE).toMatch(/v-if="failed && items\.length == 0"/);
      expect(PAGE).toMatch(/v-else-if="isEmpty"/);
      expect(PAGE).toContain('还没有兑换记录');
      expect(PAGE).toContain("uni.navigateTo({ url: '/pages/mall/mall?tab=points' })");
    });

    it('页脚三态：加载中 / 翻页失败可重试 / 没有更多了', () => {
      expect(PAGE).toContain('加载中...');
      expect(PAGE).toContain('加载失败，点击重试');
      expect(PAGE).toContain('没有更多了');
      expect(PAGE).toMatch(/v-else-if="failed && items\.length > 0"/);
      expect(PAGE).toMatch(/v-else-if="!hasMore && items\.length > 0"/);
    });

    it('首屏同时取商品目录与第一页流水', () => {
      const show = PAGE.slice(PAGE.indexOf('onShow('));
      expect(show.slice(0, 160)).toMatch(/loadShopCatalog\(\)/);
      expect(show.slice(0, 160)).toMatch(/loadFirst\(\)/);
    });
  });
});
