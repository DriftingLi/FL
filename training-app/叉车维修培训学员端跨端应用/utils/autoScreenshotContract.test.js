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

  test('S2: returns Ok/Screenshots/Skipped/HashConflicts/Error', () => {
    ['Ok', 'Screenshots', 'Skipped', 'HashConflicts', 'Error'].forEach((f) => {
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
});
