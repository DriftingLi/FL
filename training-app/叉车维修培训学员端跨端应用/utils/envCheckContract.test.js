/**
 * 环境检测模块契约守护（scripts/lib/env-check.ps1）
 *
 * 守护的不变量：
 *   E1 Test-BuildEnv 函数存在且可被 dot-source
 *   E2 返回对象包含 Ok / Error / Device / CliPath 字段
 *   E3 Ok=false 时 Error 字段非空
 *   E4 Ok=true 时 Device 和 CliPath 非空
 *   E5 各失败场景的 Error 消息包含关键词
 *   E6 检测 adb 设备（Resolve-AdbExeLocal）
 *   E7 检测 HBuilderX CLI（Resolve-CliPathLocal）
 *   E8 复用 hx-busy.ps1 忙检测
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/env-check.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('env-check.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  // E1: 函数存在
  test('E1: Test-BuildEnv function exists', () => {
    expect(src).toContain('function Test-BuildEnv');
  });

  // E2: 返回对象结构
  test('E2: returns object with Ok, Error, Device, CliPath', () => {
    expect(src).toContain('Ok');
    expect(src).toContain('Error');
    expect(src).toContain('Device');
    expect(src).toContain('CliPath');
  });

  // E3: Ok=false 时 Error 非空
  test('E3: Ok=false includes Error message', () => {
    expect(src).toContain("Ok    = $false");
    expect(src).toContain('Error =');
  });

  // E4: Ok=true 时 Device 和 CliPath 非空
  test('E4: Ok=true includes Device and CliPath', () => {
    expect(src).toContain("Ok      = $true");
    expect(src).toContain('Device  =');
    expect(src).toContain('CliPath =');
  });

  // E5: 失败消息包含关键词
  test('E5: error messages contain keywords', () => {
    expect(src).toContain('adb.exe');
    expect(src).toContain('HBuilderX');
    expect(src).toContain('设备');
  });

  // E6: 检测 adb
  test('E6: detects adb device', () => {
    expect(src).toContain('Resolve-AdbExeLocal');
    expect(src).toContain('Get-OnlineDevices');
  });

  // E7: 检测 HBuilderX CLI
  test('E7: detects HBuilderX CLI', () => {
    expect(src).toContain('Resolve-CliPathLocal');
  });

  // E8: 复用 hx-busy.ps1
  test('E8: uses hx-busy.ps1 for busy detection', () => {
    expect(src).toContain('hx-busy.ps1');
    expect(src).toContain('Wait-HxFree');
  });

  // E9（2026-09-14 加）：Q-A 用的轻量检查 Test-StaticEnv **必须不取锁**。
  // 否则「秒级静态守护」会在别的会话持锁时白等到超时 ⇒ 定位名不副实。
  test('E9: Test-StaticEnv exists and does NOT take the HBuilderX lock', () => {
    const at = src.indexOf('function Test-StaticEnv');
    expect(at).toBeGreaterThan(-1);
    const rest = src.slice(at);
    const nextFn = rest.slice(1).search(/\nfunction\s/);
    const body = nextFn === -1 ? rest : rest.slice(0, nextFn + 1);
    expect(body).not.toContain('Wait-HxFree');
    expect(body).not.toContain('Acquire-HxLock');
    expect(body).not.toContain('Resolve-TargetDevice');
    expect(body).not.toContain('Resolve-CliPathLocal');
    expect(body).toContain('pages.json');
  });
});
