// #1650 截图裁剪器（throwaway）：把 PNG 按行区间裁出小图，供 read_image 目检。
// 判读动机：亮度剖面只能告诉我「哪段亮/暗」，认不出「那段是不是 tab bar / 面板标题」；
//   三形态（full 满屏面板 / drawer 底部抽屉 / anchor 贴顶面板）的差别恰恰在**遮挡关系**上
//   （谁盖住了 tab bar、谁盖住了横幅），必须看图定案。
// 用法：node scripts/png-crop.mjs <in.png> <y0>-<y1> <out.png> [...]
import fs from 'node:fs';
import zlib from 'node:zlib';

function decodePng(buf) {
  let off = 8;
  let ihdr = null;
  const idat = [];
  while (off < buf.length) {
    const len = buf.readUInt32BE(off); off += 4;
    const type = buf.toString('ascii', off, off + 4); off += 4;
    const data = buf.subarray(off, off + len); off += len + 4;
    if (type === 'IHDR') ihdr = { width: data.readUInt32BE(0), height: data.readUInt32BE(4), bitDepth: data[8], colorType: data[9], interlace: data[12] };
    else if (type === 'IDAT') idat.push(data);
    else if (type === 'IEND') break;
  }
  if (ihdr.bitDepth !== 8) throw new Error('bitDepth ' + ihdr.bitDepth);
  if (ihdr.interlace !== 0) throw new Error('interlace');
  const bpp = ihdr.colorType === 6 ? 4 : ihdr.colorType === 2 ? 3 : ihdr.colorType === 0 ? 1 : 0;
  if (!bpp) throw new Error('colorType ' + ihdr.colorType);
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

function crc32(buf) {
  let c = ~0;
  for (let i = 0; i < buf.length; i += 1) {
    c ^= buf[i];
    for (let k = 0; k < 8; k += 1) c = (c >>> 1) ^ (0xEDB88320 & -(c & 1));
  }
  return (~c) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length, 0);
  const td = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(td), 0);
  return Buffer.concat([len, td, crc]);
}
function encodePng(width, height, rgb) {
  const stride = width * 3;
  const raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y += 1) {
    raw[y * (stride + 1)] = 0;
    rgb.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0); ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; ihdr[9] = 2; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const [inFile, range, outFile] = process.argv.slice(2);
const m = range.match(/^(\d+)-(\d+)$/);
if (!m) { console.error('range 形如 150-350'); process.exit(1); }
const y0 = Number(m[1]); const y1 = Number(m[2]);
const img = decodePng(fs.readFileSync(inFile));
const h = Math.min(img.height, y1) - y0;
if (h <= 0) { console.error('空区间'); process.exit(1); }
const outStride = img.width * 3;
const rgb = Buffer.alloc(outStride * h);
for (let y = 0; y < h; y += 1) {
  const src = img.px.subarray((y0 + y) * img.width * img.bpp, (y0 + y + 1) * img.width * img.bpp);
  for (let x = 0; x < img.width; x += 1) {
    rgb.set([src[x * img.bpp], src[x * img.bpp + 1], src[x * img.bpp + 2]], (y * img.width + x) * 3);
  }
}
fs.writeFileSync(outFile, encodePng(img.width, h, rgb));
console.log('wrote ' + outFile + ' ' + img.width + 'x' + h + ' (rows ' + y0 + '-' + (y0 + h - 1) + ')');
