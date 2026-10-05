/**
 * #1484 App 端微信登录入口：禁用态点击必须**挡在任何请求之前** —— 行为级守护（真跑 useLoginForm）
 *   ＋ 入口件接线锁
 *
 * 票面判据里最重的一条是「**禁用态下点击 ⇒ 请求层被调次数 = 0**」，它替代了旧方案「隐藏即不会误点」
 * 那道防线：入口现在两端常驻可见（#1478 判据 2 的 2026-10-03 修订），App 端在 #1482 配好微信 SDK
 * 之前永远处于未接通态 ⇒ 「看得见但不兑现」要成立，靠的是**点击链里有一道真门禁**，不是靠看不见。
 *
 * 为什么必须真跑 `.uts`（与 `utils/authPreRequestValidationBehavior.test.js` 同一条理由）：
 *   `expect(src).toContain('if (wechatLoginReady == false)')` 只能证明字面量出现过，答不了 ③ 门
 *   第一条判据「我故意弄坏被测物，它会不会红」，更证明不了「请求没发出去」。⇒ 用
 *   `utils/utsHarness.js` 起**真的** `useLoginForm`，经返回面 `onProviderPick` 驱动，数请求层次数。
 *
 * 成对取证（AGENTS.md「③ 门三条判据」之第 ③ 条：只跑通过的那一次不算验收）：
 *   · 必不红 = A1/A2/A3（真源：未接通零请求、接通才切模式、未知 provider 什么都不做）
 *   · 必红   = A4（把门禁判据反写 ⇒ 同一条点击就把请求打到 `loginByWechat`，证明 A1 不是恒真）
 *
 * 边界（照实记，别把本文件读成它够不到的面）：
 *   · `.uvue` 的模板与样式在 node 里**跑不了** ⇒ 「入口真的渲染出来了」由 B1 的挂载锁 + ①a 真机
 *     截图两道分别兜；本文件不假装能验渲染面。
 *   · B 组全是**接线守护**（源码文本事实），按 `docs/agents/guards.md` 口径**不构成 ③ 门的行为证据**
 *     —— 行为证据只有 A 组。写在这里是为了让「门禁搬出 composable / 组件不再挂载」这类退化立刻红。
 *   · 三份 json（manifest / pages / platformConfig）零改动是 **PR 级**判据（`pr-evidence.yml` 的
 *     改动集判定），不在这里断言 —— jest 里跑 git 会把测试与 worktree 状态耦合，本仓无此先例。
 */
const path = require('path');
const h = require('./contractHarness');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const FORM_REL = 'pages/login/composables/useLoginForm.uts';
const PAGE_REL = 'pages/login/login.uvue';
const PROBE_REL = 'composables/useLoginProviders.uts';
const COMPONENT_REL = 'components/login-provider-entry/login-provider-entry.uvue';

/** 文案真源（`composables/useLoginProviders.uts` 的 `WECHAT_NOT_WIRED_NOTICE`）—— 本文件是**副本**，
 *  与 `authPreRequestValidationBehavior.test.js:117` 同款做法：断言用字面量，真源搬家即红。 */
const NOTICE = '微信登录暂未开通';

/** vue 的最小响应式容器（`vue` 不在 node_modules 的解析射程内，本仓行为测试一律注入） */
const vueShim = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/** 收集 toast 的 `uni` fake */
function makeUni() {
  const toasts = [];
  return {
    toasts,
    uni: {
      showToast: (o) => { toasts.push(o.title); },
      showLoading: () => {},
      hideLoading: () => {},
      navigateTo: () => {},
      reLaunch: () => {},
      redirectTo: () => {},
    },
  };
}

/**
 * 起一个**真的**登录页表单 composable，`calls` 记录请求层被打到几次。
 * @param opts.ready 接通面快照：false = App 端样式位（现实况），true = 小程序端形态
 * @param source 变异源（缺省 = 磁盘真源）
 */
function form(opts, source) {
  const ready = !(opts != null && opts.ready === false);
  const { toasts, uni } = makeUni();
  const calls = { login: [], loginByCode: [], loginByWechat: [], sendCodeApi: [], getCaptchaApi: [], afterLoginSuccess: [], showAgreementNotice: [] };
  const bindings = {
    ...vueShim(),
    useAuthStore: () => ({
      login: (p) => { calls.login.push(p); return Promise.resolve(); },
      loginByCode: (p) => { calls.loginByCode.push(p); return Promise.resolve(); },
      loginByWechat: () => { calls.loginByWechat.push({}); return Promise.resolve({ isNew: false }); },
      getRefreshToken: () => 'faked-refresh-token',
    }),
    sendCodeApi: (target) => { calls.sendCodeApi.push(target); return Promise.resolve(); },
    // 图形验证码也是**请求**：`switchMode()` 尾部会 `loadCaptcha()` ⇒ 「先切模式后拦请求」这种写反
    // 的顺序会在这里露馅（票面「零请求」判据包含它，别只数登录三件套）。
    getCaptchaApi: () => { calls.getCaptchaApi.push(1); return Promise.resolve({ id: 'fake-captcha-id', image: 'fake-image' }); },
    saveSecureCredentials: () => {},
    saveAccountOnly: () => {},
    clearSecureCredentials: () => {},
    afterLoginSuccess: (isNew) => { calls.afterLoginSuccess.push(isNew); },
    showAgreementNotice: (name) => { calls.showAgreementNotice.push(name); },
    WECHAT_NOT_WIRED_NOTICE: NOTICE,
    uni,
    setInterval: () => 0,
    clearInterval: () => {},
    setTimeout: () => 0,
  };
  let mod;
  if (source == null) {
    mod = loadUts(path.join(ROOT, 'pages', 'login', 'composables', 'useLoginForm.uts'), bindings);
  } else {
    const os = require('os');
    const fs = require('fs');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1484-'));
    const file = path.join(dir, 'useLoginForm.uts');
    fs.writeFileSync(file, source);
    try {
      mod = loadUts(file, bindings);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }
  return { face: mod.useLoginForm({ isSupported: { value: false } }, ready), calls, toasts };
}

/** 取 `onProviderPick` 的函数体（composable 内局部函数：4 空格收尾，同 loginGating 的 fnBody 口径） */
function pickBody(src) {
  const start = src.indexOf('function onProviderPick(');
  if (start === -1) return '';
  const end = src.indexOf('\n    }', start);
  return end === -1 ? src.slice(start) : src.slice(start, end);
}

/** 单点字面量替换；锚点不唯一即抛（否则变异静默落空 ⇒「必红」用例变成假绿） */
function mutate(rel, from, to) {
  const src = readText(path.join(ROOT, rel));
  const parts = src.split(from);
  if (parts.length !== 2) throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  return parts.join(to);
}

describe('A 行为面：入口点击门禁（真跑 useLoginForm.onProviderPick）', () => {
  it('A1 App 端（接通面 = false，现实况）点微信入口 ⇒ 请求层 0 次、只给未开通提示、模式不变、不进 loading', async () => {
    const app = form({ ready: false });
    const before = app.face.mode.value;
    await app.face.onProviderPick('wechat');
    expect(app.calls.loginByWechat.length).toBe(0);
    expect(app.calls.loginByCode.length).toBe(0);
    expect(app.calls.login.length).toBe(0);
    expect(app.calls.sendCodeApi.length).toBe(0);
    expect(app.calls.getCaptchaApi.length).toBe(0);
    expect(app.calls.afterLoginSuccess.length).toBe(0);
    expect(app.toasts).toEqual([NOTICE]);
    expect(app.face.mode.value).toBe(before);
    expect(app.face.loading.value).toBe(false);
  });

  it('A2 连点五次仍然零请求（门禁不是「第一次生效、后面漏」的态）', async () => {
    const app = form({ ready: false });
    for (let i = 0; i < 5; i++) await app.face.onProviderPick('wechat');
    expect(app.calls.loginByWechat.length).toBe(0);
    expect(app.calls.loginByCode.length).toBe(0);
    expect(app.calls.getCaptchaApi.length).toBe(0);
    expect(app.toasts.length).toBe(5);
    expect(app.face.loading.value).toBe(false);
  });

  it('A3 小程序形态（接通面 = true）点入口 ⇒ 切到微信模式；点入口本身≠提交（登录请求 0 次），且不再有「未开通」提示', async () => {
    const app = form({ ready: true });
    await app.face.onProviderPick('wechat');
    expect(app.face.mode.value).toBe('wechat');
    expect(app.calls.loginByWechat.length).toBe(0);
    expect(app.toasts).toEqual([]);
    // 切模式**确实**要打一次图形验证码（switchMode 尾部 loadCaptcha，既有行为）——
    // 把它写成期望值而不是无视，A1 的「零请求」才是同一条计数的反向对照，不是两套口径。
    expect(app.calls.getCaptchaApi.length).toBe(1);
  });

  it('A4 判别力（必红一侧）：把接通面判据反写 ⇒ 同一条 App 点击就切模式并打出验证码请求，A1 的「模式不变 + 零请求」立刻塌', async () => {
    const broken = mutate(FORM_REL, 'if (wechatLoginReady == false) {', 'if (wechatLoginReady == true) {');
    const app = form({ ready: false }, broken);
    await app.face.onProviderPick('wechat');
    expect(app.face.mode.value).toBe('wechat');
    expect(app.calls.getCaptchaApi.length).toBe(1);
    expect(app.toasts).toEqual([]);
  });

  it('A5 未知 provider（Apple 槽位的未来值）⇒ 不切模式、零请求、零提示 —— 入口件按 provider 参数化，不能「任何值都放行」', async () => {
    const app = form({ ready: false });
    const before = app.face.mode.value;
    await app.face.onProviderPick('apple');
    expect(app.face.mode.value).toBe(before);
    expect(app.calls.loginByWechat.length).toBe(0);
    expect(app.calls.getCaptchaApi.length).toBe(0);
    expect(app.toasts).toEqual([]);
  });
});

describe('B 接线面：入口件存在 / 挂载 / 门禁归属 / 文案单点', () => {
  it('B1 入口件存在，且被登录页显式 import + 模板挂载（挂载条件是**端别**，不是接通态）', () => {
    expect(h.exists(COMPONENT_REL)).toBe(true);
    const page = h.read(PAGE_REL);
    expect(page).toContain("import LoginProviderEntry from '../../components/login-provider-entry/login-provider-entry.uvue'");
    expect(page).toContain('<LoginProviderEntry');
    expect(page).toContain('v-if="isAppPlatform"');
    expect(page).toContain(':provider="\'wechat\'"');
    expect(page).toContain(':ready="wechatLoginReady"');
    expect(page).toContain('@pick="onProviderPick"');
  });

  it('B1b 判别力：挂载不能只是「写在注释里」——剥掉 HTML 注释后 `<LoginProviderEntry` 仍须在', () => {
    const stripped = h.read(PAGE_REL).replace(/<!--[\s\S]*?-->/g, '');
    expect(stripped).toContain('<LoginProviderEntry');
    // 反向对照：本条判据对「只存在于注释里」的形态确实会红（把上面那行注掉就复现不出来）
    const commented = '<!-- <LoginProviderEntry v-if="isAppPlatform" /> -->';
    expect(commented.replace(/<!--[\s\S]*?-->/g, '')).not.toContain('<LoginProviderEntry');
  });

  it('B2 门禁住在 composable（可被 jest 真跑），且提示**在切模式之前** —— 顺序即「零请求」的全部保证', () => {
    const body = pickBody(h.read(FORM_REL));
    expect(body).not.toBe('');
    expect(body).toContain('WECHAT_NOT_WIRED_NOTICE');
    const toastAt = body.indexOf('uni.showToast');
    const switchAt = body.indexOf('switchMode(');
    expect(toastAt).toBeGreaterThanOrEqual(0);
    expect(switchAt).toBeGreaterThanOrEqual(0);
    expect(toastAt).toBeLessThan(switchAt);
  });

  it('B3 页面/组件的解构与声明面齐备：探测件新增**端别面**、composable 返回面新增 onProviderPick', () => {
    const probe = h.read(PROBE_REL);
    expect(probe).toContain('isAppPlatform : ComputedRef<boolean>');
    const page = h.read(PAGE_REL);
    expect(page).toContain('const { wechatAvailable, wechatLoginReady, isAppPlatform } = useLoginProviders()');
    expect(page).toContain('        onProviderPick,');
    expect(h.read(FORM_REL)).toContain('onProviderPick : (provider : string) => void');
  });

  it('B4 未开通文案仍是**单点**：全仓内联该字面量的源文件恰为探测件一处（组件与页面只引用）', () => {
    const files = h.filesUnder('.', /\.(uvue|uts)$/, true);
    const inliners = files.filter((rel) => h.read(rel).includes("'" + NOTICE + "'")).sort();
    expect(inliners).toEqual([PROBE_REL]);
  });

  it('B5 本票不得开出任何通往小程序端点的 App 侧链路：新增面里不出现 /auth/wx-login、mpWechatLoginApi、loginByWechat', () => {
    const component = h.read(COMPONENT_REL);
    expect(component).not.toContain('wx-login');
    expect(component).not.toContain('mpWechatLoginApi');
    expect(component).not.toContain('loginByWechat');
    expect(pickBody(h.read(FORM_REL))).not.toContain('loginByWechat');
    expect(pickBody(h.read(FORM_REL))).not.toContain('wx-login');
  });

  it('B6 入口件按 provider 参数化，且**不自建第二套平台判断**（端别只在探测件取数一次）', () => {
    const component = h.read(COMPONENT_REL);
    expect(component).toContain('provider? : string');
    expect(component).toContain('ready? : boolean');
    expect(component).not.toContain('uniPlatform');
    expect(component).not.toContain('getProviderSync');
    expect(component).not.toMatch(/<button[^>]*:disabled="false"/);
  });

  it('B7 新组件目录登记为跨切面基础设施（否则 `modulesDeclarationContract` 的 E2「零隐形文件」判红）', () => {
    expect(h.INFRA.dirs).toContain('components/login-provider-entry');
  });

  it('B8 条件编译指令行零增删：登录页仍是 2 条（#ifdef + #endif 一对），入口件内 0 条（② 门触发面）', () => {
    const count = (src) => (src.match(/^\s*<!--\s*#(ifdef|ifndef|endif)\b/gm) || []).length;
    expect(count(h.read(PAGE_REL))).toBe(2);
    expect(count(h.read(COMPONENT_REL))).toBe(0);
  });

  it('B9 「微信登录」这个词现在有两处字面（入口件的 provider 映射 / 小程序端那条文字链）——本条不消除重复，只钉住「两处同词」，漂了即红', () => {
    expect(h.read(COMPONENT_REL)).toContain("if (props.provider == 'wechat') return '微信登录'");
    expect(h.read(PAGE_REL)).toContain('>微信登录</text>');
  });
});
