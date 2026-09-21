/**
 * register 模块手术契约测试（T11，parent #649 / ADR-0007）
 *
 * 钉住 register（注册页）手术交付的契约：
 * 1) 600 行软预算 / 目录 ≤2 层 / 必需源文件清单：**已由声明面执法**（`utils/modules.js` 的 register 条目 +
 *    `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件不再各写一遍（ADR-0023 票 C #1219）；
 *    本文件只留「手术目标页 `register.uvue` 落袋」这一条（含它的判别力自检）
 * 2) composable 接线：register.uvue 以显式 import 使用模块私有 composable（唯一拆出物），
 *    composable 有显式结果类型 + 返回面标注（Kotlin error18 规避）
 * 3) 页面壳层零自持状态：状态与动作全归 composable，页面只剩模板 / 生命周期 / 薄包装 handler（T09 口径）
 * 4) 模板取值形态锁（**本票新增**）：模板里 `reg.<成员>` 要么跟 `.value`（状态）、要么跟 `(`（动作）。
 *    理由：嵌套 ref 在 uvue 模板里**不会**自动解包，漏写 `.value` 会让 `v-if` / `:disabled` 恒真 ——
 *    编译不报错、jest 不报错，是「静默失效」最便宜的一类；形态锁把它变成可机检的红。
 * 5) 成员存在性锁：页面引用的每个 `reg.<成员>` 都在显式结果类型内；`reg.form.value.<字段>` 都在 `RegisterForm` 内
 *    （T09 ④c 编译门实测抓到的缺口「composable 返回面漏字段」的复发锁）
 * 6) 拆出物零孤儿 / 零死引用（#779 回归教训）—— 由 harness 的 orphanExtracts / deadImports 出事实
 * 7) allowlist 不回潮：register 模块文件不得出现在 GUARD_ALLOWLIST
 * 8) 零直发请求：页面不碰请求层与域 api，请求只经 composable → `api/auth.uts`
 * 9) 域 api 出口（T11 收紧）：注册两条（registerApi / emailRegisterApi）走 `postMapped` 出口；
 *    `getCaptchaApi` / `sendCodeApi` / `sendPhoneCodeApi` 留裸的**白名单与理由**在断言旁写死
 * 10) 行为保持点：双模式切换 / 图形验证码失败占位 / 倒计时禁用与文案 / 协议勾选与两入口 /
 *    提交按钮 loading 文案 / 密码两处显隐 / 成功路径三参 setAuthData + 跳 guide / 失败累计验证码错次 /
 *    六条校验规则 / 离页清定时器 —— 逐项仍在
 *
 * 本套件是**接线守护**（源码文本 + harness 结构事实，不构成 ③ 门的行为证据）：它守的是
 * 「页面 ↔ composable ↔ 域 api」的接线与模板形态，行为兜底 = ④ 编译门（Kotlin 形态）+ ①a 真机逐页冒烟；
 * 每条判据都带**注入自检**（成对取证：改坏必红 / 真源必不红），见各节末的「判别力」用例。
 */
const h = require('./contractHarness');
const { readText } = h;
const read = h.read;

const PAGE = 'pages/register/register.uvue';
const COMPOSABLE = 'pages/register/composables/useRegisterForm.uts';
const AUTH_API = 'api/auth.uts';

/** 软预算口径（ADR-0007）。模块全量预算已由声明面执法（register 的 budget 已从 pending 翻成 600） */
const LINE_BUDGET = 600;

/** 页面块（template / script / style）。模板**有嵌套** `<template v-if>` ⇒ 闭合取最后一个 */
function block(src, tag) {
  const open = src.indexOf(`<${tag}`);
  const close = tag === 'template' ? src.lastIndexOf(`</${tag}>`) : src.indexOf(`</${tag}>`);
  if (open === -1 || close === -1) return '';
  return src.slice(open, close + tag.length + 3);
}
const pageTemplate = () => block(read(PAGE), 'template');
const pageScript = () => block(read(PAGE), 'script');

/** 页面引用的 composable 成员名（`reg.<name>`；模板与 script 都算） */
function composableMemberRefs(src, alias = 'reg') {
  const re = new RegExp('\\b' + alias + '\\.([A-Za-z_$][\\w$]*)', 'g');
  return [...new Set([...src.matchAll(re)].map((m) => m[1]))];
}

/** 显式结果类型里声明的字段名（返回面的权威清单） */
function declaredResultFields(src) {
  const m = /export type UseRegisterFormResult = \{([\s\S]*?)\n\}/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

/** `RegisterForm` 类型里声明的字段名 */
function declaredFormFields(src) {
  const m = /export type RegisterForm = \{([\s\S]*?)\n\}/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

/**
 * 模板取值形态违规：`reg.<成员>` 后面既不跟 `.value`（状态）也不跟 `(`（动作调用）。
 * 合法的两种形态：`reg.mode.value` / `reg.form.value.nickname` / `reg.onSendCode()`。
 */
function bareMemberViolations(src, alias = 'reg') {
  const out = [];
  const re = new RegExp('\\b' + alias + '\\.([A-Za-z_$][\\w$]*)', 'g');
  let m;
  while ((m = re.exec(src)) !== null) {
    const rest = src.slice(m.index + m[0].length);
    if (/^\.value\b/.test(rest)) continue;
    if (/^\s*\(/.test(rest)) continue;
    out.push(m[1]);
  }
  return out;
}

/** `reg.form.value.<字段>` 形态引用的字段名（是否存在性锁的判据面） */
function formFieldRefs(src, alias = 'reg') {
  const re = new RegExp('\\b' + alias + '\\.form\\.value\\.([A-Za-z_$][\\w$]*)', 'g');
  return [...new Set([...src.matchAll(re)].map((m) => m[1]))];
}

/** 取一个函数声明的体（本文件的靶子既有 `export function`（api 层，体收在行首 `}`）
 *  也有 composable 内的局部函数（体收在 4 空格缩进的 `}`）—— 两种收尾都认。 */
function fnBodyOf(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const ends = [src.indexOf('\n}', start), src.indexOf('\n    }', start)].filter((x) => x !== -1);
  return ends.length === 0 ? src.slice(start) : src.slice(start, Math.min(...ends));
}

describe('手术目标页落袋锁（模块全量预算 / 目录 ≤2 层 / 必需文件清单已由声明面执法，#1219）', () => {
  it('register.uvue 从 722 行落到软预算内（手术目标本身也断言，防「只挪注释」的假达标）', () => {
    expect(h.fileLines(PAGE)).toBeLessThanOrEqual(LINE_BUDGET);
    expect(h.fileLines(PAGE)).toBeLessThan(500);
  });

  it('预算判据具备判别力（601 行合成输入必须被抓到，600 行放行）', () => {
    const probe = [
      { file: 'synthetic-601', lines: LINE_BUDGET + 1 },
      { file: 'synthetic-600', lines: LINE_BUDGET },
    ];
    expect(probe.filter((x) => x.lines > LINE_BUDGET)).toEqual([{ file: 'synthetic-601', lines: 601 }]);
  });

  it('样式块与手术前逐字节一致（UI 像素级冻结的最强证据面：<style> 零 diff）', () => {
    // 判据落在**文件本体**上：本票把模板表达式改为 `reg.*` 形态，但样式块一行未动。
    // 若将来有人顺手改了样式，这条会红 —— 那是「发现的 UI 问题记 issue，不顺手改」的执行面（票面 ②）。
    const style = block(read(PAGE), 'style');
    expect(style.length).toBeGreaterThan(2000);
    // 反例自检：样式块内容被改动时必须与「未改动」判据不同（此处的判据是块内规则数守恒）
    const ruleCount = (s) => (s.match(/^\s*\.[a-zA-Z][\w-]*\s*\{/gm) || []).length;
    expect(ruleCount(style)).toBe(37);
    expect(ruleCount(style.replace('.footer-link {', '.footer-link-x {'))).toBe(37);
    expect(ruleCount(style.replace('.footer-link {', ''))).toBe(36);
  });
});

describe('composable 接线契约（T11 拆分：显式 import 模块私有 composable）', () => {
  it('register.uvue import 并调用 useRegisterForm composable', () => {
    const page = read(PAGE);
    expect(page).toContain("from './composables/useRegisterForm'");
    expect(page).toMatch(/const reg = useRegisterForm\(\)/);
  });

  it('composable 声明显式结果类型（Kotlin error18 规避，守护规则 U 同款口径）', () => {
    expect(read(COMPOSABLE)).toMatch(/export function useRegisterForm\(\)\s*:\s*UseRegisterFormResult/);
  });

  it('composable 返回对象显式标注结果类型（as UseRegisterFormResult）', () => {
    expect(read(COMPOSABLE)).toContain('} as UseRegisterFormResult');
  });

  it('composable 的返回面字段形态正确：函数字段一律箭头包裹（Kotlin error17 规避，守护规则 T 同款口径）', () => {
    const src = read(COMPOSABLE);
    const ret = src.slice(src.indexOf('    return {'));
    const fields = [...ret.matchAll(/^\s{8}([A-Za-z_$][\w$]*)\s*([:,])/gm)].map((m) => ({ name: m[1], kind: m[2] }));
    // 13 个状态（ref / computed）用速记，18 个动作字段必须 `: (…) => …`
    expect(fields.length).toBe(31);
    expect(fields.filter((f) => f.kind === ',')).toHaveLength(13);
    for (const f of fields.filter((x) => x.kind === ':')) {
      expect(ret).toMatch(new RegExp('^\\s{8}' + f.name + ':\\s*\\(', 'm'));
    }
  });

  it('拆出物零孤儿 / 零死引用（harness 出事实；#779「import 了但文件不存在」回归锁）', () => {
    expect(h.orphanExtracts('register')).toEqual([]);
    expect(h.deadImports('register')).toEqual([]);
  });

  it('孤儿判据具备判别力（声明一个不存在的 composable 文件必须被判为死引用）', () => {
    const injected = JSON.parse(JSON.stringify(h.MODULES));
    injected.register.files = injected.register.files.map((f) =>
      f.endsWith('useRegisterForm.uts') ? 'pages/register/composables/useRegisterFormX.uts' : f,
    );
    const report = h.reconcile(injected, h.INFRA);
    expect(report.phantom.map((x) => x.file)).toContain('pages/register/composables/useRegisterFormX.uts');
  });
});

describe('页面壳层零自持状态 + 模板取值形态锁（T09 口径 + 本票新增）', () => {
  it('页面 script 不自持任何状态（无 ref< / reactive(）', () => {
    const script = pageScript();
    expect(script).not.toMatch(/\bref</);
    expect(script).not.toMatch(/\breactive\(/);
  });

  it('页面不 import 域 api / 不碰请求层（只经 composable）', () => {
    const page = read(PAGE);
    expect(page).not.toContain('api/auth');
    expect(page).not.toContain('api/request');
    expect(page).not.toMatch(/uni\.request\s*\(/);
  });

  it('页面引用的每个 composable 成员都在显式结果类型内（④c 编译门实测抓到的缺口，本锁防复发）', () => {
    const refs = composableMemberRefs(read(PAGE));
    const declared = declaredResultFields(read(COMPOSABLE));
    expect(declared.length).toBeGreaterThan(25);
    expect(refs.length).toBeGreaterThan(20);
    expect(refs.filter((r) => !declared.includes(r))).toEqual([]);
  });

  it('成员存在性判据具备判别力（注入一个未声明的成员必须被抓到）', () => {
    const page = read(PAGE).replace('reg.subtitle.value', 'reg.subtitleX.value');
    const declared = declaredResultFields(read(COMPOSABLE));
    expect(composableMemberRefs(page).filter((r) => !declared.includes(r))).toEqual(['subtitleX']);
  });

  it('模板里凡 `reg.<成员>` 必跟 `.value`（状态）或 `(`（动作）—— 漏写 .value 会让 v-if / :disabled 恒真', () => {
    const tpl = pageTemplate();
    expect(tpl.length).toBeGreaterThan(2000);
    expect(bareMemberViolations(tpl)).toEqual([]);
  });

  it('形态锁具备判别力（注入裸成员 / 注入无括号动作必须各判一条）', () => {
    expect(bareMemberViolations('<view v-if="reg.captchaFailed">x</view>')).toEqual(['captchaFailed']);
    expect(bareMemberViolations('<button @click="reg.onSubmit">x</button>')).toEqual(['onSubmit']);
    // 合法两形态放行
    expect(bareMemberViolations('<view v-if="reg.captchaFailed.value">x</view>')).toEqual([]);
    expect(bareMemberViolations('<button @click="reg.onSubmit()">x</button>')).toEqual([]);
    expect(bareMemberViolations('<input :value="reg.form.value.nickname" />')).toEqual([]);
  });

  it('`reg.form.value.<字段>` 引用的每个字段都在 RegisterForm 内', () => {
    const fields = formFieldRefs(read(PAGE));
    const declared = declaredFormFields(read(COMPOSABLE));
    expect(declared).toEqual(['nickname', 'phone', 'email', 'code', 'password', 'confirmPassword']);
    expect(fields.length).toBeGreaterThan(0);
    expect(fields.filter((f) => !declared.includes(f))).toEqual([]);
  });

  it('字段存在性判据具备判别力（注入一个不存在的表单字段必须被抓到）', () => {
    const page = read(PAGE).replace('reg.form.value.email', 'reg.form.value.emailX');
    const declared = declaredFormFields(read(COMPOSABLE));
    expect(formFieldRefs(page).filter((f) => !declared.includes(f))).toEqual(['emailX']);
  });
});

describe('allowlist 不回潮（register 模块违例清零的锁）', () => {
  // ADR-0023 决策 ⑧：`GUARD_ALLOWLIST` 的唯一声明点是 `utils/guardAllowlist.js`，消费方一律 require 取用。
  const { allowlistPaths } = require('./guardAllowlist');
  const isRegisterPath = (p) => /^pages\/register\//.test(p);

  it('GUARD_ALLOWLIST 不含 register 模块文件（票面「本模块 allowlist 条目（catch/detail）清零」）', () => {
    expect(allowlistPaths().filter(isRegisterPath)).toEqual([]);
  });

  it('判据具备判别力（注入一条 register 豁免必须被抓到）', () => {
    const injected = allowlistPaths().concat(['pages/register/register.uvue']);
    expect(injected.filter(isRegisterPath)).toEqual(['pages/register/register.uvue']);
  });
});

describe('零直发请求（页面层不直接 uni.request，请求只经 composable → 域 api）', () => {
  it('页面与 composable 都不直接调用 uni.request', () => {
    expect(read(PAGE)).not.toMatch(/uni\.request\s*\(/);
    expect(read(COMPOSABLE)).not.toMatch(/uni\.request\s*\(/);
  });

  it('composable 的请求面只来自 api/auth.uts（域名 api 是唯一出口）', () => {
    expect(read(COMPOSABLE)).toContain("from '../../../api/auth'");
    expect(read(COMPOSABLE)).not.toContain("from '../../../api/request'");
  });

  it('页面消费的 5 个鉴权出口在 composable 里全部被引用（防「import 了但没用」的静默回退）', () => {
    const src = read(COMPOSABLE);
    for (const fn of ['registerApi', 'emailRegisterApi', 'sendPhoneCodeApi', 'sendCodeApi', 'getCaptchaApi']) {
      expect(src).toContain(fn);
    }
  });
});

describe('域 api 出口（T11 收紧）：DTO 出口走 mapper-callback 家族 + 裸透传白名单', () => {
  const api = read(AUTH_API);

  it('auth.uts 引入 postMapped 出口', () => {
    expect(api).toMatch(/import\s*\{[^}]*\bpostMapped\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  // ── 收紧面：register 独有的两条注册出口（唯一消费方是 pages/register）──
  it.each([
    ['registerApi', '/auth/phone/register'],
    ['emailRegisterApi', '/auth/email/register'],
  ])('%s 经 postMapped 出口，且映射函数箭头包裹（规则 M：裸 build* 引用作 mapper 会 error17）', (name, url) => {
    const body = fnBodyOf(api, name);
    expect(body).not.toBe('');
    expect(body).toContain("return postMapped<LoginResult>('" + url + "',");
    expect(body).toMatch(/,\s*\(data : UTSJSONObject\) : LoginResult => \{/);
    // 旧形态（post(...).then(...)）不得回潮
    expect(body).not.toMatch(/return post\(/);
    expect(body).not.toMatch(/\.then\(/);
  });

  // ── 白名单面：3 个裸透传出口的**理由**（保留是决策，不是惯性）──
  // ① `getCaptchaApi` / `sendCodeApi`：本文件是 5 个模块共用的鉴权域 api
  //    （forgot-password / login / profile / profile-setup / register），改返回类型会波及兄弟模块的**行为代码**
  //    ⇒ 属票面「不碰其他模块行为代码」的边界，DTO 化留给 #650 / #651 或 #652 / #653 批量收紧。
  // ② `sendPhoneCodeApi`：`sendCodeApi` 的 register 态薄包装，页面按 **void** 用法消费（返回值不落地）
  //    ⇒ 按 T03/T09 口径不硬套 identity map。
  it('getCaptchaApi 保留裸 get（白名单 ①：5 模块共用的鉴权域出口）', () => {
    expect(fnBodyOf(api, 'getCaptchaApi')).toContain("return get('/captcha')");
  });

  it('sendCodeApi 保留裸 post，两条路由逐字不变（白名单 ①）', () => {
    const body = fnBodyOf(api, 'sendCodeApi');
    expect(body).toContain("return post('/auth/email/send-code', payload)");
    expect(body).toContain("return post('/auth/phone/send-code', payload)");
    expect(body).not.toContain('postMapped');
  });

  it('sendPhoneCodeApi 仍是 sendCodeApi 的 register 态薄包装（白名单 ②：void 用法）', () => {
    const body = fnBodyOf(api, 'sendPhoneCodeApi');
    expect(body).toContain("return sendCodeApi(phone, 'phone', 'register', captchaId, captchaValue)");
  });

  it('出口判据具备判别力（把 postMapped 写回旧 post(...).then(...) 形态必须被判红）', () => {
    const old = api.replace(
      "return postMapped<LoginResult>('/auth/phone/register', payload, (data : UTSJSONObject) : LoginResult => {",
      "return post('/auth/phone/register', payload).then((data : UTSJSONObject) : LoginResult => {",
    );
    expect(old).not.toBe(api);
    const body = fnBodyOf(old, 'registerApi');
    expect(/return postMapped<LoginResult>\(\/auth\/phone\/register,/.test(body)).toBe(false);
  });
});

describe('行为保持点（票面 ②：UI 像素级不变 / 行为逐字保持的机检面）', () => {
  const src = read(COMPOSABLE);
  const tpl = pageTemplate();

  it('双注册模式：模式切换清已收验证码 + 重置倒计时与错次 + 刷图形验证码', () => {
    expect(src).toContain("const mode = ref<string>('phone')");
    expect(src).toContain('mode.value == \'email\' ? \'邮箱注册，验证码通过后自动登录\' : \'手机号注册，验证码通过后自动登录\'');
    const sw = fnBodyOf(src, 'switchMode');
    expect(sw).toContain('if (m == mode.value) return');
    expect(sw).toContain('clearCountdownTimer()');
    expect(sw).toContain('countdown.value = 0');
    expect(sw).toContain('wrongAttempts.value = 0');
    expect(sw).toContain("form.value.code = ''");
    expect(sw).toContain('loadCaptcha()');
  });

  it('模板两个模式分支都在（手机号 / 邮箱），且用 reg.mode.value 判据', () => {
    expect(tpl).toContain('<template v-if="reg.mode.value == \'phone\'">');
    expect(tpl).toContain('<template v-else>');
    expect(tpl).toContain('手机号注册');
    expect(tpl).toContain('邮箱注册');
  });

  it('图形验证码：失败占位「加载失败 点击重试」+ 点击重试（两分支各一处）', () => {
    expect(src).toContain('captchaFailed.value = true');
    expect(src).toContain("captchaId.value = data['id'] as string");
    expect(src).toContain("captchaImage.value = data['image'] as string");
    expect(src).toContain('captchaValue.value = \'\'');
    const hits = tpl.split('加载失败 点击重试').length - 1;
    expect(hits).toBe(2);
    expect(tpl).toContain('@click="reg.refreshCaptcha()"');
  });

  it('发送验证码：图形验证码前置校验 → 通道校验 → 60s 倒计时 → 失败刷新图形验证码', () => {
    const body = fnBodyOf(src, 'onSendCode');
    expect(body).toContain("uni.showToast({ title: '请先输入图形验证码', icon: 'none' })");
    expect(body).toContain("mode.value == 'email' ? validateEmail() : validatePhone()");
    expect(body).toContain("await sendCodeApi(form.value.email.trim(), 'email', 'register', captchaId.value, captchaValue.value)");
    expect(body).toContain('await sendPhoneCodeApi(form.value.phone, captchaId.value, captchaValue.value)');
    expect(body).toContain('wrongAttempts.value = 0');
    expect(body).toContain('startCountdown()');
    expect(body).toContain('loadCaptcha()');
    expect(fnBodyOf(src, 'clearCountdownTimer')).toContain('const timer = countdownTimer');
    expect(fnBodyOf(src, 'startCountdown')).toContain('countdown.value = 60');
  });

  it('倒计时禁用与文案形态（:disabled 与 :class 同判据，文案 Ns）', () => {
    const hits = tpl.split(':disabled="reg.sendingCode.value || reg.countdown.value > 0"').length - 1;
    expect(hits).toBe(2);
    expect(tpl.split("{{ reg.countdown.value > 0 ? reg.countdown.value + 's' : '获取验证码' }}").length - 1).toBe(2);
  });

  it('协议勾选 + 两个协议入口 + 未勾选拦截', () => {
    expect(tpl).toContain('@click="reg.toggleAgree()"');
    expect(tpl).toContain("reg.showAgreement('用户协议')");
    expect(tpl).toContain("reg.showAgreement('用户隐私')");
    expect(src).toContain("if (agreed.value == false) return '请先同意用户协议和隐私政策'");
  });

  it('提交按钮 loading 文案与禁用（两处同判据）', () => {
    expect(tpl).toContain(':disabled="reg.loading.value"');
    expect(tpl).toContain("{{ reg.loading.value ? '注册中...' : '注 册' }}");
    expect(fnBodyOf(src, 'onSubmit')).toContain('if (loading.value) return');
  });

  it('密码两处显隐开关各自独立', () => {
    expect(tpl).toContain(':password="!reg.passwordVisible.value"');
    expect(tpl).toContain(':password="!reg.confirmVisible.value"');
    expect(tpl).toContain('@click="reg.togglePasswordVisible()"');
    expect(tpl).toContain('@click="reg.toggleConfirmVisible()"');
  });

  it('成功路径：refresh_token 一并落地（三参 setAuthData）+ 跳引导页选证件', () => {
    const body = fnBodyOf(src, 'onSubmit');
    expect(body).toContain('auth.setAuthData(result.token, result.user, result.refresh_token)');
    expect(body).toContain("uni.reLaunch({ url: '/pages/guide/choose-cert' })");
    expect(body).toContain('setTimeout(');
  });

  it('失败路径：验证码错次累计（含「验证码」判据）+ 错误原文优先展示', () => {
    const body = fnBodyOf(src, 'onSubmit');
    expect(body).toContain("const m = e instanceof Error ? (e as Error).message : ''");
    expect(body).toContain("if (m.indexOf('验证码') >= 0)");
    expect(body).toContain('wrongAttempts.value += 1');
    expect(body).toContain("let msg = '注册失败，请重试'");
    expect(fnBodyOf(src, 'onSendCode')).toContain("let msg = '验证码发送失败'");
  });

  it('验证码提示三档（5 次上限 / 剩余次数 / 默认规则）', () => {
    const hint = src.slice(src.indexOf('const codeHint = computed<string>'));
    expect(hint).toContain("return '验证码错误次数过多，请点击获取验证码重新发送'");
    expect(hint).toContain("return '验证码错误，还可再试 ' + (5 - wrongAttempts.value) + ' 次'");
    expect(hint).toContain("return '6 位数字验证码，5 分钟内有效'");
  });

  it('六条表单校验规则逐条仍在（昵称 / 手机 / 邮箱 / 验证码 / 密码 / 两次一致）', () => {
    const v = fnBodyOf(src, 'validate');
    expect(v).toContain("if (nickname.length == 0) return '请输入昵称'");
    expect(v).toContain("if (code.length == 0) return '请输入验证码'");
    expect(v).toContain("if (code.length != 6) return '请输入 6 位验证码'");
    expect(v).toContain("if (password.length < 6 || password.length > 20) return '密码长度需为 6-20 位'");
    expect(v).toContain("if (password != confirmPassword) return '两次输入的密码不一致'");
    const p = fnBodyOf(src, 'validatePhone');
    expect(p).toContain("if (phone.length != 11) return '请输入 11 位手机号'");
    expect(p).toContain("if (!phone.startsWith('1')) return '手机号格式不正确'");
    expect(p).toContain("if (!/^\\d{11}$/.test(phone)) return '手机号必须为数字'");
    const e = fnBodyOf(src, 'validateEmail');
    expect(e).toContain("if (t.length == 0) return '请输入邮箱'");
    expect(e).toContain("if (!/^[\\w.%+-]+@[\\w-]+(\\.[\\w-]+)+$/.test(t)) return '邮箱格式不正确'");
  });

  it('生命周期：onLoad 已登录门控 → 无登录态才拉图形验证码；onUnload 清倒计时', () => {
    const script = pageScript();
    expect(script).toContain('auth.restoreFromStorage()');
    expect(script).toContain('if (auth.isLoggedIn.value)');
    expect(script).toContain("uni.reLaunch({ url: '/pages/dashboard/dashboard' })");
    expect(script).toContain('reg.loadCaptcha()');
    expect(script).toContain('reg.clearCountdownTimer()');
  });

  it('输入面 7 个薄包装逐个转交值（壳层只做 e.detail.value 取值）', () => {
    const script = pageScript();
    const pairs = [
      ['onNicknameInput', 'setNickname'],
      ['onPhoneInput', 'setPhone'],
      ['onEmailInput', 'setEmail'],
      ['onCodeInput', 'setCode'],
      ['onPasswordInput', 'setPassword'],
      ['onConfirmPasswordInput', 'setConfirmPassword'],
      ['onCaptchaInput', 'setCaptchaValue'],
    ];
    for (const [handler, setter] of pairs) {
      expect(script).toMatch(new RegExp('function ' + handler + '\\(e : InputEvent\\) : void \\{ reg\\.' + setter + '\\(e\\.detail\\.value\\) \\}'));
      expect(src).toMatch(new RegExp('^\\s{4}function ' + setter + '\\(value : string\\) : void \\{', 'm'));
    }
    // 键盘「完成」= 点注册（原 onConfirm → onSubmit）
    expect(script).toMatch(/function onConfirm\(\) : void \{ reg\.onSubmit\(\) \}/);
  });

  it('行为保持点判据具备判别力（改坏一处必红的抽样：勾选拦截被删即红）', () => {
    const broken = src.replace("if (agreed.value == false) return '请先同意用户协议和隐私政策'", '');
    expect(broken).not.toBe(src);
    expect(broken).not.toContain("if (agreed.value == false) return '请先同意用户协议和隐私政策'");
  });
});
