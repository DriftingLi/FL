import { readdirSync, statSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
const ROOT = 'D:/FL';
const SKIP = new Set(['node_modules', '.git', 'unpackage', 'kotlin-class', 'dist', '.qoder', 'kotlin-build', '.gradle']);
const files = [];
(function walk(d, depth) {
  if (depth > 9) return;
  let ents; try { ents = readdirSync(d, { withFileTypes: true }); } catch { return; }
  for (const e of ents) {
    if (SKIP.has(e.name)) continue;
    const p = join(d, e.name);
    if (e.isDirectory()) walk(p, depth + 1);
    else if (e.isFile()) {
      let st; try { st = statSync(p); } catch { continue; }
      if (st.size === 0 || st.size > 40 * 1024 * 1024) continue;
      files.push(p);
    }
  }
})(ROOT, 0);
console.log('files', files.length);
const sc = files.filter((p) => p.includes('.scratch'));
console.log('含 .scratch 的路径', sc.length, '其中有 wt1472-full1 吗:', sc.some((p) => /wt1472-full1/.test(p)));
let hits = 0, wt = [];
for (const p of files) {
  let buf; try { buf = readFileSync(p); } catch { continue; }
  if (!buf.includes('Test Suites:')) continue;
  hits++;
  if (/wt1472/.test(p)) wt.push(p);
}
console.log('Test Suites 命中文件', hits, 'wt1472 系列:', wt.join(' '));
