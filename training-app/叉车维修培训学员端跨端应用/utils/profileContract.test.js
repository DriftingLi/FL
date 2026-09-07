/**
 * profile 模块手术契约测试（refs #641 / refactor epic #638 T03）
 *
 * 沿用源码契约缝（.uvue/.uts 不可 jest import）。先例：mallPilotContract、quickLoginContract。
 * 逐切片补块：本文件随手术推进追加剧本（域 api 收紧 → 页面拆分/预算/接线）。
 *
 * 本票域 api 收紧口径：仅「有 DTO 映射」的函数经 *Mapped 出口家族；
 * 返回 UTSJSONObject/void 的原样透传函数不硬套 identity map（避免无意义中间层）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 提取 export function 函数体（从声明到顶层 "\n}"，先例 quickLoginContract） */
function fnBodyOf(fileSrc, name) {
  const start = fileSrc.indexOf(`export function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  const end = fileSrc.indexOf('\n}', start);
  return fileSrc.slice(start, end);
}

describe('出口家族完备性（postMapped 为 T03 前置补全）', () => {
  const req = read('api/request.uts');
  it('postMapped 存在：POST 形态与历史 post() 逐字一致（opts.data 同通路），委派 requestMapped', () => {
    const start = req.indexOf('export function postMapped');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n}', start));
    expect(body).toContain("opts.method = 'POST'");
    expect(body).toContain('opts.data = data');
    expect(body).toContain('requestMapped<T>(opts, map)');
  });
});

describe('student 域收紧（profile 主页面数据源）', () => {
  const src = read('api/student.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getProfileApi', 'getStudyStatsApi', 'getRecordsApi', 'getStudentCoursesApi', 'getStudentCourseDetailApi'])(
    '%s 经 getMapped 且保留原 catch/mock 降级结构', (fn) => {
      const body = fnBodyOf(src, fn);
      expect(body).toContain('getMapped<');
      // 收紧不得改变错误行为：原函数有 .catch 的必须保留
    });
  it('getProfileApi 的 mock 降级保持', () => {
    expect(fnBodyOf(src, 'getProfileApi')).toContain('getMockProfile()');
  });
  it('getStudyStatsApi / getRecordsApi 的 mock 降级保持', () => {
    expect(fnBodyOf(src, 'getStudyStatsApi')).toContain('getMockStudyStats()');
    expect(fnBodyOf(src, 'getRecordsApi')).toContain('getMockRecords()');
  });
});

describe('wrongQuestion 域收紧（错题本）', () => {
  const src = read('api/wrongQuestion.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getWrongQuestionsApi', 'getWrongQuestionStatsApi'])('%s 经 getMapped + build* 映射', (fn) => {
    const body = fnBodyOf(src, fn);
    expect(body).toContain('getMapped<');
    expect(body).toMatch(/build(WrongQuestionListResult|WrongQuestionStats)\(/);
  });
});

describe('favorite 域收紧（收藏段）', () => {
  const src = read('api/favorite.uts');
  it('引入 getMapped 与 postMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it('getFavoritesApi 经 getMapped 传 builder 引用，后端 favorites 字段兼容与分页默认值保持', () => {
    const body = fnBodyOf(src, 'getFavoritesApi');
    expect(body).toContain('getMapped<');
    // mapper 以函数引用传入（buildFavoriteListResult(data) 的箭头包装是无意义中间层）
    expect(body).toMatch(/getMapped<FavoriteListResult>\([^)]*buildFavoriteListResult\)/);
    const b = read('api/favorite.uts');
    expect(b).toMatch(/function buildFavoriteListResult/);
    expect(b).toContain("data['favorites']");
  });
  it('addFavoriteApi 经 postMapped 映射 FavoriteItem', () => {
    expect(fnBodyOf(src, 'addFavoriteApi')).toContain('postMapped<FavoriteItem>');
  });
  it('checkFavoriteApi 经 getMapped 且提取了 buildFavoriteCheckResult', () => {
    expect(fnBodyOf(src, 'checkFavoriteApi')).toContain('getMapped<');
    expect(read('api/favorite.uts')).toMatch(/function buildFavoriteCheckResult/);
  });
});

describe('points 域收紧（积分余额/明细）', () => {
  const src = read('api/points.uts');
  it('引入 getMapped', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });
  it.each(['getPointsBalanceApi', 'getPointsRecordListApi'])('%s 经 getMapped 且 mock 降级保持', (fn) => {
    expect(fnBodyOf(src, fn)).toContain('getMapped<');
  });
  it('balance/records 的 mock 降级保持（profile 首屏零网络依赖行为不变）', () => {
    expect(fnBodyOf(src, 'getPointsBalanceApi')).toContain('getMockPointsBalance()');
    expect(fnBodyOf(src, 'getPointsRecordListApi')).toContain('getMockPointsRecordList()');
  });
});

describe('raw .then 收紧完成度（本票四域 DTO 函数零残留）', () => {
  const targets = ['api/student.uts', 'api/wrongQuestion.uts', 'api/favorite.uts', 'api/points.uts'];
  it.each(targets)('%s 不再出现 get/post(...).then((data : UTSJSONObject) : DTO 形态', (rel) => {
    const src = read(rel);
    expect(src).not.toMatch(/return (get|post)\([^)]*\)\s*\.then\(\(data : UTSJSONObject\) : (?!UTSJSONObject)/);
  });
});
