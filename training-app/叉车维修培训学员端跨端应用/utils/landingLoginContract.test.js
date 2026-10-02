/**
 * #1478 着陆页契约测试 —— 启动页从「品牌开屏 → 登录页」两跳改为**未登录着陆页**
 *
 * 本套件是**接线守护**（源码文本 + 结构事实，不构成 ③ 门的行为证据；行为兜底见下）。
 * 行为面（真跑 composable / 共享出口 / 探测件，数请求层与出口被调次数）在
 * `utils/landingLoginBehavior.test.js`，两件套分工与本仓 `loginContract` ↔ `authPreRequestValidationBehavior`
 * 的同款分工一致：这里证明**接线没断**，那里证明**行为对**。
 *
 * ## 逐条 AC 与判据面（票面 Desired behavior 1-7 + AC 清单）
 *
 * | AC | 这里锁什么 | 锁不住什么（归谁） |
 * |---|---|---|
 * | 1 已登录冷启动直达 dashboard | `onLoad` 里那条 `if (auth.isLoggedIn.value)` 守卫 + 块内**唯一**一条 `uni.reLaunch` 指向 dashboard，且守卫块内**零**嵌套条件 | — |
 * | 2 分端出主 CTA | **结构事实**：主 CTA 带 `v-if="wechatAvailable"`、次级入口**不受探测门禁**（探测不可用仍在）⇒「不可用 ⇒ 次级分支」「可用 ⇒ 微信分支」两端都成立 | 「App 端**不露出**『一键』二字」是**运行期渲染事实**：同一份 `index.uvue` 同时承载两端分支，小程序 CTA 文案本身就是「微信一键登录」⇒ 静态不可验证（控制器裁定 R1），**禁止**在此写"源码不含一键"这种恒假断言 ⇒ 归维护者真机门 |
 * | 3 未勾选协议拦在请求之前 | 拦截文案与登录页**同一字面量** + 源码顺序（拦截 return 早于 `loginByWechat()`） | 「真的零请求」由 behavior 套件按桩计数断言 |
 * | 4 两入口一条成功出口 | 着陆页 composable **零** `reLaunch`、`afterLoginSuccess` 定义全仓**唯一**、新用户 toast 全仓**唯一** | 出口两条分支的行为 = behavior 套件真跑 |
 * | 5 失败可见 | `errorHint` 有写入点 **且** 页面有可见渲染位（两条都要，缺一即红） | 渲染出来长什么样 = 真机门 |
 * | 6 删手绘吉祥物 | 原手绘 class 名族零命中 + `logo.png` 图像槽位在位 | 图片视觉 = 真机门 |
 * | 7 协议行抽共享件 | 两页 import **同一件**、两页模板不再自持《用户协议》文案、共享件最小面（props/emits 清单 + 表单态不得入内） | 同视觉（`loginContract` 的模板 sha 锁已批准变更） |
 * | 探测件两页共享 | 两页 import **同一符号** + 平台判据（`uniPlatform`）全仓**只出现在探测件里**（两份实现即红） | 探测结论在真机上对不对 = 真机门 |
 * | `#ifdef` 增删 = 0 | 两个运行时面文件的条件编译**指令行计数**锁到 BASE 值（index 0 行、login 2 行），且 login 那条 `MP-WEIXIN` + `#endif` 成对仍在（不许拆） | — |
 * | 零配置面改动 | `pages.json` 路由事实：启动页仍是第一条、独立登录页**仍在路由表**（保留页降级、不退役） | 「文件一字未动」由 commit 的 `--name-only` 证明（jest 读不到 git），见 task-1-report |
 *
 * ## 为什么这些判据不是「只锁字面量」
 * 每条判据都是**函数形态**（`coldStartFacts` / `ctaFacts` / `outletFacts` …），真源用例与注入自检
 * 共用同一条函数 —— 与本仓 `apiBatchBContract` / `authLoginOutletsContract` 的「判据本体」口径一致。
 * 每个 describe 都带**成对取证**：改坏必红 + 真源必不红（防判据退化成恒真）。
 *
 * ## 刻意不做的事（越界即违规）
 * - 不锁 `pages.json` / `manifest.json` / `platformConfig.json` 的字节 sha（那三把锁会让下一张改路由的票无故变红）
 * - 不动 `loginContract.test.js` 的模板/样式 sha（本票那次变更已在该文件内注明「经批准」）
 * - 不锁 register 页的协议行（#1478 只管启动页 + 登录页两页同源；register 那份是**已登记的遗留**，见报告薄弱点）
 */
const h = require('./contractHarness');
const read = h.read;

const PAGE = 'pages/index/index.uvue';
const LANDING = 'pages/index/composables/useLandingLogin.uts';
const OUTLET = 'utils/loginOutlet.uts';
const PROBE = 'composables/useLoginProviders.uts';
const AGREEMENT = 'components/login-agreement/login-agreement.uvue';
const LOGIN_PAGE = 'pages/login/login.uvue';
const LOGIN_FORM = 'pages/login/composables/useLoginForm.uts';
const PAGES_JSON = 'pages.json';

/** 登录页现有口径（AC 3「口径一致」的字面基准；两处必须同一个字符串，不许各造同义词） */
const MSG_AGREE = '请先同意用户协议和隐私政策';

/** 注释剥离：`/* *\/` 与 `//`（本仓各契约测试同形），再加 uvue 模板的 `<!-- -->` */
const stripJs = (src) => src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');
const stripTpl = (src) => src.replace(/<!--[\s\S]*?-->/g, '');
const stripAll = (src) => stripJs(stripTpl(src));

/** 页面块（与 loginContract 同口径：模板有嵌套 `<template v-if>` ⇒ 闭合取最后一个） */
function block(src, tag) {
  const open = src.indexOf(`<${tag}`);
  const close = tag === 'template' ? src.lastIndexOf(`</${tag}>`) : src.indexOf(`</${tag}>`);
  if (open === -1 || close === -1) return '';
  return src.slice(open, close + tag.length + 3);
}

/** 出现次数（fail-loud 用：锚点不唯一的变异会静默落空 ⇒ 「必红」变假绿） */
const count = (hay, needle) => hay.split(needle).length - 1;

/**
 * 取 `anchor` 之后**花括号平衡**的那一块（判据 1 用它证明「守卫块内部没有别的东西」，
 * 而不是靠 `[\s\S]*?` 这类会跨过任意代码的非锚定匹配 —— 后者对「条件化」没有判别力）。
 */
function braceBlock(src, anchor) {
  const at = src.indexOf(anchor);
  if (at === -1) return null;
  const open = src.indexOf('{', at);
  if (open === -1) return null;
  let depth = 0;
  for (let i = open; i < src.length; i += 1) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  return null;
}

// ── AC 1：已登录冷启动落地路由（判据本体） ──────────────────────────────────

const COLDSTART_GUARD = 'if (auth.isLoggedIn.value) {';
const DASHBOARD = '/pages/dashboard/dashboard';

/** 判据本体：守卫在位、块内只有一条 reLaunch 且指向 dashboard、块内零嵌套条件/短路 */
function coldStartFacts(src) {
  const code = stripAll(src);
  const body = braceBlock(code, COLDSTART_GUARD);
  const onLoadAt = code.indexOf('onLoad(');
  const guardAt = code.indexOf(COLDSTART_GUARD);
  return {
    guardCount: count(code, COLDSTART_GUARD),
    blockFound: body !== null,
    relaunchTargets: body === null ? [] : [...body.matchAll(/uni\.reLaunch\(\{[\s\S]*?url: '([^']+)'/g)].map((m) => m[1]),
    // 「不得条件化」= 守卫块内部不得再出现 if / else / && / || / 三元
    nestedConditions: body === null ? -1 : (body.match(/\bif\s*\(|\belse\b|&&|\|\||\?/g) || []).length,
    pageReLaunchTotal: (code.match(/uni\.reLaunch\(/g) || []).length,
    restoreBeforeGuard: onLoadAt !== -1 && guardAt !== -1 && code.indexOf('auth.restoreFromStorage()') < guardAt,
    guardInsideOnLoad: onLoadAt !== -1 && guardAt > onLoadAt,
  };
}

describe('AC 1 已登录冷启动仍直达 dashboard（全应用唯一落地路由，不得删改/条件化）', () => {
  const facts = coldStartFacts(read(PAGE));

  it('守卫在位且唯一，块内恰一条 reLaunch 指向 dashboard', () => {
    expect(facts.guardCount).toBe(1);
    expect(facts.blockFound).toBe(true);
    expect(facts.relaunchTargets).toEqual([DASHBOARD]);
    expect(facts.pageReLaunchTotal).toBe(1);
  });

  it('未被条件化：守卫块内零嵌套条件/短路；守卫在 onLoad 内、且排在 restoreFromStorage 之后', () => {
    expect(facts.nestedConditions).toBe(0);
    expect(facts.guardInsideOnLoad).toBe(true);
    expect(facts.restoreBeforeGuard).toBe(true);
  });

  it('判别力：删守卫 / 包一层条件 / 换落点 / 加第二条 reLaunch，四种改坏都被同一条判据抓到', () => {
    const src = read(PAGE);
    const deleted = src.replace(`        ${COLDSTART_GUARD}`, '        if (false) {');
    expect(coldStartFacts(deleted).guardCount).toBe(0);
    const conditioned = src.replace(COLDSTART_GUARD, 'if (auth.isLoggedIn.value && isFirstRun) {');
    expect(coldStartFacts(conditioned).guardCount).toBe(0);
    const retargeted = src.replace(`url: '${DASHBOARD}'`, "url: '/pages/login/login'");
    expect(coldStartFacts(retargeted).relaunchTargets).toEqual(['/pages/login/login']);
    const doubled = src.replace(COLDSTART_GUARD, `${COLDSTART_GUARD}\n            uni.reLaunch({ url: '/pages/login/login' })`);
    expect(coldStartFacts(doubled).pageReLaunchTotal).toBe(2);
    // 对照组：真源跑同一条判据必须全绿（防判据恒真）
    expect(coldStartFacts(src)).toEqual(facts);
  });
});

// ── AC 2（裁定 R1：锁结构事实，不锁运行期文案） ─────────────────────────────

/** 主 CTA（微信）与次级入口（其他登录方式）的门禁拓扑 */
function ctaFacts(src) {
  const tpl = stripTpl(block(src, 'template'));
  const wechatBtn = /<button[^>]*class="[^"]*cta-wechat[^"]*"[^>]*>([^<]*)<\/button>/.exec(tpl);
  const secondary = [...tpl.matchAll(/<view[^>]*class="[^"]*\bcta-secondary\b[^"]*"[^>]*>/g)];
  return {
    wechatBtnFound: wechatBtn !== null,
    wechatBtnText: wechatBtn === null ? '' : wechatBtn[1],
    wechatBtnGatedByProbe: wechatBtn !== null && /v-if="wechatAvailable"/.test(wechatBtn[0]),
    wechatBtnWired: wechatBtn !== null && /@click="onWechatLogin"/.test(wechatBtn[0]),
    secondaryCount: secondary.length,
    secondaryWired: secondary.length === 1 && /@click="goOtherLogin"/.test(secondary[0][0]),
    // 关键结构事实：次级入口**不带任何** v-if/v-show ⇒ 探测不可用时它仍在（App 端有出口）
    secondaryGated: secondary.length === 1 && /\b(v-if|v-show)=/.test(secondary[0][0]),
    secondaryTextShown: /<text[^>]*class="[^"]*cta-secondary-text[^"]*">其他登录方式<\/text>/.test(tpl),
    probeGateUses: (tpl.match(/v-if="wechatAvailable"/g) || []).length,
    probeImported: /from '[^']*composables\/useLoginProviders'/.test(src),
    probeDestructured: /const \{ wechatAvailable \} = useLoginProviders\(\)/.test(src),
  };
}

describe('AC 2 分端主入口（R1：锁「探测可用⇒微信分支 / 不可用⇒次级分支」的结构事实）', () => {
  const facts = ctaFacts(read(PAGE));

  it('可用分支：主 CTA 由探测结论门禁，且接的是着陆页登录动作', () => {
    expect(facts.wechatBtnFound).toBe(true);
    expect(facts.wechatBtnText).toBe('微信一键登录');
    expect(facts.wechatBtnGatedByProbe).toBe(true);
    expect(facts.wechatBtnWired).toBe(true);
  });

  it('不可用分支：次级入口恰一条、接 goOtherLogin、且**不受探测门禁**（探测不可用时仍在）', () => {
    expect(facts.secondaryCount).toBe(1);
    expect(facts.secondaryWired).toBe(true);
    expect(facts.secondaryGated).toBe(false);
    expect(facts.secondaryTextShown).toBe(true);
  });

  it('探测件是**运行期**接入（import + 解构），不是编译期分支', () => {
    expect(facts.probeImported).toBe(true);
    expect(facts.probeDestructured).toBe(true);
    expect(facts.probeGateUses).toBe(2); // 主 CTA + 协议行（协议件只门禁微信登录，与主 CTA 同进同退）
  });

  it('判别力：摘掉主 CTA 的门禁 / 给次级入口加门禁 / 删掉次级入口，三种都必红', () => {
    const src = read(PAGE);
    const ungated = src.replace('<button class="cta-wechat" v-if="wechatAvailable"', '<button class="cta-wechat"');
    expect(ctaFacts(ungated).wechatBtnGatedByProbe).toBe(false);
    const secondaryGated = src.replace('<view class="cta-secondary" @click="goOtherLogin">',
      '<view class="cta-secondary" v-if="wechatAvailable" @click="goOtherLogin">');
    expect(ctaFacts(secondaryGated).secondaryGated).toBe(true);
    const removed = src.replace(/<view class="cta-secondary"[\s\S]*?<\/view>/, '');
    expect(ctaFacts(removed).secondaryCount).toBe(0);
    expect(ctaFacts(src)).toEqual(facts);
  });

  it('本票**不**锁「App 端源码不含『一键』」——同一份 index.uvue 承载两端分支，那是运行期渲染事实（裁定 R1），归维护者真机门', () => {
    // 这条用例的作用是把「为什么这里没有那条断言」变成可检索的事实，防止下一个会话补一条恒假锁（假绿/假红双重来源）。
    const src = stripAll(read(PAGE));
    expect(src).toContain('微信一键登录'); // 小程序端 CTA 文案就在同一文件 ⇒ 「不含一键」在本文件恒假
    const selfCheck = require('fs').existsSync(h.absOf('utils/landingLoginBehavior.test.js'));
    expect(selfCheck).toBe(true); // 运行期那一半另有行为套件与本票报告承接
  });
});

// ── `#ifdef` 指令行增删 = 0（AC：硬约束） ───────────────────────────────────

/** 指令行 = 行首（可缩进）的 `<!--` / `//` / `*` 后跟 `#ifdef|ifndef|endif` */
const IFDEF_LINE_RE = /^[ \t]*(?:<!--|\/\/|\*)[ \t]*#(?:ifdef|ifndef|endif)\b.*$/gm;
/** BASE（83e0d2f9）实测计数：启动页原本就没有条件编译；登录页那条 `#ifdef MP-WEIXIN` + `#endif` 成对 2 行 */
const IFDEF_BASE = { [PAGE]: 0, [LOGIN_PAGE]: 2 };

const ifdefCount = (src) => (src.match(IFDEF_LINE_RE) || []).length;

describe('AC `#ifdef` 指令行增删数 = 0（② 门触发面，两个运行时面文件计数锁）', () => {
  it.each(Object.keys(IFDEF_BASE))('%s 的指令行数 = BASE 值', (rel) => {
    expect(ifdefCount(read(rel))).toBe(IFDEF_BASE[rel]);
  });

  it('登录页那条 MP-WEIXIN **没被拆**（成对仍在，微信入口的编译面不变）', () => {
    const src = read(LOGIN_PAGE);
    expect(ifdefCount(src)).toBe(2);
    expect(/<!--\s*#ifdef\s+MP-WEIXIN\s*-->/.test(src)).toBe(true);
    expect(/<!--\s*#endif\s*-->/.test(src)).toBe(true);
  });

  it('判别力：给启动页加一行 MP-WEIXIN、或把登录页那条拆掉，计数都必须变（锁不是恒真）', () => {
    const injected = read(PAGE) + '\n            <!-- #ifdef MP-WEIXIN -->\n';
    expect(ifdefCount(injected)).toBe(1);
    const split = read(LOGIN_PAGE).replace(/<!--\s*#ifdef\s+MP-WEIXIN\s*-->/, '<!-- moved out -->');
    expect(ifdefCount(split)).toBe(1);
    expect(ifdefCount(read(PAGE))).toBe(0);
  });
});

// ── AC 3：未勾选协议的拦截（接线面 + 与登录页同口径） ───────────────────────

/** 判据本体：拦截文案在、拦截位置早于登录调用、且拦截块内真的有 `return`（只 toast 不 return = 拦不住） */
function agreeGateFacts(src) {
  const code = stripJs(src);
  const gateAnchor = 'if (agreed.value == false) {';
  const gateBlock = braceBlock(code, gateAnchor);
  return {
    msgOnce: count(code, `'${MSG_AGREE}'`),
    gateAt: code.indexOf(gateAnchor),
    requestAt: code.indexOf('auth.loginByWechat()'),
    returnsBeforeRequest: gateBlock !== null && /\breturn\b/.test(gateBlock),
  };
}

describe('AC 3 协议未勾选拦在请求之前（接线面；零请求的行为证据在 landingLoginBehavior）', () => {
  const facts = agreeGateFacts(read(LANDING));

  it('着陆页有拦截、文案恰一处、且源码顺序上早于 loginByWechat 调用', () => {
    expect(facts.msgOnce).toBe(1);
    expect(facts.gateAt).toBeGreaterThan(-1);
    expect(facts.requestAt).toBeGreaterThan(-1);
    expect(facts.gateAt).toBeLessThan(facts.requestAt);
    expect(facts.returnsBeforeRequest).toBe(true);
  });

  it('口径与登录页**同一个字面量**（不许各造同义词：如「请先同意协议」）', () => {
    expect(read(LOGIN_FORM)).toContain(`'${MSG_AGREE}'`);
    expect(read(LANDING)).toContain(`'${MSG_AGREE}'`);
    expect(read(LANDING)).not.toContain('请先同意协议');
  });

  it('判别力：把拦截搬到请求之后（顺序反了）必红，锚点不唯一（两处文案）同样必红', () => {
    const src = read(LANDING);
    const guardBlock = `        if (agreed.value == false) {\n            uni.showToast({ title: '${MSG_AGREE}', icon: 'none' })\n            return\n        }\n`;
    expect(src.includes(guardBlock)).toBe(true); // 锚点在位，否则下面的变异静默落空 ⇒ 「必红」变假绿
    const moved = src.replace(guardBlock, '');
    const after = moved.replace('const result = await auth.loginByWechat()',
      `const result = await auth.loginByWechat()\n        if (agreed.value == false) {\n            uni.showToast({ title: '${MSG_AGREE}', icon: 'none' })\n        }`);
    expect(after).not.toBe(src);
    const f = agreeGateFacts(after);
    expect(f.requestAt).toBeLessThan(f.gateAt); // 请求已经发出去了，拦截才来 ⇒ 判据必须翻红
    expect(f.returnsBeforeRequest).toBe(false);
    const doubled = `${src}\n    // dup\n    const dupMsg = '${MSG_AGREE}'\n`;
    expect(agreeGateFacts(doubled).msgOnce).toBe(2);
    expect(agreeGateFacts(src)).toEqual(facts);
  });
});

// ── AC 4：两入口一条成功出口（全仓不存在第二套登录成功跳转） ────────────────

/** 新用户 toast 与出口定义都是「全仓唯一」的东西：多一处 = 有人复制了成功链路 */
const NEW_USER_TOAST = '已为您自动注册账号';

/**
 * 判据本体：源文件集合 → 违例清单（真源与注入自检共用）。
 * `landingJump` 的口径要说清：着陆页**允许**的那条 reLaunch 是 AC 1 的已登录冷启动守卫，
 * 它不属于「登录成功跳转」⇒ 扫描前先把那条守卫块摘掉，剩下的任何指向 dashboard / choose-cert
 * 的跳转都是第二套成功链路（判据 4 要抓的就是它）。
 */
function outletFacts(sources) {
  const hits = { toast: [], define: [], landingJump: [] };
  for (const [rel, src] of sources) {
    let code = stripAll(src);
    const guard = braceBlock(code, COLDSTART_GUARD);
    if (guard !== null) code = code.replace(guard, '');
    if (code.includes(NEW_USER_TOAST)) hits.toast.push(rel);
    if (/export function afterLoginSuccess\s*\(/.test(code)) hits.define.push(rel);
    if (rel.startsWith('pages/index/') && /uni\.(reLaunch|switchTab|redirectTo|navigateTo)\(\{?\s*url: '\/pages\/(dashboard|guide)/.test(code)) {
      hits.landingJump.push(rel);
    }
  }
  return hits;
}

function allSources() {
  return h.filesUnder('.', /\.(uvue|uts)$/, true).map((rel) => [rel, read(rel)]);
}

describe('AC 4 成功出口唯一（着陆页不持有第二套跳转；两页消费同一个 afterLoginSuccess）', () => {
  const sources = allSources();
  const facts = outletFacts(sources);

  it('新用户 toast 与出口定义在全仓各只有一处，且都在 utils/loginOutlet.uts', () => {
    expect(facts.toast).toEqual([OUTLET]);
    expect(facts.define).toEqual([OUTLET]);
  });

  it('着陆页 composable 零落地跳转（只有出口有），且两页都从共享出口取', () => {
    expect(facts.landingJump).toEqual([]);
    expect(read(LANDING)).toContain("import { afterLoginSuccess } from '../../../utils/loginOutlet'");
    expect(read(LOGIN_FORM)).toContain("import { afterLoginSuccess } from '../../../utils/loginOutlet'");
    expect(read(LANDING)).not.toMatch(/^\s*function afterLoginSuccess/m);
  });

  it('出口两条分支的文本面仍在（新用户 choose-cert / 老用户 dashboard；行为面在 landingLoginBehavior）', () => {
    const outlet = stripJs(read(OUTLET));
    expect(outlet).toContain("uni.reLaunch({ url: '/pages/guide/choose-cert' })");
    expect(outlet).toContain(`uni.reLaunch({ url: '${DASHBOARD}' })`);
    expect(count(outlet, 'uni.reLaunch(')).toBe(2);
  });

  it('判别力：把 toast/出口复制进着陆页、或把出口的一条 reLaunch 摘掉，三种都必红', () => {
    const copied = [[OUTLET, read(OUTLET)], [LANDING, `${read(LANDING)}\n    const dup = '${NEW_USER_TOAST}'\n`]];
    expect(outletFacts(copied).toast).toEqual([OUTLET, LANDING]);
    const redefined = [[OUTLET, read(OUTLET)], [LANDING, `${read(LANDING)}\nexport function afterLoginSuccess(isNew : boolean) : void {}\n`]];
    expect(outletFacts(redefined).define).toEqual([OUTLET, LANDING]);
    const jumped = [[OUTLET, read(OUTLET)], [LANDING, `${read(LANDING)}\n    uni.reLaunch({ url: '${DASHBOARD}' })\n`]];
    expect(outletFacts(jumped).landingJump).toEqual([LANDING]);
    const gutted = read(OUTLET).replace("uni.reLaunch({ url: '/pages/guide/choose-cert' })", 'void 0');
    expect(count(stripJs(gutted), 'uni.reLaunch(')).toBe(1);
    expect(outletFacts(sources)).toEqual(facts);
  });
});

// ── AC 5：失败可见（不静默） ────────────────────────────────────────────────

/** 判据本体：写入点 + 可见渲染位（只有 toast 没有渲染位 = 小程序 toast 一闪即失） */
function failureVisibilityFacts(utsSrc, pageSrc) {
  const code = stripJs(utsSrc);
  const tpl = stripTpl(block(pageSrc, 'template'));
  return {
    hintWritten: count(code, 'errorHint.value = msg'),
    hintToast: /uni\.showToast\(\{ title: msg, icon: 'none' \}\)/.test(code),
    hintClearedOnEntry: /errorHint\.value = ''/.test(code),
    renderSlot: /v-if="errorHint\.length > 0"/.test(tpl),
    renderText: /<text[^>]*class="[^"]*error-text[^"]*">\{\{ errorHint \}\}<\/text>/.test(tpl),
  };
}

describe('AC 5 loginByWechat 失败原因在启动页可见（toast + 常驻渲染位双保险）', () => {
  const facts = failureVisibilityFacts(read(LANDING), read(PAGE));

  it('composable 有写入点与 toast；页面有 v-if 渲染位与文本槽（缺一即红）', () => {
    expect(facts.hintWritten).toBe(1);
    expect(facts.hintToast).toBe(true);
    expect(facts.hintClearedOnEntry).toBe(true);
    expect(facts.renderSlot).toBe(true);
    expect(facts.renderText).toBe(true);
  });

  it('失败文案取 Error.message（不得把原因吞成固定文案）', () => {
    const code = stripJs(read(LANDING));
    expect(code).toContain('const m = errObj.message');
    expect(code).toContain("let msg = '登录失败，请重试'"); // 兜底文案在，但不是唯一路径
    expect(code).toMatch(/msg = m\b/);
  });

  it('判别力：删渲染位 / 删写入点，两种改坏都必红', () => {
    const noSlot = read(PAGE).replace('v-if="errorHint.length > 0"', '');
    expect(failureVisibilityFacts(read(LANDING), noSlot).renderSlot).toBe(false);
    const noWrite = read(LANDING).replace('errorHint.value = msg', 'console.error(msg)');
    expect(failureVisibilityFacts(noWrite, read(PAGE)).hintWritten).toBe(0);
    expect(failureVisibilityFacts(read(LANDING), read(PAGE))).toEqual(facts);
  });
});

// ── AC 6：吉祥物改 logo 占位（删纯 view 手绘） ──────────────────────────────

/** BASE 版启动页手绘吉祥物的 class 族（逐个点名，不用「任何 m- 前缀」这种会误伤的宽口径） */
const HAND_DRAWN = ['antenna-stick', 'antenna-ball', 'm-head', 'm-face', 'm-ear', 'm-eye', 'm-eye-glow', 'm-mouth', 'm-body', 'm-chest', 'm-arm', 'm-leg'];

function mascotFacts(src) {
  const all = stripAll(src);
  return {
    handDrawn: HAND_DRAWN.filter((c) => all.includes(c)),
    logoImages: (all.match(/<image[^>]*\/static\/logo\.png/g) || []).length,
    imageTagInMascotBox: /<view class="mascot-box"[\s\S]*?<image[^>]*src="\/static\/logo\.png"/.test(all),
  };
}

describe('AC 6 启动页不再有纯 view 手绘吉祥物，图像槽位取现有 logo.png', () => {
  const facts = mascotFacts(read(PAGE));

  it('十二个手绘 class 零命中，logo 图像槽位恰一处且在 mascot 盒内', () => {
    expect(facts.handDrawn).toEqual([]);
    expect(facts.logoImages).toBe(1);
    expect(facts.imageTagInMascotBox).toBe(true);
  });

  it('判别力：把原手绘的一个节点塞回来必红；把 logo 换成第二张图同样必红', () => {
    const back = read(PAGE).replace('<view class="hero">', '<view class="hero">\n            <view class="m-head"></view>');
    expect(mascotFacts(back).handDrawn).toContain('m-head');
    const doubled = read(PAGE).replace('<image class="mascot-logo"', '<image class="mascot-logo2" src="/static/logo.png" /><image class="mascot-logo"');
    expect(mascotFacts(doubled).logoImages).toBe(2);
    expect(mascotFacts(read(PAGE))).toEqual(facts);
  });
});

// ── AC 7 + 探测件：两页同源同符号（第二真源即红） ───────────────────────────

/** 协议共享件的最小面（票面划线：`agreed` 双向 + 协议名回调 + 按通道引导文案；表单态不得入内） */
function agreementFacts(src) {
  const props = (/withDefaults\(defineProps<\{([\s\S]*?)\}>\(\)/.exec(src) || [null, ''])[1];
  const emits = (/defineEmits\(\[([^\]]*)\]\)/.exec(src) || [null, ''])[1];
  return {
    propNames: [...props.matchAll(/^\s*([A-Za-z_$][\w$]*)\s*\??\s*:/gm)].map((m) => m[1]).sort(),
    emitNames: [...emits.matchAll(/'([A-Za-z_$][\w$]*)'/g)].map((m) => m[1]).sort(),
    formState: ['countdown', 'sendingCode', 'captcha', 'phoneCode', 'emailCode', 'password', 'loading']
      .filter((token) => src.includes(token)),
    links: {
      agreement: count(stripTpl(block(src, 'template')), '《用户协议》'),
      privacy: count(stripTpl(block(src, 'template')), '《用户隐私》'),
    },
    channelHints: {
      wechat: src.includes("'未注册的微信号将自动注册账号'"),
      email: src.includes("'若邮箱未注册，'"),
      phone: src.includes("'若手机号未注册，'"),
    },
    registerLinkHiddenForWechat: /return props\.channel != 'wechat'/.test(src),
  };
}

describe('AC 7 协议行抽成共享件：启动页与登录页引用同一件、同文案', () => {
  const facts = agreementFacts(read(AGREEMENT));

  it('两页都 import 同一件、模板都用同一标签（不是两份实现）', () => {
    for (const rel of [PAGE, LOGIN_PAGE]) {
      const src = read(rel);
      expect(src).toContain("import LoginAgreement from '../../components/login-agreement/login-agreement.uvue'");
      expect(stripTpl(block(src, 'template'))).toContain('<LoginAgreement');
    }
  });

  it('两个页面的模板都不再自持协议文案（搬干净了，没留第二份）', () => {
    for (const rel of [PAGE, LOGIN_PAGE]) {
      const tpl = stripTpl(block(read(rel), 'template'));
      expect([rel, count(tpl, '《用户协议》')]).toEqual([rel, 0]);
      expect([rel, count(tpl, '《用户隐私》')]).toEqual([rel, 0]);
    }
    expect(facts.links.agreement).toBe(1);
    expect(facts.links.privacy).toBe(1);
  });

  it('最小面守票面划线：props 只有 agreed / channel，emits 只有 toggle / agreement / register，表单态零命中', () => {
    expect(facts.propNames).toEqual(['agreed', 'channel']);
    expect(facts.emitNames).toEqual(['agreement', 'register', 'toggle']);
    expect(facts.formState).toEqual([]);
  });

  it('注册引导按通道分叉（三条通道语义不同的硬口径），微信不给「立即注册」', () => {
    expect(facts.channelHints).toEqual({ wechat: true, email: true, phone: true });
    expect(facts.registerLinkHiddenForWechat).toBe(true);
  });

  it('判别力：页面把文案抄回来 / 共享件多塞一个表单态字段，两种都必红', () => {
    const copied = read(PAGE).replace('<LoginAgreement', '<text>《用户协议》</text>\n            <LoginAgreement');
    expect(count(stripTpl(block(copied, 'template')), '《用户协议》')).toBe(1);
    const fattened = read(AGREEMENT).replace("channel? : string", "channel? : string\n        countdown? : number");
    expect(agreementFacts(fattened).propNames).toContain('countdown');
    expect(agreementFacts(read(AGREEMENT))).toEqual(facts);
  });
});

describe('AC 探测件两页共享同一符号（平台判据全仓唯一，两份实现即红）', () => {
  const sources = allSources();

  it('两页都 import 根 composables/useLoginProviders 并解构 wechatAvailable', () => {
    for (const rel of [PAGE, LOGIN_PAGE]) {
      expect(read(rel)).toContain("import { useLoginProviders } from '../../composables/useLoginProviders'");
      expect(read(rel)).toContain('const { wechatAvailable } = useLoginProviders()');
    }
  });

  it('平台判据（uniPlatform）只出现在探测件里 —— 出现第二个文件即「两份实现」', () => {
    const holders = sources
      .filter(([, src]) => stripAll(src).includes('uniPlatform'))
      .map(([rel]) => rel);
    expect(holders).toEqual([PROBE]);
  });

  it('探测件返回面是**能力可用性判断**（供 #1484 决定 disabled），不是「要不要渲染」的布尔', () => {
    const src = read(PROBE);
    const declared = [...src.matchAll(/^\s{4}([A-Za-z_$][\w$]*)\s*:/gm)].map((m) => m[1]).sort();
    expect(declared).toEqual(['wechatAvailable', 'wechatUnavailableReason']);
    expect(src).toContain('wechatAvailable : ComputedRef<boolean>');
    expect(src).not.toMatch(/shouldRender|showWechat|visible\b/);
  });

  it('判别力：着陆页自己抄一份平台判断必红（计数从 1 变 2）', () => {
    const copied = sources.concat([['pages/whatever/whatever.uts', "const p = uni.getSystemInfoSync().uniPlatform\nexport const x = p == 'mp-weixin'"]]);
    const holders = copied.filter(([, src]) => stripAll(src).includes('uniPlatform')).map(([rel]) => rel);
    expect(holders).toContain('pages/whatever/whatever.uts');
    expect(holders.length).toBeGreaterThan(1);
  });
});

// ── 新增件的可编译形态（本轮审计抓获的真实缺陷 → 回归锁） ───────────────────

/**
 * `computed<T>(() : T { … })`（漏 `=>`）**不是合法 TypeScript**：UTS 是 TS 方言，这种写法
 * ① 进不了 `utils/utsHarness.js` 的执行面（行为守护静默失效），② 也过不了 ④ Kotlin 编译门。
 * 全仓既有 25 处 `computed<` 同形写法都带 `=>` ⇒ 本票新增/改动的件必须跟齐。
 * 这条锁的现实价值：#1478 半成品里两处（探测件 + 协议件）就是这个形态，本轮修的就是它。
 */
describe('#1478 新增件的 computed 回调形态（utsHarness 可解析 + ④ 门可编译的共同前提）', () => {
  const computedSites = (src) => src.split('\n')
    .filter((line) => line.includes('computed<'))
    .map((line) => line.trim());

  it.each([PROBE, AGREEMENT, LANDING, PAGE, OUTLET])('%s 的每一处 computed 都带 =>', (rel) => {
    for (const line of computedSites(read(rel))) expect([rel, line.includes('=>')]).toEqual([rel, true]);
  });

  it('本票新增件确有 computed 站点（防止上面那条退化成空集恒真）', () => {
    expect(computedSites(read(PROBE))).toHaveLength(2);
    expect(computedSites(read(AGREEMENT))).toHaveLength(2);
  });

  it('判别力：把探测件的一处 => 摘掉（= 半成品的原始形态），同一条筛子必须抓到', () => {
    const broken = read(PROBE).replace('computed<boolean>(() : boolean => {', 'computed<boolean>(() : boolean {');
    expect(broken).not.toBe(read(PROBE));
    const bad = computedSites(broken).filter((line) => !line.includes('=>'));
    expect(bad).toEqual(['const wechatAvailable = computed<boolean>(() : boolean {']);
  });
});

// ── 零配置面：路由事实（启动页仍是第一条、登录页保留在路由表） ──────────────

describe('零配置面改动可机检的那一半：pages.json 路由事实（配置三件套本票一字未动）', () => {
  const routes = () => JSON.parse(read(PAGES_JSON)).pages.map((p) => p.path);

  it('启动页路由仍是第一条（本票不换入口页），独立登录页仍在路由表（保留页降级、不退役 #1483）', () => {
    expect(routes()[0]).toBe('pages/index/index');
    expect(routes()).toContain('pages/login/login');
  });

  it('判别力：把登录页从路由表摘掉（退役）必被抓到', () => {
    const retired = JSON.stringify({ pages: JSON.parse(read(PAGES_JSON)).pages.filter((p) => p.path !== 'pages/login/login') });
    expect(JSON.parse(retired).pages.map((p) => p.path)).not.toContain('pages/login/login');
    expect(routes()).toContain('pages/login/login');
  });
});

// ── 声明面对账（新增文件都登记了；modules.js 改动是必需的、不是顺手） ────────

describe('新增四个文件的模块声明面对账（modules.js 登记是硬要求，不是可选）', () => {
  it('harness 全表对账零违规（漏登记 = 「隐形文件」，正是票 D 堵的门缝）', () => {
    const report = h.reconcile();
    expect(report.violations).toEqual([]);
    expect(report.ok).toBe(true);
  });

  it('本票新增件都登记为跨切面基础设施或 index 模块私有件（判据 2 的落位裁定）', () => {
    expect(h.INFRA.dirs).toContain('components/login-agreement');
    expect(h.INFRA.files).toContain('composables/useLoginProviders.uts');
    expect(h.MODULES.index.files).toContain('pages/index/composables/useLandingLogin.uts');
    expect(h.MODULES.index.extractDirs).toContain('pages/index/composables');
    // 出口件落 utils/**（INFRA 整目录），不新建第三条归属规则
    expect(h.INFRA.dirs).toContain('utils');
  });

  it('判别力：摘掉一处登记，对账必须把它报成 unregistered / missing', () => {
    const infra = JSON.parse(JSON.stringify(h.INFRA));
    infra.files = infra.files.filter((f) => f !== 'composables/useLoginProviders.uts');
    expect(h.reconcile(h.MODULES, infra).unregisteredSourceFiles)
      .toContain('composables/useLoginProviders.uts');
    const decls = JSON.parse(JSON.stringify(h.MODULES));
    decls.index.files = decls.index.files.filter((f) => !f.endsWith('useLandingLogin.uts'));
    expect(h.reconcile(decls, h.INFRA).missing.map((x) => x.file))
      .toContain('pages/index/composables/useLandingLogin.uts');
  });

  it('登录页/着陆页 import 的是**中立位置**的共享件，不构成跨模块私有件消费（避开三处连锁契约锁）', () => {
    expect(h.reconcile().unregisteredConsumers).toEqual([]);
    expect(h.MODULES.index.crossModuleConsumers).toEqual([]);
    expect(h.MODULES.login.crossModuleConsumers).toEqual([]);
  });
});
