/**
 * #1478 着陆页行为测试 —— 真跑「着陆页 composable + 共享成功出口 + 共享探测件」，
 * **数请求层与出口被调用次数**（不是锁字面量）
 *
 * 形态仿 `utils/authPreRequestValidationBehavior.test.js`：那个文件证明「客户端校验挡在请求之前」
 * 靠的是数 `auth.loginByCode` 被打到几次；这里用同一条缝证明 #1478 的三条行为判据。
 *
 * 本文件钉住的不变量（票面 Desired behavior + AC 的行为那一半）：
 *   I1 **未勾选协议 ⇒ 请求层零调用**（AC 3）。`uni.showToast` 出现不等于拦住：只有
 *      `loginByWechat` 计数为 0、`showLoading` 计数为 0 才算拦住。
 *   I2 **两入口一条成功出口**（AC 4）。着陆页成功后把 `isNew` 原样交给**真的**
 *      `utils/loginOutlet.afterLoginSuccess`；新用户必须排程 `choose-cert`、老用户 `dashboard`，
 *      且**延后**（执行当场不得出现 reLaunch）。
 *   I3 **失败不得静默**（AC 5）。`errorHint` 必须被写成人类可读原因（含 Error.message 透传、
 *      空 message 走兜底文案两条路径），`hideLoading` 必须发生，成功链路不得被触发。
 *   I4 **探测件是能力判断且保守回退**（AC 2 的运行期那一半；**2026-10-03 由复合判据收敛为端别单半**）：
 *      `wechatAvailable = 端别（mp-weixin）`。能力半 provider 探测**已停用**——#1487 ①a 真机实测：
 *      未配微信 SDK 的 Android 上 `getProviderSync({service:'oauth'})` 恒返回 `["weixin"]`（工程 manifest
 *      无 App 侧 oauth、uni_modules 无微信模块）⇒ 假阳性，App 端渲染出「微信一键登录」且点击即失败。
 *      函数与异常塌陷**保留待 #1482 接回**（接回前 F8 必须能抓到「误接回判据」这个形态）。
 *      保守回退不变：非 mp-weixin ⇒ 判不可用并给原因，异常**不得冒出**探测件。
 *   I5 **探测判断不在着陆页 composable 里**（判据 4「探测件被两页共享」的反面）。
 *      着陆页的 `uni` fake 里 `getSystemInfoSync` 与 `getProviderSync` 都抛错：
 *      着陆页自己抄一份端别判断或 provider 判断即红。
 *   I6 **协议详情提示走全仓单点**（第 1 轮评审 F2 返工：那句「详情页建设中」曾是第三副本）：
 *      着陆页的 `showAgreement` 只把**协议名**交给真的 `utils/agreementNotice.showAgreementNotice`，
 *      自己不再拼那句文案。注入自检成对给：C4 改单点的后缀 ⇒ 着陆页 toast 必须跟着变；
 *      C5 换成着陆页内联一份副本（评审前的形态）⇒ 单点变了而它不变，判据必须能区分这两种。
 *
 * 本文件**够不到**的东西（别把它当成能证明）：
 *   - `.uvue` 的模板与样式在 node 里跑不了 ⇒ 页面壳层、`v-if="wechatAvailable"` 的渲染分叉、
 *     「App 端不露出『一键』二字」都不在本文件执行面内（接线面在 `landingLoginContract`，
 *     渲染面归维护者真机门 ①）。
 *   - AC 1 的已登录冷启动路由写在 `pages/index/index.uvue` 的 `onLoad` 里，同样是 `.uvue` 壳层
 *     ⇒ 本文件跑不了，只有接线锁（且那条判据**禁止**被条件化，见 landingLoginContract AC 1 组）。
 *
 * 「必红」一侧由 F 组的注入变异给（每条都把真源改成一种具体的历史/事故形态，写进临时目录，
 * 真源文件一个字节不动）；「真源必不红」由 A-E 组本身给。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LANDING_UTS = path.join(ROOT, 'pages', 'index', 'composables', 'useLandingLogin.uts');
const OUTLET_UTS = path.join(ROOT, 'utils', 'loginOutlet.uts');
const PROBE_UTS = path.join(ROOT, 'composables', 'useLoginProviders.uts');
const NOTICE_UTS = path.join(ROOT, 'utils', 'agreementNotice.uts');

const LANDING_REL = path.join('pages', 'index', 'composables', 'useLandingLogin.uts');
const OUTLET_REL = path.join('utils', 'loginOutlet.uts');
const PROBE_REL = path.join('composables', 'useLoginProviders.uts');
const NOTICE_REL = path.join('utils', 'agreementNotice.uts');

/** 与登录页同一个字面量（口径一致性由 landingLoginContract 锁文本，这里锁它真被 toast 出来） */
const MSG_AGREE = '请先同意用户协议和隐私政策';
const MSG_NEW_USER = '已为您自动注册账号';
const MSG_OK = '登录成功';
const MSG_FALLBACK = '登录失败，请重试';
const CHOOSE_CERT = '/pages/guide/choose-cert';
const DASHBOARD = '/pages/dashboard/dashboard';
const NO_SUPPORT = '当前客户端不支持微信登录';
/** 入口展示但链路未接通时的单点文案（真源 = `composables/useLoginProviders.WECHAT_NOT_WIRED_NOTICE`） */
const NOT_WIRED = '微信登录暂未开通';
/** 协议详情提示的后缀：F2 返工后全仓只住 `utils/agreementNotice.uts` 一份 */
const NOTICE_SUFFIX = '详情页建设中';

/** vue 的最小响应式容器（`vue` 不在 node 解析射程内，本仓行为测试一律注入） */
const vueShim = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/**
 * `uni` fake：记 toast/loading/导航/重launch 的全部实参。
 * 两个探测入口（`getSystemInfoSync` / `getProviderSync`）**默认都抛** —— 着陆页 composable
 * 不该碰平台/provider 判断（那是共享探测件的活，I5）。探测件那边由 `probeApp` 显式喂桩。
 * @param o.systemInfo     喂给 `getSystemInfoSync()` 的返回值（探测件的端别判据）
 * @param o.providerResult 喂给 `getProviderSync()` 的返回值（探测件的能力判据）
 * @param o.providerNull   让 `getProviderSync()` 回 null（模拟返回面为空）
 * @param o.providerThrows 让 `getProviderSync()` 抛错（模拟 SDK 缺失/探测失败）
 * @param o.noProviderApi 干脆不给 `uni` 装 `getProviderSync`（模拟该端没有这个 API）
 */
function makeUni(opts) {
  const o = opts || {};
  const log = { toast: [], loading: [], hideLoading: 0, navigateTo: [], reLaunch: [], redirectTo: [], getProvider: 0, getSystemInfo: 0 };
  const uni = {
    showToast: (a) => { log.toast.push(a); },
    showLoading: (a) => { log.loading.push(a); },
    hideLoading: () => { log.hideLoading += 1; },
    navigateTo: (a) => { log.navigateTo.push(a.url); },
    redirectTo: (a) => { log.redirectTo.push(a.url); },
    reLaunch: (a) => { log.reLaunch.push(a.url); },
    getSystemInfoSync: () => {
      log.getSystemInfo += 1;
      if (o.systemInfo) return o.systemInfo;
      throw new Error('着陆页 composable 不得自行探测平台（共享探测件的活，#1478 判据 4）');
    },
    getProviderSync: () => {
      log.getProvider += 1;
      if (o.providerThrows) throw new Error('provider 探测失败（注入桩故意抛）');
      if (o.providerNull) return null;
      if (o.providerResult) return o.providerResult;
      throw new Error('着陆页 composable 不得自行探测 provider（共享探测件的活，#1478 判据 4）');
    },
  };
  if (o.noProviderApi) delete uni.getProviderSync;
  return { uni, log };
}

/** 捕获式定时器：把「延后跳转」的延时与回调都变成可断言数据 */
function captureTimer() {
  const pending = [];
  const setTimeout = (fn, ms) => { pending.push({ fn, ms }); return pending.length; };
  const flush = () => { const q = pending.splice(0); q.forEach((t) => t.fn()); return q.map((t) => t.ms); };
  return { pending, setTimeout, flush };
}

const titles = (log) => log.toast.map((t) => t.title);

/** 把变异后的源码写进临时目录再交给 harness —— 真源文件一个字节都不动 */
function loadFromSource(source, fileName, bindings) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1478-'));
  const file = path.join(dir, fileName);
  fs.writeFileSync(file, source);
  try {
    return loadUts(file, bindings);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** 单点字面量替换；锚点不唯一/不存在即抛（否则变异静默落空 ⇒ 「必红」用例变成假绿） */
function mutate(rel, from, to) {
  const src = readText(path.join(ROOT, rel));
  const parts = src.split(from);
  if (parts.length !== 2) {
    throw new Error(`变异锚点必须唯一命中：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  }
  return parts.join(to);
}

const landingSrc = () => readText(LANDING_UTS);
const outletSrc = () => readText(OUTLET_UTS);
const probeSrc = () => readText(PROBE_UTS);
const noticeSrc = () => readText(NOTICE_UTS);

/**
 * 起一个**真的**着陆页 composable。`calls.loginByWechat` 就是 I1 的计数器；
 * `outlet: true` 时把**真的** `utils/loginOutlet` 接进去（用于 I2 的端到端）。
 * 协议提示默认接**真的** `utils/agreementNotice`（I6 的端到端）；`notice` 给桩、
 * `noticeSource` 给变异后的单点源码 —— 两条都走同一个注入位，装配方式与出口件一致。
 */
function landingApp(opts) {
  const o = opts || {};
  const { uni, log } = makeUni();
  const timer = captureTimer();
  const calls = { loginByWechat: 0, afterLoginSuccess: [] };
  const outletBindings = { uni, setTimeout: timer.setTimeout };
  const outlet = loadUts(OUTLET_UTS, outletBindings);
  const notice = o.notice
    ? o.notice
    : (o.noticeSource == null
      ? loadUts(NOTICE_UTS, { uni }).showAgreementNotice
      : loadFromSource(o.noticeSource, 'agreementNotice.uts', { uni }).showAgreementNotice);
  const bindings = {
    ...vueShim(),
    uni,
    showAgreementNotice: notice,
    // #1487 ①a 真机口径：未接通提示文案是探测件导出的单点，注入位与协议提示同形
    WECHAT_NOT_WIRED_NOTICE: NOT_WIRED,
    useAuthStore: () => ({
      loginByWechat: () => {
        calls.loginByWechat += 1;
        return o.wechatImpl ? o.wechatImpl() : Promise.resolve({ isNew: false });
      },
    }),
    // 出口二选一：桩（数实参）或真件（端到端数跳转）。同一条注入位，两种装配。
    afterLoginSuccess: o.outlet
      ? (isNew) => { outlet.afterLoginSuccess(isNew); }
      : (isNew) => { calls.afterLoginSuccess.push(isNew); },
  };
  const src = o.source == null ? null : o.source;
  const mod = src == null ? loadUts(LANDING_UTS, bindings) : loadFromSource(src, 'useLandingLogin.uts', bindings);
  // 接通面默认喂 true（= 真跑原链路）；A2 组显式喂 false 走「样式位」路径。
  // 缺省 true 是刻意的：老判据全部测的是「接通后」的行为，喂错默认值会让它们悄悄变成假绿
  return { face: mod.useLandingLogin(o.loginReady !== false), log, calls, timer };
}

/** 起一个**真的**成功出口（`utils/loginOutlet.uts`），定时器与跳转都可断言 */
function outletApp(opts) {
  const o = opts || {};
  const { uni, log } = makeUni();
  const timer = captureTimer();
  const bindings = { uni, setTimeout: timer.setTimeout };
  const mod = o.source == null
    ? loadUts(OUTLET_UTS, bindings)
    : loadFromSource(o.source, 'loginOutlet.uts', bindings);
  return { afterLoginSuccess: mod.afterLoginSuccess, log, timer };
}

/** 起一个**真的**共享探测件，喂指定的端别值 + provider 探测面（缺省 = provider 探测抛错） */
function probeApp(uniPlatform, probe) {
  const o = probe || {};
  const { uni, log } = makeUni({
    systemInfo: { uniPlatform: uniPlatform },
    providerResult: o.providerResult,
    providerNull: o.providerNull,
    providerThrows: o.providerThrows,
    noProviderApi: o.noProviderApi,
  });
  const bindings = { ...vueShim(), uni };
  const mod = o.source == null ? loadUts(PROBE_UTS, bindings) : loadFromSource(o.source, 'useLoginProviders.uts', bindings);
  return { uni, log, face: mod.useLoginProviders() };
}

beforeAll(() => {
  // 被测物在失败路径上会 console.error（真实行为，不是缺陷）：静音，不改判据
  jest.spyOn(console, 'error').mockImplementation(() => {});
});
afterAll(() => {
  jest.restoreAllMocks();
});

// ── I1：未勾选协议拦在任何请求之前（AC 3） ──────────────────────────────────

describe('A. 协议门槛：未勾选 ⇒ 请求层零调用（真跑 useLandingLogin）', () => {
  test('A1 未勾选：toast 是登录页那句原文，loginByWechat / showLoading 均为 0 次，成功链路未触发', async () => {
    const app = landingApp();
    await app.face.onWechatLogin();
    expect(titles(app.log)).toEqual([MSG_AGREE]);
    expect(app.calls.loginByWechat).toBe(0);
    expect(app.log.loading.length).toBe(0);
    expect(app.calls.afterLoginSuccess).toEqual([]);
    expect(app.log.reLaunch).toEqual([]);
    expect(app.face.loading.value).toBe(false);
  });

  test('A2 勾选后：请求恰一次、门槛文案不再出现、成功后把 isNew 交给出口', async () => {
    const app = landingApp();
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
    expect(titles(app.log)).not.toContain(MSG_AGREE);
    expect(app.log.loading).toEqual([{ title: '登录中...', mask: true }]);
    expect(app.calls.afterLoginSuccess).toEqual([false]);
  });

  test('A3 登录即注册的语义只属于微信通道：isNew=true 必须原样透传给出口（不得在此塌成 false）', async () => {
    const app = landingApp({ wechatImpl: () => Promise.resolve({ isNew: true }) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.afterLoginSuccess).toEqual([true]);
  });

  test('A4 出口回 null 时按老用户处理（不得把 null 当新用户，也不得抛出去）', async () => {
    const app = landingApp({ wechatImpl: () => Promise.resolve(null) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.afterLoginSuccess).toEqual([false]);
  });

  test('A5 toggleAgree 翻转的是 agreed 本身（共享协议件走 emit 翻父页 ref 的口径）', async () => {
    const app = landingApp();
    expect(app.face.agreed.value).toBe(false);
    app.face.toggleAgree();
    expect(app.face.agreed.value).toBe(true);
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
    app.face.toggleAgree();
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1); // 第二次被门槛挡住，计数不动
  });

  test('A6 in-flight 保护：请求在飞时二次点击不再发第二条（门槛之后、请求之前）', async () => {
    const app = landingApp();
    app.face.agreed.value = true;
    app.face.loading.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(0);
    expect(app.log.loading.length).toBe(0);
  });

  test('I5 反Duplication：着陆页 composable 不自行探测平台/provider（碰两个探测入口都抛）', () => {
    expect(() => landingApp().face.goOtherLogin()).not.toThrow();
    expect(landingSrc()).not.toContain('getSystemInfoSync');
    expect(landingSrc()).not.toContain('getProviderSync');
  });
});

// ── #1487 ①a 真机口径（2026-10-03）：移动端保留入口**样式**，点击只给友好提示 ──

describe('A2. 样式位：接通面为假时点击零请求、零副作用，只给单点提示', () => {
  test('A2-1 未接通 + 已勾选协议：toast 是单点文案原文，请求/loading/errorHint 全不受影响', async () => {
    const app = landingApp({ loginReady: false });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(titles(app.log)).toEqual([NOT_WIRED]);
    expect(app.calls.loginByWechat).toBe(0);
    expect(app.log.loading.length).toBe(0);
    expect(app.log.hideLoading).toBe(0);
    expect(app.face.errorHint.value).toBe('');
    expect(app.face.loading.value).toBe(false);
    expect(app.calls.afterLoginSuccess).toEqual([]);
  });

  test('A2-2 硬顺序不变：未勾选协议时先撞协议门槛（样式位不得把协议口径顶掉）', async () => {
    const app = landingApp({ loginReady: false });
    await app.face.onWechatLogin();
    expect(titles(app.log)).toEqual([MSG_AGREE]);
    expect(titles(app.log)).not.toContain(NOT_WIRED);
    expect(app.calls.loginByWechat).toBe(0);
  });

  test('A2-3 对照组：接通面为真时同一条路径照旧真发请求（样式位不是把整条链掐死）', async () => {
    const app = landingApp({ loginReady: true });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
    expect(titles(app.log)).not.toContain(NOT_WIRED);
  });
});

// ── I3：失败可见（AC 5） ────────────────────────────────────────────────────

describe('B. 失败可见：错误原因进 errorHint 与 toast，成功链路不触发', () => {
  test('B1 reject Error：message 透传进 errorHint 与 toast，hideLoading 发生，loading 复位', async () => {
    const app = landingApp({ wechatImpl: () => Promise.reject(new Error('微信授权已取消')) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.face.errorHint.value).toBe('微信授权已取消');
    expect(titles(app.log)).toEqual(['微信授权已取消']);
    expect(app.log.hideLoading).toBe(1);
    expect(app.face.loading.value).toBe(false);
    expect(app.calls.afterLoginSuccess).toEqual([]);
    expect(app.log.reLaunch).toEqual([]);
  });

  test('B2 message 为空的 Error：兜底文案顶上（失败原因不得为空 ⇒ 页面渲染位不会出现空串）', async () => {
    const app = landingApp({ wechatImpl: () => Promise.reject(new Error('')) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.face.errorHint.value).toBe(MSG_FALLBACK);
    expect(titles(app.log)).toEqual([MSG_FALLBACK]);
  });

  test('B3 非 Error 抛出物：同样落到兜底文案且不把异常抛到页面（catch 面覆盖 instanceof 的 else 支）', async () => {
    const app = landingApp({ wechatImpl: () => Promise.reject('raw-string-failure') });
    app.face.agreed.value = true;
    await expect(app.face.onWechatLogin()).resolves.toBeUndefined();
    expect(app.face.errorHint.value).toBe(MSG_FALLBACK);
    expect(app.log.hideLoading).toBe(1);
  });

  test('B4 每次点击先清空上一次的失败原因（成功路径不留旧错误）', async () => {
    let fail = true;
    const app = landingApp({ wechatImpl: () => (fail ? Promise.reject(new Error('网络异常')) : Promise.resolve({ isNew: false })) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.face.errorHint.value).toBe('网络异常');
    fail = false;
    await app.face.onWechatLogin();
    expect(app.face.errorHint.value).toBe('');
    expect(app.calls.loginByWechat).toBe(2);
  });
});

// ── 次级入口与协议详情（AC 2 的 App 端出口 + 本票不改的既有口径） ────────────

describe('C. 次级入口与协议详情：出口真实存在（导航实参照数）+ 提示走全仓单点（I6）', () => {
  test('C1 goOtherLogin 导航到独立登录页（该页降级为次级落点、未退役 #1483）', () => {
    const app = landingApp();
    app.face.goOtherLogin();
    expect(app.log.navigateTo).toEqual(['/pages/login/login']);
  });

  test('C2 goRegister 导航到注册页（与协议件 register 事件同一条落点）', () => {
    const app = landingApp();
    app.face.goRegister();
    expect(app.log.navigateTo).toEqual(['/pages/register/register']);
  });

  test('C3 协议详情行为本票不改（brief 判据 7 明写）：端到端仍是「<协议名>详情页建设中」', () => {
    const app = landingApp();
    app.face.showAgreement('用户协议');
    app.face.showAgreement('用户隐私');
    expect(titles(app.log)).toEqual(['用户协议详情页建设中', '用户隐私详情页建设中']);
  });

  test('C4 F2 返工的本体：改**单点**的后缀 ⇒ 着陆页 toast 跟着变（证明它真走那一个件）', () => {
    const reworded = mutate(NOTICE_REL, `const AGREEMENT_NOTICE_SUFFIX = '${NOTICE_SUFFIX}'`, "const AGREEMENT_NOTICE_SUFFIX = '详情待补充'");
    const app = landingApp({ noticeSource: reworded });
    app.face.showAgreement('用户协议');
    expect(titles(app.log)).toEqual(['用户协议详情待补充']);
  });

  test('C5 成对取证：把着陆页退回评审前的内联副本 ⇒ 单点变了它**不变**（C4 因此不是假绿）', () => {
    const reworded = mutate(NOTICE_REL, `const AGREEMENT_NOTICE_SUFFIX = '${NOTICE_SUFFIX}'`, "const AGREEMENT_NOTICE_SUFFIX = '详情待补充'");
    const inlined = mutate(LANDING_REL, '        showAgreementNotice(name)',
      `        uni.showToast({ title: name + '${NOTICE_SUFFIX}', icon: 'none' })`);
    const copied = landingApp({ source: inlined, noticeSource: reworded });
    copied.face.showAgreement('用户协议');
    expect(titles(copied.log)).toEqual([`用户协议${NOTICE_SUFFIX}`]); // 副本吃不到单点的新文案 = 三处会漂移的病根
    const wired = landingApp({ noticeSource: reworded });
    wired.face.showAgreement('用户协议');
    expect(titles(wired.log)).toEqual(['用户协议详情待补充']); // 同一份变异单点，真源接线侧跟得上 ⇒ 差异来自接线
  });

  test('C6 着陆页只交协议名、不自己拼文案（转投实参照数：单点被调两次、名字原样过去）', () => {
    const seen = [];
    const app = landingApp({ notice: (name) => { seen.push(name); } });
    app.face.showAgreement('用户协议');
    app.face.showAgreement('用户隐私');
    expect(seen).toEqual(['用户协议', '用户隐私']);
    expect(titles(app.log)).toEqual([]);
  });
});

// ── I2：两入口一条成功出口（AC 4，端到端接**真**出口） ──────────────────────

describe('D. 成功出口端到端：着陆页 → 真 loginOutlet → 分叉落点', () => {
  test('D1 老用户：出口排程 500ms 后 reLaunch 到 dashboard，执行当场不得跳转', async () => {
    const app = landingApp({ outlet: true });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.log.reLaunch).toEqual([]); // 「延后」是真的延后
    expect(app.timer.pending.map((t) => t.ms)).toEqual([500]);
    expect(app.timer.flush()).toEqual([500]);
    expect(app.log.reLaunch).toEqual([DASHBOARD]);
    expect(app.log.toast).toEqual([{ title: MSG_OK, icon: 'success', duration: 1000 }]);
  });

  test('D2 新用户：出口排程 800ms 后 reLaunch 到 choose-cert（票面判据 4 硬约束）', async () => {
    const app = landingApp({ outlet: true, wechatImpl: () => Promise.resolve({ isNew: true }) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.log.toast).toEqual([{ title: MSG_NEW_USER, icon: 'none', duration: 1500 }]);
    expect(app.timer.pending.map((t) => t.ms)).toEqual([800]);
    app.timer.flush();
    expect(app.log.reLaunch).toEqual([CHOOSE_CERT]);
  });

  test('D3 着陆页不持有第二套落地逻辑：一次成功只产出**恰一条**排程 + **恰一条** reLaunch', async () => {
    for (const isNew of [true, false]) {
      const app = landingApp({ outlet: true, wechatImpl: () => Promise.resolve({ isNew: isNew }) });
      app.face.agreed.value = true;
      await app.face.onWechatLogin();
      expect(app.timer.pending.length).toBe(1);
      app.timer.flush();
      expect(app.log.reLaunch.length).toBe(1);
      expect([isNew, app.log.reLaunch]).toEqual([isNew, [isNew ? CHOOSE_CERT : DASHBOARD]]);
    }
  });

  test('D4 失败路径零落地：不排程、不跳转（出口只在成功分支被调）', async () => {
    const app = landingApp({ outlet: true, wechatImpl: () => Promise.reject(new Error('boom')) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.timer.pending).toEqual([]);
    expect(app.log.reLaunch).toEqual([]);
  });
});

// ── I4：共享探测件的能力判断与保守回退 ─────────────────────────────────────

describe('E. 探测件真跑：展示面（小程序 || App）与接通面（仅小程序）各自独立，能力半停用且失败面保守回退', () => {
  test('E1 mp-weixin ⇒ 两个面都为真、无原因文案（provider 探测抛错也不影响 —— 本期功能面不赌新 API）', () => {
    const app = probeApp('mp-weixin');
    expect(app.face.wechatAvailable.value).toBe(true);
    expect(app.face.wechatLoginReady.value).toBe(true);
    expect(app.face.wechatUnavailableReason.value).toBe('');
    expect(app.log.toast).toEqual([]);
  });

  test('E2 app ⇒ 展示面为真（2026-10-03 口径：移动端也展示该入口）、接通面为假（微信 SDK 未配置）', () => {
    const app = probeApp('app');
    expect(app.face.wechatAvailable.value).toBe(true);
    expect(app.face.wechatLoginReady.value).toBe(false);
    expect(app.face.wechatUnavailableReason.value).toBe('');
  });

  test('E3 未知/缺失端别值 ⇒ 两个面都为假 + 人类可读原因（保守回退：宁可不给入口，不可给了点不动）', () => {
    for (const platform of ['web', 'mp-toutiao', 'MP-WEIXIN', '', null, undefined]) {
      const app = probeApp(platform);
      expect([String(platform), app.face.wechatAvailable.value]).toEqual([String(platform), false]);
      expect([String(platform), app.face.wechatLoginReady.value]).toEqual([String(platform), false]);
      expect(app.face.wechatUnavailableReason.value).toBe(NO_SUPPORT);
    }
  });

  test('E4 能力半已停用（#1487 ①a 真机判红的成因）：app 端即便 provider 列出 weixin，也不得把接通面翻成真', () => {
    // 2026-10-03 真机实测：未配微信 SDK 的 Android 上 providerIds 恒为 ["weixin"]（工程 manifest
    // 无 App 侧 oauth、uni_modules 无微信模块）⇒ 接回判据即假阳性：点击真发请求然后失败
    const app = probeApp('app', { providerResult: { service: 'oauth', providerIds: ['weixin'] } });
    expect(app.face.wechatAvailable.value).toBe(true);
    expect(app.face.wechatLoginReady.value).toBe(false);
  });

  test('E5 能力半停用 ⇒ provider 探测的六种失败面都不参与任何一个面（App 展示面仍真、接通面仍假）', () => {
    const cases = [
      ['provider 抛错', { providerThrows: true }],
      ['该端没有这个 API', { noProviderApi: true }],
      ['oauth 未列 weixin', { providerResult: { providerIds: ['alipay'] } }],
      ['oauth 返回空数组', { providerResult: { providerIds: [] } }],
      ['返回面对不上', { providerResult: {} }],
      ['返回 null', { providerNull: true }],
    ];
    for (const [label, probe] of cases) {
      const app = probeApp('app', probe);
      expect([label, app.face.wechatAvailable.value, app.face.wechatLoginReady.value]).toEqual([label, true, false]);
      expect([label, app.log.getProvider]).toEqual([label, 0]); // 停用 = 一次都不调，而不是调了再丢掉
    }
  });

  test('E6 探测异常绝不冒到调用面：useLoginProviders() 在 provider 抛错时不 throw（渲染路径安全）', () => {
    expect(() => probeApp('app', { providerThrows: true })).not.toThrow();
    expect(() => probeApp('mp-weixin', { providerThrows: true })).not.toThrow();
  });

  test('E7 端别那半不许被 provider 那半赌掉：mp-weixin + provider 判不出 ⇒ 两个面仍都为真', () => {
    for (const probe of [{ providerThrows: true }, { noProviderApi: true }, { providerResult: { providerIds: [] } }]) {
      const app = probeApp('mp-weixin', probe);
      expect(app.face.wechatAvailable.value).toBe(true);
      expect(app.face.wechatLoginReady.value).toBe(true);
    }
  });

  test('E8 探测只在装配期取一次数（不在 computed 里反复打 uni）；能力半停用 ⇒ provider 零调用', () => {
    const app = probeApp('app', { providerResult: { providerIds: ['weixin'] } });
    const reads = [app.face.wechatAvailable.value, app.face.wechatAvailable.value, app.face.wechatUnavailableReason.value, app.face.wechatLoginReady.value];
    expect(reads).toEqual([true, true, '', false]);
    expect(app.log.getSystemInfo).toBe(1);
    expect(app.log.getProvider).toBe(0);
  });
});

// ── F：判别力自检（改坏必红；A-E 组即「真源必不红」的对照） ─────────────────

describe('F. 注入变异：每一种改坏形态都必须被上面同一条判据抓到', () => {
  const GUARD_BLOCK = `        if (agreed.value == false) {\n            uni.showToast({ title: '${MSG_AGREE}', icon: 'none' })\n            return\n        }\n`;
  const NOT_WIRED_BLOCK = `        if (wechatLoginReady == false) {\n            uni.showToast({ title: WECHAT_NOT_WIRED_NOTICE, icon: 'none' })\n            return\n        }\n`;
  const REQUEST_LINE = '            const result = await auth.loginByWechat()';
  const SHOW_LINE = 'return onMpWeixin || onApp';
  const READY_LINE = '        return onMpWeixin\n';

  test('F0 锚点存在性（变异落空即抛，不假绿）', () => {
    expect(landingSrc().split(GUARD_BLOCK).length - 1).toBe(1);
    expect(landingSrc().split(NOT_WIRED_BLOCK).length - 1).toBe(1);
    expect(landingSrc().split(REQUEST_LINE).length - 1).toBe(1);
    expect(landingSrc().split('        showAgreementNotice(name)').length - 1).toBe(1);
    expect(noticeSrc().split(`const AGREEMENT_NOTICE_SUFFIX = '${NOTICE_SUFFIX}'`).length - 1).toBe(1);
    expect(outletSrc().split(`uni.reLaunch({ url: '${CHOOSE_CERT}' })`).length - 1).toBe(1);
    expect(probeSrc().split('const onMpWeixin = platform == PLATFORM_MP_WEIXIN').length - 1).toBe(1);
    expect(probeSrc().split(SHOW_LINE).length - 1).toBe(1);
    expect(probeSrc().split(READY_LINE).length - 1).toBe(1);
    expect(probeSrc().split('return providerIds.indexOf(PROVIDER_OAUTH_WEIXIN) >= 0').length - 1).toBe(1);
    expect(probeSrc().split('        return false\n').length - 1).toBe(1);
  });

  test('F1 摘掉整个门槛 ⇒ 未勾选照发请求（A1 的成因）', async () => {
    const app = landingApp({ source: mutate(LANDING_REL, GUARD_BLOCK, '') });
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
    expect(titles(app.log)).not.toContain(MSG_AGREE);
  });

  test('F2 只删 return（toast 了但没拦住 —— 比 F1 更阴险的历史形态）⇒ 请求仍发出', async () => {
    const broken = mutate(LANDING_REL,
      `            uni.showToast({ title: '${MSG_AGREE}', icon: 'none' })\n            return\n`,
      `            uni.showToast({ title: '${MSG_AGREE}', icon: 'none' })\n`);
    const app = landingApp({ source: broken });
    await app.face.onWechatLogin();
    expect(titles(app.log)).toContain(MSG_AGREE); // 只看 toast 会以为守住了
    expect(app.calls.loginByWechat).toBe(1); // 计数才抓得住
  });

  test('F3 门槛搬到请求之后 ⇒ 门槛文案迟到、请求已发（顺序判据翻红）', async () => {
    const moved = mutate(LANDING_REL, GUARD_BLOCK, '')
      .replace(REQUEST_LINE, `${REQUEST_LINE}\n${GUARD_BLOCK.replace('        ', '            ').trimEnd()}`);
    const app = landingApp({ source: moved });
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
  });

  test('F3b 接通门槛搬到请求之后 ⇒ 样式位上请求已发、提示迟到（A2-1 的顺序判据翻红）', async () => {
    const moved = mutate(LANDING_REL, NOT_WIRED_BLOCK, '')
      .replace(REQUEST_LINE, `${REQUEST_LINE}\n${NOT_WIRED_BLOCK.replace('        ', '            ').trimEnd()}`);
    const app = landingApp({ source: moved, loginReady: false });
    app.face.agreed.value = true; // 否则先撞协议门槛，走不到请求
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
    expect(titles(app.log)).toContain(NOT_WIRED);
  });

  test('F4 出口把新用户分支改成直进 dashboard ⇒ 新用户丢认证引导（D2 的成因）', async () => {
    const broken = mutate(OUTLET_REL,
      `uni.reLaunch({ url: '${CHOOSE_CERT}' })`,
      `uni.reLaunch({ url: '${DASHBOARD}' })`);
    const app = outletApp({ source: broken });
    app.afterLoginSuccess(true);
    app.timer.flush();
    expect(app.log.reLaunch).toEqual([DASHBOARD]); // 与 D2 断言的 CHOOSE_CERT 相反 ⇒ D2 必红
  });

  test('F5 出口去掉延时（当场跳转）⇒ 成功 toast 被 reLaunch 打断（D1「当场不得跳转」的成因）', () => {
    const broken = mutate(OUTLET_REL,
      `        setTimeout(() => {\n            uni.reLaunch({ url: '${DASHBOARD}' })\n        }, 500)`,
      `        uni.reLaunch({ url: '${DASHBOARD}' })`);
    const app = outletApp({ source: broken });
    app.afterLoginSuccess(false);
    expect(app.timer.pending).toEqual([]);
    expect(app.log.reLaunch).toEqual([DASHBOARD]);
  });

  test('F6 失败被吞成固定文案（不透传 message）⇒ 真实原因不可见（B1 的成因）', async () => {
    const broken = mutate(LANDING_REL, 'msg = m', 'msg = msg');
    const app = landingApp({ source: broken, wechatImpl: () => Promise.reject(new Error('微信授权已取消')) });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.face.errorHint.value).toBe(MSG_FALLBACK);
    expect(app.face.errorHint.value).not.toBe('微信授权已取消');
  });

  test('F7 端别判据取反 ⇒ mp-weixin 反被判不可用（E1 的成因）', () => {
    const broken = mutate(PROBE_REL, 'const onMpWeixin = platform == PLATFORM_MP_WEIXIN', 'const onMpWeixin = platform != PLATFORM_MP_WEIXIN');
    const app = probeApp('mp-weixin', { source: broken, providerThrows: true });
    expect([app.face.wechatAvailable.value, app.face.wechatLoginReady.value]).toEqual([false, false]); // E1 期望 [true,true] ⇒ 必红
  });

  test('F7b 端别判据放宽成「非 app 即可用」⇒ web/未知值误判可用（E3 的成因，也是 R1 的「给了点不动」）', () => {
    const broken = mutate(PROBE_REL, 'platform == PLATFORM_MP_WEIXIN', "platform != 'app'");
    const app = probeApp('web', { source: broken });
    expect(app.face.wechatAvailable.value).toBe(true); // E3 期望 false ⇒ 必红
  });

  test('F8 探测件里保留能力半函数但**不许接回任何一面**（#1482 前的接缝形态）⇒ App 端被误判可接通（E2/E4 的成因）', () => {
    // 误接形态就是 2026-10-03 真机判红的那个写法：把 providerFound 拉回 || 一侧
    const broken = mutate(PROBE_REL, READY_LINE, '        return onMpWeixin || hasWechatOauthProvider()\n');
    const app = probeApp('app', { source: broken, providerResult: { providerIds: ['weixin'] } });
    expect(app.face.wechatLoginReady.value).toBe(true); // 误接即得 true ⇒ E2/E4 的 false 必红
  });

  test('F9 能力半函数的异常塌陷不得被摘（④c 的零先例 API 证据与 #1482 接缝都在它身上）⇒ 强制接回后探测异常冒出', () => {
    const wired = mutate(PROBE_REL, READY_LINE, '        return onMpWeixin || hasWechatOauthProvider()\n');
    // 能力半已停用，塌陷只有接回后才可达 —— 先接回，再摘塌陷（唯一性由 F0 的专用锚点断言担保）
    const noCatch = wired.split('        return false\n').join('        throw e\n');
    // 接通面是 computed（懒求值）：必须真读一次才走得到 hasWechatOauthProvider 的 catch
    expect(() => probeApp('app', { source: noCatch, providerThrows: true }).face.wechatLoginReady.value).toThrow();
    // 对照组一：真源（能力半未接回）同一条喂法不抛
    expect(() => probeApp('app', { providerThrows: true }).face.wechatLoginReady.value).not.toThrow();
    // 对照组二：只接回、不摘塌陷 ⇒ 也不抛（F9 抓的是「摘掉塌陷」，不是「接回」）
    expect(() => probeApp('app', { source: wired, providerThrows: true }).face.wechatLoginReady.value).not.toThrow();
  });

  test('F10 展示面把 App 那半丢掉（= 「移动端不展示」的旧口径）⇒ App 端入口消失（E2 的成因）', () => {
    const broken = mutate(PROBE_REL, SHOW_LINE, 'return onMpWeixin');
    const app = probeApp('app', { source: broken });
    expect(app.face.wechatAvailable.value).toBe(false); // E2 期望 true ⇒ 必红
  });

  test('F11 接通门槛写反（`== false` 改成 `== true`）⇒ 样式位上真发请求（A2-1 的成因）', async () => {
    const broken = mutate(LANDING_REL, 'if (wechatLoginReady == false) {', 'if (wechatLoginReady == true) {');
    const app = landingApp({ source: broken, loginReady: false });
    app.face.agreed.value = true;
    await app.face.onWechatLogin();
    expect(app.calls.loginByWechat).toBe(1);
  });
});
