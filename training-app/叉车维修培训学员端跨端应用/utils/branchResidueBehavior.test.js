/**
 * `scripts/check-branch-residue.ps1` 的判据守护（#2021，2026-10-27）
 *
 * 为什么需要：这个脚本会 `-Execute` **真删远端分支**，却在 196 个守护套件里一个都没被覆盖
 * （`grep -rln "check-branch-residue" utils/*.test.js` 零命中）。同批次还修了两个判据缺陷：
 *
 *   1. **归档分支被永久报成「孤儿」**：取证产物按既定决定「勿合主干」只留在备份分支上（#1650 先例），
 *      这类分支**永远不会有 PR** ⇒ 只用 PR 状态分类，它每次都落进待裁定列表、人每次都答「保留」。
 *      新增 `ARCHIVED` 分类：head sha 被 `archive/*` 标签覆盖 ⇒ 刻意归档，不报、不删、不计退出码。
 *   2. **「真正的根治是上游设置」这句提示是错的**：本仓 `delete_branch_on_merge` 实测早已为 `true`，
 *      残留的真正成因是「从未走过 PR」—— auto-delete 只在 PR 合并这一个事件上触发。
 *      改为读真值打 `AUTODELETE=on|off|unknown`，三个分支文案各不相同。
 *
 * 手法（沿用 `utils/gateCommitBindingBehavior.test.js` 的先例）：`-GhCli` 注入临时桩，真起 pwsh
 * 真跑脚本，**不联网、不碰 GitHub、不删任何真分支**。桩是 `node` 脚本 + 一层 `.cmd`/`.sh` 壳：
 *   - Windows 壳 `%*` 会把带引号的 `--jq` 参数在空格处**拆开**（2026-10-27 微实验实测），
 *     所以桩**只按关键字路由**（`tags?per_page` / `branches/<name>` / `--head` …），不依赖参数完整形状；
 *   - Unix 壳 `"$@"` 原样传递 —— 两种形态下关键字都在，同一份路由两边都成立。
 *
 * ⚠️ 判据 token **全 ASCII**（`ARCHIVED` / `SUMMARY …` / `AUTODELETE=` / 分支名）：pwsh 的 stdout
 *   在 Windows 上按 OEM 码页写，中文到 Node 侧会变乱码（本仓血账，见 gateCommitBindingBehavior 头注释）。
 *
 * ⚠️ 判别力（改坏哪条 ⇒ 哪个用例红）：
 *   去掉 `$archiveTags` 命中分支 ⇒ BR0/BR1 红（arch-1 落回 NEED-HUMAN）；BR2 仍绿（它本来就该不命中被覆盖的分支）
 *   把 ARCHIVED 排在 MERGED 之后 ⇒ BR0 红（arch-1 无 PR 不受影响），若给 arch-1 加 MERGED PR 则直接看出优先级
 *   head sha 比对改成「标签存在即归档」⇒ BR2 红（head 前进后仍被判归档 = 该红的漏跑）
 *   取标签失败时返回空 map 而非 $null ⇒ BR3 红（会静默把 arch-1 判成「无归档」而不报警）
 *   归档分支也进 $toDelete ⇒ BR4 红（deleted.log 里出现 arch-1）
 *   提示文案写死「真正的根治是上游设置」⇒ BR5 红（读不到 AUTODELETE=off）
 */

const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'check-branch-residue.ps1');

// 真起 pwsh + 真跑脚本几次，默认 5s 不够（先例：utils/gateCommitBindingBehavior.test.js）
jest.setTimeout(240000);

function psExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

// sha 只需满足脚本的 `[0-9a-fA-F]{7,40}` 且彼此不同
const SHA = {
  open: 'a'.repeat(40),
  arch: 'b'.repeat(40),
  archNext: 'c'.repeat(40),
  merged: 'd'.repeat(40),
  orphan: 'e'.repeat(40),
};

const STUB_SRC = [
  "const fs = require('fs');",
  "const path = require('path');",
  'const SC = __SCENARIO__;',
  'const j = process.argv.slice(2).join(\' \');',
  'function emit(lines) {',
  "  var s = (lines || []).filter(function (x) { return x !== ''; }).join('\\n');",
  "  if (s) process.stdout.write(s + '\\n');",
  '}',
  "function delLog() { return path.join(__dirname, 'deleted.log'); }",
  'function deleted() {',
  "  try { return fs.readFileSync(delLog(), 'utf8').split('\\n').filter(Boolean); } catch (e) { return []; }",
  '}',
  "if (j.indexOf('--version') >= 0) { emit(['gh version 2.97.0 (stub)']); process.exit(0); }",
  "if (j.indexOf('repo view') >= 0) {",
  "  if (j.indexOf('nameWithOwner') >= 0) emit([SC.nwo]);",
  "  else if (j.indexOf('defaultBranchRef') >= 0) emit([SC.default]);",
  '  else emit([SC.nwo]);',
  '  process.exit(0);',
  '}',
  "if (j.indexOf('-X DELETE') >= 0) {",
  '  var dm = j.match(/refs\\/heads\\/(\\S+)/);',
  "  if (dm) fs.appendFileSync(delLog(), dm[1] + '\\n');",
  '  process.exit(0);',
  '}',
  "if (j.indexOf('branches?per_page') >= 0) {",
  "  if (SC.branches === 'fail') { process.stderr.write('gh: Server Error (HTTP 500)\\n'); process.exit(1); }",
  "  emit(SC.branches.map(function (b) { return b.sha + ' ' + b.name; }));",
  '  process.exit(0);',
  '}',
  "if (j.indexOf('tags?per_page') >= 0) {",
  "  if (SC.tags === 'fail') { process.stderr.write('gh: Server Error (HTTP 500)\\n'); process.exit(1); }",
  "  emit(SC.tags.map(function (t) { return t.sha + ' ' + t.name; }));",
  '  process.exit(0);',
  '}',
  "if (j.indexOf('delete_branch_on_merge') >= 0) {",
  "  if (SC.deleteBranchOnMerge === 'fail') { process.stderr.write('gh: Server Error (HTTP 500)\\n'); process.exit(1); }",
  '  emit([String(SC.deleteBranchOnMerge)]);',
  '  process.exit(0);',
  '}',
  'var bm = j.match(/\\/branches\\/(\\S+)/);',
  'if (bm) {',
  '  var name = bm[1];',
  "  if (deleted().indexOf(name) >= 0) { process.stderr.write('gh: Not Found (HTTP 404)\\n'); process.exit(1); }",
  '  var found = (SC.branches || []).filter(function (b) { return b.name === name; })[0];',
  '  if (found) {',
  '    process.stdout.write(JSON.stringify({ name: name, commit: { sha: found.sha } }) + \'\\n\');',
  '    process.exit(0);',
  '  }',
  "  process.stderr.write('gh: Not Found (HTTP 404)\\n'); process.exit(1);",
  '}',
  'var pm = j.match(/--head\\s+(\\S+)/);',
  'if (pm) {',
  '  var rows = (SC.prs || {})[pm[1]] || [];',
  "  emit(rows.map(function (r) { return r.number + ' ' + r.state + ' ' + (r.mergedAt || ''); }));",
  '  process.exit(0);',
  '}',
  'emit([]);',
  'process.exit(0);',
].join('\n');

/** 造一个桩目录，返回可执行壳的路径与目录（后者用来读 deleted.log）。 */
function makeStub(root, name, scenario) {
  const dir = path.join(root, name);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'stub.js'), STUB_SRC.replace('__SCENARIO__', JSON.stringify(scenario)), 'utf8');
  if (process.platform === 'win32') {
    const exe = path.join(dir, 'gh.cmd');
    fs.writeFileSync(exe, '@echo off\r\nnode "%~dp0stub.js" %*\r\nexit /b %errorlevel%\r\n', 'utf8');
    return { exe, dir };
  }
  const exe = path.join(dir, 'gh.sh');
  fs.writeFileSync(exe, '#!/bin/sh\nexec node "$(dirname "$0")/stub.js" "$@"\n', 'utf8');
  fs.chmodSync(exe, 0o755);
  return { exe, dir };
}

function runScript(stub, extra) {
  const args = ['-File', SCRIPT, '-GhCli', stub.exe].concat(extra || []);
  const r = spawnSync(psExe(), psArgs(args), { encoding: 'utf8', cwd: ROOT });
  return { code: r.status, out: String(r.stdout || '') + String(r.stderr || '') };
}

let TMP = null;
beforeAll(() => {
  TMP = fs.mkdtempSync(path.join(os.tmpdir(), 'residue-'));
});
afterAll(() => {
  if (TMP) fs.rmSync(TMP, { recursive: true, force: true });
});

/**
 * 基线场景：四条分支各代表一类（活跃 PR / 归档 / 已合并残留 / 无 PR 孤儿）。
 * 归档标签只覆盖 arch-1 的 head —— `orphan-1` 与它同为「无 PR」，用作对照：
 * 只有真的被标签覆盖才算归档，不能把「无 PR」一律当归档。
 */
function mixedScenario(over) {
  return Object.assign({
    nwo: 'DriftingLi/FL',
    default: 'master',
    deleteBranchOnMerge: true,
    branches: [
      { name: 'open-1', sha: SHA.open },
      { name: 'arch-1', sha: SHA.arch },
      { name: 'merged-1', sha: SHA.merged },
      { name: 'orphan-1', sha: SHA.orphan },
    ],
    tags: [{ sha: SHA.arch, name: 'archive/keep-1' }],
    prs: {
      'open-1': [{ number: 900, state: 'OPEN' }],
      'merged-1': [{ number: 901, state: 'MERGED', mergedAt: '2026-10-01T00:00:00Z' }],
      'arch-1': [],
      'orphan-1': [],
    },
  }, over || {});
}

describe('check-branch-residue：ARCHIVED 归档豁免（#2021）', () => {
  test('BR0 归档命中：arch-1 判 ARCHIVED，同为零孤儿的 orphan-1 仍待裁定（对照腿）', () => {
    const stub = makeStub(TMP, 'br0', mixedScenario());
    const r = runScript(stub);

    expect(r.code).toBe(1); // merged-1 是残留 ⇒ 判红（既有语义不变）
    expect(r.out).toMatch(/ARCHIVED\s+arch-1/);
    expect(r.out).toMatch(/NEED-HUMAN\s+orphan-1/);
    expect(r.out).not.toMatch(/NEED-HUMAN\s+arch-1/);
    expect(r.out).toContain('SUMMARY branches=4 mergedResidue=1 needHuman=1 openPR=1 archived=1');
    expect(r.out).toContain('AUTODELETE=on');
  });

  test('BR1 纯归档：无残留无待裁定 ⇒ exit 0，且不出现在待裁定列表', () => {
    const stub = makeStub(TMP, 'br1', mixedScenario({
      branches: [{ name: 'arch-1', sha: SHA.arch }],
      prs: { 'arch-1': [] },
    }));
    const r = runScript(stub);

    expect(r.code).toBe(0);
    expect(r.out).toMatch(/ARCHIVED\s+arch-1/);
    expect(r.out).toContain('archived=1');
    expect(r.out).toContain('needHuman=0');
    expect(r.out).not.toContain('NEED-HUMAN');
  });

  test('BR2 head 前进后豁免失效：标签指着旧 sha ⇒ 恢复待裁定（fail-closed 方向）', () => {
    const stub = makeStub(TMP, 'br2', mixedScenario({
      branches: [{ name: 'arch-1', sha: SHA.archNext }],
      prs: { 'arch-1': [] },
    }));
    const r = runScript(stub);

    expect(r.code).toBe(0); // 无 MERGED 残留；needHuman 只在 -IncludeClosed 时判红
    expect(r.out).toContain('archived=0');
    expect(r.out).toMatch(/NEED-HUMAN\s+arch-1/);
  });

  test('BR3 取不到归档标签 ⇒ 不认任何归档，并显式告警（不静默当成「无归档」）', () => {
    const stub = makeStub(TMP, 'br3', mixedScenario({
      branches: [{ name: 'arch-1', sha: SHA.arch }],
      tags: 'fail',
      prs: { 'arch-1': [] },
    }));
    const r = runScript(stub);

    expect(r.out).toContain('ARCHIVE-TAGS=unavailable');
    expect(r.out).toContain('archived=0');
    expect(r.out).toMatch(/NEED-HUMAN\s+arch-1/);
  });

  test('BR4 -Execute 只删 MERGED 残留：归档 / 孤儿 / 活跃 PR 一个都不碰', () => {
    const stub = makeStub(TMP, 'br4', mixedScenario());
    const r = runScript(stub, ['-Execute']);

    const logFile = path.join(stub.dir, 'deleted.log');
    const deleted = fs.existsSync(logFile) ? fs.readFileSync(logFile, 'utf8').split('\n').filter(Boolean) : [];

    expect(deleted).toContain('merged-1');
    expect(deleted).not.toContain('arch-1');
    expect(deleted).not.toContain('orphan-1');
    expect(deleted).not.toContain('open-1');
    expect(deleted).toHaveLength(1); // 多删即红
    expect(r.code).toBe(0); // 删完复核：桩对已删分支回 404 ⇒ 不该假红
    expect(r.out).toContain('archived=1');
  });

  test('BR5 上游设置已开启时说真话：读 false 才劝人去开（缺陷 2 的判别腿）', () => {
    const stub = makeStub(TMP, 'br5', mixedScenario({
      branches: [{ name: 'orphan-1', sha: SHA.orphan }],
      tags: [],
      prs: { 'orphan-1': [] },
      deleteBranchOnMerge: false,
    }));
    const r = runScript(stub);

    expect(r.code).toBe(0);
    expect(r.out).toContain('AUTODELETE=off');
    expect(r.out).not.toContain('AUTODELETE=on');
  });
});
