/**
 * `Compare-ScreenFrames` 的**宽容比较**语义，运行期守护（#1027 收尾，2026-09-15 真机实测）
 *
 * 为什么需要（真机实测，血账）：
 *   `Wait-NavSettled` 的「画面稳定」原先是**全帧 hash 相等**。而系统状态栏里有**应用控制不了**的
 *   实时读数（MIUI「显示实时网速」的 KB/s，每 1–2 秒就变）⇒ 连拍三帧的整帧 sha256 **两两不同**
 *   （实测差异**只在顶部 0–99px 状态栏**、MaxDiff 188，其余整幅逐字节相同）⇒ 判据**永远不可能满足**
 *   ⇒ 步骤 6 必然 420 秒超时、步骤 7–9（对比 / 证据 / 还原）永远跑不到，而 app 画面早就落定了。
 *   ⇒ 改为**采样网格上的差异像素占比**（≤ 阈值即判内容一致）。本文件钉住这个语义：
 *     ① 微小的系统 UI churn **必须**判「一致」（否则又退回永远不落定）；
 *     ② 真切页 / 滚动那种大面积变化**必须**判「不一致」（fail-closed 语义不得被放宽）；
 *     ③ 缺上一帧 / 取色不可用**必须**判「不一致」并把原因回带（宁可不落定，绝不假稳定）。
 *
 * 标定（同机实测，1080×2400）：状态栏量级 churn 在 24/48/96 网格上 = 0%、192 网格 = 0.011%；
 *   真切页（对照全黑帧）= ~100% ⇒ 默认阈值 0.5% 在噪声上方 ~45×、真变化下方 ~200×。
 *
 * ⚠️ **判别力的平台边界（写实，不许假称处处有判别力）**：本函数靠 `System.Drawing` 取色，而它
 *   **只在 Windows 可用**（PowerShell 7 的 System.Drawing.Common 是 Windows-only）。所以每个用例
 *   都显式分两支、**两支都断言**（没有静默跳过），只是各自在自己平台上验该验的那一面：
 *     · Windows（本工具链的运行平台）：验「微变宽容 + 巨变拦得住」；
 *     · Linux（本仓 CI 的 ubuntu runner）：验**缺件时的 fail-closed 语义**（判不出 ⇒ 不判稳定，
 *       且回带的 `DiffPercent=-1` 哨兵值必须出现 —— 它是「没比较」与「比过了但不同」的区分点）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（仓库先例：
 *   `hxLaunchDetachBehavior.test.js` / `contractTestPatternBehavior.test.js`）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const AUTO_SHOT = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 造四张合成帧（600×800，24 步网格 ⇒ stepX=25 / stepY=33 / 共 600 个采样点）并回读判定：
 *  · base      基准帧（纯色 + 一个白块）
 *  · identical 与 base **逐字节相同**（复制）
 *  · tiny      只改 **6×6**、恰好命中**一个**采样点（1/600 = 0.167% ⇒ 应判「一致」但仍 >0）
 *  · big       改掉上半幅（真切页量级 ⇒ 应判「不一致」）
 *  判据 token 全是 ASCII（True/False/数字），JS 侧**不匹配中文**（本仓血账：OEM 码页会把中文变乱码）。
 */
function probeFrames(ctx) {
  const q = (s) => String(s).replace(/'/g, "''");
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. "${AUTO_SHOT}"`,
    'try { Add-Type -AssemblyName System.Drawing -ErrorAction Stop; $drawing = $true } catch { $drawing = $false }',
    'Write-Output ("DRAWING_AVAILABLE=" + [bool]$drawing)',
    `$dir = '${q(ctx.tmp)}'`,
    'if ($drawing) {',
    '  $w = 600; $h = 800',
    '  $bmp = New-Object System.Drawing.Bitmap $w, $h',
    '  $g = [System.Drawing.Graphics]::FromImage($bmp)',
    '  $g.Clear([System.Drawing.Color]::FromArgb(255, 210, 226, 245))',
    '  $g.FillRectangle([System.Drawing.Brushes]::White, 20, 300, 560, 200)',
    '  $g.Dispose()',
    "  $bmp.Save((Join-Path $dir 'base.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $bmp.Dispose()',
    "  Copy-Item (Join-Path $dir 'base.png') (Join-Path $dir 'identical.png') -Force",
    '  $b = New-Object System.Drawing.Bitmap (Join-Path $dir "base.png")',
    '  $g2 = [System.Drawing.Graphics]::FromImage($b)',
    '  $g2.FillRectangle([System.Drawing.Brushes]::Black, 498, 31, 6, 6)',
    '  $g2.Dispose()',
    "  $b.Save((Join-Path $dir 'tiny.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $b.Dispose()',
    '  $c = New-Object System.Drawing.Bitmap (Join-Path $dir "base.png")',
    '  $g3 = [System.Drawing.Graphics]::FromImage($c)',
    '  $g3.FillRectangle([System.Drawing.Brushes]::Black, 0, 0, $w, [int]($h / 2))',
    '  $g3.Dispose()',
    "  $c.Save((Join-Path $dir 'big.png'), [System.Drawing.Imaging.ImageFormat]::Png)",
    '  $c.Dispose()',
    '}',
    'function Emit([string]$tag, $r) {',
    '  Write-Output ($tag + "_SAME=" + [bool]$r.Same)',
    '  Write-Output ($tag + "_PCT=" + $r.DiffPercent)',
    // ⚠️ 成功路径的返回对象**没有** Error 属性 ⇒ StrictMode 下 `$r.Error` 会直接抛错（本用例第一版就栽在这），
    //    必须用 PSObject.Properties 判「属性在不在」，而不是判值是否为 $null。
    '  Write-Output ($tag + "_HAS_ERROR=" + [bool]($null -ne $r.PSObject.Properties["Error"]))',
    '}',
    "Emit 'IDENTICAL' (Compare-ScreenFrames -Path (Join-Path $dir 'identical.png') -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'TINY'      (Compare-ScreenFrames -Path (Join-Path $dir 'tiny.png')      -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'BIG'       (Compare-ScreenFrames -Path (Join-Path $dir 'big.png')       -PrevPath (Join-Path $dir 'base.png'))",
    "Emit 'MISSING'   (Compare-ScreenFrames -Path (Join-Path $dir 'base.png')      -PrevPath (Join-Path $dir 'no-such-prev.png'))",
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

describe('Compare-ScreenFrames 宽容比较语义（运行期，#1027 收尾）', () => {
  let ctx;
  let probe;
  let drawing;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'stability-')) };
    probe = probeFrames(ctx);
    if (probe.ok) {
      drawing = field(probe.stdout, 'DRAWING_AVAILABLE');
      ['IDENTICAL', 'TINY', 'BIG', 'MISSING'].forEach((tag) => {
        v[tag] = {
          same: field(probe.stdout, `${tag}_SAME`),
          pct: field(probe.stdout, `${tag}_PCT`),
          hasError: field(probe.stdout, `${tag}_HAS_ERROR`),
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

  const canDraw = () => drawing === 'True';

  // B1：探针必须真跑完、判据 token 齐备（fail-closed，不 skip）
  test('B1: 探针跑完，判据 token 齐备（pwsh 可用）', () => {
    if (!probe.ok) {
      throw new Error(
        'Compare-ScreenFrames 探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
          + `exit=${probe.status}\nstdout=${probe.stdout}\nstderr=${probe.stderr}`
      );
    }
    expect(field(probe.stdout, 'PROBE_DONE')).toBe('1');
    expect(['True', 'False']).toContain(drawing);
    ['IDENTICAL', 'TINY', 'BIG', 'MISSING'].forEach((tag) => {
      expect(['True', 'False']).toContain(v[tag].same);
      expect(['True', 'False']).toContain(v[tag].hasError);
    });
  });

  // B2：逐字节相同的两帧 ⇒ 必须判「一致」（差异 0%），否则连静止页面都落定不了
  test('B2: 完全相同的两帧 ⇒ 判内容一致（差异 0%）', () => {
    if (!canDraw()) {
      // Linux CI：取色不可用 ⇒ 只能是 fail-closed 那一支，且必须回带 -1 哨兵（「没比较」）
      expect(v.IDENTICAL.same).toBe('False');
      expect(v.IDENTICAL.hasError).toBe('True');
      expect(Number(v.IDENTICAL.pct)).toBe(-1);
      return;
    }
    expect(v.IDENTICAL.same).toBe('True');
    expect(v.IDENTICAL.hasError).toBe('False');
    expect(Number(v.IDENTICAL.pct)).toBe(0);
  });

  // B3：只改一个采样点（6×6，状态栏实时读数量级）⇒ **仍判一致**，但占比必须**大于 0**
  //     （大于 0 才证明「差异被真的看见了、只是被宽容掉了」，而不是网格恰好漏掉 ⇒ 弱断言）。
  //     这条就是本 bug 的回归钉：退回全帧 hash 相等时它会红（现实中它让步骤 7–9 永远跑不到）。
  test('B3: 命中 1/600 个采样点的微变 ⇒ 判内容一致（宽容生效，且差异确实被看见）', () => {
    if (!canDraw()) {
      expect(v.TINY.same).toBe('False');
      expect(v.TINY.hasError).toBe('True');
      return;
    }
    expect(v.TINY.same).toBe('True');
    const pct = Number(v.TINY.pct);
    expect(pct).toBeGreaterThan(0);
    expect(pct).toBeLessThanOrEqual(0.5);
  });

  // B4：真切页量级的大面积变化（上半幅）⇒ **必须**判不一致（宽容不放宽 fail-closed 语义）
  test('B4: 半幅画面变化 ⇒ 判内容不一致（该拦的照旧拦）', () => {
    if (!canDraw()) {
      expect(v.BIG.same).toBe('False');
      expect(v.BIG.hasError).toBe('True');
      return;
    }
    expect(v.BIG.same).toBe('False');
    expect(Number(v.BIG.pct)).toBeGreaterThan(0.5);
  });

  // B5：缺上一帧（首轮采样）⇒ 判不一致且回带原因，绝不假稳定
  test('B5: 缺上一帧 ⇒ 判不一致并回带原因（fail-closed）', () => {
    expect(v.MISSING.same).toBe('False');
    expect(v.MISSING.hasError).toBe('True');
  });
});
