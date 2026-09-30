/**
 * #1421（移动端 ADR-0031 决策 1/3）：课程兑换 CTA 状态机 —— **行为级**守护
 *
 * 被测物是 `utils/coursePointsCta.uts` 的 `buildCourseCta(points_price, entitled, balance)`
 * 纯函数 —— 货架价格行、详情 CTA、「还差 N 分」提示三处界面的**唯一**派生实现（形态来自
 * 原型分支 `prototype/points-redeem-cta` 的真机取证）。四态口径：
 *   - 免费（price null/0）        → 「开始/继续学习」、零兑换痕迹（story 11：免费课完全看不到兑换界面）
 *   - 未拥有·余额足               → 「N 积分兑换」、无提示（余额预判是装饰层，永不拦截 CTA）
 *   - 未拥有·余额不足             → 同 CTA + 「余额不足 · 还差 N 分 · 去赚积分」（数值 = price − balance）
 *   - 已拥有（entitled=true）     → 兑换 UI 消失、回归「开始/继续学习」（决策 6：兑换面不常驻）
 *   - 余额读数缺失（balance null）→ 提示整体缺席、干净降级为纯事后（决策 3：读不到不得报错挡道）
 *
 * 成对断言纪律（docs/agents/guards.md 判据 ③）：每格同时断言**该出现的必须出现、该缺席的必须缺席**；
 * C 组注入变异证明这组断言有判别力（把判据改坏必须变红），不是「文本在场」的假绿。
 *
 * 分工：本文件跑行为（纯函数的五组组合值）；「页面真的消费它」是接线面，
 * 由 `coursesContract.test.js` 的源码锁守（mall / course-detail 各自 import 并渲染其产物）。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const CTA_UTS = path.join(__dirname, '..', 'utils', 'coursePointsCta.uts');

/** 每次调用都是全新模块实例（utsHarness 口径）；本模块零 import，bindings 为空即 fail-closed 通过 */
function load() {
  return loadUts(CTA_UTS, {});
}

describe('A. 五组组合的成对断言（该出现的必须出现、该缺席的必须缺席）', () => {
  test('A1 免费课（price null 与 0 两形态）：直接学习，零兑换痕迹', () => {
    const { buildCourseCta } = load();
    for (const free of [null, 0]) {
      const cta = buildCourseCta(free, null, 500);
      expect(cta.text).toBe('开始学习');
      // 反向断言：免费格不得出现「兑换」字样与余额提示（接口上就没有这两个槽）
      expect(cta.redeem).toBe(false);
      expect(cta.hintText).toBe('');
      expect(cta.shortfall).toBeNull();
    }
    // 已续学形态：可派生文案由调用方给出（继续学习），状态机不抄页面文案第二份
    expect(buildCourseCta(0, null, 10, '继续学习').text).toBe('继续学习');
  });

  test('A2 未拥有·余额足：「N 积分兑换」、可点、无提示（预判不改判兑换面）', () => {
    const { buildCourseCta } = load();
    const cta = buildCourseCta(300, false, 500);
    expect(cta.text).toBe('300 积分兑换');
    expect(cta.redeem).toBe(true);
    expect(cta.hintText).toBe('');
    expect(cta.shortfall).toBeNull();
    // 边界：余额恰好等于价格 ⇒ 不算不足（>= 判据，与 Pro-sheet enough 同式）
    expect(buildCourseCta(300, false, 300).hintText).toBe('');
  });

  test('A3 未拥有·余额不足：同 CTA + 「余额不足 · 还差 N 分 · 去赚积分」，数值正确、CTA 不拦截', () => {
    const { buildCourseCta } = load();
    const cta = buildCourseCta(300, false, 120);
    expect(cta.text).toBe('300 积分兑换');
    // 装饰层判据：提示在场**不影响**可点性（扣不扣由服务端兑换管线裁定）
    expect(cta.redeem).toBe(true);
    expect(cta.hintText).toBe('余额不足 · 还差 180 分 · 去赚积分');
    // 数值单槽（原型候选 c 的 cost-block 数字被吸收进这里，形态整体否决见 ADR-0031）
    expect(cta.shortfall).toBe(180);
    expect(cta.earnHint).toBe(true);
  });

  test('A4 已拥有：兑换 UI 消失、回归学习；即使余额不足也不得带出提示（决策 6）', () => {
    const { buildCourseCta } = load();
    const cta = buildCourseCta(300, true, 0);
    expect(cta.text).toBe('开始学习');
    expect(cta.redeem).toBe(false);
    expect(cta.hintText).toBe('');
    expect(cta.shortfall).toBeNull();
    expect(buildCourseCta(300, true, 0, '继续学习').text).toBe('继续学习');
  });

  test('A5 余额读数缺失：提示整体缺席、干净降级为纯事后（不得报错挡道，决策 3）', () => {
    const { buildCourseCta } = load();
    const cta = buildCourseCta(300, false, null);
    expect(cta.text).toBe('300 积分兑换');
    expect(cta.redeem).toBe(true);
    expect(cta.hintText).toBe('');
    expect(cta.shortfall).toBeNull();
  });

  test('A6 entitled 三态可空（后端只在有主体读路径填）：null 与 false 同为「未拥有」面', () => {
    const { buildCourseCta } = load();
    const unknown = buildCourseCta(300, null, 500);
    expect(unknown.redeem).toBe(true);
    expect(unknown.text).toBe('300 积分兑换');
    // 列表面恒省略 entitled —— 但列表价格行只消费 .text 的价格形态，见 B 组
  });
});

describe('B. 货架价格行（同一函数的第二个消费面，价格语义不得各写一份）', () => {
  test('B1 renderCoursePrice：null / 0 → 免费；N → 「N 积分」（无 ¥、无金额残留）', () => {
    const { renderCoursePrice } = load();
    expect(renderCoursePrice(null)).toBe('免费');
    expect(renderCoursePrice(0)).toBe('免费');
    expect(renderCoursePrice(300)).toBe('300 积分');
    // 反向锁：任何产物都不得含金额符号（story 3：学员不该再看到 ¥）
    expect(renderCoursePrice(300)).not.toContain('¥');
  });
});

describe('C. 判别力自检（成对取证另一半）：注入变异必须让对应判据变红', () => {
  const fs = require('fs');
  const os = require('os');
  const { readText } = require('./utsHarness');

  /** 把变异后的源码写到临时目录再加载（真源一个字节不动，同 chapterNotFoundBehavior 手法） */
  function loadMutated(from, to) {
    const src = readText(CTA_UTS);
    const parts = src.split(from);
    if (parts.length !== 2) {
      throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1421-'));
    const file = path.join(dir, 'coursePointsCta.uts');
    fs.writeFileSync(file, parts.join(to));
    try {
      return loadUts(file, {});
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }

  test('C1 余额判据退化成 `>`（余额=价格被误判不足）⇒ A2 的边界断言必须红', () => {
    const mutated = loadMutated('balance < pointsPrice', 'balance <= pointsPrice');
    expect(mutated.buildCourseCta(300, false, 300).hintText).not.toBe('');
  });

  test('C2 缺口的减法被写反（balance − price）⇒ A3 的数值断言必须红', () => {
    const mutated = loadMutated('price - (balance ?? 0)', '(balance ?? 0) - price');
    expect(mutated.buildCourseCta(300, false, 120).shortfall).not.toBe(180);
  });

  test('C3 entitled 判据写反（把 true 当成未拥有）⇒ A4「已拥有兑换 UI 消失」必须红', () => {
    const mutated = loadMutated('entitled == true', 'entitled == false');
    // 变异后 entitled=true 仍走兑换面 —— 正是 A4 要防的形态
    expect(mutated.buildCourseCta(300, true, 0).redeem).toBe(true);
  });

  test('C4 余额缺失分支被摘（null 参与比较）⇒ A5「提示整体缺席」必须红', () => {
    // 把「balance != null &&」判空条件摘掉：null 会被 coerced 成 0 参与不足判定 ⇒ 冒出提示
    const mutated = loadMutated('balance != null && balance < pointsPrice', 'balance < pointsPrice');
    const cta = mutated.buildCourseCta(300, false, null);
    expect(cta.hintText).not.toBe('');
  });
});
