/**
 * 表格结构命令（增行 / 增列 / 删行 / 删列）。
 *
 * 为什么自己实现而不用 Vditor 自带的：Vditor 这四条命令**只存在于 wysiwyg 模式的
 * popover 里**（`node_modules/vditor/dist/index.js` 里 `vditor.wysiwyg.popover` 挂按钮），
 * 而本项目用的是 **IR 模式，IR 根本没有 popover** —— 所以原生按钮永远不出现。
 * 原生热键（`⌘-` / `⌘=` 等，`fixTable` 里）在 Windows 上映射成 `Ctrl+-` / `Ctrl+=`，
 * 与浏览器缩放冲突，且合成 KeyboardEvent 实测**不触发**（探针 probe3）。
 *
 * 于是命令体落在这里，照抄 Vditor 原生的 DOM 语义（列数、align 属性、
 * 「删到只剩表头」这一终态），但**不依赖它内部不可 import 的函数**。
 *
 * 造节点一律用 DOM API 而**不用 `insertAdjacentHTML`**：后者按宿主元素的上下文解析 HTML，
 * 在 `<tr>` 里塞 `<td>` 这种片段并非所有实现都可靠（happy-dom 会静默丢掉，测试环境里
 * 表现为「命令没效果」）。用 `createElement` + `insertBefore` 既没有解析歧义，也顺便
 * 拿到了强类型。
 *
 * 本文件只做 DOM 变换，不负责同步 v-model / undo 栈 —— 那三件（改 DOM → input 回调 →
 * addToUndoStack）在 `components/tutor/markdownEditorToolbar.ts` 里按实测顺序拼，
 * 因为只有那里拿得到 `Vditor` 实例。
 */

/** 单元格：TD 是数据格，TH 是表头格（Vditor 的原生删行也拒绝删表头） */
export type TableCell = HTMLTableCellElement

/** 命令失败的三类原因（都要给出人话提示，不能静默不动） */
export type TableCommandFailure = 'no-cell' | 'header-row' | 'last-column'

export type TableCommandResult =
  | { ok: true; caret?: TableCell }
  | { ok: false; failure: TableCommandFailure }

const FAILED = (failure: TableCommandFailure): TableCommandResult => ({ ok: false, failure })

/** 从选区起点上溯到最近的单元格；不在表格里返回 null */
export function cellFromRange(range: Range | null): TableCell | null {
  if (!range) return null
  const start = range.startContainer
  let current: Element | null = start.nodeType === 1 ? (start as Element) : start.parentElement
  while (current) {
    if (current.tagName === 'TD' || current.tagName === 'TH') return current as TableCell
    current = current.parentElement
  }
  return null
}

/** 单元格所在的行（用 `closest` 取，类型上就是 `HTMLTableRowElement`） */
function rowOf(cell: TableCell): HTMLTableRowElement | null {
  return cell.closest('tr')
}

/** 单元格在它那一行里的序号（原生实现就是「数前面有几个兄弟」） */
function cellIndex(cell: TableCell): number {
  const row = rowOf(cell)
  if (!row) return 0
  return Array.prototype.indexOf.call(row.children, cell) as number
}

/** 这一行里第 index 个格子（越界给 null，不做自动补格） */
function cellAt(row: HTMLTableRowElement | undefined, index: number): TableCell | null {
  const cell = row?.cells[index]
  return cell ?? null
}

/** 造一个与参照格同 align 的新格子 */
function blankCell(reference: TableCell, tagName: 'td' | 'th'): TableCell {
  const created = document.createElement(tagName) as TableCell
  const align = reference.getAttribute('align')
  if (align) created.setAttribute('align', align)
  // 一个空格子：浏览器里不塞东西高度会塌，Vditor 原生也塞了一个空格
  created.textContent = ' '
  return created
}

/** 造一行与参照行同列数、逐格同 align 的空白行 */
function blankRow(reference: HTMLTableRowElement): HTMLTableRowElement {
  const row = document.createElement('tr')
  for (let i = 0; i < reference.childElementCount; i++) {
    row.appendChild(blankCell(reference.children[i] as TableCell, 'td'))
  }
  return row
}

/**
 * 在当前行**下方**插入一行。
 *
 * 表头行（TH）下方插入要落进 tbody：原生实现用 `insertAdjacentHTML('afterend',
 * '<tbody><tr>…')`，那样在已有 tbody 时会造出第二个 tbody（DOM 合法但脏）。
 * 这里改成复用已有 tbody、没有才建，`getValue()` 的输出与原生一致。
 */
export function insertRowBelow(cell: TableCell): TableCommandResult {
  const row = rowOf(cell)
  const table = row?.closest('table')
  if (!row || !table) return FAILED('no-cell')
  const index = cellIndex(cell)
  const added = blankRow(row)
  if (cell.tagName === 'TH') {
    // 表头下方插入要落进 tbody 首行：复用已有 tbody，没有才造一个真的接上去
    let tbody = table.tBodies[0]
    if (!tbody) {
      tbody = document.createElement('tbody')
      table.appendChild(tbody)
    }
    tbody.insertBefore(added, tbody.firstElementChild)
  } else {
    row.after(added)
  }
  return { ok: true, caret: cellAt(added, index) ?? undefined }
}

/**
 * 在当前列**右侧**（`afterend`）或**左侧**（`beforebegin`）插入一列。
 *
 * 首行一定是表头：原生实现按 `table.rows[0]` 判 `i === 0` 造 `<th>`，这里照办
 * （所以没有 thead 的表也能用）。
 */
export function insertColumn(cell: TableCell, position: 'afterend' | 'beforebegin' = 'afterend'): TableCommandResult {
  const row = rowOf(cell)
  const table = row?.closest('table')
  if (!row || !table) return FAILED('no-cell')
  const index = cellIndex(cell)
  for (let i = 0; i < table.rows.length; i++) {
    const target = table.rows[i]?.cells[index]
    if (!target) continue
    const added = blankCell(target, i === 0 ? 'th' : 'td')
    if (position === 'beforebegin') target.before(added)
    else target.after(added)
  }
  // 光标落进新插入的那个空格子，作者可以立刻打字
  const caretIndex = position === 'beforebegin' ? index : index + 1
  return { ok: true, caret: cellAt(row, caretIndex) ?? undefined }
}

/**
 * 删除当前行。
 *
 * 表头行（TH）不删 —— 与原生 `deleteRow` 的判据一致：删掉表头会让整张表失去列语义。
 * tbody 只剩这一行时**连 tbody 一起删**（原生同）：留下 `<table><thead>…</thead></table>`
 * 的空表会让 `getValue()` 产出坏表格；删掉 tbody 后 Lute 输出「只有表头」的合法表，
 * 实测这一终态安全（探针 probe5：`| A | B |\n| - | - |`）。
 */
export function deleteRow(cell: TableCell): TableCommandResult {
  if (cell.tagName !== 'TD') return FAILED('header-row')
  const row = rowOf(cell)
  const table = row?.closest('table')
  if (!row || !table) return FAILED('no-cell')
  const index = cellIndex(cell)
  const rows = Array.from(table.rows)
  const rowIndex = rows.indexOf(row)

  // 先挑落脚的格子（删完这一行，光标不能凭空消失）。
  // 顺序：下一行同列 → 上一行同列 → 表头同列。**数据行优先于表头**：删完落在表头格上，
  // 作者接着打字就会改到表头文字，这是个很容易踩的坑（原生的兜底恰好会落到 TH）。
  let caret: TableCell | null = null
  for (const candidate of [rows[rowIndex + 1], rows[rowIndex - 1]]) {
    const next = cellAt(candidate, index)
    if (next?.tagName === 'TD') {
      caret = next
      break
    }
  }
  // 整张表只剩表头（tbody 已空）时才落到表头
  if (!caret) caret = cellAt(rows[0], index)

  const tbody = row.parentElement
  if (tbody && tbody.tagName === 'TBODY' && tbody.childElementCount === 1) tbody.remove()
  else row.remove()
  return { ok: true, caret: caret ?? undefined }
}

/**
 * 删除当前列。
 *
 * 与原生的一处**刻意不同**：原生在「某行只剩一个格子」时会 `tableElement.remove()`
 * 把整张表删掉 —— 用户只是想删一列，却丢了整张表，且在工具栏上没法后悔。
 * 这里改成拒绝（`last-column`）并提示「要删整表请自己选中它删」，破坏性动作留给用户选。
 */
export function deleteColumn(cell: TableCell): TableCommandResult {
  const row = rowOf(cell)
  const table = row?.closest('table')
  if (!row || !table) return FAILED('no-cell')
  const index = cellIndex(cell)
  const rows = Array.from(table.rows)
  if (rows.every((r) => r.cells.length <= 1)) return FAILED('last-column')

  for (const r of rows) r.cells[index]?.remove()
  const caret = cellAt(row, Math.min(index, row.cells.length - 1))
  return { ok: true, caret: caret ?? undefined }
}
