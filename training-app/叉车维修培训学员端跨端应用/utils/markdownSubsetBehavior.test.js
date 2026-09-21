/**
 * 论坛正文子集（第三档 `SUBSET_FORUM`）· **行为级**测试（ADR-0025 决策⑤/⑥，票 #1240 P1）
 *
 * 为什么是行为而不是源码文本：本票的验收标准写的是「**子集可枚举且被守护**」——
 * 判据落在**跑起来的解析器**上，而不是「某个字面量出现过」：
 *   - **声明即行为**：给某档的声明表加一行而 `parseMarkdown` 产不出对应块 ⇒ 对账用例必须判红；
 *   - **降级可读**（根 `CONTEXT.md` 的硬口径）：未被本档声明的语法只能退回**可见的段落文本**
 *     （表格退回逐行原始文本、mermaid 退回围栏源码），绝不允许「声明里没有 ⇒ 内容消失」。
 * 纯源码契约（签名 / 常量 / 锚点）仍由 `markdownContract.test.js` 守，两者分工见 `docs/agents/guards.md`。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 把 `.uts` 去类型后当 JS 真执行（先例
 * `utils/recruitDisplayBehavior.test.js` / `utils/concurrent401RefreshBehavior.test.js`）。
 * `utils/markdown.uts` 只有 `import type`，故注入**空绑定**即可 —— 本文件末组把这条也钉住。
 *
 * **成对取证**（③ 门判据③）：末组用**注入变异的副本**证明上面两组对账判据**有牙**——
 * 变异不是「换个写法」，而是两种真实的坏实现：① 解析器无视声明照样出块；② 声明里加了一行
 * 解析器根本没有的成员。两者都必须让判据判红，否则「可枚举且被守护」只是纸面说法。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const MD_UTS = path.join(__dirname, 'markdown.uts');

/** 每次取一个**全新模块实例**（模块级声明表是常量，互不串） */
const md = () => loadUts(MD_UTS, {});

const TABLE = '| 故障码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |';
const MERMAID = '```mermaid\ngraph TD;\nA-->B;\n```';

/**
 * 探针：声明表里的每个成员 → 一段**只用该语法**的 markdown + 它该产出的块类型。
 * 新成员进了声明表却没进这张表 ⇒ 正向对账组判红（「声明了但没人知道它该产出什么」）。
 */
const PROBES = {
  heading: { md: '### 三级标题', type: 'heading' },
  list: { md: '- 项一\n- 项二', type: 'list' },
  quote: { md: '> 注意安全', type: 'quote' },
  code: { md: '```\nconst a = 1\n```', type: 'code' },
  divider: { md: '---', type: 'divider' },
  image: { md: '![故障图](https://e.com/a.png)', type: 'image' },
  table: { md: TABLE, type: 'table' },
};

const SUBSETS = ['chapter', 'featured', 'forum'];
const types = (blocks) => blocks.map((b) => b.type);
const declared = (m, subset) => m.subsetMembers(subset);

// ===== ① 声明表本身：可枚举、按档取值 =====

describe('子集声明表：三档可枚举，未知档退到章节档', () => {
  it('`SUBSET_FORUM` 是第三档，且三档名字互不相同', () => {
    const m = md();
    expect(m.SUBSET_FORUM).toBe('forum');
    expect(new Set([m.SUBSET_CHAPTER, m.SUBSET_FEATURED, m.SUBSET_FORUM]).size).toBe(3);
  });

  it('章节档声明含表格；内容精选与论坛两档都不含（各自的理由见 ADR-0046 / ADR-0025）', () => {
    const m = md();
    expect(declared(m, m.SUBSET_CHAPTER)).toContain(m.MEMBER_TABLE);
    expect(declared(m, m.SUBSET_FEATURED)).not.toContain(m.MEMBER_TABLE);
    expect(declared(m, m.SUBSET_FORUM)).not.toContain(m.MEMBER_TABLE);
  });

  it('未知档位退到章节档 —— 与 `parseMarkdown` 的缺省档同向（否则「缺省」在两处含义不同）', () => {
    const m = md();
    expect(declared(m, 'unknown-subset')).toEqual(declared(m, m.SUBSET_CHAPTER));
    expect(declared(m, '')).toEqual(declared(m, m.SUBSET_CHAPTER));
    // 缺省实参也确实走章节档
    expect(types(m.parseMarkdown(TABLE))).toEqual(types(m.parseMarkdown(TABLE, m.SUBSET_CHAPTER)));
  });

  it('`subsetHasMember` 与声明表同源（不是第二份判断）', () => {
    const m = md();
    for (const subset of SUBSETS) {
      for (const member of Object.keys(PROBES)) {
        expect([subset, member, m.subsetHasMember(subset, member)])
          .toEqual([subset, member, declared(m, subset).includes(member)]);
      }
    }
  });
});

// ===== ② 行为对账（正向）：声明了的，必须真出对应块 =====

describe('行为对账 · 正向：声明表里有的成员，解析器必须真出对应块', () => {
  for (const subset of SUBSETS) {
    describe(`${subset} 档`, () => {
      const m = md();
      for (const member of declared(m, subset)) {
        it(`成员 ${member} 有探针，且探针真产出 ${member} 块`, () => {
          // 声明了一行却没人知道它该产出什么 ⇒ 这里判红（「声明即行为」的入口）
          expect([member, typeof PROBES[member]]).toEqual([member, 'object']);
          expect(types(m.parseMarkdown(PROBES[member].md, subset))).toContain(PROBES[member].type);
        });
      }
    });
  }
});

// ===== ③ 行为对账（反向）+ 降级可读：没声明的，不出块但内容仍可见 =====

describe('行为对账 · 反向：未声明的成员不得出块，且内容退回可见文本（降级可读）', () => {
  for (const subset of SUBSETS) {
    const m = md();
    const missing = Object.keys(PROBES).filter((member) => !declared(m, subset).includes(member));
    it(`${subset} 档：未声明成员 ${missing.join('/') || '(无)'} 一律不出对应块`, () => {
      for (const member of missing) {
        const blocks = m.parseMarkdown(PROBES[member].md, subset);
        expect([subset, member, types(blocks).includes(PROBES[member].type)])
          .toEqual([subset, member, false]);
      }
    });

    it(`${subset} 档：未声明的输入仍**可见**（段落地板承重，不许静默丢内容）`, () => {
      for (const member of missing) {
        const blocks = m.parseMarkdown(PROBES[member].md, subset);
        expect([subset, member, blocks.length > 0]).toEqual([subset, member, true]);
      }
    });
  }

  it('论坛档的表格退回**逐行原始文本**（与内容精选同口径；Web featuredMarked 亦然）', () => {
    const m = md();
    const blocks = m.parseMarkdown(TABLE, m.SUBSET_FORUM);
    expect(types(blocks)).toEqual(['paragraph', 'paragraph', 'paragraph']);
    expect(blocks[0].text).toBe('| 故障码 | 含义 |');
    expect(blocks[2].text).toBe('| E01 | 电压过低 |');
  });

  it('论坛档的 mermaid 退回**围栏源码**（ADR-0025 ③ 具名登记的唯一真降级）', () => {
    const m = md();
    const blocks = m.parseMarkdown(MERMAID, m.SUBSET_FORUM);
    expect(types(blocks)).toEqual(['code']);
    expect(blocks[0].text).toContain('graph TD;');
    expect(blocks[0].text).toContain('A-->B;');
  });

  it('论坛档与内容精选档**块集接近但不等同、且是各自的声明**（按端分档：允许不同、各自成文）', () => {
    const m = md();
    // 两者都不含 table（各自的理由不同）；论坛档另少一个 image（图文分离，与 Web 同口径）
    const forum = declared(m, m.SUBSET_FORUM);
    const featured = declared(m, m.SUBSET_FEATURED);
    expect(forum).not.toContain(m.MEMBER_TABLE);
    expect(featured).not.toContain(m.MEMBER_TABLE);
    expect(featured.filter((x) => !forum.includes(x))).toEqual([m.MEMBER_IMAGE]);
    expect(forum.filter((x) => !featured.includes(x))).toEqual([]);
    expect(m.SUBSET_FORUM).not.toBe(m.SUBSET_FEATURED);
  });

  it('论坛档的正文内嵌图片退回 **alt 文本**（图文分离；与 Web「`![]()` 展开成 alt」同口径）', () => {
    const m = md();
    const blocks = m.parseMarkdown('见 ![故障图](https://e.com/a.png) 这张', m.SUBSET_FORUM);
    expect(types(blocks)).not.toContain('image');
    expect(blocks[0].text).toBe('见 故障图 这张');
  });
});

// ===== ④ 零回归：既有两档的行为没被这次改造动过 =====

describe('零回归（ADR-0025 ⑤）：章节档与内容精选档的行为逐条不变', () => {
  it('章节档仍是唯一出表格块的档，且表头/正文 level 口径不变', () => {
    const m = md();
    const blocks = m.parseMarkdown(TABLE, m.SUBSET_CHAPTER);
    expect(types(blocks)).toEqual(['table', 'table']);
    expect(blocks.map((b) => b.level)).toEqual([m.TABLE_LEVEL_HEAD, m.TABLE_LEVEL_BODY]);
    expect(blocks[0].items).toEqual(['故障码', '含义']);
    expect(blocks.map((b) => b.text)).toEqual(['故障码 含义', 'E01 电压过低']);
  });

  it('公式在两档都降级为源码且原样可见（含下标不被斜体规则撕碎）', () => {
    const m = md();
    expect(m.parseMarkdown('公式 $a_1 + b_2$ 里的下标', m.SUBSET_CHAPTER)[0].text).toBe('公式 $a_1 + b_2$ 里的下标');
    expect(m.parseMarkdown('公式 $a_1 + b_2$ 里的下标', m.SUBSET_FORUM)[0].text).toBe('公式 $a_1 + b_2$ 里的下标');
  });

  it('代码块里的表格语法仍是代码（不被表格分支抢走）', () => {
    const m = md();
    expect(types(m.parseMarkdown('```\n| A | B |\n| --- | --- |\n```', m.SUBSET_CHAPTER))).toEqual(['code']);
  });
});

// ===== ⑤ 成对取证：注入变异后，上面两组判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树，不进仓） */
function loadMutated(replacements) {
  let src = readText(MD_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'md-subset-'));
  const file = path.join(dir, 'markdown.uts');
  fs.writeFileSync(file, src);
  return loadUts(file, {});
}

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红：真源上，论坛档不出表格块（对照组）', () => {
    const m = md();
    expect(types(m.parseMarkdown(TABLE, m.SUBSET_FORUM))).not.toContain('table');
  });

  it('必红 · 解析器无视声明：拆掉 table 闸门后，论坛档照样出表格块 ⇒ 反向对账判据必然判红', () => {
    const broken = loadMutated([['hasMember(MEMBER_TABLE) && ', '']]);
    // 声明没变（论坛档仍不含 table），而解析器已经照着「旧黑名单式」出块了
    expect(declared(broken, broken.SUBSET_FORUM)).not.toContain(broken.MEMBER_TABLE);
    expect(types(broken.parseMarkdown(TABLE, broken.SUBSET_FORUM))).toContain('table');
  });

  it('必红 · 声明多了一行：往论坛档声明里加一个解析器根本没有的成员 ⇒ 正向对账判据必然判红', () => {
    const broken = loadMutated([
      ['export const SUBSET_MEMBERS_FORUM : string[] = [', "export const SUBSET_MEMBERS_FORUM : string[] = ['footnote',"],
    ]);
    expect(declared(broken, broken.SUBSET_FORUM)).toContain('footnote');   // 声明说它有
    expect(PROBES['footnote']).toBeUndefined();                            // 却没人知道它该产出什么
    expect(types(broken.parseMarkdown('正文[^1]\n\n[^1]: 注文', broken.SUBSET_FORUM))).not.toContain('footnote');
  });
});

// ===== ⑥ 模块契约：零运行期依赖（loadUts 注入空绑定即可跑）=====

describe('模块契约：`utils/markdown.uts` 零运行期依赖', () => {
  it('空绑定即可载入（只有 `import type`，没有运行期 import）', () => {
    expect(() => loadUts(MD_UTS, {})).not.toThrow();
  });

  it('每次载入都是新实例，声明表不是可变共享状态', () => {
    const a = md();
    const b = md();
    expect(a.SUBSET_MEMBERS_FORUM).toEqual(b.SUBSET_MEMBERS_FORUM);
    expect(a.SUBSET_MEMBERS_FORUM).not.toBe(b.SUBSET_MEMBERS_FORUM);
  });
});
