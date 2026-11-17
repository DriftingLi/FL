// 表格结构按钮的执行链（#2631 需求 ②③）。
//
// 三条判据都是实测来的，别「优化」掉：
//   ① 命令改完 DOM 必须显式调 `options.input(getValue())` —— 否则 getValue 对了、v-model 还是旧值，
//      保存时写回旧内容（探针 probe5 亲测：只改 DOM 时 `window.__model` 不变）；
//   ② 必须进 undo 栈，否则这次改动撤销不了；
//   ③ 拿不到格子 / 命中的是表头 / 只剩最后一列时，只弹提示、**不动 DOM** —— 静默不动更让人困惑。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type Vditor from 'vditor'
import { createTableToolbarItems } from '../markdownEditorToolbar'

/** 造一张 Vditor IR 形态的表：thead 一行 th + tbody 若干 td */
function mountTable(rows: number, cols: number): HTMLTableElement {
  const table = document.createElement('table')
  const headRow = table.createTHead().insertRow()
  for (let c = 0; c < cols; c++) headRow.appendChild(document.createElement('th'))
  const tbody = table.createTBody()
  for (let r = 0; r < rows; r++) {
    const row = tbody.insertRow()
    for (let c = 0; c < cols; c++) row.appendChild(document.createElement('td'))
  }
  document.body.appendChild(table)
  return table
}

/** 把光标放进某个格子（模拟用户先点了格子、再点工具栏） */
function caretIn(cell: HTMLTableCellElement) {
  const range = document.createRange()
  range.selectNodeContents(cell)
  const selection = window.getSelection()!
  selection.removeAllRanges()
  selection.addRange(range)
}

/**
 * 取 tbody 第 index 行的格子。
 *
 * 不用 `tbody.rows`：happy-dom 上没有这个集合（`table.rows` 有），测试会当场 `undefined`。
 * 组件代码里一律用 `cell.closest('tr')`，两处都不依赖这个集合。
 */
function rowCells(table: HTMLTableElement, index: number): HTMLCollectionOf<HTMLTableCellElement> {
  return (table.tBodies[0].children[index] as HTMLTableRowElement).cells
}

function clearSelection() {
  window.getSelection()?.removeAllRanges()
}

/**
 * 造 Vditor 替身。`markdown` 是 getValue 的返回值，命令跑完之后测试会自己改成新值 ——
 * 真实实例由 Lute 从 DOM 重新算，替身里没必要复刻。
 */
function fakeVditor() {
  const tip = vi.fn()
  const input = vi.fn()
  const addToUndoStack = vi.fn()
  const focus = vi.fn()
  const instance = {
    tip,
    focus,
    getValue: vi.fn(() => '| A |\n| - |\n| x |'),
    vditor: {
      undo: { addToUndoStack },
      options: { input },
      ir: { range: null as Range | null }
    }
  }
  return { instance: instance as unknown as Vditor, tip, input, addToUndoStack, focus, raw: instance }
}

function itemOf(name: string) {
  const items = createTableToolbarItems(() => current.instance)
  const found = items.find((i) => i.name === name)
  if (!found) throw new Error(`没有 ${name}`)
  return found
}

let current: ReturnType<typeof fakeVditor>

describe('表格结构按钮', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    current = fakeVditor()
  })

  it('四个按钮齐备，顺序为「插行 / 插列 / 删行 / 删列」', () => {
    const items = createTableToolbarItems(() => current.instance)
    expect(items.map((i) => i.name)).toEqual([
      'table-insert-row',
      'table-insert-column',
      'table-delete-row',
      'table-delete-column'
    ])
  })

  describe('删行', () => {
    it('删掉当前行，并同步 v-model + 进 undo 栈', () => {
      const table = mountTable(2, 2)
      caretIn(rowCells(table, 0)[0])
      const before = document.querySelectorAll('tbody tr').length
      itemOf('table-delete-row').click!(new Event('click'), {})

      expect(document.querySelectorAll('tbody tr').length).toBe(before - 1)
      // 同步链：input 收到的是命令跑完之后 getValue 的值
      expect(current.input).toHaveBeenCalledWith(current.raw.getValue())
      expect(current.addToUndoStack).toHaveBeenCalled()
      expect(current.focus).toHaveBeenCalled()
      expect(current.tip).not.toHaveBeenCalled()
    })

    it('光标不在表格里：只提示、不动文档', () => {
      const paragraph = document.createElement('p')
      paragraph.textContent = '正文'
      document.body.appendChild(paragraph)
      const range = document.createRange()
      range.selectNodeContents(paragraph)
      window.getSelection()!.removeAllRanges()
      window.getSelection()!.addRange(range)

      itemOf('table-delete-row').click!(new Event('click'), {})

      expect(current.tip).toHaveBeenCalledWith(expect.stringContaining('光标放进表格'), 2000)
      expect(current.addToUndoStack).not.toHaveBeenCalled()
      expect(document.querySelector('table')).toBeNull()
    })

    it('命中表头：拒绝并说明「表头行不能删除」', () => {
      const table = mountTable(2, 2)
      caretIn(table.rows[0].cells[0]) // thead 的 th
      itemOf('table-delete-row').click!(new Event('click'), {})

      expect(current.tip).toHaveBeenCalledWith(expect.stringContaining('表头行不能删除'), 2500)
      expect(current.input).not.toHaveBeenCalled()
      expect(table.rows.length).toBe(3)
    })

    it('实时选区没了时退回 vditor.ir.range（点过别处再回来的情形）', () => {
      const table = mountTable(2, 2)
      caretIn(rowCells(table, 0)[0])
      // 模拟 blur：Vditor 把当时的 range 存进 ir.range，浏览器选区被清掉
      current.raw.vditor.ir.range = window.getSelection()!.getRangeAt(0).cloneRange()
      clearSelection()

      itemOf('table-delete-row').click!(new Event('click'), {})

      expect(document.querySelectorAll('tbody tr').length).toBe(1)
      expect(current.input).toHaveBeenCalled()
    })
  })

  describe('删列', () => {
    it('删掉当前列', () => {
      const table = mountTable(2, 3)
      caretIn(rowCells(table, 0)[1])
      itemOf('table-delete-column').click!(new Event('click'), {})
      for (const row of Array.from(table.rows)) expect(row.cells.length).toBe(2)
      expect(current.input).toHaveBeenCalled()
    })

    it('只剩最后一列：拒绝，整张表留着（原生会连表一起删，这里刻意不同）', () => {
      const table = mountTable(2, 1)
      caretIn(rowCells(table, 0)[0])
      itemOf('table-delete-column').click!(new Event('click'), {})

      expect(current.tip).toHaveBeenCalledWith(expect.stringContaining('最后一列'), 2500)
      expect(document.body.contains(table)).toBe(true)
      expect(current.input).not.toHaveBeenCalled()
    })
  })

  describe('插行 / 插列', () => {
    it('插行后多一行同列数', () => {
      const table = mountTable(1, 2)
      caretIn(rowCells(table, 0)[0])
      itemOf('table-insert-row').click!(new Event('click'), {})
      expect(table.tBodies[0].childElementCount).toBe(2)
      expect((table.tBodies[0].children[1] as HTMLTableRowElement).cells.length).toBe(2)
      expect(current.input).toHaveBeenCalled()
    })

    it('插列后每行多一格，首行仍是 th', () => {
      const table = mountTable(2, 2)
      caretIn(rowCells(table, 0)[0])
      itemOf('table-insert-column').click!(new Event('click'), {})
      for (const row of Array.from(table.rows)) expect(row.cells.length).toBe(3)
      expect(table.rows[0].cells[1].tagName).toBe('TH')
      expect(current.input).toHaveBeenCalled()
    })
  })

  it('替身没给 undo（早期版本 / 未初始化）时不炸', () => {
    const table = mountTable(1, 2)
    caretIn(rowCells(table, 0)[0])
    current.raw.vditor.undo = undefined as never
    expect(() => itemOf('table-delete-row').click!(new Event('click'), {})).not.toThrow()
    expect(current.input).toHaveBeenCalled()
  })
})

