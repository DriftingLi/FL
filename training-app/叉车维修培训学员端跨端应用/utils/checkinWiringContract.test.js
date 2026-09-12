/**
 * 打卡契约迁移收编契约测试（TDD: RED → GREEN 钉住行为契约，refs #661）
 *
 * 背景：#591（7b61761）把打卡 API 迁出 forum.uts 至 api/checkin.uts（路由
 * /check-in、/check-in/calendar，见 ADR-0028），后端同步删除 /forum/check-in/*。
 * 但迁移只做了一半：三个页面仍 import api/forum.uts 的僵尸函数（打已删路由，
 * 生产实测 404），且 checkin.uts 按新契约读字段却 as 旧类型（字段不相交，
 * 一旦有消费者即 Kotlin 编译错——同 #639 修过的 No value passed 家族）。
 *
 * 缝：api/*.uts 与页面 .uvue 无法在 jest 中 import，沿用源码契约测试缝。
 * 先例：utils/secureStorage.test.js、utils/aiChatModesContract.test.js。
 *
 * 钉住的契约：
 * 1) 消费面：打卡三页面的 checkInApi/getCheckInCalendarApi 必须 import 自 api/checkin
 * 2) 死路由清零：api/ 与 pages/ 全域不得再引用 /forum/check-in
 * 3) 类型对齐后端 ADR-0028：CheckInResult/CheckInCalendarResult 字段与
 *    后端 checkin_service.go 的 json tag 一致（streak/total/today_checked/points）
 * 4) 页面对新契约的字段消费闭环：不残留旧字段名（consecutive_days 等）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const CHECKIN_PAGES = [
  'pages/forum/check-in.uvue',
  'pages/forum/forum.uvue',
  'pages/profile/profile.uvue',
];

/** 提取类型声明体（type X = { ... }） */
function typeBody(name) {
  // CheckIn types moved to types/points.uts after types split
  const src = read('types/points.uts');
  const start = src.indexOf(`export type ${name} = {`);
  if (start === -1) throw new Error(`未找到类型 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('打卡消费面接线契约（页面 import 源 = api/checkin）', () => {
  it.each(CHECKIN_PAGES)('%s 的打卡函数 import 自 api/checkin', (rel) => {
    const src = read(rel);
    if (!/checkInApi|getCheckInCalendarApi/.test(src)) return; // 该页不消费则跳过
    const badImport = new RegExp(
      "import\\s*\\{[^}]*\\b(checkInApi|getCheckInCalendarApi)\\b[^}]*\\}\\s*from\\s*'[^']*api/forum'"
    );
    expect(badImport.test(src)).toBe(false);
    const goodImport = new RegExp(
      "import\\s*\\{[^}]*\\b(checkInApi|getCheckInCalendarApi)\\b[^}]*\\}\\s*from\\s*'[^']*api/checkin'"
    );
    expect(goodImport.test(src)).toBe(true);
  });
});

describe('死路由清零契约（/forum/check-in 全域不得引用）', () => {
  it('api/ 与 pages/ 源码不再以请求调用形式引用 forum/check-in', () => {
    const offenders = [];
    const walk = (dir) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) { walk(p); continue; }
        if (!/\.(uts|uvue)$/.test(e.name) || /\.test\./.test(e.name)) continue;
        // 只抓请求调用形态：get('/forum/check-in…) / post('/forum/check-in…)；
        // 页面导航路径 /pages/forum/check-in 与历史注释不属违例
        if (/(get|post)\s*\(\s*['"]\/forum\/check-in/.test(fs.readFileSync(p, 'utf8'))) offenders.push(path.relative(ROOT, p));
      }
    };
    walk(path.join(ROOT, 'api'));
    walk(path.join(ROOT, 'pages'));
    expect(offenders).toEqual([]);
  });

  it('forum.uts 不再导出打卡函数（僵尸段已删除）', () => {
    const src = read('api/forum.uts');
    expect(src).not.toMatch(/export function (checkInApi|getCheckInCalendarApi)\b/);
    expect(src).not.toMatch(/function (buildCheckInResult|buildCheckInCalendarResult)\b/);
  });
});

describe('类型对齐后端 ADR-0028 契约', () => {
  it('CheckInResult 字段 = checked/streak/total/today_checked/points', () => {
    const body = typeBody('CheckInResult');
    for (const f of ['checked :', 'streak :', 'total :', 'today_checked :', 'points :']) {
      expect(body).toContain(f);
    }
    for (const gone of ['checked_in', 'consecutive_days', 'points_earned', 'total_points']) {
      expect(body).not.toContain(gone);
    }
  });

  it('CheckInCalendarResult 字段 = days/streak/total/today_checked', () => {
    const body = typeBody('CheckInCalendarResult');
    for (const f of ['days :', 'streak :', 'total :', 'today_checked :']) {
      expect(body).toContain(f);
    }
    for (const gone of ['consecutive_days', 'total_points', 'points_per_day']) {
      expect(body).not.toContain(gone);
    }
  });
});

describe('页面字段消费闭环（不残留旧契约字段名）', () => {
  it('打卡三页面不再读取旧字段 consecutive_days/total_points/points_earned', () => {
    const offenders = [];
    for (const rel of CHECKIN_PAGES) {
      const src = read(rel);
      // 打卡结果的变量名三页统一为 result；profile 的 data.total_points 属积分余额接口，非本契约范围
      for (const gone of ['consecutive_days', 'total_points', 'points_earned', 'points_per_day', 'checked_in']) {
        if (new RegExp('\\bresult\\.' + gone + '\\b').test(src)) {
          offenders.push(`${rel}: ${gone}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
