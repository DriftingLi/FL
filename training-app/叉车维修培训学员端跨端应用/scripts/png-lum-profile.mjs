// #1650 截图客观判读（throwaway）：纯像素统计，不依赖任何视觉模型。
// 判据来源（cert-selector.uvue 三形态规格）：
//   full   = 深色全屏遮罩（fixed 满屏 + 面板随*v-if 挂载）
//   drawer = 底部半屏抽屉，无遮罩或浅遮罩，面板贴底、上圆角
//   anchor = 浅遮罩 rgba(0,0,0,0.15) + 贴顶面板、下圆角
// 用法：node scripts/png-lum-profile.mjs <png...>
// 输出每图：行带亮度剖面（20 带）、暗像素占比、实心白面板行范围、分类建议。
import fs from 'node:fs';
import zlib from 'node:zlib';

function decodePng(buf) {
  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  if (!buf.subarray(0, 8).equals(sig)) throw new Error('not png');
  let off = 8;
  let ihdr = null;
  const idat = [];
  while (off < buf.length) {
    const len = buf.readUInt32BE(off); off += 4;
    const type = buf.toString('ascii', off, off + 4); off += 4;
    const data = buf.subarray(off, off + len); off += len + 4;
    if (type === 'IHDR') {
      ihdr = {
        width: data.readUInt32BE(0), height: data.readUInt32BE(4),
        bitDepth: data[8], colorType: data[9], interlace: data[12],
      };
    } else if (type === 'IDAT') idat.push(data);
    else if (type === 'IEND') break;
  }
  if (!ihdr) throw new Error('no IHDR');
  if (ihdr.bitDepth !== 8) throw new Error('bitDepth ' + ihdr.bitDepth + ' unsupported');
  if (ihdr.interlace !== 0) throw new Error('interlace unsupported');
  const bpp = ihdr.colorType === 6 ? 4 : ihdr.colorType === 2 ? 3 : ihdr.colorType === 0 ? 1 : 0;
  if (!bpp) throw new Error('colorType ' + ihdr.colorType + ' unsupported');
  const raw = zlib.inflateSync(Buffer.concat(idat));
  const stride = ihdr.width * bpp;
  const out = Buffer.alloc(stride * ihdr.height);
  let pos = 0;
  for (let y = 0; y < ihdr.height; y += 1) {
    const f = raw[pos]; pos += 1;
    const line = raw.subarray(pos, pos + stride); pos += stride;
    const cur = out.subarray(y * stride, (y + 1) * stride);
    const prev = y > 0 ? out.subarray((y - 1) * stride, y * stride) : null;
    for (let x = 0; x < stride; x += 1) {
      const a = x >= bpp ? cur[x - bpp] : 0;
      const b = prev ? prev[x] : 0;
      const c = prev && x >= bpp ? prev[x - bpp] : 0;
      let v = line[x];
      if (f === 1) v = (v + a) & 0xff;
      else if (f === 2) v = (v + b) & 0xff;
      else if (f === 3) v = (v + ((a + b) >> 1)) & 0xff;
      else if (f === 4) {
        const p = a + b - c;
        const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
        v = (v + (pa <= pb && pa <= pc ? a : pb <= pc ? b : c)) & 0xff;
      }
      cur[x] = v;
    }
  }
  return { width: ihdr.width, height: ihdr.height, bpp, px: out };
}

function lum(r, g, b) { return 0.299 * r + 0.587 * g + 0.114 * b; }

function profile(file) {
  const { width, height, bpp, px } = decodePng(fs.readFileSync(file));
  const bands = 20;
  const bandH = Math.ceil(height / bands);
  const rows = [];
  for (let y = 0; y < height; y += 1) {
    let sum = 0; let sumSq = 0; const n = width;
    for (let x = 0; x < width; x += 1) {
      const i = (y * width + x) * bpp;
      const l = lum(px[i], px[i + 1], px[i + 2]);
      sum += l; sumSq += l * l;
    }
    const mean = sum / n;
    rows.push({ mean, sd: Math.sqrt(Math.max(0, sumSq / n - mean * mean)) });
  }
  // 实心白面板行：mean>228 且 sd<14（遮罩上的内容行 sd 大、纯白面板 sd 极小）
  const panelRows = [];
  for (let y = 0; y < height; y += 1) {
    if (rows[y].mean > 228 && rows[y].sd < 14) panelRows.push(y);
  }
  let panelTop = -1; let panelBottom = -1;
  if (panelRows.length) {
    // 取最长连续段
    let bestStart = panelRows[0]; let bestLen = 1; let start = panelRows[0]; let len = 1;
    for (let i = 1; i < panelRows.length; i += 1) {
      if (panelRows[i] === panelRows[i - 1] + 1) len += 1;
      else { if (len > bestLen) { bestLen = len; bestStart = start; } start = panelRows[i]; len = 1; }
    }
    if (len > bestLen) { bestLen = len; bestStart = start; }
    panelTop = bestStart; panelBottom = bestStart + bestLen - 1;
  }
  // 非面板行（跳过状态栏 0-14 与面板段）的中位亮度 = 遮罩强度代理
  const nonPanel = [];
  for (let y = 15; y < height; y += 1) {
    if (panelTop >= 0 && y >= panelTop && y <= panelBottom) continue;
    nonPanel.push(rows[y].mean);
  }
  nonPanel.sort((a, b) => a - b);
  const median = nonPanel.length ? nonPanel[Math.floor(nonPanel.length / 2)] : -1;
  const darkFrac = nonPanel.filter((v) => v < 90).length / Math.max(1, nonPanel.length);
  const bandMeans = [];
  for (let b = 0; b < bands; b += 1) {
    let s = 0; let c = 0;
    for (let y = b * bandH; y < Math.min(height, (b + 1) * bandH); y += 1) { s += rows[y].mean; c += 1; }
    bandMeans.push(Math.round(s / Math.max(1, c)));
  }
  return { file, width, height, median: Math.round(median), darkFrac: Math.round(darkFrac * 100) / 100, panelTop, panelBottom, panelFrac: panelTop >= 0 ? Math.round((panelBottom - panelTop + 1) / height * 100) : 0, bandMeans };
}

for (const f of process.argv.slice(2)) {
  try {
    const p = profile(f);
    const base = f.split(/[\\/]/).pop();
    const mask = p.darkFrac > 0.6 ? 'DARK' : p.median < 195 ? 'SHALLOW' : 'NONE';
    const where = p.panelTop < 0 ? 'no-panel' : p.panelTop < p.height * 0.25 ? 'TOP' : p.panelTop > p.height * 0.6 ? 'BOTTOM' : 'MID';
    const guess = mask === 'DARK' ? 'full?' : where === 'BOTTOM' ? 'drawer?' : where === 'TOP' && mask === 'SHALLOW' ? 'anchor?' : '?';
    console.log(base.padEnd(22) + ' median=' + String(p.median).padStart(3) + ' darkFrac=' + String(p.darkFrac).padEnd(4)
      + ' panel=[' + p.panelTop + '..' + p.panelBottom + '] ' + String(p.panelFrac).padStart(3) + '% ' + where.padEnd(9)
      + ' mask=' + mask.padEnd(7) + ' => ' + guess);
    console.log('    bands: ' + p.bandMeans.join(' '));
  } catch (e) {
    console.log(f + ' ERROR ' + String(e));
  }
}
