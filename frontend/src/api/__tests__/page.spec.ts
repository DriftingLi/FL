// 分页容器归位锁（ADR-0060 决策 6 / 票 6）。
//
// 两件事：
//   1. `toPage` 的兜底口径逐字锁住（行缺失 / total 非有限值 → [] 与 0）——17 处页面手抄里
//      的 `|| []` / `|| 0` 现在只有这一份实现。
//   2. **锁**：`pages/` 与 `components/` 里不得再有 `useAdminTable` 的 `fetch` adapter 自己
//      搭分页容器（返回一个带 `list:` / `items:` 键的对象字面量）。容器属于 api 侧的出口，
//      页面 adapter 的合法形态是 `return someApi.listX(...)`。
//
// 做法与 `utils/__tests__/statusWordsTemplate.spec.ts` 同形：把 SFC 的 `<script>` 段真解析成
// AST（用仓库已有的 typescript 包，不是正则），按**形状**判红而不是按文件名，新页面无从绕开。
// 之所以必须走 AST：页面里 `const { list: items } = useAdminTable(…)` 的**解构改名**长得一样，
// 正则会把这十几处合法写法全部误判成手抄。
//
// 存量口径：断言「当前命中集合为空」（票 6 落地后本该为 0），不写非空白名单。
import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'
import { parse, type SFCScriptBlock } from 'vue/compiler-sfc'
import ts from 'typescript'
import { toPage } from '../page'

/** frontend/src（本文件在 src/api/__tests__ 下）。 */
const SRC = resolve(__dirname, '../..')

/** 扫描射程：页面与业务组件（composable 与 api 层不在内——容器就住在那两侧）。 */
const SCAN_DIRS = ['pages', 'components']

/** 容器键：adapter 返回的对象字面量里出现即说明容器在页面侧手搭（换中立键名也拦得住）。 */
const CONTAINER_KEYS = ['list', 'items']

/** 一次命中：文件 + .vue 内的 1-based 行号 + 命中的键。 */
interface Violation {
  file: string
  line: number
  key: string
}

/** 递归收集 .vue（`__tests__` 不在射程内：测试自己的 fixture 不是页面写法）。 */
function collectVueFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    if (entry === '__tests__' || entry === 'node_modules') continue
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...collectVueFiles(full))
    else if (entry.endsWith('.vue')) out.push(full)
  }
  return out
}

/** SFC 的 <script> / <script setup> 段（两块都扫，不赌页面只用哪一种）。 */
function scriptBlocks(raw: string): SFCScriptBlock[] {
  const { descriptor, errors } = parse(raw)
  if (errors.length) throw errors[0]
  return [descriptor.script, descriptor.scriptSetup].filter((block): block is SFCScriptBlock => !!block)
}

/** 剥掉括号 / 类型断言 / await，取真正被返回的那个表达式。 */
function unwrap(expr: ts.Expression): ts.Expression {
  let node: ts.Expression = expr
  for (;;) {
    if (
      ts.isParenthesizedExpression(node) ||
      ts.isAsExpression(node) ||
      ts.isSatisfiesExpression(node) ||
      ts.isNonNullExpression(node) ||
      ts.isAwaitExpression(node)
    ) {
      node = (node as ts.ParenthesizedExpression).expression
      continue
    }
    return node
  }
}

function literalKeys(node: ts.ObjectLiteralExpression): string[] {
  const names: string[] = []
  for (const prop of node.properties) {
    if (ts.isPropertyAssignment(prop) || ts.isShorthandPropertyAssignment(prop)) {
      if (ts.isIdentifier(prop.name) || ts.isStringLiteral(prop.name)) names.push(prop.name.text)
    }
  }
  return names
}

/** 函数体内**被返回的**对象字面量（concise body 与 return 语句两种形态都算，三元分支也递归进去）。 */
function returnedLiterals(fn: ts.ArrowFunction | ts.FunctionExpression): ts.ObjectLiteralExpression[] {
  const out: ts.ObjectLiteralExpression[] = []
  const body = fn.body
  if (!body) return out
  // 只顺着「能决定返回值」的形态展开：括号 / 断言 / await / 三元分支。
  // 不递归整棵子树——否则 `return api.list({ items: [] })` 这种**传参**用的字面量会被误判。
  const collect = (expr: ts.Expression): void => {
    const value = unwrap(expr)
    if (ts.isObjectLiteralExpression(value)) out.push(value)
    else if (ts.isConditionalExpression(value)) {
      collect(value.whenTrue)
      collect(value.whenFalse)
    }
  }
  if (!ts.isBlock(body)) {
    collect(body)
    return out
  }
  const visit = (node: ts.Node): void => {
    if (ts.isReturnStatement(node) && node.expression) collect(node.expression)
    ts.forEachChild(node, visit)
  }
  ts.forEachChild(body, visit)
  return out
}

/**
 * 一段脚本的扫描结果：useAdminTable 实例数、其中解析出函数体的 fetch adapter 数、手抄命中。
 *
 * 判定面只有 fetch 的**返回形状**：解构改名（`const { list: items } = useAdminTable(…)`）与
 * 入参拼装（`const params = { page, page_size }`，对象字面量没被 return）都不算手抄。
 */
function scanScript(content: string, lineOffset: number, rel: string) {
  const source = ts.createSourceFile('scan.ts', content, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const violations: Violation[] = []
  let callSites = 0
  let adapters = 0

  const visit = (node: ts.Node): void => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'useAdminTable') {
      callSites++
      for (const arg of node.arguments) {
        if (!ts.isObjectLiteralExpression(arg)) continue
        for (const prop of arg.properties) {
          if (!ts.isPropertyAssignment(prop) || prop.name.getText() !== 'fetch') continue
          const initializer = prop.initializer
          if (!ts.isArrowFunction(initializer) && !ts.isFunctionExpression(initializer)) continue
          adapters++
          for (const literal of returnedLiterals(initializer)) {
            for (const key of literalKeys(literal)) {
              if (!CONTAINER_KEYS.includes(key)) continue
              violations.push({
                file: rel,
                line: lineOffset + source.getLineAndCharacterOfPosition(literal.getStart(source)).line + 1,
                key
              })
            }
          }
        }
      }
    }
    ts.forEachChild(node, visit)
  }
  ts.forEachChild(source, visit)
  return { callSites, adapters, violations }
}

/** 探针：直接在脚本片段上跑判定面（负向用例用）。 */
const probe = (script: string) => scanScript(script, 0, 'probe.vue').violations

/** 正则侧的 useAdminTable 实例数，与 AST 侧比对，防扫描面静默失灵。 */
const USE_COUNT_RE = /\buseAdminTable\s*(?:<[^<>()]*>)?\s*\(/g

function scanTree() {
  const violations: Violation[] = []
  let adapters = 0
  let astCallSites = 0
  let regexCallSites = 0
  for (const relDir of SCAN_DIRS) {
    for (const full of collectVueFiles(resolve(SRC, relDir))) {
      const rel = relative(SRC, full).split(sep).join('/')
      const raw = readFileSync(full, 'utf8')
      for (const block of scriptBlocks(raw)) {
        const lineOffset = raw.slice(0, block.loc.start.offset).split('\n').length - 1
        const scanned = scanScript(block.content, lineOffset, rel)
        regexCallSites += (block.content.match(USE_COUNT_RE) ?? []).length
        astCallSites += scanned.callSites
        adapters += scanned.adapters
        violations.push(...scanned.violations)
      }
    }
  }
  return { violations, adapters, astCallSites, regexCallSites }
}

describe('toPage：分页容器的唯一归一处（票 6）', () => {
  it('正常负载：行与总数原样进容器', () => {
    expect(toPage([{ id: 1 }], 42)).toEqual({ items: [{ id: 1 }], total: 42 })
  })

  it('键方言在调用方就地消解：本函数只收行数组与总数', () => {
    const generated = { topics: [{ id: 7 }], page: 1, pages: 3, total: 21 }
    expect(toPage(generated.topics, generated.total)).toEqual({ items: [{ id: 7 }], total: 21 })
  })

  it('兜底与归位前的 `|| []` / `|| 0` 等价：null / undefined / 整段缺失都收成 [] 与 0', () => {
    expect(toPage(undefined, undefined)).toEqual({ items: [], total: 0 })
    expect(toPage(null, null)).toEqual({ items: [], total: 0 })
    // 后端把 items 显式回成 null 的域（AuditLogPageResult 就是 'AuditLog[] | null'）
    expect(toPage(null, 0)).toEqual({ items: [], total: 0 })
  })

  it('total 为 0 是真的 0，不被当成「缺失」；非有限值（NaN）按 0 收', () => {
    expect(toPage([{ id: 1 }], 0)).toEqual({ items: [{ id: 1 }], total: 0 })
    expect(toPage([], Number.NaN)).toEqual({ items: [], total: 0 })
  })

  it('非数组的行载荷不塞进容器（items 恒为 T[]，页面可直接迭代）', () => {
    const forged = toPage('不是数组' as unknown as unknown[], 3)
    expect(forged.items).toEqual([])
  })
})

describe('pages/ 与 components/ 不得再手搭分页容器（ADR-0060 锁）', () => {
  const { violations, adapters, astCallSites, regexCallSites } = scanTree()

  it('扫描面真的数到了 useAdminTable 的 fetch adapter（否则下一条例是恒真）', () => {
    expect(adapters).toBeGreaterThan(0)
    // 每个 useAdminTable 实例都得有一个可解析的 fetch adapter，且正则与 AST 数到同一批
    expect(adapters).toBe(astCallSites)
    expect(regexCallSites).toBe(astCallSites)
  })

  it('当前命中集合为空（票 6 落地后页面侧不存在任何一处手抄）', () => {
    expect(violations).toEqual([])
  })

  it('负向探针：按形状判红，list: / items: 两种形态与 concise body / 三元分支都逃不掉', () => {
    // ① 归位前的老形态（那 17 处里的任意一种）
    expect(
      probe('useAdminTable<Row>({ fetch: async (paging) => { const res = await api.list(paging); return { list: res.topics || [], total: res.total || 0 } } })').length
    ).toBe(1)
    // ② 换成归位后的中立键名绕过锁 —— 同样判红
    expect(probe('useAdminTable<Row>({ fetch: async () => { return { items: rows, total: n } } })').length).toBe(1)
    // ③ concise body
    expect(probe('useAdminTable<Row>({ fetch: () => ({ items: rows, total: n }) })').length).toBe(1)
    // ④ 三元里的两个分支各自成一处
    expect(probe('useAdminTable<Row>({ fetch: async () => ok ? { list: a, total: b } : { items: c, total: d } })').length).toBe(2)
  })

  it('负向探针的对照组：合法形态不得判红', () => {
    // 直接 return api 的容器（票 6 之后的标准 adapter）
    expect(probe('useAdminTable<Row>({ fetch: (paging) => api.listTopics({ page: paging.page }) })')).toEqual([])
    // 解构改名：那个 `list:` 是 composable 导出的 ref，不是页面搭的容器
    expect(probe('const { list: items, total } = useAdminTable<Row>({ fetch: () => api.list() })')).toEqual([])
    // adapter 里拼请求参数：对象字面量没被 return
    expect(probe('useAdminTable<Row>({ fetch: () => { const params = { items: [], list: [] }; return api.list(params) } })')).toEqual([])
    // 全量字典页：容器由 api 侧的构造器给
    expect(probe('useAdminTable<Row>({ fetch: async () => { const rows = await api.all(); return toPage(rows, rows.length) } })')).toEqual([])
  })
})
