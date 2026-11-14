/**
 * 启动闪屏态**时序** · 行为级测试（#2535，行为腿）
 *
 * 票面 Q1 拍板：「首帧品牌图 → 1.5s 后淡出到着陆页内容」，且「已登录仍直接 reLaunch
 * dashboard、不闪」。这两条都**只有真跑计时器才看得见** —— 读源码只能证明「有个
 * setTimeout(…, 1500)」，证明不了「1499ms 时还满不透明」「淡出走完真的摘层」
 * 「中途 reLaunch 走掉后计时器不再往回写状态」。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`（把 `.uts` 去掉类型当 JS 真跑）。注入替身只有
 * `ref`（本仓没有 `vue` npm 包，composable 只用 `.value` 读写）；**时序逻辑与常量不替身**，
 * 全部来自 `pages/index/composables/useLandingSplash.uts` 的真导出（手抄镜像会让「真源改了
 * 而镜子没改」静默变绿）。计时器走 jest 假计时器：composable 里的 `setTimeout` /
 * `setInterval` 解析到 Node 全局，假计时器一装就接管。
 *
 * 手写分步淡出而不是 CSS transition 的原因写在真源文件头；本文件顺带把那条决定钉成事实：
 * `splashOpacity` 在淡出期**单调下降**且中间点确实落在开区间内（写了 transition 的话这里恒为 1）。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const COMPOSABLE = path.join(__dirname, '..', 'pages', 'index', 'composables', 'useLandingSplash.uts');
const PAGE = path.join(__dirname, '..', 'pages', 'index', 'index.uvue');

/** 最小 ref：composable 只用到 `.value` 读写 */
function miniRef(initial) {
  return { value: initial };
}

/** 每次调用都是全新模块实例（模块级状态互不串） */
function loadSplash() {
  return loadUts(COMPOSABLE, { ref: miniRef });
}

beforeEach(() => {
  jest.useFakeTimers();
});

afterEach(() => {
  jest.useRealTimers();
});

describe('useLandingSplash 时序（#2535 Q1：1.5s 保持 + 淡出）', () => {
  it('未登录首帧即上屏：初值 visible=true / opacity=1（不先闪一下着陆页）', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    expect(splash.splashVisible.value).toBe(true);
    expect(splash.splashOpacity.value).toBe(1);
  });

  it('保持期整段满不透明（1499ms 仍是 1），到点才开始降', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();

    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS - 1);
    expect(splash.splashOpacity.value).toBe(1);
    expect(splash.splashVisible.value).toBe(true);

    // 跨过保持期边界：至少降过一次，且还没摘层
    jest.advanceTimersByTime(mod.SPLASH_FADE_STEP_MS * 2);
    expect(splash.splashOpacity.value).toBeLessThan(1);
    expect(splash.splashOpacity.value).toBeGreaterThan(0);
    expect(splash.splashVisible.value).toBe(true);
  });

  it('淡出走完即摘层：opacity=0 且 visible=false（此时露出的正是着陆页内容）', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();

    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS + mod.SPLASH_FADE_MS + 100);
    expect(splash.splashOpacity.value).toBe(0);
    expect(splash.splashVisible.value).toBe(false);
  });

  it('淡出是分步的（不是一次性跳变）：中间至少有一个 (0,1) 开区间取值', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS);

    const seen = [];
    const steps = Math.ceil(mod.SPLASH_FADE_MS / mod.SPLASH_FADE_STEP_MS);
    for (let i = 0; i < steps; i += 1) {
      jest.advanceTimersByTime(mod.SPLASH_FADE_STEP_MS);
      seen.push(splash.splashOpacity.value);
    }
    const interior = seen.filter((v) => v > 0 && v < 1);
    expect(interior.length).toBeGreaterThan(0);
    // 单调不增：淡出期间不透明度不许回升
    for (let i = 1; i < seen.length; i += 1) {
      expect(seen[i]).toBeLessThanOrEqual(seen[i - 1] + 1e-9);
    }
    expect(seen[seen.length - 1]).toBe(0);
  });

  it('dispose() 掐断两个计时器：页面被 reLaunch 走之后不再往回写状态', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();

    // 走到淡出中途再撤
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS + mod.SPLASH_FADE_STEP_MS * 3);
    const frozen = splash.splashOpacity.value;
    expect(frozen).toBeGreaterThan(0);
    expect(frozen).toBeLessThan(1);

    splash.dispose();
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS + mod.SPLASH_FADE_MS + 5000);

    expect(splash.splashOpacity.value).toBe(frozen);
    expect(splash.splashVisible.value).toBe(true);
  });

  it('dispose() 也掐得断保持期计时器（还没开始淡出就撤）', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS - 100);
    splash.dispose();
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS + mod.SPLASH_FADE_MS + 5000);
    expect(splash.splashOpacity.value).toBe(1);
    expect(splash.splashVisible.value).toBe(true);
  });

  it('重复 start() 幂等：复位到满不透明，并从新的 1.5s 重新计时', () => {
    const mod = loadSplash();
    const splash = mod.useLandingSplash();
    splash.start();
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS + mod.SPLASH_FADE_STEP_MS * 2);
    expect(splash.splashOpacity.value).toBeLessThan(1);

    splash.start();
    expect(splash.splashOpacity.value).toBe(1);
    expect(splash.splashVisible.value).toBe(true);

    // 若上一轮计时器没被掐断，此刻会继续往下掉；这里必须仍是满的
    jest.advanceTimersByTime(mod.SPLASH_HOLD_MS - 1);
    expect(splash.splashOpacity.value).toBe(1);
  });

  it('常量就是票面拍板的 1.5s（改时长必须同时改票面与本断言）', () => {
    const mod = loadSplash();
    expect(mod.SPLASH_HOLD_MS).toBe(1500);
    expect(mod.SPLASH_FADE_MS).toBeGreaterThan(0);
    expect(mod.SPLASH_FADE_MS).toBeLessThanOrEqual(1000);
  });
});

describe('接线（#2535：闪屏层叠在着陆页内，不建路由）', () => {
  const src = readText(PAGE);

  it('只起在未登录分支：已登录那条 reLaunch 仍是无条件落地、且不触发闪屏', () => {
    // 守卫块内仍是「restore → reLaunch」原样；闪屏只在 else 分支起（已登录页面不会被绘制）
    expect(src).toMatch(/if \(auth\.isLoggedIn\.value\) \{[\s\S]*?reLaunch\(\{[\s\S]*?\/pages\/dashboard\/dashboard/);
    expect(src).toMatch(/\} else \{[\s\S]*?startSplash\(\)/);
  });

  it('计时器挂 onUnload：页面卸载必须 dispose（反模式：裸 setTimeout 不管生命周期）', () => {
    expect(src).toMatch(/onUnload\(\(\)\s*:\s*void\s*=>\s*\{[\s\S]*?disposeSplash\(\)/);
  });

  it('闪屏层在模板里、不在 N06 头部段内（status-bar → hero 之间必须仍零可点元素）', () => {
    const start = src.indexOf('class="status-bar"');
    const hero = src.indexOf('class="hero"');
    const splash = src.indexOf('class="splash"');
    expect(hero).toBeGreaterThan(start);
    expect(splash).toBeGreaterThan(hero);
  });

  it('闪屏不新增任何导航：本页仍恰一条 reLaunch（唯一已登录冷启动落地路由）', () => {
    expect((src.match(/uni\.reLaunch\(/g) || []).length).toBe(1);
  });
});
