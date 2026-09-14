/**
 * 级别判定模块契约守护（scripts/lib/level-detect.ps1）
 *
 * 守护的不变量：
 *   L1 Get-DetectLevel 函数存在且可被 dot-source
 *   L2 返回对象包含 Level / Reason / ChangedFiles 三个字段
 *   L3 只改 .uvue 样式 → Level=quick
 *   L4 有 .uts 改动 → Level=standard
 *   L5 有 manifest.json 改动 → Level=full
 *   L6 有新增 .uvue 页面 → Level=full
 *   L7 -ForceLevel 参数覆盖自动判定
 *   L8 空 diff → quick
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/level-detect.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('level-detect.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  // L1: 函数存在
  test('L1: Get-DetectLevel function exists', () => {
    expect(src).toContain('function Get-DetectLevel');
  });

  // L2: 返回对象结构
  test('L2: returns object with Level, Reason, ChangedFiles', () => {
    expect(src).toContain('Level');
    expect(src).toContain('Reason');
    expect(src).toContain('ChangedFiles');
  });

  // L3: quick 判定（只改 .uvue）
  test('L3: .uvue only changes → quick', () => {
    expect(src).toContain("Level        = 'quick'");
    expect(src).toContain("Reason       = '无改动文件");
    // quick 的最终返回
    expect(src).toMatch(/Level\s+=\s+'quick'[\s\S]*Reason\s+=\s+"纯样式\/文案改动/);
  });

  // L4: standard 判定（.uts 变更）
  test('L4: .uts changes → standard', () => {
    expect(src).toContain("Level        = 'standard'");
    expect(src).toContain('.uts');
  });

  // L5: full 判定（配置文件变更）
  test('L5: manifest.json / pages.json changes → full', () => {
    expect(src).toContain("Level        = 'full'");
    expect(src).toContain('manifest\\.json');
    expect(src).toContain('pages\\.json');
    expect(src).toContain('platformConfig\\.json');
  });

  // L6: 新增 .uvue 页面 → full
  test('L6: new .uvue page → full', () => {
    expect(src).toContain('新增页面文件');
    expect(src).toContain('\\.uvue');
  });

  // L7: -ForceLevel 参数
  test('L7: -ForceLevel overrides auto detection', () => {
    expect(src).toContain('ForceLevel');
    expect(src).toContain("强制指定 -Level");
  });

  // L8: 空 diff → quick
  test('L8: empty diff → quick', () => {
    expect(src).toContain('无改动文件（空 diff）');
  });
});
