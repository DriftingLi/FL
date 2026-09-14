/**
 * 自动截图模块契约守护（scripts/lib/auto-screenshot.ps1）
 *
 * 守护的不变量：
 *   S1 Invoke-AutoScreenshot 函数存在
 *   S2 返回对象包含 Ok / Screenshots / Skipped / HashConflicts / Error
 *   S3 读取 pages.json 获取页面列表
 *   S4 跳过的页面在 Skipped 中
 *   S5 使用 HBuilderX CLI --pagePath 导航（非 am start）
 *   S6 使用 SHA256 hash 做反假绿判据（连续相同 hash → 切页未生效）
 *   S7 支持 -CliPath 参数指定 HBuilderX CLI 路径
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

  test('S2: returns object with Ok, Screenshots, Skipped, HashConflicts, Error', () => {
    expect(src).toContain('Ok');
    expect(src).toContain('Screenshots');
    expect(src).toContain('Skipped');
    expect(src).toContain('HashConflicts');
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

  test('S5: uses HBuilderX CLI --pagePath for navigation (not am start)', () => {
    expect(src).toContain('--pagePath');
    expect(src).toContain('launch');
    expect(src).not.toMatch(/am\s+start\s+-d\s+uniapp:\/\//);
  });

  test('S6: SHA256 hash anti-false-green check', () => {
    expect(src).toContain('Get-FileHash');
    expect(src).toContain('SHA256');
    expect(src).toContain('seenHashes');
    expect(src).toContain('hash 与上一页相同');
  });

  test('S7: -CliPath parameter', () => {
    expect(src).toContain('CliPath');
    expect(src).toContain('$CliPath');
  });
});
