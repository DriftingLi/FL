// ============================================================================
// pr-evidence 校验器 —— 判据的**单一来源**（#1770）
//
// **A1 形态（2026-10-29，#2100 裁定）**：证据主键 = PR 正文里的**一行**。
//   行里写 `跑于 <sha7>` + 门自带的机读读数行（`MP_WEIXIN_RESULT` / `KOTLIN_ALL_RESULT` / `COMPILE_RESULT`）。
//   退役的四样（本文件不再有任何代码路径）：sha 绑定门评论、三候选集（PR / commit / 票评论）、
//   marker 与展示副本、①③④b 的字段签核。
//
// 为什么降到这里：整套取证仪式是从「多个陌生 agent 并发推同一仓、必须互相取证」长出来的；
// 单账号串行开发的形态不持有那个前提（#2099 实读：pr-evidence / mobile-gates 都不在 ruleset 必检，
// 这套门本来就是荣誉制）。留着它只是让人照着一页执行单写 A1、却撞上旧校验器判红。
//
// 为什么只剩这两道门：真正挡住 bug 的是「测试绿 + 编译过」（#2099 盘点）。
//   ② 是投放主渠道（微信小程序）的运行时抖动唯一机判；④ 是编译面。① 已在 ADR-0043 降为发版前一次、
//   ①b 已整体退役（#2103）；③ 与 ④b 已判纯税（两腿 / 打包链路都在 CI 与发版流程里，正文再抄一遍零信息）。
//
// 纯函数 + 单一注入点：输入 files / body / headSha / fetchCompare，输出
// { ok, errors, notes, runtime, needs4b, gates }。fetchCompare 是**注入的**取数回调
// （测试注桩、不打网络）：入参 { base, head }，返回 { status, mergeBaseSha, files, filesTruncated }；
// 取不到即返回 null（fail-closed）。因注入了异步回调，本函数返回 Promise，调用方必须 await。
//
// 消费者三处，读的都是这一份源码：
//   ① .github/workflows/pr-evidence.yml —— 不 checkout，用 repos.getContent 取本文件 → new Function 求值
//   ② .github/scripts/preflight-pr.mjs —— 直接 import（本地预检，与 CI 同一份判据）
//   ③ .github/scripts/pr-evidence-check.test.mjs —— 直接 import
// 注意 ① 的求值方式要求**本文件自包含**（不得 import 别的模块），改结构时别拆文件。
// ============================================================================

// 「验收证据行该长什么样」的**唯一真源**（#1920 起；A1 下缩到两门 + 行首形态 +
// 「跑于」锚点）。node 侧当对象字面量读，PowerShell 侧（证据生成器）抽 marker 之间的**同一段字节**
// `ConvertFrom-Json` —— 两个消费者读的字节完全相同。
// 块内规则（PowerShell 的 JSON 解析器不认别的东西）：**不加注释、不加尾逗号、正则只给 source 与 flags**。
// 加一道**走已有触发面**的门 = 往 gates 里加一条；加**新触发面**的门还要同时扩触发判定
// （`isRuntimeFile` / `hitsMpWeixinFace` 是硬编码清单，不是配置）——别以为加条 gates 就够了。
// ===== GATE_EVIDENCE_DESCRIPTOR BEGIN =====
export const GATE_EVIDENCE = {
  "heading": "## 验收证据",
  "listItem": {
    "source": "^\\s*[-*]\\s",
    "prefixStrip": "^\\s*[-*]\\s+",
    "prefix": "- "
  },
  "ranAt": {
    "label": "跑于",
    "source": "跑于\\s*`?([0-9a-fA-F]{7,40})`?"
  },
  "fieldSeparator": {
    "strip": "[·|]\\s*$",
    "display": " · "
  },
  "readingValue": {
    "source": "[A-Za-z_][\\w.-]*\\s*=\\s*\\S+",
    "hint": "token 后面必须跟着门真打出来的键值对（如 `errors=0` / `pages=3`），只写 token 或写 `...` 都是占位"
  },
  "coverage": {
    "field": {
      "source": "(?:^|\\s)coverage=([^\\s\\x60]+)"
    },
    "value": {
      "source": "^(?:changed|smoke):\\d+$|^none$"
    },
    "hint": "`coverage=` 只认 `changed:<N>` / `smoke:<N>` / `none` 三值（门在 `-Pages auto` 下产出，见 ADR-0008 ② 段）"
  },
  "gates": [
    {
      "key": "②",
      "label": "微信开发者工具无报错",
      "requires": "mpweixin",
      "tokens": ["MP_WEIXIN_RESULT"],
      "tokenHint": "门自带的 `MP_WEIXIN_RESULT` 读数行"
    },
    {
      "key": "④",
      "label": "本地编译门",
      "requires": "runtime",
      "tokens": ["KOTLIN_ALL_RESULT", "COMPILE_RESULT"],
      "tokenHint": "门自带的 `KOTLIN_ALL_RESULT`（④c）或 `COMPILE_RESULT`（④a）读数行"
    }
  ]
};
// ===== GATE_EVIDENCE_DESCRIPTOR END =====

const toRe = (spec) => new RegExp(spec.source, spec.flags || '');

// 报错文案里给一行可照抄的形状（判据与提示同源，别在两处各写一份）。
const EVIDENCE_LINE_HINT = '跑于 `<sha7>` · <门读数行原样>（图可选，附在行尾）';

// 「这张改动单算不算碰了交付形态」——运行时面与 ② 触发面共用同一判据，
// 免得「门被点亮的理由」和「门证据的失效条件」两套口径各走一路。
const RUNTIME_JSON = ['manifest.json', 'pages.json', 'platformConfig.json'];
export function isRuntimeFile(p) {
  if (p.endsWith('.uvue') || p.endsWith('.uts')) return true;
  if (!p.startsWith('training-app/')) return false;
  return RUNTIME_JSON.includes(p.split('/').pop());
}

// ④b 触发条件（ADR-0008，2026-09-11 修订）：按「交付形态」触发 —— 打包链路（AAPT2 资源合并、
// AndroidManifest 合并、R8/aar 依赖解析、证书签名）只有真打包才覆盖，本地编译门不覆盖。
// A1 下它**不判红**（正文不再写 ④b 行），只作为发版前提醒出现在 notes 里 —— 判据删了，
// 「这件事存在」的事实不该跟着消失。
function touchesPackaging(files) {
  // 这份清单与上面的 `RUNTIME_JSON` **当前恰好同集**，但概念不同：一个是「算不算运行时面」（触发 ④），
  // 一个是「算不算碰了交付形态」（触发 ④b 的 notes）。别因为看着一样就合并 —— 将来任一侧要分叉
  // （例如打包面单独加某份资源），合并的写法会把两件事绑死。
  const PACKAGE_FILES = ['manifest.json', 'pages.json', 'platformConfig.json'];
  return files.some((f) => {
    const p = f.filename;
    if (p.startsWith('training-app/') && PACKAGE_FILES.includes(p.split('/').pop())) return true;
    if (/uni_modules\/[^/]+\/utssdk\/app-android\//.test(p)) return true;
    if (/\.aar$/.test(p)) return true;
    if (/(^|\/)libs\/[^/]+\.jar$/.test(p)) return true;
    return false;
  });
}

// ② 触发条件（ADR-0008，2026-09-15 修订 #1030）：命中 MP-WEIXIN **面**，不是「文本里出现了这个词」。
//   ① 文件清单：training-app 下的 manifest.json / platformConfig.json；
//   ② 运行时面文件的 diff 里**增删了一条真正的条件编译指令行**（去掉 +/- 后行首、允许注释/引号包装，
//      是 `#ifdef|#ifndef|#if`，且表达式里含 MP-WEIXIN）。表达式**之后**的部分不判 ——
//      `// #ifdef APP-PLUS // 与 MP-WEIXIN 分支不同` 这类「真指令行 + 同行散文提及」不算命中（#1028 实证）。
// 已知边界（写实）：第 ② 条读 `patch`，GitHub 对超大 diff / 二进制不给 `patch` ⇒ 该路径 fail-open。
const MP_WEIXIN_FILES = ['manifest.json', 'platformConfig.json'];
const MP_WEIXIN_DIRECTIVE = /^\s*(?:\/{2,}|\/\*|<!--|["'])?\s*#\s*(?:ifdef|ifndef|if)\s+[A-Za-z0-9_\s|&()!-]*\bMP-WEIXIN\b/;
const hasMpWeixinDirective = (patch) =>
  String(patch || '')
    .split('\n')
    .filter((line) => /^[+-]/.test(line) && !/^(?:\+\+\+|---)/.test(line))
    .map((line) => line.slice(1))
    .some((content) => MP_WEIXIN_DIRECTIVE.test(content));
const hitsMpWeixinFace = (f) => {
  const p = f.filename;
  if (p.startsWith('training-app/') && MP_WEIXIN_FILES.includes(p.split('/').pop())) return true;
  return isRuntimeFile(p) && hasMpWeixinDirective(f.patch);
};

// ============================================================================
// ADR-0052（移动端）② 门三档分层的**影子分档器** —— 波 2：只记录，不判红、不免除。
//
// 输入 = 改动文件清单（带 patch/status）+ 注入的核心名单（.github/scripts/lib/
// mobile-gate-core-list.json；本文件必须自包含，名单只能由调用方取好传进来）+ PR 正文。
// 输出 = tiering 对象（附在返回值上）与若干条 `SHADOW-0052` note。**errors 永不经它**。
//
// 阶梯（ADR-0052 决策 3/4，兜底永远向上；判定不动 → 按最高档记）：
//   full         全门：核心页前缀 / 公共目录 / App.uvue·main.uts / 三份 json / 非 modified 状态
//                （新增·删除·改名）/ `.uts`（逻辑减配档未开）/ scope 根外的运行时文件 /
//                patch 缺失或内容识别不出 / script 块、import、组件标签、数据与事件绑定
//   lite-targeted2  非核心页 `.uvue`，diff 触到 DOM 结构、布局属性、安全区、导航栏、静态类名
//   lite-exempt2    非核心页 `.uvue`，只有文本行与非布局样式值
// PR 档 = 各文件最坏值（混档自动升全门由 max 聚合天然成立）。
//
// **代码级锁定**：名单 switches 在本波只读不生效。exemptionActive/logicLiteTierOpen
// 被翻真时不改判，只记 anomaly（波 3b 才接线）——「Shadow 期间不可能被提前打开」。
// ============================================================================

const LITE_RANK = { 'lite-exempt2': 1, 'lite-targeted2': 2, 'full': 3 };
const BUILTIN_TAGS = new Set(['view','text','image','button','input','textarea','swiper','swiper-item','scroll-view','cover-view','cover-image','icon','rich-text','progress','checkbox','checkbox-group','radio','radio-group','form','label','picker','picker-view','slider','switch','navigator','video','camera','map','canvas','ad','official-account','open-data','template','slot','block','include','list','list-item','cell','cell-group','refresh-indicator','web-view','movable-area','movable-view','page-meta','view-group','layout','span','p','div','ul','li']);
const LAYOUT_CSS = /(?:^|[;{\s])(?:display|flex(?:-[\w-]+)?|grid(?:-[\w-]+)?|justify-content|align-(?:items|content|self)|position|top|left|right|bottom|width|height|min-[\w-]+|max-[\w-]+|margin(?:-[\w-]+)?|padding(?:-[\w-]+)?|gap|line-height|transform|z-index)\b/;
const EXEMPT_CSS = /(?:^|[;{\s])(?:color|background(?:-color)?|font(?:-weight|-family)?|font-style|letter-spacing|text-align|text-decoration|text-shadow|opacity|visibility|border(?:-radius|-style|-color)?|box-shadow)\b\s*:/;
const VISUAL_WORD = /safe-area|safeArea|statusBar|statusbar|navigationBarTitleText|navigationStyle/i;

// 名单形状校验（不是业务校验）：字段缺失/类型不对 ⇒ null ⇒ 调用方按「名单不可用」fail-closed。
export function parseCoreList(raw) {
  let obj = raw;
  if (typeof raw === 'string') { try { obj = JSON.parse(raw); } catch { return null; } }
  if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return null;
  if (obj.version !== 1 || typeof obj.scopeRoot !== 'string') return null;
  for (const k of ['corePagePrefixes', 'alwaysFullGateDirPrefixes', 'alwaysFullGateFilePrefixes']) {
    if (!Array.isArray(obj[k]) || obj[k].some((p) => typeof p !== 'string' || !p)) return null;
  }
  if (!obj.switches || typeof obj.switches.exemptionActive !== 'boolean' || typeof obj.switches.logicLiteTierOpen !== 'boolean') return null;
  return obj;
}

const inPrefixSegments = (rel, prefixes) =>
  prefixes.some((p) => rel === p || rel.startsWith(p.replace(/\/+$/, '') + '/'));

// diff 内容阶梯：只看增删行（去 +/- 前缀；排除 +++/--- 文件头）。返回该文件档位。
function classifyUvuePatch(patch, list) {
  const content = String(patch || '').split('\n')
    .filter((l) => /^[+-]/.test(l) && !/^(?:\+\+\+|---)/.test(l))
    .map((l) => l.slice(1));
  if (content.length === 0) return { tier: 'full', why: 'patch 缺失或无增删行 ⇒ 识别不出，fail-closed' };
  let rank = 0; let why = '';
  const bump = (r, w) => { if (r > rank) { rank = r; why = w; } };
  const coreDirRe = new RegExp(`(?:pages)/(?:${list.corePagePrefixes.map((p) => p.replace(/^pages\//, '')).join('|')})/`);
  for (const c of content) {
    const t = c.trim();
    if (!t) continue;
    // —— 全门触发（3）——
    if (/<\/?script/i.test(t) || /^\s*(?:\/\/)?\s*(?:const|let|var|function|import|export|class|async|await)\b/.test(t)) { bump(3, 'script/顶层代码行增删'); continue; }
    if (/^\s*import\b/.test(t) || coreDirRe.test(t)) { bump(3, '依赖引用变化（import/核心路径）'); continue; }
    if (/(?:^|\s)[:@][A-Za-z_][\w.-]*\s*=|v-(?:if|else|for|model|show|bind|on|html|slot)\b|\{\{/.test(t)) { bump(3, '数据/事件绑定或插值'); continue; }
    const tagM = t.match(/<\/?([A-Za-z][\w-]*)/);
    if (tagM && !BUILTIN_TAGS.has(tagM[1].toLowerCase())) { bump(3, '自定义组件标签增删'); continue; }
    // —— ② 定向触发（2）——
    if (tagM) { bump(2, '内置标签/DOM 结构增删'); continue; }
    if (VISUAL_WORD.test(t) || LAYOUT_CSS.test(t)) { bump(2, '布局/安全区/导航栏'); continue; }
    if (/\bclass\s*=/.test(t)) { bump(2, '静态类名改动（布局影响未证）'); continue; }
    // —— 可免 ②（1）——
    if (EXEMPT_CSS.test(t)) { bump(1, '非布局样式值'); continue; }
    if (!/[<>{}]|=\s*["']/.test(t)) { bump(1, '纯文本内容行'); continue; }
    // 落不进任何一级：识别不出 ⇒ 保守（不 bump，保持当前 rank；rank 仍 0 时下面收口成 full）
  }
  if (rank === 0) return { tier: 'full', why: '增删行落不进阶梯任何一级 ⇒ 识别不出，fail-closed' };
  return { tier: rank === 3 ? 'full' : rank === 2 ? 'lite-targeted2' : 'lite-exempt2', why };
}

// 正文档位声明：`- 档位：全门|模板减配|逻辑减配`（段内任意位置，半全角冒号都认）。
export function parseDeclaredTier(bodyText) {
  const m = String(bodyText || '').match(/^\s*[-*]?\s*档位\s*[:：]\s*(全门|模板减配|逻辑减配)/m);
  return m ? m[1] : null;
}

export function classifyTiering({ files, coreList, bodyText }) {
  const runtimeFiles = (files || []).filter((f) => isRuntimeFile(f.filename));
  const declared = parseDeclaredTier(bodyText);
  const out = { state: 'shadow', tier: 'n/a', declared, verified: 'n/a', triggers: [], anomalies: [], planned: { state: 'absent' } };
  if (runtimeFiles.length === 0) { out.verified = '不适用（未命中运行时面）'; return out; }
  const list = parseCoreList(coreList);
  if (!list) {
    out.tier = 'full'; out.verified = '名单不可用';
    out.anomalies.push('名单缺失/形状不合 ⇒ fail-closed 按全门记录');
    out.planned = judgePlannedCert(bodyText, 'full');
    return out;
  }
  if (list.switches.exemptionActive) out.anomalies.push('名单 exemptionActive=true，但波 2 校验器未接线 ⇒ 忽略开关、继续影子（转正属波 3b）');
  if (list.switches.logicLiteTierOpen) out.anomalies.push('名单 logicLiteTierOpen=true，同上 ⇒ 逻辑改动仍按全门记录');
  const scope = list.scopeRoot.replace(/\/+$/, '') + '/';
  let rank = 0;
  for (const f of runtimeFiles) {
    const fn = String(f.filename || '');
    const rel = fn.startsWith(scope) ? fn.slice(scope.length) : null;
    const put = (r, why) => {
      if (r > rank) { rank = r; out.tier = r === 3 ? 'full' : r === 2 ? 'lite-targeted2' : 'lite-exempt2'; }
      if (out.triggers.length < 8) out.triggers.push(`${rel || fn}：${why}`);
    };
    if (rel === null) { put(3, 'scope 根外的运行时文件 ⇒ 不识别'); continue; }
    const base = rel.split('/').pop();
    if (['manifest.json', 'pages.json', 'platformConfig.json'].includes(base)) { put(3, '三份 json（性质判据）'); continue; }
    if (f.status && f.status !== 'modified') { put(3, `状态 ${f.status}（新增/删除/改名一律保守）`); continue; }
    if (list.alwaysFullGateFilePrefixes.includes(rel)) { put(3, '全局升档文件（App.uvue/main.uts）'); continue; }
    if (inPrefixSegments(rel, list.alwaysFullGateDirPrefixes)) { put(3, '公共目录改动（核心依赖传播面）'); continue; }
    if (inPrefixSegments(rel, list.corePagePrefixes)) { put(3, '核心页模块'); continue; }
    if (!rel.startsWith('pages/')) { put(3, '不在名单覆盖面（页面外路径）⇒ fail-closed'); continue; }
    if (rel.endsWith('.uts')) { put(3, '页面局部 .uts 逻辑（逻辑减配档未开）'); continue; }
    if (rel.endsWith('.uvue')) { const r = classifyUvuePatch(f.patch, list); put(LITE_RANK[r.tier] || 3, r.why); continue; }
    put(3, '识别不出的文件形态 ⇒ fail-closed');
  }
  out.tier = rank === 3 ? 'full' : rank === 2 ? 'lite-targeted2' : 'lite-exempt2';
  // 核身（声明 vs 事实）：本波只记不符，不判红。声明高于事实=保守方向，接受。
  if (!declared) { out.verified = '未声明（影子按记录全门对待）'; }
  else if (declared === '逻辑减配') { out.verified = '不符：逻辑减配档未开'; }
  else if (declared === '全门') { out.verified = out.tier === 'full' ? '符' : '保守方向声明（事实更轻，接受）'; }
  else if (declared === '模板减配') {
    out.verified = out.tier === 'lite-exempt2' ? '符'
      : out.tier === 'lite-targeted2' ? '不符：实际需 ② 定向'
      : '不符：事实为全门';
  }
  out.planned = judgePlannedCert(bodyText, out.tier);
  return out;
}

// planned=0 双证链（ADR-0052 决策 6）：正文出现 changed 读数行才判；影子期只记状态。
// 合法 planned=0 ⇔ 同行 `discovered=N`（N>0，证明发现过程健康）且档位为模板级
//（页面模板改动天然无套件映射）。runner 侧 `discovered=` 的产出接线属波 3a 前置。
function judgePlannedCert(bodyText, tier) {
  const lines = String(bodyText || '').match(/TEST_(?:UNIT|BEHAVIOR)_CHANGED_RESULT[^\n]*/g) || [];
  if (lines.length === 0) return { state: 'absent', why: '正文无 changed 读数行（A1 下 ③ 不入正文，常态即此）' };
  const issues = [];
  let zeroProved = 0;
  for (const line of lines) {
    const pm = line.match(/\bplanned=(\d+)/);
    if (!pm) { issues.push('planned= 缺失'); continue; }
    if (pm[1] !== '0') continue;
    const dm = line.match(/\bdiscovered=(\d+)/);
    if (!dm) { issues.push('planned=0 但同行无 discovered ⇒ 双证缺一（发现过程未证明）'); continue; }
    if (dm[1] === '0') { issues.push('discovered=0 ⇒ 测试配置失效/发现异常'); continue; }
    // 全门档的 planned=0 不靠它免任何东西（③ 两腿在 CI 全量跑），模板档才是双证链要保护的档位。
    if (tier !== 'full') zeroProved += 1;
  }
  if (issues.length > 0) return { state: 'anomaly', why: issues.join('；') };
  return { state: 'ok', why: `planned=0 均有 discovered>0 同行双证（模板档命中 ${zeroProved} 行）` };
}

//  sha 前缀比较：A1 正文只写 7 位，compare 回来的是 40 位 —— 严格 `!==` 会让祖先口径**永假**。
const shaMatches = (a, b) => Boolean(a) && Boolean(b) && (a.startsWith(b) || b.startsWith(a));

export async function validatePrEvidence({
  files, body, headSha = '', fetchCompare = null, coreList = null,
}) {
  // 错误带上门归属：例外通道要按门豁免，字符串数组做不到「只清 ②、④ 照红」。
  // 返回给调用方前统一渲染成字符串（三个消费者的接口不变）。
  const errors = [];
  const err = (gate, text) => errors.push({ gate, text });
  const notes = [];

  const runtime = files.filter((f) => isRuntimeFile(f.filename)).map((f) => f.filename);
  const needs2Why = files.filter(hitsMpWeixinFace).map((f) => f.filename);
  const needs2 = needs2Why.length > 0;
  const needs4b = touchesPackaging(files);
  const headLc = String(headSha || '').toLowerCase();

  const gates = {
    '②': { required: needs2, bound: false, commit: '', reason: '', why: needs2 ? needs2Why.slice(0, 5) : [] },
    '④': { required: runtime.length > 0, bound: false, commit: '', reason: '', why: runtime.slice(0, 5) },
  };

  // 未命中运行时面 ⇒ 整段免（分级判据，ADR-0008）。空正文也放行，这一条与改动前同义。
  if (runtime.length === 0) {
    notes.push(`未命中运行时面（改动 ${files.length} 个文件），按 ADR-0008 免人工门与 ④ 编译门。`);
    return {
      ok: true, errors, notes, runtime, needs4b, gates,
      tiering: { state: 'shadow', tier: 'n/a', declared: null, verified: '不适用（未命中运行时面）', triggers: [], anomalies: [], planned: { state: 'absent', why: '未命中运行时面' } },
    };
  }

  const clean = String(body || '').replace(/<!--[\s\S]*?-->/g, '');

  // ---- ADR-0052 影子分档（波 2）：算档、核身、planned 双证，全部只进 notes/tiering，
  // 不进 errors —— 本函数对 ok 的影响恒为「无」。----
  const tiering = classifyTiering({ files, coreList, bodyText: clean });
  notes.push(`SHADOW-0052 分档：档=${tiering.tier} 声明=${tiering.declared || '未声明'} 核身=${tiering.verified}（影子记录，不参与判据；转正属波 3b）`);
  if (tiering.triggers.length > 0) {
    notes.push(`SHADOW-0052 判据出处：${tiering.triggers.slice(0, 4).join(' / ')}${tiering.triggers.length > 4 ? ' 等' : ''}。`);
  }
  for (const a of tiering.anomalies) notes.push(`SHADOW-0052 异常：${a}`);
  if (tiering.planned.state !== 'absent') notes.push(`SHADOW-0052 planned 双证：${tiering.planned.state} —— ${tiering.planned.why}`);
  // 段名从描述符取：写死正则会让「改描述符」只动生成侧、校验侧静默不跟（第二套口径）。
  const escRe = (s) => String(s).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const head = clean.match(new RegExp(`^${escRe(GATE_EVIDENCE.heading)}\\s*$`, 'm'));
  if (!head) {
    err('*', `缺 \`${GATE_EVIDENCE.heading}\` 段（段名固定，见 .github/PULL_REQUEST_TEMPLATE.md）。`);
    return { ok: false, errors: errors.map((e) => e.text), notes, runtime, needs4b, gates, tiering };
  }
  const rest = clean.slice(head.index + head[0].length);
  const next = rest.search(/^##\s/m);
  const section = next === -1 ? rest : rest.slice(0, next);

  // 例外通道：显式风险接受，**必须点名哪道门**（#2100 决策 4；2026-10-29 review 收窄）。
  //
  // 旧实现是 `/已接受未验证风险/.test(clean)` —— 全文任意位置命中即清空全部 errors，
  // 于是「本 PR 不适用『已接受未验证风险』通道」这种**否认句**也会命中，五个字空正文也能过。
  // 收窄的三条（都不靠加填字，全靠现有结构）：
  //   ① 只在 `## 验收证据` 段内生效 ⇒ 背景叙述 / 引用的旧例子不会误触发；
  //   ② 声明必须点名门号（`- 例外：已接受未验证风险 ②`）⇒ 写得含糊就清不掉任何东西，自动退回红；
  //   ③ 只清空**被点名门**的 errors ⇒ ④ 有缺项时写 ② 的豁免救不了它。
  const WAIVE_HINT = '- 例外：已接受未验证风险 <门号>';
  const GATE_KEY_RE = new RegExp(`${GATE_EVIDENCE.gates.map((g) => escRe(g.key)).join('|')}(?![a-z])`, 'g');
  const waived = new Set();
  for (const line of section.split('\n')) {
    if (!/已接受未验证风险/.test(line)) continue;
    for (const k of line.match(GATE_KEY_RE) || []) waived.add(k);
  }

  const LIST_ITEM = toRe(GATE_EVIDENCE.listItem);
  const LIST_ITEM_PREFIX = new RegExp(GATE_EVIDENCE.listItem.prefixStrip);
  const RAN_AT = toRe(GATE_EVIDENCE.ranAt);
  const READING_VALUE = toRe(GATE_EVIDENCE.readingValue);
  const COVERAGE_FIELD = toRe(GATE_EVIDENCE.coverage.field);
  const COVERAGE_VALUE = toRe(GATE_EVIDENCE.coverage.value);

  // ④ 全家（④ / ④a / ④b / ④c）都含「④」⇒ 按行首编号匹配，④a / ④b 行不能顶替 ④ 行。
  const matchesGate = (line, gate) => {
    const core = line.replace(LIST_ITEM_PREFIX, '');
    if (!core.startsWith(gate.key)) return false;
    if (gate.key === '④') return !/[a-z]/.test(core.charAt(1));
    return true;
  };

  // 绑定判定（ADR-0008，2026-09-12 放宽）：sha7 等于 head，或它是 head 的祖先**且**其间运行时面未变。
  // **fail-closed**：compare 取不到（未知 sha / 异常 / 截断）一律判不满足，不得因 API 抖动放行。
  const compareCache = new Map();
  const compareOnce = async (base) => {
    if (typeof fetchCompare !== 'function') return null;
    if (compareCache.has(base)) return compareCache.get(base);
    let res = null;
    try {
      res = await fetchCompare({ base, head: headLc });
    } catch {
      res = null;
    }
    compareCache.set(base, res || null);
    return res || null;
  };
  const judgeBinding = async (sha) => {
    if (!headLc) return { ok: false, reason: '本次事件没给出 head sha（fail-closed）' };
    if (shaMatches(sha, headLc)) return { ok: true, reason: `等于 head` };
    const cmp = await compareOnce(sha);
    if (!cmp) {
      return { ok: false, reason: `取不到 compare(\`${sha}\`…head) 结果（未知 sha / API 异常 / 超时）⇒ fail-closed 不满足，重跑一次并写当前 head 的前 7 位` };
    }
    const status = String(cmp.status || '');
    const mergeBase = String(cmp.mergeBaseSha || '').toLowerCase();
    if ((status !== 'ahead' && status !== 'identical') || !shaMatches(sha, mergeBase)) {
      return { ok: false, reason: `它不是当前 head 的祖先（compare status=${status || '未知'}）` };
    }
    if (cmp.filesTruncated) {
      return { ok: false, reason: '与 head 之间的 diff 过大被 GitHub 截断（files ≥300）⇒ 判不出运行时面是否变过，fail-closed 不满足' };
    }
    const changed = Array.isArray(cmp.files) ? cmp.files.filter((f) => isRuntimeFile(String(f || ''))) : [];
    if (changed.length > 0) {
      return { ok: false, reason: `之后运行时面文件已变更（${changed.slice(0, 3).join('、')}${changed.length > 3 ? ' 等' : ''}）⇒ 需重跑并把 \`跑于\` 改成当前 head 的前 7 位` };
    }
    return { ok: true, reason: `是 head 的祖先且其间运行时面（\`.uvue\` / \`.uts\` / 三份 json）未变，当前 head=${headLc.slice(0, 7)}` };
  };

  const lines = section.split('\n').filter((l) => LIST_ITEM.test(l));
  for (const gate of GATE_EVIDENCE.gates) {
    const state = gates[gate.key];
    if (!state.required) {
      notes.push(`${gate.key} ${gate.label}：未触发（${gate.requires === 'mpweixin' ? '未命中 MP-WEIXIN 面，ADR-0008' : '未命中运行时面'}）⇒ 免。`);
      continue;
    }
    const where = `${gate.key} ${gate.label}`;
    const line = lines.find((l) => matchesGate(l, gate));
    if (!line) {
      err(gate.key, `${where}：缺该行。按模板写一行 \`- ${gate.key} ${EVIDENCE_LINE_HINT}\`。`);
      continue;
    }
    const token = gate.tokens.find((t) => line.includes(t));
    if (!token) {
      err(gate.key, `${where}：该行没有${gate.tokenHint} —— 门自带的机读行原样贴进来即可，不要改写它。`);
    } else {
      // token 只是「门名」，不能单独算读数。模板曾预置 `MP_WEIXIN_RESULT ...`，只判 includes
      // 就等于「照抄模板即绿」（2026-10-29 review 的 P0）。token 之后必须跟着门真打出来的键值对。
      const after = line.slice(line.indexOf(token) + token.length);
      if (!READING_VALUE.test(after)) {
        err(gate.key, `${where}：${gate.tokenHint} 后面没有实际读数 —— ${GATE_EVIDENCE.readingValue.hint}。`);
      }
    }
    // `coverage=` 形态校验（#2101/#2102）。**只校「出现即合法」，不校「必须出现」**：
    // #2102 原文写「校存在性」，但门只在 `-Pages auto` 下产出该字段 —— `-Pages <csv>`（定向单验）与
    // `-Pages none`（旧导航组）是合法跑法且没有「改动页覆盖」语义，强制出现会把它们判成假红。
    // 这与本票「删格式税」的立场一致：能判红的只剩「写了但写错」，不是「少写了一个自报字段」。
    if (gate.key === '②') {
      const cov = line.match(COVERAGE_FIELD);
      if (cov) {
        // 读数通常被反引号包着（`… coverage=changed:2`），截到反引号前；再兜一刀剥尾反引号
        const v = cov[1].replace(/`+$/, '');
        if (!COVERAGE_VALUE.test(v)) {
          err(gate.key, `${where}：\`coverage=${v}\` 不是合法形态 —— ${GATE_EVIDENCE.coverage.hint}。`);
        }
      }
    }
    const m = line.match(RAN_AT);
    if (!m) {
      err(gate.key, `${where}：缺 \`跑于 <sha7>\`（门是在哪个 commit 上跑的，写它的前 7 位）。`);
      continue;
    }
    const sha = m[1].toLowerCase();
    const v = await judgeBinding(sha);
    if (v.ok) {
      state.bound = true;
      state.commit = sha.slice(0, 7);
      state.reason = v.reason;
      // 祖先绑定要明示（不静默放宽）：证据是在**旧 commit** 上产出的，
      // 写成「已绑」而不说是哪个 sha，读起来像「证据是在当前 head 上跑的」——这在只校结构的门里属往更可信方向失真。
      if (v.reason !== '等于 head') notes.push(`${where}：绑定 ${sha.slice(0, 7)} —— ${v.reason}。`);
    } else {
      state.reason = v.reason;
      err(gate.key, `${where}：\`跑于 ${sha.slice(0, 7)}\` 绑不上当前 head —— ${v.reason}。`);
    }
  }

  if (needs2) {
    notes.push(`② 触发面命中（MP-WEIXIN 面，ADR-0043 起为常态主门、无低风险减免）：${needs2Why.slice(0, 5).join('、')}${needs2Why.length > 5 ? ' 等' : ''}。`);
  }
  if (needs4b) {
    notes.push('④b 打包面命中（三份 json / utssdk 原生面 / *.aar / libs/*.jar）：A1 下不判红、正文不写 ④b 行，但**发版前必须跑** release 云打包（ADR-0008）。');
  }

  // 例外通道只在「点名门覆盖了**全部**缺项」时放行；否则照红，并说明差在哪。
  if (errors.length > 0 && waived.size > 0) {
    const absorbed = errors.filter((e) => waived.has(e.gate));
    const blocking = errors.filter((e) => !waived.has(e.gate));
    // 点名了但该门其实没缺项 ⇒ 记 note（防模板残留，也防「我写了豁免 = 全都免了」的误读）
    const idle = [...waived].filter((k) => !errors.some((e) => e.gate === k));
    if (blocking.length === 0) {
      return {
        ok: true,
        errors: [],
        notes: [
          `⚠️ 走「已接受未验证风险」例外通道，豁免门：${[...waived].join(' / ')}。`,
          ...absorbed.map((e) => `⚠️ 已豁免：${e.text}`),
          ...idle.map((k) => `⚠️ 例外声明点了 ${k}，但那道门本次没有缺项（多写了一句，不影响结论）。`),
          '⚠️ 例外通道下必须由人执行合并（AGENTS.md：agent 不得自行合并触及运行时面的 PR）。',
          ...notes,
        ],
        runtime,
        needs4b,
        gates,
        tiering,
      };
    }
    notes.push(`例外声明只覆盖了 ${[...waived].join(' / ')}，仍有未被覆盖的缺项（见下）—— 要豁免就点名对应门号（\`${WAIVE_HINT}\`）。`);
  }
  if (errors.length === 0) {
    if (waived.size > 0) notes.push(`例外声明写了 ${[...waived].join(' / ')}，但本次没有缺项 —— 可以删掉那句话。`);
    else notes.push('证据结构完整（pr-evidence 只校结构与 sha 绑定，真伪由维护者判定）。');
  }
  return {
    ok: errors.length === 0,
    errors: errors.map((e) => e.text),
    notes,
    runtime,
    needs4b,
    gates,
    tiering,
  };
}
