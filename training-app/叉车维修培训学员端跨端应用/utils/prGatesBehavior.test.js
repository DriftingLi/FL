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
  const { name, token, code, line, emit = true } = opts;
  const file = path.join(tmpRoot, `fake-${name}.ps1`);
  const logFile = path.join(tmpRoot, `fake-${name}.log`);
  const body = [
    '$ErrorActionPreference = "Stop"',
    `. '${psQuote(HXBUSY)}'`,
    `$null = Wait-HxFree -CliExe 'dummy-not-a-real-cli' -TimeoutSeconds 5 -LogPath '${psQuote(logFile)}'`,
    "Write-Host ('FAKE_" + name + "_OWNER=' + $env:HX_LOCK_OWNER)",
    "Write-Host ('FAKE_" + name + "_LOCKPID=' + (Get-Content -LiteralPath (Join-Path $env:TEMP 'hx-agent.lock') | Select-Object -First 1))",
    emit ? `Write-Host '${psQuote(line)}'` : "# (no result token on purpose)",
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
});
