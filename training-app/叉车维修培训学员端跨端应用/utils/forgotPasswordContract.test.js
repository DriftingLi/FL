/**
 * forgot-password 模块手术契约测试（T12，parent #650 / ADR-0007）
 *
 * 钉住 forgot-password（找回密码页）手术交付的契约：
 * 1) 600 行软预算 / 目录 ≤2 层 / 必需源文件清单：**已由声明面执法**（`utils/modules.js` 的 forgot-password 条目
 *    + `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件只留「手术目标页落袋」这一条（含判别力自检）
 * 2) **`<style>` 块逐字节冻结**：UI 像素级冻结的最强静态证据面 —— sha256 锁定（判据落**文件本体**，
 *    不是「我改的时候很小心」）。术前 / 术后逐字节相同；改一字符即红，改动必须是一次显式决定。
 * 3) composable 接线：页面以显式 import 使用模块私有 composable（唯一拆出物），composable 有
 *    **显式结果类型**（Kotlin error18）+ 函数字段箭头包裹（error17）
 * 4) 页面壳层零自持状态：状态与动作全归 composable，页面只剩模板 / 生命周期 / 薄包装 handler（T09–T11 口径）
 * 5) 模板取值形态锁：`fp.<成员>` 要么跟 `.value`（状态）、要么跟 `(`（动作）—— 漏写 `.value` 会让
 *    `v-if` / `:disabled` **恒真**，编译不报、jest 不报（T11 先例，本票照搬）
 * 6) 成员存在性锁：页面引用的每个 `fp.<成员>` 都在显式结果类型内；`fp.form.value.<字段>` 都在 `ForgotPasswordForm` 内
 * 7) 拆出物零孤儿 / 零死引用（#779「import 了但文件不存在」回归锁）
 * 8) allowlist 不回潮：本模块文件不得出现在 `GUARD_ALLOWLIST`（票面「本模块 allowlist 条目（catch/detail）清零」；
 *    规则 H = catch 参数显式 `: any`、规则 I = `: any` 参数访问 `.detail`）
 * 9) 零直发请求：页面不碰请求层与域 api，请求只经 composable → `api/auth.uts`
 * 10) 域 api 出口（T12 收紧）：`getCaptchaApi` 由裸 `get` 改为 `getMapped` + 显式 DTO `CaptchaResult`，
 *     **三个消费方同 PR 机械迁移**（forgot-password / register / login，引用 #650）；`sendCodeApi` /
 *     `resetPasswordApi` / `sendPhoneCodeApi` 留裸的**白名单与理由**写死在断言旁（void 用法 / 后端 NoData）
 * 11) 行为保持点：切换方式 / 双通道校验 / 图形验证码与验证码的**行序** / 60s 倒计时 / 提交与失败累计 /
 *     三档提示 / 六条校验规则 / 生命周期 / 六个输入面薄包装 —— 逐项仍在，并含与 register 的**四处刻意差异**
 *
 * 本套件是**接线守护**（源码文本 + harness 结构事实，不构成 ③ 门的行为证据，见 `docs/agents/guards.md`）：
 * 它守的是「页面 ↔ composable ↔ 域 api」的接线与模板形态；行为兜底 = ④ 编译门（Kotlin 形态）+ ①a 真机逐页冒烟。
 * 每条判据都带**注入自检**（成对取证：改坏必红 / 真源必不红），见各节末的「判别力」用例。
 */
const crypto = require('crypto');
const h = require('./contractHarness');
const read = h.read;

const PAGE = 'pages/forgot-password/forgot-password.uvue';
const COMPOSABLE = 'pages/forgot-password/composables/useForgotPasswordForm.uts';
const AUTH_API = 'api/auth.uts';
/** T12 一并机械迁移的两个兄弟消费方（共享出口 DTO 化的连带面，见 `api/auth.uts` 文件头） */
const REGISTER_CONSUMER = 'pages/register/composables/useRegisterForm.uts';
const LOGIN_CONSUMER = 'pages/login/login.uvue';

/** 软预算口径（ADR-0007）。模块全量预算已由声明面执法（本模块的 budget 已从 pending 翻成 600） */
const LINE_BUDGET = 600;
/**
 * `<style>` 块的 sha256（术前 = 术后，逐字节）。**这不是「格式锁」而是 UI 冻结的证据锁**：
 * 改它 = 改了这一页的像素面，必须是一次显式决定（并在 PR 里给出 ①a 前后对比）。
 */
const STYLE_SHA256 = '80489de82ad55951232e1dc127886495ae77c4ebf61962c5ed6e31b3a4961af2';

/** 页面块（template / script / style）。模板**有嵌套** `<template v-if>` ⇒ 闭合取最后一个 */
function block(src, tag) {
  const open = src.indexOf(`<${tag}`);
  const close = tag === 'template' ? src.lastIndexOf(`</${tag}>`) : src.indexOf(`</${tag}>`);
  if (open === -1 || close === -1) return '';
  return src.slice(open, close + tag.length + 3);
}
const pageTemplate = () => block(read(PAGE), 'template');
const pageScript = () => block(read(PAGE), 'script');
const pageStyle = () => block(read(PAGE), 'style');
const sha256 = (s) => crypto.createHash('sha256').update(s, 'utf8').digest('hex');

/** 页面 / 任意源文件引用的 composable 成员名（`fp.<name>`） */
function memberRefs(src, alias = 'fp') {
  const re = new RegExp('\\b' + alias + '\\.([A-Za-z_$][\\w$]*)', 'g');
  return [...new Set([...src.matchAll(re)].map((m) => m[1]))];
}

/** 显式结果类型里声明的字段名（返回面的权威清单） */
function declaredResultFields(src) {
  const m = /export type UseForgotPasswordFormResult = \{([\s\S]*?)\n\}/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

/** `ForgotPasswordForm` 类型里声明的字段名 */
function declaredFormFields(src) {
  const m = /export type ForgotPasswordForm = \{([\s\S]*?)\n\}/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

/**
 * 模板取值形态违规：`fp.<成员>` 后面既不跟 `.value`（状态）也不跟 `(`（动作调用）。
 * 合法形态：`fp.mode.value` / `fp.form.value.phone` / `fp.onSendCode()`。
 */
function bareMemberViolations(src, alias = 'fp') {
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

/** `fp.form.value.<字段>` 形态引用的字段名（存在性锁的判据面） */
function formFieldRefs(src, alias = 'fp') {
  const re = new RegExp('\\b' + alias + '\\.form\\.value\\.([A-Za-z_$][\\w$]*)', 'g');
  return [...new Set([...src.matchAll(re)].map((m) => m[1]))];
}

/** 取一个函数声明的体（`export function`（api 层，收在行首 `}`）与 composable 内局部函数（收在 4 空格 `}`）两种都认） */
function fnBodyOf(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const ends = [src.indexOf('\n}', start), src.indexOf('\n    }', start)].filter((x) => x !== -1);
  return ends.length === 0 ? src.slice(start) : src.slice(start, Math.min(...ends));
}

/**
 * 剥掉注释（块注释 + 行注释）。
 * 为什么要有它（T05 两连踩、本票首版又踩一次的同一个坑）：**「零命中锁」会被解释这条锁的注释自身命中** ——
 * 本票 composable 的文件头用 `captchaFailed` 说明「与 register 的刻意差异」，于是 `not.toContain('captchaFailed')`
 * 被自己那段注释判红。判据必须落在**代码**上：注释不是行为，不该有红能力。
 */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

describe('手术目标页落袋锁（模块全量预算 / 目录 ≤2 层 / 必需文件清单已由声明面执法，#1219）', () => {
  it('forgot-password.uvue 从 675 行落到软预算内（手术目标本身也断言，防「只挪注释」的假达标）', () => {
    expect(h.fileLines(PAGE)).toBeLessThanOrEqual(LINE_BUDGET);
    expect(h.fileLines(PAGE)).toBeLessThan(500);
  });

  it('拆出物本身也在预算内（T07 裁定 B：composable 同样计入，防「页面挪成 composable」的假达标）', () => {
    expect(h.fileLines(COMPOSABLE)).toBeLessThanOrEqual(LINE_BUDGET);
  });

  it('预算判据具备判别力（601 行合成输入必须被抓到，600 行放行）', () => {
    const probe = [
      { file: 'synthetic-601', lines: LINE_BUDGET + 1 },
      { file: 'synthetic-600', lines: LINE_BUDGET },
    ];
    expect(probe.filter((x) => x.lines > LINE_BUDGET)).toEqual([{ file: 'synthetic-601', lines: 601 }]);
  });
});

describe('样式块逐字节冻结（UI 像素级不变的最强静态证据面）', () => {
  it('`<style>` 块 sha256 与术前一致（改一字符即红）', () => {
    const style = pageStyle();
    expect(style.length).toBeGreaterThan(4000);
    expect(sha256(style)).toBe(STYLE_SHA256);
  });

  it('CSS 规则数守恒（43 条，与逐字节锁互为冗余判据）', () => {
    const ruleCount = (s) => (s.match(/^\s*\.[a-zA-Z][\w-]*\s*\{/gm) || []).length;
    expect(ruleCount(pageStyle())).toBe(43);
  });

  it('判断力（改一处声明 / 删一条规则都必须与该 sha 不同）', () => {
    const style = pageStyle();
    expect(sha256(style.replace('.footer-link {', '.footer-link-x {'))).not.toBe(STYLE_SHA256);
    expect(sha256(style.replace('#2979ff', '#2979fe'))).not.toBe(STYLE_SHA256);
    const ruleCount = (s) => (s.match(/^\s*\.[a-zA-Z][\w-]*\s*\{/gm) || []).length;
    expect(ruleCount(style.replace('.footer-link {', ''))).toBe(42);
  });
});

describe('composable 接线契约（T12 拆分：显式 import 模块私有 composable）', () => {
  it('页面 import 并调用 useForgotPasswordForm composable', () => {
    const page = read(PAGE);
    expect(page).toContain("from './composables/useForgotPasswordForm'");
    expect(page).toMatch(/const fp = useForgotPasswordForm\(\)/);
  });

  it('composable 声明显式结果类型（Kotlin error18 规避，守护规则 U 同款口径）', () => {
    expect(read(COMPOSABLE)).toMatch(/export function useForgotPasswordForm\(\)\s*:\s*UseForgotPasswordFormResult/);
  });

  it('composable 返回对象显式标注结果类型（as UseForgotPasswordFormResult）', () => {
    expect(read(COMPOSABLE)).toContain('} as UseForgotPasswordFormResult');
  });

  it('返回面字段形态正确：函数字段一律箭头包裹（Kotlin error17 规避，守护规则 T 同款口径）', () => {
    const src = read(COMPOSABLE);
    const ret = src.slice(src.indexOf('    return {'));
    const fields = [...ret.matchAll(/^\s{8}([A-Za-z_$][\w$]*)\s*([:,])/gm)].map((m) => ({ name: m[1], kind: m[2] }));
    // 8 个状态（ref / computed）用速记，13 个动作字段必须 `: (…) => …`
    expect(fields.length).toBe(21);
    expect(fields.filter((f) => f.kind === ',')).toHaveLength(8);
    for (const f of fields.filter((x) => x.kind === ':')) {
      expect(ret).toMatch(new RegExp('^\\s{8}' + f.name + ':\\s*\\(', 'm'));
    }
  });

  it('拆出物零孤儿 / 零死引用（harness 出事实；#779「import 了但文件不存在」回归锁）', () => {
    expect(h.orphanExtracts('forgot-password')).toEqual([]);
    expect(h.deadImports('forgot-password')).toEqual([]);
  });

  it('孤儿判据具备判别力（声明一个不存在的 composable 文件必须被判为 phantom）', () => {
    const injected = JSON.parse(JSON.stringify(h.MODULES));
    injected['forgot-password'].files = injected['forgot-password'].files.map((f) =>
      f.endsWith('useForgotPasswordForm.uts') ? 'pages/forgot-password/composables/useForgotPasswordFormX.uts' : f,
    );
    const report = h.reconcile(injected, h.INFRA);
    expect(report.phantom.map((x) => x.file)).toContain('pages/forgot-password/composables/useForgotPasswordFormX.uts');
  });
});

describe('页面壳层零自持状态 + 模板取值形态锁（T09/T11 口径）', () => {
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

  it('页面引用的每个 composable 成员都在显式结果类型内（④c 编译门抓过的缺口，本锁防复发）', () => {
    const refs = memberRefs(read(PAGE));
    const declared = declaredResultFields(read(COMPOSABLE));
    expect(declared).toHaveLength(21);
    expect(refs.length).toBeGreaterThan(15);
    expect(refs.filter((r) => !declared.includes(r))).toEqual([]);
  });

  it('成员存在性判据具备判别力（注入一个未声明的成员必须被抓到）', () => {
    const page = read(PAGE).replace('fp.codeHint.value', 'fp.codeHintX.value');
    const declared = declaredResultFields(read(COMPOSABLE));
    expect(memberRefs(page).filter((r) => !declared.includes(r))).toEqual(['codeHintX']);
  });

  it('模板里凡 `fp.<成员>` 必跟 `.value`（状态）或 `(`（动作）—— 漏写 .value 会让 v-if / :disabled 恒真', () => {
    const tpl = pageTemplate();
    expect(tpl.length).toBeGreaterThan(2000);
    expect(bareMemberViolations(tpl)).toEqual([]);
  });

  it('形态锁具备判别力（注入裸成员 / 注入无括号动作必须各判一条）', () => {
    expect(bareMemberViolations('<view v-if="fp.codeHint">x</view>')).toEqual(['codeHint']);
    expect(bareMemberViolations('<button @click="fp.onSubmit">x</button>')).toEqual(['onSubmit']);
    expect(bareMemberViolations('<view v-if="fp.codeHint.value">x</view>')).toEqual([]);
    expect(bareMemberViolations('<button @click="fp.onSubmit()">x</button>')).toEqual([]);
    expect(bareMemberViolations('<input :value="fp.form.value.phone" />')).toEqual([]);
  });

  it('`fp.form.value.<字段>` 引用的每个字段都在 ForgotPasswordForm 内', () => {
    const fields = formFieldRefs(read(PAGE));
    const declared = declaredFormFields(read(COMPOSABLE));
    expect(declared).toEqual(['phone', 'email', 'code', 'password', 'confirmPassword']);
    expect(fields.length).toBeGreaterThan(0);
    expect(fields.filter((f) => !declared.includes(f))).toEqual([]);
  });

  it('字段存在性判据具备判别力（注入一个不存在的表单字段必须被抓到）', () => {
    const page = read(PAGE).replace('fp.form.value.email', 'fp.form.value.emailX');
    const declared = declaredFormFields(read(COMPOSABLE));
    expect(formFieldRefs(page).filter((f) => !declared.includes(f))).toEqual(['emailX']);
  });
});

describe('allowlist 不回潮（票面：本模块 allowlist 条目（catch/detail）清零）', () => {
  // ADR-0023 决策 ⑧：`GUARD_ALLOWLIST` 的唯一声明点是 `utils/guardAllowlist.js`，消费方一律 require 取用。
  const { allowlistPaths } = require('./guardAllowlist');
  const isModulePath = (p) => /^pages\/forgot-password\//.test(p);

  it('GUARD_ALLOWLIST 不含本模块文件（规则 H catch : any / 规则 I .detail 直取 均零存量）', () => {
    expect(allowlistPaths().filter(isModulePath)).toEqual([]);
  });

  it('判据具备判别力（注入一条本模块豁免必须被抓到）', () => {
    const injected = allowlistPaths().concat(['pages/forgot-password/forgot-password.uvue']);
    expect(injected.filter(isModulePath)).toEqual(['pages/forgot-password/forgot-password.uvue']);
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

  it('页面消费的 3 个鉴权出口在 composable 里全部被引用（防「import 了但没用」的静默回退）', () => {
    const src = read(COMPOSABLE);
    for (const fn of ['resetPasswordApi', 'sendCodeApi', 'getCaptchaApi']) {
      expect(src).toContain(fn);
    }
  });
});

describe('域 api 出口（T12 收紧）：getCaptchaApi DTO 化 + 共享出口的三个消费方同 PR 迁移', () => {
  const api = read(AUTH_API);

  it('auth.uts 引入 getMapped 出口', () => {
    expect(api).toMatch(/import\s*\{[^}]*\bgetMapped\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('CaptchaResult 显式 DTO 与 buildCaptcha 手动映射（ADR-0003：DTO 声明在域 api 内）', () => {
    expect(api).toMatch(/export type CaptchaResult = \{\s*\n\s*id : string\s*\n\s*image : string\s*\n\}/);
    const build = fnBodyOf(api, 'buildCaptcha');
    expect(build).toContain("const id = obj['id'] as string");
    expect(build).toContain("const image = obj['image'] as string");
  });

  it('getCaptchaApi 经 getMapped 出口，且映射函数箭头包裹（规则 M：裸 build* 引用作 mapper 会 error17）', () => {
    const body = fnBodyOf(api, 'getCaptchaApi');
    expect(body).not.toBe('');
    expect(body).toContain('return getMapped<CaptchaResult>(');
    expect(body).toContain("'/captcha'");
    expect(body).toMatch(/,\s*\(data : UTSJSONObject\) : CaptchaResult => \{?/);
    // 旧形态（裸 get）不得回潮
    expect(body).not.toContain("return get('/captcha')");
    expect(body).toMatch(/Promise<CaptchaResult>/);
  });

  it.each([
    ['forgot-password composable', COMPOSABLE],
    ['register composable', REGISTER_CONSUMER],
    ['login 页', LOGIN_CONSUMER],
  ])('共享出口的三个消费方同 PR 机械迁移：%s 按 DTO 取值（data.id / data.image）', (_name, file) => {
    const src = read(file);
    expect(src).toContain('const data = await getCaptchaApi()');
    expect(src).toContain('captchaId.value = data.id');
    expect(src).toContain('captchaImage.value = data.image');
    // 迁移前的裸索引形态不得残留（DTO 是 data class，索引取值编译不过）
    expect(src).not.toContain("data['id'] as string");
    expect(src).not.toContain("data['image'] as string");
  });

  // ── 白名单面：3 个裸透传出口的**理由**（保留是决策，不是惯性）──
  // ① `sendCodeApi`：三个消费方（forgot-password / login / register）都按 **void** 用法消费（返回值不落地）
  //    ⇒ 按 T03/T09 口径不硬套 identity map。
  // ② `sendPhoneCodeApi`：`sendCodeApi` 的 register 态薄包装，同样 void 用法。
  // ③ `resetPasswordApi`：本模块独占，但后端 apitypes 标 `NoData: true`
  //    （`backend/internal/apitypes/domains.go` 的 `{Method:"POST", Path:"/auth/*/reset-password", NoData: true}`）
  //    ⇒ 没有可 DTO 化的载荷。
  it('sendCodeApi 保留裸 post，两条路由逐字不变（白名单 ①：void 用法）', () => {
    const body = fnBodyOf(api, 'sendCodeApi');
    expect(body).toContain("return post('/auth/email/send-code', payload)");
    expect(body).toContain("return post('/auth/phone/send-code', payload)");
    expect(body).not.toContain('postMapped');
  });

  it('sendPhoneCodeApi 仍是 sendCodeApi 的 register 态薄包装（白名单 ②：void 用法）', () => {
    const body = fnBodyOf(api, 'sendPhoneCodeApi');
    expect(body).toContain("return sendCodeApi(phone, 'phone', 'register', captchaId, captchaValue)");
  });

  it('resetPasswordApi 保留裸 post（白名单 ③：后端 NoData，无载荷可映射）', () => {
    const body = fnBodyOf(api, 'resetPasswordApi');
    expect(body).toContain("return post('/auth/phone/reset-password', payload)");
    expect(body).toContain("return post('/auth/email/reset-password', payload)");
    expect(body).not.toContain('postMapped');
  });

  it('出口判据具备判别力（把 getCaptchaApi 写回旧裸形态必须被判红）', () => {
    const old = api.replace(
      "return getMapped<CaptchaResult>('/captcha', null, (data : UTSJSONObject) : CaptchaResult => buildCaptcha(data))",
      "return get('/captcha')",
    );
    expect(old).not.toBe(api);
    const body = fnBodyOf(old, 'getCaptchaApi');
    expect(body).not.toContain('getMapped<CaptchaResult>(');
    expect(body).toContain("return get('/captcha')");
  });
});

describe('行为保持点（票面 ②：UI 像素级不变 / 行为逐字保持的机检面）', () => {
  const src = read(COMPOSABLE);
  const tpl = pageTemplate();

  it('切换找回方式：清倒计时与错次 + 刷图形验证码；**不**清 form.code（与 register 的刻意差异 ①）', () => {
    expect(src).toContain("const mode = ref<string>('phone')");
    const sw = fnBodyOf(src, 'switchMode');
    expect(sw).toContain('if (m == mode.value) return');
    expect(sw).toContain('clearCountdownTimer()');
    expect(sw).toContain('countdown.value = 0');
    expect(sw).toContain('wrongAttempts.value = 0');
    expect(sw).toContain('loadCaptcha()');
    // register 的 switchMode 会 `form.value.code = ''`；本页术前就不清，逐字搬迁不「顺手对齐」
    expect(sw).not.toContain('form.value.code');
  });

  it('模板两个模式分支都在（手机号 / 邮箱），且用 fp.mode.value 判据', () => {
    expect(tpl).toContain('<template v-if="fp.mode.value == \'phone\'">');
    expect(tpl).toContain('<template v-else>');
    expect(tpl).toContain('手机号找回');
    expect(tpl).toContain('邮箱找回');
  });

  it('图形验证码行在验证码行**之前**（本页与 register 的行序相反，逐字保持）', () => {
    expect(tpl.indexOf('captcha-row')).toBeGreaterThan(0);
    expect(tpl.indexOf('code-row')).toBeGreaterThan(0);
    expect(tpl.indexOf('captcha-row')).toBeLessThan(tpl.indexOf('code-row'));
  });

  it('图形验证码：取 DTO 两字段 + 清空输入；**无**失败占位态（与 register 的刻意差异 ②）', () => {
    const body = fnBodyOf(src, 'loadCaptcha');
    expect(body).toContain('const data = await getCaptchaApi()');
    expect(body).toContain('captchaId.value = data.id');
    expect(body).toContain('captchaImage.value = data.image');
    expect(body).toContain("captchaValue.value = ''");
    expect(body).toContain("console.error('[forgot-password] load captcha failed:', e)");
    // register 有 captchaFailed 占位；本页术前没有 ⇒ 不得引入（判据落**代码**，注释里提到它不算）
    expect(stripComments(src)).not.toContain('captchaFailed');
    expect(tpl).not.toContain('加载失败 点击重试');
    expect(tpl).toContain('@click="fp.refreshCaptcha()"');
  });

  it('注释剥离判据具备判别力（代码里出现该 token 必须被抓到，注释里出现不算）', () => {
    expect(stripComments('// captchaFailed 只是注释\nconst x = 1')).not.toContain('captchaFailed');
    expect(stripComments('/* captchaFailed 块注释 */\nconst x = 1')).not.toContain('captchaFailed');
    expect(stripComments('const captchaFailed = ref<boolean>(false)')).toContain('captchaFailed');
  });

  it('发送验证码：图形验证码前置校验 → 双通道校验 → purpose=reset_password → 刷码 + 清错次 + 倒计时', () => {
    const body = fnBodyOf(src, 'onSendCode');
    expect(body).toContain("uni.showToast({ title: '请先输入图形验证码', icon: 'none' })");
    expect(body).toContain('const err = validatePhone()');
    expect(body).toContain('const err = validateEmail()');
    expect(body).toContain('await sendCodeApi(target, mode.value, \'reset_password\', captchaId.value, captchaValue.value)');
    expect(body).toContain('wrongAttempts.value = 0');
    expect(body).toContain('startCountdown()');
    expect(body).toContain('loadCaptcha()');
    expect(fnBodyOf(src, 'clearCountdownTimer')).toContain('const timer = countdownTimer');
    expect(fnBodyOf(src, 'startCountdown')).toContain('countdown.value = 60');
    expect(body).toContain("let msg = '验证码发送失败'");
  });

  it('倒计时禁用与文案形态（本页只有一行验证码行 ⇒ 各 1 处）', () => {
    expect(tpl.split(':disabled="fp.sendingCode.value || fp.countdown.value > 0"').length - 1).toBe(1);
    expect(tpl.split("{{ fp.countdown.value > 0 ? fp.countdown.value + 's' : '获取验证码' }}").length - 1).toBe(1);
  });

  it('提交按钮 loading 文案与禁用；且 onSubmit **无** loading 前置守卫（与 register 的刻意差异 ③）', () => {
    expect(tpl).toContain(':disabled="fp.loading.value"');
    expect(tpl).toContain("{{ fp.loading.value ? '提交中...' : '重置密码' }}");
    expect(fnBodyOf(src, 'onSubmit')).not.toContain('if (loading.value) return');
  });

  it('成功路径：提示「密码已重置，请使用新密码登录」→ setTimeout 1s → redirectTo 登录页（不自动登录）', () => {
    const body = fnBodyOf(src, 'onSubmit');
    expect(body).toContain("uni.showToast({ title: '密码已重置，请使用新密码登录', icon: 'success', duration: 1500 })");
    expect(body).toContain('setTimeout(');
    expect(body).toContain("uni.redirectTo({ url: '/pages/login/login' })");
    expect(body).not.toContain('setAuthData');
    expect(fnBodyOf(src, 'goLogin')).toContain("uni.redirectTo({ url: '/pages/login/login' })");
  });

  it('失败路径：验证码错次累计（含「验证码」判据）+ 错误原文优先展示 + 默认文案', () => {
    const body = fnBodyOf(src, 'onSubmit');
    expect(body).toContain("const m = e instanceof Error ? (e as Error).message : ''");
    expect(body).toContain("if (m.indexOf('验证码') >= 0)");
    expect(body).toContain('wrongAttempts.value += 1');
    expect(body).toContain("let msg = '重置失败，请重试'");
  });

  it('验证码提示三档（5 次上限 / 剩余次数 / 默认规则）', () => {
    const hint = src.slice(src.indexOf('const codeHint = computed<string>'));
    expect(hint).toContain("return '验证码错误次数过多，请点击获取验证码重新发送'");
    expect(hint).toContain("return '验证码错误，还可再试 ' + (5 - wrongAttempts.value) + ' 次'");
    expect(hint).toContain("return '6 位数字验证码，5 分钟内有效'");
  });

  it('六条表单校验规则逐条仍在（手机三档 / 邮箱两档 / 验证码两档 / 密码下限 / 两次一致）', () => {
    const v = fnBodyOf(src, 'validate');
    expect(v).toContain("if (code.length == 0) return '请输入验证码'");
    expect(v).toContain("if (code.length != 6) return '请输入 6 位验证码'");
    expect(v).toContain("if (password.length < 6) return '密码至少 6 位'");
    expect(v).toContain("if (password != confirmPassword) return '两次输入的密码不一致'");
    const p = fnBodyOf(src, 'validatePhone');
    expect(p).toContain("if (phone.length != 11) return '请输入 11 位手机号'");
    expect(p).toContain("if (!phone.startsWith('1')) return '手机号格式不正确'");
    expect(p).toContain("if (!/^\\d{11}$/.test(phone)) return '手机号必须为数字'");
    const e = fnBodyOf(src, 'validateEmail');
    expect(e).toContain("if (t.indexOf('@') <= 0 || t.indexOf('.') <= 0) return '邮箱格式不正确'");
  });

  it('密码校验缺上限 = 术前既有的文案/校验错配，原样保留并另立 #1262（本票不顺手改）', () => {
    // 页面文案与 :maxlength 承诺 6-20，后端 code_service.go 也是 6-20，唯独客户端只判下限。
    // 逐字搬迁 ⇒ 本票**保持**这个缺口，缺陷另立 https://github.com/DriftingLi/FL/issues/1262。
    expect(fnBodyOf(src, 'validate')).not.toContain('password.length > 20');
    expect(tpl).toContain('placeholder="新密码（6-20 位）"');
    expect(tpl.split(':maxlength="32"').length - 1).toBe(2);
  });

  it('生命周期：onLoad 拉图形验证码 / onUnload 清倒计时（uni 钩子不进 composable）', () => {
    const script = pageScript();
    expect(script).toContain('fp.loadCaptcha()');
    expect(script).toContain('fp.clearCountdownTimer()');
    expect(script).toContain("from '@dcloudio/uni-app'");
    expect(src).not.toContain('onLoad(');
    expect(src).not.toContain('onUnload(');
  });

  it('输入面 6 个薄包装逐个转交值（壳层只做 e.detail.value 取值）+ onConfirm → onSubmit', () => {
    const script = pageScript();
    const pairs = [
      ['onPhoneInput', 'setPhone'],
      ['onEmailInput', 'setEmail'],
      ['onCodeInput', 'setCode'],
      ['onPasswordInput', 'setPassword'],
      ['onConfirmPasswordInput', 'setConfirmPassword'],
      ['onCaptchaInput', 'setCaptchaValue'],
    ];
    for (const [handler, setter] of pairs) {
      expect(script).toMatch(new RegExp('function ' + handler + '\\(e : InputEvent\\) : void \\{ fp\\.' + setter + '\\(e\\.detail\\.value\\) \\}'));
      expect(src).toMatch(new RegExp('^\\s{4}function ' + setter + '\\(value : string\\) : void \\{', 'm'));
    }
    // 键盘「完成」= 点重置密码（原 onConfirm → onSubmit）
    expect(script).toMatch(/function onConfirm\(\) : void \{ fp\.onSubmit\(\) \}/);
  });

  it('模板 @confirm 绑定数守恒（4 处：手机号 / 邮箱 / 验证码 / 再次输入新密码；新密码框术前就没有）', () => {
    expect(tpl.split('@confirm="onConfirm"').length - 1).toBe(4);
  });

  it('行为保持点判据具备判别力（改坏一处必红的抽样：错次累计被删即红）', () => {
    const broken = src.replace('wrongAttempts.value += 1', '');
    expect(broken).not.toBe(src);
    expect(broken).not.toContain('wrongAttempts.value += 1');
  });
});
