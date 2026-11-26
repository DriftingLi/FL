/**
 * HBuilderX 占用现场快照（`Get-HxBusyStatus` / `HX_STATUS`）的**行为级守护** —— 2026-11-26，用户 M2 ①②
 *
 * 为什么需要这个文件：
 *   `hxBusyGateContract.test.js` 的 H13/H14 是**源码文本断言** —— 只能证明「源码里有 `Get-HxBusyStatus`」，
 *   证明不了「它把五种锁形态判对了」。而这次要修的症状恰恰是**判据/反馈本身不成立**：
 *   改前进入等待只会每 10 秒刷一行「已等 N 秒」，超时后才给一行 `HX_BUSY … result=timeout`，
 *   **全程没有任何一行说明「谁占着、占多久、是不是陈旧锁、该等还是该退」** —— 用户实际体验是
 *   「白等 120 秒，然后像偶发失败一样退出」。文本断言对这类缺陷从构造上就是盲的。
 *
 * 运行期断言（全部驱动**真实**函数，不复制算法）：
 *   S-B1 无锁 ⇒ `lock=free holder=none`，且**查询本身不创建锁文件**（只读性）
 *   S-B2 活会话持锁 ⇒ `lock=held` + `alive=true stale=false`
 *   S-B3 持有者已死（#974 快路）⇒ `alive=false stale=true`
 *   S-B4 pid 不可解析（未知）⇒ `alive=unknown`，且**绝不渲染成 false**
 *   S-B5 活会话 + 锁超 30 分钟 ⇒ `stale=true`，`age` 是真实秒数（时间线仍生效）
 *   S-B6 残缺锁文件（<2 行）⇒ 判 free 且**随后能取到锁**（不是「卡住」的假象）
 *   S-B7 等待超时（`Wait-HxFree -TimeoutSeconds 2 -PollSeconds 1`）⇒ exit 2，
 *        终端含 `HX_STATUS lock=held`、持有者 pid、`HX_BUSY wait=… result=timeout`，且**真的等满了时间**
 *   S-B8 `-NoWait` ⇒ 立刻 exit 2（同一行仍在），证明「要退」的出路不是空话
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与 `hxTimingBehavior.test.js` / `mpWeixinGateContract.test.js:143` 同一纪律；静默跳过等于假绿。
 *
 * 锁目录隔离：每个用例把 `$env:TEMP` 指向 `mkdtemp` 出来的独立目录**再** dot-source helper
 * （`$script:HxLockDir` 在 dot-source 时从 `$env:TEMP` 落定），因此不碰维护者/其它 agent 的真实锁。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
const { encodeCommand } = require('./probeSpawn.js');

const ROOT = path.join(__dirname, '..');
const BUSY_LIB = path.join(ROOT, 'scripts', 'lib', 'hx-busy.ps1');
const LOCK_NAME = 'hx-agent.lock';

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

/** 锁目录隔离：helper 在 dot-source 时从 `$env:TEMP` 落定锁路径。 */
function mkLockDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'hxbusy-status-'));
}

function cleanup(dir) {
  try {
    fs.rmSync(dir, { recursive: true, force: true });
  } catch (e) {
    // 清理失败不影响判据结论（下一轮 mkdtemp 是新目录）
  }
}

/** 每个探针都用的前置：隔离 TEMP → dot-source helper → 造锁/改 mtime 的小工具。 */
function prelude(dir) {
  if (dir.includes("'")) throw new Error(`临时目录名含单引号，无法安全内联：${dir}`);
  return [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    `$env:TEMP = '${dir}'`,
    `$env:TMP = '${dir}'`,
    `. "${BUSY_LIB}"`,
    'function Write-FakeLock {',
    '    param([string]$PidText, [int]$AgeMinutes = 0, [string[]]$Lines = $null)',
    `    $p = Join-Path $env:TEMP '${LOCK_NAME}'`,
    '    if ($null -ne $Lines) { Set-Content -LiteralPath $p -Encoding utf8 -Value $Lines }',
    '    else { Set-Content -LiteralPath $p -Encoding utf8 -Value @($PidText, (Get-Date -Format "yyyy-MM-dd HH:mm:ss")) }',
    '    if ($AgeMinutes -gt 0) { (Get-Item -LiteralPath $p).LastWriteTime = (Get-Date).AddMinutes(-$AgeMinutes) }',
    '}',
  ].join('\n');
}

/** 跑探针，返回 { ok, status, stdout, stderr, ms }；**不吞异常**（fail-closed 由 mustRun/断言负责）。 */
function runProbe(dir, lines) {
  const script = [prelude(dir)].concat(lines).join('\n');
  const t0 = Date.now();
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encodeCommand(script)]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, status: 0, stdout: String(stdout), stderr: '', ms: Date.now() - t0 };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
      ms: Date.now() - t0,
    };
  }
}

/** 取 `KEY=value` 机检行 */
function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

function mustRun(r) {
  if (!r.ok) {
    throw new Error(
      `探针执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
      + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
    );
  }
}

function statusLine(r) {
  mustRun(r);
  const line = pick(r.stdout, 'LINE');
  if (!line) throw new Error(`快照行缺失：\n${r.stdout}`);
  return line;
}

describe('HBuilderX 占用现场快照（HX_STATUS）行为级守护 —— 2026-11-26', () => {
  it('S-B1: 无锁 ⇒ lock=free，且查询本身不创建锁文件（只读性）', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        '$st = Get-HxBusyStatus',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status $st))',
        'Write-Output ("PRESENT=" + $st.LockPresent)',
        'Write-Output ("LEFTOVER=" + (@(Get-ChildItem -Force $env:TEMP | Select-Object -ExpandProperty Name) -join ","))',
      ]);
      const line = statusLine(r);
      expect(line).toContain('lock=free');
      expect(line).toContain('holder=none');
      expect(pick(r.stdout, 'PRESENT')).toBe('False');
      // 只读性：一次「查一下」不许留下任何文件（否则「先查」就变成「顺手抢一把」）
      expect(pick(r.stdout, 'LEFTOVER')).toBe('');
    } finally {
      cleanup(dir);
    }
  });

  it('S-B2: 活会话持锁 ⇒ lock=held + alive=true + stale=false', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "$PID"',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status (Get-HxBusyStatus)))',
      ]);
      const line = statusLine(r);
      expect(line).toContain('lock=held');
      expect(line).toMatch(/holder=\d+/);
      expect(line).toContain('alive=true');
      expect(line).toContain('stale=false');
    } finally {
      cleanup(dir);
    }
  });

  it('S-B3: 持有者已死 ⇒ alive=false + stale=true（#974 快路仍生效）', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "999999"',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status (Get-HxBusyStatus)))',
      ]);
      const line = statusLine(r);
      expect(line).toContain('lock=held');
      expect(line).toContain('alive=false');
      expect(line).toContain('stale=true');
    } finally {
      cleanup(dir);
    }
  });

  it('S-B4: pid 不可解析 ⇒ alive=unknown，绝不渲染成 false', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "not-a-pid"',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status (Get-HxBusyStatus)))',
      ]);
      const line = statusLine(r);
      expect(line).toContain('alive=unknown');
      // 「不知道」被写成 alive=false 会让人以为锁可以抢 —— 这正是本次要钉住的渲染口径
      expect(line).not.toContain('alive=false');
    } finally {
      cleanup(dir);
    }
  });

  it('S-B5: 活会话 + 锁超 30 分钟 ⇒ stale=true，age 为真实秒数（时间线仍生效）', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "$PID" -AgeMinutes 45',
        '$st = Get-HxBusyStatus',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status $st))',
        'Write-Output ("AGE=" + $st.AgeSeconds)',
      ]);
      const line = statusLine(r);
      expect(line).toContain('alive=true');
      expect(line).toContain('stale=true');
      const age = Number(pick(r.stdout, 'AGE'));
      expect(age).toBeGreaterThan(40 * 60);
    } finally {
      cleanup(dir);
    }
  });

  it('S-B6: 残缺锁文件（<2 行）⇒ 判 free 且随后能取到锁（不是卡住的假象）', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -Lines @("garbage-only-one-line")',
        'Write-Output ("LINE=" + (Format-HxBusyStatusLine -Status (Get-HxBusyStatus)))',
        // 与 Acquire-HxLock 的写入语义一致：残缺锁会被下一次取锁覆盖，故不该出现「文件在、却永远抢不到」
        'Write-Output ("ACQUIRE=" + (Acquire-HxLock))',
      ]);
      const line = statusLine(r);
      expect(line).toContain('lock=free');
      expect(pick(r.stdout, 'ACQUIRE')).toBe('True');
    } finally {
      cleanup(dir);
    }
  });

  it('S-B7: 等待超时 ⇒ exit 2，且终端带持有者信息与 HX_BUSY timeout 行，并真等满上限', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "$PID"',
        '$hx = Wait-HxFree -CliExe "C:\\nonexistent\\cli.exe" -TimeoutSeconds 2 -PollSeconds 1',
        // 跑到这里说明没有超时（超时会在 Wait-HxFree 里 exit 2）
        'Write-Output ("UNEXPECTED=" + $hx.Result)',
      ]);
      expect({ exit: r.status, unexpected: pick(r.stdout, 'UNEXPECTED') }).toEqual({ exit: 2, unexpected: null });
      expect(r.stdout).toContain('HX_STATUS lock=held');
      expect(r.stdout).toMatch(/持有者 pid=\d+/);
      expect(r.stdout).toContain('result=timeout');
      // 口径（用户 M2 ⑤）：串行是本仓编排选择，不得被读成 HBuilderX 的平台上限
      expect(r.stdout).toContain('不是 HBuilderX 的平台上限');
      // 真的等过了上限（TimeoutSeconds=2、PollSeconds=1 ⇒ 约 2 秒），不是「看一眼就退」
      expect(r.ms).toBeGreaterThanOrEqual(1800);
    } finally {
      cleanup(dir);
    }
  });

  it('S-B8: -NoWait ⇒ 立刻 exit 2（「要立刻退」的出路不是空话）', () => {
    const dir = mkLockDir();
    try {
      const r = runProbe(dir, [
        'Write-FakeLock -PidText "$PID"',
        '$hx = Wait-HxFree -CliExe "C:\\nonexistent\\cli.exe" -TimeoutSeconds 600 -NoWait',
        'Write-Output ("UNEXPECTED=" + $hx.Result)',
      ]);
      expect({ exit: r.status, unexpected: pick(r.stdout, 'UNEXPECTED') }).toEqual({ exit: 2, unexpected: null });
      expect(r.stdout).toContain('HX_STATUS lock=held');
      expect(r.stdout).toContain('HX_BUSY wait=0 result=timeout');
      expect(r.ms).toBeLessThan(5000);
    } finally {
      cleanup(dir);
    }
  });

  it('S-B9: 注册表安装信息探测在 StrictMode 下不抛错（缺 DisplayName 的条目不许把 CLI 解析整条打断）', () => {
    const dir = mkLockDir();
    try {
      // 2026-11-26 18:13 真跑 `hx:compile-only` 死在
      // 「在此对象上找不到属性"DisplayName"」——卸载项里确有条目没有该属性，
      // StrictMode 下直接成员访问会抛错并让整条 CLI 解析链断掉。
      const r = runProbe(dir, [
        '$cands = Get-HxCliCandidatesFromRegistry',
        'if ($null -eq $cands) { Write-Output "CANDS=null" } else { Write-Output ("CANDS=" + (@($cands).Count)) }',
        // 本机装没装 HBuilderX 与初装位置无关：这里只断言「能枚举完、不抛错、返回集合」
        'Write-Output ("TYPE=" + ($cands.GetType().Name))',
      ]);
      mustRun(r);
      const count = pick(r.stdout, 'CANDS');
      expect(count).not.toBeNull();
      expect(count).not.toBe('null');
      expect(Number(count)).toBeGreaterThanOrEqual(0);
      expect(r.stderr).not.toMatch(/DisplayName|找不到属性/);
    } finally {
      cleanup(dir);
    }
  });
});
