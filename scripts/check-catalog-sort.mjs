#!/usr/bin/env node
/**
 * 目录排序串第二源守卫（ADR-0060 决策 10 / 第十三波票 10）。
 *
 * 背景：课程目录的排序串有一处**声明**（catalog_specs.go / position_catalog.go 的 descriptor
 * `OrderBy` 字段，经 catalog_engine.go 的 `q.Order(spec.OrderBy)` 消费），但目录树的读面
 * （training_catalog_service.go 的 getCatalogTree）曾把专业方向与课程等级那两条串**逐字再抄一遍**
 * 写进 `q.Order("sort_order ASC, specialty_id ASC")` —— 同一判据两个宿主：改了 spec 表忘了改读面，
 * 列表与树的顺序就静默分叉，没有任何测试接得住。本票把读面改成引用 spec 声明，本守卫把
 * 「读面不得再出现与 spec 逐字同串的裸排序串」变成 CI 可核验的事实。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，ADR-0056 §5 / #1094；本文件只有判定面）：
 *   node scripts/check-catalog-sort.mjs --all  [目录]   全量扫描（默认 backend；有违规则退出 1）
 *   node scripts/check-catalog-sort.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *     —— 例外文件（ALLOWLIST）在增量门里也是**逐行**判定：只有基线即违规的行号放行，新增行上的
 *        违规照报；基线取不到即非零退出（行号级口径见 scripts/lib/guard.mjs）。
 *
 * 判定面：只扫**目录面**（仓库路径里文件名含 `catalog` 的非测试 .go，service 与 api 两侧都算），
 * 且只认一种形态：`.Order(` 的实参里出现**与 spec 声明逐字同串**的字符串字面量（第二源）。
 * 判据刻意窄在这些边界上，每条都有原因：
 *   - 「逐字同串」而非「任何裸排序串」：带表别名的课程行 `course.sort_order ASC, course.course_id ASC`
 *     与章节行 `order_num ASC, chapter_id ASC` 不是 spec 声明的那批串，ADR-0060 决策 10 明记**不动**；
 *     把它们算进射程就得给整个读面文件开 ALLOWLIST，而 `--all` 的例外是**整文件**放行 ——
 *     那等于把守卫对真正危险的两行一起盲掉（#1123 收掉的正是这类洞）。
 *   - 「整串字面量」而非「字面量片段」：renumberSortGroup 的 `q.Order("sort_order ASC, " + idCol + " ASC")`
 *     是按 ID 列参数化派生的形态（写面 swap 用），没有任何一份字面量等于 spec 声明，不算第二源。
 *   - spec 声明自身（`OrderBy: "…"`）不是 `.Order(` 调用，天然在判定之外：声明槽仍是唯一合法宿主。
 * 放行面（有意不进守卫集）：
 *   1. `*_test.go` —— 顺序锁用例（catalog_tree_shape_test.go / catalog_sort_test.go 等）按定义要写期望序；
 *   2. 非目录面文件 —— 排序串的声明面与消费面都在 catalog* 里，其余域的 ORDER BY 不归本守卫管。
 * 白名单（ALLOWLIST）初始为空：确有例外时逐条登记并写明理由，不要放宽规则本身。
 *
 * 一致性：下面的声明串是**手抄的一份**，它与 spec 文件的真实声明由
 * `scripts/check-catalog-sort.test.mjs` 的「声明集与 spec 表互等」用例锁住 ——
 * 新增/改动 spec 的 OrderBy 而不更新本表，自检即红（同 `gate-predicates` 那条「判据不许改回双份」）。
 */
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 守卫面：文件名含该片段的 .go（catalog_specs / catalog_engine / training_catalog_* / position_catalog）。 */
export const GUARDED_PATH_SEGMENT = 'catalog'

/** 扫描面后缀（Go 侧只有 .go）。 */
export const SCAN_EXTENSIONS = ['.go']

/**
 * spec 声明表的排序串（唯一合法宿主 = catalog_specs.go / position_catalog.go 的 `OrderBy` 字段）。
 * 读面再出现其中任何一条裸字面量即「第二源」，判红。
 * @type {{orderBy: string, entity: string, declaredIn: string}[]}
 */
export const SPEC_ORDER_BY_DECLARATIONS = [
  {
    orderBy: 'sort_order ASC, specialty_id ASC',
    entity: '专业方向 specialty',
    declaredIn: 'backend/internal/service/catalog_specs.go'
  },
  {
    orderBy: 'sort_order ASC, level_id ASC',
    entity: '课程等级 course_level',
    declaredIn: 'backend/internal/service/catalog_specs.go'
  },
  {
    orderBy: 'sort_order ASC, id ASC',
    entity: '题库标签 question_tag / 目标证件 credential',
    declaredIn: 'backend/internal/service/catalog_specs.go'
  },
  {
    orderBy: 'id ASC',
    entity: '证书模板 certificate_template',
    declaredIn: 'backend/internal/service/catalog_specs.go'
  },
  {
    orderBy: 'sort_order ASC, position_id ASC',
    entity: '岗位 positions',
    declaredIn: 'backend/internal/service/position_catalog.go'
  }
]

/** 声明串 → 归属实体（报告文案指认「该回哪张表取」）。 */
export const SPEC_ORDER_BY = new Map(
  SPEC_ORDER_BY_DECLARATIONS.map((d) => [d.orderBy, d.entity + '（' + d.declaredIn + '）'])
)

/**
 * 逐条登记的有意例外（路径 → 理由）。本票收口后目录读面实测 0 存量，且整文件例外会让守卫
 * 对同文件其它行一起失明（见文件头），故本表保持为空；规则绝对执行。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** 整行注释（Go 的 // 与块注释行）不是调用，不算判定对象。 */
export function isCommentLine(line) {
  const t = String(line).trim()
  return t.startsWith('//') || t.startsWith('*') || t.startsWith('/*')
}

/** Go 测试文件不进判定面（顺序锁用例按定义要写期望序）。 */
export function isTestFile(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.endsWith('_test.go')
}

/**
 * 路径是否在守卫面（目录面的非测试 .go）。
 * **ALLOWLIST 不在这里判**（#1123）：豁免由 runner 承载（`--all` 整体放行 / `--diff` 只放行基线
 * 违规行号）—— 判定面先吞掉例外文件的话 `scanSource` 恒返回空，行号级放行会静默失效。
 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  if (!p.endsWith('.go') || isTestFile(p)) return false
  const base = p.slice(p.lastIndexOf('/') + 1)
  return base.includes(GUARDED_PATH_SEGMENT)
}

/** 取一行里的 Go 字符串字面量内容（解释串与原始串都算，转义按字面留着不影响比对）。 */
export function stringLiterals(line) {
  const out = []
  const re = /"((?:[^"\\\n]|\\.)*)"|`([^`]*)`/g
  let m
  while ((m = re.exec(line)) !== null) out.push(m[1] ?? m[2])
  return out
}

/**
 * 从 spec 源码里取真实声明的排序串（`OrderBy: "…"`）。守卫判定面不依赖它（串是手抄的那份，
 * 见文件头「一致性」），自检用它把声明表与本表锁成互等。
 */
export function declaredOrderByLiterals(source) {
  const out = []
  const re = /OrderBy:\s*"((?:[^"\\\n]|\\.)*)"|OrderBy:\s*`([^`]*)`/g
  let m
  while ((m = re.exec(String(source))) !== null) out.push((m[1] ?? m[2]).trim())
  return out
}

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 命中的排序串 + 该串的声明处 + 原文）。
 * **路径不在守卫面即整体放行**（非 catalog 文件、测试、白名单由 runner 处理）——
 * 判定收敛在这里，调用方不必自己记得先过滤（少一个「忘了过滤就假红」的面）。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (isCommentLine(line)) continue
    if (!line.includes('.Order(')) continue
    for (const lit of stringLiterals(line)) {
      const entity = SPEC_ORDER_BY.get(lit.trim())
      if (entity) {
        violations.push({ file, line: i + 1, orderBy: lit.trim(), entity, text: line.trim() })
      }
    }
  }
  return violations
}

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs —— 新增守卫只需实现 scanSource 并声明这一份。
 */
export const GUARD_SPEC = {
  name: 'check-catalog-sort',
  usage: '用法: node scripts/check-catalog-sort.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'backend',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    stream: 'stdout',
    header: (ctx) => [
      '===== 目录排序串守卫：全量扫描（目录读面不得出现与 spec 逐字同串的裸排序串）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: 文件名含 ' + GUARDED_PATH_SEGMENT + ' 的非测试 .go（跳过测试与白名单）',
      '声明面: catalog_specs.go / position_catalog.go 的 OrderBy 字段（读面一律从它取）',
      '禁入串: ' + SPEC_ORDER_BY_DECLARATIONS.map((d) => '"' + d.orderBy + '"').join(' | '),
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个目录面文件均未出现与 spec 声明逐字同串的裸排序串。',
    violation: (v) =>
      v.file + ':' + v.line + ': 排序串 "' + v.orderBy + '" 又抄了一遍（声明在 ' + v.entity + '）  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。排序串只有一个宿主：目录 descriptor 的 OrderBy 字段，读面写成',
      '  spec.Order…（如 specialtyCatalogSpec().OrderBy / spec.OrderBy），不要重抄字面量（ADR-0060 决策 10）。',
      '  带表别名的课程行与章节行不在此判据内（ADR 明记不动）；新增 spec 排序串要与本脚本的声明表互等（自检锁）。'
    ]
  },
  diff: {
    pathspec: ['*.go'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-catalog-sort] 相对 ' + base + ' 无 .go 新增行，跳过。',
    ok: () => '[check-catalog-sort] 新增行未出现目录排序串的第二源，通过。',
    header: () => ['===== 新增行重抄了 spec 已声明的目录排序串（一律回 descriptor 取）====='],
    violation: (v) =>
      v.file + ':' + v.line + ': 排序串 "' + v.orderBy + '" 又抄了一遍（声明在 ' + v.entity + '）  ' + v.text,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 ADR-0060 决策 10 与 docs/agents/checks.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
