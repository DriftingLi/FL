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

  // L3: quick 判定（未命中运行时面）
  // 2026-09-15 修订：🟢 的 `Reason` 必须**分两种写清** —— 「工具链/测试改动」≠「纯样式/文案改动」。
  // 起因：两次实际会话改的是 `.ps1`，却被报成「纯样式/文案改动（1 个文件）」，误导使用者以为改动面是 UI。
  // 分档（🟢）不变，变的是**为什么**。
  test('L3: no runtime surface → quick, and Reason distinguishes toolchain from UI', () => {
    expect(src).toContain("Level        = 'quick'");
    // 空 diff 仍走 quick（独立措辞）
    expect(src).toContain("Reason       = '无改动文件");
    // 分支一：只有工具链/测试/文档类文件 ⇒ 必须点名「工具链」，不得混进「纯样式」
    expect(src).toMatch(/工具链\/测试改动，未命中运行时面/);
    // 分支二：其余（含只改 .uvue）⇒ 补上「未命中运行时面」
    expect(src).toMatch(/Level\s+=\s+'quick'[\s\S]*Reason\s+=\s+"纯样式\/文案改动，未命中运行时面/);
    // 两个分支都必须存在（防止有人删掉工具链分支、又退回单一文案）
    expect(src).toContain('$toolchainFiles');
    expect(src).toContain('$uvueFiles');
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
