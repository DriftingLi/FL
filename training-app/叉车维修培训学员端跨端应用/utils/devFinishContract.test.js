/**
 * 主脚本契约守护（scripts/dev-finish.ps1）
 *
 * 背景（2026-09-14，A-3 裁决）：HBuilderX 是**单实例串行资源**，而 `dev:finish` 有**三个**步骤需要它
 * （编译 → 真运行部署 → 逐页截图），且其中后两步**共享设备状态**：若分三次各拿一次锁，
 * 别的会话可能在「部署到目标页」与「截图」之间插进来 launch 到别的页 ⇒ **截到别人的页面**。
 * 故改为**一个临界区**覆盖步骤 4–6；步骤 1–3（不碰 HBuilderX）与 7–9（纯本地）留在锁外，
 * 尽量缩短持有时间。
 *
 * **锁交接**：`hx-busy.ps1` 的锁**按 PID 判定且不可重入** ⇒ 父进程持锁后再调子脚本会**自死锁**。
 * 故持锁后调 `Set-HxLockOwnerEnv`（写 `$env:HX_LOCK_OWNER`）交接，子脚本认到同一持有者就复用；
 * `finally` 里 `Clear-HxLockOwnerEnv` + `Release-HxLock`。
 *
 * 守护的不变量：
 *   F1  参数定义正确（Device, Level, DryRun, Distribute, UpdateBaseline, HxWaitSeconds）
 *   F2  步骤顺序正确（环境→级别→测试→编译→部署→截图→对比→证据→还原）
 *   F3  退出码语义正确（env=2, fail=1, ok=0）
 *   F4  -DryRun 不执行实际操作
 *   F5  dot-source 所有子模块
 *   F6  【A-3】dot-source hx-busy 并调 Wait-HxFree（自己持锁）
 *   F7  【A-3】锁在 finally 路径里释放（Release-HxLock）
 *   F8  【A-3】临界区**只**覆盖步骤 4–6（步骤 1–3 在锁前，7–9 在锁后）
 *   F9  【锁交接】设置并清理 $env:HX_LOCK_OWNER（Set-/Clear-HxLockOwnerEnv）
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
    ['[string]$Device', '[string]$Level', '[switch]$DryRun', '[switch]$Distribute', '[switch]$UpdateBaseline', '$HxWaitSeconds'].forEach((p) => {
      expect(src).toContain(p);
    });
  });

  test('F2: steps in correct order', () => {
    const order = ['Step 1', 'Step 2', 'Step 3', 'Step 4', 'Step 5', 'Step 6', 'Step 7', 'Step 8', 'Step 9'];
    const idx = order.map((s) => src.indexOf(s));
    idx.forEach((v, i) => expect(v).toBeGreaterThan(-1));
    for (let i = 1; i < idx.length; i++) {
      expect(idx[i]).toBeGreaterThan(idx[i - 1]);
    }
  });

  test('F3: exit codes correct (env=2, fail=1, ok=0)', () => {
    expect(src).toContain('exit 2');
    expect(src).toContain('exit 1');
    expect(src).toContain('exit 0');
  });

  test('F4: DryRun mode', () => {
    expect(src).toContain('DryRun');
    expect(src).toContain('不执行任何操作');
  });

  test('F5: dot-sources all sub-modules', () => {
    ['env-check.ps1', 'level-detect.ps1', 'test-compile.ps1', 'build-deploy.ps1', 'auto-screenshot.ps1', 'screenshot-diff.ps1', 'evidence-gen.ps1', 'contract-tests.ps1'].forEach((m) => {
      expect(src).toContain(m);
    });
  });

  test('F6: 【A-3】holds the HBuilderX lock itself (hx-busy + Wait-HxFree)', () => {
    expect(src).toMatch(/lib[\\/]hx-busy\.ps1/);
    expect(src).toContain('Wait-HxFree');
  });

  test('F7: 【A-3】releases the lock in a finally path', () => {
    const finallyAt = src.lastIndexOf('} finally {');
    expect(finallyAt).toBeGreaterThan(-1);
    expect(src.slice(finallyAt)).toContain('Release-HxLock');
  });

  test('F8: 【A-3】critical section covers ONLY steps 4–6', () => {
    const lockAt = src.indexOf('Wait-HxFree');
    const releaseAt = src.lastIndexOf('Release-HxLock');
    expect(lockAt).toBeGreaterThan(-1);
    expect(releaseAt).toBeGreaterThan(lockAt);

    // 步骤 1–3（不碰 HBuilderX）必须在取锁**之前**
    ['Write-Step 1 9', 'Write-Step 2 9', 'Write-Step 3 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(-1);
      expect(i).toBeLessThan(lockAt);
    });

    // 步骤 4–6（需要 HBuilderX）必须在锁**之内**
    ['Write-Step 4 9', 'Write-Step 5 9', 'Write-Step 6 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(lockAt);
      expect(i).toBeLessThan(releaseAt);
    });

    // 步骤 7–9（纯本地）必须在锁**之后**
    ['Write-Step 7 9', 'Write-Step 8 9', 'Write-Step 9 9'].forEach((s) => {
      const i = src.indexOf(s);
      expect(i).toBeGreaterThan(releaseAt);
    });
  });

  test('F9: 【锁交接】sets and clears $env:HX_LOCK_OWNER', () => {
    expect(src).toContain('Set-HxLockOwnerEnv');
    expect(src).toContain('Clear-HxLockOwnerEnv');
    // 清理必须在 finally 里（否则失败路径会留下陈旧交接）
    const finallyAt = src.lastIndexOf('} finally {');
    expect(src.slice(finallyAt)).toContain('Clear-HxLockOwnerEnv');
  });

  // F10（2026-09-14 Q-2 修正）：Q-A（quick 且未 -Compile）**不取锁**。
  // 否则别的会话持锁时，「秒级静态守护」会白等到超时 ⇒ 定位名不副实。
  test('F10: 【Q-A】lock acquisition is conditional on needing HBuilderX', () => {
    expect(src).toContain('$needsHx');
    // 取锁必须在 $needsHx 分支内
    const hxAt = src.indexOf('Wait-HxFree');
    expect(hxAt).toBeGreaterThan(-1);
    const guard = src.slice(Math.max(0, hxAt - 600), hxAt);
    expect(guard).toMatch(/if\s*\(\s*\$needsHx\s*\)/);
    // Q-A 路径必须走轻量检查（不要求设备）
    expect(src).toContain('Test-StaticEnv');
  });

  // F11（2026-09-14 Q-2 修正）：quick 的编译诊断由**显式开关**开启（Q-B），是默认行为之外的选择。
  test('F11: 【Q-B】-Compile switch exists and gates the quick compile', () => {
    expect(src).toMatch(/\[switch\]\$Compile\b/);
    expect(src).toContain('QuickCompile:$Compile');
    // $needsHx 必须把「quick + -Compile」算作需要 HBuilderX
    expect(src).toMatch(/\$needsHx\s*=\s*\(\$detectedLevel -ne 'quick'\)\s*-or\s*\$Compile/);
  });
});
