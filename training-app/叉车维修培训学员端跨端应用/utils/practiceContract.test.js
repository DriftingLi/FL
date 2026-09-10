/**
 * practice 模块手术契约测试（T06，parent #644 / ADR-0007）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

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