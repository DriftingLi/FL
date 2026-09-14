/**
 * 构建安装模块契约守护（scripts/lib/build-deploy.ps1）
 *
 * 守护的不变量：
 *   B1 Invoke-BuildAndDeploy 函数存在
 *   B2 返回对象包含 Ok / Deployed / Duration / ManifestRestored / PagesRestored / Error
 *   B3 Full 参数控制是否加 --cleanCache
 *   B4 manifest 还原逻辑（检测到变更时还原）
 *   B5 部署判定逻辑（基线相对）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/build-deploy.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('build-deploy.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('B1: Invoke-BuildAndDeploy function exists', () => {
    expect(src).toContain('function Invoke-BuildAndDeploy');
  });

  test('B2: returns object with Ok, Deployed, Duration, ManifestRestored, PagesRestored, Error', () => {
    expect(src).toContain('Ok');
    expect(src).toContain('Deployed');
    expect(src).toContain('Duration');
    expect(src).toContain('ManifestRestored');
    expect(src).toContain('PagesRestored');
    expect(src).toContain('Error');
  });

  test('B3: Level parameter controls compile behavior', () => {
    expect(src).toContain('Level');
    expect(src).toContain('quick');
    expect(src).toContain('跳过 kotlin-all');
  });

  test('B3b: Full parameter adds --cleanCache', () => {
    expect(src).toContain('--cleanCache');
    expect(src).toContain('$Full');
  });

  test('B4: manifest restore logic', () => {
    expect(src).toContain('manifest.json');
    expect(src).toContain('ManifestRestored');
    expect(src).toContain('Set-Content');
  });

  test('B5: pages.json restore logic', () => {
    expect(src).toContain('pages.json');
    expect(src).toContain('PagesRestored');
  });
});
