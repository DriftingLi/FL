/**
 * 选证引导态**双态切换** · 行为级测试（#2535，行为腿）
 *
 * 票面 Q2 拍板：「首屏 = 选证引导态；点击/上滑进主态；右上角 X = 跳过选证直接进首页」。
 * 前两条只有真跑手势逻辑才看得见 —— 读源码能证明「有个 onGuideTouchEnd」，证明不了
 * 「上滑 60px 才进、59px 不进」「下滑不动」「进主态是单向的」。第三条是导航归属页面，
 * 本文件用源码接线断言兜（composable 只管状态，不管 `uni.reLaunch`）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`（把 `.uts` 去掉类型当 JS 真跑）。注入替身只有 `ref`
 * （本仓没有 `vue` npm 包，composable 只用 `.value` 读写）；阈值常量不替身，取自真源导出。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const COMPOSABLE = path.join(__dirname, '..', 'pages', 'guide', 'composables', 'useChooseCertGuide.uts');
const PAGE = path.join(__dirname, '..', 'pages', 'guide', 'choose-cert.uvue');

/** 最小 ref：composable 只用到 `.value` 读写 */
function miniRef(initial) {
  return { value: initial };
}

/** 每次调用都是全新模块实例（模块级状态互不串） */
function loadGuide() {
  return loadUts(COMPOSABLE, { ref: miniRef });
}

describe('useChooseCertGuide 双态（#2535 Q2：点击/上滑进主态）', () => {
  it('进页第一帧就是引导态：初值 guideVisible=true（新用户注册成功、我的页入口都走这条）', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    expect(guide.guideVisible.value).toBe(true);
  });

  it('点击（enterMain）即进主态，且是单向的：再点不会回到引导态', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    guide.enterMain();
    expect(guide.guideVisible.value).toBe(false);
    guide.enterMain();
    expect(guide.guideVisible.value).toBe(false);
  });

  it('上滑刚好达到阈值就进主态（边界取等：位移 ≥ 阈值）', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    guide.onGuideTouchStart(400);
    guide.onGuideTouchEnd(400 - mod.GUIDE_SWIPE_UP_PX);
    expect(guide.guideVisible.value).toBe(false);
  });

  it('上滑不足阈值不进主态（避免手指微抖把引导态划走）', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    guide.onGuideTouchStart(400);
    guide.onGuideTouchEnd(400 - mod.GUIDE_SWIPE_UP_PX + 1);
    expect(guide.guideVisible.value).toBe(true);
  });

  it('下滑不动，原地不动也不动（只有向上位移才认）', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    guide.onGuideTouchStart(400);
    guide.onGuideTouchEnd(600);
    expect(guide.guideVisible.value).toBe(true);

    guide.onGuideTouchStart(400);
    guide.onGuideTouchEnd(400);
    expect(guide.guideVisible.value).toBe(true);
  });

  it('每次手势独立计位移：上一次的起点不会漏到下一次', () => {
    const mod = loadGuide();
    const guide = mod.useChooseCertGuide();
    // 第一次上滑不足阈值
    guide.onGuideTouchStart(800);
    guide.onGuideTouchEnd(780);
    expect(guide.guideVisible.value).toBe(true);
    // 第二次从新起点开始：若起点没复位（仍是 800），这里 700 会被判成 100px 上滑
    guide.onGuideTouchStart(700);
    guide.onGuideTouchEnd(690);
    expect(guide.guideVisible.value).toBe(true);
  });

  it('阈值就是 60px（改判定手感必须同时改本断言与票面）', () => {
    const mod = loadGuide();
    expect(mod.GUIDE_SWIPE_UP_PX).toBe(60);
  });
});

describe('接线（#2535：引导层叠在选证页内，不建路由）', () => {
  const src = readText(PAGE);

  it('引导层由 guideVisible 控制显隐，且点击整层进主态', () => {
    expect(src).toContain('v-if="guideVisible"');
    expect(src).toMatch(/class="guide"[^>]*@click="enterMain"/);
  });

  it('上滑手势接到 composable（页面只取触点纵坐标，判定归 composable）', () => {
    expect(src).toMatch(/@touchstart="handleGuideTouchStart"/);
    expect(src).toMatch(/@touchend="handleGuideTouchEnd"/);
    expect(src).toMatch(/onGuideTouchStart\(e\.touches\[0\]\.clientY\)/);
    expect(src).toMatch(/onGuideTouchEnd\(e\.changedTouches\[0\]\.clientY\)/);
  });

  it('X 跳过选证：stop 冒泡后直接 reLaunch 首页，且不调切换接口、不写凭证缓存', () => {
    expect(src).toMatch(/@click\.stop="onSkipGuide"/);
    const skip = src.slice(src.indexOf('function onSkipGuide'));
    const body = skip.slice(0, 240);
    expect(body).toContain("uni.reLaunch({ url: '/pages/dashboard/dashboard' })");
    expect(body).not.toContain('switchCredentialApi');
    expect(body).not.toContain('writeCurrentCredentialCache');
  });

  it('引导层是本页末子元素（绝对定位盖住页头与列表，不参与上面那条 flex 列的高度分配）', () => {
    const bottomBar = src.indexOf('class="bottom-bar"');
    const guide = src.indexOf('class="guide"');
    expect(bottomBar).toBeGreaterThan(0);
    expect(guide).toBeGreaterThan(bottomBar);
  });

  it('引导态是页内双态：两处 reLaunch 各司其职（未登录防护 / X 跳过落首页）', () => {
    // #2907：完成与返回都改走 goBack()（栈 >1 返回、只有本页才 reLaunch），页面里直接 reLaunch 的只剩这两处
    expect((src.match(/uni\.reLaunch\(/g) || []).length).toBe(2);
  });
});

describe('首页入口进来是「改证件」，不再被引导态挡住（#2907）', () => {
  const src = readText(PAGE);

  it('onLoad 读 from 查询参数，等于 dashboard 时直接进主态', () => {
    // Kotlin Function1<OnLoadOptions, Unit>：零参 onLoad 会在编译期 error17
    expect(src).toMatch(/onLoad\(\(options : OnLoadOptions\) => \{/);
    expect(src).toMatch(/const rawFrom = options\['from'\]/);
    expect(src).toMatch(/rawFrom\.toString\(\) == CHOOSE_CERT_FROM_DASHBOARD/);
    const body = src.slice(src.indexOf("onLoad((options : OnLoadOptions)"));
    expect(body.slice(0, 400)).toMatch(/enterMain\(\)/);
  });

  it('from 取值来自 composable 常量（页面不抄字面量 dashboard）', () => {
    expect(src).toContain('CHOOSE_CERT_FROM_DASHBOARD');
    expect(src).not.toMatch(/== 'dashboard'/);
  });

  it('完成成功与页头返回共用同一出口 goBack()（不为整页入口另造一条导航）', () => {
    expect(src).toContain("import { goBack } from '../../utils/navigation'");
    const confirm = src.slice(src.indexOf('function onConfirmTap'));
    expect(confirm.slice(0, 400)).toContain('goBack()');
    const back = src.slice(src.indexOf('function toback'));
    expect(back.slice(0, 200)).toContain('goBack()');
  });

  it('预选与提交归 composable：页面自己不调切换接口、不写凭证缓存', () => {
    expect(src).toContain('useChooseCertSelection()');
    expect(src).not.toContain('switchCredentialApi');
    expect(src).not.toContain('writeCurrentCredentialCache');
    expect(src).not.toContain('getCurrentCredentialApi');
  });
});
