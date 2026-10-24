/**
 * ② 门「产物内容指纹 ⇒ 自动 -SkipBuild」行为守护（#1860）。
 *
 * 判据真源 = `scripts/lib/mp-weixin-build-fingerprint.ps1`（门脚本 dot-source 同一份）。
 * 病根：同树重跑时 publish 重建一模一样的产物（≈2–3 min/次）。「产物未变」机判化 ——
 * 指纹键三组件（输入集内容哈希 + 产物内容哈希 + HBuilderX 版本令牌）**全等才命中**；
 * 失配方向 **fail 向重建**（判不出 = 重建，绝不猜命中 —— 调研 §1.2 条 2 / Turborepo
 * 「cache key = 输入哈希：同输入必命中、不同输入必不命中」）。
 *
 * 为什么判据是**真执行库**而不是断言门脚本源码文本：文本断言在「判据被摘掉、只留一句字面量」时
 * 照样绿（C20 教训）。这里用**夹具项目 + 夹具产物**真改真比对钉住语义：
 *   - 必不红：save 后什么都不变 ⇒ 命中；排除面文件（docs/scripts/*.test.js）新增 ⇒ 仍命中；
 *   - 必红：源码变 / 产物变 / 版本变 / 指纹缺失 / 指纹损坏 ⇒ 各自的 kebab reason，绝不命中；
 *   - 落盘方向：版本令牌算不出 ⇒ 不写（下次必然重建）。
 *
 * 对抗式自查：把判据改坏（摘掉 inputs 组件 / 摘掉 product 组件 / 命中方向反转）时，必红夹具当场翻红。
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/mp-weixin-build-fingerprint.ps1';
const VERSION = '5.23.2026080626|cli=deadbeef';

/** 夹具项目：源（manifest/pages/main.uts）+ 产物（app.json/app.js/页 js）。 */
function makeProject() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-fp-project-'));
  const w = (p, text) => {
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text, 'utf8');
  };
  w(path.join(dir, 'manifest.json'), JSON.stringify({ name: 'fp-fixture', 'mp-weixin': { appid: 'wxfp' } }));
  w(path.join(dir, 'pages.json'), JSON.stringify({ pages: [{ path: 'pages/index/index' }] }));
  w(path.join(dir, 'main.uts'), 'import App from "./App.uvue"\n');
  w(path.join(dir, 'App.uvue'), '<template><text>fp</text></template>\n');
  w(path.join(dir, 'pages', 'index', 'index.uvue'), '<template><text>fp</text></template>\n');
  const dist = path.join(dir, 'unpackage', 'dist', 'build', 'mp-weixin');
  w(path.join(dist, 'app.json'), JSON.stringify({ pages: ['pages/index/index'] }));
  w(path.join(dist, 'app.js'), '// fp fixture\n');
  w(path.join(dist, 'pages', 'index', 'index.js'), '// fp page\n');
  return { dir, dist };
}

/** 起 pwsh：dot-source 判据库 → 按传入语句序列执行 → 每条语句一个键回传机读 JSON。 */
function runLib(steps, io) {
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const handoff = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'mp-fp-io-')), 'io.json');
  fs.writeFileSync(handoff, JSON.stringify(io), 'utf8');
  const script = [
    "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)",
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    `$io = (Get-Content -LiteralPath '${handoff.replace(/\\/g, '\\\\')}' -Raw | ConvertFrom-Json)`,
    '$out = [ordered]@{}',
    ...steps,
    '$out | ConvertTo-Json -Depth 6 -Compress'
  ].join('\r\n');
  try {
    return execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
      encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 4 * 1024 * 1024
    });
  } finally {
    fs.rmSync(path.dirname(handoff), { recursive: true, force: true });
  }
}

function parseOut(stdout) {
  const line = String(stdout).split(/\r?\n/).filter((l) => l.trim().startsWith('{')).pop();
  if (!line) throw new Error(`指纹库无可解析 JSON 输出：\n${stdout}`);
  return JSON.parse(line);
}

/** 常用语句片段：save / hit（键名由调用方给，避免同名覆盖）。 */
const save = (key, v = '$io.version') =>
  `$out.${key} = (Save-MpWeixinBuildFingerprint -Project $io.project -Dist $io.dist -HBuilderXVersion ${v})`;
const hit = (key, v = '$io.version') =>
  `$out.${key} = (Test-MpWeixinBuildFingerprintHit -Project $io.project -Dist $io.dist -HBuilderXVersion ${v})`;
const touch = (key, rel, text) =>
  `$out.${key} = $null; Set-Content -LiteralPath (Join-Path $io.project ${JSON.stringify(rel)}) -Value ${JSON.stringify(text)} -Encoding utf8NoBOM`;
const touchDist = (key, rel, text) =>
  `$out.${key} = $null; Set-Content -LiteralPath (Join-Path $io.dist ${JSON.stringify(rel)}) -Value ${JSON.stringify(text)} -Encoding utf8NoBOM`;

describe('② 产物内容指纹行为（#1860：三组件全等才命中，fail 向重建）', () => {
  it('必不红：save 后什么都不变 ⇒ 命中；排除面文件新增 ⇒ 仍命中', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saved'),
        hit('hit1'),
        // 排除面（fail-safe 清单）：docs / scripts / jest 守护都**不是**构建输入，新增不得翻脸
        `$out.mk = $null; New-Item -ItemType Directory -Force -Path (Join-Path $io.project 'docs'), (Join-Path $io.project 'scripts'), (Join-Path $io.project 'utils') | Out-Null`,
        touch('mkDocs', 'docs/note.md', '# note\n'),
        touch('mkScript', 'scripts/tool.ps1', 'Write-Host hi\n'),
        touch('mkTest', 'utils/x.test.js', 'test()\n'),
        hit('hit2')
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.saved.Saved).toBe(true);
      expect(out.hit1.Hit).toBe(true);
      expect(out.hit1.Reason).toBe('fingerprint-hit');
      expect(out.hit2.Hit).toBe(true);
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('必红：源码变 ⇒ inputs-changed（不拿旧产物出绿）', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saved'),
        touch('ignored', 'pages/index/index.uvue', '<template><text>CHANGED</text></template>\n'),
        hit('hit')
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.saved.Saved).toBe(true);
      expect(out.hit.Hit).toBe(false);
      expect(out.hit.Reason).toBe('inputs-changed');
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('必红：产物被改 ⇒ product-changed（「产物变化 ⇒ 必然重建」）', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saved'),
        touchDist('ignored', 'app.js', '// tampered\n'),
        hit('hit')
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.hit.Hit).toBe(false);
      expect(out.hit.Reason).toBe('product-changed');
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('必红：HBuilderX 版本变 ⇒ hbuilderx-version-changed（坑位 6b：工具版本会翻结论）', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saved'),
        hit('hit', "'5.24.9999999|cli=deadbeef'")
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.saved.Saved).toBe(true);
      expect(out.hit.Hit).toBe(false);
      expect(out.hit.Reason).toBe('hbuilderx-version-changed');
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('必红：指纹文件缺失 / 损坏 ⇒ 各自 reason（fail 向重建，绝不猜命中）', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saved'),
        `$out.rm = $null; Remove-Item -LiteralPath (Get-MpWeixinBuildFingerprintPath -Dist $io.dist) -Force`,
        hit('hitMissing'),
        `$out.corrupt = $null; Set-Content -LiteralPath (Get-MpWeixinBuildFingerprintPath -Dist $io.dist) -Value 'not json {' -Encoding utf8NoBOM`,
        hit('hitCorrupt'),
        hit('hitEmptyVersion', "''")
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.saved.Saved).toBe(true);
      expect(out.hitMissing.Hit).toBe(false);
      expect(out.hitMissing.Reason).toBe('no-fingerprint-file');
      expect(out.hitCorrupt.Hit).toBe(false);
      expect(out.hitCorrupt.Reason).toBe('fingerprint-file-corrupt');
      expect(out.hitEmptyVersion.Hit).toBe(false);
      expect(out.hitEmptyVersion.Reason).toBe('hbuilderx-version-unavailable');
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('落盘方向：版本令牌算不出 ⇒ 不写指纹（下次必然重建）；重建后重 save ⇒ 重新命中', () => {
    const f = makeProject();
    try {
      const out = parseOut(runLib([
        save('saveNoVersion', "''"),
        touchDist('ignored', 'app.js', '// rebuilt\n'),
        save('saved'),
        hit('hit')
      ], { project: f.dir, dist: f.dist, version: VERSION }));
      expect(out.saveNoVersion.Saved).toBe(false);
      expect(out.saveNoVersion.Reason).toBe('hbuilderx-version-unavailable');
      expect(out.saved.Saved).toBe(true);
      expect(out.hit.Hit).toBe(true);
    } finally {
      fs.rmSync(f.dir, { recursive: true, force: true });
    }
  });

  it('版本令牌：目录名版本号解析 + cli.exe 内容哈希；cli.exe 缺失 ⇒ $null（fail 向重建）', () => {
    const hbx = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-fp-hbx-'));
    const root = path.join(hbx, 'HBuilderX.9.99.1');
    fs.mkdirSync(root, { recursive: true });
    fs.writeFileSync(path.join(root, 'cli.exe'), 'MZ fake cli bytes', 'utf8');
    try {
      const out = parseOut(runLib([
        `$out.tok = (Get-MpWeixinHBuilderXVersionToken -HbxRoot $io.hbxRoot -CliExe (Join-Path $io.hbxRoot 'cli.exe'))`,
        `$out.noCli = (Get-MpWeixinHBuilderXVersionToken -HbxRoot $io.hbxRoot -CliExe '')`
      ], { hbxRoot: root }));
      expect(out.tok).toMatch(/^ver=9\.99\.1\|cli=[0-9a-f]{64}$/);
      expect(out.noCli).toBe(null);
    } finally {
      fs.rmSync(hbx, { recursive: true, force: true });
    }
  });
});
