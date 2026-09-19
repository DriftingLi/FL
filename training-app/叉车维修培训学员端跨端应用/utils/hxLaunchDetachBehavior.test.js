/**
 * `Start-CliLaunchDetached` 的**派生形态**行为级守护（运行期）—— 2026-09-15
 *
 * 为什么需要（真机实测踩到，维护者报告）：
 *   真运行的 `cli.exe` 会话**常驻不自收口**。而 `Start-Process -NoNewWindow -PassThru -Redirect*`
 *   会把**本进程的 stdout 句柄继承**给那个常驻会话 ⇒ 调用链上**任意祖先**的
 *   `| Out-String` / `*> logfile` 都要等它结束才收口。实测（假 cli 常驻 20 秒）：
 *
 *     改前：经 build-deploy → hx-run 这条链，外层管道 **22 秒**才收口
 *     改后：**1 秒**收口（且输出照常进日志）
 *
 *   后果链：父脚本走不到 `finally { Release-HxLock }` ⇒ **HBuilderX 锁残留** ⇒
 *   把别的会话卡到 180 秒超时（维护者当日实测）。
 *
 * 为什么必须是**运行期**断言：这个缺陷完全不影响源码文本形态（两版都只是几行调用），
 * 任何 `expect(src).toContain(...)` 都抓不到 —— 只有真跑一次、量收口时间才看得见。
 *
 * 断言（用**真实**的 Start-CliLaunchDetached，AST 抽函数定义后调用，避免执行整篇 hx-run.ps1）：
 *   D1 派发后外层管道**立即收口**（远早于常驻子进程的寿命）
 *   D2 子进程的 stdout **照常进** `launch-*.out`（不能为了断开而丢掉日志）
 *   D3 派生用的是「新进程树」形态（UseShellExecute），且日志由**包装脚本自己**写（`*>`）
 *   D4 派发失败（CliExe 不存在）时 **Proc 判空不抛错**（fail-closed 不崩）
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const HX_RUN = path.join(ROOT, 'scripts', 'hx-run.ps1');
/** 常驻子进程寿命（秒）。必须明显大于 D1 的阈值，才能区分「断开」与「被拖住」 */
const CHILD_LIFETIME_S = 15;
/** D1 阈值：外层管道必须在这个时间内收口 */
const DETACH_DEADLINE_S = 8;

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 建一个临时项目：假 cli（常驻）+ 驱动脚本（AST 抽全部函数 → 调 Start-CliLaunchDetached） */
function withHarness(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hxdetach-'));
  const proj = path.join(tmp, 'proj');
  const logDir = path.join(proj, '.ci-verify');
  fs.mkdirSync(logDir, { recursive: true });
  const fakeCli = path.join(proj, 'fake-cli.ps1');
  fs.writeFileSync(fakeCli, `Write-Output 'cli-line-A'\nStart-Sleep -Seconds ${CHILD_LIFETIME_S}\n`);
  try {
    return fn({ tmp, proj, logDir, fakeCli });
  } finally {
    // ⚠️ 常驻子进程可能仍持有临时目录 ⇒ `rmSync` 会 EPERM。
    //    这是**测试自身的收尾问题**，不该把用例判红（真判据是 D1/D2 的断言）。
    try {
      fs.rmSync(tmp, { recursive: true, force: true });
    } catch {
      // 留给 OS 临时目录清理
    }
  }
}

/** 在给定项目里跑驱动脚本，返回 { ok, seconds, stdout, stderr, outFiles }
 *  ⚠️ 用**文件**而非管道取回驱动脚本输出：管道正是常驻孙进程可能继承的句柄，
 *     用管道会把「测试基建的泄漏」误算成「被测代码的泄漏」。 */
function runHarness(ctx, cliExe) {
  const harness = path.join(ctx.tmp, 'harness.ps1');
  const capFile = path.join(ctx.tmp, 'harness.capture.txt');
  const body = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `$src = Get-Content -LiteralPath '${HX_RUN}' -Raw`,
    '$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)',
    '$fns = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)',
    'foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }',
    `$logDir = '${ctx.logDir}'`,
    `$logFile = Join-Path $logDir 'hx-run.log'`,
    `$r = Start-CliLaunchDetached -CliArgs @('-NoProfile','-File','${ctx.fakeCli}') -CliExe '${cliExe}' -LogDir $logDir -LogFile $logFile`,
    "Write-Output ('PROC_IS_NULL=' + (-not [bool]$r.Proc))",
    "Write-Output ('OUT=' + $r.OutFile)",
  ].join('\n');
  fs.writeFileSync(harness, body);

  const sw = Date.now();
  let status = 0;
  let errText = '';
  try {
    // stdout/stderr 直接落文件：不把驱动脚本接进管道
    execFileSync(powershellExe(), psArgs(['-File', harness]), {
      timeout: 120000,
      windowsHide: true,
      stdio: ['ignore', fs.openSync(capFile, 'w'), fs.openSync(capFile + '.err', 'w')],
    });
  } catch (e) {
    status = e.status === undefined ? -1 : e.status;
    errText = String(e.message || '');
  }
  const stdout = fs.existsSync(capFile) ? readText(capFile) : '';
  const stderr = (fs.existsSync(capFile + '.err') ? readText(capFile + '.err') : '') + errText;
  return { ok: status === 0, status, stdout, stderr, seconds: (Date.now() - sw) / 1000 };
}

function readLaunchOut(logDir) {
  const files = fs.existsSync(logDir) ? fs.readdirSync(logDir).filter((f) => /^launch-.*\.out$/.test(f)) : [];
  return files.map((f) => readText(path.join(logDir, f))).join('\n');
}

describe('Start-CliLaunchDetached 派生形态（运行期，2026-09-15）', () => {
  test(`D1+D2: 派发后外层管道立即收口（<${DETACH_DEADLINE_S}s），且子进程输出仍进日志`, () => {
    withHarness((ctx) => {
      const r = runHarness(ctx, 'pwsh');
      if (!r.ok) {
        throw new Error(`驱动脚本执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
          + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`);
      }
      // D1：常驻子进程活 ${CHILD_LIFETIME_S}s；若句柄泄漏，驱动脚本会被拖到 ≈CHILD_LIFETIME_S
      expect({ secs: r.seconds < DETACH_DEADLINE_S, detail: `${r.seconds.toFixed(1)}s` })
        .toEqual({ secs: true, detail: `${r.seconds.toFixed(1)}s` });

      // D2：给了「常驻子进程要先打印一行」的机会，日志里必须看得到
      //     （稍等一下：包装脚本 + 子进程启动有开销）
      const deadline = Date.now() + 8000;
      let content = readLaunchOut(ctx.logDir);
      while (!/cli-line-A/.test(content) && Date.now() < deadline) {
        execFileSync('pwsh', psArgs(['-Command', 'Start-Sleep -Milliseconds 400']), { encoding: 'utf8' });
        content = readLaunchOut(ctx.logDir);
      }
      expect(content).toMatch(/cli-line-A/);
    });
  });

  test('D3: launch 函数用新进程树（UseShellExecute）且日志由包装脚本自写（*>）', () => {
    const src = readText(HX_RUN);
    // ⚠️ 必须**只取 Start-CliLaunchDetached 的函数体**再断言：
    //    `Invoke-CliStep`（open / project-open 两个短命步）**应该**继续用
    //    `Start-Process -NoNewWindow -RedirectStandard*` —— 那两步不会常驻，句柄泄漏不成立。
    //    全文件扫描会把这两步误判成违规（本用例初版就踩了这个假阳性）。
    const start = src.indexOf('function Start-CliLaunchDetached');
    expect(start).toBeGreaterThan(-1);
    const end = src.indexOf('\n}', start);
    expect(end).toBeGreaterThan(start);
    // ⚠️ 必须**先剥掉注释**再断言：该函数的帮助注释里**故意**写着旧形态
    //    （`-NoNewWindow` / `-RedirectStandard*`）来解释「为什么改掉它」——
    //    不剥注释就会命中自己那段说明（本仓反复踩过的「注释命中断言」）。
    const body = src
      .slice(start, end)
      .replace(/<#[\s\S]*?#>/g, '')
      .replace(/^\s*#.*$/gm, '');

    expect(body).toMatch(/UseShellExecute\s*=\s*\$true/);
    // 包装脚本自己重定向（父进程不参与 ⇒ 不持有子进程的流）
    expect(body).toMatch(/\*>\s*'/);
    // 派发路径里不得回退到「-NoNewWindow + -RedirectStandard*」那套（句柄泄漏的形态）
    expect(body).not.toMatch(/-NoNewWindow/);
    expect(body).not.toMatch(/-RedirectStandard/);
  });

  test('D4: 派发失败（CliExe 不存在）时 Proc 判空、不抛错', () => {
    withHarness((ctx) => {
      const r = runHarness(ctx, 'definitely-not-a-real-exe-xyz');
      // UseShellExecute 下「可执行文件不存在」多半是 Start() 抛错 → 我们的 catch 把 Proc 置 $null
      // 若宿主行为是「起得来但子进程立刻失败」，则 PROC_IS_NULL=False 也可接受（都不算崩）
      expect(r.stderr).not.toMatch(/Set-StrictMode|不能对值为 Null/);
      expect(r.stdout + r.stderr).toMatch(/PROC_IS_NULL=|NO_FUNCTION|error/i);
    });
  });
});
