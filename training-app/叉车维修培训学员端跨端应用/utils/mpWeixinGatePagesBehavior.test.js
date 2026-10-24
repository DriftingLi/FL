/**
 * ② 门 `-Pages auto` 改动页清单判据 —— 真执行守护（#1853）。
 *
 * 根因与「编码层双解耦」（`-c core.quotepath=false` + ProcessStartInfo UTF-8 显式解码）的全文叙事
 * = scripts/lib/mp-weixin-pages.ps1 头注释（唯一全文，此处不重复）。本守护钉的是它的**真执行**：
 * 真实 git 夹具仓 + gb2312 反条件下真调 `Get-MpWeixinChangedUvuePages`。
 *
 * 夹具布局铁律：中文目录段必须在**仓内相对路径**里（镜像本仓 `training-app/叉车…/pages/…`）——
 * git diff 输出的是**仓根相对路径**，把中文段放在仓根路径上（夹具 v1 的教训，2026-10-24 实测）
 * 输出全 ASCII，编码层病灶根本不进判据面，必红对照恒绿 = 空转假守护。
 *
 * 为什么必须真执行（不测源码文本）：病灶在「git 字节输出 → pwsh 字符串」的**解码层**，
 * C 系列对 -DryRun 产物的形状断言根本测不到它 —— 只有真实 git 仓 + 真实控制台代码页反条件下
 * 的真跑能复现/钉住。一次 pwsh 探针里产出三段证据：
 *   ① 必不红（iff 性质）：修复后的判据在 gb2312 反条件下，diff 含 pages/**.uvue ⟺ 清单非空，
 *      且路由换算（去 .uvue、去 /pages/ 前缀、去重）逐字正确；git 失败分支（坏区间 / 非 git 目录）
 *      落 Ok=false + kebab-case reason，不假绿；
 *   ② 必红（对照）：事故机写法（`& git` 直捕 + 同一正则）在同一反条件下**找不到**改动页 ——
 *      证明本守护真能区分好坏实现，不是恒真断言（C20 教训的反面：判据被改坏时必红夹具当场翻红）；
 *   ③ 反条件自证：`adverseReal` = 探针先验证「naive 直捕确实丢了 /pages/」才允许②参与判定 ——
 *      构造不出 gb2312 反条件的机器上②不红但①③照跑（记录在案，不是静默跳过）；
 *      Windows（② 门的实际运行面）上 adverseReal 必须为真，守护不得空转。
 *
 * 对抗式自查（判据会不会放过坏实现）：断言的是**真实 git 仓上的函数产物**（清单内容 / Ok / Reason），
 * 不是源码片段；把「-c core.quotepath=false」或「StandardOutputEncoding=UTF8」任一摘掉，
 * ② 的对照（naive 直捕）就是那个坏实现本体，必红当场翻红。
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（判据面 + ③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/mp-weixin-pages.ps1';
const GATE_REL = 'scripts/mp-weixin-check.ps1';

function git(cwd, args) {
  return execFileSync('git', args, { cwd, encoding: 'utf8' });
}

/**
 * 真实 git 夹具仓：**仓根是 ASCII 临时目录**，中文目录段在仓内（`training-app/叉车…/pages/…`，
 * 与本仓布局同构 —— 见文件头「夹具布局铁律」），本地配置 core.quotepath=false（事故机前提），
 * origin/master 用远端跟踪引用伪造（免真 remote）。三个分支覆盖 iff 两向 + git 失败分支：
 *   master     基线：pages/index/index.uvue + docs/note.md
 *   feat-a     改动页两枚（checkin / dashboard）+ 文档改动 ⇒ 清单应非空且过滤掉 docs
 *   feat-docs  只改文档 ⇒ 清单应为空且 Ok=true（本来就没有，不是降级）
 */
function buildFixtureRepo() {
  const repo = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-pages-'));
  const appDir = path.join(repo, 'training-app', '叉车维修培训学员端跨端应用');
  fs.mkdirSync(appDir, { recursive: true });
  git(repo, ['init', '-q', '-b', 'master']);
  git(repo, ['config', 'core.quotepath', 'false']);
  git(repo, ['config', 'user.email', 'guard@example.com']);
  git(repo, ['config', 'user.name', 'guard']);

  const w = (rel, text) => {
    fs.mkdirSync(path.dirname(path.join(repo, rel)), { recursive: true });
    fs.writeFileSync(path.join(repo, rel), text);
  };
  w('training-app/叉车维修培训学员端跨端应用/pages/index/index.uvue', 'index\n');
  w('docs/note.md', 'base\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'base']);
  git(repo, ['update-ref', 'refs/remotes/origin/master', 'HEAD']);

  git(repo, ['checkout', '-q', '-b', 'feat-a']);
  w('training-app/叉车维修培训学员端跨端应用/pages/checkin/checkin.uvue', 'checkin\n');
  w('training-app/叉车维修培训学员端跨端应用/pages/dashboard/dashboard.uvue', 'dashboard\n');
  w('docs/note.md', 'feat-a\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'feat-a']);

  git(repo, ['checkout', '-q', '-b', 'feat-docs', 'master']);
  w('docs/note.md', 'feat-docs\n');
  git(repo, ['add', '-A']);
  git(repo, ['commit', '-qm', 'feat-docs']);

  return { repo };
}

/**
 * 起 pwsh 探针：把控制台代码页钉成 gb2312（#1845/#1853 事故机形态）→ dot-source 判据库 →
 * 真调函数（iff 两向 + 坏区间）+ 事故机写法对照（`& git` 直捕），回传机读 JSON。
 * 返回值键大小写 = lib 里的 PascalCase（ConvertTo-Json 原样保留）；顶层键本探针自定义、全小写。
 * 探针输出最终按控制台编码落管道（cp936）：JSON 结构全 ASCII 可解析；naiveSample 的乱码
 * 字节经 node 侧 utf8 解码成替换符，不破坏 JSON 结构（GBK 双字节不产生 ASCII 引号）。
 */
function probeLib(repo) {
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const script = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    // 反条件：gb2312（码页 936）。构造不出（理论上的非 Windows 形态）记 cp=-1，让断言侧如实分流。
    'try {',
    '  [Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)',
    '  $cp = [Console]::OutputEncoding.CodePage',
    '} catch { $cp = -1 }',
    `$hit = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/master...feat-a'`,
    `$miss = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/master...feat-docs'`,
    `$bad = Get-MpWeixinChangedUvuePages -Project '${repo}' -Range 'origin/nonexistent...HEAD'`,
    // 事故机写法本体：& git 直捕（DBCS 解码）+ 与门脚本旧实现同一正则 —— ② 必红对照的判据面
    `$naive = @(& git -C '${repo}' diff --name-only 'origin/master...feat-a' 2>$null | ForEach-Object { "$_" })`,
    "$naiveFound = @($naive | Where-Object { $_ -match '(^|/)pages/.+\\.uvue$' }).Count",
    "$naiveSlash = @($naive | Where-Object { $_ -match '/pages/' }).Count",
    '$adverseReal = ($cp -eq 936) -and ($naiveSlash -eq 0) -and ($naiveFound -eq 0)',
    "[ordered]@{ cp = $cp; hit = $hit; miss = $miss; bad = $bad; naiveFound = $naiveFound; naiveSlash = $naiveSlash; adverseReal = $adverseReal; naiveSample = @($naive)[0] } | ConvertTo-Json -Depth 6 -Compress"
  ].join('\n');
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
    cwd: ROOT,
    encoding: 'utf8',
    timeout: 120000,
    windowsHide: true
  });
  return JSON.parse(out.trim().split(/\r?\n/).filter(Boolean).pop());
}

describe('② -Pages auto 改动页清单（gb2312 反条件真执行 + 事故机写法必红对照）', () => {
  let fixture = null;
  let r = null;

  beforeAll(() => {
    fixture = buildFixtureRepo();
    try {
      r = probeLib(fixture.repo);
    } finally {
      fs.rmSync(fixture.repo, { recursive: true, force: true, maxRetries: 3 });
    }
  }, 180000);

  it('接线：门脚本 dot-source 判据库并在 auto 分支取用（真源不落地 = 守护白钉）', () => {
    const gate = fs.readFileSync(path.join(ROOT, GATE_REL), 'utf8');
    expect(gate).toContain(". (Join-Path $PSScriptRoot 'lib\\mp-weixin-pages.ps1')");
    expect(gate).toContain('Get-MpWeixinChangedUvuePages -Project $Project');
    // 判据区间随真源下沉进了 lib（门计划 autoSource 只留声明）——lib 侧的缺省区间也要钉住
    const lib = fs.readFileSync(path.join(ROOT, LIB_REL), 'utf8');
    expect(lib).toContain("$Range = 'origin/master...HEAD'");
  });

  it('必不红（iff 正向）：gb2312 反条件下含 .uvue 改动的分支产出正确路由清单（docs 改动被过滤）', () => {
    expect(r.hit.Ok).toBe(true);
    expect(r.hit.Reason).toBe('');
    expect(r.hit.Pages).toEqual(['pages/checkin/checkin', 'pages/dashboard/dashboard']);
  });

  it('必不红（iff 反向）：只有非 pages/*.uvue 改动 ⇒ 清单空且 Ok=true（本来就没有，不是降级）', () => {
    expect(r.miss.Ok).toBe(true);
    expect(r.miss.Pages).toEqual([]);
  });

  it('必不红（失败分支）：坏 diff 区间 ⇒ Ok=false + git-exit-<N>（可追溯，不冒充「无改动」）', () => {
    expect(r.bad.Ok).toBe(false);
    expect(r.bad.Reason).toMatch(/^git-exit-\d+$/);
  });

  it('必红（对照）：事故机写法（& git 直捕）在同一反条件下找不到改动页 —— 判据真能区分好坏实现', () => {
    // 反条件自证：Windows（② 门运行面）上 gb2312 必须构造得出来且确实吞了斜杠，守护不得空转
    if (process.platform === 'win32') {
      expect(r.cp).toBe(936);
      expect(r.adverseReal).toBe(true);
    }
    // 同一仓同一区间：修复后判据命中 2 页，事故机写法命中 0 页 —— 差异就是本守护要钉的病灶
    if (r.adverseReal) {
      expect(r.hit.Pages.length).toBe(2);
      expect(r.naiveFound).toBe(0);
    }
  });
});
