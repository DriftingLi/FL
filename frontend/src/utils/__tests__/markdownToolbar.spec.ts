// 论坛工具栏纯编辑核心（ADR-0052）：只测「文本 + 选区 → 文本 + 选区」的变换，
// 不碰 DOM。命令语义见 utils/markdownToolbar.ts 顶部注释。
import { describe, it, expect } from 'vitest'
import {
  applyMarkdownCommand,
  MARKDOWN_TOOLBAR_ITEMS,
  MARKDOWN_TOOLBAR_DIVIDERS,
  type MarkdownCommandKey
} from '../markdownToolbar'

/** 便捷调用：只关系结果文本时用 */
function apply(text: string, start: number, end: number, command: MarkdownCommandKey) {
  return applyMarkdownCommand(text, start, end, command)
}

function textOf(text: string, start: number, end: number, command: MarkdownCommandKey) {
  return apply(text, start, end, command).text
}

describe('按钮表（提示文案单点）', () => {
  it('9 个命令，key 唯一、图标名与标签都非空', () => {
    expect(MARKDOWN_TOOLBAR_ITEMS).toHaveLength(9)
    const keys = MARKDOWN_TOOLBAR_ITEMS.map(i => i.key)
    expect(new Set(keys).size).toBe(keys.length)
    for (const item of MARKDOWN_TOOLBAR_ITEMS) {
      expect(item.label.length).toBeGreaterThan(0)
      expect(item.icon.length).toBeGreaterThan(0)
      // 中文文案：至少包含一个中日韩统一表意文字，防止有人顺手写成英文
      expect(/[\u4e00-\u9fa5]/.test(item.label)).toBe(true)
    }
  })

  it('分隔线 key 都在按钮表里', () => {
    const keys = MARKDOWN_TOOLBAR_ITEMS.map(i => i.key)
    for (const key of MARKDOWN_TOOLBAR_DIVIDERS) expect(keys).toContain(key)
  })

  it('每个命令都能跑通（新增命令漏实现会在这里炸）', () => {
    for (const item of MARKDOWN_TOOLBAR_ITEMS) {
      const r = apply('示例文本', 0, 4, item.key)
      expect(typeof r.text).toBe('string')
      expect(r.start).toBeLessThanOrEqual(r.end)
      expect(r.end).toBeLessThanOrEqual(r.text.length)
    }
  })
})

describe('加粗 / 斜体（行内包裹与去壳）', () => {
  it('无选区：插一对标记，光标落在中间', () => {
    const r = apply('abcd', 2, 2, 'bold')
    expect(r.text).toBe('ab****cd')
    expect([r.start, r.end]).toEqual([4, 4])
  })

  it('有选区：包住选中文字并保持选中', () => {
    const r = apply('abcd', 1, 3, 'bold')
    expect(r.text).toBe('a**bc**d')
    expect(r.text.slice(r.start, r.end)).toBe('bc')
  })

  it('外侧已有标记 → 去壳（切换语义）', () => {
    const r = apply('a**bc**d', 3, 5, 'bold')
    expect(r.text).toBe('abcd')
    expect(r.text.slice(r.start, r.end)).toBe('bc')
  })

  it('选区自带标记 → 去内层壳', () => {
    const r = apply('**bc**', 0, 6, 'bold')
    expect(r.text).toBe('bc')
    expect(r.text.slice(r.start, r.end)).toBe('bc')
  })

  it('斜体不会把加粗拆开（外侧是 ** 时不去壳）', () => {
    expect(textOf('a**bc**d', 3, 5, 'italic')).toBe('a***bc***d')
  })

  it('斜体去壳只看单星号', () => {
    expect(textOf('a*bc*d', 2, 4, 'italic')).toBe('abcd')
  })

  it('连续两次点击回到原文（幂等）', () => {
    const once = apply('abcd', 1, 3, 'bold')
    const twice = apply(once.text, once.start, once.end, 'bold')
    expect(twice.text).toBe('abcd')
  })
})

describe('代码', () => {
  it('空选区插一对反引号，光标在中间', () => {
    const r = apply('ab', 1, 1, 'code')
    expect(r.text).toBe('a' + '``' + 'b')
    expect([r.start, r.end]).toEqual([2, 2])
  })

  it('单行选区 → 行内代码', () => {
    expect(textOf('abcd', 1, 3, 'code')).toBe('a`bc`d')
  })

  it('两侧单反引号 → 去壳', () => {
    expect(textOf('a`bc`d', 2, 4, 'code')).toBe('abcd')
  })

  it('跨行选区 → 围栏代码块，且围栏独占一行', () => {
    const r = apply('ab\ncd', 0, 5, 'code')
    expect(r.text).toBe('```\nab\ncd\n```')
    // 选中围栏内的原文
    expect(r.text.slice(r.start, r.end)).toBe('ab\ncd')
  })

  it('选区前后还有内容时补换行，不把围栏拼在行尾', () => {
    const src = '前言 ab\ncd 后记'
    const start = src.indexOf('ab')
    const end = src.indexOf(' 后记')
    const out = apply(src, start, end, 'code').text
    expect(out).toBe('前言 \n```\nab\ncd\n```\n 后记')
  })
})

describe('链接', () => {
  it('无选区：插入占位并把 url 选中，作者直接打字覆写', () => {
    const r = apply('ab', 1, 1, 'link')
    expect(r.text).toBe('a[链接文字](url)b')
    expect(r.text.slice(r.start, r.end)).toBe('url')
  })

  it('有选区：选中文字当链接文案', () => {
    const r = apply('看这里', 0, 3, 'link')
    expect(r.text).toBe('[看这里](url)')
    expect(r.text.slice(r.start, r.end)).toBe('url')
  })
})

describe('三级标题 / 引用（行首前缀切换）', () => {
  it('无选区：给光标所在行加 ### ', () => {
    const r = apply('标题', 1, 1, 'heading3')
    expect(r.text).toBe('### 标题')
    expect(r.start).toBe(5)
  })

  it('再点一次去掉前缀（幂等）', () => {
    expect(textOf('### 标题', 3, 3, 'heading3')).toBe('标题')
  })

  it('多行选区：每一行都加', () => {
    expect(textOf('a\nb', 0, 3, 'heading3')).toBe('### a\n### b')
  })

  it('选区末端落在行首时不多改一行', () => {
    expect(textOf('a\nb', 0, 2, 'heading3')).toBe('### a\nb')
  })

  it('引用：多行加 > ，再点去掉', () => {
    const once = apply('a\nb', 0, 3, 'quote')
    expect(once.text).toBe('> a\n> b')
    expect(textOf(once.text, 0, once.text.length, 'quote')).toBe('a\nb')
  })
})

describe('三种列表', () => {
  it('无序列表：多行加 -, 已有标记则去掉', () => {
    const once = apply('a\nb', 0, 3, 'ul')
    expect(once.text).toBe('- a\n- b')
    expect(textOf(once.text, 0, once.text.length, 'ul')).toBe('a\nb')
  })

  it('有序列表：逐行编号', () => {
    expect(textOf('a\nb\nc', 0, 5, 'ol')).toBe('1. a\n2. b\n3. c')
  })

  it('有序列表 → 无序列表：先清掉原标记再换，不会叠加成 "- 1. a"', () => {
    expect(textOf('1. a\n2. b', 0, 10, 'ul')).toBe('- a\n- b')
  })

  it('无序列表 → 有序列表：重新编号', () => {
    expect(textOf('- a\n- b', 0, 7, 'ol')).toBe('1. a\n2. b')
  })

  it('任务列表：加 - [ ] ，再点回到普通段落', () => {
    const once = apply('a\nb', 0, 3, 'task')
    expect(once.text).toBe('- [ ] a\n- [ ] b')
    expect(textOf(once.text, 0, once.text.length, 'task')).toBe('a\nb')
  })

  it('任务项上点无序列表：转成普通列表项', () => {
    expect(textOf('- [ ] a', 0, 7, 'ul')).toBe('- a')
  })

  it('只改涉及行：未选中的行保持原样', () => {
    const src = 'a\nb\nc'
    // 选区覆盖第二行（b）
    expect(textOf(src, 2, 3, 'ul')).toBe('a\n- b\nc')
  })
})

describe('评审补记（#1017 评审 B1–B4）', () => {
  it('B1 整块围栏选中再点「代码」→ 去壳，不套娃', () => {
    const src = '```\nab\n```'
    const r = apply(src, 0, src.length, 'code')
    expect(r.text).toBe('ab')
    expect(r.text.slice(r.start, r.end)).toBe('ab')
  })

  it('B2 光标在文首且正文以换行开头：只改第一行空行，不吞下一行', () => {
    expect(textOf('\nabc', 0, 0, 'ul')).toBe('- \nabc')
    expect(textOf('\nabc', 0, 0, 'quote')).toBe('> \nabc')
  })

  it('B3 斜体在 ***x*** 上再点只脱一层（回到加粗），不会越点越多星', () => {
    const once = apply('a**bc**d', 3, 5, 'italic')
    expect(once.text).toBe('a***bc***d')
    const twice = apply(once.text, once.start, once.end, 'italic')
    expect(twice.text).toBe('a**bc**d')
    expect(twice.text.slice(twice.start, twice.end)).toBe('bc')
  })

  it('B4 整条链接选中再点「链接」→ 去壳回到文字', () => {
    const src = '[文字](https://a.b)'
    const r = apply(src, 0, src.length, 'link')
    expect(r.text).toBe('文字')
    expect(r.text.slice(r.start, r.end)).toBe('文字')
  })
})

describe('边界', () => {
  it('越界选区被夹回文本范围，不抛错', () => {
    const r = apply('ab', 99, 120, 'bold')
    expect(r.text).toBe('ab****')
    expect(r.end).toBeLessThanOrEqual(r.text.length)
  })

  it('空文本上执行也安全', () => {
    const r = apply('', 0, 0, 'ul')
    expect(r.text).toBe('- ')
    expect(r.start).toBeLessThanOrEqual(r.text.length)
  })

  it('纯函数：不改入参、可重复调用得到同一结果', () => {
    const src = 'a\nb'
    const first = apply(src, 0, 3, 'heading3')
    const second = apply(src, 0, 3, 'heading3')
    expect(src).toBe('a\nb')
    expect(first).toEqual(second)
  })
})
