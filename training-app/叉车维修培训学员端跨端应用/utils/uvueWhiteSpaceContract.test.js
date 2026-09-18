/**
 * uvue `white-space` 承载面契约（#1113）—— 全仓 `.uvue` 源码守护。
 *
 * 为什么需要它：uvue 原生端的 `white-space` **只支持 `<text>` / `<button>`**，写在 `<scroll-view>`
 * 等元素上会被渲染层判错并**忽略**。真机日志原文（#1081 的 ①a 实测，设备 2510DRK44C / `f0bae674`）：
 *
 *   style property `white-space` is only supported on `<text>|<button>`.
 *   there is an error on `<scroll-view class="category-scroll" …>`.
 *
 * 被忽略 ⇒ 那是**死声明**（横滑靠同一个 class 上的 `flex-direction: row` + 子项 `flex-shrink: 0`
 * 撑出溢出，与它无关），但每进一次页就打一条 error 行 —— 污染 ①a 的「应用控制台错误行」判据
 * （#1040 / #1070 / #1110 都用它当证据面）。本仓存量 7 处里 6 处属此类（#1113 已删）；第 7 处
 * （`pages/profile/personal-info.uvue` 的 `.code-btn-text`）承载是 `<text>`，**合法** ——
 * 本守护的另一半职责就是让它不被后续会话「扫到就删」。
 *
 * 判据（两条同时成立才算合规）：
 *   ① **承载合法**：`white-space` 所在规则的 class，在模板里只挂在 `<text>` / `<button>` 上。
 *      这一条必须跨「class → 承载标签」映射（票面坑位 1）：本仓有合法用法，只看 class 名判不出。
 *   ② **已登记**：该 (文件, class) 在 `LEGAL_CARRIER_SITES` 里有条目、且附了理由。
 *      登记是**活的**：登记点被删 / 承载标签变了 ⇒ 判红（照 `utils/navQueryKeyContract.test.js` 的
 *      `GUARD_ALLOWLIST` 先例）。**为什么「合法」也要登记**：这一位就是「防扫到就删」的锁 ——
 *      不登记直接写会判红，删掉登记点也会判红（`LEGAL_CARRIER_SITES` 那条 liveness 断言）。
 *
 * 票面坑位 2：**必须先剥离 CSS 注释** —— 解释这个坑位的注释里本身写着 `white-space: nowrap`
 * （`pages/profile/help-center.uvue:248`、`pages/ai-assistant/ai-feature.uvue:538`），不剥注释就会把
 * 说明本身判红（#1081 的 `utils/helpCenterContract.test.js` 就是因此加了 `stripCssComments`）。
 *
 * 形态沿用本仓既有全仓守护（先例 `utils/gradientSyntaxContract.test.js`，同为「uvue 不支持某 CSS 写法」）：
 * 先对**注入的违规样本**断言检测有效（防空跑假绿），再对**真实文件**断言零命中。
 *
 * 已知边界（写实，不宣称覆盖）：
 *   · 只判 `<style>` 块里的**规则**，且只判规则选择器的**最后一个 compound**（属性落在它指的元素上，
 *     后代 / 子选择器的祖先类不参与判定）—— 本仓 7 处全是单 class 规则。
 *   · 模板里的**内联** `style="…"` 只判承载标签是否合法，**不**要求登记（2026-09-18 现测 0 处）。
 *   · scss 的 `//` 行注释不剥（本仓样式块零使用）；`:class` 动态绑定按对象键 / 三元分支收集字面量。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');

/** 允许承载 `white-space` 的标签 —— uvue 原生端只在这两个上支持该属性（真机日志口径） */
const LEGAL_CARRIERS = ['text', 'button'];

/**
 * 已知「合法承载」登记表（**例外必须是活的**，见文件头判据 ②）。
 *
 * 加一条的前提：承载确实是 `<text>` / `<button>`，且**真的需要** `white-space`（例如一段不能换行的
 * 倒计时 / 单行按钮文案）。横滑行的溢出**不需要**它 —— 用 `flex-direction: row` + 子项 `flex-shrink: 0`。
 * 清掉一条（声明没了 / 承载改成别的标签）时**必须同批删条目**，否则 liveness 断言判红。
 */
const LEGAL_CARRIER_SITES = [
  {
    file: 'pages/profile/personal-info.uvue',
    className: 'code-btn-text',
    carrier: 'text',
    why:
      '「获取验证码」按钮里的倒计时文本（`<text class="code-btn-text">`，同页 4 处）：倒计时读秒时'
      + '文案在 `N s` ↔ `获取验证码` 之间切换，长度变化会让按钮宽度抖动 —— 需要单行不换行。'
      + '承载是 `<text>`（本仓 4 处逐处核过），是**合法用法**，不是 #1113 要清的死声明，**不要删**。',
  },
];

/* ------------------------------------------------------------------ 取段 / 剥注释 */

/**
 * 取 `<style>` 块文本，并**把行号对齐到源文件**（块之前的文本用等量换行顶替）——
 * 违规文案里的 `file:line` 要能直接跳到源文件那一行，不是「样式块内第 N 行」。
 */
function styleTextOf(source) {
  const out = [];
  const re = /<style[^>]*>[\s\S]*?<\/style>/g;
  let m;
  while ((m = re.exec(source)) !== null) {
    out.push('\n'.repeat(lineAt(source, m.index) - 1) + m[0]);
  }
  return out.join('\n');
}

/** 取 `<template>` 块文本（承载标签只能从这里读）；同样对齐行号 */
function templateOf(source) {
  const start = source.indexOf('<template>');
  const end = source.lastIndexOf('</template>');
  if (start === -1 || end === -1 || end < start) return '';
  return '\n'.repeat(lineAt(source, start) - 1) + source.slice(start, end);
}

/** 剥 CSS 注释 —— 注释里写着 `white-space` 不算实现（票面坑位 2，假阳性来源） */
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
 * 属性区允许被引号包住的 `>`（正则里的 `"[^"]*"|'[^']*'` 两支），否则 `<view v-if="a > b">` 会被截断。
 */
function openingTags(tpl) {
  const re = /<([A-Za-z][\w-]*)((?:"[^"]*"|'[^']*'|[^>"'])*)>/g;
  const out = [];
  for (const m of tpl.matchAll(re)) out.push({ tag: m[1].toLowerCase(), attrs: m[2], index: m.index });
  return out;
}

/**
 * 纯函数：模板源码 → `Map<class, Set<承载标签>>`（**本守护多出来的那一步**，票面坑位 1）。
 *
 * 三类来源都收：
 *   · 静态 `class="a b"`；
 *   · `:class="{ 'a': cond }"` 的**对象键**（键 = class 名）；
 *   · `:class="cond ? 'a' : 'b'"` 的分支字面量。
 * 比较值（`activeTab == 'all'` 里的 `all`）**不算** class —— 那是值，收了会造出「幻影承载」。
 */
function classCarriers(tpl) {
  const map = new Map();
  const add = (cls, tag) => {
    if (!cls || /\s/.test(cls)) return;
    if (!map.has(cls)) map.set(cls, new Set());
    map.get(cls).add(tag);
  };

  for (const { tag, attrs } of openingTags(tpl)) {
    // 静态 class="…" / class='…'
    for (const m of attrs.matchAll(/(?:^|\s)class\s*=\s*"([^"]*)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => add(c, tag));
    }
    for (const m of attrs.matchAll(/(?:^|\s)class\s*=\s*'([^']*)'/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => add(c, tag));
    }
    // 动态 :class="…"
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
    }
  }
  return map;
}

/* ------------------------------------------------------------ CSS 规则解析 */

/**
 * 纯函数：CSS 文本 → 所有 `white-space` 声明 `[{ selector, line, declaration }]`。
 * 用大括号栈取**最近的、非 at-rule 的**前导选择器（`@media` 里的规则照样能定位到自己的选择器）。
 */
function whiteSpaceDeclarations(css) {
  const out = [];
  const stack = [];
  let buf = '';
  let bufStartLine = 1;
  let line = 1;

  const flush = () => {
    const text = buf.trim();
    buf = '';
    if (!/^white-space\s*:/i.test(text)) return;
    const owner = [...stack].reverse().find((s) => !/^@/.test(s.prelude));
    out.push({ selector: owner ? owner.prelude : '', line: bufStartLine, declaration: text });
  };

  for (let i = 0; i < css.length; i++) {
    const ch = css[i];
    if (ch === '{') { stack.push({ prelude: buf.trim() }); buf = ''; continue; }
    if (ch === '}') { flush(); stack.pop(); buf = ''; continue; }
    if (ch === ';') { flush(); buf = ''; continue; }
    if (ch === '\n') { line += 1; buf += ch; continue; }
    if (buf.trim() === '' && !/\s/.test(ch)) bufStartLine = line;
    buf += ch;
  }
  flush();
  return out;
}

/** 选择器里最后一段 compound 的 class（`.a .b.c` → `['b', 'c']`）—— 属性落在它指的元素上 */
function selectorClasses(selector) {
  const compounds = String(selector).split(/[\s>+~]+/).filter(Boolean);
  const last = compounds[compounds.length - 1] || '';
  const out = [];
  for (const m of last.matchAll(/\.(-?[_A-Za-z][\w-]*)/g)) out.push(m[1]);
  return out;
}

/* ------------------------------------------------------------------ 扫描 */

/**
 * 纯函数：单个 `.uvue` 源码 + 仓库根相对路径 → `{ occurrences, violations, carriers }`。
 * `sites` 可注入（自检用假登记表），默认走真实 `LEGAL_CARRIER_SITES`。
 */
function scanWhiteSpace(file, source, sites = LEGAL_CARRIER_SITES) {
  const tpl = stripHtmlComments(templateOf(source));
  const css = stripCssComments(styleTextOf(source));
  const carriers = classCarriers(tpl);
  const registered = (cls) => sites.some((s) => s.file === file && s.className === cls);

  const occurrences = [];
  const violations = [];

  for (const d of whiteSpaceDeclarations(css)) {
    const classes = selectorClasses(d.selector);
    const tags = new Set();
    const unknown = [];
    for (const cls of classes) {
      const hit = carriers.get(cls);
      if (!hit || hit.size === 0) { unknown.push(cls); continue; }
      for (const t of hit) tags.add(t);
    }
    const illegal = [...tags].filter((t) => !LEGAL_CARRIERS.includes(t));

    const problems = [];
    if (classes.length === 0) {
      problems.push('选择器里没有 class（uvue 原生端只支持 class 选择器，该声明不生效）');
    }
    if (illegal.length > 0) {
      problems.push(
        '承载非法：该 class 挂在 <' + illegal.join('> / <') + '> 上，而 `white-space` 只在 <text> / <button> 上有效'
        + ' ⇒ 删掉这条声明（横滑靠 flex-direction: row + 子项 flex-shrink: 0 撑出溢出）'
      );
    }
    if (unknown.length > 0) {
      problems.push(
        '承载不可判定：class `' + unknown.join('` / `') + '` 在模板里找不到承载标签（死声明，或动态绑定的 class 没被识别）'
      );
    }
    if (problems.length === 0) {
      const missing = classes.filter((c) => !registered(c));
      if (missing.length > 0) {
        problems.push(
          '承载合法（<' + [...tags].join('> / <') + '>）但未登记：把 `' + missing.join('` / `')
          + '` 加进 LEGAL_CARRIER_SITES（附一句理由）'
        );
      }
    }

    const where = file + ':' + d.line + '（选择器 `' + (d.selector || '(空)') + '`）';
    if (problems.length > 0) violations.push(where + ' —— ' + problems.join('；'));
    occurrences.push({
      file, line: d.line, selector: d.selector, classes, tags: [...tags], ok: problems.length === 0,
    });
  }

  // 模板内联 style（只判承载标签；本仓现测 0 处）
  for (const { tag, attrs, index } of openingTags(tpl)) {
    for (const m of attrs.matchAll(/(?:^|\s)style\s*=\s*"([^"]*)"/g)) {
      if (!/white-space\s*:/i.test(m[1])) continue;
      const line = lineAt(tpl, index);
      if (LEGAL_CARRIERS.includes(tag)) {
        occurrences.push({ file, line, selector: '（内联 style）', classes: [], tags: [tag], ok: true });
      } else {
        violations.push(
          file + ':' + line + '（内联 style）—— 承载非法：`white-space` 写在 <' + tag
          + '> 上，只在 <text> / <button> 上有效'
        );
      }
    }
  }

  return { occurrences, violations, carriers };
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
      if (e.name === 'unpackage' || e.name === 'node_modules' || e.name === 'uni_modules') continue;
      collectUvue(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

/* ------------------------------------------------------------------ 测试 */

const FILES = collectUvue(ROOT)
  .map((p) => path.relative(ROOT, p).replace(/\\/g, '/'))
  .sort();
const readSource = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');
const uvue = (tpl, css) => `<template>\n${tpl}\n</template>\n\n<style lang="scss">\n${css}\n</style>\n`;

describe('uvue `white-space` 承载面契约（#1113）', () => {
  /* ---------------- ① 违规样本必须判得出来（防空跑假绿） ---------------- */
  describe('① 检测器自检：注入的违规样本必须判红', () => {
    const cases = [
      [
        '落在 <scroll-view> 上（本票 6 处的形态）',
        'pages/profile/favorites.uvue',
        uvue('<scroll-view class="filter-scroll" scroll-x="true"></scroll-view>', '.filter-scroll { white-space: nowrap; flex-direction: row; }'),
        /承载非法[\s\S]*<scroll-view>/,
      ],
      [
        '同一个 class 同时挂在 <text> 与 <scroll-view> 上（部分合法也要判红）',
        'pages/x.uvue',
        uvue('<text class="chip">a</text><scroll-view class="chip" scroll-x="true"></scroll-view>', '.chip { white-space: nowrap; }'),
        /承载非法[\s\S]*<scroll-view>/,
      ],
      [
        '落在 <view> 上',
        'pages/x.uvue',
        uvue('<view class="diag-chips"></view>', '.diag-chips { white-space: nowrap; }'),
        /承载非法[\s\S]*<view>/,
      ],
      [
        '落在 <image> 上',
        'pages/x.uvue',
        uvue('<image class="pic" src="/a.png"></image>', '.pic { white-space: nowrap; }'),
        /承载非法[\s\S]*<image>/,
      ],
      [
        '合法承载 <text> 但未登记',
        'pages/x.uvue',
        uvue('<text class="tip">x</text>', '.tip { white-space: nowrap; }'),
        /承载合法[\s\S]*未登记/,
      ],
      [
        '合法承载 <button> 但未登记',
        'pages/x.uvue',
        uvue('<button class="btn">x</button>', '.btn { white-space: nowrap; }'),
        /承载合法[\s\S]*未登记/,
      ],
      [
        'class 在模板里找不到承载（死声明 / 承载不可判定）',
        'pages/x.uvue',
        uvue('<view class="other"></view>', '.gone { white-space: nowrap; }'),
        /承载不可判定/,
      ],
      [
        '选择器里没有 class（tag 选择器，uvue 不支持）',
        'pages/x.uvue',
        uvue('<view class="a"></view>', 'view { white-space: nowrap; }'),
        /选择器里没有 class/,
      ],
      [
        '模板内联 style 落在 <scroll-view> 上',
        'pages/x.uvue',
        '<template>\n<scroll-view style="white-space: nowrap" scroll-x="true"></scroll-view>\n</template>\n',
        /内联 style[\s\S]*承载非法/,
      ],
    ];
    cases.forEach(([name, file, src, pattern]) => {
      it(name, () => {
        const { violations } = scanWhiteSpace(file, src, []);
        expect(violations.length).toBeGreaterThan(0);
        expect(violations.join('\n')).toMatch(pattern);
      });
    });
  });

  /* ---------------- ② 合规样本 / 注释不得误报 ---------------- */
  describe('② 检测器自检：合规样本必须判绿（防误报）', () => {
    const OK_SITES = [
      { file: 'pages/x.uvue', className: 'tip', carrier: 'text', why: '自检用假登记' },
      { file: 'pages/x.uvue', className: 'btn', carrier: 'button', why: '自检用假登记' },
    ];
    const ok = [
      [
        'CSS 注释里写着该词不算实现（help-center.uvue:247-251 的形态）',
        uvue('<view class="category-scroll"></view>', '/* 不写 white-space: nowrap —— 只支持 <text>|<button> */\n.category-scroll { flex-direction: row; }'),
      ],
      [
        'HTML 注释里写着该词不算实现',
        uvue('<!-- 这里不要写 white-space: nowrap -->\n<view class="a"></view>', '.a { flex-direction: row; }'),
      ],
      [
        '已登记的 <text> 正例（属性合法落点）',
        uvue('<text class="tip">3s</text>', '.tip { white-space: nowrap; }'),
      ],
      [
        '已登记的 <button> 正例',
        uvue('<button class="btn">获取验证码</button>', '.btn { white-space: nowrap; }'),
      ],
      [
        '承载来自动态 :class 对象键（映射必须认得出来）',
        uvue('<text :class="{ \'tip\': counting }">3s</text>', '.tip { white-space: nowrap; }'),
      ],
      [
        '承载来自动态 :class 三元分支',
        uvue('<button :class="counting ? \'btn\' : \'\'">3s</button>', '.btn { white-space: nowrap; }'),
      ],
      [
        '文件里完全没有这个词',
        uvue('<view class="a"></view>', '.a { flex-direction: row; }'),
      ],
    ];
    ok.forEach(([name, src]) => {
      it(name, () => {
        expect(scanWhiteSpace('pages/x.uvue', src, OK_SITES).violations).toEqual([]);
      });
    });

    it('比较值不算承载（`activeTab == \'all\'` 里的 all 不得被当成 class）', () => {
      const src = uvue(
        '<view :class="{ \'is-active\': activeTab == \'all\' }"></view>',
        '.is-active { color: #2979ff; }'
      );
      expect([...scanWhiteSpace('pages/x.uvue', src, []).carriers.keys()]).toEqual(['is-active']);
    });
  });

  /* ---------------- ③ 真实文件：零命中 + 登记表是活的 ---------------- */
  describe('③ 真实文件（全仓 .uvue）', () => {
    const scanned = FILES.map((file) => ({ file, ...scanWhiteSpace(file, readSource(file)) }));

    it('扫描面自检：文件数、class→承载映射、本票 6 处被删点的映射都必须真的抓到了（防空跑假绿）', () => {
      expect(FILES.length).toBeGreaterThan(100);

      // 本票 6 处被删的 class 至今仍挂在 <scroll-view> 上 —— 若映射塌了，第 3 条测试的「零命中」就是假绿
      const removed = [
        ['pages/profile/favorites.uvue', 'filter-scroll'],
        ['pages/profile/records.uvue', 'filter-scroll'],
        ['pages/profile/practice-records.uvue', 'filter-scroll'],
        ['pages/featured/featured-list.uvue', 'filter-scroll'],
        ['pages/ai-assistant/ai-feature.uvue', 'diag-chips'],
        ['pages/exam/mock-exam.uvue', 'palette-scroll'],
      ];
      for (const [file, cls] of removed) {
        const tpl = stripHtmlComments(templateOf(readSource(file)));
        const tags = [...(classCarriers(tpl).get(cls) || [])];
        expect([file, cls, tags]).toEqual([file, cls, ['scroll-view']]);
      }

      // 合法承载那一位也必须解析得到（登记表的 liveness 才有意义）
      const tpl = stripHtmlComments(templateOf(readSource('pages/profile/personal-info.uvue')));
      expect([...(classCarriers(tpl).get('code-btn-text') || [])]).toEqual(['text']);
      expect(classCarriers(tpl).size).toBeGreaterThanOrEqual(30);

      // 全仓映射总量（class → 承载）—— 塌成 0 时第 3 条测试会「零命中」假绿
      const totalMapped = scanned.reduce((n, s) => n + s.carriers.size, 0);
      expect(totalMapped).toBeGreaterThan(400);
    });

    it('全仓 `.uvue` 的 `white-space` 只出现在「承载合法 + 已登记」的规则上', () => {
      const offenders = scanned
        .filter((s) => s.violations.length > 0)
        .map((s) => s.violations.join('\n    '));
      expect(offenders.join('\n')).toBe('');
    });

    it('`LEGAL_CARRIER_SITES` 的例外必须是活的：登记点仍在，且承载标签与登记一致', () => {
      const stale = [];
      for (const site of LEGAL_CARRIER_SITES) {
        const hit = scanned
          .filter((s) => s.file === site.file)
          .flatMap((s) => s.occurrences)
          .find((o) => o.classes.includes(site.className) && o.tags.includes(site.carrier));
        if (!hit) stale.push(site.file + ' .' + site.className + '（登记为 <' + site.carrier + '>）');
      }
      // 清掉一条登记（声明没了 / 承载改了）必须同批删条目，禁静默留一条死登记
      expect({ stale }).toEqual({ stale: [] });
    });

    it('每条登记都附了理由，且类别只能是 <text> / <button>', () => {
      for (const site of LEGAL_CARRIER_SITES) {
        expect(LEGAL_CARRIERS).toContain(site.carrier);
        expect(typeof site.why === 'string' && site.why.length > 20).toBe(true);
      }
    });

    it('登记表与真实命中一致：全部命中点都被覆盖，且至少有一条（守护不是空跑）', () => {
      const legal = scanned.flatMap((s) => s.occurrences.filter((o) => o.classes.length > 0));
      expect(legal.length).toBe(LEGAL_CARRIER_SITES.length);
      for (const site of LEGAL_CARRIER_SITES) {
        expect(legal.some((o) => o.file === site.file && o.classes.includes(site.className))).toBe(true);
      }
    });
  });
});
