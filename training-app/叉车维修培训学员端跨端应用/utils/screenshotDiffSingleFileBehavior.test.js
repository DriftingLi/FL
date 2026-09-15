/**
 * `Compare-ScreenshotBaseline` 的**单文件 / 严格模式**行为守护（运行期）—— 2026-09-15，#1027 收尾真机实测
 *
 * 缺陷（真机实测，血账）：`$currentFiles = Get-ChildItem -Path $CurrentDir -Filter '*.png' -File`
 *   只命中**一个**文件时会退化成**标量**（PS 的管道展开），而调用方 `dev-finish.ps1` 是以
 *   `Set-StrictMode -Version Latest` 跑的 ⇒ 标量取 `.Count` 抛「在此对象上找不到属性 Count」
 *   ⇒ **步骤 7（截图对比）直接崩、步骤 8–9 跑不到**。触发面恰好是最常见的形态：
 *   **基线目录不存在（首次运行）+ 恰好一页改动**（本次真机跑就是 1 页 dashboard）。
 *
 * 为什么必须是**运行期**断言：这个缺陷完全不影响源码文本形态（`@()` 加不加都是几行），
 *   任何 `expect(src).toContain(...)` 都抓不到 —— 只有真在 StrictMode 下跑一次单文件场景才看得见
 *   （而且 0 个文件与 ≥2 个文件都**不会**触发，所以文本/计数类守护天然漏掉它）。
 *
 * 断言（真跑函数，全程 `Set-StrictMode -Version Latest`，复刻 dev:finish 的真实条件）：
 *   C1 恰一个当前截图 + 基线目录不存在 ⇒ 不抛错、`ChangedCount=1`、`NewBaseline=$true`、Diff 一项且判「新增」
 *   C2 两个当前截图 ⇒ `ChangedCount=2`、`NewBaseline=$true`（≥2 时本来就不会退化成标量，回归钉）
 *   C3 零个当前截图 ⇒ `ChangedCount=0` 且 `Error` 非空（明报无可对比），同样不抛错
 *   C4 基线已存在且内容相同 ⇒ `ChangedCount=0`、`NewBaseline=$false`（正常对比路径未被破坏）
 *
 * 不需要设备、不需要 System.Drawing（只算 MD5 与文件枚举）⇒ Windows 与 Linux CI 都能跑。
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（仓库先例：
 *   `hxLaunchDetachBehavior.test.js` / `contractTestPatternBehavior.test.js`）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCREEN_DIFF = path.join(ROOT, 'scripts', 'lib', 'screenshot-diff.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function psQuote(s) {
  return String(s).replace(/'/g, "''");
}

/** 造四个场景并回读判定。判据 token 全 ASCII（中文只在 pwsh 侧匹配）——本仓血账：JS 侧匹配中文会被 OEM 码页骗。 */
function probe(ctx) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. "${SCREEN_DIFF}"`,
    `$root = '${psQuote(ctx.tmp)}'`,
    'function MakeShot([string]$dir, [string]$name, [byte[]]$bytes) {',
    '  New-Item -ItemType Directory -Force -Path $dir | Out-Null',
    '  [System.IO.File]::WriteAllBytes((Join-Path $dir $name), $bytes)',
    '}',
    'function Emit([string]$tag, $r) {',
    '  $d = @($r.Diff)',
    '  Write-Output ($tag + "_CHANGED=" + $r.ChangedCount)',
    '  Write-Output ($tag + "_NEW=" + [bool]$r.NewBaseline)',
    '  Write-Output ($tag + "_DIFFN=" + $d.Count)',
    // 「新增」这条标签在 pwsh 侧匹配（不经 JS），用来证明分类真的发生了
    '  Write-Output ($tag + "_NEWITEM=" + @($d | Where-Object { $_ -like "*新增*" }).Count)',
    '  Write-Output ($tag + "_HAS_ERROR=" + [bool](-not [string]::IsNullOrEmpty($r.Error)))',
    '}',
    // C1：恰好一个截图 + 基线目录不存在（本次真机撞到的形态）
    "$c1 = Join-Path $root 'c1\\current'",
    "$c1b = Join-Path $root 'c1\\baseline'",
    "MakeShot $c1 'a.png' ([byte[]](1,2,3,4,5))",
    "Emit 'C1' (Compare-ScreenshotBaseline -CurrentDir $c1 -BaselineDir $c1b)",
    // C2：两个截图 + 基线目录不存在
    "$c2 = Join-Path $root 'c2\\current'",
    "$c2b = Join-Path $root 'c2\\baseline'",
    "MakeShot $c2 'a.png' ([byte[]](1,2,3,4,5))",
    "MakeShot $c2 'b.png' ([byte[]](6,7,8,9,10))",
    "Emit 'C2' (Compare-ScreenshotBaseline -CurrentDir $c2 -BaselineDir $c2b)",
    // C3：零个截图
    "$c3 = Join-Path $root 'c3\\current'",
    "$c3b = Join-Path $root 'c3\\baseline'",
    'New-Item -ItemType Directory -Force -Path $c3 | Out-Null',
    "Emit 'C3' (Compare-ScreenshotBaseline -CurrentDir $c3 -BaselineDir $c3b)",
    // C4：基线已存在且内容相同
    "$c4 = Join-Path $root 'c4\\current'",
    "$c4b = Join-Path $root 'c4\\baseline'",
    "MakeShot $c4 'a.png' ([byte[]](11,12,13))",
    "MakeShot $c4b 'a.png' ([byte[]](11,12,13))",
    "Emit 'C4' (Compare-ScreenshotBaseline -CurrentDir $c4 -BaselineDir $c4b)",
    'Write-Output "PROBE_DONE=1"',
  ].join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
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

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

describe('Compare-ScreenshotBaseline 单文件/严格模式（运行期，#1027 收尾）', () => {
  let ctx;
  let probeResult;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'shotdiff-')) };
    probeResult = probe(ctx);
    if (probeResult.ok) {
      ['C1', 'C2', 'C3', 'C4'].forEach((tag) => {
        v[tag] = {
          changed: field(probeResult.stdout, `${tag}_CHANGED`),
          isNew: field(probeResult.stdout, `${tag}_NEW`),
          diffN: field(probeResult.stdout, `${tag}_DIFFN`),
          newItem: field(probeResult.stdout, `${tag}_NEWITEM`),
          hasError: field(probeResult.stdout, `${tag}_HAS_ERROR`),
        };
      });
    }
  });

  afterAll(() => {
    if (ctx) {
      try {
        fs.rmSync(ctx.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红
      }
    }
  });

  test('P0: 探针跑完（pwsh 可用、StrictMode 下四个场景都没抛错）', () => {
    if (!probeResult.ok) {
      throw new Error(
        'Compare-ScreenshotBaseline 探针失败（pwsh 不可用或函数抛错，fail-closed 不跳过）：\n'
          + `exit=${probeResult.status}\nstdout=${probeResult.stdout}\nstderr=${probeResult.stderr}`
      );
    }
    expect(field(probeResult.stdout, 'PROBE_DONE')).toBe('1');
    ['C1', 'C2', 'C3', 'C4'].forEach((tag) => {
      expect(v[tag].changed).not.toBeNull();
    });
  });

  // C1 就是本缺陷的回归钉：修好前它整条探针都会挂（「在此对象上找不到属性 Count」）
  test('C1: 恰一个截图 + 基线不存在 ⇒ 不抛错、ChangedCount=1、NewBaseline=true、判「新增」', () => {
    expect(v.C1.changed).toBe('1');
    expect(v.C1.isNew).toBe('True');
    expect(v.C1.diffN).toBe('1');
    expect(v.C1.newItem).toBe('1');
    expect(v.C1.hasError).toBe('False');
  });

  test('C2: 两个截图 + 基线不存在 ⇒ ChangedCount=2、NewBaseline=true', () => {
    expect(v.C2.changed).toBe('2');
    expect(v.C2.isNew).toBe('True');
    expect(v.C2.diffN).toBe('2');
  });

  test('C3: 零个截图 ⇒ ChangedCount=0 且明报「无当前截图可对比」', () => {
    expect(v.C3.changed).toBe('0');
    expect(v.C3.hasError).toBe('True');
  });

  test('C4: 基线存在且内容相同 ⇒ ChangedCount=0、NewBaseline=false', () => {
    expect(v.C4.changed).toBe('0');
    expect(v.C4.isNew).toBe('False');
    expect(v.C4.diffN).toBe('1');
  });
});
