/**
 * 门评论「sha 绑定」口径的运行期守护（#1695，2026-10-16）
 *
 * 为什么需要（票面现测，#1690 run15/run16 实证）：② 门的归档链路是 `git add` + `git commit`（**仅本地**），
 *   随后贴出的证据评论把 `- commit: <sha>` 指向那个本地提交；`.github/workflows/pr-evidence.yml` 只核
 *   「sha 是不是 PR head 的祖先」，**核不出「远端可达」** ⇒ 分支一旦 rebase / force-push（过
 *   「head branch is not to date」门禁的常规动作），既往门评论里的 sha 在 GitHub 上就是 404（证据链悬空）。
 *   本票把口径收进 `scripts/lib/gate-commit.ps1`：**只绑 GitHub 点得开的提交**，不可达时如实标注。
 *
 * 本套件钉的是**那个口径本身的行为**（真执行 lib 的三个函数，不读源码文本、不联网）：
 *   GCB0 三个成员都在、探针跑完（fail-closed：pwsh 不可用或函数缺失 ⇒ 抛错，不 skip）
 *   GCB1 远端可达 ⇒ 绑 HEAD、**不加任何标注行**（证明 GCB2/GCB3 的标注不是「永远都加」）
 *   GCB2 HEAD 只是**本地归档提交** ⇒ 绑远端分支 tip（点得开）+ 如实标注「本地归档 sha（未推送）」
 *   GCB3 HEAD 里混着**非归档**改动 ⇒ **不许**借 tip 蒙混：绑 HEAD 且标注不可点开（fail-closed）
 *   GCB4 `Push-ArchiveCommit` 真推（推完远端可达）＋ 推不动时**如实回报**（Pushed=False + 原因非空）
 *   GCB5 取不到 HEAD sha ⇒ Mode=none，不抛异常（调用方那条 `-not $sha` 早退分支仍然有效）
 *
 * ⚠️ 判别力实测（本票收口会话真跑，改坏 ⇒ 哪条红）：
 *   `Resolve-CommitBinding` 去掉 `--relative`（monorepo 里 diff 路径多一层仓库根前缀）⇒ GCB2 红（B_MODE=local-head）
 *   去掉 `merge-base --is-ancestor $tip $HeadSha` ⇒ GCB3 红（把非归档改动也当成归档提交，绑了 tip）
 *   归档白名单从 `docs/verification/*` 放宽成 `*docs/verification*` 之外的任意 ⇒ GCB3 红（同上那一形态）
 *   `Push-ArchiveCommit` 把失败也当成功返回 ⇒ GCB4 红（PUSH_FAIL_NOT_PUSHED=False）
 *
 * ⚠️ 手法沿用本仓先例（`deviceCaptureBoundedShotBehavior.test.js`）：**真建仓库真跑 git**，
 *   在临时目录里造一个 bare 远端 + 一个 monorepo 形状的工作树（ProjectDir 是仓库**子目录**，
 *   这样 `--relative` 那条腿才有判别力），只 dot-source lib、不执行任何门脚本。
 *   判据 token **全 ASCII**：pwsh 的 stdout 在 Windows 上按 OEM 码页写，中文到 Node 侧会变乱码（本仓血账），
 *   所以「标注行里有没有『本地归档 sha（未推送）』」在 **PS 侧**判完只回带 True/False。
 */

const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB = path.join(ROOT, 'scripts', 'lib', 'gate-commit.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

/**
 * 一次 pwsh 会话跑完五腿：真建 bare 远端与 monorepo 形状工作树 → 逐形态调 lib → 真推 / 假远端推。
 * 全程离线（远端是本地路径），不碰 GitHub、不碰 PR。
 */
function probeBinding(ctx) {
  const script = `
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
. "${LIB}"
Write-Output ('LIB_FUNCS=' + @(Get-Command -Name Test-ShaReachable, Push-ArchiveCommit, Resolve-CommitBinding -CommandType Function).Count)

$dir = '${ctx.tmp}'
$bare = Join-Path $dir 'remote.git'
$repo = Join-Path $dir 'repo'
$proj = Join-Path $repo 'training-app\\app'
New-Item -ItemType Directory -Force -Path (Join-Path $proj 'src') | Out-Null

& git init -q --bare $bare | Out-Null
& git init -q $repo | Out-Null
& git -C $repo checkout -q -b main 2>$null | Out-Null
& git -C $repo remote add origin $bare | Out-Null
& git -C $repo config user.email 'probe@example.com'
& git -C $repo config user.name 'probe'
& git -C $repo config commit.gpgsign false

Set-Content -LiteralPath (Join-Path $proj 'src\\base.uts') -Value 'export const a = 1' -Encoding utf8
& git -C $repo add -A | Out-Null
& git -C $repo commit -q -m 'base' | Out-Null
$sha1 = (& git -C $repo rev-parse HEAD | Out-String).Trim()
& git -C $repo push -q origin HEAD:refs/heads/main | Out-Null
& git -C $repo fetch -q origin | Out-Null

# GCB1：远端可达 ⇒ 绑 HEAD、无标注
Write-Output ('SHA1_REACHABLE=' + (Test-ShaReachable -ProjectDir $proj -Sha $sha1))
$a = Resolve-CommitBinding -ProjectDir $proj -HeadSha $sha1 -Branch 'main'
Write-Output ('A_MODE=' + $a.Mode)
Write-Output ('A_SHA_EQ_SHA1=' + [bool]($a.Sha -eq $sha1))
Write-Output ('A_NOTES_EMPTY=' + [bool](@($a.NoteLines).Count -eq 0))

# GCB2：只多一个**本地**归档提交 ⇒ 绑远端 tip + 标注（不传 -Branch：走「自己认当前分支」那一支）
$archDir = Join-Path $proj 'docs\\verification\\m\\1'
New-Item -ItemType Directory -Force -Path $archDir | Out-Null
Set-Content -LiteralPath (Join-Path $archDir 'a.png') -Value 'png' -Encoding utf8
& git -C $repo add -A | Out-Null
& git -C $repo commit -q -m 'docs(verification): archive' | Out-Null
$sha2 = (& git -C $repo rev-parse HEAD | Out-String).Trim()
Write-Output ('SHA2_REACHABLE=' + (Test-ShaReachable -ProjectDir $proj -Sha $sha2))
$b = Resolve-CommitBinding -ProjectDir $proj -HeadSha $sha2
Write-Output ('B_MODE=' + $b.Mode)
Write-Output ('B_SHA_EQ_SHA1=' + [bool]($b.Sha -eq $sha1))
Write-Output ('B_REACHABLE=' + $b.Reachable)
Write-Output ('B_NOTE_LABEL=' + [bool]((@($b.NoteLines) -join ' ') -like '*本地归档 sha（未推送）*'))
# 成对取证的「必红」半边：朴素实现（无条件绑 HEAD）在同一形态下绑的 sha 在远端点不开 ——
# 判据能把「修好的」与「没修的」区分开（下面 GCB2 会断言这一行必须是 False）。
Write-Output ('NAIVE_B_SHA_REACHABLE=' + (Test-ShaReachable -ProjectDir $proj -Sha $sha2))

# GCB3：HEAD 里混着非归档改动 ⇒ 不许借 tip 蒙混
Set-Content -LiteralPath (Join-Path $proj 'src\\x.uts') -Value 'export const x = 2' -Encoding utf8
& git -C $repo add -A | Out-Null
& git -C $repo commit -q -m 'src change' | Out-Null
$sha3 = (& git -C $repo rev-parse HEAD | Out-String).Trim()
$c = Resolve-CommitBinding -ProjectDir $proj -HeadSha $sha3 -Branch 'main'
Write-Output ('C_MODE=' + $c.Mode)
Write-Output ('C_SHA_EQ_HEAD=' + [bool]($c.Sha -eq $sha3))
Write-Output ('C_REACHABLE=' + $c.Reachable)
Write-Output ('C_NOTE_LABEL=' + [bool]((@($c.NoteLines) -join ' ') -like '*本地归档 sha（未推送）*'))

# GCB4：真推（推完远端可达 ⇒ 回到「绑 HEAD 无标注」）＋ 推不动的降级
$p1 = Push-ArchiveCommit -ProjectDir $proj -Branch 'main'
Write-Output ('PUSH_OK=' + $p1.Pushed)
& git -C $repo fetch -q origin | Out-Null
Write-Output ('AFTER_PUSH_REACHABLE=' + (Test-ShaReachable -ProjectDir $proj -Sha $sha3))
$d = Resolve-CommitBinding -ProjectDir $proj -HeadSha $sha3 -Branch 'main'
Write-Output ('D_MODE=' + $d.Mode)
Write-Output ('D_NOTES_EMPTY=' + [bool](@($d.NoteLines).Count -eq 0))
$p2 = Push-ArchiveCommit -ProjectDir $proj -Branch 'main' -Remote 'no-such-remote'
Write-Output ('PUSH_FAIL_NOT_PUSHED=' + [bool](-not $p2.Pushed))
Write-Output ('PUSH_FAIL_REASON_NONEMPTY=' + [bool]($p2.Reason.ToString().Trim().Length -gt 0))

# GCB5：空 sha ⇒ none，不抛
$e = Resolve-CommitBinding -ProjectDir $proj -HeadSha ''
Write-Output ('E_MODE=' + $e.Mode)
Write-Output 'BIND_PROBE_DONE=1'
`.trim();

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 180000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

describe('门评论 sha 绑定口径（运行期，#1695）', () => {
  let ctx;
  let probe;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'gatebind-')) };
    probe = probeBinding(ctx);
    if (probe.ok) {
      [
        'LIB_FUNCS', 'BIND_PROBE_DONE',
        'SHA1_REACHABLE', 'A_MODE', 'A_SHA_EQ_SHA1', 'A_NOTES_EMPTY',
        'SHA2_REACHABLE', 'B_MODE', 'B_SHA_EQ_SHA1', 'B_REACHABLE', 'B_NOTE_LABEL', 'NAIVE_B_SHA_REACHABLE',
        'C_MODE', 'C_SHA_EQ_HEAD', 'C_REACHABLE', 'C_NOTE_LABEL',
        'PUSH_OK', 'AFTER_PUSH_REACHABLE', 'D_MODE', 'D_NOTES_EMPTY',
        'PUSH_FAIL_NOT_PUSHED', 'PUSH_FAIL_REASON_NONEMPTY', 'E_MODE',
      ].forEach((key) => { v[key] = field(probe.stdout, key); });
    }
  });

  afterAll(() => {
    if (ctx) {
      try {
        fs.rmSync(ctx.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红（先例同 deviceCaptureBoundedShotBehavior）
      }
    }
  });

  function requireProbe() {
    if (probe && probe.ok) return;
    throw new Error(
      'gate-commit 绑定口径探针失败（pwsh 不可用或 lib 抛错，fail-closed 不跳过）：\n'
        + `exit=${probe ? probe.status : 'n/a'}\nstdout=${probe ? probe.stdout : ''}\nstderr=${probe ? probe.stderr : ''}`
    );
  }

  // GCB0：三个成员都在、探针跑完（否则下面每条都是空跑）
  test('GCB0: lib 三个成员可加载，探针跑完', () => {
    requireProbe();
    expect(v.LIB_FUNCS).toBe('3');
    expect(v.BIND_PROBE_DONE).toBe('1');
  });

  // GCB1：远端可达 ⇒ 绑 HEAD、无标注（对照腿：证明标注不是「永远都加」）
  test('GCB1: 远端可达时绑 HEAD 且不加标注行', () => {
    requireProbe();
    expect(v.SHA1_REACHABLE).toBe('True');
    expect(v.A_MODE).toBe('remote-head');
    expect(v.A_SHA_EQ_SHA1).toBe('True');
    expect(v.A_NOTES_EMPTY).toBe('True');
  });

  // GCB2：只有本地归档提交 ⇒ 绑远端 tip（点得开）+ 如实标注
  test('GCB2: HEAD 只是本地归档提交时绑远端 tip 并如实标注未推送', () => {
    requireProbe();
    expect(v.SHA2_REACHABLE).toBe('False');
    expect(v.B_MODE).toBe('remote-tip');
    expect(v.B_SHA_EQ_SHA1).toBe('True'); // 绑的是远端 tip，不是那个点不开的本地 sha
    expect(v.B_REACHABLE).toBe('True');
    expect(v.B_NOTE_LABEL).toBe('True'); // 「本地归档 sha（未推送）」必须在正文里（PS 侧判，ASCII 回带）
    // 必红半边：朴素「无条件绑 HEAD」在同一形态下绑的 sha 远端点不开 ⇒ 本条有判别力，不是恒真断言
    expect(v.NAIVE_B_SHA_REACHABLE).toBe('False');
  });

  // GCB3：混进非归档改动 ⇒ fail-closed，绑 HEAD 且标注不可点开
  test('GCB3: 混着非归档改动时不许借 tip 蒙混（绑 HEAD + 标注不可点开）', () => {
    requireProbe();
    expect(v.C_MODE).toBe('local-head');
    expect(v.C_SHA_EQ_HEAD).toBe('True');
    expect(v.C_REACHABLE).toBe('False');
    expect(v.C_NOTE_LABEL).toBe('True');
  });

  // GCB4：真推 + 推不动如实回报
  test('GCB4: Push-ArchiveCommit 真推成功；推不动时如实回报（不假装成功）', () => {
    requireProbe();
    expect(v.PUSH_OK).toBe('True');
    expect(v.AFTER_PUSH_REACHABLE).toBe('True');
    expect(v.D_MODE).toBe('remote-head');
    expect(v.D_NOTES_EMPTY).toBe('True');
    expect(v.PUSH_FAIL_NOT_PUSHED).toBe('True');
    expect(v.PUSH_FAIL_REASON_NONEMPTY).toBe('True');
  });

  // GCB5：取不到 sha ⇒ none，不抛（调用方的早退分支仍有效）
  test('GCB5: 空 HEAD sha 走 none 出口，不抛异常', () => {
    requireProbe();
    expect(v.E_MODE).toBe('none');
  });
});
