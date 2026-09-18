#!/usr/bin/env node
// 纯 Node、零依赖的 PNG 像素差 —— 供 `scripts/lib/screenshot-diff.ps1` 调用。
//
// 为什么需要它（Q17：把「截图对比」从空转修成真的）：
//   旧实现只比 `Get-FileHash`（MD5）⇒ 只能回答「字节是否完全相同」。而截图噪声是**必然**存在的
//   （字体栅格化、抗锯齿、状态栏时钟/网速读数），于是两条路都坏：要么每页恒「有变化」（噪声淹没真变化），
//   要么人为忽略它（那这层就白建）。**像素差 + 阈值**才是能用的判据。
//
// 为什么不用 System.Drawing：CI（`mobile-test` job）跑在 **ubuntu** 上，而 .NET 的 System.Drawing
//   自 .NET 6 起**仅 Windows 可用**；本仓守护 `utils/screenshotDiffSingleFileBehavior.test.js` 的头部
//   明确要求「不需要 System.Drawing ⇒ Windows 与 Linux CI 都能跑」。用它会把这层判据从 CI 里摘掉。
// 为什么不用 magick/ffmpeg：它们**不保证存在**（② 门的截图入库段就是在探测它们，三个皆无时退 .NET）。
// 为什么 Node 可以：本仓移动端工具链**已经硬依赖 Node**（`npm run test:unit`、② 门的探针都是 Node）。
//   代价是 pwsh 侧需要处理「node 不在」—— 那时**回退 MD5**并把 mode 如实记进结果（不静默）。
//
// 支持的 PNG 子集（其余一律 `unsupported`，由调用方回退 MD5）：
//   8-bit、非交错、colorType ∈ {0 灰度, 2 RGB, 6 RGBA}。截图（`adb screencap` / 开发者工具）都落在这个子集里。
//
// 用法：
//   node png-diff.mjs --a <baseline.png> --b <current.png> [--threshold 0.005]
//                     [--ignore-top-rows N] [--ignore-bottom-rows N] [--tolerance N]
// 输出：**单行 JSON**
//   成功判据：{"ok":true,"mode":"pixel","width":W,"height":H,"total":N,"different":D,
//              "ratio":R,"threshold":T,"changed":bool,"ignoredRows":{"top":x,"bottom":y}}
//   回退判据：{"ok":false,"mode":"unsupported","reason":"<为什么>"}
// 退出码：0 = 给了结论（changed 与否在 JSON 里，不看退出码）；2 = 不支持（调用方回退 MD5）；3 = 参数/IO 错。
//
// ⚠️ 判据只在 `ok:true` 时可用；`ok:false` 时**不得**当成「无变化」—— 那是假绿。

import fs from 'node:fs';
import zlib from 'node:zlib';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const PNG_SIG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

/** 解出 8-bit 非交错 PNG → {width,height,channels,data}；不支持/损坏一律抛 Error（由调用方转 unsupported）。 */
export function decodePng(buf) {
  if (!Buffer.isBuffer(buf) || buf.length < 8 + 25) throw new Error('not-a-png(too-short)');
  if (!buf.subarray(0, 8).equals(PNG_SIG)) throw new Error('not-a-png(bad-signature)');

  let pos = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  let interlace = 0;
  let sawIHDR = false;
  const idat = [];

  while (pos + 8 <= buf.length) {
    const len = buf.readUInt32BE(pos);
    const type = buf.toString('ascii', pos + 4, pos + 8);
    const dataStart = pos + 8;
    if (dataStart + len + 4 > buf.length) throw new Error('chunk-truncated:' + type);
    if (type === 'IHDR') {
      if (len !== 13) throw new Error('bad-IHDR-length');
      width = buf.readUInt32BE(dataStart);
      height = buf.readUInt32BE(dataStart + 4);
      bitDepth = buf[dataStart + 8];
      colorType = buf[dataStart + 9];
      // dataStart+10 = compression, +11 = filter, +12 = interlace
      interlace = buf[dataStart + 12];
      sawIHDR = true;
    } else if (type === 'IDAT') {
      idat.push(buf.subarray(dataStart, dataStart + len));
    } else if (type === 'IEND') {
      break;
    }
    pos = dataStart + len + 4; // + 4 = CRC
  }

  if (!sawIHDR) throw new Error('no-IHDR');
  if (width <= 0 || height <= 0) throw new Error('bad-dimensions');
  if (bitDepth !== 8) throw new Error('unsupported-bit-depth:' + bitDepth);
  if (interlace !== 0) throw new Error('unsupported-interlace:' + interlace);
  if (colorType !== 0 && colorType !== 2 && colorType !== 6) throw new Error('unsupported-color-type:' + colorType);
  if (idat.length === 0) throw new Error('no-IDAT');

  const channels = colorType === 0 ? 1 : colorType === 2 ? 3 : 4;
  let raw;
  try {
    raw = zlib.inflateSync(Buffer.concat(idat));
  } catch (e) {
    throw new Error('inflate-failed:' + (e && e.message));
  }

  const stride = width * channels;
  const need = (stride + 1) * height;
  if (raw.length < need) throw new Error('inflated-too-short:' + raw.length + '<' + need);

  const out = Buffer.alloc(stride * height);
  let rp = 0;
  for (let y = 0; y < height; y += 1) {
    const ft = raw[rp];
    rp += 1;
    const row = out.subarray(y * stride, (y + 1) * stride);
    const prev = y > 0 ? out.subarray((y - 1) * stride, y * stride) : null;
    raw.copy(row, 0, rp, rp + stride);
    rp += stride;

    if (ft === 0) continue;
    if (ft === 1) {
      for (let i = channels; i < stride; i += 1) row[i] = (row[i] + row[i - channels]) & 0xff;
    } else if (ft === 2) {
      if (!prev) throw new Error('filter-type-2-on-first-row');
      for (let i = 0; i < stride; i += 1) row[i] = (row[i] + prev[i]) & 0xff;
    } else if (ft === 3) {
      for (let i = 0; i < stride; i += 1) {
        const a = i >= channels ? row[i - channels] : 0;
        const b = prev ? prev[i] : 0;
        row[i] = (row[i] + ((a + b) >> 1)) & 0xff;
      }
    } else if (ft === 4) {
      for (let i = 0; i < stride; i += 1) {
        const a = i >= channels ? row[i - channels] : 0;
        const b = prev ? prev[i] : 0;
        const c = (prev && i >= channels) ? prev[i - channels] : 0;
        const pa = Math.abs(b - c);
        const pb = Math.abs(a - c);
        const pc = Math.abs(a + b - 2 * c);
        const pr = (pa <= pb && pa <= pc) ? a : (pb <= pc ? b : c);
        row[i] = (row[i] + pr) & 0xff;
      }
    } else {
      throw new Error('unknown-filter-type:' + ft);
    }
  }

  return { width, height, channels, data: out };
}

/** 两图逐像素比：任一通道差 > tolerance 即算该像素「不同」。返回判定对象（不抛，除参数错）。 */
export function diffPng(aBuf, bBuf, opts = {}) {
  const threshold = opts.threshold === undefined ? 0.005 : Number(opts.threshold);
  const tolerance = opts.tolerance === undefined ? 0 : Number(opts.tolerance);
  const ignoreTop = Math.max(0, Number(opts.ignoreTopRows || 0));
  const ignoreBottom = Math.max(0, Number(opts.ignoreBottomRows || 0));

  let A;
  let B;
  try {
    A = decodePng(aBuf);
    B = decodePng(bBuf);
  } catch (e) {
    return { ok: false, mode: 'unsupported', reason: String((e && e.message) || e) };
  }
  if (A.width !== B.width || A.height !== B.height) {
    return { ok: false, mode: 'unsupported', reason: `size-mismatch:${A.width}x${A.height}vs${B.width}x${B.height}` };
  }

  const y0 = Math.min(ignoreTop, A.height);
  const y1 = Math.max(y0, A.height - ignoreBottom);
  let different = 0;
  const total = A.width * (y1 - y0);

  for (let y = y0; y < y1; y += 1) {
    const base = y * A.width * A.channels;
    for (let x = 0; x < A.width; x += 1) {
      const i = base + x * A.channels;
      let differs = false;
      if (tolerance === 0) {
        for (let c = 0; c < A.channels; c += 1) {
          if (A.data[i + c] !== B.data[i + c]) { differs = true; break; }
        }
      } else {
        for (let c = 0; c < A.channels; c += 1) {
          if (Math.abs(A.data[i + c] - B.data[i + c]) > tolerance) { differs = true; break; }
        }
      }
      if (differs) different += 1;
    }
  }

  const ratio = total === 0 ? 0 : different / total;
  return {
    ok: true,
    mode: 'pixel',
    width: A.width,
    height: A.height,
    total,
    different,
    ratio: Number(ratio.toFixed(6)),
    threshold,
    changed: ratio > threshold,
    ignoredRows: { top: y0, bottom: A.height - y1 },
  };
}

function parseArgs(argv) {
  const out = { a: '', b: '', threshold: '0.005', tolerance: '0', ignoreTopRows: '0', ignoreBottomRows: '0' };
  for (let i = 0; i < argv.length; i += 1) {
    const k = argv[i];
    const v = argv[i + 1];
    if (k === '--a') { out.a = v; i += 1; } else if (k === '--b') { out.b = v; i += 1; } else if (k === '--threshold') { out.threshold = v; i += 1; } else if (k === '--tolerance') { out.tolerance = v; i += 1; } else if (k === '--ignore-top-rows') { out.ignoreTopRows = v; i += 1; } else if (k === '--ignore-bottom-rows') { out.ignoreBottomRows = v; i += 1; } else if (k === '--help' || k === '-h') { out.help = true; }
  }
  return out;
}

const isMain = process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href;
if (isMain) {
  const args = parseArgs(process.argv.slice(2));
  if (args.help || !args.a || !args.b) {
    console.log(JSON.stringify({ ok: false, mode: 'usage', reason: '用法：node png-diff.mjs --a <png> --b <png> [--threshold 0.005] [--ignore-top-rows N] [--tolerance N]' }));
    process.exit(3);
  }
  let aBuf;
  let bBuf;
  try {
    aBuf = fs.readFileSync(args.a);
    bBuf = fs.readFileSync(args.b);
  } catch (e) {
    console.log(JSON.stringify({ ok: false, mode: 'unsupported', reason: 'io:' + String((e && e.message) || e) }));
    process.exit(2);
  }
  const r = diffPng(aBuf, bBuf, {
    threshold: args.threshold,
    tolerance: args.tolerance,
    ignoreTopRows: args.ignoreTopRows,
    ignoreBottomRows: args.ignoreBottomRows,
  });
  console.log(JSON.stringify(r));
  process.exit(r.ok ? 0 : 2);
}
