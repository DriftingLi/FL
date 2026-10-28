/**
 * `scripts/remove-worktree.ps1 -PruneOrphans` 的判据守护（#2081，2026-10-28）
 *
 * 为什么需要：`check-branch-residue.ps1` 只报孤儿目录（#2061 的票面就是「只报告不删除」），
 * 而主树实测有 **48 个**未注册的 `wt-*` 目录，其中 33 个挂着指回**主树** node_modules 的 junction。
 * 这类目录没有任何安全的批量出口：单棵树那套 `-Task <n>` 是给**注册表里**的树用的（还会顺手动分支），
 * 而「直接 `Remove-Item -Recurse`」在跟随链接的实现上会把主树 188 条依赖一起删掉
 * （#2061 当天一度虚惊的就是这件事）。
 *
 * 本套件钉住四件事（每条都能被改坏后单独变红）：
 *   1. 默认 **dry-run**：只出计划，一个目录都不动、一条链接都不摘。
 *   2. `-Execute` **先摘链接、再删骨架**：目录没了，而**链接目标原样还在**（承重断言）。
 *   3. 对账做不成（取不到注册表）⇒ **拒绝删**，不是「磁盘看起来干净就当干净」。
 *   4. 调用方 CWD 在待删目录内 ⇒ 拒绝（Windows 语义：进程当前目录占着目录，删不掉；强删会留下半截）。
 *
 * ⚠️ 判据 token 全 ASCII：pwsh 的 stdout 在 Windows 上按 OEM 码页写，中文到 Node 侧会变乱码。
 * ⚠️ 用例全在临时目录里造（`-WtRoot`），**绝不**碰主树那 48 个真孤儿 —— 本套件跑绿不代表它们被删了。
 */

const { spawnSync, execSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'remove-worktree.ps1');

// 被测对象是 **Windows-only** 的：`remove-worktree.ps1` 一进来就 `if (-not $IsWindows) { exit 1 }`
//（junction / cmd rmdir / icacls 都是 Windows 语义），而 ③ 门跑在 ubuntu-latest ⇒ 这里按本仓既有形态
// 跳过（同 behaviorLegWatchdogBehavior / behaviorSubsetBehavior 的 `describe.skip` 写法）。
//
// ⚠️ 这不是「没覆盖」的借口，边界是刻意划的：
//   · **分档语义**（哪些算孤儿、活链接 vs 纯骨架、读不到不许当骨架、不跟随链接）在 lib 里，
//     由 branchResidueBehavior.test.js 在**两平台**覆盖（它只读文件系统，不删东西）；
//   · **删除语义**（先摘链接再删骨架、摘不掉就不删、链接目标原样还在）只能在本机 Windows 验 ——
//     它要的正是 junction 与 ACL，Linux 上没有对应物。
const describeWin = process.platform === 'win32' ? describe : describe.skip;

jest.setTimeout(240000);

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function runPrune(extra, opts) {
  const o = opts || {};
  const args = ['-File', SCRIPT, '-PruneOrphans'].concat(extra || []);
  const r = spawnSync('pwsh', psArgs(args), { encoding: 'utf8', cwd: o.cwd || ROOT });
  return { code: r.status, out: String(r.stdout || '') + String(r.stderr || '') };
}

let TMP = null;
beforeAll(() => { TMP = fs.mkdtempSync(path.join(os.tmpdir(), 'prune-')); });
afterAll(() => { if (TMP) fs.rmSync(TMP, { recursive: true, force: true }); });

/**
 * 造「真孤儿 + 纯骨架」各一个的扫描根：
 *   wt-9001 —— 挂一条 node_modules junction，指向本用例自己的目标是目录（真机上是主树 node_modules），
 *              目标里有 2 个文件（其中 sentinel.txt 用来判「目标没被跟着删」）
 *   wt-9002 —— 纯骨架：只有空目录
 * 目标目录刻意不以 wt- 开头（否则它自己也会被当作候选目录）。
 */
function makeRoot(name) {
  const root = path.join(TMP, name);
  fs.mkdirSync(root, { recursive: true });

  const target = path.join(root, '_target_wt-9001');
  fs.mkdirSync(target, { recursive: true });
  fs.writeFileSync(path.join(target, 'sentinel.txt'), 'keep me');
  fs.writeFileSync(path.join(target, 'dep.js'), 'x');

  const linked = path.join(root, 'wt-9001', 'training-app', 'app-under-test');
  fs.mkdirSync(linked, { recursive: true });
  fs.symlinkSync(target, path.join(linked, 'node_modules'), 'junction');

  fs.mkdirSync(path.join(root, 'wt-9002', 'training-app', 'app-under-test'), { recursive: true });
  return { root, target, linkedDir: path.join(root, 'wt-9001') };
}

describeWin('remove-worktree -PruneOrphans：孤儿目录的批量安全出口（#2081）', () => {
  test('PR1 默认 dry-run：出计划、两档都点名，但一个目录都不动、链接不摘', () => {
    const { root, target, linkedDir } = makeRoot('pr1');
    const r = runPrune(['-WtRoot', root]);

    expect(r.code).toBe(0);
    expect(r.out).toContain('PRUNE_PLAN wt-9001 tier=orphan links=1');
    expect(r.out).toContain('PRUNE_PLAN wt-9002 tier=skeleton links=0');
    expect(r.out).toContain('PRUNE_LINK wt-9001 -> _target_wt-9001 exists=1');
    expect(r.out).toContain('PRUNE_SUMMARY orphans=2');
    expect(r.out).toMatch(/dry-run/i);
    // 只读：目录、junction、目标全在
    expect(fs.existsSync(linkedDir)).toBe(true);
    expect(fs.existsSync(path.join(linkedDir, 'training-app', 'app-under-test', 'node_modules'))).toBe(true);
    expect(fs.existsSync(path.join(target, 'sentinel.txt'))).toBe(true);
    expect(fs.existsSync(path.join(root, 'wt-9002'))).toBe(true);
  });

  test('PR2 -Execute：先摘链接再删骨架 —— 两个目录都没了，而链接目标一个字节没少', () => {
    const { root, target } = makeRoot('pr2');
    const before = fs.readdirSync(target).sort();
    const r = runPrune(['-WtRoot', root, '-Execute']);

    expect(r.code).toBe(0);
    expect(r.out).toContain('PRUNE_OK wt-9001');
    expect(r.out).toContain('PRUNE_OK wt-9002');
    // links=1 是 detached 的**实测数**（不是枚举数）：去掉「摘链接」这一步，这里会变成 links=0 ⇒ 红
    expect(r.out).toContain('PRUNE_SUMMARY orphans=2 removed=2 linked=1 skeleton=1 links=1 targetsKept=1/1');
    expect(r.out).toMatch(/LINK_TARGET_KEPT .*entries=2 after=2/);

    // 目录没了
    expect(fs.existsSync(path.join(root, 'wt-9001'))).toBe(false);
    expect(fs.existsSync(path.join(root, 'wt-9002'))).toBe(false);
    // 承重断言：目标目录及其文件原样还在（跟随链接的实现会在这里变红）
    expect(fs.existsSync(target)).toBe(true);
    expect(fs.readdirSync(target).sort()).toEqual(before);
    expect(fs.readFileSync(path.join(target, 'sentinel.txt'), 'utf8')).toBe('keep me');
  });

  test('PR3 对账做不成 ⇒ 拒绝删（fail-closed），目录原样留着', () => {
    const { root } = makeRoot('pr3');
    const stub = path.join(TMP, 'pr3-git-stub');
    fs.mkdirSync(stub, { recursive: true });
    const exe = path.join(stub, 'git.cmd');
    // 注册表永远为空的桩：验「对账做不成就拒绝删」这条 fail-closed 腿
    fs.writeFileSync(exe, '@echo off\r\nexit /b 0\r\n', 'utf8');

    const r = runPrune(['-WtRoot', root, '-Execute', '-GitCli', exe]);

    expect(r.code).toBe(2);
    expect(r.out).toContain('PRUNE_REFUSED');
    expect(fs.existsSync(path.join(root, 'wt-9001'))).toBe(true);
    expect(fs.existsSync(path.join(root, 'wt-9002'))).toBe(true);
  });

  test('PR5 摘链失败 ⇒ 不删（fail-closed）：目录与它里面的文件都还在，exit 3', () => {
    const { root, target } = makeRoot('pr5');
    const link = path.join(root, 'wt-9001', 'training-app', 'app-under-test', 'node_modules');
    fs.writeFileSync(path.join(root, 'wt-9001', 'marker.txt'), 'keep me too');
    // 用 ACL 拒绝 Delete 权限，复刻「这条链接摘不掉」（真实成因：句柄占用 / 权限）
    const user = `${process.env.USERDOMAIN}\\${process.env.USERNAME}`;
    execSync(`icacls "${link}" /deny "${user}:(D)"`, { stdio: 'ignore' });
    try {
      const r = runPrune(['-WtRoot', root, '-Execute']);

      expect(r.out).toContain('PRUNE_DETACH_FAIL wt-9001');
      expect(r.out).toContain('PRUNE_LINK_LEFT wt-9001');
      expect(r.code).toBe(3);
      // 承重：摘不掉就不许删 —— 目录与它里面的文件都还在（删一半的实现在这里变红）
      expect(fs.existsSync(path.join(root, 'wt-9001', 'marker.txt'))).toBe(true);
      // ⚠️ 不能用 existsSync 判这条链接：它跟随 junction，遇到带 deny-Delete ACL 的链接会 EPERM 返回 false
      //    （实测 Node 22；链接其实还在）。lstat 不跟随，才是「链接本身还在」的判据。
      expect(fs.lstatSync(link).isSymbolicLink()).toBe(true);
      expect(fs.existsSync(path.join(target, 'sentinel.txt'))).toBe(true);
    } finally {
      try { execSync(`icacls "${link}" /remove:d "${user}"`, { stdio: 'ignore' }); } catch (e) {}
    }
  });

  test('PR4 调用方 CWD 在待删目录内 ⇒ 拒绝执行（exit 5），目录原样留着', () => {
    const { root, linkedDir } = makeRoot('pr4');
    const r = runPrune(['-WtRoot', root, '-Execute'], { cwd: linkedDir });

    expect(r.code).toBe(5);
    expect(r.out).toContain('PRUNE_CWD_INSIDE');
    expect(fs.existsSync(linkedDir)).toBe(true);
    expect(fs.existsSync(path.join(root, 'wt-9002'))).toBe(true);
  });
});
