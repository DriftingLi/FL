/**
 * AI 专业版「积分兑换权益」契约测试（#920）—— 源码契约测试缝（读源文本、断言语义）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` / `composables/useAiPro.uts` /
 * `components/ai-chat/ai-chat-pro-sheet.uvue` 都无法在 jest import（uvue / UTS），
 * 而本票的三条行为链恰好横跨这三处：
 *   ① 门的事实源 —— 已拥有 ai_pro 怎么读出来的（后端权益无读面，用支出流水推导）；
 *   ② 兑换回路 —— 横幅 → 能力清单 sheet → 兑换 → **原地刷新门状态**；
 *   ③ 失败口径 —— 积分不足 / 已下架 / 已兑换各有可读归宿，**不静默失败**。
 * 钉住它们，避免后续会话「改一处忘一处」（#926 的漏网路径就是这么来的）。
 *
 * 已知边界（如实声明）：本票**不含后端改动** ⇒ 服务端未按权益拒绝被锁能力的请求，
 * 验收标准第 3 条（未解锁时服务端拒绝）待后端落地；契约测试只能钉客户端这一半。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
const SHEET = read('components/ai-chat/ai-chat-pro-sheet.uvue');
const ADAPTER = read('composables/useAiPro.uts');
const POINTS_API = read('api/points.uts');

describe('AI 专业版 = 积分兑换权益（#920）契约', () => {
  describe('① 门的事实源（composables/useAiPro.uts）', () => {
    it('SKU / 价格 / 兑换事由键集中在适配器，且 SKU 与票面、后端种子同源', () => {
      expect(ADAPTER).toMatch(/export const AI_PRO_SKU : string = 'ai_pro'/);
      expect(ADAPTER).toMatch(/export const AI_PRO_PRICE : number = 1000/);
      // 事由键由 SKU 拼接派生，不得另写死字符串（后端 RedeemShop 是 'redeem_' + sku）
      expect(ADAPTER).toMatch(/export const AI_PRO_REDEEM_REASON : string = 'redeem_' \+ AI_PRO_SKU/);
    });

    it('已拥有的判据是流水三元组**全等**，不是子串命中', () => {
      const fn = ADAPTER.slice(ADAPTER.indexOf('export function isAiProRedeemEntry'));
      expect(fn).toMatch(/reason == AI_PRO_REDEEM_REASON && refType == AI_PRO_REF_TYPE && refId == AI_PRO_SKU/);
    });

    it('扫描口径有界：只扫支出方向、单页取满额、页数有上限', () => {
      expect(ADAPTER).toMatch(/getPointsLedgerApi\(page, AI_PRO_SCAN_PAGE_SIZE, 'out'\)/);
      expect(ADAPTER).toMatch(/export const AI_PRO_SCAN_PAGE_SIZE : number = 100/);
      expect(ADAPTER).toMatch(/export const AI_PRO_SCAN_MAX_PAGES : number = 10/);
      // 超限仍未命中 ⇒ 结论不可靠标记（页面据此保持 CTA 可点，见 ③）
      expect(ADAPTER).toMatch(/proDegraded\.value = true/);
    });

    it('读失败 fail-closed：未读到即未解锁，不许 fail-open 假绿', () => {
      expect(ADAPTER).toMatch(/const proUnlocked = ref<boolean>\(false\)/);
      // 只切 loadAiProState 的函数体（redeemAiPro 里**允许**置 true —— 那是服务端应答之后）
      const fn = ADAPTER.slice(
        ADAPTER.indexOf('async function loadAiProState'),
        ADAPTER.indexOf('async function redeemAiPro')
      );
      expect(fn).toMatch(/proUnlocked\.value = unlocked/);
      expect(fn).not.toMatch(/proUnlocked\.value = true/);
    });
  });

  describe('② 兑换回路（服务端单管线，客户端不本地预扣）', () => {
    it('API 层走既有 redeem 端点，不新造机制', () => {
      expect(POINTS_API).toMatch(/export function redeemShopItemApi\(sku : string\)/);
      expect(POINTS_API).toMatch(/'\/points\/shop\/' \+ encodeURIComponent\(sku\) \+ '\/redeem'/);
    });

    it('适配器只在服务端应答之后改门状态（成功 + 已兑换两种归宿）', () => {
      const fn = ADAPTER.slice(ADAPTER.indexOf('async function redeemAiPro'));
      expect(fn).toMatch(/const res = await redeemShopItemApi\(AI_PRO_SKU\)/);
      expect(fn).toMatch(/proUnlocked\.value = true/);
      // 服务端答「已兑换」= 已拥有（自愈路径：扫描超限/历史过长的老用户不被锁在门外）
      expect(fn).toMatch(/msg\.indexOf\('已兑换'\) >= 0/);
    });

    it('横幅点开 sheet；sheet 的兑换/赚积分两条出口都接在页面上', () => {
      expect(PAGE).toMatch(/@click="openProSheet"/);
      expect(PAGE).toMatch(/<AiChatProSheet :visible="showProSheet"/);
      expect(PAGE).toMatch(/@redeem="onRedeemAiPro"/);
      expect(PAGE).toMatch(/@earn="onGoEarnPoints"/);
    });

    it('兑换成功后**原地刷新门状态**，无需重启（验收标准第 2 条）', () => {
      const fn = PAGE.slice(PAGE.indexOf('async function onRedeemAiPro'));
      expect(fn).toMatch(/if \(res\.ok\)/);
      expect(fn).toMatch(/await loadAiProState\(\)/);
    });

    it('进页面读门状态；回页时未解锁才重读（已解锁为粘性，不重复扫流水）', () => {
      expect(PAGE).toMatch(/loadAiProState\(\)/);
      const from = PAGE.indexOf('onShow(() => {');
      const onShow = PAGE.slice(from, PAGE.indexOf('</script>', from));
      // #1024：重读这一步改经 refreshProState（读完门状态顺带做「存量来源回收」），
      // 「未解锁才重读」的粘性语义不变 —— 已解锁分支只置 settled，不得再扫流水
      expect(onShow).toMatch(/if \(!proUnlocked\.value\) \{\s*\n\s*refreshProState\(\)/);
      expect(onShow).toMatch(/proStateSettled\.value = true/);
      expect(onShow).not.toMatch(/else\s*\{[^}]*refreshProState\(\)/);
    });
  });

  describe('③ 失败口径（不静默）', () => {
    it('兑换失败给可读文案（服务端原文优先，空则兜底）', () => {
      const fn = PAGE.slice(PAGE.indexOf('async function onRedeemAiPro'));
      expect(fn).toMatch(/res\.message\.length > 0 \? res\.message : '兑换失败，请稍后再试'/);
    });

    it('积分不足 ⇒ sheet 的 CTA 变成「去赚积分」并跳任务中心（不空转）', () => {
      expect(SHEET).toMatch(/积分不足 · 去赚积分/);
      expect(SHEET).toMatch(/emit\('earn'\)/);
      expect(PAGE).toMatch(/uni\.navigateTo\(\{ url: '\/pages\/points\/task-center' \}\)/);
    });

    it('余额不够时不做本地预扣、也不私自改门状态', () => {
      const fn = SHEET.slice(SHEET.indexOf('const enough = computed'));
      expect(fn).toMatch(/props\.balance >= props\.price/);
      expect(SHEET).not.toMatch(/setStorageSync|removeStorageSync/);
    });

    it('上/下架只能由服务端应答体现：客户端不硬编码在售判断', () => {
      // 下架 = redeem 返回 400「商品不存在或已下架」，适配器原样透出（不吞不猜）
      const fn = ADAPTER.slice(ADAPTER.indexOf('async function redeemAiPro'));
      expect(fn).toMatch(/return \{ ok: false, unlocked: proUnlocked\.value, message: msg/);
    });
  });

  describe('④ 两态横幅与能力清单（横幅按设计用实色；#937 口径）', () => {
    it('未解锁 = 橙 / 已解锁 = 绿，均为实色 background-color', () => {
      expect(PAGE).toMatch(/\.pro-banner\.pro-banner-unlocked \{/);
      expect(PAGE).toMatch(/background-color: #fff3e0/);
      expect(PAGE).toMatch(/background-color: #e8f8ee/);
      // 横幅**按设计**不用渐变（两态用实色更干净），不是「渐变画不出来」——
      // 2026-09-13 真机实测已证伪后者：合规语法（角度/关键字 + 2 颜色值、无百分比）
      // 是画得出来的，见 utils/gradientSyntaxContract.test.js（#937）。
      const from = PAGE.indexOf('.pro-banner {');
      const style = PAGE.slice(from, PAGE.indexOf('.pro-banner-go {', from));
      expect(style).not.toMatch(/gradient/);
    });

    it('两态文案与 CTA 由 proUnlocked 派生（不是两个写死的分支模板）', () => {
      expect(PAGE).toMatch(/const proBannerTitle = computed<string>/);
      expect(PAGE).toMatch(/'专业版 · 已解锁' : '解锁专业版'/);
      expect(PAGE).toMatch(/'已解锁' : '查看权益'/);
    });

    it('能力清单 = 门集合三项（选择模型 / 自定义模型 / 发送图片）+ 所需积分', () => {
      expect(SHEET).toMatch(/title: '选择模型'/);
      expect(SHEET).toMatch(/title: '自定义模型'/);
      expect(SHEET).toMatch(/title: '发送图片'/);
      expect(SHEET).toMatch(/所需积分/);
      expect(SHEET).toMatch(/\{\{ price \}\}/);
    });

    it('能力门四处共用同一个 proUnlocked（不是各自另立判据）', () => {
      expect(PAGE).toMatch(/:show-model-bar="proUnlocked"/);
      expect(PAGE).toMatch(/:show-image-button="proUnlocked"/);
      expect(PAGE).toMatch(/:model-readonly="!proUnlocked"/);
      expect(PAGE).toMatch(/m\.key != 'custom-models'/);
      expect(PAGE).toMatch(/:unlocked="proUnlocked"/);
    });
  });
});
