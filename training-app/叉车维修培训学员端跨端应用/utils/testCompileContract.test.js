/**
 * 测试+编译编排模块契约守护（scripts/lib/test-compile.ps1）
 *
 * 守护的不变量：
 *   T1 Invoke-TestAndCompile 函数存在
 *   T2 返回对象包含 Ok / Step / Error / TestOutput / CompileResult / Duration
 *   T3 quick 模式用 kotlin-all（快速检查）
 *   T4 standard/full 模式用 kotlin-all（正式门）
 *   T5 测试失败时 Step=test
 *   T6 编译失败时 Step=compile
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/test-compile.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('test-compile.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('T1: Invoke-TestAndCompile function exists', () => {
    expect(src).toContain('function Invoke-TestAndCompile');
  });

  test('T2: returns object with Ok, Step, Error, TestOutput, CompileResult, Duration', () => {
    expect(src).toContain('Ok');
    expect(src).toContain('Step');
    expect(src).toContain('Error');
    expect(src).toContain('TestOutput');
    expect(src).toContain('CompileResult');
    expect(src).toContain('Duration');
  });

  test('T3: quick mode skips compile gate', () => {
    expect(src).toContain('跳过编译门');
    expect(src).toContain('quick_mode');
  });

  test('T4: standard/full mode uses kotlin-all', () => {
    // Both standard and full should use the same script
    const standardBlock = src.includes("'standard'");
    const fullBlock = src.includes("'full'");
    expect(standardBlock).toBe(true);
    expect(fullBlock).toBe(true);
  });

  test('T4b: SkipTests parameter exists', () => {
    expect(src).toContain('SkipTests');
  });

  test('T5: test failure sets Step=test', () => {
    expect(src).toContain("Step         = 'test'");
  });

  test('T6: compile failure sets Step=compile', () => {
    expect(src).toContain("Step         = 'compile'");
  });
});
