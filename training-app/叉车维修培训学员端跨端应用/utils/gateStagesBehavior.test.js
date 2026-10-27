/**
 * 门脚本阶段时间戳录制器 —— 行为守护（#2001）。
 *
 * 为什么需要行为守护：接线腿（文本断言）只能证明「脚本里写了 Start-GateStage」，
 * 证明不了录制器在 Start 自动闭前段、Stop 出 STAGE_SUMMARY、阈值判 pass/fail、
 * Stop 无开段抛错这些**分支判得对** —— 这正是本票验收标准「一次真跑后日志尾
 * 能读出 4–5 段起止与秒数、段和与门墙钟差 ≤5%」的机器判据，必须现场执行被测面。
 *
 * 被测物：scripts/lib/gate-stages.ps1（pwsh -EncodedCommand dot-source 真跑）。
 * pwsh 不可用时 fail-closed 抛错，绝不 skip（照 hxTimingBehavior.test.js 模式）。
 *
 * 断言清单（机检行 KEY=value）：
 *  B1 Initialize 后日志出现 STAGE_RECORDER 头行
 *  B2 Start 写 "[STAGE] <name> start <ts>"
 *  B3 Stop 写 "[STAGE] <name> end <ts> elapsed=<N>s"（非负整数秒）
 *  B4 Start 自动闭前段：第二段 start 前出现前一段 end
 *  B5 Stop 无开段抛错（fail-closed，PROBE_DONE 缺席）
 *  B6 正常跑：STAGE_SUMMARY stages=2 total=..s wall=..s delta=..% check=pass (threshold 5%)
 *  B7 注入早于记录 10 分钟的 WallStart ⇒ delta 超阈 ⇒ check=fail（阈值分支）
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
// 带引号的路径字面量：guardClassification 的 behavior refs 判据扫的是字符串 token，
// 只写注释会被 stripComments 去掉（勿移回注释）。
const STAGES_LIB_REL = 'scripts/lib/gate-stages.ps1';
const STAGES_LIB = path.join(ROOT, STAGES_LIB_REL);

function powershellExe() { return 'pwsh'; }
function psArgs(extra) {
  const base = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') base.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return base.concat(extra);
}

function runProbe(probeLines) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '. "' + STAGES_LIB + '"',
  ].concat(probeLines).join('\n');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, status: 0, stdout, stderr: '' };
  } catch (e) {
    return { ok: false, status: e.status, stdout: String(e.stdout || ''), stderr: String(e.stderr || e.message) };
  }
}

function pick(stdout, key) {
  const m = new RegExp('^' + key + '=(.*)$', 'm').exec(stdout);
  return m ? m[1] : null;
}

function mustRun(r, label) {
  if (!r.ok) {
    throw new Error(label + ' 探针执行失败（fail-closed 不 skip）：exit=' + r.status +
      '\nstdout=' + r.stdout + '\nstderr=' + r.stderr);
  }
}

const INIT = [
  '$tmp = if ($env:TMPDIR) { $env:TMPDIR } elseif ($env:TEMP) { $env:TEMP } else { "/tmp" }; $log = Join-Path $tmp ("gate-stages-probe-" + [guid]::NewGuid().ToString("N") + ".log")',
  'Initialize-GateStageRecorder -LogPath $log',
];
const IO = (lines) => ['$out = Get-Content -LiteralPath $log -Raw'].concat(lines);

describe('gate-stages 录制器（#2001 行为守护）', () => {
  test('B1-B4: Initialize 头行 / start 行 / 自动闭前段 / STAGE_SUMMARY', () => {
    const r = runProbe(INIT.concat([
      'Start-GateStage -Name "alpha"',
      'Start-Sleep -Milliseconds 1200',
      'Start-GateStage -Name "beta"',
      'Start-Sleep -Milliseconds 300',
      'Stop-GateStage',
    ]).concat(IO([
      'Write-Host ("HAS_INIT=" + ($out -match "STAGE_RECORDER"))',
      'Write-Host ("ALPHA_START=" + ($out -match "\\[STAGE\\] alpha start "))',
      'Write-Host ("ALPHA_END=" + ($out -match "\\[STAGE\\] alpha end .*elapsed=\\d+s"))',
      'Write-Host ("BETA_START=" + ($out -match "\\[STAGE\\] beta start "))',
      'Write-Host ("SUMMARY=" + ($out -match "STAGE_SUMMARY stages=2 total=\\d+s wall=\\d+s delta=[\\-\\d.]+% check=(pass|fail) \\(threshold 5%\\)"))',
      'Write-Host ("ELAPSED_NONNEG=" + ($out -match "elapsed=\\d+s"))',
    ])));
    mustRun(r, 'B1-B4');
    expect(pick(r.stdout, 'HAS_INIT')).toBe('True');
    expect(pick(r.stdout, 'ALPHA_START')).toBe('True');
    expect(pick(r.stdout, 'ALPHA_END')).toBe('True');
    expect(pick(r.stdout, 'BETA_START')).toBe('True');
    expect(pick(r.stdout, 'SUMMARY')).toBe('True');
    expect(pick(r.stdout, 'ELAPSED_NONNEG')).toBe('True');
  });

  test('B5: Stop 无开段抛错（fail-closed）', () => {
    const r = runProbe(INIT.concat(['Stop-GateStage', 'Write-Host "PROBE_DONE=1"']));
    expect(r.ok).toBe(false);
    expect(r.stdout).not.toMatch(/^PROBE_DONE=1$/m);
  });

  test('B6: 段和与墙钟差 ≤5% ⇒ check=pass', () => {
    const r = runProbe(INIT.concat([
      'Start-GateStage -Name "s1"',
      'Start-Sleep -Milliseconds 2100',
      'Stop-GateStage',
    ]).concat(IO([
      'Write-Host ("CHECK=" + [regex]::Match($out, "STAGE_SUMMARY .*check=(pass|fail)").Groups[1].Value)',
      'Write-Host ("HAS_TOTAL=" + ($out -match "total=\\d+s"))',
    ])));
    mustRun(r, 'B6');
    expect(pick(r.stdout, 'CHECK')).toBe('pass');
    expect(pick(r.stdout, 'HAS_TOTAL')).toBe('True');
  });

  test('B7: 注入早于记录 10 分钟的 WallStart ⇒ delta 超阈 ⇒ check=fail', () => {
    const r = runProbe([
      '$tmp = if ($env:TMPDIR) { $env:TMPDIR } elseif ($env:TEMP) { $env:TEMP } else { "/tmp" }; $log = Join-Path $tmp ("gate-stages-probe-" + [guid]::NewGuid().ToString("N") + ".log")',
      '$fake = (Get-Date).AddMinutes(-10)',
      'Initialize-GateStageRecorder -LogPath $log -WallStart $fake',
      'Start-GateStage -Name "s1"',
      'Stop-GateStage',
    ].concat(IO([
      'Write-Host ("CHECK=" + [regex]::Match($out, "STAGE_SUMMARY .*check=(pass|fail)").Groups[1].Value)',
      'Write-Host ("DELTA_FAIL=" + ($out -match "delta=[\\-\\d.]+% check=fail"))',
    ])));
    mustRun(r, 'B7');
    expect(pick(r.stdout, 'CHECK')).toBe('fail');
    expect(pick(r.stdout, 'DELTA_FAIL')).toBe('True');
  });
});
