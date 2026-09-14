/**
 * 构建安装模块契约守护（scripts/lib/build-deploy.ps1）
 *
 * 背景（2026-09-14）：本模块第一版犯了**假绿** —— 构造了 `$launchArgs` 却**从未调用** cli，
 * 且无条件 `Deployed = $true`，于是 `dev:finish` 打印「✅ 已到设备（耗时 0s）」而实际什么都没做。
 * 当时的契约测试只校「结构」（函数在、字段齐），校不出「没执行操作」。
 *
 * 故本守护按三层升级（用户 2026-09-14 裁定：至少做到结构层 + 文本契约层）：
 *   结构层     fail-closed 默认值 —— 让「没做」无法被误报成成功
 *   文本契约层 断言「存在对 hx-run 的真实调用」，挡住「定义了变量但从不使用」
 *
 * 守护的不变量：
 *   B1  Invoke-BuildAndDeploy 存在
 *   B2  返回对象含 Ok / Deployed / Reason / Duration / ManifestRestored / PagesRestored / Error
 *   B3  Level 参数存在（quick/standard/full）
 *   B4  【结构层】$deployed 默认 $false（fail-closed）
 *   B5  【文本层】存在对 hx-run.ps1 的**真实调用**（复用已实测的真运行路径，不自己实现 launch）
 *   B6  解析 HX_RUN_DEPLOY 机检行作为部署判据
 *   B7  【结构层】机检行缺失 ⇒ 判 Ok=false（不假装成功）
 *   B8  【文本层】不得出现无条件的 `Deployed = $true`（第一版的假绿字面量）
 *   B9  【文本层】不得残留死代码 $launchArgs（定义了却从不使用的变量）
 *   B10 quick 模式**明说不部署**（Q-2 口径：quick 只编译诊断）
 *   B11 manifest.json 还原逻辑保留
 *   B12 pages.json 还原逻辑保留
 *   B13 Full 参数透传给 hx-run
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

  test('B2: returns Ok/Deployed/Reason/Duration/ManifestRestored/PagesRestored/Error', () => {
    ['Ok', 'Deployed', 'Reason', 'Duration', 'ManifestRestored', 'PagesRestored', 'Error'].forEach((f) => {
      expect(src).toContain(f);
    });
  });

  test('B3: Level parameter exists', () => {
    expect(src).toContain('$Level');
    expect(src).toContain("'quick'");
  });

  test('B4: 【结构层】$deployed defaults to $false (fail-closed)', () => {
    expect(src).toMatch(/\$deployed\s*=\s*\$false/);
  });

  test('B5: 【文本层】really invokes hx-run.ps1 (reuse the tested real-run path)', () => {
    // 必须**真的把它当脚本调用**：先解析出路径变量，再把该变量交给 -File。
    // 只断言「源码里出现过 hx-run.ps1」会被**注释里的提及**满足 ⇒ 假通过（本测试首版即中招）。
    expect(src).toMatch(/\$hxRun\s*=\s*Join-Path/);
    expect(src).toMatch(/hx-run\.ps1/);
    expect(src).toMatch(/-File\s+\$hxRun/);
  });

  test('B6: parses the HX_RUN_DEPLOY machine line as the deploy verdict', () => {
    expect(src).toContain('HX_RUN_DEPLOY');
  });

  test('B7: 【结构层】missing machine line ⇒ Ok=false (never fake success)', () => {
    // hx-run 未正常收口 / 机检行缺失时必须判失败，而不是放过
    expect(src).toMatch(/\$ok\s*=\s*\$false/);
    expect(src).toMatch(/机检行|\$runLine/);
  });

  test('B8: 【文本层】no unconditional `Deployed = $true` (the v1 false-green literal)', () => {
    // 返回对象里 Deployed 必须绑到变量，不得直接写 $true
    expect(src).not.toMatch(/Deployed\s*=\s*\$true/);
  });

  test('B9: 【文本层】no leftover dead code $launchArgs', () => {
    expect(src).not.toContain('$launchArgs');
  });

  test('B10: quick mode explicitly does NOT deploy', () => {
    expect(src).toContain('快速模式：不部署');
  });

  test('B11: manifest.json restore logic kept', () => {
    expect(src).toContain('manifest.json');
    expect(src).toContain('ManifestRestored');
  });

  test('B12: pages.json restore logic kept', () => {
    expect(src).toContain('pages.json');
    expect(src).toContain('PagesRestored');
  });

  test('B13: Full is forwarded to hx-run', () => {
    expect(src).toMatch(/\$Full/);
    expect(src).toMatch(/-Full/);
  });

  test('B14: 【文本层】forwards -TimeoutSeconds to hx-run', () => {
    // 同 T11：hx-run 默认 900 秒对冷缓存编译偏短，必须显式透传。
    expect(src).toMatch(/-TimeoutSeconds/);
    expect(src).toContain('$RunTimeoutSeconds');
  });
});
