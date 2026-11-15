/**
 * 「分类」视图分组纯函数 · **行为级**测试（B3 #2519，原型 769:1625 笔记本分类态）
 *
 * notebookGroups 的三条口径只有真跑才分得清（源码文本测试证明不了）：
 *   ① 题目笔记按 question_id 聚簇，组标签复用 noteBadgeText（题干摘要，18 截断）；
 *   ② 独立笔记（question_id 为 null/0）聚成一组、question_id=0（页面据此不给跳题）；
 *   ③ 组序 = 首次出现序（不做客户端重排：列表排序真源是后端 updated_at DESC）。
 *
 * 缝：utils/utsHarness.js 的 loadUts（真跑 .uts 模块）。先例 wrongQuestionDisplayBehavior.test.js。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'notebookDisplay.uts');

const display = () => loadUts(DISPLAY_UTS, {});

const q = (question_id, question_content) => ({ question_id, question_content });

describe('notebookGroups：笔记本「分类」视图分组（B3 #2519）', () => {
  const { notebookGroups } = display();

  it('空输入 ⇒ 空数组', () => {
    expect(notebookGroups([])).toEqual([]);
  });

  it('同一题的多条笔记聚成一组，标签 = noteBadgeText(题干摘要)，计数累加', () => {
    const groups = notebookGroups([q(7, '叉车液压系统的主要作用是？'), q(7, '叉车液压系统的主要作用是？')]);
    expect(groups.length).toBe(1);
    expect(groups[0].question_id).toBe(7);
    expect(groups[0].key).toBe('q7');
    expect(groups[0].label).toBe('题目：叉车液压系统的主要作用是？');
    expect(groups[0].count).toBe(2);
  });

  it('独立笔记（null / 0）聚成一组：question_id=0、标签「独立笔记」、不可跳题的键', () => {
    const groups = notebookGroups([q(null, ''), q(7, '题目甲'), q(0, '')]);
    expect(groups.length).toBe(2);
    const standalone = groups.find((g) => g.question_id === 0);
    expect(standalone.label).toBe('独立笔记');
    expect(standalone.count).toBe(2);
    expect(standalone.key).toBe('standalone');
  });

  it('组序 = 首次出现序（不做客户端重排，组序随列表数据走）', () => {
    const groups = notebookGroups([q(9, '题丙'), q(3, '题甲'), q(9, '题丙'), q(3, '题甲')]);
    expect(groups.map((g) => g.question_id)).toEqual([9, 3]);
  });

  it('题干摘要超 18 字截断（与 noteBadgeText 同一口径，不在分组层另写第二份）', () => {
    const long = '这是一条特别特别特别长的题干摘要用来验证十八字截断点';
    const groups = notebookGroups([q(1, long)]);
    expect(groups[0].label).toBe('题目：' + long.substring(0, 18) + '…');
  });
});
