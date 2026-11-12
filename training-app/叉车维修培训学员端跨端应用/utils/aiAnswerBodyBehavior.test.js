/**
 * AI 助手回答的渲染壳层（`utils/aiAnswerBody.uts`）· **行为级**测试（#1443，ADR-0046 决定 6 回填面）
 *
 * 为什么是行为而不是源码文本（`utils/forumBodyBehavior.test.js` 同判据）：#1443 的验收标准是
 * 「助手回答里的 Markdown 记号不再以源串示人」—— 这条只有**跑起来看产出块**才算数。断言
 * `.uvue` 里出现过 `aiAnswerBody` 是接线守护，壳层被改坏（档位拿错、展开被摘）它照样绿
 * （判据与分类见 `docs/agents/guards.md`）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`。注入的**各层都是真执行出来的源**——解析器与档位常量
 * 取 `utils/markdown.uts`、分词器（#2366 起壳层用它判「能不能按图切段」）取 `utils/markdownInline.uts`、
 * `expandLegacyImageMarkers` 取 `utils/aiSourcesDisplay.uts`、
 * `resolveFileUrl` 取 `utils/format.uts`——不是手抄镜像；镜像会让「真源改了而镜子没改」静默变绿
 * （#1273 先例：三层链只换坏的那层、其余取真源）。
 *
 * #2362 追加判据（移动端 ADR 0056）：独行站内图 → `image` 块（text 为**绝对**本站 URL）；
 * 站外图 → 退化成 alt 文字、绝不产 image 块。
 *
 * #2366 追加判据：句内站内图 → **按图切成三块**（段落 / 图 / 段落），图块 text 是绝对本站 URL；
 * 记号跨图（粗体 / 链接 / 行内码 / 删除线 / 斜体包住图）⇒ 整行不切、退回单段；空 alt ⇒ 不切；
 * 一行多图里只要一张不安全 ⇒ 整行不切。切分判据吃的是**分词器自己的产出**
 * （`utils/markdownInline.uts` 的 `splitInlineRuns` / `inlineRunsPlainText`，真执行注入）。
 *
 * 成对取证（③ 判据③）：末组用注入变异的副本证明上面的判据**有牙**，两种真实坏实现——
 * ① 摘掉 #1279 的展开前置（百分号编码的内网路径进正文）；② 档位拿错（AI 回答档换成论坛档，
 * 表格退回逐行段落 —— 与 #2398「表格渲成原生表格」的裁定直接冲突）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, importedNames, readText } = require('./utsHarness');

const SHELL_UTS = path.join(__dirname, 'aiAnswerBody.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');
const DISPLAY_UTS = path.join(__dirname, 'aiSourcesDisplay.uts');
const INLINE_UTS = path.join(__dirname, 'markdownInline.uts');

/** 真源各层（解析器 / 分词器 / 展开器 / URL 拼接）里壳层需要的名字（importedNames 收**源码串**
 *  不收路径 —— 传错会静默得空清单，连带把注入绑定掏空，一处错让本套件全红且看不出根因） */
const FORMAT_UTS = path.join(__dirname, 'format.uts');

/** 站内基址只在这一处夹具出现：拼接口径归 `utils/format.resolveFileUrl`，本套件不重复实现它 */
const API_BASE = 'https://api.gdyidao.cn/api';

const SHELL_IMPORTS = ['IMAGE_ALT_FALLBACK', 'expandLegacyImageMarkers', 'parseMarkdown', 'resolveFileUrl', 'SUBSET_AI_ANSWER', 'splitInlineRuns', 'inlineRunsPlainText'];
const SHELL_SRC = readText(SHELL_UTS);

const ANSWER = [
  '# 制动故障排查',
  '',
  '- 故障码：E01',
  '- 含义：电压过低',
  '',
  '> 先断主开关。',
  '',
  '```',
  'multimeter.measure(V)',
  '```',
].join('\n');

const TABLE = '| 故障码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |';
const MARKER = '先排空。<<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png | 描述:蓄能器接口>>再目视。';

/** 独行站内图 = 后端归一后的现行形状（root ADR-0091：白名单内改写成本站代理路径） */
const SITE_IMG = '![蓄能器](/api/ai-assistant/diagnosis/ossimg/linde/Linde_A-005-01.png)';
const SITE_IMG_ABS = API_BASE + '/ai-assistant/diagnosis/ossimg/linde/Linde_A-005-01.png';
/** 句内图：本档声明 inline ⇒ 解析器交出源串，壳层按图切段（#2366） */
const INLINE_SITE_IMG = '见 ![故障图](/api/ai-assistant/diagnosis/ossimg/a/b.png) 这张';
/** 站外图：后端本该降级成文字；客户端再兜一道（不猜地址、不去第三方取图） */
const FOREIGN_IMG = '![别家图](https://cdn.example.com/x/y.png)';

function markdownBindings() {
  const m = loadUts(MD_UTS, {});
  const names = importedNames(SHELL_SRC);
  const bind = {};
  for (const n of names) {
    // 解析器与档位常量出自 `markdown.uts`；展开器 / 占位词出自 `aiSourcesDisplay.uts`、
    // 拼接出自 `format.uts`、分词器出自 `markdownInline.uts` —— 各取真源，不留镜像。
    if (n === 'expandLegacyImageMarkers' || n === 'resolveFileUrl' || n === 'IMAGE_ALT_FALLBACK') continue;
    if (n === 'splitInlineRuns' || n === 'inlineRunsPlainText') continue;
    bind[n] = m[n];
  }
  return bind;
}

function shellBindings() {
  // SUBSET_CHAPTER 不在真 import 清单里，但**变异体**要用它（档位拿错的坏实现必须能跑起来，
  // 否则报 ReferenceError 而不是「判据红」—— 那测的是夹具环，不是牙齿）。先例：forumBodyBehavior。
  const m = loadUts(MD_UTS, {});
  const display = loadUts(DISPLAY_UTS, {});
  const inline = loadUts(INLINE_UTS, {});
  return Object.assign(markdownBindings(), {
    SUBSET_CHAPTER: m.SUBSET_CHAPTER,
    SUBSET_FORUM: m.SUBSET_FORUM,
    expandLegacyImageMarkers: display.expandLegacyImageMarkers,
    IMAGE_ALT_FALLBACK: display.IMAGE_ALT_FALLBACK,
    splitInlineRuns: inline.splitInlineRuns,
    inlineRunsPlainText: inline.inlineRunsPlainText,
    resolveFileUrl: loadUts(FORMAT_UTS, { API_BASE_URL: API_BASE }).resolveFileUrl,
  });
}

/** 每次取一个全新模块实例（模块级常量不可变，互不串） */
const shell = () => loadUts(SHELL_UTS, shellBindings());

/**
 * 气泡 `plain` 轴的**真实产出**（块文本已不再是终端形态）：记号由渲染层吞、图折 alt，
 * 这里等价于「用户看到什么字」。判「URL 不外露」必须看这条投影，只看块类型会漏掉整条 URL。
 */
const INLINE = loadUts(INLINE_UTS, {});
const plainOf = (text) => INLINE.inlineRunsPlainText(INLINE.splitInlineRuns(text));

describe('aiAnswerBody 壳层（真执行）', () => {
  it('注入面与真 import 一致（清单不漂第二份）', () => {
    const names = importedNames(SHELL_SRC).slice().sort();
    expect(names.length).toBeGreaterThan(0);   // 空清单=夹具坏掉（曾传路径进 importedNames，一处错全套件假根因）
    expect(names).toEqual(SHELL_IMPORTS.slice().sort());
  });

  it('助手回答按论坛档出块：标题/列表/引用/代码各成其块', () => {
    const blocks = shell().aiAnswerBlocks(ANSWER);
    expect(blocks.map((b) => b.type)).toEqual(['heading', 'list', 'quote', 'code']);
    expect(blocks[0].text).toBe('制动故障排查');
    expect(blocks[0].level).toBe(1);
    expect(blocks[1].items).toEqual(['故障码：E01', '含义：电压过低']);
  });

  it('#1279 前置生效：图片标记展开成说明文字，路径与记号都不进任何块', () => {
    const blocks = shell().aiAnswerBlocks(MARKER);
    const all = JSON.stringify(blocks);
    expect(all).toContain('蓄能器接口');
    expect(all).not.toContain('<<IMAGE');
    expect(all).not.toContain('manual/');
    expect(all).not.toContain('fault_images/');
    expect(all).not.toContain('1721219449286');
  });

  it('表格渲成 table 块（#2398），公式仍按论坛档降级（源码原样 —— 「仍可读」不是白说的）', () => {
    const blocks = shell().aiAnswerBlocks(TABLE);
    expect(blocks.map((b) => b.type)).toEqual(['table', 'table']);
    expect(blocks[0].items).toEqual(['故障码', '含义']);
    expect(blocks[1].items).toEqual(['E01', '电压过低']);
    // 表头行 level=1、正文行 level=0 —— 渲染层靠它区分表头样式（AC③ 的可见区分）
    expect(blocks.map((b) => b.level)).toEqual([1, 0]);
    const formula = shell().aiAnswerBlocks('扭矩 $M = F \\times d$ 超限。');
    expect(JSON.stringify(formula)).toContain('$M = F');
  });

  it('表内行内记号保留源串（#2398 AC④）：单元格存 `**粗**` / 行内码，由渲染层吞', () => {
    const blocks = shell().aiAnswerBlocks('| 项 | 说明 |\n| --- | --- |\n| **急停** | 按 `红色` 按钮 |');
    expect(blocks.map((b) => b.type)).toEqual(['table', 'table']);
    expect(blocks[1].items).toEqual(['**急停**', '按 `红色` 按钮']);
  });

  it('空回答 ⇒ 空数组（渲染出空气泡，与旧纯文本行为等价）', () => {
    expect(shell().aiAnswerBlocks('')).toEqual([]);
  });

  it('独行站内图 ⇒ image 块，且 text 是**绝对**本站 URL（拼接归 `resolveFileUrl`）', () => {
    const blocks = shell().aiAnswerBlocks(SITE_IMG);
    expect(blocks.map((b) => b.type)).toEqual(['image']);
    expect(blocks[0].text).toBe(SITE_IMG_ABS);
    expect(blocks[0].items).toEqual(['蓄能器']);        // alt 留着：图挂了仍读得出讲的是什么
    expect(blocks[0].text).not.toContain('cdn.example.com');
  });

  it('**绝对**站内图（已带基址）⇒ 同样是 image 块，URL 原样（归一幂等，两种入路同一条判据）', () => {
    const blocks = shell().aiAnswerBlocks('![' + '蓄能器' + '](' + SITE_IMG_ABS + ')');
    expect(blocks.map((b) => b.type)).toEqual(['image']);
    expect(blocks[0].text).toBe(SITE_IMG_ABS);
  });

  // ── #2366：句内图升块（按图切段，安全才切）──────────────────────────────

  it('句内站内图 ⇒ 按图切成三块（段落 / 图 / 段落），图块 text 是**绝对**本站 URL', () => {
    const blocks = shell().aiAnswerBlocks(INLINE_SITE_IMG);
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'image', 'paragraph']);
    expect(blocks[0].text).toBe('见');
    expect(blocks[1].text).toBe(API_BASE + '/ai-assistant/diagnosis/ossimg/a/b.png');
    expect(blocks[1].items).toEqual(['故障图']);   // alt 留着：图挂了仍读得出讲的是什么
    expect(blocks[2].text).toBe('这张');
    // 段与图各管一段：段里不再带标记，URL 只出现在图块
    expect(blocks[0].text + blocks[2].text).not.toContain('![');
    expect(blocks[0].text + blocks[2].text).not.toContain('/api/ai-assistant/');
  });

  it('一段两图 ⇒ 五块，两图各自成块、按出现顺序**不合并**', () => {
    const raw = '前 ![甲](/api/ai-assistant/diagnosis/ossimg/a.png) 中 ![乙](/api/ai-assistant/diagnosis/ossimg/b.png) 后';
    const blocks = shell().aiAnswerBlocks(raw);
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'image', 'paragraph', 'image', 'paragraph']);
    expect(blocks[1].text).toBe(API_BASE + '/ai-assistant/diagnosis/ossimg/a.png');
    expect(blocks[3].text).toBe(API_BASE + '/ai-assistant/diagnosis/ossimg/b.png');
    expect(blocks.map((b) => b.text).filter((t) => t === '甲' || t === '乙')).toEqual([]);   // 图不靠 alt 段落占位
  });

  it('边界① 记号跨图 ⇒ 整行不切，且图**折回 alt**（不切的行必须与声明 inline 之前同样不露 URL）', () => {
    // 五个格子覆盖粗体 / 链接 / 行内码 / 删除线 / 斜体包住图片；切了就会把未闭合记号落进段里。
    // 折 alt 之后的行再交分词器：「用户看到的字」必须与今日 `stripInlinePlain` 的投影同形。
    const cells = [
      ['**注意 ![图](/api/ai-assistant/diagnosis/ossimg/a.png) 安全**', '注意 图 安全'],
      ['[见 ![图](/api/ai-assistant/diagnosis/ossimg/a.png) 手册](https://e.com/a)', '见 图 手册'],
      ['`a ![图](/api/ai-assistant/diagnosis/ossimg/a.png) b`', 'a 图 b'],
      ['~~旧 ![图](/api/ai-assistant/diagnosis/ossimg/a.png) 旧~~', '旧 图 旧'],
      ['*务必 ![图](/api/ai-assistant/diagnosis/ossimg/a.png) 戴手套*', '务必 图 戴手套'],
    ];
    for (const [raw, today] of cells) {
      const blocks = shell().aiAnswerBlocks(raw);
      // 一行仍是一段：图不进块、URL 不进图集
      expect([raw, blocks.map((b) => b.type)]).toEqual([raw, ['paragraph']]);
      expect(shell().aiAnswerImageUrls(blocks)).toEqual([]);
      // 关键：**投影**里不许留 `![` / URL（链接与行内码跨图正是旧写法会把整段读成 URL 的两格）
      const shown = plainOf(blocks[0].text);
      expect([raw, shown]).toEqual([raw, today]);
      expect(shown).not.toContain('![');
      expect(shown).not.toContain('e.com');
      expect(shown).not.toContain('/api/ai-assistant/');
    }
  });

  it('边界① 的非段落块（列表项）同样折 alt：图标记不外露，URL 不进 runs', () => {
    const raw = '- 见 ![图](/api/ai-assistant/diagnosis/ossimg/a.png) 手册 [[手册](https://e.com/a)]';
    const blocks = shell().aiAnswerBlocks(raw);
    expect(blocks.map((b) => b.type)).toEqual(['list']);
    const shown = plainOf(blocks[0].items[0]);
    expect(shown).not.toContain('![');
    expect(shown).not.toContain('/api/ai-assistant/');
    expect(shown).toContain('图');
  });

  it('代码块里的 `![…](…)` 原样保留（代码内容不折 alt —— 折了就是改坏代码）', () => {
    const raw = '```\nif (x) { print("![图](/api/x.png)") }\n```';
    const blocks = shell().aiAnswerBlocks(raw);
    expect(blocks.map((b) => b.type)).toEqual(['code']);
    expect(blocks[0].text).toContain('![图](/api/x.png)');
  });

  it('边界② 空 alt ⇒ 不切（占位词 / alt 剥除路径继续，绝不产空 alt 图块）', () => {
    const blocks = shell().aiAnswerBlocks('前 ![](/api/ai-assistant/diagnosis/ossimg/a.png) 后');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph']);
    expect(shell().aiAnswerImageUrls(blocks)).toEqual([]);
    // 与声明 inline 之前同形：图标记被剥净（块级 `IMAGE_ALT_FALLBACK` 只在独行图那条路上）
    expect(plainOf(blocks[0].text)).toBe('前  后');
  });

  it('边界③ 一行多图里一张不安全 ⇒ **整行**不切（逐行取交集，不是逐图取）', () => {
    const raw = '前 ![甲](/api/ai-assistant/diagnosis/ossimg/a.png) 中 `a ![乙](/api/ai-assistant/diagnosis/ossimg/b.png) b` 后';
    const blocks = shell().aiAnswerBlocks(raw);
    expect(blocks.map((b) => b.type)).toEqual(['paragraph']);
    expect(shell().aiAnswerImageUrls(blocks)).toEqual([]);
    // 两张图都折 alt：一张变块、另一张留行内就是「逐图取」的坏形态
    const shown = plainOf(blocks[0].text);
    expect(shown).not.toContain('![');
    expect(shown).not.toContain('/api/ai-assistant/');
    expect(shown).toContain('甲');
    expect(shown).toContain('乙');
  });

  it('句内**站外**图 ⇒ 图位降级成 alt 段落、绝不产 image 块（安全切分照走，站内闸独立生效）', () => {
    const blocks = shell().aiAnswerBlocks('见 ![别家图](https://cdn.example.com/x/y.png) 这张');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'paragraph', 'paragraph']);
    expect(blocks[1].text).toBe('别家图');
    expect(JSON.stringify(blocks)).not.toContain('cdn.example.com');
  });

  it('站外图 ⇒ 退化成 alt 文字，绝不产 image 块（客户端不猜地址、不去第三方取图）', () => {
    const blocks = shell().aiAnswerBlocks(FOREIGN_IMG);
    expect(blocks.map((b) => b.type)).not.toContain('image');
    expect(blocks[0].text).toBe('别家图');
    expect(JSON.stringify(blocks)).not.toContain('cdn.example.com');
  });

  it('站外图且 alt 为空 ⇒ 退回占位词「诊断配图」，**不丢内容也不留 URL**（段落地板）', () => {
    const blocks = shell().aiAnswerBlocks('![](https://cdn.example.com/x/y.png)');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph']);
    expect(blocks[0].text).toBe('诊断配图');
    expect(JSON.stringify(blocks)).not.toContain('cdn.example.com');
  });

  it('图集与下标换算（纯函数）：按块序收集；同图重复出现时第二张的下标是 1', () => {
    const s = shell();
    const blocks = s.aiAnswerBlocks([SITE_IMG, '中间一段话', SITE_IMG, FOREIGN_IMG].join('\n\n'));
    expect(s.aiAnswerImageUrls(blocks)).toEqual([SITE_IMG_ABS, SITE_IMG_ABS]);
    // 第二张图的块序号 → 图序号必须是 1（按 URL 值反查会得到 0，预览会从第一张翻起）
    const secondImageBlock = blocks.length - 2;
    expect(blocks[secondImageBlock].type).toBe('image');
    expect(s.aiAnswerImageIndexAt(blocks, secondImageBlock)).toBe(1);
    expect(s.aiAnswerImageIndexAt(blocks, 0)).toBe(0);
  });

  it('站内 + 站外同屏：只画站内那张，站外那张退成文字，顺序不变', () => {
    const blocks = shell().aiAnswerBlocks([SITE_IMG, FOREIGN_IMG].join('\n\n'));
    expect(blocks.map((b) => b.type)).toEqual(['image', 'paragraph']);
    expect(blocks[1].text).toBe('别家图');
  });

  // ── #2362 / 移动端 ADR 0056：配图可见（只走站内代理出口）──────────────────────

});

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  /** 读真源 → 注入变异 → 落临时目录（锚点失效即红：变异必须真的进去了） */
  function mutatedCopy(replacements) {
    let src = SHELL_SRC;
    for (const [from, to] of replacements) {
      expect([from, src.includes(from)]).toEqual([from, true]);
      src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-answer-body-'));
    const file = path.join(dir, path.basename(SHELL_UTS));
    fs.writeFileSync(file, src);
    return file;
  }
  const loadMutated = (replacements) => loadUts(mutatedCopy(replacements), shellBindings());

  it('必红 · 摘掉历史标记的展开前置 ⇒ 内网路径泄漏判据必然判红（百分号内网路径进正文）', () => {
    const broken = loadMutated([['expandLegacyImageMarkers(raw)', 'raw']]);
    const all = JSON.stringify(broken.aiAnswerBlocks(MARKER));
    // 展开被摘后：标记与路径原样留在段落文本里 —— 正是 #1279 挡的那件事
    expect(all).toContain('fault_images/');
  });

  it('必红 · 档位拿错（AI 回答档→论坛档）⇒ 表格退回逐行段落，「表格渲成原生表格」判据必然判红', () => {
    // #2398 起 AI 档与章节档**都**声明 table，故变异目标换成论坛档（仍不声明 table）——
    // 拿错档位的真实后果就是「表格又变回竖线原文」，这正是本票要挡的那件事。
    const broken = loadMutated([['parseMarkdown(expandLegacyImageMarkers(raw), SUBSET_AI_ANSWER)', "parseMarkdown(expandLegacyImageMarkers(raw), 'forum')"]]);
    expect(broken.aiAnswerBlocks(TABLE).some((b) => b.type === 'table')).toBe(false);
  });

  it('必红 · 摘掉站内前缀闸 ⇒ 站外图变成 image 块，图片请求直连第三方主机', () => {
    // 坏实现 = 完全信任后端归一、客户端不设闸（ADR-0063 决策 3 的原始动机在这里失守）
    // 变异成空前缀 = 任何 URL 都放行（`startsWith('')` 恒真），等于这道闸不存在
    const broken = loadMutated([["'/api/ai-assistant/diagnosis/ossimg/'", "''"]]);
    expect(broken.aiAnswerBlocks(FOREIGN_IMG).some((b) => b.type === 'image')).toBe(true);
  });
});
