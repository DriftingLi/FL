#!/usr/bin/env node
/**
 * api seam 守卫（ADR-0053 §7 / spec #1049）。
 *
 * 背景：前端有一层请求层（frontend/src/api/*.ts，薄薄一层 typed 转发）。但页面层曾出现
 * 「绕过请求层直接打端点」的写法——6 个页面 18 个调用点，每个都是手拼 URL + `any` 类型，
 * 同一个端点在很多文件里各打一遍。这次把它们收回 api 模块后，本脚本把「页面/组件不得
 * 直接引用请求层」变成 CI 可核验的事实。
 *
 * 用法：
 *   node scripts/check-api-seam.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-api-seam.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *
 * 判定面：只扫**页面与业务组件**（`/pages/` 与 `/components/` 下的 .vue / .ts）的
 * import / export-from / 动态 import 语句里的模块说明符。
 *   —— 只认「引用了请求层」这一条可精确判定的规则；不扫 URL 字符串字面量（那会误伤
 *      文档、注释与合法的外链常量）。
 *
 * 放行面（有意不进守卫集）：
 *   1. frontend/src/api/** —— 请求层自身与它的 api 模块；
 *   2. __tests__ / *.spec.* / *.test.* —— 测试要对请求层做 mock，按定义就得 import 它；
 *   3. 其余目录（utils / composables / stores …）—— 本守卫的适用面是页面与业务组件。
 * 白名单（ALLOWLIST）初始为空：确有例外时逐条登记并写明理由，不要放宽规则本身。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const DEFAULT_SCAN_DIR = join(ROOT, 'frontend', 'src')

/** 请求层模块（页面不得直接引用）。含 `@/api/request` 与相对写法 `../api/request`。 */
export const GUARDED_MODULES = ['api/request', 'api/client']

/** 守卫面：路径含这些片段才算判定对象。 */
export const GUARDED_PATH_SEGMENTS = ['/pages/', '/components/']

/** 扫描面后缀。 */
export const SCAN_EXTENSIONS = ['.vue', '.ts']

/**
 * 逐条登记的有意例外（路径 → 理由）。收口本身把 18 处调用点全部收回 api 模块，
 * 没有为它们预留豁免；本表只有「跑守卫时实际发现、且经判定不属本次规则射程」的一条。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {
  'frontend/src/components/recruit/OnlineResumePdf.vue':
    '只用 getValidAccessToken()（取访问令牌的只读辅助）给原生 fetch 的 PDF 端点加鉴权头，' +
    '不经请求层发请求；待该组件改用 api 层的 blob 方法后删除本条。'
}

/** 测试文件不进判定面（按定义要 mock 请求层）。 */
export function isTestFile(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes('/__tests__/') || /\.(spec|test)\.[jt]s$/.test(p)
}

/** 路径是否在守卫面（页面 / 业务组件，且不是测试、不在白名单）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  if (isTestFile(p)) return false
  if (!GUARDED_PATH_SEGMENTS.some((seg) => p.includes(seg))) return false
  return !Object.prototype.hasOwnProperty.call(ALLOWLIST, p)
}

/** 模块说明符是否指向请求层（支持 `@/api/request`、`../api/request`、`./api/client` 写法）。 */
export function guardedSpecifier(spec) {
  const s = String(spec).replace(/\\/g, '/')
  return GUARDED_MODULES.find((m) => s === '@/' + m || s === m || s.endsWith('/' + m)) || null
}

const SPECIFIER_RE = /(?:^|\s)(?:import|export)\b[^'"`]*?['"]([^'"]+)['"]|import\s*\(\s*['"]([^'"]+)['"]\s*\)|require\s*\(\s*['"]([^'"]+)['"]\s*\)/g

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 命中的模块说明符 + 原文）。
 * 只认 import / export-from / 动态 import / require 的说明符。
 *
 * **路径不在守卫面即整体放行**（api 层自身、测试、白名单、非页面/组件目录）——
 * 判定收敛在这里，调用方不必自己记得先过滤（少一个「忘了过滤就假红」的面）。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    // 廉价预筛：三种语句形态都覆盖（export-from 行里没有 import 关键字）
    if (!/import|require|export/.test(line)) continue
    SPECIFIER_RE.lastIndex = 0
    let m
    while ((m = SPECIFIER_RE.exec(line)) !== null) {
      const spec = m[1] ?? m[2] ?? m[3]
      if (!spec) continue
      const guarded = guardedSpecifier(spec)
      if (guarded) {
        violations.push({ file, line: i + 1, spec, module: guarded, text: line.trim() })
      }
    }
  }
  return violations
}

function walk(dir) {
  const out = []
  let entries
  try {
    entries = readdirSync(dir, { withFileTypes: true })
  } catch {
    return out
  }
  for (const e of entries) {
    const p = join(dir, e.name)
    if (e.isDirectory()) {
      if (e.name === 'node_modules') continue
      out.push(...walk(p))
      continue
    }
    if (SCAN_EXTENSIONS.some((ext) => e.name.endsWith(ext))) out.push(p)
  }
  return out
}

function relToRoot(abs) {
  return relative(ROOT, abs).replace(/\\/g, '/')
}

function reportAll(scanDir) {
  const files = walk(scanDir)
  const violations = []
  let checked = 0
  for (const abs of files) {
    const rel = relToRoot(abs)
    if (!isGuardedPath(rel)) continue
    checked++
    violations.push(...scanSource(readFileSync(abs, 'utf8'), rel))
  }
  console.log('===== api seam 守卫：全量扫描（页面与业务组件不得直接引用请求层）=====')
  console.log('扫描目录: ' + relToRoot(scanDir))
  console.log('守卫面: ' + GUARDED_PATH_SEGMENTS.join(' / ') + '（跳过测试与白名单）')
  console.log('请求层: ' + GUARDED_MODULES.map((m) => '@/' + m).join(' / '))
  console.log('---')
  if (violations.length === 0) {
    console.log('无违规。' + checked + ' 个文件均未直接引用请求层。')
    return 0
  }
  for (const v of violations) console.log(v.file + ':' + v.line + ': ' + v.spec + '  ' + v.text)
  console.log('---')
  console.log('共 ' + violations.length + ' 处。请在 frontend/src/api/ 下补具名方法，页面只调它：')
  console.log('  —— 响应类型取 `@/api/generated/*`（后端注解 → swagger → go run ./cmd/gen-apitypes），不手写。')
  return 1
}

/** 解析 git diff 的新增行 → Map<相对路径, Set<行号>>（逐行走 hunk，上下文行也推进新侧行号）。 */
export function parseAddedLines(diffText) {
  const added = new Map()
  let file = null
  let lineNo = 0
  let inHunk = false
  for (const line of String(diffText).split('\n')) {
    const f = line.match(/^\+\+\+ b\/(.+)$/)
    if (f) {
      file = f[1]
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

function reportDiff(base) {
  try {
    execFileSync('git', ['rev-parse', '--verify', base], { cwd: ROOT, stdio: 'ignore' })
  } catch {
    console.error('[check-api-seam] 无法解析 base ref: ' + base + '（CI 上请先 git fetch）')
    return 2
  }
  const diff = execFileSync('git', ['diff', '-U0', base + '...HEAD', '--', '*.vue', '*.ts'], {
    cwd: ROOT,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024
  })
  const added = parseAddedLines(diff)
  if (added.size === 0) {
    console.log('[check-api-seam] 相对 ' + base + ' 无 .vue/.ts 新增行，跳过。')
    return 0
  }
  const violations = []
  for (const [file, lines] of added) {
    if (!isGuardedPath(file)) continue
    let source
    try {
      source = readFileSync(join(ROOT, file), 'utf8')
    } catch {
      continue // 删除的文件
    }
    for (const v of scanSource(source, file)) {
      if (lines.has(v.line)) violations.push(v)
    }
  }
  if (violations.length === 0) {
    console.log('[check-api-seam] 新增行未直接引用请求层，通过。')
    return 0
  }
  console.error('===== 新增行直接引用了请求层（页面与业务组件一律走 api/ 具名方法）=====')
  for (const v of violations) console.error(v.file + ':' + v.line + ': ' + v.spec + '  ' + v.text)
  console.error('---')
  console.error('共 ' + violations.length + ' 处。见 ADR-0053 §7 与 docs/agents/ui-conventions.md。')
  return 1
}

function main(argv) {
  const mode = argv[0] ?? '--all'
  if (mode === '--all') {
    const dir = argv[1] ? resolve(argv[1]) : DEFAULT_SCAN_DIR
    return reportAll(dir)
  }
  if (mode === '--diff') {
    return reportDiff(argv[1] ?? 'origin/master')
  }
  console.error('用法: node scripts/check-api-seam.mjs --all [目录] | --diff [base]')
  return 2
}

// 仅作为 CLI 直接运行时才执行：被 import（自检里喂源码文本）时不产生副作用。
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
