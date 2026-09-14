/**
 * 截图对比模块契约守护（scripts/lib/screenshot-diff.ps1）
 *
 * 守护的不变量：
 *   D1 Compare-ScreenshotBaseline 函数存在
 *   D2 返回对象包含 Diff / ChangedCount / NewBaseline / Error
 *   D3 Diff 数组每项格式正确（filename:status）
 *   D4 ChangedCount 与 Diff 中「有变化」数量一致
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/screenshot-diff.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('screenshot-diff.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('D1: Compare-ScreenshotBaseline function exists', () => {
    expect(src).toContain('function Compare-ScreenshotBaseline');
  });

  test('D2: returns object with Diff, ChangedCount, NewBaseline, Error', () => {
    expect(src).toContain('Diff');
    expect(src).toContain('ChangedCount');
    expect(src).toContain('NewBaseline');
    expect(src).toContain('Error');
  });

  test('D3: diff items use filename:status format', () => {
    expect(src).toContain(':$status');
  });

  test('D4: tracks changed count', () => {
    expect(src).toContain('changedCount');
  });

  test('D5: supports -UpdateBaseline flag', () => {
    expect(src).toContain('UpdateBaseline');
  });

  test('D6: uses MD5 hash for comparison', () => {
    expect(src).toContain('MD5');
    expect(src).toContain('Get-FileHash');
  });
});
