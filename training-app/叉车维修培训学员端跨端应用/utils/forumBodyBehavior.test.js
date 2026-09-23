/**
 * 论坛正文格式轴（`content_format` → 渲染档位）· **行为级**测试（ADR-0025 决策②/③，票 #1240 P2）
 *
 * 为什么是行为而不是源码文本：P2 的验收标准（ADR-0025 ⑥-1）写的是「**Web 发的 Markdown 帖在移动端
 * 不再显示源串**」—— 这条只有**跑起来看产出物**才算数。断言 `.uvue` 里出现过 `SUBSET_FORUM` 是接线
 * 守护，算法被改坏（例如 text 档也拿去解析、或解析时把论坛档换成章节档）它照样绿
 * （判据与分类见 `docs/agents/guards.md`）。故本套件跑 `utils/forumBody.uts` 的**产出物**（块数组）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`（先例 `markdownSubsetBehavior.test.js`）。
 * **注入的解析器是从 `utils/markdown.uts` 真执行出来的那一份**，不是手抄镜像 —— ③ 判据②
 * 「它测的是该测的那一支吗」由这条保证（镜像会让「解析器改了而镜子没改」静默变绿）。
 *
 * **成对取证**（③ 判据③）：末组用注入变异的副本证明上面的判据**有牙**，变异形态是两种真实坏实现：
 * ① 拆掉「非 markdown 不解析」的闸门（纯文本档也被解析，源串语义被误套）；② 解析时用错档位
 * （论坛档换成章节档，表格被渲染出来 —— 与「按端分档」的口径直接冲突）。
 *
 * ## #1273 并档进来的一条（票面：「可并进现有套件省一个套件」）
 *
 * 本套件从 #1273 起还守**格式轴的第二条消费路径**：列表摘要 `forumContentPlainText`（投影）与
 * `getContentPreview`（截断 + 去换行）。判据口径不变 —— 跑真执行看**产出的字符串**，不读源码文本。
 * 摘要与正文共用一条格式轴 ⇒ 它必须住在这条轴的旁边被同一套夹具执行，而不是另起一份镜像断言。
 * 上游三层（`markdown` → `forumBody` → `forumDisplay`）的真执行由 `utils/forumChainHarness.js` 提供，
 * 成对取证时**只换坏的那一层**、其余层仍取真源。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, importedNames, readText } = require('./utsHarness');
const { forumDisplayModule, forumChain, DISPLAY_UTS } = require('./forumChainHarness');

const BODY_UTS = path.join(__dirname, 'forumBody.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');

const TABLE = '| 故障码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |';
const MERMAID = '```mermaid\ngraph TD;\nA-->B;\n```';

/** 真源依赖注入表：解析器与档位常量都取自**真执行**出来的 `utils/markdown.uts` */
function markdownBindings() {
  const m = loadUts(MD_UTS, {});
  return {
    parseMarkdown: m.parseMarkdown,
    SUBSET_FORUM: m.SUBSET_FORUM,
    SUBSET_CHAPTER: m.SUBSET_CHAPTER,
  };
}

/** 每次取一个**全新模块实例**（模块级常量不可变，互不串） */
const body = () => loadUts(BODY_UTS, markdownBindings());

const types = (blocks) => blocks.map((b) => b.type);
const textOf = (blocks) => blocks.map((b) => b.text).join('\n');
const itemsOf = (blocks) => blocks.reduce((acc, b) => acc.concat(b.items || []), []).join('\n');

// ===== ① 渲染档位闸门：只有 markdown 走解析 =====

describe('档位闸门：`markdown` 是唯一会走解析的取值', () => {
  it('`markdown` 命中；`text` 与一切契约外取值都不命中（与 DTO 归一同向：缺省即纯文本）', () => {
    const b = body();
    expect(b.isMarkdownFormat('markdown')).toBe(true);
    for (const other of ['text', '', 'MD', 'Markdown', 'md', 'html', '  markdown  ']) {
      expect([other, b.isMarkdownFormat(other)]).toEqual([other, false]);
    }
  });

  it('闸门与产出一致（正/反向对账）：命中 ⇒ 产出块；不命中 ⇒ 空数组（壳层直出原串）', () => {
    const b = body();
    const PROBE = '**故障码 E01**';
    for (const format of ['markdown', 'text', '', 'MD', 'html']) {
      const hit = b.isMarkdownFormat(format);
      const blocks = b.forumContentBlocks(PROBE, format);
      expect([format, blocks.length > 0]).toEqual([format, hit]);
      // 不解析时必须是**空数组**，不能是「解析成一段文本」—— 后者会让壳层把两份都渲染出来
      if (!hit) expect(blocks).toEqual([]);
    }
  });
});

// ===== ② Markdown 档：源串不再出现，且按论坛子集成块（ADR-0025 ⑥-1）=====

describe('Markdown 档：解析成块，源串（星号 / 链接语法 / 图片语法）不再显示', () => {
  it('行内标记被剥离而不是原样显示（本票的原始故障：读者看见 `**故障码 E01**`）', () => {
    const b = body();
    expect(textOf(b.forumContentBlocks('**故障码 E01**', 'markdown'))).toBe('故障码 E01');
    expect(textOf(b.forumContentBlocks('见 [维修手册](https://e.com/m) 第 3 节', 'markdown')))
      .toBe('见 维修手册 第 3 节');
    // 行内代码同理（本批不做行内渲染，但也不许显示反引号）
    expect(textOf(b.forumContentBlocks('参数 `torque` 见铭牌', 'markdown'))).toBe('参数 torque 见铭牌');
  });

  it('论坛档声明的五个成员都真成块（标题 / 列表 / 引用 / 代码块 / 分隔线）', () => {
    const b = body();
    expect(types(b.forumContentBlocks('### 三级标题', 'markdown'))).toEqual(['heading']);
    expect(types(b.forumContentBlocks('- 项一\n- 项二', 'markdown'))).toEqual(['list']);
    expect(types(b.forumContentBlocks('> 注意安全', 'markdown'))).toEqual(['quote']);
    expect(types(b.forumContentBlocks('```\nconst a = 1\n```', 'markdown'))).toEqual(['code']);
    expect(types(b.forumContentBlocks('---', 'markdown'))).toEqual(['divider']);
  });

  it('未声明的语法退回**可见文本**（降级可读）：表格与正文内嵌图片都不丢内容', () => {
    const b = body();
    const table = b.forumContentBlocks(TABLE, 'markdown');
    expect(types(table)).not.toContain('table');
    expect(textOf(table)).toContain('E01');        // 逐行原始文本仍在
    expect(textOf(table)).toContain('|');          // 连表格的竖线都在（「退回原始文本」的字面口径）

    const img = b.forumContentBlocks('见 ![故障图](https://e.com/a.png) 这张', 'markdown');
    expect(types(img)).not.toContain('image');     // 图文分离：正文内嵌图片不产 image 块
    expect(textOf(img)).toContain('故障图');        // 但 alt 文本可见（与 Web 同口径）
    expect(textOf(img)).not.toContain('https://e.com/a.png');
  });

  it('mermaid 是**真降级**且已具名：围栏源码原样可见（不是空白，也不是渲染）', () => {
    const b = body();
    const blocks = b.forumContentBlocks(MERMAID, 'markdown');
    expect(types(blocks)).toEqual(['code']);
    expect(textOf(blocks)).toContain('graph TD;');
  });
});

// ===== ③ 纯文本档：直出原串（空数组 = 「不解析」，不是「没内容」）=====

describe('纯文本档：不解析（空数组 = 壳层直出原串）', () => {
  it('`text` 档不产任何块，星号按字面保留（纯文本里它就是两个字符）', () => {
    const b = body();
    expect(b.forumContentBlocks('**故障码 E01**', 'text')).toEqual([]);
    expect(b.forumContentBlocks('### 这不是标题', 'text')).toEqual([]);
  });

  it('契约外脏值（大小写不符 / 未来新值）落「不解析」一侧 —— 与 DTO 归一的缺省方向同向', () => {
    const b = body();
    for (const dirty of ['MD', 'Markdown', 'html', 'rich', '']) {
      expect([dirty, b.forumContentBlocks('**x**', dirty)]).toEqual([dirty, []]);
    }
  });

  it('空正文不产块（两种档一致）—— 空数组的语义由**档位闸门**决定，不由数组长度决定', () => {
    const b = body();
    expect(b.forumContentBlocks('', 'markdown')).toEqual([]);
    expect(b.forumContentBlocks('', 'text')).toEqual([]);
  });
});

// ===== ④ 列表摘要：格式轴的**第二条消费路径**（#1273 · 根 ADR-0044「列表摘要必须剥成纯文本」）=====

/** 真执行的展示层（上游取真源；成对取证时传变异后的那一份） */
const display = () => forumDisplayModule();

describe('纯文本投影：markdown 档剥成纯文本，text 档逐字不变', () => {
  it('记号被剥掉而不是直出给读者（本票的原始故障：列表卡片显示 `**故障码**` 的星号）', () => {
    const plain = (src) => body().forumContentPlainText(src, 'markdown');
    expect(plain('**粗**')).toBe('粗');               // 行内格式：剥成纯文本（本档不做行内渲染）
    expect(plain('## 标题')).toBe('标题');              // 标题不带 `##`
    expect(plain('- 项一\n- 项二')).toBe('项一 项二');  // 列表项只剩文字
    expect(plain('> 注意安全')).toBe('注意安全');
    expect(plain('```\nconst a = 1\n```')).toBe('const a = 1');
    expect(plain('见 [维修手册](https://e.com/m) 第 3 节')).toBe('见 维修手册 第 3 节');
    // 分隔线是纯记号 ⇒ 不贡献字符，两侧正文以空格相连
    expect(plain('### 标题\n---\n正文')).toBe('标题 正文');
  });

  it('未声明语法退回**可见文本**：表格的 `|` 仍在、内嵌图片出 alt（与详情页正文同口径）', () => {
    const b = body();
    const table = b.forumContentPlainText(TABLE, 'markdown');
    expect(table).toContain('|');          // 段落地板：退回逐行原始文本，连竖线都在
    expect(table).toContain('E01');        // 内容不丢
    const img = b.forumContentPlainText('见 ![故障图](https://e.com/a.png) 这张', 'markdown');
    expect(img).toContain('故障图');        // 图文分离：内嵌图片展开成 alt 文本
    expect(img).not.toContain('https://e.com/a.png');
  });

  it('`text` 与契约外脏值 ⇒ **逐字原样**（存量帖的预览一个字节都不变 = 零回归）', () => {
    const b = body();
    for (const format of ['text', '', 'MD', 'Markdown', 'html']) {
      expect([format, b.forumContentPlainText('**故障码** ## 不是标题', format)])
        .toEqual([format, '**故障码** ## 不是标题']);
    }
  });

  it('空正文投影成空串（两档一致）—— 与 `forumContentBlocks` 的空数组哨兵同向', () => {
    const b = body();
    expect(b.forumContentPlainText('', 'markdown')).toBe('');
    expect(b.forumContentPlainText('', 'text')).toBe('');
  });
});

describe('摘要层：投影 + 80 字截断 + **全局**去换行（`utils/forumDisplay.getContentPreview`）', () => {
  it('缺省参数 = 纯文本档 ⇒ 老调用点（只传正文）逐字不变，只多摊平换行', () => {
    expect(display().getContentPreview('**故障码**\n第二行')).toBe('**故障码** 第二行');
  });

  it('markdown 档先投影再截断：列表卡片不再显示源串（#1273 的验收面）', () => {
    expect(display().getContentPreview('**故障码 E01**\n- 项一\n- 项二', 'markdown'))
      .toBe('故障码 E01 项一 项二');
  });

  it('去换行是**全局**的：多行帖的预览不留 `\\n`（本票顺手修的既有 bug：只替换第一个换行符）', () => {
    const preview = display().getContentPreview('第一行\n第二行\n第三行\n第四行', 'text');
    expect(preview).not.toContain('\n');
    expect(preview).toBe('第一行 第二行 第三行 第四行');
  });

  it('截断口径不变：≤80 字原样、>80 字取前 80 + 省略号（投影后同样适用）', () => {
    const d = display();
    const s80 = 'a'.repeat(80);
    expect(d.getContentPreview(s80, 'text')).toBe(s80);
    expect(d.getContentPreview(`${s80}b`, 'text')).toBe(`${s80}...`);
    // 长 Markdown 正文：上限仍由这条口径决定（不是「投影后有多长就多长」）
    const long = d.getContentPreview(`## 标题\n${'- 项'.repeat(60)}`, 'markdown');
    expect([long.length, long.endsWith('...')]).toEqual([83, true]);
  });

  it('预览与正文**同源**：论坛档声明表一变，预览同批跟着变，而投影代码一行没改', () => {
    const before = display().getContentPreview(TABLE, 'markdown');
    // 把 table 收进论坛档（真实将来动作，非虚构：ADR-0025 ③ 的「另立一票」就是它）
    const file = mutatedCopy(MD_UTS, [
      ['MEMBER_CODE, MEMBER_DIVIDER,\n]', 'MEMBER_CODE, MEMBER_DIVIDER, MEMBER_TABLE,\n]'],
    ], 'forum-md-');
    const after = forumChain({ md: file }).display.getContentPreview(TABLE, 'markdown');
    expect(before).toContain('|');       // 今天：未声明 ⇒ 退回原始文本，竖线可见
    expect(after).not.toContain('|');    // 声明后：出 table 块 ⇒ 投影跟着变成单元格文字
    expect(after).toContain('E01');      // 两个方向内容都不丢（「降级必须可读」）
    expect(after).toContain('故障码');
  });
});

// ===== ⑤ 成对取证：注入变异后，上面的判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 返回**变异副本路径**（各层夹具自己真执行，工作树不动） */
function mutatedCopy(absFile, replacements, prefix) {
  let src = readText(absFile);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  const file = path.join(dir, path.basename(absFile));
  fs.writeFileSync(file, src);
  return file;
}

/** 读真源 → 注入变异 → 真执行（格式轴那一层） */
function loadMutated(replacements) {
  return loadUts(mutatedCopy(BODY_UTS, replacements, 'forum-body-'), markdownBindings());
}

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红：真源上，纯文本档不产块（对照组）', () => {
    expect(body().forumContentBlocks('**故障码 E01**', 'text')).toEqual([]);
  });

  it('必红 · 拆掉档位闸门：纯文本档也被解析 ⇒ 「text 档返回空数组」的判据必然判红', () => {
    const broken = loadMutated([['    if (!isMarkdownFormat(format)) return empty\n', '']]);
    // 坏实现真的把纯文本也解析了（星号语义被误套，正是本票要防的反向错误）
    expect(types(broken.forumContentBlocks('**故障码 E01**', 'text'))).toContain('paragraph');
    expect(broken.forumContentBlocks('**故障码 E01**', 'text')).not.toEqual([]);
  });

  it('必红 · 解析用错档位：论坛档换成章节档 ⇒ 表格被渲染成 table 块，「未声明语法不产块」判据必然判红', () => {
    const broken = loadMutated([['parseMarkdown(content, SUBSET_FORUM)', 'parseMarkdown(content, SUBSET_CHAPTER)']]);
    expect(types(broken.forumContentBlocks(TABLE, 'markdown'))).toContain('table');
  });

  // —— 以下四条钉 #1273 摘要面的判据：一条**必不红**对照 + 三条**必红**（三种真实坏实现）——

  it('必不红（摘要对照组）：真源上 markdown 档剥净、text 档逐字不变、多行摊平', () => {
    const d = display();
    expect(d.getContentPreview('**故障码**', 'markdown')).toBe('故障码');
    expect(d.getContentPreview('**故障码**', 'text')).toBe('**故障码**');
    expect(d.getContentPreview('a\nb\nc', 'text')).toBe('a b c');
  });

  it('必红 · 摘要走回「直出原串」：投影被绕过 ⇒ 「markdown 档剥成纯文本」判据必然判红', () => {
    // 坏实现 = 本票要治的那个故障本身（预览不消费 content_format）
    const file = mutatedCopy(DISPLAY_UTS, [[
      "forumContentPlainText(content, format).replace(/\\n/g, ' ')",
      "content.replace(/\\n/g, ' ')",
    ]], 'forum-preview-');
    const broken = forumDisplayModule(undefined, file);
    expect(broken.getContentPreview('**故障码 E01**', 'markdown')).toBe('**故障码 E01**');
    expect(broken.getContentPreview('**故障码 E01**', 'markdown')).not.toBe('故障码 E01');
  });

  it('必红 · 纯文本档也拿去投影：档位闸门被拆 ⇒ 「text 档逐字不变」判据必然判红', () => {
    // 坏实现 = 把作者的字面星号当 Markdown 语义误套（text 档里它就是两个字符）。
    // 两道闸门都要拆：只拆投影那一行 ⇒ `forumContentBlocks` 仍返回空数组 ⇒ 摘要变成**空白**
    // —— 那是另一种坏实现（内容消失而非误套语义），同一条「text 档逐字不变」判据照样判红。
    const broken = loadMutated([
      ['    if (!isMarkdownFormat(format)) return empty\n', ''],
      ['    if (!isMarkdownFormat(format)) return content\n', ''],
    ]);
    expect(broken.forumContentPlainText('**故障码**', 'text')).toBe('故障码');
    // 同一份坏实现顺着链下来，摘要层的「缺省即纯文本」判据也一起红（消费方不换实现，只换上游）
    const brokenDisplay = forumDisplayModule(broken);
    expect(brokenDisplay.getContentPreview('**故障码**')).toBe('故障码');
    expect(brokenDisplay.getContentPreview('**故障码**')).not.toBe('**故障码**');
  });

  it('必红 · 去换行退回「只换第一个」：全局正则换成单发匹配 ⇒ 「预览不含换行」判据必然判红', () => {
    const file = mutatedCopy(DISPLAY_UTS, [[
      "forumContentPlainText(content, format).replace(/\\n/g, ' ')",
      "forumContentPlainText(content, format).replace('\\n', ' ')",
    ]], 'forum-newline-');
    const broken = forumDisplayModule(undefined, file);
    expect(broken.getContentPreview('第一行\n第二行\n第三行', 'text')).toBe('第一行 第二行\n第三行');
    expect(broken.getContentPreview('第一行\n第二行\n第三行', 'text')).toContain('\n');
  });
});

// ===== ⑥ 模块契约：`forumBody.uts` 是薄桥，不持第二份声明 =====

describe('模块契约：只桥到 `utils/markdown.uts`，不自持解析或档位', () => {
  it('运行期 import 恰为解析入口与论坛档常量（没有第二份子集声明 / 第二份归一）', () => {
    expect(importedNames(readText(BODY_UTS)).sort()).toEqual(['SUBSET_FORUM', 'parseMarkdown']);
  });

  it('每次载入都是新实例，档位闸门不是可变共享状态', () => {
    const a = body();
    const c = body();
    expect(a.isMarkdownFormat('markdown')).toBe(c.isMarkdownFormat('markdown'));
  });
});
