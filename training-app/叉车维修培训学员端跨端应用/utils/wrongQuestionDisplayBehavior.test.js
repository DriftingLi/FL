/**
 * 错题本展示纯函数 · **行为级**测试（#1948）
 *
 * 钉住两件事，都不是源码文本测试能证明的：
 *   ① 「易错项」无数据时返回**空串**，页面据此整条不渲染。后端语义是「判错样本 < 5 /
 *      简答题 ⇒ **缺省该键**」（不是空串、不是 0），装载层把缺省读成空串之后，
 *      「缺省即不展示」这条只剩在纯函数这层还能机检——写成「易错项 」这种空标签
 *      在源码文本里一模一样，只有跑出来才分得清。
 *   ② parseAnswerKeys 从 wrong-question-card.uvue 挪进 utils（卡片恰在 600 行预算上限），
 *      搬动不得改变答案串解析行为：多选分隔符兼容、判断题 true/false 原样成一条、
 *      无分隔符时逐字符拆。这条是**搬动本身的回归锁**。
 *
 * 缝：utils/utsHarness.js 的 loadUts。先例 jobCredentialDisplayBehavior.test.js。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'wrongQuestionDisplay.uts');

const display = () => loadUts(DISPLAY_UTS, {});

describe('wrongOptionText：错题本「易错项」一行（#1948）', () => {
  const { wrongOptionText } = display();

  it('数据源缺省（装载成空串）⇒ 空串，页面整条不渲染', () => {
    expect(wrongOptionText('')).toBe('');
  });

  it('单选有值 ⇒ 易错项 + 选项', () => {
    expect(wrongOptionText('B')).toBe('易错项 B');
  });

  it('多选是**答案串组合**，原样展示、不拆成单字母（后端口径：值形如 "A,C"）', () => {
    expect(wrongOptionText('A,C')).toBe('易错项 A,C');
  });

  it('判断题的 true/false 也是一个取值，照实展示', () => {
    expect(wrongOptionText('false')).toBe('易错项 false');
  });
});

describe('parseAnswerKeys：答案串 → key 数组（#1948 从卡片挪入 utils，行为必须不变）', () => {
  const { parseAnswerKeys } = display();

  it('空串 ⇒ 空数组', () => {
    expect(parseAnswerKeys('')).toEqual([]);
  });

  it('半角逗号分隔', () => {
    expect(parseAnswerKeys('A,C')).toEqual(['A', 'C']);
  });

  it('全角逗号分隔', () => {
    expect(parseAnswerKeys('A，C')).toEqual(['A', 'C']);
  });

  it('分号与竖线分隔', () => {
    expect(parseAnswerKeys('A;B')).toEqual(['A', 'B']);
    expect(parseAnswerKeys('A|B')).toEqual(['A', 'B']);
  });

  it('分隔串里的空白项被丢掉', () => {
    expect(parseAnswerKeys('A, ,C')).toEqual(['A', 'C']);
  });

  it('判断题 true/false 整体成一条，不被逐字符拆', () => {
    expect(parseAnswerKeys('true')).toEqual(['true']);
    expect(parseAnswerKeys('false')).toEqual(['false']);
  });

  it('无分隔符的多选题干答案逐字符拆', () => {
    expect(parseAnswerKeys('AC')).toEqual(['A', 'C']);
  });
});

describe('既有纯函数不因本次搬动而失焦（getTypeName / isMultiChoice / answerFoldLabel）', () => {
  const { getTypeName, isMultiChoice, answerFoldLabel } = display();

  it('题型 code → 中文名', () => {
    expect(getTypeName('single_choice')).toBe('单选题');
    expect(getTypeName('multi_choice')).toBe('多选题');
  });

  it('多选判定与解析器共用同一套 code 口径', () => {
    expect(isMultiChoice('multi_choice')).toBe(true);
    expect(isMultiChoice('single_choice')).toBe(false);
  });

  it('折叠态默认给引导语', () => {
    expect(answerFoldLabel(false)).toBe('查看答案与解析');
  });
});
