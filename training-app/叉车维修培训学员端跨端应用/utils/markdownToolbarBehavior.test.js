/**
 * 论坛输入区工具栏 · **行为级**测试（ADR-0025 ⑥-5 / ⑥-6，票 #1240 P3）
 *
 * 本套件要机检的是 P3 那条**硬约束**（根 ADR-0052 决策 3、ADR-0025 ④ 的「文案取自 P1 声明」）：
 *
 *   > 工具栏按钮集 = 已声明子集 —— **按钮承诺的语法必须真能渲染出来**。
 *
 * 这条只有**跑起来看产出物**才算数：断言 `.uvue` 里出现过 `SUBSET_FORUM` 是接线守护，
 * 加一枚「加粗」按钮时它照样绿（`docs/agents/guards.md`：接线守护不构成 ③ 门证据）。故本套件
 * 跑两件真东西：`utils/markdownToolbar.uts`（按钮模型 + 插入逻辑）与 `utils/markdown.uts`
 * （**真执行的解析器**，不是手抄镜像 —— ③ 判据②「它测的是该测的那一支吗」由这条保证）。
 *
 * 三条判据：
 *   ① **按钮 ⊆ 声明**：每枚按钮的成员必须在论坛档的成员声明表里。移动端本档**不做行内渲染**
 *      （`**粗**` 被 `stripInline` 剥成「粗」，**不出加粗样式**）⇒ 加粗 / 斜体 / 链接 / 行内代码
 *      **不该有按钮**：放一枚就是承诺一个渲染不出来的效果。行内格式的跨端差异由 ADR-0025 ⑥-6 登记。
 *   ② **声明 ⊆ 按钮**：声明表里每个成员都得有按钮（或登记进 `DECLARED_WITHOUT_BUTTON` 并写理由）
 *      —— 否则「声明了却没入口」会静默漂过去。
 *   ③ **round-trip**：每枚按钮插入后的正文，交真执行解析器跑一遍，**必须产出该按钮承诺的块类型**
 *      —— 这是「按钮承诺必须真能渲染」的**可执行形式**（比读源码文本强一层）。
 *
 * **成对取证**（③ 判据③）：末组用注入变异的副本证明上面三条判据**有牙** —— 三种真实坏实现：
 *   ① 工具栏加一枚本档没声明的按钮（加粗）；② 少一枚按钮（分隔线）而声明还在；
 *   ③ 代码块按钮插入的语法渲染不出代码块（承诺落空）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, importedNames, readText } = require('./utsHarness');
const { forumDisplayModule, DISPLAY_UTS } = require('./forumChainHarness');

const MD_UTS = path.join(__dirname, 'markdown.uts');
const TOOLBAR_UTS = path.join(__dirname, 'markdownToolbar.uts');

/** 真源依赖注入表：成员常量与档位取值口都取自**真执行**出来的 `utils/markdown.uts` */
function markdownBindings() {
  const m = loadUts(MD_UTS, {});
  return {
    MEMBER_HEADING: m.MEMBER_HEADING,
    MEMBER_LIST: m.MEMBER_LIST,
    MEMBER_QUOTE: m.MEMBER_QUOTE,
    MEMBER_CODE: m.MEMBER_CODE,
    MEMBER_DIVIDER: m.MEMBER_DIVIDER,
    SUBSET_FORUM: m.SUBSET_FORUM,
    subsetMembers: m.subsetMembers,
  };
}

/** 每次取一个**全新模块实例** */
const toolbar = () => loadUts(TOOLBAR_UTS, markdownBindings());
/** 真执行的解析器（round-trip 的判据面） */
const parser = () => loadUts(MD_UTS, {});

/** 每枚按钮一个「一定能被该语法包住」的正文样本（round-trip 用） */
const SAMPLES = {
  heading: '标题',
  list: '项一\n项二',
  quote: '注意安全',
  code: 'const a = 1',
  divider: '正文',
};

const typesOf = (blocks) => blocks.map((b) => b.type);

// ===== ① 按钮 ⊆ 声明（本套件的核心硬约束）=====

describe('按钮集与声明表对账：工具栏不许承诺本档声明之外的语法', () => {
  it('每枚按钮的成员都在论坛档声明表里（含 member 名本身是 `MEMBER_*` 常量的对账）', () => {
    const m = loadUts(MD_UTS, {});
    const t = toolbar();
    const declared = m.SUBSET_MEMBERS_FORUM;
    for (const member of t.TOOLBAR_MEMBERS) {
      expect([member, declared.indexOf(member) >= 0]).toEqual([member, true]);
    }
  });

  it('**行内格式没有按钮**：加粗 / 斜体 / 链接 / 行内代码都不在按钮集里（本档不做行内渲染，⑥-6 登记）', () => {
    const t = toolbar();
    for (const member of ['bold', 'italic', 'link', 'inline_code', 'strikethrough', 'task_list']) {
      expect([member, t.TOOLBAR_MEMBERS.indexOf(member) >= 0]).toEqual([member, false]);
      // 「没有按钮」不是因为漏了映射：这些名字连承诺的块类型都没有
      expect([member, t.toolbarBlockType(member)]).toEqual([member, '']);
      expect([member, t.toolbarSyntax(member)]).toEqual([member, '']);
      expect([member, t.toolbarLabel(member)]).toEqual([member, '']);
    }
  });

  it('论坛档下工具栏全量出现（求交的结果就是按钮集本身）', () => {
    const m = loadUts(MD_UTS, {});
    const t = toolbar();
    expect(t.toolbarMembers(m.SUBSET_FORUM)).toEqual(t.TOOLBAR_MEMBERS);
  });
});

// ===== ② 声明 ⊆ 按钮（反向对账）=====

describe('反向对账：坛档声明的每个成员都要有按钮（或显式登记为「故意不给」）', () => {
  it('声明表 − 按钮集 = DECLARED_WITHOUT_BUTTON（今天为空 ⇒ 两个集合完全同构）', () => {
    const m = loadUts(MD_UTS, {});
    const t = toolbar();
    const noButton = m.SUBSET_MEMBERS_FORUM.filter((member) => t.TOOLBAR_MEMBERS.indexOf(member) < 0);
    expect(noButton.sort()).toEqual(t.DECLARED_WITHOUT_BUTTON.slice().sort());
    expect(t.DECLARED_WITHOUT_BUTTON).toEqual([]);
  });

  it('每枚按钮的长按中文提示非空且互不相同（文案单点，组件不许另抄一份）', () => {
    const t = toolbar();
    const labels = t.TOOLBAR_MEMBERS.map((member) => t.toolbarLabel(member));
    for (const label of labels) expect(label.length).toBeGreaterThan(0);
    expect(new Set(labels).size).toBe(labels.length);
  });

  it('长按提示的**具名映射**逐枚钉住（`###`→标题 …）：真机侧读不到 toast，判据只能落在这里', () => {
    const t = toolbar();
    const m = loadUts(MD_UTS, {});
    // 这张表就是票面 / ADR 写下的判定式；改文案必须同步改这里（真机上 adb 看不见 toast，
    // 见 docs/verification/forum/1261/README.md 未覆盖面 2 的对照实验）
    expect(t.toolbarLabel(m.MEMBER_HEADING)).toBe('标题');
    expect(t.toolbarLabel(m.MEMBER_LIST)).toBe('列表');
    expect(t.toolbarLabel(m.MEMBER_QUOTE)).toBe('引用');
    expect(t.toolbarLabel(m.MEMBER_CODE)).toBe('代码块');
    expect(t.toolbarLabel(m.MEMBER_DIVIDER)).toBe('分隔线');
    // 按钮上显示的那串记号同样逐枚钉住（长按前用户看到的就是它）
    expect(t.toolbarSyntax(m.MEMBER_HEADING)).toBe('###');
    expect(t.toolbarSyntax(m.MEMBER_LIST)).toBe('-');
    expect(t.toolbarSyntax(m.MEMBER_QUOTE)).toBe('>');
    expect(t.toolbarSyntax(m.MEMBER_CODE)).toBe('```');
    expect(t.toolbarSyntax(m.MEMBER_DIVIDER)).toBe('---');
  });
});

// ===== ③ round-trip：按钮承诺的语法必须真能渲染 =====

describe('round-trip：每枚按钮插入后的正文，交**真执行的解析器**必须产出它承诺的块类型', () => {
  it('五枚按钮逐枚对账（插入 → 按论坛档解析 → 产出该块类型）', () => {
    const t = toolbar();
    const md = parser();
    for (const member of t.TOOLBAR_MEMBERS) {
      const sample = SAMPLES[member];
      expect([member, typeof sample]).toEqual([member, 'string']);
      const edited = t.applyToolbarInsert(sample, 0, sample.length, member);
      const blocks = md.parseMarkdown(edited.text, md.SUBSET_FORUM);
      const want = t.toolbarBlockType(member);
      expect([member, want.length > 0]).toEqual([member, true]);
      // 判据是**产出物**：插入的语法真的被解析成了这一块
      expect([member, typesOf(blocks).indexOf(want) >= 0]).toEqual([member, true]);
    }
  });

  it('承诺的块类型取自解析器的同一词汇（`MarkdownBlock.type`），不是自造名', () => {
    const t = toolbar();
    const md = parser();
    const produced = new Set();
    for (const member of t.TOOLBAR_MEMBERS) {
      const sample = SAMPLES[member];
      const edited = t.applyToolbarInsert(sample, 0, sample.length, member);
      md.parseMarkdown(edited.text, md.SUBSET_FORUM).forEach((b) => produced.add(b.type));
    }
    for (const member of t.TOOLBAR_MEMBERS) {
      expect([member, produced.has(t.toolbarBlockType(member))]).toEqual([member, true]);
    }
  });
});

// ===== ④ 纯插入逻辑（行为面）=====

describe('`applyToolbarInsert` 纯插入语义', () => {
  it('行前缀类：单行选区加一次前缀，多行**逐行**加，空行跳过', () => {
    const t = toolbar();
    const h = t.applyToolbarInsert('标题', 0, 2, 'heading');
    expect(h.text).toBe('### 标题');
    const multi = t.applyToolbarInsert('项一\n项二', 0, 5, 'list');
    expect(multi.text).toBe('- 项一\n- 项二');
    // 空行不前缀：不给段落之间造一个孤立项目符号
    const gap = t.applyToolbarInsert('甲\n\n乙', 0, 4, 'quote');
    expect(gap.text).toBe('> 甲\n\n> 乙');
  });

  it('无选区：回退到**光标所在行首**加前缀，光标落在前缀之后', () => {
    const t = toolbar();
    const r = t.applyToolbarInsert('第一行\n第二行', 6, 6, 'quote');
    expect(r.text).toBe('第一行\n> 第二行');
    // 判据不写死下标算术：光标之前的文本必须以该前缀收尾，且正好是插入后的位置
    expect(r.text.substring(0, r.start).endsWith('> ')).toBe(true);
    expect([r.start, r.end]).toEqual([r.start, r.start]);
  });

  it('代码块：选区非空则整块包裹；无选区则插入空围栏并把落位放进块内', () => {
    const t = toolbar();
    const wrapped = t.applyToolbarInsert('const a = 1', 0, 11, 'code');
    expect(wrapped.text).toBe('```\nconst a = 1\n```');
    expect(wrapped.text.substring(wrapped.start, wrapped.end)).toBe('const a = 1');
    const empty = t.applyToolbarInsert('', 0, 0, 'code');
    expect(empty.text).toBe('```\n\n```');
    expect([empty.start, empty.end]).toEqual([4, 4]);
  });

  it('分隔线：独占一行且**不丢选区内容**（这一支最容易犯的错），前后不重复补换行', () => {
    const t = toolbar();
    // 选区非空：分隔线断行在前，选区内容原样保留在其后
    const withSel = t.applyToolbarInsert('正文', 0, 2, 'divider');
    expect(withSel.text).toBe('---\n正文');
    expect(withSel.text).toContain('正文');
    // 光标在行尾：前补一个换行、行尾不再补
    const atEnd = t.applyToolbarInsert('正文', 2, 2, 'divider');
    expect(atEnd.text).toBe('正文\n---');
    // 行首（前一字符已是换行）不重复补
    const atLineStart = t.applyToolbarInsert('上一段\n', 4, 4, 'divider');
    expect(atLineStart.text).toBe('上一段\n---');
    // 后面还有内容时补尾换行
    const mid = t.applyToolbarInsert('甲\n乙', 2, 2, 'divider');
    expect(mid.text).toBe('甲\n---\n乙');
  });

  it('越界选区被夹住（不产生 `substring` 的静默截断）；起点大于终点时自动交换', () => {
    const t = toolbar();
    const over = t.applyToolbarInsert('短', 0, 999, 'list');
    expect(over.text).toBe('- 短');
    const under = t.applyToolbarInsert('短', -5, -1, 'list');
    expect(under.text).toBe('- 短');
    const swapped = t.applyToolbarInsert('甲乙', 2, 0, 'quote');
    expect(swapped.text).toBe('> 甲乙');
  });

  it('未知成员：正文原样返回（点了没反应），**不**静默插半截语法', () => {
    const t = toolbar();
    const r = t.applyToolbarInsert('正文', 1, 1, 'bold');
    expect(r.text).toBe('正文');
    expect([r.start, r.end]).toEqual([1, 1]);
  });
});

// ===== ⑤ 成对取证：注入变异后，上面的判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function loadMutated(replacements) {
  let src = readText(TOOLBAR_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-toolbar-'));
  const file = path.join(dir, 'markdownToolbar.uts');
  fs.writeFileSync(file, src);
  return loadUts(file, markdownBindings());
}

describe('成对取证（必红）：本套件的三条判据在坏实现上确实会红', () => {
  it('必不红（对照）：真源上按钮集与声明表同构、五枚按钮全部 round-trip 成立', () => {
    const t = toolbar();
    const m = loadUts(MD_UTS, {});
    expect(t.toolbarMembers(m.SUBSET_FORUM)).toEqual(t.TOOLBAR_MEMBERS);
    const md = parser();
    for (const member of t.TOOLBAR_MEMBERS) {
      const sample = SAMPLES[member];
      const edited = t.applyToolbarInsert(sample, 0, sample.length, member);
      expect(typesOf(md.parseMarkdown(edited.text, md.SUBSET_FORUM))).toContain(t.toolbarBlockType(member));
    }
  });

  it('必红 · 加一枚**本档未声明**的按钮（加粗）⇒ ①「按钮 ⊆ 声明」判据必然判红', () => {
    const broken = loadMutated([
      ["    MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER,\n]", "    MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER, 'bold',\n]"],
    ]);
    const m = loadUts(MD_UTS, {});
    // 坏实现真的多出一枚按钮，而它在声明表里没有
    expect(broken.TOOLBAR_MEMBERS).toContain('bold');
    expect(m.SUBSET_MEMBERS_FORUM.indexOf('bold')).toBe(-1);
    // ⇒ 第 ① 组的逐枚断言判红；且求交结果不再等于按钮集 ⇒ 第 ① 组第三条也判红
    expect(broken.toolbarMembers(m.SUBSET_FORUM)).not.toEqual(broken.TOOLBAR_MEMBERS);
  });

  it('必红 · 少一枚按钮（分隔线）而声明还在 ⇒ ②「声明 ⊆ 按钮」判据必然判红', () => {
    const broken = loadMutated([
      ['MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE, MEMBER_DIVIDER,', 'MEMBER_HEADING, MEMBER_LIST, MEMBER_QUOTE, MEMBER_CODE,'],
    ]);
    const m = loadUts(MD_UTS, {});
    const noButton = m.SUBSET_MEMBERS_FORUM.filter((member) => broken.TOOLBAR_MEMBERS.indexOf(member) < 0);
    expect(noButton).toEqual(['divider']);           // 坏实现真的让一个声明成员没了入口
    expect(noButton.sort()).not.toEqual([]);         // ⇒ 第 ② 组的「等于 DECLARED_WITHOUT_BUTTON」判据判红
  });

  it('必红 · 换掉一枚长按文案（标题 → Heading）⇒ 具名映射判据必然判红', () => {
    const broken = loadMutated([
      ["if (member == MEMBER_HEADING) return '标题'", "if (member == MEMBER_HEADING) return 'Heading'"],
    ]);
    const m = loadUts(MD_UTS, {});
    expect(broken.toolbarLabel(m.MEMBER_HEADING)).toBe('Heading'); // 坏实现真的换了文案
    // ⇒ 「=== '标题'」的具名映射断言判红
  });

  it('必红 · 代码块按钮插入的语法渲染不出代码块（承诺落空）⇒ ③ round-trip 判据必然判红', () => {
    const broken = loadMutated([["    if (member == MEMBER_CODE) {", "    if (member == '__never__') {"]]);
    const md = parser();
    const edited = broken.applyToolbarInsert(SAMPLES.code, 0, SAMPLES.code.length, 'code');
    const produced = typesOf(md.parseMarkdown(edited.text, md.SUBSET_FORUM));
    // 坏实现插进去的语法渲染不出代码块（承诺落空），而 round-trip 判据要的正是 code
    expect(produced).not.toContain('code');
    expect(broken.toolbarBlockType('code')).toBe('code'); // 承诺还在 ⇒ 判据必然判红
  });
});

// ===== ⑥ 模块契约：不持第二份声明 =====

describe('模块契约：工具栏只桥到 `utils/markdown.uts`，不自持子集声明', () => {
  it('运行期 import 恰为五个成员常量 + 论坛档常量 + 声明表取值口（没有第二份子集声明 / 第二份解析器）', () => {
    expect(importedNames(readText(TOOLBAR_UTS)).sort()).toEqual(
      ['MEMBER_CODE', 'MEMBER_DIVIDER', 'MEMBER_HEADING', 'MEMBER_LIST', 'MEMBER_QUOTE', 'SUBSET_FORUM', 'subsetMembers'].sort()
    );
  });

  it('界面层的唯一入口 `forumToolbarMembers()` 就是「本档声明 ∩ 按钮模板」——组件因此不必直引档位常量', () => {
    const t = toolbar();
    const m = loadUts(MD_UTS, {});
    expect(t.forumToolbarMembers()).toEqual(t.toolbarMembers(m.SUBSET_FORUM));
    expect(t.forumToolbarMembers()).toEqual(t.TOOLBAR_MEMBERS);
  });

  it('按钮成员表里没有裸字面量（成员名一律经 `MEMBER_*` 常量，与声明表同一词汇）', () => {
    const t = toolbar();
    const md = loadUts(MD_UTS, {});
    const known = [md.MEMBER_HEADING, md.MEMBER_LIST, md.MEMBER_QUOTE, md.MEMBER_CODE, md.MEMBER_DIVIDER, md.MEMBER_IMAGE, md.MEMBER_TABLE];
    for (const member of t.TOOLBAR_MEMBERS) {
      expect([member, known.indexOf(member) >= 0]).toEqual([member, true]);
    }
  });
});

// ===== ⑦ 能力提示行「只讲边界」不许变成假话（⑥-5/⑥-6）=====

/** 真执行的提示行文案（`utils/forumDisplay.uts` 自 #1273 起 import 格式轴 ⇒ 经链夹具取上游） */
const boundaryNotice = () => forumDisplayModule().FORUM_MARKDOWN_BOUNDARY_NOTICE;

describe('边界提示行：它claim的每条边界都必须**与成员声明表一致**（否则那行就是假话）', () => {
  it('三条边界都在文案里点名（表格 / 图表 / 行内格式 / 图片入口）', () => {
    const notice = boundaryNotice();
    expect(notice).toContain('表格');
    expect(notice).toContain('图表');
    expect(notice).toContain('粗体');
    expect(notice).toContain('链接');
    expect(notice).toContain('图片');
    // 「只讲边界、不讲能力清单」（⑥-5）：它不许出现「支持 …」这种能力枚举
    expect(notice).not.toContain('支持');
  });

  it('文案说的边界在声明表里是**真的**：表格与正文内嵌图片都没被论坛档声明', () => {
    const md = loadUts(MD_UTS, {});
    expect(md.SUBSET_MEMBERS_FORUM).not.toContain(md.MEMBER_TABLE);
    expect(md.SUBSET_MEMBERS_FORUM).not.toContain(md.MEMBER_IMAGE);
    // 反过来也成立：声明了的成员都有按钮（第 ② 组已判，这里只声明两表的同向关系）
    const t = toolbar();
    for (const member of md.SUBSET_MEMBERS_FORUM) {
      expect([member, t.TOOLBAR_MEMBERS.indexOf(member) >= 0]).toEqual([member, true]);
    }
  });

  it('成对取证（必红 · 提示行半）：文案删掉「表格」⇒ 关键词判据必然判红', () => {
    let src = readText(DISPLAY_UTS);
    const anchor = '表格与图表显示原文';
    expect(src.includes(anchor)).toBe(true); // 锚点失效即红：变异必须真的进去了
    src = src.split(anchor).join('图表显示原文');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-notice-'));
    const file = path.join(dir, 'forumDisplay.uts');
    fs.writeFileSync(file, src);
    // 上游三层取真源、只有被改坏的这一层用变异副本（成对取证要变的是**这一处**，不是整条链）
    const broken = forumDisplayModule(undefined, file).FORUM_MARKDOWN_BOUNDARY_NOTICE;
    expect(broken).not.toContain('表格'); // ⇒ 关键词判据判红
  });

  it('成对取证（必红 · 声明表半）：给论坛档声明 table ⇒ 「边界是真的」判据必然判红', () => {
    let src = readText(MD_UTS);
    const anchor = 'MEMBER_CODE, MEMBER_DIVIDER,\n]';
    expect(src.includes(anchor)).toBe(true); // 锚点失效即红：变异必须真的进去了
    src = src.split(anchor).join('MEMBER_CODE, MEMBER_DIVIDER, MEMBER_TABLE,\n]');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-subset-'));
    const file = path.join(dir, 'markdown.uts');
    fs.writeFileSync(file, src);
    const brokenMd = loadUts(file, {});
    expect(brokenMd.SUBSET_MEMBERS_FORUM).toContain('table'); // 坏实现真的把表格收进了本档
    // ⇒ 「本档不声明 table（提示行那句话才是真的）」的断言判红
  });
});
