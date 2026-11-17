// 发布端渲染单点（ADR-0046 / #901 #903）。
//
// 这个组件是「预览 = 发布」的技术保证：学员阅读页与讲师/管理员编辑页预览挂的是同一份实现。
// 用例守的是**读者最终看到什么**：
//   - 章节档（可信面全集）：表格是真表格、公式是真公式、代码有高亮；
//   - 内容精选档：**表格照样是真表格**（门户与 Web 都渲染，2026-11 放开），
//     公式仍降级为源码（移动端解析器不认），越界说明只点名移动端——
//     不许渲染出门户/Web 看不到的东西，也不许把看得见的东西说成看不见（那同样在骗作者）。
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

describe('PublishMarkdown 内容精选档（门户可见子集，移动端降级）', () => {
  it('表格渲染成真表格：门户与 Web 都看得见，预览不许说看不见', () => {
    const w = render(TABLE, 'featured')
    expect(w.element.querySelector('table')).not.toBeNull()
    expect(w.element.querySelectorAll('tbody tr').length).toBe(1)
    expect(w.text()).toContain('电压过低')
  })

  it('表格不再触发越界说明（它已不在越界范围）', () => {
    const w = render(TABLE, 'featured')
    expect(w.element.querySelector('.el-alert')).toBeNull()
  })

  it('公式不渲染，源码以文本可见（移动端解析器不认公式）', () => {
    const w = render('扭矩 $T = 9550P/n$', 'featured')
    expect(w.element.querySelector('.katex')).toBeNull()
    expect(w.text()).toContain('$T = 9550P/n$')
  })

  it('越界语法给出可见说明，主语只点名移动端', () => {
    const w = render(`${TABLE}\n\n$$E=mc^2$$`, 'featured')
    const alert = w.element.querySelector('.el-alert')
    expect(alert).not.toBeNull()
    const text = w.text()
    expect(text).toContain('移动端不渲染')
    expect(text).toContain('公式')
    // 表格已放开，说明里不该再出现它
    expect(alert!.textContent).not.toContain('表格')
    // 门户也是能看见的，说明不许把门户写进去
    expect(alert!.textContent).not.toContain('门户')
  })

  it('只提示不阻断：全文照常渲染出来', () => {
    const w = render(`${TABLE}\n\n$$E=mc^2$$\n\n结尾段落`, 'featured')
    expect(w.element.querySelector('.el-alert')).not.toBeNull()
    expect(w.text()).toContain('结尾段落')
    expect(w.element.querySelector('table')).not.toBeNull()
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

describe('PublishMarkdown 内容排版（内嵌 HTML，编辑器工具栏产出）', () => {
  // 编辑器那五个按钮产出的就是这些写法；发布端必须原样认下来，否则「点了居中没反应」
  it('居中外壳在精选档与章节档都生效，且内部 markdown 照常解析', () => {
    const md = '<span style="display:block;text-align:center">**粗体** 居中一行</span>'
    for (const subset of ['featured', 'chapter'] as const) {
      const w = render(md, subset)
      const span = w.element.querySelector('span[style*="text-align:center"]')
      expect(span).not.toBeNull()
      expect(span!.querySelector('strong')).not.toBeNull()
    }
  })

  it('图片居中后的 markdown 图片仍是 <img>，不被转义成文本', () => {
    const md = '<span style="display:block;text-align:center">![图](https://example.com/a.png)</span>'
    const w = render(md, 'featured')
    const img = w.element.querySelector('img')
    expect(img?.getAttribute('src')).toBe('https://example.com/a.png')
  })

  it('文字颜色与字号原样透传（style 不被剥）', () => {
    // 声明按真实写法带上结尾分号：资源端给的 declaration 本身就以 `;` 收尾
    const md =
      '<span style="color:#DC2626">红字</span> 与 <span style="font-size:var(--text-xl, 1.25rem);">大字</span>'
    const w = render(md, 'featured')
    expect(w.html()).toContain('color:#DC2626')
    expect(w.html()).toContain('--text-xl')
  })
})
