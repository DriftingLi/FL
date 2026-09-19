/**
 * 截图门（`dev:finish` 步骤 7）的**行为**守护 —— #1139：从「空转」修成「真的会 fail」
 *
 * 为什么这层必须存在（本仓三个「永远绿」先例之一）：旧步骤 7 在「有变化」时**只 `Write-Host`
 *   一行黄字**，没有 `Write-Result`、没有 `exit` ⇒ **永不 fail**。任何 `expect(src).toContain(...)`
 *   都抓不到这件事 —— 源码文本长得完全一样，只有**真跑一次**才知道它红不红。
 *
 * 为什么判定被抽到 `scripts/lib/screenshot-gate.ps1` 的 `Get-PngDiffVerdict`：把「有变化要不要红」
 *   从 I/O 里拆出来才跑得动（不用设备、不用 HBuilderX）。**纯函数**，输入就是
 *   `Compare-ScreenshotBaseline` 的返回对象。
 *
 * 反永绿（**成对**断言，缺一不可 —— 只测一个方向不算数）：
 *   G3 = 同内容 + 基线存在      ⇒ Ok=true  且 ExitCode=0 （「一致」判得出来）
 *   G4 = 40×40 像素变化(19.5%)  ⇒ Ok=false 且 ExitCode=1 （**它真的会红**；缺这条就是恒绿）
 *   G5 = 同上 + -UpdateBaseline ⇒ Ok=true  且 Action=refresh-baseline（「确认是有意改动」的出路真的在）
 *   另外 G2 证明**阈值真的在起作用**：恰 1 像素不同（0.012%）⇒ 不红（否则每页恒红）。
 *
 * 为什么也要真跑 `Compare-ScreenshotBaseline`（G1–G6）而不只测纯函数：纯函数测试会自己喂
 *   `ChangedCount`，**接线断了它照样绿**。所以先跑模块拿到真实计数，再把真实结果喂给判定 ——
 *   这样「像素层真的接进来了」和「判定真的会红」是同一条链上的两件事。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）+ `node`（像素层）。**不可用时 fail-closed 抛错，不 skip**
 *   （仓库先例：`screenshotDiffSingleFileBehavior.test.js` / `hxLaunchDetachBehavior.test.js`）。
 * 不需要设备、不需要 System.Drawing、不需要外部图像工具 ⇒ Windows 与 Linux CI 都能跑。
 *
 * 判据 token 全 ASCII：本仓血账 —— JS 侧匹配中文会被 OEM 码页骗（中文文案只在 pwsh 侧匹配）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const zlib = require('zlib');

const ROOT = path.join(__dirname, '..');
const SCREEN_DIFF = path.join(ROOT, 'scripts', 'lib', 'screenshot-diff.ps1');

// ---------- 造真 PNG（8-bit RGBA、非交错、filter=0；与 pngDiffBehavior.test.js 同款）----------

const CRC_TABLE = (() => {
  const t = new Int32Array(256);
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c;
  }
  return t;
})();

function crc32(buf) {
  let c = -1;
  for (let i = 0; i < buf.length; i += 1) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ -1) >>> 0;
}

function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length, 0);
  const typeBuf = Buffer.from(type, 'ascii');
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])), 0);
  return Buffer.concat([len, typeBuf, data, crc]);
}

function makePng(width, height, rgba) {
  const stride = width * 4;
  const raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y += 1) {
    raw[y * (stride + 1)] = 0;
    rgba.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;
  ihdr[9] = 6;
  ihdr[10] = 0;
  ihdr[11] = 0;
  ihdr[12] = 0;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const W = 128;
const H = 64; // 8192 像素：1px→0.012%，1600px(40×40)→19.5%，阈值 0.5% 恰好卡在中间

function solid(rects) {
  const rgba = Buffer.alloc(W * H * 4);
  for (let i = 0; i < W * H; i += 1) {
    rgba[i * 4] = 32; rgba[i * 4 + 1] = 96; rgba[i * 4 + 2] = 160; rgba[i * 4 + 3] = 255;
  }
  (rects || []).forEach(([x0, y0, x1, y1]) => {
    for (let y = y0; y < y1; y += 1) {
      for (let x = x0; x < x1; x += 1) {
        const i = (y * W + x) * 4;
        rgba[i] = 255; rgba[i + 1] = 0; rgba[i + 2] = 0; rgba[i + 3] = 255;
      }
    }
  });
  return rgba;
}

// ---------- pwsh 探针 ----------

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function psQuote(s) {
  return String(s).replace(/'/g, "''");
}

/**
 * 真跑 `Compare-ScreenshotBaseline` + `Get-PngDiffVerdict`，回读六个场景的判定。
 * 场景（与判据表一一对应）：
 *   G1 基线目录不存在 + 一个当前截图        ⇒ 首次：NewBaseline=True、ChangedCount=1、md5 回退路径
 *   G2 基线 == 当前（真对比）               ⇒ pixel 层真的跑了、无变化、判定 Ok/ExitCode=0
 *   G3 恰 1 像素不同（0.012% < 阈值）       ⇒ 不红（证明阈值在起作用；MD5 会说这两张不同）
 *   G4 40×40 像素不同（19.5% > 阈值）       ⇒ Ok=False / ExitCode=1（**它真的会红**）
 *   G5 同上 + -UpdateBaseline               ⇒ Ok=True / refresh-baseline（出路真的在）
 *   G6 零截图                               ⇒ 判不了 ⇒ Ok=False / ExitCode=1（不当「无变化」）
 *   G10 基线来自上一轮 + 本轮确有真差异      ⇒ **真比出「有变化」**（不是退化成「判不了」）——
 *                                             原实现把基线侧也按运行起点过滤 ⇒ 这里整轮跳过，
 *                                             它就是那处回归的回归钉
 *   ── 以下各组是 #1158（2026-09-18）：目录**从不清理**，上一轮失败运行的残留不得算进本轮 ──
 *   G7  陈旧残留（也在基线里）+ 本轮 1 页     ⇒ 只本轮那页在范围内，残留被**可见地**跳过（理由+哪一侧）
 *   G8  基线里有、本轮没截到的那页            ⇒ 是**缺失候选**（原实现把「缺失」变成了死代码）
 *   G9  一张本轮产物都没有（全是残留）         ⇒ 本轮范围为空、残留数可见（不静默成「无变化」）
 *   G11 多页稳态：当前全新鲜 + 基线全来自上轮  ⇒ InScope = 全部、Skipped = 0（原实现会整轮跳过）
 */
function probe(ctx) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. "${SCREEN_DIFF}"`,
    `$root = '${psQuote(ctx.tmp)}'`,
    // 用 Change-* 而非 G1..G6 —— 名字只求可读，回读靠 TAG=
    'function Emit([string]$tag, $r, $v) {',
    '  Write-Output ($tag + "_CHANGED=" + [int]$r.ChangedCount)',
    '  Write-Output ($tag + "_NEW=" + [bool]$r.NewBaseline)',
    '  Write-Output ($tag + "_PIXELRAN=" + [bool]$r.PixelRan)',
    '  Write-Output ($tag + "_MODE=" + [string]$r.Mode)',
    '  Write-Output ($tag + "_FALLBACKN=" + @($r.Fallback).Count)',
    '  Write-Output ($tag + "_HASERROR=" + [bool](-not [string]::IsNullOrEmpty($r.Error)))',
    '  Write-Output ($tag + "_OK=" + [bool]$v.Ok)',
    '  Write-Output ($tag + "_ACTION=" + [string]$v.Action)',
    '  Write-Output ($tag + "_EXIT=" + [int]$v.ExitCode)',
    '}',
    // G1：首次运行 —— 基线目录不存在
    "$g1 = Join-Path $root 'g1'",
    'New-Item -ItemType Directory -Force -Path (Join-Path $g1 "cur") | Out-Null',
    'Copy-Item (Join-Path $root "base.png") (Join-Path $g1 "cur\\a.png") -Force',
    '$r1 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g1 "cur") -BaselineDir (Join-Path $g1 "base")',
    'Emit "G1" $r1 (Get-PngDiffVerdict -DiffResult $r1)',
    // 首跑自动填基线（复刻 dev-finish.ps1 步骤 7 的动作）
    'New-Item -ItemType Directory -Force -Path (Join-Path $g1 "base") | Out-Null',
    'Copy-Item (Join-Path $g1 "cur\\a.png") (Join-Path $g1 "base\\a.png") -Force',
    // G2：基线 == 当前（真跑像素层）
    '$r2 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g1 "cur") -BaselineDir (Join-Path $g1 "base")',
    'Emit "G2" $r2 (Get-PngDiffVerdict -DiffResult $r2)',
    // G3：恰 1 像素不同 ⇒ 阈值内 ⇒ 不红
    'Copy-Item (Join-Path $root "one-px.png") (Join-Path $g1 "cur\\a.png") -Force',
    '$r3 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g1 "cur") -BaselineDir (Join-Path $g1 "base")',
    'Emit "G3" $r3 (Get-PngDiffVerdict -DiffResult $r3)',
    // G5：40×40 像素不同 + -UpdateBaseline ⇒ 出路（顺带把基线刷成 block 版）
    'Copy-Item (Join-Path $root "block.png") (Join-Path $g1 "cur\\a.png") -Force',
    '$r5 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g1 "cur") -BaselineDir (Join-Path $g1 "base")',
    'Emit "G5" $r5 (Get-PngDiffVerdict -DiffResult $r5 -UpdateBaseline)',
    'Copy-Item (Join-Path $g1 "cur\\a.png") (Join-Path $g1 "base\\a.png") -Force',
    // G4：与基线**不同**的一轮（切回 one-px 版）且未确认 ⇒ 必须红。顺序刻意放在 G5 之后：
    //     若先跑 G4 再跑 G5，G5 就变成「与上一轮相同的图」而恒绿 —— 那种自欺正是本仓的教训。
    'Copy-Item (Join-Path $root "one-px.png") (Join-Path $g1 "cur\\a.png") -Force',
    '$r4 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g1 "cur") -BaselineDir (Join-Path $g1 "base")',
    'Emit "G4" $r4 (Get-PngDiffVerdict -DiffResult $r4)',
    // G6：零截图 ⇒ 判不了，必须红
    "$g6 = Join-Path $root 'g6'",
    'New-Item -ItemType Directory -Force -Path (Join-Path $g6 "cur") | Out-Null',
    '$r6 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g6 "cur") -BaselineDir (Join-Path $g6 "base")',
    'Emit "G6" $r6 (Get-PngDiffVerdict -DiffResult $r6)',
    // G10（生产真实形态 + 真文件 + 真入口）：基线来自**上一轮**（mtime 早于本轮起点）、
    //   本轮截图新鲜、且与基线**确有差异** ⇒ 必须真的比出「有变化」，而不是退化成
    //   「无本轮截图可对比（陈旧残留）⇒ 判不了」。这就是 #1158 修复自身踩的坑的回归钉。
    "$g10 = Join-Path $root 'g10'",
    'New-Item -ItemType Directory -Force -Path (Join-Path $g10 "cur") | Out-Null',
    'New-Item -ItemType Directory -Force -Path (Join-Path $g10 "base") | Out-Null',
    'Copy-Item (Join-Path $root "block.png") (Join-Path $g10 "cur\\a.png") -Force',
    'Copy-Item (Join-Path $root "base.png")  (Join-Path $g10 "base\\a.png") -Force',
    '$g10start = (Get-Date).AddMinutes(-1)',
    '(Get-Item (Join-Path $g10 "cur\\a.png")).LastWriteTime  = (Get-Date).AddMinutes(2)',
    '(Get-Item (Join-Path $g10 "base\\a.png")).LastWriteTime = (Get-Date).AddHours(-3)',
    '$r10 = Compare-ScreenshotBaseline -CurrentDir (Join-Path $g10 "cur") -BaselineDir (Join-Path $g10 "base") -RunStartedAt $g10start',
    'Emit "G10" $r10 (Get-PngDiffVerdict -DiffResult $r10)',
    // ---- G7–G9 / G11（issue #1158）：目录里混着上一轮失败运行的残留 ⇒ 只对**本轮产物**作结论 ----
    // 用合成时间戳直接驱动纯函数（不碰文件系统）：判据本身被测，且不依赖 sleep、不受文件系统时间精度影响。
    // 起点 = now-1h；「本轮」= now；「陈旧」= now-5h（与 2026-09-15 真机那次同款：上一轮的残留）。
    '$now = Get-Date',
    '$startPt = $now.AddHours(-1)',
    '$fresh = $now; $old = $now.AddHours(-5)',
    // G7（场景 A）：陈旧残留 + 本轮 1 页 ⇒ 本轮那页在范围内、残留被**可见地**跳过
    '$s7 = Select-ThisRunShots -RunStartedAt $startPt `',
    "  -CurrentTimes @{ 'stale.png' = \$old; 'fresh.png' = \$fresh } `",
    "  -BaselineTimes @{ 'stale.png' = \$old; 'fresh.png' = \$fresh }",
    'Write-Output ("G7_INSCOPE=" + (@($s7.InScope) -join ","))',
    'Write-Output ("G7_SKIPPEDN=" + @($s7.Skipped).Count)',
    'Write-Output ("G7_SKIPPEDNAME=" + (@($s7.Skipped | ForEach-Object { $_.Name }) -join ","))',
    'Write-Output ("G7_SKIPPEDSIDE=" + (@($s7.Skipped | ForEach-Object { $_.Side }) -join ","))',
    // G8（2026-09-18 修正后的语义）：基线里有、本轮**没截到**的那页 ⇒ 它是**缺失候选**
    //   （原实现把基线侧也按「≥ 起点」过滤 ⇒ 基线恒「陈旧」⇒ 缺失检测成了死代码，永远抓不到「整页没了」）
    '$s8 = Select-ThisRunShots -RunStartedAt $startPt `',
    "  -CurrentTimes @{ 'fresh.png' = \$fresh } `",
    "  -BaselineTimes @{ 'stale.png' = \$old; 'fresh.png' = \$fresh }",
    'Write-Output ("G8_INSCOPE=" + (@($s8.InScope) -join ","))',
    'Write-Output ("G8_MISSINGCANDIDATES=" + (@($s8.InScope | Where-Object { $_ -eq "stale.png" }).Count))',
    'Write-Output ("G8_SKIPPEDN=" + @($s8.Skipped).Count)',
    // G9：一张本轮产物都没有（全是残留）⇒ 目录里**看着有图**但本轮为空 ⇒ 明报，不静默
    '$s9 = Select-ThisRunShots -RunStartedAt $startPt `',
    "  -CurrentTimes @{ 'stale.png' = \$old } `",
    "  -BaselineTimes @{ 'stale.png' = \$old }",
    'Write-Output ("G9_INSCOPEN=" + @($s9.InScope).Count)',
    'Write-Output ("G9_SKIPPEDN=" + @($s9.Skipped).Count)',
    // G11（生产真实稳态 —— 原实现在这里会**整轮跳过**）：多页，当前全新鲜、基线全来自上一轮
    '$s11 = Select-ThisRunShots -RunStartedAt $startPt `',
    "  -CurrentTimes @{ 'a.png' = \$fresh; 'b.png' = \$fresh; 'c.png' = \$fresh } `",
    "  -BaselineTimes @{ 'a.png' = \$old; 'b.png' = \$old; 'c.png' = \$old }",
    'Write-Output ("G11_INSCOPE=" + (@($s11.InScope) -join ","))',
    'Write-Output ("G11_SKIPPEDN=" + @($s11.Skipped).Count)',
    'Write-Output "PROBE_DONE=1"',
  ].join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
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

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

describe('截图门（步骤 7）行为：一致⇒绿 / 有变化⇒红（#1139，反永绿成对断言）', () => {
  let ctx;
  let probeResult;
  const v = {};

  beforeAll(() => {
    ctx = { tmp: fs.mkdtempSync(path.join(os.tmpdir(), 'shotgate-')) };
    fs.writeFileSync(path.join(ctx.tmp, 'base.png'), makePng(W, H, solid([])));
    fs.writeFileSync(path.join(ctx.tmp, 'one-px.png'), makePng(W, H, solid([[0, 0, 1, 1]])));
    fs.writeFileSync(path.join(ctx.tmp, 'block.png'), makePng(W, H, solid([[0, 0, 40, 40]])));
    probeResult = probe(ctx);
    if (probeResult.ok) {
      ['G1', 'G2', 'G3', 'G4', 'G5', 'G6', 'G10'].forEach((tag) => {
        v[tag] = {
          changed: field(probeResult.stdout, `${tag}_CHANGED`),
          isNew: field(probeResult.stdout, `${tag}_NEW`),
          pixelRan: field(probeResult.stdout, `${tag}_PIXELRAN`),
          mode: field(probeResult.stdout, `${tag}_MODE`),
          fallbackN: field(probeResult.stdout, `${tag}_FALLBACKN`),
          hasError: field(probeResult.stdout, `${tag}_HASERROR`),
          ok: field(probeResult.stdout, `${tag}_OK`),
          action: field(probeResult.stdout, `${tag}_ACTION`),
          exit: field(probeResult.stdout, `${tag}_EXIT`),
        };
      });
    }
  });

  afterAll(() => {
    if (ctx) {
      try { fs.rmSync(ctx.tmp, { recursive: true, force: true }); } catch { /* 清理失败不判红 */ }
    }
  });

  test('P0: 探针跑完（pwsh 可用、StrictMode 下六个场景都没抛错）', () => {
    if (!probeResult.ok) {
      throw new Error(
        '截图门探针失败（pwsh 不可用或函数抛错，fail-closed 不跳过）：\n'
          + `exit=${probeResult.status}\nstdout=${probeResult.stdout}\nstderr=${probeResult.stderr}`
      );
    }
    expect(field(probeResult.stdout, 'PROBE_DONE')).toBe('1');
  });

  test('G1: 首次运行（基线为空）⇒ 自动填基线并放行（kill 掉「基线永远空」的死循环）', () => {
    expect(v.G1.isNew).toBe('True');
    expect(v.G1.changed).toBe('1');
    expect(v.G1.ok).toBe('True');
    expect(v.G1.action).toBe('write-baseline');
    expect(v.G1.exit).toBe('0');
  });

  test('G2: 基线 == 当前 ⇒ 像素层真的跑了、判无变化、放行', () => {
    expect(v.G2.pixelRan).toBe('True');
    expect(v.G2.mode).toBe('pixel');
    expect(v.G2.isNew).toBe('False');
    expect(v.G2.changed).toBe('0');
    expect(v.G2.ok).toBe('True');
    expect(v.G2.exit).toBe('0');
  });

  test('G3: 恰 1 像素不同（0.012% < 阈值 0.5%）⇒ 不红（阈值真的在起作用；MD5 会说它变了）', () => {
    expect(v.G3.pixelRan).toBe('True');
    expect(v.G3.changed).toBe('0');
    expect(v.G3.ok).toBe('True');
    expect(v.G3.exit).toBe('0');
  });

  // G4 就是本票的回归钉：修好前步骤 7 在这条上**只会打一行黄字然后继续往下走**。
  test('G4: 40×40 像素不同（19.5% > 阈值）⇒ 必须红（对照项：证明这层不是恒绿）', () => {
    expect(v.G4.pixelRan).toBe('True');
    expect(v.G4.changed).toBe('1');
    expect(v.G4.ok).toBe('False');
    expect(v.G4.action).toBe('request-decision');
    expect(v.G4.exit).toBe('1');
  });

  test('G5: 同上但带 -UpdateBaseline ⇒ 放行且动作是「刷新基线」（唯一出路的载体）', () => {
    expect(v.G5.changed).toBe('1');
    expect(v.G5.ok).toBe('True');
    expect(v.G5.action).toBe('refresh-baseline');
    expect(v.G5.exit).toBe('0');
  });

  test('G6: 零截图 ⇒ 判不了 ⇒ 红（「判不了」不得当「无变化」，那是假绿）', () => {
    expect(v.G6.hasError).toBe('True');
    expect(v.G6.ok).toBe('False');
    expect(v.G6.exit).toBe('1');
  });

  // ---- G7–G9（issue #1158）：目录从不清理 ⇒ 上一轮失败运行的残留不得算进本轮 ----

  test('G7: 场景 A —— 陈旧残留 + 本轮 1 页 ⇒ 只本轮那页在范围内，残留被**可见地**跳过', () => {
    expect(field(probeResult.stdout, 'G7_INSCOPE')).toBe('fresh.png');
    expect(field(probeResult.stdout, 'G7_SKIPPEDN')).toBe('1');
    expect(field(probeResult.stdout, 'G7_SKIPPEDNAME')).toBe('stale.png');
    // 跳过必须带**理由与哪一侧**（静默丢弃 = 另一种假绿）；只判当前侧 ⇒ side 只可能是 current
    expect(field(probeResult.stdout, 'G7_SKIPPEDSIDE')).toBe('current');
  });

  test('G10: 生产真实形态（基线来自上一轮 + 本轮确有差异）⇒ 必须真比出「有变化」', () => {
    // 修好前：基线侧也被按运行起点过滤 ⇒ 整轮被判成「陈旧残留」、ChangedCount=0、
    // 结论退化成「无本轮截图可对比」的**判不了** —— 变化被吞掉，而且错因写错。
    expect(v.G10.changed).toBe('1');
    expect(v.G10.hasError).toBe('False');
    expect(v.G10.ok).toBe('False');
    expect(v.G10.action).toBe('request-decision');
    expect(v.G10.exit).toBe('1');
  });

  test('G8: 基线里有、本轮没截到的那页 ⇒ 是「缺失」候选（修正后的语义）', () => {
    // 2026-09-18 修正：基线是上一轮的参考图，mtime **必然**早于本轮起点；按运行起点过滤基线
    // ⇒ 「缺失」这条判据永远抓不到东西（死代码）。缺失只该由「基线有、本轮没有」决定。
    expect(field(probeResult.stdout, 'G8_INSCOPE')).toBe('fresh.png,stale.png');
    expect(field(probeResult.stdout, 'G8_MISSINGCANDIDATES')).toBe('1');
    expect(field(probeResult.stdout, 'G8_SKIPPEDN')).toBe('0');
  });

  test('G11: 多页稳态（当前全新鲜 + 基线全来自上一轮）⇒ 全部在范围内、一张都不跳过', () => {
    expect(field(probeResult.stdout, 'G11_INSCOPE')).toBe('a.png,b.png,c.png');
    expect(field(probeResult.stdout, 'G11_SKIPPEDN')).toBe('0');
  });

  test('G9: 全是残留（本轮一张都没有）⇒ 本轮范围为空且残留数可见（不是「无变化」）', () => {
    expect(field(probeResult.stdout, 'G9_INSCOPEN')).toBe('0');
    expect(field(probeResult.stdout, 'G9_SKIPPEDN')).toBe('1');
  });
});
