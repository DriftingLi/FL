/**
 * 级别判定模块契约守护（scripts/lib/level-detect.ps1）
 *
 * 守护的不变量：
 *   L1 Get-DetectLevel 函数存在且可被 dot-source
 *   L2 返回对象包含 Level / Reason / ChangedFiles 三个字段
 *   L3 quick 判定 + `Reason` 的三类分流（2026-09-16 修订：`[.uvue]` 分支不得声称「未命中运行时面」）
 *   L4 有 .uts 改动 → Level=standard
 *   L5 有 manifest.json 改动 → Level=full
 *   L6 有新增 .uvue 页面 → Level=full
 *   L7 -ForceLevel 参数覆盖自动判定
 *   L8 空 diff → quick
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/level-detect.ps1';

function readSource() {
  return readText(path.join(ROOT, SCRIPT_REL));
}

describe('level-detect.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  // L1: 函数存在
  test('L1: Get-DetectLevel function exists', () => {
    expect(src).toContain('function Get-DetectLevel');
  });

  // L2: 返回对象结构
  test('L2: returns object with Level, Reason, ChangedFiles', () => {
    expect(src).toContain('Level');
    expect(src).toContain('Reason');
    expect(src).toContain('ChangedFiles');
  });

  // L3: quick 判定（**本地**口径）—— `Reason` 必须分三类写清，且不得混淆两个口径
  // 2026-09-15 修订：改 `.ps1` 被报成样式改动 ⇒ 点名「工具链/测试改动」。
  // 2026-09-16 修订（#1037）：只改既有 `.uvue` 被报成「未命中运行时面」，而 `.uvue` 在验收门口径
  //   （`pr-evidence` 的 `isRuntimeFile`）里就是运行时面 ⇒ 该分支必须写出「验收门口径 / ①③④」。
  //   ⚠️ 本用例是**文本**断言（会被注释满足）；分支真的走哪条由 levelDetectBehavior.test.js 的
  //   G4–G6 用临时仓库真跑一遍来钉 —— 这是 ADR-0011 G1–G3 的教训：文本断言证明不了分支可执行。
  test('L3: quick 判定 + Reason 三类分流（工具链 / .uvue / 其余）', () => {
    expect(src).toContain("Level        = 'quick'");
    // 空 diff 仍走 quick（独立措辞）
    expect(src).toContain("Reason       = '无改动文件");
    // 分支一：含 .uvue ⇒ 写出「门口径 + ①③④」，不得把它当免证据
    expect(src).toMatch(/模板\/样式改动[\s\S]{0,400}?验收门口径[\s\S]{0,200}?①③④/);
    // 分支二：只有工具链/测试/文档类文件 ⇒ 必须点名「工具链」
    expect(src).toMatch(/工具链\/测试改动，未命中运行时面/);
    // 分支三：其余 ⇒ 「非运行时面改动」+「未命中运行时面」
    expect(src).toMatch(/非运行时面改动，未命中运行时面/);
    // 三个分支都必须存在（防止有人删掉某支、又退回单一文案）
    expect(src).toContain('$toolchainFiles');
    expect(src).toContain('$uvueFiles');
    // 🟢 收尾建议句的单点真源（行为断言见 levelDetectBehavior.test.js G7）
    expect(src).toContain('function Get-QuickEvidenceHint');
  });

  // L4: standard 判定（.uts 变更）
  test('L4: .uts changes → standard', () => {
    expect(src).toContain("Level        = 'standard'");
    expect(src).toContain('.uts');
  });

  // L5: full 判定（配置文件变更）
  test('L5: manifest.json / pages.json changes → full', () => {
    expect(src).toContain("Level        = 'full'");
    expect(src).toContain('manifest\\.json');
    expect(src).toContain('pages\\.json');
    expect(src).toContain('platformConfig\\.json');
  });

  // L6: 新增 .uvue 页面 → full
  test('L6: new .uvue page → full', () => {
    expect(src).toContain('新增页面文件');
    expect(src).toContain('\\.uvue');
  });

  // L7: -ForceLevel 参数
  test('L7: -ForceLevel overrides auto detection', () => {
    expect(src).toContain('ForceLevel');
    expect(src).toContain("强制指定 -Level");
  });

  // L8: 空 diff → quick
  test('L8: empty diff → quick', () => {
    expect(src).toContain('无改动文件（空 diff）');
  });
});

// ── L9–L13：git 路径输出收口（#2874）────────────────────────────────────────
// 票面写的「37 处」是**纯文本**匹配量到的注释/说明文案行数：逐行看，37 行全是散文、零真实调用
// （本仓真实调用一律写成参数数组，字面量 `git diff` 只出现在注释里）。下表 23 处 = 剥注释后实测，
// 其中 1 处是 `Write-Host` 里的示例文案 ⇒ 22 处真实候选。
// verdict 含义：
//   quotepath-off           调用时关掉 core.quotepath，输出不再转义
//   decoded                 路径过唯一解码器 ConvertFrom-GitQuotedPath（真源 scripts/lib/level-detect.ps1）
//   allow-count             只判「非空 / 条数」，行内容从不被当路径用
//   allow-quotepath-immune  该命令实测不吃 core.quotepath（`worktree list --porcelain`，2026-11-26 实测）
//   allow-narrative         打印文案里的示例命令，不是真调用
const GIT_PATH_REGISTRY = [
  { file: 'scripts/dev-finish.ps1', kind: 'quotepath-off', sites: 3, evidence: 'core.quotepath=false',
    note: '1344/1489/1577 三处脏树关联，argv 内联 -c core.quotepath=false；结果只判脏/净，不作路径锚定' },
  { file: 'scripts/device-capture.ps1', kind: 'allow-count', sites: 1, evidence: 'if ($status) {',
    note: '863 只判「工作树是否干净」——空/非空，文件名一律不取' },
  { file: 'scripts/emulator-smoke.ps1', kind: 'allow-count', sites: 1, evidence: 'if ($status) {',
    note: '673 同上，只判空/非空' },
  { file: 'scripts/mp-weixin-check.ps1', kind: 'allow-count', sites: 1, evidence: 'status --porcelain -- $p',
    note: '398 逐路径单独问、只读 XY 两列，注释里写明「不解析路径列」' },
  { file: 'scripts/mp-weixin-check.ps1', kind: 'allow-narrative', sites: 1,
    evidence: '--porcelain -- manifest.json pages.json',
    note: '445 是 Write-Host 打印的示例命令行，不是调用' },
  { file: 'scripts/remove-worktree.ps1', kind: 'allow-quotepath-immune', sites: 2, evidence: 'worktree list --porcelain',
    note: '301/452 解析 worktree list --porcelain；实测该输出不吃 core.quotepath（中文路径原样）' },
  { file: 'scripts/remove-worktree.ps1', kind: 'allow-count', sites: 1, evidence: '.Count -gt 0',
    note: '317 只判脏（.Count -gt 0）' },
  { file: 'scripts/style-loop.ps1', kind: 'allow-count', sites: 1, evidence: '$dirtyCount',
    note: '464 只判脏树计数，不作路径' },
  { file: 'scripts/lib/auto-screenshot.ps1', kind: 'decoded', sites: 1, evidence: 'ConvertFrom-GitQuotedPath',
    note: '1270 两条 --name-only 都带 --relative，输出过解码器（S14 另有断言）' },
  { file: 'scripts/lib/behavior-subset.ps1', kind: 'quotepath-off', sites: 2, evidence: 'core.quotepath=false',
    note: '54/55 的 diff --name-only 与 ls-files --others 走同一条 argv（57 行加 -c core.quotepath=false）' },
  { file: 'scripts/lib/gate-commit.ps1', kind: 'quotepath-off', sites: 1, evidence: 'core.quotepath=false',
    note: '#2874 补的：141 判 docs/verification/ 归档-only，此前缺解码 ⇒ 归档名含中文时恒判非归档' },
  { file: 'scripts/lib/hx-noop.ps1', kind: 'allow-count', sites: 1, evidence: '$statusOk = ($LASTEXITCODE -eq 0)',
    note: '100 只判「git status 起不起得来」+ 脏/净，不取文件名' },
  { file: 'scripts/lib/level-detect.ps1', kind: 'decoded', sites: 3, evidence: 'ConvertFrom-GitQuotedPath',
    note: '99/119/174 三处改动集，输出全过 ConvertFrom-GitQuotedPath（本仓唯一真源）；不做 ^pages/ 锚定故无 --relative' },
  { file: 'scripts/lib/mp-weixin-pages.ps1', kind: 'quotepath-off', sites: 1, evidence: 'core.quotepath=false',
    note: '52 既有正确先例：ProcessStartInfo 起 git 时带 -c core.quotepath=false' },
  { file: 'scripts/lib/style-loop.ps1', kind: 'decoded', sites: 2, evidence: 'ConvertFrom-GitQuotedPath',
    note: '94/109 带 --relative 且输出过解码器' },
  { file: 'scripts/lib/wt-orphans.ps1', kind: 'allow-quotepath-immune', sites: 1, evidence: 'worktree list --porcelain',
    note: '57 解析 worktree list --porcelain，实测不吃 core.quotepath' },
];

describe('git 路径输出收口（#2874：裸调用守卫 + 允许列表）', () => {
  const { auditGitPathOutputs, scanGitPathOutputs, stripPsComments } = require('./ps1GitPathScan');

  test('L9: 现树每个路径输出调用点都有归属（实测 23 处 / 22 处真调用 + 1 处文案）', () => {
    const res = auditGitPathOutputs({ root: ROOT, registry: GIT_PATH_REGISTRY });
    // 失败信息必须自解释：把每条 violation 原样打出来，别只留一句 toBe(true)
    expect(res.violations).toEqual([]);
    expect(res.ok).toBe(true);
    expect(res.hits).toHaveLength(23);
    expect(res.hits.filter((h) => h.kind === 'call')).toHaveLength(22);
    expect(res.hits.filter((h) => h.kind === 'narrative')).toHaveLength(1);
  });

  // 🩸 红夹具：只断言「现树是绿的」等于没测 —— 必须证明这套判据对一个**真裸调用**会报红。
  //    夹具放在临时目录里（形状与真树相同：scripts/*.ps1），扫描器看到的是一份真实的裸调用。
  test('L10: 红夹具 —— 新裸调用（git diff --name-only 直接用输出）必须报红', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl2874-bare-'));
    try {
      fs.mkdirSync(path.join(dir, 'scripts'), { recursive: true });
      fs.writeFileSync(
        path.join(dir, 'scripts', 'bare.ps1'),
        '$names = (& git -C $ProjectDir diff --name-only HEAD 2>$null | Out-String)\n',
        'utf8',
      );
      const res = auditGitPathOutputs({ root: dir, registry: [] });
      expect(res.ok).toBe(false);
      expect(res.violations.join('\n')).toMatch(/scripts\/bare\.ps1: 扫描到 1 处/);
      // 反过来：同一夹具若没人登记就永远不许绿
      expect(res.violations.join('\n')).toMatch(/归属表里没有它/);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  // 归属不能只是一张「写着写着就与代码脱钩」的纸条：verdict 的依据串必须真在文件里。
  test('L11: 假归属不能过 —— 声称关了 quotepath 但代码里没有，必须报红', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl2874-fake-'));
    try {
      fs.mkdirSync(path.join(dir, 'scripts'), { recursive: true });
      fs.writeFileSync(
        path.join(dir, 'scripts', 'fake.ps1'),
        '$names = (& git diff --name-only HEAD 2>$null)\n',
        'utf8',
      );
      const res = auditGitPathOutputs({
        root: dir,
        registry: [{
          file: 'scripts/fake.ps1', kind: 'quotepath-off', sites: 1,
          evidence: 'core.quotepath=false', note: '假装关了转义',
        }],
      });
      expect(res.ok).toBe(false);
      expect(res.violations.join('\n')).toMatch(/依据「core\.quotepath=false」在该文件里找不到/);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  // 票面那条 Select-String 之所以量出 37：它没剥注释。这条钉住「剥注释」本身不退化成恒真。
  test('L12: 注释与说明文案不算调用（剥注释这条判据自身的守卫）', () => {
    const sample = [
      '<# 旧写法：& git diff --name-only HEAD  # 一整段块注释 #>',
      '# git status --porcelain 也不行',
      '$out = @(& git diff --name-only --relative HEAD)  # 行尾注释里的 --porcelain 不算',
      'Write-Host "示例：git status --porcelain"',
    ].join('\n');
    const stripped = stripPsComments(sample);
    expect(stripped).not.toContain('旧写法');
    expect(stripped).not.toContain('也不行');
    expect(stripped).not.toContain('行尾注释里的');
    // 字符串内容必须留下（'--name-only' 这类 token 就在引号里）
    expect(stripped).toContain('--name-only --relative HEAD');
    expect(stripped).toContain('示例：git status --porcelain');
    // 真调用 1 处 + 文案 1 处（文案被识别成 narrative，不会被当调用点）
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl2874-strip-'));
    try {
      fs.mkdirSync(path.join(dir, 'scripts'), { recursive: true });
      fs.writeFileSync(path.join(dir, 'scripts', 'sample.ps1'), sample + '\n', 'utf8');
      const hits = scanGitPathOutputs(dir);
      expect(hits).toHaveLength(2);
      expect(hits.filter((h) => h.kind === 'call')).toHaveLength(1);
      expect(hits.filter((h) => h.kind === 'narrative')).toHaveLength(1);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  // 本票实际改的那一处：判据源里必须能看到「关转义」与「--relative」同时在场。
  test('L13: gate-commit 的归档判定调用已关 quotepath（#2874 的落点）', () => {
    const gate = readText(path.join(ROOT, 'scripts/lib/gate-commit.ps1'));
    expect(gate).toMatch(/-c core\.quotepath=false[^\r\n]*diff --name-only --relative/);
  });
});

