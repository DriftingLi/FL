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
