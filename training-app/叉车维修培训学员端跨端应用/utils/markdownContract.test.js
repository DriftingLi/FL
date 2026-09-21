/**
 * Markdown 渲染子集契约（ADR-0046 移动端列 / issue #905）
 *
 * 本票要买的两件事，各由一组用例守着：
 *   ① **章节正文的表格要渲染成表格**，而不是带竖线的原始文本（生产 28 篇章节里 24 篇含表格，
 *      这是矩阵里唯一那格「破坏可读性」的降级）；
 *   ② **子集口径与 ADR-0046 的矩阵对齐**：内容精选走三端交集（表格不解析、退回原始文本），
 *      公式两档都降级为源码且**原样可见**（行内标记剥离不许吃掉 `$a_1$` 的下划线）。
 *
 * 形态按仓库惯例（format.test.js / searchDisplay.test.js / pointsDisplay.test.js）：
 *   - `utils/markdown.uts` 是 UTS，Node 不能直接 import ⇒ **镜像实现**跑行为级用例；
 *   - 文末的「镜像同步」把 .uts 源码与镜像逐条对齐，防两份实现悄悄分叉；
 *   - 页面级契约直接断言 `.uvue` 源码：谁渲染表格、谁走交集子集。
 *
 * **表格的块形状是本票实测钉住的**：`MarkdownBlock` 加「自定义类型的数组字段」
 * （`rows : MarkdownTableRow[]`）会触发 UTS 编译期改名 —— 生成 `MarkdownTableRow__1` 而引用处仍是
 * `MarkdownTableRow`，④c 整模块 kotlinc 报 60 条 unresolved reference。故表格按**行**出块，
 * 只用既有扁平字段（`items` = 该行单元格、`level` = 1 表头 / 0 正文）；下面的用例把这条钉死，
 * 免得下一个人「顺手」把它改回更漂亮的嵌套类型。
 *
 * 跨仓文档（根 `docs/adr/ADR-0046-内容渲染口径.md` 与 `API.md`）**不 skip、fail-closed**：
 * 本仓是 monorepo，这两份文件缺席意味着「登记被删了」，静默跳过正是本票要防的假绿。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const DIR = __dirname;
const ROOT = path.join(DIR, '..');
const REPO_ROOT = path.join(DIR, '..', '..', '..');
const read = (rel) => readText(path.join(ROOT, rel));
const MD_SRC = read('utils/markdown.uts');
/** 去掉注释后的源码：结构性断言（块产生点数量 / 字段形状）只看代码，不看解释性注释 */
const stripComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');
const MD_CODE = stripComments(MD_SRC);

const TABLE = '| 故障码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |';

// ===== 镜像实现（与 utils/markdown.uts 保持一致）=====

const SUBSET_CHAPTER = 'chapter';
const SUBSET_FEATURED = 'featured';
const SUBSET_FORUM = 'forum';
const MEMBER_HEADING = 'heading';
const MEMBER_LIST = 'list';
const MEMBER_QUOTE = 'quote';
const MEMBER_CODE = 'code';
const MEMBER_DIVIDER = 'divider';
const MEMBER_IMAGE = 'image';
const MEMBER_TABLE = 'table';
const SUBSET_MEMBERS_CHAPTER = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_IMAGE, MEMBER_TABLE];
const SUBSET_MEMBERS_FEATURED = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_IMAGE];
const SUBSET_MEMBERS_FORUM = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER];
/** 镜像 `subsetMembers`：未知档位退到章节档（与 parseMarkdown 的缺省档同向） */
function subsetMembers(subset) {
  if (subset === SUBSET_FEATURED) return SUBSET_MEMBERS_FEATURED;
  if (subset === SUBSET_FORUM) return SUBSET_MEMBERS_FORUM;
  return SUBSET_MEMBERS_CHAPTER;
}
const TABLE_LEVEL_HEAD = 1;
const TABLE_LEVEL_BODY = 0;
const ESCAPED_PIPE = '\u0001';

function trim(s) {
  return s.replace(/^\s+|\s+$/g, '');
}

function stripInlinePlain(text) {
  let s = text;
  s = s.replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1');
  s = s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1');
  s = s.replace(/\*\*([^\*]+)\*\*/g, '$1');
  s = s.replace(/__([^_]+)__/g, '$1');
  s = s.replace(/\*([^\*]+)\*/g, '$1');
  s = s.replace(/_([^_]+)_/g, '$1');
  s = s.replace(/`([^`]+)`/g, '$1');
  return s;
}

function dollarRun(text, from) {
  let n = 0;
  while (from + n < text.length && text.substring(from + n, from + n + 1) === '$') n++;
  return n;
}

function collectMathSpans(text) {
  const spans = [];
  let i = 0;
  while (i < text.length) {
    if (text.substring(i, i + 1) !== '$') {
      i++;
      continue;
    }
    const run = dollarRun(text, i);
    const want = run >= 2 ? 2 : 1;
    let cursor = i + want;
    let close = -1;
    while (cursor < text.length) {
      if (text.substring(cursor, cursor + 1) !== '$') {
        cursor++;
        continue;
      }
      const closeRun = dollarRun(text, cursor);
      if (closeRun >= want) {
        close = cursor;
        break;
      }
      cursor = cursor + closeRun;
    }
    if (close > i + want) {
      spans.push(i);
      spans.push(close + want);
      i = close + want;
    } else {
      i = i + want;
    }
  }
  return spans;
}

function stripInline(text) {
  const spans = collectMathSpans(text);
  if (spans.length === 0) return stripInlinePlain(text);
  let out = '';
  let cursor = 0;
  for (let i = 0; i + 1 < spans.length; i += 2) {
    const start = spans[i];
    const end = spans[i + 1];
    out += stripInlinePlain(text.substring(cursor, start));
    out += text.substring(start, end);
    cursor = end;
  }
  out += stripInlinePlain(text.substring(cursor));
  return out;
}

function hasTablePipe(line) {
  return line.indexOf('|') >= 0;
}

function splitTableRowRaw(line) {
  let s = trim(line);
  if (s.startsWith('|')) s = s.substring(1);
  if (s.endsWith('|')) s = s.substring(0, s.length - 1);
  const guarded = s.replace(/\\\|/g, ESCAPED_PIPE);
  return guarded.split('|').map((c) => trim(c.split(ESCAPED_PIPE).join('|')));
}

function isTableDelimiterRow(line) {
  if (!hasTablePipe(line)) return false;
  const cells = splitTableRowRaw(line);
  if (cells.length === 0) return false;
  return cells.every((c) => /^:?-+:?$/.test(c));
}

function normalizeTableRow(raw, width) {
  const cells = [];
  for (let i = 0; i < width; i++) cells.push(i < raw.length ? stripInline(raw[i]) : '');
  return cells;
}

function parseMarkdown(markdown, subset = SUBSET_CHAPTER) {
  const blocks = [];
  if (markdown.length === 0) return blocks;
  // 镜像的**范围**（写实，不假装全量）：chapter / featured 两档的解析语义。
  // 这两档把除 table 之外的成员**全都声明了**，故逐点闸门里只有 table 这一处会改变结果；
  // 「声明 ↔ 行为」的全量对账跑**真模块**（`utils/markdownSubsetBehavior.test.js`，含成对取证），
  // 本文件的镜像只保证既有两档的行为没被改造动过。
  const members = subsetMembers(subset);
  const hasMember = (m) => members.indexOf(m) >= 0;
  const lines = markdown.split('\n');
  const push = (o) => blocks.push(Object.assign({ type: '', level: 0, text: '', items: [] }, o));
  const pushTableRow = (cells, isHead) =>
    push({ type: 'table', level: isHead ? TABLE_LEVEL_HEAD : TABLE_LEVEL_BODY, text: cells.join(' '), items: cells });
  let i = 0;
  let inCodeBlock = false;
  let codeLines = [];
  let listItems = [];
  let listType = '';
  let quoteLines = [];

  function flushList() {
    if (listItems.length > 0) {
      push({ type: 'list', text: '', items: listItems });
      listItems = [];
      listType = '';
    }
  }
  function flushQuote() {
    if (quoteLines.length > 0) {
      push({ type: 'quote', text: quoteLines.join('\n') });
      quoteLines = [];
    }
  }

  while (i < lines.length) {
    const rawLine = lines[i];
    const line = trim(rawLine);

    if (line.startsWith('```')) {
      if (inCodeBlock) {
        push({ type: 'code', text: codeLines.join('\n') });
        codeLines = [];
        inCodeBlock = false;
      } else {
        flushList();
        flushQuote();
        inCodeBlock = true;
      }
      i++;
      continue;
    }
    if (inCodeBlock) {
      codeLines.push(rawLine);
      i++;
      continue;
    }
    if (line.length === 0) {
      flushList();
      flushQuote();
      i++;
      continue;
    }
    if (line.startsWith('$$')) {
      flushList();
      flushQuote();
      const formulaLines = [rawLine];
      i++;
      if (!(line.length > 4 && line.endsWith('$$'))) {
        while (i < lines.length) {
          const nextLine = lines[i];
          formulaLines.push(nextLine);
          i++;
          const closing = trim(nextLine);
          if (closing === '$$' || closing.endsWith('$$')) break;
        }
      }
      push({ type: 'paragraph', text: formulaLines.join('\n') });
      continue;
    }
    if (hasMember(MEMBER_TABLE) && hasTablePipe(line) && !isTableDelimiterRow(line)
      && i + 1 < lines.length && isTableDelimiterRow(trim(lines[i + 1]))) {
      flushList();
      flushQuote();
      const headerRaw = splitTableRowRaw(line);
      const width = headerRaw.length;
      pushTableRow(normalizeTableRow(headerRaw, width), true);
      i = i + 2;
      while (i < lines.length) {
        const bodyLine = trim(lines[i]);
        if (bodyLine.length === 0) break;
        if (!hasTablePipe(bodyLine)) break;
        if (isTableDelimiterRow(bodyLine)) break;
        pushTableRow(normalizeTableRow(splitTableRowRaw(bodyLine), width), false);
        i++;
      }
      continue;
    }
    if (line === '---' || line === '***' || line === '___') {
      flushList();
      flushQuote();
      push({ type: 'divider' });
      i++;
      continue;
    }
    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch != null && headingMatch.length >= 3) {
      flushList();
      flushQuote();
      push({ type: 'heading', level: headingMatch[1].length, text: stripInline(headingMatch[2] || '') });
      i++;
      continue;
    }
    if (line.startsWith('>')) {
      flushList();
      quoteLines.push(stripInline(trim(line.substring(1))));
      i++;
      continue;
    }
    const ulMatch = line.match(/^[-*+]\s+(.*)$/);
    if (ulMatch != null && ulMatch.length >= 2) {
      flushQuote();
      if (listType !== 'ul') {
        flushList();
        listType = 'ul';
      }
      listItems.push(stripInline(ulMatch[1] || ''));
      i++;
      continue;
    }
    const olMatch = line.match(/^\d+\.\s+(.*)$/);
    if (olMatch != null && olMatch.length >= 2) {
      flushQuote();
      if (listType !== 'ol') {
        flushList();
        listType = 'ol';
      }
      listItems.push(stripInline(olMatch[1] || ''));
      i++;
      continue;
    }
    const imgMatch = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
    if (imgMatch != null && imgMatch.length >= 3) {
      flushList();
      flushQuote();
      push({ type: 'image', text: imgMatch[2] || '', items: [imgMatch[1] || ''] });
      i++;
      continue;
    }
    flushList();
    flushQuote();
    push({ type: 'paragraph', text: stripInline(line) });
    i++;
  }

  if (inCodeBlock && codeLines.length > 0) push({ type: 'code', text: codeLines.join('\n') });
  flushList();
  flushQuote();
  return blocks;
}

const types = (blocks) => blocks.map((b) => b.type);
const firstText = (md, subset) => parseMarkdown(md, subset)[0].text;

// ===== ① 章节档：表格渲染 =====

describe('章节档（可信面全集）：表格按行解析成 table 块', () => {
  it('GFM 表格 → 表头行 + 正文行各一个 table 块，单元格在 items 里，text 不含竖线', () => {
    const blocks = parseMarkdown(TABLE);
    expect(types(blocks)).toEqual(['table', 'table']);
    expect(blocks.map((b) => b.level)).toEqual([TABLE_LEVEL_HEAD, TABLE_LEVEL_BODY]);
    expect(blocks[0].items).toEqual(['故障码', '含义']);
    expect(blocks[1].items).toEqual(['E01', '电压过低']);
    // 纯文本投影供搜索片段与兜底使用，不能带竖线
    expect(blocks.map((b) => b.text)).toEqual(['故障码 含义', 'E01 电压过低']);
  });

  it('单元格里的行内标记被剥成纯文本（与其余块同口径）', () => {
    const blocks = parseMarkdown('| **故障** | [手册](https://e.com/d) |\n| --- | --- |\n| `E01` | 电压 |');
    expect(blocks[0].items).toEqual(['故障', '手册']);
    expect(blocks[1].items).toEqual(['E01', '电压']);
  });

  it('列数按表头归一：不足补空串、超出截断（等宽单元格放不下多出的格子）', () => {
    const blocks = parseMarkdown('| A | B | C |\n| --- | --- | --- |\n| 1 |\n| 1 | 2 | 3 | 4 |');
    expect(blocks[1].items).toEqual(['1', '', '']);
    expect(blocks[2].items).toEqual(['1', '2', '3']);
  });

  it('`\\|` 转义不切列', () => {
    const blocks = parseMarkdown('| A | B |\n| --- | --- |\n| x \\| y | z |');
    expect(blocks[1].items).toEqual(['x | y', 'z']);
  });

  it('对齐分隔行（:--- / ---:）照样成表', () => {
    const blocks = parseMarkdown('| A | B |\n| :--- | ---: |\n| 1 | 2 |');
    expect(types(blocks)).toEqual(['table', 'table']);
  });

  it('表头恒为第一行：只有第一块是 level=1', () => {
    const blocks = parseMarkdown('| A | B |\n| --- | --- |\n| 1 | 2 |\n| 3 | 4 |');
    expect(blocks.map((b) => b.level)).toEqual([1, 0, 0]);
  });

  it('表格与相邻块互不粘连（标题 / 表格行 / 列表 各归各块）', () => {
    const blocks = parseMarkdown('# 标题\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n- 项一\n- 项二\n');
    expect(types(blocks)).toEqual(['heading', 'table', 'table', 'list']);
  });

  it('孤立的 `| --- |` 不成表（表头行本身不能是分隔行）', () => {
    const blocks = parseMarkdown('| --- | --- |\n| --- | --- |');
    expect(types(blocks)).toEqual(['paragraph', 'paragraph']);
  });

  it('代码块里的表格语法仍是代码（不被表格分支抢走）', () => {
    const blocks = parseMarkdown('```\n| A | B |\n| --- | --- |\n```');
    expect(types(blocks)).toEqual(['code']);
  });
});

// ===== ② 内容精选档：三端交集 =====

describe('内容精选档（三端交集）：表格与公式都不渲染', () => {
  it('表格不变成 table 块，退回逐行原始文本（与 Web featuredMarked 同口径）', () => {
    const blocks = parseMarkdown(TABLE, SUBSET_FEATURED);
    expect(types(blocks)).toEqual(['paragraph', 'paragraph', 'paragraph']);
    expect(blocks.some((b) => b.type === 'table')).toBe(false);
    expect(blocks[0].text).toBe('| 故障码 | 含义 |');
    expect(blocks[2].text).toBe('| E01 | 电压过低 |');
  });

  it('子集内的语法照常成块（标题 / 列表 / 引用 / 代码块 / 图片）', () => {
    const blocks = parseMarkdown(
      '# 标题\n\n- 项\n\n> 引用\n\n```js\nconst a = 1\n```\n\n![图](https://e.com/a.png)',
      SUBSET_FEATURED
    );
    expect(types(blocks)).toEqual(['heading', 'list', 'quote', 'code', 'image']);
  });

  it('公式在交集档同样降级为源码', () => {
    expect(firstText('扭矩 $T = 9550P/n$', SUBSET_FEATURED)).toBe('扭矩 $T = 9550P/n$');
  });
});

// ===== ③ 公式：降级为源码（两档一致）=====

describe('公式降级为源码：原样可见，不被行内规则吃掉', () => {
  it('行内公式连定界符一起保留', () => {
    expect(firstText('扭矩 $T = 9550P/n$ 时按此取值')).toBe('扭矩 $T = 9550P/n$ 时按此取值');
  });

  it('含下标的公式不被斜体规则 `_..._` 撕掉下划线（本票修掉的真实缺陷）', () => {
    expect(firstText('公式 $a_1 + b_2$ 里的下标')).toBe('公式 $a_1 + b_2$ 里的下标');
  });

  it('`$T$与文字`（右侧无空白）也保留', () => {
    expect(firstText('$T$与文字')).toBe('$T$与文字');
  });

  it('公式之外的行内标记仍被剥离（保护范围只到十字路口）', () => {
    expect(firstText('**粗体** $a_1$')).toBe('粗体 $a_1$');
  });

  it('块级 `$$` 多行公式收成一个块，不拆成三段', () => {
    const blocks = parseMarkdown('$$\nE = mc^2\n$$');
    expect(types(blocks)).toEqual(['paragraph']);
    expect(blocks[0].text).toBe('$$\nE = mc^2\n$$');
  });

  it('单行 `$$E=mc^2$$` 同样原样保留', () => {
    expect(firstText('$$E = mc^2$$')).toBe('$$E = mc^2$$');
  });

  it('未闭合的 `$` 不崩、原文不变', () => {
    expect(firstText('价格 $5 元的说明')).toBe('价格 $5 元的说明');
  });

  it('货币串的宽进口径：误判的后果也只是「保留原文」', () => {
    expect(firstText('价格 $5 到 $10 元')).toBe('价格 $5 到 $10 元');
  });

  it('代码块里的 `$` 不受公式逻辑影响', () => {
    const blocks = parseMarkdown('```\n$a_1$\n```');
    expect(types(blocks)).toEqual(['code']);
    expect(blocks[0].text).toBe('$a_1$');
  });
});

// ===== ④ 镜像同步：.uts ↔ 本文件的镜像 =====

describe('镜像同步：utils/markdown.uts 与本文件镜像逐条一致', () => {
  it('子集档位常量、表格 level 常量与解析签名在 .uts 内成文', () => {
    expect(MD_SRC).toContain("export const SUBSET_CHAPTER = 'chapter'");
    expect(MD_SRC).toContain("export const SUBSET_FEATURED = 'featured'");
    expect(MD_SRC).toContain('export const TABLE_LEVEL_HEAD = 1');
    expect(MD_SRC).toContain('export const TABLE_LEVEL_BODY = 0');
    expect(MD_SRC).toContain(
      'export function parseMarkdown(markdown : string, subset : string = SUBSET_CHAPTER) : MarkdownBlock[]'
    );
  });

  it('表格块只有一个产生点（pushTableRow），且被**声明表**闸住（论坛与精选两档都不声明 table）', () => {
    expect((MD_CODE.match(/type: 'table'/g) || []).length).toBe(1);
    expect(MD_SRC).toContain('function pushTableRow(cells : string[], isHead : boolean)');
    expect(MD_SRC).toContain('if (hasMember(MEMBER_TABLE) && hasTablePipe(line) && !isTableDelimiterRow(line)');
    expect(MD_SRC).toContain('function isTableDelimiterRow(line : string) : boolean');
    expect(MD_SRC).toContain('cells[i].match(/^:?-+:?$/) == null');
  });

  it('子集声明表在 .uts 内成文，且七个可开关成员各有一处闸门（声明即行为）', () => {
    expect(MD_SRC).toContain("export const SUBSET_FORUM = 'forum'");
    expect(MD_SRC).toContain('export function subsetMembers(subset : string) : string[]');
    expect(MD_SRC).toContain('export function subsetHasMember(subset : string, member : string) : boolean');
    for (const name of ['CHAPTER', 'FEATURED', 'FORUM']) {
      expect(MD_SRC).toContain(`export const SUBSET_MEMBERS_${name} : string[] = [`);
    }
    // 每个成员都必须真的被解析器问过；段落**不在**声明表里（它是降级可读的地板，不设闸门）
    // 注意这里遍历的是**常量名**（不是它们的值）：断言的是 .uts 源码里的闸门写法
    const memberNames = ['MEMBER_HEADING', 'MEMBER_LIST', 'MEMBER_QUOTE', 'MEMBER_CODE', 'MEMBER_DIVIDER', 'MEMBER_IMAGE', 'MEMBER_TABLE'];
    for (const member of memberNames) {
      expect([member, MD_CODE.includes(`hasMember(${member})`)]).toEqual([member, true]);
    }
    expect(MD_CODE).not.toContain('MEMBER_PARAGRAPH');
  });

  it('公式保护在 stripInline 内、纯剥离仍走 stripInlinePlain', () => {
    expect(MD_SRC).toContain('function stripInlinePlain(text : string) : string');
    expect(MD_SRC).toContain('const spans = collectMathSpans(text)');
    expect(MD_SRC).toContain('function collectMathSpans(text : string) : number[]');
  });

  it('不加「自定义类型的数组字段」：UTS 会改名并让整模块编译红（本票实测）', () => {
    for (const rel of ['utils/markdown.uts', 'types/common.uts']) {
      const code = stripComments(read(rel));
      expect(code).not.toContain('MarkdownTableRow');
      expect(code).not.toMatch(/\brows\s*:/);
    }
    // 表格数据走既有扁平字段
    expect(MD_CODE).toContain('items: cells,');
  });
});

// ===== ⑤ 页面级契约：谁渲染表格、谁走交集 =====

describe('页面级契约：章节面渲染表格、内容精选面走交集', () => {
  const chapter = read('pages/courses/chapter-view.uvue');
  // T08 手术（#646）：章节正文的渲染分支搬进模块私有 section 组件，因此「表格怎么渲染」这几条
  // 的**路径指向**随之改指该组件 —— 断言一条未删、未弱化（ADR-0007：「手术 PR 内可更新文件路径
  // 指向，禁删/弱化断言」）；「走哪一档」仍钉在页面（档位是页面侧决策，组件只渲染下发的块）。
  const chapterMd = read('pages/courses/components/chapter-markdown.uvue');
  const featured = read('pages/featured/featured-detail.uvue');

  it('chapter-view：table 块渲染成逐行 flex 的原生表格（单元格取自 items）', () => {
    expect(chapterMd).toContain("block.type == 'table'");
    expect(chapterMd).toContain('v-for="(cell, ci) in block.items"');
    expect(chapterMd).toContain("'md-table-cell-head': block.level == 1");
    expect(chapterMd).toContain('.md-table-cell-head');
    // 表格行必须归零 .md-block 的块间距，否则同一张表会被拆成一格一格的横条
    expect(chapterMd).toMatch(/\.md-table-row\s*\{[^}]*margin-bottom:\s*0/);
  });

  it('chapter-view：走缺省档（章节正文 = 可信面全集）', () => {
    expect(chapter).toContain('parseMarkdown(detail.value!.content)');
  });

  it('featured-detail：固定 featured 档，且不持有表格渲染分支', () => {
    expect(featured).toContain('parseMarkdown(detail.value!.content, SUBSET_FEATURED)');
    expect(featured).not.toContain("block.type == 'table'");
    expect(featured).not.toContain('md-table');
  });

  it('搜索投影不另建一套表格口径（吃 table 块的 text 投影）', () => {
    const search = read('utils/searchDisplay.uts');
    expect(search).not.toContain("== 'table'");
    expect(parseMarkdown(TABLE)[0].text.length).toBeGreaterThan(0);
  });
});

// ===== ⑥ 跨仓登记：ADR-0046 矩阵与 API.md 的移动端口径 =====

describe('ADR-0046 / API.md 的移动端登记（本票的口径落点）', () => {
  const ADR_REL = path.join(REPO_ROOT, 'docs', 'adr', 'ADR-0046-内容渲染口径.md');
  const API_REL = path.join(REPO_ROOT, 'API.md');
  const adr = readText(ADR_REL);
  const api = readText(API_REL);

  it('矩阵里移动端「表格」格已从破口改为补齐，并写清只补哪一面', () => {
    expect(adr).not.toContain('无（线上破口）');
    expect(adr).toContain('**本批补齐**（章节面渲染成表格；内容精选面按三端交集降级为原始文本）');
  });

  it('矩阵里移动端「公式」格登记为降级为源码（仍可读）', () => {
    expect(adr).toContain('降级为源码（`$...$` / `$$...$$` 原样可见，仍可读）');
  });

  it('决定 6 写清移动端两档子集与「缺什么、降级成什么」', () => {
    expect(adr).toContain('### 6. 移动端的渲染子集');
    expect(adr).toContain('退回逐行原始文本');
    expect(adr).toContain('公式的降级表现写死为「源码原样可见」');
    expect(adr).toContain('**明确不做**：数学排版（原生端无 DOM/KaTeX）');
  });

  it('决定 6 记下「表格按行出块」与 UTS 改名的坑位', () => {
    expect(adr).toContain('按行出块');
    expect(adr).toContain('MarkdownTableRow__1');
  });

  it('决定 6 登记本票的覆盖面边界（论坛 / AI 面仍纯文本，不走本解析器）', () => {
    expect(adr).toContain('论坛正文与 AI 助手回答仍是纯文本渲染');
    // 边界是真的：两个面都不 import 本解析器
    for (const rel of ['pages/forum/components/forum-topic-body.uvue', 'components/ai-chat/ai-chat-bubble.uvue']) {
      expect(read(rel)).not.toContain('utils/markdown');
    }
  });

  it('已知限制里的移动端一行已随本票更新（旧口径不再残留）', () => {
    expect(adr).not.toContain('表格与公式都不渲染');
    expect(adr).toContain('章节面的表格已渲染成原生表格');
  });

  it('API.md 章节段的移动端口径同步，且不再挂「另立 issue」', () => {
    expect(api).not.toContain('移动端的表格渲染缺口另立 issue');
    expect(api).toContain('移动端**按内容面分子集（ADR-0046 决定 6）');
  });

  it('内容精选的三端交集契约没被本票放宽（表格与公式仍不在子集内）', () => {
    expect(api).toContain('**不含表格与公式**');
  });
});
