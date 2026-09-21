// 路由跳转单一入口锁（ADR-0060 票8b / 锁段「禁 `push('/…')` 手拼守卫」）。
//
// 两件事：
//   1. `href()` 本身——它返回**具名位置**，路径由 config/pages.ts 的描述符表经 vue-router 解析，
//      调用处不再手拼绝对路径字符串。
//   2. **锁**：`src/**` 的导航调用位不得再出现手拼的绝对路径字面量。四种形态全判红：
//        - `router.push('/x')` / `router.replace('/x')` / `router.resolve('/x')`
//        - 对象实参里的 `path:` / `redirect:` 槽（`{ path: '/login', query: … }` 那一族）
//        - 模板静态属性 `to="/x"`
//        - 模板绑定属性 `:to="'/x'"` / `:to="\`/x/${i}\`"` / `:to="{ path: '/x' }"`
//      判据按**形状**（导航调用位上以 `/` 开头的字面量）而不是按文件清单，新页面无从绕开。
//      形态照 `api/__tests__/page.spec.ts`：SFC 真解析 + AST 扫描、无白名单、断言当前命中集合
//      为空、并带「合成违例必须判红」的负向探针。之所以走 AST 而不是正则：`href('Login')` 与
//      `'/login'` 在正则下都长得像「引号里的东西」，只有类型化的表达式才分得开派生与手拼。
//
// 唯一的例外是**站点根** `"/"`：ValuationLayout 的 logo 指向 legacy `/` 重定向记录，那条记录按
// 子域名与角色条件分流、没有唯一目标页，也就没有可派生的 RouteName。这个例外盖不住真实页面——
// 下面「描述符表里没有 `path === '/'` 的记录」那条把它证成了性质：表里任何页面路径都比 `/` 长，
// 所以「以 `/` 开头且后面还有字符」覆盖了全部页面。
//
// 存量口径：断言「当前命中集合为空」，不写白名单。`__tests__` 不在射程内（与 page.spec.ts 同一
// 口径）——负向探针的字符串片段不是调用位，测试自己不该把扫描器判红。
import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'
import { parse } from 'vue/compiler-sfc'
import ts from 'typescript'
import { pages, href, type RouteName } from '../pages'

/** frontend/src（本文件在 src/config/__tests__ 下）。 */
const SRC = resolve(__dirname, '../..')

/** 一次命中：文件 + 1-based 行号 + 命中的手拼串（模板串只记头段）。 */
interface Violation {
  file: string
  line: number
  text: string
}

/** 一个扫描单元：一段可交给 TS 解析器吃的源码 + 它相对文件首行的偏移。 */
interface ScanUnit {
  content: string
  lineOffset: number
}

/** 手拼绝对路径判据：以 `/` 开头且后面还有字符（唯一例外的来由见文件头）。 */
function isHandBuiltPath(raw: string): boolean {
  return raw.startsWith('/') && raw.length > 1
}

/** 递归收集 .ts / .vue。`__tests__` 与生成物不在射程内（见文件头）。 */
function collectSourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === 'dist' || entry === '__tests__') continue
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...collectSourceFiles(full))
    else if (entry.endsWith('.ts') || entry.endsWith('.vue')) out.push(full)
  }
  return out
}

// ===== 判定面：导航调用位上的绝对路径字面量 =====

/** 剥掉括号 / 断言 / await，取字面量文本；拿不到（变量、调用、具名位置对象…）即 null = 合法。 */
function absolutePathOf(node: ts.Expression): string | null {
  let expr: ts.Expression = node
  for (;;) {
    if (
      ts.isParenthesizedExpression(expr) ||
      ts.isAsExpression(expr) ||
      ts.isSatisfiesExpression(expr) ||
      ts.isNonNullExpression(expr) ||
      ts.isAwaitExpression(expr)
    ) {
      expr = (expr as ts.ParenthesizedExpression).expression
      continue
    }
    break
  }
  if (ts.isStringLiteral(expr) || ts.isNoSubstitutionTemplateLiteral(expr)) {
    return isHandBuiltPath(expr.text) ? expr.text : null
  }
  // router.push(`/a/${x}`) —— 模板串的头段就足够判红
  if (ts.isTemplateExpression(expr) && isHandBuiltPath(expr.head.text)) return expr.head.text
  return null
}

/**
 * 一个「导航实参」上的手拼命中：直接给字面量算一次，对象字面量则连嵌套层一起查
 * `path:` / `redirect:` 槽（`{ query: { redirect: '/x' } }` 也是手拼）。
 *
 * 脚本侧的 `router.push(...)` 实参、模板侧的 `:to="..."` 绑定表达式与非 `to` 的内联表达式共用
 * 这一条判定——三处本来就是同一个位置（`:to` 就是 push 的参数）。
 */
function navArgHits(arg: ts.Expression, at: (node: ts.Node) => number, rel: string): Violation[] {
  const direct = absolutePathOf(arg)
  if (direct) return [{ file: rel, line: at(arg), text: direct }]
  const out: Violation[] = []
  const visit = (node: ts.Node): void => {
    if (ts.isPropertyAssignment(node) && /^(?:path|redirect)$/.test(node.name.getText())) {
      const value = absolutePathOf(node.initializer)
      if (value) out.push({ file: rel, line: at(node.initializer), text: value })
    }
    ts.forEachChild(node, visit)
  }
  visit(arg)
  return out
}

/** 接收者是 router / $router 的 push|replace|resolve 调用（数组的 push、字符串的 replace 因此天然排除）。 */
function isRouterNavCall(node: ts.CallExpression): boolean {
  if (!ts.isPropertyAccessExpression(node.expression)) return false
  const fn = node.expression.name.text
  if (fn !== 'push' && fn !== 'replace' && fn !== 'resolve') return false
  const recv = node.expression.expression
  return ts.isIdentifier(recv) && /^\$?router$/.test(recv.text)
}

/** 一个单元的 AST 扫描结果：router 导航调用数与手拼命中。 */
function scanUnit(unit: ScanUnit, rel: string) {
  const source = ts.createSourceFile('scan.ts', unit.content, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const violations: Violation[] = []
  let navCalls = 0
  const at = (node: ts.Node): number => unit.lineOffset + source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1
  const visit = (node: ts.Node): void => {
    if (ts.isCallExpression(node) && isRouterNavCall(node)) {
      navCalls++
      for (const arg of node.arguments) violations.push(...navArgHits(arg, at, rel))
    }
    ts.forEachChild(node, visit)
  }
  ts.forEachChild(source, visit)
  return { navCalls, violations }
}

// ===== 模板侧：to 槽 + 内联表达式 =====

/** 编译器 AST 的局部形状（只取本扫描用到的字段，不跟 vue 的内部类型较真）。 */
interface AstNode {
  type: number
  props?: Array<Record<string, any>>
  children?: AstNode[]
  name?: string
  arg?: { content?: string }
  exp?: { content?: string }
  value?: { content?: string }
  loc: { start: { line: number } }
}

const ELEMENT = 1
const ATTRIBUTE = 6
const DIRECTIVE = 7
/** 被锁的属性名（= 导航实参位）。 */
const TO = 'to'
/** vue-router 的 RouteLocationAsRelativeGeneric —— `:to` 槽的合法形态就是它，故与 push 实参同一判据。 */
const NAV_ARG_WRAPPER = '__navArg('

/** 把 `:to="…"` / 任意模板内联表达式包成一个可解析的表达式单元。 */
function asExpressionUnit(text: string, lineOffset: number): ScanUnit {
  return { content: `${NAV_ARG_WRAPPER}${text})`, lineOffset }
}

interface TemplateScan {
  toSlots: number
  /** 模板里所有指令绑定表达式的文本（`@click="$router.push('/x')"` 也在内），交脚本侧扫。 */
  expressions: ScanUnit[]
  violations: Violation[]
}

function scanTemplateRoot(root: AstNode, lineOffset: number, rel: string): TemplateScan {
  const out: TemplateScan = { toSlots: 0, expressions: [], violations: [] }
  const walk = (el: AstNode): void => {
    if (el.type !== ELEMENT) {
      // RootNode 不是元素：先钻进去，别在第一步就把扫描面判成空
      for (const child of el.children ?? []) walk(child)
      return
    }
    for (const prop of el.props ?? []) {
      const staticAttr = prop.type === ATTRIBUTE && prop.name === TO
      const boundAttr = prop.type === DIRECTIVE && prop.name === 'bind' && prop.arg?.content === TO
      if (!staticAttr && !boundAttr) {
        // 其余指令（@click / @submit / :disabled…）里也可能藏着 $router.push('/x')，
        // 一并交同一判定；to 槽自己下面单独走，不重复登记。
        if (prop.type === DIRECTIVE && prop.exp) {
          out.expressions.push(asExpressionUnit(String(prop.exp.content), lineOffset + (prop.exp.loc?.start?.line ?? prop.loc.start.line) - 1))
        }
        continue
      }
      out.toSlots++
      if (staticAttr) {
        const text = String(prop.value?.content ?? '')
        if (isHandBuiltPath(text)) out.violations.push({ file: rel, line: lineOffset + prop.loc.start.line, text })
        continue
      }
      // 绑定形态交表达式扫描（同一判据、同一处实现），这里只登记位置
      const expText = String(prop.exp?.content ?? '')
      const unit = asExpressionUnit(expText, lineOffset + (prop.exp?.loc?.start?.line ?? prop.loc.start.line) - 1)
      const source = ts.createSourceFile('to.ts', unit.content, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
      const call = ((source.statements[0] as ts.ExpressionStatement)?.expression as ts.CallExpression)?.arguments?.[0]
      if (call) {
        const at = (node: ts.Node): number =>
          unit.lineOffset + source.getLineAndCharacterOfPosition(node.getStart(source)).line
        out.violations.push(...navArgHits(call, at, rel))
      }
    }
    for (const child of el.children ?? []) walk(child)
  }
  walk(root)
  return out
}

/** 粗剥注释（只给下面那条交叉计数用——判定面全在 AST 上，不受影响）。 */
const stripComments = (text: string): string =>
  text.replace(/\/\*[\s\S]*?\*\//g, ' ').replace(/(^|[^:])\/\/[^\n]*/g, '$1')

/** 正则侧独立数一遍导航调用，与 AST 侧比对，防扫描面静默失灵。 */
const countNavCallsByRegex = (units: ScanUnit[]): number =>
  units
    .map(u => stripComments(u.content))
    .join('\n')
    .match(/\$?[Rr]outer\s*\.\s*(?:push|replace|resolve)\s*\(/g)?.length ?? 0

function scanFile(full: string) {
  const rel = relative(SRC, full).split(sep).join('/')
  const raw = readFileSync(full, 'utf8')
  const units: ScanUnit[] = []
  const violations: Violation[] = []
  let toSlots = 0

  if (rel.endsWith('.vue')) {
    const { descriptor, errors } = parse(raw)
    if (errors.length) throw errors[0]
    const template = descriptor.template
    if (template?.ast) {
      const lineOffset = template.loc.start.line - 1
      const scanned = scanTemplateRoot(template.ast as unknown as AstNode, lineOffset, rel)
      toSlots = scanned.toSlots
      violations.push(...scanned.violations)
      units.push(...scanned.expressions)
    }
    for (const block of [descriptor.script, descriptor.scriptSetup]) {
      if (!block) continue
      units.push({ content: block.content, lineOffset: raw.slice(0, block.loc.start.offset).split('\n').length - 1 })
    }
  } else {
    units.push({ content: raw, lineOffset: 0 })
  }

  let navCalls = 0
  for (const unit of units) {
    const scanned = scanUnit(unit, rel)
    navCalls += scanned.navCalls
    violations.push(...scanned.violations)
  }
  // 包装前缀会让 `__navArg(...)` 里的 router 调用数重复计一次吗？不会——包装只加函数名，不加 router.
  return { rel, violations, navCalls, toSlots, regexCalls: countNavCallsByRegex(units) }
}

function scanTree() {
  const files = collectSourceFiles(SRC)
  const violations: Violation[] = []
  let navCalls = 0
  let toSlots = 0
  let regexCalls = 0
  for (const full of files) {
    const r = scanFile(full)
    navCalls += r.navCalls
    toSlots += r.toSlots
    regexCalls += r.regexCalls
    violations.push(...r.violations)
  }
  return { files, violations, navCalls, toSlots, regexCalls }
}

// ===== 探针 =====

/** 脚本片段探针。 */
function probeScript(script: string): Violation[] {
  const rel = 'probe.ts'
  const scanned = scanUnit({ content: script, lineOffset: 0 }, rel)
  return scanned.violations
}

/** 模板探针：包成真 SFC 后只跑 `to` 槽判定（模板行号偏移对本探针无意义）。 */
function probeTo(tpl: string): Violation[] {
  const { descriptor } = parse(`<template>${tpl}</template>`)
  if (!descriptor.template?.ast) throw new Error('模板没解析出 AST，探针失效')
  return scanTemplateRoot(descriptor.template.ast as unknown as AstNode, 0, 'probe.vue').violations
}

/** 模板内联表达式探针（`@click="…"` 那一族走的是脚本侧判据）。 */
function probeExpression(expr: string): Violation[] {
  const { descriptor } = parse(`<template><div @click="${expr}"></div></template>`)
  if (!descriptor.template?.ast) throw new Error('模板没解析出 AST，探针失效')
  const scanned = scanTemplateRoot(descriptor.template.ast as unknown as AstNode, 0, 'probe.vue')
  return scanned.expressions.flatMap(u => scanUnit(u, 'probe.vue').violations)
}

const SCANNED = scanTree()

describe('href()：跳转入口由页面描述符表派生（票8b）', () => {
  it('无参即具名位置；带参把 params 原样交给 vue-router 解析', () => {
    expect(href('Login')).toEqual({ name: 'Login' })
    expect(href('StudentQuestionDetail', { id: '7' })).toEqual({ name: 'StudentQuestionDetail', params: { id: '7' } })
  })

  it('RouteName 派生自本表：每条记录都带不重复的 name、绝对 path 与 component（名单与路径同处一张表）', () => {
    const names = pages.map(p => p.name as string)
    expect(new Set(names).size).toBe(names.length)
    expect(names.length).toBeGreaterThan(70)
    for (const page of pages) {
      expect(page.path.startsWith('/'), page.name + ' 的 path 必须是绝对路径').toBe(true)
      expect(typeof page.component).toBe('function')
    }
  })

  it("表里没有 path === '/' 的记录——「站点根」例外因此盖不住任何真实页面", () => {
    expect(pages.filter(p => p.path === '/')).toEqual([])
  })
})

describe('src/** 的导航调用位不得手拼绝对路径（ADR-0060 锁）', () => {
  it('扫描面真的数到了导航调用与 to 槽（否则下一条例是恒真）', () => {
    expect(SCANNED.files.length, '扫描文件数为 0 即扫描器坏了').toBeGreaterThan(200)
    expect(SCANNED.navCalls, '一处 router 导航调用都没数到即扫描器坏了').toBeGreaterThan(50)
    expect(SCANNED.toSlots, '一处模板 to 槽都没数到即扫描器坏了').toBeGreaterThan(10)
    // 两条独立计数器数到同一批：AST 漏数或正则漏数都会在这里露出来
    expect(SCANNED.regexCalls).toBe(SCANNED.navCalls)
  })

  it('当前命中集合为空（票8b 落地后导航调用位不存在任何一处手拼路径）', () => {
    expect(SCANNED.violations).toEqual([])
  })

  it('负向探针：脚本侧三种形态各自判红', () => {
    // ① 字符串实参（迁移前的 router.push('/login')）
    expect(probeScript("router.push('/login')").map(v => v.text)).toEqual(['/login'])
    // ② 模板串（迁移前的 router.push(`/training/tutor/course/${courseId}/chapters`)）
    expect(probeScript('router.replace(`/ai-assistant/${k}`)').map(v => v.text)).toEqual(['/ai-assistant/'])
    // ③ 对象实参里的 path:（迁移前的 router.push({ path: '/login', query: { redirect } })）
    expect(probeScript("router.push({ path: '/login', query: { redirect: from } })").map(v => v.text)).toEqual(['/login'])
    // ④ redirect: 槽同样是手拼路径（ValuationFooter 迁移前那一处）
    expect(probeScript("router.push({ name: 'Login', query: { redirect: '/valuation/history' } })").map(v => v.text)).toEqual(['/valuation/history'])
    // ⑤ resolve 也算导航调用位
    expect(probeScript("const p = router.resolve('/valuation').path").map(v => v.text)).toEqual(['/valuation'])
    // ⑥ 模板内联表达式里的 $router.push（@click 那一族）
    expect(probeExpression("$router.push('/training/courses')").map(v => v.text)).toEqual(['/training/courses'])
  })

  it('负向探针：模板侧三种 to 形态各自判红', () => {
    expect(probeTo('<router-link to="/training/courses">x</router-link>').map(v => v.text)).toEqual(['/training/courses'])
    expect(probeTo("<RouterLink :to=\"'/training/task-center'\">x</RouterLink>").map(v => v.text)).toEqual(['/training/task-center'])
    expect(probeTo('<router-link :to="`/recruit/jobs/${item.id}/applications`">x</router-link>').map(v => v.text)).toEqual(['/recruit/jobs/'])
    // ⑦ 绑定对象里的 path:（ValuationFooter 迁移前那一处模板形态）
    expect(probeTo(`<router-link :to="{ path: '/login', query: { redirect: '/valuation/history' } }">x</router-link>`).map(v => v.text)).toEqual(['/login', '/valuation/history'])
  })

  it('负向探针的对照组：派生形态、动态串与非 router 接收者都不得判红', () => {
    expect(probeScript("router.push(href('Login'))")).toEqual([])
    expect(probeScript("router.push({ ...href('Login'), query: { redirect: from } })")).toEqual([])
    expect(probeScript("router.push(href('StudentFeaturedDetail', { id: String(id) }))")).toEqual([])
    expect(probeScript('router.push(target)')).toEqual([])
    // 数组的 push / 字符串的 replace：接收者不是 router，不在导航调用位上
    expect(probeScript("rows.push('/not-a-navigation')")).toEqual([])
    expect(probeScript("label.replace('/x', 'y')")).toEqual([])
    // 站点根：legacy `/` 重定向记录没有 RouteName，见文件头
    expect(probeScript("router.push('/')")).toEqual([])
    expect(probeTo('<router-link to="/">x</router-link>')).toEqual([])
    expect(probeTo("<router-link :to=\"href('Login')\">x</router-link>")).toEqual([])
    expect(probeTo('<router-link :to="target">x</router-link>')).toEqual([])
    expect(probeTo('<router-link :to="item.to || \'\'">x</router-link>')).toEqual([])
    // 非路径的动态模板串：头段不以 / 开头，交解析器判，不误伤
    expect(probeTo('<router-link :to="`doc/${id}`">x</router-link>')).toEqual([])
    expect(probeExpression("$router.push(href('CourseList'))")).toEqual([])
  })

  it('RouteName 是字面量 union：表里没有的名字写不出来（孤儿路由名从「静默断链」变成编译报错）', () => {
    // 编译期判据由下面这行的 @ts-expect-error 钉住：哪天 RouteName 退化回 string，
    // 这行会因「预期有错却没有错」让 vue-tsc 直接红。
    // @ts-expect-error 不在描述符表里的路由名派生不出来
    const orphan: RouteName = 'NoSuchRouteName'
    expect(orphan).toBe('NoSuchRouteName')
  })
})
