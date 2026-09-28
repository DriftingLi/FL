/**
 * 第十四波 B 票 10（ADR-0062 决策 10「装载流归位到 composable」）的机检。
 *
 * 三条判据都是「运行期观察不到、只能问源码形状」的那类（先例：
 * `api/__tests__/credentialScopeContract.spec.ts`、`utils/__tests__/statusWordsTemplate.spec.ts`）：
 *
 * R1 筛选项变化不得直调 `handlePageChange()`。
 *    `handlePageChange` 的语义是「用户点了第 N 页 ⇒ 重装第 N 页」，它**不回第一页**。
 *    拿它处理筛选切换 = 在新筛选条件下请求旧页码那个子集 ⇒ 假空态
 *    （PointsLedger 的实测缺陷：翻到第 4 页点「支出」，屏上「暂无积分流水」而记录确实存在）。
 *    正解是 `filterDeps`（#1054 那条声明式入口）。
 *
 * R2 读当前证件的装载函数必须声明进 `facets`。
 *    ADR 的锁原文：「facet 声明槽使用后 `pages/` 不得再出现只挂在 `onMounted` 上的证件相关装载流」。
 *    判据形状：`async function X(...)` 的体内读 `credentialStore.current` ⇒ X 必须出现在
 *    同文件的 `facets:` 声明里（新增的旁路装载流因此必须回来登记，不能悄悄挂在 onMounted 上）。
 *
 * R3 页面不得再自带一条 `watch(() => credentialStore.current…)` 来重装自己的列表/facet。
 *    证件失效时机的判据宿主是 useAsyncPage（`credentialScoped` + `facets`）；
 *    页面第二处 watch 就是「7 个引用当前证件的文件里只有 2 个真在重装」那种漂移的重犯路径。
 *    （`components/credential/CredentialSwitcher.vue` 是切换器本身，不在 pages/ 扫描面内。）
 */
import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
// R4 用编译器 API 判「写页面 ref」：文本法会把 x.value === … 这类**读**误判成写（实现时踩过）。
import ts from 'typescript'

const PAGES = resolve(__dirname, '../../pages')

/** 递归列出扫描面上的 .vue（不含 spec）。 */
function pageFiles(dir: string = PAGES): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const full = resolve(dir, entry.name)
    if (entry.isDirectory()) return entry.name === '__tests__' ? [] : pageFiles(full)
    return entry.name.endsWith('.vue') ? [full] : []
  })
}

const read = (file: string) => readFileSync(file, 'utf8')
const label = (file: string) => file.slice(PAGES.length + 1).split('\\').join('/')

/** 命名装载函数的函数体（花括号配平）：`async function X(` 与 `const X = async (` 两种形态。 */
function functionBody(text: string, name: string): string {
  const re = new RegExp(`(?:async\\s+function\\s+${name}\\s*\\(|const\\s+${name}\\s*=\\s*async\\s*\\()`)
  const hit = re.exec(text)
  if (!hit) return ''
  const open = text.indexOf('{', hit.index)
  if (open < 0) return ''
  let depth = 0
  for (let i = open; i < text.length; i++) {
    if (text[i] === '{') depth++
    else if (text[i] === '}') {
      depth--
      if (depth === 0) return text.slice(open, i + 1)
    }
  }
  return text.slice(open)
}

/** 去掉注释行（说明文字不算代码事实，口径照 credentialScopeScan）。 */
const stripComments = (text: string) =>
  text
    .split('\n')
    .filter(line => !/^\s*(\/\/|<!--|\*|-->)/.test(line))
    .join('\n')

describe('票 10 R1：筛选项变化不得直调 handlePageChange()', () => {
  it('页面里 handlePageChange 只能作为分页事件的绑定值出现（不得被调用）', () => {
    const offenders: string[] = []
    for (const file of pageFiles()) {
      const lines = stripComments(read(file)).split('\n')
      lines.forEach((line, i) => {
        // 只抓「调用形态」：`handlePageChange(` 且不是本页自己声明的同名函数
        if (!/handlePageChange\s*\(/.test(line)) return
        if (/function\s+handlePageChange/.test(line)) return
        offenders.push(`${label(file)}:${i + 1} ${line.trim()}`)
      })
    }
    expect(offenders).toEqual([])
  })

  it('判据本身有效：把 PointsLedger 的旧写法塞回去就一定红（防守卫退化成假绿）', () => {
    const stale = `
const filter = ref('all')
const { handlePageChange } = useAsyncPage(loader)
const onClick = (v) => { filter.value = v; handlePageChange() }
`
    const hit = stripComments(stale)
      .split('\n')
      .some(line => /handlePageChange\s*\(/.test(line) && !/function\s+handlePageChange/.test(line))
    expect(hit).toBe(true)
  })
})

describe('票 10 R2：证件相关的装载流必须声明进 facets', () => {
  /** 一个文件里「被登记的装载流名字」：facets 槽里的名字 + 作为 useAsyncPage loader 实参的名字。 */
  function registeredLoaders(text: string): Set<string> {
    const names = new Set<string>()
    const declared = /facets\s*:\s*\[([\s\S]*?)\]/.exec(text)
    if (declared) for (const w of declared[1].match(/\w+/g) ?? []) names.add(w)
    for (const m of text.matchAll(/useAsyncPage\s*(?:<[^>]*>)?\s*\(\s*(\w+)/g)) names.add(m[1])
    return names
  }

  /** 被命名的装载函数：`async function X(` 与 `const X = async (` 两种形态。 */
  function namedLoaders(text: string): string[] {
    const names: string[] = []
    for (const m of text.matchAll(/async\s+function\s+(\w+)\s*\(/g)) names.push(m[1])
    for (const m of text.matchAll(/const\s+(\w+)\s*=\s*async\s*\(/g)) names.push(m[1])
    return names
  }

  it('每个读当前证件的命名装载函数都登记在 facets 槽（或本就是列表 loader）', () => {
    const offenders: string[] = []
    for (const file of pageFiles()) {
      const text = stripComments(read(file))
      const registered = registeredLoaders(text)
      for (const name of namedLoaders(text)) {
        if (!/credentialStore\.current/.test(functionBody(text, name))) continue
        if (!registered.has(name)) offenders.push(`${label(file)}: ${name}() 读当前证件却没进 facets 声明槽`)
      }
    }
    expect(offenders).toEqual([])
  })

  it('判据本身有效：旧写法（旁路装载只挂在 onMounted 上）一定红', () => {
    const stale = `
async function loadTags() {
  const data = await trainingApi.getTags(credentialStore.current?.id)
  tags.value = data.tags || []
}
onMounted(() => { loadCardData(); loadTags() })
`
    const registered = registeredLoaders(stale)
    const flagged = namedLoaders(stale).filter(n => /credentialStore\.current/.test(functionBody(stale, n)))
    expect(flagged).toEqual(['loadTags'])
    expect(registered.has('loadTags')).toBe(false)
  })

  it('存量三处旁路装载流确实归位了（facets 声明在场，而不是靠注释宣称）', () => {
    const facetOwners = pageFiles()
      .filter(file => /facets\s*:\s*\[/.test(stripComments(read(file))))
      .map(label)
      .sort()
    expect(facetOwners).toEqual([
      'student/CourseList.vue',
      'student/Materials.vue',
      'student/QuestionBank.vue'
    ])
  })
})

describe('票 10 R3：页面不再自带第二条证件 watch', () => {
  it('pages/ 下没有 watch(() => credentialStore.current…) 形态的手写失效刷新', () => {
    const offenders: string[] = []
    for (const file of pageFiles()) {
      const text = stripComments(read(file))
      for (const m of text.matchAll(/watch\s*\(([\s\S]{0,120})/g)) {
        if (m[1].includes('credentialStore.current')) offenders.push(label(file))
      }
    }
    expect(offenders).toEqual([])
  })
})

// ===== R4（ADR-0069 决策 1/2）：loader 只取数 =====

/**
 * R4 写回形状（ADR-0069 决策 1/2）：useAsyncPage 的 loader **只取数** —— 页面 ref 的写回
 * 一律走 apply 槽，由 composable 在「批次代数校验通过」之后调用。写回一旦回到 loader 体内，
 * 单页层面的行为锁测不到（旧轮的数据在守卫看到之前就已落地），所以这一条必须是结构锁、零豁免
 * （存量 43 文件已全量迁移）。
 *
 * 判据形状：扫 useAsyncPage 调用的 loader 实参源码（内联箭头，或按名字解析同文件的具名 loader），
 * 出现「写页面 ref」即红：
 *   - 赋值给 x.value（含 += / ??= 等复合赋值）、x.value[i] = …
 *   - 对 x.value 调**可变**方法（push / splice / sort / set / delete …）
 * 只读用法（x.value.trim() / x.value.map(…) / 条件里的 x.value）不算 —— 判据按 AST 判写目标，
 * 不看语句文本（文本法会把 'activeType.value === …' 误判成写，实现时踩过）。
 */
const MUTATING_CALLS = new Set([
  'push', 'pop', 'shift', 'unshift', 'splice', 'sort', 'reverse', 'fill', 'copyWithin',
  'set', 'add', 'delete', 'clear'
])

const SRC = resolve(__dirname, '../..')
const srcLabel = (file: string) => file.slice(SRC.length + 1).split('\\').join('/')

/** 递归列出扫描面上的 .vue/.ts（排除 __tests__ 与 spec）。 */
function sourceFiles(dir: string = SRC): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const full = resolve(dir, entry.name)
    if (entry.isDirectory()) return entry.name === '__tests__' ? [] : sourceFiles(full)
    return /[.](vue|ts)$/.test(entry.name) && !entry.name.endsWith('.spec.ts') ? [full] : []
  })
}

/** 剥掉 () / as / satisfies / ! 这些不改变「写目标」身份的包装。 */
function unwrapNode(node: ts.Node): ts.Node {
  let n = node
  while (
    ts.isParenthesizedExpression(n) || ts.isAsExpression(n) || ts.isSatisfiesExpression(n) ||
    ts.isTypeAssertionExpression(n) || ts.isNonNullExpression(n)
  ) {
    n = n.expression
  }
  return n
}

/** x.value / x.value[i] 的根名字（不是这两种形状则 null）。 */
function refTargetName(expr: ts.Node): string | null {
  if (ts.isPropertyAccessExpression(expr) && expr.name.text === 'value' && ts.isIdentifier(expr.expression)) {
    return expr.expression.text
  }
  if (
    ts.isElementAccessExpression(expr) && ts.isPropertyAccessExpression(expr.expression) &&
    expr.expression.name.text === 'value' && ts.isIdentifier(expr.expression.expression)
  ) {
    return expr.expression.expression.text
  }
  return null
}

/** 该节点自身是否是「写页面 ref」（不含子节点）。 */
function isRefWrite(node: ts.Node): string | null {
  if (ts.isBinaryExpression(node)) {
    const op = node.operatorToken.kind
    if (op >= ts.SyntaxKind.FirstAssignment && op <= ts.SyntaxKind.LastAssignment) {
      return refTargetName(unwrapNode(node.left))
    }
    return null
  }
  if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && MUTATING_CALLS.has(node.expression.name.text)) {
    return refTargetName(unwrapNode(node.expression.expression))
  }
  if (
    (ts.isPrefixUnaryExpression(node) || ts.isPostfixUnaryExpression(node)) &&
    (node.operator === ts.SyntaxKind.PlusPlusToken || node.operator === ts.SyntaxKind.MinusMinusToken)
  ) {
    return refTargetName(unwrapNode(node.operand))
  }
  return null
}

/** 子树里所有「写页面 ref」的目标名（去重）。 */
function refWritesIn(node: ts.Node): string[] {
  const names = new Set<string>()
  const visit = (n: ts.Node): void => {
    const hit = isRefWrite(n)
    if (hit) names.add(hit)
    ts.forEachChild(n, visit)
  }
  visit(node)
  return [...names]
}

/** 源码里的 script 块（.ts 视作整文件一块）。 */
function scriptBlocks(text: string): Array<{ start: number; end: number }> {
  const blocks: Array<{ start: number; end: number }> = []
  const re = /<script\b[^>]*>/g
  let m: RegExpExecArray | null
  while ((m = re.exec(text))) {
    const open = m.index + m[0].length
    const close = text.indexOf('</script>', open)
    if (close < 0) continue
    blocks.push({ start: open, end: close })
  }
  return blocks.length ? blocks : [{ start: 0, end: text.length }]
}

/** 扫一份源码文本，返回 useAsyncPage 的 loader 内出现写回的位置描述（空数组 = 合规）。 */
function loaderWriteOffenders(text: string): string[] {
  const offenders: string[] = []
  for (const block of scriptBlocks(text)) {
    const slice = text.slice(block.start, block.end)
    const sf = ts.createSourceFile('inline.ts', slice, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    // 同文件的具名函数体：命名 loader（useAsyncPage(loadX, …)）要按名字解析再扫
    const locals = new Map<string, ts.Node>()
    const collect = (n: ts.Node): void => {
      if (ts.isFunctionDeclaration(n) && n.name && n.body) locals.set(n.name.text, n.body)
      if (ts.isVariableStatement(n)) {
        for (const d of n.declarationList.declarations) {
          if (
            ts.isIdentifier(d.name) && d.initializer &&
            (ts.isArrowFunction(d.initializer) || ts.isFunctionExpression(d.initializer)) && d.initializer.body
          ) {
            locals.set(d.name.text, d.initializer.body)
          }
        }
      }
      ts.forEachChild(n, collect)
    }
    collect(sf)
    const lineOf = (pos: number) => text.slice(0, block.start + pos).split('\n').length
    const visit = (node: ts.Node): void => {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'useAsyncPage') {
        let loader: ts.Node | null = node.arguments.length ? unwrapNode(node.arguments[0]) : null
        if (loader && ts.isIdentifier(loader)) loader = locals.get(loader.text) ?? null
        if (loader) {
          for (const name of refWritesIn(loader)) {
            offenders.push(name + '.value 写回（loader 内），行 ' + lineOf(node.getStart()))
          }
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(sf)
  }
  return offenders
}

describe('R4 写回形状（ADR-0069）：useAsyncPage 的 loader 只取数', () => {
  it('src 下所有 useAsyncPage 的 loader 内没有页面 ref 写回（零豁免）', () => {
    const offenders: string[] = []
    for (const file of sourceFiles()) {
      for (const hit of loaderWriteOffenders(read(file))) {
        offenders.push(srcLabel(file) + ' ' + hit)
      }
    }
    expect(offenders).toEqual([])
  })

  it('判据本身有效：写回留在 loader 里（旧形状）一定红', () => {
    const stale = [
      '<script setup lang="ts">',
      'const items = ref<Item[]>([])',
      'const total = ref(0)',
      'const { run } = useAsyncPage(async () => {',
      '  const res = await listApi()',
      '  items.value = res.items',
      '  total.value = res.total',
      '}, { itemsRef: items })',
      '</' + 'script>'
    ].join('\n')
    expect(loaderWriteOffenders(stale)).toEqual(['items.value 写回（loader 内），行 4', 'total.value 写回（loader 内），行 4'])
  })

  it('判据不误伤：只读用法与可变方法之外的调用都不算写回', () => {
    const clean = [
      '<script setup lang="ts">',
      'const items = ref<Item[]>([])',
      'const total = ref(0)',
      'const keyword = ref("")',
      'async function loadOnce() {',
      '  const kw = keyword.value.trim()',
      '  const res = await listApi({ q: kw, page: page.value })',
      '  return res',
      '}',
      'const { run } = useAsyncPage(loadOnce, {',
      '  itemsRef: items,',
      '  apply: (res) => { items.value = res.items; total.value = res.total }',
      '})',
      '</' + 'script>'
    ].join('\n')
    expect(loaderWriteOffenders(clean)).toEqual([])
  })

  it('判据本身有效：x.value.push(...) 这类可变调用同样算写回', () => {
    const stale = [
      '<script setup lang="ts">',
      'const rows = ref<Item[]>([])',
      'const { run } = useAsyncPage(async () => {',
      '  const res = await listApi()',
      '  rows.value.push(...res.items)',
      '})',
      '</' + 'script>'
    ].join('\n')
    expect(loaderWriteOffenders(stale)).toEqual(['rows.value 写回（loader 内），行 3'])
  })
})

/**
 * R4c loader 必须**回传数据**：apply 的入参就是 loader 的返回值 —— 一个不返回任何东西的 loader
 * 只能靠「调本页/别的 composable 的写状态函数」落地，而那个形状 R4 看不见（写回在别的函数体里）。
 *
 * 存量的三处已由 #1355 全部销账（这张表的用途正是让它变短）：
 * - `pages/admin/CourseCatalog.vue` → `useCourseCatalog` 给出 `loadCatalog()`（只取数、原样回传）
 *   与 `applyCatalog()`（写回）两条出口；管理端那一次 `Promise.all` 顺带取回的课程行改成 adapter
 *   回传载荷的额外键（`CourseCatalogAdapter<R>` 泛型化），不再在 adapter 体内写页面 ref；
 * - `pages/student/Dashboard.vue` → 课程/最近学习两路 helper 直接回传数据，统计那路改用
 *   `useRoleDashboard` 的 `fetchStats()` + `applyStats()`；
 * - `pages/admin/ValuationConfigManage.vue` → 清草稿那段搬进 `useAsyncPage` 新增的 `onError` 槽
 *   （与 `apply` 同一条代数判据），loader 因此只剩取数。
 *
 * 将来若又出现「只调函数、不回传」的 loader，必须重新加条目**并附理由**（理由是「要连同另一个
 * module 的契约一起改」，不是「不想改」），而不是让锁红着绕过。
 */
const NO_RETURN_ALLOWLIST: Record<string, string> = {}

/** 扫一份源码文本，返回「块体 loader 里没有任何带值 return」的位置描述。 */
function loadersWithoutReturnValue(text: string): string[] {
  const offenders: string[] = []
  for (const block of scriptBlocks(text)) {
    const slice = text.slice(block.start, block.end)
    const sf = ts.createSourceFile('inline.ts', slice, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    const locals = new Map<string, ts.Node>()
    const collect = (n: ts.Node): void => {
      if (ts.isFunctionDeclaration(n) && n.name && n.body) locals.set(n.name.text, n.body)
      if (ts.isVariableStatement(n)) {
        for (const d of n.declarationList.declarations) {
          if (
            ts.isIdentifier(d.name) && d.initializer &&
            (ts.isArrowFunction(d.initializer) || ts.isFunctionExpression(d.initializer)) && d.initializer.body
          ) {
            locals.set(d.name.text, d.initializer.body)
          }
        }
      }
      ts.forEachChild(n, collect)
    }
    collect(sf)
    const visit = (node: ts.Node): void => {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'useAsyncPage') {
        let loader: ts.Node | null = node.arguments.length ? unwrapNode(node.arguments[0]) : null
        if (loader && ts.isIdentifier(loader)) loader = locals.get(loader.text) ?? null
        if (loader && ts.isArrowFunction(loader) && ts.isBlock(loader.body)) {
          let hasValueReturn = false
          const scanRet = (n: ts.Node): void => {
            if (ts.isReturnStatement(n) && n.expression) hasValueReturn = true
            ts.forEachChild(n, scanRet)
          }
          scanRet(loader.body)
          if (!hasValueReturn) offenders.push('useAsyncPage 的 loader 没有 return 值')
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(sf)
  }
  return offenders
}

describe('R4c 写回槽的入参来源（ADR-0069）：loader 必须回传数据', () => {
  it('src 下没有「不带值 return」的 loader（#1355 销账后登记表为空，且不得留死条目）', () => {
    const offenders: string[] = []
    for (const file of sourceFiles()) {
      const key = srcLabel(file)
      if (loadersWithoutReturnValue(read(file)).length) offenders.push(key)
    }
    // ① 实际违规必须全部登记（新增一个「只调函数、不回传」的 loader 即红）
    expect(offenders.filter(f => !(f in NO_RETURN_ALLOWLIST))).toEqual([])
    // ② 登记表不得留死条目（已迁移的文件必须从表里销账）
    expect(Object.keys(NO_RETURN_ALLOWLIST).filter(f => !offenders.includes(f))).toEqual([])
  })

  it('判据本身有效：只调写状态函数、不回传的 loader 一定红', () => {
    const stale = [
      '<script setup lang="ts">',
      'const { run } = useAsyncPage(async () => {',
      '  await loadCourses()',
      '})',
      '</' + 'script>'
    ].join('\n')
    expect(loadersWithoutReturnValue(stale)).toEqual(['useAsyncPage 的 loader 没有 return 值'])
  })
})

// ===== R5（#1354）：composable 内部的写回也必须排在批次代数比对之后 =====

/**
 * R5 写回代数（#1354，判据源仍是 ADR-0069 决策 1）：`useAdminTable.load()` 在
 * `await options.fetch(...)` 之后直接写 `list.value`/`total.value`，没有任何代数校验 ⇒
 * 两次装载重叠时后到的旧响应盖掉新状态（与真实缺陷 #5 同形）。
 *
 * **为什么不是将本件塞进 R4 的扫描面**：R4 判的是「useAsyncPage 的 loader 参数体内写**页面** ref」，
 * 而本件的 `list`/`total` 住在 composable 内部（interface 就拥有它们），页面侧的 fetch adapter
 * 只回传 `Page<T>`（容器形状另有锁：`api/__tests__/page.spec.ts`）⇒ R4 在本件上没有任何可扫的面，
 * 硬套只会扫到 0 处并恒绿。可复用的形状是**同一条判据换宿主**：把「写回之前必须有一次批次代数比对」
 * 直接钉在装载函数体内（`useAdminTable.load` / `useAsyncPage.run` / `useAsyncPage.loadMore`）。
 *
 * 判据形状（按 AST 位置，不看语句文本）：装载函数体内每一次 `x.value =` 写回、以及每一次写回槽调用
 * （`options.apply?.(res)` / `options.onError?.(error)`），若它之前出现过 `await`，那么在
 * 「最近一次 await」与「这一笔落地」之间必须有一次把批次代数计数器（`generation`）拿来比对的表达式。
 * 守卫写在写回**之后**就是没有守卫（正是 #1354 的形状）；await 之前的写回
 * （`loading.value = true` 这类起飞前的复位）没有竞态可言，不算违规。
 * 另带「找不到被测装载函数即红」的防空转半边 —— 函数改名或扫描面失灵时不得静默放行。
 */
const BATCH_COUNTER = 'generation'

/**
 * 写回槽的名字：`options.apply?.(res)` / `options.onError?.(error)` 这两个**调用点**与页面 ref 写回等价
 * —— 它们的整个用途就是把页面的写回搬到代数校验之后，挪回校验之前等于没有守卫
 * （#1355 实施时实测：只扫 `x.value =` 会让「apply 提前到守卫前」这种退化悄悄溜过 R5）。
 */
const WRITE_BACK_SLOTS = new Set(['apply', 'onError'])

/** 一次「与批次代数的比对」：`gen !== generation` / `generation === gen` 这类比较表达式。 */
function isGenerationComparison(node: ts.Node): boolean {
  if (!ts.isBinaryExpression(node)) return false
  const k = node.operatorToken.kind
  if (
    k !== ts.SyntaxKind.ExclamationEqualsToken && k !== ts.SyntaxKind.ExclamationEqualsEqualsToken &&
    k !== ts.SyntaxKind.EqualsEqualsToken && k !== ts.SyntaxKind.EqualsEqualsEqualsToken
  ) {
    return false
  }
  const sides = [node.left, node.right].filter(ts.isIdentifier).map(n => n.text)
  // 必须是一边批次计数器、另一边是本轮捕获值（`generation !== generation` 这种恒假不算守卫）
  return sides.length === 2 && sides.includes(BATCH_COUNTER) && sides.some(s => s !== BATCH_COUNTER)
}

/** R5 用：写回节点连同位置（判形状复用 R4 的 `isRefWrite`，但比先后要知道排在哪一列）。 */
function refWriteAt(node: ts.Node): { name: string; pos: number } | null {
  const name = isRefWrite(node)
  return name ? { name, pos: node.getStart() } : null
}

/** 装载函数体内「写回（或写回槽调用）排在最近一次 await 之后、代数比对之前」的违规描述。 */
function writeBackWithoutGuard(text: string, fnName: string): string[] {
  const offenders: string[] = []
  for (const block of scriptBlocks(text)) {
    const slice = text.slice(block.start, block.end)
    const sf = ts.createSourceFile('inline.ts', slice, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    const locals = new Map<string, ts.Node>()
    const collect = (n: ts.Node): void => {
      if (ts.isFunctionDeclaration(n) && n.name && n.body) locals.set(n.name.text, n.body)
      if (ts.isVariableStatement(n)) {
        for (const d of n.declarationList.declarations) {
          if (
            ts.isIdentifier(d.name) && d.initializer &&
            (ts.isArrowFunction(d.initializer) || ts.isFunctionExpression(d.initializer)) && d.initializer.body
          ) {
            locals.set(d.name.text, d.initializer.body)
          }
        }
      }
      ts.forEachChild(n, collect)
    }
    collect(sf)
    const body = locals.get(fnName)
    if (!body) {
      offenders.push(`扫描面空转：找不到装载函数 ${fnName}()`)
      continue
    }
    const awaits: number[] = []
    const guards: number[] = []
    const writes: Array<{ name: string; pos: number }> = []
    const visit = (n: ts.Node): void => {
      if (ts.isAwaitExpression(n)) awaits.push(n.getStart())
      if (isGenerationComparison(n)) guards.push(n.getStart())
      const write = refWriteAt(n)
      if (write) writes.push({ name: write.name + '.value 写回', pos: write.pos })
      // 写回槽的调用点与页面 ref 写回等价：它把「页面的写回」搬到守卫之后，挪回守卫之前就等于没有守卫
      if (ts.isCallExpression(n) && ts.isPropertyAccessExpression(n.expression) && WRITE_BACK_SLOTS.has(n.expression.name.text)) {
        writes.push({ name: n.expression.name.text + ' 槽调用', pos: n.getStart() })
      }
      ts.forEachChild(n, visit)
    }
    visit(body)
    for (const w of writes) {
      const prior = awaits.filter(a => a < w.pos)
      if (!prior.length) continue
      const lastAwait = Math.max(...prior)
      if (!guards.some(g => g > lastAwait && g < w.pos)) {
        offenders.push(`${w.name}排在最近一次 await 之后、代数比对之前（${fnName} 内无守卫）`)
      }
    }
  }
  return offenders
}

const COMPOSABLE_SRC = resolve(__dirname, '..')

describe('R5 写回代数（#1354）：composable 的写回排在批次代数比对之后', () => {
  /** 三个「await 回来后写状态」的宿主：本件补的 useAdminTable.load 与真源 useAsyncPage 的两条装载流。 */
  const targets: Array<{ file: string; fn: string }> = [
    { file: resolve(COMPOSABLE_SRC, 'useAdminTable.ts'), fn: 'load' },
    { file: resolve(COMPOSABLE_SRC, 'useAsyncPage.ts'), fn: 'run' },
    { file: resolve(COMPOSABLE_SRC, 'useAsyncPage.ts'), fn: 'loadMore' }
  ]

  it('装载函数体内每一次写回都在代数比对之后（含「函数不存在即红」的防空转半边）', () => {
    const offenders: string[] = []
    for (const t of targets) {
      for (const hit of writeBackWithoutGuard(read(t.file), t.fn)) {
        offenders.push(`${srcLabel(t.file)} ${t.fn}() ${hit}`)
      }
    }
    expect(offenders).toEqual([])
  })

  it('判据本身有效：await 之后直接写回（#1354 修复前的形状）一定红', () => {
    const stale = [
      'async function load() {',
      '  loading.value = true',
      '  const result = await options.fetch(paging, payload)',
      '  list.value = result.items',
      '  total.value = result.total',
      '  loading.value = false',
      '}'
    ].join('\n')
    expect(writeBackWithoutGuard(stale, 'load')).toEqual([
      'list.value 写回排在最近一次 await 之后、代数比对之前（load 内无守卫）',
      'total.value 写回排在最近一次 await 之后、代数比对之前（load 内无守卫）',
      'loading.value 写回排在最近一次 await 之后、代数比对之前（load 内无守卫）'
    ])
  })

  it('判据本身有效：守卫写在写回之后等于没有守卫', () => {
    const late = [
      'async function load() {',
      '  const gen = generation + 1',
      '  const result = await options.fetch(paging, payload)',
      '  list.value = result.items',
      '  if (gen !== generation) return',
      '}'
    ].join('\n')
    expect(writeBackWithoutGuard(late, 'load')).toEqual([
      'list.value 写回排在最近一次 await 之后、代数比对之前（load 内无守卫）'
    ])
  })

  it('判据不误伤：await 之前的起飞前复位与有守卫的写回都不算违规', () => {
    const clean = [
      'let generation = 0',
      'async function load() {',
      '  const gen = generation + 1',
      '  loading.value = true',
      '  loadError.value = false',
      '  const result = await options.fetch(paging, payload)',
      '  if (gen !== generation) return',
      '  list.value = result.items',
      '  total.value = result.total',
      '}'
    ].join('\n')
    expect(writeBackWithoutGuard(clean, 'load')).toEqual([])
  })

  it('防空转半边有效：装载函数改名或不存在时判红，不静默放行', () => {
    const missing = ['async function loadSomethingElse() {', '  list.value = []', '}'].join('\n')
    expect(writeBackWithoutGuard(missing, 'load')).toEqual(['扫描面空转：找不到装载函数 load()'])
  })
})


