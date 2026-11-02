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
 *   T3 必红侧（编码，2026-11-02 补）：父进程已把 `[Console]::OutputEncoding` 顶成 UTF-8
 *      （= pr-gates.ps1:104 的现场），子进程是无窗口的**真 pwsh** 打中文 ⇒ 解码若跟着父进程走，
 *      落盘日志整句变 U+FFFD（现场实测 6899 个，预检判红）。把 gate-tee 的解码改回
 *      `[Console]::OutputEncoding` ⇒ 这一条在 Windows 上翻红；非 Windows 上子进程本就吐 UTF-8，
 *      该条不具判别力（缺陷只在 Windows 存在），见 `docs/agents/guards.md` 判据①。
 *   T4 必红侧（混合编码，2026-11-02 补）：真链路 pr-gates.log 里**两种编码并存** —— 子门 pwsh 自己按系统
 *      码页吐、它拉起的原生程序（HBuilderX cli.exe / java / node）按 UTF-8 写。单编码解码解不了一边
 *      （钉 UTF-8 ⇒ fffd=6899；钉 cp936 ⇒ moji=47），两条都是预检/④c 自检判红。把解码改回任一种单编码
 *      ⇒ 这一条在 Windows 上翻红。
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
 * 中文子进程：**真 pwsh**（与 pr-gates 的子门同形），打一行含 U+4E2D 的字就退。
 *
 * 字形一律用码位（`[char]0x4E2D`）写，不落非 ASCII 字面量：本仓实测这条写入链会把
 * U+FFFD 之类的字面量整字吞掉（`evidenceCaptureEncodingBehavior.test.js` 头注同一条纪律）。
 */
function writeCjkChild(dir) {
  const file = path.join(dir, 'cjk-child.ps1');
  fs.writeFileSync(file, "Write-Output ([string][char]0x4E2D + ' cjk-ok')\\n", 'utf8');
  return file;
}

/**
 * 混合编码子进程：真链路 `pr-gates.log` 的形态 —— 同一条流里，
 * 第一行是子门 pwsh 自己打的（系统码页 cp936），第二行是它拉起的原生程序（node）打的（UTF-8）。
 * 单编码解码必然解错一边：钉 UTF-8 ⇒ 第一行变替换符；钉 cp936 ⇒ 第二行变 GBK 乱码（moji=47）。
 */
function writeMixedChild(dir) {
  const file = path.join(dir, 'mixed-child.ps1');
  fs.writeFileSync(file, [
    "Write-Output ([string][char]0x4E2D + ' cp936-ok')",
    '& $env:TEE_NODE -e "process.stdout.write(\'\\u4e2d utf8-ok\\n\')"',
    ''
  ].join('\n'), 'utf8');
  return file;
}

/**
 * 造 harness：dot-source 转发器 → 跑三个用例 → 回传机读 JSON。
 * 三个子脚本先落盘（见文件头「不拼 -e 代码」），harness 只按路径传。
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
    // T3：#2147 的编码回归锁 —— 真 pwsh 子进程（与 pr-gates 的子门同形）打中文，且父进程先被顶成
    // UTF-8（= pr-gates.ps1:104 的现场）。子进程无窗口 ⇒ 按系统码页吐字节；解码若取父进程的
    // [Console]::OutputEncoding，这一行就整句变成替换字符。
    '$pwsh = (Get-Process -Id $PID).Path',
    '$cjk = Join-Path $PSScriptRoot "cjk-child.ps1"',
    "$log3 = Join-Path $PSScriptRoot 'tee-cjk.log'",
    'try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }',
    '$r3 = Invoke-GateTee -FilePath $pwsh -Arguments @("-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", $cjk) -LogPath $log3 -DrainMs 5000',
    // T4：混合编码 —— 同一条流里 pwsh（系统码页）与它对原生程序的 UTF-8 输出并存，= 真链路 pr-gates.log 的形态
    '$mixed = Join-Path $PSScriptRoot "mixed-child.ps1"',
    "$log4 = Join-Path $PSScriptRoot 'tee-mixed.log'",
    '$env:TEE_NODE = $node',
    '$r4 = Invoke-GateTee -FilePath $pwsh -Arguments @("-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", $mixed) -LogPath $log4 -DrainMs 5000',
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
    '  cjkLines = @($r3.Lines)',
    '  cjkExit = $r3.ExitCode',
    '  cjkLog = $log3',
    '  mixedLines = @($r4.Lines)',
    '  mixedExit = $r4.ExitCode',
    '  mixedLog = $log4',
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
    writeCjkChild(dir);
    writeMixedChild(dir);
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

  it('T3 中文子进程 + 父进程已顶 UTF-8：落盘日志零 U+FFFD（#2147 的 fffd=6899 回归锁）', () => {
    const log = fs.readFileSync(result.cjkLog, 'utf8');
    // 承重条款：子进程按系统码页吐字节，解码若跟着父进程的 UTF-8 走，这里的每个中文字符都成替换符
    expect(log).not.toContain(String.fromCharCode(0xfffd));
    expect(log).toContain(String.fromCharCode(0x4e2d));
    expect(result.cjkExit).toBe(0);
  });

  it('T4 同一条流里码页段与 UTF-8 段并存：两种中文都不变形（真链路 moji=47 回归锁）', () => {
    const log = fs.readFileSync(result.mixedLog, 'utf8');
    // 承重条款：单编码解码解不了一边 —— 钉 cp936 ⇒ UTF-8 段变 GBK 乱码（现场 moji=47）；钉 UTF-8 ⇒ 码页段变替换符
    expect(log).not.toContain(String.fromCharCode(0xfffd));
    expect(log).toContain(`${String.fromCharCode(0x4e2d)} cp936-ok`);
    expect(log).toContain(`${String.fromCharCode(0x4e2d)} utf8-ok`);
    expect(result.mixedExit).toBe(0);
  });
});
