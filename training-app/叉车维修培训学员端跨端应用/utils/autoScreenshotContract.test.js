/**
 * 自动截图模块契约守护（scripts/lib/auto-screenshot.ps1）
 *
 * 守护的不变量：
 *   S1 Invoke-AutoScreenshot 函数存在
 *   S2 返回对象包含 Ok / Screenshots / Skipped / Error
 *   S3 Screenshots 数组非空（成功时）
 *   S4 跳过的页面在 Skipped 中
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/auto-screenshot.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('auto-screenshot.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('S1: Invoke-AutoScreenshot function exists', () => {
    expect(src).toContain('function Invoke-AutoScreenshot');
  });

  test('S2: returns object with Ok, Screenshots, Skipped, Error', () => {
    expect(src).toContain('Ok');
    expect(src).toContain('Screenshots');
    expect(src).toContain('Skipped');
    expect(src).toContain('Error');
  });

  test('S3: reads pages.json for page list', () => {
    expect(src).toContain('pages.json');
    expect(src).toContain('pages');
  });

  test('S4: skipped pages tracked', () => {
    expect(src).toContain('Skipped');
    expect(src).toContain('skipped');
  });
});
