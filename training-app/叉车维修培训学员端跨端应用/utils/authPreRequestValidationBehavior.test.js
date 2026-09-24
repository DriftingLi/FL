/**
 * #1262 + #1286：auth 两页的客户端校验必须**挡在任何网络请求之前** —— 行为级守护（真跑两个 composable）
 *
 * 病根（两张票，同一个 PR 收口）：
 *   ① #1286：登录页手机验证码模式缺「必须为数字」这一档。唯一能补它的一行处于**注释状态**，
 *      且注释里的正则写作 `/^d{11}$/`（匹配「字母 d 重复 11 次」）而非 `/^\d{11}$/`
 *      ⇒ 「顺手取消注释」会把每一个真实手机号判成不合法，手机验证码登录整条模式不可用。
 *   ② #1262：登录页 / 找回密码页的口令档只判下限（`< 6`），21–32 位放行到后端才被 400 拒回；
 *      注册页与后端唯一规则源（`service.validatePasswordLength`）都是 6-20。
 *
 * 本文件钉住的不变量：
 *   I1 数字档**只在真实手机号之外生效**：`'1abcdefghij'`（11 位、以 1 开头、含字母）⇒ 本地拦下、
 *      零请求；`'13800138000'` ⇒ 放行到请求层。**这两条合起来才是判据** —— 单看前者，
 *      把正则写回坏形态 `/^d{11}$/` 也照样「拦住了 1abcdefghij」，抓不出事故。
 *   I2 口令是 6-20 **区间**：21 位（票面缺口）与 32 位（`:maxlength` 允许的上界）本地拦下、零请求；
 *      6 位与 20 位（区间两侧）仍然放行。登录页与找回密码页两页同形。
 *   I3 新档位**不外溢**：登录页手机验证码 / 邮箱 / 微信三条通道的行为逐字不变（口令字段根本不参与）。
 *
 * ⚠️ 与票面探针输入的偏差（现测所得，写下来防下一个会话照抄）：#1286 建议用 `'ddddddddddd'`
 *   作为「数字档生效」的输入，但本仓该函数的档位顺序是「长度 → `startsWith('1')` → 数字」，
 *   全 d 的输入在第二档就被「手机号格式不正确」拦掉了，**根本走不到数字档**（它只能证明 startsWith
 *   那一档在）。所以 I1 用 `'1abcdefghij'`，并用合法号码做反向那一半。
 *
 * 为什么不用源码文本断言：`expect(src).toContain('/^\\d{11}$/.test(p)')` 只能证明字面量出现过 ——
 * 它答不了 ③ 门的第一条判据「我故意弄坏被测物，它会不会红」，也证明不了「请求没发出去」。
 *  ⇒ 用 `utils/utsHarness.js` 把**真的** `.uts` 跑起来，经**返回面**（`onSubmit` / `onSendCode`）驱动，
 *    数请求层被调用了几次。`validate` / `validatePhoneNum` **不在 composable 返回面上**
 *    （`useLoginForm.uts` 文件头：页面与门控均不读 ⇒ T05 死代码消费面口径），故一律走返回面。
 *  「必红」一侧由 describe D 的注入变异用例给（真源改成术前形态 / 坏正则 / 阈值挪位 ⇒ 必红）。
 *
 * 边界（照实记）：`.uvue` 的模板与样式在 node 里跑不了 ⇒ 页面壳层不在本文件执行面内；
 *   `:maxlength="11"` / `:maxlength="32"` / `type="number"` 属模板面，#1262 明确裁定**不改**，
 *   由 `utils/forgotPasswordContract.test.js` 的模板计数锁守。`uni` 的 toast 出口、`vue` 的
 *   `ref` / `computed`、`stores/auth` 与 `api/auth.uts` 都是**非被测依赖**，按最小 fake 注入。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LOGIN_UTS = path.join(ROOT, 'pages', 'login', 'composables', 'useLoginForm.uts');
const FORGOT_UTS = path.join(ROOT, 'pages', 'forgot-password', 'composables', 'useForgotPasswordForm.uts');

/** 票面缺口与区间两侧（32 = 两页密码框 `:maxlength` 的上界，本票不改） */
const PW_MIN = 'a'.repeat(6);
const PW_MAX = 'a'.repeat(20);
const PW_OVER = 'a'.repeat(21);
const PW_CEIL = 'a'.repeat(32);
/** 11 位、以 1 开头、含字母 ⇒ 只有数字档能拦住它 */
const PHONE_LETTER = '1abcdefghij';
/** 真实号段形态的合法号码：数字档**不得**拦住它 */
const PHONE_OK = '13800138000';

const MSG_DIGIT = '手机号必须为数字';
const MSG_PWD = '密码长度需为 6-20 位';

/** vue 的最小响应式容器（`vue` 不在 node_modules 的解析射程内，本仓行为测试一律注入） */
const vueShim = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/** 收集 toast 的 `uni` fake；导航/ loading 出口一律空实现（延后跳转不排程，见 setTimeout） */
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

/** 把变异后的源码写进临时目录再交给 harness —— 真源文件一个字节都不动 */
function loadFromSource(source, fileName, bindings) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1262-1286-'));
  const file = path.join(dir, fileName);
  fs.writeFileSync(file, source);
  try {
    return loadUts(file, bindings);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** 单点字面量替换；锚点不唯一即抛（否则变异静默落空 ⇒ 「必红」用例变成假绿） */
function mutate(rel, from, to) {
  const src = readText(path.join(ROOT, rel));
  const parts = src.split(from);
  if (parts.length !== 2) {
    throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  }
  return parts.join(to);
}

/** 真源文本（EOL 经读者归一，ADR-0019）—— 变异锚点的基准 */
const loginSrc = () => readText(LOGIN_UTS);
const forgotSrc = () => readText(FORGOT_UTS);

/**
 * 起一个**真的**登录页表单 composable（`pages/login/composables/useLoginForm.uts`）。
 * `calls` 记录请求层被打到几次 —— I1/I2/I3 的「零请求」判据就看它。
 */
function loginApp(source) {
  const { toasts, uni } = makeUni();
  const calls = { login: [], loginByCode: [], loginByWechat: [], sendCodeApi: [] };
  const bindings = {
    ...vueShim(),
    useAuthStore: () => ({
      login: (p) => { calls.login.push(p); return Promise.resolve(); },
      loginByCode: (p) => { calls.loginByCode.push(p); return Promise.resolve(); },
      loginByWechat: () => { calls.loginByWechat.push({}); return Promise.resolve({ isNew: false }); },
      getRefreshToken: () => 'faked-refresh-token',
    }),
    sendCodeApi: (target) => { calls.sendCodeApi.push(target); return Promise.resolve(); },
    getCaptchaApi: () => Promise.resolve({ id: 'fake-captcha-id', image: 'fake-image' }),
    saveSecureCredentials: () => {},
    saveAccountOnly: () => {},
    clearSecureCredentials: () => {},
    uni,
    setInterval: () => 0,
    clearInterval: () => {},
    setTimeout: () => 0,
  };
  const mod = source == null ? loadUts(LOGIN_UTS, bindings) : loadFromSource(source, 'useLoginForm.uts', bindings);
  return { face: mod.useLoginForm({ isSupported: { value: false } }), calls, toasts };
}

/** 起一个**真的**找回密码页 composable（`pages/forgot-password/composables/useForgotPasswordForm.uts`） */
function forgotApp(source) {
  const { toasts, uni } = makeUni();
  const calls = { resetPasswordApi: [] };
  const bindings = {
    ...vueShim(),
    resetPasswordApi: (...a) => { calls.resetPasswordApi.push(a); return Promise.resolve(); },
    sendCodeApi: () => Promise.resolve(),
    getCaptchaApi: () => Promise.resolve({ id: 'fake-captcha-id', image: 'fake-image' }),
    uni,
    setInterval: () => 0,
    clearInterval: () => {},
    setTimeout: () => 0,
  };
  const mod = source == null ? loadUts(FORGOT_UTS, bindings) : loadFromSource(source, 'useForgotPasswordForm.uts', bindings);
  return { face: mod.useForgotPasswordForm(), calls, toasts };
}

/** 手机号验证码模式提交：填齐除手机号外的全部前置（协议勾选 + 6 位验证码） */
async function submitPhone(app, phone) {
  app.face.mode.value = 'phone';
  app.face.agreed.value = true;
  app.face.phone.value = phone;
  app.face.phoneCode.value = '123456';
  await app.face.onSubmit();
}

/** 账号密码模式提交 */
async function submitPassword(app, password) {
  app.face.mode.value = 'password';
  app.face.username.value = 'student01';
  app.face.password.value = password;
  await app.face.onSubmit();
}

/** 「获取验证码」按钮（#1286 的第一落点：图形验证码前置之后、发送之前） */
async function sendPhoneCode(app, phone) {
  await app.face.loadCaptcha();
  app.face.captchaValue.value = '9x7q';
  app.face.phone.value = phone;
  await app.face.onSendCode('phone');
}

/** 找回密码页提交：填齐手机通道与验证码，只把口令留给参数 */
async function submitReset(app, password) {
  app.face.mode.value = 'phone';
  app.face.form.value = {
    phone: PHONE_OK,
    email: '',
    code: '123456',
    password,
    confirmPassword: password,
  };
  await app.face.onSubmit();
}

describe('A. #1286 数字档：真跑登录页 composable，拦非数字且不误伤合法号码', () => {
  test('A1 提交路径：11 位含字母的手机号被本地拦下，auth.loginByCode 零调用', async () => {
    const app = loginApp();
    await submitPhone(app, PHONE_LETTER);
    expect(app.toasts[app.toasts.length - 1]).toBe(MSG_DIGIT);
    expect(app.calls.loginByCode.length).toBe(0);
  });

  test('A2 获取验证码路径：同一档也挡在 sendCodeApi 之前（零短信请求）', async () => {
    const app = loginApp();
    await sendPhoneCode(app, PHONE_LETTER);
    expect(app.toasts[app.toasts.length - 1]).toBe(MSG_DIGIT);
    expect(app.calls.sendCodeApi.length).toBe(0);
  });

  test('A3 反向半边：合法号码穿过数字档，抵达请求层一次', async () => {
    const app = loginApp();
    await submitPhone(app, PHONE_OK);
    expect(app.toasts).not.toContain(MSG_DIGIT);
    expect(app.calls.loginByCode.length).toBe(1);
    expect(app.calls.loginByCode[0].target).toBe(PHONE_OK);
  });

  test('A4 档位顺序不外溢：全 d 输入仍由 startsWith 档给出原文案（证明新档没插错位置）', async () => {
    const app = loginApp();
    await submitPhone(app, 'd'.repeat(11));
    expect(app.toasts[app.toasts.length - 1]).toBe('手机号格式不正确');
    expect(app.calls.loginByCode.length).toBe(0);
  });
});

describe('B. #1262 口令 6-20 区间：两页同形，超限零请求', () => {
  test('B1 登录页 21 位（票面缺口）：本地文案 + auth.login 零调用', async () => {
    const app = loginApp();
    await submitPassword(app, PW_OVER);
    expect(app.toasts[app.toasts.length - 1]).toBe(MSG_PWD);
    expect(app.calls.login.length).toBe(0);
  });

  test('B2 登录页 32 位（:maxlength 上界）：同上', async () => {
    const app = loginApp();
    await submitPassword(app, PW_CEIL);
    expect(app.toasts[app.toasts.length - 1]).toBe(MSG_PWD);
    expect(app.calls.login.length).toBe(0);
  });

  test('B3 登录页区间两侧（6 位 / 20 位）仍然放行', async () => {
    for (const pw of [PW_MIN, PW_MAX]) {
      const app = loginApp();
      await submitPassword(app, pw);
      expect(app.toasts).not.toContain(MSG_PWD);
      expect(app.calls.login.length).toBe(1);
    }
  });

  test('B4 找回密码页 21 位 / 32 位：本地文案 + resetPasswordApi 零调用（一次性验证码不被烧）', async () => {
    for (const pw of [PW_OVER, PW_CEIL]) {
      const app = forgotApp();
      await submitReset(app, pw);
      expect(app.toasts[app.toasts.length - 1]).toBe(MSG_PWD);
      expect(app.calls.resetPasswordApi.length).toBe(0);
    }
  });

  test('B5 找回密码页区间两侧（6 位 / 20 位）仍然放行', async () => {
    for (const pw of [PW_MIN, PW_MAX]) {
      const app = forgotApp();
      await submitReset(app, pw);
      expect(app.toasts).not.toContain(MSG_PWD);
      expect(app.calls.resetPasswordApi.length).toBe(1);
    }
  });

  test('B6 文案与注册页 / 后端规则源逐字一致（不新造第三种）', async () => {
    const login = loginApp();
    await submitPassword(login, PW_OVER);
    const forgot = forgotApp();
    await submitReset(forgot, PW_OVER);
    const register = readText(path.join(ROOT, 'pages', 'register', 'composables', 'useRegisterForm.uts'));
    expect(register).toContain(`'${MSG_PWD}'`);
    expect(login.toasts[login.toasts.length - 1]).toBe(forgot.toasts[forgot.toasts.length - 1]);
  });
});

describe('C. #1262 边界：新档位不渗进其余登录通道', () => {
  test('C1 微信一键登录：口令字段不参与校验，空口令照样直达 loginByWechat', async () => {
    const app = loginApp();
    app.face.mode.value = 'wechat';
    app.face.agreed.value = true;
    await app.face.onSubmit();
    expect(app.toasts).not.toContain(MSG_PWD);
    expect(app.calls.loginByWechat.length).toBe(1);
    expect(app.calls.login.length).toBe(0);
  });

  test('C2 邮箱验证码登录：空口令照样直达 loginByCode(channel=email)', async () => {
    const app = loginApp();
    app.face.mode.value = 'email';
    app.face.agreed.value = true;
    app.face.email.value = 'a@b.co';
    app.face.emailCode.value = '123456';
    await app.face.onSubmit();
    expect(app.toasts).not.toContain(MSG_PWD);
    expect(app.calls.loginByCode.length).toBe(1);
    expect(app.calls.loginByCode[0].channel).toBe('email');
  });

  test('C3 手机号模式即便口令框里留着 32 位也不参与（校验的是 phone 分支）', async () => {
    const app = loginApp();
    app.face.password.value = PW_CEIL;
    await submitPhone(app, PHONE_OK);
    expect(app.toasts).not.toContain(MSG_PWD);
    expect(app.calls.loginByCode.length).toBe(1);
  });
});

describe('D. 判别力自检：把真源弄坏成四种历史形态，上面各条必须变红', () => {
  const DIGIT_TIER = "if (!/^\\d{11}$/.test(p)) return '手机号必须为数字'";

  test('D0 锚点存在性：数字档与两页口令档在真源里各只有一处（变异落空即抛，不假绿）', () => {
    expect(loginSrc().split(DIGIT_TIER).length - 1).toBe(1);
    expect(loginSrc().split("if (password.value.length < 6 || password.value.length > 20)").length - 1).toBe(1);
    expect(forgotSrc().split('if (password.length < 6 || password.length > 20)').length - 1).toBe(1);
  });

  test('D1 把正则写回票面注释里的坏形态 ⇒ 合法号码被误杀（A3 的成因）', async () => {
    const broken = mutate(path.join('pages', 'login', 'composables', 'useLoginForm.uts'), DIGIT_TIER, DIGIT_TIER.replace('/^\\d{11}$/', '/^d{11}$/'));
    const app = loginApp(broken);
    await submitPhone(app, PHONE_OK);
    expect(app.toasts).toContain(MSG_DIGIT);
    expect(app.calls.loginByCode.length).toBe(0);
  });

  test('D2 摘掉数字档（= 术前 #1286 现状）⇒ 含字母的号码被放行到请求层（A1/A2 的成因）', async () => {
    const broken = mutate(path.join('pages', 'login', 'composables', 'useLoginForm.uts'), `        ${DIGIT_TIER}\n`, '');
    const app = loginApp(broken);
    await submitPhone(app, PHONE_LETTER);
    expect(app.toasts).not.toContain(MSG_DIGIT);
    expect(app.calls.loginByCode.length).toBe(1);
  });

  test('D3 登录页口令退回「只判下限」（= 术前 #1262 现状）⇒ 21 位被放行到 auth.login（B1 的成因）', async () => {
    const broken = mutate(path.join('pages', 'login', 'composables', 'useLoginForm.uts'),
      "if (password.value.length < 6 || password.value.length > 20) return '密码长度需为 6-20 位'",
      "if (password.value.length < 6) return '密码至少 6 位'");
    const app = loginApp(broken);
    await submitPassword(app, PW_OVER);
    expect(app.toasts).not.toContain(MSG_PWD);
    expect(app.calls.login.length).toBe(1);
  });

  test('D4 阈值从 20 挪到 32（= 只跟 :maxlength 对齐、不跟规则源对齐）⇒ 21 位照样漏（B2 钉的是 20）', async () => {
    const broken = mutate(path.join('pages', 'login', 'composables', 'useLoginForm.uts'), 'password.value.length > 20', 'password.value.length > 32');
    const app = loginApp(broken);
    await submitPassword(app, PW_OVER);
    expect(app.calls.login.length).toBe(1);
  });

  test('D5 找回密码页口令退回「只判下限」⇒ 21 位被放行到 resetPasswordApi（B4 的成因）', async () => {
    const broken = mutate(path.join('pages', 'forgot-password', 'composables', 'useForgotPasswordForm.uts'),
      "if (password.length < 6 || password.length > 20) return '密码长度需为 6-20 位'",
      "if (password.length < 6) return '密码至少 6 位'");
    const app = forgotApp(broken);
    await submitReset(app, PW_OVER);
    expect(app.toasts).not.toContain(MSG_PWD);
    expect(app.calls.resetPasswordApi.length).toBe(1);
  });
});
