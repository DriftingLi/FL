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
const SUBSET_AI_ANSWER = 'ai-answer';
const MEMBER_HEADING = 'heading';
const MEMBER_LIST = 'list';
const MEMBER_QUOTE = 'quote';
const MEMBER_CODE = 'code';
const MEMBER_DIVIDER = 'divider';
const MEMBER_IMAGE = 'image';
const MEMBER_TABLE = 'table';
const MEMBER_INLINE = 'inline';
const SUBSET_MEMBERS_CHAPTER = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_IMAGE, MEMBER_TABLE];
const SUBSET_MEMBERS_FEATURED = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_IMAGE, MEMBER_INLINE];
const SUBSET_MEMBERS_FORUM = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_INLINE];
/** 移动端 ADR 0056 第四档：论坛档的文本面 + image、- inline */
const SUBSET_MEMBERS_AI_ANSWER = [MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, MEMBER_IMAGE];
/** 镜像 `subsetMembers`：未知档位退到章节档（与 parseMarkdown 的缺省档同向） */
function subsetMembers(subset) {
  if (subset === SUBSET_FEATURED) return SUBSET_MEMBERS_FEATURED;
  if (subset === SUBSET_FORUM) return SUBSET_MEMBERS_FORUM;
  if (subset === SUBSET_AI_ANSWER) return SUBSET_MEMBERS_AI_ANSWER;
  return SUBSET_MEMBERS_CHAPTER;
}
const TABLE_LEVEL_HEAD = 1;
const TABLE_LEVEL_BODY = 0;
const LIST_LEVEL_UL = 0;
const LIST_LEVEL_OL = 1;
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
  s = s.replace(/~~([^~]+)~~/g, '$1');
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

function stripInline(text, keepInline) {
  if (keepInline) return text;
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

function normalizeTableRow(raw, width, keepInline) {
  const cells = [];
  for (let i = 0; i < width; i++) cells.push(i < raw.length ? stripInline(raw[i], keepInline) : '');
  return cells;
}

function parseMarkdown(markdown, subset = SUBSET_CHAPTER) {
  const blocks = [];
  if (markdown.length === 0) return blocks;
  // 镜像的**范围**（写实，不假装全量）：三档的块语义。#1240 立它时只有 chapter / featured 两档，
  // #1472 起 forum 也带上了行内分叉（`keepInline`）—— 镜像必须跟着分叉，否则「论坛档到底交出什么」
  // 在本文件里是假的。这两档把除 table 之外的成员**全都声明了**，故逐点闸门里只有 table 这一处会改变结果；
  // 「声明 ↔ 行为」的全量对账跑**真模块**（`utils/markdownSubsetBehavior.test.js`，含成对取证），
  // 本文件的镜像只保证既有两档的行为没被改造动过 + 论坛档的分叉语义与 .uts 一致。
  const members = subsetMembers(subset);
  const hasMember = (m) => members.indexOf(m) >= 0;
  // ⑩-2 的分叉闸门（与 utils/markdown.uts 同源）：声明 inline ⇒ 块内文本存行内源串
  const keepInline = hasMember(MEMBER_INLINE);
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
      push({ type: 'list', level: (keepInline && listType === 'ol') ? LIST_LEVEL_OL : LIST_LEVEL_UL, text: '', items: listItems });
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
      pushTableRow(normalizeTableRow(headerRaw, width, keepInline), true);
      i = i + 2;
      while (i < lines.length) {
        const bodyLine = trim(lines[i]);
        if (bodyLine.length === 0) break;
        if (!hasTablePipe(bodyLine)) break;
        if (isTableDelimiterRow(bodyLine)) break;
        pushTableRow(normalizeTableRow(splitTableRowRaw(bodyLine), width, keepInline), false);
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
      push({ type: 'heading', level: headingMatch[1].length, text: stripInline(headingMatch[2] || '', keepInline) });
      i++;
      continue;
    }
    if (line.startsWith('>')) {
      flushList();
      quoteLines.push(stripInline(trim(line.substring(1)), keepInline));
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
      listItems.push(stripInline(ulMatch[1] || '', keepInline));
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
      listItems.push(stripInline(olMatch[1] || '', keepInline));
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
    push({ type: 'paragraph', text: stripInline(line, keepInline) });
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
    for (const name of ['CHAPTER', 'FEATURED', 'FORUM', 'AI_ANSWER']) {
      expect(MD_SRC).toContain(`export const SUBSET_MEMBERS_${name} : string[] = [`);
    }
    // 每个成员都必须真的被解析器问过；段落**不在**声明表里（它是降级可读的地板，不设闸门）
    // 注意这里遍历的是**常量名**（不是它们的值）：断言的是 .uts 源码里的闸门写法
    const memberNames = ['MEMBER_HEADING', 'MEMBER_LIST', 'MEMBER_QUOTE', 'MEMBER_CODE', 'MEMBER_DIVIDER', 'MEMBER_IMAGE', 'MEMBER_TABLE', 'MEMBER_INLINE'];
    for (const member of memberNames) {
      expect([member, MD_CODE.includes(`hasMember(${member})`)]).toEqual([member, true]);
    }
    expect(MD_CODE).not.toContain('MEMBER_PARAGRAPH');
  });

  it('⑩-2 的分叉闸门在 .uts 内成文，且镜像与它**同形**（声明表决定，不看档位名）', () => {
    // 分叉只由 hasMember(MEMBER_INLINE) 决定：这条锚点同时守两件事 ——
    //   ① 有人把 `keepInline` 改成按 `subset == SUBSET_FORUM` 判（档位名硬编码 = 第二处事实）；
    //   ② 有人改了 .uts 的分叉而镜像跟丢（镜像红 = 本文件的 forum 行为不再是真的）。
    expect(MD_CODE).toContain('const keepInline = hasMember(MEMBER_INLINE)');
    expect(MD_CODE).toContain('if (keepInline) return text');
    expect(MD_SRC).not.toMatch(/keepInline\s*=\s*subset\s*==/);
    // 镜像侧必须同样从声明表取值（名字与 .uts 一致；两边名字不同就是分叉漂移的第一步）
    expect(parseMarkdown.toString()).toContain('hasMember(MEMBER_INLINE)');
    // 有序编号落 level 槽只在**声明了 inline 的档**生效（⑩-6 拆分后的**块数组面**硬 AC 由这条兜住，#1499）
    expect(MD_CODE).toContain('LIST_LEVEL_OL');
    // list 块的文字在 `items` 里、`text` 恒空（形状从 #905 起如此），故这里断言 items 而非 text
    expect(parseMarkdown('3. 起步', SUBSET_CHAPTER)[0].items).toEqual(['起步']);
    expect(parseMarkdown('3. 起步', SUBSET_CHAPTER)[0].level).toBe(LIST_LEVEL_UL);
    expect(parseMarkdown('3. 起步', SUBSET_FORUM)[0].level).toBe(LIST_LEVEL_OL);
    // 分叉的正反两面：论坛档交出源串、章节档仍交出抹平文本
    expect(firstText('**粗** 体', SUBSET_FORUM)).toBe('**粗** 体');
    expect(firstText('**粗** 体', SUBSET_CHAPTER)).toBe('粗 体');
    // 内容精选档自 2026-10-30 #2124 起同样声明 inline（句中行内可点，与门户端 featuredMarked 对齐）：
    // 同样交出源串、同样把有序编号落到 `level` 槽 —— 这是裁定，不是「顺手加进去」（差集断言已同批改）
    expect(firstText('**粗** 体', SUBSET_FEATURED)).toBe('**粗** 体');
    expect(parseMarkdown('3. 起步', SUBSET_FEATURED)[0].level).toBe(LIST_LEVEL_OL);
  });

  it('⑩-5 的 `~~` 剥离规则在 .uts 与镜像里同源（缺任何一处都是「摘要还漏删除线」）', () => {
    expect(MD_CODE).toContain('/~~([^~]+)~~/g');
    expect(stripInlinePlain('~~作废~~')).toBe('作废');
    // 未声明 inline 的档也必须剥净（否则课程面继续漏 `~~`）—— ⑩-6 那句「逐字不变」的射程是**块数组面**，
    // 不含这条三档共用的剥离件：#1499 裁定选项 1，`~~` 源串泄漏属修复集、不是回归
    expect(firstText('参 ~~作废~~ 数', SUBSET_CHAPTER)).toBe('参 作废 数');
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
  // 两次手术、同一条纪律（ADR-0007：「手术 PR 内可更新文件路径指向，禁删/弱化断言」）：
  // ① T08（#646）把章节正文的渲染分支搬进模块私有 section 组件；
  // ② #2353 又把块怎么排搬进**四面共用**的 `components/markdown/markdown-blocks.uvue`，
  //    章节壳件只剩容器与一行转发 ⇒ 「表格怎么渲染」这几条再改指一次共用件，断言一条未删；
  //    「走哪一档」仍钉在页面（档位是页面侧决策，组件只渲染下发的块）。
  const chapterMd = read('components/markdown/markdown-blocks.uvue');
  const featured = read('pages/featured/featured-detail.uvue');

  it('chapter-view：table 块渲染成逐行 flex 的原生表格（单元格取自 items）', () => {
    expect(chapterMd).toContain("block.type == 'table'");
    expect(chapterMd).toContain('v-for="(cell, ci) in block.items"');
    expect(chapterMd).toContain("'md-table-cell-head': block.level == 1");
    expect(chapterMd).toContain('.md-table-cell-head');
    // 表格行必须归零块间距，否则同一张表会被拆成一格一格的横条。
    // #2353 起这条从「源码里有一行 margin-bottom: 0」升级成**三条钉值断言**：间距模型换成
    // margin-top-only 后，「表内续行」由 `mt-0` 令牌表达（`markdownTiers` 的第 7 行优先序），
    // 于是要证明的是「mt-0 这个类真的存在、它的值是 0、且块自身没有 margin-bottom 把它叠回来」。
    // 旧断言只证明源码里写过 0；新的三条证明跑起来就是 0（行为侧另有 markdownTypographyBehavior 承重）。
    expect(chapterMd).toMatch(/\.mt-0\s*\{\s*margin-top:\s*\$md-space-0;/);
    expect(chapterMd).not.toMatch(/\.md-block\s*\{[^}]*margin-bottom/);
    const uniScss = read('uni.scss');
    expect(uniScss).toContain('$md-space-0: 0rpx;');
  });

  it('chapter-view：走缺省档（章节正文 = 可信面全集）', () => {
    expect(chapter).toContain('parseMarkdown(detail.value!.content)');
  });

  it('featured-detail：固定 featured 档，且不持有表格渲染分支', () => {
    expect(featured).toContain('parseMarkdown(detail.value!.content, SUBSET_FEATURED)');
    expect(featured).not.toContain("block.type == 'table'");
    expect(featured).not.toContain('md-table');
  });

  /**
   * #2355 决议 D1/D2/D3 的接线组：精选面把正文交给公共件，三条轴里取 `plain`。
   *
   * 为什么这三条要单独钉：换件之后，「精选面还走不走行内 runs」「链接点开还弹不弹确认」
   * 这两件事在源码里都只剩**一行属性**，任何一条丢了都不会让别的断言红 ——
   * 而丢了的表现是「正文记号漏到屏幕上」或「链接点了没反应」，都是静默失效。
   */
  it('featured-detail：正文交给公共件，行内轴 = plain，句中链接回页面编排', () => {
    const src = read('pages/featured/featured-detail.uvue');
    expect(src).toContain("import MarkdownBlocks from '../../components/markdown/markdown-blocks.uvue'");
    expect(src).toContain(':blocks="contentBlocks"');
    expect(src).toContain('inline-axis="plain"');
    // 句中链接的**编排**仍只在页面一处（公共件只把 url 交回来）
    expect(src).toContain('@link-click="onRunLink"');
    expect(src).toContain('dispatchExternalLinkTap(url)');
    // 页面不再直引分词器：分词器只经「轴」间接取（比换件前更强的一条锁）
    expect(src).not.toContain('splitInlineRuns');
    expect(src).not.toContain('markdownInline');
    expect(src).not.toContain('runsOf(');
    expect(src).not.toContain('runClasses(');
  });

  /**
   * #2355 决议 D2 的源守卫：`FEATURED_CAPS` 的每一项 ∈ `SUBSET_MEMBERS_FEATURED` ∪ `{paragraph}`。
   * 与 AI 气泡那条同形、同理由：声明表（解析器产什么块）与 caps（渲染器画什么块）是两份天然事实。
   */
  /**
   * #2355 决议 D1/D3 的公共件侧接线组：轴有三档、缺省 `none`、链接只交回壳层。
   *
   * `none` 的缺省值本身就是一条**回归保险**：章节面与 AI 气泡都不传这个 prop，
   * 它们的行为必须逐字不变（章节子集不声明 `inline`，整段直出才是对的）。
   */
  it('公共件：inlineAxis 三档 + 缺省 none + linkClick 只交回壳层', () => {
    const src = read('components/markdown/markdown-blocks.uvue');
    for (const a of ["const AXIS_NONE : string = 'none'", "const AXIS_PLAIN : string = 'plain'", "const AXIS_FORUM : string = 'forum'"]) {
      expect(src).toContain(a);
    }
    // 缺省 = none：不传这条 prop 的壳件（章节 / AI）行为不变
    expect(src).toContain('inlineAxis? : string');
    // 缺省值必须是**字面量**：defineProps() 被提升到 setup() 之外，引用 setup 内变量是编译期错误
    // （hx:compile-only 实测原话见组件注释），这一条把它钉成不可回退的形状
    expect(src).toContain("inlineAxis: 'none'");
    expect(src).not.toContain('inlineAxis: AXIS_NONE');
    // 轴分派：forum 走格式轴（`forumBody.forumInlineRuns`）、plain 走分词器、none 直出
    expect(src).toContain('if (props.inlineAxis == AXIS_FORUM) return forumInlineRuns(src)');
    expect(src).toContain('return splitInlineRuns(src)');
    // 链接只 emit、不自己弹窗（编排留在壳层一处）
    expect(src).toContain("defineEmits(['imageClick', 'linkClick'])");
    expect(src).toContain("emit('linkClick', run.link)");
    expect(src).not.toContain('dispatchExternalLinkTap');
    expect(src).not.toContain('uni.showModal');
  });

  it('精选面的 caps 与解析器声明表同源（差集只许是段落地板 paragraph）', () => {
    const CAPS = read('pages/featured/featured-detail.uvue')
      .match(/const FEATURED_CAPS : string\[\] = \[([\s\S]*?)\]/);
    expect(CAPS).not.toBe(null);
    const declared = (CAPS[1].match(/'[a-z]+'/g) || []).map((x) => x.replace(/'/g, ''));
    const mdSrc = read('utils/markdown.uts');
    const table = mdSrc.match(/export const SUBSET_MEMBERS_FEATURED : string\[\] = \[([\s\S]*?)\]/);
    expect(table).not.toBe(null);
    const members = (table[1].match(/MEMBER_[A-Z_]+/g) || [])
      .map((c) => c.replace('MEMBER_', '').toLowerCase());
    expect(members.length).toBeGreaterThan(0);
    for (const c of declared) {
      expect([c, c === 'paragraph' || members.indexOf(c) >= 0]).toEqual([c, true]);
    }
    // 反向：声明表里的每一档都得被渲染（少一位 = 该档在精选面静默消失）。
    // 例外只有一个 `inline`：它是**行内轴开关**、不是块类型（精选档声明它之后，块数组里
    // 另有一份 `inline` 语义 —— 由公共件的 `inlineAxis` 承接，不对应任何 caps 位）。
    const BLOCK_MEMBERS = members.filter((m) => m !== 'inline');
    expect(BLOCK_MEMBERS.length).toBeGreaterThan(0);
    for (const m of BLOCK_MEMBERS) {
      expect([m, declared.indexOf(m) >= 0]).toEqual([m, true]);
    }
    // 段落地板恰好一位（`cap('paragraph')` 是整篇正文的唯一入口，重复即两份事实），
    // 且 table 不在（精选档本来就不声明它：三端交集，表格退回逐行原始文本）
    expect(declared.filter((c) => c === 'paragraph').length).toBe(1);
    expect(declared).not.toContain('table');
    expect(declared.length).toBe(BLOCK_MEMBERS.length + 1);
  });

  // ADR-0025 P2（#1240）：论坛面是第三档。档位**决策**已从页面收进 `utils/forumBody.uts`
  // （行为面由 `utils/forumBodyBehavior.test.js` 真执行承重），这里只守**接线**：页面接的是那条
  // 决策口，而不是自己重判一遍档位、更不是自持第二份子集声明。
  it('论坛面：正文与回复卡都走 `utils/forumBody` 的格式轴，页面不自己判档', () => {
    const topicBody = read('pages/forum/components/forum-topic-body.uvue');
    const replyList = read('pages/forum/components/forum-reply-list.uvue');
    for (const src of [topicBody, replyList]) {
      expect(src).toContain('forumContentBlocks');
      expect(src).toContain('isMarkdownFormat');
      // 直连解析器 / 直引档位常量 = 绕过格式轴（文本档会被解析，或论坛档被换成别的档）
      expect(src).not.toContain('parseMarkdown');
      expect(src).not.toContain('SUBSET_');
    }
    // 纯文本档的**直出**分支必须在（否则文本档会渲染成空）
    expect(topicBody).toContain('v-if="!isMarkdown"');
    expect(replyList).toContain('v-if="!isMarkdownReply(item)"');
  });

  it('论坛面：只开论坛档声明的成员，table / image 两位不在能力位里（#2356 等价改写）', () => {
    /*
     * 迁移前的判据是「这个文件里没有那两条分支」；#2356 起论坛面不再自带分支（块怎么排交给四面共用的
     * markdown-blocks），于是同一条语义换载体：**能力位里没有那一位 ⇒ 那条分支在本面永远不可达**。
     * 这不是弱化 —— 分支代码仍在公共件里（章节面要它），而「论坛面不许用它」这条只有能力位能表达。
     */
    const forumShell = read('pages/forum/components/forum-markdown-blocks.uvue');
    const caps = forumShell.match(/const FORUM_CAPS : string\[\] = \[([\s\S]*?)\]/);
    expect(caps).not.toBe(null);
    const declared = (caps[1].match(/'[a-z]+'/g) || []).map((x) => x.replace(/'/g, ''));
    for (const t of ['heading', 'paragraph', 'code', 'list', 'quote', 'divider']) {
      expect([t, declared.indexOf(t) >= 0]).toEqual([t, true]);
    }
    // 论坛档不声明 table（退回逐行原始文本）、不声明 image（图文分离，图片走 images 数组）
    expect(declared).not.toContain('table');
    expect(declared).not.toContain('image');
    // 壳件自己不持任何块分支（它只传能力位与行内轴）
    expect(forumShell).not.toContain('block.type ==');
    expect(forumShell).not.toContain('md-table');
    // 行内轴必须真的接上：漏传 inline-axis ⇒ 论坛面声明了 inline 却按直出渲染，
    // `**粗**` 会原样上屏（这正是 ADR-0025 ⑩-2 当初要消灭的形态），而且只有真机看得见。
    expect(forumShell).toContain('inline-axis="forum"');
  });

  it('⑥-1 的渲染面近似：runs 轴上文字一律经 runs 出，直出只留受守卫的兜底与代码块', () => {
    /*
     * 为什么需要这一条：#1472 的判据对象是「真正画出来的产物」，而 `.uvue` 模板进不了
     * `loadUts` 的行为缝（接线守护不构成 ③ 门证据，`docs/agents/guards.md`）。原型实测正是
     * **引用块分支直出了 `block.text`** ⇒ 记号漏到渲染面，而「按 runs 判」的机检看不见它。
     * 真判据仍在 ①a：未压缩 `uiautomator dump` 读渲染后的文本节点，断言不含成对记号。
     *
     * #2356 起这条判据的承载件从论坛渲染器换成**四面共用的块渲染件**，且组件有了两条轴：
     * 「全文件恰好一处直出」不再可能（`none` 轴的面本来就要直出）。判据因此从**计数**升级成
     * **逐行判归属**：每个文字直出口要么带 `v-if="!runsOn"` 兜底（轴关才走），要么是代码块那一处
     * （它本就不该解析）。比原来的「恰好一处」更贴身 —— 原来数不到「兜底口长在哪个分支里」。
     */
    const blocksMd = read('components/markdown/markdown-blocks.uvue');
    // ⚠️ 只在 <template> 段上计数：脚本注释里提到 v-if="!runsOn" 不算「接了一个兜底口」
    const tpl = blocksMd.slice(0, blocksMd.indexOf('</template>'));
    expect(tpl.length).toBeGreaterThan(500);   // 防空集合假绿：切歪了会得到空串
    // 轴开的四处（标题 / 段落 / 列表项 / 引用）各有一句直出兜底
    expect((tpl.match(/v-if="!runsOn"/g) || []).length).toBe(4);
    // 直出口的**总数**也钉住：少了说明某个分支根本没渲染文字，多了说明有人偷偷加了一个旁路
    expect((tpl.match(/\{\{\s*block\.text\s*\}\}/g) || []).length).toBe(4);
    expect((tpl.match(/\{\{\s*item\s*\}\}/g) || []).length).toBe(1);
    for (const line of tpl.split('\n')) {
      if (!/\{\{\s*(block\.text|item)\s*\}\}/.test(line)) continue;
      const guarded = line.indexOf('v-if="!runsOn"') >= 0;
      const isCode = line.indexOf("'md-code'") >= 0;
      expect([line.trim().slice(0, 48), guarded || isCode]).toEqual([line.trim().slice(0, 48), true]);
    }
    // runs 的入口必须在（否则上面那几条「直出受守卫」是假的：轴根本没接上）
    expect(blocksMd).toContain('runsOf(');
    expect(blocksMd).toContain('runClasses(');
    expect(blocksMd).toContain('runText(');
    expect(blocksMd).toContain('onRunTap(');
    // 精选详情页（#2124 起正文同样走 runs）：直出点同样恰好一处，且必须是代码块那一处
    // （图片分支的 `:src="block.text"` 是属性、不是文字直出，故不进计数）
    const featuredMd = read('pages/featured/featured-detail.uvue');
    expect(featuredMd).toContain('MarkdownBlocks');
    expect(featuredMd).toContain(':caps="FEATURED_CAPS"');
    expect(featuredMd).toContain('inline-axis="plain"');
    // 精选页自持的块分支计数必须为 0（换件后最典型的假绿：模板留着旧分支但不渲染）
    expect(featuredMd.match(/block\.type\s*==/g) || []).toEqual([]);
  });

  /**
   * #2355 决议 D4：列表符号按 `level` 槽出 —— OL 出编号、UL 出圆点。
   *
   * 为什么只能是接线锁而不是行为断言：`.uvue` 模板进不了 `loadUts` 的**行为缝**
   * （`docs/agents/guards.md` 明说接线守护不构成 ③ 门证据），渲染层的三目表达式没有可执行替身。
   * 可执行的那一半在解析器侧：`markdownSubsetBehavior` 已钉住「声明了 `MEMBER_INLINE` 的档
   * 才会产出 `LIST_LEVEL_OL`」，本条钉的是「产出了 OL 之后渲染层真的画编号」。
   * 两条合起来才是「精选的 1. 2. 3. 不会退化成 •」——少任何一半那句话都不成立。
   */
  it('公共件列表符号按 level 出：OL 出编号、UL 出圆点（#2355 决议 D4）', () => {
    const blocksMd = read('components/markdown/markdown-blocks.uvue');
    // 三目形态与 `forum-markdown-blocks` 同形（同一条判据在两个承载件上一致，便于互相印证）
    expect(blocksMd).toContain("block.level == 1 ? (li + 1) + '.' : '•'");
    // 旧的一律 `•` 必须绝迹（留着它就是「精选的编号还会退化成圆点」）
    expect(blocksMd).not.toContain('<text class="md-list-bullet">•</text>');
    // 编号从 1 起（`li + 1`），不是从 0 起、也不是从 `level` 取值
    expect(blocksMd).not.toContain("block.level + '.'");
  });

  it('论坛面：格式由详情壳层扁平下发（组件不自行读 topic / 回复 DTO）', () => {
    const detail = read('pages/forum/forum-detail.uvue');
    expect(detail).toContain(':format="topicFormat"');
    expect(detail).toContain('return t.content_format');
    expect(detail).toContain('contentFormat: r.content_format');
    const topicBody = read('pages/forum/components/forum-topic-body.uvue');
    const replyList = read('pages/forum/components/forum-reply-list.uvue');
    expect(topicBody).not.toMatch(/\.content_format/);
    expect(replyList).not.toMatch(/\.content_format/);
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

  it('矩阵补齐「行内格式」行，且移动端「图片 / 链接」格具名登记 #1472（链接可点需确认，⑩-9）', () => {
    // #1472 把移动端从「只剥不渲染」推进到行内渲染；矩阵此前既缺「行内格式」行、又把
    // 「图片 / 链接 · 移动端」裸写成「有」（与实现不符，ADR-0025:265 具名）。两条都登记到位。
    expect(adr).toContain('行内格式（粗体 / 斜体 / 删除线 / 行内代码）');
    expect(adr).toContain('链接经 #1472 行内可点，**移动端任何链接点开都弹确认**');
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

  it('决定 6 的覆盖面登记已回填到现状（论坛 / AI 两面都经壳层接入，旧「仍纯文本」句不得残留）', () => {
    // 旧句登记的是 #905 当时的范围，已被 ADR-0025（#1240）与 #1443 各自推进——
    // 留着它就是「拿历史快照冒充现状」，正是本条当初要防的误读的反向形态。
    expect(adr).not.toContain('论坛正文与 AI 助手回答仍是纯文本渲染');
    expect(adr).toContain('#1443 回填本条');
    // 回填后的边界换成壳层纪律，两面各自验：
    //   渲染 .uvue 不直接 import 解析器（论坛经 forumBody、AI 经 aiAnswerBody）——
    //   ⚠️ 扫描面**不含 aiSourcesDisplay.uts**：它注释里有字面 `utils/markdown`（讲 ADR 编号），
    //   拉进来就是假红；且它不在解析链上，查它没有语义。
    const body = read('pages/forum/components/forum-topic-body.uvue');
    expect(body).toContain('utils/forumBody');
    expect(body).not.toContain('utils/markdown');
    const bubble = read('components/ai-chat/ai-chat-bubble.uvue');
    expect(bubble).toContain('utils/aiAnswerBody');
    expect(bubble).not.toContain('utils/markdown');
    // 壳层存在且真的按论坛档解析（档位与展开前置的**行为**判据在
    // `utils/aiAnswerBodyBehavior.test.js` 真执行，本处只登记接线不漂）
    const shell = readText(path.join(__dirname, 'aiAnswerBody.uts'));
    expect(shell).toContain("from './markdown'");
    expect(shell).toContain('SUBSET_AI_ANSWER');
  });

  it('AI 回答面的配图接线（#2362 / 移动端 ADR 0056）：闸在壳层、画在公共件、预览走同一点', () => {
    const shell = readText(path.join(__dirname, 'aiAnswerBody.uts'));
    expect(shell).toContain("export const AI_ANSWER_IMAGE_PREFIX = '/api/ai-assistant/diagnosis/ossimg/'");
    expect(shell).toContain('resolveFileUrl');
    expect(shell).toContain('expandLegacyImageMarkers');
    // #2354（决议 D5）：AI 回答的**渲染**从气泡搬进四面共用的块渲染件 ⇒ 这一组里凡是
    // 「怎么把一个 image 块画出来」的三条（image 分支 / md-image 类 / 点击回调）随承载件改指，
    // 一条不删；而「闸在壳层」「图集与下标换算住纯函数层」「气泡接的仍是这两个纯函数」
    // 「预览起点不按 URL 反查」四条**语义未变**，仍钉在气泡与壳层上。
    const blocks = read('components/markdown/markdown-blocks.uvue');
    expect(blocks).toContain("block.type == 'image'");
    expect(blocks).toContain('md-image');
    expect(blocks).toContain('onImageTap');
    const bubble = read('components/ai-chat/ai-chat-bubble.uvue');
    expect(bubble).toContain('previewImages');
    // 图集与下标换算住在纯函数层（行为面真执行），模板只调本地名：
    // 按 URL 值反查会让「同图重复出现」的第二张永远从第一张翻起。
    expect(bubble).toContain('onPreviewAssistantImageAt');
    expect(bubble).toContain('aiAnswerImageUrls(');
    expect(bubble).toContain('aiAnswerImageIndexAt(');
    expect(bubble).not.toContain('urls.indexOf(');
    // #2354 新增：气泡把公共件接上，且**按块序号**（payload.index）换算 —— 这是 #2362
    // 「不按 URL 反查」在换件之后仍然成立的唯一证据：载荷里若只剩 url，换算必然退回反查。
    expect(bubble).toContain('MarkdownBlocks');
    expect(bubble).toContain('@image-click="onPreviewAssistantImageAt"');
    expect(bubble).toContain('payload.index');
    // 论坛正文仍图文分离。#2356 起论坛面不自带分支（块怎么排交给公共件）⇒ 这条改指**能力位**：
    // 「论坛面不许长出 image」现在由「FORUM_CAPS 里没有 image」表达（同 ⑥-1 上方那条口径）。
    const forumShell = read('pages/forum/components/forum-markdown-blocks.uvue');
    const forumCaps = forumShell.match(/const FORUM_CAPS : string\[\] = \[([\s\S]*?)\]/);
    expect(forumCaps).not.toBe(null);
    expect(forumCaps[1]).not.toContain("'image'");
  });

  it('AI 气泡接的是同一套刻度（#2354 决议 D1/D2/D5）：换件、caps 七位、不再自排块', () => {
    const bubble = read('components/ai-chat/ai-chat-bubble.uvue');
    // D1：助手正文整体换成公共件；气泡自带的 8 条分支模板必须绝迹（否则「一套刻度」是假的）
    expect(bubble).toContain("import MarkdownBlocks from '../markdown/markdown-blocks.uvue'");
    expect(bubble).toContain(':blocks="assistantBlocks"');
    expect(bubble).toContain(':caps="AI_ANSWER_CAPS"');
    // 「不再自排块」的判据：整份源码里不出现按块类型分叉的写法
    expect(bubble).not.toContain("block.type ==");
    // D2：caps 常量写在壳件脚本里（与 chapter-markdown 的 CHAPTER_CAPS 同形），不进模板字面量
    expect(bubble).toContain('const AI_ANSWER_CAPS : string[] = [');
    // 公共件的 caps 是**必填**：少传等于整面空渲染，所以「传了」这件事本身要钉
    const blocksSrc = read('components/markdown/markdown-blocks.uvue');
    expect(blocksSrc).toMatch(/caps\s*:\s*string\[\]/);
    expect(blocksSrc).not.toMatch(/caps\s*\?:\s*string\[\]/);
  });

  /**
   * D2 的源守卫：`AI_ANSWER_CAPS` 的每一项都必须 ∈ `SUBSET_MEMBERS_AI_ANSWER` ∪ `{paragraph}`。
   *
   * 为什么需要它：这两处天然是**两份事实** —— 声明表在 `utils/markdown.uts`（决定解析器产什么块），
   * caps 在气泡壳件（决定渲染器画什么块）。少一位，那一档在气泡里静默消失；多一位（比如顺手
   * 加个 table），就是渲染一条永远不可达的分支、并掩盖「档位被换错」。两者之间没有别的机检。
   *
   * 为什么 `paragraph` 要单列：它是段落地板，从来没进过声明表（同一个文件里有一条
   * `not.toContain('MEMBER_PARAGRAPH')` 钉着），但公共件的 paragraph 分支靠 `cap('paragraph')` 开
   * —— 不传这一位，整条回答只剩标题与列表。
   */
  it('AI 气泡的 caps 与解析器声明表同源（差集只许是段落地板 paragraph）', () => {
    const CAPS = read('components/ai-chat/ai-chat-bubble.uvue')
      .match(/const AI_ANSWER_CAPS : string\[\] = \[([\s\S]*?)\]/);
    expect(CAPS).not.toBe(null);
    const declared = (CAPS[1].match(/'[a-z]+'/g) || []).map((x) => x.replace(/'/g, ''));
    // 解析器那边的声明表：以 .uts 源里的字面量为准（.uts 不能在 jest 里 import，本文件全程如此）。
    // ⚠️ 用正则取数组体、不要拿 indexOf(']') 切：类型标注里的 `string[]` 自带一个 `]`，
    // 按它切会得到空串，而「空串 → 成员列表为空 → 循环体一条都不跑」正是最典型的假绿。
    const mdSrc = read('utils/markdown.uts');
    const table = mdSrc.match(/export const SUBSET_MEMBERS_AI_ANSWER : string\[\] = \[([\s\S]*?)\]/);
    expect(table).not.toBe(null);
    const members = (table[1].match(/MEMBER_[A-Z_]+/g) || [])
      .map((c) => c.replace('MEMBER_', '').toLowerCase());
    expect(members.length).toBeGreaterThan(0);
    // 每一项都要是「声明表成员」或段落地板；一条不满足就是静默消失 / 不可达分支
    for (const c of declared) {
      expect([c, c === 'paragraph' || members.indexOf(c) >= 0]).toEqual([c, true]);
    }
    // 反向：声明表里的每一档都得被渲染（少一位 = 该档在气泡里静默消失）
    for (const m of members) {
      expect([m, declared.indexOf(m) >= 0]).toEqual([m, true]);
    }
    // 段落地板恰好一位，且 table 不在（AI 档本来就不声明它）
    expect(declared).toContain('paragraph');
    expect(declared).not.toContain('table');
    // 七位：六个声明档 + paragraph
    expect(declared.length).toBe(members.length + 1);
  });

  /**
   * #2356 的守卫：论坛面的**能力位**与**行内轴**各自与真源对齐。
   *
   * 为什么必须有机检：这两处天然是**两份事实** —— 声明表在 `utils/markdown.uts`（决定解析器产什么块、
   * 块内文字留不留记号），能力位在论坛壳件（决定渲染器画什么块），行内轴在公共件与壳件的交界（决定
   * 文字怎么出）。少一位 = 那一档静默消失；多一位 = 渲染一条永远不可达的分支、并掩盖「档位被换错」；
   * 轴漏传 = 记号原样上屏。三者之间没有别的机检。
   */
  it('论坛面 caps 与解析器声明表同源（差集只许是段落地板 paragraph）', () => {
    const CAPS = read('pages/forum/components/forum-markdown-blocks.uvue')
      .match(/const FORUM_CAPS : string\[\] = \[([\s\S]*?)\]/);
    expect(CAPS).not.toBe(null);
    const declared = (CAPS[1].match(/'[a-z]+'/g) || []).map((x) => x.replace(/'/g, ''));
    // 声明表：以 .uts 源里的字面量为准（.uts 不能在 jest 里 import，本文件全程如此）。
    // ⚠️ 用正则取数组体、不要拿 indexOf(']') 切：类型标注 string[] 自带一个 ]，按它切会得到空串，
    // 而「空串 → 成员列表为空 → 循环体一条都不跑」正是最典型的假绿（#2354 实测踩过）。
    const mdSrc = read('utils/markdown.uts');
    const table = mdSrc.match(/export const SUBSET_MEMBERS_FORUM : string\[\] = \[([\s\S]*?)\]/);
    expect(table).not.toBe(null);
    const members = (table[1].match(/MEMBER_[A-Z_]+/g) || [])
      .map((c) => c.replace('MEMBER_', '').toLowerCase())
      .filter((m) => m !== 'inline');   // inline 不是块分支：它改块内文字语义，由 inline-axis 表达
    expect(members.length).toBeGreaterThan(0);
    for (const c of declared) {
      expect([c, c === 'paragraph' || members.indexOf(c) >= 0]).toEqual([c, true]);
    }
    for (const m of members) {
      expect([m, declared.indexOf(m) >= 0]).toEqual([m, true]);
    }
    expect(declared).toContain('paragraph');
    expect(declared.length).toBe(members.length + 1);   // 五个块成员 + paragraph
  });

  it('行内 run 不建第二份字号表：字号只来自档位，唯一的覆写是行内码那条刻度', () => {
    // 这条守的是本票的核心主张。反例就在迁移前的论坛渲染器里：`.md-run { font-size: 30rpx }`
    // 让 h1–h6 全部渲染成 30rpx（子 text 不继承父样式）⇒ 论坛的标题层级形同虚设。
    const blocksMd = read('components/markdown/markdown-blocks.uvue');
    // ⚠️ 只在 <style> 段上判：脚本注释里引用旧表（`.md-run { font-size: 30rpx }`）不是违规，是来路说明。
    // 取 lastIndexOf：脚本里有一句注释提到 `<style>`（讲 white-space 的登记纪律），indexOf 会从那里切开。
    const stylePart = blocksMd.slice(blocksMd.lastIndexOf('<style'));
    expect(stylePart.length).toBeGreaterThan(200);   // 防空集合假绿
    // 基类若被建回来（`.md-run { ... }`），旧表就回来了
    expect(/\.md-run\s*\{/.test(stylePart)).toBe(false);
    const bodies = [...stylePart.matchAll(/\.md-run-[a-z]+\s*\{([^}]*)\}/g)].map((m) => m[1]);
    expect(bodies.length).toBeGreaterThan(3);   // 防空集合假绿：样式位至少 b/i/s/c/l 五种
    const sizes = [];
    for (const body of bodies) {
      expect([body.trim().slice(0, 32), /\bline-height\s*:/.test(body)]).toEqual([body.trim().slice(0, 32), false]);
      const m = body.match(/\bfont-size\s*:\s*([^;]+);/);
      if (m) sizes.push(m[1].trim());
    }
    // 唯一允许的字号声明：行内码（刻度规定它是 26rpx，与正文 30rpx 不同档）
    expect(sizes).toEqual(['$md-size-code-inline']);
  });

  it('论坛轴只走格式轴：`forum` 分派到 forumInlineRuns，分词器的运行期引用只服务 `plain`', () => {
    // #2353 决议 D5 的接线口径：#2356 落 `forum` 轴时**只能**经 `utils/forumBody.forumInlineRuns`。
    // 直接拿 `utils/markdownInline` 的分词器处理论坛源串会让论坛面绕过格式轴，那是第二处档位决策。
    // （`plain` 轴自 #2355 起就用 `splitInlineRuns` —— 那是精选面的轴，与论坛的格式轴是两条。）
    const blocksMd = read('components/markdown/markdown-blocks.uvue');
    expect(blocksMd).toContain("from '../../utils/forumBody'");
    const runsOf = (blocksMd.match(/const runsOf = [\s\S]*?\n    \}/) || [''])[0];
    expect(runsOf.length).toBeGreaterThan(50);   // 防空集合假绿
    expect(runsOf).toContain('forumInlineRuns(src)');
    expect(runsOf).toContain('splitInlineRuns(src)');
    // 论坛那一支必须先于 plain 分派（顺序反了就等于论坛也走分词器）
    expect(runsOf.indexOf('forumInlineRuns(src)') < runsOf.indexOf('splitInlineRuns(src)')).toBe(true);
    // `utils/markdownInline` 的语句导入只允许两种：类型导入、或 plain 轴的 `splitInlineRuns`
    const lines = blocksMd.split('\n')
      .filter((l) => /^\s*import\b/.test(l))
      .filter((l) => l.indexOf('utils/markdownInline') >= 0);
    expect(lines.length).toBeGreaterThan(0);   // 导入必须在（否则这条判据是空跑）
    for (const line of lines) {
      const ok = /^\s*import\s+type\s/.test(line) || /import\s+\{\s*splitInlineRuns\s*\}/.test(line);
      expect([line.trim().slice(0, 60), ok]).toEqual([line.trim().slice(0, 60), true]);
    }
  });

  it('公共件根节点不带宽度 / 内边距（#2354 AC 第 2 条：气泡正文内宽不被换件改变）', () => {
    // 气泡的正文内宽完全由 `.message-bubble` 的 padding（20rpx 24rpx）与 max-width（600rpx）决定。
    // 公共件一旦自己带上 width / padding / margin，窄气泡里的内宽就会被二次收窄 —— 这是换件时
    // 最容易悄悄发生的一处几何回归（#2353 调研补记 R3 的硬约束，本票 AC 第 2 条按此判）。
    const blocksSrc = read('components/markdown/markdown-blocks.uvue');
    const bodyRule = blocksSrc.match(/\.md-body\s*\{([^}]*)\}/);
    expect(bodyRule).not.toBe(null);
    for (const prop of ['width', 'padding', 'margin']) {
      expect([prop, new RegExp('(^|[;\\s])' + prop + '\\s*:').test(bodyRule[1])]).toEqual([prop, false]);
    }
    // 气泡外壳本票不动（三条一起钉，改了就是越界）
    const bubbleSrc = read('components/ai-chat/ai-chat-bubble.uvue');
    expect(bubbleSrc).toContain('max-width: 600rpx;');
    expect(bubbleSrc).toContain('padding: 20rpx 24rpx;');
    expect(bubbleSrc).toContain('border-radius: 16rpx;');
  });

  it('跨栈判据：站内配图前缀与后端 \`diagnosisImageRoute\` 逐字同源（改一边必红）', () => {
    // 手机端唯一的跨栈判据就在这里：后端换路由而移动端漏改时，
    // 配图会整体静默退化成 alt 文字（AC1 无声失效），所以必须逐字钉住。
    const shell = readText(path.join(__dirname, 'aiAnswerBody.uts'));
    const backend = readText(path.join(REPO_ROOT, 'backend', 'internal', 'aiassistant', 'diagnosis_adapter.go'));
    const goRoute = backend.match(/const diagnosisImageRoute = "([^"]+)"/);
    expect(goRoute).not.toBe(null);
    expect(shell).toContain("export const AI_ANSWER_IMAGE_PREFIX = '" + goRoute[1] + "'");
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
