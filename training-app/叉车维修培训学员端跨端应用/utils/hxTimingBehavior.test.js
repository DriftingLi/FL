/**
 * HBuilderX 时序判据的**行为级守护**（运行期，非源码文本断言）—— #974
 *
 * 为什么需要这个文件：
 *   `hxBusyGateContract.test.js`（H1–H11）与 `hxRunContract.test.js`（C1–C13）**全是源码文本断言** ——
 *   它们只能证明「源码里出现过某个字面量」，证明不了「那个分支真的判对了」。而 #974 的两个缺陷
 *   **恰恰都是判据本身判错**，文字断言从构造上就不可能发现：
 *
 *     现象一（`hx-busy` 陈旧锁）：锁里存着持有者 pid，但 `Test-HxLockStale` 只比 mtime。
 *       持有会话已被强杀（pid 不存在）时，只要锁没满 30 分钟就判「未陈旧」⇒ 调用方反复 `exit 2`。
 *       **当日实测白等 ~25 分钟**。
 *     现象二（`hx-run` 部署判定）：只要 launch 进程一退出就立刻收手，而资源落盘比推送晚 **7–21 秒**
 *       ⇒ 把「稍后才落盘」误判成未部署（实测 3 秒就下结论），**白重跑一整轮编译（约 7 分钟）**。
 *
 *   ⇒ 与 ADR-0011 ⑥ 的纪律一致：**「出现某字面量」≠「该分支可执行 / 判得对」**，
 *     关键判据除文本断言外**必须有运行期断言**（先例 `levelDetectBehavior.test.js` G1–G3）。
 *
 * 本文件的断言全部是**运行期**的：
 *   H-B1 死 pid + 新鲜锁 ⇒ 判陈旧（**#974 的修复点**；改前为 False）
 *   H-B2 活 pid + 新鲜锁 ⇒ **判不陈旧**（新判据绝不许放行活会话）
 *   H-B3 pid 非数字 / 超大越界（**未知**）⇒ 回落时间判据（新鲜 ⇒ 不陈旧）
 *   H-B4 持有者已死但锁已超 30 分钟 ⇒ 仍判陈旧（时间线未被破坏）
 *   D-B1 **落盘晚于首次采样**：fake 采样第 3 次才前进 ⇒ 必须 PASS（advanced），且**前两次不许收手**
 *   D-B2 会话在首次采样前就退出 + 永不前进 ⇒ **窗口内不许收手**（到 60 秒才判 exited）
 *   D-B3 永不前进 ⇒ 最终收手且**不是** advanced（调用方据此判 env）
 *   D-B4 停滞快速失败（编译段已结束）仍要在窗口之后生效
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与仓库先例 `mpWeixinGateContract.test.js:143` 一致；静默跳过等于假绿。
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const BUSY_LIB = path.join(ROOT, 'scripts', 'lib', 'hx-busy.ps1');
const DEPLOY_LIB = path.join(ROOT, 'scripts', 'lib', 'hx-deploy.ps1');

/** PowerShell 宿主：本仓 npm 脚本与门脚本都用 pwsh（Windows 本机与 CI runner 都自带）。 */
function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/**
 * dot-source 两个 lib，跑给定的探针行（PowerShell 源码），返回 { ok, stdout, stderr }。
 * 失败不吞异常（fail-closed）。
 */
function runProbe(probeLines) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    `. "${BUSY_LIB}"`,
    `. "${DEPLOY_LIB}"`,
  ].concat(probeLines).join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

/** 取 `KEY=value` 机检行 */
function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

/** 失败的统一报错文案（含 pwsh 原始输出，便于定位是环境还是判据） */
function mustRun(r) {
  if (!r.ok) {
    throw new Error(
      `探针执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
      + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
    );
  }
}

/** 模拟调用方的轮询循环（与 hx-run.ps1 主循环同形），只驱动**真实的**收手判定函数 */
const SIMULATOR = `
function Simulate-DeployTicks {
    param([int]$AdvanceAt = 0, [int]$Ticks = 20, [bool]$Exited = $false, [bool]$CompileFinished = $false)
    $flags = @()
    for ($i = 1; $i -le $Ticks; $i++) {
        $polled = $i * 10
        $deployed = ($AdvanceAt -gt 0 -and $i -ge $AdvanceAt)
        $r = Test-DeployObservationStop -Deployed $deployed -PolledSeconds $polled -Exited $Exited -CompileFinished $CompileFinished -DeployDeadlineSeconds 60 -StallSeconds 300 -TimeoutSeconds 900
        $flags += "$($r.Stop)"
        if ($r.Stop) { return "$polled|$($r.Outcome)|$($flags -join ',')" }
    }
    return "none|none|$($flags -join ',')"
}
`;

describe('HBuilderX 时序判据行为级守护（#974，运行期）', () => {
  describe('hx-busy 陈旧锁：持有进程存活（现象一）', () => {
    const r = runProbe([
      '$fresh = (Get-Date).AddMinutes(-1)',
      '$old = (Get-Date).AddMinutes(-45)',
      'Write-Output ("STALE_DEAD_FRESH=" + (Test-HxLockStale -Info @{ Pid = "999999"; Started = "x"; MTime = $fresh }))',
      'Write-Output ("STALE_LIVE_FRESH=" + (Test-HxLockStale -Info @{ Pid = "$PID"; Started = "x"; MTime = $fresh }))',
      'Write-Output ("STALE_NONNUM_FRESH=" + (Test-HxLockStale -Info @{ Pid = "n/a"; Started = "x"; MTime = $fresh }))',
      'Write-Output ("STALE_EMPTY_FRESH=" + (Test-HxLockStale -Info @{ Pid = ""; Started = "x"; MTime = $fresh }))',
      'Write-Output ("STALE_HUGE_FRESH=" + (Test-HxLockStale -Info @{ Pid = "99999999999999999999"; Started = "x"; MTime = $fresh }))',
      'Write-Output ("STALE_DEAD_OLD=" + (Test-HxLockStale -Info @{ Pid = "999999"; Started = "x"; MTime = $old }))',
      'Write-Output ("STALE_LIVE_OLD=" + (Test-HxLockStale -Info @{ Pid = "$PID"; Started = "x"; MTime = $old }))',
    ]);
    mustRun(r);

    test('B1: 持有 pid 不存在 + 锁未满 30 分钟 ⇒ 判陈旧（#974 修复点）', () => {
      expect(pick(r.stdout, 'STALE_DEAD_FRESH')).toBe('True');
    });

    test('B2: 持有 pid 仍活 + 锁未满 30 分钟 ⇒ 判不陈旧（绝不放行活会话）', () => {
      expect(pick(r.stdout, 'STALE_LIVE_FRESH')).toBe('False');
    });

    test('B3: pid 非数字 / 空 / 超大越界（未知）⇒ 回落时间判据，不误判为陈旧', () => {
      ['STALE_NONNUM_FRESH', 'STALE_EMPTY_FRESH', 'STALE_HUGE_FRESH'].forEach((k) => {
        expect({ k, v: pick(r.stdout, k) }).toEqual({ k, v: 'False' });
      });
    });

    test('B4: 时间线未被破坏（死/活持有者 + 超 30 分钟 ⇒ 都判陈旧）', () => {
      expect(pick(r.stdout, 'STALE_DEAD_OLD')).toBe('True');
      expect(pick(r.stdout, 'STALE_LIVE_OLD')).toBe('True');
    });
  });

  describe('hx-run 部署观察窗：落盘晚于首次采样（现象二）', () => {
    const r = runProbe([
      SIMULATOR,
      // 第 3 次采样（=30 秒）才前进；会话早在首次采样前就退出了（实测形态）
      'Write-Output ("SIM_LATE_LANDING=" + (Simulate-DeployTicks -AdvanceAt 3 -Ticks 20 -Exited $true))',
      // 会话提前退出且永不前进 ⇒ 必须等满最短观察窗（60 秒）才收手
      'Write-Output ("SIM_EARLY_EXIT=" + (Simulate-DeployTicks -AdvanceAt 0 -Ticks 20 -Exited $true))',
      // 会话没退出、编译段也没结束、永不前进 ⇒ 到顶（900 秒）才收手
      'Write-Output ("SIM_NEVER_ADVANCE=" + (Simulate-DeployTicks -AdvanceAt 0 -Ticks 200 -Exited $false))',
      // 编译段已结束 ⇒ 停滞快速失败，但仍不得早于窗口
      'Write-Output ("SIM_STALL=" + (Simulate-DeployTicks -AdvanceAt 0 -Ticks 200 -Exited $false -CompileFinished $true))',
    ]);
    mustRun(r);

    test('B1: fake 采样第 3 次才前进 ⇒ PASS（advanced），且**前两次不许收手**', () => {
      const parts = (pick(r.stdout, 'SIM_LATE_LANDING') || '').split('|');
      expect(parts[0]).toBe('30');          // 在前进的那一次收手
      expect(parts[1]).toBe('advanced');    // 结论是「已部署」
      expect(parts[2]).toBe('False,False,True'); // 前两次确实没收手（防「首采样就下结论」回归）
    });

    test('B2: 会话提前退出 + 永不前进 ⇒ 窗口内不许收手，到 60 秒才判 exited', () => {
      const parts = (pick(r.stdout, 'SIM_EARLY_EXIT') || '').split('|');
      expect(parts[0]).toBe('60');          // **不是 10** —— 这正是 #974 的回归点
      expect(parts[1]).toBe('exited');
    });

    test('B3: 永不前进 ⇒ 最终收手，且结论不是 advanced（调用方据此判 env）', () => {
      const parts = (pick(r.stdout, 'SIM_NEVER_ADVANCE') || '').split('|');
      expect(parts[0]).toBe('900');
      expect(parts[1]).toBe('timeout');
      expect(parts[1]).not.toBe('advanced');
    });

    test('B4: 停滞快速失败仍生效，但不得早于窗口（300 秒 > 60 秒）', () => {
      const parts = (pick(r.stdout, 'SIM_STALL') || '').split('|');
      expect(parts[0]).toBe('300');
      expect(parts[1]).toBe('stalled');
      // 窗口内（10/20/…/50 秒）必须全是 False
      const flags = parts[2].split(',');
      expect(flags.slice(0, 5)).toEqual(['False', 'False', 'False', 'False', 'False']);
    });
  });
});
