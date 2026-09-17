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
 *    写死 `:empty="false"`）——空态判据的默认路径是两档各自的 `isEmpty`
 *    （`useAsyncPage` / `useAdminTable`，第十一波 #1101 / #1102），页面不得再自带
 *    第二份判定表达式。
 * ③ admin 页的两档档位**没写明归属**（第十一波 #1102，ADR-0056 §9）：同一页面里
 *    分页列表走 `useAdminTable`、只读/计数 section 走 `useAsyncPage`，两档在场的页面
 *    必须在文件顶部登记档位；登记行还要拿得出实据（见 `scanAdminTiers`）。
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
 * 判据默认路径是两档各自的 `isEmpty`（`useAsyncPage` / `useAdminTable`，同一份实现
 * `utils/listState.isEmptyList`）。#1101 把 32 处 `:empty=` 收编到只剩 1 条例外
 * （Inspection），那条例外的理由正是「useAdminTable 还没有对等判据」；#1102 给它补齐
 * `isEmpty` 后该例外即销号——**这里是空的，新代码不允许再往这里加**。
 * @type {Record<string, string>}
 */
export const EMPTY_EXCEPTIONS = {}

/**
 * 空态判据的合法值：`isEmpty`（composable 的默认路径，或页面级同名派生）或已登记的
 * 具名判据标识符。裸表达式、写死 `false`、下划线开头的私有名一律报红。
 */
export const ALLOWED_EMPTY_VALUES = ['isEmpty']

/**
 * admin 页两档档位登记（第十一波 #1102，ADR-0056 §9）。
 *
 * 「同一页面里的第二档要写明归属」：单档页面的归属由既有约定唯一确定（admin 列表页
 * → useAdminTable；非列表页/只读 section → useAsyncPage），**两档在场**的页面才有歧义，
 * 必须在文件顶部登记。登记行同时是可核对的声明：每种「档位（形态）」都配一条
 * 实据谓词（evidence），登记了却拿不出实据即报红——登记行不能退化成一句注释。
 *
 * 登记行形态（脚本在前的页面用 `//`，模板在前的页面用 HTML 注释）：
 *   // 列表档位：useAdminTable（分页列表）—— 帖子列表 / 举报队列
 *   <!-- 列表档位：useAsyncPage（只读计数）—— 删除已解决帖计数 -->
 */
export const TIER_MARKER = '列表档位'

/**
 * 登记表：档位（形态）→ 实据谓词。只有这张表里的三行是合法登记。
 * 「行内追加」不是第三个档位，而是第二档的行内形态：按行实例化装不进页面级 composable
 * 实例（setup 之外不能建 watch），所以它的实据是**复用同源判据** utils/listState.isEmptyList。
 */
export const TIER_FORMS = [
  {
    tier: 'useAdminTable',
    form: '分页列表',
    // 实例化可能带泛型实参（useAdminTable<Row>(…)）——名字与左括号之间只排除括号与换行
    evidence: /\buseAdminTable\b[^()\n]*\(/,
    hint: '页面级 useAdminTable 实例（useAdminTable(… )）'
  },
  {
    tier: 'useAsyncPage',
    form: '只读计数',
    evidence: /\buseAsyncPage\b[^()\n]*\(/,
    hint: '页面级 useAsyncPage 实例（useAsyncPage(… )）'
  },
  {
    tier: 'useAsyncPage',
    form: '行内追加',
    evidence: /\bisEmptyList\s*\(/,
    hint: '同为第二档的行内形态：复用 utils/listState.isEmptyList 的同源判据'
  }
]

/** 登记行标记：`列表档位：…`（允许 `//` / `<!--` / `*` 前缀），捕获标记后的整段。 */
const TIER_MARKER_RE = /^\s*(?:\/\/|<!--|\*)?\s*列表档位[:：]\s*(.*)$/

/** 登记表里的「档位（形态）」对。 */
const TIER_PAIR_RE = /^(useAdminTable|useAsyncPage)（([^）]*)）/

/**
 * 一行 → `{ tier, form, raw }`；不是登记行（没有标记）返回 null。
 * 标记在场但「档位（形态）」不在登记表里时 tier 为空串、raw 保留原样——调用方据此报红。
 */
export function parseTierLine(line) {
  const m = TIER_MARKER_RE.exec(line)
  if (!m) return null
  const raw = m[1].replace(/\s*-->\s*$/, '').trim()
  const pair = TIER_PAIR_RE.exec(raw)
  if (!pair) return { tier: '', form: '', raw }
  return { tier: pair[1], form: pair[2].trim(), raw }
}

/** 源码里第一次命中证据谓词的行号（1-based）；没有命中返回 1。 */
function firstEvidenceLine(lines, re) {
  for (let i = 0; i < lines.length; i++) {
    if (re.test(lines[i])) return i + 1
  }
  return 1
}

/**
 * 规则 ③：admin 页的档位登记。
 * - 出现登记行的页面：每行都必须落在 `TIER_FORMS` 里，且该形态的实据在文件里成立；
 * - 同时用了 ≥2 种档位形态的页面必须逐一登记（这就是「第二档要写明归属」的样子），
 *   只用了 1 种的页面无需登记（归属由既有约定唯一确定）。
 * 判据与豁免分层：本函数不读任何 allowlist。
 */
export function scanAdminTiers(source, file) {
  const r = rel(file)
  if (!r.startsWith('frontend/src/pages/admin/') || !r.endsWith('.vue')) return []
  const lines = String(source).split('\n')
  const violations = []
  /** @type {Set<string>} 已合法登记的「档位（形态）」 */
  const declared = new Set()
  for (let i = 0; i < lines.length; i++) {
    const parsed = parseTierLine(lines[i])
    if (!parsed) continue
    const key = parsed.tier ? `${parsed.tier}（${parsed.form}）` : parsed.raw
    const spec = TIER_FORMS.find(f => f.tier === parsed.tier && f.form === parsed.form)
    if (!spec) {
      violations.push({
        line: i + 1,
        message: `档位登记「${key}」不在登记表里 —— 可写的只有 ${TIER_FORMS.map(f => `${f.tier}（${f.form}）`).join(' / ')}（ADR-0056 §9）`
      })
      continue
    }
    if (declared.has(key)) continue
    declared.add(key)
    if (!spec.evidence.test(source)) {
      violations.push({
        line: i + 1,
        message: `档位登记「${key}」拿不出实据：文件里找不到 ${spec.hint} —— 登记行不能是一句没兑现的声明`
      })
    }
  }
  if (declared.size >= 2) return violations
  const used = TIER_FORMS.filter(f => f.evidence.test(source))
  if (used.length >= 2) {
    const at = used[used.length - 1]
    violations.push({
      line: firstEvidenceLine(lines, at.evidence),
      message: `本页同时用了 ${used.map(f => `${f.tier}（${f.form}）`).join(' + ')}，但没有在文件顶部登记档位 —— 加一行「${TIER_MARKER}：<档位>（<形态>）」（ADR-0056 §9）`
    })
  }
  return violations
}

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
 * 规则见文件头：① 手写四分支链；② `:empty=` 内联判据；③ admin 页两档档位登记。
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
  violations.push(...scanAdminTiers(source, file))
  return violations
}

/** `:empty=` 的绑定值表达式（去掉两端引号）。 */
const EMPTY_BINDING_RE = /:empty\s*=\s*"([^"]*)"|:empty\s*=\s*'([^']*)'/g

/** 具名判据标识符（`isEmpty` / `isEmptyXxx`）；下划线开头的私有名不算。 */
const EMPTY_JUDGE_RE = /^(?:isEmpty|isEmpty[A-Z0-9_$][A-Za-z0-9_$]*)$/

/**
 * @type {Set<string>} 页面脚本里声明过的具名判据名。两种写法都算：
 * - `const isEmptyXxx = …`（页面自建派生，如以 total 为准）；
 * - 从档位 composable 解构改名 `const { isEmpty: isEmptyXxx } = useAdminTable(…)`
 *   ——同页多实例（#1102：一个页面里五段分页列表）只能这样给每段一个具名判据，
 *   判据本身仍是 composable 里那一份，页面没有第二份表达式。
 */
function emptyJudgesInScript(source) {
  const names = new Set()
  const declared = /\bconst\s+(isEmpty[A-Za-z0-9_$]*)\s*(?=[:=])/g
  const renamed = /\bisEmpty\s*:\s*(isEmpty[A-Za-z0-9_$]*)/g
  let m
  while ((m = declared.exec(source)) !== null) names.add(m[1])
  while ((m = renamed.exec(source)) !== null) names.add(m[1])
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
  console.log('✓ 未发现手写四分支链；:empty= 判据均来自 isEmpty（或已登记例外）；admin 页两档档位登记齐备')
}


const isDirectRun = process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url
if (isDirectRun) {
  main().catch((e) => {
    console.error(e)
    process.exit(2)
  })
}
