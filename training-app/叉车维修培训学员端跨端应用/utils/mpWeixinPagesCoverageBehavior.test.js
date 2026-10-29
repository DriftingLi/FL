/**
 * ② 门 `-Pages auto` 的**覆盖判据**（`Resolve-MpWeixinDirectCoverage`）—— 真执行守护（#2101 落库）。
 *
 * 它守的是 #2099 盘点出的那个洞：`-Pages auto` 只认 `pages/` 下任意深度的 `.uvue`，所以改 `components/` /
 * `utils/*.uts` /
 * `App.uvue` / `pages.json` 的 PR 改动页清单为空 ⇒ ② 只跑入口页一张 = **覆盖为零的绿**。
 * 补法 = 清单为空退固定冒烟集，并在结果行如实报 `coverage=smoke:N`。
 *
 * 为什么是真执行而不是文本断言：三分支（none / smoke / changed）是**判据**，文本断言在
 * 「把 `elseif ($want.Count -eq 0)` 改成 `elseif ($false)`」「把 none 也退化成冒烟集」这两种改坏方式下
 * 都能照样绿（C20 教训）。这里拿真实 git 夹具仓跑真函数，坏实现当场翻红。
 *
 * 本守护同时钉住一条**不许动的语义**：git 判据不可用时写 `coverage=none` 且**不退化冒烟集**。
 * 「不可知」换成「我覆盖了冒烟集」= 把不知道包装成已覆盖，那正是要消灭的假绿形态。
 *
 * 夹具布局：仓根是 ASCII 临时目录，中文目录段放在**仓内相对路径**里（与本仓 `training-app/叉车…/pages/…`
 * 同构）——理由同 utils/mpWeixinGatePagesBehavior.test.js 头的「夹具布局铁律」（git 输出是仓根相对路径）。
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/mp-weixin-pages.ps1';
const GATE_REL = 'scripts/mp-weixin-check.ps1';
const SMOKE = ['pages/dashboard/dashboard', 'pages/profile/profile'];

function git(cwd, args) {
  return execFileSync('git', args, { cwd, encoding: 'utf8' });
}

/**
 * 真实 git 夹具仓，三个分支：
 *   master      基线（入口页 + 文档）
 *   feat-pages  改动两个 .uvue 页 ⇒ 期望 changed:2
 *   feat-docs   只改文档 ⇒ 清单空且 Ok=true ⇒ 期望 smoke:2（这是本票的**主场景**）
 */
function buildFixture() {
  const repo = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-coverage-'));
  const appDir = path.join(repo, 'training-app', '叉车维修培训学员端跨端应用');
  fs.mkdirSync(appDir, { recursive: true });
  git(repo, ['init', '-q', '-b', 'master']);
  git(repo, ['config', 'core.quotepath', 'false']);
  git(repo, ['config', 'user.email', 'guard@example.com']);
  git(repo, ['config', 'user.name', 'guard']);
  const w = (rel, text) => {
    const p = path.join(repo, rel);
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text);
  };
  w('training-app/叉车维修培训学员端跨端应用/pages/index/index.uvue', 'index\n');
  w('training-app/叉车维修培训学员端跨端应用/components/Foo.uvue', 'foo\n');
  w('docs/note.md', 'base\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'base']);
  git(repo, ['update-ref', 'refs/remotes/origin/master', 'HEAD']);

  git(repo, ['checkout', '-q', '-b', 'feat-pages']);
  w('training-app/叉车维修培训学员端跨端应用/pages/checkin/checkin.uvue', 'checkin\n');
  w('training-app/叉车维修培训学员端跨端应用/pages/mall/mall.uvue', 'mall\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'feat-pages']);

  git(repo, ['checkout', '-q', '-b', 'feat-docs', 'master']);
  // 只改 components（不是 pages/**/*.uvue）+ 文档 —— 正是「改组件」那个洞的形态
  w('training-app/叉车维修培训学员端跨端应用/components/Foo.uvue', 'foo-v2\n');
  w('docs/note.md', 'feat-docs\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'feat-docs']);

  return repo;
}

/**
 * 起一个 pwsh 探针：dot-source 判据库 → 真调两个函数 → 把四个场景的产物一次性回传。
 * 场景 = 三种真实 git 状态 + 一个「冒烟集为空」的边界（防 `smoke:0` 被伪造成 `smoke:2`）。
 */
function probe(repo) {
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const smokeLiteral = SMOKE.map((p) => `'${p}'`).join(', ');
  const script = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    `$pages = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/master...feat-pages'`,
    `$docs  = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/master...feat-docs'`,
    `$bad   = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/nonexistent...HEAD'`,
    `$rPages = Resolve-MpWeixinDirectCoverage -AutoPages $pages -SmokePages @(${smokeLiteral})`,
    `$rDocs  = Resolve-MpWeixinDirectCoverage -AutoPages $docs  -SmokePages @(${smokeLiteral})`,
    `$rBad   = Resolve-MpWeixinDirectCoverage -AutoPages $bad   -SmokePages @(${smokeLiteral})`,
    `$rNoSmoke = Resolve-MpWeixinDirectCoverage -AutoPages $docs -SmokePages @()`,
    '[ordered]@{',
    '  changed = $rPages; smoke = $rDocs; none = $rBad; emptySmoke = $rNoSmoke;',
    '  raw = [ordered]@{ pages = $pages; docs = $docs; bad = $bad }',
    '} | ConvertTo-Json -Depth 6 -Compress'
  ].join('\n');
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
    cwd: ROOT,
    encoding: 'utf8',
    timeout: 120000,
    windowsHide: true
  });
  return JSON.parse(out.trim().split(/\r?\n/).filter(Boolean).pop());
}

describe('② -Pages auto 覆盖判据（真执行：changed / smoke / none 三分支）', () => {
  let repo = null;
  let r = null;

  beforeAll(() => {
    repo = buildFixture();
    try {
      r = probe(repo);
    } finally {
      fs.rmSync(repo, { recursive: true, force: true, maxRetries: 3 });
    }
  }, 180000);

  it('接线：门脚本 dot-source 判据库、在 auto 分支取用、并把读数原样送进结果行', () => {
    const gate = fs.readFileSync(path.join(ROOT, GATE_REL), 'utf8');
    expect(gate).toContain(". (Join-Path $PSScriptRoot 'lib\\mp-weixin-pages.ps1')");
    expect(gate).toContain('Resolve-MpWeixinDirectCoverage -AutoPages $autoPages');
    expect(gate).toContain('$coverageField = "$($resolution.Coverage)"');
    // 冒烟集真源必须在门计划里（`smoke:2` 的 2 不是写死的，是这份清单的条数）
    expect(gate).toMatch(/smokePages\s*=\s*@\('pages\/dashboard\/dashboard',\s*'pages\/profile\/profile'\)/);
  });

  it('changed:N —— 改了 .uvue 页 ⇒ 跑改动页，读数取改动页条数（不混冒烟集）', () => {
    expect(r.raw.pages.Ok).toBe(true);
    expect(r.changed.Coverage).toBe('changed:2');
    expect(r.changed.Pages).toEqual(['pages/checkin/checkin', 'pages/mall/mall']);
    expect(r.changed.Degraded).toBe(false);
    // 混入冒烟集就是「多跑了不相关的页」，读的人会把 smoke 页当成被验证的页
    expect(r.changed.Pages).not.toContain('pages/dashboard/dashboard');
  });

  it('smoke:N —— 只改 components（清单空但 git 可用）⇒ 退固定冒烟集（本票主场景）', () => {
    expect(r.raw.docs.Ok).toBe(true);
    expect(r.raw.docs.Pages).toEqual([]);
    expect(r.smoke.Coverage).toBe('smoke:2');
    expect(r.smoke.Pages).toEqual(SMOKE);
    expect(r.smoke.Degraded).toBe(false);
  });

  it('none —— git 判据不可用 ⇒ 不退冒烟集（不可知不得包装成「已覆盖冒烟」）', () => {
    expect(r.raw.bad.Ok).toBe(false);
    expect(r.none.Coverage).toBe('none');
    expect(r.none.Pages).toEqual([]);
    expect(r.none.Degraded).toBe(true);
  });

  it('smoke:0 —— 冒烟集为空时如实报 0，不伪造 2', () => {
    expect(r.emptySmoke.Coverage).toBe('smoke:0');
    expect(r.emptySmoke.Pages).toEqual([]);
  });

  it('分离判据：Degraded 只在 none 场景为真（拿掉它，调用方的 [warn] 就不再触发）', () => {
    // 这条是上面 `none` 用例的补强：把 `Degraded=$true` 改成 `$false`（或让 none 也退冒烟集）时，
    // `Coverage` 可能被顺手改宽而仍然「看着合理」，但 Degraded 一翻就红 —— 调用方全靠它决定要不要 [warn] 说破。
    expect(r.none.Degraded).toBe(true);
    expect(r.smoke.Degraded).toBe(false);
    expect(r.changed.Degraded).toBe(false);
  });
});
