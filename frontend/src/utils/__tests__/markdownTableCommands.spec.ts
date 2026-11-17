// 表格结构命令（增行 / 增列 / 删行 / 删列）。
//
// 这四条命令的判据全部来自 Vditor 原生的 DOM 语义（不是猜的）：
// 删行只认 TD、tbody 只剩一行时连 tbody 一起删、插列要按 table.rows[0] 造 th。
// 与原生**刻意不同**的地方也在下面逐条钉住（删最后一列改成拒绝、表头下方插行复用已有 tbody），
// 免得将来有人「顺手对齐原生」把这两处改回去。
import { describe, expect, it } from 'vitest'
import {
  cellFromRange,
  deleteColumn,
  deleteRow,
  insertColumn,
  insertRowBelow
} from '../markdownTableCommands'

/** 造一张 Vditor IR 形态的表（thead 全 th + tbody 若干 td） */
function buildTable(rows: number, cols: number): HTMLTableElement {
  const table = document.createElement('table')
  const thead = table.createTHead()
  const headRow = thead.insertRow()
  for (let c = 0; c < cols; c++) headRow.appendChild(document.createElement('th'))
  const tbody = table.createTBody()
  for (let r = 0; r < rows; r++) {
    const row = tbody.insertRow()
    for (let c = 0; c < cols; c++) row.appendChild(document.createElement('td'))
  }
  document.body.appendChild(table)
  return table
}

/** 取第 rowIndex 行（0 = 表头）第 colIndex 列 */
function cell(table: HTMLTableElement, rowIndex: number, colIndex: number): HTMLTableCellElement {
  return table.rows[rowIndex].cells[colIndex]
}

/**
 * 取 tbody 里的第 index 行。
 *
 * happy-dom 上没有 `tbody.rows` 集合（`table.rows` 有），所以从 `children` 转一道；
 * 组件代码里一律用 `cell.closest('tr')`，不依赖这个集合。
 */
function tbodyRow(table: HTMLTableElement, index: number): HTMLTableRowElement {
  return table.tBodies[0].children[index] as HTMLTableRowElement
}

describe('cellFromRange', () => {
  it('从格子内部的文本节点上溯到格子', () => {
    const table = buildTable(1, 2)
    const target = cell(table, 1, 1)
    target.textContent = '内容'
    const range = document.createRange()
    range.selectNodeContents(target)
    expect(cellFromRange(range)).toBe(target)
  })

  it('不在表格里返回 null', () => {
    const paragraph = document.createElement('p')
    paragraph.textContent = '正文'
    document.body.appendChild(paragraph)
    const range = document.createRange()
    range.selectNodeContents(paragraph)
    expect(cellFromRange(range)).toBeNull()
  })

  it('null 选区返回 null（blur 后没有 range 的情形）', () => {
    expect(cellFromRange(null)).toBeNull()
  })
})

describe('insertRowBelow', () => {
  it('数据行下方插入一行同列数，光标落在新行同列', () => {
    const table = buildTable(2, 3)
    const result = insertRowBelow(cell(table, 1, 1))
    expect(result.ok).toBe(true)
    expect(table.tBodies[0].childElementCount).toBe(3)
    expect(tbodyRow(table, 1).cells.length).toBe(3)
    expect(result.ok && result.caret).toBe(tbodyRow(table, 1).cells[1])
  })

  it('表头行下方插入落进已有 tbody 首行（不再造第二个 tbody）', () => {
    const table = buildTable(2, 2)
    const result = insertRowBelow(cell(table, 0, 0))
    expect(result.ok).toBe(true)
    expect(table.tBodies.length).toBe(1)
    expect(table.tBodies[0].childElementCount).toBe(3)
    // 新行在 tbody 首行，原有两行顺次后移
    expect(result.ok && result.caret).toBe(tbodyRow(table, 0).cells[0])
  })

  it('保留原行的 align 属性', () => {
    const table = buildTable(1, 2)
    cell(table, 1, 0).setAttribute('align', 'center')
    cell(table, 1, 1).setAttribute('align', 'right')
    insertRowBelow(cell(table, 1, 0))
    const added = table.tBodies[0].children[1] as HTMLTableRowElement
    expect(added.cells[0].getAttribute('align')).toBe('center')
    expect(added.cells[1].getAttribute('align')).toBe('right')
  })
})

describe('insertColumn', () => {
  it('默认在右侧插入：首行补 th、其余补 td', () => {
    const table = buildTable(2, 2)
    const result = insertColumn(cell(table, 1, 0))
    expect(result.ok).toBe(true)
    expect(table.rows.length).toBe(3) // thead 一行 + tbody 两行
    for (const row of Array.from(table.rows)) expect(row.cells.length).toBe(3)
    expect(table.rows[0].cells[1].tagName).toBe('TH')
    expect(table.rows[2].cells[1].tagName).toBe('TD')
    expect(result.ok && result.caret).toBe(cell(table, 1, 1))
  })

  it('beforebegin 在左侧插入，光标落在新列', () => {
    const table = buildTable(1, 2)
    const result = insertColumn(cell(table, 1, 1), 'beforebegin')
    expect(result.ok).toBe(true)
    expect(result.ok && result.caret).toBe(cell(table, 1, 1))
    expect(table.rows[1].cells.length).toBe(3)
  })
})

describe('deleteRow', () => {
  it('删掉当前数据行', () => {
    const table = buildTable(2, 2)
    const target = cell(table, 1, 0)
    const result = deleteRow(target)
    expect(result.ok).toBe(true)
    expect(table.tBodies[0].childElementCount).toBe(1)
    expect(table.contains(target)).toBe(false)
  })

  it('表头行拒绝删除（与原生 deleteRow 的判据一致）', () => {
    const table = buildTable(2, 2)
    const result = deleteRow(cell(table, 0, 0))
    expect(result).toEqual({ ok: false, failure: 'header-row' })
    expect(table.rows.length).toBe(3)
  })

  it('tbody 只剩一行时连 tbody 一起删，留下只有表头的合法表', () => {
    const table = buildTable(1, 2)
    deleteRow(cell(table, 1, 0))
    expect(table.tBodies.length).toBe(0)
    expect(table.tHead).not.toBeNull()
    expect(table.rows.length).toBe(1)
  })

  it('光标落到上一行同列；没有上一行则落到下一行', () => {
    const table = buildTable(3, 2)
    // 删第二行（tbody 首行）→ 上方只有表头，故落到原第三行（下移一位后仍是下标 2）
    const result = deleteRow(cell(table, 1, 1))
    expect(result.ok && result.caret).toBe(table.rows[1].cells[1])
  })
})

describe('deleteColumn', () => {
  it('删掉当前列（每行都删该列）', () => {
    const table = buildTable(2, 3)
    const result = deleteColumn(cell(table, 1, 1))
    expect(result.ok).toBe(true)
    for (const row of Array.from(table.rows)) expect(row.cells.length).toBe(2)
    expect(result.ok && result.caret).toBe(cell(table, 1, 1))
  })

  it('只剩最后一列时拒绝，且不动整张表（原生会删整表，这里刻意不同）', () => {
    const table = buildTable(2, 1)
    const result = deleteColumn(cell(table, 1, 0))
    expect(result).toEqual({ ok: false, failure: 'last-column' })
    expect(document.body.contains(table)).toBe(true)
    expect(table.rows.length).toBe(3)
  })
})

