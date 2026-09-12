// AST → 纯文本投影（#880 / ADR-0044）。
//
// 为什么不是正则剥标记：正则与渲染器是两套规则，代码块内的符号、转义、嵌套标记都会与
// 真实渲染不一致，长期必然漂移。本实现用**与渲染同一个解析器**拿节点流再投影，
// 所以「列表摘要显示的」与「点进去看到的」同源。
import { describe, it, expect } from 'vitest'
import { markdownToPlainText } from '../markdownText'

describe('markdownToPlainText（摘要用纯文本投影）', () => {
  it('剥掉标题/加粗/行内代码标记，保留文字', () => {
    expect(markdownToPlainText('## 排查步骤')).toBe('排查步骤')
    expect(markdownToPlainText('先看 **电瓶电压**')).toBe('先看 电瓶电压')
    expect(markdownToPlainText('量 `E01` 电压')).toBe('量 E01 电压')
  })

  it('代码块内容原样保留：块内的符号不被当作语法', () => {
    const out = markdownToPlainText('```\n# 这不是标题\n**也不是加粗**\n```')
    expect(out).toContain('# 这不是标题')
    expect(out).toContain('**也不是加粗**')
  })

  it('链接保留可读文本，不把 URL 与括号带进摘要', () => {
    const out = markdownToPlainText('见 [维修手册](https://example.com/a/b) 第 3 页')
    expect(out).toContain('维修手册')
    expect(out).not.toContain('https://')
    expect(out).not.toContain('](')
  })

  it('列表与引用剥成纯文本', () => {
    expect(markdownToPlainText('- 先断电\n- 再量电压')).toBe('先断电 再量电压')
    expect(markdownToPlainText('1. 第一步\n2. 第二步')).toBe('第一步 第二步')
    expect(markdownToPlainText('> 注意安全')).toBe('注意安全')
  })

  it('折叠换行为单空格（摘要按单行渲染）', () => {
    const out = markdownToPlainText('第一行\n第二行\n\n第三行')
    expect(out).toBe('第一行 第二行 第三行')
  })

  it('纯文本原样保留（不误伤没有标记的内容）', () => {
    expect(markdownToPlainText('液压油多久换一次')).toBe('液压油多久换一次')
  })

  it('空内容返回空串（不抛错）', () => {
    expect(markdownToPlainText('')).toBe('')
  })

  it('raw HTML 以字面量保留（与 escape 渲染口径一致，摘要也不隐藏它）', () => {
    const out = markdownToPlainText('正常文字 <script>alert(1)</script> 结尾')
    expect(out).toContain('<script>')
  })
})
