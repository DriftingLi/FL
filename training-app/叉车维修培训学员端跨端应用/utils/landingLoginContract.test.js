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
 * | 2 分端出主 CTA | **结构事实**：主 CTA 带 `v-if="wechatAvailable"`（**展示面**：App 与小程序一律展示 —— 2026-10-03 维护者口径「移动端也展示，有个样式即可」）、次级入口**不受探测门禁**（探测不可用仍在）；**接通面** `wechatLoginReady` 单独门禁点击链（App 端 = 友好提示、零请求） | 「App 端**不露出**『一键』二字」**已作废**（2026-10-03 ①a 真机判红后维护者改口径：App 端**允许**展示该入口，只要求点击不空转）⇒ 该断言及其反向断言都不在此写，运行期渲染事实归真机门 |
 * | 3 未勾选协议拦在请求之前 | 拦截文案与登录页**同一字面量** + 源码顺序（拦截 return 早于 `loginByWechat()`） | 「真的零请求」由 behavior 套件按桩计数断言 |
 * | 4 两入口一条成功出口 | 着陆页 composable **零** `reLaunch`、`afterLoginSuccess` 定义全仓**唯一**、新用户 toast 全仓**唯一** | 出口两条分支的行为 = behavior 套件真跑 |
 * | 5 失败可见 | `errorHint` 有写入点 **且** 页面有可见渲染位（两条都要，缺一即红） | 渲染出来长什么样 = 真机门 |
 * | 6 删手绘吉祥物 | 原手绘 class 名族零命中 + `logo.png` 图像槽位在位 | 图片视觉 = 真机门 |
 * | 7 协议行抽共享件 | 两页 import **同一件**、两页模板不再自持《用户协议》文案、共享件最小面（props/emits 清单 + 表单态不得入内）、**注册入口按 BASE 各通道都在**（F4：判据 7 只授权换文案，未授权增删入口） | 同视觉（`loginContract` 的模板 sha 锁已批准变更） |
 * | 探测件两页共享 | 两页 import **同一符号** + 两条判据的取数入口（端别 `uniPlatform` / 能力 `getProviderSync`）全仓**只出现在探测件里**（两份实现即红）；返回面是**两个**能力判断（展示面 / 接通面），能力半**已停用**（#1482 接回） | 探测结论在真机上对不对 = 真机门（且 `getProviderSync` 在本仓的**编译面**未验，见报告 ④c） |
 * | 协议详情提示一句文案 | 「详情页建设中」在内联面上**只住 `utils/agreementNotice.uts`**，三页只是转投（F2：曾有三个副本） | 端到端文案由 `landingLoginBehavior` C3-C6 真跑单点 |
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
 * - 不锁 register 页协议行的**渲染面**（#1478 只管启动页 + 登录页两页同源；那份 UI 是已登记的
 *   遗留，归 #1483）。但第 1 轮评审 F2 把「详情页建设中」这句**文案来源**收成全局单点，
 *   register 的 composable 只是一行转投 ⇒ 可观测行为逐字不变，本套件的单点锁因此三页都管。
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
const REGISTER_FORM = 'pages/register/composables/useRegisterForm.uts';
const NOTICE = 'utils/agreementNotice.uts';
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

/**
 * 取一次函数调用的**实参文本**：`openIdx` 指向左括号之后第一个字符，按圆括号平衡读到收尾。
 *
 * 为什么要这个函数（第 1 轮评审 F3）：旧版 `landingJump` 用一条正则
 * `uni\.reLaunch\(\{?\s*url: '…'` 一步吃完，那要求 `url` 是调用对象的**第一个属性**。
 * `uni.reLaunch({ fail: () => {…}, url: '/pages/dashboard/dashboard' })` 这类换序 / 嵌套形态
 * 会**绕过检测** —— 第二套落地跳转漏检而测试仍绿（假绿）。现在先把整段实参摘出来，
 * 再在里面找 `url`，**属性序无关**。
 */
function callArgs(code, openIdx) {
  let depth = 1;
  for (let i = openIdx; i < code.length; i += 1) {
    if (code[i] === '(') depth += 1;
    else if (code[i] === ')') {
      depth -= 1;
      if (depth === 0) return code.slice(openIdx, i);
    }
  }
  return code.slice(openIdx);
}

/** 四个导航 API 每次调用带上的 `/pages/...` 落点（嵌套回调里的也算进去，宁可多抓不可漏抓） */
function navUrls(code) {
  const urls = [];
  const re = /uni\.(reLaunch|switchTab|redirectTo|navigateTo)\(/g;
  let m;
  while ((m = re.exec(code)) !== null) {
    for (const u of callArgs(code, re.lastIndex).matchAll(/url:\s*'(\/pages\/[^']*)'/g)) urls.push(u[1]);
  }
  return urls;
}

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
    probeDestructured: /const \{ wechatAvailable, wechatLoginReady \} = useLoginProviders\(\)/.test(src),
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
/** 新用户落点（出口的第一条分支）：着陆页若自持一份指向它的跳转就是第二套成功链路 */
const CHOOSE_CERT = '/pages/guide/choose-cert';

/**
 * 判据本体：源文件集合 → 违例清单（真源与注入自检共用）。
 * `landingJump` 的口径要说清：着陆页**允许**的那条 reLaunch 是 AC 1 的已登录冷启动守卫，
 * 它不属于「登录成功跳转」⇒ 扫描前先把那条守卫块摘掉，剩下的任何指向 dashboard / choose-cert
 * 的跳转都是第二套成功链路（判据 4 要抓的就是它）。
 * 匹配形态（第 1 轮评审 F3）：**属性序无关** —— 逐条调用摘出实参再看 `url`，
 * 不再要求 `url` 是第一个属性（旧正则对 `{ fail: …, url: … }` 形态漏检）。
 */
function outletFacts(sources) {
  const hits = { toast: [], define: [], landingJump: [], landingJumpUrls: [] };
  for (const [rel, src] of sources) {
    let code = stripAll(src);
    const guard = braceBlock(code, COLDSTART_GUARD);
    if (guard !== null) code = code.replace(guard, '');
    if (code.includes(NEW_USER_TOAST)) hits.toast.push(rel);
    if (/export function afterLoginSuccess\s*\(/.test(code)) hits.define.push(rel);
    if (rel.startsWith('pages/index/')) {
      const landed = navUrls(code).filter((u) => u.startsWith('/pages/dashboard') || u.startsWith('/pages/guide'));
      if (landed.length > 0) {
        hits.landingJump.push(rel);
        hits.landingJumpUrls.push([rel, landed]);
      }
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

  it('判别力（F3：属性序无关）：url 不在首位 / 嵌在 fail 之后 / switchTab 换 API，三种形态同样必红', () => {
    // 旧的正则 `uni\.reLaunch\(\{?\s*url: '…'` 要求 url 是**第一个**属性，下面第一条就会静默漏检 ⇒ 测试假绿。
    const reordered = [[LANDING, `${read(LANDING)}\n    uni.reLaunch({\n        fail: (err) => { console.error(err) },\n        url: '${DASHBOARD}'\n    })\n`]];
    const f1 = outletFacts(reordered);
    expect(f1.landingJump).toEqual([LANDING]);
    expect(f1.landingJumpUrls).toEqual([[LANDING, [DASHBOARD]]]);
    // 换 API 也抓（第二条判据 4 的另一套落地形态）
    const switched = [[LANDING, `${read(LANDING)}\n    uni.switchTab({\n        success: () => {},\n        url: '${DASHBOARD}'\n    })\n`]];
    expect(outletFacts(switched).landingJump).toEqual([LANDING]);
    // 引导页落点（choose-cert）走嵌套 fail 也抓
    const cert = [[LANDING, `${read(LANDING)}\n    uni.redirectTo({ fail: () => {}, url: '${CHOOSE_CERT}' })\n`]];
    expect(outletFacts(cert).landingJumpUrls).toEqual([[LANDING, [CHOOSE_CERT]]]);
    // 对照组：真源同一条判据必须仍是空集（防判据被改宽成「任何跳转都算」的恒红/恒绿两用）
    expect(outletFacts(sources).landingJump).toEqual([]);
    expect(navUrls(stripAll(read(LANDING)))).toEqual(['/pages/register/register', '/pages/login/login']);
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
  const tpl = stripTpl(block(src, 'template'));
  const props = (/withDefaults\(defineProps<\{([\s\S]*?)\}>\(\)/.exec(src) || [null, ''])[1];
  const emits = (/defineEmits\(\[([^\]]*)\]\)/.exec(src) || [null, ''])[1];
  return {
    propNames: [...props.matchAll(/^\s*([A-Za-z_$][\w$]*)\s*\??\s*:/gm)].map((m) => m[1]).sort(),
    emitNames: [...emits.matchAll(/'([A-Za-z_$][\w$]*)'/g)].map((m) => m[1]).sort(),
    formState: ['countdown', 'sendingCode', 'captcha', 'phoneCode', 'emailCode', 'password', 'loading']
      .filter((token) => src.includes(token)),
    links: {
      agreement: count(tpl, '《用户协议》'),
      privacy: count(tpl, '《用户隐私》'),
    },
    channelHints: {
      wechat: src.includes("'未注册的微信号将自动注册账号，'"),
      email: src.includes("'若邮箱未注册，'"),
      phone: src.includes("'若手机号未注册，'"),
    },
    // 「立即注册」入口（第 1 轮评审 F4）：BASE 里它是**无条件**存在的（登录页那行没有按通道的
    // v-if），brief 判据 7 授权的只有「同源同文案 + 按通道切换的注册引导**文案**」⇒ 入口不许按通道收掉。
    registerLinkCount: count(tpl, '立即注册'),
    registerLinkWired: /<text class="la-link" @click\.stop="onRegister">立即注册<\/text>/.test(tpl),
    registerLinkGated: /\bv-(?:if|show)="[^"]*"\s+@click\.stop="onRegister"/.test(tpl) || src.includes('showRegisterLink'),
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

  it('注册引导按通道分叉（判据 7 授权的就是这一件事：换**文案**），而「立即注册」入口各通道都在（F4 返工）', () => {
    expect(facts.channelHints).toEqual({ wechat: true, email: true, phone: true });
    expect(facts.registerLinkCount).toBe(1);
    expect(facts.registerLinkWired).toBe(true);
    expect(facts.registerLinkGated).toBe(false);
  });

  it('判别力：页面把文案抄回来 / 共享件多塞一个表单态字段，两种都必红', () => {
    const copied = read(PAGE).replace('<LoginAgreement', '<text>《用户协议》</text>\n            <LoginAgreement');
    expect(count(stripTpl(block(copied, 'template')), '《用户协议》')).toBe(1);
    const fattened = read(AGREEMENT).replace("channel? : string", "channel? : string\n        countdown? : number");
    expect(agreementFacts(fattened).propNames).toContain('countdown');
    expect(agreementFacts(read(AGREEMENT))).toEqual(facts);
  });

  it('判别力（F4）：把入口再按通道收回去（评审越界的那一版）/ 删掉入口，两种都必红', () => {
    const src = read(AGREEMENT);
    const gatedByChannel = src.replace('<text class="la-link" @click.stop="onRegister">立即注册</text>',
      '<text class="la-link" v-if="channel != \'wechat\'" @click.stop="onRegister">立即注册</text>');
    expect(gatedByChannel).not.toBe(src);
    expect(agreementFacts(gatedByChannel).registerLinkGated).toBe(true);
    const removed = src.replace('        <text class="la-link" @click.stop="onRegister">立即注册</text>\n', '');
    expect(removed).not.toBe(src);
    expect(agreementFacts(removed).registerLinkCount).toBe(0);
    expect(agreementFacts(removed).registerLinkWired).toBe(false);
    // 对照组：真源跑同一条判据不红（入口在、无门禁）
    expect(agreementFacts(src).registerLinkGated).toBe(false);
    expect(agreementFacts(src).registerLinkCount).toBe(1);
  });
});

describe('AC 探测件两页共享同一符号（平台判据全仓唯一，两份实现即红）', () => {
  const sources = allSources();

  it('两页都 import 根 composables/useLoginProviders 并解构**两个面**（展示面 + 接通面）', () => {
    for (const rel of [PAGE, LOGIN_PAGE]) {
      expect(read(rel)).toContain("import { useLoginProviders } from '../../composables/useLoginProviders'");
      expect(read(rel)).toContain('const { wechatAvailable, wechatLoginReady } = useLoginProviders()');
    }
  });

  it('两条判据的取数入口（端别 `uniPlatform` / 能力 `getProviderSync`）只出现在探测件里 —— 多一个文件即「两份实现」', () => {
    // F1 返工后判据是复合的（端别 || provider），所以**两个**取数入口都得守：
    // 只守 uniPlatform 会放行「着陆页自己再打一次 getProviderSync」这种第二套探测。
    for (const token of ['uniPlatform', 'getProviderSync']) {
      const holders = sources
        .filter(([, src]) => stripAll(src).includes(token))
        .map(([rel]) => rel);
      expect([token, holders]).toEqual([token, [PROBE]]);
    }
  });

  it('两个面的接线面（F1）：展示面 = 端别(小程序 || App)、接通面 = 端别(仅小程序)；能力半**不接**任何一面', () => {
    const code = stripJs(read(PROBE));
    expect(code).toContain("const PLATFORM_MP_WEIXIN = 'mp-weixin'");
    expect(code).toContain("const PLATFORM_APP = 'app'");
    expect(code).toContain("const PROVIDER_OAUTH_WEIXIN = 'weixin'");
    expect(code).toContain("uni.getProviderSync({ service: 'oauth' })");
    // 展示面：App 与小程序一律展示该入口（2026-10-03 维护者口径「移动端也展示，有个样式即可」）
    expect(code).toMatch(/return onMpWeixin \|\| onApp/);
    // 接通面：只有小程序端为真（App 端 manifest 未配微信 SDK，见 #1482）
    expect(code).toMatch(/const wechatLoginReady = computed<boolean>\(\(\) : boolean => \{\s*return onMpWeixin\s*\}/);
    // 能力半函数仍在（#1482 的接缝 + ④c 的零先例 API 证据面），但**不得**被任何一面调用 ——
    // 它接回判据就是 2026-10-03 ①a 真机判红的那个假阳性（未配 SDK 的 Android 上 providerIds 恒含 weixin）
    expect(code).toMatch(/function hasWechatOauthProvider\(\) : boolean \{/);
    expect(code).not.toMatch(/return onMpWeixin \|\| hasWechatOauthProvider\(\)/);
    expect(code).toMatch(/try \{[\s\S]*?\} catch \(e\) \{\s*return false\s*\}/);
  });

  it('探测件返回面是**两个能力判断**（展示面 / 接通面，供 #1484 决定 disabled），不是「要不要渲染」的布尔', () => {
    const src = read(PROBE);
    const declared = [...src.matchAll(/^\s{4}([A-Za-z_$][\w$]*)\s*:/gm)].map((m) => m[1]).sort();
    expect(declared).toEqual(['wechatAvailable', 'wechatLoginReady', 'wechatUnavailableReason']);
    expect(src).toContain('wechatAvailable : ComputedRef<boolean>');
    expect(src).toContain('wechatLoginReady : ComputedRef<boolean>');
    expect(src).not.toMatch(/shouldRender|showWechat|visible\b/);
  });

  it('判别力：着陆页自己抄一份平台判断、或自己再打一次 provider 探测，两种都必红（计数从 1 变 2）', () => {
    const copied = sources.concat([['pages/whatever/whatever.uts', "const p = uni.getSystemInfoSync().uniPlatform\nexport const x = p == 'mp-weixin'"]]);
    const holders = copied.filter(([, src]) => stripAll(src).includes('uniPlatform')).map(([rel]) => rel);
    expect(holders).toContain('pages/whatever/whatever.uts');
    expect(holders.length).toBeGreaterThan(1);
    const secondProbe = sources.concat([['pages/whatever/whatever.uts', "const r = uni.getProviderSync({ service: 'oauth' })\nexport const y = r.providerIds.indexOf('weixin')"]]);
    const probeHolders = secondProbe.filter(([, src]) => stripAll(src).includes('getProviderSync')).map(([rel]) => rel);
    expect(probeHolders).toEqual([PROBE, 'pages/whatever/whatever.uts']);
  });
});

// ── 协议详情提示全仓单点（第 1 轮评审 F2：那句 toast 在本票之前已被抄成第三份） ──

/** 这句 toast 的唯一字面量来源（三页都只是转投；副本数 = 内联面上命中该文案的文件数） */
const NOTICE_TEXT = '详情页建设中';

/**
 * 判据本体：谁**内联**了这句文案（= 副本），谁**引用**了单点（= 消费者）。
 * 执法形态与本仓 `recruitWorkspaceContract`「这句话只住在 utils/recruitDisplay.uts」同房：
 * 内联处必须**恰为单点自己一处** ⇒ 白名单为空，「再来一份」不需要改判据就会红。
 * 注册页也在消费者名单里：F2 收的是**文案来源**，它的可观测行为逐字不变（UI/行为面留给 #1483）。
 */
function noticeFacts(sources) {
  const inliners = [];
  const consumers = [];
  for (const [rel, src] of sources) {
    const code = stripAll(src);
    if (code.includes(NOTICE_TEXT)) inliners.push(rel);
    if (code.includes('utils/agreementNotice')) consumers.push(rel);
  }
  return { inliners, consumers };
}

/** 判据本体：某个 `showAgreement` 是真委派还是自带一份（`calls`=转投次数，`localToast`=本地又 toast 了一遍） */
function delegateFacts(src) {
  const body = braceBlock(stripJs(src), 'function showAgreement(');
  if (body === null) return { found: false, calls: 0, localToast: false };
  return { found: true, calls: count(body, 'showAgreementNotice(name)'), localToast: /uni\.showToast/.test(body) };
}

describe('F2 协议详情提示收成全仓单点：三页只转投，副本数 = 0（单一事实源硬判据）', () => {
  const sources = allSources();
  const facts = noticeFacts(sources);
  const DELEGATED = { found: true, calls: 1, localToast: false };

  it('这句文案在内联面上只住 utils/agreementNotice.uts 一处（登录页 / 注册页 / 着陆页都不再自持）', () => {
    expect(facts.inliners).toEqual([NOTICE]);
  });

  it('三页都引用同一个件（本票的两页 + 注册页只收文案来源）', () => {
    expect(facts.consumers).toEqual([LANDING, LOGIN_FORM, REGISTER_FORM].sort());
  });

  it('三个 `showAgreement` 都是纯转投：把协议名交出去，本地不再拼那句 toast', () => {
    for (const rel of [LANDING, LOGIN_FORM, REGISTER_FORM]) {
      expect([rel, delegateFacts(read(rel))]).toEqual([rel, DELEGATED]);
    }
  });

  it('单点自己的出口形态与搬走前逐字一致（一次 toast、icon none、协议名 + 后缀）⇒ 行为不变是判据的一部分', () => {
    const code = stripJs(read(NOTICE));
    expect(code).toContain(`const AGREEMENT_NOTICE_SUFFIX = '${NOTICE_TEXT}'`);
    expect(code).toContain("uni.showToast({ title: name + AGREEMENT_NOTICE_SUFFIX, icon: 'none' })");
    expect(count(code, 'uni.showToast(')).toBe(1);
    expect(count(code, NOTICE_TEXT)).toBe(1);
  });

  it('判别力：抄回着陆页 / 新开第四副本 / 摘掉委派，三种都必红（对照：真源跑同一条判据全绿）', () => {
    const copied = sources.concat([[LANDING, `${read(LANDING)}\n    const dup = '${NOTICE_TEXT}'\n`]]);
    expect(noticeFacts(copied).inliners).toContain(LANDING);
    const fourth = sources.concat([['pages/whatever/whatever.uts', `uni.showToast({ title: name + '${NOTICE_TEXT}', icon: 'none' })\n`]]);
    expect(noticeFacts(fourth).inliners).toEqual([NOTICE, 'pages/whatever/whatever.uts']);
    const unlinked = sources.filter(([rel]) => rel !== LANDING).concat([[LANDING, read(LANDING).replace("import { showAgreementNotice } from '../../../utils/agreementNotice'", 'void 0')]]);
    expect(noticeFacts(unlinked).consumers).not.toContain(LANDING);
    const reverted = delegateFacts(read(LANDING).replace('        showAgreementNotice(name)',
      `        uni.showToast({ title: name + '${NOTICE_TEXT}', icon: 'none' })`));
    expect(reverted).toEqual({ found: true, calls: 0, localToast: true }); // = 评审前的第三副本形态 ⇒ 上面那条必红
    // 对照组
    expect(noticeFacts(sources)).toEqual(facts);
    expect(delegateFacts(read(LANDING))).toEqual(DELEGATED);
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
    // 探测件三处：展示面 / 接通面（#1487 ①a 真机口径新增）/ 不可用原因
    expect(computedSites(read(PROBE))).toHaveLength(3);
    // F4 返工后协议件只剩一处 computed（按通道的引导**文案**）：原先那处「按通道收掉入口」
    // 的 showRegisterLink 已删除 —— 站点数从 2 掉到 1 正是那次越界被收回的证据。
    expect(computedSites(read(AGREEMENT))).toHaveLength(1);
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
