// 内容排版按钮的选区处理（#2631 反馈修复）。
//
// 每一条都对应一次真实环境里复现过的错误，不是假想的边角：
//   ① **图片被吃掉**：IR 的图片节点里没有可选中的文字，`getSelection()` 返回空串，
//      而 `insertMD` 尊重 DOM 选区 —— 占位文字把整张图片覆盖掉，作者内容当场丢失。
//   ② **再点一次去不掉壳**：块级 HTML 渲染出来的选区文本只有内层文字，看不到外面那层
//      `<div align="center">`，纯函数的「已包过」判断永远不成立，越点越套。
//   ③ **提示压在相邻按钮上**：不填 `tipPosition` 时 Vditor 拼出的 class 是
//      `vditor-tooltipped__undefined`，没有对应 CSS，气泡退化成无定位的绝对定位。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type Vditor from 'vditor'
import { createContentToolbarItems } from '../markdownEditorToolbar'

/** 造 Vditor 替身：只覆盖命令真正用到的几个成员 */
function fakeVditor(selectionText = '') {
  const tip = vi.fn()
  const input = vi.fn()
  const addToUndoStack = vi.fn()
  const insertMD = vi.fn()
  const focus = vi.fn()
  const instance = {
    tip,
    focus,
    insertMD,
    getSelection: vi.fn(() => selectionText),
    getValue: vi.fn(() => ''),
    vditor: {
      undo: { addToUndoStack },
      options: { input },
      ir: { element: document.createElement('div') }
    }
  }
  return { instance: instance as unknown as Vditor, tip, insertMD, input, addToUndoStack, focus }
}

/** 把选区设成「框住某个元素的内容」（折叠光标用 collapse=true） */
function selectElement(element: Element, collapse = false) {
  const range = document.createRange()
  range.selectNodeContents(element)
  if (collapse) range.collapse(false)
  const selection = window.getSelection()!
  selection.removeAllRanges()
  selection.addRange(range)
}

/** 造 Vditor IR 的块级 HTML 节点：左侧源码 code + 右侧渲染预览 */
function mountHtmlBlock(raw: string, innerText: string): HTMLElement {
  const block = document.createElement('div')
  block.className = 'vditor-ir__node'
  block.setAttribute('data-type', 'html-block')
  const code = document.createElement('code')
  code.setAttribute('data-type', 'html-block')
  code.textContent = raw
  const source = document.createElement('pre')
  source.className = 'vditor-ir__marker--pre vditor-ir__marker'
  source.appendChild(code)
  const preview = document.createElement('pre')
  preview.className = 'vditor-ir__preview'
  preview.textContent = innerText
  block.append(source, preview)
  document.body.appendChild(block)
  return block
}

let current: ReturnType<typeof fakeVditor>

function itemOf(name: string) {
  const items = createContentToolbarItems(() => current.instance)
  const found = items.find((i) => i.name === name)
  if (!found) throw new Error(`没有 ${name}`)
  return found
}

describe('内容排版按钮：选区处理', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    current = fakeVditor()
    window.getSelection()?.removeAllRanges()
  })

  describe('图片居中', () => {
    it('选中图片时按 ![alt](src) 拼回语法，图片不会被占位文字覆盖', () => {
      // 复刻 Vditor IR 的图片节点：源码标记 + 一个 img，没有可选中的文字
      const paragraph = document.createElement('p')
      const node = document.createElement('span')
      node.className = 'vditor-ir__node'
      node.setAttribute('data-type', 'img')
      const img = document.createElement('img')
      img.setAttribute('src', 'https://x/a.png')
      img.setAttribute('alt', '示意图')
      node.appendChild(img)
      paragraph.appendChild(node)
      document.body.appendChild(paragraph)

      selectElement(paragraph)
      // IR 里 getSelection 拿不到图片（替身保持空串，与实测一致）
      itemOf('image-center').click!(new Event('click'), {})

      expect(current.insertMD).toHaveBeenCalledWith(
        '<div align="center"><img src="https://x/a.png" alt="示意图"></div>'
      )
      expect(current.tip).not.toHaveBeenCalled()
    })

    it('光标折叠（没选中东西）时只弹提示，不动文档', () => {
      const paragraph = document.createElement('p')
      paragraph.textContent = '一段文字'
      document.body.appendChild(paragraph)
      selectElement(paragraph, true)

      itemOf('image-center').click!(new Event('click'), {})

      expect(current.insertMD).not.toHaveBeenCalled()
      expect(current.tip).toHaveBeenCalled()
    })

    it('选中纯文字时提示改用文本居中，不把文字默默包进居中容器', () => {
      current = fakeVditor('纯文字段落')
      const paragraph = document.createElement('p')
      paragraph.textContent = '纯文字段落'
      document.body.appendChild(paragraph)
      selectElement(paragraph)

      itemOf('image-center').click!(new Event('click'), {})

      expect(current.insertMD).not.toHaveBeenCalled()
      expect(current.tip).toHaveBeenCalled()
    })
  })

  describe('再点一次去壳', () => {
    it('选区落在块级 HTML 里时读 code 上的真实源码，去壳走 DOM 替换', () => {
      const block = mountHtmlBlock('<div align="center">文字</div>', '文字')
      const preview = block.querySelector('.vditor-ir__preview')!
      selectElement(preview)

      itemOf('align-center').click!(new Event('click'), {})

      // 走的是 DOM 替换：块节点被换成普通段落，且 v-model 被同步
      expect(current.insertMD).not.toHaveBeenCalled()
      expect(document.querySelector('[data-type="html-block"]')).toBeNull()
      expect(document.querySelector('p')?.textContent).toBe('文字')
      expect(current.addToUndoStack).toHaveBeenCalled()
      expect(current.input).toHaveBeenCalled()
    })

    it('块级 HTML 里包的是另一种外壳时，包一层新的而不是替换掉原内容', () => {
      const block = mountHtmlBlock('<div align="left">文字</div>', '文字')
      const preview = block.querySelector('.vditor-ir__preview')!
      selectElement(preview)

      itemOf('align-center').click!(new Event('click'), {})

      // 包壳结果包含原选区源码 ⇒ 走 insertMD，不走 DOM 替换
      expect(current.insertMD).toHaveBeenCalledWith(
        '<div align="center"><div align="left">文字</div></div>'
      )
    })
  })
})

describe('内容排版按钮：提示定位', () => {
  it('每一项都填了 tipPosition —— 留空会拼出不存在的 class，气泡压在相邻按钮上', () => {
    current = fakeVditor()
    const items = createContentToolbarItems(() => current.instance)
    for (const item of items) {
      expect(item.tipPosition, `${item.name} 缺 tipPosition`).toBe('ne')
    }
  })

  it('颜色 / 字号子菜单的文字拼在 icon 里（Custom 构造器会用 icon 覆盖 tip）', () => {
    current = fakeVditor()
    const items = createContentToolbarItems(() => current.instance)
    const color = items.find((i) => i.name === 'color')!
    const fontSize = items.find((i) => i.name === 'font-size')!
    expect(color.toolbar).toHaveLength(5)
    expect(fontSize.toolbar).toHaveLength(4)
    for (const sub of [...color.toolbar!, ...fontSize.toolbar!]) {
      expect(sub.icon).toContain(sub.tip)
    }
    // 五个色点必须是各自的色值，不能一律 currentColor
    const fills = color.toolbar!.map((s) => s.icon.match(/fill:(#[0-9A-F]{6})/i)?.[1])
    expect(new Set(fills).size).toBe(5)
  })
})