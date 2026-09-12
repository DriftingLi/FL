/**
 * 打卡日历三态计算单元测试
 *
 * utils/checkinCalendar.uts 无法在 Node 中直接运行（UTS），
 * 按仓库惯例（见 format.test.js）以镜像实现验证算法行为；
 * 生产实现见同目录 checkinCalendar.uts 的 computeDayStates / shiftDate / getDayPoints。
 *
 * computeDayStates 已改为 streak 定段（spec #599）：段区间由后端 streak 数值派生，
 * 客户端不自行回溯判定连续。「computeDayStates 三态（streak 定段）」用例集与
 * Web 端 frontend/src/utils/__tests__/checkinCalendar.spec.ts 逐条互为镜像
 * （同一名、同一输入、同一期望）——未来调整三态口径时两份测试文件必须同改。
 */

// ===== 镜像实现（与 checkinCalendar.uts 保持一致）=====

function padZero(n) {
  if (n < 10) return '0' + n;
  return '' + n;
}

function normalizeDate(date) {
  if (date.length > 10) return date.substring(0, 10);
  return date;
}

function parseDate(date) {
  const parts = date.split('-');
  if (parts.length !== 3) return null;
  const y = parseInt(parts[0]);
  const m = parseInt(parts[1]);
  const d = parseInt(parts[2]);
  if (isNaN(y) || isNaN(m) || isNaN(d)) return null;
  if (m < 1 || m > 12 || d < 1 || d > 31) return null;
  return { y, m, d };
}

function shiftDate(date, deltaDays) {
  const p = parseDate(date);
  if (p == null) return date;
  const dt = new Date(p.y, p.m - 1, p.d + deltaDays);
  return padZero(dt.getFullYear()) + '-' + padZero(dt.getMonth() + 1) + '-' + padZero(dt.getDate());
}

function dateOrder(date) {
  const p = parseDate(date);
  if (p == null) return 0;
  return p.y * 10000 + p.m * 100 + p.d;
}

function getDayPoints(days, date) {
  const key = normalizeDate(date);
  for (let i = 0; i < days.length; i++) {
    if (normalizeDate(days[i].date) === key) return days[i].points;
  }
  return 0;
}

function computeDayStates(days, today, streak) {
  const states = [];
  const checkedSet = new Map();
  for (let i = 0; i < days.length; i++) {
    if (days[i].checked) checkedSet.set(normalizeDate(days[i].date), true);
  }
  let end = normalizeDate(today);
  if (streak > 0 && !checkedSet.has(end)) {
    end = shiftDate(end, -1);
  }
  const active = streak > 0;
  const endOrder = dateOrder(end);
  const startOrder = active ? dateOrder(shiftDate(end, -(streak - 1))) : 0;
  for (let i = 0; i < days.length; i++) {
    if (!days[i].checked) {
      states.push('none');
      continue;
    }
    const keyOrder = dateOrder(normalizeDate(days[i].date));
    const inSegment = active && keyOrder >= startOrder && keyOrder <= endOrder;
    states.push(inSegment ? 'streak' : 'past');
  }
  return states;
}

// ===== 测试 =====

const day = (date, checked, points = 0) => ({ date, checked, points });

describe('normalizeDate', () => {
  it('RFC3339 带时间后缀截取前 10 位', () => {
    expect(normalizeDate('2026-09-05T00:00:00Z')).toBe('2026-09-05');
  });

  it('纯日期原样返回', () => {
    expect(normalizeDate('2026-09-05')).toBe('2026-09-05');
  });
});

describe('shiftDate', () => {
  it('前推一天并处理跨月', () => {
    expect(shiftDate('2026-09-01', -1)).toBe('2026-08-31');
  });

  it('后推一天并处理跨年', () => {
    expect(shiftDate('2026-12-31', 1)).toBe('2027-01-01');
  });

  it('非法日期原样返回', () => {
    expect(shiftDate('bad-date', -1)).toBe('bad-date');
  });
});

describe('computeDayStates 三态（streak 定段）', () => {
  it('今日已打卡：streak 段全部实心 streak', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 10),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 5),
      day('2026-09-05', true, 5),
    ];
    expect(computeDayStates(days, '2026-09-05', 5)).toEqual(['streak', 'streak', 'streak', 'streak', 'streak']);
  });

  it('今日未打卡：段尾退到昨日定段（streak=1 单日段）', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
      day('2026-09-03', false),
      day('2026-09-04', true, 5),
      day('2026-09-05', false),
    ];
    // 09-05 未签，段尾退到 09-04，streak=1 → 仅 09-04 在段内
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'past', 'none', 'streak', 'none']);
  });

  it('今日未打卡且段跨多日：段尾=昨日往前 streak 天', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 5),
      day('2026-09-05', false),
    ];
    // 段区间 [09-01, 09-04]
    expect(computeDayStates(days, '2026-09-05', 4)).toEqual(['streak', 'streak', 'streak', 'streak', 'none']);
  });

  it('今日昨日均未签：无连续段（streak=0），历史打卡归 past', () => {
    const days = [day('2026-09-01', true, 5), day('2026-09-04', false), day('2026-09-05', false)];
    expect(computeDayStates(days, '2026-09-05', 0)).toEqual(['past', 'none', 'none']);
  });

  it('跨月段：段起点落在上月，月内段首之前归 past', () => {
    const days = [
      day('2026-08-29', true, 5),
      day('2026-08-30', true, 5),
      day('2026-08-31', true, 5),
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
    ];
    // streak=4，段区间 [08-30, 09-02]，08-29 在段起点之前
    expect(computeDayStates(days, '2026-09-02', 4)).toEqual(['past', 'streak', 'streak', 'streak', 'streak']);
  });

  it('断签中段：断签历史归 past，段内实心', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', false),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 15),
      day('2026-09-05', true, 5),
    ];
    // 今日 09-05 已签，streak=3 → 段区间 [09-03, 09-05]；09-01 断签历史浅底
    expect(computeDayStates(days, '2026-09-05', 3)).toEqual(['past', 'none', 'streak', 'streak', 'streak']);
  });

  it('streak=1 边界：今日已签单日段，段外已打卡归 past', () => {
    const days = [day('2026-09-04', true, 5), day('2026-09-05', true, 5)];
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'streak']);
  });

  it('streak=1 边界：今日未签，昨日单日段', () => {
    const days = [day('2026-09-03', true, 5), day('2026-09-04', true, 5), day('2026-09-05', false)];
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'streak', 'none']);
  });

  it('完全无打卡时全部 none', () => {
    const days = [day('2026-09-04', false), day('2026-09-05', false)];
    expect(computeDayStates(days, '2026-09-05', 0)).toEqual(['none', 'none']);
  });

  it('空数组返回空数组', () => {
    expect(computeDayStates([], '2026-09-05', 0)).toEqual([]);
  });

  it('兼容 RFC3339 日期输入', () => {
    const days = [
      day('2026-09-04T00:00:00Z', true, 5),
      day('2026-09-05', true, 5),
    ];
    expect(computeDayStates(days, '2026-09-05', 2)).toEqual(['streak', 'streak']);
  });
});

describe('getDayPoints', () => {
  it('取该日实发积分', () => {
    const days = [day('2026-09-05', true, 55)];
    expect(getDayPoints(days, '2026-09-05')).toBe(55);
  });

  it('未打卡/不存在为 0', () => {
    const days = [day('2026-09-04', true, 5)];
    expect(getDayPoints(days, '2026-09-05')).toBe(0);
  });

  it('兼容 RFC3339 查询入参', () => {
    const days = [day('2026-09-05', true, 15)];
    expect(getDayPoints(days, '2026-09-05T10:00:00Z')).toBe(15);
  });
});
