/**
 * 「无代码变更预检」的**行为守护**（真跑 `scripts/lib/hx-noop.ps1`，#2027）
 *
 * 为什么要有它（Standards 轴 H1，2026-10-27 复审）：
 *   `utils/hxNoopContract.test.js` 只做源码文本扫描（落 `test:unit` 接线腿），它证明的是
 *   **扫描器**，证明不了那段 PowerShell 本身。而 `docs/agents/guards.md` 末节第 3 问写死：
 *   「若是接线守护：它守的是哪条接线？以及这条接线由**哪个行为守护**兜底行为？
 *     （两者都要，缺前者会漂、**缺后者会假绿**）」
 *   ⇒ 本文件就是那个兜底的行为守护：dot-source 真模块 → 进程内真跑函数 → 断言返回。
 *
 * 它要抓的那条真缺陷（Spec 轴 (c)1）：
 *   fail-safe 被写成了 fail-**open**。`git status` 一旦失败（索引损坏、并发 agent 的
 *   `index.lock`、dubious ownership），脚本里的 `catch { $dirty = @() }` 会把它读成
 *   「工作树干净」⇒ 判「没活」⇒ **跳过一次真需要的部署**。而这正是本模块头注释承诺绝不发生的事
 *   （「绝不用『猜』挡掉一次真需要的部署」）。
 *
 * 手法沿用 `utils/evidenceGenContract.test.js`（#2029 刚钉过 UTF-8）：`spawnSync` 起子进程、
 * 输出前钉 `[Console]::OutputEncoding`。**取不到 pwsh 即 fail-loud**，不静默 skip。
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const { spawnSync } = require('child_process');

jest.setTimeout(120000);

const ROOT = path.join(__dirname, '..');
const LIB = path.join(ROOT, 'scripts', 'lib', 'hx-noop.ps1');

/** 子进程输出前钉 UTF-8（与 scripts/dev-finish.ps1 顶部、evidenceGenContract 同款）。 */
const UTF8_OUTPUT_PREAMBLE = 'try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }';

function resolveShell() {
  for (const exe of ['pwsh', 'powershell']) {
    const r = spawnSync(exe, ['-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()'], {
      encoding: 'utf8',
    });
    if (!r.error && r.status === 0 && /\d/.test(r.stdout || '')) return exe;
  }
  throw new Error('未找到 pwsh/powershell —— 本行为守护须跑在 PowerShell 可用的环境（不静默 skip）');
}

const SHELL = resolveShell();

/** PowerShell 单引号字符串转义（路径可能含空格/单引号）。 */
function psq(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

function runPs(statements) {
  const r = spawnSync(
    SHELL,
    ['-NoProfile', '-Command', [UTF8_OUTPUT_PREAMBLE].concat(statements).join('; ')],
    { encoding: 'utf8', cwd: ROOT }
  );
  if (r.status !== 0) {
    throw new Error(`pwsh 退出码 ${r.status}：${(r.stderr || '').trim()}`);
  }
  return (r.stdout || '').trim();
}

/** 在临时目录造一个最小真仓库（真 git，不用桩）。 */
function makeRepo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hxnoop-'));
  const git = (...args) => {
    const r = spawnSync('git', ['-C', dir, ...args], { encoding: 'utf8' });
    if (r.status !== 0) throw new Error(`git ${args.join(' ')} 失败：${(r.stderr || '').trim()}`);
    return (r.stdout || '').trim();
  };
  git('init', '-q');
  git('config', 'user.email', 'hxnoop@test.local');
  git('config', 'user.name', 'hxnoop');
  fs.writeFileSync(path.join(dir, 'a.txt'), 'a\n', 'utf8');
  git('add', 'a.txt');
  git('commit', '-q', '-m', 'init');
  return { dir, git, head: git('rev-parse', 'HEAD') };
}

function markerPath(dir) {
  return path.join(dir, '.ci-verify', 'last-deploy.json');
}

function writeMarker(dir, payload) {
  const p = markerPath(dir);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, JSON.stringify(payload), 'utf8');
  return p;
}

/** 真跑一次判定；返回 'True' | 'False'。 */
function noop(dir, marker) {
  const out = runPs([
    `. ${psq(LIB)}`,
    `$r = Test-HxNoopRerun -Project ${psq(dir)} -MarkerPath ${psq(marker)}`,
    '"$($r.Noop)"',
  ]);
  return out.split(/\r?\n/).pop().trim();
}

/**
 * 同上，但同时带回 `Reason`（#2027 复审指出的弱点）：
 * 只断言 `Noop=false` 的话，「因为 git status 失败而放行」与「因为 rev-parse 失败而放行」
 * 两种**不同的提前返回**长得一模一样 ⇒ 测试会为错误的理由变绿。
 */
function noopDetail(dir, marker) {
  const out = runPs([
    `. ${psq(LIB)}`,
    `$r = Test-HxNoopRerun -Project ${psq(dir)} -MarkerPath ${psq(marker)}`,
    '"$($r.Noop)|$($r.Reason)"',
  ]);
  const line = out.split(/\r?\n/).pop().trim();
  const sep = line.indexOf('|');
  return { noop: line.slice(0, sep), reason: line.slice(sep + 1) };
}

describe('hx-noop 预检的行为（真跑 PowerShell）', () => {
  it('无标记 ⇒ 放行（fail-safe 起点）', () => {
    const { dir } = makeRepo();
    expect(noop(dir, markerPath(dir))).toBe('False');
  });

  /**
   * S2 接缝（原计划里有、第一轮漏掉的那条；Standards 指出 N8 只有文本守护）。
   * 期望值 headSha 取自 **git**，不是按实现的算法重算 —— 避免 `/tdd` 点名的套套逻辑。
   */
  it('写标记：落盘 headSha = 当前 HEAD；非仓库不抛、只返 false', () => {
    const { dir, head } = makeRepo();
    const marker = path.join(dir, '.ci-verify', 'last-deploy.json');

    const ok = runPs([
      `. ${psq(LIB)}`,
      `Write-HxDeployMarker -Project ${psq(dir)} -MarkerPath ${psq(marker)}`,
    ]);
    expect(ok.split(/\r?\n/).pop().trim()).toBe('True');

    const payload = JSON.parse(fs.readFileSync(marker, 'utf8'));
    expect(payload.headSha).toBe(head);
    expect(typeof payload.ts).toBe('string');

    // 非仓库 ⇒ 不抛、返回 false（标记只是省时间，不能因为它把一次成功部署判红）
    const notRepo = fs.mkdtempSync(path.join(os.tmpdir(), 'hxnoop-nr-'));
    const m2 = path.join(notRepo, 'm.json');
    const bad = runPs([
      `. ${psq(LIB)}`,
      `Write-HxDeployMarker -Project ${psq(notRepo)} -MarkerPath ${psq(m2)}`,
    ]);
    expect(bad.split(/\r?\n/).pop().trim()).toBe('False');
    expect(fs.existsSync(m2)).toBe(false);
  });

  it('标记存在 + HEAD 未变 + 工作树干净 ⇒ 跳过', () => {
    const { dir, head } = makeRepo();
    const marker = writeMarker(dir, { headSha: head, ts: 'test' });
    expect(noop(dir, marker)).toBe('True');
  });

  it('标记存在 + HEAD 已变 ⇒ 放行', () => {
    const { dir } = makeRepo();
    const marker = writeMarker(dir, { headSha: 'deadbeef'.repeat(5), ts: 'test' });
    expect(noop(dir, marker)).toBe('False');
  });

  it('标记存在 + 工作树有跟踪改动 ⇒ 放行', () => {
    const { dir, head } = makeRepo();
    const marker = writeMarker(dir, { headSha: head, ts: 'test' });
    fs.writeFileSync(path.join(dir, 'a.txt'), 'changed\n', 'utf8');
    expect(noop(dir, marker)).toBe('False');
  });

  /**
   * ★ 本条就是 Spec 轴 (c)1 的红：先把「干净 ⇒ 跳过」的前提钉住，再破坏 git 索引。
   * 索引损坏后 `git rev-parse HEAD` 仍可用（它不读索引），但 `git status` 会以非零退出
   * ——正是并发 agent 的 `index.lock` / dubious ownership 那一类现场。
   * 正确行为是**放行**（拿不准就别挡），当前实现会返回 True ⇒ 本条红。
   */
  it('git status 失败 ⇒ 必须放行（绝不把「读不出」当成「没活」）', () => {
    const { dir, head } = makeRepo();
    const marker = writeMarker(dir, { headSha: head, ts: 'test' });

    // 前提：此时确实是「跳过」——否则下面那条断言会在一个已经坏掉的前提上假通过
    expect(noop(dir, marker)).toBe('True');

    fs.writeFileSync(path.join(dir, '.git', 'index'), 'CORRUPTED-NOT-AN-INDEX', 'utf8');

    // 确认实验条件成立：rev-parse 还能用（否则走的是另一条 fail-safe 分支，测不到本条）
    const r = spawnSync('git', ['-C', dir, 'rev-parse', 'HEAD'], { encoding: 'utf8' });
    expect(r.status).toBe(0);

    const verdict = noopDetail(dir, marker);
    expect(verdict.noop).toBe('False');
    // 必须因为**这一条**放行：否则 rev-parse 之类的其它提前返回也会让上一行变绿（假通过）
    expect(verdict.reason).toContain('git status 失败');
  });
});
