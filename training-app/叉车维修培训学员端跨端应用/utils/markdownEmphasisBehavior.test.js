/**
 * 段落「弱强调」判据（#2399）—— **行为腿**
 *
 * 判据落在跑起来的产出物上（`loadUts` 真执行 `utils/markdownEmphasis.uts`）：给定一行段落文本，
 * 它该不该被加粗 + 拉开段前距。断的是判定结果，不是源码字面量 —— 接线守护不构成 ③ 门证据
 * （`docs/agents/guards.md`），所以判据必须是被真执行出来的。
 *
 * 三条判据同时满足才算（票面 AC ②）：行首序号 + 整行不含句末标点 + 不超过一行（约 20 字）。
 * 长度按**纯文本投影**计（解答面保留 `**` 等标记字符），边界见票面 AC ④。
 *
 * 成对取证（③ 门判据③）：末组用**注入变异的副本**证明判据有牙 —— 去掉句末标点闸 / 把 20 字
 * 上限放大，两种坏实现都必须让上面的用例翻红；每条变异带 `includes` 锚点，锚点失效即红。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const EMPH_UTS = path.join(__dirname, 'markdownEmphasis.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');

const emph = () => loadUts(EMPH_UTS, {});
/** 真源解析器：真链路用例的入参就是它的产出（不是测试自造的另一种块形状） */
const md = () => loadUts(MD_UTS, {});

/** 短判据：一行文本 → 是否弱强调 */
const isEmph = (text) => emph().isEmphasisParagraph(text);

describe('该强调：行首序号 + 短行 + 无句末标点', () => {
  it('中文序号一–十，顿号后接定位短语', () => {
    expect(isEmph('一、故障码快速定位指引')).toBe(true);
    expect(isEmph('二、常见故障码含义对照')).toBe(true);
    expect(isEmph('十、两位数序号边界')).toBe(true);
  });

  it('阿拉伯序号：顿号，或点 + 空白', () => {
    expect(isEmph('1、检查电池电压')).toBe(true);
    // 谓词层面吃「1. 」；但真实链路上它先被有序列表分支吃掉、成为 list 块（票面 Q1）——见下条真链路用例
    expect(isEmph('1. 检查电池电压')).toBe(true);
    expect(isEmph('12. 检查液压油位')).toBe(true);
  });

  it('正好 20 字仍算一行（不差一字的边界）', () => {
    const exact = '四、制动管路进气的三种典型表现与判断方法';
    expect(exact.length).toBe(20);
    expect(isEmph(exact)).toBe(true);
  });
});

describe('不该强调：宁可漏不可滥', () => {
  it('带句末标点的是句子，不是小标题', () => {
    expect(isEmph('一、这是完整的一句话，说明电压要测。')).toBe(false);
    expect(isEmph('五、液压系统压力不足！')).toBe(false);
    expect(isEmph('六、这个问题到底出在哪里？')).toBe(false);
  });

  it('超一行（21 字起）当正文段落', () => {
    const long = '七、' + '长'.repeat(19);
    expect(long.length).toBe(21);
    expect(isEmph(long)).toBe(false);
  });

  it('「十一、」不是中文序号（一–十之外不命中）', () => {
    expect(isEmph('十一、附录：工具清单')).toBe(false);
    expect(isEmph('二十、术语表')).toBe(false);
  });

  it('序号不在行首 / 没有序号 / 只有序号没内容 都不强调', () => {
    expect(isEmph('那么一、故障码快速定位指引')).toBe(false);
    expect(isEmph('故障码快速定位指引')).toBe(false);
    expect(isEmph('一、')).toBe(false);
    expect(isEmph('')).toBe(false);
    expect(isEmph('   ')).toBe(false);
  });

  it('小数不当序号：1.5mm 间隙 不误伤', () => {
    expect(isEmph('1.5mm 间隙要留够')).toBe(false);
    expect(isEmph('2.0 版本的接口已废弃')).toBe(false);
  });
});

describe('长度按纯文本投影计（解答面保留标记字符）', () => {
  it('源码 22 字、投影 18 字 ⇒ 仍强调（标记字符不算显示宽度）', () => {
    const label = '八、两位数序号边界的核对要求示例说明';
    const raw = '**' + label + '**';
    expect(label.length).toBe(18);
    expect(raw.length).toBe(22);
    expect(isEmph(raw)).toBe(true);
  });

  it('行内标记 / 链接只计入看得见的字', () => {
    expect(isEmph('一、**故障码**快速定位')).toBe(true);
    expect(isEmph('[一、故障码快速定位指引](https://example.com/a)')).toBe(true);
    expect(isEmph('`一、故障码快速定位指引`')).toBe(true);
  });
});

describe('真链路：解析器产出的段落文本直接喂进来', () => {
  it('解答面：一、二、不升标题但吃强调；「1. 」是有序列表、不进段落判据', () => {
    const blocks = md().parseMarkdown('一、故障码快速定位指引\n\n二、常见故障码含义对照\n\n1. 检查电池电压',
      'ai-answer');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'paragraph', 'list']);
    expect(blocks.map((b) => isEmph(b.text))).toEqual([true, true, false]);
  });

  it('章节面：普通正文段落一律不强调（阅读页不回归，票面 AC ⑤）', () => {
    const blocks = md().parseMarkdown('先看总览。\n\n故障码快速定位指引\n\n一、故障码快速定位指引');
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'paragraph', 'paragraph']);
    expect(blocks.map((b) => isEmph(b.text))).toEqual([false, false, true]);
  });
});

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  function mutatedCopy(replacements) {
    let src = readText(EMPH_UTS);
    for (const [from, to] of replacements) {
      expect([from, src.includes(from)]).toEqual([from, true]);
      src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'md-emphasis-'));
    const file = path.join(dir, path.basename(EMPH_UTS));
    fs.writeFileSync(file, src);
    return file;
  }
  const loadMutated = (replacements) => loadUts(mutatedCopy(replacements), {});
  const mutatedIsEmph = (replacements, text) => loadMutated(replacements).isEmphasisParagraph(text);

  it('必不红 · 原样副本与真源同结果 —— 红不是临时目录带来的', () => {
    expect(mutatedIsEmph([], '一、故障码快速定位指引')).toBe(true);
    expect(mutatedIsEmph([], '一、带句号。')).toBe(false);
  });

  it('必红 · 去掉句末标点闸 ⇒「带句号不强调」判据必然判红', () => {
    const broken = [
      ['if (plain.indexOf(SENTENCE_MARKS[i]) >= 0) return false',
        'if (plain.indexOf(SENTENCE_MARKS[i]) >= 0) return true'],
    ];
    expect(mutatedIsEmph(broken, '一、这是完整的一句话。')).toBe(true); // 真实现是 false
  });

  it('必红 · 把 20 字上限放大 ⇒「超一行不强调」判据必然判红', () => {
    const broken = [['const MAX_EMPHASIS_CHARS : number = 20', 'const MAX_EMPHASIS_CHARS : number = 200']];
    expect(mutatedIsEmph(broken, '七、' + '长'.repeat(40))).toBe(true); // 真实现是 false
  });
});
