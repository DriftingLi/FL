// 发布端渲染单点（ADR-0046 / #901 #903）。
//
// 这个组件是「预览 = 发布」的技术保证：学员阅读页与讲师/管理员编辑页预览挂的是同一份实现。
// 用例守的是**读者最终看到什么**：
//   - 章节档（可信面全集）：表格是真表格、公式是真公式、代码有高亮；
//   - 内容精选档（三端交集）：表格与公式降级为可读文本，并给出可见说明——
//     不许渲染出门户/移动端看不到的东西（那会让预览骗作者）。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import PublishMarkdown from '../PublishMarkdown.vue'

function render(content: string, subset: 'chapter' | 'featured' = 'chapter') {
  return mount(PublishMarkdown, { props: { content, subset }, global: { plugins: [epLite()] } })
}

const TABLE = '| 故障码 | 含义 |\n| --- | --- |\n| E01 | 电压过低 |'

describe('PublishMarkdown 章节档（可信面全集）', () => {
  it('表格渲染成真正的表格元素（移动端缺口与本档无关，见 ADR-0046 矩阵）', () => {
    const w = render(TABLE)
    expect(w.element.querySelector('table')).not.toBeNull()
    expect(w.element.querySelectorAll('tbody tr').length).toBe(1)
    expect(w.text()).toContain('电压过低')
  })

  it('行内公式渲染成公式，而不是 `$...$` 源码', () => {
    const w = render('扭矩 $T = 9550P/n$ 时按此取值')
    expect(w.element.querySelector('.katex')).not.toBeNull()
    expect(w.text()).not.toContain('$T = 9550P/n$')
  })

  it('块级公式渲染成展示公式', () => {
    const w = render('$$\nE = mc^2\n$$')
    expect(w.element.querySelector('.katex-display')).not.toBeNull()
    expect(w.text()).not.toContain('$$')
  })

  it('代码块带语法高亮（章节档原本就有，不许因换渲染单点而丢）', () => {
    const w = render('```js\nconst a = 1\n```')
    expect(w.html()).toContain('hljs')
    expect(w.element.querySelector('.hljs-keyword')).not.toBeNull()
  })

  it('既有语法不回归：标题 / 列表 / 引用 / 图片 / 链接', () => {
    const w = render('# 一级标题\n\n- 项目一\n- 项目二\n\n> 引用\n\n![图](https://example.com/a.png)\n\n[手册](https://example.com/doc)')
    expect(w.element.querySelector('h1')).not.toBeNull()
    expect(w.element.querySelectorAll('li').length).toBe(2)
    expect(w.element.querySelector('blockquote')).not.toBeNull()
    expect(w.element.querySelector('img')?.getAttribute('src')).toBe('https://example.com/a.png')
    expect(w.element.querySelector('a')?.getAttribute('href')).toBe('https://example.com/doc')
  })

  it('KaTeX 保持 trust=false：\`\\href\` 之类命令不产生可点击链接', () => {
    const w = render('$\\href{javascript:alert(1)}{点我}$')
    // 命令不生效：没有 <a>，也没有 javascript: 协议
    expect(w.element.querySelector('a')).toBeNull()
    expect(w.html()).not.toContain('javascript:alert(1)"')
    expect(w.html()).not.toContain("href=\"javascript")
  })
})

describe('PublishMarkdown 内容精选档（三端交集子集）', () => {
  it('表格不渲染成表格，读者看到的是原始文本（门户与移动端就是这样）', () => {
    const w = render(TABLE, 'featured')
    expect(w.element.querySelector('table')).toBeNull()
    expect(w.text()).toContain('| 故障码 | 含义 |')
    expect(w.text()).toContain('E01')
  })

  it('公式不渲染，源码以文本可见', () => {
    const w = render('扭矩 $T = 9550P/n$', 'featured')
    expect(w.element.querySelector('.katex')).toBeNull()
    expect(w.text()).toContain('$T = 9550P/n$')
  })

  it('越界语法给出可见说明（说明里点名是哪几类）', () => {
    const w = render(`${TABLE}\n\n$$E=mc^2$$`, 'featured')
    const alert = w.element.querySelector('.el-alert')
    expect(alert).not.toBeNull()
    const text = w.text()
    expect(text).toContain('门户与移动端不渲染')
    expect(text).toContain('表格')
    expect(text).toContain('公式')
  })

  it('只提示不阻断：全文照常渲染出来', () => {
    const w = render(`${TABLE}\n\n结尾段落`, 'featured')
    expect(w.element.querySelector('.el-alert')).not.toBeNull()
    expect(w.text()).toContain('结尾段落')
  })

  it('子集内语法照常渲染，且不出现说明', () => {
    const w = render('# 标题\n\n- 列表\n\n> 引用\n\n```js\nconst a = 1\n```\n\n![图](https://example.com/a.png)\n\n[链接](https://example.com/doc)', 'featured')
    expect(w.element.querySelector('.el-alert')).toBeNull()
    expect(w.element.querySelector('h1')).not.toBeNull()
    expect(w.element.querySelectorAll('li').length).toBe(1)
    expect(w.element.querySelector('blockquote')).not.toBeNull()
    expect(w.element.querySelector('img')).not.toBeNull()
    expect(w.element.querySelector('a')?.getAttribute('href')).toBe('https://example.com/doc')
  })
})
