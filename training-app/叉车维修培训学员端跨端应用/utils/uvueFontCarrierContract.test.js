/**
 * uvue「文字类样式」承载面契约（#1269）—— 全仓 `.uvue` 源码守护。
 *
 * 为什么需要它：uvue 原生端的渲染器**只把文字类样式认给文字类元素**，写在 `<view>` 上会被判错并
 * **静默忽略**。真机日志原文（#650 / T12 的 ①a 取证，术前术后两轮逐字同形，见
 * `docs/verification/forgot-password/1263/machine-lines.txt` 第 3 节）：
 *
 *   style property `font-size|color` is only supported on `<text>|<button>|<input>|<textarea>`.
 *     there is an error on `<view class="mode-tab mode-tab-active">`.
 *   style property `font-weight` is only supported on `<text>|<button>|<input>|<textarea>|<loading>`.
 *     there is an error on `<view class="mode-tab mode-tab-active">`.
 *
 * 两条后果，与 #1113 的 `white-space` 完全同类：
 *   · **死声明** —— 设计稿写的 `28rpx / #666666`、选中态 `#2979ff + bold` 在 Android 端从未生效过；
 *   · **每进一次页打一条 error** —— 污染 ①a 的「应用控制台错误行」判据（#1040 / #1070 / #1110 都拿它当证据面）。
 *
 * 先例：`components/ai-chat/ai-chat-drawer-left.uvue` 的 `.empty-tip` 早就踩过同一条（真机报
 * `text-align|font-size|color is only supported on ...`，记在 `utils/aiChatDrawerContract.test.js` 第 ② 条），
 * 但那位只锁了**一个 class** ⇒ 全仓每加一页就多一处静默忽略。本守护把它做成全仓规则。
 *
 * 判据：**一条 `<style>` 规则里的文字类样式，其选择器最后一段 compound 的每个 class，在模板里的
 * 所有承载标签都必须在白名单内。** 三点展开：
 *   ① 必须跨「class → 承载标签」映射（照 `utils/uvueWhiteSpaceContract.test.js` 的坑位 1）——
 *      只看 class 名判不出：本仓 `.paragraph` 同时挂 `<text>`（合法）与 `<view>`（该位的声明是死的）。
 *   ② **同一 class 既挂合法标签又挂非法标签也算违规** —— 非法那一位上的声明同样是死的。
 *   ③ **合规形态唯一：把声明挪到合法承载上**。与 `white-space` 那条的差别是这里**没有「合法位」登记表**
 *      —— 本仓 `<text>` 上的文字样式有 **1100+ 处**且全部合法，逐位登记没有意义。
 *      但**有**一张「已知存量」清单（`DEFERRED`）：票面写了「本票不顺手扩大范围」，全仓现扫出的 9 条规则里
 *      有 4 条属 ai-chat 面、1 条属招募面 ⇒ 那 5 位登记为存量，**只减不增**（详见 `DEFERRED` 的注释）。
 *
 * 形态沿用本仓既有全仓守护（先例 `utils/uvueWhiteSpaceContract.test.js` / `utils/gradientSyntaxContract.test.js`）：
 * 先对**注入的违规样本**断言检测有效（防空跑假绿），再对真实文件断言零命中。
 * helper 与各先例**自带重复、不抽公共模块**（本仓既有守护一律如此，抽公共件是另开一次跨守护重构）。
 *
 * 只锁**有真机判据**的四个属性：`font-size` / `color` / `font-weight`（上面两条日志原文点名的）
 * 与 `text-align`（`aiChatDrawerContract` 记录的同族日志原文 `text-align|font-size|color`）。
 * `line-height` / `font-family` **不锁** —— 本仓没有它们被判错的日志，写了就是替渲染器编规则。
 * （实测口径：把这两个属性加进判定，2026-09-24 现测的违规清单**一条都不增加**，故将来若真机给出
 * 日志，扩进 `PROPERTY_RULES` 是零成本的。）
 *
 * 已知边界（写实，不宣称覆盖）：
 *   · 只判 `<style>` 块里的规则，且只判选择器**最后一个 compound** —— 属性落在它指的元素上；
 *     后代 / 子选择器的**祖先类**不参与判定（`.box .label` 的载体是 `.label`）。逗号分组选择器**逐个分支**判。
 *   · 最后 compound 里多个 class 取**交集**（`.a.b` 要求同一元素同时带 a 与 b）。交集为空 ⇒ 该规则
 *     匹配不到任何元素，属「死选择器」，不在本守护射程。
 *   · class 在模板里**找不到承载**（全局样式、由 `.uts` 常量动态挂上的 class）⇒ **不判**。
 *     现测的量级：全仓 1577 个 occurrence 里 **71 个**属此类（2026-09-24）⇒ 判它们会 mass 假红；
 *     代价是这类位上的违规本守护看不见，故 ③ 节另锁「已判定数 ≥ `JUDGED_MIN`」防这条边界悄悄扩大。
 *   · **无模板的文件**（`App.uvue`：应用根组件，只有全局 `<style>`）⇒ 它的规则一条也不判。这条盲区具名登记在
 *     `NO_TEMPLATE_OK`，③ 节按**相等**断言它 ⇒ 想新增第二处静默盲区必须先改那张清单。理由见该常量注释。
 *   · 模板内联 `style="…"` 只判承载标签是否合法（与规则面同一白名单）。
 *   · 选择器里没有 class 的写法（tag 选择器）不判 —— 那是「uvue 只支持 class 选择器」的另一条规则，
 *     修法与本票不同（本票的修法是「把文字挪进 `<text>`」，那位是「换成 class 选择器」）。
 *   · SCSS 的 `//` 行注释不剥、`&` 嵌套父选择器不展开（2026-09-24 现测全仓 `.uvue` 的 `<style>` 里
 *     含 `&` 的选择器 **0 处**，与 `uvueWhiteSpaceContract` 同一口径）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');

/** `<text>` 系元素 —— uvue 文字类样式的共同合法载体（真机日志白名单） */
const TEXT_CARRIERS = ['text', 'button', 'input', 'textarea'];

/**
 * 属性 → 合法承载。**分组依据是日志原文里 `style property` 后面的那串属性名**，不是语义近似：
 * `font-size|color` 与 `text-align|font-size|color` 同一组，`font-weight` 单独一组且多一个 `<loading>`。
 */
const PROPERTY_RULES = [
  { props: ['font-size', 'color', 'text-align'], carriers: TEXT_CARRIERS },
  { props: ['font-weight'], carriers: TEXT_CARRIERS.concat(['loading']) },
];

const PROPERTY_LABEL = (props) => props.map((p) => '`' + p + '`').join(' / ');
const CARRIER_LABEL = (carriers) => carriers.map((c) => '<' + c + '>').join('|');

/* ------------------------------------------------------------------ 取段 / 剥注释 */

/**
 * 取 `<style>` 块的**内部**文本，且**行号天然对齐到源文件**：块外的一切（模板、script、`<style>` 标签本身）
 * 只保留换行、其余字符换成空格 —— 换行数不变 ⇒ 第 N 行仍是源文件的第 N 行，违规文案的 `file:line` 能直接跳。
 *
 * 与 `uvueWhiteSpaceContract` 的差别是这里**剥掉了 `<style …>` 标签**：那条守护只看声明文本，
 * 标签混进缓冲区无害；本守护要读**选择器**，留着它会让块内**第一条**规则的 selector 变成
 * `<style lang="scss">\n.mode-tab`（行号也被顶到标签那一行）。空格不会被 `buf.trim()` 当内容，故无害。
 */
function styleTextOf(source) {
  const blank = (s) => s.replace(/[^\n]/g, ' ');
  let out = '';
  let last = 0;
  for (const m of source.matchAll(/<style[^>]*>[\s\S]*?<\/style>/g)) {
    const open = m[0].indexOf('>') + 1;
    const close = m[0].lastIndexOf('</style>');
    out += blank(source.slice(last, m.index)) + blank(m[0].slice(0, open)) + m[0].slice(open, close);
    last = m.index + m[0].length;
  }
  return out + blank(source.slice(last));
}

/** 取 `<template>` 块文本（承载标签只能从这里读）；同样对齐行号 */
function templateOf(source) {
  const start = source.indexOf('<template>');
  const end = source.lastIndexOf('</template>');
  if (start === -1 || end === -1 || end < start) return '';
  return '\n'.repeat(lineAt(source, start) - 1) + source.slice(start, end);
}

/** 剥 CSS 注释 —— 解释坑位的注释里本身写着 `font-size: 28rpx`，不剥就会把说明判红（#1113 票面坑位 2 同源） */
function stripCssComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, ' ');
}

/** 剥 HTML 注释 —— 模板注释同理 */
function stripHtmlComments(text) {
  return text.replace(/<!--[\s\S]*?-->/g, ' ');
}

/** 某个下标所在的行号（1 起） */
function lineAt(text, index) {
  return text.slice(0, index).split('\n').length;
}

/* ------------------------------------------------------- class → 承载标签映射 */

/**
 * 模板里所有开标签：[{ tag, attrs, index }]。
 * 属性区允许被引号包住的 `>`，否则 `<view v-if="a > b">` 会被截断。
 */
function openingTags(tpl) {
  const re = /<([A-Za-z][\w-]*)((?:"[^"]*"|'[^']*'|[^>"'])*)>/g;
  const out = [];
  for (const m of tpl.matchAll(re)) out.push({ tag: m[1].toLowerCase(), attrs: m[2], index: m.index });
  return out;
}

/**
 * 纯函数：模板源码 → `Map<class, Set<承载标签>>`。三类来源都收：
 *   · 静态 `class="a b"`；
 *   · `:class="{ 'a': cond }"` 的**对象键**（键 = class 名，含未加引号的 `{ active: cond }`）；
 *   · `:class="cond ? 'a' : 'b'"` 的分支字面量。
 * 比较值（`mode == 'phone'` 里的 `phone`）**不算** class —— 那是值，收了会造出「幻影承载」。
 */
function classCarriers(tpl) {
  const map = new Map();
  const add = (cls, tag) => {
    if (!cls || /\s/.test(cls)) return;
    if (!map.has(cls)) map.set(cls, new Set());
    map.get(cls).add(tag);
  };

  for (const { tag, attrs } of openingTags(tpl)) {
    for (const m of attrs.matchAll(/(?:^|\s)class\s*=\s*"([^"]*)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => add(c, tag));
    }
    for (const m of attrs.matchAll(/(?:^|\s)class\s*=\s*'([^']*)'/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => add(c, tag));
    }
    for (const m of attrs.matchAll(/:class\s*=\s*"([^"]*)"/g)) {
      const expr = m[1];
      for (const q of expr.matchAll(/'([^'\n]*)'/g)) {
        const raw = q[1];
        if (!raw) continue;
        const after = expr.slice(q.index + q[0].length);
        const before = expr.slice(0, q.index).trimEnd();
        if (after.trimStart().startsWith(':')) { add(raw, tag); continue; } // 对象键
        if (/(==|!=|===|!==)\s*$/.test(before)) continue;                    // 比较值
        add(raw, tag);
      }
      for (const k of expr.matchAll(/(?:^|[{,\s])([A-Za-z_][\w-]*)\s*:/g)) add(k[1], tag);
    }
  }
  return map;
}

/* ------------------------------------------------------------ CSS 规则解析 */

/**
 * 纯函数：CSS 文本 → 规则清单 `[{ selector, line, decls: [{ prop, line }] }]`。
 * 大括号栈取每块自己的前导选择器（`@media` 包层不算规则，SCSS 嵌套的内层块按**自己的** prelude 成一条）。
 *
 * **逐条声明取属性名**（不是拿正则去扫整块文本）—— 这是本守护相对 `uvueWhiteSpaceContract` 多出来的一步，
 * 为的是 `.mode-tab-active { border-color / background-color }` 不能被误读成 `color`：
 * 白名单只约束**声明的属性名**，`border-bottom-color` 是边框属性，落在 `<view>` 上完全合法。
 */
function cssRules(css) {
  const rules = [];
  const stack = [];
  let buf = '';
  let bufStartLine = 1;
  let line = 1;

  const takeDecl = () => {
    const text = buf.trim();
    const m = text.match(/^([A-Za-z-][^:;{}]*):(.+)$/s);
    if (!m) return;
    const top = stack[stack.length - 1];
    if (top && !/^@/.test(top.prelude)) top.decls.push({ prop: m[1].trim().toLowerCase(), line: bufStartLine });
  };

  for (let i = 0; i < css.length; i++) {
    const ch = css[i];
    if (ch === '\n') { line += 1; buf += ch; continue; }
    if (ch === '{') { stack.push({ prelude: buf.trim(), line: bufStartLine, decls: [] }); buf = ''; continue; }
    if (ch === '}') {
      takeDecl();
      const top = stack.pop();
      if (top && !/^@/.test(top.prelude) && top.decls.length > 0) {
        rules.push({ selector: top.prelude, line: top.line, decls: top.decls });
      }
      buf = '';
      continue;
    }
    if (ch === ';') { takeDecl(); buf = ''; continue; }
    if (buf.trim() === '' && !/\s/.test(ch)) bufStartLine = line;
    buf += ch;
  }
  return rules;
}

/**
 * 选择器 → 待判分支 `[{ raw, classes }]`：逗号分组（`.a, .b`）**逐支**判，
 * 每支只取**最后一个 compound** 的 class 列表（`.box .label` → `['label']`；`.a.b` → `['a','b']`）。
 */
function selectorBranches(selector) {
  return String(selector)
    .split(',')
    .map((alt) => alt.trim())
    .filter(Boolean)
    .map((raw) => {
      const compounds = raw.split(/[\s>+~]+/).filter(Boolean);
      const last = compounds[compounds.length - 1] || '';
      const classes = [];
      for (const m of last.matchAll(/\.(-?[_A-Za-z][\w-]*)/g)) classes.push(m[1]);
      return { raw, classes };
    });
}

/* ------------------------------------------------------------------ 扫描 */

/**
 * 纯函数：单个 `.uvue` 源码 + 仓库根相对路径 → `{ occurrences, violations, carriers }`。
 * 一条规则的每个属性组各算一个 occurrence（`.x { font-size; font-weight }` 是两个组）。
 */
function scanFontCarrier(file, source) {
  const tpl = stripHtmlComments(templateOf(source));
  const css = stripCssComments(styleTextOf(source));
  const carriers = classCarriers(tpl);

  const occurrences = [];
  const violations = [];

  const judge = (where, groups, resolve) => {
    for (const group of groups) {
      const props = [];
      for (const d of group.decls) if (!props.includes(d.prop)) props.push(d.prop);
      if (props.length === 0) continue;
      const { tags, badClasses } = resolve(group);
      const illegal = tags.filter((t) => !group.carriers.includes(t));
      const message = illegal.length === 0 ? '' : where(group) + ' —— 承载非法：class 挂在 <'
        + illegal.join('> / <') + '> 上，而 ' + PROPERTY_LABEL(props)
        + ' 只在 ' + CARRIER_LABEL(group.carriers) + ' 上有效 ⇒ 该位的这些声明被渲染层判错并忽略'
        + '（修法：把文字挪进 `<text>` 子元素，让这些声明跟到 `<text>` 的 class 上）';
      occurrences.push({
        file, line: group.line, selector: group.selector, props, tags, badClasses,
        ok: illegal.length === 0, message,
      });
      if (message) violations.push(message);
    }
  };

  /* 规则面：`<style>` 里的规则 */
  judge(
    (g) => file + ':' + g.line + '（选择器 `' + (g.selector || '(空)') + '`）',
    (() => {
      const out = [];
      for (const r of cssRules(css)) {
        for (const group of PROPERTY_RULES) {
          const decls = r.decls.filter((d) => group.props.includes(d.prop));
          if (decls.length > 0) out.push({ selector: r.selector, line: r.line, decls, carriers: group.carriers });
        }
      }
      return out;
    })(),
    (group) => {
      const tags = new Set();
      const badClasses = new Set();
      for (const branch of selectorBranches(group.selector)) {
        let inter = null;
        for (const cls of branch.classes) {
          const hit = carriers.get(cls);
          if (!hit || hit.size === 0) continue; // 承载不可判定 ⇒ 不约束（见文件头边界）
          inter = inter === null ? new Set(hit) : new Set([...inter].filter((t) => hit.has(t)));
        }
        const branchTags = [...(inter || [])];
        for (const t of branchTags) tags.add(t);
        if (branchTags.some((t) => !group.carriers.includes(t))) branch.classes.forEach((c) => badClasses.add(c));
      }
      return { tags: [...tags], badClasses: [...badClasses] };
    }
  );

  /* 模板内联 `style="…"`：只判承载标签（无 class 映射可查） */
  judge(
    (g) => file + ':' + g.line + '（内联 style）',
    (() => {
      const out = [];
      for (const { tag, attrs, index } of openingTags(tpl)) {
        for (const m of attrs.matchAll(/(?:^|\s)style\s*=\s*"([^"]*)"/g)) {
          const decls = [];
          for (const piece of m[1].split(';')) {
            const mm = piece.match(/^\s*([A-Za-z-][^:;]*):(.+)$/);
            if (mm) decls.push({ prop: mm[1].trim().toLowerCase(), line: lineAt(tpl, index) });
          }
          if (decls.length === 0) continue;
          for (const group of PROPERTY_RULES) {
            const hit = decls.filter((d) => group.props.includes(d.prop));
            if (hit.length > 0) {
              out.push({ selector: tag, line: lineAt(tpl, index), decls: hit, carriers: group.carriers, tag });
            }
          }
        }
      }
      return out;
    })(),
    (group) => ({ tags: [group.tag], badClasses: [] })
  );

  return { occurrences, violations };
}

/** 递归收集项目内 `.uvue`（跳过构建产物、依赖与第三方原生插件面） */
function collectUvue(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      // 点目录一律跳。承重的是 `.scratch/`：①a 取证的复现命令会把**术前** `.uvue` 副本写到那儿
      // （见 `docs/verification/uvue-font-carrier/1269/README.md` 复现段），而它已被 gitignore ⇒
      // 扫进来会让「全仓零违规」被本机产物判红，门的含义退化成「本机恰好干净才绿」。
      // `.git` / `.ci-verify` 同族（先例 `navQueryKeyContract.test.js` 的跳过表）。
      if (e.name.startsWith('.')) continue;
      if (e.name === 'unpackage' || e.name === 'node_modules' || e.name === 'uni_modules') continue;
      collectUvue(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

/* ------------------------------------------------ 已知存量（棘轮：只减不增） */

/**
 * 票面写明「**本票不顺手扩大范围**」⇒ 全仓扫出来但**不在本票射程**的违规位逐条登记在此，各附理由。
 *
 * 这**不是豁免清单**，与 `uvueWhiteSpaceContract` 的 `LEGAL_CARRIER_SITES` 也不是一回事（那份登记的是
 * 「已核过的合法位」，这份登记的是「**已知仍坏着**的位」）。它的两条性质：
 *   · **上界**：不在这里的新违规一律判红 ⇒ 全仓锁死，新页面加不出第 6 处（现清单 5 位 / **7** 条
 *     occurrence，2026-09-24 实测）。**判据是条数而不是「这个键命中过」** —— 键取「文件 + class」，
 *     一个 class 在两个属性组上非法就是 2 条（`picker-title` 两位各 2 条），所以同一位**再加一条**
 *     同键违规会被键吞掉；登记每条的 occurrence 数（`occurrences` 字段）才真的锁得住。
 *   · **自净**：条目对应的 occurrence 数一旦**变小**（有人顺手修了那一页，全修或部分修），判据立刻判红，
 *     逼改的人回来更新数字或删条目 ⇒ 清单只会变短，不会长成一永久豁免区。
 * 收口方式：谁改到那一页，谁在同一个 PR 里删掉该页的文字并落进 `<text>`（改法见每条 `why`）。
 * 三条判据（新位 / 同位加条 / 修好）都由 `ratchetFindings` 承载，并在 ① 节用注入样本自检。
 */
const DEFERRED = [
  {
    file: 'components/ai-chat/ai-chat-custom-form.uvue',
    className: 'picker-title',
    occurrences: 2, // 两个属性组各一条（`font-size/color/text-align` + `font-weight`）
    why: 'ai-chat 面，不在本票射程。改法：标题文字挪进 <text>，`font-size/color/text-align/font-weight` 跟过去',
  },
  {
    file: 'components/ai-chat/ai-chat-custom-form.uvue',
    className: 'custom-hint',
    occurrences: 1,
    why: 'ai-chat 面，不在本票射程。改法同上（`font-size/color`）',
  },
  {
    file: 'components/ai-chat/ai-chat-model-picker.uvue',
    className: 'picker-title',
    occurrences: 2,
    why: 'ai-chat 面，不在本票射程。改法同 custom-form 的同名 class',
  },
  {
    file: 'components/ai-chat/ai-chat-model-picker.uvue',
    className: 'picker-empty',
    occurrences: 1,
    why: 'ai-chat 面，不在本票射程。改法：空态文案挪进 <text>（`text-align/font-size/color`）',
  },
  {
    file: 'pages/recruiter/resume-detail.uvue',
    className: 'paragraph',
    occurrences: 1,
    why: '招募面，不在本票射程。该 class 另有 5 处落在 <text> 上（合法），只有加载态那一位挂在 <view> 上'
      + ' ⇒ 改法是**删掉那一位的 class**：那一位里唯一的文字是 <text class="meta-item">，它自己声明了同值的'
      + ' `26rpx / #666666` ⇒ 删了零视觉变化，比其余四条都便宜',
  },
];
const deferredKey = (d) => d.file + '#' + d.className;

/**
 * 「已判定」 occurrence 数的下界。2026-09-24 现测：文件 126 · occurrence **1577** · 承载解析得出来的 **1506**
 * ⇒ 盲区 = **71 个 occurrence**（同一批按 class 名去重是 **52 种**，只在「带文字类样式的规则」里数）。
 * 取 **1200** 留 ~20% 余量：排版级改动不该让它掉下 300，而「模板整面读不到」会让它直接归零
 * （那条另有逐文件的 `templateOf` 非空断言兜着）。
 */
const JUDGED_MIN = 1200;

/**
 * 「有全局样式、却没有模板」的文件 —— 本守护**具名**的盲区。
 *
 * uni-app-x 的 `App.uvue` 是应用根组件，只有 `<script>` + 全局 `<style>`，按设计就没有 `<template>`。
 * 本守护的「class → 承载」映射**按同文件**建 ⇒ 对它建出来是空的 ⇒ 它的全局规则一条也不判。
 * 「不判」不误红，但也意味着这条边界不写出来就会长得悄无声息。
 *
 * 为什么不跨文件判：那要把「同名 class 在 A 页挂 `<button>`、在 B 页挂 `<view>`」算成违规，
 * 前提是**全局样式参与渲染层的承载校验** —— 而 #1269 已有的真机证据里，错误行全部由页面局部样式触发
 * （见 `docs/verification/forgot-password/1263/machine-lines.txt`）。拿一个未证的假设去落一条会判红的锁，
 * 判据的代价比漏洞高。现测 App.uvue 三条带文字类样式的 class：`input-base` 的使用点全在 `<input>` 上、
 * `btn-primary` 在 `<button>` 上（另有一处页面局部同名 class 已按「容器 + `<text>`」的正确形态写）、
 * `page-base` 全仓无使用点 ⇒ 这条盲区目前**没有已知的受害位**。
 *
 * 断言取 `toEqual` 而不是 `toContain` ⇒ 清单只允许被**显式**改动：新增无模板文件 = 有人造出了第二处静默盲区。
 */
const NO_TEMPLATE_OK = ['App.uvue'];

/**
 * 扫描结果 → 违规位清单 `[{ key: '<file>#<class>', …occurrence }]`。
 * 键取「文件 + 涉事 class」而不是行号：行号会随任何一次排版漂移，那样登记的清单必烂。
 */
function violatingOccurrences(scanned) {
  const out = [];
  for (const s of scanned) {
    for (const o of s.occurrences) {
      if (o.ok) continue;
      for (const cls of (o.badClasses.length > 0 ? o.badClasses : ['（内联 style）'])) {
        out.push(Object.assign({}, o, { key: s.file + '#' + cls }));
      }
    }
  }
  return out;
}

/**
 * 棘轮判定（纯函数）：输入违规 occurrence 清单（真实扫描或注入样本），输出三条红的文案。
 *
 * 三条各对一种失效方向，缺一条就会长成假锁：
 *   · `fresh` —— 出现了没登记的新位（新 class / 新文件）；
 *   · `over` —— **登记位又长出一条**同键违规。键是「文件 + class」，只看「命中与否」会把它吞掉，
 *     故比对的是 occurrence 条数（见 `DEFERRED` 注释）；
 *   · `stale` —— 登记位的条数变小或归零（有人修好了那一页，全修或部分修）⇒ 逼回来更新/删条目。
 */
function ratchetFindings(viol, deferred = DEFERRED) {
  const byKey = new Map(deferred.map((d) => [deferredKey(d), d]));
  const counts = new Map();
  for (const o of viol) counts.set(o.key, (counts.get(o.key) || 0) + 1);
  const numberOf = (d) => counts.get(deferredKey(d)) || 0;
  return {
    fresh: viol.filter((o) => !byKey.has(o.key)).map((o) => o.message),
    over: [...counts.entries()]
      .filter(([key, n]) => byKey.has(key) && n > byKey.get(key).occurrences)
      .map(([key, n]) => key + ' —— 登记时 ' + byKey.get(key).occurrences + ' 条，现测 ' + n
        + ' 条：同一个登记位又长出违规。登记表只减不增 ⇒ 把它挪进 `<text>`，'
        + '或在本 PR 里显式说明为什么这处不算红'),
    stale: deferred.filter((d) => numberOf(d) < d.occurrences).map((d) => deferredKey(d)
      + ' —— 登记 ' + d.occurrences + ' 条，现测 ' + numberOf(d) + ' 条：该位已被部分或全部修好'
      + ' ⇒ 删条目或把 `occurrences` 改小（这张清单只允许变短）'),
  };
}

/* ------------------------------------------------------------------ 测试 */

const FILES = collectUvue(ROOT)
  .map((p) => path.relative(ROOT, p).replace(/\\/g, '/'))
  .sort();
const readSource = (rel) => readText(path.join(ROOT, rel));
const uvue = (tpl, css) => `<template>\n${tpl}\n</template>\n\n<style lang="scss">\n${css}\n</style>\n`;
const violationsOf = (tpl, css) => scanFontCarrier('pages/x.uvue', uvue(tpl, css)).violations.join('\n');

describe('uvue 文字类样式的承载面契约（#1269）', () => {
  /* ---------------- ① 违规样本必须判得出来（防空跑假绿） ---------------- */
  describe('① 检测器自检：注入的违规样本必须判红', () => {
    const cases = [
      [
        '`font-size` 落在 <view> 上（本票 mode-tab 的形态）',
        '<view class="mode-tab">手机号找回</view>',
        '.mode-tab { font-size: 28rpx; }',
        /承载非法[\s\S]*<view>[\s\S]*`font-size`[\s\S]*<text>\|<button>\|<input>\|<textarea>/,
      ],
      ['`color` 落在 <view> 上', '<view class="tab">a</view>', '.tab { color: #666666; }', /承载非法[\s\S]*<view>/],
      ['`font-weight` 落在 <view> 上', '<view class="tab">a</view>', '.tab { font-weight: bold; }', /承载非法[\s\S]*`font-weight`[\s\S]*<loading>/],
      ['`text-align` 落在 <view> 上（同族日志原文点名）', '<view class="t">a</view>', '.t { text-align: center; }', /承载非法[\s\S]*`text-align`/],
      [
        '`<scroll-view>` 同样是非法承载',
        '<scroll-view class="tip" scroll-y="true"></scroll-view>',
        '.tip { color: #999999; }',
        /承载非法[\s\S]*<scroll-view>/,
      ],
      [
        '同一个 class 既挂 <text> 又挂 <view>（部分合法也判红：那位是死声明）',
        '<text class="paragraph">甲</text><view class="paragraph"><text class="meta">乙</text></view>',
        '.paragraph { font-size: 26rpx; }',
        /承载非法[\s\S]*<view>/,
      ],
      [
        '动态对象键 :class 挂上的 class（本票 `.mode-tab-active` 的形态）',
        '<view class="tab" :class="{ \'tab-active\': on }">甲</view>',
        '.tab-active { font-weight: bold; }',
        /（选择器 `\.tab-active`）[\s\S]*<view>/,
      ],
      [
        '未加引号的对象键也算承载',
        '<view class="tab" :class="{ active: on }">甲</view>',
        '.active { color: #2979ff; }',
        /（选择器 `\.active`）[\s\S]*<view>/,
      ],
      [
        '三元分支字面量也算承载',
        '<view :class="on ? \'tab-on\' : \'tab-off\'">甲</view>',
        '.tab-on { color: #2979ff; }\n.tab-off { color: #666666; }',
        /承载非法[\s\S]*<view>/,
      ],
      [
        '后代选择器看最后一段：`.box .label` 的载体是 <view> 上的 .label',
        '<view class="box"><view class="label">甲</view></view>',
        '.box .label { font-size: 28rpx; }',
        /（选择器 `\.box \.label`）[\s\S]*承载非法[\s\S]*<view>/,
      ],
      [
        '逗号分组选择器逐支判：`.a, .b` 里 .b 落在 <view> 上就判红',
        '<text class="a">甲</text><view class="b">乙</view>',
        '.a, .b { color: #666666; }',
        /（选择器 `\.a, \.b`）[\s\S]*承载非法[\s\S]*<view>/,
      ],
      [
        '模板内联 style 也判（写在 <view> 上）',
        '<view style="font-size: 28rpx;">甲</view>',
        '.none { padding: 0; }',
        /内联 style[\s\S]*承载非法[\s\S]*<view>/,
      ],
    ];

    test.each(cases)('%s', (_name, tpl, css, expected) => {
      expect(violationsOf(tpl, css)).toMatch(expected);
    });

    it('行号落在源文件的那一行上，不是「样式块内第 N 行」', () => {
      const src = uvue('<view class="tab">甲</view>', '\n\n.tab { color: #666666; }');
      const want = src.slice(0, src.indexOf('.tab {')).split('\n').length;
      const v = scanFontCarrier('pages/x.uvue', src).violations.join('\n');
      expect(v).toContain('pages/x.uvue:' + want + '（');
      // 块内相对行号必然更小（块从 `<style>` 那一行起算）——只写相对行的实现会在这里露馅
      expect(want).toBeGreaterThan(4);
    });

    it('违规清单要同时点出「哪几条声明是死的」与「只能落在哪些元素」—— 只说「违规」的文案不可执行', () => {
      const v = violationsOf('<view class="t">甲</view>', '.t { font-size: 28rpx; color: #666666; }');
      expect(v).toMatch(/`font-size` \/ `color`/);
      expect(v).toMatch(/只在 <text>\|<button>\|<input>\|<textarea> 上有效/);
      expect(v).toMatch(/修法[\s\S]*<text>/);
      // 只点名**该规则真写了**的属性：把整组的三个都念一遍，读者会去删没写的声明
      expect(violationsOf('<view class="t">甲</view>', '.t { font-size: 28rpx; }')).not.toMatch(/`text-align`/);
    });

    it('棘轮三条判据都要能被抓到：新位 / 登记位再加一条 / 登记位被修好', () => {
      // 输入用「按登记表原样长出来的」违规清单：先证明它自身不判红（否则下面三条红是空跑假绿），
      // 再逐条加/减，看对应的判据是否真的响。
      const at = (key, n) => Array.from({ length: n }, () => ({ key, message: key + ' 承载非法' }));
      const asRegistered = DEFERRED.flatMap((d) => at(deferredKey(d), d.occurrences));
      expect(ratchetFindings(asRegistered)).toEqual({ fresh: [], over: [], stale: [] });

      const added = (key) => ratchetFindings(asRegistered.concat(at(key, 1)));
      // ① 没登记过的位 ⇒ fresh
      expect(added('pages/x.uvue#brand-new-tab').fresh).toHaveLength(1);
      // ② **登记过的位**上再来一条 —— 这条正是「键命中就放行」会吞掉的方向
      const dup = added(deferredKey(DEFERRED[0]));
      expect(dup.over).toHaveLength(1);
      expect(dup.over[0]).toMatch(/登记时 2 条，现测 3 条/);
      expect(dup.fresh).toEqual([]); // 也不能被当成新位重复报
      // ③ 修好了（部分 / 全部）⇒ stale
      expect(ratchetFindings(asRegistered.slice(1)).stale).toHaveLength(1);
      const fixed = ratchetFindings(asRegistered.filter((o) => o.key !== deferredKey(DEFERRED[4])));
      expect(fixed.stale).toHaveLength(1);
      expect(fixed.stale[0]).toMatch(/现测 0 条[\s\S]*删条目/);
    });
  });

  /* ---------------- ② 合规样本不能判红（防误报） ---------------- */
  describe('② 检测器自检：合规样本必须判绿', () => {
    const cases = [
      ['文字样式落在 <text> 上（本票改完的形态）', '<text class="tab-text">甲</text>', '.tab-text { font-size: 28rpx; color: #666666; font-weight: bold; }'],
      ['落在 <button> 上', '<button class="btn">甲</button>', '.btn { font-size: 28rpx; color: #ffffff; }'],
      ['落在 <input> / <textarea> 上', '<input class="i" /><textarea class="a">甲</textarea>', '.i { color: #333333; }\n.a { font-size: 26rpx; }'],
      ['`font-weight` 落在 <loading> 上（该属性多出来的那一个合法载体）', '<loading class="l" />', '.l { font-weight: bold; }'],
      [
        '<view> 上的**边框 / 背景**色不是文字样式（正则扫块会把 `-color` 后缀误伤）',
        '<view class="tab">甲</view>',
        '.tab { background-color: #F0F1F5; border: 2rpx solid transparent; border-bottom-color: #2979ff; border-color: #2979ff; }',
      ],
      [
        '祖先类是 <view>、载体类是 <text> —— 后代选择器只看最后一段',
        '<view class="box"><text class="label">甲</text></view>',
        '.box .label { font-size: 28rpx; }',
      ],
      ['注释里写着 font-size 不算实现（票面坑位 2 同源）', '<view class="tab">甲</view>', '/* 旧的 .tab 上有 font-size: 28rpx，已挪到 text */\n.tab { padding: 8rpx; }'],
      ['模板注释里的 <view> + 样式里的 font-size 都不算', '<!-- <view class="ghost">甲</view> --><text class="real">甲</text>', '.ghost { font-size: 28rpx; }\n.real { font-size: 28rpx; }'],
      ['class 在模板里找不到承载（全局 / 动态 class）⇒ 不判', '<view class="other">甲</view>', '.from-global { color: #333333; }'],
      ['比较值不算幻影承载：`mode == \'phone\'` 里的 phone 不是 class', '<text class="t">甲</text>', '.phone { color: #333333; }\n.t { color: #333333; }'],
    ];

    test.each(cases)('%s', (_name, tpl, css) => {
      expect(violationsOf(tpl, css)).toBe('');
    });

    it('<loading> 只多授权 `font-weight`，不多授权 `font-size` / `color`（两组白名单不是一回事）', () => {
      expect(violationsOf('<loading class="l" />', '.l { font-weight: bold; }')).toBe('');
      expect(violationsOf('<loading class="l" />', '.l { font-size: 28rpx; }')).toMatch(/承载非法[\s\S]*<loading>/);
    });
  });

  /* ---------------- ③ 真实文件：扫描面是活的 + 全仓零命中 ---------------- */
  describe('③ 真实文件（全仓 .uvue）', () => {
    const scanned = FILES.map((file) => ({ file, ...scanFontCarrier(file, readSource(file)) }));

    it('扫描面自检：文件数与「class → 承载」映射都必须真的在工作（防空跑假绿）', () => {
      expect(FILES.length).toBeGreaterThan(100);
      // 全仓文字类样式的判定量：塌成 0 时下面那条「零命中」就是假绿
      const total = scanned.reduce((n, s) => n + s.occurrences.length, 0);
      expect(total).toBeGreaterThan(500);
      // `occurrences` 与「承载是否解析得出来」无关 ⇒ 只锁它会留一个洞：
      // class → 标签的映射整体塌掉（模板读不到）时它照样达标。这里另锁**已判定**的量。
      const judged = scanned.reduce((n, s) => n + s.occurrences.filter((o) => o.tags.length > 0).length, 0);
      expect(judged).toBeGreaterThan(JUDGED_MIN);
      // 逐文件：有 `<style>` 却取不到 `<template>` 的文件 = 本守护对它是瞎的（承载全不可判 ⇒ 静默放行）
      const blind = FILES.filter((f) => {
        const src = readSource(f);
        return styleTextOf(src).trim() !== '' && templateOf(src) === '';
      });
      // 等于**具名**清单（不是「空」也不是「被包含」）⇒ 新增无模板文件 = 新增一处静默盲区，必须显式过这里
      expect(blind).toEqual(NO_TEMPLATE_OK);
    });

    it('本票改动点是「容器挂 <view>、文字挂 <text>」—— 映射塌了零命中就是假的', () => {
      const carriersOf = (file, cls) => {
        const src = readSource(file);
        return [...(classCarriers(stripHtmlComments(templateOf(src))).get(cls) || [])].sort();
      };
      const pairs = [
        ['pages/forgot-password/forgot-password.uvue', 'mode-tab', ['view']],
        ['pages/forgot-password/forgot-password.uvue', 'mode-tab-text', ['text']],
        ['pages/forgot-password/forgot-password.uvue', 'mode-tab-text-active', ['text']],
        ['pages/register/register.uvue', 'mode-tab', ['view']],
        ['pages/register/register.uvue', 'mode-tab-text', ['text']],
        ['pages/register/register.uvue', 'mode-tab-text-active', ['text']],
      ];
      for (const [file, cls, want] of pairs) expect([file, cls, carriersOf(file, cls)]).toEqual([file, cls, want]);
    });

    it('本票改的两页零违规（`mode-tab` 系只剩布局与边框色）', () => {
      const mine = scanned.filter((s) => /^pages\/(forgot-password|register)\//.test(s.file) && s.violations.length > 0);
      expect(mine.map((s) => s.file + '\n    ' + s.violations.join('\n    ')).join('\n')).toBe('');
    });

    it('除「已知存量」外全仓零违规（新写的文字类样式必须落对承载）', () => {
      const { fresh } = ratchetFindings(violatingOccurrences(scanned));
      expect(fresh.join('\n')).toBe('');
    });

    it('DEFERRED 每条的 occurrence 数必须与登记时相同（修好了就删条目 —— 这张清单只允许变短）', () => {
      const { over, stale } = ratchetFindings(violatingOccurrences(scanned));
      expect([over.join('\n'), stale.join('\n')].filter(Boolean).join('\n')).toBe('');
    });
  });
});
