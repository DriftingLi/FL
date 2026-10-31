/**
 * 练习面板考点模块行副标题 · **行为级**测试（#1948 摘 mock）
 *
 * 为什么是行为而不是源码文本：本票唯一会写错的地方是「**什么时候不该出现百分比**」——
 * 零作答的考点（后端 by_tag 只收 total>0 的标签，所以它压根不在数组里）必须退回「共 N 题」，
 * 有进度但统计没取到必须退回「已完成 x/y 题」。被摘掉的那版 mock 是把 60+(tag_id*13)%35
 * 无差别塞进每一行，源码文本测试对这种「值对不对」毫无判别力（字符串在、数在，照样绿）。
 *
 * 缝：utils/utsHarness.js 的 loadUts 把 .uts 去类型后当 JS 真执行
 * （先例 jobCredentialDisplayBehavior.test.js / recruitDisplayBehavior.test.js 跑同一形态的纯函数模块）。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'practiceModuleDisplay.uts');

/** 每次取一个全新模块实例（模块级常量幂等，互不串） */
const display = () => loadUts(DISPLAY_UTS, {});

describe('moduleMetaText：考点模块行副标题的唯一文案出口', () => {
  const { moduleMetaText } = display();

  it('by_tag 命中 ⇒ 进度 + 真实正确率（后端 round1 的 33.3 按整数展示）', () => {
    expect(moduleMetaText(3, 10, 33.3, true)).toBe('已完成 3/10 正确率 33%');
  });

  it('正确率 0 是「全错」不是「没数据」，必须照实显示 0%', () => {
    expect(moduleMetaText(4, 8, 0, true)).toBe('已完成 4/8 正确率 0%');
  });

  it('满分照实显示 100%', () => {
    expect(moduleMetaText(5, 5, 100, true)).toBe('已完成 5/5 正确率 100%');
  });

  it('四舍五入而非截断（89.6 ⇒ 90，与 practice-report 页 Math.round 同口径）', () => {
    expect(moduleMetaText(1, 2, 89.6, true)).toBe('已完成 1/2 正确率 90%');
  });

  it('零作答考点（该标签不在 by_tag 里）⇒ 整行不出现「正确率」，退回共 N 题', () => {
    expect(moduleMetaText(0, 8, 0, false)).toBe('共 8 题');
  });

  it('有进度但统计没取到 ⇒ 只报题数，不得凭空补一个百分比', () => {
    expect(moduleMetaText(2, 9, 0, false)).toBe('已完成 2/9 题');
  });

  it('空考点 ⇒ 暂无题目', () => {
    expect(moduleMetaText(0, 0, 0, false)).toBe('暂无题目');
  });

  it('无百分比分支的输出里恒不含「正确率」三字（防 mock 回归的总锁）', () => {
    for (const args of [[0, 8, 0, false], [2, 9, 77, false], [0, 0, 0, false]]) {
      expect(moduleMetaText(...args)).not.toContain('正确率');
    }
  });
});

describe('accuracyText：正确率展示值', () => {
  const { accuracyText } = display();

  it('取整不带小数点', () => {
    expect(accuracyText(33.3)).toBe('33');
    expect(accuracyText(66.7)).toBe('67');
    expect(accuracyText(100)).toBe('100');
  });
});
