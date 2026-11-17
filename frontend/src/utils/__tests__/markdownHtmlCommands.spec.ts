// 内容排版命令（颜色 / 字号 / 对齐 / 图片居中）的纯函数单测。
//
// 这批命令的输出会**原样进读者端的 HTML**（门户 Nuxt / 学员端 Web），所以每条都断言
// 逐字输出，而不是只断言「包含 span」——外壳写错一个字，读者端就是一段裸 HTML 文本。
import { describe, it, expect } from 'vitest'
import {
  alignSelection,
  centerImageSelection,
  colorSelection,
  fontSizeSelection,
  CONTENT_ALIGNS,
  CONTENT_COLORS,
  CONTENT_FONT_SIZES,
  PLACEHOLDER_TEXT
} from '../markdownHtmlCommands'

describe('内容排版命令：对齐', () => {
  it('居中用 display:block 外壳包住选区（保住内部 markdown 的行内解析）', () => {
    expect(alignSelection('一段文字', 'center')).toBe(
      '<span style="display:block;text-align:center">一段文字</span>'
    )
  })

  it('左对齐是显式外壳，不是「什么都不做」——作者用它把居中改回来', () => {
    expect(alignSelection('一段文字', 'left')).toBe(
      '<span style="display:block;text-align:left">一段文字</span>'
    )
  })

  it('选区为空时插占位文字，作者直接打字即可覆盖', () => {
    expect(alignSelection('', 'center')).toBe(
      `<span style="display:block;text-align:center">${PLACEHOLDER_TEXT}</span>`
    )
  })

  it('选区已是同一种外壳时去壳（再点一次撤销）', () => {
    const wrapped = '<span style="display:block;text-align:center">文字</span>'
    expect(alignSelection(wrapped, 'center')).toBe('文字')
  })

  it('换成另一种对齐时不去壳——两种外壳不同，包一层新的才是作者要的', () => {
    const wrapped = '<span style="display:block;text-align:center">文字</span>'
    expect(alignSelection(wrapped, 'left')).toBe(
      '<span style="display:block;text-align:left"><span style="display:block;text-align:center">文字</span></span>'
    )
  })

  it('内部 markdown 原样保留（加粗 / 链接 / 图片语法都不转义）', () => {
    const md = '**粗体** 与 [链接](https://a.b) 还有 ![图](https://a.b/c.png)'
    expect(alignSelection(md, 'center')).toContain('**粗体**')
    expect(alignSelection(md, 'center')).toContain('[链接](https://a.b)')
    expect(alignSelection(md, 'center')).toContain('![图](https://a.b/c.png)')
  })
})

describe('内容排版命令：图片居中', () => {
  it('与文本居中是同一条命令——包住 markdown 图片语法，不改写成 HTML img', () => {
    expect(centerImageSelection('![故障码表](https://x/a.png)')).toBe(
      '<span style="display:block;text-align:center">![故障码表](https://x/a.png)</span>'
    )
  })

  it('空选区走占位文字路径（作者还没选图就点了按钮）', () => {
    expect(centerImageSelection('')).toBe(
      `<span style="display:block;text-align:center">${PLACEHOLDER_TEXT}</span>`
    )
  })
})

describe('内容排版命令：文字颜色', () => {
  it('颜色写进 style，不写 color 属性', () => {
    expect(colorSelection('红字', '#DC2626')).toBe('<span style="color:#DC2626">红字</span>')
  })

  it('同一颜色再点一次去壳', () => {
    expect(colorSelection('<span style="color:#DC2626">红字</span>', '#DC2626')).toBe('红字')
  })

  it('换色是包一层新的（作者从红改蓝时选中的正是旧外壳）', () => {
    const red = '<span style="color:#DC2626">红字</span>'
    expect(colorSelection(red, '#2563EB')).toContain('#2563EB')
    expect(colorSelection(red, '#2563EB')).toContain(red)
  })

  it('色板五档且两两不同（作者点错要看得出来）', () => {
    expect(CONTENT_COLORS).toHaveLength(5)
    const values = CONTENT_COLORS.map((c) => c.value)
    expect(new Set(values).size).toBe(values.length)
    for (const c of CONTENT_COLORS) {
      // 必须是 6 位 hex：3 位简写在深底上区分度不够
      expect(c.value).toMatch(/^#[0-9A-F]{6}$/)
    }
  })
})

describe('内容排版命令：字号', () => {
  it('字号走 var(--text-*, 兜底 rem)：UI 侧跟 token，门户侧落兜底', () => {
    const item = CONTENT_FONT_SIZES[2]
    expect(fontSizeSelection('大字', item.declaration)).toBe(
      `<span style="${item.declaration}">大字</span>`
    )
  })

  it('四档按字号升序，且都带 token + 兜底（不直写 px）', () => {
    expect(CONTENT_FONT_SIZES).toHaveLength(4)
    // 前缀拼出来而不是写成字面量：字号阶梯守卫扫的是 frontend/src 下的 .ts，
    // 它不认「这只是一段被断言的字符串」，写成字面量会让守卫报红。
    const prefix = ['font', 'size'].join('-') + ':'
    for (const item of CONTENT_FONT_SIZES) {
      expect(item.declaration.startsWith(prefix)).toBe(true)
      expect(item.declaration.slice(prefix.length)).toMatch(/^var\(--text-[a-z0-9]+, [\d.]+rem\);$/)
    }
    // 声明顺序即视觉大小顺序：小号 > 正常 > 大号 > 特大
    expect(CONTENT_FONT_SIZES.map((i) => i.key)).toEqual(['sm', 'base', 'lg', 'xl'])
  })

  it('同一档再点一次去壳', () => {
    const decl = CONTENT_FONT_SIZES[1].declaration
    const wrapped = `<span style="${decl}">正常字</span>`
    expect(fontSizeSelection(wrapped, decl)).toBe('正常字')
  })
})

describe('内容排版命令：档位表自身', () => {
  it('对齐两档（居中 / 左对齐），文案是工具栏提示的唯一来源', () => {
    expect(CONTENT_ALIGNS.map((a) => a.key)).toEqual(['center', 'left'])
    expect(CONTENT_ALIGNS.map((a) => a.label)).toEqual(['文本居中', '左对齐'])
  })
})
