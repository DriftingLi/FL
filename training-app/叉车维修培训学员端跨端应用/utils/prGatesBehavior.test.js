/**
 * PR 面一次取锁串 ② 与 ④c（`scripts/pr-gates.ps1`，#2112）的**行为守护**。
 *
 * 为什么这层必须存在：
 *   这张票的全部价值是一句**可机检的事实** ——「两张门共用**一次** HBuilderX 租约」。
 *   `expect(src).toContain('Wait-HxFree')` 这类接线断言对它**零判别力**：源码里 `Wait-HxFree`
 *   出现一次还是一百次，跟「运行时到底取了几次锁」没有必然关系（子进程可能照样各取一次，
 *   源码文本一模一样）。所以这里用 pwsh **真跑**入口，用**假的**门载体把两门的取锁行为
 *   原样演出来，再去数日志里的 `HX_BUSY ... result=` 行。
 *
 * 判据分组（成对，见 docs/agents/guards.md 末节）：
 *   R1 绿跑：两门都过 ⇒ exit 0，结果行 `mp=ok kotlin=ok`
 *   R2 **单租约**（本票主旨）：日志里 `result=free` **恰好 1 条**、`result=inherited` **恰好 2 条**，
 *      且两个子门看到的 `HX_LOCK_OWNER` == 锁文件里的 pid（证明「复用」而不是「各自加锁」）
 *   R3 两门结果行**原样**：假门打的那两行在入口 stdout 里逐字节出现（不加前缀、不改写）
 *   R4 顺序与不截断：② 在前；② 红时 ④c 照跑（「没跑」不许被读成「这一半没问题」）
 *   R5 最重者胜：一门 1、另一门 2 ⇒ exit 2；缺结果行（exit 0 却无 token）⇒ no-line + exit 2（fail-closed）
 *   R6 锁必须释放：红跑 / 环境跑之后 `hx-agent.lock` 都不在
 *   R7 `-DryRun`：不取锁、不写日志、不打两门（看计划这件事不能先占住共享主程序）
 *   R8 收流有界（#2147）：② 退出后**孙进程仍握着它的 stdout 句柄**（开发者工具那种常驻 GUI 的形态）
 *      ⇒ 入口必须**照样返回**（旧写法挂在 `Tee-Object` 等 EOF 上：实测 1050 s、④c 没开始），
 *      且 ④c 照跑、两门结果行都在。这条锁的是「入口**真的用了**有界收流器」—— 库里有个函数、
 *      入口却还写着管道，库的单测照样绿（接线与行为的成对义务，见 docs/agents/guards.md 末节）。
 *   R9 真实门打印形（#2162）：两门脚本的 `Write-Log` 必须把**行首结果行原样透传 stdout**
 *      —— 入口按 `^token` 认读数，而历史上通过/env 分支只把结论行写进日志文件（中文前缀行进不了
 *      解析），`PR_GATES` 因此**从未读出过 ok**。本条是源码文本判据（接线面），它的行为面 = R10 +
 *      真跑取数（#2162 票面 AC4，需 HBuilderX 环境）。
 *   R10 旧真门形钉成反例（#2162）：假门按**旧真实形状**只打中文前缀行 ⇒ 入口判 `no-line`、
 *      exit=env —— 「绿跑读成 no-line」这个成因从此有回归防线，不许再固化（#2098 教训：
 *      假门夹具若打了真门**不产出**的行首裸 token，守护就变成假绿的守卫）。
 *
 * 安全性（为什么可在任何机器上跑、也不抢维护者的 HBuilderX）：
 *   路径一律带 `-TempRoot <临时目录>`：锁目录被换到临时根（父进程与两个子门用同一个），
 *   于是永远不会撞 `$env:TEMP\hx-agent.lock` 上那个真实会话；`-Cli` 指向本进程的 pwsh，
 *   忙探测只是「跑一个能立刻退出的进程」。两门载体用 `-MpScript/-KotlinScript` 换成假门，
 *   不碰 HBuilderX、不碰微信开发者工具、不编译。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（仓库先例：
 *   styleLoopBehavior / hxLaunchDetachBehavior / screenshotDiffBehavior）。
 *
 * ⚠️ 判据 token 全 ASCII：JS 侧绝不匹配中文（pwsh stdout 的码页历史血账）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const ENTRY = path.join(ROOT, 'scripts', 'pr-gates.ps1');
const HXBUSY = path.join(ROOT, 'scripts', 'lib', 'hx-busy.ps1');

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass');
  return args.concat(extra);
}

function psQuote(s) {
  return String(s).replace(/'/g, "''");
}

/** 起一次入口。返回 { code, out }（code 是 pwsh 进程退出码，out 是合并后的 stdout+stderr）。 */
function runPrGates(args) {
  try {
    const out = execFileSync('pwsh', psArgs(['-File', ENTRY].concat(args)), {
      encoding: 'utf8',
      timeout: 180000,
      windowsHide: true,
      maxBuffer: 32 * 1024 * 1024,
      // pwsh 自己的退出码就是入口的退出码（-File 语义），非 0 会抛
    });
    return { code: 0, out: String(out) };
  } catch (e) {
    if (e.code === 'ENOENT') {
      throw new Error('本套件需要 pwsh（PowerShell 7）：找不到可执行文件（fail-closed，不 skip）');
    }
    const out = `${e.stdout || ''}${e.stderr || ''}`;
    if (typeof e.status !== 'number') throw new Error(`入口执行失败：${e.message}\n${out}`);
    return { code: e.status, out: String(out) };
  }
}

/** 造一个临时项目根（入口要看到 manifest.json 才认它是 uni-app-x 项目）。 */
function makeProject(tmpRoot) {
  const dir = fs.mkdtempSync(path.join(tmpRoot, 'proj-'));
  fs.writeFileSync(path.join(dir, 'manifest.json'), '{"appid":"__UNI__TEST"}\n', 'utf8');
  return dir;
}

/**
 * 造假门：dot-source 真的 hx-busy（取锁语义必须是真的，否则测的是假货），
 * 然后按给定 token / 退出码演一门。
 */
function makeFakeGate(tmpRoot, opts) {
  const {
    name, token, code, line,
    emit = true,
    holdStdout = false,
    // #2162：假门默认按**真门修正后的打印形**演 —— 先一行行首裸结果行（= 日志里那行，逐字节），
    // 再一行中文结论行给人读。旧形（只有中文行）走 legacyChineseOnly，留给 R10 当反例。
    legacyChineseOnly = false,
  } = opts;
  const file = path.join(tmpRoot, `fake-${name}.ps1`);
  const logFile = path.join(tmpRoot, `fake-${name}.log`);
  const printBlock = legacyChineseOnly
    ? `Write-Host ('✅ ${name} 通过：' + '${psQuote(line)}')`
    : [
        `Write-Host '${psQuote(line)}'`,
        `Write-Host ('✅ ${name} 通过：' + '${psQuote(line)}')`,
      ].join('\n');
  const body = [
    '$ErrorActionPreference = "Stop"',
    `. '${psQuote(HXBUSY)}'`,
    `$null = Wait-HxFree -CliExe 'dummy-not-a-real-cli' -TimeoutSeconds 5 -LogPath '${psQuote(logFile)}'`,
    "Write-Host ('FAKE_" + name + "_OWNER=' + $env:HX_LOCK_OWNER)",
    "Write-Host ('FAKE_" + name + "_LOCKPID=' + (Get-Content -LiteralPath (Join-Path $env:TEMP 'hx-agent.lock') | Select-Object -First 1))",
    emit ? printBlock : "# (no result token on purpose)",
    // #2147 复现件：退出后留一个**孙进程继续持有本进程的 stdout 句柄**（微信开发者工具那种常驻 GUI 的形态）。
    // 必须走 .NET + UseShellExecute=false：Start-Process 默认走 ShellExecuteEx，子进程拿不到我们的管道句柄
    // ⇒ 夹具退化成「什么都没持有」，R8 就变成假绿。
    holdStdout
      ? [
        '$holderPsi = [System.Diagnostics.ProcessStartInfo]::new()',
        '$holderPsi.FileName = (Get-Process -Id $PID).Path',
        '$holderPsi.Arguments = \'-NoProfile -NonInteractive -Command "Start-Sleep -Seconds 15"\'',
        '$holderPsi.UseShellExecute = $false',
        '$holderPsi.CreateNoWindow = $true',
        '$null = [System.Diagnostics.Process]::Start($holderPsi)'
      ].join('\n')
      : '# (no handle holder)',
    `exit ${code}`,
    ''
  ].join('\n');
  fs.writeFileSync(file, body, 'utf8');
  return { file, token, line };
}

/** 统计某个 token 在输出里出现的次数（按行精确匹配行首）。 */
function countLines(out, re) {
  return out.split(/\r?\n/).filter((l) => re.test(l)).length;
}

function resultLine(out) {
  const rows = out.split(/\r?\n/).filter((l) => l.startsWith('PR_GATES '));
  return rows.length ? rows[rows.length - 1] : '';
}

describe('pr-gates：PR 面一次取锁串 ② 与 ④c（#2112）', () => {
  let tmpRoot;

  beforeEach(() => {
    tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pr-gates-'));
  });

  afterEach(() => {
    try { fs.rmSync(tmpRoot, { recursive: true, force: true }); } catch { /* 清不掉不判红 */ }
  });

  function baseArgs(project, mpGate, kotlinGate) {
    return [
      '-Project', project,
      '-Cli', process.execPath,
      '-TempRoot', tmpRoot,
      '-MpScript', mpGate.file,
      '-KotlinScript', kotlinGate.file,
      '-LogPath', path.join(tmpRoot, 'pr-gates.log')
    ];
  }

  it('R1/R2/R3：绿跑 ⇒ exit 0、单租约（free×1 + inherited×2）、两门结果行原样', () => {
    const project = makeProject(tmpRoot);
    const mpLine = 'MP_WEIXIN_RESULT errorsTotal=0 shots=2 pages=pages/index/index coverage=smoke:2';
    const ktLine = 'KOTLIN_ALL_RESULT errors=0 classes=1505 files=120 freshness=fresh';
    const mp = makeFakeGate(tmpRoot, { name: 'MP', token: 'MP_WEIXIN_RESULT', code: 0, line: mpLine });
    const kt = makeFakeGate(tmpRoot, { name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: ktLine });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(code).toBe(0);
    expect(resultLine(out)).toMatch(/^PR_GATES mode=run mp=ok kotlin=ok exit=ok locks=1 acquire=free /);

    // R2 单租约：本票的主旨 —— 真取锁一次，两个子门都是「继承」
    expect(countLines(out, /HX_BUSY wait=\d+ result=free/)).toBe(1);
    expect(countLines(out, /HX_BUSY wait=0 result=inherited/)).toBe(2);

    // 子门看到的持有者 == 锁文件里的 pid（且非空）⇒ 是「复用」而不是「各自加锁」
    const ownerMp = out.match(/FAKE_MP_OWNER=(\d+)/);
    const pidMp = out.match(/FAKE_MP_LOCKPID=(\d+)/);
    const ownerKt = out.match(/FAKE_KT_OWNER=(\d+)/);
    expect(ownerMp).not.toBeNull();
    expect(pidMp).not.toBeNull();
    expect(ownerMp[1]).toBe(pidMp[1]);
    expect(ownerKt[1]).toBe(pidMp[1]);

    // R3 结果行**内容**逐字节原样（没有前缀、没有后缀、没有被改写）。
    // 行尾的 CR 不算改写：那是子进程 CRLF 与 pwsh 回显叠加出来的**行尾符**，不是本文具对内容的加工。
    const lines = out.split(/\r?\n/).map((l) => l.replace(/\s+$/, ''));
    expect(lines).toContain(mpLine);

    // R4 顺序：② 在 ④c 之前
    expect(out.indexOf(mpLine)).toBeGreaterThan(-1);
    expect(out.indexOf(ktLine)).toBeGreaterThan(out.indexOf(mpLine));

    // R6 锁已释放
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
  });

  it('R4/R5：② 红 ⇒ exit 1，且 ④c 照跑（不截断）', () => {
    const project = makeProject(tmpRoot);
    const mp = makeFakeGate(tmpRoot, {
      name: 'MP', token: 'MP_WEIXIN_RESULT', code: 1,
      line: 'MP_WEIXIN_RESULT errors=2 failures=consoleErrors'
    });
    const kt = makeFakeGate(tmpRoot, {
      name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: 'KOTLIN_ALL_RESULT errors=0 classes=1505'
    });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(code).toBe(1);
    expect(resultLine(out)).toMatch(/mp=fail kotlin=ok exit=fail/);
    // 「没跑」不许被读成「这一半没问题」：第二门的结果行必须在
    expect(out).toContain('KOTLIN_ALL_RESULT errors=0 classes=1505');
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
  });

  it('R5：环境受阻压过门未过（② 红 + ④c env ⇒ exit 2）', () => {
    const project = makeProject(tmpRoot);
    const mp = makeFakeGate(tmpRoot, {
      name: 'MP', token: 'MP_WEIXIN_RESULT', code: 1, line: 'MP_WEIXIN_RESULT errors=1 failures=exceptions'
    });
    const kt = makeFakeGate(tmpRoot, {
      name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 2, line: 'KOTLIN_ALL_RESULT errors=env reason=cli-unreachable'
    });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(code).toBe(2);
    expect(resultLine(out)).toMatch(/mp=fail kotlin=env exit=env/);
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
  });

  it('R5：exit 0 却没打结果行 ⇒ no-line + exit 2（fail-closed，别把「崩了」读成「过了」）', () => {
    const project = makeProject(tmpRoot);
    const mp = makeFakeGate(tmpRoot, {
      name: 'MP', token: 'MP_WEIXIN_RESULT', code: 0, line: '', emit: false
    });
    const kt = makeFakeGate(tmpRoot, {
      name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: 'KOTLIN_ALL_RESULT errors=0'
    });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(code).toBe(2);
    expect(resultLine(out)).toMatch(/mp=no-line kotlin=ok exit=env/);
  });

  it('R8：② 退出后孙进程仍握 stdout 句柄 ⇒ 入口照样返回，且 ④c 照跑（#2147）', () => {
    const project = makeProject(tmpRoot);
    const mpLine = 'MP_WEIXIN_RESULT errors=1 failures=noConsoleErrors';
    const ktLine = 'KOTLIN_ALL_RESULT errors=0 classes=1560 files=126';
    const mp = makeFakeGate(tmpRoot, {
      name: 'MP', token: 'MP_WEIXIN_RESULT', code: 1, line: mpLine, holdStdout: true
    });
    const kt = makeFakeGate(tmpRoot, { name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: ktLine });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(code).toBe(1);
    const line = resultLine(out);
    expect(line).toMatch(/mp=fail kotlin=ok exit=fail/);
    // 承重判据 = **收流被放弃这件事在日志里可读**（不是墙钟阈值）：门自己打 drain=abandoned ⇒ 它确实
    // 撞上了「子进程已退出但句柄未释放」，并且选择放弃剩余输出继续跑，而不是等 EOF。
    // ⚠️ 不再用「seconds < 12」这类贴边时序断言：门本身基线 8 s + DrainMs 5 s 已贴近 holder 存活量级，
    // 单跑/整文件跑读数会在阈值两侧摆（实测 13–16 s），那是环境敏感守护（guards.md 末节 Q4 / #2078 同族）。
    expect(out).toContain('drain=abandoned');
    // 粗上界只兜「真挂死」：holder 活 15 s，一旦在它自然退出后才收口，读数会明显更大
    const secs = Number((line.match(/seconds=(\d+)/) || [])[1]);
    expect(Number.isFinite(secs)).toBe(true);
    expect(secs).toBeLessThan(60);
    // 「没跑」不许被读成「这一半没问题」：② 红时 ④c 仍必须跑到并打出结果行
    expect(out).toContain(ktLine);
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
  });

  it('R7：-DryRun 不取锁、不写日志', () => {
    const project = makeProject(tmpRoot);
    const mp = makeFakeGate(tmpRoot, { name: 'MP', token: 'MP_WEIXIN_RESULT', code: 0, line: 'MP_WEIXIN_RESULT x=1' });
    const kt = makeFakeGate(tmpRoot, { name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: 'KOTLIN_ALL_RESULT x=1' });
    const logPath = path.join(tmpRoot, 'pr-gates.log');

    const { code, out } = runPrGates(baseArgs(project, mp, kt).concat(['-DryRun']));

    expect(code).toBe(0);
    expect(out).toContain('PR_GATES mode=dryrun exit=ok');
    expect(out).toContain('mp-weixin-check.ps1');
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
    expect(fs.existsSync(logPath)).toBe(false);
  });

  it('R9（#2162）：真实门的 Write-Log 必须把行首结果行原样透传 stdout（源码文本判据；行为面 = R10 + 票面 AC4 真跑）', () => {
    const cases = [
      ['scripts/mp-weixin-check.ps1', "'^MP_WEIXIN(_BUILDONLY)?_RESULT(\\s|$)'"],
      ['scripts/kotlin-all-check.ps1', "'^KOTLIN_ALL_RESULT(\\s|$)'"],
    ];
    for (const [rel, anchor] of cases) {
      const src = fs.readFileSync(path.join(ROOT, rel), 'utf8');
      // 找不到多行函数体 = 实现退回单行「只写文件」⇒ 假形状回来了，这条断言红即是判据
      const fn = src.match(/function Write-Log \{[\s\S]*?\n\}/);
      expect(fn).not.toBeNull();
      expect(fn[0]).toContain('Add-Content');   // 文件仍是字节级真源
      expect(fn[0]).toContain('-split');        // 逐行判行首，不许整串匹配
      expect(fn[0]).toContain(anchor);          // 锚定本门结果 token（含 -BuildOnly 独立 token）
      expect(fn[0]).toContain('Write-Host');    // 原样透传，不加前后缀
      // ② 侧额外排除投递诊断行（`comment=FAILED` 同样裸 token 开头，入口取最后一条会被它顶掉读数行）
      if (rel.includes('mp-weixin')) expect(fn[0]).toContain("-notmatch ' comment='");
    }
  });

  it('R10（#2162）：假门按**旧真门形**只打中文前缀行 ⇒ 入口判 mp=no-line、exit=env —— 成因钉成反例', () => {
    const project = makeProject(tmpRoot);
    const mpLine = 'MP_WEIXIN_RESULT errorsTotal=0 shots=1 coverage=smoke:2';
    const mp = makeFakeGate(tmpRoot, { name: 'MP', token: 'MP_WEIXIN_RESULT', code: 0, line: mpLine, legacyChineseOnly: true });
    const kt = makeFakeGate(tmpRoot, { name: 'KT', token: 'KOTLIN_ALL_RESULT', code: 0, line: 'KOTLIN_ALL_RESULT errors=0 classes=1 files=1' });

    const { code, out } = runPrGates(baseArgs(project, mp, kt));

    expect(resultLine(out)).toMatch(/mp=no-line/);
    expect(resultLine(out)).toMatch(/kotlin=ok/);
    expect(code).toBe(2); // no-line 按环境受阻算：别把「读不到」放成「过了」
    expect(fs.existsSync(path.join(tmpRoot, 'hx-agent.lock'))).toBe(false);
  });
});
