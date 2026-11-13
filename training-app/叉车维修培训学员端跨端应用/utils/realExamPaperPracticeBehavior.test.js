/**
 * #2463 按卷练习承载的判定守护（spec #2459 决策 4；票 #2463 AC②③④）
 *
 * 被测对象是 `utils/realExamPaperPractice.uts` —— paper 承载里**只有契约的三件事**：
 * 进度键 `paper:<卷 id>`、answers_state 的值形态（写对象才读得回对错）、续练游标怎么取舍；
 * 外加失败文案不许露出 transport 的 `404:` 状态前缀。
 *
 * 用 `loadUts` 跑**真源码**（与 utils/realExamPapersDisplayBehavior.test.js 同形）：
 * 这些判定错了，现象是「退出重进回到第一题」「作答丢了」「页面白屏」—— 都不是模板能兜住的。
 * 页面接线另由 utils/practiceContract.test.js 守。
 *
 * 最后一组是变异取证：把游标优先级改坏后断言读数翻面，证明「已存进度优先」不是恒真。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const uts = require('./utsHarness');

const PAPER_UTS = path.join(__dirname, 'realExamPaperPractice.uts');

/** 每次取全新模块实例（模块级状态不跨用例串） */
const paper = () => uts.loadUts(PAPER_UTS, {});

/** 变异：把真源码按 pairs 替换后写到临时目录，返回副本路径（先例 realExamPapersDisplayBehavior） */
function mutatePaper(pairs) {
  let src = uts.readText(PAPER_UTS);
  for (const pair of pairs) {
    expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
    src = src.split(pair[0]).join(pair[1]);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'real-exam-paper-'));
  const file = path.join(dir, 'realExamPaperPractice.uts');
  fs.writeFileSync(file, src);
  return file;
}

describe('paperProgressMode：进度键是 paper:<卷 id>（AC②）', () => {
  it('前缀 + 卷 id，不夹空格', () => {
    const m = paper();
    expect(m.PAPER_PRACTICE_MODE_PREFIX).toBe('paper:');
    expect(m.paperProgressMode(12)).toBe('paper:12');
    expect(m.paperProgressMode(0)).toBe('paper:0');
  });
});

describe('paperAnswerStateValue：写进 answers_state 的值（AC②③）', () => {
  it('作答、对错、正确答案三条都落进对象（与 Web 同型）', () => {
    const v = paper().paperAnswerStateValue({
      question_id: 7, user_answer: 'A,C', is_correct: false, correct_answer: 'A,B', submitted: true,
    });
    expect(v).toEqual({ user_answer: 'A,C', is_correct: false, correct_answer: 'A,B' });
  });
});

describe('parsePaperAnswerStates：续练读回作答（AC③）', () => {
  it('对象形态：连对错与正确答案一起还原，只认键为正整数的项', () => {
    const states = paper().parsePaperAnswerStates({
      '7': { user_answer: 'A,C', is_correct: true, correct_answer: 'A,C' },
      '9': { user_answer: 'B', is_correct: false, correct_answer: 'D' },
      'x': { user_answer: 'A', is_correct: true, correct_answer: 'A' },
      '0': { user_answer: 'A', is_correct: true, correct_answer: 'A' },
    });
    expect(states).toEqual([
      { question_id: 7, user_answer: 'A,C', is_correct: true, correct_answer: 'A,C', submitted: true },
      { question_id: 9, user_answer: 'B', is_correct: false, correct_answer: 'D', submitted: true },
    ]);
  });

  it('裸字符串形态：只还原作答，对错未知（correct_answer 留空 ⇒ 页面不显示判题）', () => {
    const states = paper().parsePaperAnswerStates({ '5': 'B' });
    expect(states).toEqual([
      { question_id: 5, user_answer: 'B', is_correct: false, correct_answer: '', submitted: true },
    ]);
  });

  it('null / 空对象 / 脏值不抛：空数组或跳过该键', () => {
    const m = paper();
    expect(m.parsePaperAnswerStates(null)).toEqual([]);
    expect(m.parsePaperAnswerStates({})).toEqual([]);
    expect(m.parsePaperAnswerStates({ '3': null, '4': '' })).toEqual([]);
  });

  it('序列化往返：写进去的作答读回来是同一条', () => {
    const m = paper();
    const st = { question_id: 11, user_answer: 'C', is_correct: true, correct_answer: 'C', submitted: true };
    expect(m.parsePaperAnswerStates({ '11': m.paperAnswerStateValue(st) })).toEqual([st]);
  });
});

describe('resolvePaperResumeIndex：续练游标（AC③）', () => {
  it('已存进度优先（0 < saved < total）', () => {
    const m = paper();
    expect(m.resolvePaperResumeIndex(0, 12, 50)).toBe(12);
    expect(m.resolvePaperResumeIndex(7, 12, 50)).toBe(12);
  });

  it('已存进度不合法时用后端协商值', () => {
    const m = paper();
    expect(m.resolvePaperResumeIndex(7, 0, 50)).toBe(7);
    expect(m.resolvePaperResumeIndex(7, 50, 50)).toBe(7);
  });

  it('都越界 / 题量为 0 ⇒ 从头（不能停在不存在的题上）', () => {
    const m = paper();
    expect(m.resolvePaperResumeIndex(0, 0, 50)).toBe(0);
    expect(m.resolvePaperResumeIndex(50, 50, 50)).toBe(0);
    expect(m.resolvePaperResumeIndex(3, 2, 0)).toBe(0);
  });
});

describe('paperProgressIndex：写侧游标 0 基，与读侧同口径（AC③）', () => {
  it('写进去的游标读回来还是同一题（写 1 基 ⇒ 答完第 1 题退出重进会跳到第 2 题）', () => {
    const m = paper();
    for (const idx of [0, 1, 7, 49]) {
      expect(m.resolvePaperResumeIndex(0, m.paperProgressIndex(idx), 50)).toBe(idx);
    }
  });

  it('负数挡回 0（脏值不能把页面停在不存在的题上）', () => {
    expect(paper().paperProgressIndex(-3)).toBe(0);
  });
});

describe('paperLoadErrorText：失败要可见且不露状态前缀（AC④）', () => {
  it('后端两条业务拒绝的原样透出', () => {
    const m = paper();
    expect(m.paperLoadErrorText('请先兑换该真题卷')).toBe('请先兑换该真题卷');
    expect(m.paperLoadErrorText('该卷暂无已发布题目')).toBe('该卷暂无已发布题目');
  });

  it('404 前缀剥掉（用户不该看到 404:）', () => {
    const m = paper();
    expect(m.paperLoadErrorText('404:真题卷不存在')).toBe('真题卷不存在');
    expect(m.paperLoadErrorText('404:')).toBe(m.PAPER_LOAD_FAIL_TEXT);
  });

  it('空 / null 用兜底文案（不白屏、不静默）', () => {
    const m = paper();
    expect(m.paperLoadErrorText('')).toBe('该卷加载失败，请重试');
    expect(m.paperLoadErrorText(null)).toBe('该卷加载失败，请重试');
  });
});

describe('变异取证：续练游标断言必须有牙', () => {
  it('优先级反过来（后端协商值优先）⇒ 已存进度不再生效', () => {
    const mutated = mutatePaper([[
      'if (savedIndex > 0 && savedIndex < total) return savedIndex',
      'if (backendIndex > 0 && backendIndex < total) return backendIndex',
    ]]);
    const m = uts.loadUts(mutated, {});
    expect(m.resolvePaperResumeIndex(7, 12, 50)).toBe(7);
  });
});
