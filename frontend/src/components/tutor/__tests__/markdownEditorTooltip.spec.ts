// 工具栏悬浮提示的位置锁（2026-11-18 反馈「工具按钮根本不知道怎么用」）。
//
// 坏掉的表现：悬停任何按钮都看不到说明。根因不在文案，而在几何 ——
// Vditor 把气泡放在按钮**上方**（`__n` / `__ne` / `__nw` 全是 `bottom:100%`），
// 而工具栏贴着 `.md-vditor-host` 上边界、host 又是 `overflow:hidden`（模板 :22，用来裁编辑区溢出），
// 于是气泡整块落在 host 之外被裁掉。实测（/admin/featured-content/edit/1，视口 1256×808）：
// 粗体按钮 top=725、气泡 top≈694、host top=724 ⇒ 只剩 4px 边缘可见。
//
// 为什么锁源码而不是运行期断言：这是「SFC 里必须存在某条 CSS 规则」，而 vue 的 scoped
// 样式在 vitest 里不进 DOM（插件只处理 <style> 的构建产物，测试环境拿不到 computedStyle）。
// 仓库对「文件里不该/必须有某种写法」的既有做法就是源码扫描（config/__tests__/aiFeatureDerivation.spec.ts）。
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(__dirname, '../MarkdownEditor.vue'), 'utf8')

/** 取 <style scoped> 段，避免模板里的 class 名混进来 */
function styleBlock(vue: string): string {
  const m = /<style[^>]*>([\s\S]*?)<\/style>/.exec(vue)
  if (!m) throw new Error('MarkdownEditor.vue 里找不到 <style> 段')
  return m[1]
}

/** 按选择器定位一个规则块（不引 CSS parser，够用且无依赖） */
function ruleOf(css: string, selector: RegExp): string {
  const m = selector.exec(css)
  if (!m) throw new Error('样式段里找不到选择器: ' + selector)
  const open = css.indexOf('{', m.index)
  const close = css.indexOf('}', open)
  return css.slice(open + 1, close)
}

const css = styleBlock(source)

describe('工具栏悬浮提示：必须落在 .md-vditor-host 之内', () => {
  it('气泡从按钮上方翻到下方（上方会被 host 的 overflow:hidden 裁掉）', () => {
    const rule = ruleOf(css, /\.md-vditor-host\s+:deep\(\.vditor-toolbar \.vditor-tooltipped\)::after\s*\{/)
    expect(rule).toMatch(/top:\s*100%/)
    expect(rule).toMatch(/bottom:\s*auto/)
    // 水平居中对齐：Vditor 原生是 left:50% + margin-left:-15px（硬编码 -15，宽气泡会偏）
    expect(rule).toMatch(/left:\s*50%/)
    expect(rule).toMatch(/transform:\s*translateX\(-50%\)/)
  })

  it('小三角一并隐藏（它按「在上方」定位，翻下来后会浮在按钮上）', () => {
    const rule = ruleOf(css, /\.md-vditor-host\s+:deep\(\.vditor-toolbar \.vditor-tooltipped\)::before\s*\{/)
    expect(rule).toMatch(/display:\s*none/)
  })

  it('规则必须挂在 .md-vditor-host 之下：否则盖不过 Vditor 自带的 __n/__ne/__nw', () => {
    // Vditor 的方位规则是单类选择器（.vditor-tooltipped__ne::after），
    // 我们的覆盖要带宿主与工具栏两跳才稳；同时它必须命中内置项，不能只覆盖自定义项。
    expect(css).toContain('.md-vditor-host :deep(.vditor-toolbar .vditor-tooltipped)::after')
    expect(css).not.toMatch(/\.md-vditor-host\s+:deep\(\.vditor-toolbar \[data-type/)
  })
})
