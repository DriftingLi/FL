/**
 * 打卡日历三态计算单元测试
 *
 * utils/checkinCalendar.uts 无法在 Node 中直接运行（UTS），
 * 按仓库惯例（见 format.test.js）以镜像实现验证算法行为；
 * 生产实现见同目录 checkinCalendar.uts 的 computeDayStates / shiftDate / getDayPoints。
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

function getDayPoints(days, date) {
  const key = normalizeDate(date);
  for (let i = 0; i < days.length; i++) {
    if (normalizeDate(days[i].date) === key) return days[i].points;
  }
  return 0;
}

function computeDayStates(days, today) {
  const states = [];
  const checkedSet = new Map();
  for (let i = 0; i < days.length; i++) {
    if (days[i].checked) checkedSet.set(normalizeDate(days[i].date), true);
  }
  let anchor = normalizeDate(today);
  if (!checkedSet.has(anchor)) anchor = shiftDate(anchor, -1);
  const streakSet = new Map();
  while (checkedSet.has(anchor)) {
    streakSet.set(anchor, true);
    anchor = shiftDate(anchor, -1);
  }
  for (let i = 0; i < days.length; i++) {
    const key = normalizeDate(days[i].date);
    if (!days[i].checked) {
      states.push('none');
      continue;
    }
    states.push(streakSet.has(key) ? 'streak' : 'past');
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

describe('computeDayStates 三态', () => {
  it('今日已打卡：连续段全部实心 streak', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 10),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 5),
      day('2026-09-05', true, 5),
    ];
    expect(computeDayStates(days, '2026-09-05')).toEqual(['streak', 'streak', 'streak', 'streak', 'streak']);
  });

  it('今日未打卡：从昨日往回回溯连续段', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
      day('2026-09-03', false),
      day('2026-09-04', true, 5),
      day('2026-09-05', false),
    ];
    // 09-05 未打卡，锚点回溯到 09-04，09-04 单独成段
    expect(computeDayStates(days, '2026-09-05')).toEqual(['past', 'past', 'none', 'streak', 'none']);
  });

  it('断签历史归 past，未来/未打卡归 none', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', false),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 15),
      day('2026-09-05', true, 5),
    ];
    // 今日 09-05 已打卡：03/04/05 连续段实心，01 断签历史浅底，02 未打卡灰底
    expect(computeDayStates(days, '2026-09-05')).toEqual(['past', 'none', 'streak', 'streak', 'streak']);
  });

  it('完全无打卡时全部 none', () => {
    const days = [day('2026-09-04', false), day('2026-09-05', false)];
    expect(computeDayStates(days, '2026-09-05')).toEqual(['none', 'none']);
  });

  it('空数组返回空数组', () => {
    expect(computeDayStates([], '2026-09-05')).toEqual([]);
  });

  it('兼容 RFC3339 日期输入', () => {
    const days = [
      day('2026-09-04T00:00:00Z', true, 5),
      day('2026-09-05', true, 5),
    ];
    expect(computeDayStates(days, '2026-09-05')).toEqual(['streak', 'streak']);
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
