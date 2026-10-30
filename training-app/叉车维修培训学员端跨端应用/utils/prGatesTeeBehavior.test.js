/**
 * pr-gates 的输出转发器（`scripts/lib/gate-tee.ps1`）**行为守护**（#2147）。
 *
 * 为什么必须真执行：#2147 的病根是**真句柄继承** —— 子门进程退出后，它拉起的微信开发者工具（常驻 GUI）
 * 仍握着重定向管道的写句柄，任何「等 EOF 的读法」都会挂死。把进程 mock 掉就什么也没测到
 * （#2131 的 F4 守护同一理据：mock 掉句柄继承 = 测了个空）。
 *
 * 成对断言（docs/agents/guards.md 判据①：改坏它，会不会红）：
 *   T1 必不红：正常命令 ⇒ 行**完整且按序**转发、日志拿到同一份行、DrainAbandoned=false。
 *   T2 必红侧（判别力）：主进程退出但句柄被孙进程继承 ⇒ **必须有界返回**（holder 活 15 s；
 *      断言 < 10 s 且 DrainAbandoned=true），且退出前打的那行照样转发到（不是整段丢输出）。
 *      把有界收尾摘掉 ⇒ 这一条挂到 holder 自然退出 ⇒ 翻红。
 *
 * 子进程一律 = **本进程的 node**，且待执行的脚本先落盘再由路径传入（不拼 `-e` 代码：
 * 引号要穿过 JS 模板 → pwsh 单引号 → node 源码三层，转义层一错就变成「测了个别的命令」）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）；不可用时 fail-closed 抛错，不 skip（仓库先例同 prGatesBehavior）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/gate-tee.ps1';

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass');
  return args.concat(extra);
}

/** 正常子进程：打两行就退。 */
function writeNormalChild(dir) {
  const file = path.join(dir, 'normal.js');
  fs.writeFileSync(file, "process.stdout.write('line-a\\nline-b\\n');\n", 'utf8');
  return file;
}

/** 病根复现件：主进程打一行就退，但**孙进程（detached + stdio inherit）继续握着 stdout 句柄 15 s**。 */
function writeHolderChild(dir) {
  const file = path.join(dir, 'holder.js');
  fs.writeFileSync(file, [
    "const { spawn } = require('child_process');",
    "spawn(process.execPath, ['-e', 'setTimeout(()=>{}, 15000)'], { stdio: 'inherit', detached: true }).unref();",
    "process.stdout.write('parent-done\\n');",
    'process.exit(0);',
    ''
  ].join('\n'), 'utf8');
  return file;
}

/**
 * 造 harness：dot-source 转发器 → 跑两个用例 → 回传机读 JSON。
 * 两个子脚本先落盘（见文件头「不拼 -e 代码」），harness 只按路径传。
 */
function buildHarness(dir) {
  const lib = path.join(ROOT, LIB_REL.split('/').join(path.sep)).replace(/'/g, "''");
  const script = [
    "$ErrorActionPreference = 'Stop'",
    `. '${lib}'`,
    '$node = $args[0]',
    '$normal = Join-Path $PSScriptRoot "normal.js"',
    '$holder = Join-Path $PSScriptRoot "holder.js"',
    "$log = Join-Path $PSScriptRoot 'tee-normal.log'",
    // T1：正常命令
    '$t0 = [System.Diagnostics.Stopwatch]::StartNew()',
    '$r1 = Invoke-GateTee -FilePath $node -Arguments @($normal) -LogPath $log -DrainMs 5000',
    '$ms1 = $t0.ElapsedMilliseconds',
    // T2：主进程退出后，孙进程继续持有 stdout 句柄
    '$t1 = [System.Diagnostics.Stopwatch]::StartNew()',
    '$r2 = Invoke-GateTee -FilePath $node -Arguments @($holder) -DrainMs 5000',
    '$ms2 = $t1.ElapsedMilliseconds',
    '$out = [ordered]@{',
    '  normalLines = @($r1.Lines)',
    '  normalExit = $r1.ExitCode',
    '  normalAbandoned = [bool]$r1.DrainAbandoned',
    '  normalMs = $ms1',
    '  logText = (Get-Content -LiteralPath $log -Raw)',
    '  holderLines = @($r2.Lines)',
    '  holderExit = $r2.ExitCode',
    '  holderAbandoned = [bool]$r2.DrainAbandoned',
    '  holderMs = $ms2',
    '}',
    "Write-Output ('TEERESULT ' + ($out | ConvertTo-Json -Compress -Depth 5))"
  ].join('\n');
  const file = path.join(dir, 'tee-harness.ps1');
  fs.writeFileSync(file, script, 'utf8');
  return file;
}

function runHarness(file) {
  let raw = '';
  try {
    raw = execFileSync('pwsh', psArgs(['-File', file, process.execPath]), {
      encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 8 * 1024 * 1024
    });
  } catch (e) {
    if (e.code === 'ENOENT') throw new Error('本套件需要 pwsh（PowerShell 7）：找不到可执行文件（fail-closed，不 skip）');
    throw new Error(`tee harness 执行失败：exit=${e.status}\nstdout=${e.stdout}\nstderr=${e.stderr}`);
  }
  const line = String(raw).split(/\r?\n/).find((l) => l.startsWith('TEERESULT '));
  if (!line) throw new Error(`tee harness 没有输出结果行：\n${raw}`);
  return JSON.parse(line.slice('TEERESULT '.length));
}

describe('pr-gates 输出转发器（#2147：收流有界 —— 主进程退出后不再被句柄挂死）', () => {
  jest.setTimeout(180000);
  let dir;
  let result;

  beforeAll(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gate-tee-'));
    writeNormalChild(dir);
    writeHolderChild(dir);
    result = runHarness(buildHarness(dir));
  });

  afterAll(() => {
    try { fs.rmSync(dir, { recursive: true, force: true }); } catch { /* 清不掉不判红 */ }
  });

  it('T1 正常命令：行完整按序转发、日志同份、不触发兜底', () => {
    expect(result.normalLines).toEqual(['line-a', 'line-b']);
    expect(result.normalExit).toBe(0);
    expect(result.normalAbandoned).toBe(false);
    // 实时转发之外，日志也必须拿到同一份行（两门结果行的「逐字节不变」靠它落盘）
    expect(String(result.logText)).toContain('line-a');
    expect(String(result.logText)).toContain('line-b');
  });

  it('T2 句柄被孙进程继承：命令仍然返回（<10 s），退出前打的行照样转发到', () => {
    // 主进程已退出 ⇒ 退出码是真值，不被「仍在跑」污染
    expect(result.holderExit).toBe(0);
    // 承重条款：holder 活 15 s；一旦回到那个量级，就说明又变成「等 EOF」了
    expect(result.holderMs).toBeLessThan(10000);
    expect(result.holderAbandoned).toBe(true);
    // 放弃的是「剩余」输出，不是已产生的行（否则日志里会缺结果行 ⇒ 假红）
    expect(result.holderLines).toEqual(['parent-done']);
  });
});
