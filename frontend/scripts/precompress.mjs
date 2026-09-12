#!/usr/bin/env node
/**
 * 构建侧静态资源预压缩（#791 / spec #788）。
 *
 * 为什么：生产静态资源此前由宿主机 nginx 实时 gzip（gzip on + level 6）——
 * 每个请求现压一遍、压缩率被压在 6。产物是静态的，压一次就够，且可以用最高档。
 *
 * 用法：npm run build 之后自动执行（见 package.json 的 build 脚本）；
 *       也可单独 `node scripts/precompress.mjs`。
 *
 * 产物：dist/ 下每个 .js/.css/.svg/.json 旁的 同名 .gz（同源文件一一对应）。
 * 幂等：已存在的 .gz 直接覆盖重写，重复执行结果一致。
 * 不改动原文件本身，只新增 .gz。
 */
import { readdirSync, statSync, readFileSync, writeFileSync } from 'node:fs'
import { join, extname } from 'node:path'
import { gzipSync, constants } from 'node:zlib'

const DIST = join(process.cwd(), 'dist')
const TARGET_EXT = new Set(['.js', '.css', '.svg', '.json'])
// gzip_min_length 1024（nginx 侧）：小于此值的文件不预压，避免产出无意义的 .gz
const MIN_SIZE = 1024

/** 递归收集目标文件 */
function walk(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) out.push(...walk(p))
    else if (TARGET_EXT.has(extname(name)) && st.size >= MIN_SIZE) out.push(p)
  }
  return out
}

const files = walk(DIST)
let before = 0
let after = 0

for (const file of files) {
  const src = readFileSync(file)
  const gz = gzipSync(src, { level: constants.Z_BEST_COMPRESSION })
  writeFileSync(`${file}.gz`, gz)
  before += src.length
  after += gz.length
}

const pct = before ? ((1 - after / before) * 100).toFixed(1) : '0'
console.log(
  `[precompress] ${files.length} 个文件预压完成：` +
    `${(before / 1024).toFixed(0)} KB → ${(after / 1024).toFixed(0)} KB（省 ${pct}%），` +
    `产物 dist/**/*.gz`
)
