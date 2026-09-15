/**
 * uvue 实体字面量契约守护（#957）—— 源码契约测试缝（读 `<script>` 文本、断言无实体字面量）。
 *
 * 为什么需要它：2026-09-13 真机上，专业版能力清单三行图标显示为字面量 `&#10022;` / `&#9881;` /
 * `&#128206;`（而不是图形）。机制已在**编译产物级**证实，不需要真机二分 ——
 * 同一个组件的同一份产物里：
 *
 *   模板**静态**文本 `&#10005;` → 编译成字面字符 `"✕"`       （模板编译器**解码**）
 *   `<script>` 数据字符串 `&#10022;` → 编译成 `"&#10022;"`   （**不经过任何解码路径**）
 *
 * ⇒ 实体只在**模板静态文本**里被解码；`<script>` 里的字符串字面量原样下发到渲染层，
 * `{{ item.icon }}` 于是把 8 个 ASCII 字符上屏。**不报错、不警告**地静默失败。
 *
 * 所以本测试的**范围必须切准**：只禁 `<script>` 块内的实体字面量，**模板静态文本里的实体
 * 是合法写法**（本仓现存 11 处，一直渲染正常）。范围切错会把正常写法一起判红。
 *
 * 设计沿用本仓既有守护测试的形态（`utils/gradientSyntaxContract.test.js`，ADR 0010）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 *
 * 已知边界（如实声明）：注释剥离是自己写的极简状态机（跟踪字符串与注释），**不解析正则字面量** ——
 * 若某处正则字面量里含未转义的 `//` 或引号，可能被误判成注释起点而**漏报**（假阴性），
 * 不会误报（假阳性）。本仓 `.uvue` 的 `<script>` 里暂无这种写法。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');

/** 实体字面量：十进制 `&#10022;` 与十六进制 `&#x2715;` 两种数字实体（同一类坑） */
const ENTITY = /&#(?:\d+|[xX][0-9A-Fa-f]+);/g;

/**
 * 剥离注释，但**尊重字符串字面量** —— 字符串里的 `//`（如 URL）不是注释起点。
 * 名字把两半语义都写出来：只剥注释，且不把字符串内容当注释。
 * 返回剔除注释后的文本，供实体扫描使用。
 */
function stripCommentsRespectingStrings(code) {
  let out = '';
  let i = 0;
  let state = 'code'; // code | sq | dq | bq | line | block
  while (i < code.length) {
    const ch = code[i];
    const next = code[i + 1];
    if (state === 'code') {
      if (ch === '/' && next === '/') { state = 'line'; i += 2; continue; }
      if (ch === '/' && next === '*') { state = 'block'; i += 2; continue; }
      if (ch === "'") { state = 'sq'; out += ch; i += 1; continue; }
      if (ch === '"') { state = 'dq'; out += ch; i += 1; continue; }
      if (ch === '`') { state = 'bq'; out += ch; i += 1; continue; }
      out += ch; i += 1; continue;
    }
    if (state === 'line') {
      if (ch === '\n') { state = 'code'; out += ch; }
      i += 1; continue;
    }
    if (state === 'block') {
      if (ch === '*' && next === '/') { state = 'code'; i += 2; continue; }
      if (ch === '\n') out += ch; // 保行数，便于报位置
      i += 1; continue;
    }
    // 字符串内：处理转义，遇同类引号收尾
    if (ch === '\\') { out += ch + (next === undefined ? '' : next); i += 2; continue; }
    if ((state === 'sq' && ch === "'") || (state === 'dq' && ch === '"') || (state === 'bq' && ch === '`')) {
      state = 'code'; out += ch; i += 1; continue;
    }
    out += ch; i += 1;
  }
  return out;
}

/** 抽取所有 `<script …>…</script>` 块，带**在源码里的绝对起始偏移**（报行号要用文件真行号） */
function scriptBlocksOf(source) {
  const out = [];
  const re = /<script[^>]*>([\s\S]*?)<\/script>/g;
  let m;
  while ((m = re.exec(source)) !== null) {
    const inner = m[1];
    // m[0] = 开标签 + inner + '</script>' ⇒ inner 的绝对起点由此反推
    const innerStart = m.index + m[0].length - '</script>'.length - inner.length;
    out.push({ inner, innerStart });
  }
  return out;
}

/** 纯函数：单个 .uvue 源码 → 违规清单 */
function scanEntityLiterals(source) {
  const violations = [];
  for (const { inner, innerStart } of scriptBlocksOf(source)) {
    const code = stripCommentsRespectingStrings(inner);
    // 剥离注释会**移走字符**但**保留换行** ⇒ 行号必须按「换行计数」算，
    // 不能拿剥离后文本的字符偏移去索引原文（会偏，实测偏 2–3 行）。
    const linesBeforeBlock = source.slice(0, innerStart).split('\n').length - 1;
    let m;
    ENTITY.lastIndex = 0;
    while ((m = ENTITY.exec(code)) !== null) {
      const line = linesBeforeBlock + code.slice(0, m.index).split('\n').length;
      violations.push('`<script>` 块内有实体字面量 ' + m[0] + '（文件第 ' + line + ' 行）—— 不会被解码，会原样上屏；请直接写码点字符');
    }
  }
  return violations;
}

/**
 * 纯函数：一个图标值是否是**真实图形码点**。
 * ③ 是否证（没有 `&#…;`），这条是正向（确实是图形字符）—— 两半合起来才钉得住「屏上是图形」。
 * 只要求「单码点 + 非 ASCII」，**不钉死具体数值**：ADR 0015 ⑤ 允许真机证伪字体覆盖时替换 ⚙。
 */
function isGraphicIcon(s) {
  return typeof s === 'string' && [...s].length === 1 && s.codePointAt(0) > 0x7f;
}

/** 递归收集目标 .uvue（跳过构建产物与依赖） */
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
      if (e.name === 'unpackage' || e.name === 'node_modules') continue;
      collectUvue(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

describe('uvue `<script>` 实体字面量契约（#957 编译产物实测口径）', () => {
  // ---- 注入自检：检测器必须能抓住违规，否则本测试是空跑假绿 ----
  describe('① 检测器自检（注入违规样本必须被判红）', () => {
    const cases = [
      ['十进制实体进数据字符串（#957 原样写法）',
        "<script setup lang=\"uts\">\nconst a = [{ icon: '&#10022;' }]\n</script>",
        /实体字面量 &#10022;/],
      ['十六进制数字实体（同一类坑）',
        "<script setup lang=\"uts\">\nconst a = '&#x2715;'\n</script>",
        /实体字面量 &#x2715;/],
      ['普通 <script>（非 setup）里的实体',
        "<script>\nexport default { data() { return { i: '&#9881;' } } }\n</script>",
        /实体字面量 &#9881;/],
      ['模板字符串里拼进去的实体',
        '<script setup lang="uts">\nconst a = `&#128206;`\n</script>',
        /实体字面量 &#128206;/],
      ['模板静态文本合规、但 script 里另有一处违规（混合样本）',
        "<template>\n<text>&#10005;</text>\n</template>\n<script setup lang=\"uts\">\nconst a = '&#10022;'\n</script>",
        /实体字面量 &#10022;/],
    ];
    cases.forEach(([name, src, pattern]) => {
      it(name, () => {
        const v = scanEntityLiterals(src);
        expect(v.length).toBeGreaterThan(0);
        expect(v.join('\n')).toMatch(pattern);
      });
    });
  });

  // ---- 自检的另一半：合规样本必须判绿，防误报 ----
  describe('② 检测器自检（合规样本必须判绿，防误报）', () => {
    const ok = [
      ['模板静态文本里的实体是**合法写法**（本仓现存 11 处，一直正常）',
        '<template>\n<text class="sheet-close">&#10005;</text>\n</template>\n<script setup lang="uts">\nconst a = 1\n</script>'],
      ['script 的 // 行注释里提到实体不算实现',
        "<script setup lang=\"uts\">\n// 别写 '&#10022;'，要写真实码点\nconst a = '✦'\n</script>"],
      ['script 的 /* */ 块注释里提到实体不算实现',
        "<script setup lang=\"uts\">\n/* 历史写法：'&#9881;' */\nconst a = '⚙'\n</script>"],
      ['字符串里的 URL 含 // ，不是注释起点（剥离器不得截断）',
        '<script setup lang="uts">\nconst u = \'https://example.com/a\'\nconst a = \'✦\'\n</script>'],
      ['修好之后的真实写法：数据里是码点字符',
        "<script setup lang=\"uts\">\nconst a = [{ icon: '✦' }, { icon: '⚙' }, { icon: '📎' }]\n</script>"],
      ['完全没有实体的组件',
        '<template>\n<text>选择模型</text>\n</template>\n<script setup lang="uts">\nconst a = 1\n</script>'],
    ];
    ok.forEach(([name, src]) => {
      it(name, () => {
        expect(scanEntityLiterals(src)).toEqual([]);
      });
    });
  });

  // ---- 正向判据的自检：降级样本必须判红、真实码点必须判绿 ----
  describe('②b 图形码点判据自检（isGraphicIcon）', () => {
    const rejected = [
      ['ASCII 降级（把 ✦ 误改成 x 的那类回归）', 'x'],
      ['空字符串', ''],
      ['仍是实体（③ 也该抓，这里再兜一层）', '&#10022;'],
      ['多字符（两个码点拼出来的）', '✦✦'],
    ];
    rejected.forEach(([name, icon]) => {
      it('判红：' + name, () => {
        expect(isGraphicIcon(icon)).toBe(false);
      });
    });

    const accepted = [
      ['本票实际使用的三个码点之一（✦ U+2726）', '✦'],
      ['代理对 emoji（📎 U+1F4CE，按码点算仍是 1 个）', '📎'],
      ['未在票面出现、但同为图形码点的替代品（ADR 0015 ⑤ 的应急路径）', '★'],
    ];
    accepted.forEach(([name, icon]) => {
      it('判绿：' + name, () => {
        expect(isGraphicIcon(icon)).toBe(true);
      });
    });
  });

  // ---- 对真实文件断言零命中 ----
  it('③ 全项目 .uvue 的 `<script>` 块内无任何实体字面量（模板静态文本不在此列）', () => {
    const files = [
      ...collectUvue(path.join(ROOT, 'pages')),
      ...collectUvue(path.join(ROOT, 'components')),
      path.join(ROOT, 'App.uvue'),
    ].filter((p) => { try { return fs.statSync(p).isFile(); } catch { return false; } });

    expect(files.length).toBeGreaterThan(50);   // 锚点自检：扫描面必须真的覆盖到，不许退化成空跑

    const offenders = [];
    for (const f of files) {
      const v = scanEntityLiterals(fs.readFileSync(f, 'utf8'));
      if (v.length) offenders.push(path.relative(ROOT, f) + '\n    ' + v.join('\n    '));
    }
    expect(offenders.join('\n')).toBe('');
  });

  it('④ 范围自检：模板静态文本里的实体必须仍被允许（不许把范围切大到判它违规）', () => {
    // 这条是防「修反了」的守卫：#957 的修法是改数据，不是把这 11 处静态写法一起删掉。
    const sheet = fs.readFileSync(
      path.join(ROOT, 'components/ai-chat/ai-chat-pro-sheet.uvue'), 'utf8');
    expect(sheet).toMatch(/<text class="sheet-close"[^>]*>&#10005;<\/text>/);
  });

  it('⑤ 正向自检：能力清单三行图标是**真实图形码点**（不是 ASCII 降级、也不是实体）', () => {
    // ③ 只否证 `&#…;`。若没人正向钉住「图标是图形字符」，后续会话把 '✦' 误改成 'x'
    // （或留空、或写成多个字符）测试仍全绿 —— 而它屏上同样是坏的。这条补上正向那一半。
    const sheet = fs.readFileSync(
      path.join(ROOT, 'components/ai-chat/ai-chat-pro-sheet.uvue'), 'utf8');
    const icons = [...sheet.matchAll(/\{\s*icon:\s*'([^']*)'/g)].map((m) => m[1]);

    expect(icons.length).toBe(3);                 // 锚点：能力清单仍是三项（不是空跑）
    for (const icon of icons) {
      expect(isGraphicIcon(icon)).toBe(true);     // 判据自检见 ②b（降级样本判红）
    }
    // **不钉死具体码点**：ADR 0015 ⑤ 允许在真机证伪字体覆盖时替换 ⚙（U+2699 全仓无先例；
    // ✦ / 📎 已有真机先例）。钉死数值会让那条应急路径变成改测试，反而弱化守护。
  });
});
