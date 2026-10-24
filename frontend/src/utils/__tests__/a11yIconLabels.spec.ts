// 可访问名锁（W2 / #1893）：每个可交互控件（button / a / UiButton）必须有可访问名。
//
// 判据只认三种来源：`aria-label` / `aria-labelledby` / 可见文本（静态文本、插值、
// 嵌套元素里的文本、`<img alt>`）。`title` **不算** —— 它是提示而非标签，AT 的兜底
// 行为不稳定，且移动端触屏根本看不到（#1893 的判据）。
//
// 为什么是 AST 而不是正则：按钮的可访问名可能来自嵌套 `<span>{{ label }}</span>`，
// 也可能来自 `<slot/>`（内容在调用方）—— 正则两样都判不准，会一边误报一边漏报。
// 形态照 `src/utils/__tests__/statusWordsTemplate.spec.ts` 的模板扫描。
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { parse } from 'vue/compiler-sfc'
import { baseParse, type ElementNode, type TemplateChildNode } from '@vue/compiler-dom'

const SRC = resolve(__dirname, '../..')

/** 参与判定的交互标签：原生控件 + 本仓的按钮封装（图标组件如 SwitchButton 不算）。 */
const INTERACTIVE = new Set(['button', 'a', 'UiButton', 'ElButton', 'el-button'])

const TEXTY = /[\p{L}\p{N}]/u

function staticAttr(el: ElementNode, name: string): string | undefined {
  for (const p of el.props) {
    if (p.type === 6 && p.name === name) return p.value?.content ?? ''
  }
  return undefined
}

function hasBoundAttr(el: ElementNode, name: string): boolean {
  return el.props.some((p) => {
    if (p.type !== 7 || p.name !== 'bind') return false
    const arg = p.arg as string | { content?: string } | undefined
    const argName = typeof arg === 'string' ? arg : arg?.content
    return argName === name
  })
}

/** 子树里是否存在「能当名字用」的内容 */
function hasNameContent(el: ElementNode): boolean {
  for (const c of el.children ?? []) {
    if (c.type === 2) {
      if (TEXTY.test(c.content)) return true
    } else if (c.type === 5) {
      return true // 插值：内容在运行时，算有
    } else if (c.type === 1) {
      const tag = c.tag
      if (tag === 'slot') return true // 内容由调用方给
      if (tag === 'title') return true // svg <title> 就是可访问名
      if (tag === 'img' && (staticAttr(c, 'alt') !== undefined || hasBoundAttr(c, 'alt'))) return true
      if (hasNameContent(c)) return true
    }
  }
  return false
}

interface Finding {
  file: string
  line: number
  tag: string
}

function scanTemplate(templateSource: string, file: string, baseLine = 0): Finding[] {
  const findings: Finding[] = []

  const walk = (node: TemplateChildNode | ElementNode) => {
    if (node.type === 1 && INTERACTIVE.has(node.tag)) {
      const labelled =
        staticAttr(node, 'aria-label') !== undefined ||
        hasBoundAttr(node, 'aria-label') ||
        staticAttr(node, 'aria-labelledby') !== undefined ||
        hasBoundAttr(node, 'aria-labelledby')
      if (!labelled && !hasNameContent(node)) {
        findings.push({ file, line: baseLine + node.loc.start.line, tag: node.tag })
      }
    }
    // ROOT（type 0）与普通元素都要往下走：只看 type===1 会让整棵树被判空、锁变假绿
    const children = (node as { children?: TemplateChildNode[] }).children ?? []
    for (const c of children) walk(c)
  }
  walk(baseParse(templateSource) as unknown as ElementNode)
  return findings
}

function scanFile(file: string, source: string): Finding[] {
  const { descriptor } = parse(source, { filename: file })
  if (!descriptor.template) return []
  return scanTemplate(descriptor.template.content, file, descriptor.template.loc.start.line - 1)
}

function collectVueFiles(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = resolve(dir, entry)
    if (statSync(full).isDirectory()) collectVueFiles(full, out)
    else if (full.endsWith('.vue')) out.push(full)
  }
  return out
}

describe('可访问名锁（#1893）', () => {
  it('每个 button / a / UiButton 都有 aria-label、aria-labelledby 或可见文本', () => {
    const files = collectVueFiles(SRC)
    expect(files.length, '扫描面为空（目录搬走或路径写错）').toBeGreaterThan(100)

    const findings = files.flatMap((f) => scanFile(f, readFileSync(f, 'utf8')))
    const lines = findings.map((x) => `${x.file.replace(SRC + '\\', '').replace(SRC + '/', '')}:${x.line} <${x.tag}>`)
    expect(lines, '缺少可访问名的控件：').toEqual([])
  })

  it('扫描器本身没有静默失灵（正例必须报、三种负例必须放行）', () => {
    const scan = (tpl: string) => scanTemplate(tpl, 'x.vue').length
    // 正例：纯图标、无 label、无文本
    expect(scan('<button><el-icon><Close /></el-icon></button>')).toBe(1)
    // 负例：aria-label / 插值文本 / 嵌套 span 文本 / img alt / slot
    expect(scan('<button aria-label="关闭"><el-icon><Close /></el-icon></button>')).toBe(0)
    expect(scan('<button>{{ label }}</button>')).toBe(0)
    expect(scan('<button><span>{{ label }}</span></button>')).toBe(0)
    expect(scan('<a><img alt="首页" src="/logo.png" /></a>')).toBe(0)
    expect(scan('<UiButton><slot /></UiButton>')).toBe(0)
    // 负例：title 不算可访问名（判据只认 label/labelledby/文本）
    expect(scan('<button title="关闭"><el-icon><Close /></el-icon></button>')).toBe(1)
  })
})
