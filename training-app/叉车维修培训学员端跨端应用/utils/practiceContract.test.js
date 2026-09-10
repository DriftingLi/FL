/**
 * practice 模块手术契约测试（T06，parent #644 / ADR-0007）
 *
 * 钉住 practice 手术交付的六类契约：
 * 1) 600 行软预算：pages/practice/** 全部源文件 ≤600 行
 * 2) 模块目录 ≤2 层
 * 3) composable 接线：practice-do.uvue / practice.uvue 以显式 import 使用 composable
 * 4) 组件接线零孤儿：页面 import 的组件文件必须存在，组件文件必须被页面引用
 *    （#779 回归教训：practice.uvue 改为 import 四个组件却从未创建文件，master 编译中断）
 * 5) allowlist 不回潮：practice 域文件不得出现在 GUARD_ALLOWLIST
 * 6) 零直发请求：页面层不直接 uni.request
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const PRACTICE_PAGES = ['pages/practice/practice.uvue', 'pages/practice/practice-do.uvue'];

function practiceSourceFiles(dir = 'pages/practice') {
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

describe('600 行软预算机检（pages/practice/** 达标后锁定）', () => {
  it('practice 模块全部源文件 ≤600 行', () => {
    const over = practiceSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层', () => {
    const deep = practiceSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/practice'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

describe('composable 接线契约（T06 拆分：显式 import composable）', () => {
  const doPage = read('pages/practice/practice-do.uvue');
  const mainPage = read('pages/practice/practice.uvue');

  it('practice-do.uvue import usePracticeSession composable', () => {
    expect(doPage).toContain('./composables/usePracticeSession');
  });

  it('practice-do.uvue import usePracticeComments composable', () => {
    expect(doPage).toContain('./composables/usePracticeComments');
  });

  it('practice-do.uvue import usePracticeNotes composable', () => {
    expect(doPage).toContain('./composables/usePracticeNotes');
  });

  it('practice.uvue import usePracticeOverview composable', () => {
    expect(mainPage).toContain('./composables/usePracticeOverview');
  });
});

describe('组件接线零孤儿（#779 回归锁：import 的组件文件必须存在）', () => {
  it('页面 import 的每个模块私有组件文件都真实存在', () => {
    const missing = [];
    for (const page of PRACTICE_PAGES) {
      const src = read(page);
      const re = /from\s+'\.\/components\/([^']+\.uvue)'/g;
      let m;
      while ((m = re.exec(src)) !== null) {
        const target = path.join(ROOT, 'pages/practice/components', m[1]);
        if (!fs.existsSync(target)) missing.push(page + ' -> components/' + m[1]);
      }
    }
    expect(missing).toEqual([]);
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const dir = path.join(ROOT, 'pages/practice/components');
    if (!fs.existsSync(dir)) return;
    const pagesSrc = PRACTICE_PAGES.map((p) => read(p)).join('\n');
    const orphans = fs.readdirSync(dir)
      .filter((f) => f.endsWith('.uvue'))
      .filter((f) => !pagesSrc.includes('./components/' + f));
    expect(orphans).toEqual([]);
  });
});

describe('allowlist 不回潮（practice 域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 practice 域文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/pages[/\\]practice/);
    expect(block).not.toMatch(/api[/\\]practice\.uts/);
  });
});

describe('零直发请求（页面层不直接 uni.request）', () => {
  it('practice-do.uvue 不直接调用 uni.request', () => {
    const src = read('pages/practice/practice-do.uvue');
    expect(src).not.toMatch(/uni\.request\s*\(/);
  });

  it('practice.uvue 不直接调用 uni.request', () => {
    const src = read('pages/practice/practice.uvue');
    expect(src).not.toMatch(/uni\.request\s*\(/);
  });
});