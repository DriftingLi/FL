#!/usr/bin/env node
/**
 * 测试影响面映射（**诊断工具，不是门禁**）
 *
 * 为什么有它：移动端 ③ 门单腿约 150 套件 / 3200+ 用例且 `-i` 串行，而 `pages/**` 的样式类改动
 * 只可能命中「读源码文本做断言」的那几个契约套件。每次改完都跑全量，是本轮整改里最大的单步耗时。
 * 本脚本把「改哪个页面 ⇒ 该跑哪几个套件」变成可查的静态事实：扫 `utils/*.test.js` 里对
 * `pages/… / components/… / uni.scss / pages.json` 的**字面量引用**（含 `path.join(ROOT, …)` 拆分写法），
 * 按 `scripts/lib/guard-split.js` 的同一判据标出套件落在哪条腿。
 *
 * 用法：
 *   node scripts/test-impact-map.mjs                       # 概览：套件数 / 腿分布 / 引用面
 *   node scripts/test-impact-map.mjs --json                # 全量映射（机读）
 *   node scripts/test-impact-map.mjs pages/x/y.uvue …      # 查指定文件的受影响套件
 *   node scripts/test-impact-map.mjs --changed [base]      # 按 `git status --porcelain` 算改动面并给出 jest 命令
 *
 * 退出码：0 = 完成；2 = 用法错 / 读目录失败（fail-closed）
 */

import { readFileSync, readdirSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import guardSplit from './lib/guard-split.js'

const MOBILE_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const UTILS_DIR = join(MOBILE_ROOT, 'utils')
/** `git status --porcelain` 的输出**恒相对仓库根**（不受 cwd 影响），故先剥本包前缀 */
const MOBILE_REL_PREFIX = 'training-app/叉车维修培训学员端跨端应用'

/** 去注释（保留换行无所谓：本脚本只认字符串字面量，不报行号） */
function stripComments(src) {
  return String(src)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\/\/[^\n]*/g, '')
}

/** 直接写全路径的字面量：'pages/x/y.uvue' / "components/a/b.uvue" / 'uni.scss' / 'pages.json' */
const DIRECT_RE = /['"`]((?:pages|components|utils|api|stores|composables|types)\/[^'"`\n]+?\.(?:uvue|uts|json|scss|md)|(?:uni\.scss|pages\.json))['"`]/g

/** `path.join(ROOT|__dirname, …, 'pages', 'x', 'y.uvue')` 的拆分写法 */
const JOIN_RE = /path\.join\(\s*([^)]*)\)/g

/** 只看这些前缀（其余如 `.ci-verify/*.log` 属运行期产物，不进映射） */
const TRACK_PREFIX_RE = /^(?:pages|components|utils|api|stores|composables|types)\//

function normalize(segs) {
  const out = []
  for (const s of segs) {
    if (s === '.' || s === '') continue
    if (s === '..') out.pop()
    else out.push(s)
  }
  return out.join('/')
}

/** 从一个测试源码里抽它引用的本包文件（规范化后的相对路径） */
function refsOf(src) {
  const code = stripComments(src)
  const refs = new Set()
  for (const m of code.matchAll(DIRECT_RE)) refs.add(normalize(m[1].split('/')))
  for (const m of code.matchAll(JOIN_RE)) {
    const segs = [...m[1].matchAll(/['"`]([^'"`]*)['"`]/g)].map((x) => x[1])
    if (segs.length === 0) continue
    const p = normalize(segs)
    if (TRACK_PREFIX_RE.test(p) || p === 'pages.json' || p === 'uni.scss') refs.add(p)
  }
  return [...refs]
}

function buildMap() {
  let suites
  try {
    suites = guardSplit.splitSuites(UTILS_DIR)
  } catch (e) {
    console.error(`[test-impact-map] 套件分类失败（fail-closed）：${e.message}`)
    return null
  }
  // splitSuites 返回**绝对路径**；对外统一用 basename（jest 传参也只认文件名）
  const legs = new Map()
  for (const abs of suites.wiring) legs.set(basename(abs), 'unit')
  for (const abs of suites.behavior) legs.set(basename(abs), 'behavior')
  const names = [...legs.keys()].sort()
  const map = new Map()
  for (const name of names) {
    let src
    try {
      src = readFileSync(join(UTILS_DIR, name), 'utf8')
    } catch (e) {
      console.error(`[test-impact-map] 读不出 ${name}（fail-closed）：${e.message}`)
      return null
    }
    for (const ref of refsOf(src)) {
      if (!map.has(ref)) map.set(ref, [])
      map.get(ref).push({ test: `utils/${name}`, leg: legs.get(name) })
    }
  }
  return { legs, map, names }
}

function jestCommand(entries) {
  const unit = entries.filter((e) => e.leg === 'unit').map((e) => e.test)
  const behavior = entries.filter((e) => e.leg === 'behavior').map((e) => e.test)
  const cmds = []
  if (unit.length > 0) cmds.push(`npx jest --config jest.config.unit.js -i ${unit.join(' ')}`)
  if (behavior.length > 0) cmds.push(`npx jest --config jest.config.behavior.js -i ${behavior.join(' ')}`)
  return cmds
}

/** 工作区改动（含未跟踪）→ 本包相对路径；非本包路径直接丢弃 */
function changedFiles() {
  const out = execFileSync('git', ['-c', 'core.quotepath=false', 'status', '--porcelain'], {
    cwd: MOBILE_ROOT,
    encoding: 'utf8',
  })
  const files = []
  for (const line of out.split(/\r?\n/)) {
    if (!line.trim()) continue
    let p = line.slice(3).trim()
    if (p.includes(' -> ')) p = p.split(' -> ').pop().trim()
    if (p.startsWith('"') && p.endsWith('"')) p = p.slice(1, -1)
    p = p.replace(/\\/g, '/')
    if (!p.startsWith(`${MOBILE_REL_PREFIX}/`)) continue
    files.push(p.slice(MOBILE_REL_PREFIX.length + 1))
  }
  return files
}

function uniqueEntries(list) {
  const seen = new Set()
  return list.filter((e) => {
    const k = `${e.test}#${e.leg}`
    if (seen.has(k)) return false
    seen.add(k)
    return true
  })
}

function printOverview({ legs, map, names }) {
  const unit = [...legs.values()].filter((v) => v === 'unit').length
  const behavior = names.length - unit
  console.log(`套件 ${names.length}（unit ${unit} / behavior ${behavior}）；登记引用 ${map.size} 个本包文件。`)
  const rows = [...map.entries()]
    .map(([file, tests]) => ({ file, n: tests.length }))
    .sort((a, b) => b.n - a.n || a.file.localeCompare(b.file))
  for (const r of rows.slice(0, 15)) console.log(`  ${String(r.n).padStart(2)}  ${r.file}`)
  if (rows.length > 15) console.log(`  … 其余 ${rows.length - 15} 项用 --json 看全量`)
  console.log('\n查询：node scripts/test-impact-map.mjs pages/profile/help-center.uvue')
  console.log('改动面：node scripts/test-impact-map.mjs --changed')
}

function main(argv) {
  const rest = argv.filter((a) => a !== '--json')
  const json = argv.includes('--json')
  const built = buildMap()
  if (!built) return 2

  if (rest.length === 0) {
    printOverview(built)
    return 0
  }

  if (rest[0] === '--changed') {
    let changed
    try {
      changed = changedFiles()
    } catch (e) {
      console.error(`[test-impact-map] git status 失败（fail-closed）：${e.message}`)
      return 2
    }
    if (changed.length === 0) {
      console.log('工作区无改动。')
      return 0
    }
    const entries = []
    const uncovered = []
    for (const rel of changed) {
      const hits = built.map.get(rel) ?? []
      if (hits.length > 0) entries.push(...hits)
      else if (!rel.startsWith('utils/') || !rel.endsWith('.test.js')) uncovered.push(rel)
      // 改的是测试自身 ⇒ 它自己必跑
      if (rel.startsWith('utils/') && rel.endsWith('.test.js')) {
        const name = rel.slice('utils/'.length)
        if (built.legs.has(name)) entries.push({ test: `utils/${name}`, leg: built.legs.get(name) })
      }
    }
    const uniq = uniqueEntries(entries)
    console.log(`改动文件 ${changed.length} 个；命中契约套件 ${uniq.length} 个。`)
    for (const rel of changed) {
      const hits = built.map.get(rel) ?? []
      const isSuite = rel.startsWith('utils/') && rel.endsWith('.test.js')
      const note = hits.length > 0 ? `（${hits.length} 套件）` : isSuite ? '（套件自身）' : '（无静态引用）'
      console.log(`  ${hits.length > 0 || isSuite ? '→' : '·'} ${rel}${note}`)
    }
    if (uncovered.length > 0) console.log(`  未覆盖面：${uncovered.join(', ')}（改动不在静态断言网内，按需人工判断）`)
    if (uniq.length === 0) {
      console.log('\n无受影响套件 ⇒ 无需跑测试（改动面仅样式 / 未进契约）。')
      return 0
    }
    console.log('\n受影响套件：')
    for (const e of uniq) console.log(`  [${e.leg}] ${e.test}`)
    console.log('\n可直接执行：')
    for (const cmd of jestCommand(uniq)) console.log(`  ${cmd}`)
    return 0
  }

  const keys = rest.map((k) => normalize(k.split('/')))
  const found = []
  const missing = []
  for (const k of keys) {
    const hits = built.map.get(k) ?? []
    if (hits.length === 0) missing.push(k)
    else found.push(...hits)
  }
  if (json) {
    console.log(JSON.stringify({ query: keys, tests: uniqueEntries(found), missing }, null, 2))
    return 0
  }
  const uniq = uniqueEntries(found)
  if (uniq.length === 0) console.log(`无套件读取：${keys.join(', ')}`)
  else {
    console.log(`受影响套件 ${uniq.length} 个：`)
    for (const e of uniq) console.log(`  [${e.leg}] ${e.test}`)
    console.log('\n可直接执行：')
    for (const cmd of jestCommand(uniq)) console.log(`  ${cmd}`)
  }
  if (missing.length > 0 && keys.length > 1) console.log(`（未命中：${missing.join(', ')}）`)
  return 0
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
