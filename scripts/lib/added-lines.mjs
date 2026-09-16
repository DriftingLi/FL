#!/usr/bin/env node
/**
 * 「git diff 新增行」解析的**单点实现**（ADR-0053 §10 / spec #1053）。
 *
 * 背景：仓库里有两个守卫（`check-el-controls.mjs` 的 `--diff`、`scripts/check-bare-hex.sh`
 * 的 `--diff`）各自实现了一遍「从 `git diff -U0` 里取出新增行号」，而换行与**路径转义**这两处
 * 的坑只被其中一套处理了：git 在 `core.quotepath=true`（默认）下会把含非 ASCII 的路径写成
 * `"b/\346\226\207\344\273\266.vue"` 这种八进制转义形态 —— 拿它去 open 会静默找不到文件，
 * 守卫于是**静默漏扫**（同仓的移动端脚本已经因此咬过一次）。
 *
 * 用法：
 *   Node 侧：import { parseAddedLines, addedLineList, decodeGitPath } from '../lib/added-lines.mjs'
 *   Shell 侧：node scripts/lib/added-lines.mjs --diff <base> [--pathspec '<glob>']
 *             输出 `file:lineno`（已排序去重），供 shell 守卫与 `cut` / `awk` 消费。
 *
 * 解析口径（与两个守卫原先逐字一致）：逐行走 hunk，`+++ b/…` 换文件、`@@ -a[,b] +c[,d] @@`
 * 重置新侧行号；`+` 行计入并推进，`-` 行不推进，上下文行（含空行）推进，`\ No newline…` 跳过。
 */
import { execFileSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')

/**
 * 解码 git 在 core.quotepath 下输出的 C 风格转义路径。
 * 非引号形态原样返回；引号形态按「八进制 → 字节 → UTF-8」还原（不能按字符还原：一个
 * 中文字在转义里是 3 个八进制字节）。
 */
export function decodeGitPath(raw) {
  const s = String(raw)
  if (!s.startsWith('"')) return s
  const inner = s.endsWith('"') ? s.slice(1, -1) : s.slice(1)
  const bytes = []
  for (let i = 0; i < inner.length; i++) {
    const ch = inner[i]
    if (ch !== '\\') {
      bytes.push(...Buffer.from(ch, 'utf8'))
      continue
    }
    const next = inner[++i]
    if (next === undefined) break
    if (next >= '0' && next <= '7') {
      let oct = next
      while (oct.length < 3 && inner[i + 1] >= '0' && inner[i + 1] <= '7') oct += inner[++i]
      bytes.push(parseInt(oct, 8))
      continue
    }
    const simple = { t: 9, n: 10, r: 13, a: 7, b: 8, f: 12, v: 11, '\\': 92, '"': 34 }
    if (Object.prototype.hasOwnProperty.call(simple, next)) {
      bytes.push(simple[next])
      continue
    }
    bytes.push(...Buffer.from(next, 'utf8'))
  }
  return Buffer.from(bytes).toString('utf8')
}

/** 解析 diff 的新增行 → Map<相对路径, Set<行号>>（路径已解码为仓库相对路径）。 */
export function parseAddedLines(diffText) {
  const added = new Map()
  let file = null
  let lineNo = 0
  let inHunk = false
  for (const line of String(diffText).split('\n')) {
    const f = line.match(/^\+\+\+ (.*)$/)
    if (f) {
      const raw = decodeGitPath(f[1])
      if (raw === '/dev/null') {
        file = null
        inHunk = false
        continue
      }
      file = raw.replace(/^b\//, '')
      if (!added.has(file)) added.set(file, new Set())
      inHunk = false
      continue
    }
    const h = line.match(/^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@/)
    if (h) {
      lineNo = Number(h[1])
      inHunk = true
      continue
    }
    if (!inHunk || !file) continue
    if (line.startsWith('+')) {
      if (line.startsWith('+++')) continue
      added.get(file).add(lineNo)
      lineNo++
      continue
    }
    if (line.startsWith('-')) continue // 删除行不影响新侧行号
    if (line.startsWith('\\')) continue // "\ No newline at end of file"
    lineNo++ // 上下文行（含空行）推进新侧行号
  }
  return added
}

/** 新增行清单（`file:lineno`，已排序）——shell 侧消费的形态。 */
export function addedLineList(diffText) {
  const out = []
  for (const [file, lines] of parseAddedLines(diffText)) {
    for (const ln of lines) out.push(file + ':' + ln)
  }
  return out.sort()
}

/** 取某 base 到 HEAD 的 diff 文本（`core.quotepath=false` 让路径免于转义；解码器仍作兜底）。 */
export function gitDiff(base, pathspec, cwd = ROOT) {
  const spec = pathspec ? ['--', pathspec] : []
  const run = (rangeArgs) =>
    execFileSync(
      'git',
      ['-c', 'core.quotepath=false', 'diff', '-U0', ...rangeArgs, ...spec],
      { cwd, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }
    )
  const run2 = (a, b) => run([a, b])
  try {
    // 首选三点：以「分支基点」为界，不含 master 在分支切出之后的新增
    return run([base + '...HEAD'])
  } catch (e) {
    // 浅克隆 + 浅 fetch 的 CI 里没有 merge base（"no merge base"）——
    // 退化为双点（直接比两棵树，不需要历史）。两点在「分支包含其基点」时与三点等价；
    // 分支落后 master 时会把 master 的新增当成相对 HEAD 的改动，由分支必须 up-to-date 的
    // 合并规则兜住。
    const detail = [e && e.stderr, e && e.message].filter(Boolean).join(' | ')
    if (!/no merge base/.test(detail)) {
      throw new Error('git diff 失败: ' + detail)
    }
    try {
      // 双点：两个 revision 必须是独立参数（'base HEAD' 会被 git 当成一个 revision 名）
      return run2(base, 'HEAD')
    } catch (e2) {
      throw new Error('git diff 失败: ' + [e2 && e2.stderr, e2 && e2.message].filter(Boolean).join(' | '))
    }
  }
}

function main(argv) {
  const mode = argv[0]
  if (mode !== '--diff') {
    console.error('用法: node scripts/lib/added-lines.mjs --diff <base> [--pathspec <glob>]')
    return 2
  }
  const base = argv[1] ?? 'origin/master'
  let pathspec = ''
  const pIdx = argv.indexOf('--pathspec')
  if (pIdx >= 0) pathspec = argv[pIdx + 1] ?? ''

  try {
    execFileSync('git', ['rev-parse', '--verify', base], { cwd: ROOT, stdio: 'ignore' })
  } catch {
    console.error('[added-lines] 无法解析 base ref: ' + base + '（CI 上请先 git fetch）')
    return 2
  }
  let diff = ''
  try {
    diff = gitDiff(base, pathspec)
  } catch (e) {
    // 失败必须可见（判据坏了装作没问题是假绿）：打出 git 的 stderr
    console.error('[added-lines] ' + (e && e.message ? e.message : String(e)))
    return 2
  }
  const list = addedLineList(diff)
  if (list.length) process.stdout.write(list.join('\n') + '\n')
  return 0
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
