/**
 * 行内分词器（`utils/markdownInline.uts`）· **行为级**测试（ADR-0025 ⑩-2/⑩-5/⑩-8，票 #1472）
 *
 * 为什么必须真执行而不是读源码文本：#1472 的原始故障是「读者看见 `**故障码**`」——
 * 那是**产出物**层面的病，断言 `.uvue` 里出现过某个类名抓不到它（`docs/agents/guards.md`：
 * 接线守护不构成 ③ 门证据）。本套件跑 `splitInlineRuns` 的**返回结构**与
 * `inlineRunsPlainText` 的**返回字符串**。缝 = `utils/utsHarness.js` 的 `loadUts`
 * （先例 `markdownSubsetBehavior` / `forumBodyBehavior`；本模块零运行期 import，空绑定即可）。
 *
 * ## 四组判据，各自对应票面一条 AC
 * - ① **记号消化**（⑥-1）：可见文字里不剩**成对**记号 —— 且**逐 run 判**，不拼成整段判
 *   （原型实测：跨 run 拼接会把代码内容里的 `**` 与远处一个裸 `**` 连成一次误报）；
 * - ② **嵌套继承**（⑩-2，形状决定）：粗体里的链接/代码必须是**独立 run** 且带 `bold`；
 * - ③ **正文一字不丢**（flanking + 公式保护）：裸记号字符与词内下划线**保留原样**，
 *   公式跨度不被斜体规则撕碎；
 * - ④ **协议闸**（⑩-8）：只有 http(s) 成为可点 run。
 *
 * **成对取证**（③ 门判据③）：末组用**注入变异的副本**证明上面的判据**有牙** ——
 * 三种真实的坏实现各自必须判红，另配一条**必不红**对照组（否则「红」可能只是夹具坏了）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const INLINE_UTS = path.join(__dirname, 'markdownInline.uts');
const mod = () => loadUts(INLINE_UTS, {});

/** 成对记号（裸记号字符**不算** —— 那是作者的正文，⑥-1 只要求「成对记号不显示」） */
const PAIRED = /\*\*[^*]+\*\*|~~[^~]+~~|`[^`]+`|\[[^\]]*\]\([^)]*\)/;

/** 把 runs 摊成 [{text, flags}] 便于断言；`code` run 的内容不参与「记号泄漏」判定 */
const leaves = (runs) => runs.filter((r) => !r.code).map((r) => r.text);
const plain = (src) => {
  const m = mod();
  return m.inlineRunsPlainText(m.splitInlineRuns(src));
};
const runsOf = (src) => mod().splitInlineRuns(src);
const find = (runs, needle) => runs.filter((r) => r.text === needle)[0];

// ===== ① 记号消化（⑥-1）=====

describe('① 成对记号不出现在可见文字里（逐 run 判，不拼整段）', () => {
  const CASES = [
    ['**粗** 体', '粗 体'],
    ['见 [维修手册](https://e.com/m) 第 3 节', '见 维修手册 第 3 节'],
    ['参数 `torque` 见铭牌', '参数 torque 见铭牌'],
    ['~~已作废~~ 条款', '已作废 条款'],
    ['__斜__ 与 *斜*", ', '斜 与 斜", '],
    ['看图 ![故障铭牌](https://e.com/1.png) 结束', '看图 故障铭牌 结束'],
  ];
  for (const [src, want] of CASES) {
    it(`${JSON.stringify(src)} ⇒ ${JSON.stringify(want)}`, () => {
      expect(plain(src)).toBe(want);
      const bad = leaves(runsOf(src)).filter((t) => PAIRED.test(t));
      expect(bad).toEqual([]);
    });
  }

  it('空串与无记号文本 ⇒ 各恰一条 run，不产空 run', () => {
    expect(runsOf('')).toEqual([]);
    const one = runsOf('纯文字');
    expect(one.length).toBe(1);
    expect(one[0].text).toBe('纯文字');
  });
});

// ===== ② 嵌套继承（⑩-2 的形状决定：扁平 runs + 样式位）=====

describe('② 粗体里的链接/代码成为独立 run 并继承外层样式（单趟匹配必漏的那格）', () => {
  const runs = runsOf('**粗 [链接](https://e.com/a) 和 `码`** 与 *斜 ~~删~~*');

  it('链接 run 存在、`link` 有值、且 `bold` 为真（不是把源串留在粗体里）', () => {
    const link = runs.filter((r) => r.link === 'https://e.com/a')[0];
    expect(link).toBeTruthy();
    expect(link.text).toBe('链接');
    expect(link.bold).toBe(true);
  });

  it('代码 run 在粗体内、内容原样、继承 bold', () => {
    const code = runs.filter((r) => r.code)[0];
    expect(code.text).toBe('码');
    expect(code.bold).toBe(true);
  });

  it('斜体里套删除线 ⇒ 同一条 run 两个样式位（样式正交，所以不需要树）', () => {
    const both = runs.filter((r) => r.text === '删')[0];
    expect([both.italic, both.strike]).toEqual([true, true]);
  });

  it('整个嵌套串的任何可见文字里都不剩成对记号（⑥-1 与 ⑩-2 的合取）', () => {
    expect(leaves(runs).filter((t) => PAIRED.test(t))).toEqual([]);
  });
});

// ===== ③ 正文一字不丢（flanking + 公式）=====

describe('③ 裸记号与词内下划线是正文，不是记号（现状正则会把它们当记号吃掉字符）', () => {
  it('乘号与不成对的星号 ⇒ 原样保留', () => {
    const src = '2 * 3 = 6，* 是乘号；a ** b';
    expect(plain(src)).toBe(src);
  });

  it('词内下划线（变量名）⇒ 不被拦腰咬断', () => {
    const src = '变量 a_b_c 与 snake_case_name 要留着';
    expect(plain(src)).toBe(src);
    expect(runsOf(src).filter((r) => r.italic).length).toBe(0);
  });

  it('公式跨度整体保留，下标不被斜体规则撕碎（与 markdown.uts 同一条降级）', () => {
    const src = '价格 $a_1 + b_2$ 与 **粗** 在旁';
    expect(plain(src)).toBe('价格 $a_1 + b_2$ 与 粗 在旁');
  });

  it('行内代码内容不被二次剥（根 ADR-0044 拒绝正则剥离的理由就在这格）', () => {
    const runs = runsOf('变量 `a**b` 与 反引号外的 ** 号');
    const code = runs.filter((r) => r.code)[0];
    expect(code.text).toBe('a**b');
  });
});

// ===== ④ 协议闸（⑩-8）=====

describe('④ 只有 http(s) 可点（白名单，不是拉黑三个坏协议）', () => {
  it('好协议成 link run', () => {
    const r = find(runsOf('[手册](https://gccsmile.com/m)'), '手册');
    expect(r.link).toBe('https://gccsmile.com/m');
    const h = find(runsOf('[a](http://x.com)'), 'a');
    expect(h.link).toBe('http://x.com');
  });

  it('坏协议与相对路径 ⇒ 文字仍在、但**不可点**', () => {
    for (const url of ['javascript:alert(1)', 'data:text/html,hi', 'file:///etc/passwd',
      'mailto:a@b.com', '//evil.com/x', '/relative/path']) {
      const runs = runsOf(`点 [字](${url}) 这里`);
      const r = find(runs, '字');
      expect([url, r]).toBeTruthy();
      expect([url, r.link]).toEqual([url, '']);
    }
  });

  it('URL 含一层括号不留残括号（现状 `[^)]*` 会漏一个 `)`）', () => {
    const runs = runsOf('[维基](https://zh.wikipedia.org/wiki/x_(y))');
    expect(find(runs, '维基').link).toBe('https://zh.wikipedia.org/wiki/x_(y)');
    expect(plain('[维基](https://zh.wikipedia.org/wiki/x_(y))')).toBe('维基');
    // 坏协议里的括号也不能留下游离 `)`
    expect(plain('[a](javascript:alert(1))')).toBe('a');
  });

  it('链接文本为空 ⇒ 退化成显示 URL 本身（不产空 run）', () => {
    expect(plain('[](https://e.com/a)')).toBe('https://e.com/a');
  });
});

// ===== ④b 域名投影（⑩-8：确认弹窗喂「目标域名」，不喂整条 URL）=====

describe('⑩-8 的域名投影：只取主机名，切不出就退回原串', () => {
  const domain = (u) => mod().inlineLinkDomain(u);
  it('常见形态各取其主机名（去 scheme / 截 authority / 去 userinfo / 去端口）', () => {
    expect(domain('https://gccsmile.com/forum/123?x=1')).toBe('gccsmile.com');
    expect(domain('http://sub.example.cn:8080/a')).toBe('sub.example.cn');
    expect(domain('https://user:pw@example.org/p#frag')).toBe('example.org');
    expect(domain('https://example.org')).toBe('example.org');
  });
  it('切不出主机名 ⇒ 退回整条 URL（宁可多显示，不可谎报成别的域名）', () => {
    expect(domain('https:///nopath')).toBe('https:///nopath');
  });
  it('与协议闸同源：可点 run 的 link 必过闸，域名是其显示投影', () => {
    const m = mod();
    const linkRun = m.splitInlineRuns('[x](https://a.com/y)').filter((r) => r.link.length > 0)[0];
    expect(m.inlineLinkSafe(linkRun.link)).toBe(true);
    expect(m.inlineLinkDomain(linkRun.link)).toBe('a.com');
  });
});

// ===== ⑤ 成对取证：判据在坏实现上必须红 =====

/** 读真源 → 注入变异 → 落临时目录 → 真执行（不改工作树，不进仓） */
function loadMutated(replacements) {
  let src = readText(INLINE_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);  // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'md-inline-'));
  const file = path.join(dir, 'markdownInline.uts');
  fs.writeFileSync(file, src);
  return loadUts(file, {});
}

describe('成对取证（③ 门判据③）：本套件的判据有牙', () => {
  it('必不红：真源上嵌套那格产出独立链接 run（对照组 —— 变异判红之前，先证明判据本身没坏）', () => {
    const m = mod();
    const runs = m.splitInlineRuns('**粗 [链接](https://e.com/a)**');
    expect(runs.filter((r) => r.link === 'https://e.com/a').length).toBe(1);
  });

  it('必红 · 拆掉递归（内层不再解析）⇒ 链接源串留在粗体里，⑥-1 与 ② 组同时判红', () => {
    const broken = loadMutated([
      ['    const innerRuns = splitInner(inner, mathSpansOf(inner), bold, italic, strike, link)',
        '    const innerRuns = inner.length > 0 ? [{ text: inner, bold: bold, italic: italic, strike: strike, code: false, link: link } as MarkdownInlineRun] : []'],
    ]);
    const runs = broken.splitInlineRuns('**粗 [链接](https://e.com/a)**');
    // 内层没被解析 ⇒ 记号还在文字里（这正是单趟扁平匹配的病）
    expect(runs.map((r) => r.text).join('').includes('[链接](https://e.com/a)')).toBe(true);
  });

  it('必红 · 协议闸从白名单退化成「只拉黑 javascript」⇒ mailto 也变可点', () => {
    const broken = loadMutated([
      ["  return u.indexOf('http://') == 0 || u.indexOf('https://') == 0",
        "  return u.indexOf('javascript:') != 0"],
    ]);
    expect(broken.inlineLinkSafe('https://e.com/a')).toBe(true);
    expect(broken.inlineLinkSafe('mailto:a@b.com')).toBe(true);   // 坏实现：本该不可点
  });

  it('必红 · 去掉词内下划线那道 flanking ⇒ `a_b_c` 被咬成斜体并丢字符', () => {
    const broken = loadMutated([
      ['    if (it2 != null && !isWordChar(prevChar) && !isWordChar(nextChar)) {',
        '    if (it2 != null) {'],
    ]);
    const got = broken.inlineRunsPlainText(broken.splitInlineRuns('变量 a_b_c 结束'));
    expect(got).not.toBe('变量 a_b_c 结束');   // 真源上这条是相等的（见 ③ 组）
  });
});

// ===== ⑥ 与格式轴的接线：投影与渲染共用同一次分词（⑩-5）=====

describe('模块契约：投影取自同一次分词的叶子，不留第二条剥记号的路', () => {
  it('本模块零运行期依赖（空绑定即可载入）', () => {
    expect(() => loadUts(INLINE_UTS, {})).not.toThrow();
  });

  it('`forumBody` 里**没有**第二处剥记号逻辑（正则或 stripInline 直引）', () => {
    const body = readText(path.join(__dirname, 'forumBody.uts'));
    expect(body).not.toMatch(/replace\(/);
    expect(body).not.toContain('stripInline');
  });
});
