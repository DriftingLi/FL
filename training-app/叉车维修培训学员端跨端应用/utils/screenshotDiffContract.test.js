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

  // D7（2026-09-15，#1027 收尾真机实测）：`Get-ChildItem` 的结果**必须**用 `@(...)` 包住。
  //   症状：只命中**一个** png 时它会退化成**标量**，而调用方 `dev-finish.ps1` 是以
  //   `Set-StrictMode -Version Latest` 跑的 ⇒ `$currentFiles.Count` 抛「在此对象上找不到属性 Count」
  //   ⇒ **步骤 7 直接崩**（基线目录不存在 + 恰好一页改动 = 最常见的首次运行形态；实测撞到）。
  //   行为面由 `screenshotDiffSingleFileBehavior.test.js`（C1–C4）在运行期另钉。
  test('D7: 【2026-09-15】Get-ChildItem 结果必须 @() 包住（StrictMode 下单文件会退化成标量）', () => {
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');
    expect(code).not.toMatch(/\$\w+\s*=\s*Get-ChildItem/);
    expect(code).toMatch(/\$currentFiles\s*=\s*@\(Get-ChildItem/);
    expect(code).toMatch(/\$baselineFiles\s*=\s*@\(Get-ChildItem/);
  });
});
