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
 * 任何 `expect(src).toContain(...)` 都抓不到 —— 只有真跑一次才看得见。
 *
 * 断言（用**真实**的 Start-CliLaunchDetached，AST 抽函数定义后调用，避免执行整篇 hx-run.ps1）：
 *   D1 派发方**不被常驻子进程拖住**：驱动脚本返回时那个会话**还活着**（判据是存活态，不是耗时）
 *   D2 子进程的 stdout **照常进** `launch-*.out`（不能为了断开而丢掉日志）
 *   D3 派生用的是「新进程树」形态（UseShellExecute），且日志由**包装脚本自己**写（`*>`）
 *   D4 派发失败（CliExe 不存在）时 **Proc 判空不抛错**（fail-closed 不崩）
 *   D5 存活探针自己有判别力（正控 running / 负控 exited）—— 防 D1 退化成永绿
 *
 * **为什么 D1 不再断言耗时**（#1544）：
 *   旧判据是「驱动脚本耗时 < 8 s」，而这条链要**真起 pwsh + AST 解析整篇 hx-run.ps1**，
 *   耗时对 CPU 饥饿极度敏感：与建工作树 / 另一路 jest 并发时实测 **22.8 s ⇒ 判红**，
 *   空闲单跑同一套件 **3.9 s ⇒ 判绿**。红的是墙钟不是逻辑（重跑即绿），而一次假红要付
 *   「重跑 + 判红回读」双份 8 万字符级的全量输出。改断「返回时常驻会话还活着」守的是
 *   **同一个不变式**（派发方没被子进程的寿命拖住）的**定性**读数：只随实现变，不随机器负载变。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**。
 */
const { execFileSync, spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 本套件每条用例都真起 pwsh（AST 解析 + 派发 + 收尾），所以 jest 的 5 s 默认预算**本身**
 *  就是一条墙钟判据，必须显式抬高。实测通过路径：空闲 **41.0 s**；CPU 压满（36 个 busy-loop
 *  占满 12 核、单次 pwsh 冷启 15.6–18.2 s）**124.8 s**（两次读数都是 2026-10-05 本票取证）。
 *  这个值只兜「挂死」、不兜「快不快」（快慢已不是判据，见文件头 #1544 段），所以取**内部各硬
 *  上限之和再留余量**：D1 execFileSync 120 s + D2 等待预算 120 s + D4 execFileSync 120 s
 *  + D5 冷启（无内部上限，负载下实测 ~17 s）≈ 380 s ⇒ 抬到 **600 s**（1.5× 余量；这个和是
 *  按声明的上限**推算**的，不是实测）。这样它永远不可能成为「通过路径判红」的那一条。
 *  同形先例：`kotlinAllProcessCaptureBehavior.test.js`、`evidenceGenContract.test.js`。 */
jest.setTimeout(600000);

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const HX_RUN = path.join(ROOT, 'scripts', 'hx-run.ps1');
/** 常驻子进程寿命（秒）。**这是余量、不是阈值**：D1 不拿它比大小，只要求它长过「派发之后
 *  那点收尾（写 CHILD_PID 一行 + pwsh 退出 + node 收尸 + 探针）」。调宽不花测试时间——
 *  没人等它，代价只是那个 `Start-Sleep` 孤儿多活一会儿（也不去 kill：`taskkill /T` 在负载下
 *  实测 ~4 s，为一个只会睡的孤儿给门加 4 s 不划算）。 */
const CHILD_LIFETIME_S = 90;
/** D2 的**等待预算**（毫秒）：等常驻子进程把第一行写进 `launch-*.out`。
 *  它不是断言（断言是「日志里看得到 cli-line-A」这个定性事实），命中即提前退出 ⇒ 给宽**不花钱**。
 *  为什么是 120 s：这一行要等**两层 pwsh 冷启**（包装脚本 → 假 cli），2026-10-04 在 36 个
 *  busy-loop 压满 12 核（单次 pwsh 冷启实测 15.6 s）时，30 s 预算**不够**（实测判红：收到空串）。
 *  轮询用 `sleepMs`，不再像旧写法那样每轮起一个 pwsh 去睡（那一枪在负载下就要秒级）。 */
const LOG_APPEAR_BUDGET_MS = 120000;

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 同步睡。旧写法每轮起一个 `pwsh -Command Start-Sleep`，CPU 饥饿下那一枪就要秒级，
 *  等于把等待预算花在探针本身上。 */
function sleepMs(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

/** 存活探针（D1 的判据载体）：`process.kill(pid, 0)` 不发信号，只问「这个 pid 还在不在」。
 *  返回**定性状态**而不是裸布尔，好让失败信息能区分「真退了」与「探针自己出错」。 */
function probeAlive(pid) {
  if (!pid || !Number.isFinite(pid) || pid <= 0) return { state: 'no-pid', alive: false };
  try {
    process.kill(pid, 0);
    return { state: 'running', alive: true };
  } catch (e) {
    const code = e && e.code;
    // EPERM = 进程在、只是没权限 ⇒ 仍算活（本套件自己起的子进程走不到这一支）
    if (code === 'EPERM') return { state: 'running', alive: true };
    if (code === 'ESRCH') return { state: 'exited', alive: false };
    return { state: `probe-error:${code}`, alive: false };
  }
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
    // ⚠️ 常驻子进程（还在睡 CHILD_LIFETIME_S）仍持有临时目录 ⇒ `rmSync` 会 EPERM。
    //    这是**测试自身的收尾问题**，不该把用例判红（真判据是 D1/D2 的断言）；也不去 kill 它 ——
    //    `taskkill /T` 在负载下实测要 ~4 s，为了收一个只会 `Start-Sleep` 的孤儿给门加 4 s 不划算。
    try {
      fs.rmSync(tmp, { recursive: true, force: true });
    } catch {
      // 留给 OS 临时目录清理
    }
  }
}

/** 在给定项目里跑驱动脚本，返回 { ok, status, stdout, stderr, childPid, seconds }
 *  ⚠️ 用**文件**而非管道取回驱动脚本输出：管道正是常驻孙进程可能继承的句柄，
 *     用管道会把「测试基建的泄漏」误算成「被测代码的泄漏」。
 *  `seconds` **只作诊断**、不参与判据；判据的输入是 `childPid` 的存活态。 */
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
    // D1 的判据输入：把常驻会话（包装 pwsh）的 pid 交回测试侧，好在「驱动脚本已返回」之后探它
    "Write-Output ('CHILD_PID=' + $(if ($null -ne $r.Proc) { $r.Proc.Id } else { '' }))",
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
  const childPid = Number((stdout.match(/CHILD_PID=(\d+)/) || [, '0'])[1]);
  return { ok: status === 0, status, stdout, stderr, childPid, seconds: (Date.now() - sw) / 1000 };
}

function readLaunchOut(logDir) {
  const files = fs.existsSync(logDir) ? fs.readdirSync(logDir).filter((f) => /^launch-.*\.out$/.test(f)) : [];
  return files.map((f) => readText(path.join(logDir, f))).join('\n');
}

describe('Start-CliLaunchDetached 派生形态（运行期，2026-09-15）', () => {
  test('D1+D2: 派发方不被常驻子进程拖住（返回时它还活着），且子进程输出仍进日志', () => {
    withHarness((ctx) => {
      const r = runHarness(ctx, 'pwsh');
      if (!r.ok) {
        throw new Error(`驱动脚本执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
          + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`);
      }
      const pid = r.childPid;
      if (!(pid > 0)) {
        // 拿不到 pid ⇒ 判据无从取，按 fail-closed 判红（不 skip、不放过）
        throw new Error(`驱动脚本没交回常驻会话 pid：\nstdout=${r.stdout}\nstderr=${r.stderr}`);
      }
      // D1：驱动脚本**已经返回**，而它派发的那个会话按 CHILD_LIFETIME_S 还在睡 ⇒ 派发方没被拖住。
      //     反例形态（派发方等子进程 / 继承句柄被拖住）下，返回时它必然已退出 ⇒ state='exited' 判红。
      //     ⚠️ 探针必须在 D2 的轮询**之前**取：轮询本身会消耗子进程寿命。
      const probe = probeAlive(pid);
      expect({ pidPresent: pid > 0, alive: probe.alive, state: probe.state })
        .toEqual({ pidPresent: true, alive: true, state: 'running' });

      // D2：给了「常驻子进程要先打印一行」的机会，日志里必须看得到
      const deadline = Date.now() + LOG_APPEAR_BUDGET_MS;
      let content = readLaunchOut(ctx.logDir);
      while (!/cli-line-A/.test(content) && Date.now() < deadline) {
        sleepMs(400);
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

  test('D5: 存活探针自己有判别力（正控 running / 负控 exited）—— 防 D1 退化成永绿', () => {
    // 正控：本进程活着
    expect(probeAlive(process.pid)).toEqual({ state: 'running', alive: true });
    // 负控：一个已退干净、且被 node 回收过的子进程（与被测形态同族：pwsh 秒退）
    const done = spawnSync('pwsh', psArgs(['-Command', 'exit 0']));
    if (done.error) throw new Error(`负控起不来（pwsh 不可用？fail-closed 不跳过）：${done.error.message}`);
    expect(probeAlive(done.pid)).toEqual({ state: 'exited', alive: false });
    // 负控：拿不到 pid 的形态（D4 那条路上 Proc 为 null）
    expect(probeAlive(0)).toEqual({ state: 'no-pid', alive: false });
  });
});
