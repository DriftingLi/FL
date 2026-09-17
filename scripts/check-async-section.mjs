#!/usr/bin/env node
/**
 * 列表四段式守卫（ADR-0053 §6 / issue #1054）。
 *
 * 「错误态 → 骨架 → 空态 → 内容」的渲染优先级唯一实现在 `components/ui/UiAsyncSection.vue`。
 * 本守卫拦两件事：
 *
 * ① 页面/业务组件**重新手写四分支链**：同一文件里既出现错误态组件（UiErrorState）
 *    又出现骨架或空态组件（UiSkeleton / UiEmptyState）——这正是「四态先后与互斥在
 *    页面里重新表述」的信号（收敛前 39 页各写一遍、顺序还不一致）；
 * ② 空态判据**在模板里内联**（`:empty="items.length === 0"` / `:empty="!data"` /
 *    写死 `:empty="false"`）——空态判据的默认路径是 `useAsyncPage` 的 `isEmpty`
 *    （第十一波 #1101），页面不得再自带第二份判定表达式。
 *
 * 用法：
 *   node scripts/check-async-section.mjs --all            全量扫描（CI 用）
 *   node scripts/check-async-section.mjs --diff [base]    只查相对 base 的新增行（本地用）
 *
 * 退出码：有违规 1，否则 0。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import path from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const ROOT = path.resolve(path.dirname(__filename), '..')

/**
 * 四分支链的逐条登记例外（相对路径 → 理由）。
 *
 * 第十一波 #1101 已把 #1054 的 15 条存量全部迁移到 `UiAsyncSection`（错误态走组件内
 * 的 UiErrorState 或 `#error` 插槽），因此这里**清零**：15/15 销号完成，不再有待迁移项。
 * 新代码不允许往这里加——新增违规直接报红。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/**
 * `:empty=` 内联表达式的逐条登记例外（相对路径 → 理由）。
 *
 * 判据默认路径是 `useAsyncPage` 的 `isEmpty`（或 `utils/listState.isEmptyList` 派生的
 * 页面级 `isEmpty`）。只有**判据形状确实装不进那个判据**的站点才登记在这里，
 * 每条都要写明为什么。#1101 已把 32 处 `:empty=` 收编到只剩 1 条：
 * @type {Record<string, string>}
 */
export const EMPTY_EXCEPTIONS = {
  'frontend/src/pages/admin/Inspection.vue':
    '#1101 登记例外：本页流水列表走 admin 列表状态机 useAdminTable（ADR-0039 / ui-conventions「同一页面里的第二档」），' +
    '该 composable 只暴露 list，本轮不引入与 useAsyncPage 平行的第二份 isEmpty 判据（另案，ADR-0056 §9 / #1102）'
}

/**
 * 空态判据的合法值：`isEmpty`（composable 的默认路径，或页面级同名派生）或已登记的
 * 具名判据标识符。裸表达式、写死 `false`、下划线开头的私有名一律报红。
 */
export const ALLOWED_EMPTY_VALUES = ['isEmpty']

/** 归一化为 POSIX 相对路径。 */
function rel(file) {
  const p = path.isAbsolute(file) ? path.relative(ROOT, file) : file
  return p.split(path.sep).join('/')
}

/** 路径不在守卫面即整体放行（ui 封装层自身合法组合三件套）。 */
export function isGuardedPath(file) {
  const r = rel(file)
  if (!r.startsWith('frontend/src/')) return false
  if (!r.endsWith('.vue')) return false
  if (r.startsWith('frontend/src/components/ui/')) return false
  return true
}

/**
 * 扫一份源码，返回违规说明（1-based 行号 + message）。
 * 规则见文件头：① 手写四分支链；② `:empty=` 内联判据。
 * （只 import 不使用的情况罕见且无害——按文本信号判定，与既有守卫同口径。）
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  const hits = { error: null, skeleton: null, empty: null }
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (hits.error === null && /<UiErrorState\b/.test(line)) hits.error = { line: i + 1, lineText: line.trim() }
    if (hits.skeleton === null && /<UiSkeleton\b/.test(line)) hits.skeleton = { line: i + 1, lineText: line.trim() }
    if (hits.empty === null && /<UiEmptyState\b/.test(line)) hits.empty = { line: i + 1, lineText: line.trim() }
  }
  if (hits.error && (hits.skeleton || hits.empty)) {
    const partner = hits.skeleton ? 'UiSkeleton' : 'UiEmptyState'
    const at = hits.skeleton ?? hits.empty
    violations.push({
      line: at.line,
      message: `手写四分支链：UiErrorState(第 ${hits.error.line} 行) 与 ${partner}(第 ${at.line} 行) 同文件编排 —— 改用 components/ui/UiAsyncSection.vue（渲染优先级唯一实现，#1054）`
    })
  }
  violations.push(...scanEmptyProps(lines))
  return violations
}

/** `:empty=` 的绑定值表达式（去掉两端引号）。 */
const EMPTY_BINDING_RE = /:empty\s*=\s*"([^"]*)"|:empty\s*=\s*'([^']*)'/g

/** 具名判据标识符（`isEmpty` / `isEmptyXxx`）；下划线开头的私有名不算。 */
const EMPTY_JUDGE_RE = /^(?:isEmpty|isEmpty[A-Z0-9_$][A-Za-z0-9_$]*)$/

/** @type {Set<string>} 页面脚本里声明过的具名判据名（`const isEmpty = …` / `const isEmptyX = …`）。 */
function emptyJudgesInScript(source) {
  const names = new Set()
  const re = /\bconst\s+(isEmpty[A-Za-z0-9_$]*)\s*(?=[:=])/g
  let m
  while ((m = re.exec(source)) !== null) names.add(m[1])
  return names
}

/**
 * 规则 ②：`:empty=` 只允许 `isEmpty`（composable 的默认路径或页面级同名派生）。
 * 页面自定义形状的判据要起 `isEmpty` 开头的名字并在脚本里用 `const` 声明——
 * 这样判据始终是**具名的**，评审能顺着名字找到唯一那份判定，模板里没有第二份表达式的藏身处。
 * 文件级登记例外见 `EMPTY_EXCEPTIONS`。
 */
export function scanEmptyProps(lines) {
  const violations = []
  const declared = emptyJudgesInScript(lines.join('\n'))
  for (let i = 0; i < lines.length; i++) {
    EMPTY_BINDING_RE.lastIndex = 0
    let m
    while ((m = EMPTY_BINDING_RE.exec(lines[i])) !== null) {
      const value = (m[1] ?? m[2] ?? '').trim()
      if (ALLOWED_EMPTY_VALUES.includes(value)) continue
      if (EMPTY_JUDGE_RE.test(value) && declared.has(value)) continue
      violations.push({
        line: i + 1,
        message: `:empty="${value}" 是模板内联的空态判据 —— 空态判据只认 useAsyncPage 的 isEmpty`
          + `（loader 写入的数组传给 itemsRef 即得，404 = 空态一并由它判定）；`
          + '判据形状特殊时在脚本里声明 const isEmpty = computed(…)，或改用 utils/listState 的 isEmptyList 派生。'
          + `确需保留的逐条登记到 EMPTY_EXCEPTIONS（#1101）`
      })
    }
  }
  return violations
}

/** 递归收集 .vue 文件。 */
function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, out)
    else if (entry.name.endsWith('.vue')) out.push(full)
  }
  return out
}

function runAll() {
  const files = walk(path.join(ROOT, 'frontend', 'src'))
  let count = 0
  for (const f of files) {
    const r = rel(f)
    const vs = scanSource(readFileSync(f, 'utf8'), f)
    if (vs.length && (ALLOWLIST[r] || EMPTY_EXCEPTIONS[r])) continue
    for (const v of vs) {
      count++
      console.error(`✗ ${r}:${v.line}\n  ${v.message}`)
    }
  }
  return count
}

function runDiff(parseAddedLines, base) {
  let added
  try {
    added = parseAddedLines(base, '*.vue')
  } catch (e) {
    console.error('[check-async-section] 解析新增行失败: ' + (e && e.message ? e.message : e))
    return 1
  }
  let count = 0
  for (const [file, lineSet] of added) {
    if (!isGuardedPath(file)) continue
    const r = rel(file)
    let source
    try {
      source = readFileSync(path.join(ROOT, file), 'utf8')
    } catch {
      continue
    }
    const vs = scanSource(source, file)
    for (const v of vs) {
      if (!lineSet.has(v.line)) continue
      if (ALLOWLIST[r] || EMPTY_EXCEPTIONS[r]) continue
      count++
      console.error(`✗ ${r}:${v.line}\n  ${v.message}`)
    }
  }
  return count
}

async function main() {
  const argv = process.argv.slice(2)
  if (argv.length === 0 || argv[0] === '--help') {
    console.log('用法: node scripts/check-async-section.mjs --all | --diff [base]')
    process.exit(0)
  }
  let count
  if (argv[0] === '--all') {
    count = runAll()
  } else if (argv[0] === '--diff') {
    const { parseAddedLines } = await import('./lib/added-lines.mjs')
    count = runDiff(parseAddedLines, argv[1] || 'origin/master')
  } else {
    console.error('未知参数: ' + argv[0])
    process.exit(2)
  }
  if (count > 0) {
    console.error(`\n共 ${count} 处违规。`)
    process.exit(1)
  }
  console.log('✓ 未发现手写四分支链，且 :empty= 判据均来自 isEmpty（或已登记例外）')
}


const isDirectRun = process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url
if (isDirectRun) {
  main().catch((e) => {
    console.error(e)
    process.exit(2)
  })
}
