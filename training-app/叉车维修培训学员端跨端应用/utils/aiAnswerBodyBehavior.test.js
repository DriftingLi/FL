/**
 * AI 助手回答的渲染壳层（`utils/aiAnswerBody.uts`）· **行为级**测试（#1443，ADR-0046 决定 6 回填面）
 *
 * 为什么是行为而不是源码文本（`utils/forumBodyBehavior.test.js` 同判据）：#1443 的验收标准是
 * 「助手回答里的 Markdown 记号不再以源串示人」—— 这条只有**跑起来看产出块**才算数。断言
 * `.uvue` 里出现过 `aiAnswerBody` 是接线守护，壳层被改坏（档位拿错、展开被摘）它照样绿
 * （判据与分类见 `docs/agents/guards.md`）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`。注入的**各层都是真执行出来的源**——解析器与档位常量
 * 取 `utils/markdown.uts`、`expandLegacyImageMarkers` 取 `utils/aiSourcesDisplay.uts`、
 * `resolveFileUrl` 取 `utils/format.uts`——不是手抄镜像；镜像会让「真源改了而镜子没改」静默变绿
 * （#1273 先例：三层链只换坏的那层、其余取真源）。
 *
 * #2362 追加判据（移动端 ADR 0056）：独行站内图 → `image` 块（text 为**绝对**本站 URL）；
 * 句内图 → 只留 alt（未声明 inline ⇒ 解析器剥掉语法）；站外图 → 退化成 alt 文字、绝不产 image 块。
 *
 * 成对取证（③ 判据③）：末组用注入变异的副本证明上面的判据**有牙**，两种真实坏实现——
 * ① 摘掉 #1279 的展开前置（百分号编码的内网路径进正文）；② 档位拿错（论坛档换成章节档，
 * 表格被渲染出来 —— 与「两面同档」的裁定直接冲突）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, importedNames, readText } = require('./utsHarness');

const SHELL_UTS = path.join(__dirname, 'aiAnswerBody.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');
const DISPLAY_UTS = path.join(__dirname, 'aiSourcesDisplay.uts');

/** 三层真源里壳层需要的名字（importedNames 收**源码串**不收路径 —— 传错会静默得空清单，
 *  连带把注入绑定掏空，一处错让本套件全红且看不出根因） */
const FORMAT_UTS = path.join(__dirname, 'format.uts');

/** 站内基址只在这一处夹具出现：拼接口径归 `utils/format.resolveFileUrl`，本套件不重复实现它 */
const API_BASE = 'https://api.gdyidao.cn/api';

const SHELL_IMPORTS = ['IMAGE_ALT_FALLBACK', 'expandLegacyImageMarkers', 'parseMarkdown', 'resolveFileUrl', 'SUBSET_AI_ANSWER'];
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
/** 句内图：未声明 inline 的档要在解析器里被剥成 alt（URL 不外露） */
const INLINE_SITE_IMG = '见 ![故障图](/api/ai-assistant/diagnosis/ossimg/a/b.png) 这张';
/** 站外图：后端本该降级成文字；客户端再兜一道（不猜地址、不去第三方取图） */
const FOREIGN_IMG = '![别家图](https://cdn.example.com/x/y.png)';

function markdownBindings() {
  const m = loadUts(MD_UTS, {});
  const names = importedNames(SHELL_SRC);
  const bind = {};
  for (const n of names) {
    // 壳层 import 的五名里只有解析器与档位常量出自 `markdown.uts`；
    // 展开器与占位词出自 `aiSourcesDisplay.uts`、拼接出自 `format.uts` —— 各取真源，不留镜像。
    if (n === 'expandLegacyImageMarkers' || n === 'resolveFileUrl' || n === 'IMAGE_ALT_FALLBACK') continue;
    bind[n] = m[n];
  }
  return bind;
}

function shellBindings() {
  // SUBSET_CHAPTER 不在真 import 清单里，但**变异体**要用它（档位拿错的坏实现必须能跑起来，
  // 否则报 ReferenceError 而不是「判据红」—— 那测的是夹具环，不是牙齿）。先例：forumBodyBehavior。
  const m = loadUts(MD_UTS, {});
  const display = loadUts(DISPLAY_UTS, {});
  return Object.assign(markdownBindings(), {
    SUBSET_CHAPTER: m.SUBSET_CHAPTER,
    SUBSET_FORUM: m.SUBSET_FORUM,
    expandLegacyImageMarkers: display.expandLegacyImageMarkers,
    IMAGE_ALT_FALLBACK: display.IMAGE_ALT_FALLBACK,
    resolveFileUrl: loadUts(FORMAT_UTS, { API_BASE_URL: API_BASE }).resolveFileUrl,
  });
}

/** 每次取一个全新模块实例（模块级常量不可变，互不串） */
const shell = () => loadUts(SHELL_UTS, shellBindings());

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

  it('表格与公式保持论坛档降级（按行段落、源码原样 —— 「仍可读」不是白说的）', () => {
    const blocks = shell().aiAnswerBlocks(TABLE);
    expect(blocks.every((b) => b.type === 'paragraph')).toBe(true);
    expect(JSON.stringify(blocks)).toContain('|');
    const formula = shell().aiAnswerBlocks('扭矩 $M = F \\times d$ 超限。');
    expect(JSON.stringify(formula)).toContain('$M = F');
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

  it('句内站内图 ⇒ 只留 alt（未声明 inline ⇒ 解析器剥语法，URL 不出现在任何块里）', () => {
    const blocks = shell().aiAnswerBlocks(INLINE_SITE_IMG);
    expect(blocks.map((b) => b.type)).toEqual(['paragraph']);
    expect(blocks[0].text).toBe('见 故障图 这张');
    expect(JSON.stringify(blocks)).not.toContain('/api/ai-assistant/');
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

  it('必红 · 档位拿错（AI 回答档→章节档）⇒ 表格被渲成 table 块，「窄气泡不画表格」判据必然判红', () => {
    const broken = loadMutated([['parseMarkdown(expandLegacyImageMarkers(raw), SUBSET_AI_ANSWER)', 'parseMarkdown(expandLegacyImageMarkers(raw), SUBSET_CHAPTER)']]);
    expect(broken.aiAnswerBlocks(TABLE).some((b) => b.type === 'table')).toBe(true);
  });

  it('必红 · 摘掉站内前缀闸 ⇒ 站外图变成 image 块，图片请求直连第三方主机', () => {
    // 坏实现 = 完全信任后端归一、客户端不设闸（ADR-0063 决策 3 的原始动机在这里失守）
    // 变异成空前缀 = 任何 URL 都放行（`startsWith('')` 恒真），等于这道闸不存在
    const broken = loadMutated([["'/api/ai-assistant/diagnosis/ossimg/'", "''"]]);
    expect(broken.aiAnswerBlocks(FOREIGN_IMG).some((b) => b.type === 'image')).toBe(true);
  });
});
