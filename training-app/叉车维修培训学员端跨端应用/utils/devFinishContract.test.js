/**
 * 主脚本契约守护（scripts/dev-finish.ps1）
 *
 * 守护的不变量：
 *   F1 参数定义正确（Device, Level, DryRun, Distribute, UpdateBaseline）
 *   F2 步骤顺序正确（环境→级别→测试→编译→构建→截图→对比→证据→还原）
 *   F3 退出码语义正确（env=2, fail=1, ok=0）
 *   F4 -DryRun 不执行实际操作
 *   F5 dot-source 所有子模块
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/dev-finish.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('dev-finish.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('F1: parameters defined correctly', () => {
    expect(src).toContain('[string]$Device');
    expect(src).toContain('[string]$Level');
    expect(src).toContain('[switch]$DryRun');
    expect(src).toContain('[switch]$Distribute');
    expect(src).toContain('[switch]$UpdateBaseline');
  });

  test('F2: steps in correct order', () => {
    const envIdx = src.indexOf('Step 1');
    const levelIdx = src.indexOf('Step 2');
    const testIdx = src.indexOf('Step 3');
    const compileIdx = src.indexOf('Step 4');
    const deployIdx = src.indexOf('Step 5');
    const screenshotIdx = src.indexOf('Step 6');
    const diffIdx = src.indexOf('Step 7');
    const evidenceIdx = src.indexOf('Step 8');
    const restoreIdx = src.indexOf('Step 9');

    expect(envIdx).toBeLessThan(levelIdx);
    expect(levelIdx).toBeLessThan(testIdx);
    expect(testIdx).toBeLessThan(compileIdx);
    expect(compileIdx).toBeLessThan(deployIdx);
    expect(deployIdx).toBeLessThan(screenshotIdx);
    expect(screenshotIdx).toBeLessThan(diffIdx);
    expect(diffIdx).toBeLessThan(evidenceIdx);
    expect(evidenceIdx).toBeLessThan(restoreIdx);
  });

  test('F3: exit codes correct', () => {
    expect(src).toContain('exit 2');  // env unavailable
    expect(src).toContain('exit 1');  // test/compile fail
    expect(src).toContain('exit 0');  // success
  });

  test('F4: DryRun mode', () => {
    expect(src).toContain('DryRun');
    expect(src).toContain('不执行任何操作');
  });

  test('F5: dot-sources all sub-modules', () => {
    expect(src).toContain('env-check.ps1');
    expect(src).toContain('level-detect.ps1');
    expect(src).toContain('test-compile.ps1');
    expect(src).toContain('build-deploy.ps1');
    expect(src).toContain('auto-screenshot.ps1');
    expect(src).toContain('screenshot-diff.ps1');
    expect(src).toContain('evidence-gen.ps1');
  });
});
