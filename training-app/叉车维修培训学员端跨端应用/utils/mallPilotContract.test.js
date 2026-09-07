/**
 * mall 试点手术契约测试（TDD: RED → GREEN，refs #640 / refactor epic #638 T02）
 *
 * 钉住试点交付的三类契约，采用项目既有源码契约缝（.uvue/.uts 不可被 jest import）：
 * 1) api 收紧：getCourseListApi 经 requestMapped mapper-callback 出口（#639 出口首个真实消费者）
 * 2) 组件接线：mall.uvue 以显式 import 使用三个模块私有组件（Q17 安置规则）
 * 3) 600 软预算机检：pages/mall/** 全部源文件 ≤600 行（达标后锁住防回潮，Q8）
 * 4) allowlist 不回潮：mall 域文件不得出现在 GUARD_ALLOWLIST（试点验收「清零」的锁）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 模块目录下全部源文件（.uvue/.uts，排除测试） */
function mallSourceFiles(dir = 'pages/mall') {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, dir));
  return out;
}

describe('api 收紧契约（getCourseListApi 经 requestMapped 出口家族）', () => {
  const src = read('api/course.uts');
  const req = read('api/request.uts');

  it('request.uts 提供 GET 便捷面 getMapped：委派 requestMapped 且沿用 get() 的手动 query 序列化（不交平台自动序列化，防跨端漂移）', () => {
    const start = req.indexOf('export function getMapped');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n}', start));
    expect(body).toContain('requestMapped<T>(');
    expect(body).toContain('encodeURIComponent');
  });

  it('course.uts 引入 getMapped 出口', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('getCourseListApi 函数体经 getMapped 且映射函数为 buildCourseListResult', () => {
    const start = src.indexOf('export function getCourseListApi');
    expect(start).toBeGreaterThan(-1);
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('getMapped<CourseListResult>');
    expect(body).toContain('buildCourseListResult');
    // mock 降级行为保持（UI 冻结：无网络时仍出 mock 数据）
    expect(body).toContain('getMockCourseList()');
  });
});

describe('模块私有组件接线契约（Q17 安置：pages/<module>/components/ 显式 import）', () => {
  const page = read('pages/mall/mall.uvue');
  const COMPONENTS = ['mall-sort-bar', 'mall-category-sidebar', 'mall-float-actions'];

  it.each(COMPONENTS)('组件文件存在于 pages/mall/components/%s.uvue', (name) => {
    expect(fs.existsSync(path.join(ROOT, 'pages/mall/components', `${name}.uvue`))).toBe(true);
  });

  it.each(COMPONENTS)('mall.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用三个组件标签（非只 import 不用）', () => {
    expect(page).toMatch(/<mall-sort-bar[\s>]/);
    expect(page).toMatch(/<mall-category-sidebar[\s>]/);
    expect(page).toMatch(/<mall-float-actions[\s>]/);
  });
});

describe('600 行软预算机检（pages/mall/** 达标后锁定）', () => {
  it('mall 模块全部源文件 ≤600 行', () => {
    const over = mallSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层（pages/mall/<file 或 components/<file >>>', () => {
    const deep = mallSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/mall'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

describe('allowlist 不回潮（mall 域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 mall 域文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/pages[/\\]mall/);
    expect(block).not.toMatch(/api[/\\]course\.uts/);
  });
});
