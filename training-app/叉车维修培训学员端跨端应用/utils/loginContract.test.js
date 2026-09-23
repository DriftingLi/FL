/**
 * login 模块手术契约测试（T13，parent #651 / ADR-0007）
 *
 * 钉住 login（登录页）手术交付的契约：
 * 1) 600 行软预算 / 目录 ≤2 层 / 必需源文件清单：**已由声明面执法**（`utils/modules.js` 的 login 条目
 *    从 `pending` 翻成 `BUDGET` + `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），
 *    本文件只留「手术目标页落袋」这一条（含判别力自检）
 * 2) **模板块与 `<style>` 块逐字节冻结（sha256 锁）**：本票与 T09–T12 的关键差异 —— 页面模板
 *    **一个字都没改**（解构而非 `reg.*` 前缀），所以判据可以是整块 sha 而不是「规则数守恒」。
 *    这是票面 ②「UI 像素级不变」最强的静态证据面：改一字符即红，改它必须是一次显式决定。
 * 3) composable 接线：两个模块私有 composable 显式 import、各有显式结果类型（error18）+
 *    函数字段箭头包裹（error17）+ 返回面 `as` 标注
 * 4) **生物识别实例单例注入锁（本票新增，防静默失效）**：`useBiometric()` 每次调用返回**新** ref
 *    （非单例），故页面只允许调用一次并把同一实例注入两个 composable。若两处各取一份，
 *    `checkSupport()` 写入的 `isSupported` 与模板读取的就不是同一个 ⇒ 快捷登录入口**静默不显示**，
 *    编译不报、jest 不报 —— 这类「静默失效」正是形态锁该拦住的东西（T11 的 `.value` 锁同款思路）。
 * 5) 返回面 ↔ 解构面双向对账 + 模板根标识符存在性锁：页面引用的符号必须都在 composable
 *    显式结果类型内（防「返回面漏字段」），反向不得有未被页面消费的字段（防死返回面），
 *    模板绑定表达式里的根标识符必须都在页面作用域内（防解构改名 / 漏解构）
 * 6) 门控面**只搬不改**：四个门控动作逐字位于 `useBiometricGate.uts`（其行为断言强度由
 *    `utils/loginGating.test.js` 原样守着，本文件只锁「归属唯一 + 页面不再自持」），
 *    表单与提交面位于 `useLoginForm.uts`
 * 7) 拆出物零孤儿 / 零死引用（#779 回归锁）
 * 8) allowlist 不回潮：login 模块文件不得出现在 `GUARD_ALLOWLIST`（票面 ⑦「本模块 allowlist
 *    条目清零」—— 手术前本模块就是零，故此条是**不回潮**锁而非清零动作）
 * 9) 零直发请求 + 域 api 出口：页面不碰 api 层；`useLoginForm` 只经 `api/auth.uts`；
 *    `getCaptchaApi` 的 DTO 化与 `sendCodeApi` 的裸透传白名单**理由**在 `#650` 已落锁
 *    （`utils/forgotPasswordContract.test.js` / `registerContract.test.js`），本文件只补登录侧消费面
 * 10) 行为保持点：四模式分支 / 模式切换清零 / 图形验证码前置与失败刷新 / 60s 倒计时 /
 *    四通道校验文案逐字 / 协议勾选的 password 例外 / 成功后跳转两分支 / 招聘者入口 /
 *    键盘「完成」= 提交 —— 逐项仍在
 *
 * 本套件是**接线守护**（源码文本 + harness 结构事实，不构成 ③ 门的行为证据，见 `docs/agents/guards.md`）：
 * 它守的是「页面 ↔ 两个 composable ↔ 域 api」的接线与模板冻结；行为兜底 = ④ 编译门（Kotlin 形态）
 * + ①a 真机逐页冒烟（含生物识别入口态）。每条判据都带**注入自检**（成对取证：改坏必红 / 真源必不红）。
 */
const crypto = require('crypto');
const h = require('./contractHarness');
const read = h.read;

const PAGE = 'pages/login/login.uvue';
const FORM = 'pages/login/composables/useLoginForm.uts';
const GATE = 'pages/login/composables/useBiometricGate.uts';
const AUTH_API = 'api/auth.uts';

/** 软预算口径（ADR-0007）。模块全量预算已由声明面执法（login 的 budget 已从 pending 翻成 600） */
const LINE_BUDGET = 600;

/**
 * 术前 = 术后的块级 sha256（术前值取自 `origin/master` 的 `pages/login/login.uvue`，逐字节）。
 * 模板与样式**一行未动**是本票的硬交付：解构让模板绑定名保持原样，所以这两把锁可以是整块 sha。
 */
const TEMPLATE_SHA256 = '042b6fb3c06dafa61181be2ae0d2cddeb2ea23f7e8a121b613ed5e04e40f59c1';
const STYLE_SHA256 = '04fa44fccbd995e237742c5f0ceefee0d8815d974f5ef0cb4983d69eb161a20a';

/** 页面块（template / script / style）。模板**有嵌套** `<template v-if>` ⇒ 闭合取最后一个 */
function block(src, tag) {
  const open = src.indexOf(`<${tag}`);
  const close = tag === 'template' ? src.lastIndexOf(`</${tag}>`) : src.indexOf(`</${tag}>`);
  if (open === -1 || close === -1) return '';
  return src.slice(open, close + tag.length + 3);
}
const pageTemplate = () => block(read(PAGE), 'template');
const pageScript = () => block(read(PAGE), 'script');
const sha = (s) => crypto.createHash('sha256').update(s).digest('hex');

/** 取函数体：靶子既有 `export function`（体收在行首 `}`）也有 composable 内局部函数（4 空格收尾） */
function fnBodyOf(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const ends = [src.indexOf('\n}', start), src.indexOf('\n    }', start)].filter((x) => x !== -1);
  return ends.length === 0 ? src.slice(start) : src.slice(start, Math.min(...ends));
}

/** 显式结果类型里声明的字段名（返回面的权威清单） */
function declaredResultFields(src, typeName) {
  const m = new RegExp('export type ' + typeName + ' = \\{([\\s\\S]*?)\\n\\}').exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

/** 页面里 `const { … } = useXxx(…)` 解构出来的名字（支持多行与单行两种写法） */
function destructuredNames(src, callName) {
  const at = src.indexOf('= ' + callName);
  if (at === -1) return [];
  const open = src.lastIndexOf('{', at);
  const close = src.indexOf('}', open);
  return src
    .slice(open + 1, close)
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter((s) => /^[A-Za-z_$][\w$]*$/.test(s));
}

/**
 * 模板绑定表达式的**根标识符**：只扫 `{{ }}` 与指令/绑定属性的表达式，
 * 先去字符串字面量、再去对象字面量的键（`{ height: x }` 的 `height`），
 * 最后取不被 `.` 前缀的标识符 ⇒ 得到「页面作用域必须提供哪些名字」。
 */
function templateRoots(tpl) {
  const exprs = [...tpl.matchAll(/\{\{([\s\S]*?)\}\}/g), ...tpl.matchAll(/(?:v-if|v-else-if|v-show|@[\w.]+|:[\w-]+)\s*=\s*"([^"]*)"/g)].map((m) => m[1]);
  const kw = new Set(['true', 'false', 'null', 'undefined', 'new', 'typeof', 'in', 'of', 'if', 'else', 'return']);
  const out = new Set();
  for (const raw of exprs) {
    const e = raw.replace(/'[^']*'/g, "''").replace(/"[^"]*"/g, '""').replace(/[A-Za-z_$][\w$]*\s*:(?!=)/g, '');
    for (const m of e.matchAll(/(?<![\w$.])([A-Za-z_$][\w$]*)/g)) {
      if (!kw.has(m[1])) out.add(m[1]);
    }
  }
  return [...out].sort();
}

/** 页面作用域：两个 composable 的解构名 + 壳层自有函数 + `biometric` 实例 */
function pageScope() {
  const page = read(PAGE);
  const names = new Set([...destructuredNames(page, 'useLoginForm'), ...destructuredNames(page, 'useBiometricGate')]);
  for (const m of page.matchAll(/function ([A-Za-z_$][\w$]*)\(/g)) names.add(m[1]);
  for (const m of page.matchAll(/const ([A-Za-z_$][\w$]*)\s*=/g)) names.add(m[1]);
  return names;
}

describe('手术目标页落袋锁（模块全量预算 / 目录 ≤2 层 / 必需文件清单已由声明面执法）', () => {
  it('login.uvue 从 977 行落到软预算内（手术目标本身也断言，防「只挪注释」的假达标）', () => {
    expect(h.fileLines(PAGE)).toBeLessThanOrEqual(LINE_BUDGET);
    expect(h.fileLines(PAGE)).toBeLessThan(600);
    expect(h.fileLines(PAGE)).toBeLessThan(700);
  });

  it('两个拆出物同样在软预算内（ADR-0007：预算算的是模块内**全部**源文件）', () => {
    for (const f of [FORM, GATE]) {
      expect(h.fileLines(f)).toBeGreaterThan(100);
      expect(h.fileLines(f)).toBeLessThanOrEqual(LINE_BUDGET);
    }
  });

  it('模板块与 <style> 块**逐字节**未变（票面 ②「UI 像素级不变」的 sha 锁）', () => {
    const tpl = pageTemplate();
    const style = block(read(PAGE), 'style');
    expect(tpl.length).toBeGreaterThan(8000);
    expect(style.length).toBeGreaterThan(5000);
    expect(sha(tpl)).toBe(TEMPLATE_SHA256);
    expect(sha(style)).toBe(STYLE_SHA256);
  });

  it('样式入口形态合规：`<style lang="scss">`（移动端 AGENTS.md 硬约定；T12 的「per-module 断言只加测试不改页面」同款）', () => {
    expect(read(PAGE)).toContain('<style lang="scss">');
    // 判别力：把属性摘掉必须被抓到（T12 登记的是「本页缺」，本页不缺 ⇒ 这里钉「不许丢」）
    expect(read(PAGE).replace('<style lang="scss">', '<style>')).not.toContain('<style lang="scss">');
  });

  it('sha 锁具备判别力（模板 / 样式各改一个字符都必须换 sha）', () => {
    const tpl = pageTemplate();
    const style = block(read(PAGE), 'style');
    expect(sha(tpl.replace('账号密码登录', '账号密码登录X'))).not.toBe(TEMPLATE_SHA256);
    expect(sha(style.replace('.login-page {', '.login-pageY {'))).not.toBe(STYLE_SHA256);
    // 反面对照：原样重算必等（防空块恒真的假绿）
    expect(sha(tpl)).toBe(TEMPLATE_SHA256);
  });
});

describe('composable 接线契约（T13 拆分：两个模块私有 composable 显式 import）', () => {
  it('页面显式 import 两个 composable 并各调用一次', () => {
    const page = read(PAGE);
    expect(page).toContain("from './composables/useLoginForm'");
    expect(page).toContain("from './composables/useBiometricGate'");
    expect(page).toContain('= useLoginForm(biometric)');
    expect(page).toContain('= useBiometricGate(biometric, username, password, rememberMe)');
  });

  it('两个 composable 都有显式返回类型与 `as` 标注（Kotlin error18 规避）', () => {
    expect(read(FORM)).toMatch(/export function useLoginForm\(biometric : UseBiometricResult\) : UseLoginFormResult \{/);
    expect(read(FORM)).toContain('} as UseLoginFormResult');
    expect(read(GATE)).toMatch(/export function useBiometricGate\(/);
    expect(read(GATE)).toContain(') : UseBiometricGateResult {');
    expect(read(GATE)).toContain('} as UseBiometricGateResult');
  });

  it('返回面函数字段一律箭头包裹（Kotlin error17 规避，守护规则 T 同款口径）', () => {
    for (const [file, typeName] of [[FORM, 'UseLoginFormResult'], [GATE, 'UseBiometricGateResult']]) {
      const src = read(file);
      const ret = src.slice(src.indexOf('    return {'));
      const fields = [...ret.matchAll(/^\s{8}([A-Za-z_$][\w$]*)\s*([:,])/gm)].map((m) => ({ name: m[1], kind: m[2] }));
      const declared = declaredResultFields(src, typeName);
      expect(fields.length).toBe(declared.length);
      for (const f of fields.filter((x) => x.kind === ':')) {
        expect(ret).toMatch(new RegExp('^\\s{8}' + f.name + ':\\s*\\(', 'm'));
      }
    }
  });

  it('生物识别实例**单例注入**：页面只调 useBiometric() 一次，composable 只收参数不自取', () => {
    const page = read(PAGE);
    expect([...page.matchAll(/=\s*useBiometric\(\)/g)]).toHaveLength(1);
    expect(page).toContain('const biometric = useBiometric()');
    for (const f of [FORM, GATE]) {
      const src = read(f);
      expect(src).not.toMatch(/=\s*useBiometric\(\)/);
      // 只允许 `import type { UseBiometricResult }`，不得 import 工厂本身
      expect(src).toContain("import type { UseBiometricResult } from '../../../composables/useBiometric'");
      expect(src).not.toMatch(/import \{[^}]*\buseBiometric\b[^}]*\} from/);
      expect(src).toMatch(/biometric : UseBiometricResult/);
    }
  });

  it('单例注入判据具备判别力（页面多取一份实例 / composable 自取都必须被抓到）', () => {
    const page = read(PAGE);
    const doubled = page.replace('const biometric = useBiometric()', 'const biometric = useBiometric()\n    const extra = useBiometric()');
    expect([...doubled.matchAll(/=\s*useBiometric\(\)/g)]).toHaveLength(2);
    const selfTaken = read(GATE).replace("import type { UseBiometricResult } from '../../../composables/useBiometric'", "import { useBiometric } from '../../../composables/useBiometric'");
    expect(selfTaken).toMatch(/import \{[^}]*\buseBiometric\b[^}]*\} from/);
  });

  it('拆出物零孤儿 / 零死引用（harness 出事实；#779「import 了但文件不存在」回归锁）', () => {
    expect(h.orphanExtracts('login')).toEqual([]);
    expect(h.deadImports('login')).toEqual([]);
  });

  it('孤儿判据具备判别力（声明一个不存在的 composable 文件必须被判为幽灵）', () => {
    const injected = JSON.parse(JSON.stringify(h.MODULES));
    injected.login.files = injected.login.files.map((f) => (f.endsWith('useLoginForm.uts') ? 'pages/login/composables/useLoginFormX.uts' : f));
    const report = h.reconcile(injected, h.INFRA);
    expect(report.phantom.map((x) => x.file)).toContain('pages/login/composables/useLoginFormX.uts');
  });
});

describe('页面壳层零自持状态 + 模板符号存在性（T09–T12 口径；本票用解构故无需 .value 形态锁）', () => {
  it('页面 script 不自持任何状态与计算属性（无 ref< / reactive( / computed<）', () => {
    const script = pageScript();
    expect(script).not.toMatch(/\bref</);
    expect(script).not.toMatch(/\breactive\(/);
    expect(script).not.toMatch(/\bcomputed</);
  });

  it('uni 生命周期只在页面壳层注册（composable 不注册，T06/T07/T08 口径）', () => {
    const script = pageScript();
    for (const hook of ['onLoad(', 'onShow(', 'onUnload(']) expect(script).toContain(hook);
    for (const f of [FORM, GATE]) {
      const src = read(f);
      expect(src).not.toMatch(/^\s*onLoad\(/m);
      expect(src).not.toMatch(/^\s*onUnload\(/m);
      expect(src).not.toContain("@dcloudio/uni-app");
    }
  });

  it('页面消费的每个符号都在对应 composable 的显式结果类型内（防返回面漏字段）', () => {
    const page = read(PAGE);
    const formNames = destructuredNames(page, 'useLoginForm');
    const gateNames = destructuredNames(page, 'useBiometricGate');
    const formDeclared = declaredResultFields(read(FORM), 'UseLoginFormResult');
    const gateDeclared = declaredResultFields(read(GATE), 'UseBiometricGateResult');
    expect(formNames.length).toBeGreaterThan(25);
    expect(gateNames).toHaveLength(gateDeclared.length);
    expect(formNames.filter((n) => !formDeclared.includes(n))).toEqual([]);
    expect(gateNames.filter((n) => !gateDeclared.includes(n))).toEqual([]);
  });

  it('反向对账：显式结果类型里没有「页面不消费」的死字段（T05 消费面口径）', () => {
    const page = read(PAGE);
    const formDeclared = declaredResultFields(read(FORM), 'UseLoginFormResult');
    const gateDeclared = declaredResultFields(read(GATE), 'UseBiometricGateResult');
    expect(formDeclared.filter((n) => !destructuredNames(page, 'useLoginForm').includes(n))).toEqual([]);
    expect(gateDeclared.filter((n) => !destructuredNames(page, 'useBiometricGate').includes(n))).toEqual([]);
  });

  it('存在性对账具备判别力（解构名改一个 / 结果类型删一个字段都必须被抓到）', () => {
    const page = read(PAGE);
    const declared = declaredResultFields(read(FORM), 'UseLoginFormResult');
    const renamed = destructuredNames(page.replace('        captchaFailed,', '        captchaFailedX,'), 'useLoginForm');
    expect(renamed.filter((n) => !declared.includes(n))).toEqual(['captchaFailedX']);
    const dropped = declared.filter((n) => !destructuredNames(page.replace('        countdown,', ''), 'useLoginForm').includes(n));
    expect(dropped).toEqual(['countdown']);
  });

  it('模板绑定表达式里的根标识符全部由页面作用域提供（防解构改名 / 漏解构）', () => {
    const roots = templateRoots(pageTemplate());
    const scope = pageScope();
    expect(roots.length).toBeGreaterThan(8);
    expect(roots.filter((r) => !scope.has(r))).toEqual([]);
  });

  it('模板根标识符判据具备判别力（把一个绑定改成不存在的名字必须被抓到）', () => {
    const broken = pageTemplate().replace('{{ quickLoginLabel }}', '{{ quickLoginLabelX }}');
    const roots = templateRoots(broken);
    expect(roots.filter((r) => !pageScope().has(r))).toEqual(['quickLoginLabelX']);
  });

  it('壳层输入 handler 仍是 7 个 InputEvent 薄包装（`InputEvent` 只出现在 .uvue 侧，T11 形态决定 ②）', () => {
    const script = pageScript();
    const handlers = ['onPhoneInput', 'onPhoneCodeInput', 'onUsernameInput', 'onPasswordInput', 'onEmailInput', 'onEmailCodeInput', 'onCaptchaInput'];
    const targets = ['phone', 'phoneCode', 'username', 'password', 'email', 'emailCode', 'captchaValue'];
    handlers.forEach((fn, i) => {
      expect(script).toMatch(new RegExp('function ' + fn + '\\(e : InputEvent\\) : void \\{\\s*' + targets[i] + '\\.value = e\\.detail\\.value'));
    });
    for (const f of [FORM, GATE]) {
      const src = read(f);
      // 「无 InputEvent 先例」= .uts 侧不得出现该**类型使用**（注释里提及它属文档说明，允许）
      expect(src).not.toMatch(/:\s*InputEvent\b/);
      expect(src).not.toContain('e.detail.value');
    }
  });
});

describe('门控面归属唯一（票面 ③「生物识别门控行为零改动」的结构面）', () => {
  it('四个门控动作只在 useBiometricGate.uts，页面与表单面都不再自持', () => {
    const page = read(PAGE);
    const form = read(FORM);
    const gate = read(GATE);
    for (const fn of ['initBiometricGate', 'onBiometricUnlock', 'onQuickLogin', 'toggleRemember']) {
      expect(fnBodyOf(gate, fn)).not.toBe('');
      expect(page).not.toContain('function ' + fn);
      expect(form).not.toContain('function ' + fn);
    }
    // 防「同一逻辑复制两份」：门控面的关键调用点在门控文件各出现一次
    for (const probe of ['biometric.authenticate(', 'loadSecureToken()', 'saveAccountOnly(cred.u)']) {
      expect([...gate.matchAll(new RegExp(probe.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'))].length).toBeGreaterThan(0);
      expect(page).not.toContain(probe.replace('(', ''));
    }
  });

  it('quickLoginLabel / showedStored 的生成与消费分在两侧（模板读、门控写）', () => {
    expect(read(GATE)).toContain("const quickLoginLabel = computed<string>");
    expect(pageTemplate()).toContain('{{ quickLoginLabel }}');
    expect(pageTemplate()).toContain('showedStored && biometric.isSupported.value');
  });

  it('凭据抽象层的 import 只在两个 composable 里，页面壳层不碰存储与凭据', () => {
    const page = read(PAGE);
    expect(page).not.toContain('secureStorage');
    expect(read(GATE)).toContain("from '../../../utils/secureStorage'");
    expect(read(FORM)).toContain("from '../../../utils/secureStorage'");
  });
});

describe('allowlist 不回潮（login 模块违例清零的锁）', () => {
  // ADR-0023 决策 ⑧：`GUARD_ALLOWLIST` 的唯一声明点是 `utils/guardAllowlist.js`，消费方一律 require 取用。
  const { allowlistPaths } = require('./guardAllowlist');
  const isLoginPath = (p) => /^pages\/login\//.test(p);

  it('GUARD_ALLOWLIST 不含 login 模块文件（票面 ⑦「本模块 allowlist 条目（catch/detail）清零」）', () => {
    expect(allowlistPaths().filter(isLoginPath)).toEqual([]);
  });

  it('判据具备判别力（注入一条 login 豁免必须被抓到）', () => {
    const injected = allowlistPaths().concat(['pages/login/composables/useLoginForm.uts']);
    expect(injected.filter(isLoginPath)).toEqual(['pages/login/composables/useLoginForm.uts']);
  });
});

describe('零直发请求 + 域 api 出口（票面 ⑤：页面层不直接发请求）', () => {
  it('页面不 import 任何 api 模块（只经 composable）', () => {
    const page = read(PAGE);
    expect(page).not.toContain('api/auth');
    expect(page).not.toContain('api/request');
    expect(page).not.toMatch(/uni\.request\s*\(/);
  });

  it('两个 composable 都不直接调用 uni.request，请求面只来自 api/auth.uts', () => {
    for (const f of [FORM, GATE]) {
      const src = read(f);
      expect(src).not.toMatch(/uni\.request\s*\(/);
      expect(src).not.toContain("from '../../../api/request'");
    }
    expect(read(FORM)).toContain("import { sendCodeApi, getCaptchaApi } from '../../../api/auth'");
    // 门控面走 auth store（quickLogin），不自己发请求
    expect(read(GATE)).not.toContain("from '../../../api/auth'");
    expect(read(GATE)).toContain("from '../../../stores/auth'");
  });

  it('登录侧只消费 getCaptchaApi / sendCodeApi 两个域出口（register 专属出口不得回流登录页）', () => {
    const form = read(FORM);
    for (const fn of ['getCaptchaApi', 'sendCodeApi', 'loginByCode', 'loginByWechat', 'auth.login(']) {
      expect(form).toContain(fn.replace('auth.login(', 'login('));
    }
    for (const fn of ['registerApi', 'emailRegisterApi', 'sendPhoneCodeApi', 'resetPasswordApi']) {
      expect(form).not.toContain(fn);
    }
  });

  it('getCaptchaApi 消费面按 DTO 取值（T12 #650 出口，裸索引不得回潮）', () => {
    const form = read(FORM);
    expect(form).toContain('const data = await getCaptchaApi()');
    expect(form).toContain('captchaId.value = data.id');
    expect(form).toContain('captchaImage.value = data.image');
    expect(form).not.toContain("data['id'] as string");
    expect(form).not.toContain("data['image'] as string");
  });

  it('sendCodeApi 以 purpose=login 消费（白名单理由「void 用法」由 #650 落锁，此处只锁登录侧调用形态）', () => {
    const body = fnBodyOf(read(FORM), 'onSendCode');
    expect(body).toContain("await sendCodeApi(target, channel, 'login', captchaId.value, captchaValue.value)");
    // 域出口仍是裸 `post` 透传（返回 UTSJSONObject，登录侧不落地返回值）
    expect(read(AUTH_API)).toMatch(/export function sendCodeApi\([^)]*\) : Promise<UTSJSONObject> \{/);
  });
});

describe('行为保持点（票面 ②：UI 像素级不变 / 表单行为逐字保持的机检面）', () => {
  const form = read(FORM);
  const tpl = pageTemplate();

  it('四种登录方式分支齐备，默认仍是手机号验证码', () => {
    expect(form).toContain("const mode = ref<string>('phone')");
    for (const probe of ['<template v-if="mode == \'password\'">', "<template v-else-if=\"mode == 'phone'\">", "<template v-else-if=\"mode == 'wechat'\">", '<template v-else>']) {
      expect(tpl).toContain(probe);
    }
    expect(tpl).toContain("{{ countdown > 0 ? countdown + 's' : '获取验证码' }}");
  });

  it('模式切换：同值短路 + 清倒计时 + 错次归零 + 刷图形验证码', () => {
    const sw = fnBodyOf(form, 'switchMode');
    expect(sw).toContain('if (m == mode.value) return');
    expect(sw).toContain('clearCountdownTimer()');
    expect(sw).toContain('countdown.value = 0');
    expect(sw).toContain('wrongAttempts.value = 0');
    expect(sw).toContain('loadCaptcha()');
  });

  it('发送验证码：重入/倒计时拦截 → 图形验证码前置 → 通道校验 → 60s 倒计时 → 失败刷新图形验证码', () => {
    const body = fnBodyOf(form, 'onSendCode');
    expect(body).toContain('if (sendingCode.value || countdown.value > 0) return');
    expect(body).toContain("uni.showToast({ title: '请先输入图形验证码', icon: 'none' })");
    expect(body).toContain('const err = validatePhoneNum(phone.value)');
    expect(body).toContain("uni.showToast({ title: '邮箱格式不正确', icon: 'none' })");
    expect(body).toContain("uni.showToast({ title: '验证码已发送，请查收', icon: 'none' })");
    expect(body).toContain("let msg = '验证码发送失败'");
    expect(body).toContain('startCountdown()');
    expect(fnBodyOf(form, 'startCountdown')).toContain('countdown.value = 60');
    expect(fnBodyOf(form, 'clearCountdownTimer')).toContain('const timer = countdownTimer');
  });

  it('四通道校验文案逐字仍在（手机 / 账号密码 / 微信放行 / 邮箱）', () => {
    const v = fnBodyOf(form, 'validate');
    expect(v).toContain("if (phoneCode.value.length != 6) return '请输入 6 位验证码'");
    expect(v).toContain("if (username.value.length == 0) return '请输入用户名或手机号'");
    expect(v).toContain("if (password.value.length < 6) return '密码至少 6 位'");
    expect(v).toContain("if (mode.value == 'wechat') {\n            return ''");
    expect(v).toContain("if (t.indexOf('@') <= 0 || t.indexOf('.') <= 0) return '邮箱格式不正确'");
    expect(fnBodyOf(form, 'validatePhoneNum')).toContain("if (p.length != 11) return '请输入 11 位手机号'");
  });

  it('协议勾选：非 password 模式才拦截（账号密码登录不强制勾选）', () => {
    const body = fnBodyOf(form, 'onSubmit');
    expect(body).toContain("if (mode.value != 'password') {");
    expect(body).toContain("const errAgree = agreed.value == false ? '请先同意用户协议和隐私政策' : ''");
    expect(tpl).toContain("v-if=\"mode != 'password'\"");
  });

  it('提交：四路分派 + 凭据落地三态 + 失败累计验证码错次', () => {
    const body = fnBodyOf(form, 'onSubmit');
    expect(body).toContain("if (mode.value == 'phone') {");
    expect(body).toContain('await auth.loginByCode({');
    expect(body).toContain('const result = await auth.loginByWechat()');
    expect(body).toContain('await auth.login({');
    expect(body).toContain('if (biometric.isSupported.value) {');
    expect(body).toContain('saveAccountOnly(uname)');
    expect(body).toContain('clearSecureCredentials()');
    expect(body).toContain("if (mode.value != 'password' && mode.value != 'wechat') {");
    expect(body).toContain("if (m.indexOf('验证码') >= 0)");
    expect(body).toContain('wrongAttempts.value += 1');
    expect(body).toContain("let msg = '登录失败，请重试'");
  });

  it('成功跳转两分支：新用户选证件、老用户进首页（含延时逐字）', () => {
    const body = fnBodyOf(form, 'afterLoginSuccess');
    expect(body).toContain("uni.showToast({ title: '已为您自动注册账号', icon: 'none', duration: 1500 })");
    expect(body).toContain("uni.reLaunch({ url: '/pages/guide/choose-cert' })");
    expect(body).toContain("uni.showToast({ title: '登录成功', icon: 'success', duration: 1000 })");
    expect(body).toContain("uni.reLaunch({ url: '/pages/dashboard/dashboard' })");
    expect(body).toContain('}, 800)');
    expect(body).toContain('}, 500)');
  });

  it('主按钮文案三态与验证码提示三档逐字仍在', () => {
    const btn = form.slice(form.indexOf('const btnText = computed<string>'));
    expect(btn).toContain("if (loading.value) return '登录中...'");
    expect(btn).toContain("if (mode.value == 'wechat') return '微信一键登录'");
    expect(btn).toContain("return '登 录'");
    const hint = form.slice(form.indexOf('const codeHint = computed<string>'));
    expect(hint).toContain("return '验证码错误次数过多，请点击获取验证码重新发送'");
    expect(hint).toContain("return '验证码错误，还可再试 ' + (5 - wrongAttempts.value) + ' 次'");
    expect(hint).toContain("return '6 位数字验证码，5 分钟内有效'");
  });

  it('三个跳转入口 + 键盘「完成」= 提交', () => {
    expect(fnBodyOf(form, 'goRegister')).toContain("uni.navigateTo({ url: '/pages/register/register' })");
    expect(fnBodyOf(form, 'goForgotPassword')).toContain("uni.navigateTo({ url: '/pages/forgot-password/forgot-password' })");
    expect(fnBodyOf(form, 'goRecruiterLogin')).toContain("uni.navigateTo({ url: '/pages/recruiter/login' })");
    expect(fnBodyOf(form, 'onConfirm')).toContain('onSubmit()');
  });

  it('生命周期：onLoad 已登录门控 → 拉图形验证码 → 门控初始化；onUnload 清倒计时', () => {
    const script = pageScript();
    expect(script).toContain('auth.restoreFromStorage()');
    expect(script).toContain('if (auth.isLoggedIn.value)');
    expect(script).toContain("uni.reLaunch({ url: '/pages/dashboard/dashboard' })");
    expect(script).toContain('statusBarHeight.value = sysInfo.statusBarHeight');
    expect(script).toMatch(/loadCaptcha\(\)\s*\n\s*initBiometricGate\(\)/);
    expect(script).toMatch(/onUnload\(\(\) => \{\s*clearCountdownTimer\(\)/);
  });

  it('行为保持点判据具备判别力（改坏一处必红的抽样：password 模式的凭据分叉被删即红）', () => {
    const broken = form.replace('if (biometric.isSupported.value) {', 'if (false) {');
    expect(broken).not.toBe(form);
    expect(fnBodyOf(broken, 'onSubmit')).not.toContain('if (biometric.isSupported.value) {');
  });
});
