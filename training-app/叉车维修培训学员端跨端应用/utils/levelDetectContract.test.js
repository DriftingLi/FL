/**
 * 级别判定模块契约守护（scripts/lib/level-detect.ps1）
 *
 * 守护的不变量：
 *   L1 Get-DetectLevel 函数存在且可被 dot-source
 *   L2 返回对象包含 Level / Reason / ChangedFiles 三个字段
 *   L3 quick 判定 + `Reason` 的三类分流（2026-09-16 修订：`[.uvue]` 分支不得声称「未命中运行时面」）
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

  // L3: quick 判定（**本地**口径）—— `Reason` 必须分三类写清，且不得混淆两个口径
  // 2026-09-15 修订：改 `.ps1` 被报成样式改动 ⇒ 点名「工具链/测试改动」。
  // 2026-09-16 修订（#1037）：只改既有 `.uvue` 被报成「未命中运行时面」，而 `.uvue` 在验收门口径
  //   （`pr-evidence` 的 `isRuntimeFile`）里就是运行时面 ⇒ 该分支必须写出「验收门口径 / ①③④」。
  //   ⚠️ 本用例是**文本**断言（会被注释满足）；分支真的走哪条由 levelDetectBehavior.test.js 的
  //   G4–G6 用临时仓库真跑一遍来钉 —— 这是 ADR-0011 G1–G3 的教训：文本断言证明不了分支可执行。
  test('L3: quick 判定 + Reason 三类分流（工具链 / .uvue / 其余）', () => {
    expect(src).toContain("Level        = 'quick'");
    // 空 diff 仍走 quick（独立措辞）
    expect(src).toContain("Reason       = '无改动文件");
    // 分支一：含 .uvue ⇒ 写出「门口径 + ①③④」，不得把它当免证据
    expect(src).toMatch(/模板\/样式改动[\s\S]{0,400}?验收门口径[\s\S]{0,200}?①③④/);
    // 分支二：只有工具链/测试/文档类文件 ⇒ 必须点名「工具链」
    expect(src).toMatch(/工具链\/测试改动，未命中运行时面/);
    // 分支三：其余 ⇒ 「非运行时面改动」+「未命中运行时面」
    expect(src).toMatch(/非运行时面改动，未命中运行时面/);
    // 三个分支都必须存在（防止有人删掉某支、又退回单一文案）
    expect(src).toContain('$toolchainFiles');
    expect(src).toContain('$uvueFiles');
    // 🟢 收尾建议句的单点真源（行为断言见 levelDetectBehavior.test.js G7）
    expect(src).toContain('function Get-QuickEvidenceHint');
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
