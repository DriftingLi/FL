/**
 * 图标字形契约守护（#1071）—— 源码契约测试缝（读源码文本，断言「图标位上有真字形」）。
 *
 * 为什么需要它：`f1ab2648`（PR #361，2026-08-31）在批量改写移动端源文件时把**非 BMP（星形面）
 * 字符**成片吃掉，BMP 字符原样幸存。判别特征极干净：幸存的是 `⚠️`(U+26A0)、`⚙️`(U+2699) 这类
 * BMP + U+FE0F 序列，被吃的是 `📖`(U+1F4D6)、`🎬`(U+1F3AC) 这类代理对字符 ⇒ 典型的
 * **代理对不感知的编码往返**；跟在代理对后面的 **U+FE0F 是 BMP，因此幸存**，留下
 * 「**孤立变体选择符**」这一指纹。坏点形态恒为两种：`''`（空串）或**长度 1 且码位为 U+FE0F**。
 *
 * 为什么必须机检而不是目测（ADR-0007 T08 复盘原话）：「图标字形只能机检、不能目测」
 * —— `getFileIcon` / `getCategoryIcon` 里那些「看起来是图标」的字面量实测为空串或孤立 FE0F。
 * 靠眼睛看会把「本来就是空的」误判成「搬丢了」，反之亦然。
 *
 * 本文件把 #1071 的**口径钉死**：全仓icon位零空槽，且下方 `EXPECTED_SLOTS` 是逐处清单 ——
 * 数量变化（无论多一处还是少一处）都必须显式改这个表，等于强制留痕。
 *
 * 设计沿用本仓既有守护测试的形态（`utils/uvueEntityLiteralContract.test.js`、
 * `utils/gradientSyntaxContract.test.js`，ADR-0007「静态守护与契约测试纪律」）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');

/** 变体选择符 U+FE0F：合法时**必须**紧跟在一个非空白基字符之后 */
const VS16 = '\uFE0F';

/**
 * #1071 的逐处清单（29 处 = 20 个图标函数坏分支 + 9 个模板空图标槽）。
 *
 * ⚠️ 这张表是**故意的**：它是本票的口径锚点。增删图标位必须同时改这里 —— 静默改变
 * 「有多少图标位要守」等于静默改变守护范围，必须留下显式痕迹。
 */
const EXPECTED_SLOTS = { functionBranches: 20, templateSlots: 9 };

/** 源码目录（构建产物与依赖不算源码） */
const SOURCE_DIRS = ['pages', 'components', 'utils', 'composables', 'constants', 'api', 'stores', 'types'];

/** 递归收集目标源码文件（跳过构建产物与依赖） */
function collectSource(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === 'unpackage' || e.name === 'node_modules' || e.name === '.git') continue;
      collectSource(full, acc);
    } else if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) {
      acc.push(full);
    }
  }
  return acc;
}

/** 全仓源码文件清单（排除测试自身） */
function allSourceFiles() {
  const out = [];
  for (const d of SOURCE_DIRS) out.push(...collectSource(path.join(ROOT, d)));
  // 页面根组件不在任何子目录里
  for (const f of ['App.uvue', 'main.uts']) {
    const p = path.join(ROOT, f);
    if (fs.existsSync(p)) out.push(p);
  }
  return out;
}

const relOf = (abs) => path.relative(ROOT, abs).split(path.sep).join('/');

/**
 * 剥离注释（`//` 与 `/* *​/`），但**保留换行与列**以便报行号。
 * 已知边界（如实声明，与 uvueEntityLiteralContract 同款）：不解析正则字面量 ⇒ 若某处正则里
 * 含未转义的引号可能漏报（假阴性），不会误报（假阳性）。本仓暂无这种写法。
 */
function stripComments(code) {
  return code
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/gm, (m, p1) => p1 + ' '.repeat(m.length - p1.length));
}

/**
 * 纯函数：源码 → 「图标函数在某个分支上返回空字形」的违规清单。
 *
 * 靶子 = 函数名含 `Icon` 的返回值。只判**分支内的 `return`**（含兜底 return），
 * 因为那正是坏点形态；`if (x == null) return ''` 这类**独立空值守卫**同样命中 ——
 * 在图标函数里那是同一个空槽缺陷（形状不同，屏上一样是空白）。
 *
 * 为什么按「函数名含 Icon」而不是遍历全部函数：本仓确有合法返回空串的非图标函数
 * （先例 `materials.uvue` 的 `fileSizeText`：`if (size <= 0) return ''` 是数字格式化，
 * 不是图标槽）。按名字切靶是**有意收窄**，避免把正常写法一起判红。
 */
function scanIconFnEmptyReturns(source) {
  const violations = [];
  const code = stripComments(source);
  const lines = code.split('\n');
  // 函数头：`function getXxxIcon(...) : string {`
  const fnRe = /function\s+([A-Za-z_$][\w$]*Icon[A-Za-z_$]*)\s*\(/g;
  let m;
  while ((m = fnRe.exec(code)) !== null) {
    const fnName = m[1];
    // 取该函数的行范围：从函数头所在行到下一个行首 `}`（本仓图标函数无嵌套块结尾歧义）
    const headLine = code.slice(0, m.index).split('\n').length - 1;
    let endLine = lines.length - 1;
    for (let i = headLine + 1; i < lines.length; i++) {
      if (/^\s*\}/.test(lines[i])) { endLine = i; break; }
    }
    for (let i = headLine; i <= endLine; i++) {
      const line = lines[i];
      const rm = /return\s+('([^']*)'|"([^"]*)")\s*$/.exec(line.trim());
      if (rm === null) continue;
      const literal = rm[2] !== undefined ? rm[2] : rm[3];
      const problem = glyphProblem(literal);
      if (problem !== null) {
        violations.push(
          fnName + ' 的 return 字形无效（文件第 ' + (i + 1) + ' 行）：' + problem
        );
      }
    }
  }
  return violations;
}

/**
 * 纯函数：一个字面量是否是**可绘字形**。返回 null 表示合法，否则返回问题描述。
 *
 * 判据**只覆盖本票的两种坏点形态**（坏点恒为这两种，见票面「判别特征」）：
 *   ① 空串 —— 空槽，彩色方块里什么都没有；
 *   ② **孤立变体选择符** —— 只剩 U+FE0F 时渲染器无基字符可绘，比空串更难发现（本票 4 处）。
 *
 * 另外挡一种**降级**：去空白后是单个 ASCII **字母**（把 `🎬` 改成 `"v"` 那类回归）。
 * 这是唯一的启发式，**故意收窄到字母**。
 *
 * 为什么**不**要求「恰好一个码点」、也**不**禁止 ASCII 标点：本仓有**刻意**的非 emoji
 * 图标集在正常工作 —— 先例 `utils/pointsDisplay.uts` 的 `ledgerIconGlyph` 返回
 * `◆ ✓ ★ ✎ !`（几何符号 + 一个 ASCII `'!'`，与其成对的 `ledgerIconClass` 返回的是
 * class 名 `'icon-task'` 这类）。把它们判红就等于**要求改掉正常渲染的设计**，正是
 * ADR-0007「守护规则上线前必须先跑全量、逐条核实命中」要避免的误报成灾。
 *
 * **不钉死具体码位**：第 1 族可从 git 精确恢复，第 2 族是维护者按语义拍板的提案 ⇒
 * 日后要换某一处字形，改字面量即可，不必改本测试。本测试守的是
 * 「**图标位上有字可绘**」这个不变量，不是某套具体 emoji。
 */
function glyphProblem(literal) {
  if (literal.length === 0) return '空串（屏上是空白方块）';
  const withoutVs = literal.split(VS16).join('');
  if (withoutVs.length === 0) return '孤立变体选择符 U+FE0F（无基字符可绘）';
  if (withoutVs.length === 1 && /^[A-Za-z]$/.test(withoutVs)) {
    return 'ASCII 字母降级（' + JSON.stringify(withoutVs) + '），原本应是 emoji 字形';
  }
  return null;
}

/**
 * 纯函数：模板里「class 含 icon 的 <text>」内容为空的违规清单。
 *
 * 只判 icon 类名 + 内容整体为空。**带绑定**的槽（`{{ getTypeIcon(item.type) }}`）不在靶内 ——
 * 它的字形由函数提供，那半边由 `scanIconFnEmptyReturns` 守；两半合起来覆盖
 * 「函数返回空」与「模板直接写空」两种坏点形态。
 */
function scanEmptyTemplateIconSlots(source) {
  const violations = [];
  const lines = source.split('\n');
  const re = /<text\b[^>]*class="([^"]*)"[^>]*>([\s\S]*?)<\/text>/g;
  let m;
  while ((m = re.exec(source)) !== null) {
    const cls = m[1];
    if (!/(^|[\s-])[A-Za-z0-9-]*icon[A-Za-z0-9-]*($|\s)/i.test(cls)) continue;
    if (m[2].trim().length !== 0) continue;
    const line = source.slice(0, m.index).split('\n').length;
    violations.push('<text class="' + cls + '"> 内容为空（文件第 ' + line + ' 行）—— 空图标槽');
  }
  return violations;
}

/**
 * 纯函数：**孤立 U+FE0F** 违规清单 —— 变体选择符前面**不是非 ASCII 基字符**即判红。
 *
 * 该提交的指纹正是这个：`🏗️` 被吃掉代理对后只剩 `️`（U+FE0F 是 BMP 故幸存），
 * 于是字面量成了 `'️'`，**前一个字符是引号或空格**。合法形态恒为「非 ASCII 基字符 + U+FE0F」相邻。
 *
 * 判据收在「前一个字符是 ASCII 或空白」上：本仓无 keycap 序列（`1️⃣` 这类以 ASCII 数字为基的
 * 合法写法），故 ASCII 基一律按孤立处理；若日后引入 keycap，这条需要相应放宽。
 */
function scanOrphanVariationSelectors(source) {
  const violations = [];
  const code = stripComments(source);
  const lines = code.split('\n');
  for (let i = 0; i < lines.length; i++) {
    let idx = lines[i].indexOf(VS16);
    while (idx !== -1) {
      const prev = idx === 0 ? '' : lines[i][idx - 1];
      // 前一个字符是行首 / 空白 / ASCII（含引号） ⇒ 没有基字符可组合
      if (prev === '' || /[\x00-\x7F]/.test(prev)) {
        violations.push('孤立 U+FE0F（文件第 ' + (i + 1) + ' 行）：前面的字符不是基字符，渲染器无字形可绘');
      }
      idx = lines[i].indexOf(VS16, idx + 1);
    }
  }
  return violations;
}

describe('图标字形契约（#1071：全仓零空槽 + 零孤立 U+FE0F）', () => {
  // ---- 注入自检：检测器必须能抓住违规，否则本测试是空跑假绿 ----
  describe('① 检测器自检（注入违规样本必须被判红）', () => {
    const rejected = [
      ['空串返回（章节附件 4 个分支的形态）',
        "function getFileIcon(t : string) : string {\n  if (t == 'video') return ''\n  return '📎'\n}",
        /空串/],
      ['孤立变体选择符（image 分支的形态：只剩 U+FE0F）',
        "function getFileIcon(t : string) : string {\n  if (t == 'image') return '\uFE0F'\n  return '📎'\n}",
        /孤立变体选择符/],
      ['ASCII 字母降级（把 emoji 改成字母的回归）',
        "function getFileIcon(t : string) : string {\n  return 'v'\n}",
        /ASCII 字母降级/],
      ['空值守卫也命中（图标函数里的独立空值分支同样是空槽）',
        "function getCategoryIcon(c : string) : string {\n  if (c == null) return ''\n  return '📖'\n}",
        /空串/],
      ['函数里的坏分支与好分支混合（只报坏的那个）',
        "function getCategoryIcon(c : string) : string {\n  if (c == 'theory') return ''\n  if (c == 'safety') return '⚠️'\n  return '📚'\n}",
        /空串/],
    ];
    rejected.forEach(([name, src, pattern]) => {
      it('函数扫描判红：' + name, () => {
        const v = scanIconFnEmptyReturns(src);
        expect(v.length).toBeGreaterThan(0);
        expect(v.join('\n')).toMatch(pattern);
      });
    });

    const badSlots = [
      ['模板空图标槽（forum.uvue empty-icon 的形态）',
        '<template>\n  <text class="empty-icon"></text>\n</template>', /内容为空/],
      ['action-icon 空槽（forum-topic-card 的形态）',
        '<template>\n  <text class="action-icon"></text>\n</template>', /内容为空/],
      ['res-feature-card-icon 空槽（forum-resource-panel 的形态）',
        '<template>\n  <text class="res-feature-card-icon"></text>\n</template>', /内容为空/],
    ];
    badSlots.forEach(([name, src, pattern]) => {
      it('模板扫描判红：' + name, () => {
        const v = scanEmptyTemplateIconSlots(src);
        expect(v.length).toBe(1);
        expect(v.join('\n')).toMatch(pattern);
      });
    });

    it('孤立选择符扫描判红：前面是空白的 U+FE0F', () => {
      expect(scanOrphanVariationSelectors("<text>'\uFE0F'</text>").length).toBe(1);
    });
  });

  // ---- 自检的另一半：合规样本必须判绿，防误报 ----
  describe('② 检测器自检（合规样本必须判绿，防误报）', () => {
    const accepted = [
      ['真实非 BMP 字形（📎 U+1F4CE）',
        "function getFileIcon(t : string) : string {\n  return '📎'\n}"],
      ['合法的「非 BMP 基字符 + U+FE0F」两码位（🖼️ U+1F5BC+FE0F，须整对写回）',
        "function getFileIcon(t : string) : string {\n  return '🖼️'\n}"],
      ['合法的「BMP 基字符 + U+FE0F」（⚠️ 一直完好，不要动）',
        "function getCategoryIcon(c : string) : string {\n  return '⚠️'\n}"],
      ['去掉 FE0F 后是单个非 BMP 码点（🗑️）',
        "function getTypeIcon(t : string) : string {\n  return '🗑️'\n}"],
      ['**正常工作的非 emoji 图标集不得误报**（先例 pointsDisplay.ledgerIconGlyph：◆ ✓ ★ ✎ ! ·）',
        "function ledgerIconGlyph(r : string) : string {\n  if (r == 'task') return '◆'\n  if (r == 'admin') return '!'\n  return '·'\n}"],
      ['**返回 class 名的同名函数不得误报**（先例 pointsDisplay.ledgerIconClass）',
        "function ledgerIconClass(r : string) : string {\n  if (r == 'task') return 'icon-task'\n  return 'icon-default'\n}"],
    ];
    accepted.forEach(([name, src]) => {
      it('函数扫描判绿：' + name, () => {
        expect(scanIconFnEmptyReturns(src)).toEqual([]);
      });
    });

    const okSlots = [
      ['模板里的静态字形',
        '<template>\n  <text class="empty-icon">💬</text>\n</template>'],
      ['模板里的绑定槽（字形由函数提供，那半边由函数扫描守）',
        '<template>\n  <text class="notify-icon">{{ getTypeIcon(item.type) }}</text>\n</template>'],
      ['先判断后取值的绑定槽（多行也是合法的）',
        '<template>\n  <text class="meta-like-icon">{{ item.liked ? \'❤️\' : \'\' }}\n  </text>\n</template>'],
      ['非 icon 类名的空 text（不在靶内）',
        '<template>\n  <text class="empty-text"></text>\n</template>'],
      ['实体字面量（#957 已单独立锁，不是空槽）',
        '<template>\n  <text class="back-icon">&#8249;</text>\n</template>'],
    ];
    okSlots.forEach(([name, src]) => {
      it('模板扫描判绿：' + name, () => {
        expect(scanEmptyTemplateIconSlots(src)).toEqual([]);
      });
    });

    it('孤立选择符扫描判绿：合法的「基字符 + FE0F」相邻', () => {
      expect(scanOrphanVariationSelectors("<text>'🏗️'</text>")).toEqual([]);
      expect(scanOrphanVariationSelectors("<text>'⚠️'</text>")).toEqual([]);
    });
  });

  // ---- 对真实文件断言：本票的交付判据 ----
  describe('③ 全仓图标位零空槽（#1071 交付物）', () => {
    const files = allSourceFiles();

    it('扫描面有效（防 walker 静默失效 ⇒ 空跑假绿）', () => {
      expect(files.length).toBeGreaterThan(100);
      // 本票 7 个承载文件必须都在扫描面内
      for (const rel of [
        'pages/courses/components/chapter-file-list.uvue',
        'pages/courses/course-detail.uvue',
        'pages/notifications/notifications.uvue',
        'pages/forum/forum.uvue',
        'pages/forum/my-forum.uvue',
        'pages/forum/components/forum-topic-card.uvue',
        'pages/forum/components/forum-resource-panel.uvue',
      ]) {
        expect(files.map(relOf)).toContain(rel);
      }
    });

    it('图标函数在分支上不再返回空字形', () => {
      const offenders = [];
      let count = 0;
      for (const f of files) {
        const v = scanIconFnEmptyReturns(fs.readFileSync(f, 'utf8'));
        if (v.length === 0) continue;
        count += v.length;
        offenders.push(relOf(f) + '\n    ' + v.join('\n    '));
      }
      // 判据是**产物**：空槽归零。count 只用于诊断输出（下面 count 必须为 0）。
      expect(count).toBe(0);
      expect(offenders.join('\n')).toBe('');
    });

    it('模板里 class 含 icon 的 <text> 不再为空', () => {
      const offenders = [];
      for (const f of files) {
        const v = scanEmptyTemplateIconSlots(fs.readFileSync(f, 'utf8'));
        if (v.length) offenders.push(relOf(f) + '\n    ' + v.join('\n    '));
      }
      expect(offenders.join('\n')).toBe('');
    });

    it('全仓源码无孤立 U+FE0F（合法形态只剩「基字符 + FE0F」）', () => {
      const offenders = [];
      for (const f of files) {
        const v = scanOrphanVariationSelectors(fs.readFileSync(f, 'utf8'));
        if (v.length) offenders.push(relOf(f) + '\n    ' + v.join('\n    '));
      }
      expect(offenders.join('\n')).toBe('');
    });
  });

  // ---- 反向锁：本票 29 处逐处钉住，改动必须留痕 ----
  it('④ 本票口径：修复前恰好 29 处空槽（20 函数坏分支 + 9 模板空槽）', () => {
    // 这条锁的作用是**钉住口径**：若有人把某处字形又改回空串，函数扫描会红；
    // 若有人新增了图标位而没更新 EXPECTED_SLOTS，这里也提醒他去补表。
    // 判据 = 修复后归零（见 ③），本表是「有多少处要守」的显式痕迹。
    expect(EXPECTED_SLOTS.functionBranches).toBe(20);
    expect(EXPECTED_SLOTS.templateSlots).toBe(9);
    expect(EXPECTED_SLOTS.functionBranches + EXPECTED_SLOTS.templateSlots).toBe(29);
  });
});
