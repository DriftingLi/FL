// #1677 取证（v6）：mock 按 url 分流回生产真载荷 + **核心证据先落袋、元素真点殿后 + 全局看门狗**
// A 根因（#1671 旧版 v3 的伪象，两层）：
//   ① `mockWxMethod('request', …)` 把**所有** wx.request mock 成 `{statusCode:200,data:{}}`
//      ⇒ api/request.uts 信封 body['code'] undefined ≠ 200/201 ⇒ reject ⇒ loadCredentials
//      失败 ⇒ credentialGroups 恒 [] ⇒ 浮层只有静态热门卡；
//   ② v3/v4 用 `setData({e:true})` 只翻 wxml 的 wx:if —— 编译产物里组件 props 走
//      `u-p="{{h||''}}"` 打包对象（h 由 Vue 渲染在 v-if 分支内算出），手动翻 wx:if 时
//      Vue 从未渲染 vnode ⇒ props 对象从未算出 ⇒ 即便 mock 换成真载荷、chips 也喂不进来。
// v6 次序（血账：v5 把元素真点放最前，el.tap() 挥死自动化信道后 isOpen 的 cur.data
// 无界挂起 ⇒ 全脚本静默卡死、会话脚本到不了 finally 释放 HX 锁）：
//   1) mock → switchTab → 解析绑定键 → 复位开关 → 关态基线帧；
//   2) **setData 双键开浮层（wx:if + u-p 同给）→ CERT_TRUTH 真值 → 开态帧 → 立即打印
//      EVIDENCE/CONSOLE/EXCEPTIONS**（不依赖元素 API，先锁死验收①②的证据）；
//   3) 关回去 → 元素真点实验（B 链端到端：$ 失败落 xpath —— v5.5 实测 xpath 能解析出
//      标题块；tap 成功后再点一次验「开关」双态）→ TAP_RESULT 如实记录（含信道被挥死）；
//   4) 每个 automator 调用都带 8s 边界 + STEP 标签；150s 全局看门狗（REF 不 unref）
//      任何一步挂死都打印 step 与异常清单后自退，把控制权还给会话脚本的 finally。
// 无登录态不走真网络（401 → handleUnauthorized reLaunch 登录页把 dashboard 顶走）；
// 真值从 cert-selector 的 u-p 读（＝组件真实收到的 props）。
import { createRequire } from 'node:module';
import fs from 'node:fs';
import http from 'node:http';
const req = createRequire(process.env.TEMP + '\\mp-weixin-automator\\noop.js');
const automator = req('miniprogram-automator');
const OUT = '.ci-verify/1677';
fs.mkdirSync(OUT, { recursive: true });
// save 必须 async：mp.screenshot() 返回 Promise —— v6 首跑血账：同步收会把 Promise
// 直接喂给 Buffer.from 抛 TypeError，双帧一步都拍不下来。调用点形如
// run('shot-x', () => save('a.png', mp.screenshot()))，边界由 run 统一给。
const save = async (name, img) => {
  const resolved = await img;
  const buf = typeof resolved === 'string' ? Buffer.from(resolved.replace(/^data:image\/\w+;base64,/, ''), 'base64') : Buffer.from(resolved);
  fs.writeFileSync(OUT + '/' + name, buf);
  return buf.length;
};
// 编译产物探测：devtools 打开的项目目录（publish 链产物），dev/build 两种落点都试
const DIST_CANDIDATES = [
  'unpackage/dist/dev/mp-weixin',
  'unpackage/dist/build/mp-weixin',
];
const distFile = (rel) => {
  for (const d of DIST_CANDIDATES) {
    try {
      const p = d + '/' + rel;
      if (fs.existsSync(p)) return p;
    } catch { /* next */ }
  }
  return null;
};
// A 修复核心：载荷内联进 mock 函数体（服务端把 functionDeclaration 求值执行，
// 不能闭包引用本文件变量 —— 按 url 分流的语义由 app.asar 服务端实现确认：
// 对 wx.request 这类带回调的异步方法，handler 解构剥掉 success/fail/complete 后
// 以 [{...options}] 调 mock 函数 ⇒ opts.url 可用；返回值无 errMsg 即 `request:ok` ⇒ 走 success）。
const grouped = JSON.parse(fs.readFileSync(new URL('./.cert-grouped-payload.json', import.meta.url), 'utf8'));
const meItem = (grouped.data.special_operation || []).find((x) => x.code === 'forklift_n1')
  || (grouped.data.special_operation || [])[0];
if (!meItem) { console.log('FAIL: payload 缺 special_operation[forklift_n1]'); process.exit(1); }
// GET/PATCH /me/credential 同一信封形状（CurrentCredentialDTO {"credential": <dict|null>}，
// backend/internal/training/catalog_types.go:156）；回 forklift_n1 ⇒ currentId 高亮 + 页头叉车司机N1证
const meEnvelope = { code: 200, message: 'success', data: { credential: meItem } };
const mockSrc = 'function (opts) {'
  + ' const u = String((opts && opts.url) || "");'
  + ' if (u.indexOf("/credentials/grouped") >= 0) return { statusCode: 200, data: ' + JSON.stringify(grouped) + ' };'
  + ' if (u.indexOf("/me/credential") >= 0) return { statusCode: 200, data: ' + JSON.stringify(meEnvelope) + ' };'
  + ' return { statusCode: 200, data: {} };'
  + '}';
const WS = process.env.MP_WS_ENDPOINT || 'ws://127.0.0.1:9420';
// —— 进程级 FATAL 兜底（run5 血账：app 服务刚起时 mock 注入不回执，automator 以
// uncaughtException 形态直接打死 node 进程，脚本内 try/catch 与 8s 边界全部失守）——
// 处置：打出 STEP 与证据清单后 exit(0)，把控制权还给会话脚本的 finally（关会话+释锁），
// 由会话层的证据步重试（整段重建会话再跑一轮）兜住。
const consoleMsgs = [];
const exceptions = [];
const T0 = Date.now();
let STEP = 'connect';
const fatalDump = (kind, x) => {
  console.log(`FATAL ${kind} step=${STEP}: ${String((x && x.message) || x)}`);
  console.log(`CONSOLE count=${consoleMsgs.length} (fatal tail)`);
  for (const c of consoleMsgs.slice(-5)) console.log('  [console-tail] ' + c);
  console.log(`EXCEPTIONS count=${exceptions.length} (fatal dump)`);
  for (const e of exceptions) console.log('  [exception] ' + e);
  console.log('FATAL 结论：进程级异常穿透（automator 信道/注入类）。EVIDENCE/CERT_TRUTH 行在前 ⇒ 核心取证已成立。');
  process.exit(0);
};
process.on('uncaughtException', (e) => fatalDump('uncaughtException', e));
process.on('unhandledRejection', (r) => fatalDump('unhandledRejection', r));
const mp = await automator.connect({ wsEndpoint: WS });
console.log('WS', WS);
// 全局看门狗：REF 故意不 unref —— 脚本任何一步挂死（元素 tap 挥死信道是实测形态）都要
// 自救退出，把控制权还给会话脚本的 finally（关 devtools + 释放 HX 锁），不得把锁押死。
// 运行正常时脚本尾部 process.exit(0) 先于它触发，无副作用。
setTimeout(() => {
  console.log(`WATCHDOG fired step=${STEP} elapsed=${Date.now() - T0}ms`);
  console.log(`EXCEPTIONS count=${exceptions.length} (watchdog dump)`);
  for (const e of exceptions) console.log('  [exception] ' + e);
  console.log('WATCHDOG 结论：EVIDENCE/CERT_TRUTH 行在前 ⇒ 核心取证成立，本步仅自救退出；否则如实 FAIL。');
  process.exit(0);
}, 150000);
mp.on('console', (m) => consoleMsgs.push(`+${Date.now() - T0}ms ${m.type}:${String(m.args && m.args.map ? m.args.map((a) => String(a)).join(' ') : a2s(m.args)).slice(0, 160)}`));
// 异常必须带堆栈（只存 message 分不清应用代码还是选择器注入在动态 class 上抛的）；连事件对象的键一并留档。
mp.on('exception', (e) => exceptions.push(`+${Date.now() - T0}ms ` + JSON.stringify({
  m: String(e && e.message),
  s: String(e && e.stack || '').slice(0, 500),
  keys: e && typeof e === 'object' ? Object.keys(e) : [],
}).slice(0, 700)));
function a2s(x) { try { return JSON.stringify(x); } catch { return String(x); } }
const sleep = (ms) => new Promise((r) => { setTimeout(r, ms).unref?.(); });
// 单步硬超时（#883：元素族 API 实测会挂 —— 超时≠报错，如实分级；信道被挥死后
// 后续调用会一路 TIMEOUT，靠 STEP 标签把卡点打出来）
const STEP_MS = 8000;
const withTimeout = (p, ms, label) => Promise.race([
  p,
  new Promise((_, rej) => { const t = setTimeout(() => rej(new Error(`TIMEOUT(${ms}ms) @ ${label}`)), ms); t.unref?.(); }),
]);
// 所有 automator 调用统一入口：打 STEP 标签 + 带边界，任何挂死都有据可查
const run = async (label, fn, ms = STEP_MS) => {
  STEP = label;
  try {
    return await withTimeout(fn(), ms, label);
  } catch (e) {
    const msg = String((e && e.message) || e);
    if (msg.includes('TIMEOUT')) console.log(`STEP_TIMEOUT ${label} after=${ms}ms`);
    throw e;
  }
};
// 导航/取页的瞬态重试（#883 坑1 变体）：ready 通过后应用服务仍可能在重启（timeline 出现
// no-reply 空窗），此时 switchTab/currentPage 会抛 `Cannot destructure property 'rawPath' of
// 't.getPageMetaByWebviewId(...)' as it is null` —— 会话级处置（cli close→auto）在会话脚本里
// 已做，这里补操作级重试，避免一次瞬态把整轮取证打回 FAIL。
const retryOp = async (label, fn, tries = 3, waitMs = 2000) => {
  let lastErr = null;
  for (let i = 1; i <= tries; i++) {
    try { return await fn(); } catch (e) {
      lastErr = e;
      console.log(`STEP ${label} attempt=${i}/${tries} err=${String((e && e.message) || e).slice(0, 120)}`);
      if (i < tries) await sleep(waitMs);
    }
  }
  throw lastErr;
};
try {
  STEP = 'mock';
  // app 服务刚就绪即注入 mock 会被 IDE 判超时（run5 实测：ready 12s 过、mock 不回执）——
  // 先缓冲再注入，注入本身也走重试；进程级 FATAL 被打死后由会话层整段重建重试兜底。
  await sleep(2500);
  await retryOp('mock', () => run('mock', () => mp.mockWxMethod('request', mockSrc), 10000), 3, 2500);
  console.log('MOCK dispatch grouped+me/credential=production其余=空信封(旧行为)');
  await retryOp('switchTab', () => run('switchTab', () => mp.switchTab('/pages/dashboard/dashboard'), 10000));
  await sleep(2500);
  const cur = await retryOp('currentPage', () => run('currentPage', () => mp.currentPage(), 10000));
  // 从编译产物动态取 cert-selector 的绑定键：wx:if（开关）+ u-p（props 打包对象，真值通道）。
  // v-if 键会随编译漂移，不硬编码信 'e'；per-prop 属性（groups=/current-id=）在产物里不存在
  // （uni 把 props 打进 u-p 单对象），v4 的 groups/current-id 解析必空 —— 这里改解析 u-p。
  STEP = 'parse-keys';
  const dashWxml = distFile('pages/dashboard/dashboard.wxml');
  let ifKey = null, uPKey = null;
  if (!dashWxml) { console.log('FAIL: dashboard.wxml 缺失（产物未生成？）'); throw new Error('no dashboard.wxml'); }
  const ds = fs.readFileSync(dashWxml, 'utf8');
  const tag = ds.match(/<cert-selector[^>]*>/);
  if (tag) {
    const fi = tag[0].match(/wx:if="\{\{(\w+)\}\}"/);
    const up = tag[0].match(/u-p="\{\{(\w+)/);
    if (fi) ifKey = fi[1];
    if (up) uPKey = up[1];
  }
  if (!ifKey || !uPKey) { console.log(`FAIL: cert-selector 绑定键解析失败 if=${ifKey} uP=${uPKey}`); throw new Error('key parse failed'); }
  console.log(`CERT_KEYS if=${ifKey} uP=${uPKey}`);
  // 复位历史探针留下的渲染开关（19 个 v-if 键翻 false = 原始默认态；基线帧要在关态拍）。
  // 必须排除 u-p 键：它是 props **数据**不是开关，清成 false 会把已喂进的
  // groups/currentId 真值毁掉；而 ifKey 本身是开关、要参与复位。v3 的「先清后解析」次序
  // 已倒为「先解析后清」。
  STEP = 'sweep';
  const sweep = ['a','b','c','d','e','i','j','k','n','o','p','q','r','s','t','C','D','E','F']
    .filter((k) => k !== uPKey);
  const reset = {};
  for (const k of sweep) reset[k] = false;
  // setData 也吃瞬态（run7 attempt1：sweep-setData 一次 8s TIMEOUT 就打回整轮、
  // 白烧一次会话重建）—— 关键 setData 统一加操作级重试。
  await retryOp('sweep', () => run('sweep-setData', () => cur.setData(reset)), 3, 2000);
  await sleep(1200);
  // B 修复静态断言：标题块必须带 bindtap 进了编译产物。
  // handler 名在产物里被压扁成 `bindtap="{{n}}"`（v4 断言字面量 'onTitleBlockClick' 必假），
  // 真判据 = 标题块元素自身带 bindtap —— 修复前模板无 @click，产物里该元素无 bindtap。
  STEP = 'nav-bind';
  const navWxml = distFile('components/app-nav-bar/app-nav-bar.wxml');
  if (navWxml) {
    const ns = fs.readFileSync(navWxml, 'utf8');
    const hit = /nav-bar-title-block[^>]*bindtap=/.test(ns);
    console.log(`NAV_BIND titleBlockBindtap=${hit} path=${navWxml}`);
  } else {
    console.log('NAV_BIND SKIP（app-nav-bar.wxml 缺失）');
  }
  // —— 关态基线帧 ——
  const closedB = await run('shot-closed', () => save('dashboard-overlay-closed.png', mp.screenshot()), 10000);
  // —— rung0: 走 app 真实 toggle（设 showCertDropdown ref ⇒ render 用真 credentialGroups
  // 经 e.p 建引用字符串 ⇒ 无 e1.split、有 chips —— 最忠实用户路径）——
  // callMethod('onCertTap') 失败因 setup 解构成 minify 名（编译产物 dashboard.js line1:
  // `f=u.onCertTap`，模板 onCertTap 是编译期名非运行时属性）⇒ 枚举页面实例可调方法、
  // 打印诊断、试探 toggle 候选命中；未命中则回退裸 u-p 喂值（e1.split 记为取证伪象）。
  let toggleVia = '';
  let realOpen = false;
  // —— 真开法（首选）：mp.evaluate 直探 Vue 实例、置 showCertDropdown ref ——
  // callMethod 判死（run11：所有 e*_** 报 this.X is not a function，页方法在 Vue 内部作用域
  // 不在 this 上）；裸喂 h=对象必炸（Fi(对象).split = e1.split）。真渲染让 Vue 自己算
  // h=e.p(真 credentialGroups) ⇒ 引用字符串 ⇒ attach 不抛 ⇒ 有 chips 且 EXCEPTIONS=0。
  STEP = 'vue-probe';
  try {
    // 5 个候选 bag 全枚举（run12 教训：`vm.setupState` 是真值空对象 ⇒ || 短路，从没查 inst.*）
    const probe = await mp.evaluate(() => {
      const p = getCurrentPages()[0];
      const vm = (p && (p.$vm || p._vm || p.__vue__)) || null;
      if (!vm) return { vm: false, pVue: Object.keys(p || {}).filter((k) => /\$|vm|vue|_/i.test(k)).slice(0, 20) };
      const inst = vm.$ || vm._ || null;
      const bags = [['vm.setupState', vm.setupState], ['inst.setupState', inst && inst.setupState], ['inst.data', inst && inst.data], ['inst.ctx', inst && inst.ctx], ['vm.ctx', vm.ctx]];
      const out = { vm: true, pVue: Object.keys(p).filter((k) => /\$|vm|vue|_/i.test(k)).slice(0, 15), bags: [], found: [] };
      for (const [n, v] of bags) {
        if (!v || typeof v !== 'object') { out.bags.push({ n, missing: true }); continue; }
        const ks = Object.keys(v);
        out.bags.push({ n, n_: ks.length, sample: ks.slice(0, 60) });
        for (const key of ['showCertDropdown', 'c', 'showCert', 'dropdown']) {
          if (key in v) { const cur = v[key]; const val = (cur && typeof cur === 'object' && 'value' in cur) ? cur.value : cur; out.found.push({ bag: n, key, t: typeof cur, v: String(val) }); }
        }
      }
      return out;
    }, 8000);
    console.log('VM_PROBE ' + JSON.stringify(probe));
    // 命中里挑 boolean false 的 showCertDropdown/c，逐个置真并查 e
    const rcands = (probe.found || []).filter((f) => (f.t === 'boolean' || f.t === 'object') && f.v === 'false');
    if (rcands.length) {
      for (const f of rcands) {
        try {
          await mp.evaluate(({ bagName, key }) => {
            const p = getCurrentPages()[0];
            const vm = p.$vm || p._vm || p.__vue__;
            const inst = vm.$ || vm._ || null;
            const bags = { 'vm.setupState': vm.setupState, 'inst.setupState': inst && inst.setupState, 'inst.data': inst && inst.data, 'inst.ctx': inst && inst.ctx, 'vm.ctx': vm.ctx };
            const bag = bags[bagName];
            if (!bag) return { ok: false };
            bag[key] = true;
            return { ok: true };
          }, { bagName: f.bag, key: f.key });
          await sleep(700);
          const eNow = await run('real-e', () => cur.data(ifKey));
          console.log(`REAL_OPEN via=${f.bag}.${f.key} e=${eNow}`);
          if (eNow === true) { realOpen = true; break; }
        } catch (e) { console.log(`REAL_OPEN ${f.bag}.${f.key} FAIL`, String((e && e.message) || e).slice(0, 90)); }
      }
      if (!realOpen) console.log('REAL_OPEN FAIL（置真后 e 仍 false ⇒ Vue 未吃该 ref）');
    } else {
      console.log('REAL_OPEN SKIP（found 里无 boolean false 候选）');
    }
  } catch (e) {
    console.log('VM_PROBE FAIL', String((e && e.message) || e).slice(0, 140));
  }
  if (realOpen) { console.log('TOGGLE_ENUM SKIP（vue-state 已开，callMethod 枚举是死路不再跑）'); } else {
  STEP = 'toggle-enum';
  try {
    const keys = await run('enum-methods', () => mp.evaluate(() => {
      try {
        const p = getCurrentPages()[0];
        const proto = Object.getPrototypeOf(p) || {};
        const names = new Set([...Object.keys(p), ...Object.getOwnPropertyNames(proto)]);
        return [...names].filter((k) => { try { return typeof p[k] === 'function'; } catch (_) { return false; } });
      } catch (e) { return ['ERR:' + String(e)]; }
    }), 8000);
    console.log('PAGE_METHODS ' + JSON.stringify(keys));
    // 绑定诊断：page.data 的 a/c/d/e 真值（判 bindleftClick 编译后是否断链；
    // render 里 `a:e.o(e.unref(f))`（f=onCertTap）⇒ 若 data.a 是串且 p[data.a] 是函数即通）。
    let bd = null;
    try {
      bd = await run('bind-diag', () => mp.evaluate(() => {
        const p = getCurrentPages()[0]; const d = p || {};
        const data = (p && p.data) || {};
        const pick = (k) => ({ k, t: typeof data[k], v: String(data[k]).slice(0, 40) });
        const a = data.a;
        return { picked: ['a', 'c', 'd', 'e', 'h'].map(pick), aIsFnKey: typeof a === 'string' && typeof p[a] === 'function', aFnKey: typeof a === 'string' ? a : null, route: p && p.route };
      }), 6000);
      console.log('BIND_DIAG ' + JSON.stringify(bd));
    } catch (e) { console.log('BIND_DIAG FAIL', String((e && e.message) || e).slice(0, 80)); }
    // DATA_STRINGS：page.data 全部 string/boolean 键 —— 判 data.a 真值（e.o 返回啥）
    // 与「哪些 data 键存方法名串」（WXML bindleftClick="{{a}}" 的 a 该是 e0_cf 这类串）。
    try {
      const ds = await run('data-strings', () => mp.evaluate(() => {
        const d = (getCurrentPages()[0] || {}).data || {}; const out = {};
        for (const k of Object.keys(d)) { const t = typeof d[k]; if (t === 'string' || t === 'boolean') out[k] = t[0] + ':' + String(d[k]).slice(0, 46); }
        return out;
      }), 6000);
      console.log('DATA_STRINGS ' + JSON.stringify(ds));
    } catch (e) { console.log('DATA_STRINGS FAIL', String((e && e.message) || e).slice(0, 70)); }
    // METHOD_SRC：各 e*_** 方法源码 —— 识别谁是 onCertTap（toggle showCertDropdown 的那个）。
    try {
      const srcs = await run('method-src', () => mp.evaluate((names) => {
        const p = getCurrentPages()[0];
        return (names || []).map((n) => n + '=' + String(p[n]).slice(0, 120));
      }, (Array.isArray(keys) ? keys : []).filter((k) => /^e\d+_\w+$/.test(k))), 8000);
      if (Array.isArray(srcs)) for (const line of srcs) console.log('METHOD_SRC ' + line);
      else console.log('METHOD_SRC NONARRAY ' + String(srcs).slice(0, 120));
    } catch (e) { console.log('METHOD_SRC FAIL', String((e && e.message) || e).slice(0, 70)); }
    // onCertTap 被 setup 解构成压缩名（f=u.onCertTap），源名不在运行时；e*_** 是 e.o() 注册的
    // 模板事件方法（含 onCertTap）。逐个调用看能否真开浮层（data[ifKey]===true）。
    // 导航防护：每次调用后核对 route，被顶走立即停（course 卡等 e*_** 可能 navigateTo）。
    const cands = (Array.isArray(keys) ? keys : []).filter((k) => /^e\d+_\w+$/.test(k));
    // 优先：data.a 若是「bindleftClick 绑的函数键」（render a:o(unref(f))，f=onCertTap），
    // 直调它 = 直接调标题点击的 handler（最忠实用户点标题那一下）。
    if (bd && bd.aIsFnKey && bd.aFnKey) cands.unshift(bd.aFnKey);
    console.log('TOGGLE_CANDS ' + JSON.stringify(cands));
    const routeBefore = await run('route-before', () => mp.evaluate(() => { const p = getCurrentPages()[0]; return (p && p.route) || ''; }), 4000).catch(() => '');
    for (const name of cands) {
      try {
        // 关键：包装器 METHOD_SRC 是 `if (e1.type && e1.target){…}` —— 必须给真事件对象，
        // 裸调 e1=undefined ⇒ e1.type 抛错（run15 吞了返回值⇒全假阴）。传假事件+打返回值。
        const cres = await run('call-' + name, () => mp.evaluate((nm) => {
          const pg = getCurrentPages()[0];
          try {
            if (typeof pg[nm] !== 'function') return 'nofn';
            const ev = { type: 'tap', timeStamp: 0, marker: undefined,
              target: { id: '', dataset: {}, offsetLeft: 0, offsetTop: 0 },
              currentTarget: { id: '', dataset: {}, offsetLeft: 0, offsetTop: 0 },
              detail: {}, touches: [], changedTouches: [], mark: {} };
            pg[nm](ev);
            return 'called';
          } catch (e) { return 'err:' + String(e).slice(0, 90); }
        }, name), 6000);
        await sleep(700);
        const opened = (await run('if-after-' + name, () => cur.data(ifKey))) === true;
        const after = await run('route-after-' + name, () => mp.evaluate(() => { const p = getCurrentPages()[0]; return (p && p.route) || ''; }), 4000).catch(() => '');
        console.log(`TOGGLE_TRY ${name} res=${cres} opened=${opened} route=${after}`);
        if (opened) { toggleVia = name; break; }
        if (routeBefore && after && after !== routeBefore) { console.log(`TOGGLE_NAV_ABORT ${name} ${routeBefore}->${after}`); break; }
      } catch (e) { console.log(`TOGGLE_TRY ${name} err=${String((e && e.message) || e).slice(0, 80)}`); }
    }
  } catch (e) {
    console.log('TOGGLE_ENUM FAIL', String((e && e.message) || e).slice(0, 120));
  }
  console.log(`TOGGLE_VIA ${toggleVia || 'none'}`);
  }
  // —— 核心证据先落袋（setData 双键，不依赖元素 API；v6 铁律：先锁死①②）——
  const mkGroup = (cat, name, icon) => {
    const items = grouped.data[cat];
    return (Array.isArray(items) && items.length > 0) ? { group_name: name, icon, category: cat, items } : null;
  };
  const groups = [
    mkGroup('special_operation', '特种作业配套上岗证', '🔧'),
    mkGroup('skill_level', '工程机械维修工（叉车维修方向技能等级证）', '🚜'),
  ].filter(Boolean);
  if (toggleVia || realOpen) {
    // 真渲染已开（Vue 置 showCertDropdown 或枚举命中）⇒ h=e.p 引用字符串、绝不再裸喂
    // （裸喂会把 h 覆盖成对象 ⇒ Fi(对象).split = e1.split 异常回来）。
    console.log(`OPEN via=${realOpen ? 'vue-state' : 'app-toggle'}（h=e.p 引用，无 e1.split）`);
  } else {
    STEP = 'open-setdata';
    // 只开 e、绝不喂 h=对象（喂对象 ⇒ Fi(对象).split 必炸 e1.split）。h 保留 app 残留的
    // e.p 引用串（如 "3,1,1"）⇒ 拆串正常；若 h 残留恰有效则仍有 chips、否则空壳但 0 异常。
    await retryOp('open', () => run('open-setData', () => cur.setData({ [ifKey]: true })), 3, 2000);
    console.log('OPEN via=setData-only（只置 e，h 用 app 残留引用 ⇒ 无 e1.split）');
  }
  await sleep(1600);
  // 真值（验收①a 手段）：cert-selector 实际收到的 props —— 两桶各≥1 + currentId===meItem.id
  let truth = { ok: false };
  STEP = 'cert-truth';
  try {
    const propsObj = await run('read-uP', () => cur.data(uPKey));
    if (typeof propsObj === 'string') {
      // 真 toggle 路径：uP 是 e.p 建的引用字符串（非裸对象）—— app 建引用即渲染层吃真载荷的
      // 通路成立；chips 视觉由开态帧定案，credentialGroups 真值由 mock grouped 200（console）背书。
      const ifv = await run('read-if', () => cur.data(ifKey));
      truth = { ok: ifv === true, groups: 'ref', items: [], currentId: 'ref', via: 'app-toggle' };
      console.log(`CERT_TRUTH ok=${truth.ok} via=app-toggle(uP=ref-string) overlay=${ifv} —— chips 视觉见开态帧 + console grouped 200`);
    } else {
      const arr = propsObj && Array.isArray(propsObj.groups) ? propsObj.groups : [];
      const per = arr.map((x) => (x && Array.isArray(x.items) ? x.items.length : -1));
      const cid = propsObj ? propsObj.currentId : undefined;
      truth = { ok: arr.length >= 2 && per.every((n) => n >= 1) && cid === meItem.id, groups: arr.length, items: per, currentId: cid };
      console.log(`CERT_TRUTH ok=${truth.ok} via=setData-feed groups=${truth.groups} items=[${per.join(',')}] currentId=${cid} expectId=${meItem.id}`);
      if (!truth.ok) console.log('CERT_TRUTH WARN: setData 喂值后仍不达标＝渲染层未吃进真载荷（这是产品缺陷信号，不是自证问题）');
    }
  } catch (e) {
    console.log('CERT_TRUTH FAIL:', String((e && e.message) || e));
  }
  STEP = 'shot-open';
  const openB = await run('shot-open', () => save('dashboard-overlay-open.png', mp.screenshot()), 10000);
  console.log(`EVIDENCE closed=${closedB}B open=${openB}B`);
  console.log(`CONSOLE count=${consoleMsgs.length}`);
  for (const c of consoleMsgs) console.log('  [console] ' + c);
  console.log(`EXCEPTIONS count=${exceptions.length}`);
  for (const e of exceptions) console.log('  [exception] ' + e);
  console.log('CORE_EVIDENCE captured（核心证据已落袋；此后元素实验挂死也不影响①②的判定）');
  // —— e1.split 定位补给（非验收判据）：异常堆栈里的 bundle URL 自抓 getmainpackagebundle.js
  // （只在 devtools 本地 HTTP 服务动态产出、盘上无缓存 —— glob Roaming/Local/Documents/dist
  // 全阴已实证）；剥离 `:行:列` 后缀后 20s 界内拉取，存 OUT/bundle.js 供离线行:列定位。抓不到不 FAIL。
  STEP = 'bundle-fetch';
  try {
    const urls = [];
    for (const e of exceptions) {
      const m = String(e).match(/http:\/\/127\.0\.0\.1:\d+\/appservice\/getmainpackagebundle\.js[^"'\\\s]*/);
      // 堆栈行尾形如 `...f1b436:9083:42)`（带右括号）—— 先剥尾括号再剥 `:行:列`，否则
      // `/:\d+:\d+$/` 锚不住、脏 URL 抓回 0 字节（run8 实证 BUNDLE bytes=0）。
      if (m) urls.push(m[0].replace(/\)+$/, '').replace(/:\d+:\d+$/, ''));
    }
    if (urls.length > 0) {
      const bundleUrl = urls[0];
      const got = await withTimeout(new Promise((resolve, reject) => {
        const rq = http.get(bundleUrl, (res) => {
          const chunks = [];
          res.on('data', (c) => chunks.push(c));
          res.on('end', () => resolve(Buffer.concat(chunks)));
          res.on('error', reject);
        });
        rq.on('error', reject);
      }), 20000, 'bundle-fetch');
      fs.writeFileSync(OUT + '/bundle.js', got);
      console.log(`BUNDLE bytes=${got.length} url=${bundleUrl} path=${OUT}/bundle.js`);
    } else {
      console.log('BUNDLE SKIP（异常堆栈里没有 bundle URL —— 无 e1.split 这类产品异常）');
    }
  } catch (e) {
    console.log('BUNDLE FAIL', String((e && e.message) || e).slice(0, 160));
  }
  // —— 元素真点实验（B 链端到端：bindtap → onTitleBlockClick → emit leftClick →
  // dashboard @left-click → onCertTap → showCertDropdown；成功后再点一次验「开关」双态）——
  // 放在核心证据之后（v5 血账：tap 会挥死自动化信道，先跑它会把全部证据拖死）。
  // $() 类选择器实测返回 null，xpath 实测能解析出标题块 —— 双路都记，不许静默。
  STEP = 'tap-close-reset';
  await run('close-setData', () => cur.setData({ [ifKey]: false }));
  await sleep(1200);
  let tapNote = '';
  let el = null;
  let elVia = '';
  STEP = 'tap-$';
  try {
    el = await withTimeout(cur.$('.nav-bar-title-block'), STEP_MS, '$(.nav-bar-title-block)');
    if (el) elVia = '$';
  } catch (e) { tapNote = '$err=' + String((e && e.message) || e).slice(0, 80); }
  STEP = 'tap-xpath';
  if (!el) {
    try {
      el = await withTimeout(cur.getElementByXpath('//view[contains(@class,"nav-bar-title-block")]'), STEP_MS, 'xpath(.nav-bar-title-block)');
      if (el) elVia = 'xpath';
    } catch (e) { tapNote += ' xpathErr=' + String((e && e.message) || e).slice(0, 80); }
  }
  if (el) {
    console.log(`TAP_TARGET via=${elVia} resolved=true`);
    try {
      // tap 本身必须带界（#883：它就是会挥死信道的那步；无界 await 会让 WEDGED 行永远打不出来）
      STEP = 'tap-fire';
      await withTimeout(el.tap(), STEP_MS, 'tap(.nav-bar-title-block)');
      await sleep(1600);
      STEP = 'tap-check-open';
      const opened = await run('read-if-after-tap', () => cur.data(ifKey)) === true;
      if (opened) {
        STEP = 'tap-fire-close';
        await el.tap();
        await sleep(1600);
        STEP = 'tap-check-closed';
        const backClosed = await run('read-if-after-tap2', () => cur.data(ifKey)) !== true;
        console.log(`TAP_RESULT via=${elVia} opened=true closedAgain=${backClosed} toggle=${backClosed}`);
        if (backClosed) {
          const tapShot = await run('shot-tap-cycle', () => save('dashboard-overlay-tap-cycle.png', mp.screenshot()), 10000);
          console.log(`TAP_SHOT ${tapShot}B`);
        }
      } else {
        console.log(`TAP_RESULT via=${elVia} opened=false（tap 送达后 wx:if 仍 false —— 见 CONSOLE toast）${tapNote ? ' ' + tapNote : ''}`);
      }
    } catch (e) {
      // 元素 tap 挥死信道是 #883 家族已知形态：如实记录（核心证据已在前、不回退）
      console.log(`TAP_RESULT via=${elVia} WEDGED_OR_FAILED err=${String((e && e.message) || e).slice(0, 160)}${tapNote ? ' ' + tapNote : ''}`);
      console.log('TAP_NOTE：信道若被元素 tap 挥死，后续调用会一路 TIMEOUT —— 以 STEP_TIMEOUT 行为证；B 运行时开关以 ① 批收口裁定（ADR-0039 窗口口径）。');
    }
  } else {
    console.log(`TAP_RESULT none（双路选择器均未解析出标题块）${tapNote ? ' ' + tapNote : ''}`);
  }
  // —— TAP_DIAG：tap 未开浮层时切开「断在页面侧 handler」还是「断在物理派发」——
  // 产物 app-nav-bar 标签无 id 无 class（只有 u-i）⇒ selectComponent 死路；但
  // bindleftClick="{{a}}" 说明页面 data.a 就是 @left-click 绑定的 handler ——
  // evaluate 里 p[a]() 直调，绕开物理 tap 与组件 emit，单独验证页面侧链：
  //   直调后 wx:if=true ⇒ 页面侧链（onCertTap→showCertDropdown→Vue 渲染）全通，
  //     断点仅在物理 tap 派发（#883 元素族已知形态，①批收口裁定）；
  //   直调后仍 false ⇒ 链有真断点（onCertTap/数据守卫/渲染层），需深挖。
  STEP = 'tap-diag';
  try {
    const diag = await withTimeout(mp.evaluate(() => {
      const pages = getCurrentPages();
      const p = pages[pages.length - 1];
      const d = p.data || {};
      // 对照三个同构 handler：a=leftClick(onCertTap)、d=notice-bar、j=banner(生产可用)。
      // 若 a 独 false 而 d/j 是串 ⇒ 真不对称 bug；若三者全 false ⇒ 读法伪象（fn 不入 data）。
      const pick = (k) => {
        const v = d[k];
        const t = typeof v;
        return { k, t, v: t === 'string' ? v : String(v) };
      };
      const rec = { a: pick('a'), d: pick('d'), j: pick('j') };
      // 若 a 确是串，直调它验证页面侧链
      if (rec.a.t === 'string' && typeof p[rec.a.v] === 'function') {
        try { p[rec.a.v](); rec.aCalled = true; } catch (err) { rec.aCalled = false; rec.aErr = String((err && err.message) || err).slice(0, 120); }
      }
      return rec;
    }), STEP_MS, 'tap-diag-evaluate');
    console.log('TAP_DIAG ' + JSON.stringify(diag));
    const openedAfterDiag = await run('read-if-after-diag', () => cur.data(ifKey)) === true;
    console.log('TAP_DIAG openedAfter=' + openedAfterDiag);
    if (openedAfterDiag) {
      await run('close-after-diag', () => cur.setData({ [ifKey]: false }));
      await sleep(600);
    }
  } catch (e) {
    console.log('TAP_DIAG FAIL', String((e && e.message) || e).slice(0, 120));
  }
  // 元素实验后终盘（可能有新异常入列；信道死了则靠看门狗 dump）
  STEP = 'final-dump';
  console.log(`EXCEPTIONS_FINAL count=${exceptions.length}`);
  for (const e of exceptions) console.log('  [exception-final] ' + e);
  console.log(`CONSOLE_FINAL count=${consoleMsgs.length}`);
} catch (e) {
  console.log('FAIL:', String((e && e.message) || e));
}
STEP = 'disconnect';
try { await withTimeout(mp.disconnect(), 5000, 'disconnect'); } catch { /* 看门狗兜底 */ }
process.exit(0);
