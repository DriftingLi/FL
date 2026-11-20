/**
 * 商城封面兜底 —— **行为级**守护（2026-11-20 缺图收口）
 *
 * 被测物是 `utils/mallCover.uts` 的 `courseCoverName(title)` 纯函数：商品名关键词 → 包内书封名。
 * 它是「无图商品显示哪张封面」的**唯一**派生实现，三个消费面（课程卡 / 热门段课程 / 未来的列表）
 * 都吃这一份，所以配错一张就是整屏配错 —— 这条映射值得真跑一遍，而不是只锁模板里的表达式形状。
 *
 * 为什么是这五个关键词：列表 DTO 没有商品类型字段（types/index.uts 的 CourseItem 只有标题），
 * 只能用名称对号；落不中一律「教材」。五种书封 = 设计稿 824:6971 那一套。
 *
 * 成对断言纪律（docs/agents/guards.md 判据 ③）：既断「该落到哪张」，也断「不落空 / 不越界」。
 * 接线面（模板真的拼了这个函数）由 `utils/mallPilotContract.test.js` 的「默认书封」组守。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const UTS = path.join(__dirname, '..', 'utils', 'mallCover.uts');

/** 每次调用都是全新模块实例（utsHarness 口径）；本模块零 import，bindings 为空即 fail-closed 通过 */
function load() {
  return loadUts(UTS, {});
}

const FIVE = ['textbook', 'ebook', 'material-pro', 'material-skill', 'material-aux'];

describe('courseCoverName：五个关键词各归其位', () => {
  test('「电子图书」→ ebook', () => {
    expect(load().courseCoverName('叉车维修电子图书精讲')).toBe('ebook');
  });

  test('「专业技术」→ material-pro', () => {
    expect(load().courseCoverName('叉车专业技术进阶')).toBe('material-pro');
  });

  test('「技能」→ material-skill', () => {
    expect(load().courseCoverName('货叉操作技能训练')).toBe('material-skill');
  });

  test('「辅助」→ material-aux', () => {
    expect(load().courseCoverName('液压系统辅助资料')).toBe('material-aux');
  });

  test('一个关键词都不落 → textbook（默认书封）', () => {
    const m = load();
    expect(m.courseCoverName('叉车基础知识概述')).toBe('textbook');
    expect(m.courseCoverName('')).toBe('textbook');
  });
});

describe('courseCoverName：取值面收口（不空、不越界）', () => {
  test('六门真实课程标题各落到五张之内（货叉操作技能训练吃「技能」，其余落教材）', () => {
    const m = load();
    const real = [
      '叉车基础知识概述',
      '液压系统原理与维护',
      '叉车安全操作规范',
      '日常检查与保养指南',
      '货叉操作技能训练',
      '故障诊断与排除进阶',
    ];
    const got = real.map((t) => m.courseCoverName(t));
    expect(got).toEqual(['textbook', 'textbook', 'textbook', 'textbook', 'material-skill', 'textbook']);
    // 成对：每一格都必须是五张之一，绝不返回空串或目录外名字（否则模板拼出的是 404 路径）
    for (const name of got) expect(FIVE).toContain(name);
  });

  test('任意标题都不会返回空串（模板把它拼进路径，空串会变成 cover-.png）', () => {
    const m = load();
    for (const t of ['', '啊', '课外读物', 'AI 专业版', '解锁真题套卷1套']) {
      const name = m.courseCoverName(t);
      expect(name.length).toBeGreaterThan(0);
      expect(FIVE).toContain(name);
    }
  });
});
