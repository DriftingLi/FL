/**
 * PNG 像素差模块（scripts/lib/png-diff.mjs）行为守护 —— Q17「把截图对比从空转修成真的」的地基
 *
 * 为什么这层必须存在：旧实现只比 MD5（见 utils/screenshotDiffContract.test.js 的 D6），
 *   只能回答「字节是否完全相同」。截图噪声是**必然**的（字体栅格化、抗锯齿、状态栏时钟），
 *   于是要么每页恒「有变化」（噪声淹没真变化），要么人为忽略它（这层就白建）。
 *
 * 为什么是「真跑 CLI」而不是 import 内部函数：本仓 jest 的 `moduleFileExtensions` 只有
 *   `['js','json']`，`.mjs` **解析不了**（与 `.uts` 同款的缝）；而 `spawn` 跑 CLI 正好也是
 *   `screenshot-diff.ps1` 将来调它的**真实形态**（守的契约是 stdout JSON，不是内部实现）。
 *
 * 反永绿（**成对**断言，缺一不可）：
 *   P1 = 恰好 1 像素不同、用默认阈值 ⇒ 必须 **changed=false**（证明阈值真的在起作用 —— MD5 会判「有变化」）
 *   P2 = 30% 像素不同              ⇒ 必须 **changed=true**（证明它**不是恒绿**）
 *   再补 P6：同一对图把阈值收成 0 ⇒ 必须 changed=true（证明 P1 的 false 来自**阈值**，而不是差异没被发现）
 *
 * 需要 node（跑 CLI）+ zlib（造真 PNG）⇒ Windows 与 Linux CI 都能跑，**不需要 System.Drawing**、
 * 不需要外部图像工具。zlib/CRC 是 Node 内置能力，不引入依赖。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const zlib = require('zlib');

const ROOT = path.join(__dirname, '..');
const CLI = path.join(ROOT, 'scripts', 'lib', 'png-diff.mjs');

// ---------- 造真 PNG（8-bit RGBA、非交错、filter=0）----------

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

/** rgba: Buffer(width*height*4) → 完整 PNG 字节 */
function makePng(width, height, rgba) {
  const stride = width * 4;
  const raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y += 1) {
    raw[y * (stride + 1)] = 0; // filter type 0
    rgba.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;  // bit depth
  ihdr[9] = 6;  // color type RGBA
  ihdr[10] = 0; // compression
  ihdr[11] = 0; // filter
  ihdr[12] = 0; // interlace
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const W = 40;
const H = 30; // 1200 像素：1px→0.000833，400px→0.3333

/** 纯色底图，再把指定像素改成另一个颜色 */
function solidWithChanges(indices) {
  const rgba = Buffer.alloc(W * H * 4);
  for (let i = 0; i < W * H; i += 1) {
    rgba[i * 4] = 32; rgba[i * 4 + 1] = 96; rgba[i * 4 + 2] = 160; rgba[i * 4 + 3] = 255;
  }
  indices.forEach((p) => {
    rgba[p * 4] = 255; rgba[p * 4 + 1] = 0; rgba[p * 4 + 2] = 0; rgba[p * 4 + 3] = 255;
  });
  return rgba;
}

/** 跑 CLI，容忍非零退出码（退出码本身不是判据，JSON 才是） */
function runCli(args) {
  try {
    const stdout = execFileSync('node', [CLI].concat(args), {
      encoding: 'utf8', timeout: 60000, windowsHide: true, maxBuffer: 4 * 1024 * 1024,
    });
    return { json: JSON.parse(String(stdout).trim()), exit: 0 };
  } catch (e) {
    const out = String(e.stdout || '').trim();
    let json = null;
    try { json = JSON.parse(out); } catch { /* 保持 null */ }
    return { json, exit: e.status, raw: out, stderr: String(e.stderr || e.message || '') };
  }
}

describe('png-diff.mjs：像素差 + 阈值（Q17 地基，反永绿成对断言）', () => {
  let tmp;
  const f = {};

  beforeAll(() => {
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'pngdiff-'));
    const base = makePng(W, H, solidWithChanges([]));
    const onePx = makePng(W, H, solidWithChanges([0]));
    const manyPx = makePng(W, H, solidWithChanges(Array.from({ length: 400 }, (_, i) => i)));
    // 顶部两行全改（80px = 6.67%）——「状态栏时钟/网速读数」那一族的替身
    const topRows = makePng(W, H, solidWithChanges(Array.from({ length: W * 2 }, (_, i) => i)));
    const smaller = makePng(20, H, solidWithChanges([]));

    f.base = path.join(tmp, 'base.png');
    f.onePx = path.join(tmp, 'one-px.png');
    f.manyPx = path.join(tmp, 'many-px.png');
    f.topRows = path.join(tmp, 'top-rows.png');
    f.smaller = path.join(tmp, 'smaller.png');
    f.notPng = path.join(tmp, 'not-a-png.png');

    fs.writeFileSync(f.base, base);
    fs.writeFileSync(f.onePx, onePx);
    fs.writeFileSync(f.manyPx, manyPx);
    fs.writeFileSync(f.topRows, topRows);
    fs.writeFileSync(f.smaller, smaller);
    // 复刻既有守护 utils/screenshotDiffSingleFileBehavior.test.js 喂的**假 PNG 字节**
    fs.writeFileSync(f.notPng, Buffer.from([1, 2, 3, 4, 5]));
  });

  afterAll(() => {
    if (tmp) { try { fs.rmSync(tmp, { recursive: true, force: true }); } catch { /* 清理失败不判红 */ } }
  });

  test('P0: 同内容 ⇒ mode=pixel、different=0、changed=false', () => {
    const r = runCli(['--a', f.base, '--b', f.base]);
    expect(r.json).toMatchObject({ ok: true, mode: 'pixel', different: 0, total: W * H, changed: false });
  });

  // P1 ——「阈值真的在起作用」：MD5 会说这两张图**不同**，像素层必须说「无变化」
  test('P1: 恰 1 像素不同 + 默认阈值(0.5%) ⇒ changed=false（否则每页恒红）', () => {
    const r = runCli(['--a', f.base, '--b', f.onePx]);
    expect(r.json).toMatchObject({ ok: true, mode: 'pixel', different: 1, changed: false });
    expect(r.json.ratio).toBeLessThan(r.json.threshold);
  });

  // P2 ——「它不是恒绿」：这是 P1 的对照项，缺了 P1 就是恒绿，缺了 P2 就是恒红
  test('P2: 30% 像素不同 ⇒ changed=true（对照项：证明判据不是恒绿）', () => {
    const r = runCli(['--a', f.base, '--b', f.manyPx]);
    expect(r.json).toMatchObject({ ok: true, mode: 'pixel', different: 400, changed: true });
    expect(r.json.ratio).toBeGreaterThan(r.json.threshold);
  });

  // P6 —— 证明 P1 的 false 来自**阈值**，而不是「差异没被发现」
  test('P6: 同一对图把阈值收成 0 ⇒ changed=true（证明 P1 的 false 是阈值造成的）', () => {
    const r = runCli(['--a', f.base, '--b', f.onePx, '--threshold', '0']);
    expect(r.json).toMatchObject({ ok: true, different: 1, threshold: 0, changed: true });
  });

  test('P5: --ignore-top-rows 2 可忽略顶部两行的整体改动（状态栏读数那一族）', () => {
    const strict = runCli(['--a', f.base, '--b', f.topRows]);
    expect(strict.json.changed).toBe(true); // 不忽略时 80px 改动 > 0.5% ⇒ 必须红
    const ignored = runCli(['--a', f.base, '--b', f.topRows, '--ignore-top-rows', '2']);
    expect(ignored.json).toMatchObject({ ok: true, different: 0, changed: false });
    expect(ignored.json.ignoredRows).toEqual({ top: 2, bottom: 0 });
  });

  test('P3: 非 PNG 字节 ⇒ ok=false / mode=unsupported（调用方必须回退 MD5，不得当「无变化」）', () => {
    const r = runCli(['--a', f.notPng, '--b', f.base]);
    expect(r.exit).toBe(2);
    expect(r.json.ok).toBe(false);
    expect(r.json.mode).toBe('unsupported');
    expect(String(r.json.reason)).toMatch(/png/);
  });

  test('P4: 尺寸不同 ⇒ ok=false（回退 MD5；尺寸变了本来就该算「有变化」）', () => {
    const r = runCli(['--a', f.base, '--b', f.smaller]);
    expect(r.json).toMatchObject({ ok: false, mode: 'unsupported' });
    expect(String(r.json.reason)).toMatch(/size-mismatch/);
  });
});
