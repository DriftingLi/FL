/**
 * 自动截图模块契约守护（scripts/lib/auto-screenshot.ps1）
 *
 * 背景（2026-09-14）：
 *   · 第一版只截图不翻页（已由 #989 修：改用 `--pagePath` + SHA256 反假绿）。
 *   · 第二版仍**遍历 pages.json 全部页面**（本仓 50 页），而每页一次 `cli launch --pagePath` 要过一遍
 *     HBuilderX（编译 + 推送）⇒ 全量截图的成本不可接受。`device-capture.ps1` 早已把「逐页 launch 成本高」
 *     标为待裁定，本次裁定：**只截本次改动涉及的页面**（用户 2026-09-14 Q3）。
 *
 * 守护的不变量：
 *   S1  Invoke-AutoScreenshot 存在
 *   S2  返回对象含 Ok / Screenshots / Skipped / HashConflicts / Error
 *   S3  读取 pages.json 取页面清单
 *   S4  跳过的页面记在 Skipped
 *   S5  【文本层】用 HBuilderX CLI --pagePath 导航（不得回退到 am start 深链 —— 实测不生效）
 *   S6  用 SHA256 hash 做反假绿判据（连续相同 hash ⇒ 切页未生效）
 *   S7  -CliPath 参数存在
 *   S8  【Q3】-Pages 参数：允许显式指定要截的页面
 *   S9  【Q3】-ChangedOnly：从 git diff 推导改动页面
 *   S10 【Q3+结构层】推导出 0 页 ⇒ **明报「无改动页面」并返回**，绝不回退到全量截图
 *   S11 【Q3】-MaxPages 上限兜底
 *   S12 【Q3】遍历的是**目标页集合**，不是 pages.json 的全量清单
 *   S13 【2026-09-15】**陈旧截图判据**：文件时间戳必须晚于本次运行起点
 *       （否则上次运行的同名残留 PNG 会被当成本次证据）
 *   S14 【2026-09-15】**改动页推导的路径形状**：两条 git 调用都必须 `--relative`，输出必须过
 *       `ConvertFrom-GitQuotedPath`（否则仓库根相对 + quotepath 转义 ⇒ `^pages/` 恒失配 ⇒ 推导恒 0 页，
 *       步骤 6 必然报「无改动页面」⇒ HashConflicts / TargetPages 永远取不到真值）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/auto-screenshot.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('auto-screenshot.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('S1: Invoke-AutoScreenshot function exists', () => {
    expect(src).toContain('function Invoke-AutoScreenshot');
  });

  test('S2: returns Ok/Screenshots/Skipped/HashConflicts/StaleShots/Error', () => {
    ['Ok', 'Screenshots', 'Skipped', 'HashConflicts', 'StaleShots', 'Error'].forEach((f) => {
      expect(src).toContain(f);
    });
  });

  test('S3: reads pages.json for the page list', () => {
    expect(src).toContain('pages.json');
  });

  test('S4: skipped pages tracked', () => {
    expect(src).toContain('skipped');
  });

  test('S5: 【文本层】navigates via HBuilderX CLI --pagePath (not am start deep link)', () => {
    expect(src).toContain('--pagePath');
    // 必须是**真的调用** cli（`& $CliPath …`），只出现路径字符串会被注释满足
    expect(src).toMatch(/&\s*\$CliPath/);
  });

  test('S6: SHA256 anti-false-green (identical consecutive hashes ⇒ navigation failed)', () => {
    expect(src).toContain('SHA256');
    expect(src).toContain('seenHashes');
  });

  test('S7: -CliPath parameter exists', () => {
    expect(src).toContain('$CliPath');
  });

  test('S8: 【Q3】-Pages parameter (explicit page list)', () => {
    expect(src).toContain('$Pages');
    expect(src).toMatch(/Pages\s*-split|\[string\]\$Pages/);
  });

  test('S9: 【Q3】-ChangedOnly derives changed pages from git diff', () => {
    expect(src).toContain('ChangedOnly');
    expect(src).toMatch(/git[^\r\n]*diff/);
    expect(src).toMatch(/--name-only/);
  });

  test('S10: 【结构层】0 derived pages ⇒ explicitly reports and returns (never falls back to all)', () => {
    // 注意：该短语在**文档注释里也会出现**，故不能只看第一处 —— 必须检查「有一处在代码里紧邻 return」。
    const hits = [...src.matchAll(/无改动页面/g)].map((m) => m.index);
    expect(hits.length).toBeGreaterThan(0);
    const hasReturnNear = hits.some((i) =>
      /return\s+\[pscustomobject\]/.test(src.slice(Math.max(0, i - 500), i + 500))
    );
    expect(hasReturnNear).toBe(true);
  });

  test('S11: 【Q3】-MaxPages cap', () => {
    expect(src).toContain('MaxPages');
  });

  test('S12: 【Q3】iterates the target page set, not the full pages.json list', () => {
    expect(src).toContain('$targetPages');
    expect(src).toMatch(/foreach\s*\(\s*\$page\s+in\s+\$targetPages\s*\)/);
    // 不得再直接遍历全量 $pages
    expect(src).not.toMatch(/foreach\s*\(\s*\$page\s+in\s+\$pages\s*\)/);
  });

  // S13（2026-09-15）：截图**陈旧文件**判据。`$OutputDir` 不清理、文件名按页名固定
  // ⇒ adb 静默失败时上一次运行的同名残留 PNG 会让「存在且非空」（S4 那条）照样通过，
  //    把陈旧截图当本次证据。必须有一条「文件时间戳晚于本次运行起点」的判据。
  test('S13: rejects stale screenshots (file mtime must be newer than this run)', () => {
    // 必须有运行起点，且**在循环之前**取（循环内各取一次会让判据退化成恒真）
    const runStartAt = src.indexOf('$runStarted');
    const loopAt = src.indexOf('foreach ($page in $targetPages)');
    expect(runStartAt).toBeGreaterThan(-1);
    expect(loopAt).toBeGreaterThan(-1);
    expect(runStartAt).toBeLessThan(loopAt);
    // 必须有「时间戳早于起点 ⇒ 判陈旧」的比较与归类
    expect(src).toMatch(/\$shotWritten\s+-lt\s+\$runStarted/);
    expect(src).toMatch(/\$staleShots\s*\+=/);
    // 陈旧截图必须计入 Skipped（否则不会让 Ok=false ⇒ 仍会假绿）
    const staleAt = src.indexOf('$staleShots += $pageName');
    expect(staleAt).toBeGreaterThan(-1);
    expect(src.slice(staleAt, staleAt + 300)).toMatch(/\$skipped\s*\+=/);
  });

  // S14（2026-09-15）：S9 只断言「源码里出现了 git … diff」（文本层），**推导坏掉它照样绿**。
  //   实际症状：`git diff --name-only` 在本仓输出的是**仓库根相对**路径（本项目位于
  //   training-app/<中文目录>/ 之下），且非 ASCII 段被 core.quotepath 转义成引号 + \ooo；
  //   而下面的映射按 `^pages/` 锚定 ⇒ 一条也匹配不上 ⇒ 恒推导出 0 页 ⇒ dev:finish 步骤 6
  //   必然报「无改动页面」⇒ HashConflicts / TargetPages 永远取不到真值（长跑不出来的验证债）。
  //   本条是**回归钉**，钉死「必须 --relative + 必须解码 + 解码器来自唯一真源」三点，旧代码必红。
  test('S14: 【2026-09-15】derivation uses --relative and decodes git-quoted paths', () => {
    const from = src.indexOf('确定目标页集合');
    // ⚠️ 右边界必须**从 from 之后再找**：文件头说明里就有「…本仓 `pages.json` 有 50 页 ⇒…」，
    //    直接 indexOf 那个词会命中它 ⇒ 切片为空 ⇒ 整条守护退化成恒真的空断言（正是要防的弱守护）。
    const to = src.indexOf('3) 0 页', from);
    expect(from).toBeGreaterThan(-1);
    expect(to).toBeGreaterThan(from);
    const block = src.slice(from, to);

    // 反例：不得再出现**不带走相对**的那两种调用形态
    expect(block).not.toMatch(/'--name-only',\s*'HEAD'/);
    expect(block).not.toMatch(/'--name-only',\s*'origin\/master\.\.\.HEAD'/);
    expect(block).not.toMatch(/'--name-only',\s*'--cached'/);

    // 正例：两条调用都带 --relative（锚定 '^pages/' 只有在路径相对项目根时才可能命中）
    expect(block).toMatch(/'--name-only',\s*'--relative',\s*'HEAD'/);
    expect(block).toMatch(/'--name-only',\s*'--relative',\s*'origin\/master\.\.\.HEAD'/);

    // 正例：输出必须过解码器，且解码器来自唯一真源（不在此另抄一份实现）
    expect(block).toMatch(/ConvertFrom-GitQuotedPath/);
    expect(block).toMatch(/level-detect\.ps1/);
  });
});
