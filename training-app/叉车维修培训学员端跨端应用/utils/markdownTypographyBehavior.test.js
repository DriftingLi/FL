/**
 * 阅读刻度与排版解析器（#2352）—— **行为腿**
 *
 * 判据落在**跑起来的产出物**上（`loadUts` 真执行 `utils/markdownTypography.uts`）：
 * 「给定块序列 → 每块的档位名与间距档名」。字号 / 行高 / 字色的**数值**不在本文件射程 ——
 * #2352 决议 D2 把数值收在 `uni.scss` 的 `$md-*` 段一处，这里断的是**令牌**
 * （`mt-16` 而不是 16rpx）。数值面由 #2358 的文本守卫钉「字面量只出现在刻度段」，
 * 观感由 #2353 的章节屏几何读数验收。
 *
 * 成对取证（③ 门判据③）：末组用**注入变异的副本**证明上面的判据有牙 —— 变异是两种真实的
 * 坏实现（间距表写错一行 / 标题映射降一档），做法沿用 `utils/markdownSubsetBehavior.test.js`：
 * 读真源 → `split/join` 注入 → 落临时目录 → `loadUts` 重跑，且每条变异带 `includes` 锚点，
 * 锚点失效即红（否则「必红」是假的）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const TYPO_UTS = path.join(__dirname, 'markdownTypography.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');

/** 每次取一个全新模块实例（模块内全是常量，互不串） */
const typo = () => loadUts(TYPO_UTS, {});
/** 真源解析器：本函数的入参就是它的产出（不是测试自造的另一种块形状） */
const md = () => loadUts(MD_UTS, {});

/** 块工厂：四个字段齐全，与 `MarkdownBlock` 同形 */
function blk(type, level = 0, text = '', items = []) {
  return { type, level, text, items };
}

/** 只取间距档序列（读起来比逐项 `.space` 清楚） */
const spaces = (blocks) => typo().markdownTiers(blocks).map((t) => t.space);
/** 只取字号档序列 */
const sizes = (blocks) => typo().markdownTiers(blocks).map((t) => t.size);

describe('形状：返回值与输入 1:1 对齐，且只有两个字符串字段', () => {
  it('输入 N 块 ⇒ 输出 N 项，顺序与输入一致', () => {
    const blocks = [blk('heading', 1), blk('paragraph'), blk('divider'), blk('list')];
    expect(typo().markdownTiers(blocks)).toHaveLength(blocks.length);
  });

  it('每项只有 size / space 两个键，值都落在令牌表里（不含数值、不含第三个字段）', () => {
    const tiers = typo().markdownTiers([blk('heading', 2), blk('paragraph'), blk('code'), blk('list'), blk('divider')]);
    for (const t of tiers) {
      expect(Object.keys(t).sort()).toEqual(['size', 'space']);
      expect(typeof t.size).toBe('string');
      expect(typeof t.space).toBe('string');
      // 令牌表白名单：数值（40 / '40rpx'）或第三个字段一旦混进来，这条先红
      expect(t.size).toMatch(/^(h[1-4]|body|quote|code|listText|divider)$/);
      expect(t.space).toMatch(/^mt-(0|16|24|40)$/);
    }
  });

  it('空输入 ⇒ 空数组（不特判、不造块）', () => {
    expect(typo().markdownTiers([])).toEqual([]);
  });

  it('只读不写：不改入参、同一输入两次调用结果一致', () => {
    const blocks = [blk('heading', 3), blk('quote'), blk('divider'), blk('paragraph')];
    const snapshot = JSON.stringify(blocks);
    const first = typo().markdownTiers(blocks);
    const second = typo().markdownTiers(blocks);
    expect(JSON.stringify(blocks)).toBe(snapshot);
    expect(second).toEqual(first);
  });
});

describe('字号档：块类型 + 标题层级 ⇒ 档位', () => {
  it('h1–h6 收成四档：1→h1 / 2→h2 / 3→h3 / 4、5、6→h4', () => {
    const blocks = [1, 2, 3, 4, 5, 6].map((lv) => blk('heading', lv));
    expect(sizes(blocks)).toEqual(['h1', 'h2', 'h3', 'h4', 'h4', 'h4']);
  });

  it('层级越界不炸也不漏：level 0 / 负数折到 h1，7 及以上折到 h4', () => {
    expect(sizes([blk('heading', 0), blk('heading', -1), blk('heading', 7)])).toEqual(['h1', 'h1', 'h4']);
  });

  it('引用 / 代码 / 段落 / 列表 / 分隔线各有自己的档', () => {
    expect(sizes([blk('quote'), blk('code'), blk('paragraph'), blk('list'), blk('divider')]))
      .toEqual(['quote', 'code', 'body', 'listText', 'divider']);
  });

  it('本轮不定义排版的块（table / image）与未知类型折到正文档', () => {
    expect(sizes([blk('table', 1), blk('image'), blk('未来才有的块')])).toEqual(['body', 'body', 'body']);
  });
});

describe('间距优先序（#2352 决议 D3「首块与标题优先」）', () => {
  it('首块恒 mt-0 —— 即使它就是分隔线', () => {
    expect(spaces([blk('divider'), blk('paragraph')])).toEqual(['mt-0', 'mt-16']);
    expect(spaces([blk('heading', 1)])).toEqual(['mt-0']);
  });

  it('标题块 mt-40；标题紧跟标题 mt-24', () => {
    expect(spaces([blk('paragraph'), blk('heading', 2), blk('heading', 3)]))
      .toEqual(['mt-0', 'mt-40', 'mt-24']);
  });

  it('紧跟标题的块 mt-16；再下一块回到 mt-24', () => {
    expect(spaces([blk('heading', 2), blk('paragraph'), blk('paragraph')]))
      .toEqual(['mt-0', 'mt-16', 'mt-24']);
  });

  it('分隔线自身 mt-16、其后非标题块 mt-16（前后合计 32 是两条 16，不是叠加出来的）', () => {
    expect(spaces([blk('paragraph'), blk('divider'), blk('paragraph')]))
      .toEqual(['mt-0', 'mt-16', 'mt-16']);
  });

  it('分隔线紧跟分隔线不叠加（仍是 16，不会长成 32）', () => {
    expect(spaces([blk('paragraph'), blk('divider'), blk('divider')]))
      .toEqual(['mt-0', 'mt-16', 'mt-16']);
  });

  it('标题紧跟分隔线仍 mt-40 —— D3 选 A 而非 B 的那一格（标题的呼吸优先）', () => {
    expect(spaces([blk('paragraph'), blk('divider'), blk('heading', 2)]))
      .toEqual(['mt-0', 'mt-16', 'mt-40']);
  });

  it('其余块 mt-24', () => {
    expect(spaces([blk('paragraph'), blk('paragraph'), blk('quote')]))
      .toEqual(['mt-0', 'mt-24', 'mt-24']);
  });

  it('连续 table 行：首行取四态（紧跟标题 16 / 否则 24），续行 mt-0', () => {
    expect(spaces([blk('heading', 2), blk('table', 1), blk('table', 0), blk('table', 0)]))
      .toEqual(['mt-0', 'mt-16', 'mt-0', 'mt-0']);
    expect(spaces([blk('paragraph'), blk('table', 1), blk('table', 0)]))
      .toEqual(['mt-0', 'mt-24', 'mt-0']);
  });

  it('表格后接段落回到常规间距（表尾不悬空）', () => {
    expect(spaces([blk('table', 1), blk('table', 0), blk('paragraph')]))
      .toEqual(['mt-0', 'mt-0', 'mt-24']);
  });
});

describe('真链路：既有解析器的产出直接喂进来', () => {
  it('标题 / 段落 / 分隔线 / 标题 / 段落 ⇒ mt-0, 16, 16, 40, 16（八行表里第 1/6/4/3 行各命中一次）', () => {
    const blocks = md().parseMarkdown('# 一级标题\n\n正文一\n\n---\n\n## 二级标题\n\n正文二');
    expect(blocks.map((b) => b.type)).toEqual(['heading', 'paragraph', 'divider', 'heading', 'paragraph']);
    expect(typo().markdownTiers(blocks).map((t) => t.space))
      .toEqual(['mt-0', 'mt-16', 'mt-16', 'mt-40', 'mt-16']);
    expect(sizes(blocks)).toEqual(['h1', 'body', 'divider', 'h2', 'body']);
  });

  it('章节档的表格按行出块：表头行取四态、续行归零（ADR-0046 的归零行为保住）', () => {
    const blocks = md().parseMarkdown('正文\n\n| 码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'table', 'table']);
    expect(spaces(blocks)).toEqual(['mt-0', 'mt-24', 'mt-0']);
  });
});

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  /** 读真源 → 注入变异 → 落临时目录（锚点失效即红：变异必须真的进去了） */
  function mutatedCopy(replacements) {
    let src = readText(TYPO_UTS);
    for (const [from, to] of replacements) {
      expect([from, src.includes(from)]).toEqual([from, true]);
      src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'md-typography-'));
    const file = path.join(dir, path.basename(TYPO_UTS));
    fs.writeFileSync(file, src);
    return file;
  }
  const loadMutated = (replacements) => loadUts(mutatedCopy(replacements), {});

  it('必不红 · 原样副本（不注入变异）与真源同结果 —— 红不是临时目录带来的', () => {
    const same = loadMutated([]);
    const probe = [blk('paragraph'), blk('divider'), blk('heading', 2), blk('paragraph')];
    expect(same.markdownTiers(probe).map((t) => t.space))
      .toEqual(['mt-0', 'mt-16', 'mt-40', 'mt-16']);
  });

  it('必红 · 间距表写错一行（分隔线自身 16 → 24）⇒「分隔线自身 mt-16」判据必然判红', () => {
    const broken = loadMutated([
      ['if (block.type == TYPE_DIVIDER) return SPACE_TIGHT', 'if (block.type == TYPE_DIVIDER) return SPACE_BLOCK'],
    ]);
    const got = broken.markdownTiers([blk('paragraph'), blk('divider')]).map((t) => t.space);
    expect(got).toEqual(['mt-0', 'mt-24']); // 真实现是 ['mt-0', 'mt-16']
  });

  it('必红 · 标题映射降一档（h2 → h3）⇒「h2 档」判据必然判红', () => {
    const broken = loadMutated([
      ['if (block.level == 2) return SIZE_H2', 'if (block.level == 2) return SIZE_H3'],
    ]);
    expect(broken.markdownTiers([blk('heading', 2)]).map((t) => t.size)).toEqual(['h3']); // 真实现是 ['h2']
  });
});
