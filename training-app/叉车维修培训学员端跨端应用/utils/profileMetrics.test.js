/**
 * 「我的」页面派生逻辑单元测试
 *
 * pages/profile/profile.uvue 中的推导函数按本项目约定实现为「页面内本地函数」
 * （UTS 模板编译器无法解析导入的函数，见 commit 7b6b8b5），
 * 因此与 utils/format.test.js 一样，在此镜像同一份纯逻辑并断言规则。
 * 改动 profile.uvue 中任一推导规则时，必须同步更新这里。
 */

function formatStudyDuration(minutes) {
  if (minutes <= 0) return '0m';
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  if (hours <= 0) return mins + 'm';
  if (mins <= 0) return hours + 'h';
  return hours + 'h' + mins + 'm';
}

function levelFromHours(hours) {
  if (hours < 5) return 1;
  if (hours < 20) return 2;
  if (hours < 50) return 3;
  if (hours < 100) return 4;
  return 5;
}

function rankNameFromCompleted(completed) {
  if (completed <= 0) return '初级工';
  if (completed === 1) return '中级技师';
  if (completed === 2) return '高级技师';
  return '技师';
}

function starCountFromLevel(level) {
  if (level <= 0) return 0;
  if (level > 5) return 5;
  return level;
}

function formatPoints(total) {
  if (total <= 0) return '0';
  if (total > 9999) return '9999+';
  return String(total);
}

describe('「我的」页面派生逻辑', () => {
  describe('formatStudyDuration', () => {
    it('整小时只输出小时', () => {
      expect(formatStudyDuration(48 * 60)).toBe('48h');
      expect(formatStudyDuration(60)).toBe('1h');
    });

    it('非整小时拼接分钟', () => {
      expect(formatStudyDuration(90)).toBe('1h30m');
      expect(formatStudyDuration(48 * 60 + 5)).toBe('48h5m');
    });

    it('不足一小时只输出分钟', () => {
      expect(formatStudyDuration(45)).toBe('45m');
      expect(formatStudyDuration(59)).toBe('59m');
    });

    it('零与负数归零', () => {
      expect(formatStudyDuration(0)).toBe('0m');
      expect(formatStudyDuration(-10)).toBe('0m');
    });
  });

  describe('levelFromHours', () => {
    it('按 5/20/50/100 小时分档', () => {
      expect(levelFromHours(0)).toBe(1);
      expect(levelFromHours(4.9)).toBe(1);
      expect(levelFromHours(5)).toBe(2);
      expect(levelFromHours(19.9)).toBe(2);
      expect(levelFromHours(20)).toBe(3);
      expect(levelFromHours(49)).toBe(3);
      expect(levelFromHours(50)).toBe(4);
      expect(levelFromHours(99)).toBe(4);
      expect(levelFromHours(100)).toBe(5);
      expect(levelFromHours(9999)).toBe(5);
    });
  });

  describe('rankNameFromCompleted', () => {
    it('按已完成课程数给出段位', () => {
      expect(rankNameFromCompleted(0)).toBe('初级工');
      expect(rankNameFromCompleted(1)).toBe('中级技师');
      expect(rankNameFromCompleted(2)).toBe('高级技师');
      expect(rankNameFromCompleted(3)).toBe('技师');
      expect(rankNameFromCompleted(-1)).toBe('初级工');
    });
  });

  describe('starCountFromLevel', () => {
    it('星级与等级一致且夹在 0..5', () => {
      expect(starCountFromLevel(1)).toBe(1);
      expect(starCountFromLevel(3)).toBe(3);
      expect(starCountFromLevel(5)).toBe(5);
      expect(starCountFromLevel(6)).toBe(5);
      expect(starCountFromLevel(0)).toBe(0);
      expect(starCountFromLevel(-2)).toBe(0);
    });

    it('等级始终落在页面 starList 的 5 颗星内', () => {
      for (const hours of [0, 6, 21, 60, 1000]) {
        const stars = starCountFromLevel(levelFromHours(hours));
        expect(stars).toBeGreaterThanOrEqual(0);
        expect(stars).toBeLessThanOrEqual(5);
      }
    });
  });

  describe('formatPoints', () => {
    it('正常积分原样展示', () => {
      expect(formatPoints(1600)).toBe('1600');
      expect(formatPoints(42)).toBe('42');
      expect(formatPoints(9999)).toBe('9999');
    });

    it('超过 9999 折叠，非正数归零', () => {
      expect(formatPoints(10000)).toBe('9999+');
      expect(formatPoints(0)).toBe('0');
      expect(formatPoints(-5)).toBe('0');
    });
  });
});
